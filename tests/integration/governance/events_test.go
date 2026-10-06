// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Event types and data keys this file asserts. They mirror the server's event package, which a
// separate module cannot import; the names are the contract an event consumer reads.
const (
	eventIdentitySuspended          = "IDENTITY_SUSPENDED"
	eventIdentityUnsuspended        = "IDENTITY_UNSUSPENDED"
	eventIdentityUnlocked           = "IDENTITY_UNLOCKED"
	eventIdentityUnsuspensionFailed = "IDENTITY_UNSUSPENSION_FAILED"
	eventIdentitySuspensionFailed   = "IDENTITY_SUSPENSION_FAILED"
	eventIdentitySelfUnlocked       = "IDENTITY_SELF_UNLOCKED"
	eventIdentityLocked             = "IDENTITY_LOCKED"
	eventServerConfigUpdated        = "SERVER_CONFIG_UPDATED"
	eventServerConfigUpdateFailed   = "SERVER_CONFIG_UPDATE_FAILED"

	eventKeySubject       = "sub"
	eventKeySubjectType   = "sub_type"
	eventKeyActorSub      = "act_sub"
	eventKeyNoteRecorded  = "operator_note_recorded"
	eventKeyError         = "error"
	eventKeyConfigName    = "config_name"
	eventKeyLockMethod    = "lock_method"
	eventKeyLockCount     = "lock_count"
	eventKeyLockPermanent = "lock_permanent"
	eventKeyUnlockAt      = "unlock_at"
)

// GovernanceEventsTestSuite checks what the lifecycle operations and an accountAccess write publish,
// read back from the observability file sink: the operator and the target, the outcome and its
// failure class, and never the operator note or the written value.
type GovernanceEventsTestSuite struct {
	suite.Suite
	fixture  *heldRecoveryFixture
	reader   *testutils.EventLogReader
	adminSub string
}

func TestGovernanceEventsTestSuite(t *testing.T) {
	suite.Run(t, new(GovernanceEventsTestSuite))
}

func (ts *GovernanceEventsTestSuite) SetupSuite() {
	t := ts.T()
	logPath := testutils.EventLogPath("governance-events.log")
	t.Cleanup(func() { restoreShippedPolicy(t) })
	require.NoError(t, testutils.PatchDeploymentConfig(testutils.ObservabilityFileSinkPatch(logPath, true)),
		"failed to enable the observability file sink")
	require.NoError(t, testutils.RestartServer(), "failed to restart with observability enabled")
	require.NoError(t, testutils.ObtainAdminAccessToken(), "failed to re-obtain the admin token")

	token, err := testutils.GetAccessToken()
	require.NoError(t, err)
	claims, err := testutils.DecodeJWT(token)
	require.NoError(t, err, "the admin token must be a JWT")
	require.NotEmpty(t, claims.Sub, "the admin token must name its subject")
	ts.adminSub = claims.Sub

	ts.reader = testutils.NewEventLogReader(logPath)
	ts.fixture = newHeldRecoveryFixture(t, "events")
}

// The operator note is published only as whether one was given.
func (ts *GovernanceEventsTestSuite) TestLifecycleOperationsArePublishedWithTheOperatorAndTarget() {
	userID, _, _ := ts.fixture.newUser(ts.T(), "lifecycle", nil)
	note := "events-note-" + ts.fixture.suffix
	ts.Require().NoError(ts.reader.Reset())

	runAdministrationFlowWithInputs(ts.T(), suspendFlowHandle,
		map[string]string{subjectInput: userID, operatorNoteInput: note})
	suspended := ts.awaitEvent(eventIdentitySuspended, userID)
	ts.Equal(ts.adminSub, suspended.Str(eventKeyActorSub), "the operator must be act_sub")
	ts.Equal("user", suspended.Str(eventKeySubjectType), "the target's category must be published")
	ts.Equal("success", suspended.Status)
	ts.Equal(true, suspended.Data[eventKeyNoteRecorded], "whether a note was given is published")

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	unsuspended := ts.awaitEvent(eventIdentityUnsuspended, userID)
	ts.Equal(ts.adminSub, unsuspended.Str(eventKeyActorSub), "the operator must be act_sub")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	unlocked := ts.awaitEvent(eventIdentityUnlocked, userID)
	ts.Equal(ts.adminSub, unlocked.Str(eventKeyActorSub), "the operator must be act_sub")

	// The unlock is a later request than the suspend and unsuspend, so every event they published has
	// been read by now.
	events, err := ts.reader.Read()
	ts.Require().NoError(err)
	for _, evt := range events {
		ts.NotContains(evt.Raw, note, "the operator note must never be published")
	}
}

func (ts *GovernanceEventsTestSuite) TestARefusedOperationIsPublishedWithItsFailureClass() {
	userID, _, _ := ts.fixture.newUser(ts.T(), "refused", nil)
	ts.Require().NoError(ts.reader.Reset())

	answer := attemptAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	ts.Require().Equal(governanceRefusal, answer, "unsuspend of an active account must be refused")

	failed := ts.awaitEvent(eventIdentityUnsuspensionFailed, userID)
	ts.Equal("failure", failed.Status)
	ts.Equal("not_applicable", failed.Str(eventKeyError), "the failure class must be published")
	ts.Equal(ts.adminSub, failed.Str(eventKeyActorSub), "a refusal must name the operator too")
}

func (ts *GovernanceEventsTestSuite) TestASuspendRefusedAtItsPreCheckIsPublishedAndRevokesNothing() {
	ts.Require().NoError(ts.reader.Reset())

	answer := attemptAdministrationFlow(ts.T(), suspendFlowHandle, ts.adminSub)
	ts.Require().Equal(governanceRefusal, answer, "an operator suspending themselves must be refused")

	failed := ts.awaitEvent(eventIdentitySuspensionFailed, ts.adminSub)
	ts.Equal("failure", failed.Status)
	ts.Equal("not_authorized", failed.Str(eventKeyError), "the failure class must be published")
	ts.Equal(ts.adminSub, failed.Str(eventKeyActorSub), "a refusal must name the operator too")

	// A later suspension's revocation, once the REST API enforces it, shows the deny list was refreshed
	// past anything the refused attempt could have written.
	witnessID, witnessName, witnessPassword := ts.fixture.newUser(ts.T(), "precheck_witness", nil)
	witness, err := testutils.GetHTTPClientForUser(witnessName, witnessPassword)
	ts.Require().NoError(err, "the witness must sign in")
	witnessStatus := func() int {
		status, _ := getWith(ts.T(), witness, "/users/me")
		return status
	}
	ts.Require().Equal(http.StatusOK, witnessStatus(), "the witness's token must work before its suspension")
	runAdministrationFlow(ts.T(), suspendFlowHandle, witnessID)
	ts.Require().Eventually(func() bool { return witnessStatus() == http.StatusUnauthorized },
		denyListRefreshWait, 250*time.Millisecond, "the witness's suspension must reach the REST API")

	status, body := getWith(ts.T(), testutils.GetHTTPClient(), "/users/"+ts.adminSub)
	ts.Equal(http.StatusOK, status, "a refused pre-check must revoke none of the operator's tokens: %s", body)
	ts.Equal("ACTIVE", readStatus(ts.T(), ts.adminSub).Value, "a refused suspend must hold nothing")
}

func (ts *GovernanceEventsTestSuite) TestARecoveryReleaseIsPublishedAsSelfService() {
	userID, username, _ := ts.fixture.newUser(ts.T(), "self_unlock", nil)
	failAuthentication(ts.T(), "username", username, shippedUserLockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the account must be locked first")
	ts.Require().NoError(ts.reader.Reset())

	ts.fixture.recover(ts.T(), username, "Recovered-Events-1!")
	released := ts.awaitEvent(eventIdentitySelfUnlocked, userID)
	ts.Equal("success", released.Status)
	ts.Empty(released.Str(eventKeyActorSub), "a self-service release has no operator")
}

// Wrong passwords during the lock and the refused correct one form nothing, so nothing more is
// published for it before the operator unlock that follows them.
func (ts *GovernanceEventsTestSuite) TestALockIsPublishedOnceWithoutAnOperator() {
	userID, username, password := ts.fixture.newUser(ts.T(), "locked", nil)
	ts.Require().NoError(ts.reader.Reset())

	failAuthentication(ts.T(), "username", username, shippedUserLockThreshold)
	locked := ts.awaitEvent(eventIdentityLocked, userID)
	ts.Equal("success", locked.Status)
	ts.Equal("user", locked.Str(eventKeySubjectType), "the target's category must be published")
	ts.Equal("credential", locked.Str(eventKeyLockMethod), "the locked method must be published")
	ts.Equal(float64(1), locked.Data[eventKeyLockCount], "the first lock must be published as the first rung")
	ts.Equal(false, locked.Data[eventKeyLockPermanent], "the shipped first rung expires")
	ts.False(locked.Has(eventKeyActorSub), "a lock failed sign-ins formed has no operator to publish")

	expiry, reported := readStatus(ts.T(), userID).expiryOf("credential")
	ts.Require().True(reported, "the lock must report its expiry")
	unlockAt, err := time.Parse(time.RFC3339, locked.Str(eventKeyUnlockAt))
	ts.Require().NoError(err, "unlock_at must be an RFC 3339 instant, got %q", locked.Str(eventKeyUnlockAt))
	ts.True(expiry.Equal(unlockAt), "unlock_at %s must be the lock's expiry %s", unlockAt, expiry)

	failHeldAuthentication(ts.T(), "username", username, shippedUserLockThreshold, statedLockCode)
	ts.Require().False(authenticate(ts.T(), "username", username, password).succeeded(),
		"the correct password must be refused while the lock is live")
	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	unlocked := ts.awaitEvent(eventIdentityUnlocked, userID)
	ts.Equal(ts.adminSub, unlocked.Str(eventKeyActorSub), "an operator unlock must name the operator")
	ts.Equal("user", unlocked.Str(eventKeySubjectType), "the target's category must be published")
	ts.Equal(1, ts.countEvents(eventIdentityLocked, userID), "one lock must be published once")
}

func (ts *GovernanceEventsTestSuite) TestASectionWriteIsPublishedWithoutItsValue() {
	previous, err := testutils.GetWritableServerConfig(accountAccessSection)
	ts.Require().NoError(err)
	defer func() {
		restore := []byte(previous)
		if len(bytes.TrimSpace(restore)) == 0 || string(bytes.TrimSpace(restore)) == "null" {
			restore = []byte(`{}`)
		}
		ts.NoError(testutils.PutWritableServerConfig(accountAccessSection, restore),
			"failed to restore the accountAccess section")
	}()
	ts.Require().NoError(ts.reader.Reset())

	const marker = 7364519
	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection,
		[]byte(fmt.Sprintf(`{"user":{"lastLoginResolutionSeconds":%d}}`, marker))))
	updated := ts.awaitEvent(eventServerConfigUpdated, "")
	ts.Equal(accountAccessSection, updated.Str(eventKeyConfigName), "the section must be published")
	ts.Equal(ts.adminSub, updated.Str(eventKeyActorSub), "the operator must be published")
	ts.NotContains(updated.Raw, fmt.Sprint(marker), "the written value must never be published")

	status, body, err := testutils.PutWritableServerConfigWithStatus(accountAccessSection,
		[]byte(`{"user":{"default":{"lockThreshold":2}}}`))
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, status, "a misspelled key must be refused: %s", body)
	failed := ts.awaitEvent(eventServerConfigUpdateFailed, "")
	ts.Equal(accountAccessSection, failed.Str(eventKeyConfigName))
	ts.Equal("SCF-1003", failed.Str(eventKeyError), "a refused write must publish its error code")
}

// awaitEvent waits for an event of the type, about the subject when one is given.
func (ts *GovernanceEventsTestSuite) awaitEvent(eventType, subject string) testutils.ObservabilityEvent {
	ts.T().Helper()
	evt, err := ts.reader.Await(func(evt testutils.ObservabilityEvent) bool {
		return evt.Type == eventType && (subject == "" || evt.Str(eventKeySubject) == subject)
	})
	ts.Require().NoError(err, "no %s for %q was published", eventType, subject)
	return evt
}

// countEvents counts the events of the type about the subject read since the last reset. It does not
// wait, so the caller first awaits an event published after the ones it counts.
func (ts *GovernanceEventsTestSuite) countEvents(eventType, subject string) int {
	ts.T().Helper()
	events, err := ts.reader.Read()
	ts.Require().NoError(err)
	matched := 0
	for _, evt := range events {
		if evt.Type == eventType && evt.Str(eventKeySubject) == subject {
			matched++
		}
	}
	return matched
}
