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

package resource

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

type ActionAPITestSuite struct {
	suite.Suite
	ouID             string
	resourceServerID string
	resourceID       string
}

func TestActionAPITestSuite(t *testing.T) {
	suite.Run(t, new(ActionAPITestSuite))
}

func (suite *ActionAPITestSuite) SetupSuite() {
	// Create test organization unit
	ou := testutils.OrganizationUnit{
		Handle:      "test_action_api_ou",
		Name:        "Test OU for Action API",
		Description: "Organization unit for action API testing",
		Parent:      nil,
	}
	ouID, err := testutils.CreateOrganizationUnit(ou)
	suite.Require().NoError(err, "Failed to create test organization unit")
	suite.ouID = ouID

	// Create test resource server
	rsReq := CreateResourceServerRequest{
		Name:               "Action Test Server",
		Description:        "Resource server for action testing",
		Handle:             "action-test-server",
		OUID: ouID,
	}
	rsID, err := createResourceServer(rsReq)
	suite.Require().NoError(err, "Failed to create test resource server")
	suite.resourceServerID = rsID

	// Create test resource
	resReq := CreateResourceRequest{
		Name:        "Test Resource",
		Handle:      "test-resource",
		Description: "Resource for action testing",
		Parent:      nil,
	}
	resID, err := createResource(rsID, resReq)
	suite.Require().NoError(err, "Failed to create test resource")
	suite.resourceID = resID
}

func (suite *ActionAPITestSuite) TearDownSuite() {
	if suite.resourceID != "" {
		deleteResource(suite.resourceServerID, suite.resourceID)
	}
	if suite.resourceServerID != "" {
		deleteResourceServer(suite.resourceServerID)
	}
	if suite.ouID != "" {
		testutils.DeleteOrganizationUnit(suite.ouID)
	}
}

// Action at Resource Server Level Tests

func (suite *ActionAPITestSuite) TestCreateActionAtResourceServer() {
	req := CreateActionRequest{
		Name:        "Read Action",
		Handle:      "read",
		Description: "Read action at server level",
	}

	actionID, err := createActionAtResourceServer(suite.resourceServerID, req)
	suite.Require().NoError(err, "Failed to create action at resource server")
	suite.NotEmpty(actionID)

	defer deleteAction(suite.resourceServerID, actionID)

	// Verify the created action
	action, err := getActionAtResourceServer(suite.resourceServerID, actionID)
	suite.Require().NoError(err)
	suite.Equal(req.Name, action.Name)
	suite.Equal(req.Handle, action.Handle)
	suite.Equal(req.Description, action.Description)
	suite.Equal("action-test-server:read", action.Permission, "Server-level action permission should be just the handle")
}

func (suite *ActionAPITestSuite) TestCreateActionAtResourceServerDuplicateHandle() {
	req := CreateActionRequest{
		Name:        "Duplicate Action",
		Handle:      "duplicate-action",
		Description: "First action",
	}

	actionID1, err := createActionAtResourceServer(suite.resourceServerID, req)
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, actionID1)

	// Try to create another action with the same handle at the same level
	req2 := CreateActionRequest{
		Name:        "Different Name",
		Handle:      "duplicate-action",
		Description: "Second action with same handle",
	}
	_, err = createActionAtResourceServer(suite.resourceServerID, req2)
	suite.Error(err, "Should fail with duplicate handle")
	suite.Contains(err.Error(), "409")
}

func (suite *ActionAPITestSuite) TestGetActionAtResourceServer() {
	req := CreateActionRequest{
		Name:        "write",
		Handle:      "write",
		Description: "Write action",
	}

	actionID, err := createActionAtResourceServer(suite.resourceServerID, req)
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, actionID)

	action, err := getActionAtResourceServer(suite.resourceServerID, actionID)
	suite.Require().NoError(err)
	suite.Equal(actionID, action.ID)
	suite.Equal(req.Name, action.Name)
	suite.Equal(req.Description, action.Description)
}

func (suite *ActionAPITestSuite) TestGetActionAtResourceServerNotFound() {
	_, err := getActionAtResourceServer(suite.resourceServerID, "00000000-0000-0000-0000-000000000000")
	suite.Error(err)
	suite.Contains(err.Error(), "404")
}

func (suite *ActionAPITestSuite) TestListActionsAtResourceServer() {
	// Create multiple actions
	action1 := CreateActionRequest{
		Name:        "List Action 1",
		Handle:      "list-action-1",
		Description: "First action",
	}
	action1ID, err := createActionAtResourceServer(suite.resourceServerID, action1)
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, action1ID)

	action2 := CreateActionRequest{
		Name:        "List Action 2",
		Handle:      "list-action-2",
		Description: "Second action",
	}
	action2ID, err := createActionAtResourceServer(suite.resourceServerID, action2)
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, action2ID)

	// List actions
	list, err := listActionsAtResourceServer(suite.resourceServerID, 0, 100)
	suite.Require().NoError(err)
	suite.GreaterOrEqual(list.TotalResults, 2)
	suite.Equal(1, list.StartIndex)

	// Verify our actions are in the list
	foundAction1 := false
	foundAction2 := false
	for _, action := range list.Actions {
		if action.ID == action1ID {
			foundAction1 = true
			suite.Equal(action1.Name, action.Name)
			suite.Equal("action-test-server:list-action-1", action.Permission, "Permission should be returned in list response")
		}
		if action.ID == action2ID {
			foundAction2 = true
			suite.Equal(action2.Name, action.Name)
			suite.Equal("action-test-server:list-action-2", action.Permission, "Permission should be returned in list response")
		}
	}
	suite.True(foundAction1, "Should find first action")
	suite.True(foundAction2, "Should find second action")
}

func (suite *ActionAPITestSuite) TestUpdateActionAtResourceServer() {
	req := CreateActionRequest{
		Name:        "Update Action",
		Handle:      "update-action",
		Description: "Original description",
	}

	actionID, err := createActionAtResourceServer(suite.resourceServerID, req)
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, actionID)

	// Update action - name is mutable, handle is immutable
	updateReq := UpdateActionRequest{
		Name:        "Updated Action Name",
		Description: "Updated description",
	}
	err = updateAction(suite.resourceServerID, actionID, updateReq)
	suite.Require().NoError(err)

	// Verify update
	action, err := getActionAtResourceServer(suite.resourceServerID, actionID)
	suite.Require().NoError(err)
	suite.Equal(updateReq.Name, action.Name, "Name should be mutable")
	suite.Equal(req.Handle, action.Handle, "Handle should be immutable")
	suite.Equal(updateReq.Description, action.Description)
	suite.Equal("action-test-server:update-action", action.Permission, "Permission should be immutable")
}

func (suite *ActionAPITestSuite) TestDeleteActionAtResourceServer() {
	req := CreateActionRequest{
		Name:        "Delete Action",
		Handle:      "delete-action",
		Description: "Action to delete",
	}

	actionID, err := createActionAtResourceServer(suite.resourceServerID, req)
	suite.Require().NoError(err)

	err = deleteAction(suite.resourceServerID, actionID)
	suite.Require().NoError(err)

	// Verify action is deleted
	_, err = getActionAtResourceServer(suite.resourceServerID, actionID)
	suite.Error(err)
	suite.Contains(err.Error(), "404")
}

// Action at Resource Level Tests

func (suite *ActionAPITestSuite) TestCreateActionAtResource() {
	req := CreateActionRequest{
		Name:        "view",
		Handle:      "view",
		Description: "View action at resource level",
	}

	actionID, err := createActionAtResource(suite.resourceServerID, suite.resourceID, req)
	suite.Require().NoError(err, "Failed to create action at resource")
	suite.NotEmpty(actionID)

	defer deleteActionAtResource(suite.resourceServerID, suite.resourceID, actionID)

	// Verify the created action
	action, err := getActionAtResource(suite.resourceServerID, suite.resourceID, actionID)
	suite.Require().NoError(err)
	suite.Equal(req.Name, action.Name)
	suite.Equal(req.Description, action.Description)
	suite.Equal("action-test-server:test-resource:view", action.Permission, "Action permission should be resource:action")
}

func (suite *ActionAPITestSuite) TestCreateActionAtResourceDuplicateHandle() {
	req := CreateActionRequest{
		Name:        "First Action",
		Handle:      "duplicate-resource-action",
		Description: "First action",
	}

	actionID1, err := createActionAtResource(suite.resourceServerID, suite.resourceID, req)
	suite.Require().NoError(err)
	defer deleteActionAtResource(suite.resourceServerID, suite.resourceID, actionID1)

	// Try to create another action with the same handle at the same resource
	req2 := CreateActionRequest{
		Name:        "Second Action",
		Handle:      "duplicate-resource-action",
		Description: "Second action with same handle",
	}
	_, err = createActionAtResource(suite.resourceServerID, suite.resourceID, req2)
	suite.Error(err, "Should fail with duplicate handle")
	suite.Contains(err.Error(), "409")
}

func (suite *ActionAPITestSuite) TestCreateActionSameHandleDifferentResources() {
	// Create two resources
	resource1Req := CreateResourceRequest{
		Name:        "Resource 1",
		Handle:      "resource-1",
		Description: "First resource",
		Parent:      nil,
	}
	resource1ID, err := createResource(suite.resourceServerID, resource1Req)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resource1ID)

	resource2Req := CreateResourceRequest{
		Name:        "Resource 2",
		Handle:      "resource-2",
		Description: "Second resource",
		Parent:      nil,
	}
	resource2ID, err := createResource(suite.resourceServerID, resource2Req)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resource2ID)

	// Create action with same handle under different resources - should succeed
	action1Req := CreateActionRequest{
		Name:        "Read Action Resource 1",
		Handle:      "read",
		Description: "Action at resource 1",
	}
	action1ID, err := createActionAtResource(suite.resourceServerID, resource1ID, action1Req)
	suite.Require().NoError(err)
	defer deleteActionAtResource(suite.resourceServerID, resource1ID, action1ID)

	action2Req := CreateActionRequest{
		Name:        "Read Action Resource 2",
		Handle:      "read",
		Description: "Action at resource 2",
	}
	action2ID, err := createActionAtResource(suite.resourceServerID, resource2ID, action2Req)
	suite.Require().NoError(err, "Should allow same handle under different resources")
	defer deleteActionAtResource(suite.resourceServerID, resource2ID, action2ID)

	// Verify both exist with same handle but different resources
	action1, err := getActionAtResource(suite.resourceServerID, resource1ID, action1ID)
	suite.Require().NoError(err)
	suite.Equal("read", action1.Handle)

	action2, err := getActionAtResource(suite.resourceServerID, resource2ID, action2ID)
	suite.Require().NoError(err)
	suite.Equal("read", action2.Handle)
}

func (suite *ActionAPITestSuite) TestCreateActionSameHandleDifferentLevels() {
	// Create action at resource server level
	req := CreateActionRequest{
		Name:        "Shared Handle Action Server",
		Handle:      "shared-handle-action",
		Description: "Action at server level",
	}
	serverActionID, err := createActionAtResourceServer(suite.resourceServerID, req)
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, serverActionID)

	// Create action with same handle at resource level - should succeed
	req2 := CreateActionRequest{
		Name:        "Shared Handle Action Resource",
		Handle:      "shared-handle-action",
		Description: "Action at resource level",
	}
	resourceActionID, err := createActionAtResource(suite.resourceServerID, suite.resourceID, req2)
	suite.Require().NoError(err, "Should allow same handle at different levels")
	defer deleteActionAtResource(suite.resourceServerID, suite.resourceID, resourceActionID)

	// Verify both exist
	_, err = getActionAtResourceServer(suite.resourceServerID, serverActionID)
	suite.Require().NoError(err)

	_, err = getActionAtResource(suite.resourceServerID, suite.resourceID, resourceActionID)
	suite.Require().NoError(err)
}

func (suite *ActionAPITestSuite) TestGetActionAtResource() {
	req := CreateActionRequest{
		Name:        "edit",
		Handle:      "edit",
		Description: "Edit action",
	}

	actionID, err := createActionAtResource(suite.resourceServerID, suite.resourceID, req)
	suite.Require().NoError(err)
	defer deleteActionAtResource(suite.resourceServerID, suite.resourceID, actionID)

	action, err := getActionAtResource(suite.resourceServerID, suite.resourceID, actionID)
	suite.Require().NoError(err)
	suite.Equal(actionID, action.ID)
	suite.Equal(req.Name, action.Name)
	suite.Equal(req.Description, action.Description)
}

func (suite *ActionAPITestSuite) TestGetActionAtResourceNotFound() {
	_, err := getActionAtResource(suite.resourceServerID, suite.resourceID, "00000000-0000-0000-0000-000000000000")
	suite.Error(err)
	suite.Contains(err.Error(), "404")
}

func (suite *ActionAPITestSuite) TestListActionsAtResource() {
	// Create multiple actions at resource level
	action1 := CreateActionRequest{
		Name:        "Resource Action 1",
		Handle:      "resource-action-1",
		Description: "First resource action",
	}
	action1ID, err := createActionAtResource(suite.resourceServerID, suite.resourceID, action1)
	suite.Require().NoError(err)
	defer deleteActionAtResource(suite.resourceServerID, suite.resourceID, action1ID)

	action2 := CreateActionRequest{
		Name:        "Resource Action 2",
		Handle:      "resource-action-2",
		Description: "Second resource action",
	}
	action2ID, err := createActionAtResource(suite.resourceServerID, suite.resourceID, action2)
	suite.Require().NoError(err)
	defer deleteActionAtResource(suite.resourceServerID, suite.resourceID, action2ID)

	// List actions at resource level
	list, err := listActionsAtResource(suite.resourceServerID, suite.resourceID, 0, 100)
	suite.Require().NoError(err)
	suite.GreaterOrEqual(list.TotalResults, 2)
	suite.Equal(1, list.StartIndex)

	// Verify our actions are in the list
	foundAction1 := false
	foundAction2 := false
	for _, action := range list.Actions {

		if action.ID == action1ID {
			foundAction1 = true
			suite.Equal(action1.Name, action.Name)
		}
		if action.ID == action2ID {
			foundAction2 = true
			suite.Equal(action2.Name, action.Name)
		}
	}
	suite.True(foundAction1, "Should find first action")
	suite.True(foundAction2, "Should find second action")
}

func (suite *ActionAPITestSuite) TestUpdateActionAtResource() {
	req := CreateActionRequest{
		Name:        "Update Resource Action",
		Handle:      "update-resource-action",
		Description: "Original description",
	}

	actionID, err := createActionAtResource(suite.resourceServerID, suite.resourceID, req)
	suite.Require().NoError(err)
	defer deleteActionAtResource(suite.resourceServerID, suite.resourceID, actionID)

	// Update action
	updateReq := UpdateActionRequest{
		Name:        "Updated Resource Action Name",
		Description: "Updated description for resource action",
	}
	err = updateActionAtResource(suite.resourceServerID, suite.resourceID, actionID, updateReq)
	suite.Require().NoError(err)

	// Verify update
	action, err := getActionAtResource(suite.resourceServerID, suite.resourceID, actionID)
	suite.Require().NoError(err)
	suite.Equal(updateReq.Name, action.Name, "Name should be updated")
	suite.Equal(req.Handle, action.Handle, "Handle should be immutable")
	suite.Equal(updateReq.Description, action.Description, "Description should be updated")
}

func (suite *ActionAPITestSuite) TestDeleteActionAtResource() {
	req := CreateActionRequest{
		Name:        "Delete Resource Action",
		Handle:      "delete-resource-action",
		Description: "Action to delete",
	}

	actionID, err := createActionAtResource(suite.resourceServerID, suite.resourceID, req)
	suite.Require().NoError(err)

	err = deleteActionAtResource(suite.resourceServerID, suite.resourceID, actionID)
	suite.Require().NoError(err)

	// Verify action is deleted
	_, err = getActionAtResource(suite.resourceServerID, suite.resourceID, actionID)
	suite.Error(err)
	suite.Contains(err.Error(), "404")
}

func (suite *ActionAPITestSuite) TestDeleteActionAtResourceIdempotent() {
	// Test idempotency with non-existent resource server
	err := deleteActionAtResource("00000000-0000-0000-0000-000000000000", suite.resourceID, "00000000-0000-0000-0000-000000000000")
	suite.NoError(err, "Delete should be idempotent for non-existent resource server")

	// Test idempotency with non-existent resource
	err = deleteActionAtResource(suite.resourceServerID, "00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000")
	suite.NoError(err, "Delete should be idempotent for non-existent resource")

	// Test idempotency with non-existent action
	err = deleteActionAtResource(suite.resourceServerID, suite.resourceID, "00000000-0000-0000-0000-000000000000")
	suite.NoError(err, "Delete should be idempotent for non-existent action")
}

func (suite *ActionAPITestSuite) TestDeleteActionAtResourceUsingWrongEndpoint() {
	// Create action at resource level
	req := CreateActionRequest{
		Name:        "Resource Wrong Endpoint Action",
		Handle:      "wrong-endpoint-action",
		Description: "Action to test wrong endpoint",
	}

	actionID, err := createActionAtResource(suite.resourceServerID, suite.resourceID, req)
	suite.Require().NoError(err)
	defer deleteActionAtResource(suite.resourceServerID, suite.resourceID, actionID)

	// Try to delete using resource server level endpoint - should be idempotent
	err = deleteAction(suite.resourceServerID, actionID)
	suite.NoError(err, "Delete should be idempotent")

	// Verify action still exists at resource level
	action, err := getActionAtResource(suite.resourceServerID, suite.resourceID, actionID)
	suite.Require().NoError(err, "Action should still exist")
	suite.Equal(actionID, action.ID)
}

func (suite *ActionAPITestSuite) TestDeleteActionAtResourceServerUsingWrongEndpoint() {
	// Create action at resource server level
	req := CreateActionRequest{
		Name:        "Server Wrong Endpoint Action",
		Handle:      "server-wrong-endpoint-action",
		Description: "Action to test wrong endpoint",
	}

	actionID, err := createActionAtResourceServer(suite.resourceServerID, req)
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, actionID)

	// Try to delete using resource level endpoint - should return idempotent
	err = deleteActionAtResource(suite.resourceServerID, suite.resourceID, actionID)
	suite.NoError(err, "Delete should be idempotent")

	// Verify action still exists at resource server level
	action, err := getActionAtResourceServer(suite.resourceServerID, actionID)
	suite.Require().NoError(err, "Action should still exist")
	suite.Equal(actionID, action.ID)
}

func (suite *ActionAPITestSuite) TestActionPermissionDerivationWithCustomDelimiter() {
	// Create resource server with custom delimiter
	delimiter := "-"
	rsReq := CreateResourceServerRequest{
		Name:               "Action Permission Test Server",
		Handle:            "actionpermtest",
		OUID: suite.ouID,
		Delimiter:          &delimiter,
	}
	customRsID, err := createResourceServer(rsReq)
	suite.Require().NoError(err)
	defer deleteResourceServer(customRsID)

	// Create level 1 resource: hotels
	level1Req := CreateResourceRequest{
		Name:   "Hotels",
		Handle: "hotels",
		Parent: nil,
	}
	level1ID, err := createResource(customRsID, level1Req)
	suite.Require().NoError(err)
	defer deleteResource(customRsID, level1ID)

	// Create level 2 resource: hotels-rooms
	level2Req := CreateResourceRequest{
		Name:   "Rooms",
		Handle: "rooms",
		Parent: &level1ID,
	}
	level2ID, err := createResource(customRsID, level2Req)
	suite.Require().NoError(err)
	defer deleteResource(customRsID, level2ID)

	// Create level 3 resource: hotels-rooms-suites
	level3Req := CreateResourceRequest{
		Name:   "Suites",
		Handle: "suites",
		Parent: &level2ID,
	}
	level3ID, err := createResource(customRsID, level3Req)
	suite.Require().NoError(err)
	defer deleteResource(customRsID, level3ID)

	// Create action at level 3: hotels-rooms-suites-book
	actionReq := CreateActionRequest{
		Name:   "Book",
		Handle: "book",
	}
	actionID, err := createActionAtResource(customRsID, level3ID, actionReq)
	suite.Require().NoError(err)
	defer deleteActionAtResource(customRsID, level3ID, actionID)

	action, err := getActionAtResource(customRsID, level3ID, actionID)
	suite.Require().NoError(err)
	suite.Equal("actionpermtest-hotels-rooms-suites-book", action.Permission, "Deeply nested action permission should use custom delimiter")
}

// Helper functions

func createActionAtResourceServer(resourceServerID string, req CreateActionRequest) (string, error) {
	client := testutils.GetHTTPClient()

	body, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/resource-servers/%s/actions", testServerURL, resourceServerID)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var action ActionResponse
	if err := json.NewDecoder(resp.Body).Decode(&action); err != nil {
		return "", err
	}

	return action.ID, nil
}

func createActionAtResource(resourceServerID, resourceID string, req CreateActionRequest) (string, error) {
	client := testutils.GetHTTPClient()

	body, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/resource-servers/%s/resources/%s/actions", testServerURL, resourceServerID, resourceID)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var action ActionResponse
	if err := json.NewDecoder(resp.Body).Decode(&action); err != nil {
		return "", err
	}

	return action.ID, nil
}

func getActionAtResourceServer(resourceServerID, actionID string) (*ActionResponse, error) {
	client := testutils.GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers/%s/actions/%s", testServerURL, resourceServerID, actionID)
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var action ActionResponse
	if err := json.NewDecoder(resp.Body).Decode(&action); err != nil {
		return nil, err
	}

	return &action, nil
}

func getActionAtResource(resourceServerID, resourceID, actionID string) (*ActionResponse, error) {
	client := testutils.GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers/%s/resources/%s/actions/%s", testServerURL, resourceServerID, resourceID, actionID)
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var action ActionResponse
	if err := json.NewDecoder(resp.Body).Decode(&action); err != nil {
		return nil, err
	}

	return &action, nil
}

func listActionsAtResourceServer(resourceServerID string, offset, limit int) (*ActionListResponse, error) {
	client := testutils.GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers/%s/actions?offset=%d&limit=%d", testServerURL, resourceServerID, offset, limit)
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var list ActionListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	return &list, nil
}

func listActionsAtResource(resourceServerID, resourceID string, offset, limit int) (*ActionListResponse, error) {
	client := testutils.GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers/%s/resources/%s/actions?offset=%d&limit=%d", testServerURL, resourceServerID, resourceID, offset, limit)
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var list ActionListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	return &list, nil
}

func updateAction(resourceServerID, actionID string, req UpdateActionRequest) error {
	client := testutils.GetHTTPClient()

	body, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/resource-servers/%s/actions/%s", testServerURL, resourceServerID, actionID)
	httpReq, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func updateActionAtResource(resourceServerID, resourceID, actionID string, req UpdateActionRequest) error {
	client := testutils.GetHTTPClient()

	body, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/resource-servers/%s/resources/%s/actions/%s", testServerURL, resourceServerID, resourceID, actionID)
	httpReq, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func deleteAction(resourceServerID, actionID string) error {
	client := testutils.GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers/%s/actions/%s", testServerURL, resourceServerID, actionID)
	httpReq, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func deleteActionAtResource(resourceServerID, resourceID, actionID string) error {
	client := testutils.GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers/%s/resources/%s/actions/%s", testServerURL, resourceServerID, resourceID, actionID)
	httpReq, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
