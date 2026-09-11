// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	crossOUFlowHandle = "cross_ou_provisioning_flow_test"

	errCodeUserAlreadyExistsInTargetOU = "FET-1024"
	errCodeProvisioningAttrConflict    = "FET-1080"
)

// CrossOUProvisioningTestSuite covers the provisioning executor with allowCrossOUProvisioning set.
// The user type marks both username and email unique, so a single onboarding attempt can collide
// with users sitting in different OUs and the executor has to decide which match blocks it.
type CrossOUProvisioningTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig

	rootOUID  string
	childOUID string
	typeID    string
	typeName  string
	flowID    string
}

func TestCrossOUProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(CrossOUProvisioningTestSuite))
}

// crossOUFlowNodes builds an onboarding flow whose provisioning node opts into cross-OU
// provisioning. The OU is prompted for first so a test can target an OU other than the one the
// user type is attached to.
func crossOUFlowNodes(allowedUserTypes []string) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "ou_resolver",
		},
		{
			"id":   "ou_resolver",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "OUResolverExecutor",
			},
			"properties": map[string]interface{}{
				"resolveFrom": "promptAll",
			},
			"onSuccess":    "user_type_resolver",
			"onIncomplete": "prompt_ou",
		},
		{
			"id":   "prompt_ou",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "ou_selection_input", "identifier": "ouId", "type": "OU_SELECT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_ou", "nextNode": "ou_resolver"},
				},
			},
		},
		{
			"id":   "user_type_resolver",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "UserTypeResolver",
			},
			"properties": map[string]interface{}{
				"allowedUserTypes": allowedUserTypes,
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
						{"ref": "usertype_input", "identifier": "userType", "type": "SELECT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_usertype", "nextNode": "user_type_resolver"},
				},
			},
		},
		{
			"id":   "provisioning",
			"type": "TASK_EXECUTION",
			"properties": map[string]interface{}{
				"allowCrossOUProvisioning": true,
			},
			"executor": map[string]interface{}{
				"name": "ProvisioningExecutor",
				"inputs": []map[string]interface{}{
					{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_email", "identifier": "email", "type": "TEXT_INPUT", "required": true},
				},
			},
			"onSuccess":    "end",
			"onIncomplete": "prompt_user_details",
		},
		{
			"id":   "prompt_user_details",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_email", "identifier": "email", "type": "TEXT_INPUT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_details", "nextNode": "provisioning"},
				},
			},
		},
		{
			"id":   "end",
			"type": "END",
		},
	}
}

func (ts *CrossOUProvisioningTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	rootOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "cross_ou_root_test_ou",
		Name:        "Cross OU Root Test OU",
		Description: "Root organization unit for cross-OU provisioning testing",
		Parent:      nil,
	})
	ts.Require().NoError(err, "Failed to create root organization unit")
	ts.rootOUID = rootOUID

	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "cross_ou_child_test_ou",
		Name:        "Cross OU Child Test OU",
		Description: "Child organization unit for cross-OU provisioning testing",
		Parent:      &rootOUID,
	})
	ts.Require().NoError(err, "Failed to create child organization unit")
	ts.childOUID = childOUID

	// Attached to the root OU so the type is valid for the child OU too, which is what lets one
	// flow provision into either OU.
	ts.typeName = "cross-ou-person"
	ts.typeID, err = testutils.CreateUserType(testutils.UserType{
		Name: ts.typeName,
		OUID: rootOUID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"email":    map[string]interface{}{"type": "string", "unique": true},
		},
	})
	ts.Require().NoError(err, "Failed to create user type")

	ts.flowID, err = testutils.CreateFlow(testutils.Flow{
		Name:     "Cross OU Provisioning Test Flow",
		FlowType: "USER_ONBOARDING",
		Handle:   crossOUFlowHandle,
		Nodes:    crossOUFlowNodes([]string{ts.typeName}),
	})
	ts.Require().NoError(err, "Failed to create cross-OU provisioning flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, ts.flowID)

	ts.setCrossOUDefaultHandle(crossOUFlowHandle)
}

func (ts *CrossOUProvisioningTestSuite) TearDownSuite() {
	for _, userID := range ts.config.CreatedUserIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("Failed to delete user during teardown: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}
	if ts.typeID != "" {
		if err := testutils.DeleteUserType(ts.typeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
	for _, ouID := range []string{ts.childOUID, ts.rootOUID} {
		if ouID == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// setCrossOUDefaultHandle points the writable flow section at this suite's onboarding flow and
// registers the restore on test cleanup, matching the user onboarding suite's approach.
func (ts *CrossOUProvisioningTestSuite) setCrossOUDefaultHandle(handle string) {
	ts.T().Helper()

	original, err := testutils.MergeWritableServerConfig(flowConfigSection, map[string]interface{}{
		"userOnboardingFlow": map[string]interface{}{"defaultHandle": handle},
	})
	ts.Require().NoError(err, "Failed to update flow server config")
	ts.T().Cleanup(func() {
		if err := testutils.PutWritableServerConfig(flowConfigSection, original); err != nil {
			ts.T().Errorf("cleanup: failed to restore flow server config: %v", err)
		}
	})
}

// seedUser creates a user directly through the users API so a flow attempt can collide with it.
func (ts *CrossOUProvisioningTestSuite) seedUser(ouID, username, email string) {
	ts.T().Helper()

	attrs, err := json.Marshal(map[string]string{"username": username, "email": email})
	ts.Require().NoError(err)

	userID, err := testutils.CreateUser(testutils.User{
		Type:       ts.typeName,
		OUID:       ouID,
		Attributes: attrs,
	})
	ts.Require().NoError(err, "Failed to seed user %s", username)
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, userID)
}

// onboardInto runs the flow up to the user details prompt for the given OU, then submits the
// supplied username and email and returns the resulting step.
func (ts *CrossOUProvisioningTestSuite) onboardInto(ouID, username, email string) *common.FlowStep {
	ts.T().Helper()

	reqBody, err := json.Marshal(map[string]interface{}{"flowType": "USER_ONBOARDING"})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to initiate cross-OU onboarding flow")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "Onboarding initiation should succeed: %s", string(body))

	var step common.FlowStep
	ts.Require().NoError(json.Unmarshal(body, &step))

	next, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"ouId": ouID}, "action_ou", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OU selection")

	final, err := common.CompleteFlow(next.ExecutionID,
		map[string]string{"username": username, "email": email}, "action_details", next.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit user details")
	return final
}

// A colliding unique value held by a user already in the target OU blocks provisioning, and is
// reported as recoverable so the details prompt is re-rendered rather than the flow being killed.
func (ts *CrossOUProvisioningTestSuite) TestCrossOU_MatchInTargetOU_Blocks() {
	ts.seedUser(ts.childOUID, "cross_ou_target_user", "cross_ou_target@example.com")

	step := ts.onboardInto(ts.childOUID, "cross_ou_target_user", "cross_ou_fresh_email@example.com")

	ts.Require().NotNil(step.Error, "A user already in the target OU must block provisioning")
	ts.Equal(errCodeUserAlreadyExistsInTargetOU, step.Error.Code,
		"The block must name the target OU conflict")
	ts.True(common.HasInput(step.Data.Inputs, "username"),
		"The details prompt must be offered again so the value can be corrected")
}

// The first probed unique value belongs to a user in another OU while a later one belongs to a user
// in the target OU. Cross-OU provisioning may continue past the former, so the latter is the match
// that has to win. Regression test for the foreign-OU match short-circuiting the search.
func (ts *CrossOUProvisioningTestSuite) TestCrossOU_TargetOUMatchOnLaterUniqueAttr_Wins() {
	ts.seedUser(ts.rootOUID, "cross_ou_elsewhere_user", "cross_ou_shared_email@example.com")
	ts.seedUser(ts.childOUID, "cross_ou_here_user", "cross_ou_here@example.com")

	// email is probed before username, so the foreign-OU user matches first.
	step := ts.onboardInto(ts.childOUID, "cross_ou_here_user", "cross_ou_shared_email@example.com")

	ts.Require().NotNil(step.Error, "The target OU match must block provisioning")
	ts.Equal(errCodeUserAlreadyExistsInTargetOU, step.Error.Code,
		"The target OU match must take precedence over the match in another OU")
}

// When every colliding value belongs to a user outside the target OU, cross-OU provisioning
// proceeds. The create then still fails, because attribute uniqueness is enforced across the whole
// deployment rather than per OU, so this records the boundary of the cross-OU mode.
func (ts *CrossOUProvisioningTestSuite) TestCrossOU_AllMatchesOutsideTargetOU_ProceedsThenConflicts() {
	ts.seedUser(ts.rootOUID, "cross_ou_only_elsewhere", "cross_ou_only_elsewhere@example.com")

	step := ts.onboardInto(ts.childOUID, "cross_ou_brand_new_user", "cross_ou_only_elsewhere@example.com")

	ts.Require().NotNil(step.Error, "The create must report the deployment-wide uniqueness conflict")
	ts.Equal(errCodeProvisioningAttrConflict, step.Error.Code,
		"Cross-OU provisioning proceeds past the foreign match and then hits the attribute conflict")
}
