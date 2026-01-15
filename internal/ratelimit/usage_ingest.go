package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
	"github.com/smallbiznis/railzway/internal/config"
)

const (
	keyUsageIngestOrg      = "usage:ingest:org:%s"
	keyUsageIngestEndpoint = "usage:ingest:endpoint:%s"
	keyUsageIngestLock     = "usage:ingest:lock:%s:%s:%s"
)

type UsageIngestLimiter struct {
	enabled bool

	bucket *TokenBucket
	locker *Locker

	orgRate       float64
	orgBurst      int
	endpointRate  float64
	endpointBurst int
	lockTTL       time.Duration
}

func NewUsageIngestLimiter(cfg config.Config) (*UsageIngestLimiter, error) {
	limitCfg := cfg.RateLimit
	if !limitCfg.Enabled {
		return nil, nil
	}

	addr := strings.TrimSpace(limitCfg.RedisAddr)
	if addr == "" {
		return nil, errors.New("rate limit redis addr is required")
	}
	if limitCfg.UsageIngestOrgRate <= 0 || limitCfg.UsageIngestOrgBurst <= 0 {
		return nil, errors.New("usage ingest org rate limit must be positive")
	}
	if limitCfg.UsageIngestEndpointRate <= 0 || limitCfg.UsageIngestEndpointBurst <= 0 {
		return nil, errors.New("usage ingest endpoint rate limit must be positive")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: strings.TrimSpace(limitCfg.RedisPassword),
		DB:       limitCfg.RedisDB,
	})

	return &UsageIngestLimiter{
		enabled:       true,
		bucket:        NewTokenBucket(client),
		locker:        NewLocker(client),
		orgRate:       limitCfg.UsageIngestOrgRate,
		orgBurst:      limitCfg.UsageIngestOrgBurst,
		endpointRate:  limitCfg.UsageIngestEndpointRate,
		endpointBurst: limitCfg.UsageIngestEndpointBurst,
		lockTTL:       time.Duration(limitCfg.UsageIngestConcurrencyTTLSeconds) * time.Second,
	}, nil
}

func (l *UsageIngestLimiter) Enabled() bool {
	return l != nil && l.enabled
}

func (l *UsageIngestLimiter) AllowOrg(ctx context.Context, orgID string) (*RateLimitResult, error) {
	if !l.Enabled() {
		return &RateLimitResult{Allowed: true}, nil
	}
	return l.bucket.Allow(ctx, fmt.Sprintf(keyUsageIngestOrg, strings.TrimSpace(orgID)), l.orgRate, l.orgBurst)
}

func (l *UsageIngestLimiter) AllowEndpoint(ctx context.Context, orgID string) (*RateLimitResult, error) {
	if !l.Enabled() {
		return &RateLimitResult{Allowed: true}, nil
	}
	return l.bucket.Allow(ctx, fmt.Sprintf(keyUsageIngestEndpoint, strings.TrimSpace(orgID)), l.endpointRate, l.endpointBurst)
}

func (l *UsageIngestLimiter) TryLockCustomerMeter(ctx context.Context, orgID, customerID, meterCode string) (string, bool, error) {
	if !l.Enabled() {
		return "", true, nil
	}
	key := fmt.Sprintf(
		keyUsageIngestLock,
		strings.TrimSpace(orgID),
		strings.TrimSpace(customerID),
		strings.TrimSpace(meterCode),
	)
	return l.locker.TryLock(ctx, key, l.lockTTL)
}

func (l *UsageIngestLimiter) ReleaseCustomerMeter(ctx context.Context, orgID, customerID, meterCode, token string) error {
	if !l.Enabled() {
		return nil
	}
	key := fmt.Sprintf(
		keyUsageIngestLock,
		strings.TrimSpace(orgID),
		strings.TrimSpace(customerID),
		strings.TrimSpace(meterCode),
	)
	return l.locker.Release(ctx, key, token)
}
