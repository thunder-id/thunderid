// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ImportProjectSkipSuite checks that the all-in-one server, which also runs as a Data Plane, skips the
// project documents of a Control Plane export and still imports the root organization unit beside it.
type ImportProjectSkipSuite struct {
	suite.Suite
}

func TestImportProjectSkipSuite(t *testing.T) {
	suite.Run(t, new(ImportProjectSkipSuite))
}

func (suite *ImportProjectSkipSuite) TestProjectDocumentIsSkipped() {
	rootID := "019a0000-0000-7000-8000-0000000003a1"
	handle := fmt.Sprintf("skipped-project-%d", time.Now().UnixNano())
	_ = testutils.DeleteOrganizationUnit(rootID)
	content := fmt.Sprintf("resource_type: project\nid: 019a0000-0000-7000-8000-0000000003a0\nrootOuId: %s\n"+
		"handle: %s\ndisplayName: Skipped\n---\nresource_type: organization_unit\nid: %s\nhandle: %s\n"+
		"name: Skipped\n", rootID, handle, rootID, handle)
	body, err := json.Marshal(map[string]any{
		"content": content,
		"options": map[string]any{"target": "runtime", "upsert": true},
	})
	suite.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/import", bytes.NewReader(body))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, resp.StatusCode, string(raw))
	suite.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(rootID) })

	var outcome struct {
		Summary struct {
			Failed int `json:"failed"`
		} `json:"summary"`
		Results []struct {
			ResourceType string `json:"resourceType"`
			Operation    string `json:"operation"`
			Status       string `json:"status"`
		} `json:"results"`
	}
	suite.Require().NoError(json.Unmarshal(raw, &outcome))
	suite.Equal(0, outcome.Summary.Failed, string(raw))
	operations := map[string]string{}
	for _, result := range outcome.Results {
		suite.Equal("success", result.Status, string(raw))
		operations[result.ResourceType] = result.Operation
	}
	suite.Equal("skip", operations["project"])
	suite.Equal("create", operations["organization_unit"])

	ou, err := testutils.GetOrganizationUnit(rootID)
	suite.Require().NoError(err, "the root organization unit is imported")
	suite.Equal(handle, ou.Handle)
}
