// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ImportExportResourceServerSharingSuite covers the sharing policies a resource server document carries through an
// export and an import.
type ImportExportResourceServerSharingSuite struct {
	suite.Suite
}

func TestImportExportResourceServerSharingSuite(t *testing.T) {
	suite.Run(t, new(ImportExportResourceServerSharingSuite))
}

// createOU creates an organization unit, beneath parentID when one is given, and removes it when the test ends.
func (s *ImportExportResourceServerSharingSuite) createOU(prefix string, parentID *string) string {
	s.T().Helper()
	handle := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: handle, Name: handle, Parent: parentID,
	})
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(ouID) })
	return ouID
}

// call calls the resource server API and returns the status and the decoded body.
func (s *ImportExportResourceServerSharingSuite) call(method, path string, body interface{}) (
	int, map[string]interface{}) {
	s.T().Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		s.Require().NoError(err)
		reader = strings.NewReader(string(raw))
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	s.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	s.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	decoded := map[string]interface{}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	return resp.StatusCode, decoded
}

func (s *ImportExportResourceServerSharingSuite) policies(id string) []interface{} {
	s.T().Helper()
	status, listed := s.call(http.MethodGet, "/resource-servers/"+id+"/sharing-policies", nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", listed)
	policies, _ := listed["policies"].([]interface{})
	return policies
}

func (s *ImportExportResourceServerSharingSuite) visible(id, ouID string) bool {
	s.T().Helper()
	status, _ := s.call(http.MethodGet, "/resource-servers/"+id+"/overlay-rules?ouId="+ouID, nil)
	return status == http.StatusOK
}

func (s *ImportExportResourceServerSharingSuite) importContent(
	content string, dryRun bool, variables map[string]interface{},
) *testutils.ImportResponse {
	s.T().Helper()
	response, err := postImport(testutils.ImportRequest{
		Content: content,
		DryRun:  dryRun,
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
		Vars:    variables,
	})
	s.Require().NoError(err)
	return response
}

// removeAfterTest deletes an imported resource server with its resources when the test ends.
func (s *ImportExportResourceServerSharingSuite) removeAfterTest(id string) {
	s.T().Cleanup(func() { _ = testutils.DeleteResourceServerWithChildren(id) })
}

// sharedServerDocument is an import document for a resource server owned by ouID that shares it with childID.
func sharedServerDocument(id, ouID, childID string) string {
	return strings.Join([]string{
		"resource_type: resource_server",
		"id: " + id,
		"name: Shared Import API " + id[:8],
		"identifier: https://api.example.com/shared-import/" + id,
		"ouId: " + ouID,
		"sharingPolicies:",
		"  - childUnits:",
		"      mode: selected",
		"      units:",
		"        - ouId: " + childID,
		"          reach: unit",
		"resources:",
		"  - name: Orders",
		"    handle: orders",
		"    actions:",
		"      - name: Read",
		"        handle: read",
		"",
	}, "\n")
}

// An import document records the sharing it carries, after the resource server and its resources are written.
func (s *ImportExportResourceServerSharingSuite) TestImportRecordsSharingPolicies() {
	ownerID := s.createOU("rs-import-share", nil)
	childID := s.createOU("rs-import-share-child", &ownerID)
	otherChildID := s.createOU("rs-import-share-other", &ownerID)
	id := newImportUUID(s)

	response := s.importContent(sharedServerDocument(id, ownerID, childID), false, nil)
	s.removeAfterTest(id)

	s.Require().Len(response.Results, 1)
	s.Equal("success", response.Results[0].Status, "message: %s", response.Results[0].Message)
	s.Len(s.policies(id), 1)
	s.True(s.visible(id, childID), "the unit the document shares with reaches the resource server")
	s.False(s.visible(id, otherChildID), "a unit the document does not name is not reached")
}

// An export carries the sharing recorded for a resource server, and replaying it after the resource server is deleted
// records the same sharing again.
func (s *ImportExportResourceServerSharingSuite) TestExportImportRoundTripCarriesSharingPolicies() {
	ownerID := s.createOU("rs-rt-share", nil)
	childID := s.createOU("rs-rt-share-child", &ownerID)
	id := newImportUUID(s)
	response := s.importContent(sharedServerDocument(id, ownerID, childID), false, nil)
	s.removeAfterTest(id)
	s.Require().Equal("success", response.Results[0].Status, "message: %s", response.Results[0].Message)

	exported, err := postExport(exportRequest{ResourceServers: []string{id}})
	s.Require().NoError(err)
	s.Require().Contains(exported, "sharingPolicies:", "the exported document carries the sharing")
	s.Contains(exported, childID, "including the organization unit it is shared with")

	// Deleting the resource server takes its policies with it, so the replay has to bring them back.
	s.Require().NoError(testutils.DeleteResourceServerWithChildren(id))
	status, _ := s.call(http.MethodGet, "/resource-servers/"+id, nil)
	s.Require().Equal(http.StatusNotFound, status)

	response = s.importContent(exported, false, exportedVariables(exported))
	s.Require().Len(response.Results, 1)
	s.Equal("success", response.Results[0].Status, "message: %s", response.Results[0].Message)
	s.Equal(id, response.Results[0].ResourceID)

	policies := s.policies(id)
	s.Require().Len(policies, 1, "the replay recorded the exported sharing")
	childUnits, _ := policies[0].(map[string]interface{})["childUnits"].(map[string]interface{})
	s.Require().NotNil(childUnits)
	s.Equal("selected", childUnits["mode"])
	s.True(s.visible(id, childID))
}

// A resource server deleted and created again with the same id does not come back shared.
func (s *ImportExportResourceServerSharingSuite) TestRecreatedResourceServerIsNotShared() {
	ownerID := s.createOU("rs-recreate-share", nil)
	childID := s.createOU("rs-recreate-share-child", &ownerID)
	id := newImportUUID(s)
	response := s.importContent(sharedServerDocument(id, ownerID, childID), false, nil)
	s.removeAfterTest(id)
	s.Require().Equal("success", response.Results[0].Status, "message: %s", response.Results[0].Message)

	s.Require().NoError(testutils.DeleteResourceServerWithChildren(id))
	unshared := strings.SplitN(sharedServerDocument(id, ownerID, childID), "sharingPolicies:", 2)[0] +
		"resources: []\n"
	response = s.importContent(unshared, false, nil)
	s.Require().Equal("success", response.Results[0].Status, "message: %s", response.Results[0].Message)

	s.Empty(s.policies(id))
	s.False(s.visible(id, childID))
}

// A document whose sharing is refused fails as a whole, and the resource server it created is removed.
func (s *ImportExportResourceServerSharingSuite) TestImportFailsAndRemovesResourceServerWhenSharingIsRefused() {
	ownerID := s.createOU("rs-refused-share", nil)
	stranger := s.createOU("rs-refused-stranger", nil)
	id := newImportUUID(s)

	// The stranger is not a child of the owner, so the policy is not one the owner may write.
	response := s.importContent(sharedServerDocument(id, ownerID, stranger), false, nil)
	s.removeAfterTest(id)

	s.Require().Len(response.Results, 1)
	s.Equal("failed", response.Results[0].Status)
	s.Contains(response.Results[0].Message, "sharing policy 1")
	status, _ := s.call(http.MethodGet, "/resource-servers/"+id, nil)
	s.Equal(http.StatusNotFound, status, "the resource server of a refused document is not kept")
}

// A policy cannot share part of a resource server yet, so an import naming the permissions it shares fails the item.
func (s *ImportExportResourceServerSharingSuite) TestImportRefusesAPermissionsValue() {
	ownerID := s.createOU("rs-partial-share", nil)
	id := newImportUUID(s)
	document := strings.Join([]string{
		"resource_type: resource_server",
		"id: " + id,
		"name: Partial Import API " + id[:8],
		"identifier: https://api.example.com/partial-import/" + id,
		"ouId: " + ownerID,
		"sharingPolicies:",
		"  - childUnits:",
		"      mode: all",
		"      overlayRules:",
		"        permissions:",
		"          editable: false",
		"          value: [orders]",
		"resources: []",
		"",
	}, "\n")

	response := s.importContent(document, false, nil)
	s.removeAfterTest(id)

	s.Require().Len(response.Results, 1)
	s.Equal("failed", response.Results[0].Status)
	s.Contains(response.Results[0].Message, "sharing policy 1")
	status, _ := s.call(http.MethodGet, "/resource-servers/"+id, nil)
	s.Equal(http.StatusNotFound, status, "the resource server of a refused document is not kept")
}

// A dry run writes nothing, so it records no sharing and leaves no resource server.
func (s *ImportExportResourceServerSharingSuite) TestImportDryRunRecordsNoSharing() {
	ownerID := s.createOU("rs-dry-share", nil)
	childID := s.createOU("rs-dry-share-child", &ownerID)
	id := newImportUUID(s)

	response := s.importContent(sharedServerDocument(id, ownerID, childID), true, nil)
	s.removeAfterTest(id)

	s.Require().Len(response.Results, 1)
	s.Equal("success", response.Results[0].Status, "message: %s", response.Results[0].Message)
	status, _ := s.call(http.MethodGet, "/resource-servers/"+id, nil)
	s.Equal(http.StatusNotFound, status, "a dry run must not create the resource server")
}

// A deployment-wide policy, which the API refuses to create, can come from an import.
func (s *ImportExportResourceServerSharingSuite) TestImportRecordsADeploymentWidePolicy() {
	ownerID := s.createOU("rs-wide-share", nil)
	otherRoot := s.createOU("rs-wide-share-other", nil)
	id := newImportUUID(s)
	document := strings.Join([]string{
		"resource_type: resource_server",
		"id: " + id,
		"name: Wide Import API " + id[:8],
		"identifier: https://api.example.com/wide-import/" + id,
		"ouId: " + ownerID,
		"sharingPolicies:",
		"  - allUnits: {}",
		"resources: []",
		"",
	}, "\n")

	response := s.importContent(document, false, nil)
	s.removeAfterTest(id)

	s.Require().Len(response.Results, 1)
	s.Equal("success", response.Results[0].Status, "message: %s", response.Results[0].Message)
	s.True(s.visible(id, otherRoot), "every organization unit is reached, another tree's root included")
}
