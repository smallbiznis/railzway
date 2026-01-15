package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	invoicedomain "github.com/smallbiznis/railzway/internal/invoice/domain"
)

// @Summary      List Invoices
// @Description  List available invoices
// @Tags         invoices
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        status           query     string  false  "Status"
// @Param        invoice_number   query     string  false  "Invoice Number"
// @Param        customer_id      query     string  false  "Customer ID"
// @Param        created_from     query     string  false  "Created From"
// @Param        created_to       query     string  false  "Created To"
// @Param        due_from         query     string  false  "Due From"
// @Param        due_to           query     string  false  "Due To"
// @Param        finalized_from   query     string  false  "Finalized From"
// @Param        finalized_to     query     string  false  "Finalized To"
// @Param        total_min        query     int     false  "Total Min"
// @Param        total_max        query     int     false  "Total Max"
// @Param        page_token       query     string  false  "Page Token"
// @Param        page_size        query     int     false  "Page Size"
// @Success      200  {object}  []invoicedomain.Invoice
// @Router       /invoices [get]
func (s *Server) ListInvoices(c *gin.Context) {
	var query struct {
		Status        string `form:"status"`
		InvoiceNumber string `form:"invoice_number"`
		CustomerID    string `form:"customer_id"`
		CreatedFrom   string `form:"created_from"`
		CreatedTo     string `form:"created_to"`
		DueFrom       string `form:"due_from"`
		DueTo         string `form:"due_to"`
		FinalizedFrom string `form:"finalized_from"`
		FinalizedTo   string `form:"finalized_to"`
		TotalMin      string `form:"total_min"`
		TotalMax      string `form:"total_max"`

		PageToken string `form:"page_token"`
		PageSize  int    `form:"page_size"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	status, err := parseInvoiceStatus(query.Status)
	if err != nil {
		AbortWithError(c, newValidationError("status", "invalid_status", "invalid status"))
		return
	}

	customerID, err := parseOptionalSnowflakeID(query.CustomerID)
	if err != nil {
		AbortWithError(c, newValidationError("customer_id", "invalid_customer_id", "invalid customer_id"))
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

	dueFrom, err := parseOptionalTime(query.DueFrom, false)
	if err != nil {
		AbortWithError(c, newValidationError("due_from", "invalid_due_from", "invalid due_from"))
		return
	}

	dueTo, err := parseOptionalTime(query.DueTo, true)
	if err != nil {
		AbortWithError(c, newValidationError("due_to", "invalid_due_to", "invalid due_to"))
		return
	}

	finalizedFrom, err := parseOptionalTime(query.FinalizedFrom, false)
	if err != nil {
		AbortWithError(c, newValidationError("finalized_from", "invalid_finalized_from", "invalid finalized_from"))
		return
	}

	finalizedTo, err := parseOptionalTime(query.FinalizedTo, true)
	if err != nil {
		AbortWithError(c, newValidationError("finalized_to", "invalid_finalized_to", "invalid finalized_to"))
		return
	}

	totalMin, err := parseOptionalInt64(query.TotalMin)
	if err != nil {
		AbortWithError(c, newValidationError("total_min", "invalid_total_min", "invalid total_min"))
		return
	}

	totalMax, err := parseOptionalInt64(query.TotalMax)
	if err != nil {
		AbortWithError(c, newValidationError("total_max", "invalid_total_max", "invalid total_max"))
		return
	}

	resp, err := s.invoiceSvc.List(c.Request.Context(), invoicedomain.ListInvoiceRequest{
		Status:        status,
		InvoiceNumber: &query.InvoiceNumber,
		CustomerID:    customerID,
		CreatedFrom:   createdFrom,
		CreatedTo:     createdTo,
		DueFrom:       dueFrom,
		DueTo:         dueTo,
		FinalizedFrom: finalizedFrom,
		FinalizedTo:   finalizedTo,
		TotalMin:      totalMin,
		TotalMax:      totalMax,
		PageToken:     query.PageToken,
		PageSize:      query.PageSize,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      resp.Invoices,
		"page_info": resp.PageInfo,
	})
}

// @Summary      Get Invoice
// @Description  Get invoice by ID
// @Tags         invoices
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id   path      string  true  "Invoice ID"
// @Success      200  {object}  invoicedomain.Invoice
// @Router       /invoices/{id} [get]
func (s *Server) GetInvoiceByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if _, err := snowflake.ParseString(id); err != nil {
		AbortWithError(c, newValidationError("id", "invalid_id", "invalid id"))
		return
	}

	item, err := s.invoiceSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": item})
}

// @Summary      Render Invoice
// @Description  Render invoice PDF/HTML
// @Tags         invoices
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id   path      string  true  "Invoice ID"
// @Success      200  {object}  invoicedomain.InvoiceRender
// @Router       /invoices/{id}/render [get]
func (s *Server) RenderInvoice(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if _, err := snowflake.ParseString(id); err != nil {
		AbortWithError(c, newValidationError("id", "invalid_id", "invalid id"))
		return
	}

	resp, err := s.invoiceSvc.RenderInvoice(c.Request.Context(), id)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func parseInvoiceStatus(value string) (*invoicedomain.InvoiceStatus, error) {
	status := strings.TrimSpace(value)
	if status == "" {
		return nil, nil
	}

	status = strings.ToUpper(status)
	switch invoicedomain.InvoiceStatus(status) {
	case invoicedomain.InvoiceStatusDraft,
		invoicedomain.InvoiceStatusFinalized,
		invoicedomain.InvoiceStatusVoid:
		parsed := invoicedomain.InvoiceStatus(status)
		return &parsed, nil
	default:
		return nil, errors.New("invalid_status")
	}
}
