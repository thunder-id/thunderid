// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/executormeta"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// agentTools carries the services the agent MCP tools call.
type agentTools struct {
	logger            *log.Logger
	agentService      AgentServiceInterface
	entityService     entity.EntityServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	roleService       role.RoleServiceInterface
	resourceService   resource.ResourceServiceInterface
	// flowService reads the flows approval can run, to check one can reach a user who isn't there.
	flowService flowmgt.FlowMgtServiceInterface
}

// listAgentsMCPInput is the input of the list_agents tool.
//
//nolint:lll
type listAgentsMCPInput struct {
	Name   string `json:"name,omitempty" jsonschema:"Exact name of the agent. Can be combined with owner."`
	Owner  string `json:"owner,omitempty" jsonschema:"User ID of the agent's owner, the person accountable for it. Can be combined with name."`
	OUID   string `json:"ouId,omitempty" jsonschema:"ID of an organization unit, to list the agents it owns and then the ones shared into it. Can't be combined with name or owner."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Maximum number of agents to return, at most 100"`
	Offset int    `json:"offset,omitempty" jsonschema:"Number of agents to skip"`
}

// getAgentMCPInput is the input of the get_agent tool.
//
//nolint:lll
type getAgentMCPInput struct {
	AgentID  string   `json:"agentId,omitempty" jsonschema:"ID of the agent, from list_agents. Give agentId or clientId."`
	ClientID string   `json:"clientId,omitempty" jsonschema:"OAuth client ID the agent gets tokens with. Give agentId or clientId."`
	Include  []string `json:"include,omitempty" jsonschema:"Details to add. groups: the groups the agent is in, directly or through another group. roles: its roles, granted directly or through a group. permissions: what it can do on each API, from all its roles."`
}

// agentAccessMCPInput is how an agent proves who it is and acts for users, as update_agent takes it.
//
//nolint:lll
type agentAccessMCPInput struct {
	Credential         string         `json:"credential,omitempty" jsonschema:"How the agent proves who it is. client_secret: ThunderID issues a secret. private_key_jwt: the agent signs with its own key, given as jwksUri or jwks, so no secret is issued; prefer it when the agent can hold a private key. none: no credential."`
	JWKSURI            string         `json:"jwksUri,omitempty" jsonschema:"URL of the agent's public key set, for private_key_jwt"`
	JWKS               map[string]any `json:"jwks,omitempty" jsonschema:"The agent's public key set, for private_key_jwt. A JSON Web Key Set whose keys each have a unique kid and no private members."`
	ActsForUsers       []string       `json:"actsForUsers,omitempty" jsonschema:"The ways the agent acts for users, in any combination. sign_in: a user who is present signs in, and the agent acts in that session; needs redirectUris. approval: the agent asks a user who isn't there, or is offline, to approve on their own device (CIBA); needs authFlowId. token_exchange: the agent exchanges a user token it already holds, or takes sub-tasks from another agent. Empty: the agent acts only on its own."`
	RedirectURIs       []string       `json:"redirectUris,omitempty" jsonschema:"Addresses a user returns to after signing in, for sign_in"`
	AllowedUserTypes   []string       `json:"allowedUserTypes,omitempty" jsonschema:"Handles of the user types the agent can register and resolve when it signs users in. Their attributes are the ones its tokens can carry."`
	AuthFlowID         string         `json:"authFlowId,omitempty" jsonschema:"ID of the authentication flow that sign_in and approval both run. approval needs a flow that can reach a user who isn't there; if the one given can't, the error names the flows that can. Without one, sign_in runs the default flow."`
	RegistrationFlowID string         `json:"registrationFlowId,omitempty" jsonschema:"ID of the flow a new user registers through when signing in, for sign_in. Giving one turns registration on."`
}

// updateAgentMCPInput is the input of the update_agent tool.
//
//nolint:lll
type updateAgentMCPInput struct {
	AgentID     string         `json:"agentId" jsonschema:"ID of the agent, from list_agents or get_agent"`
	Name        string         `json:"name,omitempty" jsonschema:"New name of the agent. No other agent can have it."`
	Description *string        `json:"description,omitempty" jsonschema:"New description. An empty string clears it."`
	Attributes  map[string]any `json:"attributes,omitempty" jsonschema:"Attributes to change, merged into the current ones. A value of null removes that attribute."`
	agentAccessMCPInput
}

// rotateAgentCredentialsMCPInput is the input of the rotate_agent_credentials tool.
//
//nolint:lll
type rotateAgentCredentialsMCPInput struct {
	AgentID     string         `json:"agentId" jsonschema:"ID of the agent, from list_agents or get_agent"`
	AddKey      map[string]any `json:"addKey,omitempty" jsonschema:"For an agent with a registered key set: a public key to add, as a JSON Web Key with a kid no other key uses"`
	RemoveKeyID string         `json:"removeKeyId,omitempty" jsonschema:"For an agent with a registered key set: the kid of a key to retire, once the agent signs with its new key"`
}

// agentMCPItem is an agent in the list_agents output.
type agentMCPItem struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	ClientID string `json:"clientId,omitempty"`
	Owner    string `json:"owner,omitempty"`
	OUID     string `json:"ouId"`
	Status   string `json:"status,omitempty"`
}

// agentListMCPResponse is the list_agents output.
type agentListMCPResponse struct {
	TotalResults int            `json:"totalResults"`
	Agents       []agentMCPItem `json:"agents"`
}

// agentMCPDetails is one agent, described in the terms update_agent takes.
type agentMCPDetails struct {
	ID                 string                    `json:"id"`
	Name               string                    `json:"name,omitempty"`
	Description        string                    `json:"description,omitempty"`
	OUID               string                    `json:"ouId"`
	Owner              string                    `json:"owner,omitempty"`
	Status             string                    `json:"status,omitempty"`
	ValidUntil         string                    `json:"validUntil,omitempty"`
	ClientID           string                    `json:"clientId,omitempty"`
	Credential         string                    `json:"credential"`
	JWKSURI            string                    `json:"jwksUri,omitempty"`
	KeyIDs             []string                  `json:"keyIds,omitempty"`
	ActsForUsers       []string                  `json:"actsForUsers"`
	RedirectURIs       []string                  `json:"redirectUris,omitempty"`
	AllowedUserTypes   []string                  `json:"allowedUserTypes,omitempty"`
	AuthFlowID         string                    `json:"authFlowId,omitempty"`
	RegistrationFlowID string                    `json:"registrationFlowId,omitempty"`
	OtherGrantTypes    []string                  `json:"otherGrantTypes,omitempty"`
	Attributes         map[string]any            `json:"attributes,omitempty"`
	Groups             *[]agentGroupMCPItem      `json:"groups,omitempty"`
	Roles              *[]agentRoleMCPItem       `json:"roles,omitempty"`
	Permissions        *[]agentPermissionMCPItem `json:"permissions,omitempty"`
	NotIncluded        []string                  `json:"notIncluded,omitempty"`
	Warnings           []string                  `json:"warnings,omitempty"`
}

// agentGroupMCPItem is a group an agent is in.
type agentGroupMCPItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	OUID       string `json:"ouId"`
	Membership string `json:"membership"`
}

// agentRoleMCPItem is a role an agent holds.
type agentRoleMCPItem struct {
	Name    string `json:"name"`
	Granted string `json:"granted"`
}

// agentPermissionMCPItem is what an agent can do on one API.
type agentPermissionMCPItem struct {
	API         string   `json:"api"`
	Name        string   `json:"name,omitempty"`
	Permissions []string `json:"permissions"`
}

// updateAgentMCPResponse is the update_agent output.
type updateAgentMCPResponse struct {
	ID string `json:"id"`
	// Changed holds each field of the agent, in get_agent's terms, that the update changed, with its new
	// value. A field the update removed is null.
	Changed      map[string]any `json:"changed"`
	ClientSecret string         `json:"clientSecret,omitempty"`
	Notice       string         `json:"notice,omitempty"`
}

// rotateAgentCredentialsMCPResponse is the rotate_agent_credentials output.
type rotateAgentCredentialsMCPResponse struct {
	Credential   string   `json:"credential"`
	ClientSecret string   `json:"clientSecret,omitempty"`
	KeyIDs       []string `json:"keyIds,omitempty"`
	Notice       string   `json:"notice"`
}

// credentialNames lists the credentials update_agent takes.
func credentialNames() []string {
	return []string{string(credentialClientSecret), string(credentialPrivateKeyJWT), string(credentialNone)}
}

// actingModeNames lists the ways of acting for users update_agent takes.
func actingModeNames() []string {
	modes := make([]string, 0, len(actingModes))
	for _, mode := range actingModes {
		modes = append(modes, string(mode))
	}
	return modes
}

// registerMCPTools registers the agent MCP tools.
func registerMCPTools(server *mcp.Server, tools *agentTools) {
	credentials := credentialNames()
	modes := actingModeNames()
	details := make([]string, 0, len(agentDetails))
	for _, detail := range agentDetails {
		details = append(details, string(detail))
	}

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_agents",
		Description: `Find agents by exact name, by owner (the user ID of the person accountable for an agent), ` +
			`or by organization unit. name and owner can be combined; ouId can't be combined with either. ` +
			`Without filters, it lists every agent you can see. Returns each agent's ID, name, client ID, ` +
			`owner, organization unit and status (ACTIVE, LOCKED, SUSPENDED or EXPIRED), 30 per page unless ` +
			`limit says otherwise, and the total. Use get_agent for one agent's credential, the ways it ` +
			`acts for users, and its groups, roles and permissions.`,
		InputSchema: tool.GenerateSchema[listAgentsMCPInput](
			tool.WithDefault("", "limit", defaultAgentPageSize)),
		Annotations: &mcp.ToolAnnotations{
			Title:         "List Agents",
			ReadOnlyHint:  true,
			OpenWorldHint: new(false),
		},
	}, tools.listAgents)

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_agent",
		Description: `Get one agent, by agentId or by clientId. Returns how it proves who it is (credential: ` +
			`client_secret, private_key_jwt or none) and the ways it acts for users (actsForUsers: sign_in, ` +
			`approval, token_exchange) with their settings, in the terms update_agent takes, ` +
			`and its name, owner, organization unit, status and attributes. credential none without a clientId ` +
			`means the agent has no OAuth client. otherGrantTypes lists grants these terms don't describe. It ` +
			`never returns a secret.

Groups, roles and permissions come back only when include asks for them. notIncluded lists the ones ` +
			`left out, so a section missing for that reason doesn't mean the agent has none. warnings says when ` +
			`the agent is set up in a way that can't work, such as approval with a flow that can't reach the user.`,
		InputSchema: tool.GenerateSchema[getAgentMCPInput](tool.WithEnum("", "include", details)),
		Annotations: &mcp.ToolAnnotations{
			Title:         "Get Agent",
			ReadOnlyHint:  true,
			OpenWorldHint: new(false),
		},
	}, tools.getAgent)

	mcp.AddTool(server, &mcp.Tool{
		Name: "update_agent",
		Description: `Change an agent's name, description, attributes, credential, or the ways it acts for ` +
			`users. Describe what the agent does, not OAuth settings:
` + accessTerms + `

It changes only what you send. actsForUsers, when sent, is the complete set, so read the current one ` +
			`with get_agent first. Attributes are merged, and null removes one. Switching to client_secret ` +
			`issues a new secret, returned once. Removing a way of acting stops the agent from getting those ` +
			`tokens, so confirm it with the administrator first. The owner and organization unit can't change ` +
			`here, and every problem with the input is reported in one error, with nothing changed. The ` +
			`result has the agent's ID and the fields that changed, with their new values; get_agent reads the ` +
			`whole agent.`,
		InputSchema: tool.GenerateSchema[updateAgentMCPInput](
			tool.WithEnum("", "credential", credentials),
			tool.WithEnum("", "actsForUsers", modes)),
		Annotations: &mcp.ToolAnnotations{
			Title:           "Update Agent",
			DestructiveHint: new(true),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
	}, tools.updateAgent)

	mcp.AddTool(server, &mcp.Tool{
		Name: "rotate_agent_credentials",
		Description: `Replace an agent's credential and nothing else. For a client secret, it issues a new ` +
			`secret, returned once, and the old one stops working at once. For a registered key set, addKey ` +
			`adds the agent's new public key next to the current ones, and removeKeyId retires an old key once ` +
			`the agent signs with the new one. For a key set URI, rotate the keys at that URI; there is nothing ` +
			`to change in ThunderID. Access tokens already issued stay valid until they expire. Refresh tokens ` +
			`issued before a secret rotation stop working.`,
		InputSchema: tool.GenerateSchema[rotateAgentCredentialsMCPInput](),
		Annotations: &mcp.ToolAnnotations{
			Title:           "Rotate Agent Credentials",
			DestructiveHint: new(true),
			IdempotentHint:  false,
			OpenWorldHint:   new(false),
		},
	}, tools.rotateAgentCredentials)
}

// listAgents handles the list_agents tool call.
func (t *agentTools) listAgents(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input listAgentsMCPInput,
) (*mcp.CallToolResult, *agentListMCPResponse, error) {
	if input.OUID != "" && (input.Name != "" || input.Owner != "") {
		return nil, nil, errors.New("failed to list agents: ouId can't be combined with name or owner")
	}

	filters := map[string]interface{}{}
	if input.Name != "" {
		nameAttribute, svcErr := t.nameAttribute(ctx)
		if svcErr != nil {
			return nil, nil, fmt.Errorf("failed to list agents: %s", svcErr.ErrorDescription)
		}
		filters[nameAttribute] = input.Name
	}

	var resp *model.AgentListResponse
	var svcErr *tidcommon.ServiceError
	switch {
	case input.Owner != "":
		resp, svcErr = t.agentService.GetAgentListByOwner(ctx, input.Owner, input.Limit, input.Offset, filters, true)
	case input.OUID != "":
		filters[filter.AttributeOUID] = input.OUID
		resp, svcErr = t.agentService.GetAgentList(ctx, input.Limit, input.Offset, filters, true)
	default:
		resp, svcErr = t.agentService.GetAgentList(ctx, input.Limit, input.Offset, filters, true)
	}
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to list agents: %s", svcErr.ErrorDescription)
	}

	agents := make([]agentMCPItem, 0, len(resp.Agents))
	for _, agent := range resp.Agents {
		agents = append(agents, agentMCPItem{
			ID:       agent.ID,
			Name:     agent.Display,
			ClientID: agent.ClientID,
			Owner:    ownerIDOf(agent.Owner),
			OUID:     agent.OUID,
			Status:   statusOf(agent.Status),
		})
	}
	return nil, &agentListMCPResponse{TotalResults: resp.TotalResults, Agents: agents}, nil
}

// getAgent handles the get_agent tool call.
func (t *agentTools) getAgent(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input getAgentMCPInput,
) (*mcp.CallToolResult, *agentMCPDetails, error) {
	if (input.AgentID == "") == (input.ClientID == "") {
		return nil, nil, errors.New("failed to get agent: give either agentId or clientId")
	}

	var agent *model.AgentGetResponse
	var svcErr *tidcommon.ServiceError
	if input.AgentID != "" {
		agent, svcErr = t.agentService.GetAgent(ctx, input.AgentID, true)
	} else {
		agent, svcErr = t.agentService.GetAgentByClientID(ctx, input.ClientID, true)
	}
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get agent: %s", svcErr.ErrorDescription)
	}
	nameAttribute, svcErr := t.nameAttribute(ctx)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get agent: %s", svcErr.ErrorDescription)
	}

	details := describeAgent(agent, nameAttribute)
	if err := t.addAgentDetails(ctx, &details, input.Include); err != nil {
		return nil, nil, fmt.Errorf("failed to get agent: %w", err)
	}
	// An agent set up before approval's flow was checked, or whose flow changed since, can report approval
	// that can't work, so get_agent says so.
	if slices.Contains(details.ActsForUsers, string(actingApproval)) {
		problems, err := t.approvalFlowProblems(ctx, agent.AuthFlowID)
		if err != nil {
			details.Warnings = append(details.Warnings, "approval is on, and whether its flow can work could not "+
				"be checked: "+err.Error())
		} else if len(problems) > 0 {
			details.Warnings = append(details.Warnings, "approval is on but can't work: "+
				strings.Join(problems, "; ")+". Give it a flow that can with update_agent's authFlowId")
		}
	}
	return nil, &details, nil
}

// updateAgent handles the update_agent tool call. The agent service replaces the whole agent, so the
// tool reads it, applies the change, and writes back everything it read.
func (t *agentTools) updateAgent(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input updateAgentMCPInput,
) (*mcp.CallToolResult, *updateAgentMCPResponse, error) {
	current, svcErr := t.agentService.GetAgent(ctx, input.AgentID, false)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to update agent: %s", svcErr.ErrorDescription)
	}
	nameAttribute, svcErr := t.nameAttribute(ctx)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to update agent: %s", svcErr.ErrorDescription)
	}

	attributes := decodeAttributes(current.Attributes)
	for name, value := range input.Attributes {
		if value == nil {
			delete(attributes, name)
		} else {
			attributes[name] = value
		}
	}
	if input.Name != "" {
		attributes[nameAttribute] = input.Name
	}
	problems, err := t.attributeProblems(ctx, attributes, input.Attributes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update agent: %w", err)
	}
	if input.Name != "" && strings.TrimSpace(input.Name) == "" {
		problems = append(problems, "name can't be blank")
	}

	authFlowID := current.AuthFlowID
	if input.AuthFlowID != "" {
		authFlowID = input.AuthFlowID
	}
	currentConfig := oauthConfigOf(current.InboundAuthConfig)
	oauthConfig := currentConfig
	if changesAccess(input.agentAccessMCPInput) {
		change := capabilityChange{
			jwksURI:      input.JWKSURI,
			jwks:         input.JWKS,
			redirectURIs: input.RedirectURIs,
			authFlowID:   authFlowID,
		}
		if input.Credential != "" {
			credential := agentCredential(input.Credential)
			change.credential = &credential
		}
		if input.ActsForUsers != nil {
			change.actsForUsers = modesOf(input.ActsForUsers)
		}
		var accessProblems []string
		oauthConfig, accessProblems = applyCapabilityChange(currentConfig, change)
		problems = append(problems, accessProblems...)
		if len(accessProblems) == 0 && oauthConfig == nil && currentConfig != nil {
			problems = append(problems, "with no credential and no way of acting, the agent's OAuth client "+
				"and client ID would be deleted for good, which update_agent doesn't do")
		}
	}
	// Approval is checked when this change adds it or gives it another flow, so an unrelated change
	// never fails on a setup the agent already has.
	hadApproval := slices.Contains(describeCapabilities(currentConfig).actsForUsers, actingApproval)
	wantsApproval := hadApproval
	if input.ActsForUsers != nil {
		wantsApproval = slices.Contains(input.ActsForUsers, string(actingApproval))
	}
	if wantsApproval && (!hadApproval || input.AuthFlowID != "") {
		flowProblems, err := t.approvalFlowProblems(ctx, authFlowID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to update agent: %w", err)
		}
		problems = append(problems, flowProblems...)
	}
	if len(problems) > 0 {
		return nil, nil, fmt.Errorf("cannot update the agent, so nothing was changed: %s",
			strings.Join(problems, "; "))
	}

	rawAttributes, err := json.Marshal(attributes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update agent: attributes are not valid JSON")
	}
	req := updateRequestOf(current, rawAttributes, oauthConfig)
	if input.Description != nil {
		req.Description = *input.Description
	}
	req.AuthFlowID = authFlowID
	if input.RegistrationFlowID != "" {
		req.RegistrationFlowID = input.RegistrationFlowID
		req.IsRegistrationFlowEnabled = true
	}
	if input.AllowedUserTypes != nil {
		req.AllowedUserTypes = input.AllowedUserTypes
	}

	updated, svcErr := t.agentService.UpdateAgent(ctx, current.ID, req)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to update agent: %s", svcErr.ErrorDescription)
	}
	before := describeAgent(current, nameAttribute)
	out := &updateAgentMCPResponse{ID: current.ID,
		Changed: changedFields(before, describeAgent(readViewOf(updated), nameAttribute))}
	if secret := clientSecretOf(updated.InboundAuthConfig); secret != "" {
		out.ClientSecret = secret
		out.Notice = secretNotice
	}
	return nil, out, nil
}

// rotateAgentCredentials handles the rotate_agent_credentials tool call.
func (t *agentTools) rotateAgentCredentials(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input rotateAgentCredentialsMCPInput,
) (*mcp.CallToolResult, *rotateAgentCredentialsMCPResponse, error) {
	current, svcErr := t.agentService.GetAgent(ctx, input.AgentID, false)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to rotate agent credentials: %s", svcErr.ErrorDescription)
	}
	currentConfig := oauthConfigOf(current.InboundAuthConfig)
	caps := describeCapabilities(currentConfig)
	changesKeys := input.AddKey != nil || input.RemoveKeyID != ""

	switch caps.credential {
	case credentialClientSecret:
		if changesKeys {
			return nil, nil, errors.New("failed to rotate agent credentials: addKey and removeKeyId apply to a " +
				"registered key set, and this agent uses a client secret")
		}
		secret, regenerateErr := t.agentService.RegenerateAgentSecret(ctx, current.ID)
		if regenerateErr != nil {
			return nil, nil, fmt.Errorf("failed to rotate agent credentials: %s", regenerateErr.ErrorDescription)
		}
		return nil, &rotateAgentCredentialsMCPResponse{
			Credential:   string(credentialClientSecret),
			ClientSecret: secret,
			Notice: "The old secret no longer works. " + secretNotice + " Access tokens already issued stay " +
				"valid until they expire, and refresh tokens issued before now no longer work.",
		}, nil
	case credentialPrivateKeyJWT:
		return t.rotateKeySet(ctx, current, currentConfig, input)
	case credentialNone:
	}
	return nil, nil, errors.New("failed to rotate agent credentials: the agent has no credential to rotate")
}

// rotateKeySet adds or retires keys in an agent's registered key set, leaving every other setting as it is.
func (t *agentTools) rotateKeySet(ctx context.Context, current *model.AgentGetResponse,
	currentConfig *providers.OAuthConfigWithSecret, input rotateAgentCredentialsMCPInput) (
	*mcp.CallToolResult, *rotateAgentCredentialsMCPResponse, error) {
	if currentConfig.Certificate == nil || currentConfig.Certificate.Type != providers.CertificateTypeJWKS {
		if input.AddKey != nil || input.RemoveKeyID != "" {
			return nil, nil, errors.New("failed to rotate agent credentials: the agent's keys come from a key " +
				"set URI, so rotate them there")
		}
		return nil, &rotateAgentCredentialsMCPResponse{
			Credential: string(credentialPrivateKeyJWT),
			Notice: "The agent's keys come from a key set URI, so there is nothing to rotate in ThunderID. " +
				"Publish the new key at that URI, then retire the old one there once the agent signs with the new key.",
		}, nil
	}
	if input.AddKey == nil && input.RemoveKeyID == "" {
		return nil, nil, errors.New("failed to rotate agent credentials: give addKey to add the agent's new " +
			"public key, or removeKeyId to retire an old one")
	}

	value := currentConfig.Certificate.Value
	problem := ""
	if input.AddKey != nil {
		value, problem = addKey(value, input.AddKey)
	}
	if problem == "" && input.RemoveKeyID != "" {
		value, problem = removeKey(value, input.RemoveKeyID)
	}
	if problem != "" {
		return nil, nil, fmt.Errorf("cannot rotate the agent's keys, so nothing was changed: %s", problem)
	}

	updatedConfig := *currentConfig
	updatedConfig.Certificate = &providers.Certificate{Type: providers.CertificateTypeJWKS, Value: value}
	req := updateRequestOf(current, current.Attributes, &updatedConfig)
	if _, svcErr := t.agentService.UpdateAgent(ctx, current.ID, req); svcErr != nil {
		return nil, nil, fmt.Errorf("failed to rotate agent credentials: %s", svcErr.ErrorDescription)
	}
	notice := "The agent's key set is updated. Access tokens already issued stay valid until they expire."
	if input.AddKey != nil && input.RemoveKeyID == "" {
		notice += " Once the agent signs with its new key, retire the old one with removeKeyId."
	}
	return nil, &rotateAgentCredentialsMCPResponse{
		Credential: string(credentialPrivateKeyJWT),
		KeyIDs:     keyIDs(value),
		Notice:     notice,
	}, nil
}

// nameAttribute returns the agent type attribute that holds an agent's name, its display attribute.
func (t *agentTools) nameAttribute(ctx context.Context) (string, *tidcommon.ServiceError) {
	agentType, svcErr := t.entityTypeService.GetEntityTypeByHandle(
		ctx, entitytype.TypeCategoryAgent, entitytype.DefaultAgentTypeHandle)
	if svcErr != nil {
		return "", svcErr
	}
	if agentType.SystemAttributes != nil && agentType.SystemAttributes.Display != "" {
		return agentType.SystemAttributes.Display, nil
	}
	return defaultAgentNameAttribute, nil
}

// attributeProblems checks an agent's attributes against its type. Required attributes must be present,
// and no credential attribute is accepted, since no credential goes through the MCP tools.
func (t *agentTools) attributeProblems(ctx context.Context, attributes, given map[string]any) (
	[]string, error) {
	schema, svcErr := t.entityTypeService.GetAttributes(ctx, entitytype.TypeCategoryAgent,
		entitytype.DefaultAgentTypeHandle, entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true})
	if svcErr != nil {
		return nil, errors.New(svcErr.ErrorDescription.String())
	}
	slices.SortFunc(schema, func(a, b entitytype.AttributeInfo) int {
		return strings.Compare(a.Attribute, b.Attribute)
	})

	problems := []string{}
	for _, attribute := range schema {
		_, isGiven := given[attribute.Attribute]
		switch {
		case attribute.Credential && isGiven:
			problems = append(problems, fmt.Sprintf("%s is a credential attribute, and no credential goes "+
				"through these tools", attribute.Attribute))
		case !attribute.Credential && attribute.Required && attributes[attribute.Attribute] == nil:
			problems = append(problems, fmt.Sprintf("the agent type requires the attribute %s", attribute.Attribute))
		}
	}
	return problems, nil
}

// addAgentDetails adds the sections get_agent was asked for, and lists the rest as not included.
func (t *agentTools) addAgentDetails(ctx context.Context, details *agentMCPDetails, include []string) error {
	for _, detail := range agentDetails {
		if !slices.Contains(include, string(detail)) {
			details.NotIncluded = append(details.NotIncluded, string(detail))
		}
	}
	if len(include) == 0 {
		return nil
	}

	groups, err := t.entityService.GetTransitiveEntityGroups(ctx, details.ID)
	if err != nil {
		t.logger.Error(ctx, "Failed to read an agent's groups", log.Error(err))
		return errors.New("its groups could not be read")
	}
	groupIDs := make([]string, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}

	if slices.Contains(include, string(agentDetailGroups)) {
		direct, directErr := directGroupsOf(ctx, t.entityService, details.ID)
		if directErr != nil {
			t.logger.Error(ctx, "Failed to read an agent's direct groups", log.Error(directErr))
			return errors.New("its groups could not be read")
		}
		items := make([]agentGroupMCPItem, 0, len(groups))
		for _, group := range groups {
			membership := "inherited"
			if slices.ContainsFunc(direct, func(d providers.EntityGroup) bool { return d.ID == group.ID }) {
				membership = "direct"
			}
			items = append(items, agentGroupMCPItem{
				ID: group.ID, Name: group.Name, OUID: group.OUID, Membership: membership})
		}
		details.Groups = &items
	}

	if slices.Contains(include, string(agentDetailRoles)) {
		direct, svcErr := t.roleService.GetUserRoles(ctx, details.ID, nil, "")
		if svcErr != nil {
			return errors.New("its roles could not be read")
		}
		all, svcErr := t.roleService.GetUserRoles(ctx, details.ID, groupIDs, "")
		if svcErr != nil {
			return errors.New("its roles could not be read")
		}
		slices.Sort(all)
		items := make([]agentRoleMCPItem, 0, len(all))
		for _, name := range slices.Compact(all) {
			granted := "through_group"
			if slices.Contains(direct, name) {
				granted = "directly"
			}
			items = append(items, agentRoleMCPItem{Name: name, Granted: granted})
		}
		details.Roles = &items
	}

	if slices.Contains(include, string(agentDetailPermissions)) {
		permissions, svcErr := t.roleService.GetAllPermissions(ctx, details.ID, groupIDs)
		if svcErr != nil {
			return errors.New("its permissions could not be read")
		}
		items := make([]agentPermissionMCPItem, 0, len(permissions))
		for _, serverID := range slices.Sorted(maps.Keys(permissions)) {
			item := agentPermissionMCPItem{
				API: serverID, Permissions: slices.Sorted(slices.Values(permissions[serverID]))}
			if server, svcErr := t.resourceService.GetResourceServer(ctx, serverID); svcErr == nil && server != nil {
				item.API = server.Identifier
				item.Name = server.Name
			}
			items = append(items, item)
		}
		details.Permissions = &items
	}
	return nil
}

// directGroupsOf reads every page of the groups an entity is directly in.
func directGroupsOf(ctx context.Context, entityService entity.EntityServiceInterface, entityID string) (
	[]providers.EntityGroup, error) {
	direct := []providers.EntityGroup{}
	for offset := 0; ; {
		page, err := entityService.GetEntityGroups(ctx, entityID, serverconst.MaxPageSize, offset)
		if err != nil {
			return nil, err
		}
		direct = append(direct, page...)
		offset += len(page)
		if len(page) < serverconst.MaxPageSize {
			return direct, nil
		}
	}
}

// approvalFlowProblems checks that the flow approval runs can reach a user who isn't there. Given no flow,
// it only names the flows that can, since the missing authFlowId is reported with the other checks.
func (t *agentTools) approvalFlowProblems(ctx context.Context, flowID string) ([]string, error) {
	// An agent may name a flow of another organization unit, such as a default flow, so the flows are read
	// without the caller's authorization.
	ctx = security.WithRuntimeContext(ctx)
	problems := []string{}
	if flowID != "" {
		flow, svcErr := t.flowService.GetFlow(ctx, flowID)
		switch {
		case svcErr != nil && svcErr.Type == tidcommon.ClientErrorType:
			problems = append(problems, fmt.Sprintf("authFlowId %s names no flow", flowID))
		case svcErr != nil:
			return nil, fmt.Errorf("the approval flow could not be read: %s", svcErr.ErrorDescription)
		case flow.FlowType != providers.FlowTypeAuthentication:
			problems = append(problems, fmt.Sprintf("authFlowId %s names a %s flow, and approval needs an "+
				"authentication flow", flowID, flow.FlowType))
		case reachesOfflineUser(flow):
			return nil, nil
		default:
			problems = append(problems, fmt.Sprintf("the flow %s can't reach a user who isn't there: approval "+
				"needs a flow that finds the user from login_hint and sends them a link by email or SMS", flow.Handle))
		}
	}
	suitable, err := t.approvalFlows(ctx)
	if err != nil {
		return nil, err
	}
	if len(suitable) == 0 {
		return append(problems, "no flow here can reach a user who isn't there yet, so ask the administrator to "+
			"create one, for example from the CIBA Email Notification template"), nil
	}
	return append(problems, "flows that can: "+strings.Join(suitable, ", ")), nil
}

// approvalFlows names the authentication flows that can reach a user who isn't there, as "handle (id)".
func (t *agentTools) approvalFlows(ctx context.Context) ([]string, error) {
	suitable := []string{}
	for offset := 0; ; {
		list, svcErr := t.flowService.ListFlows(ctx, serverconst.MaxPageSize, offset, providers.FlowTypeAuthentication)
		if svcErr != nil {
			return nil, fmt.Errorf("the authentication flows could not be listed: %s", svcErr.ErrorDescription)
		}
		for _, basic := range list.Flows {
			flow, svcErr := t.flowService.GetFlow(ctx, basic.ID)
			if svcErr != nil {
				return nil, fmt.Errorf("the flow %s could not be read: %s", basic.Handle, svcErr.ErrorDescription)
			}
			if reachesOfflineUser(flow) {
				suitable = append(suitable, fmt.Sprintf("%s (%s)", flow.Handle, flow.ID))
			}
		}
		offset += len(list.Flows)
		if len(list.Flows) == 0 || offset >= list.TotalResults {
			return suitable, nil
		}
	}
}

// describeAgent describes an agent in the terms update_agent takes.
func describeAgent(agent *model.AgentGetResponse, nameAttribute string) agentMCPDetails {
	caps := describeCapabilities(oauthConfigOf(agent.InboundAuthConfig))
	attributes := decodeAttributes(agent.Attributes)
	name := agent.Display
	if name == "" {
		name, _ = attributes[nameAttribute].(string)
	}
	modes := make([]string, 0, len(caps.actsForUsers))
	for _, mode := range caps.actsForUsers {
		modes = append(modes, string(mode))
	}

	details := agentMCPDetails{
		ID:               agent.ID,
		Name:             name,
		Description:      agent.Description,
		OUID:             agent.OUID,
		Owner:            ownerIDOf(agent.Owner),
		Status:           statusOf(agent.Status),
		ValidUntil:       agent.ValidUntil,
		ClientID:         agent.ClientID,
		Credential:       string(caps.credential),
		JWKSURI:          caps.jwksURI,
		KeyIDs:           caps.keyIDs,
		ActsForUsers:     modes,
		RedirectURIs:     caps.redirectURIs,
		AllowedUserTypes: agent.AllowedUserTypes,
		AuthFlowID:       agent.AuthFlowID,
		OtherGrantTypes:  caps.otherGrantTypes,
		Attributes:       attributes,
	}
	if agent.IsRegistrationFlowEnabled {
		details.RegistrationFlowID = agent.RegistrationFlowID
	}
	return details
}

// ownerIDOf returns an agent owner's ID, or an empty string when the agent has no owner.
func ownerIDOf(owner *providers.AgentOwner) string {
	if owner == nil {
		return ""
	}
	return owner.ID
}

// changesAccess reports whether an update_agent input changes how the agent proves who it is or acts for
// users.
func changesAccess(input agentAccessMCPInput) bool {
	return input.Credential != "" || input.JWKSURI != "" || input.JWKS != nil || input.ActsForUsers != nil ||
		input.RedirectURIs != nil
}

// updateRequestOf builds an agent update that writes back everything read about the agent, with the
// given attributes and OAuth settings. The agent service replaces the whole agent, so anything left out
// would be deleted. The lifespan is left out, which keeps it as it is.
func updateRequestOf(current *model.AgentGetResponse, attributes json.RawMessage,
	oauthConfig *providers.OAuthConfigWithSecret) *model.UpdateAgentRequest {
	req := &model.UpdateAgentRequest{
		OUID:                  current.OUID,
		Type:                  current.Type,
		Description:           current.Description,
		LogoURL:               current.LogoURL,
		Owner:                 current.Owner,
		Attributes:            attributes,
		InboundAuthProfileReq: current.InboundAuthProfileReq,
	}
	if oauthConfig != nil {
		req.InboundAuthConfig = []providers.InboundAuthConfigWithSecret{
			{Type: providers.OAuthInboundAuthType, OAuthConfig: oauthConfig},
		}
	}
	return req
}

// readViewOf adapts a create or update result to the read shape the tools describe.
func readViewOf(agent *model.AgentCompleteResponse) *model.AgentGetResponse {
	view := &model.AgentGetResponse{
		ID:                    agent.ID,
		OUID:                  agent.OUID,
		OUHandle:              agent.OUHandle,
		Type:                  agent.Type,
		Description:           agent.Description,
		LogoURL:               agent.LogoURL,
		Owner:                 agent.Owner,
		Attributes:            agent.Attributes,
		Status:                agent.Status,
		ValidUntil:            agent.ValidUntil,
		InboundAuthProfileReq: agent.InboundAuthProfileReq,
		InboundAuthConfig:     agent.InboundAuthConfig,
	}
	if cfg := oauthConfigOf(agent.InboundAuthConfig); cfg != nil {
		view.ClientID = cfg.ClientID
	}
	return view
}

// oauthConfigOf returns the OAuth settings among an agent's inbound configurations, or nil. The service
// refuses an agent with more than one, so a stored agent never has several.
func oauthConfigOf(configs []providers.InboundAuthConfigWithSecret) *providers.OAuthConfigWithSecret {
	cfg, svcErr := pickOAuthConfig(configs)
	if svcErr != nil {
		return nil
	}
	return cfg
}

// clientSecretOf returns the client secret a create or update result carries, which it does only when
// one was just issued.
func clientSecretOf(configs []providers.InboundAuthConfigWithSecret) string {
	if cfg := oauthConfigOf(configs); cfg != nil {
		return cfg.ClientSecret
	}
	return ""
}

// decodeAttributes reads an agent's attributes as a map, which is empty when it has none.
func decodeAttributes(raw json.RawMessage) map[string]any {
	attributes := map[string]any{}
	if len(raw) == 0 {
		return attributes
	}
	if err := json.Unmarshal(raw, &attributes); err != nil {
		return map[string]any{}
	}
	return attributes
}

// modesOf converts the ways of acting a tool input names to their typed form.
func modesOf(names []string) []actingMode {
	if names == nil {
		return nil
	}
	modes := make([]actingMode, 0, len(names))
	for _, name := range names {
		modes = append(modes, actingMode(name))
	}
	return modes
}

// statusOf returns the value of an agent's access status, or an empty string when it has none.
func statusOf(status *providers.AccessStatusReport) string {
	if status == nil {
		return ""
	}
	return string(status.Value)
}

// reachesOfflineUser reports whether a flow can ask a user who isn't there to approve: it finds the user
// from login_hint and sends them a link by email or SMS, as the CIBA templates do.
func reachesOfflineUser(flow *providers.CompleteFlowDefinition) bool {
	identifies, notifies := false, false
	for _, node := range flow.Nodes {
		if node.Executor == nil {
			continue
		}
		switch node.Executor.Name {
		case executormeta.ExecutorNameIdentifying:
			attribute, _ := node.Properties[loginHintAttributeProperty].(string)
			identifies = identifies || attribute != ""
		case executormeta.ExecutorNameEmailExecutor, executormeta.ExecutorNameSMSExecutor:
			notifies = true
		}
	}
	return identifies && notifies
}

// changedFields returns each field of after, in get_agent's terms, that differs from before, with its
// new value. A field after no longer has is null.
func changedFields(before, after agentMCPDetails) map[string]any {
	fields := func(details agentMCPDetails) map[string]any {
		out := map[string]any{}
		if raw, err := json.Marshal(details); err == nil {
			_ = json.Unmarshal(raw, &out)
		}
		return out
	}
	was, now := fields(before), fields(after)
	changed := map[string]any{}
	for name, value := range now {
		if !reflect.DeepEqual(was[name], value) {
			changed[name] = value
		}
	}
	for name := range was {
		if _, ok := now[name]; !ok {
			changed[name] = nil
		}
	}
	return changed
}
