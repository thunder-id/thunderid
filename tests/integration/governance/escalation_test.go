// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// EscalationTestSuite checks how the ladder moves over time: the last rung repeats, decay returns it
// to the bottom, a quiet window restarts the count, and failures during a live lock count for nothing.
// Every value is in seconds so the clock can be waited out, and the rungs differ by at least five seconds so
// one cannot be mistaken for another within the tolerance, the store's one-second precision included.
type EscalationTestSuite struct {
	suite.Suite
	ouID         string
	entityTypeID string
	userIDs      []string
	suffix       string
}

func TestEscalationTestSuite(t *testing.T) {
	suite.Run(t, new(EscalationTestSuite))
}

const (
	escalationThreshold    = 2
	escalationWindowSecs   = 5
	escalationDecaySeconds = 8

	// escalationLapseWait is how long past a reported expiry a case waits before relying on the lapse.
	// It stays well under escalationDecaySeconds, so waiting out a rung does not also decay the ladder.
	escalationLapseWait = 2 * time.Second

	// escalationRungTolerance absorbs the store truncating an expiry to the second and a slow read.
	escalationRungTolerance = 2500 * time.Millisecond
)

// escalationRungs is the ladder. The last rung is shorter than the one before it, so a repeat of the
// last rung is told apart from a step that ran off the end or back to the second.
var escalationRungs = []int{5, 15, 10}

var escalationTestOU = testutils.OrganizationUnit{
	Handle:      "governance_escalation_ou",
	Name:        "Governance Escalation Test OU",
	Description: "Organization unit for lock escalation acceptance tests",
}

var escalationUserType = testutils.UserType{
	Handle:      "governance_escalation_user",
	DisplayName: "governance_escalation_user",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

// escalationPolicy is a password policy with a short window, a three-rung ladder and a short decay.
func escalationPolicy() map[string]interface{} {
	return map[string]interface{}{
		"user": map[string]interface{}{
			"lockGranularity": "authentication_method",
			"default": map[string]interface{}{
				"enabled":              true,
				"threshold":            escalationThreshold,
				"failureWindowSeconds": escalationWindowSecs,
				"lockDurationsSeconds": escalationRungs,
				"lockDecaySeconds":     escalationDecaySeconds,
			},
		},
	}
}

func (ts *EscalationTestSuite) SetupSuite() {
	applyPolicy(ts.T(), escalationPolicy())

	ouID, err := testutils.CreateOrganizationUnit(escalationTestOU)
	ts.Require().NoError(err, "failed to create the test organization unit")
	ts.ouID = ouID

	escalationUserType.OUID = ts.ouID
	typeID, err := testutils.CreateUserType(escalationUserType)
	ts.Require().NoError(err, "failed to create the test user type")
	ts.entityTypeID = typeID

	ts.suffix = uniqueSuffix()
}

func (ts *EscalationTestSuite) TearDownSuite() {
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

// Lockouts with no proof of control in between serve the rungs in order, and once the
// list is exhausted the last rung repeats.
func (ts *EscalationTestSuite) TestTheLastRungRepeats() {
	userID, username := ts.newUser("repeat")

	expected := append(append([]int{}, escalationRungs...), escalationRungs[len(escalationRungs)-1])
	for i, rung := range expected {
		failAuthentication(ts.T(), "username", username, escalationThreshold)
		expiry := ts.requireRung(userID, rung, fmt.Sprintf("lockout %d must serve rung %d", i+1, rung))
		if i < len(expected)-1 {
			ts.waitOut(userID, expiry)
		}
	}
}

// Once a finite lock has ended and the decay interval has passed, the next lock serves the
// first rung again.
func (ts *EscalationTestSuite) TestDecayReturnsTheLadderToTheFirstRung() {
	userID, username := ts.newUser("decay")

	failAuthentication(ts.T(), "username", username, escalationThreshold)
	expiry := ts.requireRung(userID, escalationRungs[0], "the first lock must serve the first rung")

	time.Sleep(time.Until(expiry) + escalationDecaySeconds*time.Second + escalationLapseWait)

	failAuthentication(ts.T(), "username", username, escalationThreshold)
	ts.requireRung(userID, escalationRungs[0], "after the decay interval the ladder must start again")
}

// The failure window. A failure followed by a quiet window restarts the count, so one more failure
// does not lock; a failure inside the window still does.
func (ts *EscalationTestSuite) TestAQuietWindowRestartsTheCount() {
	userID, username := ts.newUser("window")

	failAuthentication(ts.T(), "username", username, escalationThreshold-1)
	time.Sleep(escalationWindowSecs*time.Second + escalationLapseWait)

	failAuthentication(ts.T(), "username", username, 1)
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value,
		"a failure after a quiet window must start the count again")

	failAuthentication(ts.T(), "username", username, 1)
	ts.Equal("LOCKED", readStatus(ts.T(), userID).Value,
		"a failure inside the window must still count toward the threshold")
}

// Wrong-secret failures during a live lock are not counted: they neither extend
// the lock nor advance the ladder, so the next lockout serves the second rung, not the third.
func (ts *EscalationTestSuite) TestFailuresDuringALiveLockCountForNothing() {
	userID, username := ts.newUser("live_failures")

	failAuthentication(ts.T(), "username", username, escalationThreshold)
	expiry := ts.requireRung(userID, escalationRungs[0], "the first lock must serve the first rung")

	// The store keeps whole seconds, so an extension is visible only from the next second on.
	time.Sleep(time.Second)
	failAuthentication(ts.T(), "username", username, escalationThreshold*2)
	ts.Require().True(time.Now().Before(expiry), "the failures must all fall inside the rung")

	after, reported := readStatus(ts.T(), userID).expiryOf("credential")
	ts.Require().True(reported, "the lock must still be reported")
	ts.Equal(expiry, after, "failures during a live lock must not extend it")

	ts.waitOut(userID, expiry)
	failAuthentication(ts.T(), "username", username, escalationThreshold)
	ts.requireRung(userID, escalationRungs[1],
		"failures during a live lock must not advance the ladder past the second rung")
}

// requireRung requires the user's password lock to have been formed just now with the given rung,
// and returns its expiry.
func (ts *EscalationTestSuite) requireRung(userID string, rungSeconds int, msg string) time.Time {
	ts.T().Helper()

	status := readStatus(ts.T(), userID)
	ts.Require().Equal([]string{"credential"}, status.lockedScopes(), msg+": the password must be locked")
	expiry, reported := status.expiryOf("credential")
	ts.Require().True(reported, msg+": a finite rung must report its expiry")
	ts.Require().WithinDuration(time.Now().Add(time.Duration(rungSeconds)*time.Second), expiry,
		escalationRungTolerance, msg)
	return expiry
}

// waitOut sleeps past a lock's expiry, inside the decay interval, and requires the lock to have
// lapsed.
func (ts *EscalationTestSuite) waitOut(userID string, expiry time.Time) {
	ts.T().Helper()

	time.Sleep(time.Until(expiry) + escalationLapseWait)
	ts.Require().Equal("ACTIVE", readStatus(ts.T(), userID).Value, "the rung must have lapsed")
}

// newUser creates an account no other test touches, and returns its id and username. No case here
// signs in successfully, since that would reset the ladder.
func (ts *EscalationTestSuite) newUser(name string) (userID, username string) {
	ts.T().Helper()

	username = fmt.Sprintf("escalation_%s_%s", name, ts.suffix)
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username,
		"password": "Correct-Horse-" + name + "1!",
	})
	ts.Require().NoError(err)

	userID, err = testutils.CreateUser(testutils.User{
		Type:       escalationUserType.Handle,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "failed to create the test user")
	ts.userIDs = append(ts.userIDs, userID)
	return userID, username
}
