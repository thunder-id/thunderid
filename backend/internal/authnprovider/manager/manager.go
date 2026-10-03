// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	"github.com/thunder-id/thunderid/internal/authnprovider/defaultprovider"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const defaultProviderName = defaultprovider.Name

// authnProviderManager dispatches authentication requests across one or more
// registered providers, choosing the provider for a given request based on the
// credential key supplied.
type authnProviderManager struct {
	authnProviders        map[string]providers.AuthnProviderInterface
	logger                *log.Logger
	credToProviderMapping map[string]string
}

// newAuthnProviderManager creates a new authnProviderManager from the default provider
// and the optional custom providers.
func newAuthnProviderManager(defaultProvider providers.AuthnProviderInterface,
	customProviders map[string]providers.CustomAuthnProvider) (providers.AuthnProviderManager, error) {
	if defaultProvider == nil {
		return nil, fmt.Errorf("authn provider manager: default provider must not be nil")
	}

	providerMap := make(map[string]providers.AuthnProviderInterface, len(customProviders)+1)
	providerMap[defaultProviderName] = defaultProvider
	for name, ap := range customProviders {
		if name == defaultProviderName {
			return nil, fmt.Errorf("authn provider manager: %q is reserved for the default provider", name)
		}
		if ap.Instance == nil {
			return nil, fmt.Errorf("authn provider manager: provider %q is nil", name)
		}
		providerMap[name] = ap.Instance
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthnProviderManager"))

	credMap, err := buildCredentialRouting(customProviders)
	if err != nil {
		return nil, err
	}

	return &authnProviderManager{
		authnProviders:        providerMap,
		logger:                logger,
		credToProviderMapping: credMap,
	}, nil
}

// buildCredentialRouting derives the credential-key -> provider-name routing table from
// the custom providers' declared credential keys. Keys not present in the table are routed
// to the default provider at request time. Two custom providers claiming the same key is
// an error.
func buildCredentialRouting(customProviders map[string]providers.CustomAuthnProvider) (map[string]string, error) {
	routing := map[string]string{}
	for name, p := range customProviders {
		for _, credKey := range p.Creds {
			if prev, dup := routing[credKey]; dup {
				return nil, fmt.Errorf("authn provider manager: credential %q is claimed by "+
					"multiple providers (%q and %q)", credKey, prev, name)
			}
			routing[credKey] = name
		}
	}
	return routing, nil
}

// InitiateAuthentication routes an authentication-initiation request
// to the provider that handles the given credential type.
func (m *authnProviderManager) InitiateAuthentication(ctx context.Context, credentialType string,
	initData any, metadata *providers.AuthnMetadata) (any, *tidcommon.ServiceError) {
	_, selectedProvider, svcErr := m.selectProvider(ctx, []string{credentialType})
	if svcErr != nil {
		return nil, svcErr
	}
	return selectedProvider.InitiateAuthentication(ctx, credentialType, initData, metadata)
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

	selectedProviderName, selectedProvider, svcErr := m.selectProvider(ctx, slices.Sorted(maps.Keys(credentials)))
	if svcErr != nil {
		return authUser, nil, svcErr
	}

	authResult, svcErr := selectedProvider.Authenticate(ctx, identifiers, credentials, metadata)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			m.logger.Error(ctx, "provider returned server error during authentication",
				log.String("error", svcErr.ErrorDescription.DefaultValue))
			return authUser, nil, &tidcommon.InternalServerError
		}
		if _, federated := credentials[authnprovidercm.CredentialTypeFederated]; federated &&
			svcErr.Code == authnprovidercm.ErrorCodeAmbiguousUser {
			// Two entities hold the recorded link. Reported as such so the caller does not read it as
			// a failed exchange with the connection.
			m.logger.Debug(ctx, "federated authentication resolved an ambiguous link",
				log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
			return authUser, nil, &ErrorAmbiguousUser
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
	if svcErr := m.checkSubjectAllowed(ctx, authResult.EntityReference); svcErr != nil {
		return authUser, nil, svcErr
	}

	authUser, svcErr = m.updateAuthUser(ctx, authResult, authUser, selectedProviderName)
	if svcErr != nil {
		return authUser, nil, svcErr
	}

	return authUser, authResult.AuthenticatedClaims, nil
}

// GetEntityReference resolves a single entity reference across all providers
// in the AuthUser. Each provider's pending entity-reference token is resolved
// through that provider; the resolved references must agree or the call fails.
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
			p, ok := m.authnProviders[name]
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

	if svcErr := m.checkSubjectAllowed(ctx, entityRef); svcErr != nil {
		return authUser, nil, svcErr
	}

	return authUser, entityRef, nil
}

// checkSubjectAllowed rejects a resolved subject whose category and type the application/ agent driving the
// current authentication does not accept. A nil reference means the subject is not resolved yet (the provider
// returned an entity reference token for an entity it has not provisioned), and the check applies once
// GetEntityReference resolves it.
func (m *authnProviderManager) checkSubjectAllowed(
	ctx context.Context, entityRef *providers.EntityReference) *tidcommon.ServiceError {
	if entityRef == nil {
		return nil
	}
	constraints, ok := authnprovidercm.SubjectTypeConstraintsFrom(ctx)
	if !ok || constraints.PermitsSubject(
		providers.EntityCategory(entityRef.EntityCategory), entityRef.EntityType) {
		return nil
	}
	m.logger.Debug(ctx, "resolved subject is not allowed for the application",
		log.String("entityId", entityRef.EntityID),
		log.String("entityCategory", entityRef.EntityCategory),
		log.String("entityType", entityRef.EntityType))
	return &ErrorSubjectNotAllowed
}

// maxLinkingCandidates bounds how many accounts one federated identity can be offered to verify.
// More than this means a linking attribute that barely tells accounts apart, which is a
// misconfiguration to fail on rather than a choice to carry through the flow.
const maxLinkingCandidates = 10

// entityReferenceSearcher is what a provider implements when it can list every entity an attribute
// lookup matches. Only the default provider does, so a lookup matching several entities at any
// other provider still fails closed.
type entityReferenceSearcher interface {
	SearchEntityReferences(ctx context.Context, filters map[string]interface{}) (
		[]providers.EntityReference, *tidcommon.ServiceError)
}

// ResolveFederatedCandidates returns the entities the pending federated identity's
// account-linking attributes name, with the values they matched on, or nil when the AuthUser
// carries no pending federated identity or nothing matches it.
//
// A match is a name, not a proof, which is why nothing here touches the AuthUser: the flow has the
// End-User verify one of the accounts, and that verification is what authenticates them. Every
// lookup is tried, and every entity they name counts, whether different lookups name different
// entities or one lookup names several: verification is what tells them apart.
func (m *authnProviderManager) ResolveFederatedCandidates(ctx context.Context,
	authUser providers.AuthUser) (*providers.FederatedCandidates, *tidcommon.ServiceError) {
	for _, name := range authUser.ProviderNames() {
		state, _ := authUser.StateFor(name)
		filters := accountLinkingFilters(state.EntityReferenceToken)
		if len(filters) == 0 {
			continue
		}
		p, ok := m.authnProviders[name]
		if !ok || p == nil {
			m.logger.Error(ctx, "no provider registered for authUser state entry",
				log.String("providerName", name))
			return nil, &tidcommon.InternalServerError
		}
		return m.matchAccountLinkingFilters(ctx, p, filters)
	}
	return nil, nil
}

// matchAccountLinkingFilters resolves each account-linking filter through the provider and returns
// the entities they name, with the attribute values that matched.
func (m *authnProviderManager) matchAccountLinkingFilters(ctx context.Context,
	p providers.AuthnProviderInterface, filters []map[string]interface{},
) (*providers.FederatedCandidates, *tidcommon.ServiceError) {
	var candidates *providers.FederatedCandidates
	for _, filter := range filters {
		// A filter carries no federated keys, so the provider resolves it the way it resolves any
		// other attribute token: as a lookup on indexed attributes.
		ids, svcErr := m.matchAccountLinkingFilter(ctx, p, filter)
		if svcErr != nil {
			return nil, svcErr
		}
		if len(ids) == 0 {
			continue
		}

		if candidates == nil {
			candidates = &providers.FederatedCandidates{MatchedAttributes: map[string]string{}}
		}
		for _, id := range ids {
			if !slices.Contains(candidates.EntityIDs, id) {
				candidates.EntityIDs = append(candidates.EntityIDs, id)
			}
		}
		for attr, value := range filter {
			candidates.MatchedAttributes[attr] = systemutils.ConvertInterfaceValueToString(value)
		}
	}
	if candidates == nil {
		m.logger.Debug(ctx, "no user matches the account linking attributes")
		return nil, nil
	}
	if len(candidates.EntityIDs) > maxLinkingCandidates {
		m.logger.Debug(ctx, "account linking attributes match too many users",
			log.Int("candidateCount", len(candidates.EntityIDs)))
		return nil, &ErrorAmbiguousUser
	}
	slices.Sort(candidates.EntityIDs)
	return candidates, nil
}

// matchAccountLinkingFilter returns the entities one account-linking filter names. The provider
// resolves the filter as it resolves any lookup, and only an ambiguous answer is listed through
// the provider's search, which leaves a lookup that names one entity or none exactly as it was.
func (m *authnProviderManager) matchAccountLinkingFilter(ctx context.Context,
	p providers.AuthnProviderInterface, filter map[string]interface{}) ([]string, *tidcommon.ServiceError) {
	ref, svcErr := p.GetEntityReference(ctx, filter)
	if svcErr == nil {
		if ref == nil || ref.EntityID == "" {
			return nil, nil
		}
		return []string{ref.EntityID}, nil
	}
	if svcErr.Type == tidcommon.ServerErrorType {
		m.logger.Error(ctx, "provider returned server error while matching account linking attributes",
			log.String("error", svcErr.ErrorDescription.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}
	switch svcErr.Code {
	case authnprovidercm.ErrorCodeUserNotFound:
		return nil, nil
	case authnprovidercm.ErrorCodeAmbiguousUser:
		return m.listAmbiguousMatch(ctx, p, filter)
	default:
		m.logger.Error(ctx, "provider rejected the account linking attribute lookup",
			log.String("errorCode", svcErr.Code),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}
}

// listAmbiguousMatch lists the entities a lookup the provider found ambiguous matches. A provider
// that cannot list them, or a search that does not find several, leaves nothing to verify against,
// and the lookup fails closed.
func (m *authnProviderManager) listAmbiguousMatch(ctx context.Context,
	p providers.AuthnProviderInterface, filter map[string]interface{}) ([]string, *tidcommon.ServiceError) {
	searcher, ok := p.(entityReferenceSearcher)
	if !ok {
		m.logger.Debug(ctx, "an account linking attribute matches more than one user")
		return nil, &ErrorAmbiguousUser
	}
	refs, svcErr := searcher.SearchEntityReferences(ctx, filter)
	if svcErr != nil {
		m.logger.Error(ctx, "provider failed to list the users an account linking attribute matches",
			log.String("errorCode", svcErr.Code))
		return nil, &tidcommon.InternalServerError
	}
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.EntityID != "" {
			ids = append(ids, ref.EntityID)
		}
	}
	if len(ids) < 2 {
		m.logger.Debug(ctx, "an ambiguous account linking attribute did not list several users",
			log.Int("candidateCount", len(ids)))
		return nil, &ErrorAmbiguousUser
	}
	return ids, nil
}

// accountLinkingFilters returns the account-linking filters carried by an unresolved federated
// token. It returns nil for any other token, and for a federated token that carries none, which
// means the identity has nothing to match an existing user on.
//
// The token is persisted with the flow, so the list comes back from JSON as []interface{} as well
// as in the shape it was built in.
func accountLinkingFilters(entityReferenceToken any) []map[string]interface{} {
	if !authnprovidercm.IsFederatedToken(entityReferenceToken) {
		return nil
	}
	token, _ := entityReferenceToken.(map[string]interface{})

	switch raw := token[authnprovidercm.AccountLinkingFiltersKey].(type) {
	case []map[string]interface{}:
		return raw
	case []interface{}:
		filters := make([]map[string]interface{}, 0, len(raw))
		for _, entry := range raw {
			if filter, ok := entry.(map[string]interface{}); ok && len(filter) > 0 {
				filters = append(filters, filter)
			}
		}
		return filters
	default:
		return nil
	}
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
		p, ok := m.authnProviders[name]
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

// InitiateEnrollment routes an enrollment-initiation request to the provider that handles the given credential type.
func (m *authnProviderManager) InitiateEnrollment(ctx context.Context, credentialType string,
	initData any, metadata *providers.AuthnMetadata) (any, *tidcommon.ServiceError) {
	_, selectedProvider, svcErr := m.selectProvider(ctx, []string{credentialType})
	if svcErr != nil {
		return nil, svcErr
	}
	return selectedProvider.InitiateEnrollment(ctx, credentialType, initData, metadata)
}

// Enroll routes a credential to the matching provider to complete enrollment and merges
// the provider's result into the AuthUser under the provider's name.
func (m *authnProviderManager) Enroll(ctx context.Context, identifiers, credentials map[string]interface{},
	requestedAttributes *providers.RequestedAttributes,
	metadata *providers.AuthnMetadata,
	authUser providers.AuthUser) (providers.AuthUser, providers.AuthenticatedClaims, *tidcommon.ServiceError) {
	selectedProviderName, selectedProvider, svcErr := m.selectProvider(ctx, slices.Sorted(maps.Keys(credentials)))
	if svcErr != nil {
		return authUser, nil, svcErr
	}

	authResult, svcErr := selectedProvider.Enroll(ctx, identifiers, credentials, metadata)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			m.logger.Error(ctx, "provider returned server error during enrollment",
				log.String("error", svcErr.ErrorDescription.DefaultValue))
			return authUser, nil, &tidcommon.InternalServerError
		}
		switch svcErr.Code {
		case authnprovidercm.ErrorCodeUserNotFound:
			m.logger.Debug(ctx, "enrollment failed with user not found error from provider",
				log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
			return authUser, nil, &ErrorUserNotFound
		case authnprovidercm.ErrorCodeInvalidRequest:
			m.logger.Debug(ctx, "enrollment failed with invalid request error from provider",
				log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
			return authUser, nil, &ErrorInvalidRequest
		default:
			m.logger.Debug(ctx, "enrollment failed with client error from provider",
				log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
			return authUser, nil, &ErrorEnrollmentFailed
		}
	}
	if svcErr := m.checkSubjectAllowed(ctx, authResult.EntityReference); svcErr != nil {
		return authUser, nil, svcErr
	}

	authUser, svcErr = m.updateAuthUser(ctx, authResult, authUser, selectedProviderName)
	if svcErr != nil {
		return authUser, nil, svcErr
	}

	return authUser, authResult.AuthenticatedClaims, nil
}

// LinkFederatedIdentity records a federated identity against the user this AuthUser names, on the
// provider that authenticated them. The provider holding the user is the provider that stores the
// link, so a user held by an external provider is never linked into ThunderID's own store.
func (m *authnProviderManager) LinkFederatedIdentity(ctx context.Context, authUser providers.AuthUser,
	idpID, sub string) *tidcommon.ServiceError {
	if idpID == "" || sub == "" {
		m.logger.Debug(ctx, "link requested without a connection id or subject")
		return &ErrorInvalidRequest
	}

	providerName, ok := m.linkTargetProvider(authUser)
	if !ok {
		m.logger.Debug(ctx, "link requested but the authUser carries no provider state")
		return &ErrorAuthenticationFailed
	}

	selectedProvider, ok := m.authnProviders[providerName]
	if !ok || selectedProvider == nil {
		m.logger.Error(ctx, "authUser state names a provider that is not registered",
			log.String("providerName", providerName))
		return &tidcommon.InternalServerError
	}

	state, _ := authUser.StateFor(providerName)
	token := state.EntityReferenceToken
	if token == nil {
		// GetEntityReference replaces a resolved token with the reference itself, so by the time a
		// flow reaches the link write the handle is usually the entity id.
		if state.EntityReference == nil || state.EntityReference.EntityID == "" {
			m.logger.Debug(ctx, "link requested but the provider state names no entity")
			return &ErrorInvalidRequest
		}
		token = map[string]interface{}{authnprovidercm.UserAttributeUserID: state.EntityReference.EntityID}
	}

	svcErr := selectedProvider.LinkFederatedIdentity(ctx, token, idpID, sub)
	if svcErr == nil {
		return nil
	}
	if svcErr.Type == tidcommon.ServerErrorType {
		m.logger.Error(ctx, "provider returned server error while linking a federated identity",
			log.String("error", svcErr.ErrorDescription.DefaultValue))
		return &tidcommon.InternalServerError
	}
	if svcErr.Code == authnprovidercm.ErrorCodeInvalidRequest {
		// The provider does not store federated links. There is nothing to record, and failing the
		// sign-in over it would break every federated authentication through that provider.
		m.logger.Debug(ctx, "provider does not store federated identity links",
			log.String("providerName", providerName))
		return nil
	}
	m.logger.Debug(ctx, "provider rejected the federated identity link",
		log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
	return &ErrorLinkFederatedIdentityFailed
}

// linkTargetProvider picks the provider that should store a federated link. With one provider in
// the AuthUser it is that one. With several, which happens once a password authentication follows
// the federated one, it is whichever provider claimed the federated credential.
func (m *authnProviderManager) linkTargetProvider(authUser providers.AuthUser) (string, bool) {
	names := authUser.ProviderNames()
	switch len(names) {
	case 0:
		return "", false
	case 1:
		return names[0], true
	}

	federatedOwner, ok := m.credToProviderMapping[authnprovidercm.CredentialTypeFederated]
	if !ok {
		federatedOwner = defaultProviderName
	}
	if _, present := authUser.StateFor(federatedOwner); present {
		return federatedOwner, true
	}
	return names[0], true
}

// updateAuthUser records a provider's authentication or enrollment result in the AuthUser
// under the selected provider's name.
func (m *authnProviderManager) updateAuthUser(ctx context.Context, authResult *providers.AuthnResult,
	authUser providers.AuthUser, selectedProviderName string) (providers.AuthUser, *tidcommon.ServiceError) {
	if (authResult.AttributeToken == nil && authResult.Attributes == nil) ||
		(authResult.EntityReferenceToken == nil && authResult.EntityReference == nil) {
		m.logger.Error(ctx, "provider result is missing a required entity reference or attribute value")
		return authUser, &tidcommon.InternalServerError
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
	return authUser, nil
}

// selectProvider resolves the supplied credential keys to a single provider. The request is valid as long
// as every key resolves to the same provider; keys that fan out to different providers are ambiguous
// and treated as an internal fault. At least one key must be supplied.
func (m *authnProviderManager) selectProvider(ctx context.Context, credentialTypes []string) (
	string, providers.AuthnProviderInterface, *tidcommon.ServiceError) {
	if len(credentialTypes) == 0 {
		m.logger.Error(ctx, "no credential keys supplied; cannot select a provider")
		return "", nil, &tidcommon.InternalServerError
	}

	selectedProviderName := ""
	for i, credentialType := range credentialTypes {
		providerName, ok := m.credToProviderMapping[credentialType]
		if !ok {
			// Credentials not claimed by a custom provider fall through to the default provider.
			providerName = defaultProviderName
		}
		if i == 0 {
			selectedProviderName = providerName
		} else if providerName != selectedProviderName {
			m.logger.Error(ctx, "credential keys map to multiple providers; rejecting ambiguous request",
				log.Any("credentialKeys", credentialTypes))
			return "", nil, &tidcommon.InternalServerError
		}
	}

	selectedProvider, ok := m.authnProviders[selectedProviderName]
	if !ok || selectedProvider == nil {
		m.logger.Error(ctx, "credential key mapped to a provider that is not registered",
			log.String("providerName", selectedProviderName))
		return "", nil, &tidcommon.InternalServerError
	}
	return selectedProviderName, selectedProvider, nil
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
