// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package http provides the HTTP clients and dialers for outbound calls. Initialize creates the
// factory that builds them; every client and dialer it builds refuses private addresses unless the
// factory allows them, and an unguarded client is available for destinations the deployment
// configuration sets.
package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
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

// ErrPrivateAddress is returned, wrapped, when the SSRF-safe dialer refuses a host that resolves to a
// loopback, link-local, private or unspecified address.
var ErrPrivateAddress = errors.New("refused a private address")

// DialerInterface opens network connections.
type DialerInterface interface {
	// DialContext connects to addr on the named network.
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// newDialer returns a dialer bounded by timeout; zero leaves the bound to the context a dial is given.
// With rejectPrivate it refuses every address isPrivateIP reports. The check runs on each resolved
// address just before the socket connects, so the address checked is the address connected to and a
// hostname that resolves, or is rebound, to a private address is refused too. The standard dialer
// still splits its deadline across a host's addresses and falls back between IPv6 and IPv4.
func newDialer(timeout time.Duration, rejectPrivate bool) *net.Dialer {
	d := &net.Dialer{Timeout: timeout}
	if rejectPrivate {
		d.ControlContext = refusePrivateAddress
	}
	return d
}

// refusePrivateAddress is a dialer control function that refuses to connect to an address
// isPrivateIP reports, or to one it cannot parse.
func refusePrivateAddress(_ context.Context, _, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("refused the unparsable address %q: %w", address, ErrPrivateAddress)
	}
	if isPrivateIP(net.IP(ip.WithZone("").AsSlice())) {
		return fmt.Errorf("refused the private address %s: %w", ip, ErrPrivateAddress)
	}
	return nil
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

// CloseIdleConnections closes the idle connections of the client's transport. Only a client with its
// own transport, such as one NewHTTPClientWithRootCAs builds, should be released this way; the other
// clients share their factory's transport.
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
