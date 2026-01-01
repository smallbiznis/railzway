package domain

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository interface {
	ListCatalog(ctx context.Context, db *gorm.DB) ([]CatalogProvider, error)
	FindCatalog(ctx context.Context, db *gorm.DB, provider string) (*CatalogProvider, error)
	ListConfigs(ctx context.Context, db *gorm.DB, orgID int64) ([]ProviderConfig, error)
	FindConfig(ctx context.Context, db *gorm.DB, orgID int64, provider string) (*ProviderConfig, error)
	UpsertConfig(ctx context.Context, db *gorm.DB, config *ProviderConfig) error
	UpdateStatus(ctx context.Context, db *gorm.DB, orgID int64, provider string, isActive bool, updatedAt time.Time) (bool, error)
}
