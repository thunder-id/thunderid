// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

func (s *ScopeRevocationExecutorsTestSuite) TestGroupMembershipRemoval_RemovesTheMember() {
	s.groups.On("RemoveGroupMember", mock.Anything, testGroupID, testMemberID).Return(nil)

	resp, err := newGroupMembershipRemovalExecutor(s.factory, s.groups).
		Execute(s.withPlan(revocationPlan{
			Mode:       revocation.ModeBeforeAction,
			Reason:     revocation.ReasonGroupMembershipRemoved,
			TargetID:   testGroupID,
			AssigneeID: testMemberID,
			Criteria: []revocation.Criterion{
				{Type: revocation.CriterionTypeEntityScope, Value: "digest"}},
		}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

// Both group actions record the same reason, so the reason check cannot separate them. A member removal
// handed a deletion's plan must refuse rather than fall back to deleting the group.
func (s *ScopeRevocationExecutorsTestSuite) TestGroupMembershipRemoval_RefusesADeletionPlan() {
	_, err := newGroupMembershipRemovalExecutor(s.factory, s.groups).
		Execute(s.withPlan(revocationPlan{
			Mode:     revocation.ModeBeforeAction,
			Reason:   revocation.ReasonGroupMembershipRemoved,
			TargetID: testGroupID,
			Criteria: []revocation.Criterion{
				{Type: revocation.CriterionTypeEntityScope, Value: "digest"}},
		}))

	s.Require().Error(err)
	s.Contains(err.Error(), "no departing member")
}

// Both group actions record the same reason and put the group id in the same field, so the reason
// check cannot separate them. A deletion node handed a member removal's plan must refuse: deleting the
// group would destroy every other member's membership too, which is far more than the graph asked for.
func (s *ScopeRevocationExecutorsTestSuite) TestGroupDeletion_RefusesAMembershipRemovalPlan() {
	_, err := newGroupDeletionExecutor(s.factory, s.groups).
		Execute(s.withPlan(revocationPlan{
			Mode:       revocation.ModeBeforeAction,
			Reason:     revocation.ReasonGroupMembershipRemoved,
			TargetID:   testGroupID,
			AssigneeID: testMemberID,
			Criteria: []revocation.Criterion{
				{Type: revocation.CriterionTypeEntityScope, Value: "digest"}},
		}))

	s.Require().Error(err)
	s.Contains(err.Error(), "membership removal")
}
