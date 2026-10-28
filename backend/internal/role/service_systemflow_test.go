// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"

	"github.com/stretchr/testify/mock"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// systemFlowRuns stands in for the execution service's ExecuteSystemFlow, which a test in this package
// cannot reach through the generated mock: flowexec imports the executors, which import this package.
// It records each run and answers with err.
type systemFlowRuns struct {
	handles []string
	inputs  []map[string]string
	err     *tidcommon.ServiceError
	onRun   func()
}

func (r *systemFlowRuns) execute(_ context.Context, handle string,
	inputs map[string]string) *tidcommon.ServiceError {
	r.handles = append(r.handles, handle)
	r.inputs = append(r.inputs, inputs)
	if r.onRun != nil {
		r.onRun()
	}
	return r.err
}

// A role deletion runs the system flow with the role as input before anything is removed.
func (suite *RoleServiceTestSuite) TestDeleteRole_RunsTheSystemFlowFirst() {
	var order []string
	suite.runs.onRun = func() { order = append(order, "flow") }
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil)
	suite.mockStore.On("DeleteAssignmentsByRoleID", mock.Anything, "role1").
		Run(func(mock.Arguments) { order = append(order, "assignments") }).Return(nil)
	suite.mockStore.On("DeleteRole", mock.Anything, "role1").
		Run(func(mock.Arguments) { order = append(order, "role") }).Return(nil)

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.Nil(err)
	suite.Equal([]string{systemFlowRoleDelete}, suite.runs.handles)
	suite.Equal([]map[string]string{{systemFlowInputRole: "role1"}}, suite.runs.inputs)
	suite.Equal([]string{"flow", "assignments", "role"}, order, "something was removed before the revocation")
}

// A refused system flow leaves the role and its assignments as they were.
func (suite *RoleServiceTestSuite) TestDeleteRole_RefusedFlowRemovesNothing() {
	suite.runs.err = &tidcommon.InternalServerError
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil)

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.Equal(&tidcommon.InternalServerError, err)
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteAssignmentsByRoleID", mock.Anything, mock.Anything)
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteRole", mock.Anything, mock.Anything)
}

// Without an executor the deletion is refused, so a server that forgot the wiring fails closed.
func (suite *RoleServiceTestSuite) TestDeleteRole_WithoutAnExecutorIsRefused() {
	suite.service.SetSystemFlowExecutor(nil)
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil)

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.Equal(&tidcommon.InternalServerError, err)
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteRole", mock.Anything, mock.Anything)
}

// A role that is already gone runs no flow.
func (suite *RoleServiceTestSuite) TestDeleteRole_MissingRoleRunsNoFlow() {
	suite.mockStore.On("IsRoleExist", mock.Anything, "nonexistent").Return(false, nil)

	err := suite.service.DeleteRole(context.Background(), "nonexistent")

	suite.Nil(err)
	suite.Empty(suite.runs.handles)
}

// removalOf readies the suite to remove testUserID1 from role1 while the role holds the given
// assignments, and returns the request and the assignments as stored.
func (suite *RoleAssignmentServiceTestSuite) removalOf(held []RoleAssignment) []RoleAssignment {
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
		Return([]providers.Entity{{ID: testUserID1, Category: providers.EntityCategoryUser}}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(len(held), nil)
	if len(held) > 0 {
		suite.mockStore.On("GetRoleAssignments", mock.Anything, "role1", serverconst.MaxPageSize, 0).
			Return(held, nil)
	}
	return []RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}}
}

// Removing a held assignment runs the system flow with the role and the assignee before the write.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_RunsTheSystemFlowFirst() {
	stored := []RoleAssignment{{ID: testUserID1, Type: assigneeTypeEntity}}
	request := suite.removalOf(stored)
	var order []string
	suite.runs.onRun = func() { order = append(order, "flow") }
	suite.mockStore.On("RemoveAssignments", mock.Anything, "role1", stored).
		Run(func(mock.Arguments) { order = append(order, "write") }).Return(nil)

	err := suite.service.RemoveAssignments(context.Background(), "role1", request)

	suite.Nil(err)
	suite.Equal([]string{systemFlowRoleAssignmentRemoval}, suite.runs.handles)
	suite.Equal([]map[string]string{{systemFlowInputRole: "role1", systemFlowInputAssignee: testUserID1}},
		suite.runs.inputs)
	suite.Equal([]string{"flow", "write"}, order, "the assignment was removed before the revocation")
}

// Removing an assignment the role does not hold changes nothing, so it revokes nothing.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_AnAssignmentNotHeldRunsNoFlow() {
	request := suite.removalOf(nil)
	suite.mockStore.On("RemoveAssignments", mock.Anything, "role1", mock.Anything).Return(nil)

	err := suite.service.RemoveAssignments(context.Background(), "role1", request)

	suite.Nil(err)
	suite.Empty(suite.runs.handles)
}

// A refused system flow leaves every assignment as it was.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_RefusedFlowRemovesNothing() {
	request := suite.removalOf([]RoleAssignment{{ID: testUserID1, Type: assigneeTypeEntity}})
	suite.runs.err = &tidcommon.InternalServerError

	err := suite.service.RemoveAssignments(context.Background(), "role1", request)

	suite.Equal(&tidcommon.InternalServerError, err)
	suite.mockStore.AssertNotCalled(suite.T(), "RemoveAssignments", mock.Anything, mock.Anything, mock.Anything)
}

// Without an executor the removal of a held assignment is refused, so a server that forgot the wiring
// fails closed.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_WithoutAnExecutorIsRefused() {
	request := suite.removalOf([]RoleAssignment{{ID: testUserID1, Type: assigneeTypeEntity}})
	suite.service.SetSystemFlowExecutor(nil)

	err := suite.service.RemoveAssignments(context.Background(), "role1", request)

	suite.Equal(&tidcommon.InternalServerError, err)
	suite.mockStore.AssertNotCalled(suite.T(), "RemoveAssignments", mock.Anything, mock.Anything, mock.Anything)
}
