// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/sessionmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/revocationmock"
)

type UserRollbackExecutorTestSuite struct {
	suite.Suite
	mockUserProvider *userDeletionProviderMock
	mockRevoker      *revocationmock.CriteriaRevokerInterfaceMock
	mockSessionSvc   *sessionmock.ServiceMock
	mockFlowFactory  *coremock.FlowFactoryInterfaceMock
	executor         *userRollbackExecutor
}

func TestUserRollbackExecutorSuite(t *testing.T) {
	suite.Run(t, new(UserRollbackExecutorTestSuite))
}

func (suite *UserRollbackExecutorTestSuite) SetupTest() {
	suite.mockUserProvider = newUserDeletionProviderMock(suite.T())
	suite.mockRevoker = revocationmock.NewCriteriaRevokerInterfaceMock(suite.T())
	suite.mockSessionSvc = sessionmock.NewServiceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())

	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameUserRollback, providers.ExecutorTypeUtility,
		[]providers.Input{}, []providers.Input{}, mock.Anything).
		Return(newMockExecutor("UserRollbackExecutor", providers.ExecutorTypeUtility,
			[]providers.Input{}, []providers.Input{}))

	suite.executor = newUserRollbackExecutor(
		suite.mockFlowFactory, suite.mockUserProvider, suite.mockRevoker, suite.mockSessionSvc)
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
	suite.mockRevoker.AssertNotCalled(suite.T(), "RevokeByCriteria", mock.Anything, mock.Anything)
	suite.mockSessionSvc.AssertNotCalled(suite.T(), "TerminateBySubject", mock.Anything, mock.Anything)
}

func (suite *UserRollbackExecutorTestSuite) TestExecute_RevokesTerminatesAndDeletes_Success() {
	userID := testUserID
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			userAttributeUserID: userID,
		},
	}

	suite.mockRevoker.EXPECT().RevokeByCriteria(mock.Anything, revocation.CriteriaRevocation{
		Criterion: revocation.Criterion{Type: revocation.CriterionTypeSubject, Value: userID},
		Mode:      revocation.ModeAll,
		Reason:    revocation.ReasonUserDeleted,
	}).Return(nil)
	suite.mockSessionSvc.EXPECT().TerminateBySubject(mock.Anything, userID).Return(nil)
	suite.mockUserProvider.EXPECT().DeleteUser(mock.Anything, userID).Return(nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	suite.mockRevoker.AssertExpectations(suite.T())
	suite.mockSessionSvc.AssertExpectations(suite.T())
	suite.mockUserProvider.AssertExpectations(suite.T())
}

func (suite *UserRollbackExecutorTestSuite) TestExecute_RevocationFails_ReturnsErrorWithoutDeleting() {
	userID := testUserID
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			userAttributeUserID: userID,
		},
	}

	suite.mockRevoker.EXPECT().RevokeByCriteria(mock.Anything, mock.Anything).
		Return(errors.New("revocation store unavailable"))

	result, err := suite.executor.Execute(ctx)

	require.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to revoke tokens for the user")
	suite.mockSessionSvc.AssertNotCalled(suite.T(), "TerminateBySubject", mock.Anything, mock.Anything)
	suite.mockUserProvider.AssertNotCalled(suite.T(), "DeleteUser", mock.Anything, mock.Anything)
}

func (suite *UserRollbackExecutorTestSuite) TestExecute_SessionTerminationFails_ReturnsErrorWithoutDeleting() {
	userID := testUserID
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			userAttributeUserID: userID,
		},
	}

	suite.mockRevoker.EXPECT().RevokeByCriteria(mock.Anything, mock.Anything).Return(nil)
	suite.mockSessionSvc.EXPECT().TerminateBySubject(mock.Anything, userID).
		Return(errors.New("session store unavailable"))

	result, err := suite.executor.Execute(ctx)

	require.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to terminate sessions for the user")
	suite.mockUserProvider.AssertNotCalled(suite.T(), "DeleteUser", mock.Anything, mock.Anything)
}

func (suite *UserRollbackExecutorTestSuite) TestExecute_DeletionRefused_ReturnsFailure() {
	userID := testUserID
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			userAttributeUserID: userID,
		},
	}

	suite.mockRevoker.EXPECT().RevokeByCriteria(mock.Anything, mock.Anything).Return(nil)
	suite.mockSessionSvc.EXPECT().TerminateBySubject(mock.Anything, userID).Return(nil)
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
			userAttributeUserID: userID,
		},
	}

	suite.mockRevoker.EXPECT().RevokeByCriteria(mock.Anything, mock.Anything).Return(nil)
	suite.mockSessionSvc.EXPECT().TerminateBySubject(mock.Anything, userID).Return(nil)
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
