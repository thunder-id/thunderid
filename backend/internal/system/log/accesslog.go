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

package log

import (
	"fmt"
	"net"
	"net/http"
	"time"

	sysContext "github.com/thunder-id/thunderid/internal/system/context"
)

// AccessLogHandler logs HTTP requests in Apache CLF with response time and correlation ID.
// The correlation ID should be set in the context by the CorrelationIDMiddleware.
func AccessLogHandler(logger *Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		logger.Info(fmt.Sprintf(
			"%s - - [%s] %s %s %s %d %d %d %s",
			host,
			start.Format("02/Jan/2006:15:04:05 -0700"),
			r.Method,
			r.RequestURI,
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
