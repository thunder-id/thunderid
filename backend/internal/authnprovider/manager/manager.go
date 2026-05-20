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
	authUser AuthUser) (AuthUser, *AuthnBasicResult, *serviceerror.ServiceError) {
	result, svcErr := m.provider.Authenticate(ctx, identifiers, credentials, metadata)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ServerErrorType {
			m.logger.Error("provider returned server error during authentication",
				log.String("error", svcErr.ErrorDescription.DefaultValue))
			return AuthUser{}, nil, &serviceerror.InternalServerError
		}
		switch svcErr.Code {
		case authnprovidercm.ErrorCodeUserNotFound:
			return AuthUser{}, nil, serviceerror.CustomServiceError(ErrorUserNotFound, core.I18nMessage{
				Key:          "error.authnprovider.user_not_found_description",
				DefaultValue: svcErr.ErrorDescription.DefaultValue,
			})
		case authnprovidercm.ErrorCodeInvalidRequest:
			return AuthUser{}, nil, serviceerror.CustomServiceError(ErrorInvalidRequest, core.I18nMessage{
				Key:          "error.authnprovider.invalid_request_description",
				DefaultValue: svcErr.ErrorDescription.DefaultValue,
			})
		default:
			return AuthUser{}, nil, serviceerror.CustomServiceError(ErrorAuthenticationFailed, core.I18nMessage{
				Key:          "error.authnprovider.authentication_failed_description",
				DefaultValue: svcErr.ErrorDescription.DefaultValue,
			})
		}
	}
	if !result.IsExistingUser {
		return authUser, &AuthnBasicResult{
			ExternalSub:     result.ExternalSub,
			ExternalClaims:  result.ExternalClaims,
			IsExistingUser:  false,
			IsAmbiguousUser: result.IsAmbiguousUser,
		}, nil
	}
	authUser.setIdentity(result.UserID, result.UserType, result.OUID)
	authUser.setProviderData(defaultProvider, providerData{
		token:                     result.Token,
		attributes:                result.AttributesResponse,
		isAttributeValuesIncluded: result.IsAttributeValuesIncluded,
	})
	return authUser, &AuthnBasicResult{
		UserID:         result.UserID,
		OUID:           result.OUID,
		UserType:       result.UserType,
		IsExistingUser: true,
		ExternalSub:    result.ExternalSub,
		ExternalClaims: result.ExternalClaims,
	}, nil
}

// GetUserAvailableAttributes returns the cached attributes for the default provider without making a provider call.
func (m *authnProviderManager) GetUserAvailableAttributes(ctx context.Context,
	authUser AuthUser) (*authnprovidercm.AttributesResponse, *serviceerror.ServiceError) {
	if !authUser.IsAuthenticated() {
		m.logger.Error("GetUserAvailableAttributes called with unauthenticated authUser")
		return nil, &serviceerror.InternalServerError
	}
	data, ok := authUser.getProviderData(defaultProvider)
	if !ok {
		m.logger.Error("GetUserAvailableAttributes: no provider data found for default provider")
		return nil, &serviceerror.InternalServerError
	}
	return data.attributes, nil
}

// GetUserAttributes returns attributes for the user, fetching from the provider if not already cached.
func (m *authnProviderManager) GetUserAttributes(ctx context.Context,
	requestedAttributes *authnprovidercm.RequestedAttributes,
	metadata *authnprovidercm.GetAttributesMetadata,
	authUser AuthUser) (AuthUser, *authnprovidercm.AttributesResponse, *serviceerror.ServiceError) {
	if !authUser.IsAuthenticated() {
		m.logger.Error("GetUserAttributes called with unauthenticated authUser")
		return AuthUser{}, nil, &serviceerror.InternalServerError
	}
	data, ok := authUser.getProviderData(defaultProvider)
	if !ok {
		m.logger.Error("GetUserAttributes: no provider data found for default provider")
		return AuthUser{}, nil, &serviceerror.InternalServerError
	}
	if data.isAttributeValuesIncluded {
		return authUser, data.attributes, nil
	}
	result, svcErr := m.provider.GetAttributes(ctx, data.token, requestedAttributes, metadata)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ServerErrorType {
			m.logger.Error("provider returned server error while fetching attributes",
				log.String("error", svcErr.ErrorDescription.DefaultValue))
			return AuthUser{}, nil, &serviceerror.InternalServerError
		}
		return AuthUser{}, nil, serviceerror.CustomServiceError(ErrorGetAttributesClientError, core.I18nMessage{
			Key:          "error.authnprovider.get_attributes_client_error_description",
			DefaultValue: svcErr.ErrorDescription.DefaultValue,
		})
	}
	authUser.setProviderData(defaultProvider, providerData{
		token:                     data.token,
		attributes:                result.AttributesResponse,
		isAttributeValuesIncluded: true,
	})
	return authUser, result.AttributesResponse, nil
}
