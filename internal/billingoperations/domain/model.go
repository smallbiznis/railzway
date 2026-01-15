package domain

import (
	"database/sql"
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/datatypes"
)

type BillingActionRecord struct {
	ID             snowflake.ID
	OrgID          snowflake.ID
	EntityType     string
	EntityID       snowflake.ID
	ActionType     string
	ActionBucket   time.Time
	IdempotencyKey string
	Metadata       datatypes.JSONMap
	ActorType      string
	ActorID        string
	CreatedAt      time.Time
}

type BillingAssignmentRecord struct {
	ID                  snowflake.ID
	OrgID               snowflake.ID
	EntityType          string
	EntityID            snowflake.ID
	AssignedTo          string
	AssignedAt          time.Time
	AssignmentExpiresAt time.Time
	Status              string
	ReleasedAt          sql.NullTime
	ReleasedBy          sql.NullString
	ReleaseReason       sql.NullString
	ResolvedAt          sql.NullTime
	ResolvedBy          sql.NullString
	BreachedAt          sql.NullTime
	BreachLevel         sql.NullString
	LastActionAt        sql.NullTime
	SnapshotMetadata    datatypes.JSON
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (BillingAssignmentRecord) TableName() string {
	return "billing_operation_assignments"
}

type BillingActionLookup struct {
	ID snowflake.ID `gorm:"column:id"`
}

type OverdueInvoiceRow struct {
	InvoiceID           snowflake.ID   `gorm:"column:invoice_id"`
	InvoiceNumber       string         `gorm:"column:invoice_number"`
	CustomerID          snowflake.ID   `gorm:"column:customer_id"`
	CustomerName        string         `gorm:"column:customer_name"`
	AmountDue           int64          `gorm:"column:amount_due"`
	DueAt               time.Time      `gorm:"column:due_at"`
	AssignedTo          sql.NullString `gorm:"column:assigned_to"`
	AssignedAt          sql.NullTime   `gorm:"column:assigned_at"`
	AssignmentExpiresAt sql.NullTime   `gorm:"column:assignment_expires_at"`
	Status              sql.NullString `gorm:"column:assignment_status"`
	ReleasedAt          sql.NullTime   `gorm:"column:assignment_released_at"`
	ReleasedBy          sql.NullString `gorm:"column:assignment_released_by"`
	ReleaseReason       sql.NullString `gorm:"column:assignment_release_reason"`
	LastActionAt        sql.NullTime   `gorm:"column:assignment_last_action_at"`
	BreachedAt          sql.NullTime   `gorm:"column:assignment_breached_at"`
	BreachLevel         sql.NullString `gorm:"column:assignment_breach_level"`
	TokenHash           sql.NullString `gorm:"column:token_hash"`
}

type OutstandingCustomerRow struct {
	CustomerID                 snowflake.ID   `gorm:"column:customer_id"`
	CustomerName               string         `gorm:"column:customer_name"`
	Outstanding                int64          `gorm:"column:outstanding"`
	OldestOverdueInvoiceID     sql.NullString `gorm:"column:oldest_overdue_invoice_id"`
	OldestOverdueInvoiceNumber sql.NullString `gorm:"column:oldest_overdue_invoice_number"`
	OldestOverdueAt            sql.NullTime   `gorm:"column:oldest_overdue_at"`
	LastPaymentAt              sql.NullTime   `gorm:"column:last_payment_at"`
	AssignedTo                 sql.NullString `gorm:"column:assigned_to"`
	AssignedAt                 sql.NullTime   `gorm:"column:assigned_at"`
	AssignmentExpiresAt        sql.NullTime   `gorm:"column:assignment_expires_at"`
	Status                     sql.NullString `gorm:"column:assignment_status"`
	ReleasedAt                 sql.NullTime   `gorm:"column:assignment_released_at"`
	ReleasedBy                 sql.NullString `gorm:"column:assignment_released_by"`
	ReleaseReason              sql.NullString `gorm:"column:assignment_release_reason"`
	LastActionAt               sql.NullTime   `gorm:"column:assignment_last_action_at"`
	BreachedAt                 sql.NullTime   `gorm:"column:assignment_breached_at"`
	BreachLevel                sql.NullString `gorm:"column:assignment_breach_level"`
	TokenHash                  sql.NullString `gorm:"column:token_hash"`
}

type PaymentIssueRow struct {
	CustomerID          snowflake.ID   `gorm:"column:customer_id"`
	CustomerName        string         `gorm:"column:customer_name"`
	IssueType           string         `gorm:"column:issue_type"`
	LastAttempt         sql.NullTime   `gorm:"column:last_attempt"`
	AssignedTo          sql.NullString `gorm:"column:assigned_to"`
	AssignedAt          sql.NullTime   `gorm:"column:assigned_at"`
	AssignmentExpiresAt sql.NullTime   `gorm:"column:assignment_expires_at"`
	Status              sql.NullString `gorm:"column:assignment_status"`
	ReleasedAt          sql.NullTime   `gorm:"column:assignment_released_at"`
	ReleasedBy          sql.NullString `gorm:"column:assignment_released_by"`
	ReleaseReason       sql.NullString `gorm:"column:assignment_release_reason"`
	LastActionAt        sql.NullTime   `gorm:"column:assignment_last_action_at"`
	BreachedAt          sql.NullTime   `gorm:"column:assignment_breached_at"`
	BreachLevel         sql.NullString `gorm:"column:assignment_breach_level"`
	TokenHash           sql.NullString `gorm:"column:token_hash"`
}

type CollectionQueueRow struct {
	CustomerID            snowflake.ID   `gorm:"column:customer_id"`
	CustomerName          string         `gorm:"column:customer_name"`
	Outstanding           int64          `gorm:"column:outstanding"`
	OldestUnpaidInvoiceID sql.NullString `gorm:"column:oldest_unpaid_invoice_id"`
	OldestUnpaidInvoice   sql.NullString `gorm:"column:oldest_unpaid_invoice_number"`
	OldestUnpaidAt        sql.NullTime   `gorm:"column:oldest_unpaid_at"`
	LastPaymentAt         sql.NullTime   `gorm:"column:last_payment_at"`
	AssignedTo            sql.NullString `gorm:"column:assigned_to"`
	AssignedAt            sql.NullTime   `gorm:"column:assigned_at"`
	AssignmentExpiresAt   sql.NullTime   `gorm:"column:assignment_expires_at"`
	Status                sql.NullString `gorm:"column:assignment_status"`
	ReleasedAt            sql.NullTime   `gorm:"column:assignment_released_at"`
	ReleasedBy            sql.NullString `gorm:"column:assignment_released_by"`
	ReleaseReason         sql.NullString `gorm:"column:assignment_release_reason"`
	LastActionAt          sql.NullTime   `gorm:"column:assignment_last_action_at"`
	BreachedAt            sql.NullTime   `gorm:"column:assignment_breached_at"`
	BreachLevel           sql.NullString `gorm:"column:assignment_breach_level"`
	TokenHash             sql.NullString `gorm:"column:token_hash"`
}

type FailedPaymentActionRow struct {
	CustomerID          snowflake.ID   `gorm:"column:customer_id"`
	CustomerName        string         `gorm:"column:customer_name"`
	InvoiceID           sql.NullString `gorm:"column:invoice_id"`
	InvoiceNumber       sql.NullString `gorm:"column:invoice_number"`
	AmountDue           sql.NullInt64  `gorm:"column:amount_due"`
	DueAt               sql.NullTime   `gorm:"column:due_at"`
	LastAttempt         sql.NullTime   `gorm:"column:last_attempt"`
	AssignedTo          sql.NullString `gorm:"column:assigned_to"`
	AssignedAt          sql.NullTime   `gorm:"column:assigned_at"`
	AssignmentExpiresAt sql.NullTime   `gorm:"column:assignment_expires_at"`
	Status              sql.NullString `gorm:"column:assignment_status"`
	ReleasedAt          sql.NullTime   `gorm:"column:assignment_released_at"`
	ReleasedBy          sql.NullString `gorm:"column:assignment_released_by"`
	ReleaseReason       sql.NullString `gorm:"column:assignment_release_reason"`
	LastActionAt        sql.NullTime   `gorm:"column:assignment_last_action_at"`
	BreachedAt          sql.NullTime   `gorm:"column:assignment_breached_at"`
	BreachLevel         sql.NullString `gorm:"column:assignment_breach_level"`
	TokenHash           sql.NullString `gorm:"column:token_hash"`
}

type ActionSummaryRow struct {
	CustomersWithOutstanding int   `gorm:"column:customers_with_outstanding"`
	OverdueInvoices          int   `gorm:"column:overdue_invoices"`
	FailedPaymentAttempts    int   `gorm:"column:failed_payment_attempts"`
	TotalOutstanding         int64 `gorm:"column:total_outstanding"`
}

type AssignmentRow struct {
	AssignedTo          string
	AssignedAt          time.Time
	AssignmentExpiresAt time.Time
	Status              string
	ReleasedAt          sql.NullTime
	ReleasedBy          sql.NullString
	ReleaseReason       sql.NullString
	BreachedAt          sql.NullTime
	BreachLevel         sql.NullString
	LastActionAt        sql.NullTime
}

type PerformanceMetrics struct {
	AvgResponseMS   int64   `json:"avg_response_ms"`
	CompletionRatio float64 `json:"completion_ratio"`
	EscalationRate  float64 `json:"escalation_rate"`
	ExposureHandled int64   `json:"exposure_handled"`
	TotalAssigned   int     `json:"total_assigned"`
	TotalResolved   int     `json:"total_resolved"`
	TotalEscalated  int     `json:"total_escalated"`
}


type PerformanceScores struct {
	Responsiveness int `json:"responsiveness"`
	Completion     int `json:"completion"`
	Effectiveness  int `json:"effectiveness"` // Exposure Handled score
	Risk           int `json:"risk"`          // Low Escalation score
	Total          int `json:"total"`
}


type FinOpsScoreSnapshot struct {
	OrgID          string             `json:"org_id"`
	UserID         string             `json:"user_id"`
	PeriodType     string             `json:"period_type"`
	PeriodStart    time.Time          `json:"period_start"`
	PeriodEnd      time.Time          `json:"period_end"`
	ScoringVersion string             `json:"scoring_version"`
	Metrics        PerformanceMetrics `json:"metrics"`
	Scores         PerformanceScores  `json:"scores"`
}

type FinOpsSnapshotRow struct {
	OrgID          snowflake.ID   `gorm:"column:org_id"`
	UserID         string         `gorm:"column:user_id"`
	PeriodType     string         `gorm:"column:period_type"`
	PeriodStart    time.Time      `gorm:"column:period_start"`
	PeriodEnd      time.Time      `gorm:"column:period_end"`
	ScoringVersion string         `gorm:"column:scoring_version"`
	Metrics        datatypes.JSON `gorm:"column:metrics"`
	Scores         datatypes.JSON `gorm:"column:scores"`
}

type InboxRow struct {
	EntityType   string         `gorm:"column:entity_type"`
	EntityID     string         `gorm:"column:entity_id"`
	EntityName   string         `gorm:"column:entity_name"`
	RiskCategory string         `gorm:"column:risk_category"`
	AmountDue    int64          `gorm:"column:amount_due"`
	DueAt        sql.NullTime   `gorm:"column:due_at"`
	DaysOverdue  float64        `gorm:"column:days_overdue"`
	LastAttempt  sql.NullTime   `gorm:"column:last_attempt"`
	TokenHash    sql.NullString `gorm:"column:token_hash"`
	RiskScore    int            `gorm:"column:risk_score"`
}

type MyWorkRow struct {
	AssignmentID       string          `gorm:"column:assignment_id"`
	EntityType         string          `gorm:"column:entity_type"`
	EntityID           string          `gorm:"column:entity_id"`
	SnapshotMetadata   datatypes.JSON  `gorm:"column:snapshot_metadata"`
	AssignedAt         time.Time       `gorm:"column:assigned_at"`
	Status             string          `gorm:"column:status"`
	LastActionAt       sql.NullTime    `gorm:"column:last_action_at"`
	EntityName         sql.NullString  `gorm:"column:entity_name"`
	CustomerName       sql.NullString  `gorm:"column:customer_name"`
	CustomerEmail      sql.NullString  `gorm:"column:customer_email"`
	InvoiceNumber      sql.NullString  `gorm:"column:invoice_number"`
	CurrentAmountDue   sql.NullInt64   `gorm:"column:current_amount_due"`
	CurrentDaysOverdue sql.NullFloat64 `gorm:"column:current_days_overdue"`
	TokenHash          sql.NullString  `gorm:"column:token_hash"`
}

type ResolvedRow struct {
	AssignmentID     string         `gorm:"column:assignment_id"`
	EntityType       string         `gorm:"column:entity_type"`
	EntityID         string         `gorm:"column:entity_id"`
	SnapshotMetadata datatypes.JSON `gorm:"column:snapshot_metadata"`
	Status           string         `gorm:"column:status"`
	ResolvedAt       time.Time      `gorm:"column:resolved_at"`
	ResolvedBy       sql.NullString `gorm:"column:resolved_by"`
	ReleaseReason    sql.NullString `gorm:"column:release_reason"`
	AssignedAt       time.Time      `gorm:"column:assigned_at"`
}

type TeamRow struct {
	UserID                  string `gorm:"column:user_id"`
	ActiveAssignments       int    `gorm:"column:active_assignments"`
	AvgAssignmentAgeMinutes int    `gorm:"column:avg_assignment_age_minutes"`
	TotalExposureOwned      int64  `gorm:"column:total_exposure_owned"`
	EscalationCount         int    `gorm:"column:escalation_count"`
}

type PaymentRow struct {
	ProviderPaymentID string         `gorm:"column:provider_payment_id"`
	Provider          string         `gorm:"column:provider"`
	EventType         string         `gorm:"column:event_type"`
	ReceivedAt        time.Time      `gorm:"column:received_at"`
	Currency          string         `gorm:"column:currency"`
	Payload           datatypes.JSON `gorm:"column:payload"`
}

type ExposureStatsRow struct {
	TotalExposure int64 `gorm:"column:total_exposure"`
	CurrentAmount int64 `gorm:"column:current_amount"`
	Bucket0To30   int64 `gorm:"column:bucket_0_30"`
	Bucket31To60  int64 `gorm:"column:bucket_31_60"`
	Bucket61To90  int64 `gorm:"column:bucket_61_90"`
	Bucket90Plus  int64 `gorm:"column:bucket_90_plus"`
	OverdueCount  int   `gorm:"column:overdue_count"`
}

type TopCustomerExposureRow struct {
	EntityName  string `gorm:"column:entity_name"`
	AmountDue   int64  `gorm:"column:amount_due"`
	RiskScore   int    `gorm:"column:risk_score"`
	DaysOverdue int    `gorm:"column:days_overdue"`
}

type BillingAssignmentRow struct {
	OrgID      snowflake.ID   `gorm:"column:org_id"`
	EntityID   snowflake.ID   `gorm:"column:entity_id"`
	AssignedAt sql.NullTime   `gorm:"column:assigned_at"`
	Status     sql.NullString `gorm:"column:status"`
	BreachedAt sql.NullTime   `gorm:"column:breached_at"`
}


