// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"

	"github.com/stretchr/testify/mock"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// permissionUpdate gives the service a system flow provider and readies role1, read once, to be updated
// to keep only the next permissions on rs1, while it currently holds the given ones, and returns the
// provider and the update.
func (suite *RoleServiceTestSuite) permissionUpdate(current, next []string) (*systemFlowProviderMock,
	RoleUpdateDetail) {
	flows := newSystemFlowProviderMock(suite.T())
	suite.Require().NoError(SetSystemFlowProvider(suite.service, flows))
	request := RoleUpdateDetail{Name: "Role", OUID: "ou1"}
	if len(next) > 0 {
		request.Permissions = []ResourcePermissions{{ResourceServerID: "rs1", Permissions: next}}
		suite.mockResourceService.On("ValidatePermissions", mock.Anything, "rs1", next).
			Return([]string{}, nil).Once()
	}
	suite.mockStore.On("GetRole", mock.Anything, "role1").Return(RoleWithPermissions{
		ID: "role1", OUID: "ou1", Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: current}},
	}, nil).Once()
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").
		Return(oupkg.OrganizationUnit{ID: "ou1"}, nil)
	suite.mockStore.On("CheckRoleNameExistsExcludingID", mock.Anything, "ou1", "Role", "role1").Return(false, nil)
	suite.mockSharingService.EXPECT().ListVisibleResourceIDs(mock.Anything, RoleSharingType, "ou1").
		Return(nil, nil).Once()
	return flows, request
}

func (suite *RoleServiceTestSuite) TestSetSystemFlowProvider() {
	provider := newSystemFlowProviderMock(suite.T())

	suite.NoError(SetSystemFlowProvider(suite.service, provider))
	suite.Same(provider, suite.service.(*roleService).systemFlows)
	suite.Error(SetSystemFlowProvider(NewRoleServiceInterfaceMock(suite.T()), provider))
}

// An update that drops a permission runs the pre flow with the role's new set before the write, and the
// revocation post flow, with what the pre flow shared, after.
func (suite *RoleServiceTestSuite) TestUpdateRole_DroppingAPermissionRunsThePreAndPostFlows() {
	flows, request := suite.permissionUpdate([]string{"perm1", "perm2"}, []string{"perm1"})
	var order []string
	inputs := map[string]string{
		systemFlowInputRole:        "role1",
		systemFlowInputPermissions: `[{"resourceServerId":"rs1","permissions":["perm1"]}]`,
	}
	shared := map[string]string{"revocationPlan": "plan"}
	flows.EXPECT().ExecuteSystemFlow(mock.Anything,
		&flowcommon.SystemFlowContext{Handle: systemFlowRolePermissionRemovalPre, Inputs: inputs}).
		Run(func(context.Context, *flowcommon.SystemFlowContext) { order = append(order, "pre") }).
		Return(&flowcommon.SystemFlowResult{RuntimeData: shared}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything,
		&flowcommon.SystemFlowContext{Handle: systemFlowRevocationPost, Inputs: inputs, RuntimeData: shared}).
		Run(func(context.Context, *flowcommon.SystemFlowContext) { order = append(order, "post") }).
		Return(&flowcommon.SystemFlowResult{}, nil).Once()
	suite.mockStore.On("UpdateRole", mock.Anything, "role1", request).
		Run(func(mock.Arguments) { order = append(order, "write") }).Return(nil)

	_, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", request)

	suite.Nil(err)
	suite.Equal([]string{"pre", "write", "post"}, order)
}

// Dropping every permission hands the pre flow an empty set rather than none.
func (suite *RoleServiceTestSuite) TestUpdateRole_DroppingEveryPermissionCarriesAnEmptySet() {
	flows, request := suite.permissionUpdate([]string{"perm1"}, nil)
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowRolePermissionRemovalPre && c.Inputs[systemFlowInputPermissions] == "[]"
	})).Return(&flowcommon.SystemFlowResult{}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowRevocationPost
	})).Return(&flowcommon.SystemFlowResult{}, nil).Once()
	suite.mockStore.On("UpdateRole", mock.Anything, "role1", request).Return(nil)

	_, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", request)

	suite.Nil(err)
}

// A post flow that fails after the update is written is reported, not passed over.
func (suite *RoleServiceTestSuite) TestUpdateRole_FailedPostFlowIsReported() {
	flows, request := suite.permissionUpdate([]string{"perm1", "perm2"}, []string{"perm1"})
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowRolePermissionRemovalPre
	})).Return(&flowcommon.SystemFlowResult{}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowRevocationPost
	})).Return(nil, &tidcommon.InternalServerError).Once()
	suite.mockStore.On("UpdateRole", mock.Anything, "role1", request).Return(nil)

	_, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", request)

	suite.Equal(&tidcommon.InternalServerError, err)
}

// An update that drops nothing, as one carrying the role's current permissions does, runs no flow.
func (suite *RoleServiceTestSuite) TestUpdateRole_DroppingNothingRunsNoFlow() {
	_, request := suite.permissionUpdate([]string{"perm1"}, []string{"perm1"})
	suite.mockStore.On("UpdateRole", mock.Anything, "role1", request).Return(nil)

	_, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", request)

	suite.Nil(err)
}

// A refused pre flow leaves the role as it was.
func (suite *RoleServiceTestSuite) TestUpdateRole_RefusedFlowWritesNothing() {
	flows, request := suite.permissionUpdate([]string{"perm1", "perm2"}, []string{"perm1"})
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).
		Return(nil, &tidcommon.InternalServerError).Once()

	_, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", request)

	suite.Equal(&tidcommon.InternalServerError, err)
	suite.mockStore.AssertNotCalled(suite.T(), "UpdateRole", mock.Anything, mock.Anything, mock.Anything)
}
