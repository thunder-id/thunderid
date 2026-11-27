// Copyright 2025-2026 The ThunderID Authors
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
			// Credentials the user does not hold, set by the tests that need a recent sign-in.
			"pin":        map[string]interface{}{"type": "string", "credential": true},
			"backupcode": map[string]interface{}{"type": "string", "credential": true},
			"mobile":     map[string]interface{}{"type": "mobile"},
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

// TestSelfUserUpdateProfileTooManyIndexedValuesRejected verifies that a self-update giving an indexed
// attribute more than 100 values is refused before anything is stored.
func (s *SelfUserEndpointsSuite) TestSelfUserUpdateProfileTooManyIndexedValuesRejected() {
	before := s.selfProfileAttributes()
	emails := make([]string, 101)
	for i := range emails {
		emails[i] = fmt.Sprintf("self%d@example.com", i)
	}

	resp, err := s.doUserRequest(http.MethodPut, "/users/me", map[string]interface{}{
		"attributes": map[string]interface{}{
			"username": s.username,
			"email":    emails,
		},
	})
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.requireSelfError(resp, http.StatusBadRequest, "USR-1019")

	s.Equal(before, s.selfProfileAttributes(),
		"a rejected update must leave the whole profile unchanged")
}

// TestSelfUserUpdateCredentialsMissingNameRejected verifies that a credential update that names no
// credential is refused as missing credentials, and that the existing password still authenticates
// afterwards. The endpoint reports this distinctly from a malformed body: an empty object is a well-formed
// request that names no credential to change.
func (s *SelfUserEndpointsSuite) TestSelfUserUpdateCredentialsMissingNameRejected() {
	resp, err := s.doUserRequest(http.MethodPost, "/users/me/update-credentials", map[string]interface{}{})
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.requireSelfError(resp, http.StatusBadRequest, "USR-1049")
	s.requirePasswordUnchanged()
}

// A credential the user holds changes only with its current value, and the request names one credential the
// user type declares.
func (s *SelfUserEndpointsSuite) TestSelfUserUpdateCredentialsRejections() {
	cases := []struct {
		name    string
		payload map[string]interface{}
		status  int
		code    string
	}{
		{name: "missing current value", status: http.StatusBadRequest, code: "USR-1047",
			payload: map[string]interface{}{"name": "password", "newValue": s.password + "?"}},
		{name: "wrong current value", status: http.StatusForbidden, code: "USR-1048",
			payload: map[string]interface{}{"name": "password", "currentValue": s.password + "x",
				"newValue": s.password + "?"}},
		{name: "earlier body shape", status: http.StatusBadRequest, code: "USR-1049",
			payload: map[string]interface{}{"attributes": map[string]interface{}{"password": s.password + "?"}}},
		{name: "passkey", status: http.StatusBadRequest, code: "USR-1024",
			payload: map[string]interface{}{"name": "passkey", "newValue": "passkey-value"}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			resp, err := s.doUserRequest(http.MethodPost, "/users/me/update-credentials", tc.payload)
			s.Require().NoError(err)
			defer resp.Body.Close()

			s.requireSelfError(resp, tc.status, tc.code)
		})
	}
	s.requirePasswordUnchanged()
}

// A credential the user does not hold yet is set without a current value, within the maximum authentication
// age of a fresh sign-in. Once set, it needs its current value like any other.
func (s *SelfUserEndpointsSuite) TestSelfUserSetsACredentialNotHeldYet() {
	token, err := s.signIn.ObtainAccessToken(s.ouID, "openid", s.username, s.password, "")
	s.Require().NoError(err)
	client := testutils.GetHTTPClientWithToken(token.AccessToken)
	pin := map[string]interface{}{"name": "pin", "newValue": "PinValue-2468"}

	resp, err := client.Do(s.jsonRequest(http.MethodPost, "/users/me/update-credentials", pin))
	s.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusNoContent, resp.StatusCode, string(body))

	again, err := client.Do(s.jsonRequest(http.MethodPost, "/users/me/update-credentials", pin))
	s.Require().NoError(err)
	defer again.Body.Close()
	s.requireSelfError(again, http.StatusBadRequest, "USR-1047")
}

// With a sign-in older than the maximum authentication age, setting a credential the user does not hold and
// changing a contact attribute get 401 with an RFC 9470 challenge, while other changes and a change that
// gives the current value do not. The server restarts with a one-second age.
func (s *SelfUserEndpointsSuite) TestSelfServiceChangesNeedARecentSignIn() {
	t := s.T()
	restore, err := testutils.RestartWithConfigSection("user", func(section map[string]interface{}) {
		section["self_service"] = map[string]interface{}{"max_authentication_age_seconds": 1}
	})
	t.Cleanup(func() {
		if err := restore(); err != nil {
			t.Errorf("failed to restore the user config: %v", err)
		}
	})
	s.Require().NoError(err)

	token, err := s.signIn.ObtainAccessToken(s.ouID, "openid", s.username, s.password, "")
	s.Require().NoError(err)
	client := testutils.GetHTTPClientWithToken(token.AccessToken)
	time.Sleep(2 * time.Second)
	profile := s.selfProfileAttributes()

	challenged := []struct {
		name    string
		method  string
		path    string
		payload map[string]interface{}
	}{
		{name: "credential not held", method: http.MethodPost, path: "/users/me/update-credentials",
			payload: map[string]interface{}{"name": "backupcode", "newValue": "BackupCode-1357"}},
		{name: "mobile change", method: http.MethodPut, path: "/users/me",
			payload: map[string]interface{}{"attributes": withAttribute(profile, "mobile", "+14155550123")}},
	}
	for _, tc := range challenged {
		s.Run(tc.name, func() {
			resp, err := client.Do(s.jsonRequest(tc.method, tc.path, tc.payload))
			s.Require().NoError(err)
			defer resp.Body.Close()

			challenge := resp.Header.Get("WWW-Authenticate")
			s.requireSelfError(resp, http.StatusUnauthorized, "AUTH-4011")
			s.Contains(challenge, `error="insufficient_user_authentication"`)
			s.Contains(challenge, `max_age="1"`)
		})
	}

	s.Run("unchanged contacts", func() {
		resp, err := client.Do(s.jsonRequest(http.MethodPut, "/users/me",
			map[string]interface{}{"attributes": profile}))
		s.Require().NoError(err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode, string(body))
	})

	// A credential the user holds needs its current value instead, so an old sign-in does not matter.
	s.Run("held credential with its current value", func() {
		newPassword := s.password + "#"
		resp, err := client.Do(s.jsonRequest(http.MethodPost, "/users/me/update-credentials",
			map[string]interface{}{"name": "password", "currentValue": s.password, "newValue": newPassword}))
		s.Require().NoError(err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		s.Require().NoError(err)
		s.Require().Equal(http.StatusNoContent, resp.StatusCode, string(body))
		s.password = newPassword

		// The change revokes the user's tokens up to a sub-second cutoff, so sign in again after it.
		time.Sleep(1100 * time.Millisecond)
		userClient, err := s.signIn.HTTPClientForUser(s.ouID, s.username, s.password)
		s.Require().NoError(err)
		s.userClient = userClient
	})
}

// withAttribute returns a copy of attributes with key set to value.
func withAttribute(attributes map[string]interface{}, key string, value interface{}) map[string]interface{} {
	copied := make(map[string]interface{}, len(attributes)+1)
	for k, v := range attributes {
		copied[k] = v
	}
	copied[key] = value
	return copied
}

// jsonRequest builds a request with the given JSON body.
func (s *SelfUserEndpointsSuite) jsonRequest(method, path string, payload map[string]interface{}) *http.Request {
	body, err := json.Marshal(payload)
	s.Require().NoError(err)
	req, err := http.NewRequest(method, testutils.TestServerURL+path, bytes.NewReader(body))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// requirePasswordUnchanged checks that the user's current password still signs them in and grants access.
func (s *SelfUserEndpointsSuite) requirePasswordUnchanged() {
	client, err := s.signIn.HTTPClientForUser(s.ouID, s.username, s.password)
	s.Require().NoError(err, "the existing password must still authenticate")

	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/users/me", nil)
	s.Require().NoError(err)
	verifyResp, err := client.Do(req)
	s.Require().NoError(err)
	defer verifyResp.Body.Close()
	s.Equal(http.StatusOK, verifyResp.StatusCode, "the existing password must still grant access")
}

// TestSelfUserTimestamps verifies /users/me exposes createdAt/updatedAt and that PUT /users/me
// returns the post-update values rather than the pre-update ones.
func (s *SelfUserEndpointsSuite) TestSelfUserTimestamps() {
	resp, err := s.doUserRequest(http.MethodGet, "/users/me", nil)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var before testutils.User
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&before))

	createdAt, err := time.Parse(time.RFC3339, before.CreatedAt)
	s.Require().NoError(err, "createdAt must be RFC 3339")
	updatedAt, err := time.Parse(time.RFC3339, before.UpdatedAt)
	s.Require().NoError(err, "updatedAt must be RFC 3339")
	s.Require().False(updatedAt.Before(createdAt))

	payload := map[string]interface{}{
		"attributes": map[string]interface{}{
			"username": s.username,
			"email":    s.email,
		},
	}
	updateResp, err := s.doUserRequest(http.MethodPut, "/users/me", payload)
	s.Require().NoError(err)
	defer updateResp.Body.Close()
	s.Require().Equal(http.StatusOK, updateResp.StatusCode)

	var after testutils.User
	s.Require().NoError(json.NewDecoder(updateResp.Body).Decode(&after))

	s.Equal(before.CreatedAt, after.CreatedAt, "createdAt must not change on update")
	newUpdatedAt, err := time.Parse(time.RFC3339, after.UpdatedAt)
	s.Require().NoError(err, "updatedAt must be RFC 3339")
	s.Require().False(newUpdatedAt.Before(updatedAt), "updatedAt must advance on update")

	verifyResp, err := s.doUserRequest(http.MethodGet, "/users/me", nil)
	s.Require().NoError(err)
	defer verifyResp.Body.Close()
	s.Require().Equal(http.StatusOK, verifyResp.StatusCode)

	var reread testutils.User
	s.Require().NoError(json.NewDecoder(verifyResp.Body).Decode(&reread))
	s.Equal(after.CreatedAt, reread.CreatedAt)
	s.Equal(after.UpdatedAt, reread.UpdatedAt)
}
