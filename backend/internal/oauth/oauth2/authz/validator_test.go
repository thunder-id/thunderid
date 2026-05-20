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

package authz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
)

type AuthorizationValidatorTestSuite struct {
	suite.Suite
	validator AuthorizationValidatorInterface
	oauthApp  *inboundmodel.OAuthClient
}

func TestAuthorizationValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationValidatorTestSuite))
}

func (suite *AuthorizationValidatorTestSuite) SetupTest() {
	sysconfig.ResetServerRuntime()
	err := sysconfig.InitializeServerRuntime("/tmp/test", &sysconfig.Config{
		OAuth: sysconfig.OAuthConfig{AllowWildcardRedirectURI: true},
	})
	suite.Require().NoError(err)

	suite.validator = newAuthorizationValidator()

	suite.oauthApp = &inboundmodel.OAuthClient{
		ClientID:                "test-client-id",
		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
	}
}

func (suite *AuthorizationValidatorTestSuite) TearDownTest() {
	sysconfig.ResetServerRuntime()
}

func (suite *AuthorizationValidatorTestSuite) TestnewAuthorizationValidator() {
	validator := newAuthorizationValidator()
	assert.NotNil(suite.T(), validator)
	assert.Implements(suite.T(), (*AuthorizationValidatorInterface)(nil), validator)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_Success() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.False(suite.T(), sendErrorToApp)
	assert.Empty(suite.T(), errorCode)
	assert.Empty(suite.T(), errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_MissingClientID() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.False(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errorCode)
	assert.Equal(suite.T(), "Missing client_id parameter", errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_InvalidRedirectURI() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://malicious.example.com/callback", // not in allowed list
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.False(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errorCode)
	assert.Equal(suite.T(), "Invalid redirect URI", errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateAuthzRequest_CodeGrantNotAllowed() {
	// Create an app that doesn't allow authorization code grant type
	restrictedApp := &inboundmodel.OAuthClient{
		ClientID: "test-client-id",

		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeClientCredentials}, // no auth code
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
	}

	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, restrictedApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorUnauthorizedClient, errorCode)
	assert.Equal(suite.T(), "Authorization code grant type is not allowed for the client", errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_MissingResponseType() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:    "test-client-id",
			constants.RequestParamRedirectURI: "https://client.example.com/callback",
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errorCode)
	assert.Equal(suite.T(), "Missing response_type parameter", errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_UnsupportedResponseType() {
	// Create an app that doesn't support "code" response type
	restrictedApp := &inboundmodel.OAuthClient{
		ClientID: "test-client-id",

		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{}, // no response types allowed
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
	}

	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, restrictedApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorUnsupportedResponseType, errorCode)
	assert.Equal(suite.T(), "Unsupported response type", errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_EmptyRedirectURI() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "", // empty redirect URI should be OK if app has only one registered
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.False(suite.T(), sendErrorToApp)
	assert.Empty(suite.T(), errorCode)
	assert.Empty(suite.T(), errorMessage)
}

// Resource Parameter Validation Tests

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_ValidResource() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamResource:     "https://api.example.com/resource",
		},
		Resources: []string{"https://api.example.com/resource"},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.False(suite.T(), sendErrorToApp)
	assert.Empty(suite.T(), errorCode)
	assert.Empty(suite.T(), errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_ValidMCPServerResource() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamResource:     "https://mcp.example.com/mcp",
		},
		Resources: []string{"https://mcp.example.com/mcp"},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.False(suite.T(), sendErrorToApp)
	assert.Empty(suite.T(), errorCode)
	assert.Empty(suite.T(), errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_ValidResourceWithPort() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamResource:     "https://mcp.example.com:8443",
		},
		Resources: []string{"https://mcp.example.com:8443"},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.False(suite.T(), sendErrorToApp)
	assert.Empty(suite.T(), errorCode)
	assert.Empty(suite.T(), errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_EmptyResource() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamResource:     "",
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.False(suite.T(), sendErrorToApp)
	assert.Empty(suite.T(), errorCode)
	assert.Empty(suite.T(), errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_ResourceMissingScheme() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamResource:     "api.example.com/resource",
		},
		Resources: []string{"api.example.com/resource"},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errorCode)
	assert.Contains(suite.T(), errorMessage, "must be an absolute URI")
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_ResourceWithFragment() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamResource:     "https://api.example.com/resource#fragment",
		},
		Resources: []string{"https://api.example.com/resource#fragment"},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errorCode)
	assert.Contains(suite.T(), errorMessage, "must not contain a fragment component")
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_ResourceRelativeURI() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamResource:     "/api/resource",
		},
		Resources: []string{"/api/resource"},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errorCode)
	assert.Contains(suite.T(), errorMessage, "must be an absolute URI")
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_ResourceInvalidURI() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamResource:     "not a valid uri format",
		},
		Resources: []string{"not a valid uri format"},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errorCode)
	assert.Contains(suite.T(), errorMessage, "must be an absolute URI")
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_ResourceParameterWithQuery() {
	// Test resource parameter with query component (should be valid per RFC 8707)
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamResource:     "https://api.example.com/resource?param=value",
		},
		Resources: []string{"https://api.example.com/resource?param=value"},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.False(suite.T(), sendErrorToApp)
	assert.Empty(suite.T(), errorCode)
	assert.Empty(suite.T(), errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateAuthzReq_PKCERequired_MissingCodeChallenge() {
	// Create an app that requires PKCE
	pkceApp := &inboundmodel.OAuthClient{
		ClientID: "test-client-id",

		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            true,
	}

	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			// Missing code_challenge
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, pkceApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errorCode)
	assert.Equal(suite.T(), "code_challenge is required for this application", errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateAuthzReq_PKCERequired_InvalidCodeChallenge() {
	// Create an app that requires PKCE
	pkceApp := &inboundmodel.OAuthClient{
		ClientID: "test-client-id",

		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            true,
	}

	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:            "test-client-id",
			constants.RequestParamRedirectURI:         "https://client.example.com/callback",
			constants.RequestParamResponseType:        string(constants.ResponseTypeCode),
			constants.RequestParamCodeChallenge:       "invalid-challenge",
			constants.RequestParamCodeChallengeMethod: "plain", // Not supported per OAuth 2.0 Security BCP
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, pkceApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errorCode)
	assert.Equal(suite.T(), "Invalid code_challenge or code_challenge_method parameter", errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_PKCERequired_ValidPKCE() {
	// Create an app that requires PKCE
	pkceApp := &inboundmodel.OAuthClient{
		ClientID: "test-client-id",

		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            true,
	}

	// Use a valid S256 code challenge (base64url encoded SHA256 hash)
	// This is a valid format for testing
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:            "test-client-id",
			constants.RequestParamRedirectURI:         "https://client.example.com/callback",
			constants.RequestParamResponseType:        string(constants.ResponseTypeCode),
			constants.RequestParamCodeChallenge:       "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
			constants.RequestParamCodeChallengeMethod: "S256",
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, pkceApp)

	assert.False(suite.T(), sendErrorToApp)
	assert.Empty(suite.T(), errorCode)
	assert.Empty(suite.T(), errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateAuthzReq_PKCERequired_MissingCodeChallengeMethod() {
	// Create an app that requires PKCE
	pkceApp := &inboundmodel.OAuthClient{
		ClientID: "test-client-id",

		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            true,
	}

	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:      "test-client-id",
			constants.RequestParamRedirectURI:   "https://client.example.com/callback",
			constants.RequestParamResponseType:  string(constants.ResponseTypeCode),
			constants.RequestParamCodeChallenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
			// Missing code_challenge_method - should fail instead of defaulting to S256
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, pkceApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errorCode)
	assert.Equal(suite.T(), "Invalid code_challenge or code_challenge_method parameter", errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_PKCENotRequired() {
	// Create an app that doesn't require PKCE
	nonPKCEApp := &inboundmodel.OAuthClient{
		ClientID: "test-client-id",

		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            false,
	}

	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			// No PKCE parameters - should be OK since PKCE is not required
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, nonPKCEApp)

	assert.False(suite.T(), sendErrorToApp)
	assert.Empty(suite.T(), errorCode)
	assert.Empty(suite.T(), errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateAuthzReq_PKCENotRequired_InvalidPKCEParams() {
	// Create an app that doesn't require PKCE
	nonPKCEApp := &inboundmodel.OAuthClient{
		ClientID: "test-client-id",

		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            false,
	}

	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:      "test-client-id",
			constants.RequestParamRedirectURI:   "https://client.example.com/callback",
			constants.RequestParamResponseType:  string(constants.ResponseTypeCode),
			constants.RequestParamCodeChallenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
			// Missing code_challenge_method - should fail even when PKCE is not required
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, nonPKCEApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errorCode)
	assert.Equal(suite.T(), "Invalid code_challenge or code_challenge_method parameter", errorMessage)
}

// Prompt Parameter Validation Tests (OIDC Core §3.1.2.1)

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthzRequest_PromptNone_LoginRequired() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamPrompt:       "none",
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorLoginRequired, errorCode)
	assert.Equal(suite.T(), "User authentication is required", errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_PromptLogin_Success() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamPrompt:       "login",
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.False(suite.T(), sendErrorToApp)
	assert.Empty(suite.T(), errorCode)
	assert.Empty(suite.T(), errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthzRequest_PromptConsent_ConsentRequired() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamPrompt:       "consent",
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorConsentRequired, errorCode)
	assert.Equal(suite.T(), "Consent is not supported", errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthzRequest_PromptNoneCombined_InvalidRequest() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamPrompt:       "none login",
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errorCode)
	assert.Contains(suite.T(), errorMessage, "must not be combined")
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthzRequest_PromptInvalidValue_InvalidRequest() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamPrompt:       "invalid_value",
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errorCode)
	assert.Equal(suite.T(), "Unsupported prompt parameter value", errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthzRequest_PromptSelectAccount() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamPrompt:       "select_account",
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorAccountSelectionRequired, errorCode)
	assert.Equal(suite.T(), "Account selection is not supported", errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthzRequest_PromptLoginConsent_ConsentRequired() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamPrompt:       "login consent",
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorConsentRequired, errorCode)
	assert.Equal(suite.T(), "Consent is not supported", errorMessage)
}

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthzRequest_PromptEmpty_InvalidRequest() {
	msg := &OAuthMessage{
		RequestQueryParams: map[string]string{
			constants.RequestParamClientID:     "test-client-id",
			constants.RequestParamRedirectURI:  "https://client.example.com/callback",
			constants.RequestParamResponseType: string(constants.ResponseTypeCode),
			constants.RequestParamPrompt:       "",
		},
	}

	sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(
		msg, suite.oauthApp)

	assert.True(suite.T(), sendErrorToApp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errorCode)
	assert.Equal(suite.T(), "The prompt parameter cannot be empty", errorMessage)
}

// Wildcard Redirect URI Tests (AC-06 through AC-12)

func (suite *AuthorizationValidatorTestSuite) TestValidateInitialAuthorizationRequest_WildcardRedirectURI() {
	tests := []struct {
		name             string
		registeredURIs   []string
		incomingURI      string
		wantError        bool
		wantErrorCode    string
		wantErrorMessage string
	}{
		{
			// AC-06: * matches exactly one path segment.
			name:           "SingleStarMatchesOneSegment",
			registeredURIs: []string{"https://client.example.com/cb/*"},
			incomingURI:    "https://client.example.com/cb/v1",
		},
		{
			// AC-06: * must not match two segments.
			name:             "SingleStarRejectsMultiSegment",
			registeredURIs:   []string{"https://client.example.com/cb/*"},
			incomingURI:      "https://client.example.com/cb/v1/extra",
			wantError:        true,
			wantErrorCode:    constants.ErrorInvalidRequest,
			wantErrorMessage: "Invalid redirect URI",
		},
		{
			// AC-07: ** matches multiple path segments.
			name:           "DoubleStarMatchesMultipleSegments",
			registeredURIs: []string{"https://client.example.com/app/**/cb"},
			incomingURI:    "https://client.example.com/app/tenant/region/cb",
		},
		{
			// AC-07: ** matches zero segments.
			name:           "DoubleStarMatchesZeroSegments",
			registeredURIs: []string{"https://client.example.com/app/**/cb"},
			incomingURI:    "https://client.example.com/app/cb",
		},
		{
			// AC-08: Exact match succeeds when no wildcard is registered.
			name:           "ExactMatchNoWildcard",
			registeredURIs: []string{"https://client.example.com/callback"},
			incomingURI:    "https://client.example.com/callback",
		},
		{
			// AC-09: No match returns invalid_request.
			name:             "NoMatchReturnsError",
			registeredURIs:   []string{"https://client.example.com/cb/*"},
			incomingURI:      "https://client.example.com/other",
			wantError:        true,
			wantErrorCode:    constants.ErrorInvalidRequest,
			wantErrorMessage: "Invalid redirect URI",
		},
		{
			// AC-10: Query param mismatch is rejected.
			name:             "QueryMismatchRejected",
			registeredURIs:   []string{"https://client.example.com/cb?foo=bar"},
			incomingURI:      "https://client.example.com/cb?foo=baz",
			wantError:        true,
			wantErrorCode:    constants.ErrorInvalidRequest,
			wantErrorMessage: "Invalid redirect URI",
		},
		{
			// AC-11: Multiple registered URIs — first match wins.
			name:           "MultipleURIsFirstMatchWins",
			registeredURIs: []string{"https://client.example.com/a/*", "https://client.example.com/b/*"},
			incomingURI:    "https://client.example.com/b/x",
		},
		{
			// AC-11: Multiple registered URIs — no match across any of them.
			name:             "MultipleURIsNoMatch",
			registeredURIs:   []string{"https://client.example.com/a/*", "https://client.example.com/b/*"},
			incomingURI:      "https://client.example.com/c/x",
			wantError:        true,
			wantErrorCode:    constants.ErrorInvalidRequest,
			wantErrorMessage: "Invalid redirect URI",
		},
		{
			// AC-12: Wildcard registered, redirect_uri omitted — must be rejected.
			name:             "OmittedRedirectURIWithWildcardRegistered",
			registeredURIs:   []string{"https://client.example.com/cb/*"},
			incomingURI:      "",
			wantError:        true,
			wantErrorCode:    constants.ErrorInvalidRequest,
			wantErrorMessage: "Invalid redirect URI",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			app := &inboundmodel.OAuthClient{
				ClientID:                "test-client-id",
				RedirectURIs:            tt.registeredURIs,
				GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
				ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
				TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
			}
			msg := &OAuthMessage{
				RequestQueryParams: map[string]string{
					constants.RequestParamClientID:     "test-client-id",
					constants.RequestParamRedirectURI:  tt.incomingURI,
					constants.RequestParamResponseType: string(constants.ResponseTypeCode),
				},
			}

			sendErrorToApp, errorCode, errorMessage := suite.validator.validateInitialAuthorizationRequest(msg, app)

			if tt.wantError {
				assert.False(suite.T(), sendErrorToApp)
				assert.Equal(suite.T(), tt.wantErrorCode, errorCode)
				assert.Equal(suite.T(), tt.wantErrorMessage, errorMessage)
			} else {
				assert.False(suite.T(), sendErrorToApp)
				assert.Empty(suite.T(), errorCode)
				assert.Empty(suite.T(), errorMessage)
			}
		})
	}
}
