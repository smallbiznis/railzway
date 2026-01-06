package rollup

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/billingcycle/domain"
	billingevent "github.com/smallbiznis/valora/internal/billingevent/domain"
	customerdomain "github.com/smallbiznis/valora/internal/customer/domain"
	"github.com/smallbiznis/valora/internal/events"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	ledgerdomain "github.com/smallbiznis/valora/internal/ledger/domain"
	paymentdomain "github.com/smallbiznis/valora/internal/payment/domain"
	subscriptiondomain "github.com/smallbiznis/valora/internal/subscription/domain"
	"github.com/smallbiznis/valora/pkg/db"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type customerBalance struct {
	OrgID      snowflake.ID `gorm:"primaryKey"`
	CustomerID snowflake.ID `gorm:"primaryKey"`
	Currency   string       `gorm:"primaryKey"`
	Balance    int64
	UpdatedAt  time.Time
}

func (customerBalance) TableName() string { return "customer_balances" }

type billingCycleStats struct {
	BillingCycleID snowflake.ID `gorm:"primaryKey"`
	OrgID          snowflake.ID
	PeriodStart    time.Time
	Status         string
	TotalRevenue   int64
	InvoiceCount   int
	UpdatedAt      time.Time
}

func (billingCycleStats) TableName() string { return "billing_cycle_stats" }

func TestRollupIdempotency(t *testing.T) {
	dbConn, node := setupRollupDB(t)
	seed := seedRollupData(t, dbConn, node)

	svc := NewService(Params{DB: dbConn, Log: zap.NewNop(), GenID: node})
	ctx := context.Background()

	if err := svc.ProcessPending(ctx, 10); err != nil {
		t.Fatalf("process pending: %v", err)
	}

	balanceBefore := loadCustomerBalance(t, dbConn, seed.OrgID, seed.CustomerID)
	statsBefore := loadCycleStats(t, dbConn, seed.BillingCycleID)
	if balanceBefore != 1000 {
		t.Fatalf("expected balance 1000, got %d", balanceBefore)
	}
	if statsBefore.TotalRevenue != 1000 {
		t.Fatalf("expected revenue 1000, got %d", statsBefore.TotalRevenue)
	}
	if statsBefore.InvoiceCount != 1 {
		t.Fatalf("expected invoice_count 1, got %d", statsBefore.InvoiceCount)
	}

	if err := svc.ProcessPending(ctx, 10); err != nil {
		t.Fatalf("process pending second pass: %v", err)
	}

	balanceAfter := loadCustomerBalance(t, dbConn, seed.OrgID, seed.CustomerID)
	statsAfter := loadCycleStats(t, dbConn, seed.BillingCycleID)

	if balanceBefore != balanceAfter {
		t.Fatalf("expected balance to remain %d, got %d", balanceBefore, balanceAfter)
	}
	if statsBefore.TotalRevenue != statsAfter.TotalRevenue {
		t.Fatalf("expected revenue to remain %d, got %d", statsBefore.TotalRevenue, statsAfter.TotalRevenue)
	}
	if statsBefore.InvoiceCount != statsAfter.InvoiceCount {
		t.Fatalf("expected invoice_count to remain %d, got %d", statsBefore.InvoiceCount, statsAfter.InvoiceCount)
	}
}

func TestRollupRebuildReplay(t *testing.T) {
	dbConn, node := setupRollupDB(t)
	seed := seedRollupData(t, dbConn, node)

	svc := NewService(Params{DB: dbConn, Log: zap.NewNop(), GenID: node})
	ctx := context.Background()

	if err := svc.ProcessPending(ctx, 10); err != nil {
		t.Fatalf("process pending: %v", err)
	}

	balanceBefore := loadCustomerBalance(t, dbConn, seed.OrgID, seed.CustomerID)
	statsBefore := loadCycleStats(t, dbConn, seed.BillingCycleID)
	if balanceBefore != 1000 {
		t.Fatalf("expected balance 1000, got %d", balanceBefore)
	}
	if statsBefore.TotalRevenue != 1000 {
		t.Fatalf("expected revenue 1000, got %d", statsBefore.TotalRevenue)
	}
	if statsBefore.InvoiceCount != 1 {
		t.Fatalf("expected invoice_count 1, got %d", statsBefore.InvoiceCount)
	}

	if err := svc.RebuildSnapshots(ctx, RebuildRequest{OrgID: &seed.OrgID}); err != nil {
		t.Fatalf("rebuild snapshots: %v", err)
	}

	balanceAfter := loadCustomerBalance(t, dbConn, seed.OrgID, seed.CustomerID)
	statsAfter := loadCycleStats(t, dbConn, seed.BillingCycleID)

	if balanceBefore != balanceAfter {
		t.Fatalf("expected rebuilt balance %d, got %d", balanceBefore, balanceAfter)
	}
	if statsBefore.TotalRevenue != statsAfter.TotalRevenue {
		t.Fatalf("expected rebuilt revenue %d, got %d", statsBefore.TotalRevenue, statsAfter.TotalRevenue)
	}
	if statsBefore.InvoiceCount != statsAfter.InvoiceCount {
		t.Fatalf("expected rebuilt invoice_count %d, got %d", statsBefore.InvoiceCount, statsAfter.InvoiceCount)
	}
}

type rollupSeed struct {
	OrgID          snowflake.ID
	CustomerID     snowflake.ID
	SubscriptionID snowflake.ID
	BillingCycleID snowflake.ID
	LedgerEntryID  snowflake.ID
	InvoiceID      snowflake.ID
}

func setupRollupDB(t *testing.T) (*gorm.DB, *snowflake.Node) {
	t.Helper()

	dbConn, err := db.NewTest()
	if err != nil {
		t.Fatalf("new test db: %v", err)
	}
	if err := dbConn.AutoMigrate(
		&billingevent.BillingEvent{},
		&ledgerdomain.LedgerAccount{},
		&ledgerdomain.LedgerEntry{},
		&ledgerdomain.LedgerEntryLine{},
		&customerdomain.Customer{},
		&subscriptiondomain.Subscription{},
		&domain.BillingCycle{},
		&invoicedomain.Invoice{},
		&paymentdomain.EventRecord{},
		&customerBalance{},
		&billingCycleStats{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	node, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatalf("new snowflake node: %v", err)
	}

	return dbConn, node
}

func seedRollupData(t *testing.T, dbConn *gorm.DB, node *snowflake.Node) rollupSeed {
	t.Helper()

	orgID := node.Generate()
	customerID := node.Generate()
	subscriptionID := node.Generate()
	cycleID := node.Generate()
	entryID := node.Generate()
	invoiceID := node.Generate()

	customer := customerdomain.Customer{
		ID:        customerID,
		OrgID:     orgID,
		Name:      "Acme Co",
		Email:     "billing@acme.test",
		Currency:  "USD",
		Metadata:  datatypes.JSONMap{},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := dbConn.Create(&customer).Error; err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	start := time.Now().UTC().Add(-24 * time.Hour)
	subscription := subscriptiondomain.Subscription{
		ID:               subscriptionID,
		OrgID:            orgID,
		CustomerID:       customerID,
		Status:           subscriptiondomain.SubscriptionStatusActive,
		CollectionMode:   subscriptiondomain.SendInvoice,
		StartAt:          start,
		BillingCycleType: "monthly",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if err := dbConn.Create(&subscription).Error; err != nil {
		t.Fatalf("insert subscription: %v", err)
	}

	periodStart := start
	periodEnd := start.AddDate(0, 1, 0)
	cycle := domain.BillingCycle{
		ID:             cycleID,
		OrgID:          orgID,
		SubscriptionID: subscriptionID,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		Status:         domain.BillingCycleStatusClosed,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := dbConn.Create(&cycle).Error; err != nil {
		t.Fatalf("insert billing cycle: %v", err)
	}

	accountsReceivable := ledgerdomain.LedgerAccount{
		ID:        node.Generate(),
		OrgID:     orgID,
		Code:      ledgerdomain.AccountCodeAccountsReceivable,
		Name:      "Accounts Receivable",
		CreatedAt: time.Now().UTC(),
	}
	if err := dbConn.Create(&accountsReceivable).Error; err != nil {
		t.Fatalf("insert ledger account: %v", err)
	}

	revenueAccount := ledgerdomain.LedgerAccount{
		ID:        node.Generate(),
		OrgID:     orgID,
		Code:      ledgerdomain.AccountCodeRevenueFlat,
		Name:      "Revenue",
		CreatedAt: time.Now().UTC(),
	}
	if err := dbConn.Create(&revenueAccount).Error; err != nil {
		t.Fatalf("insert revenue account: %v", err)
	}

	entry := ledgerdomain.LedgerEntry{
		ID:         entryID,
		OrgID:      orgID,
		SourceType: ledgerdomain.SourceTypeBillingCycle,
		SourceID:   cycleID,
		Currency:   "USD",
		OccurredAt: periodEnd,
		CreatedAt:  time.Now().UTC(),
	}
	if err := dbConn.Create(&entry).Error; err != nil {
		t.Fatalf("insert ledger entry: %v", err)
	}

	lines := []ledgerdomain.LedgerEntryLine{
		{
			ID:            node.Generate(),
			LedgerEntryID: entryID,
			AccountID:     accountsReceivable.ID,
			Direction:     ledgerdomain.LedgerEntryDirectionDebit,
			Amount:        1000,
			CreatedAt:     time.Now().UTC(),
		},
		{
			ID:            node.Generate(),
			LedgerEntryID: entryID,
			AccountID:     revenueAccount.ID,
			Direction:     ledgerdomain.LedgerEntryDirectionCredit,
			Amount:        1000,
			CreatedAt:     time.Now().UTC(),
		},
	}
	if err := dbConn.Create(&lines).Error; err != nil {
		t.Fatalf("insert ledger lines: %v", err)
	}

	invoice := invoicedomain.Invoice{
		ID:             invoiceID,
		OrgID:          orgID,
		BillingCycleID: cycleID,
		SubscriptionID: subscriptionID,
		CustomerID:     customerID,
		Status:         invoicedomain.InvoiceStatusFinalized,
		SubtotalAmount: 1000,
		Currency:       "USD",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := dbConn.Create(&invoice).Error; err != nil {
		t.Fatalf("insert invoice: %v", err)
	}

	ledgerEvent := billingevent.BillingEvent{
		ID:        node.Generate(),
		OrgID:     orgID,
		EventType: events.EventLedgerEntryCreated,
		Payload: datatypes.JSONMap{
			"ledger_entry_id": entryID.String(),
		},
		Published: false,
		CreatedAt: time.Now().UTC(),
	}
	if err := dbConn.Create(&ledgerEvent).Error; err != nil {
		t.Fatalf("insert ledger event: %v", err)
	}

	invoiceEvent := billingevent.BillingEvent{
		ID:        node.Generate(),
		OrgID:     orgID,
		EventType: events.EventInvoiceFinalized,
		Payload: datatypes.JSONMap{
			"invoice_id":       invoiceID.String(),
			"billing_cycle_id": cycleID.String(),
		},
		Published: false,
		CreatedAt: time.Now().UTC(),
	}
	if err := dbConn.Create(&invoiceEvent).Error; err != nil {
		t.Fatalf("insert invoice event: %v", err)
	}

	return rollupSeed{
		OrgID:          orgID,
		CustomerID:     customerID,
		SubscriptionID: subscriptionID,
		BillingCycleID: cycleID,
		LedgerEntryID:  entryID,
		InvoiceID:      invoiceID,
	}
}

func loadCustomerBalance(t *testing.T, dbConn *gorm.DB, orgID, customerID snowflake.ID) int64 {
	t.Helper()

	var row customerBalance
	if err := dbConn.Where("org_id = ? AND customer_id = ?", orgID, customerID).First(&row).Error; err != nil {
		t.Fatalf("load customer balance: %v", err)
	}
	return row.Balance
}

func loadCycleStats(t *testing.T, dbConn *gorm.DB, cycleID snowflake.ID) billingCycleStats {
	t.Helper()

	var row billingCycleStats
	if err := dbConn.Where("billing_cycle_id = ?", cycleID).First(&row).Error; err != nil {
		t.Fatalf("load cycle stats: %v", err)
	}
	return row
}
