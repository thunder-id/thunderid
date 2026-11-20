// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// An application's passkey allowed origins are stored by an import, on create and on update.
func (suite *ImportExportErrorSuite) TestImportApplicationKeepsPasskeyAllowedOrigins() {
	appID := newImportUUID(suite)
	defer func() { _ = testutils.DeleteApplication(appID) }()
	content := fmt.Sprintf("resource_type: application\nid: %s\nname: %q\ntype: custom\nouHandle: default\n"+
		"passkeyAllowedOrigins:\n  - %s\n", appID, "Import Passkey App "+appID[:8], testutils.TestServerURL)
	payload := suite.importBody(testutils.ImportRequest{Content: content, Options: runtimeOptions()})

	for _, operation := range []string{"create", "update"} {
		status, body := suite.postJSON(suite.client, "/import", payload)
		suite.Require().Equal(http.StatusOK, status, "body: %s", body)
		var resp testutils.ImportResponse
		suite.Require().NoError(json.Unmarshal(body, &resp))
		suite.Require().Len(resp.Results, 1, "body: %s", body)
		suite.Require().Equal("success", resp.Results[0].Status, "body: %s", body)
		suite.Require().Equal(operation, resp.Results[0].Operation)

		suite.Equal([]string{testutils.TestServerURL}, suite.getApplication(appID).PasskeyAllowedOrigins,
			"after %s", operation)
	}
}

// getApplication reads an application through the management API.
func (suite *ImportExportErrorSuite) getApplication(appID string) testutils.Application {
	resp, err := suite.client.Get(testutils.TestServerURL + "/applications/" + appID)
	suite.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, resp.StatusCode, "body: %s", body)

	var app testutils.Application
	suite.Require().NoError(json.Unmarshal(body, &app))
	return app
}
