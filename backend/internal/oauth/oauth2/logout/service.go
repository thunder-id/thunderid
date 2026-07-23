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

// Package logout implements the OIDC RP-Initiated Logout 1.0 end_session_endpoint
// (GET/POST /oauth2/logout). It resolves the target application from id_token_hint (or client_id),
// validates any post_logout_redirect_uri against the client's registered list, and runs the
// application's sign-out flow to terminate the SSO session before landing the browser.
package logout

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

var (
	errInvalidIDTokenHint           = errors.New("invalid id_token_hint")
	errClientMismatch               = errors.New("client_id does not match id_token_hint")
	errClientRequired               = errors.New("id_token_hint or client_id is required")
	errInvalidClient                = errors.New("invalid client")
	errInvalidPostLogoutRedirectURI = errors.New("invalid post_logout_redirect_uri")
	errIDTokenHintRequired          = errors.New("id_token_hint is required when post_logout_redirect_uri is provided")
)

// LogoutRequest holds the RP-initiated logout parameters received from the request.
type LogoutRequest struct {
	IDTokenHint           string
	ClientID              string
	PostLogoutRedirectURI string
	State                 string
	Headers               map[string][]string
	QueryParams           map[string][]string
}

// LogoutResolution is the validated target of a logout request.
type LogoutResolution struct {
	AppID                 string
	PostLogoutRedirectURI string
	State                 string
	Headers               map[string][]string
	QueryParams           map[string][]string
}

// SignOutInitiation is the result of starting an RP-initiated sign-out: the stored logout-request id
// (echoed to the gate and returned on the completion callback) and the sign-out flow execution id.
type SignOutInitiation struct {
	LogoutID    string
	ExecutionID string
}

// LogoutServiceInterface validates an RP-initiated logout request, resolves its target, initiates the
// application's sign-out flow, and completes it (issuing the post-logout redirect).
type LogoutServiceInterface interface {
	Resolve(ctx context.Context, req LogoutRequest) (*LogoutResolution, error)
	InitiateSignOutFlow(ctx context.Context, resolution *LogoutResolution) (*SignOutInitiation, *tidcommon.ServiceError)
	CompleteSignOut(ctx context.Context, logoutID string) (string, error)
}

// logoutService is the default LogoutServiceInterface implementation. It verifies the id_token_hint,
// resolves the target client (and its post-logout redirect allow-list) via the actor provider, drives
// the application's sign-out flow through the flow-exec service, and persists the in-progress logout
// request in its store so the completion callback can issue the post-logout redirect.
type logoutService struct {
	jwtService      jwt.JWTServiceInterface
	actorProvider   providers.ActorProvider
	flowExecService flowexec.FlowExecServiceInterface
	store           logoutRequestStoreInterface
	issuer          string
	logger          *log.Logger
}

func newLogoutService(jwtService jwt.JWTServiceInterface, actorProvider providers.ActorProvider,
	flowExecService flowexec.FlowExecServiceInterface, store logoutRequestStoreInterface,
	issuer string) *logoutService {
	return &logoutService{
		jwtService:      jwtService,
		actorProvider:   actorProvider,
		flowExecService: flowExecService,
		store:           store,
		issuer:          issuer,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "LogoutService")),
	}
}

// InitiateSignOutFlow persists the validated logout target server-side and initiates the application's
// sign-out flow. It returns the stored logout-request id (which the gate echoes back on completion) and
// the flow execution id. The post-logout landing is kept out of the flow entirely — OAuth resolves it on
// the completion callback — keeping the flow engine protocol-agnostic.
func (s *logoutService) InitiateSignOutFlow(
	ctx context.Context, resolution *LogoutResolution,
) (*SignOutInitiation, *tidcommon.ServiceError) {
	logoutID, err := s.store.AddRequest(ctx, logoutRequestContext{
		AppID:                 resolution.AppID,
		PostLogoutRedirectURI: resolution.PostLogoutRedirectURI,
		State:                 resolution.State,
	})
	if err != nil {
		s.logger.Error(ctx, "Failed to persist logout request", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	// id_token_hint is used by the OAuth layer to resolve the target client; the sign-out flow
	// itself never consumes it. Strip it from the forwarded initiator request so we don't persist a
	// JWT with user identity claims into the flow context store.
	forwardedQueryParams := filterQueryParams(resolution.QueryParams, constants.RequestParamIDTokenHint)

	executionID, svcErr := s.flowExecService.InitiateFlow(ctx, &flowexec.FlowInitContext{
		ApplicationID: resolution.AppID,
		FlowType:      string(providers.FlowTypeSignOut),
		InitiatorRequest: &providers.InitiatorRequest{
			Headers:     sysutils.FilterSensitiveHeaders(resolution.Headers),
			QueryParams: forwardedQueryParams,
		},
	})
	if svcErr != nil {
		return nil, svcErr
	}

	return &SignOutInitiation{LogoutID: logoutID, ExecutionID: executionID}, nil
}

// CompleteSignOut is invoked after the sign-out flow completes. It consumes the stored logout request and
// returns the post-logout redirect URI (with state appended), or "" when the RP supplied none or the
// request is unknown/expired. Protocol-level actions that must run on sign-out (e.g. token revocation)
// belong here — the OAuth layer regains control at this point, which it cannot inside the flow.
func (s *logoutService) CompleteSignOut(ctx context.Context, logoutID string) (string, error) {
	found, reqCtx, err := s.store.GetRequest(ctx, logoutID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	// Consume the request so a logout id cannot be replayed.
	if clearErr := s.store.ClearRequest(ctx, logoutID); clearErr != nil {
		s.logger.Warn(ctx, "Failed to clear logout request", log.Error(clearErr))
	}

	if reqCtx.PostLogoutRedirectURI == "" {
		return "", nil
	}
	if reqCtx.State == "" {
		return reqCtx.PostLogoutRedirectURI, nil
	}
	redirectURI, err := oauth2utils.GetURIWithQueryParams(
		reqCtx.PostLogoutRedirectURI, map[string]string{constants.RequestParamState: reqCtx.State})
	if err != nil {
		return "", err
	}
	return redirectURI, nil
}

// Resolve identifies the client from id_token_hint (preferred) or the client_id parameter, validates
// any post_logout_redirect_uri against the client's registered list, and returns the logout target.
func (s *logoutService) Resolve(ctx context.Context, req LogoutRequest) (*LogoutResolution, error) {
	// Per OIDC RP-Initiated Logout, if post_logout_redirect_uri is supplied the id_token_hint MUST be
	// supplied too; the OP must not redirect to the URI without a valid hint.
	if req.PostLogoutRedirectURI != "" && req.IDTokenHint == "" {
		return nil, errIDTokenHintRequired
	}

	clientID := req.ClientID
	if req.IDTokenHint != "" {
		hintClientID, err := s.clientIDFromIDTokenHint(ctx, req.IDTokenHint)
		if err != nil {
			return nil, err
		}
		if clientID != "" && hintClientID != "" && clientID != hintClientID {
			return nil, errClientMismatch
		}
		if clientID == "" {
			clientID = hintClientID
		}
	}
	if clientID == "" {
		return nil, errClientRequired
	}

	client, svcErr := s.actorProvider.GetOAuthClientByClientID(ctx, clientID)
	if svcErr != nil {
		// An unresolvable client is the caller's fault (unknown client id); log at debug, not error.
		if svcErr.Type == tidcommon.ClientErrorType {
			s.logger.Debug(ctx, "Client not found for logout", log.String("clientId", clientID))
		} else {
			s.logger.Error(ctx, "Failed to resolve client for logout", log.String("clientId", clientID))
		}
		return nil, errInvalidClient
	}
	if client == nil {
		return nil, errInvalidClient
	}

	if err := client.ValidatePostLogoutRedirectURI(ctx, req.PostLogoutRedirectURI); err != nil {
		return nil, errInvalidPostLogoutRedirectURI
	}

	return &LogoutResolution{
		AppID:                 client.ID,
		PostLogoutRedirectURI: req.PostLogoutRedirectURI,
		State:                 req.State,
		Headers:               req.Headers,
		QueryParams:           req.QueryParams,
	}, nil
}

// clientIDFromIDTokenHint verifies the id_token_hint was issued by this server (signature + issuer)
// and returns its audience (the client id). The token's expiry is intentionally not enforced: per
// OIDC RP-Initiated Logout, id_token_hint may be an expired ID token.
func (s *logoutService) clientIDFromIDTokenHint(ctx context.Context, idTokenHint string) (string, error) {
	if svcErr := s.jwtService.VerifyJWTSignature(ctx, idTokenHint); svcErr != nil {
		return "", errInvalidIDTokenHint
	}
	payload, err := jwt.DecodeJWTPayload(idTokenHint)
	if err != nil {
		return "", errInvalidIDTokenHint
	}
	if iss, _ := payload[constants.ClaimIss].(string); iss != s.issuer {
		return "", errInvalidIDTokenHint
	}
	return audienceClientID(payload), nil
}

// filterQueryParams returns a copy of the given query-parameter map with the named keys removed.
func filterQueryParams(params map[string][]string, exclude ...string) map[string][]string {
	if len(params) == 0 {
		return params
	}
	excluded := make(map[string]struct{}, len(exclude))
	for _, name := range exclude {
		excluded[name] = struct{}{}
	}
	filtered := make(map[string][]string, len(params))
	for name, values := range params {
		if _, drop := excluded[name]; drop {
			continue
		}
		filtered[name] = values
	}
	return filtered
}

// audienceClientID extracts the client id from an ID token. When the token has multiple audiences the
// authorized party (azp) claim identifies the client, so it is preferred; otherwise the aud claim,
// which may be a single string or an array of strings, is used.
func audienceClientID(payload map[string]interface{}) string {
	if azp, ok := payload[constants.ClaimAzp].(string); ok && azp != "" {
		return azp
	}
	switch aud := payload[constants.ClaimAud].(type) {
	case string:
		return aud
	case []interface{}:
		if len(aud) > 0 {
			if first, ok := aud[0].(string); ok {
				return first
			}
		}
	}
	return ""
}
