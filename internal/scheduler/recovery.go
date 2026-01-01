package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/smallbiznis/valora/internal/authorization"
	billingcycledomain "github.com/smallbiznis/valora/internal/billingcycle/domain"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
)

func (s *Scheduler) RecoverySweepJob(ctx context.Context) error {
	now := time.Now().UTC()
	cutoff := now.Add(-s.cfg.RecoveryThreshold)
	var jobErr error

	// Retry rating for stuck closing cycles.
	for {
		cycles, err := s.fetchBillingCyclesForWork(
			ctx,
			`status = ? AND rating_completed_at IS NULL AND closing_started_at IS NOT NULL AND closing_started_at <= ?`,
			[]any{billingcycledomain.BillingCycleStatusClosing, cutoff},
			s.cfg.MaxRatingBatchSize,
		)
		if err != nil {
			return err
		}
		if len(cycles) == 0 {
			break
		}

		for _, cycle := range cycles {
			if err := s.authorizeSystem(ctx, cycle.OrgID, authorization.ObjectBillingCycle, authorization.ActionBillingCycleRate); err != nil {
				jobErr = errors.Join(jobErr, err)
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
				_ = s.recordCycleError(ctx, cycle.ID, err)
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
				_ = s.recordCycleError(ctx, cycle.ID, err)
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
			return err
		}
		if len(cycles) == 0 {
			break
		}

		for _, cycle := range cycles {
			if err := s.authorizeSystem(ctx, cycle.OrgID, authorization.ObjectBillingCycle, authorization.ActionBillingCycleClose); err != nil {
				jobErr = errors.Join(jobErr, err)
				continue
			}
			hasResults, err := s.hasRatingResults(ctx, cycle.ID)
			if err != nil {
				jobErr = errors.Join(jobErr, err)
				_ = s.recordCycleError(ctx, cycle.ID, err)
				continue
			}
			if !hasResults {
				jobErr = errors.Join(jobErr, invoicedomain.ErrMissingRatingResults)
				_ = s.recordCycleError(ctx, cycle.ID, invoicedomain.ErrMissingRatingResults)
				continue
			}
			cycleCtx := s.withAuditContext(ctx, cycle.SubscriptionID.String(), cycle.ID.String())
			if err := s.ensureLedgerEntryForCycle(cycleCtx, cycle); err != nil {
				jobErr = errors.Join(jobErr, err)
				_ = s.recordCycleError(ctx, cycle.ID, err)
				continue
			}

			updated, err := s.markCycleClosed(ctx, cycle.ID, now)
			if err != nil {
				jobErr = errors.Join(jobErr, err)
				_ = s.recordCycleError(ctx, cycle.ID, err)
				continue
			}
			if updated {
				if err := s.upsertBillingCycleStats(ctx, s.db, cycle.ID, cycle.OrgID, cycle.PeriodStart, billingcycledomain.BillingCycleStatusClosed, now); err != nil {
					jobErr = errors.Join(jobErr, err)
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
			return err
		}
		if len(cycles) == 0 {
			break
		}

		for _, cycle := range cycles {
			if err := s.authorizeSystem(ctx, cycle.OrgID, authorization.ObjectInvoice, authorization.ActionInvoiceGenerate); err != nil {
				jobErr = errors.Join(jobErr, err)
				continue
			}
			cycleCtx := s.withAuditContext(ctx, cycle.SubscriptionID.String(), cycle.ID.String())
			if err := s.invoiceSvc.GenerateInvoice(cycleCtx, cycle.ID.String()); err != nil {
				jobErr = errors.Join(jobErr, err)
				_ = s.recordCycleError(ctx, cycle.ID, err)
				continue
			}

			invoice, err := s.loadInvoiceByCycle(ctx, cycle.ID)
			if err != nil {
				jobErr = errors.Join(jobErr, err)
				_ = s.recordCycleError(ctx, cycle.ID, err)
				continue
			}
			if invoice == nil {
				continue
			}

			if err := s.markCycleInvoiced(ctx, cycle.ID, now); err != nil {
				jobErr = errors.Join(jobErr, err)
				_ = s.recordCycleError(ctx, cycle.ID, err)
				continue
			}
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
