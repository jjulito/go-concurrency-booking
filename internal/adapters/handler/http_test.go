package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jjulito/reserva/internal/core/domain"
)

// --- Mock Service ---
type MockBookingService struct {
	ListEventsFunc        func(ctx context.Context) ([]domain.Event, error)
	GetEventSeatsFunc     func(ctx context.Context, eventID uuid.UUID) ([]domain.Seat, error)
	CreateReservationFunc func(ctx context.Context, userID, seatID, eventID uuid.UUID) (*domain.Reservation, error)
}

func (m *MockBookingService) ListEvents(ctx context.Context) ([]domain.Event, error) {
	if m.ListEventsFunc != nil {
		return m.ListEventsFunc(ctx)
	}
	return nil, nil
}
func (m *MockBookingService) GetEventSeats(ctx context.Context, eventID uuid.UUID) ([]domain.Seat, error) {
	if m.GetEventSeatsFunc != nil {
		return m.GetEventSeatsFunc(ctx, eventID)
	}
	return nil, nil
}
func (m *MockBookingService) CreateReservation(ctx context.Context, userID, seatID, eventID uuid.UUID) (*domain.Reservation, error) {
	if m.CreateReservationFunc != nil {
		return m.CreateReservationFunc(ctx, userID, seatID, eventID)
	}
	return nil, nil
}

// --- Tests ---

func TestHTTPHandler_ListEvents_Success(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	mockSvc := &MockBookingService{
		ListEventsFunc: func(ctx context.Context) ([]domain.Event, error) {
			return []domain.Event{
				{ID: uuid.New(), Name: "Concert A", TotalSeats: 100, IsActive: true},
			}, nil
		},
	}
	h := NewHTTPHandler(mockSvc)
	router := gin.New()
	h.RegisterRoutes(router)

	// Execution
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/events", nil)
	router.ServeHTTP(w, req)

	// Verification
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHTTPHandler_CreateReservation_Success(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	mockSvc := &MockBookingService{
		CreateReservationFunc: func(ctx context.Context, u, s, e uuid.UUID) (*domain.Reservation, error) {
			return domain.NewReservation(u, s, e, 0), nil
		},
	}
	h := NewHTTPHandler(mockSvc)
	router := gin.New()
	h.RegisterRoutes(router)

	payload := map[string]string{
		"user_id":  uuid.New().String(),
		"seat_id":  uuid.New().String(),
		"event_id": uuid.New().String(),
	}
	body, _ := json.Marshal(payload)

	// Execution
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/reservations", bytes.NewBuffer(body))
	router.ServeHTTP(w, req)

	// Verification
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d body: %s", w.Code, w.Body.String())
	}
}

func TestHTTPHandler_CreateReservation_Locked(t *testing.T) {
	// Setup
	mockSvc := &MockBookingService{
		CreateReservationFunc: func(ctx context.Context, u, s, e uuid.UUID) (*domain.Reservation, error) {
			return nil, domain.ErrSeatLocked
		},
	}
	h := NewHTTPHandler(mockSvc)
	router := gin.New()
	h.RegisterRoutes(router)

	payload := map[string]string{
		"user_id":  uuid.New().String(),
		"seat_id":  uuid.New().String(),
		"event_id": uuid.New().String(),
	}
	body, _ := json.Marshal(payload)

	// Execution
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/reservations", bytes.NewBuffer(body))
	router.ServeHTTP(w, req)

	// Verification
	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w.Code)
	}
}
