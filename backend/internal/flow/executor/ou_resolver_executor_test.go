// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/ouprovidermock"
)

const testParentOUID = "parent-ou-123"
const testChildOUID = "child-ou-456"

type OUResolverExecutorTestSuite struct {
	suite.Suite
	mockFlowFactory *coremock.FlowFactoryInterfaceMock
	mockOUService   *ouprovidermock.OrganizationUnitProviderMock
	executor        *ouResolverExecutor
}

func (suite *OUResolverExecutorTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockOUService = ouprovidermock.NewOrganizationUnitProviderMock(suite.T())

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

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_ParentOuIdProperty_UsedWithoutDefaultOUID() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPrompt,
			common.NodePropertyOUParent:      " " + testParentOUID + " ",
		},
		RuntimeData: map[string]string{},
		UserInputs:  map[string]string{},
	}
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{TotalResults: 2}, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Equal(suite.T(), testParentOUID, result.AdditionalData[common.DataRootOUID])
	assert.NotContains(suite.T(), result.RuntimeData, defaultOUIDKey)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_ParentOuIdProperty_TakesPrecedenceOverDefaultOUID() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPrompt,
			common.NodePropertyOUParent:      testParentOUID,
		},
		RuntimeData: map[string]string{defaultOUIDKey: "other-ou"},
		UserInputs:  map[string]string{ouIDKey: testChildOUID},
	}
	suite.mockOUService.On("IsParent", mock.Anything, testParentOUID, testChildOUID).
		Return(true, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	assert.Equal(suite.T(), testChildOUID, result.RuntimeData[ouIDKey])
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_ParentOuIdProperty_NotAStringIsIgnored() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPrompt,
			common.NodePropertyOUParent:      42,
		},
		RuntimeData: map[string]string{},
		UserInputs:  map[string]string{},
	}

	result, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_ParentOuIdProperty_IgnoredOutsideAuthenticationFlows() {
	for _, flowType := range []providers.FlowType{providers.FlowTypeRegistration, providers.FlowTypeUserOnboarding} {
		ctx := &providers.NodeContext{
			ExecutionID: "flow-123",
			FlowType:    flowType,
			NodeProperties: map[string]interface{}{
				common.NodePropertyOUResolveFrom: ouResolveFromPrompt,
				common.NodePropertyOUParent:      testParentOUID,
			},
			RuntimeData: map[string]string{},
			UserInputs:  map[string]string{},
		}

		result, err := suite.executor.Execute(ctx)

		assert.Error(suite.T(), err, string(flowType))
		assert.Nil(suite.T(), result, string(flowType))
	}
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_NoParentOU_ErrorNamesParentOuIdInSignInOnly() {
	for flowType, mentionsProperty := range map[providers.FlowType]bool{
		providers.FlowTypeAuthentication: true,
		providers.FlowTypeRegistration:   false,
	} {
		ctx := &providers.NodeContext{
			ExecutionID:    "flow-123",
			FlowType:       flowType,
			NodeProperties: map[string]interface{}{common.NodePropertyOUResolveFrom: ouResolveFromPrompt},
			RuntimeData:    map[string]string{},
			UserInputs:     map[string]string{},
		}

		_, err := suite.executor.Execute(ctx)

		assert.Error(suite.T(), err, string(flowType))
		assert.Contains(suite.T(), err.Error(), "UserTypeResolver must run before OUResolver", string(flowType))
		assert.Equal(suite.T(), mentionsProperty, strings.Contains(err.Error(), "parentOuId"), string(flowType))
	}
}

func (suite *OUResolverExecutorTestSuite) directChildrenCtx(selected string) *providers.NodeContext {
	return &providers.NodeContext{
		ExecutionID: "flow-123",
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom:        ouResolveFromPrompt,
			common.NodePropertyOUDirectChildrenOnly: true,
		},
		RuntimeData: map[string]string{defaultOUIDKey: testParentOUID},
		UserInputs:  map[string]string{ouIDKey: selected},
	}
}

func (suite *OUResolverExecutorTestSuite) handleFilterFor(handle string) interface{} {
	return mock.MatchedBy(func(f *tidcommon.FilterGroup) bool {
		return f != nil && len(f.Clauses) == 1 && f.Clauses[0].Expr.Attribute == "handle" &&
			f.Clauses[0].Expr.Operator == tidcommon.OperatorEq && f.Clauses[0].Expr.Value == handle
	})
}

func (suite *OUResolverExecutorTestSuite) noHandleMatch(selection string) {
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0,
		suite.handleFilterFor(selection)).
		Return(&providers.OrganizationUnitListResponse{}, (*tidcommon.ServiceError)(nil))
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_AcceptsHandleOfDirectChild() {
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0,
		suite.handleFilterFor("tenant-a")).
		Return(&providers.OrganizationUnitListResponse{
			TotalResults:      1,
			OrganizationUnits: []providers.OrganizationUnitBasic{{ID: testChildOUID, Handle: "tenant-a"}},
		}, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(suite.directChildrenCtx("tenant-a"))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	assert.Equal(suite.T(), testChildOUID, result.RuntimeData[ouIDKey])
	suite.mockOUService.AssertNotCalled(suite.T(), "GetOrganizationUnit", mock.Anything, mock.Anything)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_AcceptsIDOfDirectChild() {
	suite.noHandleMatch(testChildOUID)
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, testChildOUID).
		Return(providers.OrganizationUnit{ID: testChildOUID, Parent: testParentOUID}, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(suite.directChildrenCtx(testChildOUID))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	assert.Equal(suite.T(), testChildOUID, result.RuntimeData[ouIDKey])
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_RejectsDeeperDescendant() {
	suite.noHandleMatch("grandchild")
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "grandchild").
		Return(providers.OrganizationUnit{ID: "grandchild", Parent: testChildOUID}, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(suite.directChildrenCtx("grandchild"))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Equal(suite.T(), &ErrOUNotDirectChild, result.Error)
	assert.NotContains(suite.T(), result.RuntimeData, ouIDKey)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_UnknownValueAsksAgain() {
	suite.noHandleMatch("nowhere")
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "nowhere").
		Return(providers.OrganizationUnit{}, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})

	result, err := suite.executor.Execute(suite.directChildrenCtx("nowhere"))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Equal(suite.T(), &ErrInvalidOU, result.Error)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_HandleLookupClientErrorAsksAgain() {
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0, mock.Anything).
		Return((*providers.OrganizationUnitListResponse)(nil), &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})

	result, err := suite.executor.Execute(suite.directChildrenCtx("tenant-a"))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Equal(suite.T(), &ErrInvalidOU, result.Error)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_HandleLookupServerErrorFails() {
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0, mock.Anything).
		Return((*providers.OrganizationUnitListResponse)(nil), &tidcommon.InternalServerError)

	result, err := suite.executor.Execute(suite.directChildrenCtx("tenant-a"))

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_IDLookupServerErrorFails() {
	suite.noHandleMatch(testChildOUID)
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, testChildOUID).
		Return(providers.OrganizationUnit{}, &tidcommon.InternalServerError)

	result, err := suite.executor.Execute(suite.directChildrenCtx(testChildOUID))

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_AcceptsStringProperty() {
	ctx := suite.directChildrenCtx("nowhere")
	ctx.NodeProperties[common.NodePropertyOUDirectChildrenOnly] = "true"
	suite.noHandleMatch("nowhere")
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "nowhere").
		Return(providers.OrganizationUnit{}, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), &ErrInvalidOU, result.Error)
	suite.mockOUService.AssertNotCalled(suite.T(), "IsParent", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_WithoutDirectChildrenOnly_AcceptsDescendant() {
	ctx := suite.directChildrenCtx("grandchild")
	delete(ctx.NodeProperties, common.NodePropertyOUDirectChildrenOnly)
	suite.mockOUService.On("IsParent", mock.Anything, testParentOUID, "grandchild").
		Return(true, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	suite.mockOUService.AssertNotCalled(suite.T(), "GetOrganizationUnit", mock.Anything, mock.Anything)
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
