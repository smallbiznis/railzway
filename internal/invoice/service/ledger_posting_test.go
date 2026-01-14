package service

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/glebarez/sqlite"
	invoicedomain "github.com/smallbiznis/railzway/internal/invoice/domain"
	"github.com/smallbiznis/railzway/internal/invoice/render"
	ledgerdomain "github.com/smallbiznis/railzway/internal/ledger/domain"
	publicinvoicedomain "github.com/smallbiznis/railzway/internal/publicinvoice/domain"
	taxdomain "github.com/smallbiznis/railzway/internal/tax/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Mock objects
type mockTaxResolver struct {
	mock.Mock
}

func (m *mockTaxResolver) ResolveForInvoice(ctx context.Context, orgID, customerID snowflake.ID) (*taxdomain.TaxDefinition, error) {
	args := m.Mock(ctx, orgID, customerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*taxdomain.TaxDefinition), args.Error(1)
}

type mockRenderer struct {
	mock.Mock
}

func (m *mockRenderer) RenderHTML(input render.RenderInput) (string, error) {
	args := m.Mock(input)
	return args.String(0), args.Error(1)
}

type mockPublicTokenSvc struct {
	mock.Mock
}

func (m *mockPublicTokenSvc) EnsureForInvoice(ctx context.Context, invoice invoicedomain.Invoice) (publicinvoicedomain.PublicInvoiceToken, error) {
	args := m.Mock(ctx, invoice)
	return args.Get(0).(publicinvoicedomain.PublicInvoiceToken), args.Error(1)
}

type mockLedgerSvc struct {
	mock.Mock
}

func (m *mockLedgerSvc) CreateEntry(ctx context.Context, orgID snowflake.ID, sourceType string, sourceID snowflake.ID, currency string, occurredAt time.Time, lines []ledgerdomain.LedgerEntryLine) error {
	args := m.Mock(ctx, orgID, sourceType, sourceID, currency, occurredAt, lines)
	return args.Error(0)
}

func TestPostInvoiceToLedger_CorrectPostings(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})

	// Migrate tables
	db.AutoMigrate(
		&invoicedomain.Invoice{},
		&ledgerdomain.LedgerEntry{},
		&ledgerdomain.LedgerEntryLine{},
		&ledgerdomain.LedgerAccount{},
	)

	// SQLite requires specific UNIQUE indexes for ON CONFLICT to work
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS ux_ledger_entries_source ON ledger_entries(org_id, source_type, source_id)")
	// Drop the problematic unique index on type if gorm created it
	db.Exec("DROP INDEX IF EXISTS ux_ledger_accounts_org_type")

	node, _ := snowflake.NewNode(1)
	logger := zap.NewNop()

	svcInterface := NewService(ServiceParam{
		DB:    db,
		Log:   logger,
		GenID: node,
	})
	svc := svcInterface.(*Service)

	orgID := node.Generate()
	invoiceID := node.Generate()
	arAccountID := node.Generate()
	revAccountID := node.Generate()
	taxAccountID := node.Generate()

	// Seed ledger accounts
	assert.NoError(t, db.Create(&ledgerdomain.LedgerAccount{ID: arAccountID, OrgID: orgID, Code: ledgerdomain.AccountCodeAccountsReceivable, Name: "AR", Type: ledgerdomain.Assets}).Error)
	assert.NoError(t, db.Create(&ledgerdomain.LedgerAccount{ID: revAccountID, OrgID: orgID, Code: ledgerdomain.AccountCodeRevenueUsage, Name: "Revenue", Type: ledgerdomain.Income}).Error)
	assert.NoError(t, db.Create(&ledgerdomain.LedgerAccount{ID: taxAccountID, OrgID: orgID, Code: ledgerdomain.AccountCodeTaxPayable, Name: "Tax", Type: ledgerdomain.Liability}).Error)

	now := time.Now().UTC()

	// Case 1: Invoice with Tax
	invoiceWithTax := &invoicedomain.Invoice{
		ID:             invoiceID,
		OrgID:          orgID,
		Status:         invoicedomain.InvoiceStatusFinalized,
		SubtotalAmount: 10000,
		TaxAmount:      2000,
		TotalAmount:    12000,
		Currency:       "USD",
		FinalizedAt:    &now,
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		return svc.postInvoiceToLedger(context.Background(), tx, invoiceWithTax)
	})
	assert.NoError(t, err)

	// Verify
	var entry ledgerdomain.LedgerEntry
	db.First(&entry, "source_id = ?", invoiceID)

	var lines []ledgerdomain.LedgerEntryLine
	db.Find(&lines, "ledger_entry_id = ?", entry.ID)
	assert.Len(t, lines, 3)

	amounts := make(map[snowflake.ID]int64)
	for _, l := range lines {
		amounts[l.AccountID] = l.Amount
	}

	assert.Equal(t, int64(12000), amounts[arAccountID])
	assert.Equal(t, int64(10000), amounts[revAccountID])
	assert.Equal(t, int64(2000), amounts[taxAccountID])

	// Case 2: Invoice without Tax
	invoiceID2 := node.Generate()
	invoiceNoTax := &invoicedomain.Invoice{
		ID:             invoiceID2,
		OrgID:          orgID,
		Status:         invoicedomain.InvoiceStatusFinalized,
		SubtotalAmount: 5000,
		TaxAmount:      0,
		TotalAmount:    5000,
		Currency:       "USD",
		FinalizedAt:    &now,
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		return svc.postInvoiceToLedger(context.Background(), tx, invoiceNoTax)
	})
	assert.NoError(t, err)

	entry = ledgerdomain.LedgerEntry{} // Reset
	db.First(&entry, "source_id = ?", invoiceID2)
	lines = []ledgerdomain.LedgerEntryLine{} // Reset
	db.Find(&lines, "ledger_entry_id = ?", entry.ID)
	assert.Len(t, lines, 2)
}

func TestFinalizeInvoice_Idempotency(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	db.AutoMigrate(&invoicedomain.Invoice{}, &ledgerdomain.LedgerEntry{}, &ledgerdomain.LedgerEntryLine{}, &ledgerdomain.LedgerAccount{})
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS ux_ledger_entries_source ON ledger_entries(org_id, source_type, source_id)")
	db.Exec("DROP INDEX IF EXISTS ux_ledger_accounts_org_type")

	node, _ := snowflake.NewNode(1)
	svcInterface := NewService(ServiceParam{
		DB:             db,
		Log:            zap.NewNop(),
		GenID:          node,
		TaxResolver:    new(mockTaxResolver),
		Renderer:       new(mockRenderer),
		PublicTokenSvc: new(mockPublicTokenSvc),
		LedgerSvc:      new(mockLedgerSvc),
	})
	svc := svcInterface.(*Service)

	orgID := node.Generate()
	invoiceID := node.Generate()

	// Seed ledger accounts
	assert.NoError(t, db.Create(&ledgerdomain.LedgerAccount{ID: node.Generate(), OrgID: orgID, Code: ledgerdomain.AccountCodeAccountsReceivable, Name: "AR", Type: ledgerdomain.Assets}).Error)
	assert.NoError(t, db.Create(&ledgerdomain.LedgerAccount{ID: node.Generate(), OrgID: orgID, Code: ledgerdomain.AccountCodeRevenueUsage, Name: "Revenue", Type: ledgerdomain.Income}).Error)

	invoice := &invoicedomain.Invoice{
		ID:             invoiceID,
		OrgID:          orgID,
		Status:         invoicedomain.InvoiceStatusFinalized, // Already finalized
		SubtotalAmount: 10000,
		TotalAmount:    10000, // CRITICAL: Must be equal to Subtotal + Tax
		Currency:       "USD",
		FinalizedAt:    timePtr(time.Now()),
	}
	db.Create(invoice)

	// Call postInvoiceToLedger directly (internal method)
	// Since it's internal we test it via a helper or by export in _test.go if needed.
	// But we can just test if calling it again creates duplicates.

	err := db.Transaction(func(tx *gorm.DB) error {
		return svc.postInvoiceToLedger(context.Background(), tx, invoice)
	})
	assert.NoError(t, err)

	// Call again
	err = db.Transaction(func(tx *gorm.DB) error {
		return svc.postInvoiceToLedger(context.Background(), tx, invoice)
	})
	assert.NoError(t, err)

	// Verify only ONE entry
	var count int64
	db.Model(&ledgerdomain.LedgerEntry{}).Where("source_id = ?", invoiceID).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestFinalizeInvoice_Idempotency_NoOp(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	db.AutoMigrate(&invoicedomain.Invoice{}, &ledgerdomain.LedgerEntry{}, &ledgerdomain.LedgerAccount{})

	node, _ := snowflake.NewNode(1)
	svcInterface := NewService(ServiceParam{
		DB:    db,
		Log:   zap.NewNop(),
		GenID: node,
		// Mocks shouldn't be called if No-Op works
		TaxResolver:    new(mockTaxResolver),
		Renderer:       new(mockRenderer),
		PublicTokenSvc: new(mockPublicTokenSvc),
		LedgerSvc:      new(mockLedgerSvc),
	})
	svc := svcInterface.(*Service)

	orgID := node.Generate()
	invoiceID := node.Generate()

	// Create an already FINALIZED invoice
	invoice := &invoicedomain.Invoice{
		ID:             invoiceID,
		OrgID:          orgID,
		Status:         invoicedomain.InvoiceStatusFinalized,
		SubtotalAmount: 10000,
		TotalAmount:    10000,
		Currency:       "USD",
		FinalizedAt:    timePtr(time.Now()),
	}
	db.Create(invoice)

	// Call FinalizeInvoice
	// Expectation: Return nil (Success), NO side effects.
	err := svc.FinalizeInvoice(context.Background(), invoiceID.String())
	assert.NoError(t, err)

	// Verify status didn't change (still Finalized)
	var reloaded invoicedomain.Invoice
	db.First(&reloaded, "id = ?", invoiceID)
	assert.Equal(t, invoicedomain.InvoiceStatusFinalized, reloaded.Status)
}

func float64Ptr(f float64) *float64  { return &f }
func timePtr(t time.Time) *time.Time { return &t }
