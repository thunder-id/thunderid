// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

type OUDeleteExecutorTestSuite struct {
	suite.Suite
	mockOUService   *oumock.OrganizationUnitServiceInterfaceMock
	mockFlowFactory *coremock.FlowFactoryInterfaceMock
	executor        *ouDeleteExecutor
}

func TestOUDeleteExecutorSuite(t *testing.T) {
	suite.Run(t, new(OUDeleteExecutorTestSuite))
}

func (suite *OUDeleteExecutorTestSuite) SetupTest() {
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())

	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameOUDelete, providers.ExecutorTypeUtility,
		[]providers.Input{}, []providers.Input{}, mock.Anything).
		Return(newMockExecutor("OUDeleteExecutor", providers.ExecutorTypeUtility,
			[]providers.Input{}, []providers.Input{}))

	suite.executor = newOUDeleteExecutor(suite.mockFlowFactory, suite.mockOUService)
}

func (suite *OUDeleteExecutorTestSuite) TestExecute_NoOUInRuntimeData_CompletesAsNoOp() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	suite.mockOUService.AssertNotCalled(suite.T(), "DeleteOrganizationUnit", mock.Anything, mock.Anything)
}

func (suite *OUDeleteExecutorTestSuite) TestExecute_DeletesRecordedOU_Success() {
	ouID := testOUID
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			createdOUIDKey: ouID,
		},
	}

	suite.mockOUService.On("DeleteOrganizationUnit", mock.Anything, ouID).
		Return((*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *OUDeleteExecutorTestSuite) TestExecute_DeletionRefused_ReturnsFailure() {
	ouID := testOUID
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			createdOUIDKey: ouID,
		},
	}

	svcErr := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "OU-40001",
		Error: tidcommon.I18nMessage{Key: "error.test.blocking_dependency", DefaultValue: "has dependent resources"},
	}
	suite.mockOUService.On("DeleteOrganizationUnit", mock.Anything, ouID).Return(svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, result.Status)
	assert.Equal(suite.T(), &ErrOUDeletionFailed, result.Error)
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *OUDeleteExecutorTestSuite) TestExecute_ServerError_ReturnsError() {
	ouID := testOUID
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			createdOUIDKey: ouID,
		},
	}

	svcErr := &tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "OU-50001",
		Error: tidcommon.I18nMessage{Key: "error.test.internal_error", DefaultValue: "internal error"},
	}
	suite.mockOUService.On("DeleteOrganizationUnit", mock.Anything, ouID).Return(svcErr)

	result, err := suite.executor.Execute(ctx)

	require.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to delete organization unit")
	suite.mockOUService.AssertExpectations(suite.T())
}

const forwardedFailureJSON = `{"code":"FET-1010","type":"client_error","error":{"key":"k","defaultValue":"Provisioning failed"}}`

func (suite *OUDeleteExecutorTestSuite) TestExecute_ForwardFailure_FailsWithOriginalErrorAfterRollback() {
	ouID := testOUID
	ctx := &providers.NodeContext{
		ExecutionID:    "flow-123",
		NodeProperties: map[string]interface{}{propertyKeyForwardFailure: true},
		RuntimeData:    map[string]string{createdOUIDKey: ouID, "failureReasonJSON": forwardedFailureJSON},
	}
	suite.mockOUService.EXPECT().DeleteOrganizationUnit(mock.Anything, ouID).Return(nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, result.Status)
	require.NotNil(suite.T(), result.Error)
	assert.Equal(suite.T(), "FET-1010", result.Error.Code)
	assert.Equal(suite.T(), "Provisioning failed", result.Error.Error.DefaultValue)
}

func (suite *OUDeleteExecutorTestSuite) TestExecute_ForwardFailure_NoOUStillForwards() {
	ctx := &providers.NodeContext{
		ExecutionID:    "flow-123",
		NodeProperties: map[string]interface{}{propertyKeyForwardFailure: true},
		RuntimeData:    map[string]string{"failureReasonJSON": forwardedFailureJSON},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, result.Status)
	require.NotNil(suite.T(), result.Error)
	assert.Equal(suite.T(), "FET-1010", result.Error.Code)
}

func (suite *OUDeleteExecutorTestSuite) TestExecute_ForwardFailure_WithoutFailureCompletes() {
	ouID := testOUID
	ctx := &providers.NodeContext{
		ExecutionID:    "flow-123",
		NodeProperties: map[string]interface{}{propertyKeyForwardFailure: true},
		RuntimeData:    map[string]string{createdOUIDKey: ouID},
	}
	suite.mockOUService.EXPECT().DeleteOrganizationUnit(mock.Anything, ouID).Return(nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
}

func (suite *OUDeleteExecutorTestSuite) TestExecute_FailureNotForwardedWithoutProperty() {
	ouID := testOUID
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{createdOUIDKey: ouID, "failureReasonJSON": forwardedFailureJSON},
	}
	suite.mockOUService.EXPECT().DeleteOrganizationUnit(mock.Anything, ouID).Return(nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
}

func (suite *OUDeleteExecutorTestSuite) TestExecute_OUNotCreatedByThisExecution_DoesNothing() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{ouIDKey: testOUID},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	suite.mockOUService.AssertNotCalled(suite.T(), "DeleteOrganizationUnit", mock.Anything, mock.Anything)
}
