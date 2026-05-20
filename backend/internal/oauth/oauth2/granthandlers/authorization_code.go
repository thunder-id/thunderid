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
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/pkce"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// authorizationCodeGrantHandler handles the authorization code grant type.
type authorizationCodeGrantHandler struct {
	authzService    authz.AuthorizeServiceInterface
	tokenBuilder    tokenservice.TokenBuilderInterface
	attributeCache  attributecache.AttributeCacheServiceInterface
	resourceService resource.ResourceServiceInterface
}

// newAuthorizationCodeGrantHandler creates a new instance of AuthorizationCodeGrantHandler.
func newAuthorizationCodeGrantHandler(
	authzService authz.AuthorizeServiceInterface,
	tokenBuilder tokenservice.TokenBuilderInterface,
	attributeCache attributecache.AttributeCacheServiceInterface,
	resourceService resource.ResourceServiceInterface,
) GrantHandlerInterface {
	return &authorizationCodeGrantHandler{
		authzService:    authzService,
		tokenBuilder:    tokenBuilder,
		attributeCache:  attributeCache,
		resourceService: resourceService,
	}
}

// ValidateGrant validates the authorization code grant request.
func (h *authorizationCodeGrantHandler) ValidateGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *inboundmodel.OAuthClient) *model.ErrorResponse {
	if tokenRequest.GrantType == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Missing grant_type parameter",
		}
	}
	if constants.GrantType(tokenRequest.GrantType) != constants.GrantTypeAuthorizationCode {
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

	// TODO: Redirect uri is not mandatory when excluded in the authorize request and is valid scenario.
	//  This should be removed when supporting other means of authorization.
	if tokenRequest.RedirectURI == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Redirect URI is required",
		}
	}

	if errResp := resourceindicators.ValidateResourceURIs(tokenRequest.Resources); errResp != nil {
		return errResp
	}

	return nil
}

// HandleGrant processes the authorization code grant request and generates a token response.
func (h *authorizationCodeGrantHandler) HandleGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *inboundmodel.OAuthClient) (
	*model.TokenResponseDTO, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthorizationCodeGrantHandler"))

	// Retrieve and validate authorization code
	authCode, errResponse := h.retrieveAndValidateAuthCode(ctx, tokenRequest, oauthApp, logger)
	if errResponse != nil {
		return nil, errResponse
	}

	// Parse authorized scopes
	authorizedScopes := tokenservice.ParseScopes(authCode.Scopes)

	// Get user attributes from attribute cache
	attrs := make(map[string]interface{})
	if authCode.AttributeCacheID != "" {
		userAttributes, err := h.attributeCache.GetAttributeCache(ctx, authCode.AttributeCacheID)
		if err != nil {
			logger.Error("Failed to get user attributes from attribute cache. " + err.ErrorDescription.DefaultValue)
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to get user attributes from attribute cache",
			}
		}
		attrs = userAttributes.Attributes
	}

	// Always resolve the full set of resources authorized by the authz code. This is used to
	// compute the full audiences for the refresh token (RFC 8707 §5).
	fullRSes, errResp := resourceindicators.ResolveResourceServers(ctx, h.resourceService, authCode.Resources)
	if errResp != nil {
		return nil, errResp
	}
	fullAudiences, errResp := resourceindicators.ComposeAudiences(ctx, h.resourceService, authCode.ClientID,
		fullRSes, authorizedScopes)
	if errResp != nil {
		return nil, errResp
	}

	// Default: access token carries the full audiences and all authorized scopes.
	accessTokenAudiences := fullAudiences
	accessTokenScopes := authorizedScopes

	// Per RFC 8707 §2.1, the token request MAY narrow the resource set. When narrowing occurs,
	// audiences and scopes are downscoped to the requested subset.
	if len(tokenRequest.Resources) > 0 {
		// When the auth code had explicit resources, narrow by filtering the already-resolved full set.
		// When the auth code had no explicit resources, resolve the token-request resources directly.
		var narrowedRSes []*resource.ResourceServer
		if len(authCode.Resources) > 0 {
			narrowedRSes = resourceindicators.FilterByIdentifiers(fullRSes, tokenRequest.Resources)
		} else {
			narrowedRSes, errResp = resourceindicators.ResolveResourceServers(
				ctx, h.resourceService, tokenRequest.Resources)
			if errResp != nil {
				return nil, errResp
			}
		}
		accessTokenAudiences, errResp = resourceindicators.ComposeAudiences(ctx, h.resourceService,
			authCode.ClientID, narrowedRSes, authorizedScopes)
		if errResp != nil {
			return nil, errResp
		}

		// Downscope: retain OIDC scopes unchanged; filter non-OIDC scopes to only those valid on
		// the narrowed RS set.
		oidcScopes, nonOidcScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(
			strings.Join(authorizedScopes, " "), oauthApp.ScopeClaims)
		rsValidScopes, rsErr := resourceindicators.ComputeRSValidScopes(
			ctx, h.resourceService, narrowedRSes, nonOidcScopes)
		if rsErr != nil {
			return nil, rsErr
		}
		oidcScopes = append(oidcScopes, resourceindicators.UnionScopes(rsValidScopes)...)
		accessTokenScopes = oidcScopes
	}

	// Generate access token using tokenBuilder (attributes will be filtered in BuildAccessToken)
	accessToken, err := h.tokenBuilder.BuildAccessToken(&tokenservice.AccessTokenBuildContext{
		Context:          ctx,
		Subject:          authCode.AuthorizedUserID,
		Audiences:        accessTokenAudiences,
		ClientID:         tokenRequest.ClientID,
		Scopes:           accessTokenScopes,
		UserAttributes:   attrs,
		AttributeCacheID: authCode.AttributeCacheID,
		GrantType:        string(constants.GrantTypeAuthorizationCode),
		OAuthApp:         oauthApp,
		ClaimsRequest:    authCode.ClaimsRequest,
		ClaimsLocales:    authCode.ClaimsLocales,
	})
	if err != nil {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}

	// Carry the full (un-narrowed) audiences in OriginalAudiences so the token service can
	// pass them to IssueRefreshToken (RFC 8707 §5 — refresh token preserves original audience).
	accessToken.OriginalAudiences = fullAudiences

	// Build token response
	tokenResponse := &model.TokenResponseDTO{
		AccessToken: *accessToken,
	}

	// Generate ID token if 'openid' scope is present
	if slices.Contains(accessTokenScopes, constants.ScopeOpenID) {
		idToken, err := h.tokenBuilder.BuildIDToken(&tokenservice.IDTokenBuildContext{
			Context:        ctx,
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
			logger.Error("Failed to generate ID token", log.Error(err))
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
	oauthApp *inboundmodel.OAuthClient,
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
			logger.Debug("PKCE validation failed", log.Error(err))
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

	// redirect_uri is not mandatory in certain scenarios. Should match if provided with the authorization.
	if code.RedirectURI != "" && tokenRequest.RedirectURI != code.RedirectURI {
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
