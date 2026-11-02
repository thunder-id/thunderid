// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ImportDeleteSuite covers the success path of POST /import/delete, which removes a declarative
// resource file from the server's configuration directory. The error branches are covered by
// ImportExportErrorSuite; this suite is the one that actually deletes a file.
//
// The fixture file is written directly into config/resources/themes rather than through the import
// API, because POST /import rejects the file target: writing declarative files is not an API
// operation, only removing them is. Declarative resources are read once at startup with no watcher,
// so adding a file mid-run affects nothing already loaded, and the delete under test removes it
// again. The file carries a handle of this suite's own so the shipped declarative theme is never a
// candidate match.
type ImportDeleteSuite struct {
	suite.Suite
	client *http.Client

	themesDir    string
	fixturePath  string
	fixtureExtra string
}

const (
	// importDeleteThemeID is the id of the throwaway declarative theme the suite plants and deletes.
	importDeleteThemeID = "import-delete-fixture-theme"

	// importDeleteExtraThemeID lives in a second file that must survive the delete, so the scan is
	// shown to remove only the matching document's file.
	importDeleteExtraThemeID = "import-delete-bystander-theme"
)

func TestImportDeleteSuite(t *testing.T) {
	suite.Run(t, new(ImportDeleteSuite))
}

func (suite *ImportDeleteSuite) SetupSuite() {
	suite.client = testutils.GetHTTPClient()

	suite.themesDir = filepath.Join(testutils.GetExtractedProductHome(), "config", "resources", "themes")
	suite.Require().DirExists(suite.themesDir,
		"the declarative themes directory must exist for the delete to have a target")

	suite.fixturePath = filepath.Join(suite.themesDir, importDeleteThemeID+".yaml")
	suite.fixtureExtra = filepath.Join(suite.themesDir, importDeleteExtraThemeID+".yaml")
}

// SetupTest re-plants both files so each test starts from the same on-disk state.
func (suite *ImportDeleteSuite) SetupTest() {
	suite.writeThemeFile(suite.fixturePath, importDeleteThemeID, "Import Delete Fixture Theme")
	suite.writeThemeFile(suite.fixtureExtra, importDeleteExtraThemeID, "Import Delete Bystander Theme")
}

func (suite *ImportDeleteSuite) TearDownSuite() {
	for _, path := range []string{suite.fixturePath, suite.fixtureExtra} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			suite.T().Logf("Failed to remove the fixture file %s: %v", path, err)
		}
	}
}

// writeThemeFile plants a declarative theme document on disk.
func (suite *ImportDeleteSuite) writeThemeFile(path, id, displayName string) {
	suite.T().Helper()

	content := "resource_type: theme\n" +
		"id: " + id + "\n" +
		"displayName: " + displayName + "\n" +
		"description: Planted by the import delete integration test\n" +
		"theme:\n" +
		"  primaryColor: \"#123456\"\n"

	suite.Require().NoError(os.WriteFile(path, []byte(content), 0o600))
}

// deleteResource posts an /import/delete request and returns the status with the raw body.
func (suite *ImportDeleteSuite) deleteResource(resourceType, resourceKey string) (int, []byte) {
	suite.T().Helper()

	body, err := json.Marshal(map[string]string{
		"resourceType": resourceType,
		"resourceKey":  resourceKey,
	})
	suite.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost,
		testutils.TestServerURL+"/import/delete", bytes.NewReader(body))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			suite.T().Logf("Failed to close response body: %v", closeErr)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, respBody
}

// deleteResponse mirrors the DeleteResourceResponse payload.
type deleteResponse struct {
	ResourceType string `json:"resourceType"`
	ResourceKey  string `json:"resourceKey"`
	DeletedFile  string `json:"deletedFile"`
}

// TestDeleteByIDRemovesTheFile verifies the matching file is removed from disk and reported back by
// name, which is what tells the caller which file was actually acted on.
func (suite *ImportDeleteSuite) TestDeleteByIDRemovesTheFile() {
	status, body := suite.deleteResource("theme", importDeleteThemeID)
	suite.Require().Equalf(http.StatusOK, status, "unexpected status, body: %s", body)

	var resp deleteResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Equal("theme", resp.ResourceType)
	suite.Equal(importDeleteThemeID, resp.ResourceKey)
	suite.Equal(importDeleteThemeID+".yaml", resp.DeletedFile)

	suite.NoFileExists(suite.fixturePath, "the matching file must be gone from disk")
	suite.FileExists(suite.fixtureExtra,
		"a non-matching file in the same directory must be left alone")
}

// TestDeleteByDisplayNameRemovesTheFile verifies the resource key is matched against the document's
// name as well as its id, so a caller need not know the generated identifier.
func (suite *ImportDeleteSuite) TestDeleteByDisplayNameRemovesTheFile() {
	status, body := suite.deleteResource("theme", "Import Delete Fixture Theme")
	suite.Require().Equalf(http.StatusOK, status, "unexpected status, body: %s", body)

	var resp deleteResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Equal(importDeleteThemeID+".yaml", resp.DeletedFile)

	suite.NoFileExists(suite.fixturePath)
}

// TestDeleteIsNotIdempotent verifies a second delete of the same key is refused rather than
// answering 200 for a file that is no longer there, so a caller cannot read success as confirmation
// that a resource it never planted has been removed.
func (suite *ImportDeleteSuite) TestDeleteIsNotIdempotent() {
	status, _ := suite.deleteResource("theme", importDeleteThemeID)
	suite.Require().Equal(http.StatusOK, status)

	status, body := suite.deleteResource("theme", importDeleteThemeID)
	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1001", suite.errorCodeOf(body))
}

// TestDeleteDoesNotMatchAnotherResourceType verifies the resource type is part of the match: a theme
// key must not delete a file whose documents are of a different type.
func (suite *ImportDeleteSuite) TestDeleteDoesNotMatchAnotherResourceType() {
	status, body := suite.deleteResource("layout", importDeleteThemeID)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1001", suite.errorCodeOf(body))
	suite.FileExists(suite.fixturePath, "a type mismatch must not delete the theme file")
}

// errorCodeOf decodes the error code from an API error response body.
func (suite *ImportDeleteSuite) errorCodeOf(body []byte) string {
	suite.T().Helper()

	var errResp struct {
		Code string `json:"code"`
	}
	suite.Require().NoError(json.Unmarshal(body, &errResp), "body: %s", body)
	return errResp.Code
}
