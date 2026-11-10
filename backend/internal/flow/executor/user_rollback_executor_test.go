// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	"github.com/thunder-id/thunderid/internal/flow/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type UserRollbackExecutorTestSuite struct {
	suite.Suite
	mockUserProvider *userRollbackDeleterMock
	mockFlowFactory  *coremock.FlowFactoryInterfaceMock
	executor         *userRollbackExecutor
}

func TestUserRollbackExecutorSuite(t *testing.T) {
	suite.Run(t, new(UserRollbackExecutorTestSuite))
}

func (suite *UserRollbackExecutorTestSuite) SetupTest() {
	suite.mockUserProvider = newUserRollbackDeleterMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())

	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameUserRollback, providers.ExecutorTypeUtility,
		[]providers.Input{}, []providers.Input{}, mock.Anything).
		Return(newMockExecutor("UserRollbackExecutor", providers.ExecutorTypeUtility,
			[]providers.Input{}, []providers.Input{}))

	suite.executor = newUserRollbackExecutor(
		suite.mockFlowFactory, suite.mockUserProvider)
}

func (suite *UserRollbackExecutorTestSuite) TestExecute_NoUserInRuntimeData_CompletesAsNoOp() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	suite.mockUserProvider.AssertNotCalled(suite.T(), "DeleteUser", mock.Anything, mock.Anything)
}

func (suite *UserRollbackExecutorTestSuite) TestExecute_DeletesRecordedUser_Success() {
	userID := testUserID
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			common.RuntimeKeyProvisionedUserID: userID,
		},
	}

	suite.mockUserProvider.EXPECT().DeleteUser(mock.Anything, userID).Return(nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	suite.mockUserProvider.AssertExpectations(suite.T())
}

func (suite *UserRollbackExecutorTestSuite) TestExecute_DeletionRefused_ReturnsFailure() {
	userID := testUserID
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			common.RuntimeKeyProvisionedUserID: userID,
		},
	}

	svcErr := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "USR-40001",
		Error: tidcommon.I18nMessage{Key: "error.test.blocking_dependency", DefaultValue: "has dependent resources"},
	}
	suite.mockUserProvider.EXPECT().DeleteUser(mock.Anything, userID).Return(svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, result.Status)
	assert.Equal(suite.T(), &ErrUserRollbackFailed, result.Error)
}

func (suite *UserRollbackExecutorTestSuite) TestExecute_DeletionServerError_ReturnsError() {
	userID := testUserID
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			common.RuntimeKeyProvisionedUserID: userID,
		},
	}

	svcErr := &tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "USR-50001",
		Error: tidcommon.I18nMessage{Key: "error.test.internal_error", DefaultValue: "internal error"},
	}
	suite.mockUserProvider.EXPECT().DeleteUser(mock.Anything, userID).Return(svcErr)

	result, err := suite.executor.Execute(ctx)

	require.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to delete user")
}

func (suite *UserRollbackExecutorTestSuite) TestExecute_ForwardFailure_FailsWithOriginalErrorAfterRollback() {
	userID := testUserID
	ctx := &providers.NodeContext{
		ExecutionID:    "flow-123",
		NodeProperties: map[string]interface{}{propertyKeyForwardFailure: true},
		RuntimeData:    map[string]string{common.RuntimeKeyProvisionedUserID: userID, "failureReasonJSON": forwardedFailureJSON},
	}
	suite.mockUserProvider.EXPECT().DeleteUser(mock.Anything, userID).Return(nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, result.Status)
	require.NotNil(suite.T(), result.Error)
	assert.Equal(suite.T(), "FET-1010", result.Error.Code)
}

func (suite *UserRollbackExecutorTestSuite) TestExecute_ForwardFailure_RollbackErrorWins() {
	userID := testUserID
	ctx := &providers.NodeContext{
		ExecutionID:    "flow-123",
		NodeProperties: map[string]interface{}{propertyKeyForwardFailure: true},
		RuntimeData:    map[string]string{common.RuntimeKeyProvisionedUserID: userID, "failureReasonJSON": forwardedFailureJSON},
	}
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "USR-40001"}
	suite.mockUserProvider.EXPECT().DeleteUser(mock.Anything, userID).Return(svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, result.Status)
	assert.Equal(suite.T(), &ErrUserRollbackFailed, result.Error)
}

func (suite *UserRollbackExecutorTestSuite) TestExecute_UserIDNotCreatedByThisExecution_DoesNothing() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{userAttributeUserID: testUserID},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	suite.mockUserProvider.AssertNotCalled(suite.T(), "DeleteUser", mock.Anything, mock.Anything)
}
