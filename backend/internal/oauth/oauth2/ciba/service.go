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
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	flowcm "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/oauth/scope"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// cibaMaxBindingMessageLength is the maximum number of characters allowed in a binding_message.
const cibaMaxBindingMessageLength = 100

// CIBAServiceInterface defines the interface for the CIBA backchannel authentication service.
type CIBAServiceInterface interface {
	InitiateBackchannelAuth(
		ctx context.Context, request *BackchannelAuthRequest, oauthApp *inboundmodel.OAuthClient,
	) (*BackchannelAuthResponse, *cibaError)
	HandleCallback(ctx context.Context, authReqID, assertion string) *cibaError
}

// BackchannelAuthRequest carries the parsed parameters of a backchannel authentication request.
type BackchannelAuthRequest struct {
	LoginHint       string
	Scope           string
	BindingMessage  string
	RequestedExpiry string
}

// cibaService implements the CIBAServiceInterface.
type cibaService struct {
	store           CIBARequestStoreInterface
	flowExecService flowexec.FlowExecServiceInterface
	entityProvider  entityprovider.EntityProviderInterface
	jwtService      jwt.JWTServiceInterface
	inboundClient   inboundclient.InboundClientServiceInterface
	scopeValidator  scope.ScopeValidatorInterface
	logger          *log.Logger
}

// newCIBAService creates a new instance of cibaService with injected dependencies.
func newCIBAService(
	store CIBARequestStoreInterface,
	flowExecService flowexec.FlowExecServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	jwtService jwt.JWTServiceInterface,
	inboundClient inboundclient.InboundClientServiceInterface,
	scopeValidator scope.ScopeValidatorInterface,
) CIBAServiceInterface {
	return &cibaService{
		store:           store,
		flowExecService: flowExecService,
		entityProvider:  entityProvider,
		jwtService:      jwtService,
		inboundClient:   inboundClient,
		scopeValidator:  scopeValidator,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CIBAService")),
	}
}

// InitiateBackchannelAuth validates the request, resolves the user, initiates an authentication flow,
// persists a CIBA request, and returns the auth_req_id with polling metadata.
func (s *cibaService) InitiateBackchannelAuth(
	ctx context.Context, request *BackchannelAuthRequest, oauthApp *inboundmodel.OAuthClient,
) (*BackchannelAuthResponse, *cibaError) {
	if !oauthApp.IsAllowedGrantType(oauth2const.GrantTypeCIBA) {
		return nil, &cibaError{
			Code:    oauth2const.ErrorUnauthorizedClient,
			Message: "The client is not authorized to use the CIBA grant type",
		}
	}

	scopes := utils.ParseStringArray(request.Scope, " ")
	if validationErr := validateBackchannelAuthRequest(request, scopes); validationErr != nil {
		return nil, validationErr
	}

	// Validate the requested scope against what the client is authorized for, mirroring the token
	// endpoint and authorization flow. The full validated scope string is persisted and used for
	// token issuance; only the permission (non-standard) subset is passed to the flow.
	validScope, scopeErr := s.scopeValidator.ValidateScopes(ctx, request.Scope, oauthApp.ClientID)
	if scopeErr != nil {
		return nil, &cibaError{
			Code:    scopeErr.Error,
			Message: scopeErr.ErrorDescription,
		}
	}
	validScopes := utils.ParseStringArray(validScope, " ")

	userID, resolveErr := s.resolveUser(request.LoginHint)
	if resolveErr != nil {
		return nil, resolveErr
	}

	expiresIn := resolveExpiresIn(request.RequestedExpiry)

	authReqID, err := utils.GenerateUUIDv7()
	if err != nil {
		s.logger.Error("Failed to generate auth_req_id", log.Error(err))
		return nil, &cibaError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication request",
		}
	}

	// Strip standard OIDC scopes so only permission scopes are exposed to the flow as requested
	// permissions, matching the authorization_code path. The full scope set is still persisted below.
	_, permissionScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(validScope, oauthApp.ScopeClaims)

	cacheTTL := strconv.FormatInt(resolveUserAttributesCacheTTL(oauthApp), 10)
	executionID, flowErr := s.flowExecService.InitiateFlow(ctx, &flowexec.FlowInitContext{
		ApplicationID: oauthApp.ID,
		FlowType:      string(flowcm.FlowTypeAuthentication),
		RuntimeData: map[string]string{
			flowcm.RuntimeKeyClientID:                      oauthApp.ClientID,
			flowcm.RuntimeKeyRequestedPermissions:          utils.StringifyStringArray(permissionScopes, " "),
			flowcm.RuntimeKeyRequiredEssentialAttributes:   "",
			flowcm.RuntimeKeyRequiredOptionalAttributes:    getRequiredOptionalAttributes(validScopes, oauthApp),
			flowcm.RuntimeKeyUserAttributesCacheTTLSeconds: cacheTTL,
			flowcm.RuntimeKeyCIBAAuthReqID:                 authReqID,
		},
	})
	if flowErr != nil {
		s.logger.Error("Failed to initiate authentication flow", log.String("error_code", flowErr.Code))
		return nil, &cibaError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication request",
		}
	}

	now := time.Now()
	cibaRequest := &CIBAAuthRequest{
		AuthReqID:   authReqID,
		ExecutionID: executionID,
		ClientID:    oauthApp.ClientID,
		UserID:      userID,
		Scopes:      utils.StringifyStringArray(validScopes, " "),
		State:       CIBAStatePending,
		ExpiryTime:  now.Add(time.Duration(expiresIn) * time.Second),
	}
	if storeErr := s.store.Add(ctx, cibaRequest); storeErr != nil {
		s.logger.Error("Failed to store CIBA authentication request", log.Error(storeErr))
		return nil, &cibaError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication request",
		}
	}

	notificationURL, urlErr := buildNotificationURL(executionID, authReqID, request.BindingMessage)
	if urlErr != nil {
		s.logger.Error("Failed to build notification URL", log.Error(urlErr))
		return nil, &cibaError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication request",
		}
	}

	return &BackchannelAuthResponse{
		AuthReqID:       authReqID,
		ExpiresIn:       expiresIn,
		Interval:        oauth2const.CIBADefaultIntervalSeconds,
		NotificationURL: notificationURL,
	}, nil
}

// HandleCallback verifies the flow assertion, enforces the sub binding, and marks the request authenticated.
func (s *cibaService) HandleCallback(ctx context.Context, authReqID, assertion string) *cibaError {
	if authReqID == "" || assertion == "" {
		return &cibaError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "auth_req_id and assertion are required",
		}
	}

	record, err := s.store.GetByID(ctx, authReqID)
	if err != nil {
		if errors.Is(err, ErrCIBARequestNotFound) {
			return &cibaError{
				Code:    oauth2const.ErrorInvalidRequest,
				Message: "Invalid auth_req_id",
			}
		}
		s.logger.Error("Failed to retrieve CIBA authentication request", log.Error(err))
		return &cibaError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication callback",
		}
	}

	if record.State != CIBAStatePending {
		return &cibaError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Backchannel authentication request is not pending",
		}
	}
	if record.ExpiryTime.Before(time.Now()) {
		return &cibaError{
			Code:    oauth2const.ErrorExpiredToken,
			Message: "Backchannel authentication request has expired",
		}
	}

	// Resolve the owning client to obtain the expected audience (the app entity ID used as the
	// assertion `aud`) for defense-in-depth audience validation during signature verification.
	expectedAud := s.resolveExpectedAudience(ctx, record.ClientID)

	if verifyErr := s.jwtService.VerifyJWT(ctx, assertion, expectedAud, ""); verifyErr != nil {
		s.logger.Debug("Assertion verification failed",
			log.String("error", verifyErr.Error.DefaultValue))
		return &cibaError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Invalid assertion signature",
		}
	}

	claims, authTime, decodeErr := decodeAttributesFromAssertion(assertion)
	if decodeErr != nil {
		s.logger.Error("Failed to decode assertion claims", log.Error(decodeErr))
		return &cibaError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication callback",
		}
	}

	// Bind the assertion to this specific CIBA request. The auth_req_id is threaded through the
	// flow runtime data into the assertion as the ciba_auth_req_id claim; requiring it to match the
	// record prevents an assertion minted for one CIBA request from authorizing another (e.g. a
	// narrow-scope authentication being replayed against a broader-scope request for the same user).
	if claims.cibaAuthReqID == "" || claims.cibaAuthReqID != record.AuthReqID {
		s.logger.Debug("Assertion is not bound to the backchannel authentication request",
			log.MaskedString("auth_req_id", authReqID))
		return &cibaError{
			Code:    oauth2const.ErrorAccessDenied,
			Message: "Assertion does not match the backchannel authentication request",
		}
	}

	if claims.userID == "" || claims.userID != record.UserID {
		s.logger.Debug("Assertion subject does not match the resolved user",
			log.MaskedString("auth_req_id", authReqID))
		return &cibaError{
			Code:    oauth2const.ErrorAccessDenied,
			Message: "Authenticated user does not match the backchannel authentication request",
		}
	}

	if authTime.IsZero() {
		authTime = time.Now()
	}
	if markErr := s.store.MarkAuthenticated(ctx, authReqID, claims.attributeCacheID,
		claims.completedACR, authTime); markErr != nil {
		s.logger.Error("Failed to mark CIBA authentication request as authenticated", log.Error(markErr))
		return &cibaError{
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
		s.logger.Warn("Failed to resolve client for audience validation; skipping audience check",
			log.Error(err))
		return ""
	}
	if app == nil {
		return ""
	}
	return app.ID
}

// resolveUser resolves the user ID from the login_hint username via the entity provider.
func (s *cibaService) resolveUser(loginHint string) (string, *cibaError) {
	userID, epErr := s.entityProvider.IdentifyEntity(map[string]interface{}{
		oauth2const.RequestParamUsername: loginHint,
	})
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeEntityNotFound ||
			epErr.Code == entityprovider.ErrorCodeAmbiguousEntity {
			return "", &cibaError{
				Code:    oauth2const.ErrorUnknownUserID,
				Message: "Unable to resolve the user for the provided login_hint",
			}
		}
		s.logger.Error("Failed to resolve user from login_hint", log.Error(epErr))
		return "", &cibaError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process backchannel authentication request",
		}
	}
	if userID == nil || *userID == "" {
		return "", &cibaError{
			Code:    oauth2const.ErrorUnknownUserID,
			Message: "Unable to resolve the user for the provided login_hint",
		}
	}
	return *userID, nil
}

// validateBackchannelAuthRequest validates the required parameters of a backchannel authentication request.
func validateBackchannelAuthRequest(request *BackchannelAuthRequest, scopes []string) *cibaError {
	if request.LoginHint == "" {
		return &cibaError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "login_hint is required",
		}
	}
	if len(scopes) == 0 {
		return &cibaError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "scope is required",
		}
	}
	if !slices.Contains(scopes, oauth2const.ScopeOpenID) {
		return &cibaError{
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
func validateBindingMessage(bindingMessage string) *cibaError {
	if bindingMessage == "" {
		return nil
	}
	if utf8.RuneCountInString(bindingMessage) > cibaMaxBindingMessageLength {
		return &cibaError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "binding_message exceeds the maximum allowed length",
		}
	}
	for _, r := range bindingMessage {
		if !unicode.IsPrint(r) {
			return &cibaError{
				Code:    oauth2const.ErrorInvalidRequest,
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

// buildNotificationURL constructs the MVP notification URL pointing at the gate login page.
func buildNotificationURL(executionID, authReqID, bindingMessage string) (string, error) {
	gateClientConfig := config.GetServerRuntime().Config.GateClient
	loginPageURL := (&url.URL{
		Scheme: gateClientConfig.Scheme,
		Host:   fmt.Sprintf("%s:%d", gateClientConfig.Hostname, gateClientConfig.Port),
		Path:   gateClientConfig.LoginPath,
	}).String()

	queryParams := map[string]string{
		"flowType":                        string(flowcm.FlowTypeAuthentication),
		oauth2const.ExecutionID:           executionID,
		oauth2const.RequestParamAuthReqID: authReqID,
	}
	if bindingMessage != "" {
		queryParams[oauth2const.RequestParamBindingMessage] = bindingMessage
	}

	return oauth2utils.GetURIWithQueryParams(loginPageURL, queryParams)
}
