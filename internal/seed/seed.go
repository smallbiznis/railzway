package seed

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	authdomain "github.com/smallbiznis/valora/internal/auth/domain"
	"github.com/smallbiznis/valora/internal/auth/password"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	invoicetemplatedomain "github.com/smallbiznis/valora/internal/invoicetemplate/domain"
	organizationdomain "github.com/smallbiznis/valora/internal/organization/domain"
	"gorm.io/gorm"
)

const (
	defaultOrgName       = "Main"
	defaultOrgSlug       = "main"
	defaultAdminEmail    = "admin@valora.cloud"
	defaultAdminPassword = "admin"
	defaultAdminDisplay  = "Valora Admin"
)

// EnsureMainOrg seeds the default organization for startup bootstrap.
func EnsureMainOrg(db *gorm.DB) error {
	if db == nil {
		return errors.New("seed database handle is required")
	}

	node, err := snowflake.NewNode(1)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		org, err := ensureMainOrgTx(ctx, tx, node)
		_, err = ensureInvoiceSequenceTx(ctx, tx, node, org.ID)
		_, err = ensureInvoiceTemplateTx(ctx, tx, node, org.ID)
		err = ensureLedgerAccounts(ctx, tx, node, org.ID)
		return err
	})
}

// EnsureMainOrgAndAdmin seeds the default organization and admin user for OSS mode.
func EnsureMainOrgAndAdmin(db *gorm.DB) error {
	if db == nil {
		return errors.New("seed database handle is required")
	}

	node, err := snowflake.NewNode(1)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		org, err := ensureMainOrgTx(ctx, tx, node)
		if err != nil {
			return err
		}

		var user authdomain.User
		err = tx.WithContext(ctx).
			Where("provider = ? AND external_id = ?", "local", defaultAdminEmail).
			First(&user).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			hashed, err := password.Hash(defaultAdminPassword)
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			user = authdomain.User{
				ID:                  node.Generate(),
				ExternalID:          defaultAdminEmail,
				Provider:            "local",
				DisplayName:         defaultAdminDisplay,
				Email:               strings.ToLower(defaultAdminEmail),
				PasswordHash:        &hashed,
				LastPasswordChanged: nil,
				IsDefault:           true,
				CreatedAt:           now,
				UpdatedAt:           now,
			}
			if err := tx.WithContext(ctx).Create(&user).Error; err != nil {
				return err
			}
		}

		var member organizationdomain.OrganizationMember
		err = tx.WithContext(ctx).
			Where("org_id = ? AND user_id = ?", org.ID, user.ID).
			First(&member).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			now := time.Now().UTC()
			member = organizationdomain.OrganizationMember{
				ID:        node.Generate(),
				OrgID:     org.ID,
				UserID:    user.ID,
				Role:      organizationdomain.RoleOwner,
				CreatedAt: now,
			}
			if err := tx.WithContext(ctx).Create(&member).Error; err != nil {
				return err
			}
		}

		_, err = ensureInvoiceSequenceTx(ctx, tx, node, org.ID)
		_, err = ensureInvoiceTemplateTx(ctx, tx, node, org.ID)
		err = ensureLedgerAccounts(ctx, tx, node, org.ID)

		return nil
	})
}

func ensureMainOrgTx(ctx context.Context, tx *gorm.DB, node *snowflake.Node) (organizationdomain.Organization, error) {
	var org organizationdomain.Organization
	err := tx.WithContext(ctx).Where("slug = ?", defaultOrgSlug).First(&org).Error
	if err == nil {
		return org, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return org, err
	}
	now := time.Now().UTC()
	org = organizationdomain.Organization{
		ID:        node.Generate(),
		Name:      defaultOrgName,
		Slug:      defaultOrgSlug,
		IsDefault: true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := tx.WithContext(ctx).Create(&org).Error; err != nil {
		return org, err
	}
	return org, nil
}

func ensureInvoiceSequenceTx(ctx context.Context, tx *gorm.DB, node *snowflake.Node, orgID snowflake.ID) (invoicedomain.InvoiceSequence, error) {
	var seq invoicedomain.InvoiceSequence
	err := tx.WithContext(ctx).Where("org_id = ?", orgID).First(&seq).Error
	if err == nil {
		return seq, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return seq, err
	}
	now := time.Now().UTC()
	seq = invoicedomain.InvoiceSequence{
		OrgID:      orgID,
		NextNumber: 1,
		UpdatedAt:  now,
	}
	if err := tx.WithContext(ctx).Create(&seq).Error; err != nil {
		return seq, err
	}
	return seq, nil
}

func ensureInvoiceTemplateTx(ctx context.Context, tx *gorm.DB, node *snowflake.Node, orgID snowflake.ID) (invoicetemplatedomain.InvoiceTemplate, error) {
	var seq invoicetemplatedomain.InvoiceTemplate
	err := tx.WithContext(ctx).Where("org_id = ?", orgID).First(&seq).Error
	if err == nil {
		return seq, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return seq, err
	}

	header := map[string]any{
		"title":           "Invoice",
		"logo_url":        "",
		"company_name":    "{{org.name}}",
		"company_email":   "{{org.email}}",
		"company_address": "{{org.address}}",
		"bill_to_label":   "Bill to",
		"ship_to_label":   "Ship to",
	}

	footer := map[string]any{
		"note":  "Thank you for your business.",
		"legal": "This invoice is generated electronically and is valid without a signature.",
	}

	style := map[string]any{
		"table": map[string]any{
			"header_bg":       "#f1f5f9",
			"row_border":      "#e5e7eb",
			"font_size":       "12px",
			"font_family":     "Inter, system-ui, sans-serif",
			"accent_color":    "#22c55e",
			"primary_color":   "#0f172a",
			"secondary_color": "#64748b",
		},
	}

	now := time.Now().UTC()
	seq = invoicetemplatedomain.InvoiceTemplate{
		ID:        node.Generate(),
		OrgID:     orgID,
		Name:      "Default Invoice",
		Locale:    "en",
		Currency:  "USD",
		Header:    header,
		Footer:    footer,
		Style:     style,
		IsDefault: true,
		IsLocked:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := tx.WithContext(ctx).Create(&seq).Error; err != nil {
		return seq, err
	}

	return seq, nil
}

func ensureLedgerAccounts(ctx context.Context, db *gorm.DB, node *snowflake.Node, orgID snowflake.ID) error {
	type account struct {
		Code string
		Type string
		Name string
	}

	accounts := []account{
		{"accounts_receivable", "asset", "Accounts Receivable"},
		{"cash", "asset", "Cash / Bank"},

		{"revenue_usage", "income", "Usage Revenue"},
		{"revenue_flat", "income", "Subscription Revenue"},

		{"tax_payable", "liability", "Tax Payable"},
		{"credit_balance", "liability", "Customer Credit Balance"},
		{"refund_liability", "liability", "Refund Liability"},

		{"payment_fee_expense", "expense", "Payment Gateway Fees"},
		{"adjustment", "expense", "Billing Adjustment"},
	}

	for _, a := range accounts {
		err := db.WithContext(ctx).
			Exec(`
				INSERT INTO ledger_accounts (id, org_id, code, type, name)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (org_id, code) DO NOTHING
			`,
				node.Generate(),
				orgID,
				a.Code,
				a.Type,
				a.Name,
			).Error

		if err != nil {
			return err
		}
	}

	return nil
}
