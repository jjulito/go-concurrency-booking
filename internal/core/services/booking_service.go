package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"reserva/internal/core/domain"
	"reserva/internal/core/ports"
)

type BookingService struct {
	seatRepo        ports.SeatRepository
	reservationRepo ports.ReservationRepository
	eventRepo       ports.EventRepository
	lockRepo        ports.LockRepository
	transactor      ports.Transactor
}

func NewBookingService(
	seatRepo ports.SeatRepository,
	reservationRepo ports.ReservationRepository,
	eventRepo ports.EventRepository,
	lockRepo ports.LockRepository,
	transactor ports.Transactor,
) *BookingService {
	return &BookingService{
		seatRepo:        seatRepo,
		reservationRepo: reservationRepo,
		eventRepo:       eventRepo,
		lockRepo:        lockRepo,
		transactor:      transactor,
	}
}

func (s *BookingService) ListEvents(ctx context.Context) ([]domain.Event, error) {
	return s.eventRepo.ListEvents(ctx)
}

func (s *BookingService) GetEventSeats(ctx context.Context, eventID uuid.UUID) ([]domain.Seat, error) {
	event, err := s.eventRepo.GetEvent(ctx, eventID)
	if err != nil {
		return nil, err // propagates ErrEventNotFound or infra errors as-is
	}
	if event == nil || !event.IsActive {
		return nil, domain.ErrEventNotFound
	}
	return s.seatRepo.GetSeatsByEvent(ctx, eventID)
}

// CreateReservation attempts to reserve a seat with full race-condition protection:
//  1. Redis distributed lock — prevents concurrent processing of the same seat.
//  2. DB transaction — ensures the seat update and reservation insert are atomic;
//     if either fails the other is rolled back, eliminating partial-failure states.
//  3. Optimistic locking — the seat update only succeeds if the version matches,
//     catching any concurrent modification that slipped past the Redis lock.
func (s *BookingService) CreateReservation(ctx context.Context, userID, seatID, eventID uuid.UUID) (*domain.Reservation, error) {
	// 1. Distributed Lock — fail fast if another process is already booking this seat
	lockKey := fmt.Sprintf("lock:seat:%s", seatID.String())
	acquired, token, err := s.lockRepo.AcquireLock(ctx, lockKey, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !acquired {
		return nil, domain.ErrSeatLocked
	}
	defer s.lockRepo.ReleaseLock(ctx, lockKey, token)

	// 2. Atomic transaction: validate, then write seat + reservation together
	var reservation *domain.Reservation
	err = s.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		// Validate the event exists and is currently active
		event, err := s.eventRepo.GetEvent(ctx, eventID)
		if err != nil {
			return err // propagates ErrEventNotFound or infra errors as-is
		}
		if event == nil || !event.IsActive {
			return domain.ErrEventNotFound
		}

		// Fetch the seat for validation and later optimistic version check
		seat, err := s.seatRepo.GetSeat(ctx, seatID)
		if err != nil {
			return fmt.Errorf("failed to get seat: %w", err)
		}

		// Validate the seat actually belongs to the requested event
		if seat.EventID != eventID {
			return domain.ErrSeatUnavailable
		}

		if seat.Status != domain.SeatAvailable {
			return domain.ErrSeatUnavailable
		}

		reservation = domain.NewReservation(userID, seatID, eventID, 5*time.Minute)
		reservation.Amount = seat.Price // price comes from the seat, not hardcoded

		if err := s.seatRepo.UpdateSeatStatus(ctx, seatID, domain.SeatLocked, &userID, seat.Version); err != nil {
			// Version mismatch means concurrent modification despite the lock
			return domain.ErrSeatUnavailable
		}

		if err := s.reservationRepo.CreateReservation(ctx, reservation); err != nil {
			return fmt.Errorf("failed to save reservation: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return reservation, nil
}

func (s *BookingService) GetReservation(ctx context.Context, reservationID uuid.UUID) (*domain.Reservation, error) {
	return s.reservationRepo.GetReservation(ctx, reservationID)
}

// CancelReservation cancels a reservation and releases its seat atomically.
// userID must match the reservation owner; ErrUnauthorized is returned otherwise.
// Both the reservation status update and the seat release happen in a single
// transaction: if either fails, neither is persisted.
func (s *BookingService) CancelReservation(ctx context.Context, reservationID, userID uuid.UUID) error {
	return s.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		reservation, err := s.reservationRepo.GetReservation(ctx, reservationID)
		if err != nil {
			return err
		}

		if reservation.UserID != userID {
			return domain.ErrUnauthorized
		}

		if reservation.Status != domain.ReservationPending {
			return fmt.Errorf("cannot cancel reservation in status %s: %w", reservation.Status, domain.ErrInvalidStatusTransition)
		}

		if err := s.reservationRepo.UpdateStatus(ctx, reservationID, domain.ReservationCancelled); err != nil {
			return err
		}

		seat, err := s.seatRepo.GetSeat(ctx, reservation.SeatID)
		if err != nil {
			return err
		}

		return s.seatRepo.UpdateSeatStatus(ctx, reservation.SeatID, domain.SeatAvailable, nil, seat.Version)
	})
}

// ConfirmReservation finalizes a reservation after payment.
// The reservation status update and seat status update are atomic:
// both succeed or both are rolled back.
func (s *BookingService) ConfirmReservation(ctx context.Context, reservationID uuid.UUID) error {
	return s.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		reservation, err := s.reservationRepo.GetReservation(ctx, reservationID)
		if err != nil {
			return err
		}

		if reservation.Status != domain.ReservationPending {
			if reservation.Status == domain.ReservationPaid {
				return nil // idempotent
			}
			return fmt.Errorf("cannot confirm reservation in status %s: %w", reservation.Status, domain.ErrInvalidStatusTransition)
		}

		// Guard against the race window between expiry and cleanup worker
		if time.Now().After(reservation.ExpiresAt) {
			return fmt.Errorf("reservation has expired: %w", domain.ErrInvalidStatusTransition)
		}

		if err := s.reservationRepo.UpdateStatus(ctx, reservationID, domain.ReservationPaid); err != nil {
			return err
		}

		seat, err := s.seatRepo.GetSeat(ctx, reservation.SeatID)
		if err != nil {
			return err
		}

		return s.seatRepo.UpdateSeatStatus(ctx, reservation.SeatID, domain.SeatReserved, &reservation.UserID, seat.Version)
	})
}
