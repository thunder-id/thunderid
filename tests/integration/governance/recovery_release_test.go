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

// RecoveryReleaseTestSuite checks that a completed recovery flow releases every lock and the
// ladder with it, only for the identity it proved, and a credential write outside recovery releases
// nothing.
type RecoveryReleaseTestSuite struct {
	suite.Suite
	smsServer    *testutils.MockNotificationServer
	smsSenderID  string
	ouID         string
	entityTypeID string
	appID        string
	authFlowID   string
	flowIDs      []string
	appIDs       []string
	userIDs      []string
	suffix       string
	mobileSeq    int
}

func TestRecoveryReleaseTestSuite(t *testing.T) {
	suite.Run(t, new(RecoveryReleaseTestSuite))
}

var recoveryReleaseOU = testutils.OrganizationUnit{
	Handle:      "governance_recovery_ou",
	Name:        "Governance Recovery Release Test OU",
	Description: "Organization unit for the recovery release acceptance tests",
}

var recoveryReleaseUserType = testutils.UserType{
	Handle:      "governance_recovery_user",
	DisplayName: "governance_recovery_user",
	Schema: map[string]interface{}{
		"username":      map[string]interface{}{"type": "string"},
		"password":      map[string]interface{}{"type": "string", "credential": true},
		"mobile_number": map[string]interface{}{"type": "string"},
		"email":         map[string]interface{}{"type": "string"},
	},
}

// The rungs the recovery and federated suites lock on. Nothing there waits for a lock to lapse, so
// the first rung outlasts every test, and the second is far enough from it that the rung a lock
// serves is told apart by its expiry.
const (
	longFirstRungSeconds  = 300
	longSecondRungSeconds = 900
	rungServedTolerance   = 30 * time.Second
)

// The passkey relying party the test deployment accepts.
const (
	recoveryPasskeyRPID   = "localhost"
	recoveryPasskeyRPName = "ThunderID Test"
	recoveryPasskeyOrigin = "https://localhost:8095"
)

// longRungLockoutPolicy is the package policy with rungs that do not lapse during a test.
func longRungLockoutPolicy() map[string]interface{} {
	return map[string]interface{}{
		"user": map[string]interface{}{
			"lockGranularity": "authentication_method",
			"default": map[string]interface{}{
				"enabled":              true,
				"threshold":            lockThreshold,
				"failureWindowSeconds": failureWindowSecs,
				"lockDurationsSeconds": []int{longFirstRungSeconds, longSecondRungSeconds},
				"lockDecaySeconds":     lockDecaySeconds,
			},
		},
	}
}

func (ts *RecoveryReleaseTestSuite) SetupSuite() {
	applyPolicy(ts.T(), longRungLockoutPolicy())

	ts.smsServer = testutils.NewMockNotificationServer(0)
	ts.Require().NoError(ts.smsServer.Start(), "failed to start the mock notification server")

	senderID, err := testutils.CreateNotificationSender(testutils.NotificationSender{
		Name:        "Governance Recovery SMS Sender",
		Description: "Sender for the recovery release acceptance tests",
		Provider:    "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: ts.smsServer.GetSendSMSURL()},
			{Name: "http_method", Value: "POST"},
			{Name: "content_type", Value: "JSON"},
		},
	})
	ts.Require().NoError(err, "failed to create the notification sender")
	ts.smsSenderID = senderID

	ouID, err := testutils.CreateOrganizationUnit(recoveryReleaseOU)
	ts.Require().NoError(err, "failed to create the test organization unit")
	ts.ouID = ouID

	recoveryReleaseUserType.OUID = ts.ouID
	typeID, err := testutils.CreateUserType(recoveryReleaseUserType)
	ts.Require().NoError(err, "failed to create the test user type")
	ts.entityTypeID = typeID

	// Every app here must carry an isolated authentication flow. The bootstrap default one CALLs the
	// default recovery flow, so an app without it contradicts its own recovery flow and is refused.
	authFlowID, err := testutils.CreateIsolatedAuthFlow("governance-recovery-isolated-auth")
	ts.Require().NoError(err, "failed to create the isolated auth flow")
	ts.authFlowID = authFlowID
	ts.flowIDs = append(ts.flowIDs, authFlowID)

	ts.appID = ts.newRecoveryApp("release", buildGovernanceRecoveryFlow())

	ts.suffix = uniqueSuffix()
}

func (ts *RecoveryReleaseTestSuite) SetupTest() {
	ts.smsServer.ClearMessages()
}

func (ts *RecoveryReleaseTestSuite) TearDownSuite() {
	for _, appID := range ts.appIDs {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	}
	for _, userID := range ts.userIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("teardown: failed to delete user %s: %v", userID, err)
		}
	}
	for _, flowID := range ts.flowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("teardown: failed to delete flow %s: %v", flowID, err)
		}
	}
	if ts.smsSenderID != "" {
		if err := testutils.DeleteNotificationSender(ts.smsSenderID); err != nil {
			ts.T().Logf("teardown: failed to delete the notification sender: %v", err)
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
	if ts.smsServer != nil {
		_ = ts.smsServer.Stop()
	}
}

// newRecoveryApp creates a recovery flow and an application that recovers through it.
func (ts *RecoveryReleaseTestSuite) newRecoveryApp(name string, flow testutils.Flow) string {
	ts.T().Helper()

	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "failed to create the %s recovery flow", name)
	ts.flowIDs = append(ts.flowIDs, flowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                  ts.ouID,
		Name:                  "Governance Recovery " + name + " Test App",
		Description:           "Application the recovery release acceptance tests drive",
		IsRecoveryFlowEnabled: true,
		ClientID:              "governance_recovery_" + name + "_client",
		ClientSecret:          "governance_recovery_" + name + "_secret",
		RedirectURIs:          []string{"http://localhost:3000/callback"},
		AllowedUserTypes:      []string{recoveryReleaseUserType.Handle},
		AuthFlowID:            ts.authFlowID,
		RecoveryFlowID:        flowID,
	})
	ts.Require().NoError(err, "failed to create the %s application", name)
	ts.appIDs = append(ts.appIDs, appID)
	return appID
}

// newShippedRecoveryApp creates an application that recovers through the shipped recovery flow with
// the given handle, and signs in through the shipped authentication flow that one calls.
func (ts *RecoveryReleaseTestSuite) newShippedRecoveryApp(handle string) string {
	ts.T().Helper()

	recoveryFlowID, err := testutils.GetFlowIDByHandle(handle, "RECOVERY")
	ts.Require().NoError(err, "the %s recovery flow must ship", handle)
	authFlowID, err := testutils.GetFlowIDByHandle(handle, "AUTHENTICATION")
	ts.Require().NoError(err, "the %s authentication flow must ship", handle)

	name := strings.ReplaceAll(handle, "-", "_")
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                  ts.ouID,
		Name:                  "Governance Shipped Recovery " + handle + " Test App",
		Description:           "Application that recovers through a shipped recovery flow",
		IsRecoveryFlowEnabled: true,
		ClientID:              "governance_shipped_" + name + "_client",
		ClientSecret:          "governance_shipped_" + name + "_secret",
		RedirectURIs:          []string{"http://localhost:3000/callback"},
		AllowedUserTypes:      []string{recoveryReleaseUserType.Handle},
		AuthFlowID:            authFlowID,
		RecoveryFlowID:        recoveryFlowID,
	})
	ts.Require().NoError(err, "failed to create the %s application", handle)
	ts.appIDs = append(ts.appIDs, appID)
	return appID
}

// newUser creates one account with a password and a mobile number nobody else in this package uses.
func (ts *RecoveryReleaseTestSuite) newUser(name string) (userID, username, password, mobile string) {
	ts.T().Helper()

	ts.mobileSeq++
	username = fmt.Sprintf("recovery_%s_%s", name, ts.suffix)
	password = "Correct-Horse-" + name + "1!"
	mobile = fmt.Sprintf("+1710%07d", ts.mobileSeq)
	attributes, err := json.Marshal(map[string]interface{}{
		"username":      username,
		"password":      password,
		"mobile_number": mobile,
		"email":         username + "@example.com",
	})
	ts.Require().NoError(err)

	userID, err = testutils.CreateUser(testutils.User{
		Type:       recoveryReleaseUserType.Handle,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "failed to create the test user")
	ts.userIDs = append(ts.userIDs, userID)
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

	for i := range lockThreshold {
		sessionToken, code, status := sendDirectOTP(ts.T(), ts.smsServer, ts.smsSenderID, mobile)
		ts.Require().Equal(http.StatusOK, status, "the code must be dispatched on attempt %d", i+1)
		ts.Require().NotEmpty(code, "attempt %d dispatched no code", i+1)
		ts.Require().False(verifyDirectOTP(ts.T(), sessionToken, recoveryWrongCode(code)).succeeded(),
			"the wrong code must be refused on attempt %d", i+1)
	}
	expiry, reported := readStatus(ts.T(), userID).expiryOf("otp")
	ts.Require().True(reported, "the one-time password must be locked before a release is meaningful")
	return expiry
}

// recoveryWrongCode returns a code of the same shape that differs from it in every digit.
func recoveryWrongCode(code string) string {
	wrong := []rune(code)
	for i, r := range wrong {
		if r == '0' {
			wrong[i] = '1'
		} else {
			wrong[i] = '0'
		}
	}
	return string(wrong)
}

// recover drives this suite's recovery flow to completion for one account, setting newPassword.
func (ts *RecoveryReleaseTestSuite) recover(username, newPassword string) {
	ts.T().Helper()
	ts.recoverThrough(ts.appID, username, newPassword)
}

// recoverThrough is the same against a named application, for the cases that need a different flow.
func (ts *RecoveryReleaseTestSuite) recoverThrough(appID, username, newPassword string) {
	ts.T().Helper()
	ts.Require().True(ts.attemptRecoveryThrough(appID, username, newPassword, nil),
		"the recovery flow must run to completion")
}

// attemptRecoveryThrough drives the recovery flow and reports whether it completed. extra is sent
// with every step, for a request naming inputs the flow did not ask for. A held subject is refused at
// identify like an unknown one, so the flow asks for the username again.
func (ts *RecoveryReleaseTestSuite) attemptRecoveryThrough(appID, username, newPassword string,
	extra map[string]string) bool {
	ts.T().Helper()

	with := func(inputs map[string]string) map[string]string {
		for key, value := range extra {
			inputs[key] = value
		}
		return inputs
	}

	step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the recovery flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus)

	step, err = common.CompleteFlow(step.ExecutionID,
		with(map[string]string{"username": username}), "action_submit_username", step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the username")
	if step.FlowStatus != "INCOMPLETE" || !common.HasInput(step.Data.Inputs, "password") {
		return false
	}

	step, err = common.CompleteFlow(step.ExecutionID,
		with(map[string]string{"password": newPassword}), "action_submit_password", step.ChallengeToken)
	if err != nil {
		return false
	}
	return step.FlowStatus == "COMPLETE"
}

// requireLockUnchanged requires the scope to be locked still, with the expiry it had.
func (ts *RecoveryReleaseTestSuite) requireLockUnchanged(userID, scope string, expiry time.Time,
	label string) {
	ts.T().Helper()

	status := readStatus(ts.T(), userID)
	ts.Equal("LOCKED", status.Value, "%s: the lock must survive", label)
	after, reported := status.expiryOf(scope)
	ts.Require().True(reported, "%s: the %s lock must still be reported", label, scope)
	ts.Equal(expiry, after, "%s: the %s lock must be the one formed before", label, scope)
}

// A completed recovery releases the lock, so the user can sign in at once.
// Recovery is the user's only exit from the ladder.
func (ts *RecoveryReleaseTestSuite) TestRecoveryReleasesTheLock() {
	userID, username, _, _ := ts.newUser("release")
	ts.lockCredential(userID, username)

	replacement := "Recovered-Secret-9!"
	ts.recover(username, replacement)

	ts.True(authenticate(ts.T(), "username", username, replacement).succeeded(),
		"the replacement secret must work immediately")
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "the lock must be gone")
}

// A management credential write releases nothing. Replacing a secret is not proof of
// control by the account holder: an operator did it, and the ladder is not theirs to step off. The
// unlock at the end shows the write itself landed, so the refusal was the lock's.
func (ts *RecoveryReleaseTestSuite) TestAManagementCredentialWriteReleasesNothing() {
	userID, username, _, _ := ts.newUser("admin_write")
	expiry := ts.lockCredential(userID, username)

	replacement := "Rotated-Secret-7!"
	ts.Require().NoError(
		testutils.UpdateUserCredentials(userID, map[string]string{"password": replacement}),
		"failed to change the password")

	ts.False(authenticate(ts.T(), "username", username, replacement).succeeded(),
		"a management credential write must not release the lock")
	ts.requireLockUnchanged(userID, "credential", expiry, "management write")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.True(authenticate(ts.T(), "username", username, replacement).succeeded(),
		"the write must have replaced the secret")
}

// The holder's own credential update outside recovery is a write too, and releases nothing.
// A profile update cannot carry a password at all (the user APIs refuse credential fields), so the
// self-service credential endpoint is the holder's write path. Its token is obtained before the lock,
// which a lock does not revoke.
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

	ts.False(authenticate(ts.T(), "username", username, replacement).succeeded(),
		"the holder's own credential update must not release the lock")
	ts.requireLockUnchanged(userID, "credential", expiry, "holder update")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.True(authenticate(ts.T(), "username", username, replacement).succeeded(),
		"the update must have replaced the secret")
}

// A passkey enrolment is a credential write by the holder, and releases nothing. Enrolment
// needs an assertion, which a locked password cannot produce, so it is taken before the lock forms.
// The passkey sign-in at the end shows the enrolment landed, and it does not release the password
// either, being a different authentication method.
func (ts *RecoveryReleaseTestSuite) TestAPasskeyEnrolmentReleasesNothing() {
	userID, username, password, _ := ts.newUser("passkey_write")
	assertion, err := testutils.ObtainAuthAssertion(username, password)
	ts.Require().NoError(err, "the holder must sign in before the lock")
	expiry := ts.lockCredential(userID, username)

	authenticator, userHandle, err := testutils.RegisterPasskeyCredential(userID,
		recoveryPasskeyRPID, recoveryPasskeyRPName, recoveryPasskeyOrigin, assertion)
	ts.Require().NoError(err, "a lock must not refuse passkey enrolment")

	ts.False(authenticate(ts.T(), "username", username, password).succeeded(),
		"a passkey enrolment must not release the password lock")
	ts.requireLockUnchanged(userID, "credential", expiry, "passkey enrolment")

	_, err = testutils.AuthenticateWithPasskey(userID, recoveryPasskeyRPID, userHandle, authenticator)
	ts.Require().NoError(err, "the enrolled passkey must sign in")
	ts.requireLockUnchanged(userID, "credential", expiry, "passkey sign-in")
}

// A recovery releases every lock and its history, not only the one whose secret was
// replaced: it proves control of the account, not of one authentication method. A recovery that kept
// the history would serve the second rung on the next lockout.
func (ts *RecoveryReleaseTestSuite) TestRecoveryReleasesEveryLock() {
	userID, username, _, mobile := ts.newUser("every_lock")
	ts.lockCredential(userID, username)
	ts.lockOTP(userID, mobile)
	ts.Require().ElementsMatch([]string{"credential", "otp"}, readStatus(ts.T(), userID).lockedScopes(),
		"both authentication methods must be locked")

	ts.recover(username, "Recovered-Secret-11!")

	status := readStatus(ts.T(), userID)
	ts.Equal("ACTIVE", status.Value, "a completed recovery must leave nothing held")
	ts.Empty(status.lockedScopes(), "a completed recovery must leave no scope locked")

	ts.WithinDuration(time.Now().Add(longFirstRungSeconds*time.Second), ts.lockCredential(userID, username),
		rungServedTolerance, "the next password lockout must serve the first rung")
	ts.WithinDuration(time.Now().Add(longFirstRungSeconds*time.Second), ts.lockOTP(userID, mobile),
		rungServedTolerance, "the next one-time-password lockout must serve the first rung")
}

// With the release: a recovery flow proven by a code still sends one to an account whose
// one-time password is locked, and admits the correct code at the recovery plane. The release then
// clears both locks.
func (ts *RecoveryReleaseTestSuite) TestARecoveryChallengeReachesALockedAccountAndReleasesIt() {
	appID := ts.newRecoveryApp("otp", buildOTPReleaseRecoveryFlow(ts.smsSenderID))

	userID, username, _, mobile := ts.newUser("otp_recovery")
	ts.lockCredential(userID, username)
	ts.lockOTP(userID, mobile)
	ts.Require().ElementsMatch([]string{"credential", "otp"}, readStatus(ts.T(), userID).lockedScopes(),
		"both authentication methods must be locked")

	step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the recovery flow")
	ts.smsServer.ClearMessages()
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username}, "action_submit_username", step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the username")
	ts.Require().True(common.HasInput(step.Data.Inputs, "otp"), "the flow must prompt for the code")

	message := awaitSMS(ts.smsServer)
	ts.Require().NotNil(message, "a lock must not withhold the recovery challenge")
	code := message.OTP
	ts.Require().NotEmpty(code, "the recovery challenge must carry a code")

	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"otp": code}, "action_submit_otp", step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the code")
	ts.Require().True(common.HasInput(step.Data.Inputs, "password"),
		"the correct code must be admitted at the recovery plane despite the lock")

	replacement := "Recovered-By-Code-23!"
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"password": replacement}, "action_submit_password", step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the new password")
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the recovery must complete")

	ts.Empty(readStatus(ts.T(), userID).lockedScopes(), "the release must clear both locks")
	ts.True(authenticate(ts.T(), "username", username, replacement).succeeded(),
		"the replacement secret must work immediately")
}

// The recovery flows the server ships release the lock too, not only this package's fixtures. The
// application recovers through the shipped flow itself; its email node is given a sender, as an
// operator must, and is otherwise run as shipped: the link's token, a new password, then the release.
func (ts *RecoveryReleaseTestSuite) TestTheShippedRecoveryFlowsReleaseTheLock() {
	smtp := testutils.NewMockSMTPServer(0)
	ts.Require().NoError(smtp.Start(), "failed to start the mock SMTP server")
	defer func() { _ = smtp.Stop() }()
	emailSenderID, err := testutils.CreateSMTPEmailProvider("Governance Shipped Recovery Provider",
		"localhost", smtp.GetPort(), "noreply@thunderid.test")
	ts.Require().NoError(err, "failed to create the email provider")
	defer func() {
		if err := testutils.DeleteNotificationSender(emailSenderID); err != nil {
			ts.T().Logf("cleanup: failed to delete the email provider: %v", err)
		}
	}()

	for _, handle := range []string{"default-flow", "console-app-flow"} {
		ts.Run(handle, func() {
			defer nameFlowSender(ts.T(), handle, "RECOVERY", "send_recovery_email", emailSenderID)()
			appID := ts.newShippedRecoveryApp(handle)

			userID, username, _, _ := ts.newUser("shipped_" + strings.ReplaceAll(handle, "-", "_"))
			ts.lockCredential(userID, username)

			step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
			ts.Require().NoError(err, "failed to initiate the shipped recovery flow")
			smtp.ClearEmails()
			step, err = common.CompleteFlow(step.ExecutionID,
				map[string]string{"username": username}, "action_submit_username", step.ChallengeToken)
			ts.Require().NoError(err, "failed to submit the username")
			ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the flow must wait for the emailed link")

			email := awaitEmail(smtp, 5*time.Second)
			ts.Require().NotNil(email, "a lock must not withhold the recovery email")
			token := testutils.ExtractQueryParam(email.ExtractRecoveryLink(), "inviteToken")
			ts.Require().NotEmpty(token, "the recovery email must carry the link's token")

			step, err = common.CompleteFlow(step.ExecutionID,
				map[string]string{"inviteToken": token}, "", step.ChallengeToken)
			ts.Require().NoError(err, "failed to submit the link's token")
			ts.Require().True(common.HasInput(step.Data.Inputs, "password"), "the link must lead to a new password")

			replacement := "Recovered-Shipped-23!"
			step, err = common.CompleteFlow(step.ExecutionID,
				map[string]string{"password": replacement}, "action_submit_password", step.ChallengeToken)
			ts.Require().NoError(err, "failed to submit the new password")
			ts.Require().Equal("COMPLETE", step.FlowStatus, "the recovery must complete")

			ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "the shipped flow must release the lock")
			ts.True(authenticate(ts.T(), "username", username, replacement).succeeded(),
				"the replacement secret must work immediately")
		})
	}
}

// A release node reached before anything recorded proof of control fails, and clears
// nothing: naming an account is not proving it.
func (ts *RecoveryReleaseTestSuite) TestAReleaseBeforeProofReleasesNothing() {
	appID := ts.newRecoveryApp("no_proof", buildRecoveryFlowWithoutProof())

	userID, username, _, _ := ts.newUser("no_proof")
	expiry := ts.lockCredential(userID, username)

	step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the recovery flow")
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username}, "action_submit_username", step.ChallengeToken)
	ts.Require().NoError(err, "the step must be answered, not fault")

	ts.Equal("ERROR", step.FlowStatus, "a release with no proof must fail the step")
	ts.Require().NotNil(step.Error, "the failed step must carry an error")
	ts.Equal("FET-1112", step.Error.Code, "the release must fail as a refused governance operation")
	ts.requireLockUnchanged(userID, "credential", expiry, "release without proof")
}

// A subject named in the request is ignored: only the identity this run proved is
// released, and a locked bystander stays locked.
func (ts *RecoveryReleaseTestSuite) TestARecoveryReleasesOnlyTheProvenIdentity() {
	bystanderID, bystanderName, _, _ := ts.newUser("bystander")
	bystanderExpiry := ts.lockCredential(bystanderID, bystanderName)

	holderID, holderName, _, _ := ts.newUser("holder")
	ts.lockCredential(holderID, holderName)

	ts.Require().True(ts.attemptRecoveryThrough(ts.appID, holderName, "Recovered-Secret-19!",
		map[string]string{subjectInput: bystanderID}), "the holder's recovery must complete")

	ts.Equal("ACTIVE", readStatus(ts.T(), holderID).Value, "the proven identity must be released")
	ts.requireLockUnchanged(bystanderID, "credential", bystanderExpiry, "bystander")
}

// A recovery flow with no release node releases nothing; the node is what releases the ladder.
func (ts *RecoveryReleaseTestSuite) TestARecoveryFlowWithoutTheNodeReleasesNothing() {
	appID := ts.newRecoveryApp("no_release", buildRecoveryFlowWithoutRelease())

	userID, username, _, _ := ts.newUser("no_release")
	expiry := ts.lockCredential(userID, username)

	ts.recoverThrough(appID, username, "Recovered-Secret-13!")

	ts.requireLockUnchanged(userID, "credential", expiry, "no release node")
}

// A completed recovery does not release a suspension: the suspension refuses the recovery
// plane before a credential is written. The account is locked first so there is a real lock to
// release. Once the holds are lifted, the original secret still works and the attempted one does not.
func (ts *RecoveryReleaseTestSuite) TestASuspendedAccountCannotRecoverOutOfItsHold() {
	userID, username, password, _ := ts.newUser("suspended_recovery")
	ts.lockCredential(userID, username)

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.Require().Equal("SUSPENDED", readStatus(ts.T(), userID).Value, "the hold must be in place")

	replacement := "Recovered-Under-Hold-17!"
	ts.False(ts.attemptRecoveryThrough(ts.appID, username, replacement, nil),
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

// buildGovernanceRecoveryFlow is the smallest RECOVERY flow that writes a credential and then
// releases the ladder: identify, set, unlock. The unlock is its own node, as in the shipped flows.
// It has no challenge; buildOTPReleaseRecoveryFlow is the code-proven one.
func buildGovernanceRecoveryFlow() testutils.Flow {
	return testutils.Flow{
		Name:     "Governance Recovery Release Test Flow",
		Handle:   "governance-recovery-release-test",
		FlowType: "RECOVERY",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_username",
			},
			{
				"id":   "prompt_username",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_username",
								"identifier": "username",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_submit_username",
							"nextNode": "identify_user",
						},
					},
				},
			},
			{
				"id":   "identify_user",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "IdentifyingExecutor",
					"mode": "identify",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_username",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
					},
				},
				"onSuccess": "prompt_new_password",
			},
			{
				"id":   "prompt_new_password",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_new_password",
								"identifier": "password",
								"type":       "PASSWORD_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_submit_password",
							"nextNode": "set_credential",
						},
					},
				},
			},
			{
				"id":   "set_credential",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialSetter",
				},
				"onSuccess": "release_account_locks",
			},
			{
				"id":   "release_account_locks",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "IdentityGovernanceExecutor",
					"mode": "unlock",
				},
				"onSuccess": "recovery_complete",
			},
			{
				"id":   "recovery_complete",
				"type": "PROMPT",
				"next": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}
}

// recoveryFlowVariant copies the fixture above under a new name and handle, applying edit to each
// node; edit returns false to drop the node.
func recoveryFlowVariant(name, handle string, edit func(node map[string]interface{}) bool) testutils.Flow {
	flow := buildGovernanceRecoveryFlow()
	flow.Name = name
	flow.Handle = handle

	original, _ := flow.Nodes.([]map[string]interface{})
	nodes := make([]map[string]interface{}, 0, len(original))
	for _, node := range original {
		if edit(node) {
			nodes = append(nodes, node)
		}
	}
	flow.Nodes = nodes
	return flow
}

// buildRecoveryFlowWithoutRelease is the fixture with the release node taken out.
func buildRecoveryFlowWithoutRelease() testutils.Flow {
	return recoveryFlowVariant("Governance Recovery Without Release Test Flow",
		"governance-recovery-no-release-test", func(node map[string]interface{}) bool {
			switch node["id"] {
			case "release_account_locks":
				return false
			case "set_credential":
				node["onSuccess"] = "recovery_complete"
			}
			return true
		})
}

// buildRecoveryFlowWithoutProof is the fixture with nothing between identify and the release, so the
// release runs before anything proved control.
func buildRecoveryFlowWithoutProof() testutils.Flow {
	return recoveryFlowVariant("Governance Recovery Without Proof Test Flow",
		"governance-recovery-no-proof-test", func(node map[string]interface{}) bool {
			switch node["id"] {
			case "prompt_new_password", "set_credential":
				return false
			case "identify_user":
				node["onSuccess"] = "release_account_locks"
			}
			return true
		})
}

// buildOTPReleaseRecoveryFlow is a recovery flow proven by an SMS code: identify, send, verify, set,
// release.
func buildOTPReleaseRecoveryFlow(senderID string) testutils.Flow {
	usernameInput := map[string]interface{}{
		"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true,
	}
	otpInput := map[string]interface{}{
		"ref": "input_otp", "identifier": "otp", "type": "OTP_INPUT", "required": true,
	}
	passwordInput := map[string]interface{}{
		"ref": "input_new_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true,
	}
	return testutils.Flow{
		Name:     "Governance OTP Recovery Release Test Flow",
		Handle:   "governance-otp-recovery-release-test",
		FlowType: "RECOVERY",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_username"},
			{
				"id":   "prompt_username",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{{
					"inputs": []map[string]interface{}{usernameInput},
					"action": map[string]interface{}{"ref": "action_submit_username", "nextNode": "identify_user"},
				}},
			},
			{
				"id":        "identify_user",
				"type":      "TASK_EXECUTION",
				"inputs":    []map[string]interface{}{usernameInput},
				"executor":  map[string]interface{}{"name": "IdentifyingExecutor", "mode": "identify"},
				"onSuccess": "generate_otp",
			},
			{
				"id":   "generate_otp",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name":   "OTPExecutor",
					"mode":   "generate",
					"inputs": []map[string]interface{}{usernameInput},
				},
				"onSuccess": "sms_send",
			},
			{
				"id":         "sms_send",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"senderId": senderID, "smsTemplate": "otp"},
				"executor":   map[string]interface{}{"name": "SMSExecutor"},
				"onSuccess":  "prompt_otp",
			},
			{
				"id":   "prompt_otp",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{{
					"inputs": []map[string]interface{}{otpInput},
					"action": map[string]interface{}{"ref": "action_submit_otp", "nextNode": "verify_otp"},
				}},
			},
			{
				"id":           "verify_otp",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "OTPExecutor", "mode": "verify"},
				"onSuccess":    "prompt_new_password",
				"onIncomplete": "prompt_otp",
			},
			{
				"id":   "prompt_new_password",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{{
					"inputs": []map[string]interface{}{passwordInput},
					"action": map[string]interface{}{"ref": "action_submit_password", "nextNode": "set_credential"},
				}},
			},
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
