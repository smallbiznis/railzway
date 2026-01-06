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
	Kind     string
	Currency string
	Total    int64
}

func (s *Scheduler) getLedgerAccountID(
	ctx context.Context,
	orgID snowflake.ID,
	code ledgerdomain.LedgerAccountCode,
) (snowflake.ID, error) {

	var id snowflake.ID
	if err := s.db.WithContext(ctx).Raw(
		`SELECT id
		 FROM ledger_accounts
		 WHERE org_id = ? AND code = ?`,
		orgID,
		string(code),
	).Scan(&id).Error; err != nil {
		return 0, err
	}

	if id == 0 {
		return 0, ledgerdomain.ErrInvalidAccount
	}

	return id, nil
}

func (s *Scheduler) ensureLedgerEntryForCycle(
	ctx context.Context,
	cycle WorkBillingCycle,
) error {

	summary, err := s.summarizeRatingResults(ctx, cycle.OrgID, cycle.ID)
	if err != nil {
		return err
	}

	var (
		currency string
		total    int64
		lines    []ledgerdomain.LedgerEntryLine
	)

	for _, v := range summary {
		if v.Total <= 0 {
			continue
		}

		// Enforce single currency
		if currency == "" {
			currency = v.Currency
		} else if currency != v.Currency {
			return invoicedomain.ErrCurrencyMismatch
		}

		var accountCode ledgerdomain.LedgerAccountCode
		switch v.Kind {
		case "flat":
			accountCode = ledgerdomain.AccountCodeRevenueFlat
		case "usage":
			accountCode = ledgerdomain.AccountCodeRevenueUsage
		default:
			continue
		}

		revenueID, err := s.getLedgerAccountID(ctx, cycle.OrgID, accountCode)
		if err != nil {
			return err
		}

		total += v.Total

		lines = append(lines, ledgerdomain.LedgerEntryLine{
			AccountID: revenueID,
			Direction: ledgerdomain.LedgerEntryDirectionCredit,
			Currency:  v.Currency,
			Amount:    v.Total,
		})
	}

	if len(lines) == 0 {
		return invoicedomain.ErrMissingRatingResults
	}

	arID, err := s.getLedgerAccountID(
		ctx,
		cycle.OrgID,
		ledgerdomain.AccountCodeAccountsReceivable,
	)
	if err != nil {
		return err
	}

	lines = append(lines, ledgerdomain.LedgerEntryLine{
		AccountID: arID,
		Direction: ledgerdomain.LedgerEntryDirectionDebit,
		Currency:  currency,
		Amount:    total,
	})

	return s.ledgerSvc.CreateEntry(
		ctx,
		cycle.OrgID,
		string(ledgerdomain.SourceTypeBillingCycle),
		cycle.ID,
		currency,
		cycle.PeriodEnd,
		lines,
	)
}

func (s *Scheduler) summarizeRatingResults(
	ctx context.Context,
	orgID snowflake.ID,
	billingCycleID snowflake.ID,
) ([]ratingSummary, error) {

	var rows []ratingSummary

	err := s.db.WithContext(ctx).Raw(
		`
		SELECT
			CASE
				WHEN meter_id IS NULL THEN 'flat'
				ELSE 'usage'
			END AS kind,
			currency,
			SUM(amount) AS total
		FROM rating_results
		WHERE org_id = ?
		  AND billing_cycle_id = ?
		GROUP BY kind, currency
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

	return rows, nil
}

// Deprecated
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
