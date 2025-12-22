package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Status Enums
type SeatStatus string
type ReservationStatus string

const (
	SeatAvailable SeatStatus = "AVAILABLE"
	SeatLocked    SeatStatus = "LOCKED"
	SeatReserved  SeatStatus = "RESERVED"

	ReservationPending   ReservationStatus = "PENDING"
	ReservationPaid      ReservationStatus = "PAID"
	ReservationCancelled ReservationStatus = "CANCELLED"
)

var (
	ErrSeatUnavailable = errors.New("seat is not available")
	ErrSeatLocked      = errors.New("seat is currently locked")
	ErrEventNotFound   = errors.New("event not found")
)

// User entity
type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// Event entity
type Event struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	TotalSeats int       `json:"total_seats"`
	IsActive   bool      `json:"is_active"`
}

// Seat entity
type Seat struct {
	ID         uuid.UUID  `json:"id"`
	EventID    uuid.UUID  `json:"event_id"`
	Number     string     `json:"seat_number"`
	Category   string     `json:"category"`
	Status     SeatStatus `json:"status"`
	Version    int        `json:"version"` // Optimistic Locking
	ReservedBy *uuid.UUID `json:"reserved_by,omitempty"`
}

// Reservation entity
type Reservation struct {
	ID        uuid.UUID         `json:"id"`
	UserID    uuid.UUID         `json:"user_id"`
	SeatID    uuid.UUID         `json:"seat_id"`
	EventID   uuid.UUID         `json:"event_id"`
	Status    ReservationStatus `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
	ExpiresAt time.Time         `json:"expires_at"` // e.g., 5 minutes from creation
}

func NewReservation(userID, seatID, eventID uuid.UUID, duration time.Duration) *Reservation {
	return &Reservation{
		ID:        uuid.New(),
		UserID:    userID,
		SeatID:    seatID,
		EventID:   eventID,
		Status:    ReservationPending,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(duration),
	}
}
