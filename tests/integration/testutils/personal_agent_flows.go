// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import "fmt"

const (
	// PersonalAgentSignInAction submits the sign-in prompt of the login flow that
	// CreatePersonalAgentRegistrationFlows creates.
	PersonalAgentSignInAction = "action_sign_in"
	// RegisterUserAction and RegisterPersonalAgentAction are the two choices that start the registration flow.
	RegisterUserAction          = "action_register_user"
	RegisterPersonalAgentAction = "action_register_personal_agent"
	// PersonalAgentDetailsAction submits the agent details prompt.
	PersonalAgentDetailsAction = "action_agent_details"
)

// PersonalAgentRegistrationFlows are a registration flow that registers a user or a personal agent, and the
// login flow it calls to sign the owner in. An application binds both, so every flow it reaches is one it
// binds.
type PersonalAgentRegistrationFlows struct {
	LoginFlowID        string
	RegistrationFlowID string
}

// CreatePersonalAgentRegistrationFlows creates the login flow and the registration flow, with handles derived
// from prefix. The registration flow starts with a choice: sign a user up, or sign the owner in and register a
// personal agent for them.
func CreatePersonalAgentRegistrationFlows(prefix string) (*PersonalAgentRegistrationFlows, error) {
	loginFlowID, err := CreateFlow(Flow{
		Name:     "Personal Agent Owner Login " + prefix,
		FlowType: "AUTHENTICATION",
		Handle:   prefix + "-owner-login",
		Nodes:    personalAgentOwnerLoginNodes(PersonalAgentSignInAction),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create the login flow: %w", err)
	}
	registrationFlowID, err := CreateFlow(Flow{
		Name:     "Personal Agent Registration " + prefix,
		FlowType: "REGISTRATION",
		Handle:   prefix + "-registration",
		Nodes:    personalAgentRegistrationNodes(loginFlowID),
	})
	if err != nil {
		_ = DeleteFlow(loginFlowID)
		return nil, fmt.Errorf("failed to create the registration flow: %w", err)
	}
	return &PersonalAgentRegistrationFlows{LoginFlowID: loginFlowID, RegistrationFlowID: registrationFlowID}, nil
}

// Delete removes both flows, the registration flow first since it calls the login flow.
func (f *PersonalAgentRegistrationFlows) Delete() error {
	if err := DeleteFlow(f.RegistrationFlowID); err != nil {
		return err
	}
	return DeleteFlow(f.LoginFlowID)
}

// CreateUsernamePasswordLoginFlow creates a login flow that signs a user in with a username and password,
// submitted with the given action, and returns its ID. It has no link to a registration flow, so an
// application can bind it together with any registration flow.
func CreateUsernamePasswordLoginFlow(handle, signInAction string) (string, error) {
	return CreateFlow(Flow{
		Name:     "Username and Password Login " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes:    personalAgentOwnerLoginNodes(signInAction),
	})
}

// personalAgentOwnerLoginNodes signs a user in with a username and password, submitted with signInAction.
func personalAgentOwnerLoginNodes(signInAction string) []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"meta": map[string]interface{}{"components": []map[string]interface{}{
				{"type": "TEXT", "id": "heading_sign_in", "label": "Sign In", "variant": "HEADING_1"},
				{"type": "BLOCK", "id": "block_sign_in", "components": []map[string]interface{}{
					{"type": "TEXT_INPUT", "id": "input_username", "ref": "username", "label": "Username",
						"required": true},
					{"type": "PASSWORD_INPUT", "id": "input_password", "ref": "password", "label": "Password",
						"required": true},
					{"type": "ACTION", "id": signInAction, "label": "Sign In", "variant": "PRIMARY",
						"eventType": "SUBMIT"},
				}},
			}},
			"prompts": []map[string]interface{}{{
				"inputs": []map[string]interface{}{
					{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
				"action": map[string]interface{}{"ref": signInAction, "nextNode": "credentials_auth"},
			}},
		},
		{
			"id":           "credentials_auth",
			"type":         "TASK_EXECUTION",
			"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor"},
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
	}
}

// personalAgentRegistrationNodes starts with the choice between the two branches. The user branch resolves
// the user type, collects the credentials and provisions the user. The personal agent branch calls the login
// flow, resolves the bundled agent type, collects the details and provisions the agent, then shows its
// credentials.
func personalAgentRegistrationNodes(loginFlowID string) []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_registration_choice"},
		{
			"id":   "prompt_registration_choice",
			"type": "PROMPT",
			"meta": map[string]interface{}{"components": []map[string]interface{}{
				{"type": "TEXT", "id": "heading_registration_choice", "label": "Get Started",
					"variant": "HEADING_1"},
				{"type": "BLOCK", "id": "block_registration_choice", "components": []map[string]interface{}{
					{"type": "ACTION", "id": RegisterUserAction, "label": "Create an account",
						"variant": "PRIMARY", "eventType": "SUBMIT"},
					{"type": "ACTION", "id": RegisterPersonalAgentAction, "label": "Register a personal assistant",
						"variant": "SECONDARY", "eventType": "SUBMIT"},
				}},
			}},
			"prompts": []map[string]interface{}{
				{"action": map[string]interface{}{"ref": RegisterUserAction, "nextNode": "user_type_resolver"}},
				{"action": map[string]interface{}{"ref": RegisterPersonalAgentAction,
					"nextNode": "call_owner_login"}},
			},
		},
		{
			"id":        "user_type_resolver",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "UserTypeResolver"},
			"onSuccess": "prompt_signup_credentials",
		},
		{
			"id":   "prompt_signup_credentials",
			"type": "PROMPT",
			"meta": map[string]interface{}{"components": []map[string]interface{}{
				{"type": "BLOCK", "id": "block_signup", "components": []map[string]interface{}{
					{"type": "TEXT_INPUT", "id": "input_signup_username", "ref": "username", "label": "Username",
						"required": true},
					{"type": "PASSWORD_INPUT", "id": "input_signup_password", "ref": "password",
						"label": "Password", "required": true},
					{"type": "ACTION", "id": "action_signup", "label": "Sign Up", "variant": "PRIMARY",
						"eventType": "SUBMIT"},
				}},
			}},
			"prompts": []map[string]interface{}{{
				"inputs": []map[string]interface{}{
					{"ref": "input_signup_username", "identifier": "username", "type": "TEXT_INPUT",
						"required": true},
					{"ref": "input_signup_password", "identifier": "password", "type": "PASSWORD_INPUT",
						"required": true},
				},
				"action": map[string]interface{}{"ref": "action_signup", "nextNode": "user_provisioning"},
			}},
		},
		{
			"id":           "user_provisioning",
			"type":         "TASK_EXECUTION",
			"executor":     map[string]interface{}{"name": "ProvisioningExecutor"},
			"onSuccess":    "end",
			"onIncomplete": "prompt_signup_credentials",
		},
		{
			"id":        "call_owner_login",
			"type":      "CALL",
			"flow":      map[string]interface{}{"ref": loginFlowID},
			"onSuccess": "agent_type_resolution",
		},
		{
			"id":         "agent_type_resolution",
			"type":       "TASK_EXECUTION",
			"properties": map[string]interface{}{"allowedAgentTypes": []string{"default"}},
			"executor":   map[string]interface{}{"name": "AgentTypeResolver"},
			"onSuccess":  "agent_provisioning",
		},
		{
			"id":   "prompt_agent_details",
			"type": "PROMPT",
			"meta": map[string]interface{}{"components": []map[string]interface{}{
				{"type": "BLOCK", "id": "block_agent_details", "components": []map[string]interface{}{
					{"type": "DYNAMIC_INPUT_PLACEHOLDER", "id": "dynamic_agent_inputs"},
					{"type": "TEXT_INPUT", "id": "input_agent_redirect_uris", "ref": "redirectUris",
						"label": "Redirect URIs", "required": true},
					{"type": "TEXT_INPUT", "id": "input_agent_jwks_uri", "ref": "jwksUri", "label": "JWKS URI",
						"required": false},
					{"type": "TEXT_INPUT", "id": "input_agent_public_key", "ref": "publicKey",
						"label": "Public Key", "required": false},
					{"type": "ACTION", "id": PersonalAgentDetailsAction, "label": "Register Agent",
						"variant": "PRIMARY", "eventType": "SUBMIT"},
				}},
			}},
			"prompts": []map[string]interface{}{{
				"inputs": []map[string]interface{}{
					{"ref": "input_agent_redirect_uris", "identifier": "redirectUris", "type": "TEXT_INPUT",
						"required": true},
					{"ref": "input_agent_jwks_uri", "identifier": "jwksUri", "type": "TEXT_INPUT",
						"required": false},
					{"ref": "input_agent_public_key", "identifier": "publicKey", "type": "TEXT_INPUT",
						"required": false},
				},
				"action": map[string]interface{}{"ref": PersonalAgentDetailsAction, "nextNode": "agent_provisioning"},
			}},
		},
		{
			"id":           "agent_provisioning",
			"type":         "TASK_EXECUTION",
			"properties":   map[string]interface{}{"includeOptional": true},
			"executor":     map[string]interface{}{"name": "ProvisioningExecutor"},
			"onSuccess":    "agent_credentials",
			"onIncomplete": "prompt_agent_details",
			"onFailure":    "prompt_agent_details",
		},
		{
			"id":   "agent_credentials",
			"type": "PROMPT",
			"meta": map[string]interface{}{"components": []map[string]interface{}{
				{"type": "TEXT", "id": "agent_credentials_heading", "label": "Agent Registered",
					"variant": "HEADING_1"},
				{"type": "COPYABLE_TEXT", "id": "agent_credentials_agent_id", "label": "Agent ID",
					"source": "agentId"},
				{"type": "COPYABLE_TEXT", "id": "agent_credentials_client_id", "label": "Client ID",
					"source": "clientId"},
				{"type": "COPYABLE_TEXT", "id": "agent_credentials_client_secret", "label": "Client Secret",
					"source": "clientSecret", "masked": true},
			}},
			"message": "Agent registered successfully",
			"next":    "end",
		},
		{"id": "end", "type": "END"},
	}
}
