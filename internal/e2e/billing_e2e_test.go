package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/server"
	usagedomain "github.com/smallbiznis/valora/internal/usage/domain"
)

type usageRow struct {
	ID             snowflake.ID  `gorm:"column:id"`
	SubscriptionID snowflake.ID  `gorm:"column:subscription_id"`
	MeterID        *snowflake.ID `gorm:"column:meter_id"`
	Status         string        `gorm:"column:status"`
	Value          float64       `gorm:"column:value"`
	RecordedAt     time.Time     `gorm:"column:recorded_at"`
	IdempotencyKey string        `gorm:"column:idempotency_key"`
}

type ratingResultRow struct {
	ID        snowflake.ID  `gorm:"column:id"`
	PriceID   snowflake.ID  `gorm:"column:price_id"`
	MeterID   *snowflake.ID `gorm:"column:meter_id"`
	Quantity  float64       `gorm:"column:quantity"`
	UnitPrice int64         `gorm:"column:unit_price"`
	Amount    int64         `gorm:"column:amount"`
	Currency  string        `gorm:"column:currency"`
	Source    string        `gorm:"column:source"`
}

func TestE2E_RatingFlatOnly(t *testing.T) {
	resetDatabase(t, env.db)

	client, orgID := loginAdmin(t)
	suffix := testSuffix(t)
	meterID, _ := createAdminMeter(t, client, orgID, "flat-meter-"+suffix)
	productID := createAdminProduct(t, client, orgID, "flat-product-"+suffix)
	priceID := createAdminPrice(t, client, orgID, map[string]any{
		"product_id":             productID,
		"name":                   "Flat",
		"code":                   "flat-price-" + suffix,
		"pricing_model":          "FLAT",
		"billing_mode":           "LICENSED",
		"billing_interval":       "MONTH",
		"billing_interval_count": 1,
		"tax_behavior":           "EXCLUSIVE",
	})
	createAdminPriceAmount(t, client, orgID, map[string]any{
		"price_id":             priceID,
		"currency":             "USD",
		"unit_amount_cents":    500,
		"minimum_amount_cents": 500,
	})
	customerID := createAdminCustomer(t, client, orgID, "Flat Customer "+suffix)
	subscriptionID := createAdminSubscription(t, client, orgID, map[string]any{
		"customer_id":        customerID,
		"collection_mode":    "SEND_INVOICE",
		"billing_cycle_type": "MONTHLY",
		"items": []map[string]any{
			{"price_id": priceID, "meter_id": meterID, "quantity": 1},
		},
	})
	activateSubscription(t, client, orgID, subscriptionID)

	cycle := ensureBillingCycle(t, subscriptionID)
	periodStart, periodEnd := pastWindow()
	updateBillingCycleWindow(t, cycle.ID, periodStart, periodEnd)
	runRatingForCycles(t)

	if countRows(t, env.db, "usage_events", "org_id = ?", mustParseID(t, orgID)) != 0 {
		t.Fatalf("expected zero usage_events for flat-only billing")
	}

	results := fetchRatingResults(t, subscriptionID)
	if len(results) != 1 {
		t.Fatalf("expected 1 rating result, got %d", len(results))
	}
	if results[0].Amount != 500 {
		t.Fatalf("expected flat amount 500, got %d", results[0].Amount)
	}
}

func TestE2E_RatingUsageOnly(t *testing.T) {
	resetDatabase(t, env.db)

	client, orgID := loginAdmin(t)
	apiKey := createAPIKey(t, client, orgID)
	suffix := testSuffix(t)
	meterID, meterCode := createAdminMeter(t, client, orgID, "usage-meter-"+suffix)
	productID := createAdminProduct(t, client, orgID, "usage-product-"+suffix)
	priceID := createAdminPrice(t, client, orgID, map[string]any{
		"product_id":             productID,
		"name":                   "Usage Price",
		"code":                   "usage-price-" + suffix,
		"pricing_model":          "PER_UNIT",
		"billing_mode":           "METERED",
		"billing_interval":       "MONTH",
		"billing_interval_count": 1,
		"aggregate_usage":        "SUM",
		"billing_unit":           "API_CALL",
		"tax_behavior":           "EXCLUSIVE",
	})
	createAdminPriceAmount(t, client, orgID, map[string]any{
		"price_id":          priceID,
		"meter_id":          meterID,
		"currency":          "USD",
		"unit_amount_cents": 120,
	})
	customerID := createAdminCustomer(t, client, orgID, "Usage Customer "+suffix)
	subscriptionID := createAdminSubscription(t, client, orgID, map[string]any{
		"customer_id":        customerID,
		"collection_mode":    "SEND_INVOICE",
		"billing_cycle_type": "MONTHLY",
		"items": []map[string]any{
			{"price_id": priceID, "meter_id": meterID, "quantity": 1},
		},
	})
	activateSubscription(t, client, orgID, subscriptionID)

	periodStart, periodEnd := pastWindow()
	if err := env.db.Exec(
		`UPDATE subscriptions SET start_at = ? WHERE id = ?`,
		periodStart,
		mustParseID(t, subscriptionID),
	).Error; err != nil {
		t.Fatalf("update subscription start: %v", err)
	}
	recordedAt := periodStart.Add(30 * time.Minute)
	usageEvents := []float64{3, 4}
	for i, value := range usageEvents {
		ingestUsage(t, apiKey, map[string]any{
			"customer_id":     customerID,
			"meter_code":      meterCode,
			"value":           value,
			"recorded_at":     recordedAt.Add(time.Duration(i) * time.Minute),
			"idempotency_key": fmt.Sprintf("usage-only-%d-%d", i, time.Now().UnixNano()),
		})
	}

	runSnapshot(t)
	assertUsageStatus(t, customerID, usagedomain.UsageStatusEnriched, 2)

	cycle := ensureBillingCycle(t, subscriptionID)
	updateBillingCycleWindow(t, cycle.ID, periodStart, periodEnd)
	runRatingForCycles(t)

	results := fetchRatingResults(t, subscriptionID)
	if len(results) != 1 {
		t.Fatalf("expected 1 rating result, got %d", len(results))
	}
	expectedQty := usageEvents[0] + usageEvents[1]
	if results[0].Quantity != expectedQty {
		t.Fatalf("expected quantity %.2f, got %.2f", expectedQty, results[0].Quantity)
	}
	if results[0].Amount != int64(expectedQty*120) {
		t.Fatalf("expected amount %d, got %d", int64(expectedQty*120), results[0].Amount)
	}
	if results[0].MeterID == nil {
		t.Fatalf("expected usage rating result to include meter_id")
	}
}

func TestE2E_RatingHybrid(t *testing.T) {
	resetDatabase(t, env.db)

	client, orgID := loginAdmin(t)
	apiKey := createAPIKey(t, client, orgID)
	suffix := testSuffix(t)

	usageMeterID, usageMeterCode := createAdminMeter(t, client, orgID, "hybrid-usage-meter-"+suffix)
	productID := createAdminProduct(t, client, orgID, "hybrid-product-"+suffix)

	flatPriceID := createAdminPrice(t, client, orgID, map[string]any{
		"product_id":             productID,
		"name":                   "Hybrid Flat",
		"code":                   "hybrid-flat-price-" + suffix,
		"pricing_model":          "FLAT",
		"billing_mode":           "LICENSED",
		"billing_interval":       "MONTH",
		"billing_interval_count": 1,
		"tax_behavior":           "EXCLUSIVE",
	})
	createAdminPriceAmount(t, client, orgID, map[string]any{
		"price_id":             flatPriceID,
		"currency":             "USD",
		"unit_amount_cents":    900,
		"minimum_amount_cents": 900,
	})

	usagePriceID := createAdminPrice(t, client, orgID, map[string]any{
		"product_id":             productID,
		"name":                   "Hybrid Usage",
		"code":                   "hybrid-usage-price-" + suffix,
		"pricing_model":          "PER_UNIT",
		"billing_mode":           "METERED",
		"billing_interval":       "MONTH",
		"billing_interval_count": 1,
		"aggregate_usage":        "SUM",
		"billing_unit":           "API_CALL",
		"tax_behavior":           "EXCLUSIVE",
	})
	createAdminPriceAmount(t, client, orgID, map[string]any{
		"price_id":          usagePriceID,
		"meter_id":          usageMeterID,
		"currency":          "USD",
		"unit_amount_cents": 150,
	})

	customerID := createAdminCustomer(t, client, orgID, "Hybrid Customer "+suffix)
	subscriptionID := createAdminSubscription(t, client, orgID, map[string]any{
		"customer_id":        customerID,
		"collection_mode":    "SEND_INVOICE",
		"billing_cycle_type": "MONTHLY",
		"items": []map[string]any{
			{"price_id": flatPriceID, "quantity": 1},
			{"price_id": usagePriceID, "quantity": 1},
		},
	})
	activateSubscription(t, client, orgID, subscriptionID)

	periodStart, periodEnd := pastWindow()
	if err := env.db.Exec(
		`UPDATE subscriptions SET start_at = ? WHERE id = ?`,
		periodStart,
		mustParseID(t, subscriptionID),
	).Error; err != nil {
		t.Fatalf("update subscription start: %v", err)
	}
	ingestUsage(t, apiKey, map[string]any{
		"customer_id":     customerID,
		"meter_code":      usageMeterCode,
		"value":           5.0,
		"recorded_at":     periodStart.Add(20 * time.Minute),
		"idempotency_key": fmt.Sprintf("hybrid-usage-%d", time.Now().UnixNano()),
	})

	runSnapshot(t)
	assertUsageStatus(t, customerID, usagedomain.UsageStatusEnriched, 1)

	cycle := ensureBillingCycle(t, subscriptionID)
	updateBillingCycleWindow(t, cycle.ID, periodStart, periodEnd)
	runRatingForCycles(t)

	results := fetchRatingResults(t, subscriptionID)
	if len(results) != 2 {
		t.Fatalf("expected 2 rating results, got %d", len(results))
	}

	flatFound := false
	usageFound := false
	for _, result := range results {
		switch result.PriceID.String() {
		case flatPriceID:
			flatFound = true
			if result.Amount != 900 {
				t.Fatalf("expected flat amount 900, got %d", result.Amount)
			}
		case usagePriceID:
			usageFound = true
			if result.Amount != 750 {
				t.Fatalf("expected usage amount 750, got %d", result.Amount)
			}
		}
	}
	if !flatFound || !usageFound {
		t.Fatalf("expected both flat and usage rating results")
	}

}

func TestE2E_CustomerLifecycleValidation(t *testing.T) {
	resetDatabase(t, env.db)

	client, orgID := loginAdmin(t)
	apiKey := createAPIKey(t, client, orgID)
	suffix := testSuffix(t)
	meterID, meterCode := createAdminMeter(t, client, orgID, "cust-meter-"+suffix)
	productID := createAdminProduct(t, client, orgID, "cust-product-"+suffix)
	priceID := createAdminPrice(t, client, orgID, map[string]any{
		"product_id":             productID,
		"name":                   "Cust Price",
		"code":                   "cust-price-" + suffix,
		"pricing_model":          "PER_UNIT",
		"billing_mode":           "METERED",
		"billing_interval":       "MONTH",
		"billing_interval_count": 1,
		"aggregate_usage":        "SUM",
		"billing_unit":           "API_CALL",
		"tax_behavior":           "EXCLUSIVE",
	})
	createAdminPriceAmount(t, client, orgID, map[string]any{
		"price_id":          priceID,
		"meter_id":          meterID,
		"currency":          "USD",
		"unit_amount_cents": 100,
	})
	customerID := createAdminCustomer(t, client, orgID, "Cust Customer "+suffix)
	subscriptionID := createAdminSubscription(t, client, orgID, map[string]any{
		"customer_id":        customerID,
		"collection_mode":    "SEND_INVOICE",
		"billing_cycle_type": "MONTHLY",
		"items": []map[string]any{
			{"price_id": priceID, "meter_id": meterID, "quantity": 1},
		},
	})
	activateSubscription(t, client, orgID, subscriptionID)

	resp, _ := ingestUsageWithResponse(t, apiKey, map[string]any{
		"customer_id":     "",
		"meter_code":      meterCode,
		"value":           1.0,
		"recorded_at":     time.Now().UTC(),
		"idempotency_key": "missing-customer",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400 for usage without customer, got %d", resp.StatusCode)
	}

	resp, _ = createSubscriptionWithResponse(t, client, orgID, map[string]any{
		"customer_id":        "",
		"collection_mode":    "SEND_INVOICE",
		"billing_cycle_type": "MONTHLY",
		"items": []map[string]any{
			{"price_id": priceID, "meter_id": meterID, "quantity": 1},
		},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400 for subscription without customer, got %d", resp.StatusCode)
	}

	ingestUsage(t, apiKey, map[string]any{
		"customer_id":     customerID,
		"meter_code":      meterCode,
		"value":           2.0,
		"recorded_at":     time.Now().UTC(),
		"idempotency_key": fmt.Sprintf("cust-resolve-%d", time.Now().UnixNano()),
	})

	runSnapshot(t)

	var row usageRow
	if err := env.db.Raw(
		`SELECT id, subscription_id FROM usage_events WHERE customer_id = ? ORDER BY created_at DESC LIMIT 1`,
		mustParseID(t, customerID),
	).Scan(&row).Error; err != nil {
		t.Fatalf("query usage event: %v", err)
	}
	if row.SubscriptionID != mustParseID(t, subscriptionID) {
		t.Fatalf("expected usage to resolve subscription_id %s, got %s", subscriptionID, row.SubscriptionID.String())
	}
}

func TestE2E_SubscriptionLifecycleValidation(t *testing.T) {
	resetDatabase(t, env.db)

	client, orgID := loginAdmin(t)
	apiKey := createAPIKey(t, client, orgID)
	suffix := testSuffix(t)
	meterID, meterCode := createAdminMeter(t, client, orgID, "sub-meter-"+suffix)
	productID := createAdminProduct(t, client, orgID, "sub-product-"+suffix)
	priceID := createAdminPrice(t, client, orgID, map[string]any{
		"product_id":             productID,
		"name":                   "Sub Price",
		"code":                   "sub-price-" + suffix,
		"pricing_model":          "PER_UNIT",
		"billing_mode":           "METERED",
		"billing_interval":       "MONTH",
		"billing_interval_count": 1,
		"aggregate_usage":        "SUM",
		"billing_unit":           "API_CALL",
		"tax_behavior":           "EXCLUSIVE",
	})
	createAdminPriceAmount(t, client, orgID, map[string]any{
		"price_id":          priceID,
		"meter_id":          meterID,
		"currency":          "USD",
		"unit_amount_cents": 100,
	})
	customerID := createAdminCustomer(t, client, orgID, "Sub Customer "+suffix)
	subscriptionID := createAdminSubscription(t, client, orgID, map[string]any{
		"customer_id":        customerID,
		"collection_mode":    "SEND_INVOICE",
		"billing_cycle_type": "MONTHLY",
		"items": []map[string]any{
			{"price_id": priceID, "meter_id": meterID, "quantity": 1},
		},
	})
	activateSubscription(t, client, orgID, subscriptionID)

	now := time.Now().UTC()
	startAt := now.Add(-2 * time.Hour)
	endAt := now.Add(-1 * time.Hour)
	if err := env.db.Exec(
		`UPDATE subscriptions SET start_at = ?, end_at = ? WHERE id = ?`,
		startAt,
		endAt,
		mustParseID(t, subscriptionID),
	).Error; err != nil {
		t.Fatalf("update subscription window: %v", err)
	}

	events := []struct {
		key         string
		idempotency string
		at          time.Time
		value       float64
	}{
		{key: "before-start", idempotency: fmt.Sprintf("sub-life-before-%d", time.Now().UnixNano()), at: now.Add(-3 * time.Hour), value: 1},
		{key: "inside-window", idempotency: fmt.Sprintf("sub-life-inside-%d", time.Now().UnixNano()), at: now.Add(-90 * time.Minute), value: 2},
		{key: "after-end", idempotency: fmt.Sprintf("sub-life-after-%d", time.Now().UnixNano()), at: now.Add(-30 * time.Minute), value: 3},
	}
	for _, event := range events {
		ingestUsage(t, apiKey, map[string]any{
			"customer_id":     customerID,
			"meter_code":      meterCode,
			"value":           event.value,
			"recorded_at":     event.at,
			"idempotency_key": event.idempotency,
		})
	}

	runSnapshot(t)

	assertUsageStatusByKey(t, events[0].idempotency, usagedomain.UsageStatusUnmatchedSubscription)
	assertUsageStatusByKey(t, events[1].idempotency, usagedomain.UsageStatusEnriched)
	assertUsageStatusByKey(t, events[2].idempotency, usagedomain.UsageStatusUnmatchedSubscription)

	cycle := ensureBillingCycle(t, subscriptionID)
	updateBillingCycleWindow(t, cycle.ID, now.Add(-3*time.Hour), now.Add(-10*time.Minute))
	runRatingForCycles(t)

	results := fetchRatingResults(t, subscriptionID)
	if len(results) != 1 {
		t.Fatalf("expected 1 rating result, got %d", len(results))
	}
	if results[0].Quantity != events[1].value {
		t.Fatalf("expected rated quantity %.2f, got %.2f", events[1].value, results[0].Quantity)
	}
}

func TestE2E_HybridPricing_FlatAndUsage(t *testing.T) {
	resetDatabase(t, env.db)

	client, orgID := loginAdmin(t)
	apiKey := createAPIKey(t, client, orgID)
	suffix := testSuffix(t)
	usageMeterID, usageMeterCode := createAdminMeter(t, client, orgID, "hybrid-usage-meter-"+suffix)
	productID := createAdminProduct(t, client, orgID, "hybrid-product-"+suffix)

	flatPriceID := createAdminPrice(t, client, orgID, map[string]any{
		"product_id":             productID,
		"name":                   "Hybrid Flat",
		"code":                   "hybrid-flat-price-" + suffix,
		"pricing_model":          "FLAT",
		"billing_mode":           "LICENSED",
		"billing_interval":       "MONTH",
		"billing_interval_count": 1,
		"tax_behavior":           "EXCLUSIVE",
	})

	createAdminPriceAmount(t, client, orgID, map[string]any{
		"price_id":          flatPriceID,
		"currency":          "USD",
		"unit_amount_cents": 900,
	})

	usagePriceID := createAdminPrice(t, client, orgID, map[string]any{
		"product_id":             productID,
		"name":                   "Hybrid Usage",
		"code":                   "hybrid-usage-price-" + suffix,
		"pricing_model":          "PER_UNIT",
		"billing_mode":           "METERED",
		"billing_interval":       "MONTH",
		"billing_interval_count": 1,
		"aggregate_usage":        "SUM",
		"billing_unit":           "API_CALL",
		"tax_behavior":           "EXCLUSIVE",
	})

	createAdminPriceAmount(t, client, orgID, map[string]any{
		"price_id":          usagePriceID,
		"meter_id":          usageMeterID,
		"currency":          "USD",
		"unit_amount_cents": 150,
	})

	subs := createManyCustomersAndSubscriptionsParallel(t, client, orgID, 10000, flatPriceID, usagePriceID, 30)

	usages := make([]map[string]any, 0, len(subs))
	for i := 0; i < len(subs); i++ {
		usages = append(usages, map[string]any{
			"customer_id": subs[i].CustomerID,
			"meter_code":  usageMeterCode,
			"value":       randomUsageValue(),
			"recorded_at": time.Now().UTC(),
		})
	}

	ingestUsageParallel(t, apiKey, usages, 30)
}

func createAdminMeter(t *testing.T, client *http.Client, orgID, code string) (string, string) {
	t.Helper()
	headers := map[string]string{server.HeaderOrg: orgID}
	req := map[string]any{
		"code":             code,
		"name":             "Meter " + code,
		"aggregation_type": "SUM",
		"unit":             "API_CALL",
	}
	resp, body := doJSON(t, client, http.MethodPost, env.baseURL+"/admin/meters", req, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create meter failed: %d: %s", resp.StatusCode, string(body))
	}
	var payload struct {
		Data struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode meter response: %v", err)
	}
	return payload.Data.ID, payload.Data.Code
}

func createAdminProduct(t *testing.T, client *http.Client, orgID, code string) string {
	t.Helper()
	headers := map[string]string{server.HeaderOrg: orgID}
	req := map[string]any{
		"code": code,
		"name": "Product " + code,
	}
	resp, body := doJSON(t, client, http.MethodPost, env.baseURL+"/admin/products", req, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create product failed: %d: %s", resp.StatusCode, string(body))
	}
	var payload struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode product response: %v", err)
	}
	return payload.Data.ID
}

func createAdminPrice(t *testing.T, client *http.Client, orgID string, req map[string]any) string {
	t.Helper()
	headers := map[string]string{server.HeaderOrg: orgID}
	resp, body := doJSON(t, client, http.MethodPost, env.baseURL+"/admin/prices", req, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create price failed: %d: %s", resp.StatusCode, string(body))
	}
	var payload struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode price response: %v", err)
	}
	return payload.Data.ID
}

func createAdminPriceAmount(t *testing.T, client *http.Client, orgID string, req map[string]any) {
	t.Helper()
	headers := map[string]string{server.HeaderOrg: orgID}
	resp, body := doJSON(t, client, http.MethodPost, env.baseURL+"/admin/price_amounts", req, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create price amount failed: %d: %s", resp.StatusCode, string(body))
	}
}

func createAdminCustomer(t *testing.T, client *http.Client, orgID, name string) string {
	t.Helper()
	headers := map[string]string{server.HeaderOrg: orgID}
	req := map[string]any{
		"name":  name,
		"email": strings.ToLower(strings.ReplaceAll(name, " ", ".")) + "@example.com",
	}
	resp, body := doJSON(t, client, http.MethodPost, env.baseURL+"/admin/customers", req, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create customer failed: %d: %s", resp.StatusCode, string(body))
	}
	var payload struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode customer response: %v", err)
	}
	return payload.Data.ID
}

func createAdminSubscription(t *testing.T, client *http.Client, orgID string, req map[string]any) string {
	t.Helper()
	resp, body := createSubscriptionWithResponse(t, client, orgID, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create subscription failed: %d: %s", resp.StatusCode, string(body))
	}
	var payload struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode subscription response: %v", err)
	}
	if payload.Data.Status != "DRAFT" {
		t.Fatalf("expected subscription status DRAFT, got %s", payload.Data.Status)
	}
	return payload.Data.ID
}

func createSubscriptionWithResponse(t *testing.T, client *http.Client, orgID string, req map[string]any) (*http.Response, []byte) {
	t.Helper()
	headers := map[string]string{server.HeaderOrg: orgID}
	return doJSON(t, client, http.MethodPost, env.baseURL+"/admin/subscriptions", req, headers)
}

func ingestUsage(t *testing.T, apiKey string, req map[string]any) {
	t.Helper()
	resp, body := ingestUsageWithResponse(t, apiKey, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("usage ingest failed: %d: %s", resp.StatusCode, string(body))
	}
}

func ingestUsageWithResponse(t *testing.T, apiKey string, req map[string]any) (*http.Response, []byte) {
	t.Helper()
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
	return doJSON(t, newHTTPClient(), http.MethodPost, env.baseURL+"/api/usage", req, headers)
}

func runSnapshot(t *testing.T) {
	t.Helper()
	if env.snapshotWorker == nil {
		t.Fatalf("snapshot worker not available")
	}
	if err := env.snapshotWorker.RunOnce(context.Background()); err != nil {
		t.Fatalf("snapshot run failed: %v", err)
	}
}

func ensureBillingCycle(t *testing.T, subscriptionID string) billingCycleRow {
	t.Helper()
	if err := env.scheduler.EnsureBillingCyclesJob(context.Background()); err != nil {
		t.Fatalf("ensure billing cycles: %v", err)
	}
	var cycle billingCycleRow
	if err := env.db.Raw(
		`SELECT id, status, period_start, period_end FROM billing_cycles WHERE subscription_id = ?`,
		mustParseID(t, subscriptionID),
	).Scan(&cycle).Error; err != nil {
		t.Fatalf("query billing cycle: %v", err)
	}
	if cycle.ID == 0 {
		t.Fatalf("expected billing cycle created")
	}
	return cycle
}

type SubInfo struct {
	CustomerID     string
	SubscriptionID string
}

func createManyCustomersAndSubscriptionsParallel(
	t *testing.T,
	client *http.Client,
	orgID string,
	count int,
	flatPriceID string,
	usagePriceID string,
	concurrency int,
) []SubInfo {
	t.Helper()

	results := make([]SubInfo, 0, count)
	resultsMu := sync.Mutex{}

	sem := make(chan struct{}, concurrency)
	wg := sync.WaitGroup{}

	var done int64
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			suffix := fmt.Sprintf("%05d", i+1)

			customerID := createAdminCustomer(
				t, client, orgID,
				"Hybrid Customer "+suffix,
			)

			subscriptionID := createAdminSubscription(t, client, orgID, map[string]any{
				"customer_id":        customerID,
				"collection_mode":    "SEND_INVOICE",
				"billing_cycle_type": "MONTHLY",
				"items": []map[string]any{
					{"price_id": flatPriceID, "quantity": 1},
					{"price_id": usagePriceID, "quantity": 1},
				},
			})

			activateSubscription(t, client, orgID, subscriptionID)

			resultsMu.Lock()
			results = append(results, SubInfo{
				CustomerID:     customerID,
				SubscriptionID: subscriptionID,
			})
			resultsMu.Unlock()

			n := atomic.AddInt64(&done, 1)
			if n%1000 == 0 {
				percent := float64(n) / float64(count) * 100
				t.Logf("progress: %d/%d (%.1f%%)", n, count, percent)
			}
		}(i)
	}

	wg.Wait()
	return results
}

func randomUsageValue() float64 {
	return float64(rand.Intn(991) + 10) // 10..1000
}

func ingestUsageParallel(
	t *testing.T,
	apiKey string,
	usages []map[string]any,
	concurrency int,
) {
	t.Helper()

	var ingested int64
	total := len(usages)

	sem := make(chan struct{}, concurrency)
	wg := sync.WaitGroup{}

	for i, payload := range usages {
		wg.Add(1)
		go func(i int, payload map[string]any) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			payload["idempotency_key"] =
				fmt.Sprintf("usage-%d-%d", i, time.Now().UnixNano())

			ingestUsage(t, apiKey, payload)

			n := atomic.AddInt64(&ingested, 1)
			if n%1000 == 0 || n == int64(total) {
				t.Logf(
					"usage progress: %d/%d (%.1f%%)",
					n,
					total,
					float64(n)/float64(total)*100,
				)
			}
		}(i, payload)
	}

	wg.Wait()
}

func updateBillingCycleWindow(t *testing.T, cycleID snowflake.ID, start, end time.Time) {
	t.Helper()
	if err := env.db.Exec(
		`UPDATE billing_cycles SET period_start = ?, period_end = ? WHERE id = ?`,
		start,
		end,
		cycleID,
	).Error; err != nil {
		t.Fatalf("update billing cycle window: %v", err)
	}
}

func runRatingForCycles(t *testing.T) {
	t.Helper()
	if err := env.scheduler.CloseCyclesJob(context.Background()); err != nil {
		t.Fatalf("close cycles job: %v", err)
	}
	if err := env.scheduler.RatingJob(context.Background()); err != nil {
		t.Fatalf("rating job: %v", err)
	}
}

func fetchRatingResults(t *testing.T, subscriptionID string) []ratingResultRow {
	t.Helper()
	var results []ratingResultRow
	if err := env.db.Raw(
		`SELECT id, price_id, meter_id, quantity, unit_price, amount, currency, source
		 FROM rating_results
		 WHERE subscription_id = ?
		 ORDER BY created_at ASC`,
		mustParseID(t, subscriptionID),
	).Scan(&results).Error; err != nil {
		t.Fatalf("query rating results: %v", err)
	}
	return results
}

func assertUsageStatus(t *testing.T, customerID string, status string, expected int) {
	t.Helper()
	var count int64
	if err := env.db.Table("usage_events").Where("customer_id = ? AND status = ?", mustParseID(t, customerID), status).Count(&count).Error; err != nil {
		t.Fatalf("count usage events: %v", err)
	}
	if int(count) != expected {
		t.Fatalf("expected %d usage_events with status %s, got %d", expected, status, count)
	}
}

func assertUsageStatusByKey(t *testing.T, key string, status string) {
	t.Helper()
	var row usageRow
	if err := env.db.Raw(
		`SELECT id, status, recorded_at
		 FROM usage_events
		 WHERE idempotency_key = ?
		 LIMIT 1`,
		key,
	).Scan(&row).Error; err != nil {
		t.Fatalf("query usage event: %v", err)
	}
	if row.ID == 0 {
		t.Fatalf("expected usage event for %s", key)
	}
	if row.Status != status {
		t.Fatalf("expected status %s for %s, got %s", status, key, row.Status)
	}
}

func pastWindow() (time.Time, time.Time) {
	now := time.Now().UTC()
	return now.Add(-2 * time.Hour), now.Add(-1 * time.Hour)
}

func testSuffix(t *testing.T) string {
	t.Helper()
	value := strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
	return strings.ReplaceAll(value, "_", "-")
}
