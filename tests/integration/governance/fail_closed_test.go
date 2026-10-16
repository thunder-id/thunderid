// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
)

// FailClosedTestSuite checks that governance refuses when it cannot read what it needs, with the
// answer the path gives for an unreadable entity, never as a hold or a wrong credential; and that a
// suspension whose containment fails is not placed. It runs under the shipped policy.
type FailClosedTestSuite struct {
	suite.Suite
	ouID         string
	entityTypeID string
	appID        string
	userIDs      []string
	suffix       string

	// originalDatabaseConfig is the deployment's database section, captured before it is broken.
	originalDatabaseConfig map[string]interface{}
	configBroken           bool
}

func TestFailClosedTestSuite(t *testing.T) {
	suite.Run(t, new(FailClosedTestSuite))
}

var failClosedOU = testutils.OrganizationUnit{
	Handle:      "governance_fail_closed_ou",
	Name:        "Governance Fail Closed Test OU",
	Description: "Organization unit for the governance fail-closed tests",
}

var failClosedUserType = testutils.UserType{
	Handle:      "governance_fail_closed_user",
	DisplayName: "governance_fail_closed_user",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

func (ts *FailClosedTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(failClosedOU)
	ts.Require().NoError(err, "failed to create the test organization unit")
	ts.ouID = ouID

	failClosedUserType.OUID = ts.ouID
	typeID, err := testutils.CreateUserType(failClosedUserType)
	ts.Require().NoError(err, "failed to create the test user type")
	ts.entityTypeID = typeID

	ts.appID = ts.createClient()
	ts.suffix = uniqueSuffix()
}

func (ts *FailClosedTestSuite) TearDownSuite() {
	// Safety net: each test restores the database in a deferred block.
	ts.restoreRuntimePersistentDB()

	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("teardown: failed to delete application: %v", err)
		}
	}
	for _, userID := range ts.userIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("teardown: failed to delete user %s: %v", userID, err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("teardown: failed to delete user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("teardown: failed to delete organization unit: %v", err)
		}
	}
}

// At sign-in, an identity whose runtime document cannot be read is refused with a server error,
// not with a wrong credential's answer or a stated hold, and a management read reports it suspended.
// Restoring the document shows the refusal came from it.
func (ts *FailClosedTestSuite) TestAnUnreadableRuntimeRecordRefusesSignInAsAServerError() {
	userID, username, password := ts.newUser("unreadable")
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

// At OAuth client authentication, an application whose runtime document cannot be read is
// refused as invalid_client, the answer for an unreadable client.
func (ts *FailClosedTestSuite) TestAnUnreadableApplicationRecordRefusesClientAuthentication() {
	result := clientCredentials(ts.T(), failClosedGovernanceClientID, failClosedGovernanceClientSecret)
	ts.Require().Equal(http.StatusOK, result.StatusCode,
		"the client must authenticate while its record is readable: %s", result.Body)

	ts.corruptRuntimeRecord(ts.appID)
	defer ts.repairRuntimeRecord(ts.appID)

	result = clientCredentials(ts.T(), failClosedGovernanceClientID, failClosedGovernanceClientSecret)
	ts.Equal(http.StatusUnauthorized, result.StatusCode, "an unreadable client record must refuse: %s", result.Body)
	ts.Contains(string(result.Body), "invalid_client", "the refusal must be invalid_client")

	ts.repairRuntimeRecord(ts.appID)
	result = clientCredentials(ts.T(), failClosedGovernanceClientID, failClosedGovernanceClientSecret)
	ts.Equal(http.StatusOK, result.StatusCode,
		"the client must authenticate again once its record is readable: %s", result.Body)
}

// Containment runs before the hold: when the revocation store cannot be written, the suspend
// flow fails and the account is not suspended, and Suspend stays available once the store is back.
func (ts *FailClosedTestSuite) TestAFailedContainmentLeavesTheAccountActive() {
	userID, username, password := ts.newUser("containment")

	defer ts.restoreRuntimePersistentDB()
	ts.breakRevocationStore()

	status, flowStatus := attemptAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.False(status == http.StatusOK && flowStatus == "COMPLETE",
		"a suspend whose revocation step fails must not complete")
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "no hold may be written after a failed containment")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the account must still be usable")

	ts.restoreRuntimePersistentDB()
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.Equal("SUSPENDED", readStatus(ts.T(), userID).Value,
		"Suspend must succeed once the store is back")
}

// --- fixtures and helpers ---

func (ts *FailClosedTestSuite) newUser(name string) (userID, username, password string) {
	ts.T().Helper()

	username = fmt.Sprintf("fail_closed_%s_%s", name, ts.suffix)
	password = "Correct-Horse-" + name + "1!"
	attributes, err := json.Marshal(map[string]interface{}{"username": username, "password": password})
	ts.Require().NoError(err)

	userID, err = testutils.CreateUser(testutils.User{
		Type:       failClosedUserType.Handle,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "failed to create the test user")
	ts.userIDs = append(ts.userIDs, userID)
	return userID, username, password
}

// corruptRuntimeRecord stores a document that is valid JSON, so the table accepts it, but not an
// access-state document, so the server cannot read it.
func (ts *FailClosedTestSuite) corruptRuntimeRecord(entityID string) {
	ts.T().Helper()

	count, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`SELECT COUNT(*) FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = '%s';`, entityID))
	ts.Require().NoError(err)
	ts.Require().Equal("1", count, "the entity must have a runtime row to corrupt")
	_, err = testutils.QueryRuntimeDB(fmt.Sprintf(
		`UPDATE "ENTITY_RUNTIME_DATA" SET RUNTIME_ATTRIBUTES = '{"accessState":"unreadable"}' WHERE ENTITY_ID = '%s';`,
		entityID))
	ts.Require().NoError(err, "failed to corrupt the runtime record")
}

// repairRuntimeRecord puts an empty document back. Safe to call twice.
func (ts *FailClosedTestSuite) repairRuntimeRecord(entityID string) {
	if _, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`UPDATE "ENTITY_RUNTIME_DATA" SET RUNTIME_ATTRIBUTES = '{}' WHERE ENTITY_ID = '%s';`, entityID)); err != nil {
		ts.T().Errorf("failed to repair the runtime record of %s: %v", entityID, err)
	}
}

// breakRevocationStore points runtime_persistent at a database holding only the runtime table and
// restarts. The admin token is not re-obtained: it was issued before and stays valid across the
// restart, while issuing a new one may need a store this database lacks.
func (ts *FailClosedTestSuite) breakRevocationStore() {
	original, err := testutils.ReadDeploymentConfigKey("database")
	ts.Require().NoError(err, "failed to read the database section of deployment.yaml")
	originalMap, ok := original.(map[string]interface{})
	ts.Require().True(ok, "deployment.yaml has no database section")
	ts.originalDatabaseConfig = originalMap

	ts.Require().NoError(testutils.CreateEntityRuntimeOnlyDB(noRevocationStorePath),
		"failed to create the database without a revocation store")

	broken := make(map[string]interface{}, len(originalMap))
	for key, value := range originalMap {
		broken[key] = value
	}
	broken["runtime_persistent"] = map[string]interface{}{
		"type":   "sqlite",
		"sqlite": map[string]interface{}{"path": noRevocationStorePath},
	}

	ts.configBroken = true
	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{"database": broken}),
		"failed to point runtime_persistent at the database without a revocation store")
	ts.Require().NoError(testutils.RestartServer(), "the server must boot without a revocation store")
}

// restoreRuntimePersistentDB puts the captured database section back, restarts and re-obtains the admin
// token. Every step runs even if one fails, so the server is left healthy for later packages.
func (ts *FailClosedTestSuite) restoreRuntimePersistentDB() {
	if !ts.configBroken {
		return
	}
	ts.Assert().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		"database": ts.originalDatabaseConfig,
	}), "failed to restore the database section of deployment.yaml")
	ts.Assert().NoError(testutils.RestartServer(), "failed to restart after the database restore")
	ts.Assert().NoError(testutils.ObtainAdminAccessToken(), "failed to re-obtain the admin token")
	ts.configBroken = false
}

// createClient registers a confidential client that authenticates with client_credentials.
func (ts *FailClosedTestSuite) createClient() string {
	payload, err := json.Marshal(map[string]interface{}{
		"name":                      "Governance Fail Closed Client",
		"description":               "Client for the governance fail-closed tests",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{{
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
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/applications", bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, "create application failed: %s", string(body))

	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &created))
	ts.Require().NotEmpty(created.ID, "application id missing from the create response")
	return created.ID
}
