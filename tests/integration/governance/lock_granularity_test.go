// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// LockGranularityTestSuite checks one counter for the whole identity, and a rung the clock
// never lifts. Both are policy the section applies at once, and neither affects the other.
type LockGranularityTestSuite struct {
	suite.Suite
	users     *userFixture
	federated *federatedFixture
	mfa       *mfaFixture
}

func TestLockGranularityTestSuite(t *testing.T) {
	suite.Run(t, new(LockGranularityTestSuite))
}

func (ts *LockGranularityTestSuite) SetupSuite() {
	// One counter for the whole identity, whose single rung is `0`: a lock the clock never lifts.
	applyPolicy(ts.T(), lockoutPolicyWith(map[string]interface{}{"user": map[string]interface{}{
		"lockGranularity": "entity",
		"default":         nonExpiringRung(),
	}}))

	ts.users = newUserFixture(ts.T(), "granularity", testutils.UserType{Schema: credentialsSchema(
		map[string]interface{}{"mobile_number": stringAttribute, "email": stringAttribute})})
	ts.federated = newFederatedFixture(ts.T(), "granularity", ts.users.ouID, ts.users.userType)
	ts.mfa = newMFAFixture(ts.T(), "granularity", ts.users.ouID, ts.users.userType)
}

// The lock is reported under the reserved `entity` method, since it refuses the identity rather than
// the password. The direct send is not governed, so the held account still receives a code to present.
func (ts *LockGranularityTestSuite) TestEntityGranularityHoldsEveryAuthenticationMethod() {
	userID, username, mobile, _ := ts.newUser("entity_wide", 1)
	_, _, liveMobile, _ := ts.newUser("entity_wide_live", 7)

	failAuthentication(ts.T(), "username", username, lockThreshold)

	status := readStatus(ts.T(), userID)
	ts.Require().Equal("LOCKED", status.Value, "failing to the threshold must form a lock")
	ts.Equal([]string{"entity"}, status.lockedMethods(),
		"under entity granularity the lock must be held against the identity, not one method")

	heldSession, heldCode, sendStatus := sendDirectOTP(ts.T(), ts.mfa.server, ts.mfa.senderID, mobile)
	ts.Require().Equal(http.StatusOK, sendStatus, "the direct send must accept the request")
	liveSession, liveCode, sendStatus := sendDirectOTP(ts.T(), ts.mfa.server, ts.mfa.senderID, liveMobile)
	ts.Require().Equal(http.StatusOK, sendStatus, "the direct send must accept the request")

	held := verifyDirectOTP(ts.T(), heldSession, heldCode)
	wrong := verifyDirectOTP(ts.T(), liveSession, wrongCodeFor(liveCode))

	requireStatedHold(ts.T(), wrong, held, statedLockCode, "entity")
}

func (ts *LockGranularityTestSuite) TestAWrongPasswordOnAnEntityLockStatesTheLockWithoutCounting() {
	userID, username, _, _ := ts.newUser("entity_wrong", 8)
	_, liveName, _, _ := ts.newUser("entity_wrong_live", 9)

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal([]string{"entity"}, readStatus(ts.T(), userID).lockedMethods(),
		"the entity must be locked")
	revision := runtimeRevision(ts.T(), userID)

	wrong := authenticate(ts.T(), "username", liveName, "Not-The-Secret-8!")
	held := authenticate(ts.T(), "username", username, "Not-The-Secret-8!")

	requireStatedHold(ts.T(), wrong, held, statedLockCode, "entity")
	ts.NotContains(wrong.body, statedLockCode, "a wrong password on an unheld identity must not state a lock")
	ts.Equal(revision, runtimeRevision(ts.T(), userID), "a wrong password during the lock must not be counted")
}

func (ts *LockGranularityTestSuite) TestAnEntityLockHoldsAFederatedSignIn() {
	userID, username, _, password := ts.newUser("federated", 4)
	email := ts.federated.addIdentity(federatedSub("federated", ts.users.suffix))
	ts.federated.link(ts.T(), email, username, password)
	ts.Require().True(ts.federated.signedIn(ts.T(), email), "the linked account must sign in first")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal([]string{"entity"}, readStatus(ts.T(), userID).lockedMethods(),
		"the entity must be locked")

	ts.federated.requireRefusedWithStatedHold(ts.T(), email, flowStatedLockCode, "entity lock")
}

// The counter clears only when the sign-in completes, or a caller holding the password could guess
// the second factor forever.
func (ts *LockGranularityTestSuite) TestAFirstFactorDoesNotClearTheEntityCounter() {
	userID, username, _, password := ts.newUser("mfa_step", 5)

	failAuthentication(ts.T(), "username", username, lockThreshold-1)

	step := ts.mfa.passPassword(ts.T(), username, password)
	ts.mfa.submitCode(ts.T(), step, wrongCodeFor(ts.mfa.sentCode(ts.T())))

	ts.Equal([]string{"entity"}, readStatus(ts.T(), userID).lockedMethods(),
		"a failed second factor after a correct password must reach the threshold")
}

func (ts *LockGranularityTestSuite) TestACompletedMFASignInClearsTheEntityCounter() {
	userID, username, _, password := ts.newUser("mfa_complete", 6)

	failAuthentication(ts.T(), "username", username, lockThreshold-1)

	step := ts.mfa.passPassword(ts.T(), username, password)
	done := ts.mfa.submitCode(ts.T(), step, ts.mfa.sentCode(ts.T()))
	ts.Require().Equal("COMPLETE", done.FlowStatus, "the sign-in must complete")

	failAuthentication(ts.T(), "username", username, lockThreshold-1)
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value,
		"a completed sign-in must have cleared the entity counter")
}

// The rung is `0`, so this fails to one short, authenticates, and fails to one short again without
// locking.
func (ts *LockGranularityTestSuite) TestASuccessfulAuthenticationClearsTheEntityCounter() {
	userID, username, _, password := ts.newUser("success_clears", 3)

	failAuthentication(ts.T(), "username", username, lockThreshold-1)
	ts.Require().True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the correct secret must be accepted below the threshold")

	failAuthentication(ts.T(), "username", username, lockThreshold-1)

	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value,
		"proving control must have emptied the identity's counter")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the account must still authenticate after twice the threshold in cleared failures")
}

// A rung of `0` forms a lock the clock never lifts and that reports no expiry; unlock still releases it.
func (ts *LockGranularityTestSuite) TestARungOfZeroNeverLifts() {
	userID, username, _, password := ts.newUser("never_lifts", 2)

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the account must be locked")

	_, reported := readStatus(ts.T(), userID).expiryOf("entity")
	ts.False(reported, "a lock the clock never lifts must not report an expiry")

	// Long enough that the shortest rung any other suite in this package configures would have
	// lapsed. Nothing is run against the account in between.
	time.Sleep(time.Duration(firstRungSeconds)*time.Second + rungLapseTolerance)

	ts.Equal("LOCKED", readStatus(ts.T(), userID).Value, "waiting must not release the lock")
	ts.False(authenticate(ts.T(), "username", username, password).succeeded(),
		"the correct secret must still be refused")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)

	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"an operator unlock must be the way out of a lock with no clock behind it")
}

// newUser creates one account nobody else in this package touches, with a mobile number and the email
// a federated identity of the same name asserts.
func (ts *LockGranularityTestSuite) newUser(name string, mobileSeq int) (
	userID, username, mobile, password string) {
	ts.T().Helper()

	mobile = fmt.Sprintf("+1700%07d", mobileSeq)
	userID, username, password = ts.users.newUser(ts.T(), name, map[string]interface{}{
		"mobile_number": mobile,
		"email":         federatedEmail(federatedSub(name, ts.users.suffix)),
	})
	return userID, username, mobile, password
}
