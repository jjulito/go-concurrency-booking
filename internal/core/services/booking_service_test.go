package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jjulito/reserva/internal/core/domain"
)

// --- Mocks ---

type mockLockRepo struct {
	CapturedKey string
	ShouldFail  bool
	Acquired    bool
}

func (m *mockLockRepo) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	m.CapturedKey = key
	if m.ShouldFail {
		return false, nil // Lock taken
	}
	m.Acquired = true
	return true, nil
}
func (m *mockLockRepo) ReleaseLock(ctx context.Context, key string) error {
	m.Acquired = false
	return nil
}

type mockSeatRepo struct {
	SeatToReturn *domain.Seat
	UpdateErr    error
}

func (m *mockSeatRepo) GetSeat(ctx context.Context, seatID uuid.UUID) (*domain.Seat, error) {
	return m.SeatToReturn, nil
}
func (m *mockSeatRepo) GetSeatsByEvent(ctx context.Context, eventID uuid.UUID) ([]domain.Seat, error) {
	return nil, nil
}
func (m *mockSeatRepo) UpdateSeatStatus(ctx context.Context, seatID uuid.UUID, status domain.SeatStatus, reservedBy *uuid.UUID, version int) error {
	return m.UpdateErr
}

type mockResRepo struct {
	CreatedRes *domain.Reservation
}

func (m *mockResRepo) CreateReservation(ctx context.Context, res *domain.Reservation) error {
	m.CreatedRes = res
	return nil
}
func (m *mockResRepo) GetReservation(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	if m.CreatedRes != nil && m.CreatedRes.ID == id {
		return m.CreatedRes, nil
	}
	return nil, nil
}
func (m *mockResRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ReservationStatus) error {
	if m.CreatedRes != nil && m.CreatedRes.ID == id {
		m.CreatedRes.Status = status
	}
	return nil
}
func (m *mockResRepo) GetExpiredReservations(ctx context.Context) ([]domain.Reservation, error) {
	return nil, nil
}

type mockEventRepo struct{}

func (m *mockEventRepo) GetEvent(ctx context.Context, id uuid.UUID) (*domain.Event, error)   { return nil, nil }
func (m *mockEventRepo) ListEvents(ctx context.Context) ([]domain.Event, error) { return nil, nil }

// --- Tests ---

func TestBookingService_CreateReservation_Success(t *testing.T) {
	// Setup
	seatID := uuid.New()
	userID := uuid.New()
	eventID := uuid.New()

	lockRepo := &mockLockRepo{}
	seatRepo := &mockSeatRepo{
		SeatToReturn: &domain.Seat{
			ID:      seatID,
			Status:  domain.SeatAvailable,
			Version: 1,
		},
	}
	resRepo := &mockResRepo{}
	eventRepo := &mockEventRepo{}

	svc := NewBookingService(seatRepo, resRepo, eventRepo, lockRepo)

	// Execution
	res, err := svc.CreateReservation(context.Background(), userID, seatID, eventID)

	// Verification
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if res == nil {
		t.Fatal("Expected reservation to be created")
	}
	if res.Status != domain.ReservationPending {
		t.Errorf("Expected status PENDING, got %s", res.Status)
	}
	if lockRepo.CapturedKey != "lock:seat:"+seatID.String() {
		t.Errorf("Incorrect lock key: %s", lockRepo.CapturedKey)
	}
}

func TestBookingService_CreateReservation_Locked(t *testing.T) {
	// Setup
	lockRepo := &mockLockRepo{ShouldFail: true} // Simulates lock already taken
	svc := NewBookingService(&mockSeatRepo{}, &mockResRepo{}, &mockEventRepo{}, lockRepo)

	// Execution
	_, err := svc.CreateReservation(context.Background(), uuid.New(), uuid.New(), uuid.New())

	// Verification
	if err != domain.ErrSeatLocked {
		t.Errorf("Expected ErrSeatLocked, got %v", err)
	}
}

func TestBookingService_CreateReservation_SeatUnavailable(t *testing.T) {
	// Setup
	seatRepo := &mockSeatRepo{
		SeatToReturn: &domain.Seat{Status: domain.SeatLocked}, // Seat already locked in DB
	}
	svc := NewBookingService(seatRepo, &mockResRepo{}, &mockEventRepo{}, &mockLockRepo{})

	// Execution
	_, err := svc.CreateReservation(context.Background(), uuid.New(), uuid.New(), uuid.New())

	// Verification
	if err != domain.ErrSeatUnavailable {
		t.Errorf("Expected ErrSeatUnavailable, got %v", err)
	}
}
