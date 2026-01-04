package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/datatypes"
)

// InvoiceTemplate defines the layout configuration used to render invoices.
type InvoiceTemplate struct {
	ID                 snowflake.ID      `gorm:"primaryKey"`
	OrgID              snowflake.ID      `gorm:"not null;index"`
	Name               string            `gorm:"type:text;not null"`
	IsDefault          bool              `gorm:"not null;default:false"`
	Locale             string            `gorm:"type:text;not null;default:'en'"`
	Currency           string            `gorm:"type:text;not null"`
	Header             datatypes.JSONMap `gorm:"type:jsonb"`
	Footer             datatypes.JSONMap `gorm:"type:jsonb"`
	Style              datatypes.JSONMap `gorm:"type:jsonb"`
	ShowShipTo         bool              `gorm:"not null;type:boolean;default:true"`
	ShowPaymentDetails bool              `gorm:"not null;type:boolean;default:true"`
	ShowTaxDetails     bool              `gorm:"not null;type:boolean;default:false"`
	Version            int               `gorm:"not null;type:int;default:1"`
	IsLocked           bool              `gorm:"not null;type:boolean;default:false"`
	CreatedAt          time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt          time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TableName sets the database table name.
func (InvoiceTemplate) TableName() string { return "invoice_templates" }
