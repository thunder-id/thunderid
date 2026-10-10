// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package restprovider implements an authentication provider that delegates to an external service over REST.
package restprovider

import (
	"errors"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	systemhttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Name is the name of the built-in REST authn provider.
const Name = "rest"

// Initialize builds the REST authentication provider from its config block. It
// validates that base_url is set and applies defaults for the request timeout and
// correlation-ID header. Enablement is the caller's concern.
func Initialize(cfg config.RestConfig) (providers.AuthnProviderInterface, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("base_url is required when the rest authn provider is enabled")
	}
	timeout := 10 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}
	correlationIDHeader := cfg.CorrelationIDHeader
	if correlationIDHeader == "" {
		correlationIDHeader = serverconst.CorrelationIDHeaderName
	}
	// The REST provider dials the admin-configured base URL, which may be an
	// internal host, so it keeps no SSRF dial guard.
	httpClient := systemhttp.NewHTTPClient(systemhttp.HTTPClientConfig{
		Timeout: timeout,
	})
	return newRestAuthnProvider(cfg.BaseURL, cfg.Security.APIKey, correlationIDHeader, httpClient), nil
}
