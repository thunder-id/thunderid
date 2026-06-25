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
	"fmt"
	"maps"
	"slices"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/authnprovider/provider"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const defaultProviderName providerName = "default"

// authnProviderManager is a proxy struct that implements AuthnProviderManagerInterface by delegating
// to an underlying AuthnProviderInterface.
type authnProviderManager struct {
	providers             map[providerName]provider.AuthnProviderInterface
	logger                *log.Logger
	credToProviderMapping map[string]providerName
}

// defaultCredToProviderMapping returns the built-in credential-to-provider mapping
// that points every known credential key at the default provider. This is the
// base that config overlay merges into; deployments without overlays get this
// mapping verbatim.
func defaultCredToProviderMapping() map[string]providerName {
	return map[string]providerName{
		"provisionedEntityID": defaultProviderName,
		"passkey":             defaultProviderName,
		"otp":                 defaultProviderName,
		"federated":           defaultProviderName,
		"magiclink":           defaultProviderName,
		"openid4vp":           defaultProviderName,
		"password":            defaultProviderName,
		"clientSecret":        defaultProviderName,
	}
}

// newAuthnProviderManager creates a new authnProviderManager from a pre-built
// providers map (keyed by catalog name) and an optional credential-key override
// map from config. Returns an error if any override or default mapping points
// at a provider name that isn't registered.
func newAuthnProviderManager(providers map[string]provider.AuthnProviderInterface,
	credMapOverlay map[string]string) (AuthnProviderManagerInterface, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("authn provider manager: at least one provider must be registered")
	}

	registered := make(map[providerName]provider.AuthnProviderInterface, len(providers))
	for name, p := range providers {
		if p == nil {
			return nil, fmt.Errorf("authn provider manager: provider %q is nil", name)
		}
		registered[providerName(name)] = p
	}

	credMap := defaultCredToProviderMapping()
	for k, v := range credMapOverlay {
		credMap[k] = providerName(v)
	}
	for credKey, target := range credMap {
		if _, ok := registered[target]; !ok {
			return nil, fmt.Errorf("authn provider manager: credential_mapping[%q] references "+
				"unregistered provider %q", credKey, target)
		}
	}

	return &authnProviderManager{
		providers:             registered,
		logger:                log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthnProviderManager")),
		credToProviderMapping: credMap,
	}, nil
}

// AuthenticateUser authenticates with the underlying provider and returns an updated AuthUser.
func (m *authnProviderManager) AuthenticateUser(ctx context.Context, identifiers, credentials map[string]interface{},
	requestedAttributes *authnprovidercm.RequestedAttributes,
	metadata *authnprovidercm.AuthnMetadata,
	authUser AuthUser) (AuthUser, authnprovidercm.AuthenticatedClaims, *serviceerror.ServiceError) {
	if len(credentials) == 0 {
		m.logger.Debug(ctx, "no credentials provided for authentication")
		return authUser, nil, &ErrorAuthenticationFailed
	}

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

		authUserState, ok := authUser.state[defaultProviderName]
		if !ok {
			m.logger.Debug(ctx, "disambiguation requested but current user has no state for the default provider")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		if authUserState.entityReferenceToken == nil {
			m.logger.Debug(ctx, "disambiguation requested but current user's entity reference token is missing")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		entityRefToken, ok := authUserState.entityReferenceToken.(map[string]interface{})
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
		userIDToken := map[string]interface{}{authnprovidercm.UserAttributeUserID: valStr}
		authUser.state[defaultProviderName] = authState{
			entityReferenceToken: userIDToken,
			attributeToken:       userIDToken,
		}
		return authUser, nil, nil
	}

	// Determine the provider from the credential key. The current contract is one credential
	// key per request; if more than one is supplied, pick deterministically (lowest sorted key)
	// and log so the divergence is visible.
	credKeys := slices.Sorted(maps.Keys(credentials))
	if len(credKeys) > 1 {
		m.logger.Debug(ctx, "multiple credential keys provided; only the first will be used",
			log.String("selectedKey", credKeys[0]))
	}
	credKey := credKeys[0]
	selectedProviderName, ok := m.credToProviderMapping[credKey]
	if !ok {
		m.logger.Debug(ctx, "no provider mapping found for credential key",
			log.String("credentialKey", credKey))
		return authUser, nil, &ErrorAuthenticationFailed
	}
	selectedProvider, ok := m.providers[selectedProviderName]
	if !ok || selectedProvider == nil {
		m.logger.Error(ctx, "credential key mapped to a provider that is not registered",
			log.String("credentialKey", credKey), log.String("providerName", string(selectedProviderName)))
		return authUser, nil, &serviceerror.InternalServerError
	}

	authResult, svcErr := selectedProvider.Authenticate(ctx, identifiers, credentials, metadata)
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

	state := authState{}

	if authResult.EntityReferenceToken != nil {
		state.entityReferenceToken = authResult.EntityReferenceToken
		state.entityReference = nil
	} else {
		state.entityReference = authResult.EntityReference
		state.entityReferenceToken = nil
	}

	if authResult.AttributeToken != nil {
		state.attributeToken = authResult.AttributeToken
		state.attributes = nil
	} else {
		state.attributes = authResult.Attributes
		state.attributeToken = nil
	}

	if authUser.state == nil {
		authUser.state = make(map[providerName]authState)
	}
	authUser.state[selectedProviderName] = state

	return authUser, authResult.AuthenticatedClaims, nil
}

// GetEntityReference returns the entity reference for the user.
func (m *authnProviderManager) GetEntityReference(ctx context.Context, authUser AuthUser) (
	AuthUser, *authnprovidercm.EntityReference, *serviceerror.ServiceError) {
	if !authUser.IsAuthenticated() {
		m.logger.Error(ctx, "GetEntityReference called with unauthenticated authUser")
		return authUser, nil, &serviceerror.InternalServerError
	}

	var entityRef *authnprovidercm.EntityReference
	seen := false
	pendingUpdates := map[providerName]authState{}

	for _, name := range slices.Sorted(maps.Keys(authUser.state)) {
		state := authUser.state[name]
		var providerEntityRef *authnprovidercm.EntityReference
		if state.entityReferenceToken == nil {
			providerEntityRef = state.entityReference
		} else {
			p, ok := m.providers[name]
			if !ok || p == nil {
				m.logger.Error(ctx, "no provider registered for authUser state entry",
					log.String("providerName", string(name)))
				return authUser, nil, &serviceerror.InternalServerError
			}
			var err *serviceerror.ServiceError
			providerEntityRef, err = p.GetEntityReference(ctx, state.entityReferenceToken)
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
					return authUser, nil, serviceerror.CustomServiceError(
						ErrorGetEntityReferenceClientError,
						core.I18nMessage{
							Key:          "error.authnmgrservice.get_entity_reference_client_error_description",
							DefaultValue: err.ErrorDescription.DefaultValue,
						})
				}
			}
			state.entityReference = providerEntityRef
			state.entityReferenceToken = nil
			pendingUpdates[name] = state
		}
		if seen && !isEntityRefsEqual(entityRef, providerEntityRef) {
			m.logger.Debug(
				ctx, "entity reference resolution failed: multiple providers returned different entity references")
			return authUser, nil, &serviceerror.InternalServerError
		}
		entityRef = providerEntityRef
		seen = true
	}

	for name, state := range pendingUpdates {
		authUser.state[name] = state
	}

	return authUser, entityRef, nil
}

// GetUserAvailableAttributes returns the available attributes.
func (m *authnProviderManager) GetUserAvailableAttributes(ctx context.Context,
	authUser AuthUser) (*authnprovidercm.AttributesResponse, *serviceerror.ServiceError) {
	if !authUser.IsAuthenticated() {
		m.logger.Error(ctx, "GetUserAvailableAttributes called with unauthenticated authUser")
		return nil, &serviceerror.InternalServerError
	}

	availableAttributes := newAttributesResponse()
	for _, name := range slices.Sorted(maps.Keys(authUser.state)) {
		mergeAttributes(availableAttributes, authUser.state[name].attributes)
	}

	return availableAttributes, nil
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
	attributes := newAttributesResponse()
	pendingUpdates := map[providerName]authState{}
	for _, name := range slices.Sorted(maps.Keys(authUser.state)) {
		state := authUser.state[name]
		if state.attributeToken == nil {
			mergeAttributes(attributes, state.attributes)
			continue
		}
		p, ok := m.providers[name]
		if !ok || p == nil {
			m.logger.Error(ctx, "no provider registered for authUser state entry",
				log.String("providerName", string(name)))
			return authUser, nil, &serviceerror.InternalServerError
		}
		fetchedAttributes, err := p.GetAttributes(ctx, state.attributeToken, requestedAttributes, metadata)
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
		mergeAttributes(attributes, fetchedAttributes)
		state.attributes = fetchedAttributes
		state.attributeToken = nil
		pendingUpdates[name] = state
	}
	for name, state := range pendingUpdates {
		authUser.state[name] = state
	}
	return authUser, attributes, nil
}

func newAttributesResponse() *authnprovidercm.AttributesResponse {
	return &authnprovidercm.AttributesResponse{
		Attributes:    map[string]*authnprovidercm.AttributeResponse{},
		Verifications: map[string]*authnprovidercm.VerificationResponse{},
	}
}

func mergeAttributes(dst *authnprovidercm.AttributesResponse, src *authnprovidercm.AttributesResponse) {
	if src == nil {
		return
	}
	for k, v := range src.Attributes {
		dst.Attributes[k] = v
	}
	for k, v := range src.Verifications {
		dst.Verifications[k] = v
	}
}

func isEntityRefsEqual(ref1, ref2 *authnprovidercm.EntityReference) bool {
	if ref1 == nil && ref2 == nil {
		return true
	}
	if ref1 == nil || ref2 == nil {
		return false
	}
	if ref1.EntityID != ref2.EntityID {
		return false
	}
	if ref1.EntityType != ref2.EntityType {
		return false
	}
	if ref1.OUID != ref2.OUID {
		return false
	}
	// EntityCategory is intentionally not included in the equality check as it is not a required field
	// and may be missing in some cases.
	return true
}
