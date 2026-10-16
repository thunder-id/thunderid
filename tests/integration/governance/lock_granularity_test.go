// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// LockGranularityTestSuite checks one counter for the whole identity, and a rung the clock
// never lifts. Both are non-default policy read at start-up, so they need a server of their own.
type LockGranularityTestSuite struct {
	suite.Suite
	ouID         string
	entityTypeID string
	userIDs      []string
	suffix       string
	federated    *federatedFixture
	mfa          *mfaFixture
}

func TestLockGranularityTestSuite(t *testing.T) {
	suite.Run(t, new(LockGranularityTestSuite))
}

var lockGranularityOU = testutils.OrganizationUnit{
	Handle:      "governance_granularity_ou",
	Name:        "Governance Lock Granularity Test OU",
	Description: "Organization unit for the entity-granularity and non-expiring-rung tests",
}

var lockGranularityUserType = testutils.UserType{
	Handle:      "governance_granularity_user",
	DisplayName: "governance_granularity_user",
	Schema: map[string]interface{}{
		"username":      map[string]interface{}{"type": "string"},
		"password":      map[string]interface{}{"type": "string", "credential": true},
		"mobile_number": map[string]interface{}{"type": "string"},
		"email":         map[string]interface{}{"type": "string"},
	},
}

// entityGranularityPatch runs one counter for the whole identity and configures its single rung as
// `0`, a lock the clock never lifts. Both share one deployment since neither affects the other.
func entityGranularityPatch() map[string]interface{} {
	return map[string]interface{}{
		"accountAccess": map[string]interface{}{
			"user": map[string]interface{}{
				"lockGranularity": "entity",
				"default": map[string]interface{}{
					"enabled":              true,
					"threshold":            lockThreshold,
					"failureWindowSeconds": failureWindowSecs,
					"lockDurationsSeconds": []int{0},
					"lockDecaySeconds":     lockDecaySeconds,
				},
			},
		},
	}
}

func (ts *LockGranularityTestSuite) SetupSuite() {
	applyPolicy(ts.T(), entityGranularityPatch()["accountAccess"])

	ouID, err := testutils.CreateOrganizationUnit(lockGranularityOU)
	ts.Require().NoError(err, "failed to create the test organization unit")
	ts.ouID = ouID

	lockGranularityUserType.OUID = ts.ouID
	typeID, err := testutils.CreateUserType(lockGranularityUserType)
	ts.Require().NoError(err, "failed to create the test user type")
	ts.entityTypeID = typeID

	ts.suffix = uniqueSuffix()
	ts.federated = newFederatedFixture(ts.T(), "granularity", ts.ouID,
		lockGranularityUserType.Handle)
	ts.mfa = newMFAFixture(ts.T(), "granularity", ts.ouID, lockGranularityUserType.Handle)
}

func (ts *LockGranularityTestSuite) TearDownSuite() {
	ts.federated.tearDown(ts.T())
	ts.mfa.tearDown(ts.T())
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

// Under entity granularity one counter holds every authentication method, and a lock earned at
// one withholds the others. The lock is reported under the reserved `entity` scope, not
// `credential`, since it refuses the identity rather than the password. A lock earned with the
// password refuses the correct one-time password and states the lock. The direct send is not
// governed, so the code does reach the held account's phone.
func (ts *LockGranularityTestSuite) TestEntityGranularityHoldsEveryAuthenticationMethod() {
	userID, username, mobile, _ := ts.newUser("entity_wide", 1)
	_, _, liveMobile, _ := ts.newUser("entity_wide_live", 7)

	failAuthentication(ts.T(), "username", username, lockThreshold)

	status := readStatus(ts.T(), userID)
	ts.Require().Equal("LOCKED", status.Value, "failing to the threshold must form a lock")
	ts.Equal([]string{"entity"}, status.lockedScopes(),
		"under entity granularity the lock must be held against the identity, not one method")

	heldSession, heldCode, sendStatus := sendDirectOTP(ts.T(), ts.mfa.server, ts.mfa.senderID, mobile)
	ts.Require().Equal(http.StatusOK, sendStatus, "the direct send must accept the request")
	ts.Require().NotEmpty(heldCode, "the direct send must deliver a code to the held account")
	liveSession, liveCode, sendStatus := sendDirectOTP(ts.T(), ts.mfa.server, ts.mfa.senderID, liveMobile)
	ts.Require().Equal(http.StatusOK, sendStatus, "the direct send must accept the request")
	ts.Require().NotEmpty(liveCode, "the control account must receive a code")

	held := verifyDirectOTP(ts.T(), heldSession, heldCode)
	wrong := verifyDirectOTP(ts.T(), liveSession, wrongCodeFor(liveCode))

	ts.False(held.succeeded(), "a password lock under entity granularity must refuse the correct code")
	requireStatedHold(ts.T(), wrong, held, statedLockCode)
}

// A federated sign-in to a linked account whose whole entity is locked is refused.
func (ts *LockGranularityTestSuite) TestAnEntityLockHoldsAFederatedSignIn() {
	userID, username, _, password := ts.newUser("federated", 4)
	email := ts.federated.addIdentity(federatedSub("federated", ts.suffix))
	ts.federated.link(ts.T(), email, username, password)
	ts.Require().True(ts.federated.signedIn(ts.T(), email), "the linked account must sign in first")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal([]string{"entity"}, readStatus(ts.T(), userID).lockedScopes(),
		"the entity must be locked")

	ts.federated.requireRefusedWithStatedHold(ts.T(), email, flowStatedLockCode, "entity lock")
}

// A correct first factor does not clear the entity counter: it clears only when the sign-in
// completes. Otherwise a caller holding the password could guess the second factor forever.
func (ts *LockGranularityTestSuite) TestAFirstFactorDoesNotClearTheEntityCounter() {
	userID, username, _, password := ts.newUser("mfa_step", 5)

	failAuthentication(ts.T(), "username", username, lockThreshold-1)

	step := ts.mfa.passPassword(ts.T(), username, password)
	ts.mfa.sentCode(ts.T())
	ts.mfa.submitCode(ts.T(), step, "000000")

	ts.Equal([]string{"entity"}, readStatus(ts.T(), userID).lockedScopes(),
		"a failed second factor after a correct password must reach the threshold")
}

// Through a flow, a completed password and one-time-password sign-in clears the entity counter.
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

// Under entity granularity a successful authentication clears the one counter. The rung is `0`,
// so this fails to one short, authenticates, and fails to one short again without locking.
func (ts *LockGranularityTestSuite) TestASuccessfulAuthenticationClearsTheEntityCounter() {
	userID, username, password := ts.newUserWithPassword("success_clears", 3)

	failAuthentication(ts.T(), "username", username, lockThreshold-1)
	ts.Require().True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the correct secret must be accepted below the threshold")

	failAuthentication(ts.T(), "username", username, lockThreshold-1)

	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value,
		"proving control must have emptied the identity's counter")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the account must still authenticate after twice the threshold in cleared failures")
}

// A rung configured as `0` produces a lock time never lifts, and the read reports no expiry.
// Unlock is re-checked, since it is the only way out under this policy.
func (ts *LockGranularityTestSuite) TestARungOfZeroNeverLifts() {
	userID, username, password := ts.newUserWithPassword("never_lifts", 2)

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

// newUser creates one account nobody else in this package touches.
func (ts *LockGranularityTestSuite) newUser(name string, mobileSeq int) (
	userID, username, mobile, password string) {
	ts.T().Helper()

	username = fmt.Sprintf("granularity_%s_%s", name, ts.suffix)
	password = "Correct-Horse-" + name + "1!"
	mobile = fmt.Sprintf("+1700%07d", mobileSeq)
	attributes, err := json.Marshal(map[string]interface{}{
		"username":      username,
		"password":      password,
		"mobile_number": mobile,
		"email":         federatedEmail(federatedSub(name, ts.suffix)),
	})
	ts.Require().NoError(err)

	userID, err = testutils.CreateUser(testutils.User{
		Type:       lockGranularityUserType.Handle,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "failed to create the test user")
	ts.userIDs = append(ts.userIDs, userID)
	return userID, username, mobile, password
}

// newUserWithPassword is newUser for the cases that do not need the mobile number.
func (ts *LockGranularityTestSuite) newUserWithPassword(name string, mobileSeq int) (
	userID, username, password string) {
	ts.T().Helper()

	userID, username, _, password = ts.newUser(name, mobileSeq)
	return userID, username, password
}
