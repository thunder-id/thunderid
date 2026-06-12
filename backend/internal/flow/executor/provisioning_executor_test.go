/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"

	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
)

const (
	testUserType            = "INTERNAL"
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
	mockEntityTypeService     *entitytypemock.EntityTypeServiceInterfaceMock
	mockAuthnProvider         *managermock.AuthnProviderManagerInterfaceMock
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
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerInterfaceMock(suite.T())
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, authnprovidercm.AuthenticatedClaims{},
			(*serviceerror.ServiceError)(nil)).Maybe()

	// Mock the embedded identifying executor first
	identifyingMock := suite.createMockIdentifyingExecutor()
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameIdentifying, common.ExecutorTypeUtility,
		mock.Anything, mock.Anything).Return(identifyingMock).Maybe()

	mockExec := suite.createMockProvisioningExecutor()
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameProvisioning, common.ExecutorTypeRegistration,
		[]common.Input{}, []common.Input{}).Return(mockExec)

	suite.executor = newProvisioningExecutor(suite.mockFlowFactory,
		suite.mockGroupService, suite.mockRoleService, suite.mockRoleAssignmentService, suite.mockEntityProvider,
		suite.mockEntityTypeService, suite.mockAuthnProvider)
}

// expectSchemaForProvisioning sets up the schema service mocks for Execute tests.
// The (true,true) mock covers both HasRequiredInputs and getAttributesForProvisioning.
// This version does NOT include credentials - use expectSchemaWithCredentials if needed.
func (suite *ProvisioningExecutorTestSuite) expectSchemaForProvisioning() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: false},
			{Attribute: attributeEmail, Required: false},
			{Attribute: "sub", Required: false},
		}, nil).Maybe()
}

func (suite *ProvisioningExecutorTestSuite) createMockIdentifyingExecutor() core.ExecutorInterface {
	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return(ExecutorNameIdentifying).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeUtility).Maybe()
	mockExec.On("GetDefaultInputs").Return([]common.Input{}).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{}).Maybe()
	return mockExec
}

func (suite *ProvisioningExecutorTestSuite) createMockProvisioningExecutor() core.ExecutorInterface {
	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return(ExecutorNameProvisioning).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeRegistration).Maybe()
	mockExec.On("GetDefaultInputs").Return([]common.Input{}).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{}).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *core.NodeContext, execResp *common.ExecutorResponse) bool {
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
	mockExec.On("GetInputs", mock.Anything).Return([]common.Input{}).Maybe()
	mockExec.On(methodGetRequiredInputs, mock.Anything).Return([]common.Input{}).Maybe()
	return mockExec
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_NonRegistrationFlow() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_Success() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "newuser", attributeEmail: "new@example.com"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username":     "newuser",
			attributeEmail: "new@example.com",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{
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

	createdUser := &entityprovider.Entity{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("CreateEntity", mock.MatchedBy(func(u *entityprovider.Entity) bool {
		return u.OUID == testOUID && u.Type == testUserType
	}), mock.Anything).Return(createdUser, nil)

	// Mock group assignment
	suite.mockGroupService.On("AddGroupMembers", mock.Anything, "test-group-id",
		mock.MatchedBy(func(members []group.Member) bool {
			return len(members) == 1 &&
				members[0].ID == testNewUserID &&
				members[0].Type == group.MemberTypeUser
		})).Return(nil, nil)

	// Mock role assignment
	suite.mockRoleAssignmentService.On("AddAssignments", mock.Anything, "test-role-id",
		mock.MatchedBy(func(assignments []role.RoleAssignment) bool {
			return len(assignments) == 1 &&
				assignments[0].ID == testNewUserID &&
				assignments[0].Type == role.AssigneeTypeUser
		})).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockGroupService.AssertExpectations(suite.T())
	suite.mockRoleService.AssertExpectations(suite.T())
	suite.mockRoleAssignmentService.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_UserAlreadyExists() {
	suite.expectSchemaForProvisioning()
	nodeInputs := []common.Input{{Identifier: "username", Type: "string", Required: true}}
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "existinguser",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
	}

	// Override GetRequiredInputs to return node inputs so the retry path is exercised
	provMock := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	var filteredCalls []*mock.Call
	for _, call := range provMock.ExpectedCalls {
		if call.Method != methodGetRequiredInputs {
			filteredCalls = append(filteredCalls, call)
		}
	}
	provMock.ExpectedCalls = filteredCalls
	provMock.On(methodGetRequiredInputs, mock.Anything).Return(nodeInputs).Maybe()

	userID := "user-existing"
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username": "existinguser",
	}).Return(&userID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Contains(suite.T(), resp.Error.Error.DefaultValue, "User already exists")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_NoUserAttributes() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{},
		NodeInputs:  []common.Input{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CreateUserFails() {
	suite.expectSchemaForProvisioning()
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "newuser",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
	suite.mockEntityProvider.On("CreateEntity", mock.Anything, mock.Anything).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "creation failed", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrProvisioningFailed.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_AttributesFromAuthUser() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		NodeInputs:  []common.Input{{Identifier: attributeEmail, Type: "string", Required: true}},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
}

// TestGetAttributesForProvisioning_SchemaEmpty_ReturnsEmpty verifies that when the schema
// is unavailable (no userTypeKey → getUserType returns ""), an empty map is returned.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_SchemaEmpty_ReturnsEmpty() {
	ctx := &core.NodeContext{
		UserInputs:  map[string]string{"username": "testuser", attributeEmail: "test@example.com"},
		RuntimeData: map[string]string{},
		NodeInputs:  []common.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx)

	assert.Empty(suite.T(), result)
}

// TestGetAttributesForProvisioning_SchemaWhitelist_ExcludesNonSchemaAttrs verifies that the schema
// acts as a whitelist — attributes not in the schema are excluded even if present in context.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_SchemaWhitelist_ExcludesNonSchemaAttrs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{{Attribute: "username", Required: true}}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			"username": "testuser",
			"userID":   "user-123",
			"code":     "auth-code",
			"nonce":    "test-nonce",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.NotContains(suite.T(), result, "userID")
	assert.NotContains(suite.T(), result, "code")
	assert.NotContains(suite.T(), result, "nonce")
}

// TestGetAttributesForProvisioning_RequiredAttrsFromMultipleSources verifies that required schema
// attributes are resolved from UserInputs and RuntimeData.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_RequiredAttrsFromMultipleSources() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: attributeEmail, Required: true},
			{Attribute: "given_name", Required: true},
			{Attribute: "phone", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{"username": "testuser"},
		RuntimeData: map[string]string{
			userTypeKey:    testUserType,
			attributeEmail: "auth@example.com",
			"given_name":   "Test",
			"phone":        "+1234567890",
		},
		NodeInputs: []common.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.Equal(suite.T(), "auth@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "Test", result["given_name"])
	assert.Equal(suite.T(), "+1234567890", result["phone"])
}

// TestGetAttributesForProvisioning_ContextPriority verifies priority: UserInputs wins over
// AuthenticatedUser.Attributes which wins over RuntimeData (first non-empty source wins).
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_ContextPriority() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "name", Required: true},
			{Attribute: "phone", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{attributeEmail: "userinput@example.com"},
		RuntimeData: map[string]string{
			userTypeKey:    testUserType,
			attributeEmail: "runtime@example.com",
			"name":         "Authn Name",
			"phone":        "+1234567890",
		},
		NodeInputs: []common.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx)

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
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "phone", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{attributeEmail: "user@example.com"},
		RuntimeData: map[string]string{
			userTypeKey: testUserType,
			"phone":     "+1234567890",
		},
		NodeInputs: []common.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "user@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "+1234567890", result["phone"],
		"optional attr with a value must be collected when node inputs are empty")
}

// TestGetAttributesForProvisioning_OptionalAttrCollectedWhenInNodeInputs verifies that an optional
// schema attr is collected when it is explicitly listed in node inputs.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_OptionalAttrCollectedWhenInNodeInputs() {
	nodeInputs := []common.Input{
		{Identifier: attributeEmail, Type: "EMAIL_INPUT", Required: true},
		{Identifier: "phone", Type: "TEXT_INPUT", Required: false},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "phone", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			attributeEmail: "user@example.com",
			"phone":        "+1234567890",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
	}

	result, _, _ := exec.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "user@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "+1234567890", result["phone"],
		"optional attr in node inputs must be collected")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_EmptyCredentialFallsBackToRuntime() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, Required: true, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			attributePassword: "",
		},
		RuntimeData: map[string]string{
			userTypeKey:       testUserType,
			attributePassword: "runtime-secret",
		},
		NodeInputs: []common.Input{},
	}

	_, credentialAttrs, err := suite.executor.getAttributesForProvisioning(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "runtime-secret", credentialAttrs[attributePassword])
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_CredentialFromUserInputs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, Required: true, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			attributePassword: "input-secret",
		},
		RuntimeData: map[string]string{
			userTypeKey:       testUserType,
			attributePassword: "runtime-secret",
		},
		NodeInputs: []common.Input{},
	}

	_, credentialAttrs, err := suite.executor.getAttributesForProvisioning(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "input-secret", credentialAttrs[attributePassword])
}

// newExecutorWithNodeInputs creates a provisioningExecutor whose embedded ExecutorInterface
// returns the given inputs from GetRequiredInputs.
func (suite *ProvisioningExecutorTestSuite) newExecutorWithNodeInputs(inputs []common.Input) *provisioningExecutor {
	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetRequiredInputs", mock.Anything).Return(inputs).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true).Maybe()

	mockFlowFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFlowFactory.On("CreateExecutor", ExecutorNameProvisioning, common.ExecutorTypeRegistration,
		mock.Anything, mock.Anything).Return(mockExec)

	identifyingMock := suite.createMockIdentifyingExecutor()
	mockFlowFactory.On("CreateExecutor", ExecutorNameIdentifying, common.ExecutorTypeUtility,
		mock.Anything, mock.Anything).Return(identifyingMock).Maybe()

	return newProvisioningExecutor(mockFlowFactory,
		suite.mockGroupService, suite.mockRoleService, suite.mockRoleAssignmentService, suite.mockEntityProvider,
		suite.mockEntityTypeService, suite.mockAuthnProvider)
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_FilteredPath_RequiredAttrFromUserInputs() {
	nodeInputs := []common.Input{
		{Identifier: "username", Type: "TEXT_INPUT", Required: true},
		{Identifier: attributeEmail, Type: "EMAIL_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: attributeEmail, Required: true},
			{Attribute: "mobileNumber", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			"username":     "testuser",
			attributeEmail: "test@example.com",
			"mobileNumber": "0771234567",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
	}

	result, _, _ := exec.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.Equal(suite.T(), "test@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "0771234567", result["mobileNumber"],
		"required schema attr from UserInputs must be included even though it is not in node inputs")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_FilteredPath_RequiredAttrFromAuthnAttrs() {
	nodeInputs := []common.Input{
		{Identifier: "username", Type: "TEXT_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: attributeEmail, Required: true},
			{Attribute: "given_name", Required: true},
			{Attribute: "mobileNumber", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{"username": "testuser"},
		RuntimeData: map[string]string{
			userTypeKey:    testUserType,
			attributeEmail: "federated@example.com",
			"given_name":   "Test",
			"mobileNumber": "0779876543",
		},
		NodeInputs: nodeInputs,
	}

	result, _, _ := exec.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.Equal(suite.T(), "federated@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "Test", result["given_name"])
	assert.Equal(suite.T(), "0779876543", result["mobileNumber"])
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_FilteredPath_UserInputTakesPriority() {
	nodeInputs := []common.Input{
		{Identifier: attributeEmail, Type: "EMAIL_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "username", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{attributeEmail: "userinput@example.com"},
		RuntimeData: map[string]string{
			userTypeKey: testUserType,
			"username":  "federateduser",
		},
		NodeInputs: nodeInputs,
	}

	result, _, _ := exec.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "userinput@example.com", result[attributeEmail],
		"UserInputs must win over RuntimeData for the same key")
	assert.Equal(suite.T(), "federateduser", result["username"],
		"required schema attr from RuntimeData must still be included")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_UserEligibleForProvisioning() {
	suite.expectSchemaForProvisioning()
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username":     "provisioneduser",
			attributeEmail: "provisioned@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyUserEligibleForProvisioning: dataValueTrue,
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{
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

	createdUser := &entityprovider.Entity{
		ID:         "user-provisioned",
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
	suite.mockEntityProvider.On("CreateEntity", mock.MatchedBy(func(u *entityprovider.Entity) bool {
		return u.OUID == testOUID && u.Type == testUserType
	}), mock.Anything).Return(createdUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), dataValueTrue, resp.RuntimeData[common.RuntimeKeyUserAutoProvisioned])
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_UserAutoProvisionedFlag_SetAfterCreation() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "newuser", attributeEmail: "new@example.com"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username":     "newuser",
			attributeEmail: "new@example.com",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
			common.RuntimeKeyUserEligibleForProvisioning: dataValueTrue,
		},
		NodeInputs: []common.Input{
			{Identifier: "username", Type: "string", Required: true},
			{Identifier: attributeEmail, Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptionalCredentials: false,
		},
	}

	createdUser := &entityprovider.Entity{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
	suite.mockEntityProvider.On("CreateEntity", mock.Anything, mock.Anything).Return(createdUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), dataValueTrue, resp.RuntimeData[common.RuntimeKeyUserAutoProvisioned],
		"userAutoProvisioned flag should be set to true after successful provisioning")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_MissingInputs_MissingOUID() {
	suite.expectSchemaForProvisioning()
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{"username": "newuser"}).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrProvisioningFailed.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "CreateEntity")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_MissingInputs_MissingUserType() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{ouIDKey: testOUID},
		NodeInputs:  []common.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity")
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "CreateEntity")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CreateUserFailures() {
	suite.expectSchemaForProvisioning()
	tests := []struct {
		name               string
		createdUser        *entityprovider.Entity
		createUserError    *entityprovider.EntityProviderError
		expectedFailReason string
	}{
		{
			name:        "ServiceReturnsError",
			createdUser: nil,
			createUserError: entityprovider.NewEntityProviderError(
				entityprovider.ErrorCodeSystemError, "Database error", ""),
			expectedFailReason: ErrProvisioningFailed.Error.DefaultValue,
		},
		{
			name:               "CreatedUserIsNil",
			createdUser:        nil,
			createUserError:    nil,
			expectedFailReason: ErrProvisioningFailed.Error.DefaultValue,
		},
		{
			name: "CreatedUserHasEmptyID",
			createdUser: &entityprovider.Entity{
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

			ctx := &core.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    common.FlowTypeRegistration,
				UserInputs: map[string]string{
					"username": "newuser",
				},
				RuntimeData: map[string]string{
					ouIDKey:     testOUID,
					userTypeKey: testUserType,
				},
				NodeInputs: []common.Input{
					{Identifier: "username", Type: "string", Required: true},
				},
			}

			attrs := map[string]interface{}{
				"username": "newuser",
			}
			suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
				entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
			suite.mockEntityProvider.On("CreateEntity", mock.Anything, mock.Anything).
				Return(tt.createdUser, tt.createUserError)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), resp)
			assert.Equal(suite.T(), common.ExecFailure, resp.Status)
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
			ctx := &core.NodeContext{
				RuntimeData: tt.runtimeData,
				UserInputs:  tt.userInputs,
			}

			ouID := suite.executor.getOUID(ctx)

			assert.Equal(suite.T(), tt.expected, ouID)
		})
	}
}

func (suite *ProvisioningExecutorTestSuite) TestGetUserType() {
	tests := []struct {
		name        string
		runtimeData map[string]string
		expected    string
	}{
		{
			name: "Found",
			runtimeData: map[string]string{
				userTypeKey: "CUSTOM_USER_TYPE",
			},
			expected: "CUSTOM_USER_TYPE",
		},
		{
			name:        "NotFound",
			runtimeData: map[string]string{},
			expected:    "",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			ctx := &core.NodeContext{
				RuntimeData: tt.runtimeData,
			}

			userType := suite.executor.getUserType(ctx)

			assert.Equal(suite.T(), tt.expected, userType)
		})
	}
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_AllAttributesInRuntimeData() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			attributeEmail: "user@example.com",
			"username":     "testuser",
			userTypeKey:    testUserType,
		},
		NodeInputs: []common.Input{
			{Identifier: attributeEmail, Type: "string", Required: true},
			{Identifier: "username", Type: "string", Required: true},
		},
	}

	execResp := &common.ExecutorResponse{
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

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "newuser",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			"assignGroup": "test-group-id",
			"assignRole":  "test-role-id",
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	createdUser := &entityprovider.Entity{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("CreateEntity", mock.Anything, mock.Anything).Return(createdUser, nil)

	// Mock group assignment fails (e.g., group doesn't exist)
	suite.mockGroupService.On("AddGroupMembers", mock.Anything, "test-group-id", mock.Anything).
		Return(nil, &serviceerror.ServiceError{
			Error: i18ncore.I18nMessage{Key: "error.test.group_not_found", DefaultValue: "Group not found"},
		})

	// Role assignment should still be attempted
	suite.mockRoleAssignmentService.On("AddAssignments", mock.Anything, "test-role-id", mock.Anything).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.Error.Error.DefaultValue, "Failed to assign groups and roles")
	assert.Contains(suite.T(), resp.Error.Error.DefaultValue, "group")

	// Verify role assignment WAS attempted despite group failure
	suite.mockRoleService.AssertExpectations(suite.T())
}

// Test both group and role assignment failure - provisioning should fail with combined error
func (suite *ProvisioningExecutorTestSuite) TestExecute_Failure_BothGroupAndRoleAssignmentFail() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "newuser"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "newuser",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			"assignGroup": "test-group-id",
			"assignRole":  "test-role-id",
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	createdUser := &entityprovider.Entity{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("CreateEntity", mock.Anything, mock.Anything).Return(createdUser, nil)

	// Mock group assignment fails
	suite.mockGroupService.On("AddGroupMembers", mock.Anything, "test-group-id", mock.Anything).
		Return(nil, &serviceerror.ServiceError{
			Error: i18ncore.I18nMessage{Key: "error.test.group_not_found", DefaultValue: "Group not found"},
		})

	// Mock role assignment also fails
	suite.mockRoleAssignmentService.On("AddAssignments", mock.Anything, "test-role-id", mock.Anything).
		Return(&serviceerror.ServiceError{
			Error: i18ncore.I18nMessage{Key: "error.test.role_not_found", DefaultValue: "Role not found"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrProvisioningAssignmentFailed.Error.DefaultValue, resp.Error.Error.DefaultValue)

	// Verify both services were called (new behavior: try both even if one fails)
	suite.mockGroupService.AssertExpectations(suite.T())
	suite.mockRoleService.AssertExpectations(suite.T())
}

// Test role assignment failure - provisioning should fail, but group assignment succeeds
func (suite *ProvisioningExecutorTestSuite) TestExecute_Failure_RoleAssignmentFails() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "newuser"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "newuser",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			"assignGroup": "test-group-id",
			"assignRole":  "test-role-id",
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	createdUser := &entityprovider.Entity{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("CreateEntity", mock.Anything, mock.Anything).Return(createdUser, nil)

	// Group assignment succeeds
	suite.mockGroupService.On("AddGroupMembers", mock.Anything, "test-group-id", mock.Anything).
		Return(nil, nil)

	// Role assignment fails (e.g., role doesn't exist)
	suite.mockRoleAssignmentService.On("AddAssignments", mock.Anything, "test-role-id", mock.Anything).
		Return(&serviceerror.ServiceError{
			Error: i18ncore.I18nMessage{Key: "error.test.role_not_found", DefaultValue: "Role not found"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.Error.Error.DefaultValue, "Failed to assign groups and roles")
	assert.Contains(suite.T(), resp.Error.Error.DefaultValue, "role")

	// Verify both group and role services were called
	suite.mockGroupService.AssertExpectations(suite.T())
	suite.mockRoleService.AssertExpectations(suite.T())
}

// Test group with existing members - user should be appended
func (suite *ProvisioningExecutorTestSuite) TestExecute_GroupWithExistingMembers() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "newuser"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "newuser",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			"assignGroup": "test-group-id",
			"assignRole":  "test-role-id",
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	createdUser := &entityprovider.Entity{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("CreateEntity", mock.Anything, mock.Anything).Return(createdUser, nil)

	// Mock group assignment - AddGroupMembers only adds the new user, not existing members
	suite.mockGroupService.On("AddGroupMembers", mock.Anything, "test-group-id",
		mock.MatchedBy(func(members []group.Member) bool {
			return len(members) == 1 &&
				members[0].ID == testNewUserID &&
				members[0].Type == group.MemberTypeUser
		})).Return(nil, nil)

	suite.mockRoleAssignmentService.On("AddAssignments", mock.Anything, "test-role-id", mock.Anything).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockGroupService.AssertExpectations(suite.T())
}

// Test authentication flow with auto-provisioning still assigns groups/roles
func (suite *ProvisioningExecutorTestSuite) TestExecute_AuthFlow_AutoProvisioning_AssignsGroupsAndRoles() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "provisioneduser"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username": "provisioneduser",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyUserEligibleForProvisioning: dataValueTrue,
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{
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

	createdUser := &entityprovider.Entity{
		ID:         "user-provisioned",
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("CreateEntity", mock.Anything, mock.Anything).Return(createdUser, nil)

	// Mock successful group and role assignment
	suite.mockGroupService.On("AddGroupMembers", mock.Anything, "test-group-id", mock.Anything).
		Return(nil, nil)
	suite.mockRoleAssignmentService.On("AddAssignments", mock.Anything, "test-role-id", mock.Anything).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
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

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username":     "newuser",
			attributeEmail: "new@example.com",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{
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

	createdUser := &entityprovider.Entity{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("CreateEntity", mock.MatchedBy(func(u *entityprovider.Entity) bool {
		return u.OUID == testOUID && u.Type == testUserType
	}), mock.Anything).Return(createdUser, nil)

	// Mock group assignment
	suite.mockGroupService.On("AddGroupMembers", mock.Anything, "test-group-id",
		mock.MatchedBy(func(members []group.Member) bool {
			return len(members) == 1 &&
				members[0].ID == testNewUserID &&
				members[0].Type == group.MemberTypeUser
		})).Return(nil, nil)

	// Mock role assignment
	suite.mockRoleAssignmentService.On("AddAssignments", mock.Anything, "test-role-id",
		mock.MatchedBy(func(assignments []role.RoleAssignment) bool {
			return len(assignments) == 1 &&
				assignments[0].ID == testNewUserID &&
				assignments[0].Type == role.AssigneeTypeUser
		})).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)

	// Verify all mocks were called
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockGroupService.AssertExpectations(suite.T())
	suite.mockRoleService.AssertExpectations(suite.T())
}

// Cross-OU provisioning tests

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_Success() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}
	attrsJSON, _ := json.Marshal(attrs)

	existingUserID := testExistingUserID
	existingUser := &entityprovider.Entity{
		ID:   existingUserID,
		OUID: "ou-source",
	}

	createdUser := &entityprovider.Entity{
		ID:         testNewUserID,
		Type:       testUserType,
		OUID:       testOUID,
		Attributes: attrsJSON,
	}

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
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
	suite.mockEntityProvider.On("CreateEntity", mock.MatchedBy(func(u *entityprovider.Entity) bool {
		return u.OUID == testOUID
	}), mock.Anything).Return(createdUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_NotEnabled_Fails() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
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
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrUserAlreadyExists.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_SameOU_Fails() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID
	existingUser := &entityprovider.Entity{
		ID:   existingUserID,
		OUID: testOUID, // same as target
	}

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
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
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrUserAlreadyExistsInTargetOU.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_NoTargetOU_Fails() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
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
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
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
			expectedReason: "User already exists",
			message:        "Should return inputs for retry when user already exists in registration flow",
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			suite.SetupTest()
			suite.expectSchemaForProvisioning()

			nodeInputs := []common.Input{
				{Identifier: "username", Type: "string", Required: true},
			}
			ctx := &core.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    common.FlowTypeRegistration,
				UserInputs:  map[string]string{"username": "existinguser"},
				NodeInputs:  nodeInputs,
				RuntimeData: map[string]string{userTypeKey: testUserType},
			}

			// Override GetRequiredInputs to return node inputs for this test
			provMock := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
			var filteredCalls []*mock.Call
			for _, call := range provMock.ExpectedCalls {
				if call.Method != methodGetRequiredInputs {
					filteredCalls = append(filteredCalls, call)
				}
			}
			provMock.ExpectedCalls = filteredCalls
			provMock.On(methodGetRequiredInputs, mock.Anything).Return(nodeInputs).Maybe()

			existingID := tt.existingUserID
			suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
				"username": "existinguser",
			}).Return(&existingID, nil)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, common.ExecUserInputRequired, resp.Status)
			assert.Equal(t, tt.expectedReason, resp.Error.Error.DefaultValue, tt.message)
			assert.NotEmpty(t, resp.Inputs, "Inputs should be re-populated for retry")
			suite.mockEntityProvider.AssertExpectations(t)
		})
	}
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_NotEnabled_AuthnFlow_ReturnsFailure() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
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
	assert.Equal(suite.T(), common.ExecComplete, resp.Status,
		"Authentication flow should skip provisioning and return ExecComplete when user already exists")
	assert.Nil(suite.T(), resp.Error, "No error should be set when skipping provisioning in authentication flow")
	assert.Empty(suite.T(), resp.Inputs, "Inputs should not be populated for authentication flows")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_NotEnabled_RegistrationFlow_PopulatesInputs() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID

	nodeInputs := []common.Input{
		{Identifier: "sub", Type: "string", Required: true},
	}
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
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
	provMock := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	var filteredCalls []*mock.Call
	for _, call := range provMock.ExpectedCalls {
		if call.Method != methodGetRequiredInputs {
			filteredCalls = append(filteredCalls, call)
		}
	}
	provMock.ExpectedCalls = filteredCalls
	provMock.On(methodGetRequiredInputs, mock.Anything).Return(nodeInputs).Maybe()

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(&existingUserID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), ErrUserAlreadyExists.Error.DefaultValue, resp.Error.Error.DefaultValue)
	assert.NotEmpty(suite.T(), resp.Inputs, "Inputs should be populated so the user can correct their input")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_SameOU_AuthnFlow_ReturnsFailure() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID
	existingUser := &entityprovider.Entity{
		ID:   existingUserID,
		OUID: testOUID, // same as target
	}

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
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
	assert.Equal(suite.T(), common.ExecComplete, resp.Status,
		"Authentication flow should skip provisioning and return ExecComplete when user exists in target OU")
	assert.Nil(suite.T(), resp.Error, "No error should be set when skipping provisioning in authentication flow")
	assert.Empty(suite.T(), resp.Inputs, "Inputs should not be populated for authentication flows")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_SameOU_RegistrationFlow_PopulatesInputs() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID
	existingUser := &entityprovider.Entity{
		ID:   existingUserID,
		OUID: testOUID, // same as target
	}

	nodeInputs := []common.Input{
		{Identifier: "sub", Type: "string", Required: true},
	}
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
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
	provMock := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	var filteredCalls []*mock.Call
	for _, call := range provMock.ExpectedCalls {
		if call.Method != methodGetRequiredInputs {
			filteredCalls = append(filteredCalls, call)
		}
	}
	provMock.ExpectedCalls = filteredCalls
	provMock.On(methodGetRequiredInputs, mock.Anything).Return(nodeInputs).Maybe()

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(&existingUserID, nil)
	suite.mockEntityProvider.On("GetEntity", existingUserID).Return(existingUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), ErrUserAlreadyExistsInTargetOU.Error.DefaultValue, resp.Error.Error.DefaultValue)
	assert.NotEmpty(suite.T(), resp.Inputs, "Inputs should be populated so the user can correct their input")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_GetUserError() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	existingUserID := testExistingUserID

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
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
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{{Attribute: attributeEmail, DisplayName: "Email"}}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{attributeEmail: "user@example.com"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
	assert.Nil(suite.T(), execResp.ForwardedData)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaAttrSatisfiedByRuntimeData() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{{Attribute: attributeEmail, DisplayName: ""}}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType, attributeEmail: "user@example.com"},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaAttrSatisfiedByAuthnAttrs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email"},
			{Attribute: "firstName", DisplayName: "First Name"},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaAttrMissing_AppendedToInputsAndForwardedData() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email Address", Required: true},
			{Attribute: "firstName", DisplayName: "", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Len(suite.T(), execResp.Inputs, 2)

	inputMap := make(map[string]common.Input, len(execResp.Inputs))
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
	fwdInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs].([]common.Input)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), fwdInputs, 2)
}

// TestHasRequiredInputs_IncludeOptionalTrue_OptionalRenderedAsNotRequired verifies that when
// includeOptional=true, optional schema attrs are forwarded with Required=false.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalTrue_OptionalRenderedAsNotRequired() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	inputMap := make(map[string]common.Input, len(execResp.Inputs))
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
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
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
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result,
		"must not block when all required attrs are satisfied and optional was already presented")
	assert.Empty(suite.T(), execResp.Inputs,
		"nickname must not be re-prompted once it appears in the presented list")
}

// TestHasRequiredInputs_IncludeOptionalTrue_DoesNotStorePresentedOptionals verifies that
// presented-input tracking is now owned by the flow engine, not provisioning.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalTrue_DoesNotStorePresentedOptionals() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	suite.executor.HasRequiredInputs(ctx, execResp)

	_, ok := execResp.RuntimeData[common.RuntimeKeyPresentedOptionalInputs]
	assert.False(suite.T(), ok, "provisioning should not write presented-input tracking directly")
}

// TestHasRequiredInputs_IncludeOptionalTrue_RequiredBeforeOptional verifies that required missing
// attrs always appear before optional ones in the prompted list.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalTrue_RequiredBeforeOptional() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "phone", DisplayName: "Phone", Required: false},
			{Attribute: "firstName", DisplayName: "First Name", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

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
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{{Attribute: attributeEmail, DisplayName: "Email"}}, nil).Once()

	// email is already a node-defined input — schema must not create a second copy
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{{Identifier: attributeEmail, Type: "string", Required: true}},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

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
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{{Attribute: attributeEmail, DisplayName: ""}}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{attributeEmail: "user@example.com"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{{Identifier: "username", Required: true}},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result, "schema-absent node input must be ignored; all schema attrs are satisfied")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaServiceError_ReturnsFailure() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return(nil, &serviceerror.ServiceError{Code: "internal_error"}).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result, "schema service error must fail the executor")
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Empty(suite.T(), execResp.Inputs)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_RequiredCredential_PromptedAsPassword() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Len(suite.T(), execResp.Inputs, 1)
	assert.Equal(suite.T(), attributePassword, execResp.Inputs[0].Identifier)
	assert.Equal(suite.T(), common.InputTypePassword, execResp.Inputs[0].Type)
	assert.True(suite.T(), execResp.Inputs[0].Required)

	fwdInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs].([]common.Input)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), fwdInputs, 1)
	assert.Equal(suite.T(), common.InputTypePassword, fwdInputs[0].Type)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_RequiredCredentialSatisfied_ReturnsTrue() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{attributePassword: "secret"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_RequiredCredentialInAuthnAttrs_StillPrompted() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Len(suite.T(), execResp.Inputs, 1)
	assert.Equal(suite.T(), attributePassword, execResp.Inputs[0].Identifier)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_AllCredentials_PromptedByDefault() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		// No includeOptionalCredentials property — defaults to false, only required credentials prompted
		NodeInputs: []common.Input{},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	passwordFound := false
	pinFound := false
	for _, inp := range execResp.Inputs {
		if inp.Identifier == attributePassword {
			passwordFound = true
			assert.True(suite.T(), inp.Required)
			assert.Equal(suite.T(), common.InputTypePassword, inp.Type)
		}
		if inp.Identifier == attributePin {
			pinFound = true
			assert.False(suite.T(), inp.Required)
			assert.Equal(suite.T(), common.InputTypePassword, inp.Type)
		}
	}
	assert.True(suite.T(), passwordFound, "required credential must be prompted by default")
	assert.False(suite.T(), pinFound, "optional credential must NOT be prompted by default")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalCreds_False_OnlyRequired() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptionalCredentials: false,
		},
		NodeInputs: []common.Input{},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

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
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
		NodeInputs: []common.Input{},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

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
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		// Node marks pin as required even though schema says optional.
		NodeInputs: []common.Input{{Identifier: attributePin, Type: common.InputTypePassword, Required: true}},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

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
			assert.Equal(suite.T(), common.InputTypePassword, inp.Type)
		}
	}
	assert.True(suite.T(), pinFound)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_AlreadyPromptedOptionalCredential_Skipped() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			userTypeKey:                              testUserType,
			common.RuntimeKeyPresentedOptionalInputs: attributePin,
		},
		NodeInputs: []common.Input{{Identifier: attributePin, Type: common.InputTypePassword, Required: false}},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

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
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptionalCredentials: true,
		},
		NodeInputs: []common.Input{},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	passwordFound := false
	pinFound := false
	for _, inp := range execResp.Inputs {
		if inp.Identifier == attributePassword {
			passwordFound = true
			assert.True(suite.T(), inp.Required)
			assert.Equal(suite.T(), common.InputTypePassword, inp.Type)
		}
		if inp.Identifier == attributePin {
			pinFound = true
			assert.False(suite.T(), inp.Required)
			assert.Equal(suite.T(), common.InputTypePassword, inp.Type)
		}
	}
	assert.True(suite.T(), passwordFound, "includeOptionalCredentials=true must prompt required credentials")
	assert.True(suite.T(), pinFound, "includeOptionalCredentials=true must prompt optional credentials")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalCredentials_AlreadyPrompted_Skipped() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			userTypeKey:                              testUserType,
			common.RuntimeKeyPresentedOptionalInputs: attributePin,
		},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptionalCredentials: true,
		},
		NodeInputs: []common.Input{},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result, "already-prompted optional credential should not block")
	for _, inp := range execResp.Inputs {
		assert.NotEqual(suite.T(), attributePin, inp.Identifier,
			"already-prompted optional credential must not be re-prompted")
	}
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalCreds_WithRequired() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptionalCredentials: false,
		},
		NodeInputs: []common.Input{},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

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
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional:            false,
			propertyKeyDynamicInputsIncludeOptionalCredentials: true,
		},
		NodeInputs: []common.Input{},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

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
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		// Node tries to mark schema-required credential as optional — schema wins.
		NodeInputs: []common.Input{{Identifier: attributePassword, Type: common.InputTypePassword, Required: false}},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

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
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: attributePin, DisplayName: "PIN", Required: false, Credential: true},
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
		// pin is optional credential listed in node inputs so it will be prompted.
		NodeInputs: []common.Input{{Identifier: attributePin, Type: common.InputTypePassword, Required: false}},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

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
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "phone", DisplayName: "Phone", Required: true},
			{Attribute: "firstName", DisplayName: "First Name", Required: true},
			{Attribute: attributePassword, DisplayName: "Password", Required: true, Credential: true},
			{Attribute: attributePin, DisplayName: "PIN", Required: true, Credential: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyMaxDynamicInputsPerPrompt: 1,
		},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	suite.executor.HasRequiredInputs(ctx, execResp)

	credCount, ncCount := 0, 0
	for _, inp := range execResp.Inputs {
		if inp.Type == common.InputTypePassword {
			credCount++
		} else {
			ncCount++
		}
	}
	assert.Equal(suite.T(), 2, credCount, "full missing set should retain all credential inputs")
	assert.Equal(suite.T(), 3, ncCount, "full missing set should retain all non-credential inputs")

	fwdInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs].([]common.Input)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), fwdInputs, 1, "prompt batch should be capped by maxPerPrompt")
	assert.NotEqual(suite.T(), common.InputTypePassword, fwdInputs[0].Type,
		"first forwarded input should be a non-credential (non-credentials come first)")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_SchemaFilteredNoNodeInputs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: attributeEmail, Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			"username":    "testuser",
			"extra_field": "should-not-appear",
		},
		RuntimeData: map[string]string{
			userTypeKey:    testUserType,
			attributeEmail: "test@example.com",
		},
		NodeInputs: []common.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.Equal(suite.T(), "test@example.com", result[attributeEmail])
	assert.NotContains(suite.T(), result, "extra_field",
		"attrs not defined in schema must be excluded when schema is available")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_OptionalAttrCollectedWhenNoNodeInputs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "phone", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs:  map[string]string{attributeEmail: "user@example.com"},
		RuntimeData: map[string]string{userTypeKey: testUserType, "phone": "+1234567890"},
		NodeInputs:  []common.Input{},
	}

	result, _, _ := suite.executor.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "user@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "+1234567890", result["phone"],
		"optional schema attr with a value must be collected when node inputs are empty")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_SchemaServiceError_ReturnsError() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return(nil, &serviceerror.ServiceError{Code: "internal_error"}).Once()

	ctx := &core.NodeContext{
		UserInputs:  map[string]string{attributeEmail: "user@example.com", "username": "testuser"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{},
	}

	result, _, err := suite.executor.getAttributesForProvisioning(ctx)

	assert.Nil(suite.T(), result, "schema service error must return nil map")
	assert.Error(suite.T(), err, "schema service error must propagate as an error")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_OptionalAttrCollectedWithoutNodeInput() {
	nodeInputs := []common.Input{{Identifier: attributeEmail, Required: true}}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "phone", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs:  map[string]string{attributeEmail: "user@example.com", "phone": "+1234567890"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
	}

	result, _, err := exec.getAttributesForProvisioning(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "user@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "+1234567890", result["phone"],
		"optional schema attr with a value must be collected even when nodeInputSet is non-empty")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_GetAttributesError_ReturnsServerError() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{}, nil).Once()
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return(nil, &serviceerror.ServiceError{Code: "internal_error"}).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.Nil(suite.T(), resp)
	assert.Error(suite.T(), err)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_EmptySchemaAttrs_NoUserAttributes() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{}, nil).Once()
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrProvisioningUserAttrsMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_IdentifyUser_AmbiguousMatch_ReturnsFailureEarly() {
	suite.expectSchemaForProvisioning()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	suite.mockEntityProvider.On("IdentifyEntity",
		map[string]interface{}{"username": "newuser"}).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeAmbiguousEntity, "ambiguous", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.NotEqual(suite.T(), ErrUserNotFound.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_UnmarshalAttributesError_ReturnsServerError() {
	suite.expectSchemaForProvisioning()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{"username": "newuser"}).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
	suite.mockEntityProvider.On("CreateEntity", mock.Anything, mock.Anything).
		Return(&entityprovider.Entity{
			ID:         testNewUserID,
			OUID:       testOUID,
			Type:       testUserType,
			Attributes: []byte(`invalid json`),
		}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_NilRuntimeData_IsInitialized() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{},
	}
	execResp := &common.ExecutorResponse{RuntimeData: nil}

	suite.executor.HasRequiredInputs(ctx, execResp)

	assert.NotNil(suite.T(), execResp.RuntimeData)
}

func (suite *ProvisioningExecutorTestSuite) TestGetGroupToAssign_NonStringValue_ReturnsEmpty() {
	ctx := &core.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyAssignGroup: 42,
		},
	}

	result := suite.executor.getGroupToAssign(ctx)

	assert.Equal(suite.T(), "", result)
}

func (suite *ProvisioningExecutorTestSuite) TestGetRoleToAssign_NonStringValue_ReturnsEmpty() {
	ctx := &core.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyAssignRole: true,
		},
	}

	result := suite.executor.getRoleToAssign(ctx)

	assert.Equal(suite.T(), "", result)
}

func (suite *ProvisioningExecutorTestSuite) TestFetchSchemaAttributeInfos_NilService_ReturnsNil() {
	pe := &provisioningExecutor{
		ExecutorInterface:            suite.executor.ExecutorInterface,
		identifyingExecutorInterface: suite.executor.identifyingExecutorInterface,
		entityProvider:               suite.executor.entityProvider,
		groupService:                 suite.executor.groupService,
		roleService:                  suite.executor.roleService,
		entityTypeService:            nil,
		logger:                       suite.executor.logger,
	}

	ctx := &core.NodeContext{
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}

	attrs, err := pe.fetchSchemaAttributes(ctx, false, true)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), attrs)
}

func (suite *ProvisioningExecutorTestSuite) TestFetchSchemaAttributeInfos_NonCred_ServiceError_ReturnsError() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, false, true, false).
		Return(nil, &serviceerror.ServiceError{Code: "internal_error"}).Once()

	ctx := &core.NodeContext{
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}

	attrs, err := suite.executor.fetchSchemaAttributes(ctx, false, true)

	assert.Nil(suite.T(), attrs)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to fetch schema attributes for user type")
}

func (suite *ProvisioningExecutorTestSuite) TestFetchSchemaAttributeInfos_Cred_ServiceError_ReturnsError() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, false, false).
		Return(nil, &serviceerror.ServiceError{Code: "internal_error"}).Once()

	ctx := &core.NodeContext{
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}

	attrs, err := suite.executor.fetchSchemaAttributes(ctx, true, false)

	assert.Nil(suite.T(), attrs)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to fetch schema attributes for user type")
}

func (suite *ProvisioningExecutorTestSuite) TestCreateUserInStore_MissingUserType_ReturnsError() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{ouIDKey: testOUID},
	}

	result, err := suite.executor.createUserInStore(ctx, map[string]interface{}{"username": "testuser"})

	assert.Nil(suite.T(), result)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "user type not found")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_MissingUserType_ReturnsFailure() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
}

// TestHasRequiredInputs_IncludeOptionalTrue_PromptsOptionals verifies that when
// includeOptional=true, missing optional schema attributes are also requested via prompt.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalTrue_PromptsOptionals() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{attributeEmail: "user@example.com"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

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
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID:    "flow-123",
		FlowType:       common.FlowTypeRegistration,
		UserInputs:     map[string]string{attributeEmail: "user@example.com"},
		RuntimeData:    map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
}

// TestHasRequiredInputs_NodeOptionalAttr_PromptedWithoutIncludeOptional
// verifies that a schema-optional non-credential attr still prompts when the node explicitly asks
// for it, even if includeOptional is absent or false.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_NodeOptionalAttr_PromptedWithoutIncludeOptional() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, DisplayName: "Email", Required: true},
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{attributeEmail: "user@example.com"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs: []common.Input{
			{Identifier: "nickname", Type: common.InputTypeText, Required: false},
		},
		NodeProperties: map[string]interface{}{},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	require.Len(suite.T(), execResp.Inputs, 1)
	assert.Equal(suite.T(), "nickname", execResp.Inputs[0].Identifier)
	assert.False(suite.T(), execResp.Inputs[0].Required)
	fwdInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs].([]common.Input)
	assert.True(suite.T(), ok)
	require.Len(suite.T(), fwdInputs, 1)
	assert.Equal(suite.T(), "nickname", fwdInputs[0].Identifier)
}

// TestHasRequiredInputs_MaxPerPrompt_LimitsPromptedAttrs verifies that when maxPerPrompt=1,
// only one missing schema attribute is forwarded to the prompt per iteration.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_MaxPerPrompt_LimitsPromptedAttrs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: "firstName", DisplayName: "First Name", Required: true},
			{Attribute: "lastName", DisplayName: "Last Name", Required: true},
			{Attribute: "phone", DisplayName: "Phone", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyMaxDynamicInputsPerPrompt: 1,
		},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Len(suite.T(), execResp.Inputs, 3, "full missing set should be retained on the executor response")
	fwdInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs].([]common.Input)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), fwdInputs, 1, "only one input should be forwarded to the prompt per iteration")
	assert.Equal(suite.T(), "firstName", fwdInputs[0].Identifier)
}

// TestHasRequiredInputs_MaxPerPrompt_Zero_PromptsAllMissingAttrs verifies that maxPerPrompt=0
// (the default) prompts all missing attributes at once.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_MaxPerPrompt_Zero_PromptsAllMissingAttrs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: "firstName", DisplayName: "First Name", Required: true},
			{Attribute: "lastName", DisplayName: "Last Name", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID:    "flow-123",
		FlowType:       common.FlowTypeRegistration,
		UserInputs:     map[string]string{},
		RuntimeData:    map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Len(suite.T(), execResp.Inputs, 2, "all missing inputs should be prompted when maxPerPrompt is not set")
}

// TestGetAttributesForProvisioning_IncludeOptionalTrue_NoEffect verifies that
// includeOptional=true does not alter schema-backed attribute collection.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_IncludeOptionalTrue_NoEffect() {
	nodeInputs := []common.Input{
		{Identifier: attributeEmail, Type: "EMAIL_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "nickname", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
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

	result, _, err := exec.getAttributesForProvisioning(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "user@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "nick", result["nickname"],
		"optional attr with a value must be collected regardless of includeOptional")
}

// TestGetAttributesForProvisioning_IncludeOptionalFalse_CollectsOptionals verifies that
// includeOptional=false does not exclude schema-backed values during collection.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_IncludeOptionalFalse_CollectsOptionals() {
	nodeInputs := []common.Input{
		{Identifier: attributeEmail, Type: "EMAIL_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: attributeEmail, Required: true},
			{Attribute: "nickname", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			attributeEmail: "user@example.com",
			"nickname":     "nick",
		},
		RuntimeData:    map[string]string{userTypeKey: testUserType},
		NodeInputs:     nodeInputs,
		NodeProperties: map[string]interface{}{},
	}

	result, _, err := exec.getAttributesForProvisioning(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "user@example.com", result[attributeEmail])
	assert.Equal(suite.T(), "nick", result["nickname"],
		"optional attr with a value must be collected regardless of includeOptional")
}

// TestHasRequiredInputs_MaxPerPrompt_Float64_LimitsPromptedAttrs verifies that maxPerPrompt
// supplied as float64 (the type JSON unmarshalling produces) is handled correctly for the
// forwarded prompt batch.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_MaxPerPrompt_Float64_LimitsPromptedAttrs() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: "firstName", DisplayName: "First Name", Required: true},
			{Attribute: "lastName", DisplayName: "Last Name", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{
			propertyKeyMaxDynamicInputsPerPrompt: float64(1),
		},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Len(suite.T(), execResp.Inputs, 2,
		"full missing set should be retained on the executor response")
	fwdInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs].([]common.Input)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), fwdInputs, 1,
		"float64 maxPerPrompt value (from JSON) must cap the forwarded prompt batch")
}

// TestExecute_SchemaErrorOnProvisioning_ReturnsServerError verifies that when getAttributesForProvisioning
// fails with a schema service error, Execute propagates it as a server error.
func (suite *ProvisioningExecutorTestSuite) TestExecute_SchemaErrorOnProvisioning_ReturnsServerError() {
	// HasRequiredInputs: username is satisfied so execution proceeds.
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{{Attribute: "username", Required: true}}, nil).Once()
	// getAttributesForProvisioning: schema service fails.
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return(nil, &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "schema unavailable"}}).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{ouIDKey: testOUID, userTypeKey: testUserType},
		NodeInputs:  []common.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.Nil(suite.T(), resp)
	assert.Error(suite.T(), err)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "CreateEntity")
}

// TestHasRequiredInputs_NoProperties_DefaultBehavior verifies that when no properties are set the
// executor falls back to prompting only required schema attributes, all at once.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_NoProperties_DefaultBehavior() {
	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUserType, true, true, false).
		Return([]model.AttributeInfo{
			{Attribute: "firstName", DisplayName: "First Name", Required: true},
			{Attribute: "lastName", DisplayName: "Last Name", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

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

	createdUser := &entityprovider.Entity{
		ID:         testNewUserID,
		Type:       testUserType,
		OUID:       testOUID,
		Attributes: attrsJSON,
	}

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
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
		Return([]*entityprovider.Entity{
			{ID: testExistingUserID, OUID: "ou-toyota"},
			{ID: "other-user-id", OUID: "ou-honda"},
		}, nil)
	suite.mockEntityProvider.On("CreateEntity", mock.MatchedBy(func(u *entityprovider.Entity) bool {
		return u.OUID == testOUID
	}), mock.Anything).Return(createdUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

// Ambiguous user + cross-OU allowed + match found in target OU → fail "already exists in target".
func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_AmbiguousUser_MatchInTargetOU_Fails() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
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
		Return([]*entityprovider.Entity{
			{ID: testExistingUserID, OUID: testOUID},
			{ID: "other-user-id", OUID: "ou-honda"},
		}, nil)
	suite.mockEntityProvider.On("GetEntity", testExistingUserID).
		Return(&entityprovider.Entity{ID: testExistingUserID, OUID: testOUID}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), ErrUserAlreadyExistsInTargetOU.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

// Ambiguous user + cross-OU NOT allowed → fail immediately without searching.
func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_AmbiguousUser_CrossOUNotAllowed_Fails() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
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
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrAmbiguousUserIdentity.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

// Ambiguous user + cross-OU allowed + SearchEntities returns error
func (suite *ProvisioningExecutorTestSuite) TestExecute_CrossOU_AmbiguousUser_SearchError_ReturnsServerError() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"sub": "user-sub-123"}

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
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

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
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
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrFailedToIdentifyUser.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "SearchEntities", mock.Anything)
}
