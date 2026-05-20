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

// Package middleware provides HTTP middleware functions for request processing.
package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// DefaultAllowedHeaders is the default Access-Control-Allow-Headers list used
// when a CORSOptions value leaves AllowedHeaders empty. Almost every route in
// the project converges on Content-Type + Authorization, so this default
// removes per-route boilerplate and keeps the headers consistent across the
// API. Routes that need a different list pass an explicit AllowedHeaders.
var DefaultAllowedHeaders = []string{"Content-Type", "Authorization"}

// CORSOptions represents the per-route CORS response configuration. Allowed
// origins are global (server-level deployment configuration); methods,
// headers, credentials, and max-age are per-route because each route has its
// own method surface and caching profile.
//
// AllowedMethods and AllowedHeaders are slices so the response payload is
// data-driven rather than a parsed string. MaxAge is the preflight cache TTL
// in seconds; zero suppresses the Access-Control-Max-Age header. The
// per-request Origin echo is decided by the global matcher and never
// influenced by these options.
type CORSOptions struct {
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// WithCORS wraps an HTTP handler with CORS handling: origin validation,
// Vary: Origin emission, and preflight-only headers gated to OPTIONS+ACRM
// requests. The pattern is returned unchanged so the route registration site
// keeps the (pattern, handler) shape that http.ServeMux.HandleFunc expects.
func WithCORS(pattern string, handler http.HandlerFunc, opts CORSOptions) (string, http.HandlerFunc) {
	return pattern, func(w http.ResponseWriter, r *http.Request) {
		applyCORSHeaders(w, r, opts)
		handler(w, r)
	}
}

// applyCORSHeaders sets the CORS response headers when the request carries a
// valid Origin header that matches a configured allowed-origins entry.
//
// Behavior:
//   - Multiple Origin headers are refused without evaluation: the Fetch spec
//     sends exactly one, and a duplicate is a smuggling/relay signal.
//   - Vary: Origin is appended whenever the request carries an Origin header,
//     including on the deny path, so a shared cache cannot serve a response
//     produced for one origin to a different origin.
//   - The matcher is read once per request from the cors package singleton
//     installed at boot; no regex compilation runs on the hot path.
//   - Allow-Methods, Allow-Headers, and Max-Age are preflight-only response
//     headers per the Fetch spec; we emit them only on OPTIONS requests that
//     also carry Access-Control-Request-Method.
func applyCORSHeaders(w http.ResponseWriter, r *http.Request, opts CORSOptions) {
	origins := r.Header.Values("Origin")
	if len(origins) == 0 {
		return
	}
	if len(origins) > 1 {
		logger().Debug("CORS: multiple Origin headers; refusing to evaluate")
		w.Header().Add("Vary", "Origin")
		return
	}
	requestOrigin := strings.TrimSpace(origins[0])
	if requestOrigin == "" {
		return
	}

	// Vary: Origin must be set on every Origin-bearing response (allow and
	// deny) so caches key the response by Origin.
	w.Header().Add("Vary", "Origin")

	matcher := cors.GetMatcher()
	if matcher == nil {
		return
	}

	parsed, err := cors.ParseOrigin(requestOrigin)
	if err != nil {
		logger().Debug("CORS origin rejected by parser",
			log.String("origin", requestOrigin), log.Error(err))
		return
	}

	allow, echo := matcher.Match(parsed)
	if !allow {
		logger().Debug("CORS origin rejected by matcher",
			log.String("origin", requestOrigin))
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", echo)
	if opts.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if !isPreflight(r) {
		return
	}
	if methods := joinHeaderList(opts.AllowedMethods); methods != "" {
		w.Header().Set("Access-Control-Allow-Methods", methods)
	}
	headers := joinHeaderList(opts.AllowedHeaders)
	if headers == "" {
		headers = joinHeaderList(DefaultAllowedHeaders)
	}
	if headers != "" {
		w.Header().Set("Access-Control-Allow-Headers", headers)
	}
	if opts.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(opts.MaxAge))
	}
}

// isPreflight reports whether r is a CORS preflight request. A preflight is
// an OPTIONS request carrying Access-Control-Request-Method; a bare OPTIONS
// (e.g. resource discovery) is not.
func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions &&
		r.Header.Get("Access-Control-Request-Method") != ""
}

// joinHeaderList renders a slice as a comma-separated header value, returning
// the empty string for nil/empty slices so callers can use the result
// directly as a "skip if empty" signal.
func joinHeaderList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, ", ")
}

// logger returns the package-scoped logger with a CORSMiddleware component tag.
func logger() *log.Logger {
	return log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CORSMiddleware"))
}
