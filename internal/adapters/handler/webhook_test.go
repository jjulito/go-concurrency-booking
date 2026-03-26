package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

// computeSignature builds a valid Stripe v1 HMAC-SHA256 signature for testing.
func computeSignature(t *testing.T, payload []byte, secret string, ts int64) string {
	t.Helper()
	signed := fmt.Sprintf("%d.%s", ts, string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signed))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyStripeSignature_Valid(t *testing.T) {
	secret := "whsec_test_secret"
	payload := []byte(`{"type":"checkout.session.completed"}`)
	ts := time.Now().Unix()
	sig := computeSignature(t, payload, secret, ts)
	header := fmt.Sprintf("t=%d,v1=%s", ts, sig)

	if err := verifyStripeSignature(payload, header, secret); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestVerifyStripeSignature_MissingHeader(t *testing.T) {
	err := verifyStripeSignature([]byte("body"), "", "secret")
	if err == nil {
		t.Fatal("expected error for empty signature header")
	}
}

func TestVerifyStripeSignature_MissingSecret(t *testing.T) {
	err := verifyStripeSignature([]byte("body"), "t=123,v1=abc", "")
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestVerifyStripeSignature_InvalidHeaderFormat(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"no t= or v1=", "garbage"},
		{"missing v1", "t=123456"},
		{"missing t", "v1=abcdef"},
		{"empty values", "t=,v1="},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyStripeSignature([]byte("body"), tc.header, "secret")
			if err == nil {
				t.Errorf("expected error for header %q", tc.header)
			}
		})
	}
}

func TestVerifyStripeSignature_InvalidTimestamp(t *testing.T) {
	err := verifyStripeSignature([]byte("body"), "t=not_a_number,v1=abc", "secret")
	if err == nil {
		t.Fatal("expected error for non-numeric timestamp")
	}
}

func TestVerifyStripeSignature_StaleTimestamp(t *testing.T) {
	secret := "whsec_test_secret"
	payload := []byte(`{"type":"checkout.session.completed"}`)
	staleTS := time.Now().Add(-10 * time.Minute).Unix()
	sig := computeSignature(t, payload, secret, staleTS)
	header := fmt.Sprintf("t=%d,v1=%s", staleTS, sig)

	err := verifyStripeSignature(payload, header, secret)
	if err == nil {
		t.Fatal("expected error for stale timestamp (>5 min old)")
	}
}

func TestVerifyStripeSignature_WrongSignature(t *testing.T) {
	secret := "whsec_test_secret"
	payload := []byte(`{"type":"checkout.session.completed"}`)
	ts := time.Now().Unix()
	// Compute signature with a different secret
	wrongSig := computeSignature(t, payload, "wrong_secret", ts)
	header := fmt.Sprintf("t=%d,v1=%s", ts, wrongSig)

	err := verifyStripeSignature(payload, header, secret)
	if err == nil {
		t.Fatal("expected error for signature computed with wrong secret")
	}
}

func TestVerifyStripeSignature_TamperedPayload(t *testing.T) {
	secret := "whsec_test_secret"
	original := []byte(`{"type":"checkout.session.completed"}`)
	ts := time.Now().Unix()
	sig := computeSignature(t, original, secret, ts)
	header := fmt.Sprintf("t=%d,v1=%s", ts, sig)

	tampered := []byte(`{"type":"checkout.session.completed","hacked":true}`)
	err := verifyStripeSignature(tampered, header, secret)
	if err == nil {
		t.Fatal("expected error for tampered payload")
	}
}
