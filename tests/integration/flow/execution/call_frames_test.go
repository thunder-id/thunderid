// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const callFramesPassword = "SecurePass123!"

// callFramesUserType is the minimum a callee needs to authenticate someone, so the call can return
// to its caller rather than failing.
var callFramesUserType = testutils.UserType{
	Handle:      "call-frames-person",
	DisplayName: "Call Frames Person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{
			"type": "string",
		},
		"password": map[string]interface{}{
			"type":       "string",
			"credential": true,
		},
	},
}

// callerNodes builds a flow whose first step calls another flow. onFailure is optional: without it a
// failing callee ends the caller, with it the caller carries on at that node.
func callerNodes(calleeFlowID, onFailure string) []map[string]interface{} {
	callNode := map[string]interface{}{
		"id":        "call_callee",
		"type":      "CALL",
		"flow":      map[string]interface{}{"ref": calleeFlowID},
		"onSuccess": "prompt_returned",
	}
	if onFailure != "" {
		callNode["onFailure"] = onFailure
	}

	nodes := []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "call_callee"},
		callNode,
		markerPromptNode("prompt_returned", "returned_marker", "auth_assert"),
	}

	// Flow validation rejects a node nothing can reach, so the failure target exists only in the
	// variant whose call node points at it.
	if onFailure != "" {
		nodes = append(nodes, markerPromptNode(onFailure, "recovered_marker", "auth_assert"))
	}

	return append(nodes,
		map[string]interface{}{
			"id":        "auth_assert",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": "end",
		},
		map[string]interface{}{"id": "end", "type": "END"},
	)
}

// markerPromptNode builds a prompt whose single input names the branch the flow reached, so a test
// can tell where the caller resumed from the response alone.
func markerPromptNode(id, marker, nextNode string) map[string]interface{} {
	return map[string]interface{}{
		"id":   id,
		"type": "PROMPT",
		"prompts": []map[string]interface{}{
			{
				"inputs": []map[string]interface{}{
					{
						"ref":        marker,
						"identifier": marker,
						"type":       "TEXT_INPUT",
						"required":   true,
					},
				},
				"action": map[string]interface{}{
					"ref":      "action_" + marker,
					"nextNode": nextNode,
				},
			},
		},
	}
}

type CallFramesTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig

	ouID       string
	userTypeID string
	userID     string
	username   string

	pausingAppID     string
	failingAppID     string
	recoveringAppID  string
	createdAppIDList []string
}

func TestCallFramesTestSuite(t *testing.T) {
	suite.Run(t, new(CallFramesTestSuite))
}

func (ts *CallFramesTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "call_frames_test_ou",
		Name:        "Call Frames Test OU",
		Description: "Organization unit for call frame testing",
		Parent:      nil,
	})
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	userType := callFramesUserType
	userType.OUID = ouID
	ts.userTypeID, err = testutils.CreateUserType(userType)
	ts.Require().NoError(err, "Failed to create test user type")

	ts.username = common.GenerateUniqueUsername("call_frames")
	ts.userID, err = testutils.CreateUser(testutils.User{
		Type: callFramesUserType.Handle,
		OUID: ouID,
		Attributes: json.RawMessage(`{
			"username": "` + ts.username + `",
			"password": "` + callFramesPassword + `"
		}`),
	})
	ts.Require().NoError(err, "Failed to create test user")

	// A callee that pauses for credentials and then authenticates, so the call both suspends inside
	// the callee and later returns to its caller.
	pausingCalleeID := ts.createFlow("Call Frames Pausing Callee", "auth_flow_call_frames_callee", []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_001",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_002",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_001",
						"nextNode": "credentials_auth",
					},
				},
			},
		},
		{
			"id":           "credentials_auth",
			"type":         "TASK_EXECUTION",
			"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor"},
			"onSuccess":    "callee_assert",
			"onIncomplete": "prompt_credentials",
		},
		{
			"id":        "callee_assert",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": "end",
		},
		{"id": "end", "type": "END"},
	})

	// A callee that fails at its first node, because nothing has authenticated a user for it to
	// assert.
	failingCalleeID := ts.createFlow("Call Frames Failing Callee", "auth_flow_call_frames_failing_callee",
		[]map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "callee_assert"},
			{
				"id":        "callee_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		})

	pausingCallerID := ts.createFlow("Call Frames Pausing Caller", "auth_flow_call_frames_caller",
		callerNodes(pausingCalleeID, ""))
	failingCallerID := ts.createFlow("Call Frames Failing Caller", "auth_flow_call_frames_failing_caller",
		callerNodes(failingCalleeID, ""))
	recoveringCallerID := ts.createFlow("Call Frames Recovering Caller",
		"auth_flow_call_frames_recovering_caller", callerNodes(failingCalleeID, "prompt_recovered"))

	ts.pausingAppID = ts.createApp("Call Frames Pausing App", "call_frames_pausing_client", pausingCallerID)
	ts.failingAppID = ts.createApp("Call Frames Failing App", "call_frames_failing_client", failingCallerID)
	ts.recoveringAppID = ts.createApp("Call Frames Recovering App", "call_frames_recovering_client",
		recoveringCallerID)
}

func (ts *CallFramesTestSuite) createFlow(name, handle string, nodes []map[string]interface{}) string {
	ts.T().Helper()

	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     name,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes:    nodes,
	})
	ts.Require().NoError(err, "Failed to create flow %s", handle)
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	return flowID
}

func (ts *CallFramesTestSuite) createApp(name, clientID, flowID string) string {
	ts.T().Helper()

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             name,
		Description:      "Application for call frame testing",
		ClientID:         clientID,
		ClientSecret:     clientID + "_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		OUID:             ts.ouID,
		AllowedUserTypes: []string{callFramesUserType.Handle},
		AuthFlowID:       flowID,
	})
	ts.Require().NoError(err, "Failed to create application %s", name)
	ts.createdAppIDList = append(ts.createdAppIDList, appID)
	return appID
}

func (ts *CallFramesTestSuite) TearDownSuite() {
	for _, appID := range ts.createdAppIDList {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("teardown: failed to delete application: %v", err)
		}
	}
	if ts.userID != "" {
		if err := testutils.DeleteUser(ts.userID); err != nil {
			ts.T().Logf("teardown: failed to delete user: %v", err)
		}
	}
	// Callers reference callees, so the callers created last are removed first.
	for i := len(ts.config.CreatedFlowIDs) - 1; i >= 0; i-- {
		if err := testutils.DeleteFlow(ts.config.CreatedFlowIDs[i]); err != nil {
			ts.T().Logf("teardown: failed to delete flow: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("teardown: failed to delete user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("teardown: failed to delete organization unit: %v", err)
		}
	}
}

// A call that pauses inside the callee has to survive the round trip to the client: the caller's
// frame is written into the stored context and read back on resume. Once the callee completes, the
// call returns to its caller, which continues at the call node's success target rather than ending
// with the callee.
func (ts *CallFramesTestSuite) TestCall_PausesInsideCalleeAndReturnsToCaller() {
	step, err := common.InitiateAuthenticationFlow(ts.pausingAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The call should pause inside the callee")
	ts.Require().True(common.HasInput(step.Data.Inputs, "username"),
		"The paused step must be the callee's own prompt")

	returned, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		"username": ts.username,
		"password": callFramesPassword,
	}, "action_001", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to resume the paused call")
	ts.Require().Equal("INCOMPLETE", returned.FlowStatus,
		"The caller should carry on rather than end with the callee")
	ts.True(common.HasInput(returned.Data.Inputs, "returned_marker"),
		"A completed callee must return to the caller's success target")
}

// A callee that fails ends the caller too when the call node names no failure target, so a failing
// call cannot silently fall through to whatever follows it.
func (ts *CallFramesTestSuite) TestCall_CalleeFailureWithoutOnFailureEndsFlow() {
	step, err := common.InitiateAuthenticationFlow(ts.failingAppID, false, nil, "")
	ts.Require().NoError(err, "The flow should return a step rather than a transport error")
	ts.Equal("ERROR", step.FlowStatus, "A failing callee must end the caller in error")
	ts.False(common.HasInput(step.Data.Inputs, "returned_marker"),
		"A failing callee must not reach the caller's success target")
}

// When the call node does name a failure target, the caller resumes there instead of ending, which
// is what lets a flow offer an alternative after a called flow could not complete.
func (ts *CallFramesTestSuite) TestCall_CalleeFailureForwardsToOnFailure() {
	step, err := common.InitiateAuthenticationFlow(ts.recoveringAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus,
		"A failing callee with a failure target should keep the caller running")
	ts.True(common.HasInput(step.Data.Inputs, "recovered_marker"),
		"The caller must resume at the call node's failure target")
}
