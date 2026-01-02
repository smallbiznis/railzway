package metrics

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestFilterAttributesDropsForbiddenLabels(t *testing.T) {
	attrs := FilterAttributes(
		attribute.String("org_id", "123"),
		attribute.String("customer_id", "456"),
		attribute.String("meter_code", "requests"),
	)
	if len(attrs) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(attrs))
	}
	if attrs[0].Key != "meter_code" {
		t.Fatalf("expected meter_code, got %s", attrs[0].Key)
	}
}
