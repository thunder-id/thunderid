// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// errCodeMaxCallDepth is returned when nested CALL nodes exceed the engine's frame limit.
const errCodeMaxCallDepth = "FES-1013"

// callChainLength is longer than the engine's maximum call depth of 10 frames, so executing the head
// of the chain is guaranteed to cross the limit.
const callChainLength = 12

var callDepthTestOU = testutils.OrganizationUnit{
	Handle:      "call_depth_test_ou",
	Name:        "Test OU for Call Depth",
	Description: "Organization unit created for call depth testing",
	Parent:      nil,
}

type CallDepthTestSuite struct {
	suite.Suite
	ouID     string
	appID    string
	flowIDs  []string
	headFlow string
}

func TestCallDepthTestSuite(t *testing.T) {
	suite.Run(t, new(CallDepthTestSuite))
}

// SetupSuite builds a chain of authentication flows where each one calls the next, then binds an
// application to the head of the chain. The chain is built from the tail backwards because a CALL
// node must reference a flow that already exists.
func (ts *CallDepthTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(callDepthTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	// The tail is a plain authentication flow that calls nothing.
	tailID, err := testutils.CreateFlow(ts.buildLeafFlow())
	ts.Require().NoError(err, "Failed to create leaf flow")
	ts.flowIDs = append(ts.flowIDs, tailID)

	next := tailID
	for i := 0; i < callChainLength; i++ {
		flowID, err := testutils.CreateFlow(ts.buildCallingFlow(i, next))
		ts.Require().NoError(err, "Failed to create calling flow %d", i)
		ts.flowIDs = append(ts.flowIDs, flowID)
		next = flowID
	}
	ts.headFlow = next

	app := testutils.Application{
		Name:         "Call Depth Test Application",
		Description:  "Application bound to a deeply nested call chain",
		ClientID:     "call_depth_test_client",
		ClientSecret: "call_depth_test_secret", //nolint:gosec // test credential
		RedirectURIs: []string{"http://localhost:3000/callback"},
		OUID:         ts.ouID,
		AuthFlowID:   ts.headFlow,
	}
	appID, err := testutils.CreateApplication(app)
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID
}

func (ts *CallDepthTestSuite) TearDownSuite() {
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}
	// Delete from the head backwards so a flow is never removed while another still references it.
	for i := len(ts.flowIDs) - 1; i >= 0; i-- {
		if err := testutils.DeleteFlow(ts.flowIDs[i]); err != nil {
			ts.T().Logf("Failed to delete flow %s during teardown: %v", ts.flowIDs[i], err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// credentialsAuthNodes returns the credential prompt, the authentication executor and the assertion
// that make up a complete authentication flow, ending at the given node.
//
// Every flow in the chain carries these so each one is a valid authentication flow in its own right
// rather than a graph that only happens to be accepted today, which keeps the fixture standing as
// flow validation gains rules.
func credentialsAuthNodes(endNode string) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":   "credentials_prompt",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
					"action": map[string]interface{}{
						"ref":      "action_credentials",
						"nextNode": "credentials_auth",
					},
				},
			},
		},
		{
			"id":        "credentials_auth",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor"},
			"onSuccess": "auth_assert",
		},
		{
			"id":        "auth_assert",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": endNode,
		},
		{"id": endNode, "type": "END"},
	}
}

// buildLeafFlow returns the flow at the end of the chain, which calls nothing and authenticates the
// user itself.
func (ts *CallDepthTestSuite) buildLeafFlow() testutils.Flow {
	nodes := []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "credentials_prompt"},
	}
	return testutils.Flow{
		Name:     "Call Depth Leaf Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_call_depth_leaf",
		Nodes:    append(nodes, credentialsAuthNodes("end")...),
	}
}

// buildCallingFlow returns a flow whose only work is to call the given target flow.
//
// Its own authentication section sits after the CALL, so the chain still descends on the first node
// and trips the depth limit before any prompt is reached.
func (ts *CallDepthTestSuite) buildCallingFlow(index int, targetFlowID string) testutils.Flow {
	nodes := []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "call_next"},
		{
			"id":        "call_next",
			"type":      "CALL",
			"flow":      map[string]interface{}{"ref": targetFlowID},
			"onSuccess": "credentials_prompt",
		},
	}
	return testutils.Flow{
		Name:     fmt.Sprintf("Call Depth Link %d Flow", index),
		FlowType: "AUTHENTICATION",
		Handle:   fmt.Sprintf("auth_flow_call_depth_link_%d", index),
		Nodes:    append(nodes, credentialsAuthNodes("end")...),
	}
}

// A chain of nested CALL nodes deeper than the engine allows must be refused rather than recursing
// until the process runs out of stack. The limit is what stops a mutually recursive set of flows,
// which the flow designer does not prevent an operator from authoring, from taking the server down.
func (ts *CallDepthTestSuite) TestExecute_ExceedingCallDepthRejected() {
	// ErrorMaxCallDepthExceeded is a client error, so the engine refuses the request with 400 and
	// names the limit in the response rather than recursing until the process runs out of stack.
	errResp, err := common.InitiateAuthFlowWithError(ts.appID, nil)
	ts.Require().NoError(err, "a call chain past the depth limit must be rejected as a client error")
	ts.Equal(errCodeMaxCallDepth, errResp.Code, "the failure must name the call depth limit")
}
