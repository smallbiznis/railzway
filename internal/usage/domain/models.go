// Package domain contains persistence models for raw usage ingestion.
package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/datatypes"
)

const (
	UsageStatusAccepted              = "accepted"
	UsageStatusInvalid               = "invalid"
	UsageStatusEnriched              = "enriched"
	UsageStatusRated                 = "rated"
	UsageStatusUnmatchedMeter        = "unmatched_meter"
	UsageStatusUnmatchedSubscription = "unmatched_subscription"
)

// UsageEvent stores a single unit of metered activity.
type UsageEvent struct {
	ID         snowflake.ID `gorm:"primaryKey" json:"id"`
	OrgID      snowflake.ID `gorm:"not null" json:"org_id"`
	CustomerID snowflake.ID `gorm:"not null" json:"customer_id"`

	// Snapshot of subscription/meter at time of ingestion
	SubscriptionID snowflake.ID `gorm:"not null" json:"-"`

	// Snapshot of subscription item at time of ingestion
	SubscriptionItemID snowflake.ID `gorm:"" json:"-"`

	// Snapshot of meter at time of ingestion
	MeterID snowflake.ID `gorm:"not null" json:"-"`

	MeterCode      string            `gorm:"type:text;not null" json:"meter_code"`
	Value          float64           `gorm:"not null" json:"value"`
	RecordedAt     time.Time         `gorm:"not null" json:"recorded_at"`
	Status         string            `gorm:"type:text;not null;default:accepted" json:"-"`
	Error          *string           `gorm:"type:text" json:"-"`
	IdempotencyKey string            `gorm:"type:text" json:"idempotency_key"`
	Metadata       datatypes.JSONMap `gorm:"type:jsonb" json:"metadata"`
	SnapshotAt     *time.Time        `gorm:"" json:"-"`
	CreatedAt      time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP" json:"-"`
	UpdatedAt      time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP" json:"-"`
}

// TableName sets the database table name.
func (UsageEvent) TableName() string { return "usage_events" }
