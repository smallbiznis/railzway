package events

// Billing event types for snapshot rollups.
const (
	EventLedgerEntryCreated = "ledger_entry_created"
	EventInvoiceFinalized   = "invoice_finalized"
	EventInvoiceVoided      = "invoice_voided"
	EventPaymentSettled     = "payment_settled"
	EventRefundSettled      = "refund_settled"
	EventDisputeWithdrawn   = "dispute_withdrawn"
	EventDisputeReinstated  = "dispute_reinstated"
	EventUsageIngested      = "usage.ingested"
)

// LedgerEntryPayload captures the minimal data needed to roll up a ledger entry.
type LedgerEntryPayload struct {
	LedgerEntryID string `json:"ledger_entry_id"`
	SourceType    string `json:"source_type,omitempty"`
	SourceID      string `json:"source_id,omitempty"`
}

// InvoicePayload captures the minimal data needed to roll up invoice events.
type InvoicePayload struct {
	InvoiceID      string `json:"invoice_id"`
	BillingCycleID string `json:"billing_cycle_id"`
}

// UsageIngestedPayload captures the minimal data needed to kick off async usage processing.
type UsageIngestedPayload struct {
	UsageEventID       string  `json:"usage_event_id"`
	OrgID              string  `json:"org_id"`
	CustomerID         string  `json:"customer_id"`
	SubscriptionID     string  `json:"subscription_id,omitempty"`
	SubscriptionItemID string  `json:"subscription_item_id,omitempty"`
	MeterID            string  `json:"meter_id,omitempty"`
	MeterCode          string  `json:"meter_code,omitempty"`
	IdempotencyKey     *string `json:"idempotency_key,omitempty"`
}

// ToMap converts a payload into an outbox-friendly map.
func (p UsageIngestedPayload) ToMap() map[string]any {
	payload := map[string]any{
		"usage_event_id": p.UsageEventID,
		"org_id":         p.OrgID,
		"customer_id":    p.CustomerID,
		"meter_code":     p.MeterCode,
	}
	if p.SubscriptionID != "" {
		payload["subscription_id"] = p.SubscriptionID
	}
	if p.SubscriptionItemID != "" {
		payload["subscription_item_id"] = p.SubscriptionItemID
	}
	if p.MeterID != "" {
		payload["meter_id"] = p.MeterID
	}
	if p.IdempotencyKey != nil {
		payload["idempotency_key"] = *p.IdempotencyKey
	}
	return payload
}
