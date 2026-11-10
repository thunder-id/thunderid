// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type SelfUserEndpointsSuite struct {
	suite.Suite
	// The user lives outside the default OU, and a sign-in is bounded to its application's OU, so it signs in
	// through an application in its own OU rather than the Console.
	signIn     testutils.OUSignInApps
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
		Handle:                s.userType,
		DisplayName:           "User Type",
		OUID:                  ouID,
		AllowSelfRegistration: true,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
			"email":    map[string]interface{}{"type": "string", "required": true, "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"pin":      map[string]interface{}{"type": "string", "credential": true},
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
		OUID:       ouID,
		Type:       s.userType,
		Attributes: attrs,
	})
	s.Require().NoError(err)
	s.userID = userID

	client, err := s.signIn.HTTPClientForUser(s.ouID, s.username, s.password)
	s.Require().NoError(err)
	s.userClient = client
}

func (s *SelfUserEndpointsSuite) TearDownSuite() {
	s.Require().NoError(s.signIn.DeleteAll())
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

func (s *SelfUserEndpointsSuite) TestSelfUserUpdateReportsTheDerivedStatus() {
	resp, err := s.doUserRequest(http.MethodPut, "/users/me", map[string]interface{}{
		"attributes": s.selfProfileAttributes(),
		"status":     map[string]interface{}{"value": "SUSPENDED"},
	})
	s.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode, string(raw))

	var updated struct {
		Status json.RawMessage `json:"status"`
	}
	s.Require().NoError(json.Unmarshal(raw, &updated))
	s.JSONEq(`{"value":"ACTIVE"}`, string(updated.Status), "the update must report the derived status")
}

func (s *SelfUserEndpointsSuite) TestSelfUserUpdateCredentials() {
	newPassword := s.password + "!"
	payload := map[string]interface{}{
		"name":         "password",
		"currentValue": s.password,
		"newValue":     newPassword,
	}

	resp, err := s.doUserRequest(http.MethodPost, "/users/me/update-credentials", payload)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	// The change revokes the user's tokens up to a sub-second cutoff, while a grant's issue time has second
	// granularity and the comparison is at or before.
	time.Sleep(1100 * time.Millisecond)
	client, err := s.signIn.HTTPClientForUser(s.ouID, s.username, newPassword)
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

func (s *SelfUserEndpointsSuite) TestSelfUserGetMetadata() {
	resp, err := s.doUserRequest(http.MethodGet, "/users/me/meta", nil)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var res map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&res))

	schema, ok := res["schema"].(map[string]interface{})
	s.Require().True(ok, "response should contain schema map")
	s.Require().NotNil(schema["username"])
	s.Require().NotNil(schema["email"])
}

// selfProfileAttributes reads the caller's own attributes, so a rejected update can be checked
// against stored state rather than only against the error response.
func (s *SelfUserEndpointsSuite) selfProfileAttributes() map[string]interface{} {
	s.T().Helper()

	resp, err := s.doUserRequest(http.MethodGet, "/users/me", nil)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var userResp testutils.User
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&userResp))

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(userResp.Attributes, &attrs))
	return attrs
}

// requireSelfError asserts a self-service response carries the exact status and product error code.
func (s *SelfUserEndpointsSuite) requireSelfError(resp *http.Response, status int, code string) {
	s.T().Helper()

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.Require().Equal(status, resp.StatusCode, "error body: %s", string(body))

	var errResp struct {
		Code string `json:"code"`
	}
	s.Require().NoError(json.Unmarshal(body, &errResp), "error body: %s", string(body))
	s.Equal(code, errResp.Code, "error body: %s", string(body))
}

// TestSelfUserUpdateProfileEmptyBodyRejected verifies that a self-update carrying no attributes is
// refused rather than treated as a no-op, and that the profile is untouched.
func (s *SelfUserEndpointsSuite) TestSelfUserUpdateProfileEmptyBodyRejected() {
	before := s.selfProfileAttributes()

	resp, err := s.doUserRequest(http.MethodPut, "/users/me", map[string]interface{}{})
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.requireSelfError(resp, http.StatusBadRequest, "USR-1001")

	s.Equal(before, s.selfProfileAttributes(), "a rejected update must leave the profile unchanged")
}

// TestSelfUserUpdateProfileEmptyAttributeObjectRejected verifies that an explicitly empty attribute
// object is refused by schema validation rather than accepted as a wipe. This is a different
// rejection from the empty body above: `{}` leaves Attributes unset and is caught in the handler
// (USR-1001), whereas `{"attributes": {}}` is a present-but-empty object that reaches the service
// and fails the schema's required fields (USR-1019). Both must leave the profile intact.
func (s *SelfUserEndpointsSuite) TestSelfUserUpdateProfileEmptyAttributeObjectRejected() {
	before := s.selfProfileAttributes()

	resp, err := s.doUserRequest(http.MethodPut, "/users/me", map[string]interface{}{
		"attributes": map[string]interface{}{},
	})
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.requireSelfError(resp, http.StatusBadRequest, "USR-1019")

	s.Equal(before, s.selfProfileAttributes(),
		"a rejected empty-attribute update must not clear the stored attributes")
}

// TestSelfUserUpdateProfileSchemaInvalidAttributeRejected verifies that a self-update is validated
// against the user type schema: email is declared as a string, so a number is refused and the
// stored value survives.
//
// The payload carries the current valid username alongside the bad email. Sending the email alone
// would omit a required attribute — this endpoint replaces the whole attribute set — and the missing
// username produces USR-1019 on its own, so the test could pass without the numeric email ever
// being type-checked.
func (s *SelfUserEndpointsSuite) TestSelfUserUpdateProfileSchemaInvalidAttributeRejected() {
	before := s.selfProfileAttributes()

	resp, err := s.doUserRequest(http.MethodPut, "/users/me", map[string]interface{}{
		"attributes": map[string]interface{}{
			"username": s.username,
			"email":    12345,
		},
	})
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.requireSelfError(resp, http.StatusBadRequest, "USR-1019")

	s.Equal(before, s.selfProfileAttributes(),
		"a rejected update must leave the whole profile unchanged")
}

// TestSelfUserUpdateCredentialsMissingNewValueRejected verifies that a credential update with no new
// value is refused as missing credentials, and that the existing password still authenticates
// afterwards.
func (s *SelfUserEndpointsSuite) TestSelfUserUpdateCredentialsMissingNewValueRejected() {
	resp, err := s.doUserRequest(http.MethodPost, "/users/me/update-credentials",
		map[string]interface{}{"name": "password", "currentValue": s.password})
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.requireSelfError(resp, http.StatusBadRequest, "USR-1017")

	// The rejected call must not have rotated or cleared the password.
	client, err := s.signIn.HTTPClientForUser(s.ouID, s.username, s.password)
	s.Require().NoError(err, "the existing password must still authenticate")

	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/users/me", nil)
	s.Require().NoError(err)
	verifyResp, err := client.Do(req)
	s.Require().NoError(err)
	defer verifyResp.Body.Close()
	s.Equal(http.StatusOK, verifyResp.StatusCode, "the existing password must still grant access")
}

// requirePasswordStillWorks asserts the suite user still signs in with the current password.
func (s *SelfUserEndpointsSuite) requirePasswordStillWorks() {
	s.T().Helper()

	client, err := s.signIn.HTTPClientForUser(s.ouID, s.username, s.password)
	s.Require().NoError(err, "the existing password must still authenticate")

	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/users/me", nil)
	s.Require().NoError(err)
	resp, err := client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode, "the existing password must still grant access")
}

// TestSelfUserUpdateCredentialsInvalidCurrentValueRejected verifies that changing a held credential
// needs its current value, and a missing value is refused the same way as a wrong one.
func (s *SelfUserEndpointsSuite) TestSelfUserUpdateCredentialsInvalidCurrentValueRejected() {
	testCases := []struct {
		name    string
		payload map[string]interface{}
	}{
		{
			name: "Wrong current value",
			payload: map[string]interface{}{
				"name": "password", "currentValue": "Wr0ngP@ssword!", "newValue": "N3wSelfP@ssw0rd!",
			},
		},
		{
			name:    "Missing current value",
			payload: map[string]interface{}{"name": "password", "newValue": "N3wSelfP@ssw0rd!"},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			resp, err := s.doUserRequest(http.MethodPost, "/users/me/update-credentials", tc.payload)
			s.Require().NoError(err)
			defer resp.Body.Close()

			s.requireSelfError(resp, http.StatusForbidden, "USR-1047")
			s.requirePasswordStillWorks()
		})
	}
}

// TestSelfUserUpdateCredentialsNewCredentialRejected verifies that a user who holds a password can't
// add a credential they don't hold yet, since there is nothing to check it against.
func (s *SelfUserEndpointsSuite) TestSelfUserUpdateCredentialsNewCredentialRejected() {
	resp, err := s.doUserRequest(http.MethodPost, "/users/me/update-credentials",
		map[string]interface{}{"name": "pin", "newValue": "482913"})
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.requireSelfError(resp, http.StatusBadRequest, "USR-1048")
}

// TestSelfUserUpdateCredentialsAttributesBodyRejected verifies that a body in the earlier attributes
// shape is refused as malformed, and the password stays as it was.
func (s *SelfUserEndpointsSuite) TestSelfUserUpdateCredentialsAttributesBodyRejected() {
	resp, err := s.doUserRequest(http.MethodPost, "/users/me/update-credentials",
		map[string]interface{}{"attributes": map[string]interface{}{"password": "N3wSelfP@ssw0rd!"}})
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.requireSelfError(resp, http.StatusBadRequest, "USR-1001")
	s.requirePasswordStillWorks()
}
