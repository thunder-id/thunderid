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

// Package token provides the service for managing OAuth 2.0 token requests.
package token

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/granthandlers"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/scope"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// TokenServiceInterface defines the interface for OAuth 2.0 token processing.
type TokenServiceInterface interface {
	ProcessTokenRequest(
		ctx context.Context,
		tokenRequest *model.TokenRequest,
		oauthApp *inboundmodel.OAuthClient,
	) (*model.TokenResponse, *model.ErrorResponse)
}

// tokenService implements the TokenServiceInterface.
type tokenService struct {
	grantHandlerProvider granthandlers.GrantHandlerProviderInterface
	scopeValidator       scope.ScopeValidatorInterface
	observabilitySvc     observability.ObservabilityServiceInterface
	transactioner        transaction.Transactioner
}

// newTokenService creates a new instance of tokenService.
func newTokenService(
	grantHandlerProvider granthandlers.GrantHandlerProviderInterface,
	scopeValidator scope.ScopeValidatorInterface,
	observabilitySvc observability.ObservabilityServiceInterface,
	transactioner transaction.Transactioner,
) TokenServiceInterface {
	return &tokenService{
		grantHandlerProvider: grantHandlerProvider,
		scopeValidator:       scopeValidator,
		observabilitySvc:     observabilitySvc,
		transactioner:        transactioner,
	}
}

// ProcessTokenRequest validates and processes an OAuth 2.0 token request.
func (ts *tokenService) ProcessTokenRequest(
	ctx context.Context,
	tokenRequest *model.TokenRequest,
	oauthApp *inboundmodel.OAuthClient,
) (*model.TokenResponse, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TokenService"))

	startTime := time.Now().UnixMilli()
	clientID := tokenRequest.ClientID
	grantTypeStr := tokenRequest.GrantType
	scopeStr := tokenRequest.Scope

	ts.publishTokenIssuanceStartedEvent(ctx, clientID, grantTypeStr, scopeStr)

	// Validate grant_type presence.
	if grantTypeStr == "" {
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, clientID, grantTypeStr, scopeStr,
			400, "Missing grant_type parameter", startTime)
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Missing grant_type parameter",
		}
	}

	// Validate grant_type value.
	grantType := constants.GrantType(grantTypeStr)
	if !grantType.IsValid() {
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, clientID, grantTypeStr, scopeStr,
			400, "Invalid grant_type parameter", startTime)
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorUnsupportedGrantType,
			ErrorDescription: "Invalid grant_type parameter",
		}
	}

	// Look up the grant handler.
	grantHandler, handlerErr := ts.grantHandlerProvider.GetGrantHandler(grantType)
	if handlerErr != nil {
		if errors.Is(handlerErr, constants.UnSupportedGrantTypeError) {
			publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, clientID, grantTypeStr, scopeStr,
				400, "Unsupported grant type", startTime)
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorUnsupportedGrantType,
				ErrorDescription: "Unsupported grant type",
			}
		}
		logger.Error("Failed to get grant handler", log.Error(handlerErr))
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, clientID, grantTypeStr, scopeStr,
			500, "Failed to get grant handler", startTime)
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to process token request",
		}
	}

	// Validate grant type against the application.
	if !oauthApp.IsAllowedGrantType(grantType) {
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, clientID, grantTypeStr, scopeStr,
			401, "Client not authorized for grant type", startTime)
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorUnauthorizedClient,
			ErrorDescription: "The client is not authorized to use this grant type",
		}
	}

	// Validate the token request via the grant handler.
	tokenError := grantHandler.ValidateGrant(ctx, tokenRequest, oauthApp)
	if tokenError != nil && tokenError.Error != "" {
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, clientID, grantTypeStr, scopeStr,
			400, tokenError.ErrorDescription, startTime)
		return nil, tokenError
	}

	// Validate and filter scopes.
	validScopes, scopeError := ts.scopeValidator.ValidateScopes(ctx, tokenRequest.Scope, oauthApp.ClientID)
	if scopeError != nil {
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, clientID, grantTypeStr, scopeStr,
			400, scopeError.ErrorDescription, startTime)
		return nil, &model.ErrorResponse{
			Error:            scopeError.Error,
			ErrorDescription: scopeError.ErrorDescription,
		}
	}
	tokenRequest.Scope = validScopes

	// Delegate to the grant handler for token generation.
	tokenRespDTO, tokenError := grantHandler.HandleGrant(ctx, tokenRequest, oauthApp)
	if tokenError != nil {
		if tokenError.Error != "" {
			code := 400
			if tokenError.Error == constants.ErrorServerError {
				code = 500
			}
			publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, clientID, grantTypeStr, scopeStr,
				code, tokenError.ErrorDescription, startTime)
			if tokenError.Error == constants.ErrorServerError {
				tokenError.ErrorDescription = "Failed to process token request"
			}
		}
		return nil, tokenError
	}
	if tokenRespDTO == nil {
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, clientID, grantTypeStr, scopeStr,
			500, "Grant handler returned empty response", startTime)
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to process token request",
		}
	}

	// Issue refresh token if applicable.
	if grantType == constants.GrantTypeAuthorizationCode &&
		oauthApp.IsAllowedGrantType(constants.GrantTypeRefreshToken) {
		logger.Debug("Issuing refresh token for the token request",
			log.String("client_id", clientID), log.String("grant_type", grantTypeStr))

		refreshGrantHandler, handlerErr := ts.grantHandlerProvider.GetGrantHandler(constants.GrantTypeRefreshToken)
		if handlerErr != nil {
			logger.Error("Failed to get refresh grant handler", log.Error(handlerErr))
			publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, clientID, grantTypeStr, scopeStr,
				500, "Failed to get refresh grant handler", startTime)
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to process token request",
			}
		}
		refreshGrantHandlerTyped, ok := refreshGrantHandler.(granthandlers.RefreshTokenGrantHandlerInterface)
		if !ok {
			logger.Error("Failed to cast refresh grant handler",
				log.String("client_id", clientID), log.String("grant_type", grantTypeStr))
			publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, clientID, grantTypeStr, scopeStr,
				500, "Internal Server Error", startTime)
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to process token request",
			}
		}

		refreshAudiences := tokenRespDTO.AccessToken.Audiences
		if len(tokenRespDTO.AccessToken.OriginalAudiences) > 0 {
			refreshAudiences = tokenRespDTO.AccessToken.OriginalAudiences
		}
		refreshTokenError := refreshGrantHandlerTyped.IssueRefreshToken(
			ctx,
			tokenRespDTO, oauthApp,
			tokenRespDTO.AccessToken.Subject, refreshAudiences,
			grantTypeStr, tokenRespDTO.AccessToken.Scopes, tokenRespDTO.AccessToken.ClaimsRequest,
			tokenRespDTO.AccessToken.ClaimsLocales, tokenRespDTO.AccessToken.AttributeCacheID,
		)
		if refreshTokenError != nil && refreshTokenError.Error != "" {
			publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, clientID, grantTypeStr, scopeStr,
				500, refreshTokenError.ErrorDescription, startTime)
			if refreshTokenError.Error == constants.ErrorServerError {
				refreshTokenError.ErrorDescription = "Failed to process token request"
			}
			return nil, refreshTokenError
		}
	}

	// Build token response.
	scopes := strings.Join(tokenRespDTO.AccessToken.Scopes, " ")
	tokenResponse := &model.TokenResponse{
		AccessToken:  tokenRespDTO.AccessToken.Token,
		TokenType:    tokenRespDTO.AccessToken.TokenType,
		ExpiresIn:    tokenRespDTO.AccessToken.ExpiresIn,
		RefreshToken: tokenRespDTO.RefreshToken.Token,
		Scope:        scopes,
		IDToken:      tokenRespDTO.IDToken.Token,
	}

	// For token exchange, determine the issued_token_type from the request.
	if grantType == constants.GrantTypeTokenExchange {
		requestedTokenType := tokenRequest.RequestedTokenType
		if requestedTokenType == "" || requestedTokenType == string(constants.TokenTypeIdentifierAccessToken) {
			tokenResponse.IssuedTokenType = string(constants.TokenTypeIdentifierAccessToken)
		} else {
			tokenResponse.IssuedTokenType = string(constants.TokenTypeIdentifierJWT)
		}
	}

	logger.Debug("Token generated successfully",
		log.String("client_id", clientID), log.String("grant_type", grantTypeStr))

	ts.publishTokenIssuedEvent(ctx, clientID, grantTypeStr, scopes, startTime)

	return tokenResponse, nil
}

// publishTokenIssuanceStartedEvent publishes an event indicating that token issuance has started.
func (ts *tokenService) publishTokenIssuanceStartedEvent(ctx context.Context, clientID, grantType, scope string) {
	if ts.observabilitySvc == nil || !ts.observabilitySvc.IsEnabled() {
		return
	}

	evt := event.NewEvent(
		sysContext.GetTraceID(ctx),
		string(event.EventTypeTokenIssuanceStarted),
		event.ComponentAuthHandler,
	).
		WithStatus(event.StatusInProgress).
		WithData(event.DataKey.ClientID, clientID).
		WithData(event.DataKey.GrantType, grantType).
		WithData(event.DataKey.Scope, scope)

	ts.observabilitySvc.PublishEvent(evt)
}

func (ts *tokenService) publishTokenIssuedEvent(
	ctx context.Context, clientID, grantType, scope string, startTime int64,
) {
	if ts.observabilitySvc == nil || !ts.observabilitySvc.IsEnabled() {
		return
	}

	duration := time.Now().UnixMilli() - startTime

	evt := event.NewEvent(
		sysContext.GetTraceID(ctx),
		string(event.EventTypeTokenIssued),
		event.ComponentAuthHandler,
	).
		WithStatus(event.StatusSuccess).
		WithData(event.DataKey.ClientID, clientID).
		WithData(event.DataKey.GrantType, grantType).
		WithData(event.DataKey.Scope, scope).
		WithData(event.DataKey.DurationMs, fmt.Sprintf("%d", duration))

	ts.observabilitySvc.PublishEvent(evt)
}

// publishTokenIssuanceFailedEvent is a package-level helper shared by tokenService and tokenHandler.
func publishTokenIssuanceFailedEvent(
	svc observability.ObservabilityServiceInterface,
	ctx context.Context, clientID, grantType, scope string, statusCode int, message string, startTime int64,
) {
	if svc == nil || !svc.IsEnabled() {
		return
	}

	duration := time.Now().UnixMilli() - startTime

	errorType := "client_error"
	if statusCode >= 500 {
		errorType = "server_error"
	}

	evt := event.NewEvent(
		sysContext.GetTraceID(ctx),
		string(event.EventTypeTokenIssuanceFailed),
		event.ComponentAuthHandler,
	).
		WithStatus(event.StatusFailure).
		WithData(event.DataKey.ClientID, clientID).
		WithData(event.DataKey.GrantType, grantType).
		WithData(event.DataKey.Scope, scope).
		WithData(event.DataKey.Error, message).
		WithData(event.DataKey.ErrorCode, fmt.Sprintf("%d", statusCode)).
		WithData(event.DataKey.ErrorType, errorType).
		WithData(event.DataKey.DurationMs, fmt.Sprintf("%d", duration))

	svc.PublishEvent(evt)
}
