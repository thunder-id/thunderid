// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	flowOURequiredCode      = "FLM-1029"
	flowOUUnknownCode       = "FLM-1027"
	flowOUUpdateRefusedCode = "FLM-1028"
	flowDocumentNodes       = "nodes:\n" +
		"  - id: start\n    type: START\n    onSuccess: prompt_credentials\n" +
		"  - id: prompt_credentials\n    type: PROMPT\n    prompts:\n      - inputs:\n" +
		"          - ref: input_001\n            identifier: username\n            type: TEXT_INPUT\n" +
		"            required: true\n" +
		"          - ref: input_002\n            identifier: password\n            type: PASSWORD_INPUT\n" +
		"            required: true\n" +
		"        action:\n          ref: action_001\n          nextNode: credentials_auth\n" +
		"  - id: credentials_auth\n    type: TASK_EXECUTION\n    executor:\n      name: CredentialsAuthExecutor\n" +
		"      inputs:\n" +
		"        - ref: input_001\n          identifier: username\n          type: TEXT_INPUT\n" +
		"          required: true\n" +
		"        - ref: input_002\n          identifier: password\n          type: PASSWORD_INPUT\n" +
		"          required: true\n" +
		"    onSuccess: authorization_check\n" +
		"  - id: authorization_check\n    type: TASK_EXECUTION\n    executor:\n      name: AuthorizationExecutor\n" +
		"    onSuccess: auth_assert\n" +
		"  - id: auth_assert\n    type: TASK_EXECUTION\n    executor:\n      name: AuthAssertExecutor\n" +
		"    onSuccess: end\n" +
		"  - id: end\n    type: END\n"
)

// flowDocument builds a flow import document. owner holds the ouId and ouHandle lines the document names its owner
// with, each ending in a line break, or nothing.
func flowDocument(id, handle, owner string) string {
	return fmt.Sprintf("resource_type: flow\nid: %s\nhandle: %s\nname: Import Owner Flow\nflowType: AUTHENTICATION\n%s%s",
		id, handle, owner, flowDocumentNodes)
}

// importFlow imports a single flow document and returns its outcome.
func (suite *ImportExportErrorSuite) importFlow(content string, dryRun bool, upsert bool) testutils.ImportItem {
	payload := suite.importBody(testutils.ImportRequest{
		Content: content,
		DryRun:  dryRun,
		Options: testutils.ImportOptions{Upsert: upsert, ContinueOnError: true, Target: "runtime"},
	})
	status, body := suite.postJSON(suite.client, "/import", payload)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var resp testutils.ImportResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Require().Len(resp.Results, 1, "body: %s", body)
	return resp.Results[0]
}

// flowOwner returns the ouId a flow reports, or an empty string when the flow does not exist.
func (suite *ImportExportErrorSuite) flowOwner(id string) string {
	resp, err := testutils.GetHTTPClient().Get(testutils.TestServerURL + "/flows/" + id)
	suite.Require().NoError(err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var flow struct {
		OUID string `json:"ouId"`
	}
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&flow))
	return flow.OUID
}

// TestImportFlowOwner covers how an imported flow names the organization unit that owns it: by id, by handle, with
// the id winning when both are given, and what is refused when it names none or one that does not exist.
func (suite *ImportExportErrorSuite) TestImportFlowOwner() {
	defaultOUID, err := testutils.DefaultOUID()
	suite.Require().NoError(err)

	tests := []struct {
		name      string
		owner     string
		wantCode  string
		wantOwner string
	}{
		{name: "by id", owner: "ouId: " + defaultOUID + "\n", wantOwner: defaultOUID},
		{name: "by handle", owner: "ouHandle: default\n", wantOwner: defaultOUID},
		{name: "the id wins over the handle", owner: "ouId: " + defaultOUID + "\nouHandle: nowhere\n",
			wantOwner: defaultOUID},
		{name: "no owner is refused", owner: "", wantCode: flowOURequiredCode},
		{name: "unknown handle is refused", owner: "ouHandle: nowhere\n", wantCode: flowOUUnknownCode},
		{name: "unknown id is refused", owner: "ouId: 01900000-0000-7000-8000-0000000000ff\n",
			wantCode: flowOUUnknownCode},
	}
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			id := newImportUUID(suite)
			handle := fmt.Sprintf("import-flow-owner-%d", time.Now().UnixNano())
			defer func() { _ = testutils.DeleteFlow(id) }()

			dryRun := suite.importFlow(flowDocument(id, handle, tc.owner), true, false)
			result := suite.importFlow(flowDocument(id, handle, tc.owner), false, false)

			if tc.wantCode != "" {
				suite.Equal("failed", dryRun.Status, "the dry run refuses what the import refuses")
				suite.Equal(tc.wantCode, dryRun.Code)
				suite.Equal("failed", result.Status)
				suite.Equal(tc.wantCode, result.Code)
				suite.Empty(suite.flowOwner(id), "a refused flow is not created")
				return
			}
			suite.Equal("success", dryRun.Status, "message: %s", dryRun.Message)
			suite.Require().Equal("success", result.Status, "message: %s", result.Message)
			suite.Equal(tc.wantOwner, suite.flowOwner(id))
		})
	}
}

// TestImportFlowRefusesToChangeTheOwner covers an upsert of a flow that names another owner than the one it has: the
// dry run refuses it as the import does.
func (suite *ImportExportErrorSuite) TestImportFlowRefusesToChangeTheOwner() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("import-flow-other-%d", time.Now().UnixNano()), Name: "Import flow other owner",
	})
	suite.Require().NoError(err)
	id := newImportUUID(suite)
	handle := fmt.Sprintf("import-flow-owner-%d", time.Now().UnixNano())
	defer func() {
		_ = testutils.DeleteFlow(id)
		_ = testutils.DeleteOrganizationUnit(ouID)
	}()
	created := suite.importFlow(flowDocument(id, handle, "ouHandle: default\n"), false, false)
	suite.Require().Equal("success", created.Status, "message: %s", created.Message)

	other := flowDocument(id, handle, "ouId: "+ouID+"\n")
	dryRun := suite.importFlow(other, true, true)
	result := suite.importFlow(other, false, true)

	suite.Equal("failed", dryRun.Status)
	suite.Equal(flowOUUpdateRefusedCode, dryRun.Code)
	suite.Equal("failed", result.Status)
	suite.Equal(flowOUUpdateRefusedCode, result.Code)
}
