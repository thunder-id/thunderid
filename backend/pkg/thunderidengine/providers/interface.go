// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package providers provides interfaces for the providers module.
package providers

import (
	"context"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// AuthnProviderManager defines the interface for the authentication provider manager.
type AuthnProviderManager interface {
	InitiateAuthentication(ctx context.Context, credentialType string, initData any,
		metadata *AuthnMetadata) (any, *common.ServiceError)
	AuthenticateUser(ctx context.Context, identifiers, credentials map[string]interface{},
		requestedAttributes *RequestedAttributes,
		metadata *AuthnMetadata,
		authUser AuthUser) (AuthUser, AuthenticatedClaims, *common.ServiceError)
	InitiateEnrollment(ctx context.Context, credentialType string, initData any,
		metadata *AuthnMetadata) (any, *common.ServiceError)
	Enroll(ctx context.Context, identifiers, credentials map[string]interface{},
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

// AuthnProviderInterface defines the interface for authentication providers.
type AuthnProviderInterface interface {
	InitiateAuthentication(ctx context.Context, credentialType string, initData any,
		metadata *AuthnMetadata) (any, *common.ServiceError)
	Authenticate(ctx context.Context, identifiers, credentials map[string]interface{},
		metadata *AuthnMetadata) (*AuthnResult, *common.ServiceError)
	GetEntityReference(ctx context.Context, entityReferenceToken any) (*EntityReference,
		*common.ServiceError)
	GetAttributes(ctx context.Context, attributeToken any, consentedAttributes *RequestedAttributes,
		metadata *GetAttributesMetadata) (
		*AttributesResponse, *common.ServiceError)
	InitiateEnrollment(ctx context.Context, credentialType string, initData any,
		metadata *AuthnMetadata) (any, *common.ServiceError)
	Enroll(ctx context.Context, identifiers, credentials map[string]interface{},
		metadata *AuthnMetadata) (*AuthnResult, *common.ServiceError)
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
	GetActorRoles(actorID string, groupIDs []string) ([]string, *common.ServiceError)
}

// AgentMgtProvider provisions agents on behalf of runtime capabilities. The rules and semantics of an
// agent remain owned by the agent management service; this exposes only what the runtime needs.
type AgentMgtProvider interface {
	// CreateAgent provisions an agent. The returned agent carries the generated identifier and, on
	// InboundAuthConfig, the generated client credentials, alongside the request fields the agent
	// service echoes on a create response. Fields it does not echo come back zero, so the result is
	// not a full round-trip of the request. Owner is required: unlike the Agent API, the runtime
	// carries no dependable caller identity to fall back on. Errors from the agent service are
	// returned unchanged so callers can distinguish the actual failure.
	CreateAgent(ctx context.Context, agent *Agent) (*Agent, *common.ServiceError)
}

// UserMgtProvider provisions users on behalf of runtime capabilities. The rules and semantics of a
// user remain owned by the user management service; this exposes only what the runtime needs.
type UserMgtProvider interface {
	// CreateUser provisions a user and returns it with its generated ID. Errors from the user
	// service are returned unchanged so callers can distinguish the actual failure.
	CreateUser(ctx context.Context, user *User) (*User, *common.ServiceError)
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

// ApplicationAdminProvider exposes the application operations an administration flow performs. It is a
// seam rather than a direct dependency because the application service is built after the flow
// executors and sits behind them in the import graph.
type ApplicationAdminProvider interface {
	// ValidateDeleteApplication reports whether the application may be deleted, and returns the profile
	// of the artifacts it has issued. It changes no state.
	ValidateDeleteApplication(ctx context.Context, appID string) (
		*ApplicationArtifactProfile, *common.ServiceError)
	// DeleteApplication deletes the application.
	DeleteApplication(ctx context.Context, appID string) *common.ServiceError
	// ValidateCredentialAction reports whether the action may be performed on the application's
	// credential, and returns the profile of the artifacts it has issued. It changes no state.
	ValidateCredentialAction(ctx context.Context, appID string, action CredentialAction) (
		*ApplicationArtifactProfile, *common.ServiceError)
	// ApplyCredentialAction performs the action and returns the new credential value. That return is
	// the only time the value is readable: it is hashed on write and no read path returns it.
	ApplyCredentialAction(ctx context.Context, appID string, action CredentialAction) (
		string, *common.ServiceError)
}

// ResourceServerProvider defines the interface for the resource provider.
type ResourceServerProvider interface {
	GetResourceServerByIdentifier(
		ctx context.Context, identifier string,
	) (*ResourceServer, *common.ServiceError)
	GetResourceServer(
		ctx context.Context, id string,
	) (*ResourceServer, *common.ServiceError)
	ValidatePermissions(
		ctx context.Context, resourceServerID string, permissions []string,
	) ([]string, *common.ServiceError)
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
		runtimeMetadata map[string][]string) (
		*ConsentPromptData, *common.ServiceError)

	// RecordConsent records the user's consent decisions and returns the persisted consent record.
	// If the user denied any essential attribute, ErrorEssentialConsentDenied is returned.
	RecordConsent(ctx context.Context, ouID, appID, userID string,
		decisions *ConsentDecisions, sessionToken string, validityPeriod int64,
		runtimeMetadata map[string][]string) (
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

// AttestationProvider verifies a platform attestation token (e.g. a Google Play Integrity token)
// against an application's attestation configuration, proving the binary identity of a mobile
// client. It reports the verification outcome as a boolean rather than an error so that a definitive
// rejection (token invalid) is not mistaken for an operational failure (provider outage,
// misconfiguration): the latter is surfaced as a non-nil ServiceError.
type AttestationProvider interface {
	// Verify returns true when the token proves the expected binary identity. It returns false with
	// a nil error for a definitive rejection, and a non-nil ServiceError for an operational failure
	// that prevented verification from completing.
	Verify(ctx context.Context, cfg *AttestationConfig, token string) (bool, *common.ServiceError)
}

// RuntimeStoreProvider defines the interface for runtime store operations.
type RuntimeStoreProvider interface {
	// Put stores a value in the runtime store with the specified key and TTL (time-to-live) in seconds.
	Put(ctx context.Context, namespace RuntimeStoreNamespace, key string, value []byte, ttlSeconds int64) error

	// PutIfNotExists atomically stores a value only if the key does not already hold a non-expired
	// value. Returns true if the value was stored, false if an unexpired value already exists.
	PutIfNotExists(
		ctx context.Context, namespace RuntimeStoreNamespace, key string, value []byte, ttlSeconds int64,
	) (bool, error)

	// Get retrieves a value from the runtime store by its key.
	Get(ctx context.Context, namespace RuntimeStoreNamespace, key string) ([]byte, error)

	// Update updates the value associated with a key in the runtime store.
	Update(ctx context.Context, namespace RuntimeStoreNamespace, key string, value []byte) error

	// Delete removes a value from the runtime store by its key.
	Delete(ctx context.Context, namespace RuntimeStoreNamespace, key string) error

	// Take retrieves and removes a value from the runtime store by its key.
	Take(ctx context.Context, namespace RuntimeStoreNamespace, key string) ([]byte, error)

	ExtendTTL(ctx context.Context, namespace RuntimeStoreNamespace, key string, ttlSeconds int64) error

	// CompareFieldAndSwap atomically replaces the value at key with newValue, but only when the
	// top-level JSON string field in the currently stored value equals expected, preserving the
	// existing TTL. It returns true when the swap occurred, and false when the field differs or the
	// key is absent/expired. Callers use it for conditional state transitions (get, inspect, build
	// the new document, compare-and-swap) that Update cannot perform atomically. Stored values are
	// assumed to be JSON documents.
	CompareFieldAndSwap(
		ctx context.Context, namespace RuntimeStoreNamespace, key, field, expected string, newValue []byte,
	) (bool, error)
}

// Transactioner provides transaction management with automatic nesting detection.
type Transactioner interface {
	// Transact executes the given function within a transaction.
	// If a transaction already exists in the context, it reuses it.
	// Otherwise, it creates a new transaction and commits/rolls back automatically.
	Transact(ctx context.Context, txFunc func(context.Context) error) error
}

// RuntimeCryptoProvider provides asymmetric cryptographic operations including
// encryption, decryption, signing, verification, and key discovery.
type RuntimeCryptoProvider interface {
	//	 Encrypt encrypts the given content using the specified key reference, algorithm, and parameters.
	Encrypt(ctx context.Context, keyRef *KeyRef, algorithm string, params map[string]interface{},
		content []byte) ([]byte, *CryptoDetails, error)

	// Decrypt decrypts the given content using the specified key reference, algorithm, and parameters.
	Decrypt(ctx context.Context, keyRef *KeyRef, algorithm string, params map[string]interface{},
		content []byte) ([]byte, error)

	// Sign signs the given content using the specified key reference and algorithm.
	Sign(ctx context.Context, keyRef KeyRef, alg string, content []byte) ([]byte, error)

	// Verify verifies the signature of the given content using the specified key reference and algorithm.
	Verify(ctx context.Context, keyRef KeyRef, alg string, content, signature []byte) error

	// GetPublicKeys retrieves public keys based on the provided filter criteria.
	GetPublicKeys(ctx context.Context, filter PublicKeyFilter) ([]PublicKeyInfo, error)

	// GetSupportedSigningAlgorithms returns the list of signing algorithms supported by Sign and Verify.
	GetSupportedSigningAlgorithms() []string

	// GetSupportedEncryptionAlgorithms returns the list of algorithms supported by Encrypt and Decrypt.
	GetSupportedEncryptionAlgorithms() []string
}
