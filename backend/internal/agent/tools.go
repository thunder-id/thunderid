// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/agent/model"
	oauthutils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Operating modes the agent tools take and report. An agent acting on its own holds client credentials;
// a delegated agent also acts on behalf of a signed-in user; an agent with no OAuth client holds no
// credentials.
const (
	agentModeAutonomous = "autonomous"
	agentModeDelegated  = "delegated"
	agentModeNone       = "none"
)

// agentAccessPageSize caps the roles and groups the get tool reports for one agent.
const agentAccessPageSize = 100

// delegatedOnlyGrantTypes are the grant types an agent holds only while it acts on behalf of a user. They
// are dropped when the agent goes back to acting only on its own, as the Console's Delegated mode switch
// does.
var delegatedOnlyGrantTypes = []providers.GrantType{
	providers.GrantTypeAuthorizationCode,
	providers.GrantTypeCIBA,
	providers.GrantTypeRefreshToken,
	providers.GrantTypeTokenExchange,
}

// agentTools provides MCP tools for onboarding and managing agents. Each tool handles one task an
// administrator asks for, and goes through the agent service, so validation, authorization and the
// grant rules are the API's.
type agentTools struct {
	service AgentServiceInterface
}

// agentListMCPInput is the list tool input.
type agentListMCPInput struct {
	tool.PaginationInput
}

// agentMCPItem is the list tool item.
type agentMCPItem struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type"`
	OUID     string `json:"ouId"`
	OUHandle string `json:"ouHandle,omitempty"`
	ClientID string `json:"clientId,omitempty"`
	Owner    string `json:"owner,omitempty"`
	Status   string `json:"status,omitempty"`
	ReadOnly bool   `json:"readOnly"`
}

// agentListMCPResponse is the list tool output.
type agentListMCPResponse struct {
	TotalResults int            `json:"totalResults"`
	Agents       []agentMCPItem `json:"agents"`
}

// agentMCPGroup is a group the agent belongs to.
type agentMCPGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// agentMCPOutput describes an agent: who owns it, how it authenticates and, from the get tool, what it can
// access. ClientSecret is set only by the tools that issue a secret, and is never readable again.
type agentMCPOutput struct {
	ID                      string          `json:"id"`
	Name                    string          `json:"name,omitempty"`
	Type                    string          `json:"type"`
	Description             string          `json:"description,omitempty"`
	OUID                    string          `json:"ouId"`
	OUHandle                string          `json:"ouHandle,omitempty"`
	Owner                   string          `json:"owner,omitempty"`
	Status                  string          `json:"status,omitempty"`
	LastLoginAt             string          `json:"lastLoginAt,omitempty"`
	Mode                    string          `json:"mode"`
	ClientID                string          `json:"clientId,omitempty"`
	ClientSecret            string          `json:"clientSecret,omitempty"`
	TokenEndpointAuthMethod string          `json:"tokenEndpointAuthMethod,omitempty"`
	GrantTypes              []string        `json:"grantTypes,omitempty"`
	RedirectURIs            []string        `json:"redirectUris,omitempty"`
	Roles                   []string        `json:"roles,omitempty"`
	TotalRoles              int             `json:"totalRoles,omitempty"`
	Groups                  []agentMCPGroup `json:"groups,omitempty"`
	TotalGroups             int             `json:"totalGroups,omitempty"`
}

// agentOnboardMCPInput is the onboard tool input.
//
//nolint:lll
type agentOnboardMCPInput struct {
	Type         string         `json:"type" jsonschema:"Handle of the agent type. Use list_agent_types to find it."`
	OUID         string         `json:"ouId,omitempty" jsonschema:"ID of the organization unit the agent belongs to. Give ouId or ouHandle."`
	OUHandle     string         `json:"ouHandle,omitempty" jsonschema:"Handle of the organization unit the agent belongs to. Give ouId or ouHandle."`
	Description  string         `json:"description,omitempty" jsonschema:"What the agent does"`
	Owner        string         `json:"owner,omitempty" jsonschema:"ID of the user accountable for the agent. Defaults to the signed-in user."`
	Attributes   map[string]any `json:"attributes,omitempty" jsonschema:"Attribute values the agent type's schema defines, such as name"`
	Mode         string         `json:"mode,omitempty" jsonschema:"How the agent acts. autonomous: only on its own, with client credentials. delegated: also on behalf of a signed-in user, through the authorization code flow with PKCE. none: an identity with no credentials."`
	RedirectURIs []string       `json:"redirectUris,omitempty" jsonschema:"Callback URLs for the signed-in user's authorization. Required for delegated mode."`
}

// agentModeMCPInput is the set mode tool input.
//
//nolint:lll
type agentModeMCPInput struct {
	ID           string   `json:"id" jsonschema:"ID of the agent"`
	Mode         string   `json:"mode" jsonschema:"autonomous: the agent acts only on its own. delegated: the agent also acts on behalf of a signed-in user."`
	RedirectURIs []string `json:"redirectUris,omitempty" jsonschema:"Callback URLs for the signed-in user's authorization. Required to turn delegated mode on unless the agent already has some; replaces the stored list when given."`
}

// agentOwnerMCPInput is the transfer ownership tool input.
type agentOwnerMCPInput struct {
	ID    string `json:"id" jsonschema:"ID of the agent"`
	Owner string `json:"owner" jsonschema:"ID of the user who becomes accountable for the agent"`
}

// agentOffboardMCPResponse is the offboard tool output.
type agentOffboardMCPResponse struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// registerMCPTools registers the agent MCP tools with the server.
func registerMCPTools(server *mcp.Server, service AgentServiceInterface) {
	tools := &agentTools{service: service}

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_agents",
		Description: `List agents: the machine identities of AI assistants, background workers and other ` +
			`software. Returns each agent's ID, name, type, organization unit, OAuth client ID, owner and status.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Agents",
			ReadOnlyHint: true,
		},
	}, tools.listAgents)

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_agent",
		Description: `Describe an agent: who owns it, whether it acts on its own or also on behalf of ` +
			`signed-in users, how it authenticates (client ID, grant types, redirect URIs) and the roles and ` +
			`groups it holds. The client secret is never returned.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Agent",
			ReadOnlyHint: true,
		},
	}, tools.getAgent)

	mcp.AddTool(server, &mcp.Tool{
		Name: "onboard_agent",
		Description: `Set up a new agent with its credentials.

Choose the mode from what the agent has to do:
- autonomous (default): calls APIs as itself, with client credentials.
- delegated: also calls APIs on behalf of a signed-in user. It signs the user in through the ` +
			`authorization code flow with PKCE, so redirectUris is required.
- none: an identity only, with no credentials.

The response carries the client secret. It is shown only once: hand it to whoever runs the agent, ` +
			`and use rotate_agent_secret if it is lost.`,
		InputSchema: getOnboardAgentSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Onboard Agent",
			IdempotentHint: false,
		},
	}, tools.onboardAgent)

	mcp.AddTool(server, &mcp.Tool{
		Name: "set_agent_mode",
		Description: `Change whether an agent acts only on its own (autonomous) or also on behalf of ` +
			`signed-in users (delegated). Other settings are kept.

- delegated adds the authorization code and refresh token grants with PKCE. The agent needs redirect URIs.
- autonomous removes every grant that acts for a user (authorization code, CIBA, refresh token, token exchange).
- An agent onboarded with mode none gets an OAuth client, and the response carries its new secret once.`,
		InputSchema: getSetAgentModeSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Set Agent Mode",
			IdempotentHint: true,
		},
	}, tools.setAgentMode)

	mcp.AddTool(server, &mcp.Tool{
		Name: "rotate_agent_secret",
		Description: `Issue a new client secret for an agent, for example when the old one leaked or is ` +
			`lost. The old secret stops working at once. The new one is returned only in this response. ` +
			`Tokens already issued stay valid until they expire.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Rotate Agent Secret",
			IdempotentHint:  false,
			DestructiveHint: new(true),
		},
	}, tools.rotateAgentSecret)

	mcp.AddTool(server, &mcp.Tool{
		Name: "transfer_agent_ownership",
		Description: `Make another user accountable for an agent, for example when its owner leaves. ` +
			`Only the owner changes.`,
		Annotations: &mcp.ToolAnnotations{
			Title:          "Transfer Agent Ownership",
			IdempotentHint: true,
		},
	}, tools.transferAgentOwnership)

	mcp.AddTool(server, &mcp.Tool{
		Name: "offboard_agent",
		Description: `Permanently remove an agent that is no longer needed: its identity, credentials, ` +
			`role assignments and group memberships. This cannot be undone. Confirm with the user first.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Offboard Agent",
			IdempotentHint:  false,
			DestructiveHint: new(true),
		},
	}, tools.offboardAgent)
}

// listAgents handles the list_agents tool call.
func (t *agentTools) listAgents(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input agentListMCPInput,
) (*mcp.CallToolResult, *agentListMCPResponse, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 20
	}
	resp, svcErr := t.service.GetAgentList(ctx, limit, input.Offset, map[string]interface{}{}, true)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to list agents: %s", svcErr.ErrorDescription)
	}

	items := make([]agentMCPItem, len(resp.Agents))
	for i, agent := range resp.Agents {
		items[i] = agentMCPItem{
			ID:       agent.ID,
			Name:     agent.Display,
			Type:     agent.Type,
			OUID:     agent.OUID,
			OUHandle: agent.OUHandle,
			ClientID: agent.ClientID,
			Owner:    agent.Owner,
			Status:   accessStatus(agent.Status),
			ReadOnly: agent.IsReadOnly,
		}
	}

	return nil, &agentListMCPResponse{TotalResults: resp.TotalResults, Agents: items}, nil
}

// getAgent handles the get_agent tool call.
func (t *agentTools) getAgent(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input tool.IDInput,
) (*mcp.CallToolResult, *agentMCPOutput, error) {
	const failure = "failed to get agent"
	agent, svcErr := t.service.GetAgent(ctx, input.ID, true)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("%s: %s", failure, svcErr.ErrorDescription)
	}
	roles, svcErr := t.service.GetAgentRoles(ctx, input.ID, agentAccessPageSize, 0)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("%s: %s", failure, svcErr.ErrorDescription)
	}
	groups, svcErr := t.service.GetAgentGroups(ctx, input.ID, agentAccessPageSize, 0)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("%s: %s", failure, svcErr.ErrorDescription)
	}

	out := agentOutput(agent.ID, agent.Type, agent.Description, agent.OUID, agent.OUHandle, agent.Owner,
		agent.InboundAuthConfig)
	out.Name = agent.Display
	out.Status = accessStatus(agent.Status)
	out.LastLoginAt = agent.LastLoginAt
	out.Roles = roles.Roles
	out.TotalRoles = roles.TotalResults
	out.Groups = make([]agentMCPGroup, len(groups.Groups))
	for i, group := range groups.Groups {
		out.Groups[i] = agentMCPGroup{ID: group.ID, Name: group.Name}
	}
	out.TotalGroups = groups.TotalResults
	return nil, out, nil
}

// onboardAgent handles the onboard_agent tool call.
func (t *agentTools) onboardAgent(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input agentOnboardMCPInput,
) (*mcp.CallToolResult, *agentMCPOutput, error) {
	const failure = "failed to onboard agent"
	mode := input.Mode
	if mode == "" {
		mode = agentModeAutonomous
	}
	if mode == agentModeDelegated && len(input.RedirectURIs) == 0 {
		return nil, nil, fmt.Errorf("%s: delegated mode needs at least one redirect URI", failure)
	}

	var attributes json.RawMessage
	if len(input.Attributes) > 0 {
		raw, err := json.Marshal(input.Attributes)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: attributes must be a JSON object", failure)
		}
		attributes = raw
	}

	agent := &providers.Agent{
		OUID:        input.OUID,
		OUHandle:    input.OUHandle,
		Type:        input.Type,
		Description: input.Description,
		Owner:       input.Owner,
		Attributes:  attributes,
		Delegated:   mode == agentModeDelegated,
	}
	if mode != agentModeNone {
		agent.InboundAuthConfig = []providers.InboundAuthConfigWithSecret{{
			Type:        providers.OAuthInboundAuthType,
			OAuthConfig: newAgentOAuthConfig(input.RedirectURIs),
		}}
	}

	resp, svcErr := t.service.CreateAgent(ctx, agent)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("%s: %s", failure, svcErr.ErrorDescription)
	}
	return nil, completeOutput(resp), nil
}

// setAgentMode handles the set_agent_mode tool call.
func (t *agentTools) setAgentMode(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input agentModeMCPInput,
) (*mcp.CallToolResult, *agentMCPOutput, error) {
	resp, err := t.changeAgent(ctx, input.ID, "failed to set agent mode",
		func(req *model.UpdateAgentRequest) error {
			return applyAgentMode(req, input.Mode, input.RedirectURIs)
		})
	if err != nil {
		return nil, nil, err
	}
	return nil, completeOutput(resp), nil
}

// rotateAgentSecret handles the rotate_agent_secret tool call. The secret is generated here and sent
// through the agent update, the same way the Console regenerates it.
func (t *agentTools) rotateAgentSecret(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input tool.IDInput,
) (*mcp.CallToolResult, *agentMCPOutput, error) {
	const failure = "failed to rotate agent secret"
	secret, err := oauthutils.GenerateOAuth2ClientSecret()
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %s", failure, tidcommon.InternalServerError.ErrorDescription)
	}

	resp, err := t.changeAgent(ctx, input.ID, failure, func(req *model.UpdateAgentRequest) error {
		oauthConfig := agentOAuthConfig(req.InboundAuthConfig)
		if oauthConfig == nil {
			return errors.New("the agent has no OAuth client; use set_agent_mode to give it one")
		}
		if !requiresClientSecret(oauthConfig) {
			return errors.New("the agent does not authenticate with a client secret")
		}
		oauthConfig.ClientSecret = secret
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	out := completeOutput(resp)
	out.ClientSecret = secret
	return nil, out, nil
}

// transferAgentOwnership handles the transfer_agent_ownership tool call.
func (t *agentTools) transferAgentOwnership(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input agentOwnerMCPInput,
) (*mcp.CallToolResult, *agentMCPOutput, error) {
	resp, err := t.changeAgent(ctx, input.ID, "failed to transfer agent ownership",
		func(req *model.UpdateAgentRequest) error {
			if input.Owner == "" {
				return errors.New("owner is required")
			}
			req.Owner = input.Owner
			return nil
		})
	if err != nil {
		return nil, nil, err
	}
	return nil, completeOutput(resp), nil
}

// offboardAgent handles the offboard_agent tool call.
func (t *agentTools) offboardAgent(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input tool.IDInput,
) (*mcp.CallToolResult, *agentOffboardMCPResponse, error) {
	if svcErr := t.service.DeleteAgent(ctx, input.ID); svcErr != nil {
		return nil, nil, fmt.Errorf("failed to offboard agent: %s", svcErr.ErrorDescription)
	}
	return nil, &agentOffboardMCPResponse{ID: input.ID, Deleted: true}, nil
}

// changeAgent reads the agent, applies one change to it and writes it back as a full update, so a tool
// changes only what its task covers and the rest of the agent is kept. The stored client secret is kept
// unless the change sets a new one.
func (t *agentTools) changeAgent(ctx context.Context, agentID, failure string,
	change func(*model.UpdateAgentRequest) error) (*model.AgentCompleteResponse, error) {
	current, svcErr := t.service.GetAgent(ctx, agentID, false)
	if svcErr != nil {
		return nil, fmt.Errorf("%s: %s", failure, svcErr.ErrorDescription)
	}

	req := updateRequestFromAgent(current)
	if err := change(req); err != nil {
		return nil, fmt.Errorf("%s: %w", failure, err)
	}

	resp, svcErr := t.service.UpdateAgent(ctx, agentID, req)
	if svcErr != nil {
		return nil, fmt.Errorf("%s: %s", failure, svcErr.ErrorDescription)
	}
	return resp, nil
}

// updateRequestFromAgent builds the full update that leaves the agent as it is. The service does not store
// the delegated flag, so it is derived from the grant types for the delegated grant rules to apply. The
// OAuth configuration is copied, so a change to it does not reach the read response.
func updateRequestFromAgent(agent *model.AgentGetResponse) *model.UpdateAgentRequest {
	configs := make([]providers.InboundAuthConfigWithSecret, len(agent.InboundAuthConfig))
	for i, config := range agent.InboundAuthConfig {
		if config.OAuthConfig != nil {
			oauthConfig := *config.OAuthConfig
			config.OAuthConfig = &oauthConfig
		}
		configs[i] = config
	}
	return &model.UpdateAgentRequest{
		OUID:                  agent.OUID,
		Type:                  agent.Type,
		Description:           agent.Description,
		LogoURL:               agent.LogoURL,
		Owner:                 agent.Owner,
		Attributes:            agent.Attributes,
		InboundAuthProfileReq: agent.InboundAuthProfileReq,
		InboundAuthConfig:     configs,
		Delegated:             isDelegatedAgent(configs),
	}
}

// applyAgentMode switches the update to the given mode. Turning delegated mode on leaves the grant types
// to the service, which adds the authorization code and refresh token grants with PKCE to an agent that
// holds only client credentials.
func applyAgentMode(req *model.UpdateAgentRequest, mode string, redirectURIs []string) error {
	oauthConfig := agentOAuthConfig(req.InboundAuthConfig)
	if oauthConfig == nil {
		oauthConfig = newAgentOAuthConfig(nil)
		req.InboundAuthConfig = append(req.InboundAuthConfig, providers.InboundAuthConfigWithSecret{
			Type:        providers.OAuthInboundAuthType,
			OAuthConfig: oauthConfig,
		})
	}
	if len(redirectURIs) > 0 {
		oauthConfig.RedirectURIs = redirectURIs
	}

	switch mode {
	case agentModeDelegated:
		if len(oauthConfig.RedirectURIs) == 0 {
			return errors.New("delegated mode needs at least one redirect URI")
		}
		if !req.Delegated {
			req.Delegated = true
			oauthConfig.GrantTypes = withoutDelegatedOnlyGrantTypes(oauthConfig.GrantTypes)
		}
	case agentModeAutonomous:
		req.Delegated = false
		oauthConfig.GrantTypes = withoutDelegatedOnlyGrantTypes(oauthConfig.GrantTypes)
		oauthConfig.ResponseTypes = nil
		oauthConfig.PKCERequired = false
	default:
		return fmt.Errorf("mode must be %s or %s", agentModeAutonomous, agentModeDelegated)
	}
	return nil
}

// withoutDelegatedOnlyGrantTypes returns the grant types an agent keeps while it acts only on its own.
func withoutDelegatedOnlyGrantTypes(grantTypes []providers.GrantType) []providers.GrantType {
	return slices.DeleteFunc(slices.Clone(grantTypes), func(grantType providers.GrantType) bool {
		return slices.Contains(delegatedOnlyGrantTypes, grantType)
	})
}

// newAgentOAuthConfig returns the OAuth client an agent is onboarded with: a confidential client holding
// client credentials. The service adds the delegated grant types when the agent is delegated.
func newAgentOAuthConfig(redirectURIs []string) *providers.OAuthConfigWithSecret {
	return &providers.OAuthConfigWithSecret{
		GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
		RedirectURIs:            redirectURIs,
	}
}

// agentOAuthConfig returns the agent's OAuth configuration, or nil when it has none.
func agentOAuthConfig(configs []providers.InboundAuthConfigWithSecret) *providers.OAuthConfigWithSecret {
	index := oauthConfigIndex(configs)
	if index < 0 {
		return nil
	}
	return configs[index].OAuthConfig
}

// isDelegatedAgent reports whether the agent acts on behalf of a signed-in user.
func isDelegatedAgent(configs []providers.InboundAuthConfigWithSecret) bool {
	oauthConfig := agentOAuthConfig(configs)
	return oauthConfig != nil && isDelegatedGrantTypes(oauthConfig.GrantTypes)
}

// isDelegatedGrantTypes reports whether the grant types include one that acts for a signed-in user.
func isDelegatedGrantTypes(grantTypes []providers.GrantType) bool {
	return slices.ContainsFunc(grantTypes, func(grantType providers.GrantType) bool {
		return slices.Contains(delegationGrantTypes, grantType)
	})
}

// agentMode names how the agent acts, from its OAuth configuration.
func agentMode(configs []providers.InboundAuthConfigWithSecret) string {
	switch {
	case agentOAuthConfig(configs) == nil:
		return agentModeNone
	case isDelegatedAgent(configs):
		return agentModeDelegated
	default:
		return agentModeAutonomous
	}
}

// agentOutput builds the tool output from an agent's fields and OAuth configuration, without its secret.
func agentOutput(id, agentType, description, ouID, ouHandle, owner string,
	configs []providers.InboundAuthConfigWithSecret) *agentMCPOutput {
	out := &agentMCPOutput{
		ID:          id,
		Type:        agentType,
		Description: description,
		OUID:        ouID,
		OUHandle:    ouHandle,
		Owner:       owner,
		Mode:        agentMode(configs),
	}
	if oauthConfig := agentOAuthConfig(configs); oauthConfig != nil {
		out.ClientID = oauthConfig.ClientID
		out.TokenEndpointAuthMethod = string(oauthConfig.TokenEndpointAuthMethod)
		out.GrantTypes = grantTypeStrings(oauthConfig.GrantTypes)
		out.RedirectURIs = oauthConfig.RedirectURIs
	}
	return out
}

// completeOutput builds the tool output from a create or update response, carrying the client secret
// when the service issued one.
func completeOutput(resp *model.AgentCompleteResponse) *agentMCPOutput {
	out := agentOutput(resp.ID, resp.Type, resp.Description, resp.OUID, resp.OUHandle, resp.Owner,
		resp.InboundAuthConfig)
	if oauthConfig := agentOAuthConfig(resp.InboundAuthConfig); oauthConfig != nil {
		out.ClientSecret = oauthConfig.ClientSecret
	}
	out.Status = accessStatus(resp.Status)
	out.LastLoginAt = resp.LastLoginAt
	return out
}

// accessStatus returns the agent's effective access status, or empty when it is not reported.
func accessStatus(status *providers.AccessStatusReport) string {
	if status == nil {
		return ""
	}
	return string(status.Value)
}

// getOnboardAgentSchema generates the schema for the onboard_agent tool.
func getOnboardAgentSchema() *jsonschema.Schema {
	return tool.GenerateSchema[agentOnboardMCPInput](
		tool.WithEnum("", "mode", []string{agentModeAutonomous, agentModeDelegated, agentModeNone}),
		tool.WithDefault("", "mode", agentModeAutonomous),
	)
}

// getSetAgentModeSchema generates the schema for the set_agent_mode tool.
func getSetAgentModeSchema() *jsonschema.Schema {
	return tool.GenerateSchema[agentModeMCPInput](
		tool.WithEnum("", "mode", []string{agentModeAutonomous, agentModeDelegated}),
	)
}
