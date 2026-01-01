package cache

import (
	"strings"
	"time"

	meterdomain "github.com/smallbiznis/valora/internal/meter/domain"
	subscriptiondomain "github.com/smallbiznis/valora/internal/subscription/domain"
)

const (
	defaultMeterTTL        = 10 * time.Minute
	defaultSubscriptionTTL = 45 * time.Second
	defaultItemTTL         = 10 * time.Minute
)

// UsageResolverCache stores hot-path resolver lookups for usage ingest.
type UsageResolverCache interface {
	GetMeter(orgID, meterCode string) (*meterdomain.Response, bool)
	SetMeter(orgID, meterCode string, meter *meterdomain.Response)
	GetActiveSubscription(orgID, customerID string) (subscriptiondomain.Subscription, bool)
	SetActiveSubscription(orgID, customerID string, subscription subscriptiondomain.Subscription)
	GetSubscriptionItem(subscriptionID, meterID string) (subscriptiondomain.SubscriptionItem, bool)
	SetSubscriptionItem(subscriptionID, meterID string, item subscriptiondomain.SubscriptionItem)
}

type usageResolverCache struct {
	meters        Cache[string, *meterdomain.Response]
	subscriptions Cache[string, subscriptiondomain.Subscription]
	items         Cache[string, subscriptiondomain.SubscriptionItem]
	meterTTL      time.Duration
	subTTL        time.Duration
	itemTTL       time.Duration
}

// NewUsageResolverCache returns an in-memory cache tuned for usage ingest.
func NewUsageResolverCache() UsageResolverCache {
	return &usageResolverCache{
		meters:        NewTTLCache[string, *meterdomain.Response](),
		subscriptions: NewTTLCache[string, subscriptiondomain.Subscription](),
		items:         NewTTLCache[string, subscriptiondomain.SubscriptionItem](),
		meterTTL:      defaultMeterTTL,
		subTTL:        defaultSubscriptionTTL,
		itemTTL:       defaultItemTTL,
	}
}

func (c *usageResolverCache) GetMeter(orgID, meterCode string) (*meterdomain.Response, bool) {
	return c.meters.Get(cacheKey(orgID, meterCode))
}

func (c *usageResolverCache) SetMeter(orgID, meterCode string, meter *meterdomain.Response) {
	if meter == nil {
		return
	}
	c.meters.Set(cacheKey(orgID, meterCode), meter, c.meterTTL)
}

func (c *usageResolverCache) GetActiveSubscription(orgID, customerID string) (subscriptiondomain.Subscription, bool) {
	return c.subscriptions.Get(cacheKey(orgID, customerID))
}

func (c *usageResolverCache) SetActiveSubscription(orgID, customerID string, subscription subscriptiondomain.Subscription) {
	if subscription.ID == 0 {
		return
	}
	c.subscriptions.Set(cacheKey(orgID, customerID), subscription, c.subTTL)
}

func (c *usageResolverCache) GetSubscriptionItem(subscriptionID, meterID string) (subscriptiondomain.SubscriptionItem, bool) {
	return c.items.Get(cacheKey(subscriptionID, meterID))
}

func (c *usageResolverCache) SetSubscriptionItem(subscriptionID, meterID string, item subscriptiondomain.SubscriptionItem) {
	if item.ID == 0 {
		return
	}
	c.items.Set(cacheKey(subscriptionID, meterID), item, c.itemTTL)
}

func cacheKey(parts ...string) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		values = append(values, strings.ToLower(trimmed))
	}
	return strings.Join(values, "|")
}
