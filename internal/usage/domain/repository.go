package domain

import (
	"context"

	"gorm.io/gorm"
)

// SnapshotRepository provides locking and update operations for usage snapshot enrichment.
type SnapshotRepository interface {
	LockAccepted(ctx context.Context, db *gorm.DB, limit int) ([]SnapshotCandidate, error)
	UpdateSnapshot(ctx context.Context, db *gorm.DB, update SnapshotUpdate) error
}
