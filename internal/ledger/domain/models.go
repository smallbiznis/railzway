package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
)

// LedgerEntryDirection represents debit or credit postings.
type LedgerEntryDirection string

const (
	LedgerEntryDirectionDebit  LedgerEntryDirection = "debit"
	LedgerEntryDirectionCredit LedgerEntryDirection = "credit"
)

type LedgerSourceType string

const (
	// ======================
	// Billing & Revenue
	// ======================
	SourceTypeBillingCycle LedgerSourceType = "billing_cycle" // invoice charge (usage / flat)
	SourceTypeAdjustment   LedgerSourceType = "adjustment"    // late usage / correction

	// ======================
	// Payments
	// ======================
	SourceTypePayment    LedgerSourceType = "payment"     // successful customer payment
	SourceTypePaymentFee LedgerSourceType = "payment_fee" // gateway / processor fee

	// ======================
	// Credits & Refunds
	// ======================
	SourceTypeCreditGrant LedgerSourceType = "credit_grant" // promo / goodwill credit
	SourceTypeCreditUse   LedgerSourceType = "credit_use"   // credit applied to invoice
	SourceTypeRefund      LedgerSourceType = "refund"       // money returned to customer

	// ======================
	// Disputes (economic impact only)
	// ======================
	SourceTypeDisputeHold LedgerSourceType = "dispute_hold" // revenue temporarily reversed
	SourceTypeDisputeLoss LedgerSourceType = "dispute_loss" // dispute lost (final)
	SourceTypeDisputeWin  LedgerSourceType = "dispute_win"  // dispute won (reinstated)
)

type LedgerAccountCode string

const (
	// Assets
	AccountCodeAccountsReceivable LedgerAccountCode = "accounts_receivable"
	AccountCodeCash               LedgerAccountCode = "cash"

	// Revenue
	AccountCodeRevenueUsage LedgerAccountCode = "revenue_usage"
	AccountCodeRevenueFlat  LedgerAccountCode = "revenue_flat"

	// Liabilities
	AccountCodeTaxPayable    LedgerAccountCode = "tax_payable"
	AccountCodeCreditBalance LedgerAccountCode = "credit_balance"
	AccountCodeRefundLiab    LedgerAccountCode = "refund_liability"

	// Expenses
	AccountCodePaymentFeeExpense LedgerAccountCode = "payment_fee_expense"

	// Equity / Adjustment
	AccountCodeAdjustment LedgerAccountCode = "adjustment"
)

// LedgerAccount defines a chart-of-accounts entry.
type LedgerAccount struct {
	ID        snowflake.ID      `gorm:"primaryKey"`
	OrgID     snowflake.ID      `gorm:"not null;index;uniqueIndex:ux_ledger_accounts_org_code,priority:1"`
	Code      LedgerAccountCode `gorm:"type:text;not null;uniqueIndex:ux_ledger_accounts_org_code,priority:2"`
	Name      string            `gorm:"type:text;not null"`
	CreatedAt time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TableName sets the database table name.
func (LedgerAccount) TableName() string { return "ledger_accounts" }

// LedgerEntry captures the immutable header for a financial event.
type LedgerEntry struct {
	ID         snowflake.ID     `gorm:"primaryKey"`
	OrgID      snowflake.ID     `gorm:"not null;index"`
	SourceType LedgerSourceType `gorm:"type:text;not null;index"`
	SourceID   snowflake.ID     `gorm:"not null;index"`
	Currency   string           `gorm:"type:text;not null"`
	OccurredAt time.Time        `gorm:"not null"`
	CreatedAt  time.Time        `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TableName sets the database table name.
func (LedgerEntry) TableName() string { return "ledger_entries" }

// LedgerEntryLine is a double-entry posting line.
type LedgerEntryLine struct {
	ID            snowflake.ID         `gorm:"primaryKey"`
	LedgerEntryID snowflake.ID         `gorm:"not null;index"`
	AccountID     snowflake.ID         `gorm:"not null;index"`
	Direction     LedgerEntryDirection `gorm:"type:text;not null"`
	Amount        int64                `gorm:"not null"`
	CreatedAt     time.Time            `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TableName sets the database table name.
func (LedgerEntryLine) TableName() string { return "ledger_entry_lines" }
