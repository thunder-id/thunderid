// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package token provides the service for managing OAuth 2.0 token requests.
package token

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/granthandlers"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/scope"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// TokenServiceInterface defines the interface for OAuth 2.0 token processing.
type TokenServiceInterface interface {
	ProcessTokenRequest(
		ctx context.Context,
		tokenRequest *model.TokenRequest,
		oauthApp *providers.OAuthClient,
	) (*model.TokenResponse, *model.ErrorResponse)
}

// tokenService implements the TokenServiceInterface.
type tokenService struct {
	grantHandlerProvider granthandlers.GrantHandlerProviderInterface
	scopeValidator       scope.ScopeValidatorInterface
	observabilitySvc     providers.ObservabilityProvider
	dpopVerifier         dpop.VerifierInterface
	tokenEndpoint        string
	dpopRequired         bool
}

// newTokenService creates a new instance of tokenService.
func newTokenService(
	grantHandlerProvider granthandlers.GrantHandlerProviderInterface,
	scopeValidator scope.ScopeValidatorInterface,
	observabilitySvc providers.ObservabilityProvider,
	dpopVerifier dpop.VerifierInterface,
	tokenEndpoint string,
	dpopRequired bool,
) TokenServiceInterface {
	return &tokenService{
		grantHandlerProvider: grantHandlerProvider,
		scopeValidator:       scopeValidator,
		observabilitySvc:     observabilitySvc,
		dpopVerifier:         dpopVerifier,
		tokenEndpoint:        tokenEndpoint,
		dpopRequired:         dpopRequired,
	}
}

// ProcessTokenRequest validates and processes an OAuth 2.0 token request.
func (ts *tokenService) ProcessTokenRequest(
	ctx context.Context,
	tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient,
) (*model.TokenResponse, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TokenService"))

	startTime := time.Now().UnixMilli()
	clientID := tokenRequest.ClientID
	grantTypeStr := tokenRequest.GrantType
	scopeStr := tokenRequest.Scope

	ts.publishTokenIssuanceStartedEvent(ctx, oauthApp, clientID, grantTypeStr, scopeStr)

	// Validate grant_type presence.
	if grantTypeStr == "" {
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, oauthApp, clientID, grantTypeStr, scopeStr,
			400, "Missing grant_type parameter", startTime)
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Missing grant_type parameter",
		}
	}

	// Validate grant_type value.
	grantType := providers.GrantType(grantTypeStr)
	if !grantType.IsValid() {
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, oauthApp, clientID, grantTypeStr, scopeStr,
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
			publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, oauthApp, clientID, grantTypeStr, scopeStr,
				400, "Unsupported grant type", startTime)
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorUnsupportedGrantType,
				ErrorDescription: "Unsupported grant type",
			}
		}
		logger.Error(ctx, "Failed to get grant handler", log.Error(handlerErr))
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, oauthApp, clientID, grantTypeStr, scopeStr,
			500, "Failed to get grant handler", startTime)
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to process token request",
		}
	}

	// Validate grant type against the application.
	if !oauthApp.IsAllowedGrantType(grantType) {
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, oauthApp, clientID, grantTypeStr, scopeStr,
			401, "Client not authorized for grant type", startTime)
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorUnauthorizedClient,
			ErrorDescription: "The client is not authorized to use this grant type",
		}
	}

	// Validate the token request via the grant handler.
	tokenError := grantHandler.ValidateGrant(ctx, tokenRequest, oauthApp)
	if tokenError != nil && tokenError.Error != "" {
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, oauthApp, clientID, grantTypeStr, scopeStr,
			400, tokenError.ErrorDescription, startTime)
		return nil, tokenError
	}

	// Validate and filter scopes.
	validScopes, scopeError := ts.scopeValidator.ValidateScopes(ctx, tokenRequest.Scope, oauthApp.ClientID)
	if scopeError != nil {
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, oauthApp, clientID, grantTypeStr, scopeStr,
			400, scopeError.ErrorDescription, startTime)
		return nil, &model.ErrorResponse{
			Error:            scopeError.Error,
			ErrorDescription: scopeError.ErrorDescription,
		}
	}
	tokenRequest.Scope = validScopes

	dpopErr := ts.verifyDPoPProof(&ctx, oauthApp)
	if dpopErr != nil {
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, oauthApp, clientID, grantTypeStr, scopeStr,
			400, dpopErr.ErrorDescription, startTime)
		return nil, dpopErr
	}

	// Delegate to the grant handler for token generation.
	tokenRespDTO, tokenError := grantHandler.HandleGrant(ctx, tokenRequest, oauthApp)
	if tokenError != nil {
		if tokenError.Error != "" {
			code := 400
			if tokenError.Error == constants.ErrorServerError {
				code = 500
			}
			publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, oauthApp, clientID, grantTypeStr, scopeStr,
				code, tokenError.ErrorDescription, startTime)
			if tokenError.Error == constants.ErrorServerError {
				tokenError.ErrorDescription = "Failed to process token request"
			}
		}
		return nil, tokenError
	}
	if tokenRespDTO == nil {
		publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, oauthApp, clientID, grantTypeStr, scopeStr,
			500, "Grant handler returned empty response", startTime)
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to process token request",
		}
	}

	// Issue refresh token if applicable.
	if grantType.IssuesRefreshToken() &&
		oauthApp.IsAllowedGrantType(providers.GrantTypeRefreshToken) {
		logger.Debug(ctx, "Issuing refresh token for the token request",
			log.String("client_id", clientID), log.String("grant_type", grantTypeStr))

		refreshGrantHandler, handlerErr := ts.grantHandlerProvider.GetGrantHandler(providers.GrantTypeRefreshToken)
		if handlerErr != nil {
			logger.Error(ctx, "Failed to get refresh grant handler", log.Error(handlerErr))
			publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, oauthApp, clientID, grantTypeStr, scopeStr,
				500, "Failed to get refresh grant handler", startTime)
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to process token request",
			}
		}
		refreshGrantHandlerTyped, ok := refreshGrantHandler.(granthandlers.RefreshTokenGrantHandlerInterface)
		if !ok {
			logger.Error(ctx, "Failed to cast refresh grant handler",
				log.String("client_id", clientID), log.String("grant_type", grantTypeStr))
			publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, oauthApp, clientID, grantTypeStr, scopeStr,
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
			tokenRespDTO.AccessToken.TokenFamilyID,
			0,
		)
		if refreshTokenError != nil && refreshTokenError.Error != "" {
			publishTokenIssuanceFailedEvent(ts.observabilitySvc, ctx, oauthApp, clientID, grantTypeStr, scopeStr,
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
	if grantType == providers.GrantTypeTokenExchange {
		requestedTokenType := tokenRequest.RequestedTokenType
		switch {
		case requestedTokenType == string(constants.TokenTypeIdentifierIDJAG):
			tokenResponse.IssuedTokenType = string(constants.TokenTypeIdentifierIDJAG)
		case requestedTokenType == "" || requestedTokenType == string(constants.TokenTypeIdentifierAccessToken):
			tokenResponse.IssuedTokenType = string(constants.TokenTypeIdentifierAccessToken)
		default:
			tokenResponse.IssuedTokenType = string(constants.TokenTypeIdentifierJWT)
		}
	}

	logger.Debug(ctx, "Token generated successfully",
		log.String("client_id", clientID), log.String("grant_type", grantTypeStr))

	ts.publishTokenIssuedEvent(ctx, oauthApp, tokenRespDTO, clientID, grantTypeStr, scopes, startTime)

	return tokenResponse, nil
}

// verifyDPoPProof validates the DPoP proof when present and stores the resulting jkt
// in ctx for downstream grant handlers. A missing proof is rejected when the client
// requires dpop-bound access tokens or oauth.dpop.required is true.
func (ts *tokenService) verifyDPoPProof(ctx *context.Context, oauthApp *providers.OAuthClient) *model.ErrorResponse {
	proof := dpop.GetProof(*ctx)
	if proof == "" {
		if (oauthApp != nil && oauthApp.DPoPBoundAccessTokens) || ts.dpopRequired {
			return &model.ErrorResponse{
				Error:            constants.ErrorInvalidDPoPProof,
				ErrorDescription: "DPoP proof is required for this client",
			}
		}
		return nil
	}
	if ts.dpopVerifier == nil {
		return &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "DPoP verifier not configured",
		}
	}
	result, err := ts.dpopVerifier.Verify(*ctx, dpop.VerifyParams{
		Proof: proof,
		HTM:   "POST",
		HTU:   ts.tokenEndpoint,
	})
	if err != nil {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidDPoPProof,
			ErrorDescription: err.Error(),
		}
	}
	*ctx = dpop.WithJkt(*ctx, result.JKT)
	return nil
}

// publishTokenIssuanceStartedEvent publishes an event indicating that token issuance has started.
func (ts *tokenService) publishTokenIssuanceStartedEvent(
	ctx context.Context, oauthApp *providers.OAuthClient, clientID, grantType, scope string,
) {
	if ts.observabilitySvc == nil || !ts.observabilitySvc.IsEnabled() {
		return
	}

	evt := event.NewEvent(
		sysContext.GetTraceID(ctx),
		string(event.EventTypeTokenIssuanceStarted),
		event.ComponentAuthHandler,
	).
		WithStatus(providers.StatusInProgress).
		WithData(event.DataKey.ClientID, clientID).
		WithData(event.DataKey.GrantType, grantType).
		WithData(event.DataKey.Scope, scope).
		WithData(event.DataKey.CorrelationID, sysContext.GetTraceID(ctx))
	addActorData(evt, oauthApp)

	ts.observabilitySvc.PublishEvent(ctx, evt)
}

func (ts *tokenService) publishTokenIssuedEvent(
	ctx context.Context, oauthApp *providers.OAuthClient, tokenRespDTO *model.TokenResponseDTO,
	clientID, grantType, scope string, startTime int64,
) {
	if ts.observabilitySvc == nil || !ts.observabilitySvc.IsEnabled() {
		return
	}

	duration := time.Now().UnixMilli() - startTime

	// A grant carries a correlation identifier only when it originated in a login flow (via the
	// authorization code). Grants issued in a single request correlate on their own trace id.
	correlationID := tokenRespDTO.CorrelationID
	if correlationID == "" {
		correlationID = sysContext.GetTraceID(ctx)
	}

	evt := event.NewEvent(
		sysContext.GetTraceID(ctx),
		string(event.EventTypeTokenIssued),
		event.ComponentAuthHandler,
	).
		WithStatus(providers.StatusSuccess).
		WithData(event.DataKey.ClientID, clientID).
		WithData(event.DataKey.GrantType, grantType).
		WithData(event.DataKey.Scope, scope).
		WithData(event.DataKey.CorrelationID, correlationID).
		WithData(event.DataKey.DurationMs, fmt.Sprintf("%d", duration))
	addActorData(evt, oauthApp)
	addSubjectData(evt, &tokenRespDTO.AccessToken)

	ts.observabilitySvc.PublishEvent(ctx, evt)
}

// publishTokenIssuanceFailedEvent is a package-level helper shared by tokenService and tokenHandler.
// oauthApp is nil when the request failed before the client was resolved.
func publishTokenIssuanceFailedEvent(
	svc providers.ObservabilityProvider,
	ctx context.Context, oauthApp *providers.OAuthClient,
	clientID, grantType, scope string, statusCode int, message string, startTime int64,
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
		WithStatus(providers.StatusFailure).
		WithData(event.DataKey.ClientID, clientID).
		WithData(event.DataKey.GrantType, grantType).
		WithData(event.DataKey.Scope, scope).
		WithData(event.DataKey.Error, map[string]interface{}{
			"code":    fmt.Sprintf("%d", statusCode),
			"type":    errorType,
			"message": message,
		}).
		WithData(event.DataKey.CorrelationID, sysContext.GetTraceID(ctx)).
		WithData(event.DataKey.DurationMs, fmt.Sprintf("%d", duration))
	addActorData(evt, oauthApp)

	svc.PublishEvent(ctx, evt)
}

// addActorData stamps the acting principal onto a token issuance event: the entity category of the
// principal that authenticated as the client (agent or app) and its resource ID, which
// cross-references the app_id reported by flow events for the same principal. A nil client leaves
// the event unchanged.
func addActorData(evt *providers.Event, oauthApp *providers.OAuthClient) {
	if oauthApp == nil {
		return
	}
	if actorType := event.PrincipalType(string(oauthApp.EntityCategory)); actorType != "" {
		evt.WithData(event.DataKey.ActorType, actorType)
	}
	if oauthApp.ID != "" {
		evt.WithData(event.DataKey.EntityID, oauthApp.ID)
	}
}

// addSubjectData stamps the issued token's subject and its delegation relationship onto an event, so
// an agent acting for a user is observable as the actor against that user's subject (RFC 8693). The
// subject is reported as its entity resource ID, resolved while the token is built: the token's own
// sub claim is configurable per application through SubjectAttribute and may be a directly
// identifying attribute, which does not belong in the observability sinks.
func addSubjectData(evt *providers.Event, accessToken *model.TokenDTO) {
	if accessToken == nil {
		return
	}
	if accessToken.SubjectID != "" {
		evt.WithData(event.DataKey.Subject, accessToken.SubjectID)
		if subjectType := event.PrincipalType(accessToken.SubjectCategory); subjectType != "" {
			evt.WithData(event.DataKey.SubjectType, subjectType)
		}
	}
	evt.WithData(event.DataKey.IsDelegated, accessToken.ActorID != "")
	if accessToken.ActorID != "" {
		evt.WithData(event.DataKey.ActorSub, accessToken.ActorID)
	}
}
