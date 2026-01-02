package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	auditmasking "github.com/smallbiznis/valora/internal/audit/masking"
	"github.com/smallbiznis/valora/internal/config"
	"github.com/smallbiznis/valora/internal/orgcontext"
	"github.com/smallbiznis/valora/internal/paymentprovider/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB    *gorm.DB
	Log   *zap.Logger
	GenID *snowflake.Node
	Repo  domain.Repository
	Cfg   config.Config
	AuditSvc auditdomain.Service `optional:"true"`
}

type Service struct {
	db       *gorm.DB
	log      *zap.Logger
	repo     domain.Repository
	genID    *snowflake.Node
	encKey   []byte
	auditSvc auditdomain.Service
}

type encryptedPayload struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func New(p Params) domain.Service {
	secret := strings.TrimSpace(p.Cfg.PaymentProviderConfigSecret)
	var key []byte
	if secret != "" {
		sum := sha256.Sum256([]byte(secret))
		key = sum[:]
	}

	return &Service{
		db:       p.DB,
		log:      p.Log.Named("paymentprovider.service"),
		repo:     p.Repo,
		genID:    p.GenID,
		encKey:   key,
		auditSvc: p.AuditSvc,
	}
}

func (s *Service) ListCatalog(ctx context.Context) ([]domain.CatalogProviderResponse, error) {
	items, err := s.repo.ListCatalog(ctx, s.db)
	if err != nil {
		return nil, err
	}

	resp := make([]domain.CatalogProviderResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, domain.CatalogProviderResponse{
			Provider:        item.Provider,
			DisplayName:     item.DisplayName,
			Description:     item.Description,
			SupportsWebhook: item.SupportsWebhook,
			SupportsRefund:  item.SupportsRefund,
		})
	}

	return resp, nil
}

func (s *Service) ListConfigs(ctx context.Context) ([]domain.ConfigSummary, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.ListConfigs(ctx, s.db, orgID)
	if err != nil {
		return nil, err
	}

	resp := make([]domain.ConfigSummary, 0, len(items))
	for _, item := range items {
		resp = append(resp, domain.ConfigSummary{
			Provider:   item.Provider,
			IsActive:   item.IsActive,
			Configured: true,
		})
	}

	return resp, nil
}

func (s *Service) UpsertConfig(ctx context.Context, req domain.UpsertRequest) (*domain.ConfigSummary, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		return nil, domain.ErrInvalidProvider
	}

	catalog, err := s.repo.FindCatalog(ctx, s.db, provider)
	if err != nil {
		return nil, err
	}
	if catalog == nil {
		return nil, domain.ErrInvalidProvider
	}

	config := normalizeConfig(req.Config)
	if len(config) == 0 {
		return nil, domain.ErrInvalidConfig
	}

	encrypted, err := s.encryptConfig(config)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.FindConfig(ctx, s.db, orgID, provider)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	cfg := domain.ProviderConfig{
		ID:        s.genID.Generate().Int64(),
		OrgID:     orgID,
		Provider:  provider,
		Config:    encrypted,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if existing != nil {
		cfg.ID = existing.ID
		cfg.IsActive = existing.IsActive
		cfg.CreatedAt = existing.CreatedAt
	}

	if err := s.repo.UpsertConfig(ctx, s.db, &cfg); err != nil {
		return nil, err
	}

	resp := domain.ConfigSummary{
		Provider:   provider,
		IsActive:   cfg.IsActive,
		Configured: true,
	}

	if s.auditSvc != nil {
		action := "provider.rotate_secret"
		if existing == nil {
			action = "provider.enable"
		}
		targetID := provider
		metadata := map[string]any{
			"provider": provider,
		}
		if masked := auditmasking.MaskJSON(config); masked != nil {
			metadata["masked_fields"] = masked
		}
		_ = s.auditSvc.AuditLog(ctx, nil, "", nil, action, "payment_provider_config", &targetID, metadata)
	}

	return &resp, nil
}

func (s *Service) SetActive(ctx context.Context, provider string, isActive bool) (*domain.ConfigSummary, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return nil, domain.ErrInvalidProvider
	}

	catalog, err := s.repo.FindCatalog(ctx, s.db, provider)
	if err != nil {
		return nil, err
	}
	if catalog == nil {
		return nil, domain.ErrInvalidProvider
	}

	updated, err := s.repo.UpdateStatus(ctx, s.db, orgID, provider, isActive, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, domain.ErrNotFound
	}

	resp := domain.ConfigSummary{
		Provider:   provider,
		IsActive:   isActive,
		Configured: true,
	}

	if s.auditSvc != nil {
		action := "provider.disable"
		if isActive {
			action = "provider.enable"
		}
		targetID := provider
		metadata := map[string]any{
			"provider":  provider,
			"is_active": isActive,
		}
		_ = s.auditSvc.AuditLog(ctx, nil, "", nil, action, "payment_provider_config", &targetID, metadata)
	}

	return &resp, nil
}

func (s *Service) orgIDFromContext(ctx context.Context) (int64, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return 0, domain.ErrInvalidOrganization
	}
	return orgID, nil
}

func (s *Service) encryptConfig(config map[string]any) (datatypes.JSON, error) {
	if len(s.encKey) == 0 {
		return nil, domain.ErrEncryptionKeyMissing
	}

	payload, err := json.Marshal(config)
	if err != nil {
		return nil, domain.ErrInvalidConfig
	}

	block, err := aes.NewCipher(s.encKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, payload, nil)
	encoded := encryptedPayload{
		Version:    1,
		Nonce:      base64.RawStdEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext),
	}
	out, err := json.Marshal(encoded)
	if err != nil {
		return nil, err
	}

	return datatypes.JSON(out), nil
}

func normalizeConfig(config map[string]any) map[string]any {
	if len(config) == 0 {
		return nil
	}

	normalized := make(map[string]any, len(config))
	for key, value := range config {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" || value == nil {
			continue
		}

		switch cast := value.(type) {
		case string:
			trimmedValue := strings.TrimSpace(cast)
			if trimmedValue == "" {
				continue
			}
			normalized[trimmedKey] = trimmedValue
		default:
			normalized[trimmedKey] = cast
		}
	}

	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
