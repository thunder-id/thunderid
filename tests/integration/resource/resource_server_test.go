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
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL                  = "https://localhost:8095"
	systemResourceServerIdentifier = "https://localhost:8090/mcp"
)

var (
	testOU = testutils.OrganizationUnit{
		Handle:      "test_resource_ou",
		Name:        "Test Organization Unit for Resources",
		Description: "Organization unit created for resource API testing",
		Parent:      nil,
	}
)

var testOUID string

type ResourceServerAPITestSuite struct {
	suite.Suite
}

func TestResourceServerAPITestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServerAPITestSuite))
}

func (suite *ResourceServerAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	suite.Require().NoError(err, "Failed to create test organization unit")
	testOUID = ouID
}

func (suite *ResourceServerAPITestSuite) TearDownSuite() {
	if testOUID != "" {
		err := testutils.DeleteOrganizationUnit(testOUID)
		if err != nil {
			suite.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

func (suite *ResourceServerAPITestSuite) TestCreateResourceServer() {
	reqBody := CreateResourceServerRequest{
		Name:        "Booking System",
		Description: "Handles all booking operations",
		OUID:        testOUID,
	}

	rsID, err := createResourceServer(reqBody)
	suite.Require().NoError(err, "Failed to create resource server")
	suite.NotEmpty(rsID)

	defer deleteResourceServer(rsID)

	rs, err := getResourceServer(rsID)
	suite.Require().NoError(err)
	suite.Equal(reqBody.Name, rs.Name)
	suite.Equal(reqBody.Description, rs.Description)
	suite.Equal(reqBody.OUID, rs.OUID)
	suite.NotEmpty(rs.Identifier)
	suite.NotEmpty(rs.Delimiter, "Delimiter should be set to default value")
	suite.Equal(":", rs.Delimiter, "Default delimiter should be ':' based on default configuration")
}

func (suite *ResourceServerAPITestSuite) TestCreateResourceServerWithoutOptionalFields() {
	reqBody := CreateResourceServerRequest{
		Name: "Minimal Resource Server",
		OUID: testOUID,
	}

	rsID, err := createResourceServer(reqBody)
	suite.Require().NoError(err)
	suite.NotEmpty(rsID)

	defer deleteResourceServer(rsID)

	rs, err := getResourceServer(rsID)
	suite.Require().NoError(err)
	suite.Equal(reqBody.Name, rs.Name)
	suite.Empty(rs.Description)
	suite.NotEmpty(rs.Identifier)
}

func (suite *ResourceServerAPITestSuite) TestCreateResourceServerDuplicateName() {
	reqBody := CreateResourceServerRequest{
		Name: "Duplicate Resource Server",
		OUID: testOUID,
	}

	rsID1, err := createResourceServer(reqBody)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID1)

	// Try with same name - should fail
	reqBody2 := CreateResourceServerRequest{
		Name: "Duplicate Resource Server",
		OUID: testOUID,
	}
	_, err = createResourceServer(reqBody2)
	suite.Error(err, "Should fail with duplicate name")
	suite.Contains(err.Error(), "409")
}

func (suite *ResourceServerAPITestSuite) TestCreateResourceServerDuplicateIdentifier() {
	reqBody1 := CreateResourceServerRequest{
		Name:       "Resource Server With Identifier 1",
		Identifier: "https://api.example.com/booking/",
		OUID:       testOUID,
	}

	rsID1, err := createResourceServer(reqBody1)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID1)

	reqBody2 := CreateResourceServerRequest{
		Name:       "Resource Server With Identifier 2",
		Identifier: "https://api.example.com/booking/",
		OUID:       testOUID,
	}

	_, err = createResourceServer(reqBody2)
	suite.Error(err, "Should fail with duplicate identifier")
	suite.Contains(err.Error(), "409")
}

func (suite *ResourceServerAPITestSuite) TestCreateResourceServerInvalidOU() {
	reqBody := CreateResourceServerRequest{
		Name:       "Invalid OU Resource Server",
		Identifier: "https://api.example.com/invalid-ou-rs",
		OUID:       "00000000-0000-0000-0000-000000000000",
	}

	_, err := createResourceServer(reqBody)
	suite.Error(err, "Should fail with invalid OU")
	suite.Contains(err.Error(), "400")
}

func (suite *ResourceServerAPITestSuite) TestGetResourceServer() {
	reqBody := CreateResourceServerRequest{
		Name:        "Get Test Resource Server",
		Description: "Resource server for get test",
		OUID:        testOUID,
	}

	rsID, err := createResourceServer(reqBody)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID)

	rs, err := getResourceServer(rsID)
	suite.Require().NoError(err)
	suite.Equal(rsID, rs.ID)
	suite.Equal(reqBody.Name, rs.Name)
	suite.Equal(reqBody.Description, rs.Description)
}

func (suite *ResourceServerAPITestSuite) TestGetResourceServerNotFound() {
	_, err := getResourceServer("00000000-0000-0000-0000-000000000000")
	suite.Error(err)
	suite.Contains(err.Error(), "404")
}

func (suite *ResourceServerAPITestSuite) TestListResourceServers() {
	delimiter := "-"
	rs1 := CreateResourceServerRequest{
		Name:        "List Resource Server 1",
		Description: "First resource server",
		OUID:        testOUID,
		Delimiter:   &delimiter,
	}
	rs2 := CreateResourceServerRequest{
		Name:        "List Resource Server 2",
		Description: "Second resource server",
		OUID:        testOUID,
	}

	rsID1, err := createResourceServer(rs1)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID1)

	rsID2, err := createResourceServer(rs2)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID2)

	list, err := listResourceServers(0, 100)
	suite.Require().NoError(err)
	suite.GreaterOrEqual(list.TotalResults, 2)
	suite.Equal(1, list.StartIndex)

	foundRS1 := false
	foundRS2 := false
	for _, rs := range list.ResourceServers {
		if rs.ID == rsID1 {
			foundRS1 = true
			suite.Equal(rs1.Name, rs.Name)
			suite.Equal("-", rs.Delimiter)
		}
		if rs.ID == rsID2 {
			foundRS2 = true
			suite.Equal(rs2.Name, rs.Name)
		}
	}
	suite.True(foundRS1, "Should find first resource server")
	suite.True(foundRS2, "Should find second resource server")
}

func (suite *ResourceServerAPITestSuite) TestListResourceServersWithPagination() {
	list, err := listResourceServers(0, 1)
	suite.Require().NoError(err)
	suite.LessOrEqual(list.Count, 1)
	suite.Equal(1, list.StartIndex)

	if list.TotalResults > 1 {
		suite.NotEmpty(list.Links)
	}
}

func (suite *ResourceServerAPITestSuite) TestUpdateResourceServer() {
	delimiter := "/"
	reqBody := CreateResourceServerRequest{
		Name:        "Update Test Resource Server",
		Description: "Original description",
		Identifier:  "https://api.example.com/original/",
		OUID:        testOUID,
		Delimiter:   &delimiter,
	}

	rsID, err := createResourceServer(reqBody)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID)

	updateReq := UpdateResourceServerRequest{
		Name:        "Updated Resource Server",
		Description: "Updated description",
		Identifier:  "https://api.example.com/updated/",
		OUID:        testOUID,
	}

	err = updateResourceServer(rsID, updateReq)
	suite.Require().NoError(err)

	rs, err := getResourceServer(rsID)
	suite.Require().NoError(err)
	suite.Equal(updateReq.Name, rs.Name)
	suite.Equal(updateReq.Description, rs.Description)
	suite.Equal(updateReq.Identifier, rs.Identifier, "Identifier should be updated")
	suite.Equal("/", rs.Delimiter, "Delimiter should remain unchanged after update")
}

func (suite *ResourceServerAPITestSuite) TestUpdateResourceServerPreservesIdentifierWhenOmitted() {
	reqBody := CreateResourceServerRequest{
		Name:       "Preserve Identifier RS",
		Identifier: "https://api.example.com/preserve-identifier",
		OUID:       testOUID,
	}

	rsID, err := createResourceServer(reqBody)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID)

	updateReq := UpdateResourceServerRequest{
		Name: "Preserve Identifier RS Updated",
		OUID: testOUID,
	}

	err = updateResourceServer(rsID, updateReq)
	suite.Require().NoError(err)

	rs, err := getResourceServer(rsID)
	suite.Require().NoError(err)
	suite.Equal(reqBody.Identifier, rs.Identifier, "Identifier should be preserved when not provided in update")
}

func (suite *ResourceServerAPITestSuite) TestUpdateResourceServerNotFound() {
	updateReq := UpdateResourceServerRequest{
		Name: "non-existent",
		OUID: testOUID,
	}

	err := updateResourceServer("00000000-0000-0000-0000-000000000000", updateReq)
	suite.Error(err)
	suite.Contains(err.Error(), "404")
}

func (suite *ResourceServerAPITestSuite) TestUpdateResourceServerNameConflict() {
	rs1 := CreateResourceServerRequest{
		Name: "Conflict Resource Server 1",
		OUID: testOUID,
	}
	rs2 := CreateResourceServerRequest{
		Name: "Conflict Resource Server 2",
		OUID: testOUID,
	}

	rsID1, err := createResourceServer(rs1)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID1)

	rsID2, err := createResourceServer(rs2)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID2)

	// Try to update second server to have the same name as first
	updateReq := UpdateResourceServerRequest{
		Name: "Conflict Resource Server 1",
		OUID: testOUID,
	}

	err = updateResourceServer(rsID2, updateReq)
	suite.Error(err, "Should fail with name conflict")
	suite.Contains(err.Error(), "409")
}

func (suite *ResourceServerAPITestSuite) TestDeleteResourceServer() {
	reqBody := CreateResourceServerRequest{
		Name: "Delete Test Resource Server",
		OUID: testOUID,
	}

	rsID, err := createResourceServer(reqBody)
	suite.Require().NoError(err)

	err = deleteResourceServer(rsID)
	suite.Require().NoError(err)

	_, err = getResourceServer(rsID)
	suite.Error(err)
	suite.Contains(err.Error(), "404")
}

func (suite *ResourceServerAPITestSuite) TestDeleteResourceServerNotFound() {
	err := deleteResourceServer("00000000-0000-0000-0000-000000000000")
	suite.NoError(err, "Delete should be idempotent")
}

// Delimiter Tests
func (suite *ResourceServerAPITestSuite) TestCreateResourceServerWithVariousDelimiters() {
	// Valid delimiters: a-zA-Z0-9._:-/
	validDelimiters := []string{":", ".", "-", "_", "/"}

	for _, delim := range validDelimiters {
		delimiter := delim
		reqBody := CreateResourceServerRequest{
			Name:      "Server With " + delim + " Delimiter",
			OUID:      testOUID,
			Delimiter: &delimiter,
		}

		rsID, err := createResourceServer(reqBody)
		suite.Require().NoError(err, "Should accept delimiter: %s", delim)
		defer deleteResourceServer(rsID)

		rs, err := getResourceServer(rsID)
		suite.Require().NoError(err)
		suite.Equal(delim, rs.Delimiter, "Delimiter should be %s", delim)
	}

	// Invalid delimiters - characters not in a-zA-Z0-9._:-/
	invalidDelimiters := []struct {
		value       string
		description string
	}{
		{"\"", "quote"},
		{"\\", "backslash"},
		{"::", "multi-character"},
		{"ñ", "non-ASCII"},
		{"#", "hash"},
		{"|", "pipe"},
		{"!", "exclamation"},
		{"@", "at"},
		{"$", "dollar"},
	}

	for _, tc := range invalidDelimiters {
		delimiter := tc.value
		reqBody := CreateResourceServerRequest{
			Name:      "Server With " + tc.description + " Delimiter",
			OUID:      testOUID,
			Delimiter: &delimiter,
		}

		_, err := createResourceServer(reqBody)
		suite.Error(err, "Should reject %s delimiter", tc.description)
		suite.Contains(err.Error(), "400", "Should return 400 for %s delimiter", tc.description)
	}
}

func (suite *ResourceServerAPITestSuite) TestDefaultSystemResourceServerHasMCPIdentifier() {
	list, err := listResourceServers(0, 100)
	suite.Require().NoError(err)

	var systemRS *ResourceServerResponse
	for i := range list.ResourceServers {
		if list.ResourceServers[i].Name == "System" {
			systemRS = &list.ResourceServers[i]
			break
		}
	}

	suite.Require().NotNil(systemRS, "Default 'System' resource server should exist")
	suite.Equal(systemResourceServerIdentifier, systemRS.Identifier)
}

// MCPResourceServerTestSuite covers MCP-specific behaviour: the action kind discriminator and the
// cross-entity handle checks that keep a resource and an action from deriving the same permission.
type MCPResourceServerTestSuite struct {
	suite.Suite
	ouID             string
	resourceServerID string
}

func TestMCPResourceServerTestSuite(t *testing.T) {
	suite.Run(t, new(MCPResourceServerTestSuite))
}

func (suite *MCPResourceServerTestSuite) SetupSuite() {
	ou := testutils.OrganizationUnit{
		Handle:      "test_mcp_resource_ou",
		Name:        "Test OU for MCP Resource Servers",
		Description: "Organization unit for MCP resource server testing",
		Parent:      nil,
	}
	ouID, err := testutils.CreateOrganizationUnit(ou)
	suite.Require().NoError(err, "Failed to create test organization unit")
	suite.ouID = ouID

	rsID, err := createResourceServer(CreateResourceServerRequest{
		Name:        "MCP Test Server",
		Description: "Resource server for MCP testing",
		Type:        "MCP",
		OUID:        ouID,
	})
	suite.Require().NoError(err, "Failed to create MCP resource server")
	suite.resourceServerID = rsID
}

func (suite *MCPResourceServerTestSuite) TearDownSuite() {
	if suite.resourceServerID != "" {
		deleteResourceServer(suite.resourceServerID)
	}
	if suite.ouID != "" {
		testutils.DeleteOrganizationUnit(suite.ouID)
	}
}

func (suite *MCPResourceServerTestSuite) TestCreateMCPResourceServer() {
	rs, err := getResourceServer(suite.resourceServerID)
	suite.Require().NoError(err)
	suite.Equal("MCP", rs.Type)
	suite.False(rs.IsReadOnly)
}

func (suite *MCPResourceServerTestSuite) TestCreateResourceServerInvalidType() {
	_, err := createResourceServer(CreateResourceServerRequest{
		Name: "Invalid Type Server",
		Type: "not-a-type",
		OUID: suite.ouID,
	})
	suite.Error(err, "Should fail with an unsupported resource server type")
	suite.Contains(err.Error(), "400")
}

func (suite *MCPResourceServerTestSuite) TestActionKindDefaultsToToolForMCP() {
	actionID, err := createActionAtResourceServer(suite.resourceServerID, CreateActionRequest{
		Name:        "Default Kind Action",
		Handle:      "default-kind",
		Description: "Action created without an explicit kind",
	})
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, actionID)

	action, err := getActionAtResourceServer(suite.resourceServerID, actionID)
	suite.Require().NoError(err)
	suite.Equal("tool", action.Kind, "MCP actions should default to the tool kind")
}

func (suite *MCPResourceServerTestSuite) TestCreateActionWithResourceKind() {
	actionID, err := createActionAtResourceServer(suite.resourceServerID, CreateActionRequest{
		Name:   "Resource Kind Action",
		Handle: "resource-kind",
		Kind:   "resource",
	})
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, actionID)

	action, err := getActionAtResourceServer(suite.resourceServerID, actionID)
	suite.Require().NoError(err)
	suite.Equal("resource", action.Kind)
}

func (suite *MCPResourceServerTestSuite) TestCreateActionWithInvalidKind() {
	_, err := createActionAtResourceServer(suite.resourceServerID, CreateActionRequest{
		Name:   "Invalid Kind Action",
		Handle: "invalid-kind",
		Kind:   "gadget",
	})
	suite.Error(err, "Should fail with an unsupported action kind")
	suite.Contains(err.Error(), "400")
}

func (suite *MCPResourceServerTestSuite) TestResourceHandleConflictsWithActionHandle() {
	actionID, err := createActionAtResourceServer(suite.resourceServerID, CreateActionRequest{
		Name:   "Billing Tool",
		Handle: "billing",
	})
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, actionID)

	_, err = createResource(suite.resourceServerID, CreateResourceRequest{
		Name:   "Billing Group",
		Handle: "billing",
		Parent: nil,
	})
	suite.Error(err, "A resource must not reuse an action handle in the same context on MCP servers")
	suite.Contains(err.Error(), "409")
}

func (suite *MCPResourceServerTestSuite) TestActionHandleConflictsWithResourceHandle() {
	resourceID, err := createResource(suite.resourceServerID, CreateResourceRequest{
		Name:   "Inventory Group",
		Handle: "inventory",
		Parent: nil,
	})
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resourceID)

	_, err = createActionAtResourceServer(suite.resourceServerID, CreateActionRequest{
		Name:   "Inventory Tool",
		Handle: "inventory",
	})
	suite.Error(err, "An action must not reuse a resource handle in the same context on MCP servers")
	suite.Contains(err.Error(), "409")
}

func (suite *MCPResourceServerTestSuite) TestListActionsFilteredByKind() {
	toolID, err := createActionAtResourceServer(suite.resourceServerID, CreateActionRequest{
		Name:   "Filter Tool",
		Handle: "filter-tool",
		Kind:   "tool",
	})
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, toolID)

	resourceKindID, err := createActionAtResourceServer(suite.resourceServerID, CreateActionRequest{
		Name:   "Filter Resource",
		Handle: "filter-resource",
		Kind:   "resource",
	})
	suite.Require().NoError(err)
	defer deleteAction(suite.resourceServerID, resourceKindID)

	tools, err := listActionsByKind(suite.resourceServerID, "", "tool")
	suite.Require().NoError(err)
	suite.Contains(actionIDs(tools), toolID)
	suite.NotContains(actionIDs(tools), resourceKindID)
	for _, action := range tools.Actions {
		suite.Equal("tool", action.Kind)
	}

	resources, err := listActionsByKind(suite.resourceServerID, "", "resource")
	suite.Require().NoError(err)
	suite.Contains(actionIDs(resources), resourceKindID)
	suite.NotContains(actionIDs(resources), toolID)
}

func (suite *MCPResourceServerTestSuite) TestListActionsAtResourceFilteredByKind() {
	resourceID, err := createResource(suite.resourceServerID, CreateResourceRequest{
		Name:   "Documents",
		Handle: "documents",
		Parent: nil,
	})
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resourceID)

	actionID, err := createActionAtResource(suite.resourceServerID, resourceID, CreateActionRequest{
		Name:   "Read Documents",
		Handle: "read",
		Kind:   "resource",
	})
	suite.Require().NoError(err)
	defer deleteActionAtResource(suite.resourceServerID, resourceID, actionID)

	resources, err := listActionsByKind(suite.resourceServerID, resourceID, "resource")
	suite.Require().NoError(err)
	suite.Equal(1, resources.TotalResults)
	suite.Equal(actionID, resources.Actions[0].ID)
	suite.Equal("documents:read", resources.Actions[0].Permission)

	tools, err := listActionsByKind(suite.resourceServerID, resourceID, "tool")
	suite.Require().NoError(err)
	suite.Equal(0, tools.TotalResults)
}

func (suite *MCPResourceServerTestSuite) TestListActionsWithInvalidKind() {
	resp, err := doRawRequest(http.MethodGet,
		resourceServerURL("/%s/actions?kind=gadget", suite.resourceServerID), nil)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Response: %s", resp.Body)
}

func (suite *MCPResourceServerTestSuite) TestListActionsAtResourceWithInvalidKind() {
	resourceID, err := createResource(suite.resourceServerID, CreateResourceRequest{
		Name:   "Invoices",
		Handle: "invoices",
		Parent: nil,
	})
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resourceID)

	resp, err := doRawRequest(http.MethodGet,
		resourceServerURL("/%s/resources/%s/actions?kind=gadget", suite.resourceServerID, resourceID), nil)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Response: %s", resp.Body)
}

// Helper functions

// rawResponse captures the status code and decoded error payload of an API call.
type rawResponse struct {
	StatusCode int
	Body       string
	Error      ErrorResponse
}

// doRawRequest sends a request with the given raw body and returns the status code along with the
// response body. Body is sent verbatim so malformed payloads can be exercised.
func doRawRequest(method, url string, body []byte) (*rawResponse, error) {
	client := testutils.GetHTTPClient()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result := &rawResponse{StatusCode: resp.StatusCode, Body: string(bodyBytes)}
	_ = json.Unmarshal(bodyBytes, &result.Error)

	return result, nil
}

// doJSONRequest marshals the payload and sends it to the given URL.
func doJSONRequest(method, url string, payload interface{}) (*rawResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return doRawRequest(method, url, body)
}

// resourceServerURL builds a URL under the resource server API.
func resourceServerURL(format string, args ...interface{}) string {
	return testServerURL + "/resource-servers" + fmt.Sprintf(format, args...)
}

func createResourceServer(req CreateResourceServerRequest) (string, error) {
	client := testutils.GetHTTPClient()

	if req.Identifier == "" {
		req.Identifier = fmt.Sprintf("https://api.example.com/integration/%d", time.Now().UnixNano())
	}

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", testServerURL+"/resource-servers", bytes.NewBuffer(body))
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

	var rs ResourceServerResponse
	if err := json.NewDecoder(resp.Body).Decode(&rs); err != nil {
		return "", err
	}

	return rs.ID, nil
}

func getResourceServer(id string) (*ResourceServerResponse, error) {
	client := testutils.GetHTTPClient()

	httpReq, err := http.NewRequest("GET", testServerURL+"/resource-servers/"+id, nil)
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

	var rs ResourceServerResponse
	if err := json.NewDecoder(resp.Body).Decode(&rs); err != nil {
		return nil, err
	}

	return &rs, nil
}

func listResourceServers(offset, limit int) (*ResourceServerListResponse, error) {
	client := testutils.GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers?offset=%d&limit=%d", testServerURL, offset, limit)
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

	var list ResourceServerListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	return &list, nil
}

func updateResourceServer(id string, req UpdateResourceServerRequest) error {
	client := testutils.GetHTTPClient()

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("PUT", testServerURL+"/resource-servers/"+id, bytes.NewBuffer(body))
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

func deleteResourceServer(id string) error {
	client := testutils.GetHTTPClient()

	httpReq, err := http.NewRequest("DELETE", testServerURL+"/resource-servers/"+id, nil)
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
