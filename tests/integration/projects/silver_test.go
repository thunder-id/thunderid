/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package projects

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

type SilverTestSuite struct {
	suite.Suite
	client       *http.Client
	oUID         string
	entityTypeID string
	userID       string
}

const (
	credentialsAuthEndpoint = "/auth/credentials/authenticate"
	entityTypeName          = "emailuser"
	username                = "alice"
	password                = "secret123"
	email                   = "alice@example.com"
	userFilterEndpoint      = "/users?filter=username%20eq%20%22" + username + "%22"
)

var (
	testOU = testutils.OrganizationUnit{
		Handle: "test-customers",
		Name:   "Customers OU",
		Parent: nil,
	}
	entityType = testutils.UserType{
		Name: entityTypeName,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type":   "string",
				"unique": true,
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"email": map[string]interface{}{
				"type":   "string",
				"unique": true,
			},
		},
	}
	user = testutils.User{
		Type: entityTypeName,
		Attributes: json.RawMessage(`{
			"username": "` + username + `", 
			"password": "` + password + `", 
			"email": "` + email + `"
		}`),
	}
)

func TestSilverTestSuite(t *testing.T) {
	suite.Run(t, new(SilverTestSuite))
}

func (ts *SilverTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	// Create organization unit
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	ts.oUID = ouID

	// Create user type
	entityType.OUID = ts.oUID
	schemaID, err := testutils.CreateUserType(entityType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type: %v", err)
	}
	ts.entityTypeID = schemaID

	// Create user
	user.OUID = ts.oUID
	userID, err := testutils.CreateUser(user)
	if err != nil {
		ts.T().Fatalf("Failed to create test user: %v", err)
	}
	ts.userID = userID
}

func (ts *SilverTestSuite) TearDownSuite() {
	// Clean up created user
	if ts.userID != "" {
		if err := testutils.DeleteUser(ts.userID); err != nil {
			ts.T().Logf("Failed to delete test user %s: %v", ts.userID, err)
		}
	}

	// Clean up created user type
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type %s: %v", ts.entityTypeID, err)
		}
	}

	// Clean up created organization unit
	if ts.oUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.oUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit %s: %v", ts.oUID, err)
		}
	}
}

func (ts *SilverTestSuite) TestSilver_AuthenticateUser() {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": username,
		},
		"credentials": map[string]interface{}{
			"password": password,
		},
	}
	requestJSON, err := json.Marshal(authRequest)
	ts.Require().NoError(err, "Failed to marshal auth request")

	req, err := http.NewRequest("POST", testutils.TestServerURL+credentialsAuthEndpoint,
		bytes.NewReader(requestJSON))
	ts.Require().NoError(err, "Failed to create new HTTP request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send HTTP request")
	defer resp.Body.Close()
	ts.Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for successful authentication")

	var response testutils.AuthenticationResponse
	bodyBytes, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read response body")
	err = json.Unmarshal(bodyBytes, &response)
	ts.Require().NoError(err, "Failed to unmarshal authentication response")

	ts.NotEmpty(response.ID, "Response should contain user ID")
	ts.Equal(user.Type, response.Type, "Response should contain correct user type")
	ts.Equal(ts.userID, response.ID, "Response should contain the correct user ID")
}

func (ts *SilverTestSuite) TestSilver_SearchUser() {
	req, err := http.NewRequest("GET", testutils.TestServerURL+userFilterEndpoint, nil)
	ts.Require().NoError(err, "Failed to create new HTTP request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send HTTP request")
	defer resp.Body.Close()
	ts.Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for successful user search")

	var response testutils.UserListResponse
	bodyBytes, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read response body")
	err = json.Unmarshal(bodyBytes, &response)
	ts.Require().NoError(err, "Failed to unmarshal user search response")

	ts.Len(response.Users, 1, "Expected to find one user")
	ts.Equal(ts.userID, response.Users[0].ID, "Response should contain the correct user ID")
}
