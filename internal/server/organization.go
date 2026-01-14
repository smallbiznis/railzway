package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	organizationdomain "github.com/smallbiznis/railzway/internal/organization/domain"
)

type createOrganizationRequest struct {
	Name            string `json:"name"`
	CountryCode     string `json:"country_code"`
	TimezoneName    string `json:"timezone_name"`
	DefaultCurrency string `json:"default_currency"`
}

func (s *Server) CreateOrganization(c *gin.Context) {
	userID, ok := s.userIDFromSession(c)
	if !ok {
		AbortWithError(c, ErrUnauthorized)
		return
	}

	var req createOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("ShouldBindJSON: %v\n", err)
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.organizationSvc.Create(c.Request.Context(), userID, organizationdomain.CreateOrganizationRequest{
		Name:         strings.TrimSpace(req.Name),
		CountryCode:  strings.TrimSpace(req.CountryCode),
		TimezoneName: strings.TrimSpace(req.TimezoneName),
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) UpdateOrganization(c *gin.Context) {
	orgID := c.Param("id")
	if orgID == "" {
		AbortWithError(c, newValidationError("id", "missing_id", "org id is required"))
		return
	}

	userID, ok := s.userIDFromSession(c)
	if !ok {
		AbortWithError(c, ErrUnauthorized)
		return
	}

	var req struct {
		Name *string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.organizationSvc.Update(c.Request.Context(), userID, orgID, organizationdomain.UpdateOrganizationRequest{
		Name: req.Name,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) ListOrganizations(c *gin.Context) {
	userID, ok := s.userIDFromSession(c)
	if !ok {
		AbortWithError(c, ErrUnauthorized)
		return
	}

	items, err := s.organizationSvc.ListOrganizationsByUser(c.Request.Context(), userID)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (s *Server) userIDFromSession(c *gin.Context) (snowflake.ID, bool) {
	value, ok := c.Get(contextUserIDKey)
	if !ok {
		return 0, false
	}
	raw, ok := value.(string)
	if !ok {
		return 0, false
	}
	userID, err := snowflake.ParseString(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	return userID, true
}

func isOrganizationValidationError(err error) bool {
	switch err {
	case organizationdomain.ErrInvalidName,
		organizationdomain.ErrInvalidCountry,
		organizationdomain.ErrInvalidTimezone,
		organizationdomain.ErrInvalidCurrency,
		organizationdomain.ErrInvalidUser,
		organizationdomain.ErrInvalidEmail,
		organizationdomain.ErrInvalidRole:
		return true
	default:
		return false
	}
}

func (s *Server) ListOrganizationMembers(c *gin.Context) {
	orgID := c.Param("id")
	if orgID == "" {
		AbortWithError(c, newValidationError("id", "missing_id", "org id is required"))
		return
	}

	// Permission check is handled by middleware but we might want to ensure user is member of THIS org
	// For now, assuming middleware covers generic role access or we rely on Service to check membership if needed.
	// But Service ListMembers implements simple query.
	// Ideally we check if session user is member of orgID.
	userID, ok := s.userIDFromSession(c)
	if !ok {
		AbortWithError(c, ErrUnauthorized)
		return
	}

	parsedOrgID, err := snowflake.ParseString(orgID)
	if err != nil {
		AbortWithError(c, newValidationError("id", "invalid_id", "invalid organization id"))
		return
	}

	isMember, err := s.organizationSvc.IsMember(c.Request.Context(), parsedOrgID, userID)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if !isMember { // && role check? s.RequireRole usually handles it, but here we manually check membership.
		AbortWithError(c, ErrForbidden)
		return
	}

	resp, err := s.organizationSvc.ListMembers(c.Request.Context(), orgID)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}
