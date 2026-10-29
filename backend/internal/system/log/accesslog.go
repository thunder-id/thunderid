// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	sysContext "github.com/thunder-id/thunderid/internal/system/context"
)

// AccessLogHandler logs HTTP requests in Apache CLF with response time and correlation ID.
// The correlation ID should be set in the context by the CorrelationIDMiddleware.
// Paths matching skipPrefixes are served without an access log line (e.g. /console/, /gate/).
func AccessLogHandler(logger *Logger, skipPrefixes []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				next.ServeHTTP(w, r)
				return
			}
		}

		start := time.Now()

		// Extract correlation ID from context
		correlationID := sysContext.GetTraceID(r.Context())

		// Capture the status and size.
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(lrw, r)

		// Calculate elapsed time in milliseconds
		elapsedMs := time.Since(start).Milliseconds()

		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if host == "" {
			host = r.RemoteAddr
		}

		// Apache CLF-style format with correlation ID and response time
		// Format: host - - [timestamp] method uri protocol status size elapsed_ms correlation_id
		logger.Info(r.Context(), fmt.Sprintf(
			"%s - - [%s] %s %s %s %d %d %d %s",
			host,
			start.Format("02/Jan/2006:15:04:05 -0700"),
			r.Method,
			r.URL.Path,
			r.Proto,
			lrw.statusCode,
			lrw.size,
			elapsedMs,
			correlationID,
		))
	})
}

// loggingResponseWriter wraps http.ResponseWriter to capture status and size.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

// WriteHeader captures the status code and delegates to the original ResponseWriter.
func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Write captures the size of the response and delegates to the original ResponseWriter.
func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := lrw.ResponseWriter.Write(b)
	lrw.size += size
	return size, err
}

// Unwrap returns the wrapped ResponseWriter, so http.ResponseController reaches the connection, as a
// route that extends its write deadline needs.
func (lrw *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return lrw.ResponseWriter
}
