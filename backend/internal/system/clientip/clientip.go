// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package clientip

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// ResolverInterface resolves the client IP of a request from the configured trusted proxies.
type ResolverInterface interface {
	Resolve(req *http.Request) string
}

// parsedProxy is a trusted proxy with its range parsed.
type parsedProxy struct {
	cidr   netip.Prefix
	header string
}

// resolver is the ResolverInterface implementation over parsed trusted proxies.
type resolver struct {
	proxies []*parsedProxy
	logger  *log.Logger
}

// newResolver parses the trusted proxies once, so a request only matches against parsed ranges.
func newResolver(proxies []engineconfig.TrustedProxyConfig) (*resolver, error) {
	r := &resolver{logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ClientIPResolver"))}
	for i, p := range proxies {
		cidr, err := p.Range()
		if err != nil {
			return nil, fmt.Errorf("invalid server.trusted_proxies[%d]: %w", i, err)
		}
		header := p.Header
		if header == "" {
			header = constants.XForwardedForHeaderName
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
func (r *resolver) Resolve(req *http.Request) string {
	remote := parseAddr(req.RemoteAddr)
	if !remote.IsValid() {
		return ""
	}
	proxy := r.match(remote)
	if proxy == nil {
		return remote.String()
	}

	var ip netip.Addr
	if proxy.header == constants.XForwardedForHeaderName {
		ip = r.walkForwardedFor(req.Header.Values(proxy.header))
	} else {
		ip = singleIP(req.Header.Values(proxy.header))
	}
	if !ip.IsValid() {
		// Debug, not warn: a client can send a header that gives no valid IP, so this must not fill the
		// log. A proxy that never sets the header shows here when an operator turns on debug logging.
		r.logger.Debug(req.Context(),
			"Client IP fell back to the connecting address: trusted proxy did not send a valid header",
			log.String("trustedProxy", proxy.cidr.String()), log.String("header", proxy.header))
		return remote.String()
	}
	return ip.String()
}

// match returns the first trusted proxy whose range contains addr, or nil when none does.
func (r *resolver) match(addr netip.Addr) *parsedProxy {
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
func (r *resolver) walkForwardedFor(values []string) netip.Addr {
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
