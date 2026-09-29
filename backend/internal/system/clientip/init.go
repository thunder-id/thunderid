// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package clientip resolves the IP address of the client behind a request.
//
// Behind a reverse proxy the connecting address is the proxy's, so the client IP is read from the
// header the proxy reports it in. A header is only read when the connecting address is in a
// configured trusted proxy's range, since any other caller could write it. The resolved IP is for
// display and logging, never for a security decision.
package clientip

import (
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Initialize parses the trusted proxies and returns a ResolverInterface over them.
func Initialize(proxies []engineconfig.TrustedProxyConfig) (ResolverInterface, error) {
	r, err := newResolver(proxies)
	if err != nil {
		return nil, err
	}
	return r, nil
}
