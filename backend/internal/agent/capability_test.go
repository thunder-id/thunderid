// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type CapabilityTestSuite struct {
	suite.Suite
}

func TestCapabilityTestSuite(t *testing.T) {
	suite.Run(t, new(CapabilityTestSuite))
}

func credentialPtr(c agentCredential) *agentCredential {
	return &c
}

func publicKey(kid string) map[string]any {
	return map[string]any{"kty": "EC", "crv": "P-256", "x": "x-" + kid, "y": "y-" + kid, "kid": kid}
}

func (s *CapabilityTestSuite) TestDescribeCapabilities() {
	cases := []struct {
		name  string
		cfg   *providers.OAuthConfigWithSecret
		check func(agentCapabilities)
	}{
		{
			name: "no OAuth client",
			check: func(c agentCapabilities) {
				s.Equal(credentialNone, c.credential)
				s.Empty(c.actsForUsers)
				s.NotNil(c.actsForUsers)
			},
		},
		{
			name: "secret client acting in every way, with a grant the terms don't describe",
			cfg: &providers.OAuthConfigWithSecret{
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
				GrantTypes: []providers.GrantType{providers.GrantTypeTokenExchange,
					providers.GrantTypeClientCredentials, providers.GrantTypeCIBA, providers.GrantTypeAuthorizationCode,
					providers.GrantTypeRefreshToken, providers.GrantTypeJWTBearer},
				RedirectURIs: []string{"https://agent.example.com/callback"},
			},
			check: func(c agentCapabilities) {
				s.Equal(credentialClientSecret, c.credential)
				s.Equal([]actingMode{actingSignIn, actingApproval, actingTokenExchange}, c.actsForUsers)
				s.Equal([]string{string(providers.GrantTypeJWTBearer)}, c.otherGrantTypes)
				s.Equal([]string{"https://agent.example.com/callback"}, c.redirectURIs)
			},
		},
		{
			name: "registered key set",
			cfg: &providers.OAuthConfigWithSecret{
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT,
				GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
				Certificate: &providers.Certificate{Type: providers.CertificateTypeJWKS,
					Value: `{"keys":[{"kid":"k1"},{"kid":"k2"}]}`},
			},
			check: func(c agentCapabilities) {
				s.Equal(credentialPrivateKeyJWT, c.credential)
				s.Equal([]string{"k1", "k2"}, c.keyIDs)
				s.Empty(c.jwksURI)
			},
		},
		{
			name: "key set URI",
			cfg: &providers.OAuthConfigWithSecret{
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT,
				Certificate: &providers.Certificate{Type: providers.CertificateTypeJWKSURI,
					Value: "https://agent.example.com/jwks"},
			},
			check: func(c agentCapabilities) {
				s.Equal("https://agent.example.com/jwks", c.jwksURI)
			},
		},
		{
			name: "public client",
			cfg: &providers.OAuthConfigWithSecret{
				PublicClient: true, TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodNone,
				GrantTypes: []providers.GrantType{providers.GrantTypeAuthorizationCode},
			},
			check: func(c agentCapabilities) {
				s.Equal(credentialNone, c.credential)
				s.Equal([]actingMode{actingSignIn}, c.actsForUsers)
			},
		},
		{
			name: "secret-based encryption certificate is not a credential",
			cfg: &providers.OAuthConfigWithSecret{
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				Certificate: &providers.Certificate{Type: providers.CertificateTypeJWKS,
					Value: `{"keys":[{"kid":"enc"}]}`},
			},
			check: func(c agentCapabilities) {
				s.Equal(credentialClientSecret, c.credential)
				s.Empty(c.keyIDs)
			},
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			tc.check(describeCapabilities(tc.cfg))
		})
	}
}

func (s *CapabilityTestSuite) TestApplyCapabilityChange_NewAgent() {
	s.Run("acts on its own with a secret", func() {
		cfg, problems := applyCapabilityChange(nil, capabilityChange{credential: credentialPtr(credentialClientSecret)})
		s.Empty(problems)
		s.Require().NotNil(cfg)
		s.Equal([]providers.GrantType{providers.GrantTypeClientCredentials}, cfg.GrantTypes)
		s.Equal(providers.TokenEndpointAuthMethodClientSecretBasic, cfg.TokenEndpointAuthMethod)
		s.False(cfg.PKCERequired)
		s.Empty(cfg.ResponseTypes)
	})

	s.Run("identity only", func() {
		cfg, problems := applyCapabilityChange(nil, capabilityChange{credential: credentialPtr(credentialNone)})
		s.Empty(problems)
		s.Nil(cfg)
	})

	s.Run("signs users in and asks for approval, with its own key", func() {
		cfg, problems := applyCapabilityChange(nil, capabilityChange{
			credential:   credentialPtr(credentialPrivateKeyJWT),
			jwksURI:      "https://agent.example.com/jwks",
			actsForUsers: []actingMode{actingApproval, actingSignIn},
			redirectURIs: []string{"https://agent.example.com/callback"},
			authFlowID:   "ciba-flow",
		})
		s.Empty(problems)
		s.Require().NotNil(cfg)
		s.Equal([]providers.GrantType{providers.GrantTypeClientCredentials, providers.GrantTypeAuthorizationCode,
			providers.GrantTypeCIBA, providers.GrantTypeRefreshToken}, cfg.GrantTypes)
		s.Equal([]providers.ResponseType{providers.ResponseTypeCode}, cfg.ResponseTypes)
		s.True(cfg.PKCERequired)
		s.Equal(providers.TokenEndpointAuthMethodPrivateKeyJWT, cfg.TokenEndpointAuthMethod)
		s.Equal(&providers.Certificate{Type: providers.CertificateTypeJWKSURI,
			Value: "https://agent.example.com/jwks"}, cfg.Certificate)
	})

	s.Run("public client that signs users in", func() {
		cfg, problems := applyCapabilityChange(nil, capabilityChange{
			credential:   credentialPtr(credentialNone),
			actsForUsers: []actingMode{actingSignIn},
			redirectURIs: []string{"https://app.example.com/callback"},
		})
		s.Empty(problems)
		s.Require().NotNil(cfg)
		s.True(cfg.PublicClient)
		s.True(cfg.PKCERequired)
		s.Equal(providers.TokenEndpointAuthMethodNone, cfg.TokenEndpointAuthMethod)
		s.NotContains(cfg.GrantTypes, providers.GrantTypeClientCredentials)
	})

	s.Run("every problem at once", func() {
		cfg, problems := applyCapabilityChange(nil, capabilityChange{
			credential:   credentialPtr(credentialNone),
			jwksURI:      "https://agent.example.com/jwks",
			actsForUsers: []actingMode{actingSignIn, actingApproval},
		})
		s.Nil(cfg)
		joined := strings.Join(problems, "|")
		s.Contains(joined, "need a credential")
		s.Contains(joined, "sign_in needs redirectUris")
		s.Contains(joined, "approval needs authFlowId")
		s.Contains(joined, "apply only to credential private_key_jwt")
	})

	s.Run("redirect URIs without sign-in", func() {
		_, problems := applyCapabilityChange(nil, capabilityChange{
			credential: credentialPtr(credentialClientSecret), redirectURIs: []string{"https://x.example.com"}})
		s.Equal([]string{"redirectUris apply only to sign_in"}, problems)
	})

	s.Run("private_key_jwt needs exactly one key input", func() {
		_, problems := applyCapabilityChange(nil, capabilityChange{credential: credentialPtr(credentialPrivateKeyJWT)})
		s.Contains(strings.Join(problems, "|"), "give jwksUri or jwks")

		_, problems = applyCapabilityChange(nil, capabilityChange{
			credential: credentialPtr(credentialPrivateKeyJWT), jwksURI: "https://agent.example.com/jwks",
			jwks: map[string]any{"keys": []any{publicKey("k1")}}})
		s.Contains(strings.Join(problems, "|"), "not both")
	})
}

func (s *CapabilityTestSuite) TestApplyCapabilityChange_ExistingAgent() {
	signInAgent := func() *providers.OAuthConfigWithSecret {
		return &providers.OAuthConfigWithSecret{
			ClientID:                "client-1",
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
			GrantTypes: []providers.GrantType{providers.GrantTypeClientCredentials,
				providers.GrantTypeAuthorizationCode, providers.GrantTypeRefreshToken, providers.GrantTypeJWTBearer},
			ResponseTypes: []providers.ResponseType{providers.ResponseTypeCode},
			RedirectURIs:  []string{"https://agent.example.com/callback"},
			PKCERequired:  false,
			Scopes:        []string{"openid", "orders:read"},
		}
	}

	s.Run("adding approval keeps every other setting", func() {
		cfg, problems := applyCapabilityChange(signInAgent(), capabilityChange{
			actsForUsers: []actingMode{actingSignIn, actingApproval}, authFlowID: "ciba-flow"})
		s.Empty(problems)
		s.Require().NotNil(cfg)
		s.Equal(providers.TokenEndpointAuthMethodClientSecretPost, cfg.TokenEndpointAuthMethod)
		s.False(cfg.PKCERequired)
		s.Equal([]string{"openid", "orders:read"}, cfg.Scopes)
		s.Equal("client-1", cfg.ClientID)
		s.Equal([]providers.GrantType{providers.GrantTypeClientCredentials, providers.GrantTypeAuthorizationCode,
			providers.GrantTypeCIBA, providers.GrantTypeRefreshToken, providers.GrantTypeJWTBearer}, cfg.GrantTypes)
	})

	s.Run("removing sign-in keeps the refresh token while approval remains", func() {
		withApproval := signInAgent()
		withApproval.GrantTypes = append(withApproval.GrantTypes, providers.GrantTypeCIBA)
		cfg, problems := applyCapabilityChange(withApproval, capabilityChange{
			actsForUsers: []actingMode{actingApproval}})
		s.Empty(problems)
		s.Require().NotNil(cfg)
		s.Contains(cfg.GrantTypes, providers.GrantTypeRefreshToken)
		s.NotContains(cfg.GrantTypes, providers.GrantTypeAuthorizationCode)
		s.Empty(cfg.ResponseTypes)
		s.Empty(cfg.RedirectURIs)
		s.False(cfg.PKCERequired)
	})

	s.Run("removing every way of acting drops the refresh token", func() {
		cfg, problems := applyCapabilityChange(signInAgent(), capabilityChange{actsForUsers: []actingMode{}})
		s.Empty(problems)
		s.Require().NotNil(cfg)
		s.Equal([]providers.GrantType{providers.GrantTypeClientCredentials, providers.GrantTypeJWTBearer},
			cfg.GrantTypes)
	})

	s.Run("an unrelated change doesn't recheck the stored state", func() {
		public := &providers.OAuthConfigWithSecret{
			PublicClient: true, TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodNone,
			GrantTypes: []providers.GrantType{providers.GrantTypeAuthorizationCode,
				providers.GrantTypeTokenExchange},
			RedirectURIs: []string{"https://agent.example.com/callback"},
		}
		cfg, problems := applyCapabilityChange(public, capabilityChange{
			redirectURIs: []string{"https://agent.example.com/new"}})
		s.Empty(problems)
		s.Require().NotNil(cfg)
		s.Equal([]string{"https://agent.example.com/new"}, cfg.RedirectURIs)
	})

	s.Run("credential none can't keep jwt-bearer", func() {
		_, problems := applyCapabilityChange(signInAgent(), capabilityChange{
			credential: credentialPtr(credentialNone)})
		s.Contains(strings.Join(problems, "|"), "jwt-bearer")
	})

	s.Run("token exchange can't go while ID-JAG is on", func() {
		withIDJAG := &providers.OAuthConfigWithSecret{
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			GrantTypes: []providers.GrantType{providers.GrantTypeClientCredentials,
				providers.GrantTypeTokenExchange},
			Token: &providers.OAuthTokenConfig{IDJAG: &providers.IDJAGConfig{Enabled: true}},
		}
		_, problems := applyCapabilityChange(withIDJAG, capabilityChange{actsForUsers: []actingMode{}})
		s.Equal([]string{"token_exchange can't be removed while ID-JAG is on"}, problems)
	})

	s.Run("leaving private_key_jwt drops the key set unless encryption needs it", func() {
		keyed := &providers.OAuthConfigWithSecret{
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT,
			GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
			Certificate:             &providers.Certificate{Type: providers.CertificateTypeJWKS, Value: "{}"},
		}
		toSecret := capabilityChange{credential: credentialPtr(credentialClientSecret)}
		cfg, problems := applyCapabilityChange(keyed, toSecret)
		s.Empty(problems)
		s.Require().NotNil(cfg)
		s.Nil(cfg.Certificate)
		s.Equal(providers.TokenEndpointAuthMethodClientSecretBasic, cfg.TokenEndpointAuthMethod)

		keyed.UserInfo = &providers.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256"}
		cfg, problems = applyCapabilityChange(keyed, toSecret)
		s.Empty(problems)
		s.Require().NotNil(cfg)
		s.NotNil(cfg.Certificate)
	})

	s.Run("staying on private_key_jwt keeps the key set", func() {
		keyed := &providers.OAuthConfigWithSecret{
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT,
			GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
			Certificate: &providers.Certificate{Type: providers.CertificateTypeJWKSURI,
				Value: "https://agent.example.com/jwks"},
		}
		cfg, problems := applyCapabilityChange(keyed, capabilityChange{
			actsForUsers: []actingMode{actingTokenExchange}})
		s.Empty(problems)
		s.Require().NotNil(cfg)
		s.Equal(keyed.Certificate, cfg.Certificate)
	})

	s.Run("approval already on needs no new flow", func() {
		withApproval := &providers.OAuthConfigWithSecret{
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			GrantTypes: []providers.GrantType{providers.GrantTypeClientCredentials,
				providers.GrantTypeCIBA},
		}
		_, problems := applyCapabilityChange(withApproval, capabilityChange{
			actsForUsers: []actingMode{actingApproval, actingTokenExchange}})
		s.Empty(problems)
	})
}

func (s *CapabilityTestSuite) TestKeySetFromJWKS() {
	value, problem := keySetFromJWKS(map[string]any{"keys": []any{publicKey("k1"), publicKey("k2")}})
	s.Empty(problem)
	s.Equal([]string{"k1", "k2"}, keyIDs(value))

	cases := []struct {
		name    string
		jwks    map[string]any
		problem string
	}{
		{"no keys", map[string]any{"keys": []any{}}, "at least one key"},
		{"not a key set", map[string]any{"keys": "nope"}, "at least one key"},
		{"private key", map[string]any{"keys": []any{
			map[string]any{"kty": "EC", "kid": "k1", "d": "secret"}}}, "private member d"},
		{"missing kid", map[string]any{"keys": []any{map[string]any{"kty": "EC"}}}, "needs a kid"},
		{"repeated kid", map[string]any{"keys": []any{publicKey("k1"), publicKey("k1")}}, "already used"},
		{"too long", map[string]any{"keys": []any{
			map[string]any{"kty": "RSA", "kid": "k1", "n": strings.Repeat("a", maxKeySetLength)}}}, "more than"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, problem := keySetFromJWKS(tc.jwks)
			s.Contains(problem, tc.problem)
		})
	}
}

func (s *CapabilityTestSuite) TestAddAndRemoveKey() {
	stored, problem := keySetFromJWKS(map[string]any{"keys": []any{publicKey("old")}})
	s.Require().Empty(problem)

	withNew, problem := addKey(stored, publicKey("new"))
	s.Empty(problem)
	s.Equal([]string{"old", "new"}, keyIDs(withNew))

	_, problem = addKey(withNew, publicKey("old"))
	s.Contains(problem, "already used")
	_, problem = addKey(withNew, map[string]any{"kty": "EC", "kid": "k", "d": "secret"})
	s.Contains(problem, "private member")

	onlyNew, problem := removeKey(withNew, "old")
	s.Empty(problem)
	s.Equal([]string{"new"}, keyIDs(onlyNew))

	_, problem = removeKey(onlyNew, "new")
	s.Contains(problem, "last key")
	_, problem = removeKey(withNew, "missing")
	s.Contains(problem, "no key with kid missing")

	var decoded keySet
	s.Require().NoError(json.Unmarshal([]byte(onlyNew), &decoded))
	s.Len(decoded.Keys, 1)
}
