// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package http provides a centralized HTTP client service for making outbound HTTP requests.
// This package offers an abstraction over the standard http.Client to centralize HTTP operations:
//
//   - NewHTTPClient(HTTPClientConfig) - creates a client from a config struct
//   - NewDefaultHTTPClient() - creates a client with the default 30s timeout that follows redirects
//
// Safety controls (timeout, redirect policy, SSRF dial guard) are selected per caller through
// HTTPClientConfig instead of being bundled into fixed constructor combinations.
//
// Usage examples:
//
//	// Default client
//	client := httpservice.NewDefaultHTTPClient()
//
//	// Custom timeout with the SSRF dial guard
//	client := httpservice.NewHTTPClient(httpservice.HTTPClientConfig{
//		Timeout:   10 * time.Second,
//		GuardSSRF: true,
//	})
package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// HTTPClientInterface defines the interface for HTTP client operations.
type HTTPClientInterface interface {
	// Do executes an HTTP request and returns an HTTP response.
	Do(req *http.Request) (*http.Response, error)
	// Get issues a GET to the specified URL.
	Get(url string) (*http.Response, error)
	// Head issues a HEAD to the specified URL.
	Head(url string) (*http.Response, error)
	// Post issues a POST to the specified URL.
	Post(url, contentType string, body io.Reader) (*http.Response, error)
	// PostForm issues a POST to the specified URL, with data's keys and values URL-encoded as the request body.
	PostForm(url string, data url.Values) (*http.Response, error)
}

// httpClient implements HTTPClientInterface and provides a centralized HTTP client.
type httpClient struct {
	client *http.Client
}

// HTTPClientConfig carries the options for an outbound HTTP client. The zero
// value is safe: default timeout, redirects followed, no SSRF dial guard.
type HTTPClientConfig struct {
	// Timeout bounds one request. Zero means the 30s default.
	Timeout time.Duration
	// DisableTimeout builds the client with no timeout at all, leaving each
	// request bounded only by its context. Callers that set their own
	// per-request deadlines, such as the PDP engine, use this to keep the
	// client out of deadline enforcement.
	DisableTimeout bool
	// DisableRedirects returns a 3xx response instead of following it.
	DisableRedirects bool
	// CheckRedirect is a redirect policy for what DisableRedirects can't
	// express, such as refusing an https to http downgrade. Ignored when
	// DisableRedirects is set.
	CheckRedirect func(*http.Request, []*http.Request) error
	// GuardSSRF dials through the SSRF-safe dialer, refusing any host that
	// resolves to a loopback, link-local, private or unspecified address and
	// pinning the connection to the first validated IP (prevents DNS
	// rebinding). Leave it off only for targets an administrator registered
	// on an internal network.
	GuardSSRF bool
}

// defaultHTTPTimeout bounds one request unless HTTPClientConfig.Timeout overrides it.
const defaultHTTPTimeout = 30 * time.Second

// NewHTTPClient creates a client from cfg. The zero config is the default
// client: 30s timeout, redirects followed, no SSRF dial guard.
// Requires server runtime to be initialized before calling (reads TLS config at construction time).
func NewHTTPClient(cfg HTTPClientConfig) HTTPClientInterface {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultHTTPTimeout
	}
	transport := &http.Transport{
		// #nosec G402 -- Min TLS version is TLS 1.2 or higher based on config
		TLSClientConfig: &tls.Config{
			MinVersion: GetTLSVersion(config.GetServerRuntime().Config),
		},
	}
	if cfg.GuardSSRF {
		transport.DialContext = ssrfSafeDialContext
	}
	client := &http.Client{
		Transport: transport,
	}
	if !cfg.DisableTimeout {
		client.Timeout = timeout
	}
	switch {
	case cfg.DisableRedirects:
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	case cfg.CheckRedirect != nil:
		client.CheckRedirect = cfg.CheckRedirect
	}
	return &httpClient{client: client}
}

// NewDefaultHTTPClient creates a client with the default 30s timeout that follows redirects.
// This method provides complete abstraction over http.Client references.
func NewDefaultHTTPClient() HTTPClientInterface {
	return NewHTTPClient(HTTPClientConfig{})
}

// ssrfSafeDialContext resolves the target hostname and validates every returned IP against
// privateIPRanges before dialing. Connecting to the first validated IP directly pins the
// connection and prevents DNS rebinding attacks. TLS hostname verification is unaffected:
// http.Transport derives ServerName from the request URL (not addr) when TLSClientConfig.ServerName
// is empty, so the certificate is still validated against the original hostname.
func ssrfSafeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// addr has no port (unlikely from http.Transport, but handle defensively)
		host = addr
		port = "443"
	}

	ipAddrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	var safeIP net.IP
	for _, ia := range ipAddrs {
		if ia.IP.IsUnspecified() {
			return nil, fmt.Errorf("host %q resolves to an unspecified address %s", host, ia.IP)
		}
		for _, block := range privateIPRanges {
			if block.Contains(ia.IP) {
				return nil, fmt.Errorf("host %q resolves to a private address %s", host, ia.IP)
			}
		}
		if safeIP == nil {
			safeIP = ia.IP
		}
	}
	if safeIP == nil {
		return nil, fmt.Errorf("host %q resolved to no usable addresses", host)
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(safeIP.String(), port))
}

// privateIPRanges lists CIDR blocks that must not be used as JWKS fetch targets.
// Covers IPv4/IPv6 loopback, link-local (including cloud metadata services), and
// RFC1918/unique-local private ranges.
var privateIPRanges = func() []*net.IPNet {
	cidrs := []string{
		"127.0.0.0/8",    // IPv4 loopback
		"::1/128",        // IPv6 loopback
		"169.254.0.0/16", // IPv4 link-local (AWS/GCP metadata: 169.254.169.254)
		"fe80::/10",      // IPv6 link-local
		"10.0.0.0/8",     // RFC1918 private
		"172.16.0.0/12",  // RFC1918 private
		"192.168.0.0/16", // RFC1918 private
		"fc00::/7",       // IPv6 unique-local
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, ipNet, _ := net.ParseCIDR(c)
		nets = append(nets, ipNet)
	}
	return nets
}()

// IsPrivateHost reports whether host is localhost, an unspecified address, or an IP literal in a
// loopback, link-local or private range. Hostnames are not resolved.
func IsPrivateHost(host string) bool {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsUnspecified() {
		return true
	}
	for _, block := range privateIPRanges {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// IsSSRFSafeURL reports whether rawURL is safe for server-side fetching.
// It requires HTTPS and rejects hosts that are IP literals in loopback, link-local,
// or private ranges to mitigate server-side request forgery. Hostnames are not
// DNS-resolved here; apply this check again to redirect targets at fetch time.
func IsSSRFSafeURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return errors.New("URL must use HTTPS")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("URL has no host")
	}
	if ip := net.ParseIP(host); ip != nil {
		for _, block := range privateIPRanges {
			if block.Contains(ip) {
				return fmt.Errorf("host %q is a loopback, link-local, or private address", host)
			}
		}
	}
	return nil
}

// Do executes an HTTP request and returns an HTTP response.
func (c *httpClient) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

// Get issues a GET to the specified URL.
func (c *httpClient) Get(url string) (*http.Response, error) {
	return c.client.Get(url)
}

// Head issues a HEAD to the specified URL.
func (c *httpClient) Head(url string) (*http.Response, error) {
	return c.client.Head(url)
}

// Post issues a POST to the specified URL.
func (c *httpClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	return c.client.Post(url, contentType, body)
}

// PostForm issues a POST to the specified URL, with data's keys and values URL-encoded as the request body.
func (c *httpClient) PostForm(url string, data url.Values) (*http.Response, error) {
	return c.client.PostForm(url, data)
}
