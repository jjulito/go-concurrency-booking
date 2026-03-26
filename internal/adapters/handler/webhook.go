package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jjulito/reserva/internal/core/domain"
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
	// Read raw body first — signature verification requires the exact bytes Stripe sent.
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Verify the Stripe-Signature header before trusting any payload content.
	sigHeader := c.GetHeader("Stripe-Signature")
	if err := verifyStripeSignature(rawBody, sigHeader, h.stripeWebhookSecret); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook signature"})
		return
	}

	var payload StripeWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	if payload.Type == "checkout.session.completed" {
		resIDStr := payload.Data.Object.Metadata.ReservationID
		paymentStatus := payload.Data.Object.PaymentStatus

		if paymentStatus == "paid" {
			resID, err := uuid.Parse(resIDStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reservation_id in webhook metadata"})
				return
			}

			if err := h.bookingService.ConfirmReservation(c.Request.Context(), resID); err != nil {
				// Business errors (expired reservation, wrong state, already processed) must
				// return 2xx so Stripe does NOT retry the webhook. Retrying won't help and
				// Stripe will keep sending the event for up to 3 days otherwise.
				if errors.Is(err, domain.ErrInvalidStatusTransition) ||
					errors.Is(err, domain.ErrReservationNotFound) {
					c.JSON(http.StatusOK, gin.H{"status": "already_processed"})
					return
				}
				// Infrastructure errors (DB down, etc.) return 500 so Stripe retries later.
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to confirm reservation"})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "received"})
}

// verifyStripeSignature validates the Stripe-Signature header using HMAC-SHA256.
//
// Stripe format: "t=<unix_timestamp>,v1=<hex_signature>[,v1=<hex_signature>...]"
//
// Stripe may include multiple v1 signatures during webhook secret rotation.
// The event is accepted if any v1 value matches the computed signature.
//
// Steps:
//  1. Parse timestamp (t) and all v1 signatures from the header.
//  2. Reject events older than 5 minutes to prevent replay attacks.
//  3. Compute HMAC-SHA256(secret, "<t>.<rawBody>") and accept if any v1 matches.
func verifyStripeSignature(payload []byte, sigHeader, secret string) error {
	if sigHeader == "" || secret == "" {
		return fmt.Errorf("missing Stripe-Signature header or webhook secret")
	}

	var timestamp string
	var v1Sigs []string
	for _, part := range strings.Split(sigHeader, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.TrimSpace(kv[0]) {
		case "t":
			timestamp = strings.TrimSpace(kv[1])
		case "v1":
			v1Sigs = append(v1Sigs, strings.TrimSpace(kv[1]))
		}
	}

	if timestamp == "" || len(v1Sigs) == 0 {
		return fmt.Errorf("invalid Stripe-Signature header format")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp in Stripe-Signature header")
	}

	// Reject events outside a 5-minute window (replay attack prevention)
	eventTime := time.Unix(ts, 0)
	now := time.Now()
	delta := now.Sub(eventTime)
	if delta > 5*time.Minute || delta < -5*time.Minute {
		return fmt.Errorf("webhook timestamp outside allowed window: possible replay attack")
	}

	// signed_payload = timestamp + "." + raw_body
	signedPayload := fmt.Sprintf("%s.%s", timestamp, string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	expected := hex.EncodeToString(mac.Sum(nil))

	// Accept if any v1 signature matches (supports secret rotation)
	for _, v1Sig := range v1Sigs {
		if hmac.Equal([]byte(expected), []byte(v1Sig)) {
			return nil
		}
	}

	return fmt.Errorf("webhook signature mismatch")
}
