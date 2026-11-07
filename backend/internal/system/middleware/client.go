// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net"
	"net/http"
	"net/netip"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// maxUserAgentLength is the most characters of a user agent the request context carries.
const maxUserAgentLength = 256

// unroutableRanges are the special-purpose ranges that netip.Addr.IsGlobalUnicast and IsPrivate still accept
// but that no client on the internet connects from: carrier-grade NAT (RFC 6598), protocol assignments,
// documentation, benchmarking, the deprecated 6to4 relay, the reserved class E block and the IPv6
// documentation and discard prefixes.
var unroutableRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("3fff::/20"),
}

// ClientMiddleware records the client of the request on the request context: its TCP peer address and
// its user agent cut to 256 characters, only when the peer is a public address. Behind a proxy, an
// ingress or a load balancer, and in a local run, the peer is a private or loopback address that names
// no client, so nothing is recorded. Forwarded headers are caller controlled and are never read.
func ClientMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		address, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			address = r.RemoteAddr
		}
		if peer, ok := publicAddress(address); ok {
			userAgent := utils.TruncateRunes(r.UserAgent(), maxUserAgentLength)
			r = r.WithContext(syscontext.WithClient(r.Context(), peer, userAgent))
		}
		next.ServeHTTP(w, r)
	})
}

// publicAddress returns host as an address, an IPv4 address mapped into IPv6 as IPv4, and whether it is
// a public unicast address. Loopback, link-local, multicast, unspecified, private, unique local, shared,
// reserved, documentation and unparsable addresses, such as a unix socket peer, are not public.
func publicAddress(host string) (string, bool) {
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "", false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || isUnroutable(addr) {
		return "", false
	}
	return addr.String(), true
}

// isUnroutable reports whether addr is in one of the special-purpose ranges no internet client uses.
func isUnroutable(addr netip.Addr) bool {
	for _, prefix := range unroutableRanges {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
