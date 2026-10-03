// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Action refs of the prompts VerifiedLinkingFlow forwards a matched account through.
const (
	LinkAccountAction     = "link_account"
	SeparateAccountAction = "use_new_account"
	VerifyPasswordAction  = "verify_password"
)

// VerifyPasswordFlow is the verification flow VerifiedLinkingFlow calls: an ordinary username and
// password sign-in. It runs in a frame of its own, so it sees neither the linking candidates nor the
// claims the federated connection published, and asks who the End-User is like any other sign-in.
func VerifyPasswordFlow(handle string) testutils.Flow {
	usernameInput := map[string]interface{}{
		"identifier": "username", "type": "TEXT_INPUT", "required": true,
	}
	passwordInput := map[string]interface{}{
		"identifier": "password", "type": "PASSWORD_INPUT", "required": true,
	}
	return testutils.Flow{
		Name:     "Verify Password Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "credentials_prompt"},
			{
				"id":   "credentials_prompt",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{usernameInput, passwordInput},
						"action": map[string]interface{}{"ref": VerifyPasswordAction, "nextNode": "credentials_auth"},
					},
				},
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name":   "CredentialsAuthExecutor",
					"inputs": []map[string]interface{}{usernameInput, passwordInput},
				},
				"onSuccess":    "auth_assert",
				"onIncomplete": "credentials_prompt",
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

// VerifiedLinkingFlow is the authentication graph that links a federated identity to the local
// account its connection's account-linking attributes match. The linking node forwards the match to
// a prompt: the link action calls the verification flow, and whoever that flow authenticates is
// checked against the candidates when it returns to the linking node; the refusal points back at the
// linking node. extraProperties are merged into the federated node's properties.
func VerifiedLinkingFlow(handle, executorName, idpID, verifyFlowID string,
	extraProperties map[string]interface{}) testutils.Flow {
	federatedProperties := map[string]interface{}{"idpId": idpID}
	for key, value := range extraProperties {
		federatedProperties[key] = value
	}
	return testutils.Flow{
		Name:     "Verified Linking Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "federated_auth"},
			{
				"id":         "federated_auth",
				"type":       "TASK_EXECUTION",
				"properties": federatedProperties,
				"executor":   map[string]interface{}{"name": executorName},
				"onSuccess":  "linking",
			},
			{
				"id":           "linking",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "LinkingExecutor"},
				"onSuccess":    "auth_assert",
				"onIncomplete": "linking_prompt",
			},
			{
				"id":   "linking_prompt",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{"action": map[string]interface{}{
						"ref": LinkAccountAction, "type": "CONFIRM", "nextNode": "verify_account"}},
					{"action": map[string]interface{}{
						"ref": SeparateAccountAction, "type": "REJECT", "nextNode": "linking"}},
				},
			},
			{
				"id":        "verify_account",
				"type":      "CALL",
				"flow":      map[string]interface{}{"ref": verifyFlowID},
				"onSuccess": "linking",
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

// InFrameLinkingFlow is VerifiedLinkingFlow with verification run as steps in the same flow instead
// of a called flow: the link action leads to a username and password prompt, and the sign-in there
// returns to the linking node. The linking node clears the claims the connection published before
// verification and restores them after, so the prompt asks who the End-User is like any other
// sign-in.
func InFrameLinkingFlow(handle, executorName, idpID string) testutils.Flow {
	usernameInput := map[string]interface{}{
		"identifier": "username", "type": "TEXT_INPUT", "required": true,
	}
	passwordInput := map[string]interface{}{
		"identifier": "password", "type": "PASSWORD_INPUT", "required": true,
	}
	return testutils.Flow{
		Name:     "In-frame Linking Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "federated_auth"},
			{
				"id":         "federated_auth",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"idpId": idpID},
				"executor":   map[string]interface{}{"name": executorName},
				"onSuccess":  "linking",
			},
			{
				"id":           "linking",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "LinkingExecutor"},
				"onSuccess":    "auth_assert",
				"onIncomplete": "linking_prompt",
			},
			{
				"id":   "linking_prompt",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{"action": map[string]interface{}{
						"ref": LinkAccountAction, "type": "CONFIRM", "nextNode": "verify_prompt"}},
					{"action": map[string]interface{}{
						"ref": SeparateAccountAction, "type": "REJECT", "nextNode": "linking"}},
				},
			},
			{
				"id":   "verify_prompt",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{usernameInput, passwordInput},
						"action": map[string]interface{}{"ref": VerifyPasswordAction, "nextNode": "verify_auth"},
					},
				},
			},
			{
				"id":   "verify_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name":   "CredentialsAuthExecutor",
					"inputs": []map[string]interface{}{usernameInput, passwordInput},
				},
				"onSuccess":    "linking",
				"onIncomplete": "verify_prompt",
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

// LinkRequest names what LinkFederatedIdentity needs to record one link.
type LinkRequest struct {
	// Handle is unique per call, and names the throwaway flow and application.
	Handle       string
	ExecutorName string
	IDPID        string
	OUID         string
	UserType     string
	// Username and Password prove the local account the connection matches.
	Username string
	Password string
}

// LinkFederatedIdentity records the (connection, subject) link for the local account the
// connection's account-linking attributes match, by driving VerifiedLinkingFlow once through a
// throwaway application. A federated sign-in resolves a user only through a recorded link, and a
// verified linking flow is the only thing that records one for an account that already exists.
//
// The mock identity provider must already hold the identity it will return, and the connection must
// link on an attribute that identity shares with the account.
func LinkFederatedIdentity(req LinkRequest) error {
	verifyFlowID, err := testutils.CreateFlow(VerifyPasswordFlow(req.Handle + "-verify"))
	if err != nil {
		return fmt.Errorf("failed to create the verification flow: %w", err)
	}
	defer func() { _ = testutils.DeleteFlow(verifyFlowID) }()

	flowID, err := testutils.CreateFlow(
		VerifiedLinkingFlow(req.Handle, req.ExecutorName, req.IDPID, verifyFlowID, nil))
	if err != nil {
		return fmt.Errorf("failed to create the linking flow: %w", err)
	}
	defer func() { _ = testutils.DeleteFlow(flowID) }()

	regFlowID, err := testutils.CreateIsolatedRegistrationFlow(req.Handle + "-reg")
	if err != nil {
		return fmt.Errorf("failed to create the linking registration flow: %w", err)
	}
	defer func() { _ = testutils.DeleteFlow(regFlowID) }()

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:               "Linking App " + req.Handle,
		ClientID:           req.Handle,
		ClientSecret:       req.Handle + "-secret",
		RedirectURIs:       []string{"http://localhost:3000/callback"},
		AllowedUserTypes:   []string{req.UserType},
		OUID:               req.OUID,
		AuthFlowID:         flowID,
		RegistrationFlowID: regFlowID,
	})
	if err != nil {
		return fmt.Errorf("failed to create the linking application: %w", err)
	}
	defer func() { _ = testutils.DeleteApplication(appID) }()

	step, err := InitiateAuthenticationFlow(appID, false, nil, "")
	if err != nil {
		return fmt.Errorf("failed to initiate the linking flow: %w", err)
	}
	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	if err != nil {
		return fmt.Errorf("failed to simulate the federated authorization: %w", err)
	}

	step, err = CompleteFlow(step.ExecutionID, map[string]string{"code": code, "state": state}, "",
		step.ChallengeToken)
	if err != nil {
		return fmt.Errorf("failed to complete the federated authentication: %w", err)
	}
	if step.FlowStatus != "INCOMPLETE" || step.Data.AdditionalData["linkingPromptDetails"] == "" {
		return fmt.Errorf("expected the linking prompt for a matched account, got %+v", step)
	}

	step, err = CompleteFlow(step.ExecutionID, nil, LinkAccountAction, step.ChallengeToken)
	if err != nil {
		return fmt.Errorf("failed to confirm the match: %w", err)
	}
	step, err = CompleteFlow(step.ExecutionID, map[string]string{"username": req.Username, "password": req.Password},
		VerifyPasswordAction, step.ChallengeToken)
	if err != nil {
		return fmt.Errorf("failed to verify the account: %w", err)
	}
	if step.FlowStatus != "COMPLETE" {
		return fmt.Errorf("expected the verified account to be linked, got %+v", step)
	}
	return nil
}
