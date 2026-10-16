// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	eventServerConfigUpdated        = "SERVER_CONFIG_UPDATED"
	eventServerConfigUpdateFailed   = "SERVER_CONFIG_UPDATE_FAILED"

	eventKeySubject        = "sub"
	eventKeySubjectType    = "sub_type"
	eventKeyActorSub       = "act_sub"
	eventKeyNoteRecorded   = "operator_note_recorded"
	eventKeyError          = "error"
	eventKeyConfigName     = "config_name"
	governanceEventTimeout = 30 * time.Second
)

// GovernanceEventsTestSuite checks what the lifecycle operations and an accountAccess write publish,
// read back from the observability file sink: the operator and the target, the outcome and its
// failure class, and never the operator note or the written value.
type GovernanceEventsTestSuite struct {
	suite.Suite
	fixture  *heldRecoveryFixture
	reader   *governanceEventReader
	adminSub string
}

func TestGovernanceEventsTestSuite(t *testing.T) {
	suite.Run(t, new(GovernanceEventsTestSuite))
}

func (ts *GovernanceEventsTestSuite) SetupSuite() {
	t := ts.T()
	t.Cleanup(func() { restoreShippedPolicy(t) })
	require.NoError(t, testutils.PatchDeploymentConfig(map[string]interface{}{
		"observability": map[string]interface{}{
			"enabled": true,
			"output": map[string]interface{}{
				"file": map[string]interface{}{
					"enabled":    true,
					"file_path":  governanceEventLogPath(),
					"format":     "json",
					"categories": []string{"observability.all"},
				},
			},
		},
	}), "failed to enable the observability file sink")
	require.NoError(t, testutils.RestartServer(), "failed to restart with observability enabled")
	require.NoError(t, testutils.ObtainAdminAccessToken(), "failed to re-obtain the admin token")

	token, err := testutils.GetAccessToken()
	require.NoError(t, err)
	claims, err := testutils.DecodeJWT(token)
	require.NoError(t, err, "the admin token must be a JWT")
	require.NotEmpty(t, claims.Sub, "the admin token must name its subject")
	ts.adminSub = claims.Sub

	ts.reader = &governanceEventReader{path: governanceEventLogPath()}
	ts.fixture = newHeldRecoveryFixture(t, "events")
}

// Suspend, unsuspend and unlock each publish their success event with the operator as act_sub
// and the target as sub. The operator note is published only as whether one was given.
func (ts *GovernanceEventsTestSuite) TestLifecycleOperationsArePublishedWithTheOperatorAndTarget() {
	userID, _, _ := ts.fixture.newUser(ts.T(), "lifecycle")
	note := "events-note-" + ts.fixture.suffix
	ts.reader.reset(ts.T())

	runAdministrationFlowWithInputs(ts.T(), suspendFlowHandle,
		map[string]string{subjectInput: userID, operatorNoteInput: note})
	suspended := ts.reader.await(ts.T(), eventIdentitySuspended, userID)
	ts.Equal(ts.adminSub, suspended.str(eventKeyActorSub), "the operator must be act_sub")
	ts.Equal("user", suspended.str(eventKeySubjectType), "the target's category must be published")
	ts.Equal("success", suspended.Status)
	ts.Equal(true, suspended.Data[eventKeyNoteRecorded], "whether a note was given is published")

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	unsuspended := ts.reader.await(ts.T(), eventIdentityUnsuspended, userID)
	ts.Equal(ts.adminSub, unsuspended.str(eventKeyActorSub), "the operator must be act_sub")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	unlocked := ts.reader.await(ts.T(), eventIdentityUnlocked, userID)
	ts.Equal(ts.adminSub, unlocked.str(eventKeyActorSub), "the operator must be act_sub")

	ts.NotContains(ts.reader.everything(ts.T()), note, "the operator note must never be published")
}

// A refused operation publishes its failure event with the failure class.
func (ts *GovernanceEventsTestSuite) TestARefusedOperationIsPublishedWithItsFailureClass() {
	userID, _, _ := ts.fixture.newUser(ts.T(), "refused")
	ts.reader.reset(ts.T())

	status, flowStatus := attemptAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	ts.Require().False(status == http.StatusOK && flowStatus == "COMPLETE", "unsuspend of an active account must be refused")

	failed := ts.reader.await(ts.T(), eventIdentityUnsuspensionFailed, userID)
	ts.Equal("failure", failed.Status)
	ts.Equal("not_applicable", failed.str(eventKeyError), "the failure class must be published")
	ts.Equal(ts.adminSub, failed.str(eventKeyActorSub), "a refusal must name the operator too")
}

// A suspend refused at its pre-check publishes its failure event with the operator and the failure
// class, and revokes nothing: the operator's own token keeps working.
func (ts *GovernanceEventsTestSuite) TestASuspendRefusedAtItsPreCheckIsPublishedAndRevokesNothing() {
	ts.reader.reset(ts.T())

	status, flowStatus := attemptAdministrationFlow(ts.T(), suspendFlowHandle, ts.adminSub)
	ts.Require().False(status == http.StatusOK && flowStatus == "COMPLETE",
		"an operator suspending themselves must be refused")

	failed := ts.reader.await(ts.T(), eventIdentitySuspensionFailed, ts.adminSub)
	ts.Equal("failure", failed.Status)
	ts.Equal("not_authorized", failed.str(eventKeyError), "the failure class must be published")
	ts.Equal(ts.adminSub, failed.str(eventKeyActorSub), "a refusal must name the operator too")

	// Past the deny list refresh a revocation would be enforced after.
	ts.Never(func() bool { return !ts.adminTokenWorks() }, 6*time.Second, 500*time.Millisecond,
		"a refused pre-check must revoke none of the operator's tokens")
	ts.Equal("ACTIVE", readStatus(ts.T(), ts.adminSub).Value, "a refused suspend must hold nothing")
}

// adminTokenWorks reports whether the operator token the suite runs with is still accepted.
func (ts *GovernanceEventsTestSuite) adminTokenWorks() bool {
	resp, err := testutils.GetHTTPClient().Get(testServerURL + "/users/" + ts.adminSub)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

// A recovery release is published as self service, with no operator.
func (ts *GovernanceEventsTestSuite) TestARecoveryReleaseIsPublishedAsSelfService() {
	userID, username, _ := ts.fixture.newUser(ts.T(), "self_unlock")
	failAuthentication(ts.T(), "username", username, shippedUserLockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the account must be locked first")
	ts.reader.reset(ts.T())

	ts.fixture.recover(ts.T(), username, "Recovered-Events-1!")
	released := ts.reader.await(ts.T(), eventIdentitySelfUnlocked, userID)
	ts.Equal("success", released.Status)
	ts.Empty(released.str(eventKeyActorSub), "a self-service release has no operator")
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
	ts.reader.reset(ts.T())

	const marker = 7364519
	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection,
		[]byte(fmt.Sprintf(`{"user":{"lastLoginResolutionSeconds":%d}}`, marker))))
	updated := ts.reader.await(ts.T(), eventServerConfigUpdated, "")
	ts.Equal(accountAccessSection, updated.str(eventKeyConfigName), "the section must be published")
	ts.Equal(ts.adminSub, updated.str(eventKeyActorSub), "the operator must be published")
	raw, err := json.Marshal(updated)
	ts.Require().NoError(err)
	ts.NotContains(string(raw), fmt.Sprint(marker), "the written value must never be published")

	status, _ := putAccountAccessSection(ts.T(), []byte(`{"user":{"default":{"lockThreshold":2}}}`))
	ts.Require().Equal(http.StatusBadRequest, status, "a misspelled key must be refused")
	failed := ts.reader.await(ts.T(), eventServerConfigUpdateFailed, "")
	ts.Equal(accountAccessSection, failed.str(eventKeyConfigName))
	ts.Equal("SCF-1003", failed.str(eventKeyError), "a refused write must publish its error code")
}

// --- event log reader ---

// governanceEventLogPath is where the suite points the file sink, under the extracted product home.
func governanceEventLogPath() string {
	return filepath.Join(testutils.GetExtractedProductHome(), "logs", "observability", "governance-events.log")
}

// governanceEvent mirrors one record the file sink writes.
type governanceEvent struct {
	Type   string                 `json:"type"`
	Status string                 `json:"status"`
	Data   map[string]interface{} `json:"data"`
}

func (e governanceEvent) str(key string) string {
	value, _ := e.Data[key].(string)
	return value
}

// governanceEventReader reads the events appended after its last reset. The sink buffers and flushes
// on a timer, so reads poll; only complete lines are parsed.
type governanceEventReader struct {
	path   string
	offset int
}

// reset moves past every complete record written so far.
func (r *governanceEventReader) reset(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		r.offset = 0
		return
	}
	require.NoError(t, err, "failed to read the event log")
	r.offset = bytes.LastIndexByte(data, '\n') + 1
}

// since returns the complete lines written after the last reset.
func (r *governanceEventReader) since(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err, "failed to read the event log")
	if len(data) <= r.offset {
		return nil
	}
	chunk := string(data[r.offset:])
	last := strings.LastIndex(chunk, "\n")
	if last < 0 {
		return nil
	}
	return strings.Split(chunk[:last], "\n")
}

// everything waits one flush and returns every complete line written since the last reset.
func (r *governanceEventReader) everything(t *testing.T) string {
	t.Helper()
	time.Sleep(6 * time.Second)
	return strings.Join(r.since(t), "\n")
}

// await polls until an event of the type, and with the subject when one is given, is written.
func (r *governanceEventReader) await(t *testing.T, eventType, subject string) governanceEvent {
	t.Helper()
	deadline := time.Now().Add(governanceEventTimeout)
	for {
		for _, line := range r.since(t) {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var evt governanceEvent
			require.NoError(t, json.Unmarshal([]byte(line), &evt), "unreadable event: %s", line)
			if evt.Type == eventType && (subject == "" || evt.str(eventKeySubject) == subject) {
				return evt
			}
		}
		if time.Now().After(deadline) {
			require.FailNow(t, "event not published", "no %s for %q within %s", eventType, subject,
				governanceEventTimeout)
		}
		time.Sleep(250 * time.Millisecond)
	}
}
