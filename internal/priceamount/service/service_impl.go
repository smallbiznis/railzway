package service

import (
	"context"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/clock"
	"github.com/smallbiznis/valora/internal/orgcontext"
	pricedomain "github.com/smallbiznis/valora/internal/price/domain"
	priceamountdomain "github.com/smallbiznis/valora/internal/priceamount/domain"
	"github.com/smallbiznis/valora/pkg/db/option"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB        *gorm.DB
	Log       *zap.Logger
	GenID     *snowflake.Node
	Clock     clock.Clock
	Repo      priceamountdomain.Repository
	PriceRepo pricedomain.Repository
}

type Service struct {
	db        *gorm.DB
	log       *zap.Logger
	genID     *snowflake.Node
	clock     clock.Clock
	repo      priceamountdomain.Repository
	priceRepo pricedomain.Repository
}

func New(p Params) priceamountdomain.Service {
	return &Service{
		db:        p.DB,
		log:       p.Log.Named("priceamount.service"),
		genID:     p.GenID,
		clock:     p.Clock,
		repo:      p.Repo,
		priceRepo: p.PriceRepo,
	}
}

// normalizeToMinutePrecision truncates time to minute precision in UTC.
// This ensures consistent pricing window boundaries.
func normalizeToMinutePrecision(t time.Time) time.Time {
	return t.UTC().Truncate(time.Minute)
}

func (s *Service) Create(ctx context.Context, req priceamountdomain.CreateRequest) (*priceamountdomain.Response, error) {
	// 1. Resolve organization context
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return nil, priceamountdomain.ErrInvalidOrganization
	}

	// 2. Parse and validate request identifiers
	priceID, requestedMeterID, currency, err := s.parseAmountIdentifiers(req)
	if err != nil {
		return nil, err
	}

	// 3. Validate amount values before proceeding
	if err := validateAmountValues(req); err != nil {
		return nil, err
	}

	// 4. Normalize effective_from to minute precision
	effectiveFrom := normalizeToMinutePrecision(s.clock.Now())
	if req.EffectiveFrom != nil {
		effectiveFrom = normalizeToMinutePrecision(*req.EffectiveFrom)
	}
	if effectiveFrom.IsZero() {
		return nil, priceamountdomain.ErrInvalidEffectiveFrom
	}

	// 5. Validate effective_to if provided
	var effectiveTo *time.Time
	if req.EffectiveTo != nil {
		normalized := normalizeToMinutePrecision(*req.EffectiveTo)
		effectiveTo = &normalized
		if !effectiveTo.After(effectiveFrom) {
			return nil, priceamountdomain.ErrInvalidEffectiveTo
		}
	}

	var entity *priceamountdomain.PriceAmount
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 6. Ensure the price exists
		price, err := s.ensurePriceExists(ctx, tx, orgID, priceID)
		if err != nil {
			return err
		}

		if price == nil {
			return priceamountdomain.ErrInvalidPrice
		}

		// 7. CRITICAL: Resolve the pricing dimension
		// This determines the immutable (price_id, meter_id, currency) tuple
		meterID, isVersioning, err := s.resolvePricingDimension(
			ctx, tx, orgID, priceID, requestedMeterID, currency, effectiveFrom,
		)
		if err != nil {
			return err
		}

		// 8. If versioning, validate continuity and close current window
		if isVersioning {
			current, err := s.repo.FindEffectiveAt(ctx, tx, orgID, priceID, meterID, currency, effectiveFrom.Add(-time.Minute))
			if err != nil {
				return err
			}
			if current == nil {
				s.log.Error("expected current version during versioning, but none found")
				return priceamountdomain.ErrEffectiveGap
			}

			// Validate: new effective_from must be strictly after current start
			if !effectiveFrom.After(current.EffectiveFrom) {
				return priceamountdomain.ErrEffectiveOverlap
			}

			// Close the current window
			current.EffectiveTo = &effectiveFrom
			current.UpdatedAt = s.clock.Now()
			if _, err := s.repo.Update(ctx, tx, current); err != nil {
				return err
			}
		}

		// 9. Check for conflicts with future versions
		next, err := s.repo.FindNext(ctx, tx, orgID, priceID, meterID, currency, effectiveFrom)
		if err != nil {
			return err
		}

		// Align effective_to to prevent gaps/overlaps
		effectiveTo, err = s.alignEffectiveTo(effectiveFrom, effectiveTo, next)
		if err != nil {
			return err
		}

		// Check if there is already an upcoming price
		upcoming, err := s.repo.FindUpcoming(
			ctx, tx,
			orgID,
			priceID,
			meterID,
			currency,
		)
		if err != nil {
			return err
		}
		if upcoming != nil {
			return priceamountdomain.ErrUpcomingAlreadyExists
		}

		// 10. Insert new price amount (append-only)
		now := s.clock.Now()
		entity = &priceamountdomain.PriceAmount{
			ID:                 s.genID.Generate(),
			OrgID:              orgID,
			PriceID:            priceID,
			MeterID:            meterID, // Use resolved dimension, not request
			Currency:           currency,
			UnitAmountCents:    req.UnitAmountCents,
			MinimumAmountCents: req.MinimumAmountCents,
			MaximumAmountCents: req.MaximumAmountCents,
			EffectiveFrom:      effectiveFrom,
			EffectiveTo:        effectiveTo,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if req.Metadata != nil {
			entity.Metadata = datatypes.JSONMap(req.Metadata)
		}

		return s.repo.Insert(ctx, tx, entity)
	}); err != nil {
		return nil, err
	}

	return s.toResponse(entity), nil
}

// resolvePricingDimension determines the immutable pricing dimension for this request.
// Returns: (meterID, isVersioning, error)
//
// If meter_id is provided in the request:
//   - For initial creation: use the provided meter_id
//   - For versioning: verify it matches the existing dimension
//
// If meter_id is NOT provided:
//   - Load the latest price amount and inherit its meter_id
//   - This is standard versioning behavior in billing systems
func (s *Service) resolvePricingDimension(
	ctx context.Context,
	db *gorm.DB,
	orgID, priceID snowflake.ID,
	requestedMeterID *snowflake.ID,
	currency string,
	effectiveFrom time.Time,
) (*snowflake.ID, bool, error) {
	// Attempt to find the latest existing price amount for this dimension
	latest, err := s.repo.FindLatestByPriceAndCurrency(ctx, db, orgID, priceID, currency)
	if err != nil {
		return nil, false, err
	}

	// Case 1: No existing price amount - initial creation
	if latest == nil {
		// If meter_id is provided, this is a usage-based price
		if requestedMeterID != nil {
			return requestedMeterID, false, nil
		}
		// Otherwise, this is a flat price
		return nil, false, nil
	}

	// Case 2: Existing price amount found - this is versioning
	existingMeterID := latest.MeterID

	// If meter_id was explicitly provided, it MUST match the existing dimension
	if requestedMeterID != nil {
		if !metersMatch(existingMeterID, requestedMeterID) {
			// Attempting to change the pricing dimension during versioning
			s.log.Warn("meter_id mismatch during versioning",
				zap.String("existing", formatMeterID(existingMeterID)),
				zap.String("requested", formatMeterID(requestedMeterID)),
			)
			return nil, false, priceamountdomain.ErrInvalidMeterID
		}
	}

	// Inherit meter_id from the existing dimension
	return existingMeterID, true, nil
}

// metersMatch compares two meter IDs, handling nil correctly
func metersMatch(a, b *snowflake.ID) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// formatMeterID safely formats a meter ID for logging
func formatMeterID(id *snowflake.ID) string {
	if id == nil {
		return "NULL"
	}
	return id.String()
}

// alignEffectiveTo ensures no gaps or overlaps with future versions.
// This enforces continuous pricing history within each dimension.
func (s *Service) alignEffectiveTo(
	effectiveFrom time.Time,
	requested *time.Time,
	next *priceamountdomain.PriceAmount,
) (*time.Time, error) {
	// Case 1: No future version exists
	if next == nil {
		if requested != nil {
			// User wants to close this window, but there's no next version
			return nil, priceamountdomain.ErrEffectiveGap
		}
		// Open-ended window (effective_to = NULL)
		return nil, nil
	}

	// Case 2: Future version exists - must align boundaries
	nextStart := normalizeToMinutePrecision(next.EffectiveFrom)

	if requested == nil {
		// Auto-align: close this window at next version's start
		if !nextStart.After(effectiveFrom) {
			return nil, priceamountdomain.ErrEffectiveOverlap
		}
		return &nextStart, nil
	}

	// User specified effective_to - validate alignment
	if requested.Equal(nextStart) {
		return requested, nil
	}
	if requested.Before(nextStart) {
		// Would create a gap
		return nil, priceamountdomain.ErrEffectiveGap
	}
	// Would overlap with next version
	return nil, priceamountdomain.ErrEffectiveOverlap
}

// Validation and helper methods below remain largely unchanged

func (s *Service) ensurePriceExists(ctx context.Context, db *gorm.DB, orgID, priceID snowflake.ID) (*pricedomain.Price, error) {
	item, err := s.priceRepo.FindByID(ctx, db, orgID, priceID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, priceamountdomain.ErrInvalidPrice
	}
	return item, nil
}

func (s *Service) parseAmountIdentifiers(req priceamountdomain.CreateRequest) (snowflake.ID, *snowflake.ID, string, error) {
	priceID, err := parseID(req.PriceID)
	if err != nil {
		return 0, nil, "", priceamountdomain.ErrInvalidPrice
	}

	var meterID *snowflake.ID
	if req.MeterID != nil && strings.TrimSpace(*req.MeterID) != "" {
		parsedMeterID, err := parseID(*req.MeterID)
		if err != nil {
			return 0, nil, "", priceamountdomain.ErrInvalidMeterID
		}
		meterID = &parsedMeterID
	}

	currency := strings.TrimSpace(req.Currency)
	if currency == "" {
		return 0, nil, "", priceamountdomain.ErrInvalidCurrency
	}

	return priceID, meterID, currency, nil
}

func validateAmountValues(req priceamountdomain.CreateRequest) error {
	if req.UnitAmountCents < 0 {
		return priceamountdomain.ErrInvalidUnitAmount
	}

	if req.MinimumAmountCents != nil && *req.MinimumAmountCents < 0 {
		return priceamountdomain.ErrInvalidMinAmount
	}

	if req.MaximumAmountCents != nil && *req.MaximumAmountCents < 0 {
		return priceamountdomain.ErrInvalidMaxAmount
	}

	if req.MinimumAmountCents != nil && req.MaximumAmountCents != nil && *req.MaximumAmountCents < *req.MinimumAmountCents {
		return priceamountdomain.ErrInvalidMaxAmount
	}

	return nil
}

func (s *Service) toResponse(a *priceamountdomain.PriceAmount) *priceamountdomain.Response {
	var meterID *snowflake.ID
	if a.MeterID != nil {
		value := a.MeterID
		meterID = value
	}

	return &priceamountdomain.Response{
		ID:                 a.ID,
		OrganizationID:     a.OrgID,
		PriceID:            a.PriceID,
		MeterID:            meterID,
		Currency:           a.Currency,
		UnitAmountCents:    a.UnitAmountCents,
		MinimumAmountCents: a.MinimumAmountCents,
		MaximumAmountCents: a.MaximumAmountCents,
		EffectiveFrom:      a.EffectiveFrom,
		EffectiveTo:        a.EffectiveTo,
		RevokedAt:          a.RevokedAt,
		RevokedReason:      a.RevokedReason,
		Status:             deriveStatus(a, s.clock.Now()),
		CreatedAt:          a.CreatedAt,
		UpdatedAt:          a.UpdatedAt,
	}
}

func (s *Service) List(ctx context.Context, req priceamountdomain.ListPriceAmountRequest) ([]priceamountdomain.Response, error) {
	filter := priceamountdomain.PriceAmount{}

	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return nil, priceamountdomain.ErrInvalidOrganization
	}
	filter.OrgID = orgID

	var priceID snowflake.ID
	if req.PriceID != "" {
		var err error
		priceID, err = parseID(req.PriceID)
		if err != nil {
			return nil, priceamountdomain.ErrInvalidPrice
		}
		filter.PriceID = priceID
	}

	opts := []option.QueryOption{}

	// Apply effective date filters
	if req.EffectiveFrom != nil {
		opts = append(opts, option.ApplyOperator(option.Condition{
			Field:    "effective_from",
			Operator: option.LTE,
			Value:    *req.EffectiveFrom,
		}))
	}

	// Filter by EffectiveTo if provided
	if req.EffectiveTo != nil {
		opts = append(opts, option.ApplyOperator(option.Condition{
			Field:    "effective_to",
			Operator: option.LTE,
			Value:    *req.EffectiveTo,
		}))
	}

	// If no effective date filters are provided, default to currently effective amounts
	// if req.EffectiveFrom == nil && req.EffectiveTo == nil {
	// 	now := time.Now().UTC()
	// 	opts = append(opts, option.ApplyOperator(option.Condition{
	// 		Field:    "effective_from",
	// 		Operator: option.LTE,
	// 		Value:    now,
	// 	}))

	// 	opts = append(opts, whereOption{
	// 		query: "(effective_to IS NULL OR effective_to > ?)",
	// 		args:  []any{now},
	// 	})
	// }

	items, err := s.repo.List(ctx, s.db, filter, opts...)
	if err != nil {
		return nil, err
	}

	resp := make([]priceamountdomain.Response, 0, len(items))
	for i := range items {
		resp = append(resp, *s.toResponse(&items[i]))
	}

	return resp, nil
}

func (s *Service) Get(ctx context.Context, req priceamountdomain.GetPriceAmountByID) (*priceamountdomain.Response, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return nil, priceamountdomain.ErrInvalidOrganization
	}

	amountID, err := parseID(req.ID)
	if err != nil {
		return nil, priceamountdomain.ErrInvalidID
	}

	entity, err := s.repo.FindByID(ctx, s.db, orgID, amountID)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, priceamountdomain.ErrNotFound
	}

	return s.toResponse(entity), nil
}

func parseID(value string) (snowflake.ID, error) {
	return snowflake.ParseString(strings.TrimSpace(value))
}

type whereOption struct {
	query string
	args  []any
}

func (o whereOption) Apply(db *gorm.DB) *gorm.DB {
	return db.Where(o.query, o.args...)
}

func (s *Service) resolveEffectiveRange(req priceamountdomain.CreateRequest) (time.Time, *time.Time, error) {
	effectiveFrom := s.clock.Now()
	if req.EffectiveFrom != nil {
		effectiveFrom = req.EffectiveFrom.UTC()
	}
	if effectiveFrom.IsZero() {
		return time.Time{}, nil, priceamountdomain.ErrInvalidEffectiveFrom
	}

	var effectiveTo *time.Time
	if req.EffectiveTo != nil {
		value := req.EffectiveTo.UTC()
		effectiveTo = &value
		if !effectiveTo.After(effectiveFrom) {
			return time.Time{}, nil, priceamountdomain.ErrInvalidEffectiveTo
		}
	}

	return effectiveFrom, effectiveTo, nil
}

func deriveStatus(a *priceamountdomain.PriceAmount, now time.Time) string {
	if a.RevokedAt != nil {
		return "revoked"
	}

	if a.EffectiveFrom.After(now) {
		return "upcoming"
	}

	if a.EffectiveTo != nil && !a.EffectiveTo.After(now) {
		return "expired"
	}

	return "active"
}
