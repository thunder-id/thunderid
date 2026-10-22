// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"fmt"
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

	result := suite.importFlow(promptFlowDocument(id, handle, "constant: true\n"), false, false)
	suite.Equal("failed", result.Status)
	suite.Equal("FLM-1024", result.Code)

	result = suite.importFlow(promptFlowDocument(id, handle, ""), false, false)
	suite.Equal("success", result.Status, "message: %s", result.Message)
}

// TestImportFlowPromptInputHeldConstant covers the flow import path: a prompt that asks for an input an
// executor holds constant is refused, and the same flow with an ordinary executor input is imported.
func (suite *ImportExportErrorSuite) TestImportFlowPromptInputHeldConstant() {
	id := newImportUUID(suite)
	handle := "import-held-constant-" + fmt.Sprintf("%d", time.Now().UnixNano())
	defer func() {
		if err := testutils.DeleteFlow(id); err != nil {
			suite.T().Logf("Failed to delete imported flow %s: %v", id, err)
		}
	}()

	result := suite.importFlow(heldConstantFlowDocument(id, handle, "constant: true\n"), false, false)
	suite.Equal("failed", result.Status)
	suite.Equal("FLM-1024", result.Code)

	result = suite.importFlow(heldConstantFlowDocument(id, handle, ""), false, false)
	suite.Equal("success", result.Status, "message: %s", result.Message)
}

// promptFlowDocument builds a flow import document whose prompt input has a default and the given
// extra property, written as an indented YAML line or nothing.
func promptFlowDocument(id, handle, extraInputProperty string) string {
	extra := ""
	if extraInputProperty != "" {
		extra = "            " + strings.TrimRight(extraInputProperty, "\n") + "\n"
	}

	return fmt.Sprintf("resource_type: flow\nid: %s\nhandle: %q\nname: %q\nflowType: RECOVERY\nouHandle: default\nnodes:\n"+
		"  - id: start\n    type: START\n    onSuccess: prompt\n"+
		"  - id: prompt\n    type: PROMPT\n    prompts:\n"+
		"      - inputs:\n"+
		"          - ref: input_faculty\n            identifier: faculty\n            type: TEXT_INPUT\n"+
		"            required: false\n            defaultValue: Engineering\n%s"+
		"        action:\n          ref: submit\n          nextNode: end\n"+
		"  - id: end\n    type: END\n", id, handle, "Import Prompt Constant", extra)
}

// heldConstantFlowDocument builds a flow import document whose prompt asks for the faculty and whose
// executor node lists it with a default and the given extra property, written as an indented YAML line
// or nothing.
func heldConstantFlowDocument(id, handle, extraInputProperty string) string {
	extra := ""
	if extraInputProperty != "" {
		extra = "          " + strings.TrimRight(extraInputProperty, "\n") + "\n"
	}

	return fmt.Sprintf("resource_type: flow\nid: %s\nhandle: %q\nname: %q\nflowType: AUTHENTICATION\nouHandle: default\nnodes:\n"+
		"  - id: start\n    type: START\n    onSuccess: prompt\n"+
		"  - id: prompt\n    type: PROMPT\n    prompts:\n"+
		"      - inputs:\n"+
		"          - ref: input_faculty\n            identifier: faculty\n            type: TEXT_INPUT\n"+
		"            required: false\n"+
		"        action:\n          ref: submit\n          nextNode: auth_assert\n"+
		"  - id: auth_assert\n    type: TASK_EXECUTION\n    onSuccess: end\n"+
		"    executor:\n      name: AuthAssertExecutor\n      inputs:\n"+
		"        - ref: input_faculty\n          identifier: faculty\n          type: TEXT_INPUT\n"+
		"          required: false\n          defaultValue: Engineering\n%s"+
		"  - id: end\n    type: END\n", id, handle, "Import Held Constant", extra)
}
