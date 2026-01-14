package domain

import "time"

// Inbox View (Unassigned / Needs Action)

type InboxRequest struct {
	Limit int `json:"limit" form:"limit"`
}

type InboxItem struct {
	EntityType   string     `json:"entity_type"` // "invoice" | "customer"
	EntityID     string     `json:"entity_id"`
	EntityName   string     `json:"entity_name"`   // invoice_number or customer_name
	RiskCategory string     `json:"risk_category"` // "overdue" | "failed_payment" | "high_exposure"
	RiskScore    int        `json:"risk_score"`    // For sorting
	AmountDue    int64      `json:"amount_due"`
	Currency     string     `json:"currency"`
	DaysOverdue  int        `json:"days_overdue,omitempty"`
	LastAttempt  *time.Time `json:"last_attempt,omitempty"`
	PublicToken  string     `json:"public_token,omitempty"`
}

type InboxResponse struct {
	Items    []InboxItem `json:"items"`
	Currency string      `json:"currency"`
}

// My Work View (Claimed by Me)

type MyWorkRequest struct {
	Limit int `json:"limit" form:"limit"`
}

type MyWorkItem struct {
	AssignmentID string `json:"assignment_id"`
	EntityType   string `json:"entity_type"`
	EntityID     string `json:"entity_id"`
	EntityName   string `json:"entity_name"`
	CustomerName string `json:"customer_name,omitempty"`
	CustomerEmail string `json:"customer_email,omitempty"`
	InvoiceNumber string `json:"invoice_number,omitempty"`

	// Snapshot values at claim time (stable)
	AmountDueAtClaim   int64 `json:"amount_due_at_claim"`
	DaysOverdueAtClaim int   `json:"days_overdue_at_claim"`

	// Current values (optional, non-sorting)
	CurrentAmountDue   int64 `json:"current_amount_due,omitempty"`
	CurrentDaysOverdue int   `json:"current_days_overdue,omitempty"`

	Currency      string     `json:"currency"`
	ClaimedAt     time.Time  `json:"claimed_at"`
	AssignmentAge string     `json:"assignment_age"` // "2h 15m"
	Status        string     `json:"status"`         // "claimed" | "in_progress"
	LastActionAt  *time.Time `json:"last_action_at,omitempty"`
	PublicToken   string     `json:"public_token,omitempty"`
}

type MyWorkResponse struct {
	Items    []MyWorkItem `json:"items"`
	Currency string       `json:"currency"`
}

// Recently Resolved View

type RecentlyResolvedRequest struct {
	Limit int `json:"limit" form:"limit"`
}

type ResolvedItem struct {
	AssignmentID     string    `json:"assignment_id"`
	EntityType       string    `json:"entity_type"`
	EntityID         string    `json:"entity_id"`
	EntityName       string    `json:"entity_name"`
	Status           string    `json:"status"` // "resolved" | "released" | "escalated"
	ResolvedAt       time.Time `json:"resolved_at"`
	ResolvedBy       string    `json:"resolved_by"`
	Reason           string    `json:"reason,omitempty"`
	ClaimedAt        time.Time `json:"claimed_at"`
	Duration         string    `json:"duration"` // "3h 45m"
	AmountDueAtClaim int64     `json:"amount_due_at_claim"`
	Currency         string    `json:"currency"`
}

type RecentlyResolvedResponse struct {
	Items []ResolvedItem `json:"items"`
}

// Team View (Manager Only)

type TeamViewRequest struct{}

type TeamMemberWorkload struct {
	UserID             string `json:"user_id"`
	ActiveAssignments  int    `json:"active_assignments"`
	AvgAssignmentAge   string `json:"avg_assignment_age"` // "1h 30m"
	TotalExposureOwned int64  `json:"total_exposure_owned"`
	EscalationCount    int    `json:"escalation_count"`
}

type TeamSummary struct {
	TotalActiveAssignments int    `json:"total_active_assignments"`
	TotalExposure          int64  `json:"total_exposure"`
	AvgAssignmentAge       string `json:"avg_assignment_age"`
	EscalationCount        int    `json:"escalation_count"`
}

type TeamViewResponse struct {
	Members  []TeamMemberWorkload `json:"members"`
	Summary  TeamSummary          `json:"summary"`
	Currency string               `json:"currency"`
}

// Exposure Analysis View

type ExposureAnalysisRequest struct{}

type ExposureBucket struct {
	Bucket string `json:"bucket"` // "0-30", "31-60", "61-90", "90+"
	Amount int64  `json:"amount"`
	Count  int    `json:"count"`
}

type ExposureCategory struct {
	Category string `json:"category"` // "overdue", "high_exposure", "failed_payment"
	Amount   int64  `json:"amount"`
	Count    int    `json:"count"`
}

type ExposureAnalysisResponse struct {
	TotalExposure   int64              `json:"total_exposure"`
	Currency        string             `json:"currency"`
	ByRiskCategory  []ExposureCategory `json:"by_risk_category"`
	ByAgingBucket   []ExposureBucket   `json:"by_aging_bucket"`
	TopHighExposure []InboxItem        `json:"top_high_exposure"` // Reuse InboxItem for list
}

// Invoice Payment Details

type PaymentDetail struct {
	PaymentID   string    `json:"payment_id"`
	Amount      int64     `json:"amount"`
	Currency    string    `json:"currency"`
	OccurredAt  time.Time `json:"occurred_at"`
	Provider    string    `json:"provider"`         // "stripe"
	Method      string    `json:"method,omitempty"` // "card", "bank_transfer"
	CardBrand   string    `json:"card_brand,omitempty"`   // "visa", "mastercard"
	CardLast4   string    `json:"card_last4,omitempty"`   // "4242"
	Status      string    `json:"status"`           // "succeeded", "failed"
}

type InvoicePaymentsResponse struct {
	Payments []PaymentDetail `json:"payments"`
}
