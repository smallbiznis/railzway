package scheduler

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	auditcontext "github.com/smallbiznis/valora/internal/auditcontext"
	"github.com/smallbiznis/valora/internal/authorization"
	billingcycledomain "github.com/smallbiznis/valora/internal/billingcycle/domain"
	"github.com/smallbiznis/valora/internal/billingdashboard/rollup"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	ledgerdomain "github.com/smallbiznis/valora/internal/ledger/domain"
	"github.com/smallbiznis/valora/internal/orgcontext"
	ratingdomain "github.com/smallbiznis/valora/internal/rating/domain"
	"github.com/smallbiznis/valora/internal/scheduler/guard"
	subscriptiondomain "github.com/smallbiznis/valora/internal/subscription/domain"
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
	AuthzSvc        authorization.Service
	RollupSvc       *rollup.Service `optional:"true"`
	GenID           *snowflake.Node
	Config          Config `optional:"true"`
}

type Scheduler struct {
	db              *gorm.DB
	log             *zap.Logger
	cfg             Config
	genID           *snowflake.Node
	ratingSvc       ratingdomain.Service
	invoiceSvc      invoicedomain.Service
	ledgerSvc       ledgerdomain.Service
	subscriptionSvc subscriptiondomain.Service
	auditSvc        auditdomain.Service
	authzSvc        authorization.Service
	rollupSvc       *rollup.Service
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
	if p.DB == nil || p.Log == nil || p.RatingSvc == nil || p.InvoiceSvc == nil || p.LedgerSvc == nil || p.SubscriptionSvc == nil || p.GenID == nil || p.AuditSvc == nil || p.AuthzSvc == nil {
		return nil, ErrInvalidConfig
	}
	cfg := p.Config.withDefaults()
	return &Scheduler{
		db:              p.DB,
		log:             p.Log.Named("scheduler"),
		cfg:             cfg,
		genID:           p.GenID,
		ratingSvc:       p.RatingSvc,
		invoiceSvc:      p.InvoiceSvc,
		ledgerSvc:       p.LedgerSvc,
		subscriptionSvc: p.SubscriptionSvc,
		auditSvc:        p.AuditSvc,
		authzSvc:        p.AuthzSvc,
		rollupSvc:       p.RollupSvc,
	}, nil
}

func (s *Scheduler) RunOnce(ctx context.Context) error {
	ctx = auditcontext.WithActor(ctx, string(auditdomain.ActorTypeSystem), "scheduler")
	var err error
	if jobErr := s.EnsureBillingCyclesJob(ctx); jobErr != nil {
		err = errors.Join(err, jobErr)
	}
	if jobErr := s.CloseCyclesJob(ctx); jobErr != nil {
		err = errors.Join(err, jobErr)
	}
	if jobErr := s.RatingJob(ctx); jobErr != nil {
		err = errors.Join(err, jobErr)
	}
	if jobErr := s.CloseAfterRatingJob(ctx); jobErr != nil {
		err = errors.Join(err, jobErr)
	}
	if jobErr := s.InvoiceJob(ctx); jobErr != nil {
		err = errors.Join(err, jobErr)
	}
	if s.rollupSvc != nil {
		if jobErr := s.rollupSvc.ProcessRebuildRequests(ctx, s.cfg.BatchSize); jobErr != nil {
			err = errors.Join(err, jobErr)
		}
		if jobErr := s.rollupSvc.ProcessPending(ctx, s.cfg.BatchSize); jobErr != nil {
			err = errors.Join(err, jobErr)
		}
	}
	if jobErr := s.EndCanceledSubscriptionsJob(ctx); jobErr != nil {
		err = errors.Join(err, jobErr)
	}
	if jobErr := s.RecoverySweepJob(ctx); jobErr != nil {
		err = errors.Join(err, jobErr)
	}
	return err
}

func (s *Scheduler) RunForever(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.RunInterval)
	defer ticker.Stop()

	for {
		if err := s.RunOnce(ctx); err != nil {
			s.log.Warn("scheduler run failed", zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Scheduler) EnsureBillingCyclesJob(ctx context.Context) error {
	now := time.Now().UTC()
	var jobErr error

	for {
		processed, batchErr := s.ensureBillingCyclesBatch(ctx, now)
		if batchErr != nil {
			jobErr = errors.Join(jobErr, batchErr)
		}
		if processed == 0 {
			break
		}
	}

	return jobErr
}

func (s *Scheduler) CloseCyclesJob(ctx context.Context) error {
	now := time.Now().UTC()
	var jobErr error

	for {
		cycles, err := s.fetchBillingCyclesForWork(ctx, `status = ? AND period_end <= ?`, []any{billingcycledomain.BillingCycleStatusOpen, now}, s.cfg.MaxCloseBatchSize)
		if err != nil {
			return err
		}
		if len(cycles) == 0 {
			break
		}

		for _, cycle := range cycles {
			if err := s.authorizeSystem(ctx, cycle.OrgID, authorization.ObjectBillingCycle, authorization.ActionBillingCycleStartClosing); err != nil {
				jobErr = errors.Join(jobErr, err)
				continue
			}
			updated, err := s.markCycleClosing(ctx, cycle.ID, now)
			if err != nil {
				jobErr = errors.Join(jobErr, err)
				_ = s.recordCycleError(ctx, cycle.ID, err)
				continue
			}
			if updated {
				if err := s.upsertBillingCycleStats(ctx, s.db, cycle.ID, cycle.OrgID, cycle.PeriodStart, billingcycledomain.BillingCycleStatusClosing, now); err != nil {
					jobErr = errors.Join(jobErr, err)
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
	now := time.Now().UTC()
	var jobErr error

	for {
		cycles, err := s.fetchBillingCyclesForWork(ctx, `status = ? AND rating_completed_at IS NULL`, []any{billingcycledomain.BillingCycleStatusClosing}, s.cfg.MaxRatingBatchSize)
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
						"error": err.Error(),
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
						"error": err.Error(),
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
					"period_start": cycle.PeriodStart.Format(time.RFC3339),
					"period_end":   cycle.PeriodEnd.Format(time.RFC3339),
				},
			})
		}
	}

	return jobErr
}

func (s *Scheduler) CloseAfterRatingJob(ctx context.Context) error {
	now := time.Now().UTC()
	var jobErr error

	for {
		cycles, err := s.fetchBillingCyclesForWork(ctx, `status = ? AND rating_completed_at IS NOT NULL`, []any{billingcycledomain.BillingCycleStatusClosing}, s.cfg.MaxCloseBatchSize)
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
						"period_end": cycle.PeriodEnd.Format(time.RFC3339),
					},
				})
			}
		}
	}

	return jobErr
}

func (s *Scheduler) InvoiceJob(ctx context.Context) error {
	now := time.Now().UTC()
	var jobErr error

	for {
		cycles, err := s.fetchBillingCyclesForWork(ctx, `status = ? AND invoiced_at IS NULL`, []any{billingcycledomain.BillingCycleStatusClosed}, s.cfg.MaxInvoiceBatchSize)
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
			}

			if !s.cfg.FinalizeInvoices {
				continue
			}

			switch invoice.Status {
			case invoicedomain.InvoiceStatusDraft:
				if err := s.authorizeSystem(ctx, cycle.OrgID, authorization.ObjectInvoice, authorization.ActionInvoiceFinalize); err != nil {
					jobErr = errors.Join(jobErr, err)
					continue
				}
				if err := s.invoiceSvc.FinalizeInvoice(cycleCtx, invoice.ID.String()); err != nil {
					jobErr = errors.Join(jobErr, err)
					_ = s.recordCycleError(ctx, cycle.ID, err)
					continue
				}
				if err := s.markCycleInvoiceFinalized(ctx, cycle.ID, now); err != nil {
					jobErr = errors.Join(jobErr, err)
					_ = s.recordCycleError(ctx, cycle.ID, err)
				}
			case invoicedomain.InvoiceStatusFinalized:
				if err := s.markCycleInvoiceFinalized(ctx, cycle.ID, now); err != nil {
					jobErr = errors.Join(jobErr, err)
					_ = s.recordCycleError(ctx, cycle.ID, err)
				}
			}
		}
	}

	return jobErr
}

func (s *Scheduler) EndCanceledSubscriptionsJob(ctx context.Context) error {
	var jobErr error

	for {
		subscriptions, err := s.FetchSubscriptionsForWork(ctx, subscriptiondomain.SubscriptionStatusCanceled, s.cfg.BatchSize)
		if err != nil {
			return err
		}
		if len(subscriptions) == 0 {
			break
		}

		for _, subscription := range subscriptions {
			canEnd, err := s.canEndSubscription(ctx, subscription.OrgID, subscription.ID)
			if err != nil {
				jobErr = errors.Join(jobErr, err)
				continue
			}
			if !canEnd {
				continue
			}

			if err := s.authorizeSystem(ctx, subscription.OrgID, authorization.ObjectSubscription, authorization.ActionSubscriptionEnd); err != nil {
				jobErr = errors.Join(jobErr, err)
				continue
			}

			ctxWithOrg := orgcontext.WithOrgID(ctx, int64(subscription.OrgID))
			ctxWithAudit := s.withAuditContext(ctxWithOrg, subscription.ID.String(), "")
			if err := s.subscriptionSvc.TransitionSubscription(ctxWithAudit, subscription.ID.String(), subscriptiondomain.SubscriptionStatusEnded, subscriptiondomain.TransitionReason("scheduler")); err != nil {
				jobErr = errors.Join(jobErr, err)
				s.log.Warn("failed to end subscription", zap.String("subscription_id", subscription.ID.String()), zap.Error(err))
				continue
			}

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

func (s *Scheduler) ensureBillingCyclesBatch(ctx context.Context, now time.Time) (int, error) {
	var batchErr error
	processed := 0
	events := make([]auditEvent, 0)

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		subscriptions, err := s.fetchSubscriptionsForWork(ctx, tx, subscriptiondomain.SubscriptionStatusActive, s.cfg.BatchSize)
		if err != nil {
			return err
		}
		if len(subscriptions) == 0 {
			return nil
		}
		processed = len(subscriptions)

		for _, subscription := range subscriptions {
			if err := s.ensureSubscriptionCycle(ctx, tx, subscription, now, &events); err != nil {
				batchErr = errors.Join(batchErr, err)
				s.log.Warn("failed to ensure billing cycle", zap.String("subscription_id", subscription.ID.String()), zap.Error(err))
				continue
			}
		}
		return nil
	})

	if err == nil {
		for _, event := range events {
			s.emitAuditEvent(ctx, event)
		}
	}

	return processed, errors.Join(batchErr, err)
}

func (s *Scheduler) ensureSubscriptionCycle(ctx context.Context, tx *gorm.DB, subscription WorkSubscription, now time.Time, events *[]auditEvent) error {
	if err := s.authorizeSystem(ctx, subscription.OrgID, authorization.ObjectBillingCycle, authorization.ActionBillingCycleOpen); err != nil {
		return err
	}

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
