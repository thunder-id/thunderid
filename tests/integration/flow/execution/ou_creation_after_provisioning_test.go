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
	ouAfterProvisioningOU = testutils.OrganizationUnit{
		Handle:      "ou-after-provisioning-test-ou",
		Name:        "OU After Provisioning Test Organization Unit",
		Description: "Parent/default organization unit for the OU-after-Provisioning regression test",
		Parent:      nil,
	}

	ouAfterProvisioningUserType = testutils.UserType{
		Name: "ou-after-provisioning-user-type",
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
		},
	}

	// ouAfterProvisioningFlow reproduces issue #5276: a REGISTRATION flow that resolves the user
	// type, provisions the user (landing them in the user type's own default OU), then places an OU
	// Creation node afterward. Before the fix, OUExecutor would skip creation entirely the moment the
	// now-authenticated, just-provisioned user was detected as already having an entity reference.
	ouAfterProvisioningFlow = testutils.Flow{
		Name:     "OU Creation After Provisioning Test",
		FlowType: "REGISTRATION",
		Handle:   "ou_creation_after_provisioning_test",
		Nodes: []map[string]interface{}{
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
				"onSuccess":    "provisioning",
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
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
				},
				"onSuccess":    "ou_creation",
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
						},
						"action": map[string]interface{}{
							"ref":      "action_submit_credentials",
							"nextNode": "provisioning",
						},
					},
				},
			},
			{
				"id":   "ou_creation",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "OUExecutor",
					"inputs": []map[string]interface{}{
						{"ref": "input_ou_name", "identifier": "ouName", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_ou_handle", "identifier": "ouHandle", "type": "TEXT_INPUT", "required": true},
					},
				},
				"onSuccess":    "end",
				"onIncomplete": "prompt_ou_details",
			},
			{
				"id":   "prompt_ou_details",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_ouname_field",
								"identifier": "ouName",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
							{
								"ref":        "input_ouhandle_field",
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
				"id":   "end",
				"type": "END",
			},
		},
	}
)

// findChildOUByHandle looks up an organization unit by handle among a specific parent's children,
// via GET /organization-units/{parentID}/ous. The top-level GET /organization-units list only ever
// returns root-level OUs (PARENT_ID IS NULL, both in the DB-backed and file-based stores), so a
// child OU created under a parent is never visible there regardless of any filter — the parent's own
// child-listing endpoint is the only way to find it.
func findChildOUByHandle(parentID, handle string) (*testutils.OrganizationUnit, error) {
	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s/organization-units/%s/ous", testutils.TestServerURL, parentID), nil)
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
		return nil, fmt.Errorf("unexpected status listing child organization units: %d", resp.StatusCode)
	}

	var listResp struct {
		OrganizationUnits []testutils.OrganizationUnit `json:"organizationUnits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if len(listResp.OrganizationUnits) == 0 {
		return nil, fmt.Errorf("no child organization unit found with handle %q under parent %q", handle, parentID)
	}
	return &listResp.OrganizationUnits[0], nil
}

// OUCreationAfterProvisioningTestSuite verifies the fix for issue #5276: OUExecutor must not skip
// creating a new organization unit just because the current, already-authenticated user was
// provisioned earlier in the same execution, and doing so must not move that user out of their own
// default organization unit.
type OUCreationAfterProvisioningTestSuite struct {
	suite.Suite
	config            *common.TestSuiteConfig
	ouID              string
	userTypeID        string
	authFlowID        string
	flowID            string
	appID             string
	childOUID         string
	provisionedUserID string
}

func TestOUCreationAfterProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(OUCreationAfterProvisioningTestSuite))
}

func (ts *OUCreationAfterProvisioningTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(ouAfterProvisioningOU)
	ts.Require().NoError(err, "Failed to create test organization unit during setup")
	ts.ouID = ouID

	ouAfterProvisioningUserType.OUID = ts.ouID
	ouAfterProvisioningUserType.AllowSelfRegistration = true
	userTypeID, err := testutils.CreateUserType(ouAfterProvisioningUserType)
	ts.Require().NoError(err, "Failed to create user type during setup")
	ts.userTypeID = userTypeID

	authFlowID, err := testutils.CreateIsolatedAuthFlow("ou-after-provisioning-isolated-auth")
	ts.Require().NoError(err, "Failed to create isolated auth flow")
	ts.authFlowID = authFlowID

	flowID, err := testutils.CreateFlow(ouAfterProvisioningFlow)
	ts.Require().NoError(err, "Failed to create OU-after-Provisioning test flow")
	ts.flowID = flowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:                      "OU Creation After Provisioning Test Application",
		Description:               "Application for testing OUExecutor placed after ProvisioningExecutor",
		OUID:                      ts.ouID,
		IsRegistrationFlowEnabled: true,
		AuthFlowID:                ts.authFlowID,
		RegistrationFlowID:        flowID,
		ClientID:                  "ou_after_provisioning_test_client",
		ClientSecret:              "ou_after_provisioning_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{ouAfterProvisioningUserType.Name},
	})
	ts.Require().NoError(err, "Failed to create test application during setup")
	ts.appID = appID
}

func (ts *OUCreationAfterProvisioningTestSuite) TearDownSuite() {
	if ts.provisionedUserID != "" {
		if err := testutils.DeleteUser(ts.provisionedUserID); err != nil {
			ts.T().Logf("Failed to delete provisioned user during teardown: %v", err)
		}
	}
	if ts.childOUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.childOUID); err != nil {
			ts.T().Logf("Failed to delete child organization unit during teardown: %v", err)
		}
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
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

// TestOUCreationAfterProvisioningStillCreatesOU reproduces issue #5276 end to end: it provisions a
// user, asserts the flow still pauses for OU details afterward (proving OUExecutor did not silently
// skip), then asserts the new child OU actually exists under the expected parent and that the user
// remains in their own default OU rather than being moved into the new one.
func (ts *OUCreationAfterProvisioningTestSuite) TestOUCreationAfterProvisioningStillCreatesOU() {
	username := common.GenerateUniqueUsername("ouafterprovisioning")
	email := fmt.Sprintf("%s@example.com", username)
	ouHandle := fmt.Sprintf("ou-after-provisioning-child-%d", time.Now().UnixNano())

	flowStep, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"the flow should pause at prompt_credentials, since only one user type is allowed")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username": username,
		"email":    email,
		"password": "OuAfterProvisioning123!",
	}, "action_submit_credentials", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	// The whole point of this test: before the fix, OUExecutor would skip creation entirely here
	// (the now-authenticated, just-provisioned user already has an entity reference) and the flow
	// would jump straight to COMPLETE without ever asking for OU details.
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"OUExecutor must not skip creation just because the user was already provisioned in this "+
			"same execution; the flow should pause at prompt_ou_details")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"ouName":   "OU After Provisioning Child",
		"ouHandle": ouHandle,
	}, "action_ou_submit", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)

	// The new child OU must actually exist, under the expected parent.
	childOU, findErr := findChildOUByHandle(ts.ouID, ouHandle)
	ts.Require().NoError(findErr, "the child OU should have been created under the user's default OU")
	ts.Require().Equal(ouHandle, childOU.Handle)
	ts.childOUID = childOU.ID

	// The user must remain in their own default OU, not be moved into the new child OU.
	provisionedUser, findUserErr := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(findUserErr)
	ts.Require().NotNil(provisionedUser)
	ts.provisionedUserID = provisionedUser.ID
	ts.Require().Equal(ts.ouID, provisionedUser.OUID,
		"the user must stay in their own default OU rather than being moved into the new child OU")
}
