// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"slices"
	"time"
)

// Reason identifies why an artifact was revoked.
type Reason string

// Revocation reasons. A reason is either terminal, revoking the criterion's artifacts outright, or
// bounded, revoking only those established at or before the recorded action time. See boundaryReasons
// for the classification, which the write path, the deny-list query, and the Resource Server cache
// all share.
const (
	ReasonExplicit                     Reason = "explicit"
	ReasonRefreshRotation              Reason = "refresh_rotation"
	ReasonRefreshReplay                Reason = "refresh_replay"
	ReasonSessionLogout                Reason = "session_logout"
	ReasonCodeReplay                   Reason = "code_replay"
	ReasonExplicitTokenFamily          Reason = "explicit_token_family"
	ReasonApplicationDeleted           Reason = "application_deleted"
	ReasonApplicationSecretRegenerated Reason = "application_secret_regenerated"
	ReasonRoleAssignmentRemoved        Reason = "role_assignment_removed"
	ReasonRoleDeleted                  Reason = "role_deleted"
	ReasonGroupMembershipRemoved       Reason = "group_membership_removed"
	ReasonOrganizationUnitChanged      Reason = "organization_unit_changed"
	ReasonConsentRevoked               Reason = "consent_revoked"
	ReasonUserDeleted                  Reason = "user_deleted"
)

// CriterionType identifies an artifact attribute used for revocation.
type CriterionType string

// Criterion dimensions. Each names an artifact attribute that a revocation can be recorded against;
// the value stored alongside it is the revoked value for that dimension. Dimensions are kept
// separate at every enforcement point so a value of one type can never match another.
const (
	CriterionTypeTokenFamily      CriterionType = "token_family"
	CriterionTypeSubject          CriterionType = "subject"
	CriterionTypeApplicationID    CriterionType = "app.id"
	CriterionTypeApplicationKey   CriterionType = "app.key"
	CriterionTypeOrganizationUnit CriterionType = "ou.id"
	CriterionTypeRole             CriterionType = "role.id"
	CriterionTypeGroup            CriterionType = "group.id"
	CriterionTypeConsent          CriterionType = "consent.id"
	// CriterionTypeCredentialVersion names the credential-version dimension. The value is a version
	// marker, not a credential.
	CriterionTypeCredentialVersion CriterionType = "credential.version" // #nosec G101 -- dimension name, not a secret
)

// Mode controls whether revocation is permanent or bounded by an action time.
type Mode string

const (
	// ModeAll revokes every artifact matching the criterion, regardless of when it was established.
	ModeAll Mode = "revoke_all"
	// ModeBeforeAction revokes only the artifacts established at or before the revocation's cutoff.
	ModeBeforeAction Mode = "revoke_before_action"
)

// Criterion identifies one value in a revocation dimension.
type Criterion struct {
	Type  CriterionType
	Value string
}

// Identity contains trusted attributes evaluated against revocation criteria.
type Identity struct {
	JTI           string
	EstablishedAt time.Time
	Criteria      []Criterion
}

// CriteriaRevocation describes a criteria-based revocation write.
type CriteriaRevocation struct {
	Criterion Criterion
	Mode      Mode
	Cutoff    time.Time
	Reason    Reason
	// TTL is how long the deny-list row must survive, for a producer that knows the lifetime of the
	// artifacts its criterion matches. A criterion scoped to one application is the case that needs it:
	// token validity is configurable per application and uncapped, so the deployment-wide default the
	// revocation service falls back to can expire the row while matching artifacts are still valid.
	//
	// It raises the row's lifetime and never lowers it: the service writes the longer of this and its own
	// default, so a zero value keeps the default and no producer can shorten a row by supplying a small
	// value.
	TTL time.Duration
}

// boundaryReasons is the single source of truth for reasons that revoke only the artifacts
// established at or before the recorded action time. Every other reason is terminal: it revokes the
// criterion's artifacts outright. The write path, the deny-list SQL, and the Resource Server cache
// all derive from this slice so they cannot disagree about whether a revocation is terminal.
var boundaryReasons = []Reason{
	ReasonApplicationSecretRegenerated,
	ReasonRoleAssignmentRemoved,
	ReasonGroupMembershipRemoved,
	ReasonOrganizationUnitChanged,
	ReasonConsentRevoked,
}

// BoundaryReasons returns a copy of the boundary reasons, for callers that need the full set rather
// than a single membership test.
func BoundaryReasons() []Reason {
	return slices.Clone(boundaryReasons)
}

// IsBoundaryReason reports whether the reason affects only artifacts established before the action.
func IsBoundaryReason(reason Reason) bool {
	return slices.Contains(boundaryReasons, reason)
}
