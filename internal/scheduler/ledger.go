package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	ledgerdomain "github.com/smallbiznis/valora/internal/ledger/domain"
)

// Note: Deprecated
type ratingSummary struct {
	Currency string
	Total    int64
}

type RatingRevenueSummary struct {
	Currency string
	Lines    []RatingRevenueLine
}

type RatingRevenueLine struct {
	AccountCode ledgerdomain.LedgerAccountCode
	Amount      int64
}

func (s *Scheduler) ensureLedgerEntryForCycle(
	ctx context.Context,
	cycle WorkBillingCycle,
) error {

	summary, err := s.summarizeRatingResults(ctx, cycle.OrgID, cycle.ID)
	if err != nil {
		return err
	}
	if len(summary.Lines) == 0 {
		return invoicedomain.ErrMissingRatingResults
	}

	now := time.Now().UTC()

	arID, err := s.ensureLedgerAccount(
		ctx,
		cycle.OrgID,
		string(ledgerdomain.AccountCodeAccountsReceivable),
		"Accounts Receivable",
		now,
	)
	if err != nil {
		return err
	}

	var lines []ledgerdomain.LedgerEntryLine
	var total int64

	for _, line := range summary.Lines {
		if line.Amount <= 0 {
			continue
		}

		revenueID, err := s.ensureLedgerAccount(
			ctx,
			cycle.OrgID,
			string(line.AccountCode), // revenue_usage / revenue_flat
			string(line.AccountCode),
			now,
		)
		if err != nil {
			return err
		}

		lines = append(lines,
			ledgerdomain.LedgerEntryLine{
				AccountID: revenueID,
				Direction: ledgerdomain.LedgerEntryDirectionCredit,
				Amount:    line.Amount,
			},
		)

		total += line.Amount
	}

	if total <= 0 {
		return ledgerdomain.ErrInvalidLineAmount
	}

	// AR line (single, aggregated)
	lines = append(lines,
		ledgerdomain.LedgerEntryLine{
			AccountID: arID,
			Direction: ledgerdomain.LedgerEntryDirectionDebit,
			Amount:    total,
		},
	)

	return s.ledgerSvc.CreateEntry(
		ctx,
		cycle.OrgID,
		string(ledgerdomain.SourceTypeBillingCycle),
		cycle.ID,
		summary.Currency,
		cycle.PeriodEnd,
		lines,
	)
}

func (s *Scheduler) summarizeRatingResults(
	ctx context.Context,
	orgID snowflake.ID,
	billingCycleID snowflake.ID,
) (*RatingRevenueSummary, error) {

	type row struct {
		Currency   string
		ChargeType string
		Amount     int64
	}

	var rows []row

	err := s.db.WithContext(ctx).Raw(
		`
		SELECT
			rr.currency,
			rri.charge_type,
			SUM(rri.amount) AS amount
		FROM rating_results rr
		JOIN rating_result_items rri
			ON rri.rating_result_id = rr.id
		WHERE rr.org_id = ?
		  AND rr.billing_cycle_id = ?
		GROUP BY rr.currency, rri.charge_type
		`,
		orgID,
		billingCycleID,
	).Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, invoicedomain.ErrMissingRatingResults
	}

	summary := &RatingRevenueSummary{
		Currency: rows[0].Currency,
	}

	for _, r := range rows {
		var account ledgerdomain.LedgerAccountCode

		switch r.ChargeType {
		case "usage":
			account = ledgerdomain.AccountCodeRevenueUsage
		case "flat":
			account = ledgerdomain.AccountCodeRevenueFlat
		default:
			return nil, fmt.Errorf("unknown charge_type: %s", r.ChargeType)
		}

		if r.Amount <= 0 {
			continue
		}

		summary.Lines = append(summary.Lines, RatingRevenueLine{
			AccountCode: account,
			Amount:      r.Amount,
		})
	}

	if len(summary.Lines) == 0 {
		return nil, ledgerdomain.ErrInvalidLineAmount
	}

	return summary, nil
}

func (s *Scheduler) ensureLedgerAccount(ctx context.Context, orgID snowflake.ID, code string, name string, now time.Time) (snowflake.ID, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return 0, ledgerdomain.ErrInvalidAccount
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, ledgerdomain.ErrInvalidAccount
	}

	var accountID snowflake.ID
	if err := s.db.WithContext(ctx).Raw(
		`SELECT id
		 FROM ledger_accounts
		 WHERE org_id = ? AND code = ?`,
		orgID,
		code,
	).Scan(&accountID).Error; err != nil {
		return 0, err
	}
	if accountID != 0 {
		return accountID, nil
	}

	newID := s.genID.Generate()
	if err := s.db.WithContext(ctx).Exec(
		`INSERT INTO ledger_accounts (id, org_id, code, name, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (org_id, code) DO NOTHING`,
		newID,
		orgID,
		code,
		name,
		now,
	).Error; err != nil {
		return 0, err
	}

	if err := s.db.WithContext(ctx).Raw(
		`SELECT id
		 FROM ledger_accounts
		 WHERE org_id = ? AND code = ?`,
		orgID,
		code,
	).Scan(&accountID).Error; err != nil {
		return 0, err
	}
	if accountID == 0 {
		return 0, errors.New("ledger_account_not_found")
	}
	return accountID, nil
}
