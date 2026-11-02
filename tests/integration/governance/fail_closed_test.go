// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	failClosedGovernanceClientID     = "governance_fail_closed_client"
	failClosedGovernanceClientSecret = "governance_fail_closed_secret"

	// noRevocationStorePath points database.runtime_persistent at a SQLite file holding only the
	// runtime table, so governance reads and writes work while every revocation write fails. It has no
	// directory component, so it resolves against the server home under every database profile.
	noRevocationStorePath = "governance_no_revocation_store.db"

	// eventTokenRevocationFailed is published when a criteria revocation cannot be stored.
	eventTokenRevocationFailed = "TOKEN_REVOCATION_FAILED"
	eventKeyCriterionType      = "criterion_type"
	eventKeyRevocationReason   = "revocation_reason"
)

// unreadableEntitySignIn is the shipped sign-in flow's answer when the account cannot be read.
var unreadableEntitySignIn = flowAnswer{status: http.StatusOK, flowStatus: "ERROR", code: "FET-1006"}

// FailClosedTestSuite checks that governance refuses when it cannot read what it needs, with the
// answer the path gives for an unreadable entity, never as a hold or a wrong credential; and that a
// suspension whose containment fails is not placed. It runs under the shipped policy. While the
// revocation store is broken, events go to a file sink, so a failure is seen to come from containment.
type FailClosedTestSuite struct {
	suite.Suite
	users        *userFixture
	appID        string
	shippedAppID string
	events       *testutils.EventLogReader

	// originalDatabaseConfig is the deployment's database section, captured before it is broken.
	originalDatabaseConfig map[string]interface{}
	configBroken           bool
}

func TestFailClosedTestSuite(t *testing.T) {
	suite.Run(t, new(FailClosedTestSuite))
}

func (ts *FailClosedTestSuite) SetupSuite() {
	ts.users = newUserFixture(ts.T(), "fail_closed", testutils.UserType{Schema: credentialsSchema(nil)})
	ts.events = testutils.NewEventLogReader(failClosedEventLogPath())

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:        ts.users.ouID,
		Name:        "Governance Fail Closed Client",
		Description: "Client for the governance fail-closed tests",
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                failClosedGovernanceClientID,
				"clientSecret":            failClosedGovernanceClientSecret,
				"redirectUris":            []string{"https://localhost:3000"},
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err, "failed to create the client")
	ts.appID = appID
	cleanupApplication(ts.T(), appID)
	ts.shippedAppID = newShippedFlowApp(ts.T(), ts.users, "fail_closed")
}

func (ts *FailClosedTestSuite) TearDownSuite() {
	// Safety net: each test restores the database in a deferred block.
	ts.restoreRuntimePersistentDB()
}

// At sign-in, an identity whose runtime document cannot be read is refused with a server error,
// not with a wrong credential's answer or a stated hold, and a management read reports it suspended.
// Restoring the document shows the refusal came from it.
func (ts *FailClosedTestSuite) TestAnUnreadableRuntimeRecordRefusesSignInAsAServerError() {
	userID, username, password := ts.users.newUser(ts.T(), "unreadable", nil)
	ts.Require().True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the account must sign in while its record is readable")

	ts.corruptRuntimeRecord(userID)
	defer ts.repairRuntimeRecord(userID)

	refused := authenticate(ts.T(), "username", username, password)
	ts.Equal(http.StatusInternalServerError, refused.status,
		"an unreadable record must refuse as a server error: %s", refused.body)
	ts.NotContains(refused.body, "locked", "an outage must not be stated as a hold")
	ts.NotContains(refused.body, "uspended", "an outage must not be stated as a hold")

	status := readStatus(ts.T(), userID)
	ts.Equal("SUSPENDED", status.Value, "an unreadable record must read as suspended on a management read")
	ts.Require().NotNil(status.Details)
	ts.Equal("ADMINISTRATIVE_SUSPENSION", status.Details.Reason)
	ts.Empty(status.Details.Since, "an unreadable record has no suspension time to report")

	ts.repairRuntimeRecord(userID)
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the account must sign in again once its record is readable")
}

// The shipped sign-in flow answers an unreadable runtime document as it answers an unreadable account,
// never as a hold or a wrong password.
func (ts *FailClosedTestSuite) TestAnUnreadableRuntimeRecordFailsTheSignInFlow() {
	userID, username, password := ts.users.newUser(ts.T(), "unreadable_flow", nil)
	ts.Require().Equal("COMPLETE",
		answerOf(submitSignIn(ts.T(), ts.shippedAppID, shippedSignInAction, username, password)).flowStatus,
		"precondition: the account must sign in while its record is readable")

	ts.corruptRuntimeRecord(userID)
	defer ts.repairRuntimeRecord(userID)

	status, body := submitSignIn(ts.T(), ts.shippedAppID, shippedSignInAction, username, password)
	ts.Equal(unreadableEntitySignIn, answerOf(status, body),
		"an unreadable record must fail the flow as an unreadable account does: %s", body)
	ts.NotContains(body, flowStatedLockCode, "an outage must not be stated as a hold")
	ts.NotContains(body, flowStatedSuspensionCode, "an outage must not be stated as a hold")

	ts.repairRuntimeRecord(userID)
	ts.Equal("COMPLETE",
		answerOf(submitSignIn(ts.T(), ts.shippedAppID, shippedSignInAction, username, password)).flowStatus,
		"the account must sign in again once its record is readable")
}

// At OAuth client authentication, an application whose runtime document cannot be read is
// refused as invalid_client, the answer for an unreadable client.
func (ts *FailClosedTestSuite) TestAnUnreadableApplicationRecordRefusesClientAuthentication() {
	result := clientCredentials(ts.T(), failClosedGovernanceClientID, failClosedGovernanceClientSecret, "")
	ts.Require().Equal(http.StatusOK, result.StatusCode,
		"the client must authenticate while its record is readable: %s", result.Body)

	ts.corruptRuntimeRecord(ts.appID)
	defer ts.repairRuntimeRecord(ts.appID)

	result = clientCredentials(ts.T(), failClosedGovernanceClientID, failClosedGovernanceClientSecret, "")
	ts.Equal(http.StatusUnauthorized, result.StatusCode, "an unreadable client record must refuse: %s", result.Body)
	ts.Contains(string(result.Body), "invalid_client", "the refusal must be invalid_client")

	ts.repairRuntimeRecord(ts.appID)
	result = clientCredentials(ts.T(), failClosedGovernanceClientID, failClosedGovernanceClientSecret, "")
	ts.Equal(http.StatusOK, result.StatusCode,
		"the client must authenticate again once its record is readable: %s", result.Body)
}

// A public client presents no secret, so its unreadable runtime document is refused at client
// authentication itself, as invalid_client, rather than reaching the grant.
func (ts *FailClosedTestSuite) TestAnUnreadablePublicClientRecordRefusesClientAuthentication() {
	appID, clientID := newPublicClientApp(ts.T(), ts.users, "fail_closed")
	admitted := redeemUnknownCode(ts.T(), clientID)
	ts.Require().Equal(http.StatusBadRequest, admitted.StatusCode,
		"precondition: the public client must authenticate and reach the grant: %s", admitted.Body)
	ts.Require().Contains(string(admitted.Body), "invalid_grant",
		"precondition: the unknown code must be the grant's refusal")

	ts.corruptRuntimeRecord(appID)
	defer ts.repairRuntimeRecord(appID)

	refused := redeemUnknownCode(ts.T(), clientID)
	ts.Equal(http.StatusUnauthorized, refused.StatusCode,
		"an unreadable public client record must refuse at client authentication: %s", refused.Body)
	ts.Contains(string(refused.Body), "invalid_client", "the refusal must be invalid_client")

	ts.repairRuntimeRecord(appID)
	admitted = redeemUnknownCode(ts.T(), clientID)
	ts.Equal(http.StatusBadRequest, admitted.StatusCode,
		"the public client must authenticate again once its record is readable: %s", admitted.Body)
	ts.Contains(string(admitted.Body), "invalid_grant", "the repaired client must reach the grant again")
}

// Containment runs before the hold: when the revocation store cannot be written, the suspend
// flow faults and the account is not suspended, and Suspend stays available once the store is back.
func (ts *FailClosedTestSuite) TestAFailedContainmentLeavesTheAccountActive() {
	userID, username, password := ts.users.newUser(ts.T(), "containment", nil)

	defer ts.restoreRuntimePersistentDB()
	ts.breakRevocationStore()
	ts.Require().NoError(ts.events.Reset())

	ts.Equal(serverFault, attemptAdministrationFlow(ts.T(), suspendFlowHandle, userID),
		"a suspend whose revocation step cannot write must fault")
	failed, err := ts.events.Await(testutils.EventOfTypeWith(eventTokenRevocationFailed, eventKeySubject, userID))
	ts.Require().NoError(err, "the fault must come from containment failing to store the revocation")
	ts.Equal("subject", failed.Str(eventKeyCriterionType), "the failed revocation must be the subject's")
	ts.Equal("identity_suspended", failed.Str(eventKeyRevocationReason),
		"the failed revocation must be the suspension's")
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "no hold may be written after a failed containment")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the account must still be usable")

	ts.restoreRuntimePersistentDB()
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.Equal("SUSPENDED", readStatus(ts.T(), userID).Value,
		"Suspend must succeed once the store is back")
}

// corruptRuntimeRecord stores a document that is valid JSON, so the table accepts it, but not an
// access-state document, so the server cannot read it.
func (ts *FailClosedTestSuite) corruptRuntimeRecord(entityID string) {
	ts.T().Helper()

	ts.Require().Equal(1, runtimeRowCount(ts.T(), entityID), "the entity must have a runtime row to corrupt")
	ts.Require().NoError(setRuntimeAttributes(ts.T(), entityID, `{"accessState":"unreadable"}`),
		"failed to corrupt the runtime record")
}

// repairRuntimeRecord puts an empty document back. Safe to call twice.
func (ts *FailClosedTestSuite) repairRuntimeRecord(entityID string) {
	if err := setRuntimeAttributes(ts.T(), entityID, `{}`); err != nil {
		ts.T().Errorf("failed to repair the runtime record of %s: %v", entityID, err)
	}
}

// breakRevocationStore points runtime_persistent at a database holding only the runtime table, turns
// the event sink on and restarts. The admin token is not re-obtained: it was issued before and stays valid across the
// restart, while issuing a new one may need a store this database lacks.
func (ts *FailClosedTestSuite) breakRevocationStore() {
	original, err := testutils.ReadDeploymentConfigKey("database")
	ts.Require().NoError(err, "failed to read the database section of deployment.yaml")
	originalMap, ok := original.(map[string]interface{})
	ts.Require().True(ok, "deployment.yaml has no database section")
	ts.originalDatabaseConfig = originalMap

	ts.Require().NoError(testutils.CreateEntityRuntimeOnlyDB(noRevocationStorePath),
		"failed to create the database without a revocation store")
	patch := testutils.ObservabilityFileSinkPatch(failClosedEventLogPath(), true)

	broken := make(map[string]interface{}, len(originalMap))
	for key, value := range originalMap {
		broken[key] = value
	}
	broken["runtime_persistent"] = map[string]interface{}{
		"type":   "sqlite",
		"sqlite": map[string]interface{}{"path": noRevocationStorePath},
	}

	patch["database"] = broken

	ts.configBroken = true
	ts.Require().NoError(testutils.PatchDeploymentConfig(patch),
		"failed to point runtime_persistent at the database without a revocation store")
	ts.Require().NoError(testutils.RestartServer(), "the server must boot without a revocation store")
}

// restoreRuntimePersistentDB puts the captured database section back, turns the event sink off,
// restarts and re-obtains the admin token. Every step runs even if one fails, so the server is left
// healthy for later packages.
func (ts *FailClosedTestSuite) restoreRuntimePersistentDB() {
	if !ts.configBroken {
		return
	}
	patch := testutils.ObservabilityFileSinkPatch(failClosedEventLogPath(), false)
	patch["database"] = ts.originalDatabaseConfig
	ts.Assert().NoError(testutils.PatchDeploymentConfig(patch),
		"failed to restore the database section of deployment.yaml")
	ts.Assert().NoError(testutils.RestartServer(), "failed to restart after the database restore")
	ts.Assert().NoError(testutils.ObtainAdminAccessToken(), "failed to re-obtain the admin token")
	ts.configBroken = false
}

// failClosedEventLogPath is the file sink the suite reads while the revocation store is broken.
func failClosedEventLogPath() string {
	return testutils.EventLogPath("governance-fail-closed.log")
}
