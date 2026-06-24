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
	"fmt"
	"maps"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/authnprovider/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const defaultProviderName = "default"

// authnProviderManager dispatches authentication requests across one or more
// registered providers, choosing the provider for a given request based on the
// credential key supplied.
type authnProviderManager struct {
	providers             map[string]provider.AuthnProviderInterface
	logger                *log.Logger
	credToProviderMapping map[string]string
}

// defaultCredToProviderMapping returns the built-in credential-to-provider mapping
// that points every known credential key at the default provider. Config overlay
// merges into this base; deployments without overlays get this mapping verbatim.
func defaultCredToProviderMapping() map[string]string {
	return map[string]string{
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
func newAuthnProviderManager(registered map[string]provider.AuthnProviderInterface,
	credMapOverlay map[string]string) (providers.AuthnProviderManagerInterface, error) {
	if len(registered) == 0 {
		return nil, fmt.Errorf("authn provider manager: at least one provider must be registered")
	}

	for name, p := range registered {
		if p == nil {
			return nil, fmt.Errorf("authn provider manager: provider %q is nil", name)
		}
	}

	credMap := defaultCredToProviderMapping()
	maps.Copy(credMap, credMapOverlay)
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

// AuthenticateUser routes a credential to the matching provider and merges the
// provider's auth result into the AuthUser under the provider's name.
func (m *authnProviderManager) AuthenticateUser(ctx context.Context, identifiers, credentials map[string]interface{},
	requestedAttributes *providers.RequestedAttributes,
	metadata *providers.AuthnMetadata,
	authUser providers.AuthUser) (providers.AuthUser, providers.AuthenticatedClaims, *tidcommon.ServiceError) {
	if len(credentials) == 0 {
		m.logger.Debug(ctx, "no credentials provided for authentication")
		return authUser, nil, &ErrorAuthenticationFailed
	}

	if sub, ok := credentials["sub"]; ok {
		// Temporary handling of disambiguation after a federated authentication step.
		// Only works with Thunder's default authn provider.
		if subStr, ok := sub.(string); !ok || subStr == "" {
			m.logger.Debug(ctx, "disambiguation requested but sub is missing or invalid in credentials")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		if !authUser.IsAuthenticated() {
			m.logger.Debug(ctx, "disambiguation requested but current user is not authenticated")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		authUserState, ok := authUser.StateFor(defaultProviderName)
		if !ok {
			m.logger.Debug(ctx, "disambiguation requested but current user has no state for the default provider")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		if authUserState.EntityReferenceToken == nil {
			m.logger.Debug(ctx, "disambiguation requested but current user's entity reference token is missing")
			return authUser, nil, &ErrorAuthenticationFailed
		}

		entityRefToken, ok := authUserState.EntityReferenceToken.(map[string]interface{})
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
		authUser.SetStateFor(defaultProviderName, providers.AuthState{
			EntityReferenceToken: userIDToken,
			AttributeToken:       userIDToken,
		})
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
			log.String("credentialKey", credKey), log.String("providerName", selectedProviderName))
		return authUser, nil, &tidcommon.InternalServerError
	}

	authResult, svcErr := selectedProvider.Authenticate(ctx, identifiers, credentials, metadata)
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

	state := providers.AuthState{}
	if authResult.EntityReferenceToken != nil {
		state.EntityReferenceToken = authResult.EntityReferenceToken
	} else {
		state.EntityReference = authResult.EntityReference
	}
	if authResult.AttributeToken != nil {
		state.AttributeToken = authResult.AttributeToken
	} else {
		state.Attributes = authResult.Attributes
	}
	authUser.SetStateFor(selectedProviderName, state)

	return authUser, authResult.AuthenticatedClaims, nil
}

// GetEntityReference resolves a single entity reference across all providers
// in the AuthUser. Each provider's pending entity-reference token is resolved
// through that provider; the resolved references must agree (modulo
// EntityCategory) or the call fails.
func (m *authnProviderManager) GetEntityReference(ctx context.Context, authUser providers.AuthUser) (
	providers.AuthUser, *providers.EntityReference, *tidcommon.ServiceError) {
	if !authUser.IsAuthenticated() {
		m.logger.Error(ctx, "GetEntityReference called with unauthenticated authUser")
		return authUser, nil, &tidcommon.InternalServerError
	}

	var entityRef *providers.EntityReference
	seen := false

	for _, name := range authUser.ProviderNames() {
		state, _ := authUser.StateFor(name)
		var providerEntityRef *providers.EntityReference
		if state.EntityReferenceToken == nil {
			providerEntityRef = state.EntityReference
		} else {
			p, ok := m.providers[name]
			if !ok || p == nil {
				m.logger.Error(ctx, "no provider registered for authUser state entry",
					log.String("providerName", name))
				return authUser, nil, &tidcommon.InternalServerError
			}
			resolved, err := p.GetEntityReference(ctx, state.EntityReferenceToken)
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
			providerEntityRef = resolved
			state.EntityReference = resolved
			state.EntityReferenceToken = nil
			authUser.SetStateFor(name, state)
		}
		if seen && !isEntityRefsEqual(entityRef, providerEntityRef) {
			m.logger.Debug(ctx,
				"entity reference resolution failed: multiple providers returned different entity references")
			return authUser, nil, &tidcommon.InternalServerError
		}
		entityRef = providerEntityRef
		seen = true
	}

	return authUser, entityRef, nil
}

// GetUserAvailableAttributes returns the merged attributes available across
// every provider's state in the AuthUser.
func (m *authnProviderManager) GetUserAvailableAttributes(ctx context.Context,
	authUser providers.AuthUser) (*providers.AttributesResponse, *tidcommon.ServiceError) {
	if !authUser.IsAuthenticated() {
		m.logger.Error(ctx, "GetUserAvailableAttributes called with unauthenticated authUser")
		return nil, &tidcommon.InternalServerError
	}

	available := newAttributesResponse()
	for _, name := range authUser.ProviderNames() {
		state, _ := authUser.StateFor(name)
		mergeAttributes(available, state.Attributes)
	}
	return available, nil
}

// GetUserAttributes resolves and merges attributes across every provider in
// the AuthUser. Each provider's pending attribute token is fetched through
// that provider; already-resolved attributes pass through unchanged.
func (m *authnProviderManager) GetUserAttributes(ctx context.Context,
	requestedAttributes *providers.RequestedAttributes,
	metadata *providers.GetAttributesMetadata,
	authUser providers.AuthUser) (providers.AuthUser, *providers.AttributesResponse, *tidcommon.ServiceError) {
	if !authUser.IsAuthenticated() {
		m.logger.Error(ctx, "GetUserAttributes called with unauthenticated authUser")
		return authUser, nil, &tidcommon.InternalServerError
	}

	attributes := newAttributesResponse()
	for _, name := range authUser.ProviderNames() {
		state, _ := authUser.StateFor(name)
		if state.AttributeToken == nil {
			mergeAttributes(attributes, state.Attributes)
			continue
		}
		p, ok := m.providers[name]
		if !ok || p == nil {
			m.logger.Error(ctx, "no provider registered for authUser state entry",
				log.String("providerName", name))
			return authUser, nil, &tidcommon.InternalServerError
		}
		fetched, err := p.GetAttributes(ctx, state.AttributeToken, requestedAttributes, metadata)
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
		mergeAttributes(attributes, fetched)
		state.Attributes = fetched
		state.AttributeToken = nil
		authUser.SetStateFor(name, state)
	}
	return authUser, attributes, nil
}

func newAttributesResponse() *providers.AttributesResponse {
	return &providers.AttributesResponse{
		Attributes:    map[string]*providers.AttributeResponse{},
		Verifications: map[string]*providers.VerificationResponse{},
	}
}

func mergeAttributes(dst, src *providers.AttributesResponse) {
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

func isEntityRefsEqual(ref1, ref2 *providers.EntityReference) bool {
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
	// EntityCategory is intentionally excluded — it's optional and may be missing.
	return true
}
