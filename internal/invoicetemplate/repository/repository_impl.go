package repository

import (
	"context"

	"github.com/bwmarrin/snowflake"
	templatedomain "github.com/smallbiznis/valora/internal/invoicetemplate/domain"
	"gorm.io/gorm"
)

type repo struct{}

func Provide() templatedomain.Repository {
	return &repo{}
}

func (r *repo) Insert(ctx context.Context, db *gorm.DB, tmpl *templatedomain.InvoiceTemplate) error {
	return db.WithContext(ctx).Exec(
		`INSERT INTO invoice_templates (
			id, org_id, name, is_default, locale, currency, header, footer, style, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tmpl.ID,
		tmpl.OrgID,
		tmpl.Name,
		tmpl.IsDefault,
		tmpl.Locale,
		tmpl.Currency,
		tmpl.Header,
		tmpl.Footer,
		tmpl.Style,
		tmpl.CreatedAt,
		tmpl.UpdatedAt,
	).Error
}

func (r *repo) Update(ctx context.Context, db *gorm.DB, tmpl *templatedomain.InvoiceTemplate) error {
	return db.WithContext(ctx).Exec(
		`UPDATE invoice_templates
		 SET name = ?, is_default = ?, locale = ?, currency = ?, header = ?, footer = ?, style = ?, updated_at = ?
		 WHERE org_id = ? AND id = ?`,
		tmpl.Name,
		tmpl.IsDefault,
		tmpl.Locale,
		tmpl.Currency,
		tmpl.Header,
		tmpl.Footer,
		tmpl.Style,
		tmpl.UpdatedAt,
		tmpl.OrgID,
		tmpl.ID,
	).Error
}

func (r *repo) FindByID(ctx context.Context, db *gorm.DB, orgID, id snowflake.ID) (*templatedomain.InvoiceTemplate, error) {
	var tmpl templatedomain.InvoiceTemplate
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, name, is_default, locale, currency, header, footer, style, created_at, updated_at
		 FROM invoice_templates
		 WHERE org_id = ? AND id = ?`,
		orgID,
		id,
	).Scan(&tmpl).Error
	if err != nil {
		return nil, err
	}
	if tmpl.ID == 0 {
		return nil, nil
	}
	return &tmpl, nil
}

func (r *repo) FindDefault(ctx context.Context, db *gorm.DB, orgID snowflake.ID) (*templatedomain.InvoiceTemplate, error) {
	var tmpl templatedomain.InvoiceTemplate
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, name, is_default, locale, currency, header, footer, style, created_at, updated_at
		 FROM invoice_templates
		 WHERE org_id = ? AND is_default = TRUE
		 LIMIT 1`,
		orgID,
	).Scan(&tmpl).Error
	if err != nil {
		return nil, err
	}
	if tmpl.ID == 0 {
		return nil, nil
	}
	return &tmpl, nil
}

func (r *repo) List(ctx context.Context, db *gorm.DB, orgID snowflake.ID, filter templatedomain.ListRequest) ([]templatedomain.InvoiceTemplate, error) {
	var items []templatedomain.InvoiceTemplate
	stmt := db.WithContext(ctx).Model(&templatedomain.InvoiceTemplate{}).Where("org_id = ?", orgID)

	if filter.Name != "" {
		stmt = stmt.Where("name = ?", filter.Name)
	}
	if filter.IsDefault != nil {
		stmt = stmt.Where("is_default = ?", *filter.IsDefault)
	}

	stmt = stmt.Order("created_at DESC")

	if err := stmt.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
