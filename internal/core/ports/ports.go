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
	// AcquireLock returns (acquired, token, error). The token must be passed to ReleaseLock
	// to prevent releasing a lock owned by another process.
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (acquired bool, token string, err error)
	ReleaseLock(ctx context.Context, key, token string) error
}

// Transactor executes a function within a database transaction.
// If fn returns an error the transaction is rolled back; otherwise it is committed.
type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// BookingService defines the business logic
type BookingService interface {
	ListEvents(ctx context.Context) ([]domain.Event, error)
	GetEventSeats(ctx context.Context, eventID uuid.UUID) ([]domain.Seat, error)
	CreateReservation(ctx context.Context, userID, seatID, eventID uuid.UUID) (*domain.Reservation, error)
	GetReservation(ctx context.Context, reservationID uuid.UUID) (*domain.Reservation, error)
	// CancelReservation cancels a pending reservation. userID is the authenticated
	// caller; the service verifies they own the reservation before cancelling.
	CancelReservation(ctx context.Context, reservationID, userID uuid.UUID) error
	ConfirmReservation(ctx context.Context, reservationID uuid.UUID) error
}

// BackgroundWorker defines interface for background tasks
type BackgroundWorker interface {
	Start(ctx context.Context)
}
