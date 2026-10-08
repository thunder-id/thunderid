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

// The shipped user deletion flow. Executing it by handle keeps the test independent of the seeded
// flow id, which is a bootstrap detail.
const deletionFlowHandle = "default-user-deletion-flow"

// deletionSubjectInput is the input identifier the deletion executors read the target user from.
const deletionSubjectInput = "subject"

var (
	adminFlowTestOU = testutils.OrganizationUnit{
		Handle:      "admin_flow_test_ou",
		Name:        "Test OU for Administration Flows",
		Description: "Organization unit created for administration flow testing",
		Parent:      nil,
	}

	adminFlowTestUserType = testutils.UserType{
		Handle:      "admin_flow_test_user",
		DisplayName: "Admin Flow Test User",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}
)

type AdministrationFlowTestSuite struct {
	suite.Suite
	ouID         string
	entityTypeID string
	flowID       string
}

func TestAdministrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(AdministrationFlowTestSuite))
}

func (ts *AdministrationFlowTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(adminFlowTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	adminFlowTestUserType.OUID = ts.ouID
	schemaID, err := testutils.CreateUserType(adminFlowTestUserType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = schemaID

	flowID, err := testutils.GetFlowIDByHandle(deletionFlowHandle, administrationFlowType)
	ts.Require().NoError(err, "Failed to resolve the shipped user deletion flow")
	ts.Require().NotEmpty(flowID, "The shipped user deletion flow must be present")
	ts.flowID = flowID
}

func (ts *AdministrationFlowTestSuite) TearDownSuite() {
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// executeAdministrationFlow runs a flow by id as an administrator and returns the resulting step.
//
// The bearer token is set directly on a raw client because the shared test clients treat
// /flow/execute as a public endpoint and skip token injection, which would make every
// administration request anonymous.
func (ts *AdministrationFlowTestSuite) executeAdministrationFlow(
	flowID string, inputs map[string]string) (int, common.FlowStep, []byte) {
	ts.T().Helper()

	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err, "Failed to obtain admin access token")

	reqBody, err := json.Marshal(map[string]interface{}{
		"flowId": flowID,
		"inputs": inputs,
	})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to execute administration flow")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	var step common.FlowStep
	_ = json.Unmarshal(body, &step)

	return resp.StatusCode, step, body
}

// userExists reports whether the user record is still retrievable.
func (ts *AdministrationFlowTestSuite) userExists(userID string) bool {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+"/users/"+userID, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to read user")
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode == http.StatusOK
}

func (ts *AdministrationFlowTestSuite) createTestUser(username string) string {
	ts.T().Helper()

	attributes, err := json.Marshal(map[string]string{"username": username, "password": "Testpass1"})
	ts.Require().NoError(err)

	userID, err := testutils.CreateUser(testutils.User{
		Type:       adminFlowTestUserType.Handle,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "Failed to create test user")
	ts.Require().NotEmpty(userID)

	return userID
}

// Running the shipped deletion flow to completion exercises the whole administration chain in one
// execution: the permission validator, the pre-delete validation that publishes the trusted
// revocation plan, the criteria revocation write, session termination, and the record deletion.
func (ts *AdministrationFlowTestSuite) TestUserDeletionFlow_CompletesAndDeletesUser() {
	userID := ts.createTestUser(common.GenerateUniqueUsername("admin_flow_delete"))
	ts.Require().True(ts.userExists(userID), "The user should exist before deletion")

	status, step, body := ts.executeAdministrationFlow(ts.flowID,
		map[string]string{deletionSubjectInput: userID})

	ts.Require().Equal(http.StatusOK, status, "Deletion flow execution failed: %s", string(body))
	ts.Equal("COMPLETE", step.FlowStatus, "Deletion flow should run to completion: %s", string(body))
	ts.False(ts.userExists(userID), "The user record should be gone after the deletion flow")
}

// The pre-delete validation runs before anything destructive, so a subject that does not exist is
// refused rather than producing a completed flow that deleted nothing.
func (ts *AdministrationFlowTestSuite) TestUserDeletionFlow_UnknownSubjectDoesNotComplete() {
	status, step, body := ts.executeAdministrationFlow(ts.flowID,
		map[string]string{deletionSubjectInput: "01900000-0000-7000-8000-0000000000ff"})

	if status == http.StatusOK {
		ts.NotEqual("COMPLETE", step.FlowStatus,
			"Deleting an unknown subject must not report success: %s", string(body))
		return
	}
	ts.GreaterOrEqual(status, http.StatusBadRequest,
		"Deleting an unknown subject should be reported as an error: %s", string(body))
}

// The flow declares its subject as a required input, so omitting it must not delete anything.
func (ts *AdministrationFlowTestSuite) TestUserDeletionFlow_MissingSubjectDoesNotComplete() {
	status, step, body := ts.executeAdministrationFlow(ts.flowID, map[string]string{})

	if status == http.StatusOK {
		ts.NotEqual("COMPLETE", step.FlowStatus,
			"A deletion flow with no subject must not report success: %s", string(body))
		return
	}
	ts.GreaterOrEqual(status, http.StatusBadRequest,
		"A deletion flow with no subject should be reported as an error: %s", string(body))
}
