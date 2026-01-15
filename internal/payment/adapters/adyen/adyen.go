package adyen

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	paymentdomain "github.com/smallbiznis/railzway/internal/payment/domain"
)

type Factory struct{}

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) Provider() string {
	return "adyen"
}

func (f *Factory) NewAdapter(cfg paymentdomain.AdapterConfig) (paymentdomain.PaymentAdapter, error) {
	hmacKey, ok := readString(cfg.Config, "hmac_key")
	if !ok || strings.TrimSpace(hmacKey) == "" {
		return nil, paymentdomain.ErrInvalidConfig
	}

	return &Adapter{
		orgID:   cfg.OrgID,
		hmacKey: strings.TrimSpace(hmacKey),
	}, nil
}

type Adapter struct {
	orgID   snowflake.ID
	hmacKey string
}

// Verify implements the Adyen HMAC signature verification.
// Adyen calculates the HMAC using the payload body directly.
// Reference: https://docs.adyen.com/development-resources/webhooks/verify-hmac-signatures
func (a *Adapter) Verify(ctx context.Context, payload []byte, headers http.Header) error {
	// 1. Get the signature from the notification item (Adyen standard)
	// Note: Adyen sends a specific JSON structure: { "notificationItems": [ ... ] }
	// Verification usually happens PER item if we strictly follow their library pattern,
	// but here we are intercepting the raw HTTP request.
	//
	// However, Adyen usually recommends verifying the signature found IN the payload item
	// (additionalData.hmacSignature). But simpler is often checking header if available,
	// though standard Adyen webhooks rely on payload content.
	//
	// To strictly follow the interface Verify(payload []byte, header), we'll assume
	// standard practice of checking the payload content or header.
	// Adyen does NOT typically sign the whole HTTP body with a header like Stripe (X-Stripe-Signature).
	// Instead, each item in the "notificationItems" array is signed.
	//
	// To comply with our interface, we parse the payload here to find the items and verify them.

	var root adyenNotificationRoot
	if err := json.Unmarshal(payload, &root); err != nil {
		return paymentdomain.ErrInvalidPayload
	}

	if len(root.NotificationItems) == 0 {
		return paymentdomain.ErrInvalidPayload
	}

	// We iterate and verify ALL items. If any one is invalid, we reject.
	for _, item := range root.NotificationItems {
		signature := item.NotificationRequestItem.AdditionalData["hmacSignature"]
		if signature == "" {
			return paymentdomain.ErrInvalidSignature
		}

		if err := a.verifyItemSignature(item.NotificationRequestItem, signature); err != nil {
			return err
		}
	}

	return nil
}

func (a *Adapter) verifyItemSignature(item adyenNotificationRequestItem, expectedSig string) error {
	// Adyen Signature calculation requires concatenating specific fields.
	// Order: pspReference + originalReference + merchantAccountCode + merchantReference + value + currency + eventCode + success
	// We must use the raw values.
	
	// Value is int64 (minor units), need string rep
	valStr := strconv.FormatInt(item.Amount.Value, 10)
	
	parts := []string{
		item.PspReference,
		item.OriginalReference,
		item.MerchantAccountCode,
		item.MerchantReference,
		valStr,
		item.Amount.Currency,
		item.EventCode,
		item.Success,
	}

	// Escape special characters: \ replaced by \\ and : replaced by \:
	// Then join with :
	var sb strings.Builder
	for i, part := range parts {
		replaced := strings.ReplaceAll(part, "\\", "\\\\")
		replaced = strings.ReplaceAll(replaced, ":", "\\:")
		sb.WriteString(replaced)
		if i < len(parts)-1 {
			sb.WriteString(":")
		}
	}
	signingString := sb.String()

	// Calculate HMAC SHA256
	// The key is hex encoded, so decode it first
	keyBytes, err := hex.DecodeString(a.hmacKey)
	if err != nil {
		return paymentdomain.ErrInvalidConfig // Configured key is bad
	}

	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(signingString))
	calculatedSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if calculatedSig != expectedSig {
		return paymentdomain.ErrInvalidSignature
	}
	return nil
}


// Parse extracts the events. Since Adyen sends a batch (notificationItems),
// but our interface likely returns ONE event (Parse -> *PaymentEvent),
// this implies our current architecture handles one event per webhook request.
//
// If Adyen sends multiple, we might drop others or need to redesign.
// For now, we return the FIRST valid item.
func (a *Adapter) Parse(ctx context.Context, payload []byte) (*paymentdomain.PaymentEvent, error) {
	var root adyenNotificationRoot
	if err := json.Unmarshal(payload, &root); err != nil {
		return nil, paymentdomain.ErrInvalidPayload
	}

	if len(root.NotificationItems) == 0 {
		return nil, paymentdomain.ErrInvalidPayload
	}

	// Process the first item
	item := root.NotificationItems[0].NotificationRequestItem

	var eventType string
	
	// Map Event Codes
	// https://docs.adyen.com/development-resources/webhooks/webhook-types
	switch item.EventCode {
	case "AUTHORISATION":
		if item.Success == "true" {
			eventType = paymentdomain.EventTypePaymentSucceeded
		} else {
			eventType = paymentdomain.EventTypePaymentFailed
		}
	case "REFUND":
		if item.Success == "true" {
			eventType = paymentdomain.EventTypeRefunded
		} else {
			// Failed refund, usually we ignore or log.
			// Currently our domain doesn't strictly have a "RefundFailed"
			return nil, paymentdomain.ErrEventIgnored
		}
	case "CANCELLATION":
		if item.Success == "true" {
			eventType = paymentdomain.EventTypePaymentFailed // Cancellation treats payment as failed/void
		} else {
			return nil, paymentdomain.ErrEventIgnored
		}
	case "OFFER_CLOSED":
		eventType = paymentdomain.EventTypePaymentFailed
	default:
		return nil, paymentdomain.ErrEventIgnored
	}

	// Parsing Metadata for CustomerID
	// In Adyen, you usually pass metadata in "merchantReference" (if it's simply an ID) 
	// or "metadata" field if enabled.
	// For this impl, we assume merchantReference holds the InvoiceID or Metadata contains "customer_id"
	
	// Check additionalData for metadata
	customerIDStr := item.AdditionalData["metadata.customer_id"]
	invoiceIDStr := item.AdditionalData["metadata.invoice_id"]
	
	// Fallback: Adyen sometimes prefixes custom fields
	if customerIDStr == "" {
		customerIDStr = item.MerchantReference // If merchantReference IS the customer ID, edge case.
	}

	customerID, err := snowflake.ParseString(customerIDStr)
	if err != nil {
		return nil, paymentdomain.ErrInvalidCustomer
	}

	var invoiceID *snowflake.ID
	if invoiceIDStr != "" {
		id, err := snowflake.ParseString(invoiceIDStr)
		if err == nil {
			invoiceID = &id
		}
	}

	amount := item.Amount.Value // Minor units, already int64 compatible with our domain

	return &paymentdomain.PaymentEvent{
		Provider:          "adyen",
		ProviderEventID:   item.PspReference + "_" + item.EventCode, // Adyen doesn't have a unique "webhook ID", PSP ref is unique per tx
		ProviderPaymentID: item.PspReference,
		ProviderPaymentType: "payment", // Adyen unified
		Type:              eventType,
		OrgID:             a.orgID,
		CustomerID:        customerID,
		Amount:            amount,
		Currency:          strings.ToUpper(item.Amount.Currency),
		OccurredAt:        convertEventDate(item.EventDate),
		RawPayload:        payload,
		InvoiceID:         invoiceID,
	}, nil
}

func convertEventDate(dateStr string) time.Time {
	// Adyen format: 2019-06-28T18:03:50+01:00
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return time.Now().UTC()
	}
	return t.UTC()
}


func readString(config map[string]any, key string) (string, bool) {
	val, ok := config[key]
	if !ok {
		return "", false
	}
	s, ok := val.(string)
	return s, ok
}

// Structs

type adyenNotificationRoot struct {
	NotificationItems []adyenNotificationItem `json:"notificationItems"`
}

type adyenNotificationItem struct {
	NotificationRequestItem adyenNotificationRequestItem `json:"NotificationRequestItem"`
}

type adyenNotificationRequestItem struct {
	AdditionalData      map[string]string `json:"additionalData"`
	Amount              adyenAmount       `json:"amount"`
	EventCode           string            `json:"eventCode"`
	EventDate           string            `json:"eventDate"`
	MerchantAccountCode string            `json:"merchantAccountCode"`
	MerchantReference   string            `json:"merchantReference"`
	OriginalReference   string            `json:"originalReference"`
	PspReference        string            `json:"pspReference"`
	Reason              string            `json:"reason"`
	Success             string            `json:"success"` // "true" or "false"
}

type adyenAmount struct {
	Currency string `json:"currency"`
	Value    int64  `json:"value"`
}
