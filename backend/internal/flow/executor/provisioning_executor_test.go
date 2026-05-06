/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	i18ncore "github.com/asgardeo/thunder/internal/system/i18n/core"

	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidermgr "github.com/asgardeo/thunder/internal/authnprovider/manager"
	"github.com/asgardeo/thunder/internal/entityprovider"
	"github.com/asgardeo/thunder/internal/entitytype/model"
	"github.com/asgardeo/thunder/internal/flow/common"
	"github.com/asgardeo/thunder/internal/flow/core"
	"github.com/asgardeo/thunder/internal/group"
	"github.com/asgardeo/thunder/internal/role"
	"github.com/asgardeo/thunder/internal/system/error/serviceerror"
	"github.com/asgardeo/thunder/tests/mocks/authnprovider/managermock"
	"github.com/asgardeo/thunder/tests/mocks/entityprovidermock"
	"github.com/asgardeo/thunder/tests/mocks/entitytypemock"
	"github.com/asgardeo/thunder/tests/mocks/flow/coremock"
	"github.com/asgardeo/thunder/tests/mocks/groupmock"
	"github.com/asgardeo/thunder/tests/mocks/rolemock"
)

const (
	testUserType            = "INTERNAL"
	testNewUserID           = "user-new"
	methodGetRequiredInputs = "GetRequiredInputs"
)

type ProvisioningExecutorTestSuite struct {
	suite.Suite
	mockGroupService      *groupmock.GroupServiceInterfaceMock
	mockRoleService       *rolemock.RoleServiceInterfaceMock
	mockFlowFactory       *coremock.FlowFactoryInterfaceMock
	mockEntityProvider    *entityprovidermock.EntityProviderInterfaceMock
	mockEntityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	mockAuthnProvider     *managermock.AuthnProviderManagerInterfaceMock
	executor              *provisioningExecutor
}

func TestProvisioningExecutorSuite(t *testing.T) {
	suite.Run(t, new(ProvisioningExecutorTestSuite))
}

func (suite *ProvisioningExecutorTestSuite) SetupTest() {
	suite.mockGroupService = groupmock.NewGroupServiceInterfaceMock(suite.T())
	suite.mockRoleService = rolemock.NewRoleServiceInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerInterfaceMock(suite.T())

	// Mock the embedded identifying executor first
	identifyingMock := suite.createMockIdentifyingExecutor()
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameIdentifying, common.ExecutorTypeUtility,
		mock.Anything, mock.Anything).Return(identifyingMock).Maybe()

	mockExec := suite.createMockProvisioningExecutor()
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameProvisioning, common.ExecutorTypeRegistration,
		[]common.Input{}, []common.Input{}).Return(mockExec)

	suite.executor = newProvisioningExecutor(suite.mockFlowFactory,
		suite.mockGroupService, suite.mockRoleService, suite.mockEntityProvider,
		suite.mockEntityTypeService, suite.mockAuthnProvider)
}

// expectSchemaForProvisioning sets up the schema service mocks for Execute tests.
// GetRequiredNonCredentialAttributes returns empty (no schema prompting needed).
// GetNonCredentialAttributes returns a broad required-attr set covering all Execute test contexts;
// only attrs that actually have values in the test context will be collected.
// GetCredentialAttributes returns ["password"] as the default credential attribute.
func (suite *ProvisioningExecutorTestSuite) expectSchemaForProvisioning() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{}, nil).Maybe()
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: "email", Required: true},
			{Attribute: "sub", Required: true},
		}, nil).Maybe()
	suite.mockEntityTypeService.On("GetCredentialAttributes", mock.Anything, mock.Anything, testUserType).
		Return([]string{"password"}, nil).Maybe()
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

// makeAuthUserWithRuntimeAttrs creates an AuthUser with runtime attributes for testing.
func makeAuthUserWithRuntimeAttrs(attrs map[string]interface{}) authnprovidermgr.AuthUser {
	type authResultProxy struct {
		RuntimeAttributes map[string]interface{} `json:"runtimeAttributes,omitempty"`
	}
	type authUserProxy struct {
		AuthHistory []authResultProxy `json:"authHistory"`
	}
	raw, _ := json.Marshal(authUserProxy{
		AuthHistory: []authResultProxy{{RuntimeAttributes: attrs}},
	})
	var authUser authnprovidermgr.AuthUser
	_ = json.Unmarshal(raw, &authUser)
	return authUser
}

// makeAuthenticatedAuthUser creates a fully authenticated AuthUser with the given user details.
func makeAuthenticatedAuthUser(userID string) authnprovidermgr.AuthUser {
	type authResultProxy struct {
		IsVerified bool `json:"isVerified"`
	}
	type providerUserResultProxy struct {
		UserID           string `json:"userId"`
		UserType         string `json:"userType"`
		OUID             string `json:"ouId"`
		IsValuesIncluded bool   `json:"isValuesIncluded"`
	}
	type authUserProxy struct {
		AuthHistory []authResultProxy         `json:"authHistory"`
		UserHistory []providerUserResultProxy `json:"userHistory"`
		UserState   string                    `json:"userState"`
	}
	raw, _ := json.Marshal(authUserProxy{
		AuthHistory: []authResultProxy{
			{IsVerified: true},
		},
		UserHistory: []providerUserResultProxy{
			{UserID: userID, UserType: testUserType, OUID: testOUID, IsValuesIncluded: true},
		},
		UserState: "exists",
	})
	var authUser authnprovidermgr.AuthUser
	_ = json.Unmarshal(raw, &authUser)
	return authUser
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
	attrs := map[string]interface{}{"username": "newuser", "email": "new@example.com"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "newuser",
			"email":    "new@example.com",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{
			{Identifier: "username", Type: "string", Required: true},
			{Identifier: "email", Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			"assignGroup": "test-group-id",
			"assignRole":  "test-role-id",
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username": "newuser",
		"email":    "new@example.com",
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
	suite.mockRoleService.On("AddAssignments", mock.Anything, "test-role-id",
		mock.MatchedBy(func(assignments []role.RoleAssignment) bool {
			return len(assignments) == 1 &&
				assignments[0].ID == testNewUserID &&
				assignments[0].Type == role.AssigneeTypeUser
		})).Return(nil)

	authenticatedUser := makeAuthenticatedAuthUser(testNewUserID)
	suite.mockAuthnProvider.On("AuthenticateResolvedUser", mock.Anything, createdUser, mock.Anything).
		Return(authenticatedUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), testNewUserID, resp.AuthUser.GetUserID())
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockGroupService.AssertExpectations(suite.T())
	suite.mockRoleService.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_UserAlreadyExists() {
	suite.expectSchemaForProvisioning()
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "existinguser",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	userID := "user-existing"
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username": "existinguser",
	}).Return(&userID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "User already exists")
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
	assert.Contains(suite.T(), resp.FailureReason, "Failed to create user")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_AttributesFromAuthUser() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		AuthUser:    makeAuthUserWithRuntimeAttrs(map[string]interface{}{"email": "test@example.com"}),
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{{Identifier: "email", Type: "string", Required: true}},
	}

	execResp := &common.ExecutorResponse{
		Inputs:      []common.Input{{Identifier: "email", Type: "string", Required: true}},
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
		UserInputs:  map[string]string{"username": "testuser", "email": "test@example.com"},
		RuntimeData: map[string]string{},
		NodeInputs:  []common.Input{},
	}

	result, _ := suite.executor.getAttributesForProvisioning(ctx)

	assert.Empty(suite.T(), result)
}

// TestGetAttributesForProvisioning_SchemaWhitelist_ExcludesNonSchemaAttrs verifies that the schema
// acts as a whitelist — attributes not in the schema are excluded even if present in context.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_SchemaWhitelist_ExcludesNonSchemaAttrs() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
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

	result, _ := suite.executor.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.NotContains(suite.T(), result, "userID")
	assert.NotContains(suite.T(), result, "code")
	assert.NotContains(suite.T(), result, "nonce")
}

// TestGetAttributesForProvisioning_RequiredAttrsFromMultipleSources verifies that required schema
// attributes are resolved from UserInputs, AuthUser runtime attributes, and RuntimeData.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_RequiredAttrsFromMultipleSources() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: "email", Required: true},
			{Attribute: "given_name", Required: true},
			{Attribute: "phone", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{"username": "testuser"},
		AuthUser: makeAuthUserWithRuntimeAttrs(map[string]interface{}{
			"email":       "authenticated@example.com",
			"given_name":  "Test",
			"family_name": "User",
		}),
		RuntimeData: map[string]string{userTypeKey: testUserType, "phone": "+1234567890"},
		NodeInputs:  []common.Input{},
	}

	result, _ := suite.executor.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.Equal(suite.T(), "authenticated@example.com", result["email"])
	assert.Equal(suite.T(), "Test", result["given_name"])
	assert.Equal(suite.T(), "+1234567890", result["phone"])
}

// TestGetAttributesForProvisioning_ContextPriority verifies priority: UserInputs > RuntimeData > AuthUser.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_ContextPriority() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "email", Required: true},
			{Attribute: "name", Required: true},
			{Attribute: "phone", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			"email": "userinput@example.com",
		},
		AuthUser: makeAuthUserWithRuntimeAttrs(map[string]interface{}{
			"email": "authenticated@example.com",
			"name":  "Authenticated Name",
		}),
		RuntimeData: map[string]string{
			userTypeKey: testUserType,
			"phone":     "+1234567890",
		},
		NodeInputs: []common.Input{},
	}

	result, _ := suite.executor.getAttributesForProvisioning(ctx)

	// UserInputs wins for 'email'
	assert.Equal(suite.T(), "userinput@example.com", result["email"])
	// AuthUser provides 'name' (not in UserInputs or RuntimeData)
	assert.Equal(suite.T(), "Authenticated Name", result["name"])
	// RuntimeData provides 'phone' (not in other sources)
	assert.Equal(suite.T(), "+1234567890", result["phone"])
}

// TestGetAttributesForProvisioning_AllAttrsCollectedWhenNoNodeInputs verifies that when
// node inputs are empty, all schema attrs with available values are collected (both required and optional).
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_AllAttrsCollectedWhenNoNodeInputs() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "email", Required: true},
			{Attribute: "phone", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			"username": "testuser",
		},
		AuthUser: makeAuthUserWithRuntimeAttrs(map[string]interface{}{
			"email":      "authenticated@example.com",
			"given_name": "Test",
		}),
		RuntimeData: map[string]string{
			userTypeKey: testUserType,
			"phone":     "+1234567890",
		},
		NodeInputs: []common.Input{},
	}

	result, _ := suite.executor.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "authenticated@example.com", result["email"])
	assert.Equal(suite.T(), "+1234567890", result["phone"],
		"optional attr with a value must be collected when node inputs are empty")
}

// TestGetAttributesForProvisioning_OptionalAttrCollectedWhenInNodeInputs verifies that an optional
// schema attr is collected when it is explicitly listed in node inputs.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_OptionalAttrCollectedWhenInNodeInputs() {
	nodeInputs := []common.Input{
		{Identifier: "email", Type: "EMAIL_INPUT", Required: true},
		{Identifier: "phone", Type: "TEXT_INPUT", Required: false},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "email", Required: true},
			{Attribute: "phone", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			"email": "user@example.com",
			"phone": "+1234567890",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
	}

	result, _ := exec.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "user@example.com", result["email"])
	assert.Equal(suite.T(), "+1234567890", result["phone"],
		"optional attr in node inputs must be collected")
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
		suite.mockGroupService, suite.mockRoleService, suite.mockEntityProvider,
		suite.mockEntityTypeService, suite.mockAuthnProvider)
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_FilteredPath_RequiredAttrFromUserInputs() {
	nodeInputs := []common.Input{
		{Identifier: "username", Type: "TEXT_INPUT", Required: true},
		{Identifier: "email", Type: "EMAIL_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: "email", Required: true},
			{Attribute: "mobileNumber", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			"email":        "test@example.com",
			"username":     "testuser",
			"mobileNumber": "0771234567",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
	}

	result, _ := exec.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.Equal(suite.T(), "test@example.com", result["email"])
	assert.Equal(suite.T(), "0771234567", result["mobileNumber"],
		"required schema attr from UserInputs must be included even though it is not in node inputs")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_FilteredPath_RequiredAttrFromAuthnAttrs() {
	nodeInputs := []common.Input{
		{Identifier: "username", Type: "TEXT_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: "email", Required: true},
			{Attribute: "given_name", Required: true},
			{Attribute: "mobileNumber", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs:  map[string]string{"username": "testuser"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
		AuthUser: makeAuthUserWithRuntimeAttrs(map[string]interface{}{
			"email":        "federated@example.com",
			"given_name":   "Test",
			"mobileNumber": "0779876543",
		}),
	}

	result, _ := exec.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.Equal(suite.T(), "federated@example.com", result["email"])
	assert.Equal(suite.T(), "Test", result["given_name"])
	assert.Equal(suite.T(), "0779876543", result["mobileNumber"])
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_FilteredPath_UserInputTakesPriority() {
	nodeInputs := []common.Input{
		{Identifier: "email", Type: "EMAIL_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "email", Required: true},
			{Attribute: "username", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{"email": "userinput@example.com"},
		AuthUser: makeAuthUserWithRuntimeAttrs(map[string]interface{}{
			"email":    "authenticated@example.com",
			"username": "federateduser",
		}),
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
	}

	result, _ := exec.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "userinput@example.com", result["email"],
		"UserInputs must win over AuthUser for the same key")
	assert.Equal(suite.T(), "federateduser", result["username"],
		"required schema attr from AuthUser must still be included when not in UserInputs")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_SkipProvisioning_UserAlreadyExists() {
	userID := "existing-user-123"
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "existinguser",
		},
		AuthUser: makeAuthenticatedAuthUser(userID),
		RuntimeData: map[string]string{
			common.RuntimeKeySkipProvisioning: dataValueTrue,
			userTypeKey:                       testUserType,
		},
		NodeInputs: []common.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), userID, resp.RuntimeData[userAttributeUserID])
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity")
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "CreateEntity")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_SkipProvisioning_NoAuthUser_ReturnsFailure() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "existinguser"},
		RuntimeData: map[string]string{
			common.RuntimeKeySkipProvisioning: dataValueTrue,
			userTypeKey:                       testUserType,
		},
		NodeInputs: []common.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "no existing user found", resp.FailureReason)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity")
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "CreateEntity")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_SkipProvisioning_ProceedsNormally() {
	suite.expectSchemaForProvisioning()
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "newuser",
			"email":    "new@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeySkipProvisioning: "false",
			ouIDKey:                           testOUID,
			userTypeKey:                       testUserType,
		},
		NodeInputs: []common.Input{
			{Identifier: "username", Type: "string", Required: true},
			{Identifier: "email", Type: "string", Required: true},
		},
		// No NodeProperties - should skip group/role assignment
	}

	attrs := map[string]interface{}{
		"username": "newuser",
		"email":    "new@example.com",
	}
	attrsJSON, _ := json.Marshal(attrs)

	createdUser := &entityprovider.Entity{
		ID:         testNewUserID,
		OUID:       testOUID,
		Type:       testUserType,
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("IdentifyEntity", attrs).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))
	suite.mockEntityProvider.On("CreateEntity", mock.MatchedBy(func(u *entityprovider.Entity) bool {
		return u.OUID == testOUID && u.Type == testUserType
	}), mock.Anything).Return(createdUser, nil)

	// No group/role assignment mocks - assignments should be skipped

	authenticatedUser := makeAuthenticatedAuthUser(testNewUserID)
	suite.mockAuthnProvider.On("AuthenticateResolvedUser", mock.Anything, createdUser, mock.Anything).
		Return(authenticatedUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), testNewUserID, resp.AuthUser.GetUserID())
	// userAutoProvisioned flag is not set in registration flows
	assert.Equal(suite.T(), testNewUserID, resp.AuthUser.GetUserID())
	suite.mockEntityProvider.AssertExpectations(suite.T())

	// Verify no group/role methods were called
	suite.mockGroupService.AssertNotCalled(suite.T(), "GetGroup")
	suite.mockRoleService.AssertNotCalled(suite.T(), "AddAssignments")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_UserEligibleForProvisioning() {
	suite.expectSchemaForProvisioning()
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username": "provisioneduser",
			"email":    "provisioned@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyUserEligibleForProvisioning: dataValueTrue,
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{
			{Identifier: "username", Type: "string", Required: true},
			{Identifier: "email", Type: "string", Required: true},
		},
	}

	attrs := map[string]interface{}{
		"username": "provisioneduser",
		"email":    "provisioned@example.com",
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

	authenticatedUser := makeAuthenticatedAuthUser("user-provisioned")
	suite.mockAuthnProvider.On("AuthenticateResolvedUser", mock.Anything, createdUser, mock.Anything).
		Return(authenticatedUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "user-provisioned", resp.AuthUser.GetUserID())
	assert.Equal(suite.T(), dataValueTrue, resp.RuntimeData[common.RuntimeKeyUserAutoProvisioned])
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_UserAutoProvisionedFlag_SetAfterCreation() {
	suite.expectSchemaForProvisioning()
	attrs := map[string]interface{}{"username": "newuser", "email": "new@example.com"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username": "newuser",
			"email":    "new@example.com",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
			common.RuntimeKeyUserEligibleForProvisioning: dataValueTrue,
		},
		NodeInputs: []common.Input{
			{Identifier: "username", Type: "string", Required: true},
			{Identifier: "email", Type: "string", Required: true},
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

	authenticatedUser := makeAuthenticatedAuthUser(testNewUserID)
	suite.mockAuthnProvider.On("AuthenticateResolvedUser", mock.Anything, createdUser, mock.Anything).
		Return(authenticatedUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), dataValueTrue, resp.RuntimeData[common.RuntimeKeyUserAutoProvisioned],
		"userAutoProvisioned flag should be set to true after successful provisioning")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *ProvisioningExecutorTestSuite) TestAppendCredentialAttributes() {
	tests := []struct {
		name            string
		schemaCredAttrs []string
		schemaErr       *serviceerror.ServiceError
		nodeInputs      []common.Input
		userInputs      map[string]string
		runtimeData     map[string]string
		expectedAttrs   map[string]interface{}
		expectError     bool
	}{
		{
			name:            "PasswordFromUserInputs",
			schemaCredAttrs: []string{"password"},
			nodeInputs:      []common.Input{},
			userInputs:      map[string]string{"username": "testuser", "password": "secure123"},
			runtimeData:     map[string]string{userTypeKey: testUserType},
			expectedAttrs:   map[string]interface{}{"username": "testuser", "password": "secure123"},
		},
		{
			name:            "PasswordFromRuntimeData",
			schemaCredAttrs: []string{"password"},
			nodeInputs:      []common.Input{},
			userInputs:      map[string]string{"username": "testuser"},
			runtimeData:     map[string]string{userTypeKey: testUserType, "password": "runtime-pass"},
			expectedAttrs:   map[string]interface{}{"username": "testuser", "password": "runtime-pass"},
		},
		{
			name:            "NoValueForCredentialAttr_NotAdded",
			schemaCredAttrs: []string{"password"},
			nodeInputs:      []common.Input{},
			userInputs:      map[string]string{"username": "testuser"},
			runtimeData:     map[string]string{userTypeKey: testUserType},
			expectedAttrs:   map[string]interface{}{"username": "testuser"},
		},
		{
			name:            "MultipleCredentialAttrs_NoNodeInputs_AllAdded",
			schemaCredAttrs: []string{"password", "pin"},
			nodeInputs:      []common.Input{},
			userInputs:      map[string]string{"password": "pass123", "pin": "1234"},
			runtimeData:     map[string]string{userTypeKey: testUserType},
			expectedAttrs:   map[string]interface{}{"username": "testuser", "password": "pass123", "pin": "1234"},
		},
		{
			name:            "MultipleCredentialAttrs_NodeInputsFilter_OnlyDeclaredAdded",
			schemaCredAttrs: []string{"password", "pin"},
			nodeInputs: []common.Input{
				{Identifier: "password", Type: common.InputTypePassword, Required: true},
			},
			userInputs:    map[string]string{"password": "pass123", "pin": "1234"},
			runtimeData:   map[string]string{userTypeKey: testUserType},
			expectedAttrs: map[string]interface{}{"username": "testuser", "password": "pass123"},
		},
		{
			name:            "SchemaReturnsNoCredentials_NothingAdded",
			schemaCredAttrs: []string{},
			nodeInputs:      []common.Input{},
			userInputs:      map[string]string{"password": "pass123"},
			runtimeData:     map[string]string{userTypeKey: testUserType},
			expectedAttrs:   map[string]interface{}{"username": "testuser"},
		},
		{
			name:        "SchemaServiceError_ReturnsError",
			schemaErr:   &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "service error"}},
			nodeInputs:  []common.Input{},
			userInputs:  map[string]string{},
			runtimeData: map[string]string{userTypeKey: testUserType},
			expectError: true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			mockSvc := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
			if tt.schemaErr != nil {
				mockSvc.On("GetCredentialAttributes", mock.Anything, mock.Anything, testUserType).
					Return(nil, tt.schemaErr).Once()
			} else {
				mockSvc.On("GetCredentialAttributes", mock.Anything, mock.Anything, testUserType).
					Return(tt.schemaCredAttrs, nil).Once()
			}

			exec := &provisioningExecutor{
				ExecutorInterface:            suite.executor.ExecutorInterface,
				identifyingExecutorInterface: suite.executor.identifyingExecutorInterface,
				entityProvider:               suite.executor.entityProvider,
				groupService:                 suite.executor.groupService,
				roleService:                  suite.executor.roleService,
				entityTypeService:            mockSvc,
				logger:                       suite.executor.logger,
			}

			ctx := &core.NodeContext{
				UserInputs:  tt.userInputs,
				RuntimeData: tt.runtimeData,
				NodeInputs:  tt.nodeInputs,
			}

			attributes := map[string]interface{}{"username": "testuser"}
			err := exec.appendCredentialAttributes(ctx, &attributes)

			if tt.expectError {
				assert.Error(suite.T(), err)
			} else {
				assert.NoError(suite.T(), err)
				assert.Equal(suite.T(), tt.expectedAttrs, attributes)
			}
		})
	}
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_RegistrationFlow_SkipProvisioningWithExistingUser() {
	userID := "existing-user-id"
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "existinguser",
		},
		AuthUser: makeAuthenticatedAuthUser(userID),
		RuntimeData: map[string]string{
			common.RuntimeKeySkipProvisioning: dataValueTrue,
			userTypeKey:                       testUserType,
		},
		NodeInputs: []common.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), userID, resp.RuntimeData[userAttributeUserID])
	assert.Empty(suite.T(), resp.FailureReason)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity")
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "CreateEntity")
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
	assert.Equal(suite.T(), "Failed to create user", resp.FailureReason)
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
			expectedFailReason: "Failed to create user",
		},
		{
			name:               "CreatedUserIsNil",
			createdUser:        nil,
			createUserError:    nil,
			expectedFailReason: "Something went wrong while creating the user",
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
			expectedFailReason: "Something went wrong while creating the user",
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
			assert.Equal(suite.T(), tt.expectedFailReason, resp.FailureReason)
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
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			"email":     "user@example.com",
			"username":  "testuser",
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{
			{Identifier: "email", Type: "string", Required: true},
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
	suite.mockRoleService.On("AddAssignments", mock.Anything, "test-role-id", mock.Anything).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "Failed to assign groups and roles")
	assert.Contains(suite.T(), resp.FailureReason, "group")

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
	suite.mockRoleService.On("AddAssignments", mock.Anything, "test-role-id", mock.Anything).
		Return(&serviceerror.ServiceError{
			Error: i18ncore.I18nMessage{Key: "error.test.role_not_found", DefaultValue: "Role not found"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "Failed to assign groups and roles", resp.FailureReason)

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
	suite.mockRoleService.On("AddAssignments", mock.Anything, "test-role-id", mock.Anything).
		Return(&serviceerror.ServiceError{
			Error: i18ncore.I18nMessage{Key: "error.test.role_not_found", DefaultValue: "Role not found"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "Failed to assign groups and roles")
	assert.Contains(suite.T(), resp.FailureReason, "role")

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

	suite.mockRoleService.On("AddAssignments", mock.Anything, "test-role-id", mock.Anything).Return(nil)

	authenticatedUser := makeAuthenticatedAuthUser(testNewUserID)
	suite.mockAuthnProvider.On("AuthenticateResolvedUser", mock.Anything, createdUser, mock.Anything).
		Return(authenticatedUser, nil)

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
	suite.mockRoleService.On("AddAssignments", mock.Anything, "test-role-id", mock.Anything).Return(nil)

	authenticatedUser := makeAuthenticatedAuthUser("user-provisioned")
	suite.mockAuthnProvider.On("AuthenticateResolvedUser", mock.Anything, createdUser, mock.Anything).
		Return(authenticatedUser, nil)

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
	attrs := map[string]interface{}{"username": "newuser", "email": "new@example.com"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"username": "newuser",
			"email":    "new@example.com",
		},
		RuntimeData: map[string]string{
			ouIDKey:     testOUID,
			userTypeKey: testUserType,
		},
		NodeInputs: []common.Input{
			{Identifier: "username", Type: "string", Required: true},
			{Identifier: "email", Type: "string", Required: true},
		},
		NodeProperties: map[string]interface{}{
			"assignGroup": "test-group-id",
			"assignRole":  "test-role-id",
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username": "newuser",
		"email":    "new@example.com",
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
	suite.mockRoleService.On("AddAssignments", mock.Anything, "test-role-id",
		mock.MatchedBy(func(assignments []role.RoleAssignment) bool {
			return len(assignments) == 1 &&
				assignments[0].ID == testNewUserID &&
				assignments[0].Type == role.AssigneeTypeUser
		})).Return(nil)

	authenticatedUser := makeAuthenticatedAuthUser(testNewUserID)
	suite.mockAuthnProvider.On("AuthenticateResolvedUser", mock.Anything, createdUser, mock.Anything).
		Return(authenticatedUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), testNewUserID, resp.AuthUser.GetUserID())

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

	authenticatedUser := makeAuthenticatedAuthUser(testNewUserID)
	suite.mockAuthnProvider.On("AuthenticateResolvedUser", mock.Anything, createdUser, mock.Anything).
		Return(authenticatedUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), testNewUserID, resp.RuntimeData[userAttributeUserID])
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
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), "User already exists", resp.FailureReason)
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
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), "User already exists in the target organization", resp.FailureReason)
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
	assert.Equal(suite.T(), "Target OU is not set for cross-OU provisioning", resp.FailureReason)
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
			assert.Equal(t, tt.expectedReason, resp.FailureReason, tt.message)
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
	assert.Equal(suite.T(), common.ExecFailure, resp.Status,
		"Authentication flow should return ExecFailure (not UserInputRequired) when user already exists")
	assert.Equal(suite.T(), "User already exists", resp.FailureReason)
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
	assert.Equal(suite.T(), "User already exists", resp.FailureReason)
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
	assert.Equal(suite.T(), common.ExecFailure, resp.Status,
		"Authentication flow should return ExecFailure (not UserInputRequired) when user exists in target OU")
	assert.Equal(suite.T(), "User already exists in the target organization", resp.FailureReason)
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
	assert.Equal(suite.T(), "User already exists in the target organization", resp.FailureReason)
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
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{{Attribute: "email", DisplayName: "Email"}}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"email": "user@example.com"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
	assert.Nil(suite.T(), execResp.ForwardedData)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaAttrSatisfiedByRuntimeData() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{{Attribute: "email", DisplayName: ""}}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType, "email": "user@example.com"},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaAttrSatisfiedByAuthnAttrs() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{
			{Attribute: "email", DisplayName: "Email"},
			{Attribute: "firstName", DisplayName: "First Name"},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		AuthUser: makeAuthUserWithRuntimeAttrs(map[string]interface{}{
			"email":     "user@example.com",
			"firstName": "Test",
		}),
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaAttrMissing_AppendedToInputsAndForwardedData() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{
			{Attribute: "email", DisplayName: "Email Address", Required: true},
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

	emailInput, ok := inputMap["email"]
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
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "email", DisplayName: "Email", Required: true},
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
	assert.True(suite.T(), inputMap["email"].Required, "required attr must be marked required")
	assert.False(suite.T(), inputMap["nickname"].Required,
		"optional attr must be marked not-required so the UI does not force the user to fill it")
}

// TestHasRequiredInputs_IncludeOptionalTrue_SkipsOptionalAlreadyPresented verifies that when
// includeOptional=true an optional attr recorded as already presented in RuntimeData
// is not re-prompted, even if the user left it empty.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalTrue_SkipsOptionalAlreadyPresented() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "email", DisplayName: "Email", Required: true},
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"email": "user@example.com"},
		RuntimeData: map[string]string{
			userTypeKey: testUserType,
			// nickname was presented in the previous iteration and the user left it blank.
			common.RuntimeKeyPresentedOptionalAttrs: "nickname",
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

// TestHasRequiredInputs_IncludeOptionalTrue_StoresPresentedOptionals verifies that optional attrs
// included in the prompt batch are written to RuntimeData for tracking across iterations.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalTrue_StoresPresentedOptionals() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "email", DisplayName: "Email", Required: true},
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

	stored := execResp.RuntimeData[common.RuntimeKeyPresentedOptionalAttrs]
	assert.Contains(suite.T(), stored, "nickname",
		"presented optional attrs must be written to RuntimeData for subsequent iterations")
	assert.NotContains(suite.T(), stored, "email",
		"required attrs must not be written to the presented-optionals tracking key")
}

// TestHasRequiredInputs_IncludeOptionalTrue_RequiredBeforeOptional verifies that required missing
// attrs always appear before optional ones in the prompted list.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalTrue_RequiredBeforeOptional() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
			{Attribute: "email", DisplayName: "Email", Required: true},
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
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{{Attribute: "email", DisplayName: "Email"}}, nil).Once()

	// email is already a node-defined input — schema must not create a second copy
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{{Identifier: "email", Type: "string", Required: true}},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result, "node input still missing so overall result is false")
	emailCount := 0
	for _, inp := range execResp.Inputs {
		if inp.Identifier == "email" {
			emailCount++
		}
	}
	assert.Equal(suite.T(), 1, emailCount, "email must appear exactly once, not duplicated by schema")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_MissingNodeInput_SchemaAttrsSatisfied_ReturnsFalse() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{{Attribute: "email", DisplayName: ""}}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"email": "user@example.com"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{{Identifier: "username", Required: true}},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result, "node input username is missing so overall must be false")
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_SchemaServiceError_FallsThrough() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return(nil, &serviceerror.ServiceError{Code: "internal_error"}).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result, "schema service error should not fail the executor")
	assert.Empty(suite.T(), execResp.Inputs)
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_SchemaFilteredNoNodeInputs() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: "email", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			"username":    "testuser",
			"extra_field": "should-not-appear",
		},
		AuthUser: makeAuthUserWithRuntimeAttrs(map[string]interface{}{
			"email": "test@example.com",
		}),
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{},
	}

	result, _ := suite.executor.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "testuser", result["username"])
	assert.Equal(suite.T(), "test@example.com", result["email"])
	assert.NotContains(suite.T(), result, "extra_field",
		"attrs not defined in schema must be excluded when schema is available")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_OptionalAttrCollectedWhenNoNodeInputs() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "email", Required: true},
			{Attribute: "phone", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs:  map[string]string{"email": "user@example.com"},
		RuntimeData: map[string]string{userTypeKey: testUserType, "phone": "+1234567890"},
		NodeInputs:  []common.Input{},
	}

	result, _ := suite.executor.getAttributesForProvisioning(ctx)

	assert.Equal(suite.T(), "user@example.com", result["email"])
	assert.Equal(suite.T(), "+1234567890", result["phone"],
		"optional schema attr with a value must be collected when node inputs are empty")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_SchemaServiceError_ReturnsError() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return(nil, &serviceerror.ServiceError{Code: "internal_error"}).Once()

	ctx := &core.NodeContext{
		UserInputs:  map[string]string{"email": "user@example.com", "username": "testuser"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{},
	}

	result, err := suite.executor.getAttributesForProvisioning(ctx)

	assert.Nil(suite.T(), result, "schema service error must return nil map")
	assert.Error(suite.T(), err, "schema service error must propagate as an error")
}

func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_OptionalAttrSkippedWhenNotInNodeInputs() {
	nodeInputs := []common.Input{{Identifier: "email", Required: true}}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "email", Required: true},
			{Attribute: "phone", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs:  map[string]string{"email": "user@example.com", "phone": "+1234567890"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
	}

	result, err := exec.getAttributesForProvisioning(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "user@example.com", result["email"])
	assert.NotContains(suite.T(), result, "phone",
		"optional attr not in nodeInputSet must be skipped when nodeInputSet is non-empty")
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_MissingNodeInputs_ExecUserInputRequired() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
}

func (suite *ProvisioningExecutorTestSuite) TestExecute_GetAttributesError_ReturnsServerError() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{}, nil).Once()
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
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
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{}, nil).Once()
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
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
	assert.Equal(suite.T(), "No user attributes provided for provisioning", resp.FailureReason)
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
	assert.NotEqual(suite.T(), failureReasonUserNotFound, resp.FailureReason)
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
	suite.mockAuthnProvider.On("AuthenticateResolvedUser", mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &serviceerror.ServiceError{})

	resp, err := suite.executor.Execute(ctx)

	assert.Nil(suite.T(), resp)
	assert.Error(suite.T(), err)
}

func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_NilRuntimeData_IsInitialized() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
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

func (suite *ProvisioningExecutorTestSuite) TestCheckNodeInputs_InputNotSatisfiedByAuthnAttrs() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  []common.Input{{Identifier: "username", Required: true}},
		AuthUser:    makeAuthUserWithRuntimeAttrs(map[string]interface{}{"email": "test@example.com"}),
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Len(suite.T(), execResp.Inputs, 1)
	assert.Equal(suite.T(), "username", execResp.Inputs[0].Identifier)
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

func (suite *ProvisioningExecutorTestSuite) TestFetchSchemaAttributes_NilService_ReturnsNil() {
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

	attrs, err := pe.fetchSchemaAttributes(ctx, pe.logger)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), attrs)
}

func (suite *ProvisioningExecutorTestSuite) TestFetchAllNonCredentialAttributes_NilService_ReturnsNil() {
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

	attrs, err := pe.fetchAllNonCredentialAttributes(ctx)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), attrs)
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

// TestHasRequiredInputs_IncludeOptionalTrue_PromptsOptionals verifies that when
// includeOptional=true, missing optional schema attributes are also requested via prompt.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalTrue_PromptsOptionals() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "email", DisplayName: "Email", Required: true},
			{Attribute: "nickname", DisplayName: "Nickname", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{"email": "user@example.com"},
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
	assert.NotContains(suite.T(), identifiers, "email", "already-satisfied attr must not be re-prompted")
}

// TestHasRequiredInputs_IncludeOptionalFalse_SkipsOptionals verifies the default
// behavior: optional schema attrs are not prompted when includeOptional is absent or false.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_IncludeOptionalFalse_SkipsOptionals() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{
			{Attribute: "email", DisplayName: "Email", Required: true},
		}, nil).Once()

	ctx := &core.NodeContext{
		ExecutionID:    "flow-123",
		FlowType:       common.FlowTypeRegistration,
		UserInputs:     map[string]string{"email": "user@example.com"},
		RuntimeData:    map[string]string{userTypeKey: testUserType},
		NodeProperties: map[string]interface{}{},
	}
	execResp := &common.ExecutorResponse{RuntimeData: make(map[string]string)}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
	assert.Empty(suite.T(), execResp.Inputs)
}

// TestHasRequiredInputs_MaxPerPrompt_LimitsPromptedAttrs verifies that when maxPerPrompt=1,
// only one missing schema attribute is prompted per iteration.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_MaxPerPrompt_LimitsPromptedAttrs() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
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
	assert.Len(suite.T(), execResp.Inputs, 1, "only one input should be prompted per iteration")
	assert.Equal(suite.T(), "firstName", execResp.Inputs[0].Identifier)
}

// TestHasRequiredInputs_MaxPerPrompt_Zero_PromptsAllMissingAttrs verifies that maxPerPrompt=0
// (the default) prompts all missing attributes at once.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_MaxPerPrompt_Zero_PromptsAllMissingAttrs() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
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

// TestGetAttributesForProvisioning_IncludeOptionalTrue_CollectsOptionals verifies that when
// includeOptional=true, optional schema attrs are collected even when node inputs are set.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_IncludeOptionalTrue_CollectsOptionals() {
	nodeInputs := []common.Input{
		{Identifier: "email", Type: "EMAIL_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "email", Required: true},
			{Attribute: "nickname", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			"email":    "user@example.com",
			"nickname": "nick",
		},
		RuntimeData: map[string]string{userTypeKey: testUserType},
		NodeInputs:  nodeInputs,
		NodeProperties: map[string]interface{}{
			propertyKeyDynamicInputsIncludeOptional: true,
		},
	}

	result, err := exec.getAttributesForProvisioning(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "user@example.com", result["email"])
	assert.Equal(suite.T(), "nick", result["nickname"],
		"optional attr must be collected when includeOptional=true")
}

// TestGetAttributesForProvisioning_IncludeOptionalFalse_ExcludesOptionals verifies the default
// behavior: optional attrs not in node inputs are excluded when includeOptional=false.
func (suite *ProvisioningExecutorTestSuite) TestGetAttributesForProvisioning_IncludeOptionalFalse_ExcludesOptionals() {
	nodeInputs := []common.Input{
		{Identifier: "email", Type: "EMAIL_INPUT", Required: true},
	}
	exec := suite.newExecutorWithNodeInputs(nodeInputs)

	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "email", Required: true},
			{Attribute: "nickname", Required: false},
		}, nil).Once()

	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			"email":    "user@example.com",
			"nickname": "nick",
		},
		RuntimeData:    map[string]string{userTypeKey: testUserType},
		NodeInputs:     nodeInputs,
		NodeProperties: map[string]interface{}{},
	}

	result, err := exec.getAttributesForProvisioning(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "user@example.com", result["email"])
	assert.NotContains(suite.T(), result, "nickname",
		"optional attr not in node inputs must be excluded when includeOptional=false")
}

// TestHasRequiredInputs_MaxPerPrompt_Float64_LimitsPromptedAttrs verifies that maxPerPrompt
// supplied as float64 (the type JSON unmarshalling produces) is handled correctly.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_MaxPerPrompt_Float64_LimitsPromptedAttrs() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
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
	assert.Len(suite.T(), execResp.Inputs, 1,
		"float64 maxPerPrompt value (from JSON) must be handled correctly")
}

// TestExecute_AppendCredentialAttributesFails_ReturnsServerError verifies that when
// fetchCredentialAttributes fails (schema service error), Execute propagates it as a server error.
func (suite *ProvisioningExecutorTestSuite) TestExecute_AppendCredentialAttributesFails_ReturnsServerError() {
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
		Return([]model.AttributeInfo{}, nil).Once()
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, false).
		Return([]model.AttributeInfo{
			{Attribute: "username", Required: true},
		}, nil).Once()
	suite.mockEntityTypeService.On("GetCredentialAttributes", mock.Anything, mock.Anything, testUserType).
		Return(nil, &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "schema unavailable"}}).Once()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{ouIDKey: testOUID, userTypeKey: testUserType},
		NodeInputs:  []common.Input{{Identifier: "username", Type: "string", Required: true}},
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{"username": "newuser"}).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.Nil(suite.T(), resp)
	assert.Error(suite.T(), err)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "CreateEntity")
}

// TestFetchCredentialAttributes_NilService_ReturnsNil verifies that when entityTypeService is nil,
// appendCredentialAttributes is a no-op and returns no error.
func (suite *ProvisioningExecutorTestSuite) TestFetchCredentialAttributes_NilService_ReturnsNil() {
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
		UserInputs:  map[string]string{"password": "secret"},
		RuntimeData: map[string]string{userTypeKey: testUserType},
	}

	attrs := map[string]interface{}{"username": "testuser"}
	err := pe.appendCredentialAttributes(ctx, &attrs)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), map[string]interface{}{"username": "testuser"}, attrs)
}

// TestFetchCredentialAttributes_MissingUserType_ReturnsError verifies that when userType is absent
// from runtime data, appendCredentialAttributes propagates the error.
func (suite *ProvisioningExecutorTestSuite) TestFetchCredentialAttributes_MissingUserType_ReturnsError() {
	ctx := &core.NodeContext{
		UserInputs:  map[string]string{"password": "secret"},
		RuntimeData: map[string]string{},
	}

	attrs := map[string]interface{}{"username": "testuser"}
	err := suite.executor.appendCredentialAttributes(ctx, &attrs)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "user type not found")
}

// TestHasRequiredInputs_NoProperties_DefaultBehavior verifies that when no properties are set the
// executor falls back to prompting only required schema attributes, all at once.
func (suite *ProvisioningExecutorTestSuite) TestHasRequiredInputs_NoProperties_DefaultBehavior() {
	// requiredOnly=true: service returns only required attrs (optional ones are filtered by the service).
	suite.mockEntityTypeService.On("GetNonCredentialAttributes", mock.Anything, mock.Anything, testUserType, true).
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
