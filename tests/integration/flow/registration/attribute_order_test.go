// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

var attrOrderOU = testutils.OrganizationUnit{
	Handle:      "attribute-order-test-ou",
	Name:        "Attribute Order Test OU",
	Description: "Organization unit for attribute order testing",
}

// attrOrderUserType orders some attributes, leaves others unordered, and hides picture on create.
var attrOrderUserType = testutils.UserType{
	Handle:                "attribute-order-test-type",
	DisplayName:           "Attribute Order Test Type",
	AllowSelfRegistration: true,
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string", "required": true,
			"display": map[string]interface{}{"order": 2}},
		"email": map[string]interface{}{"type": "string", "required": true,
			"display": map[string]interface{}{"order": 0}},
		"given_name":  map[string]interface{}{"type": "string", "display": map[string]interface{}{"order": 1}},
		"family_name": map[string]interface{}{"type": "string", "required": true},
		"nickname":    map[string]interface{}{"type": "string"},
		"picture": map[string]interface{}{"type": "string",
			"display": map[string]interface{}{"onCreate": map[string]interface{}{"hidden": true}}},
	},
}

var attrOrderFlow = testutils.Flow{
	Name:     "Attribute Order Registration Flow",
	Handle:   "reg_flow_attr_order",
	FlowType: "REGISTRATION",
	Nodes: []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "resolve_user_type"},
		{
			"id":        "resolve_user_type",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "UserTypeResolver"},
			"onSuccess": "provisioning",
		},
		{
			"id":           "provisioning",
			"type":         "TASK_EXECUTION",
			"executor":     map[string]interface{}{"name": "ProvisioningExecutor"},
			"properties":   map[string]interface{}{"includeOptional": true},
			"onSuccess":    "end",
			"onIncomplete": "prompt_schema_attrs",
		},
		{
			"id":   "prompt_schema_attrs",
			"type": "PROMPT",
			"meta": map[string]interface{}{
				"components": []map[string]interface{}{
					{"type": "BLOCK", "id": "block_dynamic_user_inputs", "components": []map[string]interface{}{
						{"type": "DYNAMIC_INPUT_PLACEHOLDER", "id": "dynamic_inputs"},
						{"type": "ACTION", "id": "action_schema_attrs", "label": "Continue", "variant": "PRIMARY",
							"eventType": "SUBMIT"},
					}},
				},
			},
			"prompts": []map[string]interface{}{
				{"inputs": []map[string]interface{}{},
					"action": map[string]interface{}{"ref": "action_schema_attrs", "nextNode": "provisioning"}},
			},
		},
		{"id": "end", "type": "END"},
	},
}

type AttributeOrderTestSuite struct {
	suite.Suite
	ouID           string
	userTypeID     string
	flowID         string
	authFlowID     string
	appID          string
	createdUserIDs []string
}

func TestAttributeOrderTestSuite(t *testing.T) {
	suite.Run(t, new(AttributeOrderTestSuite))
}

func (ts *AttributeOrderTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(attrOrderOU)
	ts.Require().NoError(err, "Failed to create test OU")
	ts.ouID = ouID

	attrOrderUserType.OUID = ts.ouID
	userTypeID, err := testutils.CreateUserType(attrOrderUserType)
	ts.Require().NoError(err, "Failed to create user type")
	ts.userTypeID = userTypeID

	flowID, err := testutils.CreateFlow(attrOrderFlow)
	ts.Require().NoError(err, "Failed to create registration flow")
	ts.flowID = flowID

	authFlowID, err := testutils.CreateIsolatedAuthFlow("attr-order-isolated-auth")
	ts.Require().NoError(err, "Failed to create isolated auth flow")
	ts.authFlowID = authFlowID

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      ts.ouID,
		Name:                      "Attribute Order App",
		IsRegistrationFlowEnabled: true,
		RegistrationFlowID:        ts.flowID,
		ClientID:                  "attr_order_client",
		ClientSecret:              "attr_order_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{attrOrderUserType.Handle},
		AuthFlowID:                ts.authFlowID,
	})
	ts.Require().NoError(err, "Failed to create test app")
	ts.appID = appID
}

func (ts *AttributeOrderTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.createdUserIDs); err != nil {
		ts.T().Logf("Failed to clean up users: %v", err)
	}
	if err := testutils.DeleteApplication(ts.appID); err != nil {
		ts.T().Logf("Failed to delete app: %v", err)
	}
	for _, flowID := range []string{ts.flowID, ts.authFlowID} {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow %s: %v", flowID, err)
		}
	}
	if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
		ts.T().Logf("Failed to delete user type: %v", err)
	}
	if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
		ts.T().Logf("Failed to delete OU: %v", err)
	}
}

// Ordered attributes come first, lowest first, then the rest, and picture isn't asked for.
func (ts *AttributeOrderTestSuite) TestDynamicInputsFollowDisplayOrderAndPromptOnCreate() {
	expected := []string{"email", "given_name", "username", "family_name", "nickname"}

	flowStep, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate registration flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected the schema attributes prompt")

	identifiers := make([]string, 0, len(flowStep.Data.Inputs))
	for _, input := range flowStep.Data.Inputs {
		identifiers = append(identifiers, input.Identifier)
	}
	ts.Require().Equal(expected, identifiers)

	username := common.GenerateUniqueUsername("orderuser")
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"email":       username + "@example.com",
		"given_name":  "Order",
		"username":    username,
		"family_name": "User",
		"nickname":    "ordy",
	}, "action_schema_attrs", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the schema attributes")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Registration must complete without picture")

	user, err := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(err, "Failed to find the registered user")
	ts.Require().NotNil(user, "Expected the registered user")
	ts.createdUserIDs = append(ts.createdUserIDs, user.ID)
}
