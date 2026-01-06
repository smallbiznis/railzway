package snapshot

import (
	"context"
	"strings"
	"time"

	"github.com/smallbiznis/valora/internal/clock"
	meterdomain "github.com/smallbiznis/valora/internal/meter/domain"
	"github.com/smallbiznis/valora/internal/observability/metrics"
	subscriptiondomain "github.com/smallbiznis/valora/internal/subscription/domain"
	usagedomain "github.com/smallbiznis/valora/internal/usage/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB               *gorm.DB
	Log              *zap.Logger
	Clock            clock.Clock
	MeterRepo        meterdomain.Repository
	SubscriptionRepo subscriptiondomain.Repository
	UsageRepo        usagedomain.SnapshotRepository
	Config           Config `optional:"true"`
}

type Worker struct {
	db               *gorm.DB
	log              *zap.Logger
	clock            clock.Clock
	meterRepo        meterdomain.Repository
	subscriptionRepo subscriptiondomain.Repository
	usageRepo        usagedomain.SnapshotRepository
	cfg              Config
}

func NewWorker(p Params) *Worker {
	cfg := p.Config.withDefaults()
	return &Worker{
		db:               p.DB,
		log:              p.Log.Named("usage.snapshot"),
		clock:            p.Clock,
		meterRepo:        p.MeterRepo,
		subscriptionRepo: p.SubscriptionRepo,
		usageRepo:        p.UsageRepo,
		cfg:              cfg,
	}
}

func (w *Worker) RunForever(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		if err := w.RunOnce(ctx); err != nil {
			w.log.Warn("usage snapshot run failed", zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) RunOnce(parentCtx context.Context) error {
	ctx, cancel := context.WithTimeout(parentCtx, w.cfg.RunTimeout)
	defer cancel()

	_, err := w.processBatch(ctx, w.cfg.BatchSize)
	return err
}

func (w *Worker) processBatch(ctx context.Context, limit int) (int, error) {
	var rows []usagedomain.SnapshotCandidate

	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		rows, err = w.usageRepo.LockAccepted(ctx, tx, limit)
		return err
	})
	if err != nil {
		return 0, err
	}

	if len(rows) == 0 {
		return 0, nil
	}

	processed := 0
	now := w.clock.Now()

	for _, row := range rows {
		rowCtx, cancel := context.WithTimeout(ctx, w.cfg.RowTimeout)
		err := w.db.WithContext(rowCtx).Transaction(func(tx *gorm.DB) error {
			update, err := w.buildSnapshot(rowCtx, tx, row, now)
			if err != nil {
				return err
			}
			return w.usageRepo.UpdateSnapshot(rowCtx, tx, update)
		})
		cancel()

		if err != nil {
			w.log.Warn("snapshot row failed",
				zap.Error(err),
				zap.String("usage_id", row.ID.String()),
			)
			metrics.Snapshot().IncSnapshotProcessed("failed")
			continue
		}

		lag := now.Sub(row.RecordedAt)
		snapshot := metrics.Snapshot()
		snapshot.ObserveSnapshotLag(lag)
		snapshot.IncSnapshotProcessed("success")

		processed++
	}

	w.updateBacklogMetrics(ctx)

	return processed, nil
}

func (w *Worker) buildSnapshot(
	ctx context.Context,
	tx *gorm.DB,
	row usagedomain.SnapshotCandidate,
	now time.Time,
) (usagedomain.SnapshotUpdate, error) {
	update := usagedomain.SnapshotUpdate{
		ID:         row.ID,
		Status:     usagedomain.UsageStatusEnriched,
		SnapshotAt: now,
	}

	meterCode := strings.TrimSpace(row.MeterCode)
	meter, err := w.meterRepo.FindByCode(ctx, tx, row.OrgID, meterCode)
	if err != nil {
		return update, err
	}
	if meter == nil {
		update.Status = usagedomain.UsageStatusUnmatchedMeter
		return update, nil
	}

	subscription, err := w.subscriptionRepo.FindActiveByCustomerIDAt(ctx, tx, row.OrgID, row.CustomerID, row.RecordedAt)
	if err != nil {
		return update, err
	}
	if subscription == nil {
		update.Status = usagedomain.UsageStatusUnmatchedSubscription
		return update, nil
	}
	update.SubscriptionID = subscription.ID

	item, err := w.subscriptionRepo.FindSubscriptionItemByMeterIDAt(ctx, tx, row.OrgID, subscription.ID, meter.ID, row.RecordedAt)
	if err != nil {
		return update, err
	}

	if item != nil && item.ID != 0 {
		itemID := item.ID
		update.SubscriptionItemID = &itemID

		meterID := item.MeterID
		update.MeterID = meterID
	}

	return update, nil
}

func (w *Worker) updateBacklogMetrics(ctx context.Context) {
	snapshot := metrics.Snapshot()

	type row struct {
		Status   string
		Count    int
		OldestAt *time.Time
	}

	var rows []row
	err := w.db.WithContext(ctx).Raw(`
		SELECT status, COUNT(*) AS count, MIN(recorded_at) AS oldest_at
		FROM usage_events
		WHERE status IN (
			'accepted',
			'unmatched_subscription',
			'unmatched_meter'
		)
		GROUP BY status
	`).Scan(&rows).Error
	if err != nil {
		w.log.Warn("failed to update snapshot backlog metrics", zap.Error(err))
		return
	}

	// reset known statuses to 0 first (important!)
	snapshot.SetBacklog("accepted", 0)
	snapshot.SetBacklog("unmatched_subscription", 0)
	snapshot.SetBacklog("unmatched_meter", 0)

	for _, r := range rows {
		snapshot.SetBacklog(r.Status, r.Count)
		if r.OldestAt != nil {
			age := w.clock.Now().Sub(*r.OldestAt)
			snapshot.SetBacklogOldest(r.Status, age)
		}
	}
}
