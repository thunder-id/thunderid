/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package log provides a structured wrapper around the log package.
package log

import (
	"context"
	"errors"
	"io"
	stdlog "log"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/thunder-id/thunderid/internal/system/constants"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log/rollingfile"
)

var (
	logger *Logger
	once   sync.Once
)

// Logger is a wrapper around the slog logger.
type Logger struct {
	internal   *slog.Logger
	levelVar   *slog.LevelVar
	fileWriter *rollingfile.Writer
}

// OutputOptions describes where and how the logger writes. It is a log-package
// local type (rather than config.LogConfig) so this package does not depend on
// the config package, which already depends on it.
type OutputOptions struct {
	// ConsoleEnabled writes formatted records to stdout.
	ConsoleEnabled bool
	// FileEnabled writes formatted records to a rotating file.
	FileEnabled bool
	// Format selects the record format: "json" or "text" (default).
	Format string
	// File configures the rotating file writer (its Path must be resolved to an
	// absolute path by the caller). Ignored when FileEnabled is false.
	File rollingfile.Config
}

// contextHandler decorates a slog.Handler to add the trace ID (correlation ID)
// from the context to every log record, when present. The trace ID is set in
// the request context by the CorrelationIDMiddleware.
type contextHandler struct {
	slog.Handler
}

// Handle adds the trace ID from the context to the record before delegating
// to the wrapped handler. The trace ID is only added when it is actually
// present in the context; sysContext.GetTraceID is not used here as it
// generates a new ID when absent, which would stamp unrelated log records
// with distinct, misleading trace IDs.
func (h *contextHandler) Handle(ctx context.Context, record slog.Record) error {
	if ctx != nil {
		if traceID, ok := ctx.Value(sysContext.TraceIDKey).(string); ok && traceID != "" {
			record.AddAttrs(slog.String(LoggerKeyTraceID, traceID))
		}
	}
	return h.Handler.Handle(ctx, record)
}

// WithAttrs preserves the context decoration on loggers derived via With.
func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup preserves the context decoration on loggers derived via WithGroup.
func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithGroup(name)}
}

// GetLogger creates and returns a singleton instance of the logger.
func GetLogger() *Logger {
	once.Do(func() {
		err := initLogger()
		if err != nil {
			panic("Failed to initialize logger: " + err.Error())
		}
	})
	return logger
}

// initLogger initializes the slog logger.
func initLogger() error {
	// The logger is initialized before the deployment configuration is loaded, so it
	// boots at the default level. The configured level from deployment.yaml is applied
	// afterwards via SetLevel.
	level, err := parseLogLevel(constants.DefaultLogLevel)
	if err != nil {
		return errors.New("error parsing log level: " + err.Error())
	}

	levelVar := new(slog.LevelVar)
	levelVar.Set(level)

	handlerOptions := &slog.HandlerOptions{
		Level: levelVar,
	}

	logHandler := slog.NewTextHandler(os.Stdout, handlerOptions)
	if logHandler == nil {
		return errors.New("failed to create log handler")
	}

	logger = &Logger{
		internal: slog.New(&contextHandler{Handler: logHandler}),
		levelVar: levelVar,
	}

	return nil
}

// SetLevel updates the minimum log level at runtime.
func (l *Logger) SetLevel(logLevel string) error {
	level, err := parseLogLevel(logLevel)
	if err != nil {
		return err
	}
	l.levelVar.Set(level)
	return nil
}

// Configure applies the output configuration, rebuilding the underlying slog
// handler to write to the console, a rotating file, or both. It preserves the
// shared level variable so a prior SetLevel keeps taking effect, and keeps the
// trace ID decoration via contextHandler. It is intended to be called once during
// startup, right after the configured level is applied.
func (l *Logger) Configure(opts OutputOptions) error {
	writers := make([]io.Writer, 0, 2)
	if opts.ConsoleEnabled {
		writers = append(writers, os.Stdout)
	}

	var fileWriter *rollingfile.Writer
	if opts.FileEnabled {
		w, err := rollingfile.New(opts.File)
		if err != nil {
			return err
		}
		fileWriter = w
		writers = append(writers, w)
	}

	// Fall back to stdout so a misconfiguration never silences the logger.
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	var out io.Writer
	if len(writers) == 1 {
		out = writers[0]
	} else {
		out = io.MultiWriter(writers...)
	}

	handlerOptions := &slog.HandlerOptions{Level: l.levelVar}
	var handler slog.Handler
	if strings.EqualFold(opts.Format, "json") {
		handler = slog.NewJSONHandler(out, handlerOptions)
	} else {
		handler = slog.NewTextHandler(out, handlerOptions)
	}

	previous := l.fileWriter
	l.internal = slog.New(&contextHandler{Handler: handler})
	l.fileWriter = fileWriter
	if previous != nil {
		_ = previous.Close()
	}
	return nil
}

// Close releases the file writer, if any. It should be called during shutdown.
func (l *Logger) Close() error {
	if l.fileWriter != nil {
		return l.fileWriter.Close()
	}
	return nil
}

// With creates a new logger instance with additional fields.
func (l *Logger) With(fields ...Field) *Logger {
	return &Logger{
		internal: l.internal.With(convertFields(fields)...),
		levelVar: l.levelVar,
	}
}

// WithTraceID creates a new logger instance with the trace ID (correlation ID) field.
// This is a convenience method to add the trace ID to all log entries.
func (l *Logger) WithTraceID(traceID string) *Logger {
	return l.With(String(LoggerKeyTraceID, traceID))
}

// WithContext creates a new logger instance with fields extracted from the context.
// Currently extracts the trace ID (correlation ID) if present in the context.
// This is the recommended way to create a logger in HTTP handlers and other
// request-scoped code where a context is available.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	traceID := sysContext.GetTraceID(ctx)
	return l.WithTraceID(traceID)
}

// IsDebugEnabled checks if the logger is set to debug level.
func (l *Logger) IsDebugEnabled() bool {
	return l.internal.Handler().Enabled(context.Background(), slog.LevelDebug)
}

// Info logs an informational message with custom fields, automatically
// including the trace ID (correlation ID) from the context if present.
func (l *Logger) Info(ctx context.Context, msg string, fields ...Field) {
	l.internal.InfoContext(ctx, msg, convertFields(fields)...)
}

// Debug logs a debug message with custom fields, automatically
// including the trace ID (correlation ID) from the context if present.
func (l *Logger) Debug(ctx context.Context, msg string, fields ...Field) {
	l.internal.DebugContext(ctx, msg, convertFields(fields)...)
}

// Warn logs a warning message with custom fields, automatically
// including the trace ID (correlation ID) from the context if present.
func (l *Logger) Warn(ctx context.Context, msg string, fields ...Field) {
	l.internal.WarnContext(ctx, msg, convertFields(fields)...)
}

// Error logs an error message with custom fields, automatically
// including the trace ID (correlation ID) from the context if present.
func (l *Logger) Error(ctx context.Context, msg string, fields ...Field) {
	l.internal.ErrorContext(ctx, msg, convertFields(fields)...)
}

// Fatal logs a fatal message with custom fields and exits the application,
// automatically including the trace ID (correlation ID) from the context if present.
func (l *Logger) Fatal(ctx context.Context, msg string, fields ...Field) {
	l.internal.ErrorContext(ctx, msg, convertFields(fields)...)
	os.Exit(1)
}

// serverErrorWriter adapts the standard library logger output used by
// http.Server.ErrorLog into the framework logger. Connection-level errors
// such as TLS handshake failures are emitted at WARN level so they are
// routed through the structured logger instead of being written raw to stderr.
type serverErrorWriter struct {
	logger *Logger
}

// Write forwards each http.Server error line to the framework logger at WARN level.
func (w *serverErrorWriter) Write(p []byte) (int, error) {
	w.logger.Warn(context.Background(), strings.TrimSpace(string(p)))
	return len(p), nil
}

// NewServerErrorLog returns a standard library *log.Logger suitable for
// http.Server.ErrorLog that routes server connection errors (e.g. TLS
// handshake errors) through the framework logger at WARN level.
func NewServerErrorLog(logger *Logger) *stdlog.Logger {
	return stdlog.New(&serverErrorWriter{logger: logger}, "", 0)
}

// parseLogLevel parses the log level string and returns the corresponding slog.Level.
func parseLogLevel(logLevel string) (slog.Level, error) {
	var level slog.Level
	var err = level.UnmarshalText([]byte(logLevel))
	if err != nil {
		return slog.LevelError, err
	}
	return level, nil
}

// convertFields converts a slice of Field to a variadic list of slog.Attr.
func convertFields(fields []Field) []any {
	attrs := make([]any, len(fields))
	for i, field := range fields {
		attrs[i] = slog.Any(field.Key, field.Value)
	}
	return attrs
}

// maskString masks characters in a string except for the first and last characters.
func maskString(s string) string {
	if len(s) <= 3 {
		return strings.Repeat("*", len(s))
	}
	return s[:1] + strings.Repeat("*", len(s)-2) + s[len(s)-1:]
}
