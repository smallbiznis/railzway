package domain

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/gorm"
)

type Repository interface {
	WithTx(tx *gorm.DB) Repository
	FetchOrgCurrency(ctx context.Context, orgID snowflake.ID) (string, error)
	LoadEntitySnapshot(ctx context.Context, orgID snowflake.ID, entityType string, entityID snowflake.ID) (map[string]any, error)
	ListOverdueInvoices(ctx context.Context, orgID snowflake.ID, currency string, now time.Time, limit int) ([]OverdueInvoiceRow, error)
	ListOutstandingCustomers(ctx context.Context, orgID snowflake.ID, currency string, now time.Time, limit int) ([]OutstandingCustomerRow, error)
	ListPaymentIssues(ctx context.Context, orgID snowflake.ID, now time.Time, limit int) ([]PaymentIssueRow, error)
	LoadActionSummary(ctx context.Context, orgID snowflake.ID, currency string, now time.Time) (ActionSummaryRow, error)
	ListCollectionQueue(ctx context.Context, orgID snowflake.ID, currency string, now time.Time, limit int) ([]CollectionQueueRow, error)
	ListFailedPaymentActions(ctx context.Context, orgID snowflake.ID, currency string, now time.Time, limit int) ([]FailedPaymentActionRow, error)
	LoadAssignment(ctx context.Context, orgID snowflake.ID, entityType string, entityID snowflake.ID) (*AssignmentRow, error)
	LoadAssignmentForUpdate(ctx context.Context, orgID snowflake.ID, entityType string, entityID snowflake.ID) (*BillingAssignmentRecord, error)
	ListActiveAssignments(ctx context.Context) ([]BillingAssignmentRecord, error)

	InsertBillingAction(ctx context.Context, record BillingActionRecord) (bool, error)
	FindActionByIdempotencyKey(ctx context.Context, orgID snowflake.ID, key string) (*BillingActionLookup, error)
	FindActionByBucket(ctx context.Context, orgID snowflake.ID, entityType string, entityID snowflake.ID, actionType string, bucket time.Time) (*BillingActionLookup, error)

	UpsertAssignment(ctx context.Context, record BillingAssignmentRecord) error
	UpdateAssignmentStatus(ctx context.Context, orgID snowflake.ID, entityType string, entityID snowflake.ID, oldStatus, newStatus string, now time.Time) error
	EscalateAssignment(ctx context.Context, orgID snowflake.ID, entityType string, entityID snowflake.ID, breachType string, now time.Time) error

	// IA Methods
	ListInboxItems(ctx context.Context, orgID snowflake.ID, limit int, now time.Time) ([]InboxRow, error)
	ListMyWorkItems(ctx context.Context, orgID snowflake.ID, userID string, limit int, now time.Time) ([]MyWorkRow, error)
	ListRecentlyResolvedItems(ctx context.Context, orgID snowflake.ID, userID string, limit int, since time.Time) ([]ResolvedRow, error)
	GetTeamViewStats(ctx context.Context, orgID snowflake.ID, now time.Time) ([]TeamRow, error)
	ListInvoicePayments(ctx context.Context, orgID, invoiceID snowflake.ID) ([]PaymentRow, error) // invoiceID snowflake or string? Service uses string for GetInvoicePayments but query passes it as param. Payment events metadata is string. If param is string, fine. Use ID if possible.
	GetExposureStats(ctx context.Context, orgID snowflake.ID, now time.Time) (ExposureStatsRow, error)
	ListTopHighExposure(ctx context.Context, orgID snowflake.ID, now time.Time) ([]TopCustomerExposureRow, error)
	ListBillingAssignmentsForPerformance(ctx context.Context, orgID snowflake.ID, userID string, start, end time.Time) ([]BillingAssignmentRow, error)

	// FinOps methods
	FindSnapshotsByUser(ctx context.Context, orgID snowflake.ID, userID string, periodType string, start, end time.Time) ([]FinOpsScoreSnapshot, error)
	FindSnapshotsByUserWithLimit(ctx context.Context, orgID snowflake.ID, userID string, periodType string, start, end time.Time, limit int) ([]FinOpsScoreSnapshot, error)
	FindSnapshotsByOrg(ctx context.Context, orgID snowflake.ID, periodType string, start, end time.Time) ([]FinOpsScoreSnapshot, error)
}
