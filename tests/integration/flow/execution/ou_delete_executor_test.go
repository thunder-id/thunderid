// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

var (
	ouDeleteRollbackOU = testutils.OrganizationUnit{
		Handle:      "ou-delete-rollback-test-ou",
		Name:        "OU Delete Rollback Test Organization Unit",
		Description: "Parent organization unit for OUDeleteExecutor rollback testing",
		Parent:      nil,
	}

	// username and email are marked required so ProvisioningExecutor's HasRequiredInputs check
	// recognizes them as missing when credentials haven't been submitted yet — without that, it
	// treats zero required-and-missing fields as satisfied and fails outright on the empty
	// attribute set, rather than pausing at prompt_credentials for them.
	ouDeleteRollbackUserType = testutils.UserType{
		Name: "ou-delete-rollback-user-type",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type":     "string",
				"unique":   true,
				"required": true,
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"email": map[string]interface{}{
				"type":     "string",
				"unique":   true,
				"required": true,
			},
			"given_name": map[string]interface{}{
				"type": "string",
			},
			"family_name": map[string]interface{}{
				"type": "string",
			},
		},
	}

	// errCodeProvisioningAttributeConflict is ProvisioningExecutor's error code when a unique
	// attribute (email, here) conflicts with an existing user.
	errCodeProvisioningAttributeConflict = "FET-1080"
)

// ouDeleteRollbackPrefixNodes returns the node chain shared by every OUDeleteExecutor rollback
// scenario below: resolve the user type, create an OU, then attempt provisioning. Provisioning
// fails whenever the submitted email conflicts with an existing user (a deterministic,
// self-contained trigger: the conflicting user is seeded in SetupSuite, so no shared server state
// is relied upon). Every scenario's provisioning node routes onFailure straight to its own
// "ou_rollback" node — an OUDeleteExecutor is one of the executors exempt from the general
// "onFailure must target a PROMPT node" rule, precisely so a flow can wire rollback directly
// without an intervening prompt.
func ouDeleteRollbackPrefixNodes() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "user_type_resolver",
		},
		{
			"id":   "user_type_resolver",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "UserTypeResolver",
			},
			"onSuccess":    "ou_creation",
			"onIncomplete": "prompt_usertype",
		},
		{
			"id":   "prompt_usertype",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "usertype_input",
							"identifier": "userType",
							"type":       "SELECT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_usertype",
						"nextNode": "user_type_resolver",
					},
				},
			},
		},
		{
			"id":   "ou_creation",
			"type": "TASK_EXECUTION",
			"properties": map[string]interface{}{
				"parentOuId": "",
			},
			"executor": map[string]interface{}{
				"name": "OUExecutor",
			},
			"onSuccess":    "provisioning",
			"onIncomplete": "prompt_ou_details",
		},
		{
			"id":   "prompt_ou_details",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_ou_name",
							"identifier": "ouName",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_ou_handle",
							"identifier": "ouHandle",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_ou_submit",
						"nextNode": "ou_creation",
					},
				},
			},
		},
		{
			"id":   "provisioning",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "ProvisioningExecutor",
			},
			"onSuccess":    "end",
			"onFailure":    "ou_rollback",
			"onIncomplete": "prompt_credentials",
		},
		{
			"id":   "prompt_credentials",
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
						{
							"ref":        "input_email",
							"identifier": "email",
							"type":       "EMAIL_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_password",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_given_name",
							"identifier": "given_name",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_family_name",
							"identifier": "family_name",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_submit_credentials",
						"nextNode": "provisioning",
					},
				},
			},
		},
	}
}

var (
	// ouDeleteSilentRollbackFlow routes provisioning's failure directly into OUDeleteExecutor with
	// no PROMPT node in between, so the whole failure-and-rollback sequence completes within a
	// single request/response. Nothing ever reads the forwarded failure reason back out in this
	// shape, so the caller never learns why registration failed — only that it did.
	ouDeleteSilentRollbackFlow = testutils.Flow{
		Name:     "OU Delete Executor Silent Rollback Test",
		FlowType: "REGISTRATION",
		Handle:   "ou_delete_executor_silent_rollback_test",
		Nodes: append(ouDeleteRollbackPrefixNodes(),
			map[string]interface{}{
				"id":   "ou_rollback",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "OUDeleteExecutor",
				},
				"onSuccess": "end",
			},
			map[string]interface{}{
				"id":   "end",
				"type": "END",
			},
		),
	}

	// ouDeleteRollbackThenErrorFlow routes OUDeleteExecutor's own onSuccess to an interactive PROMPT
	// node rather than straight to END. OUDeleteExecutor still runs immediately, within the same
	// request as the provisioning failure, but the flow then pauses at that PROMPT node — which,
	// being interactive, is one of the node types that surfaces the failure reason still sitting
	// unconsumed in runtime data. So by the time the caller sees the real error, the rollback has
	// already happened.
	ouDeleteRollbackThenErrorFlow = testutils.Flow{
		Name:     "OU Delete Executor Rollback Then Error Test",
		FlowType: "REGISTRATION",
		Handle:   "ou_delete_executor_rollback_then_error_test",
		Nodes: append(ouDeleteRollbackPrefixNodes(),
			map[string]interface{}{
				"id":   "ou_rollback",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "OUDeleteExecutor",
				},
				"onSuccess": "final_error_prompt",
			},
			map[string]interface{}{
				"id":   "final_error_prompt",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{},
						"action": map[string]interface{}{
							"ref":      "action_final_error_ok",
							"nextNode": "end",
						},
					},
				},
			},
			map[string]interface{}{
				"id":   "end",
				"type": "END",
			},
		),
	}
)

// findOrganizationUnitByHandle looks up an organization unit's ID by handle, since a failed
// registration never reaches the point of returning the created OU's ID any other way (unlike a
// successful one, whose ID is recoverable from the issued assertion's claims).
func findOrganizationUnitByHandle(handle string) (*testutils.OrganizationUnit, error) {
	req, err := http.NewRequest("GET", testutils.TestServerURL+"/organization-units", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	q := req.URL.Query()
	q.Set("filter", fmt.Sprintf("handle eq %q", handle))
	req.URL.RawQuery = q.Encode()

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status listing organization units: %d", resp.StatusCode)
	}

	var listResp struct {
		OrganizationUnits []testutils.OrganizationUnit `json:"organizationUnits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if len(listResp.OrganizationUnits) == 0 {
		return nil, fmt.Errorf("no organization unit found with handle %q", handle)
	}
	return &listResp.OrganizationUnits[0], nil
}

// OUDeleteExecutorTestSuite verifies that an OUDeleteExecutor node, wired into a flow's own
// onFailure branch, actually rolls back an organization unit created earlier in the same
// execution — the scenario described in the "OU created by OUExecutor cannot be rolled back"
// issue this executor fixes.
type OUDeleteExecutorTestSuite struct {
	suite.Suite
	config                 *common.TestSuiteConfig
	ouID                   string
	userTypeID             string
	silentAppID            string
	rollbackThenErrorAppID string
	authFlowID             string
	conflictUserID         string
	conflictEmail          string
}

func TestOUDeleteExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(OUDeleteExecutorTestSuite))
}

func (ts *OUDeleteExecutorTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}
	ts.conflictEmail = fmt.Sprintf("ou-delete-rollback-conflict-%d@example.com", time.Now().UnixNano())

	ouID, err := testutils.CreateOrganizationUnit(ouDeleteRollbackOU)
	ts.Require().NoError(err, "Failed to create test organization unit during setup")
	ts.ouID = ouID

	ouDeleteRollbackUserType.OUID = ts.ouID
	ouDeleteRollbackUserType.AllowSelfRegistration = true
	userTypeID, err := testutils.CreateUserType(ouDeleteRollbackUserType)
	ts.Require().NoError(err, "Failed to create user type during setup")
	ts.userTypeID = userTypeID

	// Seed the conflicting user directly, rather than through a full registration, so the
	// provisioning failure below is deterministic and self-contained within this test.
	conflictAttrs, err := json.Marshal(map[string]interface{}{
		"username":    fmt.Sprintf("ou-delete-rollback-conflict-%d", time.Now().UnixNano()),
		"password":    "ConflictPass123!",
		"email":       ts.conflictEmail,
		"given_name":  "Conflict",
		"family_name": "User",
	})
	ts.Require().NoError(err, "Failed to marshal conflict user attributes")
	conflictUserID, err := testutils.CreateUser(testutils.User{
		OUID:       ts.ouID,
		Type:       ouDeleteRollbackUserType.Name,
		Attributes: conflictAttrs,
	})
	ts.Require().NoError(err, "Failed to create conflicting user during setup")
	ts.conflictUserID = conflictUserID

	authFlowID, err := testutils.CreateIsolatedAuthFlow("ou-delete-rollback-isolated-auth")
	ts.Require().NoError(err, "Failed to create isolated auth flow")
	ts.authFlowID = authFlowID

	silentFlowID, err := testutils.CreateFlow(ouDeleteSilentRollbackFlow)
	ts.Require().NoError(err, "Failed to create OU delete silent rollback flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, silentFlowID)

	rollbackThenErrorFlowID, err := testutils.CreateFlow(ouDeleteRollbackThenErrorFlow)
	ts.Require().NoError(err, "Failed to create OU delete rollback-then-error flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, rollbackThenErrorFlowID)

	silentAppID, err := testutils.CreateApplication(testutils.Application{
		Name:                      "OU Delete Executor Silent Rollback Test Application",
		Description:               "Application for testing OUDeleteExecutor's silent rollback",
		OUID:                      ts.ouID,
		IsRegistrationFlowEnabled: true,
		AuthFlowID:                ts.authFlowID,
		RegistrationFlowID:        silentFlowID,
		ClientID:                  "ou_delete_silent_rollback_test_client",
		ClientSecret:              "ou_delete_silent_rollback_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{ouDeleteRollbackUserType.Name},
	})
	ts.Require().NoError(err, "Failed to create silent rollback test application during setup")
	ts.silentAppID = silentAppID

	rollbackThenErrorAppID, err := testutils.CreateApplication(testutils.Application{
		Name:                      "OU Delete Executor Rollback Then Error Test Application",
		Description:               "Application for testing OUDeleteExecutor's rollback-then-error",
		OUID:                      ts.ouID,
		IsRegistrationFlowEnabled: true,
		AuthFlowID:                ts.authFlowID,
		RegistrationFlowID:        rollbackThenErrorFlowID,
		ClientID:                  "ou_delete_rollback_then_error_test_client",
		ClientSecret:              "ou_delete_rollback_then_error_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{ouDeleteRollbackUserType.Name},
	})
	ts.Require().NoError(err, "Failed to create rollback-then-error test application during setup")
	ts.rollbackThenErrorAppID = rollbackThenErrorAppID
}

func (ts *OUDeleteExecutorTestSuite) TearDownSuite() {
	if ts.silentAppID != "" {
		if err := testutils.DeleteApplication(ts.silentAppID); err != nil {
			ts.T().Logf("Failed to delete silent rollback test application during teardown: %v", err)
		}
	}
	if ts.rollbackThenErrorAppID != "" {
		if err := testutils.DeleteApplication(ts.rollbackThenErrorAppID); err != nil {
			ts.T().Logf("Failed to delete rollback-then-error test application during teardown: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}
	if ts.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.authFlowID); err != nil {
			ts.T().Logf("Failed to delete isolated auth flow during teardown: %v", err)
		}
	}
	if ts.conflictUserID != "" {
		if err := testutils.CleanupUsers([]string{ts.conflictUserID}); err != nil {
			ts.T().Logf("Failed to delete conflicting user during teardown: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// TestProvisioningFailureRollsBackSilently reproduces the original bug scenario using the silent
// shape: an OU is created successfully, provisioning fails terminally on a duplicate attribute,
// and OUDeleteExecutor (wired directly into provisioning's onFailure, with no PROMPT in between)
// deletes the OU that would otherwise be orphaned — all within a single request/response, since
// there is no PROMPT node forcing a pause. The tradeoff this proves: the caller is never told why
// registration failed.
func (ts *OUDeleteExecutorTestSuite) TestProvisioningFailureRollsBackSilently() {
	ouHandle := fmt.Sprintf("ou-delete-silent-%d", time.Now().UnixNano())
	username := common.GenerateUniqueUsername("oudeletesilent")

	// Submit only the user type and OU details first, deliberately withholding credentials, so the
	// flow pauses at prompt_credentials (provisioning's onIncomplete target) before provisioning
	// ever runs. Submitting credentials in the same call as everything else would let provisioning
	// fail and OUDeleteExecutor roll back within that single request/response — with no PROMPT node
	// in between to pause at, there would be no way to observe the OU while it still exists.
	flowStep, err := common.InitiateRegistrationFlow(ts.silentAppID, false, map[string]string{
		"userType": ouDeleteRollbackUserType.Name,
		"ouName":   "OU Delete Silent Rollback Test Child",
		"ouHandle": ouHandle,
	}, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"the flow should pause at prompt_credentials, before provisioning runs")

	// The OU must have been created before provisioning ever ran.
	createdOU, findErr := findOrganizationUnitByHandle(ouHandle)
	ts.Require().NoError(findErr, "OU should have been created before provisioning failed")
	ts.Require().Equal(ouHandle, createdOU.Handle)

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username":    username,
		"email":       ts.conflictEmail, // deliberately conflicts with the seeded user
		"password":    "RollbackTest123!",
		"given_name":  "Rollback",
		"family_name": "Test",
	}, "action_submit_credentials", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	// provisioning's onFailure targets OUDeleteExecutor directly, so the whole failure-and-rollback
	// sequence resolves within this single response — there is no PROMPT node to pause at.
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"onFailure routes straight into OUDeleteExecutor with no PROMPT in between, so the flow "+
			"resolves within this same request")
	ts.Require().Empty(flowStep.Assertion, "a failed registration must not issue an assertion")
	ts.Require().Nil(flowStep.Error,
		"skipping every PROMPT node means nothing ever reads the forwarded failure reason back out")

	// Confirm provisioning genuinely failed on the conflict, rather than succeeding and creating a
	// second user with the same email, by checking no user was created with this attempt's username.
	conflictingUser, findUserErr := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(findUserErr)
	ts.Require().Nil(conflictingUser, "provisioning must not have created a user for the conflicting email")

	// The whole point of OUDeleteExecutor: the OU created above must now be gone.
	_, getErr := testutils.GetOrganizationUnit(createdOU.ID)
	ts.Require().Error(getErr, "OUDeleteExecutor should have deleted the organization unit")
}

// TestProvisioningFailureRollsBackThenShowsError reproduces the same bug scenario using the
// rollback-then-error shape: OUDeleteExecutor's own onSuccess targets an interactive PROMPT node
// instead of END directly. OUDeleteExecutor still runs immediately, within the same request as the
// provisioning failure, but the flow then pauses at that PROMPT node, which surfaces the failure
// reason still sitting unconsumed in runtime data. So by the time the caller sees the real error,
// the OU is already gone.
func (ts *OUDeleteExecutorTestSuite) TestProvisioningFailureRollsBackThenShowsError() {
	ouHandle := fmt.Sprintf("ou-delete-then-error-%d", time.Now().UnixNano())
	username := common.GenerateUniqueUsername("oudeletethenerror")

	// Submit only the user type and OU details first, deliberately withholding credentials, so the
	// flow pauses at prompt_credentials (provisioning's onIncomplete target) before provisioning
	// ever runs. Submitting credentials in the same call as everything else would let provisioning
	// fail and OUDeleteExecutor roll back before this test ever observes the OU while it still
	// exists — final_error_prompt would still show the error, but only after rollback already ran.
	flowStep, err := common.InitiateRegistrationFlow(ts.rollbackThenErrorAppID, false, map[string]string{
		"userType": ouDeleteRollbackUserType.Name,
		"ouName":   "OU Delete Rollback Then Error Test Child",
		"ouHandle": ouHandle,
	}, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"the flow should pause at prompt_credentials, before provisioning runs")

	// The OU must have been created before provisioning ever ran.
	createdOU, findErr := findOrganizationUnitByHandle(ouHandle)
	ts.Require().NoError(findErr, "OU should have been created before provisioning failed")
	ts.Require().Equal(ouHandle, createdOU.Handle)

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username":    username,
		"email":       ts.conflictEmail, // deliberately conflicts with the seeded user
		"password":    "RollbackTest123!",
		"given_name":  "Rollback",
		"family_name": "Test",
	}, "action_submit_credentials", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	// The flow pauses at final_error_prompt, an interactive PROMPT node, only to surface the real
	// failure reason — OUDeleteExecutor has already run, in this same request, before this point.
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"the flow pauses at the final interactive error prompt, after rollback has already run")
	ts.Require().Empty(flowStep.Assertion, "a failed registration must not issue an assertion")
	ts.Require().NotNil(flowStep.Error, "the interactive prompt after rollback surfaces the real failure reason")
	ts.Require().Equal(errCodeProvisioningAttributeConflict, flowStep.Error.Code)

	// Confirm provisioning genuinely failed on the conflict, rather than succeeding and creating a
	// second user with the same email, by checking no user was created with this attempt's username.
	conflictingUser, findUserErr := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(findUserErr)
	ts.Require().Nil(conflictingUser, "provisioning must not have created a user for the conflicting email")

	// The whole point of OUDeleteExecutor: the OU created above is already gone, before the error
	// was ever shown to the caller.
	_, getErr := testutils.GetOrganizationUnit(createdOU.ID)
	ts.Require().Error(getErr, "OUDeleteExecutor should have deleted the organization unit")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, nil, "action_final_error_ok", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"acknowledging the error prompt completes the flow via its own END node")
	ts.Require().Empty(flowStep.Assertion, "a failed registration must not issue an assertion")
}
