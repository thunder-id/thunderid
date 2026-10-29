// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// storedPostSuspensionReason is the reason the runtime document stores on the lock an unsuspend
// leaves; a management read reports it as SUSPENSION_RELEASE_LOCK. A profile planting it must not
// make it reach the runtime row.
const storedPostSuspensionReason = "POST_SUSPENDED"

// runtimeRowSchema carries a profile attribute shaped like the runtime access-state document, so a
// profile can claim a suspension and a lock it does not have.
var runtimeRowSchema = credentialsSchema(map[string]interface{}{
	"accessState": map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"suspend": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"suspendedAt":  stringAttribute,
					"operatorNote": stringAttribute,
				},
			},
			"lock": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"account": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"unlockAt": stringAttribute,
							"reason":   stringAttribute,
						},
					},
				},
			},
		},
	},
})

// RuntimeRowTestSuite checks what the runtime row is and is not built from: an unknown identifier
// allocates nothing, a management read writes nothing, the governance read behind a sign-in attempt
// or an unlock seeds a missing row, and a profile attribute named like the runtime document governs
// nothing. It runs under the shipped policy. The row itself is read through the runtime database CLI
// (sqlite3 or psql).
type RuntimeRowTestSuite struct {
	suite.Suite
	users *userFixture
}

func TestRuntimeRowTestSuite(t *testing.T) {
	suite.Run(t, new(RuntimeRowTestSuite))
}

func (ts *RuntimeRowTestSuite) SetupSuite() {
	ts.users = newUserFixture(ts.T(), "runtime_row", testutils.UserType{Schema: runtimeRowSchema})
}

// Failed sign-ins for an identifier that names no account allocate no runtime row. Creating an account
// in the same span allocates exactly one, so the count is seen to move with an allocation and with
// nothing else.
func (ts *RuntimeRowTestSuite) TestAnUnknownIdentifierAllocatesNoRuntimeRow() {
	before := ts.allRuntimeRows()
	failAuthentication(ts.T(), "username", "no_such_user_"+ts.users.suffix, 6)
	ts.users.newUser(ts.T(), "allocation_control", nil)
	ts.Equal(before+1, ts.allRuntimeRows(),
		"only the account created alongside the failures may have allocated a runtime row")
}

func (ts *RuntimeRowTestSuite) TestAManagementReadWritesNothing() {
	userID, username, _ := ts.newRowlessUser("read")

	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "a missing row must read as the starting state")

	var found struct {
		Users []struct {
			ID     string                  `json:"id"`
			Status *testutils.AccessStatus `json:"status"`
		} `json:"users"`
	}
	ts.Require().NoError(json.Unmarshal(getManagement(ts.T(),
		"/users?filter="+url.QueryEscape(fmt.Sprintf(`username eq %q`, username))), &found),
		"unreadable user search response")
	ts.Require().Len(found.Users, 1, "the search must find the user")
	ts.Equal(userID, found.Users[0].ID, "the search must find the user")
	ts.Require().NotNil(found.Users[0].Status, "a listed user must carry a status")
	ts.Equal("ACTIVE", found.Users[0].Status.Value, "a missing row must list as the starting state")

	ts.Equal(0, runtimeRowCount(ts.T(), userID), "management reads must write nothing")
}

func (ts *RuntimeRowTestSuite) TestASignInSeedsAMissingRow() {
	userID, username, password := ts.newRowlessUser("seed")

	ts.Require().True(authenticate(ts.T(), "username", username, password).succeeded(),
		"a sign-in must be admitted on a missing row")
	ts.Equal(1, runtimeRowCount(ts.T(), userID), "the governance read behind a sign-in must seed the row")
	ts.Equal("ACTIVE", runtimeColumn(ts.T(), userID, "STATE"), "a database identity's row must be seeded ACTIVE")
}

func (ts *RuntimeRowTestSuite) TestFailedSignInsSeedAMissingRowAndLock() {
	userID, username, _ := ts.newRowlessUser("seed_failures")

	failAuthentication(ts.T(), "username", username, shippedUserLockThreshold)

	ts.Equal("LOCKED", readStatus(ts.T(), userID).Value, "failures on a missing row must still lock")
	ts.Equal(1, runtimeRowCount(ts.T(), userID), "the first failure must seed one row and the rest count in it")
}

func (ts *RuntimeRowTestSuite) TestAnUnlockSeedsAMissingRow() {
	userID, _, _ := ts.newRowlessUser("seed_unlock")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)

	ts.Equal(1, runtimeRowCount(ts.T(), userID), "the governance read behind an unlock must seed the row")
	ts.Equal("ACTIVE", runtimeColumn(ts.T(), userID, "STATE"), "an unlock must seed the row ACTIVE")
}

func (ts *RuntimeRowTestSuite) TestAnAccessStateProfileAttributeGovernsNothing() {
	claimed := map[string]interface{}{
		"suspend": map[string]interface{}{
			"suspendedAt": "2026-01-01T00:00:00Z", "operatorNote": "claimed by the profile",
		},
		"lock": map[string]interface{}{
			"account": map[string]interface{}{
				"unlockAt": "9999-12-31T23:59:59Z", "reason": storedPostSuspensionReason,
			},
		},
	}
	userID, username, password := ts.users.newUser(ts.T(), "claimed",
		map[string]interface{}{"accessState": claimed})

	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "the profile attribute must not be a hold")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the profile attribute must not refuse a sign-in")
	ts.Contains(ts.userAttributes(userID), "claimed by the profile",
		"the attribute must be stored as ordinary profile data")

	ts.Require().NoError(testutils.DeleteRuntimeRow(userID))
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"a row seeded with the attribute present must not refuse a sign-in")
	row := runtimeColumn(ts.T(), userID, `STATE || '|' || RUNTIME_ATTRIBUTES`)
	ts.Contains(row, "ACTIVE|", "the seeded row must start ACTIVE")
	ts.NotContains(row, "suspend", "the seeded row must take no suspension from the profile")
	ts.NotContains(row, storedPostSuspensionReason, "the seeded row must take no lock from the profile")
}

// newRowlessUser creates an account and deletes the runtime row its creation wrote, so it reads as
// one governance has never acted on.
func (ts *RuntimeRowTestSuite) newRowlessUser(label string) (userID, username, password string) {
	ts.T().Helper()

	userID, username, password = ts.users.newUser(ts.T(), label, nil)
	ts.Require().Equal(1, runtimeRowCount(ts.T(), userID), "creating an identity must create its runtime row")
	ts.Require().NoError(testutils.DeleteRuntimeRow(userID))
	ts.Require().Equal(0, runtimeRowCount(ts.T(), userID), "the runtime row must be gone")
	return userID, username, password
}

// allRuntimeRows counts every runtime row. Packages run one at a time, so only this package writes
// rows while it runs.
func (ts *RuntimeRowTestSuite) allRuntimeRows() int {
	ts.T().Helper()

	out, err := testutils.QueryRuntimeDB(`SELECT COUNT(*) FROM "ENTITY_RUNTIME_DATA";`)
	ts.Require().NoError(err)
	count, err := strconv.Atoi(out)
	ts.Require().NoError(err)
	return count
}

// userAttributes returns the raw profile attributes a management read returns.
func (ts *RuntimeRowTestSuite) userAttributes(userID string) string {
	ts.T().Helper()

	var user struct {
		Attributes json.RawMessage `json:"attributes"`
	}
	ts.Require().NoError(json.Unmarshal(getManagement(ts.T(), "/users/"+userID), &user))
	return string(user.Attributes)
}
