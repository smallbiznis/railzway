package repository

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	subscriptiondomain "github.com/smallbiznis/valora/internal/subscription/domain"
	"gorm.io/gorm"
)

type repo struct{}

func Provide() subscriptiondomain.Repository {
	return &repo{}
}

func (r *repo) Insert(ctx context.Context, db *gorm.DB, subscription *subscriptiondomain.Subscription) error {
	return db.WithContext(ctx).Exec(
		`INSERT INTO subscriptions (
			id, org_id, customer_id, status, collection_mode, start_at, end_at, cancel_at,
			cancel_at_period_end, canceled_at, activated_at, paused_at, resumed_at, ended_at,
			billing_anchor_day, billing_cycle_type, default_payment_term_days, default_currency,
			default_tax_behavior, metadata, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		subscription.ID,
		subscription.OrgID,
		subscription.CustomerID,
		subscription.Status,
		subscription.CollectionMode,
		subscription.StartAt,
		subscription.EndAt,
		subscription.CancelAt,
		subscription.CancelAtPeriodEnd,
		subscription.CanceledAt,
		subscription.ActivatedAt,
		subscription.PausedAt,
		subscription.ResumedAt,
		subscription.EndedAt,
		subscription.BillingAnchorDay,
		subscription.BillingCycleType,
		subscription.DefaultPaymentTermDays,
		subscription.DefaultCurrency,
		subscription.DefaultTaxBehavior,
		subscription.Metadata,
		subscription.CreatedAt,
		subscription.UpdatedAt,
	).Error
}

func (r *repo) InsertItems(ctx context.Context, db *gorm.DB, items []subscriptiondomain.SubscriptionItem) error {
	if len(items) == 0 {
		return nil
	}

	for _, item := range items {
		if err := db.WithContext(ctx).Exec(
			`INSERT INTO subscription_items (
				id, org_id, subscription_id, price_id, price_code, meter_id, meter_code, quantity,
				billing_mode, usage_behavior, billing_threshold, proration_behavior, next_period_start,
				next_period_end, metadata, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID,
			item.OrgID,
			item.SubscriptionID,
			item.PriceID,
			item.PriceCode,
			item.MeterID,
			item.MeterCode,
			item.Quantity,
			item.BillingMode,
			item.UsageBehavior,
			item.BillingThreshold,
			item.ProrationBehavior,
			item.NextPeriodStart,
			item.NextPeriodEnd,
			item.Metadata,
			item.CreatedAt,
			item.UpdatedAt,
		).Error; err != nil {
			return err
		}
	}

	return nil
}

func (r *repo) FindByID(ctx context.Context, db *gorm.DB, orgID, id snowflake.ID) (*subscriptiondomain.Subscription, error) {
	var subscription subscriptiondomain.Subscription
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, customer_id, status, collection_mode, start_at, end_at, cancel_at,
		 cancel_at_period_end, canceled_at, activated_at, paused_at, resumed_at, ended_at,
		 billing_anchor_day, billing_cycle_type, default_payment_term_days, default_currency,
		 default_tax_behavior, metadata, created_at, updated_at
		 FROM subscriptions WHERE org_id = ? AND id = ?`,
		orgID,
		id,
	).Scan(&subscription).Error
	if err != nil {
		return nil, err
	}
	if subscription.ID == 0 {
		return nil, nil
	}
	return &subscription, nil
}

func (r *repo) FindByIDForUpdate(ctx context.Context, db *gorm.DB, orgID, id snowflake.ID) (*subscriptiondomain.Subscription, error) {
	var subscription subscriptiondomain.Subscription
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, customer_id, status, collection_mode, start_at, end_at, cancel_at,
		 cancel_at_period_end, canceled_at, activated_at, paused_at, resumed_at, ended_at,
		 billing_anchor_day, billing_cycle_type, default_payment_term_days, default_currency,
		 default_tax_behavior, metadata, created_at, updated_at
		 FROM subscriptions WHERE org_id = ? AND id = ? FOR UPDATE`,
		orgID,
		id,
	).Scan(&subscription).Error
	if err != nil {
		return nil, err
	}
	if subscription.ID == 0 {
		return nil, nil
	}
	return &subscription, nil
}

func (r *repo) List(ctx context.Context, db *gorm.DB, orgID snowflake.ID) ([]subscriptiondomain.Subscription, error) {
	var subscriptions []subscriptiondomain.Subscription
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, customer_id, status, collection_mode, start_at, end_at, cancel_at,
		 cancel_at_period_end, canceled_at, activated_at, paused_at, resumed_at, ended_at,
		 billing_anchor_day, billing_cycle_type, default_payment_term_days, default_currency,
		 default_tax_behavior, metadata, created_at, updated_at
		 FROM subscriptions WHERE org_id = ? ORDER BY created_at ASC`,
		orgID,
	).Scan(&subscriptions).Error
	if err != nil {
		return nil, err
	}
	return subscriptions, nil
}

func (r *repo) FindActiveByCustomerID(ctx context.Context, db *gorm.DB, orgID, customerID snowflake.ID, statuses []subscriptiondomain.SubscriptionStatus) (*subscriptiondomain.Subscription, error) {
	var subscription subscriptiondomain.Subscription
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, customer_id, status, collection_mode, start_at, end_at, cancel_at,
		 cancel_at_period_end, canceled_at, activated_at, paused_at, resumed_at, ended_at,
		 billing_anchor_day, billing_cycle_type, default_payment_term_days, default_currency,
		 default_tax_behavior, metadata, created_at, updated_at
		 FROM subscriptions
		 WHERE org_id = ? AND customer_id = ? AND status IN ?
		 ORDER BY created_at DESC
		 LIMIT 1`,
		orgID,
		customerID,
		statuses,
	).Scan(&subscription).Error
	if err != nil {
		return nil, err
	}
	if subscription.ID == 0 {
		return nil, nil
	}
	return &subscription, nil
}

func (r *repo) FindActiveByCustomerIDAt(ctx context.Context, db *gorm.DB, orgID, customerID snowflake.ID, at time.Time) (*subscriptiondomain.Subscription, error) {
	var subscription subscriptiondomain.Subscription
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, customer_id, status, collection_mode, start_at, end_at, cancel_at,
		 cancel_at_period_end, canceled_at, activated_at, paused_at, resumed_at, ended_at,
		 billing_anchor_day, billing_cycle_type, default_payment_term_days, default_currency,
		 default_tax_behavior, metadata, created_at, updated_at
		 FROM subscriptions
		 WHERE org_id = ? AND customer_id = ?
		   AND status <> ?
		   AND start_at <= ?
		   AND (end_at IS NULL OR end_at > ?)
		   AND (cancel_at IS NULL OR cancel_at > ?)
		   AND (canceled_at IS NULL OR canceled_at > ?)
		   AND (ended_at IS NULL OR ended_at > ?)
		   AND NOT (paused_at IS NOT NULL AND paused_at <= ? AND (resumed_at IS NULL OR resumed_at > ?))
		 ORDER BY start_at DESC, created_at DESC
		 LIMIT 1`,
		orgID,
		customerID,
		subscriptiondomain.SubscriptionStatusDraft,
		at,
		at,
		at,
		at,
		at,
		at,
		at,
	).Scan(&subscription).Error
	if err != nil {
		return nil, err
	}
	if subscription.ID == 0 {
		return nil, nil
	}
	return &subscription, nil
}

func (r *repo) FindSubscriptionItemByMeterCode(ctx context.Context, db *gorm.DB, orgID, subscriptionID snowflake.ID, meterCode string) (*subscriptiondomain.SubscriptionItem, error) {
	var item subscriptiondomain.SubscriptionItem
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, subscription_id, price_id, price_code, meter_id, meter_code, quantity,
		 billing_mode, usage_behavior, billing_threshold, proration_behavior, next_period_start,
		 next_period_end, metadata, created_at, updated_at
		 FROM subscription_items
		 WHERE org_id = ? AND subscription_id = ? AND meter_code = ?
		 LIMIT 1`,
		orgID,
		subscriptionID,
		meterCode,
	).Scan(&item).Error
	if err != nil {
		return nil, err
	}
	if item.ID == 0 {
		return nil, nil
	}
	return &item, nil
}

func (r *repo) FindSubscriptionItemByMeterID(ctx context.Context, db *gorm.DB, orgID, subscriptionID, meterID snowflake.ID) (*subscriptiondomain.SubscriptionItem, error) {
	var item subscriptiondomain.SubscriptionItem
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, subscription_id, price_id, price_code, meter_id, meter_code, quantity,
		 billing_mode, usage_behavior, billing_threshold, proration_behavior, next_period_start,
		 next_period_end, metadata, created_at, updated_at
		 FROM subscription_items
		 WHERE org_id = ? AND subscription_id = ? AND meter_id = ?
		 LIMIT 1`,
		orgID,
		subscriptionID,
		meterID,
	).Scan(&item).Error
	if err != nil {
		return nil, err
	}
	if item.ID == 0 {
		return nil, nil
	}
	return &item, nil
}

func (r *repo) FindSubscriptionItemByMeterIDAt(ctx context.Context, db *gorm.DB, orgID, subscriptionID, meterID snowflake.ID, at time.Time) (*subscriptiondomain.SubscriptionItem, error) {
	var item subscriptiondomain.SubscriptionItem
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, subscription_id, price_id, price_code, meter_id, meter_code, quantity,
		 billing_mode, usage_behavior, billing_threshold, proration_behavior, next_period_start,
		 next_period_end, metadata, created_at, updated_at
		 FROM subscription_items
		 WHERE org_id = ? AND subscription_id = ? AND meter_id = ?
		   AND (next_period_start IS NULL OR next_period_start <= ?)
		   AND (next_period_end IS NULL OR next_period_end > ?)
		 ORDER BY created_at DESC
		 LIMIT 1`,
		orgID,
		subscriptionID,
		meterID,
		at,
		at,
	).Scan(&item).Error
	if err != nil {
		return nil, err
	}
	if item.ID == 0 {
		return nil, nil
	}
	return &item, nil
}
