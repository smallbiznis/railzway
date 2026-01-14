package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	auditdomain "github.com/smallbiznis/railzway/internal/audit/domain"
	auditcontext "github.com/smallbiznis/railzway/internal/auditcontext"
	"github.com/smallbiznis/railzway/internal/authorization"
	billingcycledomain "github.com/smallbiznis/railzway/internal/billingcycle/domain"
	"github.com/smallbiznis/railzway/internal/billingdashboard/rollup"
	billingopsdomain "github.com/smallbiznis/railzway/internal/billingoperations/domain"
	"github.com/smallbiznis/railzway/internal/clock"
	"github.com/smallbiznis/railzway/internal/cloudmetrics"
	invoicedomain "github.com/smallbiznis/railzway/internal/invoice/domain"
	ledgerdomain "github.com/smallbiznis/railzway/internal/ledger/domain"
	obsmetrics "github.com/smallbiznis/railzway/internal/observability/metrics"
	"github.com/smallbiznis/railzway/internal/orgcontext"
	ratingdomain "github.com/smallbiznis/railzway/internal/rating/domain"
	"github.com/smallbiznis/railzway/internal/scheduler/guard"
	subscriptiondomain "github.com/smallbiznis/railzway/internal/subscription/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB              *gorm.DB
	Log             *zap.Logger
	RatingSvc       ratingdomain.Service
	InvoiceSvc      invoicedomain.Service
	LedgerSvc       ledgerdomain.Service
	SubscriptionSvc subscriptiondomain.Service
	AuditSvc        auditdomain.Service

	AuthzSvc             authorization.Service
	BillingOperationsSvc billingopsdomain.Service
	RollupSvc            *rollup.Service `optional:"true"`
	GenID                *snowflake.Node
	Clock                clock.Clock
	Config               Config                     `optional:"true"`
	CloudMetrics         *cloudmetrics.CloudMetrics `optional:"true"`
}

type Scheduler struct {
	db                   *gorm.DB
	log                  *zap.Logger
	cfg                  Config
	genID                *snowflake.Node
	clock                clock.Clock
	ratingSvc            ratingdomain.Service
	invoiceSvc           invoicedomain.Service
	ledgerSvc            ledgerdomain.Service
	subscriptionSvc      subscriptiondomain.Service
	auditSvc             auditdomain.Service
	authzSvc             authorization.Service
	billingOperationsSvc billingopsdomain.Service
	rollupSvc            *rollup.Service
	cloudMetrics         *cloudmetrics.CloudMetrics
}

type auditEvent struct {
	OrgID          snowflake.ID
	Action         string
	TargetType     string
	TargetID       string
	SubscriptionID string
	BillingCycleID string
	Metadata       map[string]any
}

func New(p Params) (*Scheduler, error) {
	if p.DB == nil || p.Log == nil || p.RatingSvc == nil || p.InvoiceSvc == nil || p.LedgerSvc == nil || p.SubscriptionSvc == nil || p.GenID == nil || p.AuditSvc == nil || p.AuthzSvc == nil || p.Clock == nil || p.BillingOperationsSvc == nil {
		return nil, ErrInvalidConfig
	}
	cfg := p.Config.withDefaults()
	return &Scheduler{
		db:                   p.DB,
		log:                  p.Log.Named("scheduler").With(zap.String("component", "scheduler")),
		cfg:                  cfg,
		genID:                p.GenID,
		clock:                p.Clock,
		ratingSvc:            p.RatingSvc,
		invoiceSvc:           p.InvoiceSvc,
		ledgerSvc:            p.LedgerSvc,
		subscriptionSvc:      p.SubscriptionSvc,
		auditSvc:             p.AuditSvc,
		authzSvc:             p.AuthzSvc,
		billingOperationsSvc: p.BillingOperationsSvc,
		rollupSvc:            p.RollupSvc,
		cloudMetrics:         p.CloudMetrics,
	}, nil
}

func (s *Scheduler) runJob(
	parent context.Context,
	name string,
	batchSize int,
	timeout time.Duration,
	fn func(ctx context.Context) error,
) error {
	start := s.clock.Now()
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	ctx = auditcontext.WithActor(ctx, string(auditdomain.ActorTypeSystem), "scheduler")
	ctx, run, owner := s.ensureJobRun(ctx, name, batchSize)
	if owner {
		s.logJobStart(ctx, run)
	}
	log := s.logger(ctx).With(
		zap.String("job", name),
		zap.String("run_id", run.runID),
	)
	schedMetrics := obsmetrics.Scheduler()
	schedMetrics.IncJobRun(name)

	err := fn(ctx)
	duration := time.Since(start)
	schedMetrics.ObserveJobDuration(name, duration)
	if s.cloudMetrics != nil {
		go s.cloudMetrics.SetSchedulerJobDuration(name, duration)
	}
	if owner {
		if err != nil && run != nil && run.errorCount == 0 {
			run.IncError()
		}
		s.logJobFinish(ctx, run)
	}
	if err == nil {
		return nil
	}

	// ✅ treat deadline as soft-timeout
	isTimeout := errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
	if isTimeout {
		schedMetrics.IncJobTimeout(name)
	}
	schedMetrics.IncJobError(name, err)
	if isTimeout {
		log.Warn("job timed out",
			zap.Duration("timeout", timeout),
			zap.Error(err),
		)
		return nil
	}

	return fmt.Errorf("%s: %w", name, err)
}

func (s *Scheduler) RunOnce(parent context.Context) error {
	var err error

	jobs := []struct {
		Name    string
		Enabled bool
		Run     func(context.Context) error
	}{
		{"ensure_cycles", s.isJobEnabled("ensure_cycles"), func(ctx context.Context) error {
			return s.runJob(ctx, "ensure_cycles", s.cfg.BatchSize, 30*time.Second, s.EnsureBillingCyclesJob)
		}},
		{"close_cycles", s.isJobEnabled("close_cycles"), func(ctx context.Context) error {
			return s.runJob(ctx, "close_cycles", s.cfg.MaxCloseBatchSize, 30*time.Second, s.CloseCyclesJob)
		}},
		{"rating", s.isJobEnabled("rating"), func(ctx context.Context) error {
			return s.runJob(ctx, "rating", s.cfg.MaxRatingBatchSize, 30*time.Second, s.RatingJob)
		}},
		{"close_after_rating", s.isJobEnabled("close_after_rating"), func(ctx context.Context) error {
			return s.runJob(ctx, "close_after_rating", s.cfg.MaxCloseBatchSize, 30*time.Second, s.CloseAfterRatingJob)
		}},
		{"invoice", s.isJobEnabled("invoice"), func(ctx context.Context) error {
			return s.runJob(ctx, "invoice", s.cfg.MaxInvoiceBatchSize, 30*time.Second, s.InvoiceJob)
		}},
	}

	for _, job := range jobs {
		if job.Enabled {
			err = errors.Join(err, job.Run(parent))
		}
	}

	if s.rollupSvc != nil {
		if s.isJobEnabled("rollup_rebuild") {
			err = errors.Join(err, s.runJob(parent, "rollup_rebuild", s.cfg.BatchSize, 30*time.Minute, func(ctx context.Context) error {
				return s.rollupSvc.ProcessRebuildRequests(ctx, s.cfg.BatchSize)
			}))
		}
		if s.isJobEnabled("rollup_pending") {
			err = errors.Join(err, s.runJob(parent, "rollup_pending", s.cfg.BatchSize, 30*time.Second, func(ctx context.Context) error {
				return s.rollupSvc.ProcessPending(ctx, s.cfg.BatchSize)
			}))
		}
	}

	otherJobs := []struct {
		Name    string
		Enabled bool
		Run     func(context.Context) error
	}{
		{"end_canceled_subs", s.isJobEnabled("end_canceled_subs"), func(ctx context.Context) error {
			return s.runJob(ctx, "end_canceled_subs", s.cfg.BatchSize, 30*time.Second, s.EndCanceledSubscriptionsJob)
		}},
		{"recovery_sweep", s.isJobEnabled("recovery_sweep"), func(ctx context.Context) error {
			return s.runJob(ctx, "recovery_sweep", maxInt(s.cfg.MaxRatingBatchSize, s.cfg.MaxCloseBatchSize, s.cfg.MaxInvoiceBatchSize), 30*time.Second, s.RecoverySweepJob)
		}},
		{"sla_evaluation", s.isJobEnabled("sla_evaluation"), func(ctx context.Context) error {
			return s.runJob(ctx, "sla_evaluation", s.cfg.BatchSize, 30*time.Second, s.SLAEvaluationJob)
		}},
		{"finops_scoring", s.isJobEnabled("finops_scoring"), func(ctx context.Context) error {
			return s.runJob(ctx, "finops_scoring", 1, 24*time.Hour, s.FinOpsScoringJob)
		}},
	}

	for _, job := range otherJobs {
		if job.Enabled {
			err = errors.Join(err, job.Run(parent))
		}
	}

	return err
}

func (s *Scheduler) RunForever(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.RunInterval)
	defer ticker.Stop()
	nextRun := s.clock.Now().Add(s.cfg.RunInterval)
	schedMetrics := obsmetrics.Scheduler()

	for {
		runLag := time.Since(nextRun)
		if runLag > 0 {
			schedMetrics.ObserveRunLoopLag(runLag)
		}
		if err := s.RunOnce(ctx); err != nil {
			s.log.Warn("scheduler run failed", zap.Error(err))
		}
		nextRun = nextRun.Add(s.cfg.RunInterval)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Scheduler) isJobEnabled(jobName string) bool {
	// If EnabledJobs is empty, all jobs are enabled by default (monolith mode)
	if len(s.cfg.EnabledJobs) == 0 {
		return true
	}
	for _, enabled := range s.cfg.EnabledJobs {
		if strings.EqualFold(enabled, jobName) {
			return true
		}
	}
	return false
}

func (s *Scheduler) EnsureBillingCyclesJob(ctx context.Context) error {
	ctx, run, owner := s.ensureJobRun(ctx, "ensure_cycles", s.cfg.BatchSize)
	if owner {
		s.logJobStart(ctx, run)
		defer s.logJobFinish(ctx, run)
	}
	now := s.clock.Now()
	var jobErr error

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		processed, batchErr := s.ensureBillingCyclesBatch(ctx, now, run)
		if batchErr != nil {
			jobErr = errors.Join(jobErr, batchErr)
		}
		run.AddProcessed(processed)
		if processed == 0 {
			break
		}
	}

	return jobErr
}

func (s *Scheduler) CloseCyclesJob(ctx context.Context) error {
	ctx, run, owner := s.ensureJobRun(ctx, "close_cycles", s.cfg.MaxCloseBatchSize)
	if owner {
		s.logJobStart(ctx, run)
		defer s.logJobFinish(ctx, run)
	}
	now := s.clock.Now()
	fmt.Printf("CloseCycle: %v\n", now)
	var jobErr error

	for {
		cycles, err := s.fetchBillingCyclesForWork(ctx, `status = ? AND period_end <= ?`, []any{billingcycledomain.BillingCycleStatusOpen, now}, s.cfg.MaxCloseBatchSize)
		if err != nil {
			s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "close_cycles", 0, err)
			return err
		}
		if len(cycles) == 0 {
			break
		}

		for _, cycle := range cycles {
			s.logCycleClaimed(ctx, "close_cycles", cycle)
			if err := s.authorizeSystem(ctx, cycle.OrgID, authorization.ObjectBillingCycle, authorization.ActionBillingCycleStartClosing); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.authorize.failed", "close_cycles", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				continue
			}
			updated, err := s.markCycleClosing(ctx, cycle.ID, now)
			if err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "close_cycles", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageCloseCycles, err)
				continue
			}
			if updated {
				run.AddProcessed(1)
				if err := s.upsertBillingCycleStats(ctx, s.db, cycle.ID, cycle.OrgID, cycle.PeriodStart, billingcycledomain.BillingCycleStatusClosing, now); err != nil {
					jobErr = errors.Join(jobErr, err)
					s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "close_cycles", cycle.OrgID, err,
						zap.String("cycle_id", idString(cycle.ID)),
						zap.String("subscription_id", idString(cycle.SubscriptionID)),
					)
				}
				s.emitAuditEvent(ctx, auditEvent{
					OrgID:          cycle.OrgID,
					Action:         "billing_cycle.closing_started",
					TargetType:     "billing_cycle",
					TargetID:       cycle.ID.String(),
					SubscriptionID: cycle.SubscriptionID.String(),
					BillingCycleID: cycle.ID.String(),
					Metadata: map[string]any{
						"period_end": cycle.PeriodEnd.Format(time.RFC3339),
					},
				})
			}
		}
	}

	return jobErr
}

func (s *Scheduler) RatingJob(ctx context.Context) error {
	ctx, run, owner := s.ensureJobRun(ctx, "rating", s.cfg.MaxRatingBatchSize)
	if owner {
		s.logJobStart(ctx, run)
		defer s.logJobFinish(ctx, run)
	}
	now := s.clock.Now()
	var jobErr error

	for {
		cycles, err := s.fetchBillingCyclesForWork(ctx, `status = ? AND rating_completed_at IS NULL`, []any{billingcycledomain.BillingCycleStatusClosing}, s.cfg.MaxRatingBatchSize)
		if err != nil {
			s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "rating", 0, err)
			return err
		}
		if len(cycles) == 0 {
			break
		}

		for _, cycle := range cycles {
			s.logCycleClaimed(ctx, "rating", cycle)
			if err := s.authorizeSystem(ctx, cycle.OrgID, authorization.ObjectBillingCycle, authorization.ActionBillingCycleRate); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.authorize.failed", "rating", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				continue
			}
			cycleCtx := s.withAuditContext(ctx, cycle.SubscriptionID.String(), cycle.ID.String())
			s.emitAuditEvent(cycleCtx, auditEvent{
				OrgID:          cycle.OrgID,
				Action:         "rating.started",
				TargetType:     "billing_cycle",
				TargetID:       cycle.ID.String(),
				SubscriptionID: cycle.SubscriptionID.String(),
				BillingCycleID: cycle.ID.String(),
				Metadata: map[string]any{
					"period_start": cycle.PeriodStart.Format(time.RFC3339),
					"period_end":   cycle.PeriodEnd.Format(time.RFC3339),
				},
			})

			if err := s.ratingSvc.RunRating(cycleCtx, cycle.ID.String()); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "rating", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				if s.cloudMetrics != nil {
					go s.cloudMetrics.IncEngineError(cycle.OrgID.String(), "rating")
				}
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageRating, err)
				s.emitAuditEvent(cycleCtx, auditEvent{
					OrgID:          cycle.OrgID,
					Action:         "rating.failed",
					TargetType:     "billing_cycle",
					TargetID:       cycle.ID.String(),
					SubscriptionID: cycle.SubscriptionID.String(),
					BillingCycleID: cycle.ID.String(),
					Metadata: map[string]any{
						"error": err.Error(),
					},
				})
				continue
			}

			if err := s.markRatingCompleted(ctx, cycle.ID, now); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "rating", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageRating, err)
				s.emitAuditEvent(cycleCtx, auditEvent{
					OrgID:          cycle.OrgID,
					Action:         "rating.failed",
					TargetType:     "billing_cycle",
					TargetID:       cycle.ID.String(),
					SubscriptionID: cycle.SubscriptionID.String(),
					BillingCycleID: cycle.ID.String(),
					Metadata: map[string]any{
						"error": err.Error(),
					},
				})
				continue
			}

			run.AddProcessed(1)
			s.emitAuditEvent(cycleCtx, auditEvent{
				OrgID:          cycle.OrgID,
				Action:         "rating.completed",
				TargetType:     "billing_cycle",
				TargetID:       cycle.ID.String(),
				SubscriptionID: cycle.SubscriptionID.String(),
				BillingCycleID: cycle.ID.String(),
				Metadata: map[string]any{
					"period_start": cycle.PeriodStart.Format(time.RFC3339),
					"period_end":   cycle.PeriodEnd.Format(time.RFC3339),
				},
			})
		}
	}

	return jobErr
}

func (s *Scheduler) CloseAfterRatingJob(ctx context.Context) error {
	ctx, run, owner := s.ensureJobRun(ctx, "close_after_rating", s.cfg.MaxCloseBatchSize)
	if owner {
		s.logJobStart(ctx, run)
		defer s.logJobFinish(ctx, run)
	}
	now := s.clock.Now()
	var jobErr error

	for {
		cycles, err := s.fetchBillingCyclesForWork(ctx, `status = ? AND rating_completed_at IS NOT NULL`, []any{billingcycledomain.BillingCycleStatusClosing}, s.cfg.MaxCloseBatchSize)
		if err != nil {
			s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "close_after_rating", 0, err)
			return err
		}
		if len(cycles) == 0 {
			break
		}

		for _, cycle := range cycles {
			s.logCycleClaimed(ctx, "close_after_rating", cycle)
			if err := s.authorizeSystem(ctx, cycle.OrgID, authorization.ObjectBillingCycle, authorization.ActionBillingCycleClose); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.authorize.failed", "close_after_rating", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				continue
			}
			hasResults, err := s.hasRatingResults(ctx, cycle.ID)
			if err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "close_after_rating", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageCloseAfterRating, err)
				continue
			}
			if !hasResults {
				jobErr = errors.Join(jobErr, invoicedomain.ErrMissingRatingResults)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "close_after_rating", cycle.OrgID, invoicedomain.ErrMissingRatingResults,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageCloseAfterRating, invoicedomain.ErrMissingRatingResults)
				continue
			}

			cycleCtx := s.withAuditContext(ctx, cycle.SubscriptionID.String(), cycle.ID.String())
			if err := s.ensureLedgerEntryForCycle(cycleCtx, cycle); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "close_after_rating", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageCloseAfterRating, err)
				continue
			}

			updated, err := s.markCycleClosed(ctx, cycle.ID, now)
			if err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "close_after_rating", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageCloseAfterRating, err)
				continue
			}
			if updated {
				run.AddProcessed(1)
				if err := s.upsertBillingCycleStats(ctx, s.db, cycle.ID, cycle.OrgID, cycle.PeriodStart, billingcycledomain.BillingCycleStatusClosed, now); err != nil {
					jobErr = errors.Join(jobErr, err)
					s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "close_after_rating", cycle.OrgID, err,
						zap.String("cycle_id", idString(cycle.ID)),
						zap.String("subscription_id", idString(cycle.SubscriptionID)),
					)
				}
				s.emitAuditEvent(cycleCtx, auditEvent{
					OrgID:          cycle.OrgID,
					Action:         "billing_cycle.closed",
					TargetType:     "billing_cycle",
					TargetID:       cycle.ID.String(),
					SubscriptionID: cycle.SubscriptionID.String(),
					BillingCycleID: cycle.ID.String(),
					Metadata: map[string]any{
						"period_end": cycle.PeriodEnd.Format(time.RFC3339),
					},
				})
			}
		}
	}

	return jobErr
}

func (s *Scheduler) InvoiceJob(ctx context.Context) error {
	ctx, run, owner := s.ensureJobRun(ctx, "invoice", s.cfg.MaxInvoiceBatchSize)
	if owner {
		s.logJobStart(ctx, run)
		defer s.logJobFinish(ctx, run)
	}

	now := s.clock.Now()
	var jobErr error
	schedMetrics := obsmetrics.Scheduler()

	for {
		cycles, err := s.fetchBillingCyclesForWork(ctx, `status = ? AND invoiced_at IS NULL`, []any{billingcycledomain.BillingCycleStatusClosed}, s.cfg.MaxInvoiceBatchSize)
		if err != nil {
			s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "invoice", 0, err)
			return err
		}
		if len(cycles) == 0 {
			break
		}

		for _, cycle := range cycles {
			s.logCycleClaimed(ctx, "invoice", cycle)
			if err := s.authorizeSystem(ctx, cycle.OrgID, authorization.ObjectInvoice, authorization.ActionInvoiceGenerate); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.authorize.failed", "invoice", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				continue
			}

			cycleCtx := s.withAuditContext(ctx, cycle.SubscriptionID.String(), cycle.ID.String())
			invoice, err := s.invoiceSvc.GenerateInvoice(cycleCtx, cycle.ID.String())
			if err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "invoice.generate.failed", "invoice", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				if s.cloudMetrics != nil {
					go s.cloudMetrics.IncEngineError(cycle.OrgID.String(), "invoice_generation")
				}
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageInvoice, err)
				continue
			}

			if invoice == nil {
				continue
			}

			s.logInvoiceGenerated(ctx, cycle, invoice.ID)
			if s.cloudMetrics != nil {
				go s.cloudMetrics.IncInvoiceGenerated(cycle.OrgID.String())
			}

			if err := s.markCycleInvoiced(ctx, cycle.ID, now); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "invoice", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("invoice_id", idString(invoice.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageInvoice, err)
			} else {
				run.AddProcessed(1)
				schedMetrics.IncBillingCycleTransition(
					string(billingcycledomain.BillingCycleStatusClosed),
					obsmetrics.BillingCycleTransitionInvoiced,
				)
			}

			if !s.cfg.FinalizeInvoices {
				continue
			}

			switch invoice.Status {
			case invoicedomain.InvoiceStatusDraft:
				if err := s.authorizeSystem(ctx, cycle.OrgID, authorization.ObjectInvoice, authorization.ActionInvoiceFinalize); err != nil {
					jobErr = errors.Join(jobErr, err)
					s.logSchedulerError(ctx, run, "scheduler.authorize.failed", "invoice", cycle.OrgID, err,
						zap.String("cycle_id", idString(cycle.ID)),
						zap.String("invoice_id", idString(invoice.ID)),
						zap.String("subscription_id", idString(cycle.SubscriptionID)),
					)
					continue
				}
				if err := s.invoiceSvc.FinalizeInvoice(cycleCtx, invoice.ID.String()); err != nil {
					jobErr = errors.Join(jobErr, err)
					s.logSchedulerError(ctx, run, "invoice.finalize.failed", "invoice", cycle.OrgID, err,
						zap.String("cycle_id", idString(cycle.ID)),
						zap.String("invoice_id", idString(invoice.ID)),
						zap.String("subscription_id", idString(cycle.SubscriptionID)),
					)
					_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageInvoice, err)
					continue
				}
				s.logInvoiceFinalized(ctx, cycle, invoice.ID)
				if err := s.markCycleInvoiceFinalized(ctx, cycle.ID, now); err != nil {
					jobErr = errors.Join(jobErr, err)
					s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "invoice", cycle.OrgID, err,
						zap.String("cycle_id", idString(cycle.ID)),
						zap.String("invoice_id", idString(invoice.ID)),
						zap.String("subscription_id", idString(cycle.SubscriptionID)),
					)
					_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageInvoice, err)
				}
			case invoicedomain.InvoiceStatusFinalized:
				if err := s.markCycleInvoiceFinalized(ctx, cycle.ID, now); err != nil {
					jobErr = errors.Join(jobErr, err)
					s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "invoice", cycle.OrgID, err,
						zap.String("cycle_id", idString(cycle.ID)),
						zap.String("invoice_id", idString(invoice.ID)),
						zap.String("subscription_id", idString(cycle.SubscriptionID)),
					)
					_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageInvoice, err)
				}
			}
		}
	}

	return jobErr
}

func (s *Scheduler) EndCanceledSubscriptionsJob(ctx context.Context) error {
	ctx, run, owner := s.ensureJobRun(ctx, "end_canceled_subs", s.cfg.BatchSize)
	if owner {
		s.logJobStart(ctx, run)
		defer s.logJobFinish(ctx, run)
	}
	var jobErr error

	for {
		subscriptions, err := s.FetchSubscriptionsForWork(ctx, subscriptiondomain.SubscriptionStatusCanceled, s.cfg.BatchSize)
		if err != nil {
			s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "end_canceled_subs", 0, err)
			return err
		}
		if len(subscriptions) == 0 {
			break
		}

		for _, subscription := range subscriptions {
			if ctx.Err() != nil {
				jobErr = errors.Join(jobErr, err)
				continue
			}

			canEnd, err := s.canEndSubscription(ctx, subscription.OrgID, subscription.ID)
			if err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "end_canceled_subs", subscription.OrgID, err,
					zap.String("subscription_id", idString(subscription.ID)),
				)
				continue
			}
			if !canEnd {
				continue
			}

			if err := s.authorizeSystem(ctx, subscription.OrgID, authorization.ObjectSubscription, authorization.ActionSubscriptionEnd); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.authorize.failed", "end_canceled_subs", subscription.OrgID, err,
					zap.String("subscription_id", idString(subscription.ID)),
				)
				continue
			}

			ctxWithOrg := orgcontext.WithOrgID(ctx, int64(subscription.OrgID))
			ctxWithAudit := s.withAuditContext(ctxWithOrg, subscription.ID.String(), "")
			if err := s.subscriptionSvc.TransitionSubscription(ctxWithAudit, subscription.ID.String(), subscriptiondomain.SubscriptionStatusEnded, subscriptiondomain.TransitionReason("scheduler")); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "end_canceled_subs", subscription.OrgID, err,
					zap.String("subscription_id", idString(subscription.ID)),
				)
				continue
			}
			run.AddProcessed(1)

			s.emitAuditEvent(ctxWithAudit, auditEvent{
				OrgID:          subscription.OrgID,
				Action:         "subscription.end",
				TargetType:     "subscription",
				TargetID:       subscription.ID.String(),
				SubscriptionID: subscription.ID.String(),
				Metadata: map[string]any{
					"reason": "scheduler",
				},
			})
		}
	}

	return jobErr
}

func (s *Scheduler) ensureBillingCyclesBatch(ctx context.Context, now time.Time, run *jobRun) (int, error) {
	var batchErr error
	events := make([]auditEvent, 0)
	schedMetrics := obsmetrics.Scheduler()
	jobName := "ensure_cycles"

	// 1) Ambil batch subscription dalam TX pendek (untuk lock/claim work)
	var subs []WorkSubscription
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var err error
		subs, err = s.fetchSubscriptionsNeedingCycle(ctx, tx, s.cfg.BatchSize)
		return err
	})
	if err != nil {
		schedMetrics.IncBatchDeferred(jobName, classifyEnsureCyclesDeferredReason(err))
		s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", jobName, 0, err)
		return 0, err
	}
	if len(subs) == 0 {
		schedMetrics.IncBatchDeferred(jobName, obsmetrics.SchedulerBatchDeferredReasonSkipLockedEmpty)
		return 0, nil
	}

	processed := 0

	// 2) Proses per subscription dalam TX kecil
	for _, sub := range subs {
		if ctx.Err() != nil {
			// stop gracefully; jangan lanjut bikin error rantai
			batchErr = errors.Join(batchErr, ctx.Err())
			schedMetrics.IncBatchDeferred(jobName, classifyEnsureCyclesDeferredReason(ctx.Err()))
			break
		}

		if err := s.authorizeSystem(ctx, sub.OrgID, authorization.ObjectBillingCycle, authorization.ActionBillingCycleOpen); err != nil {
			batchErr = errors.Join(batchErr, err)
			s.logSchedulerError(ctx, run, "scheduler.authorize.failed", jobName, sub.OrgID, err,
				zap.String("subscription_id", idString(sub.ID)),
			)
			continue
		}

		// ⚠️ collect events per sub agar tidak race
		subEvents := make([]auditEvent, 0, 2)
		txErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return s.ensureSubscriptionCycle(ctx, tx, sub, now, &subEvents)
		})
		if txErr != nil {
			batchErr = errors.Join(batchErr, txErr)
			schedMetrics.IncBatchDeferred(jobName, classifyEnsureCyclesDeferredReason(txErr))
			s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", jobName, sub.OrgID, txErr,
				zap.String("subscription_id", idString(sub.ID)),
			)
			s.logger(s.withLogContext(ctx, sub.OrgID)).Info("ensure billing cycle deferred",
				zap.String("subscription_id", idString(sub.ID)),
				zap.Error(txErr),
			)
			continue
		}

		processed++
		events = append(events, subEvents...)
	}

	// 3) Emit audit events di luar transaction
	for _, ev := range events {
		if ctx.Err() != nil {
			batchErr = errors.Join(batchErr, ctx.Err())
			schedMetrics.IncBatchDeferred(jobName, classifyEnsureCyclesDeferredReason(ctx.Err()))
			break
		}
		s.emitAuditEvent(ctx, ev)
	}

	if processed > 0 {
		schedMetrics.IncBatchProcessed(jobName)
		schedMetrics.AddBatchProcessed(jobName, "subscriptions", processed)
	}
	return processed, batchErr
}

func classifyEnsureCyclesDeferredReason(err error) string {
	if err == nil {
		return obsmetrics.SchedulerJobReasonUnknown
	}
	reason := obsmetrics.ClassifySchedulerJobReason(err)
	if reason == obsmetrics.SchedulerJobReasonUnknown {
		return obsmetrics.SchedulerJobReasonUnknown
	}
	return reason
}

func maxInt(values ...int) int {
	max := 0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

func (s *Scheduler) ensureSubscriptionCycle(ctx context.Context, tx *gorm.DB, subscription WorkSubscription, now time.Time, events *[]auditEvent) error {
	// NO authorize here
	if err := guard.EnsureSubscriptionCanOpenBillingCycle(subscription.Status, subscription.ActivatedAt, subscription.BillingCycleType); err != nil {
		return err
	}

	openCycle, openCount, err := s.findOpenCycle(ctx, tx, subscription.OrgID, subscription.ID)
	if err != nil {
		return err
	}
	if openCount > 1 {
		return billingcycledomain.ErrMultipleOpenCycles
	}

	if openCycle != nil {
		if err := guard.EnsureBillingCycleCanClose(openCycle.Status, openCycle.PeriodEnd, now); err == nil {
			if err := s.authorizeSystem(ctx, subscription.OrgID, authorization.ObjectBillingCycle, authorization.ActionBillingCycleStartClosing); err != nil {
				return err
			}
			updated, err := s.markCycleClosingTx(ctx, tx, openCycle.ID, now)
			if err != nil {
				return err
			}
			if updated {
				*events = append(*events, auditEvent{
					OrgID:          subscription.OrgID,
					Action:         "billing_cycle.closing_started",
					TargetType:     "billing_cycle",
					TargetID:       openCycle.ID.String(),
					SubscriptionID: subscription.ID.String(),
					BillingCycleID: openCycle.ID.String(),
					Metadata: map[string]any{
						"period_end": openCycle.PeriodEnd.Format(time.RFC3339),
					},
				})
			}
			return nil
		}
		return nil
	}

	lastCycle, err := s.findLastCycle(ctx, tx, subscription.OrgID, subscription.ID)
	if err != nil {
		return err
	}

	periodStart := *subscription.ActivatedAt
	if lastCycle != nil && lastCycle.PeriodEnd.After(periodStart) {
		periodStart = lastCycle.PeriodEnd
	}
	if periodStart.After(now) {
		return nil
	}

	periodEnd, err := nextPeriodEnd(periodStart, subscription.BillingCycleType)
	if err != nil {
		return err
	}
	if !periodEnd.After(periodStart) {
		return billingcycledomain.ErrInvalidCyclePeriod
	}

	cycleID := s.genID.Generate()
	if err := s.insertCycle(ctx, tx, cycleID, subscription.OrgID, subscription.ID, periodStart, periodEnd, now); err != nil {
		return err
	}
	*events = append(*events, auditEvent{
		OrgID:          subscription.OrgID,
		Action:         "billing_cycle.opened",
		TargetType:     "billing_cycle",
		TargetID:       cycleID.String(),
		SubscriptionID: subscription.ID.String(),
		BillingCycleID: cycleID.String(),
		Metadata: map[string]any{
			"period_start": periodStart.Format(time.RFC3339),
			"period_end":   periodEnd.Format(time.RFC3339),
		},
	})
	return nil
}

func nextPeriodEnd(start time.Time, cycleType string) (time.Time, error) {
	switch strings.ToLower(strings.TrimSpace(cycleType)) {
	case "monthly":
		return start.AddDate(0, 1, 0), nil
	case "weekly":
		return start.AddDate(0, 0, 7), nil
	case "daily":
		return start.AddDate(0, 0, 1), nil
	default:
		return time.Time{}, subscriptiondomain.ErrInvalidBillingCycleType
	}
}

type invoiceRow struct {
	ID          snowflake.ID
	Status      invoicedomain.InvoiceStatus
	FinalizedAt *time.Time
}

func (s *Scheduler) loadInvoiceByCycle(ctx context.Context, cycleID snowflake.ID) (*invoiceRow, error) {
	var invoice invoiceRow
	err := s.db.WithContext(ctx).Raw(
		`SELECT id, status, finalized_at
		 FROM invoices
		 WHERE billing_cycle_id = ?
		 LIMIT 1`,
		cycleID,
	).Scan(&invoice).Error
	if err != nil {
		return nil, err
	}
	if invoice.ID == 0 {
		return nil, nil
	}
	return &invoice, nil
}

func (s *Scheduler) withAuditContext(ctx context.Context, subscriptionID, billingCycleID string) context.Context {
	ctx = auditcontext.WithActor(ctx, string(auditdomain.ActorTypeSystem), "scheduler")
	if subscriptionID != "" {
		ctx = auditcontext.WithSubscriptionID(ctx, subscriptionID)
	}
	if billingCycleID != "" {
		ctx = auditcontext.WithBillingCycleID(ctx, billingCycleID)
	}
	return ctx
}

func (s *Scheduler) emitAuditEvent(ctx context.Context, event auditEvent) {
	if s.auditSvc == nil {
		return
	}
	ctx = s.withAuditContext(ctx, event.SubscriptionID, event.BillingCycleID)
	orgID := event.OrgID
	targetID := event.TargetID
	_ = s.auditSvc.AuditLog(ctx, &orgID, "", nil, event.Action, event.TargetType, &targetID, event.Metadata)
}

func (s *Scheduler) authorizeSystem(ctx context.Context, orgID snowflake.ID, object string, action string) error {
	if s.authzSvc == nil {
		return authorization.ErrForbidden
	}
	return s.authzSvc.Authorize(ctx, "system", orgID.String(), object, action)
}

func (s *Scheduler) SLAEvaluationJob(ctx context.Context) error {
	ctx, run, owner := s.ensureJobRun(ctx, "sla_evaluation", s.cfg.BatchSize)
	if owner {
		s.logJobStart(ctx, run)
		defer s.logJobFinish(ctx, run)
	}

	if err := s.billingOperationsSvc.EvaluateSLAs(ctx); err != nil {
		s.logSchedulerError(ctx, run, "sla.evaluate.failed", "sla_evaluation", 0, err)
		return err
	}

	return nil
}

func (s *Scheduler) FinOpsScoringJob(ctx context.Context) error {
	ctx, run, owner := s.ensureJobRun(ctx, "finops_scoring", 1)
	if owner {
		s.logJobStart(ctx, run)
		defer s.logJobFinish(ctx, run)
	}

	if err := s.billingOperationsSvc.AggregateDailyPerformance(ctx); err != nil {
		s.logSchedulerError(ctx, run, "finops.scoring.failed", "finops_scoring", 0, err)
		return err
	}

	return nil
}
