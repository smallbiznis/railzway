package domain

import (
	"context"

	"github.com/bwmarrin/snowflake"
	"gorm.io/gorm"
)

type Repository interface {
	Insert(ctx context.Context, db *gorm.DB, key *APIKey) error
	Update(ctx context.Context, db *gorm.DB, key *APIKey) error
	FindByKeyID(ctx context.Context, db *gorm.DB, orgID snowflake.ID, keyID string) (*APIKey, error)
	List(ctx context.Context, db *gorm.DB, orgID snowflake.ID) ([]APIKey, error)
}
