// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package clientip

import (
	"net/netip"
	"strings"
)

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
