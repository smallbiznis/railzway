package domain

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/gorm"
)

type Repository interface {
	FindDispute(ctx context.Context, db *gorm.DB, provider string, providerDisputeID string) (*DisputeRecord, error)
	FindDisputeForUpdate(ctx context.Context, db *gorm.DB, provider string, providerDisputeID string) (*DisputeRecord, error)
	InsertDispute(ctx context.Context, db *gorm.DB, record *DisputeRecord) (bool, error)
	UpdateDispute(ctx context.Context, db *gorm.DB, record *DisputeRecord) error
	MarkProcessed(ctx context.Context, db *gorm.DB, id snowflake.ID, processedAt time.Time) error
}
