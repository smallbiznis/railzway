package service

import (
	"context"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	templatedomain "github.com/smallbiznis/valora/internal/invoicetemplate/domain"
	"github.com/smallbiznis/valora/internal/orgcontext"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB       *gorm.DB
	Log      *zap.Logger
	GenID    *snowflake.Node
	Repo     templatedomain.Repository
	AuditSvc auditdomain.Service
}

type Service struct {
	db       *gorm.DB
	log      *zap.Logger
	genID    *snowflake.Node
	repo     templatedomain.Repository
	auditSvc auditdomain.Service
}

func NewService(p Params) templatedomain.Service {
	return &Service{
		db:       p.DB,
		log:      p.Log.Named("invoicetemplate.service"),
		genID:    p.GenID,
		repo:     p.Repo,
		auditSvc: p.AuditSvc,
	}
}

func (s *Service) Create(ctx context.Context, req templatedomain.CreateRequest) (*templatedomain.Response, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, templatedomain.ErrInvalidName
	}

	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		return nil, templatedomain.ErrInvalidCurrency
	}

	locale := strings.TrimSpace(req.Locale)
	if locale == "" {
		locale = "en"
	}

	now := time.Now().UTC()
	tmpl := &templatedomain.InvoiceTemplate{
		ID:        s.genID.Generate(),
		OrgID:     orgID,
		Name:      name,
		IsDefault: req.IsDefault,
		Locale:    locale,
		Currency:  currency,
		Header:    normalizeMap(req.Header),
		Footer:    normalizeMap(req.Footer),
		Style:     normalizeMap(req.Style),
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.IsDefault {
			if err := s.unsetDefault(ctx, tx, orgID, now); err != nil {
				return err
			}
		}
		return s.repo.Insert(ctx, tx, tmpl)
	})
	if err != nil {
		return nil, err
	}

	s.emitAudit(ctx, "invoice_template.created", tmpl, nil)
	return s.toResponse(tmpl), nil
}

func (s *Service) List(ctx context.Context, req templatedomain.ListRequest) ([]templatedomain.Response, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	filter := templatedomain.ListRequest{
		Name:      strings.TrimSpace(req.Name),
		IsDefault: req.IsDefault,
	}

	items, err := s.repo.List(ctx, s.db, orgID, filter)
	if err != nil {
		return nil, err
	}

	resp := make([]templatedomain.Response, 0, len(items))
	for i := range items {
		resp = append(resp, *s.toResponse(&items[i]))
	}
	return resp, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*templatedomain.Response, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	templateID, err := templatedomain.ParseID(id)
	if err != nil {
		return nil, templatedomain.ErrInvalidID
	}

	item, err := s.repo.FindByID(ctx, s.db, orgID, templateID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, templatedomain.ErrNotFound
	}

	return s.toResponse(item), nil
}

func (s *Service) Update(ctx context.Context, req templatedomain.UpdateRequest) (*templatedomain.Response, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	templateID, err := templatedomain.ParseID(req.ID)
	if err != nil {
		return nil, templatedomain.ErrInvalidID
	}

	item, err := s.repo.FindByID(ctx, s.db, orgID, templateID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, templatedomain.ErrNotFound
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, templatedomain.ErrInvalidName
		}
		item.Name = name
	}

	if req.Currency != nil {
		currency := strings.ToUpper(strings.TrimSpace(*req.Currency))
		if currency == "" {
			return nil, templatedomain.ErrInvalidCurrency
		}
		item.Currency = currency
	}

	if req.Locale != nil {
		locale := strings.TrimSpace(*req.Locale)
		if locale == "" {
			return nil, templatedomain.ErrInvalidLocale
		}
		item.Locale = locale
	}

	if req.Header != nil {
		item.Header = normalizeMap(req.Header)
	}

	if req.Footer != nil {
		item.Footer = normalizeMap(req.Footer)
	}

	if req.Style != nil {
		item.Style = normalizeMap(req.Style)
	}

	item.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, s.db, item); err != nil {
		return nil, err
	}

	s.emitAudit(ctx, "invoice_template.updated", item, nil)
	return s.toResponse(item), nil
}

func (s *Service) SetDefault(ctx context.Context, id string) (*templatedomain.Response, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	templateID, err := templatedomain.ParseID(id)
	if err != nil {
		return nil, templatedomain.ErrInvalidID
	}

	item, err := s.repo.FindByID(ctx, s.db, orgID, templateID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, templatedomain.ErrNotFound
	}

	now := time.Now().UTC()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.unsetDefault(ctx, tx, orgID, now); err != nil {
			return err
		}
		item.IsDefault = true
		item.UpdatedAt = now
		return s.repo.Update(ctx, tx, item)
	})
	if err != nil {
		return nil, err
	}

	s.emitAudit(ctx, "invoice_template.default_set", item, nil)
	return s.toResponse(item), nil
}

func (s *Service) unsetDefault(ctx context.Context, tx *gorm.DB, orgID snowflake.ID, now time.Time) error {
	return tx.WithContext(ctx).Exec(
		`UPDATE invoice_templates
		 SET is_default = FALSE, updated_at = ?
		 WHERE org_id = ? AND is_default = TRUE`,
		now,
		orgID,
	).Error
}

func (s *Service) emitAudit(ctx context.Context, action string, tmpl *templatedomain.InvoiceTemplate, extra map[string]any) {
	if s.auditSvc == nil || tmpl == nil {
		return
	}
	metadata := map[string]any{
		"name":       tmpl.Name,
		"is_default": tmpl.IsDefault,
		"locale":     tmpl.Locale,
		"currency":   tmpl.Currency,
	}
	for key, value := range extra {
		if key == "" {
			continue
		}
		metadata[key] = value
	}

	targetID := tmpl.ID.String()
	orgID := tmpl.OrgID
	_ = s.auditSvc.AuditLog(ctx, &orgID, "", nil, action, "invoice_template", &targetID, metadata)
}

func (s *Service) orgIDFromContext(ctx context.Context) (snowflake.ID, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return 0, templatedomain.ErrInvalidOrganization
	}
	return snowflake.ID(orgID), nil
}

func (s *Service) toResponse(tmpl *templatedomain.InvoiceTemplate) *templatedomain.Response {
	if tmpl == nil {
		return nil
	}
	return &templatedomain.Response{
		ID:        tmpl.ID.String(),
		OrgID:     tmpl.OrgID.String(),
		Name:      tmpl.Name,
		IsDefault: tmpl.IsDefault,
		Locale:    tmpl.Locale,
		Currency:  tmpl.Currency,
		Header:    map[string]any(tmpl.Header),
		Footer:    map[string]any(tmpl.Footer),
		Style:     map[string]any(tmpl.Style),
		CreatedAt: tmpl.CreatedAt,
		UpdatedAt: tmpl.UpdatedAt,
	}
}

func normalizeMap(input map[string]any) datatypes.JSONMap {
	if input == nil {
		return datatypes.JSONMap{}
	}
	output := datatypes.JSONMap{}
	for key, value := range input {
		if key == "" {
			continue
		}
		output[key] = value
	}
	return output
}
