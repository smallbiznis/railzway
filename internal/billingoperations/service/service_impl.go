package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	auditdomain "github.com/smallbiznis/railzway/internal/audit/domain"
	auditcontext "github.com/smallbiznis/railzway/internal/auditcontext"
	"github.com/smallbiznis/railzway/internal/billingoperations/domain"
	"github.com/smallbiznis/railzway/internal/clock"
	"github.com/smallbiznis/railzway/internal/config"

	ledgerdomain "github.com/smallbiznis/railzway/internal/ledger/domain"
	"github.com/smallbiznis/railzway/internal/orgcontext"
	paymentdomain "github.com/smallbiznis/railzway/internal/payment/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB       *gorm.DB
	Log      *zap.Logger
	Clock    clock.Clock
	GenID    *snowflake.Node
	AuditSvc auditdomain.Service `optional:"true"`
	Cfg      config.Config

	BillingConfig *config.BillingConfigHolder
}

type Service struct {
	db       *gorm.DB
	log      *zap.Logger
	clock    clock.Clock
	genID    *snowflake.Node
	auditSvc auditdomain.Service
	encKey   []byte

	billingCfg   *config.BillingConfigHolder
	snapshotRepo *FinOpsSnapshotRepository
}

func NewService(p Params) domain.Service {
	secret := strings.TrimSpace(p.Cfg.PaymentProviderConfigSecret)
	var key []byte
	if secret != "" {
		sum := sha256.Sum256([]byte(secret))
		key = sum[:]
	}

	return &Service{
		db:       p.DB,
		log:      p.Log.Named("billingoperations.service"),
		clock:    p.Clock,
		genID:    p.GenID,
		auditSvc: p.AuditSvc,
		encKey:   key,

		billingCfg:   p.BillingConfig,
		snapshotRepo: NewFinOpsSnapshotRepository(p.DB),
	}
}

func (s *Service) ListOverdueInvoices(ctx context.Context, limit int) (domain.OverdueInvoicesResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.OverdueInvoicesResponse{}, domain.ErrInvalidOrganization
	}
	if limit <= 0 {
		limit = 25
	}

	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.OverdueInvoicesResponse{}, err
	}

	now := s.clock.Now().UTC()
	rows, err := s.listOverdueInvoices(ctx, orgID, currency, now, limit)
	if err != nil {
		return domain.OverdueInvoicesResponse{}, err
	}

	invoices := make([]domain.OverdueInvoice, 0, len(rows))
	for _, row := range rows {
		invoiceNumber := strings.TrimSpace(row.InvoiceNumber)
		if invoiceNumber == "" {
			invoiceNumber = row.InvoiceID.String()
		}

		daysOverdue := int(now.Sub(row.DueAt).Hours() / 24)
		if daysOverdue < 0 {
			daysOverdue = 0
		}

		assignedToProp := domain.Assignment{}
		if row.AssignedTo.Valid {
			assignedAtVal := time.Time{}
			if row.AssignedAt.Valid {
				assignedAtVal = row.AssignedAt.Time
			}
			assignedToProp = assignmentFields(
				row.AssignedTo,
				assignedAtVal,
				row.AssignmentExpiresAt,
				row.Status.String,
				row.ReleasedAt,
				row.ReleasedBy,
				row.ReleaseReason,
				row.BreachedAt,
				row.BreachLevel,
				row.LastActionAt,
				now,
			)
		} else {
			assignedToProp = assignmentFields(
				row.AssignedTo,
				time.Time{}, // no assigned at
				row.AssignmentExpiresAt,
				"", // no status
				sql.NullTime{},
				sql.NullString{},
				sql.NullString{},
				sql.NullTime{},
				sql.NullString{},
				sql.NullTime{},
				now,
			)
		}

		assignmentPtr := &assignedToProp
		if assignedToProp.AssignedTo == "" && assignedToProp.Status == "" {
			assignmentPtr = nil
		}

		invoices = append(invoices, domain.OverdueInvoice{
			InvoiceID:     row.InvoiceID.String(),
			InvoiceNumber: invoiceNumber,
			CustomerID:    row.CustomerID.String(),
			CustomerName:  row.CustomerName,
			AmountDue:     row.AmountDue,
			Currency:      currency,
			DueAt:         row.DueAt,
			DaysOverdue:   daysOverdue,
			PublicToken:   decryptToken(s.encKey, row.TokenHash.String),
			Assignment:    assignmentPtr,
		})

	}

	return domain.OverdueInvoicesResponse{
		Currency: currency,
		Invoices: invoices,
		HasData:  len(invoices) > 0,
	}, nil
}

func (s *Service) ListOutstandingCustomers(ctx context.Context, limit int) (domain.OutstandingCustomersResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.OutstandingCustomersResponse{}, domain.ErrInvalidOrganization
	}
	if limit <= 0 {
		limit = 25
	}

	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.OutstandingCustomersResponse{}, err
	}

	now := s.clock.Now().UTC()
	rows, err := s.listOutstandingCustomers(ctx, orgID, currency, now, limit)
	if err != nil {
		return domain.OutstandingCustomersResponse{}, err
	}

	customers := make([]domain.OutstandingCustomer, 0, len(rows))
	for _, row := range rows {
		oldestOverdueInvoiceID := ""
		if row.OldestOverdueInvoiceID.Valid {
			oldestOverdueInvoiceID = row.OldestOverdueInvoiceID.String
		}

		oldestOverdueInvoiceNumber := ""
		if row.OldestOverdueInvoiceNumber.Valid {
			oldestOverdueInvoiceNumber = strings.TrimSpace(row.OldestOverdueInvoiceNumber.String)
		}
		if oldestOverdueInvoiceNumber == "" && oldestOverdueInvoiceID != "" {
			oldestOverdueInvoiceNumber = oldestOverdueInvoiceID
		}

		var oldestOverdueAt *time.Time
		var oldestOverdueDays int
		if row.OldestOverdueAt.Valid {
			due := row.OldestOverdueAt.Time.UTC()
			oldestOverdueAt = &due
			oldestOverdueDays = int(now.Sub(due).Hours() / 24)
			if oldestOverdueDays < 0 {
				oldestOverdueDays = 0
			}
		}

		var lastPaymentAt *time.Time
		if row.LastPaymentAt.Valid {
			occurred := row.LastPaymentAt.Time.UTC()
			lastPaymentAt = &occurred
		}

		assignedToProp := domain.Assignment{}
		if row.AssignedTo.Valid {
			assignedAtVal := time.Time{}
			if row.AssignedAt.Valid {
				assignedAtVal = row.AssignedAt.Time
			}
			assignedToProp = assignmentFields(
				row.AssignedTo,
				assignedAtVal,
				row.AssignmentExpiresAt,
				row.Status.String,
				row.ReleasedAt,
				row.ReleasedBy,
				row.ReleaseReason,
				row.BreachedAt,
				row.BreachLevel,
				row.LastActionAt,
				now,
			)
		} else {
			assignedToProp = assignmentFields(
				row.AssignedTo,
				time.Time{},
				row.AssignmentExpiresAt,
				"",
				sql.NullTime{},
				sql.NullString{},
				sql.NullString{},
				sql.NullTime{},
				sql.NullString{}, // Added missing argument
				sql.NullTime{},   // Added missing argument
				now,
			)
		}

		assignmentPtr := &assignedToProp
		if assignedToProp.AssignedTo == "" && assignedToProp.Status == "" {
			assignmentPtr = nil
		}

		customers = append(customers, domain.OutstandingCustomer{
			CustomerID:             row.CustomerID.String(),
			CustomerName:           row.CustomerName,
			OutstandingBalance:     row.Outstanding,
			Currency:               currency,
			OldestOverdueInvoiceID: oldestOverdueInvoiceID,
			OldestOverdueInvoice:   oldestOverdueInvoiceNumber,
			OldestOverdueAt:        oldestOverdueAt,
			LastPaymentAt:          lastPaymentAt,
			OldestOverdueDays:      oldestOverdueDays,
			HasOverdueOutstanding:  oldestOverdueAt != nil,
			PublicToken:            decryptToken(s.encKey, row.TokenHash.String),
			Assignment:             assignmentPtr,
		})

	}

	return domain.OutstandingCustomersResponse{
		Currency:  currency,
		Customers: customers,
		HasData:   len(customers) > 0,
	}, nil
}

func (s *Service) ListPaymentIssues(ctx context.Context, limit int) (domain.PaymentIssuesResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.PaymentIssuesResponse{}, domain.ErrInvalidOrganization
	}
	if limit <= 0 {
		limit = 25
	}

	now := s.clock.Now().UTC()
	rows, err := s.listPaymentIssues(ctx, orgID, now, limit)
	if err != nil {
		return domain.PaymentIssuesResponse{}, err
	}

	issues := make([]domain.PaymentIssue, 0, len(rows))
	for _, row := range rows {
		var lastAttempt *time.Time
		if row.LastAttempt.Valid {
			occurred := row.LastAttempt.Time.UTC()
			lastAttempt = &occurred
		}
		assignedToProp := domain.Assignment{}
		if row.AssignedTo.Valid {
			assignedAtVal := time.Time{}
			if row.AssignedAt.Valid {
				assignedAtVal = row.AssignedAt.Time
			}
			assignedToProp = assignmentFields(
				row.AssignedTo,
				assignedAtVal,
				row.AssignmentExpiresAt,
				row.Status.String,
				row.ReleasedAt,
				row.ReleasedBy,
				row.ReleaseReason,
				row.BreachedAt,
				row.BreachLevel,
				row.LastActionAt,
				now,
			)
		} else {
			assignedToProp = assignmentFields(
				row.AssignedTo,
				time.Time{}, // no assigned at
				row.AssignmentExpiresAt,
				"", // no status
				sql.NullTime{},
				sql.NullString{},
				sql.NullString{},
				sql.NullTime{},
				sql.NullString{},
				sql.NullTime{},
				now,
			)
		}

		assignmentPtr := &assignedToProp
		if assignedToProp.AssignedTo == "" && assignedToProp.Status == "" {
			assignmentPtr = nil
		}

		issues = append(issues, domain.PaymentIssue{
			CustomerID:          row.CustomerID.String(),
			CustomerName:        row.CustomerName,
			IssueType:           row.IssueType,
			LastAttempt:         lastAttempt,
			AssignedTo:          assignedToProp.AssignedTo,
			AssignmentExpiresAt: &assignedToProp.AssignmentExpiresAt,
			Assignment:          assignmentPtr,
		})
	}

	return domain.PaymentIssuesResponse{
		Issues:  issues,
		HasData: len(issues) > 0,
	}, nil
}

func (s *Service) GetOperations(ctx context.Context, limit int) (domain.BillingOperationsResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.BillingOperationsResponse{}, domain.ErrInvalidOrganization
	}
	if limit <= 0 {
		limit = 25
	}

	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.BillingOperationsResponse{}, err
	}

	now := s.clock.Now().UTC()
	summary, err := s.loadActionSummary(ctx, orgID, currency, now)
	if err != nil {
		return domain.BillingOperationsResponse{}, err
	}

	overdueRows, err := s.listOverdueInvoices(ctx, orgID, currency, now, limit)
	if err != nil {
		return domain.BillingOperationsResponse{}, err
	}
	failedRows, err := s.listFailedPaymentActions(ctx, orgID, currency, now, limit)
	if err != nil {
		return domain.BillingOperationsResponse{}, err
	}
	queueRows, err := s.listCollectionQueue(ctx, orgID, currency, now, limit)
	if err != nil {
		return domain.BillingOperationsResponse{}, err
	}
	paymentRows, err := s.listPaymentIssues(ctx, orgID, now, limit)
	if err != nil {
		return domain.BillingOperationsResponse{}, err
	}

	criticalActions := make([]domain.CriticalAction, 0, len(overdueRows)+len(failedRows))
	for _, row := range overdueRows {
		invoiceNumber := strings.TrimSpace(row.InvoiceNumber)
		if invoiceNumber == "" {
			invoiceNumber = row.InvoiceID.String()
		}

		dueAt := row.DueAt.UTC()
		daysOverdue := int(now.Sub(dueAt).Hours() / 24)
		if daysOverdue < 0 {
			daysOverdue = 0
		}

		assignedToProp := domain.Assignment{}
		if row.AssignedTo.Valid {
			assignedAtVal := time.Time{}
			if row.AssignedAt.Valid {
				assignedAtVal = row.AssignedAt.Time
			}
			assignedToProp = assignmentFields(
				row.AssignedTo,
				assignedAtVal,
				row.AssignmentExpiresAt,
				row.Status.String,
				row.ReleasedAt,
				row.ReleasedBy,
				row.ReleaseReason,
				row.BreachedAt,
				row.BreachLevel,
				row.LastActionAt,
				now,
			)
		} else {
			assignedToProp = assignmentFields(
				row.AssignedTo,
				time.Time{}, // no assigned at
				row.AssignmentExpiresAt,
				"", // no status
				sql.NullTime{},
				sql.NullString{},
				sql.NullString{},
				sql.NullTime{},
				sql.NullString{},
				sql.NullTime{},
				now,
			)
		}

		assignmentPtr := &assignedToProp
		if assignedToProp.AssignedTo == "" && assignedToProp.Status == "" {
			assignmentPtr = nil
		}

		criticalActions = append(criticalActions, domain.CriticalAction{
			Category:            domain.CriticalCategoryOverdueInvoice,
			InvoiceID:           row.InvoiceID.String(),
			InvoiceNumber:       invoiceNumber,
			CustomerID:          row.CustomerID.String(),
			CustomerName:        row.CustomerName,
			AmountDue:           row.AmountDue,
			Currency:            currency,
			DueAt:               &dueAt,
			DaysOverdue:         daysOverdue,
			AssignedTo:          assignedToProp.AssignedTo,
			AssignmentExpiresAt: &assignedToProp.AssignmentExpiresAt,
			PublicToken:         decryptToken(s.encKey, row.TokenHash.String),
			Assignment:          assignmentPtr,
		})
	}

	for _, row := range failedRows {
		invoiceID := ""
		if row.InvoiceID.Valid {
			invoiceID = strings.TrimSpace(row.InvoiceID.String)
		}

		invoiceNumber := ""
		if row.InvoiceNumber.Valid {
			invoiceNumber = strings.TrimSpace(row.InvoiceNumber.String)
		}
		if invoiceNumber == "" {
			invoiceNumber = invoiceID
		}

		var dueAt *time.Time
		daysOverdue := 0
		if row.DueAt.Valid {
			due := row.DueAt.Time.UTC()
			dueAt = &due
			daysOverdue = int(now.Sub(due).Hours() / 24)
			if daysOverdue < 0 {
				daysOverdue = 0
			}
		}

		var lastAttempt *time.Time
		if row.LastAttempt.Valid {
			occurred := row.LastAttempt.Time.UTC()
			lastAttempt = &occurred
		}

		assignedToProp := domain.Assignment{}
		if row.AssignedTo.Valid {
			assignedAtVal := time.Time{}
			if row.AssignedAt.Valid {
				assignedAtVal = row.AssignedAt.Time
			}
			assignedToProp = assignmentFields(
				row.AssignedTo,
				assignedAtVal,
				row.AssignmentExpiresAt,
				row.Status.String,
				row.ReleasedAt,
				row.ReleasedBy,
				row.ReleaseReason,
				row.BreachedAt,
				row.BreachLevel,
				row.LastActionAt,
				now,
			)
		} else {
			assignedToProp = assignmentFields(
				row.AssignedTo,
				time.Time{}, // no assigned at
				row.AssignmentExpiresAt,
				"", // no status
				sql.NullTime{},
				sql.NullString{},
				sql.NullString{},
				sql.NullTime{},
				sql.NullString{},
				sql.NullTime{},
				now,
			)
		}

		assignmentPtr := &assignedToProp
		if assignedToProp.AssignedTo == "" && assignedToProp.Status == "" {
			assignmentPtr = nil
		}

		amountDue := int64(0)
		if row.AmountDue.Valid {
			amountDue = row.AmountDue.Int64
		}

		criticalActions = append(criticalActions, domain.CriticalAction{
			Category:            domain.CriticalCategoryFailedPayment,
			InvoiceID:           invoiceID,
			InvoiceNumber:       invoiceNumber,
			CustomerID:          row.CustomerID.String(),
			CustomerName:        row.CustomerName,
			AmountDue:           amountDue,
			Currency:            currency,
			DueAt:               dueAt,
			DaysOverdue:         daysOverdue,
			LastAttempt:         lastAttempt,
			AssignedTo:          assignedToProp.AssignedTo,
			AssignmentExpiresAt: &assignedToProp.AssignmentExpiresAt,
			PublicToken:         decryptToken(s.encKey, row.TokenHash.String),
			Assignment:          assignmentPtr,
		})
	}

	sort.SliceStable(criticalActions, func(i, j int) bool {
		if criticalActions[i].DaysOverdue != criticalActions[j].DaysOverdue {
			return criticalActions[i].DaysOverdue > criticalActions[j].DaysOverdue
		}
		if criticalActions[i].AmountDue != criticalActions[j].AmountDue {
			return criticalActions[i].AmountDue > criticalActions[j].AmountDue
		}
		if criticalActions[i].Category != criticalActions[j].Category {
			return criticalActions[i].Category < criticalActions[j].Category
		}
		if criticalActions[i].CustomerName != criticalActions[j].CustomerName {
			return criticalActions[i].CustomerName < criticalActions[j].CustomerName
		}
		return criticalActions[i].InvoiceID < criticalActions[j].InvoiceID
	})
	if len(criticalActions) > limit {
		criticalActions = criticalActions[:limit]
	}

	queue := make([]domain.CollectionQueueEntry, 0, len(queueRows))
	for _, row := range queueRows {
		oldestInvoiceID := ""
		if row.OldestUnpaidInvoiceID.Valid {
			oldestInvoiceID = strings.TrimSpace(row.OldestUnpaidInvoiceID.String)
		}
		oldestInvoiceNumber := ""
		if row.OldestUnpaidInvoice.Valid {
			oldestInvoiceNumber = strings.TrimSpace(row.OldestUnpaidInvoice.String)
		}
		if oldestInvoiceNumber == "" {
			oldestInvoiceNumber = oldestInvoiceID
		}

		var oldestUnpaidAt *time.Time
		oldestUnpaidDays := 0
		if row.OldestUnpaidAt.Valid {
			due := row.OldestUnpaidAt.Time.UTC()
			oldestUnpaidAt = &due
			oldestUnpaidDays = int(now.Sub(due).Hours() / 24)
			if oldestUnpaidDays < 0 {
				oldestUnpaidDays = 0
			}
		}

		var lastPaymentAt *time.Time
		if row.LastPaymentAt.Valid {
			occurred := row.LastPaymentAt.Time.UTC()
			lastPaymentAt = &occurred
		}

		assignedToProp := domain.Assignment{}
		if row.AssignedTo.Valid {
			assignedAtVal := time.Time{}
			if row.AssignedAt.Valid {
				assignedAtVal = row.AssignedAt.Time
			}
			assignedToProp = assignmentFields(
				row.AssignedTo,
				assignedAtVal,
				row.AssignmentExpiresAt,
				row.Status.String,
				row.ReleasedAt,
				row.ReleasedBy,
				row.ReleaseReason,
				row.BreachedAt,
				row.BreachLevel,
				row.LastActionAt,
				now,
			)
		} else {
			assignedToProp = assignmentFields(
				row.AssignedTo,
				time.Time{}, // no assigned at
				row.AssignmentExpiresAt,
				"", // no status
				sql.NullTime{},
				sql.NullString{},
				sql.NullString{},
				sql.NullTime{},
				sql.NullString{},
				sql.NullTime{},
				now,
			)
		}

		assignmentPtr := &assignedToProp
		if assignedToProp.AssignedTo == "" && assignedToProp.Status == "" {
			assignmentPtr = nil
		}

		queue = append(queue, domain.CollectionQueueEntry{
			CustomerID:            row.CustomerID.String(),
			CustomerName:          row.CustomerName,
			OutstandingBalance:    row.Outstanding,
			Currency:              currency,
			OldestUnpaidInvoiceID: oldestInvoiceID,
			OldestUnpaidInvoice:   oldestInvoiceNumber,
			OldestUnpaidAt:        oldestUnpaidAt,
			OldestUnpaidDays:      oldestUnpaidDays,
			LastPaymentAt:         lastPaymentAt,
			AgingBucket:           computeAgingBucket(oldestUnpaidDays),
			RiskLevel:             computeRiskLevel(row.Outstanding, oldestUnpaidDays),
			AssignedTo:            assignedToProp.AssignedTo,
			AssignmentExpiresAt:   &assignedToProp.AssignmentExpiresAt,
			PublicToken:           decryptToken(s.encKey, row.TokenHash.String),
			Assignment:            assignmentPtr,
		})

	}

	issues := make([]domain.PaymentIssue, 0, len(paymentRows))
	for _, row := range paymentRows {
		var lastAttempt *time.Time
		if row.LastAttempt.Valid {
			occurred := row.LastAttempt.Time.UTC()
			lastAttempt = &occurred
		}
		assignedToProp := domain.Assignment{}
		if row.AssignedTo.Valid {
			assignedAtVal := time.Time{}
			if row.AssignedAt.Valid {
				assignedAtVal = row.AssignedAt.Time
			}
			assignedToProp = assignmentFields(
				row.AssignedTo,
				assignedAtVal,
				row.AssignmentExpiresAt,
				row.Status.String,
				row.ReleasedAt,
				row.ReleasedBy,
				row.ReleaseReason,
				row.BreachedAt,
				row.BreachLevel,
				row.LastActionAt,
				now,
			)
		} else {
			assignedToProp = assignmentFields(
				row.AssignedTo,
				time.Time{}, // no assigned at
				row.AssignmentExpiresAt,
				"", // no status
				sql.NullTime{},
				sql.NullString{},
				sql.NullString{},
				sql.NullTime{},
				sql.NullString{},
				sql.NullTime{},
				now,
			)
		}

		assignmentPtr := &assignedToProp
		if assignedToProp.AssignedTo == "" && assignedToProp.Status == "" {
			assignmentPtr = nil
		}

		issues = append(issues, domain.PaymentIssue{
			CustomerID:          row.CustomerID.String(),
			CustomerName:        row.CustomerName,
			IssueType:           row.IssueType,
			LastAttempt:         lastAttempt,
			AssignedTo:          assignedToProp.AssignedTo,
			AssignmentExpiresAt: &assignedToProp.AssignmentExpiresAt,
			Assignment:          assignmentPtr,
		})
	}

	return domain.BillingOperationsResponse{
		Currency: currency,
		Summary: domain.ActionSummary{
			CustomersWithOutstanding: summary.CustomersWithOutstanding,
			OverdueInvoices:          summary.OverdueInvoices,
			FailedPaymentAttempts:    summary.FailedPaymentAttempts,
			TotalOutstanding:         summary.TotalOutstanding,
			Currency:                 currency,
		},
		CriticalActions: criticalActions,
		CollectionQueue: queue,
		PaymentIssues:   issues,
		GeneratedAt:     now,
	}, nil
}

func (s *Service) RecordAction(ctx context.Context, req domain.RecordActionRequest) (domain.RecordActionResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.RecordActionResponse{}, domain.ErrInvalidOrganization
	}

	entityType := strings.TrimSpace(req.EntityType)
	if entityType != domain.EntityTypeInvoice && entityType != domain.EntityTypeCustomer {
		return domain.RecordActionResponse{}, domain.ErrInvalidEntityType
	}

	actionType := strings.TrimSpace(req.ActionType)
	if actionType != domain.ActionTypeFollowUp &&
		actionType != domain.ActionTypeRetryPayment &&
		actionType != domain.ActionTypeMarkReviewed {
		return domain.RecordActionResponse{}, domain.ErrInvalidActionType
	}

	entityID, err := parseSnowflakeID(req.EntityID)
	if err != nil {
		return domain.RecordActionResponse{}, domain.ErrInvalidEntityID
	}

	idempotencyKey := normalizeIdempotencyKey(req.IdempotencyKey)
	if req.IdempotencyKey != "" && idempotencyKey == "" {
		return domain.RecordActionResponse{}, domain.ErrInvalidIdempotencyKey
	}

	now := s.clock.Now().UTC()
	bucket := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	actionID := s.genID.Generate()

	beforeSnapshot, err := s.loadEntitySnapshot(ctx, orgID, entityType, entityID, now)
	if err != nil {
		return domain.RecordActionResponse{}, err
	}
	afterSnapshot := beforeSnapshot

	metadata := datatypes.JSONMap{
		"entity_type":   entityType,
		"entity_id":     entityID.String(),
		"action_type":   actionType,
		"action_bucket": bucket.Format("2006-01-02"),
		"before":        beforeSnapshot,
		"after":         afterSnapshot,
	}
	for key, value := range req.Metadata {
		if strings.TrimSpace(key) == "" {
			continue
		}
		metadata[key] = value
	}

	actorType, actorID := auditcontext.ActorFromContext(ctx)

	inserted, err := s.insertBillingAction(ctx, domain.BillingActionRecord{
		ID:             actionID,
		OrgID:          orgID,
		EntityType:     entityType,
		EntityID:       entityID,
		ActionType:     actionType,
		ActionBucket:   bucket,
		IdempotencyKey: idempotencyKey,
		Metadata:       metadata,
		ActorType:      actorType,
		ActorID:        actorID,
		CreatedAt:      now,
	})
	if err != nil {
		return domain.RecordActionResponse{}, err
	}

	actionStatus := domain.ActionStatusRecorded
	resolvedActionID := actionID.String()
	if !inserted {
		actionStatus = domain.ActionStatusDuplicate
		resolvedActionID = ""
		if idempotencyKey != "" {
			existing, err := s.findActionByIdempotencyKey(ctx, orgID, idempotencyKey)
			if err == nil && existing != nil {
				resolvedActionID = existing.ID.String()
			}
		} else {
			existing, err := s.findActionByBucket(ctx, orgID, entityType, entityID, actionType, bucket)
			if err == nil && existing != nil {
				resolvedActionID = existing.ID.String()
			}
		}
	}

	// Update assignment status if needed
	if inserted && actionType != domain.ActionTypeClaim && actionType != domain.ActionTypeRelease {
		// Fire-and-forget update to assignment
		// Using a separate goroutine or just ignoring error for now to keep latency low?
		// Better to do it inline transactionally if we want strong consistency,
		// but RecordAction isn't transactional with insertBillingAction right now.
		// Let's do a quick update.
		if err := s.db.WithContext(ctx).Exec(
			`UPDATE billing_operation_assignments
			 SET status = ?, last_action_at = ?, updated_at = ?
			 WHERE org_id = ? AND entity_type = ? AND entity_id = ? 
			   AND status = ?`,
			domain.AssignmentStatusInProgress, now, now,
			orgID, entityType, entityID,
			domain.AssignmentStatusAssigned,
		).Error; err != nil {
			s.log.Warn("failed to update assignment status on action", zap.Error(err))
		}
	}

	if s.auditSvc != nil {
		targetID := resolvedActionID
		if targetID == "" {
			targetID = actionID.String()
		}
		if err := s.auditSvc.AuditLog(ctx, &orgID, "", nil, buildAuditAction(actionType), "billing_operation_action", &targetID, map[string]any{
			"entity_type":   entityType,
			"entity_id":     entityID.String(),
			"action_type":   actionType,
			"action_bucket": bucket.Format("2006-01-02"),
			"status":        actionStatus,
			"before":        beforeSnapshot,
			"after":         afterSnapshot,
		}); err != nil {
			return domain.RecordActionResponse{}, err
		}
	}

	return domain.RecordActionResponse{
		ActionID:   resolvedActionID,
		Status:     actionStatus,
		RecordedAt: now,
	}, nil
}

func (s *Service) ClaimAssignment(
	ctx context.Context,
	req domain.ClaimAssignmentRequest,
) (domain.AssignmentResponse, error) {

	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.AssignmentResponse{}, domain.ErrInvalidOrganization
	}

	entityType := strings.TrimSpace(req.EntityType)
	if entityType != domain.EntityTypeInvoice && entityType != domain.EntityTypeCustomer {
		return domain.AssignmentResponse{}, domain.ErrInvalidEntityType
	}

	entityID, err := parseSnowflakeID(req.EntityID)
	if err != nil {
		return domain.AssignmentResponse{}, domain.ErrInvalidEntityID
	}

	assignedTo := strings.TrimSpace(req.AssignedTo)
	if assignedTo == "" {
		_, actorID := auditcontext.ActorFromContext(ctx)
		assignedTo = strings.TrimSpace(actorID)
	}
	if assignedTo == "" {
		return domain.AssignmentResponse{}, domain.ErrInvalidAssignee
	}

	ttlMinutes := req.AssignmentTTLMinutes
	if ttlMinutes == 0 {
		ttlMinutes = 120
	}
	if ttlMinutes < 0 {
		return domain.AssignmentResponse{}, domain.ErrInvalidAssignmentTTL
	}

	now := s.clock.Now().UTC()
	expiresAt := now.Add(time.Duration(ttlMinutes) * time.Minute)

	var result *domain.AssignmentResponse
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		existing, err := s.loadAssignmentForUpdate(
			ctx, tx, orgID, entityType, entityID,
		)
		if err != nil {
			return err
		}

		if existing != nil && existing.Status != domain.AssignmentStatusReleased {
			// Already assigned
			if existing.AssignedTo != assignedTo {
				return domain.ErrAssignmentConflict
			}
			// Same user, extend expiry
			record := *existing
			record.AssignmentExpiresAt = expiresAt
			record.UpdatedAt = now

			if err := s.upsertAssignmentTx(ctx, tx, record); err != nil {
				return err
			}

			result = &domain.AssignmentResponse{
				Assignment: domain.Assignment{
					EntityType:          entityType,
					EntityID:            entityID.String(),
					Status:              existing.Status,
					AssignedTo:          assignedTo,
					AssignedAt:          existing.AssignedAt,
					AssignmentExpiresAt: expiresAt,
					LastActionAt:        timePtr(existing.LastActionAt),
				},
				Status: domain.AssignmentStatusAssigned,
			}
			return nil
		}

		// ✅ claim / insert new
		// Capture entity snapshot for task stability
		snapshot, err := s.loadEntitySnapshot(ctx, orgID, entityType, entityID, now)
		if err != nil {
			s.log.Warn("failed to load entity snapshot", zap.Error(err))
			snapshot = make(map[string]interface{})
		}

		snapshotJSON, err := json.Marshal(snapshot)
		if err != nil {
			s.log.Warn("failed to marshal snapshot", zap.Error(err))
			snapshotJSON = []byte("{}")
		}

		record := domain.BillingAssignmentRecord{
			ID:                  s.genID.Generate(),
			OrgID:               orgID,
			EntityType:          entityType,
			EntityID:            entityID,
			AssignedTo:          assignedTo,
			AssignedAt:          now,
			AssignmentExpiresAt: expiresAt,
			Status:              domain.AssignmentStatusAssigned,
			SnapshotMetadata:    datatypes.JSON(snapshotJSON),
			CreatedAt:           now,
			UpdatedAt:           now,
		}

		if err := s.upsertAssignmentTx(ctx, tx, record); err != nil {
			return err
		}

		result = &domain.AssignmentResponse{
			Assignment: domain.Assignment{
				EntityType:          entityType,
				EntityID:            entityID.String(),
				Status:              domain.AssignmentStatusAssigned,
				AssignedTo:          assignedTo,
				AssignedAt:          now,
				AssignmentExpiresAt: expiresAt,
			},
			Status: domain.AssignmentStatusAssigned,
		}

		// Record claim action
		actionID := s.genID.Generate()
		bucket := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		if _, err := s.insertBillingAction(ctx, domain.BillingActionRecord{
			ID:           actionID,
			OrgID:        orgID,
			EntityType:   entityType,
			EntityID:     entityID,
			ActionType:   domain.ActionTypeClaim,
			ActionBucket: bucket,
			Metadata: datatypes.JSONMap{
				"assignment_id": record.ID.String(),
				"expires_at":    expiresAt,
			},
			ActorType: "user", // Should ideally come from context
			ActorID:   assignedTo,
			CreatedAt: now,
		}); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return domain.AssignmentResponse{}, err
	}
	if result == nil {
		return domain.AssignmentResponse{}, fmt.Errorf("internal error: result not set in transaction")
	}

	if s.auditSvc != nil && result.Status == domain.AssignmentStatusAssigned {
		targetID := result.Assignment.EntityID
		_ = s.auditSvc.AuditLog(ctx, &orgID, "", nil,
			"billing_operations.assignment.claimed",
			"billing_operation_assignment",
			&targetID,
			map[string]any{
				"entity_type": entityType,
				"entity_id":   entityID.String(),
				"assigned_to": assignedTo,
				"expires_at":  expiresAt.Format(time.RFC3339),
			},
		)
	}

	return *result, nil
}

func (s *Service) ReleaseAssignment(ctx context.Context, req domain.ReleaseAssignmentRequest) error {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.ErrInvalidOrganization
	}

	entityType := strings.TrimSpace(req.EntityType)
	if entityType != domain.EntityTypeInvoice && entityType != domain.EntityTypeCustomer {
		return domain.ErrInvalidEntityType
	}

	entityID, err := parseSnowflakeID(req.EntityID)
	if err != nil {
		return domain.ErrInvalidEntityID
	}

	releasedBy := strings.TrimSpace(req.ReleasedBy)
	if releasedBy == "" {
		_, actorID := auditcontext.ActorFromContext(ctx)
		releasedBy = strings.TrimSpace(actorID)
	}
	if releasedBy == "" {
		return domain.ErrInvalidAssignee
	}

	now := s.clock.Now().UTC()

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := s.loadAssignmentForUpdate(ctx, tx, orgID, entityType, entityID)
		if err != nil {
			return err
		}
		if existing == nil || existing.Status == domain.AssignmentStatusReleased {
			return nil // Already not assigned or released
		}

		existing.Status = domain.AssignmentStatusReleased
		existing.ReleasedAt = sql.NullTime{Time: now, Valid: true}
		existing.ReleasedBy = sql.NullString{String: releasedBy, Valid: true}
		existing.ReleaseReason = sql.NullString{String: req.Reason, Valid: true}
		existing.ResolvedAt = sql.NullTime{Time: now, Valid: true}
		existing.ResolvedBy = sql.NullString{String: releasedBy, Valid: true}
		existing.UpdatedAt = now

		if err := s.upsertAssignmentTx(ctx, tx, *existing); err != nil {
			return err
		}

		// Record release action
		actionID := s.genID.Generate()
		bucket := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		snapshot, _ := s.loadEntitySnapshot(ctx, orgID, entityType, entityID, now)

		if _, err := s.insertBillingAction(ctx, domain.BillingActionRecord{
			ID:           actionID,
			OrgID:        orgID,
			EntityType:   entityType,
			EntityID:     entityID,
			ActionType:   domain.ActionTypeRelease,
			ActionBucket: bucket,
			Metadata: datatypes.JSONMap{
				"assignment_id": existing.ID.String(),
				"released_by":   releasedBy,
				"reason":        req.Reason,
				"snapshot":      snapshot,
			},
			ActorType: "user",
			ActorID:   releasedBy,
			CreatedAt: now,
		}); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	if s.auditSvc != nil {
		targetID := entityID.String()
		_ = s.auditSvc.AuditLog(ctx, &orgID, "", nil,
			"billing_operations.assignment.released",
			"billing_operation_assignment",
			&targetID,
			map[string]any{
				"entity_type": entityType,
				"entity_id":   entityID.String(),
				"released_by": releasedBy,
				"reason":      req.Reason,
			},
		)
	}

	return nil
}

func (s *Service) ResolveAssignment(ctx context.Context, req domain.ResolveAssignmentRequest) error {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.ErrInvalidOrganization
	}

	entityType := strings.TrimSpace(req.EntityType)
	if entityType != domain.EntityTypeInvoice && entityType != domain.EntityTypeCustomer {
		return domain.ErrInvalidEntityType
	}

	entityID, err := parseSnowflakeID(req.EntityID)
	if err != nil {
		return domain.ErrInvalidEntityID
	}

	resolvedBy := strings.TrimSpace(req.ResolvedBy)
	if resolvedBy == "" {
		_, actorID := auditcontext.ActorFromContext(ctx)
		resolvedBy = strings.TrimSpace(actorID)
	}
	if resolvedBy == "" {
		return domain.ErrInvalidAssignee
	}

	now := s.clock.Now().UTC()

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := s.loadAssignmentForUpdate(ctx, tx, orgID, entityType, entityID)
		if err != nil {
			return err
		}
		if existing == nil {
			return nil // No assignment to resolve
		}
		if existing.Status == domain.AssignmentStatusResolved {
			return nil // Already resolved
		}

		// Update to resolved status
		existing.Status = domain.AssignmentStatusResolved
		existing.ResolvedAt = sql.NullTime{Time: now, Valid: true}
		existing.ResolvedBy = sql.NullString{String: resolvedBy, Valid: true}
		existing.ReleaseReason = sql.NullString{String: req.Resolution, Valid: true}
		existing.UpdatedAt = now

		if err := tx.Save(existing).Error; err != nil {
			return err
		}

		// Record resolve action
		actionID := s.genID.Generate()
		bucket := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		if _, err := s.insertBillingAction(ctx, domain.BillingActionRecord{
			ID:           actionID,
			OrgID:        orgID,
			EntityType:   entityType,
			EntityID:     entityID,
			ActionType:   domain.ActionTypeResolve,
			ActionBucket: bucket,
			Metadata: datatypes.JSONMap{
				"assignment_id": existing.ID.String(),
				"resolution":    req.Resolution,
				"resolved_by":   resolvedBy,
			},
			ActorType: "user",
			ActorID:   resolvedBy,
			CreatedAt: now,
		}); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Audit log
	if s.auditSvc != nil {
		targetID := entityID.String()
		_ = s.auditSvc.AuditLog(ctx, &orgID, "", nil,
			"billing_operations.assignment.resolved",
			"billing_operation_assignment",
			&targetID,
			map[string]any{
				"entity_type": entityType,
				"entity_id":   entityID.String(),
				"resolution":  req.Resolution,
				"resolved_by": resolvedBy,
			},
		)
	}

	return nil
}

func (s *Service) loadAssignmentForUpdate(
	ctx context.Context,
	tx *gorm.DB,
	orgID snowflake.ID,
	entityType string,
	entityID snowflake.ID,
) (*domain.BillingAssignmentRecord, error) {
	var row domain.BillingAssignmentRecord
	query := `SELECT id, org_id, entity_type, entity_id,
		        assigned_to, assigned_at, assignment_expires_at,
		        status, released_at, released_by, release_reason, last_action_at,
				snapshot_metadata, created_at, updated_at
		 FROM billing_operation_assignments
		 WHERE org_id = ? AND entity_type = ? AND entity_id = ?`

	if tx.Dialector.Name() != "sqlite" {
		query += " FOR UPDATE"
	}

	err := tx.WithContext(ctx).Raw(query, orgID, entityType, entityID).Scan(&row).Error

	if err != nil {
		return nil, err
	}
	if row.AssignedTo == "" {
		return nil, nil
	}
	return &row, nil
}

func timePtr(t sql.NullTime) *time.Time {
	if t.Valid {
		val := t.Time.UTC()
		return &val
	}
	return nil
}

func (s *Service) EvaluateSLAs(ctx context.Context) error {
	const (
		initialResponseSLA = 30 * time.Minute
		idleActionSLA      = 60 * time.Minute
	)
	now := s.clock.Now().UTC()

	var records []domain.BillingAssignmentRecord
	// Find active assignments that are NOT already escalated
	if err := s.db.WithContext(ctx).Where("status IN ? AND breached_at IS NULL",
		[]string{domain.AssignmentStatusAssigned, domain.AssignmentStatusInProgress}).
		Find(&records).Error; err != nil {
		return err
	}

	for _, rec := range records {
		isBreached := false
		breachType := ""

		// Check Initial Response SLA (assigned -> first action)
		if rec.Status == domain.AssignmentStatusAssigned {
			if now.Sub(rec.AssignedAt) > initialResponseSLA {
				isBreached = true
				breachType = "initial_response"
			}
		}

		// Check Idle Action SLA (last_action -> now)
		if !isBreached && rec.LastActionAt.Valid {
			if now.Sub(rec.LastActionAt.Time) > idleActionSLA {
				isBreached = true
				breachType = "idle_action"
			}
		}

		if isBreached {
			// Escalate in transaction
			err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				// 1. Update Assignment
				if err := tx.Model(&domain.BillingAssignmentRecord{}).
					Where("id = ?", rec.ID).
					Updates(map[string]interface{}{
						"status":       domain.AssignmentStatusEscalated,
						"breached_at":  now,
						"breach_level": breachType,
						"resolved_at":  now,
						"resolved_by":  "system",
						"updated_at":   now,
					}).Error; err != nil {
					return err
				}

				// 2. Record Breach Action
				actionID := s.genID.Generate()
				bucket := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

				metadata := datatypes.JSONMap{
					"assignment_id": rec.ID.String(),
					"breach_type":   breachType,
					"minutes_idle":  0,
				}
				if rec.LastActionAt.Valid {
					metadata["minutes_idle"] = int(now.Sub(rec.LastActionAt.Time).Minutes())
				} else {
					metadata["minutes_since_assigned"] = int(now.Sub(rec.AssignedAt).Minutes())
				}

				return tx.Create(&domain.BillingActionRecord{
					ID:           actionID,
					OrgID:        rec.OrgID,
					EntityType:   rec.EntityType,
					EntityID:     rec.EntityID,
					ActionType:   "sla_breached",
					ActionBucket: bucket,
					Metadata:     metadata,
					ActorType:    "system",
					ActorID:      "sla_monitor",
					CreatedAt:    now,
				}).Error
			})

			if err != nil {
				s.log.Error("failed to escalate assignment",
					zap.String("assignment_id", rec.ID.String()),
					zap.Error(err))
				continue
			}

			// Audit Log
			if s.auditSvc != nil {
				targetID := rec.EntityID.String()
				_ = s.auditSvc.AuditLog(ctx, &rec.OrgID, "system", nil,
					"billing_operations.assignment.escalated",
					"billing_operation_assignment",
					&targetID,
					map[string]any{
						"breach_type":   breachType,
						"assignment_id": rec.ID.String(),
					})
			}
		}
	}
	return nil
}

type overdueInvoiceRow struct {
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

type outstandingCustomerRow struct {
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

type paymentIssueRow struct {
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

type collectionQueueRow struct {
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

type failedPaymentActionRow struct {
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

type actionSummaryRow struct {
	CustomersWithOutstanding int   `gorm:"column:customers_with_outstanding"`
	OverdueInvoices          int   `gorm:"column:overdue_invoices"`
	FailedPaymentAttempts    int   `gorm:"column:failed_payment_attempts"`
	TotalOutstanding         int64 `gorm:"column:total_outstanding"`
}

type assignmentRow struct {
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

func (s *Service) listOverdueInvoices(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	now time.Time,
	limit int,
) ([]overdueInvoiceRow, error) {
	var rows []overdueInvoiceRow
	query := `
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		)
		SELECT
			i.id AS invoice_id,
			COALESCE(i.invoice_number::text, '') AS invoice_number,
			c.id AS customer_id,
			c.name AS customer_name,
			GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS amount_due,
			i.due_at AS due_at,
			boa.assigned_to AS assigned_to,
			boa.assigned_at AS assigned_at,
			boa.assignment_expires_at AS assignment_expires_at,
			boa.status AS assignment_status,
			boa.released_at AS assignment_released_at,
			boa.released_by AS assignment_released_by,
			boa.release_reason AS assignment_release_reason,
			boa.last_action_at AS assignment_last_action_at,
			ipt.token_hash AS token_hash
		FROM invoices i

		JOIN customers c ON c.id = i.customer_id
		LEFT JOIN settled s ON s.invoice_id_text = i.id::text
		LEFT JOIN invoice_public_tokens ipt ON ipt.invoice_id = i.id AND ipt.revoked_at IS NULL
		LEFT JOIN billing_operation_assignments boa
			ON boa.org_id = ?

			AND boa.entity_type = ?
			AND boa.entity_id = i.id
			AND boa.status != 'released'
		WHERE i.org_id = ?
		  AND i.status = 'FINALIZED'
		  AND i.voided_at IS NULL
		  AND i.paid_at IS NULL
		  AND i.currency = ?
		  AND i.due_at IS NOT NULL
		  AND i.due_at < ?
		  AND GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) > 0
		ORDER BY i.due_at ASC
		LIMIT ?`

	if err := s.db.WithContext(ctx).Raw(
		query,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		domain.EntityTypeInvoice,
		orgID,
		currency,
		now,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) listOutstandingCustomers(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	now time.Time,
	limit int,
) ([]outstandingCustomerRow, error) {
	var rows []outstandingCustomerRow
	query := `
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		), invoice_outstanding AS (
			SELECT
				i.id AS invoice_id,
				i.customer_id,
				COALESCE(i.invoice_number::text, '') AS invoice_number,
				i.due_at,
				GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS outstanding
			FROM invoices i
			LEFT JOIN settled s ON s.invoice_id_text = i.id::text
			WHERE i.org_id = ?
			  AND i.status = 'FINALIZED'
			  AND i.voided_at IS NULL
			  AND i.currency = ?
			  AND i.currency = ?
			  AND i.currency = ?
			  AND i.currency = ?
			  AND i.currency = ?
		), totals AS (
			SELECT customer_id, SUM(outstanding) AS outstanding
			FROM invoice_outstanding
			WHERE outstanding > 0
			GROUP BY customer_id
		), oldest_overdue AS (
			SELECT DISTINCT ON (customer_id)
				customer_id,
				invoice_id,
				invoice_number,
				due_at
			FROM invoice_outstanding
			WHERE outstanding > 0 AND due_at IS NOT NULL AND due_at < ?
			ORDER BY customer_id, due_at ASC, invoice_id ASC
		), last_payment AS (
			SELECT customer_id, MAX(received_at) AS last_payment_at
			FROM payment_events
			WHERE org_id = ? AND event_type = 'payment_succeeded'
			GROUP BY customer_id
		)
		SELECT
			c.id AS customer_id,
			c.name AS customer_name,
			t.outstanding AS outstanding,
			oo.invoice_id::text AS oldest_overdue_invoice_id,
			oo.invoice_number AS oldest_overdue_invoice_number,
			oo.due_at AS oldest_overdue_at,
			lp.last_payment_at AS last_payment_at,
			ipt.token_hash AS token_hash,
			boa.assigned_to AS assigned_to,
			boa.assigned_at AS assigned_at,
			boa.assignment_expires_at AS assignment_expires_at,
			boa.status AS assignment_status,
			boa.released_at AS assignment_released_at,
			boa.released_by AS assignment_released_by,
			boa.release_reason AS assignment_release_reason,
			boa.last_action_at AS assignment_last_action_at
		FROM totals t

		JOIN customers c ON c.id = t.customer_id
		LEFT JOIN oldest_overdue oo ON oo.customer_id = t.customer_id
		LEFT JOIN invoice_public_tokens ipt ON ipt.invoice_id = oo.invoice_id AND ipt.revoked_at IS NULL
		LEFT JOIN last_payment lp ON lp.customer_id = t.customer_id
		LEFT JOIN billing_operation_assignments boa
			ON boa.org_id = ?
			AND boa.entity_type = ?
			AND boa.entity_id = c.id
			AND boa.status != 'released'
		WHERE c.org_id = ?
		ORDER BY t.outstanding DESC
		LIMIT ?`

	if err := s.db.WithContext(ctx).Raw(
		query,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		currency,
		now,
		orgID,
		domain.EntityTypeCustomer,
		orgID,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) loadActionSummary(ctx context.Context, orgID snowflake.ID, currency string, now time.Time) (actionSummaryRow, error) {
	var row actionSummaryRow
	query := `
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		), invoice_outstanding AS (
			SELECT
				i.id AS invoice_id,
				i.customer_id,
				i.due_at,
				GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS outstanding
			FROM invoices i
			LEFT JOIN settled s ON s.invoice_id_text = i.id::text
			WHERE i.org_id = ?
			  AND i.status = 'FINALIZED'
			  AND i.voided_at IS NULL
			  AND i.currency = ?
		), totals AS (
			SELECT customer_id, SUM(outstanding) AS outstanding
			FROM invoice_outstanding
			WHERE outstanding > 0
			GROUP BY customer_id
		)
		SELECT
			COALESCE((SELECT COUNT(*) FROM totals), 0) AS customers_with_outstanding,
			COALESCE((SELECT COUNT(*) FROM invoice_outstanding WHERE outstanding > 0 AND due_at IS NOT NULL AND due_at < ?), 0) AS overdue_invoices,
			COALESCE((SELECT COUNT(*) FROM payment_events WHERE org_id = ? AND event_type = ?), 0) AS failed_payment_attempts,
			COALESCE((SELECT SUM(outstanding) FROM totals), 0) AS total_outstanding`

	if err := s.db.WithContext(ctx).Raw(
		query,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		currency,
		now,
		orgID,
		paymentdomain.EventTypePaymentFailed,
	).Scan(&row).Error; err != nil {
		return actionSummaryRow{}, err
	}
	return row, nil
}

func (s *Service) listCollectionQueue(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	now time.Time,
	limit int,
) ([]collectionQueueRow, error) {
	var rows []collectionQueueRow
	query := `
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		), invoice_outstanding AS (
			SELECT
				i.id AS invoice_id,
				i.customer_id,
				COALESCE(i.invoice_number::text, '') AS invoice_number,
				i.due_at,
				COALESCE(i.issued_at, i.created_at) AS issued_at,
				GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS outstanding
			FROM invoices i
			LEFT JOIN settled s ON s.invoice_id_text = i.id::text
			WHERE i.org_id = ?
			  AND i.status = 'FINALIZED'
			  AND i.voided_at IS NULL
			  AND i.currency = ?
		), totals AS (
			SELECT customer_id, SUM(outstanding) AS outstanding
			FROM invoice_outstanding
			WHERE outstanding > 0
			GROUP BY customer_id
		), oldest_unpaid AS (
			SELECT DISTINCT ON (customer_id)
				customer_id,
				invoice_id,
				invoice_number,
				due_at,
				issued_at
			FROM invoice_outstanding
			WHERE outstanding > 0
			ORDER BY customer_id, COALESCE(due_at, issued_at) ASC, invoice_id ASC
		), last_payment AS (
			SELECT customer_id, MAX(received_at) AS last_payment_at
			FROM payment_events
			WHERE org_id = ? AND event_type = 'payment_succeeded'
			GROUP BY customer_id
		)
		SELECT
			c.id AS customer_id,
			c.name AS customer_name,
			t.outstanding AS outstanding,
			ou.invoice_id::text AS oldest_unpaid_invoice_id,
			ou.invoice_number AS oldest_unpaid_invoice_number,
			ou.due_at AS oldest_unpaid_at,
			lp.last_payment_at AS last_payment_at,
			boa.assigned_to AS assigned_to,
			boa.assigned_at AS assigned_at,
			boa.assignment_expires_at AS assignment_expires_at,
			boa.status AS assignment_status,
			boa.released_at AS assignment_released_at,
			boa.released_by AS assignment_released_by,
			boa.release_reason AS assignment_release_reason,
			boa.last_action_at AS assignment_last_action_at,
			ipt.token_hash AS token_hash
		FROM totals t
		JOIN customers c ON c.id = t.customer_id
		LEFT JOIN oldest_unpaid ou ON ou.customer_id = t.customer_id
		LEFT JOIN invoice_public_tokens ipt ON ipt.invoice_id = ou.invoice_id AND ipt.revoked_at IS NULL
		LEFT JOIN last_payment lp ON lp.customer_id = t.customer_id
		LEFT JOIN billing_operation_assignments boa
			ON boa.org_id = ?
			AND boa.entity_type = ?
			AND boa.entity_id = c.id
			AND boa.status != 'released'
		WHERE c.org_id = ?
		ORDER BY
			CASE
				WHEN ou.due_at IS NULL THEN 1
				WHEN ou.due_at <= (?::timestamptz - interval '60 days') THEN 3
				WHEN ou.due_at <= (?::timestamptz - interval '31 days') THEN 2
				ELSE 1
			END DESC,
			t.outstanding DESC,
			c.id ASC
		LIMIT ?`

	if err := s.db.WithContext(ctx).Raw(
		query,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		currency,
		orgID,
		orgID,
		domain.EntityTypeCustomer,
		orgID,
		now,
		now,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) listFailedPaymentActions(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	now time.Time,
	limit int,
) ([]failedPaymentActionRow, error) {
	var rows []failedPaymentActionRow
	query := `
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		), failed AS (
			SELECT
				pe.customer_id AS customer_id,
				c.name AS customer_name,
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				MAX(pe.received_at) AS last_attempt
			FROM payment_events pe
			JOIN customers c ON c.id = pe.customer_id
			WHERE pe.org_id = ?
			  AND pe.event_type = ?
			GROUP BY pe.customer_id, c.name, invoice_id_text
		)
		SELECT
			f.customer_id AS customer_id,
			f.customer_name AS customer_name,
			f.invoice_id_text AS invoice_id,
			COALESCE(i.invoice_number::text, '') AS invoice_number,
			GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS amount_due,
			i.due_at AS due_at,
			f.last_attempt AS last_attempt,
			boa.assigned_to AS assigned_to,
			boa.assigned_at AS assigned_at,
			boa.assignment_expires_at AS assignment_expires_at,
			boa.status AS assignment_status,
			boa.released_at AS assignment_released_at,
			boa.released_by AS assignment_released_by,
			boa.release_reason AS assignment_release_reason,
			boa.last_action_at AS assignment_last_action_at,
			ipt.token_hash AS token_hash
		FROM failed f

		LEFT JOIN invoices i
			ON i.id::text = f.invoice_id_text
			AND i.org_id = ?
			AND i.status = 'FINALIZED'
			AND i.voided_at IS NULL
			AND i.currency = ?
		LEFT JOIN settled s ON s.invoice_id_text = i.id::text
		LEFT JOIN invoice_public_tokens ipt ON ipt.invoice_id = i.id AND ipt.revoked_at IS NULL
		LEFT JOIN billing_operation_assignments boa
			ON boa.org_id = ?
			AND boa.entity_type = ?

			AND boa.entity_id = f.customer_id
			AND boa.status != 'released'
		WHERE (i.id IS NULL OR GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) > 0)
		ORDER BY f.last_attempt DESC
		LIMIT ?`

	if err := s.db.WithContext(ctx).Raw(
		query,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		paymentdomain.EventTypePaymentFailed,
		orgID,
		currency,
		orgID,
		domain.EntityTypeCustomer,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) listPaymentIssues(ctx context.Context, orgID snowflake.ID, now time.Time, limit int) ([]paymentIssueRow, error) {
	var rows []paymentIssueRow
	query := `
		SELECT
			pe.customer_id AS customer_id,
			c.name AS customer_name,
			pe.event_type AS issue_type,
			MAX(pe.received_at) AS last_attempt,
			boa.assigned_to AS assigned_to,
			boa.assigned_at AS assigned_at,
			boa.assignment_expires_at AS assignment_expires_at,
			boa.status AS assignment_status,
			boa.released_at AS assignment_released_at,
			boa.released_by AS assignment_released_by,
			boa.release_reason AS assignment_release_reason,
			boa.last_action_at AS assignment_last_action_at
		FROM payment_events pe
		JOIN customers c ON c.id = pe.customer_id
		LEFT JOIN billing_operation_assignments boa
			ON boa.org_id = ?
			AND boa.entity_type = ?
			AND boa.entity_id = pe.customer_id
			AND boa.status != 'released'
		WHERE pe.org_id = ?
		  AND pe.event_type = ?
		GROUP BY pe.customer_id, c.name, pe.event_type, boa.assigned_to, boa.assigned_at, boa.assignment_expires_at, boa.status, boa.released_at, boa.released_by, boa.release_reason, boa.last_action_at
		ORDER BY last_attempt DESC
		LIMIT ?`

	if err := s.db.WithContext(ctx).Raw(
		query,
		orgID,
		domain.EntityTypeCustomer,
		orgID,
		paymentdomain.EventTypePaymentFailed,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) loadOrgCurrency(ctx context.Context, orgID snowflake.ID) (string, error) {
	var row struct {
		Currency string `gorm:"column:currency"`
	}
	if err := s.db.WithContext(ctx).Raw(
		`SELECT currency FROM organization_billing_preferences WHERE org_id = ? LIMIT 1`,
		orgID,
	).Scan(&row).Error; err != nil {
		return "", err
	}
	currency := strings.ToUpper(strings.TrimSpace(row.Currency))
	if currency == "" {
		currency = "USD"
	}
	return currency, nil
}

func assignmentFields(
	assignedTo sql.NullString,
	assignedAt time.Time,
	expiresAt sql.NullTime,
	status string,
	releasedAt sql.NullTime,
	releasedBy sql.NullString,
	releaseReason sql.NullString,
	breachedAt sql.NullTime,
	breachLevel sql.NullString,
	lastActionAt sql.NullTime,
	now time.Time,
) domain.Assignment {
	assigned := strings.TrimSpace(assignedTo.String)
	if !assignedTo.Valid {
		assigned = ""
	}
	expiresVal := time.Time{}
	if expiresAt.Valid {
		expiresVal = expiresAt.Time.UTC()
	}

	releaseVal := (*time.Time)(nil)
	if releasedAt.Valid {
		t := releasedAt.Time.UTC()
		releaseVal = &t
	}

	breachVal := (*time.Time)(nil)
	if breachedAt.Valid {
		t := breachedAt.Time.UTC()
		breachVal = &t
	}

	finalStatus := status
	if finalStatus == "" {
		if assigned != "" {
			finalStatus = domain.AssignmentStatusAssigned
		}
	}

	// Compute SLA
	slaStatus := ""
	timeSinceAssigned := ""

	if assigned != "" && finalStatus != domain.AssignmentStatusReleased {
		referenceTime := assignedAt
		if lastActionAt.Valid {
			referenceTime = lastActionAt.Time
		}

		minutesSince := int(now.Sub(referenceTime).Minutes())
		if minutesSince < 0 {
			minutesSince = 0
		}

		switch {
		case minutesSince < 30:
			slaStatus = domain.SLAFresh
		case minutesSince < 90:
			slaStatus = domain.SLAActive
		case minutesSince < 240:
			slaStatus = domain.SLAAging
		default:
			slaStatus = domain.SLAStale
		}

		// Human readable duration
		duration := now.Sub(assignedAt)
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		if hours > 0 {
			timeSinceAssigned = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			timeSinceAssigned = fmt.Sprintf("%dm", minutes)
		}
	}

	return domain.Assignment{
		Status:              finalStatus,
		AssignedTo:          assigned,
		AssignedAt:          assignedAt,
		AssignmentExpiresAt: expiresVal,
		ReleasedAt:          releaseVal,
		ReleasedBy:          strings.TrimSpace(releasedBy.String),
		ReleaseReason:       strings.TrimSpace(releaseReason.String),
		BreachedAt:          breachVal,
		BreachLevel:         strings.TrimSpace(breachLevel.String),
		LastActionAt:        timePtr(lastActionAt),
		SLAStatus:           slaStatus,
		TimeSinceAssigned:   timeSinceAssigned,
	}
}

func computeAgingBucket(days int) string {
	switch {
	case days >= 60:
		return "60+"
	case days >= 31:
		return "31-60"
	default:
		return "0-30"
	}
}

func computeRiskLevel(outstanding int64, oldestDays int) string {
	switch {
	case oldestDays >= 60 || outstanding >= 1000000:
		return "high"
	case oldestDays >= 31 || outstanding >= 250000:
		return "medium"
	default:
		return "low"
	}
}

func normalizeIdempotencyKey(value string) string {
	return strings.TrimSpace(value)
}

func parseSnowflakeID(value string) (snowflake.ID, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0, domain.ErrInvalidEntityID
	}
	parsed, err := snowflake.ParseString(raw)
	if err != nil || parsed == 0 {
		return 0, domain.ErrInvalidEntityID
	}
	return parsed, nil
}

func buildAuditAction(actionType string) string {
	switch actionType {
	case domain.ActionTypeFollowUp:
		return "billing_operations.follow_up"
	case domain.ActionTypeRetryPayment:
		return "billing_operations.retry_payment"
	case domain.ActionTypeMarkReviewed:
		return "billing_operations.mark_reviewed"
	default:
		return "billing_operations.action"
	}
}

func (s *Service) insertBillingAction(ctx context.Context, record domain.BillingActionRecord) (bool, error) {
	if record.ID == 0 {
		return false, domain.ErrInvalidEntityID
	}
	if s.db == nil {
		return false, gorm.ErrInvalidDB
	}
	var idempotencyValue any
	if record.IdempotencyKey != "" {
		idempotencyValue = record.IdempotencyKey
	}
	metadata := record.Metadata
	if metadata == nil {
		metadata = datatypes.JSONMap{}
	}
	result := s.db.WithContext(ctx).Exec(
		`INSERT INTO billing_operation_actions (
			id, org_id, entity_type, entity_id, action_type, action_bucket,
			idempotency_key, metadata, actor_type, actor_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT DO NOTHING`,
		record.ID,
		record.OrgID,
		record.EntityType,
		record.EntityID,
		record.ActionType,
		record.ActionBucket,
		idempotencyValue,
		metadata,
		strings.TrimSpace(record.ActorType),
		strings.TrimSpace(record.ActorID),
		record.CreatedAt,
	)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (s *Service) findActionByIdempotencyKey(ctx context.Context, orgID snowflake.ID, key string) (*domain.BillingActionLookup, error) {
	var row domain.BillingActionLookup
	if err := s.db.WithContext(ctx).Raw(
		`SELECT id FROM billing_operation_actions WHERE org_id = ? AND idempotency_key = ? LIMIT 1`,
		orgID,
		key,
	).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &row, nil
}

func (s *Service) findActionByBucket(ctx context.Context, orgID snowflake.ID, entityType string, entityID snowflake.ID, actionType string, bucket time.Time) (*domain.BillingActionLookup, error) {
	var row domain.BillingActionLookup
	if err := s.db.WithContext(ctx).Raw(
		`SELECT id FROM billing_operation_actions
		 WHERE org_id = ? AND entity_type = ? AND entity_id = ? AND action_type = ? AND action_bucket = ?
		 LIMIT 1`,
		orgID,
		entityType,
		entityID,
		actionType,
		bucket,
	).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &row, nil
}

func (s *Service) loadAssignment(ctx context.Context, orgID snowflake.ID, entityType string, entityID snowflake.ID) (*assignmentRow, error) {
	var row assignmentRow
	if err := s.db.WithContext(ctx).Raw(
		`SELECT assigned_to, assigned_at, assignment_expires_at,
		        status, released_at, released_by, release_reason, last_action_at
		 FROM billing_operation_assignments
		 WHERE org_id = ? AND entity_type = ? AND entity_id = ?
		 LIMIT 1`,
		orgID,
		entityType,
		entityID,
	).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.AssignedTo == "" {
		return nil, nil
	}
	return &row, nil
}

func (s *Service) upsertAssignmentTx(
	ctx context.Context,
	tx *gorm.DB,
	record domain.BillingAssignmentRecord,
) error {

	return tx.WithContext(ctx).Exec(
		`INSERT INTO billing_operation_assignments (
			id, org_id, entity_type, entity_id,
			assigned_to, assigned_at, assignment_expires_at,
			status, released_at, released_by, release_reason, last_action_at,
			snapshot_metadata,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (org_id, entity_type, entity_id) DO UPDATE SET
			assigned_to = EXCLUDED.assigned_to,
			assigned_at = EXCLUDED.assigned_at,
			assignment_expires_at = EXCLUDED.assignment_expires_at,
			status = EXCLUDED.status,
			released_at = EXCLUDED.released_at,
			released_by = EXCLUDED.released_by,
			release_reason = EXCLUDED.release_reason,
			last_action_at = EXCLUDED.last_action_at,
			snapshot_metadata = EXCLUDED.snapshot_metadata,
			updated_at = EXCLUDED.updated_at`,
		record.ID,
		record.OrgID,
		record.EntityType,
		record.EntityID,
		record.AssignedTo,
		record.AssignedAt,
		record.AssignmentExpiresAt,
		record.Status,
		record.ReleasedAt,
		record.ReleasedBy,
		record.ReleaseReason,
		record.LastActionAt,
		record.SnapshotMetadata,
		record.CreatedAt,
		record.UpdatedAt,
	).Error
}

func (s *Service) loadEntitySnapshot(ctx context.Context, orgID snowflake.ID, entityType string, entityID snowflake.ID, now time.Time) (map[string]any, error) {
	switch entityType {
	case domain.EntityTypeInvoice:
		return s.loadInvoiceSnapshot(ctx, orgID, entityID, now)
	case domain.EntityTypeCustomer:
		return s.loadCustomerSnapshot(ctx, orgID, entityID, now)
	default:
		return nil, domain.ErrInvalidEntityType
	}
}

func (s *Service) loadInvoiceSnapshot(ctx context.Context, orgID snowflake.ID, invoiceID snowflake.ID, now time.Time) (map[string]any, error) {
	var row struct {
		InvoiceID     snowflake.ID `gorm:"column:invoice_id"`
		InvoiceNumber string       `gorm:"column:invoice_number"`
		Status        string       `gorm:"column:status"`
		CustomerID    snowflake.ID `gorm:"column:customer_id"`
		CustomerName  string       `gorm:"column:customer_name"`
		Currency      string       `gorm:"column:currency"`
		DueAt         sql.NullTime `gorm:"column:due_at"`
		AmountDue     int64        `gorm:"column:amount_due"`
	}
	query := `
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		)
		SELECT
			i.id AS invoice_id,
			COALESCE(i.invoice_number::text, '') AS invoice_number,
			i.status AS status,
			i.customer_id AS customer_id,
			c.name AS customer_name,
			i.currency AS currency,
			i.due_at AS due_at,
			GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS amount_due
		FROM invoices i
		JOIN customers c ON c.id = i.customer_id
		LEFT JOIN settled s ON s.invoice_id_text = i.id::text
		WHERE i.org_id = ? AND i.id = ?
		LIMIT 1`

	if err := s.db.WithContext(ctx).Raw(
		query,
		orgID,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		invoiceID,
	).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.InvoiceID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	snapshot := map[string]any{
		"invoice_id":     row.InvoiceID.String(),
		"invoice_number": strings.TrimSpace(row.InvoiceNumber),
		"status":         row.Status,
		"customer_id":    row.CustomerID.String(),
		"customer_name":  row.CustomerName,
		"amount_due":     row.AmountDue,
		"currency":       strings.ToUpper(strings.TrimSpace(row.Currency)),
	}
	if row.DueAt.Valid {
		due := row.DueAt.Time.UTC()
		daysOverdue := int(now.Sub(due).Hours() / 24)
		if daysOverdue < 0 {
			daysOverdue = 0
		}
		snapshot["due_at"] = due.Format(time.RFC3339)
		snapshot["days_overdue"] = daysOverdue
	}
	return snapshot, nil
}

func (s *Service) loadCustomerSnapshot(ctx context.Context, orgID snowflake.ID, customerID snowflake.ID, now time.Time) (map[string]any, error) {
	var row collectionQueueRow
	query := `
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		), invoice_outstanding AS (
			SELECT
				i.id AS invoice_id,
				i.customer_id,
				COALESCE(i.invoice_number::text, '') AS invoice_number,
				i.due_at,
				COALESCE(i.issued_at, i.created_at) AS issued_at,
				GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS outstanding
			FROM invoices i
			LEFT JOIN settled s ON s.invoice_id_text = i.id::text
			WHERE i.org_id = ?
			  AND i.status = 'FINALIZED'
			  AND i.voided_at IS NULL
			  AND i.currency = ?
		), totals AS (
			SELECT customer_id, SUM(outstanding) AS outstanding
			FROM invoice_outstanding
			WHERE outstanding > 0
			GROUP BY customer_id
		), oldest_unpaid AS (
			SELECT DISTINCT ON (customer_id)
				customer_id,
				invoice_id,
				invoice_number,
				due_at,
				issued_at
			FROM invoice_outstanding
			WHERE outstanding > 0
			ORDER BY customer_id, COALESCE(due_at, issued_at) ASC, invoice_id ASC
		), last_payment AS (
			SELECT customer_id, MAX(received_at) AS last_payment_at
			FROM payment_events
			WHERE org_id = ? AND event_type = 'payment_succeeded'
			GROUP BY customer_id
		)
		SELECT
			c.id AS customer_id,
			c.name AS customer_name,
			COALESCE(t.outstanding, 0) AS outstanding,
			ou.invoice_id::text AS oldest_unpaid_invoice_id,
			ou.invoice_number AS oldest_unpaid_invoice_number,
			ou.due_at AS oldest_unpaid_at,
			lp.last_payment_at AS last_payment_at,
			NULL AS assigned_to,
			NULL AS assignment_expires_at
		FROM customers c
		LEFT JOIN totals t ON t.customer_id = c.id
		LEFT JOIN oldest_unpaid ou ON ou.customer_id = c.id
		LEFT JOIN last_payment lp ON lp.customer_id = c.id
		WHERE c.org_id = ? AND c.id = ?
		LIMIT 1`

	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).Raw(
		query,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		currency,
		orgID,
		orgID,
		customerID,
	).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.CustomerID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	oldestDays := 0
	if row.OldestUnpaidAt.Valid {
		due := row.OldestUnpaidAt.Time.UTC()
		oldestDays = int(now.Sub(due).Hours() / 24)
		if oldestDays < 0 {
			oldestDays = 0
		}
	}

	snapshot := map[string]any{
		"customer_id":         row.CustomerID.String(),
		"customer_name":       row.CustomerName,
		"outstanding_balance": row.Outstanding,
		"currency":            currency,
		"oldest_unpaid_days":  oldestDays,
		"aging_bucket":        computeAgingBucket(oldestDays),
		"risk_level":          computeRiskLevel(row.Outstanding, oldestDays),
	}
	if row.OldestUnpaidAt.Valid {
		due := row.OldestUnpaidAt.Time.UTC()
		snapshot["oldest_unpaid_at"] = due.Format(time.RFC3339)
	}
	if row.OldestUnpaidInvoiceID.Valid {
		snapshot["oldest_unpaid_invoice_id"] = strings.TrimSpace(row.OldestUnpaidInvoiceID.String)
	}
	if row.OldestUnpaidInvoice.Valid {
		snapshot["oldest_unpaid_invoice_number"] = strings.TrimSpace(row.OldestUnpaidInvoice.String)
	}
	if row.LastPaymentAt.Valid {
		occurred := row.LastPaymentAt.Time.UTC()
		snapshot["last_payment_at"] = occurred.Format(time.RFC3339)
	}
	return snapshot, nil
}

func decryptToken(key []byte, ciphertextB64 string) string {
	ciphertextB64 = strings.TrimSpace(ciphertextB64)
	if len(key) == 0 || ciphertextB64 == "" {
		return ""
	}

	ciphertext, err := base64.RawStdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return ""
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return ""
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return ""
	}

	return string(plaintext)
}

type billingAssignmentRow struct {
	OrgID      snowflake.ID   `gorm:"column:org_id"`
	EntityID   snowflake.ID   `gorm:"column:entity_id"`
	AssignedAt sql.NullTime   `gorm:"column:assigned_at"`
	Status     sql.NullString `gorm:"column:status"`
	BreachedAt sql.NullTime   `gorm:"column:breached_at"`
}

func (s *Service) CalculatePerformance(ctx context.Context, userID string, start, end time.Time) (domain.FinOpsScoreSnapshot, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.FinOpsScoreSnapshot{}, domain.ErrInvalidOrganization
	}

	// 1. Fetch Assignments in period
	var assignments []billingAssignmentRow
	if err := s.db.WithContext(ctx).Table("billing_operation_assignments").
		Where("org_id = ? AND assigned_to = ? AND assigned_at >= ? AND assigned_at < ?", orgID, userID, start, end).
		Find(&assignments).Error; err != nil {
		return domain.FinOpsScoreSnapshot{}, err
	}

	metrics := domain.PerformanceMetrics{
		TotalAssigned: len(assignments),
	}

	if len(assignments) == 0 {
		return domain.FinOpsScoreSnapshot{
			OrgID:          orgID.String(),
			UserID:         userID,
			PeriodType:     domain.PeriodTypeDaily,
			PeriodStart:    start,
			PeriodEnd:      end,
			ScoringVersion: domain.ScoringVersionV1EqualWeight,
			Metrics:        metrics,
			Scores:         domain.PerformanceScores{},
		}, nil
	}

	var totalResponseTime time.Duration
	var responseCount int64

	for _, a := range assignments {
		// Completion: resolved means status is 'released'
		// This indicates the assignment workflow was completed.
		if a.Status.String == domain.AssignmentStatusReleased {
			metrics.TotalResolved++
		}
		// Escalation: status is 'escalated' OR breached_at is set
		// Any breach or explicit escalation counts as an escalation.
		if a.Status.String == domain.AssignmentStatusEscalated || a.BreachedAt.Valid {
			metrics.TotalEscalated++
		}

		// Responsiveness: Check first action
		var firstAction domain.BillingActionRecord
		err := s.db.WithContext(ctx).Table("billing_operation_actions").
			Where("org_id = ? AND entity_id = ? AND created_at >= ? AND created_at < ?", orgID, a.EntityID, a.AssignedAt.Time, end).
			Order("created_at ASC").
			Limit(1).
			Scan(&firstAction).Error

		if err == nil && firstAction.ID != 0 {
			diff := firstAction.CreatedAt.Sub(a.AssignedAt.Time)
			if diff < 0 {
				diff = 0
			}
			totalResponseTime += diff
			responseCount++
		}

		// Exposure Handled: Sum of amount_due from snapshots in 'released' actions
		// Measures the risk volume handled by the user.
		if a.Status.String == domain.AssignmentStatusReleased {
			var releaseAction domain.BillingActionRecord
			err := s.db.WithContext(ctx).Table("billing_operation_actions").
				Where("org_id = ? AND entity_id = ? AND action_type = ? AND created_at >= ?", orgID, a.EntityID, domain.ActionTypeRelease, a.AssignedAt.Time).
				Order("created_at DESC").
				Limit(1).
				Scan(&releaseAction).Error

			if err == nil && releaseAction.ID != 0 {
				// Try extract from snapshot
				if snap, ok := releaseAction.Metadata["snapshot"].(map[string]any); ok {
					var amt int64
					if val, ok := snap["amount_due"].(float64); ok {
						amt = int64(val)
					} else if val, ok := snap["amount_due"].(int64); ok {
						amt = val
					} else if val, ok := snap["amount_due"].(int); ok {
						amt = int64(val)
					} else if val, ok := snap["amount_due"].(json.Number); ok {
						if v, err := val.Float64(); err == nil {
							amt = int64(v)
						}
					}

					metrics.ExposureHandled += amt
				}
			}
		}
	}

	if responseCount > 0 {
		metrics.AvgResponseMS = int64(totalResponseTime.Milliseconds()) / responseCount
	}
	metrics.CompletionRatio = float64(metrics.TotalResolved) / float64(metrics.TotalAssigned)
	metrics.EscalationRate = float64(metrics.TotalEscalated) / float64(metrics.TotalAssigned)

	// Scoring Model
	// Calculate Score (Simple Equal Weight V1)
	// scoring_version = "v1_equal_weight"
	scores := domain.PerformanceScores{}

	// Normalize metrics 0-100
	// 1. Responsiveness: < 1h = 100, > 24h = 0
	if metrics.AvgResponseMS > 0 {
		avgHours := float64(metrics.AvgResponseMS) / 3600000.0
		if avgHours <= 1 {
			scores.Responsiveness = 100
		} else if avgHours >= 24 {
			scores.Responsiveness = 0
		} else {
			scores.Responsiveness = int(100 - ((avgHours-1)/23)*100)
		}
	} else {
		// No actions? If assigned, score 0. If not assigned, skip.
		if metrics.TotalAssigned > 0 {
			scores.Responsiveness = 0
		}
	}

	// 2. Completion: Resolved / Assigned
	if metrics.TotalAssigned > 0 {
		scores.Completion = int(metrics.CompletionRatio * 100)
	}

	// 3. Risk: (1 - EscalationRate) * 100
	if metrics.TotalAssigned > 0 {
		scores.Risk = int((1.0 - metrics.EscalationRate) * 100)
	}

	// 4. Effectiveness: Log scale of exposure?
	// For V1, simple tiered score
	if metrics.ExposureHandled > 100000 { // > 100k
		scores.Effectiveness = 100
	} else if metrics.ExposureHandled > 10000 { // > 10k
		scores.Effectiveness = 75
	} else if metrics.ExposureHandled > 0 {
		scores.Effectiveness = 50
	} else {
		scores.Effectiveness = 0
	}

	// Total: Average
	scores.Total = (scores.Responsiveness + scores.Completion + scores.Risk + scores.Effectiveness) / 4

	return domain.FinOpsScoreSnapshot{
		OrgID:          orgID.String(),
		UserID:         userID,
		PeriodType:     domain.PeriodTypeDaily,
		PeriodStart:    start,
		PeriodEnd:      end,
		ScoringVersion: domain.ScoringVersionV1EqualWeight,
		Metrics:        metrics,
		Scores:         scores,
	}, nil
}

func (s *Service) GetPerformanceHistory(ctx context.Context, userID string, limit int) ([]domain.FinOpsScoreSnapshot, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return nil, domain.ErrInvalidOrganization
	}
	if limit <= 0 {
		limit = 30
	}

	type outputRow struct {
		OrgID       snowflake.ID
		UserID      string
		PeriodStart time.Time
		PeriodEnd   time.Time
		Metrics     datatypes.JSON
		Scores      datatypes.JSON
	}

	var rows []outputRow
	if err := s.db.WithContext(ctx).Table("finops_performance_snapshots").
		Where("org_id = ? AND user_id = ?", orgID, userID).
		Order("period_start DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	snapshots := make([]domain.FinOpsScoreSnapshot, len(rows))
	for i, r := range rows {
		var m domain.PerformanceMetrics
		var sc domain.PerformanceScores
		_ = json.Unmarshal(r.Metrics, &m)
		_ = json.Unmarshal(r.Scores, &sc)

		snapshots[i] = domain.FinOpsScoreSnapshot{
			OrgID:       r.OrgID.String(),
			UserID:      r.UserID,
			PeriodStart: r.PeriodStart,
			PeriodEnd:   r.PeriodEnd,
			Metrics:     m,
			Scores:      sc,
		}
	}
	return snapshots, nil
}

func (s *Service) AggregateDailyPerformance(ctx context.Context) error {
	// 1. Identify period: Yesterday 00:00 to 23:59
	now := s.clock.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	// 2. Find active users in that period
	// 2. Find active users in that period (Grouped by Org)
	type UserOrg struct {
		OrgID      snowflake.ID
		AssignedTo string
	}
	var userOrgs []UserOrg
	if err := s.db.WithContext(ctx).Table("billing_operation_assignments").
		Select("DISTINCT org_id, assigned_to").
		Where("assigned_at >= ? AND assigned_at < ?", start, end).
		Scan(&userOrgs).Error; err != nil {
		return err
	}

	for _, uo := range userOrgs {
		if uo.AssignedTo == "" {
			continue
		}

		// Create context with OrgID
		orgCtx := orgcontext.WithOrgID(ctx, uo.OrgID.Int64())

		snapshot, err := s.CalculatePerformance(orgCtx, uo.AssignedTo, start, end)
		if err != nil {
			s.log.Error("failed to calc performance", zap.Error(err), zap.String("user", uo.AssignedTo))
			continue
		}

		// Upsert into snapshots table
		// ID, OrgID, UserID, Start, End, Metrics, Scores, TotalScore, CreatedAt

		// Map struct to DB model (jsonb fields)
		// Immutable Snapshot: Delete existing for period, then Insert
		// "Recompute = delete + insert"
		err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			// 1. Delete existing snapshot for this user/org/period
			if err := tx.Exec(`
					DELETE FROM finops_performance_snapshots 
					WHERE org_id = ? AND user_id = ? AND period_type = ? AND period_start = ?
				`, uo.OrgID, uo.AssignedTo, snapshot.PeriodType, start).Error; err != nil {
				return err
			}

			// 2. Insert new snapshot
			return tx.Exec(`
					INSERT INTO finops_performance_snapshots 
					(id, org_id, user_id, period_type, period_start, period_end, scoring_version, metrics, scores, total_score, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				`, s.genID.Generate(), uo.OrgID, uo.AssignedTo, snapshot.PeriodType, start, end, snapshot.ScoringVersion,
				datatypes.JSON(toJson(snapshot.Metrics)),
				datatypes.JSON(toJson(snapshot.Scores)),
				snapshot.Scores.Total,
				now, now).Error
		})

		if err != nil {
			s.log.Error("failed to persist snapshot", zap.Error(err))
		}
	}
	return nil
}

func toJson(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// API Methods (Read-Only from Snapshots)

func (s *Service) GetMyPerformance(ctx context.Context, userID string, req domain.GetPerformanceRequest) (*domain.PerformanceResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return nil, domain.ErrInvalidOrganization
	}
	if userID == "" {
		return nil, domain.ErrInvalidAssignee // Reuse generic user error
	}

	// Defaults if not provided (though binding should handle some)
	start := req.From
	end := req.To
	now := s.clock.Now().UTC()

	if end.IsZero() {
		end = now
	}
	if start.IsZero() {
		// Default to some reasonable window if not specified, e.g., 30 days
		start = end.AddDate(0, 0, -30)
	}

	// Default limit if not provided
	limit := req.Limit
	if limit <= 0 {
		limit = 30
	}

	snapshots, err := s.snapshotRepo.FindByUserWithLimit(ctx, snowflake.ID(orgID), userID, req.PeriodType, start, end, limit)
	if err != nil {
		return nil, err
	}

	// Map keys to API response
	apiSnapshots := make([]domain.APISnapshot, len(snapshots))
	for i, s := range snapshots {
		apiSnapshots[i] = domain.APISnapshot{
			PeriodStart: s.PeriodStart,
			PeriodEnd:   s.PeriodEnd,
			TotalScore:  s.Scores.Total,
			Scores:      s.Scores,
			Metrics: domain.APIMetrics{
				// Convert MS to Minutes (1800000ms -> 30m)
				AvgResponseMinutes: float64(s.Metrics.AvgResponseMS) / 60000.0,
				CompletionRatio:    s.Metrics.CompletionRatio,
				EscalationRatio:    s.Metrics.EscalationRate,
				ExposureHandled:    s.Metrics.ExposureHandled,
			},
		}
	}

	return &domain.PerformanceResponse{
		UserID:         userID,
		PeriodType:     req.PeriodType,
		ScoringVersion: domain.ScoringVersionV1EqualWeight,
		Snapshots:      apiSnapshots,
	}, nil
}

func (s *Service) GetTeamPerformance(ctx context.Context, req domain.GetPerformanceRequest) (*domain.TeamPerformanceResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return nil, domain.ErrInvalidOrganization
	}

	// Note: Role check should be done by handler/middleware. Service assumes authorization.

	start := req.From
	end := req.To
	now := s.clock.Now().UTC()

	if end.IsZero() {
		end = now
	}
	if start.IsZero() {
		start = end.AddDate(0, 0, -30)
	}

	snapshots, err := s.snapshotRepo.FindByOrg(ctx, snowflake.ID(orgID), req.PeriodType, start, end)
	if err != nil {
		return nil, err
	}

	// Group by UserID
	grouped := make(map[string][]domain.FinOpsScoreSnapshot)
	for _, snap := range snapshots {
		grouped[snap.UserID] = append(grouped[snap.UserID], snap)
	}

	// Aggregate per user
	teamSummaries := make([]domain.TeamMemberSummary, 0, len(grouped))
	for uid, snaps := range grouped {
		var totalScore int
		var totalAssigned, totalResolved, totalEscalated int
		var totalExposure int64
		var weightedResponseMS float64
		var count int

		for _, s := range snaps {
			totalScore += s.Scores.Total
			count++
			totalAssigned += s.Metrics.TotalAssigned
			totalResolved += s.Metrics.TotalResolved
			totalEscalated += s.Metrics.TotalEscalated
			totalExposure += s.Metrics.ExposureHandled
			// Weighted average for response time based on volume
			weightedResponseMS += float64(s.Metrics.AvgResponseMS) * float64(s.Metrics.TotalResolved)
		}

		avgScore := 0
		if count > 0 {
			avgScore = totalScore / count
		}

		var avgResponseMS int64
		if totalResolved > 0 {
			avgResponseMS = int64(weightedResponseMS / float64(totalResolved))
		}

		var completionRatio, escalationRate float64
		if totalAssigned > 0 {
			completionRatio = float64(totalResolved) / float64(totalAssigned)
			escalationRate = float64(totalEscalated) / float64(totalAssigned)
		}

		teamSummaries = append(teamSummaries, domain.TeamMemberSummary{
			UserID:   uid,
			AvgScore: avgScore,
			MetricsSummary: domain.APIMetrics{
				// Aggregated Logic
				// We can re-calculate avg response minutes from weighted avg MS
				AvgResponseMinutes: float64(avgResponseMS) / 60000.0,
				CompletionRatio:    completionRatio,
				EscalationRatio:    escalationRate,
				ExposureHandled:    totalExposure,
			},
		})
	}

	// Sort by UserID for determinism
	sort.Slice(teamSummaries, func(i, j int) bool {
		return teamSummaries[i].UserID < teamSummaries[j].UserID
	})

	return &domain.TeamPerformanceResponse{
		PeriodType: req.PeriodType,
		TeamSize:   len(teamSummaries),
		Snapshots:  teamSummaries,
	}, nil
}
