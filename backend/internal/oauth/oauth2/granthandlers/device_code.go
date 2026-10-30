// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/deviceauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// deviceCodeGrantHandler validates Device Code grants and issues tokens for approved requests.
type deviceCodeGrantHandler struct {
	deviceService   deviceauth.DeviceAuthServiceInterface
	tokenBuilder    tokenservice.TokenBuilderInterface
	attributeCache  attributecache.AttributeCacheServiceInterface
	resourceService providers.ResourceServerProvider
	logger          *log.Logger
}

// newDeviceCodeGrantHandler creates a Device Code grant handler with its token dependencies.
func newDeviceCodeGrantHandler(
	deviceService deviceauth.DeviceAuthServiceInterface,
	tokenBuilder tokenservice.TokenBuilderInterface,
	attributeCache attributecache.AttributeCacheServiceInterface,
	resourceService providers.ResourceServerProvider,
) GrantHandlerInterface {
	return &deviceCodeGrantHandler{
		deviceService:   deviceService,
		tokenBuilder:    tokenBuilder,
		attributeCache:  attributeCache,
		resourceService: resourceService,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "DeviceCodeGrantHandler")),
	}
}

// ValidateGrant validates the Device Code grant and its client and resource bindings.
func (h *deviceCodeGrantHandler) ValidateGrant(
	ctx context.Context, tokenRequest *model.TokenRequest, oauthApp *providers.OAuthClient,
) *model.ErrorResponse {
	if providers.GrantType(tokenRequest.GrantType) != providers.GrantTypeDeviceCode {
		return &model.ErrorResponse{
			Error: constants.ErrorUnsupportedGrantType, ErrorDescription: "Unsupported grant type"}
	}
	if tokenRequest.DeviceCode == "" {
		return &model.ErrorResponse{
			Error: constants.ErrorInvalidRequest, ErrorDescription: "device_code is required"}
	}

	record, err := h.deviceService.GetByDeviceCode(ctx, tokenRequest.DeviceCode)
	if err != nil {
		if errors.Is(err, deviceauth.ErrDeviceAuthRequestNotFound) {
			return invalidDeviceCodeError()
		}
		return tokenProcessingError()
	}
	if record.ClientID != oauthApp.ClientID {
		return invalidDeviceCodeError()
	}
	if errResp := resourceindicators.ValidateResourceURIs(tokenRequest.Resources); errResp != nil {
		return errResp
	}
	if errResp := enforceDevicePollingResource(tokenRequest.Resources, record.Resources); errResp != nil {
		return errResp
	}
	return nil
}

// enforceDevicePollingResource ensures a polling request uses the authorized resource.
func enforceDevicePollingResource(pollingResources, storedResources []string) *model.ErrorResponse {
	if len(pollingResources) == 0 {
		return nil
	}
	if len(pollingResources) > 1 || len(storedResources) == 0 || pollingResources[0] != storedResources[0] {
		return &model.ErrorResponse{Error: constants.ErrorInvalidTarget,
			ErrorDescription: "The resource parameter does not match the authorized resource"}
	}
	return nil
}

// HandleGrant evaluates the request state and issues tokens when authorization is complete.
func (h *deviceCodeGrantHandler) HandleGrant(
	ctx context.Context, tokenRequest *model.TokenRequest, oauthApp *providers.OAuthClient,
) (*model.TokenResponseDTO, *model.ErrorResponse) {
	record, err := h.deviceService.GetByDeviceCode(ctx, tokenRequest.DeviceCode)
	if err != nil {
		if errors.Is(err, deviceauth.ErrDeviceAuthRequestNotFound) {
			return nil, invalidDeviceCodeError()
		}
		return nil, tokenProcessingError()
	}

	now := time.Now()
	if now.After(record.ExpiryTime) {
		if record.State != deviceauth.DeviceStateExpired && record.State != deviceauth.DeviceStateConsumed {
			if updateErr := h.deviceService.UpdateState(
				ctx, record.RequestID, deviceauth.DeviceStateExpired,
			); updateErr != nil {
				h.logger.Error(ctx, "Failed to mark Device Grant request as expired", log.Error(updateErr))
			}
		}
		return nil, &model.ErrorResponse{Error: constants.ErrorExpiredToken,
			ErrorDescription: "The device authorization request has expired"}
	}

	switch record.State {
	case deviceauth.DeviceStatePending, deviceauth.DeviceStateRedeeming:
		return nil, h.handlePending(ctx, record, now)
	case deviceauth.DeviceStateDenied:
		return nil, &model.ErrorResponse{Error: constants.ErrorAccessDenied,
			ErrorDescription: "The user denied the device authorization request"}
	case deviceauth.DeviceStateFailed:
		return nil, &model.ErrorResponse{Error: constants.ErrorServerError,
			ErrorDescription: "Device authorization could not be completed"}
	case deviceauth.DeviceStateConsumed:
		return nil, invalidDeviceCodeError()
	case deviceauth.DeviceStateApproved:
		return h.issueTokens(ctx, record, oauthApp)
	default:
		return nil, invalidDeviceCodeError()
	}
}

// handlePending applies the Device Grant polling interval to a pending request.
func (h *deviceCodeGrantHandler) handlePending(
	ctx context.Context, record *deviceauth.DeviceAuthRequest, now time.Time,
) *model.ErrorResponse {
	interval := record.PollInterval
	if interval <= 0 {
		interval = constants.DeviceAuthorizationDefaultIntervalSeconds
	}
	tooFast := !record.LastPolledAt.IsZero() &&
		now.Sub(record.LastPolledAt) < time.Duration(interval)*time.Second
	if tooFast {
		interval += constants.DeviceAuthorizationSlowDownIncrementSeconds
	}
	if updateErr := h.deviceService.UpdatePolling(ctx, record.RequestID, now, interval); updateErr != nil {
		h.logger.Error(ctx, "Failed to update Device Grant polling data", log.Error(updateErr))
		return tokenProcessingError()
	}
	if tooFast {
		return &model.ErrorResponse{Error: constants.ErrorSlowDown,
			ErrorDescription: "The client is polling too frequently"}
	}
	return &model.ErrorResponse{Error: constants.ErrorAuthorizationPending,
		ErrorDescription: "The user has not yet completed authorization"}
}

// issueTokens builds tokens for an approved request and marks the request as consumed.
func (h *deviceCodeGrantHandler) issueTokens(
	ctx context.Context, record *deviceauth.DeviceAuthRequest, oauthApp *providers.OAuthClient,
) (*model.TokenResponseDTO, *model.ErrorResponse) {
	scopeStr := record.AuthorizedScopes
	if scopeStr == "" {
		scopeStr = record.StandardScopes
	}
	audiences, scopes, bindErr := h.resolveIssuedAudiencesAndScopes(ctx, record, oauthApp, scopeStr)
	if bindErr != nil {
		return nil, bindErr
	}

	attrs := make(map[string]interface{})
	if record.AttributeCacheID != "" {
		cacheEntry, cacheErr := h.attributeCache.GetAttributeCache(ctx, record.AttributeCacheID)
		if cacheErr != nil {
			h.logger.Error(ctx, "Failed to get user attributes from attribute cache",
				log.String("error", cacheErr.ErrorDescription.DefaultValue))
			return nil, tokenProcessingError()
		}
		attrs = cacheEntry.Attributes
	}

	userSubConfig := oauthApp.UserAccessTokenConfig()
	accessTokenCtx := &tokenservice.AccessTokenBuildContext{
		Subject: record.UserID, Audiences: audiences, ClientID: oauthApp.ClientID, Scopes: scopes,
		SubjectAttributes: tokenservice.FilterAttributesByAllowList(attrs, userSubConfig),
		AttributeCacheID:  record.AttributeCacheID, GrantType: string(providers.GrantTypeDeviceCode),
		OAuthApp: oauthApp, ValidityPeriod: userSubConfig.ValidityPeriodOrZero(), DPoPJkt: dpop.GetJkt(ctx),
	}
	if oauthApp.ShouldAppendActorClaim() {
		accessTokenCtx.ActorClaims = &tokenservice.SubjectTokenClaims{Sub: oauthApp.ID}
	}
	accessToken, err := h.tokenBuilder.BuildAccessToken(ctx, accessTokenCtx)
	if err != nil {
		h.logger.Error(ctx, "Failed to generate Device Grant access token", log.Error(err))
		return nil, &model.ErrorResponse{
			Error: constants.ErrorServerError, ErrorDescription: "Failed to generate token"}
	}
	accessToken.OriginalAudiences = audiences
	tokenResponse := &model.TokenResponseDTO{AccessToken: *accessToken, SessionID: record.SessionID}

	if slices.Contains(scopes, constants.ScopeOpenID) {
		idToken, idErr := h.tokenBuilder.BuildIDToken(ctx, &tokenservice.IDTokenBuildContext{
			Subject: record.UserID, Audience: oauthApp.ClientID, Scopes: scopes,
			UserAttributes: attrs, AuthTime: record.AuthTime.Unix(), OAuthApp: oauthApp,
			CompletedACR: record.CompletedACR, SessionID: record.SessionID,
		})
		if idErr != nil {
			h.logger.Error(ctx, "Failed to generate Device Grant ID token", log.Error(idErr))
			return nil, &model.ErrorResponse{
				Error: constants.ErrorServerError, ErrorDescription: "Failed to generate token"}
		}
		tokenResponse.IDToken = *idToken
	}

	consumed, consumeErr := h.deviceService.MarkConsumed(ctx, record.RequestID)
	if consumeErr != nil {
		h.logger.Error(ctx, "Failed to consume Device Grant request", log.Error(consumeErr))
		return nil, tokenProcessingError()
	}
	if !consumed {
		return nil, invalidDeviceCodeError()
	}
	return tokenResponse, nil
}

// resolveIssuedAudiencesAndScopes determines the audiences and scopes bound to issued tokens.
func (h *deviceCodeGrantHandler) resolveIssuedAudiencesAndScopes(
	ctx context.Context, record *deviceauth.DeviceAuthRequest, oauthApp *providers.OAuthClient, scopeStr string,
) ([]string, []string, *model.ErrorResponse) {
	oidcScopes, permissionScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(scopeStr, oauthApp.ScopeClaims)
	if len(record.Resources) == 0 {
		if len(permissionScopes) > 0 {
			return nil, nil, invalidDeviceCodeError()
		}
		return []string{oauthApp.ResolveDefaultAudience(oauthApp.ClientID)}, oidcScopes, nil
	}

	resourceIdentifier := record.Resources[0]
	rs, svcErr := h.resourceService.GetResourceServerByIdentifier(ctx, resourceIdentifier)
	if svcErr != nil {
		h.logger.Error(ctx, "Failed to resolve stored Device Grant resource server",
			log.String("error", svcErr.ErrorDescription.DefaultValue))
		return nil, nil, tokenProcessingError()
	}
	downscoped, downscopeErr := resourceindicators.DownscopeToResourceServer(
		ctx, h.resourceService, rs.ID, permissionScopes)
	if downscopeErr != nil {
		return nil, nil, downscopeErr
	}
	scopes := make([]string, 0, len(oidcScopes)+len(downscoped))
	scopes = append(scopes, oidcScopes...)
	scopes = append(scopes, downscoped...)
	return []string{resourceIdentifier}, scopes, nil
}

// invalidDeviceCodeError returns the token endpoint error for an invalid device code.
func invalidDeviceCodeError() *model.ErrorResponse {
	return &model.ErrorResponse{Error: constants.ErrorInvalidGrant,
		ErrorDescription: "Invalid device_code"}
}

// tokenProcessingError returns the token endpoint error for an internal processing failure.
func tokenProcessingError() *model.ErrorResponse {
	return &model.ErrorResponse{Error: constants.ErrorServerError,
		ErrorDescription: "Failed to process token request"}
}
