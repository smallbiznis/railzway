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
	"github.com/smallbiznis/railzway/internal/config"
	invoicedomain "github.com/smallbiznis/railzway/internal/invoice/domain"
	paymentdomain "github.com/smallbiznis/railzway/internal/payment/domain"
	paymentproviderdomain "github.com/smallbiznis/railzway/internal/providers/payment/domain"
	publicinvoicedomain "github.com/smallbiznis/railzway/internal/publicinvoice/domain"
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

	items, err := s.loadInvoiceItems(ctx, row)
	if err != nil {
		return nil, err
	}

	settledAmount := s.loadInvoiceSettledAmount(ctx, row)
	view, status := s.buildPublicInvoiceView(row, items, settledAmount)
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

func (s *Service) CreateCheckoutSession(
	ctx context.Context,
	orgID snowflake.ID,
	token string,
	provider string,
) (*publicinvoicedomain.CheckoutSessionResponse, error) {
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

	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "stripe" // Default fallback
	}

	switch provider {
	case "stripe":
		return s.createStripeSession(ctx, row, amountToCharge)
	case "adyen":
		return s.createAdyenSession(ctx, row, amountToCharge, token)
	case "braintree":
		return s.createBraintreeSession(ctx, row, amountToCharge)
	default:
		return nil, paymentdomain.ErrInvalidProvider
	}
}

func (s *Service) ProcessCheckoutSession(
	ctx context.Context,
	orgID snowflake.ID,
	token string,
	provider string,
	payload map[string]any,
) (*publicinvoicedomain.ProcessSessionResponse, error) {
	row, err := s.loadPublicInvoice(ctx, orgID, token)
	if err != nil {
		return nil, err
	}
	if row == nil || !isInvoiceViewable(row.Status) {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}

	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "braintree":
		return s.processBraintreePayment(ctx, row, payload)
	default:
		return nil, errors.New("provider_does_not_require_manual_processing")
	}
}

func (s *Service) processBraintreePayment(
	ctx context.Context,
	row *publicinvoicedomain.InvoiceRecord,
	payload map[string]any,
) (*publicinvoicedomain.ProcessSessionResponse, error) {
	nonce, ok := payload["nonce"].(string)
	if !ok || nonce == "" {
		return nil, errors.New("missing_nonce")
	}

	amount, err := s.resolveInvoiceAmount(ctx, row)
	if err != nil {
		return nil, err
	}

	config, err := s.providerRepo.FindConfig(ctx, s.db, row.OrgID.Int64(), "braintree")
	if err != nil || config == nil || !config.IsActive {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}

	decrypted, err := s.decryptProviderConfig(config.Config)
	if err != nil {
		return nil, err
	}

	merchantID := readConfigString(decrypted, "merchant_id")
	publicKey := readConfigString(decrypted, "public_key")
	privateKey := readConfigString(decrypted, "private_key")

	client := newBraintreeClient(merchantID, publicKey, privateKey)
	
	// Create transaction
	txID, err := client.createTransaction(ctx, nonce, amount)
	if err != nil {
		return &publicinvoicedomain.ProcessSessionResponse{
			Success:        false,
			FailureMessage: err.Error(),
		}, nil
	}

	if err := s.updateInvoiceMetadata(ctx, row, "braintree", txID, merchantID); err != nil {
		// Log error but payment succeeded
	}

	return &publicinvoicedomain.ProcessSessionResponse{
		Success:   true,
		PaymentID: txID,
		Status:    "succeeded",
	}, nil
}

func (s *Service) createStripeSession(
	ctx context.Context,
	row *publicinvoicedomain.InvoiceRecord,
	amount int64,
) (*publicinvoicedomain.CheckoutSessionResponse, error) {
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
		intent, err = client.createPaymentIntent(ctx, row, amount)
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
			intent, err = client.createPaymentIntent(ctx, row, amount)
			if err != nil {
				return nil, err
			}
		default:
			if intent.Amount != amount {
				intent, err = client.updatePaymentIntentAmount(ctx, intent.ID, amount)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if err := s.updateInvoiceMetadata(ctx, row, "stripe", intent.ID, cfg.accountID); err != nil {
		return nil, err
	}

	pubKey := readMetadataString(row.Metadata, "stripe_publishable_key") // Optimization: store pk?
	// Or just reload it?
	if pubKey == "" {
		pubKey, _ = s.loadStripePublishableKey(ctx, row.OrgID)
	}

	return &publicinvoicedomain.CheckoutSessionResponse{
		Provider:     "stripe",
		SessionToken: intent.ClientSecret,
		PublicConfig: map[string]any{
			"publishable_key": pubKey,
			"account_id":      cfg.accountID,
		},
		Metadata: map[string]any{
			"payment_intent_id": intent.ID,
		},
	}, nil
}

func (s *Service) createAdyenSession(
	ctx context.Context,
	row *publicinvoicedomain.InvoiceRecord,
	amount int64,
	token string,
) (*publicinvoicedomain.CheckoutSessionResponse, error) {
	config, err := s.providerRepo.FindConfig(ctx, s.db, row.OrgID.Int64(), "adyen")
	if err != nil || config == nil || !config.IsActive {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}

	decrypted, err := s.decryptProviderConfig(config.Config)
	if err != nil {
		return nil, err
	}

	apiKey := readConfigString(decrypted, "api_key")
	merchantAccount := readConfigString(decrypted, "merchant_account")
	environment := readConfigString(decrypted, "environment")
	clientKey := readConfigString(decrypted, "client_key") // Public key for frontend

	if apiKey == "" || merchantAccount == "" {
		return nil, paymentdomain.ErrInvalidConfig
	}

	client := newAdyenClient(apiKey, merchantAccount, environment)

	// Return URL should typically be the invoice page itself
	// We might need to construct it from a known base URL or pass it in.
	// For now, assuming a frontend route convention: /invoices/{token}
	returnURL := "https://valora.smallbiznis.com/invoices/" + token // TODO: Make configurable

	resp, err := client.createSession(ctx, row, amount, returnURL)
	if err != nil {
		return nil, err
	}

	if err := s.updateInvoiceMetadata(ctx, row, "adyen", resp.ID, merchantAccount); err != nil {
		return nil, err
	}

	return &publicinvoicedomain.CheckoutSessionResponse{
		Provider:     "adyen",
		SessionToken: resp.SessionData,
		PublicConfig: map[string]any{
			"client_key":       clientKey,
			"environment":      environment,
			"merchant_account": merchantAccount,
		},
		Metadata: map[string]any{
			"session_id": resp.ID,
		},
	}, nil
}

func (s *Service) createBraintreeSession(
	ctx context.Context,
	row *publicinvoicedomain.InvoiceRecord,
	amount int64,
) (*publicinvoicedomain.CheckoutSessionResponse, error) {
	config, err := s.providerRepo.FindConfig(ctx, s.db, row.OrgID.Int64(), "braintree")
	if err != nil || config == nil || !config.IsActive {
		return nil, publicinvoicedomain.ErrInvoiceUnavailable
	}

	decrypted, err := s.decryptProviderConfig(config.Config)
	if err != nil {
		return nil, err
	}

	merchantID := readConfigString(decrypted, "merchant_id")
	publicKey := readConfigString(decrypted, "public_key")
	privateKey := readConfigString(decrypted, "private_key")

	if merchantID == "" || publicKey == "" || privateKey == "" {
		return nil, paymentdomain.ErrInvalidConfig
	}

	client := newBraintreeClient(merchantID, publicKey, privateKey)

	token, err := client.generateClientToken(ctx, row.CustomerID.String())
	if err != nil {
		return nil, err
	}

	// Braintree doesn't have a specific "session ID" at this stage, the token is enough.
	if err := s.updateInvoiceMetadata(ctx, row, "braintree", "", merchantID); err != nil {
		return nil, err
	}

	return &publicinvoicedomain.CheckoutSessionResponse{
		Provider:     "braintree",
		SessionToken: token, // This is the 'client_token' used by drop-in UI
		PublicConfig: map[string]any{
			// Braintree frontend usually just needs the client token
		},
		Metadata: map[string]any{
			"amount": amount,
		},
	}, nil
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
) ([]publicinvoicedomain.PublicInvoiceItem, error) {
	if row == nil {
		return nil, nil
	}
	items, err := s.repo.ListInvoiceItems(ctx, s.db, row.OrgID, row.ID)
	if err != nil {
		return nil, err
	}

	result := make([]publicinvoicedomain.PublicInvoiceItem, 0, len(items))
	for _, item := range items {
		result = append(result, publicinvoicedomain.PublicInvoiceItem{
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			Amount:      item.Amount,
			LineType:    item.LineType,
		})
	}

	return result, nil
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
	settledAmount int64,
) (publicinvoicedomain.PublicInvoiceView, publicinvoicedomain.PublicInvoiceStatus) {
	// Strict billing: Use persisted totals only.
	// Frontend/Service should never recompute totals.
	totalAmount := row.TotalAmount
	subtotalAmount := row.SubtotalAmount
	taxAmount := row.TaxAmount

	amountPaid := int64(0)
	if row.PaidAt != nil {
		amountPaid = totalAmount
	} else if settledAmount > 0 {
		// Ledgers are the source of truth for partial payments (credits)
		amountPaid = settledAmount
	} else {
		// Fallback for non-ledger systems (should be deprecated)
		amountPaid = readMetadataAmount(row.Metadata, "amount_paid")
	}

	// Safety clamping
	if amountPaid < 0 {
		amountPaid = 0
	}
	if amountPaid > totalAmount {
		amountPaid = totalAmount
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
	return row.TotalAmount, nil
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
	provider string,
	paymentID string,
	accountID string,
) error {
	if invoice == nil {
		return nil
	}
	metadata := invoice.Metadata
	if metadata == nil {
		metadata = datatypes.JSONMap{}
	}
	metadata["payment_provider"] = provider

	switch provider {
	case "stripe":
		metadata["stripe_payment_intent_id"] = paymentID
		if accountID != "" {
			metadata["stripe_account_id"] = accountID
		}
	case "adyen":
		metadata["adyen_psp_reference"] = paymentID
	case "braintree":
		metadata["braintree_transaction_id"] = paymentID
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
