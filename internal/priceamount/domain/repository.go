package domain

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/pkg/db/option"
	"gorm.io/gorm"
)

type Repository interface {
	Insert(ctx context.Context, db *gorm.DB, amount *PriceAmount) error
	FindOne(ctx context.Context, db *gorm.DB, amount *PriceAmount) (*PriceAmount, error)
	FindByID(ctx context.Context, db *gorm.DB, orgID, id snowflake.ID) (*PriceAmount, error)
	List(ctx context.Context, db *gorm.DB, f PriceAmount, opts ...option.QueryOption) ([]PriceAmount, error)
	Update(ctx context.Context, db *gorm.DB, amount *PriceAmount) (*PriceAmount, error)
	FindEffectiveAt(ctx context.Context, db *gorm.DB, orgID, priceID snowflake.ID, meterID *snowflake.ID, currency string, at time.Time) (*PriceAmount, error)
	FindPrevious(ctx context.Context, db *gorm.DB, orgID, priceID snowflake.ID, meterID *snowflake.ID, currency string, before time.Time) (*PriceAmount, error)
	FindNext(ctx context.Context, db *gorm.DB, orgID, priceID snowflake.ID, meterID *snowflake.ID, currency string, after time.Time) (*PriceAmount, error)
	ListOverlapping(ctx context.Context, db *gorm.DB, orgID, priceID snowflake.ID, meterID *snowflake.ID, currency string, start, end time.Time) ([]PriceAmount, error)
}
