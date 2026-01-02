package snapshot

import (
	"context"
	"errors"
	"strings"
	"time"

	meterdomain "github.com/smallbiznis/valora/internal/meter/domain"
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
	MeterRepo        meterdomain.Repository
	SubscriptionRepo subscriptiondomain.Repository
	UsageRepo        usagedomain.SnapshotRepository
	Config           Config `optional:"true"`
}

type Worker struct {
	db               *gorm.DB
	log              *zap.Logger
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
		if err := w.RunOnce(); err != nil {
			w.log.Warn("usage snapshot run failed", zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) RunOnce() error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := w.processBatch(ctx, w.cfg.BatchSize)
	return err
}

func (w *Worker) processBatch(ctx context.Context, limit int) (int, error) {
	if w.db == nil || w.usageRepo == nil || w.meterRepo == nil || w.subscriptionRepo == nil {
		return 0, errors.New("snapshot_worker_unavailable")
	}
	if limit <= 0 {
		limit = w.cfg.BatchSize
	}

	processed := 0
	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows, err := w.usageRepo.LockAccepted(ctx, tx, limit)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		now := time.Now().UTC()
		for _, row := range rows {
			update, err := w.buildSnapshot(ctx, tx, row, now)
			if err != nil {
				return err
			}
			if err := w.usageRepo.UpdateSnapshot(ctx, tx, update); err != nil {
				return err
			}
			processed++
		}
		return nil
	})
	if err != nil {
		return processed, err
	}
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
