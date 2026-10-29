// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"testing"

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
		"lockGranularity": "account",
		"default":         nonExpiringRung(),
	}}))

	ts.users = newUserFixture(ts.T(), "granularity", testutils.UserType{Schema: credentialsSchema(
		map[string]interface{}{"mobile_number": stringAttribute, "email": stringAttribute})})
	ts.federated = newFederatedFixture(ts.T(), "granularity", ts.users.ouID, ts.users.userType)
	ts.mfa = newMFAFixture(ts.T(), "granularity", ts.users.ouID, ts.users.userType)
}

// The lock is reported under the reserved `account` method, since it refuses the identity rather than
// the password. The direct send is not governed, so the held account still receives a code to present.
func (ts *LockGranularityTestSuite) TestAccountGranularityHoldsEveryAuthenticationMethod() {
	userID, username, mobile, _ := ts.newUser("account_wide")
	_, _, liveMobile, _ := ts.newUser("account_wide_live")

	failAuthentication(ts.T(), "username", username, lockThreshold)

	status := readStatus(ts.T(), userID)
	ts.Require().Equal("LOCKED", status.Value, "failing to the threshold must form a lock")
	ts.Equal([]string{"account"}, status.lockedMethods(),
		"under account granularity the lock must be held against the identity, not one method")

	heldSession, heldCode := sendDirectOTP(ts.T(), ts.mfa.server, ts.mfa.senderID, mobile)
	liveSession, liveCode := sendDirectOTP(ts.T(), ts.mfa.server, ts.mfa.senderID, liveMobile)

	held := verifyDirectOTP(ts.T(), heldSession, heldCode)
	wrong := verifyDirectOTP(ts.T(), liveSession, wrongCodeFor(liveCode))

	requireStatedHold(ts.T(), wrong, held, statedLockCode, "account")
}

func (ts *LockGranularityTestSuite) TestAWrongPasswordOnAnAccountLockStatesTheLockWithoutCounting() {
	userID, username, _, _ := ts.newUser("account_wrong")
	_, liveName, _, _ := ts.newUser("account_wrong_live")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal([]string{"account"}, readStatus(ts.T(), userID).lockedMethods(),
		"the account must be locked")
	revision := runtimeRevision(ts.T(), userID)

	wrong := authenticate(ts.T(), "username", liveName, "Not-The-Secret-8!")
	held := authenticate(ts.T(), "username", username, "Not-The-Secret-8!")

	requireStatedHold(ts.T(), wrong, held, statedLockCode, "account")
	ts.NotContains(wrong.body, statedLockCode, "a wrong password on an unheld identity must not state a lock")
	ts.Equal(revision, runtimeRevision(ts.T(), userID), "a wrong password during the lock must not be counted")
}

func (ts *LockGranularityTestSuite) TestAnAccountLockHoldsAFederatedSignIn() {
	userID, username, _, password := ts.newUser("federated")
	email := ts.federated.addIdentity(federatedSub("federated", ts.users.suffix))
	ts.federated.link(ts.T(), email, username, password)
	ts.Require().True(ts.federated.signedIn(ts.T(), email), "the linked account must sign in first")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal([]string{"account"}, readStatus(ts.T(), userID).lockedMethods(),
		"the account must be locked")

	ts.federated.requireRefusedWithStatedHold(ts.T(), email, flowStatedLockCode, "account lock")
}

// The counter clears only when the sign-in completes, or a caller holding the password could guess
// the second factor forever.
func (ts *LockGranularityTestSuite) TestAFirstFactorDoesNotClearTheAccountCounter() {
	userID, username, _, password := ts.newUser("mfa_step")

	failAuthentication(ts.T(), "username", username, lockThreshold-1)

	step := ts.mfa.passPassword(ts.T(), username, password)
	ts.mfa.submitCode(ts.T(), step, wrongCodeFor(ts.mfa.sentCode(ts.T())))

	ts.Equal([]string{"account"}, readStatus(ts.T(), userID).lockedMethods(),
		"a failed second factor after a correct password must reach the threshold")
}

func (ts *LockGranularityTestSuite) TestACompletedMFASignInClearsTheAccountCounter() {
	userID, username, _, password := ts.newUser("mfa_complete")

	failAuthentication(ts.T(), "username", username, lockThreshold-1)

	step := ts.mfa.passPassword(ts.T(), username, password)
	done := ts.mfa.submitCode(ts.T(), step, ts.mfa.sentCode(ts.T()))
	ts.Require().Equal("COMPLETE", done.FlowStatus, "the sign-in must complete")

	failAuthentication(ts.T(), "username", username, lockThreshold-1)
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value,
		"a completed sign-in must have cleared the account counter")
}

// The rung is `0`, so this fails to one short, authenticates, and fails to one short again without
// locking.
func (ts *LockGranularityTestSuite) TestASuccessfulAuthenticationClearsTheAccountCounter() {
	userID, username, _, password := ts.newUser("success_clears")

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
	userID, username, _, password := ts.newUser("never_lifts")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the account must be locked")

	_, reported := readStatus(ts.T(), userID).expiryOf("account")
	ts.False(reported, "a lock the clock never lifts must not report an expiry")

	// A lock with a duration reports its expiry, so the missing expiry already rules out a rung; this
	// wait shows the lock holds on. Only status reads, which write nothing, run against the account.
	ts.Never(func() bool {
		status, err := testutils.GetAccessStatus(userCollection, userID)
		return err != nil || status.Value != "LOCKED"
	}, rungLapseTolerance, statusPollInterval,
		"waiting must not release the lock")
	requireRefusedAsHeld(ts.T(), authenticate(ts.T(), "username", username, password), statedLockCode,
		"the correct secret must still be refused")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)

	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"an operator unlock must be the way out of a lock with no clock behind it")
}

// newUser creates one account nobody else in this package touches, with a mobile number and the email
// a federated identity of the same name asserts.
func (ts *LockGranularityTestSuite) newUser(name string) (userID, username, mobile, password string) {
	ts.T().Helper()

	mobile = newMobile()
	userID, username, password = ts.users.newUser(ts.T(), name, map[string]interface{}{
		"mobile_number": mobile,
		"email":         federatedEmail(federatedSub(name, ts.users.suffix)),
	})
	return userID, username, mobile, password
}
