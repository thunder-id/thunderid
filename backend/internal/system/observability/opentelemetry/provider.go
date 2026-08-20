// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package opentelemetry provides OpenTelemetry initialization and configuration.
package opentelemetry

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc/grpclog"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// Config holds OpenTelemetry configuration.
type Config struct {
	Enabled        bool    `json:"enabled"`
	ExporterType   string  `json:"exporter_type"`   // "otlp", "stdout"
	OTLPEndpoint   string  `json:"otlp_endpoint"`   // e.g., "localhost:4317"
	ServiceName    string  `json:"service_name"`    // e.g., "thunderid-iam"
	ServiceVersion string  `json:"service_version"` // e.g., "1.0.0"
	Environment    string  `json:"environment"`     // e.g., "production", "development"
	SampleRate     float64 `json:"sample_rate"`     // 0.0 to 1.0 (1.0 = sample all traces)
	Insecure       bool    `json:"insecure"`        // Set to true to disable TLS (not recommended for production)
}

// newTracerProvider creates and configures an OpenTelemetry TracerProvider.
// This is a package-private constructor. Use Initialize() instead.
// This is based on your working sample code pattern.
func newTracerProvider(ctx context.Context, cfg Config) (*sdktrace.TracerProvider, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("OpenTelemetry is disabled")
	}

	// Route OpenTelemetry's and gRPC's own diagnostics through the framework logger. This
	// runs first because the gRPC logger must be replaced before any gRPC call is made,
	// and the OTLP exporter created below is the first such caller.
	routeDiagnosticsToLogger()

	// Set defaults
	if cfg.ServiceName == "" {
		cfg.ServiceName = "thunderid-iam"
	}
	if cfg.ServiceVersion == "" {
		cfg.ServiceVersion = "1.0.0"
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 1.0 // Sample all traces by default
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on configuration
	var exporter sdktrace.SpanExporter
	switch cfg.ExporterType {
	case "otlp":
		exporter, err = createOTLPExporter(ctx, cfg)
	case "stdout":
		exporter, err = createStdoutExporter()
	default:
		return nil, fmt.Errorf("unsupported exporter type: %s (supported: otlp, stdout)", cfg.ExporterType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create sampler based on sample rate
	var sampler sdktrace.Sampler
	if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SampleRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Create tracer provider with batch span processor (like your sample)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(1*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithSampler(sampler),
	)

	// Set as global tracer provider
	otel.SetTracerProvider(tracerProvider)

	// Set up trace context propagation (like your sample)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tracerProvider, nil
}

// createOTLPExporter creates an OTLP gRPC exporter.
func createOTLPExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	if cfg.OTLPEndpoint == "" {
		return nil, fmt.Errorf("OTLP endpoint is required when using otlp exporter")
	}

	// Configure TLS based on the Insecure setting
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
	}

	if cfg.Insecure {
		// Disable TLS for development/testing
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	// If Insecure is false, the default behavior is to use TLS with system certificates

	return otlptracegrpc.New(ctx, opts...)
}

// createStdoutExporter creates a stdout exporter for testing. Spans are written as one
// JSON object per line, so they do not break a line-oriented log pipeline when the log
// format is json and both streams share stdout.
func createStdoutExporter() (sdktrace.SpanExporter, error) {
	return stdouttrace.New()
}

// loggerComponentName identifies OpenTelemetry's own diagnostics in the log records.
const loggerComponentName = "OpenTelemetry"

// grpcComponentName identifies the gRPC transport's diagnostics in the log records.
const grpcComponentName = "gRPC"

// OpenTelemetry maps its verbosity levels onto logr V-levels: warnings are logged at
// V(1), informational messages at V(4), and debug messages at V(8).
const (
	otelWarnVerbosity  = 1
	otelInfoVerbosity  = 4
	otelDebugVerbosity = 8
)

// routeDiagnosticsOnce guards the global logger installations below. gRPC's
// grpclog.SetLoggerV2 is explicitly not mutex-protected and must be called before any
// gRPC call, and OpenTelemetry only honors the first SetErrorHandler, so installing them
// exactly once is both necessary and sufficient.
var routeDiagnosticsOnce sync.Once

// routeDiagnosticsToLogger sends the internal errors and diagnostics of OpenTelemetry and
// of the gRPC transport its OTLP exporter runs over through the framework logger. All of
// them default to writing plain text to stderr, which bypasses the configured log level,
// format, and file output.
//
// The records are emitted against context.Background(): they are raised by the libraries'
// own background machinery (batch exporters, the global provider, connection management)
// rather than by a request, so there is no trace ID to attach.
func routeDiagnosticsToLogger() {
	routeDiagnosticsOnce.Do(func() {
		logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

		otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
			logger.Error(context.Background(), "OpenTelemetry reported an error", log.Error(err))
		}))
		otel.SetLogger(logr.New(&otelLogSink{logger: logger}))

		grpclog.SetLoggerV2(&grpcLogSink{
			logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, grpcComponentName)),
		})
	})
}

// otelLogSink adapts OpenTelemetry's logr-based internal logger to the framework logger.
type otelLogSink struct {
	logger *log.Logger
}

var _ logr.LogSink = (*otelLogSink)(nil)

// Init is required by logr.LogSink; the framework logger needs no runtime information.
func (s *otelLogSink) Init(logr.RuntimeInfo) {}

// Enabled gates the library's own work before it builds a record: debug-verbosity
// messages are skipped unless debug logging is on. The framework logger applies the
// configured level to everything else when the record is actually emitted.
func (s *otelLogSink) Enabled(level int) bool {
	return level < otelDebugVerbosity || s.logger.IsDebugEnabled()
}

// Info maps the logr verbosity onto the framework log levels.
func (s *otelLogSink) Info(level int, msg string, keysAndValues ...any) {
	switch {
	case level <= otelWarnVerbosity:
		s.logger.Warn(context.Background(), msg, pairsToFields(keysAndValues)...)
	case level <= otelInfoVerbosity:
		s.logger.Info(context.Background(), msg, pairsToFields(keysAndValues)...)
	default:
		s.logger.Debug(context.Background(), msg, pairsToFields(keysAndValues)...)
	}
}

// Error logs an OpenTelemetry internal error.
func (s *otelLogSink) Error(err error, msg string, keysAndValues ...any) {
	s.logger.Error(context.Background(), msg, append(pairsToFields(keysAndValues), log.Error(err))...)
}

// WithValues returns a sink whose records carry the given key/value pairs.
func (s *otelLogSink) WithValues(keysAndValues ...any) logr.LogSink {
	return &otelLogSink{logger: s.logger.With(pairsToFields(keysAndValues)...)}
}

// WithName returns a sink whose records carry the given logger name.
func (s *otelLogSink) WithName(name string) logr.LogSink {
	return &otelLogSink{logger: s.logger.With(log.String("logger", name))}
}

// pairsToFields converts logr's flat key/value pairs into log fields. A trailing key
// without a value is dropped, and a non-string key is rendered with its default format.
func pairsToFields(keysAndValues []any) []log.Field {
	fields := make([]log.Field, 0, len(keysAndValues)/2)
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", keysAndValues[i])
		}
		fields = append(fields, log.Any(key, keysAndValues[i+1]))
	}
	return fields
}

// grpcLogSink adapts gRPC's logger to the framework logger. The OTLP exporter runs over
// gRPC, whose default logger writes warnings and errors to stderr as plain text.
type grpcLogSink struct {
	logger *log.Logger
}

var _ grpclog.LoggerV2 = (*grpcLogSink)(nil)

// grpcDefaultVerbosity mirrors gRPC's own default verbosity level, so V-gated logging
// behaves as it does with the library's stock logger.
const grpcDefaultVerbosity = 0

// V reports whether the requested verbosity level is enabled.
func (s *grpcLogSink) V(level int) bool { return level <= grpcDefaultVerbosity }

// gRPC's informational logging is very chatty and its own default logger discards it
// unless GRPC_GO_LOG_SEVERITY_LEVEL asks for it, so it is mapped to debug here.
func (s *grpcLogSink) Info(args ...any)                 { s.log(s.logger.Debug, fmt.Sprint(args...)) }
func (s *grpcLogSink) Infoln(args ...any)               { s.log(s.logger.Debug, fmt.Sprintln(args...)) }
func (s *grpcLogSink) Infof(format string, args ...any) { s.logf(s.logger.Debug, format, args...) }

func (s *grpcLogSink) Warning(args ...any)   { s.log(s.logger.Warn, fmt.Sprint(args...)) }
func (s *grpcLogSink) Warningln(args ...any) { s.log(s.logger.Warn, fmt.Sprintln(args...)) }
func (s *grpcLogSink) Warningf(format string, args ...any) {
	s.logf(s.logger.Warn, format, args...)
}

func (s *grpcLogSink) Error(args ...any)                 { s.log(s.logger.Error, fmt.Sprint(args...)) }
func (s *grpcLogSink) Errorln(args ...any)               { s.log(s.logger.Error, fmt.Sprintln(args...)) }
func (s *grpcLogSink) Errorf(format string, args ...any) { s.logf(s.logger.Error, format, args...) }

// gRPC requires that a Fatal log exits the process, which log.Logger.Fatal does.
func (s *grpcLogSink) Fatal(args ...any)                 { s.log(s.logger.Fatal, fmt.Sprint(args...)) }
func (s *grpcLogSink) Fatalln(args ...any)               { s.log(s.logger.Fatal, fmt.Sprintln(args...)) }
func (s *grpcLogSink) Fatalf(format string, args ...any) { s.logf(s.logger.Fatal, format, args...) }

// log emits msg through the given level, trimming the newline the Sprintln-style
// variants append.
func (s *grpcLogSink) log(emit func(context.Context, string, ...log.Field), msg string) {
	emit(context.Background(), strings.TrimSpace(msg))
}

// logf formats the message before emitting it through the given level.
func (s *grpcLogSink) logf(emit func(context.Context, string, ...log.Field), format string, args ...any) {
	s.log(emit, fmt.Sprintf(format, args...))
}
