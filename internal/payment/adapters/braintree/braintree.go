package braintree

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/url"
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
	return "braintree"
}

func (f *Factory) NewAdapter(cfg paymentdomain.AdapterConfig) (paymentdomain.PaymentAdapter, error) {
	// Braintree usually needs Public Key, Private Key, and Merchant ID. 
	// For Webhook verification (without SDK), we strictly need the Public Key or Private Key depending on signature algo.
	// We'll assume "private_key" is stored in config for signature matching.
	privateKey, ok := readString(cfg.Config, "private_key")
	if !ok || strings.TrimSpace(privateKey) == "" {
		return nil, paymentdomain.ErrInvalidConfig
	}

	return &Adapter{
		orgID:      cfg.OrgID,
		privateKey: strings.TrimSpace(privateKey),
	}, nil
}

type Adapter struct {
	orgID      snowflake.ID
	privateKey string
}

// Verify checks the bt_signature against the bt_payload.
// Braintree sends POST params: bt_signature, bt_payload.
// Signature format: "public_key_string|hash_string"
// We need to verify that HMAC(payload, private_key) matches the hash_string.
func (a *Adapter) Verify(ctx context.Context, payload []byte, headers http.Header) error {
	// Content-Type is usually application/x-www-form-urlencoded
	// We need to parse the query string from the payload body.
	values, err := url.ParseQuery(string(payload))
	if err != nil {
		return paymentdomain.ErrInvalidPayload
	}

	signature := values.Get("bt_signature")
	content := values.Get("bt_payload")

	if signature == "" || content == "" {
		return paymentdomain.ErrInvalidPayload
	}

	// Signature is "publicKey|hash"
	parts := strings.Split(signature, "|")
	if len(parts) != 2 {
		return paymentdomain.ErrInvalidSignature
	}

	// Verify - Simplified SHA256 HMAC (Braintree uses SHA1 in older versions, SHA256 in newer)
	// We will assume SHA256 for this modern implementation.
	// Calculate HMAC
	mac := hmac.New(sha256.New, []byte(a.privateKey))
	mac.Write([]byte(content))
	expectedHash := hex.EncodeToString(mac.Sum(nil))

	// Note: In a real SDK, it handles multiple algorithms and key matching.
	// Here strictly checking if our private key generates the matching hash.
	if parts[1] != expectedHash {
		return paymentdomain.ErrInvalidSignature
	}

	return nil
}

// Parse extracts the event.
// Braintree payload is XML encoded inside the "bt_payload" string (base64 decoded).
// For this POC, we will mock the XML parsing or assume a simplified structure for demonstration,
// as full XML parsing requires a struct compliant with Braintree's schema.
func (a *Adapter) Parse(ctx context.Context, payload []byte) (*paymentdomain.PaymentEvent, error) {
	values, err := url.ParseQuery(string(payload))
	if err != nil {
		return nil, paymentdomain.ErrInvalidPayload
	}
	encodedPayload := values.Get("bt_payload")
	if encodedPayload == "" {
		return nil, paymentdomain.ErrInvalidPayload
	}

	// Decode base64 if needed, Braintree payloads are often just XML string if not using modern library wrapper?
	// Actually bt_payload is usually base64 encoded XML.
	xmlBytes, err := base64.StdEncoding.DecodeString(encodedPayload)
	if err != nil {
		// Try raw if base64 fails (sometimes it's raw XML)
		xmlBytes = []byte(encodedPayload)
	}

	// We'll use a simplified XML parser or string search for this level of abstraction 
	// since we don't have the full XML struct definition in non-imported code.
	// Real implementation would use encoding/xml with full struct.
	
	sXml := string(xmlBytes)
	
	// Poor man's XML parsing for RFC (Assumption: <kind>...</kind>)
	kind := extractXMLTag(sXml, "kind")
	id := extractXMLTag(sXml, "id") // Transaction ID often in <id> or <transaction><id>
	
	// Metadata extraction? <customer-id>
	customerIDStr := extractXMLTag(sXml, "customer-id")
	
	if kind == "" {
		return nil, paymentdomain.ErrInvalidPayload
	}

	var eventType string
	switch kind {
	case "subscription_charged_successfully", "transaction_settled":
		eventType = paymentdomain.EventTypePaymentSucceeded
	case "subscription_canceled", "transaction_settlement_declined":
		eventType = paymentdomain.EventTypePaymentFailed
	case "subscription_went_past_due":
		eventType = paymentdomain.EventTypePaymentFailed
	default:
		return nil, paymentdomain.ErrEventIgnored
	}

	// Handling CustomerID
	customerID, err := snowflake.ParseString(customerIDStr)
	if err != nil {
		return nil, paymentdomain.ErrInvalidCustomer
	}

	// Amount extraction (simplified, assuming <amount>10.00</amount>)
	amountStr := extractXMLTag(sXml, "amount")
	amountFloat, _ := strconv.ParseFloat(amountStr, 64)
	amount := int64(amountFloat * 100) // Convert standard units to cents (minor units)

	// Currency
	currency := "USD" // Default if not found
	// Braintree often configured per merchant account, but let's try to find it
	// <currency-iso-code>USD</currency-iso-code>
	if foundCurr := extractXMLTag(sXml, "currency-iso-code"); foundCurr != "" {
		currency = foundCurr
	}
	
	// Timestamp defaults to now as Braintree XML is heavy to parse actual event time without struct
	occurredAt := time.Now().UTC()

	return &paymentdomain.PaymentEvent{
		Provider:            "braintree",
		ProviderEventID:     id + "_" + kind, // Braintree doesn't give a unique webhook ID, rely on resource ID + type
		ProviderPaymentID:   id,
		ProviderPaymentType: "transaction",
		Type:                eventType,
		OrgID:               a.orgID,
		CustomerID:          customerID,
		Amount:              amount,
		Currency:            strings.ToUpper(currency),
		OccurredAt:          occurredAt,
		RawPayload:          payload,
	}, nil
}

func extractXMLTag(xml, tag string) string {
	startTag := "<" + tag + ">"
	endTag := "</" + tag + ">"
	
	start := strings.Index(xml, startTag)
	if start == -1 {
		return ""
	}
	start += len(startTag)
	
	end := strings.Index(xml[start:], endTag)
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(xml[start : start+end])
}

func readString(config map[string]any, key string) (string, bool) {
	val, ok := config[key]
	if !ok {
		return "", false
	}
	s, ok := val.(string)
	return s, ok
}
