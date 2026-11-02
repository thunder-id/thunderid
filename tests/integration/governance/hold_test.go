// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
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
	users *userFixture
}

func TestHoldTestSuite(t *testing.T) {
	suite.Run(t, new(HoldTestSuite))
}

func (ts *HoldTestSuite) SetupSuite() {
	applyPolicy(ts.T(), lockoutPolicyWith(userDefaults(nonExpiringRung())))
	ts.users = newUserFixture(ts.T(), "hold",
		testutils.UserType{Schema: credentialsSchema(map[string]interface{}{"nickname": stringAttribute})})
}

// newUser creates an account no other test touches, with its name as its nickname.
func (ts *HoldTestSuite) newUser(name string) (userID, username, password string) {
	ts.T().Helper()
	userID, username, password = ts.users.newUser(ts.T(), name, map[string]interface{}{"nickname": name})
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
	requireStatedHold(ts.T(), wrong, suspended, statedSuspensionCode, "credential")
	ts.NotContains(suspended.body, statedLockCode, "a suspension must not be stated as a lock")
}

// With automatic locking switched off, a wrong password on a suspended account still states the
// suspension and is not counted.
func (ts *HoldTestSuite) TestASuspensionIsStatedWithLockingSwitchedOff() {
	locking := nonExpiringRung()
	locking["enabled"] = false
	applyPolicy(ts.T(), lockoutPolicyWith(userDefaults(locking)))

	userID, username, _ := ts.newUser("locking_off_suspended")
	_, liveUser, _ := ts.newUser("locking_off_live")
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	revision := runtimeRevision(ts.T(), userID)

	wrong := authenticate(ts.T(), "username", liveUser, "WrongSecret1!")
	held := authenticate(ts.T(), "username", username, "WrongSecret1!")

	requireStatedHold(ts.T(), wrong, held, statedSuspensionCode, "credential")
	ts.Equal(revision, runtimeRevision(ts.T(), userID), "a wrong password on a suspended account must not be counted")
}

// The operator's note reaches a management read, and only a management read.
// The note is optional, so a suspension placed without one must also complete.
func (ts *HoldTestSuite) TestTheOperatorNoteReachesAManagementRead() {
	withNote, _, _ := ts.newUser("suspend_note")
	withoutNote, _, _ := ts.newUser("suspend_no_note")

	runAdministrationFlowAs(ts.T(), adminToken(ts.T()), suspendFlowHandle, map[string]string{
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
// Neither write releases the suspension, and once the holds are lifted the written credential is the
// account's.
func (ts *HoldTestSuite) TestASuspendedRecordStaysWritable() {
	userID, username, password := ts.newUser("suspend_writable")

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

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.Require().Equal("ACTIVE", readStatus(ts.T(), userID).Value,
		"the holds must be lifted before the credentials are compared")
	ts.True(authenticate(ts.T(), "username", username, "Remediated-1!").succeeded(),
		"the credential written under the hold must be the account's")
	ts.False(authenticate(ts.T(), "username", username, password).succeeded(),
		"the credential the write replaced must no longer authenticate")
}

func (ts *HoldTestSuite) TestAnUpdateReportsTheHold() {
	userID, username, _ := ts.newUser("update_reports_lock")
	failAuthentication(ts.T(), "username", username, lockThreshold)
	read := readStatus(ts.T(), userID)
	ts.Require().Equal("LOCKED", read.Value, "precondition: failing the password to the threshold locks it")

	status, body := ts.updateNickname(userID, username, "updated_while_locked")
	ts.Require().Equal(http.StatusOK, status, "a locked record's attributes must stay writable: %s", body)
	var updated struct {
		Status statusReport `json:"status"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(body), &updated), body)
	ts.Equal(read, updated.Status, "the update must report the lock as a read does")
	ts.Equal([]string{"credential"}, updated.Status.lockedMethods(), "the update must report what is locked")
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

	answer := attemptAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.Equal(http.StatusOK, answer.status)
	ts.Equal("COMPLETE", answer.flowStatus, "a suspended record must stay remediable")

	ts.Equal("SUSPENDED", readStatus(ts.T(), userID).Value,
		"unlocking must not release the suspension")
	ts.False(authenticate(ts.T(), "username", username, password).succeeded(),
		"the suspension must still withhold authentication")

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	assertTheRecoveryLock(ts.T(), readStatus(ts.T(), userID), "after unlock")
}

// A suspension is reported in preference to a lock, in either order: placed over a live lock,
// and with a full threshold of failures presented while it holds. Every wrong password presented
// while it holds is refused with the suspension stated, never the lock. Releasing it replaces every
// lock entry with the one non-expiring recovery lock, so the earlier credential lock is gone.
func (ts *HoldTestSuite) TestASuspensionIsReportedOverALock() {
	lockFirst, lockFirstUser, _ := ts.newUser("precedence_lock_first")
	holdFirst, holdFirstUser, _ := ts.newUser("precedence_hold_first")

	failAuthentication(ts.T(), "username", lockFirstUser, lockThreshold)
	locked := readStatus(ts.T(), lockFirst)
	ts.Require().Equal("LOCKED", locked.Value, "the account must be locked first")
	ts.Require().Equal([]string{"credential"}, locked.lockedMethods())
	runAdministrationFlow(ts.T(), suspendFlowHandle, lockFirst)
	overLock := authenticate(ts.T(), "username", lockFirstUser, "DefinitelyNotTheSecret!")
	ts.Contains(overLock.body, statedSuspensionCode,
		"a wrong password on a suspended, locked account must state the suspension")
	ts.NotContains(overLock.body, statedLockCode, "the suspension must be stated in preference to the lock")

	runAdministrationFlow(ts.T(), suspendFlowHandle, holdFirst)
	failHeldAuthentication(ts.T(), "username", holdFirstUser, lockThreshold, statedSuspensionCode)

	for name, id := range map[string]string{"lock first": lockFirst, "hold first": holdFirst} {
		held := readStatus(ts.T(), id)
		ts.Equal("SUSPENDED", held.Value, "%s: the suspension must be the status reported", name)
		ts.Require().NotNil(held.Details, "%s: the suspension must report its details", name)
		ts.Equal("ADMINISTRATIVE_SUSPENSION", held.Details.Reason, name)
		ts.NotEmpty(held.Details.Since, "%s: the read must carry when the hold was placed", name)
		ts.Empty(held.Details.LockedMethods, "%s: a suspension reports no lock beside it", name)
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
	return putManagement(ts.T(), "/"+userCollection+"/"+userID, map[string]interface{}{
		"type":       ts.users.userType,
		"ouId":       ts.users.ouID,
		"attributes": map[string]interface{}{"username": username, "nickname": nickname},
	})
}

// assertTheRecoveryLock requires the status a user unsuspend lands on: LOCKED by the suspension's
// release, on the whole account, with no expiry.
func assertTheRecoveryLock(t *testing.T, status statusReport, label string) {
	t.Helper()

	require.Equal(t, "LOCKED", status.Value, "%s: an unsuspended user must land on a lock", label)
	require.NotNil(t, status.Details, "%s: the lock must report its details", label)
	require.Equal(t, "SUSPENSION_RELEASE_LOCK", status.Details.Reason,
		"%s: the lock must be attributed to the suspension's release", label)
	require.Equal(t, []string{"account"}, status.lockedMethods(),
		"%s: the release lock must hold the whole account, replacing every earlier entry", label)
	_, expires := status.expiryOf("account")
	require.False(t, expires, "%s: the release lock must not expire on its own", label)
}
