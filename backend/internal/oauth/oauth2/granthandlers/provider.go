// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"slices"

	"github.com/thunder-id/thunderid/internal/attributecache"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/ciba"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// GrantHandlerProviderInterface defines the interface for the grant handler provider.
type GrantHandlerProviderInterface interface {
	GetGrantHandler(grantType providers.GrantType) (GrantHandlerInterface, error)
}

// GrantHandlerProvider implements the GrantHandlerProviderInterface.
type GrantHandlerProvider struct {
	clientCredentialsGrantHandler GrantHandlerInterface
	authorizationCodeGrantHandler GrantHandlerInterface
	refreshTokenGrantHandler      GrantHandlerInterface
	tokenExchangeGrantHandler     GrantHandlerInterface
	cibaGrantHandler              GrantHandlerInterface
	jwtBearerGrantHandler         GrantHandlerInterface
}

// newGrantHandlerProvider creates a new instance of GrantHandlerProvider.
func newGrantHandlerProvider(
	jwtService jwt.JWTServiceInterface,
	authzService authz.AuthorizeServiceInterface,
	tokenBuilder tokenservice.TokenBuilderInterface,
	tokenValidator tokenservice.TokenValidatorInterface,
	attrCacheService attributecache.AttributeCacheServiceInterface,
	ouService providers.OrganizationUnitProvider,
	rbacAuthzService providers.AuthorizationProvider,
	actorProvider providers.ActorProvider,
	resourceService providers.ResourceServerProvider,
	cibaService ciba.CIBAServiceInterface,
	refreshTokenRevoker revocation.RefreshTokenRevokerInterface,
	criteriaRevoker revocation.CriteriaRevokerInterface,
	cfg oauthconfig.Config,
) GrantHandlerProviderInterface {
	allowedGrantTypes := cfg.OAuth.AllowedGrantTypes
	grantProvider := &GrantHandlerProvider{}
	if isGrantTypeAllowed(allowedGrantTypes, providers.GrantTypeClientCredentials) {
		grantProvider.clientCredentialsGrantHandler = newClientCredentialsGrantHandler(
			tokenBuilder, ouService, rbacAuthzService, actorProvider, resourceService, cfg)
	}
	if isGrantTypeAllowed(allowedGrantTypes, providers.GrantTypeAuthorizationCode) {
		grantProvider.authorizationCodeGrantHandler = newAuthorizationCodeGrantHandler(
			authzService, tokenBuilder, attrCacheService, resourceService, cfg)
	}
	if isGrantTypeAllowed(allowedGrantTypes, providers.GrantTypeRefreshToken) {
		grantProvider.refreshTokenGrantHandler = newRefreshTokenGrantHandler(
			jwtService, tokenBuilder, tokenValidator, attrCacheService, resourceService,
			rbacAuthzService, actorProvider, refreshTokenRevoker, criteriaRevoker, cfg)
	}
	if isGrantTypeAllowed(allowedGrantTypes, providers.GrantTypeTokenExchange) {
		grantProvider.tokenExchangeGrantHandler = newTokenExchangeGrantHandler(
			tokenBuilder, tokenValidator, resourceService, cfg)
	}
	if isGrantTypeAllowed(allowedGrantTypes, providers.GrantTypeCIBA) {
		grantProvider.cibaGrantHandler = newCIBAGrantHandler(cibaService, tokenBuilder, attrCacheService,
			resourceService, cfg)
	}
	if isGrantTypeAllowed(allowedGrantTypes, providers.GrantTypeJWTBearer) {
		grantProvider.jwtBearerGrantHandler = newJWTBearerGrantHandler(
			tokenBuilder, tokenValidator, resourceService, cfg)
	}
	return grantProvider
}

// isGrantTypeAllowed reports whether the given grant type may be registered. An empty
// allow list means no restriction is configured, so every grant type is allowed.
func isGrantTypeAllowed(allowedGrantTypes []string, grantType providers.GrantType) bool {
	if len(allowedGrantTypes) == 0 {
		return true
	}
	return slices.Contains(allowedGrantTypes, string(grantType))
}

// GetGrantHandler returns the appropriate grant handler for the given grant type.
func (p *GrantHandlerProvider) GetGrantHandler(grantType providers.GrantType) (GrantHandlerInterface, error) {
	var handler GrantHandlerInterface
	switch grantType {
	case providers.GrantTypeClientCredentials:
		handler = p.clientCredentialsGrantHandler
	case providers.GrantTypeAuthorizationCode:
		handler = p.authorizationCodeGrantHandler
	case providers.GrantTypeRefreshToken:
		handler = p.refreshTokenGrantHandler
	case providers.GrantTypeTokenExchange:
		handler = p.tokenExchangeGrantHandler
	case providers.GrantTypeCIBA:
		handler = p.cibaGrantHandler
	case providers.GrantTypeJWTBearer:
		handler = p.jwtBearerGrantHandler
	}
	if handler == nil {
		return nil, constants.UnSupportedGrantTypeError
	}
	return handler, nil
}
