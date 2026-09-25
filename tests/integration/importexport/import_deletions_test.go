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

// ImportDeletionsSuite covers the deletions an import request carries: the resources a configuration
// dropped, removed from the deployment the configuration was applied to.
//
// This is a different operation from POST /import/delete, which ImportDeleteSuite covers. That
// removes a declarative file from the server's configuration directory. These deletions remove
// runtime resources through the services that own them, and are reported per resource in the same
// results list the upserts use.
//
// Themes are the fixture throughout: they are created and read over plain HTTP, nothing else
// references them, and removing one leaves the rest of the deployment alone.
type ImportDeletionsSuite struct {
	suite.Suite
	// created holds the themes a test made, so one that fails part way still cleans up after itself.
	created []string
}

func TestImportDeletionsSuite(t *testing.T) {
	suite.Run(t, new(ImportDeletionsSuite))
}

// TearDownTest removes whatever a test created and the deletion under test did not.
func (suite *ImportDeletionsSuite) TearDownTest() {
	for _, id := range suite.created {
		req, err := http.NewRequest(http.MethodDelete, testutils.TestServerURL+"/design/themes/"+id, nil)
		if err != nil {
			continue
		}
		resp, err := testutils.GetHTTPClient().Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}
	suite.created = nil
}

// A deletion removes the resource and is counted separately from the upserts.
func (suite *ImportDeletionsSuite) TestADeletionRemovesTheResource() {
	id := suite.plantTheme("removes")

	result := suite.importWith(importRequest{
		Deletions: []resourceDeletion{{ResourceType: "theme", ID: id}},
	})

	suite.Require().Len(result.Results, 1)
	suite.Equal("success", result.Results[0].Status, "message: %s", result.Results[0].Message)
	suite.Equal("delete", result.Results[0].Operation)
	suite.Equal(id, result.Results[0].ResourceID)
	suite.Equal(1, result.Summary.Deleted)
	suite.Equal(0, result.Summary.Failed)

	suite.assertGone(id)
}

// Re-applying the same configuration has to be a no-op, so removing something already absent is a
// success rather than the failure that would make a retry impossible.
func (suite *ImportDeletionsSuite) TestDeletingSomethingAbsentSucceeds() {
	id := suite.plantTheme("absent")

	first := suite.importWith(importRequest{Deletions: []resourceDeletion{{ResourceType: "theme", ID: id}}})
	suite.Require().Equal("success", first.Results[0].Status)

	second := suite.importWith(importRequest{Deletions: []resourceDeletion{{ResourceType: "theme", ID: id}}})
	suite.Require().Len(second.Results, 1)
	suite.Equal("success", second.Results[0].Status,
		"deleting an already absent resource failed: %s", second.Results[0].Message)
	suite.Equal(0, second.Summary.Failed)
}

// A resource type whose service cannot remove is reported as such, rather than silently doing
// nothing and leaving the configuration looking reconciled when it is not.
func (suite *ImportDeletionsSuite) TestAnUndeletableTypeIsReported() {
	for _, resourceType := range []string{"translation", "server_config"} {
		suite.Run(resourceType, func() {
			result := suite.importWith(importRequest{
				Deletions: []resourceDeletion{{ResourceType: resourceType, ID: "anything"}},
			})

			suite.Require().Len(result.Results, 1)
			suite.Equal("failed", result.Results[0].Status,
				"%s reported as deletable", resourceType)
			suite.NotEmpty(result.Results[0].Message)
			suite.Equal(1, result.Summary.Failed)
			suite.Equal(0, result.Summary.Deleted)
		})
	}
}

// A resource type nothing knows about is refused, not guessed at.
func (suite *ImportDeletionsSuite) TestAnUnknownTypeIsRefused() {
	result := suite.importWith(importRequest{
		Deletions: []resourceDeletion{{ResourceType: "not-a-resource-type", ID: "some-id"}},
	})

	suite.Require().Len(result.Results, 1)
	suite.Equal("failed", result.Results[0].Status)
	suite.Equal(1, result.Summary.Failed)
}

// A deletion needs both halves to name anything, and saying which is missing beats a generic refusal.
func (suite *ImportDeletionsSuite) TestADeletionNeedsBothTypeAndID() {
	for label, deletion := range map[string]resourceDeletion{
		"no id":   {ResourceType: "theme"},
		"no type": {ID: "some-id"},
		"neither": {},
	} {
		suite.Run(label, func() {
			result := suite.importWith(importRequest{Deletions: []resourceDeletion{deletion}})

			suite.Require().Len(result.Results, 1)
			suite.Equal("failed", result.Results[0].Status)
			suite.Contains(result.Results[0].Message, "required")
		})
	}
}

// A dry run reports what it would remove and removes nothing.
func (suite *ImportDeletionsSuite) TestADryRunRemovesNothing() {
	id := suite.plantTheme("dry-run")

	result := suite.importWith(importRequest{
		DryRun:    true,
		Deletions: []resourceDeletion{{ResourceType: "theme", ID: id}},
	})

	suite.Require().Len(result.Results, 1)
	suite.Equal("success", result.Results[0].Status)
	suite.assertPresent(id, "a dry run removed the resource")
}

// One refused deletion does not hold back the rest of the request.
func (suite *ImportDeletionsSuite) TestOneRefusalDoesNotStopTheOthers() {
	first := suite.plantTheme("batch-one")
	second := suite.plantTheme("batch-two")

	result := suite.importWith(importRequest{
		Deletions: []resourceDeletion{
			{ResourceType: "theme", ID: first},
			{ResourceType: "translation", ID: "cannot-be-deleted"},
			{ResourceType: "theme", ID: second},
		},
	})

	suite.Require().Len(result.Results, 3)
	suite.Equal(2, result.Summary.Deleted)
	suite.Equal(1, result.Summary.Failed)

	suite.assertGone(first)
	suite.assertGone(second)
}

// A request carrying deletions and no content is a valid request: removing what a configuration
// dropped does not require also restating what it kept.
func (suite *ImportDeletionsSuite) TestDeletionsWithoutContent() {
	id := suite.plantTheme("no-content")

	result := suite.importWith(importRequest{
		Content:   "",
		Deletions: []resourceDeletion{{ResourceType: "theme", ID: id}},
	})

	suite.Equal(0, result.Summary.TotalDocuments, "an empty payload counted as a document")
	suite.Equal(1, result.Summary.Deleted)
	suite.assertGone(id)
}

// A request with neither content nor deletions is refused exactly as it was before. Deletions gave
// an empty payload something to mean; without them it still means nothing.
func (suite *ImportDeletionsSuite) TestNeitherContentNorDeletionsIsStillRefused() {
	id := suite.plantTheme("untouched")

	status, body := suite.importExpectingStatus(importRequest{})

	suite.Equal(http.StatusBadRequest, status, "an empty request was accepted: %s", body)
	suite.assertPresent(id, "a request carrying nothing removed a resource")
}

// Every type the importer can remove resolves to its own service. Deleting an id none of them holds
// still resolves that service and calls it, so this walks the whole set in one request and asserts
// each one answers: a type wired to nothing would come back unsupported rather than already absent.
//
// The ids are deliberately absent. What is under test is that the type resolves and the removal is
// reported as a success, not that any particular resource went away.
func (suite *ImportDeletionsSuite) TestEveryDeletableTypeResolves() {
	deletable := []string{
		"application",
		"connection",
		"flow",
		"organization_unit",
		"user_type",
		"role",
		"group",
		"resource_server",
		"theme",
		"layout",
		"user",
		"agent",
		"presentation_definition",
		"credential_configuration",
	}

	deletions := make([]resourceDeletion, 0, len(deletable))
	for _, resourceType := range deletable {
		deletions = append(deletions, resourceDeletion{
			ResourceType: resourceType,
			ID:           "00000000-0000-0000-0000-000000000000",
		})
	}

	result := suite.importWith(importRequest{Deletions: deletions})

	suite.Require().Len(result.Results, len(deletable))
	for i, outcome := range result.Results {
		suite.Equal("success", outcome.Status,
			"%s did not resolve to a delete: %s", deletable[i], outcome.Message)
		suite.Equal("delete", outcome.Operation)
	}
	suite.Equal(0, result.Summary.Failed)

	// Not every service can say whether the id existed. Most return success for an absent id
	// rather than a not-found error, so the count here is the number of deletions that completed,
	// which is more than the number of resources that were actually there: none of them were.
	// Only the services that answer not-found are excluded from it.
	suite.LessOrEqual(result.Summary.Deleted, len(deletable))
}

// A user_type deletion is scoped by category, which defaults to the user category. A category that
// names nothing is refused rather than quietly falling back to the default.
func (suite *ImportDeletionsSuite) TestAUserTypeDeletionIsScopedByCategory() {
	absent := "00000000-0000-0000-0000-000000000000"

	for label, tc := range map[string]struct {
		deletion   resourceDeletion
		wantStatus string
	}{
		"category omitted": {resourceDeletion{ResourceType: "user_type", ID: absent}, "success"},
		"category given":   {resourceDeletion{ResourceType: "user_type", ID: absent, Category: "user"}, "success"},
		"category invalid": {resourceDeletion{ResourceType: "user_type", ID: absent, Category: "nonsense"}, "failed"},
	} {
		suite.Run(label, func() {
			result := suite.importWith(importRequest{Deletions: []resourceDeletion{tc.deletion}})

			suite.Require().Len(result.Results, 1)
			suite.Equal(tc.wantStatus, result.Results[0].Status, "message: %s", result.Results[0].Message)
		})
	}
}

// plantTheme creates a throwaway theme and returns its id, registering it for cleanup.
func (suite *ImportDeletionsSuite) plantTheme(suffix string) string {
	suite.T().Helper()

	handle := fmt.Sprintf("import-deletions-%s-%d", suffix, time.Now().UnixNano())
	payload, err := json.Marshal(map[string]interface{}{
		"handle":      handle,
		"displayName": handle,
		"theme":       map[string]interface{}{},
	})
	suite.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost,
		testutils.TestServerURL+"/design/themes", bytes.NewReader(payload))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusCreated, resp.StatusCode, "could not plant a theme: %s", string(body))

	var created map[string]interface{}
	suite.Require().NoError(json.Unmarshal(body, &created))
	id, _ := created["id"].(string)
	suite.Require().NotEmpty(id)

	suite.created = append(suite.created, id)
	return id
}

// importWith posts an import request and returns the response, failing the test if it is not a 200.
func (suite *ImportDeletionsSuite) importWith(reqBody importRequest) *importResponse {
	suite.T().Helper()

	if reqBody.Options.Target == "" {
		reqBody.Options.Target = "runtime"
	}
	// The API carries on past a failure by default. The zero value of this struct would send false
	// and stop at the first refusal, which is not the behaviour under test.
	reqBody.Options.ContinueOnError = true

	payload, err := json.Marshal(reqBody)
	suite.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost,
		testutils.TestServerURL+"/import", bytes.NewReader(payload))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, resp.StatusCode, "import failed: %s", string(body))

	var parsed importResponse
	suite.Require().NoError(json.Unmarshal(body, &parsed))
	return &parsed
}

// importExpectingStatus posts an import request and returns the status and body as they came back,
// for the requests the server is meant to refuse outright rather than report on.
func (suite *ImportDeletionsSuite) importExpectingStatus(reqBody importRequest) (int, string) {
	suite.T().Helper()

	if reqBody.Options.Target == "" {
		reqBody.Options.Target = "runtime"
	}

	payload, err := json.Marshal(reqBody)
	suite.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost,
		testutils.TestServerURL+"/import", bytes.NewReader(payload))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, string(body)
}

// assertGone fails unless the theme has been removed.
func (suite *ImportDeletionsSuite) assertGone(id string) {
	suite.T().Helper()
	suite.Equal(http.StatusNotFound, suite.themeStatus(id), "the theme was still there")
}

// assertPresent fails unless the theme is still there.
func (suite *ImportDeletionsSuite) assertPresent(id, message string) {
	suite.T().Helper()
	suite.Equal(http.StatusOK, suite.themeStatus(id), message)
}

func (suite *ImportDeletionsSuite) themeStatus(id string) int {
	suite.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/design/themes/"+id, nil)
	suite.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}
