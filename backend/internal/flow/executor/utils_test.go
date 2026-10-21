// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
<<<<<<< HEAD
	"encoding/json"
=======
	"context"
>>>>>>> 2b18ce8b0 (Sync federated users with per-connection sync modes)
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
<<<<<<< HEAD
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
=======
	"github.com/thunder-id/thunderid/internal/entityprovider"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
>>>>>>> 2b18ce8b0 (Sync federated users with per-connection sync modes)
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

// expectEntityReferenceResolved stubs GetEntityReference to resolve authUser to an existing local
// user, modeling account linking that matched an existing local account.
func expectEntityReferenceResolved(m *managermock.AuthnProviderManagerMock, authUser providers.AuthUser) {
	m.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(authUser, &providers.EntityReference{EntityID: "local-user-123"}, (*tidcommon.ServiceError)(nil))
}

// expectEntityReferenceNotFound stubs GetEntityReference to report no matching local user,
// modeling account linking that did not resolve to an existing local account.
func expectEntityReferenceNotFound(m *managermock.AuthnProviderManagerMock, authUser providers.AuthUser) {
	m.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(authUser, (*providers.EntityReference)(nil),
			&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "USER_NOT_FOUND"})
}

// setupSocialAuthExecutorMock creates the shared mocks for social auth executor constructor tests
// (GitHub, Google) and wires the CreateExecutor expectation for the given executor name.
func setupSocialAuthExecutorMock(t *testing.T, executorName string) (
	*coremock.FlowFactoryInterfaceMock,
	*idpmock.IDPServiceInterfaceMock,
	*managermock.AuthnProviderManagerMock,
) {
	t.Helper()
	mockFlowFactory := coremock.NewFlowFactoryInterfaceMock(t)
	mockIDPService := idpmock.NewIDPServiceInterfaceMock(t)
	mockAuthnProvider := managermock.NewAuthnProviderManagerMock(t)
	baseExec := coremock.NewExecutorInterfaceMock(t)
	mockFlowFactory.On("CreateExecutor", executorName,
		providers.ExecutorTypeAuthentication, defaultCodeOnlyInputs, []providers.Input{}, mock.Anything).
		Return(baseExec).Once()
	return mockFlowFactory, mockIDPService, mockAuthnProvider
}

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
		{"MagicLink executor", ExecutorNameMagicLink, authncm.AuthenticatorMagicLink},
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

func (s *UtilsTestSuite) TestInputTypeForSchemaType() {
	tests := []struct {
		name         string
		schemaType   string
		expectedType string
	}{
		{"Boolean attribute is prompted as a checkbox", entitytypemodel.TypeBoolean, providers.InputTypeBoolean},
		{"Number attribute is prompted as a number input", entitytypemodel.TypeNumber, providers.InputTypeNumber},
		{"String attribute is prompted as text", entitytypemodel.TypeString, providers.InputTypeText},
		{"Unknown type falls back to text", "geo", providers.InputTypeText},
		{"Empty type falls back to text", "", providers.InputTypeText},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.expectedType, inputTypeForSchemaType(tt.schemaType))
		})
	}
}

func (s *UtilsTestSuite) TestSchemaTypeForInputType() {
	tests := []struct {
		name         string
		inputType    string
		expectedType string
	}{
		{"Checkbox maps to boolean", providers.InputTypeBoolean, entitytypemodel.TypeBoolean},
		{"Number input maps to number", providers.InputTypeNumber, entitytypemodel.TypeNumber},
		{"Text input needs no conversion", providers.InputTypeText, ""},
		{"Unset input type needs no conversion", "", ""},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.expectedType, schemaTypeForInputType(tt.inputType))
		})
	}
}

// defaultCodeOnlyInputs is the standard default input set for OAuth/OIDC executors that only require an
// authorization code.
var defaultCodeOnlyInputs = []providers.Input{
	{Identifier: "code", Type: "string", Required: true},
}

// createMockAuthExecutor creates a mock executor for OAuth/OIDC authentication.
func createMockAuthExecutor(t *testing.T, executorName string) providers.Executor {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(executorName).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return(defaultCodeOnlyInputs).Maybe()
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
		idpID                string
		attributeConfig      *providers.AttributeConfiguration
		noAttributeConfig    bool
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
			name: "Sub under its own name in RuntimeData, UserInputs or user attributes is not compared",
			federatedIdentifiers: map[string]interface{}{
				"email": "user@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{
				"sub": "sub789",
			},
			ctx: &providers.NodeContext{
				RuntimeData: map[string]string{
					"sub": "sub456",
				},
				UserInputs: map[string]string{
					"sub": "sub456",
				},
			},
			expectedValid: true,
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
			name:  "Same connection and sub as an earlier external identity returns true",
			idpID: "idp-first",
			federatedIdentifiers: map[string]interface{}{
				"sub": "sub-first",
			},
			ctx: &providers.NodeContext{
				RuntimeData: map[string]string{
					common.RuntimeKeyExternalIdentity: externalIdentityEntry("idp-first", "sub-first",
						map[string]interface{}{"sub": "sub-first"}),
				},
			},
			expectedValid: true,
		},
		{
			name:  "Same connection with a different sub than an earlier external identity returns false",
			idpID: "idp-first",
			federatedIdentifiers: map[string]interface{}{
				"sub": "sub-second",
			},
			ctx: &providers.NodeContext{
				RuntimeData: map[string]string{
					common.RuntimeKeyExternalIdentity: externalIdentityEntry("idp-first", "sub-first",
						map[string]interface{}{"sub": "sub-first"}),
				},
			},
			expectedValid: false,
		},
		{
			name:  "Different connection with the same sub as an earlier external identity returns false",
			idpID: "idp-second",
			federatedIdentifiers: map[string]interface{}{
				"sub": "sub-first",
			},
			ctx: &providers.NodeContext{
				RuntimeData: map[string]string{
					common.RuntimeKeyExternalIdentity: externalIdentityEntry("idp-first", "sub-first",
						map[string]interface{}{"sub": "sub-first"}),
				},
			},
			expectedValid: false,
		},
		{
			name:  "Different connection and sub than an earlier external identity returns false",
			idpID: "idp-second",
			federatedIdentifiers: map[string]interface{}{
				"sub": "sub-second",
			},
			ctx: &providers.NodeContext{
				RuntimeData: map[string]string{
					common.RuntimeKeyExternalIdentity: externalIdentityEntry("idp-first", "sub-first",
						map[string]interface{}{"sub": "sub-first"}),
				},
			},
			expectedValid: false,
		},
		{
			name:  "Earlier external identity without a subject, as OpenID4VP sets it, is not compared by sub",
			idpID: "idp-first",
			federatedIdentifiers: map[string]interface{}{
				"sub": "sub-first",
			},
			ctx: &providers.NodeContext{
				RuntimeData: map[string]string{
					common.RuntimeKeyExternalIdentity: externalIdentityEntry("", "",
						map[string]interface{}{"sub": "did:example:wallet"}),
				},
			},
			expectedValid: true,
		},
		{
			name:  "Email mismatch with an earlier external identity's claims returns false",
			idpID: "idp-first",
			federatedIdentifiers: map[string]interface{}{
				"email": "second@example.com",
				"sub":   "sub-first",
			},
			ctx: &providers.NodeContext{
				RuntimeData: map[string]string{
					common.RuntimeKeyExternalIdentity: externalIdentityEntry("idp-first", "sub-first",
						map[string]interface{}{"email": "first@example.com"}),
				},
			},
			expectedValid: false,
		},
		{
			name: "Mismatch on the local attribute a custom linking attribute maps to returns false",
			attributeConfig: &providers.AttributeConfiguration{
				AccountLinking:     &providers.AccountLinking{Attributes: []string{"phone_number"}},
				UserTypeResolution: &providers.UserTypeResolution{Default: "customer"},
				UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{
					UserType: "customer",
					Attributes: []providers.AttributeMapping{
						{ExternalAttribute: "phone_number", LocalAttribute: "mobileNumber"},
					},
				}},
			},
			federatedIdentifiers: map[string]interface{}{
				"phone_number": "+15550001",
				"mobileNumber": "+15550001",
				"sub":          "sub123",
			},
			existingIdentifiers: map[string]interface{}{
				"mobileNumber": "+15550002",
			},
			ctx:           &providers.NodeContext{},
			expectedValid: false,
		},
		{
			name: "Match on the local attribute a custom linking attribute maps to returns true",
			attributeConfig: &providers.AttributeConfiguration{
				AccountLinking:     &providers.AccountLinking{Attributes: []string{"phone_number"}},
				UserTypeResolution: &providers.UserTypeResolution{Default: "customer"},
				UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{
					UserType: "customer",
					Attributes: []providers.AttributeMapping{
						{ExternalAttribute: "phone_number", LocalAttribute: "mobileNumber"},
					},
				}},
			},
			federatedIdentifiers: map[string]interface{}{
				"phone_number": "+15550001",
				"mobileNumber": "+15550001",
				"sub":          "sub123",
			},
			ctx: &providers.NodeContext{
				UserInputs: map[string]string{"mobileNumber": "+15550001"},
			},
			expectedValid: true,
		},
		{
			name: "Email mismatch is ignored when email is not a linking attribute",
			attributeConfig: &providers.AttributeConfiguration{
				AccountLinking: &providers.AccountLinking{Attributes: []string{"username"}},
			},
			federatedIdentifiers: map[string]interface{}{
				"email":    "user1@example.com",
				"username": "user1",
				"sub":      "sub123",
			},
			existingIdentifiers: map[string]interface{}{
				"email":    "user2@example.com",
				"username": "user1",
			},
			ctx:           &providers.NodeContext{},
			expectedValid: true,
		},
		{
			name:            "Email mismatch fails when the connection has no account linking",
			attributeConfig: &providers.AttributeConfiguration{},
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
			name: "Email mismatch fails when the connection has no account-linking attributes",
			attributeConfig: &providers.AttributeConfiguration{
				AccountLinking: &providers.AccountLinking{},
			},
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
			name:              "Missing federated email passes when the connection has no attribute configuration",
			noAttributeConfig: true,
			federatedIdentifiers: map[string]interface{}{
				"sub": "sub123",
			},
			existingIdentifiers: map[string]interface{}{
				"email": "user2@example.com",
			},
			ctx:           &providers.NodeContext{},
			expectedValid: true,
		},
		{
			name:              "Email mismatch fails when the connection has no attribute configuration",
			noAttributeConfig: true,
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
			name:              "Matching email passes when the connection has no attribute configuration",
			noAttributeConfig: true,
			federatedIdentifiers: map[string]interface{}{
				"email": "user1@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{
				"email": "user1@example.com",
			},
			ctx:           &providers.NodeContext{},
			expectedValid: true,
		},
		{
			name:            "Sub mismatch still fails when the connection has no account linking",
			idpID:           "idp-first",
			attributeConfig: &providers.AttributeConfiguration{},
			federatedIdentifiers: map[string]interface{}{
				"sub": "sub-second",
			},
			ctx: &providers.NodeContext{
				RuntimeData: map[string]string{
					common.RuntimeKeyExternalIdentity: externalIdentityEntry("idp-first", "sub-first", nil),
				},
			},
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Unless a case says otherwise, the connection links on email, as seeded connections do.
			attributeConfig := tt.attributeConfig
			if attributeConfig == nil && !tt.noAttributeConfig {
				attributeConfig = &providers.AttributeConfiguration{
					AccountLinking: &providers.AccountLinking{Attributes: []string{"email"}},
				}
			}
			idpDTO := &providers.IDPDTO{ID: tt.idpID, AttributeConfiguration: attributeConfig}
			valid := validateFederatedIdentifierConsistency(tt.ctx, idpDTO, tt.federatedIdentifiers,
				tt.existingIdentifiers)
			s.Equal(tt.expectedValid, valid)
		})
	}
}

<<<<<<< HEAD
// externalIdentityEntry encodes an external identity the way the federated executors set it.
func externalIdentityEntry(idpID, sub string, claims map[string]interface{}) string {
	encoded, err := json.Marshal(core.ExternalIdentity{IdpID: idpID, Sub: sub, Claims: claims})
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// externalSub reads the subject back from the external identity entry.
func externalSub(runtimeData map[string]string) string {
	if identity := core.GetExternalIdentity(runtimeData); identity != nil {
		return identity.Sub
	}
	return ""
}

// externalClaim reads one claim back from the external identity entry.
func externalClaim(runtimeData map[string]string, name string) string {
	value, _ := core.GetExternalClaim(runtimeData, name)
	return value
}

// Claims are stored only inside the external identity entry, so a claim sharing a name with flow
// control state never lands under that name.
func (s *UtilsTestSuite) TestSetExternalIdentity_KeepsClaimsOutOfFlowState() {
	execResp := &providers.ExecutorResponse{RuntimeData: map[string]string{}}

	err := setExternalIdentity(execResp, "idp-1", "sub-123", map[string]interface{}{
		"sub":                             "sub-123",
		"email":                           "user@example.com",
		"groups":                          []interface{}{"eng", "ops"},
		userAttributeUserID:               "victim-id",
		ouIDKey:                           "other-ou",
		categoryTypeKey:                   "admin",
		common.RuntimeKeyEntityState:      "exists",
		common.RuntimeKeyExternalIdentity: "forged",
	})

	s.NoError(err)
	s.Len(execResp.RuntimeData, 1)
	identity := core.GetExternalIdentity(execResp.RuntimeData)
	s.NotNil(identity)
	s.Equal("idp-1", identity.IdpID)
	s.Equal("sub-123", identity.Sub)
	s.Equal([]interface{}{"eng", "ops"}, identity.Claims["groups"])
	s.Equal("victim-id", identity.Claims[userAttributeUserID])
}

// Token metadata describes the token, not the user, so it is never kept as a claim. The sub claim
// is dropped only when it is set as the subject, and the caller's map is left untouched.
func (s *UtilsTestSuite) TestSetExternalIdentity_DropsTokenMetadata() {
	claims := map[string]interface{}{
		"sub": "sub-123", "aud": "client", "exp": float64(1), "iat": float64(1), "nbf": float64(1),
		"iss": "https://idp", "jti": "j", "at_hash": "a", "c_hash": "c", "azp": "client", "nonce": "n",
		"email": "user@example.com",
	}
	execResp := &providers.ExecutorResponse{}

	s.NoError(setExternalIdentity(execResp, "idp-1", "sub-123", claims))

	identity := core.GetExternalIdentity(execResp.RuntimeData)
	s.Equal("sub-123", identity.Sub)
	s.Equal(map[string]interface{}{"email": "user@example.com"}, identity.Claims)
	s.Len(claims, 12)

	s.NoError(setExternalIdentity(execResp, "", "", map[string]interface{}{"sub": "holder"}))
	s.Equal("holder", core.GetExternalIdentity(execResp.RuntimeData).Claims["sub"])
}

// A later sign-in replaces the earlier entry whole, so claims from two providers never mix.
func (s *UtilsTestSuite) TestSetExternalIdentity_ReplacesEarlierEntry() {
	execResp := &providers.ExecutorResponse{RuntimeData: map[string]string{
		common.RuntimeKeyExternalIdentity: externalIdentityEntry("idp-first", "sub-first",
			map[string]interface{}{"given_name": "First"}),
	}}

	err := setExternalIdentity(execResp, "idp-second", "sub-second",
		map[string]interface{}{"email": "second@example.com"})

	s.NoError(err)
	identity := core.GetExternalIdentity(execResp.RuntimeData)
	s.Equal("idp-second", identity.IdpID)
	s.Equal(map[string]interface{}{"email": "second@example.com"}, identity.Claims)
}

// setFederatedEntityState records whether the federated identity resolved to one user, several or
// none.
func (s *UtilsTestSuite) TestSetFederatedEntityState() {
	tests := []struct {
		name      string
		ref       *providers.EntityReference
		svcErr    *tidcommon.ServiceError
		wantState string
	}{
		{name: "one user", ref: &providers.EntityReference{EntityID: "user-1"}, wantState: entityStateExists},
		{name: "no user", svcErr: &authnprovidermgr.ErrorUserNotFound, wantState: entityStateNotExists},
		{name: "several users", svcErr: &authnprovidermgr.ErrorAmbiguousUser, wantState: entityStateAmbiguous},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			authnProvider := managermock.NewAuthnProviderManagerMock(s.T())
			authnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
				Return(newAuthenticatedAuthUser(), tc.ref, tc.svcErr)
			execResp := &providers.ExecutorResponse{
				AuthUser:    newAuthenticatedAuthUser(),
				RuntimeData: map[string]string{},
			}

			setFederatedEntityState(s.T().Context(), execResp, authnProvider)

			s.Equal(tc.wantState, execResp.RuntimeData[common.RuntimeKeyEntityState])
=======
func (s *UtilsTestSuite) TestResolveAndSetMappedAuthorizationTargets_SetsSyncModes() {
	mockIDPService := idpmock.NewIDPServiceInterfaceMock(s.T())
	mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").Return(&providers.IDPDTO{
		ID: "idp-123",
		AttributeConfiguration: &providers.AttributeConfiguration{
			Sync: &providers.SyncConfiguration{GroupsAndRoles: providers.SyncModeSessionOnly},
		},
	}, (*tidcommon.ServiceError)(nil))
	mockIDPService.On("ResolveDirectAuthorizationTargets", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil, (*tidcommon.ServiceError)(nil))
	execResp := &providers.ExecutorResponse{}

	_, _, err := resolveAndSetMappedAuthorizationTargets(context.Background(), execResp, mockIDPService,
		nil, "idp-123", nil, log.GetLogger())

	s.NoError(err)
	s.Equal(string(providers.SyncModeSessionOnly), execResp.RuntimeData[common.RuntimeKeyGroupsAndRolesSyncMode])
	s.Equal(string(providers.SyncModeOverrideReceived), execResp.RuntimeData[common.RuntimeKeyAttributesSyncMode])
}

func (s *UtilsTestSuite) TestResolveAndSetMappedAuthorizationTargets_ExpandsSessionGroupsOnce() {
	mockIDPService := idpmock.NewIDPServiceInterfaceMock(s.T())
	mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").Return(&providers.IDPDTO{
		ID: "idp-123",
		AttributeConfiguration: &providers.AttributeConfiguration{
			Sync: &providers.SyncConfiguration{GroupsAndRoles: providers.SyncModeSessionOnly},
		},
	}, (*tidcommon.ServiceError)(nil))
	mockIDPService.On("ResolveDirectAuthorizationTargets", mock.Anything, mock.Anything, mock.Anything).
		Return([]providers.AuthorizationTarget{{Type: providers.AuthorizationTargetGroup, ID: "child"}}, nil,
			(*tidcommon.ServiceError)(nil))
	mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	mockEntityProvider.On("GetTransitiveGroupAncestors", "child").Return([]string{"parent"}, nil).Once()
	execResp := &providers.ExecutorResponse{}

	_, _, err := resolveAndSetMappedAuthorizationTargets(context.Background(), execResp, mockIDPService,
		mockEntityProvider, "idp-123", nil, log.GetLogger())

	s.NoError(err)
	s.Equal("child", execResp.RuntimeData[common.RuntimeKeyMappedGroupIDs])
	s.Equal("child parent", execResp.RuntimeData[common.RuntimeKeySessionGroupIDs])
}

// A declarative-only store can't hold mapped memberships, so outside session only its mapped groups or
// roles are kept to the session instead.
func (s *UtilsTestSuite) TestResolveAndSetMappedAuthorizationTargets_DeclarativeStoresKeepToSession() {
	tests := []struct {
		name            string
		groupStore      string
		roleStore       string
		expectedGroups  string
		expectedRoles   string
		expandsAncestor bool
	}{
		{"mutable stores", "mutable", "mutable", "", "", false},
		{"declarative groups", "declarative", "mutable", "child parent", "", true},
		{"declarative roles", "mutable", "declarative", "", "role-1", false},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{
				Group: config.GroupConfig{Store: tc.groupStore},
				Role:  config.RoleConfig{Store: tc.roleStore},
			}))
			defer config.ResetServerRuntime()

			mockIDPService := idpmock.NewIDPServiceInterfaceMock(s.T())
			mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").Return(&providers.IDPDTO{
				ID: "idp-123",
				AttributeConfiguration: &providers.AttributeConfiguration{
					Sync: &providers.SyncConfiguration{GroupsAndRoles: providers.SyncModeNoSync},
				},
			}, (*tidcommon.ServiceError)(nil))
			mockIDPService.On("ResolveDirectAuthorizationTargets", mock.Anything, mock.Anything, mock.Anything).
				Return([]providers.AuthorizationTarget{
					{Type: providers.AuthorizationTargetGroup, ID: "child"},
					{Type: providers.AuthorizationTargetRole, ID: "role-1"},
				}, nil, (*tidcommon.ServiceError)(nil))
			mockEntityProvider := entityprovidermock.NewEntityProviderInterfaceMock(s.T())
			if tc.expandsAncestor {
				mockEntityProvider.On("GetTransitiveGroupAncestors", "child").Return([]string{"parent"}, nil).Once()
			}
			execResp := &providers.ExecutorResponse{}

			_, _, err := resolveAndSetMappedAuthorizationTargets(context.Background(), execResp, mockIDPService,
				mockEntityProvider, "idp-123", nil, log.GetLogger())

			s.NoError(err)
			s.Equal(tc.expectedGroups, execResp.RuntimeData[common.RuntimeKeySessionGroupIDs])
			s.Equal(tc.expectedRoles, execResp.RuntimeData[common.RuntimeKeySessionRoleIDs])
>>>>>>> 2b18ce8b0 (Sync federated users with per-connection sync modes)
		})
	}
}

<<<<<<< HEAD
func (s *UtilsTestSuite) TestStatedHold_StatesOnlyTheManagerHoldCodes() {
	s.Nil(statedHold(nil))
	s.Nil(statedHold(&tidcommon.ErrorUnauthorized))
	s.Equal(&ErrAccountLocked, statedHold(&authnprovidermgr.ErrorAccountLocked))
	s.Equal(&ErrAccountSuspended, statedHold(&authnprovidermgr.ErrorAccountSuspended))
}

func (s *UtilsTestSuite) TestStatedDecision_StatesOnlyANamedHold() {
	s.Nil(statedDecision(providers.AccessDecision{}))
	s.Nil(statedDecision(providers.AccessDecision{Admitted: true}))
	s.Equal(&ErrAccountLocked, statedDecision(providers.AccessDecision{Reason: providers.AccessRefusalReasonLocked}))
	s.Equal(&ErrAccountSuspended,
		statedDecision(providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended}))
=======
func (s *UtilsTestSuite) TestResolveAndSetMappedAuthorizationTargets_ContinuesWithoutDirectTargets() {
	mockIDPService := idpmock.NewIDPServiceInterfaceMock(s.T())
	mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return(&providers.IDPDTO{ID: "idp-123"}, (*tidcommon.ServiceError)(nil))
	mockIDPService.On("ResolveDirectAuthorizationTargets", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil, &tidcommon.ServiceError{Type: tidcommon.ServerErrorType})
	execResp := &providers.ExecutorResponse{}

	_, resolved, err := resolveAndSetMappedAuthorizationTargets(context.Background(), execResp, mockIDPService,
		nil, "idp-123", nil, log.GetLogger())

	s.NoError(err)
	s.False(resolved.Complete)
}

func (s *UtilsTestSuite) TestResolveAndSetMappedAuthorizationTargets_FailsWhenIDPUnresolved() {
	mockIDPService := idpmock.NewIDPServiceInterfaceMock(s.T())
	mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").Return(nil, &tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Error: tidcommon.I18nMessage{DefaultValue: "store unavailable"},
	})
	execResp := &providers.ExecutorResponse{}

	_, _, err := resolveAndSetMappedAuthorizationTargets(context.Background(), execResp, mockIDPService,
		nil, "idp-123", nil, log.GetLogger())

	s.Error(err)
	s.NotContains(execResp.RuntimeData, common.RuntimeKeyAttributesSyncMode)
}

func (s *UtilsTestSuite) TestCopyFederatedAttributesToRuntimeData_SkipsSyncModes() {
	execResp := &providers.ExecutorResponse{}

	copyFederatedAttributesToRuntimeData(execResp, map[string]interface{}{
		common.RuntimeKeyGroupsAndRolesSyncMode: "no_sync",
		common.RuntimeKeyAttributesSyncMode:     "no_sync",
		"email":                                 "user@example.com",
	})

	s.NotContains(execResp.RuntimeData, common.RuntimeKeyGroupsAndRolesSyncMode)
	s.NotContains(execResp.RuntimeData, common.RuntimeKeyAttributesSyncMode)
	s.Equal("user@example.com", execResp.RuntimeData["email"])
}

// federatedSignInSyncCase is a returning user's federated sign-in through an OAuth or OIDC executor.
type federatedSignInSyncCase struct {
	name       string
	flowType   providers.FlowType
	idpErr     *tidcommon.ServiceError
	syncErr    *entityprovider.EntityProviderError
	wantSync   bool
	wantStatus providers.ExecutorStatus
	wantErr    bool
}

var federatedSignInSyncCases = []federatedSignInSyncCase{
	{name: "sign-in syncs the returning user", flowType: providers.FlowTypeAuthentication,
		wantSync: true, wantStatus: providers.ExecComplete},
	{name: "registration that finds the user syncs it", flowType: providers.FlowTypeRegistration,
		wantSync: true, wantStatus: providers.ExecComplete},
	{name: "a rejected sync fails sign-in", flowType: providers.FlowTypeAuthentication, wantSync: true,
		syncErr:    entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSyncRejected, "Sync rejected", ""),
		wantStatus: providers.ExecFailure},
	{name: "a server error during sync is returned", flowType: providers.FlowTypeAuthentication, wantSync: true,
		syncErr: entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "System error", ""),
		wantErr: true},
	{name: "an unresolved IDP is returned", flowType: providers.FlowTypeAuthentication,
		idpErr: &tidcommon.ServiceError{Type: tidcommon.ServerErrorType}, wantErr: true},
}

// runFederatedSignInSyncCase signs in a returning user whose provider sends a mapped email, through
// process, and checks the account is synced as tc expects.
func runFederatedSignInSyncCase(
	t *testing.T, tc federatedSignInSyncCase, authn *managermock.AuthnProviderManagerMock,
	idps *idpmock.IDPServiceInterfaceMock, entities *entityprovidermock.EntityProviderInterfaceMock,
	process func(*providers.NodeContext, *providers.ExecutorResponse) error,
) {
	authUser := newOAuthAuthenticatedUser()
	authn.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(authUser, providers.AuthenticatedClaims{
		"sub": "existing-user-sub", "email": "new@example.com",
	}, (*tidcommon.ServiceError)(nil))
	idps.On("GetIdentityProvider", mock.Anything, "idp-123").Return(&providers.IDPDTO{
		ID: "idp-123",
		AttributeConfiguration: &providers.AttributeConfiguration{
			UserTypeResolution: &providers.UserTypeResolution{Default: "Person"},
			UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{
				UserType: "Person", Attributes: []providers.AttributeMapping{
					{ExternalAttribute: "email", LocalAttribute: "email"},
				},
			}},
		},
	}, tc.idpErr)
	if tc.idpErr == nil {
		expectEntityReferenceResolved(authn, authUser)
		idps.On("ResolveDirectAuthorizationTargets", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil, (*tidcommon.ServiceError)(nil))
	}
	if tc.wantSync {
		entities.On("SyncFederatedEntity", "local-user-123", providers.FederatedEntitySync{
			AttributesByType: map[string]map[string]interface{}{"Person": {"email": "new@example.com"}},
		}).Return(tc.syncErr).Once()
	}
	ctx := &providers.NodeContext{
		Context:        context.Background(),
		ExecutionID:    "flow-123",
		FlowType:       tc.flowType,
		UserInputs:     map[string]string{"code": "auth_code_123"},
		NodeProperties: map[string]interface{}{"idpId": "idp-123", "allowRegistrationWithExistingUser": true},
	}
	execResp := &providers.ExecutorResponse{AdditionalData: map[string]string{}, RuntimeData: map[string]string{}}

	err := process(ctx, execResp)

	if tc.wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	assert.Equal(t, tc.wantStatus, execResp.Status)
	if tc.wantStatus == providers.ExecFailure {
		assert.Equal(t, &ErrFederatedUserSyncFailed, execResp.Error)
	}
>>>>>>> 2b18ce8b0 (Sync federated users with per-connection sync modes)
}
