package domain

import (
	"context"
	"errors"
	"time"
)

type Granularity string

const (
	GranularityDay   Granularity = "day"
	GranularityMonth Granularity = "month"
)

type OverviewRequest struct {
	Start       time.Time
	End         time.Time
	Granularity Granularity
	Compare     bool
}

type SeriesPoint struct {
	Period string `json:"period"`
	Value  int64  `json:"value"`
}

type MRRResponse struct {
	Currency      string        `json:"currency"`
	Current       *int64        `json:"current,omitempty"`
	Previous      *int64        `json:"previous,omitempty"`
	GrowthAmount  *int64        `json:"growth_amount,omitempty"`
	GrowthRate    *float64      `json:"growth_rate,omitempty"`
	Series        []SeriesPoint `json:"series"`
	CompareSeries []SeriesPoint `json:"compare_series,omitempty"`
	HasData       bool          `json:"has_data"`
}

type MRRMovementResponse struct {
	Currency      string `json:"currency"`
	NewMRR        int64  `json:"new_mrr"`
	ExpansionMRR  int64  `json:"expansion_mrr"`
	ContractionMRR int64 `json:"contraction_mrr"`
	ChurnedMRR    int64  `json:"churned_mrr"`
	NetMRRChange  int64  `json:"net_mrr_change"`
	HasData       bool   `json:"has_data"`
}

type RevenueResponse struct {
	Currency      string        `json:"currency"`
	Total         *int64        `json:"total,omitempty"`
	Previous      *int64        `json:"previous,omitempty"`
	GrowthAmount  *int64        `json:"growth_amount,omitempty"`
	GrowthRate    *float64      `json:"growth_rate,omitempty"`
	Series        []SeriesPoint `json:"series"`
	CompareSeries []SeriesPoint `json:"compare_series,omitempty"`
	HasData       bool          `json:"has_data"`
}

type OutstandingBalanceResponse struct {
	Currency    string `json:"currency"`
	Outstanding int64  `json:"outstanding"`
	Overdue     int64  `json:"overdue"`
	HasData     bool   `json:"has_data"`
}

type CollectionRateResponse struct {
	Currency        string   `json:"currency"`
	CollectionRate *float64 `json:"collection_rate,omitempty"`
	CollectedAmount int64    `json:"collected_amount"`
	InvoicedAmount  int64    `json:"invoiced_amount"`
	HasData         bool     `json:"has_data"`
}

type SubscribersResponse struct {
	Current       *int64        `json:"current,omitempty"`
	Previous      *int64        `json:"previous,omitempty"`
	GrowthAmount  *int64        `json:"growth_amount,omitempty"`
	GrowthRate    *float64      `json:"growth_rate,omitempty"`
	ChurnRate     *float64      `json:"churn_rate,omitempty"`
	Series        []SeriesPoint `json:"series"`
	CompareSeries []SeriesPoint `json:"compare_series,omitempty"`
	HasData       bool          `json:"has_data"`
}

// Service exposes revenue-first billing overview data.
type Service interface {
	GetMRR(ctx context.Context, req OverviewRequest) (MRRResponse, error)
	GetMRRMovement(ctx context.Context, req OverviewRequest) (MRRMovementResponse, error)
	GetRevenue(ctx context.Context, req OverviewRequest) (RevenueResponse, error)
	GetOutstandingBalance(ctx context.Context, req OverviewRequest) (OutstandingBalanceResponse, error)
	GetCollectionRate(ctx context.Context, req OverviewRequest) (CollectionRateResponse, error)
	GetSubscribers(ctx context.Context, req OverviewRequest) (SubscribersResponse, error)
}

var (
	ErrInvalidOrganization = errors.New("invalid_organization")
)
