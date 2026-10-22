// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// unreadableEntitySignIn is the shipped sign-in flow's answer when the account cannot be read.
var unreadableEntitySignIn = flowAnswer{status: http.StatusOK, flowStatus: "ERROR", code: "FET-1006"}

// FailClosedTestSuite checks that governance refuses when it cannot read what it needs, with the
// answer the path gives for an unreadable entity, never as a hold or a wrong credential. It runs under
// the shipped policy.
type FailClosedTestSuite struct {
	suite.Suite
	users        *userFixture
	shippedAppID string
}

func TestFailClosedTestSuite(t *testing.T) {
	suite.Run(t, new(FailClosedTestSuite))
}

func (ts *FailClosedTestSuite) SetupSuite() {
	ts.users = newUserFixture(ts.T(), "fail_closed", testutils.UserType{Schema: credentialsSchema(nil)})
	ts.shippedAppID = newShippedFlowApp(ts.T(), ts.users, "fail_closed")
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
