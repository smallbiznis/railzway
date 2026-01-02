package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	billingcycledomain "github.com/smallbiznis/valora/internal/billingcycle/domain"
	"github.com/smallbiznis/valora/internal/events"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	"github.com/smallbiznis/valora/internal/invoice/render"
	templatedomain "github.com/smallbiznis/valora/internal/invoicetemplate/domain"
	ledgerdomain "github.com/smallbiznis/valora/internal/ledger/domain"
	"github.com/smallbiznis/valora/internal/orgcontext"
	"github.com/smallbiznis/valora/pkg/db/option"
	"github.com/smallbiznis/valora/pkg/repository"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ServiceParam struct {
	fx.In

	DB           *gorm.DB
	Log          *zap.Logger
	GenID        *snowflake.Node
	AuditSvc     auditdomain.Service
	TemplateRepo templatedomain.Repository
	Renderer     render.Renderer
	Outbox       *events.Outbox `optional:"true"`
}

type Service struct {
	db  *gorm.DB
	log *zap.Logger

	genID        *snowflake.Node
	invoicerepo  repository.Repository[invoicedomain.Invoice]
	auditSvc     auditdomain.Service
	templateRepo templatedomain.Repository
	renderer     render.Renderer
	outbox       *events.Outbox
}

func NewService(p ServiceParam) invoicedomain.Service {
	return &Service{
		db:    p.DB,
		log:   p.Log.Named("invoice.service"),
		genID: p.GenID,

		invoicerepo:  repository.ProvideStore[invoicedomain.Invoice](p.DB),
		auditSvc:     p.AuditSvc,
		templateRepo: p.TemplateRepo,
		renderer:     p.Renderer,
		outbox:       p.Outbox,
	}
}

func (s *Service) List(ctx context.Context, req invoicedomain.ListInvoiceRequest) (invoicedomain.ListInvoiceResponse, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return invoicedomain.ListInvoiceResponse{}, err
	}

	filter := &invoicedomain.Invoice{OrgID: orgID}
	if req.Status != nil {
		filter.Status = *req.Status
	}
	if req.CustomerID != nil {
		filter.CustomerID = *req.CustomerID
	}
	if req.InvoiceNumber != nil {
		filter.InvoiceNumber = req.InvoiceNumber
	}

	options := []option.QueryOption{
		option.WithSortBy(option.QuerySortBy{Allow: map[string]bool{"created_at": true}}),
	}
	if req.CreatedFrom != nil {
		options = append(options, option.ApplyOperator(option.Condition{
			Field:    "created_at",
			Operator: option.GTE,
			Value:    *req.CreatedFrom,
		}))
	}
	if req.CreatedTo != nil {
		options = append(options, option.ApplyOperator(option.Condition{
			Field:    "created_at",
			Operator: option.LTE,
			Value:    *req.CreatedTo,
		}))
	}
	if req.DueFrom != nil {
		options = append(options, option.ApplyOperator(option.Condition{
			Field:    "due_at",
			Operator: option.GTE,
			Value:    *req.DueFrom,
		}))
	}
	if req.DueTo != nil {
		options = append(options, option.ApplyOperator(option.Condition{
			Field:    "due_at",
			Operator: option.LTE,
			Value:    *req.DueTo,
		}))
	}
	if req.FinalizedFrom != nil {
		options = append(options, option.ApplyOperator(option.Condition{
			Field:    "finalized_at",
			Operator: option.GTE,
			Value:    *req.FinalizedFrom,
		}))
	}
	if req.FinalizedTo != nil {
		options = append(options, option.ApplyOperator(option.Condition{
			Field:    "finalized_at",
			Operator: option.LTE,
			Value:    *req.FinalizedTo,
		}))
	}
	if req.TotalMin != nil {
		options = append(options, option.ApplyOperator(option.Condition{
			Field:    "subtotal_amount",
			Operator: option.GTE,
			Value:    *req.TotalMin,
		}))
	}
	if req.TotalMax != nil {
		options = append(options, option.ApplyOperator(option.Condition{
			Field:    "subtotal_amount",
			Operator: option.LTE,
			Value:    *req.TotalMax,
		}))
	}

	items, err := s.invoicerepo.Find(ctx, filter, options...)
	if err != nil {
		return invoicedomain.ListInvoiceResponse{}, err
	}

	invoices := make([]invoicedomain.Invoice, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		invoices = append(invoices, *item)
	}

	return invoicedomain.ListInvoiceResponse{Invoices: invoices}, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (invoicedomain.Invoice, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return invoicedomain.Invoice{}, err
	}

	invoiceID, err := snowflake.ParseString(strings.TrimSpace(id))
	if err != nil {
		return invoicedomain.Invoice{}, err
	}

	item, err := s.invoicerepo.FindOne(ctx, &invoicedomain.Invoice{ID: invoiceID, OrgID: orgID})
	if err != nil {
		return invoicedomain.Invoice{}, err
	}
	if item == nil {
		return invoicedomain.Invoice{}, gorm.ErrRecordNotFound
	}

	return *item, nil
}

func (s *Service) GenerateInvoice(ctx context.Context, billingCycleID string) error {
	cycleID, err := parseID(strings.TrimSpace(billingCycleID))
	if err != nil {
		return invoicedomain.ErrInvalidBillingCycle
	}

	var createdInvoice *invoicedomain.Invoice
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		cycle, err := s.loadBillingCycleForUpdate(ctx, tx, cycleID)
		if err != nil {
			return err
		}
		if cycle == nil {
			return invoicedomain.ErrBillingCycleNotFound
		}
		if cycle.Status != billingcycledomain.BillingCycleStatusClosed {
			return invoicedomain.ErrBillingCycleNotClosed
		}
		if !cycle.PeriodEnd.After(cycle.PeriodStart) {
			return invoicedomain.ErrInvalidBillingCycle
		}

		existingID, err := s.findInvoiceByBillingCycle(ctx, tx, cycle.ID)
		if err != nil {
			return err
		}
		if existingID != 0 {
			return nil
		}

		if err := s.lockOrganization(ctx, tx, cycle.OrgID); err != nil {
			return err
		}

		subscription, err := s.loadSubscription(ctx, tx, cycle.OrgID, cycle.SubscriptionID)
		if err != nil {
			return err
		}
		if subscription == nil || subscription.CustomerID == 0 {
			return invoicedomain.ErrInvalidBillingCycle
		}

		entry, err := s.loadLedgerEntryForCycle(ctx, tx, cycle.OrgID, cycle.ID)
		if err != nil {
			return err
		}
		if entry == nil {
			return invoicedomain.ErrMissingLedgerEntry
		}

		var subtotal int64
		lines, err := s.listLedgerEntryLines(ctx, tx, entry.ID)
		if err != nil {
			return err
		}
		if len(lines) == 0 {
			return invoicedomain.ErrMissingLedgerEntry
		}

		creditLines := make([]ledgerEntryLineRow, 0, len(lines))
		for _, line := range lines {
			if line.Direction != ledgerdomain.LedgerEntryDirectionCredit {
				continue
			}
			subtotal += line.Amount
			creditLines = append(creditLines, line)
		}
		if len(creditLines) == 0 {
			return invoicedomain.ErrMissingLedgerEntry
		}

		invoiceNumber, err := s.nextInvoiceNumber(ctx, tx, cycle.OrgID)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		invoiceID := s.genID.Generate()
		invoice := invoicedomain.Invoice{
			ID:             invoiceID,
			OrgID:          cycle.OrgID,
			InvoiceNumber:  &invoiceNumber,
			BillingCycleID: cycle.ID,
			SubscriptionID: cycle.SubscriptionID,
			CustomerID:     subscription.CustomerID,
			Status:         invoicedomain.InvoiceStatusDraft,
			SubtotalAmount: subtotal,
			Currency:       entry.Currency,
			PeriodStart:    &cycle.PeriodStart,
			PeriodEnd:      &cycle.PeriodEnd,
			IssuedAt:       &now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		inserted, err := s.insertInvoice(ctx, tx, invoice)
		if err != nil {
			return err
		}
		if !inserted {
			return nil
		}
		createdInvoice = &invoice

		for _, line := range creditLines {
			description := strings.TrimSpace(line.AccountName)
			if description == "" {
				description = strings.TrimSpace(line.AccountCode)
			}
			if description == "" {
				description = "Ledger entry"
			}
			if err := s.insertInvoiceItem(ctx, tx, invoicedomain.InvoiceItem{
				ID:                s.genID.Generate(),
				OrgID:             cycle.OrgID,
				InvoiceID:         invoiceID,
				LedgerEntryLineID: &line.ID,
				Description:       description,
				Quantity:          1,
				UnitPrice:         line.Amount,
				Amount:            line.Amount,
				CreatedAt:         now,
			}); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}
	if createdInvoice != nil {
		s.emitAudit(ctx, "invoice.generate", createdInvoice, nil)
	}
	return nil
}

func (s *Service) FinalizeInvoice(ctx context.Context, invoiceID string) error {
	id, err := parseID(strings.TrimSpace(invoiceID))
	if err != nil {
		return invoicedomain.ErrInvalidInvoiceID
	}

	var finalizedInvoice *invoicedomain.Invoice
	var renderedChecksum string
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		invoice, err := s.loadInvoiceForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if invoice == nil {
			return invoicedomain.ErrInvoiceNotFound
		}
		if invoice.Status != invoicedomain.InvoiceStatusDraft {
			return invoicedomain.ErrInvoiceNotDraft
		}

		// Snapshot rendered output at finalization so future template edits never change history.
		renderedHTML, tmpl, err := s.renderInvoiceHTML(ctx, tx, invoice)
		if err != nil {
			return err
		}
		if tmpl != nil {
			invoice.InvoiceTemplateID = &tmpl.ID
		}
		invoice.RenderedHTML = &renderedHTML
		checksum := sha256.Sum256([]byte(renderedHTML))
		renderedChecksum = hex.EncodeToString(checksum[:])

		now := time.Now().UTC()
		if err := tx.WithContext(ctx).Exec(
			`UPDATE invoices
			 SET status = ?, finalized_at = ?, invoice_template_id = ?, rendered_html = ?, rendered_pdf_url = ?, updated_at = ?
			 WHERE id = ?`,
			invoicedomain.InvoiceStatusFinalized,
			now,
			invoice.InvoiceTemplateID,
			invoice.RenderedHTML,
			invoice.RenderedPDFURL,
			now,
			id,
		).Error; err != nil {
			return err
		}
		invoice.FinalizedAt = &now
		finalizedInvoice = invoice

		if s.outbox != nil {
			if err := s.outbox.PublishTx(ctx, tx, events.Event{
				OrgID: invoice.OrgID,
				Type:  events.EventInvoiceFinalized,
				Payload: map[string]any{
					"invoice_id":       invoice.ID.String(),
					"billing_cycle_id": invoice.BillingCycleID.String(),
				},
				DedupeKey: "invoice_finalized:" + invoice.ID.String(),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if finalizedInvoice != nil {
		metadata := map[string]any{
			"previous_status": string(invoicedomain.InvoiceStatusDraft),
		}
		if renderedChecksum != "" {
			metadata["rendered_checksum"] = renderedChecksum
		}
		if finalizedInvoice.InvoiceTemplateID != nil {
			metadata["invoice_template_id"] = finalizedInvoice.InvoiceTemplateID.String()
		}
		s.emitAudit(ctx, "invoice.finalize", finalizedInvoice, metadata)
	}
	return nil
}

func (s *Service) VoidInvoice(ctx context.Context, invoiceID string, reason string) error {
	id, err := parseID(strings.TrimSpace(invoiceID))
	if err != nil {
		return invoicedomain.ErrInvalidInvoiceID
	}

	var voidedInvoice *invoicedomain.Invoice
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		invoice, err := s.loadInvoiceForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if invoice == nil {
			return invoicedomain.ErrInvoiceNotFound
		}
		if invoice.Status != invoicedomain.InvoiceStatusFinalized {
			return invoicedomain.ErrInvoiceNotFinalized
		}

		now := time.Now().UTC()
		if err := tx.WithContext(ctx).Exec(
			`UPDATE invoices
			 SET status = ?, voided_at = ?, updated_at = ?
			 WHERE id = ?`,
			invoicedomain.InvoiceStatusVoid,
			now,
			now,
			id,
		).Error; err != nil {
			return err
		}
		voidedInvoice = invoice

		if s.outbox != nil {
			if err := s.outbox.PublishTx(ctx, tx, events.Event{
				OrgID: invoice.OrgID,
				Type:  events.EventInvoiceVoided,
				Payload: map[string]any{
					"invoice_id":       invoice.ID.String(),
					"billing_cycle_id": invoice.BillingCycleID.String(),
				},
				DedupeKey: "invoice_voided:" + invoice.ID.String(),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if voidedInvoice != nil {
		metadata := map[string]any{
			"previous_status": string(invoicedomain.InvoiceStatusFinalized),
		}
		reason = strings.TrimSpace(reason)
		if reason != "" {
			metadata["reason"] = reason
		}
		s.emitAudit(ctx, "invoice.void", voidedInvoice, metadata)
	}
	return nil
}

func (s *Service) emitAudit(ctx context.Context, action string, invoice *invoicedomain.Invoice, extra map[string]any) {
	if s.auditSvc == nil || invoice == nil {
		return
	}
	metadata := map[string]any{
		"billing_cycle_id": invoice.BillingCycleID.String(),
		"subscription_id":  invoice.SubscriptionID.String(),
		"customer_id":      invoice.CustomerID.String(),
		"currency":         invoice.Currency,
		"subtotal_amount":  invoice.SubtotalAmount,
	}
	if invoice.InvoiceNumber != nil {
		metadata["invoice_number"] = *invoice.InvoiceNumber
	}
	if invoice.InvoiceTemplateID != nil {
		metadata["invoice_template_id"] = invoice.InvoiceTemplateID.String()
	}
	if invoice.PeriodStart != nil {
		metadata["period_start"] = invoice.PeriodStart.Format(time.RFC3339)
	}
	if invoice.PeriodEnd != nil {
		metadata["period_end"] = invoice.PeriodEnd.Format(time.RFC3339)
	}
	for key, value := range extra {
		if key == "" {
			continue
		}
		metadata[key] = value
	}

	targetID := invoice.ID.String()
	orgID := invoice.OrgID
	_ = s.auditSvc.AuditLog(ctx, &orgID, "", nil, action, "invoice", &targetID, metadata)
}

func (s *Service) orgIDFromContext(ctx context.Context) (snowflake.ID, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return 0, invoicedomain.ErrInvalidOrganization
	}
	return snowflake.ID(orgID), nil
}

type billingCycleRow struct {
	ID             snowflake.ID
	OrgID          snowflake.ID
	SubscriptionID snowflake.ID
	PeriodStart    time.Time
	PeriodEnd      time.Time
	Status         billingcycledomain.BillingCycleStatus
}

type subscriptionRow struct {
	ID         snowflake.ID
	OrgID      snowflake.ID
	CustomerID snowflake.ID
}

type ledgerEntryRow struct {
	ID         snowflake.ID
	OrgID      snowflake.ID
	Currency   string
	OccurredAt time.Time
}

type ledgerEntryLineRow struct {
	ID          snowflake.ID
	AccountID   snowflake.ID
	Direction   ledgerdomain.LedgerEntryDirection
	Amount      int64
	AccountCode string `gorm:"column:account_code"`
	AccountName string `gorm:"column:account_name"`
}

func (s *Service) loadBillingCycleForUpdate(ctx context.Context, tx *gorm.DB, id snowflake.ID) (*billingCycleRow, error) {
	var cycle billingCycleRow
	err := tx.WithContext(ctx).Raw(
		`SELECT id, org_id, subscription_id, period_start, period_end, status
		 FROM billing_cycles
		 WHERE id = ?
		 FOR UPDATE`,
		id,
	).Scan(&cycle).Error
	if err != nil {
		return nil, err
	}
	if cycle.ID == 0 {
		return nil, nil
	}
	return &cycle, nil
}

func (s *Service) loadSubscription(ctx context.Context, tx *gorm.DB, orgID, subscriptionID snowflake.ID) (*subscriptionRow, error) {
	var sub subscriptionRow
	err := tx.WithContext(ctx).Raw(
		`SELECT id, org_id, customer_id
		 FROM subscriptions
		 WHERE org_id = ? AND id = ?`,
		orgID,
		subscriptionID,
	).Scan(&sub).Error
	if err != nil {
		return nil, err
	}
	if sub.ID == 0 {
		return nil, nil
	}
	return &sub, nil
}

func (s *Service) findInvoiceByBillingCycle(ctx context.Context, tx *gorm.DB, billingCycleID snowflake.ID) (snowflake.ID, error) {
	var invoiceID snowflake.ID
	err := tx.WithContext(ctx).Raw(
		`SELECT id
		 FROM invoices
		 WHERE billing_cycle_id = ?
		 LIMIT 1`,
		billingCycleID,
	).Scan(&invoiceID).Error
	if err != nil {
		return 0, err
	}
	return invoiceID, nil
}

func (s *Service) loadLedgerEntryForCycle(ctx context.Context, tx *gorm.DB, orgID, billingCycleID snowflake.ID) (*ledgerEntryRow, error) {
	var entry ledgerEntryRow
	err := tx.WithContext(ctx).Raw(
		`SELECT id, org_id, currency, occurred_at
		 FROM ledger_entries
		 WHERE org_id = ? AND source_type = ? AND source_id = ?
		 LIMIT 1`,
		orgID,
		ledgerdomain.SourceTypeBillingCycle,
		billingCycleID,
	).Scan(&entry).Error
	if err != nil {
		return nil, err
	}
	if entry.ID == 0 {
		return nil, nil
	}
	return &entry, nil
}

func (s *Service) listLedgerEntryLines(ctx context.Context, tx *gorm.DB, ledgerEntryID snowflake.ID) ([]ledgerEntryLineRow, error) {
	var lines []ledgerEntryLineRow
	err := tx.WithContext(ctx).Raw(
		`SELECT l.id, l.account_id, l.direction, l.amount,
		        a.code AS account_code, a.name AS account_name
		 FROM ledger_entry_lines l
		 JOIN ledger_accounts a ON a.id = l.account_id
		 WHERE l.ledger_entry_id = ?
		 ORDER BY l.id ASC`,
		ledgerEntryID,
	).Scan(&lines).Error
	if err != nil {
		return nil, err
	}
	return lines, nil
}

func (s *Service) lockOrganization(ctx context.Context, tx *gorm.DB, orgID snowflake.ID) error {
	var id snowflake.ID
	err := tx.WithContext(ctx).Raw(
		`SELECT id
		 FROM organizations
		 WHERE id = ?
		 FOR UPDATE`,
		orgID,
	).Scan(&id).Error
	if err != nil {
		return err
	}
	if id == 0 {
		return invoicedomain.ErrInvalidOrganization
	}
	return nil
}

func (s *Service) nextInvoiceNumber(ctx context.Context, tx *gorm.DB, orgID snowflake.ID) (int64, error) {
	var next int64
	err := tx.WithContext(ctx).Raw(
		`SELECT COALESCE(MAX(invoice_number), 0) + 1
		 FROM invoices
		 WHERE org_id = ?`,
		orgID,
	).Scan(&next).Error
	if err != nil {
		return 0, err
	}
	return next, nil
}

func (s *Service) insertInvoice(ctx context.Context, tx *gorm.DB, invoice invoicedomain.Invoice) (bool, error) {
	result := tx.WithContext(ctx).Exec(
		`INSERT INTO invoices (
			id, org_id, invoice_number, billing_cycle_id, subscription_id, customer_id,
			invoice_template_id, status, subtotal_amount, currency, period_start, period_end,
			issued_at, due_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (billing_cycle_id) DO NOTHING`,
		invoice.ID,
		invoice.OrgID,
		invoice.InvoiceNumber,
		invoice.BillingCycleID,
		invoice.SubscriptionID,
		invoice.CustomerID,
		invoice.InvoiceTemplateID,
		invoice.Status,
		invoice.SubtotalAmount,
		invoice.Currency,
		invoice.PeriodStart,
		invoice.PeriodEnd,
		invoice.IssuedAt,
		invoice.DueAt,
		invoice.CreatedAt,
		invoice.UpdatedAt,
	)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	return true, nil
}

func (s *Service) insertInvoiceItem(ctx context.Context, tx *gorm.DB, item invoicedomain.InvoiceItem) error {
	return tx.WithContext(ctx).Exec(
		`INSERT INTO invoice_items (
			id, org_id, invoice_id, rating_result_id, ledger_entry_line_id,
			description, quantity, unit_price, amount, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID,
		item.OrgID,
		item.InvoiceID,
		item.RatingResultID,
		item.LedgerEntryLineID,
		item.Description,
		item.Quantity,
		item.UnitPrice,
		item.Amount,
		item.CreatedAt,
	).Error
}

func (s *Service) loadInvoiceForUpdate(ctx context.Context, tx *gorm.DB, id snowflake.ID) (*invoicedomain.Invoice, error) {
	var invoice invoicedomain.Invoice
	err := tx.WithContext(ctx).Raw(
		`SELECT id, org_id, invoice_number, billing_cycle_id, subscription_id, customer_id,
		        invoice_template_id, status, subtotal_amount, currency, period_start, period_end,
		        issued_at, due_at, finalized_at, voided_at, rendered_html, rendered_pdf_url,
		        created_at, updated_at
		 FROM invoices
		 WHERE id = ?
		 FOR UPDATE`,
		id,
	).Scan(&invoice).Error
	if err != nil {
		return nil, err
	}
	if invoice.ID == 0 {
		return nil, nil
	}
	return &invoice, nil
}

func parseID(raw string) (snowflake.ID, error) {
	return snowflake.ParseString(raw)
}
