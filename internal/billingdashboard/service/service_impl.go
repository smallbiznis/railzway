package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	billingdashboard "github.com/smallbiznis/valora/internal/billingdashboard/domain"
	"github.com/smallbiznis/valora/internal/orgcontext"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB  *gorm.DB
	Log *zap.Logger
}

type Service struct {
	db  *gorm.DB
	log *zap.Logger
}

func NewService(p Params) billingdashboard.Service {
	return &Service{
		db:  p.DB,
		log: p.Log.Named("billingdashboard.service"),
	}
}

type customerBalanceRow struct {
	CustomerID    snowflake.ID  `gorm:"column:customer_id"`
	Name          string        `gorm:"column:name"`
	Balance       int64         `gorm:"column:balance"`
	Currency      string        `gorm:"column:currency"`
	LastInvoiceID *snowflake.ID `gorm:"column:last_invoice_id"`
}

func (s *Service) ListCustomerBalances(ctx context.Context) (billingdashboard.CustomerBalancesResponse, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return billingdashboard.CustomerBalancesResponse{}, err
	}

	var rows []customerBalanceRow
	query := `
		WITH latest_invoice AS (
			SELECT DISTINCT ON (i.customer_id) i.customer_id, i.id, i.currency, i.created_at
			FROM invoices i
			WHERE i.org_id = ?
			ORDER BY i.customer_id, i.created_at DESC, i.id DESC
		)
		SELECT c.id AS customer_id,
		       c.name AS name,
		       COALESCE(cb.balance, 0) AS balance,
		       COALESCE(NULLIF(cb.currency, ''), NULLIF(li.currency, ''), NULLIF(c.currency, ''), '') AS currency,
		       li.id AS last_invoice_id
		FROM customers c
		LEFT JOIN customer_balances cb ON cb.customer_id = c.id AND cb.org_id = ?
		LEFT JOIN latest_invoice li ON li.customer_id = c.id
		WHERE c.org_id = ?
		ORDER BY c.name ASC`

	if err := s.db.WithContext(ctx).Raw(
		query,
		orgID,
		orgID,
		orgID,
	).Scan(&rows).Error; err != nil {
		return billingdashboard.CustomerBalancesResponse{}, err
	}

	customers := make([]billingdashboard.CustomerBalance, 0, len(rows))
	for _, row := range rows {
		balance := row.Balance
		paymentStatus := "settled"
		switch {
		case balance > 0:
			paymentStatus = "due"
		case balance < 0:
			paymentStatus = "credit"
		}

		currency := strings.ToUpper(strings.TrimSpace(row.Currency))
		lastInvoiceID := ""
		if row.LastInvoiceID != nil && *row.LastInvoiceID != 0 {
			lastInvoiceID = row.LastInvoiceID.String()
		}

		customers = append(customers, billingdashboard.CustomerBalance{
			CustomerID:    row.CustomerID.String(),
			Name:          row.Name,
			Balance:       balance,
			Currency:      currency,
			LastInvoiceID: lastInvoiceID,
			PaymentStatus: paymentStatus,
		})
	}

	return billingdashboard.CustomerBalancesResponse{Customers: customers}, nil
}

type billingCycleRow struct {
	CycleID      snowflake.ID `gorm:"column:id"`
	PeriodStart  time.Time    `gorm:"column:period_start"`
	Status       string       `gorm:"column:status"`
	TotalRevenue int64        `gorm:"column:total_revenue"`
	InvoiceCount int64        `gorm:"column:invoice_count"`
}

func (s *Service) ListBillingCycles(ctx context.Context) (billingdashboard.BillingCycleSummaryResponse, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return billingdashboard.BillingCycleSummaryResponse{}, err
	}

	var rows []billingCycleRow
	query := `
		SELECT billing_cycle_id AS id,
		       period_start,
		       status,
		       total_revenue,
		       invoice_count
		FROM billing_cycle_stats
		WHERE org_id = ?
		ORDER BY period_start DESC, billing_cycle_id DESC
		LIMIT 36`

	if err := s.db.WithContext(ctx).Raw(
		query,
		orgID,
	).Scan(&rows).Error; err != nil {
		return billingdashboard.BillingCycleSummaryResponse{}, err
	}

	cycles := make([]billingdashboard.BillingCycleSummary, 0, len(rows))
	for _, row := range rows {
		period := row.PeriodStart.UTC().Format("2006-01")
		status := strings.ToLower(strings.TrimSpace(row.Status))
		cycles = append(cycles, billingdashboard.BillingCycleSummary{
			CycleID:      row.CycleID.String(),
			Period:       period,
			TotalRevenue: row.TotalRevenue,
			InvoiceCount: row.InvoiceCount,
			Status:       status,
		})
	}

	return billingdashboard.BillingCycleSummaryResponse{Cycles: cycles}, nil
}

type activityRow struct {
	Action    string            `gorm:"column:action"`
	Metadata  datatypes.JSONMap `gorm:"column:metadata"`
	CreatedAt time.Time         `gorm:"column:created_at"`
}

func (s *Service) ListBillingActivity(ctx context.Context, limit int) (billingdashboard.BillingActivityResponse, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return billingdashboard.BillingActivityResponse{}, err
	}
	if limit <= 0 {
		limit = 15
	}

	actions := []string{
		"billing_cycle.closed",
		"billing_cycle.closing_started",
		"invoice.generate",
		"invoice.finalize",
		"invoice.void",
		"invoice.generated",
		"invoice.finalized",
		"invoice.voided",
		"payment.received",
	}

	var rows []activityRow
	if err := s.db.WithContext(ctx).Raw(
		`SELECT action, metadata, created_at
		 FROM audit_logs
		 WHERE org_id = ? AND action IN ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		orgID,
		actions,
		limit,
	).Scan(&rows).Error; err != nil {
		return billingdashboard.BillingActivityResponse{}, err
	}

	activity := make([]billingdashboard.BillingActivity, 0, len(rows))
	for _, row := range rows {
		message := buildActivityMessage(row.Action, row.Metadata)
		if message == "" {
			continue
		}
		activity = append(activity, billingdashboard.BillingActivity{
			Message:    message,
			OccurredAt: row.CreatedAt,
		})
	}

	return billingdashboard.BillingActivityResponse{Activity: activity}, nil
}

func buildActivityMessage(action string, metadata datatypes.JSONMap) string {
	switch strings.TrimSpace(action) {
	case "billing_cycle.closed":
		if period := formatPeriod(metadata); period != "" {
			return fmt.Sprintf("Billing cycle %s closed", period)
		}
		return "Billing cycle closed"
	case "billing_cycle.closing_started":
		if period := formatPeriod(metadata); period != "" {
			return fmt.Sprintf("Billing cycle %s closing", period)
		}
		return "Billing cycle closing"
	case "invoice.generate", "invoice.generated":
		return formatInvoiceMessage("created", metadata)
	case "invoice.finalize", "invoice.finalized":
		return formatInvoiceMessage("finalized", metadata)
	case "invoice.void", "invoice.voided":
		return formatInvoiceMessage("voided", metadata)
	case "payment.received":
		if customer := formatCustomerLabel(metadata); customer != "" {
			return fmt.Sprintf("Payment received from %s", customer)
		}
		return "Payment received"
	default:
		return ""
	}
}

func formatInvoiceMessage(verb string, metadata datatypes.JSONMap) string {
	label := formatInvoiceLabel(metadata)
	if label == "" {
		return fmt.Sprintf("Invoice %s", verb)
	}
	return fmt.Sprintf("Invoice %s %s", label, verb)
}

func formatPeriod(metadata datatypes.JSONMap) string {
	value, ok := metadata["period_end"]
	if !ok {
		value = metadata["period_start"]
	}
	parsed, ok := value.(string)
	if !ok || strings.TrimSpace(parsed) == "" {
		return ""
	}
	at, err := time.Parse(time.RFC3339, parsed)
	if err != nil {
		return ""
	}
	return at.UTC().Format("Jan 2006")
}

func formatInvoiceLabel(metadata datatypes.JSONMap) string {
	if value, ok := metadata["invoice_number"]; ok {
		switch typed := value.(type) {
		case float64:
			return fmt.Sprintf("INV-%d", int64(typed))
		case int64:
			return fmt.Sprintf("INV-%d", typed)
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed != "" {
				return fmt.Sprintf("INV-%s", trimmed)
			}
		}
	}
	return ""
}

func formatCustomerLabel(metadata datatypes.JSONMap) string {
	if value, ok := metadata["customer_name"]; ok {
		if name, ok := value.(string); ok {
			return strings.TrimSpace(name)
		}
	}
	return ""
}

func (s *Service) orgIDFromContext(ctx context.Context) (snowflake.ID, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return 0, billingdashboard.ErrInvalidOrganization
	}
	return snowflake.ID(orgID), nil
}
