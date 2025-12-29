package service

import (
	"context"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/orgcontext"
	pricedomain "github.com/smallbiznis/valora/internal/price/domain"
	pricetierdomain "github.com/smallbiznis/valora/internal/pricetier/domain"
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
	Repo      pricetierdomain.Repository
	PriceRepo pricedomain.Repository
}

type Service struct {
	db        *gorm.DB
	log       *zap.Logger
	genID     *snowflake.Node
	repo      pricetierdomain.Repository
	priceRepo pricedomain.Repository
}

func New(p Params) pricetierdomain.Service {
	return &Service{
		db:        p.DB,
		log:       p.Log.Named("pricetier.service"),
		genID:     p.GenID,
		repo:      p.Repo,
		priceRepo: p.PriceRepo,
	}
}

func (s *Service) Create(ctx context.Context, req pricetierdomain.CreateRequest) (*pricetierdomain.Response, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	priceID, unit, err := s.parseTierIdentifiers(req)
	if err != nil {
		return nil, err
	}

	if err := validateTierValues(req); err != nil {
		return nil, err
	}

	if err := s.ensurePriceExists(ctx, orgID, priceID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	entity := &pricetierdomain.PriceTier{
		ID:              s.genID.Generate(),
		OrgID:           orgID,
		PriceID:         priceID,
		TierMode:        req.TierMode,
		StartQuantity:   req.StartQuantity,
		EndQuantity:     req.EndQuantity,
		UnitAmountCents: req.UnitAmountCents,
		FlatAmountCents: req.FlatAmountCents,
		Unit:            unit,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if req.Metadata != nil {
		entity.Metadata = datatypes.JSONMap(req.Metadata)
	}

	if err := s.repo.Insert(ctx, s.db, entity); err != nil {
		return nil, err
	}

	return s.toResponse(entity), nil
}

func (s *Service) List(ctx context.Context) ([]pricetierdomain.Response, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.List(ctx, s.db, orgID)
	if err != nil {
		return nil, err
	}

	resp := make([]pricetierdomain.Response, 0, len(items))
	for i := range items {
		resp = append(resp, *s.toResponse(&items[i]))
	}

	return resp, nil
}

func (s *Service) Get(ctx context.Context, id string) (*pricetierdomain.Response, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	tierID, err := parseID(id)
	if err != nil {
		return nil, pricetierdomain.ErrInvalidID
	}

	entity, err := s.repo.FindByID(ctx, s.db, orgID, tierID)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, pricetierdomain.ErrNotFound
	}

	return s.toResponse(entity), nil
}

func (s *Service) orgIDFromContext(ctx context.Context) (snowflake.ID, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return 0, pricetierdomain.ErrInvalidOrganization
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
		return pricetierdomain.ErrInvalidPrice
	}
	return nil
}

func (s *Service) toResponse(t *pricetierdomain.PriceTier) *pricetierdomain.Response {
	return &pricetierdomain.Response{
		ID:              t.ID.String(),
		OrganizationID:  t.OrgID.String(),
		PriceID:         t.PriceID.String(),
		TierMode:        t.TierMode,
		StartQuantity:   t.StartQuantity,
		EndQuantity:     t.EndQuantity,
		UnitAmountCents: t.UnitAmountCents,
		FlatAmountCents: t.FlatAmountCents,
		Unit:            t.Unit,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

func parseID(value string) (snowflake.ID, error) {
	return snowflake.ParseString(strings.TrimSpace(value))
}

func (s *Service) parseTierIdentifiers(req pricetierdomain.CreateRequest) (snowflake.ID, string, error) {
	priceID, err := parseID(req.PriceID)
	if err != nil {
		return 0, "", pricetierdomain.ErrInvalidPrice
	}

	unit := strings.TrimSpace(req.Unit)
	if unit == "" {
		return 0, "", pricetierdomain.ErrInvalidUnit
	}

	return priceID, unit, nil
}

func validateTierValues(req pricetierdomain.CreateRequest) error {
	if req.TierMode < 0 {
		return pricetierdomain.ErrInvalidTierMode
	}

	if req.StartQuantity <= 0 {
		return pricetierdomain.ErrInvalidStartQty
	}

	if req.EndQuantity != nil && *req.EndQuantity <= req.StartQuantity {
		return pricetierdomain.ErrInvalidEndQty
	}

	if req.UnitAmountCents != nil && *req.UnitAmountCents < 0 {
		return pricetierdomain.ErrInvalidUnitAmount
	}

	if req.FlatAmountCents != nil && *req.FlatAmountCents < 0 {
		return pricetierdomain.ErrInvalidFlatAmount
	}

	if req.UnitAmountCents == nil && req.FlatAmountCents == nil {
		return pricetierdomain.ErrInvalidUnitAmount
	}

	return nil
}
