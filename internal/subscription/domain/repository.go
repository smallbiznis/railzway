package domain

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/gorm"
)

type Repository interface {
	Insert(ctx context.Context, db *gorm.DB, subscription *Subscription) error
	InsertItems(ctx context.Context, db *gorm.DB, items []SubscriptionItem) error
	FindByID(ctx context.Context, db *gorm.DB, orgID, id snowflake.ID) (*Subscription, error)
	FindByIDForUpdate(ctx context.Context, db *gorm.DB, orgID, id snowflake.ID) (*Subscription, error)
	List(ctx context.Context, db *gorm.DB, orgID snowflake.ID) ([]Subscription, error)
	FindActiveByCustomerID(ctx context.Context, db *gorm.DB, orgID, customerID snowflake.ID, statuses []SubscriptionStatus) (*Subscription, error)
	FindActiveByCustomerIDAt(ctx context.Context, db *gorm.DB, orgID, customerID snowflake.ID, at time.Time) (*Subscription, error)
	FindSubscriptionItemByMeterID(ctx context.Context, db *gorm.DB, orgID, subscriptionID, meterID snowflake.ID) (*SubscriptionItem, error)
	FindSubscriptionItemByMeterIDAt(ctx context.Context, db *gorm.DB, orgID, subscriptionID, meterID snowflake.ID, at time.Time) (*SubscriptionItem, error)
	FindSubscriptionItemByMeterCode(ctx context.Context, db *gorm.DB, orgID, subscriptionID snowflake.ID, meterCode string) (*SubscriptionItem, error)
}
