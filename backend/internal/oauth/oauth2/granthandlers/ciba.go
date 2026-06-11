/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"time"

	"github.com/thunder-id/thunderid/internal/attributecache"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/ciba"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// cibaGrantHandler handles the OpenID Connect CIBA grant type (poll mode).
type cibaGrantHandler struct {
	cibaService    ciba.CIBAServiceInterface
	tokenBuilder   tokenservice.TokenBuilderInterface
	attributeCache attributecache.AttributeCacheServiceInterface
}

// newCIBAGrantHandler creates a new instance of cibaGrantHandler.
func newCIBAGrantHandler(
	cibaService ciba.CIBAServiceInterface,
	tokenBuilder tokenservice.TokenBuilderInterface,
	attributeCache attributecache.AttributeCacheServiceInterface,
) GrantHandlerInterface {
	return &cibaGrantHandler{
		cibaService:    cibaService,
		tokenBuilder:   tokenBuilder,
		attributeCache: attributeCache,
	}
}

// ValidateGrant validates the CIBA grant request.
func (h *cibaGrantHandler) ValidateGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *inboundmodel.OAuthClient) *model.ErrorResponse {
	if constants.GrantType(tokenRequest.GrantType) != constants.GrantTypeCIBA {
		return &model.ErrorResponse{
			Error:            constants.ErrorUnsupportedGrantType,
			ErrorDescription: "Unsupported grant type",
		}
	}
	if tokenRequest.AuthReqID == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "auth_req_id is required",
		}
	}

	record, err := h.cibaService.GetByAuthReqID(ctx, tokenRequest.AuthReqID)
	if err != nil {
		if errors.Is(err, ciba.ErrCIBARequestNotFound) {
			return &model.ErrorResponse{
				Error:            constants.ErrorInvalidGrant,
				ErrorDescription: "Invalid auth_req_id",
			}
		}
		return &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to process token request",
		}
	}
	if record.ClientID != oauthApp.ClientID {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid auth_req_id",
		}
	}

	return nil
}

// HandleGrant processes the CIBA grant request following the poll-mode state machine.
func (h *cibaGrantHandler) HandleGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *inboundmodel.OAuthClient) (*model.TokenResponseDTO, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CIBAGrantHandler"))

	record, err := h.cibaService.GetByAuthReqID(ctx, tokenRequest.AuthReqID)
	if err != nil {
		if errors.Is(err, ciba.ErrCIBARequestNotFound) {
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidGrant,
				ErrorDescription: "Invalid auth_req_id",
			}
		}
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to process token request",
		}
	}

	now := time.Now()

	// Expiry takes precedence over all other states.
	if now.After(record.ExpiryTime) {
		if record.State != ciba.CIBAStateExpired && record.State != ciba.CIBAStateConsumed {
			if updateErr := h.cibaService.UpdateState(ctx, record.AuthReqID, ciba.CIBAStateExpired); updateErr != nil {
				logger.Error(ctx, "Failed to mark CIBA request as expired", log.Error(updateErr))
			}
		}
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorExpiredToken,
			ErrorDescription: "The authentication request has expired",
		}
	}

	switch record.State {
	case ciba.CIBAStatePending:
		return nil, h.handlePending(ctx, record, now, logger)
	case ciba.CIBAStateDenied:
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorAccessDenied,
			ErrorDescription: "The user denied the authentication request",
		}
	case ciba.CIBAStateConsumed:
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "The authentication request has already been used",
		}
	case ciba.CIBAStateAuthenticated:
		return h.issueTokens(ctx, record, oauthApp, logger)
	default:
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid authentication request state",
		}
	}
}

// handlePending enforces the polling interval and returns slow_down or authorization_pending.
func (h *cibaGrantHandler) handlePending(ctx context.Context, record *ciba.CIBAAuthRequest,
	now time.Time, logger *log.Logger) *model.ErrorResponse {
	tooFast := !record.LastPolledAt.IsZero() &&
		now.Sub(record.LastPolledAt) < time.Duration(constants.CIBADefaultIntervalSeconds)*time.Second

	if updateErr := h.cibaService.UpdateLastPolled(ctx, record.AuthReqID, now); updateErr != nil {
		logger.Error(ctx, "Failed to update CIBA last polled time", log.Error(updateErr))
	}

	if tooFast {
		return &model.ErrorResponse{
			Error:            constants.ErrorSlowDown,
			ErrorDescription: "The client is polling too frequently",
		}
	}
	return &model.ErrorResponse{
		Error:            constants.ErrorAuthorizationPending,
		ErrorDescription: "The user has not yet completed authentication",
	}
}

// issueTokens builds the access, refresh, and (when openid) ID tokens, then marks the request consumed.
func (h *cibaGrantHandler) issueTokens(ctx context.Context, record *ciba.CIBAAuthRequest,
	oauthApp *inboundmodel.OAuthClient, logger *log.Logger) (*model.TokenResponseDTO, *model.ErrorResponse) {
	// Use AuthorizedScopes (StandardScopes + authorized permissions from assertion) when available.
	// Falls back to StandardScopes (OIDC only) if callback hasn't been processed yet.
	scopeStr := record.AuthorizedScopes
	if scopeStr == "" {
		scopeStr = record.StandardScopes
	}
	scopes := tokenservice.ParseScopes(scopeStr)

	attrs := make(map[string]interface{})
	if record.AttributeCacheID != "" {
		cacheEntry, cacheErr := h.attributeCache.GetAttributeCache(ctx, record.AttributeCacheID)
		if cacheErr != nil {
			logger.Error(ctx, "Failed to get user attributes from attribute cache",
				log.String("error", cacheErr.ErrorDescription.DefaultValue))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to get user attributes from attribute cache",
			}
		}
		attrs = cacheEntry.Attributes
	}

	accessToken, err := h.tokenBuilder.BuildAccessToken(ctx, &tokenservice.AccessTokenBuildContext{
		Subject:          record.UserID,
		Audiences:        []string{oauthApp.ClientID},
		ClientID:         oauthApp.ClientID,
		Scopes:           scopes,
		UserAttributes:   attrs,
		AttributeCacheID: record.AttributeCacheID,
		GrantType:        string(constants.GrantTypeCIBA),
		OAuthApp:         oauthApp,
	})
	if err != nil {
		logger.Error(ctx, "Failed to generate access token", log.Error(err))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}

	tokenResponse := &model.TokenResponseDTO{
		AccessToken: *accessToken,
	}

	if slices.Contains(scopes, constants.ScopeOpenID) {
		idToken, idErr := h.tokenBuilder.BuildIDToken(ctx, &tokenservice.IDTokenBuildContext{
			Subject:        record.UserID,
			Audience:       oauthApp.ClientID,
			Scopes:         scopes,
			UserAttributes: attrs,
			AuthTime:       record.AuthTime.Unix(),
			OAuthApp:       oauthApp,
			CompletedACR:   record.CompletedACR,
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

	// Atomically consume the request to enforce one-time use. If another concurrent poll already
	// consumed it, reject this one with invalid_grant.
	consumed, consumeErr := h.cibaService.MarkConsumed(ctx, record.AuthReqID)
	if consumeErr != nil {
		logger.Error(ctx, "Failed to consume CIBA authentication request", log.Error(consumeErr))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to process token request",
		}
	}
	if !consumed {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "The authentication request has already been used",
		}
	}

	return tokenResponse, nil
}
