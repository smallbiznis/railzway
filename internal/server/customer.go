package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	customerdomain "github.com/smallbiznis/railzway/internal/customer/domain"
	"github.com/smallbiznis/railzway/pkg/db/pagination"
)

type createCustomerRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// @Summary      Create Customer
// @Description  Create a new customer
// @Tags         customers
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        request body createCustomerRequest true "Create Customer Request"
// @Success      200  {object}  customerdomain.Customer
// @Router       /customers [post]
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

// @Summary      List Customers
// @Description  List available customers
// @Tags         customers
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        name          query     string  false  "Name"
// @Param        email         query     string  false  "Email"
// @Param        currency      query     string  false  "Currency"
// @Param        created_from  query     string  false  "Created From"
// @Param        created_to    query     string  false  "Created To"
// @Param        page_token    query     string  false  "Page Token"
// @Param        page_size     query     int     false  "Page Size"
// @Success      200  {object}  []customerdomain.Customer
// @Router       /customers [get]
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

// @Summary      Get Customer
// @Description  Get customer by ID
// @Tags         customers
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id   path      string  true  "Customer ID"
// @Success      200  {object}  customerdomain.Customer
// @Router       /customers/{id} [get]
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
