package service

import (
	"context"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
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
	Repo      priceamountdomain.Repository
	PriceRepo pricedomain.Repository
}

type Service struct {
	db        *gorm.DB
	log       *zap.Logger
	genID     *snowflake.Node
	repo      priceamountdomain.Repository
	priceRepo pricedomain.Repository
}

func New(p Params) priceamountdomain.Service {
	return &Service{
		db:        p.DB,
		log:       p.Log.Named("priceamount.service"),
		genID:     p.GenID,
		repo:      p.Repo,
		priceRepo: p.PriceRepo,
	}
}

func (s *Service) Create(ctx context.Context, req priceamountdomain.CreateRequest) (*priceamountdomain.Response, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	priceID, meterID, currency, err := s.parseAmountIdentifiers(req)
	if err != nil {
		return nil, err
	}

	effectiveFrom, effectiveTo, err := s.resolveEffectiveRange(req)
	if err != nil {
		return nil, err
	}

	if err := validateAmountValues(req); err != nil {
		return nil, err
	}

	var entity *priceamountdomain.PriceAmount
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.ensurePriceExists(ctx, tx, orgID, priceID); err != nil {
			return err
		}

		current, err := s.repo.FindEffectiveAt(ctx, tx, orgID, priceID, meterID, currency, effectiveFrom)
		if err != nil {
			return err
		}

		next, err := s.repo.FindNext(ctx, tx, orgID, priceID, meterID, currency, effectiveFrom)
		if err != nil {
			return err
		}

		effectiveTo, err = s.alignEffectiveTo(effectiveFrom, effectiveTo, next)
		if err != nil {
			return err
		}

		if err := s.validateContinuity(ctx, tx, orgID, priceID, meterID, currency, effectiveFrom, current); err != nil {
			return err
		}

		now := time.Now().UTC()
		if current != nil {
			current.EffectiveTo = &effectiveFrom
			current.UpdatedAt = now
			if _, err := s.repo.Update(ctx, tx, current); err != nil {
				return err
			}
		}

		// Pricing is versioned and append-only: we close the prior window, then insert a new row.
		entity = &priceamountdomain.PriceAmount{
			ID:                 s.genID.Generate(),
			OrgID:              orgID,
			PriceID:            priceID,
			MeterID:            meterID,
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

func (s *Service) List(ctx context.Context, req priceamountdomain.ListPriceAmountRequest) ([]priceamountdomain.Response, error) {
	filter := priceamountdomain.PriceAmount{}

	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	filter.OrgID = orgID

	var priceID snowflake.ID
	if req.PriceID != "" {
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
			Operator: option.GTE,
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
	if req.EffectiveFrom == nil && req.EffectiveTo == nil {
		now := time.Now().UTC()
		opts = append(opts, option.ApplyOperator(option.Condition{
			Field:    "effective_from",
			Operator: option.LTE,
			Value:    now,
		}))

		opts = append(opts, whereOption{
			query: "(effective_to IS NULL OR effective_to > ?)",
			args:  []any{now},
		})
	}

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
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
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

func (s *Service) orgIDFromContext(ctx context.Context) (snowflake.ID, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return 0, priceamountdomain.ErrInvalidOrganization
	}
	return snowflake.ID(orgID), nil
}

func (s *Service) priceExists(ctx context.Context, db *gorm.DB, orgID, priceID snowflake.ID) (bool, error) {
	item, err := s.priceRepo.FindByID(ctx, db, orgID, priceID)
	if err != nil {
		return false, err
	}
	return item != nil, nil
}

func (s *Service) ensurePriceExists(ctx context.Context, db *gorm.DB, orgID, priceID snowflake.ID) error {
	exists, err := s.priceExists(ctx, db, orgID, priceID)
	if err != nil {
		return err
	}
	if !exists {
		return priceamountdomain.ErrInvalidPrice
	}
	return nil
}

func (s *Service) toResponse(a *priceamountdomain.PriceAmount) *priceamountdomain.Response {
	var meterID *string
	if a.MeterID != nil {
		value := a.MeterID.String()
		meterID = &value
	}

	return &priceamountdomain.Response{
		ID:                 a.ID.String(),
		OrganizationID:     a.OrgID.String(),
		PriceID:            a.PriceID.String(),
		MeterID:            meterID,
		Currency:           a.Currency,
		UnitAmountCents:    a.UnitAmountCents,
		MinimumAmountCents: a.MinimumAmountCents,
		MaximumAmountCents: a.MaximumAmountCents,
		EffectiveFrom:      a.EffectiveFrom,
		EffectiveTo:        a.EffectiveTo,
		CreatedAt:          a.CreatedAt,
		UpdatedAt:          a.UpdatedAt,
	}
}

func parseID(value string) (snowflake.ID, error) {
	return snowflake.ParseString(strings.TrimSpace(value))
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

type whereOption struct {
	query string
	args  []any
}

func (o whereOption) Apply(db *gorm.DB) *gorm.DB {
	return db.Where(o.query, o.args...)
}

func (s *Service) resolveEffectiveRange(req priceamountdomain.CreateRequest) (time.Time, *time.Time, error) {
	effectiveFrom := time.Now().UTC()
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

func (s *Service) alignEffectiveTo(
	effectiveFrom time.Time,
	requested *time.Time,
	next *priceamountdomain.PriceAmount,
) (*time.Time, error) {
	if next == nil {
		if requested != nil {
			return nil, priceamountdomain.ErrEffectiveGap
		}
		return nil, nil
	}

	nextStart := next.EffectiveFrom.UTC()
	if requested == nil {
		if !nextStart.After(effectiveFrom) {
			return nil, priceamountdomain.ErrEffectiveOverlap
		}
		value := nextStart
		return &value, nil
	}

	if requested.Equal(nextStart) {
		return requested, nil
	}
	if requested.Before(nextStart) {
		return nil, priceamountdomain.ErrEffectiveGap
	}
	return nil, priceamountdomain.ErrEffectiveOverlap
}

func (s *Service) validateContinuity(
	ctx context.Context,
	db *gorm.DB,
	orgID, priceID snowflake.ID,
	meterID *snowflake.ID,
	currency string,
	effectiveFrom time.Time,
	current *priceamountdomain.PriceAmount,
) error {
	if current != nil {
		if !effectiveFrom.After(current.EffectiveFrom) {
			return priceamountdomain.ErrEffectiveOverlap
		}
		return nil
	}

	previous, err := s.repo.FindPrevious(ctx, db, orgID, priceID, meterID, currency, effectiveFrom)
	if err != nil {
		return err
	}
	if previous == nil {
		return nil
	}
	if previous.EffectiveTo == nil {
		return priceamountdomain.ErrEffectiveOverlap
	}
	if previous.EffectiveTo.Before(effectiveFrom) {
		return priceamountdomain.ErrEffectiveGap
	}
	if !previous.EffectiveTo.Equal(effectiveFrom) {
		return priceamountdomain.ErrEffectiveOverlap
	}
	return nil
}
