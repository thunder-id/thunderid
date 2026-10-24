// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The actions the prompts in this package's flows submit.
const (
	signInAction           = "action_sign_in"
	submitUsernameAction   = "action_submit_username"
	submitPasswordAction   = "action_submit_password"
	submitOTPAction        = "action_submit_otp"
	passkeyAssertionAction = "action_assertion"
)

// The inputs this package's flows ask for.
var (
	usernameInput    = flowInput("input_username", "username", "TEXT_INPUT")
	passwordInput    = flowInput("input_password", "password", "PASSWORD_INPUT")
	newPasswordInput = flowInput("input_new_password", "password", "PASSWORD_INPUT")
	otpInput         = flowInput("input_otp", "otp", "OTP_INPUT")
)

// flowInput is one required input a prompt asks for and an executor reads.
func flowInput(ref, identifier, inputType string) map[string]interface{} {
	return map[string]interface{}{"ref": ref, "identifier": identifier, "type": inputType, "required": true}
}

// startNode is the flow's start, moving to next.
func startNode(next string) map[string]interface{} {
	return map[string]interface{}{"id": "start", "type": "START", "onSuccess": next}
}

// endNode is the flow's end.
func endNode() map[string]interface{} {
	return map[string]interface{}{"id": "end", "type": "END"}
}

// promptNode asks for inputs and moves to next on action.
func promptNode(id, action, next string, inputs ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":   id,
		"type": "PROMPT",
		"prompts": []map[string]interface{}{{
			"inputs": inputs,
			"action": map[string]interface{}{"ref": action, "nextNode": next},
		}},
	}
}

// taskNode runs an executor, in mode when one is given, with any extra node keys (properties,
// onIncomplete, onFailure), and moves to onSuccess.
func taskNode(id, executor, mode, onSuccess string, extra map[string]interface{}) map[string]interface{} {
	executorDef := map[string]interface{}{"name": executor}
	if mode != "" {
		executorDef["mode"] = mode
	}
	node := map[string]interface{}{"id": id, "type": "TASK_EXECUTION", "executor": executorDef, "onSuccess": onSuccess}
	for key, value := range extra {
		node[key] = value
	}
	return node
}

// withExecutorInputs gives a task node's executor the inputs it reads.
func withExecutorInputs(node map[string]interface{}, inputs ...map[string]interface{}) map[string]interface{} {
	node["executor"].(map[string]interface{})["inputs"] = inputs
	return node
}

// usernamePrompt asks for the username and moves to the identify step.
func usernamePrompt() map[string]interface{} {
	return promptNode("prompt_username", submitUsernameAction, "identify_user", usernameInput)
}

// identifyNode identifies the account by username and moves to next.
func identifyNode(next string) map[string]interface{} {
	return withExecutorInputs(taskNode("identify_user", "IdentifyingExecutor", "identify", next, nil), usernameInput)
}

// newPasswordPrompt asks for a new password and moves to the step that writes it.
func newPasswordPrompt() map[string]interface{} {
	return promptNode("prompt_new_password", submitPasswordAction, "set_credential", newPasswordInput)
}

// setCredentialNode writes the new password and moves to the release.
func setCredentialNode() map[string]interface{} {
	return taskNode("set_credential", "CredentialSetter", "", "release_account_locks", nil)
}

// releaseNode releases the identified account's locks and moves to next.
func releaseNode(next string) map[string]interface{} {
	return taskNode("release_account_locks", "IdentityGovernanceExecutor", "unlock", next, nil)
}

// recoveryCompleteNode is the closing prompt a recovery flow ends on.
func recoveryCompleteNode() map[string]interface{} {
	return map[string]interface{}{"id": "recovery_complete", "type": "PROMPT", "next": "end"}
}

// smsSendNode sends the generated code through senderID.
func smsSendNode(senderID, onSuccess string, extra map[string]interface{}) map[string]interface{} {
	return taskNode("sms_send", "SMSExecutor", "", onSuccess, mergeJSONObjects(map[string]interface{}{
		"properties": map[string]interface{}{"senderId": senderID, "smsTemplate": "otp"},
	}, extra))
}

// governanceRecoveryFlow is the smallest recovery flow that writes a credential and then releases the
// ladder: identify, set, unlock. The unlock is its own node, as in the shipped flows. It has no
// challenge; otpReleaseRecoveryFlow is the code-proven one.
func governanceRecoveryFlow(name, handle string) testutils.Flow {
	return testutils.Flow{
		Name:     name,
		Handle:   handle,
		FlowType: "RECOVERY",
		Nodes: []map[string]interface{}{
			startNode("prompt_username"),
			usernamePrompt(),
			identifyNode("prompt_new_password"),
			newPasswordPrompt(),
			setCredentialNode(),
			releaseNode("recovery_complete"),
			recoveryCompleteNode(),
			endNode(),
		},
	}
}

// recoveryFlowVariant copies governanceRecoveryFlow under a new name and handle, applying edit to each
// node; edit returns false to drop the node.
func recoveryFlowVariant(name, handle string, edit func(node map[string]interface{}) bool) testutils.Flow {
	flow := governanceRecoveryFlow(name, handle)
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

// recoveryFlowWithoutRelease is governanceRecoveryFlow with the release node taken out.
func recoveryFlowWithoutRelease() testutils.Flow {
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

// recoveryFlowWithoutProof is governanceRecoveryFlow with nothing between identify and the release, so
// the release runs before anything proved control.
func recoveryFlowWithoutProof() testutils.Flow {
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

// otpReleaseRecoveryFlow is a recovery flow proven by an SMS code: identify, send, verify, set,
// release.
func otpReleaseRecoveryFlow(senderID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Governance OTP Recovery Release Test Flow",
		Handle:   "governance-otp-recovery-release-test",
		FlowType: "RECOVERY",
		Nodes: []map[string]interface{}{
			startNode("prompt_username"),
			usernamePrompt(),
			identifyNode("generate_otp"),
			withExecutorInputs(taskNode("generate_otp", "OTPExecutor", "generate", "sms_send", nil), usernameInput),
			smsSendNode(senderID, "prompt_otp", nil),
			promptNode("prompt_otp", submitOTPAction, "verify_otp", otpInput),
			taskNode("verify_otp", "OTPExecutor", "verify", "prompt_new_password",
				map[string]interface{}{"onIncomplete": "prompt_otp"}),
			newPasswordPrompt(),
			setCredentialNode(),
			releaseNode("recovery_complete"),
			recoveryCompleteNode(),
			endNode(),
		},
	}
}

// otpReleaseRecoveryFlowForACall is the SMS-code recovery flow ending at the release. The closing
// display prompt would end the whole execution, so a calling flow drops it.
func otpReleaseRecoveryFlowForACall(senderID string) testutils.Flow {
	flow := recoveryFlowVariantOf(otpReleaseRecoveryFlow(senderID), "for-call")
	original, _ := flow.Nodes.([]map[string]interface{})
	nodes := make([]map[string]interface{}, 0, len(original))
	for _, node := range original {
		if node["id"] == "recovery_complete" {
			continue
		}
		if node["id"] == "release_account_locks" {
			node["onSuccess"] = "end"
		}
		nodes = append(nodes, node)
	}
	flow.Nodes = nodes
	return flow
}

// otpDispatchRecoveryFlow is a recovery flow that dispatches a one-time password after identifying the
// account: identify, generate, send, prompt. It stops at the prompt, since a test of it is about the
// dispatch.
func otpDispatchRecoveryFlow(senderID string) testutils.Flow {
	toPrompt := map[string]interface{}{"onFailure": "otp_sent_status"}
	return testutils.Flow{
		Name:     "Governance Dispatch Recovery Test Flow",
		Handle:   "governance-dispatch-recovery-test",
		FlowType: "RECOVERY",
		Nodes: []map[string]interface{}{
			startNode("prompt_username"),
			usernamePrompt(),
			mergeJSONObjects(identifyNode("generate_otp"), toPrompt),
			taskNode("generate_otp", "OTPExecutor", "generate", "sms_send", toPrompt),
			smsSendNode(senderID, "otp_sent_status", toPrompt),
			promptNode("otp_sent_status", submitOTPAction, "end", otpInput),
			endNode(),
		},
	}
}

// passkeyRecoveryFlow is a recovery flow proven by a passkey: identify, challenge, verify, set,
// release.
func passkeyRecoveryFlow() testutils.Flow {
	assertionInputs := []map[string]interface{}{
		flowInput("input_credential_id", "credentialId", "TEXT_INPUT"),
		flowInput("input_client_data", "clientDataJSON", "TEXT_INPUT"),
		flowInput("input_auth_data", "authenticatorData", "TEXT_INPUT"),
		flowInput("input_signature", "signature", "TEXT_INPUT"),
		{"ref": "input_user_handle", "identifier": "userHandle", "type": "TEXT_INPUT", "required": false},
	}
	return testutils.Flow{
		Name:     "Governance Passkey Recovery Test Flow",
		Handle:   "governance-passkey-recovery-test",
		FlowType: "RECOVERY",
		Nodes: []map[string]interface{}{
			startNode("prompt_username"),
			usernamePrompt(),
			identifyNode("passkey_challenge"),
			taskNode("passkey_challenge", "PasskeyAuthExecutor", "challenge", "prompt_assertion",
				map[string]interface{}{"properties": map[string]interface{}{
					"relyingPartyId": recoveryPasskeyRPID, "relyingPartyName": recoveryPasskeyRPName,
				}}),
			promptNode("prompt_assertion", passkeyAssertionAction, "passkey_verify", assertionInputs...),
			taskNode("passkey_verify", "PasskeyAuthExecutor", "verify", "prompt_new_password", nil),
			newPasswordPrompt(),
			setCredentialNode(),
			releaseNode("recovery_complete"),
			recoveryCompleteNode(),
			endNode(),
		},
	}
}

// passwordSignInFlow is the shipped sign-in shape, of flowType: a credentials prompt the credentials
// node returns to when it is incomplete. An authentication flow asserts the account at the end.
func passwordSignInFlow(name, handle, flowType string) testutils.Flow {
	credentials := withExecutorInputs(taskNode("credentials_auth", "CredentialsAuthExecutor", "", "auth_assert",
		map[string]interface{}{"onIncomplete": "prompt_credentials"}), usernameInput, passwordInput)
	nodes := []map[string]interface{}{
		startNode("prompt_credentials"),
		promptNode("prompt_credentials", signInAction, "credentials_auth", usernameInput, passwordInput),
		credentials,
	}
	if flowType == "AUTHENTICATION" {
		nodes = append(nodes, taskNode("auth_assert", "AuthAssertExecutor", "", "end", nil))
	} else {
		credentials["onSuccess"] = "end"
	}
	return testutils.Flow{Name: name, Handle: handle, FlowType: flowType, Nodes: append(nodes, endNode())}
}

// signInCallingRecoveryFlow is a sign-in flow that calls a recovery flow and asserts whoever it returns
// with.
func signInCallingRecoveryFlow(name, recoveryFlowID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Governance Sign-in Calling Recovery Test Flow " + name,
		Handle:   "governance-signin-calling-recovery-" + strings.ReplaceAll(name, "_", "-"),
		FlowType: "AUTHENTICATION",
		Nodes: []map[string]interface{}{
			startNode("call_recovery"),
			{
				"id":        "call_recovery",
				"type":      "CALL",
				"flow":      map[string]interface{}{"ref": recoveryFlowID},
				"onSuccess": "auth_assert",
			},
			taskNode("auth_assert", "AuthAssertExecutor", "", "end", nil),
			endNode(),
		},
	}
}

// recoveryFlowVariantOf gives a fixture flow its own name and handle, so a test can create it again.
func recoveryFlowVariantOf(flow testutils.Flow, suffix string) testutils.Flow {
	flow.Name += " " + suffix
	flow.Handle += "-" + suffix
	return flow
}

// heldRecoveryFixture is a user fixture and an application whose recovery flow identifies the
// account, writes a credential and releases its locks, for suites that only need users and a way into
// recovery. It deletes itself when the test that built it ends.
type heldRecoveryFixture struct {
	*userFixture
	appID   string
	flowIDs []string
}

// newHeldRecoveryFixture creates the fixture's organization unit, user type, flows and application.
func newHeldRecoveryFixture(t *testing.T, name string) *heldRecoveryFixture {
	t.Helper()

	f := &heldRecoveryFixture{
		userFixture: newUserFixture(t, name, testutils.UserType{Schema: credentialsSchema(nil)}),
	}
	// Registered after the user fixture's, so it runs first and the application goes before its type.
	t.Cleanup(func() { f.tearDown(t) })

	recoveryFlowID, err := testutils.CreateFlow(governanceRecoveryFlow(
		"Governance Held Recovery Flow "+name, "governance-"+name+"-recovery"))
	require.NoError(t, err, "failed to create the recovery flow")
	f.flowIDs = append(f.flowIDs, recoveryFlowID)

	// The bootstrap authentication flow calls the default recovery flow, so the app carries its own.
	authFlowID, err := testutils.CreateFlow(passwordSignInFlow("Governance Held Sign-in Flow "+name,
		"governance-"+name+"-auth", "AUTHENTICATION"))
	require.NoError(t, err, "failed to create the sign-in flow")
	f.flowIDs = append(f.flowIDs, authFlowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                  f.ouID,
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

// tearDown deletes the application and its flows, logging rather than failing.
func (f *heldRecoveryFixture) tearDown(t *testing.T) {
	t.Helper()
	if f.appID != "" {
		if err := testutils.DeleteApplication(f.appID); err != nil {
			t.Logf("teardown: failed to delete application: %v", err)
		}
	}
	for _, flowID := range f.flowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			t.Logf("teardown: failed to delete flow %s: %v", flowID, err)
		}
	}
}

// submitRecoveryUsername starts the recovery flow and submits a username, returning the raw answer
// whatever its status, since a stated refusal and an unknown-user answer are the subject.
func (f *heldRecoveryFixture) submitRecoveryUsername(t *testing.T, username string) (int, string) {
	t.Helper()

	step, err := common.InitiateRecoveryFlow(f.appID, false, nil, "")
	require.NoError(t, err, "failed to initiate the recovery flow")
	require.Equal(t, "INCOMPLETE", step.FlowStatus)

	return postFlow(t, nil, "", map[string]interface{}{
		"executionId":    step.ExecutionID,
		"challengeToken": step.ChallengeToken,
		"inputs":         map[string]string{"username": username},
		"action":         submitUsernameAction,
	})
}

// recover drives the recovery flow to completion with a new password.
func (f *heldRecoveryFixture) recover(t *testing.T, username, newPassword string) {
	t.Helper()
	recoverThrough(t, f.appID, username, newPassword)
}
