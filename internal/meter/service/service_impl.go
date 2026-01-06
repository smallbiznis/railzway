package service

import (
	"context"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	meterdomain "github.com/smallbiznis/valora/internal/meter/domain"
	"github.com/smallbiznis/valora/internal/orgcontext"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB    *gorm.DB
	Log   *zap.Logger
	GenID *snowflake.Node
	Repo  meterdomain.Repository
}

type Service struct {
	db    *gorm.DB
	log   *zap.Logger
	repo  meterdomain.Repository
	genID *snowflake.Node
}

func New(p Params) meterdomain.Service {
	return &Service{
		db:    p.DB,
		log:   p.Log.Named("meter.service"),
		repo:  p.Repo,
		genID: p.GenID,
	}
}

func (s *Service) Create(ctx context.Context, req meterdomain.CreateRequest) (*meterdomain.Response, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return nil, meterdomain.ErrInvalidOrganization
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		return nil, meterdomain.ErrInvalidCode
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, meterdomain.ErrInvalidName
	}

	aggregation := strings.TrimSpace(req.Aggregation)
	if aggregation == "" {
		return nil, meterdomain.ErrInvalidAggregation
	}

	unit := strings.TrimSpace(req.Unit)
	if unit == "" {
		return nil, meterdomain.ErrInvalidUnit
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	now := time.Now().UTC()
	m := &meterdomain.Meter{
		ID:          s.genID.Generate(),
		OrgID:       orgID,
		Code:        code,
		Name:        name,
		Aggregation: aggregation,
		Unit:        unit,
		Active:      active,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Insert(ctx, s.db, m); err != nil {
		return nil, err
	}

	return s.toResponse(m), nil
}

func (s *Service) List(ctx context.Context, req meterdomain.ListRequest) ([]meterdomain.Response, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return nil, meterdomain.ErrInvalidOrganization
	}

	filter := meterdomain.ListRequest{
		Name:    strings.TrimSpace(req.Name),
		Code:    strings.TrimSpace(req.Code),
		Active:  req.Active,
		SortBy:  strings.TrimSpace(req.SortBy),
		OrderBy: strings.TrimSpace(req.OrderBy),
	}

	items, err := s.repo.List(ctx, s.db, orgID, filter)
	if err != nil {
		return nil, err
	}

	resp := make([]meterdomain.Response, 0, len(items))
	for i := range items {
		resp = append(resp, *s.toResponse(&items[i]))
	}

	return resp, nil
}

func (s *Service) Update(ctx context.Context, req meterdomain.UpdateRequest) (*meterdomain.Response, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return nil, meterdomain.ErrInvalidOrganization
	}

	meterID, err := meterdomain.ParseID(strings.TrimSpace(req.ID))
	if err != nil {
		return nil, meterdomain.ErrInvalidID
	}

	item, err := s.repo.FindByID(ctx, s.db, orgID, meterID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, meterdomain.ErrMeterNotFound
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, meterdomain.ErrInvalidName
		}
		item.Name = name
	}

	if req.Aggregation != nil {
		aggregation := strings.TrimSpace(*req.Aggregation)
		if aggregation == "" {
			return nil, meterdomain.ErrInvalidAggregation
		}
		item.Aggregation = aggregation
	}

	if req.Unit != nil {
		unit := strings.TrimSpace(*req.Unit)
		if unit == "" {
			return nil, meterdomain.ErrInvalidUnit
		}
		item.Unit = unit
	}

	if req.Active != nil {
		item.Active = *req.Active
	}

	item.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, s.db, item); err != nil {
		return nil, err
	}

	return s.toResponse(item), nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return meterdomain.ErrInvalidOrganization
	}

	meterID, err := meterdomain.ParseID(strings.TrimSpace(id))
	if err != nil {
		return meterdomain.ErrInvalidID
	}

	item, err := s.repo.FindByID(ctx, s.db, orgID, meterID)
	if err != nil {
		return err
	}
	if item == nil {
		return meterdomain.ErrMeterNotFound
	}

	return s.repo.Delete(ctx, s.db, orgID, meterID)
}

func (s *Service) GetByCode(ctx context.Context, code string) (*meterdomain.Response, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return nil, meterdomain.ErrInvalidOrganization
	}

	item, err := s.repo.FindByCode(ctx, s.db, orgID, code)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, meterdomain.ErrMeterNotFound
	}

	return s.toResponse(item), nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*meterdomain.Response, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return nil, meterdomain.ErrInvalidOrganization
	}

	meterID, err := meterdomain.ParseID(id)
	if err != nil {
		return nil, meterdomain.ErrInvalidID
	}

	item, err := s.repo.FindByID(ctx, s.db, orgID, meterID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, meterdomain.ErrMeterNotFound
	}

	return s.toResponse(item), nil
}

func (s *Service) toResponse(m *meterdomain.Meter) *meterdomain.Response {
	return &meterdomain.Response{
		ID:             m.ID.String(),
		OrganizationID: m.OrgID.String(),
		Code:           m.Code,
		Name:           m.Name,
		Aggregation:    m.Aggregation,
		Unit:           m.Unit,
		Active:         m.Active,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}
