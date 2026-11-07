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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
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

func (s *CriteriaRevocationExecutorTestSuite) TestDeclaresRevokeAsTheDefaultMode() {
	meta := newCriteriaRevocationExecutor(s.factory, revocationmock.NewCriteriaRevokerInterfaceMock(s.T())).GetMeta()

	s.Require().NotNil(meta)
	s.Equal(ExecutorModeRevoke, meta.DefaultMode)
	s.Equal([]string{ExecutorModeAdvanceCutoff, ExecutorModeRevoke}, meta.SupportedModes)
}

func (s *CriteriaRevocationExecutorTestSuite) TestRevokeModesWriteThePlanCutoff() {
	for _, mode := range []string{"", ExecutorModeRevoke} {
		s.Run("mode="+mode, func() {
			revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
			plan := roleDeletionPlan()
			plan.Cutoff = time.Now().UTC().Add(-time.Hour)
			revoker.On("RevokeCriteriaBatch", mock.Anything,
				mock.MatchedBy(func(batch []revocation.CriteriaRevocation) bool {
					return len(batch) == 1 && batch[0].Cutoff.Equal(plan.Cutoff) &&
						batch[0].Mode == revocation.ModeBeforeAction && batch[0].Criterion == plan.Criteria[0]
				})).Return(nil).Once()

			resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(
				withRevocationPlan(s.T(), mode, plan))

			s.Require().NoError(err)
			s.Equal(providers.ExecComplete, resp.Status)
		})
	}
}

func (s *CriteriaRevocationExecutorTestSuite) TestAdvanceCutoff_AdvancesTheCutoff() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	planCutoff := time.Now().UTC().Add(-time.Hour)
	var recorded time.Time
	revoker.On("RevokeCriteriaBatch", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			recorded = args.Get(1).([]revocation.CriteriaRevocation)[0].Cutoff
		}).Return(nil)

	resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(
		withRevocationPlan(s.T(), ExecutorModeAdvanceCutoff, revocationPlan{
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
	s.True(recorded.After(planCutoff), "advancing the cutoff must not reuse the plan's")
}

// roleDeletionPostContext is the post flow of a role deletion, handed the holders the delete removed.
func roleDeletionPostContext(t *testing.T, plan revocationPlan, removedHolders string) *providers.NodeContext {
	ctx := withRevocationPlan(t, ExecutorModeAdvanceCutoff, plan)
	ctx.FlowType = providers.FlowTypeSystem
	ctx.UserInputs = map[string]string{revocationInputRole: testRoleID}
	if removedHolders != "" {
		ctx.UserInputs[revocationInputRemovedHolders] = removedHolders
	}
	return ctx
}

// recordBatch makes the revoker record the criteria of the batch it is handed.
func recordBatch(revoker *revocationmock.CriteriaRevokerInterfaceMock) *[]revocation.Criterion {
	recorded := &[]revocation.Criterion{}
	revoker.On("RevokeCriteriaBatch", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			for _, row := range args.Get(1).([]revocation.CriteriaRevocation) {
				*recorded = append(*recorded, row.Criterion)
			}
		}).Return(nil).Once()
	return recorded
}

func holderCriterion(entityID string) revocation.Criterion {
	return revocation.Criterion{
		Type:  revocation.CriterionTypeEntityScope,
		Value: revocation.EntityScopeCriterionValue(entityID, testCaliforniaAud, testLicenceScope),
	}
}

func roleDeletionPlanWithHolderScopes() revocationPlan {
	plan := roleDeletionPlan()
	plan.HolderScopes = []revocation.AudienceScope{{Audience: testCaliforniaAud, Scope: testLicenceScope}}
	return plan
}

// A holder assigned after the pre flow planned the revocation is revoked once the delete removes it.
func (s *CriteriaRevocationExecutorTestSuite) TestAdvanceCutoff_RevokesAHolderAddedAfterThePlan() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	recorded := recordBatch(revoker)

	resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(roleDeletionPostContext(s.T(),
		roleDeletionPlanWithHolderScopes(), `["`+testAssigneeID+`","`+testMemberID+`"]`))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Equal([]revocation.Criterion{holderCriterion(testAssigneeID), holderCriterion(testMemberID)}, *recorded)
}

// A role that had no holder when the plan was made still revokes the one assigned before the delete.
func (s *CriteriaRevocationExecutorTestSuite) TestAdvanceCutoff_RevokesAHolderOfARoleThatHadNone() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	recorded := recordBatch(revoker)
	plan := roleDeletionPlanWithHolderScopes()
	plan.Criteria = nil
	plan.NothingToRevoke = true

	resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(roleDeletionPostContext(s.T(),
		plan, `["`+testMemberID+`"]`))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Equal([]revocation.Criterion{holderCriterion(testMemberID)}, *recorded)
}

// When nobody was assigned after the plan, the post flow writes the plan's criteria and nothing more.
func (s *CriteriaRevocationExecutorTestSuite) TestAdvanceCutoff_AddsNothingWhenNoHolderWasAdded() {
	for name, removed := range map[string]string{
		"every removed holder planned": `["` + testAssigneeID + `"]`,
		"no removed holder":            "",
	} {
		s.Run(name, func() {
			revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
			recorded := recordBatch(revoker)

			resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(roleDeletionPostContext(s.T(),
				roleDeletionPlanWithHolderScopes(), removed))

			s.Require().NoError(err)
			s.Equal(providers.ExecComplete, resp.Status)
			s.Equal([]revocation.Criterion{holderCriterion(testAssigneeID)}, *recorded)
		})
	}
}

// A role deletion plan with no scopes and no holder added writes nothing.
func (s *CriteriaRevocationExecutorTestSuite) TestAdvanceCutoff_NothingToRevokeStaysEmpty() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	plan := roleDeletionPlan()
	plan.Criteria = nil
	plan.NothingToRevoke = true

	resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(roleDeletionPostContext(s.T(),
		plan, `["`+testMemberID+`"]`))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	revoker.AssertNotCalled(s.T(), "RevokeCriteriaBatch", mock.Anything, mock.Anything)
}

// Only a system flow, whose inputs come from the service, may name removed holders.
func (s *CriteriaRevocationExecutorTestSuite) TestAdvanceCutoff_AdministrationFlowCannotNameHolders() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	recorded := recordBatch(revoker)
	ctx := roleDeletionPostContext(s.T(), roleDeletionPlanWithHolderScopes(), `["`+testMemberID+`"]`)
	ctx.FlowType = providers.FlowTypeAdministration

	resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Equal([]revocation.Criterion{holderCriterion(testAssigneeID)}, *recorded)
}

func (s *CriteriaRevocationExecutorTestSuite) TestAdvanceCutoff_MalformedRemovedHoldersFailTheNode() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())

	resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(roleDeletionPostContext(s.T(),
		roleDeletionPlanWithHolderScopes(), `not-json`))

	s.Nil(resp)
	s.ErrorContains(err, "failed to decode the removed holders")
	revoker.AssertNotCalled(s.T(), "RevokeCriteriaBatch", mock.Anything, mock.Anything)
}

func (s *CriteriaRevocationExecutorTestSuite) TestAdvanceCutoff_SkipsTerminalPlans() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())

	resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(
		withRevocationPlan(s.T(), ExecutorModeAdvanceCutoff, revocationPlan{
			Mode:     revocation.ModeAll,
			Reason:   revocation.ReasonUserDeleted,
			TargetID: "app-1",
			Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeApplicationKey, Value: "client-1"}},
		}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	revoker.AssertNotCalled(s.T(), "RevokeCriteriaBatch", mock.Anything, mock.Anything)
}

func (s *CriteriaRevocationExecutorTestSuite) TestRefusesAnUnsupportedMode() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())

	resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(
		withRevocationPlan(s.T(), ExecutorModeRoleDeletion, roleDeletionPlan()))

	s.Nil(resp)
	s.ErrorContains(err, "unsupported mode")
	revoker.AssertNotCalled(s.T(), "RevokeCriteriaBatch", mock.Anything, mock.Anything)
}

func (s *CriteriaRevocationExecutorTestSuite) TestRefusesWhenTheRevokerRejectsTheFanOut() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	revoker.On("RevokeCriteriaBatch", mock.Anything, mock.Anything).
		Return(revocation.ErrorEntryLimitExceeded.WithParams(map[string]string{"entries": "12", "max": "10"}))

	resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(withRevocationPlan(s.T(), "",
		roleDeletionPlan()))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Require().NotNil(resp.Error)
	s.Equal(revocation.ErrorEntryLimitExceeded.Code, resp.Error.Code)
	s.Equal("The change would record 12 revocation entries, more than the 10 allowed for one change",
		resp.Error.ErrorDescription.String())
}

func (s *CriteriaRevocationExecutorTestSuite) TestServerErrorFromTheRevokerFailsTheNode() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	revoker.On("RevokeCriteriaBatch", mock.Anything, mock.Anything).Return(&tidcommon.InternalServerError)

	resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(withRevocationPlan(s.T(), "",
		roleDeletionPlan()))

	s.Nil(resp)
	s.ErrorContains(err, "failed to revoke tokens by criteria")
}

func roleDeletionPlan() revocationPlan {
	return revocationPlan{
		Mode:     revocation.ModeBeforeAction,
		Reason:   revocation.ReasonRoleDeleted,
		TargetID: testRoleID,
		Cutoff:   time.Now().UTC(),
		Criteria: []revocation.Criterion{{
			Type:  revocation.CriterionTypeEntityScope,
			Value: revocation.EntityScopeCriterionValue(testAssigneeID, testCaliforniaAud, testLicenceScope),
		}},
	}
}
