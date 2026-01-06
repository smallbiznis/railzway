package service_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	"github.com/smallbiznis/valora/internal/config"
	ledgerdomain "github.com/smallbiznis/valora/internal/ledger/domain"
	ledgerservice "github.com/smallbiznis/valora/internal/ledger/service"
	"github.com/smallbiznis/valora/internal/payment/adapters"
	"github.com/smallbiznis/valora/internal/payment/adapters/stripe"
	disputerepo "github.com/smallbiznis/valora/internal/payment/dispute/repository"
	disputeservice "github.com/smallbiznis/valora/internal/payment/dispute/service"
	paymentdomain "github.com/smallbiznis/valora/internal/payment/domain"
	paymentrepo "github.com/smallbiznis/valora/internal/payment/repository"
	paymentservice "github.com/smallbiznis/valora/internal/payment/service"
	paymentwebhook "github.com/smallbiznis/valora/internal/payment/webhook"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type encryptedPayload struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type noopAuditService struct{}

func (noopAuditService) AuditLog(ctx context.Context, orgID *snowflake.ID, actorType string, actorID *string, action string, targetType string, targetID *string, metadata map[string]any) error {
	return nil
}

func (noopAuditService) List(ctx context.Context, req auditdomain.ListAuditLogRequest) (auditdomain.ListAuditLogResponse, error) {
	return auditdomain.ListAuditLogResponse{}, nil
}

func TestIngestWebhookCreatesLedgerEntry(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	node, err := snowflake.NewNode(10)
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	auditSvc := noopAuditService{}
	ledgerSvc := ledgerservice.NewService(ledgerservice.Params{
		DB:       db,
		Log:      zap.NewNop(),
		GenID:    node,
		AuditSvc: auditSvc,
	})

	configSecret := "config_secret"
	stripeSecret := "whsec_test"
	adapterRegistry := adapters.NewRegistry(stripe.NewFactory())
	paymentSvc := paymentservice.NewService(paymentservice.Params{
		DB:        db,
		Log:       zap.NewNop(),
		GenID:     node,
		LedgerSvc: ledgerSvc,
		AuditSvc:  auditSvc,
		Repo:      paymentrepo.Provide(),
	})
	disputeSvc := disputeservice.NewService(disputeservice.Params{
		DB:        db,
		Log:       zap.NewNop(),
		GenID:     node,
		LedgerSvc: ledgerSvc,
		AuditSvc:  auditSvc,
		Repo:      disputerepo.Provide(),
	})
	webhookSvc := paymentwebhook.NewService(paymentwebhook.Params{
		DB:         db,
		Log:        zap.NewNop(),
		PaymentSvc: paymentSvc,
		DisputeSvc: disputeSvc,
		Adapters:   adapterRegistry,
		Cfg:        config.Config{PaymentProviderConfigSecret: configSecret},
	})

	orgID := node.Generate()
	customerID := node.Generate()

	if err := seedCustomer(db, orgID, customerID); err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	configPayload, err := encryptConfig(configSecret, map[string]any{
		"webhook_secret": stripeSecret,
	})
	if err != nil {
		t.Fatalf("encrypt config: %v", err)
	}

	now := time.Now().UTC()
	if err := seedProviderConfig(db, node.Generate(), orgID, "stripe", configPayload, now); err != nil {
		t.Fatalf("seed provider config: %v", err)
	}

	payload := []byte(fmt.Sprintf(`{"id":"evt_1","type":"payment_intent.succeeded","created":%d,"data":{"object":{"id":"pi_1","amount":2000,"amount_received":2000,"currency":"usd","created":%d,"metadata":{"customer_id":"%s"}}}}`, now.Unix(), now.Unix(), customerID.String()))
	header := buildStripeSignatureHeader(stripeSecret, payload, now.Unix())

	reqHeader := http.Header{}
	reqHeader.Set("Stripe-Signature", header)

	if err := webhookSvc.IngestWebhook(ctx, "stripe", payload, reqHeader); err != nil {
		t.Fatalf("ingest webhook: %v", err)
	}

	assertCount(t, db, "SELECT COUNT(1) FROM payment_events", 1)
	assertCount(t, db, "SELECT COUNT(1) FROM ledger_entries", 1)
	assertCount(t, db, "SELECT COUNT(1) FROM ledger_entry_lines", 2)

	var eventType string
	if err := db.Raw("SELECT event_type FROM payment_events LIMIT 1").Scan(&eventType).Error; err != nil {
		t.Fatalf("scan event_type: %v", err)
	}
	if eventType != paymentdomain.EventTypePaymentSucceeded {
		t.Fatalf("expected event_type %s, got %s", paymentdomain.EventTypePaymentSucceeded, eventType)
	}

	var sourceType string
	if err := db.Raw("SELECT source_type FROM ledger_entries LIMIT 1").Scan(&sourceType).Error; err != nil {
		t.Fatalf("scan source_type: %v", err)
	}
	if sourceType != string(ledgerdomain.SourceTypePayment) {
		t.Fatalf("expected source_type %s, got %s", ledgerdomain.SourceTypePayment, sourceType)
	}

	var processedAt string
	if err := db.Raw("SELECT processed_at FROM payment_events LIMIT 1").Scan(&processedAt).Error; err != nil {
		t.Fatalf("scan processed_at: %v", err)
	}
	if processedAt == "" {
		t.Fatalf("expected processed_at to be set")
	}
}

func TestIngestWebhookCreatesDisputeLedgerEntry(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	node, err := snowflake.NewNode(11)
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	auditSvc := noopAuditService{}
	ledgerSvc := ledgerservice.NewService(ledgerservice.Params{
		DB:       db,
		Log:      zap.NewNop(),
		GenID:    node,
		AuditSvc: auditSvc,
	})

	configSecret := "config_secret"
	stripeSecret := "whsec_test"
	adapterRegistry := adapters.NewRegistry(stripe.NewFactory())
	paymentSvc := paymentservice.NewService(paymentservice.Params{
		DB:        db,
		Log:       zap.NewNop(),
		GenID:     node,
		LedgerSvc: ledgerSvc,
		AuditSvc:  auditSvc,
		Repo:      paymentrepo.Provide(),
	})
	disputeSvc := disputeservice.NewService(disputeservice.Params{
		DB:        db,
		Log:       zap.NewNop(),
		GenID:     node,
		LedgerSvc: ledgerSvc,
		AuditSvc:  auditSvc,
		Repo:      disputerepo.Provide(),
	})
	webhookSvc := paymentwebhook.NewService(paymentwebhook.Params{
		DB:         db,
		Log:        zap.NewNop(),
		PaymentSvc: paymentSvc,
		DisputeSvc: disputeSvc,
		Adapters:   adapterRegistry,
		Cfg:        config.Config{PaymentProviderConfigSecret: configSecret},
	})

	orgID := node.Generate()
	customerID := node.Generate()

	if err := seedCustomer(db, orgID, customerID); err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	configPayload, err := encryptConfig(configSecret, map[string]any{
		"webhook_secret": stripeSecret,
	})
	if err != nil {
		t.Fatalf("encrypt config: %v", err)
	}

	now := time.Now().UTC()
	if err := seedProviderConfig(db, node.Generate(), orgID, "stripe", configPayload, now); err != nil {
		t.Fatalf("seed provider config: %v", err)
	}

	payload := []byte(fmt.Sprintf(`{"id":"evt_2","type":"charge.dispute.funds_withdrawn","created":%d,"data":{"object":{"id":"dp_1","amount":1200,"currency":"usd","reason":"fraudulent","created":%d,"metadata":{"customer_id":"%s"}}}}`, now.Unix(), now.Unix(), customerID.String()))
	header := buildStripeSignatureHeader(stripeSecret, payload, now.Unix())

	reqHeader := http.Header{}
	reqHeader.Set("Stripe-Signature", header)

	if err := webhookSvc.IngestWebhook(ctx, "stripe", payload, reqHeader); err != nil {
		t.Fatalf("ingest webhook: %v", err)
	}

	assertCount(t, db, "SELECT COUNT(1) FROM payment_disputes", 1)
	assertCount(t, db, "SELECT COUNT(1) FROM ledger_entries", 1)
	assertCount(t, db, "SELECT COUNT(1) FROM ledger_entry_lines", 2)

	var status string
	if err := db.Raw("SELECT status FROM payment_disputes LIMIT 1").Scan(&status).Error; err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != "withdrawn" {
		t.Fatalf("expected status withdrawn, got %s", status)
	}

	var sourceType string
	if err := db.Raw("SELECT source_type FROM ledger_entries LIMIT 1").Scan(&sourceType).Error; err != nil {
		t.Fatalf("scan source_type: %v", err)
	}
	if sourceType != string(ledgerdomain.SourceTypePayment) {
		t.Fatalf("expected source_type %s, got %s", ledgerdomain.SourceTypePayment, sourceType)
	}
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:memdb_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	schema := []string{
		`CREATE TABLE payment_provider_configs (
			id BIGINT PRIMARY KEY,
			org_id BIGINT NOT NULL,
			provider TEXT NOT NULL,
			config TEXT NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE UNIQUE INDEX ux_payment_provider_configs_org_provider ON payment_provider_configs(org_id, provider)`,
		`CREATE TABLE payment_events (
			id BIGINT PRIMARY KEY,
			org_id BIGINT NOT NULL,
			provider TEXT NOT NULL,
			provider_event_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			customer_id BIGINT NOT NULL,
			payload TEXT NOT NULL,
			received_at TIMESTAMPTZ NOT NULL,
			processed_at TIMESTAMPTZ
		)`,
		`CREATE UNIQUE INDEX ux_payment_events_provider_event_id ON payment_events(provider, provider_event_id)`,
		`CREATE TABLE payment_disputes (
			id BIGINT PRIMARY KEY,
			org_id BIGINT NOT NULL,
			provider TEXT NOT NULL,
			provider_dispute_id TEXT NOT NULL,
			provider_event_id TEXT NOT NULL,
			customer_id BIGINT NOT NULL,
			amount BIGINT NOT NULL,
			currency TEXT NOT NULL,
			status TEXT NOT NULL,
			reason TEXT,
			received_at TIMESTAMPTZ NOT NULL,
			processed_at TIMESTAMPTZ
		)`,
		`CREATE UNIQUE INDEX ux_payment_disputes_provider_dispute_id ON payment_disputes(provider, provider_dispute_id)`,
		`CREATE TABLE ledger_accounts (
			id BIGINT PRIMARY KEY,
			org_id BIGINT NOT NULL,
			code TEXT NOT NULL,
			name TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE UNIQUE INDEX ux_ledger_accounts_org_code ON ledger_accounts(org_id, code)`,
		`CREATE TABLE ledger_entries (
			id BIGINT PRIMARY KEY,
			org_id BIGINT NOT NULL,
			source_type TEXT NOT NULL,
			source_id BIGINT NOT NULL,
			currency TEXT NOT NULL,
			occurred_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE UNIQUE INDEX ux_ledger_entries_source ON ledger_entries(org_id, source_type, source_id)`,
		`CREATE TABLE ledger_entry_lines (
			id BIGINT PRIMARY KEY,
			ledger_entry_id BIGINT NOT NULL,
			account_id BIGINT NOT NULL,
			direction TEXT NOT NULL,
			amount BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE audit_logs (
			id BIGINT PRIMARY KEY,
			org_id BIGINT,
			actor_type TEXT NOT NULL,
			actor_id TEXT,
			action TEXT NOT NULL,
			target_type TEXT NOT NULL,
			target_id TEXT,
		metadata TEXT NOT NULL,
		ip_address TEXT,
		user_agent TEXT,
		created_at TIMESTAMPTZ NOT NULL
	)`,
		`CREATE TABLE customers (
			id BIGINT PRIMARY KEY,
			org_id BIGINT NOT NULL,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE subscriptions (
			id BIGINT PRIMARY KEY,
			org_id BIGINT NOT NULL,
			customer_id BIGINT NOT NULL
		)`,
		`CREATE TABLE billing_cycles (
			id BIGINT PRIMARY KEY,
			org_id BIGINT NOT NULL,
			subscription_id BIGINT NOT NULL
		)`,
	}

	for _, stmt := range schema {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("schema exec failed: %v", err)
		}
	}

	return db
}

func seedCustomer(db *gorm.DB, orgID, customerID snowflake.ID) error {
	return db.Exec(
		"INSERT INTO customers (id, org_id, name) VALUES (?, ?, ?)",
		customerID,
		orgID,
		"Acme Co",
	).Error
}

func seedProviderConfig(db *gorm.DB, id, orgID snowflake.ID, provider string, config []byte, now time.Time) error {
	return db.Exec(
		"INSERT INTO payment_provider_configs (id, org_id, provider, config, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id,
		orgID,
		provider,
		config,
		true,
		now,
		now,
	).Error
}

func encryptConfig(secret string, config map[string]any) ([]byte, error) {
	payload, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, payload, nil)
	encoded := encryptedPayload{
		Version:    1,
		Nonce:      base64.RawStdEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext),
	}
	return json.Marshal(encoded)
}

func buildStripeSignatureHeader(secret string, payload []byte, timestamp int64) string {
	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signedPayload))
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
}

func assertCount(t *testing.T, db *gorm.DB, query string, expected int64) {
	t.Helper()

	var count int64
	if err := db.Raw(query).Scan(&count).Error; err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != expected {
		t.Fatalf("expected %d, got %d", expected, count)
	}
}
