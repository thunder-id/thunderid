// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// shortFlowExpirySeconds is the smallest positive expiry the configuration accepts, so the expiry
// test waits the shortest time it can while still crossing the boundary.
const shortFlowExpirySeconds = 1

var (
	lifecycleTestOU = testutils.OrganizationUnit{
		Handle:      "flow_lifecycle_test_ou",
		Name:        "Test OU for Flow Lifecycle",
		Description: "Organization unit created for flow lifecycle testing",
		Parent:      nil,
	}

	// A flow that stops at a prompt, so an execution exists to resume and to let expire.
	lifecycleTestFlow = testutils.Flow{
		Name:     "Flow Lifecycle Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_lifecycle_test",
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
						},
						"action": map[string]interface{}{
							"ref":      "action_001",
							"nextNode": "auth_assert",
						},
					},
				},
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

	lifecycleTestApp = testutils.Application{
		Name:         "Flow Lifecycle Test Application",
		Description:  "Application for testing flow execution lifecycle",
		ClientID:     "flow_lifecycle_test_client",
		ClientSecret: "flow_lifecycle_test_secret",
		RedirectURIs: []string{"http://localhost:3000/callback"},
	}
)

type FlowLifecycleTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig
	ouID   string
	appID  string
	flowID string
}

func TestFlowLifecycleTestSuite(t *testing.T) {
	suite.Run(t, new(FlowLifecycleTestSuite))
}

func (ts *FlowLifecycleTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(lifecycleTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	flowID, err := testutils.CreateFlow(lifecycleTestFlow)
	ts.Require().NoError(err, "Failed to create test flow")
	ts.flowID = flowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	lifecycleTestApp.OUID = ts.ouID
	lifecycleTestApp.AuthFlowID = flowID
	appID, err := testutils.CreateApplication(lifecycleTestApp)
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID
}

func (ts *FlowLifecycleTestSuite) TearDownSuite() {
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// patchFlowConfig applies the given keys over the writable layer of the flow section and registers
// the restore, so a mid-test failure still returns the deployment to its original configuration.
func (ts *FlowLifecycleTestSuite) patchFlowConfig(patch map[string]interface{}) {
	ts.T().Helper()

	original, err := testutils.MergeWritableServerConfig(flowConfigSection, patch)
	ts.Require().NoError(err, "Failed to update flow server config")
	ts.T().Cleanup(func() {
		if err := testutils.PutWritableServerConfig(flowConfigSection, original); err != nil {
			ts.T().Errorf("cleanup: failed to restore flow server config: %v", err)
		}
	})
}

// An execution started without inputs must be resumable by its execution id, carrying the flow
// forward from the stored context rather than starting over.
//
// The assertion is that the context is found and advanced, not that the flow reaches a particular
// terminal state: the node after the prompt asserts an authenticated user, which this test never
// establishes. Being accepted at all is what proves resume works, since an unknown or expired
// execution id is rejected instead (see TestExpiry_ExpiredExecutionRejected).
func (ts *FlowLifecycleTestSuite) TestResume_ContinuesExistingExecution() {
	step, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Flow should pause at the prompt")
	ts.Require().NotEmpty(step.ExecutionID, "A paused flow must return an execution id")

	resumed, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": "someone@example.com"}, "action_001", "")
	ts.Require().NoError(err, "Resuming an existing execution should be accepted")
	ts.Equal(step.ExecutionID, resumed.ExecutionID, "Resume must stay on the same execution")
	ts.NotEmpty(resumed.FlowStatus, "Resume must report a flow status")
}

// Resuming a prompt without supplying its required input is refused rather than silently
// re-presenting the prompt, so a caller cannot loop on an execution without making progress.
func (ts *FlowLifecycleTestSuite) TestResume_WithoutRequiredInputFails() {
	step, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate flow")
	ts.Require().NotEmpty(step.ExecutionID, "A paused flow must return an execution id")

	resumed, err := common.ResumeFlow(step.ExecutionID)
	ts.Require().NoError(err, "A bare resume should still return a flow step")
	ts.Equal("ERROR", resumed.FlowStatus, "Resuming without the required input should fail the step")
}

// An application keeps pointing at its authentication flow after that flow is deleted, because
// deleting a flow does not clear the reference. Rather than failing every sign-in for that
// application, the engine falls back to the configured default authentication flow.
//
// The default handle is pointed at a flow this test creates, so the fallback is identified by the
// prompt it lands on rather than by whatever the deployment happens to ship as its default.
func (ts *FlowLifecycleTestSuite) TestFallback_DeletedFlowFallsBackToDefault() {
	defaultFlowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Flow Lifecycle Fallback Default Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_lifecycle_fallback_default",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_fallback"},
			{
				"id":   "prompt_fallback",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "fallback_marker",
								"identifier": "fallback_marker",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_fallback",
							"nextNode": "auth_assert",
						},
					},
				},
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	})
	ts.Require().NoError(err, "Failed to create the fallback default flow")
	ts.T().Cleanup(func() {
		if err := testutils.DeleteFlow(defaultFlowID); err != nil {
			ts.T().Logf("cleanup: failed to delete fallback default flow: %v", err)
		}
	})

	ts.patchFlowConfig(map[string]interface{}{
		"authFlow": map[string]interface{}{"defaultHandle": "auth_flow_lifecycle_fallback_default"},
	})

	// A flow of its own, so deleting it cannot disturb the other tests in this suite.
	doomedFlowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Flow Lifecycle Doomed Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_lifecycle_doomed",
		Nodes:    lifecycleTestFlow.Nodes,
	})
	ts.Require().NoError(err, "Failed to create the flow that gets deleted")

	orphanedAppID, err := testutils.CreateApplication(testutils.Application{
		Name:         "Flow Lifecycle Fallback Application",
		Description:  "Application whose authentication flow is deleted out from under it",
		ClientID:     "flow_lifecycle_fallback_client",
		ClientSecret: "flow_lifecycle_fallback_secret",
		RedirectURIs: []string{"http://localhost:3000/callback"},
		OUID:         ts.ouID,
		AuthFlowID:   doomedFlowID,
	})
	ts.Require().NoError(err, "Failed to create the application")
	ts.T().Cleanup(func() {
		if err := testutils.DeleteApplication(orphanedAppID); err != nil {
			ts.T().Logf("cleanup: failed to delete fallback application: %v", err)
		}
	})

	ts.Require().NoError(testutils.DeleteFlow(doomedFlowID),
		"Deleting a referenced flow should be allowed, which is what creates the dangling reference")

	step, err := common.InitiateAuthenticationFlow(orphanedAppID, false, nil, "")
	ts.Require().NoError(err, "A dangling flow reference should not fail the request")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The fallback flow should run")
	ts.True(common.HasInput(step.Data.Inputs, "fallback_marker"),
		"The request must be served by the configured default authentication flow")
}

// A flow context is held for the configured expiry and no longer, after which its execution id is
// no longer usable. This is the branch that separates "unknown execution" from "expired execution",
// and both surface as the same client error by design.
func (ts *FlowLifecycleTestSuite) TestExpiry_ExpiredExecutionRejected() {
	// The expiry is read from the merged server config on every execution, so this takes effect
	// without restarting the server.
	ts.patchFlowConfig(map[string]interface{}{
		"authFlow": map[string]interface{}{"expirySeconds": shortFlowExpirySeconds},
	})

	step, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate flow")
	ts.Require().NotEmpty(step.ExecutionID, "A paused flow must return an execution id")

	// Wait past the expiry with margin, so the context is gone rather than merely stale.
	time.Sleep(time.Duration(shortFlowExpirySeconds)*time.Second + 2*time.Second)

	errResp, err := common.CompleteAuthFlowWithError(step.ExecutionID,
		map[string]string{"username": "someone"}, "")
	ts.Require().NoError(err, "Expected a client error for an expired execution")
	ts.Equal(errCodeInvalidExecutionID, errResp.Code,
		"An expired execution must be rejected as an invalid execution id")
}
