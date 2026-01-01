package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/snowflake"
	customerdomain "github.com/smallbiznis/valora/internal/customer/domain"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	"github.com/smallbiznis/valora/internal/invoice/render"
	templatedomain "github.com/smallbiznis/valora/internal/invoicetemplate/domain"
	"gorm.io/gorm"
)

func (s *Service) RenderInvoice(ctx context.Context, invoiceID string) (invoicedomain.RenderInvoiceResponse, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return invoicedomain.RenderInvoiceResponse{}, err
	}

	id, err := parseID(strings.TrimSpace(invoiceID))
	if err != nil {
		return invoicedomain.RenderInvoiceResponse{}, invoicedomain.ErrInvalidInvoiceID
	}

	item, err := s.invoicerepo.FindOne(ctx, &invoicedomain.Invoice{ID: id, OrgID: orgID})
	if err != nil {
		return invoicedomain.RenderInvoiceResponse{}, err
	}
	if item == nil {
		return invoicedomain.RenderInvoiceResponse{}, invoicedomain.ErrInvoiceNotFound
	}

	if item.Status != invoicedomain.InvoiceStatusDraft {
		if item.RenderedHTML == nil {
			return invoicedomain.RenderInvoiceResponse{}, invoicedomain.ErrInvoiceRenderMissing
		}
		resp := invoicedomain.RenderInvoiceResponse{
			RenderedHTML:   *item.RenderedHTML,
			RenderedPDFURL: item.RenderedPDFURL,
			IsSnapshot:     true,
		}
		if item.InvoiceTemplateID != nil {
			templateID := item.InvoiceTemplateID.String()
			resp.InvoiceTemplateID = &templateID
		}
		return resp, nil
	}

	html, tmpl, err := s.renderInvoiceHTML(ctx, s.db, item)
	if err != nil {
		return invoicedomain.RenderInvoiceResponse{}, err
	}

	resp := invoicedomain.RenderInvoiceResponse{
		RenderedHTML: html,
		IsSnapshot:   false,
	}
	if tmpl != nil {
		templateID := tmpl.ID.String()
		resp.InvoiceTemplateID = &templateID
	}
	return resp, nil
}

func (s *Service) renderInvoiceHTML(ctx context.Context, db *gorm.DB, invoice *invoicedomain.Invoice) (string, *templatedomain.InvoiceTemplate, error) {
	if invoice == nil {
		return "", nil, invoicedomain.ErrInvoiceNotFound
	}
	if s.renderer == nil {
		return "", nil, errors.New("renderer_not_configured")
	}

	tmpl, err := s.resolveTemplate(ctx, db, invoice.OrgID, invoice.InvoiceTemplateID)
	if err != nil {
		return "", nil, err
	}

	items, err := s.listInvoiceItems(ctx, db, invoice.OrgID, invoice.ID)
	if err != nil {
		return "", nil, err
	}

	customer, err := s.loadCustomer(ctx, db, invoice.OrgID, invoice.CustomerID)
	if err != nil {
		return "", nil, err
	}

	input := render.RenderInput{
		Template: buildTemplateView(tmpl),
		Invoice:  buildInvoiceView(invoice),
		Customer: buildCustomerView(customer),
		Items:    buildLineItemViews(items),
	}

	html, err := s.renderer.RenderHTML(input)
	if err != nil {
		return "", nil, err
	}
	return html, tmpl, nil
}

func (s *Service) resolveTemplate(ctx context.Context, db *gorm.DB, orgID snowflake.ID, templateID *snowflake.ID) (*templatedomain.InvoiceTemplate, error) {
	if s.templateRepo == nil {
		return nil, invoicedomain.ErrInvoiceTemplateNotFound
	}
	if templateID != nil && *templateID != 0 {
		item, err := s.templateRepo.FindByID(ctx, db, orgID, *templateID)
		if err != nil {
			return nil, err
		}
		if item == nil {
			return nil, invoicedomain.ErrInvoiceTemplateNotFound
		}
		return item, nil
	}

	item, err := s.templateRepo.FindDefault(ctx, db, orgID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, invoicedomain.ErrInvoiceTemplateNotFound
	}
	return item, nil
}

func (s *Service) listInvoiceItems(ctx context.Context, db *gorm.DB, orgID, invoiceID snowflake.ID) ([]invoicedomain.InvoiceItem, error) {
	var items []invoicedomain.InvoiceItem
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, invoice_id, rating_result_id, ledger_entry_line_id, subscription_item_id,
		        description, quantity, unit_price, amount, metadata, created_at
		 FROM invoice_items
		 WHERE org_id = ? AND invoice_id = ?
		 ORDER BY created_at ASC, id ASC`,
		orgID,
		invoiceID,
	).Scan(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

type customerRow struct {
	ID    snowflake.ID
	Name  string
	Email string
}

func (s *Service) loadCustomer(ctx context.Context, db *gorm.DB, orgID, customerID snowflake.ID) (*customerRow, error) {
	var customer customerRow
	err := db.WithContext(ctx).Raw(
		`SELECT id, name, email
		 FROM customers
		 WHERE org_id = ? AND id = ?`,
		orgID,
		customerID,
	).Scan(&customer).Error
	if err != nil {
		return nil, err
	}
	if customer.ID == 0 {
		return nil, customerdomain.ErrNotFound
	}
	return &customer, nil
}

func buildTemplateView(tmpl *templatedomain.InvoiceTemplate) render.TemplateView {
	if tmpl == nil {
		return render.TemplateView{}
	}
	return render.TemplateView{
		Name:         tmpl.Name,
		Locale:       tmpl.Locale,
		Currency:     tmpl.Currency,
		LogoURL:      templateValue(map[string]any(tmpl.Header), "logo_url"),
		CompanyName:  templateValue(map[string]any(tmpl.Header), "company_name"),
		FooterNotes:  templateValue(map[string]any(tmpl.Footer), "notes"),
		FooterLegal:  templateValue(map[string]any(tmpl.Footer), "legal"),
		PrimaryColor: templateValue(map[string]any(tmpl.Style), "primary_color"),
		FontFamily:   templateValue(map[string]any(tmpl.Style), "font"),
	}
}

func buildInvoiceView(invoice *invoicedomain.Invoice) render.InvoiceView {
	if invoice == nil {
		return render.InvoiceView{}
	}
	number := ""
	if invoice.InvoiceNumber != nil {
		number = fmtInvoiceNumber(*invoice.InvoiceNumber)
	}
	return render.InvoiceView{
		ID:             invoice.ID.String(),
		Number:         number,
		Status:         string(invoice.Status),
		IssuedAt:       invoice.IssuedAt,
		DueAt:          invoice.DueAt,
		PeriodStart:    invoice.PeriodStart,
		PeriodEnd:      invoice.PeriodEnd,
		SubtotalAmount: invoice.SubtotalAmount,
		Currency:       invoice.Currency,
	}
}

func buildCustomerView(customer *customerRow) render.CustomerView {
	if customer == nil {
		return render.CustomerView{}
	}
	return render.CustomerView{
		Name:  customer.Name,
		Email: customer.Email,
	}
}

func buildLineItemViews(items []invoicedomain.InvoiceItem) []render.LineItemView {
	views := make([]render.LineItemView, 0, len(items))
	for _, item := range items {
		views = append(views, render.LineItemView{
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			Amount:      item.Amount,
		})
	}
	return views
}

func templateValue(data map[string]any, key string) string {
	if data == nil || key == "" {
		return ""
	}
	value, ok := data[key]
	if !ok {
		return ""
	}
	s, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func fmtInvoiceNumber(value int64) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}
