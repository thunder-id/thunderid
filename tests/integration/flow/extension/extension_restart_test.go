// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	extRestartConnectionName = "ext-restart-service"
	observabilityConfigKey   = "observability"
	extRestartEventLog       = "extension-events.log"

	eventTypeExtensionCall  = "EXTENSION_CALL"
	outcomeFailed           = "FAILED"
	outcomeAttemptsExceeded = "ATTEMPTS_EXCEEDED"
	outcomeConfigError      = "CONFIG_ERROR"
)

// extRestartOlderSteps are ext_step properties a flow saved before save validation could hold. Each
// flow is created well formed and then rewritten in the configuration database to these values.
var extRestartOlderSteps = map[string]func(properties map[string]interface{}){
	"NoConnection":          func(p map[string]interface{}) { delete(p, "connectionId") },
	"ResponseKeysNonString": func(p map[string]interface{}) { p["responseKeys"] = []interface{}{1} },
	"InputKeysString":       func(p map[string]interface{}) { p["inputKeys"] = "username" },
	"MaxAttemptsZero":       func(p map[string]interface{}) { p["maxAttempts"] = 0 },
	// A key the runtimeDataKeys allow list refuses, saved before the allow list existed.
	"RuntimeDataKeysNotAllowed": func(p map[string]interface{}) { p["runtimeDataKeys"] = []string{"clientId"} },
}

var (
	extRestartOU = testutils.OrganizationUnit{
		Handle:      "ext-restart-ou",
		Name:        "Extension Restart Test OU",
		Description: "OU for extension executor tests that need a server restart",
	}

	extRestartUserType = testutils.UserType{
		Handle:      "ext_restart_person",
		DisplayName: "Extension Restart Test Person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}

	extRestartUser = testutils.User{
		Type: "ext_restart_person",
		Attributes: json.RawMessage(`{
			"username": "extrestartuser",
			"password": "ExtRestartPass123!"
		}`),
	}

	extRestartApp = testutils.Application{
		Name:                      "Extension Restart Test Application",
		Description:               "Application whose login flow stops at the extension's attempt limit",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "ext_restart_client",
		ClientSecret:              "ext_restart_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"ext_restart_person"},
	}
)

// ExtensionRestartTestSuite covers what needs a server restart: a stored step that predates save
// validation fails before any call (AC1.6), which needs the server to load the rewritten flow, and
// the attempt-limit audit event (AC8.4), which needs observability enabled. Both share one restart.
type ExtensionRestartTestSuite struct {
	suite.Suite
	config       *common.TestSuiteConfig
	service      *mockService
	fixtures     *identityFixtures
	connectionID string
	olderFlowIDs map[string]string
	events       *testutils.EventLogReader
}

func TestExtensionRestartTestSuite(t *testing.T) {
	suite.Run(t, new(ExtensionRestartTestSuite))
}

func (ts *ExtensionRestartTestSuite) SetupSuite() {
	original, err := testutils.ReadDeploymentConfigKey(observabilityConfigKey)
	ts.Require().NoError(err, "Failed to read the observability section of deployment.yaml")
	// Registered before the patch: a failing SetupSuite skips TearDownSuite, but never a cleanup.
	t := ts.T()
	t.Cleanup(func() { restoreObservabilityConfig(t, original) })

	ts.config = &common.TestSuiteConfig{}
	ts.service = newMockService(seedValues{})
	ts.fixtures = createIdentityFixtures(&ts.Suite, extRestartOU, extRestartUserType, extRestartUser)

	connectionID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name: extRestartConnectionName, OUID: ts.fixtures.ouID, URL: ts.service.connectionURL(), TimeoutMS: 5000,
	})
	ts.Require().NoError(err, "Failed to create the HTTP endpoint connection")
	ts.connectionID = connectionID

	flowID, err := testutils.CreateFlow(retryExtensionFlow("Extension Restart Attempts Flow",
		"ext_restart_attempts_flow", ts.connectionID, ts.service.server.URL, 1))
	ts.Require().NoError(err, "Failed to create the attempt-limit flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	ts.fixtures.createApplication(&ts.Suite, extRestartApp, flowID)

	ts.olderFlowIDs = map[string]string{}
	for name, rewrite := range extRestartOlderSteps {
		olderID, err := testutils.CreateFlow(testutils.ExtensionAuthFlow("Extension Restart Older "+name+" Flow",
			"ext_restart_older_"+strings.ToLower(name)+"_flow",
			map[string]interface{}{"connectionId": ts.connectionID}))
		ts.Require().NoError(err, "Failed to create the older flow %s", name)
		ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, olderID)
		ts.olderFlowIDs[name] = olderID
		if testutils.GetDBType() == "sqlite" {
			ts.rewriteStoredStep(olderID, rewrite)
		}
	}

	logPath := testutils.EventLogPath(extRestartEventLog)
	ts.Require().NoError(testutils.PatchDeploymentConfig(testutils.ObservabilityFileSinkPatch(logPath, true)),
		"Failed to enable observability")
	ts.Require().NoError(testutils.RestartServer(), "Failed to restart the server")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain the admin token after restart")
	ts.events = testutils.NewEventLogReader(logPath)
}

func (ts *ExtensionRestartTestSuite) TearDownSuite() {
	ts.fixtures.deleteApplication(&ts.Suite)
	for i := len(ts.config.CreatedFlowIDs) - 1; i >= 0; i-- {
		if err := testutils.DeleteFlow(ts.config.CreatedFlowIDs[i]); err != nil {
			ts.T().Logf("Failed to delete test flow %s during teardown: %v", ts.config.CreatedFlowIDs[i], err)
		}
	}
	if ts.connectionID != "" {
		if err := testutils.DeleteHTTPConnection(ts.connectionID); err != nil {
			ts.T().Logf("Failed to delete HTTP connection during teardown: %v", err)
		}
	}
	ts.fixtures.deleteIdentities(&ts.Suite)
	if ts.service != nil {
		ts.service.close()
	}
}

func (ts *ExtensionRestartTestSuite) SetupTest() {
	ts.service.reset()
	ts.Require().NoError(ts.events.Reset(), "Failed to move past the events published so far")
}

// restoreObservabilityConfig puts back the observability section captured before the suite and
// restarts the server, so no later suite inherits the sink.
func restoreObservabilityConfig(t *testing.T, original interface{}) {
	if err := testutils.PatchDeploymentConfig(map[string]interface{}{observabilityConfigKey: original}); err != nil {
		t.Logf("Failed to restore the observability section of deployment.yaml: %v", err)
		return
	}
	if err := testutils.RestartServer(); err != nil {
		t.Logf("Server did not restart cleanly after the observability restore: %v", err)
		return
	}
	if err := testutils.ObtainAdminAccessToken(); err != nil {
		t.Logf("Failed to re-obtain the admin token after the observability restore: %v", err)
	}
}

// rewriteStoredStep applies rewrite to ext_step's properties in the flow's active stored version. The
// server reads the change only after a restart, since flows are cached.
func (ts *ExtensionRestartTestSuite) rewriteStoredStep(flowID string, rewrite func(map[string]interface{})) {
	ts.T().Helper()
	// flowID is a server-generated UUID, so it is safe to inline.
	row, err := testutils.QueryConfigDB(fmt.Sprintf(
		`SELECT DEPLOYMENT_ID, ACTIVE_VERSION FROM "FLOW" WHERE ID='%s';`, flowID))
	ts.Require().NoError(err, "Failed to read the stored flow %s", flowID)
	deploymentID, version, found := strings.Cut(row, "|")
	ts.Require().True(found, "The stored flow %s should have a deployment and version: %q", flowID, row)

	where := fmt.Sprintf(`FLOW_ID='%s' AND VERSION=%s AND DEPLOYMENT_ID='%s'`, flowID, version, deploymentID)
	stored, err := testutils.QueryConfigDB(`SELECT NODES FROM "FLOW_VERSION" WHERE ` + where + `;`)
	ts.Require().NoError(err, "Failed to read the stored nodes of flow %s", flowID)
	var nodes []map[string]interface{}
	ts.Require().NoError(json.Unmarshal([]byte(stored), &nodes), "The stored nodes should decode: %s", stored)

	rewritten := false
	for _, node := range nodes {
		if node["id"] != "ext_step" {
			continue
		}
		properties, ok := node["properties"].(map[string]interface{})
		ts.Require().True(ok, "ext_step should be stored with its properties: %v", node)
		rewrite(properties)
		rewritten = true
	}
	ts.Require().True(rewritten, "The stored flow %s should hold ext_step", flowID)

	encoded, err := json.Marshal(nodes)
	ts.Require().NoError(err, "Failed to encode the rewritten nodes")
	changed, err := testutils.QueryConfigDB(fmt.Sprintf(
		`UPDATE "FLOW_VERSION" SET NODES='%s' WHERE %s; SELECT changes();`,
		strings.ReplaceAll(string(encoded), "'", "''"), where))
	ts.Require().NoError(err, "Failed to rewrite the stored nodes of flow %s", flowID)
	ts.Require().Equal("1", changed, "Exactly one stored version of flow %s should be rewritten", flowID)
}

// awaitCallEvent waits for the EXTENSION_CALL event of executionID with outcome.
func (ts *ExtensionRestartTestSuite) awaitCallEvent(executionID, outcome string) testutils.ObservabilityEvent {
	ts.T().Helper()
	evt, err := ts.events.Await(func(evt testutils.ObservabilityEvent) bool {
		return evt.Type == eventTypeExtensionCall && evt.Str("execution_id") == executionID &&
			evt.Str("outcome") == outcome
	})
	ts.Require().NoError(err, "The %s event of execution %s should be published", outcome, executionID)
	return evt
}

// callEvents returns the EXTENSION_CALL events of executionID read so far with outcome.
func (ts *ExtensionRestartTestSuite) callEvents(executionID, outcome string) []testutils.ObservabilityEvent {
	ts.T().Helper()
	events, err := ts.events.Read()
	ts.Require().NoError(err, "Failed to read the event log")
	var matched []testutils.ObservabilityEvent
	for _, evt := range events {
		if evt.Type == eventTypeExtensionCall && evt.Str("execution_id") == executionID &&
			evt.Str("outcome") == outcome {
			matched = append(matched, evt)
		}
	}
	return matched
}

func (ts *ExtensionRestartTestSuite) TestAttemptLimitPublishesAttemptsExceeded() {
	ts.service.answer(retryCheckNode, okAnswer(`{"actionStatus":"FAILED","failureReason":"wrong_code"}`))
	step := ts.fixtures.startLogin(&ts.Suite)
	prompt, raw := ts.fixtures.submitCredentials(&ts.Suite, step)
	ts.Require().Equal("INCOMPLETE", prompt.FlowStatus, "FAILED should show the credentials prompt again: %s", raw)

	// A limit of 1 is reached by the first FAILED, so the second arrival stops without a call.
	result, raw := ts.fixtures.submitCredentials(&ts.Suite, prompt)
	assertAttemptsExceededAtCredentials(&ts.Suite, result, raw)
	ts.Require().Len(ts.service.callsFrom(retryCheckNode), 1, "The service should be called once")

	// The FAILED event is published before the ATTEMPTS_EXCEEDED one, so both have been read once the
	// later one is.
	exceeded := ts.awaitCallEvent(step.ExecutionID, outcomeAttemptsExceeded)
	ts.Len(ts.callEvents(step.ExecutionID, outcomeFailed), 1, "The called attempt should publish one FAILED event")
	ts.Len(ts.callEvents(step.ExecutionID, outcomeAttemptsExceeded), 1,
		"The stopped arrival should publish one ATTEMPTS_EXCEEDED event")
	ts.Equal(retryCheckNode, exceeded.Str("node_id"), "The event should name the step")
	ts.Equal(ts.connectionID, exceeded.Str("connection_id"), "The event should name the step's connection")
	ts.False(exceeded.Has("http_status"), "No call was made, so the event should carry no HTTP status")
	ts.False(exceeded.Has("failure_reason"),
		"The stopped arrival has no answer, so the event should carry no failure reason")
}

func (ts *ExtensionRestartTestSuite) TestStoredMalformedStepFailsBeforeAnyCall() {
	if testutils.GetDBType() != "sqlite" {
		ts.T().Skip("the older-flow fixture rewrites the SQLite config DB")
	}
	for name := range extRestartOlderSteps {
		ts.Run(name, func() {
			ts.service.reset()
			// The service would answer with fields, so a call would also store them.
			ts.service.answer("ext_step", okAnswer(`{"actionStatus":"SUCCESS","riskScore":"7"}`))
			ts.fixtures.bindFlow(&ts.Suite, ts.olderFlowIDs[name])

			// A step ThunderID cannot use is a failure inside ThunderID, not the service's, so the request
			// fails with the generic internal error rather than following a route.
			step := ts.fixtures.startLogin(&ts.Suite)
			submitExpectingInternalError(&ts.Suite, step,
				map[string]string{"username": ts.fixtures.username, "password": ts.fixtures.password},
				credentialsAction)
			evt := ts.awaitCallEvent(step.ExecutionID, outcomeConfigError)
			ts.Equal("ext_step", evt.Str("node_id"), "The event should name the step")
			ts.Empty(ts.service.allCalls(), "No request may be sent for an unusable step")
		})
	}
}
