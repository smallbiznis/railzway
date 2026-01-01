package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	templatedomain "github.com/smallbiznis/valora/internal/invoicetemplate/domain"
)

func (s *Server) CreateInvoiceTemplate(c *gin.Context) {
	var req templatedomain.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.invoiceTemplateSvc.Create(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (s *Server) ListInvoiceTemplates(c *gin.Context) {
	var req templatedomain.ListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.invoiceTemplateSvc.List(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (s *Server) GetInvoiceTemplateByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))

	resp, err := s.invoiceTemplateSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (s *Server) UpdateInvoiceTemplate(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))

	var req templatedomain.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}
	req.ID = id

	resp, err := s.invoiceTemplateSvc.Update(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (s *Server) SetDefaultInvoiceTemplate(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))

	resp, err := s.invoiceTemplateSvc.SetDefault(c.Request.Context(), id)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}
