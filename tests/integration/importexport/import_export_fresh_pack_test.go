/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package importexport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

type exportRequest struct {
	Applications      []string `json:"applications,omitempty"`
	OrganizationUnits []string `json:"organizationUnits,omitempty"`
	Flows             []string `json:"flows,omitempty"`
	Themes            []string `json:"themes,omitempty"`
	Layouts           []string `json:"layouts,omitempty"`
}

type importRequest struct {
	Content string                 `json:"content"`
	DryRun  bool                   `json:"dryRun,omitempty"`
	Options importOptions          `json:"options"`
	Vars    map[string]interface{} `json:"variables,omitempty"`
}

type importOptions struct {
	Upsert          bool   `json:"upsert"`
	ContinueOnError bool   `json:"continueOnError"`
	Target          string `json:"target"`
}

type importResponse struct {
	Summary importSummary `json:"summary"`
	Results []importItem  `json:"results"`
}

type importSummary struct {
	TotalDocuments int `json:"totalDocuments"`
	Imported       int `json:"imported"`
	Failed         int `json:"failed"`
}

type importItem struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
	Operation    string `json:"operation,omitempty"`
	Status       string `json:"status"`
	Code         string `json:"code,omitempty"`
	Message      string `json:"message,omitempty"`
}

type createThemeRequest struct {
	Handle      string                 `json:"handle"`
	DisplayName string                 `json:"displayName"`
	Description string                 `json:"description,omitempty"`
	Theme       map[string]interface{} `json:"theme"`
}

type createLayoutRequest struct {
	Handle      string                 `json:"handle"`
	DisplayName string                 `json:"displayName"`
	Description string                 `json:"description,omitempty"`
	Layout      map[string]interface{} `json:"layout"`
}

type exportFile struct {
	FileName string `json:"fileName"`
	Content  string `json:"content"`
}

type ImportExportFreshPackSuite struct {
	suite.Suite
}

func TestImportExportFreshPackSuite(t *testing.T) {
	suite.Run(t, new(ImportExportFreshPackSuite))
}

func (suite *ImportExportFreshPackSuite) SetupSuite() {
	if os.Getenv("SERVER_EXTRACTED_HOME") == "" {
		suite.T().Skip("requires integration harness context (SERVER_EXTRACTED_HOME is not set)")
	}
}

func (suite *ImportExportFreshPackSuite) TearDownSuite() {
	if os.Getenv("SERVER_EXTRACTED_HOME") == "" {
		return
	}
	if testutils.GetDBType() != "sqlite" {
		return
	}

	err := suite.resetToFreshPack()
	if err != nil {
		suite.T().Logf("failed to restore fresh pack in teardown: %v", err)
	}
}

func (suite *ImportExportFreshPackSuite) TestExportImportAcrossFreshPack() {
	if testutils.GetDBType() != "sqlite" {
		suite.T().Skip("fresh-pack reset integration test currently supports sqlite only")
	}

	now := time.Now().UnixNano()
	handleSuffix := fmt.Sprintf("%d", now)

	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Import Export Auth Flow " + handleSuffix,
		FlowType: "AUTHENTICATION",
		Handle:   "import-export-auth-flow-" + handleSuffix,
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "auth_assert",
			},
			{
				"id":   "auth_assert",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthAssertExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	})
	suite.Require().NoError(err)

	themeID, err := suite.createTheme(createThemeRequest{
		Handle:      "import-export-theme-" + handleSuffix,
		DisplayName: "Import Export Theme " + handleSuffix,
		Description: "Theme for export/import fresh-pack test",
		Theme: map[string]interface{}{
			"palette": map[string]interface{}{
				"primary": "#0F766E",
				"accent":  "#FB923C",
			},
		},
	})
	suite.Require().NoError(err)

	layoutID, err := suite.createLayout(createLayoutRequest{
		Handle:      "import-export-layout-" + handleSuffix,
		DisplayName: "Import Export Layout " + handleSuffix,
		Description: "Layout for export/import fresh-pack test",
		Layout: map[string]interface{}{
			"layout": map[string]interface{}{
				"version": 1,
			},
		},
	})
	suite.Require().NoError(err)

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "import-export-ou-" + handleSuffix,
		Name:        "Import Export OU " + handleSuffix,
		Description: "OU for export/import fresh-pack test",
		Parent:      nil,
	})
	suite.Require().NoError(err)

	yamlContent, err := suite.exportResources(exportRequest{
		OrganizationUnits: []string{ouID},
		Flows:             []string{flowID},
		Themes:            []string{themeID},
		Layouts:           []string{layoutID},
	})
	suite.Require().NoError(err)
	suite.Require().NotEmpty(yamlContent)

	err = suite.resetToFreshPack()
	suite.Require().NoError(err)

	suite.assertNotFound("/organization-units/" + ouID)
	suite.assertNotFound("/flows/" + flowID)
	suite.assertNotFound("/design/themes/" + themeID)
	suite.assertNotFound("/design/layouts/" + layoutID)

	importResp, err := suite.importResources(importRequest{
		Content: yamlContent,
		DryRun:  false,
		Options: importOptions{
			Upsert:          false,
			ContinueOnError: true,
			Target:          "runtime",
		},
		Vars: map[string]interface{}{},
	})
	suite.Require().NoError(err)
	suite.Require().NotNil(importResp)

	suite.Equal(4, importResp.Summary.TotalDocuments)
	suite.Equal(4, importResp.Summary.Imported)
	suite.Equal(0, importResp.Summary.Failed)
	suite.Len(importResp.Results, 4)

	resourceTypeToPath := map[string]string{
		"organization_unit": "/organization-units/%s",
		"flow":              "/flows/%s",
		"theme":             "/design/themes/%s",
		"layout":            "/design/layouts/%s",
	}

	seenTypes := map[string]bool{}
	for _, result := range importResp.Results {
		suite.Equal("success", result.Status)
		suite.Equal("create", result.Operation)
		suite.NotEmpty(result.ResourceType)
		suite.NotEmpty(result.ResourceID)

		pathPattern, ok := resourceTypeToPath[result.ResourceType]
		suite.True(ok, "unexpected resourceType in import response: %s", result.ResourceType)
		suite.assertFound(fmt.Sprintf(pathPattern, result.ResourceID))
		seenTypes[result.ResourceType] = true
	}

	suite.True(seenTypes["organization_unit"])
	suite.True(seenTypes["flow"])
	suite.True(seenTypes["theme"])
	suite.True(seenTypes["layout"])
}

func (suite *ImportExportFreshPackSuite) exportResources(reqBody exportRequest) (string, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal export request: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/export",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create export request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/x-yaml")

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send export request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read export response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("export request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var exportResp struct {
		Resources string       `json:"resources"`
		Files     []exportFile `json:"files"`
	}
	if err := json.Unmarshal(body, &exportResp); err != nil {
		return "", fmt.Errorf("failed to parse export response: %w", err)
	}

	if exportResp.Resources != "" {
		return exportResp.Resources, nil
	}

	if len(exportResp.Files) > 0 {
		combined := ""
		for i, file := range exportResp.Files {
			if i > 0 {
				combined += "\n---\n"
			}
			combined += "# File: " + file.FileName + "\n" + file.Content
		}
		return combined, nil
	}

	return "", fmt.Errorf("export response does not contain resources or files")
}

func (suite *ImportExportFreshPackSuite) importResources(reqBody importRequest) (*importResponse, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal import request: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/import",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create import request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send import request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read import response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("import request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var parsed importResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse import response: %w", err)
	}

	return &parsed, nil
}

func (suite *ImportExportFreshPackSuite) createTheme(request createThemeRequest) (string, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal theme request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/design/themes", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create theme request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send theme request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read theme response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("theme creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var created map[string]interface{}
	if err := json.Unmarshal(body, &created); err != nil {
		return "", fmt.Errorf("failed to parse theme response: %w", err)
	}

	themeID, _ := created["id"].(string)
	if themeID == "" {
		return "", fmt.Errorf("theme response does not contain id")
	}

	return themeID, nil
}

func (suite *ImportExportFreshPackSuite) createLayout(request createLayoutRequest) (string, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal layout request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/design/layouts", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create layout request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send layout request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read layout response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("layout creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var created map[string]interface{}
	if err := json.Unmarshal(body, &created); err != nil {
		return "", fmt.Errorf("failed to parse layout response: %w", err)
	}

	layoutID, _ := created["id"].(string)
	if layoutID == "" {
		return "", fmt.Errorf("layout response does not contain id")
	}

	return layoutID, nil
}

func (suite *ImportExportFreshPackSuite) assertFound(path string) {
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+path, nil)
	suite.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode, "expected resource at %s", path)
}

func (suite *ImportExportFreshPackSuite) assertNotFound(path string) {
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+path, nil)
	suite.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode, "expected resource to be absent at %s", path)
}

func (suite *ImportExportFreshPackSuite) resetToFreshPack() error {
	testutils.StopServer()

	if err := testutils.RunInitScript(testutils.GetZipFilePattern()); err != nil {
		return fmt.Errorf("failed to run init script: %w", err)
	}

	if err := testutils.RunSetupScript(); err != nil {
		return fmt.Errorf("failed to run setup script: %w", err)
	}

	if err := testutils.RestartServer(); err != nil {
		return fmt.Errorf("failed to restart server: %w", err)
	}

	if err := testutils.ObtainAdminAccessToken(); err != nil {
		return fmt.Errorf("failed to re-obtain admin token: %w", err)
	}

	return nil
}

// groupRoleExportRequest is a superset export request that includes groups and roles.
type groupRoleExportRequest struct {
	Groups          []string `json:"groups,omitempty"`
	Roles           []string `json:"roles,omitempty"`
	ResourceServers []string `json:"resourceServers,omitempty"`
}

// resourceItem is a minimal representation of a resource returned by the list endpoint.
type resourceItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Handle string `json:"handle"`
}

// GroupRoleResourceImportExportSuite tests export and import of groups, roles, and resource
// servers including the features introduced in commits 7c9ff551 and 97672e591:
//   - Group export (with members) via the group exporter
//   - Group import with members (AddGroupMembers called on create and update paths)
//   - Role import with assignments (Assignments forwarded on create; AddAssignments on update)
//   - Resource server import with nested resources (parent resolved via handle map)
type GroupRoleResourceImportExportSuite struct {
	suite.Suite
	ouID         string
	userTypeID   string
	userID       string
	handleSuffix string
}

func TestGroupRoleResourceImportExportSuite(t *testing.T) {
	suite.Run(t, new(GroupRoleResourceImportExportSuite))
}

func (s *GroupRoleResourceImportExportSuite) SetupSuite() {
	s.handleSuffix = fmt.Sprintf("%d", time.Now().UnixNano())

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "grr-ie-ou-" + s.handleSuffix,
		Name:        "GRR Import Export OU " + s.handleSuffix,
		Description: "OU for group/role/resource import-export tests",
		Parent:      nil,
	})
	s.Require().NoError(err)
	s.ouID = ouID

	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: "grr-person-" + s.handleSuffix,
		OUID: s.ouID,
		Schema: map[string]interface{}{
			"email": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
		},
	})
	s.Require().NoError(err)
	s.userTypeID = userTypeID

	userID, err := testutils.CreateUser(testutils.User{
		Type: "grr-person-" + s.handleSuffix,
		OUID: s.ouID,
		Attributes: json.RawMessage(`{
			"email": "grr-test-` + s.handleSuffix + `@example.com",
			"password": "TestPassword123!"
		}`),
	})
	s.Require().NoError(err)
	s.userID = userID
}

func (s *GroupRoleResourceImportExportSuite) TearDownSuite() {
	if s.userID != "" {
		_ = testutils.DeleteUser(s.userID)
	}
	if s.userTypeID != "" {
		_ = testutils.DeleteUserType(s.userTypeID)
	}
	if s.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(s.ouID)
	}
}

// TestGroupExportIncludesMembers creates a group with a member, exports it,
// and verifies the exported YAML contains the group fields and member data.
func (s *GroupRoleResourceImportExportSuite) TestGroupExportIncludesMembers() {
	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:        "Export Members Group " + s.handleSuffix,
		Description: "Group with a member for export test",
		OUID:        s.ouID,
	})
	s.Require().NoError(err)
	defer testutils.DeleteGroup(groupID)

	// Add a member to the group
	s.Require().NoError(s.addGroupMember(groupID, s.userID, "user"))

	// Export this group
	yamlContent, err := s.exportGroups([]string{groupID})
	s.Require().NoError(err)
	s.Require().NotEmpty(yamlContent)

	// Verify exported YAML contains expected group fields
	s.Assert().Contains(yamlContent, "name: Export Members Group "+s.handleSuffix)
	s.Assert().Contains(yamlContent, "ou_id: "+s.ouID)
	s.Assert().Contains(yamlContent, "members:")
	s.Assert().Contains(yamlContent, "id: "+s.userID)
	s.Assert().Contains(yamlContent, "type: user")
}

// TestGroupExportNoMembers verifies that a group without members is exported without
// a members field (omitempty).
func (s *GroupRoleResourceImportExportSuite) TestGroupExportNoMembers() {
	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:        "Export No Members Group " + s.handleSuffix,
		Description: "Group without members for export test",
		OUID:        s.ouID,
	})
	s.Require().NoError(err)
	defer testutils.DeleteGroup(groupID)

	yamlContent, err := s.exportGroups([]string{groupID})
	s.Require().NoError(err)
	s.Require().NotEmpty(yamlContent)

	s.Assert().Contains(yamlContent, "name: Export No Members Group "+s.handleSuffix)
	// Members should be omitted when empty
	s.Assert().NotContains(yamlContent, "members:")
}

// TestGroupExportMultipleGroups verifies that multiple groups are all included in a
// single export response.
func (s *GroupRoleResourceImportExportSuite) TestGroupExportMultipleGroups() {
	groupID1, err := testutils.CreateGroup(testutils.Group{
		Name: "Multi Group 1 " + s.handleSuffix,
		OUID: s.ouID,
	})
	s.Require().NoError(err)
	defer testutils.DeleteGroup(groupID1)

	groupID2, err := testutils.CreateGroup(testutils.Group{
		Name: "Multi Group 2 " + s.handleSuffix,
		OUID: s.ouID,
	})
	s.Require().NoError(err)
	defer testutils.DeleteGroup(groupID2)

	yamlContent, err := s.exportGroups([]string{groupID1, groupID2})
	s.Require().NoError(err)
	s.Require().NotEmpty(yamlContent)

	s.Assert().Contains(yamlContent, "name: Multi Group 1 "+s.handleSuffix)
	s.Assert().Contains(yamlContent, "name: Multi Group 2 "+s.handleSuffix)
}

// TestImportGroupWithMembers imports a group YAML containing members and verifies
// the group is created and members are attached.
func (s *GroupRoleResourceImportExportSuite) TestImportGroupWithMembers() {
	groupName := "Import Members Group " + s.handleSuffix

	yamlContent := fmt.Sprintf(`name: %s
description: Imported group with a member
ou_id: %s
members:
  - id: %s
    type: user
`, groupName, s.ouID, s.userID)

	resp, err := s.importResources(importRequest{
		Content: yamlContent,
		Options: importOptions{
			Upsert:          false,
			ContinueOnError: false,
			Target:          "runtime",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().Equal(1, resp.Summary.TotalDocuments)
	s.Require().Equal(1, resp.Summary.Imported)
	s.Require().Equal(0, resp.Summary.Failed)

	result := resp.Results[0]
	s.Equal("success", result.Status)
	s.Equal("create", result.Operation)
	s.NotEmpty(result.ResourceID)

	defer testutils.DeleteGroup(result.ResourceID)

	// Verify members were added
	members, err := testutils.GetGroupMembers(result.ResourceID)
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.Equal(s.userID, members[0].ID)
	s.Equal("user", members[0].Type)
}

// TestImportGroupWithoutMembers imports a group YAML without members and verifies
// the group is created successfully with no members.
func (s *GroupRoleResourceImportExportSuite) TestImportGroupWithoutMembers() {
	groupName := "Import No Members Group " + s.handleSuffix

	yamlContent := fmt.Sprintf(`name: %s
description: Imported group without members
ou_id: %s
`, groupName, s.ouID)

	resp, err := s.importResources(importRequest{
		Content: yamlContent,
		Options: importOptions{
			Upsert:          false,
			ContinueOnError: false,
			Target:          "runtime",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(1, resp.Summary.Imported)
	s.Equal(0, resp.Summary.Failed)

	result := resp.Results[0]
	s.Equal("success", result.Status)
	s.Equal("create", result.Operation)

	defer testutils.DeleteGroup(result.ResourceID)

	members, err := testutils.GetGroupMembers(result.ResourceID)
	s.Require().NoError(err)
	s.Empty(members)
}

// TestImportGroupUpsertUpdateWithMembers imports a group with a known ID twice (upsert).
// The first import creates it; the second import updates it and should attach members.
func (s *GroupRoleResourceImportExportSuite) TestImportGroupUpsertUpdateWithMembers() {
	groupID, err := testutils.CreateGroup(testutils.Group{
		Name: "Upsert Members Group " + s.handleSuffix,
		OUID: s.ouID,
	})
	s.Require().NoError(err)
	defer testutils.DeleteGroup(groupID)

	// Second import (update path) with the same ID and members
	yamlContent := fmt.Sprintf(`id: %s
name: Upsert Members Group %s
description: Updated via import
ou_id: %s
members:
  - id: %s
    type: user
`, groupID, s.handleSuffix, s.ouID, s.userID)

	resp, err := s.importResources(importRequest{
		Content: yamlContent,
		Options: importOptions{
			Upsert:          true,
			ContinueOnError: false,
			Target:          "runtime",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(1, resp.Summary.Imported)
	s.Equal(0, resp.Summary.Failed)

	result := resp.Results[0]
	s.Equal("success", result.Status)
	s.Equal("update", result.Operation)
	s.Equal(groupID, result.ResourceID)

	// Verify members were added on the update path
	members, err := testutils.GetGroupMembers(groupID)
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.Equal(s.userID, members[0].ID)
}

// TestImportRoleWithAssignmentsCreate imports a new role with group assignments and
// verifies assignments are attached after creation.
func (s *GroupRoleResourceImportExportSuite) TestImportRoleWithAssignmentsCreate() {
	// Create a group to assign
	groupID, err := testutils.CreateGroup(testutils.Group{
		Name: "Role Assignment Group " + s.handleSuffix,
		OUID: s.ouID,
	})
	s.Require().NoError(err)
	defer testutils.DeleteGroup(groupID)

	roleName := "Import Role With Assignments " + s.handleSuffix

	yamlContent := fmt.Sprintf(`name: %s
description: Imported role with group assignment
ou_id: %s
permissions: []
assignments:
  - id: %s
    type: group
`, roleName, s.ouID, groupID)

	resp, err := s.importResources(importRequest{
		Content: yamlContent,
		Options: importOptions{
			Upsert:          false,
			ContinueOnError: false,
			Target:          "runtime",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(1, resp.Summary.Imported)
	s.Equal(0, resp.Summary.Failed)

	result := resp.Results[0]
	s.Equal("success", result.Status)
	s.Equal("create", result.Operation)
	s.NotEmpty(result.ResourceID)

	defer testutils.DeleteRole(result.ResourceID)

	// Verify the group assignment was applied
	assignments, err := testutils.GetRoleAssignments(result.ResourceID)
	s.Require().NoError(err)
	found := false
	for _, a := range assignments {
		if a.ID == groupID && a.Type == "group" {
			found = true
			break
		}
	}
	s.True(found, "expected group %s to be assigned to the imported role", groupID)
}

// TestImportRoleWithAssignmentsUpdate imports a role that already exists (upsert update path)
// with assignments and verifies AddAssignments is called.
func (s *GroupRoleResourceImportExportSuite) TestImportRoleWithAssignmentsUpdate() {
	// Create the role and group first
	groupID, err := testutils.CreateGroup(testutils.Group{
		Name: "Role Upsert Group " + s.handleSuffix,
		OUID: s.ouID,
	})
	s.Require().NoError(err)
	defer testutils.DeleteGroup(groupID)

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Upsert Role " + s.handleSuffix,
		OUID: s.ouID,
	})
	s.Require().NoError(err)
	defer testutils.DeleteRole(roleID)

	yamlContent := fmt.Sprintf(`id: %s
name: Upsert Role %s
description: Updated via import with assignment
ou_id: %s
permissions: []
assignments:
  - id: %s
    type: group
`, roleID, s.handleSuffix, s.ouID, groupID)

	resp, err := s.importResources(importRequest{
		Content: yamlContent,
		Options: importOptions{
			Upsert:          true,
			ContinueOnError: false,
			Target:          "runtime",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(1, resp.Summary.Imported)
	s.Equal(0, resp.Summary.Failed)

	result := resp.Results[0]
	s.Equal("success", result.Status)
	s.Equal("update", result.Operation)
	s.Equal(roleID, result.ResourceID)

	// Verify the assignment was added on the update path
	assignments, err := testutils.GetRoleAssignments(roleID)
	s.Require().NoError(err)
	found := false
	for _, a := range assignments {
		if a.ID == groupID && a.Type == "group" {
			found = true
			break
		}
	}
	s.True(found, "expected group assignment to be present after upsert-update import")
}

// TestImportRoleNoAssignments imports a role without assignments and verifies it succeeds
// with an empty assignment list.
func (s *GroupRoleResourceImportExportSuite) TestImportRoleNoAssignments() {
	roleName := "Import Role No Assignments " + s.handleSuffix

	yamlContent := fmt.Sprintf(`name: %s
description: Imported role without assignments
ou_id: %s
permissions: []
`, roleName, s.ouID)

	resp, err := s.importResources(importRequest{
		Content: yamlContent,
		Options: importOptions{
			Upsert:          false,
			ContinueOnError: false,
			Target:          "runtime",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(1, resp.Summary.Imported)
	s.Equal(0, resp.Summary.Failed)

	result := resp.Results[0]
	s.Equal("success", result.Status)
	s.Equal("create", result.Operation)

	defer testutils.DeleteRole(result.ResourceID)

	assignments, err := testutils.GetRoleAssignments(result.ResourceID)
	s.Require().NoError(err)
	s.Empty(assignments)
}

// TestImportResourceServerWithNestedResources imports a resource server that has a
// parent-child resource hierarchy. Verifies the child resource is created with the
// correct parent (resolved via the handle map) and actions are attached.
func (s *GroupRoleResourceImportExportSuite) TestImportResourceServerWithNestedResources() {
	handle := "nested-rs-" + s.handleSuffix

	yamlContent := fmt.Sprintf(`# resource_type: resource_server
name: Nested Resource Server %s
description: Resource server with nested resources
handle: %s
ou_id: %s
delimiter: ":"
resources:
  - name: Parent Resource
    handle: parent-%s
    description: A top-level resource
    actions:
      - name: Read Parent
        handle: read-parent
  - name: Child Resource
    handle: child-%s
    parent: parent-%s
    description: A child of the parent resource
    actions:
      - name: Read Child
        handle: read-child
`, s.handleSuffix, handle, s.ouID,
		s.handleSuffix, s.handleSuffix, s.handleSuffix)

	resp, err := s.importResources(importRequest{
		Content: yamlContent,
		Options: importOptions{
			Upsert:          false,
			ContinueOnError: false,
			Target:          "runtime",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(1, resp.Summary.TotalDocuments)
	s.Equal(1, resp.Summary.Imported)
	s.Equal(0, resp.Summary.Failed)

	result := resp.Results[0]
	s.Equal("success", result.Status)
	s.Equal("create", result.Operation)
	s.NotEmpty(result.ResourceID)

	defer testutils.DeleteResourceServer(result.ResourceID)

	rsID := result.ResourceID

	// Retrieve resources and verify parent-child relationship
	topLevelResources, err := s.getResourcesForServer(rsID, "")
	s.Require().NoError(err)
	s.Require().Len(topLevelResources, 1, "expected one top-level resource")

	parentResource := topLevelResources[0]
	s.Equal("Parent Resource", parentResource.Name)
	s.Equal("parent-"+s.handleSuffix, parentResource.Handle)

	childResources, err := s.getResourcesForServer(rsID, parentResource.ID)
	s.Require().NoError(err)
	s.Require().Len(childResources, 1, "expected one child resource under parent")

	childResource := childResources[0]
	s.Equal("Child Resource", childResource.Name)
	s.Equal("child-"+s.handleSuffix, childResource.Handle)
}

// TestImportResourceServerUpsertNestedResources re-imports a resource server that
// already exists (upsert path). The nested resources also already exist so conflict
// recovery must use the scoped parent ID when looking up the existing child resource.
func (s *GroupRoleResourceImportExportSuite) TestImportResourceServerUpsertNestedResources() {
	handle := "upsert-rs-" + s.handleSuffix

	buildYAML := func(id string) string {
		idLine := ""
		if id != "" {
			idLine = "id: " + id + "\n"
		}
		return fmt.Sprintf(`# resource_type: resource_server
%sname: Upsert Resource Server %s
description: Resource server for upsert test
handle: %s
ou_id: %s
delimiter: ":"
resources:
  - name: Upsert Parent
    handle: upsert-parent-%s
    actions:
      - name: Read
        handle: read
  - name: Upsert Child
    handle: upsert-child-%s
    parent: upsert-parent-%s
    actions:
      - name: Write
        handle: write
`, idLine, s.handleSuffix, handle, s.ouID,
			s.handleSuffix, s.handleSuffix, s.handleSuffix)
	}

	// First import — creates the resource server and resources (no id in YAML).
	resp, err := s.importResources(importRequest{
		Content: buildYAML(""),
		Options: importOptions{
			Upsert:          false,
			ContinueOnError: false,
			Target:          "runtime",
		},
	})
	s.Require().NoError(err)
	s.Equal(1, resp.Summary.Imported)
	s.Equal(0, resp.Summary.Failed)
	rsID := resp.Results[0].ResourceID
	defer testutils.DeleteResourceServer(rsID)

	// Second import (upsert) — include the ID so the upsert path triggers an update.
	resp2, err := s.importResources(importRequest{
		Content: buildYAML(rsID),
		Options: importOptions{
			Upsert:          true,
			ContinueOnError: false,
			Target:          "runtime",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp2)
	s.Equal(1, resp2.Summary.Imported)
	s.Equal(0, resp2.Summary.Failed)

	result := resp2.Results[0]
	s.Equal("success", result.Status)

	// Parent and child resources must still exist under the server
	topLevel, err := s.getResourcesForServer(rsID, "")
	s.Require().NoError(err)
	s.Require().Len(topLevel, 1)
	s.Equal("Upsert Parent", topLevel[0].Name)

	children, err := s.getResourcesForServer(rsID, topLevel[0].ID)
	s.Require().NoError(err)
	s.Require().Len(children, 1)
	s.Equal("Upsert Child", children[0].Name)
}

func (s *GroupRoleResourceImportExportSuite) importResources(reqBody importRequest) (*importResponse, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal import request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/import", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create import request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send import request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read import response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("import request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var parsed importResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse import response: %w", err)
	}

	return &parsed, nil
}

func (s *GroupRoleResourceImportExportSuite) exportGroups(groupIDs []string) (string, error) {
	reqBody := groupRoleExportRequest{
		Groups: groupIDs,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal export request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/export", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create export request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send export request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read export response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("export request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var exportResp struct {
		Resources string `json:"resources"`
	}
	if err := json.Unmarshal(body, &exportResp); err != nil {
		return "", fmt.Errorf("failed to parse export response: %w", err)
	}

	return exportResp.Resources, nil
}

// addGroupMember adds a single member to a group via the members API.
func (s *GroupRoleResourceImportExportSuite) addGroupMember(groupID, memberID, memberType string) error {
	reqBody := map[string]interface{}{
		"members": []map[string]interface{}{
			{"id": memberID, "type": memberType},
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal add members request: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/groups/"+groupID+"/members/add",
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("failed to create add members request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to add group member: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("add group member failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// getResourcesForServer lists resources under a resource server, optionally scoped to a parent.
func (s *GroupRoleResourceImportExportSuite) getResourcesForServer(serverID, parentID string) ([]resourceItem, error) {
	url := testutils.TestServerURL + "/resource-servers/" + serverID + "/resources"
	if parentID != "" {
		url += "?parentId=" + parentID
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource list request: %w", err)
	}

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource list response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list resources failed with status %d: %s", resp.StatusCode, string(body))
	}

	var listResp struct {
		Resources []resourceItem `json:"resources"`
	}
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse resource list response: %w", err)
	}

	return listResp.Resources, nil
}
