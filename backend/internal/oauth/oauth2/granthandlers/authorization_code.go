/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"context"
	"slices"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/pkce"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// authorizationCodeGrantHandler handles the authorization code grant type.
type authorizationCodeGrantHandler struct {
	authzService        authz.AuthorizeServiceInterface
	tokenBuilder        tokenservice.TokenBuilderInterface
	attributeCache      attributecache.AttributeCacheServiceInterface
	resourceService     providers.ResourceServerProvider
	serverConfigService serverconfig.ServerConfigService
}

// newAuthorizationCodeGrantHandler creates a new instance of AuthorizationCodeGrantHandler.
func newAuthorizationCodeGrantHandler(
	authzService authz.AuthorizeServiceInterface,
	tokenBuilder tokenservice.TokenBuilderInterface,
	attributeCache attributecache.AttributeCacheServiceInterface,
	resourceService providers.ResourceServerProvider,
	serverConfigService serverconfig.ServerConfigService,
) GrantHandlerInterface {
	return &authorizationCodeGrantHandler{
		authzService:        authzService,
		tokenBuilder:        tokenBuilder,
		attributeCache:      attributeCache,
		resourceService:     resourceService,
		serverConfigService: serverConfigService,
	}
}

// ValidateGrant validates the authorization code grant request.
func (h *authorizationCodeGrantHandler) ValidateGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) *model.ErrorResponse {
	if tokenRequest.GrantType == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Missing grant_type parameter",
		}
	}
	if providers.GrantType(tokenRequest.GrantType) != providers.GrantTypeAuthorizationCode {
		return &model.ErrorResponse{
			Error:            constants.ErrorUnsupportedGrantType,
			ErrorDescription: "Unsupported grant type",
		}
	}
	if tokenRequest.Code == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Authorization code is required",
		}
	}
	if tokenRequest.ClientID == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidClient,
			ErrorDescription: "client_id is required",
		}
	}

	if errResp := resourceindicators.ValidateResourceURIs(tokenRequest.Resources); errResp != nil {
		return errResp
	}

	return nil
}

// HandleGrant processes the authorization code grant request and generates a token response.
func (h *authorizationCodeGrantHandler) HandleGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) (
	*model.TokenResponseDTO, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthorizationCodeGrantHandler"))

	// Retrieve and validate authorization code
	authCode, errResponse := h.retrieveAndValidateAuthCode(ctx, tokenRequest, oauthApp, logger)
	if errResponse != nil {
		return nil, errResponse
	}

	if errResp := dpop.VerifyProofBinding(ctx, authCode.DPoPJkt, "authorization code"); errResp != nil {
		return nil, errResp
	}

	// Parse authorized scopes
	authorizedScopes := tokenservice.ParseScopes(authCode.Scopes)

	// Get user attributes from attribute cache
	attrs := make(map[string]interface{})
	if authCode.AttributeCacheID != "" {
		userAttributes, err := h.attributeCache.GetAttributeCache(ctx, authCode.AttributeCacheID)
		if err != nil {
			logger.Error(ctx,
				"Failed to get user attributes from attribute cache. "+err.ErrorDescription.DefaultValue)
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to get user attributes from attribute cache",
			}
		}
		attrs = userAttributes.Attributes
	}

	// Bind the token to a single target resource server. Prefer the token request's resource
	// (validated as a subset of the code's resources); otherwise use the resource recorded on the
	// authorization code; otherwise fall back to the configured default resource server.
	effectiveResources := tokenRequest.Resources
	if len(effectiveResources) == 0 {
		effectiveResources = authCode.Resources
	}

	// Retain OIDC scopes unchanged; downscope non-OIDC scopes to permissions defined on the target RS.
	oidcScopes, nonOidcScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(
		strings.Join(authorizedScopes, " "), oauthApp.ScopeClaims)
	targetRS, errResp := resourceindicators.ResolveAudienceBinding(
		ctx, h.resourceService, h.serverConfigService, effectiveResources, nonOidcScopes)
	if errResp != nil {
		return nil, errResp
	}

	var accessTokenAudiences, accessTokenScopes []string
	if targetRS == nil {
		// OIDC-only (or scopeless) request with no resource: the token is not bound to a resource
		// server, so its audience is the app's configured default audiences (falling back to the
		// client_id) and it carries only the OIDC scopes.
		accessTokenAudiences = []string{oauthApp.ResolveDefaultAudience(tokenRequest.ClientID)}
		accessTokenScopes = oidcScopes
	} else {
		downscopedNonOidc, dErr := resourceindicators.DownscopeToResourceServer(
			ctx, h.resourceService, targetRS.ID, nonOidcScopes)
		if dErr != nil {
			return nil, dErr
		}
		accessTokenAudiences = []string{targetRS.Identifier}
		accessTokenScopes = make([]string, 0, len(oidcScopes)+len(downscopedNonOidc))
		accessTokenScopes = append(accessTokenScopes, oidcScopes...)
		accessTokenScopes = append(accessTokenScopes, downscopedNonOidc...)
	}

	// Generate access token using tokenBuilder (attributes will be filtered in BuildAccessToken)
	userSubConfig := oauthApp.UserAccessTokenConfig()
	accessTokenCtx := &tokenservice.AccessTokenBuildContext{
		Subject:           authCode.AuthorizedUserID,
		Audiences:         accessTokenAudiences,
		ClientID:          tokenRequest.ClientID,
		Scopes:            accessTokenScopes,
		SubjectAttributes: tokenservice.FilterAttributesByAllowList(attrs, userSubConfig),
		AttributeCacheID:  authCode.AttributeCacheID,
		GrantType:         string(providers.GrantTypeAuthorizationCode),
		OAuthApp:          oauthApp,
		ClaimsRequest:     authCode.ClaimsRequest,
		ClaimsLocales:     authCode.ClaimsLocales,
		ValidityPeriod:    userSubConfig.ValidityPeriodOrZero(),
		DPoPJkt:           dpop.GetJkt(ctx),
	}
	if oauthApp.ShouldAppendActorClaim() {
		accessTokenCtx.ActorClaims = &tokenservice.SubjectTokenClaims{Sub: oauthApp.ID}
	}
	accessToken, err := h.tokenBuilder.BuildAccessToken(ctx, accessTokenCtx)
	if err != nil {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}

	// The refresh token preserves this single audience for continuity.
	accessToken.OriginalAudiences = accessTokenAudiences

	// Build token response
	tokenResponse := &model.TokenResponseDTO{
		AccessToken: *accessToken,
	}

	// Generate ID token if 'openid' scope is present
	if slices.Contains(accessTokenScopes, constants.ScopeOpenID) {
		idToken, err := h.tokenBuilder.BuildIDToken(ctx, &tokenservice.IDTokenBuildContext{
			Subject:        authCode.AuthorizedUserID,
			Audience:       tokenRequest.ClientID,
			Scopes:         accessTokenScopes,
			UserAttributes: attrs,
			AuthTime:       authCode.TimeCreated.Unix(),
			OAuthApp:       oauthApp,
			ClaimsRequest:  authCode.ClaimsRequest,
			Nonce:          authCode.Nonce,
			CompletedACR:   authCode.CompletedACR,
		})
		if err != nil {
			logger.Error(ctx, "Failed to generate ID token", log.Error(err))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to generate token",
			}
		}
		tokenResponse.IDToken = *idToken
	}

	return tokenResponse, nil
}

func (h *authorizationCodeGrantHandler) retrieveAndValidateAuthCode(
	ctx context.Context,
	tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient,
	logger *log.Logger,
) (*authz.AuthorizationCode, *model.ErrorResponse) {
	authCode, codeErr := h.authzService.GetAuthorizationCodeDetails(ctx, tokenRequest.ClientID, tokenRequest.Code)
	if codeErr != nil {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid authorization code",
		}
	}

	// Validate the retrieved authorization code
	errResponse := validateAuthorizationCode(tokenRequest, *authCode)
	if errResponse != nil && errResponse.Error != "" {
		return nil, errResponse
	}

	// Validate PKCE if required or if code challenge was provided during authorization
	if oauthApp.RequiresPKCE() || authCode.CodeChallenge != "" {
		if tokenRequest.CodeVerifier == "" {
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidRequest,
				ErrorDescription: "code_verifier is required",
			}
		}

		// Validate PKCE
		if err := pkce.ValidatePKCE(authCode.CodeChallenge, authCode.CodeChallengeMethod,
			tokenRequest.CodeVerifier); err != nil {
			logger.Debug(ctx, "PKCE validation failed", log.Error(err))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidGrant,
				ErrorDescription: "Invalid code verifier",
			}
		}
	}
	return authCode, nil
}

// validateAuthorizationCode validates the authorization code against the token request.
func validateAuthorizationCode(tokenRequest *model.TokenRequest,
	code authz.AuthorizationCode) *model.ErrorResponse {
	if tokenRequest.ClientID != code.ClientID {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid authorization code",
		}
	}

	// RFC 6749 §4.1.3: required only if included in the authorize request.
	if code.RedirectURIProvided && tokenRequest.RedirectURI != code.RedirectURI {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid redirect URI",
		}
	}

	// If the authorization request included resources, the token request may either omit
	// resources entirely or supply a subset of those previously authorized.
	if len(code.Resources) > 0 && len(tokenRequest.Resources) > 0 {
		for _, r := range tokenRequest.Resources {
			if !slices.Contains(code.Resources, r) {
				return &model.ErrorResponse{
					Error:            constants.ErrorInvalidTarget,
					ErrorDescription: "Resource parameter mismatch",
				}
			}
		}
	}

	if code.ExpiryTime.Before(time.Now()) {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Expired authorization code",
		}
	}

	return nil
}
