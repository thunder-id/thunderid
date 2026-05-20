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
	"strings"
	"time"

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
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/idp"
	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const svcLoggerComponentName = "AuthenticationService"

// crossAllowedIDPTypes is the list of IDP types that allow cross-type authentication.
var crossAllowedIDPTypes = []idp.IDPType{idp.IDPTypeOAuth, idp.IDPTypeOIDC}

// AuthenticationServiceInterface defines the interface for the authentication service.
type AuthenticationServiceInterface interface {
	AuthenticateWithCredentials(ctx context.Context, identifiers, credentials map[string]interface{},
		skipAssertion bool, existingAssertion string) (*common.AuthenticationResponse, *serviceerror.ServiceError)
	SendOTP(ctx context.Context, senderID string, channel notifcommon.ChannelType, recipient string) (
		string, *serviceerror.ServiceError)
	VerifyOTP(ctx context.Context, sessionToken string, skipAssertion bool, existingAssertion, otp string) (
		*common.AuthenticationResponse, *serviceerror.ServiceError)
	StartIDPAuthentication(ctx context.Context, requestedType idp.IDPType, idpID string) (
		*IDPAuthInitData, *serviceerror.ServiceError)
	FinishIDPAuthentication(ctx context.Context, requestedType idp.IDPType, sessionToken string, skipAssertion bool,
		existingAssertion, code string) (*common.AuthenticationResponse, *serviceerror.ServiceError)
	// Passkey methods
	StartPasskeyRegistration(ctx context.Context, userID, relyingPartyID, relyingPartyName string,
		authSelection *PasskeyAuthenticatorSelectionDTO, attestation string,
	) (interface{}, *serviceerror.ServiceError)
	FinishPasskeyRegistration(ctx context.Context, credential PasskeyPublicKeyCredentialDTO, sessionToken,
		credentialName string) (interface{}, *serviceerror.ServiceError)
	StartPasskeyAuthentication(
		ctx context.Context, userID, relyingPartyID string,
	) (interface{}, *serviceerror.ServiceError)
	FinishPasskeyAuthentication(
		ctx context.Context,
		credentialID, credentialType string,
		response PasskeyCredentialResponseDTO,
		sessionToken string,
		skipAssertion bool,
		existingAssertion string,
	) (*common.AuthenticationResponse, *serviceerror.ServiceError)
}

// authenticationService is the default implementation of the AuthenticationServiceInterface.
type authenticationService struct {
	idpService             idp.IDPServiceInterface
	jwtService             jwt.JWTServiceInterface
	authAssertionGenerator assert.AuthAssertGeneratorInterface
	authnProvider          authnprovidermgr.AuthnProviderManagerInterface
	otpService             otp.OTPAuthnServiceInterface
	magicLinkService       magiclink.MagicLinkAuthnServiceInterface
	oauthService           oauth.OAuthAuthnServiceInterface
	oidcService            oidc.OIDCAuthnServiceInterface
	googleService          google.GoogleOIDCAuthnServiceInterface
	githubService          github.GithubOAuthAuthnServiceInterface
	passkeyService         passkey.PasskeyServiceInterface
}

// newAuthenticationService creates a new instance of AuthenticationService.
func newAuthenticationService(
	idpSvc idp.IDPServiceInterface,
	jwtSvc jwt.JWTServiceInterface,
	authAssertGen assert.AuthAssertGeneratorInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	otpAuthnSvc otp.OTPAuthnServiceInterface,
	magicLinkSvc magiclink.MagicLinkAuthnServiceInterface,
	oauthAuthnSvc oauth.OAuthAuthnServiceInterface,
	oidcAuthnSvc oidc.OIDCAuthnServiceInterface,
	googleAuthnSvc google.GoogleOIDCAuthnServiceInterface,
	githubAuthnSvc github.GithubOAuthAuthnServiceInterface,
	passkeySvc passkey.PasskeyServiceInterface,
) AuthenticationServiceInterface {
	return &authenticationService{
		idpService:             idpSvc,
		jwtService:             jwtSvc,
		authAssertionGenerator: authAssertGen,
		authnProvider:          authnProvider,
		otpService:             otpAuthnSvc,
		magicLinkService:       magicLinkSvc,
		oauthService:           oauthAuthnSvc,
		oidcService:            oidcAuthnSvc,
		googleService:          googleAuthnSvc,
		githubService:          githubAuthnSvc,
		passkeyService:         passkeySvc,
	}
}

// AuthenticateWithCredentials authenticates a user using credentials.
func (as *authenticationService) AuthenticateWithCredentials(ctx context.Context, identifiers,
	credentials map[string]interface{}, skipAssertion bool, existingAssertion string) (
	*common.AuthenticationResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug("Authenticating with credentials")

	if len(identifiers) == 0 || len(credentials) == 0 {
		return nil, &ErrorEmptyAttributesOrCredentials
	}

	newAuthUser, basicResult, svcErr := as.authnProvider.AuthenticateUser(ctx, identifiers, credentials, nil, nil,
		authnprovidermgr.AuthUser{})
	if svcErr != nil {
		return nil, as.mapCredentialsAuthnError(svcErr, logger)
	}

	if basicResult == nil {
		logger.Error("Credentials authenticate response is nil")
		return nil, &serviceerror.InternalServerError
	}

	_, attrsResponse, svcErr := as.authnProvider.GetUserAttributes(ctx, nil, nil, newAuthUser)
	if svcErr != nil {
		return nil, as.mapCredentialsGetAttributesError(svcErr, logger)
	}

	authResponse := &common.AuthenticationResponse{
		ID:   basicResult.UserID,
		Type: basicResult.UserType,
		OUID: basicResult.OUID,
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
			logger.Error("Failed to marshal user attributes")
			return nil, &serviceerror.InternalServerError
		}

		authenticatedUser := &entityprovider.Entity{
			ID:         basicResult.UserID,
			Type:       basicResult.UserType,
			OUID:       basicResult.OUID,
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

// SendOTP sends an OTP to the specified recipient for authentication.
func (as *authenticationService) SendOTP(ctx context.Context, senderID string, channel notifcommon.ChannelType,
	recipient string) (string, *serviceerror.ServiceError) {
	return as.otpService.SendOTP(ctx, senderID, channel, recipient)
}

// VerifyOTP verifies an OTP and returns the authenticated user.
func (as *authenticationService) VerifyOTP(ctx context.Context, sessionToken string, skipAssertion bool,
	existingAssertion, otpCode string) (*common.AuthenticationResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug("Verifying OTP for authentication")

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": sessionToken,
			"otp":          otpCode,
		},
	}
	_, basicResult, svcErr := as.authnProvider.AuthenticateUser(
		ctx, nil, credentials, nil, nil, authnprovidermgr.AuthUser{})
	if svcErr != nil {
		if svcErr.Type == serviceerror.ServerErrorType {
			return nil, &serviceerror.InternalServerError
		}
		if svcErr.Code == authnprovidermgr.ErrorAuthenticationFailed.Code {
			return nil, &ErrorOTPAuthenticationFailed
		}
		return nil, svcErr
	}

	authResponse := &common.AuthenticationResponse{
		ID:   basicResult.UserID,
		Type: basicResult.UserType,
		OUID: basicResult.OUID,
	}

	// Generate assertion if not skipped
	if !skipAssertion {
		userForAssertion := &entityprovider.Entity{
			ID:         basicResult.UserID,
			Type:       basicResult.UserType,
			OUID:       basicResult.OUID,
			Attributes: nil, // Attributes not needed for assertion generation in OTP flow
		}
		svcErr = as.validateAndAppendAuthAssertion(ctx, authResponse, userForAssertion, common.AuthenticatorSMSOTP,
			existingAssertion, logger)
		if svcErr != nil {
			return nil, svcErr
		}
	}

	return authResponse, nil
}

// StartIDPAuthentication initiates authentication against an IDP.
func (as *authenticationService) StartIDPAuthentication(ctx context.Context, requestedType idp.IDPType, idpID string) (
	*IDPAuthInitData, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug("Starting IDP authentication", log.String("idpId", idpID))

	if strings.TrimSpace(idpID) == "" {
		return nil, &common.ErrorInvalidIDPID
	}

	identityProvider, svcErr := as.idpService.GetIdentityProvider(ctx, idpID)
	if svcErr != nil {
		return nil, as.handleIDPServiceError(idpID, svcErr, logger)
	}

	if svcErr := as.validateIDPType(requestedType, identityProvider.Type, logger); svcErr != nil {
		return nil, svcErr
	}

	// Route to appropriate service based on IDP type
	var redirectURL string
	var buildURLErr *serviceerror.ServiceError
	switch identityProvider.Type {
	case idp.IDPTypeOAuth:
		redirectURL, buildURLErr = as.oauthService.BuildAuthorizeURL(ctx, idpID)
	case idp.IDPTypeOIDC:
		redirectURL, buildURLErr = as.oidcService.BuildAuthorizeURL(ctx, idpID)
	case idp.IDPTypeGoogle:
		redirectURL, buildURLErr = as.googleService.BuildAuthorizeURL(ctx, idpID)
	case idp.IDPTypeGitHub:
		redirectURL, buildURLErr = as.githubService.BuildAuthorizeURL(ctx, idpID)
	default:
		logger.Error("Unsupported IDP type", log.String("idpId", idpID),
			log.String("type", string(identityProvider.Type)))
		return nil, &serviceerror.InternalServerError
	}

	if buildURLErr != nil {
		return nil, buildURLErr
	}

	// Generate session token
	sessionToken, err := as.createSessionToken(ctx, idpID, identityProvider.Type)
	if err != nil {
		logger.Error("Failed to create session token", log.String("idpId", idpID),
			log.String("error", err.Error.DefaultValue))
		return nil, &serviceerror.InternalServerError
	}

	return &IDPAuthInitData{
		RedirectURL:  redirectURL,
		SessionToken: sessionToken,
	}, nil
}

// FinishIDPAuthentication completes authentication against an IDP.
func (as *authenticationService) FinishIDPAuthentication(ctx context.Context, requestedType idp.IDPType,
	sessionToken string, skipAssertion bool, existingAssertion, code string) (
	*common.AuthenticationResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug("Finishing IDP authentication")

	if strings.TrimSpace(sessionToken) == "" {
		return nil, &common.ErrorEmptySessionToken
	}
	if strings.TrimSpace(code) == "" {
		return nil, &common.ErrorEmptyAuthCode
	}

	// Verify and decode session token
	sessionData, svcErr := as.verifyAndDecodeSessionToken(sessionToken, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	if svcErr := as.validateIDPType(requestedType, sessionData.IDPType, logger); svcErr != nil {
		return nil, svcErr
	}

	credentials := map[string]interface{}{
		"federated": &common.FederatedAuthCredential{
			IDPID:   sessionData.IDPID,
			IDPType: sessionData.IDPType,
			Code:    code,
		},
	}
	_, basicResult, svcErr := as.authnProvider.AuthenticateUser(
		ctx, nil, credentials, nil, nil, authnprovidermgr.AuthUser{})
	if svcErr != nil {
		return nil, as.mapFederatedAuthnError(svcErr, logger)
	}
	if basicResult == nil {
		logger.Error("Federated authenticate response is nil")
		return nil, &serviceerror.InternalServerError
	}
	if basicResult.IsAmbiguousUser {
		return nil, &common.ErrorUserNotFound
	}
	if !basicResult.IsExistingUser {
		return nil, &common.ErrorUserNotFound
	}

	user := &entityprovider.Entity{
		ID:   basicResult.UserID,
		Type: basicResult.UserType,
		OUID: basicResult.OUID,
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
			logger.Error("Failed to get authenticator name for IDP type",
				log.String("idpType", string(sessionData.IDPType)), log.Error(err))
			return nil, &serviceerror.InternalServerError
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
	ctx context.Context, authResponse *common.AuthenticationResponse, user *entityprovider.Entity, authenticator string,
	existingAssertion string, logger *log.Logger,
) *serviceerror.ServiceError {
	logger.Debug("Generating auth assertion", log.MaskedString(log.LoggerKeyUserID, user.ID))

	authenticatorRef := &common.AuthenticatorReference{
		Authenticator: authenticator,
		Timestamp:     time.Now().Unix(),
	}

	// Extract existing assurance if provided and set appropriate step number
	var existingAssurance *assert.AssuranceContext
	if strings.TrimSpace(existingAssertion) != "" {
		var assertionSub string
		var svcErr *serviceerror.ServiceError
		existingAssurance, assertionSub, svcErr = as.extractClaimsFromAssertion(existingAssertion, logger)
		if svcErr != nil {
			return svcErr
		}

		// Validate that the assertion subject matches the current user
		if assertionSub != user.ID {
			logger.Debug("Assertion subject mismatch", log.MaskedString("assertionSub", assertionSub),
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
	assertionResult, svcErr := as.getAssertionResult(existingAssurance, authenticatorRef)
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
		logger.Error("Failed to generate auth assertion", log.String("error", err.Error.DefaultValue))
		return &serviceerror.InternalServerError
	}

	authResponse.Assertion = token
	return nil
}

// getAssertionResult generates or updates an assertion result based on existing context.
func (as *authenticationService) getAssertionResult(existingContext *assert.AssuranceContext,
	newAuthenticator *common.AuthenticatorReference) (
	*assert.AssertionResult, *serviceerror.ServiceError) {
	var assertionResult *assert.AssertionResult
	var svcErr *serviceerror.ServiceError
	if existingContext != nil && newAuthenticator != nil {
		// Update existing assurance with new authenticator
		assertionResult, svcErr = as.authAssertionGenerator.UpdateAssertion(
			existingContext, *newAuthenticator)
	} else if newAuthenticator != nil {
		// Generate new assurance from authenticator
		assertionResult, svcErr = as.authAssertionGenerator.GenerateAssertion(
			[]common.AuthenticatorReference{*newAuthenticator})
	}

	return assertionResult, svcErr
}

// extractClaimsFromAssertion extracts assurance context and subject from an existing JWT assertion.
func (as *authenticationService) extractClaimsFromAssertion(assertion string,
	logger *log.Logger) (*assert.AssuranceContext, string, *serviceerror.ServiceError) {
	jwtConfig := config.GetServerRuntime().Config.JWT

	if err := as.jwtService.VerifyJWT(assertion, "", jwtConfig.Issuer); err != nil {
		logger.Debug("Failed to verify JWT signature of the assertion", log.String("error", err.Error.DefaultValue))
		return nil, "", &common.ErrorInvalidAssertion
	}

	payload, err := jwt.DecodeJWTPayload(assertion)
	if err != nil {
		logger.Debug("Failed to decode JWT assertion", log.Error(err))
		return nil, "", &common.ErrorInvalidAssertion
	}

	// Extract subject claim
	subClaim, ok := payload["sub"]
	if !ok {
		logger.Debug("No 'sub' claim found in JWT assertion")
		return nil, "", &common.ErrorInvalidAssertion
	}
	sub, ok := subClaim.(string)
	if !ok || strings.TrimSpace(sub) == "" {
		logger.Debug("Invalid 'sub' claim in JWT assertion")
		return nil, "", &common.ErrorInvalidAssertion
	}

	// Extract assurance claim
	assuranceClaim, ok := payload["assurance"]
	if !ok {
		logger.Debug("No assurance claim found in JWT assertion")
		return nil, "", &common.ErrorInvalidAssertion
	}

	// Convert assurance claim to AssuranceContext
	assuranceBytes, err := json.Marshal(assuranceClaim)
	if err != nil {
		logger.Debug("Failed to marshal assurance claim", log.Error(err))
		return nil, "", &common.ErrorInvalidAssertion
	}

	var assuranceCtx assert.AssuranceContext
	if err := json.Unmarshal(assuranceBytes, &assuranceCtx); err != nil {
		logger.Debug("Failed to unmarshal assurance claim to AssuranceContext", log.Error(err))
		return nil, "", &common.ErrorInvalidAssertion
	}

	return &assuranceCtx, sub, nil
}

// mapFederatedAuthnError maps provider manager errors to federated-authentication-specific service errors.
func (as *authenticationService) mapFederatedAuthnError(svcErr *serviceerror.ServiceError,
	logger *log.Logger) *serviceerror.ServiceError {
	switch svcErr.Code {
	case authnprovidermgr.ErrorAuthenticationFailed.Code:
		return &ErrorFederatedAuthenticationFailed
	case authnprovidermgr.ErrorUserNotFound.Code:
		return &ErrorFederatedAuthenticationFailed
	case authnprovidermgr.ErrorInvalidRequest.Code:
		return &ErrorFederatedAuthenticationFailed
	default:
		logger.Error("Error occurred while performing federated authentication",
			log.String("errorCode", svcErr.Code), log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return &serviceerror.InternalServerError
	}
}

// mapCredentialsAuthnError maps provider manager errors to credentials-specific service errors.
func (as *authenticationService) mapCredentialsAuthnError(svcErr *serviceerror.ServiceError,
	logger *log.Logger) *serviceerror.ServiceError {
	switch svcErr.Code {
	case authnprovidermgr.ErrorAuthenticationFailed.Code:
		return &ErrorInvalidCredentials
	case authnprovidermgr.ErrorUserNotFound.Code:
		return &common.ErrorUserNotFound
	case authnprovidermgr.ErrorInvalidRequest.Code:
		return &ErrorEmptyAttributesOrCredentials
	default:
		logger.Error("Error occurred while authenticating with credentials",
			log.String("errorCode", svcErr.Code), log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return &serviceerror.InternalServerError
	}
}

// mapCredentialsGetAttributesError maps provider manager errors from GetUserAttributes to credentials-specific errors.
func (as *authenticationService) mapCredentialsGetAttributesError(svcErr *serviceerror.ServiceError,
	logger *log.Logger) *serviceerror.ServiceError {
	switch svcErr.Code {
	case authnprovidermgr.ErrorGetAttributesClientError.Code:
		return &ErrorInvalidToken
	default:
		logger.Error("Error occurred while getting attributes for credentials authentication",
			log.String("errorCode", svcErr.Code), log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return &serviceerror.InternalServerError
	}
}

// handleIDPServiceError handles errors from IDP service.
func (as *authenticationService) handleIDPServiceError(idpID string, svcErr *serviceerror.ServiceError,
	logger *log.Logger) *serviceerror.ServiceError {
	if svcErr.Type == serviceerror.ClientErrorType {
		errDesc := fmt.Sprintf(
			"An error occurred while retrieving the identity provider with ID %s: %s",
			idpID,
			svcErr.ErrorDescription.DefaultValue,
		)
		return serviceerror.CustomServiceError(common.ErrorClientErrorWhileRetrievingIDP, core.I18nMessage{
			Key:          "error.authnservice.error_retrieving_idp_description",
			DefaultValue: errDesc,
		})
	}

	logger.Error("Error occurred while retrieving IDP", log.String("idpId", idpID), log.Any("error", svcErr))
	return &serviceerror.InternalServerError
}

// validateIDPType validates that the requested IDP type matches the actual IDP type.
func (as *authenticationService) validateIDPType(requestedType, actualType idp.IDPType,
	logger *log.Logger) *serviceerror.ServiceError {
	if requestedType != "" && requestedType != actualType {
		// Allow cross-type authentication for certain types
		if slices.Contains(crossAllowedIDPTypes, requestedType) &&
			slices.Contains(crossAllowedIDPTypes, actualType) {
			return nil
		}

		logger.Debug("IDP type mismatch", log.String("requested", string(requestedType)),
			log.String("actual", string(actualType)))
		return &common.ErrorInvalidIDPType
	}

	return nil
}

// createSessionToken creates a JWT session token with authentication session data.
func (as *authenticationService) createSessionToken(ctx context.Context, idpID string, idpType idp.IDPType) (
	string, *serviceerror.ServiceError) {
	sessionData := AuthSessionData{
		IDPID:   idpID,
		IDPType: idpType,
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
func (as *authenticationService) verifyAndDecodeSessionToken(token string, logger *log.Logger) (
	*AuthSessionData, *serviceerror.ServiceError) {
	// Verify JWT signature and claims
	jwtConfig := config.GetServerRuntime().Config.JWT
	svcErr := as.jwtService.VerifyJWT(token, "auth-svc", jwtConfig.Issuer)
	if svcErr != nil {
		logger.Debug("Error verifying session token", log.String("error", svcErr.Error.DefaultValue))
		return nil, &common.ErrorInvalidSessionToken
	}

	// Parse and extract authentication session data
	payload, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		logger.Debug("Error decoding session token payload", log.Error(err))
		return nil, &common.ErrorInvalidSessionToken
	}

	authDataClaim, ok := payload["auth_data"]
	if !ok {
		logger.Debug("auth_data claim not found in session token")
		return nil, &common.ErrorInvalidSessionToken
	}

	authDataBytes, err := json.Marshal(authDataClaim)
	if err != nil {
		logger.Debug("Error marshaling auth_data claim", log.Error(err))
		return nil, &common.ErrorInvalidSessionToken
	}

	var sessionData AuthSessionData
	err = json.Unmarshal(authDataBytes, &sessionData)
	if err != nil {
		logger.Debug("Error marshaling auth_data claim", log.Error(err))
		return nil, &common.ErrorInvalidSessionToken
	}

	return &sessionData, nil
}

// StartPasskeyRegistration starts the passkey registration process.
func (as *authenticationService) StartPasskeyRegistration(
	ctx context.Context, userID, relyingPartyID, relyingPartyName string,
	authSelection *PasskeyAuthenticatorSelectionDTO, attestation string,
) (interface{}, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug("Starting Passkey registration")

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

	return as.passkeyService.StartRegistration(ctx, req)
}

// FinishPasskeyRegistration completes the passkey registration process.
func (as *authenticationService) FinishPasskeyRegistration(
	ctx context.Context, credential PasskeyPublicKeyCredentialDTO,
	sessionToken, credentialName string,
) (interface{}, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug("Finishing Passkey registration")

	req := &passkey.PasskeyRegistrationFinishRequest{
		CredentialID:      credential.ID,
		CredentialType:    credential.Type,
		ClientDataJSON:    credential.Response.ClientDataJSON,
		AttestationObject: credential.Response.AttestationObject,
		SessionToken:      sessionToken,
		CredentialName:    credentialName,
	}

	return as.passkeyService.FinishRegistration(ctx, req)
}

// StartPasskeyAuthentication starts the passkey authentication process.
func (as *authenticationService) StartPasskeyAuthentication(ctx context.Context, userID, relyingPartyID string) (
	interface{}, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug("Starting Passkey authentication")

	req := &passkey.PasskeyAuthenticationStartRequest{
		UserID:         userID,
		RelyingPartyID: relyingPartyID,
	}
	return as.passkeyService.StartAuthentication(ctx, req)
}

// FinishPasskeyAuthentication completes the passkey authentication process.
func (as *authenticationService) FinishPasskeyAuthentication(ctx context.Context, credentialID, credentialType string,
	response PasskeyCredentialResponseDTO, sessionToken string, skipAssertion bool,
	existingAssertion string) (*common.AuthenticationResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, svcLoggerComponentName))
	logger.Debug("Finishing Passkey authentication")

	passkeyCredential := &passkey.PasskeyAuthenticationFinishRequest{
		CredentialID:      credentialID,
		CredentialType:    credentialType,
		ClientDataJSON:    response.ClientDataJSON,
		AuthenticatorData: response.AuthenticatorData,
		Signature:         response.Signature,
		UserHandle:        response.UserHandle,
		SessionToken:      sessionToken,
	}
	credentials := map[string]interface{}{"passkey": passkeyCredential}
	_, basicResult, svcErr := as.authnProvider.AuthenticateUser(
		ctx, nil, credentials, nil, nil, authnprovidermgr.AuthUser{})
	if svcErr != nil {
		if svcErr.Type == serviceerror.ServerErrorType {
			return nil, &serviceerror.InternalServerError
		}
		if svcErr.Code == authnprovidermgr.ErrorAuthenticationFailed.Code {
			return nil, &ErrorPasskeyAuthenticationFailed
		}
		return nil, svcErr
	}

	authResponse := &common.AuthenticationResponse{
		ID:   basicResult.UserID,
		Type: basicResult.UserType,
		OUID: basicResult.OUID,
	}

	// Generate assertion if not skipped
	if !skipAssertion {
		// Create entity object from authResponse for assertion generation
		userForAssertion := &entityprovider.Entity{
			ID:   basicResult.UserID,
			Type: basicResult.UserType,
			OUID: basicResult.OUID,
		}

		svcErr = as.validateAndAppendAuthAssertion(ctx, authResponse, userForAssertion, common.AuthenticatorPasskey,
			existingAssertion, logger)
		if svcErr != nil {
			return nil, svcErr
		}
	}

	return authResponse, nil
}
