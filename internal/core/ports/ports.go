package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jjulito/reserva/internal/core/domain"
)

// SeatRepository defines storage operations for seats
type SeatRepository interface {
	GetSeat(ctx context.Context, seatID uuid.UUID) (*domain.Seat, error)
	GetSeatsByEvent(ctx context.Context, eventID uuid.UUID) ([]domain.Seat, error)
	// UpdateSeatStatus updates the status with optimistic locking (checking version)
	UpdateSeatStatus(ctx context.Context, seatID uuid.UUID, status domain.SeatStatus, reservedBy *uuid.UUID, currentVersion int) error
}

// ReservationRepository defines storage operations for reservations
type ReservationRepository interface {
	CreateReservation(ctx context.Context, reservation *domain.Reservation) error
	GetReservation(ctx context.Context, reservationID uuid.UUID) (*domain.Reservation, error)
	UpdateStatus(ctx context.Context, reservationID uuid.UUID, status domain.ReservationStatus) error
	GetExpiredReservations(ctx context.Context) ([]domain.Reservation, error)
}

// EventRepository defines storage operations for events
type EventRepository interface {
	GetEvent(ctx context.Context, eventID uuid.UUID) (*domain.Event, error)
	ListEvents(ctx context.Context) ([]domain.Event, error)
}

// LockRepository defines distributed locking operations (Redis)
type LockRepository interface {
	// AcquireLock returns true if lock is acquired, false otherwise
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string) error
}

// BookingService defines the business logic
type BookingService interface {
	ListEvents(ctx context.Context) ([]domain.Event, error)
	GetEventSeats(ctx context.Context, eventID uuid.UUID) ([]domain.Seat, error)
	CreateReservation(ctx context.Context, userID, seatID, eventID uuid.UUID) (*domain.Reservation, error)
	GetReservation(ctx context.Context, reservationID uuid.UUID) (*domain.Reservation, error)
	CancelReservation(ctx context.Context, reservationID uuid.UUID) error
	ConfirmReservation(ctx context.Context, reservationID uuid.UUID) error
}

// BackgroundWorker defines interface for background tasks
type BackgroundWorker interface {
	Start(ctx context.Context)
}
