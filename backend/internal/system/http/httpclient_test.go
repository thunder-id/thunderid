// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// HTTPClientTestSuite defines the test suite for HTTP client service.
type HTTPClientTestSuite struct {
	suite.Suite
}

// TestHTTPClientSuite runs the HTTP client test suite.
func TestHTTPClientSuite(t *testing.T) {
	suite.Run(t, new(HTTPClientTestSuite))
}

func (suite *HTTPClientTestSuite) SetupSuite() {
	thunderConfig := &config.Config{
		TLS: config.TLSConfig{
			MinVersion: "1.3",
		},
	}
	err := config.InitializeServerRuntime("", thunderConfig)
	assert.NoError(suite.T(), err)
}

// newTestClient returns a client with timeout from a factory that allows private addresses, so it can
// reach the httptest servers on loopback.
func newTestClient(timeout time.Duration) HTTPClientInterface {
	return newOutboundClientFactory(true).NewHTTPClient(timeout)
}

func (suite *HTTPClientTestSuite) TestNewHTTPClient_SetsTheTimeout() {
	timeout := 5 * time.Second
	client := newTestClient(timeout)
	assert.Implements(suite.T(), (*HTTPClientInterface)(nil), client)
	assert.Equal(suite.T(), timeout, client.(*HTTPClient).client.Timeout)
}

func (suite *HTTPClientTestSuite) TestDo() {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	}))
	defer testServer.Close()

	client := newTestClient(30 * time.Second)

	// Create a request
	req, err := http.NewRequest("GET", testServer.URL, nil)
	assert.NoError(suite.T(), err)

	// Execute the request
	resp, err := client.Do(req)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func (suite *HTTPClientTestSuite) TestDoWithPost() {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(suite.T(), "POST", r.Method)
		assert.Equal(suite.T(), "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	defer testServer.Close()

	client := newTestClient(30 * time.Second)

	// Create a POST request
	req, err := http.NewRequest("POST", testServer.URL, strings.NewReader(`{"test": "data"}`))
	assert.NoError(suite.T(), err)
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := client.Do(req)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)

	_ = resp.Body.Close()
}

func (suite *HTTPClientTestSuite) TestDoWithTimeout() {
	// Create a test server that delays response
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Create client with short timeout
	client := newTestClient(100 * time.Millisecond)

	// Create a request
	req, err := http.NewRequest("GET", testServer.URL, nil)
	assert.NoError(suite.T(), err)

	// Execute the request - should timeout
	resp, err := client.Do(req)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
	assert.Contains(suite.T(), err.Error(), "deadline exceeded")
}

func (suite *HTTPClientTestSuite) TestGet() {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(suite.T(), "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("get response"))
	}))
	defer testServer.Close()

	client := newTestClient(30 * time.Second)

	// Execute the GET request
	resp, err := client.Get(testServer.URL)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func (suite *HTTPClientTestSuite) TestDoWithError() {
	// Create a test server and immediately close it to ensure the
	// connection attempt fails without relying on external network
	// conditions.
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	testServer.Close()

	client := newTestClient(30 * time.Second)

	// Create a request to the closed server
	req, err := http.NewRequest("GET", testServer.URL, nil)
	assert.NoError(suite.T(), err)

	// Execute the request - should fail because the server is closed
	resp, err := client.Do(req)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
}

func (suite *HTTPClientTestSuite) TestHead() {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(suite.T(), "HEAD", r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	client := newTestClient(30 * time.Second)

	// Execute the HEAD request
	resp, err := client.Head(testServer.URL)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func (suite *HTTPClientTestSuite) TestPost() {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(suite.T(), "POST", r.Method)
		assert.Equal(suite.T(), "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), `{"test": "data"}`, string(body))

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	defer testServer.Close()

	client := newTestClient(30 * time.Second)

	// Execute the POST request
	resp, err := client.Post(testServer.URL, "application/json", strings.NewReader(`{"test": "data"}`))
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)

	_ = resp.Body.Close()
}

func (suite *HTTPClientTestSuite) TestGuardedDial_RefusesPrivateAddresses() {
	// IP literals — LookupIPAddr returns them directly without DNS, so this exercises
	// the same validation path as a hostname that DNS-resolves to a private address.
	blockedAddrs := []string{
		"127.0.0.1:443",            // IPv4 loopback
		"169.254.169.254:443",      // IPv4 link-local (instance metadata)
		"10.0.0.1:443",             // RFC1918
		"172.16.0.1:443",           // RFC1918
		"192.168.1.1:443",          // RFC1918
		"[::1]:443",                // IPv6 loopback
		"[fc00::1]:443",            // IPv6 unique-local (ULA)
		"[fe80::1]:443",            // IPv6 link-local
		"100.64.0.1:443",           // RFC 6598 shared address space
		"100.100.100.200:443",      // instance metadata, in RFC 6598 space
		"0.1.2.3:443",              // "this network"
		"192.0.0.8:443",            // IETF protocol assignments
		"198.18.0.1:443",           // benchmarking
		"224.0.0.1:443",            // IPv4 multicast
		"255.255.255.255:443",      // broadcast
		"[ff02::1]:443",            // IPv6 multicast
		"[::ffff:127.0.0.1]:443",   // IPv4-mapped loopback
		"[64:ff9b::a9fe:a9fe]:443", // NAT64 of the link-local metadata address
		"[64:ff9b:1::1]:443",       // local-use NAT64
		"[2002:a00:1::1]:443",      // 6to4 of 10.0.0.1
		"[2002:7f00:1::1]:443",     // 6to4 of 127.0.0.1
	}
	dialer := newDialer(10*time.Second, true)
	for _, addr := range blockedAddrs {
		_, err := dialer.DialContext(context.Background(), "tcp", addr)
		assert.ErrorContains(suite.T(), err, "private address", "addr %s should be blocked", addr)
		assert.ErrorIs(suite.T(), err, ErrPrivateAddress, "addr %s should wrap ErrPrivateAddress", addr)
	}
	_, err := dialer.DialContext(context.Background(), "tcp", "0.0.0.0:443")
	assert.ErrorIs(suite.T(), err, ErrPrivateAddress)

	// Public IP: SSRF check passes; use an already-canceled context so the dial fails
	// immediately and deterministically with context.Canceled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = dialer.DialContext(ctx, "tcp", "1.1.1.1:443")
	assert.Error(suite.T(), err)
	assert.NotErrorIs(suite.T(), err, ErrPrivateAddress)
	assert.NotContains(suite.T(), err.Error(), "private address")
	assert.NotContains(suite.T(), err.Error(), "resolved to no usable")
}

func (suite *HTTPClientTestSuite) TestPostForm() {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(suite.T(), "POST", r.Method)
		assert.Equal(suite.T(), "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		err := r.ParseForm()
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), "value1", r.FormValue("key1"))
		assert.Equal(suite.T(), "value2", r.FormValue("key2"))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("form received"))
	}))
	defer testServer.Close()

	client := newTestClient(30 * time.Second)

	// Prepare form data
	formData := url.Values{}
	formData.Set("key1", "value1")
	formData.Set("key2", "value2")

	// Execute the PostForm request
	resp, err := client.PostForm(testServer.URL, formData)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func (suite *HTTPClientTestSuite) TestNewHTTPClientWithoutRedirects() {
	timeout := 3 * time.Second
	client := newOutboundClientFactory(true).NewHTTPClientWithoutRedirects(timeout)
	assert.Implements(suite.T(), (*HTTPClientInterface)(nil), client)

	httpClient := client.(*HTTPClient)
	assert.Equal(suite.T(), timeout, httpClient.client.Timeout)
	transport := httpClient.client.Transport.(*http.Transport)
	assert.NotNil(suite.T(), transport.DialContext, "the dial goes through the provider's dialer")
	assert.Nil(suite.T(), transport.Proxy, "no proxy, so HTTP_PROXY cannot route around the guard")
}

func (suite *HTTPClientTestSuite) TestClientWithoutRedirects_DoesNotFollowRedirects() {
	landed := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		landed = true
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirecting.Close()

	client := newOutboundClientFactory(true).NewHTTPClientWithoutRedirects(5 * time.Second)
	resp, err := client.Post(redirecting.URL, "application/x-www-form-urlencoded",
		strings.NewReader("logout_token=x"))

	assert.NoError(suite.T(), err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(suite.T(), http.StatusFound, resp.StatusCode, "the redirect is returned, not followed")
	assert.Equal(suite.T(), target.URL, resp.Header.Get("Location"))
	assert.False(suite.T(), landed, "the redirect target must never be called")
}

func (suite *HTTPClientTestSuite) TestClientWithoutRedirects_ReachesLoopback() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// httptest binds to 127.0.0.1, which a guarded client would refuse to dial.
	resp, err := newOutboundClientFactory(true).NewHTTPClientWithoutRedirects(5 * time.Second).Get(server.URL)

	assert.NoError(suite.T(), err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

func (suite *HTTPClientTestSuite) TestClientWithoutRedirects_HonoursTimeout() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := newOutboundClientFactory(true).NewHTTPClientWithoutRedirects(50 * time.Millisecond).Get(server.URL)

	assert.Error(suite.T(), err)
	if resp != nil {
		_ = resp.Body.Close()
	}
}

func (suite *HTTPClientTestSuite) TestNewHTTPClientWithRootCAs() {
	timeout := 3 * time.Second
	roots := x509.NewCertPool()
	client := newOutboundClientFactory(true).NewHTTPClientWithRootCAs(timeout, roots)
	assert.Implements(suite.T(), (*HTTPClientInterface)(nil), client)

	httpClient := client.(*HTTPClient)
	assert.Equal(suite.T(), timeout, httpClient.client.Timeout)
	transport := httpClient.client.Transport.(*http.Transport)
	assert.Same(suite.T(), roots, transport.TLSClientConfig.RootCAs)
	assert.Equal(suite.T(), uint16(tls.VersionTLS13), transport.TLSClientConfig.MinVersion)
	assert.NotNil(suite.T(), transport.DialContext, "the dial goes through the provider's dialer")
}

func (suite *HTTPClientTestSuite) TestClientWithRootCAs_TrustsOnlyTheGivenAuthorities() {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	trusted := x509.NewCertPool()
	trusted.AddCert(server.Certificate())

	client := newOutboundClientFactory(true).NewHTTPClientWithRootCAs(5*time.Second, trusted)
	resp, err := client.Get(server.URL)
	assert.NoError(suite.T(), err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	client.(*HTTPClient).CloseIdleConnections()

	refused, err := newOutboundClientFactory(true).NewHTTPClientWithRootCAs(5*time.Second, x509.NewCertPool()).
		Get(server.URL)
	if refused != nil {
		_ = refused.Body.Close()
	}
	var unknownAuthority x509.UnknownAuthorityError
	assert.ErrorAs(suite.T(), err, &unknownAuthority, "a certificate from another authority is refused")
}

func (suite *HTTPClientTestSuite) TestClientWithRootCAs_DoesNotFollowRedirects() {
	landed := false
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		landed = true
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	redirecting := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirecting.Close()
	trusted := x509.NewCertPool()
	trusted.AddCert(redirecting.Certificate())
	trusted.AddCert(target.Certificate())

	resp, err := newOutboundClientFactory(true).NewHTTPClientWithRootCAs(5*time.Second, trusted).Get(redirecting.URL)

	assert.NoError(suite.T(), err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(suite.T(), http.StatusFound, resp.StatusCode, "the redirect is returned, not followed")
	assert.False(suite.T(), landed, "the redirect target must never be called")
}

func (suite *HTTPClientTestSuite) TestIsPrivateHost() {
	for _, host := range []string{"localhost", "LOCALHOST", "app.localhost", "127.0.0.1", "::1", "169.254.169.254",
		"10.1.2.3", "172.20.0.1", "192.168.0.1", "fd12::1", "fe80::1", "0.0.0.0", "::", "100.64.0.1",
		"100.127.255.255", "100.100.100.200", "0.1.2.3", "192.0.0.1", "198.19.255.255", "224.0.0.251",
		"255.255.255.255", "ff02::1", "::ffff:10.0.0.1", "64:ff9b::a9fe:a9fe", "64:ff9b:1::1", "2002:c0a8:101::1"} {
		assert.True(suite.T(), IsPrivateHost(host), host)
	}
	for _, host := range []string{"rp.example.com", "8.8.8.8", "2001:4860:4860::8888", "172.32.0.1",
		"100.63.255.255", "100.128.0.1", "198.20.0.1", "64:ff9b::808:808", "2002:808:808::1"} {
		assert.False(suite.T(), IsPrivateHost(host), host)
	}
}

func (suite *HTTPClientTestSuite) TestGuardedClients_RefusePrivateAddresses() {
	provider := newOutboundClientFactory(false)
	for name, client := range map[string]HTTPClientInterface{
		"check redirect": provider.NewHTTPClientWithCheckRedirect(time.Second, refuseRedirects),
		"root CAs":       provider.NewHTTPClientWithRootCAs(time.Second, x509.NewCertPool()),
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		// httptest binds to 127.0.0.1, which the guard refuses.
		resp, err := client.Get(server.URL)
		if resp != nil {
			_ = resp.Body.Close()
		}
		server.Close()
		assert.ErrorIs(suite.T(), err, ErrPrivateAddress, name)
	}
}

func (suite *HTTPClientTestSuite) TestClientWithoutRedirects_RejectPrivateUsesTheSSRFGuard() {
	client := newOutboundClientFactory(false).NewHTTPClientWithoutRedirects(time.Second).(*HTTPClient)
	transport := client.client.Transport.(*http.Transport)
	assert.NotNil(suite.T(), transport.DialContext, "the SSRF-safe dialer is wired in")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// httptest binds to 127.0.0.1, which the guard refuses.
	resp, err := client.Get(server.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	assert.Error(suite.T(), err)
}

func (suite *HTTPClientTestSuite) TestGuardedDial_RefusesUnspecifiedAddress() {
	for _, addr := range []string{"0.0.0.0:443", "[::]:443"} {
		_, err := newDialer(time.Second, true).DialContext(context.Background(), "tcp", addr)
		assert.ErrorIs(suite.T(), err, ErrPrivateAddress, addr)
	}
}

// A redirect policy given to the provider is applied to every redirect.
func (suite *HTTPClientTestSuite) TestClientWithCheckRedirect_AppliesThePolicy() {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirecting.Close()
	refused := errors.New("refused")

	resp, err := newOutboundClientFactory(true).NewHTTPClientWithCheckRedirect(time.Second,
		func(*http.Request, []*http.Request) error { return refused }).Get(redirecting.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	assert.ErrorIs(suite.T(), err, refused)
}

// A dialer carries its timeout, and checks the address it connects to only when it refuses private ones.
func (suite *HTTPClientTestSuite) TestNewDialer_GuardsOnlyWhenRejectingPrivate() {
	guarded := newDialer(5*time.Second, true)
	suite.Equal(5*time.Second, guarded.Timeout)
	suite.NotNil(guarded.ControlContext)

	unguarded := newDialer(5*time.Second, false)
	suite.Equal(5*time.Second, unguarded.Timeout)
	suite.Nil(unguarded.ControlContext)
}

// The control function refuses a private address, a zoned one included, and one it cannot parse, and
// lets a public address through.
func (suite *HTTPClientTestSuite) TestRefusePrivateAddress() {
	for _, addr := range []string{"10.0.0.1:80", "[fe80::1%en0]:80", "[::1]:443", "not-an-ip:80"} {
		suite.ErrorIs(refusePrivateAddress(context.Background(), "tcp", addr, nil), ErrPrivateAddress, addr)
	}
	suite.NoError(refusePrivateAddress(context.Background(), "tcp", "203.0.113.1:443", nil))
	suite.NoError(refusePrivateAddress(context.Background(), "tcp6", "[2001:db8::1]:443", nil))
}

// A hostname is checked on the address it resolves to, so one that resolves to loopback is refused.
func (suite *HTTPClientTestSuite) TestGuardedDial_RefusesAHostnameThatResolvesToLoopback() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	suite.Require().NoError(err)
	defer func() { _ = listener.Close() }()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	suite.Require().NoError(err)

	_, err = newDialer(time.Second, true).DialContext(context.Background(), "tcp", "localhost:"+port)

	suite.ErrorIs(err, ErrPrivateAddress)
}

// A dialer that allows private addresses reaches them.
func (suite *HTTPClientTestSuite) TestUnguardedDial_ReachesPrivateAddresses() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	suite.Require().NoError(err)
	defer func() { _ = listener.Close() }()

	conn, err := newDialer(time.Second, false).DialContext(context.Background(), "tcp", listener.Addr().String())

	suite.Require().NoError(err)
	_ = conn.Close()
}
