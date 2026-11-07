// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"errors"

	"github.com/stretchr/testify/mock"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// recordSystemFlows gives the service a provider that expects exactly times flow runs and records each in
// order, failing the run of failing with err and sharing the handle of each run that succeeds.
func (suite *RoleServiceTestSuite) recordSystemFlows(failing string, err *tidcommon.ServiceError,
	order *[]string, times int) *[]*flowcommon.SystemFlowContext {
	runs := &[]*flowcommon.SystemFlowContext{}
	flows := newSystemFlowProviderMock(suite.T())
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
			*tidcommon.ServiceError) {
			*runs = append(*runs, sysCtx)
			*order = append(*order, sysCtx.Handle)
			if sysCtx.Handle == failing {
				return nil, err
			}
			return &flowcommon.SystemFlowResult{RuntimeData: map[string]string{"plan": sysCtx.Handle}}, nil
		}).Times(times)
	suite.Require().NoError(SetSystemFlowProvider(suite.service, flows))
	return runs
}

// expectRoleRemoval sets up a role that exists and may be deleted, recording its removal in order.
func (suite *RoleServiceTestSuite) expectRoleRemoval(order *[]string, deleteErr error) {
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil).Once()
	suite.unmandated()
	suite.mockSharingService.EXPECT().DeletePoliciesForResource(mock.Anything, RoleSharingType, "role1").
		Return(nil).Once()
	suite.mockStore.On("DeleteAssignmentsByRoleID", mock.Anything, "role1").
		Run(func(mock.Arguments) { *order = append(*order, "assignments") }).Return(nil).Once()
	suite.mockStore.On("DeleteRole", mock.Anything, "role1").
		Run(func(mock.Arguments) { *order = append(*order, "role") }).Return(deleteErr).Once()
}

// A role deletion runs the pre flow with the role as input before anything is removed, and the post
// flow, with what the pre flow shared, once the role is gone.
func (suite *RoleServiceTestSuite) TestDeleteRole_RunsThePreAndPostFlowsAroundTheWrite() {
	order := []string{}
	runs := suite.recordSystemFlows("", nil, &order, 2)
	suite.expectRoleRemoval(&order, nil)

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.Nil(err)
	suite.Equal([]string{systemFlowRoleDeletePre, "assignments", "role", systemFlowRevocationPost}, order)
	inputs := map[string]string{systemFlowInputRole: "role1"}
	suite.Equal([]*flowcommon.SystemFlowContext{
		{Handle: systemFlowRoleDeletePre, Inputs: inputs},
		{Handle: systemFlowRevocationPost, Inputs: inputs,
			RuntimeData: map[string]string{"plan": systemFlowRoleDeletePre}},
	}, *runs)
}

// A refused pre flow leaves the role and its assignments as they were, and runs no post flow.
func (suite *RoleServiceTestSuite) TestDeleteRole_RefusedPreFlowRemovesNothing() {
	order := []string{}
	suite.recordSystemFlows(systemFlowRoleDeletePre, &ErrorMandatedRoleCannotBeDeleted, &order, 1)
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil).Once()

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.Equal(&ErrorMandatedRoleCannotBeDeleted, err)
	suite.Equal([]string{systemFlowRoleDeletePre}, order)
	suite.mockSharingService.AssertNotCalled(suite.T(), "DeletePoliciesForResource",
		mock.Anything, mock.Anything, mock.Anything)
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteAssignmentsByRoleID", mock.Anything, mock.Anything)
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteRole", mock.Anything, mock.Anything)
}

// A delete that fails runs no post flow, so the cutoff stays where the pre flow left it.
func (suite *RoleServiceTestSuite) TestDeleteRole_FailedWriteRunsNoPostFlow() {
	order := []string{}
	suite.recordSystemFlows("", nil, &order, 1)
	suite.expectRoleRemoval(&order, errors.New("db down"))

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.Equal(&tidcommon.InternalServerError, err)
	suite.Equal([]string{systemFlowRoleDeletePre, "assignments", "role"}, order)
}

// A post flow that fails after the role is deleted is reported, not passed over.
func (suite *RoleServiceTestSuite) TestDeleteRole_FailedPostFlowIsReported() {
	order := []string{}
	suite.recordSystemFlows(systemFlowRevocationPost, &tidcommon.InternalServerError, &order, 2)
	suite.expectRoleRemoval(&order, nil)

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.Equal(&tidcommon.InternalServerError, err)
	suite.Equal([]string{systemFlowRoleDeletePre, "assignments", "role", systemFlowRevocationPost}, order)
}

// A role that is already gone runs no flow, so a retry after a partial delete revokes nothing again.
func (suite *RoleServiceTestSuite) TestDeleteRole_MissingRoleRunsNoFlow() {
	suite.Require().NoError(SetSystemFlowProvider(suite.service, newSystemFlowProviderMock(suite.T())))
	suite.mockStore.On("IsRoleExist", mock.Anything, "nonexistent").Return(false, nil).Once()

	err := suite.service.DeleteRole(context.Background(), "nonexistent")

	suite.Nil(err)
}

// The server injects the flow engine once it exists; a service that cannot take it is reported.
func (suite *RoleServiceTestSuite) TestSetSystemFlowProvider() {
	provider := newSystemFlowProviderMock(suite.T())

	suite.NoError(SetSystemFlowProvider(suite.service, provider))
	suite.Same(provider, suite.service.(*roleService).systemFlows)
	suite.Error(SetSystemFlowProvider(NewRoleServiceInterfaceMock(suite.T()), provider))
}
