// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	passkeyRegRelyingPartyID   = "localhost"
	passkeyRegRelyingPartyName = "ThunderID Test"
	// passkeyRegOrigin is set as the application's only allowed passkey origin, which is what the
	// flow executor validates the attestation against. It must also be listed under
	// passkey.allowed_origins in the test deployment.yaml, since TestPasskeyRegistration_Success
	// authenticates with the enrolled credential through the direct passkey API, and that path
	// takes its allowed origins from server config.
	passkeyRegOrigin = "https://localhost:8095"
)

// buildPasskeyRegistrationFlow assembles a registration flow that provisions the user first, since
// the register_start mode needs a user in context, then enrols a passkey for them.
//
// registerStartProperties is merged into the register_start node, so tests can vary the relying
// party and authenticator configuration.
func buildPasskeyRegistrationFlow(
	name, handle string, registerStartProperties map[string]interface{},
) testutils.Flow {
	return testutils.Flow{
		Name:     name,
		FlowType: "REGISTRATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "user_type_resolver"},
			{
				// Registration flows are required to carry a UserTypeResolver.
				"id":           "user_type_resolver",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "UserTypeResolver"},
				"onSuccess":    "prompt_attributes",
				"onIncomplete": "prompt_usertype",
			},
			{
				"id":   "prompt_usertype",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "usertype_input", "identifier": "userType", "type": "SELECT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_usertype", "nextNode": "user_type_resolver"},
					},
				},
			},
			{
				"id":   "prompt_attributes",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
							{"ref": "input_email", "identifier": "email", "type": "TEXT_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_attributes", "nextNode": "provisioning"},
					},
				},
			},
			{
				"id":        "provisioning",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
				"onSuccess": "passkey_register_start",
			},
			{
				"id":         "passkey_register_start",
				"type":       "TASK_EXECUTION",
				"properties": registerStartProperties,
				"executor": map[string]interface{}{
					"name": "PasskeyAuthExecutor",
					"mode": "register_start",
				},
				"onSuccess": "prompt_attestation",
			},
			{
				"id":   "prompt_attestation",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_credential_id", "identifier": "credentialId", "type": "TEXT_INPUT", "required": true},
							{"ref": "input_client_data", "identifier": "clientDataJSON", "type": "TEXT_INPUT", "required": true},
							{"ref": "input_attestation", "identifier": "attestationObject", "type": "TEXT_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_attestation", "nextNode": "passkey_register_finish"},
					},
				},
			},
			{
				"id":   "passkey_register_finish",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "PasskeyAuthExecutor",
					"mode": "register_finish",
				},
				"onSuccess": "auth_assert",
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

var (
	passkeyRegTestOU = testutils.OrganizationUnit{
		Handle:      "passkey-registration-test-ou",
		Name:        "Passkey Registration Test OU",
		Description: "Organization unit for passkey registration flow tests",
	}

	passkeyRegEntityType = testutils.UserType{
		Handle:                "passkey_reg_user",
		DisplayName:           "Passkey Reg User",
		AllowSelfRegistration: true,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"email":    map[string]interface{}{"type": "string"},
		},
	}

	passkeyRegTestApp = testutils.Application{
		Name:                      "Passkey Registration Flow Test Application",
		Description:               "Application for testing passkey registration flows",
		IsRegistrationFlowEnabled: true,
		ClientID:                  "passkey_registration_flow_test_client",
		ClientSecret:              "passkey_registration_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"passkey_reg_user"},
		PasskeyAllowedOrigins:     []string{passkeyRegOrigin},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}
)

type PasskeyRegistrationTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig

	appID        string
	entityTypeID string

	baselineFlowID       string
	authenticatorSelFlow string
	attestationFlow      string
	defaultRPNameFlow    string
}

func TestPasskeyRegistrationTestSuite(t *testing.T) {
	suite.Run(t, new(PasskeyRegistrationTestSuite))
}

func (ts *PasskeyRegistrationTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(passkeyRegTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	passkeyRegTestOU.ID = ouID

	passkeyRegEntityType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(passkeyRegEntityType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = entityTypeID

	ts.baselineFlowID = ts.createFlow(buildPasskeyRegistrationFlow(
		"Passkey Registration Flow Test", "reg_flow_passkey_test",
		map[string]interface{}{
			"relyingPartyId":   passkeyRegRelyingPartyID,
			"relyingPartyName": passkeyRegRelyingPartyName,
		}))

	ts.authenticatorSelFlow = ts.createFlow(buildPasskeyRegistrationFlow(
		"Passkey Registration Authenticator Selection Test", "reg_flow_passkey_authsel_test",
		map[string]interface{}{
			"relyingPartyId":   passkeyRegRelyingPartyID,
			"relyingPartyName": passkeyRegRelyingPartyName,
			"authenticatorSelection": map[string]interface{}{
				"residentKey":      "required",
				"userVerification": "required",
			},
		}))

	ts.attestationFlow = ts.createFlow(buildPasskeyRegistrationFlow(
		"Passkey Registration Attestation Test", "reg_flow_passkey_attestation_test",
		map[string]interface{}{
			"relyingPartyId":   passkeyRegRelyingPartyID,
			"relyingPartyName": passkeyRegRelyingPartyName,
			"attestation":      "direct",
		}))

	// No relyingPartyName, so the relying party ID should be used in its place.
	ts.defaultRPNameFlow = ts.createFlow(buildPasskeyRegistrationFlow(
		"Passkey Registration Default RP Name Test", "reg_flow_passkey_default_rpname_test",
		map[string]interface{}{"relyingPartyId": passkeyRegRelyingPartyID}))

	// An isolated auth flow avoids the cross-type reference check, which rejects an application
	// whose authentication flow points at a different registration flow than the one configured.
	isolatedAuthFlowID, err := testutils.CreateIsolatedAuthFlow("passkey-registration-isolated-auth")
	ts.Require().NoError(err, "Failed to create isolated auth flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, isolatedAuthFlowID)

	passkeyRegTestApp.OUID = ouID
	passkeyRegTestApp.RegistrationFlowID = ts.baselineFlowID
	passkeyRegTestApp.AuthFlowID = isolatedAuthFlowID
	appID, err := testutils.CreateApplication(passkeyRegTestApp)
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID
}

func (ts *PasskeyRegistrationTestSuite) createFlow(flow testutils.Flow) string {
	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "Failed to create flow %s", flow.Handle)
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	return flowID
}

func (ts *PasskeyRegistrationTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users during teardown: %v", err)
	}

	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete application during teardown: %v", err)
		}
	}

	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow %s during teardown: %v", flowID, err)
		}
	}

	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}

	if passkeyRegTestOU.ID != "" {
		if err := testutils.DeleteOrganizationUnit(passkeyRegTestOU.ID); err != nil {
			ts.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}
}

// useFlow points the test application at the given registration flow.
func (ts *PasskeyRegistrationTestSuite) useFlow(flowID string) {
	ts.Require().NoError(common.UpdateAppConfig(ts.appID, "", flowID),
		"Failed to point the application at flow %s", flowID)
}

// startRegistration provisions a user and returns the step that carries the creation options.
func (ts *PasskeyRegistrationTestSuite) startRegistration(username string) *common.FlowStep {
	step, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate registration flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Expected flow status to be INCOMPLETE")

	step, err = common.CompleteFlow(step.ExecutionID, map[string]string{
		"username": username,
		"email":    username + "@example.com",
	}, "action_attributes", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit registration attributes")

	return step
}

// passkeyCreationOptions captures the fields of the credential creation options these tests assert
// on. It is the decoded form of the passkeyCreationOptions entry in a flow step's additional data.
type passkeyCreationOptions struct {
	Challenge    string `json:"challenge"`
	RelyingParty struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"rp"`
	User struct {
		ID string `json:"id"`
	} `json:"user"`
	AuthenticatorSelection struct {
		ResidentKey      string `json:"residentKey"`
		UserVerification string `json:"userVerification"`
	} `json:"authenticatorSelection"`
	Attestation string `json:"attestation"`
}

// creationOptions decodes the credential creation options the register_start node returns.
func (ts *PasskeyRegistrationTestSuite) creationOptions(step *common.FlowStep) passkeyCreationOptions {
	var options passkeyCreationOptions

	raw, ok := step.Data.AdditionalData["passkeyCreationOptions"]
	ts.Require().True(ok, "Flow step should carry passkey creation options")
	ts.Require().NoError(json.Unmarshal([]byte(raw), &options),
		"Failed to decode passkey creation options")
	ts.Require().NotEmpty(options.Challenge, "Challenge should not be empty")

	return options
}

// completeRegistration enrols a credential for the challenge in the given step and returns the
// final flow step along with the authenticator that now holds the credential.
func (ts *PasskeyRegistrationTestSuite) completeRegistration(
	step *common.FlowStep, challenge string,
) (*common.FlowStep, *testutils.VirtualAuthenticator) {
	// A fresh authenticator per registration, since each one owns a single credential ID and every
	// test in this suite provisions a new user.
	authenticator, err := testutils.NewVirtualAuthenticator(passkeyRegRelyingPartyID, passkeyRegOrigin)
	ts.Require().NoError(err, "Failed to create virtual authenticator")

	credentialID, clientDataJSON, attestationObject, err := authenticator.CreateAttestationResponse(
		challenge, true)
	ts.Require().NoError(err, "Failed to build attestation response")

	finalStep, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		"credentialId":      credentialID,
		"clientDataJSON":    clientDataJSON,
		"attestationObject": attestationObject,
	}, "action_attestation", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the attestation")

	return finalStep, authenticator
}

// TestPasskeyRegistration_Success registers a user and a passkey in one flow, then confirms the
// credential works by authenticating with it. Stored credentials are not exposed by any API, so
// using the credential is the only way to prove registration persisted it correctly.
func (ts *PasskeyRegistrationTestSuite) TestPasskeyRegistration_Success() {
	ts.useFlow(ts.baselineFlowID)

	step := ts.startRegistration("passkeyreguser")
	options := ts.creationOptions(step)
	ts.Equal(passkeyRegRelyingPartyID, options.RelyingParty.ID, "Creation options should carry the relying party ID")
	ts.Equal(passkeyRegRelyingPartyName, options.RelyingParty.Name, "Creation options should carry the relying party name")

	finalStep, authenticator := ts.completeRegistration(step, options.Challenge)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().Nil(finalStep.Error, "Error should be nil for a successful registration")
	ts.Require().NotEmpty(finalStep.Assertion, "A JWT assertion should be returned")

	claims, err := testutils.ValidateJWTAssertionFields(finalStep.Assertion, ts.appID,
		passkeyRegEntityType.Handle, passkeyRegTestOU.ID, passkeyRegTestOU.Name, passkeyRegTestOU.Handle)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(claims, "JWT claims should not be nil")

	// Track the provisioned user so teardown removes it.
	user, err := testutils.FindUserByAttribute("username", "passkeyreguser")
	ts.Require().NoError(err, "Failed to look up the registered user")
	ts.Require().NotNil(user, "The registration flow should have provisioned a user")
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)

	// Authenticate with the credential just enrolled. Stored credentials are not readable through
	// any API, so this is the only evidence that registration persisted a usable credential, and the
	// only check that registration and authentication agree on the stored format.
	authResponse, err := testutils.AuthenticateWithPasskey(
		user.ID, passkeyRegRelyingPartyID, options.User.ID, authenticator)
	ts.Require().NoError(err, "The credential enrolled during registration should authenticate")
	ts.Equal(user.ID, authResponse.ID, "Authentication should identify the registered user")
	ts.NotEmpty(authResponse.Assertion, "Authentication should issue an assertion")
}

// TestPasskeyRegistration_AuthenticatorSelectionProperty checks the node level authenticator
// selection is passed through into the creation options the client receives.
func (ts *PasskeyRegistrationTestSuite) TestPasskeyRegistration_AuthenticatorSelectionProperty() {
	ts.useFlow(ts.authenticatorSelFlow)

	step := ts.startRegistration("passkeyregauthsel")
	options := ts.creationOptions(step)

	ts.Equal("required", options.AuthenticatorSelection.ResidentKey,
		"Configured resident key requirement should reach the client")
	ts.Equal("required", options.AuthenticatorSelection.UserVerification,
		"Configured user verification requirement should reach the client")

	finalStep, _ := ts.completeRegistration(step, options.Challenge)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow status to be COMPLETE")

	user, err := testutils.FindUserByAttribute("username", "passkeyregauthsel")
	ts.Require().NoError(err, "Failed to look up the registered user")
	ts.Require().NotNil(user, "The registration flow should have provisioned a user")
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
}

// TestPasskeyRegistration_AttestationProperty checks the node level attestation preference is
// passed through into the creation options.
func (ts *PasskeyRegistrationTestSuite) TestPasskeyRegistration_AttestationProperty() {
	ts.useFlow(ts.attestationFlow)

	step := ts.startRegistration("passkeyregattest")
	options := ts.creationOptions(step)

	ts.Equal("direct", options.Attestation, "Configured attestation preference should reach the client")

	finalStep, _ := ts.completeRegistration(step, options.Challenge)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow status to be COMPLETE")

	user, err := testutils.FindUserByAttribute("username", "passkeyregattest")
	ts.Require().NoError(err, "Failed to look up the registered user")
	ts.Require().NotNil(user, "The registration flow should have provisioned a user")
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
}

// TestPasskeyRegistration_RelyingPartyNameDefault checks that omitting the relying party name falls
// back to the relying party ID.
func (ts *PasskeyRegistrationTestSuite) TestPasskeyRegistration_RelyingPartyNameDefault() {
	ts.useFlow(ts.defaultRPNameFlow)

	step := ts.startRegistration("passkeyregdefaultrp")
	options := ts.creationOptions(step)

	ts.Equal(passkeyRegRelyingPartyID, options.RelyingParty.Name,
		"Relying party name should fall back to the relying party ID")

	finalStep, _ := ts.completeRegistration(step, options.Challenge)
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow status to be COMPLETE")

	user, err := testutils.FindUserByAttribute("username", "passkeyregdefaultrp")
	ts.Require().NoError(err, "Failed to look up the registered user")
	ts.Require().NotNil(user, "The registration flow should have provisioned a user")
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
}
