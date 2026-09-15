// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

func (s *ScopeRevocationExecutorsTestSuite) TestScopeDeletion_DeletesTheCarriedAction() {
	s.resources.On("DeleteAction", mock.Anything, testRSID, testResourceID, testActionID).Return(nil)

	resp, err := newScopeDeletionExecutor(s.factory, s.resources).
		Execute(s.withPlan(revocationPlan{
			Mode:     revocation.ModeBeforeAction,
			Reason:   revocation.ReasonScopeDeleted,
			TargetID: testRSID,
			ActionArgs: map[string]string{
				revocationInputAction:   testActionID,
				revocationInputResource: testResourceID,
			},
			Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeScope, Value: "digest"}},
		}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

// Without an action id the node has nothing to delete, and deleting whatever the resource server's
// first action happens to be would be far worse than refusing.
func (s *ScopeRevocationExecutorsTestSuite) TestScopeDeletion_RefusesAPlanWithNoAction() {
	_, err := newScopeDeletionExecutor(s.factory, s.resources).
		Execute(s.withPlan(revocationPlan{
			Mode:     revocation.ModeBeforeAction,
			Reason:   revocation.ReasonScopeDeleted,
			TargetID: testRSID,
			Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeScope, Value: "digest"}},
		}))

	s.Require().Error(err)
	s.Contains(err.Error(), "no target action")
}
