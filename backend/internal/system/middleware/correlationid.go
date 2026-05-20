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

package middleware

import (
	"net/http"

	sysContext "github.com/thunder-id/thunderid/internal/system/context"
)

// CorrelationIDMiddleware extracts or generates a correlation ID (trace ID) for each request.
// It checks the following headers in order:
// 1. X-Correlation-ID
// 2. X-Request-ID
// 3. X-Trace-ID
// If none are present, it generates a new UUID.
// The correlation ID is:
// - Stored in the request context for use by handlers
// - Added to the response headers (X-Correlation-ID)
// - Available for logging and observability throughout the request lifecycle
func CorrelationIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to extract correlation ID from common headers
		correlationID := extractCorrelationID(r)

		// Add correlation ID to request context (EnsureTraceID will generate one if empty)
		ctx := r.Context()
		if correlationID != "" {
			ctx = sysContext.WithTraceID(ctx, correlationID)
		} else {
			ctx = sysContext.EnsureTraceID(ctx)
			correlationID = sysContext.GetTraceID(ctx)
		}
		r = r.WithContext(ctx)

		// Add correlation ID to response headers so clients can track requests
		w.Header().Set("X-Correlation-ID", correlationID)

		// Continue with the next handler
		next.ServeHTTP(w, r)
	})
}

// extractCorrelationID attempts to extract a correlation ID from request headers.
// It checks multiple common header names in priority order.
func extractCorrelationID(r *http.Request) string {
	// Check common correlation ID header names in order of priority
	headers := []string{
		"X-Correlation-ID",
		"X-Request-ID",
		"X-Trace-ID",
	}

	for _, header := range headers {
		if id := r.Header.Get(header); id != "" {
			return id
		}
	}

	return ""
}
