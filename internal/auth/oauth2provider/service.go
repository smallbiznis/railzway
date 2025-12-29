package oauth2provider

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	authdomain "github.com/smallbiznis/valora/internal/auth/domain"
	"go.uber.org/zap"
)

type Clock interface {
	Now() time.Time
}

type TokenGenerator interface {
	NewToken() (string, error)
}

type Service struct {
	cfg      Config
	store    Store
	users    authdomain.Repository
	clock    Clock
	tokenGen TokenGenerator
	log      *zap.Logger
}

type defaultClock struct{}

type defaultTokenGenerator struct{}

func (defaultClock) Now() time.Time {
	return time.Now().UTC()
}

func (defaultTokenGenerator) NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func NewService(cfg Config, store Store, users authdomain.Repository, log *zap.Logger) *Service {
	return &Service{
		cfg:      cfg,
		store:    store,
		users:    users,
		clock:    defaultClock{},
		tokenGen: defaultTokenGenerator{},
		log:      log.Named("auth.oauth2.provider"),
	}
}

type AuthorizeRequest struct {
	ClientID            string
	RedirectURI         string
	Scopes              []string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	UserID              snowflake.ID
}

type AuthorizeResult struct {
	Code        string
	RedirectURI string
	State       string
	ExpiresAt   time.Time
}

func (s *Service) Authorize(ctx context.Context, req AuthorizeRequest) (*AuthorizeResult, error) {
	if strings.TrimSpace(req.ClientID) == "" || strings.TrimSpace(req.RedirectURI) == "" {
		return nil, ErrInvalidRequest
	}
	if req.UserID == 0 {
		return nil, ErrInvalidRequest
	}
	if !s.isValidClientID(req.ClientID) {
		return nil, ErrInvalidClient
	}
	if err := validateRedirectURI(req.RedirectURI); err != nil {
		return nil, err
	}
	if err := validateScopes(req.Scopes); err != nil {
		return nil, err
	}

	challenge := strings.TrimSpace(req.CodeChallenge)
	method := strings.TrimSpace(req.CodeChallengeMethod)
	if challenge != "" && method != "" && !strings.EqualFold(method, "S256") {
		return nil, ErrInvalidRequest
	}

	rawCode, err := s.tokenGen.NewToken()
	if err != nil {
		return nil, err
	}

	expiresAt := s.clock.Now().Add(s.cfg.CodeTTL)

	var challengeHash *string
	var challengeMethod *string
	if challenge != "" {
		hash := hashString(challenge)
		challengeHash = &hash
		methodValue := "S256"
		challengeMethod = &methodValue
	}

	code := &AuthorizationCode{
		CodeHash:            hashToken(rawCode),
		ClientID:            req.ClientID,
		RedirectURI:         req.RedirectURI,
		UserID:              req.UserID,
		Scopes:              req.Scopes,
		CodeChallengeHash:   challengeHash,
		CodeChallengeMethod: challengeMethod,
		ExpiresAt:           expiresAt,
	}

	if err := s.store.CreateAuthorizationCode(ctx, code); err != nil {
		return nil, err
	}

	return &AuthorizeResult{
		Code:        rawCode,
		RedirectURI: req.RedirectURI,
		State:       req.State,
		ExpiresAt:   expiresAt,
	}, nil
}

type TokenRequest struct {
	ClientID     string
	ClientSecret string
	Code         string
	RedirectURI  string
	CodeVerifier string
}

type TokenResponse struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int
	ExpiresAt   time.Time
	Scopes      []string
	UserID      snowflake.ID
}

func (s *Service) ExchangeToken(ctx context.Context, req TokenRequest) (*TokenResponse, error) {
	if err := s.validateTokenRequest(req); err != nil {
		return nil, err
	}

	codeHash, code, now, err := s.loadAuthorizationCode(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := s.verifyCodeChallenge(req, code); err != nil {
		return nil, err
	}

	if err := s.markCodeUsed(ctx, codeHash, now); err != nil {
		return nil, err
	}

	accessToken, expiresAt, err := s.createAccessToken(ctx, code, now)
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(s.cfg.AccessTTL.Seconds()),
		ExpiresAt:   expiresAt,
		Scopes:      code.Scopes,
		UserID:      code.UserID,
	}, nil
}

func (s *Service) validateTokenRequest(req TokenRequest) error {
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.RedirectURI) == "" {
		return ErrInvalidRequest
	}
	if !s.isValidClientID(req.ClientID) || !s.isValidClientSecret(req.ClientSecret) {
		return ErrInvalidClient
	}
	return nil
}

func (s *Service) loadAuthorizationCode(ctx context.Context, req TokenRequest) (string, *AuthorizationCode, time.Time, error) {
	codeHash := hashToken(req.Code)
	code, err := s.store.GetAuthorizationCode(ctx, codeHash)
	if err != nil {
		return "", nil, time.Time{}, err
	}

	now := s.clock.Now()
	if code.UsedAt != nil {
		return "", nil, time.Time{}, ErrCodeUsed
	}
	if now.After(code.ExpiresAt) {
		return "", nil, time.Time{}, ErrCodeExpired
	}
	if code.ClientID != req.ClientID {
		return "", nil, time.Time{}, ErrInvalidClient
	}
	if code.RedirectURI != req.RedirectURI {
		return "", nil, time.Time{}, ErrInvalidRedirectURI
	}

	return codeHash, code, now, nil
}

func (s *Service) verifyCodeChallenge(req TokenRequest, code *AuthorizationCode) error {
	if code.CodeChallengeHash == nil {
		return nil
	}
	if strings.TrimSpace(req.CodeVerifier) == "" {
		return ErrPKCEMismatch
	}
	if !verifyPKCE(req.CodeVerifier, *code.CodeChallengeHash) {
		return ErrPKCEMismatch
	}
	return nil
}

func (s *Service) markCodeUsed(ctx context.Context, codeHash string, now time.Time) error {
	used, err := s.store.MarkAuthorizationCodeUsed(ctx, codeHash, now)
	if err != nil {
		return err
	}
	if !used {
		return ErrCodeUsed
	}
	return nil
}

func (s *Service) createAccessToken(ctx context.Context, code *AuthorizationCode, now time.Time) (string, time.Time, error) {
	accessToken, err := s.tokenGen.NewToken()
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt := now.Add(s.cfg.AccessTTL)
	token := &AccessToken{
		TokenHash: hashToken(accessToken),
		ClientID:  code.ClientID,
		UserID:    code.UserID,
		Scopes:    code.Scopes,
		ExpiresAt: expiresAt,
	}
	if err := s.store.CreateAccessToken(ctx, token); err != nil {
		return "", time.Time{}, err
	}
	return accessToken, expiresAt, nil
}

type UserInfo struct {
	Sub         string   `json:"sub"`
	Email       string   `json:"email"`
	Name        string   `json:"name,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`
	Orgs        []string `json:"orgs,omitempty"`
}

func (s *Service) UserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, ErrInvalidToken
	}

	tokenHash := hashToken(accessToken)
	stored, err := s.store.GetAccessToken(ctx, tokenHash)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	if stored.RevokedAt != nil || now.After(stored.ExpiresAt) {
		return nil, ErrInvalidToken
	}

	user, err := s.users.FindByID(ctx, stored.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if strings.TrimSpace(user.ExternalID) == "" {
		return nil, ErrInvalidToken
	}

	name := strings.TrimSpace(user.DisplayName)
	if name == "" {
		name = strings.TrimSpace(user.Email)
	}

	return &UserInfo{
		Sub:         user.ExternalID,
		Email:       user.Email,
		Name:        name,
		DisplayName: name,
	}, nil
}

func (s *Service) isValidClientID(value string) bool {
	if strings.TrimSpace(value) == "" || strings.TrimSpace(s.cfg.ClientID) == "" {
		return false
	}
	return subtleConstantEquals(value, s.cfg.ClientID)
}

func (s *Service) isValidClientSecret(value string) bool {
	if strings.TrimSpace(value) == "" || strings.TrimSpace(s.cfg.ClientSecret) == "" {
		return false
	}
	return subtleConstantEquals(value, s.cfg.ClientSecret)
}

func subtleConstantEquals(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func validateRedirectURI(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ErrInvalidRedirectURI
	}
	if strings.ToLower(parsed.Scheme) != "https" {
		return ErrInvalidRedirectURI
	}
	host := strings.ToLower(parsed.Hostname())
	if !strings.HasSuffix(host, ".usevalora.net") {
		return ErrInvalidRedirectURI
	}
	if strings.HasPrefix(parsed.Path, "/login/usevalora_cloud") {
		return nil
	}
	if strings.HasPrefix(parsed.Path, "/internal/auth/oauth/callback/usevalora_cloud") {
		return nil
	}
	return ErrInvalidRedirectURI
}

func validateScopes(scopes []string) error {
	if len(scopes) == 0 {
		return ErrInvalidScope
	}
	hasUserEmail := false
	hasOpenID := false
	hasEmail := false
	hasProfile := false
	for _, scope := range scopes {
		scope = strings.TrimSpace(strings.ToLower(scope))
		switch scope {
		case "user:email":
			hasUserEmail = true
		case "openid":
			hasOpenID = true
		case "email":
			hasEmail = true
		case "profile":
			hasProfile = true
		}
	}
	if hasUserEmail || (hasOpenID && hasEmail && hasProfile) {
		return nil
	}
	return ErrInvalidScope
}

func parseScopeList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	if len(parts) == 0 {
		return nil
	}
	return parts
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func hashString(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func verifyPKCE(verifier string, expectedChallengeHash string) bool {
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])
	computed := hashString(challenge)
	return subtleConstantEquals(computed, expectedChallengeHash)
}
