package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	ledgerdomain "github.com/smallbiznis/valora/internal/ledger/domain"
	obsmetrics "github.com/smallbiznis/valora/internal/observability/metrics"
	paymentdomain "github.com/smallbiznis/valora/internal/payment/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB         *gorm.DB
	Log        *zap.Logger
	GenID      *snowflake.Node
	LedgerSvc  ledgerdomain.Service
	AuditSvc   auditdomain.Service
	Repo       paymentdomain.Repository
	ObsMetrics *obsmetrics.Metrics `optional:"true"`
}

type Service struct {
	db         *gorm.DB
	log        *zap.Logger
	genID      *snowflake.Node
	ledgerSvc  ledgerdomain.Service
	auditSvc   auditdomain.Service
	repo       paymentdomain.Repository
	obsMetrics *obsmetrics.Metrics
}

func NewService(p Params) *Service {
	return &Service{
		db:         p.DB,
		log:        p.Log.Named("payment.service"),
		genID:      p.GenID,
		ledgerSvc:  p.LedgerSvc,
		auditSvc:   p.AuditSvc,
		repo:       p.Repo,
		obsMetrics: p.ObsMetrics,
	}
}

func (s *Service) ProcessEvent(ctx context.Context, event *paymentdomain.PaymentEvent, payload []byte) error {
	if event == nil {
		return paymentdomain.ErrInvalidEvent
	}
	event.Provider = strings.ToLower(strings.TrimSpace(event.Provider))
	if event.Provider == "" {
		return paymentdomain.ErrInvalidProvider
	}
	if !json.Valid(payload) {
		return paymentdomain.ErrInvalidPayload
	}
	if err := validateEvent(event); err != nil {
		return err
	}

	now := time.Now().UTC()
	received := paymentdomain.EventRecord{
		ID:              s.genID.Generate(),
		OrgID:           event.OrgID,
		Provider:        event.Provider,
		ProviderEventID: event.ProviderEventID,
		EventType:       event.Type,
		CustomerID:      event.CustomerID,
		Payload:         datatypes.JSON(payload),
		ReceivedAt:      now,
	}

	inserted, err := s.insertEvent(ctx, &received)
	if err != nil {
		return err
	}
	stored := &received
	if !inserted {
		stored, err = s.loadEvent(ctx, event.Provider, event.ProviderEventID)
		if err != nil {
			return err
		}
		if stored == nil {
			return paymentdomain.ErrInvalidEvent
		}
		if stored.ProcessedAt != nil {
			return paymentdomain.ErrEventAlreadyProcessed
		}
	}

	if err := s.processEvent(ctx, stored, event); err != nil {
		return err
	}

	if err := s.markProcessed(ctx, stored.ID, now); err != nil {
		return err
	}

	if inserted && s.obsMetrics != nil {
		s.obsMetrics.RecordPaymentEvent(ctx, event.Provider, event.Type)
	}

	return nil
}

func validateEvent(event *paymentdomain.PaymentEvent) error {
	if event == nil {
		return paymentdomain.ErrInvalidEvent
	}
	event.ProviderEventID = strings.TrimSpace(event.ProviderEventID)
	if event.ProviderEventID == "" {
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
	switch event.Type {
	case paymentdomain.EventTypePaymentSucceeded, paymentdomain.EventTypeRefunded:
		if event.Amount <= 0 {
			return paymentdomain.ErrInvalidAmount
		}
	case paymentdomain.EventTypePaymentFailed:
	default:
		return paymentdomain.ErrInvalidEvent
	}
	return nil
}

func (s *Service) insertEvent(ctx context.Context, event *paymentdomain.EventRecord) (bool, error) {
	return s.repo.InsertEvent(ctx, s.db, event)
}

func (s *Service) loadEvent(ctx context.Context, provider string, providerEventID string) (*paymentdomain.EventRecord, error) {
	return s.repo.FindEvent(ctx, s.db, provider, providerEventID)
}

func (s *Service) markProcessed(ctx context.Context, id snowflake.ID, processedAt time.Time) error {
	return s.repo.MarkProcessed(ctx, s.db, id, processedAt)
}

func (s *Service) processEvent(ctx context.Context, stored *paymentdomain.EventRecord, event *paymentdomain.PaymentEvent) error {
	if stored == nil || event == nil {
		return paymentdomain.ErrInvalidEvent
	}

	switch event.Type {
	case paymentdomain.EventTypePaymentSucceeded:
		if err := s.settlePayment(ctx, stored, event); err != nil {
			return err
		}
	case paymentdomain.EventTypeRefunded:
		if err := s.settleRefund(ctx, stored, event); err != nil {
			return err
		}
	case paymentdomain.EventTypePaymentFailed:
		return s.writeAuditLog(ctx, "payment.failed", stored, event, nil)
	default:
		return paymentdomain.ErrInvalidEvent
	}

	return nil
}

func (s *Service) settlePayment(
	ctx context.Context,
	stored *paymentdomain.EventRecord,
	event *paymentdomain.PaymentEvent,
) error {

	if err := s.createPaymentLedgerEntry(
		ctx,
		stored,
		event,
		string(ledgerdomain.SourceTypePayment),
		ledgerdomain.AccountCodeCash,
		ledgerdomain.AccountCodeAccountsReceivable,
		ledgerdomain.LedgerEntryDirectionDebit,
		ledgerdomain.LedgerEntryDirectionCredit,
	); err != nil {
		return err
	}

	if err := s.updateInvoiceSettlement(ctx, stored.OrgID, event, false); err != nil {
		return err
	}

	balance, err := s.customerBalance(ctx, stored.OrgID, event.CustomerID, event.Currency)
	if err != nil {
		return err
	}

	return s.writeAuditLog(
		ctx,
		"payment.received",
		stored,
		event,
		map[string]any{"balance": balance},
	)
}

func (s *Service) settleRefund(
	ctx context.Context,
	stored *paymentdomain.EventRecord,
	event *paymentdomain.PaymentEvent,
) error {

	if err := s.createPaymentLedgerEntry(
		ctx,
		stored,
		event,
		string(ledgerdomain.SourceTypeRefund),
		ledgerdomain.AccountCodeRefundLiab,
		ledgerdomain.AccountCodeCash,
		ledgerdomain.LedgerEntryDirectionDebit,
		ledgerdomain.LedgerEntryDirectionCredit,
	); err != nil {
		return err
	}

	balance, err := s.customerBalance(ctx, stored.OrgID, event.CustomerID, event.Currency)
	if err != nil {
		return err
	}

	return s.writeAuditLog(
		ctx,
		"payment.refunded",
		stored,
		event,
		map[string]any{"balance": balance},
	)
}

func (s *Service) createPaymentLedgerEntry(
	ctx context.Context,
	stored *paymentdomain.EventRecord,
	event *paymentdomain.PaymentEvent,
	sourceType string,
	debitAccount ledgerdomain.LedgerAccountCode,
	creditAccount ledgerdomain.LedgerAccountCode,
	debitDir ledgerdomain.LedgerEntryDirection,
	creditDir ledgerdomain.LedgerEntryDirection,
) error {

	now := time.Now().UTC()

	debitID, err := s.ensureLedgerAccount(
		ctx,
		stored.OrgID,
		string(debitAccount),
		string(debitAccount),
		now,
	)
	if err != nil {
		return err
	}

	creditID, err := s.ensureLedgerAccount(
		ctx,
		stored.OrgID,
		string(creditAccount),
		string(creditAccount),
		now,
	)
	if err != nil {
		return err
	}

	lines := []ledgerdomain.LedgerEntryLine{
		{AccountID: debitID, Direction: debitDir, Amount: event.Amount},
		{AccountID: creditID, Direction: creditDir, Amount: event.Amount},
	}

	return s.ledgerSvc.CreateEntry(
		ctx,
		stored.OrgID,
		sourceType,
		stored.ID,
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

func (s *Service) updateInvoiceSettlement(ctx context.Context, orgID snowflake.ID, event *paymentdomain.PaymentEvent, isRefund bool) error {
	if event == nil || event.InvoiceID == nil || *event.InvoiceID == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct {
			ID             snowflake.ID      `gorm:"column:id"`
			OrgID          snowflake.ID      `gorm:"column:org_id"`
			SubtotalAmount int64             `gorm:"column:subtotal_amount"`
			Metadata       datatypes.JSONMap `gorm:"column:metadata"`
		}
		if err := tx.WithContext(ctx).Raw(
			`SELECT id, org_id, subtotal_amount, metadata
			 FROM invoices
			 WHERE id = ? AND org_id = ?
			 FOR UPDATE`,
			*event.InvoiceID,
			orgID,
		).Scan(&row).Error; err != nil {
			return err
		}
		if row.ID == 0 {
			return nil
		}

		paid := readMetadataAmount(row.Metadata, "amount_paid")
		if isRefund {
			paid -= event.Amount
		} else {
			paid += event.Amount
		}
		if paid < 0 {
			paid = 0
		}
		if row.Metadata == nil {
			row.Metadata = datatypes.JSONMap{}
		}
		row.Metadata["amount_paid"] = paid

		now := time.Now().UTC()
		if row.SubtotalAmount > 0 && paid >= row.SubtotalAmount {
			row.Metadata["paid_at"] = now.Format(time.RFC3339)
		} else {
			delete(row.Metadata, "paid_at")
		}

		if err := tx.WithContext(ctx).Exec(
			`UPDATE invoices
			 SET metadata = ?, updated_at = ?
			 WHERE id = ? AND org_id = ?`,
			row.Metadata,
			now,
			row.ID,
			orgID,
		).Error; err != nil {
			return err
		}

		return nil
	})
}

func readMetadataAmount(metadata datatypes.JSONMap, key string) int64 {
	if metadata == nil {
		return 0
	}
	value, ok := metadata[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed
		}
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func (s *Service) customerBalance(
	ctx context.Context,
	orgID snowflake.ID,
	customerID snowflake.ID,
	currency string,
) (int64, error) {

	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return 0, paymentdomain.ErrInvalidCurrency
	}

	var balance int64

	err := s.db.WithContext(ctx).Raw(
		`
		SELECT COALESCE(
			SUM(CASE l.direction WHEN 'debit' THEN l.amount ELSE -l.amount END),
			0
		) AS balance
		FROM ledger_entries le
		JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
		JOIN ledger_accounts a ON a.id = l.account_id

		-- billing & adjustment → subscription scoped
		LEFT JOIN billing_cycles bc
			ON bc.id = le.source_id
		   AND le.source_type IN (?, ?)
		LEFT JOIN subscriptions s
			ON s.id = bc.subscription_id

		-- payment / refund / fee → payment scoped
		LEFT JOIN payment_events pe
			ON pe.id = le.source_id
		   AND le.source_type IN (?, ?, ?)

		-- disputes → dispute scoped
		LEFT JOIN payment_disputes pd
			ON pd.id = le.source_id
		   AND le.source_type IN (?, ?, ?)

		WHERE le.org_id = ?
		  AND a.code = ?
		  AND le.currency = ?
		  AND (
			   (le.source_type IN (?, ?) AND s.customer_id = ?)
			OR (le.source_type IN (?, ?, ?) AND pe.customer_id = ?)
			OR (le.source_type IN (?, ?, ?) AND pd.customer_id = ?)
		  )
		`,
		// joins
		ledgerdomain.SourceTypeBillingCycle,
		ledgerdomain.SourceTypeAdjustment,

		ledgerdomain.SourceTypePayment,
		ledgerdomain.SourceTypePaymentFee,
		ledgerdomain.SourceTypeRefund,

		ledgerdomain.SourceTypeDisputeHold,
		ledgerdomain.SourceTypeDisputeWin,
		ledgerdomain.SourceTypeDisputeLoss,

		// filters
		orgID,
		ledgerdomain.AccountCodeAccountsReceivable,
		currency,

		// customer resolution
		ledgerdomain.SourceTypeBillingCycle,
		ledgerdomain.SourceTypeAdjustment,
		customerID,

		ledgerdomain.SourceTypePayment,
		ledgerdomain.SourceTypePaymentFee,
		ledgerdomain.SourceTypeRefund,
		customerID,

		ledgerdomain.SourceTypeDisputeHold,
		ledgerdomain.SourceTypeDisputeWin,
		ledgerdomain.SourceTypeDisputeLoss,
		customerID,
	).Scan(&balance).Error

	if err != nil {
		return 0, err
	}

	return balance, nil
}

func (s *Service) writeAuditLog(ctx context.Context, action string, stored *paymentdomain.EventRecord, event *paymentdomain.PaymentEvent, extra map[string]any) error {
	if s.auditSvc == nil {
		s.log.Warn("audit service unavailable for payment event", zap.String("action", action))
		return nil
	}
	if stored == nil || event == nil {
		return paymentdomain.ErrInvalidEvent
	}
	metadata := map[string]any{
		"provider":          stored.Provider,
		"provider_event_id": stored.ProviderEventID,
		"customer_id":       stored.CustomerID.String(),
		"amount":            event.Amount,
		"currency":          strings.ToUpper(strings.TrimSpace(event.Currency)),
		"event_type":        stored.EventType,
		"payment_event_id":  stored.ID.String(),
		"occurred_at":       event.OccurredAt.UTC().Format(time.RFC3339),
		"received_at":       stored.ReceivedAt.UTC().Format(time.RFC3339),
	}
	if event.InvoiceID != nil && *event.InvoiceID != 0 {
		metadata["invoice_id"] = event.InvoiceID.String()
	}
	if name := s.loadCustomerName(ctx, stored.OrgID, stored.CustomerID); name != "" {
		metadata["customer_name"] = name
	}
	for key, value := range extra {
		if key == "" {
			continue
		}
		metadata[key] = value
	}

	targetID := stored.ID.String()
	orgID := stored.OrgID
	if err := s.auditSvc.AuditLog(ctx, &orgID, "", nil, action, "payment_event", &targetID, metadata); err != nil {
		s.log.Warn("failed to write payment audit log", zap.String("action", action), zap.Error(err))
		return nil
	}
	return nil
}

func (s *Service) loadCustomerName(ctx context.Context, orgID snowflake.ID, customerID snowflake.ID) string {
	var name string
	if err := s.db.WithContext(ctx).Raw(
		`SELECT name
		 FROM customers
		 WHERE org_id = ? AND id = ?`,
		orgID,
		customerID,
	).Scan(&name).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(name)
}
