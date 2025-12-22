package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewReservation(t *testing.T) {
	// Setup
	userID := uuid.New()
	seatID := uuid.New()
	eventID := uuid.New()
	duration := 5 * time.Minute

	// Execution
	res := NewReservation(userID, seatID, eventID, duration)

	// Verification
	if res.ID == uuid.Nil {
		t.Error("Expected reservation ID to be set")
	}
	if res.UserID != userID {
		t.Errorf("Expected userID %v, got %v", userID, res.UserID)
	}
	if res.SeatID != seatID {
		t.Errorf("Expected seatID %v, got %v", seatID, res.SeatID)
	}
	if res.EventID != eventID {
		t.Errorf("Expected eventID %v, got %v", eventID, res.EventID)
	}
	if res.Status != ReservationPending {
		t.Errorf("Expected status %s, got %s", ReservationPending, res.Status)
	}
	if res.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
	// Check ExpiresAt covers roughly the duration (allow small delta for execution time)
	expectedExpires := res.CreatedAt.Add(duration)
	diff := expectedExpires.Sub(res.ExpiresAt)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("Expected ExpiresAt to be roughly %v, got %v", expectedExpires, res.ExpiresAt)
	}
}
