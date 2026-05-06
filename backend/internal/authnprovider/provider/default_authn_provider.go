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

// Package provider provides authentication provider implementations.
package provider

import (
	"context"
	"encoding/json"
	"errors"

	authncommon "github.com/asgardeo/thunder/internal/authn/common"
	"github.com/asgardeo/thunder/internal/authn/otp"
	"github.com/asgardeo/thunder/internal/authn/passkey"
	authnprovidercm "github.com/asgardeo/thunder/internal/authnprovider/common"
	"github.com/asgardeo/thunder/internal/entity"
	"github.com/asgardeo/thunder/internal/idp"
	"github.com/asgardeo/thunder/internal/system/error/serviceerror"
	"github.com/asgardeo/thunder/internal/system/i18n/core"
	"github.com/asgardeo/thunder/internal/system/log"
)

type defaultAuthnProvider struct {
	entitySvc      entity.EntityServiceInterface
	passkeyService passkey.PasskeyServiceInterface
	otpService     otp.OTPAuthnServiceInterface
	federatedAuths map[idp.IDPType]authncommon.FederatedAuthenticator
	logger         *log.Logger
}

// newDefaultAuthnProvider creates a new internal user authn provider.
func newDefaultAuthnProvider(entitySvc entity.EntityServiceInterface,
	passkeyService passkey.PasskeyServiceInterface, otpService otp.OTPAuthnServiceInterface,
	federatedAuths map[idp.IDPType]authncommon.FederatedAuthenticator) AuthnProviderInterface {
	return &defaultAuthnProvider{
		entitySvc:      entitySvc,
		passkeyService: passkeyService,
		otpService:     otpService,
		federatedAuths: federatedAuths,
		logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DefaultAuthnProvider")),
	}
}

// Authenticate authenticates the user using the internal entity service.
func (p *defaultAuthnProvider) Authenticate(
	ctx context.Context, authType string, authnData any,
	metadata *authnprovidercm.AuthnMetadata,
) (*authnprovidercm.AuthnResult, *serviceerror.ServiceError) {
	if authnData == nil {
		return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
			"Credentials are required", "Credentials are required for authentication")
	}

	authOutcome, svcErr := p.resolveCredentials(ctx, authType, authnData)
	if svcErr != nil {
		return nil, svcErr
	}
	if authOutcome.earlyReturn != nil {
		return authOutcome.earlyReturn, nil
	}

	entityResult, getErr := p.entitySvc.GetEntity(ctx, authOutcome.entityID)
	if getErr != nil {
		if errors.Is(getErr, entity.ErrEntityNotFound) {
			return nil, newClientError(authnprovidercm.ErrorCodeUserNotFound,
				"User not found", "The specified user does not exist")
		}
		return nil, p.logAndReturnServerError("Failed to get entity after authentication",
			log.String("error", getErr.Error()))
	}

	var attributes map[string]interface{}
	if len(entityResult.Attributes) > 0 {
		if err := json.Unmarshal(entityResult.Attributes, &attributes); err != nil {
			return nil, p.logAndReturnServerError("Failed to get allowed attributes", log.String("error", err.Error()))
		}
	}

	return &authnprovidercm.AuthnResult{
		EntityID:                  authOutcome.entityID,
		EntityCategory:            string(entityResult.Category),
		EntityType:                entityResult.Type,
		OUID:                      entityResult.OUID,
		UserID:                    authOutcome.entityID,
		Token:                     authOutcome.entityID,
		UserType:                  entityResult.Type,
		IsAttributeValuesIncluded: true,
		AttributesResponse:        buildAttributesResponse(attributes),
		IsExistingUser:            true,
		ExternalSub:               authOutcome.externalSub,
		ExternalClaims:            authOutcome.externalClaims,
		AuthType:                  authOutcome.credentialType,
	}, nil
}

type credentialOutcome struct {
	entityID       string
	externalSub    string
	externalClaims map[string]interface{}
	earlyReturn    *authnprovidercm.AuthnResult
	credentialType string
}

func (p *defaultAuthnProvider) resolveCredentials(
	ctx context.Context, authType string, authnData any,
) (*credentialOutcome, *serviceerror.ServiceError) {
	if authType == authnprovidercm.AuthnDataTypePasskey {
		return p.authenticateWithPasskey(ctx, authnData)
	}
	if authType == authnprovidercm.AuthnDataTypeOTP {
		return p.authenticateWithOTP(ctx, authnData)
	}
	if authType == authnprovidercm.AuthnDataTypeFederated {
		return p.authenticateWithFederated(ctx, authnData)
	}
	if authType == authnprovidercm.AuthnDataTypeCredentials {
		return p.authenticateWithCredentials(ctx, authnData)
	}
	return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
		"Unsupported authentication type", "The provided authentication type is not supported")
}

func (p *defaultAuthnProvider) authenticateWithPasskey(
	ctx context.Context, authnData any,
) (*credentialOutcome, *serviceerror.ServiceError) {
	passkeyAuthnData, ok := authnData.(*authnprovidercm.PasskeyAuthnData)
	if !ok {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid passkey payload", "The provided passkey credential is invalid")
	}
	cred := &passkey.PasskeyAuthenticationFinishRequest{
		CredentialID:      passkeyAuthnData.CredentialID,
		CredentialType:    passkeyAuthnData.CredentialType,
		ClientDataJSON:    passkeyAuthnData.ClientDataJSON,
		AuthenticatorData: passkeyAuthnData.AuthenticatorData,
		Signature:         passkeyAuthnData.Signature,
		UserHandle:        passkeyAuthnData.UserHandle,
		SessionToken:      passkeyAuthnData.SessionToken,
	}
	authResponse, authErr := p.passkeyService.FinishAuthentication(ctx, cred)
	if authErr != nil {
		return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
			authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
	}
	return &credentialOutcome{entityID: authResponse.ID, credentialType: authnprovidercm.AuthenticatorPasskey}, nil
}

func (p *defaultAuthnProvider) authenticateWithOTP(
	ctx context.Context, authnData any,
) (*credentialOutcome, *serviceerror.ServiceError) {
	otpAuthnData, ok := authnData.(*authnprovidercm.OTPAuthnData)
	if !ok {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid OTP payload", "The provided OTP credential is invalid")
	}
	authResult, authErr := p.otpService.Authenticate(ctx, otpAuthnData.SessionToken, otpAuthnData.OTP)
	if authErr != nil {
		if authErr.Type == serviceerror.ClientErrorType {
			if authErr.Code == otp.ErrorIncorrectOTP.Code {
				return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
					authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
			}
			return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
				authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
		}
		return nil, p.logAndReturnServerError("OTP authentication failed with server error",
			log.String("error", authErr.Error.DefaultValue),
			log.String("errorDescription", authErr.ErrorDescription.DefaultValue))
	}
	if authResult.InternalEntity == nil {
		return &credentialOutcome{
			credentialType: authnprovidercm.AuthenticatorSMSOTP,
			earlyReturn: &authnprovidercm.AuthnResult{
				IsExistingUser:            false,
				IsAttributeValuesIncluded: true,
				AttributesResponse:        buildAttributesResponse(authResult.VerifiedIdentifiers),
			},
		}, nil
	}
	return &credentialOutcome{entityID: authResult.InternalEntity.ID,
		credentialType: authnprovidercm.AuthenticatorSMSOTP}, nil
}

func (p *defaultAuthnProvider) authenticateWithFederated(
	ctx context.Context, authnData any,
) (*credentialOutcome, *serviceerror.ServiceError) {
	fedAuthnData, ok := authnData.(*authnprovidercm.FederatedAuthnData)
	if !ok {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid federated credential payload", "The provided federated credential is invalid")
	}
	svc, ok := p.federatedAuths[fedAuthnData.IDPType]
	if !ok {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Unsupported IDP type", "The provided IDP type is not supported for federated authentication")
	}
	authResult, authErr := svc.Authenticate(ctx, fedAuthnData.IDPID, fedAuthnData.OAuthCredential.Code)
	if authErr != nil {
		if authErr.Type == serviceerror.ClientErrorType {
			return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
				authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
		}
		return nil, p.logAndReturnServerError("Federated authentication failed with server error",
			log.String("error", authErr.Error.DefaultValue),
			log.String("errorDescription", authErr.ErrorDescription.DefaultValue))
	}
	if authResult.InternalEntity == nil {
		return &credentialOutcome{
			credentialType: getFederatedAuthenticatorName(fedAuthnData.IDPType),
			earlyReturn: &authnprovidercm.AuthnResult{
				ExternalSub:               authResult.Sub,
				ExternalClaims:            authResult.Claims,
				IsExistingUser:            false,
				IsAmbiguousUser:           authResult.IsAmbiguousUser,
				IsAttributeValuesIncluded: true,
				AttributesResponse:        buildAttributesResponse(authResult.Claims),
			},
		}, nil
	}
	return &credentialOutcome{
		credentialType: getFederatedAuthenticatorName(fedAuthnData.IDPType),
		entityID:       authResult.InternalEntity.ID,
		externalSub:    authResult.Sub,
		externalClaims: authResult.Claims,
	}, nil
}

func (p *defaultAuthnProvider) authenticateWithCredentials(
	ctx context.Context, authnData any,
) (*credentialOutcome, *serviceerror.ServiceError) {
	credsAuthnData, ok := authnData.(*authnprovidercm.CredentialsAuthnData)
	if !ok {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid credential payload", "The provided credential is invalid")
	}

	if userID, ok := credsAuthnData.Identifiers["userID"]; ok && userID != "" {
		return p.authenticateByUserID(ctx, userID, credsAuthnData.Credentials)
	}
	return p.authenticateByIdentifiers(ctx, credsAuthnData)
}

func (p *defaultAuthnProvider) authenticateByUserID(
	ctx context.Context, userID interface{}, credentials map[string]interface{},
) (*credentialOutcome, *serviceerror.ServiceError) {
	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid user ID", "The provided userID is invalid")
	}
	authResult, authErr := p.entitySvc.AuthenticateEntityByID(ctx, userIDStr, credentials)
	if authErr != nil {
		return nil, p.handleEntityAuthError(authErr, "Basic authentication by ID failed with server error")
	}
	return &credentialOutcome{entityID: authResult.EntityID, credentialType: authnprovidercm.AuthenticatorCredentials},
		nil
}

func (p *defaultAuthnProvider) authenticateByIdentifiers(
	ctx context.Context, authnData *authnprovidercm.CredentialsAuthnData,
) (*credentialOutcome, *serviceerror.ServiceError) {
	authResult, authErr := p.entitySvc.AuthenticateEntity(ctx, authnData.Identifiers, authnData.Credentials)
	if authErr != nil {
		return nil, p.handleEntityAuthError(authErr, "Basic authentication failed with server error")
	}
	return &credentialOutcome{entityID: authResult.EntityID, credentialType: authnprovidercm.AuthenticatorCredentials},
		nil
}

func (p *defaultAuthnProvider) handleEntityAuthError(err error, serverMsg string) *serviceerror.ServiceError {
	if errors.Is(err, entity.ErrEntityNotFound) {
		return newClientError(authnprovidercm.ErrorCodeUserNotFound,
			"User not found", "The specified user does not exist")
	}
	if errors.Is(err, entity.ErrAuthenticationFailed) {
		return newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
			"Authentication failed", "Invalid credentials provided")
	}
	return p.logAndReturnServerError(serverMsg, log.String("error", err.Error()))
}

// GetAttributes retrieves the user attributes using the internal entity service.
func (p *defaultAuthnProvider) GetAttributes(
	ctx context.Context,
	token string,
	requestedAttributes *authnprovidercm.RequestedAttributes,
	metadata *authnprovidercm.GetAttributesMetadata,
) (*authnprovidercm.GetAttributesResult, *serviceerror.ServiceError) {
	entityID := token

	entityResult, getErr := p.entitySvc.GetEntity(ctx, entityID)
	if getErr != nil {
		if errors.Is(getErr, entity.ErrEntityNotFound) {
			return nil, newClientError(authnprovidercm.ErrorCodeInvalidToken,
				"Invalid token", "The specified token is invalid")
		}
		return nil, p.logAndReturnServerError("Failed to get entity attributes",
			log.String("error", getErr.Error()))
	}

	var allAttributes map[string]interface{}
	if len(entityResult.Attributes) > 0 {
		if err := json.Unmarshal(entityResult.Attributes, &allAttributes); err != nil {
			return nil, p.logAndReturnServerError("Failed to unmarshal entity attributes",
				log.String("error", err.Error()))
		}
	}

	attributesResponse := &authnprovidercm.AttributesResponse{
		Attributes:    make(map[string]*authnprovidercm.AttributeResponse),
		Verifications: make(map[string]*authnprovidercm.VerificationResponse),
	}

	if requestedAttributes != nil && len(requestedAttributes.Attributes) > 0 {
		for attrName := range requestedAttributes.Attributes {
			if val, ok := allAttributes[attrName]; ok {
				attributesResponse.Attributes[attrName] = &authnprovidercm.AttributeResponse{
					Value: val,
					AssuranceMetadataResponse: &authnprovidercm.AssuranceMetadataResponse{
						IsVerified:     false,
						VerificationID: "",
					},
				}
			}
		}
	} else {
		for attrName, val := range allAttributes {
			attributesResponse.Attributes[attrName] = &authnprovidercm.AttributeResponse{
				Value: val,
				AssuranceMetadataResponse: &authnprovidercm.AssuranceMetadataResponse{
					IsVerified:     false,
					VerificationID: "",
				},
			}
		}
	}

	return &authnprovidercm.GetAttributesResult{
		EntityID:           entityResult.ID,
		EntityCategory:     string(entityResult.Category),
		EntityType:         entityResult.Type,
		OUID:               entityResult.OUID,
		UserID:             entityResult.ID,
		UserType:           entityResult.Type,
		AttributesResponse: attributesResponse,
	}, nil
}

// GetAuthenticatorMetadata returns the metadata of the specified authenticator.
func (p *defaultAuthnProvider) GetAuthenticatorMetadata(authenticatorName string) *authnprovidercm.AuthenticatorMeta {
	if authenticatorName == authnprovidercm.AuthenticatorCredentials {
		return &authnprovidercm.AuthenticatorMeta{
			Name:    authnprovidercm.AuthenticatorCredentials,
			Factors: []authnprovidercm.AuthenticationFactor{authnprovidercm.FactorKnowledge},
		}
	}
	if authenticatorName == authnprovidercm.AuthenticatorPasskey {
		return &authnprovidercm.AuthenticatorMeta{
			Name: authnprovidercm.AuthenticatorPasskey,
			Factors: []authnprovidercm.AuthenticationFactor{authnprovidercm.FactorPossession,
				authnprovidercm.FactorInherence},
		}
	}
	if authenticatorName == authnprovidercm.AuthenticatorSMSOTP {
		return &authnprovidercm.AuthenticatorMeta{
			Name:    authnprovidercm.AuthenticatorSMSOTP,
			Factors: []authnprovidercm.AuthenticationFactor{authnprovidercm.FactorPossession},
		}
	}
	if authenticatorName == authnprovidercm.AuthenticatorOAuth {
		return &authnprovidercm.AuthenticatorMeta{
			Name:    authnprovidercm.AuthenticatorOAuth,
			Factors: []authnprovidercm.AuthenticationFactor{authnprovidercm.FactorKnowledge},
		}
	}
	if authenticatorName == authnprovidercm.AuthenticatorOIDC {
		return &authnprovidercm.AuthenticatorMeta{
			Name:    authnprovidercm.AuthenticatorOIDC,
			Factors: []authnprovidercm.AuthenticationFactor{authnprovidercm.FactorKnowledge},
		}
	}
	if authenticatorName == authnprovidercm.AuthenticatorGithub {
		return &authnprovidercm.AuthenticatorMeta{
			Name:    authnprovidercm.AuthenticatorGithub,
			Factors: []authnprovidercm.AuthenticationFactor{authnprovidercm.FactorKnowledge},
		}
	}
	if authenticatorName == authnprovidercm.AuthenticatorGoogle {
		return &authnprovidercm.AuthenticatorMeta{
			Name:    authnprovidercm.AuthenticatorGoogle,
			Factors: []authnprovidercm.AuthenticationFactor{authnprovidercm.FactorKnowledge},
		}
	}
	return nil
}

func buildAttributesResponse(attrs map[string]interface{}) *authnprovidercm.AttributesResponse {
	resp := &authnprovidercm.AttributesResponse{
		Attributes:    make(map[string]*authnprovidercm.AttributeResponse),
		Verifications: make(map[string]*authnprovidercm.VerificationResponse),
	}
	for k, v := range attrs {
		resp.Attributes[k] = &authnprovidercm.AttributeResponse{
			Value: v,
			AssuranceMetadataResponse: &authnprovidercm.AssuranceMetadataResponse{
				IsVerified: false,
			},
		}
	}
	return resp
}

func newClientError(code, msg, desc string) *serviceerror.ServiceError {
	return &serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: code,
		Error: core.I18nMessage{
			Key:          "error.authnproviderservice." + code,
			DefaultValue: msg,
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnproviderservice." + code + "_description",
			DefaultValue: desc,
		},
	}
}

func (p *defaultAuthnProvider) logAndReturnServerError(msg string, fields ...log.Field) *serviceerror.ServiceError {
	p.logger.Error(msg, fields...)
	err := serviceerror.InternalServerError
	return &err
}

func getFederatedAuthenticatorName(idpType idp.IDPType) string {
	switch idpType {
	case idp.IDPTypeGoogle:
		return authnprovidercm.AuthenticatorGoogle
	case idp.IDPTypeGitHub:
		return authnprovidercm.AuthenticatorGithub
	case idp.IDPTypeOIDC:
		return authnprovidercm.AuthenticatorOIDC
	case idp.IDPTypeOAuth:
		return authnprovidercm.AuthenticatorOIDC
	default:
		return ""
	}
}
