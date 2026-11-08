// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"errors"
	"slices"

	"github.com/stretchr/testify/mock"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// removalOf readies the suite to remove testUserID1 from role1 in its owner's organization unit while
// the role holds the given assignments there, with the system flows mocked, and returns the request and
// the flow engine.
func (suite *RoleAssignmentServiceTestSuite) removalOf(held []RoleAssignment) (
	[]RoleAssignment, *systemFlowProviderMock) {
	flows := newSystemFlowProviderMock(suite.T())
	suite.Require().NoError(SetAssignmentSystemFlowProvider(suite.service, flows))
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
		Return([]providers.Entity{ownedUser()}, nil)
	suite.ownedRole("role1")
	suite.mockStore.On("ListMandatedAssignmentsForRole", mock.Anything, "role1", testOwnerOU).
		Return([]RoleAssignment{}, nil)
	suite.mockStore.On("GetDirectAssignments", mock.Anything, "role1", testOwnerOU, []string{testUserID1}).
		Return(held, nil).Once()
	return []RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}}, flows
}

// heldByOwner is testUserID1's assignment to role1 in its owner's organization unit, as stored.
func heldByOwner() []RoleAssignment {
	return []RoleAssignment{{ID: testUserID1, Type: assigneeTypeEntity, OUID: testOwnerOU}}
}

// Removing a held assignment runs the pre flow with the role, the assignee and the organization unit
// before the write, and the revocation post flow, with what the pre flow shared, after.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_RunsThePreAndPostFlowsAroundTheWrite() {
	stored := heldByOwner()
	request, flows := suite.removalOf(stored)
	var order []string
	inputs := map[string]string{
		systemFlowInputRole: "role1", systemFlowInputAssignee: testUserID1,
		systemFlowInputAssignmentOU: testOwnerOU,
	}
	shared := map[string]string{"revocationPlan": "plan"}
	flows.EXPECT().ExecuteSystemFlow(mock.Anything,
		&flowcommon.SystemFlowContext{Handle: systemFlowRoleAssignmentRemovalPre, Inputs: inputs}).
		Run(func(context.Context, *flowcommon.SystemFlowContext) { order = append(order, "pre") }).
		Return(&flowcommon.SystemFlowResult{RuntimeData: shared}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything,
		&flowcommon.SystemFlowContext{Handle: systemFlowRevocationPost, Inputs: inputs, RuntimeData: shared}).
		Run(func(context.Context, *flowcommon.SystemFlowContext) { order = append(order, "post") }).
		Return(&flowcommon.SystemFlowResult{}, nil).Once()
	suite.mockStore.On("RemoveAssignments", mock.Anything, "role1", stored).
		Run(func(mock.Arguments) { order = append(order, "write") }).Return(nil)

	err := suite.service.RemoveAssignments(context.Background(), "role1", "", request)

	suite.Nil(err)
	suite.Equal([]string{"pre", "write", "post"}, order)
	suite.mockStore.AssertNotCalled(suite.T(), "GetRoleAssignments", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

// A removal naming more assignments than a request may remove is refused before anything is read.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_TooManyAssignmentsIsRefused() {
	request := make([]RoleAssignment, maxRemovedAssignments+1)
	for i := range request {
		request[i] = RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser}
	}

	err := suite.service.RemoveAssignments(context.Background(), "role1", "", request)

	suite.Equal(&ErrorTooManyAssignments, err)
	suite.mockStore.AssertNotCalled(suite.T(), "GetRole", mock.Anything, mock.Anything)
}

// An assignee named twice is revoked once.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_AnAssigneeNamedTwiceRunsOneFlow() {
	stored := heldByOwner()
	request, flows := suite.removalOf(stored)
	request = append(request, request[0])
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowRoleAssignmentRemovalPre
	})).Return(&flowcommon.SystemFlowResult{}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowRevocationPost
	})).Return(&flowcommon.SystemFlowResult{}, nil).Once()
	suite.mockStore.On("RemoveAssignments", mock.Anything, "role1", mock.Anything).Return(nil)

	err := suite.service.RemoveAssignments(context.Background(), "role1", "", request)

	suite.Nil(err)
}

// A post flow that fails after the removal is written is reported, not passed over, and the other
// assignees' post flows still run.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_FailedPostFlowIsReported() {
	const otherUser = "user2"
	stored := append(heldByOwner(), RoleAssignment{ID: otherUser, Type: assigneeTypeEntity, OUID: testOwnerOU})
	flows := newSystemFlowProviderMock(suite.T())
	suite.Require().NoError(SetAssignmentSystemFlowProvider(suite.service, flows))
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, mock.MatchedBy(func(ids []string) bool {
		sorted := slices.Sorted(slices.Values(ids))
		return slices.Equal(sorted, []string{testUserID1, otherUser})
	})).Return([]providers.Entity{ownedUser(),
		{ID: otherUser, Category: providers.EntityCategoryUser, OUID: testOwnerOU}}, nil)
	suite.ownedRole("role1")
	suite.mockStore.On("ListMandatedAssignmentsForRole", mock.Anything, "role1", testOwnerOU).
		Return([]RoleAssignment{}, nil)
	suite.mockStore.On("GetDirectAssignments", mock.Anything, "role1", testOwnerOU,
		[]string{testUserID1, otherUser}).Return(stored, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowRoleAssignmentRemovalPre
	})).Return(&flowcommon.SystemFlowResult{}, nil).Twice()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowRevocationPost && c.Inputs[systemFlowInputAssignee] == testUserID1
	})).Return(nil, &tidcommon.InternalServerError).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowRevocationPost && c.Inputs[systemFlowInputAssignee] == otherUser
	})).Return(&flowcommon.SystemFlowResult{}, nil).Once()
	suite.mockStore.On("RemoveAssignments", mock.Anything, "role1", stored).Return(nil)

	err := suite.service.RemoveAssignments(context.Background(), "role1", "", []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser}, {ID: otherUser, Type: AssigneeTypeUser}})

	suite.Equal(&tidcommon.InternalServerError, err)
}

// Removing an assignment the role does not hold changes nothing, so no flow runs.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_AnAssignmentNotHeldRunsNoFlow() {
	request, _ := suite.removalOf(nil)
	suite.mockStore.On("RemoveAssignments", mock.Anything, "role1", mock.Anything).Return(nil)

	err := suite.service.RemoveAssignments(context.Background(), "role1", "", request)

	suite.Nil(err)
}

// A refused pre flow leaves every assignment as it was.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_RefusedFlowRemovesNothing() {
	request, flows := suite.removalOf(heldByOwner())
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).
		Return(nil, &tidcommon.InternalServerError).Once()

	err := suite.service.RemoveAssignments(context.Background(), "role1", "", request)

	suite.Equal(&tidcommon.InternalServerError, err)
	suite.mockStore.AssertNotCalled(suite.T(), "RemoveAssignments", mock.Anything, mock.Anything, mock.Anything)
}

// A failed read of the direct assignments removes nothing and runs no flow.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_FailedHeldReadRemovesNothing() {
	suite.Require().NoError(
		SetAssignmentSystemFlowProvider(suite.service, newSystemFlowProviderMock(suite.T())))
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
		Return([]providers.Entity{ownedUser()}, nil)
	suite.ownedRole("role1")
	suite.mockStore.On("GetDirectAssignments", mock.Anything, "role1", testOwnerOU, []string{testUserID1}).
		Return(nil, errors.New("store error"))

	err := suite.service.RemoveAssignments(context.Background(), "role1", "",
		[]RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}})

	suite.Equal(&tidcommon.InternalServerError, err)
	suite.mockStore.AssertNotCalled(suite.T(), "RemoveAssignments", mock.Anything, mock.Anything, mock.Anything)
}

// Adding an assignment revokes nothing, so no flow runs.
func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_RunsNoFlow() {
	suite.Require().NoError(
		SetAssignmentSystemFlowProvider(suite.service, newSystemFlowProviderMock(suite.T())))
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
		Return([]providers.Entity{ownedUser()}, nil)
	suite.ownedRole("role1")
	suite.mockStore.On("AddAssignments", mock.Anything, "role1", heldByOwner()).Return(nil)

	err := suite.service.AddAssignments(context.Background(), "role1", "",
		[]RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}})

	suite.Nil(err)
}

// The provider is bound only to the service implementation.
func (suite *RoleAssignmentServiceTestSuite) TestSetAssignmentSystemFlowProvider() {
	provider := newSystemFlowProviderMock(suite.T())

	suite.NoError(SetAssignmentSystemFlowProvider(suite.service, provider))
	suite.Equal(provider, suite.service.(*roleAssignmentService).systemFlows)
	suite.Error(SetAssignmentSystemFlowProvider(NewRoleAssignmentServiceInterfaceMock(suite.T()), provider))
}
