// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package common defines shared constants for authentication providers.
package common

const (
	// UserAttributeUserID is the attribute key used to identify the user ID.
	UserAttributeUserID = "userID"
	// UserAttributeSub is the attribute key used to identify the subject (sub) claim in authentication results.
	UserAttributeSub = "sub"
	// UserAttributeFederatedIdpID is the token key carrying the identity provider connection a
	// federated authentication ran against. Paired with UserAttributeSub it names a federated
	// identity; neither key is a credential type and nothing dispatches on them.
	UserAttributeFederatedIdpID = "federatedIdpId"
	// AccountLinkingFiltersKey is the federated token key carrying the connection's account-linking
	// lookups: an ordered list of attribute filters, any one of which may name the local user. Each
	// filter is a flat map of local attribute to value whose entries must all match.
	AccountLinkingFiltersKey = "accountLinkingFilters"
)

// IsFederatedToken reports whether a token names a federated identity. Both halves are required,
// because a subject is unique only within the connection that issued it.
func IsFederatedToken(token any) bool {
	parsed, ok := token.(map[string]interface{})
	if !ok {
		return false
	}
	idpID, _ := parsed[UserAttributeFederatedIdpID].(string)
	sub, _ := parsed[UserAttributeSub].(string)
	return idpID != "" && sub != ""
}

// SystemAttrCredentialUpdatedAt is the entity system-attribute key recording when the entity's
// authentication credential last changed: a user's password, or a client secret.
const SystemAttrCredentialUpdatedAt = "credentialUpdatedAt" // #nosec G101 -- attribute key, not a secret

// SystemAttrLinkedIDs is the entity system-attribute key holding the entity's links to federated
// identity provider subjects, keyed by connection ID and then by subject, each subject holding
// that link's own data:
//
//	{"linkedIds": {"<idpId>": {"<sub>": {}}}}
//
// One connection can hold several subjects, so a user may link more than one account per
// connection. Each subject indexes as its own ENTITY_IDENTIFIER row under the name
// "linkedIds.<idpId>", which the identifier table's multi-value primary key allows. Only the
// subject is indexed; the per-link object is not.
//
// Server-owned, like SystemAttrCredentialUpdatedAt: it is preserved across wholesale system
// attribute replacements and never surfaced through the user API.
const SystemAttrLinkedIDs = "linkedIds"

// Credential type keys used in the credentials map passed to the authentication providers.
const (
	// CredentialTypeProvisionedEntityID identifies an entity provisioned earlier in the same flow.
	CredentialTypeProvisionedEntityID = "provisionedEntityID"
	// CredentialTypePasskey identifies a passkey credential.
	CredentialTypePasskey = "passkey"
	// CredentialTypeOTP identifies a one time password.
	CredentialTypeOTP = "otp"
	// CredentialTypeFederated identifies an authorization code from an external identity provider.
	CredentialTypeFederated = "federated"
	// CredentialTypeMagicLink identifies a magic link token.
	CredentialTypeMagicLink = "magiclink"
	// CredentialTypeOpenID4VP identifies a verifiable presentation.
	CredentialTypeOpenID4VP = "openid4vp"
	// CredentialTypeClientSecret is the client secret of an application or agent.
	CredentialTypeClientSecret = "clientSecret"
	// CredentialTypeFlowSecret is the flow secret authenticating flow initiation.
	CredentialTypeFlowSecret = "flowSecret"
)

// InternalCredentialTypes are the credential types providers dispatch on by key name. Every
// dispatched type belongs here, otherwise an API client can select that authentication mechanism
// directly and bypass the flow that owns it.
var InternalCredentialTypes = []string{
	CredentialTypeProvisionedEntityID,
	CredentialTypePasskey,
	CredentialTypeOTP,
	CredentialTypeFederated,
	CredentialTypeMagicLink,
	CredentialTypeOpenID4VP,
	// The provider manager dispatches on sub to disambiguate after a federated step.
	UserAttributeSub,
}

// SystemCredentialTypes are machine credentials of an application or agent. The entity layer
// verifies credentials by key name across the merged schema and system credentials, so these
// authenticate the owning entity through any credential verification path.
var SystemCredentialTypes = []string{
	CredentialTypeClientSecret,
	CredentialTypeFlowSecret,
}

// FindReservedCredentialType returns the first credential type reserved for internal use, and
// whether one was found. Callers accepting a client supplied credentials map must reject on true.
func FindReservedCredentialType(credentials map[string]interface{}) (string, bool) {
	for _, reserved := range InternalCredentialTypes {
		if _, ok := credentials[reserved]; ok {
			return reserved, true
		}
	}
	for _, reserved := range SystemCredentialTypes {
		if _, ok := credentials[reserved]; ok {
			return reserved, true
		}
	}
	return "", false
}
