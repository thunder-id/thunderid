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

package common

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/idp"
)

type AuthenticatorTestSuite struct {
	suite.Suite
}

func TestAuthenticatorTestSuite(t *testing.T) {
	suite.Run(t, new(AuthenticatorTestSuite))
}

// SetupTest runs before each test to initialize the registry with test data
func (suite *AuthenticatorTestSuite) SetupTest() {
	// Register all authenticators for testing
	RegisterAuthenticator(AuthenticatorMeta{
		Name:    AuthenticatorCredentials,
		Factors: []AuthenticationFactor{FactorKnowledge},
	})
	RegisterAuthenticator(AuthenticatorMeta{
		Name:    AuthenticatorSMSOTP,
		Factors: []AuthenticationFactor{FactorPossession},
	})
	RegisterAuthenticator(AuthenticatorMeta{
		Name:          AuthenticatorGoogle,
		Factors:       []AuthenticationFactor{FactorKnowledge},
		AssociatedIDP: idp.IDPTypeGoogle,
	})
	RegisterAuthenticator(AuthenticatorMeta{
		Name:          AuthenticatorGithub,
		Factors:       []AuthenticationFactor{FactorKnowledge},
		AssociatedIDP: idp.IDPTypeGitHub,
	})
	RegisterAuthenticator(AuthenticatorMeta{
		Name:          AuthenticatorOAuth,
		Factors:       []AuthenticationFactor{FactorKnowledge},
		AssociatedIDP: idp.IDPTypeOAuth,
	})
	RegisterAuthenticator(AuthenticatorMeta{
		Name:          AuthenticatorOIDC,
		Factors:       []AuthenticationFactor{FactorKnowledge},
		AssociatedIDP: idp.IDPTypeOIDC,
	})
}

func (suite *AuthenticatorTestSuite) TestGetAuthenticatorMetaData() {
	testCases := []struct {
		name              string
		authenticator     string
		expectNil         bool
		expectedName      string
		expectedAALWeight int
	}{
		{
			name:              "Credentials authenticator",
			authenticator:     AuthenticatorCredentials,
			expectNil:         false,
			expectedName:      AuthenticatorCredentials,
			expectedAALWeight: 1,
		},
		{
			name:              "SMS OTP authenticator",
			authenticator:     AuthenticatorSMSOTP,
			expectNil:         false,
			expectedName:      AuthenticatorSMSOTP,
			expectedAALWeight: 1,
		},
		{
			name:              "Google authenticator",
			authenticator:     AuthenticatorGoogle,
			expectNil:         false,
			expectedName:      AuthenticatorGoogle,
			expectedAALWeight: 1,
		},
		{
			name:              "GitHub authenticator",
			authenticator:     AuthenticatorGithub,
			expectNil:         false,
			expectedName:      AuthenticatorGithub,
			expectedAALWeight: 1,
		},
		{
			name:              "OAuth authenticator",
			authenticator:     AuthenticatorOAuth,
			expectNil:         false,
			expectedName:      AuthenticatorOAuth,
			expectedAALWeight: 1,
		},
		{
			name:              "OIDC authenticator",
			authenticator:     AuthenticatorOIDC,
			expectNil:         false,
			expectedName:      AuthenticatorOIDC,
			expectedAALWeight: 1,
		},
		{
			name:          "Unknown authenticator",
			authenticator: "UnknownAuthenticator",
			expectNil:     true,
		},
		{
			name:          "Empty authenticator name",
			authenticator: "",
			expectNil:     true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := getAuthenticatorMetaData(tc.authenticator)

			if tc.expectNil {
				suite.Nil(result)
			} else {
				suite.NotNil(result)
				suite.Equal(tc.expectedName, result.Name)
				suite.NotEmpty(result.Factors)
			}
		})
	}
}

func (suite *AuthenticatorTestSuite) TestGetAuthenticatorFactors() {
	testCases := []struct {
		name            string
		authenticator   string
		expectedFactors []AuthenticationFactor
	}{
		{
			name:            "Credentials authenticator",
			authenticator:   AuthenticatorCredentials,
			expectedFactors: []AuthenticationFactor{FactorKnowledge},
		},
		{
			name:            "SMS OTP authenticator",
			authenticator:   AuthenticatorSMSOTP,
			expectedFactors: []AuthenticationFactor{FactorPossession},
		},
		{
			name:            "Google authenticator",
			authenticator:   AuthenticatorGoogle,
			expectedFactors: []AuthenticationFactor{FactorKnowledge},
		},
		{
			name:            "GitHub authenticator",
			authenticator:   AuthenticatorGithub,
			expectedFactors: []AuthenticationFactor{FactorKnowledge},
		},
		{
			name:            "OAuth authenticator",
			authenticator:   AuthenticatorOAuth,
			expectedFactors: []AuthenticationFactor{FactorKnowledge},
		},
		{
			name:            "OIDC authenticator",
			authenticator:   AuthenticatorOIDC,
			expectedFactors: []AuthenticationFactor{FactorKnowledge},
		},
		{
			name:            "Unknown authenticator returns nil",
			authenticator:   "UnknownAuthenticator",
			expectedFactors: []AuthenticationFactor{},
		},
		{
			name:            "Empty authenticator name returns nil",
			authenticator:   "",
			expectedFactors: []AuthenticationFactor{},
		},
		{
			name:            "Random string returns nil",
			authenticator:   "RandomString123",
			expectedFactors: []AuthenticationFactor{},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := GetAuthenticatorFactors(tc.authenticator)
			suite.Equal(tc.expectedFactors, result)
		})
	}
}

func (suite *AuthenticatorTestSuite) TestGetAuthenticatorNameForIDPType() {
	testCases := []struct {
		name             string
		idpType          idp.IDPType
		expectedAuthName string
		expectError      bool
	}{
		{
			name:             "Google IDP type",
			idpType:          idp.IDPTypeGoogle,
			expectedAuthName: AuthenticatorGoogle,
			expectError:      false,
		},
		{
			name:             "GitHub IDP type",
			idpType:          idp.IDPTypeGitHub,
			expectedAuthName: AuthenticatorGithub,
			expectError:      false,
		},
		{
			name:             "OAuth IDP type",
			idpType:          idp.IDPTypeOAuth,
			expectedAuthName: AuthenticatorOAuth,
			expectError:      false,
		},
		{
			name:             "OIDC IDP type",
			idpType:          idp.IDPTypeOIDC,
			expectedAuthName: AuthenticatorOIDC,
			expectError:      false,
		},
		{
			name:             "Unknown IDP type defaults to OAuth",
			idpType:          "UnknownIDPType",
			expectedAuthName: "",
			expectError:      true,
		},
		{
			name:             "Empty IDP type defaults to OAuth",
			idpType:          "",
			expectedAuthName: "",
			expectError:      true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result, err := GetAuthenticatorNameForIDPType(tc.idpType)
			if tc.expectError {
				suite.Error(err)
			} else {
				suite.NoError(err)
				suite.Equal(tc.expectedAuthName, result)
			}
		})
	}
}

func (suite *AuthenticatorTestSuite) TestAuthenticatorRegistry() {
	suite.Run("Registry contains all expected authenticators", func() {
		expectedAuthenticators := []string{
			AuthenticatorCredentials,
			AuthenticatorSMSOTP,
			AuthenticatorGoogle,
			AuthenticatorGithub,
			AuthenticatorOAuth,
			AuthenticatorOIDC,
		}

		registryMu.RLock()
		registrySize := len(authenticatorRegistry)
		registryMu.RUnlock()

		suite.Equal(len(expectedAuthenticators), registrySize)

		for _, auth := range expectedAuthenticators {
			meta := getAuthenticatorMetaData(auth)
			suite.NotNil(meta, "Authenticator %s should exist in registry", auth)
			suite.Equal(auth, meta.Name)
			suite.NotEmpty(meta.Factors, "Authenticator %s should have factors", auth)
		}
	})
}

func (suite *AuthenticatorTestSuite) TestAuthenticatorMetaStructure() {
	suite.Run("AuthenticatorMeta has correct fields", func() {
		meta := AuthenticatorMeta{
			Name:    "TestAuthenticator",
			Factors: []AuthenticationFactor{FactorKnowledge, FactorPossession},
		}

		suite.Equal("TestAuthenticator", meta.Name)
		suite.Len(meta.Factors, 2)
		suite.Contains(meta.Factors, FactorKnowledge)
		suite.Contains(meta.Factors, FactorPossession)
	})
}

func (suite *AuthenticatorTestSuite) TestAuthenticatorReferenceStructure() {
	suite.Run("AuthenticatorReference has correct fields", func() {
		ref := AuthenticatorReference{
			Authenticator: AuthenticatorCredentials,
			Step:          1,
		}

		suite.Equal(AuthenticatorCredentials, ref.Authenticator)
		suite.Equal(1, ref.Step)
		suite.NotNil(ref.Timestamp)
	})
}

func (suite *AuthenticatorTestSuite) TestAuthenticatorConstants() {
	testCases := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "Credentials authenticator constant",
			constant: AuthenticatorCredentials,
			expected: "CredentialsAuthenticator",
		},
		{
			name:     "SMS OTP authenticator constant",
			constant: AuthenticatorSMSOTP,
			expected: "SMSOTPAuthenticator",
		},
		{
			name:     "Google authenticator constant",
			constant: AuthenticatorGoogle,
			expected: "GoogleOIDCAuthenticator",
		},
		{
			name:     "GitHub authenticator constant",
			constant: AuthenticatorGithub,
			expected: "GithubOAuthAuthenticator",
		},
		{
			name:     "OAuth authenticator constant",
			constant: AuthenticatorOAuth,
			expected: "OAuthAuthenticator",
		},
		{
			name:     "OIDC authenticator constant",
			constant: AuthenticatorOIDC,
			expected: "OIDCAuthenticator",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.Equal(tc.expected, tc.constant)
		})
	}
}
