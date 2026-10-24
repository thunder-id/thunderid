// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// Declarative fixtures loaded from tests/integration/resources/declarative_resources/resource_servers.
	declarativeResourceServerID       = "decl-rs-1"
	declarativeResourceHandle         = "test-resource"
	declarativeOUHandleServerID       = "decl-rs-2"
	declarativeOUHandleServerName     = "Declarative OU Handle Resource Server"
	declarativeMCPServerID            = "decl-rs-3"
	declarativeRepeatedHandleServerID = "decl-rs-4"
	declarativeRepeatedActionServerID = "decl-rs-5"
	declarativeReadActionHandle       = "read"
	declarativeWriteActionHandle      = "write"
	declarativeResourcePermission     = "test-resource"
	declarativeReadPermission         = "test-resource:read"
	declarativeImmutableResourceCode  = "RES-1019"
	declarativeImmutableActionCode    = "RES-1020"
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

// TestDeclarativeRepeatedResourceHandle verifies that a handle repeated under different parents
// gives each resource its own ID, permission, and actions.
func (suite *DeclarativeResourceServerTestSuite) TestDeclarativeRepeatedResourceHandle() {
	childByHandle := func(parentID, handle string) ResourceResponse {
		children, err := listResources(declarativeRepeatedHandleServerID, parentID, 0, 100)
		suite.Require().NoError(err)
		for _, res := range children.Resources {
			if res.Handle == handle {
				return res
			}
		}
		suite.FailNowf("resource not found", "no child %q under %q", handle, parentID)
		return ResourceResponse{}
	}

	top, err := listResources(declarativeRepeatedHandleServerID, "", 0, 100)
	suite.Require().NoError(err)
	suite.Require().Equal(1, top.TotalResults)
	app := top.Resources[0]
	suite.Equal("app", app.Permission)

	project := childByHandle(app.ID, "project")
	team := childByHandle(app.ID, "team")
	projectMember := childByHandle(project.ID, "member")
	teamMember := childByHandle(team.ID, "member")
	suite.NotEqual(projectMember.ID, teamMember.ID, "A repeated handle gets its own ID under each parent")

	for _, tc := range []struct {
		member     ResourceResponse
		parentID   string
		name       string
		permission string
		action     string
	}{
		{projectMember, project.ID, "Project Member", "app:project:member", "app:project:member:read"},
		{teamMember, team.ID, "Team Member", "app:team:member", "app:team:member:write"},
	} {
		res, err := getResource(declarativeRepeatedHandleServerID, tc.member.ID)
		suite.Require().NoError(err)
		suite.Equal(tc.name, res.Name)
		suite.Equal(tc.permission, res.Permission)
		suite.Require().NotNil(res.Parent)
		suite.Equal(tc.parentID, *res.Parent)

		actions, err := listActionsAtResource(declarativeRepeatedHandleServerID, tc.member.ID, 0, 100)
		suite.Require().NoError(err)
		suite.Require().Equal(1, actions.TotalResults)
		suite.Equal(tc.action, actions.Actions[0].Permission)
	}

	lead := childByHandle(teamMember.ID, "lead")
	suite.Equal("app:team:member:lead", lead.Permission,
		"A resource listed before its parent is attached to the parent its path names")
}

// TestDeclarativeRepeatedHandleIDsAndActions verifies the ID of a declarative resource and action is
// the resource server ID, the delimiter and the path, and that an action handle repeated under
// different resources resolves only within the resource it is requested under.
func (suite *DeclarativeResourceServerTestSuite) TestDeclarativeRepeatedHandleIDsAndActions() {
	rs, err := getResourceServer(declarativeRepeatedActionServerID)
	suite.Require().NoError(err)
	suite.Equal(".", rs.Delimiter)

	top, err := listResources(declarativeRepeatedActionServerID, "", 0, 100)
	suite.Require().NoError(err)
	suite.Require().Equal(1, top.TotalResults)
	suite.Equal("decl-rs-5.app", top.Resources[0].ID)

	type member struct {
		id, permission, actionID string
	}
	members := map[string]*member{
		"project": {id: "decl-rs-5.app.project.member", permission: "app.project.member"},
		"team":    {id: "decl-rs-5.app.team.member", permission: "app.team.member"},
	}
	for parent, m := range members {
		parentRes, err := getResource(declarativeRepeatedActionServerID, "decl-rs-5.app."+parent)
		suite.Require().NoError(err)

		children, err := listResources(declarativeRepeatedActionServerID, parentRes.ID, 0, 100)
		suite.Require().NoError(err)
		suite.Require().Equal(1, children.TotalResults)
		suite.Equal(m.id, children.Resources[0].ID)
		suite.Equal(m.permission, children.Resources[0].Permission)

		res, err := getResource(declarativeRepeatedActionServerID, m.id)
		suite.Require().NoError(err)
		suite.Require().NotNil(res.Parent)
		suite.Equal(parentRes.ID, *res.Parent)

		actions, err := listActionsAtResource(declarativeRepeatedActionServerID, m.id, 0, 100)
		suite.Require().NoError(err)
		suite.Require().Equal(1, actions.TotalResults)
		suite.Equal(m.id+".read", actions.Actions[0].ID)
		suite.Equal(m.permission+".read", actions.Actions[0].Permission)
		m.actionID = actions.Actions[0].ID

		action, err := getActionAtResource(declarativeRepeatedActionServerID, m.id, m.actionID)
		suite.Require().NoError(err)
		suite.Equal(m.permission+".read", action.Permission)
	}

	_, err = getActionAtResource(declarativeRepeatedActionServerID, members["team"].id, members["project"].actionID)
	suite.Require().Error(err, "An action is not found under a resource that does not hold it")
	suite.Contains(err.Error(), "404")
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
	suite.Contains(yamlContent, "identifier: {{."+exportIdentifierVariable+"}}",
		"The identifier is the deployment's own, so it is exported as a template variable")
	suite.NotContains(yamlContent, "identifier: https://api.example.com/export-rs")
	suite.Contains(yamlContent, `delimiter: ":"`)
	suite.Contains(yamlContent, "handle: catalog")
	suite.Contains(yamlContent, "handle: items")
	suite.Contains(yamlContent, "parent: catalog", "Child resources should reference their parent handle")
	suite.Contains(yamlContent, "handle: list", "Nested actions should be exported")
}

// The environment file beside the export carries the identifier the template variable stands for.
func (suite *ResourceServerExportTestSuite) TestExportCarriesTheIdentifierInTheEnvironmentFile() {
	export, err := exportResourceServersResponse([]string{suite.resourceServerID})
	suite.Require().NoError(err)

	suite.Contains(export.EnvironmentVariables, exportIdentifierVariable+"=https://api.example.com/export-rs")
}

// An export imported with another value for the identifier variable serves under that identifier,
// its delimiter and resources as they were.
func (suite *ResourceServerExportTestSuite) TestImportFillsTheIdentifierFromTheVariables() {
	export, err := exportResourceServersResponse([]string{suite.resourceServerID})
	suite.Require().NoError(err)
	defer func() {
		_, err := doJSONRequest(http.MethodPut, resourceServerURL("/%s", suite.resourceServerID),
			UpdateResourceServerRequest{
				Name:       "Export Resource Server",
				Identifier: "https://api.example.com/export-rs",
				OUID:       suite.ouID,
			})
		suite.NoError(err)
	}()

	resp, err := doJSONRequest(http.MethodPost, testServerURL+"/import", map[string]interface{}{
		"content":   export.Resources,
		"variables": map[string]string{exportIdentifierVariable: "https://api.example.com/export-rs-moved"},
		"options":   map[string]interface{}{"target": "runtime", "upsert": true},
	})
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, resp.StatusCode, resp.Body)
	var outcome struct {
		Summary struct {
			Imported int `json:"imported"`
			Failed   int `json:"failed"`
		} `json:"summary"`
	}
	suite.Require().NoError(json.Unmarshal([]byte(resp.Body), &outcome))
	suite.Require().Equal(0, outcome.Summary.Failed, resp.Body)
	suite.Require().Equal(1, outcome.Summary.Imported, resp.Body)

	server, err := getResourceServer(suite.resourceServerID)
	suite.Require().NoError(err)
	suite.Equal("https://api.example.com/export-rs-moved", server.Identifier)
	suite.Equal(":", server.Delimiter)
	resources, err := listResources(suite.resourceServerID, "", 0, 100)
	suite.Require().NoError(err)
	handles := make([]string, 0, len(resources.Resources))
	for _, res := range resources.Resources {
		handles = append(handles, res.Handle)
	}
	suite.Contains(handles, "catalog")
}

// TestRepeatedHandleExportAndImport verifies a resource tree that repeats a handle under different
// parents exports each parent as its full path and imports back with every resource under its own
// parent.
func (suite *ResourceServerExportTestSuite) TestRepeatedHandleExportAndImport() {
	rsID, err := createResourceServer(CreateResourceServerRequest{
		Name:       "Repeated Handle Export",
		Identifier: "https://api.example.com/repeated-handle-export",
		OUID:       suite.ouID,
	})
	suite.Require().NoError(err)
	defer func() { suite.NoError(testutils.DeleteResourceServerWithChildren(rsID)) }()

	create := func(name, handle string, parent *string) string {
		id, err := createResource(rsID, CreateResourceRequest{Name: name, Handle: handle, Parent: parent})
		suite.Require().NoError(err)
		return id
	}
	app := create("App", "app", nil)
	project := create("Project", "project", &app)
	team := create("Team", "team", &app)
	for _, parent := range []string{project, team} {
		member := create("Member", "member", &parent)
		_, err = createActionAtResource(rsID, member, CreateActionRequest{Name: "Read", Handle: "read"})
		suite.Require().NoError(err)
	}

	export, err := exportResourceServersResponse([]string{rsID})
	suite.Require().NoError(err)
	suite.Contains(export.Resources, "parent: app:project", "A nested parent is exported as its full path")
	suite.Contains(export.Resources, "parent: app:team")

	variables := map[string]string{}
	for _, line := range strings.Split(export.EnvironmentVariables, "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			variables[key] = value
		}
	}
	suite.Require().NotEmpty(variables)
	suite.Require().NoError(testutils.DeleteResourceServerWithChildren(rsID))

	resp, err := doJSONRequest(http.MethodPost, testServerURL+"/import", map[string]interface{}{
		"content":   export.Resources,
		"variables": variables,
		"options":   map[string]interface{}{"target": "runtime", "upsert": true},
	})
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, resp.StatusCode, resp.Body)
	var outcome struct {
		Summary struct {
			Imported int `json:"imported"`
			Failed   int `json:"failed"`
		} `json:"summary"`
	}
	suite.Require().NoError(json.Unmarshal([]byte(resp.Body), &outcome))
	suite.Require().Equal(0, outcome.Summary.Failed, resp.Body)

	top, err := listResources(rsID, "", 0, 100)
	suite.Require().NoError(err)
	suite.Require().Equal(1, top.TotalResults)
	children, err := listResources(rsID, top.Resources[0].ID, 0, 100)
	suite.Require().NoError(err)
	suite.Require().Equal(2, children.TotalResults)

	for _, child := range children.Resources {
		members, err := listResources(rsID, child.ID, 0, 100)
		suite.Require().NoError(err)
		suite.Require().Equal(1, members.TotalResults, "%s holds its own member", child.Handle)
		suite.Equal("member", members.Resources[0].Handle)
		suite.Equal("app:"+child.Handle+":member", members.Resources[0].Permission)

		actions, err := listActionsAtResource(rsID, members.Resources[0].ID, 0, 100)
		suite.Require().NoError(err)
		suite.Require().Equal(1, actions.TotalResults)
		suite.Equal("app:"+child.Handle+":member:read", actions.Actions[0].Permission)
	}
}

// TestImportRejectsInvalidResourceTrees verifies the import API refuses a resource server whose
// resources cannot be resolved to unique paths, and names the cause.
func (suite *ResourceServerExportTestSuite) TestImportRejectsInvalidResourceTrees() {
	tests := []struct {
		name      string
		delimiter string
		resources string
		errSubstr string
	}{
		{
			name: "same handle under the same parent",
			resources: `
  - { name: App, handle: app }
  - { name: Member, handle: member, parent: app }
  - { name: Member Again, handle: member, parent: app }
`,
			errSubstr: "duplicate resource handle 'member' found under parent 'app'",
		},
		{
			name: "same handle at the top level",
			resources: `
  - { name: App, handle: app }
  - { name: App Again, handle: app }
`,
			errSubstr: "duplicate resource handle 'app' found at the top level",
		},
		{
			name: "bare parent that names no resource",
			resources: `
  - { name: Child, handle: child, parent: ghost }
`,
			errSubstr: "parent resource handle 'ghost' not found for resource 'child'",
		},
		{
			name: "bare parent shared by several resources",
			resources: `
  - { name: App, handle: app }
  - { name: Project, handle: project, parent: app }
  - { name: Team, handle: team, parent: app }
  - { name: Project Member, handle: member, parent: project }
  - { name: Team Member, handle: member, parent: team }
  - { name: Lead, handle: lead, parent: member }
`,
			errSubstr: "parent resource handle 'member' is ambiguous for resource 'lead'",
		},
		{
			name: "parent path that names no resource",
			resources: `
  - { name: App, handle: app }
  - { name: Member, handle: member, parent: "app:ghost" }
`,
			errSubstr: "parent resource path 'app:ghost' not found for resource 'member'",
		},
		{
			name: "resources that are each other's parent",
			resources: `
  - { name: A, handle: a, parent: b }
  - { name: B, handle: b, parent: a }
`,
			errSubstr: "circular parent reference detected",
		},
		{
			name: "resource handle that contains the delimiter",
			resources: `
  - { name: App, handle: "app:one" }
`,
			errSubstr: "invalid handle 'app:one' for resource 'App'",
		},
		{
			name: "action handle that contains the delimiter",
			resources: `
  - name: App
    handle: app
    actions:
      - { name: Read, handle: "read:all" }
`,
			errSubstr: "invalid handle 'read:all' for action 'Read'",
		},
		{
			name:      "delimiter outside the allowed set",
			delimiter: "#",
			resources: `
  - { name: App, handle: app }
`,
			errSubstr: "delimiter",
		},
	}

	for i, tc := range tests {
		suite.Run(tc.name, func() {
			identifier := fmt.Sprintf("https://api.example.com/invalid-tree-%d", i)
			defer func() {
				// A rejected import can leave the resource server it created behind.
				servers, err := listResourceServers(0, 100)
				if err != nil {
					return
				}
				for _, server := range servers.ResourceServers {
					if server.Identifier == identifier {
						_ = deleteResourceServer(server.ID)
					}
				}
			}()

			delimiter := tc.delimiter
			if delimiter == "" {
				delimiter = ":"
			}
			content := fmt.Sprintf(
				"resource_type: resource_server\nname: Invalid Tree %d\nidentifier: %s\nouId: %s\n"+
					"delimiter: \"%s\"\nresources:%s", i, identifier, suite.ouID, delimiter, tc.resources)
			resp, err := doJSONRequest(http.MethodPost, testServerURL+"/import", map[string]interface{}{
				"content": content,
				"options": map[string]interface{}{"target": "runtime", "upsert": true},
			})
			suite.Require().NoError(err)

			var outcome struct {
				Summary struct {
					Failed int `json:"failed"`
				} `json:"summary"`
			}
			suite.Require().NoError(json.Unmarshal([]byte(resp.Body), &outcome), resp.Body)
			suite.Equal(1, outcome.Summary.Failed, resp.Body)
			suite.Contains(resp.Body, tc.errSubstr)
		})
	}
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

// exportIdentifierVariable is the variable the export of the suite's resource server names its
// identifier by.
const exportIdentifierVariable = "RESOURCE_SERVER_EXPORT_RESOURCE_SERVER_IDENTIFIER"

// exportResourceServers exports the given resource servers and returns the generated YAML.
func exportResourceServers(ids []string) (string, error) {
	export, err := exportResourceServersResponse(ids)
	if err != nil {
		return "", err
	}
	return export.Resources, nil
}

// exportResourceServersResponse exports the given resource servers and returns the export's answer.
func exportResourceServersResponse(ids []string) (*exportResponse, error) {
	resp, err := doJSONRequest(http.MethodPost, testServerURL+"/export", exportRequest{ResourceServers: ids})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, resp.Body)
	}

	var export exportResponse
	if err := json.Unmarshal([]byte(resp.Body), &export); err != nil {
		return nil, err
	}
	return &export, nil
}
