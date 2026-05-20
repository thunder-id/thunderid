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

package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
)

const (
	errRedirectURIFragment          = "redirect URI must not contain a fragment component"
	errRedirectURINotRegistered     = "your application's redirect URL does not match with the registered redirect URLs"
	errRedirectURIRequired          = "redirect URI is required in the authorization request"
	errRedirectURINotFullyQualified = "registered redirect URI is not fully qualified"
)

type OAuthClientTestSuite struct {
	suite.Suite
}

func TestOAuthClientTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthClientTestSuite))
}

func (suite *OAuthClientTestSuite) SetupTest() {
	sysconfig.ResetServerRuntime()
	suite.Require().NoError(sysconfig.InitializeServerRuntime("/tmp/test", &sysconfig.Config{}))
}

func (suite *OAuthClientTestSuite) TestIsAllowedGrantType_AuthorizationCode() {
	c := &model.OAuthClient{
		GrantTypes: []oauth2const.GrantType{
			oauth2const.GrantTypeAuthorizationCode,
			oauth2const.GrantTypeRefreshToken,
		},
	}

	suite.True(c.IsAllowedGrantType(oauth2const.GrantTypeAuthorizationCode))
}

func (suite *OAuthClientTestSuite) TestIsAllowedGrantType_ClientCredentials() {
	c := &model.OAuthClient{
		GrantTypes: []oauth2const.GrantType{
			oauth2const.GrantTypeClientCredentials,
		},
	}

	suite.True(c.IsAllowedGrantType(oauth2const.GrantTypeClientCredentials))
}

func (suite *OAuthClientTestSuite) TestIsAllowedGrantType_RefreshToken() {
	c := &model.OAuthClient{
		GrantTypes: []oauth2const.GrantType{
			oauth2const.GrantTypeRefreshToken,
		},
	}

	suite.True(c.IsAllowedGrantType(oauth2const.GrantTypeRefreshToken))
}

func (suite *OAuthClientTestSuite) TestIsAllowedGrantType_TokenExchange() {
	c := &model.OAuthClient{
		GrantTypes: []oauth2const.GrantType{
			oauth2const.GrantTypeTokenExchange,
		},
	}

	suite.True(c.IsAllowedGrantType(oauth2const.GrantTypeTokenExchange))
}

func (suite *OAuthClientTestSuite) TestIsAllowedGrantType_NotAllowed() {
	c := &model.OAuthClient{
		GrantTypes: []oauth2const.GrantType{
			oauth2const.GrantTypeAuthorizationCode,
		},
	}

	suite.False(c.IsAllowedGrantType(oauth2const.GrantTypeClientCredentials))
}

func (suite *OAuthClientTestSuite) TestIsAllowedGrantType_EmptyGrantType() {
	c := &model.OAuthClient{
		GrantTypes: []oauth2const.GrantType{
			oauth2const.GrantTypeAuthorizationCode,
		},
	}

	suite.False(c.IsAllowedGrantType(""))
}

func (suite *OAuthClientTestSuite) TestIsAllowedGrantType_EmptyGrantTypesList() {
	c := &model.OAuthClient{
		GrantTypes: []oauth2const.GrantType{},
	}

	suite.False(c.IsAllowedGrantType(oauth2const.GrantTypeAuthorizationCode))
}

func (suite *OAuthClientTestSuite) TestIsAllowedGrantType_NilGrantTypesList() {
	c := &model.OAuthClient{
		GrantTypes: nil,
	}

	suite.False(c.IsAllowedGrantType(oauth2const.GrantTypeAuthorizationCode))
}

func (suite *OAuthClientTestSuite) TestIsAllowedGrantType_MultipleGrantTypes() {
	c := &model.OAuthClient{
		GrantTypes: []oauth2const.GrantType{
			oauth2const.GrantTypeAuthorizationCode,
			oauth2const.GrantTypeClientCredentials,
			oauth2const.GrantTypeRefreshToken,
			oauth2const.GrantTypeTokenExchange,
		},
	}

	suite.True(c.IsAllowedGrantType(oauth2const.GrantTypeAuthorizationCode))
	suite.True(c.IsAllowedGrantType(oauth2const.GrantTypeClientCredentials))
	suite.True(c.IsAllowedGrantType(oauth2const.GrantTypeRefreshToken))
	suite.True(c.IsAllowedGrantType(oauth2const.GrantTypeTokenExchange))
}

func (suite *OAuthClientTestSuite) TestIsAllowedResponseType_Code() {
	c := &model.OAuthClient{
		ResponseTypes: []oauth2const.ResponseType{
			oauth2const.ResponseTypeCode,
		},
	}

	suite.True(c.IsAllowedResponseType("code"))
}

func (suite *OAuthClientTestSuite) TestIsAllowedResponseType_NotAllowed() {
	c := &model.OAuthClient{
		ResponseTypes: []oauth2const.ResponseType{
			oauth2const.ResponseTypeCode,
		},
	}

	suite.False(c.IsAllowedResponseType("token"))
}

func (suite *OAuthClientTestSuite) TestIsAllowedResponseType_EmptyResponseType() {
	c := &model.OAuthClient{
		ResponseTypes: []oauth2const.ResponseType{
			oauth2const.ResponseTypeCode,
		},
	}

	suite.False(c.IsAllowedResponseType(""))
}

func (suite *OAuthClientTestSuite) TestIsAllowedResponseType_EmptyResponseTypesList() {
	c := &model.OAuthClient{
		ResponseTypes: []oauth2const.ResponseType{},
	}

	suite.False(c.IsAllowedResponseType("code"))
}

func (suite *OAuthClientTestSuite) TestIsAllowedResponseType_NilResponseTypesList() {
	c := &model.OAuthClient{
		ResponseTypes: nil,
	}

	suite.False(c.IsAllowedResponseType("code"))
}

func (suite *OAuthClientTestSuite) TestIsAllowedResponseType_MultipleResponseTypes() {
	c := &model.OAuthClient{
		ResponseTypes: []oauth2const.ResponseType{
			oauth2const.ResponseTypeCode,
			"token",
			"id_token",
		},
	}

	suite.True(c.IsAllowedResponseType("code"))
	suite.True(c.IsAllowedResponseType("token"))
	suite.True(c.IsAllowedResponseType("id_token"))
}

func (suite *OAuthClientTestSuite) TestIsAllowedTokenEndpointAuthMethod_ClientSecretBasic() {
	c := &model.OAuthClient{
		TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodClientSecretBasic,
	}

	suite.True(c.IsAllowedTokenEndpointAuthMethod(oauth2const.TokenEndpointAuthMethodClientSecretBasic))
}

func (suite *OAuthClientTestSuite) TestIsAllowedTokenEndpointAuthMethod_ClientSecretPost() {
	c := &model.OAuthClient{
		TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodClientSecretPost,
	}

	suite.True(c.IsAllowedTokenEndpointAuthMethod(oauth2const.TokenEndpointAuthMethodClientSecretPost))
}

func (suite *OAuthClientTestSuite) TestIsAllowedTokenEndpointAuthMethod_None() {
	c := &model.OAuthClient{
		TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodNone,
	}

	suite.True(c.IsAllowedTokenEndpointAuthMethod(oauth2const.TokenEndpointAuthMethodNone))
}

func (suite *OAuthClientTestSuite) TestIsAllowedTokenEndpointAuthMethod_NotAllowed() {
	c := &model.OAuthClient{
		TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodClientSecretBasic,
	}

	suite.False(c.IsAllowedTokenEndpointAuthMethod(oauth2const.TokenEndpointAuthMethodClientSecretPost))
}

func (suite *OAuthClientTestSuite) TestIsAllowedTokenEndpointAuthMethod_Empty() {
	c := &model.OAuthClient{
		TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodClientSecretBasic,
	}

	suite.False(c.IsAllowedTokenEndpointAuthMethod(""))
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_ValidWithSingleRegisteredURI() {
	c := &model.OAuthClient{
		RedirectURIs: []string{"https://example.com/callback"},
	}

	suite.NoError(c.ValidateRedirectURI("https://example.com/callback"))
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_ValidHTTPLocalhostWithPort() {
	c := &model.OAuthClient{
		RedirectURIs: []string{"http://localhost:3000/callback"},
	}

	suite.NoError(c.ValidateRedirectURI("http://localhost:3000/callback"))
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_ValidHTTPSWithPath() {
	c := &model.OAuthClient{
		RedirectURIs: []string{"https://app.example.com/oauth/callback"},
	}

	suite.NoError(c.ValidateRedirectURI("https://app.example.com/oauth/callback"))
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_ValidCustomScheme() {
	c := &model.OAuthClient{
		RedirectURIs: []string{"myapp://callback"},
	}

	suite.NoError(c.ValidateRedirectURI("myapp://callback"))
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_ValidWithQueryParameters() {
	c := &model.OAuthClient{
		RedirectURIs: []string{"https://example.com/callback?param=value"},
	}

	suite.NoError(c.ValidateRedirectURI("https://example.com/callback?param=value"))
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_InvalidWithFragment() {
	c := &model.OAuthClient{
		RedirectURIs: []string{"https://example.com/callback#fragment"},
	}

	err := c.ValidateRedirectURI("https://example.com/callback#fragment")
	suite.EqualError(err, errRedirectURIFragment)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_NotRegistered() {
	c := &model.OAuthClient{
		RedirectURIs: []string{"https://example.com/callback"},
	}

	err := c.ValidateRedirectURI("https://different.com/callback")
	suite.EqualError(err, errRedirectURINotRegistered)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_EmptyWithSingleFullyQualifiedURI() {
	c := &model.OAuthClient{
		RedirectURIs: []string{"https://example.com/callback"},
	}

	suite.NoError(c.ValidateRedirectURI(""))
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_EmptyWithMultipleURIs() {
	c := &model.OAuthClient{
		RedirectURIs: []string{
			"https://example.com/callback",
			"https://example.com/callback2",
		},
	}

	err := c.ValidateRedirectURI("")
	suite.EqualError(err, errRedirectURIRequired)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_EmptyWithPartialRegisteredURI() {
	c := &model.OAuthClient{
		RedirectURIs: []string{"/callback"},
	}

	err := c.ValidateRedirectURI("")
	suite.EqualError(err, errRedirectURINotFullyQualified)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_EmptyWithInvalidRegisteredURI() {
	c := &model.OAuthClient{
		RedirectURIs: []string{"://invalid"},
	}

	err := c.ValidateRedirectURI("")
	suite.EqualError(err, errRedirectURINotFullyQualified)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_EmptyRedirectURIsList() {
	c := &model.OAuthClient{
		RedirectURIs: []string{},
	}

	err := c.ValidateRedirectURI("")
	suite.EqualError(err, errRedirectURIRequired)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_NilRedirectURIsList() {
	c := &model.OAuthClient{
		RedirectURIs: nil,
	}

	err := c.ValidateRedirectURI("")
	suite.EqualError(err, errRedirectURIRequired)
}

func (suite *OAuthClientTestSuite) TestRequiresPKCE_PKCERequiredTrue() {
	c := &model.OAuthClient{PKCERequired: true, PublicClient: false}
	suite.True(c.RequiresPKCE())
}

func (suite *OAuthClientTestSuite) TestRequiresPKCE_PublicClientTrue() {
	c := &model.OAuthClient{PKCERequired: false, PublicClient: true}
	suite.True(c.RequiresPKCE())
}

func (suite *OAuthClientTestSuite) TestRequiresPKCE_BothTrue() {
	c := &model.OAuthClient{PKCERequired: true, PublicClient: true}
	suite.True(c.RequiresPKCE())
}

func (suite *OAuthClientTestSuite) TestRequiresPKCE_BothFalse() {
	c := &model.OAuthClient{PKCERequired: false, PublicClient: false}
	suite.False(c.RequiresPKCE())
}

type OAuthHelperTestSuite struct {
	suite.Suite
}

func TestOAuthHelperTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthHelperTestSuite))
}

func (suite *OAuthHelperTestSuite) SetupTest() {
	sysconfig.ResetServerRuntime()
	suite.Require().NoError(sysconfig.InitializeServerRuntime("/tmp/test", &sysconfig.Config{}))
}

func (suite *OAuthHelperTestSuite) TestIsAllowedGrantType_ValidGrantType() {
	grantTypes := []oauth2const.GrantType{
		oauth2const.GrantTypeAuthorizationCode,
		oauth2const.GrantTypeRefreshToken,
	}

	suite.True(model.IsAllowedGrantType(grantTypes, oauth2const.GrantTypeAuthorizationCode))
}

func (suite *OAuthHelperTestSuite) TestIsAllowedGrantType_InvalidGrantType() {
	grantTypes := []oauth2const.GrantType{
		oauth2const.GrantTypeAuthorizationCode,
	}

	suite.False(model.IsAllowedGrantType(grantTypes, oauth2const.GrantTypeClientCredentials))
}

func (suite *OAuthHelperTestSuite) TestIsAllowedGrantType_EmptyGrantType() {
	grantTypes := []oauth2const.GrantType{
		oauth2const.GrantTypeAuthorizationCode,
	}

	suite.False(model.IsAllowedGrantType(grantTypes, ""))
}

func (suite *OAuthHelperTestSuite) TestIsAllowedGrantType_EmptyList() {
	suite.False(model.IsAllowedGrantType([]oauth2const.GrantType{}, oauth2const.GrantTypeAuthorizationCode))
}

func (suite *OAuthHelperTestSuite) TestIsAllowedGrantType_NilList() {
	suite.False(model.IsAllowedGrantType(nil, oauth2const.GrantTypeAuthorizationCode))
}

func (suite *OAuthHelperTestSuite) TestIsAllowedResponseType_ValidResponseType() {
	responseTypes := []oauth2const.ResponseType{
		oauth2const.ResponseTypeCode,
		"token",
	}

	suite.True(model.IsAllowedResponseType(responseTypes, "code"))
}

func (suite *OAuthHelperTestSuite) TestIsAllowedResponseType_InvalidResponseType() {
	responseTypes := []oauth2const.ResponseType{
		oauth2const.ResponseTypeCode,
	}

	suite.False(model.IsAllowedResponseType(responseTypes, "token"))
}

func (suite *OAuthHelperTestSuite) TestIsAllowedResponseType_EmptyResponseType() {
	responseTypes := []oauth2const.ResponseType{
		oauth2const.ResponseTypeCode,
	}

	suite.False(model.IsAllowedResponseType(responseTypes, ""))
}

func (suite *OAuthHelperTestSuite) TestIsAllowedResponseType_EmptyList() {
	suite.False(model.IsAllowedResponseType([]oauth2const.ResponseType{}, "code"))
}

func (suite *OAuthHelperTestSuite) TestIsAllowedResponseType_NilList() {
	suite.False(model.IsAllowedResponseType(nil, "code"))
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_ValidSingleURI() {
	err := model.ValidateRedirectURI([]string{"https://example.com/callback"}, "https://example.com/callback")
	suite.NoError(err)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_ValidMultipleURIs() {
	redirectURIs := []string{
		"https://example.com/callback",
		"https://example.com/callback2",
	}

	err := model.ValidateRedirectURI(redirectURIs, "https://example.com/callback2")
	suite.NoError(err)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_InvalidNotRegistered() {
	err := model.ValidateRedirectURI([]string{"https://example.com/callback"}, "https://different.com/callback")
	suite.EqualError(err, errRedirectURINotRegistered)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_InvalidWithFragment() {
	uri := "https://example.com/callback#fragment"
	err := model.ValidateRedirectURI([]string{uri}, uri)
	suite.EqualError(err, errRedirectURIFragment)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_EmptyURIWithSingleFullyQualified() {
	err := model.ValidateRedirectURI([]string{"https://example.com/callback"}, "")
	suite.NoError(err)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_EmptyURIWithMultiple() {
	redirectURIs := []string{
		"https://example.com/callback",
		"https://example.com/callback2",
	}

	err := model.ValidateRedirectURI(redirectURIs, "")
	suite.EqualError(err, errRedirectURIRequired)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_EmptyURIWithPartialRegistered() {
	err := model.ValidateRedirectURI([]string{"/callback"}, "")
	suite.EqualError(err, errRedirectURINotFullyQualified)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_EmptyURIWithNoScheme() {
	err := model.ValidateRedirectURI([]string{"example.com/callback"}, "")
	suite.EqualError(err, errRedirectURINotFullyQualified)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_EmptyURIWithNoHost() {
	err := model.ValidateRedirectURI([]string{"https:///callback"}, "")
	suite.EqualError(err, errRedirectURINotFullyQualified)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_EmptyURIList() {
	err := model.ValidateRedirectURI([]string{}, "")
	suite.EqualError(err, errRedirectURIRequired)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_NilList() {
	err := model.ValidateRedirectURI(nil, "")
	suite.EqualError(err, errRedirectURIRequired)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_CustomScheme() {
	err := model.ValidateRedirectURI([]string{"myapp://callback"}, "myapp://callback")
	suite.NoError(err)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_LocalhostHTTP() {
	err := model.ValidateRedirectURI([]string{"http://localhost:3000/callback"}, "http://localhost:3000/callback")
	suite.NoError(err)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_WithQueryParams() {
	uri := "https://example.com/callback?foo=bar"
	suite.NoError(model.ValidateRedirectURI([]string{uri}, uri))
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_IPAddress() {
	err := model.ValidateRedirectURI([]string{"https://192.168.1.1/callback"}, "https://192.168.1.1/callback")
	suite.NoError(err)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_Localhost127() {
	err := model.ValidateRedirectURI([]string{"http://127.0.0.1:8080/callback"}, "http://127.0.0.1:8080/callback")
	suite.NoError(err)
}

func (suite *OAuthHelperTestSuite) TestValidateRedirectURI_InvalidURLFormat() {
	uri := "http://example.com/callback\x00invalid"
	err := model.ValidateRedirectURI([]string{uri}, uri)
	suite.Error(err)
	assert.Contains(suite.T(), err.Error(), "invalid redirect URI")
}

func (suite *OAuthClientTestSuite) TestRequiresPAR_GlobalConfigEnabled() {
	sysconfig.ResetServerRuntime()
	cfg := &sysconfig.Config{}
	cfg.OAuth.PAR.RequirePAR = true
	suite.Require().NoError(sysconfig.InitializeServerRuntime("/tmp/test", cfg))

	c := &model.OAuthClient{RequirePushedAuthorizationRequests: false}
	suite.True(c.RequiresPAR())
}

func (suite *OAuthClientTestSuite) TestRequiresPAR_PerClientEnabled() {
	c := &model.OAuthClient{RequirePushedAuthorizationRequests: true}
	suite.True(c.RequiresPAR())
}

func (suite *OAuthClientTestSuite) TestRequiresPAR_BothFalse() {
	c := &model.OAuthClient{RequirePushedAuthorizationRequests: false}
	suite.False(c.RequiresPAR())
}

func (suite *OAuthHelperTestSuite) TestMatchAnyRedirectURIPattern_WildcardEnabled_Matches() {
	sysconfig.ResetServerRuntime()
	cfg := &sysconfig.Config{}
	cfg.OAuth.AllowWildcardRedirectURI = true
	suite.Require().NoError(sysconfig.InitializeServerRuntime("/tmp/test", cfg))

	err := model.ValidateRedirectURI(
		[]string{"https://app.example.com/*"},
		"https://app.example.com/cb",
	)
	suite.NoError(err)
}

func (suite *OAuthHelperTestSuite) TestMatchAnyRedirectURIPattern_WildcardDisabled_NoMatch() {
	err := model.ValidateRedirectURI(
		[]string{"https://app.example.com/*"},
		"https://app.example.com/cb",
	)
	suite.Error(err)
}

func (suite *OAuthHelperTestSuite) TestMatchAnyRedirectURIPattern_HostWildcardEnabled_Matches() {
	sysconfig.ResetServerRuntime()
	cfg := &sysconfig.Config{}
	cfg.OAuth.AllowWildcardRedirectURI = true
	suite.Require().NoError(sysconfig.InitializeServerRuntime("/tmp/test", cfg))

	err := model.ValidateRedirectURI(
		[]string{"https://tenant-app-*-*.gateway.example.com/cb"},
		"https://tenant-app-019dfc78-f19ab4f2.gateway.example.com/cb",
	)
	suite.NoError(err)
}

func (suite *OAuthHelperTestSuite) TestMatchAnyRedirectURIPattern_HostWildcardEnabled_NonMatchingDynamicPart() {
	sysconfig.ResetServerRuntime()
	cfg := &sysconfig.Config{}
	cfg.OAuth.AllowWildcardRedirectURI = true
	suite.Require().NoError(sysconfig.InitializeServerRuntime("/tmp/test", cfg))

	// Hyphen inside the dynamic part is not in [0-9a-zA-Z]+, so this must fail.
	err := model.ValidateRedirectURI(
		[]string{"https://app-*-prod.example.com/cb"},
		"https://app-foo-bar-prod.example.com/cb",
	)
	suite.Error(err)
}

func (suite *OAuthHelperTestSuite) TestMatchAnyRedirectURIPattern_HostWildcardDisabled_NoMatch() {
	// Default: AllowWildcardRedirectURI = false. Note the pattern would never have made it
	// past registration with the flag off, but we still verify the matcher returns no match.
	err := model.ValidateRedirectURI(
		[]string{"https://app-*.example.com/cb"},
		"https://app-prod.example.com/cb",
	)
	suite.Error(err)
}

func (suite *OAuthHelperTestSuite) TestMatchAnyRedirectURIPattern_HostWildcardDoesNotCrossDot() {
	sysconfig.ResetServerRuntime()
	cfg := &sysconfig.Config{}
	cfg.OAuth.AllowWildcardRedirectURI = true
	suite.Require().NoError(sysconfig.InitializeServerRuntime("/tmp/test", cfg))

	err := model.ValidateRedirectURI(
		[]string{"https://app-*.example.com/cb"},
		"https://app-foo.evil.example.com/cb",
	)
	suite.Error(err)
}
