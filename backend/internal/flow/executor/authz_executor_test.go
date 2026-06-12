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

	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	authzsvc "github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/authzmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

const testExistingUser123ID = "existing-user-123"

// createTestAuthzExecutor creates an authorization executor with mocks for testing
func createTestAuthzExecutor(t *testing.T,
	mockAuthzService *authzmock.AuthorizationServiceInterfaceMock,
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock,
	mockAuthnProvider *managermock.AuthnProviderManagerInterfaceMock) *authorizationExecutor {
	mockFlowFactory := coremock.NewFlowFactoryInterfaceMock(t)

	// Mock the CreateExecutor method to return a base executor
	mockFlowFactory.On("CreateExecutor", ExecutorNameAuthorization, common.ExecutorTypeUtility,
		[]common.Input{}, []common.Input{}).
		Return(createMockExecutor(t, "AuthorizationExecutor", common.ExecutorTypeUtility))

	return newAuthorizationExecutor(mockFlowFactory, mockAuthzService, mockEntityProvider, mockAuthnProvider)
}

// newAuthzAuthenticatedAuthUser creates an AuthUser that returns true for IsAuthenticated().
func newAuthzAuthenticatedAuthUser() authnprovidermgr.AuthUser {
	var authUser authnprovidermgr.AuthUser
	_ = authUser.UnmarshalJSON([]byte(`{"entityReferenceToken":"tok","attributeToken":"tok"}`))
	return authUser
}

// createMockExecutor creates a mock executor for testing purposes
func createMockExecutor(t *testing.T, name string, executorType common.ExecutorType) core.ExecutorInterface {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(name).Maybe()
	mockExec.On("GetType").Return(executorType).Maybe()
	mockExec.On("GetDefaultInputs").Return([]common.Input{}).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{}).Maybe()
	return mockExec
}

func TestNewAuthorizationExecutor(t *testing.T) {
	mockAuthzService := authzmock.NewAuthorizationServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	assert.NotNil(t, executor)
	assert.Equal(t, "AuthorizationExecutor", executor.GetName())
	prerequisites := executor.GetPrerequisites()
	assert.Empty(t, prerequisites)
}

func TestAuthorizationExecutor_Execute_Success(t *testing.T) {
	// Setup
	mockAuthzService := new(authzmock.AuthorizationServiceInterfaceMock)
	mockEntityProvider := new(entityprovidermock.EntityProviderInterfaceMock)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	authUser := newAuthzAuthenticatedAuthUser()
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		FlowType:    common.FlowTypeAuthentication,
		AuthUser:    authUser,
		RuntimeData: map[string]string{
			requestedPermissionsKey: "read:documents write:documents delete:documents",
			"groups":                `["group1", "group2"]`,
		},
	}

	mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(authUser, &authnprovidercm.EntityReference{EntityID: "user123"}, nil)

	mockAuthzService.On("EvaluateAccessBatch",
		mock.Anything,
		mock.MatchedBy(func(req authzsvc.AccessEvaluationsRequest) bool {
			return len(req.Evaluations) == 3 &&
				req.Evaluations[0].Subject.ID == "user123" &&
				len(req.Evaluations[0].Subject.GroupIDs) == 2 &&
				req.Evaluations[0].Subject.GroupIDs[0] == "group1" &&
				req.Evaluations[0].Subject.GroupIDs[1] == "group2" &&
				req.Evaluations[0].ResourceServer.Handle == "" &&
				req.Evaluations[0].Permission.Name == "read:documents" &&
				req.Evaluations[1].Permission.Name == "write:documents" &&
				req.Evaluations[2].Permission.Name == "delete:documents"
		})).Return(&authzsvc.AccessEvaluationsResponse{
		Evaluations: []authzsvc.AccessEvaluationResponse{
			{Decision: true},
			{Decision: true},
			{Decision: false},
		},
	}, nil)

	// Execute
	resp, err := executor.Execute(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, common.ExecComplete, resp.Status)
	assert.Equal(t, "read:documents write:documents", resp.RuntimeData[authorizedPermissionsKey])

	mockAuthzService.AssertExpectations(t)
}

func TestAuthorizationExecutor_Execute_PartialPermissions(t *testing.T) {
	// Setup - user requests multiple permissions but only gets some
	mockAuthzService := new(authzmock.AuthorizationServiceInterfaceMock)
	mockEntityProvider := new(entityprovidermock.EntityProviderInterfaceMock)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	authUser := newAuthzAuthenticatedAuthUser()
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		FlowType:    common.FlowTypeAuthentication,
		AuthUser:    authUser,
		RuntimeData: map[string]string{
			requestedPermissionsKey: "read:documents write:documents delete:documents",
		},
	}

	mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(authUser, &authnprovidercm.EntityReference{EntityID: "user123"}, nil)

	mockEntityProvider.On("GetTransitiveEntityGroups", "user123").Return(
		[]entityprovider.EntityGroup{}, nil)

	// User only has read permission
	mockAuthzService.On("EvaluateAccessBatch", mock.Anything, mock.Anything).Return(
		&authzsvc.AccessEvaluationsResponse{
			Evaluations: []authzsvc.AccessEvaluationResponse{
				{Decision: true},
				{Decision: false},
				{Decision: false},
			},
		}, nil)

	// Execute
	resp, err := executor.Execute(ctx)

	// Assert - should succeed with partial permissions
	assert.NoError(t, err)
	assert.Equal(t, common.ExecComplete, resp.Status)
	assert.Equal(t, "read:documents", resp.RuntimeData[authorizedPermissionsKey])

	mockAuthzService.AssertExpectations(t)
	mockEntityProvider.AssertExpectations(t)
}

func TestAuthorizationExecutor_Execute_NoPermissions(t *testing.T) {
	// Setup - user has no permissions at all
	mockAuthzService := new(authzmock.AuthorizationServiceInterfaceMock)
	mockEntityProvider := new(entityprovidermock.EntityProviderInterfaceMock)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	authUser := newAuthzAuthenticatedAuthUser()
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		FlowType:    common.FlowTypeAuthentication,
		AuthUser:    authUser,
		RuntimeData: map[string]string{
			requestedPermissionsKey: "read:documents write:documents",
		},
	}

	mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(authUser, &authnprovidercm.EntityReference{EntityID: "user123"}, nil)

	mockEntityProvider.On("GetTransitiveEntityGroups", "user123").Return(
		[]entityprovider.EntityGroup{}, nil)

	mockAuthzService.On("EvaluateAccessBatch", mock.Anything, mock.Anything).Return(
		&authzsvc.AccessEvaluationsResponse{
			Evaluations: []authzsvc.AccessEvaluationResponse{
				{Decision: false},
				{Decision: false},
			},
		}, nil)

	// Execute
	resp, err := executor.Execute(ctx)

	// Assert - should succeed with empty permissions
	assert.NoError(t, err)
	assert.Equal(t, common.ExecComplete, resp.Status)
	assert.Equal(t, "", resp.RuntimeData[authorizedPermissionsKey])

	mockAuthzService.AssertExpectations(t)
	mockEntityProvider.AssertExpectations(t)
}

func TestAuthorizationExecutor_Execute_NotAuthenticated(t *testing.T) {
	// Setup - user not authenticated
	mockAuthzService := new(authzmock.AuthorizationServiceInterfaceMock)
	mockEntityProvider := new(entityprovidermock.EntityProviderInterfaceMock)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		FlowType:    common.FlowTypeAuthentication,
		RuntimeData: make(map[string]string),
	}

	// Execute
	resp, err := executor.Execute(ctx)

	// Assert - should FAIL (changed behavior from original design)
	assert.NoError(t, err)
	assert.Equal(t, common.ExecFailure, resp.Status)
	assert.Equal(t, ErrUserNotAuthenticated.Code, resp.Error.Code)

	// Service should NOT be called
	mockAuthzService.AssertNotCalled(t, "EvaluateAccessBatch")
}

func TestAuthorizationExecutor_Execute_ServiceError(t *testing.T) {
	// Setup - service returns error
	mockAuthzService := new(authzmock.AuthorizationServiceInterfaceMock)
	mockEntityProvider := new(entityprovidermock.EntityProviderInterfaceMock)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	authUser := newAuthzAuthenticatedAuthUser()
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		FlowType:    common.FlowTypeAuthentication,
		AuthUser:    authUser,
		RuntimeData: map[string]string{
			requestedPermissionsKey: "read:documents write:documents",
		},
	}

	mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(authUser, &authnprovidercm.EntityReference{EntityID: "user123"}, nil)

	mockEntityProvider.On("GetTransitiveEntityGroups", "user123").Return(
		[]entityprovider.EntityGroup{}, nil)

	mockAuthzService.On("EvaluateAccessBatch", mock.Anything, mock.Anything).Return(
		nil, &serviceerror.ServiceError{
			Error: i18ncore.I18nMessage{Key: "error.test.service_error", DefaultValue: "service error"},
		})

	// Execute
	resp, err := executor.Execute(ctx)

	// Assert - should fail the flow
	assert.NoError(t, err)
	assert.Equal(t, common.ExecFailure, resp.Status)

	mockAuthzService.AssertExpectations(t)
	mockEntityProvider.AssertExpectations(t)
}

func TestAuthorizationExecutor_Execute_GroupExtractionError(t *testing.T) {
	// Setup - user group retrieval fails and execution should fail before authz service call
	mockAuthzService := new(authzmock.AuthorizationServiceInterfaceMock)
	mockEntityProvider := new(entityprovidermock.EntityProviderInterfaceMock)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	authUser := newAuthzAuthenticatedAuthUser()
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		FlowType:    common.FlowTypeAuthentication,
		AuthUser:    authUser,
		RuntimeData: map[string]string{
			requestedPermissionsKey: "read:documents write:documents",
		},
	}

	mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(authUser, &authnprovidercm.EntityReference{EntityID: "user123"}, nil)

	mockEntityProvider.On("GetTransitiveEntityGroups", "user123").Return(
		nil, entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeSystemError,
			"failed to retrieve groups",
			"failed to retrieve groups"))

	// Execute
	resp, err := executor.Execute(ctx)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)

	mockAuthzService.AssertNotCalled(t, "EvaluateAccessBatch")
	mockEntityProvider.AssertExpectations(t)
}

func TestAuthorizationExecutor_Execute_NoRequestedPermissions(t *testing.T) {
	// This test verifies behavior when extractRequestedPermissions returns empty
	// The service should NOT be called, and should return early with ExecComplete

	mockAuthzService := new(authzmock.AuthorizationServiceInterfaceMock)
	mockEntityProvider := new(entityprovidermock.EntityProviderInterfaceMock)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	authUser := newAuthzAuthenticatedAuthUser()
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		FlowType:    common.FlowTypeAuthentication,
		AuthUser:    authUser,
		RuntimeData: make(map[string]string), // No requestedPermissionsKey
	}

	mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(authUser, &authnprovidercm.EntityReference{EntityID: "user123"}, nil)

	// Execute
	resp, err := executor.Execute(ctx)

	// Assert - should return early without calling service
	assert.NoError(t, err)
	assert.Equal(t, common.ExecComplete, resp.Status)
	assert.Empty(t, resp.RuntimeData[authorizedPermissionsKey])

	// Service should NOT be called
	mockAuthzService.AssertNotCalled(t, "EvaluateAccessBatch")
}

func TestAuthorizationExecutor_ExtractGroupIDs_NoGroupsInContext(t *testing.T) {
	mockAuthzService := authzmock.NewAuthorizationServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	ctx := &core.NodeContext{
		RuntimeData: make(map[string]string),
	}

	mockEntityProvider.On("GetTransitiveEntityGroups", "user123").Return(
		[]entityprovider.EntityGroup{}, nil)

	groupIDs, err := executor.extractGroupIDs(ctx, "user123")
	assert.NoError(t, err)
	assert.Empty(t, groupIDs)
}

func TestAuthorizationExecutor_ExtractGroupIDs_FromRuntimeData(t *testing.T) {
	mockAuthzService := authzmock.NewAuthorizationServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	ctx := &core.NodeContext{
		RuntimeData: map[string]string{
			"groups": `["runtime-group1", "runtime-group2"]`,
		},
	}

	groupIDs, err := executor.extractGroupIDs(ctx, "user123")
	assert.NoError(t, err)
	assert.Equal(t, []string{"runtime-group1", "runtime-group2"}, groupIDs)
}

func TestExtractRequestedPermissions(t *testing.T) {
	tests := []struct {
		name        string
		runtimeData map[string]string
		UserInputs  map[string]string
		expected    []string
	}{
		{
			name: "Space-separated permissions",
			runtimeData: map[string]string{
				requestedPermissionsKey: "read:documents write:documents delete:documents",
			},
			expected: []string{"read:documents", "write:documents", "delete:documents"},
		},
		{
			name: "Single permission",
			runtimeData: map[string]string{
				requestedPermissionsKey: "read:documents",
			},
			expected: []string{"read:documents"},
		},
		{
			name:        "No requested permissions",
			runtimeData: map[string]string{},
			expected:    []string{},
		},
		{
			name: "Empty string",
			runtimeData: map[string]string{
				requestedPermissionsKey: "",
			},
			expected: []string{},
		},
		{
			name: "Permissions from User Inputs",
			UserInputs: map[string]string{
				requestedPermissionsKey: "edit:documents share:documents",
			},
			expected: []string{"edit:documents", "share:documents"},
		},
		{
			name: "Permissions Priority to Runtime Data",
			runtimeData: map[string]string{
				requestedPermissionsKey: "view:documents delete:documents",
			},
			UserInputs: map[string]string{
				requestedPermissionsKey: "edit:documents share:documents",
			},
			expected: []string{"view:documents", "delete:documents"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &core.NodeContext{
				FlowType:    common.FlowTypeAuthentication,
				RuntimeData: tt.runtimeData,
				UserInputs:  tt.UserInputs,
			}

			permissions := extractRequestedPermissions(ctx)
			assert.Equal(t, tt.expected, permissions)
		})
	}
}

func TestAuthorizationExecutor_ExtractGroupIDs_WithNoGroups(t *testing.T) {
	mockAuthzService := authzmock.NewAuthorizationServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	ctx := &core.NodeContext{
		RuntimeData: make(map[string]string),
	}

	mockEntityProvider.On("GetTransitiveEntityGroups", "user123").Return(
		[]entityprovider.EntityGroup{}, nil)

	groupIDs, err := executor.extractGroupIDs(ctx, "user123")
	assert.NoError(t, err)
	assert.Empty(t, groupIDs)
}

func TestAuthorizationExecutor_Execute_WithMultipleGroups(t *testing.T) {
	mockAuthzService := new(authzmock.AuthorizationServiceInterfaceMock)
	mockEntityProvider := new(entityprovidermock.EntityProviderInterfaceMock)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	authUser := newAuthzAuthenticatedAuthUser()
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		FlowType:    common.FlowTypeAuthentication,
		AuthUser:    authUser,
		RuntimeData: map[string]string{
			requestedPermissionsKey: "read:documents write:documents delete:documents",
			"groups":                `["admin", "editor", "viewer"]`,
		},
	}

	mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(authUser, &authnprovidercm.EntityReference{EntityID: "user123"}, nil)

	mockAuthzService.On("EvaluateAccessBatch",
		mock.Anything,
		mock.MatchedBy(func(req authzsvc.AccessEvaluationsRequest) bool {
			return len(req.Evaluations) == 3 &&
				req.Evaluations[0].Subject.ID == "user123" &&
				len(req.Evaluations[0].Subject.GroupIDs) == 3 &&
				req.Evaluations[0].Subject.GroupIDs[0] == "admin" &&
				req.Evaluations[0].Subject.GroupIDs[1] == "editor" &&
				req.Evaluations[0].Subject.GroupIDs[2] == "viewer"
		})).Return(&authzsvc.AccessEvaluationsResponse{
		Evaluations: []authzsvc.AccessEvaluationResponse{
			{Decision: true},
			{Decision: true},
			{Decision: true},
		},
	}, nil)

	// Execute
	resp, err := executor.Execute(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, common.ExecComplete, resp.Status)
	assert.Equal(t, "read:documents write:documents delete:documents", resp.RuntimeData[authorizedPermissionsKey])

	mockAuthzService.AssertExpectations(t)
}

func TestSetAuthorizedPermissions(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		expected    string
	}{
		{
			name:        "Multiple permissions",
			permissions: []string{"read:documents", "write:documents", "delete:documents"},
			expected:    "read:documents write:documents delete:documents",
		},
		{
			name:        "Single permission",
			permissions: []string{"read:documents"},
			expected:    "read:documents",
		},
		{
			name:        "Empty permissions",
			permissions: []string{},
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execResp := &common.ExecutorResponse{
				RuntimeData: make(map[string]string),
			}

			setAuthorizedPermissions(execResp, tt.permissions)
			assert.Equal(t, tt.expected, execResp.RuntimeData[authorizedPermissionsKey])
		})
	}
}

func TestAuthorizationExecutor_Execute_RegistrationFlow_UnauthenticatedWithoutPermissions(t *testing.T) {
	// Setup - registration flow with unauthenticated user and no requested permissions
	mockAuthzService := new(authzmock.AuthorizationServiceInterfaceMock)
	mockEntityProvider := new(entityprovidermock.EntityProviderInterfaceMock)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	ctx := &core.NodeContext{
		ExecutionID: "test-registration-flow",
		FlowType:    common.FlowTypeRegistration,
		RuntimeData: make(map[string]string),
	}

	// Execute
	resp, err := executor.Execute(ctx)

	// Assert - should succeed (bypass authentication check for registration)
	assert.NoError(t, err)
	assert.Equal(t, common.ExecComplete, resp.Status)
	assert.Empty(t, resp.RuntimeData[authorizedPermissionsKey])

	// Service should NOT be called since there are no requested permissions
	mockAuthzService.AssertNotCalled(t, "EvaluateAccessBatch")
}

func TestAuthorizationExecutor_Execute_RegistrationFlow_UnauthenticatedWithPermissions(t *testing.T) {
	// Setup - registration flow with unauthenticated user but WITH requested permissions
	mockAuthzService := new(authzmock.AuthorizationServiceInterfaceMock)
	mockEntityProvider := new(entityprovidermock.EntityProviderInterfaceMock)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	ctx := &core.NodeContext{
		ExecutionID: "test-registration-flow",
		FlowType:    common.FlowTypeRegistration,
		RuntimeData: map[string]string{
			requestedPermissionsKey: "read:documents write:documents",
		},
	}

	// Execute
	resp, err := executor.Execute(ctx)

	// Assert - registration flow returns early for unauthenticated users and the authorization service is NOT invoked
	assert.NoError(t, err)
	assert.Equal(t, common.ExecComplete, resp.Status)
	assert.Equal(t, "", resp.RuntimeData[authorizedPermissionsKey])

	mockAuthzService.AssertNotCalled(t, "EvaluateAccessBatch")
}

func TestAuthorizationExecutor_Execute_RegistrationFlow_AuthenticatedWithPermissions(t *testing.T) {
	// Setup - registration flow with authenticated user (edge case but possible)
	mockAuthzService := new(authzmock.AuthorizationServiceInterfaceMock)
	mockEntityProvider := new(entityprovidermock.EntityProviderInterfaceMock)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	existingUserID := testExistingUser123ID
	authUser := newAuthzAuthenticatedAuthUser()
	ctx := &core.NodeContext{
		ExecutionID: "test-registration-flow",
		FlowType:    common.FlowTypeRegistration,
		AuthUser:    authUser,
		RuntimeData: map[string]string{
			requestedPermissionsKey: "read:profile write:profile",
			"groups":                `["new-users"]`,
		},
	}

	mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(authUser, &authnprovidercm.EntityReference{EntityID: existingUserID}, nil)

	mockAuthzService.On("EvaluateAccessBatch",
		mock.Anything,
		mock.MatchedBy(func(req authzsvc.AccessEvaluationsRequest) bool {
			return len(req.Evaluations) == 2 &&
				req.Evaluations[0].Subject.ID == existingUserID &&
				len(req.Evaluations[0].Subject.GroupIDs) == 1 &&
				req.Evaluations[0].Subject.GroupIDs[0] == "new-users"
		})).Return(&authzsvc.AccessEvaluationsResponse{
		Evaluations: []authzsvc.AccessEvaluationResponse{
			{Decision: true},
			{Decision: false},
		},
	}, nil)

	// Execute
	resp, err := executor.Execute(ctx)

	// Assert - should succeed and call service
	assert.NoError(t, err)
	assert.Equal(t, common.ExecComplete, resp.Status)
	assert.Equal(t, "read:profile", resp.RuntimeData[authorizedPermissionsKey])

	mockAuthzService.AssertExpectations(t)
}

func TestAuthorizationExecutor_Execute_NonRegistrationFlow_UnauthenticatedShouldFail(t *testing.T) {
	// Setup - non-registration flow types should fail if unauthenticated
	testCases := []struct {
		name     string
		flowType common.FlowType
	}{
		{
			name:     "Authentication flow",
			flowType: common.FlowTypeAuthentication,
		},
		{
			name:     "User onboarding flow",
			flowType: common.FlowTypeUserOnboarding,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAuthzService := new(authzmock.AuthorizationServiceInterfaceMock)
			mockEntityProvider := new(entityprovidermock.EntityProviderInterfaceMock)
			mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
			executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

			ctx := &core.NodeContext{
				ExecutionID: "test-flow",
				FlowType:    tc.flowType,
				RuntimeData: map[string]string{
					requestedPermissionsKey: "read:documents",
				},
			}

			// Execute
			resp, err := executor.Execute(ctx)

			// Assert - should fail
			assert.NoError(t, err)
			assert.Equal(t, common.ExecFailure, resp.Status)
			assert.Equal(t, ErrUserNotAuthenticated.Code, resp.Error.Code)

			// Service should NOT be called
			mockAuthzService.AssertNotCalled(t, "EvaluateAccessBatch")
		})
	}
}

func TestAuthorizationExecutor_ExtractGroupIDs_FromEntityProvider(t *testing.T) {
	mockAuthzService := authzmock.NewAuthorizationServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	ctx := &core.NodeContext{
		Context:     context.Background(),
		RuntimeData: make(map[string]string), // No groups in runtime data
	}

	mockEntityProvider.On("GetTransitiveEntityGroups", "test-user-123").Return(
		[]entityprovider.EntityGroup{
			{ID: "svc-group-1"},
			{ID: "svc-group-2"},
		}, nil)

	groupIDs, err := executor.extractGroupIDs(ctx, "test-user-123")
	assert.NoError(t, err)
	assert.Equal(t, []string{"svc-group-1", "svc-group-2"}, groupIDs)
	mockEntityProvider.AssertExpectations(t)
}

func TestAuthorizationExecutor_ExtractGroupIDs_FromEntityProvider_Error(t *testing.T) {
	mockAuthzService := authzmock.NewAuthorizationServiceInterfaceMock(t)
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(t)
	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	executor := createTestAuthzExecutor(t, mockAuthzService, mockEntityProvider, mockAuthnProvider)

	ctx := &core.NodeContext{
		Context:     context.Background(),
		RuntimeData: make(map[string]string), // No groups in runtime data
	}

	mockEntityProvider.On("GetTransitiveEntityGroups", "test-user-err").Return(
		nil, entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeSystemError,
			"failed to retrieve groups",
			"failed to retrieve groups"))

	groupIDs, err := executor.extractGroupIDs(ctx, "test-user-err")
	assert.Error(t, err)
	assert.Nil(t, groupIDs)
	mockEntityProvider.AssertExpectations(t)
}
