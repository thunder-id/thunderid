// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
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

func (s *UtilsTestSuite) TestConvertToSchemaType() {
	tests := []struct {
		name          string
		value         string
		schemaType    string
		expectedValue interface{}
	}{
		{"Checked box becomes true", "true", entitytypemodel.TypeBoolean, true},
		{"Unchecked box becomes false", "false", entitytypemodel.TypeBoolean, false},
		{"Boolean accepts alternate spellings", "TRUE", entitytypemodel.TypeBoolean, true},
		{"Unparseable boolean is left for schema validation to reject", "yes", entitytypemodel.TypeBoolean, "yes"},
		{"Number becomes a float", "42", entitytypemodel.TypeNumber, float64(42)},
		{"Fractional number becomes a float", "1.5", entitytypemodel.TypeNumber, 1.5},
		{"Unparseable number is left for schema validation to reject", "many", entitytypemodel.TypeNumber, "many"},
		{"String attribute is untouched", "true", entitytypemodel.TypeString, "true"},
		{"Unknown schema type is untouched", "true", "", "true"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.expectedValue, convertToSchemaType(tt.value, tt.schemaType))
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
			name:            "Email mismatch is ignored when the connection has no account linking",
			attributeConfig: &providers.AttributeConfiguration{},
			federatedIdentifiers: map[string]interface{}{
				"email": "user1@example.com",
				"sub":   "sub123",
			},
			existingIdentifiers: map[string]interface{}{
				"email": "user2@example.com",
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
			if attributeConfig == nil {
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
