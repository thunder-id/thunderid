// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package tokenservice

import (
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the token service components (builder and validator).
// Returns both TokenBuilderInterface and TokenValidatorInterface for centralized token operations.
func Initialize(
	cfg oauthconfig.Config,
	jwtService jwt.JWTServiceInterface,
	jweService jwe.JWEServiceInterface,
	resolver *jwksresolver.Resolver,
	idpService providers.IDPProvider,
	enforcementService revocation.EnforcementServiceInterface,
	jtiStore jti.JTIStoreInterface,
	actorProvider providers.ActorProvider,
) (TokenBuilderInterface, TokenValidatorInterface) {
	tokenBuilder := newTokenBuilder(cfg, jwtService, jweService, resolver, actorProvider)
	tokenValidator := newTokenValidator(cfg, jwtService, idpService, enforcementService, jtiStore)
	return tokenBuilder, tokenValidator
}
