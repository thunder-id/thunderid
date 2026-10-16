// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// RuntimeRowTestSuite checks what the runtime row is and is not built from: an unknown identifier
// allocates nothing, a management read writes nothing, a sign-in seeds a missing row, and a profile
// attribute named like the runtime document governs nothing. It runs under the shipped policy. The
// row itself is read through the runtime database CLI (sqlite3 or psql).
type RuntimeRowTestSuite struct {
	suite.Suite
	ouID         string
	entityTypeID string
	userIDs      []string
	suffix       string
}

func TestRuntimeRowTestSuite(t *testing.T) {
	suite.Run(t, new(RuntimeRowTestSuite))
}

var runtimeRowOU = testutils.OrganizationUnit{
	Handle:      "governance_runtime_row_ou",
	Name:        "Governance Runtime Row Test OU",
	Description: "Organization unit for the runtime row acceptance tests",
}

// runtimeRowUserType carries a profile attribute shaped like the runtime access-state document, so a
// profile can claim a suspension and a lock it does not have.
var runtimeRowUserType = testutils.UserType{
	Handle:      "governance_runtime_row_user",
	DisplayName: "governance_runtime_row_user",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"accessState": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"suspend": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"suspendedAt":  map[string]interface{}{"type": "string"},
						"operatorNote": map[string]interface{}{"type": "string"},
					},
				},
				"lock": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"entity": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"unlockAt": map[string]interface{}{"type": "string"},
								"reason":   map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
		},
	},
}

func (ts *RuntimeRowTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(runtimeRowOU)
	ts.Require().NoError(err, "failed to create the test organization unit")
	ts.ouID = ouID

	runtimeRowUserType.OUID = ts.ouID
	typeID, err := testutils.CreateUserType(runtimeRowUserType)
	ts.Require().NoError(err, "failed to create the test user type")
	ts.entityTypeID = typeID

	ts.suffix = uniqueSuffix()
}

func (ts *RuntimeRowTestSuite) TearDownSuite() {
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

// Failed sign-ins for an identifier that names no account allocate no runtime row.
func (ts *RuntimeRowTestSuite) TestAnUnknownIdentifierAllocatesNoRuntimeRow() {
	before := ts.runtimeRowCount("")
	failAuthentication(ts.T(), "username", "no_such_user_"+ts.suffix, 6)
	ts.Equal(before, ts.runtimeRowCount(""), "an unknown identifier must allocate no runtime row")
}

// A management read of an identity with no runtime row reports the starting
// state and writes nothing; the governance read behind a sign-in seeds the row.
func (ts *RuntimeRowTestSuite) TestAManagementReadWritesNothingAndASignInSeedsTheRow() {
	userID, username, password := ts.newUser("seed", nil)
	ts.Require().Equal(1, ts.runtimeRowCount(userID), "creating an identity must create its runtime row")
	ts.queryRuntimeDB(fmt.Sprintf(`DELETE FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = '%s';`, userID))

	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "a missing row must read as the starting state")
	ts.listUsers()
	ts.Equal(0, ts.runtimeRowCount(userID), "management reads must write nothing")

	ts.Require().True(authenticate(ts.T(), "username", username, password).succeeded(),
		"a sign-in must be admitted on a missing row")
	ts.Equal(1, ts.runtimeRowCount(userID), "the governance read behind a sign-in must seed the row")
	ts.Equal("ACTIVE", ts.queryRuntimeDB(fmt.Sprintf(
		`SELECT STATE FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = '%s';`, userID)),
		"a database identity's row must be seeded ACTIVE")
}

// A profile attribute named accessState is ordinary profile data: it is stored as written, it
// neither suspends nor locks, and a row seeded later takes nothing from it.
func (ts *RuntimeRowTestSuite) TestAnAccessStateProfileAttributeGovernsNothing() {
	claimed := map[string]interface{}{
		"suspend": map[string]interface{}{
			"suspendedAt": "2026-01-01T00:00:00Z", "operatorNote": "claimed by the profile",
		},
		"lock": map[string]interface{}{
			"entity": map[string]interface{}{"unlockAt": "9999-12-31T23:59:59Z", "reason": "POST_SUSPENDED"},
		},
	}
	userID, username, password := ts.newUser("claimed", claimed)

	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "the profile attribute must not be a hold")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the profile attribute must not refuse a sign-in")
	ts.Contains(ts.userAttributes(userID), "claimed by the profile",
		"the attribute must be stored as ordinary profile data")

	ts.queryRuntimeDB(fmt.Sprintf(`DELETE FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = '%s';`, userID))
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"a row seeded with the attribute present must not refuse a sign-in")
	row := ts.queryRuntimeDB(fmt.Sprintf(
		`SELECT STATE || '|' || RUNTIME_ATTRIBUTES FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = '%s';`, userID))
	ts.Contains(row, "ACTIVE|", "the seeded row must start ACTIVE")
	ts.NotContains(row, "suspend", "the seeded row must take no suspension from the profile")
	ts.NotContains(row, "POST_SUSPENDED", "the seeded row must take no lock from the profile")
}

// --- helpers ---

func (ts *RuntimeRowTestSuite) newUser(name string, accessState map[string]interface{}) (
	userID, username, password string) {
	ts.T().Helper()

	username = fmt.Sprintf("runtime_row_%s_%s", name, ts.suffix)
	password = "Correct-Horse-" + name + "1!"
	attributes := map[string]interface{}{"username": username, "password": password}
	if accessState != nil {
		attributes["accessState"] = accessState
	}
	raw, err := json.Marshal(attributes)
	ts.Require().NoError(err)

	userID, err = testutils.CreateUser(testutils.User{
		Type:       runtimeRowUserType.Handle,
		OUID:       ts.ouID,
		Attributes: raw,
	})
	ts.Require().NoError(err, "failed to create the test user")
	ts.userIDs = append(ts.userIDs, userID)
	return userID, username, password
}

func (ts *RuntimeRowTestSuite) queryRuntimeDB(query string) string {
	ts.T().Helper()
	out, err := testutils.QueryRuntimeDB(query)
	ts.Require().NoError(err)
	return out
}

// runtimeRowCount counts runtime rows, for one entity or, with an empty ID, all of them.
func (ts *RuntimeRowTestSuite) runtimeRowCount(entityID string) int {
	ts.T().Helper()

	query := `SELECT COUNT(*) FROM "ENTITY_RUNTIME_DATA";`
	if entityID != "" {
		query = fmt.Sprintf(`SELECT COUNT(*) FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = '%s';`, entityID)
	}
	count, err := strconv.Atoi(ts.queryRuntimeDB(query))
	ts.Require().NoError(err)
	return count
}

// listUsers performs a management list read.
func (ts *RuntimeRowTestSuite) listUsers() {
	ts.T().Helper()

	resp, err := testutils.GetHTTPClient().Get(testServerURL + "/users?limit=100")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "the user list must be readable")
}

// userAttributes returns the raw profile attributes a management read returns.
func (ts *RuntimeRowTestSuite) userAttributes(userID string) string {
	ts.T().Helper()

	resp, err := testutils.GetHTTPClient().Get(testServerURL + "/users/" + userID)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "the user must be readable: %s", string(body))

	var user struct {
		Attributes json.RawMessage `json:"attributes"`
	}
	ts.Require().NoError(json.Unmarshal(body, &user))
	return string(user.Attributes)
}
