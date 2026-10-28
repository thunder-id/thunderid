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
	// ResolveLinkCandidates returns the entities the AuthUser's pending identity matches on its
	// account-linking attributes, or nil when nothing matches.
	ResolveLinkCandidates(ctx context.Context, authUser AuthUser) (
		*LinkCandidates, *common.ServiceError)
	GetUserAvailableAttributes(ctx context.Context,
		authUser AuthUser) (*AttributesResponse, *common.ServiceError)
	GetUserAttributes(ctx context.Context,
		requestedAttributes *RequestedAttributes,
		metadata *GetAttributesMetadata,
		authUser AuthUser) (AuthUser, *AttributesResponse, *common.ServiceError)
	// LinkAccount records a linked account against the user the AuthUser names, on the provider
	// that authenticated that user.
	LinkAccount(ctx context.Context, authUser AuthUser,
		idpID, sub string) *common.ServiceError
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
	// StoreAccountLink records that the referenced entity authenticates as the given subject at the
	// given connection. The token is this provider's entity reference token, or {"userID": "<entity id>"}
	// once the entity is resolved. A provider that does not store links returns ErrorCodeNotImplemented.
	StoreAccountLink(ctx context.Context, entityReferenceToken any,
		idpID, sub string) *common.ServiceError
	// SearchEntityReferences returns every entity an attribute lookup matches. A provider that cannot
	// list the matches returns ErrorCodeNotImplemented.
	SearchEntityReferences(ctx context.Context, filters map[string]interface{}) (
		[]EntityReference, *common.ServiceError)
}

// ActorProvider resolves inbound actors and exposes their OAuth and membership data.
type ActorProvider interface {
	GetOAuthClientByClientID(
		ctx context.Context, clientID string,
	) (*OAuthClient, *common.ServiceError)
	// GetOAuthClientByID returns the runtime OAuth client of the application or agent with the given
	// entity id, so a caller that knows only the application can address its OAuth configuration. A
	// missing client is a nil client with no error, as in GetOAuthClientByClientID.
	GetOAuthClientByID(
		ctx context.Context, id string,
	) (*OAuthClient, *common.ServiceError)
	GetOAuthProfileByID(
		ctx context.Context, id string,
	) (*OAuthProfile, *common.ServiceError)
	GetInboundClientByID(
		ctx context.Context, id string,
	) (*InboundClient, *common.ServiceError)
	IsOAuthClientAccessibleFromOU(ctx context.Context, client *OAuthClient, ouID string) (bool, *common.ServiceError)
	AuthenticateActor(
		ctx context.Context, identifiers, credentials map[string]interface{},
	) *common.ServiceError
	GetActor(ctx context.Context, actorID string) (*Entity, *common.ServiceError)
	GetActorGroups(ctx context.Context, actorID string) ([]EntityGroup, *common.ServiceError)
	GetActorRoles(ctx context.Context, actorID string, groupIDs []string) ([]string, *common.ServiceError)
	GetTransitiveGroupAncestors(ctx context.Context, groupID string) ([]string, *common.ServiceError)
}

// AgentMgtProvider provisions agents on behalf of runtime capabilities. The rules and semantics of an
// agent remain owned by the agent management service; this exposes only what the runtime needs.
type AgentMgtProvider interface {
	// CreateAgent provisions the supplied agent through the agent management service, which applies
	// its OAuth defaults and validates the agent. Agent.Delegated selects the grant types the agent
	// is given and allowed.
	//
	// The returned agent carries the generated identifier and, on InboundAuthConfig, the generated
	// client credentials. Fields the agent service does not echo come back zero, so the result is
	// not a full round-trip. Owner falls back to the authenticated caller when unset.
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
	// GetSystemFlow returns the system flow with the given handle. A provider that serves no system
	// flows returns a not-found error.
	GetSystemFlow(ctx context.Context, handle string) (*CompleteFlowDefinition, *common.ServiceError)
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
	GetDirectAuthorizationTargets(
		ctx context.Context, idp *IDPDTO, claims map[string]interface{},
	) ([]AuthorizationTarget, *common.ServiceError)
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

// AuthorizationDetailTypeProvider provides the registered RFC 9396 authorization details types.
type AuthorizationDetailTypeProvider interface {
	// GetAuthorizationDetailTypeByValue returns the type registered under the given value, or nil if none.
	GetAuthorizationDetailTypeByValue(ctx context.Context, value string) (
		*AuthorizationDetailType, *common.ServiceError)
	// ListAllAuthorizationDetailTypes returns every registered type.
	ListAllAuthorizationDetailTypes(ctx context.Context) ([]AuthorizationDetailType, *common.ServiceError)
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

// IdentityGovernanceProvider checks access, records granted activity, and applies lifecycle operations.
type IdentityGovernanceProvider interface {
	// EvaluateAccess reports whether the identity may be used at the plane and authentication method;
	// a refusal names its hold in the decision. An empty or unknown entityID is admitted. An error
	// means the provider could not decide, and the caller must refuse.
	EvaluateAccess(ctx context.Context, entityID string, plane AccessPlane,
		method AuthenticationMethod) (AccessDecision, *common.ServiceError)

	// AdmitSignIn is EvaluateAccess at AccessPlaneSession for a completing sign-in. When it admits, it
	// also clears a lapsed account-wide failure record.
	AdmitSignIn(ctx context.Context, entityID string) (AccessDecision, *common.ServiceError)

	// RecordLogin stamps when the identity was last granted access. Callers do not fail the grant on
	// its error.
	RecordLogin(ctx context.Context, entityID string) *common.ServiceError

	// Unlock releases every automatic lock. It does not release a suspension.
	Unlock(ctx context.Context, entityID string) *common.ServiceError

	// Suspend places an administrative hold. It writes state only, so the caller composes revocation,
	// and writes nothing for an identity already suspended. operatorNote is optional, at most 100
	// characters, and never disclosed.
	Suspend(ctx context.Context, entityID, operatorNote string) *common.ServiceError

	// ValidateSuspend checks that the caller may suspend the identity with operatorNote, and writes
	// nothing. An identity already suspended passes.
	ValidateSuspend(ctx context.Context, entityID, operatorNote string) *common.ServiceError

	// Unsuspend releases the hold. A user is left with a non-expiring account-wide lock, which a
	// recovery or an unlock clears; an agent keeps any automatic lock still in force, and an
	// application becomes active.
	Unsuspend(ctx context.Context, entityID string) *common.ServiceError
}
