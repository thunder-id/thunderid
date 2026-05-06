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
	"encoding/json"
	"time"

	authnprovidercm "github.com/asgardeo/thunder/internal/authnprovider/common"
	"github.com/asgardeo/thunder/internal/authnprovider/provider"
	"github.com/asgardeo/thunder/internal/entityprovider"
	"github.com/asgardeo/thunder/internal/system/error/serviceerror"
	"github.com/asgardeo/thunder/internal/system/i18n/core"
	"github.com/asgardeo/thunder/internal/system/log"
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
func (m *authnProviderManager) AuthenticateUser(ctx context.Context, authnType string, authnData any,
	requestedAttributes *authnprovidercm.RequestedAttributes,
	metadata *authnprovidercm.AuthnMetadata,
	authUser AuthUser) (AuthUser, *serviceerror.ServiceError) {
	result, svcErr := m.provider.Authenticate(ctx, authnType, authnData, metadata)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ServerErrorType {
			m.logger.Error("provider returned server error during authentication",
				log.String("error", svcErr.ErrorDescription.DefaultValue))
			return AuthUser{}, &serviceerror.InternalServerError
		}
		switch svcErr.Code {
		case authnprovidercm.ErrorCodeUserNotFound:
			return AuthUser{}, &ErrorUserNotFound
		case authnprovidercm.ErrorCodeInvalidRequest:
			return AuthUser{}, &ErrorInvalidRequest
		default:
			return AuthUser{}, &ErrorAuthenticationFailed
		}
	}

	currentTime := time.Now().Unix()
	authResult := authResult{
		isVerified:    true,
		authenticator: result.AuthType,
		timestamp:     currentTime,
	}
	userResult := providerUserResult{
		attributes: make(map[string]interface{}),
		timestamp:  currentTime,
	}

	var err error
	if result.IsExistingUser {
		err = authUser.setUserState(ProviderUserStateExists)
	} else if result.IsAmbiguousUser {
		err = authUser.setUserState(ProviderUserStateAmbiguous)
	} else {
		err = authUser.setUserState(ProviderUserStateNotExists)
	}
	if err != nil {
		m.logger.Error("failed to set user state on authUser", log.String("error", err.Error()))
		return AuthUser{}, &ErrorAuthenticationFailed
	}

	attributesResult := make(map[string]interface{})
	for k, v := range result.AttributesResponse.Attributes {
		attributesResult[k] = v.Value
	}

	if result.IsExistingUser {
		userResult.attributes = attributesResult
		userResult.isValuesIncluded = result.IsAttributeValuesIncluded
		if !result.IsAttributeValuesIncluded {
			// if existing user but no included attributes, we can only rely on the included token to
			// fetch attributes later.
			if result.Token == "" {
				m.logger.Error(
					"provider did not include attribute values or a token to fetch attributes for an existing user")
				return AuthUser{}, &serviceerror.InternalServerError
			}
			userResult.token = result.Token
		}
	} else if result.IsAttributeValuesIncluded {
		// if no existing user, included attributes are attributes resolved at runtime
		// eg: userinfo from federated auth
		//     mobile number from sms OTP auth
		authResult.runtimeAttributes = attributesResult
		if result.ExternalSub != "" {
			authResult.runtimeAttributes["sub"] = result.ExternalSub
		}
	}

	if svcErr := m.validateIdentityField(
		"provider returned a different user ID than the one already set in authUser",
		"existingUserID", "newUserID", authUser.GetUserID(), result.UserID, true); svcErr != nil {
		return AuthUser{}, svcErr
	}
	userResult.userID = result.UserID

	if svcErr := m.validateIdentityField(
		"provider returned a different user type than the one already set in authUser",
		"existingUserType", "newUserType", authUser.GetUserType(), result.UserType, false); svcErr != nil {
		return AuthUser{}, svcErr
	}
	userResult.userType = result.UserType

	if svcErr := m.validateIdentityField(
		"provider returned a different OUID than the one already set in authUser",
		"existingOUID", "newOUID", authUser.GetOUID(), result.OUID, false); svcErr != nil {
		return AuthUser{}, svcErr
	}
	userResult.ouID = result.OUID

	authUser.authHistory = append(authUser.authHistory, &authResult)

	if result.IsExistingUser {
		authUser.userHistory = append(authUser.userHistory, &userResult)
	}

	return authUser, nil
}

// AuthenticateResolvedUser is used to complete the authentication of a user whose last authentication step resulted
// in a non-existing local user.
// i.e. user was ambigous or did not exist at the time AuthenticateUser was called, but has since been resolved to
// an existing user (e.g. through user provisioning or disambiguation).
func (m *authnProviderManager) AuthenticateResolvedUser(ctx context.Context, resolvedUser *entityprovider.Entity,
	authUser AuthUser) (AuthUser, *serviceerror.ServiceError) {
	userResult := providerUserResult{
		attributes: make(map[string]interface{}),
		timestamp:  time.Now().Unix(),
	}

	if svcErr := m.validateIdentityField(
		"resolved user has a different user ID than the one already set in authUser",
		"existingUserID", "newUserID", authUser.GetUserID(), resolvedUser.ID, true); svcErr != nil {
		return AuthUser{}, svcErr
	}
	userResult.userID = resolvedUser.ID

	if svcErr := m.validateIdentityField(
		"resolved user has a different user type than the one already set in authUser",
		"existingUserType", "newUserType", authUser.GetUserType(), resolvedUser.Type, false); svcErr != nil {
		return AuthUser{}, svcErr
	}
	userResult.userType = resolvedUser.Type

	if svcErr := m.validateIdentityField(
		"resolved user has a different OUID than the one already set in authUser",
		"existingOUID", "newOUID", authUser.GetOUID(), resolvedUser.OUID, false); svcErr != nil {
		return AuthUser{}, svcErr
	}
	userResult.ouID = resolvedUser.OUID

	if resolvedUser.Attributes != nil {
		// populate lastAuthResult.attributes by using attributes from resolved user
		var resolvedUserAttributes map[string]interface{}
		if err := json.Unmarshal(resolvedUser.Attributes, &resolvedUserAttributes); err != nil {
			m.logger.Error("failed to unmarshal resolved user attributes", log.String("error", err.Error()))
			return AuthUser{}, &serviceerror.InternalServerError
		}
		userResult.attributes = resolvedUserAttributes
		userResult.isValuesIncluded = true
	}

	authUser.userHistory = append(authUser.userHistory, &userResult)
	err := authUser.setUserState(ProviderUserStateExists)
	if err != nil {
		m.logger.Error("failed to set user state on authUser", log.String("error", err.Error()))
		return AuthUser{}, &ErrorAuthenticationFailed
	}

	return authUser, nil
}

func (m *authnProviderManager) AuthenticateForRegistration(ctx context.Context, credentialType string,
	authUser AuthUser) (AuthUser, *serviceerror.ServiceError) {
	// TODO: this should also go through authn provider.

	authResult := authResult{
		timestamp:     time.Now().Unix(),
		authenticator: credentialType,
		isVerified:    true,
	}

	authUser.authHistory = append(authUser.authHistory, &authResult)

	return authUser, nil
}

// GetUserAvailableAttributes returns the cached attributes for the default provider without making a provider call.
func (m *authnProviderManager) GetUserAvailableAttributes(ctx context.Context,
	authUser AuthUser) (*authnprovidercm.AttributesResponse, *serviceerror.ServiceError) {
	result := &authnprovidercm.AttributesResponse{
		Attributes: make(map[string]*authnprovidercm.AttributeResponse),
	}

	// runtime attributes have a lower precedence than provider attributes in attribute conflict resolution.
	for _, authResult := range authUser.authHistory {
		for attrName, attrValue := range authResult.runtimeAttributes {
			result.Attributes[attrName] = &authnprovidercm.AttributeResponse{Value: attrValue}
		}
	}
	for _, userResult := range authUser.userHistory {
		for attrName, attrValue := range userResult.attributes {
			result.Attributes[attrName] = &authnprovidercm.AttributeResponse{Value: attrValue}
		}
	}

	return result, nil
}

// GetUserAttributes returns attributes for the user, fetching from the provider if not already cached.
func (m *authnProviderManager) GetUserAttributes(ctx context.Context,
	requestedAttributes *authnprovidercm.RequestedAttributes,
	metadata *authnprovidercm.GetAttributesMetadata,
	authUser AuthUser) (AuthUser, *authnprovidercm.AttributesResponse, *serviceerror.ServiceError) {
	// TODO: we do not preserve attribute verification data. need to improve this.

	result := &authnprovidercm.AttributesResponse{
		Attributes: make(map[string]*authnprovidercm.AttributeResponse),
	}

	// runtime attributes have a lower precedence than provider attributes in attribute conflict resolution.
	for _, authResult := range authUser.authHistory {
		for attrName, attrValue := range authResult.runtimeAttributes {
			if isAttributeRequested(attrName, requestedAttributes) {
				result.Attributes[attrName] = &authnprovidercm.AttributeResponse{Value: attrValue}
			}
		}
	}

	for _, userResult := range authUser.userHistory {
		if !userResult.isValuesIncluded {
			userResult.attributes = make(map[string]interface{})
			fetchedAttributes, svcErr := m.provider.GetAttributes(ctx, userResult.token, requestedAttributes, metadata)
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
			for fetchedAttrName, fetchedAttrResponse := range fetchedAttributes.AttributesResponse.Attributes {
				userResult.attributes[fetchedAttrName] = fetchedAttrResponse.Value
			}
			userResult.isValuesIncluded = true
		}
		for attrName, attrValue := range userResult.attributes {
			if isAttributeRequested(attrName, requestedAttributes) {
				result.Attributes[attrName] = &authnprovidercm.AttributeResponse{Value: attrValue}
			}
		}
	}

	return authUser, result, nil
}

// GetAuthenticatorMetadata returns the metadata for the given authenticator by delegating to the provider.
func (m *authnProviderManager) GetAuthenticatorMetadata(authenticatorName string) *authnprovidercm.AuthenticatorMeta {
	return m.provider.GetAuthenticatorMetadata(authenticatorName)
}

// GetAuthenticatorFactors returns the authentication factors for the given authenticator.
func (m *authnProviderManager) GetAuthenticatorFactors(
	authenticatorName string) []authnprovidercm.AuthenticationFactor {
	meta := m.GetAuthenticatorMetadata(authenticatorName)
	if meta == nil {
		return nil
	}
	return meta.Factors
}

// isAttributeRequested returns true if attrName should be included in the response.
// If no attribute filter is specified, all attributes are included.
func isAttributeRequested(attrName string, requestedAttributes *authnprovidercm.RequestedAttributes) bool {
	if requestedAttributes == nil || requestedAttributes.Attributes == nil {
		return true
	}
	_, ok := requestedAttributes.Attributes[attrName]
	return ok
}

// validateIdentityField checks that a new identity field value does not conflict with an existing one.
// Set masked to true for sensitive fields (e.g. user ID) to redact values in logs.
func (m *authnProviderManager) validateIdentityField(logMsg, existingKey, newKey, existingVal, newVal string,
	masked bool) *serviceerror.ServiceError {
	if newVal != "" && existingVal != "" && existingVal != newVal {
		logField := log.String
		if masked {
			logField = log.MaskedString
		}
		m.logger.Error(logMsg,
			logField(existingKey, existingVal),
			logField(newKey, newVal))
		return serviceerror.CustomServiceError(ErrorAuthenticationFailed, core.I18nMessage{
			Key:          "error.authnprovider.inconsistent_user_identity_description",
			DefaultValue: "authentication failed due to inconsistent user identity information",
		})
	}
	return nil
}
