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
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	yaml "gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cors"
)

type CORSMiddlewareTestSuite struct {
	suite.Suite
}

func TestCORSMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(CORSMiddlewareTestSuite))
}

// initRuntime parses the given YAML allowed-origins document, installs a
// fresh CORS matcher from it, and seeds the runtime config singleton. Pass
// the empty string to install an empty matcher. SetupTest calls this with
// the default doc; tests that need different origins call ResetServerRuntime
// + initRuntime directly. cors.InitializeMatcher is invoked explicitly here
// because production wires it from the server bootstrap; tests that bypass
// LoadConfig + main own that step themselves.
func (suite *CORSMiddlewareTestSuite) initRuntime(allowedOriginsYAML string) {
	var entries cors.OriginEntries
	if allowedOriginsYAML != "" {
		suite.Require().NoError(yaml.Unmarshal([]byte(allowedOriginsYAML), &entries))
	}
	cfg := &config.Config{CORS: config.CORSConfig{AllowedOrigins: entries}}
	suite.Require().NoError(cors.InitializeMatcher(cfg.CORS.AllowedOrigins))
	suite.Require().NoError(config.InitializeServerRuntime("/tmp", cfg))
}

func (suite *CORSMiddlewareTestSuite) SetupTest() {
	suite.initRuntime(`
- https://example.com
- https://test.com
- regex: ^https://[a-z0-9-]+\.staging\.example\.com$
`)
}

func (suite *CORSMiddlewareTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// newGetRequest returns a (GET request, recorder) pair primed with the given
// Origin header (or no Origin if origin == ""). Most tests only need a simple
// GET to exercise the non-preflight path; preflight tests use preflightRequest.
// The target path is fixed because the middleware is path-agnostic — the wrap
// pattern just gets passed through unchanged.
func newGetRequest(origin string) (*http.Request, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return req, httptest.NewRecorder()
}

// preflightRequest builds a CORS preflight (OPTIONS + Access-Control-Request-Method).
func preflightRequest(target, origin, acrm string) (*http.Request, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodOptions, target, nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", acrm)
	return req, httptest.NewRecorder()
}

// noopHandler is the dummy handler used by tests that only care about CORS
// response headers.
func noopHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// fullOpts is the typical CORSOptions value used by most tests.
var fullOpts = CORSOptions{
	AllowedMethods:   []string{"GET", "POST"},
	AllowedHeaders:   DefaultAllowedHeaders,
	AllowCredentials: true,
	MaxAge:           600,
}

// --- Existing behavior, updated for preflight gating. ---------------------

func (suite *CORSMiddlewareTestSuite) TestWithCORS_ValidOrigin() {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}
	pattern, wrapped := WithCORS("GET /test", handler, fullOpts)

	assert.Equal(suite.T(), "GET /test", pattern)

	req, w := newGetRequest("https://example.com")
	wrapped(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(suite.T(), "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(suite.T(), "Origin", w.Header().Get("Vary"))
	// Allow-Methods/Allow-Headers/Max-Age are preflight-only per the Fetch
	// spec; a simple GET must not carry them.
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Methods"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Headers"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Max-Age"))
	assert.Equal(suite.T(), "OK", w.Body.String())
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_EchoesRawHeader() {
	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	// Case-mixed scheme/host canonicalizes to the bare-host form, so the
	// matcher accepts it; the response must echo the raw inbound header
	// rather than the canonicalized rule key.
	req, w := newGetRequest("HTTPS://Example.COM")
	wrapped(w, req)

	assert.Equal(suite.T(), "HTTPS://Example.COM", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(suite.T(), "Origin", w.Header().Get("Vary"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_RegexOrigin() {
	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req, w := newGetRequest("https://tenant-1.staging.example.com")
	wrapped(w, req)

	assert.Equal(suite.T(), "https://tenant-1.staging.example.com",
		w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(suite.T(), "Origin", w.Header().Get("Vary"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_InvalidOrigin() {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}
	_, wrapped := WithCORS("GET /test", handler, fullOpts)

	req, w := newGetRequest("https://malicious.com")
	wrapped(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Methods"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Headers"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Credentials"))
	// Vary: Origin is set on the deny path too — a shared cache must not
	// serve a denied-for-A response back to allowed-B.
	assert.Equal(suite.T(), "Origin", w.Header().Get("Vary"))
	assert.Equal(suite.T(), "OK", w.Body.String())
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_SubstringPretenderRejected() {
	// The HEAD implementation used strings.Contains, so a request origin that
	// embedded an allowed origin as a substring (e.g. via a path-like suffix)
	// would have been accepted. Origin-equality matching must reject it.
	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req, w := newGetRequest("https://attacker.com/https://example.com")
	wrapped(w, req)

	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(suite.T(), "Origin", w.Header().Get("Vary"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_UnparseableOrigin() {
	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req, w := newGetRequest("javascript://example.com")
	wrapped(w, req)

	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Origin"))
	// Vary is set before parsing, so the deny path still contributes to
	// cache keying.
	assert.Equal(suite.T(), "Origin", w.Header().Get("Vary"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_NoOriginHeader() {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}
	_, wrapped := WithCORS("GET /test", handler, fullOpts)

	req, w := newGetRequest("")
	wrapped(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Origin"))
	// No Vary: Origin when the request lacks Origin altogether — there's
	// nothing for a cache to key on.
	assert.Empty(suite.T(), w.Header().Get("Vary"))
	assert.Equal(suite.T(), "OK", w.Body.String())
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_WithoutCredentials() {
	opts := CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           600,
	}
	_, wrapped := WithCORS("GET /test", noopHandler, opts)

	req, w := newGetRequest("https://example.com")
	wrapped(w, req)

	assert.Equal(suite.T(), "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Credentials"))
	// Simple GET: no preflight headers.
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Methods"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Headers"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Max-Age"))
	assert.Equal(suite.T(), "Origin", w.Header().Get("Vary"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_EmptyOptions() {
	_, wrapped := WithCORS("GET /test", noopHandler, CORSOptions{})

	req, w := newGetRequest("https://example.com")
	wrapped(w, req)

	assert.Equal(suite.T(), "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Methods"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Headers"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(suite.T(), "Origin", w.Header().Get("Vary"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_MultipleAllowedOrigins() {
	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req1, w1 := newGetRequest("https://example.com")
	wrapped(w1, req1)
	assert.Equal(suite.T(), "https://example.com", w1.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(suite.T(), "Origin", w1.Header().Get("Vary"))

	req2, w2 := newGetRequest("https://test.com")
	wrapped(w2, req2)
	assert.Equal(suite.T(), "https://test.com", w2.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(suite.T(), "Origin", w2.Header().Get("Vary"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_NoOriginsConfigured() {
	config.ResetServerRuntime()
	suite.initRuntime("")

	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req, w := newGetRequest("https://example.com")
	wrapped(w, req)

	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Origin"))
	// Empty config still fails closed via a real (empty) matcher; Vary is
	// set so caches don't pool an allow response with this deny one.
	assert.Equal(suite.T(), "Origin", w.Header().Get("Vary"))
}

// --- Preflight gating. ----------------------------------------------------

func (suite *CORSMiddlewareTestSuite) TestWithCORS_PreflightEmitsAllHeaders() {
	_, wrapped := WithCORS("OPTIONS /test", noopHandler, fullOpts)

	req, w := preflightRequest("/test", "https://example.com", "POST")
	wrapped(w, req)

	assert.Equal(suite.T(), "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(suite.T(), "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(suite.T(), "GET, POST", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(suite.T(), "Content-Type, Authorization", w.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(suite.T(), "600", w.Header().Get("Access-Control-Max-Age"))
	assert.Equal(suite.T(), "Origin", w.Header().Get("Vary"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_PreflightDefaultsHeaders() {
	// When AllowedHeaders is left unset, the middleware falls back to
	// DefaultAllowedHeaders so routes don't have to repeat the same list.
	opts := CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   nil,
		AllowCredentials: true,
		MaxAge:           600,
	}
	_, wrapped := WithCORS("OPTIONS /test", noopHandler, opts)

	req, w := preflightRequest("/test", "https://example.com", "GET")
	wrapped(w, req)

	assert.Equal(suite.T(), "Content-Type, Authorization",
		w.Header().Get("Access-Control-Allow-Headers"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_PreflightZeroMaxAgeOmitted() {
	opts := CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           0,
	}
	_, wrapped := WithCORS("OPTIONS /test", noopHandler, opts)

	req, w := preflightRequest("/test", "https://example.com", "GET")
	wrapped(w, req)

	assert.Empty(suite.T(), w.Header().Get("Access-Control-Max-Age"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_BareOptionsNotPreflight() {
	// OPTIONS without Access-Control-Request-Method is not a preflight per
	// the Fetch spec; preflight-only headers must not be emitted.
	_, wrapped := WithCORS("OPTIONS /test", noopHandler, fullOpts)

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	wrapped(w, req)

	assert.Equal(suite.T(), "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Methods"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Headers"))
	assert.Empty(suite.T(), w.Header().Get("Access-Control-Max-Age"))
}

// --- Multi-Origin / whitespace handling. ---------------------------------

func (suite *CORSMiddlewareTestSuite) TestWithCORS_MultipleOriginHeadersRefused() {
	// Two Origin headers is a smuggling/relay signal; the Fetch spec sends
	// exactly one. Middleware must refuse without evaluating either, but
	// still set Vary: Origin so a shared cache cannot serve this no-ACAO
	// response back to a legitimate single-Origin request to the same URL.
	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Add("Origin", "https://example.com")
	req.Header.Add("Origin", "https://malicious.com")
	w := httptest.NewRecorder()
	wrapped(w, req)

	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(suite.T(), "Origin", w.Header().Get("Vary"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_OriginWhitespaceTrimmed() {
	// Header values can carry incidental whitespace; trim before matching so
	// a legitimate origin isn't blocked by an extra space.
	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req, w := newGetRequest("  https://example.com  ")
	wrapped(w, req)

	// The matcher echoes parsed.Raw, which is the trimmed value.
	assert.Equal(suite.T(), "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(suite.T(), "Origin", w.Header().Get("Vary"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_OriginAllWhitespaceIgnored() {
	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req, w := newGetRequest("   ")
	wrapped(w, req)

	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Origin"))
	// An all-whitespace value is treated as "no Origin"; no Vary.
	assert.Empty(suite.T(), w.Header().Get("Vary"))
}

// --- IPv6 / IDN / null. ---------------------------------------------------

func (suite *CORSMiddlewareTestSuite) TestWithCORS_IPv6Origin() {
	config.ResetServerRuntime()
	suite.initRuntime(`
- http://[::1]:8080
`)

	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req, w := newGetRequest("http://[::1]:8080")
	wrapped(w, req)

	assert.Equal(suite.T(), "http://[::1]:8080",
		w.Header().Get("Access-Control-Allow-Origin"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_IDNOriginPunycodeEquivalence() {
	// Configure with the Punycode form; a request that uses the Unicode form
	// must canonicalize to the same value and match.
	config.ResetServerRuntime()
	suite.initRuntime(`
- https://xn--mnchen-3ya.example
`)

	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req, w := newGetRequest("https://münchen.example")
	wrapped(w, req)

	assert.Equal(suite.T(), "https://münchen.example",
		w.Header().Get("Access-Control-Allow-Origin"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_NullOriginAllowed() {
	config.ResetServerRuntime()
	suite.initRuntime(`
- "null"
`)

	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req, w := newGetRequest("null")
	wrapped(w, req)

	assert.Equal(suite.T(), "null", w.Header().Get("Access-Control-Allow-Origin"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_NullOriginRejectedWhenNotConfigured() {
	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req, w := newGetRequest("null")
	wrapped(w, req)

	assert.Empty(suite.T(), w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(suite.T(), "Origin", w.Header().Get("Vary"))
}

// --- Trailing dot / case insensitivity. ----------------------------------

func (suite *CORSMiddlewareTestSuite) TestWithCORS_TrailingDotEquivalence() {
	// Hosts with a trailing dot are equivalent to hosts without one per DNS
	// canonicalization; a request that uses the trailing-dot form must match
	// a literal that doesn't.
	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req, w := newGetRequest("https://example.com.")
	wrapped(w, req)

	assert.Equal(suite.T(), "https://example.com.",
		w.Header().Get("Access-Control-Allow-Origin"))
}

func (suite *CORSMiddlewareTestSuite) TestWithCORS_HostCaseInsensitive() {
	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req, w := newGetRequest("https://EXAMPLE.com")
	wrapped(w, req)

	assert.Equal(suite.T(), "https://EXAMPLE.com",
		w.Header().Get("Access-Control-Allow-Origin"))
}

// --- Hot reload. ---------------------------------------------------------

func (suite *CORSMiddlewareTestSuite) TestWithCORS_HotReloadMatcherTakesEffect() {
	// The middleware reads the matcher from CORSConfig on every request, so
	// a fresh matcher installed via ResetServerRuntime + Initialize takes
	// effect on the very next request.
	_, wrapped := WithCORS("GET /test", noopHandler, fullOpts)

	req1, w1 := newGetRequest("https://example.com")
	wrapped(w1, req1)
	assert.Equal(suite.T(), "https://example.com", w1.Header().Get("Access-Control-Allow-Origin"))

	config.ResetServerRuntime()
	suite.initRuntime(`
- https://other.com
`)

	req2, w2 := newGetRequest("https://example.com")
	wrapped(w2, req2)
	assert.Empty(suite.T(), w2.Header().Get("Access-Control-Allow-Origin"))

	req3, w3 := newGetRequest("https://other.com")
	wrapped(w3, req3)
	assert.Equal(suite.T(), "https://other.com", w3.Header().Get("Access-Control-Allow-Origin"))
}
