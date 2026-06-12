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
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/openid4vp"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// UserAttributeUserID is the attribute key used to identify the user ID.
const UserAttributeUserID = "userID"

type authnResult struct {
	token               map[string]interface{}
	authenticatedClaims map[string]interface{}
}

type defaultAuthnProvider struct {
	entitySvc        entity.EntityServiceInterface
	passkeyService   passkey.PasskeyServiceInterface
	otpService       otp.OTPAuthnServiceInterface
	magicLinkService magiclink.MagicLinkAuthnServiceInterface
	openid4vpService openid4vp.OpenID4VPServiceInterface
	federatedAuths   map[idp.IDPType]authncommon.FederatedAuthenticator
	logger           *log.Logger
}

// newDefaultAuthnProvider creates a new internal user authn provider.
func newDefaultAuthnProvider(entitySvc entity.EntityServiceInterface,
	passkeyService passkey.PasskeyServiceInterface, otpService otp.OTPAuthnServiceInterface,
	magicLinkService magiclink.MagicLinkAuthnServiceInterface,
	openid4vpService openid4vp.OpenID4VPServiceInterface,
	federatedAuths map[idp.IDPType]authncommon.FederatedAuthenticator) AuthnProviderInterface {
	return &defaultAuthnProvider{
		entitySvc:        entitySvc,
		passkeyService:   passkeyService,
		otpService:       otpService,
		magicLinkService: magicLinkService,
		openid4vpService: openid4vpService,
		federatedAuths:   federatedAuths,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DefaultAuthnProvider")),
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

	authnResult, svcErr := p.resolveCredentials(ctx, identifiers, credentials)
	if svcErr != nil {
		return nil, svcErr
	}

	result := &authnprovidercm.AuthnResult{
		AuthenticatedClaims: authnResult.authenticatedClaims,
	}

	entityID := ""
	if idVal, ok := authnResult.token[UserAttributeUserID]; ok {
		entityID, _ = idVal.(string)
	}
	if entityID == "" {
		identifiedEntityID, identifyErr := p.entitySvc.IdentifyEntity(ctx, authnResult.token)
		if identifyErr != nil {
			if errors.Is(identifyErr, entity.ErrEntityNotFound) || errors.Is(identifyErr, entity.ErrAmbiguousEntity) {
				// Entity does not yet exist at provider. Return the entity reference token and attribute token
				// to the caller to retrieve the entity reference and attributes later.
				result.EntityReferenceToken = authnResult.token
				result.AttributeToken = authnResult.token
				return result, nil
			}
			return nil, p.logAndReturnServerError(ctx, "Failed to identify entity after authentication",
				log.String("error", identifyErr.Error()))
		}
		entityID = *identifiedEntityID
	}

	entityResult, getErr := p.entitySvc.GetEntity(ctx, entityID)
	if getErr != nil {
		return nil, p.logAndReturnServerError(ctx, "Failed to get entity after authentication",
			log.String("error", getErr.Error()))
	}
	result.EntityReference = &authnprovidercm.EntityReference{
		EntityID:       entityResult.ID,
		EntityCategory: string(entityResult.Category),
		EntityType:     entityResult.Type,
		OUID:           entityResult.OUID,
	}
	attributes := make(map[string]interface{})
	if len(entityResult.Attributes) > 0 {
		if err := json.Unmarshal(entityResult.Attributes, &attributes); err != nil {
			return nil, p.logAndReturnServerError(ctx, "Failed to get attributes", log.String("error", err.Error()))
		}
	}
	result.Attributes = buildAttributesResponse(attributes)

	return result, nil
}

// GetEntityReference retrieves the entity reference using the internal entity service.
func (p *defaultAuthnProvider) GetEntityReference(ctx context.Context, entityReferenceToken any,
) (*authnprovidercm.EntityReference, *serviceerror.ServiceError) {
	parsedToken, ok := entityReferenceToken.(map[string]interface{})
	if !ok {
		return nil, p.logAndReturnServerError(ctx, "Invalid token format")
	}
	entityID := ""
	if idVal, ok := parsedToken[UserAttributeUserID]; ok {
		entityID, _ = idVal.(string)
	}
	if entityID == "" {
		identifiedEntityID, identifyErr := p.entitySvc.IdentifyEntity(ctx, parsedToken)
		if identifyErr != nil {
			if errors.Is(identifyErr, entity.ErrEntityNotFound) {
				return nil, newClientError(authnprovidercm.ErrorCodeUserNotFound,
					"User not found", "No user found matching the provided entity reference token")
			}
			if errors.Is(identifyErr, entity.ErrAmbiguousEntity) {
				return nil, newClientError(authnprovidercm.ErrorCodeAmbiguousUser,
					"Ambiguous user", "Multiple users found matching the provided entity reference token")
			}
			return nil, p.logAndReturnServerError(ctx, "Failed to identify entity after authentication",
				log.String("error", identifyErr.Error()))
		}
		entityID = *identifiedEntityID
	}

	entityResult, getErr := p.entitySvc.GetEntity(ctx, entityID)
	if getErr != nil {
		return nil, p.logAndReturnServerError(ctx, "Failed to get entity attributes",
			log.String("error", getErr.Error()))
	}

	return &authnprovidercm.EntityReference{
		EntityID:       entityResult.ID,
		EntityCategory: string(entityResult.Category),
		EntityType:     entityResult.Type,
		OUID:           entityResult.OUID,
	}, nil
}

// GetAttributes retrieves the user attributes using the internal entity service.
func (p *defaultAuthnProvider) GetAttributes(
	ctx context.Context,
	attributeToken any,
	consentedAttributes *authnprovidercm.RequestedAttributes,
	metadata *authnprovidercm.GetAttributesMetadata,
) (*authnprovidercm.AttributesResponse, *serviceerror.ServiceError) {
	parsedToken, ok := attributeToken.(map[string]interface{})
	if !ok {
		return nil, p.logAndReturnServerError(ctx, "Invalid token format")
	}
	entityID := ""
	if idVal, ok := parsedToken[UserAttributeUserID]; ok {
		entityID, _ = idVal.(string)
	}
	if entityID == "" {
		identifiedEntityID, identifyErr := p.entitySvc.IdentifyEntity(ctx, parsedToken)
		if identifyErr != nil {
			if errors.Is(identifyErr, entity.ErrEntityNotFound) {
				return nil, newClientError(authnprovidercm.ErrorCodeUserNotFound,
					"User not found", "No user found matching the provided entity reference token")
			}
			if errors.Is(identifyErr, entity.ErrAmbiguousEntity) {
				return nil, newClientError(authnprovidercm.ErrorCodeAmbiguousUser,
					"Ambiguous user", "Multiple users found matching the provided entity reference token")
			}
			return nil, p.logAndReturnServerError(ctx, "Failed to identify entity after authentication",
				log.String("error", identifyErr.Error()))
		}
		entityID = *identifiedEntityID
	}

	entityResult, getErr := p.entitySvc.GetEntity(ctx, entityID)
	if getErr != nil {
		return nil, p.logAndReturnServerError(ctx, "Failed to get entity attributes",
			log.String("error", getErr.Error()))
	}

	var allAttributes map[string]interface{}
	if len(entityResult.Attributes) > 0 {
		if err := json.Unmarshal(entityResult.Attributes, &allAttributes); err != nil {
			return nil, p.logAndReturnServerError(ctx, "Failed to unmarshal entity attributes",
				log.String("error", err.Error()))
		}
	}

	attributesResponse := &authnprovidercm.AttributesResponse{
		Attributes:    make(map[string]*authnprovidercm.AttributeResponse),
		Verifications: make(map[string]*authnprovidercm.VerificationResponse),
	}

	if consentedAttributes != nil && len(consentedAttributes.Attributes) > 0 {
		for attrName := range consentedAttributes.Attributes {
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

	return attributesResponse, nil
}

func (p *defaultAuthnProvider) resolveCredentials(
	ctx context.Context,
	identifiers, credentials map[string]interface{},
) (*authnResult, *serviceerror.ServiceError) {
	if provisioned, ok := credentials["provisionedEntityID"]; ok {
		return p.AuthenticateForProvisioning(ctx, provisioned)
	}
	if passkeyCredential, ok := credentials["passkey"]; ok {
		return p.authenticateWithPasskey(ctx, passkeyCredential)
	}
	if otpCredential, ok := credentials["otp"]; ok {
		return p.authenticateWithOTP(ctx, otpCredential)
	}
	if fedCred, ok := credentials["federated"]; ok {
		return p.authenticateWithFederated(ctx, fedCred)
	}
	if mlCred, ok := credentials["magiclink"]; ok {
		return p.authenticateWithMagicLink(ctx, mlCred)
	}
	if vpCred, ok := credentials["openid4vp"]; ok {
		return p.authenticateWithOpenID4VP(ctx, vpCred)
	}
	if userID, ok := identifiers["userID"]; ok && userID != "" {
		return p.authenticateByUserID(ctx, userID, credentials)
	}
	return p.authenticateByIdentifiers(ctx, identifiers, credentials)
}

func (p *defaultAuthnProvider) AuthenticateForProvisioning(
	ctx context.Context, raw interface{},
) (*authnResult, *serviceerror.ServiceError) {
	userID, ok := raw.(string)
	if !ok || userID == "" {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid provisioning credential payload",
			"The provided provisioning credential must be a non-empty userID string")
	}
	return &authnResult{
		token:               map[string]interface{}{UserAttributeUserID: userID},
		authenticatedClaims: map[string]interface{}{UserAttributeUserID: userID},
	}, nil
}

func (p *defaultAuthnProvider) authenticateWithPasskey(
	ctx context.Context, raw interface{},
) (*authnResult, *serviceerror.ServiceError) {
	cred, ok := raw.(*passkey.PasskeyAuthenticationFinishRequest)
	if !ok || cred == nil {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid passkey payload", "The provided passkey credential is invalid")
	}
	result, authErr := p.passkeyService.FinishAuthentication(ctx, cred)
	if authErr != nil {
		return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
			authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
	}
	return &authnResult{
		token:               result.Token,
		authenticatedClaims: result.AuthenticatedClaims,
	}, nil
}

func (p *defaultAuthnProvider) authenticateWithOTP(
	ctx context.Context, raw interface{},
) (*authnResult, *serviceerror.ServiceError) {
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
	result, authErr := p.otpService.Authenticate(ctx, sessionToken, otpValue)
	if authErr != nil {
		if authErr.Type == serviceerror.ClientErrorType {
			if authErr.Code == otp.ErrorIncorrectOTP.Code {
				return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
					authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
			}
			return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
				authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
		}
		return nil, p.logAndReturnServerError(ctx, "OTP authentication failed with server error",
			log.String("error", authErr.Error.DefaultValue),
			log.String("errorDescription", authErr.ErrorDescription.DefaultValue))
	}
	return &authnResult{
		token:               result.Token,
		authenticatedClaims: result.AuthenticatedClaims,
	}, nil
}

func (p *defaultAuthnProvider) authenticateWithFederated(
	ctx context.Context, raw interface{},
) (*authnResult, *serviceerror.ServiceError) {
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
	result, authErr := svc.Authenticate(ctx, cred.IDPID, cred.Code)
	if authErr != nil {
		if authErr.Type == serviceerror.ClientErrorType {
			return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
				authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
		}
		return nil, p.logAndReturnServerError(ctx, "Federated authentication failed with server error",
			log.String("error", authErr.Error.DefaultValue),
			log.String("errorDescription", authErr.ErrorDescription.DefaultValue))
	}
	return &authnResult{
		token:               result.Token,
		authenticatedClaims: result.AuthenticatedClaims,
	}, nil
}

func (p *defaultAuthnProvider) authenticateWithMagicLink(
	ctx context.Context, raw interface{},
) (*authnResult, *serviceerror.ServiceError) {
	mlCredential, ok := raw.(map[string]interface{})
	if !ok {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid magic link payload", "The provided magic link credential is invalid")
	}
	token, ok := mlCredential["token"].(string)
	if !ok || token == "" {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid magic link payload", "token is required")
	}
	subjectAttribute, _ := mlCredential["subjectAttribute"].(string)

	result, authErr := p.magicLinkService.Authenticate(ctx, token, subjectAttribute)
	if authErr != nil {
		if authErr.Type == serviceerror.ClientErrorType {
			return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
				authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
		}
		return nil, p.logAndReturnServerError(ctx, "Magic link authentication failed with server error",
			log.String("error", authErr.Error.DefaultValue),
			log.String("errorDescription", authErr.ErrorDescription.DefaultValue))
	}
	return &authnResult{
		token:               result.Token,
		authenticatedClaims: result.AuthenticatedClaims,
	}, nil
}

func (p *defaultAuthnProvider) authenticateWithOpenID4VP(
	ctx context.Context, raw interface{},
) (*authnResult, *serviceerror.ServiceError) {
	cred, ok := raw.(*authncommon.OpenID4VPCredential)
	if !ok || cred == nil || cred.State == "" {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid OpenID4VP payload", "The provided OpenID4VP credential is invalid")
	}
	if p.openid4vpService == nil {
		return nil, p.logAndReturnServerError(ctx, "OpenID4VP service is not configured")
	}
	result, svcErr := p.openid4vpService.Authenticate(ctx, cred.State)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
				svcErr.Error.DefaultValue, svcErr.ErrorDescription.DefaultValue)
		}
		return nil, p.logAndReturnServerError(ctx, "OpenID4VP authentication failed with server error",
			log.String("error", svcErr.ErrorDescription.DefaultValue))
	}
	return &authnResult{
		token:               result.Token,
		authenticatedClaims: result.AuthenticatedClaims,
	}, nil
}

func (p *defaultAuthnProvider) authenticateByUserID(
	ctx context.Context, userID interface{}, credentials map[string]interface{},
) (*authnResult, *serviceerror.ServiceError) {
	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid user ID", "The provided userID is invalid")
	}
	result, authErr := p.entitySvc.AuthenticateEntityByID(ctx, userIDStr, credentials)
	if authErr != nil {
		return nil, p.handleEntityAuthError(ctx, authErr, "Basic authentication by ID failed with server error")
	}
	return &authnResult{
		token:               map[string]interface{}{UserAttributeUserID: result.EntityID},
		authenticatedClaims: map[string]interface{}{UserAttributeUserID: result.EntityID},
	}, nil
}

func (p *defaultAuthnProvider) authenticateByIdentifiers(
	ctx context.Context, identifiers, credentials map[string]interface{},
) (*authnResult, *serviceerror.ServiceError) {
	result, authErr := p.entitySvc.AuthenticateEntity(ctx, identifiers, credentials)
	if authErr != nil {
		return nil, p.handleEntityAuthError(ctx, authErr, "Basic authentication failed with server error")
	}
	return &authnResult{
		token:               map[string]interface{}{UserAttributeUserID: result.EntityID},
		authenticatedClaims: map[string]interface{}{UserAttributeUserID: result.EntityID},
	}, nil
}

func (p *defaultAuthnProvider) handleEntityAuthError(
	ctx context.Context, err error, serverMsg string) *serviceerror.ServiceError {
	if errors.Is(err, entity.ErrEntityNotFound) {
		return newClientError(authnprovidercm.ErrorCodeUserNotFound,
			"User not found", "The specified user does not exist")
	}
	if errors.Is(err, entity.ErrAuthenticationFailed) {
		return newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
			"Authentication failed", "Invalid credentials provided")
	}
	return p.logAndReturnServerError(ctx, serverMsg, log.String("error", err.Error()))
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

func (p *defaultAuthnProvider) logAndReturnServerError(
	ctx context.Context, msg string, fields ...log.Field) *serviceerror.ServiceError {
	p.logger.Error(ctx, msg, fields...)
	err := serviceerror.InternalServerError
	return &err
}
