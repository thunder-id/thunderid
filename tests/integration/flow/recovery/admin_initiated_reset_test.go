// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package recovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	adminInitiatedFlowType  = "ADMIN_INITIATED"
	adminResetFlowHandle    = "default-admin-credential-reset-flow"
	adminResetSendNode      = "send_link"
	adminResetSubmitAction  = "action_prompt_new_credential"
	adminResetSuspendHandle = "default-user-suspend-flow"
	adminResetRedirectURI   = "https://localhost:3000/admin-reset"
	adminResetClientSecret  = "admin-reset-client-secret"

	// The flow's answers: an expired or superseded reset, a stated suspension, and a missing email.
	adminResetCodeVoided           = "FET-1073"
	adminResetCodeSuspended        = "FET-1114"
	adminResetCodeNoEmail          = "FET-1037"
	adminResetCodeExtraInput       = "FET-1141"
	adminResetCodeUnsupportedInput = "FET-1046"
	adminResetCodeNoAuthn          = "FES-1017"
	adminResetCodeNoPermission     = "FES-1019"

	// adminResetDeliveryWait bounds the wait for an email the server is expected to send.
	adminResetDeliveryWait = 5 * time.Second
	// adminResetLockThreshold is the user lock threshold the server ships with.
	adminResetLockThreshold = 5
)

// AdminInitiatedResetTestSuite drives the shipped administrator-initiated credential reset flow: the
// administrator starts it for a user, the link is emailed to the user's stored address, and the user completes
// it from the link without signing in.
type AdminInitiatedResetTestSuite struct {
	suite.Suite
	suffix     string
	ouID       string
	userType   string
	smtp       *testutils.MockSMTPServer
	senderID   string
	flowHandle string
	clientID   string
	appID      string
}

// resetUser is an account a test created, with what it signs in with.
type resetUser struct {
	id        string
	username  string
	password  string
	pin       string
	email     string
	workEmail string
}

func TestAdminInitiatedResetSuite(t *testing.T) {
	suite.Run(t, new(AdminInitiatedResetTestSuite))
}

func (ts *AdminInitiatedResetTestSuite) SetupSuite() {
	t := ts.T()
	ts.suffix = fmt.Sprintf("%d", time.Now().UnixNano())

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "admin-reset-ou-" + ts.suffix, Name: "Admin Reset OU " + ts.suffix,
		Description: "Organization unit for the administrator-initiated reset tests",
	})
	ts.Require().NoError(err, "failed to create the organization unit")
	ts.ouID = ouID
	t.Cleanup(func() {
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			t.Logf("cleanup: failed to delete the organization unit: %v", err)
		}
	})

	ts.userType = "admin_reset_user_" + ts.suffix
	typeID, err := testutils.CreateUserType(testutils.UserType{
		Handle: ts.userType, DisplayName: "Admin Reset User", OUID: ouID,
		Schema: map[string]interface{}{
			"username":    map[string]interface{}{"type": "string", "required": true, "unique": true},
			"password":    map[string]interface{}{"type": "string", "credential": true, "regex": "^.{8,}$"},
			"accessCode":  map[string]interface{}{"type": "string", "credential": true, "displayName": "Access code"},
			"numericCode": map[string]interface{}{"type": "number", "credential": true, "displayName": "Numeric code"},
			"pin":         map[string]interface{}{"type": "string", "credential": true},
			"email":       map[string]interface{}{"type": "email"},
			"workEmail":   map[string]interface{}{"type": "email"},
		},
	})
	ts.Require().NoError(err, "failed to create the user type")
	t.Cleanup(func() {
		if err := testutils.DeleteUserType(typeID); err != nil {
			t.Logf("cleanup: failed to delete the user type: %v", err)
		}
	})

	ts.clientID = "admin_reset_" + ts.suffix
	ts.appID, err = testutils.CreateApplication(testutils.Application{
		OUID: ouID, Name: "Admin Reset App " + ts.suffix, AllowedUserTypes: []string{ts.userType},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId": ts.clientID, "clientSecret": adminResetClientSecret,
				"redirectUris":            []string{adminResetRedirectURI},
				"grantTypes":              []string{"authorization_code", "refresh_token"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_post",
				"scopes":                  []string{"openid"},
			},
		}},
	})
	ts.Require().NoError(err, "failed to create the sign-in application")
	t.Cleanup(func() {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			t.Logf("cleanup: failed to delete the sign-in application: %v", err)
		}
	})

	ts.smtp = testutils.NewMockSMTPServer(0)
	ts.Require().NoError(ts.smtp.Start(), "failed to start the mock SMTP server")
	t.Cleanup(func() { _ = ts.smtp.Stop() })
	ts.senderID, err = testutils.CreateSMTPEmailProvider("Admin Reset Provider "+ts.suffix, "localhost",
		ts.smtp.GetPort(), "noreply@thunderid.test")
	ts.Require().NoError(err, "failed to create the SMTP email provider")
	t.Cleanup(func() {
		if err := testutils.DeleteNotificationSender(ts.senderID); err != nil {
			t.Logf("cleanup: failed to delete the sender: %v", err)
		}
	})

	ts.flowHandle = ts.copyFlow("admin-reset-"+ts.suffix, "", ts.senderID)
}

func (ts *AdminInitiatedResetTestSuite) SetupTest() {
	ts.smtp.ClearEmails()
}

// The old password stops working once the link is sent, the administrator never sees the link, the user
// sets a new password from the link without signing in, and the used link cannot set another.
func (ts *AdminInitiatedResetTestSuite) TestTheUserResetsThePasswordFromTheEmailedLink() {
	user := ts.newUser("happy", true)

	started, link := ts.sendLink(user, "password")

	inviteToken := testutils.ExtractQueryParam(link, "inviteToken")
	ts.Require().NotEmpty(inviteToken, "the link must carry its token")
	ts.NotContains(string(started.Body), inviteToken, "the administrator must not get the link token")
	ts.NotContains(string(started.Body), "inviteLink", "the administrator must not get the link")
	ts.Empty(started.Step.Data.Inputs, "the administrator is asked for nothing once the link is sent")
	ts.Equal("true", started.Step.Data.AdditionalData["emailSent"], "the administrator must be told the email was sent")
	ts.NotContains(started.Step.Data.AdditionalData, "inviteLink", "the administrator must not get the link")
	ts.False(ts.authenticates(user.username, "password", user.password),
		"the old password must stop working once the link is sent")
	ts.True(ts.authenticates(user.username, "pin", user.pin), "the reset must leave the PIN working")

	prompt := ts.openLink(link)
	ts.Require().True(common.HasInput(prompt.Data.Inputs, "password"), "the user must be asked for a password")
	ts.False(common.HasInput(prompt.Data.Inputs, "pin"), "the user must not be asked for a PIN")
	done := ts.submit(prompt, adminResetSubmitAction, map[string]string{"password": "Brand-New-Pass-1!"})

	ts.Require().NotNil(done.Step, "the new password must be answered in the flow: %s", done.Body)
	ts.Equal("COMPLETE", done.Step.FlowStatus, "setting the new password must complete the reset: %s", done.Body)
	ts.True(ts.authenticates(user.username, "password", "Brand-New-Pass-1!"), "the new password must work")
	ts.False(ts.authenticates(user.username, "password", user.password), "the old password must stay blocked")

	reused := ts.continueAsUser(map[string]interface{}{"executionId": started.Step.ExecutionID,
		"inputs": map[string]string{"inviteToken": inviteToken}})
	ts.False(reused.Step != nil && reused.Step.FlowStatus != "ERROR",
		"a used link must be refused: %s", reused.Body)
	ts.NotContains(string(reused.Body), user.username, "a refused link must not name the user")
	ts.True(ts.authenticates(user.username, "password", "Brand-New-Pass-1!"),
		"a used link must not change the new password")
}

// With more than one credential declared, the administrator is asked which one to reset, and nothing is
// sent before the choice.
func (ts *AdminInitiatedResetTestSuite) TestTheAdministratorChoosesTheCredential() {
	user := ts.newUser("choose", true)

	asked := ts.start(ts.adminToken(), ts.flowHandle, map[string]string{"subject": user.id})

	ts.Require().NotNil(asked.Step, "the start must be answered in the flow: %s", asked.Body)
	ts.Require().Equal("INCOMPLETE", asked.Step.FlowStatus, "the flow must wait for the choice: %s", asked.Body)
	ts.True(common.HasInput(asked.Step.Data.Inputs, "credential"), "the administrator must be asked: %s",
		asked.Body)
	ts.requireNoEmail("nothing may be sent before the credential is chosen")
	ts.True(ts.authenticates(user.username, "password", user.password), "nothing may change before the choice")

	chosen := ts.continueAs(ts.adminToken(), map[string]interface{}{
		"executionId": asked.Step.ExecutionID, "challengeToken": asked.Step.ChallengeToken,
		"inputs": map[string]string{"credential": "pin"},
	})

	ts.Require().NotNil(chosen.Step, "the choice must be answered in the flow: %s", chosen.Body)
	ts.Equal("INCOMPLETE", chosen.Step.FlowStatus, "the flow must wait once the link is sent: %s", chosen.Body)
	ts.awaitLink(user)
	ts.False(ts.authenticates(user.username, "pin", user.pin), "the chosen credential must be blocked")
	ts.True(ts.authenticates(user.username, "password", user.password), "the other credential must still work")
}

// A PIN reset asks the user for a PIN, sets it, and leaves the password as it was.
func (ts *AdminInitiatedResetTestSuite) TestAPINResetSetsANewPINAndKeepsThePassword() {
	user := ts.newUser("pin", true)

	_, link := ts.sendLink(user, "pin")

	ts.False(ts.authenticates(user.username, "pin", user.pin), "the old PIN must stop working once the link is sent")
	ts.True(ts.authenticates(user.username, "password", user.password), "the password must keep working")
	prompt := ts.openLink(link)
	ts.Require().True(common.HasInput(prompt.Data.Inputs, "pin"), "the user must be asked for a PIN: %+v", prompt)
	ts.False(common.HasInput(prompt.Data.Inputs, "password"), "the user must not be asked for a password")
	done := ts.submit(prompt, adminResetSubmitAction, map[string]string{"pin": "864213", "password": "Sneaked-In-1!"})

	ts.Require().NotNil(done.Step, "the new PIN must be answered in the flow: %s", done.Body)
	ts.Equal("COMPLETE", done.Step.FlowStatus, "setting the new PIN must complete the reset: %s", done.Body)
	ts.True(ts.authenticates(user.username, "pin", "864213"), "the new PIN must work")
	ts.True(ts.authenticates(user.username, "password", user.password), "the password must be untouched")
	ts.False(ts.authenticates(user.username, "password", "Sneaked-In-1!"),
		"a password sent with the PIN must not be set")
}

// A start that carries any input besides the user and the credential is refused before anything is sent, so the
// administrator cannot supply the address the email step reads, even one the email step names outside the
// profile.
func (ts *AdminInitiatedResetTestSuite) TestAStartCarryingAnotherInputIsRefused() {
	recipientHandle := ts.copyFlowSendingTo("admin-reset-recipient-"+ts.suffix, "recipient")
	for _, test := range []struct{ input, handle string }{
		{input: "email", handle: ts.flowHandle},
		{input: "workEmail", handle: ts.flowHandle},
		{input: "recipient", handle: recipientHandle},
	} {
		ts.Run(test.input, func() {
			ts.smtp.ClearEmails()
			user := ts.newUser("recipient_"+test.input, true)
			result := ts.start(ts.adminToken(), test.handle, map[string]string{
				"subject": user.id, "credential": "password", test.input: "another-recipient@example.com",
			})

			ts.requireRefused(result, adminResetCodeExtraInput, "another input in the start must be refused")
			ts.requireNoEmail("nothing may be sent when the start carries another input")
			ts.True(ts.authenticates(user.username, "password", user.password), "the password must keep working")
		})
	}
}

// The flow's email step names the profile attribute the link is sent to.
func (ts *AdminInitiatedResetTestSuite) TestTheFlowChoosesTheRecipientAttribute() {
	handle := ts.copyFlowSendingTo("admin-reset-work-"+ts.suffix, "workEmail")
	user := ts.newUser("work_recipient", true)

	result := ts.start(ts.adminToken(), handle, map[string]string{"subject": user.id, "credential": "password"})

	ts.Require().NotNil(result.Step, string(result.Body))
	ts.Require().Equal("INCOMPLETE", result.Step.FlowStatus, string(result.Body))
	link := ts.awaitLinkAt(user.workEmail)
	done := ts.submit(ts.openLink(link), adminResetSubmitAction, map[string]string{"password": "Work-Pass-1!"})
	ts.Require().NotNil(done.Step, string(done.Body))
	ts.Equal("COMPLETE", done.Step.FlowStatus, string(done.Body))
	ts.True(ts.authenticates(user.username, "password", "Work-Pass-1!"))
}

func (ts *AdminInitiatedResetTestSuite) TestUnsupportedCredentialsAreRefusedBeforeDelivery() {
	for _, credential := range []string{"undeclared", "passkey"} {
		ts.Run(credential, func() {
			user := ts.newUser("unsupported_"+credential, true)
			result := ts.start(ts.adminToken(), ts.flowHandle,
				map[string]string{"subject": user.id, "credential": credential})

			ts.requireRefused(result, "USR-1024", "an unsupported credential must be refused")
			ts.requireNoEmail("a refused credential must not send an email")
			ts.True(ts.authenticates(user.username, "password", user.password), "the password must remain usable")
		})
	}
}

func (ts *AdminInitiatedResetTestSuite) TestAnInvalidValueCanBeCorrected() {
	user := ts.newUser("policy", true)
	_, link := ts.sendLink(user, "password")
	prompt := ts.openLink(link)

	invalid := ts.submit(prompt, adminResetSubmitAction, map[string]string{"password": "short"})

	ts.Require().NotNil(invalid.Step, string(invalid.Body))
	ts.Require().Equal("INCOMPLETE", invalid.Step.FlowStatus, string(invalid.Body))
	ts.True(common.HasInput(invalid.Step.Data.Inputs, "password"), "the rejected credential must be requested again")
	ts.False(ts.authenticates(user.username, "password", "short"), "the rejected value must not be stored")
	done := ts.submit(invalid.Step, adminResetSubmitAction, map[string]string{"password": "Policy-Pass-1!"})
	ts.Require().NotNil(done.Step, string(done.Body))
	ts.Equal("COMPLETE", done.Step.FlowStatus, string(done.Body))
	ts.True(ts.authenticates(user.username, "password", "Policy-Pass-1!"))
}

func (ts *AdminInitiatedResetTestSuite) TestCustomCredentialNamesUseTheirOwnInput() {
	for _, test := range []struct {
		name, oldValue, newValue, label string
	}{
		{name: "accessCode", oldValue: "Old-Access-1!", newValue: "New-Access-1!", label: "Access code"},
		{name: "pin", oldValue: "135790", newValue: "001234", label: "pin"},
	} {
		ts.Run(test.name, func() {
			ts.smtp.ClearEmails()
			user := ts.newUser("custom_"+test.name, true)
			ts.Require().True(ts.authenticates(user.username, test.name, test.oldValue),
				"precondition: the original credential authenticates")
			_, link := ts.sendLink(user, test.name)
			ts.False(ts.authenticates(user.username, test.name, test.oldValue), "the original credential is removed")
			prompt := ts.openLink(link)
			ts.Require().Len(prompt.Data.Inputs, 1)
			ts.Equal(test.name, prompt.Data.Inputs[0].Identifier)
			ts.Equal("PASSWORD_INPUT", prompt.Data.Inputs[0].Type, "credential inputs must be masked")
			meta, err := json.Marshal(prompt.Data.Meta)
			ts.Require().NoError(err)
			ts.Contains(string(meta), `"label":"`+test.label+`"`, "the schema label or attribute name must be shown")
			done := ts.submit(prompt, adminResetSubmitAction, map[string]string{test.name: test.newValue})
			ts.Require().NotNil(done.Step, string(done.Body))
			ts.Equal("COMPLETE", done.Step.FlowStatus, string(done.Body))
			ts.True(ts.authenticates(user.username, test.name, test.newValue), "the replacement credential authenticates")
			ts.True(ts.authenticates(user.username, "password", user.password), "other credentials stay unchanged")
			if test.name == "pin" {
				ts.False(ts.authenticates(user.username, "pin", "1234"), "string PIN leading zeros are significant")
			}
		})
	}
}

// A number credential is refused before anything is sent, since it is not hashed when a user is created.
func (ts *AdminInitiatedResetTestSuite) TestANumberCredentialIsRefusedAtTheStart() {
	user := ts.newUser("number_credential", true)

	result := ts.start(ts.adminToken(), ts.flowHandle,
		map[string]string{"subject": user.id, "credential": "numericCode"})

	ts.requireRefused(result, adminResetCodeUnsupportedInput, "a number credential must be refused")
	ts.requireNoEmail("nothing may be sent for a number credential")
	ts.True(ts.authenticates(user.username, "password", user.password), "the password must keep working")
}

// Starting the reset signs the user out: an access token and a refresh token issued before it are refused.
func (ts *AdminInitiatedResetTestSuite) TestStartingTheResetRevokesTheUsersTokens() {
	user := ts.newUser("signout", true)
	tokens, err := testutils.ObtainAccessTokenWithPassword(ts.clientID, adminResetRedirectURI, "openid",
		user.username, user.password, true, adminResetClientSecret)
	ts.Require().NoError(err, "precondition: the user signs in")
	ts.Require().NotEmpty(tokens.RefreshToken, "precondition: the sign-in returns a refresh token")
	before, err := testutils.RefreshAccessTokenWithClientCredentialsInBody(ts.clientID, adminResetClientSecret,
		tokens.RefreshToken)
	ts.Require().NoError(err, "precondition: the refresh token works before the reset")
	ts.Require().True(ts.tokenActive(before.AccessToken), "precondition: the access token is active before the reset")

	ts.sendLink(user, "password")

	ts.Eventually(func() bool { return !ts.tokenActive(before.AccessToken) }, 10*time.Second,
		250*time.Millisecond, "the user's access token must be revoked once the reset starts")

	ts.Eventually(func() bool {
		_, err := testutils.RefreshAccessTokenWithClientCredentialsInBody(ts.clientID, adminResetClientSecret,
			before.RefreshToken)
		return err != nil && strings.Contains(err.Error(), "invalid_grant")
	}, 10*time.Second, 250*time.Millisecond, "the user's refresh token must be refused after the reset starts")
}

// A start without an administrator's token, or by a user without the permission, is refused before anything is
// sent or changed.
func (ts *AdminInitiatedResetTestSuite) TestAStartWithoutAnAdministratorIsRefused() {
	user := ts.newUser("unauthorized", true)
	other := ts.newUser("unauthorized_other", true)
	otherToken, err := testutils.ObtainAccessTokenWithPassword(ts.clientID, adminResetRedirectURI, "openid",
		other.username, other.password, true, adminResetClientSecret)
	ts.Require().NoError(err, "precondition: another user signs in")

	for name, test := range map[string]struct {
		token  string
		status int
		code   string
	}{
		"no token":             {"", http.StatusUnauthorized, adminResetCodeNoAuthn},
		"another user's token": {otherToken.AccessToken, http.StatusForbidden, adminResetCodeNoPermission},
	} {
		ts.Run(name, func() {
			result := ts.start(test.token, ts.flowHandle,
				map[string]string{"subject": user.id, "credential": "password"})

			ts.Nil(result.Step, "the start must be refused before the flow runs: %s", result.Body)
			ts.Equal(test.status, result.StatusCode, string(result.Body))
			ts.Require().NotNil(result.Error, "the refusal must be reported: %s", result.Body)
			ts.Equal(test.code, result.Error.Code)
		})
	}
	ts.requireNoEmail("nothing may be sent when the start is refused")
	ts.True(ts.authenticates(user.username, "password", user.password), "the password must keep working")
}

// An organization unit administrator resets a user of its unit, and is refused for users outside it alike,
// whether or not they are suspended, with nothing sent or changed.
func (ts *AdminInitiatedResetTestSuite) TestAnOrganizationUnitAdministratorIsConfinedToItsUnit() {
	token, childOU := ts.ouAdministratorToken()
	handle := ts.copyFlow("admin-reset-ou-"+ts.suffix, childOU, ts.senderID)
	outside := ts.newUser("ou_outside", true)
	suspendedOutside := ts.newUser("ou_outside_suspended", true)
	ts.suspend(suspendedOutside)
	inside := ts.newUserIn("ou_inside", childOU, true)

	refusals := map[string]string{}
	for name, target := range map[string]resetUser{"active": outside, "suspended": suspendedOutside} {
		refused := ts.start(token, handle, map[string]string{"subject": target.id, "credential": "password"})

		ts.Require().NotNil(refused.Step, "the refusal must be answered in the flow: %s", refused.Body)
		ts.Equal("ERROR", refused.Step.FlowStatus, "a user outside the unit must be refused: %s", refused.Body)
		ts.Require().NotNil(refused.Step.Error, "the refusal must be reported: %s", refused.Body)
		refusals[name] = refused.Step.Error.Code
	}
	// A suspended user cannot sign in at all, so only the active user shows the password was kept.
	ts.True(ts.authenticates(outside.username, "password", outside.password),
		"a user outside the unit must keep the password")
	ts.requireNoEmail("nothing may be sent to a user outside the unit")
	ts.Equal(refusals["active"], refusals["suspended"],
		"the refusal must not tell the administrator that a user outside the unit is suspended")

	sent := ts.start(token, handle, map[string]string{"subject": inside.id, "credential": "password"})

	ts.Require().NotNil(sent.Step, "the start must run: %s", sent.Body)
	ts.Require().Equal("INCOMPLETE", sent.Step.FlowStatus, "a user of the unit is sent a link: %s", sent.Body)
	link := ts.awaitLink(inside)
	ts.False(ts.authenticates(inside.username, "password", inside.password), "the old password must be blocked")
	done := ts.submit(ts.openLink(link), adminResetSubmitAction, map[string]string{"password": "Unit-Reset-1!"})
	ts.Require().NotNil(done.Step, "the reset must be answered in the flow: %s", done.Body)
	ts.Equal("COMPLETE", done.Step.FlowStatus, "the user must complete the reset an OU administrator started: %s",
		done.Body)
	ts.True(ts.authenticates(inside.username, "password", "Unit-Reset-1!"), "the new password must work")
}

// An organization unit administrator cannot start a copy of the flow its unit neither owns nor was shared.
func (ts *AdminInitiatedResetTestSuite) TestAnOrganizationUnitAdministratorCannotStartAnUnsharedFlow() {
	token, childOU := ts.ouAdministratorToken()
	inside := ts.newUserIn("ou_unshared", childOU, true)

	result := ts.start(token, ts.flowHandle, map[string]string{"subject": inside.id, "credential": "password"})

	ts.Nil(result.Step, "the start must be refused before the flow runs: %s", result.Body)
	ts.Equal(http.StatusForbidden, result.StatusCode, string(result.Body))
	ts.Require().NotNil(result.Error, "the refusal must be reported: %s", result.Body)
	ts.Equal("FES-1029", result.Error.Code)
	ts.requireNoEmail("nothing may be sent when the flow is not shared with the unit")
	ts.True(ts.authenticates(inside.username, "password", inside.password), "the password must keep working")
}

// A user without a stored email is refused, nothing is sent, and the credential keeps working.
func (ts *AdminInitiatedResetTestSuite) TestAUserWithoutAnEmailIsRefused() {
	user := ts.newUser("no_email", false)

	result := ts.start(ts.adminToken(), ts.flowHandle,
		map[string]string{"subject": user.id, "credential": "password"})

	ts.requireRefused(result, adminResetCodeNoEmail, "a user without an email must be refused")
	ts.requireNoEmail("nothing may be sent to a user without an email")
	ts.True(ts.authenticates(user.username, "password", user.password), "the password must keep working")
}

// A link the email could not carry is never handed to the administrator, and the credential keeps working,
// whether the sender is gone or the mail server does not answer.
func (ts *AdminInitiatedResetTestSuite) TestAFailedSendKeepsTheCredentialAndHidesTheLink() {
	removed, err := testutils.CreateSMTPEmailProvider("Admin Reset removed "+ts.suffix, "127.0.0.1", 2525,
		"noreply@thunderid.test")
	ts.Require().NoError(err, "create the sender to remove")
	ts.Require().NoError(testutils.DeleteNotificationSender(removed), "remove the sender the flow will name")
	unreachable := testutils.NewMockSMTPServer(0)
	ts.Require().NoError(unreachable.Start())
	unreachablePort := unreachable.GetPort()
	ts.Require().NoError(unreachable.Stop(), "stop the mail server the sender names")
	closed, err := testutils.CreateSMTPEmailProvider("Admin Reset unreachable "+ts.suffix, "127.0.0.1",
		unreachablePort, "noreply@thunderid.test")
	ts.Require().NoError(err, "create the sender of a mail server that does not answer")
	ts.T().Cleanup(func() { _ = testutils.DeleteNotificationSender(closed) })

	for name, senderID := range map[string]string{"a removed sender": removed, "an unreachable server": closed} {
		ts.Run(name, func() {
			handle := ts.copyFlow("admin-reset-failing-"+strings.ReplaceAll(name, " ", "-")+"-"+ts.suffix, "",
				senderID)
			user := ts.newUser("failed_send_"+strings.ReplaceAll(name, " ", "_"), true)

			result := ts.start(ts.adminToken(), handle,
				map[string]string{"subject": user.id, "credential": "password"})

			ts.False(result.Step != nil && result.Step.FlowStatus == "INCOMPLETE",
				"a reset whose email was not sent must not be accepted: %s", result.Body)
			for _, leaked := range []string{"inviteLink", "inviteToken", "executionId="} {
				ts.NotContains(string(result.Body), leaked, "the administrator must not get the link")
			}
			ts.True(ts.authenticates(user.username, "password", user.password), "the password must keep working")
		})
	}
	ts.requireNoEmail("nothing may be delivered")
}

// A second reset voids the link of the first, and only the latest link completes.
func (ts *AdminInitiatedResetTestSuite) TestASecondResetVoidsTheFirstLink() {
	user := ts.newUser("second", true)
	_, first := ts.sendLink(user, "password")
	ts.smtp.ClearEmails()
	_, second := ts.sendLink(user, "password")

	refused := ts.openLinkRaw(first)

	ts.requireRefused(refused, adminResetCodeVoided, "the first link must be refused once a second reset starts")
	ts.NotContains(string(refused.Body), user.username, "a refused link must not name the user")
	done := ts.submit(ts.openLink(second), adminResetSubmitAction, map[string]string{"password": "Second-Link-1!"})
	ts.Require().NotNil(done.Step, "the second link must be answered in the flow: %s", done.Body)
	ts.Equal("COMPLETE", done.Step.FlowStatus, "the latest link must complete: %s", done.Body)
	ts.True(ts.authenticates(user.username, "password", "Second-Link-1!"), "the new password must work")
}

// A link opened before a later credential change voids it at the password step, so it cannot overwrite the
// newer credential.
func (ts *AdminInitiatedResetTestSuite) TestACredentialChangeAfterTheLinkWasOpenedVoidsIt() {
	user := ts.newUser("changed_after_open", true)
	_, link := ts.sendLink(user, "password")
	prompt := ts.openLink(link)
	ts.Require().NoError(testutils.UpdateUserCredentials(user.id, map[string]string{"password": "Admin-Set-1!"}),
		"precondition: an administrator sets the password directly")

	refused := ts.submit(prompt, adminResetSubmitAction, map[string]string{"password": "Too-Late-1!"})

	ts.requireRefused(refused, adminResetCodeVoided, "a link a later change voided must not set the credential")
	ts.True(ts.authenticates(user.username, "password", "Admin-Set-1!"), "the later credential must stand")
	ts.False(ts.authenticates(user.username, "password", "Too-Late-1!"), "the voided link must set nothing")
}

// A forgot-password reset during a pending administrator reset works, and voids the pending link.
func (ts *AdminInitiatedResetTestSuite) TestForgotPasswordVoidsAPendingLink() {
	appID := ts.recoveryApplication()
	user := ts.newUser("forgot", true)
	_, link := ts.sendLink(user, "password")
	ts.smtp.ClearEmails()

	step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
	ts.Require().NoError(err, "precondition: forgot password starts")
	step, err = common.CompleteFlow(step.ExecutionID, map[string]string{"username": user.username},
		"action_submit_username", step.ChallengeToken)
	ts.Require().NoError(err)
	recoveryLink := ts.awaitLink(user)
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"inviteToken": testutils.ExtractQueryParam(recoveryLink, "inviteToken")}, "",
		step.ChallengeToken)
	ts.Require().NoError(err)
	step, err = common.CompleteFlow(step.ExecutionID, map[string]string{"password": "Forgot-Reset-1!"},
		"action_submit_password", step.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", step.FlowStatus, "a pending administrator reset must not block forgot password")
	ts.True(ts.authenticates(user.username, "password", "Forgot-Reset-1!"), "the recovered password must work")

	refused := ts.openLinkRaw(link)

	ts.requireRefused(refused, adminResetCodeVoided, "the pending link must be refused after forgot password")
	ts.True(ts.authenticates(user.username, "password", "Forgot-Reset-1!"), "the recovered password must stand")
}

// A suspended user is refused at the start, and nothing is sent or changed.
func (ts *AdminInitiatedResetTestSuite) TestASuspendedUserIsRefusedAtTheStart() {
	user := ts.newUser("suspended_start", true)
	ts.suspend(user)

	result := ts.start(ts.adminToken(), ts.flowHandle,
		map[string]string{"subject": user.id, "credential": "password"})

	ts.requireRefused(result, adminResetCodeSuspended, "a suspended user must be refused at the start")
	ts.requireNoEmail("nothing may be sent to a suspended user")
	ts.Equal("SUSPENDED", ts.status(user).Value, "the user must stay suspended")
}

// A user suspended after the link was sent cannot complete the reset, and the reset leaves the suspension.
func (ts *AdminInitiatedResetTestSuite) TestAUserSuspendedAfterTheLinkWasSentCannotComplete() {
	user := ts.newUser("suspended_later", true)
	_, link := ts.sendLink(user, "password")
	ts.suspend(user)

	refused := ts.openLinkRaw(link)

	ts.requireRefused(refused, adminResetCodeSuspended, "a suspended user must not complete the reset")
	ts.False(ts.authenticates(user.username, "password", user.password), "the old password must stay blocked")
	ts.Equal("SUSPENDED", ts.status(user).Value, "the reset must not remove the suspension")
}

// A user locked by failed sign-ins receives the link, completes the reset, and the lock is released.
func (ts *AdminInitiatedResetTestSuite) TestALockedUserCompletesTheResetAndIsUnlocked() {
	user := ts.newUser("locked", true)
	for i := 0; i < adminResetLockThreshold; i++ {
		ts.False(ts.authenticates(user.username, "password", "Wrong-Password-1!"))
	}
	ts.Require().Equal("LOCKED", ts.status(user).Value, "precondition: failed sign-ins lock the user")

	_, link := ts.sendLink(user, "password")
	ts.Equal("LOCKED", ts.status(user).Value, "sending the link must not release the lock")
	done := ts.submit(ts.openLink(link), adminResetSubmitAction, map[string]string{"password": "Unlocked-1!"})

	ts.Require().NotNil(done.Step, "the reset must be answered in the flow: %s", done.Body)
	ts.Equal("COMPLETE", done.Step.FlowStatus, "a locked user must complete the reset: %s", done.Body)
	ts.Equal("ACTIVE", ts.status(user).Value, "completing the reset must release the lock")
	ts.True(ts.authenticates(user.username, "password", "Unlocked-1!"), "the new password must work")
}

// The Console reads which flow to start from adminCredentialResetFlow, so bootstrap names the shipped flow there.
func (ts *AdminInitiatedResetTestSuite) TestServerConfigNamesTheShippedFlow() {
	resp, err := testutils.GetHTTPClient().Get(testutils.TestServerURL + "/server-config/flow")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var layers struct {
		Merged map[string]struct {
			DefaultHandle string `json:"defaultHandle"`
		} `json:"merged"`
	}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&layers))
	ts.Equal(adminResetFlowHandle, layers.Merged["adminCredentialResetFlow"].DefaultHandle)
}

// copyFlow copies the shipped flow, owned by ouID or the default unit when empty, with senderID on its send
// node, deletes it when the test ends, and returns its handle.
func (ts *AdminInitiatedResetTestSuite) copyFlow(handle, ouID, senderID string) string {
	t := ts.T()
	flowID, err := testutils.CopyFlowWithSendersInOU(adminResetFlowHandle, adminInitiatedFlowType, handle, ouID,
		map[string]string{adminResetSendNode: senderID}, nil)
	ts.Require().NoError(err, "failed to copy the shipped administrator credential reset flow")
	t.Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			t.Logf("cleanup: failed to delete flow %s: %v", flowID, err)
		}
	})
	return handle
}

// copyFlowSendingTo copies the shipped flow, with the suite's sender, sending the link to the attribute given.
func (ts *AdminInitiatedResetTestSuite) copyFlowSendingTo(handle, attribute string) string {
	t := ts.T()
	shippedID, err := testutils.GetFlowIDByHandle(adminResetFlowHandle, adminInitiatedFlowType)
	ts.Require().NoError(err)
	flow, err := testutils.GetFlow(shippedID)
	ts.Require().NoError(err)
	ts.Require().NoError(testutils.SetFlowNodeProperty(&flow, adminResetSendNode, "senderId", ts.senderID))
	ts.Require().NoError(testutils.ModifyFlowNode(&flow, adminResetSendNode, func(node map[string]interface{}) {
		executor, _ := node["executor"].(map[string]interface{})
		ts.Require().NotNil(executor, "the send node must carry an executor")
		executor["inputs"] = []map[string]interface{}{{"identifier": attribute, "type": "EMAIL_INPUT", "required": true}}
	}))
	flow.Handle = handle
	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "failed to create the flow sending to %s", attribute)
	t.Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			t.Logf("cleanup: failed to delete flow %s: %v", flowID, err)
		}
	})
	return handle
}

// newUser creates a user of the suite's type with a password and a PIN, and an email when withEmail is set.
func (ts *AdminInitiatedResetTestSuite) newUser(name string, withEmail bool) resetUser {
	return ts.newUserIn(name, ts.ouID, withEmail)
}

// newUserIn is newUser in the unit given.
func (ts *AdminInitiatedResetTestSuite) newUserIn(name, ouID string, withEmail bool) resetUser {
	t := ts.T()
	username := fmt.Sprintf("reset_%s_%s", name, ts.suffix)
	user := resetUser{username: username, password: "Old-Pass-" + name + "1!", pin: "135790"}
	attributes := map[string]interface{}{"username": username, "password": user.password, "pin": user.pin,
		"accessCode": "Old-Access-1!", "numericCode": 135790}
	if withEmail {
		user.email = username + "@example.com"
		user.workEmail = "work_" + username + "@example.com"
		attributes["email"] = user.email
		attributes["workEmail"] = user.workEmail
	}
	raw, err := json.Marshal(attributes)
	ts.Require().NoError(err)
	user.id, err = testutils.CreateUser(testutils.User{OUID: ouID, Type: ts.userType, Attributes: raw})
	ts.Require().NoError(err, "failed to create user %s", username)
	t.Cleanup(func() {
		if err := testutils.DeleteUser(user.id); err != nil {
			t.Logf("cleanup: failed to delete user %s: %v", username, err)
		}
	})
	return user
}

// adminToken returns the administrator's access token.
func (ts *AdminInitiatedResetTestSuite) adminToken() string {
	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	return token
}

// start starts the flow the handle names as the caller the token names.
func (ts *AdminInitiatedResetTestSuite) start(token, handle string,
	inputs map[string]string) *common.FlowExecutionResult {
	return ts.continueAs(token, map[string]interface{}{
		"flowType": adminInitiatedFlowType, "handle": handle, "inputs": inputs, "verbose": true,
	})
}

// continueAs posts one flow request as the caller the token names, or with no token when it is empty.
func (ts *AdminInitiatedResetTestSuite) continueAs(token string,
	request map[string]interface{}) *common.FlowExecutionResult {
	result, err := common.ExecuteFlowWithToken(token, request)
	ts.Require().NoError(err)
	return result
}

// continueAsUser posts one flow request with no token, as the user's browser does.
func (ts *AdminInitiatedResetTestSuite) continueAsUser(request map[string]interface{}) *common.FlowExecutionResult {
	return ts.continueAs("", request)
}

// sendLink has the administrator start the suite's flow for the user's credential, and returns the start's
// response and the link the user receives.
func (ts *AdminInitiatedResetTestSuite) sendLink(user resetUser,
	credential string) (*common.FlowExecutionResult, string) {
	result := ts.start(ts.adminToken(), ts.flowHandle, map[string]string{"subject": user.id, "credential": credential})
	ts.Require().Equal(http.StatusOK, result.StatusCode, "the administrator's start must be accepted: %s", result.Body)
	ts.Require().NotNil(result.Step, "the start must be answered in the flow: %s", result.Body)
	ts.Require().Equal("INCOMPLETE", result.Step.FlowStatus, "the flow must wait once the link is sent: %s",
		result.Body)
	return result, ts.awaitLink(user)
}

// awaitLink waits for the email to the user's stored address and returns the link it carries.
func (ts *AdminInitiatedResetTestSuite) awaitLink(user resetUser) string {
	return ts.awaitLinkAt(user.email)
}

// awaitLinkAt waits for the email to the address given and returns the link it carries.
func (ts *AdminInitiatedResetTestSuite) awaitLinkAt(address string) string {
	var email *testutils.EmailMessage
	ts.Require().Eventually(func() bool {
		email = ts.smtp.GetLastEmail()
		return email != nil
	}, adminResetDeliveryWait, 50*time.Millisecond, "no email arrived for %s", address)
	ts.Require().Equal([]string{address}, email.To, "the email must go to that stored address only")
	link := email.ExtractRecoveryLink()
	ts.Require().NotEmpty(link, "the email must carry the link")
	return link
}

// requireNoEmail requires that the requests made so far sent no email. The answer to each request is the
// barrier: executors send before the flow answers, and the mock records a message before acknowledging it.
func (ts *AdminInitiatedResetTestSuite) requireNoEmail(msg string) {
	ts.Empty(ts.smtp.GetEmails(), msg)
}

// openLinkRaw continues the flow from the link as the user's browser does, with no token.
func (ts *AdminInitiatedResetTestSuite) openLinkRaw(link string) *common.FlowExecutionResult {
	return ts.continueAsUser(map[string]interface{}{
		"executionId": testutils.ExtractQueryParam(link, "executionId"),
		"inputs":      map[string]string{"inviteToken": testutils.ExtractQueryParam(link, "inviteToken")},
		"verbose":     true,
	})
}

// openLink opens the link and requires the flow to ask for the new credential.
func (ts *AdminInitiatedResetTestSuite) openLink(link string) *common.FlowStep {
	result := ts.openLinkRaw(link)
	ts.Require().NotNil(result.Step, "the link must reach the flow: %s", result.Body)
	ts.Require().Equal("INCOMPLETE", result.Step.FlowStatus, "the link must lead to the new credential: %s",
		result.Body)
	return result.Step
}

// submit sends the user's inputs on the step's action, with no token.
func (ts *AdminInitiatedResetTestSuite) submit(step *common.FlowStep, action string,
	inputs map[string]string) *common.FlowExecutionResult {
	return ts.continueAsUser(map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken, "action": action, "inputs": inputs,
	})
}

// requireRefused requires the flow to answer with an error step carrying code.
func (ts *AdminInitiatedResetTestSuite) requireRefused(result *common.FlowExecutionResult, code, reason string) {
	ts.Require().NotNil(result.Step, "%s, in the flow: %s", reason, result.Body)
	ts.Require().Equal("ERROR", result.Step.FlowStatus, "%s: %s", reason, result.Body)
	ts.Require().NotNil(result.Step.Error, "%s, with its reason: %s", reason, result.Body)
	ts.Equal(code, result.Step.Error.Code, "%s: %s", reason, result.Body)
}

// authenticates reports whether the credential signs the user in through the credentials endpoint.
func (ts *AdminInitiatedResetTestSuite) authenticates(username, credentialKey, value string) bool {
	ok, err := testutils.AuthenticateWithCredential("username", username, credentialKey, value)
	ts.Require().NoError(err)
	return ok
}

// tokenActive reports whether introspection, as the suite's application, finds the token active.
func (ts *AdminInitiatedResetTestSuite) tokenActive(token string) bool {
	result, err := testutils.IntrospectTokenResult(token, ts.clientID, adminResetClientSecret, true)
	ts.Require().NoError(err)
	return result.Active
}

// status returns the user's effective access status.
func (ts *AdminInitiatedResetTestSuite) status(user resetUser) *testutils.AccessStatus {
	status, err := testutils.GetAccessStatus("users", user.id)
	ts.Require().NoError(err)
	return status
}

// suspend suspends the user through the shipped suspension flow.
func (ts *AdminInitiatedResetTestSuite) suspend(user resetUser) {
	flowID, err := testutils.GetFlowIDByHandle(adminResetSuspendHandle, "ADMINISTRATION")
	ts.Require().NoError(err)
	result := ts.continueAs(ts.adminToken(),
		map[string]interface{}{"flowId": flowID, "inputs": map[string]string{"subject": user.id}})
	ts.Require().NotNil(result.Step, "precondition: the user is suspended: %s", result.Body)
	ts.Require().Equal("COMPLETE", result.Step.FlowStatus, "precondition: the user is suspended: %s", result.Body)
}

// recoveryApplication creates an application whose recovery flow emails a reset link through the suite's
// sender, and returns its ID.
func (ts *AdminInitiatedResetTestSuite) recoveryApplication() string {
	t := ts.T()
	flow := buildEmailLinkPasswordRecoveryFlow(ts.senderID)
	flow.Handle = "admin-reset-forgot-" + ts.suffix
	flow.Name = "Admin Reset Forgot Password " + ts.suffix
	recoveryFlowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "failed to create the recovery flow")
	t.Cleanup(func() { _ = testutils.DeleteFlow(recoveryFlowID) })
	authFlowID, err := testutils.CreateIsolatedAuthFlow("admin-reset-forgot-auth-" + ts.suffix)
	ts.Require().NoError(err, "failed to create the authentication flow")
	t.Cleanup(func() { _ = testutils.DeleteFlow(authFlowID) })
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID: ts.ouID, Name: "Admin Reset Forgot " + ts.suffix, IsRecoveryFlowEnabled: true,
		ClientID: "admin_reset_forgot_" + ts.suffix, ClientSecret: adminResetClientSecret,
		RedirectURIs: []string{adminResetRedirectURI}, AllowedUserTypes: []string{ts.userType},
		AuthFlowID: authFlowID, RecoveryFlowID: recoveryFlowID,
	})
	ts.Require().NoError(err, "failed to create the recovery application")
	t.Cleanup(func() { _ = testutils.DeleteApplication(appID) })
	return appID
}

// ouAdministratorToken creates a child unit of the suite's unit and a user there holding org:user but not the
// root system permission, and returns that user's token and the unit.
func (ts *AdminInitiatedResetTestSuite) ouAdministratorToken() (string, string) {
	t := ts.T()
	name := fmt.Sprintf("ouadmin%d", time.Now().UnixNano())
	parent := ts.ouID
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "admin-reset-" + name, Name: "Admin Reset " + name, Parent: &parent,
	})
	ts.Require().NoError(err, "create the child unit")
	var signInApps testutils.OUSignInApps
	t.Cleanup(func() {
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			t.Logf("cleanup: delete the child unit: %v", err)
		}
	})
	t.Cleanup(func() {
		if err := signInApps.DeleteAll(); err != nil {
			t.Logf("cleanup: delete the sign-in application: %v", err)
		}
	})

	admin := ts.newUserIn(name, ouID, true)
	identifier := "https://admin-reset.example.com/" + name
	rsID, err := testutils.CreateOrgScopedResourceServer(ouID, "Admin Reset "+name+" RS", identifier, "user")
	ts.Require().NoError(err, "create the resource server")
	t.Cleanup(func() {
		if err := testutils.DeleteResourceServerWithChildren(rsID); err != nil {
			t.Logf("cleanup: delete the resource server: %v", err)
		}
	})
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Admin Reset " + name, OUID: ouID,
		Assignments: []testutils.Assignment{{ID: admin.id, Type: "user"}},
		Permissions: []testutils.ResourcePermissions{{ResourceServerID: rsID, Permissions: []string{"org:user"}}},
	})
	ts.Require().NoError(err, "create the role")
	t.Cleanup(func() {
		if err := testutils.DeleteRole(roleID); err != nil {
			t.Logf("cleanup: delete the role: %v", err)
		}
	})

	token, err := signInApps.ObtainAccessToken(ouID, "system org:user", admin.username, admin.password, identifier)
	ts.Require().NoError(err, "sign the administrator in")
	return token.AccessToken, ouID
}
