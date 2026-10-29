// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// createChildOU creates an organization unit under parentID for a test and removes it when the test ends. The unit is
// created after its parent, so it is also removed before it.
func (s *ImportExportConnectionOUSuite) createChildOU(parentID, prefix string) string {
	s.T().Helper()
	handle := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: handle, Name: handle, Parent: &parentID,
	})
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(ouID) })
	return ouID
}

// sharingCall calls a sharing endpoint of a connection and returns the status and the decoded body.
func (s *ImportExportConnectionOUSuite) sharingCall(
	method, path string, body interface{},
) (int, map[string]interface{}) {
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

// sharingPolicies returns the sharing policies of a connection.
func (s *ImportExportConnectionOUSuite) sharingPolicies(vendor, id string) []interface{} {
	s.T().Helper()
	status, listed := s.sharingCall(http.MethodGet, "/connections/"+vendor+"/"+id+"/sharing-policies", nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", listed)
	policies, _ := listed["policies"].([]interface{})
	return policies
}

// sharedWithDocument returns an import document for the kind, owned by ouID, that shares the connection with childID.
func sharedWithDocument(kind ownedConnectionKind, id, name, ouID, childID string) string {
	return kind.document(id, name, "ouId: "+ouID+"\n") +
		"sharingPolicies:\n  - childUnits:\n      mode: selected\n      units:\n" +
		"        - ouId: " + childID + "\n          reach: unit\n"
}

// An export carries the sharing policies recorded for a connection, and replaying the exported document records the
// same sharing rather than producing a connection nobody else can use.
func (s *ImportExportConnectionOUSuite) TestExportImportRoundTripCarriesSharingPolicies() {
	for _, kind := range ownedConnectionKinds() {
		s.Run(kind.vendor, func() {
			ouID, _ := s.createOU("conn-rt-share-" + kind.vendor)
			childID := s.createChildOU(ouID, "conn-rt-share-child-"+kind.vendor)
			name := fmt.Sprintf("Share Round Trip %s %d", kind.vendor, time.Now().UnixNano())
			id, err := kind.create(name, ouID)
			s.Require().NoError(err)
			s.deleteAfterTest(kind.vendor, id)
			s.T().Cleanup(func() { _ = kind.delete(id) })

			status, created := s.sharingCall(http.MethodPost, "/connections/"+kind.vendor+"/"+id+"/sharing-policies",
				map[string]interface{}{"childUnits": map[string]interface{}{
					"mode": "selected",
					"units": []map[string]interface{}{
						{"ouId": childID, "reach": "unit"},
					},
				}})
			s.Require().Equal(http.StatusCreated, status, "body: %v", created)

			exported := s.exportConnection(id)
			document := s.parseExportedDocument(exported, id)
			s.Require().Contains(document, "sharingPolicies", "the exported document carries the sharing")
			s.Contains(exported, childID, "including the organization unit it is shared with")

			// Deleting the connection takes its policies with it, so the replay has to bring them back.
			s.Require().NoError(kind.delete(id))
			s.Equal(http.StatusNotFound, s.connectionStatus(kind.vendor, id))

			response := s.importContent(exported, false, exportedVariables(exported))
			s.Require().Len(response.Results, 1)
			s.Equal("success", response.Results[0].Status, "message: %s", response.Results[0].Message)
			s.Equal(id, response.Results[0].ResourceID)

			policies := s.sharingPolicies(kind.vendor, id)
			s.Require().Len(policies, 1, "the replay recorded the exported sharing")
			childUnits, _ := policies[0].(map[string]interface{})["childUnits"].(map[string]interface{})
			s.Require().NotNil(childUnits)
			s.Equal("selected", childUnits["mode"])
		})
	}
}

// An import document records the sharing it declares against the owner it resolved.
func (s *ImportExportConnectionOUSuite) TestImportRecordsDeclaredSharingPolicies() {
	for _, kind := range ownedConnectionKinds() {
		s.Run(kind.vendor, func() {
			ouID, _ := s.createOU("conn-import-share-" + kind.vendor)
			childID := s.createChildOU(ouID, "conn-import-share-child-"+kind.vendor)
			id := newImportUUID(s)
			name := fmt.Sprintf("Share Import %s %d", kind.vendor, time.Now().UnixNano())

			response := s.importContent(sharedWithDocument(kind, id, name, ouID, childID), false, nil)
			s.Require().Len(response.Results, 1)
			s.Equal("success", response.Results[0].Status, "message: %s", response.Results[0].Message)
			s.deleteAfterTest(kind.vendor, id)

			s.Len(s.sharingPolicies(kind.vendor, id), 1)
			status, overlay := s.sharingCall(http.MethodGet,
				"/connections/"+kind.vendor+"/"+id+"/overlay-rules?ouId="+childID, nil)
			s.Require().Equal(http.StatusOK, status, "body: %v", overlay)
			s.Equal(true, overlay["visible"], "the unit the document shares with can use the connection")
		})
	}
}

// A document whose sharing the framework refuses fails as a whole: the connection is not left behind, where it would
// hold the name a retry needs.
func (s *ImportExportConnectionOUSuite) TestImportFailsAndRemovesConnectionWhenSharingIsRefused() {
	for _, kind := range ownedConnectionKinds() {
		s.Run(kind.vendor, func() {
			ouID, _ := s.createOU("conn-refused-share-" + kind.vendor)
			stranger, _ := s.createOU("conn-refused-stranger-" + kind.vendor)
			id := newImportUUID(s)
			name := fmt.Sprintf("Share Refused %s %d", kind.vendor, time.Now().UnixNano())

			// The stranger is not a child of the owner, so the policy is not one the owner may write.
			response := s.importContent(sharedWithDocument(kind, id, name, ouID, stranger), false, nil)
			s.deleteAfterTest(kind.vendor, id)

			s.Require().Len(response.Results, 1)
			s.Equal("failed", response.Results[0].Status)
			s.Contains(response.Results[0].Message, "sharing policy 1")
			s.Equal(http.StatusNotFound, s.connectionStatus(kind.vendor, id),
				"the connection of a refused document is not kept")
		})
	}
}

// A dry run writes nothing, so it records no sharing and leaves no connection.
func (s *ImportExportConnectionOUSuite) TestImportDryRunRecordsNoSharing() {
	for _, kind := range ownedConnectionKinds() {
		s.Run(kind.vendor, func() {
			ouID, _ := s.createOU("conn-dry-share-" + kind.vendor)
			childID := s.createChildOU(ouID, "conn-dry-share-child-"+kind.vendor)
			id := newImportUUID(s)
			name := fmt.Sprintf("Share Dry Run %s %d", kind.vendor, time.Now().UnixNano())

			response := s.importContent(sharedWithDocument(kind, id, name, ouID, childID), true, nil)
			s.deleteAfterTest(kind.vendor, id)

			s.Require().Len(response.Results, 1)
			s.Equal("success", response.Results[0].Status, "message: %s", response.Results[0].Message)
			s.Equal(http.StatusNotFound, s.connectionStatus(kind.vendor, id), "a dry run must not create the connection")
		})
	}
}
