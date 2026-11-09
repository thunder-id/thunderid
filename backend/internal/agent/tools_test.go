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
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/executormeta"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/agentmock"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowmgtmock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
)

const (
	toolAgentID  = "agent-1"
	toolClientID = "client-1"
	toolOwnerID  = "user-1"
)

type AgentToolsTestSuite struct {
	suite.Suite
	agents      *agentmock.AgentServiceInterfaceMock
	entities    *entitymock.EntityServiceInterfaceMock
	entityTypes *entitytypemock.EntityTypeServiceInterfaceMock
	roles       *rolemock.RoleServiceInterfaceMock
	resources   *resourcemock.ResourceServiceInterfaceMock
	flows       *flowmgtmock.FlowMgtServiceInterfaceMock
	session     *mcp.ClientSession
}

func TestAgentToolsTestSuite(t *testing.T) {
	suite.Run(t, new(AgentToolsTestSuite))
}

// SetupTest connects with onboard_agent registered, as a control plane registers it.
func (s *AgentToolsTestSuite) SetupTest() {
	s.connect("", true)
}

// connect registers the tools over mocked services and connects an in-memory MCP client, so every call
// crosses the SDK's schema validation and output encoding as an agent's would. caller is the subject
// the tools see. withOnboard registers onboard_agent too, as a control plane does.
func (s *AgentToolsTestSuite) connect(caller string, withOnboard bool) {
	s.agents = agentmock.NewAgentServiceInterfaceMock(s.T())
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.entityTypes = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.roles = rolemock.NewRoleServiceInterfaceMock(s.T())
	s.resources = resourcemock.NewResourceServiceInterfaceMock(s.T())
	// ciba-flow finds the user from login_hint and emails them a link, so approval can run it.
	s.flows = flowmgtmock.NewFlowMgtServiceInterfaceMock(s.T())
	s.flows.On("GetFlow", mock.Anything, "ciba-flow").Return(cibaFlow(), (*tidcommon.ServiceError)(nil)).Maybe()
	s.flows.On("ListFlows", mock.Anything, 100, 0, providers.FlowTypeAuthentication).Return(
		&flowmgt.FlowListResponse{Flows: []flowmgt.BasicFlowDefinition{{ID: "ciba-flow", Handle: "ciba-email"}}},
		(*tidcommon.ServiceError)(nil)).Maybe()
	s.entityTypes.On("GetEntityTypeByHandle", mock.Anything, entitytype.TypeCategoryAgent,
		entitytype.DefaultAgentTypeHandle).Return(&entitytype.EntityType{
		Handle: entitytype.DefaultAgentTypeHandle, SystemAttributes: &entitytype.SystemAttributes{Display: "name"},
	}, (*tidcommon.ServiceError)(nil)).Maybe()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	tools := &agentTools{
		logger:            log.GetLogger(),
		agentService:      s.agents,
		entityService:     s.entities,
		entityTypeService: s.entityTypes,
		roleService:       s.roles,
		resourceService:   s.resources,
		flowService:       s.flows,
	}
	registerMCPTools(server, tools)
	if withOnboard {
		registerOnboardAgentTool(server, tools)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverCtx := security.WithSecurityContext(context.Background(),
		security.NewSecurityContextForTest(caller, "", "", nil, nil))
	serverSession, err := server.Connect(serverCtx, serverTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	s.session, err = client.Connect(context.Background(), clientTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = s.session.Close() })
}

// agentSchema sets the agent type's attributes.
func (s *AgentToolsTestSuite) agentSchema(attributes ...entitytype.AttributeInfo) {
	if len(attributes) == 0 {
		attributes = []entitytype.AttributeInfo{{Attribute: "name", Type: "string", Required: true, Unique: true}}
	}
	s.entityTypes.On("GetAttributes", mock.Anything, entitytype.TypeCategoryAgent,
		entitytype.DefaultAgentTypeHandle, mock.Anything).Return(attributes, (*tidcommon.ServiceError)(nil))
}

// ownerExists makes toolOwnerID a user.
func (s *AgentToolsTestSuite) ownerExists() {
	s.entities.On("GetEntityProfile", mock.Anything, toolOwnerID).
		Return(&providers.Entity{ID: toolOwnerID, Category: providers.EntityCategoryUser}, nil)
}

func (s *AgentToolsTestSuite) call(name string, args map[string]any) *mcp.CallToolResult {
	result, err := s.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	s.Require().NoError(err)
	return result
}

func (s *AgentToolsTestSuite) structured(result *mcp.CallToolResult, out any) {
	s.Require().False(result.IsError, "tool returned an error: %v", result.Content)
	raw, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, out))
}

func (s *AgentToolsTestSuite) errorText(result *mcp.CallToolResult) string {
	s.Require().True(result.IsError)
	s.Require().NotEmpty(result.Content)
	text, ok := result.Content[0].(*mcp.TextContent)
	s.Require().True(ok)
	return text.Text
}

func toolAgent(cfg *providers.OAuthConfigWithSecret) *model.AgentGetResponse {
	agent := &model.AgentGetResponse{
		ID: toolAgentID, OUID: "ou-1", Type: entitytype.DefaultAgentTypeHandle, Display: "support-agent",
		Description: "Helps customers", Owner: &providers.AgentOwner{ID: toolOwnerID}, ClientID: toolClientID,
		Attributes: json.RawMessage(`{"name":"support-agent","model":"gpt"}`),
		Status:     &providers.AccessStatusReport{Value: providers.AccessStatusActive},
		InboundAuthProfileReq: inboundmodel.InboundAuthProfileReq{
			AuthFlowID: "flow-1", ThemeID: "theme-1", AllowedUserTypes: []string{"customer"}},
	}
	if cfg != nil {
		cfg.ClientID = toolClientID
		agent.InboundAuthConfig = []providers.InboundAuthConfigWithSecret{
			{Type: providers.OAuthInboundAuthType, OAuthConfig: cfg}}
	}
	return agent
}

func completeOf(agent *model.AgentGetResponse, secret string) *model.AgentCompleteResponse {
	complete := &model.AgentCompleteResponse{
		ID: agent.ID, OUID: agent.OUID, Type: agent.Type, Description: agent.Description, Owner: agent.Owner,
		Attributes: agent.Attributes, Status: agent.Status, InboundAuthProfileReq: agent.InboundAuthProfileReq,
	}
	if cfg := oauthConfigOf(agent.InboundAuthConfig); cfg != nil {
		withSecret := *cfg
		withSecret.ClientSecret = secret
		complete.InboundAuthConfig = []providers.InboundAuthConfigWithSecret{
			{Type: providers.OAuthInboundAuthType, OAuthConfig: &withSecret}}
	}
	return complete
}

func secretConfig() *providers.OAuthConfigWithSecret {
	return &providers.OAuthConfigWithSecret{
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
		GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
	}
}

func (s *AgentToolsTestSuite) TestRegisterMCPTools() {
	result, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)
	tools := map[string]*mcp.Tool{}
	for _, t := range result.Tools {
		tools[t.Name] = t
		s.NotContains(t.Description, "—", t.Name)
		s.NotContains(t.Description, "--", t.Name)
	}
	s.Len(tools, 5)

	s.True(tools["list_agents"].Annotations.ReadOnlyHint)
	s.True(tools["get_agent"].Annotations.ReadOnlyHint)
	onboard := tools["onboard_agent"].Annotations
	s.False(onboard.ReadOnlyHint)
	s.False(*onboard.DestructiveHint)
	s.False(onboard.IdempotentHint)
	update := tools["update_agent"].Annotations
	s.True(*update.DestructiveHint)
	s.True(update.IdempotentHint)
	rotate := tools["rotate_agent_credentials"].Annotations
	s.True(*rotate.DestructiveHint)
	s.False(rotate.IdempotentHint)
	for _, t := range tools {
		s.False(*t.Annotations.OpenWorldHint, t.Name)
	}

	var schema struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Enum  []string `json:"enum"`
			Items *struct {
				Enum []string `json:"enum"`
			} `json:"items"`
		} `json:"properties"`
	}
	raw, err := json.Marshal(tools["onboard_agent"].InputSchema)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, &schema))
	s.Contains(schema.Required, "name")
	s.ElementsMatch([]string{"client_secret", "private_key_jwt", "none"}, schema.Properties["credential"].Enum)
	s.Require().NotNil(schema.Properties["actsForUsers"].Items)
	s.ElementsMatch([]string{"sign_in", "approval", "token_exchange"}, schema.Properties["actsForUsers"].Items.Enum)
}

// Without onboard_agent, as on a data plane, the other agent tools stand on their own: none refers to
// onboard_agent, and update_agent explains the terms itself.
func (s *AgentToolsTestSuite) TestRegisterMCPTools_WithoutOnboard() {
	s.connect("", false)
	result, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)
	names := make([]string, 0, len(result.Tools))
	for _, t := range result.Tools {
		names = append(names, t.Name)
		s.NotContains(t.Description, "onboard_agent", t.Name)
		if t.Name == "update_agent" {
			s.Contains(t.Description, "- credential:")
			s.Contains(t.Description, "- actsForUsers:")
		}
	}
	s.ElementsMatch([]string{"list_agents", "get_agent", "update_agent", "rotate_agent_credentials"}, names)
}

func (s *AgentToolsTestSuite) TestListAgents() {
	page := &model.AgentListResponse{TotalResults: 1, Agents: []model.BasicAgentResponse{{
		ID: toolAgentID, Display: "support-agent", ClientID: toolClientID,
		Owner: &providers.AgentOwner{ID: toolOwnerID}, OUID: "ou-1",
		Status: &providers.AccessStatusReport{Value: providers.AccessStatusSuspended},
	}}}

	s.Run("by name", func() {
		s.agents.On("GetAgentList", mock.Anything, mock.Anything, 0,
			map[string]interface{}{"name": "support-agent"}, true).Return(page, (*tidcommon.ServiceError)(nil)).Once()
		var out agentListMCPResponse
		s.structured(s.call("list_agents", map[string]any{"name": "support-agent"}), &out)
		s.Equal(1, out.TotalResults)
		s.Equal([]agentMCPItem{{ID: toolAgentID, Name: "support-agent", ClientID: toolClientID, Owner: toolOwnerID,
			OUID: "ou-1", Status: "SUSPENDED"}}, out.Agents)
	})

	s.Run("by owner and name", func() {
		s.agents.On("GetAgentListByOwner", mock.Anything, toolOwnerID, mock.Anything, 0,
			map[string]interface{}{"name": "support-agent"}, true).Return(page, (*tidcommon.ServiceError)(nil)).Once()
		var out agentListMCPResponse
		s.structured(s.call("list_agents", map[string]any{"owner": toolOwnerID, "name": "support-agent"}), &out)
		s.Len(out.Agents, 1)
	})

	s.Run("by organization unit", func() {
		s.agents.On("GetAgentList", mock.Anything, mock.Anything, 0,
			map[string]interface{}{"ouId": "ou-1"}, true).Return(page, (*tidcommon.ServiceError)(nil)).Once()
		var out agentListMCPResponse
		s.structured(s.call("list_agents", map[string]any{"ouId": "ou-1"}), &out)
		s.Len(out.Agents, 1)
	})

	s.Run("organization unit with another filter", func() {
		text := s.errorText(s.call("list_agents", map[string]any{"ouId": "ou-1", "owner": toolOwnerID}))
		s.Contains(text, "ouId can't be combined")
	})

	s.Run("service error", func() {
		s.agents.On("GetAgentList", mock.Anything, mock.Anything, 0, map[string]interface{}{}, true).
			Return((*model.AgentListResponse)(nil), &ErrorInvalidLimit).Once()
		s.Contains(s.errorText(s.call("list_agents", map[string]any{})), "failed to list agents")
	})
}

func (s *AgentToolsTestSuite) TestGetAgent() {
	cfg := secretConfig()
	cfg.GrantTypes = append(cfg.GrantTypes, providers.GrantTypeAuthorizationCode, providers.GrantTypeRefreshToken,
		providers.GrantTypeCIBA, providers.GrantTypeJWTBearer)
	cfg.RedirectURIs = []string{"https://agent.example.com/callback"}

	// The agent's flow-1 only signs in a user who is present, so its approval can't work.
	s.flows.On("GetFlow", mock.Anything, "flow-1").Return(signInFlow(), (*tidcommon.ServiceError)(nil)).Maybe()

	s.Run("by agent ID, in onboard_agent's terms", func() {
		s.agents.On("GetAgent", mock.Anything, toolAgentID, true).
			Return(toolAgent(cfg), (*tidcommon.ServiceError)(nil)).Once()
		var out agentMCPDetails
		s.structured(s.call("get_agent", map[string]any{"agentId": toolAgentID}), &out)
		s.Require().Len(out.Warnings, 1)
		s.Contains(out.Warnings[0], "approval is on but can't work")
		s.Contains(out.Warnings[0], "the flow default-flow can't reach a user who isn't there")
		s.Contains(out.Warnings[0], "flows that can: ciba-email (ciba-flow)")
		s.Equal("support-agent", out.Name)
		s.Equal("client_secret", out.Credential)
		s.Equal([]string{"sign_in", "approval"}, out.ActsForUsers)
		s.Equal([]string{string(providers.GrantTypeJWTBearer)}, out.OtherGrantTypes)
		s.Equal("ACTIVE", out.Status)
		s.Equal("gpt", out.Attributes["model"])
		s.Equal([]string{"groups", "roles", "permissions"}, out.NotIncluded)
		s.Nil(out.Groups)
	})

	s.Run("by client ID", func() {
		s.agents.On("GetAgentByClientID", mock.Anything, toolClientID, true).
			Return(toolAgent(nil), (*tidcommon.ServiceError)(nil)).Once()
		var out agentMCPDetails
		s.structured(s.call("get_agent", map[string]any{"clientId": toolClientID}), &out)
		s.Equal("none", out.Credential)
		s.Empty(out.ActsForUsers)
	})

	s.Run("needs exactly one identifier", func() {
		s.Contains(s.errorText(s.call("get_agent", map[string]any{})), "agentId or clientId")
		s.Contains(s.errorText(s.call("get_agent", map[string]any{"agentId": "a", "clientId": "c"})),
			"agentId or clientId")
	})

	s.Run("with groups, roles and permissions", func() {
		s.agents.On("GetAgent", mock.Anything, toolAgentID, true).
			Return(toolAgent(cfg), (*tidcommon.ServiceError)(nil)).Once()
		s.entities.On("GetTransitiveEntityGroups", mock.Anything, toolAgentID).Return([]providers.EntityGroup{
			{ID: "g1", Name: "Support", OUID: "ou-1"}, {ID: "g2", Name: "Staff", OUID: "ou-1"}}, nil).Once()
		s.entities.On("GetEntityGroups", mock.Anything, toolAgentID, 100, 0).
			Return([]providers.EntityGroup{{ID: "g1"}}, nil).Once()
		s.roles.On("GetUserRoles", mock.Anything, toolAgentID, []string(nil), "").
			Return([]string{"Order Viewer"}, (*tidcommon.ServiceError)(nil)).Once()
		s.roles.On("GetUserRoles", mock.Anything, toolAgentID, []string{"g1", "g2"}, "").
			Return([]string{"Order Viewer", "Order Support"}, (*tidcommon.ServiceError)(nil)).Once()
		permissions := security.PermissionSet{"rs-1": {"orders:update", "orders:read"}}
		s.roles.On("GetAllPermissions", mock.Anything, toolAgentID, []string{"g1", "g2"}).
			Return(permissions, (*tidcommon.ServiceError)(nil)).Once()
		s.resources.On("GetResourceServer", mock.Anything, "rs-1").Return(&providers.ResourceServer{
			Identifier: "https://api.example.com/orders", Name: "Orders"}, (*tidcommon.ServiceError)(nil)).Once()

		var out agentMCPDetails
		s.structured(s.call("get_agent", map[string]any{
			"agentId": toolAgentID, "include": []string{"groups", "roles", "permissions"}}), &out)
		s.Require().NotNil(out.Groups)
		s.Equal([]agentGroupMCPItem{{ID: "g1", Name: "Support", OUID: "ou-1", Membership: "direct"},
			{ID: "g2", Name: "Staff", OUID: "ou-1", Membership: "inherited"}}, *out.Groups)
		s.Require().NotNil(out.Roles)
		s.Equal([]agentRoleMCPItem{{Name: "Order Support", Granted: "through_group"},
			{Name: "Order Viewer", Granted: "directly"}}, *out.Roles)
		s.Require().NotNil(out.Permissions)
		s.Equal([]agentPermissionMCPItem{{ResourceServer: "https://api.example.com/orders", Name: "Orders",
			Permissions: []string{"orders:read", "orders:update"}}}, *out.Permissions)
		s.Empty(out.NotIncluded)
	})

	s.Run("include outside the enum", func() {
		s.Contains(s.errorText(s.call("get_agent", map[string]any{"agentId": toolAgentID,
			"include": []string{"secrets"}})), "include")
	})

	s.Run("service error", func() {
		s.agents.On("GetAgent", mock.Anything, "missing", true).
			Return((*model.AgentGetResponse)(nil), &ErrorAgentNotFound).Once()
		s.Contains(s.errorText(s.call("get_agent", map[string]any{"agentId": "missing"})), "does not exist")
	})
}

func (s *AgentToolsTestSuite) TestOnboardAgent_SecretAgentOnItsOwn() {
	s.agentSchema()
	s.ownerExists()
	var created *providers.Agent
	created1 := completeOf(toolAgent(secretConfig()), "s3cret")
	created1.OUID = "ou-default"
	s.agents.On("CreateAgent", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { created = args.Get(1).(*providers.Agent) }).
		Return(created1, (*tidcommon.ServiceError)(nil)).Once()

	var out onboardAgentMCPResponse
	s.structured(s.call("onboard_agent", map[string]any{"name": "inventory-agent", "owner": toolOwnerID,
		"include": []string{"token_guide"}}), &out)

	s.Require().NotNil(created)
	s.Equal(entitytype.DefaultAgentTypeHandle, created.Type)
	s.Equal(defaultOUHandle, created.OUHandle)
	s.Empty(created.OUID)
	s.Equal(toolOwnerID, created.Owner.ID)
	s.False(created.Delegated)
	s.JSONEq(`{"name":"inventory-agent"}`, string(created.Attributes))
	cfg := oauthConfigOf(created.InboundAuthConfig)
	s.Require().NotNil(cfg)
	s.Equal([]providers.GrantType{providers.GrantTypeClientCredentials}, cfg.GrantTypes)
	s.Equal(providers.TokenEndpointAuthMethodClientSecretBasic, cfg.TokenEndpointAuthMethod)

	s.Equal("s3cret", out.ClientSecret)
	s.Contains(out.Notice, "only time")
	s.Equal("ou-default", out.Agent.OUID)
	s.Contains(out.Endpoints, "control plane")
	s.Contains(out.Endpoints, "/.well-known/openid-configuration")
	s.Require().Len(out.GetTokens, 1)
	s.Equal("on_its_own", out.GetTokens[0].For)
	s.Contains(out.GetTokens[0].ClientAuthentication, "HTTP Basic")
}

func (s *AgentToolsTestSuite) TestOnboardAgent_KeyAgentActingForUsers() {
	s.agentSchema()
	s.ownerExists()
	var created *providers.Agent
	s.agents.On("CreateAgent", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { created = args.Get(1).(*providers.Agent) }).
		Return(func(_ context.Context, agent *providers.Agent) *model.AgentCompleteResponse {
			view := toolAgent(oauthConfigOf(agent.InboundAuthConfig))
			return completeOf(view, "")
		}, (*tidcommon.ServiceError)(nil)).Once()

	var out onboardAgentMCPResponse
	s.structured(s.call("onboard_agent", map[string]any{
		"name": "support-agent", "owner": toolOwnerID, "ouId": "ou-1",
		"credential": "private_key_jwt", "jwksUri": "https://agent.example.com/jwks",
		"actsForUsers": []string{"approval", "sign_in"}, "redirectUris": []string{"https://agent.example.com/cb"},
		"authFlowId": "ciba-flow", "registrationFlowId": "reg-flow", "include": []string{"token_guide"},
	}), &out)

	s.Require().NotNil(created)
	s.Equal("ou-1", created.OUID)
	s.Empty(created.OUHandle)
	s.Equal("ciba-flow", created.AuthFlowID)
	s.Equal("reg-flow", created.RegistrationFlowID)
	s.True(created.IsRegistrationFlowEnabled)
	cfg := oauthConfigOf(created.InboundAuthConfig)
	s.Require().NotNil(cfg)
	s.Equal([]providers.GrantType{providers.GrantTypeClientCredentials, providers.GrantTypeAuthorizationCode,
		providers.GrantTypeCIBA, providers.GrantTypeRefreshToken}, cfg.GrantTypes)
	s.True(cfg.PKCERequired)
	s.Equal(&providers.Certificate{Type: providers.CertificateTypeJWKSURI,
		Value: "https://agent.example.com/jwks"}, cfg.Certificate)

	s.Empty(out.ClientSecret)
	s.Equal("private_key_jwt", out.Agent.Credential)
	ways := make([]string, 0, len(out.GetTokens))
	for _, guide := range out.GetTokens {
		ways = append(ways, guide.For)
		s.Contains(guide.ClientAuthentication, "client_assertion")
	}
	s.Equal([]string{"on_its_own", "sign_in", "approval"}, ways)
	s.Equal("POST /oauth2/bc-authorize", out.GetTokens[2].Steps[0].Request)
	s.Contains(out.GetTokens[1].Note, "refresh_token")
}

func (s *AgentToolsTestSuite) TestOnboardAgent_ReportsEveryProblem() {
	s.agentSchema(
		entitytype.AttributeInfo{Attribute: "name", Type: "string", Required: true},
		entitytype.AttributeInfo{Attribute: "modelProvider", Type: "string", Required: true},
		entitytype.AttributeInfo{Attribute: "password", Type: "string", Required: true, Credential: true},
	)
	s.entities.On("GetEntityProfile", mock.Anything, "app-1").
		Return(&providers.Entity{ID: "app-1", Category: providers.EntityCategoryApp}, nil)

	text := s.errorText(s.call("onboard_agent", map[string]any{
		"name": "support-agent", "owner": "app-1", "credential": "none",
		"actsForUsers": []string{"sign_in", "approval"},
	}))
	s.Contains(text, "nothing was created")
	for _, problem := range []string{"modelProvider", "password", "Console", "owner must be", "need a credential",
		"redirectUris", "authFlowId"} {
		s.Contains(text, problem)
	}
	s.agents.AssertNotCalled(s.T(), "CreateAgent", mock.Anything, mock.Anything)
}

func (s *AgentToolsTestSuite) TestOnboardAgent_Owner() {
	s.Run("without one, the service makes the caller the owner", func() {
		s.connect("caller-1", true)
		s.agentSchema()
		var created *providers.Agent
		s.agents.On("CreateAgent", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { created = args.Get(1).(*providers.Agent) }).
			Return(completeOf(toolAgent(nil), ""), (*tidcommon.ServiceError)(nil)).Once()
		var out onboardAgentMCPResponse
		s.structured(s.call("onboard_agent", map[string]any{"name": "a", "credential": "none"}), &out)
		s.Require().NotNil(created)
		s.Nil(created.Owner, "the service resolves the caller, local or external")
		s.Nil(created.InboundAuthConfig)
		s.Empty(out.GetTokens)
	})

	s.Run("a given owner must be an existing user", func() {
		s.connect("caller-1", true)
		s.agentSchema()
		s.entities.On("GetEntityProfile", mock.Anything, "not-a-user").
			Return(&providers.Entity{ID: "not-a-user", Category: providers.EntityCategoryAgent}, nil).Once()
		s.Contains(s.errorText(s.call("onboard_agent", map[string]any{"name": "a", "owner": "not-a-user"})),
			"owner must be the ID of an existing user")
		s.agents.AssertNotCalled(s.T(), "CreateAgent", mock.Anything, mock.Anything)
	})
}

func (s *AgentToolsTestSuite) TestOnboardAgent_ServiceErrors() {
	s.Run("Default organization unit missing", func() {
		s.agentSchema()
		s.ownerExists()
		s.agents.On("CreateAgent", mock.Anything, mock.Anything).
			Return((*model.AgentCompleteResponse)(nil), &ErrorOrganizationUnitNotFound).Once()
		s.Contains(s.errorText(s.call("onboard_agent", map[string]any{"name": "a", "owner": toolOwnerID})),
			"give ouId")
	})

	s.Run("private key in jwks", func() {
		s.connect("", true)
		s.agentSchema()
		s.ownerExists()
		text := s.errorText(s.call("onboard_agent", map[string]any{
			"name": "a", "owner": toolOwnerID, "credential": "private_key_jwt",
			"jwks": map[string]any{"keys": []any{map[string]any{"kty": "EC", "kid": "k1", "d": "secret"}}},
		}))
		s.Contains(text, "private member")
	})
}

func (s *AgentToolsTestSuite) TestUpdateAgent_AddsApprovalAndKeepsTheRest() {
	s.agentSchema()
	current := &providers.OAuthConfigWithSecret{
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
		GrantTypes: []providers.GrantType{providers.GrantTypeClientCredentials, providers.GrantTypeAuthorizationCode,
			providers.GrantTypeRefreshToken, providers.GrantTypeJWTBearer},
		ResponseTypes: []providers.ResponseType{providers.ResponseTypeCode},
		RedirectURIs:  []string{"https://agent.example.com/callback"},
		Scopes:        []string{"openid", "orders:read"},
	}
	s.agents.On("GetAgent", mock.Anything, toolAgentID, false).
		Return(toolAgent(current), (*tidcommon.ServiceError)(nil)).Once()
	var req *model.UpdateAgentRequest
	s.agents.On("UpdateAgent", mock.Anything, toolAgentID, mock.Anything).
		Run(func(args mock.Arguments) { req = args.Get(2).(*model.UpdateAgentRequest) }).
		Return(completeOf(toolAgent(secretConfig()), ""), (*tidcommon.ServiceError)(nil)).Once()

	var out updateAgentMCPResponse
	s.structured(s.call("update_agent", map[string]any{"agentId": toolAgentID,
		"actsForUsers": []string{"sign_in", "approval"}, "authFlowId": "ciba-flow"}), &out)

	s.Require().NotNil(req)
	s.Equal(entitytype.DefaultAgentTypeHandle, req.Type)
	s.Equal("ou-1", req.OUID)
	s.Equal(toolOwnerID, req.Owner.ID)
	s.Equal("Helps customers", req.Description)
	s.JSONEq(`{"name":"support-agent","model":"gpt"}`, string(req.Attributes))
	s.Equal("theme-1", req.ThemeID)
	s.Equal([]string{"customer"}, req.AllowedUserTypes)
	s.Equal("ciba-flow", req.AuthFlowID)
	s.Nil(req.ValidUntil)
	s.False(req.Delegated)
	cfg := oauthConfigOf(req.InboundAuthConfig)
	s.Require().NotNil(cfg)
	s.Equal(toolClientID, cfg.ClientID)
	s.Empty(cfg.ClientSecret)
	s.Equal(providers.TokenEndpointAuthMethodClientSecretPost, cfg.TokenEndpointAuthMethod)
	s.False(cfg.PKCERequired)
	s.Equal([]string{"openid", "orders:read"}, cfg.Scopes)
	s.Contains(cfg.GrantTypes, providers.GrantTypeCIBA)
	s.Contains(cfg.GrantTypes, providers.GrantTypeJWTBearer)
	s.Empty(out.ClientSecret)
}

func (s *AgentToolsTestSuite) TestUpdateAgent_RenameAndMergeAttributes() {
	s.agentSchema()
	current := secretConfig()
	s.agents.On("GetAgent", mock.Anything, toolAgentID, false).
		Return(toolAgent(current), (*tidcommon.ServiceError)(nil)).Once()
	var req *model.UpdateAgentRequest
	s.agents.On("UpdateAgent", mock.Anything, toolAgentID, mock.Anything).
		Run(func(args mock.Arguments) { req = args.Get(2).(*model.UpdateAgentRequest) }).
		Return(completeOf(toolAgent(secretConfig()), ""), (*tidcommon.ServiceError)(nil)).Once()

	s.structured(s.call("update_agent", map[string]any{"agentId": toolAgentID, "name": "renamed",
		"description": "", "attributes": map[string]any{"model": nil, "function": "support"}}),
		&updateAgentMCPResponse{})

	s.Require().NotNil(req)
	s.JSONEq(`{"name":"renamed","function":"support"}`, string(req.Attributes))
	s.Empty(req.Description)
	s.Equal(current, oauthConfigOf(req.InboundAuthConfig))
	s.Equal("flow-1", req.AuthFlowID)
}

func (s *AgentToolsTestSuite) TestUpdateAgent_Refusals() {
	s.Run("deleting the OAuth client", func() {
		s.agentSchema()
		s.agents.On("GetAgent", mock.Anything, toolAgentID, false).
			Return(toolAgent(secretConfig()), (*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("update_agent", map[string]any{"agentId": toolAgentID, "credential": "none",
			"actsForUsers": []string{}}))
		s.Contains(text, "deleted for good")
		s.agents.AssertNotCalled(s.T(), "UpdateAgent", mock.Anything, mock.Anything, mock.Anything)
	})

	s.Run("a blank name", func() {
		s.connect("", false)
		s.agentSchema()
		s.agents.On("GetAgent", mock.Anything, toolAgentID, false).
			Return(toolAgent(secretConfig()), (*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("update_agent", map[string]any{"agentId": toolAgentID, "name": "   "}))
		s.Contains(text, "name can't be blank")
		s.agents.AssertNotCalled(s.T(), "UpdateAgent", mock.Anything, mock.Anything, mock.Anything)
	})

	s.Run("a credential attribute", func() {
		s.connect("", false)
		s.agentSchema(entitytype.AttributeInfo{Attribute: "password", Type: "string", Credential: true})
		s.agents.On("GetAgent", mock.Anything, toolAgentID, false).
			Return(toolAgent(secretConfig()), (*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("update_agent", map[string]any{"agentId": toolAgentID,
			"attributes": map[string]any{"password": "x"}}))
		s.Contains(text, "password is a credential attribute")
	})

	s.Run("agent not found", func() {
		s.connect("", false)
		s.agents.On("GetAgent", mock.Anything, "missing", false).
			Return((*model.AgentGetResponse)(nil), &ErrorAgentNotFound).Once()
		s.Contains(s.errorText(s.call("update_agent", map[string]any{"agentId": "missing", "name": "x"})),
			"failed to update agent")
	})
}

func (s *AgentToolsTestSuite) TestUpdateAgent_SwitchToSecretReturnsIt() {
	s.agentSchema()
	keyed := &providers.OAuthConfigWithSecret{
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT,
		GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
		Certificate:             &providers.Certificate{Type: providers.CertificateTypeJWKS, Value: `{"keys":[]}`},
	}
	s.agents.On("GetAgent", mock.Anything, toolAgentID, false).
		Return(toolAgent(keyed), (*tidcommon.ServiceError)(nil)).Once()
	var req *model.UpdateAgentRequest
	s.agents.On("UpdateAgent", mock.Anything, toolAgentID, mock.Anything).
		Run(func(args mock.Arguments) { req = args.Get(2).(*model.UpdateAgentRequest) }).
		Return(completeOf(toolAgent(secretConfig()), "new-secret"), (*tidcommon.ServiceError)(nil)).Once()

	var out updateAgentMCPResponse
	s.structured(s.call("update_agent", map[string]any{"agentId": toolAgentID, "credential": "client_secret"}), &out)

	cfg := oauthConfigOf(req.InboundAuthConfig)
	s.Require().NotNil(cfg)
	s.Nil(cfg.Certificate)
	s.Equal(providers.TokenEndpointAuthMethodClientSecretBasic, cfg.TokenEndpointAuthMethod)
	s.Equal("new-secret", out.ClientSecret)
	s.Contains(out.Notice, "only time")
	s.Equal(toolAgentID, out.ID)
}

func (s *AgentToolsTestSuite) TestRotateAgentCredentials_Secret() {
	s.agents.On("GetAgent", mock.Anything, toolAgentID, false).
		Return(toolAgent(secretConfig()), (*tidcommon.ServiceError)(nil))
	s.agents.On("RegenerateAgentSecret", mock.Anything, toolAgentID).
		Return("fresh", (*tidcommon.ServiceError)(nil)).Once()

	var out rotateAgentCredentialsMCPResponse
	s.structured(s.call("rotate_agent_credentials", map[string]any{"agentId": toolAgentID}), &out)
	s.Equal("client_secret", out.Credential)
	s.Equal("fresh", out.ClientSecret)
	s.Contains(out.Notice, "old secret no longer works")

	text := s.errorText(s.call("rotate_agent_credentials", map[string]any{"agentId": toolAgentID,
		"removeKeyId": "k1"}))
	s.Contains(text, "uses a client secret")
}

func (s *AgentToolsTestSuite) TestRotateAgentCredentials_KeySet() {
	stored, problem := keySetFromJWKS(map[string]any{"keys": []any{publicKey("k1")}})
	s.Require().Empty(problem)
	keyed := func() *providers.OAuthConfigWithSecret {
		return &providers.OAuthConfigWithSecret{
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT,
			GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
			Certificate:             &providers.Certificate{Type: providers.CertificateTypeJWKS, Value: stored},
		}
	}

	s.Run("adds a key next to the current one", func() {
		s.agents.On("GetAgent", mock.Anything, toolAgentID, false).
			Return(toolAgent(keyed()), (*tidcommon.ServiceError)(nil)).Once()
		var req *model.UpdateAgentRequest
		s.agents.On("UpdateAgent", mock.Anything, toolAgentID, mock.Anything).
			Run(func(args mock.Arguments) { req = args.Get(2).(*model.UpdateAgentRequest) }).
			Return(completeOf(toolAgent(keyed()), ""), (*tidcommon.ServiceError)(nil)).Once()

		var out rotateAgentCredentialsMCPResponse
		s.structured(s.call("rotate_agent_credentials", map[string]any{"agentId": toolAgentID,
			"addKey": publicKey("k2")}), &out)
		s.Equal([]string{"k1", "k2"}, out.KeyIDs)
		s.Contains(out.Notice, "removeKeyId")
		cfg := oauthConfigOf(req.InboundAuthConfig)
		s.Require().NotNil(cfg)
		s.Equal([]string{"k1", "k2"}, keyIDs(cfg.Certificate.Value))
		s.JSONEq(`{"name":"support-agent","model":"gpt"}`, string(req.Attributes))
	})

	s.Run("won't remove the last key", func() {
		s.agents.On("GetAgent", mock.Anything, toolAgentID, false).
			Return(toolAgent(keyed()), (*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("rotate_agent_credentials", map[string]any{"agentId": toolAgentID,
			"removeKeyId": "k1"}))
		s.Contains(text, "last key")
	})

	s.Run("key set URI has nothing to rotate", func() {
		byURI := keyed()
		byURI.Certificate = &providers.Certificate{Type: providers.CertificateTypeJWKSURI,
			Value: "https://agent.example.com/jwks"}
		s.agents.On("GetAgent", mock.Anything, toolAgentID, false).
			Return(toolAgent(byURI), (*tidcommon.ServiceError)(nil)).Once()
		var out rotateAgentCredentialsMCPResponse
		s.structured(s.call("rotate_agent_credentials", map[string]any{"agentId": toolAgentID}), &out)
		s.Contains(out.Notice, "nothing to rotate")
	})

	s.Run("no credential", func() {
		s.agents.On("GetAgent", mock.Anything, toolAgentID, false).
			Return(toolAgent(nil), (*tidcommon.ServiceError)(nil)).Once()
		s.Contains(s.errorText(s.call("rotate_agent_credentials", map[string]any{"agentId": toolAgentID})),
			"no credential to rotate")
	})

	s.agents.AssertNumberOfCalls(s.T(), "UpdateAgent", 1)
}

// RegisterOnboardAgentTool, which a control plane calls, adds onboard_agent alone.
func (s *AgentToolsTestSuite) TestRegisterOnboardAgentTool() {
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	RegisterOnboardAgentTool(server, s.agents, s.entities, s.entityTypes, s.flows)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	s.Require().NoError(err)
	defer func() { _ = serverSession.Close() }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	s.Require().NoError(err)
	defer func() { _ = session.Close() }()

	result, err := session.ListTools(context.Background(), nil)
	s.Require().NoError(err)
	s.Require().Len(result.Tools, 1)
	s.Equal("onboard_agent", result.Tools[0].Name)
}

// cibaFlow is a flow that finds the user from login_hint and emails them an approval link.
// An approval flow must be an authentication flow, however its steps look.
func (s *AgentToolsTestSuite) TestApprovalFlowMustBeAnAuthenticationFlow() {
	s.agentSchema()
	s.ownerExists()
	admin := cibaFlow()
	admin.ID, admin.FlowType = "admin-flow", providers.FlowTypeAdministration
	s.flows.On("GetFlow", mock.Anything, "admin-flow").Return(admin, (*tidcommon.ServiceError)(nil)).Once()
	text := s.errorText(s.call("onboard_agent", map[string]any{"name": "a", "owner": toolOwnerID,
		"actsForUsers": []string{"approval"}, "authFlowId": "admin-flow"}))
	s.Contains(text, "approval needs an authentication flow")
	s.agents.AssertNotCalled(s.T(), "CreateAgent", mock.Anything, mock.Anything)
}

func cibaFlow() *providers.CompleteFlowDefinition {
	return &providers.CompleteFlowDefinition{ID: "ciba-flow", Handle: "ciba-email",
		FlowType: providers.FlowTypeAuthentication, Nodes: []providers.NodeDefinition{
			{ID: "identify", Executor: &providers.ExecutorDefinition{Name: executormeta.ExecutorNameIdentifying},
				Properties: map[string]interface{}{loginHintAttributeProperty: "username"}},
			{ID: "send", Executor: &providers.ExecutorDefinition{Name: executormeta.ExecutorNameEmailExecutor}},
		}}
}

// signInFlow is a flow that asks whoever is present for credentials, so it can't reach a user who isn't
// there.
func signInFlow() *providers.CompleteFlowDefinition {
	return &providers.CompleteFlowDefinition{ID: "flow-1", Handle: "default-flow",
		FlowType: providers.FlowTypeAuthentication, Nodes: []providers.NodeDefinition{
			{ID: "credentials", Executor: &providers.ExecutorDefinition{Name: "CredentialsAuthExecutor"}},
		}}
}

func (s *AgentToolsTestSuite) TestReachesOfflineUser() {
	s.True(reachesOfflineUser(cibaFlow()))
	s.False(reachesOfflineUser(signInFlow()))

	noHint := cibaFlow()
	noHint.Nodes[0].Properties = nil
	s.False(reachesOfflineUser(noHint), "an identifying step without loginHintAttribute can't use login_hint")

	noLink := cibaFlow()
	noLink.Nodes = noLink.Nodes[:1]
	s.False(reachesOfflineUser(noLink), "a flow that sends nothing can't reach the user")

	bySMS := cibaFlow()
	bySMS.Nodes[1].Executor.Name = executormeta.ExecutorNameSMSExecutor
	s.True(reachesOfflineUser(bySMS))
}

// Approval is refused with a flow that can't reach a user who isn't there, and the refusal names the
// flows that can.
func (s *AgentToolsTestSuite) TestApprovalFlowChecks() {
	s.Run("onboarding with a sign-in flow", func() {
		s.agentSchema()
		s.ownerExists()
		s.flows.On("GetFlow", mock.Anything, "flow-1").Return(signInFlow(),
			(*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("onboard_agent", map[string]any{"name": "a", "owner": toolOwnerID,
			"actsForUsers": []string{"approval"}, "authFlowId": "flow-1"}))
		s.Contains(text, "nothing was created")
		s.Contains(text, "the flow default-flow can't reach a user who isn't there")
		s.Contains(text, "flows that can: ciba-email (ciba-flow)")
		s.agents.AssertNotCalled(s.T(), "CreateAgent", mock.Anything, mock.Anything)
	})

	s.Run("onboarding with no flow names the ones that can", func() {
		s.connect("", true)
		s.agentSchema()
		s.ownerExists()
		text := s.errorText(s.call("onboard_agent", map[string]any{"name": "a", "owner": toolOwnerID,
			"actsForUsers": []string{"approval"}}))
		s.Contains(text, "approval needs authFlowId")
		s.Contains(text, "flows that can: ciba-email (ciba-flow)")
	})

	s.Run("adding approval to an agent whose flow can't reach the user", func() {
		s.connect("", true)
		s.agentSchema()
		s.agents.On("GetAgent", mock.Anything, toolAgentID, false).
			Return(toolAgent(secretConfig()), (*tidcommon.ServiceError)(nil)).Once()
		s.flows.On("GetFlow", mock.Anything, "flow-1").Return(signInFlow(),
			(*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("update_agent", map[string]any{"agentId": toolAgentID,
			"actsForUsers": []string{"approval"}}))
		s.Contains(text, "nothing was changed")
		s.Contains(text, "the flow default-flow can't reach a user who isn't there")
		s.agents.AssertNotCalled(s.T(), "UpdateAgent", mock.Anything, mock.Anything, mock.Anything)
	})

	s.Run("no flow can reach the user yet", func() {
		s.connect("", true)
		s.flows.ExpectedCalls = nil
		s.agentSchema()
		s.ownerExists()
		s.flows.On("GetFlow", mock.Anything, "flow-1").Return(signInFlow(),
			(*tidcommon.ServiceError)(nil)).Twice()
		s.flows.On("ListFlows", mock.Anything, 100, 0, providers.FlowTypeAuthentication).Return(
			&flowmgt.FlowListResponse{Flows: []flowmgt.BasicFlowDefinition{{ID: "flow-1", Handle: "default-flow"}}},
			(*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("onboard_agent", map[string]any{"name": "a", "owner": toolOwnerID,
			"actsForUsers": []string{"approval"}, "authFlowId": "flow-1"}))
		s.Contains(text, "ask the administrator to create one")
	})
}

// The approval flow and the flows suggested for it are read whatever flows the caller may read, as an
// agent may name a flow of another organization unit.
func (s *AgentToolsTestSuite) TestApprovalFlowProblems_ReadsFlowsAsTheRuntime() {
	flows := flowmgtmock.NewFlowMgtServiceInterfaceMock(s.T())
	runtime := mock.MatchedBy(security.IsRuntimeContext)
	flows.On("GetFlow", runtime, "ciba-flow").Return(cibaFlow(), (*tidcommon.ServiceError)(nil))
	flows.On("ListFlows", runtime, 100, 0, providers.FlowTypeAuthentication).Return(
		&flowmgt.FlowListResponse{Flows: []flowmgt.BasicFlowDefinition{{ID: "ciba-flow", Handle: "ciba-email"}}},
		(*tidcommon.ServiceError)(nil))
	tools := &agentTools{logger: log.GetLogger(), flowService: flows}

	problems, err := tools.approvalFlowProblems(context.Background(), "")
	s.Require().NoError(err)
	s.Equal([]string{"flows that can: ciba-email (ciba-flow)"}, problems)
}

// An agent whose approval flow can reach the user gets no warning.
func (s *AgentToolsTestSuite) TestGetAgent_NoWarningWithACIBAFlow() {
	cfg := secretConfig()
	cfg.GrantTypes = append(cfg.GrantTypes, providers.GrantTypeCIBA)
	agent := toolAgent(cfg)
	agent.AuthFlowID = "ciba-flow"
	s.agents.On("GetAgent", mock.Anything, toolAgentID, true).Return(agent, (*tidcommon.ServiceError)(nil)).Once()
	var out agentMCPDetails
	s.structured(s.call("get_agent", map[string]any{"agentId": toolAgentID}), &out)
	s.Equal([]string{"approval"}, out.ActsForUsers)
	s.Empty(out.Warnings)
}

// onboard_agent leaves the token guide out unless asked, and says so.
func (s *AgentToolsTestSuite) TestOnboardAgent_TokenGuideOnlyWhenAsked() {
	s.agentSchema()
	s.ownerExists()
	s.agents.On("CreateAgent", mock.Anything, mock.Anything).
		Return(completeOf(toolAgent(secretConfig()), "s3cret"), (*tidcommon.ServiceError)(nil)).Once()
	var out onboardAgentMCPResponse
	s.structured(s.call("onboard_agent", map[string]any{"name": "a", "owner": toolOwnerID}), &out)
	s.Empty(out.GetTokens)
	s.Empty(out.Endpoints)
	s.Equal([]string{"token_guide"}, out.NotIncluded)
	s.Equal("s3cret", out.ClientSecret, "the secret is always returned, since it is shown only once")
}

func (s *AgentToolsTestSuite) TestChangedFields() {
	before := agentMCPDetails{ID: "a1", Name: "bot", Description: "old", Credential: "client_secret",
		ActsForUsers: []string{}, RedirectURIs: []string{"https://a/cb"}}
	after := agentMCPDetails{ID: "a1", Name: "bot", Description: "new", Credential: "client_secret",
		ActsForUsers: []string{"token_exchange"}}
	s.Equal(map[string]any{"description": "new", "actsForUsers": []any{"token_exchange"}, "redirectUris": nil},
		changedFields(before, after))
	s.Empty(changedFields(after, after), "an update that changes nothing reports nothing")
}

// Every agent tool needs the agent permission its API needs, so none falls back to root by accident.
func (s *AgentToolsTestSuite) TestToolsRequireTheAgentAPIPermissions() {
	s.connect(toolOwnerID, true)
	tools, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)
	s.Require().NotEmpty(tools.Tools)
	security.ForEachPermissionPlaneForTest(func(plane string) {
		s.Run(plane, func() {
			p := security.GetSystemPermissions()
			for _, registered := range tools.Tools {
				s.Contains([]string{p.AgentView, p.AgentCreate, p.AgentUpdate},
					security.ResolveMCPToolPermission(registered.Name),
					"tool %q must require an agent permission", registered.Name)
			}
		})
	})
}
