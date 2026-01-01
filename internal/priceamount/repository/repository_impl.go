package repository

import (
	"context"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	priceamountdomain "github.com/smallbiznis/valora/internal/priceamount/domain"
	"github.com/smallbiznis/valora/pkg/db/option"
	"gorm.io/gorm"
)

type repo struct{}

func Provide() priceamountdomain.Repository {
	return &repo{}
}

func (r *repo) Insert(ctx context.Context, db *gorm.DB, amount *priceamountdomain.PriceAmount) error {
	return db.WithContext(ctx).Exec(
		`INSERT INTO price_amounts (
			id, org_id, price_id, meter_id, currency, unit_amount_cents, minimum_amount_cents, maximum_amount_cents, effective_from, effective_to,
			metadata, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		amount.ID,
		amount.OrgID,
		amount.PriceID,
		amount.MeterID,
		amount.Currency,
		amount.UnitAmountCents,
		amount.MinimumAmountCents,
		amount.MaximumAmountCents,
		amount.EffectiveFrom,
		amount.EffectiveTo,
		amount.Metadata,
		amount.CreatedAt,
		amount.UpdatedAt,
	).Error
}

func (r *repo) FindByID(ctx context.Context, db *gorm.DB, orgID, id snowflake.ID) (*priceamountdomain.PriceAmount, error) {
	var amount priceamountdomain.PriceAmount
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, price_id, meter_id, currency, unit_amount_cents, minimum_amount_cents, maximum_amount_cents, effective_from, effective_to,
		 metadata, created_at, updated_at
		 FROM price_amounts WHERE org_id = ? AND id = ?`,
		orgID,
		id,
	).Scan(&amount).Error
	if err != nil {
		return nil, err
	}
	if amount.ID == 0 {
		return nil, nil
	}
	return &amount, nil
}

func (r *repo) List(ctx context.Context, db *gorm.DB, f priceamountdomain.PriceAmount, opts ...option.QueryOption) ([]priceamountdomain.PriceAmount, error) {
	var items []priceamountdomain.PriceAmount
	stmt := db.WithContext(ctx).Model(&priceamountdomain.PriceAmount{})

	for _, opt := range opts {
		stmt = opt.Apply(stmt)
	}

	if err := stmt.Where(f).Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (r *repo) FindOne(ctx context.Context, db *gorm.DB, amount *priceamountdomain.PriceAmount) (*priceamountdomain.PriceAmount, error) {
	var result priceamountdomain.PriceAmount
	err := db.WithContext(ctx).Where(amount).First(&result).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

func (r *repo) Update(ctx context.Context, db *gorm.DB, amount *priceamountdomain.PriceAmount) (*priceamountdomain.PriceAmount, error) {
	err := db.WithContext(ctx).Model(&priceamountdomain.PriceAmount{}).
		Where("org_id = ? AND id = ?", amount.OrgID, amount.ID).
		Updates(map[string]interface{}{
			// Only allow closing windows; pricing is append-only.
			"effective_to": amount.EffectiveTo,
			"updated_at":   amount.UpdatedAt,
		}).Error
	if err != nil {
		return nil, err
	}
	return amount, nil
}

func (r *repo) FindEffectiveAt(
	ctx context.Context,
	db *gorm.DB,
	orgID, priceID snowflake.ID,
	meterID *snowflake.ID,
	currency string,
	at time.Time,
) (*priceamountdomain.PriceAmount, error) {
	var amount priceamountdomain.PriceAmount
	query := `
		SELECT id, org_id, price_id, meter_id, currency, unit_amount_cents, minimum_amount_cents, maximum_amount_cents,
		       effective_from, effective_to, metadata, created_at, updated_at
		FROM price_amounts
		WHERE org_id = ? AND price_id = ?
		  AND effective_from <= ?
		  AND (effective_to IS NULL OR effective_to > ?)`
	args := []any{orgID, priceID, at, at}
	query, args = applyMeterCondition(query, args, meterID)
	query, args = applyCurrencyCondition(query, args, currency)
	query += `
		ORDER BY effective_from DESC
		LIMIT 1`
	if err := db.WithContext(ctx).Raw(query, args...).Scan(&amount).Error; err != nil {
		return nil, err
	}
	if amount.ID == 0 {
		return nil, nil
	}
	return &amount, nil
}

func (r *repo) FindPrevious(
	ctx context.Context,
	db *gorm.DB,
	orgID, priceID snowflake.ID,
	meterID *snowflake.ID,
	currency string,
	before time.Time,
) (*priceamountdomain.PriceAmount, error) {
	var amount priceamountdomain.PriceAmount
	query := `
		SELECT id, org_id, price_id, meter_id, currency, unit_amount_cents, minimum_amount_cents, maximum_amount_cents,
		       effective_from, effective_to, metadata, created_at, updated_at
		FROM price_amounts
		WHERE org_id = ? AND price_id = ?
		  AND effective_from < ?`
	args := []any{orgID, priceID, before}
	query, args = applyMeterCondition(query, args, meterID)
	query, args = applyCurrencyCondition(query, args, currency)
	query += `
		ORDER BY effective_from DESC
		LIMIT 1`
	if err := db.WithContext(ctx).Raw(query, args...).Scan(&amount).Error; err != nil {
		return nil, err
	}
	if amount.ID == 0 {
		return nil, nil
	}
	return &amount, nil
}

func (r *repo) FindNext(
	ctx context.Context,
	db *gorm.DB,
	orgID, priceID snowflake.ID,
	meterID *snowflake.ID,
	currency string,
	after time.Time,
) (*priceamountdomain.PriceAmount, error) {
	var amount priceamountdomain.PriceAmount
	query := `
		SELECT id, org_id, price_id, meter_id, currency, unit_amount_cents, minimum_amount_cents, maximum_amount_cents,
		       effective_from, effective_to, metadata, created_at, updated_at
		FROM price_amounts
		WHERE org_id = ? AND price_id = ?
		  AND effective_from > ?`
	args := []any{orgID, priceID, after}
	query, args = applyMeterCondition(query, args, meterID)
	query, args = applyCurrencyCondition(query, args, currency)
	query += `
		ORDER BY effective_from ASC
		LIMIT 1`
	if err := db.WithContext(ctx).Raw(query, args...).Scan(&amount).Error; err != nil {
		return nil, err
	}
	if amount.ID == 0 {
		return nil, nil
	}
	return &amount, nil
}

func (r *repo) ListOverlapping(
	ctx context.Context,
	db *gorm.DB,
	orgID, priceID snowflake.ID,
	meterID *snowflake.ID,
	currency string,
	start, end time.Time,
) ([]priceamountdomain.PriceAmount, error) {
	var items []priceamountdomain.PriceAmount
	query := `
		SELECT id, org_id, price_id, meter_id, currency, unit_amount_cents, minimum_amount_cents, maximum_amount_cents,
		       effective_from, effective_to, metadata, created_at, updated_at
		FROM price_amounts
		WHERE org_id = ? AND price_id = ?
		  AND effective_from < ?
		  AND (effective_to IS NULL OR effective_to > ?)`
	args := []any{orgID, priceID, end, start}
	query, args = applyMeterCondition(query, args, meterID)
	query, args = applyCurrencyCondition(query, args, currency)
	query += `
		ORDER BY effective_from ASC`
	if err := db.WithContext(ctx).Raw(query, args...).Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func applyMeterCondition(query string, args []any, meterID *snowflake.ID) (string, []any) {
	if meterID != nil {
		query += " AND meter_id = ?"
		args = append(args, *meterID)
		return query, args
	}
	query += " AND meter_id IS NULL"
	return query, args
}

func applyCurrencyCondition(query string, args []any, currency string) (string, []any) {
	trimmed := strings.TrimSpace(currency)
	if trimmed == "" {
		return query, args
	}
	query += " AND currency = ?"
	args = append(args, trimmed)
	return query, args
}
