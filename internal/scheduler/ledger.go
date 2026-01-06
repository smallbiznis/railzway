package scheduler

import (
	"context"
	"errors"
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

func (s *Scheduler) ensureLedgerEntryForCycle(
	ctx context.Context,
	cycle WorkBillingCycle,
) error {

	summary, err := s.summarizeRatingResults(ctx, cycle.OrgID, cycle.ID)
	if err != nil {
		return err
	}

	if summary.Total <= 0 {
		return invoicedomain.ErrMissingRatingResults
	}

	now := s.clock.Now()
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

	revenueUsageID, err := s.ensureLedgerAccount(
		ctx,
		cycle.OrgID,
		string(ledgerdomain.AccountCodeRevenue),
		"Revenue (Usage)",
		now,
	)
	if err != nil {
		return err
	}

	lines := []ledgerdomain.LedgerEntryLine{
		{
			AccountID: arID,
			Direction: ledgerdomain.LedgerEntryDirectionDebit,
			Amount:    summary.Total,
		},
		{
			AccountID: revenueUsageID,
			Direction: ledgerdomain.LedgerEntryDirectionCredit,
			Amount:    summary.Total,
		},
	}

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
) (*ratingSummary, error) {

	type row struct {
		Currency string
		Total    int64
	}

	var rows []row

	err := s.db.WithContext(ctx).Raw(
		`
		SELECT
			currency,
			SUM(amount) AS total
		FROM rating_results
		WHERE org_id = ?
		  AND billing_cycle_id = ?
		GROUP BY currency
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
	if len(rows) > 1 {
		return nil, invoicedomain.ErrCurrencyMismatch
	}

	if rows[0].Total <= 0 {
		return nil, ledgerdomain.ErrInvalidLineAmount
	}

	return &ratingSummary{
		Currency: rows[0].Currency,
		Total:    rows[0].Total,
	}, nil
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
