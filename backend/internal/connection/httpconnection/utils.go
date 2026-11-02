// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/outboundauth/httpauth"
)

// SameOrigin reports whether two URLs share scheme, host and port, with default ports normalized.
// A URL that cannot be parsed matches nothing.
func SameOrigin(a, b string) bool {
	originA, ok := endpointOrigin(a)
	if !ok {
		return false
	}
	originB, ok := endpointOrigin(b)
	return ok && originA == originB
}

// endpointOrigin returns the normalized scheme://host:port of a URL.
func endpointOrigin(raw string) (string, bool) {
	parsed, ok := httpauth.ParseEndpointURL(raw)
	if !ok {
		return "", false
	}
	port := explicitPort(parsed)
	if port == "" {
		port = defaultPort(parsed.Scheme)
	}
	return parsed.Scheme + "://" + net.JoinHostPort(strings.ToLower(parsed.Hostname()), port), true
}

// canonicalURL renders a parsed endpoint URL with a lower-case host and without a default port, so
// that origins compare the same wherever the URL is read.
func canonicalURL(parsed *url.URL) string {
	canonical := *parsed
	host := strings.ToLower(parsed.Hostname())
	switch port := explicitPort(parsed); {
	case port != "":
		canonical.Host = net.JoinHostPort(host, port)
	case strings.Contains(host, ":"):
		canonical.Host = "[" + host + "]"
	default:
		canonical.Host = host
	}
	return canonical.String()
}

// explicitPort returns the URL's port without leading zeros, or "" when it is absent or the
// scheme's default. The URL must have passed httpauth.ParseEndpointURL.
func explicitPort(parsed *url.URL) string {
	number, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return ""
	}
	if port := strconv.Itoa(number); port != defaultPort(parsed.Scheme) {
		return port
	}
	return ""
}

// defaultPort returns the port a URL of the given scheme uses when it names none.
func defaultPort(scheme string) string {
	if scheme == urlSchemeHTTP {
		return defaultPortHTTP
	}
	return defaultPortHTTPS
}

// lowerCaseSet returns the given values as a set keyed in lower case.
func lowerCaseSet(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[strings.ToLower(value)] = struct{}{}
	}
	return set
}
