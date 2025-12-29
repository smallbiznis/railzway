// Package domain contains persistence models for raw usage ingestion.
package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/datatypes"
)

// UsageEvent stores a single unit of metered activity.
type UsageEvent struct {
	ID                 snowflake.ID      `gorm:"primaryKey"`
	OrgID              snowflake.ID      `gorm:"not null"`
	CustomerID         snowflake.ID      `gorm:"not null"`
	SubscriptionID     snowflake.ID      `gorm:"not null"`
	SubscriptionItemID snowflake.ID      `gorm:"not null"`
	MeterID            snowflake.ID      `gorm:"not null"`
	MeterCode          string            `gorm:"type:text;not null"` // snapshot
	Value              float64           `gorm:"not null"`
	RecordedAt         time.Time         `gorm:"not null"`
	IdempotencyKey     *string           `gorm:"type:text"`
	Metadata           datatypes.JSONMap `gorm:"type:jsonb"`
	CreatedAt          time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt          time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TableName sets the database table name.
func (UsageEvent) TableName() string { return "usage_events" }
