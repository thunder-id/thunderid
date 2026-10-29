// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"

	"github.com/stretchr/testify/mock"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// systemFlowRuns stands in for the execution service's ExecuteSystemFlow, which a test in this package
// cannot reach through the generated mock: flowexec imports the executors, which import this package.
// It records each run and answers with err; the step that finishes the revocation answers with
// finishErr.
type systemFlowRuns struct {
	handles   []string
	inputs    []map[string]string
	err       *tidcommon.ServiceError
	finishErr *tidcommon.ServiceError
	onRun     func()
	onFinish  func()
}

func (r *systemFlowRuns) execute(_ context.Context, handle string, inputs map[string]string) (
	func(context.Context) *tidcommon.ServiceError, *tidcommon.ServiceError) {
	r.handles = append(r.handles, handle)
	r.inputs = append(r.inputs, inputs)
	if r.onRun != nil {
		r.onRun()
	}
	if r.err != nil {
		return nil, r.err
	}
	return func(context.Context) *tidcommon.ServiceError {
		if r.onFinish != nil {
			r.onFinish()
		}
		return r.finishErr
	}, nil
}

// A role deletion runs the system flow with the role as input before anything is removed, and finishes
// its revocation once the role is gone.
func (suite *RoleServiceTestSuite) TestDeleteRole_RunsTheSystemFlowFirst() {
	var order []string
	suite.runs.onRun = func() { order = append(order, "flow") }
	suite.runs.onFinish = func() { order = append(order, "finish") }
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil)
	suite.mockStore.On("DeleteAssignmentsByRoleID", mock.Anything, "role1").
		Run(func(mock.Arguments) { order = append(order, "assignments") }).Return(nil)
	suite.mockStore.On("DeleteRole", mock.Anything, "role1").
		Run(func(mock.Arguments) { order = append(order, "role") }).Return(nil)

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.Nil(err)
	suite.Equal([]string{systemFlowRoleDelete}, suite.runs.handles)
	suite.Equal([]map[string]string{{systemFlowInputRole: "role1"}}, suite.runs.inputs)
	suite.Equal([]string{"flow", "assignments", "role", "finish"}, order)
}

// A revocation that cannot be finished after the role is deleted is reported, not passed over.
func (suite *RoleServiceTestSuite) TestDeleteRole_UnfinishedRevocationIsReported() {
	suite.runs.finishErr = &tidcommon.InternalServerError
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil)
	suite.mockStore.On("DeleteAssignmentsByRoleID", mock.Anything, "role1").Return(nil)
	suite.mockStore.On("DeleteRole", mock.Anything, "role1").Return(nil)

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.Equal(&tidcommon.InternalServerError, err)
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
