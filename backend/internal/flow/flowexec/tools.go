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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// executeFlowMCPInput is the input of the execute_flow tool.
//
//nolint:lll
type executeFlowMCPInput struct {
	Flow           string            `json:"flow,omitempty" jsonschema:"The flow to start. agent_onboarding creates an agent. user_onboarding creates a user or invites one. Give flow or executionId."`
	ExecutionID    string            `json:"executionId,omitempty" jsonschema:"ID of a run that paused, from an earlier execute_flow result, to go on with it. Give flow or executionId."`
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

// executeFlowMCPResponse is the execute_flow output.
type executeFlowMCPResponse struct {
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

// flowNextCallMCPItem is the execute_flow call that goes on with a paused run. The action is filled in
// when the step offers only one; the caller adds the inputs the step asks for.
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

// registerMCPTools registers the flow execution MCP tools with the server.
func registerMCPTools(server *mcp.Server, tools *flowTools) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "execute_flow",
		Description: `Run one of the flows this server uses to add accounts, the way the Console does. ` +
			`Pick the flow by what the administrator asks for:
- agent_onboarding: "add an agent for the support team", "register our inventory bot". It creates the ` +
			`agent and returns its agentId, clientId and clientSecret.
- user_onboarding: "add Alice as a user", "invite alice@example.com". It creates the user directly, or ` +
			`invites them to set up their own account.

The administrator configures each flow, so its steps and inputs vary. Start with flow and every value you ` +
			`already know in inputs. When a step needs more, the result has status needs_input, the inputs it ` +
			`asks for, the actions it offers, in error why a value was refused, and in next the call that goes ` +
			`on: ask the administrator for what's missing, then send next with those inputs, choosing an action ` +
			`when next has none. Use ` +
			`list_organization_units for an ouId; an agent goes under the unit of its agent type, from ` +
			`list_agent_types. status info means a step shows the values in data and asks for nothing, and ` +
			`status complete means the flow finished. A clientSecret in data is shown only once.

Change an agent's credential or the ways it acts for users afterwards with update_agent.`,
		InputSchema: tool.GenerateSchema[executeFlowMCPInput](
			tool.WithEnum("", "flow", []string{string(flowAgentOnboarding), string(flowUserOnboarding)})),
		Annotations: &mcp.ToolAnnotations{
			Title:           "Execute Flow",
			DestructiveHint: new(false),
			IdempotentHint:  false,
			OpenWorldHint:   new(false),
		},
	}, tools.executeFlow)
}

// executeFlow handles the execute_flow tool call.
func (t *flowTools) executeFlow(ctx context.Context, _ *mcp.CallToolRequest, input executeFlowMCPInput) (
	*mcp.CallToolResult, *executeFlowMCPResponse, error) {
	if (input.Flow == "") == (input.ExecutionID == "") {
		return nil, nil, errors.New("failed to execute flow: give flow to start one, or executionId to go on " +
			"with a paused one")
	}

	var step *FlowStep
	var svcErr *tidcommon.ServiceError
	if input.Flow != "" {
		flowID, err := t.onboardingFlowID(ctx, onboardingFlow(input.Flow))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to execute flow: %w", err)
		}
		// The user onboarding flow is not an administration flow, so it starts by type, which runs the
		// same configured flow.
		if onboardingFlow(input.Flow) == flowAgentOnboarding {
			step, svcErr = t.service.ExecuteByID(ctx, flowID, "", true, input.Action, input.Inputs, "", nil)
		} else {
			step, svcErr = t.service.Execute(ctx, "", "", string(providers.FlowTypeUserOnboarding), "", true,
				input.Action, input.Inputs, "", "", "", nil)
		}
	} else {
		pausedAt, err := t.checkOnboardingExecution(ctx, input.ExecutionID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to execute flow: %w", err)
		}
		step, svcErr = t.service.ExecuteByID(ctx, "", input.ExecutionID, true, input.Action, input.Inputs,
			input.ChallengeToken, nil)
		// A step goes on only when an action is sent. A call that left it out and is still paused at the
		// same step with a single action takes that action, so the run doesn't ask for the same step again.
		if svcErr == nil && input.Action == "" {
			if action := t.onlyActionAtSameStep(ctx, input.ExecutionID, pausedAt, step); action != "" {
				step, svcErr = t.service.ExecuteByID(ctx, "", input.ExecutionID, true, action, input.Inputs,
					step.ChallengeToken, nil)
			}
		}
	}
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to execute flow: %s", svcErr.ErrorDescription)
	}
	if step.Status == providers.FlowStatusError {
		if step.Error != nil {
			return nil, nil, fmt.Errorf("the flow stopped: %s", step.Error.ErrorDescription)
		}
		return nil, nil, errors.New("the flow stopped with an error")
	}
	return nil, describeFlowStep(step), nil
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
	flow, svcErr := t.flows.GetFlowByHandle(ctx, handle, flowType)
	if svcErr != nil {
		return "", fmt.Errorf("the configured %s flow %q can't be used: %s", purpose, handle,
			svcErr.ErrorDescription)
	}
	return flow.ID, nil
}

// checkOnboardingExecution refuses an execution that doesn't run one of the flows execute_flow starts,
// so an execution ID from elsewhere, such as someone's sign-in, can't be driven through this tool.
// It returns the node the run is paused at.
func (t *flowTools) checkOnboardingExecution(ctx context.Context, executionID string) (string, error) {
	flowID, nodeID, svcErr := t.service.GetExecutionPosition(ctx, executionID)
	if svcErr != nil {
		if svcErr.Code == ErrorInvalidExecutionID.Code {
			return "", errors.New("no paused run has this executionId; it finished or expired, so start the " +
				"flow again")
		}
		return "", fmt.Errorf("%s", svcErr.ErrorDescription)
	}
	for _, purpose := range onboardingFlows {
		if id, err := t.onboardingFlowID(ctx, purpose); err == nil && id == flowID {
			return nodeID, nil
		}
	}
	return "", errors.New("this executionId belongs to a run execute_flow didn't start")
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

// describeFlowStep turns a flow step into the execute_flow result.
func describeFlowStep(step *FlowStep) *executeFlowMCPResponse {
	result := &executeFlowMCPResponse{Data: step.Data.AdditionalData}
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
