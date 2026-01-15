package domain

import (
	"context"
	"errors"
	"time"
)

type OverdueInvoice struct {
	InvoiceID     string      `json:"invoice_id"`
	InvoiceNumber string      `json:"invoice_number"`
	CustomerID    string      `json:"customer_id"`
	CustomerName  string      `json:"customer_name"`
	AmountDue     int64       `json:"amount_due"`
	Currency      string      `json:"currency"`
	DueAt         time.Time   `json:"due_at"`
	DaysOverdue   int         `json:"days_overdue"`
	PublicToken   string      `json:"public_token,omitempty"`
	Assignment    *Assignment `json:"assignment,omitempty"`
}

type OverdueInvoicesResponse struct {
	Currency string           `json:"currency"`
	Invoices []OverdueInvoice `json:"invoices"`
	HasData  bool             `json:"has_data"`
}

type OutstandingCustomer struct {
	CustomerID             string      `json:"customer_id"`
	CustomerName           string      `json:"customer_name"`
	OutstandingBalance     int64       `json:"outstanding_balance"`
	Currency               string      `json:"currency"`
	OldestOverdueInvoiceID string      `json:"oldest_overdue_invoice_id,omitempty"`
	OldestOverdueInvoice   string      `json:"oldest_overdue_invoice,omitempty"`
	OldestOverdueAt        *time.Time  `json:"oldest_overdue_at,omitempty"`
	LastPaymentAt          *time.Time  `json:"last_payment_at,omitempty"`
	OldestOverdueDays      int         `json:"oldest_overdue_days,omitempty"`
	HasOverdueOutstanding  bool        `json:"has_overdue_outstanding"`
	PublicToken            string      `json:"public_token,omitempty"`
	Assignment             *Assignment `json:"assignment,omitempty"`
}

type OutstandingCustomersResponse struct {
	Currency  string                `json:"currency"`
	Customers []OutstandingCustomer `json:"customers"`
	HasData   bool                  `json:"has_data"`
}

type PaymentIssue struct {
	CustomerID          string      `json:"customer_id"`
	CustomerName        string      `json:"customer_name"`
	IssueType           string      `json:"issue_type"`
	LastAttempt         *time.Time  `json:"last_attempt"`
	AssignedTo          string      `json:"assigned_to,omitempty"`
	AssignmentExpiresAt *time.Time  `json:"assignment_expires_at,omitempty"`
	Assignment          *Assignment `json:"assignment,omitempty"`
}

type PaymentIssuesResponse struct {
	Issues  []PaymentIssue `json:"issues"`
	HasData bool           `json:"has_data"`
}

type ActionSummary struct {
	CustomersWithOutstanding int    `json:"customers_with_outstanding"`
	OverdueInvoices          int    `json:"overdue_invoices"`
	FailedPaymentAttempts    int    `json:"failed_payment_attempts"`
	TotalOutstanding         int64  `json:"total_outstanding"`
	Currency                 string `json:"currency"`
}

type CriticalAction struct {
	Category            string      `json:"category"`
	InvoiceID           string      `json:"invoice_id,omitempty"`
	InvoiceNumber       string      `json:"invoice_number,omitempty"`
	CustomerID          string      `json:"customer_id"`
	CustomerName        string      `json:"customer_name"`
	AmountDue           int64       `json:"amount_due"`
	Currency            string      `json:"currency"`
	DueAt               *time.Time  `json:"due_at,omitempty"`
	DaysOverdue         int         `json:"days_overdue"`
	LastAttempt         *time.Time  `json:"last_attempt,omitempty"`
	AssignedTo          string      `json:"assigned_to,omitempty"`
	AssignmentExpiresAt *time.Time  `json:"assignment_expires_at,omitempty"`
	PublicToken         string      `json:"public_token,omitempty"`
	Assignment          *Assignment `json:"assignment,omitempty"`
}

type CollectionQueueEntry struct {
	CustomerID            string      `json:"customer_id"`
	CustomerName          string      `json:"customer_name"`
	OutstandingBalance    int64       `json:"outstanding_balance"`
	Currency              string      `json:"currency"`
	OldestUnpaidInvoiceID string      `json:"oldest_unpaid_invoice_id,omitempty"`
	OldestUnpaidInvoice   string      `json:"oldest_unpaid_invoice,omitempty"`
	OldestUnpaidAt        *time.Time  `json:"oldest_unpaid_at,omitempty"`
	OldestUnpaidDays      int         `json:"oldest_unpaid_days,omitempty"`
	LastPaymentAt         *time.Time  `json:"last_payment_at,omitempty"`
	AgingBucket           string      `json:"aging_bucket"`
	RiskLevel             string      `json:"risk_level"`
	AssignedTo            string      `json:"assigned_to,omitempty"`
	AssignmentExpiresAt   *time.Time  `json:"assignment_expires_at,omitempty"`
	PublicToken           string      `json:"public_token,omitempty"`
	Assignment            *Assignment `json:"assignment,omitempty"`
}

type BillingOperationsResponse struct {
	Currency        string                 `json:"currency"`
	Summary         ActionSummary          `json:"summary"`
	CriticalActions []CriticalAction       `json:"critical_actions"`
	CollectionQueue []CollectionQueueEntry `json:"collection_queue"`
	PaymentIssues   []PaymentIssue         `json:"payment_issues"`
	GeneratedAt     time.Time              `json:"generated_at"`
}

type RecordActionRequest struct {
	ActionType     string         `json:"action_type"`
	EntityType     string         `json:"entity_type"`
	EntityID       string         `json:"entity_id"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type RecordActionResponse struct {
	ActionID   string    `json:"action_id,omitempty"`
	Status     string    `json:"status"`
	RecordedAt time.Time `json:"recorded_at"`
}

type ClaimAssignmentRequest struct {
	EntityType           string `json:"entity_type"`
	EntityID             string `json:"entity_id"`
	AssignedTo           string `json:"assigned_to,omitempty"`
	AssignmentTTLMinutes int    `json:"assignment_ttl_minutes,omitempty"`
}

type AssignmentResponse struct {
	Assignment Assignment `json:"assignment"`
	Status     string     `json:"status"`
}

type ReleaseAssignmentRequest struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Reason     string `json:"reason"`
	ReleasedBy string `json:"released_by"`
}

type ResolveAssignmentRequest struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Resolution string `json:"resolution"` // e.g., "payment_received", "issue_fixed"
	ResolvedBy string `json:"resolved_by"`
}

type Assignment struct {
	EntityType          string     `json:"entity_type"`
	EntityID            string     `json:"entity_id"`
	Status              string     `json:"status"`
	AssignedTo          string     `json:"assigned_to"`
	AssignedAt          time.Time  `json:"assigned_at"`
	AssignmentExpiresAt time.Time  `json:"assignment_expires_at"`
	LastActionAt        *time.Time `json:"last_action_at,omitempty"`
	ReleasedAt          *time.Time `json:"released_at,omitempty"`
	ReleasedBy          string     `json:"released_by,omitempty"`
	ReleaseReason       string     `json:"release_reason,omitempty"`
	BreachedAt          *time.Time `json:"breached_at,omitempty"`
	BreachLevel         string     `json:"breach_level,omitempty"`
	SLAStatus           string     `json:"sla_status"`
	TimeSinceAssigned   string     `json:"time_since_assigned"`
}

type RecordFollowUpRequest struct {
	AssignmentID  string `json:"assignment_id"`
	EmailProvider string `json:"email_provider"` // "gmail", "outlook", "default"
}

const (
	EntityTypeInvoice  = "invoice"
	EntityTypeCustomer = "customer"
)

const (
	ActionTypeFollowUp     = "follow_up"
	ActionTypeRetryPayment = "retry_payment"
	ActionTypeMarkReviewed = "mark_reviewed"
	ActionTypeClaim        = "claim"
	ActionTypeRelease      = "released"
	ActionTypeResolve      = "resolve"
)

const (
	ActionStatusRecorded  = "recorded"
	ActionStatusDuplicate = "duplicate"
)

const (
	AssignmentStatusAssigned   = "assigned"
	AssignmentStatusInProgress = "in_progress"
	AssignmentStatusReleased   = "released"
	AssignmentStatusResolved   = "resolved"
)

const (
	SLAFresh    = "fresh"
	SLAActive   = "active"
	SLAAging    = "aging"
	SLAStale    = "stale"
	SLABreached = "breached"
	SLAResolved = "resolved"
)

const (
	AssignmentStatusEscalated = "escalated"
)

const (
	CriticalCategoryOverdueInvoice = "overdue_invoice"
	CriticalCategoryFailedPayment  = "failed_payment"
)


type Service interface {
	ListOverdueInvoices(ctx context.Context, limit int) (OverdueInvoicesResponse, error)
	ListOutstandingCustomers(ctx context.Context, limit int) (OutstandingCustomersResponse, error)
	ListPaymentIssues(ctx context.Context, limit int) (PaymentIssuesResponse, error)
	GetOperations(ctx context.Context, limit int) (BillingOperationsResponse, error)
	RecordAction(ctx context.Context, req RecordActionRequest) (RecordActionResponse, error)
	ClaimAssignment(ctx context.Context, req ClaimAssignmentRequest) (AssignmentResponse, error)
	ReleaseAssignment(ctx context.Context, req ReleaseAssignmentRequest) error
	ResolveAssignment(ctx context.Context, req ResolveAssignmentRequest) error
	EvaluateSLAs(ctx context.Context) error
	CalculatePerformance(ctx context.Context, userID string, start, end time.Time) (FinOpsScoreSnapshot, error)
	GetPerformanceHistory(ctx context.Context, userID string, limit int) ([]FinOpsScoreSnapshot, error)
	AggregateDailyPerformance(ctx context.Context) error

	// API Methods (Read-Only from Snapshots)
	GetMyPerformance(ctx context.Context, userID string, req GetPerformanceRequest) (*PerformanceResponse, error)
	GetTeamPerformance(ctx context.Context, req GetPerformanceRequest) (*TeamPerformanceResponse, error)

	// IA Methods (Task-Centric Views)
	GetInbox(ctx context.Context, req InboxRequest) (InboxResponse, error)
	GetMyWork(ctx context.Context, userID string, req MyWorkRequest) (MyWorkResponse, error)
	GetRecentlyResolved(ctx context.Context, userID string, req RecentlyResolvedRequest) (RecentlyResolvedResponse, error)
	GetTeamView(ctx context.Context, req TeamViewRequest) (TeamViewResponse, error)
	GetExposureAnalysis(ctx context.Context, req ExposureAnalysisRequest) (ExposureAnalysisResponse, error)

	// Follow-Up Email (opens user's email client)
	RecordFollowUp(ctx context.Context, req RecordFollowUpRequest) error

	// Invoice Payment Details
	GetInvoicePayments(ctx context.Context, invoiceID string) (InvoicePaymentsResponse, error)
}

var (
	ErrInvalidOrganization   = errors.New("invalid_organization")
	ErrInvalidEntityType     = errors.New("invalid_entity_type")
	ErrInvalidEntityID       = errors.New("invalid_entity_id")
	ErrInvalidActionType     = errors.New("invalid_action_type")
	ErrInvalidAssignee       = errors.New("invalid_assignee")
	ErrInvalidIdempotencyKey = errors.New("invalid_idempotency_key")
	ErrInvalidAssignmentTTL  = errors.New("invalid_assignment_ttl")
	ErrAssignmentConflict    = errors.New("assignment_conflict")
)
