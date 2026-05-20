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
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

var (
	// Test users specifically for filtering tests
	filterTestEntityTypes = []testutils.UserType{
		{
			Name: "employee",
			Schema: map[string]interface{}{
				"username": map[string]interface{}{"type": "string"},
				"email":    map[string]interface{}{"type": "string"},
				"age":      map[string]interface{}{"type": "number"},
				"department": map[string]interface{}{
					"type": "string",
				},
				"isActive": map[string]interface{}{"type": "boolean"},
				"address": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{"type": "string"},
						"zip":  map[string]interface{}{"type": "string"},
					},
				},
				"contactPreferences": map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "string"},
				},
			},
		},
		{
			Name: "customer",
			Schema: map[string]interface{}{
				"username": map[string]interface{}{"type": "string"},
				"email":    map[string]interface{}{"type": "string"},
				"age":      map[string]interface{}{"type": "number"},
				"department": map[string]interface{}{
					"type": "string",
				},
				"isActive": map[string]interface{}{"type": "boolean"},
				"address": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{"type": "string"},
						"zip":  map[string]interface{}{"type": "string"},
					},
				},
				"contactPreferences": map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "string"},
				},
			},
		},
	}

	filterTestUsers = []testutils.User{
		{
			Type:       "employee",
			Attributes: json.RawMessage(`{"username": "john.doe", "email": "john.doe@example.com", "age": 25, "department": "Engineering", "isActive": true, "address": {"city": "Mountain View", "zip": "94040"}, "contactPreferences": ["email", "sms"]}`),
		},
		{
			Type:       "employee",
			Attributes: json.RawMessage(`{"username": "jane.smith", "email": "jane.smith@example.com", "age": 30, "department": "Marketing", "isActive": false, "address": {"city": "Palo Alto", "zip": "94301"}, "contactPreferences": ["email"]}`),
		},
		{
			Type:       "customer",
			Attributes: json.RawMessage(`{"username": "bob.wilson", "email": "bob.wilson@example.com", "age": 25, "department": "Sales", "isActive": true, "address": {"city": "San Francisco", "zip": "94102"}, "contactPreferences": ["phone", "sms"]}`),
		},
		{
			Type:       "customer",
			Attributes: json.RawMessage(`{"username": "alice.brown", "email": "alice.brown@example.com", "age": 35, "department": "Engineering", "isActive": true, "address": {"city": "Mountain View", "zip": "94041"}, "contactPreferences": ["email", "phone"]}`),
		},
	}

	filterTestOU = OUCreateRequest{
		Handle:      "filter-test-ou",
		Name:        "Filter Test Organization Unit",
		Description: "Organization unit created for filter testing",
		Parent:      nil,
	}
)

var (
	filterTestOUID      string
	filterTestUserIDs   []string
	filterTestSchemaIDs []string
)

type UserFilterTestSuite struct {
	suite.Suite
}

func TestUserFilterTestSuite(t *testing.T) {
	suite.Run(t, new(UserFilterTestSuite))
}

// SetupSuite creates test organization unit and users for filtering tests
func (ts *UserFilterTestSuite) SetupSuite() {
	// Create the organization unit for filter tests
	ouID, err := createOrganizationUnit(filterTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create organization unit during filter test setup: %v", err)
	}
	filterTestOUID = ouID

	filterTestSchemaIDs = make([]string, 0, len(filterTestEntityTypes))
	for _, schema := range filterTestEntityTypes {
		schema.OUID = filterTestOUID
		schemaID, err := testutils.CreateUserType(schema)
		if err != nil {
			ts.T().Fatalf("Failed to create user type during filter test setup: %v", err)
		}
		filterTestSchemaIDs = append(filterTestSchemaIDs, schemaID)
	}

	// Create test users for filtering
	filterTestUserIDs = make([]string, 0, len(filterTestUsers))
	for _, user := range filterTestUsers {
		user.OUID = filterTestOUID
		userID, err := createUser(user)
		if err != nil {
			ts.T().Fatalf("Failed to create filter test user during setup: %v", err)
		}
		filterTestUserIDs = append(filterTestUserIDs, userID)
	}
}

// TearDownSuite cleans up filter test users and organization unit
func (ts *UserFilterTestSuite) TearDownSuite() {
	// Delete filter test users
	for _, userID := range filterTestUserIDs {
		err := deleteUser(userID)
		if err != nil {
			ts.T().Logf("Failed to delete filter test user during teardown: %v", err)
		}
	}

	// Delete the filter test organization unit
	if filterTestOUID != "" {
		err := deleteOrganizationUnit(filterTestOUID)
		if err != nil {
			ts.T().Logf("Failed to delete filter test organization unit during teardown: %v", err)
		}
	}

	for _, schemaID := range filterTestSchemaIDs {
		if schemaID == "" {
			continue
		}
		if err := testutils.DeleteUserType(schemaID); err != nil {
			ts.T().Logf("Failed to delete filter test user type during teardown: %v", err)
		}
	}
}

// Test user filtering by string attribute
func (ts *UserFilterTestSuite) TestFilterByStringAttribute() {
	// Filter by username
	filter := `username eq "john.doe"`
	users := ts.getUsersWithFilter(filter)

	ts.Require().Equal(1, len(users), "Expected exactly 1 user with username 'john.doe'")
	ts.Require().NotNil(users[0].Attributes, "User attributes should not be nil")

	var userAttrs map[string]interface{}
	err := json.Unmarshal(users[0].Attributes, &userAttrs)
	ts.NoError(err, "Failed to unmarshal user attributes")
	ts.Equal("john.doe", userAttrs["username"], "Expected username to be 'john.doe'")
}

// Test user filtering by email attribute
func (ts *UserFilterTestSuite) TestFilterByEmailAttribute() {
	// Filter by email
	filter := `email eq "jane.smith@example.com"`
	users := ts.getUsersWithFilter(filter)

	ts.Require().Equal(1, len(users), "Expected exactly 1 user with email 'jane.smith@example.com'")
	ts.Require().NotNil(users[0].Attributes, "User attributes should not be nil")

	var userAttrs map[string]interface{}
	err := json.Unmarshal(users[0].Attributes, &userAttrs)
	ts.NoError(err, "Failed to unmarshal user attributes")
	ts.Equal("jane.smith@example.com", userAttrs["email"], "Expected email to match")
}

// Test user filtering by number attribute
func (ts *UserFilterTestSuite) TestFilterByNumberAttribute() {
	// Filter by age
	filter := `age eq 25`
	users := ts.getUsersWithFilter(filter)

	ts.GreaterOrEqual(len(users), 2, "Expected at least 2 users with age 25")

	for i, user := range users {
		ts.Require().NotNil(user.Attributes, "User attributes should not be nil for user %d", i)

		var userAttrs map[string]interface{}
		err := json.Unmarshal(user.Attributes, &userAttrs)
		ts.NoError(err, "Failed to unmarshal user attributes for user %d", i)
		ts.Equal(float64(25), userAttrs["age"], "Expected age to be 25 for user %d", i)
	}
}

// Test user filtering by boolean attribute
func (ts *UserFilterTestSuite) TestFilterByBooleanAttribute() {
	// Filter by isActive = true
	filter := `isActive eq true`
	users := ts.getUsersWithFilter(filter)

	ts.GreaterOrEqual(len(users), 3, "Expected at least 3 active users")

	for i, user := range users {
		ts.Require().NotNil(user.Attributes, "User attributes should not be nil for user %d", i)

		var userAttrs map[string]interface{}
		err := json.Unmarshal(user.Attributes, &userAttrs)
		ts.NoError(err, "Failed to unmarshal user attributes for user %d", i)
		ts.True(userAttrs["isActive"].(bool), "Expected isActive to be true for user %d", i)
	}

	// Filter by isActive = false
	filter = `isActive eq false`
	users = ts.getUsersWithFilter(filter)

	ts.Require().Equal(1, len(users), "Expected exactly 1 inactive user")
	ts.Require().NotNil(users[0].Attributes, "User attributes should not be nil")

	var userAttrs map[string]interface{}
	err := json.Unmarshal(users[0].Attributes, &userAttrs)
	ts.NoError(err, "Failed to unmarshal user attributes")
	ts.False(userAttrs["isActive"].(bool), "Expected isActive to be false")
}

// Test user filtering by nested object attribute
func (ts *UserFilterTestSuite) TestFilterByNestedObjectAttribute() {
	// Filter by address.city
	filter := `address.city eq "Mountain View"`
	users := ts.getUsersWithFilter(filter)

	ts.Equal(2, len(users), "Expected exactly 2 users in Mountain View")

	for i, user := range users {
		ts.Require().NotNil(user.Attributes, "User attributes should not be nil for user %d", i)

		var userAttrs map[string]interface{}
		err := json.Unmarshal(user.Attributes, &userAttrs)
		ts.NoError(err, "Failed to unmarshal user attributes for user %d", i)

		address := userAttrs["address"].(map[string]interface{})
		ts.Equal("Mountain View", address["city"], "Expected city to be 'Mountain View' for user %d", i)
	}
}

// Test user filtering by nested zip code
func (ts *UserFilterTestSuite) TestFilterByNestedZipCode() {
	// Filter by address.zip
	filter := `address.zip eq "94040"`
	users := ts.getUsersWithFilter(filter)

	ts.Require().Equal(1, len(users), "Expected exactly 1 user with zip code '94040'")
	ts.Require().NotNil(users[0].Attributes, "User attributes should not be nil")

	var userAttrs map[string]interface{}
	err := json.Unmarshal(users[0].Attributes, &userAttrs)
	ts.NoError(err, "Failed to unmarshal user attributes")

	address := userAttrs["address"].(map[string]interface{})
	ts.Equal("94040", address["zip"], "Expected zip code to be '94040'")
}

// Test user filtering by department (enum-like attribute)
func (ts *UserFilterTestSuite) TestFilterByDepartment() {
	// Filter by department = "Engineering"
	filter := `department eq "Engineering"`
	users := ts.getUsersWithFilter(filter)

	ts.Equal(2, len(users), "Expected exactly 2 users in Engineering department")

	for i, user := range users {
		ts.Require().NotNil(user.Attributes, "User attributes should not be nil for user %d", i)

		var userAttrs map[string]interface{}
		err := json.Unmarshal(user.Attributes, &userAttrs)
		ts.NoError(err, "Failed to unmarshal user attributes for user %d", i)
		ts.Equal("Engineering", userAttrs["department"], "Expected department to be 'Engineering' for user %d", i)
	}
}

// Test user filtering with no results
func (ts *UserFilterTestSuite) TestFilterNoResults() {
	// Filter for non-existent username
	filter := `username eq "nonexistent.user"`
	users := ts.getUsersWithFilter(filter)

	ts.Equal(0, len(users), "Expected no users for non-existent username")
}

// Test user filtering with pagination
func (ts *UserFilterTestSuite) TestFilterWithPagination() {
	// Filter active users with pagination
	filter := `isActive eq true`

	req, err := http.NewRequest("GET", testServerURL+"/users", nil)
	ts.NoError(err, "Failed to create request")

	// Add query parameters
	q := url.Values{}
	q.Add("filter", filter)
	q.Add("limit", "2")
	q.Add("offset", "0")
	req.URL.RawQuery = q.Encode()

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.NoError(err, "Failed to send request")
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode, "Expected status 200")

	var userListResponse testutils.UserListResponse
	err = json.NewDecoder(resp.Body).Decode(&userListResponse)
	ts.NoError(err, "Failed to parse response body")

	ts.LessOrEqual(len(userListResponse.Users), 2, "Expected at most 2 users with limit=2")
	ts.GreaterOrEqual(userListResponse.TotalResults, 3, "Expected at least 3 total active users")

	// Verify all returned users match the filter
	for i, user := range userListResponse.Users {
		ts.Require().NotNil(user.Attributes, "User attributes should not be nil for user %d", i)

		var userAttrs map[string]interface{}
		err := json.Unmarshal(user.Attributes, &userAttrs)
		ts.NoError(err, "Failed to unmarshal user attributes for user %d", i)
		ts.True(userAttrs["isActive"].(bool), "Expected all returned users to be active for user %d", i)
	}
}

// Test invalid filter formats
func (ts *UserFilterTestSuite) TestInvalidFilterFormats() {
	invalidFilters := []string{
		"username invalid format", // Missing operator
		"username eq",             // Missing value
		"eq john.doe",             // Missing attribute
		"username = john.doe",     // Wrong operator
		"username eq john.doe",    // Missing quotes for string
		"age eq abc",              // Invalid number
		"",                        // Empty filter
	}

	client := testutils.GetHTTPClient()

	for _, filter := range invalidFilters {
		req, err := http.NewRequest("GET", testServerURL+"/users", nil)
		ts.NoError(err, "Failed to create request")

		q := url.Values{}
		q.Add("filter", filter)
		req.URL.RawQuery = q.Encode()

		resp, err := client.Do(req)
		ts.NoError(err, "Failed to send request")

		ts.Equal(http.StatusBadRequest, resp.StatusCode,
			"Expected status 400 for invalid filter: %s", filter)

		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		if err == nil {
			ts.Equal("USR-1020", errorResp["code"], "Expected error code USR-1020")
			message, ok := errorResp["message"].(map[string]interface{})
			ts.True(ok, "Expected message to be an i18n object")
			ts.Contains(message["defaultValue"], "Invalid filter parameter", "Expected filter error message")
		}

		resp.Body.Close()
	}
}

// Test filter case sensitivity
func (ts *UserFilterTestSuite) TestFilterCaseSensitivity() {
	// Test uppercase - should return no results
	filter := `username eq "JOHN.DOE"`
	users := ts.getUsersWithFilter(filter)
	ts.Equal(0, len(users), "Filter should be case-sensitive")

	// Test correct case - should return 1 result
	filter = `username eq "john.doe"`
	users = ts.getUsersWithFilter(filter)
	ts.Equal(1, len(users), "Expected 1 user with correct case")
}

// Test multiple matching users
func (ts *UserFilterTestSuite) TestMultipleMatchingUsers() {
	// Filter by department that has multiple users
	filter := `department eq "Engineering"`
	users := ts.getUsersWithFilter(filter)

	ts.Equal(2, len(users), "Expected exactly 2 users in Engineering")

	expectedUsernames := []string{"john.doe", "alice.brown"}
	actualUsernames := make([]string, len(users))

	for i, user := range users {
		ts.Require().NotNil(user.Attributes, "User attributes should not be nil for user %d", i)

		var userAttrs map[string]interface{}
		err := json.Unmarshal(user.Attributes, &userAttrs)
		ts.NoError(err, "Failed to unmarshal user attributes for user %d", i)
		actualUsernames[i] = userAttrs["username"].(string)
	}

	ts.ElementsMatch(expectedUsernames, actualUsernames, "Expected specific usernames to match")
}

// Helper method to get users with filter
func (ts *UserFilterTestSuite) getUsersWithFilter(filter string) []testutils.User {
	req, err := http.NewRequest("GET", testServerURL+"/users", nil)
	ts.NoError(err, "Failed to create request")

	q := url.Values{}
	q.Add("filter", filter)
	req.URL.RawQuery = q.Encode()

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.NoError(err, "Failed to send request")
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for filter request")

	var userListResponse testutils.UserListResponse
	err = json.NewDecoder(resp.Body).Decode(&userListResponse)
	ts.NoError(err, "Failed to parse response body")

	return userListResponse.Users
}
