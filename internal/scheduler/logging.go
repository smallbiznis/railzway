package scheduler

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	obscontext "github.com/smallbiznis/valora/internal/observability/context"
	obslogger "github.com/smallbiznis/valora/internal/observability/logger"
	obsmetrics "github.com/smallbiznis/valora/internal/observability/metrics"
	"go.uber.org/zap"
)

type jobRun struct {
	job            string
	runID          string
	batchSize      int
	startedAt      time.Time
	processedCount int
	errorCount     int
}

type jobRunKey struct{}

func (r *jobRun) AddProcessed(count int) {
	if r == nil || count <= 0 {
		return
	}
	r.processedCount += count
}

func (r *jobRun) IncError() {
	if r == nil {
		return
	}
	r.errorCount++
}

func (s *Scheduler) ensureJobRun(ctx context.Context, job string, batchSize int) (context.Context, *jobRun, bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	if existing := jobRunFromContext(ctx); existing != nil {
		return ctx, existing, false
	}
	run := &jobRun{
		job:       job,
		runID:     s.genID.Generate().String(),
		batchSize: batchSize,
		startedAt: time.Now(),
	}
	ctx = context.WithValue(ctx, jobRunKey{}, run)
	ctx = s.withLogContext(ctx, 0)
	return ctx, run, true
}

func jobRunFromContext(ctx context.Context) *jobRun {
	if ctx == nil {
		return nil
	}
	if run, ok := ctx.Value(jobRunKey{}).(*jobRun); ok {
		return run
	}
	return nil
}

func (s *Scheduler) withLogContext(ctx context.Context, orgID snowflake.ID) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = obscontext.WithActor(ctx, "system", "scheduler")
	if orgID != 0 {
		ctx = obscontext.WithOrgID(ctx, orgID.String())
	}
	return ctx
}

func (s *Scheduler) logger(ctx context.Context) *zap.Logger {
	return obslogger.WithContext(ctx, s.log)
}

func (s *Scheduler) logJobStart(ctx context.Context, run *jobRun) {
	if run == nil {
		return
	}
	s.logger(ctx).Info("scheduler.job.start",
		zap.String("job", run.job),
		zap.String("run_id", run.runID),
		zap.Int("batch_size", run.batchSize),
	)
}

func (s *Scheduler) logJobFinish(ctx context.Context, run *jobRun) {
	if run == nil {
		return
	}
	fields := []zap.Field{
		zap.String("job", run.job),
		zap.String("run_id", run.runID),
		zap.Int64("duration_ms", time.Since(run.startedAt).Milliseconds()),
		zap.Int("processed_count", run.processedCount),
		zap.Int("error_count", run.errorCount),
	}
	log := s.logger(ctx)
	if run.errorCount > 0 {
		log.Warn("scheduler.job.finish", fields...)
		return
	}
	log.Info("scheduler.job.finish", fields...)
}

func (s *Scheduler) logSchedulerError(ctx context.Context, run *jobRun, msg string, job string, orgID snowflake.ID, err error, fields ...zap.Field) {
	if err == nil {
		return
	}
	if run != nil {
		run.IncError()
	}
	ctx = s.withLogContext(ctx, orgID)
	errorType := obsmetrics.ClassifySchedulerErrorType(err)
	retryable := obsmetrics.IsSchedulerErrorRetryable(err)
	baseFields := []zap.Field{
		zap.String("job", job),
		zap.String("org_id", idString(orgID)),
		zap.String("error_type", errorType),
		zap.String("error", err.Error()),
		zap.Bool("retryable", retryable),
	}
	s.logger(ctx).Error(msg, append(baseFields, fields...)...)
}

func (s *Scheduler) logCycleClaimed(ctx context.Context, job string, cycle WorkBillingCycle) {
	ctx = s.withLogContext(ctx, cycle.OrgID)
	s.logger(ctx).Debug("scheduler.cycle.claimed",
		zap.String("job", job),
		zap.String("cycle_id", idString(cycle.ID)),
		zap.String("org_id", idString(cycle.OrgID)),
		zap.String("subscription_id", idString(cycle.SubscriptionID)),
		zap.String("status", string(cycle.Status)),
	)
}

func (s *Scheduler) logInvoiceGenerated(ctx context.Context, cycle WorkBillingCycle, invoiceID snowflake.ID) {
	ctx = s.withLogContext(ctx, cycle.OrgID)
	s.logger(ctx).Info("invoice.generated",
		zap.String("cycle_id", idString(cycle.ID)),
		zap.String("invoice_id", idString(invoiceID)),
		zap.String("org_id", idString(cycle.OrgID)),
		zap.String("status", "draft"),
	)
}

func (s *Scheduler) logInvoiceFinalized(ctx context.Context, cycle WorkBillingCycle, invoiceID snowflake.ID) {
	ctx = s.withLogContext(ctx, cycle.OrgID)
	s.logger(ctx).Info("invoice.finalized",
		zap.String("cycle_id", idString(cycle.ID)),
		zap.String("invoice_id", idString(invoiceID)),
		zap.String("org_id", idString(cycle.OrgID)),
	)
}

func idString(id snowflake.ID) string {
	if id == 0 {
		return ""
	}
	return id.String()
}
