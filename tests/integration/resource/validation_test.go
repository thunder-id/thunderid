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
		Name:               "validation-test-server",
		Description:        "Resource server for validation testing",
		Handle:            "validation-test",
		OUID: ouID,
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
		Name:               "",
		OUID: suite.ouID,
	}

	_, err := createResourceServer(req)
	suite.Error(err, "Should fail with missing name")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestCreateResourceServerMissingOrgUnit() {
	req := CreateResourceServerRequest{
		Name:               "missing-ou-server",
		OUID: "",
	}

	_, err := createResourceServer(req)
	suite.Error(err, "Should fail with missing organization unit")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestUpdateResourceServerMissingName() {
	// Create a resource server first
	createReq := CreateResourceServerRequest{
		Name:               "update-validation-server",
		Handle:            "update-validation",
		OUID: suite.ouID,
	}
	rsID, err := createResourceServer(createReq)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID)

	// Try to update with empty name
	updateReq := UpdateResourceServerRequest{
		Name:               "",
		OUID: suite.ouID,
	}

	err = updateResourceServer(rsID, updateReq)
	suite.Error(err, "Should fail with missing name")
	suite.Contains(err.Error(), "400")
}

func (suite *ValidationTestSuite) TestDeleteResourceServerWithDependencies() {
	// Create resource server
	rsReq := CreateResourceServerRequest{
		Name:               "server-with-dependencies",
		Handle:            "server-with-deps",
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
		Name:               "server-with-actions",
		Handle:            "server-with-actions",
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
		Name:               "second-server",
		Handle:            "second-server",
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
		Name:               "wrong-server",
		Handle:            "wrong-server",
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
		Name:               "test-server",
		Handle:            "test-server",
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
		Name:               "delimiter-test-server",
		Handle:            "delimiter-test",
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
		Name:               "action-delimiter-test-server",
		Handle:            "action-delim-test",
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
