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

type ratingSummary struct {
	Currency string
	Total    int64
}

func (s *Scheduler) ensureLedgerEntryForCycle(ctx context.Context, cycle WorkBillingCycle) error {
	summaries, err := s.summarizeRatingResults(ctx, cycle.OrgID, cycle.ID)
	if err != nil {
		return err
	}
	if len(summaries) == 0 {
		return invoicedomain.ErrMissingRatingResults
	}
	if len(summaries) > 1 {
		return invoicedomain.ErrCurrencyMismatch
	}
	summary := summaries[0]
	if summary.Total < 0 {
		return ledgerdomain.ErrInvalidLineAmount
	}

	now := time.Now().UTC()
	accountsReceivableID, err := s.ensureLedgerAccount(ctx, cycle.OrgID, ledgerdomain.AccountCodeAccountsReceivable, "Accounts Receivable", now)
	if err != nil {
		return err
	}
	revenueID, err := s.ensureLedgerAccount(ctx, cycle.OrgID, ledgerdomain.AccountCodeRevenue, "Revenue", now)
	if err != nil {
		return err
	}

	lines := []ledgerdomain.LedgerEntryLine{
		{AccountID: accountsReceivableID, Direction: ledgerdomain.LedgerEntryDirectionDebit, Amount: summary.Total},
		{AccountID: revenueID, Direction: ledgerdomain.LedgerEntryDirectionCredit, Amount: summary.Total},
	}

	return s.ledgerSvc.CreateEntry(
		ctx,
		cycle.OrgID,
		ledgerdomain.SourceTypeBillingCycle,
		cycle.ID,
		summary.Currency,
		cycle.PeriodEnd,
		lines,
	)
}

func (s *Scheduler) summarizeRatingResults(ctx context.Context, orgID, cycleID snowflake.ID) ([]ratingSummary, error) {
	var summaries []ratingSummary
	err := s.db.WithContext(ctx).Raw(
		`SELECT currency, SUM(amount) AS total
		 FROM rating_results
		 WHERE org_id = ? AND billing_cycle_id = ?
		 GROUP BY currency`,
		orgID,
		cycleID,
	).Scan(&summaries).Error
	if err != nil {
		return nil, err
	}
	return summaries, nil
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
