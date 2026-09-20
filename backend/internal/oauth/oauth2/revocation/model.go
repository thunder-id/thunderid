// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"time"

	sharedrevocation "github.com/thunder-id/thunderid/internal/revocation"
)

// RevokeOutcome is the protocol-level result of a revocation request, mapped to HTTP by the handler.
type RevokeOutcome int

const (
	// RevokeOutcomeRevoked indicates success — the token was revoked, or it was invalid/expired/unknown
	// and treated as a no-op success per RFC 7009 §2.2. Maps to HTTP 200.
	RevokeOutcomeRevoked RevokeOutcome = iota
	// RevokeOutcomeNotOwned indicates the token was issued to a different client. Maps to 400 invalid_grant.
	RevokeOutcomeNotOwned
)

// RevocationReason enumerates why a single token was revoked.
type RevocationReason = sharedrevocation.Reason

const (
	// RevocationReasonExplicit denotes a client/admin-initiated revocation (RFC 7009).
	RevocationReasonExplicit = sharedrevocation.ReasonExplicit
	// RevocationReasonRefreshRotation denotes revocation of a consumed refresh token on rotation.
	RevocationReasonRefreshRotation = sharedrevocation.ReasonRefreshRotation
	// RevocationReasonRefreshReplay denotes family revocation triggered by refresh-token replay
	// (a rotated, already-revoked refresh token presented again).
	RevocationReasonRefreshReplay = sharedrevocation.ReasonRefreshReplay
	// RevocationReasonSessionLogout denotes family revocation triggered by an SSO session sign-out.
	RevocationReasonSessionLogout = sharedrevocation.ReasonSessionLogout
	// RevocationReasonCodeReplay denotes family revocation triggered by authorization-code replay.
	RevocationReasonCodeReplay = sharedrevocation.ReasonCodeReplay
	// RevocationReasonExplicitTokenFamily denotes family revocation triggered by an explicit (RFC 7009)
	// revocation of a token that carries a token family id.
	RevocationReasonExplicitTokenFamily = sharedrevocation.ReasonExplicitTokenFamily
	// RevocationReasonApplicationDeleted permanently revokes artifacts issued to a deleted application.
	RevocationReasonApplicationDeleted = sharedrevocation.ReasonApplicationDeleted
	// RevocationReasonApplicationSecretRegenerated revokes artifacts established before secret regeneration.
	RevocationReasonApplicationSecretRegenerated = sharedrevocation.ReasonApplicationSecretRegenerated
	// RevocationReasonRoleAssignmentRemoved revokes artifacts established before a role assignment was removed.
	RevocationReasonRoleAssignmentRemoved = sharedrevocation.ReasonRoleAssignmentRemoved
	// RevocationReasonRolePermissionRemoved revokes artifacts established before a role's scopes shrank.
	RevocationReasonRolePermissionRemoved = sharedrevocation.ReasonRolePermissionRemoved
	// RevocationReasonRoleDeleted revokes artifacts established before a role was deleted.
	RevocationReasonRoleDeleted = sharedrevocation.ReasonRoleDeleted
	// RevocationReasonScopeDeleted revokes artifacts established before a scope was deleted.
	RevocationReasonScopeDeleted = sharedrevocation.ReasonScopeDeleted
	// RevocationReasonGroupMembershipRemoved revokes artifacts established before group membership was removed.
	RevocationReasonGroupMembershipRemoved = sharedrevocation.ReasonGroupMembershipRemoved
	// RevocationReasonOrganizationUnitChanged revokes artifacts established before an organization-unit change.
	RevocationReasonOrganizationUnitChanged = sharedrevocation.ReasonOrganizationUnitChanged
	// RevocationReasonConsentRevoked revokes artifacts established before consent was revoked.
	RevocationReasonConsentRevoked = sharedrevocation.ReasonConsentRevoked
	// RevocationReasonUserDeleted permanently revokes artifacts belonging to a deleted user.
	RevocationReasonUserDeleted = sharedrevocation.ReasonUserDeleted
)

// CriterionType identifies an artifact attribute used for criteria-based revocation.
type CriterionType = sharedrevocation.CriterionType

// Criterion dimensions, re-exported from the revocation package so OAuth callers need not import it
// directly. See that package for what each dimension matches.
const (
	CriterionTypeTokenFamily       = sharedrevocation.CriterionTypeTokenFamily
	CriterionTypeSubject           = sharedrevocation.CriterionTypeSubject
	CriterionTypeApplicationID     = sharedrevocation.CriterionTypeApplicationID
	CriterionTypeApplicationKey    = sharedrevocation.CriterionTypeApplicationKey
	CriterionTypeOrganizationUnit  = sharedrevocation.CriterionTypeOrganizationUnit
	CriterionTypeRole              = sharedrevocation.CriterionTypeRole
	CriterionTypeGroup             = sharedrevocation.CriterionTypeGroup
	CriterionTypeConsent           = sharedrevocation.CriterionTypeConsent
	CriterionTypeCredentialVersion = sharedrevocation.CriterionTypeCredentialVersion
	CriterionTypeEntityScope       = sharedrevocation.CriterionTypeEntityScope
	CriterionTypeScope             = sharedrevocation.CriterionTypeScope
)

// RevocationMode controls whether a criterion applies permanently or only to artifacts established
// before a trusted action boundary.
type RevocationMode = sharedrevocation.Mode

// Revocation modes, re-exported from the revocation package.
const (
	RevocationModeAll          = sharedrevocation.ModeAll
	RevocationModeBeforeAction = sharedrevocation.ModeBeforeAction
)

// Criterion identifies one value in a criteria-based revocation dimension.
type Criterion = sharedrevocation.Criterion

// RevocationIdentity contains the trusted attributes used to evaluate an artifact against the deny lists.
type RevocationIdentity = sharedrevocation.Identity

// CriteriaRevocation describes a criteria-based revocation write.
type CriteriaRevocation = sharedrevocation.CriteriaRevocation

// EntityScopeCriterionValue derives the CriterionTypeEntityScope value for one principal, audience
// and scope. Re-exported so OAuth callers derive it through the same implementation the writers use.
func EntityScopeCriterionValue(entityID, audience, scope string) string {
	return sharedrevocation.EntityScopeCriterionValue(entityID, audience, scope)
}

// ScopeCriterionValue derives the CriterionTypeScope value for one audience and scope.
func ScopeCriterionValue(audience, scope string) string {
	return sharedrevocation.ScopeCriterionValue(audience, scope)
}

// RevokedToken represents a single revoked token entry in the deny list.
type RevokedToken struct {
	// ID is the surrogate UUID v7 primary key. Generated by the store when empty.
	ID string
	// JTI is the jti claim of the revoked token and the deny-list lookup key.
	JTI string
	// RevocationReason records why the token was revoked.
	RevocationReason RevocationReason
	// RevokedAt is the time the revocation was recorded.
	RevokedAt time.Time
	// ExpiryTime is the revoked token's original expiry; the row is removable once this passes.
	ExpiryTime time.Time
}
