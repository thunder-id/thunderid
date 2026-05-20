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

const (
	credentialsAuthEndpoint = "/auth/credentials/authenticate"
)

var (
	indexedAttributesTestOU = testutils.OrganizationUnit{
		Handle:      "indexed-attrs-test-ou",
		Name:        "Indexed Attributes Test Organization Unit",
		Description: "Organization unit for indexed attributes testing",
		Parent:      nil,
	}

	indexedAttributesEntityTypes = map[string]testutils.UserType{
		"all_indexed": {
			Name: "all_indexed",
			Schema: map[string]interface{}{
				"username": map[string]interface{}{
					"type": "string",
				},
				"email": map[string]interface{}{
					"type": "string",
				},
				"mobileNumber": map[string]interface{}{
					"type": "string",
				},
				"sub": map[string]interface{}{
					"type": "string",
				},
				"password": map[string]interface{}{
					"type": "string",
					"credential": true,
				},
			},
		},
		"partial_indexed": {
			Name: "partial_indexed",
			Schema: map[string]interface{}{
				"username": map[string]interface{}{
					"type": "string",
				},
				"email": map[string]interface{}{
					"type": "string",
				},
				"displayName": map[string]interface{}{
					"type": "string",
				},
				"password": map[string]interface{}{
					"type": "string",
					"credential": true,
				},
			},
		},
		"no_indexed": {
			Name: "no_indexed",
			Schema: map[string]interface{}{
				"displayName": map[string]interface{}{
					"type": "string",
				},
				"department": map[string]interface{}{
					"type": "string",
				},
				"password": map[string]interface{}{
					"type": "string",
					"credential": true,
				},
			},
		},
		"mixed_types": {
			Name: "mixed_types",
			Schema: map[string]interface{}{
				"username": map[string]interface{}{
					"type": "string",
				},
				"age": map[string]interface{}{
					"type": "number",
				},
				"active": map[string]interface{}{
					"type": "boolean",
				},
				"metadata": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"key": map[string]interface{}{"type": "string"},
					},
				},
				"tags": map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "string"},
				},
				"password": map[string]interface{}{
					"type": "string",
					"credential": true,
				},
			},
		},
	}
)

type IndexedAttributesTestSuite struct {
	suite.Suite
	client        *http.Client
	entityTypeIDs map[string]string
	ouID          string
}

func TestIndexedAttributesTestSuite(t *testing.T) {
	suite.Run(t, new(IndexedAttributesTestSuite))
}

func (suite *IndexedAttributesTestSuite) SetupSuite() {
	suite.client = testutils.GetHTTPClient()
	suite.entityTypeIDs = make(map[string]string)

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(indexedAttributesTestOU)
	if err != nil {
		suite.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	suite.ouID = ouID

	// Create user types
	for userType, schema := range indexedAttributesEntityTypes {
		schema.OUID = suite.ouID
		schemaID, err := testutils.CreateUserType(schema)
		if err != nil {
			suite.T().Fatalf("Failed to create user type %s during setup: %v", userType, err)
		}
		suite.entityTypeIDs[userType] = schemaID
	}
}

func (suite *IndexedAttributesTestSuite) TearDownSuite() {
	// All users are cleaned up by individual test defers

	// Delete user types
	for userType, schemaID := range suite.entityTypeIDs {
		if schemaID != "" {
			err := testutils.DeleteUserType(schemaID)
			if err != nil {
				suite.T().Errorf("Failed to delete user type %s during teardown: %v", userType, err)
			}
		}
	}

	// Delete test organization unit
	if suite.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
			suite.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// Test Suite 1: User Creation with Indexed Attributes

func (suite *IndexedAttributesTestSuite) TestCreateUserWithAllIndexedAttributes() {
	attributes := map[string]interface{}{
		"username":     "indexed_user1",
		"email":        "indexed1@example.com",
		"mobileNumber": "+1234567890",
		"sub":          "user-sub-123",
		"password":     "SecurePass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "all_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err, "Failed to create user with all indexed attributes")
	suite.NotEmpty(userID, "User ID should not be empty")

	// Cleanup after test
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()

	// Verify user can be retrieved
	retrievedUser, err := suite.getUser(userID)
	suite.Require().NoError(err)
	suite.Equal(userID, retrievedUser.ID)
	suite.Equal("all_indexed", retrievedUser.Type)
}

func (suite *IndexedAttributesTestSuite) TestCreateUserWithPartialIndexedAttributes() {
	attributes := map[string]interface{}{
		"username":    "indexed_user2",
		"email":       "indexed2@example.com",
		"displayName": "Jane Smith",
		"password":    "Pass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "partial_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err, "Failed to create user with partial indexed attributes")
	suite.NotEmpty(userID, "User ID should not be empty")

	// Cleanup after test
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()

	// Verify user was created
	retrievedUser, err := suite.getUser(userID)
	suite.Require().NoError(err)
	suite.Equal(userID, retrievedUser.ID)
}

func (suite *IndexedAttributesTestSuite) TestCreateUserWithNoIndexedAttributes() {
	attributes := map[string]interface{}{
		"displayName": "Bob Jones",
		"department":  "Engineering",
		"password":    "Pass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "no_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err, "Failed to create user with no indexed attributes")
	suite.NotEmpty(userID, "User ID should not be empty")

	// Cleanup after test
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()
}

func (suite *IndexedAttributesTestSuite) TestCreateUserWithComplexTypes() {
	attributes := map[string]interface{}{
		"username": "indexed_user_complex",
		"age":      30,
		"active":   true,
		"metadata": map[string]interface{}{"key": "value"},
		"tags":     []string{"tag1", "tag2"},
		"password": "Pass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "mixed_types",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err, "Failed to create user with complex types")
	suite.NotEmpty(userID, "User ID should not be empty")

	// Cleanup after test
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()

	// Verify user was created and all attributes preserved
	retrievedUser, err := suite.getUser(userID)
	suite.Require().NoError(err)

	var retrievedAttrs map[string]interface{}
	err = json.Unmarshal(retrievedUser.Attributes, &retrievedAttrs)
	suite.Require().NoError(err)

	suite.Equal("indexed_user_complex", retrievedAttrs["username"])
	suite.Equal(float64(30), retrievedAttrs["age"])
	suite.Equal(true, retrievedAttrs["active"])
}

// Test Suite 2: User Update with Indexed Attributes

func (suite *IndexedAttributesTestSuite) TestUpdateUserAddIndexedAttribute() {
	// Create user without email
	attributes := map[string]interface{}{
		"username":    "update_user1",
		"displayName": "Alice",
		"password":    "Pass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "partial_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err)

	// Cleanup after test
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()

	// Update user to add email (indexed attribute)
	updatedAttributes := map[string]interface{}{
		"username":    "update_user1",
		"email":       "alice@example.com",
		"displayName": "Alice Updated",
		"password":    "Pass123!",
	}

	updatedAttrsJSON, err := json.Marshal(updatedAttributes)
	suite.Require().NoError(err)

	updatedUser := testutils.User{
		Type:             "partial_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(updatedAttrsJSON),
	}

	err = suite.updateUser(userID, updatedUser)
	suite.Require().NoError(err, "Failed to update user with new indexed attribute")

	// Verify update
	retrievedUser, err := suite.getUser(userID)
	suite.Require().NoError(err)

	var retrievedAttrs map[string]interface{}
	err = json.Unmarshal(retrievedUser.Attributes, &retrievedAttrs)
	suite.Require().NoError(err)
	suite.Equal("alice@example.com", retrievedAttrs["email"])
	suite.Equal("Alice Updated", retrievedAttrs["displayName"])
}

func (suite *IndexedAttributesTestSuite) TestUpdateUserModifyIndexedAttributeValue() {
	// Create user with email
	attributes := map[string]interface{}{
		"username": "update_user2",
		"email":    "bob@old.com",
		"password": "Pass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "partial_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err)

	// Cleanup after test
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()

	// Update email to new value
	updatedAttributes := map[string]interface{}{
		"username": "update_user2",
		"email":    "bob@new.com",
		"password": "Pass123!",
	}

	updatedAttrsJSON, err := json.Marshal(updatedAttributes)
	suite.Require().NoError(err)

	updatedUser := testutils.User{
		Type:             "partial_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(updatedAttrsJSON),
	}

	err = suite.updateUser(userID, updatedUser)
	suite.Require().NoError(err, "Failed to update user with modified indexed attribute")

	// Verify update
	retrievedUser, err := suite.getUser(userID)
	suite.Require().NoError(err)

	var retrievedAttrs map[string]interface{}
	err = json.Unmarshal(retrievedUser.Attributes, &retrievedAttrs)
	suite.Require().NoError(err)
	suite.Equal("bob@new.com", retrievedAttrs["email"])
}

func (suite *IndexedAttributesTestSuite) TestUpdateUserRemoveIndexedAttribute() {
	// Create user with username, email, and mobileNumber
	attributes := map[string]interface{}{
		"username":     "update_user3",
		"email":        "charlie@example.com",
		"mobileNumber": "+9876543210",
		"sub":          "sub-charlie",
		"password":     "Pass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "all_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err)

	// Cleanup after test
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()

	// Update user removing mobileNumber
	updatedAttributes := map[string]interface{}{
		"username": "update_user3",
		"email":    "charlie@example.com",
		"sub":      "sub-charlie",
		"password": "Pass123!",
	}

	updatedAttrsJSON, err := json.Marshal(updatedAttributes)
	suite.Require().NoError(err)

	updatedUser := testutils.User{
		Type:             "all_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(updatedAttrsJSON),
	}

	err = suite.updateUser(userID, updatedUser)
	suite.Require().NoError(err, "Failed to update user with removed indexed attribute")

	// Verify update - mobileNumber should be gone
	retrievedUser, err := suite.getUser(userID)
	suite.Require().NoError(err)

	var retrievedAttrs map[string]interface{}
	err = json.Unmarshal(retrievedUser.Attributes, &retrievedAttrs)
	suite.Require().NoError(err)
	suite.Equal("charlie@example.com", retrievedAttrs["email"])
	_, hasMobileNumber := retrievedAttrs["mobileNumber"]
	suite.False(hasMobileNumber, "mobileNumber should be removed")
}

// Test Suite 3: Authentication with Indexed Attributes

func (suite *IndexedAttributesTestSuite) TestAuthenticateWithSingleIndexedAttributeUsername() {
	// Create user for authentication
	attributes := map[string]interface{}{
		"username": "auth_user1",
		"email":    "auth1@test.com",
		"password": "TestPass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "partial_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err)

	// Cleanup after test
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()

	// Authenticate using username
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "auth_user1",
		},
		"credentials": map[string]interface{}{
			"password": "TestPass123!",
		},
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")
	suite.Equal(userID, response.ID)
	suite.Equal("partial_indexed", response.Type)
}

func (suite *IndexedAttributesTestSuite) TestAuthenticateWithSingleIndexedAttributeEmail() {
	// Create user for authentication
	attributes := map[string]interface{}{
		"username": "auth_user_email",
		"email":    "auth_email@test.com",
		"password": "TestPass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "partial_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err)

	// Cleanup after test
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()

	// Authenticate using email
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"email": "auth_email@test.com",
		},
		"credentials": map[string]interface{}{
			"password": "TestPass123!",
		},
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")
	suite.Equal(userID, response.ID)
}

func (suite *IndexedAttributesTestSuite) TestAuthenticateWithSingleIndexedAttributeMobileNumber() {
	// Create user with mobile number
	attributes := map[string]interface{}{
		"username":     "auth_user2",
		"email":        "auth2@test.com",
		"mobileNumber": "+1111111111",
		"password":     "TestPass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "all_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err)

	// Cleanup after test
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()

	// Authenticate using mobile number
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"mobileNumber": "+1111111111",
		},
		"credentials": map[string]interface{}{
			"password": "TestPass123!",
		},
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")
	suite.Equal(userID, response.ID)
}

func (suite *IndexedAttributesTestSuite) TestAuthenticateWithMultipleIndexedAttributes() {
	// Create user with all indexed attributes
	attributes := map[string]interface{}{
		"username":     "auth_user3",
		"email":        "auth3@test.com",
		"mobileNumber": "+2222222222",
		"sub":          "sub-auth3",
		"password":     "TestPass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "all_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err)

	// Cleanup after test
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()

	// Authenticate using all indexed attributes
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username":     "auth_user3",
			"email":        "auth3@test.com",
			"mobileNumber": "+2222222222",
			"sub":          "sub-auth3",
		},
		"credentials": map[string]interface{}{
			"password": "TestPass123!",
		},
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")
	suite.Equal(userID, response.ID)
}

func (suite *IndexedAttributesTestSuite) TestAuthenticateWithMixedIndexedAndNonIndexedAttributes() {
	// Create user with both indexed and non-indexed attributes
	attributes := map[string]interface{}{
		"username":    "auth_user4",
		"email":       "auth4@test.com",
		"displayName": "Auth User 4",
		"password":    "TestPass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "partial_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err)

	// Cleanup after test
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()

	// Authenticate using indexed + non-indexed attributes (hybrid query)
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username":    "auth_user4",
			"displayName": "Auth User 4",
		},
		"credentials": map[string]interface{}{
			"password": "TestPass123!",
		},
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")
	suite.Equal(userID, response.ID)
}

func (suite *IndexedAttributesTestSuite) TestAuthenticateWithOnlyNonIndexedAttributes() {
	// Create user with only non-indexed attributes
	attributes := map[string]interface{}{
		"displayName": "Auth User 5",
		"department":  "Sales",
		"password":    "TestPass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "no_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err)

	// Cleanup after test
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()

	// Authenticate using only non-indexed attributes (fallback to JSON query)
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"displayName": "Auth User 5",
			"department":  "Sales",
		},
		"credentials": map[string]interface{}{
			"password": "TestPass123!",
		},
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")
	suite.Equal(userID, response.ID)
}

func (suite *IndexedAttributesTestSuite) TestAuthenticateWithDifferentIndexedAttributeVariations() {
	// Create user with all indexed attributes
	attributes := map[string]interface{}{
		"username":     "auth_user_variations",
		"email":        "auth_variations@test.com",
		"mobileNumber": "+3333333333",
		"sub":          "sub-variations",
		"password":     "TestPass123!",
	}

	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             "all_indexed",
		OUID:             suite.ouID,
		Attributes:       json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err)

	// Cleanup after all subtests
	defer func() {
		if userID != "" {
			testutils.DeleteUser(userID)
		}
	}()

	testCases := []struct {
		name        string
		authRequest map[string]interface{}
	}{
		{
			name: "Username only",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"username": "auth_user_variations",
				},
				"credentials": map[string]interface{}{
					"password": "TestPass123!",
				},
			},
		},
		{
			name: "Email only",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"email": "auth_variations@test.com",
				},
				"credentials": map[string]interface{}{
					"password": "TestPass123!",
				},
			},
		},
		{
			name: "MobileNumber only",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"mobileNumber": "+3333333333",
				},
				"credentials": map[string]interface{}{
					"password": "TestPass123!",
				},
			},
		},
		{
			name: "Sub only",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"sub": "sub-variations",
				},
				"credentials": map[string]interface{}{
					"password": "TestPass123!",
				},
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			response, statusCode, err := suite.sendAuthRequest(tc.authRequest)
			suite.Require().NoError(err)
			suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")
			suite.Equal(userID, response.ID)
		})
	}
}

// Helper methods

func (suite *IndexedAttributesTestSuite) getUser(userID string) (*testutils.User, error) {
	req, err := http.NewRequest("GET", testutils.TestServerURL+"/users/"+userID, nil)
	if err != nil {
		return nil, err
	}

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user, status code: %d", resp.StatusCode)
	}

	var user testutils.User
	err = json.NewDecoder(resp.Body).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (suite *IndexedAttributesTestSuite) updateUser(userID string, user testutils.User) error {
	requestJSON, err := json.Marshal(user)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", testutils.TestServerURL+"/users/"+userID, bytes.NewReader(requestJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update user, status code: %d", resp.StatusCode)
	}

	return nil
}

func (suite *IndexedAttributesTestSuite) sendAuthRequest(authRequest map[string]interface{}) (
	*testutils.AuthenticationResponse, int, error) {
	requestJSON, err := json.Marshal(authRequest)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequest("POST", testutils.TestServerURL+credentialsAuthEndpoint,
		bytes.NewReader(requestJSON))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var response testutils.AuthenticationResponse
	bodyBytes, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return &response, resp.StatusCode, nil
}
