// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

var (
	pathTestOU = testutils.OrganizationUnit{
		Handle:      "test-ou-for-users",
		Name:        "Test OU for Users",
		Description: "Test organization unit for user path-based operations",
	}

	testUserType = testutils.UserType{
		Name:             "employee",
		SystemAttributes: &testutils.UserTypeSystemAttributes{Display: "username"},
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type":     "string",
				"unique":   true,
				"required": true,
			},
			"email": map[string]interface{}{
				"type":     "string",
				"required": true,
			},
			"department": map[string]interface{}{
				"type": "string",
			},
		},
	}
	employeeEntityTypeID string
)

// CreateUserByPathRequest represents the request body for creating a user by path.
type CreateUserByPathRequest struct {
	Type       string          `json:"type"`
	Groups     []string        `json:"groups,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
}

type UserTreeAPITestSuite struct {
	suite.Suite
	testOUID string
}

func TestUserTreeAPITestSuite(t *testing.T) {
	suite.Run(t, new(UserTreeAPITestSuite))
}

func (suite *UserTreeAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(pathTestOU)
	if err != nil {
		suite.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}

	testUserType.OUID = ouID
	schemaID, err := testutils.CreateUserType(testUserType)
	if err != nil {
		suite.T().Fatalf("Failed to create employee user type during setup: %v", err)
	}

	employeeEntityTypeID = schemaID

	suite.testOUID = ouID
	suite.T().Logf("Created test OU with ID: %s and handle: %s", suite.testOUID, pathTestOU.Handle)
}

func (suite *UserTreeAPITestSuite) TearDownSuite() {
	if employeeEntityTypeID != "" {
		if err := testutils.DeleteUserType(employeeEntityTypeID); err != nil {
			suite.T().Logf("Failed to delete employee user type during teardown: %v", err)
		}
	}

	if suite.testOUID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.testOUID); err != nil {
			suite.T().Logf("Failed to delete test OU during teardown: %v", err)
		}
	}
}

// TestGetUsersByPath tests retrieving users by organization unit handle path
func (suite *UserTreeAPITestSuite) TestGetUsersByPath() {
	if suite.testOUID == "" {
		suite.T().Fatal("OU ID is not available for path-based user retrieval")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/users/tree/"+pathTestOU.Handle, nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err, "Failed to read response body: %v", err)

	var userListResponse testutils.UserListResponse
	err = json.Unmarshal(body, &userListResponse)
	suite.Require().NoError(err)

	// Verify the response structure
	suite.GreaterOrEqual(userListResponse.TotalResults, 0)
	suite.Equal(userListResponse.StartIndex, 1)
	suite.Equal(userListResponse.Count, len(userListResponse.Users))
}

// TestCreateUserByPath tests creating a user by organization unit handle path
func (suite *UserTreeAPITestSuite) TestCreateUserByPath() {
	if suite.testOUID == "" {
		suite.T().Fatal("OU ID is not available for path-based user creation")
	}

	client := testutils.GetHTTPClient()

	createRequest := CreateUserByPathRequest{
		Type:       "employee",
		Attributes: json.RawMessage(`{"username": "test.user", "email": "test.user@example.com", "department": "Engineering"}`),
	}

	requestJSON, err := json.Marshal(createRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+"/users/tree/"+pathTestOU.Handle, bytes.NewBuffer(requestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusCreated, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var createdUser testutils.User
	err = json.Unmarshal(body, &createdUser)
	suite.Require().NoError(err)

	// Verify the created user
	suite.NotEmpty(createdUser.ID)
	suite.Equal(suite.testOUID, createdUser.OUID)
	suite.Equal("employee", createdUser.Type)
	suite.NotEmpty(createdUser.Attributes)

	// Clean up: delete the created user
	if err := testutils.DeleteUser(createdUser.ID); err != nil {
		suite.T().Logf("Failed to delete created user: %v", err)
	}
}

// TestGetUsersByInvalidPath tests retrieving users by invalid organization unit handle path
func (suite *UserTreeAPITestSuite) TestGetUsersByInvalidPath() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/users/tree/nonexistent-ou", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp testutils.ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("USR-1005", errorResp.Code)
	suite.Equal("Organization unit not found", errorResp.Message.DefaultValue)
}

// TestGetUsersByPathWithPagination tests retrieving users by path with pagination parameters
func (suite *UserTreeAPITestSuite) TestGetUsersByPathWithPagination() {
	if suite.testOUID == "" {
		suite.T().Fatal("OU ID is not available for pagination test")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/users/tree/"+pathTestOU.Handle+"?limit=5&offset=0", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var userListResponse testutils.UserListResponse
	err = json.Unmarshal(body, &userListResponse)
	suite.Require().NoError(err)

	// Verify pagination parameters
	suite.GreaterOrEqual(userListResponse.TotalResults, 0)
	suite.Equal(userListResponse.StartIndex, 1)
	suite.LessOrEqual(userListResponse.Count, 5)
}

// TestGetUsersByPathWithDisplay covers the display-resolution branch of GetUsersByPath, which runs
// only when include=display is requested and the organization unit holds at least one user.
func (suite *UserTreeAPITestSuite) TestGetUsersByPathWithDisplay() {
	username := "display.user"
	userID := suite.createUserInTestOU(username, "display.user@example.com")
	defer func() { _ = testutils.DeleteUser(userID) }()

	resp := suite.doTree(http.MethodGet,
		"/users/tree/"+pathTestOU.Handle+"?include=display", nil)
	defer func() { _ = resp.Body.Close() }()
	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp testutils.UserListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))
	suite.Require().NotEmpty(listResp.Users)

	created := suite.findUser(listResp.Users, userID)
	suite.Require().NotNil(created, "the created user must appear in the listing")
	suite.Equal(username, created.Display,
		"display must resolve to the user type's display attribute, not the identifier")
	suite.Equal(pathTestOU.Handle, created.OUHandle)
}

// TestGetUsersByPathWithoutDisplayOmitsDisplay pins display as opt-in. Resolution costs a batch
// entity fetch per request, so a change that made it unconditional would be a silent cost and
// exposure increase that only asserting its absence catches.
func (suite *UserTreeAPITestSuite) TestGetUsersByPathWithoutDisplayOmitsDisplay() {
	username := "nodisplay.user"
	userID := suite.createUserInTestOU(username, "nodisplay.user@example.com")
	defer func() { _ = testutils.DeleteUser(userID) }()

	resp := suite.doTree(http.MethodGet, "/users/tree/"+pathTestOU.Handle, nil)
	defer func() { _ = resp.Body.Close() }()
	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp testutils.UserListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	created := suite.findUser(listResp.Users, userID)
	suite.Require().NotNil(created)
	suite.Empty(created.Display, "display must not be returned unless requested")
}

// TestGetUsersByPathWithDisplayOnEmptyOU covers the second half of the branch guard: an empty
// organization unit must skip resolution and return an empty list rather than erroring.
func (suite *UserTreeAPITestSuite) TestGetUsersByPathWithDisplayOnEmptyOU() {
	emptyOU := testutils.OrganizationUnit{
		Handle: "tree-display-empty-ou",
		Name:   "Tree Display Empty OU",
	}
	ouID, err := testutils.CreateOrganizationUnit(emptyOU)
	suite.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(ouID) }()

	resp := suite.doTree(http.MethodGet, "/users/tree/"+emptyOU.Handle+"?include=display", nil)
	defer func() { _ = resp.Body.Close() }()
	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp testutils.UserListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))
	suite.Equal(0, listResp.TotalResults)
	suite.Empty(listResp.Users)
}

// createUserInTestOU creates a user under the suite's organization unit and returns its id.
func (suite *UserTreeAPITestSuite) createUserInTestOU(username, email string) string {
	suite.T().Helper()

	reqBody, err := json.Marshal(CreateUserByPathRequest{
		Type: "employee",
		Attributes: json.RawMessage(
			`{"username":"` + username + `","email":"` + email + `","department":"Engineering"}`),
	})
	suite.Require().NoError(err)

	resp := suite.doTree(http.MethodPost, "/users/tree/"+pathTestOU.Handle, bytes.NewBuffer(reqBody))
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusCreated, resp.StatusCode, "create failed: %s", string(body))

	var created testutils.User
	suite.Require().NoError(json.Unmarshal(body, &created))
	suite.Require().NotEmpty(created.ID)
	return created.ID
}

// findUser returns the listed user with the given id, or nil.
func (suite *UserTreeAPITestSuite) findUser(users []testutils.User, id string) *testutils.User {
	for i := range users {
		if users[i].ID == id {
			return &users[i]
		}
	}
	return nil
}

// doTree issues a request against the tree routes and returns the response.
func (suite *UserTreeAPITestSuite) doTree(method, path string, body io.Reader) *http.Response {
	suite.T().Helper()

	req, err := http.NewRequest(method, testServerURL+path, body)
	suite.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	return resp
}

// requireTreeError asserts a tree response carries the exact status and product error code.
func (suite *UserTreeAPITestSuite) requireTreeError(resp *http.Response, status int, code string) {
	suite.T().Helper()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(status, resp.StatusCode, "error body: %s", string(body))

	var errorResp testutils.ErrorResponse
	suite.Require().NoError(json.Unmarshal(body, &errorResp), "error body: %s", string(body))
	suite.Equal(code, errorResp.Code, "error body: %s", string(body))
}

// TestTreePathAndPaginationRejections verifies that a malformed handle path and out-of-range
// pagination parameters are each refused with their own exact status and product error code, rather
// than being clamped or resolved to some default subtree.
func (suite *UserTreeAPITestSuite) TestTreePathAndPaginationRejections() {
	cases := []struct {
		name string
		path string
		code string
	}{
		// Scenario 58: a path made only of whitespace is not a handle. Repeated-slash paths are
		// deliberately not tested — http.NewServeMux canonicalizes them before dispatch, so they
		// never reach the validator.
		{name: "whitespace only path", path: "/users/tree/%20", code: "USR-1009"},
		{name: "whitespace path segments", path: "/users/tree/%20%20/%20", code: "USR-1009"},

		// Scenario 59: limit must be a positive integer no greater than MaxPageSize (100).
		{name: "limit of zero", path: "/users/tree/" + pathTestOU.Handle + "?limit=0", code: "USR-1011"},
		{name: "limit above the maximum", path: "/users/tree/" + pathTestOU.Handle + "?limit=101", code: "USR-1011"},
		{name: "limit not a number", path: "/users/tree/" + pathTestOU.Handle + "?limit=abc", code: "USR-1011"},

		// Scenario 60: offset must be a non-negative integer, and carries its own code.
		{name: "negative offset", path: "/users/tree/" + pathTestOU.Handle + "?offset=-1", code: "USR-1012"},
		{name: "offset not a number", path: "/users/tree/" + pathTestOU.Handle + "?offset=abc", code: "USR-1012"},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			resp := suite.doTree(http.MethodGet, tc.path, nil)
			defer func() { _ = resp.Body.Close() }()

			suite.requireTreeError(resp, http.StatusBadRequest, tc.code)
		})
	}
}

// TestGetUsersByPathAtMaximumLimitAccepted is the control for the limit rejections above: the value
// at the top of the accepted range succeeds. Without it, "limit=101 is rejected" would also be
// satisfied by a server that rejected every limit.
func (suite *UserTreeAPITestSuite) TestGetUsersByPathAtMaximumLimitAccepted() {
	resp := suite.doTree(http.MethodGet, "/users/tree/"+pathTestOU.Handle+"?limit=100", nil)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode, "response body: %s", string(body))
}

// TestCreateUserByPathNonExistentOURejected verifies that a by-path create naming an unknown
// organization unit is refused and creates nothing. The handle path is the only place the target OU
// comes from on this route, so an unresolvable path must not fall back to a default OU.
func (suite *UserTreeAPITestSuite) TestCreateUserByPathNonExistentOURejected() {
	const username = "tree-orphan-user"

	payload, err := json.Marshal(CreateUserByPathRequest{
		Type: "employee",
		Attributes: json.RawMessage(
			`{"username":"` + username + `","email":"tree-orphan@example.com"}`),
	})
	suite.Require().NoError(err)

	resp := suite.doTree(http.MethodPost, "/users/tree/no-such-ou-handle", bytes.NewReader(payload))
	defer func() { _ = resp.Body.Close() }()

	suite.requireTreeError(resp, http.StatusNotFound, "USR-1005")

	suite.Equal(0, suite.countUsersByUsername(username),
		"a rejected by-path create must not persist a user anywhere")
}

// countUsersByUsername returns how many users carry the given username. It filters server-side
// rather than scanning a page of results, so absence cannot be reported merely by paging past a row.
func (suite *UserTreeAPITestSuite) countUsersByUsername(username string) int {
	suite.T().Helper()

	resp := suite.doTree(http.MethodGet,
		"/users?filter="+url.QueryEscape(`username eq "`+username+`"`), nil)
	defer func() { _ = resp.Body.Close() }()
	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp testutils.UserListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))
	return listResp.TotalResults
}
