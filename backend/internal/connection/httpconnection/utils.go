// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"net"
	"net/url"
	"strconv"
	"strings"
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
	parsed, ok := parseEndpointURL(raw)
	if !ok {
		return "", false
	}
	port := explicitPort(parsed)
	if port == "" {
		port = defaultPort(parsed.Scheme)
	}
	return parsed.Scheme + "://" + net.JoinHostPort(strings.ToLower(parsed.Hostname()), port), true
}

// parseEndpointURL parses an absolute http or https URL with a valid host and port. Userinfo is
// refused: it would be returned in clear on every read, outside the masking of secrets.
func parseEndpointURL(raw string) (*url.URL, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || !isValidHost(parsed.Hostname()) ||
		(parsed.Scheme != urlSchemeHTTP && parsed.Scheme != urlSchemeHTTPS) {
		return nil, false
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < minPort || number > maxPort {
			return nil, false
		}
	}
	return parsed, true
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

// isValidHost reports whether host is an IP literal or a DNS name.
func isValidHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	name := strings.TrimSuffix(host, ".")
	if name == "" || len(name) > maxHostLength {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if !isValidLabel(label) {
			return false
		}
	}
	return true
}

// isValidLabel reports whether label is a DNS label. An underscore is accepted, as service names
// in container networks use it.
func isValidLabel(label string) bool {
	if label == "" || len(label) > maxLabelLength || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, c := range label {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

// explicitPort returns the URL's port without leading zeros, or "" when it is absent or the
// scheme's default. The URL must have passed parseEndpointURL.
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
