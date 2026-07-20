/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package session

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const sessionsEndpoint = "/sessions"

var testOU = testutils.OrganizationUnit{
	Handle:      "session-visibility-test-ou",
	Name:        "Session Visibility Test Organization Unit",
	Description: "Organization unit for session visibility integration testing",
	Parent:      nil,
}

var sessionVisibilityUserType = testutils.UserType{
	Name: "session_visibility_user",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{
			"type": "string",
		},
	},
}

// sessionListResponse mirrors the subset of the GET /sessions payload this suite asserts on.
// See backend/internal/flow/session/mgt/model.go for the full response shape.
type sessionListResponse struct {
	TotalResults int `json:"totalResults"`
	Sessions     []struct {
		ID     string `json:"id"`
		UserID string `json:"userId"`
	} `json:"sessions"`
}

type SessionVisibilityTestSuite struct {
	suite.Suite
	client     *http.Client
	ouID       string
	userTypeID string
	userID     string
}

func TestSessionVisibilityTestSuite(t *testing.T) {
	suite.Run(t, new(SessionVisibilityTestSuite))
}

func (suite *SessionVisibilityTestSuite) SetupSuite() {
	suite.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testOU)
	suite.Require().NoError(err, "Failed to create test organization unit during setup")
	suite.ouID = ouID

	userType := sessionVisibilityUserType
	userType.OUID = suite.ouID
	userTypeID, err := testutils.CreateUserType(userType)
	suite.Require().NoError(err, "Failed to create test user type during setup")
	suite.userTypeID = userTypeID

	attributes, err := json.Marshal(map[string]interface{}{
		"username": "session_visibility_user1",
	})
	suite.Require().NoError(err, "Failed to marshal user attributes")

	userID, err := testutils.CreateUser(testutils.User{
		Type:       sessionVisibilityUserType.Name,
		OUID:       suite.ouID,
		Attributes: json.RawMessage(attributes),
	})
	suite.Require().NoError(err, "Failed to create test user during setup")
	suite.userID = userID
}

func (suite *SessionVisibilityTestSuite) TearDownSuite() {
	if suite.userID != "" {
		if err := testutils.DeleteUser(suite.userID); err != nil {
			suite.T().Errorf("Failed to delete user %s during teardown: %v", suite.userID, err)
		}
	}

	if suite.userTypeID != "" {
		if err := testutils.DeleteUserType(suite.userTypeID); err != nil {
			suite.T().Errorf("Failed to delete user type %s during teardown: %v", suite.userTypeID, err)
		}
	}

	if suite.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
			suite.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// TestSessionListRequiresExactlyOneFilter verifies that GET /sessions rejects a request that
// supplies neither the userId nor the appId filter with SSM-1002 (session.mgt.error.invalid_filter).
func (suite *SessionVisibilityTestSuite) TestSessionListRequiresExactlyOneFilter() {
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+sessionsEndpoint, nil)
	suite.Require().NoError(err, "Failed to build request")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err, "Failed to send request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err, "Failed to read response body")

	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Expected status 400 when no filter is supplied")
	suite.Contains(string(body), "SSM-1002", "Expected error body to contain SSM-1002")
}

// TestSessionListForNeverAuthenticatedUser exercises GET /sessions?userId=<id> for a user that
// was created but has never authenticated, using the admin client. This is also the first
// end-to-end probe of whether the seeded Administrator role's root "system" permission
// hierarchically covers "system:session:view" (see backend/internal/system/security/permissions.go);
// a 403 here would mean the seeded admin role does not actually grant session visibility.
//
// NOTE: the task brief's preferred Test 2 drives a real browser SSO session to completion (via
// the flow-centric browser SSO work introduced by PR #3779, backend/internal/flow/session) so the
// listing is non-empty and its participants/cookie-exposure can be asserted. As of writing,
// tests/integration has no SSO/flow-execution test that establishes such a session and captures
// its cookie (no cookie-jar helper, no "tid_sso_"-prefixed cookie handling anywhere under
// tests/integration) — see PR #3779's diff, which only touched backend/ and frontend/, not
// tests/integration/. Building that harness is out of scope for this test file, so this test
// instead covers the filter's zero-result path: a user that exists but was never authenticated
// must have no live sessions.
func (suite *SessionVisibilityTestSuite) TestSessionListForNeverAuthenticatedUser() {
	req, err := http.NewRequest(http.MethodGet,
		testutils.TestServerURL+sessionsEndpoint+"?userId="+url.QueryEscape(suite.userID), nil)
	suite.Require().NoError(err, "Failed to build request")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err, "Failed to send request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err, "Failed to read response body")
	suite.Require().Equal(http.StatusOK, resp.StatusCode,
		"Expected status 200 for a valid userId filter. Response: %s", string(body))

	var listResp sessionListResponse
	err = json.Unmarshal(body, &listResp)
	suite.Require().NoError(err, "Failed to parse response body")

	suite.Equal(0, listResp.TotalResults, "A user that never authenticated should have no live sessions")
	suite.Empty(listResp.Sessions, "A user that never authenticated should have no sessions in the page")
}
