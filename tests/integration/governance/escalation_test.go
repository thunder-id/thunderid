// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	escalationThreshold    = 2
	escalationWindowSecs   = 5
	escalationDecaySeconds = 8

	// escalationLapseWait is how long past a reported expiry a lock may take to read as lapsed, and the
	// margin the decay and quiet-window sleeps add. It stays well under escalationDecaySeconds, so waiting
	// out a rung does not also decay the ladder.
	escalationLapseWait = 2 * time.Second

	// escalationCeiling is the consecutive-failure ceiling the ceiling cases set: two lockouts' worth.
	escalationCeiling = 2 * escalationThreshold
)

// escalationRungs is the ladder. The last rung is shorter than the one before it, so a repeat of the
// last rung is told apart from a step that ran off the end or back to the second.
var escalationRungs = []int{2, 6, 4}

// liveLockRungs is the ladder of the case that presents secrets during a lock: the first rung leaves
// that case seconds of slack, and the rest are told apart as escalationRungs are.
var liveLockRungs = []int{8, 6, 4}

// EscalationTestSuite checks how the ladder moves over time: the last rung repeats, decay returns it
// to the bottom, a quiet window restarts the count, and failures during a live lock count for nothing.
// Every value is in seconds so the clock can be waited out. The rung a lock serves is read from the
// lock itself, so no assertion depends on how long the test took to read it.
type EscalationTestSuite struct {
	suite.Suite
	users *userFixture
}

func TestEscalationTestSuite(t *testing.T) {
	suite.Run(t, new(EscalationTestSuite))
}

func (ts *EscalationTestSuite) SetupSuite() {
	applyPolicy(ts.T(), escalationPolicy())
	ts.users = newUserFixture(ts.T(), "escalation", testutils.UserType{Schema: credentialsSchema(nil)})
}

func (ts *EscalationTestSuite) TestTheLastRungRepeats() {
	userID, username, _ := ts.users.newUser(ts.T(), "repeat", nil)

	expected := append(append([]int{}, escalationRungs...), escalationRungs[len(escalationRungs)-1])
	for i, rung := range expected {
		failAuthentication(ts.T(), "username", username, escalationThreshold)
		expiry := ts.requireRung(userID, rung, fmt.Sprintf("lockout %d must serve rung %d", i+1, rung))
		if i < len(expected)-1 {
			awaitLapse(ts.T(), userID, expiry, escalationLapseWait)
		}
	}
}

// Decay shows in no read, so this wait is fixed rather than polled.
func (ts *EscalationTestSuite) TestDecayReturnsTheLadderToTheFirstRung() {
	userID, username, _ := ts.users.newUser(ts.T(), "decay", nil)

	failAuthentication(ts.T(), "username", username, escalationThreshold)
	expiry := ts.requireRung(userID, escalationRungs[0], "the first lock must serve the first rung")

	time.Sleep(time.Until(expiry) + escalationDecaySeconds*time.Second + escalationLapseWait)

	failAuthentication(ts.T(), "username", username, escalationThreshold)
	ts.requireRung(userID, escalationRungs[0], "after the decay interval the ladder must start again")
}

// A quiet window shows in no read either, so this wait is fixed too.
func (ts *EscalationTestSuite) TestAQuietWindowRestartsTheCount() {
	userID, username, _ := ts.users.newUser(ts.T(), "window", nil)

	failAuthentication(ts.T(), "username", username, escalationThreshold-1)
	time.Sleep(escalationWindowSecs*time.Second + escalationLapseWait)

	failAuthentication(ts.T(), "username", username, 1)
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value,
		"a failure after a quiet window must start the count again")

	failAuthentication(ts.T(), "username", username, 1)
	ts.Equal("LOCKED", readStatus(ts.T(), userID).Value,
		"a failure inside the window must still count toward the threshold")
}

// Uncounted failures write nothing, so the lock is not extended, and the next lockout serves the
// second rung, not the third.
func (ts *EscalationTestSuite) TestFailuresDuringALiveLockCountForNothing() {
	applyPolicy(ts.T(), mergeJSONObjects(escalationPolicy(),
		userDefaults(map[string]interface{}{"lockDurationsSeconds": liveLockRungs})))
	userID, username, _ := ts.users.newUser(ts.T(), "live_failures", nil)

	failAuthentication(ts.T(), "username", username, escalationThreshold)
	expiry := ts.requireRung(userID, liveLockRungs[0], "the first lock must serve the first rung")
	revision := runtimeRevision(ts.T(), userID)

	failHeldAuthentication(ts.T(), "username", username, escalationThreshold*2, statedLockCode)
	ts.Equal(revision, runtimeRevision(ts.T(), userID), "failures during a live lock must write nothing")

	awaitLapse(ts.T(), userID, expiry, escalationLapseWait)
	failAuthentication(ts.T(), "username", username, escalationThreshold)
	ts.requireRung(userID, liveLockRungs[1],
		"failures during a live lock must not advance the ladder past the second rung")
}

// Failures with no success in between reach the ceiling across a lapsed lock.
func (ts *EscalationTestSuite) TestTheConsecutiveCeilingHoldsUntilAnUnlock() {
	applyPolicy(ts.T(), escalationCeilingPolicy())
	userID, username, password := ts.users.newUser(ts.T(), "ceiling", nil)
	_, liveName, _ := ts.users.newUser(ts.T(), "ceiling_live", nil)

	failAuthentication(ts.T(), "username", username, escalationThreshold)
	awaitLapse(ts.T(), userID,
		ts.requireRung(userID, escalationRungs[0], "the first lock must serve the first rung"), escalationLapseWait)

	failAuthentication(ts.T(), "username", username, escalationCeiling-escalationThreshold)
	status := readStatus(ts.T(), userID)
	ts.Require().Equal([]string{"credential"}, status.lockedMethods(), "the ceiling must lock the password")
	_, reported := status.expiryOf("credential")
	ts.False(reported, "a lock at the ceiling must not lapse, so it reports no expiry")

	wrong := authenticate(ts.T(), "username", liveName, "wrong-password")
	held := authenticate(ts.T(), "username", username, password)
	requireStatedHold(ts.T(), wrong, held, statedLockCode, "credential")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"an unlock must release the ceiling's lock")
}

func (ts *EscalationTestSuite) TestASuccessResetsTheConsecutiveCount() {
	applyPolicy(ts.T(), escalationCeilingPolicy())
	userID, username, password := ts.users.newUser(ts.T(), "ceiling_reset", nil)

	failAuthentication(ts.T(), "username", username, escalationThreshold)
	awaitLapse(ts.T(), userID,
		ts.requireRung(userID, escalationRungs[0], "the first lock must serve the first rung"), escalationLapseWait)
	ts.Require().True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the lapsed lock must admit the correct password")

	failAuthentication(ts.T(), "username", username, escalationCeiling-escalationThreshold)
	ts.requireRung(userID, escalationRungs[0],
		"after a success the ceiling must not be reached, and the ladder starts again")
}

// requireRung requires the user's password lock to serve the given rung, read as the lock's unlock
// time less the failure that formed it, and returns its stored expiry. Both come from the runtime row,
// so a short rung cannot lapse before they are read.
func (ts *EscalationTestSuite) requireRung(userID string, rungSeconds int, msg string) time.Time {
	ts.T().Helper()

	ts.Require().Equal(time.Duration(rungSeconds)*time.Second, servedRung(ts.T(), userID, "credential"), msg)
	return storedExpiry(ts.T(), userID, "credential")
}

// escalationPolicy is a password policy with a short window, a three-rung ladder and a short decay.
func escalationPolicy() map[string]interface{} {
	return lockoutPolicyWith(userDefaults(map[string]interface{}{
		"threshold":            escalationThreshold,
		"failureWindowSeconds": escalationWindowSecs,
		"lockDurationsSeconds": escalationRungs,
		"lockDecaySeconds":     escalationDecaySeconds,
	}))
}

// escalationCeilingPolicy is escalationPolicy with a consecutive-failure ceiling.
func escalationCeilingPolicy() map[string]interface{} {
	return mergeJSONObjects(escalationPolicy(),
		userDefaults(map[string]interface{}{"maxConsecutiveFailures": escalationCeiling}))
}
