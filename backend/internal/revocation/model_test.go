// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type RevocationModelTestSuite struct {
	suite.Suite
}

func TestRevocationModelTestSuite(t *testing.T) {
	suite.Run(t, new(RevocationModelTestSuite))
}

// allReasons is every reason the package declares. Classification is a security decision — a
// terminal reason revokes an artifact outright while a boundary reason only revokes what predates
// the action — so each one is pinned explicitly rather than derived from the implementation.
var allReasons = map[Reason]bool{
	ReasonExplicit:                     false,
	ReasonRefreshRotation:              false,
	ReasonRefreshReplay:                false,
	ReasonSessionLogout:                false,
	ReasonCodeReplay:                   false,
	ReasonExplicitTokenFamily:          false,
	ReasonApplicationDeleted:           false,
	ReasonUserDeleted:                  false,
	ReasonApplicationSecretRegenerated: true,
	ReasonRoleAssignmentRemoved:        true,
	ReasonRolePermissionRemoved:        true,
	// A deleted role, and a deleted scope, revoke only what predates the action. The scopes they
	// carried can be regranted through another role, or the name reused, and a terminal row would
	// then deny tokens the principal is legitimately entitled to.
	ReasonRoleDeleted:             true,
	ReasonScopeDeleted:            true,
	ReasonGroupMembershipRemoved:  true,
	ReasonOrganizationUnitChanged: true,
	ReasonConsentRevoked:          true,
}

func (s *RevocationModelTestSuite) TestIsBoundaryReason() {
	for reason, wantBoundary := range allReasons {
		s.Run(string(reason), func() {
			s.Equal(wantBoundary, IsBoundaryReason(reason),
				"reason %q is classified differently than the deny-list write path expects", reason)
		})
	}
}

// An unrecognized reason must be treated as terminal. Falling back to "boundary" would let an
// artifact established after the revocation slip past the deny list.
func (s *RevocationModelTestSuite) TestIsBoundaryReasonUnknownIsTerminal() {
	s.False(IsBoundaryReason(Reason("")))
	s.False(IsBoundaryReason(Reason("not_a_reason")))
}

// BoundaryReasons and IsBoundaryReason must not disagree: the SQL deny-list predicate is built from
// the former while the Resource Server cache classifies entries with the latter.
func (s *RevocationModelTestSuite) TestBoundaryReasonsMatchesPredicate() {
	boundary := BoundaryReasons()
	s.Len(boundary, 8)

	for _, reason := range boundary {
		s.True(IsBoundaryReason(reason), "%q is listed as boundary but not classified as one", reason)
	}
	for reason, wantBoundary := range allReasons {
		s.Equal(wantBoundary, slicesContains(boundary, reason),
			"reason %q disagrees between the boundary list and its expected classification", reason)
	}
}

// The caller receives a copy, so a mutation cannot reach the package-level source of truth that the
// write path and the deny-list query both read.
func (s *RevocationModelTestSuite) TestBoundaryReasonsReturnsCopy() {
	first := BoundaryReasons()
	s.Require().NotEmpty(first)
	first[0] = Reason("tampered")

	s.NotContains(BoundaryReasons(), Reason("tampered"))
	s.False(IsBoundaryReason(Reason("tampered")))
}

func slicesContains(reasons []Reason, target Reason) bool {
	for _, reason := range reasons {
		if reason == target {
			return true
		}
	}
	return false
}
