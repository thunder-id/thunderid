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

// Package constants defines global constants used across the system module.
package constants

const (
	// LogLevelEnvironmentVariable is the environment variable name for the log level.
	LogLevelEnvironmentVariable = "LOG_LEVEL"
	// DefaultLogLevel is the default log level used if not specified.
	DefaultLogLevel = "info"
)

// AuthorizationHeaderName is the name of the authorization header used in HTTP requests.
const AuthorizationHeaderName = "Authorization"

// AcceptHeaderName is the name of the accept header used in HTTP requests.
const AcceptHeaderName = "Accept"

// ContentTypeHeaderName is the name of the content type header used in HTTP requests.
const ContentTypeHeaderName = "Content-Type"

// CorrelationIDHeaderName is the name of the correlation ID (trace ID) header used to propagate
// the request's trace ID across service boundaries.
const CorrelationIDHeaderName = "X-Correlation-ID"

// TokenTypeBearer is the token type used in bearer authentication.
const TokenTypeBearer = "Bearer"

// AuthSchemeBasic is the authentication scheme prefix used in HTTP Basic authentication.
const AuthSchemeBasic = "Basic "

// AuthSchemeBearer is the authentication scheme prefix used in HTTP Bearer authentication.
const AuthSchemeBearer = "Bearer "

// ContentTypeJSON is the content type for JSON data.
const ContentTypeJSON = "application/json"

// ContentTypeJWT is the content type for JWT data.
const ContentTypeJWT = "application/jwt"

// ContentTypeFormURLEncoded is the content type for form-urlencoded data.
const ContentTypeFormURLEncoded = "application/x-www-form-urlencoded"

// WWWAuthenticateHeaderName is the name of the WWW-Authenticate header used in HTTP responses.
const WWWAuthenticateHeaderName = "WWW-Authenticate"

// XFrameOptionsHeaderName is the name of the X-Frame-Options header used in HTTP responses.
const XFrameOptionsHeaderName = "X-Frame-Options"

// XFrameOptionsDeny is the X-Frame-Options value that prevents any framing of the page.
const XFrameOptionsDeny = "DENY"

// ContentSecurityPolicyHeaderName is the name of the Content-Security-Policy header used in HTTP responses.
const ContentSecurityPolicyHeaderName = "Content-Security-Policy"

// ContentSecurityPolicyFrameAncestorsNone is the CSP directive that prevents the page from being embedded in frames.
const ContentSecurityPolicyFrameAncestorsNone = "frame-ancestors 'none'"

// CacheControlHeaderName is the name of the cache-control header used in HTTP responses.
const CacheControlHeaderName = "Cache-Control"

// CacheControlNoCache is the cache-control directive to force revalidation.
const CacheControlNoCache = "no-cache"

// CacheControlNoStore is the cache-control directive to prevent caching.
const CacheControlNoStore = "no-store"

// CacheControlMustRevalidate is the cache-control directive to require revalidation of stale cache entries.
const CacheControlMustRevalidate = "must-revalidate"

// PragmaHeaderName is the name of the pragma header used in HTTP responses.
const PragmaHeaderName = "Pragma"

// PragmaNoCache is the pragma value to prevent caching.
const PragmaNoCache = "no-cache"

// ExpiresHeaderName is the name of the expires header used in HTTP responses.
const ExpiresHeaderName = "Expires"

// CacheControlNoCacheComposite is the combined cache-control directive to prevent caching and require revalidation.
const CacheControlNoCacheComposite = "no-cache, no-store, must-revalidate"

// ExpiresZero is the expires value to indicate immediate expiration.
const ExpiresZero = "0"

// DefaultPageSize is the default limit for pagination when not specified.
const DefaultPageSize = 30

// MaxPageSize is the maximum allowed limit for pagination.
const MaxPageSize = 100

// MaxCompositeStoreRecords is the maximum number of records that can be fetched in composite/hybrid store mode.
// This limit prevents memory exhaustion when merging results from multiple data sources (database + file-based).
// For larger datasets, use search functionality instead of list operations.
const MaxCompositeStoreRecords = 1000

// CompositeStoreLimitWarning is the message displayed when the composite store result limit is exceeded.
const CompositeStoreLimitWarning = "Result limit exceeded in hybrid mode. Use search for larger datasets."

// StoreMode represents the storage mode for resources.
type StoreMode string

// Store mode constants define how resources are persisted and retrieved.
const (
	// StoreModeMutable indicates resources are stored only in the database and can be modified via API.
	StoreModeMutable StoreMode = "mutable"
	// StoreModeDeclarative indicates resources are loaded only from declarative files (read-only).
	StoreModeDeclarative StoreMode = "declarative"
	// StoreModeComposite indicates resources are merged from both database and declarative files (hybrid mode).
	StoreModeComposite StoreMode = "composite"
)
