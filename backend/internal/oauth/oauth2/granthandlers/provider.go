/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
	"github.com/thunder-id/thunderid/internal/serverconfig"
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
	serverConfigService serverconfig.ServerConfigService,
	cibaService ciba.CIBAServiceInterface,
	refreshTokenRevoker revocation.RefreshTokenRevokerInterface,
	cfg oauthconfig.Config,
) GrantHandlerProviderInterface {
	allowedGrantTypes := cfg.OAuth.AllowedGrantTypes
	grantProvider := &GrantHandlerProvider{}
	if isGrantTypeAllowed(allowedGrantTypes, providers.GrantTypeClientCredentials) {
		grantProvider.clientCredentialsGrantHandler = newClientCredentialsGrantHandler(
			tokenBuilder, ouService, rbacAuthzService, actorProvider, resourceService, serverConfigService)
	}
	if isGrantTypeAllowed(allowedGrantTypes, providers.GrantTypeAuthorizationCode) {
		grantProvider.authorizationCodeGrantHandler = newAuthorizationCodeGrantHandler(
			authzService, tokenBuilder, attrCacheService, resourceService, serverConfigService)
	}
	if isGrantTypeAllowed(allowedGrantTypes, providers.GrantTypeRefreshToken) {
		grantProvider.refreshTokenGrantHandler = newRefreshTokenGrantHandler(
			jwtService, tokenBuilder, tokenValidator, attrCacheService, resourceService,
			serverConfigService, refreshTokenRevoker, cfg)
	}
	if isGrantTypeAllowed(allowedGrantTypes, providers.GrantTypeTokenExchange) {
		grantProvider.tokenExchangeGrantHandler = newTokenExchangeGrantHandler(
			tokenBuilder, tokenValidator, rbacAuthzService, actorProvider, resourceService, serverConfigService)
	}
	if isGrantTypeAllowed(allowedGrantTypes, providers.GrantTypeCIBA) {
		grantProvider.cibaGrantHandler = newCIBAGrantHandler(cibaService, tokenBuilder, attrCacheService,
			resourceService)
	}
	if isGrantTypeAllowed(allowedGrantTypes, providers.GrantTypeJWTBearer) {
		grantProvider.jwtBearerGrantHandler = newJWTBearerGrantHandler(
			tokenBuilder, tokenValidator, resourceService, serverConfigService)
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
