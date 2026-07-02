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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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
		{"CredentialsAuth executor", ExecutorNameCredentialsAuth, authncm.AuthenticatorCredentials},
		{"OTP executor", ExecutorNameOTPExecutor, authncm.AuthenticatorOTP},
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
func createMockAuthExecutor(t *testing.T, executorName string) providers.Executor {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(executorName).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return([]providers.Input{
		{Identifier: "code", Type: "string", Required: true},
	}).Maybe()
	mockExec.On("GetPrerequisites").Return([]providers.Input{}).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *providers.NodeContext, execResp *providers.ExecutorResponse) bool {
			if code, ok := ctx.UserInputs["code"]; ok && code != "" {
				return true
			}
			if len(ctx.NodeInputs) == 0 {
				return true
			}
			execResp.Inputs = []providers.Input{{Identifier: "code", Type: "string", Required: true}}
			return false
		}).Maybe()
	return mockExec
}

func (s *UtilsTestSuite) TestGetUserAttribute() {
	tests := []struct {
		name         string
		user         *providers.Entity
		attributeKey string
		expectedVal  string
		expectError  bool
	}{
		{
			name: "Success case",
			user: &providers.Entity{
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
			user: &providers.Entity{
				Attributes: []byte(``),
			},
			attributeKey: "email",
			expectError:  true,
		},
		{
			name: "Invalid JSON attributes",
			user: &providers.Entity{
				Attributes: []byte(`invalid-json`),
			},
			attributeKey: "email",
			expectError:  true,
		},
		{
			name: "Attribute not found",
			user: &providers.Entity{
				Attributes: []byte(`{"other":"data"}`),
			},
			attributeKey: "email",
			expectError:  true,
		},
		{
			name: "Non-string attribute value",
			user: &providers.Entity{
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
			ctx := &providers.NodeContext{NodeProperties: tt.properties}
			result := isAuthenticationWithoutLocalUserAllowed(ctx)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *UtilsTestSuite) TestFindInputByType() {
	tests := []struct {
		name        string
		inputs      []providers.Input
		inputType   string
		expected    providers.Input
		expectFound bool
	}{
		{
			name:        "Empty inputs",
			inputs:      []providers.Input{},
			inputType:   providers.InputTypeEmail,
			expected:    providers.Input{},
			expectFound: false,
		},
		{
			name: "Type found",
			inputs: []providers.Input{
				{Identifier: "mobile", Type: "phone"},
				{Identifier: "workEmail", Type: providers.InputTypeEmail},
			},
			inputType:   providers.InputTypeEmail,
			expected:    providers.Input{Identifier: "workEmail", Type: providers.InputTypeEmail},
			expectFound: true,
		},
		{
			name: "Type not found",
			inputs: []providers.Input{
				{Identifier: "mobile", Type: "phone"},
			},
			inputType:   providers.InputTypeEmail,
			expected:    providers.Input{},
			expectFound: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			res, found := findInputByType(tt.inputs, tt.inputType)
			s.Equal(tt.expectFound, found)
			s.Equal(tt.expected, res)
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
			ctx := &providers.NodeContext{NodeProperties: tt.properties}
			result := isRegistrationWithExistingUserAllowed(ctx)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *UtilsTestSuite) TestResolveInputIdentifierByType() {
	tests := []struct {
		name      string
		ctx       *providers.NodeContext
		inputType string
		fallback  string
		expected  string
	}{
		{
			name: "Type found in NodeInputs",
			ctx: &providers.NodeContext{
				NodeInputs: []providers.Input{
					{Identifier: "customEmailIdentifier", Type: providers.InputTypeEmail},
				},
			},
			inputType: providers.InputTypeEmail,
			fallback:  "defaultEmail",
			expected:  "customEmailIdentifier",
		},
		{
			name: "Type not found, returns fallback",
			ctx: &providers.NodeContext{
				NodeInputs: []providers.Input{
					{Identifier: "phone", Type: "mobile"},
				},
			},
			inputType: providers.InputTypeEmail,
			fallback:  "defaultEmail",
			expected:  "defaultEmail",
		},
		{
			name: "Empty NodeInputs, returns fallback",
			ctx: &providers.NodeContext{
				NodeInputs: []providers.Input{},
			},
			inputType: providers.InputTypeEmail,
			fallback:  "defaultEmail",
			expected:  "defaultEmail",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := resolveInputIdentifierByType(tt.ctx, tt.inputType, tt.fallback)
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
			ctx := &providers.NodeContext{NodeProperties: tt.properties}
			result := isCrossOUProvisioningAllowed(ctx)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *UtilsTestSuite) TestValidateFederatedIdentifierConsistency() {
	tests := []struct {
		name                 string
		federatedIdentifiers map[string]interface{}
		existingIdentifiers  map[string]interface{}
		ctx                  *providers.NodeContext
		expectedValid        bool
	}{
		{
			name:                 "Nil federated identifiers returns true",
			federatedIdentifiers: nil,
			existingIdentifiers:  nil,
			ctx:                  &providers.NodeContext{},
			expectedValid:        true,
		},
		{
			name:                 "Empty federated identifiers returns true",
			federatedIdentifiers: map[string]interface{}{},
			existingIdentifiers:  map[string]interface{}{},
			ctx:                  &providers.NodeContext{},
			expectedValid:        true,
		},
		{
			name: "Email matches UserInputs returns true",
			federatedIdentifiers: map[string]interface{}{
				"email": "user@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{},
			ctx: &providers.NodeContext{
				UserInputs: map[string]string{
					"email": "user@example.com",
				},
			},
			expectedValid: true,
		},
		{
			name: "Email mismatch with UserInputs returns false",
			federatedIdentifiers: map[string]interface{}{
				"email": "user1@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{},
			ctx: &providers.NodeContext{
				UserInputs: map[string]string{
					"email": "user2@example.com",
				},
			},
			expectedValid: false,
		},
		{
			name: "Email matches RuntimeData returns true",
			federatedIdentifiers: map[string]interface{}{
				"email": "user@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{},
			ctx: &providers.NodeContext{
				RuntimeData: map[string]string{
					"email": "user@example.com",
				},
			},
			expectedValid: true,
		},
		{
			name: "Email mismatch with RuntimeData returns false",
			federatedIdentifiers: map[string]interface{}{
				"email": "user1@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{},
			ctx: &providers.NodeContext{
				RuntimeData: map[string]string{
					"email": "user2@example.com",
				},
			},
			expectedValid: false,
		},
		{
			name: "Email matches existing identifiers returns true",
			federatedIdentifiers: map[string]interface{}{
				"email": "user@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{
				"email": "user@example.com",
			},
			ctx:           &providers.NodeContext{},
			expectedValid: true,
		},
		{
			name: "Email mismatch with existing identifiers returns false",
			federatedIdentifiers: map[string]interface{}{
				"email": "user1@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{
				"email": "user2@example.com",
			},
			ctx:           &providers.NodeContext{},
			expectedValid: false,
		},
		{
			name: "Sub matches RuntimeData returns true",
			federatedIdentifiers: map[string]interface{}{
				"email": "user@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{},
			ctx: &providers.NodeContext{
				RuntimeData: map[string]string{
					"sub": "sub123",
				},
			},
			expectedValid: true,
		},
		{
			name: "Sub mismatch with RuntimeData returns false",
			federatedIdentifiers: map[string]interface{}{
				"email": "user@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{},
			ctx: &providers.NodeContext{
				RuntimeData: map[string]string{
					"sub": "sub456",
				},
			},
			expectedValid: false,
		},
		{
			name: "Empty UserInputs email is skipped",
			federatedIdentifiers: map[string]interface{}{
				"email": "user@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{},
			ctx: &providers.NodeContext{
				UserInputs: map[string]string{
					"email": "",
				},
			},
			expectedValid: true,
		},
		{
			name: "Empty RuntimeData email is skipped",
			federatedIdentifiers: map[string]interface{}{
				"email": "user@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{},
			ctx: &providers.NodeContext{
				RuntimeData: map[string]string{
					"email": "",
				},
			},
			expectedValid: true,
		},
		{
			name: "Missing email from UserInputs and RuntimeData is allowed",
			federatedIdentifiers: map[string]interface{}{
				"email": "user@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{},
			ctx:                 &providers.NodeContext{},
			expectedValid:       true,
		},
		{
			name: "Multiple attributes with one mismatch returns false",
			federatedIdentifiers: map[string]interface{}{
				"email": "user1@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{},
			ctx: &providers.NodeContext{
				UserInputs: map[string]string{
					"email": "user1@example.com",
					"sub":   "sub456",
				},
				RuntimeData: map[string]string{
					"sub": "sub123",
				},
			},
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			valid := validateFederatedIdentifierConsistency(tt.ctx, tt.federatedIdentifiers, tt.existingIdentifiers)
			s.Equal(tt.expectedValid, valid)
		})
	}
}

func (s *UtilsTestSuite) TestBuildAuthnMetadata_WithAllFields() {
	ctx := &providers.NodeContext{
		Application: providers.Application{
			Metadata: map[string]interface{}{
				"tenant_id": "tenant-123",
				"region":    "us-west",
			},
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{
					Type: providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{
						ClientID: "oauth-client-1",
					},
				},
				{
					Type: providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{
						ClientID: "oauth-client-2",
					},
				},
			},
		},
	}

	metadata := buildAuthnMetadata(ctx)

	assert.NotNil(s.T(), metadata)
	assert.NotNil(s.T(), metadata.AppMetadata)
	assert.Equal(s.T(), "tenant-123", metadata.AppMetadata["tenant_id"])
	assert.Equal(s.T(), "us-west", metadata.AppMetadata["region"])

	clientIDs, ok := metadata.AppMetadata["client_ids"].([]string)
	assert.True(s.T(), ok)
	assert.Len(s.T(), clientIDs, 2)
	assert.Contains(s.T(), clientIDs, "oauth-client-1")
	assert.Contains(s.T(), clientIDs, "oauth-client-2")
}

func (s *UtilsTestSuite) TestBuildAuthnMetadata_WithNoMetadata() {
	ctx := &providers.NodeContext{
		Application: providers.Application{},
	}

	metadata := buildAuthnMetadata(ctx)

	assert.NotNil(s.T(), metadata)
	assert.NotNil(s.T(), metadata.AppMetadata)
	assert.Len(s.T(), metadata.AppMetadata, 0)
	assert.NotNil(s.T(), metadata.RuntimeMetadata)
	assert.Equal(s.T(), "", metadata.RuntimeMetadata["authorization_request_id"])
	assert.Equal(s.T(), "", metadata.RuntimeMetadata["current_client_id"])
}

func (s *UtilsTestSuite) TestBuildAuthnMetadata_WithOnlyAppMetadata() {
	ctx := &providers.NodeContext{
		Application: providers.Application{
			Metadata: map[string]interface{}{
				"environment": "production",
				"version":     "1.0.0",
			},
		},
	}

	metadata := buildAuthnMetadata(ctx)

	assert.NotNil(s.T(), metadata)
	assert.Equal(s.T(), "production", metadata.AppMetadata["environment"])
	assert.Equal(s.T(), "1.0.0", metadata.AppMetadata["version"])
	_, hasClientIDs := metadata.AppMetadata["client_ids"]
	assert.False(s.T(), hasClientIDs)
}

func (s *UtilsTestSuite) TestBuildAuthnMetadata_WithOnlyClientIDs() {
	ctx := &providers.NodeContext{
		Application: providers.Application{
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{
					Type: providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{
						ClientID: "single-oauth-client",
					},
				},
			},
		},
	}

	metadata := buildAuthnMetadata(ctx)

	assert.NotNil(s.T(), metadata)
	clientIDs, ok := metadata.AppMetadata["client_ids"].([]string)
	assert.True(s.T(), ok)
	assert.Len(s.T(), clientIDs, 1)
	assert.Equal(s.T(), "single-oauth-client", clientIDs[0])
}

func (s *UtilsTestSuite) TestBuildAuthnMetadata_WithNilOAuthConfig() {
	ctx := &providers.NodeContext{
		Application: providers.Application{
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{
					Type:        providers.OAuthInboundAuthType,
					OAuthConfig: nil,
				},
			},
		},
	}

	metadata := buildAuthnMetadata(ctx)

	assert.NotNil(s.T(), metadata)
	_, hasClientIDs := metadata.AppMetadata["client_ids"]
	assert.False(s.T(), hasClientIDs)
}

func (s *UtilsTestSuite) TestBuildAuthnMetadata_WithEmptyClientID() {
	ctx := &providers.NodeContext{
		Application: providers.Application{
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{
					Type: providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{
						ClientID: "",
					},
				},
			},
		},
	}

	metadata := buildAuthnMetadata(ctx)

	assert.NotNil(s.T(), metadata)
	_, hasClientIDs := metadata.AppMetadata["client_ids"]
	assert.False(s.T(), hasClientIDs)
}

func (s *UtilsTestSuite) TestBuildAuthnMetadata_WithMixedInboundConfigs() {
	ctx := &providers.NodeContext{
		Application: providers.Application{
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{
					Type: providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{
						ClientID: "valid-client",
					},
				},
				{
					Type:        providers.OAuthInboundAuthType,
					OAuthConfig: nil,
				},
				{
					Type: providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{
						ClientID: "",
					},
				},
				{
					Type: providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{
						ClientID: "another-valid-client",
					},
				},
			},
		},
	}

	metadata := buildAuthnMetadata(ctx)

	assert.NotNil(s.T(), metadata)
	clientIDs, ok := metadata.AppMetadata["client_ids"].([]string)
	assert.True(s.T(), ok)
	assert.Len(s.T(), clientIDs, 2)
	assert.Contains(s.T(), clientIDs, "valid-client")
	assert.Contains(s.T(), clientIDs, "another-valid-client")
}

func (s *UtilsTestSuite) TestBuildGetAttributesMetadata_WithLocale() {
	ctx := &providers.NodeContext{
		Application: providers.Application{
			Metadata: map[string]interface{}{
				"tenant_id": "tenant-123",
			},
		},
		RuntimeData: map[string]string{
			"required_locales": "en-US",
		},
	}

	metadata := buildGetAttributesMetadata(ctx)

	assert.NotNil(s.T(), metadata)
	assert.Equal(s.T(), "en-US", metadata.Locale)
	assert.Equal(s.T(), "tenant-123", metadata.AppMetadata["tenant_id"])
}

func (s *UtilsTestSuite) TestBuildGetAttributesMetadata_WithoutLocale() {
	ctx := &providers.NodeContext{
		Application: providers.Application{},
		RuntimeData: map[string]string{},
	}

	metadata := buildGetAttributesMetadata(ctx)

	assert.NotNil(s.T(), metadata)
	assert.Empty(s.T(), metadata.Locale)
	assert.NotNil(s.T(), metadata.AppMetadata)
	assert.Len(s.T(), metadata.AppMetadata, 0)
	assert.NotNil(s.T(), metadata.RuntimeMetadata)
	assert.Equal(s.T(), "", metadata.RuntimeMetadata["authorization_request_id"])
	assert.Equal(s.T(), "", metadata.RuntimeMetadata["current_client_id"])
}

func (s *UtilsTestSuite) TestBuildAuthnMetadata_WithRuntimeMetadata() {
	ctx := &providers.NodeContext{
		Application: providers.Application{},
		RuntimeData: map[string]string{
			common.RuntimeKeyAuthorizationRequestID: "auth-req-123",
			common.RuntimeKeyClientID:               "oauth-client-abc",
			"ext_customKey":                         "custom-value",
			"non_ext_key":                           "should-be-excluded",
		},
	}

	metadata := buildAuthnMetadata(ctx)

	assert.NotContains(s.T(), metadata.AppMetadata, "current_client_id")
	assert.Equal(s.T(), "oauth-client-abc", metadata.RuntimeMetadata["current_client_id"])
	assert.Equal(s.T(), "auth-req-123", metadata.RuntimeMetadata["authorization_request_id"])
	assert.Equal(s.T(), "custom-value", metadata.RuntimeMetadata["ext_customKey"])
	assert.NotContains(s.T(), metadata.RuntimeMetadata, "non_ext_key")
}

func (s *UtilsTestSuite) TestBuildGetAttributesMetadata_WithRuntimeMetadata() {
	ctx := &providers.NodeContext{
		Application: providers.Application{
			Metadata: map[string]interface{}{
				"tenant_id": "tenant-123",
			},
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyAuthorizationRequestID: "auth-req-456",
			common.RuntimeKeyClientID:               "oauth-client-xyz",
			"ext_tenantHint":                        "hint-value",
			"required_locales":                      "en-GB",
			"internal_key":                          "ignored",
		},
	}

	metadata := buildGetAttributesMetadata(ctx)

	assert.Equal(s.T(), "en-GB", metadata.Locale)
	assert.Equal(s.T(), "tenant-123", metadata.AppMetadata["tenant_id"])
	assert.NotContains(s.T(), metadata.AppMetadata, "current_client_id")
	assert.Equal(s.T(), "oauth-client-xyz", metadata.RuntimeMetadata["current_client_id"])
	assert.Equal(s.T(), "auth-req-456", metadata.RuntimeMetadata["authorization_request_id"])
	assert.Equal(s.T(), "hint-value", metadata.RuntimeMetadata["ext_tenantHint"])
	assert.NotContains(s.T(), metadata.RuntimeMetadata, "internal_key")
	assert.NotContains(s.T(), metadata.RuntimeMetadata, "required_locales")
}
