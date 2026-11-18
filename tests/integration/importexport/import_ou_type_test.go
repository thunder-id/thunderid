// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// TestImportOUTypeUpsert covers the upsert flow of an organization unit type import: a dry run and a real
// import of a new type are creates, the same type imported again is an update, and the type is served by
// the organization unit type API with its exported ID.
func (suite *ImportExportErrorSuite) TestImportOUTypeUpsert() {
	id := newImportUUID(suite)
	handle := "import-out-" + fmt.Sprintf("%d", time.Now().UnixNano())
	defer suite.deleteImportedOUType(id)

	result := suite.importUserType(ouTypeDocument(id, handle, "Import OU Type", "region"), true, runtimeOptions())
	suite.Equal("success", result.Status, "message: %s", result.Message)
	suite.Equal("create", result.Operation)
	suite.Equal("ou_type", result.ResourceType)

	result = suite.importUserType(ouTypeDocument(id, handle, "Import OU Type", "region"), false, runtimeOptions())
	suite.Require().Equal("success", result.Status, "message: %s", result.Message)
	suite.Equal("create", result.Operation)
	suite.Equal(id, result.ResourceID)

	status, body := suite.getOUType(id)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)
	suite.Contains(string(body), `"handle":"`+handle+`"`)

	result = suite.importUserType(ouTypeDocument(id, handle, "Import OU Type Renamed", "region"), false,
		runtimeOptions())
	suite.Equal("success", result.Status, "message: %s", result.Message)
	suite.Equal("update", result.Operation)
	suite.Equal("Import OU Type Renamed", result.ResourceName)
}

// TestImportOUTypeRefusesACredentialAttribute covers that an imported organization unit type gets the
// same credential check as one created through the API.
func (suite *ImportExportErrorSuite) TestImportOUTypeRefusesACredentialAttribute() {
	id := newImportUUID(suite)
	defer suite.deleteImportedOUType(id)
	document := ouTypeDocument(id, "import-out-credential-"+fmt.Sprintf("%d", time.Now().UnixNano()),
		"Credential", "apiKey") + "    credential: true\n"

	result := suite.importUserType(document, false, runtimeOptions())

	suite.Equal("failed", result.Status)
	suite.Equal("OUT-1009", result.Code, "message: %s", result.Message)
}

// TestExportOUTypeRoundTrips covers that an exported organization unit type is an ou_type document that
// imports back.
func (suite *ImportExportErrorSuite) TestExportOUTypeRoundTrips() {
	id := newImportUUID(suite)
	handle := "export-out-" + fmt.Sprintf("%d", time.Now().UnixNano())
	defer suite.deleteImportedOUType(id)
	result := suite.importUserType(ouTypeDocument(id, handle, "Export OU Type", "region"), false, runtimeOptions())
	suite.Require().Equal("success", result.Status, "message: %s", result.Message)

	exported, err := postExport(exportRequest{OUTypes: []string{id}})
	suite.Require().NoError(err)
	suite.Contains(exported, "resource_type: ou_type")
	suite.Contains(exported, "handle: "+handle)

	result = suite.importUserType(exported, false, runtimeOptions())
	suite.Equal("success", result.Status, "message: %s", result.Message)
	suite.Equal("update", result.Operation)
	suite.Equal(id, result.ResourceID)
}

// TestImportOUTypeDryRunOfAnExistingType covers that a dry run of a type that already exists reports an
// update and changes nothing.
func (suite *ImportExportErrorSuite) TestImportOUTypeDryRunOfAnExistingType() {
	id := newImportUUID(suite)
	handle := "import-out-dry-run-" + fmt.Sprintf("%d", time.Now().UnixNano())
	defer suite.deleteImportedOUType(id)
	result := suite.importUserType(ouTypeDocument(id, handle, "Dry Run", "region"), false, runtimeOptions())
	suite.Require().Equal("success", result.Status, "message: %s", result.Message)

	result = suite.importUserType(ouTypeDocument(id, handle, "Dry Run Renamed", "region"), true, runtimeOptions())

	suite.Equal("success", result.Status, "message: %s", result.Message)
	suite.Equal("update", result.Operation)
	status, body := suite.getOUType(id)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)
	suite.Contains(string(body), `"displayName":"Dry Run"`)
}

// TestImportOUTypeSchemaAsJSONString covers a document that writes the schema as a JSON string rather
// than as YAML.
func (suite *ImportExportErrorSuite) TestImportOUTypeSchemaAsJSONString() {
	id := newImportUUID(suite)
	handle := "import-out-json-schema-" + fmt.Sprintf("%d", time.Now().UnixNano())
	defer suite.deleteImportedOUType(id)
	document := fmt.Sprintf("resource_type: ou_type\nid: %s\nhandle: %q\ndisplayName: \"JSON Schema\"\n"+
		"ouHandle: default\nschema: '{\"region\":{\"type\":\"string\"}}'\n", id, handle)

	result := suite.importUserType(document, false, runtimeOptions())

	suite.Require().Equal("success", result.Status, "message: %s", result.Message)
	status, body := suite.getOUType(id)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)
	suite.Contains(string(body), `"region"`)
}

// TestImportOUTypeUpdateRefused covers that an upsert of an existing type reports the refusal of the
// organization unit type service as a failed update.
func (suite *ImportExportErrorSuite) TestImportOUTypeUpdateRefused() {
	id := newImportUUID(suite)
	handle := "import-out-update-refused-" + fmt.Sprintf("%d", time.Now().UnixNano())
	defer suite.deleteImportedOUType(id)
	result := suite.importUserType(ouTypeDocument(id, handle, "Update Refused", "region"), false, runtimeOptions())
	suite.Require().Equal("success", result.Status, "message: %s", result.Message)

	result = suite.importUserType(ouTypeDocument(id, handle, "Update Refused", "apiKey")+"    credential: true\n",
		false, runtimeOptions())

	suite.Equal("failed", result.Status)
	suite.Equal("update", result.Operation)
	suite.Equal("OUT-1009", result.Code, "message: %s", result.Message)
}

// TestImportOUTypeDeletion covers that an import deletion removes an organization unit type.
func (suite *ImportExportErrorSuite) TestImportOUTypeDeletion() {
	id := newImportUUID(suite)
	handle := "import-out-deletion-" + fmt.Sprintf("%d", time.Now().UnixNano())
	defer suite.deleteImportedOUType(id)
	result := suite.importUserType(ouTypeDocument(id, handle, "Deletion", "region"), false, runtimeOptions())
	suite.Require().Equal("success", result.Status, "message: %s", result.Message)

	payload := suite.importBody(testutils.ImportRequest{
		Deletions: []testutils.ResourceDeletion{{ResourceType: "ou_type", ID: id}},
	})
	status, body := suite.postJSON(suite.client, "/import", payload)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)
	var resp testutils.ImportResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))

	suite.Require().Len(resp.Results, 1, "body: %s", body)
	suite.Equal("success", resp.Results[0].Status, "message: %s", resp.Results[0].Message)
	suite.Equal("delete", resp.Results[0].Operation)
	status, body = suite.getOUType(id)
	suite.Equal(http.StatusNotFound, status, "body: %s", body)
}

// getOUType reads an organization unit type through its API.
func (suite *ImportExportErrorSuite) getOUType(id string) (int, []byte) {
	resp, err := suite.client.Get(testutils.TestServerURL + "/organization-unit-types/" + id)
	suite.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, body
}

// deleteImportedOUType removes an organization unit type created by an import.
func (suite *ImportExportErrorSuite) deleteImportedOUType(id string) {
	req, err := http.NewRequest(http.MethodDelete, testutils.TestServerURL+"/organization-unit-types/"+id, nil)
	suite.Require().NoError(err)
	resp, err := suite.client.Do(req)
	if err != nil {
		suite.T().Logf("Failed to delete imported organization unit type %s: %v", id, err)
		return
	}
	_ = resp.Body.Close()
}

// ouTypeDocument builds an organization unit type import document in the default organization unit with
// one string attribute.
func ouTypeDocument(id, handle, displayName, attribute string) string {
	return fmt.Sprintf("resource_type: ou_type\nid: %s\nhandle: %q\ndisplayName: %q\nouHandle: default\n"+
		"schema:\n  %s:\n    type: string\n", id, handle, displayName, attribute)
}
