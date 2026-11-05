// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	toolAgentFlowHandle = "agent-onboarding"
	toolAgentFlowID     = "agent-flow-id"
	toolUserFlowHandle  = "user-onboarding"
	toolUserFlowID      = "user-flow-id"
)

type FlowToolsTestSuite struct {
	suite.Suite
	service      *FlowExecServiceInterfaceMock
	flows        *FlowProviderMock
	serverConfig *serverConfigProviderMock
	session      *mcp.ClientSession
}

func TestFlowToolsTestSuite(t *testing.T) {
	suite.Run(t, new(FlowToolsTestSuite))
}

func (s *FlowToolsTestSuite) SetupTest() {
	s.connect(flowconfig.FlowSectionConfig{
		AgentOnboardingFlow: flowconfig.FlowTypeConfig{DefaultHandle: toolAgentFlowHandle},
		UserOnboardingFlow:  flowconfig.FlowTypeConfig{DefaultHandle: toolUserFlowHandle},
	})
}

// connect registers the tool over mocked services, with cfg as the server's flow configuration, and
// connects an in-memory MCP client, so every call crosses the SDK's schema validation as an agent's
// would.
func (s *FlowToolsTestSuite) connect(cfg flowconfig.FlowSectionConfig) {
	s.service = NewFlowExecServiceInterfaceMock(s.T())
	s.flows = NewFlowProviderMock(s.T())
	s.serverConfig = newServerConfigProviderMock(s.T())
	s.serverConfig.On("GetMergedConfig", mock.Anything, "flow").
		Return(cfg, (*tidcommon.ServiceError)(nil)).Maybe()
	s.flows.On("GetFlowByHandle", mock.Anything, toolAgentFlowHandle, providers.FlowTypeAdministration).
		Return(&providers.CompleteFlowDefinition{ID: toolAgentFlowID}, (*tidcommon.ServiceError)(nil)).Maybe()
	s.flows.On("GetFlowByHandle", mock.Anything, toolUserFlowHandle, providers.FlowTypeUserOnboarding).
		Return(&providers.CompleteFlowDefinition{ID: toolUserFlowID}, (*tidcommon.ServiceError)(nil)).Maybe()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	registerMCPTools(server, &flowTools{service: s.service, flows: s.flows, serverConfig: s.serverConfig})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	s.session, err = client.Connect(context.Background(), clientTransport, nil)
	s.Require().NoError(err)
}

func (s *FlowToolsTestSuite) call(args map[string]any) *mcp.CallToolResult {
	result, err := s.session.CallTool(context.Background(),
		&mcp.CallToolParams{Name: "execute_flow", Arguments: args})
	s.Require().NoError(err)
	return result
}

func (s *FlowToolsTestSuite) structured(result *mcp.CallToolResult) executeFlowMCPResponse {
	s.Require().False(result.IsError, "tool returned an error: %v", result.Content)
	raw, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	var out executeFlowMCPResponse
	s.Require().NoError(json.Unmarshal(raw, &out))
	return out
}

func (s *FlowToolsTestSuite) errorText(result *mcp.CallToolResult) string {
	s.Require().True(result.IsError)
	s.Require().NotEmpty(result.Content)
	text, ok := result.Content[0].(*mcp.TextContent)
	s.Require().True(ok)
	return text.Text
}

func (s *FlowToolsTestSuite) TestRegisterMCPTools() {
	result, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)
	s.Require().Len(result.Tools, 1)
	t := result.Tools[0]
	s.Equal("execute_flow", t.Name)
	s.NotContains(t.Description, "—")
	s.NotContains(t.Description, "--")
	s.False(t.Annotations.ReadOnlyHint)
	s.False(*t.Annotations.DestructiveHint)
	s.False(t.Annotations.IdempotentHint)
	s.False(*t.Annotations.OpenWorldHint)

	var schema struct {
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	raw, err := json.Marshal(t.InputSchema)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, &schema))
	s.ElementsMatch([]string{"agent_onboarding", "user_onboarding"}, schema.Properties["flow"].Enum)

	s.True(s.call(map[string]any{"flow": "authentication"}).IsError, "an unlisted flow is refused")
}

// Agent onboarding runs the configured administration flow by ID, with every input at once, and the
// credentials it ends with come back with the one-time notice.
func (s *FlowToolsTestSuite) TestAgentOnboarding_Completes() {
	inputs := map[string]string{"name": "inventory-bot", "ouId": "ou-1"}
	s.service.On("ExecuteByID", mock.Anything, toolAgentFlowID, "", true, "", inputs, "",
		[]SessionData(nil)).Return(&FlowStep{
		ExecutionID: "exec-1",
		Status:      providers.FlowStatusComplete,
		Data: FlowData{AdditionalData: map[string]string{
			common.DataAgentID: "agent-1", common.DataAgentClientID: "client-1",
			common.DataAgentClientSecret: "s3cret"}},
	}, (*tidcommon.ServiceError)(nil)).Once()

	out := s.structured(s.call(map[string]any{
		"flow": "agent_onboarding", "inputs": map[string]any{"name": "inventory-bot", "ouId": "ou-1"}}))

	s.Equal(flowStatusComplete, out.Status)
	s.Empty(out.ExecutionID, "a finished run has nothing to go on with")
	s.Equal("agent-1", out.Data[common.DataAgentID])
	s.Equal("s3cret", out.Data[common.DataAgentClientSecret])
	s.Contains(out.Notice, "only this once")
}

// A step that needs more returns the execution ID, what it asks for, the actions it offers, and the
// values it refused.
func (s *FlowToolsTestSuite) TestAgentOnboarding_NeedsInput() {
	s.service.On("ExecuteByID", mock.Anything, toolAgentFlowID, "", true, "", map[string]string(nil), "",
		[]SessionData(nil)).Return(&FlowStep{
		ExecutionID:    "exec-1",
		ChallengeToken: "token-1",
		Status:         providers.FlowStatusIncomplete,
		Error: &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "the selected unit isn't allowed"}},
		Data: FlowData{
			Inputs: []providers.Input{
				{Identifier: "name", Type: "TEXT_INPUT", Required: true},
				{Identifier: "modelProvider", Type: "SELECT", Options: []string{"openai", "anthropic"}},
				{Identifier: "function", Type: "SELECT"},
				{Identifier: "model", Type: "SELECT"},
			},
			Actions:     []common.Action{{Ref: "action_agent_details"}, {Ref: "action_agent_details"}},
			FieldErrors: []common.FieldError{{Identifier: "name", Message: "is taken"}},
			Meta: map[string]any{"components": []any{map[string]any{"type": "BLOCK", "components": []any{
				map[string]any{"ref": "function", "type": "SELECT", "options": []any{"assistant", "orchestrator"}},
				map[string]any{"ref": "model", "type": "SELECT",
					"options": []any{map[string]any{"value": "gpt", "label": "GPT"}}},
			}}}},
		},
	}, (*tidcommon.ServiceError)(nil)).Once()

	out := s.structured(s.call(map[string]any{"flow": "agent_onboarding"}))

	s.Equal(flowStatusNeedsInput, out.Status)
	s.Equal("exec-1", out.ExecutionID)
	s.Equal("token-1", out.ChallengeToken)
	s.Equal("the selected unit isn't allowed", out.Error)
	s.Equal([]flowInputMCPItem{
		{Identifier: "name", Type: "TEXT_INPUT", Required: true},
		{Identifier: "modelProvider", Type: "SELECT", Options: []string{"openai", "anthropic"}},
		{Identifier: "function", Type: "SELECT", Options: []string{"assistant", "orchestrator"}},
		{Identifier: "model", Type: "SELECT", Options: []string{"gpt"}},
	}, out.Inputs, "a dropdown's options come from the step's display when its input lists none")
	s.Equal([]string{"action_agent_details"}, out.Actions)
	s.Equal(&flowNextCallMCPItem{ExecutionID: "exec-1", ChallengeToken: "token-1", Action: "action_agent_details"},
		out.Next, "next is the call that goes on, with the only action filled in")
	s.Equal([]flowFieldErrorMCPItem{{Identifier: "name", Message: "is taken"}}, out.FieldErrors)
	s.Empty(out.Notice)
}

// User onboarding is not an administration flow, so it starts by type, after the configured flow is
// confirmed to exist.
func (s *FlowToolsTestSuite) TestUserOnboarding_StartsByType() {
	s.service.On("Execute", mock.Anything, "", "", string(providers.FlowTypeUserOnboarding), "", true,
		"action_invite_user", map[string]string{"email": "alice@example.com"}, "", "", "", []SessionData(nil)).
		Return(&FlowStep{
			ExecutionID: "exec-2",
			Status:      providers.FlowStatusIncomplete,
			Data: FlowData{Actions: []common.Action{
				{Ref: "action_send_email_invite"}, {Ref: "action_share_manually"}}},
		}, (*tidcommon.ServiceError)(nil)).Once()

	out := s.structured(s.call(map[string]any{"flow": "user_onboarding", "action": "action_invite_user",
		"inputs": map[string]any{"email": "alice@example.com"}}))

	s.Equal(flowStatusNeedsInput, out.Status)
	s.Equal([]string{"action_send_email_invite", "action_share_manually"}, out.Actions)
}

// A paused run of an onboarding flow goes on by its execution ID, and a step that only shows values is
// reported as info.
func (s *FlowToolsTestSuite) TestContinue_InfoStep() {
	s.service.On("GetExecutionPosition", mock.Anything, "exec-2").
		Return(toolUserFlowID, "prompt_invite_method", (*tidcommon.ServiceError)(nil)).Once()
	s.service.On("ExecuteByID", mock.Anything, "", "exec-2", true, "action_share_manually",
		map[string]string(nil), "token-1", []SessionData(nil)).Return(&FlowStep{
		ExecutionID:    "exec-2",
		ChallengeToken: "token-2",
		Status:         providers.FlowStatusIncomplete,
		Data:           FlowData{AdditionalData: map[string]string{common.DataInviteLink: "https://invite"}},
	}, (*tidcommon.ServiceError)(nil)).Once()

	out := s.structured(s.call(map[string]any{"executionId": "exec-2", "challengeToken": "token-1",
		"action": "action_share_manually"}))

	s.Equal(flowStatusInfo, out.Status)
	s.Equal("token-2", out.ChallengeToken, "the next call needs the rotated token")
	s.Equal("exec-2", out.ExecutionID)
	s.Equal("https://invite", out.Data[common.DataInviteLink])
	s.Contains(out.Notice, "asks for nothing")
	s.Contains(out.Notice, "inviteLink lets whoever opens it", "an invite link comes with a warning")
}

func (s *FlowToolsTestSuite) TestRefusals() {
	stepErr := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "FEX-1",
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "the owner must be a user"}}
	cases := []struct {
		name     string
		cfg      *flowconfig.FlowSectionConfig
		prepare  func()
		args     map[string]any
		expected string
	}{
		{
			name:     "neither flow nor executionId",
			args:     map[string]any{},
			expected: "give flow to start one",
		},
		{
			name:     "both flow and executionId",
			args:     map[string]any{"flow": "agent_onboarding", "executionId": "exec-1"},
			expected: "give flow to start one",
		},
		{
			name:     "flow not configured",
			cfg:      &flowconfig.FlowSectionConfig{},
			args:     map[string]any{"flow": "agent_onboarding"},
			expected: "set flow.agentOnboardingFlow.defaultHandle",
		},
		{
			name: "configured flow missing",
			cfg: &flowconfig.FlowSectionConfig{
				UserOnboardingFlow: flowconfig.FlowTypeConfig{DefaultHandle: "gone"}},
			prepare: func() {
				s.flows.On("GetFlowByHandle", mock.Anything, "gone", providers.FlowTypeUserOnboarding).
					Return((*providers.CompleteFlowDefinition)(nil), &tidcommon.ServiceError{
						Type: tidcommon.ClientErrorType, Code: "FLM-1003",
						ErrorDescription: tidcommon.I18nMessage{DefaultValue: "flow not found"}}).Once()
			},
			args:     map[string]any{"flow": "user_onboarding"},
			expected: `the configured user_onboarding flow "gone" can't be used: flow not found`,
		},
		{
			name: "execution of another flow",
			prepare: func() {
				s.service.On("GetExecutionPosition", mock.Anything, "sign-in").
					Return("auth-flow-id", "prompt_credentials", (*tidcommon.ServiceError)(nil)).Once()
			},
			args:     map[string]any{"executionId": "sign-in"},
			expected: "didn't start",
		},
		{
			name: "finished or expired execution",
			prepare: func() {
				s.service.On("GetExecutionPosition", mock.Anything, "old").
					Return("", "", &ErrorInvalidExecutionID).Once()
			},
			args:     map[string]any{"executionId": "old"},
			expected: "finished or expired",
		},
		{
			name: "a step refuses",
			prepare: func() {
				s.service.On("ExecuteByID", mock.Anything, toolAgentFlowID, "", true, "", mock.Anything, "",
					mock.Anything).Return(&FlowStep{Status: providers.FlowStatusError, Error: stepErr},
					(*tidcommon.ServiceError)(nil)).Once()
			},
			args:     map[string]any{"flow": "agent_onboarding"},
			expected: "the flow stopped: the owner must be a user",
		},
		{
			name: "the service refuses",
			prepare: func() {
				s.service.On("ExecuteByID", mock.Anything, toolAgentFlowID, "", true, "", mock.Anything, "",
					mock.Anything).Return((*FlowStep)(nil), &ErrorAdministrationPermissionRequired).Once()
			},
			args:     map[string]any{"flow": "agent_onboarding"},
			expected: "failed to execute flow",
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			if tc.cfg != nil {
				s.connect(*tc.cfg)
			} else {
				s.SetupTest()
			}
			if tc.prepare != nil {
				tc.prepare()
			}
			s.Contains(s.errorText(s.call(tc.args)), tc.expected)
		})
	}
}

// A call that goes on with a paused run but leaves out the action takes the step's only action when the
// run is still paused at the same step, instead of asking for it again.
func (s *FlowToolsTestSuite) TestContinue_TakesTheOnlyActionAtTheSameStep() {
	s.service.On("GetExecutionPosition", mock.Anything, "exec-1").
		Return(toolAgentFlowID, "prompt_owner", (*tidcommon.ServiceError)(nil)).Twice()
	s.service.On("ExecuteByID", mock.Anything, "", "exec-1", true, "", map[string]string(nil), "token-1",
		[]SessionData(nil)).Return(&FlowStep{
		ExecutionID: "exec-1", ChallengeToken: "token-2", Status: providers.FlowStatusIncomplete,
		Data: FlowData{Inputs: []providers.Input{{Identifier: "owner", Type: "USER_SELECT"}},
			Actions: []common.Action{{Ref: "action_agent_owner"}}},
	}, (*tidcommon.ServiceError)(nil)).Once()
	s.service.On("ExecuteByID", mock.Anything, "", "exec-1", true, "action_agent_owner", map[string]string(nil),
		"token-2", []SessionData(nil)).Return(&FlowStep{
		ExecutionID: "exec-1", ChallengeToken: "token-3", Status: providers.FlowStatusIncomplete,
		Data: FlowData{Inputs: []providers.Input{{Identifier: "model", Type: "TEXT_INPUT"}},
			Actions: []common.Action{{Ref: "action_agent_details"}}},
	}, (*tidcommon.ServiceError)(nil)).Once()

	out := s.structured(s.call(map[string]any{"executionId": "exec-1", "challengeToken": "token-1"}))

	s.Equal([]string{"action_agent_details"}, out.Actions, "the run moved past the owner step")
	s.Equal("token-3", out.Next.ChallengeToken)
}

// When the run moved on to another step, the missing action isn't filled in, so a new step's inputs are
// never skipped.
func (s *FlowToolsTestSuite) TestContinue_LeavesANewStepAlone() {
	s.service.On("GetExecutionPosition", mock.Anything, "exec-1").
		Return(toolAgentFlowID, "prompt_owner", (*tidcommon.ServiceError)(nil)).Once()
	s.service.On("GetExecutionPosition", mock.Anything, "exec-1").
		Return(toolAgentFlowID, "prompt_agent_details", (*tidcommon.ServiceError)(nil)).Once()
	s.service.On("ExecuteByID", mock.Anything, "", "exec-1", true, "", map[string]string{"owner": "u1"},
		"token-1", []SessionData(nil)).Return(&FlowStep{
		ExecutionID: "exec-1", ChallengeToken: "token-2", Status: providers.FlowStatusIncomplete,
		Data: FlowData{Inputs: []providers.Input{{Identifier: "model", Type: "TEXT_INPUT"}},
			Actions: []common.Action{{Ref: "action_agent_details"}}},
	}, (*tidcommon.ServiceError)(nil)).Once()

	out := s.structured(s.call(map[string]any{"executionId": "exec-1", "challengeToken": "token-1",
		"inputs": map[string]any{"owner": "u1"}}))

	s.Equal(flowStatusNeedsInput, out.Status)
	s.Equal([]string{"action_agent_details"}, out.Actions)
}
