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

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
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
	ctx context.Context,
	identifiers, credentials map[string]interface{},
	metadata *authnprovidercm.AuthnMetadata,
) (*authnprovidercm.AuthnResult, *serviceerror.ServiceError) {
	if credentials == nil {
		return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
			"Credentials are required", "Credentials are required for authentication")
	}

	authOutcome, svcErr := p.resolveCredentials(ctx, identifiers, credentials)
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

	attributesResponse := &authnprovidercm.AttributesResponse{
		Attributes:    make(map[string]*authnprovidercm.AttributeResponse),
		Verifications: make(map[string]*authnprovidercm.VerificationResponse),
	}
	for k := range attributes {
		attributesResponse.Attributes[k] = &authnprovidercm.AttributeResponse{
			AssuranceMetadataResponse: &authnprovidercm.AssuranceMetadataResponse{
				IsVerified: false,
			},
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
		IsAttributeValuesIncluded: false,
		AttributesResponse:        attributesResponse,
		IsExistingUser:            true,
		ExternalSub:               authOutcome.externalSub,
		ExternalClaims:            authOutcome.externalClaims,
	}, nil
}

type credentialOutcome struct {
	entityID       string
	externalSub    string
	externalClaims map[string]interface{}
	earlyReturn    *authnprovidercm.AuthnResult
}

func (p *defaultAuthnProvider) resolveCredentials(
	ctx context.Context,
	identifiers, credentials map[string]interface{},
) (*credentialOutcome, *serviceerror.ServiceError) {
	if passkeyCredential, ok := credentials["passkey"]; ok {
		return p.authenticateWithPasskey(ctx, passkeyCredential)
	}
	if otpCredential, ok := credentials["otp"]; ok {
		return p.authenticateWithOTP(ctx, otpCredential)
	}
	if fedCred, ok := credentials["federated"]; ok {
		return p.authenticateWithFederated(ctx, fedCred)
	}
	if userID, ok := identifiers["userID"]; ok && userID != "" {
		return p.authenticateByUserID(ctx, userID, credentials)
	}
	return p.authenticateByIdentifiers(ctx, identifiers, credentials)
}

func (p *defaultAuthnProvider) authenticateWithPasskey(
	ctx context.Context, raw interface{},
) (*credentialOutcome, *serviceerror.ServiceError) {
	cred, ok := raw.(*passkey.PasskeyAuthenticationFinishRequest)
	if !ok || cred == nil {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid passkey payload", "The provided passkey credential is invalid")
	}
	authResponse, authErr := p.passkeyService.FinishAuthentication(ctx, cred)
	if authErr != nil {
		return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
			authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
	}
	return &credentialOutcome{entityID: authResponse.ID}, nil
}

func (p *defaultAuthnProvider) authenticateWithOTP(
	ctx context.Context, raw interface{},
) (*credentialOutcome, *serviceerror.ServiceError) {
	otpCredential, ok := raw.(map[string]interface{})
	if !ok {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid OTP payload", "The provided OTP credential is invalid")
	}
	sessionToken, ok := otpCredential["sessionToken"].(string)
	if !ok || sessionToken == "" {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid OTP payload", "sessionToken is required")
	}
	otpValue, ok := otpCredential["otp"].(string)
	if !ok || otpValue == "" {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid OTP payload", "otp is required")
	}
	authResponse, authErr := p.otpService.Authenticate(ctx, sessionToken, otpValue)
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
	return &credentialOutcome{entityID: authResponse.ID}, nil
}

func (p *defaultAuthnProvider) authenticateWithFederated(
	ctx context.Context, raw interface{},
) (*credentialOutcome, *serviceerror.ServiceError) {
	cred, ok := raw.(*authncommon.FederatedAuthCredential)
	if !ok || cred == nil {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid federated credential payload", "The provided federated credential is invalid")
	}
	if cred.IDPID == "" {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Missing IDP ID", "The federated credential must include a non-empty IDP ID")
	}
	if cred.Code == "" {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Missing authorization code", "The federated credential must include a non-empty authorization code")
	}
	svc, ok := p.federatedAuths[cred.IDPType]
	if !ok {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Unsupported IDP type", "The provided IDP type is not supported for federated authentication")
	}
	authResult, authErr := svc.Authenticate(ctx, cred.IDPID, cred.Code)
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
			earlyReturn: &authnprovidercm.AuthnResult{
				ExternalSub:     authResult.Sub,
				ExternalClaims:  authResult.Claims,
				IsExistingUser:  false,
				IsAmbiguousUser: authResult.IsAmbiguousUser,
			},
		}, nil
	}
	return &credentialOutcome{
		entityID:       authResult.InternalEntity.ID,
		externalSub:    authResult.Sub,
		externalClaims: authResult.Claims,
	}, nil
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
	return &credentialOutcome{entityID: authResult.EntityID}, nil
}

func (p *defaultAuthnProvider) authenticateByIdentifiers(
	ctx context.Context, identifiers, credentials map[string]interface{},
) (*credentialOutcome, *serviceerror.ServiceError) {
	authResult, authErr := p.entitySvc.AuthenticateEntity(ctx, identifiers, credentials)
	if authErr != nil {
		return nil, p.handleEntityAuthError(authErr, "Basic authentication failed with server error")
	}
	return &credentialOutcome{entityID: authResult.EntityID}, nil
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
