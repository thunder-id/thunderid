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
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestGetAuthnServiceName() {
	tests := []struct {
		name         string
		executorName string
		expectedName string
	}{
		{"BasicAuth executor", ExecutorNameBasicAuth, authncm.AuthenticatorCredentials},
		{"SMS Auth executor", ExecutorNameSMSAuth, authncm.AuthenticatorSMSOTP},
		{"OAuth executor", ExecutorNameOAuth, authncm.AuthenticatorOAuth},
		{"OIDC Auth executor", ExecutorNameOIDCAuth, authncm.AuthenticatorOIDC},
		{"GitHub Auth executor", ExecutorNameGitHubAuth, authncm.AuthenticatorGithub},
		{"Google Auth executor", ExecutorNameGoogleAuth, authncm.AuthenticatorGoogle},
		{"Unknown executor returns empty string", "UnknownExecutor", ""},
		{"Provisioning executor returns empty string", ExecutorNameProvisioning, ""},
		{"AuthAssert executor returns empty string", ExecutorNameAuthAssert, ""},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := getAuthnServiceName(tt.executorName)
			s.Equal(tt.expectedName, result)
		})
	}
}

// createMockAuthExecutor creates a mock executor for OAuth/OIDC authentication.
func createMockAuthExecutor(t *testing.T, executorName string) core.ExecutorInterface {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(executorName).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return([]common.Input{
		{Identifier: "code", Type: "string", Required: true},
	}).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{}).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *core.NodeContext, execResp *common.ExecutorResponse) bool {
			if code, ok := ctx.UserInputs["code"]; ok && code != "" {
				return true
			}
			if len(ctx.NodeInputs) == 0 {
				return true
			}
			execResp.Inputs = []common.Input{{Identifier: "code", Type: "string", Required: true}}
			return false
		}).Maybe()
	return mockExec
}

func (s *UtilsTestSuite) TestGetUserAttribute() {
	tests := []struct {
		name         string
		user         *entityprovider.Entity
		attributeKey string
		expectedVal  string
		expectError  bool
	}{
		{
			name: "Success case",
			user: &entityprovider.Entity{
				Attributes: []byte(`{"email":"user@example.com"}`),
			},
			attributeKey: "email",
			expectedVal:  "user@example.com",
			expectError:  false,
		},
		{
			name:         "Nil user",
			user:         nil,
			attributeKey: "email",
			expectError:  true,
		},
		{
			name: "Empty attributes",
			user: &entityprovider.Entity{
				Attributes: []byte(``),
			},
			attributeKey: "email",
			expectError:  true,
		},
		{
			name: "Invalid JSON attributes",
			user: &entityprovider.Entity{
				Attributes: []byte(`invalid-json`),
			},
			attributeKey: "email",
			expectError:  true,
		},
		{
			name: "Attribute not found",
			user: &entityprovider.Entity{
				Attributes: []byte(`{"other":"data"}`),
			},
			attributeKey: "email",
			expectError:  true,
		},
		{
			name: "Non-string attribute value",
			user: &entityprovider.Entity{
				Attributes: []byte(`{"email":123}`),
			},
			attributeKey: "email",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			val, err := GetUserAttribute(tt.user, tt.attributeKey)
			if tt.expectError {
				s.Error(err)
				s.Empty(val)
			} else {
				s.NoError(err)
				s.Equal(tt.expectedVal, val)
			}
		})
	}
}

func (s *UtilsTestSuite) TestIsAuthenticationWithoutLocalUserAllowed() {
	tests := []struct {
		name       string
		properties map[string]interface{}
		expected   bool
	}{
		{
			name: "Property true",
			properties: map[string]interface{}{
				common.NodePropertyAllowAuthenticationWithoutLocalUser: true,
			},
			expected: true,
		},
		{
			name: "Property false",
			properties: map[string]interface{}{
				common.NodePropertyAllowAuthenticationWithoutLocalUser: false,
			},
			expected: false,
		},
		{
			name: "Property missing",
			properties: map[string]interface{}{
				"other": true,
			},
			expected: false,
		},
		{
			name: "Property invalid type",
			properties: map[string]interface{}{
				common.NodePropertyAllowAuthenticationWithoutLocalUser: "true",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			ctx := &core.NodeContext{NodeProperties: tt.properties}
			result := isAuthenticationWithoutLocalUserAllowed(ctx)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *UtilsTestSuite) TestIsRegistrationWithExistingUserAllowed() {
	tests := []struct {
		name       string
		properties map[string]interface{}
		expected   bool
	}{
		{
			name: "Property true",
			properties: map[string]interface{}{
				common.NodePropertyAllowRegistrationWithExistingUser: true,
			},
			expected: true,
		},
		{
			name: "Property false",
			properties: map[string]interface{}{
				common.NodePropertyAllowRegistrationWithExistingUser: false,
			},
			expected: false,
		},
		{
			name: "Property missing",
			properties: map[string]interface{}{
				"other": true,
			},
			expected: false,
		},
		{
			name: "Property invalid type",
			properties: map[string]interface{}{
				common.NodePropertyAllowRegistrationWithExistingUser: 1,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			ctx := &core.NodeContext{NodeProperties: tt.properties}
			result := isRegistrationWithExistingUserAllowed(ctx)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *UtilsTestSuite) TestIsCrossOUProvisioningAllowed() {
	tests := []struct {
		name       string
		properties map[string]interface{}
		expected   bool
	}{
		{
			name: "Property true",
			properties: map[string]interface{}{
				common.NodePropertyAllowCrossOUProvisioning: true,
			},
			expected: true,
		},
		{
			name: "Property false",
			properties: map[string]interface{}{
				common.NodePropertyAllowCrossOUProvisioning: false,
			},
			expected: false,
		},
		{
			name: "Property missing",
			properties: map[string]interface{}{
				"other": true,
			},
			expected: false,
		},
		{
			name: "Property invalid type",
			properties: map[string]interface{}{
				common.NodePropertyAllowCrossOUProvisioning: []string{"true"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			ctx := &core.NodeContext{NodeProperties: tt.properties}
			result := isCrossOUProvisioningAllowed(ctx)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *UtilsTestSuite) TestValidateFederatedIdentifierConsistency() {
	tests := []struct {
		name          string
		basicResult   *authnprovidermgr.AuthnBasicResult
		ctx           *core.NodeContext
		expectedValid bool
		expectError   bool
	}{
		{
			name:          "Nil basicResult returns true",
			basicResult:   nil,
			ctx:           &core.NodeContext{},
			expectedValid: true,
			expectError:   false,
		},
		{
			name: "No federated identifiers returns true",
			basicResult: &authnprovidermgr.AuthnBasicResult{
				ExternalSub:    "",
				ExternalClaims: map[string]interface{}{},
			},
			ctx:           &core.NodeContext{},
			expectedValid: true,
			expectError:   false,
		},
		{
			name: "Email matches UserInputs returns true",
			basicResult: &authnprovidermgr.AuthnBasicResult{
				ExternalSub: "sub123",
				ExternalClaims: map[string]interface{}{
					"email": "user@example.com",
				},
			},
			ctx: &core.NodeContext{
				UserInputs: map[string]string{
					"email": "user@example.com",
				},
			},
			expectedValid: true,
			expectError:   false,
		},
		{
			name: "Email mismatch with UserInputs returns false",
			basicResult: &authnprovidermgr.AuthnBasicResult{
				ExternalSub: "sub123",
				ExternalClaims: map[string]interface{}{
					"email": "user1@example.com",
				},
			},
			ctx: &core.NodeContext{
				UserInputs: map[string]string{
					"email": "user2@example.com",
				},
			},
			expectedValid: false,
			expectError:   false,
		},
		{
			name: "Email matches RuntimeData returns true",
			basicResult: &authnprovidermgr.AuthnBasicResult{
				ExternalSub: "sub123",
				ExternalClaims: map[string]interface{}{
					"email": "user@example.com",
				},
			},
			ctx: &core.NodeContext{
				RuntimeData: map[string]string{
					"email": "user@example.com",
				},
			},
			expectedValid: true,
			expectError:   false,
		},
		{
			name: "Email mismatch with RuntimeData returns false",
			basicResult: &authnprovidermgr.AuthnBasicResult{
				ExternalSub: "sub123",
				ExternalClaims: map[string]interface{}{
					"email": "user1@example.com",
				},
			},
			ctx: &core.NodeContext{
				RuntimeData: map[string]string{
					"email": "user2@example.com",
				},
			},
			expectedValid: false,
			expectError:   false,
		},
		{
			name: "Email matches AuthenticatedUser.Attributes returns true",
			basicResult: &authnprovidermgr.AuthnBasicResult{
				ExternalSub: "sub123",
				ExternalClaims: map[string]interface{}{
					"email": "user@example.com",
				},
			},
			ctx: &core.NodeContext{
				AuthenticatedUser: authncm.AuthenticatedUser{
					Attributes: map[string]interface{}{
						"email": "user@example.com",
					},
				},
			},
			expectedValid: true,
			expectError:   false,
		},
		{
			name: "Email mismatch with AuthenticatedUser.Attributes returns false",
			basicResult: &authnprovidermgr.AuthnBasicResult{
				ExternalSub: "sub123",
				ExternalClaims: map[string]interface{}{
					"email": "user1@example.com",
				},
			},
			ctx: &core.NodeContext{
				AuthenticatedUser: authncm.AuthenticatedUser{
					Attributes: map[string]interface{}{
						"email": "user2@example.com",
					},
				},
			},
			expectedValid: false,
			expectError:   false,
		},
		{
			name: "Sub matches RuntimeData returns true",
			basicResult: &authnprovidermgr.AuthnBasicResult{
				ExternalSub: "sub123",
				ExternalClaims: map[string]interface{}{
					"email": "user@example.com",
				},
			},
			ctx: &core.NodeContext{
				RuntimeData: map[string]string{
					"sub": "sub123",
				},
			},
			expectedValid: true,
			expectError:   false,
		},
		{
			name: "Sub mismatch with RuntimeData returns false",
			basicResult: &authnprovidermgr.AuthnBasicResult{
				ExternalSub: "sub123",
				ExternalClaims: map[string]interface{}{
					"email": "user@example.com",
				},
			},
			ctx: &core.NodeContext{
				RuntimeData: map[string]string{
					"sub": "sub456",
				},
			},
			expectedValid: false,
			expectError:   false,
		},
		{
			name: "Empty UserInputs email is skipped",
			basicResult: &authnprovidermgr.AuthnBasicResult{
				ExternalSub: "sub123",
				ExternalClaims: map[string]interface{}{
					"email": "user@example.com",
				},
			},
			ctx: &core.NodeContext{
				UserInputs: map[string]string{
					"email": "",
				},
			},
			expectedValid: true,
			expectError:   false,
		},
		{
			name: "Empty RuntimeData email is skipped",
			basicResult: &authnprovidermgr.AuthnBasicResult{
				ExternalSub: "sub123",
				ExternalClaims: map[string]interface{}{
					"email": "user@example.com",
				},
			},
			ctx: &core.NodeContext{
				RuntimeData: map[string]string{
					"email": "",
				},
			},
			expectedValid: true,
			expectError:   false,
		},
		{
			name: "Missing email from UserInputs and RuntimeData is allowed",
			basicResult: &authnprovidermgr.AuthnBasicResult{
				ExternalSub: "sub123",
				ExternalClaims: map[string]interface{}{
					"email": "user@example.com",
				},
			},
			ctx:           &core.NodeContext{},
			expectedValid: true,
			expectError:   false,
		},
		{
			name: "Multiple attributes with one mismatch returns false",
			basicResult: &authnprovidermgr.AuthnBasicResult{
				ExternalSub: "sub123",
				ExternalClaims: map[string]interface{}{
					"email": "user1@example.com",
				},
			},
			ctx: &core.NodeContext{
				UserInputs: map[string]string{
					"email": "user1@example.com",
					"sub":   "sub456",
				},
				RuntimeData: map[string]string{
					"sub": "sub123",
				},
			},
			expectedValid: false,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			valid := validateFederatedIdentifierConsistency(tt.ctx, tt.basicResult)
			s.Equal(tt.expectedValid, valid)
		})
	}
}

func (s *UtilsTestSuite) TestGetAuthenticatedIdentifierValue() {
	tests := []struct {
		name          string
		ctx           *core.NodeContext
		key           string
		expectedValue string
	}{
		{
			name: "Nil Attributes map returns empty string",
			ctx: &core.NodeContext{
				AuthenticatedUser: authncm.AuthenticatedUser{
					Attributes: nil,
				},
			},
			key:           "email",
			expectedValue: "",
		},
		{
			name: "Empty Attributes map returns empty string",
			ctx: &core.NodeContext{
				AuthenticatedUser: authncm.AuthenticatedUser{
					Attributes: map[string]interface{}{},
				},
			},
			key:           "email",
			expectedValue: "",
		},
		{
			name: "Key not found returns empty string",
			ctx: &core.NodeContext{
				AuthenticatedUser: authncm.AuthenticatedUser{
					Attributes: map[string]interface{}{
						"other": "value",
					},
				},
			},
			key:           "email",
			expectedValue: "",
		},
		{
			name: "String value is returned",
			ctx: &core.NodeContext{
				AuthenticatedUser: authncm.AuthenticatedUser{
					Attributes: map[string]interface{}{
						"email": "user@example.com",
					},
				},
			},
			key:           "email",
			expectedValue: "user@example.com",
		},
		{
			name: "Sub string value is returned",
			ctx: &core.NodeContext{
				AuthenticatedUser: authncm.AuthenticatedUser{
					Attributes: map[string]interface{}{
						"sub": "sub123",
					},
				},
			},
			key:           "sub",
			expectedValue: "sub123",
		},
		{
			name: "Non-string value is converted to string",
			ctx: &core.NodeContext{
				AuthenticatedUser: authncm.AuthenticatedUser{
					Attributes: map[string]interface{}{
						"id": 123,
					},
				},
			},
			key:           "id",
			expectedValue: "123",
		},
		{
			name: "Boolean value is converted to string",
			ctx: &core.NodeContext{
				AuthenticatedUser: authncm.AuthenticatedUser{
					Attributes: map[string]interface{}{
						"active": true,
					},
				},
			},
			key:           "active",
			expectedValue: "true",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			value := getAuthenticatedIdentifierValue(tt.ctx, tt.key)
			s.Equal(tt.expectedValue, value)
		})
	}
}
