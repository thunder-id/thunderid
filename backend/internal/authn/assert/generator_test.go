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

package assert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type GeneratorTestSuite struct {
	suite.Suite
	generator AuthAssertGeneratorInterface
}

func TestGeneratorTestSuite(t *testing.T) {
	suite.Run(t, new(GeneratorTestSuite))
}

func (suite *GeneratorTestSuite) SetupTest() {
	suite.generator = newAuthAssertGenerator()

	// Register all authenticators
	authncm.RegisterAuthenticator(authncm.AuthenticatorMeta{
		Name:    authncm.AuthenticatorCredentials,
		Factors: []authncm.AuthenticationFactor{authncm.FactorKnowledge},
	})
	authncm.RegisterAuthenticator(authncm.AuthenticatorMeta{
		Name:    authncm.AuthenticatorSMSOTP,
		Factors: []authncm.AuthenticationFactor{authncm.FactorPossession},
	})
	authncm.RegisterAuthenticator(authncm.AuthenticatorMeta{
		Name:          authncm.AuthenticatorGoogle,
		Factors:       []authncm.AuthenticationFactor{authncm.FactorKnowledge},
		AssociatedIDP: idp.IDPTypeGoogle,
	})
	authncm.RegisterAuthenticator(authncm.AuthenticatorMeta{
		Name:          authncm.AuthenticatorGithub,
		Factors:       []authncm.AuthenticationFactor{authncm.FactorKnowledge},
		AssociatedIDP: idp.IDPTypeGitHub,
	})
	authncm.RegisterAuthenticator(authncm.AuthenticatorMeta{
		Name:          authncm.AuthenticatorOAuth,
		Factors:       []authncm.AuthenticationFactor{authncm.FactorKnowledge},
		AssociatedIDP: idp.IDPTypeOAuth,
	})
	authncm.RegisterAuthenticator(authncm.AuthenticatorMeta{
		Name:          authncm.AuthenticatorOIDC,
		Factors:       []authncm.AuthenticationFactor{authncm.FactorKnowledge},
		AssociatedIDP: idp.IDPTypeOIDC,
	})
}

func (suite *GeneratorTestSuite) TestGenerateAssertionSingleAuthenticator() {
	testCases := []struct {
		name          string
		authenticator string
		expectedAAL   AssuranceLevel
		expectedIAL   AssuranceLevel
	}{
		{
			name:          "Credentials authenticator",
			authenticator: authncm.AuthenticatorCredentials,
			expectedAAL:   AALLevel1,
			expectedIAL:   IALLevel1,
		},
		{
			name:          "SMS OTP authenticator",
			authenticator: authncm.AuthenticatorSMSOTP,
			expectedAAL:   AALLevel1,
			expectedIAL:   IALLevel1,
		},
		{
			name:          "Google authenticator",
			authenticator: authncm.AuthenticatorGoogle,
			expectedAAL:   AALLevel1,
			expectedIAL:   IALLevel1,
		},
		{
			name:          "GitHub authenticator",
			authenticator: authncm.AuthenticatorGithub,
			expectedAAL:   AALLevel1,
			expectedIAL:   IALLevel1,
		},
		{
			name:          "OAuth authenticator",
			authenticator: authncm.AuthenticatorOAuth,
			expectedAAL:   AALLevel1,
			expectedIAL:   IALLevel1,
		},
		{
			name:          "OIDC authenticator",
			authenticator: authncm.AuthenticatorOIDC,
			expectedAAL:   AALLevel1,
			expectedIAL:   IALLevel1,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			authenticators := []authncm.AuthenticatorReference{
				{
					Authenticator: tc.authenticator,
					Step:          1,
					Timestamp:     time.Now().Unix(),
				},
			}

			result, err := suite.generator.GenerateAssertion(authenticators)

			suite.Nil(err)
			suite.NotNil(result)
			suite.NotNil(result.Context)
			suite.Equal(tc.expectedAAL, result.Context.AAL)
			suite.Equal(tc.expectedIAL, result.Context.IAL)
			suite.Len(result.Context.Authenticators, 1)
			suite.Equal(tc.authenticator, result.Context.Authenticators[0].Authenticator)
		})
	}
}

func (suite *GeneratorTestSuite) TestGenerateAssertionMultipleAuthenticators() {
	testCases := []struct {
		name               string
		authenticators     []string
		expectedAAL        AssuranceLevel
		expectedIAL        AssuranceLevel
		authenticatorCount int
	}{
		{
			name:               "Password + SMS OTP (MFA)",
			authenticators:     []string{authncm.AuthenticatorCredentials, authncm.AuthenticatorSMSOTP},
			expectedAAL:        AALLevel2,
			expectedIAL:        IALLevel1,
			authenticatorCount: 2,
		},
		{
			name:               "Google + SMS OTP (MFA)",
			authenticators:     []string{authncm.AuthenticatorGoogle, authncm.AuthenticatorSMSOTP},
			expectedAAL:        AALLevel2,
			expectedIAL:        IALLevel1,
			authenticatorCount: 2,
		},
		{
			name:               "GitHub + SMS OTP (MFA)",
			authenticators:     []string{authncm.AuthenticatorGithub, authncm.AuthenticatorSMSOTP},
			expectedAAL:        AALLevel2,
			expectedIAL:        IALLevel1,
			authenticatorCount: 2,
		},
		{
			name:               "OAuth + SMS OTP (MFA)",
			authenticators:     []string{authncm.AuthenticatorOAuth, authncm.AuthenticatorSMSOTP},
			expectedAAL:        AALLevel2,
			expectedIAL:        IALLevel1,
			authenticatorCount: 2,
		},
		{
			name:               "OIDC + SMS OTP (MFA)",
			authenticators:     []string{authncm.AuthenticatorOIDC, authncm.AuthenticatorSMSOTP},
			expectedAAL:        AALLevel2,
			expectedIAL:        IALLevel1,
			authenticatorCount: 2,
		},
		{
			name:               "Invalid combination (Google + GitHub)",
			authenticators:     []string{authncm.AuthenticatorGoogle, authncm.AuthenticatorGithub},
			expectedAAL:        AALLevel1,
			expectedIAL:        IALLevel1,
			authenticatorCount: 2,
		},
		{
			name: "Invalid combination (Password + Google + GitHub)",
			authenticators: []string{authncm.AuthenticatorCredentials, authncm.AuthenticatorGoogle,
				authncm.AuthenticatorGithub},
			expectedAAL:        AALLevel1,
			expectedIAL:        IALLevel1,
			authenticatorCount: 3,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			authenticators := make([]authncm.AuthenticatorReference, len(tc.authenticators))
			for i, auth := range tc.authenticators {
				authenticators[i] = authncm.AuthenticatorReference{
					Authenticator: auth,
					Step:          i + 1,
					Timestamp:     time.Now().Unix(),
				}
			}

			result, err := suite.generator.GenerateAssertion(authenticators)

			suite.Nil(err)
			suite.NotNil(result)
			suite.NotNil(result.Context)
			suite.Equal(tc.expectedAAL, result.Context.AAL)
			suite.Equal(tc.expectedIAL, result.Context.IAL)
			suite.Len(result.Context.Authenticators, tc.authenticatorCount)
		})
	}
}

func (suite *GeneratorTestSuite) TestGenerateAssertionDuplicateAuthenticators() {
	authenticators := []authncm.AuthenticatorReference{
		{
			Authenticator: authncm.AuthenticatorCredentials,
			Step:          1,
			Timestamp:     time.Now().Unix(),
		},
		{
			Authenticator: authncm.AuthenticatorCredentials,
			Step:          2,
			Timestamp:     time.Now().Unix(),
		},
	}

	result, err := suite.generator.GenerateAssertion(authenticators)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(AALLevel1, result.Context.AAL)
	suite.Len(result.Context.Authenticators, 2)
}

func (suite *GeneratorTestSuite) TestGenerateAssertionEmptyAuthenticators() {
	result, err := suite.generator.GenerateAssertion([]authncm.AuthenticatorReference{})

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorNoAuthenticators.Code, err.Code)
}

func (suite *GeneratorTestSuite) TestGenerateAssertionNilAuthenticators() {
	result, err := suite.generator.GenerateAssertion(nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorNoAuthenticators.Code, err.Code)
}

func (suite *GeneratorTestSuite) TestUpdateAssertionWithNilContext() {
	authenticator := authncm.AuthenticatorReference{
		Authenticator: authncm.AuthenticatorCredentials,
		Step:          1,
		Timestamp:     time.Now().Unix(),
	}

	result, err := suite.generator.UpdateAssertion(nil, authenticator)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(AALLevel1, result.Context.AAL)
	suite.Len(result.Context.Authenticators, 1)
}

func (suite *GeneratorTestSuite) TestUpdateAssertionAddingSecondFactor() {
	existingContext := &AssuranceContext{
		AAL: AALLevel1,
		IAL: IALLevel1,
		Authenticators: []authncm.AuthenticatorReference{
			{
				Authenticator: authncm.AuthenticatorCredentials,
				Step:          1,
				Timestamp:     time.Now().Unix(),
			},
		},
	}

	newAuthenticator := authncm.AuthenticatorReference{
		Authenticator: authncm.AuthenticatorSMSOTP,
		Step:          2,
		Timestamp:     time.Now().Unix(),
	}

	result, err := suite.generator.UpdateAssertion(existingContext, newAuthenticator)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(AALLevel2, result.Context.AAL)
	suite.Equal(IALLevel1, result.Context.IAL)
	suite.Len(result.Context.Authenticators, 2)
}

func (suite *GeneratorTestSuite) TestUpdateAssertionWithInvalidAuthenticator() {
	existingContext := &AssuranceContext{
		AAL: AALLevel1,
		IAL: IALLevel1,
		Authenticators: []authncm.AuthenticatorReference{
			{
				Authenticator: authncm.AuthenticatorCredentials,
				Step:          1,
				Timestamp:     time.Now().Unix(),
			},
		},
	}

	newAuthenticator := authncm.AuthenticatorReference{
		Authenticator: "",
		Step:          2,
		Timestamp:     time.Now().Unix(),
	}

	result, err := suite.generator.UpdateAssertion(existingContext, newAuthenticator)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidAuthenticator.Code, err.Code)
}

func (suite *GeneratorTestSuite) TestUpdateAssertionMultipleTimes() {
	context1, err1 := suite.generator.GenerateAssertion([]authncm.AuthenticatorReference{
		{
			Authenticator: authncm.AuthenticatorCredentials,
			Step:          1,
			Timestamp:     time.Now().Unix(),
		},
	})
	suite.Nil(err1)
	suite.Equal(AALLevel1, context1.Context.AAL)

	context2, err2 := suite.generator.UpdateAssertion(context1.Context, authncm.AuthenticatorReference{
		Authenticator: authncm.AuthenticatorSMSOTP,
		Step:          2,
		Timestamp:     time.Now().Unix(),
	})
	suite.Nil(err2)
	suite.Equal(AALLevel2, context2.Context.AAL)
	suite.Len(context2.Context.Authenticators, 2)

	context3, err3 := suite.generator.UpdateAssertion(context2.Context, authncm.AuthenticatorReference{
		Authenticator: authncm.AuthenticatorGoogle,
		Step:          3,
		Timestamp:     time.Now().Unix(),
	})
	suite.Nil(err3)
	// Adding Google (knowledge factor) to existing Credentials (knowledge) + SMS OTP (possession)
	// still has 2 factors (knowledge + possession), so AAL2
	suite.Equal(AALLevel2, context3.Context.AAL)
	suite.Len(context3.Context.Authenticators, 3)
}

func (suite *GeneratorTestSuite) TestVerifyAssurance() {
	testCases := []struct {
		name          string
		contextAAL    AssuranceLevel
		contextIAL    AssuranceLevel
		requiredAAL   AssuranceLevel
		requiredIAL   AssuranceLevel
		expectSuccess bool
		expectedError *serviceerror.ServiceError
	}{
		{
			name:          "Exact AAL match",
			contextAAL:    AALLevel2,
			contextIAL:    IALLevel1,
			requiredAAL:   AALLevel2,
			requiredIAL:   IALLevel1,
			expectSuccess: true,
		},
		{
			name:          "Higher AAL than required",
			contextAAL:    AALLevel3,
			contextIAL:    IALLevel1,
			requiredAAL:   AALLevel2,
			requiredIAL:   IALLevel1,
			expectSuccess: true,
		},
		{
			name:          "Lower AAL than required",
			contextAAL:    AALLevel1,
			contextIAL:    IALLevel1,
			requiredAAL:   AALLevel2,
			requiredIAL:   IALLevel1,
			expectSuccess: false,
		},
		{
			name:          "Exact IAL match",
			contextAAL:    AALLevel1,
			contextIAL:    IALLevel2,
			requiredAAL:   AALLevel1,
			requiredIAL:   IALLevel2,
			expectSuccess: true,
		},
		{
			name:          "Higher IAL than required",
			contextAAL:    AALLevel1,
			contextIAL:    IALLevel3,
			requiredAAL:   AALLevel1,
			requiredIAL:   IALLevel2,
			expectSuccess: true,
		},
		{
			name:          "Lower IAL than required",
			contextAAL:    AALLevel1,
			contextIAL:    IALLevel1,
			requiredAAL:   AALLevel1,
			requiredIAL:   IALLevel2,
			expectSuccess: false,
		},
		{
			name:          "No required levels - invalid input",
			contextAAL:    AALLevel1,
			contextIAL:    IALLevel1,
			requiredAAL:   "",
			requiredIAL:   "",
			expectSuccess: false,
			expectedError: &ErrorNoAssuranceRequirements,
		},
		{
			name:          "Only AAL required",
			contextAAL:    AALLevel2,
			contextIAL:    IALLevel1,
			requiredAAL:   AALLevel2,
			requiredIAL:   "",
			expectSuccess: true,
		},
		{
			name:          "Only IAL required",
			contextAAL:    AALLevel1,
			contextIAL:    IALLevel2,
			requiredAAL:   "",
			requiredIAL:   IALLevel2,
			expectSuccess: true,
		},
		{
			name:          "Both AAL and IAL higher than required",
			contextAAL:    AALLevel3,
			contextIAL:    IALLevel3,
			requiredAAL:   AALLevel1,
			requiredIAL:   IALLevel1,
			expectSuccess: true,
		},
		{
			name:          "AAL meets but IAL does not",
			contextAAL:    AALLevel2,
			contextIAL:    IALLevel1,
			requiredAAL:   AALLevel2,
			requiredIAL:   IALLevel2,
			expectSuccess: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			context := &AssuranceContext{
				AAL: tc.contextAAL,
				IAL: tc.contextIAL,
				Authenticators: []authncm.AuthenticatorReference{
					{
						Authenticator: authncm.AuthenticatorCredentials,
						Step:          1,
						Timestamp:     time.Now().Unix(),
					},
				},
			}

			verified, err := suite.generator.VerifyAssurance(context, tc.requiredAAL, tc.requiredIAL)

			suite.Equal(tc.expectSuccess, verified)
			if tc.expectedError != nil {
				suite.NotNil(err)
				suite.Equal(tc.expectedError.Code, err.Code)
			} else {
				suite.Nil(err)
			}
		})
	}
}

func (suite *GeneratorTestSuite) TestVerifyAssuranceNilContext() {
	verified, err := suite.generator.VerifyAssurance(nil, AALLevel1, IALLevel1)
	suite.False(verified)
	suite.NotNil(err)
	suite.Equal(ErrorNilAssuranceContext.Code, err.Code)
}

func (suite *GeneratorTestSuite) TestExtractUniqueFactors() {
	generator := &authAssertGenerator{}

	testCases := []struct {
		name                    string
		authenticators          []authncm.AuthenticatorReference
		expectedUniqueAuthCount int
		expectedUniqueFactors   int
		expectedAuthContains    []string
		expectedFactorsContains []authncm.AuthenticationFactor
	}{
		{
			name: "All unique authenticators with different factors",
			authenticators: []authncm.AuthenticatorReference{
				{Authenticator: authncm.AuthenticatorCredentials, Step: 1, Timestamp: time.Now().Unix()},
				{Authenticator: authncm.AuthenticatorSMSOTP, Step: 2, Timestamp: time.Now().Unix()},
				{Authenticator: authncm.AuthenticatorGoogle, Step: 3, Timestamp: time.Now().Unix()},
			},
			expectedUniqueAuthCount: 3,
			expectedUniqueFactors:   2, // KNOWLEDGE and POSSESSION
			expectedAuthContains: []string{authncm.AuthenticatorCredentials, authncm.AuthenticatorSMSOTP,
				authncm.AuthenticatorGoogle},
			expectedFactorsContains: []authncm.AuthenticationFactor{
				authncm.FactorKnowledge, authncm.FactorPossession,
			},
		},
		{
			name: "Duplicate authenticators",
			authenticators: []authncm.AuthenticatorReference{
				{Authenticator: authncm.AuthenticatorCredentials, Step: 1, Timestamp: time.Now().Unix()},
				{Authenticator: authncm.AuthenticatorCredentials, Step: 2, Timestamp: time.Now().Unix()},
				{Authenticator: authncm.AuthenticatorSMSOTP, Step: 3, Timestamp: time.Now().Unix()},
			},
			expectedUniqueAuthCount: 2,
			expectedUniqueFactors:   2, // KNOWLEDGE and POSSESSION
			expectedAuthContains:    []string{authncm.AuthenticatorCredentials, authncm.AuthenticatorSMSOTP},
			expectedFactorsContains: []authncm.AuthenticationFactor{
				authncm.FactorKnowledge, authncm.FactorPossession,
			},
		},
		{
			name: "All same authenticator",
			authenticators: []authncm.AuthenticatorReference{
				{Authenticator: authncm.AuthenticatorCredentials, Step: 1, Timestamp: time.Now().Unix()},
				{Authenticator: authncm.AuthenticatorCredentials, Step: 2, Timestamp: time.Now().Unix()},
			},
			expectedUniqueAuthCount: 1,
			expectedUniqueFactors:   1, // KNOWLEDGE only
			expectedAuthContains:    []string{authncm.AuthenticatorCredentials},
			expectedFactorsContains: []authncm.AuthenticationFactor{authncm.FactorKnowledge},
		},
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GeneratorTestSuite"))
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			authMap, factorSet := generator.extractUniqueAuthenticators(tc.authenticators, logger)

			suite.Equal(tc.expectedUniqueAuthCount, len(authMap))
			suite.Equal(tc.expectedUniqueFactors, len(factorSet))

			for _, expected := range tc.expectedAuthContains {
				suite.Contains(authMap, expected)
			}

			for _, expected := range tc.expectedFactorsContains {
				suite.Contains(factorSet, expected)
			}
		})
	}
}

func (suite *GeneratorTestSuite) TestCalculateAAL() {
	generator := &authAssertGenerator{}

	testCases := []struct {
		name           string
		authenticators []authncm.AuthenticatorReference
		expectedAAL    AssuranceLevel
	}{
		{
			name: "Single authenticator - AAL1",
			authenticators: []authncm.AuthenticatorReference{
				{Authenticator: authncm.AuthenticatorCredentials, Step: 1, Timestamp: time.Now().Unix()},
			},
			expectedAAL: AALLevel1,
		},
		{
			name: "Valid MFA combination",
			authenticators: []authncm.AuthenticatorReference{
				{Authenticator: authncm.AuthenticatorCredentials, Step: 1, Timestamp: time.Now().Unix()},
				{Authenticator: authncm.AuthenticatorSMSOTP, Step: 2, Timestamp: time.Now().Unix()},
			},
			expectedAAL: AALLevel2,
		},
		{
			name: "Invalid combination - both knowledge factors",
			authenticators: []authncm.AuthenticatorReference{
				{Authenticator: authncm.AuthenticatorGoogle, Step: 1, Timestamp: time.Now().Unix()},
				{Authenticator: authncm.AuthenticatorGithub, Step: 2, Timestamp: time.Now().Unix()},
			},
			expectedAAL: AALLevel1,
		},
		{
			name: "Three authenticators with two factors (knowledge + possession)",
			authenticators: []authncm.AuthenticatorReference{
				{Authenticator: authncm.AuthenticatorCredentials, Step: 1, Timestamp: time.Now().Unix()},
				{Authenticator: authncm.AuthenticatorSMSOTP, Step: 2, Timestamp: time.Now().Unix()},
				{Authenticator: authncm.AuthenticatorGoogle, Step: 3, Timestamp: time.Now().Unix()},
			},
			expectedAAL: AALLevel2,
		},
		{
			name: "Single unknown authenticator",
			authenticators: []authncm.AuthenticatorReference{
				{Authenticator: "UnknownAuthenticator", Step: 1, Timestamp: time.Now().Unix()},
			},
			expectedAAL: AALUnknown,
		},
		{
			name:           "Empty authenticator list",
			authenticators: []authncm.AuthenticatorReference{},
			expectedAAL:    AALUnknown,
		},
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GeneratorTestSuite"))
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			_, factorSet := generator.extractUniqueAuthenticators(tc.authenticators, logger)
			result := generator.calculateAAL(factorSet, logger)
			suite.Equal(tc.expectedAAL, result)
		})
	}
}

func (suite *GeneratorTestSuite) TestCalculateIAL() {
	generator := &authAssertGenerator{}
	result := generator.calculateIAL()
	suite.Equal(IALLevel1, result)
}

func (suite *GeneratorTestSuite) TestMeetsAssuranceLevel() {
	generator := &authAssertGenerator{}

	testCases := []struct {
		name     string
		actual   AssuranceLevel
		required AssuranceLevel
		expected bool
	}{
		{
			name:     "AAL1 meets AAL1",
			actual:   AALLevel1,
			required: AALLevel1,
			expected: true,
		},
		{
			name:     "AAL2 meets AAL1",
			actual:   AALLevel2,
			required: AALLevel1,
			expected: true,
		},
		{
			name:     "AAL3 meets AAL2",
			actual:   AALLevel3,
			required: AALLevel2,
			expected: true,
		},
		{
			name:     "AAL1 does not meet AAL2",
			actual:   AALLevel1,
			required: AALLevel2,
			expected: false,
		},
		{
			name:     "AAL2 does not meet AAL3",
			actual:   AALLevel2,
			required: AALLevel3,
			expected: false,
		},
		{
			name:     "IAL1 meets IAL1",
			actual:   IALLevel1,
			required: IALLevel1,
			expected: true,
		},
		{
			name:     "IAL2 meets IAL1",
			actual:   IALLevel2,
			required: IALLevel1,
			expected: true,
		},
		{
			name:     "IAL1 does not meet IAL2",
			actual:   IALLevel1,
			required: IALLevel2,
			expected: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := generator.meetsAssuranceLevel(tc.actual, tc.required)
			suite.Equal(tc.expected, result)
		})
	}
}
