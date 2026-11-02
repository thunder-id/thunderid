// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpauth

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

const (
	urlSchemeHTTP  = "http"
	urlSchemeHTTPS = "https"
	minPort        = 1
	maxPort        = 65535

	// DNS limits on a host name and on each of its labels.
	maxHostLength  = 253
	maxLabelLength = 63
)

// ParseEndpointURL parses an absolute http or https URL with a valid host and port. Userinfo is
// refused: it would be returned in clear on every read, outside the masking of secrets.
func ParseEndpointURL(raw string) (*url.URL, bool) {
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

// IsLoopbackHost reports whether a host names the loopback interface.
func IsLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
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
