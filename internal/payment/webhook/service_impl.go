package webhook

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/config"
	"github.com/smallbiznis/valora/internal/payment/adapters"
	disputedomain "github.com/smallbiznis/valora/internal/payment/dispute/domain"
	disputeservice "github.com/smallbiznis/valora/internal/payment/dispute/service"
	paymentdomain "github.com/smallbiznis/valora/internal/payment/domain"
	paymentservice "github.com/smallbiznis/valora/internal/payment/service"
	paymentproviderdomain "github.com/smallbiznis/valora/internal/paymentprovider/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB         *gorm.DB
	Log        *zap.Logger
	PaymentSvc *paymentservice.Service
	DisputeSvc *disputeservice.Service
	Adapters   *adapters.Registry
	Cfg        config.Config
}

type Service struct {
	db         *gorm.DB
	log        *zap.Logger
	paymentSvc *paymentservice.Service
	disputeSvc *disputeservice.Service
	adapters   *adapters.Registry
	encKey     []byte
}

type encryptedPayload struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type providerConfigRow struct {
	OrgID  snowflake.ID
	Config datatypes.JSON
}

func NewService(p Params) paymentdomain.Service {
	secret := strings.TrimSpace(p.Cfg.PaymentProviderConfigSecret)
	var key []byte
	if secret != "" {
		sum := sha256.Sum256([]byte(secret))
		key = sum[:]
	}

	return &Service{
		db:         p.DB,
		log:        p.Log.Named("payment.webhook"),
		paymentSvc: p.PaymentSvc,
		disputeSvc: p.DisputeSvc,
		adapters:   p.Adapters,
		encKey:     key,
	}
}

func (s *Service) IngestWebhook(ctx context.Context, provider string, payload []byte, headers http.Header) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return paymentdomain.ErrInvalidProvider
	}
	if s.adapters == nil || !s.adapters.ProviderExists(provider) {
		return paymentdomain.ErrProviderNotFound
	}
	if !json.Valid(payload) {
		return paymentdomain.ErrInvalidPayload
	}

	configs, err := s.listActiveConfigs(ctx, provider)
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		return paymentdomain.ErrProviderNotFound
	}

	_, paymentEvent, disputeEvent, err := s.matchAdapter(ctx, provider, payload, headers, configs)
	if err != nil {
		if errors.Is(err, paymentdomain.ErrEventIgnored) {
			return nil
		}
		if errors.Is(err, paymentdomain.ErrInvalidCustomer) {
			s.log.Warn("payment webhook missing customer mapping", zap.String("provider", provider))
		}
		return err
	}

	if disputeEvent != nil {
		if s.disputeSvc == nil {
			return errors.New("dispute_service_unavailable")
		}
		if disputeEvent.RawPayload == nil {
			disputeEvent.RawPayload = payload
		}
		return s.disputeSvc.ProcessEvent(ctx, disputeEvent)
	}

	if paymentEvent == nil {
		return paymentdomain.ErrInvalidSignature
	}
	if s.paymentSvc == nil {
		return errors.New("payment_service_unavailable")
	}
	if paymentEvent.RawPayload == nil {
		paymentEvent.RawPayload = payload
	}
	return s.paymentSvc.ProcessEvent(ctx, paymentEvent, payload)
}

func (s *Service) listActiveConfigs(ctx context.Context, provider string) ([]providerConfigRow, error) {
	var rows []providerConfigRow
	err := s.db.WithContext(ctx).Raw(
		`SELECT org_id, config
		 FROM payment_provider_configs
		 WHERE provider = ? AND is_active = TRUE`,
		provider,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) matchAdapter(
	ctx context.Context,
	provider string,
	payload []byte,
	headers http.Header,
	configs []providerConfigRow,
) (paymentdomain.PaymentAdapter, *paymentdomain.PaymentEvent, *disputedomain.DisputeEvent, error) {
	var configErr error
	for _, cfg := range configs {
		decrypted, err := s.decryptConfig(cfg.Config)
		if err != nil {
			if errors.Is(err, paymentproviderdomain.ErrEncryptionKeyMissing) {
				return nil, nil, nil, err
			}
			configErr = err
			continue
		}

		adapter, err := s.adapters.NewAdapter(provider, paymentdomain.AdapterConfig{
			OrgID:    cfg.OrgID,
			Provider: provider,
			Config:   decrypted,
		})
		if err != nil {
			configErr = err
			continue
		}

		if err := adapter.Verify(ctx, payload, headers); err != nil {
			if errors.Is(err, paymentdomain.ErrInvalidSignature) {
				continue
			}
			return nil, nil, nil, err
		}

		if disputeAdapter, ok := adapter.(disputedomain.DisputeAdapter); ok {
			disputeEvent, err := disputeAdapter.ParseDispute(ctx, payload)
			if err == nil {
				disputeEvent.Provider = provider
				disputeEvent.OrgID = cfg.OrgID
				return adapter, nil, disputeEvent, nil
			}
			if !errors.Is(err, paymentdomain.ErrEventIgnored) {
				return nil, nil, nil, err
			}
		}

		paymentEvent, err := adapter.Parse(ctx, payload)
		if err != nil {
			if errors.Is(err, paymentdomain.ErrEventIgnored) {
				return adapter, nil, nil, err
			}
			return nil, nil, nil, err
		}
		paymentEvent.Provider = provider
		paymentEvent.OrgID = cfg.OrgID
		return adapter, paymentEvent, nil, nil
	}

	if configErr != nil {
		return nil, nil, nil, configErr
	}
	return nil, nil, nil, paymentdomain.ErrInvalidSignature
}

func (s *Service) decryptConfig(encrypted datatypes.JSON) (map[string]any, error) {
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
