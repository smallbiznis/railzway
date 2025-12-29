package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	authconfig "github.com/smallbiznis/valora/internal/auth/config"
)

const (
	defaultTokenSize = 32
)

type Service interface {
	RedirectURL(ctx context.Context, providerName string, req RedirectRequest) (*RedirectResult, error)
	Login(ctx context.Context, providerName string, req LoginRequest) (*LoginResult, error)
}

type RedirectRequest struct {
	RedirectURI string
}

type RedirectResult struct {
	URL         string
	State       string
	CodeVerifier string
}

type LoginRequest struct {
	Code         string
	RedirectURI  string
	CodeVerifier string
}

type LoginResult struct {
	ProviderName string
	AllowSignUp  bool
	Identity     Identity
}

type Identity struct {
	ExternalID  string
	Email       string
	DisplayName string
}

type service struct {
	registry   authconfig.AuthProviderRegistry
	httpClient *http.Client
}

func NewService(registry authconfig.AuthProviderRegistry) Service {
	return &service{
		registry:   registry,
		httpClient: http.DefaultClient,
	}
}

func (s *service) RedirectURL(ctx context.Context, providerName string, req RedirectRequest) (*RedirectResult, error) {
	_ = ctx

	cfg, err := s.lookupProvider(providerName)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.AuthURL) == "" {
		return nil, ErrInvalidProvider
	}
	if strings.TrimSpace(req.RedirectURI) == "" {
		return nil, ErrInvalidRequest
	}

	state, err := randomToken(defaultTokenSize)
	if err != nil {
		return nil, err
	}
	verifier, err := randomToken(defaultTokenSize)
	if err != nil {
		return nil, err
	}

	authURL, err := buildAuthURL(cfg, req.RedirectURI, state, pkceChallenge(verifier))
	if err != nil {
		return nil, err
	}

	return &RedirectResult{
		URL:          authURL,
		State:        state,
		CodeVerifier: verifier,
	}, nil
}

func (s *service) Login(ctx context.Context, providerName string, req LoginRequest) (*LoginResult, error) {
	cfg, err := s.lookupProvider(providerName)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Code) == "" {
		return nil, ErrInvalidRequest
	}
	if strings.TrimSpace(cfg.TokenURL) == "" || strings.TrimSpace(cfg.APIURL) == "" {
		return nil, ErrInvalidProvider
	}

	token, err := s.exchangeCode(ctx, cfg, req.Code, req.RedirectURI, req.CodeVerifier)
	if err != nil {
		return nil, err
	}

	identity, err := s.fetchIdentity(ctx, cfg, token)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		ProviderName: cfg.Type,
		AllowSignUp:  cfg.AllowSignUp,
		Identity:     identity,
	}, nil
}

func (s *service) lookupProvider(rawName string) (authconfig.AuthProviderConfig, error) {
	name := strings.ToLower(strings.TrimSpace(rawName))
	if name == "" {
		return authconfig.AuthProviderConfig{}, ErrProviderNotFound
	}

	cfg, ok := s.registry.Active[name]
	if !ok {
		return authconfig.AuthProviderConfig{}, ErrProviderNotFound
	}
	if cfg.Type == "" || !cfg.Enabled {
		return authconfig.AuthProviderConfig{}, ErrProviderNotFound
	}
	return cfg, nil
}

func buildAuthURL(cfg authconfig.AuthProviderConfig, redirectURI, state, challenge string) (string, error) {
	parsed, err := url.Parse(cfg.AuthURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", cfg.ClientID)
	query.Set("redirect_uri", redirectURI)
	if len(cfg.Scopes) > 0 {
		query.Set("scope", strings.Join(cfg.Scopes, " "))
	}
	query.Set("state", state)
	if challenge != "" {
		query.Set("code_challenge", challenge)
		query.Set("code_challenge_method", "S256")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	IDToken     string `json:"id_token"`
}

func (s *service) exchangeCode(ctx context.Context, cfg authconfig.AuthProviderConfig, code, redirectURI, verifier string) (*tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", cfg.ClientID)
	if strings.TrimSpace(cfg.ClientSecret) != "" {
		form.Set("client_secret", cfg.ClientSecret)
	}
	if strings.TrimSpace(verifier) != "" {
		form.Set("code_verifier", verifier)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, ErrUnauthorized
	}

	var token tokenResponse
	if err := json.Unmarshal(body, &token); err == nil && token.AccessToken != "" {
		return &token, nil
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, ErrUnauthorized
	}
	token.AccessToken = values.Get("access_token")
	token.TokenType = values.Get("token_type")
	token.IDToken = values.Get("id_token")
	if token.AccessToken == "" {
		return nil, ErrUnauthorized
	}
	return &token, nil
}

func (s *service) fetchIdentity(ctx context.Context, cfg authconfig.AuthProviderConfig, token *tokenResponse) (Identity, error) {
	if token == nil || strings.TrimSpace(token.AccessToken) == "" {
		return Identity{}, ErrUnauthorized
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.APIURL, nil)
	if err != nil {
		return Identity{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return Identity{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Identity{}, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Identity{}, ErrUnauthorized
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return Identity{}, ErrUnauthorized
	}

	identity := Identity{
		ExternalID:  firstClaim(payload, "sub", "id", "user_id", "uid"),
		Email:       firstClaim(payload, "email"),
		DisplayName: firstClaim(payload, "name", "display_name", "login", "username", "preferred_username"),
	}
	if identity.DisplayName == "" {
		identity.DisplayName = identity.Email
	}
	if identity.ExternalID == "" || identity.Email == "" {
		return Identity{}, ErrUnauthorized
	}

	return identity, nil
}

func firstClaim(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			if str := claimToString(value); str != "" {
				return str
			}
		}
	}
	return ""
}

func claimToString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func randomToken(size int) (string, error) {
	if size <= 0 {
		size = defaultTokenSize
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
