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

package requestvalidator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
)

type AuthzValidationTestSuite struct {
	suite.Suite
	oauthApp *inboundmodel.OAuthClient
}

func TestAuthzValidationTestSuite(t *testing.T) {
	suite.Run(t, new(AuthzValidationTestSuite))
}

func (suite *AuthzValidationTestSuite) SetupTest() {
	suite.oauthApp = &inboundmodel.OAuthClient{
		ClientID:                "test-client-id",
		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
	}
}

func (suite *AuthzValidationTestSuite) validParams() map[string]string {
	return map[string]string{
		constants.RequestParamResponseType: string(constants.ResponseTypeCode),
	}
}

// ValidateAuthorizationRequestParams tests

func (suite *AuthzValidationTestSuite) TestValidateParams_Success() {
	params := suite.validParams()

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp)

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_MissingResponseType() {
	params := map[string]string{}

	errCode, _ := ValidateAuthorizationRequestParams(params, suite.oauthApp)

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_UnsupportedResponseType() {
	params := map[string]string{
		constants.RequestParamResponseType: "token",
	}

	errCode, _ := ValidateAuthorizationRequestParams(params, suite.oauthApp)

	assert.Equal(suite.T(), constants.ErrorUnsupportedResponseType, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_GrantTypeNotAllowed() {
	app := &inboundmodel.OAuthClient{
		ClientID:                "test-client-id",
		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeClientCredentials},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
	}
	params := suite.validParams()

	errCode, _ := ValidateAuthorizationRequestParams(params, app)

	assert.Equal(suite.T(), constants.ErrorUnauthorizedClient, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PKCERequired_MissingCodeChallenge() {
	app := &inboundmodel.OAuthClient{
		ClientID:                "test-client-id",
		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            true,
	}
	params := suite.validParams()

	errCode, errMsg := ValidateAuthorizationRequestParams(params, app)

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
	assert.Equal(suite.T(), "code_challenge is required for this application", errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PKCERequired_InvalidCodeChallenge() {
	app := &inboundmodel.OAuthClient{
		ClientID:                "test-client-id",
		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            true,
	}
	params := suite.validParams()
	params[constants.RequestParamCodeChallenge] = "invalid"
	params[constants.RequestParamCodeChallengeMethod] = "plain"

	errCode, _ := ValidateAuthorizationRequestParams(params, app)

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PKCERequired_ValidPKCE() {
	app := &inboundmodel.OAuthClient{
		ClientID:                "test-client-id",
		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            true,
	}
	params := suite.validParams()
	params[constants.RequestParamCodeChallenge] = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	params[constants.RequestParamCodeChallengeMethod] = "S256"

	errCode, errMsg := ValidateAuthorizationRequestParams(params, app)

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_NonceTooLong() {
	params := suite.validParams()
	params[constants.RequestParamNonce] = strings.Repeat("a", constants.MaxNonceLength+1)

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp)

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
	assert.Equal(suite.T(), "nonce exceeds maximum allowed length", errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_ValidNonce() {
	params := suite.validParams()
	params[constants.RequestParamNonce] = strings.Repeat("a", constants.MaxNonceLength)

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp)

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptLogin_Success() {
	params := suite.validParams()
	params[constants.RequestParamPrompt] = "login"

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp)

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptNone_LoginRequired() {
	params := suite.validParams()
	params[constants.RequestParamPrompt] = "none"

	errCode, _ := ValidateAuthorizationRequestParams(params, suite.oauthApp)

	assert.Equal(suite.T(), constants.ErrorLoginRequired, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptInvalid() {
	params := suite.validParams()
	params[constants.RequestParamPrompt] = "invalid_value"

	errCode, _ := ValidateAuthorizationRequestParams(params, suite.oauthApp)

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptNoneCombined() {
	params := suite.validParams()
	params[constants.RequestParamPrompt] = "none login"

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp)

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
	assert.Contains(suite.T(), errMsg, "must not be combined")
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptConsent() {
	params := suite.validParams()
	params[constants.RequestParamPrompt] = "consent"

	errCode, _ := ValidateAuthorizationRequestParams(params, suite.oauthApp)

	assert.Equal(suite.T(), constants.ErrorConsentRequired, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptSelectAccount() {
	params := suite.validParams()
	params[constants.RequestParamPrompt] = "select_account"

	errCode, _ := ValidateAuthorizationRequestParams(params, suite.oauthApp)

	assert.Equal(suite.T(), constants.ErrorAccountSelectionRequired, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptEmpty() {
	params := suite.validParams()
	params[constants.RequestParamPrompt] = ""

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp)

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
	assert.Equal(suite.T(), "The prompt parameter cannot be empty", errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptNotPresent_Success() {
	// When prompt key is not in the map at all, it should not be validated.
	params := suite.validParams()

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp)

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

// ValidatePromptParameter tests

func (suite *AuthzValidationTestSuite) TestValidatePromptParameter_Login() {
	errCode, _ := ValidatePromptParameter("login")
	assert.Empty(suite.T(), errCode)
}

func (suite *AuthzValidationTestSuite) TestValidatePromptParameter_None_LoginRequired() {
	errCode, _ := ValidatePromptParameter("none")
	assert.Equal(suite.T(), constants.ErrorLoginRequired, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidatePromptParameter_Consent() {
	errCode, _ := ValidatePromptParameter("consent")
	assert.Equal(suite.T(), constants.ErrorConsentRequired, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidatePromptParameter_SelectAccount() {
	errCode, _ := ValidatePromptParameter("select_account")
	assert.Equal(suite.T(), constants.ErrorAccountSelectionRequired, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidatePromptParameter_Invalid() {
	errCode, _ := ValidatePromptParameter("invalid_value")
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidatePromptParameter_Empty() {
	errCode, _ := ValidatePromptParameter("")
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidatePromptParameter_NoneWithOther() {
	errCode, errMsg := ValidatePromptParameter("none login")
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
	assert.Contains(suite.T(), errMsg, "must not be combined")
}

func (suite *AuthzValidationTestSuite) TestValidatePromptParameter_LoginConsent() {
	errCode, _ := ValidatePromptParameter("login consent")
	assert.Equal(suite.T(), constants.ErrorConsentRequired, errCode)
}

type ACRValuesTestSuite struct {
	suite.Suite
}

func TestACRValuesTestSuite(t *testing.T) {
	suite.Run(t, new(ACRValuesTestSuite))
}

func (suite *ACRValuesTestSuite) TestParseACRValues_SingleACR() {
	result := parseACRValues("urn:thunder:acr:password")
	assert.Equal(suite.T(), []string{"urn:thunder:acr:password"}, result)
}

func (suite *ACRValuesTestSuite) TestParseACRValues_MultipleACRs() {
	result := parseACRValues("urn:thunder:acr:password urn:thunder:acr:generated-code")
	assert.Equal(suite.T(),
		[]string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"}, result)
}

func (suite *ACRValuesTestSuite) TestParseACRValues_DeduplicatesPreservingFirstOccurrence() {
	result := parseACRValues("urn:thunder:acr:generated-code urn:thunder:acr:generated-code urn:thunder:acr:password")
	assert.Equal(suite.T(),
		[]string{"urn:thunder:acr:generated-code", "urn:thunder:acr:password"}, result)
}

func (suite *ACRValuesTestSuite) TestParseACRValues_EmptyString() {
	result := parseACRValues("")
	assert.Empty(suite.T(), result)
}

func (suite *ACRValuesTestSuite) TestParseACRValues_OnlyWhitespace() {
	result := parseACRValues("   ")
	assert.Empty(suite.T(), result)
}

func (suite *ACRValuesTestSuite) TestParseACRValues_ExtraSpacesBetweenACRs() {
	result := parseACRValues("urn:thunder:acr:password   urn:thunder:acr:generated-code")
	assert.Equal(suite.T(),
		[]string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"}, result)
}

func (suite *ACRValuesTestSuite) TestParseACRValues_PreservesOrder() {
	result := parseACRValues("urn:thunder:acr:biometrics urn:thunder:acr:password urn:thunder:acr:generated-code")
	assert.Equal(suite.T(), []string{
		"urn:thunder:acr:biometrics",
		"urn:thunder:acr:password",
		"urn:thunder:acr:generated-code",
	}, result)
}

func (suite *ACRValuesTestSuite) TestResolveACRValues_NoRequest_NoDefaults() {
	assert.Equal(suite.T(), "", ResolveACRValues("", nil))
}

func (suite *ACRValuesTestSuite) TestResolveACRValues_NoRequest_FallsBackToDefaults() {
	defaults := []string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"}
	result := ResolveACRValues("", defaults)
	assert.ElementsMatch(suite.T(), defaults, strings.Fields(result))
}

func (suite *ACRValuesTestSuite) TestResolveACRValues_AllRequestedInDefaults_PreservesRequestedOrder() {
	defaults := []string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"}
	result := ResolveACRValues("urn:thunder:acr:generated-code urn:thunder:acr:password", defaults)
	assert.Equal(suite.T(),
		[]string{"urn:thunder:acr:generated-code", "urn:thunder:acr:password"},
		strings.Fields(result))
}

func (suite *ACRValuesTestSuite) TestResolveACRValues_SomeNotInDefaults_FiltersOutUnknown() {
	defaults := []string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"}
	result := ResolveACRValues("urn:thunder:acr:password urn:thunder:acr:biometrics", defaults)
	assert.Equal(suite.T(), []string{"urn:thunder:acr:password"}, strings.Fields(result))
}

func (suite *ACRValuesTestSuite) TestResolveACRValues_NoneInDefaults_FallsBackToDefaults() {
	defaults := []string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"}
	result := ResolveACRValues("urn:thunder:acr:biometrics urn:thunder:acr:linked-wallet", defaults)
	assert.ElementsMatch(suite.T(), defaults, strings.Fields(result))
}

func (suite *ACRValuesTestSuite) TestResolveACRValues_DuplicatesDeduped() {
	defaults := []string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"}
	result := ResolveACRValues(
		"urn:thunder:acr:password urn:thunder:acr:password urn:thunder:acr:generated-code", defaults)
	assert.Equal(suite.T(),
		[]string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"},
		strings.Fields(result))
}

func (suite *ACRValuesTestSuite) TestResolveACRValues_RequestPresent_NoDefaults_ReturnsEmpty() {
	result := ResolveACRValues("urn:thunder:acr:password urn:thunder:acr:generated-code", nil)
	assert.Equal(suite.T(), "", result)
}
