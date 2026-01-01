package repository

import (
	"context"
	"time"

	"github.com/smallbiznis/valora/internal/paymentprovider/domain"
	"gorm.io/gorm"
)

type repo struct{}

func Provide() domain.Repository {
	return &repo{}
}

func (r *repo) ListCatalog(ctx context.Context, db *gorm.DB) ([]domain.CatalogProvider, error) {
	var providers []domain.CatalogProvider
	err := db.WithContext(ctx).Raw(
		`SELECT provider, display_name, description, supports_webhook, supports_refund, created_at
		 FROM payment_provider_catalog
		 ORDER BY display_name`,
	).Scan(&providers).Error
	if err != nil {
		return nil, err
	}
	return providers, nil
}

func (r *repo) FindCatalog(ctx context.Context, db *gorm.DB, provider string) (*domain.CatalogProvider, error) {
	var item domain.CatalogProvider
	err := db.WithContext(ctx).Raw(
		`SELECT provider, display_name, description, supports_webhook, supports_refund, created_at
		 FROM payment_provider_catalog
		 WHERE provider = ?
		 LIMIT 1`,
		provider,
	).Scan(&item).Error
	if err != nil {
		return nil, err
	}
	if item.Provider == "" {
		return nil, nil
	}
	return &item, nil
}

func (r *repo) ListConfigs(ctx context.Context, db *gorm.DB, orgID int64) ([]domain.ProviderConfig, error) {
	var configs []domain.ProviderConfig
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, provider, config, is_active, created_at, updated_at
		 FROM payment_provider_configs
		 WHERE org_id = ?
		 ORDER BY created_at DESC`,
		orgID,
	).Scan(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

func (r *repo) FindConfig(ctx context.Context, db *gorm.DB, orgID int64, provider string) (*domain.ProviderConfig, error) {
	var item domain.ProviderConfig
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, provider, config, is_active, created_at, updated_at
		 FROM payment_provider_configs
		 WHERE org_id = ? AND provider = ?
		 LIMIT 1`,
		orgID,
		provider,
	).Scan(&item).Error
	if err != nil {
		return nil, err
	}
	if item.ID == 0 {
		return nil, nil
	}
	return &item, nil
}

func (r *repo) UpsertConfig(ctx context.Context, db *gorm.DB, config *domain.ProviderConfig) error {
	return db.WithContext(ctx).Exec(
		`INSERT INTO payment_provider_configs (
			id, org_id, provider, config, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (org_id, provider)
		DO UPDATE SET config = EXCLUDED.config,
			is_active = EXCLUDED.is_active,
			updated_at = EXCLUDED.updated_at`,
		config.ID,
		config.OrgID,
		config.Provider,
		config.Config,
		config.IsActive,
		config.CreatedAt,
		config.UpdatedAt,
	).Error
}

func (r *repo) UpdateStatus(ctx context.Context, db *gorm.DB, orgID int64, provider string, isActive bool, updatedAt time.Time) (bool, error) {
	res := db.WithContext(ctx).Exec(
		`UPDATE payment_provider_configs
		 SET is_active = ?, updated_at = ?
		 WHERE org_id = ? AND provider = ?`,
		isActive,
		updatedAt,
		orgID,
		provider,
	)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
