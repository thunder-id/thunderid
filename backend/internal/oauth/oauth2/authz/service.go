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

// Package authz implements the OAuth2 authorization functionality.
package authz

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	flowcm "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/authz/requestvalidator"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// AuthorizeServiceInterface defines the interface for authorization services.
type AuthorizeServiceInterface interface {
	GetAuthorizationCodeDetails(ctx context.Context, clientID string, code string) (*AuthorizationCode, error)
	HandleInitialAuthorizationRequest(
		ctx context.Context, msg *OAuthMessage,
	) (*AuthorizationInitResult, *AuthorizationError)
	HandleAuthorizationCallback(ctx context.Context, authID string, assertion string) (string, *AuthorizationError)
}

// authorizeService implements the AuthorizeService for managing OAuth2 authorization flows.
type authorizeService struct {
	inboundClient   inboundclient.InboundClientServiceInterface
	resourceService resource.ResourceServiceInterface
	authZValidator  AuthorizationValidatorInterface
	authCodeStore   AuthorizationCodeStoreInterface
	authReqStore    authorizationRequestStoreInterface
	parService      par.PARServiceInterface
	jwtService      jwt.JWTServiceInterface
	flowExecService flowexec.FlowExecServiceInterface
	transactioner   transaction.Transactioner
	logger          *log.Logger
}

// newAuthorizeService creates a new instance of authorizeService with injected dependencies.
func newAuthorizeService(
	inboundClient inboundclient.InboundClientServiceInterface,
	resourceService resource.ResourceServiceInterface,
	jwtService jwt.JWTServiceInterface,
	flowExecService flowexec.FlowExecServiceInterface,
	authCodeStore AuthorizationCodeStoreInterface,
	authReqStore authorizationRequestStoreInterface,
	parService par.PARServiceInterface,
	transactioner transaction.Transactioner,
) AuthorizeServiceInterface {
	return &authorizeService{
		inboundClient:   inboundClient,
		resourceService: resourceService,
		authZValidator:  newAuthorizationValidator(),
		authCodeStore:   authCodeStore,
		authReqStore:    authReqStore,
		parService:      parService,
		jwtService:      jwtService,
		flowExecService: flowExecService,
		transactioner:   transactioner,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthorizeService")),
	}
}

// GetAuthorizationCodeDetails retrieves and consumes the authorization code.
func (as *authorizeService) GetAuthorizationCodeDetails(
	ctx context.Context, clientID string, code string,
) (*AuthorizationCode, error) {
	var record *AuthorizationCode
	err := as.transactioner.Transact(ctx, func(ctx context.Context) error {
		var err error
		record, err = as.authCodeStore.GetAuthorizationCode(ctx, code)
		if err != nil {
			return err
		}

		if record.ClientID != clientID {
			return errors.New("client ID mismatch for authorization code")
		}

		consumed, err := as.authCodeStore.ConsumeAuthorizationCode(ctx, code)
		if err != nil {
			return err
		}
		if !consumed {
			// TODO: Revoke all access tokens already granted for this authorization code
			// when the code has already been consumed (replay attack detected).
			return errAuthorizationCodeAlreadyConsumed
		}
		return nil
	})
	if err != nil {
		as.logger.Error("Failed to get authorization code details", log.Error(err))
		return nil, err
	}
	return record, nil
}

// HandleInitialAuthorizationRequest processes an initial authorization request from the client.
// Returns the query params needed to redirect to the login page, or a structured authorization error.
func (as *authorizeService) HandleInitialAuthorizationRequest(ctx context.Context, msg *OAuthMessage) (
	*AuthorizationInitResult, *AuthorizationError) {
	clientID := msg.RequestQueryParams[oauth2const.RequestParamClientID]
	requestURI := msg.RequestQueryParams[oauth2const.RequestParamRequestURI]

	if clientID == "" {
		return nil, &AuthorizationError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Missing client_id parameter",
		}
	}

	// Retrieve the OAuth client based on the client ID.
	app, lookupErr := as.inboundClient.GetOAuthClientByClientID(ctx, clientID)
	if lookupErr != nil {
		as.logger.Error("Failed to retrieve OAuth client", log.Error(lookupErr))
		return nil, &AuthorizationError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process authorization request",
		}
	}
	if app == nil {
		return nil, &AuthorizationError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Invalid client_id",
		}
	}

	// If request_uri is present, resolve the pushed authorization request.
	if requestURI != "" {
		return as.handlePARAuthorizationRequest(ctx, requestURI, clientID, app)
	}

	// Enforce PAR requirement: if PAR is required (per-client or global), reject requests without request_uri.
	if app.RequiresPAR() {
		return nil, &AuthorizationError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Pushed authorization request is required for this client",
		}
	}

	return as.handleStandardAuthorizationRequest(ctx, msg, app)
}

// handlePARAuthorizationRequest resolves a request_uri from a PAR and continues the authorization flow.
func (as *authorizeService) handlePARAuthorizationRequest(
	ctx context.Context, requestURI string, clientID string, app *inboundmodel.OAuthClient,
) (*AuthorizationInitResult, *AuthorizationError) {
	oauthParams, err := as.parService.ResolvePushedAuthorizationRequest(ctx, requestURI, clientID)
	if err != nil {
		as.logger.Debug("Failed to resolve PAR request", log.Error(err))
		if errors.Is(err, par.ErrPARResolutionFailed) {
			return nil, &AuthorizationError{
				Code:    oauth2const.ErrorServerError,
				Message: "Failed to process authorization request",
			}
		}
		return nil, &AuthorizationError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Invalid, expired, or already used request_uri",
		}
	}

	return as.initiateFlowAndStoreRequest(ctx, oauthParams, app)
}

// handleStandardAuthorizationRequest processes a standard authorization request (without PAR).
func (as *authorizeService) handleStandardAuthorizationRequest(
	ctx context.Context, msg *OAuthMessage, app *inboundmodel.OAuthClient,
) (*AuthorizationInitResult, *AuthorizationError) {
	// Extract required parameters.
	redirectURI := msg.RequestQueryParams[oauth2const.RequestParamRedirectURI]
	scope := msg.RequestQueryParams[oauth2const.RequestParamScope]
	state := msg.RequestQueryParams[oauth2const.RequestParamState]
	responseType := msg.RequestQueryParams[oauth2const.RequestParamResponseType]

	// Extract PKCE parameters.
	codeChallenge := msg.RequestQueryParams[oauth2const.RequestParamCodeChallenge]
	codeChallengeMethod := msg.RequestQueryParams[oauth2const.RequestParamCodeChallengeMethod]

	resources := msg.Resources

	// Extract claims parameter.
	claimsParam := msg.RequestQueryParams[oauth2const.RequestParamClaims]

	// Extract claims_locales parameter.
	claimsLocales := msg.RequestQueryParams[oauth2const.RequestParamClaimsLocales]

	nonce := msg.RequestQueryParams[oauth2const.RequestParamNonce]
	acrValues := msg.RequestQueryParams[oauth2const.RequestParamAcrValues]

	// Parse the claims parameter if present.
	var claimsRequest *oauth2model.ClaimsRequest
	if claimsParam != "" {
		var err error
		claimsRequest, err = oauth2utils.ParseClaimsRequest(claimsParam)
		if err != nil {
			as.logger.Debug("Failed to parse claims parameter", log.Error(err))
			return nil, &AuthorizationError{
				Code:    oauth2const.ErrorInvalidRequest,
				Message: "The claims request parameter is malformed or contains invalid values",
			}
		}
	}

	// Validate the authorization request.
	sendErrorToApp, errorCode, errorMessage := as.authZValidator.validateInitialAuthorizationRequest(msg, app)
	if errorCode != "" {
		authErr := &AuthorizationError{
			Code:    errorCode,
			Message: errorMessage,
			State:   state,
		}
		if sendErrorToApp && redirectURI != "" {
			authErr.SendErrorToClient = true
			authErr.ClientRedirectURI = redirectURI
		}
		return nil, authErr
	}

	oidcScopes, nonOidcScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(scope, app.ScopeClaims)

	// Resolve resource identifiers to Resource Servers and downscope non-OIDC scopes against
	// the union of permissions defined on those Resource Servers. Unknown identifiers cause
	// invalid_target; scopes not defined on any RS are silently dropped.
	_, nonOidcScopes, errResp := resourceindicators.ResolveAndDownscope(
		ctx, as.resourceService, resources, nonOidcScopes)
	if errResp != nil {
		return nil, &AuthorizationError{
			Code:              errResp.Error,
			Message:           errResp.ErrorDescription,
			SendErrorToClient: true,
			ClientRedirectURI: redirectURI,
			State:             state,
		}
	}

	// Construct authorization request context.
	oauthParams := &oauth2model.OAuthParameters{
		State:               state,
		ClientID:            app.ClientID,
		RedirectURI:         redirectURI,
		ResponseType:        responseType,
		StandardScopes:      oidcScopes,
		PermissionScopes:    nonOidcScopes,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		Resources:           resources,
		ClaimsRequest:       claimsRequest,
		ClaimsLocales:       claimsLocales,
		Nonce:               nonce,
		AcrValues:           acrValues,
	}

	// Set the redirect URI if not provided in the request. Invalid cases are already handled at this point.
	// TODO: This should be removed when supporting other means of authorization.
	if redirectURI == "" {
		if len(app.RedirectURIs) == 0 {
			as.logger.Error("OAuth application has no registered redirect URIs",
				log.String("client_id", app.ClientID))
			return nil, &AuthorizationError{
				Code:    oauth2const.ErrorServerError,
				Message: "Failed to process authorization request",
			}
		}
		oauthParams.RedirectURI = app.RedirectURIs[0]
	}

	return as.initiateFlowAndStoreRequest(ctx, oauthParams, app)
}

// initiateFlowAndStoreRequest initiates the authentication flow and stores the authorization request context.
// This is the common path shared by both standard and PAR-based authorization requests.
func (as *authorizeService) initiateFlowAndStoreRequest(
	ctx context.Context, oauthParams *oauth2model.OAuthParameters, app *inboundmodel.OAuthClient,
) (*AuthorizationInitResult, *AuthorizationError) {
	effectiveAcrValues := requestvalidator.ResolveACRValues(oauthParams.AcrValues, app.AcrValues)
	essentialAttributes, optionalAttributes := getRequiredAttributes(
		oauthParams.StandardScopes, oauthParams.ClaimsRequest, oauthParams.ResponseType, app)

	// Initiate flow with OAuth context.
	runtimeData := map[string]string{
		flowcm.RuntimeKeyClientID:                      oauthParams.ClientID,
		flowcm.RuntimeKeyRequestedPermissions:          utils.StringifyStringArray(oauthParams.PermissionScopes, " "),
		flowcm.RuntimeKeyRequiredEssentialAttributes:   essentialAttributes,
		flowcm.RuntimeKeyRequiredOptionalAttributes:    optionalAttributes,
		flowcm.RuntimeKeyRequiredLocales:               oauthParams.ClaimsLocales,
		flowcm.RuntimeKeyUserAttributesCacheTTLSeconds: fmt.Sprintf("%d", resolveUserAttributesCacheTTL(app)),
	}
	if effectiveAcrValues != "" {
		runtimeData[flowcm.RuntimeKeyRequestedAuthClasses] = effectiveAcrValues
	}
	flowInitCtx := &flowexec.FlowInitContext{
		ApplicationID: app.ID,
		FlowType:      string(flowcm.FlowTypeAuthentication),
		RuntimeData:   runtimeData,
	}

	executionID, flowErr := as.flowExecService.InitiateFlow(ctx, flowInitCtx)
	if flowErr != nil {
		as.logger.Error("Failed to initiate authentication flow",
			log.String("error_code", flowErr.Code))
		return nil, &AuthorizationError{
			Code:              oauth2const.ErrorServerError,
			Message:           "Failed to process authorization request",
			SendErrorToClient: true,
			ClientRedirectURI: oauthParams.RedirectURI,
			State:             oauthParams.State,
		}
	}

	authRequestCtx := authRequestContext{
		OAuthParameters: *oauthParams,
	}

	// Store authorization request context in the store.
	identifier, storeErr := as.authReqStore.AddRequest(ctx, authRequestCtx)
	if storeErr != nil {
		as.logger.Error("Failed to store authorization request context", log.Error(storeErr))
		return nil, &AuthorizationError{
			Code:              oauth2const.ErrorServerError,
			Message:           "Failed to process authorization request",
			SendErrorToClient: true,
			ClientRedirectURI: oauthParams.RedirectURI,
			State:             oauthParams.State,
		}
	}

	// Build query parameters for login page redirect.
	queryParams := make(map[string]string)
	queryParams[oauth2const.AuthID] = identifier
	queryParams[oauth2const.AppID] = app.ID
	queryParams[oauth2const.ExecutionID] = executionID

	// Add insecure warning if the redirect URI is not using TLS.
	// TODO: May require another redirection to a warn consent page when it directly goes to a federated IDP.
	parsedRedirectURI, err := utils.ParseURL(oauthParams.RedirectURI)
	if err != nil {
		as.logger.Error("Failed to parse redirect URI", log.Error(err))
		return nil, &AuthorizationError{
			Code:              oauth2const.ErrorServerError,
			Message:           "Failed to process authorization request",
			SendErrorToClient: true,
			ClientRedirectURI: oauthParams.RedirectURI,
			State:             oauthParams.State,
		}
	}
	if parsedRedirectURI.Scheme == "http" {
		queryParams[oauth2const.ShowInsecureWarning] = "true"
	}

	return &AuthorizationInitResult{QueryParams: queryParams}, nil
}

// HandleAuthorizationCallback processes the callback assertion from the flow engine.
// Returns the client redirect URI (with authorization code) on success, or a structured error.
func (as *authorizeService) HandleAuthorizationCallback(ctx context.Context, authID string, assertion string) (
	string, *AuthorizationError) {
	var redirectURI string
	var authErr *AuthorizationError

	err := func() error {
		// Load the authorization request context.
		authRequestCtx, err := as.loadAuthRequestContext(ctx, authID)
		if err != nil {
			if errors.Is(err, errAuthRequestNotFound) {
				authErr = &AuthorizationError{
					Code:    oauth2const.ErrorInvalidRequest,
					Message: "Invalid authorization request",
				}
				return err
			}

			authErr = &AuthorizationError{
				Code:    oauth2const.ErrorServerError,
				Message: "Failed to process authorization request",
			}
			return err
		}

		if assertion == "" {
			authErr = &AuthorizationError{
				Code:              oauth2const.ErrorInvalidRequest,
				Message:           "Invalid authorization request",
				SendErrorToClient: true,
				ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
				State:             authRequestCtx.OAuthParameters.State,
			}
			return errors.New("assertion is empty")
		}

		// Verify the assertion.
		if err := as.verifyAssertion(assertion); err != nil {
			as.logger.Debug("Assertion verification failed", log.Error(err))
			authErr = &AuthorizationError{
				Code:              oauth2const.ErrorInvalidRequest,
				Message:           "Authorization request failed",
				SendErrorToClient: true,
				ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
				State:             authRequestCtx.OAuthParameters.State,
			}
			return err
		}

		// Decode user attributes from the assertion.
		claims, authTime, err := decodeAttributesFromAssertion(assertion)
		if err != nil {
			authErr = &AuthorizationError{
				Code:              oauth2const.ErrorServerError,
				Message:           "Failed to process authorization request",
				SendErrorToClient: true,
				ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
				State:             authRequestCtx.OAuthParameters.State,
			}
			return err
		}

		if claims.userID == "" {
			authErr = &AuthorizationError{
				Code:              oauth2const.ErrorServerError,
				Message:           "Authorization request failed",
				SendErrorToClient: true,
				ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
				State:             authRequestCtx.OAuthParameters.State,
			}
			return errors.New("user ID is empty")
		}

		// Validate sub claim constraint if specified in claims parameter.
		// If sub claim is requested with a value constraint and doesn't match, authentication must fail.
		hasOpenIDScope := slices.Contains(authRequestCtx.OAuthParameters.StandardScopes, oauth2const.ScopeOpenID)
		if hasOpenIDScope {
			if err := validateSubClaimConstraint(
				authRequestCtx.OAuthParameters.ClaimsRequest, claims.userID,
			); err != nil {
				as.logger.Debug("Sub claim validation failed", log.Error(err))
				authErr = &AuthorizationError{
					Code:              oauth2const.ErrorAccessDenied,
					Message:           "Authorization request failed",
					SendErrorToClient: true,
					ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
					State:             authRequestCtx.OAuthParameters.State,
				}
				return err
			}
		}

		// Extract authorized permissions for permission scopes.
		// Overwrite the non-OIDC scopes in auth request context with the authorized scopes from the assertion.
		if claims.authorizedPermissions != "" {
			authRequestCtx.OAuthParameters.PermissionScopes = utils.ParseStringArray(
				claims.authorizedPermissions, " ")
		} else {
			// Clear permission scopes if no authorized permissions in assertion.
			authRequestCtx.OAuthParameters.PermissionScopes = []string{}
		}

		// Generate the authorization code.
		authzCode, err := createAuthorizationCode(authRequestCtx, &claims, authTime)
		if err != nil {
			authErr = &AuthorizationError{
				Code:              oauth2const.ErrorServerError,
				Message:           "Failed to process authorization request",
				SendErrorToClient: true,
				ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
				State:             authRequestCtx.OAuthParameters.State,
			}
			return err
		}

		// Persist the authorization code.
		if persistErr := as.authCodeStore.InsertAuthorizationCode(ctx, authzCode); persistErr != nil {
			authErr = &AuthorizationError{
				Code:              oauth2const.ErrorServerError,
				Message:           "Failed to process authorization request",
				SendErrorToClient: true,
				ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
				State:             authRequestCtx.OAuthParameters.State,
			}
			return persistErr
		}

		// Construct the redirect URI with the authorization code.
		queryParams := map[string]string{
			"code":                      authzCode.Code,
			oauth2const.RequestParamIss: config.GetServerRuntime().Config.JWT.Issuer,
		}
		if authRequestCtx.OAuthParameters.State != "" {
			queryParams[oauth2const.RequestParamState] = authRequestCtx.OAuthParameters.State
		}
		redirectURI, err = oauth2utils.GetURIWithQueryParams(authzCode.RedirectURI, queryParams)
		if err != nil {
			authErr = &AuthorizationError{
				Code:              oauth2const.ErrorServerError,
				Message:           "Failed to process authorization request",
				SendErrorToClient: true,
				ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
				State:             authRequestCtx.OAuthParameters.State,
			}
			return err
		}

		return nil
	}()

	if authErr != nil {
		if authErr.Code == oauth2const.ErrorServerError {
			as.logger.Error("Failed to process authorization callback", log.Error(err))
		}
		return "", authErr
	}
	if err != nil {
		as.logger.Error("Failed to process authorization callback", log.Error(err))
		return "", &AuthorizationError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process authorization request",
		}
	}

	return redirectURI, nil
}

// loadAuthRequestContext loads the authorization request context from the store using the auth ID.
func (as *authorizeService) loadAuthRequestContext(ctx context.Context, authID string) (*authRequestContext, error) {
	ok, authRequestCtx, err := as.authReqStore.GetRequest(ctx, authID)
	if err != nil {
		as.logger.Error("Failed to retrieve authorization request context", log.Error(err))
		return nil, errors.New("failed to retrieve authorization request context")
	}
	if !ok {
		as.logger.Debug("Authorization request context not found", log.String("auth_id", authID))
		return nil, errAuthRequestNotFound
	}

	// Remove the authorization request context after retrieval.
	if clearErr := as.authReqStore.ClearRequest(ctx, authID); clearErr != nil {
		as.logger.Error("Failed to clear authorization request context", log.Error(clearErr))
	}
	return &authRequestCtx, nil
}

// verifyAssertion verifies the JWT assertion.
func (as *authorizeService) verifyAssertion(assertion string) error {
	if err := as.jwtService.VerifyJWT(assertion, "", ""); err != nil {
		as.logger.Debug("Invalid assertion signature", log.String("error", err.Error.DefaultValue))
		return errors.New("invalid assertion signature")
	}
	return nil
}

// decodeAttributesFromAssertion decodes user attributes from the flow assertion JWT.
func decodeAttributesFromAssertion(assertion string) (assertionClaims, time.Time, error) {
	claims := assertionClaims{}

	_, jwtPayload, err := jwt.DecodeJWT(assertion)
	if err != nil {
		return claims, time.Time{}, fmt.Errorf("failed to decode the JWT token: %w", err)
	}

	// Extract authentication time from iat claim.
	authTime := time.Time{}
	if iatValue, ok := jwtPayload["iat"]; ok {
		switch v := iatValue.(type) {
		case float64:
			authTime = time.Unix(int64(v), 0)
		case int64:
			authTime = time.Unix(v, 0)
		case int:
			authTime = time.Unix(int64(v), 0)
		default:
			return claims, time.Time{}, errors.New("JWT 'iat' claim has unexpected type")
		}
	}

	for key, value := range jwtPayload {
		// Extract sub claim.
		if key == oauth2const.ClaimSub {
			if strValue, ok := value.(string); ok {
				claims.userID = strValue
			} else {
				return claims, time.Time{}, errors.New("JWT 'sub' claim is not a string")
			}
			continue
		}

		// Extract authorized_permissions claim.
		if key == "authorized_permissions" {
			if strValue, ok := value.(string); ok {
				claims.authorizedPermissions = strValue
			}
			continue
		}

		if key == "aci" {
			strValue, ok := value.(string)
			if !ok {
				return claims, time.Time{}, errors.New("JWT 'aci' claim is not a string")
			}
			claims.attributeCacheID = strValue
			continue
		}

		if key == oauth2const.ClaimCompletedAuthClass {
			strValue, ok := value.(string)
			if !ok {
				return claims, time.Time{}, errors.New("JWT 'completed_auth_class' claim is not a string")
			}
			claims.completedACR = strValue
			continue
		}
	}

	return claims, authTime, nil
}

// createAuthorizationCode generates an authorization code based on the provided
// authorization request context and authenticated user.
func createAuthorizationCode(
	authRequestCtx *authRequestContext,
	claims *assertionClaims,
	authTime time.Time,
) (AuthorizationCode, error) {
	clientID := authRequestCtx.OAuthParameters.ClientID
	redirectURI := authRequestCtx.OAuthParameters.RedirectURI

	if clientID == "" || redirectURI == "" {
		return AuthorizationCode{}, errors.New("client_id or redirect_uri is missing")
	}

	if claims.userID == "" {
		return AuthorizationCode{}, errors.New("authenticated user not found")
	}

	// Use provided authTime, or fallback to current time if zero (iat claim was not available).
	if authTime.IsZero() {
		authTime = time.Now()
	}

	standardScopes := authRequestCtx.OAuthParameters.StandardScopes
	permissionScopes := authRequestCtx.OAuthParameters.PermissionScopes
	allScopes := append(append([]string{}, standardScopes...), permissionScopes...)
	resources := authRequestCtx.OAuthParameters.Resources

	oauthConfig := config.GetServerRuntime().Config.OAuth
	validityPeriod := oauthConfig.AuthorizationCode.ValidityPeriod
	expiryTime := authTime.Add(time.Duration(validityPeriod) * time.Second)

	codeID, err := utils.GenerateUUIDv7()
	if err != nil {
		return AuthorizationCode{}, errors.New("failed to generate UUID")
	}

	code, err := oauth2utils.GenerateAuthorizationCode()
	if err != nil {
		return AuthorizationCode{}, errors.New("failed to generate authorization code")
	}

	return AuthorizationCode{
		CodeID:              codeID,
		Code:                code,
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		AuthorizedUserID:    claims.userID,
		AttributeCacheID:    claims.attributeCacheID,
		TimeCreated:         authTime,
		ExpiryTime:          expiryTime,
		Scopes:              utils.StringifyStringArray(allScopes, " "),
		State:               AuthCodeStateActive,
		CodeChallenge:       authRequestCtx.OAuthParameters.CodeChallenge,
		CodeChallengeMethod: authRequestCtx.OAuthParameters.CodeChallengeMethod,
		Resources:           resources,
		ClaimsRequest:       authRequestCtx.OAuthParameters.ClaimsRequest,
		ClaimsLocales:       authRequestCtx.OAuthParameters.ClaimsLocales,
		Nonce:               authRequestCtx.OAuthParameters.Nonce,
		CompletedACR:        claims.completedACR,
	}, nil
}

// getRequiredAttributes determines the essential and optional user attributes required based on OIDC scopes,
// claims parameter, response type, and app configuration.
func getRequiredAttributes(oidcScopes []string, claimsRequest *oauth2model.ClaimsRequest, responseType string,
	app *inboundmodel.OAuthClient) (essentialAttributes, optionalAttributes string) {
	if app == nil {
		return "", ""
	}

	essentialAttributesMap := make(map[string]bool)
	optionalAttributesMap := make(map[string]bool)

	// Add access token attributes from app config
	if app.Token != nil {
		appendAccessTokenAttributes(app, optionalAttributesMap)
	}

	// Process OIDC-related attributes only if openid scope is present
	if slices.Contains(oidcScopes, oauth2const.ScopeOpenID) {
		appendOIDCAttributes(oidcScopes, claimsRequest, responseType, app,
			essentialAttributesMap, optionalAttributesMap)
	}

	// Remove any duplicates between essential and optional attributes, giving precedence to essential
	if len(essentialAttributesMap) > 0 && len(optionalAttributesMap) > 0 {
		for attr := range essentialAttributesMap {
			if optionalAttributesMap[attr] {
				delete(optionalAttributesMap, attr)
			}
		}
	}

	// Convert attribute maps to space-separated strings
	essentialAttributes = strings.Join(slices.Collect(maps.Keys(essentialAttributesMap)), " ")
	optionalAttributes = strings.Join(slices.Collect(maps.Keys(optionalAttributesMap)), " ")

	return essentialAttributes, optionalAttributes
}

// appendAccessTokenAttributes appends access token attributes from app configuration.
func appendAccessTokenAttributes(app *inboundmodel.OAuthClient, attributesMap map[string]bool) {
	if app.Token.AccessToken != nil && len(app.Token.AccessToken.UserAttributes) > 0 {
		for _, attr := range app.Token.AccessToken.UserAttributes {
			attributesMap[attr] = true
		}
	}
}

// appendOIDCAttributes appends OIDC-related attributes from scopes and claims parameters.
func appendOIDCAttributes(oidcScopes []string, claimsRequest *oauth2model.ClaimsRequest, responseType string,
	app *inboundmodel.OAuthClient, essentialAttributes, optionalAttributes map[string]bool) {
	var idTokenAllowedSet map[string]bool
	if app.Token != nil {
		idTokenAllowedSet = buildIDTokenAllowedSet(app.Token.IDToken)
	}
	userInfoAllowedSet := buildUserInfoAllowedSet(app.UserInfo)

	appendAttributesFromClaimsParameter(claimsRequest, idTokenAllowedSet, userInfoAllowedSet,
		essentialAttributes, optionalAttributes)

	appendAttributesFromScopes(oidcScopes, responseType, app, idTokenAllowedSet, userInfoAllowedSet,
		optionalAttributes)
}

// buildIDTokenAllowedSet creates a set of allowed attributes for ID token.
func buildIDTokenAllowedSet(idTokenConfig *inboundmodel.IDTokenConfig) map[string]bool {
	if idTokenConfig == nil || len(idTokenConfig.UserAttributes) == 0 {
		return nil
	}
	allowedSet := make(map[string]bool, len(idTokenConfig.UserAttributes))
	for _, attr := range idTokenConfig.UserAttributes {
		allowedSet[attr] = true
	}
	return allowedSet
}

// buildUserInfoAllowedSet creates a set of allowed attributes for UserInfo.
func buildUserInfoAllowedSet(userInfoConfig *inboundmodel.UserInfoConfig) map[string]bool {
	if userInfoConfig == nil || len(userInfoConfig.UserAttributes) == 0 {
		return nil
	}
	allowedSet := make(map[string]bool, len(userInfoConfig.UserAttributes))
	for _, attr := range userInfoConfig.UserAttributes {
		allowedSet[attr] = true
	}
	return allowedSet
}

// appendAttributesFromClaimsParameter appends user attributes requested via the claims parameter.
func appendAttributesFromClaimsParameter(claimsRequest *oauth2model.ClaimsRequest,
	idTokenAllowedSet, userInfoAllowedSet, essentialAttributes, optionalAttributes map[string]bool) {
	if claimsRequest == nil {
		return
	}

	// Append id token attributes
	if claimsRequest.IDToken != nil && idTokenAllowedSet != nil {
		for name, value := range claimsRequest.IDToken {
			if idTokenAllowedSet[name] {
				if value != nil && value.Essential {
					essentialAttributes[name] = true
				} else {
					optionalAttributes[name] = true
				}
			}
		}
	}

	// Append user info attributes
	if claimsRequest.UserInfo != nil && userInfoAllowedSet != nil {
		for name, value := range claimsRequest.UserInfo {
			if userInfoAllowedSet[name] {
				if value != nil && value.Essential {
					essentialAttributes[name] = true
				} else {
					optionalAttributes[name] = true
				}
			}
		}
	}
}

// appendAttributesFromScopes appends user attributes based on OIDC scopes and app configuration.
func appendAttributesFromScopes(oidcScopes []string, responseType string, app *inboundmodel.OAuthClient,
	idTokenAllowedSet, userInfoAllowedSet map[string]bool, optionalAttributes map[string]bool) {
	for _, scope := range oidcScopes {
		scopeAttributes := resolveScopeAttributes(scope, app.ScopeClaims)
		appendAttributesForScope(scopeAttributes, responseType,
			idTokenAllowedSet, userInfoAllowedSet, optionalAttributes)
	}
}

// resolveScopeAttributes resolves attributes for a scope, checking app-specific mappings first.
func resolveScopeAttributes(scope string, scopeAttributesMapping map[string][]string) []string {
	// Check app-specific scope attributes mapping first
	if scopeAttributesMapping != nil {
		if appAttributes, exists := scopeAttributesMapping[scope]; exists {
			return appAttributes
		}
	}

	// Fall back to standard OIDC scopes
	if standardScope, exists := oauth2const.StandardOIDCScopes[scope]; exists {
		return standardScope.Claims
	}

	return nil
}

// appendAttributesForScope appends attributes for a particular scope based on response type and
// allowed attributes in app config.
// When using scopes, all attributes are treated as optional since there is no way to determine
// which attributes are essential vs optional.
func appendAttributesForScope(scopeAttributes []string, responseType string,
	idTokenAllowedSet, userInfoAllowedSet, optionalAttributes map[string]bool) {
	for _, attribute := range scopeAttributes {
		if responseType == string(oauth2const.ResponseTypeIDToken) {
			// If response type does not issue an access token, add claim to id token
			if idTokenAllowedSet != nil && idTokenAllowedSet[attribute] {
				optionalAttributes[attribute] = true
			}
		} else {
			// If response type issues an access token, add claim to userinfo
			if userInfoAllowedSet != nil && userInfoAllowedSet[attribute] {
				optionalAttributes[attribute] = true
			}
		}
	}
}

// validateSubClaimConstraint validates the sub claim constraint if specified in the claims parameter.
func validateSubClaimConstraint(claimsRequest *oauth2model.ClaimsRequest, actualSubject string) error {
	if claimsRequest == nil {
		return nil
	}

	// Check id_token sub claim constraint.
	if claimsRequest.IDToken != nil {
		if subReq, exists := claimsRequest.IDToken["sub"]; exists && subReq != nil {
			if !subReq.MatchesValue(actualSubject) {
				return errors.New("sub claim in id_token does not match requested value")
			}
		}
	}

	// Check userinfo sub claim constraint.
	if claimsRequest.UserInfo != nil {
		if subReq, exists := claimsRequest.UserInfo["sub"]; exists && subReq != nil {
			if !subReq.MatchesValue(actualSubject) {
				return errors.New("sub claim in userinfo does not match requested value")
			}
		}
	}

	return nil
}

// resolveUserAttributesCacheTTL determines the TTL for caching user attributes based on the
// token validity configuration. The largest of the access and refresh token (if allowed) validity
// periods is taken as the base, then the authorization code validity period is added to cover
// the window between code issuance and token exchange.
// A fixed buffer of attributeCacheTTLBufferSeconds is added to cover the window between
// authentication completion and token issuance.
func resolveUserAttributesCacheTTL(app *inboundmodel.OAuthClient) int64 {
	maxTTL := tokenservice.ResolveTokenConfig(app, tokenservice.TokenTypeAccess).ValidityPeriod
	if app.IsAllowedGrantType(oauth2const.GrantTypeRefreshToken) {
		refreshTTL := tokenservice.ResolveTokenConfig(app, tokenservice.TokenTypeRefresh).ValidityPeriod
		if refreshTTL > maxTTL {
			maxTTL = refreshTTL
		}
	}
	authCodeTTL := config.GetServerRuntime().Config.OAuth.AuthorizationCode.ValidityPeriod
	return maxTTL + authCodeTTL + oauth2const.AttributeCacheTTLBufferSeconds
}
