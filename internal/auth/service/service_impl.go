package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/google/uuid"
	"github.com/smallbiznis/valora/internal/auth/domain"
	"github.com/smallbiznis/valora/internal/auth/password"
	"go.uber.org/zap"
)

const (
	sessionTokenBytes = 32
	sessionTTL        = 7 * 24 * time.Hour

	minPasswordLength = 8
)

type Service struct {
	log         *zap.Logger
	repo        domain.Repository
	sessionRepo domain.SessionRepository
	genID       *snowflake.Node
}

func New(log *zap.Logger, repo domain.Repository, sessionRepo domain.SessionRepository, genID *snowflake.Node) domain.Service {
	return &Service{
		log:         log.Named("auth.service"),
		repo:        repo,
		sessionRepo: sessionRepo,
		genID:       genID,
	}
}

func (s *Service) CreateUser(ctx context.Context, req domain.CreateUserRequest) (*domain.User, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	if strings.TrimSpace(req.Password) == "" || len(strings.TrimSpace(req.Password)) < minPasswordLength {
		return nil, domain.ErrInvalidCredentials
	}

	if _, err := s.repo.FindOne(ctx, domain.User{
		Email: email,
	}); err == nil {
		return nil, domain.ErrUserExists
	} else if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, err
	}

	hashed, err := password.Hash(req.Password)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(req.Username)
	}
	if displayName == "" {
		displayName = defaultDisplayName(email)
	}
	user := &domain.User{
		ID:                  s.genID.Generate(),
		ExternalID:          newExternalID(),
		Provider:            "local",
		DisplayName:         displayName,
		Email:               email,
		PasswordHash:        &hashed,
		IsDefault:           false,
		LastPasswordChanged: &now,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Service) Login(ctx context.Context, req domain.LoginRequest) (*domain.LoginResult, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	if strings.TrimSpace(req.Password) == "" {
		return nil, domain.ErrInvalidCredentials
	}

	user, err := s.repo.FindOne(ctx, domain.User{
		Email:    email,
		Provider: "local",
	})
	if err != nil {
		fmt.Printf("err: %v\n", err)
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}

	if user.PasswordHash == nil || !password.Verify(req.Password, *user.PasswordHash) {
		fmt.Printf("invalid_credentials")
		return nil, domain.ErrInvalidCredentials
	}

	rawToken, err := newSessionToken()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	session := &domain.Session{
		ID:               s.genID.Generate(),
		UserID:           user.ID,
		SessionTokenHash: hashToken(rawToken),
		UserAgent:        strings.TrimSpace(req.UserAgent),
		IPAddress:        strings.TrimSpace(req.IPAddress),
		OrgIDs:           []int64{},
		ExpiresAt:        now.Add(sessionTTL),
		CreatedAt:        now,
		LastSeenAt:       now,
	}
	if err := s.sessionRepo.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	mustChangePassword := user.IsDefault || user.LastPasswordChanged == nil
	passwordState := "rotated"
	if mustChangePassword {
		passwordState = "default"
	}

	return &domain.LoginResult{
		Session: &domain.SessionView{
			Metadata: map[string]any{
				"user_id":               user.ID.String(),
				"external_id":           user.ExternalID,
				"provider":              user.Provider,
				"display_name":          user.DisplayName,
				"email":                 user.Email,
				"is_default":            user.IsDefault,
				"last_password_changed": user.LastPasswordChanged,
				"must_change_password":  mustChangePassword,
				"password_state":        passwordState,
				"auth_provider":         "local",
			},
		},
		RawToken:  rawToken,
		ExpiresAt: session.ExpiresAt,
		SessionID: session.ID,
	}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return domain.ErrInvalidSession
	}

	session, err := s.sessionRepo.GetSessionByTokenHash(ctx, hashToken(token))
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return domain.ErrInvalidSession
		}
		return err
	}

	now := time.Now().UTC()
	return s.sessionRepo.RevokeSession(ctx, session.ID, now)
}

func (s *Service) Authenticate(ctx context.Context, rawToken string) (*domain.Session, error) {
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return nil, domain.ErrInvalidSession
	}

	session, err := s.sessionRepo.GetSessionByTokenHash(ctx, hashToken(token))
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return nil, domain.ErrInvalidSession
		}
		return nil, err
	}

	now := time.Now().UTC()
	if session.RevokedAt != nil {
		return nil, domain.ErrSessionRevoked
	}
	if now.After(session.ExpiresAt) {
		return nil, domain.ErrSessionExpired
	}

	if err := s.sessionRepo.UpdateLastSeen(ctx, session.ID, now); err != nil {
		return nil, err
	}

	return session, nil
}

func (s *Service) UpdateSessionOrgContext(ctx context.Context, sessionID snowflake.ID, activeOrgID *int64, orgIDs []int64) error {
	return s.sessionRepo.UpdateOrgContext(ctx, sessionID, activeOrgID, orgIDs)
}

func (s *Service) ChangePassword(ctx context.Context, userID string, newPassword string) error {
	if newPassword == "" {
		return domain.ErrInvalidCredentials
	}

	id, err := snowflake.ParseString(userID)
	if err != nil {
		return err
	}

	_, err = s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	hashed, err := password.Hash(newPassword)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	fields := map[string]any{
		"password_hash":         hashed,
		"last_password_changed": &now,
		"is_default":            false,
		"updated_at":            now,
	}

	return s.repo.UpdateFields(ctx, id, fields)
}

func (s *Service) CurrentUser(ctx context.Context) (*domain.User, error) {
	_ = ctx
	return nil, domain.ErrUserNotFound
}

func normalizeEmail(raw string) (string, error) {
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(addr.Address)), nil
}

func defaultDisplayName(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		return strings.TrimSpace(parts[0])
	}
	return email
}

func newExternalID() string {
	return uuid.NewString()
}

func newSessionToken() (string, error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
