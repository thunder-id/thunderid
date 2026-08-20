// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/cors"
)

func boolPtr(b bool) *bool { return &b }

type ValidateTestSuite struct {
	suite.Suite
}

func TestValidateSuite(t *testing.T) {
	suite.Run(t, new(ValidateTestSuite))
}

// ----- GetServerURL -----

func (suite *ValidateTestSuite) TestGetServerURL_PublicURL() {
	cfg := &ServerConfig{PublicURL: "https://example.com"}
	assert.Equal(suite.T(), "https://example.com", GetServerURL(cfg))
}

func (suite *ValidateTestSuite) TestGetServerURL_HTTPSScheme() {
	cfg := &ServerConfig{Hostname: "localhost", Port: 9090}
	assert.Equal(suite.T(), "https://localhost:9090", GetServerURL(cfg))
}

func (suite *ValidateTestSuite) TestGetServerURL_HTTPScheme() {
	cfg := &ServerConfig{Hostname: "localhost", Port: 8080, HTTPOnly: true}
	assert.Equal(suite.T(), "http://localhost:8080", GetServerURL(cfg))
}

// ----- TrustedIssuerConfig -----

func (suite *ValidateTestSuite) TestTrustedIssuerConfig_IsConfigured() {
	assert.False(suite.T(), (&TrustedIssuerConfig{}).IsConfigured())
	assert.True(suite.T(), (&TrustedIssuerConfig{Issuer: "https://issuer.example.com"}).IsConfigured())
}

func (suite *ValidateTestSuite) TestTrustedIssuerConfig_Validate() {
	suite.T().Run("unconfigured passes", func(t *testing.T) {
		assert.NoError(t, (&TrustedIssuerConfig{}).Validate())
	})

	suite.T().Run("missing JWKS URL", func(t *testing.T) {
		c := &TrustedIssuerConfig{Issuer: "https://issuer.example.com", Audience: "aud"}
		assert.ErrorContains(t, c.Validate(), "jwks_url must be set")
	})

	suite.T().Run("missing audience", func(t *testing.T) {
		c := &TrustedIssuerConfig{
			Issuer:  "https://issuer.example.com",
			JWKSURL: "https://issuer.example.com/.well-known/jwks.json",
		}
		assert.ErrorContains(t, c.Validate(), "audience must be set")
	})

	suite.T().Run("HTTPS JWKS URL passes", func(t *testing.T) {
		c := &TrustedIssuerConfig{
			Issuer:   "https://issuer.example.com",
			JWKSURL:  "https://issuer.example.com/.well-known/jwks.json",
			Audience: "https://api.example.com",
		}
		assert.NoError(t, c.Validate())
	})

	suite.T().Run("localhost HTTP JWKS URL passes", func(t *testing.T) {
		c := &TrustedIssuerConfig{
			Issuer:   "https://issuer.example.com",
			JWKSURL:  "http://localhost:8080/.well-known/jwks.json",
			Audience: "https://api.example.com",
		}
		assert.NoError(t, c.Validate())
	})

	suite.T().Run("127.0.0.1 HTTP JWKS URL passes", func(t *testing.T) {
		c := &TrustedIssuerConfig{
			Issuer:   "https://issuer.example.com",
			JWKSURL:  "http://127.0.0.1:8080/.well-known/jwks.json",
			Audience: "https://api.example.com",
		}
		assert.NoError(t, c.Validate())
	})

	suite.T().Run("non-localhost HTTP JWKS URL fails", func(t *testing.T) {
		c := &TrustedIssuerConfig{
			Issuer:   "https://issuer.example.com",
			JWKSURL:  "http://remote.example.com/.well-known/jwks.json",
			Audience: "https://api.example.com",
		}
		assert.ErrorContains(t, c.Validate(), "https")
	})

	suite.T().Run("unsupported scheme fails", func(t *testing.T) {
		c := &TrustedIssuerConfig{
			Issuer:   "https://issuer.example.com",
			JWKSURL:  "ftp://issuer.example.com/.well-known/jwks.json",
			Audience: "https://api.example.com",
		}
		assert.ErrorContains(t, c.Validate(), "https scheme")
	})
}

// ----- SecurityConfig -----

func (suite *ValidateTestSuite) TestSecurityConfig_Validate() {
	suite.T().Run("negative JWKSCacheTTL fails", func(t *testing.T) {
		assert.ErrorContains(t, (&SecurityConfig{JWKSCacheTTL: -1}).Validate(), "jwks_cache_ttl")
	})

	suite.T().Run("zero JWKSCacheTTL passes", func(t *testing.T) {
		assert.NoError(t, (&SecurityConfig{JWKSCacheTTL: 0}).Validate())
	})

	suite.T().Run("positive JWKSCacheTTL passes", func(t *testing.T) {
		assert.NoError(t, (&SecurityConfig{JWKSCacheTTL: 300}).Validate())
	})

	suite.T().Run("propagates TrustedIssuer error", func(t *testing.T) {
		c := &SecurityConfig{
			TrustedIssuer: TrustedIssuerConfig{Issuer: "https://issuer.example.com"},
		}
		assert.Error(t, c.Validate())
	})

	suite.T().Run("propagates TokenRevocation error", func(t *testing.T) {
		c := &SecurityConfig{
			TokenRevocation: TokenRevocationConfig{Enabled: boolPtr(true), SyncIntervalSeconds: -1},
		}
		assert.ErrorContains(t, c.Validate(), "sync_interval_seconds")
	})
}

// ----- TokenRevocationConfig -----

func (suite *ValidateTestSuite) TestTokenRevocationConfig_Validate() {
	suite.T().Run("disabled skips validation", func(t *testing.T) {
		assert.NoError(t, (&TokenRevocationConfig{Enabled: boolPtr(false), SyncIntervalSeconds: -1}).Validate())
	})

	suite.T().Run("negative interval fails when enabled", func(t *testing.T) {
		assert.ErrorContains(t,
			(&TokenRevocationConfig{Enabled: boolPtr(true), SyncIntervalSeconds: -1}).Validate(),
			"sync_interval_seconds")
	})

	suite.T().Run("zero interval passes when enabled", func(t *testing.T) {
		assert.NoError(t, (&TokenRevocationConfig{Enabled: boolPtr(true), SyncIntervalSeconds: 0}).Validate())
	})

	suite.T().Run("positive interval passes when enabled", func(t *testing.T) {
		assert.NoError(t,
			(&TokenRevocationConfig{Enabled: boolPtr(true), Source: "db", SyncIntervalSeconds: 30}).Validate())
	})

	suite.T().Run("empty source passes when enabled", func(t *testing.T) {
		assert.NoError(t, (&TokenRevocationConfig{Enabled: boolPtr(true), SyncIntervalSeconds: 30}).Validate())
	})

	suite.T().Run("unsupported source fails when enabled", func(t *testing.T) {
		assert.ErrorContains(t,
			(&TokenRevocationConfig{Enabled: boolPtr(true), Source: "events", SyncIntervalSeconds: 30}).Validate(),
			"source")
	})
}

// ----- DPoPConfig -----

func (suite *ValidateTestSuite) TestDPoPConfig_IsConfigured() {
	assert.False(suite.T(), (&DPoPConfig{}).IsConfigured())
	assert.True(suite.T(), (&DPoPConfig{Required: true}).IsConfigured())
	assert.True(suite.T(), (&DPoPConfig{IatWindow: 60}).IsConfigured())
	assert.True(suite.T(), (&DPoPConfig{AllowedAlgs: []string{"ES256"}}).IsConfigured())
}

func (suite *ValidateTestSuite) TestDPoPConfig_Validate() {
	suite.T().Run("unconfigured passes", func(t *testing.T) {
		assert.NoError(t, (&DPoPConfig{}).Validate())
	})

	suite.T().Run("zero IatWindow fails", func(t *testing.T) {
		c := &DPoPConfig{Required: true, MaxJTILength: 128, AllowedAlgs: []string{"ES256"}}
		assert.ErrorContains(t, c.Validate(), "iat_window")
	})

	suite.T().Run("negative Leeway fails", func(t *testing.T) {
		c := &DPoPConfig{IatWindow: 60, Leeway: -1, MaxJTILength: 128, AllowedAlgs: []string{"ES256"}}
		assert.ErrorContains(t, c.Validate(), "leeway")
	})

	suite.T().Run("zero MaxJTILength fails", func(t *testing.T) {
		c := &DPoPConfig{IatWindow: 60, Leeway: 5, AllowedAlgs: []string{"ES256"}}
		assert.ErrorContains(t, c.Validate(), "max_jti_length")
	})

	suite.T().Run("empty AllowedAlgs fails", func(t *testing.T) {
		c := &DPoPConfig{IatWindow: 60, Leeway: 5, MaxJTILength: 128}
		assert.ErrorContains(t, c.Validate(), "allowed_algs")
	})

	suite.T().Run("unsupported algorithm fails", func(t *testing.T) {
		c := &DPoPConfig{IatWindow: 60, Leeway: 5, MaxJTILength: 128, AllowedAlgs: []string{"HS256"}}
		assert.ErrorContains(t, c.Validate(), "HS256")
	})

	suite.T().Run("valid config passes", func(t *testing.T) {
		c := &DPoPConfig{IatWindow: 60, Leeway: 5, MaxJTILength: 128, AllowedAlgs: []string{"ES256", "RS256"}}
		assert.NoError(t, c.Validate())
	})

	suite.T().Run("all supported algorithms pass", func(t *testing.T) {
		algs := []string{"ES256", "ES384", "ES512", "PS256", "PS384", "PS512", "RS256", "RS384", "RS512", "EdDSA"}
		c := &DPoPConfig{IatWindow: 60, MaxJTILength: 128, AllowedAlgs: algs}
		assert.NoError(t, c.Validate())
	})
}

// ----- AuthClassConfig -----

func (suite *ValidateTestSuite) TestAuthClassConfig_Validate() {
	suite.T().Run("empty config passes", func(t *testing.T) {
		assert.NoError(t, (&AuthClassConfig{}).Validate())
	})

	suite.T().Run("blank AMR entry fails", func(t *testing.T) {
		c := &AuthClassConfig{Amrs: []string{" "}}
		assert.ErrorContains(t, c.Validate(), "AMR entry must not be empty")
	})

	suite.T().Run("blank ACR key fails", func(t *testing.T) {
		c := &AuthClassConfig{
			Amrs:   []string{"pwd"},
			AcrAMR: map[string][]string{" ": {"pwd"}},
		}
		assert.ErrorContains(t, c.Validate(), "ACR value must not be empty")
	})

	suite.T().Run("empty AMR list for ACR fails", func(t *testing.T) {
		c := &AuthClassConfig{
			Amrs:   []string{"pwd"},
			AcrAMR: map[string][]string{"urn:acr:low": {}},
		}
		assert.ErrorContains(t, c.Validate(), "empty AMR list")
	})

	suite.T().Run("blank AMR key in ACR mapping fails", func(t *testing.T) {
		c := &AuthClassConfig{
			Amrs:   []string{"pwd"},
			AcrAMR: map[string][]string{"urn:acr:low": {" "}},
		}
		assert.ErrorContains(t, c.Validate(), "empty AMR key")
	})

	suite.T().Run("unknown AMR key in ACR mapping fails", func(t *testing.T) {
		c := &AuthClassConfig{
			Amrs:   []string{"pwd"},
			AcrAMR: map[string][]string{"urn:acr:low": {"otp"}},
		}
		assert.ErrorContains(t, c.Validate(), "unknown AMR key")
	})

	suite.T().Run("valid config passes", func(t *testing.T) {
		c := &AuthClassConfig{
			Amrs:   []string{"pwd", "otp"},
			AcrAMR: map[string][]string{"urn:acr:low": {"pwd"}, "urn:acr:high": {"pwd", "otp"}},
		}
		assert.NoError(t, c.Validate())
	})
}

// ----- cors.Validate -----

func (suite *ValidateTestSuite) TestCORSConfig_Validate() {
	suite.T().Run("valid literal origins pass", func(t *testing.T) {
		var origins cors.OriginEntries
		suite.Require().NoError(yaml.Unmarshal([]byte(`
- https://example.com
- https://other.example.com
`), &origins))
		assert.NoError(t, cors.Validate(origins))
	})

	suite.T().Run("wildcard origin fails", func(t *testing.T) {
		var origins cors.OriginEntries
		suite.Require().NoError(yaml.Unmarshal([]byte(`- "*"`), &origins))
		assert.Error(t, cors.Validate(origins))
	})

	suite.T().Run("invalid regex fails", func(t *testing.T) {
		var origins cors.OriginEntries
		suite.Require().NoError(yaml.Unmarshal([]byte(`- regex: '['`), &origins))
		assert.Error(t, cors.Validate(origins))
	})
}

// ----- OAuthConfig.SendServerErrorsToClientEnabled -----

// TestOAuthConfig_SendServerErrorsToClientEnabled tests the behavior of the SendServerErrorsToClientEnabled method.
func (suite *ValidateTestSuite) TestOAuthConfig_SendServerErrorsToClientEnabled() {
	enabled, disabled := true, false
	tests := []struct {
		name     string
		value    *bool
		expected bool
	}{
		{"UnsetDefaultsToSuppressing", nil, false},
		{"ExplicitTrue", &enabled, true},
		{"ExplicitFalse", &disabled, false},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			c := OAuthConfig{SendServerErrorsToClient: tt.value}
			assert.Equal(t, tt.expected, c.SendServerErrorsToClientEnabled())
		})
	}
}

func TestServerConfig_Validate_DeploymentIDSource(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		claim   string
		wantErr bool
	}{
		{"empty defaults to server", "", "", false},
		{"server mode needs no claim", DeploymentIDSourceServer, "", false},
		{"token mode with claim is valid", DeploymentIDSourceToken, "deploymentId", false},
		{"token mode without claim is rejected", DeploymentIDSourceToken, "", true},
		{"unknown source is rejected", "tenant", "deploymentId", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ServerConfig{DeploymentIDSource: tt.source, DeploymentIDClaim: tt.claim}
			err := c.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for source=%q claim=%q, got nil", tt.source, tt.claim)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for source=%q claim=%q: %v", tt.source, tt.claim, err)
			}
		})
	}
}
