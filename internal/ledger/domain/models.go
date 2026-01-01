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

const (
	SourceTypeBillingCycle = "billing_cycle"
	SourceTypeAdjustment   = "adjustment"
	SourceTypeRefund       = "refund"
	SourceTypePaymentEvent = "payment_event"
	SourceTypeDisputeWithdrawn  = "dispute_withdrawn"
	SourceTypeDisputeReinstated = "dispute_reinstated"
)

const (
	AccountCodeAccountsReceivable = "accounts_receivable"
	AccountCodeRevenue            = "revenue"
	AccountCodeTaxPayable         = "tax_payable"
	AccountCodeCredit             = "credit"
	AccountCodeCashClearing       = "cash_clearing"
)

// LedgerAccount defines a chart-of-accounts entry.
type LedgerAccount struct {
	ID        snowflake.ID `gorm:"primaryKey"`
	OrgID     snowflake.ID `gorm:"not null;index;uniqueIndex:ux_ledger_accounts_org_code,priority:1"`
	Code      string       `gorm:"type:text;not null;uniqueIndex:ux_ledger_accounts_org_code,priority:2"`
	Name      string       `gorm:"type:text;not null"`
	CreatedAt time.Time    `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TableName sets the database table name.
func (LedgerAccount) TableName() string { return "ledger_accounts" }

// LedgerEntry captures the immutable header for a financial event.
type LedgerEntry struct {
	ID         snowflake.ID `gorm:"primaryKey"`
	OrgID      snowflake.ID `gorm:"not null;index"`
	SourceType string       `gorm:"type:text;not null;index"`
	SourceID   snowflake.ID `gorm:"not null;index"`
	Currency   string       `gorm:"type:text;not null"`
	OccurredAt time.Time    `gorm:"not null"`
	CreatedAt  time.Time    `gorm:"not null;default:CURRENT_TIMESTAMP"`
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
