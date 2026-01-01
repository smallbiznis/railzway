package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
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

	genID           *snowflake.Node
	ratingrepo      repository.Repository[ratingdomain.RatingResult]
	priceAmountRepo priceamountdomain.Repository
}

type ServiceParam struct {
	fx.In

	DB              *gorm.DB
	Log             *zap.Logger
	GenID           *snowflake.Node
	PriceAmountRepo priceamountdomain.Repository
}

func NewService(p ServiceParam) ratingdomain.Service {
	return &Service{
		db:  p.DB,
		log: p.Log.Named("rating.service"),

		genID:           p.GenID,
		ratingrepo:      repository.ProvideStore[ratingdomain.RatingResult](p.DB),
		priceAmountRepo: p.PriceAmountRepo,
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
			if item.MeterID == nil {
				if err := s.rateFlatItem(ctx, tx, cycle, item, now); err != nil {
					return err
				}
				continue
			}

			windows, err := s.buildPriceWindows(ctx, tx, cycle.OrgID, item.PriceID, item.MeterID, cycle.PeriodStart, cycle.PeriodEnd)
			if err != nil {
				return err
			}

			for _, window := range windows {
				qty, err := s.aggregateUsage(tx, cycle.OrgID, cycle.SubscriptionID, *item.MeterID, window.Start, window.End)
				if err != nil {
					return err
				}

				if qty < 0 {
					return ratingdomain.ErrInvalidQuantity
				}

				if err := s.insertRatingWindow(tx, cycle, item, window, qty, "usage_events", now); err != nil {
					return err
				}
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

type priceWindow struct {
	Start  time.Time
	End    time.Time
	Amount *priceamountdomain.PriceAmount
}

func (s *Service) rateFlatItem(
	ctx context.Context,
	tx *gorm.DB,
	cycle *billingCycleRow,
	item subscriptionItemRow,
	now time.Time,
) error {
	priceAmount, err := s.resolvePriceAmountAt(ctx, tx, cycle.OrgID, item.PriceID, nil, cycle.PeriodStart)
	if err != nil {
		return err
	}
	if priceAmount == nil {
		return ratingdomain.ErrMissingPriceAmount
	}

	window := priceWindow{
		Start:  cycle.PeriodStart,
		End:    cycle.PeriodEnd,
		Amount: priceAmount,
	}
	return s.insertRatingWindow(tx, cycle, item, window, 1.0, "flat_rate", now)
}

func (s *Service) buildPriceWindows(
	ctx context.Context,
	tx *gorm.DB,
	orgID, priceID snowflake.ID,
	meterID *snowflake.ID,
	periodStart, periodEnd time.Time,
) ([]priceWindow, error) {
	boundaries := []time.Time{periodStart, periodEnd}

	specific, err := s.priceAmountRepo.ListOverlapping(ctx, tx, orgID, priceID, meterID, "", periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	boundaries = appendEffectiveBoundaries(boundaries, specific, periodStart, periodEnd)

	defaults, err := s.priceAmountRepo.ListOverlapping(ctx, tx, orgID, priceID, nil, "", periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	boundaries = appendEffectiveBoundaries(boundaries, defaults, periodStart, periodEnd)

	boundaries = uniqueSortedTimes(boundaries)
	windows := make([]priceWindow, 0, len(boundaries)-1)
	for i := 0; i < len(boundaries)-1; i++ {
		start := boundaries[i]
		end := boundaries[i+1]
		if !end.After(start) {
			continue
		}

		// Resolve price by usage time to keep rating historically correct.
		amount, err := s.resolvePriceAmountAt(ctx, tx, orgID, priceID, meterID, start)
		if err != nil {
			return nil, err
		}
		if amount == nil {
			return nil, ratingdomain.ErrMissingPriceAmount
		}

		windows = append(windows, priceWindow{
			Start:  start,
			End:    end,
			Amount: amount,
		})
	}

	return windows, nil
}

func (s *Service) resolvePriceAmountAt(
	ctx context.Context,
	tx *gorm.DB,
	orgID, priceID snowflake.ID,
	meterID *snowflake.ID,
	at time.Time,
) (*priceamountdomain.PriceAmount, error) {
	amount, err := s.priceAmountRepo.FindEffectiveAt(ctx, tx, orgID, priceID, meterID, "", at)
	if err != nil {
		return nil, err
	}
	if amount != nil || meterID == nil {
		return amount, nil
	}
	return s.priceAmountRepo.FindEffectiveAt(ctx, tx, orgID, priceID, nil, "", at)
}

func (s *Service) insertRatingWindow(
	tx *gorm.DB,
	cycle *billingCycleRow,
	item subscriptionItemRow,
	window priceWindow,
	quantity float64,
	source string,
	now time.Time,
) error {
	if quantity < 0 {
		return ratingdomain.ErrInvalidQuantity
	}

	unitPrice := window.Amount.UnitAmountCents

	// Rating is windowed by price versions to keep historical invoices stable.
	rawAmount := quantity * float64(unitPrice)
	amount := roundMoney(rawAmount)

	if window.Amount.MinimumAmountCents != nil && *window.Amount.MinimumAmountCents > 0 {
		amount = max(amount, *window.Amount.MinimumAmountCents)
	}
	if window.Amount.MaximumAmountCents != nil && *window.Amount.MaximumAmountCents > 0 {
		amount = min(amount, *window.Amount.MaximumAmountCents)
	}

	checksum := buildChecksum(cycle.ID, cycle.SubscriptionID, item.PriceID, item.MeterID, window.Start, window.End)

	return s.insertRatingResult(tx, ratingdomain.RatingResult{
		ID:             s.genID.Generate(),
		OrgID:          cycle.OrgID,
		SubscriptionID: cycle.SubscriptionID,
		BillingCycleID: cycle.ID,
		MeterID:        item.MeterID,
		PriceID:        item.PriceID,
		Quantity:       quantity,
		UnitPrice:      unitPrice,
		Amount:         amount,
		Currency:       window.Amount.Currency,
		PeriodStart:    window.Start,
		PeriodEnd:      window.End,
		Source:         source,
		Checksum:       checksum,
		CreatedAt:      now,
	})
}

func appendEffectiveBoundaries(
	boundaries []time.Time,
	amounts []priceamountdomain.PriceAmount,
	periodStart, periodEnd time.Time,
) []time.Time {
	for _, amount := range amounts {
		start := amount.EffectiveFrom.UTC()
		if start.After(periodStart) && start.Before(periodEnd) {
			boundaries = append(boundaries, start)
		}
		if amount.EffectiveTo != nil {
			end := amount.EffectiveTo.UTC()
			if end.After(periodStart) && end.Before(periodEnd) {
				boundaries = append(boundaries, end)
			}
		}
	}
	return boundaries
}

func uniqueSortedTimes(times []time.Time) []time.Time {
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	out := make([]time.Time, 0, len(times))
	for _, t := range times {
		if len(out) == 0 || !t.Equal(out[len(out)-1]) {
			out = append(out, t)
		}
	}
	return out
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
