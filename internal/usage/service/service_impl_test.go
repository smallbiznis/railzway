package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/cache"
	meterdomain "github.com/smallbiznis/valora/internal/meter/domain"
	"github.com/smallbiznis/valora/internal/orgcontext"
	usagedomain "github.com/smallbiznis/valora/internal/usage/domain"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type meterStub struct {
	mu       sync.Mutex
	calls    int
	response *meterdomain.Response
	err      error
}

func (m *meterStub) Create(ctx context.Context, req meterdomain.CreateRequest) (*meterdomain.Response, error) {
	return nil, m.err
}

func (m *meterStub) List(ctx context.Context, req meterdomain.ListRequest) ([]meterdomain.Response, error) {
	return nil, m.err
}

func (m *meterStub) GetByID(ctx context.Context, id string) (*meterdomain.Response, error) {
	return nil, m.err
}

func (m *meterStub) GetByCode(ctx context.Context, code string) (*meterdomain.Response, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *meterStub) Update(ctx context.Context, req meterdomain.UpdateRequest) (*meterdomain.Response, error) {
	return nil, m.err
}

func (m *meterStub) Delete(ctx context.Context, id string) error {
	return m.err
}

func (m *meterStub) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func TestIngestIdempotentInsert(t *testing.T) {
	node := mustNode(t)
	orgID := node.Generate()
	customerID := node.Generate()
	meterID := node.Generate()

	meter := &meterStub{
		response: &meterdomain.Response{
			ID:   meterID.String(),
			Code: "api_calls",
		},
	}
	service, db := setupUsageService(t, node, meter, cache.NewUsageResolverCache(), orgID, customerID)
	ctx := orgcontext.WithOrgID(context.Background(), int64(orgID))

	key := "idem-key"
	req := usagedomain.CreateIngestRequest{
		CustomerID:     customerID.String(),
		MeterCode:      "api_calls",
		Value:          42,
		RecordedAt:     time.Now().UTC(),
		IdempotencyKey: key,
	}

	first, err := service.Ingest(ctx, req)
	if err != nil {
		t.Fatalf("ingest first: %v", err)
	}

	second, err := service.Ingest(ctx, req)
	if err != nil {
		t.Fatalf("ingest second: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected idempotent usage event, got %s vs %s", first.ID.String(), second.ID.String())
	}

	if count := countUsageEvents(t, db); count != 1 {
		t.Fatalf("expected 1 usage event, got %d", count)
	}
}

func TestIngestConcurrentIdempotent(t *testing.T) {
	node := mustNode(t)
	orgID := node.Generate()
	customerID := node.Generate()
	meterID := node.Generate()

	meter := &meterStub{
		response: &meterdomain.Response{
			ID:   meterID.String(),
			Code: "storage_gb",
		},
	}
	service, db := setupUsageService(t, node, meter, cache.NewUsageResolverCache(), orgID, customerID)
	ctx := orgcontext.WithOrgID(context.Background(), int64(orgID))

	key := "idem-concurrent"
	req := usagedomain.CreateIngestRequest{
		CustomerID:     customerID.String(),
		MeterCode:      "storage_gb",
		Value:          3.14,
		RecordedAt:     time.Now().UTC(),
		IdempotencyKey: key,
	}

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.Ingest(ctx, req)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("ingest concurrent: %v", err)
		}
	}

	if count := countUsageEvents(t, db); count != 1 {
		t.Fatalf("expected 1 usage event after concurrent ingest, got %d", count)
	}
}

func TestIngestDoesNotResolveMeter(t *testing.T) {
	node := mustNode(t)
	orgID := node.Generate()
	customerID := node.Generate()
	meterID := node.Generate()

	meter := &meterStub{
		response: &meterdomain.Response{
			ID:   meterID.String(),
			Code: "requests",
		},
	}
	resolverCache := cache.NewUsageResolverCache()
	service, _ := setupUsageService(t, node, meter, resolverCache, orgID, customerID)
	ctx := orgcontext.WithOrgID(context.Background(), int64(orgID))

	key := "idem-cache"
	req := usagedomain.CreateIngestRequest{
		CustomerID:     customerID.String(),
		MeterCode:      "requests",
		Value:          1,
		RecordedAt:     time.Now().UTC(),
		IdempotencyKey: key,
	}

	if _, err := service.Ingest(ctx, req); err != nil {
		t.Fatalf("ingest cache miss: %v", err)
	}
	if meter.Calls() != 0 {
		t.Fatalf("expected no meter lookup during ingest, got %d", meter.Calls())
	}

	if _, err := service.Ingest(ctx, req); err != nil {
		t.Fatalf("ingest cache hit: %v", err)
	}
	if meter.Calls() != 0 {
		t.Fatalf("expected no meter lookup during ingest, got %d", meter.Calls())
	}
}

func setupUsageService(
	t *testing.T,
	node *snowflake.Node,
	meterSvc meterdomain.Service,
	resolverCache cache.UsageResolverCache,
	orgID, customerID snowflake.ID,
) (usagedomain.Service, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_loc=auto", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	_ = db.Exec("PRAGMA busy_timeout = 5000").Error
	_ = db.Exec("PRAGMA journal_mode = WAL").Error
	prepareUsageSchema(t, db)
	seedCustomer(t, db, orgID, customerID)

	service := NewService(ServiceParam{
		DB:            db,
		Log:           zap.NewNop(),
		GenID:         node,
		MeterSvc:      meterSvc,
		ResolverCache: resolverCache,
	})

	return service, db
}

func prepareUsageSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE customers (
		id BIGINT PRIMARY KEY,
		org_id BIGINT NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create customers: %v", err)
	}
	if err := db.Exec(`CREATE TABLE usage_events (
		id BIGINT PRIMARY KEY,
		org_id BIGINT NOT NULL,
		customer_id BIGINT NOT NULL,
		subscription_id BIGINT NOT NULL,
		subscription_item_id BIGINT,
		meter_id BIGINT NOT NULL,
		meter_code TEXT NOT NULL,
		value DOUBLE PRECISION NOT NULL,
		recorded_at DATETIME NOT NULL,
		status TEXT NOT NULL DEFAULT 'accepted',
		error TEXT,
		idempotency_key TEXT,
		metadata JSON,
		snapshot_at DATETIME,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create usage_events: %v", err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX uidx_usage_idempotency_key
		ON usage_events (org_id, idempotency_key)`).Error; err != nil {
		t.Fatalf("create usage idempotency index: %v", err)
	}
}

func seedCustomer(t *testing.T, db *gorm.DB, orgID, customerID snowflake.ID) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO customers (id, org_id) VALUES (?, ?)`,
		customerID,
		orgID,
	).Error; err != nil {
		t.Fatalf("seed customer: %v", err)
	}
}

func countUsageEvents(t *testing.T, db *gorm.DB) int {
	t.Helper()
	var count int
	if err := db.Raw(`SELECT COUNT(1) FROM usage_events`).Scan(&count).Error; err != nil {
		t.Fatalf("count usage events: %v", err)
	}
	return count
}

func mustNode(t *testing.T) *snowflake.Node {
	t.Helper()
	node, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatalf("new node: %v", err)
	}
	return node
}
