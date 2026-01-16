package server

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	billingoverviewdomain "github.com/smallbiznis/railzway/internal/billingoverview/domain"
)

func (s *Server) GetBillingOverviewMRR(c *gin.Context) {
	if s.billingOverviewSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	req, err := parseBillingOverviewRequest(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOverviewSvc.GetMRR(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	if c.Query("format") == "csv" {
		writeCSV(c, "mrr.csv", resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetBillingOverviewRevenue(c *gin.Context) {
	if s.billingOverviewSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	req, err := parseBillingOverviewRequest(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOverviewSvc.GetRevenue(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	if c.Query("format") == "csv" {
		writeCSV(c, "revenue.csv", resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetBillingOverviewMRRMovement(c *gin.Context) {
	if s.billingOverviewSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	req, err := parseBillingOverviewRequest(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOverviewSvc.GetMRRMovement(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	if c.Query("format") == "csv" {
		writeCSV(c, "mrr_movement.csv", resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetBillingOverviewOutstandingBalance(c *gin.Context) {
	if s.billingOverviewSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	req, err := parseBillingOverviewRequest(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOverviewSvc.GetOutstandingBalance(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	if c.Query("format") == "csv" {
		writeCSV(c, "outstanding.csv", resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetBillingOverviewCollectionRate(c *gin.Context) {
	if s.billingOverviewSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	req, err := parseBillingOverviewRequest(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOverviewSvc.GetCollectionRate(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	if c.Query("format") == "csv" {
		writeCSV(c, "collection_rate.csv", resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetBillingOverviewSubscribers(c *gin.Context) {
	if s.billingOverviewSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	req, err := parseBillingOverviewRequest(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOverviewSvc.GetSubscribers(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	if c.Query("format") == "csv" {
		writeCSV(c, "subscribers.csv", resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func parseBillingOverviewRequest(c *gin.Context) (billingoverviewdomain.OverviewRequest, error) {
	startValue, err := parseOptionalTime(c.Query("start"), false)
	if err != nil {
		return billingoverviewdomain.OverviewRequest{}, newValidationError("start", "invalid_time", "invalid start time")
	}
	endValue, err := parseOptionalTime(c.Query("end"), true)
	if err != nil {
		return billingoverviewdomain.OverviewRequest{}, newValidationError("end", "invalid_time", "invalid end time")
	}

	granularityValue := strings.ToLower(strings.TrimSpace(c.Query("granularity")))
	if granularityValue == "" {
		granularityValue = string(billingoverviewdomain.GranularityDay)
	}

	var granularity billingoverviewdomain.Granularity
	switch granularityValue {
	case string(billingoverviewdomain.GranularityDay):
		granularity = billingoverviewdomain.GranularityDay
	case string(billingoverviewdomain.GranularityMonth):
		granularity = billingoverviewdomain.GranularityMonth
	default:
		return billingoverviewdomain.OverviewRequest{}, newValidationError("granularity", "invalid_granularity", "invalid granularity")
	}

	compareValue, err := parseOptionalBool(c.Query("compare"))
	if err != nil {
		return billingoverviewdomain.OverviewRequest{}, newValidationError("compare", "invalid_compare", "invalid compare flag")
	}

	now := time.Now().UTC()
	start := now.AddDate(0, 0, -30)
	end := now
	if startValue != nil {
		start = startValue.UTC()
	}
	if endValue != nil {
		end = endValue.UTC()
	}
	if startValue == nil && endValue != nil {
		start = end.AddDate(0, 0, -30)
	}
	if start.After(end) {
		return billingoverviewdomain.OverviewRequest{}, newValidationError("range", "invalid_range", "start must be before end")
	}

	compare := false
	if compareValue != nil {
		compare = *compareValue
	}

	return billingoverviewdomain.OverviewRequest{
		Start:       start,
		End:         end,
		Granularity: granularity,
		Compare:     compare,
	}, nil
}

func writeCSV(c *gin.Context, filename string, data interface{}) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	switch v := data.(type) {
	case *billingoverviewdomain.RevenueResponse:
		_ = writer.Write([]string{"Period", "Revenue", "Previous Revenue"})
		for i, point := range v.Series {
			row := []string{point.Period, fmt.Sprintf("%d", point.Value)}
			if len(v.CompareSeries) > i {
				row = append(row, fmt.Sprintf("%d", v.CompareSeries[i].Value))
			} else {
				row = append(row, "")
			}
			_ = writer.Write(row)
		}
	case *billingoverviewdomain.MRRResponse:
		_ = writer.Write([]string{"Period", "MRR", "Previous MRR"})
		for i, point := range v.Series {
			row := []string{point.Period, fmt.Sprintf("%d", point.Value)}
			if len(v.CompareSeries) > i {
				row = append(row, fmt.Sprintf("%d", v.CompareSeries[i].Value))
			} else {
				row = append(row, "")
			}
			_ = writer.Write(row)
		}
	case *billingoverviewdomain.SubscribersResponse:
		_ = writer.Write([]string{"Period", "Subscribers", "Previous Subscribers"})
		for i, point := range v.Series {
			row := []string{point.Period, fmt.Sprintf("%d", point.Value)}
			if len(v.CompareSeries) > i {
				row = append(row, fmt.Sprintf("%d", v.CompareSeries[i].Value))
			} else {
				row = append(row, "")
			}
			_ = writer.Write(row)
		}
	case *billingoverviewdomain.MRRMovementResponse:
		_ = writer.Write([]string{"Metric", "Value"})
		_ = writer.Write([]string{"New MRR", fmt.Sprintf("%d", v.NewMRR)})
		_ = writer.Write([]string{"Expansion MRR", fmt.Sprintf("%d", v.ExpansionMRR)})
		_ = writer.Write([]string{"Contraction MRR", fmt.Sprintf("%d", v.ContractionMRR)})
		_ = writer.Write([]string{"Churned MRR", fmt.Sprintf("%d", v.ChurnedMRR)})
		_ = writer.Write([]string{"Net MRR Change", fmt.Sprintf("%d", v.NetMRRChange)})
	case *billingoverviewdomain.OutstandingBalanceResponse:
		_ = writer.Write([]string{"Metric", "Value"})
		_ = writer.Write([]string{"Outstanding", fmt.Sprintf("%d", v.Outstanding)})
		_ = writer.Write([]string{"Overdue", fmt.Sprintf("%d", v.Overdue)})
	case *billingoverviewdomain.CollectionRateResponse:
		_ = writer.Write([]string{"Metric", "Value"})
		if v.CollectionRate != nil {
			_ = writer.Write([]string{"Collection Rate", strconv.FormatFloat(*v.CollectionRate, 'f', 2, 64)})
		} else {
			_ = writer.Write([]string{"Collection Rate", "N/A"})
		}
		_ = writer.Write([]string{"Collected Amount", fmt.Sprintf("%d", v.CollectedAmount)})
		_ = writer.Write([]string{"Invoiced Amount", fmt.Sprintf("%d", v.InvoicedAmount)})
	default:
		// Fallback for unknown types or just empty CSV
	}
}
