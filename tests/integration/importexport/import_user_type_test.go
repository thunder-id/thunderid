// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// TestImportUserTypeUpsert covers the upsert flow of a user type import: a dry run and a real import of a
// new user type are creates, the same user type imported again is an update, and a changed handle fails.
func (suite *ImportExportErrorSuite) TestImportUserTypeUpsert() {
	id := newImportUUID(suite)
	handle := "import-ut-" + fmt.Sprintf("%d", time.Now().UnixNano())
	defer suite.deleteImportedUserType(id)

	result := suite.importUserType(userTypeDocument(id, handle, "Import User Type"), true, runtimeOptions())
	suite.Equal("success", result.Status, "message: %s", result.Message)
	suite.Equal("create", result.Operation)

	result = suite.importUserType(userTypeDocument(id, handle, "Import User Type"), false, runtimeOptions())
	suite.Require().Equal("success", result.Status, "message: %s", result.Message)
	suite.Equal("create", result.Operation)
	suite.Equal(id, result.ResourceID)

	result = suite.importUserType(userTypeDocument(id, handle, "Import User Type"), true, runtimeOptions())
	suite.Equal("success", result.Status, "message: %s", result.Message)
	suite.Equal("update", result.Operation)

	result = suite.importUserType(userTypeDocument(id, handle, "Import User Type Renamed"), false, runtimeOptions())
	suite.Equal("success", result.Status, "message: %s", result.Message)
	suite.Equal("update", result.Operation)
	suite.Equal("Import User Type Renamed", result.ResourceName)

	result = suite.importUserType(userTypeDocument(id, handle+"-changed", "Import User Type"), false,
		runtimeOptions())
	suite.Equal("failed", result.Status)
	suite.Equal("USRS-1017", result.Code)
}

// TestImportUserTypeWithoutUpsert covers the create only flow of a user type import.
func (suite *ImportExportErrorSuite) TestImportUserTypeWithoutUpsert() {
	id := newImportUUID(suite)
	handle := "import-ut-no-upsert-" + fmt.Sprintf("%d", time.Now().UnixNano())
	defer suite.deleteImportedUserType(id)
	options := importOptions{Upsert: false, ContinueOnError: true, Target: "runtime"}

	result := suite.importUserType(userTypeDocument(id, handle, "Import Without Upsert"), false, options)
	suite.Require().Equal("success", result.Status, "message: %s", result.Message)
	suite.Equal("create", result.Operation)

	result = suite.importUserType(userTypeDocument(id, handle, "Import Without Upsert"), false, options)
	suite.Equal("failed", result.Status)
	suite.NotEmpty(result.Code)
}

// TestImportUserTypeInvalidDocuments covers user type documents that fail to import.
func (suite *ImportExportErrorSuite) TestImportUserTypeInvalidDocuments() {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	testCases := []struct {
		name    string
		content string
	}{
		{
			name:    "handle is not a string",
			content: "resource_type: user_type\nhandle: [a, b]\ndisplayName: Invalid\n",
		},
		{
			name: "invalid category",
			content: userTypeDocument(newImportUUID(suite), "import-ut-category-"+suffix, "Invalid Category") +
				"category: robot\n",
		},
		{
			name:    "empty display name",
			content: userTypeDocument(newImportUUID(suite), "import-ut-empty-"+suffix, ""),
		},
		{
			name: "display name too long",
			content: userTypeDocument(newImportUUID(suite), "import-ut-long-"+suffix,
				strings.Repeat("a", 101)),
		},
		{
			name:    "invalid handle",
			content: userTypeDocument(newImportUUID(suite), "Invalid Handle "+suffix, "Invalid Handle"),
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := suite.importUserType(tc.content, false, runtimeOptions())
			suite.Equal("failed", result.Status)
			suite.NotEmpty(result.Code)
		})
	}
}

// importUserType imports a single user type document and returns its outcome.
func (suite *ImportExportErrorSuite) importUserType(content string, dryRun bool, options importOptions) importItem {
	payload := suite.importBody(importRequest{Content: content, DryRun: dryRun, Options: options})

	status, body := suite.postJSON(suite.client, "/import", payload)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var resp importResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Require().Len(resp.Results, 1, "body: %s", body)
	return resp.Results[0]
}

// deleteImportedUserType removes a user type created by an import.
func (suite *ImportExportErrorSuite) deleteImportedUserType(id string) {
	if err := testutils.DeleteUserType(id); err != nil {
		suite.T().Logf("Failed to delete imported user type %s: %v", id, err)
	}
}

// userTypeDocument builds a user type import document in the default organization unit.
func userTypeDocument(id, handle, displayName string) string {
	return fmt.Sprintf("resource_type: user_type\nid: %s\nhandle: %q\ndisplayName: %q\nouHandle: default\n"+
		"schema:\n  email:\n    type: string\n", id, handle, displayName)
}

// newImportUUID returns a random version 4 UUID.
func newImportUUID(suite *ImportExportErrorSuite) string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	suite.Require().NoError(err)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
