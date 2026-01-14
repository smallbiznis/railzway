package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	"github.com/smallbiznis/railzway/internal/apikey"
	"github.com/smallbiznis/railzway/internal/audit"
	auditdomain "github.com/smallbiznis/railzway/internal/audit/domain"
	"github.com/smallbiznis/railzway/internal/auth"
	authlocal "github.com/smallbiznis/railzway/internal/auth/local"
	authoauth2provider "github.com/smallbiznis/railzway/internal/auth/oauth2provider"
	"github.com/smallbiznis/railzway/internal/auth/session"
	"github.com/smallbiznis/railzway/internal/authorization"
	"github.com/smallbiznis/railzway/internal/billingcycle"
	"github.com/smallbiznis/railzway/internal/billingdashboard"
	"github.com/smallbiznis/railzway/internal/billingoperations"
	"github.com/smallbiznis/railzway/internal/billingoverview"
	"github.com/smallbiznis/railzway/internal/clock"
	"github.com/smallbiznis/railzway/internal/cloudmetrics"
	"github.com/smallbiznis/railzway/internal/config"
	"github.com/smallbiznis/railzway/internal/customer"
	"github.com/smallbiznis/railzway/internal/events"
	"github.com/smallbiznis/railzway/internal/feature"
	"github.com/smallbiznis/railzway/internal/invoice"
	invoicedomain "github.com/smallbiznis/railzway/internal/invoice/domain"
	"github.com/smallbiznis/railzway/internal/invoicetemplate"
	"github.com/smallbiznis/railzway/internal/ledger"
	ledgerdomain "github.com/smallbiznis/railzway/internal/ledger/domain"
	"github.com/smallbiznis/railzway/internal/meter"
	"github.com/smallbiznis/railzway/internal/migration"
	"github.com/smallbiznis/railzway/internal/observability"
	"github.com/smallbiznis/railzway/internal/organization"
	"github.com/smallbiznis/railzway/internal/payment"
	"github.com/smallbiznis/railzway/internal/price"
	"github.com/smallbiznis/railzway/internal/priceamount"
	"github.com/smallbiznis/railzway/internal/pricetier"
	"github.com/smallbiznis/railzway/internal/product"
	"github.com/smallbiznis/railzway/internal/productfeature"
	emailprovider "github.com/smallbiznis/railzway/internal/providers/email"
	paymentprovider "github.com/smallbiznis/railzway/internal/providers/payment"
	pdfprovider "github.com/smallbiznis/railzway/internal/providers/pdf"
	"github.com/smallbiznis/railzway/internal/publicinvoice"
	"github.com/smallbiznis/railzway/internal/rating"
	ratingdomain "github.com/smallbiznis/railzway/internal/rating/domain"
	"github.com/smallbiznis/railzway/internal/reference"
	"github.com/smallbiznis/railzway/internal/scheduler"
	"github.com/smallbiznis/railzway/internal/seed"
	"github.com/smallbiznis/railzway/internal/server"
	"github.com/smallbiznis/railzway/internal/subscription"
	subscriptiondomain "github.com/smallbiznis/railzway/internal/subscription/domain"
	"github.com/smallbiznis/railzway/internal/usage"
	"github.com/smallbiznis/railzway/internal/usage/snapshot"
	"github.com/smallbiznis/railzway/pkg/db"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type testEnv struct {
	app            *fx.App
	server         *server.Server
	db             *gorm.DB
	baseURL        string
	scheduler      *scheduler.Scheduler
	snapshotWorker *snapshot.Worker
	httpSrv        *httptest.Server
}

var env *testEnv

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	setDefaultEnv()

	var err error
	env, err = startEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to start test environment:", err)
		os.Exit(1)
	}

	code := m.Run()
	env.shutdown()
	os.Exit(code)
}

func TestE2E_HealthCheck(t *testing.T) {
	resetDatabase(t, env.db)

	resp, err := http.Get(env.baseURL + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestE2E_BootstrapDefaultOrgAndAdmin(t *testing.T) {
	resetDatabase(t, env.db)

	org := struct {
		ID        int64
		Name      string
		Slug      string
		IsDefault bool
	}{}
	if err := env.db.Raw(
		`SELECT id, name, slug, is_default FROM organizations WHERE slug = ?`,
		"main",
	).Scan(&org).Error; err != nil {
		t.Fatalf("query default org: %v", err)
	}
	if org.ID == 0 || !org.IsDefault {
		t.Fatalf("default org not found")
	}

	user := struct {
		ID        int64
		Email     string
		IsDefault bool
	}{}
	if err := env.db.Raw(
		`SELECT id, email, is_default FROM users WHERE email = ?`,
		"admin@valora.cloud",
	).Scan(&user).Error; err != nil {
		t.Fatalf("query admin user: %v", err)
	}
	if user.ID == 0 || !user.IsDefault {
		t.Fatalf("default admin not found")
	}

	client, orgID := loginAdmin(t)
	if orgID == "" {
		t.Fatalf("expected org id after login")
	}

	reqURL := env.baseURL + "/auth/user/orgs"
	resp, body := doJSON(t, client, http.MethodGet, reqURL, nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 for orgs, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestE2E_APIKeyAuthentication(t *testing.T) {
	resetDatabase(t, env.db)

	client, orgID := loginAdmin(t)
	fixture := createBillingFixture(t, client, orgID)
	activateSubscription(t, client, orgID, fixture.SubscriptionID)

	apiKey := createAPIKey(t, client, orgID)

	usageReq := map[string]any{
		"customer_id":     fixture.CustomerID,
		"meter_code":      fixture.MeterCode,
		"value":           3.0,
		"recorded_at":     time.Now().UTC(),
		"idempotency_key": fmt.Sprintf("e2e-%d", time.Now().UnixNano()),
		"metadata": map[string]any{
			"source": "e2e",
		},
	}
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
	resp, body := doJSON(t, newHTTPClient(), http.MethodPost, env.baseURL+"/api/usage", usageReq, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 for usage ingest, got %d: %s", resp.StatusCode, string(body))
	}

	usageReq["idempotency_key"] = fmt.Sprintf("e2e-%d", time.Now().UnixNano())
	resp, body = doJSON(t, newHTTPClient(), http.MethodPost, env.baseURL+"/api/usage", usageReq, map[string]string{
		"Authorization": "Bearer invalid",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for invalid api key, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestE2E_SubscriptionLifecycle(t *testing.T) {
	resetDatabase(t, env.db)

	client, orgID := loginAdmin(t)
	fixture := createBillingFixture(t, client, orgID)

	if countRows(t, env.db, "invoices", "subscription_id = ?", mustParseID(t, fixture.SubscriptionID)) != 0 {
		t.Fatalf("expected zero invoices before activation")
	}

	if err := env.scheduler.EnsureBillingCyclesJob(context.Background()); err != nil {
		t.Fatalf("ensure billing cycles: %v", err)
	}
	if countRows(t, env.db, "billing_cycles", "subscription_id = ?", mustParseID(t, fixture.SubscriptionID)) != 0 {
		t.Fatalf("expected zero billing cycles before activation")
	}

	activateSubscription(t, client, orgID, fixture.SubscriptionID)
	status := getSubscriptionStatus(t, client, orgID, fixture.SubscriptionID)
	if status != "ACTIVE" {
		t.Fatalf("expected status ACTIVE, got %s", status)
	}

	if err := env.scheduler.EnsureBillingCyclesJob(context.Background()); err != nil {
		t.Fatalf("ensure billing cycles after activation: %v", err)
	}
	if countRows(t, env.db, "billing_cycles", "subscription_id = ?", mustParseID(t, fixture.SubscriptionID)) == 0 {
		t.Fatalf("expected billing cycle after activation")
	}

	cancelSubscription(t, client, orgID, fixture.SubscriptionID)
	status = getSubscriptionStatus(t, client, orgID, fixture.SubscriptionID)
	if status != "CANCELED" {
		t.Fatalf("expected status CANCELED, got %s", status)
	}
}

func TestE2E_BillingCycleAndInvoice(t *testing.T) {
	resetDatabase(t, env.db)

	client, orgID := loginAdmin(t)
	fixture := createBillingFixture(t, client, orgID)
	activateSubscription(t, client, orgID, fixture.SubscriptionID)

	if err := env.scheduler.EnsureBillingCyclesJob(context.Background()); err != nil {
		t.Fatalf("ensure billing cycles: %v", err)
	}

	cycle := billingCycleRow{}
	if err := env.db.Raw(
		`SELECT id, status, period_start, period_end FROM billing_cycles WHERE subscription_id = ?`,
		mustParseID(t, fixture.SubscriptionID),
	).Scan(&cycle).Error; err != nil {
		t.Fatalf("query billing cycle: %v", err)
	}
	if cycle.ID == 0 {
		t.Fatalf("expected billing cycle created")
	}

	now := time.Now().UTC()
	periodStart := now.Add(-2 * time.Hour)
	periodEnd := now.Add(-1 * time.Hour)
	if err := env.db.Exec(
		`UPDATE billing_cycles SET period_start = ?, period_end = ? WHERE id = ?`,
		periodStart,
		periodEnd,
		cycle.ID,
	).Error; err != nil {
		t.Fatalf("fast-forward billing cycle: %v", err)
	}

	if err := env.scheduler.RunOnce(context.Background()); err != nil {
		t.Fatalf("scheduler run: %v", err)
	}

	invoice := invoiceRow{}
	if err := env.db.Raw(
		`SELECT id, status, subtotal_amount, updated_at FROM invoices WHERE subscription_id = ?`,
		mustParseID(t, fixture.SubscriptionID),
	).Scan(&invoice).Error; err != nil {
		t.Fatalf("query invoice: %v", err)
	}
	if invoice.ID == 0 {
		t.Fatalf("expected invoice generated")
	}
	if countRows(t, env.db, "invoices", "subscription_id = ?", mustParseID(t, fixture.SubscriptionID)) != 1 {
		t.Fatalf("expected single invoice after scheduler run")
	}

	if err := env.scheduler.RunOnce(context.Background()); err != nil {
		t.Fatalf("scheduler second run: %v", err)
	}
	if countRows(t, env.db, "invoices", "subscription_id = ?", mustParseID(t, fixture.SubscriptionID)) != 1 {
		t.Fatalf("expected invoice count unchanged")
	}

	invoiceAfter := invoiceRow{}
	if err := env.db.Raw(
		`SELECT id, status, subtotal_amount, updated_at FROM invoices WHERE subscription_id = ?`,
		mustParseID(t, fixture.SubscriptionID),
	).Scan(&invoiceAfter).Error; err != nil {
		t.Fatalf("query invoice after: %v", err)
	}
	if invoiceAfter.ID != invoice.ID {
		t.Fatalf("expected invoice id stable")
	}
	if invoiceAfter.SubtotalAmount != invoice.SubtotalAmount {
		t.Fatalf("expected invoice amount immutable")
	}
	if !invoiceAfter.UpdatedAt.Equal(invoice.UpdatedAt) {
		t.Fatalf("expected invoice updated_at immutable")
	}
}

func TestE2E_AuditLog(t *testing.T) {
	resetDatabase(t, env.db)

	client, orgID := loginAdmin(t)
	fixture := createBillingFixture(t, client, orgID)
	activateSubscription(t, client, orgID, fixture.SubscriptionID)

	var adminID snowflake.ID
	if err := env.db.Raw(
		`SELECT id FROM users WHERE email = ?`,
		"admin@valora.cloud",
	).Scan(&adminID).Error; err != nil {
		t.Fatalf("query admin id: %v", err)
	}
	if adminID == 0 {
		t.Fatalf("admin id not found")
	}

	logEntry := auditdomain.AuditLog{}
	if err := env.db.Raw(
		`SELECT id, actor_type, actor_id, action FROM audit_logs WHERE action = ? ORDER BY created_at DESC LIMIT 1`,
		"subscription.activate",
	).Scan(&logEntry).Error; err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if logEntry.ID == 0 {
		t.Fatalf("expected audit log entry")
	}
	if logEntry.ActorType != string(auditdomain.ActorTypeUser) {
		t.Fatalf("expected actor_type user, got %s", logEntry.ActorType)
	}
	adminIDString := adminID.String()
	if logEntry.ActorID == nil || *logEntry.ActorID != adminIDString {
		t.Fatalf("expected actor_id %s, got %v", adminIDString, logEntry.ActorID)
	}
}

type billingFixture struct {
	CustomerID     string
	MeterCode      string
	SubscriptionID string
}

type billingCycleRow struct {
	ID          snowflake.ID `gorm:"column:id"`
	Status      string       `gorm:"column:status"`
	PeriodStart time.Time    `gorm:"column:period_start"`
	PeriodEnd   time.Time    `gorm:"column:period_end"`
}

type invoiceRow struct {
	ID             snowflake.ID `gorm:"column:id"`
	Status         string       `gorm:"column:status"`
	SubtotalAmount int64        `gorm:"column:subtotal_amount"`
	UpdatedAt      time.Time    `gorm:"column:updated_at"`
}

func startEnv() (*testEnv, error) {
	var (
		srv         *server.Server
		dbConn      *gorm.DB
		cfg         config.Config
		log         *zap.Logger
		genID       *snowflake.Node
		ratingSvc   ratingdomain.Service
		invoiceSvc  invoicedomain.Service
		ledgerSvc   ledgerdomain.Service
		subSvc      subscriptiondomain.Service
		auditSvc    auditdomain.Service
		authzSvc    authorization.Service
		snapWorker  *snapshot.Worker
		httpSrv     *httptest.Server
		schedulerSv *scheduler.Scheduler
	)

	app := fx.New(
		observability.Module,
		config.Module,
		db.Module,
		clock.Module,
		cloudmetrics.Module,
		authorization.Module,
		audit.Module,
		events.Module,
		auth.Module,
		authlocal.Module,
		authoauth2provider.Module,
		session.Module,
		apikey.Module,
		customer.Module,
		billingdashboard.Module,
		billingoperations.Module,
		billingoverview.Module,
		emailprovider.Module,
		pdfprovider.Module,
		invoice.Module,
		invoicetemplate.Module,
		ledger.Module,
		meter.Module,
		organization.Module,
		payment.Module,
		paymentprovider.Module,
		price.Module,
		priceamount.Module,
		pricetier.Module,
		product.Module,
		productfeature.Module,
		feature.Module,
		publicinvoice.Module,
		reference.Module,
		subscription.Module,
		usage.Module,
		rating.Module,
		billingcycle.Module,
		migration.Module,
		fx.Provide(scheduler.New),
		fx.Provide(func() *snowflake.Node {
			node, err := snowflake.NewNode(1)
			if err != nil {
				panic(err)
			}
			return node
		}),
		fx.Provide(server.NewEngine),
		fx.Provide(server.NewServer),
		fx.Invoke(server.RegisterRoutes),
		fx.Populate(&srv, &dbConn, &cfg, &log, &genID, &ratingSvc, &invoiceSvc, &ledgerSvc, &subSvc, &auditSvc, &authzSvc, &snapWorker, &schedulerSv),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := app.Start(ctx); err != nil {
		return nil, err
	}

	if strings.ToLower(strings.TrimSpace(cfg.DBType)) != "postgres" {
		app.Stop(context.Background())
		return nil, fmt.Errorf("expected postgres db, got %s", cfg.DBType)
	}

	sqlDB, err := dbConn.DB()
	if err != nil {
		app.Stop(context.Background())
		return nil, err
	}
	if err := migration.RunMigrations(sqlDB); err != nil {
		app.Stop(context.Background())
		return nil, err
	}
	if err := seed.EnsureMainOrg(dbConn); err != nil {
		app.Stop(context.Background())
		return nil, err
	}

	httpSrv = httptest.NewServer(srv.Engine())

	return &testEnv{
		app:            app,
		server:         srv,
		db:             dbConn,
		baseURL:        httpSrv.URL,
		scheduler:      schedulerSv,
		snapshotWorker: snapWorker,
		httpSrv:        httpSrv,
	}, nil
}

func (e *testEnv) shutdown() {
	if e == nil {
		return
	}
	if e.httpSrv != nil {
		e.httpSrv.Close()
	}
	if e.app != nil {
		_ = e.app.Stop(context.Background())
	}
}

func setDefaultEnv() {
	setEnvIfEmpty("ENVIRONMENT", "test")
	setEnvIfEmpty("APP_MODE", "oss")
	setEnvIfEmpty("ENSURE_DEFAULT_ORG_AND_USER", "true")
	setEnvIfEmpty("AUTH_COOKIE_SECURE", "false")
	setEnvIfEmpty("LOG_LEVEL", "error")
}

func setEnvIfEmpty(key, value string) {
	if strings.TrimSpace(os.Getenv(key)) != "" {
		return
	}
	_ = os.Setenv(key, value)
}

func resetDatabase(t *testing.T, dbConn *gorm.DB) {
	t.Helper()
	if err := truncateAllTables(dbConn); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
	if err := seed.EnsureMainOrgAndAdmin(dbConn); err != nil {
		t.Fatalf("seed default org and admin: %v", err)
	}
}

func truncateAllTables(dbConn *gorm.DB) error {
	type tableRow struct {
		Name string `gorm:"column:tablename"`
	}
	var rows []tableRow
	if err := dbConn.Raw(
		`SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'schema_migrations'`,
	).Scan(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	tables := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Name) == "" {
			continue
		}
		tables = append(tables, `"`+row.Name+`"`)
	}
	if len(tables) == 0 {
		return nil
	}

	stmt := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(tables, ", "))
	return dbConn.Exec(stmt).Error
}

func loginAdmin(t *testing.T) (*http.Client, string) {
	t.Helper()
	client := newHTTPClient()

	req := map[string]any{
		"email":    "admin@valora.cloud",
		"password": "admin",
	}
	resp, body := doJSON(t, client, http.MethodPost, env.baseURL+"/auth/login", req, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: %d: %s", resp.StatusCode, string(body))
	}

	baseURL, err := url.Parse(env.baseURL)
	if err == nil {
		cookies := client.Jar.Cookies(baseURL)
		found := false
		for _, cookie := range cookies {
			if cookie.Name == "_sid" && strings.TrimSpace(cookie.Value) != "" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected session cookie after login")
		}
	}

	reqURL := env.baseURL + "/auth/user/orgs"
	resp, body = doJSON(t, client, http.MethodGet, reqURL, nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list orgs failed: %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		Orgs []struct {
			ID string `json:"id"`
		} `json:"orgs"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode orgs: %v", err)
	}
	if len(payload.Orgs) == 0 {
		t.Fatalf("no orgs returned")
	}
	return client, strings.TrimSpace(payload.Orgs[0].ID)
}

func createBillingFixture(t *testing.T, client *http.Client, orgID string) billingFixture {
	t.Helper()

	headers := map[string]string{
		server.HeaderOrg: orgID,
	}

	meterResp := struct {
		Data struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		} `json:"data"`
	}{}
	meterReq := map[string]any{
		"code":             "e2e-meter",
		"name":             "E2E Meter",
		"aggregation_type": "SUM",
		"unit":             "API_CALL",
	}
	resp, body := doJSON(t, client, http.MethodPost, env.baseURL+"/admin/meters", meterReq, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create meter failed: %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, &meterResp); err != nil {
		t.Fatalf("decode meter response: %v", err)
	}

	productResp := struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}{}
	productReq := map[string]any{
		"code": "e2e-product",
		"name": "E2E Product",
	}
	resp, body = doJSON(t, client, http.MethodPost, env.baseURL+"/admin/products", productReq, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create product failed: %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, &productResp); err != nil {
		t.Fatalf("decode product response: %v", err)
	}

	priceResp := struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}{}
	priceReq := map[string]any{
		"product_id":             productResp.Data.ID,
		"code":                   "e2e-price",
		"pricing_model":          "PER_UNIT",
		"billing_mode":           "METERED",
		"billing_interval":       "MONTH",
		"billing_interval_count": 1,
		"aggregate_usage":        "SUM",
		"billing_unit":           "API_CALL",
		"tax_behavior":           "EXCLUSIVE",
	}
	resp, body = doJSON(t, client, http.MethodPost, env.baseURL+"/admin/prices", priceReq, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create price failed: %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, &priceResp); err != nil {
		t.Fatalf("decode price response: %v", err)
	}

	priceAmountReq := map[string]any{
		"price_id":          priceResp.Data.ID,
		"meter_id":          meterResp.Data.ID,
		"currency":          "USD",
		"unit_amount_cents": 100,
	}
	resp, body = doJSON(t, client, http.MethodPost, env.baseURL+"/admin/price_amounts", priceAmountReq, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create price amount failed: %d: %s", resp.StatusCode, string(body))
	}

	customerResp := struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}{}
	customerReq := map[string]any{
		"name":  "E2E Customer",
		"email": "e2e@example.com",
	}
	resp, body = doJSON(t, client, http.MethodPost, env.baseURL+"/admin/customers", customerReq, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create customer failed: %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, &customerResp); err != nil {
		t.Fatalf("decode customer response: %v", err)
	}

	subscriptionResp := struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}{}
	subscriptionReq := map[string]any{
		"customer_id":        customerResp.Data.ID,
		"collection_mode":    "SEND_INVOICE",
		"billing_cycle_type": "MONTHLY",
		"items": []map[string]any{
			{"price_id": priceResp.Data.ID, "meter_id": meterResp.Data.ID, "quantity": 1},
		},
	}
	resp, body = doJSON(t, client, http.MethodPost, env.baseURL+"/admin/subscriptions", subscriptionReq, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create subscription failed: %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, &subscriptionResp); err != nil {
		t.Fatalf("decode subscription response: %v", err)
	}
	if subscriptionResp.Data.Status != "DRAFT" {
		t.Fatalf("expected subscription status DRAFT, got %s", subscriptionResp.Data.Status)
	}

	return billingFixture{
		CustomerID:     customerResp.Data.ID,
		MeterCode:      meterResp.Data.Code,
		SubscriptionID: subscriptionResp.Data.ID,
	}
}

func activateSubscription(t *testing.T, client *http.Client, orgID, subscriptionID string) {
	t.Helper()
	headers := map[string]string{server.HeaderOrg: orgID}
	resp, body := doJSON(t, client, http.MethodPost, env.baseURL+"/admin/subscriptions/"+subscriptionID+"/activate", nil, headers)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("activate subscription failed: %d: %s", resp.StatusCode, string(body))
	}
}

func cancelSubscription(t *testing.T, client *http.Client, orgID, subscriptionID string) {
	t.Helper()
	headers := map[string]string{server.HeaderOrg: orgID}
	resp, body := doJSON(t, client, http.MethodPost, env.baseURL+"/admin/subscriptions/"+subscriptionID+"/cancel", nil, headers)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("cancel subscription failed: %d: %s", resp.StatusCode, string(body))
	}
}

func getSubscriptionStatus(t *testing.T, client *http.Client, orgID, subscriptionID string) string {
	t.Helper()
	headers := map[string]string{server.HeaderOrg: orgID}
	resp, body := doJSON(t, client, http.MethodGet, env.baseURL+"/admin/subscriptions/"+subscriptionID, nil, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get subscription failed: %d: %s", resp.StatusCode, string(body))
	}
	var payload struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode subscription response: %v", err)
	}
	return payload.Data.Status
}

func createAPIKey(t *testing.T, client *http.Client, orgID string) string {
	t.Helper()
	headers := map[string]string{server.HeaderOrg: orgID}
	req := map[string]any{"name": "E2E Key"}
	resp, body := doJSON(t, client, http.MethodPost, env.baseURL+"/admin/api-keys", req, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create api key failed: %d: %s", resp.StatusCode, string(body))
	}
	var payload struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode api key response: %v", err)
	}
	if strings.TrimSpace(payload.APIKey) == "" {
		t.Fatalf("expected api key value")
	}
	return payload.APIKey
}

func countRows(t *testing.T, dbConn *gorm.DB, table string, where string, args ...any) int64 {
	t.Helper()
	var count int64
	if err := dbConn.Table(table).Where(where, args...).Count(&count).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func mustParseID(t *testing.T, value string) snowflake.ID {
	t.Helper()
	parsed, err := snowflake.ParseString(strings.TrimSpace(value))
	if err != nil || parsed == 0 {
		t.Fatalf("invalid snowflake id: %s", value)
	}
	return parsed
}

func doJSON(t *testing.T, client *http.Client, method, reqURL string, payload any, headers map[string]string) (*http.Response, []byte) {
	t.Helper()

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("encode json: %v", err)
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return resp, data
}

func newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: 15 * time.Second,
		Jar:     jar,
	}
}
