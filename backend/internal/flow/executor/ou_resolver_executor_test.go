/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package executor

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

const testParentOUID = "parent-ou-123"
const testChildOUID = "child-ou-456"

type OUResolverExecutorTestSuite struct {
	suite.Suite
	mockFlowFactory *coremock.FlowFactoryInterfaceMock
	mockOUService   *oumock.OrganizationUnitServiceInterfaceMock
	executor        *ouResolverExecutor
}

func (suite *OUResolverExecutorTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())

	defaultInputs := []providers.Input{
		{
			Ref:        "ou_selection_input",
			Identifier: ouIDKey,
			Type:       providers.InputTypeOUSelect,
			Required:   true,
		},
	}

	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNameOUResolver,
		providers.ExecutorTypeUtility,
		defaultInputs,
		[]providers.Input{}, mock.Anything).Return(
		newMockExecutor("OUResolverExecutor", providers.ExecutorTypeUtility, defaultInputs, []providers.Input{}))

	suite.executor = newOUResolverExecutor(suite.mockFlowFactory, suite.mockOUService)
}

// --- Caller strategy tests ---

func (suite *OUResolverExecutorTestSuite) TestExecute_ResolveFromCaller_Success() {
	callerOUID := "caller-ou-123"
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"caller-user", callerOUID, "token",
		[]string{"system"}, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromCaller,
		},
		RuntimeData: map[string]string{
			defaultOUIDKey: "default-ou-456",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), callerOUID, resp.RuntimeData[ouIDKey])
}

func (suite *OUResolverExecutorTestSuite) TestExecute_ResolveFromCaller_CallerOUMissing() {
	httpCtx := context.Background()
	// Security context without OU.
	authCtx := security.NewSecurityContextForTest(
		"caller-user", "", "token",
		[]string{"system"}, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromCaller,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrOUResolutionFailed.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_ResolveFromNotConfigured() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"caller-user", "caller-ou-123", "token",
		[]string{"system"}, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &providers.NodeContext{
		ExecutionID:    "test-flow",
		Context:        httpCtx,
		NodeProperties: map[string]interface{}{},
		RuntimeData: map[string]string{
			defaultOUIDKey: "default-ou-456",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Empty(suite.T(), resp.RuntimeData[ouIDKey])
}

func (suite *OUResolverExecutorTestSuite) TestExecute_UnsupportedResolveFrom() {
	httpCtx := context.Background()

	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: "unsupported",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.Error.ErrorDescription.String(),
		"Unsupported OU resolution strategy: unsupported")
}

func (suite *OUResolverExecutorTestSuite) TestExecute_PropertyMissing() {
	httpCtx := context.Background()

	ctx := &providers.NodeContext{
		ExecutionID:    "test-flow",
		Context:        httpCtx,
		NodeProperties: map[string]interface{}{},
		RuntimeData: map[string]string{
			defaultOUIDKey: "default-ou-456",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Empty(suite.T(), resp.RuntimeData[ouIDKey])
}

func (suite *OUResolverExecutorTestSuite) TestExecute_NilNodeProperties() {
	httpCtx := context.Background()

	ctx := &providers.NodeContext{
		ExecutionID:    "test-flow",
		Context:        httpCtx,
		NodeProperties: nil,
		RuntimeData: map[string]string{
			defaultOUIDKey: "default-ou-456",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Empty(suite.T(), resp.RuntimeData[ouIDKey])
}

func (suite *OUResolverExecutorTestSuite) TestExecute_PropertyWrongType() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"caller-user", "caller-ou-123", "token",
		[]string{"system"}, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: 123, // Not a string.
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Empty(suite.T(), resp.RuntimeData[ouIDKey])
}

func (suite *OUResolverExecutorTestSuite) TestExecute_NilContext() {
	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     nil,
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromCaller,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrOUResolutionFailed.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

// --- Prompt strategy tests ---

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_NoDefaultOUID_ReturnsError() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPrompt,
		},
		RuntimeData: map[string]string{},
		UserInputs:  map[string]string{},
	}

	result, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "no defaultOUID in runtime data")
	suite.mockOUService.AssertNotCalled(
		suite.T(), "GetOrganizationUnitChildren", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_UserSelectedOU_Valid() {
	parentOUID := testParentOUID
	selectedOUID := testChildOUID

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPrompt,
		},
		RuntimeData: map[string]string{
			defaultOUIDKey: parentOUID,
		},
		UserInputs: map[string]string{
			ouIDKey: selectedOUID,
		},
	}

	suite.mockOUService.On("IsParent", mock.Anything, parentOUID, selectedOUID).
		Return(true, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	assert.Equal(suite.T(), selectedOUID, result.RuntimeData[ouIDKey])
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_UserSelectedOU_NotInSubtree() {
	parentOUID := testParentOUID
	selectedOUID := "unrelated-ou-789"

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPrompt,
		},
		RuntimeData: map[string]string{
			defaultOUIDKey: parentOUID,
		},
		UserInputs: map[string]string{
			ouIDKey: selectedOUID,
		},
	}

	suite.mockOUService.On("IsParent", mock.Anything, parentOUID, selectedOUID).
		Return(false, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Contains(suite.T(), result.Error.ErrorDescription.DefaultValue,
		ErrOUNotValidForUserType.ErrorDescription.DefaultValue)
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_UserSelectedOU_ServerError() {
	parentOUID := testParentOUID
	selectedOUID := testChildOUID

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPrompt,
		},
		RuntimeData: map[string]string{
			defaultOUIDKey: parentOUID,
		},
		UserInputs: map[string]string{
			ouIDKey: selectedOUID,
		},
	}

	svcErr := &tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "OU-50001",
		Error: tidcommon.I18nMessage{Key: "error.test.internal_error", DefaultValue: "internal error"},
	}
	suite.mockOUService.On("IsParent", mock.Anything, parentOUID, selectedOUID).
		Return(false, svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to validate selected organization unit")
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_UserSelectedOU_ClientError() {
	parentOUID := testParentOUID
	selectedOUID := testChildOUID

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPrompt,
		},
		RuntimeData: map[string]string{
			defaultOUIDKey: parentOUID,
		},
		UserInputs: map[string]string{
			ouIDKey: selectedOUID,
		},
	}

	svcErr := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "OU-40001",
		Error: tidcommon.I18nMessage{Key: "error.test.not_found", DefaultValue: "not found"},
	}
	suite.mockOUService.On("IsParent", mock.Anything, parentOUID, selectedOUID).
		Return(false, svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Contains(suite.T(), result.Error.ErrorDescription.DefaultValue, "not valid")
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_NoChildOUs_Skips() {
	parentOUID := testParentOUID

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPrompt,
		},
		RuntimeData: map[string]string{
			defaultOUIDKey: parentOUID,
		},
		UserInputs: map[string]string{},
	}

	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, parentOUID, 1, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{TotalResults: 0}, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_HasChildOUs_RequestsInput() {
	parentOUID := testParentOUID

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPrompt,
		},
		RuntimeData: map[string]string{
			defaultOUIDKey: parentOUID,
		},
		UserInputs: map[string]string{},
	}

	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, parentOUID, 1, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{TotalResults: 3}, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Equal(suite.T(), parentOUID, result.AdditionalData[common.DataRootOUID])
	assert.NotEmpty(suite.T(), result.Inputs)
	assert.Equal(suite.T(), ouIDKey, result.Inputs[0].Identifier)
	assert.Equal(suite.T(), providers.InputTypeOUSelect, result.Inputs[0].Type)
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_GetChildrenError_ReturnsError() {
	parentOUID := testParentOUID

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPrompt,
		},
		RuntimeData: map[string]string{
			defaultOUIDKey: parentOUID,
		},
		UserInputs: map[string]string{},
	}

	svcErr := &tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "OU-50001",
		Error: tidcommon.I18nMessage{Key: "error.test.internal_error", DefaultValue: "internal error"},
	}
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, parentOUID, 1, 0, mock.Anything).
		Return((*providers.OrganizationUnitListResponse)(nil), svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to check child organization units")
	suite.mockOUService.AssertExpectations(suite.T())
}

// --- PromptAll strategy tests ---

func (suite *OUResolverExecutorTestSuite) TestExecute_PromptAll_FirstInvocation_RequestsInput() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPromptAll,
		},
		RuntimeData: map[string]string{},
		UserInputs:  map[string]string{},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.NotEmpty(suite.T(), result.Inputs)
	assert.Equal(suite.T(), ouIDKey, result.Inputs[0].Identifier)
	assert.Equal(suite.T(), providers.InputTypeOUSelect, result.Inputs[0].Type)
	// PromptAll should NOT set DataRootOUID (frontend shows full tree)
	assert.Empty(suite.T(), result.AdditionalData[common.DataRootOUID])
	suite.mockOUService.AssertNotCalled(suite.T(), "IsOrganizationUnitExists", mock.Anything, mock.Anything)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_PromptAll_ValidOUSelection() {
	selectedOUID := "valid-ou-123"

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPromptAll,
		},
		RuntimeData: map[string]string{},
		UserInputs: map[string]string{
			ouIDKey: selectedOUID,
		},
	}

	suite.mockOUService.On("IsOrganizationUnitExists", mock.Anything, selectedOUID).
		Return(true, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	assert.Equal(suite.T(), selectedOUID, result.RuntimeData[ouIDKey])
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *OUResolverExecutorTestSuite) TestExecute_PromptAll_NonExistentOU() {
	selectedOUID := "nonexistent-ou-999"

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPromptAll,
		},
		RuntimeData: map[string]string{},
		UserInputs: map[string]string{
			ouIDKey: selectedOUID,
		},
	}

	suite.mockOUService.On("IsOrganizationUnitExists", mock.Anything, selectedOUID).
		Return(false, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Equal(suite.T(), ErrOUNotFound.ErrorDescription.DefaultValue, result.Error.ErrorDescription.DefaultValue)
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *OUResolverExecutorTestSuite) TestExecute_PromptAll_ServiceError() {
	selectedOUID := "some-ou-123"

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPromptAll,
		},
		RuntimeData: map[string]string{},
		UserInputs: map[string]string{
			ouIDKey: selectedOUID,
		},
	}

	svcErr := &tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "OU-50001",
		Error: tidcommon.I18nMessage{Key: "error.test.internal_error", DefaultValue: "internal error"},
	}
	suite.mockOUService.On("IsOrganizationUnitExists", mock.Anything, selectedOUID).
		Return(false, svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to validate selected organization unit")
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *OUResolverExecutorTestSuite) TestExecute_PromptAll_EmptyOUInput_RequestsInput() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPromptAll,
		},
		RuntimeData: map[string]string{},
		UserInputs: map[string]string{
			ouIDKey: "",
		},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.NotEmpty(suite.T(), result.Inputs)
	suite.mockOUService.AssertNotCalled(suite.T(), "IsOrganizationUnitExists", mock.Anything, mock.Anything)
}

func TestOUResolverExecutorSuite(t *testing.T) {
	suite.Run(t, new(OUResolverExecutorTestSuite))
}
