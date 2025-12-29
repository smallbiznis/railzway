package service

import (
	"context"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	billingcycledomain "github.com/smallbiznis/valora/internal/billingcycle/domain"
	subscriptiondomain "github.com/smallbiznis/valora/internal/subscription/domain"
	"github.com/smallbiznis/valora/pkg/repository"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ServiceParam struct {
	fx.In

	DB    *gorm.DB
	Log   *zap.Logger
	GenID *snowflake.Node
}

type Service struct {
	db  *gorm.DB
	log *zap.Logger

	genID            *snowflake.Node
	billingcyclerepo repository.Repository[billingcycledomain.BillingCycle]
}

func NewService(p ServiceParam) billingcycledomain.Service {
	return &Service{
		db:  p.DB,
		log: p.Log.Named("billingcycle.service"),

		genID:            p.GenID,
		billingcyclerepo: repository.ProvideStore[billingcycledomain.BillingCycle](p.DB),
	}
}

func (s *Service) List(ctx context.Context, req billingcycledomain.ListBillingCycleRequest) (billingcycledomain.ListBillingCycleResponse, error) {
	_ = ctx
	_ = req
	return billingcycledomain.ListBillingCycleResponse{}, nil
}

func (s *Service) EnsureBillingCycles(ctx context.Context) error {
	now := time.Now().UTC()
	subscriptions, err := s.listActiveSubscriptions(ctx)
	if err != nil {
		return err
	}

	for _, subscription := range subscriptions {
		if err := s.ensureSubscriptionCycles(ctx, subscription, now); err != nil {
			return err
		}
	}

	return nil
}

type activeSubscription struct {
	ID               snowflake.ID
	OrgID            snowflake.ID
	Status           subscriptiondomain.SubscriptionStatus
	ActivatedAt      *time.Time
	BillingCycleType string
}

type billingCycleRow struct {
	ID             snowflake.ID
	OrgID          snowflake.ID
	SubscriptionID snowflake.ID
	PeriodStart    time.Time
	PeriodEnd      time.Time
	Status         billingcycledomain.BillingCycleStatus
	OpenedAt       *time.Time
	ClosedAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (s *Service) listActiveSubscriptions(ctx context.Context) ([]activeSubscription, error) {
	var subscriptions []activeSubscription
	err := s.db.WithContext(ctx).Raw(
		`SELECT id, org_id, status, activated_at, billing_cycle_type
		 FROM subscriptions
		 WHERE status = ?
		 ORDER BY id`,
		subscriptiondomain.SubscriptionStatusActive,
	).Scan(&subscriptions).Error
	if err != nil {
		return nil, err
	}
	return subscriptions, nil
}

func (s *Service) ensureSubscriptionCycles(ctx context.Context, subscription activeSubscription, now time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locked, err := s.lockSubscription(ctx, tx, subscription.OrgID, subscription.ID)
		if err != nil {
			return err
		}
		if locked == nil || locked.Status != subscriptiondomain.SubscriptionStatusActive {
			return nil
		}
		if locked.ActivatedAt == nil {
			s.log.Warn("active subscription missing activated_at", zap.String("subscription_id", locked.ID.String()))
			return nil
		}

		openCycle, err := s.findOpenCycle(ctx, tx, locked.OrgID, locked.ID)
		if err != nil {
			return err
		}

		if openCycle != nil {
			if now.Before(openCycle.PeriodEnd) {
				return nil
			}
			if err := s.closeCycle(ctx, tx, openCycle, now); err != nil {
				return err
			}
			if openCycle.Status != billingcycledomain.BillingCycleStatusClosed {
				return nil
			}
		}

		lastCycle, err := s.findLastCycle(ctx, tx, locked.OrgID, locked.ID)
		if err != nil {
			return err
		}

		periodStart := *locked.ActivatedAt
		if lastCycle != nil && lastCycle.PeriodEnd.After(periodStart) {
			periodStart = lastCycle.PeriodEnd
		}

		if periodStart.After(now) {
			return nil
		}

		periodEnd, err := nextPeriodEnd(periodStart, locked.BillingCycleType)
		if err != nil {
			return err
		}
		if !periodEnd.After(periodStart) {
			return billingcycledomain.ErrInvalidCyclePeriod
		}

		return s.insertCycle(ctx, tx, locked.OrgID, locked.ID, periodStart, periodEnd, now)
	})
}

func (s *Service) lockSubscription(ctx context.Context, tx *gorm.DB, orgID, subscriptionID snowflake.ID) (*activeSubscription, error) {
	var subscription activeSubscription
	err := tx.WithContext(ctx).Raw(
		`SELECT id, org_id, status, activated_at, billing_cycle_type
		 FROM subscriptions
		 WHERE org_id = ? AND id = ?
		 FOR UPDATE`,
		orgID,
		subscriptionID,
	).Scan(&subscription).Error
	if err != nil {
		return nil, err
	}
	if subscription.ID == 0 {
		return nil, nil
	}
	return &subscription, nil
}

func (s *Service) findOpenCycle(ctx context.Context, tx *gorm.DB, orgID, subscriptionID snowflake.ID) (*billingCycleRow, error) {
	openCount, err := s.countOpenCycles(ctx, tx, orgID, subscriptionID)
	if err != nil {
		return nil, err
	}
	if openCount > 1 {
		return nil, billingcycledomain.ErrMultipleOpenCycles
	}

	var cycle billingCycleRow
	err = tx.WithContext(ctx).Raw(
		`SELECT id, org_id, subscription_id, period_start, period_end, status, opened_at, closed_at, created_at, updated_at
		 FROM billing_cycles
		 WHERE org_id = ? AND subscription_id = ? AND status IN (?, ?)
		 ORDER BY period_start DESC
		 LIMIT 1
		 FOR UPDATE`,
		orgID,
		subscriptionID,
		billingcycledomain.BillingCycleStatusOpen,
		billingcycledomain.BillingCycleStatusClosing,
	).Scan(&cycle).Error
	if err != nil {
		return nil, err
	}
	if cycle.ID == 0 {
		return nil, nil
	}
	return &cycle, nil
}

func (s *Service) countOpenCycles(ctx context.Context, tx *gorm.DB, orgID, subscriptionID snowflake.ID) (int64, error) {
	var count int64
	if err := tx.WithContext(ctx).Raw(
		`SELECT COUNT(1)
		 FROM billing_cycles
		 WHERE org_id = ? AND subscription_id = ? AND status IN (?, ?)`,
		orgID,
		subscriptionID,
		billingcycledomain.BillingCycleStatusOpen,
		billingcycledomain.BillingCycleStatusClosing,
	).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Service) findLastCycle(ctx context.Context, tx *gorm.DB, orgID, subscriptionID snowflake.ID) (*billingCycleRow, error) {
	var cycle billingCycleRow
	err := tx.WithContext(ctx).Raw(
		`SELECT id, org_id, subscription_id, period_start, period_end, status, opened_at, closed_at, created_at, updated_at
		 FROM billing_cycles
		 WHERE org_id = ? AND subscription_id = ?
		 ORDER BY period_end DESC
		 LIMIT 1`,
		orgID,
		subscriptionID,
	).Scan(&cycle).Error
	if err != nil {
		return nil, err
	}
	if cycle.ID == 0 {
		return nil, nil
	}
	return &cycle, nil
}

func (s *Service) closeCycle(ctx context.Context, tx *gorm.DB, cycle *billingCycleRow, now time.Time) error {
	if cycle.Status == billingcycledomain.BillingCycleStatusOpen {
		if err := s.updateCycleStatus(ctx, tx, cycle, billingcycledomain.BillingCycleStatusClosing, now, cycle.ClosedAt); err != nil {
			return err
		}
	}

	if err := s.triggerCycleFinalization(ctx, tx, cycle); err != nil {
		return err
	}

	if cycle.Status != billingcycledomain.BillingCycleStatusClosed {
		closedAt := now
		return s.updateCycleStatus(ctx, tx, cycle, billingcycledomain.BillingCycleStatusClosed, now, &closedAt)
	}

	return nil
}

func (s *Service) updateCycleStatus(ctx context.Context, tx *gorm.DB, cycle *billingCycleRow, status billingcycledomain.BillingCycleStatus, now time.Time, closedAt *time.Time) error {
	cycle.Status = status
	cycle.UpdatedAt = now
	if closedAt != nil {
		cycle.ClosedAt = closedAt
	}
	return tx.WithContext(ctx).Exec(
		`UPDATE billing_cycles
		 SET status = ?, closed_at = ?, updated_at = ?
		 WHERE org_id = ? AND id = ?`,
		cycle.Status,
		cycle.ClosedAt,
		cycle.UpdatedAt,
		cycle.OrgID,
		cycle.ID,
	).Error
}

func (s *Service) triggerCycleFinalization(ctx context.Context, tx *gorm.DB, cycle *billingCycleRow) error {
	_ = ctx
	_ = tx
	_ = cycle
	return nil
}

func (s *Service) insertCycle(ctx context.Context, tx *gorm.DB, orgID, subscriptionID snowflake.ID, periodStart, periodEnd, now time.Time) error {
	openedAt := now
	return tx.WithContext(ctx).Exec(
		`INSERT INTO billing_cycles (
			id, org_id, subscription_id, period_start, period_end, status, opened_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.genID.Generate(),
		orgID,
		subscriptionID,
		periodStart,
		periodEnd,
		billingcycledomain.BillingCycleStatusOpen,
		openedAt,
		now,
		now,
	).Error
}

func nextPeriodEnd(start time.Time, cycleType string) (time.Time, error) {
	switch strings.ToLower(strings.TrimSpace(cycleType)) {
	case "monthly":
		return start.AddDate(0, 1, 0), nil
	case "weekly":
		return start.AddDate(0, 0, 7), nil
	case "daily":
		return start.AddDate(0, 0, 1), nil
	default:
		return time.Time{}, subscriptiondomain.ErrInvalidBillingCycleType
	}
}
