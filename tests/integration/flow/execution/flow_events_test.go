// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	eventTypeFlowStarted        = "FLOW_STARTED"
	eventTypeNodeExecStarted    = "FLOW_NODE_EXECUTION_STARTED"
	eventTypeNodeExecCompleted  = "FLOW_NODE_EXECUTION_COMPLETED"
	eventTypeNodeExecFailed     = "FLOW_NODE_EXECUTION_FAILED"
	eventTypeFlowCompletedEvent = "FLOW_COMPLETED"

	// The observability file adapter flushes on a fixed five second ticker, so a test that reads the
	// sink has to wait past one tick rather than expecting an immediate write.
	eventFlushWait = 9 * time.Second

	eventsTestPassword = "SecurePass123!"
)

// Observability is off by default, so the events it publishes need the deployment section enabled
// and a restart before any of them are written. The file sink is what makes the events assertable.
var (
	observabilityEnablePatch = map[string]interface{}{
		"observability": map[string]interface{}{
			"enabled": true,
			"output": map[string]interface{}{
				"file": map[string]interface{}{
					"enabled": true,
					"format":  "json",
				},
			},
		},
	}

	observabilityDisablePatch = map[string]interface{}{
		"observability": map[string]interface{}{
			"enabled": false,
		},
	}
)

// observabilityEvent is the subset of a published event this suite asserts on.
type observabilityEvent struct {
	TraceID string                 `json:"trace_id"`
	Type    string                 `json:"type"`
	Status  string                 `json:"status"`
	Data    map[string]interface{} `json:"data"`
}

// dataString reads a string-valued data field, which is how the flow engine writes the scalar
// fields; non-scalar fields (such as a structured error) come back as the empty string.
func (e observabilityEvent) dataString(key string) string {
	if value, ok := e.Data[key].(string); ok {
		return value
	}
	return ""
}

type FlowEventsTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig

	ouID       string
	userTypeID string
	appID      string
	// An application whose flow asserts an authenticated user without authenticating one, so the
	// run fails at a node instead of pausing for input.
	failingAppID string
	userID       string
	username     string
}

func TestFlowEventsTestSuite(t *testing.T) {
	suite.Run(t, new(FlowEventsTestSuite))
}

func (ts *FlowEventsTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ts.Require().NoError(testutils.PatchDeploymentConfig(observabilityEnablePatch),
		"failed to enable observability")
	ts.Require().NoError(testutils.RestartServer(),
		"failed to restart server with observability enabled")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(),
		"failed to re-obtain admin token after restart")

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "flow_events_test_ou",
		Name:        "Flow Events Test OU",
		Description: "Organization unit for flow observability event testing",
		Parent:      nil,
	})
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	ts.userTypeID, err = testutils.CreateUserType(testutils.UserType{
		Handle:      "flow-events-person",
		DisplayName: "Flow Events Person",
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
		},
	})
	ts.Require().NoError(err, "Failed to create test user type")

	ts.username = common.GenerateUniqueUsername("flow_events")
	ts.userID, err = testutils.CreateUser(testutils.User{
		Type: "flow-events-person",
		OUID: ouID,
		Attributes: json.RawMessage(`{
			"username": "` + ts.username + `",
			"password": "` + eventsTestPassword + `"
		}`),
	})
	ts.Require().NoError(err, "Failed to create test user")

	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Flow Events Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_events_test",
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
	})
	ts.Require().NoError(err, "Failed to create test flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	ts.appID, err = testutils.CreateApplication(testutils.Application{
		Name:             "Flow Events Test Application",
		Description:      "Application for flow observability event testing",
		ClientID:         "flow_events_test_client",
		ClientSecret:     "flow_events_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		OUID:             ouID,
		AllowedUserTypes: []string{"flow-events-person"},
		AuthFlowID:       flowID,
	})
	ts.Require().NoError(err, "Failed to create test application")

	failingFlowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Flow Events Failing Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_events_failing_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "auth_assert",
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
	})
	ts.Require().NoError(err, "Failed to create failing test flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, failingFlowID)

	ts.failingAppID, err = testutils.CreateApplication(testutils.Application{
		Name:             "Flow Events Failing Test Application",
		Description:      "Application whose flow fails at its first node",
		ClientID:         "flow_events_failing_test_client",
		ClientSecret:     "flow_events_failing_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		OUID:             ouID,
		AllowedUserTypes: []string{"flow-events-person"},
		AuthFlowID:       failingFlowID,
	})
	ts.Require().NoError(err, "Failed to create failing test application")
}

func (ts *FlowEventsTestSuite) TearDownSuite() {
	for _, appID := range []string{ts.appID, ts.failingAppID} {
		if appID == "" {
			continue
		}
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("teardown: failed to delete application: %v", err)
		}
	}
	if ts.userID != "" {
		if err := testutils.DeleteUser(ts.userID); err != nil {
			ts.T().Logf("teardown: failed to delete user: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
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

	if err := testutils.PatchDeploymentConfig(observabilityDisablePatch); err != nil {
		ts.T().Logf("teardown: failed to restore observability config: %v", err)
	}
	if err := testutils.RestartServer(); err != nil {
		ts.T().Logf("teardown: server did not restart cleanly after config restore: %v", err)
	}
	if err := testutils.ObtainAdminAccessToken(); err != nil {
		ts.T().Logf("teardown: failed to re-obtain admin token after restore: %v", err)
	}
}

// observabilityLogPath is where the file sink writes when no explicit path is configured.
func (ts *FlowEventsTestSuite) observabilityLogPath() string {
	return filepath.Join(testutils.GetExtractedProductHome(), "logs", "observability", "observability.log")
}

// eventsForExecution reads the sink and returns the events carrying the given execution id. The
// adapter buffers and flushes on a fixed ticker, so the read is retried until the caller's condition
// holds. Waiting on a precise condition rather than on any event of a type is what stops the read
// from returning a half-flushed prefix of the run: an earlier node's event of the same type would
// otherwise satisfy the wait while the node under test is still buffered.
func (ts *FlowEventsTestSuite) eventsForExecution(executionID string,
	done func([]observabilityEvent) bool) []observabilityEvent {
	ts.T().Helper()

	deadline := time.Now().Add(eventFlushWait)
	var matched []observabilityEvent

	for {
		matched = ts.readEvents(executionID)
		if done(matched) || time.Now().After(deadline) {
			return matched
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// hasType reports whether an event of the given type is present.
func hasType(eventType string) func([]observabilityEvent) bool {
	return func(events []observabilityEvent) bool {
		return typesOf(events)[eventType]
	}
}

// hasNodeEvent reports whether the given node published an event of the given type.
func hasNodeEvent(eventType, nodeID string) func([]observabilityEvent) bool {
	return func(events []observabilityEvent) bool {
		return eventFor(events, eventType, nodeID) != nil
	}
}

func (ts *FlowEventsTestSuite) readEvents(executionID string) []observabilityEvent {
	ts.T().Helper()

	file, err := os.Open(ts.observabilityLogPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		ts.Require().NoError(err, "Failed to open the observability sink")
	}
	defer file.Close()

	var matched []observabilityEvent
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var evt observabilityEvent
		if err := json.Unmarshal(line, &evt); err != nil {
			continue
		}
		if evt.TraceID == executionID || evt.dataString("execution_id") == executionID {
			matched = append(matched, evt)
		}
	}
	return matched
}

// typesOf returns the event types present in the given events.
func typesOf(events []observabilityEvent) map[string]bool {
	types := make(map[string]bool, len(events))
	for _, evt := range events {
		types[evt.Type] = true
	}
	return types
}

// eventFor returns the first event of the given type touching the given node.
func eventFor(events []observabilityEvent, eventType, nodeID string) *observabilityEvent {
	for i := range events {
		if events[i].Type == eventType && events[i].dataString("node_id") == nodeID {
			return &events[i]
		}
	}
	return nil
}

// A completed flow publishes the execution trail an operator needs to follow it: the flow starting,
// each node starting and completing, and the flow completing. All of them are correlated by the
// execution id, which is what makes a single run reconstructable from the sink.
func (ts *FlowEventsTestSuite) TestFlowEvents_SuccessfulRunPublishesNodeTrail() {
	step, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Flow should pause at the credentials prompt")

	completed, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		"username": ts.username,
		"password": eventsTestPassword,
	}, "action_001", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit credentials")
	ts.Require().Equal("COMPLETE", completed.FlowStatus, "The flow should authenticate the user")

	events := ts.eventsForExecution(step.ExecutionID, hasType(eventTypeFlowCompletedEvent))
	ts.Require().NotEmpty(events, "The run should have published events to the sink")

	present := typesOf(events)
	ts.True(present[eventTypeFlowStarted], "the flow start should be published")
	ts.True(present[eventTypeNodeExecStarted], "node executions should be published as they start")
	ts.True(present[eventTypeNodeExecCompleted], "node executions should be published as they complete")
	ts.True(present[eventTypeFlowCompletedEvent], "the flow completion should be published")

	authEvent := eventFor(events, eventTypeNodeExecCompleted, "credentials_auth")
	ts.Require().NotNil(authEvent, "the authenticating node must publish a completion event")
	ts.Equal("success", authEvent.Status, "a node that completed must be published as a success")
	ts.Equal("1", authEvent.dataString("attempt_number"),
		"the first execution of a node is its first attempt")
	ts.NotEmpty(authEvent.dataString("step_number"), "a node execution must carry its step number")
	ts.NotEmpty(authEvent.dataString("duration_ms"), "a node execution must carry how long it took")
}

// A wrong password does not fail the node. The executor forwards back to the prompt, so the run
// publishes the authenticating node as completed with the forwarding status and the reason, and the
// re-presented prompt as incomplete carrying the same reason. Both are published as successes: the
// step did not pass, but nothing broke.
func (ts *FlowEventsTestSuite) TestFlowEvents_ForwardedNodePublishesReason() {
	step, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate flow")

	retried, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		"username": ts.username,
		"password": "WrongPassword123!",
	}, "action_001", step.ChallengeToken)
	ts.Require().NoError(err, "A wrong password should still return a flow step")
	ts.Require().Equal("INCOMPLETE", retried.FlowStatus,
		"A wrong password should re-prompt rather than end the flow")

	events := ts.eventsForExecution(step.ExecutionID,
		hasNodeEvent(eventTypeNodeExecCompleted, "credentials_auth"))
	ts.Require().NotEmpty(events, "The run should have published events to the sink")

	authEvent := eventFor(events, eventTypeNodeExecCompleted, "credentials_auth")
	ts.Require().NotNil(authEvent, "the authenticating node must publish its execution")
	ts.Equal("success", authEvent.Status,
		"a node that forwarded is not a broken node, so it is published as a success")
	ts.Equal("FORWARD", authEvent.dataString("node_status"),
		"a node that forwards back to its prompt must be published with the forwarding status")
	ts.NotNil(authEvent.Data["error"], "the published event must carry why the step did not pass")
}

// A node that genuinely fails is published as a failure, and the run it belongs to is published as
// failed. Asserting an authenticated user without anything having authenticated one is the shortest
// flow that fails at a node rather than pausing for input.
func (ts *FlowEventsTestSuite) TestFlowEvents_FailedNodePublishesFailure() {
	step, err := common.InitiateAuthenticationFlow(ts.failingAppID, false, nil, "")
	ts.Require().NoError(err, "The flow should return a step rather than a transport error")
	ts.Require().NotEmpty(step.ExecutionID, "A failed run must still report its execution id")

	events := ts.eventsForExecution(step.ExecutionID, hasNodeEvent(eventTypeNodeExecFailed, "auth_assert"))
	ts.Require().NotEmpty(events, "The run should have published events to the sink")

	authEvent := eventFor(events, eventTypeNodeExecFailed, "auth_assert")
	ts.Require().NotNil(authEvent, "a node that failed must publish a failure event")
	ts.Equal("failure", authEvent.Status, "a failed node must be published as a failure")
	ts.Equal("ERROR", authEvent.dataString("node_status"),
		"a failed node must be published with the error status")
	ts.NotNil(authEvent.Data["error"], "a failure event must carry what went wrong")
}
