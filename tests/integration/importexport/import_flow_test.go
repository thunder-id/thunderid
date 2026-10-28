// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// TestImportFlowPromptInputConstant covers the flow import path: a constant on a prompt input is refused,
// and the same flow without the constant is imported.
func (suite *ImportExportErrorSuite) TestImportFlowPromptInputConstant() {
	id := newImportUUID(suite)
	handle := "import-prompt-constant-" + fmt.Sprintf("%d", time.Now().UnixNano())
	defer func() {
		if err := testutils.DeleteFlow(id); err != nil {
			suite.T().Logf("Failed to delete imported flow %s: %v", id, err)
		}
	}()

	result := suite.importFlow(promptFlowDocument(id, handle, "constant: true\n"))
	suite.Equal("failed", result.Status)
	suite.Equal("FLM-1024", result.Code)

	result = suite.importFlow(promptFlowDocument(id, handle, ""))
	suite.Equal("success", result.Status, "message: %s", result.Message)
}

// importFlow imports a single flow document and returns its outcome.
func (suite *ImportExportErrorSuite) importFlow(content string) importItem {
	payload := suite.importBody(importRequest{Content: content, Options: runtimeOptions()})

	status, body := suite.postJSON(suite.client, "/import", payload)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var resp importResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Require().Len(resp.Results, 1, "body: %s", body)
	return resp.Results[0]
}

// promptFlowDocument builds a flow import document whose prompt input has a default and the given
// extra property, written as an indented YAML line or nothing.
func promptFlowDocument(id, handle, extraInputProperty string) string {
	extra := ""
	if extraInputProperty != "" {
		extra = "            " + strings.TrimRight(extraInputProperty, "\n") + "\n"
	}

	return fmt.Sprintf("resource_type: flow\nid: %s\nhandle: %q\nname: %q\nflowType: RECOVERY\nnodes:\n"+
		"  - id: start\n    type: START\n    onSuccess: prompt\n"+
		"  - id: prompt\n    type: PROMPT\n    prompts:\n"+
		"      - inputs:\n"+
		"          - ref: input_faculty\n            identifier: faculty\n            type: TEXT_INPUT\n"+
		"            required: false\n            default: Engineering\n%s"+
		"        action:\n          ref: submit\n          nextNode: end\n"+
		"  - id: end\n    type: END\n", id, handle, "Import Prompt Constant", extra)
}
