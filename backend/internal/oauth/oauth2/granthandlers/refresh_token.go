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
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/attributecache"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// refreshTokenGrantHandler handles the refresh token grant type.
type refreshTokenGrantHandler struct {
	cfg                 oauthconfig.Config
	jwtService          jwt.JWTServiceInterface
	tokenBuilder        tokenservice.TokenBuilderInterface
	tokenValidator      tokenservice.TokenValidatorInterface
	attrCacheService    attributecache.AttributeCacheServiceInterface
	resourceService     providers.ResourceServerProvider
	serverConfigService serverconfig.ServerConfigService
	refreshRevoker      revocation.RefreshTokenRevokerInterface
}

// newRefreshTokenGrantHandler creates a new instance of RefreshTokenGrantHandler.
func newRefreshTokenGrantHandler(
	jwtService jwt.JWTServiceInterface,
	tokenBuilder tokenservice.TokenBuilderInterface,
	tokenValidator tokenservice.TokenValidatorInterface,
	attrCacheService attributecache.AttributeCacheServiceInterface,
	resourceService providers.ResourceServerProvider,
	serverConfigService serverconfig.ServerConfigService,
	refreshRevoker revocation.RefreshTokenRevokerInterface,
	cfg oauthconfig.Config,
) RefreshTokenGrantHandlerInterface {
	return &refreshTokenGrantHandler{
		cfg:                 cfg,
		jwtService:          jwtService,
		tokenBuilder:        tokenBuilder,
		tokenValidator:      tokenValidator,
		attrCacheService:    attrCacheService,
		resourceService:     resourceService,
		serverConfigService: serverConfigService,
		refreshRevoker:      refreshRevoker,
	}
}

// ValidateGrant validates the refresh token grant request.
func (h *refreshTokenGrantHandler) ValidateGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) *model.ErrorResponse {
	if providers.GrantType(tokenRequest.GrantType) != providers.GrantTypeRefreshToken {
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
	oauthApp *providers.OAuthClient) (
	*model.TokenResponseDTO, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RefreshTokenGrantHandler"))

	// Validate refresh token using token validator
	// ValidateRefreshToken verifies the token and enforces the RFC 7009 deny list. A revoked token is
	// rejected as invalid_grant like any other invalid token; an unavailable deny list fails closed
	// with a server_error.
	refreshTokenClaims, err := h.tokenValidator.ValidateRefreshToken(
		ctx, tokenRequest.RefreshToken, tokenRequest.ClientID)
	if err != nil {
		logger.Debug(ctx, "Failed to validate refresh token", log.Error(err))
		if errors.Is(err, revocation.ErrEnforcementUnavailable) {
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Token revocation status could not be verified",
			}
		}
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid refresh token",
		}
	}

	if errResp := dpop.VerifyProofBinding(ctx, refreshTokenClaims.DPoPJkt, "refresh token"); errResp != nil {
		return nil, errResp
	}

	newTokenScopes, scopeErr := h.validateAndApplyScopes(ctx, tokenRequest.Scope, refreshTokenClaims.Scopes, logger)
	if scopeErr != nil {
		return nil, scopeErr
	}

	// The refresh token is bound to exactly one resource server audience. When the request supplies
	// a resource it must match that audience; when omitted, the bound audience is reused.
	if len(refreshTokenClaims.Audiences) != 1 {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Refresh token is not bound to a single resource server",
		}
	}
	audience := refreshTokenClaims.Audiences[0]
	if len(tokenRequest.Resources) > 1 {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidTarget,
			ErrorDescription: "Only a single resource parameter is supported",
		}
	}
	if len(tokenRequest.Resources) == 1 && tokenRequest.Resources[0] != audience {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidTarget,
			ErrorDescription: "Requested resource does not match the refresh token audience",
		}
	}
	audiences := []string{audience}

	oidcScopes, nonOidcScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(
		strings.Join(newTokenScopes, " "), oauthApp.ScopeClaims)
	if audience == tokenRequest.ClientID {
		// The original token was not bound to a resource server (OIDC-only): its audience is the
		// client_id, which is not a resource server, so there are no permissions to downscope against.
		newTokenScopes = oidcScopes
	} else {
		// Resolve the bound resource server to downscope scopes to its currently defined permissions.
		targetRS, rsErr := h.resourceService.GetResourceServerByIdentifier(ctx, audience)
		if rsErr != nil {
			if rsErr.Type == tidcommon.ServerErrorType {
				return nil, &model.ErrorResponse{
					Error:            constants.ErrorServerError,
					ErrorDescription: "Failed to resolve resource server",
				}
			}
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidTarget,
				ErrorDescription: "The resource server bound to the refresh token no longer exists",
			}
		}
		downscopedNonOidc, scopeErr := resourceindicators.DownscopeToResourceServer(
			ctx, h.resourceService, targetRS.ID, nonOidcScopes)
		if scopeErr != nil {
			return nil, scopeErr
		}
		newTokenScopes = make([]string, 0, len(oidcScopes)+len(downscopedNonOidc))
		newTokenScopes = append(newTokenScopes, oidcScopes...)
		newTokenScopes = append(newTokenScopes, downscopedNonOidc...)
	}

	// Get user attributes from attribute cache.
	// cacheEntry is kept so its current TTLSeconds can be compared later.
	attrs := make(map[string]interface{})
	var cacheEntry *attributecache.AttributeCache
	var fetchErr *tidcommon.ServiceError
	if refreshTokenClaims.AttributeCacheID != "" {
		cacheEntry, fetchErr = h.attrCacheService.GetAttributeCache(ctx, refreshTokenClaims.AttributeCacheID)
		if fetchErr != nil {
			logger.Error(ctx, "Failed to get user attributes from attribute cache",
				log.String("error", fetchErr.ErrorDescription.DefaultValue))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to get user attributes from attribute cache",
			}
		}
		if cacheEntry == nil {
			logger.Error(ctx, "Attribute cache entry not found for cache ID",
				log.String("cache_id", refreshTokenClaims.AttributeCacheID))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to get user attributes from attribute cache",
			}
		}
		attrs = cacheEntry.Attributes
	}

	userSubConfig := oauthApp.UserAccessTokenConfig()
	accessTokenCtx := &tokenservice.AccessTokenBuildContext{
		Subject:           refreshTokenClaims.Sub,
		Audiences:         audiences,
		ClientID:          tokenRequest.ClientID,
		Scopes:            newTokenScopes,
		SubjectAttributes: tokenservice.FilterAttributesByAllowList(attrs, userSubConfig),
		AttributeCacheID:  refreshTokenClaims.AttributeCacheID,
		GrantType:         refreshTokenClaims.GrantType,
		OAuthApp:          oauthApp,
		ClaimsRequest:     refreshTokenClaims.ClaimsRequest,
		ClaimsLocales:     refreshTokenClaims.ClaimsLocales,
		ValidityPeriod:    userSubConfig.ValidityPeriodOrZero(),
		DPoPJkt:           dpop.GetJkt(ctx),
	}
	// Replay the on-behalf-of decision frozen at issuance, sourced from the stored marker
	// rather than the client's current setting.
	if refreshTokenClaims.ActorSub != "" {
		accessTokenCtx.ActorClaims = &tokenservice.SubjectTokenClaims{Sub: refreshTokenClaims.ActorSub}
	}
	accessToken, err := h.tokenBuilder.BuildAccessToken(ctx, accessTokenCtx)
	if err != nil {
		logger.Error(ctx, "Failed to generate access token", log.Error(err))
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
		idToken, idErr := h.tokenBuilder.BuildIDToken(ctx, &tokenservice.IDTokenBuildContext{
			Subject:        refreshTokenClaims.Sub,
			Audience:       tokenRequest.ClientID,
			Scopes:         newTokenScopes,
			UserAttributes: attrs,
			OAuthApp:       oauthApp,
			ClaimsRequest:  refreshTokenClaims.ClaimsRequest,
		})
		if idErr != nil {
			logger.Error(ctx, "Failed to generate ID token", log.Error(idErr))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to generate token",
			}
		}
		tokenResponse.IDToken = *idToken
	}

	renewRefreshToken := h.cfg.OAuth.RefreshToken.RenewOnGrant

	// Issue a new refresh token if renew_on_grant is enabled; otherwise reuse the existing one.
	// The new refresh token carries the same single resource server audience.
	if renewRefreshToken {
		logger.Debug(ctx, "Renewing refresh token", log.String("client_id", tokenRequest.ClientID))
		errResp := h.IssueRefreshToken(ctx, tokenResponse, oauthApp,
			refreshTokenClaims.Sub, audiences,
			refreshTokenClaims.GrantType, newTokenScopes,
			refreshTokenClaims.ClaimsRequest, refreshTokenClaims.ClaimsLocales,
			refreshTokenClaims.AttributeCacheID)
		if errResp != nil && errResp.Error != "" {
			logger.Error(ctx, "Failed to issue refresh token", log.String("error", errResp.Error))
			return nil, errResp
		}

		// Single-use: revoke the consumed refresh token so it cannot be replayed (RFC 9700 §4.14.2).
		// Fail closed — if the revocation cannot be recorded, the old token would remain usable, so the
		// rotation is rejected and the client retries with the still-valid old token.
		if h.refreshRevoker != nil && h.cfg.OAuth.RefreshToken.RevokePreviousOnRenew {
			expiryTime := time.Unix(refreshTokenClaims.Exp, 0).UTC()
			if err := h.refreshRevoker.RevokeRefreshToken(
				ctx, refreshTokenClaims.JTI, expiryTime); err != nil {
				logger.Error(ctx, "Failed to revoke rotated refresh token", log.Error(err))
				return nil, &model.ErrorResponse{
					Error:            constants.ErrorServerError,
					ErrorDescription: "Failed to rotate refresh token",
				}
			}
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
		accessToken.ExpiresIn, renewRefreshToken, refreshTokenClaims.AttributeCacheID,
		logger); errResp != nil {
		return nil, errResp
	}

	return tokenResponse, nil
}

// IssueRefreshToken generates a new refresh token for the given OAuth application and scopes.
func (h *refreshTokenGrantHandler) IssueRefreshToken(
	ctx context.Context,
	tokenResponse *model.TokenResponseDTO,
	oauthApp *providers.OAuthClient,
	subject string, audiences []string, grantType string,
	scopes []string,
	claimsRequest *model.ClaimsRequest,
	claimsLocales string,
	attributeCacheID string,
) *model.ErrorResponse {
	tokenCtx := &tokenservice.RefreshTokenBuildContext{
		ClientID:             oauthApp.ClientID,
		Scopes:               scopes,
		GrantType:            grantType,
		AccessTokenSubject:   subject,
		AccessTokenAudiences: audiences,
		AttributeCacheID:     attributeCacheID,
		OAuthApp:             oauthApp,
		ClaimsRequest:        claimsRequest,
		ClaimsLocales:        claimsLocales,
		DPoPJkt:              dpopJktForRefresh(ctx, oauthApp),
	}
	if oauthApp.ShouldAppendActorClaim() {
		tokenCtx.ActorSub = oauthApp.ID
	}

	// Build refresh token using token builder
	refreshToken, err := h.tokenBuilder.BuildRefreshToken(ctx, tokenCtx)
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

// dpopJktForRefresh returns the DPoP jkt to bind onto a newly issued refresh token.
// Confidential clients receive unbound refresh tokens.
func dpopJktForRefresh(ctx context.Context, oauthApp *providers.OAuthClient) string {
	if oauthApp == nil || !oauthApp.PublicClient {
		return ""
	}
	return dpop.GetJkt(ctx)
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
	oauthApp *providers.OAuthClient,
	refreshIat, accessExpiresIn int64,
	renewRefreshToken bool,
	cacheID string,
	logger *log.Logger,
) *model.ErrorResponse {
	if cacheEntry == nil {
		return nil
	}
	now := time.Now().Unix()
	refreshValidity := tokenservice.ResolveTokenConfig(
		h.cfg, oauthApp, tokenservice.TokenTypeRefresh, 0).ValidityPeriod
	if renewRefreshToken {
		refreshIat = now // newly issued token starts from now
	}
	refreshExpiry := refreshIat + refreshValidity
	accessExpiry := now + accessExpiresIn
	maxExpiry := refreshExpiry
	if accessExpiry > maxExpiry {
		maxExpiry = accessExpiry
	}
	desiredTTL := maxExpiry - now + constants.AttributeCacheTTLBufferSeconds
	extErr := h.attrCacheService.ExtendAttributeCacheTTL(ctx, cacheID, int(desiredTTL))
	if extErr != nil {
		logger.Error(ctx, "Failed to extend attribute cache TTL",
			log.String("cache_id", cacheID),
			log.String("error", extErr.Error.String()))
		return &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to extend attribute cache TTL",
		}
	}
	return nil
}

// validateAndApplyScopes validates and applies OAuth2 scope downscoping logic per RFC 6749 §6.
// If no scopes are requested, all refresh token scopes are granted.
// If scopes are requested, they must be a subset of the original grant; otherwise an invalid_scope error is returned.
func (h *refreshTokenGrantHandler) validateAndApplyScopes(ctx context.Context, requestedScopes string,
	refreshTokenScopes []string, logger *log.Logger) ([]string, *model.ErrorResponse) {
	trimmedRequestedScopes := tokenservice.ParseScopes(requestedScopes)

	if len(trimmedRequestedScopes) == 0 {
		logger.Debug(ctx, "No scopes requested. Granting all scopes from refresh token",
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

	logger.Debug(ctx, "Applied scope downscoping", log.Any("grantedScopes", trimmedRequestedScopes))
	return trimmedRequestedScopes, nil
}
