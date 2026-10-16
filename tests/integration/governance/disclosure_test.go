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

// Stated holds, as the flow executors answer them.
const (
	flowStatedSuspensionCode = "FET-1114"
	flowStatedLockCode       = "FET-1113"
)

// ShippedDisclosureTestSuite checks the refusals under the shipped policy, which no suite in this
// package otherwise runs under: every place a held holder can reach states the hold.
type ShippedDisclosureTestSuite struct {
	suite.Suite
	fixture *heldRecoveryFixture
}

func TestShippedDisclosureTestSuite(t *testing.T) {
	suite.Run(t, new(ShippedDisclosureTestSuite))
}

func (ts *ShippedDisclosureTestSuite) SetupSuite() {
	ts.fixture = newHeldRecoveryFixture(ts.T(), "shipped_disclosure")
}

// Under the shipped policy, a suspended account presenting its correct password is told it is
// suspended, with a refused sign-in's status.
func (ts *ShippedDisclosureTestSuite) TestSignInStatesTheSuspension() {
	_, other, _ := ts.fixture.newUser(ts.T(), "default_wrong")
	userID, username, password := ts.fixture.newUser(ts.T(), "default_suspended")
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	wrongSecret := authenticate(ts.T(), "username", other, "Not-The-Secret-1!")
	held := authenticate(ts.T(), "username", username, password)
	requireStatedHold(ts.T(), wrongSecret, held, statedSuspensionCode)
	ts.NotContains(held.body, statedLockCode, "a suspension must not be stated as a lock")
}

// Under the shipped policy, five failures lock, and the lock is stated.
func (ts *ShippedDisclosureTestSuite) TestSignInStatesTheLock() {
	userID, username, password := ts.fixture.newUser(ts.T(), "default_locked")
	failAuthentication(ts.T(), "username", username, shippedUserLockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value,
		"the shipped threshold must lock the account")

	wrongSecret := authenticate(ts.T(), "username", username, "Not-The-Secret-2!")
	held := authenticate(ts.T(), "username", username, password)
	requireStatedHold(ts.T(), wrongSecret, held, statedLockCode)
	ts.NotContains(held.body, statedSuspensionCode, "a lock must not be stated as a suspension")

	// An identifier that names no account keeps its own answer and is never stated as held.
	unknown := authenticate(ts.T(), "username", "no_such_user_"+ts.fixture.suffix, password)
	ts.Require().False(unknown.succeeded())
	ts.NotContains(unknown.body, statedLockCode, "an unknown identifier must not be stated as locked")
	ts.NotContains(unknown.body, statedSuspensionCode, "an unknown identifier must not be stated as suspended")
}

// Recovery states the suspension, and an unknown account is never stated as held.
func (ts *ShippedDisclosureTestSuite) TestRecoveryStatesTheSuspension() {
	userID, username, _ := ts.fixture.newUser(ts.T(), "default_recovery")
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	_, held := ts.fixture.submitRecoveryUsername(ts.T(), username)
	ts.Contains(held, flowStatedSuspensionCode, "recovery must state the suspension: %s", held)

	_, unknown := ts.fixture.submitRecoveryUsername(ts.T(), "no_such_user_"+ts.fixture.suffix)
	ts.NotContains(unknown, flowStatedSuspensionCode, "an unknown account must not be stated as held")
	ts.NotContains(unknown, flowStatedLockCode, "an unknown account must not be stated as held")
}

// Passkey enrolment states the suspension, with a refused sign-in's status.
func (ts *ShippedDisclosureTestSuite) TestPasskeyEnrolmentStatesTheSuspension() {
	userID, username, password := ts.fixture.newUser(ts.T(), "default_passkey")
	assertion, err := testutils.ObtainAuthAssertion(username, password)
	ts.Require().NoError(err, "the account must prove itself before the suspension")
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	status, body := startPasskeyEnrolment(ts.T(), userID, assertion)
	ts.Equal(http.StatusUnauthorized, status, "a stated hold keeps a refused sign-in's status: %s", body)
	ts.Contains(body, statedSuspensionCode, "the enrolment refusal must state the suspension")
}

// SuspensionDisclosureTestSuite checks that the sign-in flow keeps a held holder on the prompt with the
// hold stated, under the package policy with a non-expiring rung. It also carries a declarative user
// whose file starts it suspended, since this suite restarts the server anyway.
type SuspensionDisclosureTestSuite struct {
	suite.Suite
	fixture *heldRecoveryFixture
}

func TestSuspensionDisclosureTestSuite(t *testing.T) {
	suite.Run(t, new(SuspensionDisclosureTestSuite))
}

// declarativeSuspendedUser is a user whose declarative file sets the initial state SUSPENDED.
const (
	declarativeSuspendedUserID       = "governance-declarative-suspended"
	declarativeSuspendedUserPassword = "Declarative-Suspended-1!"
)

func (ts *SuspensionDisclosureTestSuite) SetupSuite() {
	t := ts.T()
	policy := accountAccessPatch()["accountAccess"].(map[string]interface{})
	user := policy["user"].(map[string]interface{})
	user["default"].(map[string]interface{})["lockDurationsSeconds"] = []int{0}

	// One cleanup removes the declarative user before the shipped policy's restart, so the file is
	// never loaded again; it is registered before anything changes.
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
	applyPolicy(t, policy)

	ts.fixture = newHeldRecoveryFixture(t, "stated_disclosure")
}

// Through the sign-in flow, a suspended holder presenting the correct password is returned to the
// prompt with the suspension stated, not sent to the client as an error.
func (ts *SuspensionDisclosureTestSuite) TestSignInFlowKeepsASuspendedHolderOnThePrompt() {
	userID, username, password := ts.fixture.newUser(ts.T(), "flow_suspended")
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	status, body := ts.fixture.signIn(ts.T(), username, password)
	requireRepromptedWith(ts.T(), status, body, flowStatedSuspensionCode)
	ts.NotContains(body, flowStatedLockCode, "a suspension must not be stated as a lock")
}

// Through the sign-in flow, a locked holder presenting the correct password is returned to the
// prompt with the lock stated.
func (ts *SuspensionDisclosureTestSuite) TestSignInFlowKeepsALockedHolderOnThePrompt() {
	userID, username, password := ts.fixture.newUser(ts.T(), "flow_locked")
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the threshold must lock the account")

	status, body := ts.fixture.signIn(ts.T(), username, password)
	requireRepromptedWith(ts.T(), status, body, flowStatedLockCode)
	ts.NotContains(body, flowStatedSuspensionCode, "a lock must not be stated as a suspension")
}

// A declarative file's state is where the identity starts: a missing
// runtime row reads as SUSPENDED and refuses, and once an operator releases it the runtime row wins
// over the file.
func (ts *SuspensionDisclosureTestSuite) TestADeclarativeInitialStateIsOnlyWhereTheIdentityStarts() {
	ts.Equal("SUSPENDED", readStatus(ts.T(), declarativeSuspendedUserID).Value,
		"the declarative initial state must be what a management read reports")
	held := authenticate(ts.T(), "username", declarativeSuspendedUserID, declarativeSuspendedUserPassword)
	ts.Require().False(held.succeeded(), "a declaratively suspended account must be refused")
	ts.Contains(held.body, statedSuspensionCode, "the refusal must be the suspension")

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, declarativeSuspendedUserID)
	ts.Equal("LOCKED", readStatus(ts.T(), declarativeSuspendedUserID).Value,
		"once released, the runtime row must win over the file's SUSPENDED")

	runAdministrationFlow(ts.T(), unlockFlowHandle, declarativeSuspendedUserID)
	ts.Equal("ACTIVE", readStatus(ts.T(), declarativeSuspendedUserID).Value, "the release must hold")
	ts.True(authenticate(ts.T(), "username", declarativeSuspendedUserID,
		declarativeSuspendedUserPassword).succeeded(), "the released declarative user must sign in")
}

// --- fixtures and helpers ---

// heldRecoveryFixture is an OU, a user type and an application whose recovery flow identifies the
// account, writes a credential and releases its locks, for suites that only need users and a way into
// recovery. It deletes itself when the test that built it ends.
type heldRecoveryFixture struct {
	name       string
	suffix     string
	ouID       string
	userTypeID string
	userType   string
	appID      string
	flowIDs    []string
	userIDs    []string
}

func newHeldRecoveryFixture(t *testing.T, name string) *heldRecoveryFixture {
	t.Helper()

	f := &heldRecoveryFixture{name: name, suffix: uniqueSuffix(), userType: "governance_" + name + "_user"}
	t.Cleanup(func() { f.tearDown(t) })
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "governance_" + name + "_ou",
		Name:        "Governance " + name + " OU",
		Description: "Organization unit for the " + name + " governance tests",
	})
	require.NoError(t, err, "failed to create the test organization unit")
	f.ouID = ouID

	typeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      f.userType,
		DisplayName: f.userType,
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	require.NoError(t, err, "failed to create the test user type")
	f.userTypeID = typeID

	recoveryFlowID, err := testutils.CreateFlow(heldRecoveryFlow("governance-" + name + "-recovery"))
	require.NoError(t, err, "failed to create the recovery flow")
	f.flowIDs = append(f.flowIDs, recoveryFlowID)

	// The bootstrap authentication flow calls the default recovery flow, so the app carries its own.
	authFlowID, err := testutils.CreateFlow(heldSignInFlow("governance-" + name + "-auth"))
	require.NoError(t, err, "failed to create the sign-in flow")
	f.flowIDs = append(f.flowIDs, authFlowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                  ouID,
		Name:                  "Governance " + name + " App",
		Description:           "Application the " + name + " governance tests recover through",
		IsRecoveryFlowEnabled: true,
		ClientID:              "governance_" + name + "_client",
		ClientSecret:          "governance_" + name + "_secret",
		RedirectURIs:          []string{"http://localhost:3000/callback"},
		AllowedUserTypes:      []string{f.userType},
		AuthFlowID:            authFlowID,
		RecoveryFlowID:        recoveryFlowID,
	})
	require.NoError(t, err, "failed to create the test application")
	f.appID = appID
	return f
}

// tearDown deletes what the fixture created, logging rather than failing.
func (f *heldRecoveryFixture) tearDown(t *testing.T) {
	t.Helper()
	if f.appID != "" {
		if err := testutils.DeleteApplication(f.appID); err != nil {
			t.Logf("teardown: failed to delete application: %v", err)
		}
	}
	for _, userID := range f.userIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			t.Logf("teardown: failed to delete user %s: %v", userID, err)
		}
	}
	for _, flowID := range f.flowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			t.Logf("teardown: failed to delete flow %s: %v", flowID, err)
		}
	}
	if f.userTypeID != "" {
		if err := testutils.DeleteUserType(f.userTypeID); err != nil {
			t.Logf("teardown: failed to delete user type: %v", err)
		}
	}
	if f.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(f.ouID); err != nil {
			t.Logf("teardown: failed to delete organization unit: %v", err)
		}
	}
}

// newUser creates one account of the fixture's type.
func (f *heldRecoveryFixture) newUser(t *testing.T, name string) (userID, username, password string) {
	t.Helper()

	username = fmt.Sprintf("%s_%s_%s", f.name, name, f.suffix)
	password = "Correct-Horse-" + name + "1!"
	attributes, err := json.Marshal(map[string]interface{}{"username": username, "password": password})
	require.NoError(t, err)

	userID, err = testutils.CreateUser(testutils.User{Type: f.userType, OUID: f.ouID, Attributes: attributes})
	require.NoError(t, err, "failed to create the test user")
	f.userIDs = append(f.userIDs, userID)
	return userID, username, password
}

// submitRecoveryUsername starts the recovery flow and submits a username, returning the raw answer
// whatever its status, since a stated refusal and an unknown-user answer are the subject.
func (f *heldRecoveryFixture) submitRecoveryUsername(t *testing.T, username string) (int, string) {
	t.Helper()

	step, err := common.InitiateRecoveryFlow(f.appID, false, nil, "")
	require.NoError(t, err, "failed to initiate the recovery flow")
	require.Equal(t, "INCOMPLETE", step.FlowStatus)

	return continueFlowRaw(t, map[string]interface{}{
		"executionId":    step.ExecutionID,
		"challengeToken": step.ChallengeToken,
		"inputs":         map[string]string{"username": username},
		"action":         "action_submit_username",
	})
}

// signIn starts the sign-in flow and submits a username and password, returning the raw answer.
func (f *heldRecoveryFixture) signIn(t *testing.T, username, password string) (int, string) {
	t.Helper()

	step, err := common.InitiateAuthenticationFlow(f.appID, false, nil, "")
	require.NoError(t, err, "failed to initiate the sign-in flow")
	require.Equal(t, "INCOMPLETE", step.FlowStatus)

	return continueFlowRaw(t, map[string]interface{}{
		"executionId":    step.ExecutionID,
		"challengeToken": step.ChallengeToken,
		"inputs":         map[string]string{"username": username, "password": password},
		"action":         "action_sign_in",
	})
}

// requireRepromptedWith checks a sign-in answer stayed on the prompt and carries the stated hold.
func requireRepromptedWith(t *testing.T, status int, body, code string) {
	t.Helper()

	require.Equal(t, http.StatusOK, status, body)
	var step common.FlowStep
	require.NoError(t, json.Unmarshal([]byte(body), &step), body)
	require.Equal(t, "INCOMPLETE", step.FlowStatus, "a stated hold must stay on the prompt: %s", body)
	require.Empty(t, step.Data.RedirectURL, "a stated hold must not leave the sign-in page")
	require.Contains(t, body, code, "the prompt must state the hold")
	require.Contains(t, body, `"identifier":"password"`, "the prompt must ask for the credentials again")
}

// recover drives the recovery flow to completion with a new password.
func (f *heldRecoveryFixture) recover(t *testing.T, username, newPassword string) {
	t.Helper()

	step, err := common.InitiateRecoveryFlow(f.appID, false, nil, "")
	require.NoError(t, err, "failed to initiate the recovery flow")
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username}, "action_submit_username", step.ChallengeToken)
	require.NoError(t, err, "failed to submit the username")
	require.Equal(t, "INCOMPLETE", step.FlowStatus, "the account must be identified")
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"password": newPassword}, "action_submit_password", step.ChallengeToken)
	require.NoError(t, err, "failed to submit the new password")
	require.Equal(t, "COMPLETE", step.FlowStatus, "the recovery must complete")
}

// continueFlowRaw posts one flow step and returns the status and body unparsed.
func continueFlowRaw(t *testing.T, payload map[string]interface{}) (int, string) {
	t.Helper()

	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	require.NoError(t, err, "flow request failed")
	defer resp.Body.Close()

	response, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(response)
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

// heldRecoveryFlow is the smallest recovery flow that identifies the account, writes a credential and
// releases its locks.
func heldRecoveryFlow(handle string) testutils.Flow {
	textInput := func(ref, identifier, inputType string) map[string]interface{} {
		return map[string]interface{}{"ref": ref, "identifier": identifier, "type": inputType, "required": true}
	}
	prompt := func(id, input, action, next string, inputType, identifier string) map[string]interface{} {
		return map[string]interface{}{
			"id":   id,
			"type": "PROMPT",
			"prompts": []map[string]interface{}{{
				"inputs": []map[string]interface{}{textInput(input, identifier, inputType)},
				"action": map[string]interface{}{"ref": action, "nextNode": next},
			}},
		}
	}
	return testutils.Flow{
		Name:     "Governance Held Recovery Flow " + handle,
		Handle:   handle,
		FlowType: "RECOVERY",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_username"},
			prompt("prompt_username", "input_username", "action_submit_username", "identify_user",
				"TEXT_INPUT", "username"),
			{
				"id":   "identify_user",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name":   "IdentifyingExecutor",
					"mode":   "identify",
					"inputs": []map[string]interface{}{textInput("input_username", "username", "TEXT_INPUT")},
				},
				"onSuccess": "prompt_new_password",
			},
			prompt("prompt_new_password", "input_new_password", "action_submit_password", "set_credential",
				"PASSWORD_INPUT", "password"),
			{
				"id":        "set_credential",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialSetter"},
				"onSuccess": "release_account_locks",
			},
			{
				"id":        "release_account_locks",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "IdentityGovernanceExecutor", "mode": "unlock"},
				"onSuccess": "recovery_complete",
			},
			{"id": "recovery_complete", "type": "PROMPT", "next": "end"},
			{"id": "end", "type": "END"},
		},
	}
}

// heldSignInFlow is the shipped sign-in shape: a credentials prompt the credentials node returns to
// when it is incomplete.
func heldSignInFlow(handle string) testutils.Flow {
	inputs := []map[string]interface{}{
		{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	return testutils.Flow{
		Name:     "Governance Held Sign-in Flow " + handle,
		Handle:   handle,
		FlowType: "AUTHENTICATION",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{{
					"inputs": inputs,
					"action": map[string]interface{}{"ref": "action_sign_in", "nextNode": "credentials_auth"},
				}},
			},
			{
				"id":           "credentials_auth",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": inputs},
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_credentials",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}
