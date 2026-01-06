package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	ledgerdomain "github.com/smallbiznis/valora/internal/ledger/domain"
	disputedomain "github.com/smallbiznis/valora/internal/payment/dispute/domain"
	paymentdomain "github.com/smallbiznis/valora/internal/payment/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB        *gorm.DB
	Log       *zap.Logger
	GenID     *snowflake.Node
	LedgerSvc ledgerdomain.Service
	AuditSvc  auditdomain.Service
	Repo      disputedomain.Repository
}

type Service struct {
	db        *gorm.DB
	log       *zap.Logger
	genID     *snowflake.Node
	ledgerSvc ledgerdomain.Service
	auditSvc  auditdomain.Service
	repo      disputedomain.Repository
}

func NewService(p Params) *Service {
	return &Service{
		db:        p.DB,
		log:       p.Log.Named("payment.dispute"),
		genID:     p.GenID,
		ledgerSvc: p.LedgerSvc,
		auditSvc:  p.AuditSvc,
		repo:      p.Repo,
	}
}

func (s *Service) ProcessEvent(ctx context.Context, event *disputedomain.DisputeEvent) error {
	if err := validateDisputeEvent(event); err != nil {
		return err
	}
	if s.repo == nil {
		return errors.New("dispute_repository_unavailable")
	}

	event.Provider = strings.ToLower(strings.TrimSpace(event.Provider))
	if event.Provider == "" {
		return paymentdomain.ErrInvalidProvider
	}
	event.Reason = strings.TrimSpace(event.Reason)

	now := time.Now().UTC()
	var stored *disputedomain.DisputeRecord

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := s.repo.FindDisputeForUpdate(ctx, tx, event.Provider, event.ProviderDisputeID)
		if err != nil {
			return err
		}
		if existing != nil && existing.ProviderEventID == event.ProviderEventID && existing.ProcessedAt != nil {
			return paymentdomain.ErrEventAlreadyProcessed
		}

		status := statusForEvent(event.Type)
		if existing == nil {
			record := disputedomain.DisputeRecord{
				ID:                s.genID.Generate(),
				OrgID:             event.OrgID,
				Provider:          event.Provider,
				ProviderDisputeID: event.ProviderDisputeID,
				ProviderEventID:   event.ProviderEventID,
				CustomerID:        event.CustomerID,
				Amount:            event.Amount,
				Currency:          event.Currency,
				Status:            status,
				Reason:            event.Reason,
				ReceivedAt:        now,
			}
			inserted, err := s.repo.InsertDispute(ctx, tx, &record)
			if err != nil {
				return err
			}
			if !inserted {
				loaded, err := s.repo.FindDisputeForUpdate(ctx, tx, event.Provider, event.ProviderDisputeID)
				if err != nil {
					return err
				}
				if loaded == nil {
					return errors.New("dispute_not_found")
				}
				if loaded.ProviderEventID == event.ProviderEventID && loaded.ProcessedAt != nil {
					return paymentdomain.ErrEventAlreadyProcessed
				}
				existing = loaded
			} else {
				stored = &record
				return nil
			}
		}

		existing.ProviderEventID = event.ProviderEventID
		existing.CustomerID = event.CustomerID
		existing.Amount = event.Amount
		existing.Currency = event.Currency
		if event.Reason != "" {
			existing.Reason = event.Reason
		}
		existing.Status = nextStatus(existing.Status, status)
		existing.ReceivedAt = now
		existing.ProcessedAt = nil

		if err := s.repo.UpdateDispute(ctx, tx, existing); err != nil {
			return err
		}
		stored = existing
		return nil
	})
	if err != nil {
		return err
	}
	if stored == nil {
		return paymentdomain.ErrInvalidEvent
	}

	switch event.Type {
	case disputedomain.EventTypeDisputeFundsWithdrawn:
		if err := s.createLedgerEntry(
			ctx,
			stored,
			event,
			string(ledgerdomain.SourceTypeDisputeHold),
			ledgerdomain.LedgerEntryDirectionCredit,
			ledgerdomain.LedgerEntryDirectionDebit,
		); err != nil {
			return err
		}

	case disputedomain.EventTypeDisputeFundsReinstated:
		if err := s.createLedgerEntry(
			ctx,
			stored,
			event,
			string(ledgerdomain.SourceTypeDisputeWin),
			ledgerdomain.LedgerEntryDirectionDebit,
			ledgerdomain.LedgerEntryDirectionCredit,
		); err != nil {
			return err
		}
	}

	action := auditAction(event.Type)
	if action == "" {
		return paymentdomain.ErrInvalidEvent
	}
	if err := s.writeAuditLog(ctx, action, stored, event); err != nil {
		return err
	}

	if err := s.repo.MarkProcessed(ctx, s.db, stored.ID, now); err != nil {
		return err
	}

	return nil
}

func validateDisputeEvent(event *disputedomain.DisputeEvent) error {
	if event == nil {
		return paymentdomain.ErrInvalidEvent
	}

	event.ProviderEventID = strings.TrimSpace(event.ProviderEventID)
	if event.ProviderEventID == "" {
		return paymentdomain.ErrInvalidEvent
	}
	event.ProviderDisputeID = strings.TrimSpace(event.ProviderDisputeID)
	if event.ProviderDisputeID == "" {
		return paymentdomain.ErrInvalidEvent
	}
	event.Type = strings.TrimSpace(event.Type)
	if event.Type == "" {
		return paymentdomain.ErrInvalidEvent
	}
	if event.OrgID == 0 {
		return paymentdomain.ErrInvalidEvent
	}
	if event.CustomerID == 0 {
		return paymentdomain.ErrInvalidCustomer
	}

	currency := strings.TrimSpace(event.Currency)
	if currency == "" {
		return paymentdomain.ErrInvalidCurrency
	}
	event.Currency = strings.ToUpper(currency)
	if event.OccurredAt.IsZero() {
		return paymentdomain.ErrInvalidEvent
	}
	if event.Amount <= 0 {
		return paymentdomain.ErrInvalidAmount
	}

	switch event.Type {
	case disputedomain.EventTypeDisputeCreated,
		disputedomain.EventTypeDisputeFundsWithdrawn,
		disputedomain.EventTypeDisputeFundsReinstated,
		disputedomain.EventTypeDisputeClosed:
		return nil
	default:
		return paymentdomain.ErrInvalidEvent
	}
}

func statusForEvent(eventType string) string {
	switch eventType {
	case disputedomain.EventTypeDisputeCreated:
		return disputedomain.DisputeStatusOpen
	case disputedomain.EventTypeDisputeFundsWithdrawn:
		return disputedomain.DisputeStatusWithdrawn
	case disputedomain.EventTypeDisputeFundsReinstated:
		return disputedomain.DisputeStatusReinstated
	case disputedomain.EventTypeDisputeClosed:
		return disputedomain.DisputeStatusClosed
	default:
		return ""
	}
}

func nextStatus(current string, desired string) string {
	if desired == "" {
		return current
	}
	if current == disputedomain.DisputeStatusClosed {
		return current
	}
	if desired == disputedomain.DisputeStatusClosed {
		return desired
	}

	rank := map[string]int{
		disputedomain.DisputeStatusOpen:       1,
		disputedomain.DisputeStatusWithdrawn:  2,
		disputedomain.DisputeStatusReinstated: 3,
		disputedomain.DisputeStatusClosed:     4,
	}

	currentRank := rank[strings.TrimSpace(current)]
	desiredRank := rank[strings.TrimSpace(desired)]
	if desiredRank == 0 {
		return current
	}
	if currentRank == 0 || desiredRank > currentRank {
		return desired
	}
	return current
}

func (s *Service) createLedgerEntry(
	ctx context.Context,
	dispute *disputedomain.DisputeRecord,
	event *disputedomain.DisputeEvent,
	sourceType string,
	cashDirection ledgerdomain.LedgerEntryDirection,
	arDirection ledgerdomain.LedgerEntryDirection,
) error {
	if s.ledgerSvc == nil {
		return errors.New("ledger_service_unavailable")
	}
	if dispute == nil || event == nil {
		return paymentdomain.ErrInvalidEvent
	}

	now := time.Now().UTC()

	cashID, err := s.ensureLedgerAccount(
		ctx,
		dispute.OrgID,
		string(ledgerdomain.AccountCodeRefundLiab),
		"Refund / Dispute Liability",
		now,
	)
	if err != nil {
		return err
	}

	arID, err := s.ensureLedgerAccount(
		ctx,
		dispute.OrgID,
		string(ledgerdomain.AccountCodeAccountsReceivable),
		"Accounts Receivable",
		now,
	)
	if err != nil {
		return err
	}

	lines := []ledgerdomain.LedgerEntryLine{
		{AccountID: cashID, Direction: cashDirection, Amount: event.Amount},
		{AccountID: arID, Direction: arDirection, Amount: event.Amount},
	}

	return s.ledgerSvc.CreateEntry(
		ctx,
		dispute.OrgID,
		sourceType,
		dispute.ID,
		event.Currency,
		event.OccurredAt,
		lines,
	)
}

func (s *Service) ensureLedgerAccount(ctx context.Context, orgID snowflake.ID, code string, name string, now time.Time) (snowflake.ID, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return 0, ledgerdomain.ErrInvalidAccount
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, ledgerdomain.ErrInvalidAccount
	}

	var accountID snowflake.ID
	if err := s.db.WithContext(ctx).Raw(
		`SELECT id
		 FROM ledger_accounts
		 WHERE org_id = ? AND code = ?`,
		orgID,
		code,
	).Scan(&accountID).Error; err != nil {
		return 0, err
	}
	if accountID != 0 {
		return accountID, nil
	}

	newID := s.genID.Generate()
	if err := s.db.WithContext(ctx).Exec(
		`INSERT INTO ledger_accounts (id, org_id, code, name, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (org_id, code) DO NOTHING`,
		newID,
		orgID,
		code,
		name,
		now,
	).Error; err != nil {
		return 0, err
	}

	if err := s.db.WithContext(ctx).Raw(
		`SELECT id
		 FROM ledger_accounts
		 WHERE org_id = ? AND code = ?`,
		orgID,
		code,
	).Scan(&accountID).Error; err != nil {
		return 0, err
	}
	if accountID == 0 {
		return 0, errors.New("ledger_account_not_found")
	}
	return accountID, nil
}

func (s *Service) writeAuditLog(ctx context.Context, action string, stored *disputedomain.DisputeRecord, event *disputedomain.DisputeEvent) error {
	if s.auditSvc == nil {
		s.log.Warn("audit service unavailable for dispute event", zap.String("action", action))
		return nil
	}
	if stored == nil || event == nil {
		return paymentdomain.ErrInvalidEvent
	}
	metadata := map[string]any{
		"provider":            stored.Provider,
		"provider_event_id":   stored.ProviderEventID,
		"provider_dispute_id": stored.ProviderDisputeID,
		"dispute_id":          stored.ID.String(),
		"customer_id":         stored.CustomerID.String(),
		"amount":              event.Amount,
		"currency":            strings.ToUpper(strings.TrimSpace(event.Currency)),
		"status":              stored.Status,
		"occurred_at":         event.OccurredAt.UTC().Format(time.RFC3339),
		"received_at":         stored.ReceivedAt.UTC().Format(time.RFC3339),
	}
	if stored.Reason != "" {
		metadata["reason"] = stored.Reason
	}

	targetID := stored.ID.String()
	orgID := stored.OrgID
	if err := s.auditSvc.AuditLog(ctx, &orgID, "", nil, action, "payment_dispute", &targetID, metadata); err != nil {
		s.log.Warn("failed to write dispute audit log", zap.String("action", action), zap.Error(err))
		return nil
	}
	return nil
}

func auditAction(eventType string) string {
	switch eventType {
	case disputedomain.EventTypeDisputeCreated:
		return "dispute.opened"
	case disputedomain.EventTypeDisputeFundsWithdrawn:
		return "dispute.withdrawn"
	case disputedomain.EventTypeDisputeFundsReinstated:
		return "dispute.reinstated"
	case disputedomain.EventTypeDisputeClosed:
		return "dispute.closed"
	default:
		return ""
	}
}
