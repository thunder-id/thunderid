// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/tests/mocks/agentmgtprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
	"github.com/thunder-id/thunderid/tests/mocks/usermgtprovidermock"
)

const (
	testUserType            = "INTERNAL"
	testAgentType           = "SERVICE_AGENT"
	testNewAgentID          = "agent-new"
	testNewUserID           = "user-new"
	methodGetRequiredInputs = "GetRequiredInputs"
	attributeEmail          = "email"
	attributePassword       = "password"
	attributePin            = "pin"
)

type ProvisioningExecutorTestSuite struct {
	suite.Suite
	mockGroupService          *groupmock.GroupServiceInterfaceMock
	mockRoleService           *rolemock.RoleServiceInterfaceMock
	mockRoleAssignmentService *rolemock.RoleAssignmentServiceInterfaceMock
	mockFlowFactory           *coremock.FlowFactoryInterfaceMock
	mockEntityProvider        *entityprovidermock.EntityProviderInterfaceMock
	mockUserMgtProvider       *usermgtprovidermock.UserMgtProviderMock
	mockAgentMgtProvider      *agentmgtprovidermock.AgentMgtProviderMock
	mockEntityTypeService     *entitytypemock.EntityTypeServiceInterfaceMock
	mockAuthnProvider         *managermock.AuthnProviderManagerMock
	executor                  *provisioningExecutor
}

func TestProvisioningExecutorSuite(t *testing.T) {
	suite.Run(t, new(ProvisioningExecutorTestSuite))
}

func (suite *ProvisioningExecutorTestSuite) SetupTest() {
	suite.mockGroupService = groupmock.NewGroupServiceInterfaceMock(suite.T())
	suite.mockRoleService = rolemock.NewRoleServiceInterfaceMock(suite.T())
	suite.mockRoleAssignmentService = rolemock.NewRoleAssignmentServiceInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockUserMgtProvider = usermgtprovidermock.NewUserMgtProviderMock(suite.T())
	suite.mockAgentMgtProvider = agentmgtprovidermock.NewAgentMgtProviderMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(newAuthenticatedAuthUser(), providers.AuthenticatedClaims{},
			(*tidcommon.ServiceError)(nil)).Maybe()

	// Mock the embedded identifying executor first
	identifyingMock := suite.createMockIdentifyingExecutor()
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameIdentifying, providers.ExecutorTypeUtility,
		mock.Anything, mock.Anything, mock.Anything).Return(identifyingMock).Maybe()

	mockExec := suite.createMockProvisioningExecutor()
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameProvisioning, providers.ExecutorTypeRegistration,
		[]providers.Input{}, []providers.Input{}, mock.Anything).Return(mockExec)

	suite.executor = newProvisioningExecutor(suite.mockFlowFactory,
		suite.mockGroupService, suite.mockRoleService, suite.mockRoleAssignmentService, suite.mockEntityProvider,
		suite.mockUserMgtProvider, suite.mockAgentMgtProvider, suite.mockEntityTypeService,
		suite.mockAuthnProvider)
}

// expectSchemaForProvisioning sets up the schema service mocks for Execute tests.
// The (true,true) mock covers both HasRequiredInputs and getAttributesForProvisioning.
// This version does NOT include credentials - use expectSchemaWithCredentials if needed.
func (suite *ProvisioningExecutorTestSuite) expectSchemaForProvisioning() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: false},
			{Attribute: attributeEmail, Required: false},
			{Attribute: "sub", Required: false},
		}, nil).Maybe()
}

// expectSchemaForAgentProvisioning sets up the agent type schema mock for agent-mode Execute tests.
func (suite *ProvisioningExecutorTestSuite) expectSchemaForAgentProvisioning() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, entitytype.TypeCategoryAgent, testAgentType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{{Attribute: "model", Required: false}}, nil).Maybe()
}

func (suite *ProvisioningExecutorTestSuite) createMockIdentifyingExecutor() providers.Executor {
	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return(ExecutorNameIdentifying).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeUtility).Maybe()
	mockExec.On("GetDefaultInputs").Return([]providers.Input{}).Maybe()
	mockExec.On("GetPrerequisites").Return([]providers.Input{}).Maybe()
	return mockExec
}

func (suite *ProvisioningExecutorTestSuite) createMockProvisioningExecutor() providers.Executor {
	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return(ExecutorNameProvisioning).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeRegistration).Maybe()
	mockExec.On("GetDefaultInputs").Return([]providers.Input{}).Maybe()
	mockExec.On("GetPrerequisites").Return([]providers.Input{}).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *providers.NodeContext, execResp *providers.ExecutorResponse) bool {
			if len(ctx.NodeInputs) == 0 {
				return true
			}
			for _, input := range ctx.NodeInputs {
				if _, ok := ctx.UserInputs[input.Identifier]; !ok {
					if _, ok := ctx.RuntimeData[input.Identifier]; !ok {
						execResp.Inputs = append(execResp.Inputs, input)
					}
				}
			}
			return len(execResp.Inputs) == 0
		}).Maybe()
	mockExec.On("GetInputs", mock.Anything).Return([]providers.Input{}).Maybe()
	mockExec.On(methodGetRequiredInputs, mock.Anything).Return([]providers.Input{}).Maybe()
	return mockExec
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_NonRegistrationFlow() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_Success() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "newuser", attributeEmail: "new@example.com"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username":     "newuser",
			attributeEmail: "new@example.com",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{
			{Identifier: "username", Type: "string", Required: true},
			{Identifier: attributeEmail, Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			"assignGroup": "test-group-id",
			"assignRole":  "test-role-id",
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username":     "newuser",
		attributeEmail: "new@example.com",
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	createdUser := &providers.User{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockUserMgtProvider.On("CreateUser", mock.Anything,
		mock.MatchedBy(func(u *providers.User) bool {
			return u.OUID == testOUID && u.Type == testUserType
		})).Return(createdUser, nil)

	suite.mockGroupService.On("AddMembersToGroups",
		mock.Anything, []group.Member{{ID: testNewUserID, Type: group.MemberTypeUser}}, []string{"test-group-id"}).
		Return((*tidcommon.ServiceError)(nil))
	suite.mockRoleAssignmentService.On("AddAssigneesToRoles", mock.Anything,
		[]role.RoleAssignment{{ID: testNewUserID, Type: role.AssigneeTypeUser}}, []string{"test-role-id"}).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockGroupService.AssertExpectations(suite.T())
	suite.mockRoleService.AssertExpectations(suite.T())
	suite.mockRoleAssignmentService.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_UserAlreadyExists() {
	suite.expectSchemaForProvisioning()
	nodeInputs := []providers.Input{{Identifier: "username", Type: "string", Required: true}}
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "existinguser",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
	}

	// Override GetRequiredInputs to return node inputs so the retry path is exercised
	provMock := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	for _, call := range provMock.ExpectedCalls {
		if call.Method == methodGetRequiredInputs {
			call.Unset()
		}
	}
	provMock.On(methodGetRequiredInputs, mock.Anything).Return(nodeInputs).Maybe()

	userID := "user-existing"
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username": "existinguser",
	}).Return(&userID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Contains(suite.T(), resp.Error.Error.String(), "The user already exists")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_NoUserAttributes() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{},
		NodeInputs:  []providers.Input{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CreateUserFails() {
	suite.expectSchemaForProvisioning()
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "newuser",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
	suite.mockUserMgtProvider.On("CreateUser", mock.Anything, mock.Anything).
		Return(nil, &tidcommon.InternalServerError)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	// The user service error reaches the flow unchanged instead of being flattened.
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, resp.Error.Code)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CreateUserFails_AttributeConflict() {
	suite.expectSchemaForProvisioning()
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "existinguser",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
	suite.mockUserMgtProvider.On("CreateUser", mock.Anything, mock.Anything).
		Return(nil, &user.ErrorAttributeConflict)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrProvisioningAttributeConflict.Code, resp.Error.Code)
	assert.Equal(suite.T(), ErrProvisioningAttributeConflict.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_AttributesFromAuthUser() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		NodeInputs:  []providers.Input{{Identifier: attributeEmail, Type: "string", Required: true}},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
}

// TestHasRequiredInputs_PromptsBooleanAttributeAsCheckbox verifies that a missing boolean schema
// attribute is prompted with a boolean input type rather than as free text, so the client can
// render a control that produces a value the schema accepts.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_PromptsBooleanAttributeAsCheckbox() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "username", Type: model.TypeString, Required: true},
			{Attribute: "active", Type: model.TypeBoolean, Required: true},
			{Attribute: "age", Type: model.TypeNumber, Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		NodeInputs:  []providers.Input{},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}

	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	promptedTypes := make(map[string]string, len(execResp.Inputs))
	for _, input := range execResp.Inputs {
		promptedTypes[input.Identifier] = input.Type
	}
	assert.Equal(suite.T(), providers.InputTypeText, promptedTypes["username"])
	assert.Equal(suite.T(), providers.InputTypeBoolean, promptedTypes["active"])
	assert.Equal(suite.T(), providers.InputTypeNumber, promptedTypes["age"])
}

// TestGetAttributesForProvisioning_ConvertsToSchemaTypes verifies that collected values, which the
// engine carries as strings, reach the store as the types the schema declares.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_ConvertsToSchemaTypes() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "username", Type: model.TypeString, Required: true},
			{Attribute: "active", Type: model.TypeBoolean, Required: true},
			{Attribute: "verified", Type: model.TypeBoolean, Required: true},
			{Attribute: "age", Type: model.TypeNumber, Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{
			"username": "testuser",
			"active":   "true",
			"age":      "42",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType, "verified": "false"},
		NodeInputs:  []providers.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.Equal(suite.T(), true, result["active"])
	assert.Equal(suite.T(), false, result["verified"])
	assert.Equal(suite.T(), float64(42), result["age"])
}

// TestGetAttributesForProvisioning_UnparseableBooleanIsPassedThrough verifies that a value that
// does not parse is left as-is, so schema validation reports it instead of a zero value being
// silently substituted.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_UnparseableBooleanIsPassedThrough() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "active", Type: model.TypeBoolean, Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs:  map[string]string{"active": "affirmative"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []providers.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.Equal(suite.T(), "affirmative", result["active"])
}

// TestGetAttributesForProvisioning_SchemaEmpty_ReturnsEmpty verifies that when the schema
// is unavailable (no userTypeKey → getUserType returns ""), an empty map is returned.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_SchemaEmpty_ReturnsEmpty() {
	ctx := &providers.NodeContext{
		UserInputs:  map[string]string{"username": "testuser", attributeEmail: "test@example.com"},
		RuntimeData: map[string]string{},
		NodeInputs:  []providers.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.Empty(suite.T(), result)
}

// TestGetAttributesForProvisioning_SchemaWhitelist_ExcludesNonSchemaAttrs verifies that the schema
// acts as a whitelist — attributes not in the schema are excluded even if present in context.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_SchemaWhitelist_ExcludesNonSchemaAttrs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{{Attribute: "username", Required: true}}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{
			"username": "testuser",
			"userID":   "user-123",
			"code":     "auth-code",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []providers.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.NotContains(suite.T(), result, "userID")
	assert.NotContains(suite.T(), result, "code")
}

// TestGetAttributesForProvisioning_RequiredAttrsFromMultipleSources verifies that required schema
// attributes are resolved from UserInputs and RuntimeData.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_RequiredAttrsFromMultipleSources() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: attributeEmail, Required: true},
			{Attribute: "given_name", Required: true},
			{Attribute: "phone", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{"username": "testuser"},
		RuntimeData: map[string]string{
			userTypeKey:    testUserType,
			attributeEmail: "auth@example.com",
			"given_name":   "Test",
			"phone":        "+1234567890",
		},
		NodeInputs: []providers.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.Equal(suite.T(), "auth@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "Test", result["given_name"])
	assert.Equal(suite.T(), "+1234567890", result["phone"])
}

// TestGetAttributesForProvisioning_ContextPriority verifies priority: UserInputs wins over
// AuthenticatedUser.Attributes which wins over RuntimeData (first non-empty source wins).
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_ContextPriority() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "name", Required: true},
			{Attribute: "phone", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{attributeEmail: "userinput@example.com"},
		RuntimeData: map[string]string{
			userTypeKey:    testUserType,
			attributeEmail: "runtime@example.com",
			"name":         "Authn Name",
			"phone":        "+1234567890",
		},
		NodeInputs: []providers.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	// UserInputs is checked first — wins for email.
	assert.Equal(suite.T(), "userinput@example.com", result[attributeEmail])
	// Only in RuntimeData — comes from there.
	assert.Equal(suite.T(), "Authn Name", result["name"])
	// Only in RuntimeData — comes from there.
	assert.Equal(suite.T(), "+1234567890", result["phone"])
}

// TestGetAttributesForProvisioning_AllAttrsCollectedWhenNoNodeInputs verifies that when
// node inputs are empty, all schema attrs with available values are collected (both required and optional).
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_AllAttrsCollectedWhenNoNodeInputs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "phone", Required: false},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{attributeEmail: "user@example.com"},
		RuntimeData: map[string]string{
			userTypeKey: testUserType,
			"phone":     "+1234567890",
		},
		NodeInputs: []providers.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.Equal(suite.T(), "user@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "+1234567890", result["phone"],
		"optional attr with a value must be collected when node inputs are empty")
}

// TestGetAttributesForProvisioning_OptionalAttrCollectedWhenInNodeInputs verifies that an optional
// schema attr is collected when it is explicitly listed in node inputs.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_OptionalAttrCollectedWhenInNodeInputs() {
	nodeInputs := []providers.Input{
		{Identifier: attributeEmail, Type: "EMAIL_INPUT", Required: true},
		{Identifier: "phone", Type: "TEXT_INPUT", Required: false},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "phone", Required: false},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{
			attributeEmail: "user@example.com",
			"phone":        "+1234567890",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
	}

	result, _, _ := exec.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.Equal(suite.T(), "user@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "+1234567890", result["phone"],
		"optional attr in node inputs must be collected")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_EmptyCredentialFallsBackToRuntime() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, Required: true, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{
			attributePassword: "",
		},
		RuntimeData: map[string]string{
			userTypeKey:       testUserType,
			attributePassword: "runtime-secret",
		},
		NodeInputs: []providers.Input{},
	}

	_, credentialAttrs, err := suite.executor.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "runtime-secret", credentialAttrs[attributePassword])
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_CredentialFromUserInputs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, Required: true, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{
			attributePassword: "input-secret",
		},
		RuntimeData: map[string]string{
			userTypeKey:       testUserType,
			attributePassword: "runtime-secret",
		},
		NodeInputs: []providers.Input{},
	}

	_, credentialAttrs, err := suite.executor.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "input-secret", credentialAttrs[attributePassword])
}

// newExecutorWithNodeInputs creates a provisioningExecutor whose embedded ExecutorInterface
// returns the given inputs from GetRequiredInputs.
func (suite *ProvisioningExecutorTestSuite) newExecutorWithNodeInputs(inputs []providers.Input) *provisioningExecutor {
	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetRequiredInputs", mock.Anything).Return(inputs).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true).Maybe()

	mockFlowFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFlowFactory.On("CreateExecutor", ExecutorNameProvisioning, providers.ExecutorTypeRegistration,
		mock.Anything, mock.Anything, mock.Anything).Return(mockExec)

	identifyingMock := suite.createMockIdentifyingExecutor()
	mockFlowFactory.On("CreateExecutor", ExecutorNameIdentifying, providers.ExecutorTypeUtility,
		mock.Anything, mock.Anything, mock.Anything).Return(identifyingMock).Maybe()

	return newProvisioningExecutor(mockFlowFactory,
		suite.mockGroupService, suite.mockRoleService, suite.mockRoleAssignmentService, suite.mockEntityProvider,
		suite.mockUserMgtProvider, suite.mockAgentMgtProvider, suite.mockEntityTypeService,
		suite.mockAuthnProvider)
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_FilteredPath_RequiredAttrFromUserInputs() {
	nodeInputs := []providers.Input{
		{Identifier: "username", Type: "TEXT_INPUT", Required: true},
		{Identifier: attributeEmail, Type: "EMAIL_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: attributeEmail, Required: true},
			{Attribute: "mobile_number", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{
			"username":      "testuser",
			attributeEmail:  "test@example.com",
			"mobile_number": "0771234567",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
	}

	result, _, _ := exec.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.Equal(suite.T(), "test@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "0771234567", result["mobile_number"],
		"required schema attr from UserInputs must be included even though it is not in node inputs")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_FilteredPath_RequiredAttrFromAuthnAttrs() {
	nodeInputs := []providers.Input{
		{Identifier: "username", Type: "TEXT_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: attributeEmail, Required: true},
			{Attribute: "given_name", Required: true},
			{Attribute: "mobile_number", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{"username": "testuser"},
		RuntimeData: map[string]string{
			userTypeKey:     testUserType,
			attributeEmail:  "federated@example.com",
			"given_name":    "Test",
			"mobile_number": "0779876543",
		},
		NodeInputs: nodeInputs,
	}

	result, _, _ := exec.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.Equal(suite.T(), "federated@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "Test", result["given_name"])
	assert.Equal(suite.T(), "0779876543", result["mobile_number"])
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_FilteredPath_UserInputTakesPriority() {
	nodeInputs := []providers.Input{
		{Identifier: attributeEmail, Type: "EMAIL_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "username", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{attributeEmail: "userinput@example.com"},
		RuntimeData: map[string]string{
			userTypeKey: testUserType,
			"username":  "federateduser",
		},
		NodeInputs: nodeInputs,
	}

	result, _, _ := exec.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.Equal(suite.T(), "userinput@example.com", result[attributeEmail],
		"UserInputs must win over RuntimeData for the same key")
	assert.Equal(suite.T(), "federateduser", result["username"],
		"required schema attr from RuntimeData must still be included")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_AllowRegistrationWithExistingUser_SkipsProvisioning() {
	suite.expectSchemaForProvisioning()
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "existinguser",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyAllowRegistrationWithExistingUser: dataValueTrue,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
	}

	userID := testExistingUser123ID
	attrs := map[string]interface{}{
		"username": "existinguser",
	}
	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(&userID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "CreateUser")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_NewUser_NoGroupOrRoleProperties() {
	suite.expectSchemaForProvisioning()
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username":     "newuser",
			attributeEmail: "new@example.com",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{
			{Identifier: "username", Type: "string", Required: true},
			{Identifier: attributeEmail, Type: "string", Required: true},
		},
		// No NodeProperties - should skip group/role assignment
	}

	attrs := map[string]interface{}{
		"username":     "newuser",
		attributeEmail: "new@example.com",
	}
	attrsJSON, _ := json.Marshal(attrs)

	createdUser := &providers.User{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
	suite.mockUserMgtProvider.On("CreateUser", mock.Anything,
		mock.MatchedBy(func(u *providers.User) bool {
			return u.OUID == testOUID && u.Type == testUserType
		})).Return(createdUser, nil)

	// No group/role assignment mocks - assignments should be skipped

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())

	// Verify no group/role methods were called
	suite.mockGroupService.AssertNotCalled(suite.T(), "AddMembersToGroups")
	suite.mockRoleAssignmentService.AssertNotCalled(suite.T(), "AddAssigneesToRoles")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_UserEligibleForProvisioning() {
	suite.expectSchemaForProvisioning()
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username":     "provisioneduser",
			attributeEmail: "provisioned@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyUserEligibleForProvisioning: dataValueTrue,
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{
			{Identifier: "username", Type: "string", Required: true},
			{Identifier: attributeEmail, Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptionalCredentials: false,
		},
	}

	attrs := map[string]interface{}{
		"username":     "provisioneduser",
		attributeEmail: "provisioned@example.com",
	}
	attrsJSON, _ := json.Marshal(attrs)

	createdUser := &providers.User{
		ID:         "user-provisioned",
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
	suite.mockUserMgtProvider.On("CreateUser", mock.Anything,
		mock.MatchedBy(func(u *providers.User) bool {
			return u.OUID == testOUID && u.Type == testUserType
		})).Return(createdUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), dataValueTrue, resp.RuntimeData[common.RuntimeKeyUserAutoProvisioned])
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_UserAutoProvisionedFlag_SetAfterCreation() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "newuser", attributeEmail: "new@example.com"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username":     "newuser",
			attributeEmail: "new@example.com",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
			common.RuntimeKeyUserEligibleForProvisioning: dataValueTrue,
		},
		NodeInputs: []providers.Input{
			{Identifier: "username", Type: "string", Required: true},
			{Identifier: attributeEmail, Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptionalCredentials: false,
		},
	}

	createdUser := &providers.User{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
	suite.mockUserMgtProvider.On("CreateUser", mock.Anything, mock.Anything).Return(createdUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), dataValueTrue, resp.RuntimeData[common.RuntimeKeyUserAutoProvisioned],
		"userAutoProvisioned flag should be set to true after successful provisioning")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_MissingInputs_MissingOUID() {
	suite.expectSchemaForProvisioning()
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []providers.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{"username": "newuser"}).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	suite.mockUserMgtProvider.On("CreateUser", mock.Anything,
		mock.MatchedBy(func(u *providers.User) bool {
			return u.OUID == ""
		})).Return(nil, &user.ErrorInvalidOUID).Once()

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	// The organization unit is validated by the user service, whose error reaches the flow intact.
	assert.Equal(suite.T(), user.ErrorInvalidOUID.Code, resp.Error.Code)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

// An ordinary user-provisioning flow must reach the user management provider with the organization
// unit and user type resolved from the node. This is the regression guard for existing provisioning
// flows.
func (suite *ProvisioningExecutorTestSuite) TestExecute_UserProvisioning_DelegatesToUserMgtProvider() {
	suite.expectSchemaForProvisioning()
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	var received *providers.User
	suite.mockUserMgtProvider.On("CreateUser", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			received = args.Get(1).(*providers.User)
		}).
		Return(&providers.User{ID: testNewUserID, OUID: testOUID, Type: testUserType}, nil).Once()

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.NotEqual(suite.T(), providers.ExecFailure, resp.Status)

	// The request carries exactly what the node resolved; the user service owns the rest.
	assert.NotNil(suite.T(), received)
	assert.Equal(suite.T(), testOUID, received.OUID)
	assert.Equal(suite.T(), testUserType, received.Type)
	assert.JSONEq(suite.T(), `{"username":"newuser"}`, string(received.Attributes))
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_MissingInputs_MissingUserType() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{ouIDKey: testOUID},
		NodeInputs:  []providers.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity")
	suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "CreateUser")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CreateUserFailures() {
	suite.expectSchemaForProvisioning()
	tests := []struct {
		name               string
		createdUser        *providers.User
		createUserError    *tidcommon.ServiceError
		expectedFailReason string
	}{
		{
			// The user service error reaches the flow unchanged rather than being flattened.
			name:               "ServiceReturnsError",
			createdUser:        nil,
			createUserError:    &tidcommon.InternalServerError,
			expectedFailReason: tidcommon.InternalServerError.Error.DefaultValue,
		},
		{
			name:               "CreatedUserIsNil",
			createdUser:        nil,
			createUserError:    nil,
			expectedFailReason: ErrProvisioningFailed.Error.DefaultValue,
		},
		{
			name: "CreatedUserHasEmptyID",
			createdUser: &providers.User{
				ID:         "",
				OUID:       testOUID,
				Type:       testUserType,
				Attributes: []byte(`{"username":"newuser"}`),
			},
			createUserError:    nil,
			expectedFailReason: ErrProvisioningFailed.Error.DefaultValue,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Clear expectations before each test
			suite.mockEntityProvider.ExpectedCalls = nil
			suite.mockUserMgtProvider.ExpectedCalls = nil

			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    providers.FlowTypeRegistration,
				UserInputs: map[string]string{
					"username": "newuser",
				},
				RuntimeData: map[string]string{
					ouIDKey:     testOUID,
					userTypeKey: testUserType,
				},
				NodeInputs: []providers.Input{
					{Identifier: "username", Type: "string", Required: true},
				},
			}

			attrs := map[string]interface{}{
				"username": "newuser",
			}
			suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
				entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
			suite.mockUserMgtProvider.On("CreateUser", mock.Anything, mock.Anything).
				Return(tt.createdUser, tt.createUserError)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), resp)
			assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
			assert.Equal(suite.T(), tt.expectedFailReason, resp.Error.Error.DefaultValue)
			suite.mockEntityProvider.AssertExpectations(suite.T())
		})
	}
}

func (suite *ProvisioningExecutorTestSuite) TestGetOUID() {
	tests := []struct {
		name        string
		runtimeData map[string]string
		userInputs  map[string]string
		expected    string
	}{
		{
			name: "RuntimeOUIDTakesPriority",
			runtimeData: map[string]string{
				ouIDKey:        "ou-from-resolver",
				defaultOUIDKey: "ou-from-usertype",
			},
			userInputs: map[string]string{
				ouIDKey: "ou-from-userinput",
			},
			expected: "ou-from-resolver",
		},
		{
			name: "DefaultOUIDWhenNoExplicitOUID",
			runtimeData: map[string]string{
				defaultOUIDKey: "ou-from-usertype",
			},
			expected: "ou-from-usertype",
		},
		{
			name:        "ReturnsEmptyWhenNotFound",
			runtimeData: map[string]string{},
			expected:    "",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			ctx := &providers.NodeContext{
				RuntimeData: tt.runtimeData,
				UserInputs:  tt.userInputs,
			}

			ouID := suite.executor.getOUID(ctx)

			assert.Equal(suite.T(), tt.expected, ouID)
		})
	}
}

// Each category carries the type to provision into under its own runtime key.
func (suite *ProvisioningExecutorTestSuite) TestGetEntityType() {
	tests := []struct {
		name        string
		category    entitytype.TypeCategory
		runtimeData map[string]string
		expected    string
	}{
		{
			name:     "Found",
			category: entitytype.TypeCategoryUser,
			runtimeData: map[string]string{
				userTypeKey: "CUSTOM_USER_TYPE",
			},
			expected: "CUSTOM_USER_TYPE",
		},
		{
			name:        "NotFound",
			category:    entitytype.TypeCategoryUser,
			runtimeData: map[string]string{},
			expected:    "",
		},
		{
			name:     "AgentCategoryReadsAgentTypeKey",
			category: entitytype.TypeCategoryAgent,
			runtimeData: map[string]string{
				agentTypeKey: "CUSTOM_AGENT_TYPE",
				userTypeKey:  "CUSTOM_USER_TYPE",
			},
			expected: "CUSTOM_AGENT_TYPE",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			ctx := &providers.NodeContext{
				RuntimeData: tt.runtimeData,
			}

			entityType := suite.executor.getEntityType(ctx, tt.category)

			assert.Equal(suite.T(), tt.expected, entityType)
		})
	}
}

// The mode property names the entity category to provision into, and a node that sets no mode
// keeps provisioning the default category.
func (suite *ProvisioningExecutorTestSuite) TestResolveCategory() {
	tests := []struct {
		name           string
		nodeProperties map[string]interface{}
		expected       entitytype.TypeCategory
		expectErr      bool
	}{
		{name: "NoModePropertyUsesDefault", nodeProperties: nil, expected: defaultProvisioningCategory},
		{
			name:           "NullModeUsesDefault",
			nodeProperties: map[string]interface{}{propertyKeyProvisioningMode: nil},
			expected:       defaultProvisioningCategory,
		},
		{
			name:           "UserMode",
			nodeProperties: map[string]interface{}{propertyKeyProvisioningMode: "user"},
			expected:       entitytype.TypeCategoryUser,
		},
		{
			name:           "AgentMode",
			nodeProperties: map[string]interface{}{propertyKeyProvisioningMode: "agent"},
			expected:       entitytype.TypeCategoryAgent,
		},
		{
			name:           "UnknownMode",
			nodeProperties: map[string]interface{}{propertyKeyProvisioningMode: "machine"},
			expectErr:      true,
		},
	}

	// The constant is spelled out in constants.go to keep that file import-free.
	assert.Equal(suite.T(), entitytype.TypeCategoryUser,
		entitytype.TypeCategory(defaultProvisioningCategory))

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			category, err := categoryFromMode(
				&providers.NodeContext{NodeProperties: tt.nodeProperties})

			if tt.expectErr {
				assert.Error(suite.T(), err)
				return
			}
			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), tt.expected, category)
		})
	}
}

// The interface entry point carries no category, so it resolves one itself and fails the node when
// the mode property does not name a category.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_UnknownProvisioningMode_ReturnsFailure() {
	ctx := &providers.NodeContext{
		ExecutionID:    "flow-123",
		NodeProperties: map[string]interface{}{propertyKeyProvisioningMode: "machine"},
		RuntimeData:    map[string]string{ouIDKey: testOUID, userTypeKey: testUserType},
	}
	execResp := &providers.ExecutorResponse{}

	assert.False(suite.T(), suite.executor.HasRequiredInputs(ctx, execResp))
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_UnknownProvisioningMode_ReturnsError() {
	ctx := &providers.NodeContext{
		ExecutionID:    "flow-123",
		FlowType:       providers.FlowTypeRegistration,
		NodeProperties: map[string]interface{}{propertyKeyProvisioningMode: "machine"},
		UserInputs:     map[string]string{"username": "newuser"},
		RuntimeData:    map[string]string{ouIDKey: testOUID, userTypeKey: testUserType},
	}

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "CreateUser")
}

// Creating the entity is the one category-bound step. A category with no creator fails there,
// after the category-independent preparation has run.
// An agent-mode node reaches the agent management provider with the target the node resolved, the
// schema attributes it collected, and the agent's own fields the flow collected. It publishes the
// generated identifier and client credentials, which the create response is the only chance to read.
func (suite *ProvisioningExecutorTestSuite) TestExecute_AgentProvisioning_DelegatesToAgentMgtProvider() {
	suite.expectSchemaForAgentProvisioning()
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	var received *providers.Agent
	suite.mockAgentMgtProvider.On("CreateAgent", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			received = args.Get(1).(*providers.Agent)
		}).
		Return(&providers.Agent{
			ID: testNewAgentID,
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{
					Type: providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{
						ClientID:     "client-abc",
						ClientSecret: "secret-xyz",
					},
				},
			},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		NodeProperties: map[string]interface{}{
			propertyKeyProvisioningMode: string(entitytype.TypeCategoryAgent),
		},
		UserInputs: map[string]string{
			"model":        "claude",
			nameKey:        "support-bot",
			descriptionKey: "Handles support tickets",
			logoURLKey:     "https://example.com/logo.png",
		},
		RuntimeData: map[string]string{
			ouIDKey:      testOUID,
			agentTypeKey: testAgentType,
			ownerKey:     "user-owner-1",
		},
		NodeInputs: []providers.Input{{Identifier: "model", Type: "string", Required: true}},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	require.NotNil(suite.T(), resp)
	assert.NotEqual(suite.T(), providers.ExecFailure, resp.Status)

	// The target and the agent's own fields are forwarded as collected; the agent service validates them.
	require.NotNil(suite.T(), received)
	assert.Equal(suite.T(), testOUID, received.OUID)
	assert.Equal(suite.T(), testAgentType, received.Type)
	assert.Equal(suite.T(), "support-bot", received.Name)
	assert.Equal(suite.T(), "Handles support tickets", received.Description)
	assert.Equal(suite.T(), "https://example.com/logo.png", received.LogoURL)
	assert.Equal(suite.T(), "user-owner-1", received.Owner)
	// Only schema attributes reach the attributes payload.
	assert.JSONEq(suite.T(), `{"model":"claude"}`, string(received.Attributes))

	assert.Equal(suite.T(), testNewAgentID, resp.AdditionalData[common.DataAgentID])
	assert.Equal(suite.T(), "client-abc", resp.AdditionalData[common.DataAgentClientID])
	assert.Equal(suite.T(), "secret-xyz", resp.AdditionalData[common.DataAgentClientSecret])
	suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "CreateUser")
}

// Every attribute on the default agent type is optional, and an agent's name is a column on its
// record rather than a schema attribute. An agent given only a name therefore reaches the create
// call with an empty attributes payload, and must not be rejected as having supplied nothing.
func (suite *ProvisioningExecutorTestSuite) TestExecute_AgentProvisioning_NameOnlyIsNotEmptyProvisioning() {
	suite.expectSchemaForAgentProvisioning()

	var received *providers.Agent
	suite.mockAgentMgtProvider.On("CreateAgent", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			received = args.Get(1).(*providers.Agent)
		}).
		Return(&providers.Agent{ID: testNewAgentID}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAdministration,
		NodeProperties: map[string]interface{}{
			propertyKeyProvisioningMode: string(entitytype.TypeCategoryAgent),
		},
		UserInputs: map[string]string{nameKey: "support-bot"},
		RuntimeData: map[string]string{
			ouIDKey:      testOUID,
			agentTypeKey: testAgentType,
		},
		NodeInputs: []providers.Input{},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity", mock.Anything)

	assert.NoError(suite.T(), err)
	require.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	require.NotNil(suite.T(), received)
	assert.Equal(suite.T(), "support-bot", received.Name)
	assert.JSONEq(suite.T(), `{}`, string(received.Attributes))
}

// An attribute the schema restricts to a fixed set is offered as a choice. A prompt node that
// declares the field itself carries empty options and is enriched from these by identifier, which
// is what lets a flow fix the field order without repeating the permitted values.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_EnumAttributeIsOfferedAsSelect() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, entitytype.TypeCategoryAgent, testAgentType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "modelProvider", Required: false, Type: "string",
				Enum: []string{"openai", "anthropic", "gemini"}},
			{Attribute: "model", Required: false, Type: "string"},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAdministration,
		NodeProperties: map[string]interface{}{
			propertyKeyProvisioningMode:             string(entitytype.TypeCategoryAgent),
			propertyKeyDynamicInputsIncludeOptional: true,
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{ouIDKey: testOUID, agentTypeKey: testAgentType},
		NodeInputs:  []providers.Input{},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	suite.False(suite.executor.HasRequiredInputs(ctx, execResp))

	byID := make(map[string]providers.Input, len(execResp.Inputs))
	for _, inp := range execResp.Inputs {
		byID[inp.Identifier] = inp
	}

	enumInput, ok := byID["modelProvider"]
	suite.Require().True(ok, "the enum attribute must be prompted")
	suite.Equal(providers.InputTypeSelect, enumInput.Type)
	suite.Equal([]string{"openai", "anthropic", "gemini"}, enumInput.Options,
		"options must keep the order the schema declared them in")

	plainInput, ok := byID["model"]
	suite.Require().True(ok)
	suite.NotEqual(providers.InputTypeSelect, plainInput.Type, "an unconstrained attribute stays free text")
	suite.Empty(plainInput.Options)
}

// A user carries no record fields of its own, so empty attribute maps really do mean nothing was
// supplied and the node must still fail.
func (suite *ProvisioningExecutorTestSuite) TestExecute_UserProvisioning_NoAttributesStillFails() {
	suite.expectSchemaForProvisioning()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{nameKey: "not-a-user-field"},
		RuntimeData: map[string]string{ouIDKey: testOUID, userTypeKey: testUserType},
		NodeInputs:  []providers.Input{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	require.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrProvisioningAttrsMissing.Code, resp.Error.Code)
	suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "CreateUser")
}

// A provisioned agent is attached to groups and roles as an agent principal. The mapping functions
// are covered separately; this asserts the executor actually applies them, because a correct
// mapping wired to the wrong call would still hand the services a user principal.
func (suite *ProvisioningExecutorTestSuite) TestExecute_AgentProvisioning_AssignsAsAnAgentPrincipal() {
	suite.expectSchemaForAgentProvisioning()
	suite.mockAgentMgtProvider.On("CreateAgent", mock.Anything, mock.Anything, mock.Anything).
		Return(&providers.Agent{ID: testNewAgentID}, nil).Once()

	suite.mockGroupService.On("AddMembersToGroups", mock.Anything,
		[]group.Member{{ID: testNewAgentID, Type: group.MemberTypeAgent}}, []string{"agent-group-id"}).
		Return((*tidcommon.ServiceError)(nil))
	suite.mockRoleAssignmentService.On("AddAssigneesToRoles", mock.Anything,
		[]role.RoleAssignment{{ID: testNewAgentID, Type: role.AssigneeTypeAgent}}, []string{"agent-role-id"}).
		Return((*tidcommon.ServiceError)(nil))

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAdministration,
		NodeProperties: map[string]interface{}{
			propertyKeyProvisioningMode: string(entitytype.TypeCategoryAgent),
			propertyKeyAssignGroup:      "agent-group-id",
			propertyKeyAssignRole:       "agent-role-id",
		},
		UserInputs:  map[string]string{nameKey: "support-bot"},
		RuntimeData: map[string]string{ouIDKey: testOUID, agentTypeKey: testAgentType},
		NodeInputs:  []providers.Input{},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity", mock.Anything)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockGroupService.AssertExpectations(suite.T())
	suite.mockRoleAssignmentService.AssertExpectations(suite.T())
}

// Redirect URIs are the one part of an agent's OAuth configuration a flow supplies. They are
// attached only when collected, so an absent value leaves the inbound auth config unset and the
// provider free to apply its own default rather than receiving an empty list to interpret.
func (suite *ProvisioningExecutorTestSuite) TestExecute_AgentProvisioning_RedirectURIsInput() {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{name: "AbsentLeavesTheConfigUnset", input: "", expected: nil},
		{name: "SingleURIIsForwarded", input: "https://a.example.com/cb",
			expected: []string{"https://a.example.com/cb"}},
		{name: "SeveralURIsAreSplitAndTrimmed", input: " https://a.example.com/cb , https://b.example.com/cb ",
			expected: []string{"https://a.example.com/cb", "https://b.example.com/cb"}},
		{name: "SeparatorsOnlyLeaveTheConfigUnset", input: " , ", expected: nil},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTest()
			suite.expectSchemaForAgentProvisioning()
			suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(nil,
				entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

			var received *providers.Agent
			suite.mockAgentMgtProvider.On("CreateAgent", mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					received = args.Get(1).(*providers.Agent)
				}).
				Return(&providers.Agent{ID: testNewAgentID}, nil).Once()

			userInputs := map[string]string{"model": "claude", nameKey: "support-bot"}
			if tt.input != "" {
				userInputs[redirectURIsKey] = tt.input
			}
			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    providers.FlowTypeAdministration,
				NodeProperties: map[string]interface{}{
					propertyKeyProvisioningMode: string(entitytype.TypeCategoryAgent),
				},
				UserInputs:  userInputs,
				RuntimeData: map[string]string{ouIDKey: testOUID, agentTypeKey: testAgentType},
				NodeInputs:  []providers.Input{},
			}

			_, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			require.NotNil(suite.T(), received)
			if tt.expected == nil {
				assert.Empty(suite.T(), received.InboundAuthConfig,
					"the provider decides the configuration when the flow supplied no URIs")
				return
			}
			require.Len(suite.T(), received.InboundAuthConfig, 1)
			require.NotNil(suite.T(), received.InboundAuthConfig[0].OAuthConfig)
			assert.Equal(suite.T(), tt.expected, received.InboundAuthConfig[0].OAuthConfig.RedirectURIs)
			// The shape is the provider's to derive, so nothing else is set here.
			assert.Empty(suite.T(), received.InboundAuthConfig[0].OAuthConfig.GrantTypes)
		})
	}
}

// Delegated is read from a fixed collected input rather than the entity type schema, and reaches
// the executor as a string. It is parsed the way a boolean schema attribute is, so the casings and
// the 1/0 forms a hand-authored flow may carry are accepted alongside the "true" a checkbox
// submits. Anything unparseable leaves the agent acting on its own behalf, and the input never
// reaches the schema attributes payload.
func (suite *ProvisioningExecutorTestSuite) TestExecute_AgentProvisioning_DelegatedInput() {
	tests := []struct {
		name      string
		delegated string
		expected  bool
	}{
		{name: "AbsentIsNotDelegated", delegated: "", expected: false},
		{name: "TrueIsDelegated", delegated: dataValueTrue, expected: true},
		{name: "FalseIsNotDelegated", delegated: dataValueFalse, expected: false},
		{name: "MixedCaseTrueIsDelegated", delegated: "True", expected: true},
		{name: "UpperCaseTrueIsDelegated", delegated: "TRUE", expected: true},
		{name: "OneIsDelegated", delegated: "1", expected: true},
		{name: "ZeroIsNotDelegated", delegated: "0", expected: false},
		{name: "UnrecognizedValueIsNotDelegated", delegated: "yes", expected: false},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTest()
			suite.expectSchemaForAgentProvisioning()
			suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(nil,
				entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

			var received *providers.Agent
			var receivedDelegated bool
			suite.mockAgentMgtProvider.On("CreateAgent", mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					received = args.Get(1).(*providers.Agent)
					receivedDelegated = args.Get(2).(bool)
				}).
				Return(&providers.Agent{ID: testNewAgentID}, nil).Once()

			userInputs := map[string]string{"model": "claude", nameKey: "support-bot"}
			if tt.delegated != "" {
				userInputs[delegatedKey] = tt.delegated
			}
			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    providers.FlowTypeRegistration,
				NodeProperties: map[string]interface{}{
					propertyKeyProvisioningMode: string(entitytype.TypeCategoryAgent),
				},
				UserInputs:  userInputs,
				RuntimeData: map[string]string{ouIDKey: testOUID, agentTypeKey: testAgentType},
				NodeInputs:  []providers.Input{{Identifier: "model", Type: "string", Required: true}},
			}

			_, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			require.NotNil(suite.T(), received)
			assert.Equal(suite.T(), tt.expected, receivedDelegated)
			// The flag steers the agent's auth shape; it is not one of its attributes.
			assert.JSONEq(suite.T(), `{"model":"claude"}`, string(received.Attributes))
		})
	}
}

// An agent service failure reaches the flow unchanged, so the caller can tell a name or owner
// violation it can correct from a configuration problem it cannot.
func (suite *ProvisioningExecutorTestSuite) TestExecute_AgentProvisioning_SurfacesServiceError() {
	suite.expectSchemaForAgentProvisioning()
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
	svcErr := &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AGT-1003",
		Error: tidcommon.I18nMessage{
			Key: "agent.name_required", DefaultValue: "Agent name is required",
		},
	}
	suite.mockAgentMgtProvider.On("CreateAgent", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, svcErr).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		NodeProperties: map[string]interface{}{
			propertyKeyProvisioningMode: string(entitytype.TypeCategoryAgent),
		},
		UserInputs:  map[string]string{"model": "claude"},
		RuntimeData: map[string]string{ouIDKey: testOUID, agentTypeKey: testAgentType},
		NodeInputs:  []providers.Input{{Identifier: "model", Type: "string", Required: true}},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	require.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	require.NotNil(suite.T(), resp.Error)
	assert.Equal(suite.T(), "AGT-1003", resp.Error.Code)
	assert.Equal(suite.T(), "Agent name is required", resp.Error.Error.DefaultValue)
}

// A category with no creator cannot be reached through a node today, but the create step still
// refuses it rather than reporting a missing identifier.
func (suite *ProvisioningExecutorTestSuite) TestCreateEntity_UnsupportedCategory_Fails() {
	execResp := &providers.ExecutorResponse{AdditionalData: map[string]string{}}

	entityID, svcErr := suite.executor.createEntity(&providers.NodeContext{ExecutionID: "flow-123"},
		entitytype.TypeCategory("machine"), map[string]interface{}{}, execResp, suite.executor.logger)

	assert.Empty(suite.T(), entityID)
	require.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrProvisioningFailed.Code, svcErr.Code)
	assert.Equal(suite.T(), "Failed to provision the machine", svcErr.Error.String())
}

// The already-exists errors name the category being provisioned rather than hard-coding "user".
func (suite *ProvisioningExecutorTestSuite) TestExecute_AgentAlreadyExists_ErrorNamesTheCategory() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, entitytype.TypeCategoryAgent, testAgentType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{{Attribute: "username", Required: false}}, nil).Maybe()
	existingID := "agent-existing"
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(&existingID, nil)

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		NodeProperties: map[string]interface{}{
			propertyKeyProvisioningMode: string(entitytype.TypeCategoryAgent),
		},
		UserInputs:  map[string]string{"username": "newagent"},
		RuntimeData: map[string]string{ouIDKey: testOUID, agentTypeKey: testAgentType},
		NodeInputs:  []providers.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	require.NotNil(suite.T(), resp)
	require.NotNil(suite.T(), resp.Error)
	assert.Equal(suite.T(), ErrEntityAlreadyExists.Code, resp.Error.Code)
	assert.Equal(suite.T(), "The agent already exists", resp.Error.Error.String())
	assert.Equal(suite.T(), "The provided attributes already belong to an existing agent",
		resp.Error.ErrorDescription.String())
}

// With no target in runtime data, the application's allowed user types are the candidate source.
// Exactly one self-registrable type resolves; none or several leave the target unresolved, which is
// not an error.
func (suite *ProvisioningExecutorTestSuite) TestGetDefaultEntityRef_UserCategory() {
	tests := []struct {
		name        string
		allowed     []string
		entityTypes map[string]*entitytype.EntityType
		expected    *entityRef
	}{
		{
			name:     "NoAllowedUserTypes",
			allowed:  nil,
			expected: nil,
		},
		{
			name:    "SingleSelfRegistrableTypeResolves",
			allowed: []string{testUserType},
			entityTypes: map[string]*entitytype.EntityType{
				testUserType: {Name: testUserType, OUID: testOUID, AllowSelfRegistration: true},
			},
			expected: &entityRef{entityType: testUserType, ouID: testOUID},
		},
		{
			name:    "NoSelfRegistrableType",
			allowed: []string{testUserType},
			entityTypes: map[string]*entitytype.EntityType{
				testUserType: {Name: testUserType, OUID: testOUID, AllowSelfRegistration: false},
			},
			expected: nil,
		},
		{
			name:    "AmbiguousSelfRegistrableTypes",
			allowed: []string{testUserType, "EXTERNAL"},
			entityTypes: map[string]*entitytype.EntityType{
				testUserType: {Name: testUserType, OUID: testOUID, AllowSelfRegistration: true},
				"EXTERNAL":   {Name: "EXTERNAL", OUID: testOUID, AllowSelfRegistration: true},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTest()
			for name, et := range tt.entityTypes {
				suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything,
					entitytype.TypeCategoryUser, name).
					Return(et, (*tidcommon.ServiceError)(nil)).Maybe()
			}
			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				Application: providers.Application{
					InboundAuthProfile: providers.InboundAuthProfile{AllowedUserTypes: tt.allowed},
				},
			}

			ref, err := suite.executor.getDefaultEntityRef(ctx, entitytype.TypeCategoryUser)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), tt.expected, ref)
		})
	}
}

func (suite *ProvisioningExecutorTestSuite) TestGetDefaultEntityRef_EntityTypeLookupFails() {
	suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything,
		entitytype.TypeCategoryUser, testUserType).
		Return(nil, &tidcommon.ServiceError{Code: "internal_error",
			Error: tidcommon.I18nMessage{DefaultValue: "boom"}}).Once()
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		Application: providers.Application{
			InboundAuthProfile: providers.InboundAuthProfile{AllowedUserTypes: []string{testUserType}},
		},
	}

	ref, err := suite.executor.getDefaultEntityRef(ctx, entitytype.TypeCategoryUser)

	assert.Nil(suite.T(), ref)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to retrieve entity type")
}

// The agent types an application accepts name the target, which is what lets a flow provision an
// agent with no type or organization unit resolver node ahead of the provisioning one.
func (suite *ProvisioningExecutorTestSuite) TestGetDefaultEntityRef_AgentCategory_ResolvesTheAllowedAgentType() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		Application: providers.Application{
			InboundAuthProfile: providers.InboundAuthProfile{AllowedAgentTypes: []string{testAgentType}},
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryAgent,
		testAgentType).
		Return(&entitytype.EntityType{Name: testAgentType, OUID: testOUID},
			(*tidcommon.ServiceError)(nil)).Once()

	ref, err := suite.executor.getDefaultEntityRef(ctx, entitytype.TypeCategoryAgent)

	assert.NoError(suite.T(), err)
	require.NotNil(suite.T(), ref)
	assert.Equal(suite.T(), testAgentType, ref.entityType)
	assert.Equal(suite.T(), testOUID, ref.ouID, "the organization unit comes from the agent type itself")
}

// An application that names no agent type still provisions, so a deployment that never configured
// one keeps working.
func (suite *ProvisioningExecutorTestSuite) TestGetDefaultEntityRef_AgentCategory_FallsBackToTheDefaultType() {
	ctx := &providers.NodeContext{ExecutionID: "flow-123"}
	suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryAgent,
		entitytype.DefaultAgentTypeName).
		Return(&entitytype.EntityType{Name: entitytype.DefaultAgentTypeName, OUID: testOUID},
			(*tidcommon.ServiceError)(nil)).Once()

	ref, err := suite.executor.getDefaultEntityRef(ctx, entitytype.TypeCategoryAgent)

	assert.NoError(suite.T(), err)
	require.NotNil(suite.T(), ref)
	assert.Equal(suite.T(), entitytype.DefaultAgentTypeName, ref.entityType)
}

// Several allowed types leave the choice to a resolver node rather than picking one arbitrarily.
func (suite *ProvisioningExecutorTestSuite) TestGetDefaultEntityRef_AgentCategory_SeveralAllowedIsUnresolved() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		Application: providers.Application{
			InboundAuthProfile: providers.InboundAuthProfile{
				AllowedAgentTypes: []string{testAgentType, "another"},
			},
		},
	}

	ref, err := suite.executor.getDefaultEntityRef(ctx, entitytype.TypeCategoryAgent)

	assert.NoError(suite.T(), err, "an unresolved target is not an error")
	assert.Nil(suite.T(), ref)
	suite.mockEntityTypeService.AssertNotCalled(suite.T(), "GetEntityTypeByName", mock.Anything,
		entitytype.TypeCategoryAgent, mock.Anything)
}

// An agent is provisioned on an administrator's behalf, so the type need not permit the entity to
// register itself the way the user candidates must.
func (suite *ProvisioningExecutorTestSuite) TestGetDefaultEntityRef_AgentCategory_IgnoresSelfRegistration() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		Application: providers.Application{
			InboundAuthProfile: providers.InboundAuthProfile{AllowedAgentTypes: []string{testAgentType}},
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryAgent,
		testAgentType).
		Return(&entitytype.EntityType{Name: testAgentType, OUID: testOUID, AllowSelfRegistration: false},
			(*tidcommon.ServiceError)(nil)).Once()

	ref, err := suite.executor.getDefaultEntityRef(ctx, entitytype.TypeCategoryAgent)

	assert.NoError(suite.T(), err)
	require.NotNil(suite.T(), ref)
	assert.Equal(suite.T(), testAgentType, ref.entityType)
}

// Each category carries its type under its own runtime key, and neither has a fixed fallback.
func (suite *ProvisioningExecutorTestSuite) TestGetEntityType_ComesFromRuntimeData() {
	ctx := &providers.NodeContext{ExecutionID: "flow-123", RuntimeData: map[string]string{}}

	assert.Empty(suite.T(), suite.executor.getEntityType(ctx, entitytype.TypeCategoryAgent),
		"an unset agent type is resolved from the application, not assumed here")
	assert.Empty(suite.T(), suite.executor.getEntityType(ctx, entitytype.TypeCategoryUser),
		"a user type is resolved by a node, so it has no fixed fallback")

	ctx.RuntimeData[agentTypeKey] = "from-runtime"
	assert.Equal(suite.T(), "from-runtime", suite.executor.getEntityType(ctx, entitytype.TypeCategoryAgent),
		"runtime data still wins when a node resolved a type")
}

// The steps around the create address the provisioned entity as a principal of its own category.
func (suite *ProvisioningExecutorTestSuite) TestCategoryPrincipalTypes() {
	assert.Equal(suite.T(), group.MemberTypeUser, groupMemberTypeFor(entitytype.TypeCategoryUser))
	assert.Equal(suite.T(), group.MemberTypeAgent, groupMemberTypeFor(entitytype.TypeCategoryAgent))
	assert.Equal(suite.T(), role.AssigneeTypeUser, roleAssigneeTypeFor(entitytype.TypeCategoryUser))
	assert.Equal(suite.T(), role.AssigneeTypeAgent, roleAssigneeTypeFor(entitytype.TypeCategoryAgent))
	assert.Equal(suite.T(), user.ErrorAttributeConflict.Code,
		attributeConflictCodeFor(entitytype.TypeCategoryUser))
	assert.Equal(suite.T(), agentAttributeConflictCode, attributeConflictCodeFor(entitytype.TypeCategoryAgent))
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_AllAttributesInRuntimeData() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			attributeEmail: "user@example.com",
			"username":     "testuser",
			userTypeKey:    testUserType,
		},
		NodeInputs: []providers.Input{
			{Identifier: attributeEmail, Type: "string", Required: true},
			{Identifier: "username", Type: "string", Required: true},
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	inputRequired := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), inputRequired)
	assert.Equal(suite.T(), 0, len(execResp.Inputs))
}

// Test group assignment failure - provisioning should fail, but role assignment should still be attempted
func (suite *ProvisioningExecutorTestSuite) TestExecute_Failure_GroupAssignmentFails() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "newuser"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "newuser",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			"assignGroup": "test-group-id",
			"assignRole":  "test-role-id",
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	createdUser := &providers.User{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockUserMgtProvider.On("CreateUser", mock.Anything, mock.Anything).Return(createdUser, nil)

	suite.mockGroupService.On("AddMembersToGroups",
		mock.Anything, []group.Member{{ID: testNewUserID, Type: group.MemberTypeUser}}, []string{"test-group-id"}).
		Return(&tidcommon.ServiceError{
			Error: tidcommon.I18nMessage{Key: "error.test.group_not_found", DefaultValue: "Group not found"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrProvisioningAssignmentFailed.Error.DefaultValue, resp.Error.Error.DefaultValue)

	suite.mockGroupService.AssertExpectations(suite.T())
	suite.mockRoleService.AssertExpectations(suite.T())
}

// Test role assignment failure - provisioning should fail, but group assignment succeeds
func (suite *ProvisioningExecutorTestSuite) TestExecute_Failure_RoleAssignmentFails() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "newuser"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "newuser",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			"assignGroup": "test-group-id",
			"assignRole":  "test-role-id",
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	createdUser := &providers.User{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockUserMgtProvider.On("CreateUser", mock.Anything, mock.Anything).Return(createdUser, nil)

	suite.mockGroupService.On("AddMembersToGroups",
		mock.Anything, []group.Member{{ID: testNewUserID, Type: group.MemberTypeUser}}, []string{"test-group-id"}).
		Return((*tidcommon.ServiceError)(nil))
	suite.mockRoleAssignmentService.On("AddAssigneesToRoles", mock.Anything,
		[]role.RoleAssignment{{ID: testNewUserID, Type: role.AssigneeTypeUser}}, []string{"test-role-id"}).
		Return(&tidcommon.ServiceError{
			Error: tidcommon.I18nMessage{Key: "error.test.role_not_found", DefaultValue: "Role not found"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrProvisioningAssignmentFailed.Error.DefaultValue, resp.Error.Error.DefaultValue)

	suite.mockGroupService.AssertExpectations(suite.T())
	suite.mockRoleAssignmentService.AssertExpectations(suite.T())
	suite.mockRoleService.AssertExpectations(suite.T())
}

// Test group with existing members - user should be appended
func (suite *ProvisioningExecutorTestSuite) TestExecute_GroupWithExistingMembers() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "newuser"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "newuser",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			"assignGroup": "test-group-id",
			"assignRole":  "test-role-id",
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	createdUser := &providers.User{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockUserMgtProvider.On("CreateUser", mock.Anything, mock.Anything).Return(createdUser, nil)

	suite.mockGroupService.On("AddMembersToGroups",
		mock.Anything, []group.Member{{ID: testNewUserID, Type: group.MemberTypeUser}}, []string{"test-group-id"}).
		Return((*tidcommon.ServiceError)(nil))
	suite.mockRoleAssignmentService.On("AddAssigneesToRoles", mock.Anything,
		[]role.RoleAssignment{{ID: testNewUserID, Type: role.AssigneeTypeUser}}, []string{"test-role-id"}).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockGroupService.AssertExpectations(suite.T())
}

// Test authentication flow with auto-provisioning still assigns groups/roles
func (suite *ProvisioningExecutorTestSuite) TestExecute_AuthFlow_AutoProvisioning_AssignsGroupsAndRoles() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "provisioneduser"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username": "provisioneduser",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyUserEligibleForProvisioning: dataValueTrue,
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			"assignGroup": "test-group-id",
			"assignRole":  "test-role-id",
			propertyKeyDynamicInputsIncludeOptionalCredentials: false,
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	createdUser := &providers.User{
		ID:         "user-provisioned",
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockUserMgtProvider.On("CreateUser", mock.Anything, mock.Anything).Return(createdUser, nil)

	suite.mockGroupService.On("AddMembersToGroups",
		mock.Anything, []group.Member{{ID: "user-provisioned", Type: group.MemberTypeUser}}, []string{"test-group-id"}).
		Return((*tidcommon.ServiceError)(nil))
	suite.mockRoleAssignmentService.On("AddAssigneesToRoles", mock.Anything,
		[]role.RoleAssignment{{ID: "user-provisioned", Type: role.AssigneeTypeUser}}, []string{"test-role-id"}).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), dataValueTrue, resp.RuntimeData[common.RuntimeKeyUserAutoProvisioned])

	// Verify assignments were made
	suite.mockGroupService.AssertExpectations(suite.T())
	suite.mockRoleService.AssertExpectations(suite.T())
}

// Test successful provisioning with both group and role assignment (detailed verification)
func (suite *ProvisioningExecutorTestSuite) TestExecute_Success_WithGroupAndRoleAssignment() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "newuser", attributeEmail: "new@example.com"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username":     "newuser",
			attributeEmail: "new@example.com",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{
			{Identifier: "username", Type: "string", Required: true},
			{Identifier: attributeEmail, Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			"assignGroup": "test-group-id",
			"assignRole":  "test-role-id",
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username":     "newuser",
		attributeEmail: "new@example.com",
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	createdUser := &providers.User{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockUserMgtProvider.On("CreateUser", mock.Anything,
		mock.MatchedBy(func(u *providers.User) bool {
			return u.OUID == testOUID && u.Type == testUserType
		})).Return(createdUser, nil)

	suite.mockGroupService.On("AddMembersToGroups",
		mock.Anything, []group.Member{{ID: testNewUserID, Type: group.MemberTypeUser}}, []string{"test-group-id"}).
		Return((*tidcommon.ServiceError)(nil))
	suite.mockRoleAssignmentService.On("AddAssigneesToRoles", mock.Anything,
		[]role.RoleAssignment{{ID: testNewUserID, Type: role.AssigneeTypeUser}}, []string{"test-role-id"}).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)

	// Verify all mocks were called
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockGroupService.AssertExpectations(suite.T())
	suite.mockRoleService.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_Success_WithMultipleGroupsAndRoles() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "newuser"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			"assignGroup": "group-1, group-2",
			"assignRole":  "role-1, role-2",
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	createdUser := &providers.User{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}
	suite.mockUserMgtProvider.On("CreateUser", mock.Anything, mock.Anything).Return(createdUser, nil)

	suite.mockGroupService.On("AddMembersToGroups",
		mock.Anything, []group.Member{{ID: testNewUserID, Type: group.MemberTypeUser}}, []string{"group-1", "group-2"}).
		Return((*tidcommon.ServiceError)(nil))
	suite.mockRoleAssignmentService.On("AddAssigneesToRoles", mock.Anything,
		[]role.RoleAssignment{{ID: testNewUserID, Type: role.AssigneeTypeUser}}, []string{"role-1", "role-2"}).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)

	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockGroupService.AssertExpectations(suite.T())
	suite.mockRoleAssignmentService.AssertExpectations(suite.T())
}

// Cross-OU provisioning tests

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_Success() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}
	attrsJSON, _ := json.Marshal(attrs)

	existingUserID := testExistingUserID
	existingUser := &providers.Entity{
		ID:   existingUserID,
		OUID: "ou-source",
	}

	createdUser := &providers.User{
		ID:         testNewUserID,
		Type:       testUserType,
		OUID:       testOUID,
		Attributes: attrsJSON,
	}

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"sub": "user-sub-123",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeProperties: map[string]interface{}{
			common.NodePropertyAllowCrossOUProvisioning: true,
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(&existingUserID, nil)
	suite.mockEntityProvider.On("GetEntity", existingUserID).Return(existingUser, nil)
	suite.mockUserMgtProvider.On("CreateUser", mock.Anything,
		mock.MatchedBy(func(u *providers.User) bool {
			return u.OUID == testOUID
		})).Return(createdUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_NotEnabled_Fails() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"sub": "user-sub-123",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeProperties: map[string]interface{}{},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(&existingUserID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "The user already exists", resp.Error.Error.String())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_SameOU_Fails() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID
	existingUser := &providers.Entity{
		ID:   existingUserID,
		OUID: testOUID, // same as target
	}

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"sub": "user-sub-123",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeProperties: map[string]interface{}{
			common.NodePropertyAllowCrossOUProvisioning: true,
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(&existingUserID, nil)
	suite.mockEntityProvider.On("GetEntity", existingUserID).Return(existingUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "The user already exists in the target organization", resp.Error.Error.String())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_NoTargetOU_Fails() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"sub": "user-sub-123",
		},
		RuntimeData: map[string]string{
			userTypeKey: testUserType,
			// no ouIDKey — target OU not set
		},
		NodeProperties: map[string]interface{}{
			common.NodePropertyAllowCrossOUProvisioning: true,
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(&existingUserID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrCrossOUProvisioningTargetMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_RetryableProvisioningErrors() {
	tests := []struct {
		name           string
		existingUserID string
		expectedReason string
		message        string
	}{
		{
			name:           "User already exists",
			existingUserID: "user-existing",
			expectedReason: "The user already exists",
			message:        "Should return inputs for retry when user already exists in registration flow",
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			suite.SetupTest()
			suite.expectSchemaForProvisioning()

			nodeInputs := []providers.Input{
				{Identifier: "username", Type: "string", Required: true},
			}
			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    providers.FlowTypeRegistration,
				UserInputs:  map[string]string{"username": "existinguser"},
				NodeInputs:  nodeInputs,
				RuntimeData: map[string]string{userTypeKey: testUserType},
			}

			// Override GetRequiredInputs to return node inputs for this test
			provMock := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
			for _, call := range provMock.ExpectedCalls {
				if call.Method == methodGetRequiredInputs {
					call.Unset()
				}
			}
			provMock.On(methodGetRequiredInputs, mock.Anything).Return(nodeInputs).Maybe()

			existingID := tt.existingUserID
			suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
				"username": "existinguser",
			}).Return(&existingID, nil)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, providers.ExecUserInputRequired, resp.Status)
			assert.Equal(t, tt.expectedReason, resp.Error.Error.String(), tt.message)
			assert.NotEmpty(t, resp.Inputs, "Inputs should be re-populated for retry")
			suite.mockEntityProvider.AssertExpectations(t)
		})
	}
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_NotEnabled_AuthnFlow_ReturnsFailure() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"sub": "user-sub-123",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
			common.RuntimeKeyUserEligibleForProvisioning: dataValueTrue,
		},
		NodeProperties: map[string]interface{}{},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(&existingUserID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status,
		"Authentication flow should skip provisioning and return ExecComplete when user already exists")
	assert.Nil(suite.T(), resp.Error, "No error should be set when skipping provisioning in authentication flow")
	assert.Empty(suite.T(), resp.Inputs, "Inputs should not be populated for authentication flows")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_NotEnabled_RegistrationFlow_PopulatesInputs() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID

	nodeInputs := []providers.Input{
		{Identifier: "sub", Type: "string", Required: true},
	}
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"sub": "user-sub-123",
		},
		NodeInputs: nodeInputs,
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeProperties: map[string]interface{}{},
	}

	// Override GetRequiredInputs to return node inputs
	provMock := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	for _, call := range provMock.ExpectedCalls {
		if call.Method == methodGetRequiredInputs {
			call.Unset()
		}
	}
	provMock.On(methodGetRequiredInputs, mock.Anything).Return(nodeInputs).Maybe()

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(&existingUserID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), "The user already exists", resp.Error.Error.String())
	assert.NotEmpty(suite.T(), resp.Inputs, "Inputs should be populated so the user can correct their input")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_SameOU_AuthnFlow_ReturnsFailure() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID
	existingUser := &providers.Entity{
		ID:   existingUserID,
		OUID: testOUID, // same as target
	}

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"sub": "user-sub-123",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
			common.RuntimeKeyUserEligibleForProvisioning: dataValueTrue,
		},
		NodeProperties: map[string]interface{}{
			common.NodePropertyAllowCrossOUProvisioning: true,
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(&existingUserID, nil)
	suite.mockEntityProvider.On("GetEntity", existingUserID).Return(existingUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status,
		"Authentication flow should skip provisioning and return ExecComplete when user exists in target OU")
	assert.Nil(suite.T(), resp.Error, "No error should be set when skipping provisioning in authentication flow")
	assert.Empty(suite.T(), resp.Inputs, "Inputs should not be populated for authentication flows")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_SameOU_RegistrationFlow_PopulatesInputs() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID
	existingUser := &providers.Entity{
		ID:   existingUserID,
		OUID: testOUID, // same as target
	}

	nodeInputs := []providers.Input{
		{Identifier: "sub", Type: "string", Required: true},
	}
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"sub": "user-sub-123",
		},
		NodeInputs: nodeInputs,
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeProperties: map[string]interface{}{
			common.NodePropertyAllowCrossOUProvisioning: true,
		},
	}

	// Override GetRequiredInputs to return node inputs
	provMock := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	for _, call := range provMock.ExpectedCalls {
		if call.Method == methodGetRequiredInputs {
			call.Unset()
		}
	}
	provMock.On(methodGetRequiredInputs, mock.Anything).Return(nodeInputs).Maybe()

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(&existingUserID, nil)
	suite.mockEntityProvider.On("GetEntity", existingUserID).Return(existingUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), "The user already exists in the target organization", resp.Error.Error.String())
	assert.NotEmpty(suite.T(), resp.Inputs, "Inputs should be populated so the user can correct their input")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_GetUserError() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"sub": "user-sub-123",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeProperties: map[string]interface{}{
			common.NodePropertyAllowCrossOUProvisioning: true,
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(&existingUserID, nil)
	suite.mockEntityProvider.On("GetEntity", existingUserID).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "db error", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaAttrSatisfiedByUserInputs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{{Attribute: attributeEmail, DisplayName: "Email"}}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{attributeEmail: "user@example.com"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
	assert.Nil(suite.T(), execResp.ForwardedData)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaAttrSatisfiedByRuntimeData() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{{Attribute: attributeEmail, DisplayName: ""}}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType, attributeEmail: "user@example.com"},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaAttrSatisfiedByAuthnAttrs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email"},
			{Attribute: "firstName", DisplayName: "First Name"},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaAttrMissing_AppendedToInputsAndForwardedData() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email Address", Required: true},
			{Attribute: "firstName", DisplayName: "", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Len(suite.T(), execResp.Inputs, 2)

	inputMap := make(map[string]providers.Input, len(execResp.Inputs))
	for _, inp := range execResp.Inputs {
		inputMap[inp.Identifier] = inp
	}

	emailInput, ok := inputMap[attributeEmail]
	assert.True(suite.T(), ok)
	assert.True(suite.T(), emailInput.Required, "required schema attr must have Required=true in the built input")
	assert.Equal(suite.T(), "Email Address", emailInput.DisplayName)

	firstNameInput, ok := inputMap["firstName"]
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "", firstNameInput.DisplayName)

	assert.NotNil(suite.T(), execResp.ForwardedData)
	fwdInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs].([]providers.Input)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), fwdInputs, 2)
}

// TestHasRequiredInputs_IncludeOptionalTrue_OptionalRenderedAsNotRequired verifies that when
// includeOptional=true, optional schema attrs are forwarded with Required=false.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalTrue_OptionalRenderedAsNotRequired() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	inputMap := make(map[string]providers.Input, len(execResp.Inputs))
	for _, inp := range execResp.Inputs {
		inputMap[inp.Identifier] = inp
	}
	assert.True(suite.T(), inputMap[attributeEmail].Required, "required attr must be marked required")
	assert.False(suite.T(), inputMap["nickname"].Required,
		"optional attr must be marked not-required so the UI does not force the user to fill it")
}

// TestHasRequiredInputs_IncludeOptionalTrue_SkipsOptionalAlreadyPresented verifies that when
// includeOptional=true an optional attr recorded as already presented in RuntimeData
// is not re-prompted, even if the user left it empty.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalTrue_SkipsOptionalAlreadyPresented() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{attributeEmail: "user@example.com"},
		RuntimeData: map[string]string{
			userTypeKey: testUserType,
			// nickname was presented in the previous iteration and the user left it blank.
			common.RuntimeKeyPresentedOptionalInputs: "nickname",
		},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result,
		"must not block when all required attrs are satisfied and optional was already presented")
	assert.Empty(suite.T(), execResp.Inputs,
		"nickname must not be re-prompted once it appears in the presented list")
}

// TestHasRequiredInputs_IncludeOptionalTrue_DoesNotStorePresentedOptionals verifies that
// presented-input tracking is now owned by the flow engine, not provisioning.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalTrue_DoesNotStorePresentedOptionals() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	suite.executor.HasRequiredInputs(ctx, execResp)

	_, ok := execResp.RuntimeData[common.RuntimeKeyPresentedOptionalInputs]
	assert.False(suite.T(), ok, "provisioning should not write presented-input tracking directly")
}

// TestHasRequiredInputs_IncludeOptionalTrue_RequiredBeforeOptional verifies that required missing
// attrs always appear before optional ones in the prompted list.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalTrue_RequiredBeforeOptional() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "phone", DisplayName: "Phone", Required: false},
			{Attribute: "firstName", DisplayName: "First Name", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	suite.executor.HasRequiredInputs(ctx, execResp)

	require := true
	for _, inp := range execResp.Inputs {
		if inp.Required {
			assert.True(suite.T(), require,
				"required attr %q must come before optional attrs", inp.Identifier)
		} else {
			require = false
		}
	}
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaAttrCoveredByNodeInput_NotDuplicated() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{{Attribute: attributeEmail, DisplayName: "Email"}}, nil).Once()

	// email is already a node-defined input — schema must not create a second copy
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []providers.Input{{Identifier: attributeEmail, Type: "string", Required: true}},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result, "node input still missing so overall result is false")
	emailCount := 0
	for _, inp := range execResp.Inputs {
		if inp.Identifier == attributeEmail {
			emailCount++
		}
	}
	assert.Equal(suite.T(), 1, emailCount, "email must appear exactly once, not duplicated by schema")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IgnoresAbsentNodeInputWhenSchemaAttrsSatisfied() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{{Attribute: attributeEmail, DisplayName: ""}}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{attributeEmail: "user@example.com"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []providers.Input{{Identifier: "username", Required: true}},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result, "schema-absent node input must be ignored; all schema attrs are satisfied")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaServiceError_ReturnsFailure() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return(nil, &tidcommon.ServiceError{Code: "internal_error"}).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result, "schema service error must fail the executor")
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
	assert.Empty(suite.T(), execResp.Inputs)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_RequiredCredential_PromptedAsPassword() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Len(suite.T(), execResp.Inputs, 1)
	assert.Equal(suite.T(), attributePassword, execResp.Inputs[0].Identifier)
	assert.Equal(suite.T(), providers.InputTypePassword, execResp.Inputs[0].Type)
	assert.True(suite.T(), execResp.Inputs[0].Required)

	fwdInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs].([]providers.Input)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), fwdInputs, 1)
	assert.Equal(suite.T(), providers.InputTypePassword, fwdInputs[0].Type)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_RequiredCredentialSatisfied_ReturnsTrue() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{attributePassword: "secret"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_RequiredCredentialInAuthnAttrs_StillPrompted() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Len(suite.T(), execResp.Inputs, 1)
	assert.Equal(suite.T(), attributePassword, execResp.Inputs[0].Identifier)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_AllCredentials_PromptedByDefault() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		// No includeOptionalCredentials property — defaults to false, only required credentials prompted
		NodeInputs: []providers.Input{},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	passwordFound := false
	pinFound := false
	for _, inp := range execResp.Inputs {
		if inp.Identifier == attributePassword {
			passwordFound = true
			assert.True(suite.T(), inp.Required)
			assert.Equal(suite.T(), providers.InputTypePassword, inp.Type)
		}
		if inp.Identifier == attributePin {
			pinFound = true
			assert.False(suite.T(), inp.Required)
			assert.Equal(suite.T(), providers.InputTypePassword, inp.Type)
		}
	}
	assert.True(suite.T(), passwordFound, "required credential must be prompted by default")
	assert.False(suite.T(), pinFound, "optional credential must NOT be prompted by default")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalCreds_False_OnlyRequired() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptionalCredentials: false,
		},
		NodeInputs: []providers.Input{},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	// Should still be missing because password and email are required and not satisfied
	assert.False(suite.T(), result)
	passwordFound := false
	pinFound := false
	for _, inp := range execResp.Inputs {
		if inp.Identifier == attributePassword {
			passwordFound = true
			assert.True(suite.T(), inp.Required)
		}
		if inp.Identifier == attributePin {
			pinFound = true
		}
		if inp.Identifier == attributeEmail {
			assert.True(suite.T(), inp.Required)
		}
	}
	assert.True(suite.T(), passwordFound,
		"required credential must be prompted even when includeOptionalCredentials is false")
	assert.False(suite.T(), pinFound,
		"optional credential must not be prompted when includeOptionalCredentials is false")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptional_IndependentOfCredentials() {
	// includeOptional controls non-credential attrs, includeOptionalCredentials (default false)
	// controls optional credentials independently.
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
		NodeInputs: []providers.Input{},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	suite.executor.HasRequiredInputs(ctx, execResp)

	nicknameFound := false
	pinFound := false
	for _, inp := range execResp.Inputs {
		if inp.Identifier == "nickname" {
			nicknameFound = true
		}
		if inp.Identifier == attributePin {
			pinFound = true
		}
	}
	assert.True(suite.T(), nicknameFound, "includeOptional must prompt optional non-credential attrs")
	assert.False(suite.T(), pinFound, "optional credentials are NOT prompted by default")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_NodeInputUpgradesOptionalCredentialToRequired() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		// Node marks pin as required even though schema says optional.
		NodeInputs: []providers.Input{{Identifier: attributePin, Type: providers.InputTypePassword, Required: true}},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	pinFound := false
	for _, inp := range execResp.Inputs {
		if inp.Identifier == attributePin {
			pinFound = true
			assert.True(
				suite.T(), inp.Required,
				"node input upgrading optional credential to required must be honored",
			)
			assert.Equal(suite.T(), providers.InputTypePassword, inp.Type)
		}
	}
	assert.True(suite.T(), pinFound)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_AlreadyPromptedOptionalCredential_Skipped() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			userTypeKey:                              testUserType,
			common.RuntimeKeyPresentedOptionalInputs: attributePin,
		},
		NodeInputs: []providers.Input{{Identifier: attributePin, Type: providers.InputTypePassword, Required: false}},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result, "already-prompted optional credential should not block progress")
	for _, inp := range execResp.Inputs {
		assert.NotEqual(
			suite.T(), attributePin, inp.Identifier,
			"already-prompted optional credential must not be re-prompted",
		)
	}
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalCreds_True_AllPrompted() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptionalCredentials: true,
		},
		NodeInputs: []providers.Input{},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	passwordFound := false
	pinFound := false
	for _, inp := range execResp.Inputs {
		if inp.Identifier == attributePassword {
			passwordFound = true
			assert.True(suite.T(), inp.Required)
			assert.Equal(suite.T(), providers.InputTypePassword, inp.Type)
		}
		if inp.Identifier == attributePin {
			pinFound = true
			assert.False(suite.T(), inp.Required)
			assert.Equal(suite.T(), providers.InputTypePassword, inp.Type)
		}
	}
	assert.True(suite.T(), passwordFound, "includeOptionalCredentials=true must prompt required credentials")
	assert.True(suite.T(), pinFound, "includeOptionalCredentials=true must prompt optional credentials")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalCredentials_AlreadyPrompted_Skipped() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			userTypeKey:                              testUserType,
			common.RuntimeKeyPresentedOptionalInputs: attributePin,
		},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptionalCredentials: true,
		},
		NodeInputs: []providers.Input{},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result, "already-prompted optional credential should not block")
	for _, inp := range execResp.Inputs {
		assert.NotEqual(suite.T(), attributePin, inp.Identifier,
			"already-prompted optional credential must not be re-prompted")
	}
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalCreds_WithRequired() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptionalCredentials: false,
		},
		NodeInputs: []providers.Input{},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result, "should still need inputs when password and email are missing")
	emailFound := false
	passwordFound := false
	for _, inp := range execResp.Inputs {
		if inp.Identifier == attributeEmail {
			emailFound = true
		}
		if inp.Identifier == attributePassword {
			passwordFound = true
		}
	}
	assert.True(suite.T(), emailFound, "non-credential attributes must still be prompted")
	assert.True(suite.T(), passwordFound,
		"required credentials must be prompted even when includeOptionalCredentials=false")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_OptionalCreds_IndependentOfOptional() {
	// includeOptionalCredentials and includeOptional work independently.
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional:            false,
			propertyKeyDynamicInputsIncludeOptionalCredentials: true,
		},
		NodeInputs: []providers.Input{},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	suite.executor.HasRequiredInputs(ctx, execResp)

	pinFound := false
	nicknameFound := false
	for _, inp := range execResp.Inputs {
		if inp.Identifier == attributePin {
			pinFound = true
		}
		if inp.Identifier == "nickname" {
			nicknameFound = true
		}
	}
	assert.True(suite.T(), pinFound, "includeOptionalCredentials=true must prompt optional credentials")
	assert.False(suite.T(), nicknameFound, "includeOptional=false must not prompt optional non-credentials")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaRequiredCredentialNotLoweredByNodeInput() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		// Node tries to mark schema-required credential as optional — schema wins.
		NodeInputs: []providers.Input{
			{Identifier: attributePassword, Type: providers.InputTypePassword, Required: false},
		},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	for _, inp := range execResp.Inputs {
		if inp.Identifier == attributePassword {
			assert.True(suite.T(), inp.Required,
				"schema-required credential cannot be lowered to optional by node input")
		}
	}
}

// TestHasRequiredInputs_Ordering verifies the required non-credentials → optional
// non-credentials → required credentials → optional credentials ordering.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_Ordering_NonCredFirst_CredNext() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
		// pin is optional credential listed in node inputs so it will be prompted.
		NodeInputs: []providers.Input{{Identifier: attributePin, Type: providers.InputTypePassword, Required: false}},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	suite.executor.HasRequiredInputs(ctx, execResp)

	// Expected order: email (req non-cred) → nickname (opt non-cred) → password
	// (req cred) → pin (opt cred)
	identifiers := make([]string, 0, len(execResp.Inputs))
	for _, inp := range execResp.Inputs {
		identifiers = append(identifiers, inp.Identifier)
	}
	assert.Equal(suite.T(), []string{attributeEmail, "nickname", attributePassword, attributePin}, identifiers)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_MaxPerPrompt_CapsForwardedPromptBatchOnly() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "phone", DisplayName: "Phone", Required: true},
			{Attribute: "firstName", DisplayName: "First Name", Required: true},
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
			{Attribute: attributePin, DisplayName: "PIN", Required: true, Credential: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyMaxDynamicInputsPerPrompt: 1,
		},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	suite.executor.HasRequiredInputs(ctx, execResp)

	credCount, ncCount := 0, 0
	for _, inp := range execResp.Inputs {
		if inp.Type == providers.InputTypePassword {
			credCount++
		} else {
			ncCount++
		}
	}
	assert.Equal(suite.T(), 2, credCount, "full missing set should retain all credential inputs")
	assert.Equal(suite.T(), 3, ncCount, "full missing set should retain all non-credential inputs")

	fwdInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs].([]providers.Input)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), fwdInputs, 1, "prompt batch should be capped by maxPerPrompt")
	assert.NotEqual(suite.T(), providers.InputTypePassword, fwdInputs[0].Type,
		"first forwarded input should be a non-credential (non-credentials come first)")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_SchemaFilteredNoNodeInputs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: attributeEmail, Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{
			"username":    "testuser",
			"extra_field": "should-not-appear",
		},
		RuntimeData: map[string]string{
			userTypeKey:    testUserType,
			attributeEmail: "test@example.com",
		},
		NodeInputs: []providers.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.Equal(suite.T(), "test@example.com", result[attributeEmail])
	assert.NotContains(suite.T(), result, "extra_field",
		"attrs not defined in schema must be excluded when schema is available")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_OptionalAttrCollectedWhenNoNodeInputs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "phone", Required: false},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs:  map[string]string{attributeEmail: "user@example.com"},
		RuntimeData: map[string]string{userTypeKey: testUserType, "phone": "+1234567890"},
		NodeInputs:  []providers.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.Equal(suite.T(), "user@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "+1234567890", result["phone"],
		"optional schema attr with a value must be collected when node inputs are empty")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_SchemaServiceError_ReturnsError() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return(nil, &tidcommon.ServiceError{Code: "internal_error"}).Once()

	ctx := &providers.NodeContext{
		UserInputs:  map[string]string{attributeEmail: "user@example.com", "username": "testuser"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []providers.Input{},
	}

	result, _, err := suite.executor.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.Nil(suite.T(), result, "schema service error must return nil map")
	assert.Error(suite.T(), err, "schema service error must propagate as an error")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_OptionalAttrCollectedWithoutNodeInput() {
	nodeInputs := []providers.Input{{Identifier: attributeEmail, Required: true}}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "phone", Required: false},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs:  map[string]string{attributeEmail: "user@example.com", "phone": "+1234567890"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
	}

	result, _, err := exec.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "user@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "+1234567890", result["phone"],
		"optional schema attr with a value must be collected even when nodeInputSet is non-empty")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_GetAttributesError_ReturnsServerError() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{}, nil).Once()
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return(nil, &tidcommon.ServiceError{Code: "internal_error"}).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []providers.Input{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.Nil(suite.T(), resp)
	assert.Error(suite.T(), err)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_EmptySchemaAttrs_NoUserAttributes() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{}, nil).Once()
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []providers.Input{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "No user attributes provided for provisioning", resp.Error.Error.String())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_IdentifyEntity_AmbiguousMatch_ReturnsFailureEarly() {
	suite.expectSchemaForProvisioning()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	suite.mockEntityProvider.On("IdentifyEntity",
		map[string]interface{}{"username": "newuser"}).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeAmbiguousEntity, "ambiguous", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.NotEqual(suite.T(), ErrUserNotFound.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_UnmarshalAttributesError_ReturnsServerError() {
	suite.expectSchemaForProvisioning()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []providers.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{"username": "newuser"}).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
	suite.mockUserMgtProvider.On("CreateUser", mock.Anything, mock.Anything).
		Return(&providers.User{
			ID:         testNewUserID,
			OUID:       testOUID,
			Type:       testUserType,
			Attributes: []byte(`invalid json`),
		}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_NilRuntimeData_IsInitialized() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []providers.Input{},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: nil}

	suite.executor.HasRequiredInputs(ctx, execResp)

	assert.NotNil(suite.T(), execResp.RuntimeData)
}

func (suite *ProvisioningExecutorTestSuite) TestGetGroupsToAssign_NonStringValue_ReturnsNil() {
	ctx := &providers.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyAssignGroup: 42,
		},
	}

	result := suite.executor.getGroupsToAssign(ctx)

	assert.Nil(suite.T(), result)
}

func (suite *ProvisioningExecutorTestSuite) TestGetRolesToAssign_NonStringValue_ReturnsNil() {
	ctx := &providers.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyAssignRole: true,
		},
	}

	result := suite.executor.getRolesToAssign(ctx)

	assert.Nil(suite.T(), result)
}

func (suite *ProvisioningExecutorTestSuite) TestFetchSchemaAttributeInfos_NilService_ReturnsNil() {
	pe := &provisioningExecutor{
		Executor:                     suite.executor.Executor,
		identifyingExecutorInterface: suite.executor.identifyingExecutorInterface,
		entityProvider:               suite.executor.entityProvider,
		groupService:                 suite.executor.groupService,
		roleService:                  suite.executor.roleService,
		entityTypeService:            nil,
		logger:                       suite.executor.logger,
	}

	ctx := &providers.NodeContext{
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}

	attrs, err := pe.fetchSchemaAttributes(ctx, entitytype.TypeCategoryUser, false, true)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), attrs)
}

func (suite *ProvisioningExecutorTestSuite) TestFetchSchemaAttributeInfos_NonCred_ServiceError_ReturnsError() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowNonCredential: true}).
		Return(nil, &tidcommon.ServiceError{Code: "internal_error"}).Once()

	ctx := &providers.NodeContext{
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}

	attrs, err := suite.executor.fetchSchemaAttributes(ctx, entitytype.TypeCategoryUser, false, true)

	assert.Nil(suite.T(), attrs)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to fetch schema attributes for entity type")
}

func (suite *ProvisioningExecutorTestSuite) TestFetchSchemaAttributeInfos_Cred_ServiceError_ReturnsError() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true}).
		Return(nil, &tidcommon.ServiceError{Code: "internal_error"}).Once()

	ctx := &providers.NodeContext{
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}

	attrs, err := suite.executor.fetchSchemaAttributes(ctx, entitytype.TypeCategoryUser, true, false)

	assert.Nil(suite.T(), attrs)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to fetch schema attributes for entity type")
}

// The user service owns organization-unit and user-type validation. The executor forwards the
// request and surfaces the service's typed error rather than pre-validating and flattening it.
func (suite *ProvisioningExecutorTestSuite) TestCreateUserInStore_MissingUserType_SurfacesServiceError() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{ouIDKey: testOUID},
	}
	suite.mockUserMgtProvider.On("CreateUser", mock.Anything,
		mock.MatchedBy(func(u *providers.User) bool {
			return u.Type == ""
		})).Return(nil, &user.ErrorEntityTypeNotFound).Once()

	result, svcErr := suite.executor.createUserInStore(ctx, map[string]interface{}{"username": "testuser"})

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), user.ErrorEntityTypeNotFound.Code, svcErr.Code)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_MissingUserType_ReturnsFailure() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
}

// TestHasRequiredInputs_IncludeOptionalTrue_PromptsOptionals verifies that when
// includeOptional=true, missing optional schema attributes are also requested via prompt.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalTrue_PromptsOptionals() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{attributeEmail: "user@example.com"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	identifiers := make([]string, 0, len(execResp.Inputs))
	for _, inp := range execResp.Inputs {
		identifiers = append(identifiers, inp.Identifier)
	}
	assert.Contains(suite.T(), identifiers, "nickname",
		"optional attr must be prompted when includeOptional=true")
	assert.NotContains(suite.T(), identifiers, attributeEmail, "already-satisfied attr must not be re-prompted")
}

// TestHasRequiredInputs_IncludeOptionalFalse_SkipsOptionals verifies the default
// behavior: optional schema attrs are not prompted when includeOptional is absent or false.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalFalse_SkipsOptionals() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID:    "flow-123",
		FlowType:       providers.FlowTypeRegistration,
		UserInputs:     map[string]string{attributeEmail: "user@example.com"},
		RuntimeData:    map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
}

// TestHasRequiredInputs_NodeOptionalAttr_PromptedWithoutIncludeOptional
// verifies that a schema-optional non-credential attr still prompts when the node explicitly asks
// for it, even if includeOptional is absent or false.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_NodeOptionalAttr_PromptedWithoutIncludeOptional() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{attributeEmail: "user@example.com"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs: []providers.Input{
			{Identifier: "nickname", Type: providers.InputTypeText, Required: false},
		},
		NodeProperties: map[string]interface{}{},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	require.Len(suite.T(), execResp.Inputs, 1)
	assert.Equal(suite.T(), "nickname", execResp.Inputs[0].Identifier)
	assert.False(suite.T(), execResp.Inputs[0].Required)
	fwdInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs].([]providers.Input)
	assert.True(suite.T(), ok)
	require.Len(suite.T(), fwdInputs, 1)
	assert.Equal(suite.T(), "nickname", fwdInputs[0].Identifier)
}

// TestHasRequiredInputs_MaxPerPrompt_LimitsPromptedAttrs verifies that when maxPerPrompt=1,
// only one missing schema attribute is forwarded to the prompt per iteration.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_MaxPerPrompt_LimitsPromptedAttrs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "firstName", DisplayName: "First Name", Required: true},
			{Attribute: "lastName", DisplayName: "Last Name", Required: true},
			{Attribute: "phone", DisplayName: "Phone", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyMaxDynamicInputsPerPrompt: 1,
		},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Len(suite.T(), execResp.Inputs, 3, "full missing set should be retained on the executor response")
	fwdInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs].([]providers.Input)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), fwdInputs, 1, "only one input should be forwarded to the prompt per iteration")
	assert.Equal(suite.T(), "firstName", fwdInputs[0].Identifier)
}

// TestHasRequiredInputs_MaxPerPrompt_Zero_PromptsAllMissingAttrs verifies that maxPerPrompt=0
// (the default) prompts all missing attributes at once.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_MaxPerPrompt_Zero_PromptsAllMissingAttrs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "firstName", DisplayName: "First Name", Required: true},
			{Attribute: "lastName", DisplayName: "Last Name", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID:    "flow-123",
		FlowType:       providers.FlowTypeRegistration,
		UserInputs:     map[string]string{},
		RuntimeData:    map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Len(suite.T(), execResp.Inputs, 2, "all missing inputs should be prompted when maxPerPrompt is not set")
}

// TestGetAttributesForProvisioning_IncludeOptionalTrue_NoEffect verifies that
// includeOptional=true does not alter schema-backed attribute collection.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_IncludeOptionalTrue_NoEffect() {
	nodeInputs := []providers.Input{
		{Identifier: attributeEmail, Type: "EMAIL_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "nickname", Required: false},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{
			attributeEmail: "user@example.com",
			"nickname":     "nick",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
	}

	result, _, err := exec.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "user@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "nick", result["nickname"],
		"optional attr with a value must be collected regardless of includeOptional")
}

// TestGetAttributesForProvisioning_IncludeOptionalFalse_CollectsOptionals verifies that
// includeOptional=false does not exclude schema-backed values during collection.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_IncludeOptionalFalse_CollectsOptionals() {
	nodeInputs := []providers.Input{
		{Identifier: attributeEmail, Type: "EMAIL_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "nickname", Required: false},
		}, nil).Once()

	ctx := &providers.NodeContext{
		UserInputs: map[string]string{
			attributeEmail: "user@example.com",
			"nickname":     "nick",
		},
		RuntimeData:    map[string]string{userTypeKey: testUserType},
		NodeInputs:     nodeInputs,
		NodeProperties: map[string]interface{}{},
	}

	result, _, err := exec.getAttributesForProvisioning(ctx, entitytype.TypeCategoryUser)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "user@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "nick", result["nickname"],
		"optional attr with a value must be collected regardless of includeOptional")
}

// TestHasRequiredInputs_MaxPerPrompt_Float64_LimitsPromptedAttrs verifies that maxPerPrompt
// supplied as float64 (the type JSON unmarshalling produces) is handled correctly for the
// forwarded prompt batch.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_MaxPerPrompt_Float64_LimitsPromptedAttrs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "firstName", DisplayName: "First Name", Required: true},
			{Attribute: "lastName", DisplayName: "Last Name", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyMaxDynamicInputsPerPrompt: float64(1),
		},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Len(suite.T(), execResp.Inputs, 2,
		"full missing set should be retained on the executor response")
	fwdInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs].([]providers.Input)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), fwdInputs, 1,
		"float64 maxPerPrompt value (from JSON) must cap the forwarded prompt batch")
}

// TestExecute_SchemaErrorOnProvisioning_ReturnsServerError verifies that when getAttributesForProvisioning
// fails with a schema service error, Execute propagates it as a server error.
func (suite *ProvisioningExecutorTestSuite) TestExecute_SchemaErrorOnProvisioning_ReturnsServerError() {
	// HasRequiredInputs: username is satisfied so execution proceeds.
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{{Attribute: "username", Required: true}}, nil).Once()
	// getAttributesForProvisioning: schema service fails.
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return(nil, &tidcommon.ServiceError{Error: tidcommon.I18nMessage{DefaultValue: "schema unavailable"}}).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{ouIDKey: testOUID, userTypeKey: testUserType},
		NodeInputs:  []providers.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.Nil(suite.T(), resp)
	assert.Error(suite.T(), err)
	suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "CreateUser")
}

// TestHasRequiredInputs_NoProperties_DefaultBehavior verifies that when no properties are set the
// executor falls back to prompting only required schema attributes, all at once.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_NoProperties_DefaultBehavior() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType,
		model.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]model.AttributeInfo{
			{Attribute: "firstName", DisplayName: "First Name", Required: true},
			{Attribute: "lastName", DisplayName: "Last Name", Required: true},
		}, nil).Once()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &providers.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	ids := make([]string, 0, len(execResp.Inputs))
	for _, inp := range execResp.Inputs {
		ids = append(ids, inp.Identifier)
	}
	assert.Contains(suite.T(), ids, "firstName")
	assert.Contains(suite.T(), ids, "lastName")
	assert.Len(suite.T(), execResp.Inputs, 2,
		"all required missing inputs must be prompted at once when maxPerPrompt is absent")
}

// Ambiguous user (exists in multiple OUs) + cross-OU allowed + no match in target OU → create.
func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_AmbiguousUser_NoMatchInTargetOU_Creates() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}
	attrsJSON, _ := json.Marshal(attrs)

	createdUser := &providers.User{
		ID:         testNewUserID,
		Type:       testUserType,
		OUID:       testOUID,
		Attributes: attrsJSON,
	}

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{"sub": "user-sub-123"},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeProperties: map[string]interface{}{
			common.NodePropertyAllowCrossOUProvisioning: true,
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeAmbiguousEntity, "ambiguous", ""))
	suite.mockEntityProvider.On("SearchEntities", attrs).
		Return([]*providers.Entity{
			{ID: testExistingUserID, OUID: "ou-toyota"},
			{ID: "other-user-id", OUID: "ou-honda"},
		}, nil)
	suite.mockUserMgtProvider.On("CreateUser", mock.Anything,
		mock.MatchedBy(func(u *providers.User) bool {
			return u.OUID == testOUID
		})).Return(createdUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

// Ambiguous user + cross-OU allowed + match found in target OU → fail "already exists in target".
func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_AmbiguousUser_MatchInTargetOU_Fails() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{"sub": "user-sub-123"},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeProperties: map[string]interface{}{
			common.NodePropertyAllowCrossOUProvisioning: true,
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeAmbiguousEntity, "ambiguous", ""))
	suite.mockEntityProvider.On("SearchEntities", attrs).
		Return([]*providers.Entity{
			{ID: testExistingUserID, OUID: testOUID},
			{ID: "other-user-id", OUID: "ou-honda"},
		}, nil)
	suite.mockEntityProvider.On("GetEntity", testExistingUserID).
		Return(&providers.Entity{ID: testExistingUserID, OUID: testOUID}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), "The user already exists in the target organization", resp.Error.Error.String())
}

// Ambiguous user + cross-OU NOT allowed → fail immediately without searching.
func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_AmbiguousUser_CrossOUNotAllowed_Fails() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{"sub": "user-sub-123"},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeProperties: map[string]interface{}{},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeAmbiguousEntity, "ambiguous", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrAmbiguousUserIdentity.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

// Ambiguous user + cross-OU allowed + SearchEntities returns error
func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_AmbiguousUser_SearchError_ReturnsServerError() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{"sub": "user-sub-123"},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeProperties: map[string]interface{}{
			common.NodePropertyAllowCrossOUProvisioning: true,
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeAmbiguousEntity, "ambiguous", ""))
	suite.mockEntityProvider.On("SearchEntities", attrs).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "search failed", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
}

// Non-ambiguous system error + cross-OU allowed → fail immediately, no search attempted.
func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_SystemError_NoSearchAttempted() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs:  map[string]string{"sub": "user-sub-123"},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeProperties: map[string]interface{}{
			common.NodePropertyAllowCrossOUProvisioning: true,
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "db error", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrFailedToIdentifyUser.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "SearchEntities", mock.Anything)
}
