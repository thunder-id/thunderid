// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type ActionDeletionExecutorTestSuite struct {
	suite.Suite
	factory   core.FlowFactoryInterface
	resources *resourceAdminProviderMock
}

func TestActionDeletionExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(ActionDeletionExecutorTestSuite))
}

func (s *ActionDeletionExecutorTestSuite) SetupTest() {
	s.factory = newAdministrationTestFactory(s.T())
	s.resources = newResourceAdminProviderMock(s.T())
}

func (s *ActionDeletionExecutorTestSuite) TestDeletesTheActionTheValidatorPlanned() {
	s.resources.On("ValidateDeleteAction", mock.Anything, testRSID, optionalID(testResourceID), testActionID).
		Return(&revocation.AccessRevocationTarget{
			Scopes: []revocation.AudienceScope{{Audience: testCaliforniaAud, Scope: testLicenceScope}},
		}, nil)
	s.resources.On("DeleteAction", mock.Anything, testRSID, optionalID(testResourceID), testActionID).Return(nil)

	pre, err := newAccessChangeValidator(s.factory, nil, nil, s.resources).Execute(
		administrationNodeContext(ExecutorModeActionDeletion, map[string]string{
			revocationInputResourceServer: testRSID,
			revocationInputResource:       testResourceID,
			revocationInputAction:         testActionID,
		}))
	s.Require().NoError(err)
	s.Require().Equal(providers.ExecComplete, pre.Status)

	ctx := administrationNodeContext("", nil)
	ctx.SharedRuntimeData = pre.SharedRuntimeData
	resp, err := newActionDeletionExecutor(s.factory, s.resources).Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *ActionDeletionExecutorTestSuite) TestRefusesAPlanWithNoAction() {
	_, err := newActionDeletionExecutor(s.factory, s.resources).Execute(withRevocationPlan(s.T(), "",
		revocationPlan{
			Mode:     revocation.ModeBeforeAction,
			Reason:   revocation.ReasonActionDeleted,
			TargetID: testRSID,
			Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeScope, Value: "digest"}},
		}))

	s.Require().Error(err)
	s.Contains(err.Error(), "no target action")
}

func (s *ActionDeletionExecutorTestSuite) TestRejectsAForeignPlan() {
	_, err := newActionDeletionExecutor(s.factory, s.resources).Execute(withRevocationPlan(s.T(), "",
		revocationPlan{
			Mode:     revocation.ModeBeforeAction,
			Reason:   revocation.ReasonRoleAssignmentRemoved,
			TargetID: testRoleID,
			Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeEntityScope, Value: "digest"}},
		}))

	s.Require().Error(err)
	s.Contains(err.Error(), "was produced for")
}

func actionDeletionPlan() revocationPlan {
	return revocationPlan{
		Mode:       revocation.ModeBeforeAction,
		Reason:     revocation.ReasonActionDeleted,
		TargetID:   testRSID,
		ActionArgs: map[string]string{revocationInputAction: testActionID},
		Criteria:   []revocation.Criterion{{Type: revocation.CriterionTypeScope, Value: "digest"}},
	}
}

func (s *ActionDeletionExecutorTestSuite) TestDeletesAnActionWithNoResource() {
	s.resources.On("DeleteAction", mock.Anything, testRSID, (*string)(nil), testActionID).Return(nil)

	resp, err := newActionDeletionExecutor(s.factory, s.resources).Execute(
		withRevocationPlan(s.T(), "", actionDeletionPlan()))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *ActionDeletionExecutorTestSuite) TestCarriesTheServiceRefusal() {
	refusal := tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "RES-1030"}
	s.resources.On("DeleteAction", mock.Anything, testRSID, (*string)(nil), testActionID).Return(&refusal)

	resp, err := newActionDeletionExecutor(s.factory, s.resources).Execute(
		withRevocationPlan(s.T(), "", actionDeletionPlan()))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Require().NotNil(resp.Error)
	s.Equal("RES-1030", resp.Error.Code)
}

func (s *ActionDeletionExecutorTestSuite) TestUncodedRefusalFallsBackToTheNodeError() {
	s.resources.On("DeleteAction", mock.Anything, testRSID, (*string)(nil), testActionID).
		Return(&tidcommon.ServiceError{Type: tidcommon.ClientErrorType})

	resp, err := newActionDeletionExecutor(s.factory, s.resources).Execute(
		withRevocationPlan(s.T(), "", actionDeletionPlan()))

	s.Require().NoError(err)
	s.Require().NotNil(resp.Error)
	s.Equal(ErrActionDeletionFailed.Code, resp.Error.Code)
}

func (s *ActionDeletionExecutorTestSuite) TestServerErrorFailsTheFlow() {
	s.resources.On("DeleteAction", mock.Anything, testRSID, (*string)(nil), testActionID).
		Return(&tidcommon.InternalServerError)

	_, err := newActionDeletionExecutor(s.factory, s.resources).Execute(
		withRevocationPlan(s.T(), "", actionDeletionPlan()))

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to delete action")
}

func (s *ActionDeletionExecutorTestSuite) TestRefusesWithoutItsSeam() {
	_, err := newActionDeletionExecutor(s.factory, nil).Execute(
		withRevocationPlan(s.T(), "", actionDeletionPlan()))

	s.Require().Error(err)
	s.EqualError(err, "resource service is not configured")
}
