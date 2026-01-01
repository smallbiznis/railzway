package domain

import (
	"context"

	"github.com/bwmarrin/snowflake"
	"gorm.io/gorm"
)

type Repository interface {
	Insert(ctx context.Context, db *gorm.DB, tmpl *InvoiceTemplate) error
	Update(ctx context.Context, db *gorm.DB, tmpl *InvoiceTemplate) error
	FindByID(ctx context.Context, db *gorm.DB, orgID, id snowflake.ID) (*InvoiceTemplate, error)
	FindDefault(ctx context.Context, db *gorm.DB, orgID snowflake.ID) (*InvoiceTemplate, error)
	List(ctx context.Context, db *gorm.DB, orgID snowflake.ID, filter ListRequest) ([]InvoiceTemplate, error)
}
