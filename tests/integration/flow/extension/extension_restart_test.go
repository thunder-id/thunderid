// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	extRestartConnectionName = "ext-restart-service"
	observabilityConfigKey   = "observability"

	eventTypeExtensionCall   = "EXTENSION_CALL"
	outcomeFailed            = "FAILED"
	outcomeAttemptsExceeded  = "ATTEMPTS_EXCEEDED"
	extRestartEventWait      = 30 * time.Second
	extRestartEventPollEvery = 500 * time.Millisecond
)

// extRestartObservability enables the file sink, written to its default path, so the events the
// extension step publishes can be read back.
var extRestartObservability = map[string]interface{}{
	"enabled": true,
	"output": map[string]interface{}{
		"file": map[string]interface{}{"enabled": true, "format": "json"},
	},
}

// extRestartOlderSteps are ext_step properties a flow saved before save validation could hold. Each
// flow is created well formed and then rewritten in the configuration database to these values.
var extRestartOlderSteps = map[string]func(properties map[string]interface{}){
	"NoConnection":          func(p map[string]interface{}) { delete(p, "connectionId") },
	"ResponseKeysNonString": func(p map[string]interface{}) { p["responseKeys"] = []interface{}{1} },
	"InputKeysString":       func(p map[string]interface{}) { p["inputKeys"] = "username" },
	"MaxAttemptsZero":       func(p map[string]interface{}) { p["maxAttempts"] = 0 },
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
		Name: extRestartConnectionName, URL: ts.service.connectionURL(), TimeoutMS: 5000,
	})
	ts.Require().NoError(err, "Failed to create the HTTP endpoint connection")
	ts.connectionID = connectionID

	flowID, err := testutils.CreateFlow(retryExtensionFlow("Extension Restart Attempts Flow",
		"ext_restart_attempts_flow", ts.connectionID, 1))
	ts.Require().NoError(err, "Failed to create the attempt-limit flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	ts.fixtures.createApplication(&ts.Suite, extRestartApp, flowID)

	ts.olderFlowIDs = map[string]string{}
	for name, rewrite := range extRestartOlderSteps {
		olderID, err := testutils.CreateFlow(singleExtensionFlow("Extension Restart Older "+name+" Flow",
			"ext_restart_older_"+strings.ToLower(name)+"_flow",
			map[string]interface{}{"connectionId": ts.connectionID}))
		ts.Require().NoError(err, "Failed to create the older flow %s", name)
		ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, olderID)
		ts.olderFlowIDs[name] = olderID
		if testutils.GetDBType() == "sqlite" {
			ts.rewriteStoredStep(olderID, rewrite)
		}
	}

	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		observabilityConfigKey: extRestartObservability,
	}), "Failed to enable observability")
	ts.Require().NoError(testutils.RestartServer(), "Failed to restart the server")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain the admin token after restart")
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

// publishedEvent is the part of an observability event these tests read.
type publishedEvent struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

func (e publishedEvent) str(key string) string {
	value, _ := e.Data[key].(string)
	return value
}

// extensionCallEvents reads the sink until done holds for the EXTENSION_CALL events of executionID,
// or the bounded wait ends, and returns them. The sink flushes on a ticker, so the read is retried.
func (ts *ExtensionRestartTestSuite) extensionCallEvents(executionID string,
	done func([]publishedEvent) bool) []publishedEvent {
	ts.T().Helper()
	path := filepath.Join(testutils.GetExtractedProductHome(), "logs", "observability", "observability.log")
	deadline := time.Now().Add(extRestartEventWait)
	for {
		var matched []publishedEvent
		if file, err := os.Open(path); err == nil {
			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				var evt publishedEvent
				if json.Unmarshal(scanner.Bytes(), &evt) == nil && evt.Type == eventTypeExtensionCall &&
					evt.str("execution_id") == executionID {
					matched = append(matched, evt)
				}
			}
			_ = file.Close()
		} else if !os.IsNotExist(err) {
			ts.Require().NoError(err, "Failed to open the observability sink")
		}
		if done(matched) || time.Now().After(deadline) {
			return matched
		}
		time.Sleep(extRestartEventPollEvery)
	}
}

func withOutcomes(outcomes ...string) func([]publishedEvent) bool {
	return func(events []publishedEvent) bool {
		for _, outcome := range outcomes {
			if len(eventsWithOutcome(events, outcome)) == 0 {
				return false
			}
		}
		return true
	}
}

func eventsWithOutcome(events []publishedEvent, outcome string) []publishedEvent {
	var out []publishedEvent
	for _, evt := range events {
		if evt.str("outcome") == outcome {
			out = append(out, evt)
		}
	}
	return out
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

	events := ts.extensionCallEvents(step.ExecutionID, withOutcomes(outcomeFailed, outcomeAttemptsExceeded))
	ts.Len(eventsWithOutcome(events, outcomeFailed), 1, "The called attempt should publish one FAILED event")
	exceeded := eventsWithOutcome(events, outcomeAttemptsExceeded)
	ts.Require().Len(exceeded, 1, "The stopped arrival should publish one ATTEMPTS_EXCEEDED event")
	ts.Equal(retryCheckNode, exceeded[0].str("node_id"), "The event should name the step")
	ts.Equal(ts.connectionID, exceeded[0].str("connection_id"), "The event should name the step's connection")
	ts.NotContains(exceeded[0].Data, "http_status", "No call was made, so the event should carry no HTTP status")
	ts.NotContains(exceeded[0].Data, "failure_reason",
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

			result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
			ts.Equal("ERROR", result.FlowStatus, "A stored step with an unusable property should end the flow: %s",
				raw)
			ts.Empty(result.Assertion, "A failed flow must not return an assertion")
			ts.Require().NotNil(result.Error, "The flow should report the step's error: %s", raw)
			ts.Equal(errCodeExtensionConfigInvalid, result.Error.Code, "The error should be the configuration error")
			ts.Equal(msgExtensionConfigInvalid, result.Error.Message.DefaultValue,
				"The client should see the configuration error message")
			ts.Empty(ts.service.allCalls(), "No request may be sent for an unusable step")
		})
	}
}
