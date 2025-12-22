package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jjulito/reserva/internal/core/domain"
	"github.com/jjulito/reserva/internal/core/ports"
)

type BookingService struct {
	seatRepo        ports.SeatRepository
	reservationRepo ports.ReservationRepository
	eventRepo       ports.EventRepository
	lockRepo        ports.LockRepository
}

func NewBookingService(
	seatRepo ports.SeatRepository,
	reservationRepo ports.ReservationRepository,
	eventRepo ports.EventRepository,
	lockRepo ports.LockRepository,
) *BookingService {
	return &BookingService{
		seatRepo:        seatRepo,
		reservationRepo: reservationRepo,
		eventRepo:       eventRepo,
		lockRepo:        lockRepo,
	}
}

func (s *BookingService) ListEvents(ctx context.Context) ([]domain.Event, error) {
	return s.eventRepo.ListEvents(ctx)
}

func (s *BookingService) GetEventSeats(ctx context.Context, eventID uuid.UUID) ([]domain.Seat, error) {
	return s.seatRepo.GetSeatsByEvent(ctx, eventID)
}

// CreateReservation attempts to reserve a seat handling race conditions
func (s *BookingService) CreateReservation(ctx context.Context, userID, seatID, eventID uuid.UUID) (*domain.Reservation, error) {
	// 1. Distributed Lock (Redis) - Fail fast if someone is already processing this seat
	lockKey := fmt.Sprintf("lock:seat:%s", seatID.String())
	acquired, err := s.lockRepo.AcquireLock(ctx, lockKey, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !acquired {
		return nil, domain.ErrSeatLocked // Someone else is processing it right now
	}
	defer s.lockRepo.ReleaseLock(ctx, lockKey)

	// 2. Fetch Seat State
	seat, err := s.seatRepo.GetSeat(ctx, seatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seat: %w", err)
	}

	// 3. Domain Validation
	if seat.Status != domain.SeatAvailable {
		return nil, domain.ErrSeatUnavailable
	}

	// 4. Create Reservation Object
	reservation := domain.NewReservation(userID, seatID, eventID, 5*time.Minute)

	// 5. Transactional Update (Optimistic Locking via Repository)
	// We update seat status to LOCKED, increment version, and set reserved_by
	// The repository method MUST ensure it only updates if version matches seat.Version
	err = s.seatRepo.UpdateSeatStatus(ctx, seatID, domain.SeatLocked, &userID, seat.Version)
	if err != nil {
		// If this fails, it means the version changed between Read (step 2) and Write (step 5)
		return nil, domain.ErrSeatUnavailable
	}

	// 6. Save Reservation
	if err := s.reservationRepo.CreateReservation(ctx, reservation); err != nil {
		return nil, fmt.Errorf("failed to save reservation: %w", err)
	}

	return reservation, nil
}

func (s *BookingService) GetReservation(ctx context.Context, reservationID uuid.UUID) (*domain.Reservation, error) {
	return s.reservationRepo.GetReservation(ctx, reservationID)
}

func (s *BookingService) CancelReservation(ctx context.Context, reservationID uuid.UUID) error {
	// 1. Get Reservation
	reservation, err := s.reservationRepo.GetReservation(ctx, reservationID)
	if err != nil {
		return err
	}

	// 2. Validate Status
	if reservation.Status != domain.ReservationPending {
		return fmt.Errorf("cannot cancel reservation in status: %s", reservation.Status)
	}

	// 3. Update Reservation Status
	err = s.reservationRepo.UpdateStatus(ctx, reservationID, domain.ReservationCancelled)
	if err != nil {
		return err
	}

	// 4. Release Seat
	seat, err := s.seatRepo.GetSeat(ctx, reservation.SeatID)
	if err != nil {
		// Log error? The reservation is cancelled but seat might be stuck.
		return err
	}

	// We optimistically update the seat back to AVAILABLE.
	// We don't need a ReservedBy here anymore.
	err = s.seatRepo.UpdateSeatStatus(ctx, reservation.SeatID, domain.SeatAvailable, nil, seat.Version)
	if err != nil {
		return fmt.Errorf("failed to release seat: %w", err)
	}

	return nil
}

// ConfirmReservation finalizes a reservation after payment.
// It updates the reservation status to PAID and the seat status to RESERVED.
func (s *BookingService) ConfirmReservation(ctx context.Context, reservationID uuid.UUID) error {
	// 1. Get Reservation
	reservation, err := s.reservationRepo.GetReservation(ctx, reservationID)
	if err != nil {
		return err
	}

	// 2. Validate Status
	if reservation.Status != domain.ReservationPending {
		// If already paid, idempotent success
		if reservation.Status == domain.ReservationPaid {
			return nil
		}
		return fmt.Errorf("cannot confirm reservation in status: %s", reservation.Status)
	}

	// 3. Update Reservation to PAID
	err = s.reservationRepo.UpdateStatus(ctx, reservationID, domain.ReservationPaid)
	if err != nil {
		return err
	}

	// 4. Update Seat to RESERVED
	// We use the last known version from the repo or just force update if we trust the business flow?
	// Optimistic locking is still good practice.
	// We need to fetch the seat first to get version.
	seat, err := s.seatRepo.GetSeat(ctx, reservation.SeatID)
	if err != nil {
		return err
	}

	// Transition from LOCKED to RESERVED
	// Note: 'reservedBy' should already be set, but we confirm it.
	err = s.seatRepo.UpdateSeatStatus(ctx, reservation.SeatID, domain.SeatReserved, &reservation.UserID, seat.Version)
	if err != nil {
		return fmt.Errorf("failed to finalize seat status: %w", err)
	}

	return nil
}
