// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package jwt provides functionalities for handling JSON Web Tokens (JWTs).
package jwt

import (
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the JWT service. A JWKS an administrator configures is fetched through a
// client from outbound, a client's JWKS URI through one from strictOutbound, which always refuses
// private addresses, and a JWKS the deployment configuration sets, such as the trusted issuer's,
// through outbound's unguarded client.
func Initialize(
	runtimeProvider providers.RuntimeCryptoProvider, cfg joseconfig.Config,
	outbound, strictOutbound httpservice.OutboundClientFactoryInterface,
) (JWTServiceInterface, error) {
	return newJWTService(outbound.NewHTTPClient(jwksFetchTimeout), strictOutbound.NewHTTPClient(jwksFetchTimeout),
		outbound.NewUnguardedHTTPClient(jwksFetchTimeout), runtimeProvider, cfg)
}
