// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package jwt provides functionalities for handling JSON Web Tokens (JWTs).
package jwt

import (
	"time"

	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the JWT service.
func Initialize(
	runtimeProvider providers.RuntimeCryptoProvider, cfg joseconfig.Config,
) (JWTServiceInterface, error) {
	// JWKS URLs come from the connection config, so they get the SSRF dial guard.
	httpClient := httpservice.NewHTTPClient(httpservice.HTTPClientConfig{
		Timeout:   10 * time.Second,
		GuardSSRF: true,
	})
	return newJWTService(httpClient, runtimeProvider, cfg)
}
