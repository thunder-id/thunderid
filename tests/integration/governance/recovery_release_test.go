// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// recoveryWrongCodeErrorCode is the flow's answer to a wrong one-time code.
const recoveryWrongCodeErrorCode = "FET-1008"

// The passkey relying party: the origin is one the test deployment's passkey.allowed_origins lists, and
// the name is only a display name.
const (
	recoveryPasskeyRPID   = "localhost"
	recoveryPasskeyRPName = "ThunderID Test"
	recoveryPasskeyOrigin = "https://localhost:8095"
)

// RecoveryReleaseTestSuite checks that a completed recovery flow releases every lock and the
// ladder with it, only for the identity it proved, and a credential write outside recovery releases
// nothing.
type RecoveryReleaseTestSuite struct {
	suite.Suite
	smsServer   *testutils.MockNotificationServer
	smsSenderID string
	users       *userFixture
	appID       string
	authFlowID  string
}

func TestRecoveryReleaseTestSuite(t *testing.T) {
	suite.Run(t, new(RecoveryReleaseTestSuite))
}

func (ts *RecoveryReleaseTestSuite) SetupSuite() {
	t := ts.T()
	applyPolicy(t, longRungLockoutPolicy())

	ts.smsServer, ts.smsSenderID = newSMSSender(t, "Recovery")
	ts.users = newUserFixture(t, "recovery", testutils.UserType{Schema: credentialsSchema(
		map[string]interface{}{"mobile_number": stringAttribute, "email": stringAttribute})})

	// Every app here must carry an isolated authentication flow. The bootstrap default one CALLs the
	// default recovery flow, so an app without it contradicts its own recovery flow and is refused.
	authFlowID, err := testutils.CreateIsolatedAuthFlow("governance-recovery-isolated-auth")
	ts.Require().NoError(err, "failed to create the isolated auth flow")
	cleanupFlow(t, authFlowID)
	ts.authFlowID = authFlowID

	ts.appID = ts.newRecoveryApp("release", governanceRecoveryFlow("Governance Recovery Release Test Flow",
		"governance-recovery-release-test"))
}

func (ts *RecoveryReleaseTestSuite) SetupTest() {
	ts.smsServer.ClearMessages()
}

func (ts *RecoveryReleaseTestSuite) TestRecoveryReleasesTheLock() {
	userID, username, _, _ := ts.newUser("release")
	ts.lockCredential(userID, username)

	replacement := "Recovered-Secret-9!"
	recoverThrough(ts.T(), ts.appID, username, replacement)

	ts.True(authenticate(ts.T(), "username", username, replacement).succeeded(),
		"the replacement secret must work immediately")
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "the lock must be gone")
}

// The unlock at the end shows the write landed, so the refusal was the lock's.
func (ts *RecoveryReleaseTestSuite) TestAManagementCredentialWriteReleasesNothing() {
	userID, username, _, _ := ts.newUser("admin_write")
	expiry := ts.lockCredential(userID, username)

	replacement := "Rotated-Secret-7!"
	ts.Require().NoError(
		testutils.UpdateUserCredentials(userID, map[string]string{"password": replacement}),
		"failed to change the password")

	requireRefusedAsHeld(ts.T(), authenticate(ts.T(), "username", username, replacement), statedLockCode,
		"a management credential write must not release the lock")
	ts.requireLockUnchanged(userID, "credential", expiry, "management write")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.True(authenticate(ts.T(), "username", username, replacement).succeeded(),
		"the write must have replaced the secret")
}

// The user APIs refuse credential fields, so the self-service credential endpoint is the holder's
// write path; its token is obtained before the lock, which a lock does not revoke.
func (ts *RecoveryReleaseTestSuite) TestASelfServiceCredentialUpdateReleasesNothing() {
	userID, username, password, _ := ts.newUser("self_write")
	holder, err := testutils.GetHTTPClientForUser(username, password)
	ts.Require().NoError(err, "the holder must sign in before the lock")
	expiry := ts.lockCredential(userID, username)

	replacement := "Self-Rotated-Secret-21!"
	payload, err := json.Marshal(map[string]interface{}{
		"attributes": map[string]interface{}{"password": replacement},
	})
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/users/me/update-credentials",
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := holder.Do(req)
	ts.Require().NoError(err, "the self-service update request failed")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	ts.Require().Equal(http.StatusNoContent, resp.StatusCode,
		"a lock must not refuse the holder's own credential update: %s", string(body))

	requireRefusedAsHeld(ts.T(), authenticate(ts.T(), "username", username, replacement), statedLockCode,
		"the holder's own credential update must not release the lock")
	ts.requireLockUnchanged(userID, "credential", expiry, "holder update")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.True(authenticate(ts.T(), "username", username, replacement).succeeded(),
		"the update must have replaced the secret")
}

// Enrolment needs an assertion, which a locked password cannot produce, so it is taken before the lock
// forms. The passkey sign-in at the end shows the enrolment landed.
func (ts *RecoveryReleaseTestSuite) TestAPasskeyEnrolmentReleasesNothing() {
	userID, username, password, _ := ts.newUser("passkey_write")
	assertion, err := testutils.ObtainAuthAssertion(username, password)
	ts.Require().NoError(err, "the holder must sign in before the lock")
	expiry := ts.lockCredential(userID, username)

	authenticator, userHandle, err := testutils.RegisterPasskeyCredential(userID,
		recoveryPasskeyRPID, recoveryPasskeyRPName, recoveryPasskeyOrigin, assertion)
	ts.Require().NoError(err, "a lock must not refuse passkey enrolment")

	requireRefusedAsHeld(ts.T(), authenticate(ts.T(), "username", username, password), statedLockCode,
		"a passkey enrolment must not release the password lock")
	ts.requireLockUnchanged(userID, "credential", expiry, "passkey enrolment")

	_, err = testutils.AuthenticateWithPasskey(userID, recoveryPasskeyRPID, userHandle, authenticator)
	ts.Require().NoError(err, "the enrolled passkey must sign in")
	ts.requireLockUnchanged(userID, "credential", expiry, "passkey sign-in")
}

// A recovery that kept the history would serve the second rung on the next lockout.
func (ts *RecoveryReleaseTestSuite) TestRecoveryReleasesEveryLock() {
	userID, username, _, mobile := ts.newUser("every_lock")
	ts.lockCredential(userID, username)
	ts.lockOTP(userID, mobile)
	ts.Require().ElementsMatch([]string{"credential", "otp"}, readStatus(ts.T(), userID).lockedMethods(),
		"both authentication methods must be locked")

	recoverThrough(ts.T(), ts.appID, username, "Recovered-Secret-11!")

	status := readStatus(ts.T(), userID)
	ts.Equal("ACTIVE", status.Value, "a completed recovery must leave nothing held")
	ts.Empty(status.lockedMethods(), "a completed recovery must leave no method locked")

	ts.lockCredential(userID, username)
	ts.Equal(longFirstRungSeconds*time.Second, servedRung(ts.T(), userID, "credential"),
		"the next password lockout must serve the first rung")
	ts.lockOTP(userID, mobile)
	ts.Equal(longFirstRungSeconds*time.Second, servedRung(ts.T(), userID, "otp"),
		"the next one-time-password lockout must serve the first rung")
}

func (ts *RecoveryReleaseTestSuite) TestARecoveryChallengeReachesALockedAccountAndReleasesIt() {
	appID := ts.newRecoveryApp("otp", otpReleaseRecoveryFlow(ts.smsSenderID))

	userID, username, _, mobile := ts.newUser("otp_recovery")
	ts.lockCredential(userID, username)
	ts.lockOTP(userID, mobile)
	ts.Require().ElementsMatch([]string{"credential", "otp"}, readStatus(ts.T(), userID).lockedMethods(),
		"both authentication methods must be locked")

	step, code := ts.startCodeRecovery(appID, username, "a lock must not withhold the recovery challenge")

	step, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"otp": code}, submitOTPAction, step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the code")
	ts.Require().True(common.HasInput(step.Data.Inputs, "password"),
		"the correct code must be admitted in recovery despite the lock")

	replacement := "Recovered-By-Code-23!"
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"password": replacement}, submitPasswordAction, step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the new password")
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the recovery must complete")

	ts.Empty(readStatus(ts.T(), userID).lockedMethods(), "the release must clear both locks")
	ts.True(authenticate(ts.T(), "username", username, replacement).succeeded(),
		"the replacement secret must work immediately")
}

func (ts *RecoveryReleaseTestSuite) TestWrongRecoveryCodesDoNotLockSignIn() {
	appID := ts.newRecoveryApp("otp_uncounted",
		recoveryFlowVariantOf(otpReleaseRecoveryFlow(ts.smsSenderID), "uncounted"))
	userID, username, _, _ := ts.newUser("otp_uncounted")

	step, code := ts.startCodeRecovery(appID, username, "the recovery code must be sent")

	var err error
	for i := range lockThreshold {
		step, err = common.CompleteFlow(step.ExecutionID,
			map[string]string{"otp": wrongCodeFor(code)}, submitOTPAction, step.ChallengeToken)
		ts.Require().NoError(err, "failed to submit wrong code %d", i+1)
		ts.Require().True(common.HasInput(step.Data.Inputs, "otp"), "wrong code %d must be asked again", i+1)
	}

	status := readStatus(ts.T(), userID)
	ts.Equal("ACTIVE", status.Value, "wrong recovery codes must not lock sign-in")
	ts.Empty(status.lockedMethods(), "wrong recovery codes must lock no sign-in method")

	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"otp": code}, submitOTPAction, step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the code")
	ts.True(common.HasInput(step.Data.Inputs, "password"), "the right code must still proceed")
}

func (ts *RecoveryReleaseTestSuite) TestAWrongRecoveryCodeOnALockedAccountIsAPlainFailure() {
	appID := ts.newRecoveryApp("otp_locked_wrong",
		recoveryFlowVariantOf(otpReleaseRecoveryFlow(ts.smsSenderID), "locked_wrong"))
	passwordLockedID, passwordLockedName, _, _ := ts.newUser("wrong_code_password_locked")
	codeLockedID, codeLockedName, _, codeLockedMobile := ts.newUser("wrong_code_otp_locked")
	passwordExpiry := ts.lockCredential(passwordLockedID, passwordLockedName)
	codeExpiry := ts.lockOTP(codeLockedID, codeLockedMobile)

	for _, locked := range []struct {
		name, userID, username, method string
		expiry                         time.Time
	}{
		{"password locked", passwordLockedID, passwordLockedName, "credential", passwordExpiry},
		{"one-time password locked", codeLockedID, codeLockedName, "otp", codeExpiry},
	} {
		step, code := ts.startCodeRecovery(appID, locked.username,
			locked.name+": a lock must not withhold the recovery code")

		for i := range lockThreshold {
			var err error
			step, err = common.CompleteFlow(step.ExecutionID,
				map[string]string{"otp": wrongCodeFor(code)}, submitOTPAction, step.ChallengeToken)
			ts.Require().NoError(err, "%s: failed to submit wrong code %d", locked.name, i+1)
			ts.Require().True(common.HasInput(step.Data.Inputs, "otp"),
				"%s: wrong code %d must be asked again", locked.name, i+1)
			ts.Require().NotNil(step.Error, "%s: wrong code %d must be refused", locked.name, i+1)
			ts.Equal(recoveryWrongCodeErrorCode, step.Error.Code,
				"%s: wrong code %d must get the plain wrong-code answer, not the stated lock", locked.name, i+1)
		}

		ts.requireLockUnchanged(locked.userID, locked.method, locked.expiry, locked.name)
		ts.Equal([]string{locked.method}, readStatus(ts.T(), locked.userID).lockedMethods(),
			"%s: wrong recovery codes must not be counted toward a lock", locked.name)
	}
}

// A password checked inside a recovery flow is not a recovery code, so the lock still holds it.
func (ts *RecoveryReleaseTestSuite) TestASignInThroughARecoveryFlowStillHonoursThePasswordLock() {
	appID := ts.newSignInThroughRecoveryApp("password", passwordSignInFlow(
		"Governance Password Recovery Test Flow", "governance-password-recovery-test", "RECOVERY"))

	signIn := func(username string, passwords ...string) *common.FlowStep {
		step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
		ts.Require().NoError(err, "failed to initiate the sign-in flow")
		for _, password := range passwords {
			ts.Require().True(common.HasInput(step.Data.Inputs, "password"), "the flow must ask for the password")
			step, err = common.CompleteFlow(step.ExecutionID,
				map[string]string{"username": username, "password": password}, signInAction, step.ChallengeToken)
			ts.Require().NoError(err, "failed to submit the password")
		}
		return step
	}

	userID, username, password, _ := ts.newUser("call_locked")
	expiry := ts.lockCredential(userID, username)
	step := signIn(username, password)
	requireRepromptedWith(ts.T(), step, flowStatedLockCode)
	ts.Empty(step.Assertion, "no assertion may be issued for a locked password")
	ts.requireLockUnchanged(userID, "credential", expiry, "sign-in through recovery")

	countedID, countedName, _, _ := ts.newUser("call_counted")
	wrong := make([]string, lockThreshold)
	for i := range wrong {
		wrong[i] = fmt.Sprintf("Not-The-Password-%d!", i)
	}
	signIn(countedName, wrong...)
	ts.Equal([]string{"credential"}, readStatus(ts.T(), countedID).lockedMethods(),
		"wrong passwords in a recovery flow must still lock the password")
}

func (ts *RecoveryReleaseTestSuite) TestASignInThroughARecoveryThatReleasesCompletes() {
	appID := ts.newSignInThroughRecoveryApp("otp_release", otpReleaseRecoveryFlowForACall(ts.smsSenderID))
	userID, username, _, mobile := ts.newUser("call_otp_released")
	ts.lockOTP(userID, mobile)

	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the sign-in flow")
	step, code := ts.sendRecoveryCode(step, username, "a lock must not withhold the recovery code")
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"otp": code}, submitOTPAction, step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the code")
	ts.Require().True(common.HasInput(step.Data.Inputs, "password"), "the code must get past the lock in recovery")
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"password": "Recovered-In-Sign-In-29!"}, submitPasswordAction, step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the new password")

	ts.Equal("COMPLETE", step.FlowStatus, "a recovery that released the lock completes the sign-in")
	ts.NotEmpty(step.Assertion, "the released account must be signed in")
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "the recovery must have released the lock")
}

// The hold an unsuspend leaves holds every sign-in method, yet a passkey still proves control in
// recovery.
func (ts *RecoveryReleaseTestSuite) TestAPasskeyRecoversFromThePostSuspensionHold() {
	appID := ts.newRecoveryApp("passkey", passkeyRecoveryFlow())
	userID, username, password, _ := ts.newUser("passkey_recovery")
	assertion, err := testutils.ObtainAuthAssertion(username, password)
	ts.Require().NoError(err, "the holder must sign in before the suspension")
	authenticator, userHandle, err := testutils.RegisterPasskeyCredential(userID,
		recoveryPasskeyRPID, recoveryPasskeyRPName, recoveryPasskeyOrigin, assertion)
	ts.Require().NoError(err, "failed to enrol the passkey")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the unsuspend must leave its hold")

	step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the recovery flow")
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username}, submitUsernameAction, step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the username")
	raw, ok := step.Data.AdditionalData["passkeyChallenge"]
	ts.Require().True(ok, "precondition: the recovery must issue a passkey challenge for the account: %+v", step)
	var options struct {
		Challenge string `json:"challenge"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(raw), &options), "unreadable passkey challenge")

	credentialID, clientDataJSON, authenticatorData, signature, err :=
		authenticator.CreateAssertionResponse(options.Challenge, true)
	ts.Require().NoError(err, "failed to sign the passkey challenge")
	step, err = common.CompleteFlow(step.ExecutionID, map[string]string{
		"credentialId": credentialID, "clientDataJSON": clientDataJSON,
		"authenticatorData": authenticatorData, "signature": signature, "userHandle": userHandle,
	}, passkeyAssertionAction, step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the passkey assertion")
	ts.Require().True(common.HasInput(step.Data.Inputs, "password"),
		"the passkey must get past the post-suspension hold in recovery")

	replacement := "Recovered-By-Passkey-31!"
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"password": replacement}, submitPasswordAction, step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the new password")
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the recovery must complete")
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "the recovery must release the hold")
	ts.True(authenticate(ts.T(), "username", username, replacement).succeeded(),
		"the replacement secret must work immediately")
}

// Each shipped recovery flow runs as a copy whose email node names a sender, as an operator must name
// one, and whose link back to sign-in calls the application's own sign-in flow; the path the test
// drives is the shipped one.
func (ts *RecoveryReleaseTestSuite) TestTheShippedRecoveryFlowsReleaseTheLock() {
	smtp := testutils.NewMockSMTPServer(0)
	ts.Require().NoError(smtp.Start(), "failed to start the mock SMTP server")
	defer func() {
		if err := smtp.Stop(); err != nil {
			ts.T().Logf("cleanup: failed to stop the mock SMTP server: %v", err)
		}
	}()
	emailSenderID, err := testutils.CreateSMTPEmailProvider("Governance Shipped Recovery Provider",
		"localhost", smtp.GetPort(), "noreply@thunderid.test")
	ts.Require().NoError(err, "failed to create the email provider")
	defer func() {
		if err := testutils.DeleteNotificationSender(emailSenderID); err != nil {
			ts.T().Logf("cleanup: failed to delete the email provider: %v", err)
		}
	}()

	for _, handle := range []string{defaultFlowHandle, "console-app-flow"} {
		ts.Run(handle, func() {
			recoveryFlowID, _ := copyShippedFlowWithSender(ts.T(), handle, "RECOVERY", "send_recovery_email",
				emailSenderID, map[string]string{"call_authentication": ts.authFlowID})
			appID := ts.newApp(handle, "Governance Shipped Recovery "+handle+" Test App",
				"governance_shipped_"+strings.ReplaceAll(handle, "-", "_"),
				"Application that recovers through a copy of a shipped recovery flow", ts.authFlowID, recoveryFlowID)

			userID, username, _, _ := ts.newUser("shipped_" + strings.ReplaceAll(handle, "-", "_"))
			ts.lockCredential(userID, username)

			step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
			ts.Require().NoError(err, "failed to initiate the shipped recovery flow")
			smtp.ClearEmails()
			step, err = common.CompleteFlow(step.ExecutionID,
				map[string]string{"username": username}, submitUsernameAction, step.ChallengeToken)
			ts.Require().NoError(err, "failed to submit the username")
			ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the flow must wait for the emailed link")

			email := awaitDelivered(ts.T(), smtp.GetLastEmail, "a lock must not withhold the recovery email")
			token := testutils.ExtractQueryParam(email.ExtractRecoveryLink(), "inviteToken")
			ts.Require().NotEmpty(token, "the recovery email must carry the link's token")

			step, err = common.CompleteFlow(step.ExecutionID,
				map[string]string{"inviteToken": token}, "", step.ChallengeToken)
			ts.Require().NoError(err, "failed to submit the link's token")
			ts.Require().True(common.HasInput(step.Data.Inputs, "password"), "the link must lead to a new password")

			replacement := "Recovered-Shipped-23!"
			step, err = common.CompleteFlow(step.ExecutionID,
				map[string]string{"password": replacement}, submitPasswordAction, step.ChallengeToken)
			ts.Require().NoError(err, "failed to submit the new password")
			ts.Require().Equal("COMPLETE", step.FlowStatus, "the recovery must complete")

			ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "the shipped flow must release the lock")
			ts.True(authenticate(ts.T(), "username", username, replacement).succeeded(),
				"the replacement secret must work immediately")
		})
	}
}

func (ts *RecoveryReleaseTestSuite) TestAReleaseBeforeProofReleasesNothing() {
	appID := ts.newRecoveryApp("no_proof", recoveryFlowWithoutProof())

	userID, username, _, _ := ts.newUser("no_proof")
	expiry := ts.lockCredential(userID, username)

	step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the recovery flow")
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username}, submitUsernameAction, step.ChallengeToken)
	ts.Require().NoError(err, "the step must be answered, not fault")

	ts.Equal("ERROR", step.FlowStatus, "a release with no proof must fail the step")
	ts.Require().NotNil(step.Error, "the failed step must carry an error")
	ts.Equal(governanceRefusal.code, step.Error.Code, "the release must fail as a refused governance operation")
	ts.requireLockUnchanged(userID, "credential", expiry, "release without proof")
}

func (ts *RecoveryReleaseTestSuite) TestARecoveryReleasesOnlyTheProvenIdentity() {
	bystanderID, bystanderName, _, _ := ts.newUser("bystander")
	bystanderExpiry := ts.lockCredential(bystanderID, bystanderName)

	holderID, holderName, _, _ := ts.newUser("holder")
	ts.lockCredential(holderID, holderName)

	ts.Require().True(attemptRecovery(ts.T(), ts.appID, holderName, "Recovered-Secret-19!",
		map[string]string{subjectInput: bystanderID}), "the holder's recovery must complete")

	ts.Equal("ACTIVE", readStatus(ts.T(), holderID).Value, "the proven identity must be released")
	ts.requireLockUnchanged(bystanderID, "credential", bystanderExpiry, "bystander")
}

func (ts *RecoveryReleaseTestSuite) TestARecoveryFlowWithoutTheNodeReleasesNothing() {
	appID := ts.newRecoveryApp("no_release", recoveryFlowWithoutRelease())

	userID, username, _, _ := ts.newUser("no_release")
	expiry := ts.lockCredential(userID, username)

	recoverThrough(ts.T(), appID, username, "Recovered-Secret-13!")

	ts.requireLockUnchanged(userID, "credential", expiry, "no release node")
}

// The account is locked first so there is a real lock to release. Once the holds are lifted, the
// original secret still works and the attempted one does not.
func (ts *RecoveryReleaseTestSuite) TestASuspendedAccountCannotRecoverOutOfItsHold() {
	userID, username, password, _ := ts.newUser("suspended_recovery")
	ts.lockCredential(userID, username)

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.Require().Equal("SUSPENDED", readStatus(ts.T(), userID).Value, "the hold must be in place")

	replacement := "Recovered-Under-Hold-17!"
	ts.False(attemptRecovery(ts.T(), ts.appID, username, replacement, nil),
		"a suspended account must not be able to drive a recovery to completion")
	ts.Equal("SUSPENDED", readStatus(ts.T(), userID).Value, "the suspension must survive the attempt")

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.Require().Equal("ACTIVE", readStatus(ts.T(), userID).Value,
		"the holds must be lifted before the secrets are compared")

	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the original secret must still be the account's")
	ts.False(authenticate(ts.T(), "username", username, replacement).succeeded(),
		"the credential must not have been written")
}

// A suspension placed after the username step admitted the account is refused at the credential write:
// the flow fails with the suspension stated, and once the holds are lifted the original secret is still
// the account's.
func (ts *RecoveryReleaseTestSuite) TestASuspensionAfterTheUsernameStepStopsTheCredentialWrite() {
	userID, username, password, _ := ts.newUser("suspended_mid_recovery")

	step, err := common.InitiateRecoveryFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the recovery flow")
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username}, submitUsernameAction, step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the username")
	ts.Require().True(common.HasInput(step.Data.Inputs, "password"),
		"precondition: the username step must admit the account and ask for the new password: %+v", step)

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	replacement := "Recovered-After-Suspension-31!"
	status, raw := postFlow(ts.T(), nil, "", map[string]interface{}{
		"executionId":    step.ExecutionID,
		"inputs":         map[string]string{"password": replacement},
		"action":         submitPasswordAction,
		"challengeToken": step.ChallengeToken,
	})
	ts.Equal(flowAnswer{status: http.StatusOK, flowStatus: "ERROR", code: flowStatedSuspensionCode},
		answerOf(status, raw), "the credential write must be refused with the suspension stated: %s", raw)

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.Require().Equal("ACTIVE", readStatus(ts.T(), userID).Value,
		"the holds must be lifted before the secrets are compared")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the original secret must still be the account's")
	ts.False(authenticate(ts.T(), "username", username, replacement).succeeded(),
		"the credential must not have been written")
}

// newRecoveryApp creates a recovery flow and an application that recovers through it, both deleted when
// the test that made them ends.
func (ts *RecoveryReleaseTestSuite) newRecoveryApp(name string, flow testutils.Flow) string {
	ts.T().Helper()

	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "failed to create the %s recovery flow", name)
	cleanupFlow(ts.T(), flowID)

	return ts.newApp(name, "Governance Recovery "+name+" Test App", "governance_recovery_"+name,
		"Application the recovery release acceptance tests drive", ts.authFlowID, flowID)
}

// newSignInThroughRecoveryApp creates an application whose sign-in flow calls the given recovery flow
// and then asserts, as an operator could compose it.
func (ts *RecoveryReleaseTestSuite) newSignInThroughRecoveryApp(name string, recovery testutils.Flow) string {
	ts.T().Helper()

	recoveryFlowID, err := testutils.CreateFlow(recovery)
	ts.Require().NoError(err, "failed to create the %s recovery flow", name)
	cleanupFlow(ts.T(), recoveryFlowID)
	authFlowID, err := testutils.CreateFlow(signInCallingRecoveryFlow(name, recoveryFlowID))
	ts.Require().NoError(err, "failed to create the %s sign-in flow", name)
	cleanupFlow(ts.T(), authFlowID)

	return ts.newApp(name, "Governance Recovery Call "+name+" Test App", "governance_recovery_call_"+name,
		"Application that signs in through its recovery flow", authFlowID, recoveryFlowID)
}

// newApp creates an application of the suite's user type with recovery enabled, deleted when the test
// that made it ends.
func (ts *RecoveryReleaseTestSuite) newApp(name, displayName, clientPrefix, description, authFlowID,
	recoveryFlowID string) string {
	ts.T().Helper()

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                  ts.users.ouID,
		Name:                  displayName,
		Description:           description,
		IsRecoveryFlowEnabled: true,
		ClientID:              clientPrefix + "_client",
		ClientSecret:          clientPrefix + "_secret",
		RedirectURIs:          []string{"http://localhost:3000/callback"},
		AllowedUserTypes:      []string{ts.users.userType},
		AuthFlowID:            authFlowID,
		RecoveryFlowID:        recoveryFlowID,
	})
	ts.Require().NoError(err, "failed to create the %s application", name)
	cleanupApplication(ts.T(), appID)
	return appID
}

// newUser creates one account with a password, a mobile number and an email address nobody else in
// this package uses.
func (ts *RecoveryReleaseTestSuite) newUser(name string) (userID, username, password, mobile string) {
	ts.T().Helper()

	mobile = newMobile()
	userID, username, password = ts.users.newUser(ts.T(), name, map[string]interface{}{
		"mobile_number": mobile,
		"email":         ts.users.usernameFor(name) + "@example.com",
	})
	return userID, username, password, mobile
}

// lockCredential fails the password to the threshold and returns the expiry of the lock it forms.
func (ts *RecoveryReleaseTestSuite) lockCredential(userID, username string) time.Time {
	ts.T().Helper()

	failAuthentication(ts.T(), "username", username, lockThreshold)
	expiry, reported := readStatus(ts.T(), userID).expiryOf("credential")
	ts.Require().True(reported, "the password must be locked before a release is meaningful")
	return expiry
}

// lockOTP fails the one-time password to the threshold through the atomic endpoints, each wrong code
// against its own freshly issued session, and returns the expiry of the lock it forms.
func (ts *RecoveryReleaseTestSuite) lockOTP(userID, mobile string) time.Time {
	ts.T().Helper()

	failDirectOTP(ts.T(), ts.smsServer, ts.smsSenderID, mobile, lockThreshold)
	expiry, reported := readStatus(ts.T(), userID).expiryOf("otp")
	ts.Require().True(reported, "the one-time password must be locked before a release is meaningful")
	return expiry
}

// startCodeRecovery starts an application's code recovery flow for one account and returns the step
// asking for the code and the code the account was sent. msg says why a code must arrive.
func (ts *RecoveryReleaseTestSuite) startCodeRecovery(appID, username, msg string) (*common.FlowStep, string) {
	ts.T().Helper()

	step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the recovery flow")
	return ts.sendRecoveryCode(step, username, msg)
}

// sendRecoveryCode identifies the account at a code recovery flow's username prompt and returns the
// step asking for the code and the code the account was sent.
func (ts *RecoveryReleaseTestSuite) sendRecoveryCode(step *common.FlowStep, username, msg string) (
	*common.FlowStep, string) {
	ts.T().Helper()

	ts.smsServer.ClearMessages()
	step, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username}, submitUsernameAction, step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the username")
	ts.Require().True(common.HasInput(step.Data.Inputs, "otp"), "the flow must prompt for the code: %+v", step)
	message := awaitDelivered(ts.T(), ts.smsServer.GetLastMessage, msg)
	ts.Require().NotEmpty(message.OTP, "the recovery message must carry a code")
	return step, message.OTP
}

// requireLockUnchanged requires the method to be locked still, with the expiry it had.
func (ts *RecoveryReleaseTestSuite) requireLockUnchanged(userID, method string, expiry time.Time,
	label string) {
	ts.T().Helper()

	status := readStatus(ts.T(), userID)
	ts.Equal("LOCKED", status.Value, "%s: the lock must survive", label)
	after, reported := status.expiryOf(method)
	ts.Require().True(reported, "%s: the %s lock must still be reported", label, method)
	ts.Equal(expiry, after, "%s: the %s lock must be the one formed before", label, method)
}
