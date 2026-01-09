package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	billingoverview "github.com/smallbiznis/valora/internal/billingoverview/domain"
	"github.com/smallbiznis/valora/internal/clock"
	ledgerdomain "github.com/smallbiznis/valora/internal/ledger/domain"
	"github.com/smallbiznis/valora/internal/orgcontext"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB    *gorm.DB
	Log   *zap.Logger
	Clock clock.Clock
}

type Service struct {
	db    *gorm.DB
	log   *zap.Logger
	clock clock.Clock
}

func NewService(p Params) billingoverview.Service {
	return &Service{
		db:    p.DB,
		log:   p.Log.Named("billingoverview.service"),
		clock: p.Clock,
	}
}

func (s *Service) GetMRR(ctx context.Context, req billingoverview.OverviewRequest) (billingoverview.MRRResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return billingoverview.MRRResponse{}, billingoverview.ErrInvalidOrganization
	}

	start, end := normalizeRange(req, s.clock.Now())
	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return billingoverview.MRRResponse{}, err
	}
	series, err := s.listMRRSeries(ctx, orgID, currency, start, end, req.Granularity)
	if err != nil {
		return billingoverview.MRRResponse{}, err
	}

	var compareSeries []billingoverview.SeriesPoint
	if req.Compare {
		prevStart, prevEnd := shiftRange(start, end, req.Granularity)
		compareSeries, err = s.listMRRSeries(ctx, orgID, currency, prevStart, prevEnd, req.Granularity)
		if err != nil {
			return billingoverview.MRRResponse{}, err
		}
	}

	current := lastSeriesValue(series)
	previous := lastSeriesValue(compareSeries)
	growthAmount, growthRate := computeGrowth(current, previous)

	return billingoverview.MRRResponse{
		Currency:      currency,
		Current:       current,
		Previous:      previous,
		GrowthAmount:  growthAmount,
		GrowthRate:    growthRate,
		Series:        series,
		CompareSeries: compareSeries,
		HasData:       len(series) > 0,
	}, nil
}

func (s *Service) GetMRRMovement(ctx context.Context, req billingoverview.OverviewRequest) (billingoverview.MRRMovementResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return billingoverview.MRRMovementResponse{}, billingoverview.ErrInvalidOrganization
	}

	start, end := normalizeRange(req, s.clock.Now())
	rangeEnd := endOfPeriod(end, req.Granularity)
	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return billingoverview.MRRMovementResponse{}, err
	}

	startSnapshot, err := s.listMRRSnapshot(ctx, orgID, currency, start)
	if err != nil {
		return billingoverview.MRRMovementResponse{}, err
	}
	endSnapshot, err := s.listMRRSnapshot(ctx, orgID, currency, rangeEnd)
	if err != nil {
		return billingoverview.MRRMovementResponse{}, err
	}

	var newMRR int64
	var expansionMRR int64
	var contractionMRR int64
	var churnedMRR int64

	for subscriptionID, endValue := range endSnapshot {
		startValue, ok := startSnapshot[subscriptionID]
		if !ok {
			newMRR += endValue
			continue
		}
		switch {
		case endValue > startValue:
			expansionMRR += endValue - startValue
		case endValue < startValue:
			contractionMRR += startValue - endValue
		}
	}

	for subscriptionID, startValue := range startSnapshot {
		if _, ok := endSnapshot[subscriptionID]; !ok {
			churnedMRR += startValue
		}
	}

	netMRRChange := newMRR + expansionMRR - contractionMRR - churnedMRR
	hasData := len(startSnapshot) > 0 || len(endSnapshot) > 0

	return billingoverview.MRRMovementResponse{
		Currency:       currency,
		NewMRR:         newMRR,
		ExpansionMRR:   expansionMRR,
		ContractionMRR: contractionMRR,
		ChurnedMRR:     churnedMRR,
		NetMRRChange:   netMRRChange,
		HasData:        hasData,
	}, nil
}

func (s *Service) GetRevenue(ctx context.Context, req billingoverview.OverviewRequest) (billingoverview.RevenueResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return billingoverview.RevenueResponse{}, billingoverview.ErrInvalidOrganization
	}

	start, end := normalizeRange(req, s.clock.Now())
	rangeEnd := endOfPeriod(end, req.Granularity)
	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return billingoverview.RevenueResponse{}, err
	}
	series, err := s.listRevenueSeries(ctx, orgID, currency, start, rangeEnd, req.Granularity)
	if err != nil {
		return billingoverview.RevenueResponse{}, err
	}

	var compareSeries []billingoverview.SeriesPoint
	if req.Compare {
		prevStart, prevEnd := shiftRange(start, end, req.Granularity)
		prevRangeEnd := endOfPeriod(prevEnd, req.Granularity)
		compareSeries, err = s.listRevenueSeries(ctx, orgID, currency, prevStart, prevRangeEnd, req.Granularity)
		if err != nil {
			return billingoverview.RevenueResponse{}, err
		}
	}

	total := sumSeries(series)
	previous := sumSeries(compareSeries)
	growthAmount, growthRate := computeGrowth(total, previous)

	return billingoverview.RevenueResponse{
		Currency:      currency,
		Total:         total,
		Previous:      previous,
		GrowthAmount:  growthAmount,
		GrowthRate:    growthRate,
		Series:        series,
		CompareSeries: compareSeries,
		HasData:       len(series) > 0,
	}, nil
}

func (s *Service) GetOutstandingBalance(ctx context.Context, req billingoverview.OverviewRequest) (billingoverview.OutstandingBalanceResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return billingoverview.OutstandingBalanceResponse{}, billingoverview.ErrInvalidOrganization
	}

	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return billingoverview.OutstandingBalanceResponse{}, err
	}

	now := s.clock.Now().UTC()
	row, err := s.loadOutstandingBalance(ctx, orgID, currency, now)
	if err != nil {
		return billingoverview.OutstandingBalanceResponse{}, err
	}

	return billingoverview.OutstandingBalanceResponse{
		Currency:    currency,
		Outstanding: row.Outstanding,
		Overdue:     row.Overdue,
		HasData:     row.InvoiceCount > 0,
	}, nil
}

func (s *Service) GetCollectionRate(ctx context.Context, req billingoverview.OverviewRequest) (billingoverview.CollectionRateResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return billingoverview.CollectionRateResponse{}, billingoverview.ErrInvalidOrganization
	}

	start, end := normalizeRange(req, s.clock.Now())
	rangeEnd := endOfPeriod(end, req.Granularity)
	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return billingoverview.CollectionRateResponse{}, err
	}

	invoicedAmount, err := s.loadInvoicedAmount(ctx, orgID, currency, start, rangeEnd)
	if err != nil {
		return billingoverview.CollectionRateResponse{}, err
	}

	collectedAmount, err := s.loadCollectedAmount(ctx, orgID, currency, start, rangeEnd)
	if err != nil {
		return billingoverview.CollectionRateResponse{}, err
	}

	var collectionRate *float64
	if invoicedAmount > 0 {
		rate := float64(collectedAmount) / float64(invoicedAmount)
		collectionRate = &rate
	}

	return billingoverview.CollectionRateResponse{
		Currency:         currency,
		CollectionRate:  collectionRate,
		CollectedAmount: collectedAmount,
		InvoicedAmount:  invoicedAmount,
		HasData:         invoicedAmount > 0,
	}, nil
}

func (s *Service) GetSubscribers(ctx context.Context, req billingoverview.OverviewRequest) (billingoverview.SubscribersResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return billingoverview.SubscribersResponse{}, billingoverview.ErrInvalidOrganization
	}

	start, end := normalizeRange(req, s.clock.Now())
	rangeEnd := endOfPeriod(end, req.Granularity)
	series, err := s.listSubscribersSeries(ctx, orgID, start, end, req.Granularity)
	if err != nil {
		return billingoverview.SubscribersResponse{}, err
	}

	var compareSeries []billingoverview.SeriesPoint
	if req.Compare {
		prevStart, prevEnd := shiftRange(start, end, req.Granularity)
		compareSeries, err = s.listSubscribersSeries(ctx, orgID, prevStart, prevEnd, req.Granularity)
		if err != nil {
			return billingoverview.SubscribersResponse{}, err
		}
	}

	current := lastSeriesValue(series)
	previous := lastSeriesValue(compareSeries)
	growthAmount, growthRate := computeGrowth(current, previous)

	churnRate, err := s.calculateChurn(ctx, orgID, start, rangeEnd)
	if err != nil {
		return billingoverview.SubscribersResponse{}, err
	}

	return billingoverview.SubscribersResponse{
		Current:       current,
		Previous:      previous,
		GrowthAmount:  growthAmount,
		GrowthRate:    growthRate,
		ChurnRate:     churnRate,
		Series:        series,
		CompareSeries: compareSeries,
		HasData:       len(series) > 0,
	}, nil
}

func (s *Service) loadOrgCurrency(ctx context.Context, orgID snowflake.ID) (string, error) {
	var row struct {
		Currency string `gorm:"column:currency"`
	}
	if err := s.db.WithContext(ctx).Raw(
		`SELECT currency FROM organization_billing_preferences WHERE org_id = ? LIMIT 1`,
		orgID,
	).Scan(&row).Error; err != nil {
		return "", err
	}
	currency := strings.ToUpper(strings.TrimSpace(row.Currency))
	if currency == "" {
		currency = "USD"
	}
	return currency, nil
}

func normalizeRange(req billingoverview.OverviewRequest, now time.Time) (time.Time, time.Time) {
	start := req.Start
	end := req.End
	if start.IsZero() || end.IsZero() {
		end = now.UTC()
		start = end.AddDate(0, 0, -30)
	}
	start = start.UTC()
	end = end.UTC()

	switch req.Granularity {
	case billingoverview.GranularityMonth:
		start = truncateToMonth(start)
		end = truncateToMonth(end)
	default:
		start = truncateToDay(start)
		end = truncateToDay(end)
	}

	return start, end
}

func shiftRange(start, end time.Time, granularity billingoverview.Granularity) (time.Time, time.Time) {
	switch granularity {
	case billingoverview.GranularityMonth:
		months := monthSpan(start, end)
		return start.AddDate(0, -months, 0), end.AddDate(0, -months, 0)
	default:
		days := daySpan(start, end)
		return start.AddDate(0, 0, -days), end.AddDate(0, 0, -days)
	}
}

func endOfPeriod(value time.Time, granularity billingoverview.Granularity) time.Time {
	switch granularity {
	case billingoverview.GranularityMonth:
		return value.AddDate(0, 1, 0).Add(-time.Nanosecond)
	default:
		return value.AddDate(0, 0, 1).Add(-time.Nanosecond)
	}
}

func daySpan(start, end time.Time) int {
	if end.Before(start) {
		return 0
	}
	return int(end.Sub(start).Hours()/24) + 1
}

func monthSpan(start, end time.Time) int {
	if end.Before(start) {
		return 0
	}
	yearDiff := end.Year() - start.Year()
	monthDiff := int(end.Month()) - int(start.Month())
	return yearDiff*12 + monthDiff + 1
}

func truncateToDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func truncateToMonth(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func computeGrowth(current *int64, previous *int64) (*int64, *float64) {
	if current == nil || previous == nil {
		return nil, nil
	}
	diff := *current - *previous
	growth := float64(0)
	if *previous != 0 {
		growth = float64(diff) / float64(*previous)
	} else {
		return &diff, nil
	}
	return &diff, &growth
}

func lastSeriesValue(series []billingoverview.SeriesPoint) *int64 {
	if len(series) == 0 {
		return nil
	}
	value := series[len(series)-1].Value
	return &value
}

func sumSeries(series []billingoverview.SeriesPoint) *int64 {
	if len(series) == 0 {
		return nil
	}
	var total int64
	for _, point := range series {
		total += point.Value
	}
	return &total
}

func (s *Service) listMRRSeries(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	start time.Time,
	end time.Time,
	granularity billingoverview.Granularity,
) ([]billingoverview.SeriesPoint, error) {
	periodTrunc, periodInterval, periodFormat := granularitySettings(granularity)

	query := fmt.Sprintf(
		`
		WITH periods AS (
			SELECT generate_series(
				date_trunc('%s', ?::timestamptz),
				date_trunc('%s', ?::timestamptz),
				interval '%s'
			) AS period_start
		),
		period_bounds AS (
			SELECT
				period_start,
				period_start + interval '%s' - interval '1 microsecond' AS period_end
			FROM periods
		)
		SELECT
			to_char(pb.period_start, '%s') AS period,
			COALESCE(
				SUM(
					ROUND(
						COALESCE(si.quantity, 1) * COALESCE(pa.unit_amount_cents, 0) *
						CASE p.billing_interval
							WHEN 'MONTH' THEN 1.0 / NULLIF(p.billing_interval_count, 0)
							WHEN 'YEAR' THEN 1.0 / (12.0 * NULLIF(p.billing_interval_count, 0))
							WHEN 'WEEK' THEN 30.0 / (7.0 * NULLIF(p.billing_interval_count, 0))
							WHEN 'DAY' THEN 30.0 / NULLIF(p.billing_interval_count, 0)
							ELSE 0
						END
					)
				),
				0
			)::BIGINT AS value
		FROM period_bounds pb
		LEFT JOIN subscriptions s
			ON s.org_id = ?
			AND s.status <> 'DRAFT'
			AND s.start_at <= pb.period_end
			AND (s.end_at IS NULL OR s.end_at > pb.period_end)
			AND (s.cancel_at IS NULL OR s.cancel_at > pb.period_end)
			AND (s.canceled_at IS NULL OR s.canceled_at > pb.period_end)
			AND (s.ended_at IS NULL OR s.ended_at > pb.period_end)
			AND NOT (s.paused_at IS NOT NULL AND s.paused_at <= pb.period_end AND (s.resumed_at IS NULL OR s.resumed_at > pb.period_end))
		LEFT JOIN subscription_items si
			ON si.org_id = s.org_id
			AND si.subscription_id = s.id
			AND si.billing_mode = 'LICENSED'
		LEFT JOIN prices p
			ON p.org_id = si.org_id
			AND p.id = si.price_id
			AND p.active = true
		LEFT JOIN price_amounts pa
			ON pa.org_id = p.org_id
			AND pa.price_id = p.id
			AND pa.currency = ?
			AND pa.effective_from <= pb.period_end
			AND (pa.effective_to IS NULL OR pa.effective_to > pb.period_end)
		GROUP BY pb.period_start
		ORDER BY pb.period_start
		`,
		periodTrunc,
		periodTrunc,
		periodInterval,
		periodInterval,
		periodFormat,
	)

	var rows []seriesRow
	if err := s.db.WithContext(ctx).Raw(
		query,
		start,
		end,
		orgID,
		currency,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}

	return mapSeriesRows(rows), nil
}

type mrrSnapshotRow struct {
	SubscriptionID snowflake.ID `gorm:"column:subscription_id"`
	MRR            int64        `gorm:"column:mrr"`
}

func (s *Service) listMRRSnapshot(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	at time.Time,
) (map[snowflake.ID]int64, error) {
	query := `
		SELECT
			s.id AS subscription_id,
			COALESCE(
				SUM(
					ROUND(
						COALESCE(si.quantity, 1) * COALESCE(pa.unit_amount_cents, 0) *
						CASE p.billing_interval
							WHEN 'MONTH' THEN 1.0 / NULLIF(p.billing_interval_count, 0)
							WHEN 'YEAR' THEN 1.0 / (12.0 * NULLIF(p.billing_interval_count, 0))
							WHEN 'WEEK' THEN 30.0 / (7.0 * NULLIF(p.billing_interval_count, 0))
							WHEN 'DAY' THEN 30.0 / NULLIF(p.billing_interval_count, 0)
							ELSE 0
						END
					)
				),
				0
			)::BIGINT AS mrr
		FROM subscriptions s
		LEFT JOIN subscription_items si
			ON si.org_id = s.org_id
			AND si.subscription_id = s.id
			AND si.billing_mode = 'LICENSED'
		LEFT JOIN prices p
			ON p.org_id = si.org_id
			AND p.id = si.price_id
			AND p.active = true
		LEFT JOIN price_amounts pa
			ON pa.org_id = p.org_id
			AND pa.price_id = p.id
			AND pa.currency = ?
			AND pa.effective_from <= ?
			AND (pa.effective_to IS NULL OR pa.effective_to > ?)
		WHERE s.org_id = ?
			AND s.status <> 'DRAFT'
			AND s.start_at <= ?
			AND (s.end_at IS NULL OR s.end_at > ?)
			AND (s.cancel_at IS NULL OR s.cancel_at > ?)
			AND (s.canceled_at IS NULL OR s.canceled_at > ?)
			AND (s.ended_at IS NULL OR s.ended_at > ?)
			AND NOT (s.paused_at IS NOT NULL AND s.paused_at <= ? AND (s.resumed_at IS NULL OR s.resumed_at > ?))
		GROUP BY s.id
	`

	var rows []mrrSnapshotRow
	if err := s.db.WithContext(ctx).Raw(
		query,
		currency,
		at,
		at,
		orgID,
		at,
		at,
		at,
		at,
		at,
		at,
		at,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[snowflake.ID]int64, len(rows))
	for _, row := range rows {
		if row.SubscriptionID == 0 {
			continue
		}
		result[row.SubscriptionID] = row.MRR
	}
	return result, nil
}

func (s *Service) listRevenueSeries(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	start time.Time,
	end time.Time,
	granularity billingoverview.Granularity,
) ([]billingoverview.SeriesPoint, error) {
	periodTrunc, periodInterval, periodFormat := granularitySettings(granularity)

	query := fmt.Sprintf(
		`
		WITH periods AS (
			SELECT generate_series(
				date_trunc('%s', ?::timestamptz),
				date_trunc('%s', ?::timestamptz),
				interval '%s'
			) AS period_start
		),
		revenue AS (
			SELECT
				date_trunc('%s', le.occurred_at) AS period_start,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS total
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.occurred_at >= ?
			  AND le.occurred_at <= ?
			  AND le.source_type IN (?, ?)
			  AND a.code IN (?, ?)
			GROUP BY 1
		)
		SELECT
			to_char(p.period_start, '%s') AS period,
			COALESCE(r.total, 0)::BIGINT AS value
		FROM periods p
		LEFT JOIN revenue r ON r.period_start = p.period_start
		ORDER BY p.period_start
		`,
		periodTrunc,
		periodTrunc,
		periodInterval,
		periodTrunc,
		periodFormat,
	)

	var rows []seriesRow
	if err := s.db.WithContext(ctx).Raw(
		query,
		start,
		end,
		orgID,
		currency,
		start,
		end,
		string(ledgerdomain.SourceTypeBillingCycle),
		string(ledgerdomain.SourceTypeAdjustment),
		string(ledgerdomain.AccountCodeRevenueFlat),
		string(ledgerdomain.AccountCodeRevenueUsage),
	).Scan(&rows).Error; err != nil {
		return nil, err
	}

	return mapSeriesRows(rows), nil
}

func (s *Service) listSubscribersSeries(
	ctx context.Context,
	orgID snowflake.ID,
	start time.Time,
	end time.Time,
	granularity billingoverview.Granularity,
) ([]billingoverview.SeriesPoint, error) {
	periodTrunc, periodInterval, periodFormat := granularitySettings(granularity)
	query := fmt.Sprintf(
		`
		WITH periods AS (
			SELECT generate_series(
				date_trunc('%s', ?::timestamptz),
				date_trunc('%s', ?::timestamptz),
				interval '%s'
			) AS period_start
		),
		period_bounds AS (
			SELECT
				period_start,
				period_start + interval '%s' - interval '1 microsecond' AS period_end
			FROM periods
		)
		SELECT
			to_char(pb.period_start, '%s') AS period,
			COALESCE(COUNT(DISTINCT s.customer_id), 0)::BIGINT AS value
		FROM period_bounds pb
		LEFT JOIN subscriptions s
			ON s.org_id = ?
			AND s.status <> 'DRAFT'
			AND s.start_at <= pb.period_end
			AND (s.end_at IS NULL OR s.end_at > pb.period_end)
			AND (s.cancel_at IS NULL OR s.cancel_at > pb.period_end)
			AND (s.canceled_at IS NULL OR s.canceled_at > pb.period_end)
			AND (s.ended_at IS NULL OR s.ended_at > pb.period_end)
			AND NOT (s.paused_at IS NOT NULL AND s.paused_at <= pb.period_end AND (s.resumed_at IS NULL OR s.resumed_at > pb.period_end))
		GROUP BY pb.period_start
		ORDER BY pb.period_start
		`,
		periodTrunc,
		periodTrunc,
		periodInterval,
		periodInterval,
		periodFormat,
	)

	var rows []seriesRow
	if err := s.db.WithContext(ctx).Raw(
		query,
		start,
		end,
		orgID,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}

	return mapSeriesRows(rows), nil
}

func (s *Service) calculateChurn(ctx context.Context, orgID snowflake.ID, start, end time.Time) (*float64, error) {
	activeAtStart, err := s.countActiveSubscribersAt(ctx, orgID, start)
	if err != nil {
		return nil, err
	}
	if activeAtStart == 0 {
		return nil, nil
	}

	canceled, err := s.countCanceledSubscribers(ctx, orgID, start, end)
	if err != nil {
		return nil, err
	}

	rate := float64(canceled) / float64(activeAtStart)
	return &rate, nil
}

func (s *Service) countCanceledSubscribers(
	ctx context.Context,
	orgID snowflake.ID,
	start time.Time,
	end time.Time,
) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).Raw(
		`SELECT COUNT(DISTINCT customer_id)
		 FROM subscriptions
		 WHERE org_id = ?
		   AND (
			 (canceled_at IS NOT NULL AND canceled_at >= ? AND canceled_at <= ?)
			 OR (ended_at IS NOT NULL AND ended_at >= ? AND ended_at <= ?)
		   )`,
		orgID,
		start,
		end,
		start,
		end,
	).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Service) countActiveSubscribersAt(ctx context.Context, orgID snowflake.ID, at time.Time) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).Raw(
		`SELECT COUNT(DISTINCT customer_id)
		 FROM subscriptions
		 WHERE org_id = ?
		   AND status <> 'DRAFT'
		   AND start_at <= ?
		   AND (end_at IS NULL OR end_at > ?)
		   AND (cancel_at IS NULL OR cancel_at > ?)
		   AND (canceled_at IS NULL OR canceled_at > ?)
		   AND (ended_at IS NULL OR ended_at > ?)
		   AND NOT (paused_at IS NOT NULL AND paused_at <= ? AND (resumed_at IS NULL OR resumed_at > ?))`,
		orgID,
		at,
		at,
		at,
		at,
		at,
		at,
		at,
	).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

type outstandingBalanceRow struct {
	InvoiceCount int64 `gorm:"column:invoice_count"`
	Outstanding  int64 `gorm:"column:outstanding"`
	Overdue      int64 `gorm:"column:overdue"`
}

func (s *Service) loadOutstandingBalance(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	now time.Time,
) (outstandingBalanceRow, error) {
	var row outstandingBalanceRow
	if err := s.db.WithContext(ctx).Raw(
		`
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		)
		SELECT
			COUNT(1) AS invoice_count,
			COALESCE(
				SUM(GREATEST(i.total_amount - COALESCE(s.settled_amount, 0), 0)),
				0
			) AS outstanding,
			COALESCE(
				SUM(
					CASE
						WHEN i.due_at IS NOT NULL AND i.due_at < ?
						THEN GREATEST(i.total_amount - COALESCE(s.settled_amount, 0), 0)
						ELSE 0
					END
				),
				0
			) AS overdue
		FROM invoices i
		LEFT JOIN settled s ON s.invoice_id_text = i.id::text
		WHERE i.org_id = ?
		  AND i.status = 'FINALIZED'
		  AND i.voided_at IS NULL
		  AND i.currency = ?
		`,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		now,
		orgID,
		currency,
	).Scan(&row).Error; err != nil {
		return outstandingBalanceRow{}, err
	}

	return row, nil
}

func (s *Service) loadInvoicedAmount(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	start time.Time,
	end time.Time,
) (int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Raw(
		`
		SELECT COALESCE(SUM(total_amount), 0) AS total
		FROM invoices
		WHERE org_id = ?
		  AND status = 'FINALIZED'
		  AND voided_at IS NULL
		  AND currency = ?
		  AND COALESCE(issued_at, created_at) >= ?
		  AND COALESCE(issued_at, created_at) <= ?
		`,
		orgID,
		currency,
		start,
		end,
	).Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Service) loadCollectedAmount(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	start time.Time,
	end time.Time,
) (int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Raw(
		`
		SELECT COALESCE(
			SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END),
			0
		) AS total
		FROM ledger_entries le
		JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
		JOIN ledger_accounts a ON a.id = l.account_id
		WHERE le.org_id = ?
		  AND le.currency = ?
		  AND le.occurred_at >= ?
		  AND le.occurred_at <= ?
		  AND le.source_type IN (?, ?)
		  AND a.code IN (?, ?)
		`,
		orgID,
		currency,
		start,
		end,
		string(ledgerdomain.SourceTypeBillingCycle),
		string(ledgerdomain.SourceTypeAdjustment),
		string(ledgerdomain.AccountCodeRevenueFlat),
		string(ledgerdomain.AccountCodeRevenueUsage),
	).Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

type seriesRow struct {
	Period string `gorm:"column:period"`
	Value  int64  `gorm:"column:value"`
}

func mapSeriesRows(rows []seriesRow) []billingoverview.SeriesPoint {
	points := make([]billingoverview.SeriesPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, billingoverview.SeriesPoint{
			Period: row.Period,
			Value:  row.Value,
		})
	}
	return points
}

func granularitySettings(granularity billingoverview.Granularity) (string, string, string) {
	switch granularity {
	case billingoverview.GranularityMonth:
		return "month", "1 month", "YYYY-MM"
	default:
		return "day", "1 day", "YYYY-MM-DD"
	}
}
