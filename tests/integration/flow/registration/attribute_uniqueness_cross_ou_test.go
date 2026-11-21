// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// errCodeEntityExistsInTargetOU is raised when the email is already held in the organization unit
// the flow provisions into, which is what cross-OU provisioning scopes the email check to.
const errCodeEntityExistsInTargetOU = "FET-1024"

var crossOUSalesOU = testutils.OrganizationUnit{
	Handle:      "crossou-uniqueness-sales",
	Name:        "Cross OU Uniqueness Sales",
	Description: "Organization unit holding the seeded email for cross-OU uniqueness testing",
}

var crossOUMarketingOU = testutils.OrganizationUnit{
	Handle:      "crossou-uniqueness-marketing",
	Name:        "Cross OU Uniqueness Marketing",
	Description: "Second organization unit the same email may be reused in",
}

// crossOUSalesType keeps username unique but leaves email open, which is what cross-OU provisioning
// requires: one person holds the same address in more than one organization unit.
var crossOUSalesType = testutils.UserType{
	Handle:                "crossou-uniqueness-sales-type",
	DisplayName:           "Cross OU Uniqueness Sales Type",
	AllowSelfRegistration: true,
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string", "unique": true},
		"email":    map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

var crossOUMarketingType = testutils.UserType{
	Handle:                "crossou-uniqueness-marketing-type",
	DisplayName:           "Cross OU Uniqueness Marketing Type",
	AllowSelfRegistration: true,
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string", "unique": true},
		"email":    map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

type AttributeUniquenessCrossOUTestSuite struct {
	suite.Suite
	salesOUID       string
	marketingOUID   string
	salesTypeID     string
	marketingTypeID string
	salesFlowID     string
	marketingFlowID string
	authFlowID      string
	salesAppID      string
	marketingAppID  string
	seededEmail     string
	createdUserIDs  []string
}

func TestAttributeUniquenessCrossOUTestSuite(t *testing.T) {
	suite.Run(t, new(AttributeUniquenessCrossOUTestSuite))
}

// buildCrossOUUniquenessFlow mirrors buildUniquenessFlow but turns on allowCrossOUProvisioning, so
// the validator checks email against the organization unit the flow provisions into rather than the
// whole deployment. UserTypeResolver supplies that unit as the resolved user type own OU.
//
// placement, when not empty, is set as provisioningOU on the validator node. The validator resolves
// the target unit the way the provisioning executor places a new entity, so the two placements are
// exercised by giving the two flows different values.
func buildCrossOUUniquenessFlow(handle, placement string) testutils.Flow {
	validatorProperties := map[string]interface{}{"allowCrossOUProvisioning": true}
	if placement != "" {
		validatorProperties["provisioningOU"] = placement
	}
	return testutils.Flow{
		Name:     "Cross OU Attribute Uniqueness Registration Flow",
		FlowType: "REGISTRATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "resolve_user_type"},
			{
				"id":        "resolve_user_type",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "UserTypeResolver"},
				"onSuccess": "prompt_attributes",
			},
			{
				"id":   "prompt_attributes",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
							{"ref": "input_002", "identifier": "email", "type": "TEXT_INPUT", "required": false},
							{"ref": "input_003", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_001", "nextNode": "check_uniqueness"},
					},
				},
			},
			{
				"id":           "check_uniqueness",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "AttributeUniquenessValidator"},
				"properties":   validatorProperties,
				"onSuccess":    "provision_user",
				"onIncomplete": "prompt_attributes",
			},
			{
				"id":        "provision_user",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}

func (ts *AttributeUniquenessCrossOUTestSuite) SetupSuite() {
	salesOUID, err := testutils.CreateOrganizationUnit(crossOUSalesOU)
	ts.Require().NoError(err, "Failed to create the sales OU")
	ts.salesOUID = salesOUID

	marketingOUID, err := testutils.CreateOrganizationUnit(crossOUMarketingOU)
	ts.Require().NoError(err, "Failed to create the marketing OU")
	ts.marketingOUID = marketingOUID

	crossOUSalesType.OUID = ts.salesOUID
	salesTypeID, err := testutils.CreateUserType(crossOUSalesType)
	ts.Require().NoError(err, "Failed to create the sales user type")
	ts.salesTypeID = salesTypeID

	crossOUMarketingType.OUID = ts.marketingOUID
	marketingTypeID, err := testutils.CreateUserType(crossOUMarketingType)
	ts.Require().NoError(err, "Failed to create the marketing user type")
	ts.marketingTypeID = marketingTypeID

	// Seed the holder of the email in the sales OU. A later registration into that same OU must be
	// rejected, while the same address into marketing must be accepted.
	seedUsername := common.GenerateUniqueUsername("crossouseed")
	ts.seededEmail = seedUsername + "@example.com"
	userIDs, err := testutils.CreateMultipleUsers(testutils.User{
		OUID: ts.salesOUID,
		Type: crossOUSalesType.Handle,
		Attributes: json.RawMessage(`{
			"username": "` + seedUsername + `",
			"email":    "` + ts.seededEmail + `",
			"password": "Occupied123!"
		}`),
	})
	ts.Require().NoError(err, "Failed to seed the user holding the email in the sales OU")
	ts.createdUserIDs = append(ts.createdUserIDs, userIDs...)

	// The sales flow leaves the placement unset, so the validator takes the unit the flow resolved.
	salesFlowID, err := testutils.CreateFlow(buildCrossOUUniquenessFlow("reg_flow_crossou_sales", ""))
	ts.Require().NoError(err, "Failed to create the sales registration flow")
	ts.salesFlowID = salesFlowID

	// The marketing flow asks for the user type own unit, the other placement the validator honours.
	marketingFlowID, err := testutils.CreateFlow(
		buildCrossOUUniquenessFlow("reg_flow_crossou_marketing", "userType"))
	ts.Require().NoError(err, "Failed to create the marketing registration flow")
	ts.marketingFlowID = marketingFlowID

	authFlowID, err := testutils.CreateIsolatedAuthFlow("crossou-uniqueness-isolated-auth")
	ts.Require().NoError(err, "Failed to create the isolated auth flow")
	ts.authFlowID = authFlowID

	salesAppID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      ts.salesOUID,
		Name:                      "Cross OU Uniqueness Sales App",
		IsRegistrationFlowEnabled: true,
		RegistrationFlowID:        ts.salesFlowID,
		ClientID:                  "crossou_uniqueness_sales_client",
		ClientSecret:              "crossou_uniqueness_sales_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{crossOUSalesType.Handle},
		AuthFlowID:                ts.authFlowID,
	})
	ts.Require().NoError(err, "Failed to create the sales app")
	ts.salesAppID = salesAppID

	marketingAppID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      ts.marketingOUID,
		Name:                      "Cross OU Uniqueness Marketing App",
		IsRegistrationFlowEnabled: true,
		RegistrationFlowID:        ts.marketingFlowID,
		ClientID:                  "crossou_uniqueness_marketing_client",
		ClientSecret:              "crossou_uniqueness_marketing_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{crossOUMarketingType.Handle},
		AuthFlowID:                ts.authFlowID,
	})
	ts.Require().NoError(err, "Failed to create the marketing app")
	ts.marketingAppID = marketingAppID
}

func (ts *AttributeUniquenessCrossOUTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.createdUserIDs); err != nil {
		ts.T().Logf("Failed to clean up users: %v", err)
	}
	for _, appID := range []string{ts.salesAppID, ts.marketingAppID} {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete app %s: %v", appID, err)
		}
	}
	for _, flowID := range []string{ts.salesFlowID, ts.marketingFlowID, ts.authFlowID} {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow %s: %v", flowID, err)
		}
	}
	for _, typeID := range []string{ts.salesTypeID, ts.marketingTypeID} {
		if err := testutils.DeleteUserType(typeID); err != nil {
			ts.T().Logf("Failed to delete user type %s: %v", typeID, err)
		}
	}
	for _, ouID := range []string{ts.salesOUID, ts.marketingOUID} {
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			ts.T().Logf("Failed to delete OU %s: %v", ouID, err)
		}
	}
}

// submitCrossOURegistration initiates the registration flow for appID and submits attrs at
// prompt_attributes. CompleteFlow fails on any non-200, so NoError covers the transport status.
func (ts *AttributeUniquenessCrossOUTestSuite) submitCrossOURegistration(
	appID string, attrs map[string]string) *common.FlowStep {
	flowStep, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate the registration flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "username"),
		"Expected a username input at prompt_attributes")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, attrs, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the registration attributes")
	return flowStep
}

// crossOUUsersMatching returns the users matching an exact attribute value via GET /users?filter=.
func (ts *AttributeUniquenessCrossOUTestSuite) crossOUUsersMatching(attr, value string) []testutils.User {
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/users", nil)
	ts.Require().NoError(err, "Failed to build the user list request")

	q := url.Values{}
	q.Add("filter", attr+` eq "`+value+`"`)
	req.URL.RawQuery = q.Encode()

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "GET /users with a filter failed")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode, "Expected 200 from the filtered user list")

	var listResp testutils.UserListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp),
		"Failed to decode the user list response")
	return listResp.Users
}

// Scenario 1: the email already belongs to a user in the organization unit being provisioned into,
// so the validator rejects it before anything is created. This is the case issue #2690 reported,
// where the invite was generated and only failed later, at provisioning.
func (ts *AttributeUniquenessCrossOUTestSuite) TestEmailHeldInTargetOUIsRejected() {
	freeUsername := common.GenerateUniqueUsername("crossousales")

	flowStep := ts.submitCrossOURegistration(ts.salesAppID, map[string]string{
		"username": freeUsername,
		"email":    ts.seededEmail,
		"password": "Fresh123!",
	})

	ts.Equal("INCOMPLETE", flowStep.FlowStatus,
		"onIncomplete routing leaves the flow INCOMPLETE, not ERROR")
	ts.Require().NotNil(flowStep.Error, "Expected a structured target-OU conflict error")
	ts.Equal(errCodeEntityExistsInTargetOU, flowStep.Error.Code,
		"The conflict must be reported as an existing entity in the target organization unit")
	ts.True(common.HasInput(flowStep.Data.Inputs, "username"),
		"onIncomplete must route back to prompt_attributes")

	ts.Empty(ts.crossOUUsersMatching("username", freeUsername),
		"A rejected registration must not create the account")
}

// Scenario 2: the same email in a different organization unit is left alone, which is the point of
// cross-OU provisioning. A deployment-wide email check would reject this.
func (ts *AttributeUniquenessCrossOUTestSuite) TestEmailHeldInAnotherOUIsAllowed() {
	freeUsername := common.GenerateUniqueUsername("crossoumkt")

	flowStep := ts.submitCrossOURegistration(ts.marketingAppID, map[string]string{
		"username": freeUsername,
		"email":    ts.seededEmail,
		"password": "Fresh123!",
	})

	ts.Nil(flowStep.Error, "The same email in another organization unit must not raise a conflict")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"Registration must complete when the email is only held in another organization unit")

	created := ts.crossOUUsersMatching("username", freeUsername)
	ts.Require().Len(created, 1, "Expected exactly one user for username %q", freeUsername)
	ts.createdUserIDs = append(ts.createdUserIDs, created[0].ID)
	ts.Equal(ts.marketingOUID, created[0].OUID,
		"The provisioned user must land in the marketing organization unit")
}

// Scenario 3: a blank email skips the target-OU lookup, so an optional address does not block
// registration when cross-OU provisioning is on.
func (ts *AttributeUniquenessCrossOUTestSuite) TestBlankEmailSkipsTargetOUCheck() {
	freeUsername := common.GenerateUniqueUsername("crossoublank")

	flowStep := ts.submitCrossOURegistration(ts.salesAppID, map[string]string{
		"username": freeUsername,
		"email":    "",
		"password": "Fresh123!",
	})

	ts.Nil(flowStep.Error, "A blank email must not raise a target-OU conflict")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"Registration must complete when the email is blank")

	created := ts.crossOUUsersMatching("username", freeUsername)
	ts.Require().Len(created, 1, "Expected exactly one user for username %q", freeUsername)
	ts.createdUserIDs = append(ts.createdUserIDs, created[0].ID)
}
