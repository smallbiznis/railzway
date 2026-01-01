package repository

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/payment/domain"
	"gorm.io/gorm"
)

type repo struct{}

func Provide() domain.Repository {
	return &repo{}
}

func (r *repo) FindEvent(ctx context.Context, db *gorm.DB, provider string, providerEventID string) (*domain.EventRecord, error) {
	var item domain.EventRecord
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, provider, provider_event_id, event_type, customer_id,
			payload, received_at, processed_at
		 FROM payment_events
		 WHERE provider = ? AND provider_event_id = ?
		 LIMIT 1`,
		provider,
		providerEventID,
	).Scan(&item).Error
	if err != nil {
		return nil, err
	}
	if item.ID == 0 {
		return nil, nil
	}
	return &item, nil
}

func (r *repo) InsertEvent(ctx context.Context, db *gorm.DB, event *domain.EventRecord) (bool, error) {
	res := db.WithContext(ctx).Exec(
		`INSERT INTO payment_events (
			id, org_id, provider, provider_event_id, event_type, customer_id,
			payload, received_at, processed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (provider, provider_event_id) DO NOTHING`,
		event.ID,
		event.OrgID,
		event.Provider,
		event.ProviderEventID,
		event.EventType,
		event.CustomerID,
		event.Payload,
		event.ReceivedAt,
		event.ProcessedAt,
	)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *repo) MarkProcessed(ctx context.Context, db *gorm.DB, id snowflake.ID, processedAt time.Time) error {
	return db.WithContext(ctx).Exec(
		`UPDATE payment_events
		 SET processed_at = ?
		 WHERE id = ?`,
		processedAt,
		id,
	).Error
}
