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
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/thunder-id/thunderid/internal/system/constants"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
)

var (
	logger *Logger
	once   sync.Once
)

// Logger is a wrapper around the slog logger.
type Logger struct {
	internal *slog.Logger
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
	// Read log level from the environment variable.
	logLevel := os.Getenv(constants.LogLevelEnvironmentVariable)
	if logLevel == "" {
		logLevel = constants.DefaultLogLevel
	}
	// Parse the log level.
	level, err := parseLogLevel(logLevel)
	if err != nil {
		return errors.New("error parsing log level: " + err.Error())
	}

	handlerOptions := &slog.HandlerOptions{
		Level: level,
	}

	logHandler := slog.NewTextHandler(os.Stdout, handlerOptions)
	if logHandler == nil {
		return errors.New("failed to create log handler")
	}

	logger = &Logger{
		internal: slog.New(logHandler),
	}

	return nil
}

// With creates a new logger instance with additional fields.
func (l *Logger) With(fields ...Field) *Logger {
	return &Logger{
		internal: l.internal.With(convertFields(fields)...),
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

// Info logs an informational message with custom fields.
func (l *Logger) Info(msg string, fields ...Field) {
	l.internal.Info(msg, convertFields(fields)...)
}

// Debug logs a debug message with custom fields.
func (l *Logger) Debug(msg string, fields ...Field) {
	l.internal.Debug(msg, convertFields(fields)...)
}

// Warn logs a warning message with custom fields.
func (l *Logger) Warn(msg string, fields ...Field) {
	l.internal.Warn(msg, convertFields(fields)...)
}

// Error logs an error message with custom fields.
func (l *Logger) Error(msg string, fields ...Field) {
	l.internal.Error(msg, convertFields(fields)...)
}

// Fatal logs a fatal message with custom fields and exits the application.
func (l *Logger) Fatal(msg string, fields ...Field) {
	l.internal.Error(msg, convertFields(fields)...)
	os.Exit(1)
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
