package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jjulito/reserva/internal/core/domain"
	"github.com/jjulito/reserva/internal/core/ports"
)

type HTTPHandler struct {
	bookingService ports.BookingService
}

func NewHTTPHandler(bookingService ports.BookingService) *HTTPHandler {
	return &HTTPHandler{bookingService: bookingService}
}

func (h *HTTPHandler) RegisterRoutes(router *gin.Engine) {
	v1 := router.Group("/api/v1")
	{
		v1.GET("/events", h.ListEvents)
		v1.GET("/events/:id/seats", h.GetEventSeats)
		v1.POST("/reservations", h.CreateReservation)
		v1.GET("/reservations/:id", h.GetReservation)
		v1.POST("/reservations/:id/cancel", h.CancelReservation)
		v1.POST("/webhooks/stripe", h.HandleStripeWebhook)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, seats)
}

type CreateReservationRequest struct {
	UserID  string `json:"user_id" binding:"required"`
	SeatID  string `json:"seat_id" binding:"required"`
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
	var req CreateReservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := uuid.Parse(req.UserID)
	seatID, _ := uuid.Parse(req.SeatID)
	eventID, _ := uuid.Parse(req.EventID)

	reservation, err := h.bookingService.CreateReservation(c.Request.Context(), userID, seatID, eventID)
	if err != nil {
		switch err {
		case domain.ErrSeatLocked:
			c.JSON(http.StatusConflict, gin.H{"error": "Seat is currently locked by another user"})
		case domain.ErrSeatUnavailable:
			c.JSON(http.StatusConflict, gin.H{"error": "Seat is already reserved"})
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
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reservation ID"})
		return
	}

	reservation, err := h.bookingService.GetReservation(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Reservation not found"})
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
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reservation ID"})
		return
	}

	err = h.bookingService.CancelReservation(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reservation cancelled successfully"})
}
