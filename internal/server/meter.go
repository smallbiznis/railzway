package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	meterdomain "github.com/smallbiznis/railzway/internal/meter/domain"
)

type createMeterRequest struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	AggregationType string  `json:"aggregation_type"`
	Unit            string  `json:"unit"`
	Description     *string `json:"description"`
	Active          *bool   `json:"active"`
}

type updateMeterRequest struct {
	Name            *string `json:"name,omitempty"`
	AggregationType *string `json:"aggregation_type,omitempty"`
	Unit            *string `json:"unit,omitempty"`
	Active          *bool   `json:"active,omitempty"`
}


// @Summary      Create Meter
// @Description  Create a new meter
// @Tags         meters
// @Accept       json
// @Produce      json
// @Param        request body createMeterRequest true "Create Meter Request"
// @Success      200  {object}  meterdomain.Meter
// @Router       /meters [post]
func (s *Server) CreateMeter(c *gin.Context) {
	var req createMeterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.meterSvc.Create(c.Request.Context(), meterdomain.CreateRequest{
		Code:        strings.TrimSpace(req.Code),
		Name:        strings.TrimSpace(req.Name),
		Aggregation: strings.TrimSpace(req.AggregationType),
		Unit:        strings.TrimSpace(req.Unit),
		Active:      req.Active,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// @Summary      List Meters
// @Description  List meters
// @Tags         meters
// @Accept       json
// @Produce      json
// @Param        name     query     string  false  "Name"
// @Param        code     query     string  false  "Code"
// @Param        active   query     bool    false  "Active"
// @Param        sort_by  query     string  false  "Sort By"
// @Param        order_by query     string  false  "Order By"
// @Success      200  {object}  []meterdomain.Meter
// @Router       /meters [get]
func (s *Server) ListMeters(c *gin.Context) {
	var query struct {
		Name    string `form:"name"`
		Code    string `form:"code"`
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

	resp, err := s.meterSvc.List(c.Request.Context(), meterdomain.ListRequest{
		Name:    strings.TrimSpace(query.Name),
		Code:    strings.TrimSpace(query.Code),
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

// @Summary      Get Meter
// @Description  Get meter by ID
// @Tags         meters
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Meter ID"
// @Success      200  {object}  meterdomain.Meter
// @Router       /meters/{id} [get]
func (s *Server) GetMeterByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	resp, err := s.meterSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// @Summary      Update Meter
// @Description  Update meter
// @Tags         meters
// @Accept       json
// @Produce      json
// @Param        id       path      string             true  "Meter ID"
// @Param        request  body      updateMeterRequest true  "Update Meter Request"
// @Success      200  {object}  meterdomain.Meter
// @Router       /meters/{id} [patch]
func (s *Server) UpdateMeter(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))

	var req updateMeterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.meterSvc.Update(c.Request.Context(), meterdomain.UpdateRequest{
		ID:          id,
		Name:        trimStringPtr(req.Name),
		Aggregation: trimStringPtr(req.AggregationType),
		Unit:        trimStringPtr(req.Unit),
		Active:      req.Active,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// @Summary      Delete Meter
// @Description  Delete meter
// @Tags         meters
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Meter ID"
// @Success      204
// @Router       /meters/{id} [delete]
func (s *Server) DeleteMeter(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))

	if err := s.meterSvc.Delete(c.Request.Context(), id); err != nil {
		AbortWithError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func isMeterValidationError(err error) bool {
	switch err {
	case meterdomain.ErrInvalidOrganization,
		meterdomain.ErrInvalidCode,
		meterdomain.ErrInvalidName,
		meterdomain.ErrInvalidAggregation,
		meterdomain.ErrInvalidUnit,
		meterdomain.ErrInvalidID:
		return true
	default:
		return false
	}
}


func trimStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}
