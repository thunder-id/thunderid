// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package constants defines global constants used across the system module.
package constants

// DefaultLogLevel is the default log level used if not specified.
const DefaultLogLevel = "info"

// AuthorizationHeaderName is the name of the authorization header used in HTTP requests.
const AuthorizationHeaderName = "Authorization"

// CookieHeaderName is the name of the cookie header used in HTTP requests.
const CookieHeaderName = "Cookie"

// SetCookieHeaderName is the name of the set-cookie header used in HTTP responses.
const SetCookieHeaderName = "Set-Cookie"

// ProxyAuthorizationHeaderName is the name of the proxy-authorization header used in HTTP requests.
const ProxyAuthorizationHeaderName = "Proxy-Authorization"

// AcceptHeaderName is the name of the accept header used in HTTP requests.
const AcceptHeaderName = "Accept"

// ContentTypeHeaderName is the name of the content type header used in HTTP requests.
const ContentTypeHeaderName = "Content-Type"

// CorrelationIDHeaderName is the name of the correlation ID (trace ID) header used to propagate
// the request's trace ID across service boundaries.
const CorrelationIDHeaderName = "X-Correlation-ID"

// FlowSecretHeaderName is the name of the header used to present a Flow Secret when initiating
// a flow directly over HTTP.
const FlowSecretHeaderName = "Flow-Secret"

// AttestationTokenHeaderName is the name of the header used to present a platform attestation token
// (e.g. a Google Play Integrity token) when a mobile application initiates a flow directly over HTTP.
const AttestationTokenHeaderName = "Attestation-Token"

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

// ContentSecurityPolicyReportOnlyHeaderName is the name of the Content-Security-Policy-Report-Only header.
const ContentSecurityPolicyReportOnlyHeaderName = "Content-Security-Policy-Report-Only"

// ContentTypeHTML is the content type for HTML documents.
const ContentTypeHTML = "text/html; charset=utf-8"

// CSPNoncePlaceholder is the token in index.html that serveIndex replaces with the per-request CSP
// nonce, so the frontend's own inline <style> tags can read it from the resulting meta tag.
const CSPNoncePlaceholder = "__CSP_NONCE__"

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

// SCIMContentType is the SCIM-specific content type required by RFC 7644.
const SCIMContentType = "application/scim+json"
