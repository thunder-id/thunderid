// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package requestvalidator

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	testJKT      = "0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I"
	testOtherJKT = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
)

type AuthzValidationTestSuite struct {
	suite.Suite
	oauthApp *providers.OAuthClient
}

func TestAuthzValidationTestSuite(t *testing.T) {
	suite.Run(t, new(AuthzValidationTestSuite))
}

func (suite *AuthzValidationTestSuite) SetupTest() {
	suite.oauthApp = &providers.OAuthClient{
		ClientID:                "test-client-id",
		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
	}
}

func (suite *AuthzValidationTestSuite) validParams() url.Values {
	return url.Values{
		constants.RequestParamResponseType: {string(providers.ResponseTypeCode)},
	}
}

// ValidateAuthorizationRequestParams tests

func (suite *AuthzValidationTestSuite) TestValidateParams_Success() {
	params := suite.validParams()

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_RequestObjectRejected() {
	params := suite.validParams()
	params.Set(constants.RequestParamRequest, "eyJhbGciOiAibm9uZSJ9.eyJzdGF0ZSI6ICJhYmMifQ.")

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Equal(suite.T(), constants.ErrorRequestNotSupported, errCode)
	assert.NotEmpty(suite.T(), errMsg)
}

// The request object is rejected before any other parameter is validated, so a request carrying
// one is never processed on the query string alone.
func (suite *AuthzValidationTestSuite) TestValidateParams_RequestObjectRejectedBeforeOtherParams() {
	params := url.Values{
		constants.RequestParamRequest: {"eyJhbGciOiAibm9uZSJ9.eyJzdGF0ZSI6ICJhYmMifQ."},
	}

	errCode, _ := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Equal(suite.T(), constants.ErrorRequestNotSupported, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_EmptyRequestParamIgnored() {
	params := suite.validParams()
	params.Set(constants.RequestParamRequest, "")

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_MissingResponseType() {
	params := url.Values{}

	errCode, _ := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_UnsupportedResponseType() {
	params := url.Values{
		constants.RequestParamResponseType: {"token"},
	}

	errCode, _ := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Equal(suite.T(), constants.ErrorUnsupportedResponseType, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_QueryResponseMode() {
	params := suite.validParams()
	params.Set(constants.RequestParamResponseMode, constants.ResponseModeQuery)

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_UnsupportedResponseMode() {
	for _, responseMode := range []string{"fragment", "form_post"} {
		suite.T().Run(responseMode, func(t *testing.T) {
			params := suite.validParams()
			params.Set(constants.RequestParamResponseMode, responseMode)

			errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

			assert.Equal(t, constants.ErrorInvalidRequest, errCode)
			assert.Equal(t, "Unsupported response_mode parameter", errMsg)
		})
	}
}

func (suite *AuthzValidationTestSuite) TestValidateParams_GrantTypeNotAllowed() {
	app := &providers.OAuthClient{
		ClientID:                "test-client-id",
		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
		ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
	}
	params := suite.validParams()

	errCode, _ := ValidateAuthorizationRequestParams(params, app, "")

	assert.Equal(suite.T(), constants.ErrorUnauthorizedClient, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PKCERequired_MissingCodeChallenge() {
	app := &providers.OAuthClient{
		ClientID:                "test-client-id",
		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            true,
	}
	params := suite.validParams()

	errCode, errMsg := ValidateAuthorizationRequestParams(params, app, "")

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
	assert.Equal(suite.T(), "code_challenge is required for this application", errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PKCERequired_InvalidCodeChallenge() {
	app := &providers.OAuthClient{
		ClientID:                "test-client-id",
		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            true,
	}
	params := suite.validParams()
	params.Set(constants.RequestParamCodeChallenge, "invalid")
	params.Set(constants.RequestParamCodeChallengeMethod, "plain")

	errCode, _ := ValidateAuthorizationRequestParams(params, app, "")

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PKCERequired_ValidPKCE() {
	app := &providers.OAuthClient{
		ClientID:                "test-client-id",
		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            true,
	}
	params := suite.validParams()
	params.Set(constants.RequestParamCodeChallenge, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM")
	params.Set(constants.RequestParamCodeChallengeMethod, "S256")

	errCode, errMsg := ValidateAuthorizationRequestParams(params, app, "")

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_NonceTooLong() {
	params := suite.validParams()
	params.Set(constants.RequestParamNonce, strings.Repeat("a", constants.MaxNonceLength+1))

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
	assert.Equal(suite.T(), "nonce exceeds maximum allowed length", errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_ValidNonce() {
	params := suite.validParams()
	params.Set(constants.RequestParamNonce, strings.Repeat("a", constants.MaxNonceLength))

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptLogin_Success() {
	params := suite.validParams()
	params.Set(constants.RequestParamPrompt, "login")

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

// TestValidateParams_PromptNone_Accepted covers prompt=none passing shared parameter validation.
// Whether it can be honored depends on an existing SSO session, which this validation cannot see,
// so the authorize endpoint decides it against the resolved session instead.
func (suite *AuthzValidationTestSuite) TestValidateParams_PromptNone_Accepted() {
	params := suite.validParams()
	params.Set(constants.RequestParamPrompt, "none")

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptInvalid() {
	params := suite.validParams()
	params.Set(constants.RequestParamPrompt, "invalid_value")

	errCode, _ := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptNoneCombined() {
	params := suite.validParams()
	params.Set(constants.RequestParamPrompt, "none login")

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
	assert.Contains(suite.T(), errMsg, "must not be combined")
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptConsent_Success() {
	params := suite.validParams()
	params.Set(constants.RequestParamPrompt, "consent")

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptLoginConsent_Success() {
	params := suite.validParams()
	params.Set(constants.RequestParamPrompt, "login consent")

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptSelectAccount() {
	params := suite.validParams()
	params.Set(constants.RequestParamPrompt, "select_account")

	errCode, _ := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Equal(suite.T(), constants.ErrorAccountSelectionRequired, errCode)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptEmpty() {
	params := suite.validParams()
	params.Set(constants.RequestParamPrompt, "")

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
	assert.Equal(suite.T(), "The prompt parameter cannot be empty", errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_DPoPJktParamOnly_Success() {
	params := suite.validParams()
	params.Set(constants.RequestParamDPoPJkt, testJKT)

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_DPoPHeaderOnly_Success() {
	params := suite.validParams()

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, testJKT)

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_DPoPJktAndHeaderMatch_Success() {
	params := suite.validParams()
	params.Set(constants.RequestParamDPoPJkt, testJKT)

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, testJKT)

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidateParams_DPoPJktAndHeaderMismatch_Rejected() {
	params := suite.validParams()
	params.Set(constants.RequestParamDPoPJkt, testJKT)

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, testOtherJKT)

	assert.Equal(suite.T(), constants.ErrorInvalidDPoPProof, errCode)
	assert.Contains(suite.T(), errMsg, "does not match")
}

func (suite *AuthzValidationTestSuite) TestValidateParams_DPoPJktParamMalformed_Rejected() {
	params := suite.validParams()
	params.Set(constants.RequestParamDPoPJkt, "not-a-thumbprint")

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errCode)
	assert.Contains(suite.T(), errMsg, "Invalid dpop_jkt")
}

func (suite *AuthzValidationTestSuite) TestValidateParams_PromptNotPresent_Success() {
	// When prompt key is not in the map at all, it should not be validated.
	params := suite.validParams()

	errCode, errMsg := ValidateAuthorizationRequestParams(params, suite.oauthApp, "")

	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

// ValidatePromptParameter tests

func (suite *AuthzValidationTestSuite) TestValidatePromptParameter_Login() {
	errCode, _ := ValidatePromptParameter("login")
	assert.Empty(suite.T(), errCode)
}

// TestValidatePromptParameter_None_Accepted covers "none" being a valid parameter value on its
// own. The login_required decision belongs to the authorize endpoint, which can consult the
// session this function cannot see.
func (suite *AuthzValidationTestSuite) TestValidatePromptParameter_None_Accepted() {
	errCode, errMsg := ValidatePromptParameter("none")
	assert.Empty(suite.T(), errCode)
	assert.Empty(suite.T(), errMsg)
}

func (suite *AuthzValidationTestSuite) TestValidatePromptParameter_Consent() {
	errCode, _ := ValidatePromptParameter("consent")
	assert.Empty(suite.T(), errCode)
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
	assert.Empty(suite.T(), errCode)
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
