// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The actions the passkey enrolment flow's prompts submit.
const (
	enrolPasskeyAction       = "action_enrol_passkey"
	passkeyAttestationAction = "action_attestation"
)

// The declarative suspended user, whose file sets the initial state SUSPENDED.
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

	status, body, err := testutils.StartPasskeyRegistrationRaw(userID, recoveryPasskeyRPID, recoveryPasskeyRPName,
		assertion)
	ts.Require().NoError(err, "the start request must be answered")
	ts.Equal(http.StatusUnauthorized, status, "a stated hold keeps a refused sign-in's status: %s", body)
	ts.Contains(string(body), statedSuspensionCode, "the enrolment refusal must state the suspension")
}

// A registration started before the suspension is refused at its finish, and stores nothing: once the
// holds are lifted, the authenticator it would have enrolled cannot sign in.
func (ts *DisclosureTestSuite) TestPasskeyEnrolmentFinishStatesTheSuspension() {
	userID, username, password := ts.fixture.newUser(ts.T(), "finish_passkey", nil)
	assertion, err := testutils.ObtainAuthAssertion(username, password)
	ts.Require().NoError(err, "the account must prove itself before the suspension")
	registration, err := testutils.StartPasskeyRegistration(userID, recoveryPasskeyRPID, recoveryPasskeyRPName,
		recoveryPasskeyOrigin, assertion)
	ts.Require().NoError(err, "the registration must start before the suspension")
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	status, body, err := registration.FinishRaw()
	ts.Require().NoError(err, "the finish request must be answered")
	ts.Equal(http.StatusUnauthorized, status, "a stated hold keeps a refused sign-in's status: %s", body)
	ts.Contains(string(body), statedSuspensionCode, "the enrolment refusal must state the suspension")

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.Require().Equal("ACTIVE", readStatus(ts.T(), userID).Value, "the holds must be lifted first")
	_, err = testutils.AuthenticateWithPasskey(userID, recoveryPasskeyRPID, registration.UserHandle,
		registration.Authenticator)
	ts.Error(err, "a refused registration must not have stored the passkey")
}

// A suspension placed after the password step stops a flow's passkey enrolment at its start, with the
// suspension stated.
func (ts *DisclosureTestSuite) TestPasskeyEnrolmentStartInAFlowStatesTheSuspension() {
	appID := ts.newPasskeyEnrolmentApp("flow_start")
	userID, username, password := ts.fixture.newUser(ts.T(), "flow_passkey_start", nil)
	step := ts.signInToTheEnrolmentPrompt(appID, username, password)
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	answer := answerOf(postFlow(ts.T(), nil, "", map[string]interface{}{
		"executionId":    step.ExecutionID,
		"action":         enrolPasskeyAction,
		"challengeToken": step.ChallengeToken,
	}))
	ts.Equal(flowAnswer{status: http.StatusOK, flowStatus: "ERROR", code: flowStatedSuspensionCode}, answer,
		"the enrolment start must fail with the suspension stated")
}

// A suspension placed between a flow's passkey enrolment start and finish refuses the finish, which
// states the suspension and asks for the attestation again rather than completing the flow.
func (ts *DisclosureTestSuite) TestPasskeyEnrolmentFinishInAFlowStatesTheSuspension() {
	appID := ts.newPasskeyEnrolmentApp("flow_finish")
	userID, username, password := ts.fixture.newUser(ts.T(), "flow_passkey_finish", nil)
	step := ts.signInToTheEnrolmentPrompt(appID, username, password)
	step, err := common.CompleteFlow(step.ExecutionID, map[string]string{}, enrolPasskeyAction, step.ChallengeToken)
	ts.Require().NoError(err, "failed to start the enrolment")
	raw, ok := step.Data.AdditionalData["passkeyCreationOptions"]
	ts.Require().True(ok, "the enrolment must start before the suspension: %+v", step)
	var options struct {
		Challenge string `json:"challenge"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(raw), &options), "unreadable passkey creation options")
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	authenticator, err := testutils.NewVirtualAuthenticator(recoveryPasskeyRPID, recoveryPasskeyOrigin)
	ts.Require().NoError(err)
	credentialID, clientDataJSON, attestationObject, err := authenticator.CreateAttestationResponse(
		options.Challenge, true)
	ts.Require().NoError(err, "failed to build the attestation")
	status, body := postFlow(ts.T(), nil, "", map[string]interface{}{
		"executionId": step.ExecutionID,
		"inputs": map[string]string{
			"credentialId": credentialID, "clientDataJSON": clientDataJSON, "attestationObject": attestationObject,
		},
		"action":         passkeyAttestationAction,
		"challengeToken": step.ChallengeToken,
	})
	refused := flowStepOf(ts.T(), status, body)

	ts.Equal("INCOMPLETE", refused.FlowStatus, "a refused finish must not complete the flow: %+v", refused)
	ts.Empty(refused.Assertion, "a refused finish must not sign the holder in")
	ts.Require().NotNil(refused.Error, "the finish must state the suspension: %+v", refused)
	ts.Equal(flowStatedSuspensionCode, refused.Error.Code, "the finish must state the suspension")
	ts.True(common.HasInput(refused.Data.Inputs, "attestationObject"),
		"the finish must ask for the attestation again: %+v", refused)
}

// newPasskeyEnrolmentApp creates an application whose sign-in flow enrols a passkey after the
// password, both deleted when the test ends.
func (ts *DisclosureTestSuite) newPasskeyEnrolmentApp(name string) string {
	ts.T().Helper()
	flowID, err := testutils.CreateFlow(passkeyEnrolmentSignInFlow(name + "-" + ts.fixture.suffix))
	ts.Require().NoError(err, "failed to create the passkey enrolment flow")
	cleanupFlow(ts.T(), flowID)
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:             ts.fixture.ouID,
		Name:             "Governance Disclosure Passkey " + name,
		Description:      "Application whose sign-in flow enrols a passkey",
		ClientID:         "governance_disclosure_passkey_" + name,
		ClientSecret:     "governance_disclosure_passkey_" + name + "_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{ts.fixture.userType},
		AuthFlowID:       flowID,
	})
	ts.Require().NoError(err, "failed to create the passkey enrolment application")
	cleanupApplication(ts.T(), appID)
	return appID
}

// signInToTheEnrolmentPrompt signs in with the password and requires the flow to stop at the prompt
// that starts the passkey enrolment.
func (ts *DisclosureTestSuite) signInToTheEnrolmentPrompt(appID, username, password string) *common.FlowStep {
	ts.T().Helper()
	status, body := submitSignIn(ts.T(), appID, signInAction, username, password)
	step := flowStepOf(ts.T(), status, body)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the password must be accepted: %+v", step)
	ts.Require().Nil(step.Error, "the password must be accepted: %+v", step)
	return step
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

// passkeyEnrolmentSignInFlow is a sign-in flow that, once the password is accepted, offers a passkey
// enrolment and runs its start and finish before asserting the account.
func passkeyEnrolmentSignInFlow(handleSuffix string) testutils.Flow {
	credentials := withExecutorInputs(taskNode("credentials_auth", "CredentialsAuthExecutor", "", "prompt_enrol",
		map[string]interface{}{"onIncomplete": "prompt_credentials"}), usernameInput, passwordInput)
	return testutils.Flow{
		Name:     "Governance Passkey Enrolment Flow " + handleSuffix,
		Handle:   "governance-passkey-enrolment-" + handleSuffix,
		FlowType: "AUTHENTICATION",
		Nodes: []map[string]interface{}{
			startNode("prompt_credentials"),
			promptNode("prompt_credentials", signInAction, "credentials_auth", usernameInput, passwordInput),
			credentials,
			{
				"id":   "prompt_enrol",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{{
					"inputs": []map[string]interface{}{},
					"action": map[string]interface{}{"ref": enrolPasskeyAction, "nextNode": "passkey_register_start"},
				}},
			},
			taskNode("passkey_register_start", "PasskeyAuthExecutor", "register_start", "prompt_attestation",
				map[string]interface{}{"properties": map[string]interface{}{
					"relyingPartyId": recoveryPasskeyRPID, "relyingPartyName": recoveryPasskeyRPName,
				}}),
			promptNode("prompt_attestation", passkeyAttestationAction, "passkey_register_finish",
				flowInput("input_credential_id", "credentialId", "TEXT_INPUT"),
				flowInput("input_client_data", "clientDataJSON", "TEXT_INPUT"),
				flowInput("input_attestation", "attestationObject", "TEXT_INPUT")),
			taskNode("passkey_register_finish", "PasskeyAuthExecutor", "register_finish", "auth_assert", nil),
			taskNode("auth_assert", "AuthAssertExecutor", "", "end", nil),
			endNode(),
		},
	}
}
