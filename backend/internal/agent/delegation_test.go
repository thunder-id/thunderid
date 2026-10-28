// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type DelegationTestSuite struct {
	suite.Suite
}

func TestDelegationTestSuite(t *testing.T) {
	suite.Run(t, new(DelegationTestSuite))
}

const (
	clientCredentials = providers.GrantTypeClientCredentials
	authCode          = providers.GrantTypeAuthorizationCode
	refreshToken      = providers.GrantTypeRefreshToken
	ciba              = providers.GrantTypeCIBA
	tokenExchange     = providers.GrantTypeTokenExchange
	jwtBearer         = providers.GrantTypeJWTBearer
)

func agentWithGrants(delegated bool, grantTypes ...providers.GrantType) *providers.Agent {
	return &providers.Agent{
		Delegated: delegated,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					GrantTypes:   grantTypes,
					RedirectURIs: []string{"https://app.example.com/callback"},
				},
			},
		},
	}
}

func (suite *DelegationTestSuite) oauthConfig(agent *providers.Agent) *providers.OAuthConfigWithSecret {
	suite.Require().Len(agent.InboundAuthConfig, 1)
	suite.Require().NotNil(agent.InboundAuthConfig[0].OAuthConfig)
	return agent.InboundAuthConfig[0].OAuthConfig
}

func (suite *DelegationTestSuite) TestAgentActingOnItsOwnBehalf() {
	tests := []struct {
		name     string
		requests []providers.GrantType
		expected []providers.GrantType
	}{
		{"NoGrantsGetsClientCredentials", nil, []providers.GrantType{clientCredentials}},
		{"ClientCredentialsIsLeftAlone", []providers.GrantType{clientCredentials},
			[]providers.GrantType{clientCredentials}},
		{"ClientCredentialsIsAddedFirst", []providers.GrantType{authCode},
			[]providers.GrantType{clientCredentials, authCode}},
		{"ClientCredentialsIsNotDuplicated", []providers.GrantType{refreshToken, clientCredentials},
			[]providers.GrantType{refreshToken, clientCredentials}},
		{"EveryRequestedGrantIsAccepted", []providers.GrantType{refreshToken, jwtBearer},
			[]providers.GrantType{clientCredentials, refreshToken, jwtBearer}},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			agent := agentWithGrants(false, tt.requests...)

			suite.Nil(applyGrantTypes(agent))

			config := suite.oauthConfig(agent)
			suite.Equal(tt.expected, config.GrantTypes)
			suite.Empty(config.ResponseTypes)
			suite.False(config.PKCERequired)
		})
	}
}

func (suite *DelegationTestSuite) TestAgentWithoutAnOAuthConfigGetsNoneUnlessDelegated() {
	agent := &providers.Agent{}

	suite.Nil(applyGrantTypes(agent))

	suite.Empty(agent.InboundAuthConfig)
}

func (suite *DelegationTestSuite) TestDelegatedAgentGetsTheDefaultGrants() {
	tests := []struct {
		name     string
		requests []providers.GrantType
	}{
		{"NoGrants", nil},
		{"ClientCredentialsOnly", []providers.GrantType{clientCredentials}},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			agent := agentWithGrants(true, tt.requests...)

			suite.Nil(applyGrantTypes(agent))

			config := suite.oauthConfig(agent)
			suite.Equal([]providers.GrantType{clientCredentials, authCode, refreshToken}, config.GrantTypes)
			suite.Equal([]providers.ResponseType{providers.ResponseTypeCode}, config.ResponseTypes)
			suite.True(config.PKCERequired)
			suite.Equal([]string{"https://app.example.com/callback"}, config.RedirectURIs)
		})
	}
}

func (suite *DelegationTestSuite) TestDelegatedAgentWithoutAnOAuthConfigGetsOne() {
	agent := &providers.Agent{Delegated: true}

	suite.Nil(applyGrantTypes(agent))

	config := suite.oauthConfig(agent)
	suite.Equal(providers.OAuthInboundAuthType, agent.InboundAuthConfig[0].Type)
	suite.Equal([]providers.GrantType{clientCredentials, authCode, refreshToken}, config.GrantTypes)
}

func (suite *DelegationTestSuite) TestDelegatedAgentKeepsGrantsThatLetItActForAUser() {
	tests := []struct {
		name     string
		requests []providers.GrantType
		expected []providers.GrantType
	}{
		{"AuthorizationCode", []providers.GrantType{authCode}, []providers.GrantType{clientCredentials, authCode}},
		{"CIBA", []providers.GrantType{ciba}, []providers.GrantType{clientCredentials, ciba}},
		{"TokenExchange", []providers.GrantType{tokenExchange},
			[]providers.GrantType{clientCredentials, tokenExchange}},
		{"RefreshTokenAlongsideOne", []providers.GrantType{refreshToken, authCode},
			[]providers.GrantType{clientCredentials, refreshToken, authCode}},
		{"ClientCredentialsAlongsideOne", []providers.GrantType{clientCredentials, ciba},
			[]providers.GrantType{clientCredentials, ciba}},
		{"OtherGrantsAreNotChecked", []providers.GrantType{authCode, jwtBearer},
			[]providers.GrantType{clientCredentials, authCode, jwtBearer}},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			agent := agentWithGrants(true, tt.requests...)

			suite.Nil(applyGrantTypes(agent))

			config := suite.oauthConfig(agent)
			suite.Equal(tt.expected, config.GrantTypes)
			// The defaults were not applied, so the caller's choices about the flow stand.
			suite.Empty(config.ResponseTypes)
			suite.False(config.PKCERequired)
		})
	}
}

func (suite *DelegationTestSuite) TestDelegatedAgentNeedsAGrantThatLetsItActForAUser() {
	tests := []struct {
		name     string
		requests []providers.GrantType
	}{
		{"RefreshTokenAlone", []providers.GrantType{refreshToken}},
		{"RefreshTokenWithClientCredentials", []providers.GrantType{clientCredentials, refreshToken}},
		{"AnUnrelatedGrant", []providers.GrantType{jwtBearer}},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			agent := agentWithGrants(true, tt.requests...)

			svcErr := applyGrantTypes(agent)

			suite.Require().NotNil(svcErr)
			suite.Equal(ErrorMissingDelegationGrantType.Code, svcErr.Code)
			suite.Equal("authorization_code, urn:openid:params:grant-type:ciba, "+
				"urn:ietf:params:oauth:grant-type:token-exchange", svcErr.ErrorDescription.Params["grantTypes"])
		})
	}
}

func (suite *DelegationTestSuite) TestPublicClientIsNotGivenClientCredentials() {
	tests := []struct {
		name      string
		delegated bool
		requests  []providers.GrantType
		expected  []providers.GrantType
	}{
		{"AuthorizationCode", false, []providers.GrantType{authCode}, []providers.GrantType{authCode}},
		{"DelegatedAuthorizationCode", true, []providers.GrantType{authCode}, []providers.GrantType{authCode}},
		{"DelegatedDefaults", true, nil, []providers.GrantType{authCode, refreshToken}},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			agent := agentWithGrants(tt.delegated, tt.requests...)
			agent.InboundAuthConfig[0].OAuthConfig.PublicClient = true
			agent.InboundAuthConfig[0].OAuthConfig.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodNone

			suite.Nil(applyGrantTypes(agent))

			suite.Equal(tt.expected, suite.oauthConfig(agent).GrantTypes)
		})
	}
}

func (suite *DelegationTestSuite) TestAnExplicitClientCredentialsGrantIsKeptForAPublicClient() {
	agent := agentWithGrants(false, clientCredentials)
	agent.InboundAuthConfig[0].OAuthConfig.PublicClient = true

	suite.Nil(applyGrantTypes(agent))

	suite.Equal([]providers.GrantType{clientCredentials}, suite.oauthConfig(agent).GrantTypes,
		"the inbound client service reports the combination the caller asked for")
}

func (suite *DelegationTestSuite) TestTheCallersAgentConfigIsNotMutated() {
	agent := agentWithGrants(true)
	original := agent.InboundAuthConfig[0].OAuthConfig

	suite.Nil(applyGrantTypes(agent))

	suite.Empty(original.GrantTypes, "the caller's config keeps what it was given")
	suite.False(original.PKCERequired)
	suite.NotSame(original, agent.InboundAuthConfig[0].OAuthConfig)
}

func (suite *DelegationTestSuite) TestOtherInboundConfigsAreKept() {
	agent := agentWithGrants(true)
	other := providers.InboundAuthConfigWithSecret{Type: "saml"}
	agent.InboundAuthConfig = append([]providers.InboundAuthConfigWithSecret{other}, agent.InboundAuthConfig...)

	suite.Nil(applyGrantTypes(agent))

	suite.Require().Len(agent.InboundAuthConfig, 2)
	suite.Equal(other, agent.InboundAuthConfig[0])
	suite.Equal([]providers.GrantType{clientCredentials, authCode, refreshToken},
		agent.InboundAuthConfig[1].OAuthConfig.GrantTypes)
}

func (suite *DelegationTestSuite) TestPersonalAgentFollowsThePersonalRules() {
	tests := []struct {
		name      string
		delegated bool
		requests  []providers.GrantType
		expected  []providers.GrantType
		codeFlow  bool
	}{
		{"NoGrantsGetsAuthorizationCode", false, nil, []providers.GrantType{authCode}, true},
		{"DelegatedIsIgnored", true, nil, []providers.GrantType{authCode}, true},
		{"ClientCredentialsIsNotAdded", false, []providers.GrantType{authCode, refreshToken},
			[]providers.GrantType{authCode, refreshToken}, true},
		{"CIBAAloneNeedsNoCodeFlow", false, []providers.GrantType{ciba}, []providers.GrantType{ciba}, false},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			agent := agentWithGrants(tt.delegated, tt.requests...)
			agent.Kind = providers.AgentKindPersonalAssistant

			suite.Nil(applyGrantTypes(agent))

			config := suite.oauthConfig(agent)
			suite.Equal(tt.expected, config.GrantTypes)
			suite.Equal(tt.codeFlow, config.PKCERequired)
			if tt.codeFlow {
				suite.Equal([]providers.ResponseType{providers.ResponseTypeCode}, config.ResponseTypes)
			} else {
				suite.Empty(config.ResponseTypes)
			}
		})
	}
}

func (suite *DelegationTestSuite) TestPersonalAgentRefusesAGrantOutsideThePersonalSet() {
	for _, grantType := range []providers.GrantType{clientCredentials, tokenExchange, jwtBearer} {
		suite.Run(string(grantType), func() {
			agent := agentWithGrants(false, authCode, grantType)
			agent.Kind = providers.AgentKindPersonalAssistant

			svcErr := applyGrantTypes(agent)

			suite.Require().NotNil(svcErr)
			suite.Equal(ErrorGrantTypeNotAllowedForPersonalAgent.Code, svcErr.Code)
		})
	}
}

func (suite *DelegationTestSuite) TestPersonalAgentWithoutAnOAuthConfigGetsOne() {
	agent := &providers.Agent{Kind: providers.AgentKindPersonalAssistant}

	suite.Nil(applyGrantTypes(agent))

	suite.Equal([]providers.GrantType{authCode}, suite.oauthConfig(agent).GrantTypes)
}
