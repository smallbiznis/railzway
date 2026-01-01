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
