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

// Package providers provides interfaces for the providers module.
package providers

import (
	"context"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// AuthnProviderManager defines the interface for the authentication provider manager.
type AuthnProviderManager interface {
	AuthenticateUser(ctx context.Context, identifiers, credentials map[string]interface{},
		requestedAttributes *RequestedAttributes,
		metadata *AuthnMetadata,
		authUser AuthUser) (AuthUser, AuthenticatedClaims, *common.ServiceError)
	GetEntityReference(ctx context.Context, authUser AuthUser) (
		AuthUser, *EntityReference, *common.ServiceError)
	GetUserAvailableAttributes(ctx context.Context,
		authUser AuthUser) (*AttributesResponse, *common.ServiceError)
	GetUserAttributes(ctx context.Context,
		requestedAttributes *RequestedAttributes,
		metadata *GetAttributesMetadata,
		authUser AuthUser) (AuthUser, *AttributesResponse, *common.ServiceError)
}

// ActorProvider resolves inbound actors and exposes their OAuth and membership data.
type ActorProvider interface {
	GetOAuthClientByClientID(
		ctx context.Context, clientID string,
	) (*OAuthClient, *common.ServiceError)
	GetOAuthProfileByID(
		ctx context.Context, id string,
	) (*OAuthProfile, *common.ServiceError)
	GetInboundClientByID(
		ctx context.Context, id string,
	) (*InboundClient, *common.ServiceError)
	AuthenticateActor(
		ctx context.Context, identifiers, credentials map[string]interface{},
	) *common.ServiceError
	GetActor(actorID string) (*Entity, *common.ServiceError)
	GetActorGroups(actorID string) ([]EntityGroup, *common.ServiceError)
}

// I18nProvider defines the interface for the i18n provider.
type I18nProvider interface {
	ResolveTranslations(
		ctx context.Context,
		language string,
		namespace string,
	) (*LanguageTranslationsResponse, *common.ServiceError)
	ListLanguages(ctx context.Context) ([]string, *common.ServiceError)
}

// DesignProvider defines the interface for the design resolve service.
type DesignProvider interface {
	ResolveDesign(
		ctx context.Context, resolveType DesignResolveType, id string,
	) (*DesignResponse, *common.ServiceError)
}

// OrganizationUnitProvider defines the interface for the organization unit provider.
type OrganizationUnitProvider interface {
	GetOrganizationUnit(ctx context.Context, id string) (OrganizationUnit, *common.ServiceError)
	GetOrganizationUnitList(
		ctx context.Context, limit, offset int, f *common.FilterGroup,
	) (*OrganizationUnitListResponse, *common.ServiceError)
	CreateOrganizationUnit(
		ctx context.Context, request OrganizationUnitRequestWithID,
	) (OrganizationUnit, *common.ServiceError)
	IsParent(ctx context.Context, parentID, childID string) (bool, *common.ServiceError)
	IsOrganizationUnitExists(ctx context.Context, id string) (bool, *common.ServiceError)
	GetOrganizationUnitChildren(
		ctx context.Context, id string, limit, offset int, f *common.FilterGroup,
	) (*OrganizationUnitListResponse, *common.ServiceError)
}

// FlowProvider defines the flow management operations required for flow execution.
type FlowProvider interface {
	GetFlowByHandle(ctx context.Context, handle string, flowType FlowType) (
		*CompleteFlowDefinition, *common.ServiceError)
	GetFlow(ctx context.Context, flowID string) (*CompleteFlowDefinition, *common.ServiceError)
}

// ResourceServerProvider defines the interface for the resource provider.
type ResourceServerProvider interface {
	GetResourceServerByIdentifier(
		ctx context.Context, identifier string,
	) (*ResourceServer, *common.ServiceError)
	ValidatePermissions(
		ctx context.Context, resourceServerID string, permissions []string,
	) ([]string, *common.ServiceError)
	FindResourceServersByPermissions(
		ctx context.Context, permissions []string,
	) ([]ResourceServer, *common.ServiceError)
}

// IDPProvider defines the interface for the identity provider provider.
type IDPProvider interface {
	GetIdentityProvidersByProperty(ctx context.Context, propertyKey,
		propertyValue string) ([]IDPDTO, *common.ServiceError)
	GetIdentityProvider(ctx context.Context, idpID string) (*IDPDTO, *common.ServiceError)
}

// ConsentProvider provides functionality to resolve consent requirements and
// record user consent decisions during runtime authentication flows.
type ConsentProvider interface {
	// ResolveConsent checks whether the user has provided required consents for the given
	// application, attribute set, and authorized permission set. Returns nil if all required
	// consents are active; otherwise returns ConsentPromptData describing which purposes /
	// elements still need user consent. When forceReprompt is true, consent is re-prompted for
	// all required claims regardless of existing active consent.
	ResolveConsent(ctx context.Context, ouID, appID, appName, userID string,
		essentialAttributes, optionalAttributes, authorizedPermissions []string,
		availableAttributes *AttributesResponse, forceReprompt bool,
		runtimeMetadata map[string]string) (
		*ConsentPromptData, *common.ServiceError)

	// RecordConsent records the user's consent decisions and returns the persisted consent record.
	// If the user denied any essential attribute, ErrorEssentialConsentDenied is returned.
	RecordConsent(ctx context.Context, ouID, appID, userID string,
		decisions *ConsentDecisions, sessionToken string, validityPeriod int64,
		runtimeMetadata map[string]string) (
		*Consent, *common.ServiceError)
}

// CaptchaValidationProvider defines the contract for verifying captcha tokens.
type CaptchaValidationProvider interface {
	// Verify validates the given captcha token and returns the verification result. An invalid
	// token is reported through the result's negative verdict, while operational failures (provider
	// unavailable or misconfigured) are returned as a server-side service error.
	Verify(ctx context.Context, token string) (*CaptchaVerificationResult, *common.ServiceError)
}

// Executor defines the interface for executors.
type Executor interface {
	Execute(ctx *NodeContext) (*ExecutorResponse, error)
	GetName() string
	GetType() ExecutorType
	GetDefaultInputs() []Input
	GetPrerequisites() []Input
	HasRequiredInputs(ctx *NodeContext, execResp *ExecutorResponse) bool
	ValidatePrerequisites(ctx *NodeContext, execResp *ExecutorResponse,
		authnProvider AuthnProviderManager) bool
	GetUserIDFromContext(ctx *NodeContext, execResp *ExecutorResponse,
		authnProvider AuthnProviderManager) string
	GetRequiredInputs(ctx *NodeContext) []Input
	GetExecutionPolicy(mode string) *ExecutionPolicy
	GetMeta() *ExecutorMeta
}

// ObservabilityProvider defines the interface for the observability provider.
type ObservabilityProvider interface {
	// PublishEvent publishes an event to the observability system.
	// This is a no-op if observability is disabled.
	// The context carries the request trace ID used for correlated logging.
	PublishEvent(ctx context.Context, evt *Event)

	// IsEnabled returns true if observability is enabled and operational.
	IsEnabled() bool
}

// AuthorizationProvider defines the interface for authorization operations.
// This is the public interface exposed to external consumers.
type AuthorizationProvider interface {
	// EvaluateAccess evaluates a single fine-grained access request.
	EvaluateAccess(
		ctx context.Context,
		request AccessEvaluationRequest,
	) (*AccessEvaluationResponse, *common.ServiceError)

	// EvaluateAccessBatch evaluates multiple fine-grained access requests.
	EvaluateAccessBatch(
		ctx context.Context,
		request AccessEvaluationsRequest,
	) (*AccessEvaluationsResponse, *common.ServiceError)
}

// RuntimeStoreProvider defines the interface for runtime store operations.
type RuntimeStoreProvider interface {
	// Put stores a value in the runtime store with the specified key and TTL (time-to-live) in seconds.
	Put(ctx context.Context, namespace RuntimeStoreNamespace, key string, value []byte, ttlSeconds int64) error

	// Get retrieves a value from the runtime store by its key.
	Get(ctx context.Context, namespace RuntimeStoreNamespace, key string) ([]byte, error)

	// Update updates the value associated with a key in the runtime store.
	Update(ctx context.Context, namespace RuntimeStoreNamespace, key string, value []byte) error

	// Delete removes a value from the runtime store by its key.
	Delete(ctx context.Context, namespace RuntimeStoreNamespace, key string) error

	// Take retrieves and removes a value from the runtime store by its key.
	Take(ctx context.Context, namespace RuntimeStoreNamespace, key string) ([]byte, error)

	ExtendTTL(ctx context.Context, namespace RuntimeStoreNamespace, key string, ttlSeconds int64) error
}
