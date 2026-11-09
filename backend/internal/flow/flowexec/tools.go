// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// onboardMCPInput is the input of the onboard_agents and onboard_users tools.
//
//nolint:lll
type onboardMCPInput struct {
	ExecutionID    string            `json:"executionId,omitempty" jsonschema:"ID of a run that paused, from an earlier result of this tool, to go on with it. Leave it out to start a new run."`
	ChallengeToken string            `json:"challengeToken,omitempty" jsonschema:"The challengeToken from the result that paused, when it had one. It changes on every step, so always send the latest."`
	Action         string            `json:"action,omitempty" jsonschema:"The action to take, one of the actions the paused step returned. Send it every time you go on with a paused run, even when the step offers only one."`
	Inputs         map[string]string `json:"inputs,omitempty" jsonschema:"Values for the flow's inputs, by identifier, such as name or ouId. Every value you already know can be given at once, and each step takes the ones it asks for."`
}

// flowInputMCPItem is one input a paused step asks for.
type flowInputMCPItem struct {
	Identifier string   `json:"identifier"`
	Type       string   `json:"type"`
	Required   bool     `json:"required"`
	Options    []string `json:"options,omitempty"`
}

// flowFieldErrorMCPItem is one input value a step refused.
type flowFieldErrorMCPItem struct {
	Identifier string `json:"identifier"`
	Message    string `json:"message"`
}

// onboardMCPResponse is the onboard_agents and onboard_users output.
type onboardMCPResponse struct {
	Status         flowRunStatus           `json:"status"`
	ExecutionID    string                  `json:"executionId,omitempty"`
	ChallengeToken string                  `json:"challengeToken,omitempty"`
	Error          string                  `json:"error,omitempty"`
	Inputs         []flowInputMCPItem      `json:"inputs,omitempty"`
	Actions        []string                `json:"actions,omitempty"`
	FieldErrors    []flowFieldErrorMCPItem `json:"fieldErrors,omitempty"`
	Data           map[string]string       `json:"data,omitempty"`
	Notice         string                  `json:"notice,omitempty"`
	Next           *flowNextCallMCPItem    `json:"next,omitempty"`
}

// flowNextCallMCPItem is the call that goes on with a paused run, to the tool that started it. The action
// is filled in when the step offers only one; the caller adds the inputs the step asks for.
type flowNextCallMCPItem struct {
	ExecutionID    string `json:"executionId"`
	ChallengeToken string `json:"challengeToken,omitempty"`
	Action         string `json:"action,omitempty"`
}

// flowTools carries the services the flow MCP tools call.
type flowTools struct {
	service      FlowExecServiceInterface
	flows        providers.FlowProvider
	serverConfig serverConfigProvider
}

// stepsGuide says how a client drives an onboarding flow, for both onboarding tools.
const stepsGuide = `The administrator configures the flow, so its steps and inputs vary. Start with every ` +
	`value you already know in inputs. When a step needs more, the result has status needs_input, the inputs ` +
	`it asks for, the actions it offers, in error why a value was refused, and in next the call that goes on: ` +
	`ask the administrator for what's missing, then send next with those inputs, choosing an action when next ` +
	`has none. status info means a step shows the values in data and asks for nothing, and status complete ` +
	`means the flow finished.`

// registerMCPTools registers the onboarding MCP tools with the server, one for each onboarding flow.
func registerMCPTools(server *mcp.Server, tools *flowTools) {
	mcp.AddTool(server, &mcp.Tool{
		Name: string(toolOnboardAgents),
		Description: `Create (onboard) an agent, such as "add an agent for the support team" or "register our ` +
			`inventory bot". This is the tool for any request to create, add or register an agent on this ` +
			`server: it runs the agent onboarding flow the server uses, the way the Console does, and returns ` +
			`the agent's agentId, clientId and clientSecret. The clientSecret is shown only once.

` + stepsGuide + ` An agent goes under the unit of its agent type, from list_agent_types.

Change an agent's credential or the ways it acts for users afterwards with update_agent. To create a user, ` +
			`use onboard_users.`,
		InputSchema: tool.GenerateSchema[onboardMCPInput](),
		Annotations: &mcp.ToolAnnotations{
			Title:           "Onboard Agents",
			DestructiveHint: new(false),
			IdempotentHint:  false,
			OpenWorldHint:   new(false),
		},
	}, tools.onboard(flowAgentOnboarding))

	mcp.AddTool(server, &mcp.Tool{
		Name: string(toolOnboardUsers),
		Description: `Create (onboard) a user, or invite one, such as "add Alice as a user" or "invite ` +
			`alice@example.com". This is the tool for any request to create, add or invite a user on this ` +
			`server: it runs the user onboarding flow the server uses, the way the Console does, which creates ` +
			`the user directly or invites them to set up their own account.

` + stepsGuide + ` Use list_organization_units for an ouId.

To create an agent, use onboard_agents.`,
		InputSchema: tool.GenerateSchema[onboardMCPInput](),
		Annotations: &mcp.ToolAnnotations{
			Title:           "Onboard Users",
			DestructiveHint: new(false),
			IdempotentHint:  false,
			OpenWorldHint:   new(false),
		},
	}, tools.onboard(flowUserOnboarding))
}

// onboard returns the handler of the onboarding tool for purpose. It starts that purpose's flow, or goes on
// with a paused run of it.
func (t *flowTools) onboard(purpose onboardingFlow) func(context.Context, *mcp.CallToolRequest,
	onboardMCPInput) (*mcp.CallToolResult, *onboardMCPResponse, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input onboardMCPInput) (
		*mcp.CallToolResult, *onboardMCPResponse, error) {
		var step *FlowStep
		var svcErr *tidcommon.ServiceError
		if input.ExecutionID == "" {
			flowID, err := t.onboardingFlowID(ctx, purpose)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to start onboarding: %w", err)
			}
			// The user onboarding flow is not an administration flow, so it starts by type, which runs the
			// same configured flow.
			if purpose == flowAgentOnboarding {
				step, svcErr = t.service.ExecuteByID(ctx, flowID, "", true, input.Action, input.Inputs, "", nil)
			} else {
				step, svcErr = t.service.Execute(ctx, "", "", string(providers.FlowTypeUserOnboarding), "", true,
					input.Action, input.Inputs, "", "", "", nil)
			}
		} else {
			pausedAt, err := t.checkOnboardingExecution(ctx, input.ExecutionID, purpose)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to go on with onboarding: %w", err)
			}
			step, svcErr = t.service.ExecuteByID(ctx, "", input.ExecutionID, true, input.Action, input.Inputs,
				input.ChallengeToken, nil)
			// A step goes on only when an action is sent. A call that left it out and is still paused at the
			// same step with a single action takes that action, so the run doesn't ask for the same step
			// again.
			if svcErr == nil && input.Action == "" {
				if action := t.onlyActionAtSameStep(ctx, input.ExecutionID, pausedAt, step); action != "" {
					step, svcErr = t.service.ExecuteByID(ctx, "", input.ExecutionID, true, action, input.Inputs,
						step.ChallengeToken, nil)
				}
			}
		}
		if svcErr != nil {
			return nil, nil, fmt.Errorf("failed to run onboarding: %s", svcErr.ErrorDescription)
		}
		if step.Status == providers.FlowStatusError {
			if step.Error != nil {
				return nil, nil, fmt.Errorf("the flow stopped: %s", step.Error.ErrorDescription)
			}
			return nil, nil, errors.New("the flow stopped with an error")
		}
		return nil, describeFlowStep(step), nil
	}
}

// onboardingFlowID returns the ID of the flow the server configuration names for the given purpose.
func (t *flowTools) onboardingFlowID(ctx context.Context, purpose onboardingFlow) (string, error) {
	cfg := readFlowSectionConfig(ctx, t.serverConfig)
	key, handle, flowType := "agentOnboardingFlow", cfg.AgentOnboardingFlow.DefaultHandle,
		providers.FlowTypeAdministration
	if purpose == flowUserOnboarding {
		key, handle, flowType = "userOnboardingFlow", cfg.UserOnboardingFlow.DefaultHandle,
			providers.FlowTypeUserOnboarding
	}
	if handle == "" {
		return "", fmt.Errorf("no %s flow is configured; set flow.%s.defaultHandle in the server configuration",
			purpose, key)
	}
	// Who may run an onboarding flow is decided by the flow and the services it calls, not by flow:view,
	// so the configured flow is read with the server's authority, as the flow engine reads the flows it
	// runs. The flow itself still runs as the caller.
	flow, svcErr := t.flows.GetFlowByHandle(security.WithRuntimeContext(ctx), handle, flowType)
	if svcErr != nil {
		return "", fmt.Errorf("the configured %s flow %q can't be used: %s", purpose, handle,
			svcErr.ErrorDescription)
	}
	return flow.ID, nil
}

// checkOnboardingExecution refuses an execution that doesn't run purpose's flow, so an execution ID from
// elsewhere, such as someone's sign-in or the other onboarding tool's run, can't be driven through this
// tool. It returns the node the run is paused at.
func (t *flowTools) checkOnboardingExecution(ctx context.Context, executionID string, purpose onboardingFlow) (
	string, error) {
	flowID, nodeID, svcErr := t.service.GetExecutionPosition(ctx, executionID)
	if svcErr != nil {
		if svcErr.Code == ErrorInvalidExecutionID.Code {
			return "", errors.New("no paused run has this executionId; it finished or expired, so start the " +
				"flow again")
		}
		return "", fmt.Errorf("%s", svcErr.ErrorDescription)
	}
	if id, err := t.onboardingFlowID(ctx, purpose); err == nil && id == flowID {
		return nodeID, nil
	}
	return "", fmt.Errorf("this executionId belongs to a run %s didn't start", toolNameOf(purpose))
}

// onlyActionAtSameStep returns the step's one action when the run is still paused at the node it was
// paused at before the call, and nothing otherwise.
func (t *flowTools) onlyActionAtSameStep(ctx context.Context, executionID, pausedAt string, step *FlowStep) string {
	if pausedAt == "" || step.Status != providers.FlowStatusIncomplete {
		return ""
	}
	actions := stepActions(step)
	if len(actions) != 1 {
		return ""
	}
	_, nodeID, svcErr := t.service.GetExecutionPosition(ctx, executionID)
	if svcErr != nil || nodeID != pausedAt {
		return ""
	}
	return actions[0]
}

// stepActions lists a step's distinct action refs.
func stepActions(step *FlowStep) []string {
	actions := []string{}
	for _, action := range step.Data.Actions {
		if action.Ref != "" && !slices.Contains(actions, action.Ref) {
			actions = append(actions, action.Ref)
		}
	}
	return actions
}

// describeFlowStep turns a flow step into an onboarding tool result.
func describeFlowStep(step *FlowStep) *onboardMCPResponse {
	result := &onboardMCPResponse{Data: step.Data.AdditionalData}
	notices := []string{}
	if _, ok := step.Data.AdditionalData[common.DataAgentClientSecret]; ok {
		notices = append(notices, secretNotice)
	}
	if _, ok := step.Data.AdditionalData[common.DataInviteLink]; ok {
		notices = append(notices, inviteNotice)
	}
	result.Notice = strings.Join(notices, " ")
	if step.Status == providers.FlowStatusComplete {
		result.Status = flowStatusComplete
		return result
	}

	result.ExecutionID = step.ExecutionID
	result.ChallengeToken = step.ChallengeToken
	if step.Error != nil {
		result.Error = step.Error.ErrorDescription.String()
	}
	// The display metadata comes from the flow's definition in whatever shape it was decoded into, so it
	// is read as plain JSON.
	var meta any
	if raw, err := json.Marshal(step.Data.Meta); err == nil {
		if unmarshalErr := json.Unmarshal(raw, &meta); unmarshalErr != nil {
			meta = nil
		}
	}
	for _, input := range step.Data.Inputs {
		options := input.Options
		if len(options) == 0 {
			options = metaOptions(meta, input.Identifier)
		}
		result.Inputs = append(result.Inputs, flowInputMCPItem{
			Identifier: input.Identifier, Type: input.Type, Required: input.Required, Options: options})
	}
	result.Actions = stepActions(step)
	result.Next = &flowNextCallMCPItem{ExecutionID: step.ExecutionID, ChallengeToken: step.ChallengeToken}
	if len(result.Actions) == 1 {
		result.Next.Action = result.Actions[0]
	}
	for _, fieldErr := range step.Data.FieldErrors {
		result.FieldErrors = append(result.FieldErrors, flowFieldErrorMCPItem(fieldErr))
	}
	if len(result.Inputs) == 0 && len(result.Actions) == 0 {
		result.Status = flowStatusInfo
		result.Notice = strings.TrimSpace(result.Notice + " " + infoNotice)
		return result
	}
	result.Status = flowStatusNeedsInput
	return result
}

// metaOptions returns the values a step's display offers for an input, such as a dropdown's choices. They
// sit on the display component whose ref is the input's identifier, as plain values or as {value, label}.
func metaOptions(meta any, identifier string) []string {
	switch node := meta.(type) {
	case map[string]any:
		if ref, _ := node["ref"].(string); ref == identifier {
			if raw, ok := node["options"].([]any); ok {
				options := make([]string, 0, len(raw))
				for _, option := range raw {
					switch value := option.(type) {
					case string:
						options = append(options, value)
					case map[string]any:
						if v, ok := value["value"].(string); ok {
							options = append(options, v)
						}
					}
				}
				return options
			}
		}
		for _, child := range node {
			if options := metaOptions(child, identifier); len(options) > 0 {
				return options
			}
		}
	case []any:
		for _, child := range node {
			if options := metaOptions(child, identifier); len(options) > 0 {
				return options
			}
		}
	}
	return nil
}
