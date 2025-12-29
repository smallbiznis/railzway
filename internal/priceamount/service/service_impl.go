package service

import (
	"context"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/orgcontext"
	pricedomain "github.com/smallbiznis/valora/internal/price/domain"
	priceamountdomain "github.com/smallbiznis/valora/internal/priceamount/domain"
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

	if err := validateAmountValues(req); err != nil {
		return nil, err
	}

	if err := s.ensurePriceExists(ctx, orgID, priceID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	entity := &priceamountdomain.PriceAmount{
		ID:                 s.genID.Generate(),
		OrgID:              orgID,
		PriceID:            priceID,
		MeterID:            meterID,
		Currency:           currency,
		UnitAmountCents:    req.UnitAmountCents,
		MinimumAmountCents: req.MinimumAmountCents,
		MaximumAmountCents: req.MaximumAmountCents,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if req.Metadata != nil {
		entity.Metadata = datatypes.JSONMap(req.Metadata)
	}

	if err := s.repo.Insert(ctx, s.db, entity); err != nil {
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

	items, err := s.repo.List(ctx, s.db, filter)
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

func (s *Service) priceExists(ctx context.Context, orgID, priceID snowflake.ID) (bool, error) {
	item, err := s.priceRepo.FindByID(ctx, s.db, orgID, priceID)
	if err != nil {
		return false, err
	}
	return item != nil, nil
}

func (s *Service) ensurePriceExists(ctx context.Context, orgID, priceID snowflake.ID) error {
	exists, err := s.priceExists(ctx, orgID, priceID)
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
