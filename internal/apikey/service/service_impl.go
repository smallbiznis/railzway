package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	apikeydomain "github.com/smallbiznis/valora/internal/apikey/domain"
	"github.com/smallbiznis/valora/internal/orgcontext"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	apiKeyPrefix              = "vk_live_key_"
	apiKeySecretBytes         = 32
	apiKeyRotationGracePeriod = 24 * time.Hour
)

type Params struct {
	fx.In

	DB    *gorm.DB
	Log   *zap.Logger
	GenID *snowflake.Node
	Repo  apikeydomain.Repository
}

type Service struct {
	db    *gorm.DB
	log   *zap.Logger
	repo  apikeydomain.Repository
	genID *snowflake.Node
}

func New(p Params) apikeydomain.Service {
	return &Service{
		db:    p.DB,
		log:   p.Log.Named("apikey.service"),
		repo:  p.Repo,
		genID: p.GenID,
	}
}

func (s *Service) List(ctx context.Context) ([]apikeydomain.Response, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.List(ctx, s.db, orgID)
	if err != nil {
		return nil, err
	}

	resp := make([]apikeydomain.Response, 0, len(items))
	for i := range items {
		resp = append(resp, s.toResponse(&items[i]))
	}

	return resp, nil
}

func (s *Service) Create(ctx context.Context, req apikeydomain.CreateRequest) (*apikeydomain.SecretResponse, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apikeydomain.ErrInvalidName
	}

	now := time.Now().UTC()
	id := s.genID.Generate()
	keyID := newKeyID(id)
	plain, hash, err := generateAPIKey(keyID)
	if err != nil {
		return nil, err
	}

	key := &apikeydomain.APIKey{
		ID:        id,
		OrgID:     orgID,
		KeyID:     keyID,
		Name:      name,
		Scope:     apikeydomain.ScopeUsageWrite,
		KeyHash:   hash,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Insert(ctx, s.db, key); err != nil {
		return nil, err
	}

	return &apikeydomain.SecretResponse{KeyID: key.KeyID, APIKey: plain}, nil
}

func (s *Service) Rotate(ctx context.Context, keyID string) (*apikeydomain.SecretResponse, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(keyID)
	if trimmed == "" {
		return nil, apikeydomain.ErrInvalidKeyID
	}

	var result *apikeydomain.SecretResponse
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := s.repo.FindByKeyID(ctx, tx, orgID, trimmed)
		if err != nil {
			return err
		}
		if current == nil || !current.IsActive || isExpired(current.ExpiresAt) {
			return apikeydomain.ErrNotFound
		}

		now := time.Now().UTC()
		current.ExpiresAt = ptrTime(now.Add(apiKeyRotationGracePeriod))
		current.UpdatedAt = now
		if err := s.repo.Update(ctx, tx, current); err != nil {
			return err
		}

		id := s.genID.Generate()
		newKeyID := newKeyID(id)
		plain, hash, err := generateAPIKey(newKeyID)
		if err != nil {
			return err
		}

		rotatedFrom := current.KeyID
		next := &apikeydomain.APIKey{
			ID:               id,
			OrgID:            orgID,
			KeyID:            newKeyID,
			Name:             current.Name,
			Scope:            current.Scope,
			KeyHash:          hash,
			IsActive:         true,
			CreatedAt:        now,
			UpdatedAt:        now,
			RotatedFromKeyID: &rotatedFrom,
		}

		if err := s.repo.Insert(ctx, tx, next); err != nil {
			return err
		}

		result = &apikeydomain.SecretResponse{KeyID: next.KeyID, APIKey: plain}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) Revoke(ctx context.Context, keyID string) error {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return err
	}

	trimmed := strings.TrimSpace(keyID)
	if trimmed == "" {
		return apikeydomain.ErrInvalidKeyID
	}

	key, err := s.repo.FindByKeyID(ctx, s.db, orgID, trimmed)
	if err != nil {
		return err
	}
	if key == nil {
		return apikeydomain.ErrNotFound
	}

	now := time.Now().UTC()
	key.IsActive = false
	key.UpdatedAt = now
	if key.ExpiresAt == nil || key.ExpiresAt.After(now) {
		key.ExpiresAt = &now
	}
	return s.repo.Update(ctx, s.db, key)
}

func (s *Service) orgIDFromContext(ctx context.Context) (snowflake.ID, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return 0, apikeydomain.ErrInvalidOrganization
	}
	return snowflake.ID(orgID), nil
}

func (s *Service) toResponse(key *apikeydomain.APIKey) apikeydomain.Response {
	return apikeydomain.Response{
		KeyID:            key.KeyID,
		Name:             key.Name,
		Scope:            key.Scope,
		IsActive:         key.IsActive,
		CreatedAt:        key.CreatedAt,
		LastUsedAt:       key.LastUsedAt,
		ExpiresAt:        key.ExpiresAt,
		RotatedFromKeyID: key.RotatedFromKeyID,
	}
}

func generateAPIKey(keyID string) (string, string, error) {
	secret := make([]byte, apiKeySecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", "", err
	}

	secretPart := hex.EncodeToString(secret)
	trimmed := strings.TrimPrefix(keyID, "key_")
	plain := fmt.Sprintf("%s%s_%s", apiKeyPrefix, trimmed, secretPart)
	return plain, hashAPIKey(plain), nil
}

func hashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func newKeyID(id snowflake.ID) string {
	return "key_" + strings.ToUpper(strconv.FormatInt(int64(id), 36))
}

func isExpired(expiresAt *time.Time) bool {
	if expiresAt == nil {
		return false
	}
	return time.Now().UTC().After(*expiresAt)
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
