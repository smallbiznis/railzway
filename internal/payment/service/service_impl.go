package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	"github.com/smallbiznis/valora/internal/config"
	ledgerdomain "github.com/smallbiznis/valora/internal/ledger/domain"
	"github.com/smallbiznis/valora/internal/payment/adapters"
	paymentdomain "github.com/smallbiznis/valora/internal/payment/domain"
	paymentproviderdomain "github.com/smallbiznis/valora/internal/paymentprovider/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB        *gorm.DB
	Log       *zap.Logger
	GenID     *snowflake.Node
	LedgerSvc ledgerdomain.Service
	AuditSvc  auditdomain.Service
	Repo      paymentdomain.Repository
	Cfg       config.Config
	Adapters  *adapters.Registry
}

type Service struct {
	db        *gorm.DB
	log       *zap.Logger
	genID     *snowflake.Node
	ledgerSvc ledgerdomain.Service
	auditSvc  auditdomain.Service
	repo      paymentdomain.Repository
	adapters  *adapters.Registry
	encKey    []byte
}

type encryptedPayload struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type providerConfigRow struct {
	OrgID  snowflake.ID
	Config datatypes.JSON
}

func NewService(p Params) paymentdomain.Service {
	secret := strings.TrimSpace(p.Cfg.PaymentProviderConfigSecret)
	var key []byte
	if secret != "" {
		sum := sha256.Sum256([]byte(secret))
		key = sum[:]
	}

	return &Service{
		db:        p.DB,
		log:       p.Log.Named("payment.service"),
		genID:     p.GenID,
		ledgerSvc: p.LedgerSvc,
		auditSvc:  p.AuditSvc,
		repo:      p.Repo,
		adapters:  p.Adapters,
		encKey:    key,
	}
}

func (s *Service) IngestWebhook(ctx context.Context, provider string, payload []byte, headers http.Header) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return paymentdomain.ErrInvalidProvider
	}
	if s.adapters == nil || !s.adapters.ProviderExists(provider) {
		return paymentdomain.ErrProviderNotFound
	}
	if !json.Valid(payload) {
		return paymentdomain.ErrInvalidPayload
	}

	configs, err := s.listActiveConfigs(ctx, provider)
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		return paymentdomain.ErrProviderNotFound
	}

	_, event, err := s.matchAdapter(ctx, provider, payload, headers, configs)
	if err != nil {
		if errors.Is(err, paymentdomain.ErrEventIgnored) {
			return nil
		}
		if errors.Is(err, paymentdomain.ErrInvalidCustomer) {
			s.log.Warn("payment webhook missing customer mapping", zap.String("provider", provider))
		}
		return err
	}
	if event == nil {
		return paymentdomain.ErrInvalidSignature
	}

	now := time.Now().UTC()
	received := paymentdomain.EventRecord{
		ID:              s.genID.Generate(),
		OrgID:           event.OrgID,
		Provider:        provider,
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
		stored, err = s.loadEvent(ctx, provider, event.ProviderEventID)
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

	return nil
}

func (s *Service) listActiveConfigs(ctx context.Context, provider string) ([]providerConfigRow, error) {
	var rows []providerConfigRow
	err := s.db.WithContext(ctx).Raw(
		`SELECT org_id, config
		 FROM payment_provider_configs
		 WHERE provider = ? AND is_active = TRUE`,
		provider,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) matchAdapter(
	ctx context.Context,
	provider string,
	payload []byte,
	headers http.Header,
	configs []providerConfigRow,
) (paymentdomain.PaymentAdapter, *paymentdomain.PaymentEvent, error) {
	var configErr error
	for _, cfg := range configs {
		decrypted, err := s.decryptConfig(cfg.Config)
		if err != nil {
			if errors.Is(err, paymentproviderdomain.ErrEncryptionKeyMissing) {
				return nil, nil, err
			}
			configErr = err
			continue
		}

		adapter, err := s.adapters.NewAdapter(provider, paymentdomain.AdapterConfig{
			OrgID:    cfg.OrgID,
			Provider: provider,
			Config:   decrypted,
		})
		if err != nil {
			configErr = err
			continue
		}

		if err := adapter.Verify(ctx, payload, headers); err != nil {
			if errors.Is(err, paymentdomain.ErrInvalidSignature) {
				continue
			}
			return nil, nil, err
		}

		event, err := adapter.Parse(ctx, payload)
		if err != nil {
			if errors.Is(err, paymentdomain.ErrEventIgnored) {
				return adapter, nil, err
			}
			return nil, nil, err
		}
		event.Provider = provider
		event.OrgID = cfg.OrgID
		if err := validateEvent(event); err != nil {
			return nil, nil, err
		}
		return adapter, event, nil
	}

	if configErr != nil {
		return nil, nil, configErr
	}
	return nil, nil, paymentdomain.ErrInvalidSignature
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

func (s *Service) settlePayment(ctx context.Context, stored *paymentdomain.EventRecord, event *paymentdomain.PaymentEvent) error {
	if err := s.createLedgerEntry(ctx, stored, event, ledgerdomain.LedgerEntryDirectionDebit, ledgerdomain.LedgerEntryDirectionCredit); err != nil {
		return err
	}

	if err := s.updateInvoiceSettlement(ctx, stored.OrgID, event, false); err != nil {
		return err
	}

	balance, err := s.customerBalance(ctx, stored.OrgID, event.CustomerID, event.Currency)
	if err != nil {
		return err
	}

	metadata := map[string]any{"balance": balance}
	return s.writeAuditLog(ctx, "payment.received", stored, event, metadata)
}

func (s *Service) settleRefund(ctx context.Context, stored *paymentdomain.EventRecord, event *paymentdomain.PaymentEvent) error {
	if err := s.createLedgerEntry(ctx, stored, event, ledgerdomain.LedgerEntryDirectionCredit, ledgerdomain.LedgerEntryDirectionDebit); err != nil {
		return err
	}

	if err := s.updateInvoiceSettlement(ctx, stored.OrgID, event, true); err != nil {
		return err
	}

	balance, err := s.customerBalance(ctx, stored.OrgID, event.CustomerID, event.Currency)
	if err != nil {
		return err
	}

	metadata := map[string]any{"balance": balance}
	return s.writeAuditLog(ctx, "payment.refunded", stored, event, metadata)
}

func (s *Service) createLedgerEntry(
	ctx context.Context,
	stored *paymentdomain.EventRecord,
	event *paymentdomain.PaymentEvent,
	cashDirection ledgerdomain.LedgerEntryDirection,
	arDirection ledgerdomain.LedgerEntryDirection,
) error {
	now := time.Now().UTC()
	cashID, err := s.ensureLedgerAccount(ctx, stored.OrgID, ledgerdomain.AccountCodeCashClearing, "Cash / Clearing", now)
	if err != nil {
		return err
	}
	arID, err := s.ensureLedgerAccount(ctx, stored.OrgID, ledgerdomain.AccountCodeAccountsReceivable, "Accounts Receivable", now)
	if err != nil {
		return err
	}

	lines := []ledgerdomain.LedgerEntryLine{
		{AccountID: cashID, Direction: cashDirection, Amount: event.Amount},
		{AccountID: arID, Direction: arDirection, Amount: event.Amount},
	}

	return s.ledgerSvc.CreateEntry(
		ctx,
		stored.OrgID,
		ledgerdomain.SourceTypePaymentEvent,
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

func (s *Service) customerBalance(ctx context.Context, orgID snowflake.ID, customerID snowflake.ID, currency string) (int64, error) {
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return 0, paymentdomain.ErrInvalidCurrency
	}

	var balance int64
	err := s.db.WithContext(ctx).Raw(
		`SELECT COALESCE(SUM(CASE l.direction WHEN 'debit' THEN l.amount ELSE -l.amount END), 0) AS balance
		 FROM ledger_entries le
		 JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
		 JOIN ledger_accounts a ON a.id = l.account_id
		 LEFT JOIN billing_cycles bc ON bc.id = le.source_id AND le.source_type = ?
		 LEFT JOIN subscriptions s ON s.id = bc.subscription_id
		 LEFT JOIN payment_events pe ON pe.id = le.source_id AND le.source_type = ?
		 WHERE le.org_id = ?
		   AND a.code = ?
		   AND le.currency = ?
		   AND ((le.source_type = ? AND s.customer_id = ?)
		     OR (le.source_type = ? AND pe.customer_id = ?))`,
		ledgerdomain.SourceTypeBillingCycle,
		ledgerdomain.SourceTypePaymentEvent,
		orgID,
		ledgerdomain.AccountCodeAccountsReceivable,
		currency,
		ledgerdomain.SourceTypeBillingCycle,
		customerID,
		ledgerdomain.SourceTypePaymentEvent,
		customerID,
	).Scan(&balance).Error
	if err != nil {
		return 0, err
	}
	return balance, nil
}

func (s *Service) writeAuditLog(ctx context.Context, action string, stored *paymentdomain.EventRecord, event *paymentdomain.PaymentEvent, extra map[string]any) error {
	if s.auditSvc == nil {
		return errors.New("audit_service_unavailable")
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
	return s.auditSvc.AuditLog(ctx, &orgID, "", nil, action, "payment_event", &targetID, metadata)
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

func (s *Service) decryptConfig(encrypted datatypes.JSON) (map[string]any, error) {
	if len(s.encKey) == 0 {
		return nil, paymentproviderdomain.ErrEncryptionKeyMissing
	}
	if len(encrypted) == 0 {
		return nil, paymentdomain.ErrInvalidConfig
	}

	var payload encryptedPayload
	if err := json.Unmarshal(encrypted, &payload); err != nil {
		return nil, paymentdomain.ErrInvalidConfig
	}
	if payload.Version != 1 {
		return nil, paymentdomain.ErrInvalidConfig
	}

	nonce, err := base64.RawStdEncoding.DecodeString(payload.Nonce)
	if err != nil {
		return nil, paymentdomain.ErrInvalidConfig
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		return nil, paymentdomain.ErrInvalidConfig
	}

	block, err := aes.NewCipher(s.encKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, paymentdomain.ErrInvalidConfig
	}

	var out map[string]any
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, paymentdomain.ErrInvalidConfig
	}
	if len(out) == 0 {
		return nil, paymentdomain.ErrInvalidConfig
	}
	return out, nil
}
