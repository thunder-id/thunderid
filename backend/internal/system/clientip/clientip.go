// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package clientip resolves the IP address of the client behind a request and carries it on the
// request context.
//
// Behind a reverse proxy the connecting address is the proxy's, so the client IP is read from the
// header the proxy reports it in. A header is only read when the connecting address is in a
// configured trusted proxy's range, since any other caller could write it. The resolved IP is for
// display and logging, never for a security decision.
package clientip

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"sync"
)

// forwardedFor is the header a trusted proxy reports the client IP in when none is set.
const forwardedFor = "X-Forwarded-For"

// ctxKey is the private context key under which the client IP is stored.
type ctxKey struct{}

// WithIP returns a context carrying the given client IP.
func WithIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ctxKey{}, ip)
}

// FromContext returns the client IP carried by the context, or "" when there is none.
func FromContext(ctx context.Context) string {
	ip, _ := ctx.Value(ctxKey{}).(string)
	return ip
}

// TrustedProxy is a reverse proxy, or a range of them, allowed to report the client IP, and the header
// it reports it in. An empty Header means X-Forwarded-For.
type TrustedProxy struct {
	CIDR   string
	Header string
}

// FallbackFunc is called the first time a trusted proxy's header gives no valid IP, so the fallback to
// the connecting address can be reported once per proxy instead of on every request.
type FallbackFunc func(ctx context.Context, cidr netip.Prefix, header string)

// parsedProxy is a TrustedProxy with its range parsed.
type parsedProxy struct {
	cidr         netip.Prefix
	header       string
	fallbackOnce sync.Once
}

// Resolver resolves the client IP of a request from the configured trusted proxies.
type Resolver struct {
	proxies    []*parsedProxy
	onFallback FallbackFunc
}

// NewResolver parses the trusted proxies once, so a request only matches against parsed ranges.
// onFallback may be nil.
func NewResolver(proxies []TrustedProxy, onFallback FallbackFunc) (*Resolver, error) {
	r := &Resolver{onFallback: onFallback}
	for i, p := range proxies {
		cidr, err := netip.ParsePrefix(p.CIDR)
		if err != nil {
			return nil, fmt.Errorf("invalid server.trusted_proxies[%d]: %w", i, err)
		}
		header := p.Header
		if header == "" {
			header = forwardedFor
		}
		r.proxies = append(r.proxies, &parsedProxy{
			cidr:   cidr,
			header: http.CanonicalHeaderKey(header),
		})
	}
	return r, nil
}

// Resolve returns the client IP of the request, or "" when its connecting address is not an IP.
//
// The connecting address is the client IP unless it is in a trusted proxy's range. Then the header of
// the first matching proxy is read: X-Forwarded-For is walked from right to left, any other header is
// read as one IP. When that header gives no valid IP, the connecting address is used.
func (r *Resolver) Resolve(req *http.Request) string {
	remote := parseAddr(req.RemoteAddr)
	if !remote.IsValid() {
		return ""
	}
	proxy := r.match(remote)
	if proxy == nil {
		return remote.String()
	}

	var ip netip.Addr
	if proxy.header == forwardedFor {
		ip = r.walkForwardedFor(req.Header.Values(proxy.header))
	} else {
		ip = singleIP(req.Header.Values(proxy.header))
	}
	if !ip.IsValid() {
		if r.onFallback != nil {
			proxy.fallbackOnce.Do(func() { r.onFallback(req.Context(), proxy.cidr, proxy.header) })
		}
		return remote.String()
	}
	return ip.String()
}

// match returns the first trusted proxy whose range contains addr, or nil when none does.
func (r *Resolver) match(addr netip.Addr) *parsedProxy {
	for _, p := range r.proxies {
		if p.cidr.Contains(addr) {
			return p
		}
	}
	return nil
}

// walkForwardedFor reads X-Forwarded-For from right to left, all header lines joined, and returns the
// first address not in a trusted proxy's range. Each proxy appends the address it received the request
// from, so anything left of that address could have been written by the client and is never read. An
// invalid address reached before it rejects the header. When every address is trusted, the leftmost one
// is returned.
func (r *Resolver) walkForwardedFor(values []string) netip.Addr {
	if len(values) == 0 {
		return netip.Addr{}
	}
	hops := strings.Split(strings.Join(values, ","), ",")
	var leftmost netip.Addr
	for i := len(hops) - 1; i >= 0; i-- {
		hop := parseAddr(strings.TrimSpace(hops[i]))
		if !hop.IsValid() {
			return netip.Addr{}
		}
		if r.match(hop) == nil {
			return hop
		}
		leftmost = hop
	}
	return leftmost
}

// singleIP reads a header that carries one IP, such as X-Real-IP. A header sent more than once is
// rejected, since only one of the lines can have been set by the proxy.
func singleIP(values []string) netip.Addr {
	if len(values) != 1 {
		return netip.Addr{}
	}
	return parseIP(strings.TrimSpace(values[0]))
}

// parseAddr parses an address with or without a port, such as "203.0.113.10:443",
// "[2001:db8::1]:443", "203.0.113.10" or "[2001:db8::1]".
func parseAddr(s string) netip.Addr {
	if addrPort, err := netip.ParseAddrPort(s); err == nil {
		return addrPort.Addr().Unmap().WithZone("")
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = s[1 : len(s)-1]
	}
	return parseIP(s)
}

// parseIP parses an IP address in its canonical form: an IPv4-mapped IPv6 address becomes IPv4 and any
// IPv6 zone is dropped, so it matches an IPv4 range. It returns the zero Addr when s is not an IP.
func parseIP(s string) netip.Addr {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap().WithZone("")
}
