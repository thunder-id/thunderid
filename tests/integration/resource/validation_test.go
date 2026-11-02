// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type ValidationTestSuite struct {
	suite.Suite
	ouID             string
	resourceServerID string
}

func TestValidationTestSuite(t *testing.T) {
	suite.Run(t, new(ValidationTestSuite))
}

func (suite *ValidationTestSuite) SetupSuite() {
	// Create test organization unit
	ou := testutils.OrganizationUnit{
		Handle:      "test_validation_ou",
		Name:        "Test OU for Validation",
		Description: "Organization unit for validation testing",
		Parent:      nil,
	}
	ouID, err := testutils.CreateOrganizationUnit(ou)
	suite.Require().NoError(err, "Failed to create test organization unit")
	suite.ouID = ouID

	// Create test resource server
	rsReq := CreateResourceServerRequest{
		Name:        "validation-test-server",
		Description: "Resource server for validation testing",
		OUID:        ouID,
	}
	rsID, err := createResourceServer(rsReq)
	suite.Require().NoError(err, "Failed to create test resource server")
	suite.resourceServerID = rsID
}

func (suite *ValidationTestSuite) TearDownSuite() {
	if suite.resourceServerID != "" {
		deleteResourceServer(suite.resourceServerID)
	}
	if suite.ouID != "" {
		testutils.DeleteOrganizationUnit(suite.ouID)
	}
}

// Resource Server Validation Tests

func (suite *ValidationTestSuite) TestCreateResourceServerMissingName() {
	req := CreateResourceServerRequest{
		Name: "",
		OUID: suite.ouID,
	}

	_, err := createResourceServer(req)
	suite.Error(err, "Should fail with missing name")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestCreateResourceServerMissingOrgUnit() {
	req := CreateResourceServerRequest{
		Name: "missing-ou-server",
		OUID: "",
	}

	_, err := createResourceServer(req)
	suite.Error(err, "Should fail with missing organization unit")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestCreateResourceServerMissingIdentifier() {
	// Identifier is mandatory on create. Post a raw request without the identifier field
	// (the createResourceServer helper auto-fills one, so it cannot exercise this path).
	body, _ := json.Marshal(map[string]interface{}{
		"name": "missing-identifier-server",
		"ouId": suite.ouID,
	})

	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("POST", testServerURL+"/resource-servers", bytes.NewReader(body))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	suite.Equal(http.StatusBadRequest, resp.StatusCode,
		"creating a resource server without an identifier must be rejected. Response: %s", string(respBody))
	suite.Contains(string(respBody), "identifier", "rejection should reference the missing identifier field")
}

func (suite *ValidationTestSuite) TestUpdateResourceServerMissingName() {
	// Create a resource server first
	createReq := CreateResourceServerRequest{
		Name: "update-validation-server",
		OUID: suite.ouID,
	}
	rsID, err := createResourceServer(createReq)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID)

	// Try to update with empty name
	updateReq := UpdateResourceServerRequest{
		Name: "",
		OUID: suite.ouID,
	}

	err = updateResourceServer(rsID, updateReq)
	suite.Error(err, "Should fail with missing name")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestDeleteResourceServerWithDependencies() {
	// Create resource server
	rsReq := CreateResourceServerRequest{
		Name: "server-with-dependencies",
		OUID: suite.ouID,
	}
	rsID, err := createResourceServer(rsReq)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID)

	// Create a resource under it
	resReq := CreateResourceRequest{
		Name:   "Dependent Resource",
		Handle: "dependent-resource",
		Parent: nil,
	}
	resID, err := createResource(rsID, resReq)
	suite.Require().NoError(err)
	defer deleteResource(rsID, resID)

	// Try to delete resource server - should fail
	err = deleteResourceServer(rsID)
	suite.Error(err, "Should fail when resource server has resources")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestDeleteResourceServerWithActions() {
	// Create resource server
	rsReq := CreateResourceServerRequest{
		Name: "server-with-actions",
		OUID: suite.ouID,
	}
	rsID, err := createResourceServer(rsReq)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID)

	// Create an action under it
	actionReq := CreateActionRequest{
		Name:        "Dependent Action",
		Handle:      "dependent-action",
		Description: "Action that makes server non-deletable",
	}
	actionID, err := createActionAtResourceServer(rsID, actionReq)
	suite.Require().NoError(err)
	defer deleteAction(rsID, actionID)

	// Try to delete resource server - should fail
	err = deleteResourceServer(rsID)
	suite.Error(err, "Should fail when resource server has actions")
	suite.Contains(err.Error(), "400")
}

// Resource Validation Tests

func (suite *ValidationTestSuite) TestCreateResourceMissingName() {
	req := CreateResourceRequest{
		Name:   "",
		Handle: "missing-name-resource",
		Parent: nil,
	}

	_, err := createResource(suite.resourceServerID, req)
	suite.Error(err, "Should fail with missing name")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestCreateResourceMissingHandle() {
	req := CreateResourceRequest{
		Name:   "missing-handle-resource",
		Handle: "",
		Parent: nil,
	}

	_, err := createResource(suite.resourceServerID, req)
	suite.Error(err, "Should fail with missing handle")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestCreateResourceNonExistentResourceServer() {
	req := CreateResourceRequest{
		Name:   "Resource Invalid Server",
		Handle: "resource-invalid-server",
		Parent: nil,
	}

	_, err := createResource("00000000-0000-0000-0000-000000000000", req)
	suite.Error(err, "Should fail with non-existent resource server")
	suite.Contains(err.Error(), "404")
}

func (suite *ValidationTestSuite) TestDeleteResourceWithActions() {
	// Create resource
	resReq := CreateResourceRequest{
		Name:   "Resource With Actions",
		Handle: "resource-with-actions",
		Parent: nil,
	}
	resID, err := createResource(suite.resourceServerID, resReq)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resID)

	// Create action under it
	actionReq := CreateActionRequest{
		Name:        "Blocking Action",
		Handle:      "blocking-action",
		Description: "Action that prevents resource deletion",
	}
	actionID, err := createActionAtResource(suite.resourceServerID, resID, actionReq)
	suite.Require().NoError(err)
	defer deleteActionAtResource(suite.resourceServerID, resID, actionID)

	// Try to delete resource - should fail
	err = deleteResource(suite.resourceServerID, resID)
	suite.Error(err, "Should fail when resource has actions")
	suite.Contains(err.Error(), "400")
}

// Action Validation Tests

func (suite *ValidationTestSuite) TestCreateActionMissingName() {
	req := CreateActionRequest{
		Name:        "",
		Handle:      "missing-name-action",
		Description: "Action without name",
	}

	_, err := createActionAtResourceServer(suite.resourceServerID, req)
	suite.Error(err, "Should fail with missing name")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestCreateActionMissingHandle() {
	req := CreateActionRequest{
		Name:        "missing-handle-action",
		Handle:      "",
		Description: "Action without handle",
	}

	_, err := createActionAtResourceServer(suite.resourceServerID, req)
	suite.Error(err, "Should fail with missing handle")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestCreateActionAtNonExistentResourceServer() {
	req := CreateActionRequest{
		Name:        "Orphan Action",
		Handle:      "orphan-action",
		Description: "Action for non-existent server",
	}

	_, err := createActionAtResourceServer("00000000-0000-0000-0000-000000000000", req)
	suite.Error(err, "Should fail with non-existent resource server")
	suite.Contains(err.Error(), "404")
}

func (suite *ValidationTestSuite) TestCreateActionAtNonExistentResource() {
	req := CreateActionRequest{
		Name:        "Orphan Resource Action",
		Handle:      "orphan-resource-action",
		Description: "Action for non-existent resource",
	}

	_, err := createActionAtResource(suite.resourceServerID, "00000000-0000-0000-0000-000000000000", req)
	suite.Error(err, "Should fail with non-existent resource")
	suite.Contains(err.Error(), "404")
}

// Pagination Validation Tests

func (suite *ValidationTestSuite) TestListResourceServersInvalidLimit() {
	_, err := listResourceServers(0, 0)
	suite.Error(err, "Should fail with limit 0")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestListResourceServersInvalidOffset() {
	_, err := listResourceServers(-1, 10)
	suite.Error(err, "Should fail with negative offset")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestListResourceServersExceedMaxLimit() {
	_, err := listResourceServers(0, 1000)
	suite.Error(err, "Should fail with limit exceeding maximum")
	suite.Contains(err.Error(), "400")
}

// Cross-resource server validation tests

func (suite *ValidationTestSuite) TestCreateResourceInDifferentResourceServer() {
	// Create second resource server
	rsReq := CreateResourceServerRequest{
		Name: "second-server",
		OUID: suite.ouID,
	}
	rs2ID, err := createResourceServer(rsReq)
	suite.Require().NoError(err)
	defer deleteResourceServer(rs2ID)

	// Create resource in first server
	res1Req := CreateResourceRequest{
		Name:   "Resource In Server 1",
		Handle: "resource-in-server1",
		Parent: nil,
	}
	res1ID, err := createResource(suite.resourceServerID, res1Req)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, res1ID)

	// Try to create resource in second server with parent from first server
	res2Req := CreateResourceRequest{
		Name:   "Resource In Server 2",
		Handle: "resource-in-server2",
		Parent: &res1ID,
	}
	_, err = createResource(rs2ID, res2Req)
	suite.Error(err, "Should fail when parent is from different resource server")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestGetResourceFromWrongResourceServer() {
	// Create second resource server
	rsReq := CreateResourceServerRequest{
		Name: "wrong-server",
		OUID: suite.ouID,
	}
	rs2ID, err := createResourceServer(rsReq)
	suite.Require().NoError(err)
	defer deleteResourceServer(rs2ID)

	// Create resource in first server
	resReq := CreateResourceRequest{
		Name:   "Resource In Correct Server",
		Handle: "resource-in-correct-server",
		Parent: nil,
	}
	resID, err := createResource(suite.resourceServerID, resReq)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resID)

	// Try to get resource from wrong server
	_, err = getResource(rs2ID, resID)
	suite.Error(err, "Should fail when accessing resource from wrong server")
	suite.Contains(err.Error(), "404")
}

// Helper function to send raw HTTP requests for testing malformed requests

func (suite *ValidationTestSuite) TestMalformedJSONRequest() {
	client := testutils.GetHTTPClient()

	malformedJSON := []byte(`{"name": "test", invalid json}`)
	url := fmt.Sprintf("%s/resource-servers", testServerURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(malformedJSON))
	suite.Require().NoError(err)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Should return 400 for malformed JSON")
}

func (suite *ValidationTestSuite) TestInvalidContentType() {
	client := testutils.GetHTTPClient()

	req := CreateResourceServerRequest{
		Name: "test-server",
		OUID: suite.ouID,
	}
	body, _ := json.Marshal(req)

	url := fmt.Sprintf("%s/resource-servers", testServerURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	suite.Require().NoError(err)
	httpReq.Header.Set("Content-Type", "text/plain")

	resp, err := client.Do(httpReq)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	// Server might accept it or reject it depending on implementation
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnsupportedMediaType {
		bodyBytes, _ := io.ReadAll(resp.Body)
		suite.T().Logf("Correctly rejected invalid content type: %d, %s", resp.StatusCode, string(bodyBytes))
	}
}

// Handle Validation Tests (must not contain delimiter)

func (suite *ValidationTestSuite) TestCreateResourceHandleContainsDelimiter() {
	// First create a resource server to get its delimiter
	rsReq := CreateResourceServerRequest{
		Name: "delimiter-test-server",
		OUID: suite.ouID,
	}
	rsID, err := createResourceServer(rsReq)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID)

	rs, err := getResourceServer(rsID)
	suite.Require().NoError(err)
	delimiter := rs.Delimiter

	// Try to create resource with handle containing the delimiter
	req := CreateResourceRequest{
		Name:   "Invalid Handle Resource",
		Handle: "bad" + delimiter + "handle",
		Parent: nil,
	}

	_, err = createResource(rsID, req)
	suite.Error(err, "Should fail when handle contains delimiter")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestCreateActionHandleContainsDelimiter() {
	// First create a resource server to get its delimiter
	rsReq := CreateResourceServerRequest{
		Name: "action-delimiter-test-server",
		OUID: suite.ouID,
	}
	rsID, err := createResourceServer(rsReq)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID)

	rs, err := getResourceServer(rsID)
	suite.Require().NoError(err)
	delimiter := rs.Delimiter

	// Try to create action with handle containing the delimiter
	req := CreateActionRequest{
		Name:   "Invalid Handle Action",
		Handle: "bad" + delimiter + "handle",
	}

	_, err = createActionAtResourceServer(rsID, req)
	suite.Error(err, "Should fail when handle contains delimiter")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestCreateResourceHandleInvalidCharacters() {
	req := CreateResourceRequest{
		Name:   "Invalid Characters Resource",
		Handle: "bad handle",
		Parent: nil,
	}

	_, err := createResource(suite.resourceServerID, req)
	suite.Error(err, "Should fail when handle contains space")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestCreateActionHandleInvalidCharacters() {
	req := CreateActionRequest{
		Name:   "Invalid Characters Action",
		Handle: "bad\"handle",
	}

	_, err := createActionAtResourceServer(suite.resourceServerID, req)
	suite.Error(err, "Should fail when handle contains invalid characters")
	suite.Contains(err.Error(), "400")
}

// Malformed request body tests

func (suite *ValidationTestSuite) TestMalformedJSONOnWriteEndpoints() {
	resID, err := createResource(suite.resourceServerID, CreateResourceRequest{
		Name:   "Malformed Body Resource",
		Handle: "malformed-body-resource",
		Parent: nil,
	})
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resID)

	actionID, err := createActionAtResource(suite.resourceServerID, resID, CreateActionRequest{
		Name:   "Malformed Body Action",
		Handle: "malformed-body-action",
	})
	suite.Require().NoError(err)
	defer deleteActionAtResource(suite.resourceServerID, resID, actionID)

	serverActionID, err := createActionAtResourceServer(suite.resourceServerID, CreateActionRequest{
		Name:   "Malformed Body Server Action",
		Handle: "malformed-body-server-action",
	})
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, serverActionID)

	malformed := []byte(`{"name": "broken", }`)

	testCases := []struct {
		description string
		method      string
		url         string
	}{
		{"update resource server", http.MethodPut, resourceServerURL("/%s", suite.resourceServerID)},
		{"create resource", http.MethodPost, resourceServerURL("/%s/resources", suite.resourceServerID)},
		{"update resource", http.MethodPut, resourceServerURL("/%s/resources/%s", suite.resourceServerID, resID)},
		{"create action at resource server", http.MethodPost,
			resourceServerURL("/%s/actions", suite.resourceServerID)},
		{"update action at resource server", http.MethodPut,
			resourceServerURL("/%s/actions/%s", suite.resourceServerID, serverActionID)},
		{"create action at resource", http.MethodPost,
			resourceServerURL("/%s/resources/%s/actions", suite.resourceServerID, resID)},
		{"update action at resource", http.MethodPut,
			resourceServerURL("/%s/resources/%s/actions/%s", suite.resourceServerID, resID, actionID)},
	}

	for _, tc := range testCases {
		resp, err := doRawRequest(tc.method, tc.url, malformed)
		suite.Require().NoError(err)
		suite.Equal(http.StatusBadRequest, resp.StatusCode,
			"Should return 400 for malformed JSON on %s. Response: %s", tc.description, resp.Body)
	}
}

func (suite *ValidationTestSuite) TestStructuredValidationErrorsOnMissingRequiredFields() {
	resID, err := createResource(suite.resourceServerID, CreateResourceRequest{
		Name:   "Structured Validation Resource",
		Handle: "structured-validation-resource",
		Parent: nil,
	})
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resID)

	serverActionID, err := createActionAtResourceServer(suite.resourceServerID, CreateActionRequest{
		Name:   "Structured Validation Action",
		Handle: "structured-validation-action",
	})
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, serverActionID)

	testCases := []struct {
		description string
		method      string
		url         string
		body        map[string]interface{}
	}{
		{"update resource server without organization unit", http.MethodPut,
			resourceServerURL("/%s", suite.resourceServerID),
			map[string]interface{}{"name": "No Organization Unit"}},
		{"update action at resource server without name", http.MethodPut,
			resourceServerURL("/%s/actions/%s", suite.resourceServerID, serverActionID),
			map[string]interface{}{"description": "No name"}},
		{"create action at resource without name", http.MethodPost,
			resourceServerURL("/%s/resources/%s/actions", suite.resourceServerID, resID),
			map[string]interface{}{"handle": "no-name"}},
	}

	for _, tc := range testCases {
		resp, err := doJSONRequest(tc.method, tc.url, tc.body)
		suite.Require().NoError(err)
		suite.Equal(http.StatusBadRequest, resp.StatusCode,
			"Should return 400 for %s. Response: %s", tc.description, resp.Body)
	}
}

func (suite *ValidationTestSuite) TestUpdateResourceMissingName() {
	resID, err := createResource(suite.resourceServerID, CreateResourceRequest{
		Name:   "Update Validation Resource",
		Handle: "update-validation-resource",
		Parent: nil,
	})
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resID)

	resp, err := doJSONRequest(http.MethodPut,
		resourceServerURL("/%s/resources/%s", suite.resourceServerID, resID),
		UpdateResourceRequest{Name: ""})
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Response: %s", resp.Body)
}

func (suite *ValidationTestSuite) TestCreateResourceHandleExceedsMaxLength() {
	handle := ""
	for i := 0; i < 101; i++ {
		handle += "a"
	}

	_, err := createResource(suite.resourceServerID, CreateResourceRequest{
		Name:   "Too Long Handle Resource",
		Handle: handle,
		Parent: nil,
	})
	suite.Error(err, "Should fail when handle exceeds the maximum length")
	suite.Contains(err.Error(), "400")
}

// Pagination validation on nested collections

func (suite *ValidationTestSuite) TestListResourcesInvalidPaginationParams() {
	testCases := []struct {
		description string
		query       string
	}{
		{"non-numeric limit", "?limit=abc"},
		{"zero limit", "?limit=0"},
		{"negative offset", "?offset=-1"},
	}

	for _, tc := range testCases {
		resp, err := doRawRequest(http.MethodGet,
			resourceServerURL("/%s/resources%s", suite.resourceServerID, tc.query), nil)
		suite.Require().NoError(err)
		suite.Equal(http.StatusBadRequest, resp.StatusCode,
			"Should return 400 for %s. Response: %s", tc.description, resp.Body)
	}
}

func (suite *ValidationTestSuite) TestListActionsInvalidPaginationParams() {
	resID, err := createResource(suite.resourceServerID, CreateResourceRequest{
		Name:   "Action Pagination Resource",
		Handle: "action-pagination-resource",
		Parent: nil,
	})
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resID)

	testCases := []struct {
		description string
		url         string
	}{
		{"resource server level, non-numeric limit",
			resourceServerURL("/%s/actions?limit=abc", suite.resourceServerID)},
		{"resource server level, negative offset",
			resourceServerURL("/%s/actions?offset=-1", suite.resourceServerID)},
		{"resource server level, zero limit",
			resourceServerURL("/%s/actions?limit=0", suite.resourceServerID)},
		{"resource level, non-numeric limit",
			resourceServerURL("/%s/resources/%s/actions?limit=abc", suite.resourceServerID, resID)},
		{"resource level, negative offset",
			resourceServerURL("/%s/resources/%s/actions?offset=-1", suite.resourceServerID, resID)},
		{"resource level, zero limit",
			resourceServerURL("/%s/resources/%s/actions?limit=0", suite.resourceServerID, resID)},
	}

	for _, tc := range testCases {
		resp, err := doRawRequest(http.MethodGet, tc.url, nil)
		suite.Require().NoError(err)
		suite.Equal(http.StatusBadRequest, resp.StatusCode,
			"Should return 400 for %s. Response: %s", tc.description, resp.Body)
	}
}

// Not found propagation on collection endpoints

func (suite *ValidationTestSuite) TestListResourcesForNonExistentResourceServer() {
	_, err := listResources("00000000-0000-0000-0000-000000000000", "", 0, 10)
	suite.Error(err, "Should fail for a non-existent resource server")
	suite.Contains(err.Error(), "404")
}

func (suite *ValidationTestSuite) TestListResourcesWithNonExistentParent() {
	_, err := listResources(suite.resourceServerID, "00000000-0000-0000-0000-000000000000", 0, 10)
	suite.Error(err, "Should fail for a non-existent parent resource")
	suite.Contains(err.Error(), "404")
}

func (suite *ValidationTestSuite) TestListActionsForNonExistentResourceServer() {
	_, err := listActionsAtResourceServer("00000000-0000-0000-0000-000000000000", 0, 10)
	suite.Error(err, "Should fail for a non-existent resource server")
	suite.Contains(err.Error(), "404")
}

func (suite *ValidationTestSuite) TestListActionsForNonExistentResource() {
	_, err := listActionsAtResource(suite.resourceServerID, "00000000-0000-0000-0000-000000000000", 0, 10)
	suite.Error(err, "Should fail for a non-existent resource")
	suite.Contains(err.Error(), "404")
}

func (suite *ValidationTestSuite) TestGetActionAtNonExistentResource() {
	_, err := getActionAtResource(suite.resourceServerID, "00000000-0000-0000-0000-000000000000",
		"00000000-0000-0000-0000-000000000000")
	suite.Error(err, "Should fail for a non-existent resource")
	suite.Contains(err.Error(), "404")
}

func (suite *ValidationTestSuite) TestUpdateActionAtNonExistentResource() {
	err := updateActionAtResource(suite.resourceServerID, "00000000-0000-0000-0000-000000000000",
		"00000000-0000-0000-0000-000000000000", UpdateActionRequest{Name: "Updated"})
	suite.Error(err, "Should fail for a non-existent resource")
	suite.Contains(err.Error(), "404")
}

// Idempotent delete tests

func (suite *ValidationTestSuite) TestDeleteActionAtNonExistentResourceServer() {
	err := deleteAction("00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000")
	suite.NoError(err, "Delete should be idempotent when the resource server does not exist")
}

func (suite *ValidationTestSuite) TestDeleteActionAtNonExistentResource() {
	err := deleteActionAtResource(suite.resourceServerID, "00000000-0000-0000-0000-000000000000",
		"00000000-0000-0000-0000-000000000000")
	suite.NoError(err, "Delete should be idempotent when the resource does not exist")
}

// Identifier conflict on update

func (suite *ValidationTestSuite) TestUpdateResourceServerIdentifierConflict() {
	firstID, err := createResourceServer(CreateResourceServerRequest{
		Name:       "Identifier Conflict Server 1",
		Identifier: "https://api.example.com/identifier-conflict-1",
		OUID:       suite.ouID,
	})
	suite.Require().NoError(err)
	defer deleteResourceServer(firstID)

	secondID, err := createResourceServer(CreateResourceServerRequest{
		Name:       "Identifier Conflict Server 2",
		Identifier: "https://api.example.com/identifier-conflict-2",
		OUID:       suite.ouID,
	})
	suite.Require().NoError(err)
	defer deleteResourceServer(secondID)

	err = updateResourceServer(secondID, UpdateResourceServerRequest{
		Name:       "Identifier Conflict Server 2",
		Identifier: "https://api.example.com/identifier-conflict-1",
		OUID:       suite.ouID,
	})
	suite.Error(err, "Should fail when the identifier is already taken")
	suite.Contains(err.Error(), "409")
}

// roleResponse is the role payload, limited to the fields needed to assert permission cleanup.
type roleResponse struct {
	ID          string                          `json:"id"`
	Name        string                          `json:"name"`
	Permissions []testutils.ResourcePermissions `json:"permissions"`
}

// PermissionDependencyTestSuite covers permission validation performed for roles and the dependency
// guards and cascade cleanup that run when resource entities are deleted.
type PermissionDependencyTestSuite struct {
	suite.Suite
	ouID             string
	resourceServerID string
}

func TestPermissionDependencyTestSuite(t *testing.T) {
	suite.Run(t, new(PermissionDependencyTestSuite))
}

func (suite *PermissionDependencyTestSuite) SetupSuite() {
	ou := testutils.OrganizationUnit{
		Handle:      "test_resource_permission_ou",
		Name:        "Test OU for Resource Permissions",
		Description: "Organization unit for resource permission testing",
		Parent:      nil,
	}
	ouID, err := testutils.CreateOrganizationUnit(ou)
	suite.Require().NoError(err, "Failed to create test organization unit")
	suite.ouID = ouID

	rsID, err := createResourceServer(CreateResourceServerRequest{
		Name:        "Permission Test Server",
		Description: "Resource server for permission testing",
		OUID:        ouID,
	})
	suite.Require().NoError(err, "Failed to create test resource server")
	suite.resourceServerID = rsID
}

func (suite *PermissionDependencyTestSuite) TearDownSuite() {
	if suite.resourceServerID != "" {
		deleteResourceServer(suite.resourceServerID)
	}
	if suite.ouID != "" {
		testutils.DeleteOrganizationUnit(suite.ouID)
	}
}

func (suite *PermissionDependencyTestSuite) TestRoleWithUnknownPermissionRejected() {
	_, err := testutils.CreateRole(testutils.Role{
		Name:        "Role With Unknown Permission",
		Description: "Role referencing a permission that does not exist",
		OUID:        suite.ouID,
		Permissions: []testutils.ResourcePermissions{{
			ResourceServerID: suite.resourceServerID,
			Permissions:      []string{"does-not-exist"},
		}},
	})
	suite.Error(err, "Role creation should fail for an unknown permission")
	suite.Contains(err.Error(), "400")
}

func (suite *PermissionDependencyTestSuite) TestRoleWithUnknownResourceServerRejected() {
	_, err := testutils.CreateRole(testutils.Role{
		Name:        "Role With Unknown Resource Server",
		Description: "Role referencing a resource server that does not exist",
		OUID:        suite.ouID,
		Permissions: []testutils.ResourcePermissions{{
			ResourceServerID: "00000000-0000-0000-0000-000000000000",
			Permissions:      []string{"read"},
		}},
	})
	suite.Error(err, "Role creation should fail for an unknown resource server")
	suite.Contains(err.Error(), "400")
}

func (suite *PermissionDependencyTestSuite) TestRoleWithEmptyPermissionListAccepted() {
	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        "Role With Empty Permission List",
		Description: "Role that references a resource server without permissions",
		OUID:        suite.ouID,
		Permissions: []testutils.ResourcePermissions{{
			ResourceServerID: suite.resourceServerID,
			Permissions:      []string{},
		}},
	})
	suite.Require().NoError(err)
	defer testutils.DeleteRole(roleID)

	role, err := getRole(roleID)
	suite.Require().NoError(err)
	suite.Empty(permissionsFor(role, suite.resourceServerID))
}

func (suite *PermissionDependencyTestSuite) TestRoleWithDeclarativePermissionAccepted() {
	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        "Role With Declarative Permission",
		Description: "Role referencing a permission from a declarative resource server",
		OUID:        suite.ouID,
		Permissions: []testutils.ResourcePermissions{{
			ResourceServerID: declarativeResourceServerID,
			Permissions:      []string{declarativeReadPermission},
		}},
	})
	suite.Require().NoError(err, "Permissions of a declarative resource server should validate")
	defer testutils.DeleteRole(roleID)

	role, err := getRole(roleID)
	suite.Require().NoError(err)
	suite.Contains(permissionsFor(role, declarativeResourceServerID), declarativeReadPermission)
}

// TestDeleteActionRemovesRolePermission verifies that deleting an action cascades into the roles
// holding the permission it contributed.
func (suite *PermissionDependencyTestSuite) TestDeleteActionRemovesRolePermission() {
	resourceID, err := createResource(suite.resourceServerID, CreateResourceRequest{
		Name:   "Cascade Resource",
		Handle: "cascade-resource",
		Parent: nil,
	})
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resourceID)

	actionID, err := createActionAtResource(suite.resourceServerID, resourceID, CreateActionRequest{
		Name:   "Cascade Action",
		Handle: "cascade-action",
	})
	suite.Require().NoError(err)

	permission := "cascade-resource:cascade-action"
	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        "Role With Cascading Permission",
		Description: "Role holding a permission that is removed with its action",
		OUID:        suite.ouID,
		Permissions: []testutils.ResourcePermissions{{
			ResourceServerID: suite.resourceServerID,
			Permissions:      []string{permission},
		}},
	})
	suite.Require().NoError(err)
	defer testutils.DeleteRole(roleID)

	role, err := getRole(roleID)
	suite.Require().NoError(err)
	suite.Contains(permissionsFor(role, suite.resourceServerID), permission)

	err = deleteActionAtResource(suite.resourceServerID, resourceID, actionID)
	suite.Require().NoError(err)

	role, err = getRole(roleID)
	suite.Require().NoError(err)
	suite.NotContains(permissionsFor(role, suite.resourceServerID), permission,
		"Deleting an action should remove the permission it contributed from roles")
}

// TestDeleteResourceWithActionIsBlocked verifies the restrict dependency guard on resources.
func (suite *PermissionDependencyTestSuite) TestDeleteResourceWithActionIsBlocked() {
	resourceID, err := createResource(suite.resourceServerID, CreateResourceRequest{
		Name:   "Guarded Resource",
		Handle: "guarded-resource",
		Parent: nil,
	})
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resourceID)

	actionID, err := createActionAtResource(suite.resourceServerID, resourceID, CreateActionRequest{
		Name:   "Guarding Action",
		Handle: "guarding-action",
	})
	suite.Require().NoError(err)
	defer deleteActionAtResource(suite.resourceServerID, resourceID, actionID)

	resp, err := doRawRequest(http.MethodDelete,
		resourceServerURL("/%s/resources/%s", suite.resourceServerID, resourceID), nil)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Response: %s", resp.Body)
	suite.Equal("RES-1006", resp.Error.Code, "Deletion should be refused while an action depends on the resource")
}

// TestDeleteResourceServerWithResourceIsBlocked verifies the restrict dependency guard on resource
// servers.
func (suite *PermissionDependencyTestSuite) TestDeleteResourceServerWithResourceIsBlocked() {
	rsID, err := createResourceServer(CreateResourceServerRequest{
		Name: "Guarded Resource Server",
		OUID: suite.ouID,
	})
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID)

	resourceID, err := createResource(rsID, CreateResourceRequest{
		Name:   "Guarding Resource",
		Handle: "guarding-resource",
		Parent: nil,
	})
	suite.Require().NoError(err)
	defer deleteResource(rsID, resourceID)

	resp, err := doRawRequest(http.MethodDelete, resourceServerURL("/%s", rsID), nil)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Response: %s", resp.Body)
	suite.Equal("RES-1006", resp.Error.Code,
		"Deletion should be refused while a resource depends on the resource server")
}

// Helper functions

// getRole fetches a role by ID.
func getRole(roleID string) (*roleResponse, error) {
	resp, err := doRawRequest(http.MethodGet, testServerURL+"/roles/"+roleID, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, resp.Body)
	}

	var role roleResponse
	if err := json.Unmarshal([]byte(resp.Body), &role); err != nil {
		return nil, err
	}
	return &role, nil
}

// permissionsFor returns the permissions a role holds on the given resource server.
func permissionsFor(role *roleResponse, resourceServerID string) []string {
	for _, resPerm := range role.Permissions {
		if resPerm.ResourceServerID == resourceServerID {
			return resPerm.Permissions
		}
	}
	return nil
}
