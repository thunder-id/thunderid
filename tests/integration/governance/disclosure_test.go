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
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// declarativeSuspendedUser is a user whose declarative file sets the initial state SUSPENDED.
const (
	declarativeSuspendedUserID       = "governance-declarative-suspended"
	declarativeSuspendedUserPassword = "Declarative-Suspended-1!"
)

// DisclosureTestSuite checks that every refusal a held holder can reach states the hold, under the
// shipped policy, and that the shipped sign-in flow keeps the holder on its prompt. It also carries a
// declarative user whose file starts it suspended, which needs the restart this suite makes.
type DisclosureTestSuite struct {
	suite.Suite
	fixture      *heldRecoveryFixture
	shippedAppID string
}

func TestDisclosureTestSuite(t *testing.T) {
	suite.Run(t, new(DisclosureTestSuite))
}

func (ts *DisclosureTestSuite) SetupSuite() {
	// Captured for the cleanup, which runs after the suite's tests.
	t := ts.T()

	// A fixture under resources/declarative_resources would load for every package, so this package's
	// user is written into the running pack instead. One cleanup removes it before the shipped policy's
	// restart, so it is never loaded again; it is registered before anything changes.
	userFile := filepath.Join(testutils.GetExtractedProductHome(), "config", "resources", "users",
		declarativeSuspendedUserID+".yaml")
	t.Cleanup(func() {
		if err := os.Remove(userFile); err != nil && !os.IsNotExist(err) {
			t.Errorf("cleanup: failed to remove the declarative user: %v", err)
		}
		restoreShippedPolicy(t)
	})
	require.NoError(t, os.WriteFile(userFile, []byte(fmt.Sprintf(`resource_type: user
id: %s
type: person
ouId: decl-ou-1
state: SUSPENDED
attributes:
  username: %s
  email: %s@example.com
credentials:
  password: %q
`, declarativeSuspendedUserID, declarativeSuspendedUserID, declarativeSuspendedUserID,
		declarativeSuspendedUserPassword)), 0600), "failed to write the declarative user")
	require.NoError(t, testutils.RestartServer(), "failed to restart with the declarative user")
	require.NoError(t, testutils.ObtainAdminAccessToken(), "failed to re-obtain the admin token")

	ts.fixture = newHeldRecoveryFixture(t, "disclosure")
	ts.shippedAppID = newShippedFlowApp(t, ts.fixture.userFixture, "disclosure")
}

func (ts *DisclosureTestSuite) TestSignInStatesTheSuspension() {
	_, other, _ := ts.fixture.newUser(ts.T(), "default_wrong", nil)
	userID, username, password := ts.fixture.newUser(ts.T(), "default_suspended", nil)
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	wrongSecret := authenticate(ts.T(), "username", other, "Not-The-Secret-1!")
	for _, presented := range []struct{ name, secret string }{
		{"the correct password", password},
		{"a wrong password", "Not-The-Secret-3!"},
	} {
		held := authenticate(ts.T(), "username", username, presented.secret)
		requireStatedHold(ts.T(), wrongSecret, held, statedSuspensionCode, "credential")
		ts.NotContains(held.body, statedLockCode, "%s: a suspension must not be stated as a lock",
			presented.name)
	}
}

// Five failures lock under the shipped policy.
func (ts *DisclosureTestSuite) TestSignInStatesTheLock() {
	userID, username, password := ts.fixture.newUser(ts.T(), "default_locked", nil)
	_, other, _ := ts.fixture.newUser(ts.T(), "default_unlocked", nil)
	failAuthentication(ts.T(), "username", username, shippedUserLockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value,
		"the shipped threshold must lock the account")

	wrongSecret := authenticate(ts.T(), "username", other, "Not-The-Secret-2!")
	for _, presented := range []struct{ name, secret string }{
		{"the correct password", password},
		{"a wrong password", "Not-The-Secret-2!"},
	} {
		held := authenticate(ts.T(), "username", username, presented.secret)
		requireStatedHold(ts.T(), wrongSecret, held, statedLockCode, "credential")
		ts.NotContains(held.body, statedSuspensionCode, "%s: a lock must not be stated as a suspension",
			presented.name)
	}

	unknown := authenticate(ts.T(), "username", "no_such_user_"+ts.fixture.suffix, password)
	ts.Require().False(unknown.succeeded())
	ts.NotContains(unknown.body, statedLockCode, "an unknown identifier must not be stated as locked")
	ts.NotContains(unknown.body, statedSuspensionCode, "an unknown identifier must not be stated as suspended")
}

func (ts *DisclosureTestSuite) TestRecoveryStatesTheSuspension() {
	userID, username, _ := ts.fixture.newUser(ts.T(), "default_recovery", nil)
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	_, held := ts.fixture.submitRecoveryUsername(ts.T(), username)
	ts.Contains(held, flowStatedSuspensionCode, "recovery must state the suspension: %s", held)

	_, unknown := ts.fixture.submitRecoveryUsername(ts.T(), "no_such_user_"+ts.fixture.suffix)
	ts.NotContains(unknown, flowStatedSuspensionCode, "an unknown account must not be stated as held")
	ts.NotContains(unknown, flowStatedLockCode, "an unknown account must not be stated as held")
}

func (ts *DisclosureTestSuite) TestPasskeyEnrolmentStatesTheSuspension() {
	userID, username, password := ts.fixture.newUser(ts.T(), "default_passkey", nil)
	assertion, err := testutils.ObtainAuthAssertion(username, password)
	ts.Require().NoError(err, "the account must prove itself before the suspension")
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	status, body := startPasskeyEnrolment(ts.T(), userID, assertion)
	ts.Equal(http.StatusUnauthorized, status, "a stated hold keeps a refused sign-in's status: %s", body)
	ts.Contains(body, statedSuspensionCode, "the enrolment refusal must state the suspension")
}

func (ts *DisclosureTestSuite) TestTheShippedSignInFlowKeepsASuspendedHolderOnThePrompt() {
	userID, username, password := ts.fixture.newUser(ts.T(), "flow_suspended", nil)
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	for _, secret := range []string{password, "Not-The-Secret-4!"} {
		status, body := submitSignIn(ts.T(), ts.shippedAppID, shippedSignInAction, username, secret)
		requireRepromptedWith(ts.T(), flowStepOf(ts.T(), status, body), flowStatedSuspensionCode)
		ts.NotContains(body, flowStatedLockCode, "a suspension must not be stated as a lock")
	}
}

// The lock is formed under a rung that never lapses, so the prompt cannot be reached through a lapse.
func (ts *DisclosureTestSuite) TestTheShippedSignInFlowKeepsALockedHolderOnThePrompt() {
	applyPolicy(ts.T(), lockoutPolicyWith(userDefaults(nonExpiringRung())))
	userID, username, password := ts.fixture.newUser(ts.T(), "flow_locked", nil)
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the threshold must lock the account")

	for _, secret := range []string{password, "Not-The-Secret-5!"} {
		status, body := submitSignIn(ts.T(), ts.shippedAppID, shippedSignInAction, username, secret)
		requireRepromptedWith(ts.T(), flowStepOf(ts.T(), status, body), flowStatedLockCode)
		ts.NotContains(body, flowStatedSuspensionCode, "a lock must not be stated as a suspension")
	}
}

// A missing runtime row reads as the file's SUSPENDED, and once an operator releases it the runtime
// row wins over the file. A user unsuspend lands on the recovery lock, so the release reads LOCKED
// until an unlock.
func (ts *DisclosureTestSuite) TestADeclarativeInitialStateIsOnlyWhereTheIdentityStarts() {
	ts.Equal("SUSPENDED", readStatus(ts.T(), declarativeSuspendedUserID).Value,
		"the declarative initial state must be what a management read reports")
	held := authenticate(ts.T(), "username", declarativeSuspendedUserID, declarativeSuspendedUserPassword)
	ts.Require().False(held.succeeded(), "a declaratively suspended account must be refused")
	ts.Contains(held.body, statedSuspensionCode, "the refusal must be the suspension")

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, declarativeSuspendedUserID)
	assertTheRecoveryLock(ts.T(), readStatus(ts.T(), declarativeSuspendedUserID),
		"once released, the runtime row must win over the file's SUSPENDED")

	runAdministrationFlow(ts.T(), unlockFlowHandle, declarativeSuspendedUserID)
	ts.Equal("ACTIVE", readStatus(ts.T(), declarativeSuspendedUserID).Value, "the release must hold")
	ts.True(authenticate(ts.T(), "username", declarativeSuspendedUserID,
		declarativeSuspendedUserPassword).succeeded(), "the released declarative user must sign in")
}

// flowStepOf requires a flow answer to have been given and returns the step it carries.
func flowStepOf(t *testing.T, status int, body string) *common.FlowStep {
	t.Helper()

	require.Equal(t, http.StatusOK, status, body)
	var step common.FlowStep
	require.NoError(t, json.Unmarshal([]byte(body), &step), body)
	return &step
}

// requireRepromptedWith checks a sign-in step stayed on the credentials prompt and carries the stated
// hold.
func requireRepromptedWith(t *testing.T, step *common.FlowStep, code string) {
	t.Helper()

	require.Equal(t, "INCOMPLETE", step.FlowStatus, "a stated hold must stay on the prompt: %+v", step)
	require.Empty(t, step.Data.RedirectURL, "a stated hold must not leave the sign-in page")
	require.NotNil(t, step.Error, "the prompt must state the hold: %+v", step)
	require.Equal(t, code, step.Error.Code, "the prompt must state the hold")
	require.True(t, common.HasInput(step.Data.Inputs, "password"),
		"the prompt must ask for the credentials again: %+v", step)
}

// startPasskeyEnrolment calls the direct passkey registration start endpoint with a proof of the user.
func startPasskeyEnrolment(t *testing.T, userID, assertion string) (int, string) {
	t.Helper()

	body, err := json.Marshal(map[string]interface{}{
		"userId":           userID,
		"relyingPartyId":   "localhost",
		"relyingPartyName": "Governance Tests",
		"assertion":        assertion,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/register/passkey/start", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	require.NoError(t, err, "passkey registration start failed")
	defer resp.Body.Close()

	response, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(response)
}
