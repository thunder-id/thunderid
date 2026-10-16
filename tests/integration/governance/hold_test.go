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

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// HoldTestSuite checks the administrative holds: what each withholds, and that they are a separate
// dimension from a lock. Its locks never lapse on their own, so a status read after a lock is
// formed cannot pass because the lock expired in between.
type HoldTestSuite struct {
	suite.Suite
	ouID         string
	entityTypeID string
	userIDs      []string
	suffix       string
}

func TestHoldTestSuite(t *testing.T) {
	suite.Run(t, new(HoldTestSuite))
}

var holdTestOU = testutils.OrganizationUnit{
	Handle:      "governance_hold_ou",
	Name:        "Governance Hold Test OU",
	Description: "Organization unit for administrative hold acceptance tests",
}

var holdUserType = testutils.UserType{
	Handle:      "governance_hold_user",
	DisplayName: "governance_hold_user",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"nickname": map[string]interface{}{"type": "string"},
	},
}

// holdSuitePolicy is the package policy with one non-expiring rung.
func holdSuitePolicy() interface{} {
	policy := accountAccessPatch()["accountAccess"].(map[string]interface{})
	user := policy["user"].(map[string]interface{})
	user["default"].(map[string]interface{})["lockDurationsSeconds"] = []int{0}
	return policy
}

func (ts *HoldTestSuite) SetupSuite() {
	applyPolicy(ts.T(), holdSuitePolicy())

	ouID, err := testutils.CreateOrganizationUnit(holdTestOU)
	ts.Require().NoError(err, "failed to create the test organization unit")
	ts.ouID = ouID

	holdUserType.OUID = ts.ouID
	typeID, err := testutils.CreateUserType(holdUserType)
	ts.Require().NoError(err, "failed to create the test user type")
	ts.entityTypeID = typeID

	ts.suffix = uniqueSuffix()
}

func (ts *HoldTestSuite) TearDownSuite() {
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

func (ts *HoldTestSuite) newUser(name string) (userID, username, password string) {
	ts.T().Helper()

	username = fmt.Sprintf("hold_%s_%s", name, ts.suffix)
	password = "Correct-Horse-" + name + "1!"
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username,
		"password": password,
		"nickname": name,
	})
	ts.Require().NoError(err)

	userID, err = testutils.CreateUser(testutils.User{
		Type:       holdUserType.Handle,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "failed to create the test user")
	ts.userIDs = append(ts.userIDs, userID)
	return userID, username, password
}

// Suspend withholds authentication; unsuspend releases the hold; the read says which.
// Unsuspend does not hand the account straight back, so the status is checked, not a sign-in.
func (ts *HoldTestSuite) TestSuspendWithholdsAuthentication() {
	userID, username, password := ts.newUser("suspend")
	_, liveUser, _ := ts.newUser("suspend_live")

	ts.Require().True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the account must work before it is suspended")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	suspended := authenticate(ts.T(), "username", username, password)
	wrong := authenticate(ts.T(), "username", liveUser, "WrongSecret1!")

	ts.False(suspended.succeeded(), "a suspended account must not authenticate")
	ts.Equal("SUSPENDED", readStatus(ts.T(), userID).Value, "the read must report the hold")

	// The refusal states the suspension, with the status a refused sign-in has.
	requireStatedHold(ts.T(), wrong, suspended, statedSuspensionCode)
	ts.NotContains(suspended.body, statedLockCode, "a suspension must not be stated as a lock")
}

// The operator's note reaches a management read, and only a management read.
// The note is optional, so a suspension placed without one must also complete.
func (ts *HoldTestSuite) TestTheOperatorNoteReachesAManagementRead() {
	withNote, _, _ := ts.newUser("suspend_note")
	withoutNote, _, _ := ts.newUser("suspend_no_note")

	runAdministrationFlowWithInputs(ts.T(), suspendFlowHandle, map[string]string{
		subjectInput:      withNote,
		operatorNoteInput: "Credential exposure reported by the security team",
	})
	runAdministrationFlow(ts.T(), suspendFlowHandle, withoutNote)

	held := readStatus(ts.T(), withNote)
	ts.Equal("SUSPENDED", held.Value)
	ts.Require().NotNil(held.Details, "a suspension must report what produced it")
	ts.Equal("ADMINISTRATIVE_SUSPENSION", held.Details.Reason)
	ts.Equal("Credential exposure reported by the security team", held.Details.OperatorNote,
		"the operator's own words must reach the one audience entitled to read them")
	ts.NotEmpty(held.Details.Since, "a hold must report when it was placed")

	silent := readStatus(ts.T(), withoutNote)
	ts.Equal("SUSPENDED", silent.Value, "an operator note is optional, not required")
	ts.Require().NotNil(silent.Details)
	ts.Empty(silent.Details.OperatorNote, "no operator note given means none reported")
}

// A suspended record stays writable, and that is the whole point of the state: the hold
// is the window in which an operator remediates a suspected compromise, and remediating means writing.
// Neither write releases the suspension.
func (ts *HoldTestSuite) TestASuspendedRecordStaysWritable() {
	userID, username, _ := ts.newUser("suspend_writable")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	ts.Require().NoError(testutils.UpdateUserCredentials(userID, map[string]string{"password": "Remediated-1!"}),
		"a suspended record's credential must stay writable so an operator can remediate")
	ts.Equal("SUSPENDED", readStatus(ts.T(), userID).Value, "a credential change must not release the hold")

	status, body := ts.updateNickname(userID, username, "remediated")
	ts.Require().Equal(http.StatusOK, status, "a suspended record's attributes must stay writable: %s", body)
	ts.Contains(body, "remediated", "the attribute write must have been applied")
	ts.Equal("SUSPENDED", readStatus(ts.T(), userID).Value, "an attribute write must not release the hold")
	ts.False(authenticate(ts.T(), "username", username, "Remediated-1!").succeeded(),
		"the new credential must not authenticate while the hold is on")
}

// Unlock is not refused on a suspended identity, and it does not release the suspension: the
// release still has to be an unsuspend, which then lands on its own lock. The lock never lapses
// here, so it is still in force when the suspension is placed over it.
func (ts *HoldTestSuite) TestUnlockOnASuspendedIdentityDoesNotRelease() {
	userID, username, password := ts.newUser("unlock_under_suspension")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the account must be locked first")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.Require().Equal("SUSPENDED", readStatus(ts.T(), userID).Value,
		"a suspension is reported in preference to the lock underneath it")

	status, flowStatus := attemptAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.Equal(http.StatusOK, status)
	ts.Equal("COMPLETE", flowStatus, "a suspended record must stay remediable")

	ts.Equal("SUSPENDED", readStatus(ts.T(), userID).Value,
		"unlocking must not release the suspension")
	ts.False(authenticate(ts.T(), "username", username, password).succeeded(),
		"the suspension must still withhold authentication")

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	assertTheRecoveryLock(ts.T(), readStatus(ts.T(), userID), "after unlock")
}

// A suspension is reported in preference to a lock, in either order: placed over a live lock,
// and with a full threshold of failures presented while it holds. Releasing it replaces every lock
// entry with the one non-expiring recovery lock, so the earlier credential lock is gone.
func (ts *HoldTestSuite) TestASuspensionIsReportedOverALock() {
	lockFirst, lockFirstUser, _ := ts.newUser("precedence_lock_first")
	holdFirst, holdFirstUser, _ := ts.newUser("precedence_hold_first")

	failAuthentication(ts.T(), "username", lockFirstUser, lockThreshold)
	locked := readStatus(ts.T(), lockFirst)
	ts.Require().Equal("LOCKED", locked.Value, "the account must be locked first")
	ts.Require().Equal([]string{"credential"}, locked.lockedScopes())
	runAdministrationFlow(ts.T(), suspendFlowHandle, lockFirst)

	runAdministrationFlow(ts.T(), suspendFlowHandle, holdFirst)
	failAuthentication(ts.T(), "username", holdFirstUser, lockThreshold)

	for name, id := range map[string]string{"lock first": lockFirst, "hold first": holdFirst} {
		held := readStatus(ts.T(), id)
		ts.Equal("SUSPENDED", held.Value, "%s: the suspension must be the status reported", name)
		if ts.NotNil(held.Details, "%s: the suspension must report its details", name) {
			ts.Equal("ADMINISTRATIVE_SUSPENSION", held.Details.Reason, name)
			ts.NotEmpty(held.Details.Since, "%s: the read must carry when the hold was placed", name)
			ts.Empty(held.Details.LockedScopes, "%s: a suspension reports no lock beside it", name)
		}
	}

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, lockFirst)
	assertTheRecoveryLock(ts.T(), readStatus(ts.T(), lockFirst), "lock first")
}

// Unsuspending does not hand the account straight back: it lands on a non-expiring account-wide
// lock that only proof of control or an operator clears.
func (ts *HoldTestSuite) TestUnsuspendDoesNotHandTheAccountBack() {
	userID, username, password := ts.newUser("unsuspend")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)

	assertTheRecoveryLock(ts.T(), readStatus(ts.T(), userID), "unsuspended")
	ts.False(authenticate(ts.T(), "username", username, password).succeeded(),
		"an unsuspended account must still be held until control is proven")
}

// updateNickname replaces the user's attributes through the management API and returns the status
// and the body the write answered with.
func (ts *HoldTestSuite) updateNickname(userID, username, nickname string) (int, string) {
	ts.T().Helper()

	payload, err := json.Marshal(map[string]interface{}{
		"type":       holdUserType.Handle,
		"ouId":       ts.ouID,
		"attributes": map[string]interface{}{"username": username, "nickname": nickname},
	})
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPut, testServerURL+"/users/"+userID, bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "the update request failed")
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, string(body)
}

// assertTheRecoveryLock requires the status a user unsuspend lands on: LOCKED by the suspension's
// release, on the whole account, with no expiry.
func assertTheRecoveryLock(t *testing.T, status statusReport, label string) {
	t.Helper()

	require.Equal(t, "LOCKED", status.Value, "%s: an unsuspended user must land on a lock", label)
	require.NotNil(t, status.Details, "%s: the lock must report its details", label)
	require.Equal(t, "SUSPENSION_RELEASE_LOCK", status.Details.Reason,
		"%s: the lock must be attributed to the suspension's release", label)
	require.Equal(t, []string{"entity"}, status.lockedScopes(),
		"%s: the release lock must hold the whole account, replacing every earlier entry", label)
	_, expires := status.expiryOf("entity")
	require.False(t, expires, "%s: the release lock must not expire on its own", label)
}
