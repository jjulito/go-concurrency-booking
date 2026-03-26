package services

import (
	"context"
	"errors"
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

func (m *mockLockRepo) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, string, error) {
	m.CapturedKey = key
	if m.ShouldFail {
		return false, "", nil // Lock taken by another process
	}
	m.Acquired = true
	return true, "test-token", nil
}

func (m *mockLockRepo) ReleaseLock(ctx context.Context, key, token string) error {
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
	CreatedRes      *domain.Reservation
	UpdateStatusErr error
}

func (m *mockResRepo) CreateReservation(ctx context.Context, res *domain.Reservation) error {
	m.CreatedRes = res
	return nil
}
func (m *mockResRepo) GetReservation(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	if m.CreatedRes != nil && m.CreatedRes.ID == id {
		return m.CreatedRes, nil
	}
	return nil, domain.ErrReservationNotFound
}
func (m *mockResRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ReservationStatus) error {
	if m.UpdateStatusErr != nil {
		return m.UpdateStatusErr
	}
	if m.CreatedRes != nil && m.CreatedRes.ID == id {
		m.CreatedRes.Status = status
	}
	return nil
}
func (m *mockResRepo) GetExpiredReservations(ctx context.Context) ([]domain.Reservation, error) {
	return nil, nil
}

type mockEventRepo struct{}

func (m *mockEventRepo) GetEvent(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	return &domain.Event{ID: id, IsActive: true}, nil
}
func (m *mockEventRepo) ListEvents(ctx context.Context) ([]domain.Event, error) { return nil, nil }

// mockTransactor executes fn directly without a real DB transaction.
type mockTransactor struct{}

func (m *mockTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// --- Tests ---

func TestBookingService_CreateReservation_Success(t *testing.T) {
	seatID := uuid.New()
	userID := uuid.New()
	eventID := uuid.New()

	lockRepo := &mockLockRepo{}
	seatRepo := &mockSeatRepo{
		SeatToReturn: &domain.Seat{
			ID:      seatID,
			EventID: eventID, // must match for the seat-event ownership check
			Status:  domain.SeatAvailable,
			Version: 1,
		},
	}
	resRepo := &mockResRepo{}
	eventRepo := &mockEventRepo{}
	transactor := &mockTransactor{}

	svc := NewBookingService(seatRepo, resRepo, eventRepo, lockRepo, transactor)

	res, err := svc.CreateReservation(context.Background(), userID, seatID, eventID)

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
	lockRepo := &mockLockRepo{ShouldFail: true}
	svc := NewBookingService(&mockSeatRepo{}, &mockResRepo{}, &mockEventRepo{}, lockRepo, &mockTransactor{})

	_, err := svc.CreateReservation(context.Background(), uuid.New(), uuid.New(), uuid.New())

	if !errors.Is(err, domain.ErrSeatLocked) {
		t.Errorf("Expected ErrSeatLocked, got %v", err)
	}
}

func TestBookingService_CreateReservation_SeatUnavailable(t *testing.T) {
	eventID := uuid.New()
	seatID := uuid.New()
	seatRepo := &mockSeatRepo{
		// EventID matches so the ownership check passes — the seat STATUS is what should fail
		SeatToReturn: &domain.Seat{
			ID:      seatID,
			EventID: eventID,
			Status:  domain.SeatLocked, // not AVAILABLE → ErrSeatUnavailable
		},
	}
	svc := NewBookingService(seatRepo, &mockResRepo{}, &mockEventRepo{}, &mockLockRepo{}, &mockTransactor{})

	_, err := svc.CreateReservation(context.Background(), uuid.New(), seatID, eventID)

	if !errors.Is(err, domain.ErrSeatUnavailable) {
		t.Errorf("Expected ErrSeatUnavailable, got %v", err)
	}
}

// --- CancelReservation tests ---

func TestBookingService_CancelReservation_Success(t *testing.T) {
	userID := uuid.New()
	seatID := uuid.New()
	eventID := uuid.New()
	res := &domain.Reservation{
		ID:        uuid.New(),
		UserID:    userID,
		SeatID:    seatID,
		EventID:   eventID,
		Status:    domain.ReservationPending,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	resRepo := &mockResRepo{CreatedRes: res}
	seatRepo := &mockSeatRepo{SeatToReturn: &domain.Seat{ID: seatID, EventID: eventID, Status: domain.SeatLocked, Version: 1}}
	svc := NewBookingService(seatRepo, resRepo, &mockEventRepo{}, &mockLockRepo{}, &mockTransactor{})

	err := svc.CancelReservation(context.Background(), res.ID, userID)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if resRepo.CreatedRes.Status != domain.ReservationCancelled {
		t.Errorf("Expected status CANCELLED, got %s", resRepo.CreatedRes.Status)
	}
}

func TestBookingService_CancelReservation_Unauthorized(t *testing.T) {
	ownerID := uuid.New()
	otherUserID := uuid.New()
	res := &domain.Reservation{
		ID:        uuid.New(),
		UserID:    ownerID,
		Status:    domain.ReservationPending,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	resRepo := &mockResRepo{CreatedRes: res}
	svc := NewBookingService(&mockSeatRepo{}, resRepo, &mockEventRepo{}, &mockLockRepo{}, &mockTransactor{})

	err := svc.CancelReservation(context.Background(), res.ID, otherUserID)

	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("Expected ErrUnauthorized, got %v", err)
	}
}

func TestBookingService_CancelReservation_NonPending(t *testing.T) {
	userID := uuid.New()
	res := &domain.Reservation{
		ID:        uuid.New(),
		UserID:    userID,
		Status:    domain.ReservationPaid, // not PENDING
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	resRepo := &mockResRepo{CreatedRes: res}
	svc := NewBookingService(&mockSeatRepo{}, resRepo, &mockEventRepo{}, &mockLockRepo{}, &mockTransactor{})

	err := svc.CancelReservation(context.Background(), res.ID, userID)

	if !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Errorf("Expected ErrInvalidStatusTransition, got %v", err)
	}
}

// --- ConfirmReservation tests ---

func TestBookingService_ConfirmReservation_Success(t *testing.T) {
	userID := uuid.New()
	seatID := uuid.New()
	eventID := uuid.New()
	res := &domain.Reservation{
		ID:        uuid.New(),
		UserID:    userID,
		SeatID:    seatID,
		EventID:   eventID,
		Status:    domain.ReservationPending,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	resRepo := &mockResRepo{CreatedRes: res}
	seatRepo := &mockSeatRepo{SeatToReturn: &domain.Seat{ID: seatID, EventID: eventID, Status: domain.SeatLocked, Version: 1}}
	svc := NewBookingService(seatRepo, resRepo, &mockEventRepo{}, &mockLockRepo{}, &mockTransactor{})

	err := svc.ConfirmReservation(context.Background(), res.ID)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if resRepo.CreatedRes.Status != domain.ReservationPaid {
		t.Errorf("Expected status PAID, got %s", resRepo.CreatedRes.Status)
	}
}

func TestBookingService_ConfirmReservation_AlreadyPaid(t *testing.T) {
	res := &domain.Reservation{
		ID:        uuid.New(),
		Status:    domain.ReservationPaid, // already PAID — should be idempotent
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	resRepo := &mockResRepo{CreatedRes: res}
	svc := NewBookingService(&mockSeatRepo{}, resRepo, &mockEventRepo{}, &mockLockRepo{}, &mockTransactor{})

	err := svc.ConfirmReservation(context.Background(), res.ID)

	if err != nil {
		t.Errorf("Expected no error for already-PAID reservation (idempotent), got %v", err)
	}
}

func TestBookingService_ConfirmReservation_Expired(t *testing.T) {
	res := &domain.Reservation{
		ID:        uuid.New(),
		Status:    domain.ReservationPending,
		ExpiresAt: time.Now().Add(-1 * time.Minute), // already expired
	}
	resRepo := &mockResRepo{CreatedRes: res}
	svc := NewBookingService(&mockSeatRepo{}, resRepo, &mockEventRepo{}, &mockLockRepo{}, &mockTransactor{})

	err := svc.ConfirmReservation(context.Background(), res.ID)

	if !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Errorf("Expected ErrInvalidStatusTransition for expired reservation, got %v", err)
	}
}

func TestBookingService_ConfirmReservation_SeatUpdateFails(t *testing.T) {
	userID := uuid.New()
	seatID := uuid.New()
	eventID := uuid.New()
	res := &domain.Reservation{
		ID:        uuid.New(),
		UserID:    userID,
		SeatID:    seatID,
		EventID:   eventID,
		Status:    domain.ReservationPending,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	resRepo := &mockResRepo{CreatedRes: res}
	seatRepo := &mockSeatRepo{
		SeatToReturn: &domain.Seat{ID: seatID, EventID: eventID, Status: domain.SeatLocked, Version: 1},
		UpdateErr:    domain.ErrSeatUnavailable, // seat update fails
	}
	svc := NewBookingService(seatRepo, resRepo, &mockEventRepo{}, &mockLockRepo{}, &mockTransactor{})

	err := svc.ConfirmReservation(context.Background(), res.ID)

	if err == nil {
		t.Error("Expected error when seat update fails, got nil")
	}
}
