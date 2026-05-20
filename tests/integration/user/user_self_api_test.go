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

package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

type SelfUserEndpointsSuite struct {
	suite.Suite
	ouID       string
	schemaID   string
	userClient *http.Client
	userID     string
	userType   string
	username   string
	email      string
	password   string
}

func TestSelfUserEndpointsSuite(t *testing.T) {
	suite.Run(t, new(SelfUserEndpointsSuite))
}

func (s *SelfUserEndpointsSuite) SetupSuite() {
	s.userType = "self-user-type"
	s.username = "self.user"
	s.email = "self.user@example.com"
	s.password = "SelfUserP@ssw0rd!"

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "self-ou",
		Name:   "Self User OU",
	})
	s.Require().NoError(err)
	s.ouID = ouID

	schema := testutils.UserType{
		Name:                  s.userType,
		OUID:                  ouID,
		AllowSelfRegistration: true,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
			"email":    map[string]interface{}{"type": "string", "required": true, "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}
	schemaID, err := testutils.CreateUserType(schema)
	s.Require().NoError(err)
	s.schemaID = schemaID

	attrs, err := json.Marshal(map[string]interface{}{
		"username": s.username,
		"email":    s.email,
		"password": s.password,
	})
	s.Require().NoError(err)

	userID, err := testutils.CreateUser(testutils.User{
		OUID:             ouID,
		Type:             s.userType,
		Attributes:       attrs,
	})
	s.Require().NoError(err)
	s.userID = userID

	client, err := testutils.GetHTTPClientForUser(s.username, s.password)
	s.Require().NoError(err)
	s.userClient = client
}

func (s *SelfUserEndpointsSuite) TearDownSuite() {
	if s.userID != "" {
		s.Require().NoError(testutils.DeleteUser(s.userID))
	}
	if s.schemaID != "" {
		s.Require().NoError(testutils.DeleteUserType(s.schemaID))
	}
	if s.ouID != "" {
		s.Require().NoError(testutils.DeleteOrganizationUnit(s.ouID))
	}
}

func (s *SelfUserEndpointsSuite) doUserRequest(method, path string, payload interface{}) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, testutils.TestServerURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return s.userClient.Do(req)
}

func (s *SelfUserEndpointsSuite) TestSelfUserGetProfile() {
	resp, err := s.doUserRequest(http.MethodGet, "/users/me", nil)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var userResp testutils.User
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&userResp))

	s.Equal(s.userID, userResp.ID)
	s.Equal(s.userType, userResp.Type)

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(userResp.Attributes, &attrs))
	s.Equal(s.username, attrs["username"])
	s.Equal(s.email, attrs["email"])
}

func (s *SelfUserEndpointsSuite) TestSelfUserUpdateProfile() {
	newUsername := s.username + ".updated"
	newEmail := s.email + ".updated"
	payload := map[string]interface{}{
		"attributes": map[string]interface{}{
			"username": newUsername,
			"email":    newEmail,
		},
	}

	resp, err := s.doUserRequest(http.MethodPut, "/users/me", payload)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var userResp testutils.User
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&userResp))

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(userResp.Attributes, &attrs))
	s.Equal(newEmail, attrs["email"])
	s.Equal(newUsername, attrs["username"])

	s.email = newEmail
	s.username = newUsername
}

func (s *SelfUserEndpointsSuite) TestSelfUserUpdateCredentials() {
	newPassword := s.password + "!"
	payload := map[string]interface{}{
		"attributes": map[string]interface{}{
			"password": newPassword,
		},
	}

	resp, err := s.doUserRequest(http.MethodPost, "/users/me/update-credentials", payload)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	client, err := testutils.GetHTTPClientForUser(s.username, newPassword)
	s.Require().NoError(err)
	s.userClient = client
	s.password = newPassword

	verifyResp, err := s.doUserRequest(http.MethodGet, "/users/me", nil)
	s.Require().NoError(err)
	defer verifyResp.Body.Close()

	s.Require().Equal(http.StatusOK, verifyResp.StatusCode)

	var userResp testutils.User
	s.Require().NoError(json.NewDecoder(verifyResp.Body).Decode(&userResp))
	s.Equal(s.userID, userResp.ID)
}
