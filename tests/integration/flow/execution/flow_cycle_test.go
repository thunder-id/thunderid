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

var (
	cycleTestOU = testutils.OrganizationUnit{
		Handle:      "flow_cycle_test_ou",
		Name:        "Test OU for Flow Cycles",
		Description: "Organization unit created for flow cycle testing",
		Parent:      nil,
	}

	// The credentials node asks for an input the prompt never collects, so the prompt completes with
	// the inputs it already has and the flow cycles between the two nodes without pausing.
	cycleTestFlow = testutils.Flow{
		Name:     "Flow Cycle Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_cycle_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_credentials",
			},
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
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_003",
							"identifier": "nickname",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
					},
				},
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_credentials",
			},
			{
				"id":   "auth_assert",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthAssertExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}

	cycleTestApp = testutils.Application{
		Name:         "Flow Cycle Test Application",
		Description:  "Application for testing flows that cycle without pausing",
		ClientID:     "flow_cycle_test_client",
		ClientSecret: "flow_cycle_test_secret", //nolint:gosec // test credential
		RedirectURIs: []string{"http://localhost:3000/callback"},
	}
)

type FlowCycleTestSuite struct {
	suite.Suite
	ouID   string
	flowID string
	appID  string
}

func TestFlowCycleTestSuite(t *testing.T) {
	suite.Run(t, new(FlowCycleTestSuite))
}

func (ts *FlowCycleTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(cycleTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	flowID, err := testutils.CreateFlow(cycleTestFlow)
	ts.Require().NoError(err, "Failed to create test flow")
	ts.flowID = flowID

	cycleTestApp.OUID = ts.ouID
	cycleTestApp.AuthFlowID = flowID
	appID, err := testutils.CreateApplication(cycleTestApp)
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID
}

func (ts *FlowCycleTestSuite) TearDownSuite() {
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}
	if ts.flowID != "" {
		if err := testutils.DeleteFlow(ts.flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

func (ts *FlowCycleTestSuite) TestCyclingFlowFailsWithServerError() {
	flowStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	inputs := map[string]string{"username": "cycle_user", "password": "cycle_password"}
	_, rawBody, err := common.CompleteFlowWithRawResponse(flowStep.ExecutionID, inputs, "action_001",
		flowStep.ChallengeToken)

	ts.Require().ErrorContains(err, "unexpected status 500")
	var errResp common.ErrorResponse
	ts.Require().NoError(json.Unmarshal([]byte(rawBody), &errResp), "Failed to parse error response")
	ts.Equal("SSE-5000", errResp.Code)
}
