package domain

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/gorm"
)

type AuditCursor struct {
	ID        snowflake.ID
	CreatedAt time.Time
}

type ListFilter struct {
	OrgID      snowflake.ID
	Action     string
	TargetType string
	TargetID   string
	ActorType  string
	StartAt    *time.Time
	EndAt      *time.Time
	Cursor     *AuditCursor
	Limit      int
}

type Repository interface {
	Insert(ctx context.Context, db *gorm.DB, entry *AuditLog) error
	List(ctx context.Context, db *gorm.DB, filter ListFilter) ([]*AuditLog, error)
}
