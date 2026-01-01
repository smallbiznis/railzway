package domain

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/gorm"
)

type Repository interface {
	FindEvent(ctx context.Context, db *gorm.DB, provider string, providerEventID string) (*EventRecord, error)
	InsertEvent(ctx context.Context, db *gorm.DB, event *EventRecord) (bool, error)
	MarkProcessed(ctx context.Context, db *gorm.DB, id snowflake.ID, processedAt time.Time) error
}
