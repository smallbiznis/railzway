package oauth2provider

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	authdomain "github.com/smallbiznis/valora/internal/auth/domain"
	"github.com/smallbiznis/valora/internal/auth/session"
	"github.com/smallbiznis/valora/internal/config"
	orgdomain "github.com/smallbiznis/valora/internal/organization/domain"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Handler handles OAuth2 provider endpoints.
type Handler struct {
	svc      *Service
	authsvc  authdomain.Service
	sessions *session.Manager
	log      *zap.Logger
	cfg      config.Config
	db       *gorm.DB
	genID    *snowflake.Node
}

func NewHandler(svc *Service, authsvc authdomain.Service, sessions *session.Manager, log *zap.Logger, cfg config.Config, db *gorm.DB, genID *snowflake.Node) *Handler {
	return &Handler{
		svc:      svc,
		authsvc:  authsvc,
		sessions: sessions,
		log:      log.Named("auth.oauth2.handler"),
		cfg:      cfg,
		db:       db,
		genID:    genID,
	}
}

func RegisterRoutes(r *gin.Engine, h *Handler) {
	group := r.Group("/internal/auth/oauth2")
	group.GET("/authorize", h.Authorize)
	group.POST("/token", h.Token)
	group.GET("/userinfo", h.UserInfo)
}

func (h *Handler) Authorize(c *gin.Context) {
	responseType := strings.TrimSpace(c.Query("response_type"))
	clientID := strings.TrimSpace(c.Query("client_id"))
	redirectURI := strings.TrimSpace(c.Query("redirect_uri"))
	scope := strings.TrimSpace(c.Query("scope"))
	state := strings.TrimSpace(c.Query("state"))
	codeChallenge := strings.TrimSpace(c.Query("code_challenge"))
	codeChallengeMethod := strings.TrimSpace(c.Query("code_challenge_method"))

	if responseType != "code" {
		writeOAuthError(c, http.StatusBadRequest, "unsupported_response_type")
		return
	}

	token, ok := h.sessions.ReadToken(c)
	if !ok {
		writeOAuthError(c, http.StatusUnauthorized, "login_required")
		return
	}

	sessionData, err := h.authsvc.Authenticate(c.Request.Context(), token)
	if err != nil {
		writeOAuthError(c, http.StatusUnauthorized, "login_required")
		return
	}

	result, err := h.svc.Authorize(c.Request.Context(), AuthorizeRequest{
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scopes:              parseScopeList(scope),
		State:               state,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		UserID:              sessionData.UserID,
	})
	if err != nil {
		writeOAuthError(c, mapAuthorizeErrorStatus(err), mapAuthorizeErrorCode(err))
		return
	}

	redirectURL, err := appendAuthCode(result.RedirectURI, result.Code, result.State)
	if err != nil {
		writeOAuthError(c, http.StatusBadRequest, "invalid_request")
		return
	}

	h.log.Info("oauth2 authorize issued code",
		zap.String("request_id", requestID(c)),
		zap.String("user_id", sessionData.UserID.String()),
		zap.String("client_id", clientID),
	)

	c.Redirect(http.StatusFound, redirectURL)
}

func (h *Handler) Token(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		writeOAuthError(c, http.StatusBadRequest, "invalid_request")
		return
	}

	grantType := strings.TrimSpace(c.PostForm("grant_type"))
	code := strings.TrimSpace(c.PostForm("code"))
	redirectURI := strings.TrimSpace(c.PostForm("redirect_uri"))
	clientID := strings.TrimSpace(c.PostForm("client_id"))
	clientSecret := strings.TrimSpace(c.PostForm("client_secret"))
	codeVerifier := strings.TrimSpace(c.PostForm("code_verifier"))

	basicID, basicSecret := parseBasicAuth(c)
	if basicID != "" {
		if clientID != "" && clientID != basicID {
			writeOAuthError(c, http.StatusUnauthorized, "invalid_client")
			return
		}
		clientID = basicID
		clientSecret = basicSecret
	}

	if grantType != "authorization_code" {
		writeOAuthError(c, http.StatusBadRequest, "unsupported_grant_type")
		return
	}

	resp, err := h.svc.ExchangeToken(c.Request.Context(), TokenRequest{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Code:         code,
		RedirectURI:  redirectURI,
		CodeVerifier: codeVerifier,
	})
	if err != nil {
		writeOAuthError(c, mapTokenErrorStatus(err), mapTokenErrorCode(err))
		return
	}
	if err := h.ensureAutoOrgMembership(c.Request.Context(), resp.UserID); err != nil {
		writeOAuthError(c, http.StatusInternalServerError, "server_error")
		return
	}

	h.log.Info("oauth2 token issued",
		zap.String("request_id", requestID(c)),
		zap.String("client_id", clientID),
	)

	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(http.StatusOK, gin.H{
		"access_token": resp.AccessToken,
		"token_type":   resp.TokenType,
		"expires_in":   resp.ExpiresIn,
	})
}

func (h *Handler) UserInfo(c *gin.Context) {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader == "" {
		c.Status(http.StatusUnauthorized)
		return
	}

	parts := strings.Fields(authHeader)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		c.Status(http.StatusUnauthorized)
		return
	}

	claims, err := h.svc.UserInfo(c.Request.Context(), parts[1])
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	c.JSON(http.StatusOK, claims)
}

func parseBasicAuth(c *gin.Context) (string, string) {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if header == "" {
		return "", ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Basic") {
		return "", ""
	}
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ""
	}
	creds := strings.SplitN(string(decoded), ":", 2)
	if len(creds) != 2 {
		return "", ""
	}
	return creds[0], creds[1]
}

func appendAuthCode(rawRedirectURI, code, state string) (string, error) {
	redirectURL, err := url.Parse(rawRedirectURI)
	if err != nil {
		return "", err
	}
	query := redirectURL.Query()
	query.Set("code", code)
	if state != "" {
		query.Set("state", state)
	}
	redirectURL.RawQuery = query.Encode()
	return redirectURL.String(), nil
}

func (h *Handler) ensureAutoOrgMembership(ctx context.Context, userID snowflake.ID) error {
	if !h.cfg.IsCloud() || userID == 0 {
		return nil
	}
	cfg := h.cfg.Bootstrap
	if !cfg.AllowAssignOrg {
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

	var org orgdomain.Organization
	if err := h.db.WithContext(ctx).First(&org, "id = ?", orgID).Error; err != nil {
		return err
	}

	var count int64
	if err := h.db.WithContext(ctx).
		Model(&orgdomain.OrganizationMember{}).
		Where("org_id = ? AND user_id = ?", orgID, userID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	member := orgdomain.OrganizationMember{
		ID:        h.genID.Generate(),
		OrgID:     orgID,
		UserID:    userID,
		Role:      role,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.db.WithContext(ctx).Create(&member).Error; err != nil {
		return err
	}
	return nil
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

func writeOAuthError(c *gin.Context, status int, code string) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(status, gin.H{"error": code})
}

func mapAuthorizeErrorStatus(err error) int {
	switch err {
	case ErrInvalidClient:
		return http.StatusUnauthorized
	case ErrInvalidRequest, ErrInvalidRedirectURI, ErrInvalidScope:
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}

func mapAuthorizeErrorCode(err error) string {
	switch err {
	case ErrInvalidClient:
		return "invalid_client"
	case ErrInvalidRedirectURI:
		return "invalid_redirect_uri"
	case ErrInvalidScope:
		return "invalid_scope"
	case ErrInvalidRequest:
		return "invalid_request"
	default:
		return "invalid_request"
	}
}

func mapTokenErrorStatus(err error) int {
	switch err {
	case ErrInvalidClient:
		return http.StatusUnauthorized
	case ErrInvalidRequest, ErrInvalidRedirectURI, ErrInvalidCode, ErrCodeUsed, ErrCodeExpired, ErrPKCEMismatch:
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}

func mapTokenErrorCode(err error) string {
	switch err {
	case ErrInvalidClient:
		return "invalid_client"
	case ErrInvalidRedirectURI:
		return "invalid_grant"
	case ErrCodeUsed, ErrCodeExpired, ErrInvalidCode:
		return "invalid_grant"
	case ErrPKCEMismatch:
		return "invalid_grant"
	case ErrInvalidRequest:
		return "invalid_request"
	default:
		return "invalid_request"
	}
}

func requestID(c *gin.Context) string {
	if v := strings.TrimSpace(c.GetHeader("X-Request-Id")); v != "" {
		return v
	}
	if v := strings.TrimSpace(c.GetHeader("X-Request-ID")); v != "" {
		return v
	}
	if v := strings.TrimSpace(c.GetString("request_id")); v != "" {
		return v
	}
	return ""
}
