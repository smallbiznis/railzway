package logger

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestFromContextIncludesTrace(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	orig := zap.L()
	zap.ReplaceGlobals(zap.New(core))
	defer zap.ReplaceGlobals(orig)

	traceID, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	spanID, _ := trace.SpanIDFromHex("0123456789abcdef")
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	FromContext(ctx).Info("hello")
	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["trace_id"] != traceID.String() {
		t.Fatalf("expected trace_id %q, got %q", traceID.String(), fields["trace_id"])
	}
	if fields["span_id"] != spanID.String() {
		t.Fatalf("expected span_id %q, got %q", spanID.String(), fields["span_id"])
	}
}
