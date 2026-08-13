// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ImportExportErrorSuite covers the request-level failure branches of /import, /import/delete and
// /export. The round-trip suites in this package only ever assert success, so every IMP-*/EXP-*
// code, the per-document failure outcome, and the continueOnError early-break are exercised here.
type ImportExportErrorSuite struct {
	suite.Suite
	client      *http.Client
	plainClient *http.Client
}

func TestImportExportErrorSuite(t *testing.T) {
	suite.Run(t, new(ImportExportErrorSuite))
}

func (suite *ImportExportErrorSuite) SetupSuite() {
	suite.client = testutils.GetHTTPClient()
	suite.plainClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// --- POST /import request validation ---

func (suite *ImportExportErrorSuite) TestImportEmptyContentReturns400() {
	status, body := suite.postJSON(suite.client, "/import", `{"content":""}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1001", suite.errorCode(body))
}

func (suite *ImportExportErrorSuite) TestImportMissingContentFieldReturns400() {
	status, body := suite.postJSON(suite.client, "/import", `{}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1001", suite.errorCode(body))
}

func (suite *ImportExportErrorSuite) TestImportMalformedBodyReturns400() {
	status, body := suite.postJSON(suite.client, "/import", `{`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1001", suite.errorCode(body))
}

// TestImportFileTargetReturns400 pins that the file target is rejected; only the runtime target is
// supported through the API.
func (suite *ImportExportErrorSuite) TestImportFileTargetReturns400() {
	payload := suite.importBody(importRequest{
		Content: "resource_type: organization_unit\nname: Target Test\nhandle: target-test\n",
		Options: importOptions{Upsert: true, ContinueOnError: true, Target: "file"},
	})

	status, body := suite.postJSON(suite.client, "/import", payload)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1001", suite.errorCode(body))
}

func (suite *ImportExportErrorSuite) TestImportInvalidYAMLReturns400() {
	payload := suite.importBody(importRequest{
		Content: "resource_type: organization_unit\n  name: badly indented\n\tmore: bad\n",
		Options: runtimeOptions(),
	})

	status, body := suite.postJSON(suite.client, "/import", payload)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1002", suite.errorCode(body))
}

// TestImportNonMappingRootReturns400 covers the parser's "root must be a YAML mapping" branch.
func (suite *ImportExportErrorSuite) TestImportNonMappingRootReturns400() {
	payload := suite.importBody(importRequest{
		Content: "- just\n- a\n- sequence\n",
		Options: runtimeOptions(),
	})

	status, body := suite.postJSON(suite.client, "/import", payload)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1002", suite.errorCode(body))
}

// TestImportUnknownResourceTypeReturns400 covers the parser's unresolvable-resource-type branch.
// An unknown type fails the whole request rather than producing a per-document failure.
func (suite *ImportExportErrorSuite) TestImportUnknownResourceTypeReturns400() {
	payload := suite.importBody(importRequest{
		Content: "resource_type: not_a_real_type\nname: Nope\n",
		Options: runtimeOptions(),
	})

	status, body := suite.postJSON(suite.client, "/import", payload)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1002", suite.errorCode(body))
}

func (suite *ImportExportErrorSuite) TestImportMissingResourceTypeFieldReturns400() {
	payload := suite.importBody(importRequest{
		Content: "name: No Resource Type\nhandle: no-resource-type\n",
		Options: runtimeOptions(),
	})

	status, body := suite.postJSON(suite.client, "/import", payload)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1002", suite.errorCode(body))
}

// TestImportUnresolvedTemplateVariableReturns400 pins missingkey=error: a placeholder with no
// matching variable is a hard failure, not an empty substitution.
func (suite *ImportExportErrorSuite) TestImportUnresolvedTemplateVariableReturns400() {
	payload := suite.importBody(importRequest{
		Content: "resource_type: organization_unit\nname: {{.MISSING_VAR}}\nhandle: missing-var\n",
		Options: runtimeOptions(),
	})

	status, body := suite.postJSON(suite.client, "/import", payload)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1003", suite.errorCode(body))
}

func (suite *ImportExportErrorSuite) TestImportMalformedTemplateReturns400() {
	payload := suite.importBody(importRequest{
		Content: "resource_type: organization_unit\nname: {{ .Unclosed \nhandle: unclosed\n",
		Options: runtimeOptions(),
	})

	status, body := suite.postJSON(suite.client, "/import", payload)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1003", suite.errorCode(body))
}

// TestImportEmptyDocumentsSucceedsWithZeroCount asserts content that carries no YAML documents
// (comments and blank lines only) parses to zero documents and reports an empty, successful import
// rather than an error. Content that is literally empty is rejected earlier, by the content check.
func (suite *ImportExportErrorSuite) TestImportEmptyDocumentsSucceedsWithZeroCount() {
	payload := suite.importBody(importRequest{
		Content: "# no documents here\n\n",
		Options: runtimeOptions(),
	})

	status, body := suite.postJSON(suite.client, "/import", payload)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var resp importResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Equal(0, resp.Summary.TotalDocuments)
	suite.Equal(0, resp.Summary.Imported)
	suite.Equal(0, resp.Summary.Failed)
	suite.Empty(resp.Results)
}

// --- Per-document failure outcomes ---

// TestImportInvalidDocumentReportsFailedOutcome asserts a document that parses but fails domain
// validation yields a 200 with a per-item failed outcome carrying a code, not a request-level error.
func (suite *ImportExportErrorSuite) TestImportInvalidDocumentReportsFailedOutcome() {
	// An OU document with no name/handle passes the parser and fails in the OU service.
	payload := suite.importBody(importRequest{
		Content: "resource_type: organization_unit\ndescription: missing name and handle\n",
		Options: runtimeOptions(),
	})

	status, body := suite.postJSON(suite.client, "/import", payload)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var resp importResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Equal(1, resp.Summary.TotalDocuments)
	suite.Equal(0, resp.Summary.Imported)
	suite.Equal(1, resp.Summary.Failed)
	suite.Require().Len(resp.Results, 1)
	suite.Equal("failed", resp.Results[0].Status)
	suite.Equal("organization_unit", resp.Results[0].ResourceType)
	suite.NotEmpty(resp.Results[0].Code, "a failed outcome must carry an error code")
}

// TestImportContinueOnErrorFalseStopsAtFirstFailure asserts the early break: with continueOnError
// disabled, documents after the first failure are never attempted, so len(results) is short of
// totalDocuments.
func (suite *ImportExportErrorSuite) TestImportContinueOnErrorFalseStopsAtFirstFailure() {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	// The invalid OU sorts first (same resource type, original order preserved), so the valid one
	// that follows must never be attempted.
	content := "resource_type: organization_unit\ndescription: missing name and handle\n" +
		"---\n" +
		fmt.Sprintf("resource_type: organization_unit\nname: Should Not Import %s\nhandle: should-not-import-%s\n",
			suffix, suffix)

	payload := suite.importBody(importRequest{
		Content: content,
		Options: importOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
	})

	status, body := suite.postJSON(suite.client, "/import", payload)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var resp importResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Equal(2, resp.Summary.TotalDocuments)
	suite.Equal(1, resp.Summary.Failed)
	suite.Equal(0, resp.Summary.Imported)
	suite.Len(resp.Results, 1, "processing must stop after the first failure")
}

// TestImportContinueOnErrorTrueProcessesRemaining is the counterpart: the same payload with
// continueOnError enabled imports the valid document despite the earlier failure.
func (suite *ImportExportErrorSuite) TestImportContinueOnErrorTrueProcessesRemaining() {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	handle := "continue-on-error-" + suffix
	content := "resource_type: organization_unit\ndescription: missing name and handle\n" +
		"---\n" +
		fmt.Sprintf("resource_type: organization_unit\nname: Continue On Error %s\nhandle: %s\n", suffix, handle)

	payload := suite.importBody(importRequest{
		Content: content,
		Options: runtimeOptions(),
	})

	status, body := suite.postJSON(suite.client, "/import", payload)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var resp importResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Equal(2, resp.Summary.TotalDocuments)
	suite.Equal(1, resp.Summary.Failed)
	suite.Equal(1, resp.Summary.Imported)
	suite.Len(resp.Results, 2)

	suite.deleteImportedOU(&resp)
}

// TestImportDryRunDoesNotPersist asserts a dry run reports success without creating the resource.
func (suite *ImportExportErrorSuite) TestImportDryRunDoesNotPersist() {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	handle := "dry-run-ou-" + suffix

	payload := suite.importBody(importRequest{
		Content: fmt.Sprintf("resource_type: organization_unit\nname: Dry Run OU %s\nhandle: %s\n", suffix, handle),
		DryRun:  true,
		Options: runtimeOptions(),
	})

	status, body := suite.postJSON(suite.client, "/import", payload)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var resp importResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Equal(1, resp.Summary.Imported)
	suite.Equal(0, resp.Summary.Failed)

	// Nothing was written, so a real import of the same handle still creates it.
	confirmPayload := suite.importBody(importRequest{
		Content: fmt.Sprintf("resource_type: organization_unit\nname: Dry Run OU %s\nhandle: %s\n", suffix, handle),
		Options: runtimeOptions(),
	})
	confirmStatus, confirmBody := suite.postJSON(suite.client, "/import", confirmPayload)
	suite.Require().Equal(http.StatusOK, confirmStatus, "body: %s", confirmBody)

	var confirmResp importResponse
	suite.Require().NoError(json.Unmarshal(confirmBody, &confirmResp))
	suite.Require().Len(confirmResp.Results, 1)
	suite.Equal("success", confirmResp.Results[0].Status)
	suite.Equal("create", confirmResp.Results[0].Operation,
		"the dry run must not have persisted the OU, so this is still a create")

	suite.deleteImportedOU(&confirmResp)
}

// --- POST /import/delete ---

func (suite *ImportExportErrorSuite) TestImportDeleteMissingFieldsReturns400() {
	for _, body := range []string{
		`{}`,
		`{"resourceType":"application"}`,
		`{"resourceKey":"some-key"}`,
	} {
		status, respBody := suite.postJSON(suite.client, "/import/delete", body)

		suite.Equal(http.StatusBadRequest, status, "payload: %s", body)
		suite.Equal("IMP-1001", suite.errorCode(respBody), "payload: %s", body)
	}
}

func (suite *ImportExportErrorSuite) TestImportDeleteMalformedBodyReturns400() {
	status, body := suite.postJSON(suite.client, "/import/delete", `{`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1001", suite.errorCode(body))
}

// TestImportDeleteUnsupportedResourceTypeReturns400 pins that only file-backed resource types can
// be deleted; group is importable but has no declarative directory mapping.
func (suite *ImportExportErrorSuite) TestImportDeleteUnsupportedResourceTypeReturns400() {
	status, body := suite.postJSON(suite.client, "/import/delete",
		`{"resourceType":"group","resourceKey":"some-group"}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1001", suite.errorCode(body))
}

func (suite *ImportExportErrorSuite) TestImportDeleteUnknownResourceTypeReturns400() {
	status, body := suite.postJSON(suite.client, "/import/delete",
		`{"resourceType":"not_a_real_type","resourceKey":"whatever"}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1001", suite.errorCode(body))
}

// TestImportDeleteUnknownKeyReturns400 covers the no-matching-file branch for a supported resource
// type (the directory may or may not exist; both branches answer IMP-1001).
func (suite *ImportExportErrorSuite) TestImportDeleteUnknownKeyReturns400() {
	status, body := suite.postJSON(suite.client, "/import/delete",
		`{"resourceType":"application","resourceKey":"definitely-not-a-real-application-key"}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1001", suite.errorCode(body))
}

func (suite *ImportExportErrorSuite) TestImportDeleteBlankKeyReturns400() {
	status, body := suite.postJSON(suite.client, "/import/delete",
		`{"resourceType":"application","resourceKey":"   "}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("IMP-1001", suite.errorCode(body))
}

// --- POST /export ---

func (suite *ImportExportErrorSuite) TestExportMalformedBodyReturns400() {
	status, body := suite.postJSON(suite.client, "/export", `{`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("EXP-1001", suite.errorCode(body))
}

// TestExportUnknownIDReturnsError asserts an export naming only a non-existent resource does not
// silently return an empty document.
func (suite *ImportExportErrorSuite) TestExportUnknownIDReturnsError() {
	status, body := suite.postJSON(suite.client, "/export",
		`{"applications":["00000000-0000-0000-0000-000000000000"]}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("EXP-1002", suite.errorCode(body))
}

// --- Auth gate ---

// TestImportEndpointsRequireAuth asserts both import routes reject unauthenticated callers. They
// are gated on the root permission, so an anonymous request must never reach the service.
func (suite *ImportExportErrorSuite) TestImportEndpointsRequireAuth() {
	importStatus, _ := suite.postJSON(suite.plainClient, "/import", `{"content":""}`)
	suite.Equal(http.StatusUnauthorized, importStatus)

	deleteStatus, _ := suite.postJSON(suite.plainClient, "/import/delete",
		`{"resourceType":"application","resourceKey":"anything"}`)
	suite.Equal(http.StatusUnauthorized, deleteStatus)
}

// --- helpers ---

// runtimeOptions returns the default runtime-target options with upsert and continueOnError on.
func runtimeOptions() importOptions {
	return importOptions{Upsert: true, ContinueOnError: true, Target: "runtime"}
}

// importBody marshals an import request to a JSON string.
func (suite *ImportExportErrorSuite) importBody(req importRequest) string {
	payload, err := json.Marshal(req)
	suite.Require().NoError(err)
	return string(payload)
}

// deleteImportedOU removes any OU created by an import so the shared server is not left mutated.
func (suite *ImportExportErrorSuite) deleteImportedOU(resp *importResponse) {
	for _, result := range resp.Results {
		if result.Status != "success" || result.ResourceType != "organization_unit" || result.ResourceID == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(result.ResourceID); err != nil {
			suite.T().Logf("Failed to delete imported OU %s: %v", result.ResourceID, err)
		}
	}
}

// postJSON POSTs a JSON body to a path and returns the status and response body.
func (suite *ImportExportErrorSuite) postJSON(client *http.Client, path, body string) (int, []byte) {
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+path, bytes.NewReader([]byte(body)))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
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

// errorCode decodes the error code from an API error response body.
func (suite *ImportExportErrorSuite) errorCode(body []byte) string {
	var errResp struct {
		Code string `json:"code"`
	}
	suite.Require().NoError(json.Unmarshal(body, &errResp), "body: %s", body)
	return errResp.Code
}
