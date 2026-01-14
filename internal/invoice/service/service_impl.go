package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	auditdomain "github.com/smallbiznis/railzway/internal/audit/domain"
	billingcycledomain "github.com/smallbiznis/railzway/internal/billingcycle/domain"
	"github.com/smallbiznis/railzway/internal/events"
	invoicedomain "github.com/smallbiznis/railzway/internal/invoice/domain"
	invoiceformat "github.com/smallbiznis/railzway/internal/invoice/format"
	"github.com/smallbiznis/railzway/internal/invoice/render"
	templatedomain "github.com/smallbiznis/railzway/internal/invoicetemplate/domain"
	ledgerdomain "github.com/smallbiznis/railzway/internal/ledger/domain"
	meterdomain "github.com/smallbiznis/railzway/internal/meter/domain"
	"github.com/smallbiznis/railzway/internal/orgcontext"
	pricedomain "github.com/smallbiznis/railzway/internal/price/domain"
	priceamountdomain "github.com/smallbiznis/railzway/internal/priceamount/domain"
	"github.com/smallbiznis/railzway/internal/providers/email"
	"github.com/smallbiznis/railzway/internal/providers/pdf"
	publicinvoicedomain "github.com/smallbiznis/railzway/internal/publicinvoice/domain"
	ratingdomain "github.com/smallbiznis/railzway/internal/rating/domain"
	taxdomain "github.com/smallbiznis/railzway/internal/tax/domain"
	taxservice "github.com/smallbiznis/railzway/internal/tax/service"
	"github.com/smallbiznis/railzway/pkg/db/option"
	"github.com/smallbiznis/railzway/pkg/repository"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type invoiceItemPart struct {
	// Type could be: "usage" | "subscription" | "credit"
	Type invoicedomain.InvoiceItemLineType

	// Interval
	Interval *pricedomain.BillingInterval

	DisplayName string  // e.g. "Actions", "Active Storage", "Essentials Plan", "Sign Up Credit"
	Quantity    float64 // for UI table qty column (Temporal uses 1 line item but mentions total qty in desc)
	UnitLabel   string  // "unit", "GB-hour", etc (optional)

	// For showing "Rate: $X / unit"
	RateAmount int64

	// Actual item pricing fields
	UnitPriceAmount int64 // for table "Unit price" column
	Amount          int64 // for table "Amount" column (can be negative for credit)

	Currency string

	// optional: for UI/PDF drilldown
	// MeterCode string
	// PriceID   int64
	// Metadata  map[string]any
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

type ServiceParam struct {
	fx.In

	DB             *gorm.DB
	Log            *zap.Logger
	GenID          *snowflake.Node
	AuditSvc       auditdomain.Service
	TemplateRepo   templatedomain.Repository
	Renderer       render.Renderer
	PublicTokenSvc publicinvoicedomain.PublicInvoiceTokenService
	TaxResolver    taxdomain.TaxResolver
	LedgerSvc      ledgerdomain.Service
	Outbox         *events.Outbox `optional:"true"`
	EmailProvider  email.Provider
	PDFProvider    pdf.Provider
}

type Service struct {
	db  *gorm.DB
	log *zap.Logger

	genID          *snowflake.Node
	invoicerepo    repository.Repository[invoicedomain.Invoice]
	auditSvc       auditdomain.Service
	templateRepo   templatedomain.Repository
	renderer       render.Renderer
	publicTokenSvc publicinvoicedomain.PublicInvoiceTokenService
	taxResolver    taxdomain.TaxResolver
	ledgerSvc      ledgerdomain.Service
	outbox         *events.Outbox
	emailProvider  email.Provider
	pdfProvider    pdf.Provider
}

func NewService(p ServiceParam) invoicedomain.Service {
	return &Service{
		db:    p.DB,
		log:   p.Log.Named("invoice.service"),
		genID: p.GenID,

		invoicerepo:    repository.ProvideStore[invoicedomain.Invoice](p.DB),
		auditSvc:       p.AuditSvc,
		templateRepo:   p.TemplateRepo,
		renderer:       p.Renderer,
		publicTokenSvc: p.PublicTokenSvc,
		taxResolver:    p.TaxResolver,
		ledgerSvc:      p.LedgerSvc,
		outbox:         p.Outbox,
		emailProvider:  p.EmailProvider,
		pdfProvider:    p.PDFProvider,
	}
}

func (s *Service) List(ctx context.Context, req invoicedomain.ListInvoiceRequest) (invoicedomain.ListInvoiceResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return invoicedomain.ListInvoiceResponse{}, invoicedomain.ErrInvalidOrganization
	}

	filter := &invoicedomain.Invoice{OrgID: orgID}
	if req.Status != nil {
		filter.Status = *req.Status
	}
	if req.CustomerID != nil {
		filter.CustomerID = *req.CustomerID
	}
	if req.InvoiceNumber != nil {
		filter.InvoiceNumber = *req.InvoiceNumber
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
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return invoicedomain.Invoice{}, invoicedomain.ErrInvalidOrganization
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

func (s *Service) GenerateInvoice(ctx context.Context, billingCycleID string) (*invoicedomain.Invoice, error) {
	cycleID, err := parseID(strings.TrimSpace(billingCycleID))
	if err != nil {
		return nil, invoicedomain.ErrInvalidBillingCycle
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

		rating, err := s.loadRating(ctx, tx, cycle.ID)
		if err != nil {
			return err
		}
		if rating == nil {
			return invoicedomain.ErrMissingRatingResults
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
		displayNumber, err := invoiceformat.FormatInvoiceNumber(invoiceformat.DefaultInvoiceNumberTemplate, now, invoiceNumber)
		if err != nil {
			return err
		}
		invoiceID := s.genID.Generate()
		invoice := invoicedomain.Invoice{
			ID:             invoiceID,
			OrgID:          cycle.OrgID,
			InvoiceSeq:     &invoiceNumber,
			InvoiceNumber:  displayNumber,
			BillingCycleID: cycle.ID,
			SubscriptionID: cycle.SubscriptionID,
			CustomerID:     subscription.CustomerID,
			Status:         invoicedomain.InvoiceStatusDraft,
			SubtotalAmount: subtotal,
			Currency:       entry.Currency,
			PeriodStart:    &cycle.PeriodStart,
			PeriodEnd:      &cycle.PeriodEnd,
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

		if err := s.listInvoiceItemPartsFromRating(ctx, tx, *cycle, invoiceID); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if createdInvoice != nil {
		s.emitAudit(ctx, "invoice.generate", createdInvoice, nil)
	}

	return createdInvoice, nil
}

func (s *Service) listInvoiceItemPartsFromRating(
	ctx context.Context,
	tx *gorm.DB,
	cycle billingCycleRow,
	invoiceID snowflake.ID,
) error {

	// 1. Load active entitlements for the cycle
	entitlements, err := s.listEntitlementsForCycle(ctx, tx, cycle.OrgID, cycle.SubscriptionID, cycle.PeriodStart, cycle.PeriodEnd)
	if err != nil {
		return err
	}
	// Index entitlements by FeatureCode for joining
	entitlementMap := make(map[string]invoicedomain.SubscriptionEntitlement)
	// Also index by MeterID for fallback if FeatureCode is missing in old data (though strictly required by prompt)
	// Prompt says: "Joining rating_results to entitlements by: ... feature_code"
	for _, e := range entitlements {
		entitlementMap[e.FeatureCode] = e
	}

	var rows []struct {
		ID          snowflake.ID
		OrgID       snowflake.ID
		MeterID     snowflake.ID
		PriceID     snowflake.ID
		FeatureCode string
		Quantity    float64
		UnitPrice   int64
		Amount      int64
		Currency    string
		Source      string
	}

	if err := tx.WithContext(ctx).
		Table("rating_results").
		Select(`
		id,
		org_id,
		meter_id,
		price_id,
		feature_code,
		quantity,
		unit_price,
		amount,
		currency,
		source
	`).
		Where("billing_cycle_id = ?", cycle.ID).
		Scan(&rows).Error; err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, r := range rows {
		// Strict Join Rule: WAS Must match entitlement
		// Now: Optional.
		ent, ok := entitlementMap[r.FeatureCode]
		description := "Usage"
		if ok {
			description = ent.FeatureName
		} else if r.MeterID == 0 {
			description = "Subscription"
		}

		invoiceItem := invoicedomain.InvoiceItem{
			ID:             s.genID.Generate(),
			OrgID:          r.OrgID,
			InvoiceID:      invoiceID,
			RatingResultID: &r.ID,
			Quantity:       float64(r.Quantity),
			UnitPrice:      r.UnitPrice,
			Amount:         r.Amount,
			Description:    description, // Use snapshot data or fallback
			LineType:       invoicedomain.InvoiceItemLineTypeUsage,
			CreatedAt:      now,
		}

		// Refine Line Type based on Entitlement or Rating?
		// If MeterID is nil, likely Subscription/Flat
		if r.MeterID == 0 {
			invoiceItem.LineType = invoicedomain.InvoiceItemLineTypeSubscription
		}

		// Enrich description (e.g. usage dates, rate)
		part := invoiceItemPart{
			Type:        invoiceItem.LineType,
			DisplayName: invoiceItem.Description,
			Quantity:    r.Quantity,
			RateAmount:  r.UnitPrice,
			Currency:    r.Currency,
			UnitLabel:   "unit", // Default, could be enriched from entitlement metadata if available
		}
		invoiceItem.Description = s.formatInvoiceItemDescription(part, cycle)

		if err := s.insertInvoiceItem(ctx, tx, invoiceItem); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) listEntitlementsForCycle(
	ctx context.Context,
	tx *gorm.DB,
	orgID, subscriptionID snowflake.ID,
	start, end time.Time,
) ([]invoicedomain.SubscriptionEntitlement, error) {
	// Find entitlements that overlap with the cycle
	// EffectiveFrom < PeriodEnd AND (EffectiveTo IS NULL OR EffectiveTo > PeriodStart)
	var ents []invoicedomain.SubscriptionEntitlement
	err := tx.WithContext(ctx).Raw(`
		SELECT * FROM subscription_entitlements
		WHERE org_id = ? AND subscription_id = ?
		AND effective_from < ?
		AND (effective_to IS NULL OR effective_to > ?)
	`, orgID, subscriptionID, end, start).Scan(&ents).Error
	if err != nil {
		return nil, err
	}
	return ents, nil
}

func (s *Service) formatSnapshotItemDescription(
	baseName string,
	quantity float64,
	amount int64,
	currency string,
	cycle billingCycleRow,
	lineType invoicedomain.InvoiceItemLineType,
) string {
	// Snapshot logic: No looking up Price table.
	base := strings.TrimSpace(baseName)

	switch lineType {
	case invoicedomain.InvoiceItemLineTypeUsage:
		parts := make([]string, 0, 1)
		if quantity > 0 {
			parts = append(parts, fmt.Sprintf("Qty: %g", quantity))
		}
		// Calculate implied rate since we don't have unit_label from Price table anymore
		// unit_price = amount / quantity (roughly)
		// Or just show total.
		if len(parts) > 0 {
			base = fmt.Sprintf("%s (%s)", base, strings.Join(parts, ", "))
		}
	}

	period := fmt.Sprintf(
		"%s – %s",
		cycle.PeriodStart.Format("Jan 2, 2006"),
		cycle.PeriodEnd.Format("Jan 2, 2006"),
	)
	return base + "\n" + period
}

func (s *Service) formatInvoiceItemDescription(
	p invoiceItemPart,
	cycle billingCycleRow,
) string {
	// Goal: Temporal-like, informative, PDF-safe snapshot
	//
	// Examples:
	// Subscription:
	//   "Base Subscription (Monthly)"
	//   "Jan 1 – Jan 31, 2026"
	//
	// Usage:
	//   "Actions (Total Qty: 123.00, Rate: USD 0.000010 / unit)"
	//   "Jan 1 – Jan 31, 2026"

	base := strings.TrimSpace(p.DisplayName)

	// ---- Line-type specific enrichment ----
	switch p.Type {

	case invoicedomain.InvoiceItemLineTypeSubscription:
		// Subscription lines are usually flat / periodic.
		// We intentionally DO NOT show quantity unless explicitly meaningful.
		// Rate is optional; usually already implied by plan.
		if p.RateAmount > 0 {
			rate := fmt.Sprintf(
				"%s",
				formatMoney(p.RateAmount, p.Currency),
			)
			base = fmt.Sprintf("%s (%s)", base, rate)
		}

	case invoicedomain.InvoiceItemLineTypeUsage:
		// Usage lines must be explicit: quantity + rate.
		parts := make([]string, 0, 2)

		if p.Quantity >= 0 {
			parts = append(parts,
				fmt.Sprintf("Total Qty: %.2f", p.Quantity),
			)
		}

		if p.RateAmount >= 0 {
			// High precision rate for description
			c := strings.ToUpper(p.Currency)
			decimals, ok := currencyDecimals[c]
			if !ok {
				decimals = 2
			}
			divisor := math.Pow(10, float64(decimals))
			val := float64(p.RateAmount) / divisor

			rate := fmt.Sprintf(
				"%s %.6f / %s",
				c,
				val,
				unitOrDefault(p.UnitLabel),
			)
			parts = append(parts, fmt.Sprintf("Rate: %s", rate))
		}

		if len(parts) > 0 {
			base = fmt.Sprintf("%s (%s)", base, strings.Join(parts, ", "))
		}
	}

	// ---- Period (always shown, PDF-friendly) ----
	period := fmt.Sprintf(
		"%s – %s",
		cycle.PeriodStart.Format("Jan 2, 2006"),
		cycle.PeriodEnd.Format("Jan 2, 2006"),
	)

	// Newline is intentional:
	// - HTML renderer can split into title / subtitle
	// - PDF renderer keeps line-break
	return base + "\n" + period
}

func (s *Service) FinalizeInvoice(ctx context.Context, invoiceID string) error {
	id, err := parseID(strings.TrimSpace(invoiceID))
	if err != nil {
		return invoicedomain.ErrInvalidInvoiceID
	}

	var finalizedInvoice *invoicedomain.Invoice
	var publicToken publicinvoicedomain.PublicInvoiceToken
	var renderedChecksum string
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		invoice, err := s.loadInvoiceForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if invoice == nil {
			return invoicedomain.ErrInvoiceNotFound
		}
		if invoice.Status == invoicedomain.InvoiceStatusFinalized {
			s.log.Info("invoice already finalized, skipping", zap.String("invoice_id", invoiceID))
			return nil
		}
		if invoice.Status != invoicedomain.InvoiceStatusDraft {
			return invoicedomain.ErrInvoiceNotDraft
		}
		if invoice.SubtotalAmount < 0 {
			return invoicedomain.ErrInvalidSubtotal
		}

		// Tax is resolved and frozen at finalize-time.
		taxDef, err := s.taxResolver.ResolveForInvoice(ctx, invoice.OrgID, invoice.CustomerID)
		if err != nil {
			return err
		}
		invoice.TaxRate = nil
		invoice.TaxCode = nil
		invoice.TaxAmount = 0

		now := time.Now().UTC()
		dueAt := now.AddDate(0, 0, 30)

		if taxDef != nil {
			switch taxDef.TaxMode {
			case taxdomain.TaxModeExclusive:
				invoice.TaxAmount = taxservice.ComputeTaxExclusive(invoice.SubtotalAmount, taxDef.Rate)
			case taxdomain.TaxModeInclusive:
				invoice.TaxAmount = taxservice.ComputeTaxInclusive(invoice.SubtotalAmount, taxDef.Rate)
			default:
				invoice.TaxAmount = 0
			}
			invoice.TaxRate = taxDef.Rate
			invoice.TaxCode = &taxDef.Code

			// SNAPSHOT: Create InvoiceTaxLine
			taxLine := invoicedomain.InvoiceTaxLine{
				ID:        s.genID.Generate(),
				OrgID:     invoice.OrgID,
				InvoiceID: invoice.ID,
				TaxCode:   &taxDef.Code,
				TaxName:   taxDef.Name,
				TaxMode:   string(taxDef.TaxMode),
				TaxRate:   *taxDef.Rate,
				Amount:    invoice.TaxAmount,
				CreatedAt: now,
			}
			if err := tx.WithContext(ctx).Create(&taxLine).Error; err != nil {
				return err
			}
		}
		invoice.TotalAmount = invoice.SubtotalAmount + invoice.TaxAmount

		// Snapshot rendered output at finalization so future template edits never change history.
		invoice.Status = invoicedomain.InvoiceStatusFinalized
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

		invoice.DueAt = &dueAt
		invoice.IssuedAt = &now
		invoice.FinalizedAt = &now

		if err := tx.WithContext(ctx).Exec(
			`UPDATE invoices
			 SET status = ?, finalized_at = ?, issued_at = ?, due_at = ?, invoice_template_id = ?, rendered_html = ?, rendered_pdf_url = ?, tax_rate = ?, tax_code = ?, tax_amount = ?, total_amount = ?, updated_at = ?
			 WHERE id = ?`,
			invoice.Status,
			invoice.FinalizedAt,
			invoice.IssuedAt,
			invoice.DueAt,
			invoice.InvoiceTemplateID,
			invoice.RenderedHTML,
			invoice.RenderedPDFURL,
			invoice.TaxRate,
			invoice.TaxCode,
			invoice.TaxAmount,
			invoice.TotalAmount,
			now,
			id,
		).Error; err != nil {
			return err
		}
		finalizedInvoice = invoice

		if publicToken, err = s.publicTokenSvc.EnsureForInvoice(ctx, *finalizedInvoice); err != nil {
			return err
		}

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

		// CRITICAL: Post to ledger ONLY on finalization, within same transaction
		// This ensures accounting reflects ONLY finalized invoices
		// Idempotency is handled by ledger service (ON CONFLICT DO NOTHING)
		if err := s.postInvoiceToLedger(ctx, tx, invoice); err != nil {
			return err
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

		// Trigger Notifications (Async)
		// Trigger Notifications (Async)
		go func(inv *invoicedomain.Invoice, tokenHash string) {
			// Create a detached context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()

			if err := s.sendInvoiceNotification(ctx, inv, tokenHash); err != nil {
				s.log.Error("failed to send invoice notification", zap.Error(err), zap.String("invoice_id", inv.ID.String()))
			}
		}(finalizedInvoice, publicToken.TokenHash)
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
	if invoice.InvoiceNumber != "" {
		metadata["invoice_number"] = invoice.InvoiceNumber
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

func (s *Service) loadBillingCycleForUpdate(ctx context.Context, tx *gorm.DB, id snowflake.ID) (*billingCycleRow, error) {
	var cycle billingCycleRow
	err := tx.WithContext(ctx).Raw(
		`SELECT id, org_id, subscription_id, period_start, period_end, status
		 FROM billing_cycles
		 WHERE id = ?
		 FOR UPDATE SKIP LOCKED`,
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

func (s *Service) nextInvoiceNumber(
	ctx context.Context,
	tx *gorm.DB,
	orgID snowflake.ID,
) (int64, error) {

	var next int64
	err := tx.WithContext(ctx).Raw(`
		UPDATE invoice_sequences
		SET next_number = next_number + 1,
		    updated_at = now()
		WHERE org_id = ?
		RETURNING next_number - 1
	`, orgID).Scan(&next).Error

	return next, err
}

func (s *Service) loadRating(ctx context.Context, tx *gorm.DB, cycleID snowflake.ID) (*ratingdomain.RatingResult, error) {
	var rating ratingdomain.RatingResult
	err := tx.WithContext(ctx).Raw(
		`SELECT id, org_id, subscription_id, meter_id, price_id,
		quantity, unit_price, amount, currency, period_start, period_end
		FROM rating_results
		WHERE billing_cycle_id = ?
		`, cycleID,
	).Scan(&rating).Error
	if err != nil {
		return nil, err
	}

	return &rating, nil
}

func (s *Service) loadMeter(
	ctx context.Context,
	tx *gorm.DB,
	meterID snowflake.ID,
) (*meterdomain.Meter, error) {

	var meter meterdomain.Meter
	err := tx.WithContext(ctx).
		Where("id = ?", meterID).
		Limit(1).
		Take(&meter).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, meterdomain.ErrMeterNotFound
		}

		return nil, err
	}

	return &meter, nil
}

func (s *Service) loadPrice(
	ctx context.Context,
	tx *gorm.DB,
	priceID snowflake.ID,
) (*pricedomain.Price, error) {

	var price pricedomain.Price
	err := tx.WithContext(ctx).
		Where(`
			id = ?
		`, priceID).
		Limit(1).
		Take(&price).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pricedomain.ErrNotFound
		}
		return nil, err
	}

	return &price, nil
}

func (s *Service) loadPriceAmount(
	ctx context.Context,
	tx *gorm.DB,
	priceID snowflake.ID,
	currency string,
	from, to time.Time,
) (*priceamountdomain.PriceAmount, error) {

	var pa priceamountdomain.PriceAmount

	err := tx.WithContext(ctx).
		Where(`
			price_id = ?
			AND currency = ?
			AND effective_from <= ?
			AND (effective_to IS NULL OR effective_to >= ?)
		`,
			priceID,
			strings.ToUpper(currency),
			to,
			from,
		).
		Order("effective_from DESC").
		Limit(1).
		Take(&pa).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, priceamountdomain.ErrNotFound
		}
		return nil, err
	}

	return &pa, nil
}

func (s *Service) insertInvoice(ctx context.Context, tx *gorm.DB, invoice invoicedomain.Invoice) (bool, error) {
	result := tx.WithContext(ctx).Exec(
		`INSERT INTO invoices (
			id, org_id, invoice_seq, invoice_number, billing_cycle_id, subscription_id, customer_id,
			invoice_template_id, status, subtotal_amount, total_amount, currency, period_start, period_end,
			issued_at, due_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (billing_cycle_id) DO NOTHING`,
		invoice.ID,
		invoice.OrgID,
		invoice.InvoiceSeq,
		invoice.InvoiceNumber,
		invoice.BillingCycleID,
		invoice.SubscriptionID,
		invoice.CustomerID,
		invoice.InvoiceTemplateID,
		invoice.Status,
		invoice.SubtotalAmount,
		invoice.TotalAmount,
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
			id, org_id, invoice_id, rating_result_id,
			description, quantity, unit_price, amount, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID,
		item.OrgID,
		item.InvoiceID,
		item.RatingResultID,
		item.Description,
		item.Quantity,
		item.UnitPrice,
		item.Amount,
		item.CreatedAt,
	).Error
}

func (s *Service) loadInvoiceForUpdate(ctx context.Context, tx *gorm.DB, id snowflake.ID) (*invoicedomain.Invoice, error) {
	var invoice invoicedomain.Invoice
	query := `SELECT id, org_id, invoice_number, billing_cycle_id, subscription_id, customer_id,
		        invoice_template_id, status, subtotal_amount, tax_rate, tax_code, tax_amount, total_amount, currency, period_start, period_end,
		        issued_at, due_at, finalized_at, voided_at, rendered_html, rendered_pdf_url,
		        created_at, updated_at
		 FROM invoices
		 WHERE id = ?`

	if tx.Dialector.Name() != "sqlite" {
		query += " FOR UPDATE"
	}

	err := tx.WithContext(ctx).Raw(
		query,
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

func sumCreditLines(lines []ledgerEntryLineRow) (int64, []ledgerEntryLineRow) {
	var subtotal int64
	credits := make([]ledgerEntryLineRow, 0, len(lines))
	for _, l := range lines {
		if l.Direction != ledgerdomain.LedgerEntryDirectionCredit {
			continue
		}
		subtotal += l.Amount
		credits = append(credits, l)
	}
	return subtotal, credits
}

func unitOrDefault(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return "unit"
	}
	return u
}

func formatQty(v float64) string {
	// keep stable format like "0.00"
	return fmt.Sprintf("%.2f", v)
}

var currencyDecimals = map[string]int{
	"USD": 2,
	"EUR": 2,
	"SGD": 2,
	"CNY": 2,
	"IDR": 0,
}

func formatMoney(amount int64, currency string) string {
	c := strings.ToUpper(currency)
	decimals, ok := currencyDecimals[c]
	if !ok {
		// fallback: assume 2 decimals
		decimals = 2
	}

	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}

	if decimals == 0 {
		return fmt.Sprintf("%s%s %d", sign, c, amount)
	}

	divisor := int64(1)
	for i := 0; i < decimals; i++ {
		divisor *= 10
	}

	major := amount / divisor
	minor := amount % divisor

	return fmt.Sprintf(
		"%s%s %d.%0*d",
		sign,
		c,
		major,
		decimals,
		minor,
	)
}

// sendInvoiceNotification generates PDF and sends email
func (s *Service) sendInvoiceNotification(ctx context.Context, invoice *invoicedomain.Invoice, tokenHash string) error {
	// 1. Load Org and Customer for details
	// Fetch Org
	type Org struct {
		Name         string
		SupportEmail string
	}
	var org Org
	if err := s.db.WithContext(ctx).Table("organizations").Select("name, support_email").Where("id = ?", invoice.OrgID).Scan(&org).Error; err != nil {
		s.log.Error("failed to fetch organization for notification", zap.Error(err))
		// Fallback to defaults if DB fail (unlikely inside transaction, but safe)
		org.Name = "Railzway Billing"     // Generic fallback
		org.SupportEmail = "support@railzway.com"
	}

	// Fetch Customer
	type Customer struct {
		Email string
	}
	var cust Customer
	if err := s.db.WithContext(ctx).Table("customers").Select("email").Where("id = ? AND org_id = ?", invoice.CustomerID, invoice.OrgID).Scan(&cust).Error; err != nil {
		s.log.Error("failed to fetch customer for notification", zap.Error(err))
	}
	
	// Default support email if empty in DB
	if org.SupportEmail == "" {
		org.SupportEmail = "support@railzway.com" 
	}

	// 2. Generate PDF (Proof of concept)
	pdfData := pdf.InvoiceData{
		InvoiceNumber: invoice.ID.String(),
		IssueDate:     invoice.IssuedAt.Format("January 2, 2006"),
		DueDate:       invoice.DueAt.Format("January 2, 2006"),
		TotalDue:      fmt.Sprintf("$%.2f", float64(invoice.TotalAmount)/100.0), // TODO: Currency formatting
		Total:         fmt.Sprintf("$%.2f", float64(invoice.TotalAmount)/100.0),
		OrgName:       org.Name,
		// Populate other fields as needed
	}

	pdfReader, err := s.pdfProvider.GenerateInvoice(ctx, pdfData)
	var pdfBytes []byte
	if err != nil {
		s.log.Error("failed to generate invoice PDF", zap.Error(err))
	} else if pdfReader != nil {
		pdfBytes, err = io.ReadAll(pdfReader)
		if err != nil {
			s.log.Error("failed to read PDF content", zap.Error(err))
		}
	}

	// 3. Send Email
	emailData := struct {
		OrgName         string
		Total           string
		DueDate         string
		PaymentLink     string
		InvoiceNumber   string
		OrgContactEmail string
	}{
		OrgName:         org.Name,
		Total:           pdfData.Total, // Reuse formatted total strings
		DueDate:         pdfData.DueDate,
		PaymentLink:     fmt.Sprintf("http://localhost:5173/%s/%s", invoice.OrgID, tokenHash), // TODO: BaseURL from config
		InvoiceNumber:   invoice.ID.String(),
		OrgContactEmail: org.SupportEmail,
	}

	to := []string{cust.Email}
	if cust.Email == "" {
		// Fallback for dev/demo if customer email is missing
		s.log.Warn("customer email missing, using fallback", zap.String("invoice_id", invoice.ID.String()))
		to = []string{"taufiktriantono4@gmail.com"} 
	}

	msg := email.EmailMessage{
		To:         to,
		SenderName: org.Name,
		ReplyTo:    org.SupportEmail,
		Subject:    fmt.Sprintf("New invoice from %s. #%s", org.Name, invoice.InvoiceNumber),
	}

	if len(pdfBytes) > 0 {
		msg.Attachments = []email.Attachment{
			{
				Filename: fmt.Sprintf("invoice-%s.pdf", invoice.InvoiceNumber),
				Content:  pdfBytes,
			},
		}
	}

	err = s.emailProvider.SendTemplate(ctx, msg, "invoice_new", emailData)
	if err != nil {
		s.log.Error("failed to send invoice email", zap.Error(err))
		return err
	}

	s.log.Info("invoice notification sent", zap.String("invoice_id", invoice.ID.String()), zap.String("to", to[0]), zap.Bool("has_pdf", len(pdfBytes) > 0))
	return nil
}
