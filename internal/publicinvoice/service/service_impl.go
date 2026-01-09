package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/config"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	paymentdomain "github.com/smallbiznis/valora/internal/payment/domain"
	paymentproviderdomain "github.com/smallbiznis/valora/internal/paymentprovider/domain"
	publicinvoicedomain "github.com/smallbiznis/valora/internal/publicinvoice/domain"
	"go.uber.org/fx"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB           *gorm.DB
	Repo         publicinvoicedomain.Repository
	ProviderRepo paymentproviderdomain.Repository
	Cfg          config.Config
}

type Service struct {
	db           *gorm.DB
	repo         publicinvoicedomain.Repository
	providerRepo paymentproviderdomain.Repository
	encKey       []byte
}

type encryptedPayload struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type invoiceTotals struct {
	subtotal int64
	tax      int64
	total    int64
}

type stripeConfig struct {
	secretKey string
	accountID string
}

func New(p Params) publicinvoicedomain.Service {
	secret := strings.TrimSpace(p.Cfg.PaymentProviderConfigSecret)
	var key []byte
	if secret != "" {
		sum := sha256.Sum256([]byte(secret))
		key = sum[:]
	}

	return &Service{
		db:           p.DB,
		repo:         p.Repo,
		providerRepo: p.ProviderRepo,
		encKey:       key,
	}
}

func (s *Service) GetInvoiceForPublicView(
	ctx context.Context,
	orgID snowflake.ID,
	token string,
) (*publicinvoicedomain.PublicInvoiceResponse, error) {
	row, err := s.loadPublicInvoice(ctx, orgID, token)
	if err != nil {
		return nil, err
	}
	if row == nil || !isInvoiceViewable(row.Status) {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}

	items, totals, err := s.loadInvoiceItems(ctx, row)
	if err != nil {
		return nil, err
	}

	settledAmount := s.loadInvoiceSettledAmount(ctx, row)
	view, status := s.buildPublicInvoiceView(row, items, totals, settledAmount)
	return &publicinvoicedomain.PublicInvoiceResponse{
		Status:  status,
		Invoice: view,
	}, nil
}

func (s *Service) GetInvoicePublicStatus(
	ctx context.Context,
	orgID snowflake.ID,
	token string,
) (publicinvoicedomain.PublicInvoiceStatus, error) {
	row, err := s.loadPublicInvoice(ctx, orgID, token)
	if err != nil {
		return publicinvoicedomain.PublicInvoiceStatusUnpaid, err
	}
	if row == nil || !isInvoiceViewable(row.Status) {
		return publicinvoicedomain.PublicInvoiceStatusUnpaid, publicinvoicedomain.ErrInvoiceUnavailable
	}

	return publicInvoiceStatus(row), nil
}

func (s *Service) CreateOrReusePaymentIntent(
	ctx context.Context,
	orgID snowflake.ID,
	token string,
) (*publicinvoicedomain.PaymentIntentResponse, error) {
	row, err := s.loadPublicInvoice(ctx, orgID, token)
	if err != nil {
		return nil, err
	}
	if row == nil || !isInvoiceViewable(row.Status) {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}
	if !isInvoicePayable(row.Status) {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}
	if invoicePaid(row) {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}

	amountToCharge, err := s.resolveInvoiceAmount(ctx, row)
	if err != nil {
		return nil, err
	}
	if amountToCharge <= 0 {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}

	cfg, err := s.loadStripeConfig(ctx, row.OrgID)
	if err != nil {
		if errors.Is(err, publicinvoicedomain.ErrInvoiceUnavailable) {
			return nil, publicinvoicedomain.ErrInvoiceUnavailable
		}
		return nil, err
	}

	client := newStripeClient(cfg.secretKey, cfg.accountID)
	intentID := readMetadataString(row.Metadata, "stripe_payment_intent_id")
	var intent stripePaymentIntent

	if intentID == "" {
		intent, err = client.createPaymentIntent(ctx, row, amountToCharge)
		if err != nil {
			return nil, err
		}
	} else {
		intent, err = client.retrievePaymentIntent(ctx, intentID)
		if err != nil {
			return nil, err
		}
		switch intent.Status {
		case "succeeded":
			return nil, publicinvoicedomain.ErrInvoiceUnavailable
		case "canceled":
			intent, err = client.createPaymentIntent(ctx, row, amountToCharge)
			if err != nil {
				return nil, err
			}
		default:
			if intent.Amount != amountToCharge {
				intent, err = client.updatePaymentIntentAmount(ctx, intent.ID, amountToCharge)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if err := s.updateInvoiceMetadata(ctx, row, intent.ID, cfg.accountID); err != nil {
		return nil, err
	}

	return &publicinvoicedomain.PaymentIntentResponse{ClientSecret: intent.ClientSecret}, nil
}

func (s *Service) ListPaymentMethods(
	ctx context.Context,
	orgID snowflake.ID,
) ([]publicinvoicedomain.PublicPaymentMethod, error) {
	rows, err := s.repo.ListPaymentMethods(ctx, s.db, orgID)
	if err != nil {
		return nil, err
	}

	methods := make([]publicinvoicedomain.PublicPaymentMethod, 0, len(rows))
	stripeKey := ""
	stripeKeyLoaded := false
	for _, row := range rows {
		method := publicinvoicedomain.PublicPaymentMethod{
			Provider:            row.Provider,
			Type:                paymentMethodType(row.Provider),
			DisplayName:         row.DisplayName,
			SupportsInstallment: false,
		}
		if strings.EqualFold(row.Provider, "stripe") {
			if !stripeKeyLoaded {
				key, err := s.loadStripePublishableKey(ctx, orgID)
				if err == nil {
					stripeKey = key
				}
				stripeKeyLoaded = true
			}
			method.PublishableKey = stripeKey
		}
		methods = append(methods, method)
	}

	return methods, nil
}

func (s *Service) loadPublicInvoice(
	ctx context.Context,
	orgID snowflake.ID,
	token string,
) (*publicinvoicedomain.InvoiceRecord, error) {
	token = strings.TrimSpace(token)
	if orgID == 0 || token == "" {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}
	row, err := s.repo.FindInvoiceByToken(ctx, s.db, orgID, token)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}
	if row.OrgID != orgID {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}
	return row, nil
}

func (s *Service) loadInvoiceItems(
	ctx context.Context,
	row *publicinvoicedomain.InvoiceRecord,
) ([]publicinvoicedomain.PublicInvoiceItem, invoiceTotals, error) {
	if row == nil {
		return nil, invoiceTotals{}, nil
	}
	items, err := s.repo.ListInvoiceItems(ctx, s.db, row.OrgID, row.ID)
	if err != nil {
		return nil, invoiceTotals{}, err
	}

	result := make([]publicinvoicedomain.PublicInvoiceItem, 0, len(items))
	var totals invoiceTotals
	for _, item := range items {
		result = append(result, publicinvoicedomain.PublicInvoiceItem{
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			Amount:      item.Amount,
			LineType:    item.LineType,
		})

		totals.total += item.Amount
		if strings.EqualFold(item.LineType, string(invoicedomain.InvoiceItemLineTypeTax)) {
			totals.tax += item.Amount
		} else {
			totals.subtotal += item.Amount
		}
	}

	return result, totals, nil
}

func (s *Service) loadInvoiceSettledAmount(
	ctx context.Context,
	row *publicinvoicedomain.InvoiceRecord,
) int64 {
	if row == nil || row.PaidAt != nil {
		return 0
	}
	currency := strings.ToUpper(strings.TrimSpace(row.Currency))
	if currency == "" {
		return 0
	}
	settled, err := s.repo.FindInvoiceSettledAmount(ctx, s.db, row.OrgID, row.ID, currency)
	if err != nil {
		return 0
	}
	return settled
}

func (s *Service) buildPublicInvoiceView(
	row *publicinvoicedomain.InvoiceRecord,
	items []publicinvoicedomain.PublicInvoiceItem,
	totals invoiceTotals,
	settledAmount int64,
) (publicinvoicedomain.PublicInvoiceView, publicinvoicedomain.PublicInvoiceStatus) {
	totalAmount := row.SubtotalAmount
	if totalAmount == 0 && totals.total > 0 {
		totalAmount = totals.total
	}

	taxAmount := totals.tax
	subtotalAmount := totals.subtotal
	if totalAmount > 0 {
		subtotalAmount = totalAmount - taxAmount
		if subtotalAmount < 0 {
			subtotalAmount = totalAmount
			taxAmount = 0
		}
	}

	amountPaid := int64(0)
	if row.PaidAt != nil {
		amountPaid = totalAmount
	} else if settledAmount > 0 {
		amountPaid = settledAmount
	} else {
		// Metadata can drift; only use it as an unpaid fallback until ledger is wired in.
		amountPaid = readMetadataAmount(row.Metadata, "amount_paid")
	}
	if amountPaid < 0 {
		amountPaid = 0
	}

	amountDue := totalAmount - amountPaid
	if amountDue < 0 {
		amountDue = 0
	}

	view := publicinvoicedomain.PublicInvoiceView{
		OrgID:          row.OrgID.String(),
		OrgName:        row.OrgName,
		InvoiceNumber:  row.InvoiceNumber,
		InvoiceStatus:  strings.TrimSpace(row.Status),
		IssueDate:      formatTimeRFC3339(row.IssuedAt),
		DueDate:        formatTimeRFC3339(row.DueAt),
		PaidDate:       formatTimeRFC3339(row.PaidAt),
		PaymentState:   string(publicInvoiceStatus(row)),
		BillToName:     row.CustomerName,
		BillToEmail:    row.CustomerEmail,
		Currency:       row.Currency,
		AmountDue:      amountDue,
		SubtotalAmount: subtotalAmount,
		TaxAmount:      taxAmount,
		TotalAmount:    totalAmount,
		Items:          items,
	}

	return view, publicInvoiceStatus(row)
}

func (s *Service) resolveInvoiceAmount(ctx context.Context, row *publicinvoicedomain.InvoiceRecord) (int64, error) {
	if row.SubtotalAmount > 0 {
		return row.SubtotalAmount, nil
	}
	_, totals, err := s.loadInvoiceItems(ctx, row)
	if err != nil {
		return 0, err
	}
	if totals.total > 0 {
		return totals.total, nil
	}
	return 0, nil
}

func (s *Service) loadStripeConfig(ctx context.Context, orgID snowflake.ID) (*stripeConfig, error) {
	if orgID == 0 {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}

	cfg, err := s.providerRepo.FindConfig(ctx, s.db, orgID.Int64(), "stripe")
	if err != nil {
		return nil, err
	}
	if cfg == nil || !cfg.IsActive {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}

	decrypted, err := s.decryptProviderConfig(cfg.Config)
	if err != nil {
		return nil, err
	}

	secret := readConfigString(decrypted, "api_key", "secret_key")
	if secret == "" {
		return nil, paymentdomain.ErrInvalidConfig
	}

	return &stripeConfig{
		secretKey: secret,
		accountID: readConfigString(decrypted, "stripe_account_id"),
	}, nil
}

func (s *Service) loadStripePublishableKey(ctx context.Context, orgID snowflake.ID) (string, error) {
	if orgID == 0 {
		return "", nil
	}

	cfg, err := s.providerRepo.FindConfig(ctx, s.db, orgID.Int64(), "stripe")
	if err != nil {
		return "", err
	}
	if cfg == nil || !cfg.IsActive {
		return "", nil
	}

	decrypted, err := s.decryptProviderConfig(cfg.Config)
	if err != nil {
		return "", err
	}

	return readConfigString(decrypted, "publishable_key", "public_key"), nil
}

func (s *Service) updateInvoiceMetadata(
	ctx context.Context,
	invoice *publicinvoicedomain.InvoiceRecord,
	paymentIntentID string,
	accountID string,
) error {
	if invoice == nil {
		return nil
	}
	metadata := invoice.Metadata
	if metadata == nil {
		metadata = datatypes.JSONMap{}
	}
	metadata["payment_provider"] = "stripe"
	metadata["stripe_payment_intent_id"] = paymentIntentID
	if accountID != "" {
		metadata["stripe_account_id"] = accountID
	}

	return s.repo.UpdateInvoiceMetadata(ctx, s.db, invoice.OrgID, invoice.ID, metadata, time.Now().UTC())
}

func (s *Service) decryptProviderConfig(encrypted datatypes.JSON) (map[string]any, error) {
	if len(s.encKey) == 0 {
		return nil, paymentproviderdomain.ErrEncryptionKeyMissing
	}
	if len(encrypted) == 0 {
		return nil, paymentdomain.ErrInvalidConfig
	}

	var payload encryptedPayload
	if err := json.Unmarshal(encrypted, &payload); err != nil {
		return nil, paymentdomain.ErrInvalidConfig
	}
	if payload.Version != 1 {
		return nil, paymentdomain.ErrInvalidConfig
	}

	nonce, err := base64.RawStdEncoding.DecodeString(payload.Nonce)
	if err != nil {
		return nil, paymentdomain.ErrInvalidConfig
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		return nil, paymentdomain.ErrInvalidConfig
	}

	block, err := aes.NewCipher(s.encKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, paymentdomain.ErrInvalidConfig
	}

	var out map[string]any
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, paymentdomain.ErrInvalidConfig
	}
	if len(out) == 0 {
		return nil, paymentdomain.ErrInvalidConfig
	}
	return out, nil
}

func isInvoiceViewable(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "ISSUED", "FINALIZED", "OPEN", "PROCESSING", "PAID", "VOID":
		return true
	case string(invoicedomain.InvoiceStatusDraft):
		return false
	default:
		return false
	}
}

func isInvoicePayable(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "ISSUED", "FINALIZED", "OPEN":
		return true
	default:
		return false
	}
}

func publicInvoiceStatus(row *publicinvoicedomain.InvoiceRecord) publicinvoicedomain.PublicInvoiceStatus {
	if row == nil {
		return publicinvoicedomain.PublicInvoiceStatusUnpaid
	}
	status := strings.ToUpper(strings.TrimSpace(row.Status))
	switch status {
	case "VOID":
		return publicinvoicedomain.PublicInvoiceStatusFailed
	}

	if invoicePaid(row) {
		return publicinvoicedomain.PublicInvoiceStatusPaid
	}
	if readMetadataString(row.Metadata, "payment_failed_at") != "" {
		return publicinvoicedomain.PublicInvoiceStatusFailed
	}
	return publicinvoicedomain.PublicInvoiceStatusUnpaid
}

func invoicePaid(row *publicinvoicedomain.InvoiceRecord) bool {
	if row == nil {
		return false
	}
	return row.PaidAt != nil
}

func readMetadataAmount(metadata datatypes.JSONMap, key string) int64 {
	if metadata == nil {
		return 0
	}
	value, ok := metadata[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed
		}
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func readMetadataString(metadata datatypes.JSONMap, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	}
	return ""
}

func readConfigString(config map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := config[key]
		if !ok {
			continue
		}
		if str, ok := value.(string); ok {
			return strings.TrimSpace(str)
		}
	}
	return ""
}

func paymentMethodType(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "stripe":
		return "card"
	case "manual":
		return "bank_transfer"
	case "midtrans", "xendit":
		return "local_payment"
	default:
		return "local_payment"
	}
}

func formatTimeRFC3339(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
