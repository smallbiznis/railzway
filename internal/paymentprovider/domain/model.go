package domain

import (
	"time"

	"gorm.io/datatypes"
)

type CatalogProvider struct {
	Provider        string    `json:"provider" gorm:"primaryKey;type:text"`
	DisplayName     string    `json:"display_name" gorm:"type:text;not null"`
	Description     *string   `json:"description,omitempty" gorm:"type:text"`
	SupportsWebhook bool      `json:"supports_webhook" gorm:"not null;default:true"`
	SupportsRefund  bool      `json:"supports_refund" gorm:"not null;default:false"`
	CreatedAt       time.Time `json:"created_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
}

func (CatalogProvider) TableName() string { return "payment_provider_catalog" }

type ProviderConfig struct {
	ID        int64          `json:"id" gorm:"primaryKey"`
	OrgID     int64          `json:"organization_id" gorm:"column:org_id;not null;index:ux_payment_provider_configs_org_provider,priority:1"`
	Provider  string         `json:"provider" gorm:"type:text;not null;index:ux_payment_provider_configs_org_provider,priority:2"`
	Config    datatypes.JSON `json:"config" gorm:"type:jsonb;not null"`
	IsActive  bool           `json:"is_active" gorm:"not null;default:true"`
	CreatedAt time.Time      `json:"created_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
}

func (ProviderConfig) TableName() string { return "payment_provider_configs" }
