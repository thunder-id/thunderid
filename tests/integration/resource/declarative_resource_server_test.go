// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// Declarative fixtures loaded from tests/integration/resources/declarative_resources/resource_servers.
	declarativeResourceServerID      = "decl-rs-1"
	declarativeResourceHandle        = "test-resource"
	declarativeOUHandleServerID      = "decl-rs-2"
	declarativeOUHandleServerName    = "Declarative OU Handle Resource Server"
	declarativeMCPServerID           = "decl-rs-3"
	declarativeReadActionHandle      = "read"
	declarativeWriteActionHandle     = "write"
	declarativeResourcePermission    = "test-resource"
	declarativeReadPermission        = "test-resource:read"
	declarativeImmutableResourceCode = "RES-1019"
	declarativeImmutableActionCode   = "RES-1020"
)

// DeclarativeResourceServerTestSuite covers the read paths of file-backed (declarative) resource
// servers in composite mode, and the immutability guards on their resources and actions.
type DeclarativeResourceServerTestSuite struct {
	suite.Suite
	resourceID string
	actionID   string
}

func TestDeclarativeResourceServerTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeResourceServerTestSuite))
}

func (suite *DeclarativeResourceServerTestSuite) SetupSuite() {
	list, err := listResources(declarativeResourceServerID, "", 0, 100)
	suite.Require().NoError(err, "Failed to list declarative resources")
	suite.Require().NotEmpty(list.Resources, "Declarative resource server should expose its resources")

	for _, res := range list.Resources {
		if res.Handle == declarativeResourceHandle {
			suite.resourceID = res.ID
		}
	}
	suite.Require().NotEmpty(suite.resourceID, "Declarative resource %q should exist", declarativeResourceHandle)

	actions, err := listActionsAtResource(declarativeResourceServerID, suite.resourceID, 0, 100)
	suite.Require().NoError(err, "Failed to list declarative actions")
	for _, action := range actions.Actions {
		if action.Handle == declarativeReadActionHandle {
			suite.actionID = action.ID
		}
	}
	suite.Require().NotEmpty(suite.actionID, "Declarative action %q should exist", declarativeReadActionHandle)
}

func (suite *DeclarativeResourceServerTestSuite) TestGetDeclarativeResourceServer() {
	rs, err := getResourceServer(declarativeResourceServerID)
	suite.Require().NoError(err)
	suite.Equal(declarativeResourceServerID, rs.ID)
	suite.Equal("Declarative Resource Server", rs.Name)
	suite.Equal(":", rs.Delimiter)
	suite.True(rs.IsReadOnly, "Declarative resource server should be reported as read-only")
}

func (suite *DeclarativeResourceServerTestSuite) TestListMergesDeclarativeAndRuntimeResourceServers() {
	list, err := listResourceServers(0, 100)
	suite.Require().NoError(err)

	found := false
	for _, rs := range list.ResourceServers {
		if rs.ID == declarativeResourceServerID {
			found = true
			suite.True(rs.IsReadOnly, "Declarative resource server should be read-only in list responses")
		}
	}
	suite.True(found, "Merged list should contain the declarative resource server")
}

func (suite *DeclarativeResourceServerTestSuite) TestListDeclarativeResources() {
	list, err := listResources(declarativeResourceServerID, "", 0, 100)
	suite.Require().NoError(err)
	suite.GreaterOrEqual(list.TotalResults, 1)

	found := false
	for _, res := range list.Resources {
		if res.Handle == declarativeResourceHandle {
			found = true
			suite.Equal(declarativeResourcePermission, res.Permission)
			suite.Nil(res.Parent, "Declarative test resource is top level")
		}
	}
	suite.True(found, "Declarative resource should be listed")
}

func (suite *DeclarativeResourceServerTestSuite) TestListDeclarativeResourcesByParent() {
	list, err := listResources(declarativeResourceServerID, suite.resourceID, 0, 100)
	suite.Require().NoError(err)
	suite.Equal(0, list.TotalResults, "Declarative test resource has no children")
}

func (suite *DeclarativeResourceServerTestSuite) TestGetDeclarativeResource() {
	res, err := getResource(declarativeResourceServerID, suite.resourceID)
	suite.Require().NoError(err)
	suite.Equal(declarativeResourceHandle, res.Handle)
	suite.Equal(declarativeResourcePermission, res.Permission)
}

func (suite *DeclarativeResourceServerTestSuite) TestListDeclarativeActionsAtResource() {
	list, err := listActionsAtResource(declarativeResourceServerID, suite.resourceID, 0, 100)
	suite.Require().NoError(err)
	suite.Equal(2, list.TotalResults, "Declarative resource declares read and write actions")

	permissions := map[string]string{}
	for _, action := range list.Actions {
		permissions[action.Handle] = action.Permission
	}
	suite.Equal(declarativeReadPermission, permissions[declarativeReadActionHandle])
	suite.Equal("test-resource:write", permissions[declarativeWriteActionHandle])
}

func (suite *DeclarativeResourceServerTestSuite) TestGetDeclarativeActionAtResource() {
	action, err := getActionAtResource(declarativeResourceServerID, suite.resourceID, suite.actionID)
	suite.Require().NoError(err)
	suite.Equal(declarativeReadActionHandle, action.Handle)
	suite.Equal(declarativeReadPermission, action.Permission)
}

func (suite *DeclarativeResourceServerTestSuite) TestUpdateDeclarativeResourceRejected() {
	resp, err := doJSONRequest(http.MethodPut,
		resourceServerURL("/%s/resources/%s", declarativeResourceServerID, suite.resourceID),
		UpdateResourceRequest{Name: "Renamed Declarative Resource"})
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Response: %s", resp.Body)
	suite.Equal(declarativeImmutableResourceCode, resp.Error.Code)
}

func (suite *DeclarativeResourceServerTestSuite) TestDeleteDeclarativeResourceRejected() {
	resp, err := doRawRequest(http.MethodDelete,
		resourceServerURL("/%s/resources/%s", declarativeResourceServerID, suite.resourceID), nil)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Response: %s", resp.Body)
	suite.Equal(declarativeImmutableResourceCode, resp.Error.Code)
}

func (suite *DeclarativeResourceServerTestSuite) TestUpdateDeclarativeActionRejected() {
	resp, err := doJSONRequest(http.MethodPut,
		resourceServerURL("/%s/resources/%s/actions/%s",
			declarativeResourceServerID, suite.resourceID, suite.actionID),
		UpdateActionRequest{Name: "Renamed Declarative Action"})
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Response: %s", resp.Body)
	suite.Equal(declarativeImmutableActionCode, resp.Error.Code)
}

func (suite *DeclarativeResourceServerTestSuite) TestDeleteDeclarativeActionRejected() {
	resp, err := doRawRequest(http.MethodDelete,
		resourceServerURL("/%s/resources/%s/actions/%s",
			declarativeResourceServerID, suite.resourceID, suite.actionID), nil)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Response: %s", resp.Body)
	suite.Equal(declarativeImmutableActionCode, resp.Error.Code)
}

func (suite *DeclarativeResourceServerTestSuite) TestUpdateDeclarativeActionAtResourceServerRejected() {
	resp, err := doJSONRequest(http.MethodPut,
		resourceServerURL("/%s/actions/%s", declarativeResourceServerID, suite.actionID),
		UpdateActionRequest{Name: "Renamed Declarative Action"})
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Response: %s", resp.Body)
	suite.Equal(declarativeImmutableActionCode, resp.Error.Code)
}

func (suite *DeclarativeResourceServerTestSuite) TestDeleteDeclarativeActionAtResourceServerRejected() {
	resp, err := doRawRequest(http.MethodDelete,
		resourceServerURL("/%s/actions/%s", declarativeResourceServerID, suite.actionID), nil)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Response: %s", resp.Body)
	suite.Equal(declarativeImmutableActionCode, resp.Error.Code)
}

// TestDeclarativeResourceServerResolvedByOUHandle verifies the declarative loader resolves
// ouHandle to an organization unit ID for file-backed resource servers.
func (suite *DeclarativeResourceServerTestSuite) TestDeclarativeResourceServerResolvedByOUHandle() {
	rs, err := getResourceServer(declarativeOUHandleServerID)
	suite.Require().NoError(err)
	suite.Equal(declarativeOUHandleServerName, rs.Name)
	suite.NotEmpty(rs.OUID, "ouHandle should be resolved to an organization unit ID at load time")
	suite.True(rs.IsReadOnly)

	list, err := listResources(declarativeOUHandleServerID, "", 0, 100)
	suite.Require().NoError(err)
	suite.Equal(1, list.TotalResults)
	suite.Equal("reports", list.Resources[0].Handle)
	suite.Equal("reports", list.Resources[0].Permission)
}

// TestDeclarativeMCPResourceServer covers the MCP rules applied while loading declarative resource
// servers: the action kind defaulting, nested permission derivation and the kind filter over the
// file-backed store.
func (suite *DeclarativeResourceServerTestSuite) TestDeclarativeMCPResourceServer() {
	rs, err := getResourceServer(declarativeMCPServerID)
	suite.Require().NoError(err)
	suite.Equal("MCP", rs.Type)
	suite.True(rs.IsReadOnly)

	list, err := listResources(declarativeMCPServerID, "", 0, 100)
	suite.Require().NoError(err)
	suite.Require().Equal(2, list.TotalResults, "Only top level resources are listed without a parent filter")
	suite.Equal("tools", list.Resources[0].Handle)
	suite.Equal("tools", list.Resources[0].Permission)
	suite.Equal("prompts", list.Resources[1].Handle)
	toolsID := list.Resources[0].ID

	children, err := listResources(declarativeMCPServerID, toolsID, 0, 100)
	suite.Require().NoError(err)
	suite.Equal(1, children.TotalResults)
	suite.Equal("nested", children.Resources[0].Handle)
	suite.Equal("tools:nested", children.Resources[0].Permission,
		"Nested declarative resources derive a chained permission")

	actions, err := listActionsAtResource(declarativeMCPServerID, toolsID, 0, 100)
	suite.Require().NoError(err)
	suite.Equal(2, actions.TotalResults)

	kinds := map[string]string{}
	for _, action := range actions.Actions {
		kinds[action.Handle] = action.Kind
	}
	suite.Equal("tool", kinds["search"], "MCP actions default to the tool kind when loaded from a file")
	suite.Equal("resource", kinds["fetch"])
}

func (suite *DeclarativeResourceServerTestSuite) TestListDeclarativeActionsFilteredByKind() {
	list, err := listResources(declarativeMCPServerID, "", 0, 100)
	suite.Require().NoError(err)

	var toolsResourceID string
	for _, res := range list.Resources {
		if res.Handle == "tools" {
			toolsResourceID = res.ID
		}
	}
	suite.Require().NotEmpty(toolsResourceID)

	tools, err := listActionsByKind(declarativeMCPServerID, toolsResourceID, "tool")
	suite.Require().NoError(err)
	suite.Equal(1, tools.TotalResults)
	suite.Equal("search", tools.Actions[0].Handle)

	resources, err := listActionsByKind(declarativeMCPServerID, toolsResourceID, "resource")
	suite.Require().NoError(err)
	suite.Equal(1, resources.TotalResults)
	suite.Equal("fetch", resources.Actions[0].Handle)
	suite.Equal("tools:fetch", resources.Actions[0].Permission)
}

// exportRequest is the request payload of the export API, limited to the fields used here.
type exportRequest struct {
	ResourceServers []string `json:"resourceServers,omitempty"`
}

// exportResponse is the JSON payload returned by the export API.
type exportResponse struct {
	Resources            string `json:"resources"`
	EnvironmentVariables string `json:"environment_variables"`
}

// ResourceServerExportTestSuite covers the declarative export path of resource servers, which walks
// the full resource tree and its actions.
type ResourceServerExportTestSuite struct {
	suite.Suite
	ouID             string
	resourceServerID string
	parentID         string
	childID          string
}

func TestResourceServerExportTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServerExportTestSuite))
}

func (suite *ResourceServerExportTestSuite) SetupSuite() {
	ou := testutils.OrganizationUnit{
		Handle:      "test_resource_export_ou",
		Name:        "Test OU for Resource Server Export",
		Description: "Organization unit for resource server export testing",
		Parent:      nil,
	}
	ouID, err := testutils.CreateOrganizationUnit(ou)
	suite.Require().NoError(err, "Failed to create test organization unit")
	suite.ouID = ouID

	rsID, err := createResourceServer(CreateResourceServerRequest{
		Name:        "Export Resource Server",
		Description: "Resource server for export testing",
		Identifier:  "https://api.example.com/export-rs",
		OUID:        ouID,
	})
	suite.Require().NoError(err, "Failed to create test resource server")
	suite.resourceServerID = rsID

	parentID, err := createResource(rsID, CreateResourceRequest{
		Name:        "Catalog",
		Handle:      "catalog",
		Description: "Top level catalog resource",
		Parent:      nil,
	})
	suite.Require().NoError(err, "Failed to create parent resource")
	suite.parentID = parentID

	childID, err := createResource(rsID, CreateResourceRequest{
		Name:   "Items",
		Handle: "items",
		Parent: &parentID,
	})
	suite.Require().NoError(err, "Failed to create child resource")
	suite.childID = childID

	_, err = createActionAtResource(rsID, childID, CreateActionRequest{
		Name:        "List Items",
		Handle:      "list",
		Description: "List catalog items",
	})
	suite.Require().NoError(err, "Failed to create action")
}

func (suite *ResourceServerExportTestSuite) TearDownSuite() {
	// Actions and child resources must go before their parents.
	if suite.childID != "" {
		actions, err := listActionsAtResource(suite.resourceServerID, suite.childID, 0, 100)
		if err == nil {
			for _, action := range actions.Actions {
				deleteActionAtResource(suite.resourceServerID, suite.childID, action.ID)
			}
		}
		deleteResource(suite.resourceServerID, suite.childID)
	}
	if suite.parentID != "" {
		deleteResource(suite.resourceServerID, suite.parentID)
	}
	if suite.resourceServerID != "" {
		deleteResourceServer(suite.resourceServerID)
	}
	if suite.ouID != "" {
		testutils.DeleteOrganizationUnit(suite.ouID)
	}
}

func (suite *ResourceServerExportTestSuite) TestExportResourceServerByID() {
	yamlContent, err := exportResourceServers([]string{suite.resourceServerID})
	suite.Require().NoError(err)
	suite.Require().NotEmpty(yamlContent)

	suite.Contains(yamlContent, "resource_type: resource_server")
	suite.Contains(yamlContent, "name: Export Resource Server")
	suite.Contains(yamlContent, "identifier: https://api.example.com/export-rs")
	suite.Contains(yamlContent, "handle: catalog")
	suite.Contains(yamlContent, "handle: items")
	suite.Contains(yamlContent, "parent: catalog", "Child resources should reference their parent handle")
	suite.Contains(yamlContent, "handle: list", "Nested actions should be exported")
}

func (suite *ResourceServerExportTestSuite) TestExportResourceServersWithWildcard() {
	yamlContent, err := exportResourceServers([]string{"*"})
	suite.Require().NoError(err)
	suite.Require().NotEmpty(yamlContent)

	suite.Contains(yamlContent, "name: Export Resource Server")
	suite.NotContains(yamlContent, "name: Declarative Resource Server",
		"Declarative resource servers are excluded from wildcard export")
}

func (suite *ResourceServerExportTestSuite) TestExportDeclarativeResourceServerByID() {
	yamlContent, err := exportResourceServers([]string{declarativeResourceServerID})
	suite.Require().NoError(err)
	suite.Require().NotEmpty(yamlContent)

	suite.Contains(yamlContent, "name: Declarative Resource Server")
	suite.Contains(yamlContent, "handle: "+declarativeResourceHandle)
	suite.Contains(yamlContent, "handle: "+declarativeReadActionHandle)
}

func (suite *ResourceServerExportTestSuite) TestExportNonExistentResourceServer() {
	_, err := exportResourceServers([]string{"00000000-0000-0000-0000-000000000000"})
	suite.Error(err, "Exporting an unknown resource server should fail")
}

// exportResourceServers exports the given resource servers and returns the generated YAML.
func exportResourceServers(ids []string) (string, error) {
	resp, err := doJSONRequest(http.MethodPost, testServerURL+"/export", exportRequest{ResourceServers: ids})
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, resp.Body)
	}

	var export exportResponse
	if err := json.Unmarshal([]byte(resp.Body), &export); err != nil {
		return "", err
	}
	return export.Resources, nil
}
