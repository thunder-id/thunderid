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

package ciba

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	flowcm "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// cibaMaxBindingMessageLength is the maximum number of characters allowed in a binding_message.
const cibaMaxBindingMessageLength = 256

// CIBAServiceInterface defines the interface for the CIBA backchannel authentication service.
// It covers the full lifecycle: initiation, callback, and token-endpoint polling operations.
// The grant handler uses this interface instead of the raw store so the store stays private.
type CIBAServiceInterface interface {
	InitiateBackchannelAuth(
		ctx context.Context, request *BackchannelAuthRequest, oauthApp *inboundmodel.OAuthClient,
	) (*BackchannelAuthResponse, *CIBAError)
	HandleCallback(ctx context.Context, authReqID, assertion string) *CIBAError

	// Polling operations used by the CIBA grant handler at the token endpoint.
	GetByAuthReqID(ctx context.Context, authReqID string) (*CIBAAuthRequest, error)
	UpdateLastPolled(ctx context.Context, authReqID string, polledAt time.Time) error
	UpdateState(ctx context.Context, authReqID string, state CIBARequestState) error
	MarkConsumed(ctx context.Context, authReqID string) (bool, error)
}

// cibaService implements the CIBAServiceInterface.
type cibaService struct {
	store           CIBARequestStoreInterface
	flowExecService flowexec.FlowExecServiceInterface
	jwtService      jwt.JWTServiceInterface
	inboundClient   inboundclient.InboundClientServiceInterface
	resourceService resource.ResourceServiceInterface
	logger          *log.Logger
}

// newCIBAService creates a new instance of cibaService with injected dependencies.
func newCIBAService(
	store CIBARequestStoreInterface,
	flowExecService flowexec.FlowExecServiceInterface,
	jwtService jwt.JWTServiceInterface,
	inboundClient inboundclient.InboundClientServiceInterface,
	resourceService resource.ResourceServiceInterface,
) CIBAServiceInterface {
	return &cibaService{
		store:           store,
		flowExecService: flowExecService,
		jwtService:      jwtService,
		inboundClient:   inboundClient,
		resourceService: resourceService,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CIBAService")),
	}
}

// InitiateBackchannelAuth validates the request, resolves the user, initiates an authentication flow,
// persists a CIBA request, and returns the auth_req_id with polling metadata.
func (s *cibaService) InitiateBackchannelAuth(
	ctx context.Context, request *BackchannelAuthRequest, oauthApp *inboundmodel.OAuthClient,
) (*BackchannelAuthResponse, *CIBAError) {
	if !oauthApp.IsAllowedGrantType(oauth2const.GrantTypeCIBA) {
		return nil, &CIBAError{
			Code:    oauth2const.ErrorUnauthorizedClient,
			Message: "The client is not authorized to use the CIBA grant type",
		}
	}

	scopes := utils.ParseStringArray(request.Scope, " ")
	if validationErr := validateBackchannelAuthRequest(request, scopes); validationErr != nil {
		return nil, validationErr
	}

	authReqID, err := utils.GenerateUUIDv7()
	if err != nil {
		s.logger.Error(ctx, "Failed to generate auth_req_id", log.Error(err))
		return nil, &CIBAError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication request",
		}
	}

	expiresIn := resolveExpiresIn(request.RequestedExpiry)

	// Separate OIDC (standard) and permission scopes — mirrors the authorization_code path exactly.
	// auth code uses SeparateOIDCAndNonOIDCScopes + ResolveAndDownscope; no scopeValidator needed.
	oidcScopes, permissionScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(request.Scope, oauthApp.ScopeClaims)

	// Validate permission scopes against resource server definitions
	// Unknown permission scopes are silently dropped; unknown resource servers cause invalid_target.
	_, permissionScopes, rsErr := resourceindicators.ResolveAndDownscope(
		ctx, s.resourceService, []string{}, permissionScopes)
	if rsErr != nil {
		return nil, &CIBAError{Code: rsErr.Error, Message: rsErr.ErrorDescription}
	}
	cacheTTL := strconv.FormatInt(resolveUserAttributesCacheTTL(oauthApp), 10)

	// authReqID is injected into runtime data for two reasons:
	//   a) auth_assert_executor binds it as the ciba_auth_req_id claim in the assertion JWT,
	//      enabling the callback to verify this assertion authorizes this specific request.
	//   b) invite_executor includes it in the notification link URL so Gate UI can pass it
	//      back in the CIBA callback call on flow completion.
	bindingMessage := request.BindingMessage
	if bindingMessage == "" {
		bindingMessage = defaultBindingMessage(authReqID)
	}

	runtimeData := map[string]string{
		flowcm.RuntimeKeyCIBAAuthReqID:               authReqID,
		flowcm.RuntimeKeyClientID:                    oauthApp.ClientID,
		flowcm.RuntimeKeyRequestedPermissions:        utils.StringifyStringArray(permissionScopes, " "),
		flowcm.RuntimeKeyRequiredEssentialAttributes: "",
		flowcm.RuntimeKeyRequiredOptionalAttributes: getRequiredOptionalAttributes(
			append(oidcScopes, permissionScopes...), oauthApp),
		flowcm.RuntimeKeyUserAttributesCacheTTLSeconds: cacheTTL,
		flowcm.RuntimeKeyBindingMessage:                bindingMessage,
	}
	if request.ACRValues != "" {
		runtimeData[flowcm.RuntimeKeyRequestedAuthClasses] = request.ACRValues
	}

	// Execute the flow server-side. login_hint is placed in InitialInputs so the identifying
	// executor resolves the user from ctx.UserInputs without an interactive input step.
	// authReqID is in RuntimeData so invite_executor can include it in the notification link
	// and auth_assert_executor can bind it as the ciba_auth_req_id claim in the assertion.
	// The flow runs until it pauses at the display-only prompt node placed after the
	// notification executor, signaling that the notification has been sent to the user.
	flowStep, flowErr := s.flowExecService.InitiateAndExecute(ctx, &flowexec.FlowInitContext{
		ApplicationID: oauthApp.ID,
		FlowType:      string(flowcm.FlowTypeAuthentication),
		RuntimeData:   runtimeData,
		ExpirySeconds: expiresIn,
		InitialInputs: map[string]string{
			flowcm.UserInputKeyLoginHint: request.LoginHint,
		},
	})
	if flowErr != nil {
		s.logger.Error(ctx, "Failed to initiate and execute CIBA authentication flow",
			log.String("error_code", flowErr.Code))
		return nil, &CIBAError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication request",
		}
	}

	if flowStep.Status == flowcm.FlowStatusError {
		return nil, mapFlowErrorToCIBAError(flowStep.Error.Error.DefaultValue)
	}

	now := time.Now()
	cibaRequest := &CIBAAuthRequest{
		AuthReqID:      authReqID,
		ClientID:       oauthApp.ClientID,
		StandardScopes: utils.StringifyStringArray(oidcScopes, " "),
		State:          CIBAStatePending,
		ExpiryTime:     now.Add(time.Duration(expiresIn) * time.Second),
	}
	if storeErr := s.store.Add(ctx, cibaRequest); storeErr != nil {
		s.logger.Error(ctx, "Failed to store CIBA authentication request", log.Error(storeErr))
		return nil, &CIBAError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication request",
		}
	}

	return &BackchannelAuthResponse{
		AuthReqID: authReqID,
		ExpiresIn: expiresIn,
		Interval:  oauth2const.CIBADefaultIntervalSeconds,
	}, nil
}

// HandleCallback verifies the flow assertion, enforces the sub binding, and marks the request authenticated.
func (s *cibaService) HandleCallback(ctx context.Context, authReqID, assertion string) *CIBAError {
	if authReqID == "" || assertion == "" {
		return &CIBAError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "auth_req_id and assertion are required",
		}
	}

	record, err := s.store.GetByID(ctx, authReqID)
	if err != nil {
		if errors.Is(err, ErrCIBARequestNotFound) {
			return &CIBAError{
				Code:    oauth2const.ErrorInvalidRequest,
				Message: "Invalid auth_req_id",
			}
		}
		s.logger.Error(ctx, "Failed to retrieve CIBA authentication request", log.Error(err))
		return &CIBAError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication callback",
		}
	}

	if record.State != CIBAStatePending {
		return &CIBAError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Backchannel authentication request is not pending",
		}
	}
	if record.ExpiryTime.Before(time.Now()) {
		return &CIBAError{
			Code:    oauth2const.ErrorExpiredToken,
			Message: "Backchannel authentication request has expired",
		}
	}

	// Resolve the owning client to obtain the expected audience (the app entity ID used as the
	// assertion `aud`) for defense-in-depth audience validation during signature verification.
	expectedAud := s.resolveExpectedAudience(ctx, record.ClientID)

	if verifyErr := s.jwtService.VerifyJWT(ctx, assertion, expectedAud, ""); verifyErr != nil {
		s.logger.Debug(ctx, "Assertion verification failed",
			log.String("error", verifyErr.Error.DefaultValue))
		return &CIBAError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Invalid assertion signature",
		}
	}

	claims, authTime, decodeErr := decodeAttributesFromAssertion(assertion)
	if decodeErr != nil {
		s.logger.Error(ctx, "Failed to decode assertion claims", log.Error(decodeErr))
		return &CIBAError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication callback",
		}
	}

	// Bind the assertion to this specific CIBA request. The auth_req_id is threaded through the
	// flow runtime data into the assertion as the ciba_auth_req_id claim; requiring it to match the
	// record prevents an assertion minted for one CIBA request from authorizing another (e.g. a
	// narrow-scope authentication being replayed against a broader-scope request for the same user).
	if claims.cibaAuthReqID == "" || claims.cibaAuthReqID != record.AuthReqID {
		s.logger.Debug(ctx, "Assertion is not bound to the backchannel authentication request",
			log.MaskedString("auth_req_id", authReqID))
		return &CIBAError{
			Code:    oauth2const.ErrorAccessDenied,
			Message: "Assertion does not match the backchannel authentication request",
		}
	}

	// The user ID is first known here, extracted from the assertion sub claim.
	// It was not pre-resolved at request initiation; the flow's identifying executor
	// resolves it from the login_hint during server-side execution.
	if claims.userID == "" {
		s.logger.Debug(ctx, "Assertion subject is missing",
			log.MaskedString("auth_req_id", authReqID))
		return &CIBAError{
			Code:    oauth2const.ErrorAccessDenied,
			Message: "Assertion subject is missing",
		}
	}

	if authTime.IsZero() {
		authTime = time.Now()
	}

	// Build AuthorizedScopes — mirrors auth code callback exactly:
	//   StandardScopes (OIDC) were stored at initiation, no client lookup needed here.
	//   authorized_permissions from the assertion replaces the permission scopes.
	permissionScopeList := utils.ParseStringArray(claims.authorizedPermissions, " ")
	authorizedScopes := utils.StringifyStringArray(
		append(utils.ParseStringArray(record.StandardScopes, " "), permissionScopeList...),
		" ")

	if markErr := s.store.MarkAuthenticated(ctx, authReqID, claims.userID, authorizedScopes,
		claims.attributeCacheID, claims.completedACR, authTime); markErr != nil {
		s.logger.Error(ctx, "Failed to mark CIBA authentication request as authenticated",
			log.Error(markErr))
		return &CIBAError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication callback",
		}
	}

	return nil
}

// resolveExpectedAudience resolves the app entity ID for the given client ID, which the flow uses
// as the assertion `aud`. It returns an empty string (skipping the audience check) on lookup
// failure; the ciba_auth_req_id binding remains the primary protection in that case.
func (s *cibaService) resolveExpectedAudience(ctx context.Context, clientID string) string {
	app, err := s.inboundClient.GetOAuthClientByClientID(ctx, clientID)
	if err != nil {
		s.logger.Warn(ctx, "Failed to resolve client for audience validation; skipping audience check",
			log.Error(err))
		return ""
	}
	if app == nil {
		return ""
	}
	return app.ID
}

// mapFlowErrorToCIBAError maps a flow failure reason to the appropriate CIBA error.
// User-not-found and ambiguous-user failures map to unknown_user_id per CIBA Core 1.0 §7.3.
func mapFlowErrorToCIBAError(failureReason string) *CIBAError {
	switch failureReason {
	case "User not found", "User identity is ambiguous":
		return &CIBAError{
			Code:    oauth2const.ErrorUnknownUserID,
			Message: "Unable to resolve the user for the provided login_hint",
		}
	default:
		return &CIBAError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication request",
		}
	}
}

// validateBackchannelAuthRequest validates the required parameters of a backchannel authentication request.
func validateBackchannelAuthRequest(request *BackchannelAuthRequest, scopes []string) *CIBAError {
	if request.LoginHint == "" {
		return &CIBAError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "login_hint is required",
		}
	}
	if len(scopes) == 0 {
		return &CIBAError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "scope is required",
		}
	}
	if !slices.Contains(scopes, oauth2const.ScopeOpenID) {
		return &CIBAError{
			Code:    oauth2const.ErrorInvalidScope,
			Message: "scope must include openid",
		}
	}
	if bindingErr := validateBindingMessage(request.BindingMessage); bindingErr != nil {
		return bindingErr
	}
	return nil
}

// validateBindingMessage enforces the CIBA Core 1.0 §7.1 constraints on the optional binding_message:
// it must be relatively short and consist of plain, printable characters suitable for display.
func validateBindingMessage(bindingMessage string) *CIBAError {
	if bindingMessage == "" {
		return nil
	}
	if utf8.RuneCountInString(bindingMessage) > cibaMaxBindingMessageLength {
		return &CIBAError{
			Code:    oauth2const.ErrorInvalidBindingMessage,
			Message: "binding_message exceeds the maximum allowed length",
		}
	}
	for _, r := range bindingMessage {
		if !unicode.IsPrint(r) {
			return &CIBAError{
				Code:    oauth2const.ErrorInvalidBindingMessage,
				Message: "binding_message contains unsupported characters",
			}
		}
	}
	return nil
}

// resolveExpiresIn clamps a client-requested expiry to the server maximum, defaulting when unset or invalid.
func resolveExpiresIn(requestedExpiry string) int64 {
	if requestedExpiry == "" {
		return oauth2const.CIBADefaultExpiresInSeconds
	}
	requested, err := strconv.ParseInt(strings.TrimSpace(requestedExpiry), 10, 64)
	if err != nil || requested <= 0 {
		return oauth2const.CIBADefaultExpiresInSeconds
	}
	if requested > oauth2const.CIBAMaxExpiresInSeconds {
		return oauth2const.CIBAMaxExpiresInSeconds
	}
	return requested
}

// defaultBindingMessage returns a generic authorization context message used when the client
// does not supply a binding_message. The message is shown to the user in the notification
// so they understand why authentication is being requested.
// defaultBindingMessage derives a request-specific message from authReqID so concurrent
// requests can be correlated by the user. The short code is extracted from the UUID.
func defaultBindingMessage(authReqID string) string {
	clean := strings.ReplaceAll(authReqID, "-", "")
	if len(clean) < 8 {
		return "An authentication request has been initiated on your behalf. Please review and confirm."
	}
	code := strings.ToUpper(clean[:4] + "-" + clean[4:8])
	return "An authentication request has been initiated on your behalf. Code: " + code + ". Please review and confirm."
}

// resolveUserAttributesCacheTTL determines the TTL for caching user attributes during the flow.
// It mirrors the authorization code path: the largest of the access and refresh token (if allowed)
// validity periods is the base, plus the CIBA request lifetime to cover the poll window, plus a
// fixed buffer. Setting this in the flow runtime data is what makes the auth assertion cache the
// resolved attributes and emit the aci claim (consumed by the CIBA callback).
func resolveUserAttributesCacheTTL(app *inboundmodel.OAuthClient) int64 {
	maxTTL := tokenservice.ResolveTokenConfig(app, tokenservice.TokenTypeAccess).ValidityPeriod
	if app.IsAllowedGrantType(oauth2const.GrantTypeRefreshToken) {
		refreshTTL := tokenservice.ResolveTokenConfig(app, tokenservice.TokenTypeRefresh).ValidityPeriod
		if refreshTTL > maxTTL {
			maxTTL = refreshTTL
		}
	}
	return maxTTL + oauth2const.CIBAMaxExpiresInSeconds + oauth2const.AttributeCacheTTLBufferSeconds
}

// GetByAuthReqID retrieves a CIBA authentication request by ID. Used by the grant handler at the
// token endpoint to check request state without exposing the store directly.
func (s *cibaService) GetByAuthReqID(ctx context.Context, authReqID string) (*CIBAAuthRequest, error) {
	return s.store.GetByID(ctx, authReqID)
}

// UpdateLastPolled updates the last polled timestamp of a CIBA authentication request.
func (s *cibaService) UpdateLastPolled(ctx context.Context, authReqID string, polledAt time.Time) error {
	return s.store.UpdateLastPolled(ctx, authReqID, polledAt)
}

// UpdateState updates the state of a CIBA authentication request.
func (s *cibaService) UpdateState(ctx context.Context, authReqID string, state CIBARequestState) error {
	return s.store.UpdateState(ctx, authReqID, state)
}

// MarkConsumed atomically transitions an authenticated request to consumed.
func (s *cibaService) MarkConsumed(ctx context.Context, authReqID string) (bool, error) {
	return s.store.MarkConsumed(ctx, authReqID)
}

// decodeAttributesFromAssertion decodes claims from the flow assertion JWT using the shared
// base decoder, then extracts the CIBA-specific and permission claims.
func decodeAttributesFromAssertion(assertion string) (assertionClaims, time.Time, error) {
	base, payload, err := oauth2utils.DecodeFlowAssertionClaims(assertion)
	if err != nil {
		return assertionClaims{}, time.Time{}, fmt.Errorf("failed to decode assertion: %w", err)
	}

	claims := assertionClaims{
		userID:           base.UserID,
		attributeCacheID: base.AttributeCacheID,
		completedACR:     base.CompletedACR,
	}

	if cibaValue, ok := payload[oauth2const.ClaimCIBAAuthReqID]; ok {
		strValue, ok := cibaValue.(string)
		if !ok {
			return assertionClaims{}, time.Time{}, errors.New("JWT 'ciba_auth_req_id' claim is not a string")
		}
		claims.cibaAuthReqID = strValue
	}

	if v, ok := payload["authorized_permissions"].(string); ok {
		claims.authorizedPermissions = v
	}

	return claims, base.AuthTime, nil
}
