package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	customerdomain "github.com/smallbiznis/valora/internal/customer/domain"
	"github.com/smallbiznis/valora/pkg/db/pagination"
)

type createCustomerRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *Server) CreateCustomer(c *gin.Context) {
	var req createCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.customerSvc.Create(c.Request.Context(), customerdomain.CreateCustomerRequest{
		Name:  strings.TrimSpace(req.Name),
		Email: strings.TrimSpace(req.Email),
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	if s.auditSvc != nil {
		targetID := resp.ID.String()
		_ = s.auditSvc.AuditLog(c.Request.Context(), nil, "", nil, "customer.create", "customer", &targetID, map[string]any{
			"customer_id": resp.ID.String(),
			"name":        resp.Name,
			"email":       resp.Email,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (s *Server) ListCustomers(c *gin.Context) {
	var query struct {
		pagination.Pagination
		Name        string `form:"name"`
		Email       string `form:"email"`
		Currency    string `form:"currency"`
		CreatedFrom string `form:"created_from"`
		CreatedTo   string `form:"created_to"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	createdFrom, err := parseOptionalTime(query.CreatedFrom, false)
	if err != nil {
		AbortWithError(c, newValidationError("created_from", "invalid_created_from", "invalid created_from"))
		return
	}

	createdTo, err := parseOptionalTime(query.CreatedTo, true)
	if err != nil {
		AbortWithError(c, newValidationError("created_to", "invalid_created_to", "invalid created_to"))
		return
	}

	resp, err := s.customerSvc.List(c.Request.Context(), customerdomain.ListCustomerRequest{
		PageToken:   query.PageToken,
		PageSize:    int32(query.PageSize),
		Name:        strings.TrimSpace(query.Name),
		Email:       strings.TrimSpace(query.Email),
		Currency:    strings.TrimSpace(query.Currency),
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (s *Server) GetCustomerByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	resp, err := s.customerSvc.GetByID(c.Request.Context(), customerdomain.GetCustomerRequest{
		ID: id,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func isCustomerValidationError(err error) bool {
	switch err {
	case customerdomain.ErrInvalidOrganization,
		customerdomain.ErrInvalidName,
		customerdomain.ErrInvalidEmail,
		customerdomain.ErrInvalidID:
		return true
	default:
		return false
	}
}
