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

// Package authn implements the authentication service for authenticating users against different methods.
package authn

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/authn/assert"
	"github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/github"
	"github.com/thunder-id/thunderid/internal/authn/google"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	"github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/template"
)

const svcLoggerComponentName = "AuthenticationService"

// crossAllowedIDPTypes is the list of IDP types that allow cross-type authentication.
var crossAllowedIDPTypes = []providers.IDPType{providers.IDPTypeOAuth, providers.IDPTypeOIDC}

// AuthenticationServiceInterface defines the interface for the authentication service.
type AuthenticationServiceInterface interface {
	AuthenticateWithCredentials(ctx context.Context, identifiers, credentials map[string]interface{},
		skipAssertion bool, existingAssertion string) (*common.AuthenticationResponse, *tidcommon.ServiceError)
	SendOTP(ctx context.Context, senderID string, channel notifcommon.ChannelType, recipient, priorSessionToken string) (
		string, *tidcommon.ServiceError)
	VerifyOTP(ctx context.Context, sessionToken string, skipAssertion bool, existingAssertion, otp string) (
		*common.AuthenticationResponse, *tidcommon.ServiceError)
	StartIDPAuthentication(ctx context.Context, requestedType providers.IDPType, idpID string) (
		*IDPAuthInitData, *tidcommon.ServiceError)
	FinishIDPAuthentication(
		ctx context.Context,
		requestedType providers.IDPType,
		sessionToken string,
		skipAssertion bool,
		existingAssertion, code string,
	) (*common.AuthenticationResponse, *tidcommon.ServiceError)
	// Passkey methods
	StartPasskeyRegistration(ctx context.Context, userID, relyingPartyID, relyingPartyName string,
		authSelection *PasskeyAuthenticatorSelectionDTO, attestation string,
	) (interface{}, *tidcommon.ServiceError)
	FinishPasskeyRegistration(ctx context.Context, credential PasskeyPublicKeyCredentialDTO,
		sessionToken string, skipAssertion bool, existingAssertion string,
	) (*common.AuthenticationResponse, *tidcommon.ServiceError)
	StartPasskeyAuthentication(
		ctx context.Context, userID, relyingPartyID string,
	) (interface{}, *tidcommon.ServiceError)
	FinishPasskeyAuthentication(
		ctx context.Context,
		credentialID, credentialType string,
		response PasskeyCredentialResponseDTO,
		sessionToken string,
		skipAssertion bool,
		existingAssertion string,
	) (*common.AuthenticationResponse, *tidcommon.ServiceError)
}

// authenticationService is the default implementation of the AuthenticationServiceInterface.
type authenticationService struct {
	idpService             idp.IDPServiceInterface
	jwtService             jwt.JWTServiceInterface
	authAssertionGenerator assert.AuthAssertGeneratorInterface
	authnProvider          providers.AuthnProviderManager
	otpService             otp.OTPAuthnServiceInterface
	notifSenderSvc         notification.NotificationSenderServiceInterface
	templateService        template.TemplateServiceInterface
	magicLinkService       magiclink.MagicLinkAuthnServiceInterface
	oauthService           oauth.OAuthAuthnServiceInterface
	oidcService            oidc.OIDCAuthnServiceInterface
	googleService          google.GoogleOIDCAuthnServiceInterface
	githubService          github.GithubOAuthAuthnServiceInterface
}

// newAuthenticationService creates a new instance of AuthenticationService.
func newAuthenticationService(
	idpSvc idp.IDPServiceInterface,
	jwtSvc jwt.JWTServiceInterface,
	authAssertGen assert.AuthAssertGeneratorInterface,
	authnProvider providers.AuthnProviderManager,
	otpAuthnSvc otp.OTPAuthnServiceInterface,
	notifSenderSvc notification.NotificationSenderServiceInterface,
	templateSvc template.TemplateServiceInterface,
	magicLinkSvc magiclink.MagicLinkAuthnServiceInterface,
	oauthAuthnSvc oauth.OAuthAuthnServiceInterface,
	oidcAuthnSvc oidc.OIDCAuthnServiceInterface,
	googleAuthnSvc google.GoogleOIDCAuthnServiceInterface,
	githubAuthnSvc github.GithubOAuthAuthnServiceInterface,
) AuthenticationServiceInterface {
	return &authenticationService{
		idpService:             idpSvc,
		jwtService:             jwtSvc,
		authAssertionGenerator: authAssertGen,
		authnProvider:          authnProvider,
		otpService:             otpAuthnSvc,
		notifSenderSvc:         notifSenderSvc,
		templateService:        templateSvc,
		magicLinkService:       magicLinkSvc,
		oauthService:           oauthAuthnSvc,
		oidcService:            oidcAuthnSvc,
		googleService:          googleAuthnSvc,
		githubService:          githubAuthnSvc,
	}
}

// AuthenticateWithCredentials authenticates a user using credentials.
func (as *authenticationService) AuthenticateWithCredentials(ctx context.Context, identifiers,
	credentials map[string]interface{}, skipAssertion bool, existingAssertion string) (
	*common.AuthenticationResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug(ctx, "Authenticating with credentials")

	if len(identifiers) == 0 || len(credentials) == 0 {
		return nil, &ErrorEmptyAttributesOrCredentials
	}

	newAuthUser, _, svcErr := as.authnProvider.AuthenticateUser(ctx, identifiers, credentials, nil, nil,
		providers.AuthUser{})
	if svcErr != nil {
		return nil, as.mapCredentialsAuthnError(ctx, svcErr, logger)
	}

	newAuthUser, entityRef, svcErr := as.authnProvider.GetEntityReference(ctx, newAuthUser)
	if svcErr != nil {
		return nil, as.mapCredentialsGetAttributesError(ctx, svcErr, logger)
	}

	_, attrsResponse, svcErr := as.authnProvider.GetUserAttributes(ctx, nil, nil, newAuthUser)
	if svcErr != nil {
		return nil, as.mapCredentialsGetAttributesError(ctx, svcErr, logger)
	}

	authResponse := &common.AuthenticationResponse{
		ID:   entityRef.EntityID,
		Type: entityRef.EntityType,
		OUID: entityRef.OUID,
	}

	// Generate assertion if not skipped
	if !skipAssertion {
		authUserAttributes := make(map[string]interface{})
		if attrsResponse != nil && attrsResponse.Attributes != nil {
			for attrName, attrValue := range attrsResponse.Attributes {
				authUserAttributes[attrName] = attrValue.Value
			}
		}
		authUserAttributesJSON, err := json.Marshal(authUserAttributes)
		if err != nil {
			logger.Error(ctx, "Failed to marshal user attributes")
			return nil, &tidcommon.InternalServerError
		}

		authenticatedUser := &providers.Entity{
			ID:         entityRef.EntityID,
			Type:       entityRef.EntityType,
			OUID:       entityRef.OUID,
			Attributes: authUserAttributesJSON,
		}
		svcErr = as.validateAndAppendAuthAssertion(
			ctx, authResponse, authenticatedUser, common.AuthenticatorCredentials, existingAssertion, logger)
		if svcErr != nil {
			return nil, svcErr
		}
	}

	return authResponse, nil
}

// SendOTP generates an OTP, renders the SMS template, and delivers it to the recipient.
func (as *authenticationService) SendOTP(ctx context.Context, senderID string, channel notifcommon.ChannelType,
	recipient, priorSessionToken string) (string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))

	sessionToken, otpValue, _, svcErr := as.otpService.GenerateOTP(ctx, recipient, "mobile_number", priorSessionToken)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			logger.Error(ctx, "Failed to generate OTP", log.String("error", svcErr.Code))
			return "", &tidcommon.InternalServerError
		}
		return "", svcErr
	}

	otpCfg := config.GetServerRuntime().Config.Notification.OTP
	expiryMinutes := strconv.FormatInt(int64(otpCfg.ValidityPeriodSeconds)/60, 10)
	templateData := template.TemplateData{"otpCode": otpValue, "expiryMinutes": expiryMinutes}
	rendered, renderErr := as.templateService.Render(ctx, template.ScenarioOTP, template.TemplateTypeSMS, templateData)
	if renderErr != nil {
		if renderErr.Type == tidcommon.ServerErrorType {
			logger.Error(ctx, "Failed to render OTP template", log.String("error", renderErr.Code))
			return "", &tidcommon.InternalServerError
		}
		return "", renderErr
	}

	notifData := notifcommon.NotificationData{Recipient: recipient, Body: rendered.Body}
	if sendErr := as.notifSenderSvc.Send(ctx, channel, senderID, notifData); sendErr != nil {
		if sendErr.Type == tidcommon.ServerErrorType {
			logger.Error(ctx, "Failed to send OTP notification", log.String("error", sendErr.Code))
			return "", &tidcommon.InternalServerError
		}
		return "", sendErr
	}

	return sessionToken, nil
}

// VerifyOTP verifies an OTP and returns the authenticated user.
func (as *authenticationService) VerifyOTP(ctx context.Context, sessionToken string, skipAssertion bool,
	existingAssertion, otpCode string) (*common.AuthenticationResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug(ctx, "Verifying OTP for authentication")

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": sessionToken,
			"otp":          otpCode,
		},
	}
	authUser, _, svcErr := as.authnProvider.AuthenticateUser(
		ctx, nil, credentials, nil, nil, providers.AuthUser{})
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, &tidcommon.InternalServerError
		}
		if svcErr.Code == authnprovidermgr.ErrorAuthenticationFailed.Code {
			return nil, &ErrorOTPAuthenticationFailed
		}
		return nil, svcErr
	}

	_, entityRef, svcErr := as.authnProvider.GetEntityReference(ctx, authUser)
	if svcErr != nil {
		return nil, as.mapCredentialsGetAttributesError(ctx, svcErr, logger)
	}

	authResponse := &common.AuthenticationResponse{
		ID:   entityRef.EntityID,
		Type: entityRef.EntityType,
		OUID: entityRef.OUID,
	}

	// Generate assertion if not skipped
	if !skipAssertion {
		userForAssertion := &providers.Entity{
			ID:         entityRef.EntityID,
			Type:       entityRef.EntityType,
			OUID:       entityRef.OUID,
			Attributes: nil, // Attributes not needed for assertion generation in OTP flow
		}
		svcErr = as.validateAndAppendAuthAssertion(ctx, authResponse, userForAssertion, common.AuthenticatorOTP,
			existingAssertion, logger)
		if svcErr != nil {
			return nil, svcErr
		}
	}

	return authResponse, nil
}

// StartIDPAuthentication initiates authentication against an IDP.
func (as *authenticationService) StartIDPAuthentication(
	ctx context.Context,
	requestedType providers.IDPType,
	idpID string,
) (
	*IDPAuthInitData, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug(ctx, "Starting IDP authentication", log.String("idpId", idpID))

	if strings.TrimSpace(idpID) == "" {
		return nil, &common.ErrorInvalidIDPID
	}

	identityProvider, svcErr := as.idpService.GetIdentityProvider(ctx, idpID)
	if svcErr != nil {
		return nil, as.handleIDPServiceError(ctx, idpID, svcErr, logger)
	}

	if svcErr := as.validateIDPType(ctx, requestedType, identityProvider.Type, logger); svcErr != nil {
		return nil, svcErr
	}

	// Route to appropriate service based on IDP type
	var redirectURL string
	var metadata map[string]string
	var buildURLErr *tidcommon.ServiceError
	switch identityProvider.Type {
	case providers.IDPTypeOAuth:
		redirectURL, _, buildURLErr = as.oauthService.BuildAuthorizeURL(ctx, idpID)
	case providers.IDPTypeOIDC:
		redirectURL, metadata, buildURLErr = as.oidcService.BuildAuthorizeURL(ctx, idpID)
	case providers.IDPTypeGoogle:
		redirectURL, metadata, buildURLErr = as.googleService.BuildAuthorizeURL(ctx, idpID)
	case providers.IDPTypeGitHub:
		redirectURL, _, buildURLErr = as.githubService.BuildAuthorizeURL(ctx, idpID)
	default:
		logger.Error(ctx, "Unsupported IDP type", log.String("idpId", idpID),
			log.String("type", string(identityProvider.Type)))
		return nil, &tidcommon.InternalServerError
	}

	if buildURLErr != nil {
		return nil, buildURLErr
	}

	// Generate session token, embedding the OIDC nonce when present.
	var nonce string
	if metadata != nil {
		nonce = metadata[oauth2const.RequestParamNonce]
	}
	sessionToken, err := as.createSessionToken(ctx, idpID, identityProvider.Type, nonce)
	if err != nil {
		logger.Error(ctx, "Failed to create session token", log.String("idpId", idpID),
			log.String("error", err.Error.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}

	return &IDPAuthInitData{
		RedirectURL:  redirectURL,
		SessionToken: sessionToken,
	}, nil
}

// FinishIDPAuthentication completes authentication against an IDP.
func (as *authenticationService) FinishIDPAuthentication(ctx context.Context, requestedType providers.IDPType,
	sessionToken string, skipAssertion bool, existingAssertion, code string) (
	*common.AuthenticationResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug(ctx, "Finishing IDP authentication")

	if strings.TrimSpace(sessionToken) == "" {
		return nil, &common.ErrorEmptySessionToken
	}
	if strings.TrimSpace(code) == "" {
		return nil, &common.ErrorEmptyAuthCode
	}

	// Verify and decode session token
	sessionData, svcErr := as.verifyAndDecodeSessionToken(ctx, sessionToken, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	if svcErr := as.validateIDPType(ctx, requestedType, sessionData.IDPType, logger); svcErr != nil {
		return nil, svcErr
	}

	credentials := map[string]interface{}{
		"federated": &common.FederatedAuthCredential{
			IDPID:   sessionData.IDPID,
			IDPType: sessionData.IDPType,
			AuthorizationData: common.AuthorizationData{
				Code:  code,
				Nonce: sessionData.Nonce,
			},
		},
	}
	authUser, _, svcErr := as.authnProvider.AuthenticateUser(
		ctx, nil, credentials, nil, nil, providers.AuthUser{})
	if svcErr != nil {
		return nil, as.mapFederatedAuthnError(ctx, svcErr, logger)
	}

	_, entityRef, svcErr := as.authnProvider.GetEntityReference(ctx, authUser)
	if svcErr != nil {
		return nil, as.mapCredentialsGetAttributesError(ctx, svcErr, logger)
	}

	user := &providers.Entity{
		ID:   entityRef.EntityID,
		Type: entityRef.EntityType,
		OUID: entityRef.OUID,
	}

	authResponse := &common.AuthenticationResponse{
		ID:   user.ID,
		Type: user.Type,
		OUID: user.OUID,
	}

	// Generate assertion if not skipped
	if !skipAssertion {
		authenticatorName, err := common.GetAuthenticatorNameForIDPType(sessionData.IDPType)
		if err != nil {
			logger.Error(ctx, "Failed to get authenticator name for IDP type",
				log.String("idpType", string(sessionData.IDPType)), log.Error(err))
			return nil, &tidcommon.InternalServerError
		}

		svcErr = as.validateAndAppendAuthAssertion(ctx, authResponse, user, authenticatorName,
			existingAssertion, logger)
		if svcErr != nil {
			return nil, svcErr
		}
	}

	return authResponse, nil
}

// validateAndAppendAuthAssertion validates and appends a generated auth assertion to the authentication response.
func (as *authenticationService) validateAndAppendAuthAssertion(
	ctx context.Context, authResponse *common.AuthenticationResponse, user *providers.Entity, authenticator string,
	existingAssertion string, logger *log.Logger,
) *tidcommon.ServiceError {
	logger.Debug(ctx, "Generating auth assertion", log.MaskedString(log.LoggerKeyUserID, user.ID))

	authenticatorRef := &common.AuthenticatorReference{
		Authenticator: authenticator,
		Timestamp:     time.Now().Unix(),
	}

	// Extract existing assurance if provided and set appropriate step number
	var existingAssurance *assert.AssuranceContext
	if strings.TrimSpace(existingAssertion) != "" {
		var assertionSub string
		var svcErr *tidcommon.ServiceError
		existingAssurance, assertionSub, svcErr = as.extractClaimsFromAssertion(ctx, existingAssertion, logger)
		if svcErr != nil {
			return svcErr
		}

		// Validate that the assertion subject matches the current user
		if assertionSub != user.ID {
			logger.Debug(ctx, "Assertion subject mismatch", log.MaskedString("assertionSub", assertionSub),
				log.MaskedString(log.LoggerKeyUserID, user.ID))
			return &common.ErrorAssertionSubjectMismatch
		}

		if existingAssurance != nil {
			authenticatorRef.Step = len(existingAssurance.Authenticators) + 1
		} else {
			authenticatorRef.Step = 1
		}
	} else {
		authenticatorRef.Step = 1
	}

	// Prepare JWT claims
	jwtClaims := make(map[string]interface{})
	if user.Type != "" {
		jwtClaims["userType"] = user.Type
	}
	if user.OUID != "" {
		jwtClaims["ouId"] = user.OUID
	}

	// Get authentication assertion result
	assertionResult, svcErr := as.getAssertionResult(ctx, existingAssurance, authenticatorRef)
	if svcErr != nil {
		return svcErr
	}

	if assertionResult != nil {
		jwtClaims["assurance"] = assertionResult.Context
	}

	// Generate auth assertion JWT
	jwtConfig := config.GetServerRuntime().Config.JWT
	jwtClaims["aud"] = jwtConfig.Audience
	token, _, err := as.jwtService.GenerateJWT(ctx, user.ID, jwtConfig.Issuer,
		jwtConfig.ValidityPeriod, jwtClaims, jwt.TokenTypeJWT, "")
	if err != nil {
		logger.Error(ctx, "Failed to generate auth assertion", log.String("error", err.Error.DefaultValue))
		return &tidcommon.InternalServerError
	}

	authResponse.Assertion = token
	return nil
}

// getAssertionResult generates or updates an assertion result based on existing context.
func (as *authenticationService) getAssertionResult(ctx context.Context, existingContext *assert.AssuranceContext,
	newAuthenticator *common.AuthenticatorReference) (
	*assert.AssertionResult, *tidcommon.ServiceError) {
	var assertionResult *assert.AssertionResult
	var svcErr *tidcommon.ServiceError
	if existingContext != nil && newAuthenticator != nil {
		// Update existing assurance with new authenticator
		assertionResult, svcErr = as.authAssertionGenerator.UpdateAssertion(ctx,
			existingContext, *newAuthenticator)
	} else if newAuthenticator != nil {
		// Generate new assurance from authenticator
		assertionResult, svcErr = as.authAssertionGenerator.GenerateAssertion(ctx,
			[]common.AuthenticatorReference{*newAuthenticator})
	}

	return assertionResult, svcErr
}

// extractClaimsFromAssertion extracts assurance context and subject from an existing JWT assertion.
func (as *authenticationService) extractClaimsFromAssertion(ctx context.Context, assertion string,
	logger *log.Logger) (*assert.AssuranceContext, string, *tidcommon.ServiceError) {
	jwtConfig := config.GetServerRuntime().Config.JWT

	if err := as.jwtService.VerifyJWT(ctx, assertion, "", jwtConfig.Issuer); err != nil {
		logger.Debug(ctx, "Failed to verify JWT signature of the assertion",
			log.String("error", err.Error.DefaultValue))
		return nil, "", &common.ErrorInvalidAssertion
	}

	payload, err := jwt.DecodeJWTPayload(assertion)
	if err != nil {
		logger.Debug(ctx, "Failed to decode JWT assertion", log.Error(err))
		return nil, "", &common.ErrorInvalidAssertion
	}

	// Extract subject claim
	subClaim, ok := payload["sub"]
	if !ok {
		logger.Debug(ctx, "No 'sub' claim found in JWT assertion")
		return nil, "", &common.ErrorInvalidAssertion
	}
	sub, ok := subClaim.(string)
	if !ok || strings.TrimSpace(sub) == "" {
		logger.Debug(ctx, "Invalid 'sub' claim in JWT assertion")
		return nil, "", &common.ErrorInvalidAssertion
	}

	// Extract assurance claim
	assuranceClaim, ok := payload["assurance"]
	if !ok {
		logger.Debug(ctx, "No assurance claim found in JWT assertion")
		return nil, "", &common.ErrorInvalidAssertion
	}

	// Convert assurance claim to AssuranceContext
	assuranceBytes, err := json.Marshal(assuranceClaim)
	if err != nil {
		logger.Debug(ctx, "Failed to marshal assurance claim", log.Error(err))
		return nil, "", &common.ErrorInvalidAssertion
	}

	var assuranceCtx assert.AssuranceContext
	if err := json.Unmarshal(assuranceBytes, &assuranceCtx); err != nil {
		logger.Debug(ctx, "Failed to unmarshal assurance claim to AssuranceContext", log.Error(err))
		return nil, "", &common.ErrorInvalidAssertion
	}

	return &assuranceCtx, sub, nil
}

// mapFederatedAuthnError maps provider manager errors to federated-authentication-specific service errors.
func (as *authenticationService) mapFederatedAuthnError(ctx context.Context, svcErr *tidcommon.ServiceError,
	logger *log.Logger) *tidcommon.ServiceError {
	switch svcErr.Code {
	case authnprovidermgr.ErrorAuthenticationFailed.Code:
		return &ErrorFederatedAuthenticationFailed
	case authnprovidermgr.ErrorUserNotFound.Code:
		return &ErrorFederatedAuthenticationFailed
	case authnprovidermgr.ErrorInvalidRequest.Code:
		return &ErrorFederatedAuthenticationFailed
	default:
		logger.Error(ctx, "Error occurred while performing federated authentication",
			log.String("errorCode", svcErr.Code), log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return &tidcommon.InternalServerError
	}
}

// mapCredentialsAuthnError maps provider manager errors to credentials-specific service errors.
func (as *authenticationService) mapCredentialsAuthnError(ctx context.Context, svcErr *tidcommon.ServiceError,
	logger *log.Logger) *tidcommon.ServiceError {
	switch svcErr.Code {
	case authnprovidermgr.ErrorAuthenticationFailed.Code:
		return &ErrorInvalidCredentials
	case authnprovidermgr.ErrorUserNotFound.Code:
		return &common.ErrorUserNotFound
	case authnprovidermgr.ErrorInvalidRequest.Code:
		return &ErrorEmptyAttributesOrCredentials
	default:
		logger.Error(ctx, "Error occurred while authenticating with credentials",
			log.String("errorCode", svcErr.Code), log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return &tidcommon.InternalServerError
	}
}

// mapCredentialsGetAttributesError maps provider manager errors from GetUserAttributes to credentials-specific errors.
func (as *authenticationService) mapCredentialsGetAttributesError(
	ctx context.Context, svcErr *tidcommon.ServiceError,
	logger *log.Logger) *tidcommon.ServiceError {
	switch svcErr.Code {
	case authnprovidermgr.ErrorGetAttributesClientError.Code,
		authnprovidermgr.ErrorGetEntityReferenceClientError.Code:
		return &ErrorInvalidToken
	default:
		logger.Error(ctx, "Error occurred while getting attributes for credentials authentication",
			log.String("errorCode", svcErr.Code), log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return &tidcommon.InternalServerError
	}
}

// handleIDPServiceError handles errors from IDP service.
func (as *authenticationService) handleIDPServiceError(
	ctx context.Context, idpID string, svcErr *tidcommon.ServiceError,
	logger *log.Logger) *tidcommon.ServiceError {
	if svcErr.Type == tidcommon.ClientErrorType {
		errDesc := fmt.Sprintf(
			"An error occurred while retrieving the identity provider with ID %s: %s",
			idpID,
			svcErr.ErrorDescription.DefaultValue,
		)
		return tidcommon.CustomServiceError(common.ErrorClientErrorWhileRetrievingIDP, tidcommon.I18nMessage{
			Key:          "error.authnservice.error_retrieving_idp_description",
			DefaultValue: errDesc,
		})
	}

	logger.Error(ctx, "Error occurred while retrieving IDP",
		log.String("idpId", idpID), log.Any("error", svcErr))
	return &tidcommon.InternalServerError
}

// validateIDPType validates that the requested IDP type matches the actual IDP type.
func (as *authenticationService) validateIDPType(ctx context.Context, requestedType, actualType providers.IDPType,
	logger *log.Logger) *tidcommon.ServiceError {
	if requestedType != "" && requestedType != actualType {
		// Allow cross-type authentication for certain types
		if slices.Contains(crossAllowedIDPTypes, requestedType) &&
			slices.Contains(crossAllowedIDPTypes, actualType) {
			return nil
		}

		logger.Debug(ctx, "IDP type mismatch", log.String("requested", string(requestedType)),
			log.String("actual", string(actualType)))
		return &common.ErrorInvalidIDPType
	}

	return nil
}

// createSessionToken creates a JWT session token with authentication session data.
func (as *authenticationService) createSessionToken(ctx context.Context, idpID string,
	idpType providers.IDPType, nonce string) (string, *tidcommon.ServiceError) {
	sessionData := AuthSessionData{
		IDPID:   idpID,
		IDPType: idpType,
		Nonce:   nonce,
	}
	claims := map[string]interface{}{
		"auth_data": sessionData,
	}

	jwtConfig := config.GetServerRuntime().Config.JWT
	claims["aud"] = "auth-svc"
	token, _, err := as.jwtService.GenerateJWT(ctx, "auth-svc", jwtConfig.Issuer, 600, claims, jwt.TokenTypeJWT, "")
	if err != nil {
		return "", err
	}

	return token, nil
}

// verifyAndDecodeSessionToken verifies the JWT signature and decodes the auth session data.
func (as *authenticationService) verifyAndDecodeSessionToken(ctx context.Context, token string, logger *log.Logger) (
	*AuthSessionData, *tidcommon.ServiceError) {
	// Verify JWT signature and claims
	jwtConfig := config.GetServerRuntime().Config.JWT
	svcErr := as.jwtService.VerifyJWT(ctx, token, "auth-svc", jwtConfig.Issuer)
	if svcErr != nil {
		logger.Debug(ctx, "Error verifying session token", log.String("error", svcErr.Error.DefaultValue))
		return nil, &common.ErrorInvalidSessionToken
	}

	// Parse and extract authentication session data
	payload, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		logger.Debug(ctx, "Error decoding session token payload", log.Error(err))
		return nil, &common.ErrorInvalidSessionToken
	}

	authDataClaim, ok := payload["auth_data"]
	if !ok {
		logger.Debug(ctx, "auth_data claim not found in session token")
		return nil, &common.ErrorInvalidSessionToken
	}

	authDataBytes, err := json.Marshal(authDataClaim)
	if err != nil {
		logger.Debug(ctx, "Error marshaling auth_data claim", log.Error(err))
		return nil, &common.ErrorInvalidSessionToken
	}

	var sessionData AuthSessionData
	err = json.Unmarshal(authDataBytes, &sessionData)
	if err != nil {
		logger.Debug(ctx, "Error marshaling auth_data claim", log.Error(err))
		return nil, &common.ErrorInvalidSessionToken
	}

	return &sessionData, nil
}

// StartPasskeyRegistration starts the passkey registration process.
func (as *authenticationService) StartPasskeyRegistration(
	ctx context.Context, userID, relyingPartyID, relyingPartyName string,
	authSelection *PasskeyAuthenticatorSelectionDTO, attestation string,
) (interface{}, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug(ctx, "Starting Passkey registration")

	var passkeyAuthSel *passkey.AuthenticatorSelection
	if authSelection != nil {
		passkeyAuthSel = &passkey.AuthenticatorSelection{
			AuthenticatorAttachment: authSelection.AuthenticatorAttachment,
			RequireResidentKey:      authSelection.RequireResidentKey,
			ResidentKey:             authSelection.ResidentKey,
			UserVerification:        authSelection.UserVerification,
		}
	}

	req := &passkey.PasskeyRegistrationStartRequest{
		UserID:                 userID,
		RelyingPartyID:         relyingPartyID,
		RelyingPartyName:       relyingPartyName,
		AuthenticatorSelection: passkeyAuthSel,
		Attestation:            attestation,
	}

	result, svcErr := as.authnProvider.InitiateEnrollment(ctx, passkey.CredentialType, req, nil)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, &tidcommon.InternalServerError
		}
		return nil, &ErrorPasskeyEnrollmentFailed
	}
	return result, nil
}

// FinishPasskeyRegistration completes the passkey registration process.
func (as *authenticationService) FinishPasskeyRegistration(
	ctx context.Context, credential PasskeyPublicKeyCredentialDTO,
	sessionToken string, skipAssertion bool,
	existingAssertion string,
) (*common.AuthenticationResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug(ctx, "Finishing Passkey registration")

	req := &passkey.PasskeyRegistrationFinishRequest{
		CredentialID:      credential.ID,
		CredentialType:    credential.Type,
		ClientDataJSON:    credential.Response.ClientDataJSON,
		AttestationObject: credential.Response.AttestationObject,
		SessionToken:      sessionToken,
	}
	credentials := map[string]interface{}{passkey.CredentialType: req}
	authUser, _, svcErr := as.authnProvider.Enroll(
		ctx, nil, credentials, nil, nil, providers.AuthUser{})
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, &tidcommon.InternalServerError
		}
		return nil, &ErrorPasskeyEnrollmentFailed
	}

	return as.buildPasskeyAuthenticationResponse(ctx, authUser, skipAssertion, existingAssertion, logger)
}

// StartPasskeyAuthentication starts the passkey authentication process.
func (as *authenticationService) StartPasskeyAuthentication(ctx context.Context, userID, relyingPartyID string) (
	interface{}, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug(ctx, "Starting Passkey authentication")

	req := &passkey.PasskeyAuthenticationStartRequest{
		UserID:         userID,
		RelyingPartyID: relyingPartyID,
	}
	result, svcErr := as.authnProvider.InitiateAuthentication(ctx, passkey.CredentialType, req, nil)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, &tidcommon.InternalServerError
		}
		return nil, &ErrorPasskeyAuthenticationFailed
	}
	return result, nil
}

// FinishPasskeyAuthentication completes the passkey authentication process.
func (as *authenticationService) FinishPasskeyAuthentication(ctx context.Context, credentialID, credentialType string,
	response PasskeyCredentialResponseDTO, sessionToken string, skipAssertion bool,
	existingAssertion string) (*common.AuthenticationResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug(ctx, "Finishing Passkey authentication")

	passkeyCredential := &passkey.PasskeyAuthenticationFinishRequest{
		CredentialID:      credentialID,
		CredentialType:    credentialType,
		ClientDataJSON:    response.ClientDataJSON,
		AuthenticatorData: response.AuthenticatorData,
		Signature:         response.Signature,
		UserHandle:        response.UserHandle,
		SessionToken:      sessionToken,
	}
	credentials := map[string]interface{}{passkey.CredentialType: passkeyCredential}
	authUser, _, svcErr := as.authnProvider.AuthenticateUser(
		ctx, nil, credentials, nil, nil, providers.AuthUser{})
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, &tidcommon.InternalServerError
		}
		return nil, &ErrorPasskeyAuthenticationFailed
	}

	return as.buildPasskeyAuthenticationResponse(ctx, authUser, skipAssertion, existingAssertion, logger)
}

// buildPasskeyAuthenticationResponse resolves the entity reference for an authenticated
// AuthUser and builds the authentication response.
func (as *authenticationService) buildPasskeyAuthenticationResponse(ctx context.Context,
	authUser providers.AuthUser, skipAssertion bool, existingAssertion string, logger *log.Logger,
) (*common.AuthenticationResponse, *tidcommon.ServiceError) {
	_, entityRef, svcErr := as.authnProvider.GetEntityReference(ctx, authUser)
	if svcErr != nil {
		return nil, as.mapCredentialsGetAttributesError(ctx, svcErr, logger)
	}

	authResponse := &common.AuthenticationResponse{
		ID:   entityRef.EntityID,
		Type: entityRef.EntityType,
		OUID: entityRef.OUID,
	}

	if skipAssertion {
		return authResponse, nil
	}

	userForAssertion := &providers.Entity{
		ID:   entityRef.EntityID,
		Type: entityRef.EntityType,
		OUID: entityRef.OUID,
	}
	if svcErr := as.validateAndAppendAuthAssertion(ctx, authResponse, userForAssertion,
		common.AuthenticatorPasskey, existingAssertion, logger); svcErr != nil {
		return nil, svcErr
	}
	return authResponse, nil
}
