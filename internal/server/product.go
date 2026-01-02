package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	productdomain "github.com/smallbiznis/valora/internal/product/domain"
)

type createProductRequest struct {
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Description *string        `json:"description"`
	Active      *bool          `json:"active"`
	Metadata    map[string]any `json:"metadata"`
}

func (s *Server) CreateProduct(c *gin.Context) {
	var req createProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.productSvc.Create(c.Request.Context(), productdomain.CreateRequest{
		Code:        strings.TrimSpace(req.Code),
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		Active:      req.Active,
		Metadata:    req.Metadata,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	if s.auditSvc != nil {
		targetID := resp.ID
		_ = s.auditSvc.AuditLog(c.Request.Context(), nil, "", nil, "product.create", "product", &targetID, map[string]any{
			"product_id": resp.ID,
			"code":       resp.Code,
			"name":       resp.Name,
			"active":     resp.Active,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (s *Server) ListProducts(c *gin.Context) {
	var query struct {
		Name    string `form:"name"`
		Active  string `form:"active"`
		SortBy  string `form:"sort_by"`
		OrderBy string `form:"order_by"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	active, err := parseOptionalBool(query.Active)
	if err != nil {
		AbortWithError(c, newValidationError("active", "invalid_active", "invalid active"))
		return
	}

	resp, err := s.productSvc.List(c.Request.Context(), productdomain.ListRequest{
		Name:    strings.TrimSpace(query.Name),
		Active:  active,
		SortBy:  strings.TrimSpace(query.SortBy),
		OrderBy: strings.TrimSpace(query.OrderBy),
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (s *Server) GetProductByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	resp, err := s.productSvc.Get(c.Request.Context(), id)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func isProductValidationError(err error) bool {
	switch err {
	case productdomain.ErrInvalidOrganization,
		productdomain.ErrInvalidCode,
		productdomain.ErrInvalidName,
		productdomain.ErrInvalidID:
		return true
	default:
		return false
	}
}
