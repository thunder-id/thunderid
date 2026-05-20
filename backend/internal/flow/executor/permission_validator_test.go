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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type PermissionValidatorTestSuite struct {
	suite.Suite
	mockFlowFactory *coremock.FlowFactoryInterfaceMock
	executor        *permissionValidator
}

func (suite *PermissionValidatorTestSuite) SetupTest() {
	security.InitSystemPermissions("")

	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())

	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNamePermissionValidator,
		common.ExecutorTypeUtility,
		[]common.Input{},
		[]common.Input{}).Return(mockBaseExecutor)

	suite.executor = newPermissionValidator(suite.mockFlowFactory)
}

func (suite *PermissionValidatorTestSuite) TestExecute_DefaultScopeCheck_Success() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		[]string{"system", "other"}, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *PermissionValidatorTestSuite) TestExecute_DefaultScopeCheck_Failure() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		[]string{"other"}, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "Insufficient permissions", resp.FailureReason)
}

func (suite *PermissionValidatorTestSuite) TestExecute_CustomScopeCheck_Success() {
	type testCase struct {
		name           string
		nodeProps      map[string]interface{}
		contextScopes  []string
		expectedStatus common.ExecutorStatus
	}

	testCases := []testCase{
		{
			name: "Success - Required scopes configured and present",
			nodeProps: map[string]interface{}{
				propertyKeyRequiredScopes: []interface{}{"scope1"},
			},
			contextScopes:  []string{"scope1"},
			expectedStatus: common.ExecComplete,
		},
		{
			name: "Failure - Required scopes configured but missing",
			nodeProps: map[string]interface{}{
				propertyKeyRequiredScopes: []interface{}{"scope1"},
			},
			contextScopes:  []string{"scope2"},
			expectedStatus: common.ExecFailure,
		},
		{
			name:           "Success - No required scopes configured (default system scope present)",
			nodeProps:      nil,
			contextScopes:  []string{"system"},
			expectedStatus: common.ExecComplete,
		},
		{
			name: "Success - Empty required scopes (default system scope present)",
			nodeProps: map[string]interface{}{
				propertyKeyRequiredScopes: []interface{}{},
			},
			contextScopes:  []string{"system"},
			expectedStatus: common.ExecComplete,
		},
		{
			name: "Success - Multiple required scopes configured (OR logic) - scope1 present",
			nodeProps: map[string]interface{}{
				propertyKeyRequiredScopes: []interface{}{"scope1", "scope2"},
			},
			contextScopes:  []string{"scope1"},
			expectedStatus: common.ExecComplete,
		},
		{
			name: "Success - Multiple required scopes configured (OR logic) - scope2 present",
			nodeProps: map[string]interface{}{
				propertyKeyRequiredScopes: []interface{}{"scope1", "scope2"},
			},
			contextScopes:  []string{"scope2"},
			expectedStatus: common.ExecComplete,
		},
		{
			name: "Failure - Multiple required scopes configured - none present",
			nodeProps: map[string]interface{}{
				propertyKeyRequiredScopes: []interface{}{"scope1", "scope2"},
			},
			contextScopes:  []string{"scope3"},
			expectedStatus: common.ExecFailure,
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

			ctx := &core.NodeContext{
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

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
		NodeProperties: map[string]interface{}{
			propertyKeyRequiredScopes: []interface{}{"scopeA", "scopeB"},
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *PermissionValidatorTestSuite) TestExecute_AuthorizedPermissionsCheck_Success() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		[]string{"read", "write", "admin"}, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
		NodeProperties: map[string]interface{}{
			propertyKeyRequiredScopes: []interface{}{"admin"},
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *PermissionValidatorTestSuite) TestExecute_NoHTTPContext() {
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		Context:     nil,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "Insufficient permissions", resp.FailureReason)
}

func (suite *PermissionValidatorTestSuite) TestExecute_EmptyScopes() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		nil, nil,
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "Insufficient permissions", resp.FailureReason)
}

func (suite *PermissionValidatorTestSuite) TestExecute_NoScopesInContext() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		nil, nil, // No permissions
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "Insufficient permissions", resp.FailureReason)
}

func (suite *PermissionValidatorTestSuite) TestExecute_ScopesWithUnexpectedType() {
	httpCtx := context.Background()
	authCtx := security.NewSecurityContextForTest(
		"user1", "ou1", "token",
		nil, nil, // No valid permissions
	)
	httpCtx = security.WithSecurityContextTest(httpCtx, authCtx)

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		Context:     httpCtx,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "Insufficient permissions", resp.FailureReason)
}

func TestPermissionValidatorSuite(t *testing.T) {
	suite.Run(t, new(PermissionValidatorTestSuite))
}
