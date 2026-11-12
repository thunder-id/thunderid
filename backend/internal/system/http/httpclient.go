// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package http provides a centralized HTTP client service for making outbound HTTP requests.
// This package offers an abstraction over the standard http.Client to centralize HTTP operations:
//
//   - NewHTTPClient() - creates a client with default 30s timeout
//   - NewHTTPClientWithTimeout(duration) - creates a client with custom timeout
//   - NewHTTPClientWithCheckRedirect(policy) - creates a client with a redirect policy and an SSRF dial guard
//   - NewHTTPClientWithoutRedirects(duration, rejectPrivate) - creates a client with a custom timeout that never
//     follows redirects, optionally with the SSRF dial guard
//   - NewHTTPClientWithRootCAs(duration, rootCAs, rejectPrivate) - creates a client with a custom timeout that never
//     follows redirects and trusts the given certificate authorities, optionally with the SSRF dial guard
//   - NewOutboundClientProvider(allowPrivateAddresses) - creates the provider of clients and dialers for
//     destinations an administrator configures, guarded unless private addresses are allowed
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
	"crypto/x509"
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
// and to pin the TCP connection to the validated IPs (prevents DNS rebinding).
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

// NewHTTPClientWithoutRedirects creates an HTTPClient with the given timeout that returns a 3xx response
// instead of following it. With rejectPrivate it dials through the SSRF-safe dialer, refusing any host
// that resolves to a loopback, link-local, private or unspecified address. Without it there is no
// guard, so callers must make sure the target was set by an authorized principal.
func NewHTTPClientWithoutRedirects(timeout time.Duration, rejectPrivate bool) HTTPClientInterface {
	transport := &http.Transport{
		// #nosec G402 -- Min TLS version is TLS 1.2 or higher based on config
		TLSClientConfig: &tls.Config{
			MinVersion: GetTLSVersion(config.GetServerRuntime().Config),
		},
	}
	if rejectPrivate {
		transport.DialContext = ssrfSafeDialContext
	}
	return &HTTPClient{
		client: &http.Client{
			Timeout:       timeout,
			Transport:     transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// NewHTTPClientWithRootCAs creates an HTTPClient with the given timeout that trusts the given certificate
// authorities and returns a 3xx response instead of following it. Use it to reach a server whose
// certificate a private authority issued, with verification kept on. With rejectPrivate it dials
// through the SSRF-safe dialer, refusing any host that resolves to a loopback, link-local, private
// or unspecified address.
func NewHTTPClientWithRootCAs(timeout time.Duration, rootCAs *x509.CertPool, rejectPrivate bool) HTTPClientInterface {
	transport := &http.Transport{
		// #nosec G402 -- Min TLS version is TLS 1.2 or higher based on config
		TLSClientConfig: &tls.Config{
			MinVersion: GetTLSVersion(config.GetServerRuntime().Config),
			RootCAs:    rootCAs,
		},
	}
	if rejectPrivate {
		transport.DialContext = ssrfSafeDialContext
	}
	return &HTTPClient{
		client: &http.Client{
			Timeout:       timeout,
			Transport:     transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// ErrPrivateAddress is returned, wrapped, when the SSRF-safe dialer refuses a host that resolves to a
// loopback, link-local, private or unspecified address.
var ErrPrivateAddress = errors.New("refused a private address")

// ssrfSafeDialContext dials through the SSRF-safe dialer with a 10-second connect timeout.
func ssrfSafeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return (&dialer{timeout: 10 * time.Second, rejectPrivate: true}).DialContext(ctx, network, addr)
}

// dialer opens network connections bounded by timeout. With rejectPrivate it resolves the target
// hostname and validates every returned IP against privateIPRanges before dialing. Connecting to the
// validated IPs directly pins the connection and prevents DNS rebinding attacks, so the address
// checked is the address connected to. TLS hostname verification is unaffected: http.Transport derives
// ServerName from the request URL (not addr) when TLSClientConfig.ServerName is empty, so the
// certificate is still validated against the original hostname.
type dialer struct {
	timeout       time.Duration
	rejectPrivate bool
	// resolver and conns, when set, replace the system resolver and a plain net.Dialer for the guarded
	// dial. Only tests set them.
	resolver ipResolverInterface
	conns    connDialerInterface
}

// ipResolverInterface resolves a host to its IP addresses, as net.Resolver does.
type ipResolverInterface interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// connDialerInterface opens a connection, as net.Dialer does.
type connDialerInterface interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// DialContext connects to addr on the named network. timeout bounds the whole dial, the resolution and
// every connection attempt together, as it does for the standard dialer; zero leaves it unbounded.
func (d *dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	netDialer := &net.Dialer{Timeout: d.timeout}
	if !d.rejectPrivate {
		return netDialer.DialContext(ctx, network, addr)
	}
	if d.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.timeout)
		defer cancel()
	}
	var resolver ipResolverInterface = net.DefaultResolver
	if d.resolver != nil {
		resolver = d.resolver
	}
	var conns connDialerInterface = &net.Dialer{}
	if d.conns != nil {
		conns = d.conns
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// addr has no port (unlikely from http.Transport, but handle defensively)
		host = addr
		port = "443"
	}

	ipAddrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	for _, ia := range ipAddrs {
		if ia.IP.IsUnspecified() {
			return nil, fmt.Errorf("host %q resolves to an unspecified address %s: %w", host, ia.IP, ErrPrivateAddress)
		}
		for _, block := range privateIPRanges {
			if block.Contains(ia.IP) {
				return nil, fmt.Errorf("host %q resolves to a private address %s: %w", host, ia.IP, ErrPrivateAddress)
			}
		}
	}
	if len(ipAddrs) == 0 {
		return nil, fmt.Errorf("host %q resolved to no usable addresses", host)
	}

	// Every address was validated above, so trying them in turn, as the standard dialer does, keeps a
	// host reachable when its first address is not (an IPv6 address on an IPv4-only network). Like the
	// standard dialer, each attempt gets a share of the time left, so an address that drops connection
	// attempts cannot use up the whole deadline before a reachable one is tried.
	var dialErr error
	for i, ia := range ipAddrs {
		attemptCtx, cancel := ctx, context.CancelFunc(func() {})
		if deadline, ok := ctx.Deadline(); ok {
			attemptCtx, cancel = context.WithDeadline(ctx, attemptDeadline(time.Now(), deadline, len(ipAddrs)-i))
		}
		conn, err := conns.DialContext(attemptCtx, network, net.JoinHostPort(ia.IP.String(), port))
		cancel()
		if err == nil {
			return conn, nil
		}
		dialErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, dialErr
}

// minAttemptTime is the least share of the remaining time a connection attempt gets when more
// addresses follow it, unless less than that remains, as the standard dialer's saneMinimum is.
const minAttemptTime = 2 * time.Second

// attemptDeadline is the deadline of one connection attempt when remaining addresses, this one
// included, are left to try before deadline: an equal share of the time left, at least
// minAttemptTime, never past deadline. The last address gets all that is left.
func attemptDeadline(now, deadline time.Time, remaining int) time.Time {
	timeLeft := deadline.Sub(now)
	if remaining <= 1 || timeLeft <= 0 {
		return deadline
	}
	share := timeLeft / time.Duration(remaining)
	if share < minAttemptTime {
		share = min(minAttemptTime, timeLeft)
	}
	return now.Add(share)
}

// privateIPRanges lists the CIDR blocks no guarded call may reach: IPv4/IPv6 loopback, link-local
// (including cloud metadata services), RFC1918/unique-local private ranges, the RFC 6598 shared
// address space some clusters and cloud metadata services use, and the "this network" block.
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
		"100.64.0.0/10",  // RFC 6598 shared address space (carrier-grade NAT; some cluster pod and
		// service ranges, and Alibaba Cloud metadata at 100.100.100.200)
		"0.0.0.0/8", // "this network" (RFC 1122); 0.0.0.0 itself is also refused as unspecified
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

// CloseIdleConnections closes the connections the client keeps open for reuse, so a client built for
// one call releases them once the call is done.
func (c *HTTPClient) CloseIdleConnections() {
	c.client.CloseIdleConnections()
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
