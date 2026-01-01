package repository

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	disputedomain "github.com/smallbiznis/valora/internal/payment/dispute/domain"
	"gorm.io/gorm"
)

type repo struct{}

func Provide() disputedomain.Repository {
	return &repo{}
}

func (r *repo) FindDispute(ctx context.Context, db *gorm.DB, provider string, providerDisputeID string) (*disputedomain.DisputeRecord, error) {
	return findDispute(ctx, db, provider, providerDisputeID, false)
}

func (r *repo) FindDisputeForUpdate(ctx context.Context, db *gorm.DB, provider string, providerDisputeID string) (*disputedomain.DisputeRecord, error) {
	return findDispute(ctx, db, provider, providerDisputeID, true)
}

func (r *repo) InsertDispute(ctx context.Context, db *gorm.DB, record *disputedomain.DisputeRecord) (bool, error) {
	res := db.WithContext(ctx).Exec(
		`INSERT INTO payment_disputes (
			id, org_id, provider, provider_dispute_id, provider_event_id, customer_id,
			amount, currency, status, reason, received_at, processed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (provider, provider_dispute_id) DO NOTHING`,
		record.ID,
		record.OrgID,
		record.Provider,
		record.ProviderDisputeID,
		record.ProviderEventID,
		record.CustomerID,
		record.Amount,
		record.Currency,
		record.Status,
		record.Reason,
		record.ReceivedAt,
		record.ProcessedAt,
	)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *repo) UpdateDispute(ctx context.Context, db *gorm.DB, record *disputedomain.DisputeRecord) error {
	return db.WithContext(ctx).Exec(
		`UPDATE payment_disputes
		 SET provider_event_id = ?, customer_id = ?, amount = ?, currency = ?, status = ?, reason = ?,
		     received_at = ?, processed_at = ?
		 WHERE id = ?`,
		record.ProviderEventID,
		record.CustomerID,
		record.Amount,
		record.Currency,
		record.Status,
		record.Reason,
		record.ReceivedAt,
		record.ProcessedAt,
		record.ID,
	).Error
}

func (r *repo) MarkProcessed(ctx context.Context, db *gorm.DB, id snowflake.ID, processedAt time.Time) error {
	return db.WithContext(ctx).Exec(
		`UPDATE payment_disputes
		 SET processed_at = ?
		 WHERE id = ?`,
		processedAt,
		id,
	).Error
}

func findDispute(ctx context.Context, db *gorm.DB, provider string, providerDisputeID string, forUpdate bool) (*disputedomain.DisputeRecord, error) {
	var record disputedomain.DisputeRecord
	query := `SELECT id, org_id, provider, provider_dispute_id, provider_event_id, customer_id,
		amount, currency, status, reason, received_at, processed_at
	 FROM payment_disputes
	 WHERE provider = ? AND provider_dispute_id = ?
	 LIMIT 1`
	if forUpdate && db.Dialector.Name() != "sqlite" {
		query += " FOR UPDATE"
	}
	err := db.WithContext(ctx).Raw(query, provider, providerDisputeID).Scan(&record).Error
	if err != nil {
		return nil, err
	}
	if record.ID == 0 {
		return nil, nil
	}
	return &record, nil
}
