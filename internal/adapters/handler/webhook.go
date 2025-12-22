package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// StripeWebhookPayload matches the structure of Stripe event objects
type StripeWebhookPayload struct {
	Type string `json:"type"`
	Data struct {
		Object struct {
			Metadata struct {
				ReservationID string `json:"reservation_id"`
			} `json:"metadata"`
			PaymentStatus string `json:"payment_status"`
		} `json:"object"`
	} `json:"data"`
}

// HandleStripeWebhook godoc
// @Summary Handle Stripe payment events
// @Tags webhooks
// @Accept json
// @Produce json
// @Success 200 {string} string "Received"
// @Router /webhooks/stripe [post]
func (h *HTTPHandler) HandleStripeWebhook(c *gin.Context) {
	var payload StripeWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	// Only handle checkout.session.completed
	if payload.Type == "checkout.session.completed" {
		resIDStr := payload.Data.Object.Metadata.ReservationID
		paymentStatus := payload.Data.Object.PaymentStatus

		if paymentStatus == "paid" {
			// Update reservation status
			resID, _ := uuid.Parse(resIDStr)
			if err := h.bookingService.ConfirmReservation(c.Request.Context(), resID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to confirm reservation"})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "received"})
}
