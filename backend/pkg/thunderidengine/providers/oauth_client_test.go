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

package providers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

type OAuthClientTestSuite struct {
	suite.Suite
}

func TestOAuthClientSuite(t *testing.T) {
	suite.Run(t, new(OAuthClientTestSuite))
}

func (suite *OAuthClientTestSuite) SetupTest() {
	sysconfig.ResetServerRuntime()
}

func (suite *OAuthClientTestSuite) TearDownTest() {
	sysconfig.ResetServerRuntime()
}

// setupRuntime initializes a minimal runtime config for a specific subtest.
// It resets before initializing and registers cleanup via t.Cleanup.
func (suite *OAuthClientTestSuite) setupRuntime(t *testing.T, oauthCfg engineconfig.OAuthConfig) {
	t.Helper()
	sysconfig.ResetServerRuntime()
	cfg := &sysconfig.Config{OAuth: oauthCfg}
	require.NoError(t, sysconfig.InitializeServerRuntime(t.TempDir(), cfg))
	t.Cleanup(sysconfig.ResetServerRuntime)
}

// ----- IsAllowedGrantType (package-level) -----

func (suite *OAuthClientTestSuite) TestIsAllowedGrantType() {
	grantTypes := []GrantType{GrantTypeAuthorizationCode, GrantTypeRefreshToken}
	assert.True(suite.T(), IsAllowedGrantType(grantTypes, GrantTypeAuthorizationCode))
	assert.True(suite.T(), IsAllowedGrantType(grantTypes, GrantTypeRefreshToken))
	assert.False(suite.T(), IsAllowedGrantType(grantTypes, GrantTypeClientCredentials))
	assert.False(suite.T(), IsAllowedGrantType(grantTypes, ""))
}

// ----- IsAllowedResponseType (package-level) -----

func (suite *OAuthClientTestSuite) TestIsAllowedResponseType() {
	responseTypes := []ResponseType{ResponseTypeCode}
	assert.True(suite.T(), IsAllowedResponseType(responseTypes, string(ResponseTypeCode)))
	assert.False(suite.T(), IsAllowedResponseType(responseTypes, string(ResponseTypeIDToken)))
	assert.False(suite.T(), IsAllowedResponseType(responseTypes, ""))
}

// ----- OAuthClient methods -----

func (suite *OAuthClientTestSuite) TestOAuthClient_IsAllowedGrantType() {
	client := &OAuthClient{
		GrantTypes: []GrantType{GrantTypeAuthorizationCode, GrantTypeRefreshToken},
	}
	assert.True(suite.T(), client.IsAllowedGrantType(GrantTypeAuthorizationCode))
	assert.False(suite.T(), client.IsAllowedGrantType(GrantTypeClientCredentials))
}

func (suite *OAuthClientTestSuite) TestOAuthClient_IsAllowedResponseType() {
	client := &OAuthClient{ResponseTypes: []ResponseType{ResponseTypeCode}}
	assert.True(suite.T(), client.IsAllowedResponseType(string(ResponseTypeCode)))
	assert.False(suite.T(), client.IsAllowedResponseType(string(ResponseTypeIDToken)))
}

func (suite *OAuthClientTestSuite) TestOAuthClient_IsAllowedTokenEndpointAuthMethod() {
	client := &OAuthClient{TokenEndpointAuthMethod: TokenEndpointAuthMethodClientSecretBasic}
	assert.True(suite.T(), client.IsAllowedTokenEndpointAuthMethod(TokenEndpointAuthMethodClientSecretBasic))
	assert.False(suite.T(), client.IsAllowedTokenEndpointAuthMethod(TokenEndpointAuthMethodNone))
}

func (suite *OAuthClientTestSuite) TestOAuthClient_RequiresPKCE() {
	suite.T().Run("PKCERequired flag", func(t *testing.T) {
		assert.True(t, (&OAuthClient{PKCERequired: true}).RequiresPKCE())
	})
	suite.T().Run("PublicClient flag", func(t *testing.T) {
		assert.True(t, (&OAuthClient{PublicClient: true}).RequiresPKCE())
	})
	suite.T().Run("neither flag set", func(t *testing.T) {
		assert.False(t, (&OAuthClient{}).RequiresPKCE())
	})
}

func (suite *OAuthClientTestSuite) TestOAuthClient_ShouldAppendActorClaim() {
	suite.T().Run("agent always appends act claim", func(t *testing.T) {
		assert.True(t, (&OAuthClient{EntityCategory: EntityCategoryAgent}).ShouldAppendActorClaim())
	})
	suite.T().Run("app with IncludeActClaim true appends act claim", func(t *testing.T) {
		client := &OAuthClient{EntityCategory: EntityCategoryApp, IncludeActClaim: true}
		assert.True(t, client.ShouldAppendActorClaim())
	})
	suite.T().Run("app without IncludeActClaim does not append", func(t *testing.T) {
		assert.False(t, (&OAuthClient{EntityCategory: EntityCategoryApp}).ShouldAppendActorClaim())
	})
	suite.T().Run("user entity does not append", func(t *testing.T) {
		assert.False(t, (&OAuthClient{EntityCategory: EntityCategoryUser}).ShouldAppendActorClaim())
	})
}

func (suite *OAuthClientTestSuite) TestOAuthClient_ResolveDefaultAudience() {
	suite.T().Run("returns the configured default audience", func(t *testing.T) {
		client := &OAuthClient{Token: &OAuthTokenConfig{AccessToken: &AccessTokenConfig{
			DefaultAudience: "https://api.example.com",
		}}}
		assert.Equal(t, "https://api.example.com", client.ResolveDefaultAudience("client-123"))
	})
	suite.T().Run("falls back to client_id when default audience unset", func(t *testing.T) {
		client := &OAuthClient{Token: &OAuthTokenConfig{AccessToken: &AccessTokenConfig{}}}
		assert.Equal(t, "client-123", client.ResolveDefaultAudience("client-123"))
	})
	suite.T().Run("falls back to client_id when token config unset", func(t *testing.T) {
		assert.Equal(t, "client-123", (&OAuthClient{}).ResolveDefaultAudience("client-123"))
	})
}

func (suite *OAuthClientTestSuite) TestOAuthClient_RequiresPAR() {
	suite.T().Run("client flag forces PAR", func(t *testing.T) {
		suite.setupRuntime(t, engineconfig.OAuthConfig{PAR: engineconfig.PARConfig{RequirePAR: false}})
		assert.True(t, (&OAuthClient{RequirePushedAuthorizationRequests: true}).RequiresPAR())
	})

	suite.T().Run("global config forces PAR", func(t *testing.T) {
		suite.setupRuntime(t, engineconfig.OAuthConfig{PAR: engineconfig.PARConfig{RequirePAR: true}})
		assert.True(t, (&OAuthClient{RequirePushedAuthorizationRequests: false}).RequiresPAR())
	})

	suite.T().Run("neither forces PAR", func(t *testing.T) {
		suite.setupRuntime(t, engineconfig.OAuthConfig{PAR: engineconfig.PARConfig{RequirePAR: false}})
		assert.False(t, (&OAuthClient{RequirePushedAuthorizationRequests: false}).RequiresPAR())
	})
}

// ----- ValidateRedirectURI -----

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_ExactMatch() {
	suite.setupRuntime(suite.T(), engineconfig.OAuthConfig{})
	err := ValidateRedirectURI(context.Background(),
		[]string{"https://example.com/callback"}, "https://example.com/callback")
	assert.NoError(suite.T(), err)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_NoMatch() {
	suite.setupRuntime(suite.T(), engineconfig.OAuthConfig{})
	err := ValidateRedirectURI(context.Background(),
		[]string{"https://example.com/callback"}, "https://evil.com/callback")
	assert.Error(suite.T(), err)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_EmptyURI() {
	suite.setupRuntime(suite.T(), engineconfig.OAuthConfig{})

	suite.T().Run("single registered URI defaults to it", func(t *testing.T) {
		err := ValidateRedirectURI(context.Background(), []string{"https://example.com/callback"}, "")
		assert.NoError(t, err)
	})

	suite.T().Run("multiple registered URIs require explicit URI", func(t *testing.T) {
		err := ValidateRedirectURI(context.Background(), []string{"https://a.com/cb", "https://b.com/cb"}, "")
		assert.Error(t, err)
	})

	suite.T().Run("wildcard in single registered URI requires explicit URI", func(t *testing.T) {
		err := ValidateRedirectURI(context.Background(), []string{"https://*.example.com/callback"}, "")
		assert.Error(t, err)
	})
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_FragmentRejected() {
	suite.setupRuntime(suite.T(), engineconfig.OAuthConfig{})
	err := ValidateRedirectURI(context.Background(),
		[]string{"https://example.com/callback#fragment"}, "https://example.com/callback#fragment")
	assert.Error(suite.T(), err)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_WildcardDisabled() {
	suite.setupRuntime(suite.T(), engineconfig.OAuthConfig{AllowWildcardRedirectURI: false})
	err := ValidateRedirectURI(context.Background(),
		[]string{"https://*.example.com/callback"}, "https://sub.example.com/callback")
	assert.Error(suite.T(), err)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_WildcardEnabled() {
	suite.setupRuntime(suite.T(), engineconfig.OAuthConfig{AllowWildcardRedirectURI: true})
	err := ValidateRedirectURI(context.Background(),
		[]string{"https://*.example.com/callback"}, "https://sub.example.com/callback")
	assert.NoError(suite.T(), err)
}

func (suite *OAuthClientTestSuite) TestOAuthClient_ValidateRedirectURI() {
	suite.setupRuntime(suite.T(), engineconfig.OAuthConfig{})
	client := &OAuthClient{RedirectURIs: []string{"https://example.com/callback"}}
	assert.NoError(suite.T(), client.ValidateRedirectURI(context.Background(), "https://example.com/callback"))
	assert.Error(suite.T(), client.ValidateRedirectURI(context.Background(), "https://other.com/callback"))
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_InvalidRegisteredURI() {
	suite.setupRuntime(suite.T(), engineconfig.OAuthConfig{})
	err := ValidateRedirectURI(context.Background(), []string{"/relative/callback"}, "")
	assert.ErrorContains(suite.T(), err, "not fully qualified")
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_SkipsInvalidWildcardPattern() {
	suite.setupRuntime(suite.T(), engineconfig.OAuthConfig{AllowWildcardRedirectURI: true})
	err := ValidateRedirectURI(context.Background(),
		[]string{"https://*", "https://example.com/callback"}, "https://example.com/callback")
	assert.NoError(suite.T(), err)
}
