// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Audit event names, target types and outcome keys this file asserts. They mirror the server's audit
// package, which a separate module cannot import.
const (
	auditUserLock      = "user.lock"
	auditUserUnlock    = "user.unlock"
	auditUserSuspend   = "user.suspend"
	auditUserUnsuspend = "user.unsuspend"

	auditTargetUser = "user"

	auditResultSucceeded = "succeeded"
	auditResultDenied    = "denied"
	auditResultFailed    = "failed"
)

// GovernanceAuditTestSuite checks the audit lines the lifecycle operations and a lock formation write to
// the server's standard output: the operator or an anonymous actor, the identity as target and subject,
// the outcome and its reason, and never the operator note.
type GovernanceAuditTestSuite struct {
	suite.Suite
	fixture  *heldRecoveryFixture
	reader   *testutils.AuditLogReader
	adminSub string
}

func TestGovernanceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(GovernanceAuditTestSuite))
}

// SetupSuite restarts the server so this process captures its standard output.
func (ts *GovernanceAuditTestSuite) SetupSuite() {
	t := ts.T()
	require.NoError(t, testutils.RestartServer(), "failed to restart the server so its output is captured")
	require.NoError(t, testutils.ObtainAdminAccessToken(), "failed to re-obtain the admin token")

	token, err := testutils.GetAccessToken()
	require.NoError(t, err)
	claims, err := testutils.DecodeJWT(token)
	require.NoError(t, err, "the admin token must be a JWT")
	require.NotEmpty(t, claims.Sub, "the admin token must name its subject")
	ts.adminSub = claims.Sub

	ts.reader = testutils.NewAuditLogReader()
	ts.fixture = newHeldRecoveryFixture(t, "audit")
}

// Suspend, unsuspend and unlock through the administration flows write one line each, with the
// operator as actor and the user as target and subject. The operator note appears in no line.
func (ts *GovernanceAuditTestSuite) TestLifecycleOperationsWriteOneLineEach() {
	userID, _, _ := ts.fixture.newUser(ts.T(), "lifecycle", nil)
	note := "audit-note-" + ts.fixture.suffix
	ts.Require().NoError(ts.reader.Reset())

	runAdministrationFlowAs(ts.T(), adminToken(ts.T()), suspendFlowHandle,
		map[string]string{subjectInput: userID, operatorNoteInput: note})
	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)

	user := testutils.AuditRef{Type: auditTargetUser, ID: userID}
	admin := testutils.AuditRef{Type: auditTargetUser, ID: ts.adminSub}
	for _, name := range []string{auditUserSuspend, auditUserUnsuspend, auditUserUnlock} {
		line := ts.awaitLine(name, userID)
		ts.Equal(strings.TrimPrefix(name, "user."), line.Action)
		ts.Equal(admin, line.Actor, "%s must name the operator", name)
		ts.Equal(user, line.Target, "%s must name the user", name)
		ts.Equal(user, line.Subject, "%s must belong to the user's activity", name)
		ts.Equal(auditResultSucceeded, line.Outcome["result"], "%s must succeed", name)
		ts.Empty(line.Details, "%s must carry no details", name)
	}

	// The unlock was the last request, so every line the suspend wrote has been read by now.
	lines, err := ts.reader.Read()
	ts.Require().NoError(err)
	for _, line := range lines {
		ts.NotContains(line.Raw, note, "the operator note must never be written")
	}
	ts.Equal(1, ts.countLines(auditUserSuspend, userID), "one suspend must write one line")
}

// A refused operation is written with its outcome: an operator suspending themselves is denied, an
// unsuspend of an active account fails with its reason, and an unknown identity writes no line since its
// category, and so its event, is unknown.
func (ts *GovernanceAuditTestSuite) TestRefusedOperationsAreWrittenWithTheirOutcome() {
	activeID, _, _ := ts.fixture.newUser(ts.T(), "refused", nil)
	rootID, _, _, rootToken := newRootOperator(ts.T(), ts.fixture.userFixture)
	unknownID := fmt.Sprintf("00000000-0000-4000-8000-%012d", time.Now().UnixNano()%1_000_000_000_000)
	ts.Require().NoError(ts.reader.Reset())

	ts.Require().Equal(governanceRefusal, attemptAdministrationFlowAs(ts.T(), rootToken, suspendFlowHandle,
		map[string]string{subjectInput: rootID}), "an operator suspending themselves must be refused")
	denied := ts.awaitLine(auditUserSuspend, rootID)
	root := testutils.AuditRef{Type: auditTargetUser, ID: rootID}
	ts.Equal(root, denied.Actor)
	ts.Equal(root, denied.Subject, "a denied attempt names the identity it was refused on")
	ts.Equal(map[string]string{"result": auditResultDenied, "reason": "forbidden"}, denied.Outcome)

	ts.Require().Equal(governanceRefusal, attemptAdministrationFlow(ts.T(), suspendFlowHandle, unknownID),
		"a suspend of an unknown identity must be refused")

	ts.Require().Equal(governanceRefusal, attemptAdministrationFlow(ts.T(), unsuspendFlowHandle, activeID),
		"an unsuspend of an active account must be refused")
	failed := ts.awaitLine(auditUserUnsuspend, activeID)
	ts.Equal(testutils.AuditRef{Type: auditTargetUser, ID: activeID}, failed.Target)
	ts.Equal(testutils.AuditRef{}, failed.Subject, "a failed attempt names no subject")
	ts.Equal(map[string]string{"result": auditResultFailed, "reason": "igs_1002"}, failed.Outcome)

	ts.Zero(ts.countLines(auditUserSuspend, unknownID), "an unknown identity must write no line")
}

// Failed sign-ins that reach the threshold write one lock line with no operator and the locked method,
// and a recovery that releases the lock writes an unlock line with no operator either.
func (ts *GovernanceAuditTestSuite) TestALockAndItsRecoveryReleaseAreWrittenWithoutAnOperator() {
	userID, username, _ := ts.fixture.newUser(ts.T(), "locked", nil)
	ts.Require().NoError(ts.reader.Reset())

	failAuthentication(ts.T(), "username", username, shippedUserLockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the account must be locked")
	locked := ts.awaitLine(auditUserLock, userID)
	user := testutils.AuditRef{Type: auditTargetUser, ID: userID}
	ts.Equal("lock", locked.Action)
	ts.Equal(testutils.AuditRef{Type: "anonymous"}, locked.Actor, "failed sign-ins have no operator")
	ts.Equal(user, locked.Target)
	ts.Equal(user, locked.Subject)
	ts.Equal(auditResultSucceeded, locked.Outcome["result"])
	ts.Equal(map[string]string{"authentication_method": "credential"}, locked.Details)
	ts.NotContains(locked.Raw, username, "the attempted username must never be written")

	ts.fixture.recover(ts.T(), username, "Recovered-Audit-1!")
	released := ts.awaitLine(auditUserUnlock, userID)
	ts.Equal(testutils.AuditRef{Type: "anonymous"}, released.Actor, "a self-service release has no operator")
	ts.Equal(user, released.Subject)
	ts.Equal(auditResultSucceeded, released.Outcome["result"])
	ts.Equal(1, ts.countLines(auditUserLock, userID), "one lock must write one line")
}

// awaitLine waits for an audit line of the event about the target ID.
func (ts *GovernanceAuditTestSuite) awaitLine(event, targetID string) testutils.AuditLine {
	ts.T().Helper()
	line, err := ts.reader.Await(func(line testutils.AuditLine) bool {
		return line.Event == event && line.Target.ID == targetID
	})
	ts.Require().NoError(err, "no %s line for %q was written", event, targetID)
	return line
}

// countLines counts the audit lines of the event about the target ID read since the last reset. It does
// not wait, so the caller first awaits a line written after the ones it counts.
func (ts *GovernanceAuditTestSuite) countLines(event, targetID string) int {
	ts.T().Helper()
	lines, err := ts.reader.Read()
	ts.Require().NoError(err)
	matched := 0
	for _, line := range lines {
		if line.Event == event && line.Target.ID == targetID {
			matched++
		}
	}
	return matched
}
