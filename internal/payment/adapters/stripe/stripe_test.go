package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	paymentdomain "github.com/smallbiznis/valora/internal/payment/domain"
)

func TestVerifySignature(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"id":"evt_123","type":"charge.succeeded","data":{"object":{}}}`)
	timestamp := time.Now().Unix()

	header := buildStripeSignatureHeader(secret, payload, timestamp)
	reqHeader := http.Header{}
	reqHeader.Set("Stripe-Signature", header)

	adapter := &Adapter{orgID: 1, webhookSecret: secret}
	if err := adapter.Verify(context.Background(), payload, reqHeader); err != nil {
		t.Fatalf("expected valid signature, got error: %v", err)
	}

	reqHeader.Set("Stripe-Signature", buildStripeSignatureHeader("wrong", payload, timestamp))
	if err := adapter.Verify(context.Background(), payload, reqHeader); err == nil {
		t.Fatalf("expected invalid signature error")
	}
}

func TestParsePaymentEvent(t *testing.T) {
	node, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatalf("new node: %v", err)
	}
	customerID := node.Generate().String()
	invoiceID := node.Generate().String()
	created := time.Now().UTC().Unix()

	tests := []struct {
		name     string
		event    any
		wantType string
		amount   int64
	}{{
		name: "payment_intent.succeeded",
		event: map[string]any{
			"id":      "evt_pi",
			"type":    "payment_intent.succeeded",
			"created": created,
			"data": map[string]any{
				"object": map[string]any{
					"id":              "pi_1",
					"amount":          2500,
					"amount_received": 2500,
					"currency":        "usd",
					"created":         created,
					"metadata": map[string]any{
						"customer_id": customerID,
						"invoice_id":  invoiceID,
					},
				},
			},
		},
		wantType: paymentdomain.EventTypePaymentSucceeded,
		amount:   2500,
	}, {
		name: "charge.refunded",
		event: map[string]any{
			"id":      "evt_charge",
			"type":    "charge.refunded",
			"created": created,
			"data": map[string]any{
				"object": map[string]any{
					"id":              "ch_1",
					"amount":          5000,
					"amount_refunded": 1200,
					"currency":        "usd",
					"created":         created,
					"metadata": map[string]any{
						"customer_id": customerID,
					},
				},
			},
		},
		wantType: paymentdomain.EventTypeRefunded,
		amount:   1200,
	}}

	adapter := &Adapter{orgID: 1, webhookSecret: "whsec_test"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}
			event, err := adapter.Parse(context.Background(), payload)
			if err != nil {
				t.Fatalf("parse event: %v", err)
			}
			if event.Type != tt.wantType {
				t.Fatalf("expected type %s, got %s", tt.wantType, event.Type)
			}
			if event.Amount != tt.amount {
				t.Fatalf("expected amount %d, got %d", tt.amount, event.Amount)
			}
			if event.CustomerID == 0 {
				t.Fatalf("expected customer ID")
			}
			if event.Currency != "USD" {
				t.Fatalf("expected currency USD, got %s", event.Currency)
			}
		})
	}
}

func buildStripeSignatureHeader(secret string, payload []byte, timestamp int64) string {
	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signedPayload))
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
}
