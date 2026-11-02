// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/agent/model"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/agentmock"
)

const toolsAgentID = "agent-1"

type AgentToolsTestSuite struct {
	suite.Suite
	mockService *agentmock.AgentServiceInterfaceMock
	session     *mcp.ClientSession
}

func TestAgentToolsTestSuite(t *testing.T) {
	suite.Run(t, new(AgentToolsTestSuite))
}

// SetupTest registers the tools over a mocked agent service and connects an in-memory MCP client, so
// every call crosses the SDK's schema validation and output encoding as an agent's would.
func (s *AgentToolsTestSuite) SetupTest() {
	s.mockService = agentmock.NewAgentServiceInterfaceMock(s.T())

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	registerMCPTools(server, s.mockService)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	s.session, err = client.Connect(ctx, clientTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = s.session.Close() })
}

func (s *AgentToolsTestSuite) call(name string, args map[string]any) *mcp.CallToolResult {
	result, err := s.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	s.Require().NoError(err)
	return result
}

func (s *AgentToolsTestSuite) errorText(result *mcp.CallToolResult) string {
	s.Require().True(result.IsError)
	s.Require().NotEmpty(result.Content)
	text, ok := result.Content[0].(*mcp.TextContent)
	s.Require().True(ok)
	return text.Text
}

func (s *AgentToolsTestSuite) structured(result *mcp.CallToolResult, out any) {
	s.Require().False(result.IsError, "tool returned an error: %v", result.Content)
	raw, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, out))
}

// storedAgent is an agent as the service reads it: no client secret, and the settings a task-scoped
// change must keep.
func storedAgent(grantTypes ...providers.GrantType) *model.AgentGetResponse {
	agent := &model.AgentGetResponse{
		ID:          toolsAgentID,
		OUID:        "ou-1",
		OUHandle:    "engineering",
		Type:        "assistant",
		Display:     "Helper",
		Description: "Answers support tickets",
		LogoURL:     "https://example.com/logo.png",
		Owner:       "user-1",
		Attributes:  json.RawMessage(`{"name":"Helper"}`),
		Status:      &providers.AccessStatusReport{Value: "ACTIVE"},
		InboundAuthProfileReq: inboundmodel.InboundAuthProfileReq{
			AuthFlowID:       "flow-1",
			AllowedUserTypes: []string{"customer"},
		},
	}
	if len(grantTypes) > 0 {
		config := &providers.OAuthConfigWithSecret{
			ClientID:                "client-1",
			GrantTypes:              grantTypes,
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
		}
		if isDelegatedGrantTypes(grantTypes) {
			config.RedirectURIs = []string{"https://agent.example.com/callback"}
			config.ResponseTypes = []providers.ResponseType{providers.ResponseTypeCode}
			config.PKCERequired = true
		}
		agent.InboundAuthConfig = []providers.InboundAuthConfigWithSecret{{
			Type: providers.OAuthInboundAuthType, OAuthConfig: config,
		}}
	}
	return agent
}

// completeResponse echoes an update request as the service would, with the given secret on the OAuth
// entry.
func completeResponse(req *model.UpdateAgentRequest, secret string) *model.AgentCompleteResponse {
	return &model.AgentCompleteResponse{
		ID:                    toolsAgentID,
		OUID:                  req.OUID,
		Type:                  req.Type,
		Description:           req.Description,
		Owner:                 req.Owner,
		InboundAuthProfileReq: req.InboundAuthProfileReq,
		InboundAuthConfig:     annotateOAuthConfig(req.InboundAuthConfig, "client-1", secret),
	}
}

// captureUpdate makes the service hold the stored agent and records the update the tool sends.
func (s *AgentToolsTestSuite) captureUpdate(stored *model.AgentGetResponse, secret string) *model.UpdateAgentRequest {
	s.mockService.On("GetAgent", mock.Anything, toolsAgentID, false).
		Return(stored, (*tidcommon.ServiceError)(nil)).Once()
	received := &model.UpdateAgentRequest{}
	s.mockService.On("UpdateAgent", mock.Anything, toolsAgentID, mock.Anything).
		Return(func(_ context.Context, _ string, req *model.UpdateAgentRequest) (
			*model.AgentCompleteResponse, *tidcommon.ServiceError) {
			*received = *req
			return completeResponse(req, secret), nil
		}).Once()
	return received
}

// assertKept checks that a task-scoped change sends back the settings it does not cover.
func (s *AgentToolsTestSuite) assertKept(req *model.UpdateAgentRequest) {
	s.Equal("ou-1", req.OUID)
	s.Equal("assistant", req.Type)
	s.Equal("Answers support tickets", req.Description)
	s.Equal("https://example.com/logo.png", req.LogoURL)
	s.JSONEq(`{"name":"Helper"}`, string(req.Attributes))
	s.Equal("flow-1", req.AuthFlowID)
	s.Equal([]string{"customer"}, req.AllowedUserTypes)
}

func (s *AgentToolsTestSuite) oauthConfig(req *model.UpdateAgentRequest) *providers.OAuthConfigWithSecret {
	config := agentOAuthConfig(req.InboundAuthConfig)
	s.Require().NotNil(config)
	return config
}

func (s *AgentToolsTestSuite) TestRegisterMCPTools() {
	result, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)

	tools := map[string]*mcp.Tool{}
	for _, registered := range result.Tools {
		tools[registered.Name] = registered
	}
	s.Len(tools, 7)
	for _, name := range []string{"list_agents", "get_agent"} {
		s.Require().Contains(tools, name)
		s.True(tools[name].Annotations.ReadOnlyHint, name)
	}
	for _, name := range []string{"rotate_agent_secret", "offboard_agent"} {
		s.Require().Contains(tools, name)
		s.Require().NotNil(tools[name].Annotations.DestructiveHint, name)
		s.True(*tools[name].Annotations.DestructiveHint, name)
	}
	s.Contains(tools, "onboard_agent")
	s.Contains(tools, "set_agent_mode")
	s.Contains(tools, "transfer_agent_ownership")

	onboardSchema, err := json.Marshal(tools["onboard_agent"].InputSchema)
	s.Require().NoError(err)
	s.Contains(string(onboardSchema), `"enum":["autonomous","delegated","none"]`)
	modeSchema, err := json.Marshal(tools["set_agent_mode"].InputSchema)
	s.Require().NoError(err)
	s.Contains(string(modeSchema), `"enum":["autonomous","delegated"]`)
}

func (s *AgentToolsTestSuite) TestListAgents() {
	s.mockService.On("GetAgentList", mock.Anything, 20, 0, map[string]interface{}{}, true).
		Return(&model.AgentListResponse{
			TotalResults: 1,
			Agents: []model.BasicAgentResponse{{
				ID: toolsAgentID, Display: "Helper", Type: "assistant", OUID: "ou-1", ClientID: "client-1",
				Owner: "user-1", IsReadOnly: true, Status: &providers.AccessStatusReport{Value: "SUSPENDED"},
			}},
		}, (*tidcommon.ServiceError)(nil)).Once()

	var out agentListMCPResponse
	s.structured(s.call("list_agents", map[string]any{}), &out)

	s.Equal(1, out.TotalResults)
	s.Require().Len(out.Agents, 1)
	s.Equal(agentMCPItem{ID: toolsAgentID, Name: "Helper", Type: "assistant", OUID: "ou-1",
		ClientID: "client-1", Owner: "user-1", Status: "SUSPENDED", ReadOnly: true}, out.Agents[0])
}

func (s *AgentToolsTestSuite) TestListAgents_PaginationAndError() {
	s.mockService.On("GetAgentList", mock.Anything, 5, 10, map[string]interface{}{}, true).
		Return(nil, &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Limit is out of range"}}).Once()

	result := s.call("list_agents", map[string]any{"limit": 5, "offset": 10})

	s.Equal("failed to list agents: Limit is out of range", s.errorText(result))
}

func (s *AgentToolsTestSuite) TestGetAgent() {
	s.mockService.On("GetAgent", mock.Anything, toolsAgentID, true).
		Return(storedAgent(providers.GrantTypeClientCredentials, providers.GrantTypeAuthorizationCode),
			(*tidcommon.ServiceError)(nil)).Once()
	s.mockService.On("GetAgentRoles", mock.Anything, toolsAgentID, agentAccessPageSize, 0).
		Return(&model.AgentRoleListResponse{TotalResults: 1, Roles: []string{"Ticket Reader"}},
			(*tidcommon.ServiceError)(nil)).Once()
	s.mockService.On("GetAgentGroups", mock.Anything, toolsAgentID, agentAccessPageSize, 0).
		Return(&model.AgentGroupListResponse{TotalResults: 1,
			Groups: []model.AgentGroup{{ID: "group-1", Name: "Support Bots", OUID: "ou-1"}}},
			(*tidcommon.ServiceError)(nil)).Once()

	var out agentMCPOutput
	s.structured(s.call("get_agent", map[string]any{"id": toolsAgentID}), &out)

	s.Equal("Helper", out.Name)
	s.Equal("engineering", out.OUHandle)
	s.Equal("user-1", out.Owner)
	s.Equal("ACTIVE", out.Status)
	s.Equal(agentModeDelegated, out.Mode)
	s.Equal("client-1", out.ClientID)
	s.Empty(out.ClientSecret)
	s.Equal([]string{"client_credentials", "authorization_code"}, out.GrantTypes)
	s.Equal([]string{"https://agent.example.com/callback"}, out.RedirectURIs)
	s.Equal([]string{"Ticket Reader"}, out.Roles)
	s.Equal(1, out.TotalRoles)
	s.Equal([]agentMCPGroup{{ID: "group-1", Name: "Support Bots"}}, out.Groups)
	s.Equal(1, out.TotalGroups)
}

func (s *AgentToolsTestSuite) TestGetAgent_Errors() {
	notFound := &tidcommon.ServiceError{ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Agent not found"}}

	s.Run("agent", func() {
		s.mockService.On("GetAgent", mock.Anything, "missing", true).
			Return(nil, notFound).Once()
		s.Equal("failed to get agent: Agent not found",
			s.errorText(s.call("get_agent", map[string]any{"id": "missing"})))
	})
	s.Run("roles", func() {
		s.mockService.On("GetAgent", mock.Anything, toolsAgentID, true).
			Return(storedAgent(), (*tidcommon.ServiceError)(nil)).Once()
		s.mockService.On("GetAgentRoles", mock.Anything, toolsAgentID, agentAccessPageSize, 0).
			Return(nil, &tidcommon.InternalServerError).Once()
		s.Contains(s.errorText(s.call("get_agent", map[string]any{"id": toolsAgentID})),
			"failed to get agent: ")
	})
	s.Run("groups", func() {
		s.mockService.On("GetAgent", mock.Anything, toolsAgentID, true).
			Return(storedAgent(), (*tidcommon.ServiceError)(nil)).Once()
		s.mockService.On("GetAgentRoles", mock.Anything, toolsAgentID, agentAccessPageSize, 0).
			Return(&model.AgentRoleListResponse{}, (*tidcommon.ServiceError)(nil)).Once()
		s.mockService.On("GetAgentGroups", mock.Anything, toolsAgentID, agentAccessPageSize, 0).
			Return(nil, &tidcommon.InternalServerError).Once()
		s.Contains(s.errorText(s.call("get_agent", map[string]any{"id": toolsAgentID})),
			"failed to get agent: ")
	})
}

// captureCreate records the agent the onboard tool hands to the service and answers with the given
// client secret.
func (s *AgentToolsTestSuite) captureCreate(secret string) *providers.Agent {
	received := &providers.Agent{}
	s.mockService.On("CreateAgent", mock.Anything, mock.Anything).
		Return(func(_ context.Context, agent *providers.Agent) (
			*model.AgentCompleteResponse, *tidcommon.ServiceError) {
			*received = *agent
			return &model.AgentCompleteResponse{
				ID: toolsAgentID, OUID: "ou-1", Type: agent.Type, Owner: "user-1",
				InboundAuthConfig: annotateOAuthConfig(agent.InboundAuthConfig, "client-1", secret),
			}, nil
		}).Once()
	return received
}

func (s *AgentToolsTestSuite) TestOnboardAgent_AutonomousByDefault() {
	received := s.captureCreate("new-secret")

	var out agentMCPOutput
	s.structured(s.call("onboard_agent", map[string]any{
		"type": "assistant", "ouHandle": "engineering", "description": "Answers support tickets",
		"attributes": map[string]any{"name": "Helper"},
	}), &out)

	s.Equal("assistant", received.Type)
	s.Equal("engineering", received.OUHandle)
	s.False(received.Delegated)
	s.JSONEq(`{"name":"Helper"}`, string(received.Attributes))
	config := agentOAuthConfig(received.InboundAuthConfig)
	s.Require().NotNil(config)
	s.Equal([]providers.GrantType{providers.GrantTypeClientCredentials}, config.GrantTypes)
	s.Equal(providers.TokenEndpointAuthMethodClientSecretBasic, config.TokenEndpointAuthMethod)

	s.Equal(toolsAgentID, out.ID)
	s.Equal(agentModeAutonomous, out.Mode)
	s.Equal("client-1", out.ClientID)
	s.Equal("new-secret", out.ClientSecret)
}

func (s *AgentToolsTestSuite) TestOnboardAgent_Delegated() {
	received := s.captureCreate("new-secret")

	s.structured(s.call("onboard_agent", map[string]any{
		"type": "assistant", "ouId": "ou-1", "owner": "user-2", "mode": "delegated",
		"redirectUris": []any{"https://agent.example.com/callback"},
	}), &agentMCPOutput{})

	s.True(received.Delegated)
	s.Equal("ou-1", received.OUID)
	s.Equal("user-2", received.Owner)
	config := agentOAuthConfig(received.InboundAuthConfig)
	s.Require().NotNil(config)
	s.Equal([]string{"https://agent.example.com/callback"}, config.RedirectURIs)
}

func (s *AgentToolsTestSuite) TestOnboardAgent_DelegatedNeedsRedirectURIs() {
	result := s.call("onboard_agent", map[string]any{
		"type": "assistant", "ouId": "ou-1", "mode": "delegated",
	})

	s.Equal("failed to onboard agent: delegated mode needs at least one redirect URI", s.errorText(result))
	s.mockService.AssertNotCalled(s.T(), "CreateAgent", mock.Anything, mock.Anything)
}

func (s *AgentToolsTestSuite) TestOnboardAgent_NoCredentials() {
	received := s.captureCreate("")

	var out agentMCPOutput
	s.structured(s.call("onboard_agent", map[string]any{
		"type": "assistant", "ouId": "ou-1", "mode": "none",
	}), &out)

	s.Empty(received.InboundAuthConfig)
	s.Equal(agentModeNone, out.Mode)
	s.Empty(out.ClientSecret)
}

func (s *AgentToolsTestSuite) TestOnboardAgent_ServiceError() {
	s.mockService.On("CreateAgent", mock.Anything, mock.Anything).
		Return(nil, &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Organization unit not found"}}).Once()

	result := s.call("onboard_agent", map[string]any{"type": "assistant", "ouId": "missing"})

	s.Equal("failed to onboard agent: Organization unit not found", s.errorText(result))
}

func (s *AgentToolsTestSuite) TestSetAgentMode_TurnsDelegatedOn() {
	received := s.captureUpdate(storedAgent(providers.GrantTypeClientCredentials), "")

	var out agentMCPOutput
	s.structured(s.call("set_agent_mode", map[string]any{
		"id": toolsAgentID, "mode": "delegated", "redirectUris": []any{"https://agent.example.com/callback"},
	}), &out)

	s.True(received.Delegated)
	s.assertKept(received)
	s.Equal("user-1", received.Owner)
	config := s.oauthConfig(received)
	s.Equal([]providers.GrantType{providers.GrantTypeClientCredentials}, config.GrantTypes)
	s.Equal([]string{"https://agent.example.com/callback"}, config.RedirectURIs)
	s.Empty(config.ClientSecret, "the stored secret is kept by sending none")
	s.Empty(out.ClientSecret)
}

func (s *AgentToolsTestSuite) TestSetAgentMode_DelegatedNeedsRedirectURIs() {
	s.mockService.On("GetAgent", mock.Anything, toolsAgentID, false).
		Return(storedAgent(providers.GrantTypeClientCredentials), (*tidcommon.ServiceError)(nil)).Once()

	result := s.call("set_agent_mode", map[string]any{"id": toolsAgentID, "mode": "delegated"})

	s.Equal("failed to set agent mode: delegated mode needs at least one redirect URI", s.errorText(result))
	s.mockService.AssertNotCalled(s.T(), "UpdateAgent", mock.Anything, mock.Anything, mock.Anything)
}

func (s *AgentToolsTestSuite) TestSetAgentMode_TurnsDelegatedOff() {
	received := s.captureUpdate(storedAgent(providers.GrantTypeClientCredentials,
		providers.GrantTypeAuthorizationCode, providers.GrantTypeRefreshToken, providers.GrantTypeTokenExchange), "")

	var out agentMCPOutput
	s.structured(s.call("set_agent_mode", map[string]any{"id": toolsAgentID, "mode": "autonomous"}), &out)

	s.False(received.Delegated)
	s.assertKept(received)
	config := s.oauthConfig(received)
	s.Equal([]providers.GrantType{providers.GrantTypeClientCredentials}, config.GrantTypes)
	s.Empty(config.ResponseTypes)
	s.False(config.PKCERequired)
	s.Equal(agentModeAutonomous, out.Mode)
}

func (s *AgentToolsTestSuite) TestSetAgentMode_GivesCredentialsToAgentWithout() {
	received := s.captureUpdate(storedAgent(), "issued-secret")

	var out agentMCPOutput
	s.structured(s.call("set_agent_mode", map[string]any{"id": toolsAgentID, "mode": "autonomous"}), &out)

	config := s.oauthConfig(received)
	s.Equal([]providers.GrantType{providers.GrantTypeClientCredentials}, config.GrantTypes)
	s.Equal(agentModeAutonomous, out.Mode)
	s.Equal("issued-secret", out.ClientSecret)
}

func (s *AgentToolsTestSuite) TestApplyAgentMode_RefusesUnknownMode() {
	req := updateRequestFromAgent(storedAgent(providers.GrantTypeClientCredentials))

	s.EqualError(applyAgentMode(req, "sometimes", nil), "mode must be autonomous or delegated")
}

func (s *AgentToolsTestSuite) TestRotateAgentSecret() {
	received := s.captureUpdate(storedAgent(providers.GrantTypeClientCredentials,
		providers.GrantTypeAuthorizationCode), "")

	var out agentMCPOutput
	s.structured(s.call("rotate_agent_secret", map[string]any{"id": toolsAgentID}), &out)

	config := s.oauthConfig(received)
	s.NotEmpty(config.ClientSecret)
	s.Equal(config.ClientSecret, out.ClientSecret)
	s.Equal("client-1", config.ClientID)
	s.True(received.Delegated, "a delegated agent stays held to the delegated grant rules")
	s.assertKept(received)
	s.Equal("user-1", received.Owner)
	s.Equal([]string{"https://agent.example.com/callback"}, config.RedirectURIs)
}

func (s *AgentToolsTestSuite) TestRotateAgentSecret_Refused() {
	publicClient := storedAgent(providers.GrantTypeClientCredentials)
	publicClient.InboundAuthConfig[0].OAuthConfig.PublicClient = true

	cases := []struct {
		name   string
		stored *model.AgentGetResponse
		want   string
	}{
		{"no OAuth client", storedAgent(), "failed to rotate agent secret: " +
			"the agent has no OAuth client; use set_agent_mode to give it one"},
		{"public client", publicClient,
			"failed to rotate agent secret: the agent does not authenticate with a client secret"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.mockService.On("GetAgent", mock.Anything, toolsAgentID, false).
				Return(tc.stored, (*tidcommon.ServiceError)(nil)).Once()
			s.Equal(tc.want, s.errorText(s.call("rotate_agent_secret", map[string]any{"id": toolsAgentID})))
		})
	}
	s.mockService.AssertNotCalled(s.T(), "UpdateAgent", mock.Anything, mock.Anything, mock.Anything)
}

func (s *AgentToolsTestSuite) TestRotateAgentSecret_ReadError() {
	s.mockService.On("GetAgent", mock.Anything, "missing", false).
		Return(nil, &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Agent not found"}}).Once()

	result := s.call("rotate_agent_secret", map[string]any{"id": "missing"})

	s.Equal("failed to rotate agent secret: Agent not found", s.errorText(result))
}

func (s *AgentToolsTestSuite) TestTransferAgentOwnership() {
	received := s.captureUpdate(storedAgent(providers.GrantTypeClientCredentials), "")

	var out agentMCPOutput
	s.structured(s.call("transfer_agent_ownership", map[string]any{
		"id": toolsAgentID, "owner": "user-2"}), &out)

	s.Equal("user-2", received.Owner)
	s.assertKept(received)
	s.False(received.Delegated)
	s.Empty(s.oauthConfig(received).ClientSecret)
	s.Equal("user-2", out.Owner)
	s.Empty(out.ClientSecret)
}

func (s *AgentToolsTestSuite) TestTransferAgentOwnership_Errors() {
	s.Run("empty owner", func() {
		s.mockService.On("GetAgent", mock.Anything, toolsAgentID, false).
			Return(storedAgent(), (*tidcommon.ServiceError)(nil)).Once()
		s.Equal("failed to transfer agent ownership: owner is required", s.errorText(
			s.call("transfer_agent_ownership", map[string]any{"id": toolsAgentID, "owner": ""})))
	})
	s.Run("update refused", func() {
		s.mockService.On("GetAgent", mock.Anything, toolsAgentID, false).
			Return(storedAgent(), (*tidcommon.ServiceError)(nil)).Once()
		s.mockService.On("UpdateAgent", mock.Anything, toolsAgentID, mock.Anything).
			Return(nil, &tidcommon.ServiceError{
				ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Owner not found"}}).Once()
		s.Equal("failed to transfer agent ownership: Owner not found", s.errorText(
			s.call("transfer_agent_ownership", map[string]any{"id": toolsAgentID, "owner": "nobody"})))
	})
}

func (s *AgentToolsTestSuite) TestOffboardAgent() {
	s.mockService.On("DeleteAgent", mock.Anything, toolsAgentID).Return((*tidcommon.ServiceError)(nil)).Once()

	var out agentOffboardMCPResponse
	s.structured(s.call("offboard_agent", map[string]any{"id": toolsAgentID}), &out)

	s.Equal(agentOffboardMCPResponse{ID: toolsAgentID, Deleted: true}, out)
}

func (s *AgentToolsTestSuite) TestOffboardAgent_ServiceError() {
	s.mockService.On("DeleteAgent", mock.Anything, toolsAgentID).Return(&tidcommon.ServiceError{
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Declarative agents cannot be modified"}}).Once()

	result := s.call("offboard_agent", map[string]any{"id": toolsAgentID})

	s.Equal("failed to offboard agent: Declarative agents cannot be modified", s.errorText(result))
}
