package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/datatypes"
)

type PriceAmount struct {
	ID                 snowflake.ID  `json:"id" gorm:"primaryKey"`
	OrgID              snowflake.ID  `json:"organization_id" gorm:"column:org_id;not null;index"`
	PriceID            snowflake.ID  `json:"price_id" gorm:"column:price_id;not null;index"`
	MeterID            *snowflake.ID `gorm:"column:meter_id;index"`
	Currency           string        `json:"currency" gorm:"type:text;not null"`
	UnitAmountCents    int64         `json:"unit_amount_cents" gorm:"not null"`
	MinimumAmountCents *int64        `json:"minimum_amount_cents,omitempty" gorm:""`
	MaximumAmountCents *int64        `json:"maximum_amount_cents,omitempty" gorm:""`
	EffectiveFrom      time.Time     `json:"effective_from" gorm:"not null;default:CURRENT_TIMESTAMP"`
	EffectiveTo        *time.Time    `json:"effective_to,omitempty" gorm:""`

	RevokedAt     *time.Time
	RevokedReason *string

	Metadata  datatypes.JSONMap `json:"metadata,omitempty" gorm:"type:jsonb"`
	CreatedAt time.Time         `json:"created_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time         `json:"updated_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
}

func (PriceAmount) TableName() string { return "price_amounts" }
