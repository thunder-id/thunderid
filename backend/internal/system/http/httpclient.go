// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package http provides a centralized HTTP client service for making outbound HTTP requests.
// This package offers an abstraction over the standard http.Client to centralize HTTP operations:
//
//   - NewHTTPClient() - creates a client with default 30s timeout
//   - NewHTTPClientWithTimeout(duration) - creates a client with custom timeout
//   - Initialize(allowPrivateAddresses) - creates the provider of clients and dialers for configured
//     destinations, which dial through the SSRF guard unless private addresses are allowed
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

// ErrPrivateAddress is returned, wrapped, when the SSRF-safe dialer refuses a host that resolves to a
// loopback, link-local, private or unspecified address.
var ErrPrivateAddress = errors.New("refused a private address")

// dialer opens network connections bounded by timeout. It resolves the target hostname and connects
// to the returned IPs in turn. With rejectPrivate it first validates every returned IP with
// isPrivateIP; without it the dial is the same, only the check is skipped, so a deployment that
// allows private addresses runs the same dial path as one that refuses them. Connecting to the
// validated IPs directly pins the connection and prevents DNS rebinding attacks, so the address
// checked is the address connected to. TLS hostname verification is unaffected: http.Transport derives
// ServerName from the request URL (not addr) when TLSClientConfig.ServerName is empty, so the
// certificate is still validated against the original hostname.
type dialer struct {
	timeout       time.Duration
	rejectPrivate bool
	resolver      ipResolverInterface
	conns         DialerInterface
}

// newDialer creates a dialer bounded by timeout that resolves through the system resolver and connects
// with a plain net.Dialer. With rejectPrivate it refuses hosts that resolve to a private address.
func newDialer(timeout time.Duration, rejectPrivate bool) *dialer {
	return &dialer{timeout: timeout, rejectPrivate: rejectPrivate, resolver: net.DefaultResolver,
		conns: &net.Dialer{}}
}

// ipResolverInterface resolves a host to its IP addresses, as net.Resolver does.
type ipResolverInterface interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// DialContext connects to addr on the named network. timeout bounds the whole dial, the resolution and
// every connection attempt together, as it does for the standard dialer; zero leaves it unbounded.
func (d *dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if d.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.timeout)
		defer cancel()
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// addr has no port (unlikely from http.Transport, but handle defensively)
		host = addr
		port = "443"
	}

	ipAddrs, err := d.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	if d.rejectPrivate {
		if err := refusePrivate(host, ipAddrs); err != nil {
			return nil, err
		}
	}
	if len(ipAddrs) == 0 {
		return nil, fmt.Errorf("host %q resolved to no usable addresses", host)
	}

	// Every address is one the dialer may reach, so trying them in turn, as the standard dialer does,
	// keeps a host reachable when its first address is not (an IPv6 address on an IPv4-only network). Like the
	// standard dialer, each attempt gets a share of the time left, so an address that drops connection
	// attempts cannot use up the whole deadline before a reachable one is tried.
	var dialErr error
	for i, ia := range ipAddrs {
		attemptCtx, cancel := ctx, context.CancelFunc(func() {})
		if deadline, ok := ctx.Deadline(); ok {
			attemptCtx, cancel = context.WithDeadline(ctx, attemptDeadline(time.Now(), deadline, len(ipAddrs)-i))
		}
		conn, err := d.conns.DialContext(attemptCtx, network, net.JoinHostPort(ia.IP.String(), port))
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

// refusePrivate returns an error naming the first of host's addresses that isPrivateIP reports, and
// nil when none is.
func refusePrivate(host string, ipAddrs []net.IPAddr) error {
	for _, ia := range ipAddrs {
		if isPrivateIP(ia.IP) {
			return fmt.Errorf("host %q resolves to a private address %s: %w", host, ia.IP, ErrPrivateAddress)
		}
	}
	return nil
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
// (including instance metadata services), RFC1918/unique-local private ranges, the RFC 6598 shared
// address space some clusters and instance metadata services use, the "this network" block, the IETF
// protocol assignments, benchmarking, reserved and broadcast blocks, multicast, and the local-use
// NAT64 prefix, whose embedding of an IPv4 address depends on the network.
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
		// service ranges, and the instance metadata service some providers serve at 100.100.100.200)
		"0.0.0.0/8",      // "this network" (RFC 1122); 0.0.0.0 itself is also refused as unspecified
		"192.0.0.0/24",   // IETF protocol assignments (RFC 6890)
		"198.18.0.0/15",  // benchmarking (RFC 2544)
		"224.0.0.0/4",    // IPv4 multicast
		"240.0.0.0/4",    // reserved (RFC 1112), including the 255.255.255.255 broadcast address
		"ff00::/8",       // IPv6 multicast
		"64:ff9b:1::/48", // local-use NAT64 (RFC 8215)
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, ipNet, _ := net.ParseCIDR(c)
		nets = append(nets, ipNet)
	}
	return nets
}()

// nat64Prefix is the well-known NAT64 prefix (RFC 6052), whose last 32 bits are the IPv4 address a
// NAT64 gateway connects to.
var nat64Prefix = &net.IPNet{IP: net.ParseIP("64:ff9b::"), Mask: net.CIDRMask(96, 128)}

// sixToFourPrefix is the 6to4 prefix (RFC 3056), whose next 32 bits are the IPv4 address of the relay
// the packets are tunneled to.
var sixToFourPrefix = &net.IPNet{IP: net.ParseIP("2002::"), Mask: net.CIDRMask(16, 128)}

// isPrivateIP reports whether ip is unspecified or in privateIPRanges, or is an IPv6 address that
// embeds an IPv4 address that is: a NAT64 or 6to4 address reaches the IPv4 address it carries, so
// 64:ff9b::a9fe:a9fe is refused as 169.254.169.254 is. An IPv4-mapped IPv6 address is matched as
// its IPv4 address by net.IPNet.Contains.
func isPrivateIP(ip net.IP) bool {
	if ip.IsUnspecified() {
		return true
	}
	for _, block := range privateIPRanges {
		if block.Contains(ip) {
			return true
		}
	}
	if ip.To4() != nil {
		return false
	}
	if nat64Prefix.Contains(ip) {
		return isPrivateIP(ip[12:16])
	}
	if sixToFourPrefix.Contains(ip) {
		return isPrivateIP(ip[2:6])
	}
	return false
}

// IsPrivateHost reports whether host is localhost or an IP literal isPrivateIP reports. Hostnames are
// not resolved.
func IsPrivateHost(host string) bool {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && isPrivateIP(ip)
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
	if ip := net.ParseIP(host); ip != nil && isPrivateIP(ip) {
		return fmt.Errorf("host %q is a loopback, link-local, or private address", host)
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
