package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
)

// APIKey stores hashed API credentials scoped to an organization.
type APIKey struct {
	ID               snowflake.ID `gorm:"primaryKey"`
	OrgID            snowflake.ID `gorm:"column:org_id;not null;uniqueIndex:ux_api_keys_org_key_id,priority:1"`
	KeyID            string       `gorm:"column:key_id;type:text;not null;uniqueIndex:ux_api_keys_org_key_id,priority:2"`
	Name             string       `gorm:"type:text;not null"`
	Scope            string       `gorm:"type:text;not null"`
	KeyHash          string       `gorm:"column:key_hash;type:text;not null"`
	IsActive         bool         `gorm:"column:is_active;not null;default:true"`
	CreatedAt        time.Time    `gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt        time.Time    `gorm:"not null;default:CURRENT_TIMESTAMP"`
	LastUsedAt       *time.Time   `gorm:"column:last_used_at"`
	ExpiresAt        *time.Time   `gorm:"column:expires_at"`
	RotatedFromKeyID *string      `gorm:"column:rotated_from_key_id;type:text"`
}

// TableName sets the database table name.
func (APIKey) TableName() string { return "api_keys" }
