package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/datatypes"
)

// ActorType represents who triggered an action.
type ActorType string

const (
	ActorTypeUser   ActorType = "user"
	ActorTypeSystem ActorType = "system"
	ActorTypeAPIKey ActorType = "api_key"
)

// AuditLog captures an immutable record of a security or billing action.
type AuditLog struct {
	ID         snowflake.ID      `gorm:"primaryKey"`
	OrgID      *snowflake.ID     `gorm:"index"`
	ActorType  string            `gorm:"type:text;not null"`
	ActorID    *string           `gorm:"type:text"`
	Action     string            `gorm:"type:text;not null;index"`
	TargetType string            `gorm:"type:text;not null"`
	TargetID   *string           `gorm:"type:text"`
	Metadata   datatypes.JSONMap `gorm:"type:jsonb;not null;default:'{}'"`
	IPAddress  *string           `gorm:"type:text"`
	UserAgent  *string           `gorm:"type:text"`
	CreatedAt  time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TableName sets the database table name.
func (AuditLog) TableName() string { return "audit_logs" }
