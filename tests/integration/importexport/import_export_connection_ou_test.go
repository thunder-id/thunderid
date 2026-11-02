// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"gopkg.in/yaml.v3"
)

// exportedPlaceholder matches the {{.VAR}} tokens the exporter writes in place of a secret.
var exportedPlaceholder = regexp.MustCompile(`{{\s*\.([A-Za-z0-9_]+)\s*}}`)

// ownedConnectionKind describes one kind of connection that import and export carry: how to create and delete one
// through the API, how to write its import document, and the error codes an owner problem is reported with.
type ownedConnectionKind struct {
	vendor string
	// create creates a connection with the given name, owned by ouID, and returns its id.
	create func(name, ouID string) (string, error)
	// delete deletes a connection that create returned.
	delete func(id string) error
	// document returns an import document with the given id and name. owner holds the ouId and ouHandle lines.
	document func(id, name, owner string) string
	// requiredCode is the code of the failure when the document names no owner.
	requiredCode string
	// unknownCode is the code of the failure when the document names an organization unit that does not exist.
	unknownCode string
}

// ownedConnectionKinds returns an identity provider, a notification sender, an AuthZEN PDP connection and an HTTP
// connection, one for each service that holds connections and that import and export carry.
func ownedConnectionKinds() []ownedConnectionKind {
	return []ownedConnectionKind{
		{
			vendor: "google",
			create: func(name, ouID string) (string, error) {
				return testutils.CreateIDP(testutils.IDP{
					Name: name, Type: "GOOGLE", OUID: ouID,
					Properties: []testutils.IDPProperty{
						{Name: "client_id", Value: "ou-roundtrip-client"},
						{Name: "client_secret", Value: "ou-roundtrip-secret", IsSecret: true},
						{Name: "redirect_uri", Value: "https://localhost:3000/callback"},
						{Name: "scopes", Value: "openid,email,profile"},
					},
				})
			},
			delete: testutils.DeleteIDP,
			document: func(id, name, owner string) string {
				return fmt.Sprintf(`resource_type: connection
id: %s
type: google
name: %s
%sclientId: ou-import-client
clientSecret: ou-import-secret
redirectUri: https://localhost:3000/callback
`, id, name, owner)
			},
			requiredCode: "IDP-1016",
			unknownCode:  "IDP-1014",
		},
		{
			vendor: "twilio",
			create: func(name, ouID string) (string, error) {
				return testutils.CreateNotificationSender(testutils.NotificationSender{
					Name: name, Provider: "twilio", OUID: ouID,
					Properties: []testutils.SenderProperty{
						{Name: "account_sid", Value: "AC00000000000000000000000000000000"},
						{Name: "auth_token", Value: "ou-roundtrip-token", IsSecret: true},
						{Name: "sender_id", Value: "+15005550006"},
					},
				})
			},
			delete: testutils.DeleteNotificationSender,
			document: func(id, name, owner string) string {
				return fmt.Sprintf(`resource_type: connection
id: %s
type: twilio
name: %s
%saccountSid: AC00000000000000000000000000000000
authToken: ou-import-token
senderId: "+15005550006"
`, id, name, owner)
			},
			requiredCode: "MNS-1020",
			unknownCode:  "MNS-1018",
		},
		{
			vendor: "authzen-pdp",
			create: func(name, ouID string) (string, error) {
				return testutils.CreateAuthZENPDPConnection(name, ouID, "https://pdp.example.com/access/v1/evaluation")
			},
			delete: testutils.DeleteAuthZENPDPConnection,
			document: func(id, name, owner string) string {
				return fmt.Sprintf(`resource_type: connection
id: %s
type: authzen-pdp
name: %s
%sendpoint: https://pdp.example.com/access/v1/evaluation
timeoutMs: 1000
retryCount: 1
`, id, name, owner)
			},
			requiredCode: "CON-1025",
			unknownCode:  "CON-1023",
		},
		{
			vendor: "http",
			create: func(name, ouID string) (string, error) {
				return testutils.CreateHTTPConnection(testutils.HTTPConnection{
					Name: name, OUID: ouID, URL: "https://ext.example.com/evaluate"})
			},
			delete: testutils.DeleteHTTPConnection,
			document: func(id, name, owner string) string {
				return fmt.Sprintf(`resource_type: connection
id: %s
type: http
name: %s
%surl: https://ext.example.com/evaluate
`, id, name, owner)
			},
			requiredCode: "CON-1030",
			unknownCode:  "CON-1028",
		},
	}
}

// ImportExportConnectionOUSuite covers the owning organization unit of a connection through export and import: the
// ouId an exported document carries, and the ouHandle an imported document may name instead.
//
// Every test creates the organization units it needs and removes them after the connections they own, since an
// organization unit that still owns a connection cannot be deleted.
type ImportExportConnectionOUSuite struct {
	suite.Suite
}

func TestImportExportConnectionOUSuite(t *testing.T) {
	suite.Run(t, new(ImportExportConnectionOUSuite))
}

func (s *ImportExportConnectionOUSuite) TestExportImportRoundTripKeepsOUID() {
	for _, kind := range ownedConnectionKinds() {
		s.Run(kind.vendor, func() {
			ouID, _ := s.createOU("conn-rt-ou-" + kind.vendor)
			name := fmt.Sprintf("OU Round Trip %s %d", kind.vendor, time.Now().UnixNano())
			id, err := kind.create(name, ouID)
			s.Require().NoError(err)
			s.T().Cleanup(func() { _ = kind.delete(id) })

			exported := s.exportConnection(id)
			document := s.parseExportedDocument(exported, id)
			s.Equal(ouID, document["ouId"], "the exported document should carry the owner as ouId")
			s.NotContains(document, "ouHandle", "the exported document should not name the owner by handle")

			s.Require().NoError(kind.delete(id))
			s.Equal(http.StatusNotFound, s.connectionStatus(kind.vendor, id))

			response := s.importContent(exported, false, exportedVariables(exported))
			s.Require().Len(response.Results, 1)
			s.Equal(1, response.Summary.Imported, "message: %s", response.Results[0].Message)
			s.Equal(0, response.Summary.Failed, "message: %s", response.Results[0].Message)
			s.Equal("success", response.Results[0].Status, "message: %s", response.Results[0].Message)
			s.Equal("create", response.Results[0].Operation)
			s.Equal(id, response.Results[0].ResourceID, "the import should keep the exported id")
			s.deleteAfterTest(kind.vendor, id)

			s.Equal(ouID, s.connectionOwner(kind.vendor, id), "the imported connection should keep its owner")
		})
	}
}

func (s *ImportExportConnectionOUSuite) TestImportNamesOwnerByOUHandle() {
	for _, kind := range ownedConnectionKinds() {
		s.Run(kind.vendor, func() {
			ouID, handle := s.createOU("conn-handle-ou-" + kind.vendor)
			id := newImportUUID(s)
			name := fmt.Sprintf("OU Handle %s %d", kind.vendor, time.Now().UnixNano())

			response := s.importContent(kind.document(id, name, "ouHandle: "+handle+"\n"), false, nil)
			s.Require().Len(response.Results, 1)
			s.Equal("success", response.Results[0].Status, "message: %s", response.Results[0].Message)
			s.Equal("create", response.Results[0].Operation)
			s.Equal(1, response.Summary.Imported)
			s.Equal(0, response.Summary.Failed)
			s.deleteAfterTest(kind.vendor, id)

			s.Equal(ouID, s.connectionOwner(kind.vendor, id),
				"the connection should be owned by the unit the handle names")
		})
	}
}

func (s *ImportExportConnectionOUSuite) TestImportDryRunResolvesOUHandle() {
	for _, kind := range ownedConnectionKinds() {
		s.Run(kind.vendor, func() {
			_, handle := s.createOU("conn-dry-ou-" + kind.vendor)

			validID := newImportUUID(s)
			validName := fmt.Sprintf("OU Dry Run %s %d", kind.vendor, time.Now().UnixNano())
			valid := s.importContent(kind.document(validID, validName, "ouHandle: "+handle+"\n"), true, nil)
			s.Require().Len(valid.Results, 1)
			s.Equal("success", valid.Results[0].Status, "message: %s", valid.Results[0].Message)
			s.Equal(0, valid.Summary.Failed)
			s.Equal(http.StatusNotFound, s.connectionStatus(kind.vendor, validID),
				"a dry run must not create the connection")

			unknownID := newImportUUID(s)
			unknown := s.importContent(kind.document(unknownID,
				fmt.Sprintf("OU Dry Run Unknown %s %d", kind.vendor, time.Now().UnixNano()),
				fmt.Sprintf("ouHandle: missing-conn-ou-%d\n", time.Now().UnixNano())), true, nil)
			s.Require().Len(unknown.Results, 1)
			s.Equal("failed", unknown.Results[0].Status, "an unknown handle should fail the item in a dry run")
			s.Equal(kind.unknownCode, unknown.Results[0].Code, "message: %s", unknown.Results[0].Message)
			s.Equal(1, unknown.Summary.Failed)
			s.Equal(0, unknown.Summary.Imported)
			s.Equal(http.StatusNotFound, s.connectionStatus(kind.vendor, unknownID))
		})
	}
}

func (s *ImportExportConnectionOUSuite) TestImportRefusesUnknownOUHandle() {
	for _, kind := range ownedConnectionKinds() {
		s.Run(kind.vendor, func() {
			id := newImportUUID(s)
			name := fmt.Sprintf("OU Unknown Handle %s %d", kind.vendor, time.Now().UnixNano())
			owner := fmt.Sprintf("ouHandle: missing-conn-ou-%d\n", time.Now().UnixNano())

			response := s.importContent(kind.document(id, name, owner), false, nil)
			s.Require().Len(response.Results, 1)
			s.Equal("failed", response.Results[0].Status)
			s.Equal(kind.unknownCode, response.Results[0].Code, "message: %s", response.Results[0].Message)
			s.Equal(1, response.Summary.Failed)
			s.Equal(0, response.Summary.Imported)
			s.Equal(http.StatusNotFound, s.connectionStatus(kind.vendor, id),
				"a refused document must not create a connection")
		})
	}
}

func (s *ImportExportConnectionOUSuite) TestImportRefusesDocumentWithoutOwner() {
	for _, kind := range ownedConnectionKinds() {
		s.Run(kind.vendor, func() {
			id := newImportUUID(s)
			name := fmt.Sprintf("OU Missing Owner %s %d", kind.vendor, time.Now().UnixNano())

			response := s.importContent(kind.document(id, name, ""), false, nil)
			s.Require().Len(response.Results, 1)
			s.Equal("failed", response.Results[0].Status)
			s.Equal(kind.requiredCode, response.Results[0].Code, "message: %s", response.Results[0].Message)
			s.Equal(1, response.Summary.Failed)
			s.Equal(0, response.Summary.Imported)
			s.Equal(http.StatusNotFound, s.connectionStatus(kind.vendor, id),
				"a refused document must not create a connection")
		})
	}
}

func (s *ImportExportConnectionOUSuite) TestImportOUIDWinsOverOUHandle() {
	for _, kind := range ownedConnectionKinds() {
		s.Run(kind.vendor, func() {
			ouOne, _ := s.createOU("conn-both-one-" + kind.vendor)
			ouTwo, handleTwo := s.createOU("conn-both-two-" + kind.vendor)
			s.NotEqual(ouOne, ouTwo)

			id := newImportUUID(s)
			name := fmt.Sprintf("OU Both %s %d", kind.vendor, time.Now().UnixNano())
			owner := fmt.Sprintf("ouId: %s\nouHandle: %s\n", ouOne, handleTwo)

			response := s.importContent(kind.document(id, name, owner), false, nil)
			s.Require().Len(response.Results, 1)
			s.Equal("success", response.Results[0].Status, "message: %s", response.Results[0].Message)
			s.Equal(1, response.Summary.Imported)
			s.deleteAfterTest(kind.vendor, id)

			s.Equal(ouOne, s.connectionOwner(kind.vendor, id), "the ouId should win over the ouHandle")
		})
	}
}

// createOU creates a top-level organization unit and removes it when the test ends. It registers the removal at the
// time it is called, and cleanups run last in first out, so a connection registered afterwards is deleted before the
// organization unit that owns it. It returns the unit's id and its handle, which is also its handle path.
func (s *ImportExportConnectionOUSuite) createOU(prefix string) (string, string) {
	s.T().Helper()
	handle := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{Handle: handle, Name: handle})
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			s.T().Logf("cleanup: failed to delete organization unit %s: %v", ouID, err)
		}
	})
	return ouID, handle
}

// deleteAfterTest removes a connection that import created when the test ends, whether or not the test already removed
// it. The connection is addressed by vendor, since the test helpers only know the connections they created themselves.
func (s *ImportExportConnectionOUSuite) deleteAfterTest(vendor, id string) {
	s.T().Cleanup(func() {
		req, err := http.NewRequest(http.MethodDelete,
			fmt.Sprintf("%s/connections/%s/%s", testutils.TestServerURL, vendor, id), nil)
		if err != nil {
			return
		}
		resp, err := testutils.GetHTTPClient().Do(req)
		if err != nil {
			return
		}
		_ = resp.Body.Close()
	})
}

// getConnection reads a connection through its vendor and returns the status and the body.
func (s *ImportExportConnectionOUSuite) getConnection(vendor, id string) (int, []byte) {
	s.T().Helper()
	resp, err := testutils.GetHTTPClient().Get(
		fmt.Sprintf("%s/connections/%s/%s", testutils.TestServerURL, vendor, id))
	s.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return resp.StatusCode, body
}

// connectionStatus returns the status the vendor read of a connection answers with.
func (s *ImportExportConnectionOUSuite) connectionStatus(vendor, id string) int {
	s.T().Helper()
	status, _ := s.getConnection(vendor, id)
	return status
}

// connectionOwner returns the ouId the vendor read of a connection reports.
func (s *ImportExportConnectionOUSuite) connectionOwner(vendor, id string) string {
	s.T().Helper()
	status, body := s.getConnection(vendor, id)
	s.Require().Equal(http.StatusOK, status, "connection %s/%s should exist: %s", vendor, id, string(body))

	var connection struct {
		ID   string `json:"id"`
		OUID string `json:"ouId"`
	}
	s.Require().NoError(json.Unmarshal(body, &connection))
	s.Equal(id, connection.ID)
	return connection.OUID
}

// exportConnection exports a connection by id and returns the YAML the export carries.
func (s *ImportExportConnectionOUSuite) exportConnection(id string) string {
	s.T().Helper()
	exported, err := postExport(exportRequest{Connections: []string{id}})
	s.Require().NoError(err)
	s.Require().NotEmpty(exported, "the export should carry the connection")
	return exported
}

// parseExportedDocument decodes the exported document whose id matches. An exported secret is a bare {{.VAR}} token,
// which YAML reads as the start of a flow mapping, so the tokens are replaced with a plain scalar first.
func (s *ImportExportConnectionOUSuite) parseExportedDocument(bundle, id string) map[string]interface{} {
	s.T().Helper()
	parseable := exportedPlaceholder.ReplaceAllString(bundle, "redacted")
	decoder := yaml.NewDecoder(strings.NewReader(parseable))
	for {
		var document map[string]interface{}
		err := decoder.Decode(&document)
		if errors.Is(err, io.EOF) {
			break
		}
		s.Require().NoError(err, "failed to decode the exported YAML: %s", parseable)
		if document != nil && document["id"] == id {
			return document
		}
	}
	s.Require().Fail("the export did not contain connection "+id, parseable)
	return nil
}

// exportedVariables supplies a value for every {{.VAR}} token in an export, so the document can be imported again.
func exportedVariables(exported string) map[string]interface{} {
	variables := map[string]interface{}{}
	for _, match := range exportedPlaceholder.FindAllStringSubmatch(exported, -1) {
		variables[match[1]] = "round-trip-" + strings.ToLower(match[1])
	}
	return variables
}

// importContent imports a document into the runtime layer and returns the report. The API carries on past a refused
// item by default, so the request does the same.
func (s *ImportExportConnectionOUSuite) importContent(
	content string, dryRun bool, variables map[string]interface{},
) *testutils.ImportResponse {
	s.T().Helper()
	response, err := postImport(testutils.ImportRequest{
		Content: content,
		DryRun:  dryRun,
		Options: testutils.ImportOptions{Upsert: false, ContinueOnError: true, Target: "runtime"},
		Vars:    variables,
	})
	s.Require().NoError(err)
	return response
}
