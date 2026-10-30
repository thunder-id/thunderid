// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The actions of the flows this suite builds.
const (
	actionSubmitUsername = "action_submit_username"
	actionSubmitDetails  = "action_submit_details"
	actionSubmitOTP      = "action_submit_otp"
	actionSubmitEmail    = "action_submit_email"
	actionSendInvite     = "action_send_email_invite"
	actionShareInvite    = "action_share_manually"
	actionContinue       = "action_continue"

	displayNameAttribute = "displayName"
)

// OtherFlowTypesTestSuite checks verification in the flow types that establish their user themselves:
// recovery verifies the user it identified, and registration and onboarding verify the value they
// collected for the user they create. It also checks the flow types that have no user to verify.
type OtherFlowTypesTestSuite struct {
	suite.Suite
	fixture    *verificationFixture
	authFlowID string
}

func TestOtherFlowTypesTestSuite(t *testing.T) {
	suite.Run(t, new(OtherFlowTypesTestSuite))
}

func (ts *OtherFlowTypesTestSuite) SetupSuite() {
	t := ts.T()
	ts.fixture = newVerificationFixture(t, "other_types")

	ts.authFlowID = createFlow(t, testutils.Flow{
		Name:     "Attribute Verification Other Types Sign-in",
		FlowType: "AUTHENTICATION",
		Handle:   "verification-other-types-sign-in-" + ts.fixture.suffix,
		Nodes:    linearNodes(taskNode("auth_assert", "AuthAssertExecutor", "")),
	})
}

func (ts *OtherFlowTypesTestSuite) SetupTest() {
	ts.fixture.smtp.ClearEmails()
}

// createApp creates an application for the fixture's user type, with the flows app names, and deletes
// it when the test ends.
func (ts *OtherFlowTypesTestSuite) createApp(name string, app testutils.Application) string {
	t := ts.T()
	t.Helper()

	app.Name = "Attribute Verification " + name + " App " + ts.fixture.suffix
	app.Description = "Application whose flow verifies a contact"
	app.OUID = ts.fixture.ouID
	app.ClientID = "verification_" + name + "_" + ts.fixture.suffix
	app.ClientSecret = "Verification-Other-Types-Secret1!"
	app.AllowedUserTypes = []string{ts.fixture.userType}
	if app.AuthFlowID == "" {
		app.AuthFlowID = ts.authFlowID
	}
	return createApp(t, app)
}

// recoveryApp creates a flow of the recovery type from nodes and an application it recovers for.
func (ts *OtherFlowTypesTestSuite) recoveryApp(name string, nodes []map[string]interface{}) string {
	ts.T().Helper()

	flowID := createFlow(ts.T(), testutils.Flow{Name: "Attribute Verification Recovery " + name,
		FlowType: "RECOVERY", Handle: "verification-recovery-" + name + "-" + ts.fixture.suffix, Nodes: nodes})
	return ts.createApp("recovery_"+name, testutils.Application{IsRecoveryFlowEnabled: true,
		RecoveryFlowID: flowID})
}

// registrationApp creates a flow of the registration type from nodes and an application it registers
// users for.
func (ts *OtherFlowTypesTestSuite) registrationApp(name string, nodes []map[string]interface{}) string {
	ts.T().Helper()

	flowID := createFlow(ts.T(), testutils.Flow{Name: "Attribute Verification Registration " + name,
		FlowType: "REGISTRATION", Handle: "verification-registration-" + name + "-" + ts.fixture.suffix,
		Nodes: nodes})
	return ts.createApp("registration_"+name, testutils.Application{IsRegistrationFlowEnabled: true,
		RegistrationFlowID: flowID})
}

// continueFlow continues the step with inputs through action, requiring the flow to answer.
func (ts *OtherFlowTypesTestSuite) continueFlow(step *common.FlowStep, inputs map[string]string,
	action string) *common.FlowStep {
	ts.T().Helper()

	next, err := common.CompleteFlow(step.ExecutionID, inputs, action, step.ChallengeToken)
	ts.Require().NoError(err, "the request must be answered in the flow")
	ts.False(common.HasInput(next.Data.Inputs, "attribute"),
		"the flow must not ask which attribute to verify: %+v", next.Data)
	return next
}

// requireStepRefused requires the flow to have stopped with code.
func (ts *OtherFlowTypesTestSuite) requireStepRefused(step *common.FlowStep, code, reason string) {
	ts.T().Helper()

	ts.Require().Equal(flowStatusError, step.FlowStatus, "%s: %+v", reason, step)
	ts.Require().NotNil(step.Error, "the refusal must be reported: %+v", step)
	ts.Require().Equal(code, step.Error.Code, "%s: %+v", reason, step.Error)
}

// newUserDetails is a username and an address for a user a flow will create, distinct from every other
// test's.
func (ts *OtherFlowTypesTestSuite) newUserDetails(name string) (string, string) {
	username := fmt.Sprintf("verify_%s_%s", name, ts.fixture.suffix)
	return username, fmt.Sprintf("%s.%s@example.com", name, ts.fixture.suffix)
}

// createdUser returns the user a flow created with username, or nil when there is none, and deletes it
// with the fixture.
func (ts *OtherFlowTypesTestSuite) createdUser(username string) (*testutils.User, map[string]interface{}) {
	ts.T().Helper()

	status, body := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodGet,
		"/users?filter="+url.QueryEscape(fmt.Sprintf("username eq %q", username)), nil)
	ts.Require().Equal(http.StatusOK, status, "the user list must answer: %s", body)
	var list testutils.UserListResponse
	ts.Require().NoError(json.Unmarshal(body, &list))
	if len(list.Users) == 0 {
		return nil, nil
	}
	ts.Require().Len(list.Users, 1, "one user must have the username: %s", body)
	user := list.Users[0]
	ts.fixture.userIDs = append(ts.fixture.userIDs, user.ID)
	var attributes map[string]interface{}
	ts.Require().NoError(json.Unmarshal(user.Attributes, &attributes))
	return &user, attributes
}

// Recovery that identifies the user verifies their email with the code sent to the stored address.
func (ts *OtherFlowTypesTestSuite) TestRecoveryVerifiesTheIdentifiedUsersEmailWithACode() {
	appID := ts.recoveryApp("code", recoveryCodeFlow(ts.fixture.senderID))
	user := ts.fixture.newUser(ts.T(), "recovery_code")
	started := time.Now()

	step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
	ts.Require().NoError(err)
	step = ts.continueFlow(step, map[string]string{"username": user.username}, actionSubmitUsername)
	ts.Require().Equal(flowStatusIncomplete, step.FlowStatus, "the flow must wait for the code: %+v", step)
	done := ts.continueFlow(step, map[string]string{"otp": ts.fixture.awaitCode(ts.T(), user.email)},
		actionSubmitOTP)

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the recovery must complete: %+v", done)
	requireVerifiedSince(ts.T(), user.id, emailAttribute, started)
	ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "only the email may be verified")
}

// Recovery that identifies the user verifies their email with the link sent to the stored address.
func (ts *OtherFlowTypesTestSuite) TestRecoveryVerifiesTheIdentifiedUsersEmailWithALink() {
	appID := ts.recoveryApp("link", recoveryLinkFlow(ts.fixture.senderID))
	user := ts.fixture.newUser(ts.T(), "recovery_link")
	started := time.Now()

	step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
	ts.Require().NoError(err)
	step = ts.continueFlow(step, map[string]string{"username": user.username}, actionSubmitUsername)
	ts.Require().Equal(flowStatusIncomplete, step.FlowStatus, "the flow must wait for the link: %+v", step)
	link := ts.fixture.awaitEmailTo(ts.T(), user.email).ExtractRecoveryLink()
	ts.Require().NotEmpty(link, "the email must carry the link")
	done := openLink(ts.T(), link, testutils.ExtractQueryParam(link, "inviteToken"))

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the recovery must complete: %+v", done)
	requireVerifiedSince(ts.T(), user.id, emailAttribute, started)
}

// The attribute property decides what is verified: an attribute input naming another contact does not
// change it.
func (ts *OtherFlowTypesTestSuite) TestAnAttributeInputDoesNotChangeTheProperty() {
	appID := ts.recoveryApp("property", recoveryCodeFlow(ts.fixture.senderID))
	user := ts.fixture.newUser(ts.T(), "recovery_property")

	step, err := common.InitiateRecoveryFlow(appID, false, map[string]string{"attribute": mobileAttribute}, "")
	ts.Require().NoError(err)
	step = ts.continueFlow(step, map[string]string{"username": user.username, "attribute": mobileAttribute},
		actionSubmitUsername)
	ts.Require().Equal(flowStatusIncomplete, step.FlowStatus, "the flow must wait for the code: %+v", step)
	done := ts.continueFlow(step, map[string]string{"otp": ts.fixture.awaitCode(ts.T(), user.email)},
		actionSubmitOTP)

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the recovery must complete: %+v", done)
	ts.NotEmpty(verifiedAt(ts.T(), user.id, emailAttribute), "the attribute the property names must be verified")
	ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "the attribute an input names must not be verified")
}

// Recovery that prepares before identifying a user fails without sending, even after collecting a
// username.
func (ts *OtherFlowTypesTestSuite) TestRecoveryPreparingBeforeIdentifyingIsRefused() {
	nodes := linearNodes(append([]map[string]interface{}{
		promptNode("prompt_username", actionSubmitUsername, "username"),
		prepareNode(emailAttribute),
	}, append(emailCodeNodes(ts.fixture.senderID), recordNode(""))...)...)
	appID := ts.recoveryApp("unidentified", nodes)
	user := ts.fixture.newUser(ts.T(), "recovery_unidentified")

	step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
	ts.Require().NoError(err)
	step = ts.continueFlow(step, map[string]string{"username": user.username}, actionSubmitUsername)

	ts.requireStepRefused(step, errCodeVerificationUnavailable, "preparation must have no user to verify")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "nothing may be recorded")
}

// startRegistration starts the application's registration flow and submits, of a username, the address
// and a display name, those its details prompt asks for. The display name is shaped like an address, so
// only the user type tells it is not a contact.
func (ts *OtherFlowTypesTestSuite) startRegistration(appID, username, address string) *common.FlowStep {
	ts.T().Helper()

	step, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().True(common.HasInput(step.Data.Inputs, "username"), "the flow must ask for the details: %+v",
		step.Data)
	details := map[string]string{"username": username, emailAttribute: address,
		displayNameAttribute: "display." + address}
	inputs := map[string]string{}
	for _, input := range step.Data.Inputs {
		inputs[input.Identifier] = details[input.Identifier]
	}
	return ts.continueFlow(step, inputs, actionSubmitDetails)
}

// Registration that pins the collected email, checks a code sent to it and then creates the user
// verifies the new user's email.
func (ts *OtherFlowTypesTestSuite) TestRegistrationVerifiesTheNewUsersEmail() {
	appID := ts.registrationApp("verified", registrationFlow(ts.fixture.senderID, nil,
		taskNode("provisioning", "ProvisioningExecutor", ""), recordNode("")))
	username, address := ts.newUserDetails("registration_verified")
	started := time.Now()

	step := ts.startRegistration(appID, username, address)
	ts.Require().Equal(flowStatusIncomplete, step.FlowStatus, "the flow must wait for the code: %+v", step)
	done := ts.continueFlow(step, map[string]string{"otp": ts.fixture.awaitCode(ts.T(), address)}, actionSubmitOTP)

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the registration must complete: %+v", done)
	user, attributes := ts.createdUser(username)
	ts.Require().NotNil(user, "the registration must create the user")
	ts.Equal(address, attributes[emailAttribute], "the user must have the address the code was sent to")
	requireVerifiedSince(ts.T(), user.ID, emailAttribute, started)
}

// An address entered after the code check is the one the user is created with, and it is not verified:
// the code went to another address.
func (ts *OtherFlowTypesTestSuite) TestRegistrationWithAnAddressChangedAfterTheCodeRecordsNothing() {
	appID := ts.registrationApp("changed", registrationFlow(ts.fixture.senderID, nil,
		promptNode("prompt_email_again", actionSubmitEmail, emailAttribute),
		taskNode("provisioning", "ProvisioningExecutor", ""), recordNode("")))
	username, address := ts.newUserDetails("registration_changed")
	changed := "changed." + address

	// A prompt does not ask again for an input the flow holds, so the new address comes with the code,
	// in the request the prompt runs in.
	step := ts.startRegistration(appID, username, address)
	done := ts.continueFlow(step, map[string]string{"otp": ts.fixture.awaitCode(ts.T(), address),
		emailAttribute: changed}, actionSubmitOTP)

	user, attributes := ts.createdUser(username)
	ts.Require().NotNil(user, "the registration must create the user")
	ts.Equal(changed, attributes[emailAttribute], "the user must be created with the address entered last")
	ts.requireStepRefused(done, errCodeVerificationRefused, "recording must fail for an address not proven")
	ts.Empty(verifiedAt(ts.T(), user.ID, emailAttribute), "an address the code was not sent to must not be verified")
}

// Recording before the flow has created its user fails and records nothing, and the pin is not kept for
// the user the flow creates afterwards.
func (ts *OtherFlowTypesTestSuite) TestRegistrationRecordingBeforeTheUserExistsRecordsNothing() {
	recordFailed := promptNode("record_failed", actionContinue)
	recordFailed["prompts"].([]map[string]interface{})[0]["action"].(map[string]interface{})["nextNode"] =
		"provisioning"
	record := recordNode("record_failed")
	record["onSuccess"] = "provisioning"
	appID := ts.registrationApp("early_record", registrationFlow(ts.fixture.senderID, nil,
		record, recordFailed, taskNode("provisioning", "ProvisioningExecutor", "")))
	username, address := ts.newUserDetails("registration_early_record")

	step := ts.startRegistration(appID, username, address)
	step = ts.continueFlow(step, map[string]string{"otp": ts.fixture.awaitCode(ts.T(), address)}, actionSubmitOTP)
	ts.Require().True(common.HasAction(step.Data.Actions, actionContinue),
		"recording must fail while the flow has no user: %+v", step)
	ts.Require().NotNil(step.Error, "the failure must be reported: %+v", step)
	ts.Equal(errCodeVerificationRefused, step.Error.Code, "recording must fail while the flow has no user")
	done := ts.continueFlow(step, nil, actionContinue)

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "precondition: the flow goes on to create the user: %+v",
		done)
	user, _ := ts.createdUser(username)
	ts.Require().NotNil(user, "precondition: the registration creates the user")
	ts.Empty(verifiedAt(ts.T(), user.ID, emailAttribute), "a record before the user existed must record nothing")
}

// Registration cannot pin before the user type is resolved, for an attribute the user type does not
// declare as a contact, or without a collected value.
func (ts *OtherFlowTypesTestSuite) TestRegistrationPreparationWithoutAContactToPinIsRefused() {
	for _, tc := range []struct {
		name, key string
		nodes     []map[string]interface{}
	}{
		{"before the user type", "untyped", linearNodes(append([]map[string]interface{}{
			promptNode("prompt_details", actionSubmitDetails, "username", emailAttribute, displayNameAttribute),
			prepareNode(emailAttribute),
			taskNode("user_type_resolver", "UserTypeResolver", ""),
		}, append(emailCodeNodes(ts.fixture.senderID), taskNode("provisioning", "ProvisioningExecutor", ""),
			recordNode(""))...)...)},
		{"not a contact", "not_contact", registrationFlow(ts.fixture.senderID, prepareNode(displayNameAttribute),
			taskNode("provisioning", "ProvisioningExecutor", ""), recordNode(""))},
		{"no value", "no_value", linearNodes(append([]map[string]interface{}{
			taskNode("user_type_resolver", "UserTypeResolver", ""),
			promptNode("prompt_details", actionSubmitDetails, "username", displayNameAttribute),
			prepareNode(emailAttribute),
		}, append(emailCodeNodes(ts.fixture.senderID), taskNode("provisioning", "ProvisioningExecutor", ""),
			recordNode(""))...)...)},
	} {
		ts.Run(tc.name, func() {
			ts.fixture.smtp.ClearEmails()
			appID := ts.registrationApp("refused_"+tc.key, tc.nodes)
			username, address := ts.newUserDetails("registration_" + tc.key)

			step := ts.startRegistration(appID, username, address)

			ts.requireStepRefused(step, errCodeVerificationRefused, "preparation must have no contact to pin")
			ts.fixture.requireNoEmail(ts.T(), "nothing may be sent")
		})
	}
}

// startOnboarding names the onboarding flow as the default until the test ends, starts it as the
// administrator and submits the invitee's address.
func (ts *OtherFlowTypesTestSuite) startOnboarding(address string) *common.FlowStep {
	ts.T().Helper()

	handle := "verification-onboarding-" + ts.fixture.suffix
	createFlow(ts.T(), testutils.Flow{Name: "Attribute Verification Onboarding", FlowType: "USER_ONBOARDING",
		Handle: handle, Nodes: onboardingFlow(ts.fixture.userType, ts.fixture.senderID)})
	useFlowConfigWith(ts.T(), "userOnboardingFlow", map[string]interface{}{"defaultHandle": handle})

	result := execute(ts.T(), adminToken(ts.T()), map[string]interface{}{"flowType": "USER_ONBOARDING"})
	ts.Require().Equal(http.StatusOK, result.StatusCode, "the administrator's start must be accepted: %s",
		result.Body)
	ts.Require().True(common.HasInput(result.Step.Data.Inputs, emailAttribute),
		"the flow must ask for the invitee's address: %s", result.Body)
	return ts.continueFlow(result.Step, map[string]string{emailAttribute: address}, actionSubmitEmail)
}

// finishOnboarding opens the invite link as the invitee and submits the username.
func (ts *OtherFlowTypesTestSuite) finishOnboarding(link, username string) *common.FlowStep {
	ts.T().Helper()

	step := openLink(ts.T(), link, testutils.ExtractQueryParam(link, "inviteToken"))
	ts.Require().True(common.HasInput(step.Data.Inputs, "username"),
		"the invitee must be asked for the details: %+v", step)
	return ts.continueFlow(step, map[string]string{"username": username}, actionSubmitDetails)
}

// Onboarding that pins the invited address, emails the invite to it and records after creating the
// user verifies the new user's email.
func (ts *OtherFlowTypesTestSuite) TestOnboardingByEmailedInviteVerifiesTheNewUsersEmail() {
	username, address := ts.newUserDetails("onboarding_emailed")
	started := time.Now()

	step := ts.startOnboarding(address)
	step = ts.continueFlow(step, nil, actionSendInvite)
	ts.Require().Equal(flowStatusIncomplete, step.FlowStatus, "the flow must wait for the invitee: %+v", step)
	link := ts.fixture.awaitEmailTo(ts.T(), address).ExtractRecoveryLink()
	ts.Require().NotEmpty(link, "the email must carry the invite link")
	done := ts.finishOnboarding(link, username)

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the onboarding must complete: %+v", done)
	user, attributes := ts.createdUser(username)
	ts.Require().NotNil(user, "the onboarding must create the user")
	ts.Equal(address, attributes[emailAttribute], "the user must have the invited address")
	requireVerifiedSince(ts.T(), user.ID, emailAttribute, started)
}

// An invite link returned to the administrator proves nothing about the address: the onboarding
// records nothing.
func (ts *OtherFlowTypesTestSuite) TestOnboardingByAReturnedLinkRecordsNothing() {
	username, address := ts.newUserDetails("onboarding_shared")

	step := ts.startOnboarding(address)
	step = ts.continueFlow(step, nil, actionShareInvite)
	link := step.Data.AdditionalData["inviteLink"]
	ts.Require().NotEmpty(link, "precondition: the administrator receives the invite link: %+v", step.Data)
	ts.fixture.requireNoEmail(ts.T(), "precondition: no invite is emailed")
	ts.finishOnboarding(link, username)

	user, attributes := ts.createdUser(username)
	ts.Require().NotNil(user, "precondition: the onboarding creates the user")
	ts.Equal(address, attributes[emailAttribute], "precondition: the user has the invited address")
	ts.Empty(verifiedAt(ts.T(), user.ID, emailAttribute), "a link nobody emailed must not verify the address")
}

// Preparation in a sign-in, sign-out or administration flow fails before anything is sent: none has a
// user it may verify, even once the user has signed in or with a subject among its inputs.
func (ts *OtherFlowTypesTestSuite) TestPreparationInFlowsWithoutAUserIsRefused() {
	user := ts.fixture.newUser(ts.T(), "sign_in_out")
	verification := func() []map[string]interface{} {
		return append([]map[string]interface{}{prepareNode(emailAttribute)},
			append(emailCodeNodes(ts.fixture.senderID), recordNode(""))...)
	}

	ts.Run("sign-in", func() {
		ts.fixture.smtp.ClearEmails()
		flowID := createFlow(ts.T(), testutils.Flow{Name: "Attribute Verification Sign-in Preparation",
			FlowType: "AUTHENTICATION", Handle: "verification-sign-in-prepare-" + ts.fixture.suffix,
			Nodes: linearNodes(append(append([]map[string]interface{}{
				taskNode("credentials_auth", "CredentialsAuthExecutor", "")}, verification()...),
				taskNode("auth_assert", "AuthAssertExecutor", ""))...)})
		appID := ts.createApp("sign_in", testutils.Application{AuthFlowID: flowID})

		step, err := common.InitiateAuthenticationFlow(appID, false,
			map[string]string{"username": user.username, "password": user.password}, "")
		ts.Require().NoError(err)

		ts.requireStepRefused(step, errCodeVerificationUnavailable, "a sign-in flow must have no user to verify")
		ts.fixture.requireNoEmail(ts.T(), "nothing may be sent")
	})

	ts.Run("sign-out", func() {
		ts.fixture.smtp.ClearEmails()
		flowID := createFlow(ts.T(), testutils.Flow{Name: "Attribute Verification Sign-out Preparation",
			FlowType: "SIGNOUT", Handle: "verification-sign-out-prepare-" + ts.fixture.suffix,
			Nodes: linearNodes(verification()...)})
		appID := ts.createApp("sign_out", testutils.Application{SignOutFlowID: flowID})

		step, err := common.InitiateFlowWithHandle(appID, "SIGNOUT", "", nil)
		ts.Require().NoError(err)

		ts.requireStepRefused(step, errCodeVerificationUnavailable, "a sign-out flow must have no user to verify")
		ts.fixture.requireNoEmail(ts.T(), "nothing may be sent")
	})

	ts.Run("administration", func() {
		ts.fixture.smtp.ClearEmails()
		handle := "verification-administration-prepare-" + ts.fixture.suffix
		createFlow(ts.T(), testutils.Flow{Name: "Attribute Verification Administration Preparation",
			FlowType: "ADMINISTRATION", Handle: handle, Nodes: linearNodes(verification()...)})

		flowID, err := testutils.GetFlowIDByHandle(handle, "ADMINISTRATION")
		ts.Require().NoError(err)
		status, body, err := testutils.ExecuteFlowRaw(nil, adminToken(ts.T()),
			map[string]interface{}{"flowId": flowID, "inputs": map[string]string{"subject": user.id}})
		ts.Require().NoError(err)
		ts.Require().Equal(http.StatusOK, status, "the administrator's start must be accepted: %s", body)
		var step common.FlowStep
		ts.Require().NoError(json.Unmarshal(body, &step))

		ts.requireStepRefused(&step, errCodeVerificationUnavailable,
			"an administration flow must have no user to verify, even with a subject input")
		ts.fixture.requireNoEmail(ts.T(), "nothing may be sent")
	})

	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "nothing may be recorded")
}

// linearNodes runs nodes in order between a start and an end node: each node without a successor goes
// to the next one.
func linearNodes(nodes ...map[string]interface{}) []map[string]interface{} {
	all := append([]map[string]interface{}{{"id": "start", "type": "START"}}, nodes...)
	all = append(all, map[string]interface{}{"id": "end", "type": "END"})
	for i, node := range all[:len(all)-1] {
		next := all[i+1]["id"]
		if node["type"] == "PROMPT" {
			if prompts, ok := node["prompts"].([]map[string]interface{}); ok {
				action := prompts[0]["action"].(map[string]interface{})
				if action["nextNode"] == nil {
					action["nextNode"] = next
				}
			} else if node["next"] == nil {
				node["next"] = next
			}
			continue
		}
		if node["onSuccess"] == nil {
			node["onSuccess"] = next
		}
	}
	return all
}

// taskNode runs executor in mode, or in its only mode when mode is empty.
func taskNode(id, executor, mode string) map[string]interface{} {
	exec := map[string]interface{}{"name": executor}
	if mode != "" {
		exec["mode"] = mode
	}
	return map[string]interface{}{"id": id, "type": "TASK_EXECUTION", "executor": exec}
}

// promptNode asks for the identifiers and continues through action.
func promptNode(id, action string, identifiers ...string) map[string]interface{} {
	inputs := make([]map[string]interface{}, 0, len(identifiers))
	for _, identifier := range identifiers {
		inputType := "TEXT_INPUT"
		switch identifier {
		case emailAttribute:
			inputType = "EMAIL_INPUT"
		case "otp":
			inputType = "OTP_INPUT"
		}
		inputs = append(inputs, map[string]interface{}{"ref": "input_" + identifier, "identifier": identifier,
			"type": inputType, "required": true})
	}
	return map[string]interface{}{
		"id":   id,
		"type": "PROMPT",
		"prompts": []map[string]interface{}{{
			"inputs": inputs,
			"action": map[string]interface{}{"ref": action},
		}},
	}
}

// prepareNode pins attribute, which its property names.
func prepareNode(attribute string) map[string]interface{} {
	node := taskNode("prepare_verification", "AttributeVerificationExecutor", "prepare")
	node["properties"] = map[string]interface{}{"attribute": attribute}
	return node
}

// recordNode records the pin, going to onFailure when recording fails and onFailure is not empty.
func recordNode(onFailure string) map[string]interface{} {
	node := taskNode("record_verification", "AttributeVerificationExecutor", "record")
	if onFailure != "" {
		node["onFailure"] = onFailure
	}
	return node
}

// emailCodeNodes generate a code, email it through senderID and check the code the user submits.
func emailCodeNodes(senderID string) []map[string]interface{} {
	send := taskNode("send_email", "EmailExecutor", "send")
	send["properties"] = map[string]interface{}{"senderId": senderID, "emailTemplate": "otp"}
	send["executor"].(map[string]interface{})["inputs"] = []map[string]interface{}{
		{"identifier": emailAttribute, "type": "EMAIL_INPUT", "required": true}}
	generate := taskNode("generate_otp", "OTPExecutor", "generate")
	generate["executor"].(map[string]interface{})["inputs"] = []map[string]interface{}{
		{"identifier": emailAttribute, "type": "EMAIL_INPUT", "required": true}}
	return []map[string]interface{}{
		generate,
		send,
		promptNode("prompt_otp", actionSubmitOTP, "otp"),
		taskNode("verify_otp", "OTPExecutor", "verify"),
	}
}

// identifyNodes ask for a username and identify its user.
func identifyNodes() []map[string]interface{} {
	identify := taskNode("identify_user", "IdentifyingExecutor", "identify")
	identify["executor"].(map[string]interface{})["inputs"] = []map[string]interface{}{
		{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true}}
	return []map[string]interface{}{promptNode("prompt_username", actionSubmitUsername, "username"), identify}
}

// recoveryCodeFlow identifies the user, pins their email and records it once they submit the code
// emailed to it.
func recoveryCodeFlow(senderID string) []map[string]interface{} {
	nodes := append(identifyNodes(), prepareNode(emailAttribute))
	nodes = append(nodes, emailCodeNodes(senderID)...)
	return linearNodes(append(nodes, recordNode(""))...)
}

// recoveryLinkFlow identifies the user, pins their email and records it once they open the link
// emailed to it.
func recoveryLinkFlow(senderID string) []map[string]interface{} {
	send := taskNode("send_link", "EmailExecutor", "send")
	send["properties"] = map[string]interface{}{"senderId": senderID, "emailTemplate": "password-recovery"}
	send["executor"].(map[string]interface{})["inputs"] = []map[string]interface{}{
		{"identifier": emailAttribute, "type": "EMAIL_INPUT", "required": true}}
	verify := taskNode("verify_link", "InviteExecutor", "verify")
	verify["executor"].(map[string]interface{})["inputs"] = []map[string]interface{}{
		{"ref": "input_invite_token", "identifier": "inviteToken", "type": "HIDDEN", "required": true}}
	nodes := append(identifyNodes(), prepareNode(emailAttribute), taskNode("generate_link", "InviteExecutor",
		"generate"), send, map[string]interface{}{"id": "link_sent", "type": "PROMPT", "next": "verify_link"},
		verify, recordNode(""))
	return linearNodes(nodes...)
}

// registrationFlow resolves the user type, collects the details, pins the email with prepare (or the
// email prepare node when prepare is nil), checks a code emailed to it and then runs after.
func registrationFlow(senderID string, prepare map[string]interface{},
	after ...map[string]interface{}) []map[string]interface{} {
	if prepare == nil {
		prepare = prepareNode(emailAttribute)
	}
	nodes := []map[string]interface{}{
		taskNode("user_type_resolver", "UserTypeResolver", ""),
		promptNode("prompt_details", actionSubmitDetails, "username", emailAttribute, displayNameAttribute),
		prepare,
	}
	nodes = append(nodes, emailCodeNodes(senderID)...)
	return linearNodes(append(nodes, after...)...)
}

// onboardingFlow is the invite branch of the shipped onboarding flow with the invited address pinned
// before the invite and recorded after the user is created: the administrator enters the address and
// either emails the invite or receives the link.
func onboardingFlow(userType, senderID string) []map[string]interface{} {
	resolver := taskNode("user_type_resolver", "UserTypeResolver", "")
	resolver["properties"] = map[string]interface{}{"allowedUserTypes": []string{userType}}
	send := taskNode("send_invite_email", "EmailExecutor", "send")
	send["properties"] = map[string]interface{}{"senderId": senderID, "emailTemplate": "user-invite"}
	send["executor"].(map[string]interface{})["inputs"] = []map[string]interface{}{
		{"identifier": emailAttribute, "type": "EMAIL_INPUT", "required": true}}
	send["onSuccess"] = "email_invite_status"
	generateEmail := taskNode("invite_generate_email", "InviteExecutor", "generate")
	generateEmail["onSuccess"] = "send_invite_email"
	generateManual := taskNode("invite_generate_manual", "InviteExecutor", "generate")
	generateManual["onSuccess"] = "invite_link_status"
	verify := taskNode("invite_verify", "InviteExecutor", "verify")
	verify["executor"].(map[string]interface{})["inputs"] = []map[string]interface{}{
		{"ref": "input_invite_token", "identifier": "inviteToken", "type": "HIDDEN", "required": true}}
	provisioning := taskNode("provisioning", "ProvisioningExecutor", "")
	provisioning["onSuccess"] = "record_verification"
	provisioning["onIncomplete"] = "prompt_user_details"
	details := promptNode("prompt_user_details", actionSubmitDetails, "username")
	details["prompts"].([]map[string]interface{})[0]["action"].(map[string]interface{})["nextNode"] = "provisioning"

	return linearNodes(
		resolver,
		promptNode("prompt_email", actionSubmitEmail, emailAttribute),
		prepareNode(emailAttribute),
		map[string]interface{}{"id": "prompt_invite_method", "type": "PROMPT", "prompts": []map[string]interface{}{
			{"action": map[string]interface{}{"ref": actionSendInvite, "nextNode": "invite_generate_email"}},
			{"action": map[string]interface{}{"ref": actionShareInvite, "nextNode": "invite_generate_manual"}},
		}},
		generateEmail, send,
		map[string]interface{}{"id": "email_invite_status", "type": "PROMPT", "next": "invite_verify"},
		generateManual,
		map[string]interface{}{"id": "invite_link_status", "type": "PROMPT", "next": "invite_verify"},
		verify, provisioning, details, recordNode(""),
	)
}
