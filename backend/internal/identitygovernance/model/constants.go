// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package model defines the governance vocabulary, the scope registry and the stored access state.
package model

// SystemAttrAccessState is the runtime-attribute key holding failure and lock state. Written only by
// the entity store, never from caller input.
const SystemAttrAccessState = "accessState"

// AccessScope names the authentication method a lock is scoped to. The set is closed because a scope
// is used as a path segment in lock-state writes.
type AccessScope string

// The registered access scopes.
const (
	// AccessScopeCredential covers password, PIN, secret answer and other values verified in one
	// credential check. Lockable.
	AccessScopeCredential AccessScope = "credential"

	// AccessScopeOTP is the one-time-password method. Lockable.
	AccessScopeOTP AccessScope = "otp"

	// AccessScopePasskey is the passkey method. Not lockable: a failure names no entity.
	AccessScopePasskey AccessScope = "passkey"

	// AccessScopeFederated is the external identity provider. Not lockable.
	AccessScopeFederated AccessScope = "federated"

	// AccessScopeMagicLink is the magic-link method. Not lockable.
	AccessScopeMagicLink AccessScope = "magiclink"

	// AccessScopeOpenID4VP is the verifiable-presentation method. Not lockable.
	AccessScopeOpenID4VP AccessScope = "openid4vp"

	// AccessScopeSystemCredential covers clientSecret and flowSecret. Neither counted nor locked.
	AccessScopeSystemCredential AccessScope = "system_credential" // #nosec G101 -- scope name, not a secret

	// AccessScopeEntity is the reserved key for entity-wide lock counting. It is never derived from
	// an authentication method.
	AccessScopeEntity AccessScope = "entity"
)

// AllAccessScopes lists every registered access scope.
var AllAccessScopes = []AccessScope{
	AccessScopeCredential,
	AccessScopeOTP,
	AccessScopePasskey,
	AccessScopeFederated,
	AccessScopeMagicLink,
	AccessScopeOpenID4VP,
	AccessScopeSystemCredential,
	AccessScopeEntity,
}

// accessScopes is AllAccessScopes as a set, for membership checks.
var accessScopes = func() map[AccessScope]struct{} {
	scopes := make(map[AccessScope]struct{}, len(AllAccessScopes))
	for _, scope := range AllAccessScopes {
		scopes[scope] = struct{}{}
	}
	return scopes
}()

// IsAccessScope reports whether scope is registered, including AccessScopeEntity.
func IsAccessScope(scope AccessScope) bool {
	_, ok := accessScopes[scope]
	return ok
}

// ScopeClass says whether an authentication method's failures are counted and locked. It is fixed
// per method, not configurable.
type ScopeClass string

const (
	// ClassNotLockable means failures are neither counted nor locked. The zero value, so an unknown
	// scope locks nothing.
	ClassNotLockable ScopeClass = ""

	// ClassLockable means failures are counted and may form a lock.
	ClassLockable ScopeClass = "lockable"
)

// Counted reports whether a failure at this class is recorded. Only lockable classes are.
func (c ScopeClass) Counted() bool {
	return c == ClassLockable
}

// scopeClasses maps each registered scope to its class, for every entity category.
var scopeClasses = map[AccessScope]ScopeClass{
	AccessScopeCredential: ClassLockable,
	AccessScopeOTP:        ClassLockable,

	// Not locked, so a stale client secret in a retry loop cannot take the client offline.
	AccessScopeSystemCredential: ClassNotLockable,

	// These name no entity on the failure path.
	AccessScopePasskey:   ClassNotLockable,
	AccessScopeMagicLink: ClassNotLockable,
	AccessScopeFederated: ClassNotLockable,
	AccessScopeOpenID4VP: ClassNotLockable,

	// Entity-wide scope; never derived.
	AccessScopeEntity: ClassLockable,
}

// ClassOf returns the class of a scope. An unregistered scope is not lockable.
func ClassOf(scope AccessScope) ScopeClass {
	class, ok := scopeClasses[scope]
	if !ok {
		return ClassNotLockable
	}
	return class
}
