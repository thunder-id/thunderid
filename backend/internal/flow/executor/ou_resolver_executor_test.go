// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
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

	// The real executor returns the inputs declared on the node in place of the defaults, which the resolver reads
	// in direct children mode.
	baseExecutor := newMockExecutor("OUResolverExecutor", providers.ExecutorTypeUtility, defaultInputs,
		[]providers.Input{}).(*coremock.ExecutorInterfaceMock)
	remaining := baseExecutor.ExpectedCalls[:0]
	for _, call := range baseExecutor.ExpectedCalls {
		if call.Method != "GetRequiredInputs" {
			remaining = append(remaining, call)
		}
	}
	baseExecutor.ExpectedCalls = remaining
	baseExecutor.On("GetRequiredInputs", mock.Anything).Return(
		func(ctx *providers.NodeContext) []providers.Input {
			if len(ctx.NodeInputs) > 0 {
				return ctx.NodeInputs
			}
			return defaultInputs
		})

	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNameOUResolver,
		providers.ExecutorTypeUtility,
		defaultInputs,
		[]providers.Input{}, mock.Anything).Return(baseExecutor)

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

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_NoParentOU_ErrorDependsOnFlowType() {
	for flowType, expected := range map[providers.FlowType]string{
		providers.FlowTypeAuthentication: "no parentOuId configured",
		providers.FlowTypeRegistration:   "UserTypeResolver must run before OUResolver",
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
		assert.Contains(suite.T(), err.Error(), expected, string(flowType))
	}
}

// directChildrenCtx is a prompt node limited to direct children whose declared input is declaredInput ("" declares
// none, so the default ouId input applies), with the user having submitted value under key.
func (suite *OUResolverExecutorTestSuite) directChildrenCtx(declaredInput, key, value string) *providers.NodeContext {
	properties := map[string]interface{}{
		common.NodePropertyOUResolveFrom:        ouResolveFromPrompt,
		common.NodePropertyOUDirectChildrenOnly: true,
	}
	var nodeInputs []providers.Input
	if declaredInput != "" {
		nodeInputs = []providers.Input{
			{Ref: "ou_input", Identifier: declaredInput, Type: providers.InputTypeText, Required: true},
		}
	}
	inputs := map[string]string{}
	if key != "" {
		inputs[key] = value
	}
	return &providers.NodeContext{
		ExecutionID:    "flow-123",
		NodeProperties: properties,
		NodeInputs:     nodeInputs,
		RuntimeData:    map[string]string{defaultOUIDKey: testParentOUID},
		UserInputs:     inputs,
	}
}

func (suite *OUResolverExecutorTestSuite) handleFilterFor(handle string) interface{} {
	return mock.MatchedBy(func(f *tidcommon.FilterGroup) bool {
		return f != nil && len(f.Clauses) == 1 && f.Clauses[0].Expr.Attribute == "handle" &&
			f.Clauses[0].Expr.Operator == tidcommon.OperatorEq && f.Clauses[0].Expr.Value == handle
	})
}

func (suite *OUResolverExecutorTestSuite) childWithHandle(handle string) {
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0,
		suite.handleFilterFor(handle)).
		Return(&providers.OrganizationUnitListResponse{
			TotalResults:      1,
			OrganizationUnits: []providers.OrganizationUnitBasic{{ID: testChildOUID, Handle: handle}},
		}, (*tidcommon.ServiceError)(nil))
}

func (suite *OUResolverExecutorTestSuite) noChildWithHandle(handle string) {
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0,
		suite.handleFilterFor(handle)).
		Return(&providers.OrganizationUnitListResponse{}, (*tidcommon.ServiceError)(nil))
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_ByID_AcceptsDirectChild() {
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, testChildOUID).
		Return(providers.OrganizationUnit{ID: testChildOUID, Parent: testParentOUID}, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(suite.directChildrenCtx("", ouIDKey, testChildOUID))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	assert.Equal(suite.T(), testChildOUID, result.RuntimeData[ouIDKey])
	suite.mockOUService.AssertNotCalled(suite.T(), "GetOrganizationUnitChildren",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_ByID_RejectsDeeperDescendant() {
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "grandchild").
		Return(providers.OrganizationUnit{ID: "grandchild", Parent: testChildOUID}, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(suite.directChildrenCtx(ouIDKey, ouIDKey, "grandchild"))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Equal(suite.T(), &ErrOUNotDirectChild, result.Error)
	assert.Equal(suite.T(), testParentOUID, result.AdditionalData[common.DataRootOUID])
	assert.NotContains(suite.T(), result.RuntimeData, ouIDKey)
	assert.Equal(suite.T(), ouIDKey, result.Inputs[0].Identifier)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_ByID_UnknownIDAsksAgain() {
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "nowhere").
		Return(providers.OrganizationUnit{}, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})

	result, err := suite.executor.Execute(suite.directChildrenCtx("", ouIDKey, "nowhere"))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Equal(suite.T(), &ErrInvalidOU, result.Error)
	assert.Equal(suite.T(), testParentOUID, result.AdditionalData[common.DataRootOUID])
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_ByID_ServerErrorFails() {
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, testChildOUID).
		Return(providers.OrganizationUnit{}, &tidcommon.InternalServerError)

	result, err := suite.executor.Execute(suite.directChildrenCtx(ouIDKey, ouIDKey, testChildOUID))

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_ByHandle_AcceptsDirectChild() {
	suite.childWithHandle("tenant-a")

	result, err := suite.executor.Execute(suite.directChildrenCtx(ouHandleKey, ouHandleKey, "tenant-a"))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	assert.Equal(suite.T(), testChildOUID, result.RuntimeData[ouIDKey])
	suite.mockOUService.AssertNotCalled(suite.T(), "GetOrganizationUnit", mock.Anything, mock.Anything)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_ByHandle_NoChildWithHandleAsksAgain() {
	suite.noChildWithHandle("tenant-b-team")

	result, err := suite.executor.Execute(suite.directChildrenCtx(ouHandleKey, ouHandleKey, "tenant-b-team"))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Equal(suite.T(), &ErrInvalidOU, result.Error)
	assert.Equal(suite.T(), testParentOUID, result.AdditionalData[common.DataRootOUID])
	assert.NotContains(suite.T(), result.RuntimeData, ouIDKey)
	assert.Equal(suite.T(), ouHandleKey, result.Inputs[0].Identifier)
	assert.Equal(suite.T(), providers.InputTypeText, result.Inputs[0].Type)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_ByHandle_LookupErrors() {
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0,
		suite.handleFilterFor("a")).
		Return((*providers.OrganizationUnitListResponse)(nil), &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0,
		suite.handleFilterFor("b")).
		Return((*providers.OrganizationUnitListResponse)(nil), &tidcommon.InternalServerError)

	// A client error here concerns the parent OU or the caller, not the handle the user typed, so both fail.
	clientErr, err := suite.executor.Execute(suite.directChildrenCtx(ouHandleKey, ouHandleKey, "a"))
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), clientErr)

	serverErr, err := suite.executor.Execute(suite.directChildrenCtx(ouHandleKey, ouHandleKey, "b"))
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), serverErr)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_ByHandle_IgnoresAnID() {
	ctx := suite.directChildrenCtx(ouHandleKey, ouIDKey, testChildOUID)
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{TotalResults: 2}, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Equal(suite.T(), ouHandleKey, result.Inputs[0].Identifier)
	assert.NotContains(suite.T(), result.RuntimeData, ouIDKey)
	suite.mockOUService.AssertNotCalled(suite.T(), "GetOrganizationUnit", mock.Anything, mock.Anything)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_ByID_IgnoresAHandle() {
	ctx := suite.directChildrenCtx("", ouHandleKey, "tenant-a")
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{TotalResults: 2}, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Equal(suite.T(), ouIDKey, result.Inputs[0].Identifier)
	assert.Equal(suite.T(), providers.InputTypeOUSelect, result.Inputs[0].Type)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DeclaredHandleInputIsIgnoredWithoutDirectChildrenOnly() {
	ctx := suite.directChildrenCtx(ouHandleKey, ouHandleKey, "tenant-a")
	delete(ctx.NodeProperties, common.NodePropertyOUDirectChildrenOnly)
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{TotalResults: 2}, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Equal(suite.T(), ouIDKey, result.Inputs[0].Identifier)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_DirectChildrenOnly_IgnoresStringProperty() {
	ctx := suite.directChildrenCtx("", ouIDKey, "grandchild")
	ctx.NodeProperties[common.NodePropertyOUDirectChildrenOnly] = "true"
	suite.mockOUService.On("IsParent", mock.Anything, testParentOUID, "grandchild").
		Return(true, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_WithoutDirectChildrenOnly_AcceptsDescendant() {
	ctx := suite.directChildrenCtx("", ouIDKey, "grandchild")
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
	assert.NotContains(suite.T(), result.RuntimeData, ouIDKey)
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_NoChildOUs_ParentOuIdBecomesResolvedOU() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{
			common.NodePropertyOUResolveFrom: ouResolveFromPrompt,
			common.NodePropertyOUParent:      testParentOUID,
		},
		RuntimeData: map[string]string{},
		UserInputs:  map[string]string{},
	}
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{TotalResults: 0}, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	assert.Equal(suite.T(), testParentOUID, result.RuntimeData[ouIDKey])
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

// --- Sign-in boundary tests ---

// boundedCtx is a context carrying a sign-in boundary of the application alone, as for a shared app.
func (suite *OUResolverExecutorTestSuite) boundedCtx() context.Context {
	return authnprovidercm.WithSignInBoundary(context.Background(),
		authnprovidercm.SignInBoundary{ApplicationID: "app-1"})
}

// signedInCtx is a context carrying the sign-in boundary of app-1 after user-1 signed in.
func (suite *OUResolverExecutorTestSuite) signedInCtx() context.Context {
	return authnprovidercm.WithSignInBoundary(context.Background(),
		authnprovidercm.SignInBoundary{ApplicationID: "app-1", EntityID: "user-1"})
}

// eligibleCtx is a bounded sign-in step prompting a signed-in user, with the given extra node properties.
func (suite *OUResolverExecutorTestSuite) eligibleCtx(properties map[string]interface{},
	inputs map[string]string) *providers.NodeContext {
	nodeProperties := map[string]interface{}{common.NodePropertyOUResolveFrom: ouResolveFromPrompt}
	for k, v := range properties {
		nodeProperties[k] = v
	}
	return &providers.NodeContext{
		ExecutionID:    "flow-123",
		Context:        suite.signedInCtx(),
		FlowType:       providers.FlowTypeAuthentication,
		NodeProperties: nodeProperties,
		RuntimeData:    map[string]string{},
		UserInputs:     inputs,
	}
}

// ouTree registers the tree acme > {engineering > platform, sales} with the OU service.
func (suite *OUResolverExecutorTestSuite) ouTree() {
	for _, ou := range []providers.OrganizationUnit{
		{ID: "acme-ou", Handle: "acme"},
		{ID: "eng-ou", Handle: "engineering", Parent: "acme-ou"},
		{ID: "platform-ou", Handle: "platform", Parent: "eng-ou"},
		{ID: "sales-ou", Handle: "sales", Parent: "acme-ou"},
	} {
		suite.mockOUService.On("GetOrganizationUnit", mock.Anything, ou.ID).
			Return(ou, (*tidcommon.ServiceError)(nil)).Maybe()
	}
}

func (suite *OUResolverExecutorTestSuite) eligible(ous []string) {
	suite.mockOUService.On("GetEligibleOUs", mock.Anything).Return(ous, (*tidcommon.ServiceError)(nil))
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Bounded_DecidedOUIsKeptByEveryStrategy() {
	for _, strategy := range []string{ouResolveFromCaller, ouResolveFromPrompt, ouResolveFromPromptAll} {
		suite.Run(strategy, func() {
			ctx := &providers.NodeContext{
				ExecutionID:    "flow-123",
				Context:        suite.boundedCtx(),
				FlowType:       providers.FlowTypeAuthentication,
				NodeProperties: map[string]interface{}{common.NodePropertyOUResolveFrom: strategy},
				RuntimeData:    map[string]string{ouIDKey: "decided-ou", defaultOUIDKey: testParentOUID},
				UserInputs:     map[string]string{ouIDKey: testChildOUID},
			}

			result, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), providers.ExecComplete, result.Status)
			assert.NotContains(suite.T(), result.RuntimeData, ouIDKey)
		})
	}
	suite.mockOUService.AssertNotCalled(suite.T(), "GetEligibleOUs", mock.Anything)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Eligible_NoEligibleOUFails() {
	suite.eligible([]string{})

	result, err := suite.executor.Execute(suite.eligibleCtx(nil, map[string]string{}))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, result.Status)
	assert.Equal(suite.T(), ErrNoAccessToApplication.Code, result.Error.Code)
	assert.Empty(suite.T(), result.RuntimeData[ouIDKey])
}

// A signed-in user the OU provider cannot list OUs for picks below the parent OU, as before sign-in.
func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Eligible_UnlistedUserPicksBelowTheParentOU() {
	suite.mockOUService.On("GetEligibleOUs", mock.Anything).Return([]string(nil),
		&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "OU-1020"})
	suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{TotalResults: 2}, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(suite.eligibleCtx(
		map[string]interface{}{common.NodePropertyOUParent: testParentOUID}, map[string]string{}))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	suite.Require().Len(result.Inputs, 1)
	assert.Equal(suite.T(), providers.InputTypeOUSelect, result.Inputs[0].Type)
	assert.Equal(suite.T(), testParentOUID, result.AdditionalData[common.DataRootOUID])
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Eligible_OneEligibleOUIsDecided() {
	suite.eligible([]string{"eng-ou"})

	result, err := suite.executor.Execute(suite.eligibleCtx(nil, map[string]string{}))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	assert.Equal(suite.T(), "eng-ou", result.RuntimeData[ouIDKey])
	suite.mockOUService.AssertNotCalled(suite.T(), "GetOrganizationUnit", mock.Anything, mock.Anything)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Eligible_SeveralEligibleOUsAreOfferedByHandlePath() {
	suite.eligible([]string{"sales-ou", "platform-ou", "eng-ou"})
	suite.ouTree()

	result, err := suite.executor.Execute(suite.eligibleCtx(nil, map[string]string{}))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	assert.Nil(suite.T(), result.Error)
	assert.Empty(suite.T(), result.RuntimeData[ouIDKey])
	suite.Require().Len(result.Inputs, 1)
	assert.Equal(suite.T(), ouIDKey, result.Inputs[0].Identifier)
	assert.Equal(suite.T(), providers.InputTypeSelect, result.Inputs[0].Type)
	assert.True(suite.T(), result.Inputs[0].Required)
	assert.Equal(suite.T(), []string{"acme/engineering", "acme/engineering/platform", "acme/sales"},
		result.Inputs[0].Options)
	assert.Equal(suite.T(), result.Inputs, result.ForwardedData[common.ForwardedDataKeyInputs])
	// Shared ancestors are read once.
	suite.mockOUService.AssertNumberOfCalls(suite.T(), "GetOrganizationUnit", 4)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Eligible_ChosenPathDecidesItsOU() {
	suite.eligible([]string{"sales-ou", "eng-ou"})
	suite.ouTree()

	result, err := suite.executor.Execute(suite.eligibleCtx(nil, map[string]string{ouIDKey: "acme/sales"}))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	assert.Equal(suite.T(), "sales-ou", result.RuntimeData[ouIDKey])
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Eligible_AnswerOutsideTheOptionsAsksAgain() {
	for _, answer := range []string{"acme/platform", "platform-ou", "acme"} {
		suite.Run(answer, func() {
			suite.SetupTest()
			suite.eligible([]string{"sales-ou", "eng-ou"})
			suite.ouTree()

			result, err := suite.executor.Execute(suite.eligibleCtx(nil, map[string]string{ouIDKey: answer}))

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
			assert.Equal(suite.T(), ErrInvalidOU.Code, result.Error.Code)
			assert.Empty(suite.T(), result.RuntimeData[ouIDKey])
			suite.Require().Len(result.Inputs, 1)
			assert.Equal(suite.T(), []string{"acme/engineering", "acme/sales"}, result.Inputs[0].Options)
			assert.Equal(suite.T(), result.Inputs, result.ForwardedData[common.ForwardedDataKeyInputs])
		})
	}
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Eligible_ParentOUKeepsItselfAndDescendants() {
	suite.eligible([]string{"acme-ou", "eng-ou", "platform-ou", "other-ou"})
	suite.ouTree()
	for _, ouID := range []string{"eng-ou", "platform-ou"} {
		suite.mockOUService.On("IsParent", mock.Anything, "acme-ou", ouID).
			Return(true, (*tidcommon.ServiceError)(nil))
	}
	suite.mockOUService.On("IsParent", mock.Anything, "acme-ou", "other-ou").
		Return(false, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(suite.eligibleCtx(
		map[string]interface{}{common.NodePropertyOUParent: "acme-ou"}, map[string]string{}))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
	suite.Require().Len(result.Inputs, 1)
	assert.Equal(suite.T(), []string{"acme", "acme/engineering", "acme/engineering/platform"},
		result.Inputs[0].Options)
	suite.mockOUService.AssertNotCalled(suite.T(), "IsParent", mock.Anything, "acme-ou", "acme-ou")
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Eligible_DirectChildrenOnlyKeepsDirectChildren() {
	suite.eligible([]string{"acme-ou", "platform-ou", "sales-ou"})
	suite.ouTree()

	result, err := suite.executor.Execute(suite.eligibleCtx(map[string]interface{}{
		common.NodePropertyOUParent:             "acme-ou",
		common.NodePropertyOUDirectChildrenOnly: true,
	}, map[string]string{}))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	assert.Equal(suite.T(), "sales-ou", result.RuntimeData[ouIDKey])
	suite.mockOUService.AssertNotCalled(suite.T(), "IsParent", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Eligible_DirectChildrenAreOfferedByHandle() {
	directChildren := map[string]interface{}{
		common.NodePropertyOUParent:             "acme-ou",
		common.NodePropertyOUDirectChildrenOnly: true,
	}
	tests := []struct {
		name   string
		answer string
		want   string
	}{
		{name: "NoAnswer"},
		{name: "Handle", answer: "sales", want: "sales-ou"},
		{name: "HandlePath", answer: "acme/sales"},
	}
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			suite.eligible([]string{"sales-ou", "platform-ou", "eng-ou"})
			suite.ouTree()

			inputs := map[string]string{ouIDKey: tc.answer}
			result, err := suite.executor.Execute(suite.eligibleCtx(directChildren, inputs))

			assert.NoError(suite.T(), err)
			if tc.want != "" {
				assert.Equal(suite.T(), providers.ExecComplete, result.Status)
				assert.Equal(suite.T(), tc.want, result.RuntimeData[ouIDKey])
				return
			}
			assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
			if tc.answer != "" {
				suite.Require().NotNil(result.Error)
				assert.Equal(suite.T(), ErrInvalidOU.Code, result.Error.Code)
			}
			suite.Require().Len(result.Inputs, 1)
			assert.Equal(suite.T(), []string{"engineering", "sales"}, result.Inputs[0].Options)
		})
	}
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Eligible_NothingBelowParentOUFails() {
	suite.eligible([]string{"acme-ou", "platform-ou"})
	suite.ouTree()

	result, err := suite.executor.Execute(suite.eligibleCtx(map[string]interface{}{
		common.NodePropertyOUParent:             "acme-ou",
		common.NodePropertyOUDirectChildrenOnly: true,
	}, map[string]string{}))

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, result.Status)
	assert.Equal(suite.T(), ErrNoAccessToApplication.Code, result.Error.Code)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Eligible_ServiceErrorsFail() {
	svcErr := &tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "OU-50001",
		Error: tidcommon.I18nMessage{Key: "error.test.internal_error", DefaultValue: "internal error"},
	}
	tests := []struct {
		name       string
		properties map[string]interface{}
		setup      func()
	}{
		{"EligibleOUs", nil, func() {
			suite.mockOUService.On("GetEligibleOUs", mock.Anything).Return([]string(nil), svcErr)
		}},
		{"Hierarchy", map[string]interface{}{common.NodePropertyOUParent: "acme-ou"}, func() {
			suite.eligible([]string{"eng-ou"})
			suite.mockOUService.On("IsParent", mock.Anything, "acme-ou", "eng-ou").Return(false, svcErr)
		}},
		{"HandlePath", nil, func() {
			suite.eligible([]string{"eng-ou", "sales-ou"})
			suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "eng-ou").
				Return(providers.OrganizationUnit{}, svcErr)
		}},
	}
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setup()

			result, err := suite.executor.Execute(suite.eligibleCtx(tc.properties, map[string]string{}))

			assert.Error(suite.T(), err)
			assert.Nil(suite.T(), result)
		})
	}
}

// Before sign-in the prompt strategy checks the pick in a bounded flow, and never lists eligible OUs.
func (suite *OUResolverExecutorTestSuite) TestExecute_Bounded_PromptBeforeSignInDoesNotReadEligibleOUs() {
	ctx := suite.directChildrenCtx("", ouIDKey, testChildOUID)
	delete(ctx.NodeProperties, common.NodePropertyOUDirectChildrenOnly)
	suite.withApplicationAccess(ctx)
	suite.mockOUService.On("IsEligibleOU", askedWithoutAUser(), testChildOUID).
		Return(true, (*tidcommon.ServiceError)(nil))
	suite.mockOUService.On("IsParent", mock.Anything, testParentOUID, testChildOUID).
		Return(true, (*tidcommon.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, result.Status)
	assert.Equal(suite.T(), testChildOUID, result.RuntimeData[ouIDKey])
	suite.mockOUService.AssertNotCalled(suite.T(), "GetEligibleOUs", mock.Anything)
}

// Without a boundary an OU already in runtime data does not stop a strategy from resolving.
func (suite *OUResolverExecutorTestSuite) TestExecute_Unbounded_StrategiesIgnoreADecidedOU() {
	callerCtx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("caller-user", "caller-ou", "token", []string{"system"}, nil))
	tests := []struct {
		name       string
		strategy   string
		ctx        context.Context
		setup      func()
		wantStatus providers.ExecutorStatus
		wantOUID   string
	}{
		{"Caller", ouResolveFromCaller, callerCtx, func() {}, providers.ExecComplete, "caller-ou"},
		{"Prompt", ouResolveFromPrompt, context.Background(), func() {
			suite.mockOUService.On("GetOrganizationUnitChildren", mock.Anything, testParentOUID, 1, 0, mock.Anything).
				Return(&providers.OrganizationUnitListResponse{TotalResults: 2}, (*tidcommon.ServiceError)(nil))
		}, providers.ExecUserInputRequired, ""},
		{"PromptAll", ouResolveFromPromptAll, context.Background(), func() {},
			providers.ExecUserInputRequired, ""},
	}
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setup()
			ctx := &providers.NodeContext{
				ExecutionID:    "flow-123",
				Context:        tc.ctx,
				FlowType:       providers.FlowTypeRegistration,
				NodeProperties: map[string]interface{}{common.NodePropertyOUResolveFrom: tc.strategy},
				RuntimeData:    map[string]string{ouIDKey: "decided-ou", defaultOUIDKey: testParentOUID},
				UserInputs:     map[string]string{},
			}

			result, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), tc.wantStatus, result.Status)
			assert.Equal(suite.T(), tc.wantOUID, result.RuntimeData[ouIDKey])
		})
	}
}

func TestOUResolverExecutorSuite(t *testing.T) {
	suite.Run(t, new(OUResolverExecutorTestSuite))
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Eligible_EqualHandlePathsAreOrderedAndMatchedByOUID() {
	for _, ou := range []providers.OrganizationUnit{
		{ID: "ou-b", Handle: "acme"},
		{ID: "ou-a", Handle: "acme"},
	} {
		suite.mockOUService.On("GetOrganizationUnit", mock.Anything, ou.ID).
			Return(ou, (*tidcommon.ServiceError)(nil)).Maybe()
	}
	suite.eligible([]string{"ou-b", "ou-a"})
	for i := 0; i < 20; i++ {
		result, err := suite.executor.Execute(suite.eligibleCtx(nil, map[string]string{}))
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), []string{"acme", "acme"}, result.Inputs[0].Options)

		result, err = suite.executor.Execute(suite.eligibleCtx(nil, map[string]string{ouIDKey: "acme"}))
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), "ou-a", result.RuntimeData[ouIDKey])
	}
}

// --- Application access of a picked OU ---

// pickMode is one way the prompt strategy takes a pick: the context carrying it, and the lookups that accept it.
type pickMode struct {
	name       string
	ctx        func(suite *OUResolverExecutorTestSuite) *providers.NodeContext
	accept     func(suite *OUResolverExecutorTestSuite)
	unknown    func(suite *OUResolverExecutorTestSuite)
	inputType  string
	rootOUData bool
}

func (suite *OUResolverExecutorTestSuite) pickModes() []pickMode {
	return []pickMode{
		{
			name: "TreePick",
			ctx: func(s *OUResolverExecutorTestSuite) *providers.NodeContext {
				ctx := s.directChildrenCtx("", ouIDKey, testChildOUID)
				delete(ctx.NodeProperties, common.NodePropertyOUDirectChildrenOnly)
				return ctx
			},
			accept: func(s *OUResolverExecutorTestSuite) {
				s.mockOUService.On("IsParent", mock.Anything, testParentOUID, testChildOUID).
					Return(true, (*tidcommon.ServiceError)(nil))
			},
			unknown: func(s *OUResolverExecutorTestSuite) {
				s.mockOUService.On("IsParent", mock.Anything, testParentOUID, testChildOUID).
					Return(false, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})
			},
			inputType: ouIDKey,
		},
		{
			name: "DirectChildByID",
			ctx: func(s *OUResolverExecutorTestSuite) *providers.NodeContext {
				return s.directChildrenCtx("", ouIDKey, testChildOUID)
			},
			accept: func(s *OUResolverExecutorTestSuite) {
				s.mockOUService.On("GetOrganizationUnit", mock.Anything, testChildOUID).Return(
					providers.OrganizationUnit{ID: testChildOUID, Parent: testParentOUID},
					(*tidcommon.ServiceError)(nil))
			},
			unknown: func(s *OUResolverExecutorTestSuite) {
				s.mockOUService.On("GetOrganizationUnit", mock.Anything, testChildOUID).Return(
					providers.OrganizationUnit{}, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})
			},
			inputType:  ouIDKey,
			rootOUData: true,
		},
		{
			name: "DirectChildByHandle",
			ctx: func(s *OUResolverExecutorTestSuite) *providers.NodeContext {
				return s.directChildrenCtx(ouHandleKey, ouHandleKey, "tenant-a")
			},
			accept:     func(s *OUResolverExecutorTestSuite) { s.childWithHandle("tenant-a") },
			unknown:    func(s *OUResolverExecutorTestSuite) { s.noChildWithHandle("tenant-a") },
			inputType:  ouHandleKey,
			rootOUData: true,
		},
	}
}

// withApplicationAccess makes ctx a bounded sign-in step of app-1.
func (suite *OUResolverExecutorTestSuite) withApplicationAccess(ctx *providers.NodeContext) {
	ctx.Context = suite.boundedCtx()
	ctx.FlowType = providers.FlowTypeAuthentication
}

// askedWithoutAUser matches a context carrying the sign-in boundary of app-1 with no signed-in user.
func askedWithoutAUser() any {
	return mock.MatchedBy(func(c context.Context) bool {
		b, ok := authnprovidercm.SignInBoundaryFrom(c)
		return ok && b == authnprovidercm.SignInBoundary{ApplicationID: "app-1"}
	})
}

// A pick made after sign-in by a user the OU provider cannot list OUs for is still checked against the application
// alone; completion checks the user.
func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Pick_AfterSignInIsCheckedAgainstTheApplication() {
	ctx := suite.directChildrenCtx("", ouIDKey, testChildOUID)
	ctx.Context = suite.signedInCtx()
	ctx.FlowType = providers.FlowTypeAuthentication
	suite.mockOUService.On("GetEligibleOUs", mock.Anything).Return([]string(nil),
		&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "OU-1020"})
	suite.pickModes()[1].accept(suite)
	suite.mockOUService.On("IsEligibleOU", askedWithoutAUser(), testChildOUID).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()

	result, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, result.Status)
	suite.Equal(testChildOUID, result.RuntimeData[ouIDKey])
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Pick_ApplicationNotUsableFromOU_AsksAgain() {
	for _, mode := range suite.pickModes() {
		suite.Run(mode.name, func() {
			suite.SetupTest()
			ctx := mode.ctx(suite)
			ctx.NodeProperties[common.NodePropertyOUParent] = testParentOUID
			suite.withApplicationAccess(ctx)
			mode.accept(suite)
			suite.mockOUService.On("IsEligibleOU", askedWithoutAUser(), testChildOUID).
				Return(false, (*tidcommon.ServiceError)(nil))

			result, err := suite.executor.Execute(ctx)

			suite.NoError(err)
			suite.Equal(providers.ExecUserInputRequired, result.Status)
			suite.Equal(&ErrApplicationNotInOU, result.Error)
			suite.NotContains(result.RuntimeData, ouIDKey)
			suite.Require().Len(result.Inputs, 1)
			suite.Equal(mode.inputType, result.Inputs[0].Identifier)
			if mode.rootOUData {
				suite.Equal(testParentOUID, result.AdditionalData[common.DataRootOUID])
			}
		})
	}
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Pick_ApplicationUsableFromOU_Completes() {
	for _, mode := range suite.pickModes() {
		suite.Run(mode.name, func() {
			suite.SetupTest()
			ctx := mode.ctx(suite)
			suite.withApplicationAccess(ctx)
			mode.accept(suite)
			suite.mockOUService.On("IsEligibleOU", askedWithoutAUser(), testChildOUID).
				Return(true, (*tidcommon.ServiceError)(nil))

			result, err := suite.executor.Execute(ctx)

			suite.NoError(err)
			suite.Equal(providers.ExecComplete, result.Status)
			suite.Nil(result.Error)
			suite.Equal(testChildOUID, result.RuntimeData[ouIDKey])
		})
	}
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Pick_UnknownPickKeepsItsError() {
	for _, mode := range suite.pickModes() {
		suite.Run(mode.name, func() {
			suite.SetupTest()
			ctx := mode.ctx(suite)
			suite.withApplicationAccess(ctx)
			mode.unknown(suite)

			result, err := suite.executor.Execute(ctx)

			suite.NoError(err)
			suite.Equal(providers.ExecUserInputRequired, result.Status)
			suite.NotEqual(&ErrApplicationNotInOU, result.Error)
			suite.NotNil(result.Error)
			suite.mockOUService.AssertNotCalled(suite.T(), "IsEligibleOU", mock.Anything, mock.Anything)
		})
	}
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Pick_UnboundedFlowSkipsTheCheck() {
	for _, mode := range suite.pickModes() {
		suite.Run(mode.name, func() {
			suite.SetupTest()
			ctx := mode.ctx(suite)
			mode.accept(suite)

			result, err := suite.executor.Execute(ctx)

			suite.NoError(err)
			suite.Equal(providers.ExecComplete, result.Status)
			suite.Equal(testChildOUID, result.RuntimeData[ouIDKey])
			suite.mockOUService.AssertNotCalled(suite.T(), "IsEligibleOU", mock.Anything, mock.Anything)
		})
	}
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Pick_BoundaryWithoutApplicationSkipsTheCheck() {
	ctx := suite.directChildrenCtx("", ouIDKey, testChildOUID)
	ctx.Context = authnprovidercm.WithSignInBoundary(context.Background(),
		authnprovidercm.SignInBoundary{OUID: testParentOUID})
	suite.pickModes()[1].accept(suite)

	result, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, result.Status)
	suite.mockOUService.AssertNotCalled(suite.T(), "IsEligibleOU", mock.Anything, mock.Anything)
}

func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Pick_AccessCheckErrorFailsTheStep() {
	for _, mode := range suite.pickModes() {
		suite.Run(mode.name, func() {
			suite.SetupTest()
			ctx := mode.ctx(suite)
			suite.withApplicationAccess(ctx)
			mode.accept(suite)
			suite.mockOUService.On("IsEligibleOU", askedWithoutAUser(), testChildOUID).
				Return(false, &tidcommon.InternalServerError)

			result, err := suite.executor.Execute(ctx)

			suite.Error(err)
			suite.Nil(result)
		})
	}
}

// A client error from the OU provider, such as an OU that does not exist, is a pick the application cannot use.
func (suite *OUResolverExecutorTestSuite) TestExecute_Prompt_Pick_AccessCheckClientErrorAsksAgain() {
	ctx := suite.directChildrenCtx("", ouIDKey, testChildOUID)
	suite.withApplicationAccess(ctx)
	suite.pickModes()[1].accept(suite)
	suite.mockOUService.On("IsEligibleOU", askedWithoutAUser(), testChildOUID).
		Return(false, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "OU-1003"})

	result, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecUserInputRequired, result.Status)
	suite.Equal(&ErrApplicationNotInOU, result.Error)
	suite.NotContains(result.RuntimeData, ouIDKey)
}
