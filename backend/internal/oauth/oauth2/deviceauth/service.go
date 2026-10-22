// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package deviceauth manages Device Authorization Grant requests from initiation through verification and completion.
package deviceauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"
	"unicode"

	flowcm "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/executormeta"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const userCodeAlphabet = "BCDFGHJKLMNPQRSTVWXZ"

// DeviceAuthServiceInterface defines the Device Authorization Grant service operations.
type DeviceAuthServiceInterface interface {
	IsDeviceRequest(ctx context.Context, requestID string) (bool, error)
	InitiateDeviceAuthorization(
		ctx context.Context, request *DeviceAuthorizationRequest, oauthApp *providers.OAuthClient,
	) (*DeviceAuthorizationResponse, error)
	HandleCallback(ctx context.Context, requestID, assertion string) *DeviceAuthError
	GetByDeviceCode(ctx context.Context, deviceCode string) (*DeviceAuthRequest, error)
	UpdatePolling(ctx context.Context, requestID string, polledAt time.Time, interval int64) error
	UpdateState(ctx context.Context, requestID string, state DeviceRequestState) error
	MarkConsumed(ctx context.Context, requestID string) (bool, error)
}

// deviceAuthService implements the DeviceAuthServiceInterface.
type deviceAuthService struct {
	store           DeviceAuthRequestStoreInterface
	actorProvider   providers.ActorProvider
	flowProvider    providers.FlowProvider
	resourceService providers.ResourceServerProvider
	jwtService      jwt.JWTServiceInterface
	cfg             oauthconfig.Config
	logger          *log.Logger
}

// newDeviceAuthService creates a Device Authorization Grant service with its dependencies.
func newDeviceAuthService(store DeviceAuthRequestStoreInterface, actorProvider providers.ActorProvider,
	flowProvider providers.FlowProvider, resourceService providers.ResourceServerProvider,
	jwtService jwt.JWTServiceInterface, cfg oauthconfig.Config) DeviceAuthServiceInterface {
	service := &deviceAuthService{
		store:           store,
		actorProvider:   actorProvider,
		flowProvider:    flowProvider,
		resourceService: resourceService,
		jwtService:      jwtService,
		cfg:             cfg,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DeviceAuthService")),
	}
	return service
}

// UserCodeVerifier verifies browser-entered codes against Device Grant requests.
type UserCodeVerifier struct {
	store DeviceAuthRequestStoreInterface
}

// NewUserCodeVerifier constructs the verifier used by the Device Grant system flow.
func NewUserCodeVerifier(runtimeStore providers.RuntimeStoreProvider) *UserCodeVerifier {
	return &UserCodeVerifier{store: newDeviceAuthStore(runtimeStore)}
}

// VerifyUserCode claims a pending request and returns its trusted application and request IDs.
func (v *UserCodeVerifier) VerifyUserCode(ctx context.Context, userCode string) (*DeviceVerificationResponse, error) {
	return verifyUserCode(ctx, v.store, userCode)
}

// IsDeviceRequest reports whether requestID belongs to a Device Grant request.
func (s *deviceAuthService) IsDeviceRequest(ctx context.Context, requestID string) (bool, error) {
	_, err := s.store.GetByRequestID(ctx, requestID)
	if errors.Is(err, ErrDeviceAuthRequestNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get device authorization request by request ID: %w", err)
	}
	return true, nil
}

// InitiateDeviceAuthorization creates and stores a new device authorization request.
func (s *deviceAuthService) InitiateDeviceAuthorization(
	ctx context.Context,
	request *DeviceAuthorizationRequest,
	oauthApp *providers.OAuthClient,
) (*DeviceAuthorizationResponse, error) {
	if !oauthApp.IsAllowedGrantType(providers.GrantTypeDeviceCode) {
		return nil, &DeviceAuthError{
			Code:    oauth2const.ErrorUnauthorizedClient,
			Message: "unauthorized client",
		}
	}

	oidcScopes, permissionScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(
		request.Scope, oauthApp.ScopeClaims)
	targetRS, resourceErr := resourceindicators.ResolveAudienceBinding(
		ctx, s.resourceService, request.Resources, permissionScopes)
	if resourceErr != nil {
		return nil, &DeviceAuthError{Code: resourceErr.Error, Message: resourceErr.ErrorDescription}
	}

	var effectiveResources []string
	if targetRS != nil {
		permissionScopes, resourceErr = resourceindicators.DownscopeToResourceServer(
			ctx, s.resourceService, targetRS.ID, permissionScopes)
		if resourceErr != nil {
			return nil, &DeviceAuthError{Code: resourceErr.Error, Message: resourceErr.ErrorDescription}
		}
		effectiveResources = []string{targetRS.Identifier}
	}

	authFlowID, err := s.resolveApplicationAuthenticationFlow(ctx, oauthApp.ID)
	if err != nil {
		return nil, err
	}
	requestedAttributes := requiredOptionalAttributes(append(oidcScopes, permissionScopes...), oauthApp)
	if requestedAttributes == "" && len(permissionScopes) == 0 {
		return nil, &DeviceAuthError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Device authorization requires a consent purpose for the requested attributes or permissions",
		}
	}

	expiresIn := s.cfg.OAuth.Device.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = oauth2const.DeviceAuthorizationDefaultExpiresInSeconds
	}
	interval := s.cfg.OAuth.Device.Interval
	if interval <= 0 {
		interval = oauth2const.DeviceAuthorizationDefaultIntervalSeconds
	}
	requestID, err := utils.GenerateUUIDv7()
	if err != nil {
		return nil, fmt.Errorf("generate device authorization request ID: %w", err)
	}
	deviceCode, err := randomDeviceCode()
	if err != nil {
		return nil, err
	}
	var record *DeviceAuthRequest
	for attempts := 0; attempts < 5; attempts++ {
		userCode, err := randomUserCode()
		if err != nil {
			return nil, err
		}
		record = &DeviceAuthRequest{
			RequestID:                  requestID,
			ApplicationID:              oauthApp.ID,
			ApplicationAuthFlowID:      authFlowID,
			DeviceCodeHash:             hashDeviceCode(deviceCode),
			UserCode:                   userCode,
			ClientID:                   oauthApp.ClientID,
			StandardScopes:             utils.StringifyStringArray(oidcScopes, " "),
			RequestedPermissions:       utils.StringifyStringArray(permissionScopes, " "),
			RequiredOptionalAttributes: requestedAttributes,
			Resources:                  effectiveResources,
			State:                      DeviceStatePending,
			PollInterval:               interval,
			ExpiryTime:                 time.Now().Add(time.Duration(expiresIn) * time.Second),
		}
		if err = s.store.Add(ctx, record); err == nil {
			break
		}
		if !errors.Is(err, ErrDeviceAuthUserCodeCollision) {
			return nil, err
		}
		record = nil
	}
	if record == nil {
		return nil, fmt.Errorf("generate unique user code")
	}
	verificationURI := s.verificationURI(record.ApplicationID)
	verificationURIComplete, err := url.Parse(verificationURI)
	if err != nil {
		return nil, fmt.Errorf("parse device verification URI: %w", err)
	}
	query := verificationURIComplete.Query()
	query.Set("user_code", record.UserCode)
	verificationURIComplete.RawQuery = query.Encode()
	return &DeviceAuthorizationResponse{DeviceCode: deviceCode, UserCode: record.UserCode,
		VerificationURI: verificationURI, VerificationURIComplete: verificationURIComplete.String(),
		ExpiresIn: expiresIn, Interval: interval}, nil
}

// resolveApplicationAuthenticationFlow returns the authentication flow configured for an application.
func (s *deviceAuthService) resolveApplicationAuthenticationFlow(
	ctx context.Context, applicationID string,
) (string, error) {
	if s.actorProvider == nil || s.flowProvider == nil {
		return "", fmt.Errorf("application authentication flow providers are not configured")
	}
	client, svcErr := s.actorProvider.GetInboundClientByID(ctx, applicationID)
	if svcErr != nil || client == nil || client.AuthFlowID == "" {
		return "", &DeviceAuthError{Code: oauth2const.ErrorServerError,
			Message: "Failed to resolve the application's authentication flow"}
	}
	flow, svcErr := s.flowProvider.GetFlow(ctx, client.AuthFlowID)
	if svcErr != nil || flow == nil || flow.FlowType != providers.FlowTypeAuthentication {
		return "", &DeviceAuthError{Code: oauth2const.ErrorServerError,
			Message: "The application's authentication flow is invalid"}
	}
	for _, node := range flow.Nodes {
		if node.Type == string(flowcm.NodeTypeTaskExecution) && node.Executor != nil &&
			node.Executor.Name == executormeta.ExecutorNameConsent {
			return flow.ID, nil
		}
	}
	return "", &DeviceAuthError{Code: oauth2const.ErrorInvalidRequest,
		Message: "The application's authentication flow must include User Consent for device authorization"}
}

func verifyUserCode(ctx context.Context, store DeviceAuthRequestStoreInterface,
	userCode string) (*DeviceVerificationResponse, error) {
	record, err := store.GetByUserCode(ctx, userCode)
	if err != nil {
		if errors.Is(err, ErrDeviceAuthRequestNotFound) {
			return nil, unavailableUserCodeError()
		}
		return nil, fmt.Errorf("get device authorization request by user code: %w", err)
	}
	if time.Now().After(record.ExpiryTime) || record.State != DeviceStatePending {
		return nil, unavailableUserCodeError()
	}

	claimed, err := store.MarkRedeeming(ctx, record.RequestID)
	if err != nil {
		return nil, fmt.Errorf("claim device authorization request: %w", err)
	}
	if !claimed {
		return nil, unavailableUserCodeError()
	}

	resourceServerIdentifier := ""
	if len(record.Resources) != 0 {
		resourceServerIdentifier = record.Resources[0]
	}
	return &DeviceVerificationResponse{
		ApplicationID:              record.ApplicationID,
		ApplicationAuthFlowID:      record.ApplicationAuthFlowID,
		AuthID:                     record.RequestID,
		RequestedPermissions:       record.RequestedPermissions,
		ResourceServerIdentifier:   resourceServerIdentifier,
		RequiredOptionalAttributes: record.RequiredOptionalAttributes,
	}, nil
}

// unavailableUserCodeError returns the generic response for an unusable user code.
func unavailableUserCodeError() *DeviceAuthError {
	return &DeviceAuthError{
		Code:    oauth2const.ErrorInvalidRequest,
		Message: "The user code is invalid or unavailable",
	}
}

// requiredOptionalAttributes returns the user attributes needed for the requested scopes and tokens.
func requiredOptionalAttributes(scopes []string, app *providers.OAuthClient) string {
	attributes := make(map[string]bool)
	if app.Token != nil && app.Token.AccessToken != nil && app.Token.AccessToken.UserConfig != nil {
		for _, attribute := range app.Token.AccessToken.UserConfig.Attributes {
			attributes[attribute] = true
		}
	}
	if slices.Contains(scopes, oauth2const.ScopeOpenID) {
		allowed := make(map[string]bool)
		if app.Token != nil && app.Token.IDToken != nil {
			for _, attribute := range app.Token.IDToken.UserAttributes {
				allowed[attribute] = true
			}
		}
		if app.UserInfo != nil {
			for _, attribute := range app.UserInfo.UserAttributes {
				allowed[attribute] = true
			}
		}
		for _, scope := range scopes {
			for _, attribute := range app.ScopeClaims[scope] {
				if allowed[attribute] {
					attributes[attribute] = true
				}
			}
		}
	}
	selected := make([]string, 0, len(attributes))
	for attribute := range attributes {
		selected = append(selected, attribute)
	}
	slices.Sort(selected)
	return strings.Join(selected, " ")
}

// HandleCallback verifies a terminal flow assertion and applies its outcome to the Device Grant request.
func (s *deviceAuthService) HandleCallback(ctx context.Context, requestID, assertion string) *DeviceAuthError {
	record, callbackErr := s.loadRequestForCallback(ctx, requestID, assertion)
	if callbackErr != nil {
		return callbackErr
	}

	claims, authTime, err := decodeDeviceAssertion(assertion)
	if err != nil {
		s.logger.Error(ctx, "Failed to decode Device Grant assertion", log.Error(err))
		return deviceCallbackServerError()
	}
	if claims.requestID == "" || claims.requestID != record.RequestID {
		return &DeviceAuthError{Code: oauth2const.ErrorAccessDenied,
			Message: "Assertion does not match the device authorization request"}
	}

	if claims.flowErrorType != "" {
		state := DeviceStateFailed
		if claims.flowErrorType == flowcm.FlowErrorTypeEndUser {
			state = DeviceStateDenied
		}
		if err := s.store.MarkFailed(ctx, record.RequestID, state); err != nil {
			s.logger.Error(ctx, "Failed to complete Device Grant request", log.Error(err))
			return deviceCallbackServerError()
		}
		return nil
	}

	if claims.userID == "" {
		return &DeviceAuthError{Code: oauth2const.ErrorAccessDenied, Message: "Assertion subject is missing"}
	}
	if claims.consentID == "" {
		if err := s.store.MarkFailed(ctx, record.RequestID, DeviceStateFailed); err != nil {
			s.logger.Error(ctx, "Failed to reject Device Grant without recorded consent", log.Error(err))
			return deviceCallbackServerError()
		}
		return &DeviceAuthError{Code: oauth2const.ErrorAccessDenied,
			Message: "Explicit user consent is required for device authorization"}
	}
	if !claims.consentApproved {
		if err := s.store.MarkFailed(ctx, record.RequestID, DeviceStateDenied); err != nil {
			s.logger.Error(ctx, "Failed to record Device Grant denial", log.Error(err))
			return deviceCallbackServerError()
		}
		return &DeviceAuthError{Code: oauth2const.ErrorAccessDenied, Message: "Device authorization was denied"}
	}
	if authTime.IsZero() {
		authTime = time.Now()
	}
	authorizedScopes := utils.StringifyStringArray(append(
		utils.ParseStringArray(record.StandardScopes, " "),
		utils.ParseStringArray(claims.authorizedPermissions, " ")...), " ")
	if err := s.store.MarkApproved(ctx, record.RequestID, claims.userID, authorizedScopes,
		claims.attributeCacheID, claims.completedACR, claims.sessionID, authTime); err != nil {
		s.logger.Error(ctx, "Failed to approve Device Grant request", log.Error(err))
		return deviceCallbackServerError()
	}
	return nil
}

// loadRequestForCallback loads and validates the Device Grant request bound to a flow assertion.
func (s *deviceAuthService) loadRequestForCallback(
	ctx context.Context, requestID, assertion string,
) (*DeviceAuthRequest, *DeviceAuthError) {
	if requestID == "" || assertion == "" {
		return nil, &DeviceAuthError{Code: oauth2const.ErrorInvalidRequest,
			Message: "authId and assertion are required"}
	}
	record, err := s.store.GetByRequestID(ctx, requestID)
	if err != nil {
		if errors.Is(err, ErrDeviceAuthRequestNotFound) {
			return nil, &DeviceAuthError{Code: oauth2const.ErrorInvalidRequest,
				Message: "Invalid device authorization request"}
		}
		s.logger.Error(ctx, "Failed to retrieve Device Grant request", log.Error(err))
		return nil, deviceCallbackServerError()
	}
	if record.ExpiryTime.Before(time.Now()) {
		return nil, &DeviceAuthError{Code: oauth2const.ErrorExpiredToken,
			Message: "Device authorization request has expired"}
	}
	if record.State != DeviceStateRedeeming {
		return nil, &DeviceAuthError{Code: oauth2const.ErrorInvalidRequest,
			Message: "Device authorization request is not redeeming"}
	}
	if s.jwtService == nil {
		return nil, deviceCallbackServerError()
	}
	if verifyErr := s.jwtService.VerifyJWT(ctx, assertion, record.ApplicationID, ""); verifyErr != nil {
		return nil, &DeviceAuthError{Code: oauth2const.ErrorInvalidRequest,
			Message: "Invalid assertion signature"}
	}
	return record, nil
}

// decodeDeviceAssertion extracts Device Grant callback values from a flow assertion.
func decodeDeviceAssertion(assertion string) (assertionClaims, time.Time, error) {
	base, payload, err := oauth2utils.DecodeFlowAssertionClaims(assertion)
	if err != nil {
		return assertionClaims{}, time.Time{}, err
	}
	claims := assertionClaims{userID: base.UserID, attributeCacheID: base.AttributeCacheID,
		completedACR: base.CompletedACR}
	if value, ok := payload[oauth2const.ClaimAuthorizationRequestID].(string); ok {
		claims.requestID = value
	}
	if value, ok := payload["authorized_permissions"].(string); ok {
		claims.authorizedPermissions = value
	}
	if value, ok := payload[oauth2const.ClaimSessionID].(string); ok {
		claims.sessionID = value
	}
	if value, ok := payload[oauth2const.ClaimConsentID].(string); ok {
		claims.consentID = value
	}
	if value, ok := payload[oauth2const.ClaimConsentApproved].(bool); ok {
		claims.consentApproved = value
	}
	if value, ok := payload[flowcm.ClaimFlowErrorType].(string); ok {
		claims.flowErrorType = value
	}
	return claims, base.AuthTime, nil
}

// deviceCallbackServerError returns the generic callback processing error.
func deviceCallbackServerError() *DeviceAuthError {
	return &DeviceAuthError{Code: oauth2const.ErrorServerError,
		Message: "Failed to process device authorization callback"}
}

// GetByDeviceCode retrieves a Device Grant request for token polling.
func (s *deviceAuthService) GetByDeviceCode(ctx context.Context, deviceCode string) (*DeviceAuthRequest, error) {
	return s.store.GetByDeviceCode(ctx, deviceCode)
}

// UpdatePolling records the latest poll time and current minimum interval.
func (s *deviceAuthService) UpdatePolling(
	ctx context.Context, requestID string, polledAt time.Time, interval int64,
) error {
	return s.store.UpdatePolling(ctx, requestID, polledAt, interval)
}

// UpdateState updates the state of a Device Grant request.
func (s *deviceAuthService) UpdateState(
	ctx context.Context, requestID string, state DeviceRequestState,
) error {
	return s.store.UpdateState(ctx, requestID, state)
}

// MarkConsumed atomically transitions an approved Device Grant request to consumed.
func (s *deviceAuthService) MarkConsumed(ctx context.Context, requestID string) (bool, error) {
	return s.store.MarkConsumed(ctx, requestID)
}

// verificationURI builds the browser verification URI for an application.
func (s *deviceAuthService) verificationURI(applicationID string) string {
	u := url.URL{
		Scheme: s.cfg.GateClient.Scheme,
		Host:   fmt.Sprintf("%s:%d", s.cfg.GateClient.Hostname, s.cfg.GateClient.Port),
		Path:   path.Join(s.cfg.GateClient.Path, "device"),
	}
	query := u.Query()
	query.Set("applicationId", applicationID)
	u.RawQuery = query.Encode()
	return u.String()
}

// randomDeviceCode generates a high-entropy device code.
func randomDeviceCode() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate device code: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// randomUserCode generates a short code for user verification.
func randomUserCode() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate user code: %w", err)
	}
	var builder strings.Builder
	for i, value := range bytes {
		if i == 4 {
			builder.WriteByte('-')
		}
		builder.WriteByte(userCodeAlphabet[int(value)%len(userCodeAlphabet)])
	}
	return builder.String(), nil
}

// normalizeUserCode removes separators and converts a user code to uppercase.
func normalizeUserCode(userCode string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToUpper(r)
	}, userCode)
}
