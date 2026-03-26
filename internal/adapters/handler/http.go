package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jjulito/reserva/internal/core/domain"
	"github.com/jjulito/reserva/internal/core/ports"
)

type HTTPHandler struct {
	bookingService      ports.BookingService
	stripeWebhookSecret string
}

func NewHTTPHandler(bookingService ports.BookingService, stripeWebhookSecret string) *HTTPHandler {
	return &HTTPHandler{
		bookingService:      bookingService,
		stripeWebhookSecret: stripeWebhookSecret,
	}
}

func (h *HTTPHandler) RegisterRoutes(router *gin.Engine, middlewares ...gin.HandlerFunc) {
	v1 := router.Group("/api/v1")

	// Public routes — no rate limiting (webhook must never be throttled or
	// Stripe will back off and retries will pile up).
	v1.GET("/events", h.ListEvents)
	v1.GET("/events/:id/seats", h.GetEventSeats)
	v1.POST("/webhooks/stripe", h.HandleStripeWebhook)

	// Protected routes — rate limiter + auth middleware applied here only,
	// so public and webhook endpoints are not affected.
	protected := v1.Group("/")
	protected.Use(middlewares...)
	protected.Use(AuthMiddleware())
	{
		protected.GET("/reservations/:id", h.GetReservation)
		protected.POST("/reservations", h.CreateReservation)
		protected.POST("/reservations/:id/cancel", h.CancelReservation)
	}
}

// ListEvents godoc
// @Summary List all active events
// @Tags events
// @Produce json
// @Success 200 {array} domain.Event
// @Router /events [get]
func (h *HTTPHandler) ListEvents(c *gin.Context) {
	events, err := h.bookingService.ListEvents(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}

// GetEventSeats godoc
// @Summary Get seats for an event
// @Tags seats
// @Param id path string true "Event ID"
// @Produce json
// @Success 200 {array} domain.Seat
// @Router /events/{id}/seats [get]
func (h *HTTPHandler) GetEventSeats(c *gin.Context) {
	eventIDStr := c.Param("id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	seats, err := h.bookingService.GetEventSeats(c.Request.Context(), eventID)
	if err != nil {
		if errors.Is(err, domain.ErrEventNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found or no longer active"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, seats)
}

// CreateReservationRequest no longer includes user_id — the authenticated user's
// identity is taken from the X-User-ID header injected by the API gateway.
type CreateReservationRequest struct {
	SeatID  string `json:"seat_id"  binding:"required"`
	EventID string `json:"event_id" binding:"required"`
}

// CreateReservation godoc
// @Summary Attempt to reserve a seat
// @Tags reservations
// @Accept json
// @Produce json
// @Success 201 {object} domain.Reservation
// @Failure 409 {object} map[string]string "Seat unavailable or locked"
// @Router /reservations [post]
func (h *HTTPHandler) CreateReservation(c *gin.Context) {
	userID := c.MustGet(userIDKey).(uuid.UUID)

	var req CreateReservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	seatID, err := uuid.Parse(req.SeatID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid seat_id: must be a UUID"})
		return
	}
	eventID, err := uuid.Parse(req.EventID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event_id: must be a UUID"})
		return
	}

	reservation, err := h.bookingService.CreateReservation(c.Request.Context(), userID, seatID, eventID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrSeatLocked):
			c.JSON(http.StatusConflict, gin.H{"error": "Seat is currently locked by another user"})
		case errors.Is(err, domain.ErrSeatUnavailable):
			c.JSON(http.StatusConflict, gin.H{"error": "Seat is already reserved"})
		case errors.Is(err, domain.ErrSeatNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Seat not found"})
		case errors.Is(err, domain.ErrEventNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found or no longer active"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, reservation)
}

// GetReservation godoc
// @Summary Get reservation details
// @Tags reservations
// @Produce json
// @Param id path string true "Reservation ID"
// @Success 200 {object} domain.Reservation
// @Router /reservations/{id} [get]
func (h *HTTPHandler) GetReservation(c *gin.Context) {
	userID := c.MustGet(userIDKey).(uuid.UUID)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reservation ID"})
		return
	}

	reservation, err := h.bookingService.GetReservation(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrReservationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Reservation not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	if reservation.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not the owner of this reservation"})
		return
	}

	c.JSON(http.StatusOK, reservation)
}

// CancelReservation godoc
// @Summary Cancel a reservation
// @Tags reservations
// @Param id path string true "Reservation ID"
// @Success 200 {object} map[string]string
// @Router /reservations/{id}/cancel [post]
func (h *HTTPHandler) CancelReservation(c *gin.Context) {
	userID := c.MustGet(userIDKey).(uuid.UUID)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reservation ID"})
		return
	}

	err = h.bookingService.CancelReservation(c.Request.Context(), id, userID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrReservationNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Reservation not found"})
		case errors.Is(err, domain.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not the owner of this reservation"})
		case errors.Is(err, domain.ErrInvalidStatusTransition):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reservation cancelled successfully"})
}
