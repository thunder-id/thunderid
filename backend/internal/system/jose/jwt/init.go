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

// jwksFetchTimeout bounds the retrieval of a JWKS.
const jwksFetchTimeout = 10 * time.Second

// Initialize initializes the JWT service. A JWKS an administrator or a client configures is fetched
// through a client from outbound; one the deployment configuration sets, such as the trusted
// issuer's, through outbound's unguarded client.
func Initialize(
	runtimeProvider providers.RuntimeCryptoProvider, cfg joseconfig.Config,
	outbound httpservice.OutboundClientProviderInterface,
) (JWTServiceInterface, error) {
	return newJWTService(outbound.NewHTTPClient(jwksFetchTimeout),
		outbound.NewUnguardedHTTPClient(jwksFetchTimeout), runtimeProvider, cfg)
}
