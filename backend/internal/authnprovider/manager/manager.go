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

// Package manager manages authentication providers and their interactions.
package manager

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/authnprovider/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// authnProviderManager is a proxy struct that implements AuthnProviderManager by delegating
// to an underlying AuthnProviderInterface.
type authnProviderManager struct {
	provider provider.AuthnProviderInterface
	logger   *log.Logger
}

// newAuthnProviderManager creates a new authnProviderManager.
func newAuthnProviderManager(p provider.AuthnProviderInterface) providers.AuthnProviderManager {
	return &authnProviderManager{
		provider: p,
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthnProviderManager")),
	}
}

// AuthenticateUser authenticates with the underlying provider and returns an updated AuthUser.
func (m *authnProviderManager) AuthenticateUser(ctx context.Context, identifiers, credentials map[string]interface{},
	requestedAttributes *providers.RequestedAttributes,
	metadata *providers.AuthnMetadata,
	authUser providers.AuthUser) (providers.AuthUser, providers.AuthenticatedClaims, *tidcommon.ServiceError) {
	if sub, ok := credentials["sub"]; ok {
		if subStr, ok := sub.(string); !ok || subStr == "" {
			m.logger.Debug(ctx, "disambiguation requested but sub is missing or invalid in credentials")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		if !authUser.IsAuthenticated() {
			m.logger.Debug(ctx, "disambiguation requested but current user is not authenticated")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		if authUser.EntityReferenceToken() == nil {
			m.logger.Debug(ctx, "disambiguation requested but current user's entity reference token is missing")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		entityRefToken, ok := authUser.EntityReferenceToken().(map[string]interface{})
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
		authUser := providers.AuthUser{}
		authUser.SetEntityReferenceToken(map[string]interface{}{
			authnprovidercm.UserAttributeUserID: userID,
		})
		authUser.SetAttributeToken(map[string]interface{}{
			authnprovidercm.UserAttributeUserID: userID,
		})
		return authUser, nil, nil
	}

	authResult, svcErr := m.provider.Authenticate(ctx, identifiers, credentials, metadata)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			m.logger.Error(ctx, "provider returned server error during authentication",
				log.String("error", svcErr.ErrorDescription.DefaultValue))
			return authUser, nil, &tidcommon.InternalServerError
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
		return authUser, nil, &tidcommon.InternalServerError
	}

	if authResult.EntityReferenceToken != nil {
		authUser.SetEntityReferenceToken(authResult.EntityReferenceToken)
	} else {
		authUser.SetEntityReference(authResult.EntityReference)
	}

	if authResult.AttributeToken != nil {
		authUser.SetAttributeToken(authResult.AttributeToken)
	} else {
		authUser.SetAttributes(authResult.Attributes)
	}

	return authUser, authResult.AuthenticatedClaims, nil
}

// GetEntityReference returns the entity reference for the user.
func (m *authnProviderManager) GetEntityReference(ctx context.Context, authUser providers.AuthUser) (
	providers.AuthUser, *providers.EntityReference, *tidcommon.ServiceError) {
	if !authUser.IsAuthenticated() {
		m.logger.Error(ctx, "GetEntityReference called with unauthenticated authUser")
		return authUser, nil, &tidcommon.InternalServerError
	}

	if authUser.EntityReferenceToken() == nil {
		return authUser, authUser.EntityReference(), nil
	}

	entityRef, err := m.provider.GetEntityReference(ctx, authUser.EntityReferenceToken())
	if err != nil {
		if err.Type == tidcommon.ServerErrorType {
			m.logger.Error(ctx, "provider returned server error while fetching entity reference",
				log.String("error", err.ErrorDescription.DefaultValue))
			return authUser, nil, &tidcommon.InternalServerError
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
			return authUser, nil, tidcommon.CustomServiceError(
				ErrorGetEntityReferenceClientError,
				tidcommon.I18nMessage{
					Key:          "error.authnmgrservice.get_entity_reference_client_error_description",
					DefaultValue: err.ErrorDescription.DefaultValue,
				})
		}
	}

	authUser.SetEntityReference(entityRef)

	return authUser, entityRef, nil
}

// GetUserAvailableAttributes returns the available attributes.
func (m *authnProviderManager) GetUserAvailableAttributes(ctx context.Context,
	authUser providers.AuthUser) (*providers.AttributesResponse, *tidcommon.ServiceError) {
	if !authUser.IsAuthenticated() {
		m.logger.Error(ctx, "GetUserAvailableAttributes called with unauthenticated authUser")
		return nil, &tidcommon.InternalServerError
	}
	return authUser.Attributes(), nil
}

// GetUserAttributes returns attributes for the user.
func (m *authnProviderManager) GetUserAttributes(ctx context.Context,
	requestedAttributes *providers.RequestedAttributes,
	metadata *providers.GetAttributesMetadata,
	authUser providers.AuthUser) (providers.AuthUser, *providers.AttributesResponse, *tidcommon.ServiceError) {
	if !authUser.IsAuthenticated() {
		m.logger.Error(ctx, "GetUserAttributes called with unauthenticated authUser")
		return authUser, nil, &tidcommon.InternalServerError
	}
	if authUser.AttributeToken() == nil {
		return authUser, authUser.Attributes(), nil
	}
	fetchedAttributes, err := m.provider.GetAttributes(ctx, authUser.AttributeToken(),
		requestedAttributes, metadata)
	if err != nil {
		if err.Type == tidcommon.ServerErrorType {
			m.logger.Error(ctx, "provider returned server error while fetching attributes",
				log.String("error", err.ErrorDescription.DefaultValue))
			return authUser, nil, &tidcommon.InternalServerError
		}
		return authUser, nil, tidcommon.CustomServiceError(ErrorGetAttributesClientError, tidcommon.I18nMessage{
			Key:          "error.authnprovider.get_attributes_client_error_description",
			DefaultValue: err.ErrorDescription.DefaultValue,
		})
	}
	authUser.SetAttributes(fetchedAttributes)

	return authUser, fetchedAttributes, nil
}
