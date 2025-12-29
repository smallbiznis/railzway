package server

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	authdomain "github.com/smallbiznis/valora/internal/auth/domain"
	authoauth "github.com/smallbiznis/valora/internal/auth/oauth"
	orgdomain "github.com/smallbiznis/valora/internal/organization/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	oauthStateCookie     = "oauth_state"
	oauthVerifierCookie  = "oauth_code_verifier"
	oauthRedirectCookie  = "oauth_redirect_to"
	oauthStateTTL        = 10 * time.Minute
	oauthSessionTTL      = 7 * 24 * time.Hour
	sessionTokenSize     = 32
	oauthErrorRedirectTo = "/login?error=oauth_login"
)

type oauthIdentity struct {
	ExternalID  string
	Email       string
	DisplayName string
}

func (s *Server) OAuthLogin(c *gin.Context) {
	provider := strings.TrimSpace(c.Param("name"))
	if provider == "" {
		AbortWithError(c, ErrNotFound)
		return
	}

	if strings.TrimSpace(c.Query("error")) != "" {
		logOAuthError(c, provider)
		s.clearOAuthCookies(c)
		redirectToOAuthError(c)
		return
	}

	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		if err := s.startOAuthLogin(c, provider); err != nil {
			handleOAuthError(c, provider, err)
		}
		return
	}

	if err := s.handleOAuthCallback(c, provider, code); err != nil {
		handleOAuthError(c, provider, err)
	}
}

func (s *Server) startOAuthLogin(c *gin.Context, provider string) error {
	redirectURI := s.oauthRedirectURI(c, provider)
	result, err := s.oauthsvc.RedirectURL(c.Request.Context(), provider, authoauth.RedirectRequest{
		RedirectURI: redirectURI,
	})
	if err != nil {
		return err
	}

	s.setOAuthCookie(c, oauthStateCookieName(), result.State, oauthStateTTL)
	if strings.TrimSpace(result.CodeVerifier) != "" {
		s.setOAuthCookie(c, oauthVerifierCookieName(), result.CodeVerifier, oauthStateTTL)
	}

	redirectTarget := sanitizeRedirectPath(firstNonEmpty(c.Query("redirectTo"), c.Query("redirect_to")))
	if redirectTarget != "" {
		s.setOAuthCookie(c, oauthRedirectCookieName(), redirectTarget, oauthStateTTL)
	}

	c.Redirect(http.StatusFound, result.URL)
	return nil
}

func (s *Server) handleOAuthCallback(c *gin.Context, provider string, code string) error {
	state := strings.TrimSpace(c.Query("state"))
	storedState, err := c.Cookie(oauthStateCookieName())
	if err != nil || storedState == "" || state == "" || !subtleConstantEquals(state, storedState) {
		s.clearOAuthCookies(c)
		return ErrUnauthorized
	}

	verifier, _ := c.Cookie(oauthVerifierCookieName())
	redirectTarget, _ := c.Cookie(oauthRedirectCookieName())
	s.clearOAuthCookies(c)

	redirectURI := s.oauthRedirectURI(c, provider)
	result, err := s.oauthsvc.Login(c.Request.Context(), provider, authoauth.LoginRequest{
		Code:         code,
		RedirectURI:  redirectURI,
		CodeVerifier: verifier,
	})
	if err != nil {
		return err
	}

	allowSignUp := result.AllowSignUp
	if !s.cfg.IsCloud() && s.cfg.Bootstrap.AllowSignUp {
		allowSignUp = true
	}
	user, err := s.findOrCreateOAuthUser(c.Request.Context(), result.ProviderName, oauthIdentity{
		ExternalID:  result.Identity.ExternalID,
		Email:       result.Identity.Email,
		DisplayName: result.Identity.DisplayName,
	}, allowSignUp)
	if err != nil {
		return err
	}
	if err := s.ensureAutoOrgMembership(c.Request.Context(), user.ID); err != nil {
		return err
	}

	loginResult, err := s.createLoginResult(c, user, result.ProviderName)
	if err != nil {
		return err
	}
	s.sessions.Set(c, loginResult.RawToken, loginResult.ExpiresAt)
	s.enrichSessionMetadata(c, loginResult)

	redirectTarget = sanitizeRedirectPath(redirectTarget)
	if redirectTarget == "" {
		redirectTarget = "/"
	}
	c.Redirect(http.StatusFound, redirectTarget)
	return nil
}

func (s *Server) findOrCreateOAuthUser(ctx context.Context, provider string, identity oauthIdentity, allowSignUp bool) (*authdomain.User, error) {
	var user authdomain.User
	err := s.db.WithContext(ctx).Where("external_id = ?", identity.ExternalID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if !allowSignUp {
			return nil, ErrUnauthorized
		}
		now := time.Now().UTC()
		user = authdomain.User{
			ID:                  s.genID.Generate(),
			ExternalID:          identity.ExternalID,
			Provider:            provider,
			DisplayName:         identity.DisplayName,
			Email:               identity.Email,
			Metadata:            datatypes.JSONMap{},
			IsDefault:           false,
			LastPasswordChanged: nil,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
			return nil, err
		}
		return &user, nil
	}
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}
	if identity.Email != "" && identity.Email != user.Email {
		updates["email"] = identity.Email
		user.Email = identity.Email
	}
	if identity.DisplayName != "" && identity.DisplayName != user.DisplayName {
		updates["display_name"] = identity.DisplayName
		user.DisplayName = identity.DisplayName
	}
	if len(updates) > 0 {
		updates["updated_at"] = time.Now().UTC()
		if err := s.db.WithContext(ctx).Model(&authdomain.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return &user, nil
}

func (s *Server) createLoginResult(c *gin.Context, user *authdomain.User, authProvider string) (*authdomain.LoginResult, error) {
	rawToken, err := newRandomToken(sessionTokenSize)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	session := &authdomain.Session{
		ID:               s.genID.Generate(),
		UserID:           user.ID,
		SessionTokenHash: hashToken(rawToken),
		UserAgent:        strings.TrimSpace(c.Request.UserAgent()),
		IPAddress:        strings.TrimSpace(c.ClientIP()),
		OrgIDs:           []int64{},
		ExpiresAt:        now.Add(oauthSessionTTL),
		CreatedAt:        now,
		LastSeenAt:       now,
	}

	if err := s.db.WithContext(c.Request.Context()).Create(session).Error; err != nil {
		return nil, err
	}

	mustChangePassword := false
	passwordState := "rotated"
	if user.Provider == "local" && (user.IsDefault || user.LastPasswordChanged == nil) {
		passwordState = "default"
		mustChangePassword = true
	}

	return &authdomain.LoginResult{
		Session: &authdomain.SessionView{
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
				"auth_provider":         authProvider,
			},
		},
		RawToken:  rawToken,
		ExpiresAt: session.ExpiresAt,
		SessionID: session.ID,
	}, nil
}

func (s *Server) ensureOrgMembership(ctx context.Context, userID snowflake.ID, orgIDs []snowflake.ID, role string) error {
	if len(orgIDs) == 0 {
		return nil
	}

	for _, orgID := range orgIDs {
		var org orgdomain.Organization
		if err := s.db.WithContext(ctx).First(&org, "id = ?", orgID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}

		var count int64
		if err := s.db.WithContext(ctx).
			Model(&orgdomain.OrganizationMember{}).
			Where("org_id = ? AND user_id = ?", orgID, userID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}

		member := orgdomain.OrganizationMember{
			ID:        s.genID.Generate(),
			OrgID:     orgID,
			UserID:    userID,
			Role:      role,
			CreatedAt: time.Now().UTC(),
		}
		if err := s.db.WithContext(ctx).Create(&member).Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) ensureAutoOrgMembership(ctx context.Context, userID snowflake.ID) error {
	if s.cfg.IsCloud() {
		return nil
	}
	cfg := s.cfg.Bootstrap
	if !cfg.AllowSignUp || !cfg.AllowAssignOrg {
		return nil
	}
	orgIDRaw := strings.TrimSpace(cfg.AutoAssignOrgID)
	role := strings.ToUpper(strings.TrimSpace(cfg.AutoAssignOrgRole))
	if orgIDRaw == "" || role == "" {
		return nil
	}
	if !roleAllowed(cfg.AllowAssignUserRole, role) {
		return nil
	}

	orgID, err := snowflake.ParseString(orgIDRaw)
	if err != nil {
		return err
	}
	return s.ensureOrgMembership(ctx, userID, []snowflake.ID{orgID}, role)
}

func roleAllowed(allowedRaw string, role string) bool {
	allowedRaw = strings.TrimSpace(allowedRaw)
	if allowedRaw == "" || role == "" {
		return false
	}
	parts := strings.FieldsFunc(allowedRaw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	for _, part := range parts {
		if strings.ToUpper(strings.TrimSpace(part)) == role {
			return true
		}
	}
	return false
}

func (s *Server) oauthRedirectURI(c *gin.Context, provider string) string {
	base := requestBaseURL(c)
	return fmt.Sprintf("%s/login/%s", base, url.PathEscape(provider))
}

func requestBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if proto := firstHeaderValue(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		scheme = strings.ToLower(proto)
	}
	host := c.Request.Host
	if forwarded := firstHeaderValue(c.GetHeader("X-Forwarded-Host")); forwarded != "" {
		host = forwarded
	}
	return scheme + "://" + host
}

func handleOAuthError(c *gin.Context, provider string, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, authoauth.ErrProviderNotFound),
		errors.Is(err, authoauth.ErrProviderNotSupported):
		AbortWithError(c, ErrNotFound)
	default:
		log.Printf("oauth login failed provider=%s err=%v", provider, err)
		redirectToOAuthError(c)
	}
}

func logOAuthError(c *gin.Context, provider string) {
	errCode := strings.TrimSpace(c.Query("error"))
	errDesc := strings.TrimSpace(c.Query("error_description"))
	errURI := strings.TrimSpace(c.Query("error_uri"))
	log.Printf("oauth login error provider=%s error=%s description=%s uri=%s", provider, errCode, errDesc, errURI)
}

func redirectToOAuthError(c *gin.Context) {
	c.Redirect(http.StatusFound, oauthErrorRedirectTo)
}

func firstHeaderValue(value string) string {
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, ","); idx >= 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func sanitizeRedirectPath(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "//") || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return ""
	}
	if !strings.HasPrefix(value, "/") {
		return ""
	}
	return value
}

func (s *Server) setOAuthCookie(c *gin.Context, name string, value string, ttl time.Duration) {
	maxAge := int(ttl.Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, value, maxAge, "/", "", s.cfg.AuthCookieSecure, true)
}

func (s *Server) clearOAuthCookies(c *gin.Context) {
	s.clearCookie(c, oauthStateCookieName())
	s.clearCookie(c, oauthVerifierCookieName())
	s.clearCookie(c, oauthRedirectCookieName())
}

func (s *Server) clearCookie(c *gin.Context, name string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, "", -1, "/", "", s.cfg.AuthCookieSecure, true)
}

func oauthStateCookieName() string {
	return oauthStateCookie
}

func oauthVerifierCookieName() string {
	return oauthVerifierCookie
}

func oauthRedirectCookieName() string {
	return oauthRedirectCookie
}

func newRandomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func subtleConstantEquals(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}
