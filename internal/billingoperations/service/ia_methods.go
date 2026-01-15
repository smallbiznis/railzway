package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/smallbiznis/railzway/internal/billingoperations/domain"
	"github.com/smallbiznis/railzway/internal/orgcontext"
	"go.uber.org/zap"
)

// GetInbox returns unassigned risky items that need attention
// Routing Rule: Entity appears ONLY if it meets billing risk criteria AND has NO active assignment
func (s *Service) GetInbox(ctx context.Context, req domain.InboxRequest) (domain.InboxResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.InboxResponse{}, domain.ErrInvalidOrganization
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 25
	}

	currency, err := s.repo.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.InboxResponse{}, err
	}

	rows, err := s.repo.ListInboxItems(ctx, orgID, limit, s.clock.Now().UTC())
	if err != nil {
		return domain.InboxResponse{}, err
	}

	items := make([]domain.InboxItem, 0, len(rows))
	for _, row := range rows {
		var lastAttempt *time.Time
		if row.LastAttempt.Valid {
			t := row.LastAttempt.Time.UTC()
			lastAttempt = &t
		}

		items = append(items, domain.InboxItem{
			EntityType:   row.EntityType,
			EntityID:     row.EntityID,
			EntityName:   row.EntityName,
			RiskCategory: row.RiskCategory,
			RiskScore:    row.RiskScore,
			AmountDue:    row.AmountDue,
			Currency:     currency,
			DaysOverdue:  int(row.DaysOverdue),
			LastAttempt:  lastAttempt,
			PublicToken:  decryptToken(s.encKey, row.TokenHash.String),
		})
	}

	return domain.InboxResponse{
		Items:    items,
		Currency: currency,
	}, nil
}

// GetMyWork returns tasks currently owned by the logged-in user
// Routing Rule: assigned_to = current_user AND status IN (claimed, in_progress)
// CRITICAL: Never filters by billing state - tasks remain visible until explicitly resolved/released
func (s *Service) GetMyWork(ctx context.Context, userID string, req domain.MyWorkRequest) (domain.MyWorkResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.MyWorkResponse{}, domain.ErrInvalidOrganization
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	currency, err := s.repo.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.MyWorkResponse{}, err
	}

	now := s.clock.Now().UTC()
	rows, err := s.repo.ListMyWorkItems(ctx, orgID, userID, limit, now)
	if err != nil {
		return domain.MyWorkResponse{}, err
	}

	items := make([]domain.MyWorkItem, 0, len(rows))
	for _, row := range rows {
		// Parse snapshot metadata
		var snapshot map[string]interface{}
		if len(row.SnapshotMetadata) > 0 {
			if err := json.Unmarshal(row.SnapshotMetadata, &snapshot); err != nil {
				s.log.Warn("failed to unmarshal snapshot metadata", zap.Error(err))
				snapshot = make(map[string]interface{})
			}
		} else {
			snapshot = make(map[string]interface{})
		}

		// Extract snapshot values (stable)
		entityName := row.EntityName.String
		if name, ok := snapshot["entity_name"].(string); ok && name != "" {
			entityName = name
		}

		amountDueAtClaim := int64(0)
		if amt, ok := snapshot["amount_due"].(float64); ok {
			amountDueAtClaim = int64(amt)
		}

		daysOverdueAtClaim := 0
		if days, ok := snapshot["days_overdue"].(float64); ok {
			daysOverdueAtClaim = int(days)
		}

		// Current values (optional)
		var currentAmountDue int64
		if row.CurrentAmountDue.Valid {
			currentAmountDue = row.CurrentAmountDue.Int64
		}

		var currentDaysOverdue int
		if row.CurrentDaysOverdue.Valid {
			currentDaysOverdue = int(row.CurrentDaysOverdue.Float64)
		}

		// Calculate assignment age
		duration := now.Sub(row.AssignedAt)
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		var assignmentAge string
		if hours > 0 {
			assignmentAge = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			assignmentAge = fmt.Sprintf("%dm", minutes)
		}

		var lastActionAt *time.Time
		if row.LastActionAt.Valid {
			t := row.LastActionAt.Time.UTC()
			lastActionAt = &t
		}

		items = append(items, domain.MyWorkItem{
			AssignmentID:  row.AssignmentID,
			EntityType:    row.EntityType,
			EntityID:      row.EntityID,
			EntityName:    entityName,
			CustomerName:  row.CustomerName.String,
			CustomerEmail: row.CustomerEmail.String,
			InvoiceNumber: row.InvoiceNumber.String,

			AmountDueAtClaim:   amountDueAtClaim,
			DaysOverdueAtClaim: daysOverdueAtClaim,
			CurrentAmountDue:   currentAmountDue,
			CurrentDaysOverdue: currentDaysOverdue,
			Currency:           currency,
			ClaimedAt:          row.AssignedAt.UTC(),
			AssignmentAge:      assignmentAge,
			Status:             row.Status,
			LastActionAt:       lastActionAt,
			PublicToken:        decryptToken(s.encKey, row.TokenHash.String),
		})
	}

	return domain.MyWorkResponse{
		Items:    items,
		Currency: currency,
	}, nil
}

// GetRecentlyResolved returns completed, released, or escalated work
// Routing Rule: assigned_to = current_user AND status IN (resolved, released, escalated) AND resolved_at > 30 days ago
func (s *Service) GetRecentlyResolved(ctx context.Context, userID string, req domain.RecentlyResolvedRequest) (domain.RecentlyResolvedResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.RecentlyResolvedResponse{}, domain.ErrInvalidOrganization
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	now := s.clock.Now().UTC()
	thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)

	currency, err := s.repo.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.RecentlyResolvedResponse{}, err
	}

	rows, err := s.repo.ListRecentlyResolvedItems(ctx, orgID, userID, limit, thirtyDaysAgo)
	if err != nil {
		return domain.RecentlyResolvedResponse{}, err
	}

	items := make([]domain.ResolvedItem, 0, len(rows))
	for _, row := range rows {
		// Parse snapshot metadata
		var snapshot map[string]interface{}
		if len(row.SnapshotMetadata) > 0 {
			if err := json.Unmarshal(row.SnapshotMetadata, &snapshot); err != nil {
				s.log.Warn("failed to unmarshal snapshot metadata", zap.Error(err))
				snapshot = make(map[string]interface{})
			}
		} else {
			snapshot = make(map[string]interface{})
		}

		entityName := ""
		if name, ok := snapshot["entity_name"].(string); ok {
			entityName = name
		}

		amountDueAtClaim := int64(0)
		if amt, ok := snapshot["amount_due"].(float64); ok {
			amountDueAtClaim = int64(amt)
		}

		// Calculate duration
		duration := row.ResolvedAt.Sub(row.AssignedAt)
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		var durationStr string
		if hours > 0 {
			durationStr = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			durationStr = fmt.Sprintf("%dm", minutes)
		}

		items = append(items, domain.ResolvedItem{
			AssignmentID:     row.AssignmentID,
			EntityType:       row.EntityType,
			EntityID:         row.EntityID,
			EntityName:       entityName,
			Status:           row.Status,
			ResolvedAt:       row.ResolvedAt.UTC(),
			ResolvedBy:       row.ResolvedBy.String,
			Reason:           row.ReleaseReason.String,
			ClaimedAt:        row.AssignedAt.UTC(),
			Duration:         durationStr,
			AmountDueAtClaim: amountDueAtClaim,
			Currency:         currency,
		})
	}

	return domain.RecentlyResolvedResponse{
		Items: items,
	}, nil
}

// GetTeamView returns operational oversight for managers
// Routing Rule: Manager role only
// Explicitly FORBIDDEN: Leaderboards, ranking by score, best/worst labels
func (s *Service) GetTeamView(ctx context.Context, req domain.TeamViewRequest) (domain.TeamViewResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.TeamViewResponse{}, domain.ErrInvalidOrganization
	}

	currency, err := s.repo.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.TeamViewResponse{}, err
	}

	rows, err := s.repo.GetTeamViewStats(ctx, orgID, s.clock.Now().UTC())
	if err != nil {
		return domain.TeamViewResponse{}, err
	}

	var (
		totalActiveAssignments int
		totalExposure          int64
		totalEscalationCount   int
		weightedAgeMinutes     int
		totalAssignmentsForAge int
	)

	members := make([]domain.TeamMemberWorkload, 0, len(rows))
	for _, row := range rows {
		// Update summary aggregates
		totalActiveAssignments += row.ActiveAssignments
		totalExposure += row.TotalExposureOwned
		totalEscalationCount += row.EscalationCount
		if row.ActiveAssignments > 0 {
			weightedAgeMinutes += row.AvgAssignmentAgeMinutes * row.ActiveAssignments
			totalAssignmentsForAge += row.ActiveAssignments
		}

		// Format average assignment age
		hours := row.AvgAssignmentAgeMinutes / 60
		minutes := row.AvgAssignmentAgeMinutes % 60
		var avgAge string
		if hours > 0 {
			avgAge = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			avgAge = fmt.Sprintf("%dm", minutes)
		}

		members = append(members, domain.TeamMemberWorkload{
			UserID:             row.UserID,
			ActiveAssignments:  row.ActiveAssignments,
			AvgAssignmentAge:   avgAge,
			TotalExposureOwned: row.TotalExposureOwned,
			EscalationCount:    row.EscalationCount,
		})
	}

	// Calculate global average age
	var globalAvgAge string
	if totalAssignmentsForAge > 0 {
		avgMins := weightedAgeMinutes / totalAssignmentsForAge
		h := avgMins / 60
		m := avgMins % 60
		if h > 0 {
			globalAvgAge = fmt.Sprintf("%dh %dm", h, m)
		} else {
			globalAvgAge = fmt.Sprintf("%dm", m)
		}
	} else {
		globalAvgAge = "0m"
	}

	return domain.TeamViewResponse{
		Members: members,
		Summary: domain.TeamSummary{
			TotalActiveAssignments: totalActiveAssignments,
			TotalExposure:          totalExposure,
			AvgAssignmentAge:       globalAvgAge,
			EscalationCount:        totalEscalationCount,
		},
		Currency: currency,
	}, nil
}

// GetInvoicePayments returns payment events associated with an invoice
func (s *Service) GetInvoicePayments(ctx context.Context, invoiceID string) (domain.InvoicePaymentsResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.InvoicePaymentsResponse{}, domain.ErrInvalidOrganization
	}

	invID, err := parseSnowflakeID(invoiceID)
	if err != nil {
		return domain.InvoicePaymentsResponse{}, err
	}

	rows, err := s.repo.ListInvoicePayments(ctx, orgID, invID)
	if err != nil {
		return domain.InvoicePaymentsResponse{}, err
	}

	payments := make([]domain.PaymentDetail, 0, len(rows))
	for _, row := range rows {
		var payload map[string]interface{}
		if err := json.Unmarshal(row.Payload, &payload); err != nil {
			continue
		}

		// Extract amount (Stripe: data.object.amount)
		amount := int64(0)
		data, _ := payload["data"].(map[string]interface{})
		obj, _ := data["object"].(map[string]interface{})
		if val, ok := obj["amount"].(float64); ok {
			amount = int64(val)
		}

		// Status mapping
		status := "unknown"
		switch row.EventType {
		case "payment_succeeded", "charge.succeeded":
			status = "succeeded"
		case "payment_failed":
			status = "failed"
		case "refunded", "charge.refunded":
			status = "refunded"
		}

		// Extract card details (Stripe specific)
		// charges.data[0].payment_method_details.card or payment_method_details.card
		var method, brand, last4 string

		var paymentMethodDetails map[string]interface{}

		// Check top-level payment_method_details (Charge object)
		if pmd, ok := obj["payment_method_details"].(map[string]interface{}); ok {
			paymentMethodDetails = pmd
		} else if charges, ok := obj["charges"].(map[string]interface{}); ok {
			// Check inside charges (PaymentIntent object)
			if dataList, ok := charges["data"].([]interface{}); ok && len(dataList) > 0 {
				if firstCharge, ok := dataList[0].(map[string]interface{}); ok {
					if pmd, ok := firstCharge["payment_method_details"].(map[string]interface{}); ok {
						paymentMethodDetails = pmd
					}
				}
			}
		}

		if paymentMethodDetails != nil {
			if typeStr, ok := paymentMethodDetails["type"].(string); ok {
				method = typeStr
			}
			if card, ok := paymentMethodDetails["card"].(map[string]interface{}); ok {
				if b, ok := card["brand"].(string); ok {
					brand = b
				}
				if l, ok := card["last4"].(string); ok {
					last4 = l
				}
			}
		}

		payments = append(payments, domain.PaymentDetail{
			PaymentID:  row.ProviderPaymentID,
			Amount:     amount,
			Currency:   row.Currency,
			OccurredAt: row.ReceivedAt.UTC(),
			Provider:   row.Provider,
			Method:     method,
			CardBrand:  brand,
			CardLast4:  last4,
			Status:     status,
		})
	}

	return domain.InvoicePaymentsResponse{
		Payments: payments,
	}, nil
}
func (s *Service) GetExposureAnalysis(ctx context.Context, req domain.ExposureAnalysisRequest) (domain.ExposureAnalysisResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.ExposureAnalysisResponse{}, domain.ErrInvalidOrganization
	}

	currency, err := s.repo.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.ExposureAnalysisResponse{}, err
	}

	now := s.clock.Now().UTC()

	stats, err := s.repo.GetExposureStats(ctx, orgID, now)
	if err != nil {
		return domain.ExposureAnalysisResponse{}, err
	}

	topRows, err := s.repo.ListTopHighExposure(ctx, orgID, now)
	if err != nil {
		return domain.ExposureAnalysisResponse{}, err
	}

	topItems := make([]domain.InboxItem, 0, len(topRows))
	for _, r := range topRows {
		topItems = append(topItems, domain.InboxItem{
			EntityType:   "customer",
			EntityName:   r.EntityName,
			AmountDue:    r.AmountDue,
			RiskScore:    r.RiskScore,
			DaysOverdue:  r.DaysOverdue,
			Currency:     currency,
			RiskCategory: "high_exposure",
		})
	}

	// 3. Construct Response
	aging := []domain.ExposureBucket{
		{Bucket: "Current", Amount: stats.CurrentAmount, Count: 0},
		{Bucket: "1-30 Days", Amount: stats.Bucket0To30, Count: 0},
		{Bucket: "31-60 Days", Amount: stats.Bucket31To60, Count: 0},
		{Bucket: "61-90 Days", Amount: stats.Bucket61To90, Count: 0},
		{Bucket: "90+ Days", Amount: stats.Bucket90Plus, Count: 0},
	}

	// For Risk Category, we simplify:
	// "Overdue" -> Sum of all buckets > 0
	// "High Exposure" -> Sum of top 5 customers (simplified) or sum of all > 100k
	// "Failed Payment" -> We could query, but for now let's stick to Overdue vs Current
	riskCats := []domain.ExposureCategory{
		{Category: "Overdue", Amount: stats.Bucket0To30 + stats.Bucket31To60 + stats.Bucket61To90 + stats.Bucket90Plus, Count: stats.OverdueCount},
		{Category: "Current", Amount: stats.CurrentAmount, Count: 0}, // Count not easily available from agg
	}

	return domain.ExposureAnalysisResponse{
		TotalExposure:   stats.TotalExposure,
		Currency:        currency,
		ByRiskCategory:  riskCats,
		ByAgingBucket:   aging,
		TopHighExposure: topItems,
	}, nil
}
