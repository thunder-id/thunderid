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

package manager

import (
	"context"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/authnprovider/provider"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// authnProviderManager is a proxy struct that implements AuthnProviderManagerInterface by delegating
// to an underlying AuthnProviderInterface.
type authnProviderManager struct {
	provider provider.AuthnProviderInterface
	logger   *log.Logger
}

// newAuthnProviderManager creates a new authnProviderManager.
func newAuthnProviderManager(p provider.AuthnProviderInterface) AuthnProviderManagerInterface {
	return &authnProviderManager{
		provider: p,
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthnProviderManager")),
	}
}

// AuthenticateUser authenticates with the underlying provider and returns an updated AuthUser.
func (m *authnProviderManager) AuthenticateUser(ctx context.Context, identifiers, credentials map[string]interface{},
	requestedAttributes *authnprovidercm.RequestedAttributes,
	metadata *authnprovidercm.AuthnMetadata,
	authUser AuthUser) (AuthUser, authnprovidercm.AuthenticatedClaims, *serviceerror.ServiceError) {
	if sub, ok := credentials["sub"]; ok {
		// Temporary handling of disambiguation after a federated authentication step.
		// Only works with Thunder's default authn provider
		if subStr, ok := sub.(string); !ok || subStr == "" {
			m.logger.Debug(ctx, "disambiguation requested but sub is missing or invalid in credentials")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		if !authUser.IsAuthenticated() {
			m.logger.Debug(ctx, "disambiguation requested but current user is not authenticated")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		if authUser.entityReferenceToken == nil {
			m.logger.Debug(ctx, "disambiguation requested but current user's entity reference token is missing")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		entityRefToken, ok := authUser.entityReferenceToken.(map[string]interface{})
		if !ok || entityRefToken == nil {
			m.logger.Debug(ctx,
				"disambiguation requested but current user's entity reference token is missing or invalid")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		subClaim, ok := entityRefToken[authnprovidercm.UserAttributeSub]
		if !ok {
			m.logger.Debug(ctx,
				"disambiguation requested but current user's entity reference token is missing sub claim")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		if systemutils.ConvertInterfaceValueToString(subClaim) !=
			systemutils.ConvertInterfaceValueToString(sub) {
			m.logger.Debug(ctx, "disambiguation requested but sub claim in credentials "+
				"does not match current user's sub claim")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		val, ok := identifiers[authnprovidercm.UserAttributeUserID]
		if !ok {
			m.logger.Debug(ctx, "disambiguation requested but userID is missing or invalid in identifiers")
			return authUser, nil, &ErrorAuthenticationFailed
		}
		valStr, ok := val.(string)
		if !ok || valStr == "" {
			m.logger.Debug(ctx, "disambiguation requested but userID is missing or invalid in identifiers")
			return authUser, nil, &ErrorAuthenticationFailed
		}
		userID := valStr
		authUser = AuthUser{
			entityReferenceToken: map[string]interface{}{
				authnprovidercm.UserAttributeUserID: userID,
			},
			entityReference: nil,
			attributeToken: map[string]interface{}{
				authnprovidercm.UserAttributeUserID: userID,
			},
			attributes: nil,
		}
		return authUser, nil, nil
	}

	authResult, svcErr := m.provider.Authenticate(ctx, identifiers, credentials, metadata)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ServerErrorType {
			m.logger.Error(ctx, "provider returned server error during authentication",
				log.String("error", svcErr.ErrorDescription.DefaultValue))
			return authUser, nil, &serviceerror.InternalServerError
		}
		switch svcErr.Code {
		case authnprovidercm.ErrorCodeUserNotFound:
			m.logger.Debug(ctx, "authentication failed with user not found error from provider",
				log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
			return authUser, nil, &ErrorUserNotFound
		case authnprovidercm.ErrorCodeInvalidRequest:
			m.logger.Debug(ctx, "authentication failed with invalid request error from provider",
				log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
			return authUser, nil, &ErrorInvalidRequest
		default:
			m.logger.Debug(ctx, "authentication failed with client error from provider",
				log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
			return authUser, nil, &ErrorAuthenticationFailed
		}
	}
	if (authResult.AttributeToken == nil && authResult.Attributes == nil) ||
		(authResult.EntityReferenceToken == nil && authResult.EntityReference == nil) {
		m.logger.Error(ctx, "provider Authenticate result is missing both entity reference and attribute values")
		return authUser, nil, &serviceerror.InternalServerError
	}

	if authResult.EntityReferenceToken != nil {
		authUser.entityReferenceToken = authResult.EntityReferenceToken
		authUser.entityReference = nil
	} else {
		authUser.entityReference = authResult.EntityReference
		authUser.entityReferenceToken = nil
	}

	if authResult.AttributeToken != nil {
		authUser.attributeToken = authResult.AttributeToken
		authUser.attributes = nil
	} else {
		authUser.attributes = authResult.Attributes
		authUser.attributeToken = nil
	}

	return authUser, authResult.AuthenticatedClaims, nil
}

// GetEntityReference returns the entity reference for the user.
func (m *authnProviderManager) GetEntityReference(ctx context.Context, authUser AuthUser) (
	AuthUser, *authnprovidercm.EntityReference, *serviceerror.ServiceError) {
	if !authUser.IsAuthenticated() {
		m.logger.Error(ctx, "GetEntityReference called with unauthenticated authUser")
		return authUser, nil, &serviceerror.InternalServerError
	}

	if authUser.entityReferenceToken == nil {
		// If entity reference token is nil, entity reference is already fetched and can be returned directly.
		return authUser, authUser.entityReference, nil
	}

	entityRef, err := m.provider.GetEntityReference(ctx, authUser.entityReferenceToken)
	if err != nil {
		if err.Type == serviceerror.ServerErrorType {
			m.logger.Error(ctx, "provider returned server error while fetching entity reference",
				log.String("error", err.ErrorDescription.DefaultValue))
			return authUser, nil, &serviceerror.InternalServerError
		}
		switch err.Code {
		case authnprovidercm.ErrorCodeUserNotFound:
			m.logger.Debug(ctx, "entity reference resolution failed: user not found",
				log.String("errorDescription", err.ErrorDescription.DefaultValue))
			return authUser, nil, &ErrorUserNotFound
		case authnprovidercm.ErrorCodeAmbiguousUser:
			m.logger.Debug(ctx, "entity reference resolution failed: ambiguous user",
				log.String("errorDescription", err.ErrorDescription.DefaultValue))
			return authUser, nil, &ErrorAmbiguousUser
		default:
			return authUser, nil, serviceerror.CustomServiceError(ErrorGetEntityReferenceClientError, core.I18nMessage{
				Key:          "error.authnmgrservice.get_entity_reference_client_error_description",
				DefaultValue: err.ErrorDescription.DefaultValue,
			})
		}
	}
	authUser.entityReference = entityRef
	authUser.entityReferenceToken = nil

	return authUser, entityRef, nil
}

// GetUserAvailableAttributes returns the available attributes.
func (m *authnProviderManager) GetUserAvailableAttributes(ctx context.Context,
	authUser AuthUser) (*authnprovidercm.AttributesResponse, *serviceerror.ServiceError) {
	if !authUser.IsAuthenticated() {
		m.logger.Error(ctx, "GetUserAvailableAttributes called with unauthenticated authUser")
		return nil, &serviceerror.InternalServerError
	}
	return authUser.attributes, nil
}

// GetUserAttributes returns attributes for the user.
func (m *authnProviderManager) GetUserAttributes(ctx context.Context,
	requestedAttributes *authnprovidercm.RequestedAttributes,
	metadata *authnprovidercm.GetAttributesMetadata,
	authUser AuthUser) (AuthUser, *authnprovidercm.AttributesResponse, *serviceerror.ServiceError) {
	if !authUser.IsAuthenticated() {
		m.logger.Error(ctx, "GetUserAttributes called with unauthenticated authUser")
		return authUser, nil, &serviceerror.InternalServerError
	}
	if authUser.attributeToken == nil {
		// If attribute token is nil, attribute values are already fetched and can be returned directly.
		return authUser, authUser.attributes, nil
	}
	fetchedAttributes, err := m.provider.GetAttributes(ctx, authUser.attributeToken, requestedAttributes, metadata)
	if err != nil {
		if err.Type == serviceerror.ServerErrorType {
			m.logger.Error(ctx, "provider returned server error while fetching attributes",
				log.String("error", err.ErrorDescription.DefaultValue))
			return authUser, nil, &serviceerror.InternalServerError
		}
		return authUser, nil, serviceerror.CustomServiceError(ErrorGetAttributesClientError, core.I18nMessage{
			Key:          "error.authnprovider.get_attributes_client_error_description",
			DefaultValue: err.ErrorDescription.DefaultValue,
		})
	}
	authUser.attributes = fetchedAttributes
	authUser.attributeToken = nil

	return authUser, fetchedAttributes, nil
}
