// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
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
	eventKeyAppliedBy     = "applied_by"
	eventKeyError         = "error"
	eventKeyConfigName    = "config_name"
	eventKeyLockMethod    = "lock_method"
	eventKeyLockCount     = "lock_count"
	eventKeyLockPermanent = "lock_permanent"
	eventKeyUnlockAt      = "unlock_at"
)

// operatorNoteCap is the longest operator note a suspension takes, in characters.
const operatorNoteCap = 100

// GovernanceEventsTestSuite checks what the lifecycle operations and an accountAccess write publish,
// read back from the observability file sink: the operator and the target, the outcome and its
// failure class, and never the operator note or the written value.
type GovernanceEventsTestSuite struct {
	suite.Suite
	fixture  *heldRecoveryFixture
	reader   *testutils.EventLogReader
	audit    *testutils.AuditLogReader
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
	ts.audit = testutils.NewAuditLogReader()
	ts.fixture = newHeldRecoveryFixture(t, "events")
}

// The operator note is published only as whether one was given.
func (ts *GovernanceEventsTestSuite) TestLifecycleOperationsArePublishedWithTheOperatorAndTarget() {
	userID, _, _ := ts.fixture.newUser(ts.T(), "lifecycle", nil)
	note := "events-note-" + ts.fixture.suffix
	ts.resetReaders()

	runAdministrationFlowAs(ts.T(), adminToken(ts.T()), suspendFlowHandle,
		map[string]string{subjectInput: userID, operatorNoteInput: note})
	suspended := ts.awaitEvent(eventIdentitySuspended, userID)
	ts.Equal(ts.adminSub, suspended.Str(eventKeyActorSub), "the operator must be act_sub")
	ts.Equal("user", suspended.Str(eventKeySubjectType), "the target's category must be published")
	ts.Equal("success", suspended.Status)
	ts.Equal(true, suspended.Data[eventKeyNoteRecorded], "whether a note was given is published")
	ts.Equal("organization", suspended.Str(eventKeyAppliedBy), "an operator's suspension is the organization's")

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	unsuspended := ts.awaitEvent(eventIdentityUnsuspended, userID)
	ts.Equal(ts.adminSub, unsuspended.Str(eventKeyActorSub), "the operator must be act_sub")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	unlocked := ts.awaitEvent(eventIdentityUnlocked, userID)
	ts.Equal(ts.adminSub, unlocked.Str(eventKeyActorSub), "the operator must be act_sub")

	user := testutils.AuditRef{Type: auditTargetUser, ID: userID}
	wantDetails := map[string]map[string]string{
		auditUserSuspend:   {"applied_by": "organization"},
		auditUserUnsuspend: {"applied_by": "organization"},
		auditUserUnlock:    {"self_service": "false"},
	}
	for _, name := range []string{auditUserSuspend, auditUserUnsuspend, auditUserUnlock} {
		ts.requireOperatorLine(name, user, ts.adminSub, wantDetails[name])
	}

	// The unlock is a later request than the suspend and unsuspend, so every event they published has
	// been read by now.
	events, err := ts.reader.Read()
	ts.Require().NoError(err)
	for _, evt := range events {
		ts.NotContains(evt.Raw, note, "the operator note must never be published")
	}
	lines, err := ts.audit.Read()
	ts.Require().NoError(err)
	for _, line := range lines {
		ts.NotContains(line.Raw, note, "the operator note must never be written to the audit log")
	}
	ts.Equal(1, ts.countLines(auditUserSuspend, userID), "one suspend must write one line")
}

func (ts *GovernanceEventsTestSuite) TestARefusedOperationIsPublishedWithItsFailureClass() {
	userID, _, _ := ts.fixture.newUser(ts.T(), "refused", nil)
	ts.resetReaders()

	answer := attemptAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	ts.Require().Equal(governanceRefusal, answer, "unsuspend of an active account must be refused")

	failed := ts.awaitEvent(eventIdentityUnsuspensionFailed, userID)
	ts.Equal("failure", failed.Status)
	ts.Equal("not_applicable", failed.Str(eventKeyError), "the failure class must be published")
	ts.Equal(ts.adminSub, failed.Str(eventKeyActorSub), "a refusal must name the operator too")

	line := ts.awaitLine(auditUserUnsuspend, userID)
	ts.Equal(testutils.AuditRef{Type: auditTargetUser, ID: userID}, line.Target)
	ts.Equal(testutils.AuditRef{}, line.Subject, "a failed attempt names no subject")
	ts.Equal(map[string]string{"result": auditResultFailed, "reason": "igs_1002"}, line.Outcome)
}

// The operator is a throwaway root, so a regressed pre-check suspends it rather than the
// administrator every other suite runs as.
func (ts *GovernanceEventsTestSuite) TestASuspendRefusedAtItsPreCheckIsPublishedAndRevokesNothing() {
	rootID, _, _, rootToken := newRootOperator(ts.T(), ts.fixture.userFixture)
	ts.resetReaders()

	answer := attemptAdministrationFlowAs(ts.T(), rootToken, suspendFlowHandle,
		map[string]string{subjectInput: rootID})
	ts.Require().Equal(governanceRefusal, answer, "an operator suspending themselves must be refused")

	failed := ts.awaitEvent(eventIdentitySuspensionFailed, rootID)
	ts.Equal("failure", failed.Status)
	ts.Equal("not_authorized", failed.Str(eventKeyError), "the failure class must be published")
	ts.Equal(rootID, failed.Str(eventKeyActorSub), "a refusal must name the operator too")

	denied := ts.awaitLine(auditUserSuspend, rootID)
	root := testutils.AuditRef{Type: auditTargetUser, ID: rootID}
	ts.Equal(root, denied.Actor)
	ts.Equal(root, denied.Subject, "a denied attempt names the identity it was refused on")
	ts.Equal(map[string]string{"result": auditResultDenied, "reason": "forbidden"}, denied.Outcome)

	// A later suspension's revocation, once the REST API enforces it, shows the deny list was refreshed
	// past anything the refused attempt could have written.
	witnessID, witnessName, witnessPassword := ts.fixture.newUser(ts.T(), "precheck_witness", nil)
	witnessStatus := ts.holderStatus(witnessName, witnessPassword)
	runAdministrationFlow(ts.T(), suspendFlowHandle, witnessID)
	ts.Require().Eventually(func() bool { return witnessStatus() == http.StatusUnauthorized },
		denyListRefreshWait, 250*time.Millisecond, "the witness's suspension must reach the REST API")

	status, body := getWith(ts.T(), testutils.GetHTTPClientWithToken(rootToken), "/users/"+rootID)
	ts.Equal(http.StatusOK, status, "a refused pre-check must revoke none of the operator's tokens: %s", body)
	ts.Equal("ACTIVE", readStatus(ts.T(), rootID).Value, "a refused suspend must hold nothing")
}

// A suspend or unsuspend naming no one asks for a subject and publishes nothing. One naming an identity
// that does not exist is refused, published as not_found, and writes nothing: no revocation and no hold.
func (ts *GovernanceEventsTestSuite) TestAnOperationWithoutAKnownSubjectIsRefusedAndWritesNothing() {
	unknownID := fmt.Sprintf("00000000-0000-4000-8000-%012d", time.Now().UnixNano()%1_000_000_000_000)
	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	ts.resetReaders()

	for _, handle := range []string{suspendFlowHandle, unsuspendFlowHandle} {
		status, raw := executeAdministrationFlowAs(ts.T(), token, handle, map[string]string{})
		step := flowStepOf(ts.T(), status, raw)
		ts.Equal("INCOMPLETE", step.FlowStatus, "%s without a subject must ask for one: %s", handle, raw)
		ts.True(common.HasInput(step.Data.Inputs, subjectInput), "%s must ask for the subject: %s", handle, raw)
	}

	ts.Equal(governanceRefusal, attemptAdministrationFlow(ts.T(), suspendFlowHandle, unknownID),
		"a suspend of an unknown identity must be refused")
	ts.Equal("not_found", ts.awaitEvent(eventIdentitySuspensionFailed, unknownID).Str(eventKeyError),
		"the suspend's failure class must be published")
	ts.Equal(governanceRefusal, attemptAdministrationFlow(ts.T(), unsuspendFlowHandle, unknownID),
		"an unsuspend of an unknown identity must be refused")
	ts.Equal("not_found", ts.awaitEvent(eventIdentityUnsuspensionFailed, unknownID).Str(eventKeyError),
		"the unsuspend's failure class must be published")
	ts.Zero(criteriaRowCount(ts.T(), "subject", unknownID), "a refused suspend must revoke nothing")
	ts.Zero(runtimeRowCount(ts.T(), unknownID), "nothing may be held for an unknown identity")
	ts.Zero(ts.countLines(auditUserSuspend, unknownID), "an unknown identity must write no audit line")

	// The refusals awaited above came after the attempts without a subject, so every event those
	// attempts could have published has been read by now. The sink flushes on a ticker, so an earlier
	// test's event can land after the reset; only one naming no subject comes from these attempts.
	events, err := ts.reader.Read()
	ts.Require().NoError(err)
	for _, evt := range events {
		switch evt.Type {
		case eventIdentitySuspended, eventIdentitySuspensionFailed, eventIdentityUnsuspended,
			eventIdentityUnsuspensionFailed:
			ts.NotEmpty(evt.Str(eventKeySubject), "an operation without a subject must publish nothing")
		}
	}
}

// The operator note is capped at operatorNoteCap characters, counted as characters rather than bytes.
// An over-long note is refused before containment, so the account stays active and its token keeps
// working.
func (ts *GovernanceEventsTestSuite) TestAnOverLongOperatorNoteIsRefusedBeforeContainment() {
	refusedID, refusedName, refusedPassword := ts.fixture.newUser(ts.T(), "long_note", nil)
	acceptedID, acceptedName, acceptedPassword := ts.fixture.newUser(ts.T(), "longest_note", nil)
	refusedHolder := ts.holderStatus(refusedName, refusedPassword)
	acceptedHolder := ts.holderStatus(acceptedName, acceptedPassword)
	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	ts.resetReaders()

	answer := attemptAdministrationFlowAs(ts.T(), token, suspendFlowHandle, map[string]string{
		subjectInput:      refusedID,
		operatorNoteInput: strings.Repeat("a", operatorNoteCap+1),
	})
	ts.Equal(governanceRefusal, answer, "a note over the cap must be refused")
	ts.Equal("note_too_long", ts.awaitEvent(eventIdentitySuspensionFailed, refusedID).Str(eventKeyError),
		"the failure class must be published")
	ts.Equal("ACTIVE", readStatus(ts.T(), refusedID).Value, "a refused note must hold nothing")
	ts.Zero(criteriaRowCount(ts.T(), "subject", refusedID), "a refused note must revoke nothing")

	// Two-byte characters, so a byte count would refuse it.
	longest := strings.Repeat("é", operatorNoteCap)
	runAdministrationFlowAs(ts.T(), adminToken(ts.T()), suspendFlowHandle,
		map[string]string{subjectInput: acceptedID, operatorNoteInput: longest})
	held := readStatus(ts.T(), acceptedID)
	ts.Equal("SUSPENDED", held.Value, "a note at the cap must be accepted")
	ts.Require().NotNil(held.Details)
	ts.Equal(longest, held.Details.OperatorNote, "a note at the cap must be kept whole")

	// The accepted suspension reaching the REST API shows the deny list was refreshed past anything the
	// refused attempt could have written.
	ts.Require().Eventually(func() bool { return acceptedHolder() == http.StatusUnauthorized },
		denyListRefreshWait, 250*time.Millisecond, "the accepted suspension must reach the REST API")
	ts.Equal(http.StatusOK, refusedHolder(), "the refused suspension must leave the holder's token working")
}

// holderStatus signs the account in and returns a probe of the status /users/me answers its token with.
func (ts *GovernanceEventsTestSuite) holderStatus(username, password string) func() int {
	ts.T().Helper()
	holder, err := testutils.GetHTTPClientForOUUser(ts.fixture.signInClientID, username, password)
	ts.Require().NoError(err, "the holder must sign in")
	probe := func() int {
		status, _ := getWith(ts.T(), holder, "/users/me")
		return status
	}
	ts.Require().Equal(http.StatusOK, probe(), "the holder's token must work before the suspension")
	return probe
}

// A recovery release is published as a self-service unlock, with no operator.
func (ts *GovernanceEventsTestSuite) TestARecoveryReleaseIsPublishedAsSelfService() {
	userID, username, _ := ts.fixture.newUser(ts.T(), "self_unlock", nil)
	failAuthentication(ts.T(), "username", username, shippedUserLockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the account must be locked first")
	ts.resetReaders()

	ts.fixture.recover(ts.T(), username, "Recovered-Events-1!")
	released := ts.awaitEvent(eventIdentitySelfUnlocked, userID)
	ts.Equal("success", released.Status)
	ts.Empty(released.Str(eventKeyActorSub), "a self-service release has no operator")

	line := ts.awaitLine(auditUserUnlock, userID)
	user := testutils.AuditRef{Type: auditTargetUser, ID: userID}
	ts.Equal(user, line.Actor, "a recovery release names the released identity as actor")
	ts.Equal(user, line.Subject)
	ts.Equal(auditResultSucceeded, line.Outcome["result"])
	ts.Equal(map[string]string{"self_service": "true"}, line.Details, "a recovery release is self-service")
}

// A lock is published once with no operator: wrong passwords during it and the refused correct one
// publish nothing more before the unlock.
func (ts *GovernanceEventsTestSuite) TestALockIsPublishedOnceWithoutAnOperator() {
	userID, username, password := ts.fixture.newUser(ts.T(), "locked", nil)
	ts.resetReaders()

	failAuthentication(ts.T(), "username", username, shippedUserLockThreshold)
	locked := ts.awaitEvent(eventIdentityLocked, userID)
	ts.Equal("success", locked.Status)
	ts.Equal("user", locked.Str(eventKeySubjectType), "the target's category must be published")
	ts.Equal("credential", locked.Str(eventKeyLockMethod), "the locked method must be published")
	ts.Equal(float64(1), locked.Data[eventKeyLockCount], "the first lock must be published as the first rung")
	ts.Equal(false, locked.Data[eventKeyLockPermanent], "the shipped first rung expires")
	ts.False(locked.Has(eventKeyActorSub), "a lock failed sign-ins formed has no operator to publish")

	user := testutils.AuditRef{Type: auditTargetUser, ID: userID}
	lockLine := ts.awaitLine(auditUserLock, userID)
	ts.Equal("lock", lockLine.Action)
	ts.Equal(testutils.AuditRef{Type: "anonymous"}, lockLine.Actor, "failed sign-ins have no operator")
	ts.Equal(user, lockLine.Target)
	ts.Equal(user, lockLine.Subject)
	ts.Equal(auditResultSucceeded, lockLine.Outcome["result"])
	ts.Equal(map[string]string{"authentication_method": "credential"}, lockLine.Details)
	ts.NotContains(lockLine.Raw, username, "the attempted username must never be written")

	expiry, reported := readStatus(ts.T(), userID).expiryOf("credential")
	ts.Require().True(reported, "the lock must report its expiry")
	unlockAt, err := time.Parse(time.RFC3339, locked.Str(eventKeyUnlockAt))
	ts.Require().NoError(err, "unlock_at must be an RFC 3339 instant, got %q", locked.Str(eventKeyUnlockAt))
	ts.True(expiry.Equal(unlockAt), "unlock_at %s must be the lock's expiry %s", unlockAt, expiry)

	failHeldAuthentication(ts.T(), "username", username, shippedUserLockThreshold, statedLockCode)
	requireRefusedAsHeld(ts.T(), authenticate(ts.T(), "username", username, password), statedLockCode,
		"the correct password must be refused while the lock is live")
	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	unlocked := ts.awaitEvent(eventIdentityUnlocked, userID)
	ts.Equal(ts.adminSub, unlocked.Str(eventKeyActorSub), "an operator unlock must name the operator")
	ts.Equal("user", unlocked.Str(eventKeySubjectType), "the target's category must be published")
	ts.Equal(1, ts.countEvents(eventIdentityLocked, userID), "one lock must be published once")
	ts.Equal(1, ts.countLines(auditUserLock, userID), "one lock must write one line")
}

// Agents write the same lines as users: the lock is anonymous, the operator's suspend, unsuspend and
// unlock name the operator, and the agent is target and subject.
func (ts *GovernanceEventsTestSuite) TestAnAgentsLifecycleAndLockAreWrittenToTheAuditLog() {
	t := ts.T()
	applyPolicy(t, agentLockPolicy())
	snapshot := widenDefaultAgentType(t)
	agentID, username, _ := newPasswordAgent(t, snapshot.OUID, "audit", ts.fixture.suffix)
	ts.resetReaders()

	failAuthentication(t, "agent_username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readEntityStatus(t, agentCollection, agentID).Value,
		"the agent must be locked")
	agent := testutils.AuditRef{Type: auditTargetAgent, ID: agentID}
	locked := ts.awaitLine(auditAgentLock, agentID)
	ts.Equal(testutils.AuditRef{Type: "anonymous"}, locked.Actor, "failed sign-ins have no operator")
	ts.Equal(agent, locked.Target)
	ts.Equal(agent, locked.Subject)
	ts.Equal(auditResultSucceeded, locked.Outcome["result"])
	ts.Equal(map[string]string{"authentication_method": "credential"}, locked.Details)

	runAdministrationFlow(t, agentSuspendFlowHandle, agentID)
	runAdministrationFlow(t, agentUnsuspendFlowHandle, agentID)
	runAdministrationFlow(t, agentUnlockFlowHandle, agentID)
	wantDetails := map[string]map[string]string{
		auditAgentSuspend:   {"applied_by": "organization"},
		auditAgentUnsuspend: {"applied_by": "organization"},
		auditAgentUnlock:    {"self_service": "false"},
	}
	for _, name := range []string{auditAgentSuspend, auditAgentUnsuspend, auditAgentUnlock} {
		ts.requireOperatorLine(name, agent, ts.adminSub, wantDetails[name])
	}
}

// An accountAccess write publishes the operator and the section and never the value; a refused
// write publishes its error code.
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
	ts.resetReaders()

	const marker = 7364519
	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection,
		[]byte(fmt.Sprintf(`{"user":{"lastLoginResolutionSeconds":%d}}`, marker))))
	updated := ts.awaitEvent(eventServerConfigUpdated, "")
	ts.Equal(accountAccessSection, updated.Str(eventKeyConfigName), "the section must be published")
	// A data plane takes server configuration only from an import with the management API key, which
	// writes as its own principal.
	ts.Equal("management-api-key", updated.Str(eventKeyActorSub), "the operator must be published")
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

func (ts *GovernanceEventsTestSuite) TestConcurrentFailuresPublishOneLockEpisode() {
	userID, username, _ := ts.fixture.newUser(ts.T(), "concurrent_events", nil)

	failAuthenticationConcurrently(ts.T(), "username", username, shippedUserLockThreshold*4)

	locked := ts.awaitEvent(eventIdentityLocked, userID)
	ts.Equal(float64(1), locked.Data[eventKeyLockCount], "the burst must publish the first episode")
	ts.Equal("credential", locked.Str(eventKeyLockMethod))
	failHeldAuthentication(ts.T(), "username", username, shippedUserLockThreshold, statedLockCode)
	// This later event is the barrier for every lock event the completed burst published.
	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.awaitEvent(eventIdentityUnlocked, userID)
	ts.Equal(1, ts.countEvents(eventIdentityLocked, userID), "one episode must publish one lock event")
}

// resetReaders starts a fresh window on the event and audit readers.
func (ts *GovernanceEventsTestSuite) resetReaders() {
	ts.T().Helper()
	ts.Require().NoError(ts.reader.Reset())
	ts.Require().NoError(ts.audit.Reset())
}

// requireOperatorLine waits for the audit line of the event about the target and requires the operator
// as actor, the target as subject, success and the details.
func (ts *GovernanceEventsTestSuite) requireOperatorLine(event string, target testutils.AuditRef,
	operatorSub string, details map[string]string) {
	ts.T().Helper()
	line := ts.awaitLine(event, target.ID)
	ts.Equal(event[strings.Index(event, ".")+1:], line.Action)
	ts.Equal(testutils.AuditRef{Type: auditTargetUser, ID: operatorSub}, line.Actor,
		"%s must name the operator", event)
	ts.Equal(target, line.Target, "%s must name the identity", event)
	ts.Equal(target, line.Subject, "%s must belong to the identity's activity", event)
	ts.Equal(auditResultSucceeded, line.Outcome["result"], "%s must succeed", event)
	ts.Equal(details, line.Details, "%s must carry its details", event)
}

// awaitLine waits for an audit line of the event about the target ID.
func (ts *GovernanceEventsTestSuite) awaitLine(event, targetID string) testutils.AuditLine {
	ts.T().Helper()
	line, err := ts.audit.Await(func(line testutils.AuditLine) bool {
		return line.Event == event && line.Target.ID == targetID
	})
	ts.Require().NoError(err, "no %s line for %q was written", event, targetID)
	return line
}

// countLines counts the audit lines of the event about the target ID read since the last reset. It does
// not wait, so the caller first awaits a line written after the ones it counts.
func (ts *GovernanceEventsTestSuite) countLines(event, targetID string) int {
	ts.T().Helper()
	lines, err := ts.audit.Read()
	ts.Require().NoError(err)
	matched := 0
	for _, line := range lines {
		if line.Event == event && line.Target.ID == targetID {
			matched++
		}
	}
	return matched
}
