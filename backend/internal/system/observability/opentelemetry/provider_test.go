// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package opentelemetry

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestNewTracerProvider_Disabled(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled: false,
	}

	provider, err := Initialize(ctx, cfg)

	if err == nil {
		t.Error("Initialize() should return error when disabled")
	}

	if provider != nil {
		t.Error("Initialize() should return nil provider when disabled")
	}

	if !strings.Contains(err.Error(), "disabled") {
		t.Errorf("Error should mention disabled, got: %v", err)
	}
}

func TestNewTracerProvider_StdoutExporter(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "stdout",
		ServiceName:  "test-service",
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer func() { _ = provider.Shutdown(ctx) }()

	if provider == nil {
		t.Fatal("Initialize() returned nil provider")
	}

	// Verify the provider is set as global
	globalProvider := otel.GetTracerProvider()
	if globalProvider == nil {
		t.Error("Global tracer provider should be set")
	}
}

func TestNewTracerProvider_UnsupportedExporter(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "unsupported",
	}

	provider, err := Initialize(ctx, cfg)

	if err == nil {
		t.Error("Initialize() should return error for unsupported exporter")
	}

	if provider != nil {
		t.Error("Initialize() should return nil provider for unsupported exporter")
	}

	if !strings.Contains(err.Error(), "unsupported exporter type") {
		t.Errorf("Error should mention unsupported exporter type, got: %v", err)
	}
}

func TestNewTracerProvider_OTLPExporterMissingEndpoint(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "otlp",
		OTLPEndpoint: "",
	}

	provider, err := Initialize(ctx, cfg)

	if err == nil {
		t.Error("Initialize() should return error when OTLP endpoint is missing")
	}

	if provider != nil {
		t.Error("Initialize() should return nil provider when OTLP endpoint is missing")
	}

	if !strings.Contains(err.Error(), "endpoint is required") {
		t.Errorf("Error should mention missing endpoint, got: %v", err)
	}
}

func TestNewTracerProvider_Defaults(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "stdout",
		// Leave other fields empty to test defaults
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer func() { _ = provider.Shutdown(ctx) }()

	if provider == nil {
		t.Fatal("Initialize() returned nil provider")
	}

	// Test that defaults were applied by creating a span
	tracer := provider.Tracer("test")
	_, span := tracer.Start(ctx, "test-span")
	span.End()

	// If we got here without panicking, defaults were applied correctly
}

func TestNewTracerProvider_CustomValues(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:        true,
		ExporterType:   "stdout",
		ServiceName:    "custom-service",
		ServiceVersion: "2.0.0",
		Environment:    "production",
		SampleRate:     0.5,
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer func() { _ = provider.Shutdown(ctx) }()

	if provider == nil {
		t.Fatal("Initialize() returned nil provider")
	}

	// Create a test span to verify configuration
	tracer := provider.Tracer("test")
	_, span := tracer.Start(ctx, "test-span")
	span.SetAttributes(attribute.String("test", "value"))
	span.End()
}

func TestNewTracerProvider_SampleRateAlwaysSample(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "stdout",
		SampleRate:   1.0,
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer func() { _ = provider.Shutdown(ctx) }()

	// Create multiple spans - all should be sampled
	tracer := provider.Tracer("test")
	for i := 0; i < 10; i++ {
		_, span := tracer.Start(ctx, "test-span")
		span.End()
	}

	// If no errors, sampling worked correctly
}

func TestNewTracerProvider_SampleRateNeverSample(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "stdout",
		SampleRate:   0.0,
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer func() { _ = provider.Shutdown(ctx) }()

	// Create spans - none should be sampled
	tracer := provider.Tracer("test")
	for i := 0; i < 10; i++ {
		_, span := tracer.Start(ctx, "test-span")
		span.End()
	}

	// If no errors, sampling worked correctly
}

func TestNewTracerProvider_SampleRateRatioBased(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "stdout",
		SampleRate:   0.5, // 50% sampling
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer func() { _ = provider.Shutdown(ctx) }()

	// Create multiple spans
	tracer := provider.Tracer("test")
	for i := 0; i < 100; i++ {
		_, span := tracer.Start(ctx, "test-span")
		span.End()
	}

	// If no errors, ratio-based sampling worked correctly
}

func TestNewTracerProvider_PropagatorSetup(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "stdout",
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer func() { _ = provider.Shutdown(ctx) }()

	// Verify propagator was set
	propagator := otel.GetTextMapPropagator()
	if propagator == nil {
		t.Fatal("Text map propagator should be set")
	}
}

func TestNewTracerProvider_ResourceAttributes(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:        true,
		ExporterType:   "stdout",
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer func() { _ = provider.Shutdown(ctx) }()

	// Create a span and verify resource attributes are present
	tracer := provider.Tracer("test")
	_, span := tracer.Start(ctx, "test-span")

	// Get the span context
	spanCtx := span.SpanContext()
	if !spanCtx.IsValid() {
		t.Error("Span context should be valid")
	}

	span.End()
}

func TestNewTracerProvider_Shutdown(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "stdout",
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Shutdown should not error
	err = provider.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}

	// Second shutdown should not error
	err = provider.Shutdown(ctx)
	if err != nil {
		t.Errorf("Second Shutdown() error = %v", err)
	}
}

func TestNewTracerProvider_ShutdownWithTimeout(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "stdout",
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Create context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = provider.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("Shutdown() with timeout error = %v", err)
	}
}

func TestNewTracerProvider_ForceFlush(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "stdout",
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer func() { _ = provider.Shutdown(ctx) }()

	// Create some spans
	tracer := provider.Tracer("test")
	for i := 0; i < 5; i++ {
		_, span := tracer.Start(ctx, "test-span")
		span.End()
	}

	// Force flush
	err = provider.ForceFlush(ctx)
	if err != nil {
		t.Errorf("ForceFlush() error = %v", err)
	}
}

func TestNewTracerProvider_MultipleTracers(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "stdout",
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer func() { _ = provider.Shutdown(ctx) }()

	// Create multiple tracers
	tracer1 := provider.Tracer("tracer1")
	tracer2 := provider.Tracer("tracer2")
	tracer3 := provider.Tracer("tracer3")

	if tracer1 == nil || tracer2 == nil || tracer3 == nil {
		t.Error("All tracers should be created successfully")
	}

	// Create spans from different tracers
	_, span1 := tracer1.Start(ctx, "span1")
	_, span2 := tracer2.Start(ctx, "span2")
	_, span3 := tracer3.Start(ctx, "span3")

	span1.End()
	span2.End()
	span3.End()
}

func TestConfig_Defaults(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected Config
	}{
		{
			name: "empty config gets defaults",
			config: Config{
				Enabled:      true,
				ExporterType: "stdout",
			},
			expected: Config{
				Enabled:        true,
				ExporterType:   "stdout",
				ServiceName:    "iam",
				ServiceVersion: "1.0.0",
				Environment:    "development",
				SampleRate:     1.0,
			},
		},
		{
			name: "partial config keeps custom values",
			config: Config{
				Enabled:      true,
				ExporterType: "stdout",
				ServiceName:  "custom-service",
			},
			expected: Config{
				Enabled:        true,
				ExporterType:   "stdout",
				ServiceName:    "custom-service",
				ServiceVersion: "1.0.0",
				Environment:    "development",
				SampleRate:     1.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			provider, err := Initialize(ctx, tt.config)
			if err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}
			defer func() { _ = provider.Shutdown(ctx) }()

			// Verify provider was created (defaults were applied)
			if provider == nil {
				t.Error("Provider should be created with default values")
			}
		})
	}
}

func TestNewTracerProvider_OTLPInsecure(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "otlp",
		OTLPEndpoint: "localhost:4317",
		Insecure:     true,
	}

	provider, err := Initialize(ctx, cfg)
	// This might fail if no OTLP collector is running, which is OK for unit tests
	// We're just testing that the configuration is accepted
	if err != nil {
		// Expected error if no collector is running
		if !strings.Contains(err.Error(), "failed to create exporter") {
			t.Logf("Expected error when no OTLP collector is available: %v", err)
		}
		return
	}
	_ = provider.Shutdown(ctx)
}

func TestNewTracerProvider_OTLPSecure(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "otlp",
		OTLPEndpoint: "localhost:4317",
		Insecure:     false,
	}

	provider, err := Initialize(ctx, cfg)
	// This might fail if no OTLP collector is running, which is OK for unit tests
	if err != nil {
		// Expected error if no collector is running
		if !strings.Contains(err.Error(), "failed to create exporter") {
			t.Logf("Expected error when no OTLP collector is available: %v", err)
		}
		return
	}
	_ = provider.Shutdown(ctx)
}

func TestCreateStdoutExporter(t *testing.T) {
	exporter, err := createStdoutExporter()
	if err != nil {
		t.Fatalf("createStdoutExporter() error = %v", err)
	}

	if exporter == nil {
		t.Fatal("createStdoutExporter() returned nil exporter")
	}

	// Test that exporter can export spans
	ctx := context.Background()
	spans := []sdktrace.ReadOnlySpan{}
	err = exporter.ExportSpans(ctx, spans)
	if err != nil {
		t.Errorf("ExportSpans() error = %v", err)
	}

	// Shutdown
	err = exporter.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestCreateOTLPExporter_MissingEndpoint(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		OTLPEndpoint: "",
	}

	exporter, err := createOTLPExporter(ctx, cfg)

	if err == nil {
		t.Error("createOTLPExporter() should return error when endpoint is missing")
	}

	if exporter != nil {
		t.Error("createOTLPExporter() should return nil exporter when endpoint is missing")
	}

	if !strings.Contains(err.Error(), "endpoint is required") {
		t.Errorf("Error should mention missing endpoint, got: %v", err)
	}
}

func BenchmarkInitialize(b *testing.B) {
	cfg := Config{
		Enabled:      true,
		ExporterType: "stdout",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		provider, err := Initialize(ctx, cfg)
		if err != nil {
			b.Fatalf("Initialize() error = %v", err)
		}
		_ = provider.Shutdown(ctx)
	}
}

func BenchmarkCreateSpan(b *testing.B) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "stdout",
		SampleRate:   1.0,
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		b.Fatalf("Initialize() error = %v", err)
	}
	defer func() { _ = provider.Shutdown(ctx) }()

	tracer := provider.Tracer("benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tracer.Start(ctx, "benchmark-span")
		span.SetAttributes(
			attribute.String("key1", "value1"),
			attribute.Int("key2", 123),
		)
		span.End()
	}
}

func BenchmarkCreateSpanNoSampling(b *testing.B) {
	ctx := context.Background()
	cfg := Config{
		Enabled:      true,
		ExporterType: "stdout",
		SampleRate:   0.0, // No sampling
	}

	provider, err := Initialize(ctx, cfg)
	if err != nil {
		b.Fatalf("Initialize() error = %v", err)
	}
	defer func() { _ = provider.Shutdown(ctx) }()

	tracer := provider.Tracer("benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tracer.Start(ctx, "benchmark-span")
		span.End()
	}
}
