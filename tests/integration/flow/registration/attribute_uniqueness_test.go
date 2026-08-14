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

// errCodeAttributeNotUnique is raised when a unique attribute value is already taken.
const errCodeAttributeNotUnique = "FET-1061"

var uniquenessOU = testutils.OrganizationUnit{
	Handle:      "attribute-uniqueness-test-ou",
	Name:        "Attribute Uniqueness Test OU",
	Description: "Organization unit for attribute uniqueness validator testing",
}

// uniqueAttrsUserType declares two unique attributes so a conflict can be attributed precisely.
var uniqueAttrsUserType = testutils.UserType{
	Name:                  "uniqueness-test-customer",
	AllowSelfRegistration: true,
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string", "unique": true},
		"email":    map[string]interface{}{"type": "string", "unique": true},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

// noUniqueAttrsUserType has no unique attributes, so the validator must pass everything through.
var noUniqueAttrsUserType = testutils.UserType{
	Name:                  "uniqueness-test-open",
	AllowSelfRegistration: true,
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"email":    map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

type AttributeUniquenessTestSuite struct {
	suite.Suite
	ouID             string
	uniqueTypeID     string
	openTypeID       string
	uniqueFlowID     string
	openFlowID       string
	authFlowID       string
	uniqueAppID      string
	openAppID        string
	existingUsername string
	existingEmail    string
	createdUserIDs   []string
}

func TestAttributeUniquenessTestSuite(t *testing.T) {
	suite.Run(t, new(AttributeUniquenessTestSuite))
}

// buildUniquenessFlow returns a registration flow that routes a uniqueness rejection back to the
// prompt via onIncomplete. REGISTRATION requires UserTypeResolver and ProvisioningExecutor, and
// onIncomplete must target a PROMPT node. email is optional so a blank value reaches the validator.
func buildUniquenessFlow(handle string) testutils.Flow {
	return testutils.Flow{
		Name:     "Attribute Uniqueness Registration Flow",
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

func (ts *AttributeUniquenessTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(uniquenessOU)
	ts.Require().NoError(err, "Failed to create test OU")
	ts.ouID = ouID

	uniqueAttrsUserType.OUID = ts.ouID
	uniqueTypeID, err := testutils.CreateUserType(uniqueAttrsUserType)
	ts.Require().NoError(err, "Failed to create user type with unique attributes")
	ts.uniqueTypeID = uniqueTypeID

	noUniqueAttrsUserType.OUID = ts.ouID
	openTypeID, err := testutils.CreateUserType(noUniqueAttrsUserType)
	ts.Require().NoError(err, "Failed to create user type without unique attributes")
	ts.openTypeID = openTypeID

	// Seed the user whose username and email later registrations must collide with.
	ts.existingUsername = common.GenerateUniqueUsername("uniqoccupied")
	ts.existingEmail = ts.existingUsername + "@example.com"
	userIDs, err := testutils.CreateMultipleUsers(testutils.User{
		OUID: ts.ouID,
		Type: uniqueAttrsUserType.Name,
		Attributes: json.RawMessage(`{
			"username": "` + ts.existingUsername + `",
			"email":    "` + ts.existingEmail + `",
			"password": "Occupied123!"
		}`),
	})
	ts.Require().NoError(err, "Failed to seed the conflicting user")
	ts.createdUserIDs = append(ts.createdUserIDs, userIDs...)

	uniqueFlowID, err := testutils.CreateFlow(buildUniquenessFlow("reg_flow_attr_uniqueness"))
	ts.Require().NoError(err, "Failed to create uniqueness registration flow")
	ts.uniqueFlowID = uniqueFlowID

	openFlowID, err := testutils.CreateFlow(buildUniquenessFlow("reg_flow_attr_uniqueness_open"))
	ts.Require().NoError(err, "Failed to create open-type registration flow")
	ts.openFlowID = openFlowID

	// A custom registration flow needs an isolated auth flow. Left to default, the app's auth flow
	// CALLs the default registration flow, and the server rejects the mismatch with APP-1039.
	authFlowID, err := testutils.CreateIsolatedAuthFlow("attr-uniqueness-isolated-auth")
	ts.Require().NoError(err, "Failed to create isolated auth flow")
	ts.authFlowID = authFlowID

	// One app per user type: a single allowed user type lets UserTypeResolver auto-select.
	uniqueAppID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      ts.ouID,
		Name:                      "Attribute Uniqueness App",
		IsRegistrationFlowEnabled: true,
		RegistrationFlowID:        ts.uniqueFlowID,
		ClientID:                  "attr_uniqueness_client",
		ClientSecret:              "attr_uniqueness_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{uniqueAttrsUserType.Name},
		AuthFlowID:                ts.authFlowID,
	})
	ts.Require().NoError(err, "Failed to create uniqueness test app")
	ts.uniqueAppID = uniqueAppID

	openAppID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      ts.ouID,
		Name:                      "Attribute Uniqueness Open App",
		IsRegistrationFlowEnabled: true,
		RegistrationFlowID:        ts.openFlowID,
		ClientID:                  "attr_uniqueness_open_client",
		ClientSecret:              "attr_uniqueness_open_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{noUniqueAttrsUserType.Name},
		AuthFlowID:                ts.authFlowID,
	})
	ts.Require().NoError(err, "Failed to create open-type test app")
	ts.openAppID = openAppID
}

func (ts *AttributeUniquenessTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.createdUserIDs); err != nil {
		ts.T().Logf("Failed to clean up users: %v", err)
	}
	for _, appID := range []string{ts.uniqueAppID, ts.openAppID} {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete app %s: %v", appID, err)
		}
	}
	for _, flowID := range []string{ts.uniqueFlowID, ts.openFlowID, ts.authFlowID} {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow %s: %v", flowID, err)
		}
	}
	for _, typeID := range []string{ts.uniqueTypeID, ts.openTypeID} {
		if err := testutils.DeleteUserType(typeID); err != nil {
			ts.T().Logf("Failed to delete user type %s: %v", typeID, err)
		}
	}
	if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
		ts.T().Logf("Failed to delete OU: %v", err)
	}
}

// submitRegistration initiates the registration flow for appID and submits attrs at
// prompt_attributes. CompleteFlow fails on any non-200, so NoError covers the transport status.
func (ts *AttributeUniquenessTestSuite) submitRegistration(
	appID string, attrs map[string]string) *common.FlowStep {
	flowStep, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate registration flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "username"),
		"Expected username input at prompt_attributes")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, attrs, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit registration attributes")
	return flowStep
}

// usersMatching returns the users matching an exact attribute value via GET /users?filter=.
func (ts *AttributeUniquenessTestSuite) usersMatching(attr, value string) []testutils.User {
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

// trackCreatedUser records a provisioned user for teardown and returns it.
func (ts *AttributeUniquenessTestSuite) trackCreatedUser(username string) testutils.User {
	created := ts.usersMatching("username", username)
	ts.Require().Len(created, 1, "Expected exactly one user for username %q", username)
	ts.createdUserIDs = append(ts.createdUserIDs, created[0].ID)
	return created[0]
}

// assertProvisioned checks the persisted user carries the expected type, OU and attributes.
func (ts *AttributeUniquenessTestSuite) assertProvisioned(
	user testutils.User, userType, username, email string) {
	ts.Equal(userType, user.Type, "The provisioned user must carry the resolved user type")
	ts.Equal(ts.ouID, user.OUID, "The provisioned user must land in the user type's OU")

	var attrs map[string]interface{}
	ts.Require().NoError(json.Unmarshal(user.Attributes, &attrs),
		"Failed to decode the persisted attributes")
	ts.Equal(username, attrs["username"])
	if email != "" {
		ts.Equal(email, attrs["email"])
	}
	ts.NotContains(attrs, "password", "The credential must not be readable back")
}

// Scenario 1: a username conflict names the attribute, re-prompts, and creates nothing.
func (ts *AttributeUniquenessTestSuite) TestUniquenessConflictOnUsernameNamesAttribute() {
	freeEmail := common.GenerateUniqueUsername("freeemail") + "@example.com"
	before := len(ts.usersMatching("username", ts.existingUsername))

	flowStep := ts.submitRegistration(ts.uniqueAppID, map[string]string{
		"username": ts.existingUsername,
		"email":    freeEmail,
		"password": "Fresh123!",
	})

	ts.Equal("INCOMPLETE", flowStep.FlowStatus,
		"onIncomplete routing leaves the flow INCOMPLETE, not ERROR")
	ts.Require().NotNil(flowStep.Error, "Expected a structured uniqueness error")
	ts.Equal(errCodeAttributeNotUnique, flowStep.Error.Code)
	ts.Equal("username", flowStep.Error.Message.Params["attribute"],
		"The error must name the conflicting attribute")
	ts.True(common.HasInput(flowStep.Data.Inputs, "username"),
		"onIncomplete must route back to prompt_attributes")

	// No side effects: no duplicate of the taken username, and nothing for the free email.
	ts.Len(ts.usersMatching("username", ts.existingUsername), before,
		"A rejected registration must not create a duplicate account")
	ts.Empty(ts.usersMatching("email", freeEmail),
		"A rejected registration must not persist the non-conflicting attributes either")
}

// Scenario 3: with two unique attributes, the error names the one that actually conflicts.
func (ts *AttributeUniquenessTestSuite) TestUniquenessConflictOnEmailNamesEmailNotUsername() {
	freeUsername := common.GenerateUniqueUsername("freeuser")

	flowStep := ts.submitRegistration(ts.uniqueAppID, map[string]string{
		"username": freeUsername,
		"email":    ts.existingEmail,
		"password": "Fresh123!",
	})

	ts.Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().NotNil(flowStep.Error, "Expected a structured uniqueness error")
	ts.Equal(errCodeAttributeNotUnique, flowStep.Error.Code)
	ts.Equal("email", flowStep.Error.Message.Params["attribute"],
		"The error must name email, not the first-declared unique attribute")

	ts.Empty(ts.usersMatching("username", freeUsername),
		"A rejected registration must not create the account")
}

// Scenario 2: retrying with a free value after a conflict provisions the user. This is the
// behaviour unit tests cannot show, because it depends on onIncomplete routing.
func (ts *AttributeUniquenessTestSuite) TestUniquenessRetryAfterConflictSucceeds() {
	flowStep, err := common.InitiateRegistrationFlow(ts.uniqueAppID, false, nil, "")
	ts.Require().NoError(err)

	// First attempt collides on username.
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username": ts.existingUsername,
		"email":    common.GenerateUniqueUsername("retryemail") + "@example.com",
		"password": "Fresh123!",
	}, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().NotNil(flowStep.Error, "Expected the first attempt to be rejected")
	ts.Require().Equal(errCodeAttributeNotUnique, flowStep.Error.Code)

	// Second attempt on the same execution uses a free username.
	freeUsername := common.GenerateUniqueUsername("retryfree")
	freeEmail := freeUsername + "@example.com"
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username": freeUsername,
		"email":    freeEmail,
		"password": "Fresh123!",
	}, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"Retry with free values must complete the flow after a prior conflict")
	ts.Nil(flowStep.Error, "The successful retry must carry no error")

	ts.assertProvisioned(ts.trackCreatedUser(freeUsername),
		uniqueAttrsUserType.Name, freeUsername, freeEmail)
}

// Scenario 4: a unique attribute left blank is skipped rather than checked
// (attribute_uniqueness_validator.go:83), so an optional unique field does not block registration.
func (ts *AttributeUniquenessTestSuite) TestUniquenessSkipsBlankUniqueAttribute() {
	username := common.GenerateUniqueUsername("blankattr")

	flowStep := ts.submitRegistration(ts.uniqueAppID, map[string]string{
		"username": username,
		"email":    "",
		"password": "Fresh123!",
	})

	// Prove execution advanced past the prompt rather than being rejected by input validation.
	ts.Empty(flowStep.Data.FieldErrors,
		"A blank optional attribute must not fail prompt validation before the validator runs")
	ts.Nil(flowStep.Error, "A blank unique attribute must not raise a conflict")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"Registration must complete when an optional unique attribute is blank")

	ts.assertProvisioned(ts.trackCreatedUser(username), uniqueAttrsUserType.Name, username, "")
}

// Scenario 5: a user type with no unique attributes passes straight through the validator.
func (ts *AttributeUniquenessTestSuite) TestUniquenessNoUniqueAttributesCompletes() {
	username := common.GenerateUniqueUsername("openuser")
	email := username + "@example.com"

	flowStep := ts.submitRegistration(ts.openAppID, map[string]string{
		"username": username,
		"email":    email,
		"password": "Fresh123!",
	})

	ts.Nil(flowStep.Error, "A user type with no unique attributes must not raise a conflict")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)

	created := ts.trackCreatedUser(username)
	ts.assertProvisioned(created, noUniqueAttrsUserType.Name, username, email)
}
