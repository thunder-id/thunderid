// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type PermissionValidatorTestSuite struct {
	suite.Suite
	mockFlowFactory *coremock.FlowFactoryInterfaceMock
	executor        *permissionValidator
}

func (suite *PermissionValidatorTestSuite) SetupTest() {
	security.InitControlPlanePermissions("")

	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())

	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNamePermissionValidator,
		providers.ExecutorTypeUtility,
		[]providers.Input{},
		[]providers.Input{}, mock.Anything).Return(mockBaseExecutor)

	suite.executor = newPermissionValidator(suite.mockFlowFactory)
}

func (suite *PermissionValidatorTestSuite) TestExecute_DefaultScopeCheck_Success() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		[]string{"system", "other"}, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *PermissionValidatorTestSuite) TestExecute_DefaultScopeCheck_Failure() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		[]string{"other"}, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrInsufficientPermissions.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *PermissionValidatorTestSuite) TestExecute_CustomScopeCheck_Success() {
	type testCase struct {
		name           string
		nodeProps      map[string]interface{}
		contextScopes  []string
		expectedStatus providers.ExecutorStatus
	}

	testCases := []testCase{
		{
			name: "Success - Required scopes configured and present",
			nodeProps: map[string]interface{}{
				propertyKeyRequiredScopes: []interface{}{"scope1"},
			},
			contextScopes:  []string{"scope1"},
			expectedStatus: providers.ExecComplete,
		},
		{
			name: "Failure - Required scopes configured but missing",
			nodeProps: map[string]interface{}{
				propertyKeyRequiredScopes: []interface{}{"scope1"},
			},
			contextScopes:  []string{"scope2"},
			expectedStatus: providers.ExecFailure,
		},
		{
			name:           "Success - No required scopes configured (default system scope present)",
			nodeProps:      nil,
			contextScopes:  []string{"system"},
			expectedStatus: providers.ExecComplete,
		},
		{
			name: "Success - Empty required scopes (default system scope present)",
			nodeProps: map[string]interface{}{
				propertyKeyRequiredScopes: []interface{}{},
			},
			contextScopes:  []string{"system"},
			expectedStatus: providers.ExecComplete,
		},
		{
			name: "Success - Multiple required scopes configured (OR logic) - scope1 present",
			nodeProps: map[string]interface{}{
				propertyKeyRequiredScopes: []interface{}{"scope1", "scope2"},
			},
			contextScopes:  []string{"scope1"},
			expectedStatus: providers.ExecComplete,
		},
		{
			name: "Success - Multiple required scopes configured (OR logic) - scope2 present",
			nodeProps: map[string]interface{}{
				propertyKeyRequiredScopes: []interface{}{"scope1", "scope2"},
			},
			contextScopes:  []string{"scope2"},
			expectedStatus: providers.ExecComplete,
		},
		{
			name: "Failure - Multiple required scopes configured - none present",
			nodeProps: map[string]interface{}{
				propertyKeyRequiredScopes: []interface{}{"scope1", "scope2"},
			},
			contextScopes:  []string{"scope3"},
			expectedStatus: providers.ExecFailure,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			httpCtx := context.Background()
			authCtx := security.NewSecurityContextForTest(
				"user1", "ou1", "token",
				tc.contextScopes, nil,
			)
			httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

			ctx := &providers.NodeContext{
				ExecutionID:    "test-flow",
				Context:        httpCtx,
				NodeProperties: tc.nodeProps,
			}

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), tc.expectedStatus, resp.Status)
		})
	}
}

func (suite *PermissionValidatorTestSuite) TestExecute_MultipleRequiredScopes_OR_Success() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		[]string{"scopeB"}, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
		NodeProperties: map[string]interface{}{
			propertyKeyRequiredScopes: []interface{}{"scopeA", "scopeB"},
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *PermissionValidatorTestSuite) TestExecute_AuthorizedPermissionsCheck_Success() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		[]string{"read", "write", "admin"}, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
		NodeProperties: map[string]interface{}{
			propertyKeyRequiredScopes: []interface{}{"admin"},
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *PermissionValidatorTestSuite) TestExecute_HierarchicalScopeCheck_Success() {
	type testCase struct {
		name           string
		requiredScopes []interface{}
		contextScopes  []string
	}

	testCases := []testCase{
		{
			name:           "Root scope satisfies a narrower required scope",
			requiredScopes: []interface{}{"system:user"},
			contextScopes:  []string{"system"},
		},
		{
			name:           "Exact scoped permission still satisfies its own requirement",
			requiredScopes: []interface{}{"system:user"},
			contextScopes:  []string{"system:user"},
		},
		{
			name:           "Parent scope satisfies a deeply nested required scope",
			requiredScopes: []interface{}{"system:user:view"},
			contextScopes:  []string{"system"},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			httpCtx := context.Background()
			authCtx := security.NewSecurityContextForTest(
				"user1", "ou1", "token",
				tc.contextScopes, nil,
			)
			httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

			ctx := &providers.NodeContext{
				ExecutionID: "test-flow",
				Context:     httpCtx,
				NodeProperties: map[string]interface{}{
					propertyKeyRequiredScopes: tc.requiredScopes,
				},
			}

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(t, err)
			assert.Equal(t, providers.ExecComplete, resp.Status)
		})
	}
}

func (suite *PermissionValidatorTestSuite) TestExecute_HierarchicalScopeCheck_Failure() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		[]string{"system:user"}, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
		NodeProperties: map[string]interface{}{
			propertyKeyRequiredScopes: []interface{}{"system"},
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
}

func (suite *PermissionValidatorTestSuite) TestExecute_NoHTTPContext() {
	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     nil,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrInsufficientPermissions.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *PermissionValidatorTestSuite) TestExecute_EmptyScopes() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		nil, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrInsufficientPermissions.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *PermissionValidatorTestSuite) TestExecute_NoScopesInContext() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		nil, nil, // No permissions
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrInsufficientPermissions.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *PermissionValidatorTestSuite) TestExecute_ScopesWithUnexpectedType() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		nil, nil, // No valid permissions
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrInsufficientPermissions.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

// A bootstrap flow lists each plane's permission for the same resource, and a node with no required
// scopes falls back to the permission each plane names for administration flows.
func (suite *PermissionValidatorTestSuite) TestExecute_PerPlaneScopes() {
	both := []interface{}{"system:user", "org:user"}
	cases := map[string][]struct {
		permission string
		required   []interface{}
		want       providers.ExecutorStatus
	}{
		"ControlPlane": {
			{"system", both, providers.ExecComplete},
			{"system:user", both, providers.ExecComplete},
			{"org:group", both, providers.ExecFailure},
			{"system", nil, providers.ExecComplete},
			{"org", nil, providers.ExecFailure},
		},
		"DataPlane": {
			{"system", both, providers.ExecComplete},
			{"org", both, providers.ExecComplete},
			{"org:user", both, providers.ExecComplete},
			{"org:user:update", both, providers.ExecFailure},
			{"org:group", both, providers.ExecFailure},
			{"system", nil, providers.ExecComplete},
			{"org", nil, providers.ExecComplete},
			{"org:user", nil, providers.ExecFailure},
		},
	}
	security.ForEachPermissionPlaneForTest(func(plane string) {
		suite.Run(plane, func() {
			for _, tc := range cases[plane] {
				ctx := &providers.NodeContext{
					ExecutionID: "test-flow",
					Context: security.WithSecurityContextTest(context.Background(),
						security.NewSecurityContextForTest("user1", "ou1", "token", []string{tc.permission}, nil)),
				}
				if tc.required != nil {
					ctx.NodeProperties = map[string]interface{}{propertyKeyRequiredScopes: tc.required}
				}

				resp, err := suite.executor.Execute(ctx)

				suite.NoError(err)
				suite.Equal(tc.want, resp.Status, "%s with required %v", tc.permission, tc.required)
			}
		})
	})
}

func TestPermissionValidatorSuite(t *testing.T) {
	suite.Run(t, new(PermissionValidatorTestSuite))
}
