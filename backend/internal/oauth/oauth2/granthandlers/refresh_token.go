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
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// refreshTokenGrantHandler handles the refresh token grant type.
type refreshTokenGrantHandler struct {
	jwtService       jwt.JWTServiceInterface
	tokenBuilder     tokenservice.TokenBuilderInterface
	tokenValidator   tokenservice.TokenValidatorInterface
	attrCacheService attributecache.AttributeCacheServiceInterface
	resourceService  resource.ResourceServiceInterface
}

// newRefreshTokenGrantHandler creates a new instance of RefreshTokenGrantHandler.
func newRefreshTokenGrantHandler(
	jwtService jwt.JWTServiceInterface,
	tokenBuilder tokenservice.TokenBuilderInterface,
	tokenValidator tokenservice.TokenValidatorInterface,
	attrCacheService attributecache.AttributeCacheServiceInterface,
	resourceService resource.ResourceServiceInterface,
) RefreshTokenGrantHandlerInterface {
	return &refreshTokenGrantHandler{
		jwtService:       jwtService,
		tokenBuilder:     tokenBuilder,
		tokenValidator:   tokenValidator,
		attrCacheService: attrCacheService,
		resourceService:  resourceService,
	}
}

// ValidateGrant validates the refresh token grant request.
func (h *refreshTokenGrantHandler) ValidateGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *inboundmodel.OAuthClient) *model.ErrorResponse {
	if constants.GrantType(tokenRequest.GrantType) != constants.GrantTypeRefreshToken {
		return &model.ErrorResponse{
			Error:            constants.ErrorUnsupportedGrantType,
			ErrorDescription: "Unsupported grant type",
		}
	}
	if tokenRequest.RefreshToken == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Refresh token is required",
		}
	}
	if tokenRequest.ClientID == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Client ID is required",
		}
	}

	if errResp := resourceindicators.ValidateResourceURIs(tokenRequest.Resources); errResp != nil {
		return errResp
	}

	return nil
}

// HandleGrant processes the refresh token grant request and generates a new token response.
func (h *refreshTokenGrantHandler) HandleGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *inboundmodel.OAuthClient) (
	*model.TokenResponseDTO, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RefreshTokenGrantHandler"))

	// Validate refresh token using token validator
	refreshTokenClaims, err := h.tokenValidator.ValidateRefreshToken(tokenRequest.RefreshToken, tokenRequest.ClientID)
	if err != nil {
		logger.Debug("Failed to validate refresh token", log.Error(err))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid refresh token",
		}
	}

	newTokenScopes, scopeErr := h.validateAndApplyScopes(tokenRequest.Scope, refreshTokenClaims.Scopes, logger)
	if scopeErr != nil {
		return nil, scopeErr
	}

	// Compute narrowed audiences per RFC 8707 §2.1. When the client supplies resource parameters,
	// narrow the audience to the intersection with the original refresh-token audiences.
	// An empty intersection is a client error (invalid_target).
	audiences := refreshTokenClaims.Audiences
	if len(tokenRequest.Resources) > 0 {
		original := make(map[string]struct{}, len(refreshTokenClaims.Audiences))
		for _, a := range refreshTokenClaims.Audiences {
			original[a] = struct{}{}
		}
		narrowed := make([]string, 0, len(tokenRequest.Resources))
		for _, r := range tokenRequest.Resources {
			if _, ok := original[r]; ok {
				narrowed = append(narrowed, r)
			}
		}
		if len(narrowed) == 0 {
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidTarget,
				ErrorDescription: "Requested resources do not match any audience in the original grant",
			}
		}
		audiences = narrowed

		// Downscope: resolve the narrowed RSes and filter scopes to only those valid on them.
		resolvedRSes, rsErr := resourceindicators.ResolveResourceServers(ctx, h.resourceService, narrowed)
		if rsErr != nil {
			return nil, rsErr
		}
		narrowedRSes := resourceindicators.FilterByIdentifiers(resolvedRSes, narrowed)
		oidcScopes, nonOidcScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(
			strings.Join(newTokenScopes, " "), oauthApp.ScopeClaims)
		rsValidScopes, scopeErr := resourceindicators.ComputeRSValidScopes(
			ctx, h.resourceService, narrowedRSes, nonOidcScopes)
		if scopeErr != nil {
			return nil, scopeErr
		}
		oidcScopes = append(oidcScopes, resourceindicators.UnionScopes(rsValidScopes)...)
		newTokenScopes = oidcScopes
	}

	// Get user attributes from attribute cache.
	// cacheEntry is kept so its current TTLSeconds can be compared later.
	attrs := make(map[string]interface{})
	var cacheEntry *attributecache.AttributeCache
	var fetchErr *serviceerror.ServiceError
	if refreshTokenClaims.AttributeCacheID != "" {
		cacheEntry, fetchErr = h.attrCacheService.GetAttributeCache(ctx, refreshTokenClaims.AttributeCacheID)
		if fetchErr != nil {
			logger.Error("Failed to get user attributes from attribute cache",
				log.String("error", fetchErr.ErrorDescription.DefaultValue))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to get user attributes from attribute cache",
			}
		}
		if cacheEntry == nil {
			logger.Error("Attribute cache entry not found for cache ID",
				log.String("cache_id", refreshTokenClaims.AttributeCacheID))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to get user attributes from attribute cache",
			}
		}
		attrs = cacheEntry.Attributes
	}

	accessToken, err := h.tokenBuilder.BuildAccessToken(&tokenservice.AccessTokenBuildContext{
		Context:          ctx,
		Subject:          refreshTokenClaims.Sub,
		Audiences:        audiences,
		ClientID:         tokenRequest.ClientID,
		Scopes:           newTokenScopes,
		UserAttributes:   attrs,
		AttributeCacheID: refreshTokenClaims.AttributeCacheID,
		GrantType:        refreshTokenClaims.GrantType,
		OAuthApp:         oauthApp,
		ClaimsRequest:    refreshTokenClaims.ClaimsRequest,
		ClaimsLocales:    refreshTokenClaims.ClaimsLocales,
	})
	if err != nil {
		logger.Error("Failed to generate access token", log.Error(err))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate access token",
		}
	}

	// Prepare the token response
	tokenResponse := &model.TokenResponseDTO{
		AccessToken: *accessToken,
	}

	// Generate ID token if 'openid' scope is present
	if slices.Contains(newTokenScopes, constants.ScopeOpenID) {
		idToken, idErr := h.tokenBuilder.BuildIDToken(&tokenservice.IDTokenBuildContext{
			Context:        ctx,
			Subject:        refreshTokenClaims.Sub,
			Audience:       tokenRequest.ClientID,
			Scopes:         newTokenScopes,
			UserAttributes: attrs,
			OAuthApp:       oauthApp,
			ClaimsRequest:  refreshTokenClaims.ClaimsRequest,
		})
		if idErr != nil {
			logger.Error("Failed to generate ID token", log.Error(idErr))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to generate token",
			}
		}
		tokenResponse.IDToken = *idToken
	}

	// Check configuration for refresh token renewal
	conf := config.GetServerRuntime().Config
	renewRefreshToken := conf.OAuth.RefreshToken.RenewOnGrant

	// Issue a new refresh token if renew_on_grant is enabled; otherwise reuse the existing one.
	// RFC 8707 §5: the refresh token preserves the full original audience, not the narrowed one.
	if renewRefreshToken {
		logger.Debug("Renewing refresh token", log.String("client_id", tokenRequest.ClientID))
		errResp := h.IssueRefreshToken(ctx, tokenResponse, oauthApp,
			refreshTokenClaims.Sub, refreshTokenClaims.Audiences,
			refreshTokenClaims.GrantType, newTokenScopes,
			refreshTokenClaims.ClaimsRequest, refreshTokenClaims.ClaimsLocales,
			refreshTokenClaims.AttributeCacheID)
		if errResp != nil && errResp.Error != "" {
			logger.Error("Failed to issue refresh token", log.String("error", errResp.Error))
			return nil, errResp
		}
	} else {
		tokenResponse.RefreshToken = model.TokenDTO{
			Token:    tokenRequest.RefreshToken,
			IssuedAt: refreshTokenClaims.Iat,
			Scopes:   refreshTokenClaims.Scopes,
			ClientID: tokenRequest.ClientID,
		}
	}

	if errResp := h.extendCacheTTL(ctx, cacheEntry, oauthApp, refreshTokenClaims.Iat,
		accessToken.ExpiresIn, renewRefreshToken, refreshTokenClaims.AttributeCacheID, logger); errResp != nil {
		return nil, errResp
	}

	return tokenResponse, nil
}

// IssueRefreshToken generates a new refresh token for the given OAuth application and scopes.
func (h *refreshTokenGrantHandler) IssueRefreshToken(
	ctx context.Context,
	tokenResponse *model.TokenResponseDTO,
	oauthApp *inboundmodel.OAuthClient,
	subject string, audiences []string, grantType string,
	scopes []string,
	claimsRequest *model.ClaimsRequest,
	claimsLocales string,
	attributeCacheID string,
) *model.ErrorResponse {
	tokenCtx := &tokenservice.RefreshTokenBuildContext{
		Context:              ctx,
		ClientID:             oauthApp.ClientID,
		Scopes:               scopes,
		GrantType:            grantType,
		AccessTokenSubject:   subject,
		AccessTokenAudiences: audiences,
		AttributeCacheID:     attributeCacheID,
		OAuthApp:             oauthApp,
		ClaimsRequest:        claimsRequest,
		ClaimsLocales:        claimsLocales,
	}

	// Build refresh token using token builder
	refreshToken, err := h.tokenBuilder.BuildRefreshToken(tokenCtx)
	if err != nil {
		return &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate refresh token",
		}
	}

	if tokenResponse == nil {
		tokenResponse = &model.TokenResponseDTO{}
	}
	tokenResponse.RefreshToken = *refreshToken
	return nil
}

// extendCacheTTL extends the attribute cache TTL when the desired lifetime exceeds what is already
// stored. The desired TTL is the larger of:
//   - the refresh token's actual expiry (iat + validity; for a renewed token, iat = now)
//   - the newly issued access token's expiry (now + ExpiresIn)
//
// This ensures the cache outlives whichever token lives longest without needlessly
// re-writing an already-sufficient entry.
func (h *refreshTokenGrantHandler) extendCacheTTL(
	ctx context.Context,
	cacheEntry *attributecache.AttributeCache,
	oauthApp *inboundmodel.OAuthClient,
	refreshIat, accessExpiresIn int64,
	renewRefreshToken bool,
	cacheID string,
	logger *log.Logger,
) *model.ErrorResponse {
	if cacheEntry == nil {
		return nil
	}
	now := time.Now().Unix()
	refreshValidity := tokenservice.ResolveTokenConfig(oauthApp, tokenservice.TokenTypeRefresh).ValidityPeriod
	if renewRefreshToken {
		refreshIat = now // newly issued token starts from now
	}
	refreshExpiry := refreshIat + refreshValidity
	accessExpiry := now + accessExpiresIn
	maxExpiry := refreshExpiry
	if accessExpiry > maxExpiry {
		maxExpiry = accessExpiry
	}
	desiredTTL := int(maxExpiry-now) + constants.AttributeCacheTTLBufferSeconds
	if desiredTTL > cacheEntry.TTLSeconds {
		if extErr := h.attrCacheService.ExtendAttributeCacheTTL(ctx, cacheID, desiredTTL); extErr != nil {
			logger.Error("Failed to extend attribute cache TTL",
				log.String("cache_id", cacheID),
				log.String("error", extErr.Error.String()))
			return &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to extend attribute cache TTL",
			}
		}
	}
	return nil
}

// validateAndApplyScopes validates and applies OAuth2 scope downscoping logic per RFC 6749 §6.
// If no scopes are requested, all refresh token scopes are granted.
// If scopes are requested, they must be a subset of the original grant; otherwise an invalid_scope error is returned.
func (h *refreshTokenGrantHandler) validateAndApplyScopes(requestedScopes string,
	refreshTokenScopes []string, logger *log.Logger) ([]string, *model.ErrorResponse) {
	trimmedRequestedScopes := tokenservice.ParseScopes(requestedScopes)

	if len(trimmedRequestedScopes) == 0 {
		logger.Debug("No scopes requested. Granting all scopes from refresh token",
			log.Any("scopes", refreshTokenScopes))
		return refreshTokenScopes, nil
	}

	for _, requestedScope := range trimmedRequestedScopes {
		if !slices.Contains(refreshTokenScopes, requestedScope) {
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidScope,
				ErrorDescription: "Requested scope exceeds the scope granted by the resource owner",
			}
		}
	}

	logger.Debug("Applied scope downscoping", log.Any("grantedScopes", trimmedRequestedScopes))
	return trimmedRequestedScopes, nil
}
