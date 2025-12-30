package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	billingcycledomain "github.com/smallbiznis/valora/internal/billingcycle/domain"
	priceamountdomain "github.com/smallbiznis/valora/internal/priceamount/domain"
	ratingdomain "github.com/smallbiznis/valora/internal/rating/domain"
	"github.com/smallbiznis/valora/pkg/repository"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Service struct {
	db  *gorm.DB
	log *zap.Logger

	genID      *snowflake.Node
	ratingrepo repository.Repository[ratingdomain.RatingResult]
}

type ServiceParam struct {
	fx.In

	DB    *gorm.DB
	Log   *zap.Logger
	GenID *snowflake.Node
}

func NewService(p ServiceParam) ratingdomain.Service {
	return &Service{
		db:  p.DB,
		log: p.Log.Named("rating.service"),

		genID:      p.GenID,
		ratingrepo: repository.ProvideStore[ratingdomain.RatingResult](p.DB),
	}
}

func (s *Service) RunRating(ctx context.Context, billingCycleID string) error {
	cycleID, err := parseID(billingCycleID)
	if err != nil {
		return ratingdomain.ErrInvalidBillingCycle
	}

	cycle, err := s.loadBillingCycle(ctx, cycleID)
	if err != nil {
		return err
	}
	if cycle == nil {
		return ratingdomain.ErrBillingCycleNotFound
	}
	if cycle.Status != billingcycledomain.BillingCycleStatusClosing {
		return ratingdomain.ErrBillingCycleNotClosing
	}
	if !cycle.PeriodEnd.After(cycle.PeriodStart) {
		return ratingdomain.ErrInvalidBillingCycle
	}

	items, err := s.listSubscriptionItems(ctx, cycle.OrgID, cycle.SubscriptionID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return ratingdomain.ErrNoSubscriptionItems
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		for _, item := range items {

			var (
				quantity float64
				source   string
			)

			if item.MeterID == nil {
				quantity = 1.0
				source = "flat_rate"
			} else {
				qty, err := s.aggregateUsage(tx, cycle.OrgID, cycle.SubscriptionID, *item.MeterID, cycle.PeriodStart, cycle.PeriodEnd)
				if err != nil {
					return err
				}

				if qty < 0 {
					return ratingdomain.ErrInvalidQuantity
				}

				quantity = qty
				source = "usage_events"
			}

			priceAmount, err := s.loadPriceAmount(tx, cycle.OrgID, item.PriceID, item.MeterID)
			if err != nil {
				return err
			}
			if priceAmount == nil {
				return ratingdomain.ErrMissingPriceAmount
			}

			unitPrice := priceAmount.UnitAmountCents

			// Calculate raw amount
			rawAmount := quantity * float64(unitPrice)

			// Apply rounding policy (explicit!)
			amount := roundMoney(rawAmount)

			// Apply min / max guards (if defined)
			if priceAmount.MinimumAmountCents != nil && *priceAmount.MinimumAmountCents > 0 {
				amount = max(amount, *priceAmount.MinimumAmountCents)
			}
			if priceAmount.MaximumAmountCents != nil && *priceAmount.MaximumAmountCents > 0 {
				amount = min(amount, *priceAmount.MaximumAmountCents)
			}

			// Build deterministic checksum
			checksum := buildChecksum(cycle.ID, cycle.SubscriptionID, item.PriceID, item.MeterID, cycle.PeriodStart, cycle.PeriodEnd)

			// Insert rating result (idempotent)
			if err := s.insertRatingResult(tx, ratingdomain.RatingResult{
				ID:             s.genID.Generate(),
				OrgID:          cycle.OrgID,
				SubscriptionID: cycle.SubscriptionID,
				BillingCycleID: cycle.ID,
				MeterID:        item.MeterID,
				PriceID:        item.PriceID,
				Quantity:       quantity,
				UnitPrice:      unitPrice,
				Amount:         amount,
				Currency:       priceAmount.Currency,
				PeriodStart:    cycle.PeriodStart,
				PeriodEnd:      cycle.PeriodEnd,
				Source:         source,
				Checksum:       checksum,
				CreatedAt:      now,
			}); err != nil {
				return err
			}
		}

		return nil
	})
}

type billingCycleRow struct {
	ID             snowflake.ID
	OrgID          snowflake.ID
	SubscriptionID snowflake.ID
	PeriodStart    time.Time
	PeriodEnd      time.Time
	Status         billingcycledomain.BillingCycleStatus
}

type subscriptionItemRow struct {
	ID             snowflake.ID
	OrgID          snowflake.ID
	SubscriptionID snowflake.ID
	PriceID        snowflake.ID
	MeterID        *snowflake.ID
}

func (s *Service) loadBillingCycle(ctx context.Context, id snowflake.ID) (*billingCycleRow, error) {
	var row billingCycleRow
	err := s.db.WithContext(ctx).Raw(
		`SELECT id, org_id, subscription_id, period_start, period_end, status
		 FROM billing_cycles
		 WHERE id = ?`,
		id,
	).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &row, nil
}

func (s *Service) listSubscriptionItems(ctx context.Context, orgID, subscriptionID snowflake.ID) ([]subscriptionItemRow, error) {
	var items []subscriptionItemRow
	err := s.db.WithContext(ctx).Raw(
		`SELECT id, org_id, subscription_id, price_id, meter_id
		 FROM subscription_items
		 WHERE org_id = ? AND subscription_id = ?`,
		orgID,
		subscriptionID,
	).Scan(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) aggregateUsage(tx *gorm.DB, orgID, subscriptionID, meterID snowflake.ID, periodStart, periodEnd time.Time) (float64, error) {
	var quantity float64
	err := tx.Raw(
		`SELECT COALESCE(SUM(value), 0)
		 FROM usage_events
		 WHERE org_id = ? AND subscription_id = ? AND meter_id = ?
		 AND recorded_at >= ? AND recorded_at < ?`,
		orgID,
		subscriptionID,
		meterID,
		periodStart,
		periodEnd,
	).Scan(&quantity).Error
	if err != nil {
		return 0, err
	}
	return quantity, nil
}

func (s *Service) loadPriceAmount(tx *gorm.DB, orgID snowflake.ID, priceID snowflake.ID, meterID *snowflake.ID) (*priceamountdomain.PriceAmount, error) {
	var amount priceamountdomain.PriceAmount
	err := tx.Raw(
		`SELECT id, org_id, price_id, meter_id, currency, unit_amount_cents, minimum_amount_cents, maximum_amount_cents, created_at, updated_at
		 FROM price_amounts
		 WHERE org_id = ? AND price_id = ? AND (meter_id = ? OR meter_id IS NULL)
		 ORDER BY meter_id DESC
		 LIMIT 1`,
		orgID,
		priceID,
		meterID,
	).Scan(&amount).Error
	if err != nil {
		return nil, err
	}
	if amount.ID == 0 {
		return nil, nil
	}
	return &amount, nil
}

func (s *Service) insertRatingResult(tx *gorm.DB, result ratingdomain.RatingResult) error {
	return tx.Exec(
		`INSERT INTO rating_results (
			id, org_id, subscription_id, billing_cycle_id, meter_id, price_id,
			quantity, unit_price, amount, currency, period_start, period_end,
			source, checksum, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (checksum) DO NOTHING`,
		result.ID,
		result.OrgID,
		result.SubscriptionID,
		result.BillingCycleID,
		result.MeterID,
		result.PriceID,
		result.Quantity,
		result.UnitPrice,
		result.Amount,
		result.Currency,
		result.PeriodStart,
		result.PeriodEnd,
		result.Source,
		result.Checksum,
		result.CreatedAt,
	).Error
}

func buildChecksum(
	billingCycleID snowflake.ID,
	subscriptionID snowflake.ID,
	priceID snowflake.ID,
	meterID *snowflake.ID, // pointer
	periodStart, periodEnd time.Time,
) string {

	meterPart := "flat"
	if meterID != nil && *meterID != 0 {
		meterPart = meterID.String()
	}

	payload := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s",
		billingCycleID.String(),
		subscriptionID.String(),
		meterPart,
		priceID.String(),
		periodStart.UTC().Format(time.RFC3339Nano),
		periodEnd.UTC().Format(time.RFC3339Nano),
	)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func roundMoney(raw float64) int64 {
	return int64(math.Floor(raw + 0.5))
}

func parseID(value string) (snowflake.ID, error) {
	return snowflake.ParseString(strings.TrimSpace(value))
}
