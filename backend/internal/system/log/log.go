// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	"sync/atomic"

	"github.com/thunder-id/thunderid/internal/system/constants"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log/rollingfile"
)

var (
	logger *Logger
	once   sync.Once
)

// Log record formats accepted by Configure and SetFormat. Any other value is treated
// as formatText.
const (
	formatText = "text"
	formatJSON = "json"
)

// Logger is a wrapper around the slog logger.
type Logger struct {
	internal *slog.Logger
	root     *root
}

// root holds the process-wide logging state shared by the singleton logger and every
// logger derived from it. Configure swaps the state atomically, so loggers derived
// before Configure ran also write through the newly configured handler.
type root struct {
	state    atomic.Pointer[rootState]
	levelVar *slog.LevelVar

	// mu serializes Configure, SetFormat and Close, and guards the fields below.
	mu         sync.Mutex
	fileWriter *rollingfile.Writer
	// out is the sink the current handler writes to, and format is the format it was
	// built with. SetFormat rebuilds the handler from them so it can change the format
	// without discarding the configured output.
	out    io.Writer
	format string
}

// rootState is an immutable snapshot of the configured output handler. A new value is
// allocated on every Configure, so derived handlers can detect that their cached
// handler chain is stale by comparing pointers.
type rootState struct {
	handler slog.Handler
}

// derivation records a single WithAttrs or WithGroup call so it can be replayed, in
// order, onto a swapped root handler.
type derivation struct {
	// group is the group name for a WithGroup call, and empty for WithAttrs. slog
	// treats WithGroup("") as a no-op, so an empty name is never recorded and the
	// field is an unambiguous discriminator.
	group string
	attrs []slog.Attr
}

// resolvedHandler caches a replayed handler chain together with the root state it was
// built from.
type resolvedHandler struct {
	from    *rootState
	handler slog.Handler
}

// dynamicHandler is the handler installed on every Logger. It owns no output handler of
// its own: it resolves the root's current handler and replays the recorded derivations
// onto it. The result is cached until Configure installs a new root state, so the
// steady-state path is two atomic loads and no allocation.
type dynamicHandler struct {
	// derivations is immutable once the handler is constructed.
	derivations []derivation
	root        *root
	cache       atomic.Pointer[resolvedHandler]
}

var _ slog.Handler = (*dynamicHandler)(nil)

// resolve returns the handler for the current root state, rebuilding and caching the
// derivation chain when the state has been swapped since the last call.
func (h *dynamicHandler) resolve() slog.Handler {
	state := h.root.state.Load()
	if cached := h.cache.Load(); cached != nil && cached.from == state {
		return cached.handler
	}

	handler := state.handler
	for _, d := range h.derivations {
		if d.group != "" {
			handler = handler.WithGroup(d.group)
			continue
		}
		handler = handler.WithAttrs(d.attrs)
	}
	// Concurrent resolvers build equivalent chains, so the last store wins.
	h.cache.Store(&resolvedHandler{from: state, handler: handler})
	return handler
}

// Enabled reports whether the current handler handles records at the given level.
func (h *dynamicHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.resolve().Enabled(ctx, level)
}

// Handle delegates the record to the current handler.
func (h *dynamicHandler) Handle(ctx context.Context, record slog.Record) error {
	return h.resolve().Handle(ctx, record)
}

// WithAttrs records the attributes so they are replayed on the current handler.
func (h *dynamicHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return h.derive(derivation{attrs: attrs})
}

// WithGroup records the group so it is replayed on the current handler.
func (h *dynamicHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return h.derive(derivation{group: name})
}

// derive appends d to the recorded derivations. The slice is copied rather than
// appended in place so sibling handlers derived from the same parent never share a
// backing array and overwrite each other's last derivation.
func (h *dynamicHandler) derive(d derivation) *dynamicHandler {
	derivations := make([]derivation, len(h.derivations), len(h.derivations)+1)
	copy(derivations, h.derivations)
	return &dynamicHandler{derivations: append(derivations, d), root: h.root}
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

// newFormatHandler builds the output handler for the given format. It is the single
// place that maps a format name onto an slog handler, so Configure and SetFormat cannot
// drift apart.
func newFormatHandler(format string, out io.Writer, levelVar *slog.LevelVar) slog.Handler {
	handlerOptions := &slog.HandlerOptions{Level: levelVar}
	if strings.EqualFold(format, formatJSON) {
		return slog.NewJSONHandler(out, handlerOptions)
	}
	return slog.NewTextHandler(out, handlerOptions)
}

// newLogger wires a Logger onto base, which writes to out in format. Loggers derived
// from the result follow whatever handler Configure installs later.
func newLogger(base slog.Handler, levelVar *slog.LevelVar, out io.Writer, format string) *Logger {
	r := &root{levelVar: levelVar, out: out, format: format}
	r.state.Store(&rootState{handler: base})
	return &Logger{internal: slog.New(&dynamicHandler{root: r}), root: r}
}

// initLogger initializes the slog logger.
func initLogger() error {
	// The logger is initialized before the deployment configuration is loaded, so it
	// boots at the default level and format. The configured values from deployment.yaml
	// are applied afterwards via SetLevel and Configure.
	level, err := parseLogLevel(constants.DefaultLogLevel)
	if err != nil {
		return errors.New("error parsing log level: " + err.Error())
	}

	levelVar := new(slog.LevelVar)
	levelVar.Set(level)

	logHandler := newFormatHandler(formatText, os.Stdout, levelVar)
	logger = newLogger(&contextHandler{Handler: logHandler}, levelVar, os.Stdout, formatText)

	return nil
}

// SetLevel updates the minimum log level at runtime.
func (l *Logger) SetLevel(logLevel string) error {
	level, err := parseLogLevel(logLevel)
	if err != nil {
		return err
	}
	l.root.levelVar.Set(level)
	return nil
}

// Configure applies the output configuration, rebuilding the underlying slog
// handler to write to the console, a rotating file, or both. It preserves the
// shared level variable so a prior SetLevel keeps taking effect, and keeps the
// trace ID decoration via contextHandler. Because the handler lives on the shared
// root, loggers derived before Configure ran also pick up the new output, so the
// order of Configure relative to service construction does not matter.
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

	handler := newFormatHandler(opts.Format, out, l.root.levelVar)

	l.root.mu.Lock()
	defer l.root.mu.Unlock()

	previous := l.root.fileWriter
	l.root.state.Store(&rootState{handler: &contextHandler{Handler: handler}})
	l.root.fileWriter = fileWriter
	l.root.out = out
	l.root.format = opts.Format
	if previous != nil {
		_ = previous.Close()
	}
	return nil
}

// SetFormat changes the record format without touching where the records go. It exists
// for callers that own the format but not the output configuration, such as an embedded
// engine running inside a host application that has already configured the file sink:
// calling Configure there would replace the host's output with a console-only one and
// close its file writer.
func (l *Logger) SetFormat(format string) error {
	if !strings.EqualFold(format, formatText) && !strings.EqualFold(format, formatJSON) {
		return errors.New("unsupported log format: " + format)
	}

	l.root.mu.Lock()
	defer l.root.mu.Unlock()

	if strings.EqualFold(format, l.root.format) {
		return nil
	}

	handler := newFormatHandler(format, l.root.out, l.root.levelVar)
	l.root.state.Store(&rootState{handler: &contextHandler{Handler: handler}})
	l.root.format = format
	return nil
}

// Close releases the file writer, if any. It should be called during shutdown.
// The writer lives on the shared root, so closing any derived logger closes the
// single process-wide file sink. Close is idempotent.
func (l *Logger) Close() error {
	l.root.mu.Lock()
	defer l.root.mu.Unlock()

	if l.root.fileWriter == nil {
		return nil
	}
	err := l.root.fileWriter.Close()
	l.root.fileWriter = nil
	return err
}

// With creates a new logger instance with additional fields. The returned logger shares
// the root state, so it follows any later Configure or SetFormat call.
func (l *Logger) With(fields ...Field) *Logger {
	return &Logger{
		internal: l.internal.With(convertFields(fields)...),
		root:     l.root,
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
