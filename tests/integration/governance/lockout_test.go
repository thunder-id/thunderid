// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// LockoutTestSuite checks what a wrong secret costs and the two non-recovery exits: the clock and
// an operator. The recovery exit is in recovery_release_test.go. Its rungs outlast every case, except
// in the cases that wait for a lock to lapse, which set rungs of their own.
type LockoutTestSuite struct {
	suite.Suite
	users *userFixture
}

func TestLockoutTestSuite(t *testing.T) {
	suite.Run(t, new(LockoutTestSuite))
}

func (ts *LockoutTestSuite) SetupSuite() {
	applyPolicy(ts.T(), longRungLockoutPolicy())
	ts.users = newUserFixture(ts.T(), "lockout", testutils.UserType{Schema: credentialsSchema(nil)})
}

func (ts *LockoutTestSuite) TestFailuresBelowTheThresholdLeaveTheAccountUsable() {
	_, username, password := ts.users.newUser(ts.T(), "below", nil)

	failAuthentication(ts.T(), "username", username, lockThreshold-1)

	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the correct secret must still authenticate below the threshold")
}

func (ts *LockoutTestSuite) TestTheThresholdRefusesEvenTheCorrectSecret() {
	userID, username, password := ts.users.newUser(ts.T(), "threshold", nil)

	failAuthentication(ts.T(), "username", username, lockThreshold)

	requireRefusedAsHeld(ts.T(), authenticate(ts.T(), "username", username, password), statedLockCode,
		"the correct secret must be refused while the lock is live")

	status := readStatus(ts.T(), userID)
	ts.Equal("LOCKED", status.Value, "the management read must report the lock")
	ts.Contains(status.lockedMethods(), "credential",
		"the lock must be held against the authentication_method that failed")
}

func (ts *LockoutTestSuite) TestASuccessfulAuthenticationClearsTheCount() {
	userID, username, password := ts.users.newUser(ts.T(), "reset", nil)

	failAuthentication(ts.T(), "username", username, lockThreshold-1)
	ts.Require().True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the clearing authentication must itself succeed")

	// One more failure would have been the threshold had the count survived.
	failAuthentication(ts.T(), "username", username, 1)

	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"proof of control must have cleared the count")
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "nothing should be held")
}

// Nothing but status reads runs against the account while it waits. The rung is long enough that the
// lock is still live when its expiry is read.
func (ts *LockoutTestSuite) TestALockLapsesOnItsOwn() {
	applyPolicy(ts.T(), lockoutPolicyWith(userDefaults(map[string]interface{}{
		"lockDurationsSeconds": []int{firstRungSeconds},
	})))
	userID, username, password := ts.users.newUser(ts.T(), "lapse", nil)

	failAuthentication(ts.T(), "username", username, lockThreshold)

	expiry, reported := readStatus(ts.T(), userID).expiryOf("credential")
	ts.Require().True(reported, "a lock that lifts on its own must report when")
	awaitLapse(ts.T(), userID, expiry, rungLapseTolerance)

	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the same correct secret must work once the rung has elapsed")
}

// No sign-in succeeds between the lockouts, since that steps the ladder back to the bottom.
func (ts *LockoutTestSuite) TestRepeatedLockoutsServeLongerRungs() {
	applyLockoutPolicy(ts.T())
	userID, username, _ := ts.users.newUser(ts.T(), "ladder", nil)

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal(firstRungSeconds*time.Second, servedRung(ts.T(), userID, "credential"),
		"the first lockout must serve the first rung")
	awaitLapse(ts.T(), userID, storedExpiry(ts.T(), userID, "credential"), rungLapseTolerance)

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Equal(secondRungSeconds*time.Second, servedRung(ts.T(), userID, "credential"),
		"a second lockout with no proof of control in between must serve the second rung")
}

func (ts *LockoutTestSuite) TestProofOfControlResetsTheLadder() {
	applyLockoutPolicy(ts.T())
	userID, username, password := ts.users.newUser(ts.T(), "ladder_reset", nil)

	failAuthentication(ts.T(), "username", username, lockThreshold)
	awaitLapse(ts.T(), userID, storedExpiry(ts.T(), userID, "credential"), rungLapseTolerance)

	ts.Require().True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the account must be usable again once the rung has lapsed")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Equal(firstRungSeconds*time.Second, servedRung(ts.T(), userID, "credential"),
		"after proof of control the ladder must serve the first rung again")
}

func (ts *LockoutTestSuite) TestAttemptsAgainstAnUnknownAccountLockNobody() {
	userID, username, password := ts.users.newUser(ts.T(), "unknown", nil)

	unknown := "ghost_" + ts.users.suffix
	failAuthentication(ts.T(), "username", unknown, lockThreshold*2)

	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"a real account must be unaffected by failures against a name that does not exist")
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "nothing may be held")
}

// The runtime row's revision moves with every write, so an unchanged revision shows the attempts
// did not extend the lock. Wrong secrets during a lock are in escalation_test.go.
func (ts *LockoutTestSuite) TestACorrectSecretDoesNotExtendTheLock() {
	userID, username, password := ts.users.newUser(ts.T(), "extend", nil)

	failAuthentication(ts.T(), "username", username, lockThreshold)
	revision := runtimeRevision(ts.T(), userID)

	for range 3 {
		requireRefusedAsHeld(ts.T(), authenticate(ts.T(), "username", username, password), statedLockCode,
			"a locked account must refuse the correct secret")
	}

	ts.Equal(revision, runtimeRevision(ts.T(), userID),
		"presenting the correct secret during the lock must write nothing")
	ts.Equal([]string{"credential"}, readStatus(ts.T(), userID).lockedMethods(),
		"the lock must still be the only one held")
}

// The rungs outlast the test, so only the unlock can have released the account.
func (ts *LockoutTestSuite) TestUnlockReleasesTheAccount() {
	userID, username, password := ts.users.newUser(ts.T(), "unlock", nil)

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the account must be locked first")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)

	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"unlock must release the account immediately")
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "nothing may still be held")
}

// The rungs outlast the test, so only the unlock can have released the first lock. No sign-in
// succeeds in between, since that resets the ladder too.
func (ts *LockoutTestSuite) TestUnlockResetsTheLadder() {
	userID, username, _ := ts.users.newUser(ts.T(), "unlock_ladder", nil)

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the account must be locked first")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "the unlock must have released the first lock")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Equal(longFirstRungSeconds*time.Second, servedRung(ts.T(), userID, "credential"),
		"after an unlock the ladder must serve the first rung again, not the next one")
}

func (ts *LockoutTestSuite) TestUnlockingAnUnlockedAccountIsHarmless() {
	userID, username, password := ts.users.newUser(ts.T(), "noop_unlock", nil)

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)

	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "the account must stay active")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the account must stay usable")
}
