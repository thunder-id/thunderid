// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package http provides a centralized HTTP client service for making outbound HTTP requests.
// This package offers an abstraction over the standard http.Client to centralize HTTP operations:
//
//   - NewHTTPClient() - creates a client with default 30s timeout
//   - NewHTTPClientWithTimeout(duration) - creates a client with custom timeout
//   - NewHTTPClientWithCheckRedirect(policy) - creates a client with a redirect policy and an SSRF dial guard
//   - NewHTTPClientForRegisteredEndpoint(duration) - creates a client for administrator-registered endpoints
//
// Usage examples:
//
//	// Default client
//	client := httpservice.NewHTTPClient()
//
//	// Custom timeout
//	client := httpservice.NewHTTPClientWithTimeout(10 * time.Second)
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

// HTTPClient implements HTTPClientInterface and provides a centralized HTTP client.
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient creates a new HTTPClient with default 30-second timeout.
// This method provides complete abstraction over http.Client references.
func NewHTTPClient() HTTPClientInterface {
	return NewHTTPClientWithTimeout(30 * time.Second)
}

// NewHTTPClientWithTimeout creates a new HTTPClient with a custom timeout.
// This is a convenience method for creating clients with specific timeouts.
func NewHTTPClientWithTimeout(timeout time.Duration) HTTPClientInterface {
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				// #nosec G402 -- Min TLS version is TLS 1.2 or higher based on config
				TLSClientConfig: &tls.Config{
					MinVersion: GetTLSVersion(config.GetServerRuntime().Config),
				},
			},
		},
	}
}

// NewHTTPClientWithCheckRedirect creates an HTTPClient with a custom redirect policy.
// Use this when redirect behavior must be controlled, e.g. to prevent HTTPS→HTTP downgrades.
// Requires server runtime to be initialized before calling (reads TLS config at construction time).
// ssrfSafeDialContext is wired in to block hostnames that DNS-resolve to private/loopback addresses
// and to pin the TCP connection to the first validated IP (prevents DNS rebinding).
func NewHTTPClientWithCheckRedirect(checkRedirect func(*http.Request, []*http.Request) error) HTTPClientInterface {
	return &HTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: ssrfSafeDialContext,
				// #nosec G402 -- Min TLS version is TLS 1.2 or higher based on config
				TLSClientConfig: &tls.Config{
					MinVersion: GetTLSVersion(config.GetServerRuntime().Config),
				},
			},
			CheckRedirect: checkRedirect,
		},
	}
}

// NewHTTPClientForRegisteredEndpoint creates an HTTPClient for calling an endpoint an administrator
// registered on a client, such as a back-channel logout URI. It never follows redirects and uses the
// caller's timeout. It deliberately applies no SSRF dial guard: the target comes from client
// configuration, never from a request parameter, so URL provenance is the control, and registered
// endpoints on private networks are legitimate.
func NewHTTPClientForRegisteredEndpoint(timeout time.Duration) HTTPClientInterface {
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				// #nosec G402 -- Min TLS version is TLS 1.2 or higher based on config
				TLSClientConfig: &tls.Config{
					MinVersion: GetTLSVersion(config.GetServerRuntime().Config),
				},
			},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
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
func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

// Get issues a GET to the specified URL.
func (c *HTTPClient) Get(url string) (*http.Response, error) {
	return c.client.Get(url)
}

// Head issues a HEAD to the specified URL.
func (c *HTTPClient) Head(url string) (*http.Response, error) {
	return c.client.Head(url)
}

// Post issues a POST to the specified URL.
func (c *HTTPClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	return c.client.Post(url, contentType, body)
}

// PostForm issues a POST to the specified URL, with data's keys and values URL-encoded as the request body.
func (c *HTTPClient) PostForm(url string, data url.Values) (*http.Response, error) {
	return c.client.PostForm(url, data)
}
