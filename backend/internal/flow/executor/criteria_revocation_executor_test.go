// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/revocationmock"
)

type CriteriaRevocationExecutorTestSuite struct {
	suite.Suite
	factory core.FlowFactoryInterface
}

func TestCriteriaRevocationExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(CriteriaRevocationExecutorTestSuite))
}

func (s *CriteriaRevocationExecutorTestSuite) SetupTest() {
	s.factory = newAdministrationTestFactory(s.T())
}

func (s *CriteriaRevocationExecutorTestSuite) TestPostRevocationIsRegisteredUnderItsOwnName() {
	s.Equal(ExecutorNamePostRevocation,
		newPostRevocationExecutor(s.factory, revocationmock.NewCriteriaRevokerInterfaceMock(s.T())).GetName())
}

// The post-revocation node closes the window between the revocation and the action it precedes.
func (s *CriteriaRevocationExecutorTestSuite) TestPostRevocation_AdvancesTheCutoff() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	planCutoff := time.Now().UTC().Add(-time.Hour)
	var recorded time.Time
	revoker.On("RevokeCriteriaBatch", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			recorded = args.Get(1).([]revocation.CriteriaRevocation)[0].Cutoff
		}).Return(nil)

	resp, err := newPostRevocationExecutor(s.factory, revoker).Execute(withRevocationPlan(s.T(), "",
		revocationPlan{
			Mode:     revocation.ModeBeforeAction,
			Reason:   revocation.ReasonRoleAssignmentRemoved,
			TargetID: testRoleID,
			Cutoff:   planCutoff,
			Criteria: []revocation.Criterion{{
				Type:  revocation.CriterionTypeEntityScope,
				Value: revocation.EntityScopeCriterionValue(testAssigneeID, testCaliforniaAud, testLicenceScope),
			}},
		}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.True(recorded.After(planCutoff), "the re-stamp must advance the cutoff, not reuse the plan's")
}

// A terminal plan has no cutoff to advance, and rewriting one would be refused as a mode mismatch.
func (s *CriteriaRevocationExecutorTestSuite) TestPostRevocation_SkipsTerminalPlans() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())

	resp, err := newPostRevocationExecutor(s.factory, revoker).Execute(withRevocationPlan(s.T(), "",
		revocationPlan{
			Mode:     revocation.ModeAll,
			Reason:   revocation.ReasonApplicationDeleted,
			TargetID: "app-1",
			Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeApplicationKey, Value: "client-1"}},
		}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	revoker.AssertNotCalled(s.T(), "RevokeCriteriaBatch", mock.Anything, mock.Anything)
}

// An over-cap refusal from the revoker reaches the admin as FET-1106, and the action never runs.
func (s *CriteriaRevocationExecutorTestSuite) TestRefusesWhenTheRevokerRejectsTheFanOut() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	revoker.On("RevokeCriteriaBatch", mock.Anything, mock.Anything).
		Return(&revocation.CriteriaLimitExceededError{Criteria: 12, Max: 10})

	resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(withRevocationPlan(s.T(), "",
		revocationPlan{
			Mode:     revocation.ModeBeforeAction,
			Reason:   revocation.ReasonRoleDeleted,
			TargetID: testRoleID,
			Cutoff:   time.Now().UTC(),
			Criteria: []revocation.Criterion{{
				Type:  revocation.CriterionTypeEntityScope,
				Value: revocation.EntityScopeCriterionValue(testAssigneeID, testCaliforniaAud, testLicenceScope),
			}},
		}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Require().NotNil(resp.Error)
	s.Equal(ErrRevocationFanOutTooLarge.Code, resp.Error.Code)
	s.Equal("12", resp.Error.ErrorDescription.Params["criteria"])
	s.Equal("10", resp.Error.ErrorDescription.Params["max"])
	s.Contains(resp.Error.ErrorDescription.DefaultValue, "{{criteria}}")
	s.Contains(resp.Error.ErrorDescription.DefaultValue, "{{max}}")
}
