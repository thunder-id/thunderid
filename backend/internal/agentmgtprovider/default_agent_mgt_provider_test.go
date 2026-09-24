// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agentmgtprovider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/agentmock"
)

const (
	testAgentID    = "agent-id-123"
	testAgentOwner = "owner-id-456"
)

type DefaultAgentMgtProviderTestSuite struct {
	suite.Suite
	mockService *agentmock.AgentServiceInterfaceMock
	provider    AgentMgtProviderService
}

func (suite *DefaultAgentMgtProviderTestSuite) SetupTest() {
	suite.mockService = agentmock.NewAgentServiceInterfaceMock(suite.T())
	suite.provider = newDefaultAgentMgtProvider()
	suite.provider.SetAgentService(suite.mockService)
}

func TestDefaultAgentMgtProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultAgentMgtProviderTestSuite))
}

func newTestAgent() *providers.Agent {
	return &providers.Agent{
		OUID:  "ou-id-abc",
		Type:  "default",
		Name:  "test-agent",
		Owner: testAgentOwner,
	}
}

// withRedirectURIs attaches the one OAuth value a caller may supply, the way the flow executor does.
func withRedirectURIs(agent *providers.Agent, uris ...string) *providers.Agent {
	agent.InboundAuthConfig = []providers.InboundAuthConfigWithSecret{
		{
			Type:        providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{RedirectURIs: uris},
		},
	}
	return agent
}

// captureCreatedAgent stubs the agent service and returns a pointer to the agent it was handed, so a
// test can assert on the configuration the provider derived.
func (suite *DefaultAgentMgtProviderTestSuite) captureCreatedAgent() **providers.Agent {
	captured := new(*providers.Agent)
	suite.mockService.On("CreateAgent", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			*captured = args.Get(1).(*providers.Agent)
		}).
		Return(&model.AgentCompleteResponse{ID: testAgentID}, nil).Once()
	return captured
}

// oauthConfigOf returns the single OAuth config the provider is expected to attach.
func (suite *DefaultAgentMgtProviderTestSuite) oauthConfigOf(a *providers.Agent) *providers.OAuthConfigWithSecret {
	suite.Require().NotNil(a)
	suite.Require().Len(a.InboundAuthConfig, 1)
	suite.Equal(providers.OAuthInboundAuthType, a.InboundAuthConfig[0].Type)
	suite.Require().NotNil(a.InboundAuthConfig[0].OAuthConfig)
	return a.InboundAuthConfig[0].OAuthConfig
}

// An agent acting on its own behalf authenticates with client credentials only. The inbound client
// service rejects response types on a client-credentials-only client, so this shape is the only
// legal one for a non-delegated agent.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentDerivesOwnBehalfOAuthConfig() {
	captured := suite.captureCreatedAgent()

	_, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent(), false)

	suite.Nil(svcErr)
	config := suite.oauthConfigOf(*captured)
	suite.Equal([]providers.GrantType{providers.GrantTypeClientCredentials}, config.GrantTypes)
	suite.Empty(config.ResponseTypes)
	suite.Empty(config.RedirectURIs)
	suite.False(config.PKCERequired)
	suite.False(config.PublicClient)
	suite.Equal(providers.TokenEndpointAuthMethodClientSecretBasic, config.TokenEndpointAuthMethod)
	suite.Empty((*captured).AllowedUserTypes)
}

// An agent with no logo would draw as nothing in a listing, so one is supplied. The console's
// create wizard set the same avatar before this moved to the provider.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentSuppliesALogoWhenNoneIsGiven() {
	captured := suite.captureCreatedAgent()

	_, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent(), false)

	suite.Nil(svcErr)
	suite.Equal(defaultAgentLogoURL, (*captured).LogoURL)
}

// A caller that chose a logo keeps it.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentKeepsACallerSuppliedLogo() {
	captured := suite.captureCreatedAgent()
	agent := newTestAgent()
	agent.LogoURL = "https://example.com/logo.png"

	_, svcErr := suite.provider.CreateAgent(context.Background(), agent, false)

	suite.Nil(svcErr)
	suite.Equal("https://example.com/logo.png", (*captured).LogoURL)
}

// A delegated agent signs a user in, which the inbound client service only accepts with the
// authorization code grant, the code response type and at least one redirect URI.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentDerivesDelegatedOAuthConfig() {
	req := withRedirectURIs(newTestAgent(), "https://app.example.com/callback")
	req.AllowedUserTypes = []string{"customer"}

	captured := suite.captureCreatedAgent()

	_, svcErr := suite.provider.CreateAgent(context.Background(), req, true)

	suite.Nil(svcErr)
	config := suite.oauthConfigOf(*captured)
	suite.Equal([]providers.GrantType{
		providers.GrantTypeClientCredentials,
		providers.GrantTypeAuthorizationCode,
		providers.GrantTypeRefreshToken,
	}, config.GrantTypes)
	suite.Equal([]providers.ResponseType{providers.ResponseTypeCode}, config.ResponseTypes)
	suite.Equal([]string{"https://app.example.com/callback"}, config.RedirectURIs)
	suite.True(config.PKCERequired)
	suite.False(config.PublicClient)
	suite.Equal(providers.TokenEndpointAuthMethodClientSecretBasic, config.TokenEndpointAuthMethod)
	suite.Equal(req.AllowedUserTypes, (*captured).AllowedUserTypes)
}

// Token settings and flow identifiers are left unset so the inbound client service applies the
// organization unit and server defaults. Sending them would pin a new agent to whatever the
// provider happened to hardcode.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentLeavesTokenAndFlowDefaultsToTheService() {
	captured := suite.captureCreatedAgent()

	_, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent(), false)

	suite.Nil(svcErr)
	suite.Nil(suite.oauthConfigOf(*captured).Token)
	suite.Empty((*captured).AuthFlowID)
	suite.Empty((*captured).RegistrationFlowID)
	suite.Empty((*captured).RecoveryFlowID)
	suite.Empty((*captured).SignOutFlowID)
}

// Allowed user types describe who an agent may act for, which is meaningless without delegation.
// Honoring them anyway would give a client-credentials-only agent user-facing configuration.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentIgnoresAllowedUserTypesWhenNotDelegated() {
	req := newTestAgent()
	req.AllowedUserTypes = []string{"customer"}

	captured := suite.captureCreatedAgent()

	_, svcErr := suite.provider.CreateAgent(context.Background(), req, false)

	suite.Nil(svcErr)
	suite.Empty((*captured).AllowedUserTypes)
}

// The entity data the caller collected must reach the agent service untouched; only the
// authentication configuration is the provider's to decide.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentPassesEntityDataThrough() {
	req := &providers.Agent{
		OUID:        "ou-id-abc",
		Type:        "default",
		Name:        "billing-agent",
		Description: "handles invoices",
		LogoURL:     "https://example.com/logo.png",
		Owner:       testAgentOwner,
		Attributes:  []byte(`{"model":"claude"}`),
	}

	captured := suite.captureCreatedAgent()

	_, svcErr := suite.provider.CreateAgent(context.Background(), req, false)

	suite.Nil(svcErr)
	suite.Equal(req.OUID, (*captured).OUID)
	suite.Equal(req.Type, (*captured).Type)
	suite.Equal(req.Name, (*captured).Name)
	suite.Equal(req.Description, (*captured).Description)
	suite.Equal(req.LogoURL, (*captured).LogoURL)
	suite.Equal(req.Owner, (*captured).Owner)
	suite.Equal(req.Attributes, (*captured).Attributes)
}

// A delegated agent that names no redirect URI is forwarded with none. The provider substitutes
// nothing and reports no error; supplying one is the caller's business.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentDelegatedWithoutRedirectURIsGetsNone() {
	captured := suite.captureCreatedAgent()

	_, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent(), true)

	suite.Nil(svcErr)
	suite.Empty(suite.oauthConfigOf(*captured).RedirectURIs)
}

// Likewise the allowed user types: an unnamed list stays empty rather than being filled in.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentDelegatedWithoutUserTypesGetsNone() {
	captured := suite.captureCreatedAgent()

	_, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent(), true)

	suite.Nil(svcErr)
	suite.Empty((*captured).AllowedUserTypes)
}

// A non-delegated agent has no authorization code grant, so it gets no redirect URI at all.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentNotDelegatedGetsNoRedirectURI() {
	captured := suite.captureCreatedAgent()

	_, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent(), false)

	suite.Nil(svcErr)
	suite.Empty(suite.oauthConfigOf(*captured).RedirectURIs)
}

// Redirect URIs are the one OAuth value a caller may supply. Everything else it left on the inbound
// auth config is discarded, because the shape the inbound client service accepts is not the
// caller's to choose.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentKeepsOnlyCallerRedirectURIs() {
	req := newTestAgent()
	req.InboundAuthConfig = []providers.InboundAuthConfigWithSecret{
		{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				RedirectURIs:            []string{"https://app.example.com/cb"},
				GrantTypes:              []providers.GrantType{providers.GrantTypeTokenExchange},
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodNone,
				PKCERequired:            false,
			},
		},
	}

	captured := suite.captureCreatedAgent()

	_, svcErr := suite.provider.CreateAgent(context.Background(), req, true)

	suite.Nil(svcErr)
	config := suite.oauthConfigOf(*captured)
	suite.Equal([]string{"https://app.example.com/cb"}, config.RedirectURIs, "the caller's URIs are kept")
	suite.NotContains(config.GrantTypes, providers.GrantTypeTokenExchange,
		"a grant type the caller asked for is discarded")
	suite.Equal(providers.TokenEndpointAuthMethodClientSecretBasic, config.TokenEndpointAuthMethod)
	suite.True(config.PKCERequired)
}

// The caller's value must not be altered underneath it, so the provider derives onto a copy.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentDoesNotMutateTheCallersAgent() {
	req := newTestAgent()
	suite.captureCreatedAgent()

	_, svcErr := suite.provider.CreateAgent(context.Background(), req, true)

	suite.Nil(svcErr)
	suite.Empty(req.InboundAuthConfig, "the caller's agent still carries no derived configuration")
	suite.Empty(req.AllowedUserTypes)
}

// With neither an explicit owner nor an authenticated subject there is nothing to own the agent.
// Reaching the agent service would silently persist an ownerless agent, since its own fallback
// resolves to the same empty subject and the system attributes simply omit the field.
// The agent service resolves an absent owner to the authenticated subject itself, so an empty owner
// is forwarded rather than rejected here.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentWithoutOwnerIsForwarded() {
	req := newTestAgent()
	req.Owner = ""

	captured := suite.captureCreatedAgent()

	_, svcErr := suite.provider.CreateAgent(context.Background(), req, false)

	suite.Nil(svcErr)
	suite.Empty((*captured).Owner)
}

// A supplied owner reaches the agent service unchanged, which routes it through the service's owner
// existence check.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentPassesOwnerExplicitly() {
	captured := suite.captureCreatedAgent()

	_, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent(), false)

	suite.Nil(svcErr)
	suite.Equal(testAgentOwner, (*captured).Owner)
}

func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentWithNilRequestIsRejectedBeforeDelegation() {
	resp, svcErr := suite.provider.CreateAgent(context.Background(), nil, false)

	suite.Nil(resp)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidRequestFormat.Code, svcErr.Code)
	suite.mockService.AssertNotCalled(suite.T(), "CreateAgent", mock.Anything, mock.Anything)
}

// The agent service is injected after construction, so a wiring mistake must surface as a refused
// operation at the call site rather than a nil dereference.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentBeforeAgentServiceIsInjected() {
	provider := newDefaultAgentMgtProvider()

	resp, svcErr := provider.CreateAgent(context.Background(), newTestAgent(), false)

	suite.Nil(resp)
	suite.NotNil(svcErr)
	suite.Equal(ErrorAgentProvisioningDisabled.Code, svcErr.Code)
}

// Failures raised by the agent service must reach the caller with their original code. A runtime
// caller distinguishes a name clash it can retry with different input from a configuration problem
// it cannot, so collapsing these into one generic error would remove the only signal it has.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentPreservesServiceErrors() {
	tests := []struct {
		name     string
		scenario string
		svcErr   *tidcommon.ServiceError
	}{
		{
			name:     "DuplicateName",
			scenario: "another agent already uses the requested name",
			svcErr:   &agent.ErrorAgentAlreadyExistsWithName,
		},
		{
			name:     "OwnerNotFound",
			scenario: "the supplied owner does not resolve to a known entity",
			svcErr:   &agent.ErrorOwnerNotFound,
		},
		{
			name:     "OrganizationUnitNotFound",
			scenario: "the target organization unit does not exist",
			svcErr:   &agent.ErrorOrganizationUnitNotFound,
		},
		{
			name:     "ClientIDTaken",
			scenario: "the requested client ID is already in use",
			svcErr:   &agent.ErrorAgentAlreadyExistsWithClientID,
		},
		{
			name:     "SchemaValidationFailed",
			scenario: "the attributes do not satisfy the agent type schema",
			svcErr:   &agent.ErrorSchemaValidationFailed,
		},
		{
			name:     "InvalidAgentType",
			scenario: "the agent type is missing",
			svcErr:   &agent.ErrorInvalidAgentType,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			mockService := agentmock.NewAgentServiceInterfaceMock(suite.T())
			provider := newDefaultAgentMgtProvider()
			provider.SetAgentService(mockService)
			mockService.On("CreateAgent", mock.Anything, mock.Anything).
				Return(nil, tt.svcErr).Once()

			resp, svcErr := provider.CreateAgent(context.Background(), newTestAgent(), false)

			suite.Nil(resp)
			suite.NotNil(svcErr, tt.scenario)
			suite.Equal(tt.svcErr.Code, svcErr.Code, tt.scenario)
			suite.Equal(tidcommon.ClientErrorType, svcErr.Type, tt.scenario)
		})
	}
}

// A system failure must stay a server error. Reporting an outage as a client error would tell the
// caller to change its input when retrying is the correct response.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentPreservesServerErrors() {
	suite.mockService.On("CreateAgent", mock.Anything, mock.Anything).
		Return(nil, &tidcommon.InternalServerError).Once()

	resp, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent(), false)

	suite.Nil(resp)
	suite.NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	suite.Equal(tidcommon.ServerErrorType, svcErr.Type)
}

// The agent service must see a runtime-marked context that still carries the caller's values, so
// authorization is elevated without losing request-scoped data such as the trace ID.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentElevatesCallerContext() {
	type ctxKey string
	const traceKey ctxKey = "trace-id"

	callerCtx := context.WithValue(context.Background(), traceKey, "trace-789")

	var observed context.Context
	suite.mockService.On("CreateAgent", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			observed = args.Get(0).(context.Context)
		}).
		Return(&model.AgentCompleteResponse{ID: testAgentID}, nil).Once()

	_, svcErr := suite.provider.CreateAgent(callerCtx, newTestAgent(), false)

	suite.Nil(svcErr)
	suite.True(security.IsRuntimeContext(observed))
	suite.Equal("trace-789", observed.Value(traceKey))
}

// The generated client secret is the reason a runtime capability provisions an agent at all. It
// crosses toProviderAgent, so this guards against the mapper dropping it.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentReturnsGeneratedClientCredentials() {
	suite.mockService.On("CreateAgent", mock.Anything, mock.Anything).
		Return(&model.AgentCompleteResponse{
			ID: testAgentID,
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:     "generated-client-id",
					ClientSecret: "generated-client-secret",
				},
			}},
		}, nil).Once()

	resp, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent(), false)

	suite.Nil(svcErr)
	suite.Require().Len(resp.InboundAuthConfig, 1)
	suite.Require().NotNil(resp.InboundAuthConfig[0].OAuthConfig)
	suite.Equal("generated-client-id", resp.InboundAuthConfig[0].OAuthConfig.ClientID)
	suite.Equal("generated-client-secret", resp.InboundAuthConfig[0].OAuthConfig.ClientSecret)
}

// A service that returns neither a response nor an error must not take the runtime down with a nil
// dereference inside the mapper.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentHandlesEmptyServiceResponse() {
	suite.mockService.On("CreateAgent", mock.Anything, mock.Anything).
		Return(nil, nil).Once()

	resp, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent(), false)

	suite.Nil(svcErr)
	suite.Nil(resp)
}

// toProviderAgent is a hand-maintained field copy, so a field the agent service starts returning
// later can be silently zeroed for every caller. This populates every field the response can carry
// and asserts each one survives the mapping.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentMapsEveryResponseField() {
	assertion := &providers.AssertionConfig{}
	loginConsent := &providers.LoginConsentConfig{ValidityPeriod: 3600}
	attestation := &providers.AttestationConfig{}

	full := &model.AgentCompleteResponse{
		ID:          testAgentID,
		OUID:        "ou-id-abc",
		OUHandle:    "engineering",
		Type:        "default",
		Name:        "test-agent",
		Description: "an agent",
		LogoURL:     "avatar:shape=circle",
		Owner:       testAgentOwner,
		Attributes:  []byte(`{"k":"v"}`),
	}
	full.AuthFlowID = "auth-flow"
	full.AuthFlowHandle = "auth-handle"
	full.RegistrationFlowID = "reg-flow"
	full.RegistrationFlowHandle = "reg-handle"
	full.IsRegistrationFlowEnabled = true
	full.RecoveryFlowID = "rec-flow"
	full.RecoveryFlowHandle = "rec-handle"
	full.IsRecoveryFlowEnabled = true
	full.SignOutFlowID = "out-flow"
	full.SignOutFlowHandle = "out-handle"
	full.ThemeID = "theme-1"
	full.LayoutID = "layout-1"
	full.Assertion = assertion
	full.LoginConsent = loginConsent
	full.AllowedUserTypes = []string{"customer"}
	full.SubjectAttribute = map[string]string{"customer": "email"}
	full.PasskeyAllowedOrigins = []string{"https://example.com"}
	full.Attestation = attestation

	suite.mockService.On("CreateAgent", mock.Anything, mock.Anything).Return(full, nil).Once()

	resp, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent(), false)

	suite.Nil(svcErr)
	suite.Equal(full.ID, resp.ID)
	suite.Equal(full.OUID, resp.OUID)
	suite.Equal(full.OUHandle, resp.OUHandle)
	suite.Equal(full.Type, resp.Type)
	suite.Equal(full.Name, resp.Name)
	suite.Equal(full.Description, resp.Description)
	suite.Equal(full.LogoURL, resp.LogoURL)
	suite.Equal(full.Owner, resp.Owner)
	suite.Equal(full.Attributes, resp.Attributes)
	suite.Equal(full.AuthFlowID, resp.AuthFlowID)
	suite.Equal(full.AuthFlowHandle, resp.AuthFlowHandle)
	suite.Equal(full.RegistrationFlowID, resp.RegistrationFlowID)
	suite.Equal(full.RegistrationFlowHandle, resp.RegistrationFlowHandle)
	suite.Equal(full.IsRegistrationFlowEnabled, resp.IsRegistrationFlowEnabled)
	suite.Equal(full.RecoveryFlowID, resp.RecoveryFlowID)
	suite.Equal(full.RecoveryFlowHandle, resp.RecoveryFlowHandle)
	suite.Equal(full.IsRecoveryFlowEnabled, resp.IsRecoveryFlowEnabled)
	suite.Equal(full.SignOutFlowID, resp.SignOutFlowID)
	suite.Equal(full.SignOutFlowHandle, resp.SignOutFlowHandle)
	suite.Equal(full.ThemeID, resp.ThemeID)
	suite.Equal(full.LayoutID, resp.LayoutID)
	suite.Equal(assertion, resp.Assertion)
	suite.Equal(loginConsent, resp.LoginConsent)
	suite.Equal(full.AllowedUserTypes, resp.AllowedUserTypes)
	suite.Equal(full.SubjectAttribute, resp.SubjectAttribute)
	suite.Equal(full.PasskeyAllowedOrigins, resp.PasskeyAllowedOrigins)
	suite.Equal(attestation, resp.Attestation)
}
