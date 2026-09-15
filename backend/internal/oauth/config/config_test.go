// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package oauthconfig

import (
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
)

type OAuthConfigTestSuite struct {
	suite.Suite
}

func TestOAuthConfigTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthConfigTestSuite))
}

func (s *OAuthConfigTestSuite) SetupTest() {
	config.ResetServerRuntime()
}

func (s *OAuthConfigTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *OAuthConfigTestSuite) TestFromServerRuntime() {
	cfg := &config.Config{
		Server: engineconfig.ServerConfig{
			Identifier: "dep-1",
			Hostname:   "thunder.io",
			Port:       443,
			PublicURL:  "https://thunder.io",
		},
		Database: config.DatabaseConfig{
			RuntimeTransient: config.DataSource{Type: "sqlite"},
		},
		JWT: engineconfig.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
		OAuth: config.OAuthConfig{
			PAR: engineconfig.PARConfig{ExpiresIn: 600},
		},
		GateClient: engineconfig.GateClientConfig{
			Scheme:   "https",
			Hostname: "localhost",
			Port:     3000,
		},
	}
	err := config.InitializeServerRuntime("/tmp/test-oauth-config", cfg)
	s.Require().NoError(err)

	result := FromServerRuntime()

	s.Equal("dep-1", result.DeploymentID)
	s.Equal("sqlite", result.RuntimeTransientDBType)
	s.Equal("https://thunder.io", result.BaseURL)
	s.Equal("https://thunder.io", result.JWT.Issuer)
	s.Equal(int64(600), result.OAuth.PAR.ExpiresIn)
	s.Equal("localhost", result.GateClient.Hostname)

	s.NotEmpty(result.OAuth.DefaultScopeClaimsMapping, "default mapping should be seeded")
	s.Len(result.OAuth.DefaultScopeClaimsMapping, len(constants.StandardOIDCScopes))
	for scope, def := range constants.StandardOIDCScopes {
		s.ElementsMatch(def.Claims, result.OAuth.DefaultScopeClaimsMapping[scope], "scope %q claims mismatch", scope)
	}
	standardScopeNames := make([]string, 0, len(constants.StandardOIDCScopes))
	for scope := range constants.StandardOIDCScopes {
		standardScopeNames = append(standardScopeNames, scope)
	}
	s.ElementsMatch(standardScopeNames, result.OAuth.AllowedScopes)
	s.ElementsMatch([]string{constants.SubjectTypePublic}, result.OAuth.AllowedSubjectTypes)
	for _, c := range constants.GetStandardClaims() {
		s.Contains(result.OAuth.AllowedClaims, c, "allowed_claims must include standard JWT claim %q", c)
	}
	for _, def := range constants.StandardOIDCScopes {
		for _, c := range def.Claims {
			s.Contains(result.OAuth.AllowedClaims, c, "allowed_claims must include mapped claim %q", c)
		}
	}
}

func (s *OAuthConfigTestSuite) TestApplyOIDCDefaults_Idempotent() {
	var oauth engineconfig.OAuthConfig
	applyOIDCDefaults(&oauth)
	first := oauth
	applyOIDCDefaults(&oauth)
	s.ElementsMatch(first.AllowedScopes, oauth.AllowedScopes)
	s.ElementsMatch(first.AllowedClaims, oauth.AllowedClaims)
	s.ElementsMatch(first.AllowedSubjectTypes, oauth.AllowedSubjectTypes)
	s.Equal(first.DefaultScopeClaimsMapping, oauth.DefaultScopeClaimsMapping)
	s.ElementsMatch(first.AllowedGrantTypes, oauth.AllowedGrantTypes)
	s.ElementsMatch(first.AllowedResponseTypes, oauth.AllowedResponseTypes)
	s.ElementsMatch(first.AllowedAuthMethods, oauth.AllowedAuthMethods)
}

func (s *OAuthConfigTestSuite) TestApplyOIDCDefaults_SeedsAllowedListsWhenEmpty() {
	var oauth engineconfig.OAuthConfig
	applyOIDCDefaults(&oauth)
	s.NotEmpty(oauth.AllowedGrantTypes)
	s.NotEmpty(oauth.AllowedResponseTypes)
	s.NotEmpty(oauth.AllowedAuthMethods)
	s.Contains(oauth.AllowedGrantTypes, "authorization_code")
	s.Contains(oauth.AllowedResponseTypes, "code")
	s.Contains(oauth.AllowedAuthMethods, "client_secret_basic")
}

func (s *OAuthConfigTestSuite) TestApplyOIDCDefaults_PreservesConfiguredAllowedLists() {
	oauth := engineconfig.OAuthConfig{
		AllowedGrantTypes:    []string{"client_credentials"},
		AllowedResponseTypes: []string{"code"},
		AllowedAuthMethods:   []string{"client_secret_post"},
	}
	applyOIDCDefaults(&oauth)
	s.Equal([]string{"client_credentials"}, oauth.AllowedGrantTypes)
	s.Equal([]string{"code"}, oauth.AllowedResponseTypes)
	s.Equal([]string{"client_secret_post"}, oauth.AllowedAuthMethods)
}
