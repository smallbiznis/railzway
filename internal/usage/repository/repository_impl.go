package repository

import (
	"context"

	usagedomain "github.com/smallbiznis/valora/internal/usage/domain"
	"gorm.io/gorm"
)

type snapshotRepo struct{}

func ProvideSnapshot() usagedomain.SnapshotRepository {
	return &snapshotRepo{}
}

func (r *snapshotRepo) LockAccepted(ctx context.Context, db *gorm.DB, limit int) ([]usagedomain.SnapshotCandidate, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []usagedomain.SnapshotCandidate
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, customer_id, meter_code, recorded_at
		 FROM usage_events
		 WHERE status = ?
		 ORDER BY recorded_at ASC
		 LIMIT ?
		 FOR UPDATE SKIP LOCKED`,
		usagedomain.UsageStatusAccepted,
		limit,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *snapshotRepo) UpdateSnapshot(ctx context.Context, db *gorm.DB, update usagedomain.SnapshotUpdate) error {
	var subscriptionItem any
	if update.SubscriptionItemID != nil {
		subscriptionItem = *update.SubscriptionItemID
	}
	return db.WithContext(ctx).Exec(
		`UPDATE usage_events
		 SET meter_id = ?,
		     subscription_id = ?,
		     subscription_item_id = ?,
		     snapshot_at = ?,
		     status = ?,
		     updated_at = ?
		 WHERE id = ? AND status = ?`,
		update.MeterID,
		update.SubscriptionID,
		subscriptionItem,
		update.SnapshotAt,
		update.Status,
		update.SnapshotAt,
		update.ID,
		usagedomain.UsageStatusAccepted,
	).Error
}
