// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package opentelemetry

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Initialize creates and configures a TracerProvider based on the provided configuration,
// sets it as the global tracer provider, and configures trace context propagation.
//
// Parameters:
//   - ctx: Context for initialization (used for exporter setup)
//   - cfg: OpenTelemetry configuration including exporter type, endpoint, service details, etc.
//
// Returns:
//   - *sdktrace.TracerProvider: The initialized tracer provider
//   - error: Error if initialization fails (e.g., invalid config, exporter creation failure)
//
// Example:
//
//	cfg := opentelemetry.Config{
//	    Enabled:      true,
//	    ExporterType: "otlp",
//	    OTLPEndpoint: "localhost:4317",
//	    ServiceName:  "acme-iam",
//	}
//	provider, err := opentelemetry.Initialize(ctx, cfg)
//	if err != nil {
//	    return err
//	}
//	defer provider.Shutdown(ctx)
func Initialize(ctx context.Context, cfg Config) (*sdktrace.TracerProvider, error) {
	return newTracerProvider(ctx, cfg)
}
