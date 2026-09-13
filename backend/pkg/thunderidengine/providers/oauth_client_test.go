// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
func (suite *OAuthClientTestSuite) setupRuntime(t *testing.T, oauthCfg sysconfig.OAuthConfig) {
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
		suite.setupRuntime(t, sysconfig.OAuthConfig{PAR: engineconfig.PARConfig{RequirePAR: false}})
		assert.True(t, (&OAuthClient{RequirePushedAuthorizationRequests: true}).RequiresPAR())
	})

	suite.T().Run("global config forces PAR", func(t *testing.T) {
		suite.setupRuntime(t, sysconfig.OAuthConfig{PAR: engineconfig.PARConfig{RequirePAR: true}})
		assert.True(t, (&OAuthClient{RequirePushedAuthorizationRequests: false}).RequiresPAR())
	})

	suite.T().Run("neither forces PAR", func(t *testing.T) {
		suite.setupRuntime(t, sysconfig.OAuthConfig{PAR: engineconfig.PARConfig{RequirePAR: false}})
		assert.False(t, (&OAuthClient{RequirePushedAuthorizationRequests: false}).RequiresPAR())
	})
}

// ----- ValidateRedirectURI -----

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_ExactMatch() {
	suite.setupRuntime(suite.T(), sysconfig.OAuthConfig{})
	err := ValidateRedirectURI(context.Background(),
		[]string{"https://example.com/callback"}, "https://example.com/callback")
	assert.NoError(suite.T(), err)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_NoMatch() {
	suite.setupRuntime(suite.T(), sysconfig.OAuthConfig{})
	err := ValidateRedirectURI(context.Background(),
		[]string{"https://example.com/callback"}, "https://evil.com/callback")
	assert.Error(suite.T(), err)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_HTTPTransport() {
	suite.setupRuntime(suite.T(), sysconfig.OAuthConfig{})
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{name: "HTTPS remote host", uri: "https://example.com/callback"},
		{name: "HTTP localhost", uri: "http://localhost:3000/callback"},
		{name: "HTTP IPv4 loopback", uri: "http://127.0.0.1:3000/callback"},
		{name: "HTTP IPv6 loopback", uri: "http://[::1]:3000/callback"},
		{name: "HTTP remote host", uri: "http://example.com/callback", wantErr: true},
		{name: "HTTP private IP", uri: "http://192.168.1.10/callback", wantErr: true},
		{name: "HTTP other IPv4 loopback", uri: "http://127.0.0.2/callback", wantErr: true},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			err := ValidateRedirectURI(context.Background(), []string{tt.uri}, tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_EmptyURI() {
	suite.setupRuntime(suite.T(), sysconfig.OAuthConfig{})

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

	suite.T().Run("single HTTP loopback URI defaults to it", func(t *testing.T) {
		err := ValidateRedirectURI(context.Background(), []string{"http://localhost:3000/callback"}, "")
		assert.NoError(t, err)
	})

	suite.T().Run("single remote HTTP URI is rejected", func(t *testing.T) {
		err := ValidateRedirectURI(context.Background(), []string{"http://example.com/callback"}, "")
		assert.Error(t, err)
	})
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_FragmentRejected() {
	suite.setupRuntime(suite.T(), sysconfig.OAuthConfig{})
	err := ValidateRedirectURI(context.Background(),
		[]string{"https://example.com/callback#fragment"}, "https://example.com/callback#fragment")
	assert.Error(suite.T(), err)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_WildcardDisabled() {
	suite.setupRuntime(suite.T(), sysconfig.OAuthConfig{AllowWildcardRedirectURI: false})
	err := ValidateRedirectURI(context.Background(),
		[]string{"https://*.example.com/callback"}, "https://sub.example.com/callback")
	assert.Error(suite.T(), err)
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_WildcardEnabled() {
	suite.setupRuntime(suite.T(), sysconfig.OAuthConfig{AllowWildcardRedirectURI: true})
	err := ValidateRedirectURI(context.Background(),
		[]string{"https://*.example.com/callback"}, "https://sub.example.com/callback")
	assert.NoError(suite.T(), err)
}

func (suite *OAuthClientTestSuite) TestOAuthClient_ValidateRedirectURI() {
	suite.setupRuntime(suite.T(), sysconfig.OAuthConfig{})
	client := &OAuthClient{RedirectURIs: []string{"https://example.com/callback"}}
	assert.NoError(suite.T(), client.ValidateRedirectURI(context.Background(), "https://example.com/callback"))
	assert.Error(suite.T(), client.ValidateRedirectURI(context.Background(), "https://other.com/callback"))
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_InvalidRegisteredURI() {
	suite.setupRuntime(suite.T(), sysconfig.OAuthConfig{})
	err := ValidateRedirectURI(context.Background(), []string{"/relative/callback"}, "")
	assert.ErrorContains(suite.T(), err, "not fully qualified")
}

func (suite *OAuthClientTestSuite) TestValidateRedirectURI_SkipsInvalidWildcardPattern() {
	suite.setupRuntime(suite.T(), sysconfig.OAuthConfig{AllowWildcardRedirectURI: true})
	err := ValidateRedirectURI(context.Background(),
		[]string{"https://*", "https://example.com/callback"}, "https://example.com/callback")
	assert.NoError(suite.T(), err)
}
