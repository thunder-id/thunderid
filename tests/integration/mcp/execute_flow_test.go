// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// flowActions returns the actions an execute_flow result offers.
func flowActions(out map[string]any) []string {
	raw, _ := out["actions"].([]any)
	actions := make([]string, 0, len(raw))
	for _, action := range raw {
		ref, _ := action.(string)
		actions = append(actions, ref)
	}
	return actions
}

// The user onboarding flow runs through execute_flow step by step: it asks whether to create the user
// or invite them, then how to deliver the invite, and a link shared by hand comes back as a step that
// only shows it, since the invited user finishes the flow.
func (ts *MCPTestSuite) TestExecuteFlow_UserOnboardingInvite() {
	token := ts.mcpScopedAdminToken()
	handle := fmt.Sprintf("mcp-invitee-%d", time.Now().UnixNano())
	typeID, err := testutils.CreateUserType(testutils.UserType{
		Handle: handle, DisplayName: handle, OUID: ts.ouID,
		Schema: map[string]interface{}{
			"email": map[string]interface{}{"type": "string", "unique": true},
		},
	})
	ts.Require().NoError(err, "failed to create the user type")
	defer func() { _ = testutils.DeleteUserType(typeID) }()

	out := ts.executeFlow(token, map[string]any{"flow": "user_onboarding",
		"inputs": map[string]any{"userType": handle, "ouId": ts.ouID}})
	ts.Require().Equalf("needs_input", out["status"], "the flow must ask how to onboard: %v", out)
	ts.Subset(flowActions(out), []string{"action_create_user_now", "action_invite_user"})
	executionID := out["executionId"]
	ts.Require().NotEmpty(executionID)

	out = ts.executeFlow(token, map[string]any{"executionId": executionID,
		"challengeToken": out["challengeToken"], "action": "action_invite_user",
		"inputs": map[string]any{"email": fmt.Sprintf("%s@example.com", handle)}})
	ts.Require().Equalf("needs_input", out["status"], "the flow must ask how to deliver the invite: %v", out)
	ts.Subset(flowActions(out), []string{"action_send_email_invite", "action_share_manually"})

	out = ts.executeFlow(token, map[string]any{"executionId": executionID,
		"challengeToken": out["challengeToken"], "action": "action_share_manually"})
	ts.Equalf("info", out["status"], "the invite link is shown: %v", out)
	data, _ := out["data"].(map[string]any)
	ts.NotEmpty(data["inviteLink"], out)
	ts.NotEmpty(out["notice"])
}

// A value a step refuses comes back with the reason, so the agent can ask the administrator for another:
// the agent onboarding flow places agents only under the agent type's own unit.
func (ts *MCPTestSuite) TestExecuteFlow_RefusedValueComesWithTheReason() {
	out := ts.executeFlow(ts.mcpScopedAdminToken(), map[string]any{"flow": "agent_onboarding",
		"inputs": map[string]any{"name": fmt.Sprintf("mcp-misplaced-%d", time.Now().UnixNano()), "agentType": "default",
			"ouId": ts.ouID}})
	ts.Equalf("needs_input", out["status"], "the flow must ask for another unit: %v", out)
	ts.NotEmptyf(out["error"], "the step must say why it asks again: %v", out)
	ts.Contains(fmt.Sprint(out["inputs"]), "ouId")
}

// An execution ID that no paused run has is refused with a hint to start again.
func (ts *MCPTestSuite) TestExecuteFlow_UnknownExecutionRefused() {
	result, raw := ts.callMCPTool(ts.mcpScopedAdminToken(), "execute_flow",
		map[string]any{"executionId": "01900000-0000-7000-8000-00000000dead"})
	ts.Truef(result.Result.IsError, "an unknown execution must be refused: %s", raw)
	ts.Contains(raw, "finished or expired")
}
