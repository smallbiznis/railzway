package service

import (
	"context"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	"github.com/smallbiznis/valora/internal/events"
	ledgerdomain "github.com/smallbiznis/valora/internal/ledger/domain"
	obsmetrics "github.com/smallbiznis/valora/internal/observability/metrics"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB         *gorm.DB
	Log        *zap.Logger
	GenID      *snowflake.Node
	AuditSvc   auditdomain.Service
	Outbox     *events.Outbox      `optional:"true"`
	ObsMetrics *obsmetrics.Metrics `optional:"true"`
}

type Service struct {
	db         *gorm.DB
	log        *zap.Logger
	genID      *snowflake.Node
	auditSvc   auditdomain.Service
	outbox     *events.Outbox
	obsMetrics *obsmetrics.Metrics
}

func NewService(p Params) ledgerdomain.Service {
	return &Service{
		db:         p.DB,
		log:        p.Log.Named("ledger.service"),
		genID:      p.GenID,
		auditSvc:   p.AuditSvc,
		outbox:     p.Outbox,
		obsMetrics: p.ObsMetrics,
	}
}

func (s *Service) CreateEntry(
	ctx context.Context,
	orgID snowflake.ID,
	sourceType string,
	sourceID snowflake.ID,
	currency string,
	occurredAt time.Time,
	lines []ledgerdomain.LedgerEntryLine,
) error {
	if orgID == 0 {
		return ledgerdomain.ErrInvalidOrganization
	}

	sourceType = strings.TrimSpace(sourceType)
	if sourceType == "" {
		return ledgerdomain.ErrInvalidSourceType
	}
	if sourceID == 0 {
		return ledgerdomain.ErrInvalidSourceID
	}

	currency = strings.TrimSpace(currency)
	if currency == "" {
		return ledgerdomain.ErrInvalidCurrency
	}
	if occurredAt.IsZero() {
		return ledgerdomain.ErrInvalidOccurredAt
	}

	if len(lines) < 2 {
		return ledgerdomain.ErrInvalidEntryLines
	}

	normalized := make([]ledgerdomain.LedgerEntryLine, 0, len(lines))
	for _, line := range lines {
		if line.AccountID == 0 {
			return ledgerdomain.ErrInvalidAccount
		}
		direction, err := normalizeDirection(line.Direction)
		if err != nil {
			return err
		}
		if line.Amount < 0 {
			return ledgerdomain.ErrInvalidLineAmount
		}
		normalized = append(normalized, ledgerdomain.LedgerEntryLine{
			AccountID: line.AccountID,
			Direction: direction,
			Currency:  line.Currency,
			Amount:    line.Amount,
		})
	}

	if err := ledgerdomain.ValidateBalanced(normalized); err != nil {
		return err
	}

	auditSvc := s.auditSvc
	if auditSvc == nil {
		s.log.Warn("audit service unavailable for ledger entry", zap.String("source_type", string(sourceType)), zap.String("source_id", sourceID.String()))
	}

	inserted := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entryID := s.genID.Generate()
		now := time.Now().UTC()
		result := tx.WithContext(ctx).Exec(
			`INSERT INTO ledger_entries (
				id, org_id, source_type, source_id, currency, occurred_at, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (org_id, source_type, source_id) DO NOTHING`,
			entryID,
			orgID,
			sourceType,
			sourceID,
			currency,
			occurredAt.UTC(),
			now,
		)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		inserted = true

		for _, line := range normalized {
			if err := tx.WithContext(ctx).Exec(
				`INSERT INTO ledger_entry_lines (
					id, ledger_entry_id, account_id, direction, currency, amount, created_at
				) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				s.genID.Generate(),
				entryID,
				line.AccountID,
				string(line.Direction),
				line.Currency,
				line.Amount,
				now,
			).Error; err != nil {
				return err
			}
		}

		if s.outbox != nil {
			payload := map[string]any{
				"ledger_entry_id": entryID.String(),
				"source_type":     sourceType,
				"source_id":       sourceID.String(),
			}
			if err := s.outbox.PublishTx(ctx, tx, events.Event{
				OrgID:     orgID,
				Type:      events.EventLedgerEntryCreated,
				Payload:   payload,
				DedupeKey: "ledger_entry:" + entryID.String(),
			}); err != nil {
				return err
			}
		}

		entryIDStr := entryID.String()
		metadata := map[string]any{
			"source_type":     sourceType,
			"source_id":       sourceID.String(),
			"ledger_entry_id": entryIDStr,
		}
		if auditSvc != nil {
			if err := auditSvc.AuditLog(ctx, &orgID, "", nil, "ledger.entry_created", "ledger_entry", &entryIDStr, metadata); err != nil {
				s.log.Warn("failed to write ledger audit log", zap.Error(err))
			}
		}

		return nil
	})
	if err != nil {
		return err
	}
	if inserted && s.obsMetrics != nil {
		s.obsMetrics.RecordLedgerEntry(ctx, sourceType)
	}
	return nil
}

func normalizeDirection(direction ledgerdomain.LedgerEntryDirection) (ledgerdomain.LedgerEntryDirection, error) {
	normalized := strings.ToLower(strings.TrimSpace(string(direction)))
	switch normalized {
	case string(ledgerdomain.LedgerEntryDirectionDebit):
		return ledgerdomain.LedgerEntryDirectionDebit, nil
	case string(ledgerdomain.LedgerEntryDirectionCredit):
		return ledgerdomain.LedgerEntryDirectionCredit, nil
	default:
		return "", ledgerdomain.ErrInvalidLineDirection
	}
}
