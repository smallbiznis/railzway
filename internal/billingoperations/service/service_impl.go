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
	"github.com/smallbiznis/railzway/internal/billingoperations/repository" // Import repository
	"github.com/smallbiznis/railzway/internal/clock"
	"github.com/smallbiznis/railzway/internal/config"

	"github.com/smallbiznis/railzway/internal/orgcontext"
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
	repo     domain.Repository // Inject Repository
	db       *gorm.DB
	log      *zap.Logger
	clock    clock.Clock
	genID    *snowflake.Node
	auditSvc auditdomain.Service
	encKey   []byte

	billingCfg   *config.BillingConfigHolder
}

func NewService(p Params) domain.Service {
	repo := repository.NewRepository(p.DB)

	secret := strings.TrimSpace(p.Cfg.PaymentProviderConfigSecret)
	var key []byte
	if secret != "" {
		sum := sha256.Sum256([]byte(secret))
		key = sum[:]
	}

	return &Service{
		repo:         repo,
		db:           p.DB,
		log:          p.Log.Named("billingoperations.service"),
		clock:        p.Clock,
		genID:        p.GenID,
		auditSvc:     p.AuditSvc,
		encKey:       key,
		billingCfg:   p.BillingConfig,
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

	currency, err := s.repo.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.OverdueInvoicesResponse{}, err
	}

	now := s.clock.Now().UTC()
	rows, err := s.repo.ListOverdueInvoices(ctx, orgID, currency, now, limit)
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

	currency, err := s.repo.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.OutstandingCustomersResponse{}, err
	}

	now := s.clock.Now().UTC()
	rows, err := s.repo.ListOutstandingCustomers(ctx, orgID, currency, now, limit)
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
	rows, err := s.repo.ListPaymentIssues(ctx, orgID, now, limit)
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

	currency, err := s.repo.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.BillingOperationsResponse{}, err
	}

	now := s.clock.Now().UTC()
	summary, err := s.repo.LoadActionSummary(ctx, orgID, currency, now)
	if err != nil {
		return domain.BillingOperationsResponse{}, err
	}

	overdueRows, err := s.repo.ListOverdueInvoices(ctx, orgID, currency, now, limit)
	if err != nil {
		return domain.BillingOperationsResponse{}, err
	}
	failedRows, err := s.repo.ListFailedPaymentActions(ctx, orgID, currency, now, limit)
	if err != nil {
		return domain.BillingOperationsResponse{}, err
	}
	queueRows, err := s.repo.ListCollectionQueue(ctx, orgID, currency, now, limit)
	if err != nil {
		return domain.BillingOperationsResponse{}, err
	}
	paymentRows, err := s.repo.ListPaymentIssues(ctx, orgID, now, limit)
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

	beforeSnapshot, err := s.repo.LoadEntitySnapshot(ctx, orgID, entityType, entityID)
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

	inserted, err := s.repo.InsertBillingAction(ctx, domain.BillingActionRecord{
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
			existing, err := s.repo.FindActionByIdempotencyKey(ctx, orgID, idempotencyKey)
			if err == nil && existing != nil {
				resolvedActionID = existing.ID.String()
			}
		} else {
			existing, err := s.repo.FindActionByBucket(ctx, orgID, entityType, entityID, actionType, bucket)
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
		if err := s.repo.UpdateAssignmentStatus(
			ctx, orgID, entityType, entityID,
			domain.AssignmentStatusAssigned, domain.AssignmentStatusInProgress,
			now,
		); err != nil {
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
		repoTx := s.repo.WithTx(tx)

		existing, err := repoTx.LoadAssignmentForUpdate(
			ctx, orgID, entityType, entityID,
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

			if err := repoTx.UpsertAssignment(ctx, record); err != nil {
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
		snapshot, err := s.repo.LoadEntitySnapshot(ctx, orgID, req.EntityType, entityID)
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

		if err := repoTx.UpsertAssignment(ctx, record); err != nil {
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

		if _, err := repoTx.InsertBillingAction(ctx, domain.BillingActionRecord{
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
		repoTx := s.repo.WithTx(tx)

		existing, err := repoTx.LoadAssignmentForUpdate(ctx, orgID, entityType, entityID)
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

		if err := repoTx.UpsertAssignment(ctx, *existing); err != nil {
			return err
		}

		// Record release action
		actionID := s.genID.Generate()
		bucket := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		snapshot, err := s.repo.LoadEntitySnapshot(ctx, orgID, entityType, entityID)
		if err != nil {
			s.log.Warn("failed to load entity snapshot", zap.Error(err))
			snapshot = make(map[string]interface{})
		}

		if _, err := repoTx.InsertBillingAction(ctx, domain.BillingActionRecord{
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
		repoTx := s.repo.WithTx(tx)

		existing, err := repoTx.LoadAssignmentForUpdate(ctx, orgID, entityType, entityID)
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

		if err := repoTx.UpsertAssignment(ctx, *existing); err != nil {
			return err
		}

		// Record resolve action
		actionID := s.genID.Generate()
		bucket := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		if _, err := repoTx.InsertBillingAction(ctx, domain.BillingActionRecord{
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


func timePtr(t sql.NullTime) *time.Time {
	if t.Valid {
		val := t.Time.UTC()
		return &val
	}
	return nil
}

func parseSnowflakeID(id string) (snowflake.ID, error) {
	return snowflake.ParseString(id)
}

func normalizeIdempotencyKey(key string) string {
	return strings.TrimSpace(key)
}

func buildAuditAction(actionType string) string {
	return "billing_operations.action." + strings.ToLower(actionType)
}

func computeAgingBucket(days int) string {
	switch {
	case days <= 30:
		return "0-30"
	case days <= 60:
		return "31-60"
	case days <= 90:
		return "61-90"
	default:
		return "90+"
	}
}

func computeRiskLevel(amount int64, days int) string {
	// Simple heuristic
	score := int(amount/10000) + days
	if score > 100 {
		return "high"
	}
	if score > 50 {
		return "medium"
	}
	return "low"
}

func (s *Service) EvaluateSLAs(ctx context.Context) error {
	const (
		initialResponseSLA = 30 * time.Minute
		idleActionSLA      = 60 * time.Minute
	)
	now := s.clock.Now().UTC()

	var records []domain.BillingAssignmentRecord
	// Find active assignments that are NOT already escalated
	records, err := s.repo.ListActiveAssignments(ctx)
	if err != nil {
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
				repoTx := s.repo.WithTx(tx)

				// 1. Update Assignment
				if err := repoTx.EscalateAssignment(ctx, rec.OrgID, rec.EntityType, rec.EntityID, breachType, now); err != nil {
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

				_, err := repoTx.InsertBillingAction(ctx, domain.BillingActionRecord{
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
				})
				return err
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


func (s *Service) CalculatePerformance(ctx context.Context, userID string, start, end time.Time) (domain.FinOpsScoreSnapshot, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.FinOpsScoreSnapshot{}, domain.ErrInvalidOrganization
	}

	// 1. Fetch Assignments in period
	assignments, err := s.repo.ListBillingAssignmentsForPerformance(ctx, orgID, userID, start, end)
	if err != nil {
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

	snapshots, err := s.repo.FindSnapshotsByUserWithLimit(ctx, snowflake.ID(orgID), userID, req.PeriodType, start, end, limit)
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

	snapshots, err := s.repo.FindSnapshotsByOrg(ctx, snowflake.ID(orgID), req.PeriodType, start, end)
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
