package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/datatypes"
)

// BillingCycleStatus represents rating/invoicing progress for a cycle.
type BillingCycleStatus string

const (
	BillingCycleStatusOpen    BillingCycleStatus = "OPEN"
	BillingCycleStatusClosing BillingCycleStatus = "CLOSING"
	BillingCycleStatusClosed  BillingCycleStatus = "CLOSED"
)

// BillingCycle represents a billing period for a subscription.
type BillingCycle struct {
	ID                 snowflake.ID       `gorm:"primaryKey"`
	OrgID              snowflake.ID       `gorm:"not null;index"`
	SubscriptionID     snowflake.ID       `gorm:"not null;index;uniqueIndex:ux_billing_cycle_period,priority:1"`
	PeriodStart        time.Time          `gorm:"not null;uniqueIndex:ux_billing_cycle_period,priority:2"`
	PeriodEnd          time.Time          `gorm:"not null;uniqueIndex:ux_billing_cycle_period,priority:3"`
	Status             BillingCycleStatus `gorm:"type:text;not null;default:'OPEN'"`
	OpenedAt           *time.Time         `gorm:"column:opened_at"`
	ClosingStartedAt   *time.Time         `gorm:"column:closing_started_at"`
	RatingCompletedAt  *time.Time         `gorm:"column:rating_completed_at"`
	InvoicedAt         *time.Time         `gorm:"column:invoiced_at"`
	InvoiceFinalizedAt *time.Time         `gorm:"column:invoice_finalized_at"`
	ClosedAt           *time.Time         `gorm:"column:closed_at"`
	LastError          *string            `gorm:"column:last_error;type:text"`
	LastErrorAt        *time.Time         `gorm:"column:last_error_at"`
	Metadata           datatypes.JSONMap  `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt          time.Time          `gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt          time.Time          `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TableName sets the database table name.
func (BillingCycle) TableName() string { return "billing_cycles" }
