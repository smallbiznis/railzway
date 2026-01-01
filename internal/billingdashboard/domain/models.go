package domain

import "time"

// CustomerBalance represents a customer's net position.
type CustomerBalance struct {
	CustomerID    string `json:"customer_id"`
	Name          string `json:"name"`
	Balance       int64  `json:"balance"`
	Currency      string `json:"currency"`
	LastInvoiceID string `json:"last_invoice_id,omitempty"`
	PaymentStatus string `json:"payment_status"`
}

// CustomerBalancesResponse is the API response for customer balances.
type CustomerBalancesResponse struct {
	Customers []CustomerBalance `json:"customers"`
}

// BillingCycleSummary captures revenue and invoicing stats for a cycle.
type BillingCycleSummary struct {
	CycleID      string `json:"cycle_id"`
	Period       string `json:"period"`
	TotalRevenue int64  `json:"total_revenue"`
	InvoiceCount int64  `json:"invoice_count"`
	Status       string `json:"status"`
}

// BillingCycleSummaryResponse is the API response for billing cycles.
type BillingCycleSummaryResponse struct {
	Cycles []BillingCycleSummary `json:"cycles"`
}

// BillingActivity represents a human-readable billing event.
type BillingActivity struct {
	Message    string    `json:"message"`
	OccurredAt time.Time `json:"occurred_at"`
}

// BillingActivityResponse is the API response for billing activity.
type BillingActivityResponse struct {
	Activity []BillingActivity `json:"activity"`
}
