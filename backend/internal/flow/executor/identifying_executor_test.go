// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type IdentifyingExecutorTestSuite struct {
	suite.Suite
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock
	mockFlowFactory    *coremock.FlowFactoryInterfaceMock
	mockAuthnProvider  *managermock.AuthnProviderManagerMock
	executor           *identifyingExecutor
}

func TestIdentifyingExecutorSuite(t *testing.T) {
	suite.Run(t, new(IdentifyingExecutorTestSuite))
}

func (suite *IdentifyingExecutorTestSuite) SetupTest() {
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())

	mockExec := createMockExecutor(suite.T(), ExecutorNameIdentifying, providers.ExecutorTypeUtility)
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameIdentifying, providers.ExecutorTypeUtility,
		[]providers.Input{}, []providers.Input{}, mock.Anything).Return(mockExec)

	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())
	suite.executor = newIdentifyingExecutor(ExecutorNameIdentifying, []providers.Input{},
		[]providers.Input{}, suite.mockFlowFactory, suite.mockEntityProvider, suite.mockAuthnProvider,
		newAccessGuard(nil))
}

func (suite *IdentifyingExecutorTestSuite) TestNewIdentifyingExecutor() {
	assert.NotNil(suite.T(), suite.executor)
	assert.NotNil(suite.T(), suite.executor.entityProvider)

	// Test default name
	exec := newIdentifyingExecutor(
		"",
		[]providers.Input{},
		[]providers.Input{},
		suite.mockFlowFactory,
		suite.mockEntityProvider,
		nil, newAccessGuard(nil),
	)
	assert.NotNil(suite.T(), exec)
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyEntity_Success() {
	filters := map[string]interface{}{"username": "testuser"}
	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}
	// Use package-level testUserID constant
	userID := testUserID
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, filters).Return(&userID, nil)

	result, err := suite.executor.IdentifyEntity(context.Background(), filters, execResp, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), testUserID, *result)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyEntity_UserNotFound() {
	filters := map[string]interface{}{"username": "nonexistent"}
	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, filters).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	result, err := suite.executor.IdentifyEntity(context.Background(), filters, execResp, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), ErrEntityNotFound.Error.DefaultValue, execResp.Error.Error.DefaultValue)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyEntity_ErrorNamesTheCategory() {
	cases := []struct {
		category                 entitytype.TypeCategory
		wantError, wantDescError string
	}{
		{entitytype.TypeCategoryUser, "The user could not be found",
			"The user could not be found in the system"},
		{entitytype.TypeCategoryAgent, "The agent could not be found",
			"The agent could not be found in the system"},
	}

	for _, tc := range cases {
		filters := map[string]interface{}{"username": "nonexistent"}
		execResp := &providers.ExecutorResponse{
			RuntimeData: make(map[string]string),
		}

		suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, filters).Return(nil,
			entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", "")).Once()

		result, err := suite.executor.IdentifyEntity(context.Background(), filters, execResp, tc.category)

		assert.NoError(suite.T(), err)
		assert.Nil(suite.T(), result)
		assert.Equal(suite.T(), tc.wantError, execResp.Error.Error.String())
		assert.Equal(suite.T(), tc.wantDescError, execResp.Error.ErrorDescription.String())
	}

	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyEntity_ServiceError() {
	filters := map[string]interface{}{"username": "testuser"}
	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, filters).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "", ""))

	result, err := suite.executor.IdentifyEntity(context.Background(), filters, execResp, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), ErrFailedToIdentifyEntity.Error.DefaultValue, execResp.Error.Error.DefaultValue)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyEntity_EmptyUserID() {
	filters := map[string]interface{}{"username": "testuser"}
	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}
	emptyID := ""

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, filters).Return(&emptyID, nil)

	result, err := suite.executor.IdentifyEntity(context.Background(), filters, execResp, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), ErrEntityNotFound.Error.DefaultValue, execResp.Error.Error.DefaultValue)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyEntity_FilterNonSearchableAttributes() {
	filters := map[string]interface{}{
		"username": "testuser",
		"password": "secret123",
		"code":     "auth-code",
		"otp":      "123456",
	}
	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}
	// Use package-level testUserID constant
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
		"username": "testuser",
	}).Return(func() *string {
		userID := testUserID
		return &userID
	}(), nil)

	result, err := suite.executor.IdentifyEntity(context.Background(), filters, execResp, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), testUserID, *result)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyEntity_WithEmail() {
	filters := map[string]interface{}{"email": "test@example.com"}
	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}
	emailUserID := "user-456"

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, filters).Return(&emailUserID, nil)

	result, err := suite.executor.IdentifyEntity(context.Background(), filters, execResp, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user-456", *result)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyEntity_Withmobile_number() {
	filters := map[string]interface{}{"mobile_number": "+1234567890"}
	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}
	mobileUserID := "user-789"

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, filters).Return(&mobileUserID, nil)

	result, err := suite.executor.IdentifyEntity(context.Background(), filters, execResp, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user-789", *result)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_Success_UserInputs() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"username": "testuser"},
	}
	// Use package-level testUserID constant
	// Configure mock base executor
	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "username", Type: "string", Required: true},
	})

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
		"username": "testuser",
	}).Return(func() *string {
		userID := testUserID
		return &userID
	}(), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), testUserID, resp.RuntimeData[userAttributeUserID])
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_Success_RuntimeData() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  make(map[string]string),
		RuntimeData: map[string]string{"username": "testuser"},
	}
	// Use package-level testUserID constant
	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "username", Type: "string", Required: true},
	})

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
		"username": "testuser",
	}).Return(func() *string {
		userID := testUserID
		return &userID
	}(), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), testUserID, resp.RuntimeData[userAttributeUserID])
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_UserInputRequired() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(false)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_Failure_IdentifyEntityError() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"username": "testuser"},
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "username", Type: "string", Required: true},
	})

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
		"username": "testuser",
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	// IdentifyEntity method in implementation swallows the error and returns nil, nil.
	// Then Execute checks for nil userID and returns UserNotFound.
	// So we should expect ErrEntityNotFound
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), ErrEntityNotFound.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_Failure_UserNotFound() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"username": "nonexistent"},
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "username", Type: "string", Required: true},
	})

	emptyID := ""
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
		"username": "nonexistent",
	}).Return(&emptyID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), ErrEntityNotFound.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

// TestExecute_Success_WithVariousAttributes tests successful user identification with different attributes.
func (suite *IdentifyingExecutorTestSuite) TestExecute_Success_WithVariousAttributes() {
	testCases := []struct {
		name       string
		attribute  string
		value      string
		expectedID string
	}{
		{"email", "email", "test@example.com", "user-email-456"},
		{"mobile_number", "mobile_number", "+1234567890", "user-mobile-789"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				UserInputs:  map[string]string{tc.attribute: tc.value},
			}

			mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
			mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
			mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
				{Identifier: tc.attribute, Type: "string", Required: true},
			})

			suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
				tc.attribute: tc.value,
			}).Return(&tc.expectedID, nil)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), resp)
			assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
			assert.Equal(suite.T(), tc.expectedID, resp.RuntimeData[userAttributeUserID])
			suite.mockEntityProvider.AssertExpectations(suite.T())
		})
	}
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_Success_WithMultipleAttributes() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs: map[string]string{
			"username": "testuser",
			"email":    "test@example.com",
		},
	}
	multiAttrUserID := "user-multi-123"

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "username", Type: "string", Required: true},
		{Identifier: "email", Type: "string", Required: true},
	})

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
		"username": "testuser",
		"email":    "test@example.com",
	}).Return(&multiAttrUserID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), multiAttrUserID, resp.RuntimeData[userAttributeUserID])
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

// TestExecute_Failure_UserNotFoundByAttribute tests failure handling when user is not found by different attributes.
func (suite *IdentifyingExecutorTestSuite) TestExecute_Failure_UserNotFoundByAttribute() {
	testCases := []struct {
		name      string
		attribute string
		value     string
	}{
		{"email", "email", "nonexistent@example.com"},
		{"mobile_number", "mobile_number", "+0000000000"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				UserInputs:  map[string]string{tc.attribute: tc.value},
			}

			mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
			mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
			mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
				{Identifier: tc.attribute, Type: "string", Required: true},
			})

			suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
				tc.attribute: tc.value,
			}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), resp)
			assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
			assert.Equal(suite.T(), ErrEntityNotFound.Error.DefaultValue, resp.Error.Error.DefaultValue)
			suite.mockEntityProvider.AssertExpectations(suite.T())
		})
	}
}

// TestExecute_Success_FromRuntimeData tests successful identification when attributes come from RuntimeData.
func (suite *IdentifyingExecutorTestSuite) TestExecute_Success_FromRuntimeData() {
	testCases := []struct {
		name       string
		attribute  string
		value      string
		expectedID string
	}{
		{"email", "email", "runtime@example.com", "user-runtime-email-456"},
		{"mobile_number", "mobile_number", "+9876543210", "user-runtime-mobile-789"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				UserInputs:  make(map[string]string),
				RuntimeData: map[string]string{tc.attribute: tc.value},
			}

			mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
			mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
			mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
				{Identifier: tc.attribute, Type: "string", Required: true},
			})

			suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
				tc.attribute: tc.value,
			}).Return(&tc.expectedID, nil)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), resp)
			assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
			assert.Equal(suite.T(), tc.expectedID, resp.RuntimeData[userAttributeUserID])
			suite.mockEntityProvider.AssertExpectations(suite.T())
		})
	}
}

// TestExecute_Failure_EmptyInput tests failure handling when input value is an empty string.
func (suite *IdentifyingExecutorTestSuite) TestExecute_Failure_EmptyInput() {
	testCases := []struct {
		name      string
		attribute string
	}{
		{"username", "username"},
		{"email", "email"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				UserInputs:  map[string]string{tc.attribute: ""},
			}

			mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
			mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
			mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
				{Identifier: tc.attribute, Type: "string", Required: true},
			})

			emptyID := ""
			suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
				tc.attribute: "",
			}).Return(&emptyID, nil)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), resp)
			assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
			assert.Equal(suite.T(), ErrEntityNotFound.Error.DefaultValue, resp.Error.Error.DefaultValue)
			suite.mockEntityProvider.AssertExpectations(suite.T())
		})
	}
}

// TestExecute_UserInputsPriorityOverRuntimeData tests that UserInputs takes priority over RuntimeData.
func (suite *IdentifyingExecutorTestSuite) TestExecute_UserInputsPriorityOverRuntimeData() {
	testCases := []struct {
		name           string
		attribute      string
		userInputValue string
		runtimeValue   string
		expectedID     string
	}{
		{"username", "username", "userinput-user", "runtime-user", "user-from-userinput-123"},
		{"email", "email", "userinput@example.com", "runtime@example.com", "user-from-email-userinput-456"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			// Both UserInputs and RuntimeData have the same key
			// UserInputs should take priority
			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				UserInputs:  map[string]string{tc.attribute: tc.userInputValue},
				RuntimeData: map[string]string{tc.attribute: tc.runtimeValue},
			}

			mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
			mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
			mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
				{Identifier: tc.attribute, Type: "string", Required: true},
			})

			// The mock should be called with the UserInputs value, not the RuntimeData value
			suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
				tc.attribute: tc.userInputValue,
			}).Return(&tc.expectedID, nil)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), resp)
			assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
			assert.Equal(suite.T(), tc.expectedID, resp.RuntimeData[userAttributeUserID])
			suite.mockEntityProvider.AssertExpectations(suite.T())
		})
	}
}

// --- Resolve mode tests ---

// Test user attribute JSON helpers to keep lines under 120 chars.
var (
	attrsAlexJohnson = json.RawMessage(
		`{"given_name":"Alex","family_name":"Johnson"}`)
	attrsAlexSmith = json.RawMessage(
		`{"given_name":"Alex","family_name":"Smith"}`)
	attrsAlex = json.RawMessage(`{"given_name":"Alex"}`)
)

func (suite *IdentifyingExecutorTestSuite) TestExecuteResolve_UniqueUser() {
	ctx := &providers.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeResolve,
		UserInputs:   map[string]string{"given_name": "Alex"},
		RuntimeData:  make(map[string]string),
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
	})

	suite.mockEntityProvider.On("SearchEntities", mock.Anything, map[string]interface{}{
		"given_name": "Alex",
	}).Return([]*providers.Entity{
		{ID: "user-1", Type: "Person", Attributes: attrsAlex},
	}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "user-1", resp.RuntimeData[userAttributeUserID])
}

func (suite *IdentifyingExecutorTestSuite) TestExecuteResolve_AmbiguousUser() {
	ctx := &providers.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeResolve,
		UserInputs:   map[string]string{"given_name": "Alex"},
		RuntimeData:  make(map[string]string),
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
	})

	suite.mockEntityProvider.On("SearchEntities", mock.Anything, map[string]interface{}{
		"given_name": "Alex",
	}).Return([]*providers.Entity{
		{ID: "user-1", Type: "Person", Attributes: attrsAlexJohnson},
		{ID: "user-2", Type: "Engineer", Attributes: attrsAlexSmith},
	}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyCandidateUsers])
	assert.NotNil(suite.T(), resp.ForwardedData)
}

func (suite *IdentifyingExecutorTestSuite) TestExecuteResolve_FilteredToOne() {
	candidates := []*providers.Entity{
		{ID: "user-1", Type: "Person", Attributes: attrsAlexJohnson},
		{ID: "user-2", Type: "Person", Attributes: attrsAlexSmith},
	}
	candidatesJSON, _ := json.Marshal(candidates)

	ctx := &providers.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeResolve,
		UserInputs:   map[string]string{"given_name": "Alex", "family_name": "Smith"},
		RuntimeData: map[string]string{
			common.RuntimeKeyCandidateUsers: string(candidatesJSON),
		},
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
		{Identifier: "family_name", Type: "TEXT_INPUT", Required: true},
	})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "user-2", resp.RuntimeData[userAttributeUserID])
}

func (suite *IdentifyingExecutorTestSuite) TestExecuteResolve_StillAmbiguous() {
	candidates := []*providers.Entity{
		{ID: "user-1", Type: "Person", Attributes: attrsAlexSmith},
		{ID: "user-2", Type: "Engineer", Attributes: attrsAlexSmith},
	}
	candidatesJSON, _ := json.Marshal(candidates)

	ctx := &providers.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeResolve,
		UserInputs:   map[string]string{"given_name": "Alex", "family_name": "Smith"},
		RuntimeData: map[string]string{
			common.RuntimeKeyCandidateUsers: string(candidatesJSON),
		},
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
		{Identifier: "family_name", Type: "TEXT_INPUT", Required: true},
	})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyCandidateUsers])
}

func (suite *IdentifyingExecutorTestSuite) TestExecuteResolve_FilteredToNone() {
	candidates := []*providers.Entity{
		{ID: "user-1", Type: "Person", Attributes: attrsAlexJohnson},
	}
	candidatesJSON, _ := json.Marshal(candidates)

	ctx := &providers.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeResolve,
		UserInputs:   map[string]string{"given_name": "Alex", "family_name": "Williams"},
		RuntimeData: map[string]string{
			common.RuntimeKeyCandidateUsers: string(candidatesJSON),
		},
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
		{Identifier: "family_name", Type: "TEXT_INPUT", Required: true},
	})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), ErrEntityNotFound.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

// --- check_state mode tests ---

func (suite *IdentifyingExecutorTestSuite) TestExecuteCheckState_NoMatch() {
	ctx := &providers.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeCheckState,
		UserInputs:   map[string]string{"given_name": "Alex"},
		RuntimeData:  make(map[string]string),
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
	})

	suite.mockEntityProvider.On("SearchEntities", mock.Anything, map[string]interface{}{
		"given_name": "Alex",
	}).Return([]*providers.Entity{}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), entityStateNotExists, resp.RuntimeData[common.RuntimeKeyEntityState])
}

func (suite *IdentifyingExecutorTestSuite) TestExecuteCheckState_SingleMatch() {
	ctx := &providers.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeCheckState,
		UserInputs:   map[string]string{"given_name": "Alex"},
		RuntimeData:  make(map[string]string),
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
	})

	suite.mockEntityProvider.On("SearchEntities", mock.Anything, map[string]interface{}{
		"given_name": "Alex",
	}).Return([]*providers.Entity{
		{ID: "user-1", Type: "Person", Attributes: attrsAlex},
	}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), entityStateExists, resp.RuntimeData[common.RuntimeKeyEntityState])
}

func (suite *IdentifyingExecutorTestSuite) TestExecuteCheckState_MultipleMatches() {
	ctx := &providers.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeCheckState,
		UserInputs:   map[string]string{"given_name": "Alex"},
		RuntimeData:  make(map[string]string),
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
	})

	suite.mockEntityProvider.On("SearchEntities", mock.Anything, map[string]interface{}{
		"given_name": "Alex",
	}).Return([]*providers.Entity{
		{ID: "user-1", Type: "Person", Attributes: attrsAlexJohnson},
		{ID: "user-2", Type: "Engineer", Attributes: attrsAlexSmith},
	}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), entityStateAmbiguous, resp.RuntimeData[common.RuntimeKeyEntityState])
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_IdentifyMode_AmbiguousUser() {
	ctx := &providers.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeIdentify,
		UserInputs:   map[string]string{"given_name": "Alex"},
		RuntimeData:  make(map[string]string),
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
	})

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
		"given_name": "Alex",
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeAmbiguousEntity, "Ambiguous user", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrAmbiguousEntityIdentity.Error.DefaultValue, resp.Error.Error.DefaultValue)
	assert.Empty(suite.T(), resp.Inputs, "Inputs must not be populated for ambiguous user in identify mode")
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_IdentifyMode_UserNotFound_PopulatesInputsForRetry() {
	inputs := []providers.Input{{Identifier: "username", Type: "TEXT_INPUT", Required: true}}
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"username": "nonexistent"},
		RuntimeData: make(map[string]string),
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return(inputs)

	// IdentifyEntity sets ExecFailure + userNotFound; executeIdentify must promote to UserInputRequired
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
		"username": "nonexistent",
	}).Return(nil, entityprovider.NewEntityProviderError(
		entityprovider.ErrorCodeEntityNotFound, "not found", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), ErrEntityNotFound.Error.DefaultValue, resp.Error.Error.DefaultValue)
	assert.NotEmpty(suite.T(), resp.Inputs, "Inputs must be populated for retry when user is not found")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_IdentifyMode_SystemError() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"username": "testuser"},
		RuntimeData: make(map[string]string),
	}

	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "username", Type: "TEXT_INPUT", Required: true},
	})

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
		"username": "testuser",
	}).Return(nil, entityprovider.NewEntityProviderError(
		entityprovider.ErrorCodeSystemError, "System error", "db unavailable"))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrFailedToIdentifyEntity.Error.DefaultValue, resp.Error.Error.DefaultValue)
	assert.Empty(suite.T(), resp.Inputs, "Inputs must not be populated for non-recoverable errors")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func TestFilterUsersByAttributes(t *testing.T) {
	users := []*providers.Entity{
		{ID: "u1", Type: "Person", Attributes: attrsAlexJohnson},
		{ID: "u2", Type: "Person", Attributes: attrsAlexSmith},
		{ID: "u3", Type: "Engineer", Attributes: attrsAlexSmith},
	}

	result := filterUsersByAttributes(users, map[string]interface{}{"family_name": "Smith"})
	assert.Len(t, result, 2)

	result = filterUsersByAttributes(users, map[string]interface{}{"userType": "Engineer"})
	assert.Len(t, result, 1)
	assert.Equal(t, "u3", result[0].ID)

	result = filterUsersByAttributes(users, map[string]interface{}{
		"family_name": "Smith",
		"userType":    "Person",
	})
	assert.Len(t, result, 1)
	assert.Equal(t, "u2", result[0].ID)

	result = filterUsersByAttributes(users, map[string]interface{}{"family_name": "Doe"})
	assert.Empty(t, result)
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_RetryableIdentificationErrors() {
	tests := []struct {
		name           string
		attribute      string
		value          string
		entityError    *entityprovider.EntityProviderError
		emptyID        bool
		expectedReason string
		message        string
	}{
		{
			name:           "User not found",
			attribute:      "username",
			value:          "nonexistent",
			entityError:    entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""),
			expectedReason: ErrEntityNotFound.Error.DefaultValue,
			message:        "Should return inputs for retry when user is not found",
		},
		{
			name:           "Empty user ID returned",
			attribute:      "username",
			value:          "testuser",
			emptyID:        true,
			expectedReason: ErrEntityNotFound.Error.DefaultValue,
			message:        "Should return inputs for retry when empty user ID is returned",
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			suite.SetupTest()

			inputs := []providers.Input{{Identifier: tt.attribute, Type: "string", Required: true}}
			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				UserInputs:  map[string]string{tt.attribute: tt.value},
			}

			mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
			mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
			mockBase.On("GetRequiredInputs", mock.Anything).Return(inputs)

			if tt.emptyID {
				emptyID := ""
				suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
					tt.attribute: tt.value,
				}).Return(&emptyID, nil)
			} else {
				suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{
					tt.attribute: tt.value,
				}).Return(nil, tt.entityError)
			}

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, providers.ExecUserInputRequired, resp.Status)
			assert.Equal(t, tt.expectedReason, resp.Error.Error.DefaultValue, tt.message)
			assert.NotEmpty(t, resp.Inputs, "Inputs should be re-populated for retry")
			suite.mockEntityProvider.AssertExpectations(t)
		})
	}
}

func TestExtractDisambiguationOptions(t *testing.T) {
	candidates := []*providers.Entity{
		{ID: "u1", Type: "Person", Attributes: attrsAlexJohnson},
		{ID: "u2", Type: "Person", Attributes: attrsAlexSmith},
		{ID: "u3", Type: "Engineer", Attributes: attrsAlexSmith},
	}

	inputs := extractDisambiguationOptions(candidates)

	inputsByKey := make(map[string]providers.Input)
	for _, input := range inputs {
		inputsByKey[input.Identifier] = input
	}

	assert.Contains(t, inputsByKey, "userType")
	assert.ElementsMatch(t, []string{"Person", "Engineer"}, inputsByKey["userType"].Options)
	assert.Equal(t, providers.InputTypeSelect, inputsByKey["userType"].Type)

	assert.Contains(t, inputsByKey, "family_name")
	assert.ElementsMatch(t, []string{"Johnson", "Smith"}, inputsByKey["family_name"].Options)
	assert.Equal(t, providers.InputTypeSelect, inputsByKey["family_name"].Type)

	assert.NotContains(t, inputsByKey, "given_name")
}

// --- Entity ID (userID) path tests ---

func (suite *IdentifyingExecutorTestSuite) TestIdentifyEntity_WithEntityID_Success() {
	entityID := testUserID
	filters := map[string]interface{}{userAttributeUserID: entityID}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	suite.mockEntityProvider.On("GetEntityProfile", mock.Anything, entityID).
		Return(&providers.Entity{ID: entityID}, nil)

	result, err := suite.executor.IdentifyEntity(context.Background(), filters, execResp, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), entityID, *result)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyEntity_WithEntityID_NotFound() {
	filters := map[string]interface{}{userAttributeUserID: "missing-id"}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	suite.mockEntityProvider.On("GetEntityProfile", mock.Anything, "missing-id").
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	result, err := suite.executor.IdentifyEntity(context.Background(), filters, execResp, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), ErrEntityNotFound.Error.DefaultValue, execResp.Error.Error.DefaultValue)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyEntity_WithEntityID_SystemError() {
	filters := map[string]interface{}{userAttributeUserID: testUserID}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	suite.mockEntityProvider.On("GetEntityProfile", mock.Anything, testUserID).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "", ""))

	result, err := suite.executor.IdentifyEntity(context.Background(), filters, execResp, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), ErrFailedToIdentifyEntity.Error.DefaultValue, execResp.Error.Error.DefaultValue)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyEntity_WithEntityID_EmptyEntityID_FallsThrough() {
	filters := map[string]interface{}{userAttributeUserID: ""}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	emptyID := ""
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{userAttributeUserID: ""}).
		Return(&emptyID, nil)

	result, err := suite.executor.IdentifyEntity(context.Background(), filters, execResp, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
}

// linkHolderCtx is a resolve-mode context for a federated identity several users are linked to.
func linkHolderCtx(runtime, userInputs map[string]string) *providers.NodeContext {
	runtimeData := map[string]string{
		common.RuntimeKeyEntityState:      entityStateAmbiguous,
		common.RuntimeKeyExternalIdentity: externalIdentityEntry("idp-a", "sub-1", nil),
	}
	for key, value := range runtime {
		runtimeData[key] = value
	}
	return &providers.NodeContext{
		ExecutionID:  "flow-1",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeResolve,
		AuthUser:     newAuthenticatedAuthUser(),
		RuntimeData:  runtimeData,
		UserInputs:   userInputs,
	}
}

// linkHolders are two users holding the same link in different OUs.
func linkHolders() []*providers.Entity {
	return []*providers.Entity{
		{ID: "holder-sales", OUHandle: "sales", Type: "person"},
		{ID: "holder-support", OUHandle: "support", Type: "person"},
	}
}

func storedLinkHolders(holders []*providers.Entity) map[string]string {
	encoded, _ := json.Marshal(holders)
	return map[string]string{common.RuntimeKeyCandidateUsers: string(encoded)}
}

// expectBind expects the picked holder to be authenticated onto the AuthUser.
func (suite *IdentifyingExecutorTestSuite) expectBind(entityID string, svcErr *tidcommon.ServiceError) {
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
		map[string]interface{}{authnprovidercm.CredentialTypeProvisionedEntityID: entityID},
		mock.Anything, mock.Anything, mock.Anything).
		Return(newAuthenticatedAuthUser(), providers.AuthenticatedClaims{}, svcErr).Once()
}

// The holders come from the link, with no typed inputs asked for, and the user is offered what tells
// them apart.
func (suite *IdentifyingExecutorTestSuite) TestResolveLinkHolders_FirstPassOffersHolders() {
	suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").Return(linkHolders(), nil).Once()

	resp, err := suite.executor.Execute(linkHolderCtx(nil, nil))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecUserInputRequired, resp.Status)
	suite.Empty(resp.RuntimeData[common.RuntimeKeyCandidateUsers])
	options, _ := resp.ForwardedData[common.ForwardedDataKeyInputs].([]providers.Input)
	suite.Require().Len(options, 1)
	suite.Equal("ouHandle", options[0].Identifier)
	suite.ElementsMatch([]string{"sales", "support"}, options[0].Options)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "SearchEntities", mock.Anything, mock.Anything)
}

// A pick narrows the holders to one, which is bound to the AuthUser. Inputs other than the offered
// options do not filter the holders.
func (suite *IdentifyingExecutorTestSuite) TestResolveLinkHolders_PickBindsHolder() {
	suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").Return(linkHolders(), nil).Once()
	suite.expectBind("holder-support", nil)
	ctx := linkHolderCtx(nil, map[string]string{"ouHandle": "support", "code": "auth-code"})

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal("holder-support", resp.RuntimeData[userAttributeUserID])
	suite.Equal(entityStateExists, resp.RuntimeData[common.RuntimeKeyEntityState])
	suite.True(resp.AuthUser.IsAuthenticated())
}

// Candidates an attribute-based resolve left behind are not link holders. They are never offered or
// bound, so a pick of one is offered the holders again.
func (suite *IdentifyingExecutorTestSuite) TestResolveLinkHolders_IgnoresStoredCandidates() {
	suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").Return(linkHolders(), nil).Once()
	stale := storedLinkHolders([]*providers.Entity{
		{ID: "other-user-hr", OUHandle: "hr", Type: "person"},
		{ID: "other-user-it", OUHandle: "it", Type: "person"},
	})
	ctx := linkHolderCtx(stale, map[string]string{"ouHandle": "hr"})

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecUserInputRequired, resp.Status)
	options, _ := resp.ForwardedData[common.ForwardedDataKeyInputs].([]providers.Input)
	suite.Require().Len(options, 1)
	suite.ElementsMatch([]string{"sales", "support"}, options[0].Options)
	suite.mockAuthnProvider.AssertNotCalled(suite.T(), "AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A pick that matches no holder is offered the holders again rather than read as no user.
func (suite *IdentifyingExecutorTestSuite) TestResolveLinkHolders_UnofferedPickIsOfferedAgain() {
	suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").Return(linkHolders(), nil).Once()
	ctx := linkHolderCtx(nil, map[string]string{"ouHandle": "elsewhere"})

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecUserInputRequired, resp.Status)
	// The error makes the prompt drop the stale pick, so it is asked for again rather than resubmitted.
	suite.Require().NotNil(resp.Error)
	suite.Equal(ErrEntityNotFound.Code, resp.Error.Code)
	options, _ := resp.ForwardedData[common.ForwardedDataKeyInputs].([]providers.Input)
	suite.Require().Len(options, 1)
	suite.ElementsMatch([]string{"sales", "support"}, options[0].Options)
}

// Holders nothing tells apart cannot be picked between, so the flow fails.
func (suite *IdentifyingExecutorTestSuite) TestResolveLinkHolders_IndistinguishableHoldersFail() {
	suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").Return([]*providers.Entity{
		{ID: "holder-1", OUHandle: "sales", Type: "person"},
		{ID: "holder-2", OUHandle: "sales", Type: "person"},
	}, nil).Once()

	resp, err := suite.executor.Execute(linkHolderCtx(nil, nil))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrFailedToIdentifyEntity.Code, resp.Error.Code)
}

// Holders in the same OU are still picked between on what else tells them apart.
func (suite *IdentifyingExecutorTestSuite) TestResolveLinkHolders_SameOUHoldersPickedOnType() {
	suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").Return([]*providers.Entity{
		{ID: "holder-1", OUHandle: "sales", Type: "person"},
		{ID: "holder-2", OUHandle: "sales", Type: "partner"},
	}, nil).Once()

	resp, err := suite.executor.Execute(linkHolderCtx(nil, nil))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecUserInputRequired, resp.Status)
	options, _ := resp.ForwardedData[common.ForwardedDataKeyInputs].([]providers.Input)
	suite.Require().Len(options, 1)
	suite.Equal("userType", options[0].Identifier)
}

func (suite *IdentifyingExecutorTestSuite) TestResolveLinkHolders_SingleHolderBindsWithoutPrompt() {
	suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").
		Return([]*providers.Entity{{ID: "holder-1", OUHandle: "sales"}}, nil).Once()
	suite.expectBind("holder-1", nil)

	resp, err := suite.executor.Execute(linkHolderCtx(nil, nil))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal("holder-1", resp.RuntimeData[userAttributeUserID])
}

func (suite *IdentifyingExecutorTestSuite) TestResolveLinkHolders_NoHolderFails() {
	suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").
		Return([]*providers.Entity{}, nil).Once()

	resp, err := suite.executor.Execute(linkHolderCtx(nil, nil))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrEntityNotFound.Code, resp.Error.Code)
}

func (suite *IdentifyingExecutorTestSuite) TestResolveLinkHolders_HolderLookupFails() {
	suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "", "")).Once()

	resp, err := suite.executor.Execute(linkHolderCtx(nil, nil))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrFailedToIdentifyEntity.Code, resp.Error.Code)
}

func (suite *IdentifyingExecutorTestSuite) TestResolveLinkHolders_BindFailures() {
	suite.Run("client error fails the node", func() {
		suite.SetupTest()
		suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").
			Return(linkHolders(), nil).Once()
		suite.expectBind("holder-sales", &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "X"})

		resp, err := suite.executor.Execute(linkHolderCtx(nil, map[string]string{"ouHandle": "sales"}))

		suite.Require().NoError(err)
		suite.Equal(providers.ExecFailure, resp.Status)
	})
	suite.Run("unauthenticated result fails the node", func() {
		suite.SetupTest()
		suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").
			Return(linkHolders(), nil).Once()
		suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything).
			Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, (*tidcommon.ServiceError)(nil)).Once()

		resp, err := suite.executor.Execute(linkHolderCtx(nil, map[string]string{"ouHandle": "sales"}))

		suite.Require().NoError(err)
		suite.Equal(providers.ExecFailure, resp.Status)
		suite.Empty(resp.RuntimeData[userAttributeUserID])
	})
	suite.Run("a refusal fails the node with the hold it states", func() {
		cases := []struct {
			code string
			want string
		}{
			{code: authnprovidermgr.ErrorAccountLocked.Code, want: ErrAccountLocked.Code},
			{code: authnprovidermgr.ErrorAccountSuspended.Code, want: ErrAccountSuspended.Code},
			{code: authnprovidermgr.ErrorAuthenticationFailed.Code, want: ErrFailedToIdentifyEntity.Code},
		}
		for _, tc := range cases {
			suite.SetupTest()
			suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").
				Return(linkHolders(), nil).Once()
			suite.expectBind("holder-sales", &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: tc.code})

			resp, err := suite.executor.Execute(linkHolderCtx(nil, map[string]string{"ouHandle": "sales"}))

			suite.Require().NoError(err)
			suite.Equal(providers.ExecFailure, resp.Status)
			suite.Equal(tc.want, resp.Error.Code)
			suite.Empty(resp.RuntimeData[userAttributeUserID])
		}
	})
	suite.Run("server error is returned", func() {
		suite.SetupTest()
		suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").
			Return(linkHolders(), nil).Once()
		suite.expectBind("holder-sales", &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "X"})

		_, err := suite.executor.Execute(linkHolderCtx(nil, map[string]string{"ouHandle": "sales"}))

		suite.Error(err)
	})
}

// After a federated sign-in the link decides the user. Without several holders there is nothing to
// pick, and registration settles the holders at provisioning, so the node passes through rather than
// resolving on the identity's claims. The unstubbed providers assert nothing is looked up.
func (suite *IdentifyingExecutorTestSuite) TestResolveFederated_PassesThrough() {
	for name, ctx := range map[string]*providers.NodeContext{
		"one holder": linkHolderCtx(map[string]string{common.RuntimeKeyEntityState: entityStateExists}, nil),
		"no holder":  linkHolderCtx(map[string]string{common.RuntimeKeyEntityState: entityStateNotExists}, nil),
		"registration": func() *providers.NodeContext {
			ctx := linkHolderCtx(nil, nil)
			ctx.FlowType = providers.FlowTypeRegistration
			return ctx
		}(),
	} {
		suite.Run(name, func() {
			suite.SetupTest()

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			suite.Equal(providers.ExecComplete, resp.Status)
			suite.Empty(resp.RuntimeData[userAttributeUserID])
		})
	}
}

// Without a federated identity, resolve mode keeps resolving on its inputs whatever the entity state.
func (suite *IdentifyingExecutorTestSuite) TestResolve_WithoutFederatedIdentityResolvesOnInputs() {
	ctx := linkHolderCtx(nil, nil)
	delete(ctx.RuntimeData, common.RuntimeKeyExternalIdentity)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).On("HasRequiredInputs", mock.Anything, mock.Anything).
		Return(false).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecUserInputRequired, resp.Status)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "GetLinkedEntities", mock.Anything, mock.Anything)
}

// After a federated sign-in, check_state counts the link's holders. It asks for no inputs and never
// searches on attributes or claims, so a claim matching some other user cannot change the state.
func (suite *IdentifyingExecutorTestSuite) TestCheckStateFederated_CountsLinkHolders() {
	for name, tc := range map[string]struct {
		holders   []*providers.Entity
		wantState string
	}{
		"no holder":       {holders: []*providers.Entity{}, wantState: entityStateNotExists},
		"one holder":      {holders: linkHolders()[:1], wantState: entityStateExists},
		"several holders": {holders: linkHolders(), wantState: entityStateAmbiguous},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").
				Return(tc.holders, nil).Once()
			ctx := linkHolderCtx(nil, nil)
			ctx.ExecutorMode = ExecutorModeCheckState
			ctx.NodeInputs = []providers.Input{{Identifier: "sub", Required: true}}

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			suite.Equal(providers.ExecComplete, resp.Status)
			suite.Empty(resp.Inputs)
			suite.Equal(tc.wantState, resp.RuntimeData[common.RuntimeKeyEntityState])
			suite.mockEntityProvider.AssertNotCalled(suite.T(), "SearchEntities", mock.Anything, mock.Anything)
		})
	}
}

// After a federated sign-in, identify takes the user from the link. It asks for no inputs and never
// searches on attributes or claims, so a claim matching some other user cannot name that user.
func (suite *IdentifyingExecutorTestSuite) TestIdentifyFederated_UsesLinkHolder() {
	for name, tc := range map[string]struct {
		holders    []*providers.Entity
		wantStatus providers.ExecutorStatus
		wantUserID string
		wantError  string
	}{
		"one holder": {holders: linkHolders()[:1], wantStatus: providers.ExecComplete,
			wantUserID: "holder-sales"},
		"no holder": {holders: []*providers.Entity{}, wantStatus: providers.ExecFailure,
			wantError: ErrEntityNotFound.Code},
		"several holders": {holders: linkHolders(), wantStatus: providers.ExecFailure,
			wantError: ErrAmbiguousEntityIdentity.Code},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").
				Return(tc.holders, nil).Once()
			ctx := linkHolderCtx(nil, nil)
			ctx.ExecutorMode = ExecutorModeIdentify
			ctx.RuntimeData[common.RuntimeKeyExternalIdentity] = externalIdentityEntry("idp-a", "sub-1",
				map[string]interface{}{"email": "other@example.com"})
			ctx.NodeInputs = []providers.Input{{Identifier: "email", Required: true}}

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			suite.Equal(tc.wantStatus, resp.Status)
			suite.Empty(resp.Inputs)
			suite.Equal(tc.wantUserID, resp.RuntimeData[userAttributeUserID])
			if tc.wantError != "" {
				suite.Equal(tc.wantError, resp.Error.Code)
			}
			suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity", mock.Anything, mock.Anything)
			suite.mockEntityProvider.AssertNotCalled(suite.T(), "SearchEntities", mock.Anything, mock.Anything)
		})
	}
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyFederated_HolderLookupFails() {
	suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "", "")).Once()
	ctx := linkHolderCtx(nil, nil)
	ctx.ExecutorMode = ExecutorModeIdentify

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrFailedToIdentifyEntity.Code, resp.Error.Code)
}

// A provider that cannot list the link's holders keeps the state the federated executor recorded.
func (suite *IdentifyingExecutorTestSuite) TestCheckStateFederated_KeepsStateWithoutHolderListing() {
	suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeNotImplemented, "", "")).Once()
	ctx := linkHolderCtx(nil, nil)
	ctx.ExecutorMode = ExecutorModeCheckState

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.NotContains(resp.RuntimeData, common.RuntimeKeyEntityState)
}

func (suite *IdentifyingExecutorTestSuite) TestCheckStateFederated_HolderLookupFails() {
	suite.mockEntityProvider.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "", "")).Once()
	ctx := linkHolderCtx(nil, nil)
	ctx.ExecutorMode = ExecutorModeCheckState

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrFailedToIdentifyEntity.Code, resp.Error.Code)
}

// identifyRecoveryTarget identifies testuser in a recovery flow against the given decision or fault,
// by login hint when hintAttr is set.
func (suite *IdentifyingExecutorTestSuite) identifyRecoveryTarget(decision providers.AccessDecision,
	fault *tidcommon.ServiceError, hintAttr string) (*providers.ExecutorResponse, error) {
	suite.executor = newIdentifyingExecutor(ExecutorNameIdentifying, []providers.Input{},
		[]providers.Input{}, suite.mockFlowFactory, suite.mockEntityProvider, nil,
		newAccessGuard(expectAccess(suite.T(), testUserID, providers.AccessPlaneRecovery, "", decision, fault)))
	userID := testUserID
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything, map[string]interface{}{"username": "testuser"}).
		Return(&userID, nil)

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRecovery,
		UserInputs:  map[string]string{"username": "testuser"},
	}
	if hintAttr != "" {
		ctx.NodeProperties = map[string]interface{}{propertyKeyLoginHintAttribute: hintAttr}
		ctx.UserInputs = map[string]string{userInputLoginHint: "testuser"}
		return suite.executor.Execute(ctx)
	}
	mockBase := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "username", Type: "string", Required: true},
	})
	return suite.executor.Execute(ctx)
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_HeldRecoveryTargetReadsAsUnknown() {
	resp, err := suite.identifyRecoveryTarget(providers.AccessDecision{}, nil, "")

	suite.Require().NoError(err)
	suite.Equal(providers.ExecUserInputRequired, resp.Status)
	suite.Equal(ErrEntityNotFound.Code, resp.Error.Code)
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_HeldRecoveryTargetStatesTheHold() {
	resp, err := suite.identifyRecoveryTarget(providers.AccessDecision{
		Reason: providers.AccessRefusalReasonSuspended,
	}, nil, "")

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrAccountSuspended.Code, resp.Error.Code)
}

// With a login hint there is no one to re-prompt.
func (suite *IdentifyingExecutorTestSuite) TestExecute_HeldRecoveryTargetByLoginHintFails() {
	resp, err := suite.identifyRecoveryTarget(providers.AccessDecision{}, nil, "username")

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrEntityNotFound.Code, resp.Error.Code)
	suite.Empty(resp.Inputs)
	suite.Empty(resp.RuntimeData[userAttributeUserID])
}

// Answered as a failed account lookup.
func (suite *IdentifyingExecutorTestSuite) TestExecute_RecoveryTargetGovernanceFaultFails() {
	resp, err := suite.identifyRecoveryTarget(providers.AccessDecision{}, &tidcommon.InternalServerError, "")

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrFailedToIdentifyEntity.Code, resp.Error.Code)
	suite.Empty(resp.RuntimeData[userAttributeUserID])
}
