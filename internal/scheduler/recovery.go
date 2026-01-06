package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/smallbiznis/valora/internal/authorization"
	billingcycledomain "github.com/smallbiznis/valora/internal/billingcycle/domain"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	obsmetrics "github.com/smallbiznis/valora/internal/observability/metrics"
	"go.uber.org/zap"
)

func (s *Scheduler) RecoverySweepJob(ctx context.Context) error {
	ctx, run, owner := s.ensureJobRun(ctx, "recovery_sweep", maxInt(s.cfg.MaxRatingBatchSize, s.cfg.MaxCloseBatchSize, s.cfg.MaxInvoiceBatchSize))
	if owner {
		s.logJobStart(ctx, run)
		defer s.logJobFinish(ctx, run)
	}
	now := time.Now().UTC()
	cutoff := now.Add(-s.cfg.RecoveryThreshold)
	var jobErr error
	schedMetrics := obsmetrics.Scheduler()

	// Retry rating for stuck closing cycles.
	for {
		cycles, err := s.fetchBillingCyclesForWork(
			ctx,
			`status = ? AND rating_completed_at IS NULL AND closing_started_at IS NOT NULL AND closing_started_at <= ?`,
			[]any{billingcycledomain.BillingCycleStatusClosing, cutoff},
			s.cfg.MaxRatingBatchSize,
		)
		if err != nil {
			s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "recovery_sweep", 0, err)
			return err
		}
		if len(cycles) == 0 {
			break
		}

		for _, cycle := range cycles {
			s.logCycleClaimed(ctx, "recovery_sweep", cycle)
			if err := s.authorizeSystem(ctx, cycle.OrgID, authorization.ObjectBillingCycle, authorization.ActionBillingCycleRate); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.authorize.failed", "recovery_sweep", cycle.OrgID, err,
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
					"recovery":     true,
					"period_start": cycle.PeriodStart.Format(time.RFC3339),
					"period_end":   cycle.PeriodEnd.Format(time.RFC3339),
				},
			})

			if err := s.ratingSvc.RunRating(cycleCtx, cycle.ID.String()); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "recovery_sweep", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageRecoveryRating, err)
				s.emitAuditEvent(cycleCtx, auditEvent{
					OrgID:          cycle.OrgID,
					Action:         "rating.failed",
					TargetType:     "billing_cycle",
					TargetID:       cycle.ID.String(),
					SubscriptionID: cycle.SubscriptionID.String(),
					BillingCycleID: cycle.ID.String(),
					Metadata: map[string]any{
						"recovery": true,
						"error":    err.Error(),
					},
				})
				continue
			}
			if err := s.markRatingCompleted(ctx, cycle.ID, now); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "recovery_sweep", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageRecoveryRating, err)
				s.emitAuditEvent(cycleCtx, auditEvent{
					OrgID:          cycle.OrgID,
					Action:         "rating.failed",
					TargetType:     "billing_cycle",
					TargetID:       cycle.ID.String(),
					SubscriptionID: cycle.SubscriptionID.String(),
					BillingCycleID: cycle.ID.String(),
					Metadata: map[string]any{
						"recovery": true,
						"error":    err.Error(),
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
					"recovery":     true,
					"period_start": cycle.PeriodStart.Format(time.RFC3339),
					"period_end":   cycle.PeriodEnd.Format(time.RFC3339),
				},
			})
			s.emitAuditEvent(cycleCtx, auditEvent{
				OrgID:          cycle.OrgID,
				Action:         "billing_cycle.recovered",
				TargetType:     "billing_cycle",
				TargetID:       cycle.ID.String(),
				SubscriptionID: cycle.SubscriptionID.String(),
				BillingCycleID: cycle.ID.String(),
				Metadata: map[string]any{
					"step": "rating",
				},
			})
		}
	}

	// Close cycles that finished rating but are still closing.
	for {
		cycles, err := s.fetchBillingCyclesForWork(
			ctx,
			`status = ? AND rating_completed_at IS NOT NULL`,
			[]any{billingcycledomain.BillingCycleStatusClosing},
			s.cfg.MaxCloseBatchSize,
		)
		if err != nil {
			s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "recovery_sweep", 0, err)
			return err
		}
		if len(cycles) == 0 {
			break
		}

		for _, cycle := range cycles {
			s.logCycleClaimed(ctx, "recovery_sweep", cycle)
			if err := s.authorizeSystem(ctx, cycle.OrgID, authorization.ObjectBillingCycle, authorization.ActionBillingCycleClose); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.authorize.failed", "recovery_sweep", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				continue
			}
			hasResults, err := s.hasRatingResults(ctx, cycle.ID)
			if err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "recovery_sweep", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageRecoveryClose, err)
				continue
			}
			if !hasResults {
				jobErr = errors.Join(jobErr, invoicedomain.ErrMissingRatingResults)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "recovery_sweep", cycle.OrgID, invoicedomain.ErrMissingRatingResults,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageRecoveryClose, invoicedomain.ErrMissingRatingResults)
				continue
			}
			cycleCtx := s.withAuditContext(ctx, cycle.SubscriptionID.String(), cycle.ID.String())
			if err := s.ensureLedgerEntryForCycle(cycleCtx, cycle); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "recovery_sweep", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageRecoveryClose, err)
				continue
			}

			updated, err := s.markCycleClosed(ctx, cycle.ID, now)
			if err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "recovery_sweep", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageRecoveryClose, err)
				continue
			}
			if updated {
				run.AddProcessed(1)
				if err := s.upsertBillingCycleStats(ctx, s.db, cycle.ID, cycle.OrgID, cycle.PeriodStart, billingcycledomain.BillingCycleStatusClosed, now); err != nil {
					jobErr = errors.Join(jobErr, err)
					s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "recovery_sweep", cycle.OrgID, err,
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
						"recovery":   true,
						"period_end": cycle.PeriodEnd.Format(time.RFC3339),
					},
				})
				s.emitAuditEvent(cycleCtx, auditEvent{
					OrgID:          cycle.OrgID,
					Action:         "billing_cycle.recovered",
					TargetType:     "billing_cycle",
					TargetID:       cycle.ID.String(),
					SubscriptionID: cycle.SubscriptionID.String(),
					BillingCycleID: cycle.ID.String(),
					Metadata: map[string]any{
						"step": "close",
					},
				})
			}
		}
	}

	// Retry invoicing for closed cycles stuck without invoices.
	for {
		cycles, err := s.fetchBillingCyclesForWork(
			ctx,
			`status = ? AND invoiced_at IS NULL AND closed_at IS NOT NULL AND closed_at <= ?`,
			[]any{billingcycledomain.BillingCycleStatusClosed, cutoff},
			s.cfg.MaxInvoiceBatchSize,
		)
		if err != nil {
			s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "recovery_sweep", 0, err)
			return err
		}
		if len(cycles) == 0 {
			break
		}

		for _, cycle := range cycles {
			s.logCycleClaimed(ctx, "recovery_sweep", cycle)
			if err := s.authorizeSystem(ctx, cycle.OrgID, authorization.ObjectInvoice, authorization.ActionInvoiceGenerate); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.authorize.failed", "recovery_sweep", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				continue
			}
			cycleCtx := s.withAuditContext(ctx, cycle.SubscriptionID.String(), cycle.ID.String())
			invoice, err := s.invoiceSvc.GenerateInvoice(cycleCtx, cycle.ID.String())
			if err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "invoice.generate.failed", "recovery_sweep", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageRecoveryInvoice, err)
				continue
			}

			// invoice, err := s.loadInvoiceByCycle(ctx, cycle.ID)
			// if err != nil {
			// 	jobErr = errors.Join(jobErr, err)
			// 	_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageRecoveryInvoice, err)
			// 	continue
			// }
			if invoice == nil {
				continue
			}
			s.logInvoiceGenerated(ctx, cycle, invoice.ID)

			if err := s.markCycleInvoiced(ctx, cycle.ID, now); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.logSchedulerError(ctx, run, "scheduler.cycle.process.failed", "recovery_sweep", cycle.OrgID, err,
					zap.String("cycle_id", idString(cycle.ID)),
					zap.String("invoice_id", idString(invoice.ID)),
					zap.String("subscription_id", idString(cycle.SubscriptionID)),
				)
				_ = s.recordCycleErrorWithMetrics(ctx, cycle.ID, obsmetrics.CycleStageRecoveryInvoice, err)
				continue
			}
			run.AddProcessed(1)
			schedMetrics.IncBillingCycleTransition(
				string(billingcycledomain.BillingCycleStatusClosed),
				obsmetrics.BillingCycleTransitionInvoiced,
			)
			s.emitAuditEvent(cycleCtx, auditEvent{
				OrgID:          cycle.OrgID,
				Action:         "billing_cycle.recovered",
				TargetType:     "billing_cycle",
				TargetID:       cycle.ID.String(),
				SubscriptionID: cycle.SubscriptionID.String(),
				BillingCycleID: cycle.ID.String(),
				Metadata: map[string]any{
					"step":       "invoice",
					"invoice_id": invoice.ID.String(),
				},
			})
		}
	}

	return jobErr
}
