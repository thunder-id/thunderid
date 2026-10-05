// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"github.com/thunder-id/thunderid/internal/attributecache"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	oauth2authz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/ciba"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the grant handler provider. oauth2AuthzService is created by the
// caller (oauth/init.go) so it can also be passed to the callback dispatcher without
// granthandlers needing to own authz initialization or expose it.
func Initialize(
	jwtService jwt.JWTServiceInterface,
	oauth2AuthzService oauth2authz.AuthorizeServiceInterface,
	tokenBuilder tokenservice.TokenBuilderInterface,
	tokenValidator tokenservice.TokenValidatorInterface,
	attrCacheService attributecache.AttributeCacheServiceInterface,
	ouService providers.OrganizationUnitProvider,
	authzService providers.AuthorizationProvider,
	actorProvider providers.ActorProvider,
	resourceService providers.ResourceServerProvider,
	cibaService ciba.CIBAServiceInterface,
	refreshTokenRevoker revocation.RefreshTokenRevokerInterface,
	criteriaRevoker revocation.CriteriaRevokerInterface,
	graceConfigReader OAuthServerConfigReader,
	cfg oauthconfig.Config,
) GrantHandlerProviderInterface {
	return newGrantHandlerProvider(
		jwtService,
		oauth2AuthzService,
		tokenBuilder,
		tokenValidator,
		attrCacheService,
		ouService,
		authzService,
		actorProvider,
		resourceService,
		cibaService,
		refreshTokenRevoker,
		criteriaRevoker,
		graceConfigReader,
		cfg,
	)
}
