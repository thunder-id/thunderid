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
// Themes are the fixture throughout: they are read over plain HTTP, nothing else references them, and
// removing one leaves the rest of the deployment alone. A data plane takes a theme, and its removal,
// only from an import with the management API key, so the requests here present that key.
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
		_ = testutils.DeleteImportedResource("theme", id)
	}
	suite.created = nil
}

// A deletion removes the resource and is counted separately from the upserts.
func (suite *ImportDeletionsSuite) TestADeletionRemovesTheResource() {
	id := suite.plantTheme("removes")

	result := suite.importWith(testutils.ImportRequest{
		Deletions: []testutils.ResourceDeletion{{ResourceType: "theme", ID: id}},
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

	first := suite.importWith(testutils.ImportRequest{Deletions: []testutils.ResourceDeletion{{ResourceType: "theme", ID: id}}})
	suite.Require().Equal("success", first.Results[0].Status)

	second := suite.importWith(testutils.ImportRequest{Deletions: []testutils.ResourceDeletion{{ResourceType: "theme", ID: id}}})
	suite.Require().Len(second.Results, 1)
	suite.Equal("success", second.Results[0].Status,
		"deleting an already absent resource failed: %s", second.Results[0].Message)
	suite.Equal(0, second.Summary.Failed)
}

// A resource type whose service cannot remove is reported as such, rather than silently doing
// nothing and leaving the configuration looking reconciled when it is not.
func (suite *ImportDeletionsSuite) TestAnUndeletableTypeIsReported() {
	result := suite.importWith(testutils.ImportRequest{
		Deletions: []testutils.ResourceDeletion{{ResourceType: "server_config", ID: "anything"}},
	})

	suite.Require().Len(result.Results, 1)
	suite.Equal("failed", result.Results[0].Status, "server_config reported as deletable")
	suite.NotEmpty(result.Results[0].Message)
	suite.Equal(1, result.Summary.Failed)
	suite.Equal(0, result.Summary.Deleted)
}

// A translation is removed by its language, which clears every custom translation of that language.
func (suite *ImportDeletionsSuite) TestATranslationDeletionClearsTheLanguage() {
	const language, namespace = "nl-BE", "integration-import-deletions"
	_, err := testutils.ImportResource("translation", map[string]interface{}{
		"language":     language,
		"translations": map[string]map[string]string{namespace: {"greeting": "Hallo"}},
	}, true)
	suite.Require().NoError(err)
	suite.Equal(map[string]string{"greeting": "Hallo"}, suite.overrides(language, namespace))

	result := suite.importWith(testutils.ImportRequest{
		Deletions: []testutils.ResourceDeletion{{ResourceType: "translation", ID: language}},
	})

	suite.Require().Len(result.Results, 1)
	suite.Equal("success", result.Results[0].Status, "message: %s", result.Results[0].Message)
	suite.Equal(0, result.Summary.Failed)
	suite.Empty(suite.overrides(language, namespace), "the language kept its translations")
}

// A data plane takes no theme removal from a token, so an import presenting one reports the deletion
// refused and leaves the theme in place.
func (suite *ImportDeletionsSuite) TestATokenCannotRemoveATheme() {
	id := suite.plantTheme("token")

	result := suite.importWithClient(testutils.GetHTTPClient(), testutils.ImportRequest{
		Deletions: []testutils.ResourceDeletion{{ResourceType: "theme", ID: id}},
	})

	suite.Require().Len(result.Results, 1)
	suite.Equal("failed", result.Results[0].Status)
	suite.Equal("SSE-4030", result.Results[0].Code)
	suite.assertPresent(id, "a token removed the theme")
}

// A resource type nothing knows about is refused, not guessed at.
func (suite *ImportDeletionsSuite) TestAnUnknownTypeIsRefused() {
	result := suite.importWith(testutils.ImportRequest{
		Deletions: []testutils.ResourceDeletion{{ResourceType: "not-a-resource-type", ID: "some-id"}},
	})

	suite.Require().Len(result.Results, 1)
	suite.Equal("failed", result.Results[0].Status)
	suite.Equal(1, result.Summary.Failed)
}

// A deletion needs both halves to name anything, and saying which is missing beats a generic refusal.
func (suite *ImportDeletionsSuite) TestADeletionNeedsBothTypeAndID() {
	for label, deletion := range map[string]testutils.ResourceDeletion{
		"no id":   {ResourceType: "theme"},
		"no type": {ID: "some-id"},
		"neither": {},
	} {
		suite.Run(label, func() {
			result := suite.importWith(testutils.ImportRequest{Deletions: []testutils.ResourceDeletion{deletion}})

			suite.Require().Len(result.Results, 1)
			suite.Equal("failed", result.Results[0].Status)
			suite.Contains(result.Results[0].Message, "required")
		})
	}
}

// A dry run reports what it would remove and removes nothing.
func (suite *ImportDeletionsSuite) TestADryRunRemovesNothing() {
	id := suite.plantTheme("dry-run")

	result := suite.importWith(testutils.ImportRequest{
		DryRun:    true,
		Deletions: []testutils.ResourceDeletion{{ResourceType: "theme", ID: id}},
	})

	suite.Require().Len(result.Results, 1)
	suite.Equal("success", result.Results[0].Status)
	suite.assertPresent(id, "a dry run removed the resource")
}

// One refused deletion does not hold back the rest of the request.
func (suite *ImportDeletionsSuite) TestOneRefusalDoesNotStopTheOthers() {
	first := suite.plantTheme("batch-one")
	second := suite.plantTheme("batch-two")

	result := suite.importWith(testutils.ImportRequest{
		Deletions: []testutils.ResourceDeletion{
			{ResourceType: "theme", ID: first},
			{ResourceType: "server_config", ID: "cannot-be-deleted"},
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

	result := suite.importWith(testutils.ImportRequest{
		Content:   "",
		Deletions: []testutils.ResourceDeletion{{ResourceType: "theme", ID: id}},
	})

	suite.Equal(0, result.Summary.TotalDocuments, "an empty payload counted as a document")
	suite.Equal(1, result.Summary.Deleted)
	suite.assertGone(id)
}

// A request with neither content nor deletions is refused exactly as it was before. Deletions gave
// an empty payload something to mean; without them it still means nothing.
func (suite *ImportDeletionsSuite) TestNeitherContentNorDeletionsIsStillRefused() {
	id := suite.plantTheme("untouched")

	status, body := suite.importExpectingStatus(testutils.ImportRequest{})

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
		"agent_type",
		"role",
		"group",
		"resource_server",
		"theme",
		"layout",
		"user",
		"agent",
		"presentation_definition",
		"credential_configuration",
		"translation",
	}

	deletions := make([]testutils.ResourceDeletion, 0, len(deletable))
	for _, resourceType := range deletable {
		id := "00000000-0000-0000-0000-000000000000"
		if resourceType == "translation" {
			// A translation is identified by its language, here one nothing writes.
			id = "zu-ZA"
		}
		deletions = append(deletions, testutils.ResourceDeletion{ResourceType: resourceType, ID: id})
	}

	result := suite.importWith(testutils.ImportRequest{Deletions: deletions})

	suite.Require().Len(result.Results, len(deletable))
	for _, outcome := range result.Results {
		suite.Equal("success", outcome.Status,
			"%s did not resolve to a delete: %s", outcome.ResourceType, outcome.Message)
		suite.Equal("delete", outcome.Operation)
	}
	suite.Equal(0, result.Summary.Failed)

	// Not every service can say whether the id existed. Most return success for an absent id
	// rather than a not-found error, so the count here is the number of deletions that completed,
	// which is more than the number of resources that were actually there: none of them were.
	// Only the services that answer not-found are excluded from it.
	suite.LessOrEqual(result.Summary.Deleted, len(deletable))
}

// Taking an assignee off a role, or a member off a group, succeeds for one deleted since, which lost the
// assignment and the membership with it, and still takes off the others.
func (suite *ImportDeletionsSuite) TestRemovingMembersSkipsThoseAlreadyGone() {
	suffix := fmt.Sprint(time.Now().UnixNano())
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{Handle: "removal-" + suffix,
		Name: "Removal"})
	suite.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(ouID) }()
	typeID, err := testutils.CreateUserType(testutils.UserType{Handle: "removal-" + suffix, DisplayName: "Removal",
		OUID: ouID, Schema: map[string]any{"email": map[string]any{"type": "string"}}})
	suite.Require().NoError(err)
	defer func() { _ = testutils.DeleteUserType(typeID) }()
	person := func(name string) string {
		id, err := testutils.CreateUser(testutils.User{Type: "removal-" + suffix, OUID: ouID,
			Attributes: json.RawMessage(fmt.Sprintf(`{"email": "%s-%s@example.com"}`, name, suffix))})
		suite.Require().NoError(err)
		return id
	}
	kept, gone := person("kept"), person("gone")
	defer func() { _ = testutils.DeleteUser(kept) }()
	groupID, err := testutils.CreateGroup(testutils.Group{Name: "removal-" + suffix, OUID: ouID,
		Members: []testutils.Member{{Id: kept, Type: "user"}, {Id: gone, Type: "user"}}})
	suite.Require().NoError(err)
	defer func() { _ = testutils.DeleteGroup(groupID) }()
	roleID, err := testutils.CreateRole(testutils.Role{Name: "removal-" + suffix, OUID: ouID,
		Assignments: []testutils.Assignment{{ID: kept, Type: "user"}, {ID: gone, Type: "user"}}})
	suite.Require().NoError(err)
	defer func() { _ = testutils.DeleteRole(roleID) }()
	suite.Require().NoError(testutils.DeleteUser(gone))

	result := suite.importWith(testutils.ImportRequest{Deletions: []testutils.ResourceDeletion{
		{ResourceType: "role_assignment", ID: roleID, Members: []testutils.RemovedMember{
			{ID: kept, Type: "user", OUID: ouID}, {ID: gone, Type: "user", OUID: ouID}}},
		{ResourceType: "group_member", ID: groupID, Members: []testutils.RemovedMember{
			{ID: kept, Type: "user"}, {ID: gone, Type: "user"}}},
	}})

	suite.Require().Len(result.Results, 2)
	for _, outcome := range result.Results {
		suite.Equal("success", outcome.Status, "%s: %s", outcome.ResourceType, outcome.Message)
	}
	assignees, err := testutils.GetRoleAssignments(roleID)
	suite.Require().NoError(err)
	suite.Empty(assignees, "the role kept an assignee the import took off")
	members, err := testutils.GetGroupMembers(groupID)
	suite.Require().NoError(err)
	suite.Empty(members, "the group kept a member the import took off")
}

// An agent type deletion reaches the entity type service in the agent category, which refuses it while
// an agent is of the agent type. The refusal is reported with the service's own reason rather than
// skipped, so a configuration that dropped an agent type still in use does not look reconciled. Once
// the agent is gone, the same deletion removes the agent type.
func (suite *ImportDeletionsSuite) TestAnAgentTypeDeletionReportsTheServiceRefusal() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "import-deletions-agent-type-ou", Name: "Import Deletions Agent Type OU",
	})
	suite.Require().NoError(err)
	defer func() { suite.NoError(testutils.DeleteOrganizationUnit(ouID)) }()

	agentTypeID, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle: "import-deletions-agents", DisplayName: "Import Deletions Agents", OUID: ouID,
		Schema: map[string]interface{}{"name": map[string]interface{}{"type": "string", "required": true}},
	})
	suite.Require().NoError(err)
	defer func() { _ = testutils.DeleteAgentType(agentTypeID) }()

	agentID, err := testutils.CreateAgent(testutils.Agent{
		Type: "import-deletions-agents", OUID: ouID,
		Attributes: map[string]interface{}{"name": "import-deletions-agent"},
	})
	suite.Require().NoError(err)
	agentDeleted := false
	defer func() {
		if !agentDeleted {
			_ = testutils.DeleteAgent(agentID)
		}
	}()

	deletion := testutils.ImportRequest{Deletions: []testutils.ResourceDeletion{{
		ResourceType: "agent_type",
		ID:           agentTypeID,
	}}}

	result := suite.importWith(deletion)
	suite.Require().Len(result.Results, 1)
	suite.Equal("failed", result.Results[0].Status)
	suite.Equal("USRS-1022", result.Results[0].Code, "message: %s", result.Results[0].Message)
	suite.Equal(1, result.Summary.Failed)
	suite.Equal(http.StatusOK, suite.getStatus("/agent-types/"+agentTypeID), "a refused deletion must keep it")

	suite.Require().NoError(testutils.DeleteAgent(agentID))
	agentDeleted = true

	result = suite.importWith(deletion)
	suite.Require().Len(result.Results, 1)
	suite.Equal("success", result.Results[0].Status, "message: %s", result.Results[0].Message)
	suite.Equal(1, result.Summary.Deleted)
	suite.Equal(http.StatusNotFound, suite.getStatus("/agent-types/"+agentTypeID))
}

// A connection is held by one of four services, and a deletion reaches whichever holds it: an
// identity provider, a notification sender, an AuthZEN PDP connection and an HTTP endpoint connection
// are each removed by the same resource type.
func (suite *ImportDeletionsSuite) TestADeletionRemovesAConnectionFromWhicheverServiceHoldsIt() {
	suffix := time.Now().UnixNano()
	planted := map[string]string{
		"google": suite.plantConnection("google", map[string]interface{}{
			"name": fmt.Sprintf("import-deletions-google-%d", suffix), "clientId": "g-client",
			"clientSecret": "g-secret", "redirectUri": "https://localhost:3000/google/callback",
			"scopes": []string{"openid"},
		}),
		"twilio": suite.plantConnection("twilio", map[string]interface{}{
			"name":       fmt.Sprintf("import-deletions-twilio-%d", suffix),
			"accountSid": "AC00000000000000000000000000000000",
			"authToken":  "tw-token",
			"senderId":   "+15005550006",
		}),
		"authzen-pdp": suite.plantConnection("authzen-pdp", map[string]interface{}{
			"name":     fmt.Sprintf("import-deletions-pdp-%d", suffix),
			"endpoint": "https://pdp.example.com/access/v1/evaluation",
		}),
		"http": suite.plantConnection("http", map[string]interface{}{
			"name": fmt.Sprintf("import-deletions-http-%d", suffix),
			"url":  "https://import-deletions.example.com/extension",
		}),
	}

	deletions := make([]testutils.ResourceDeletion, 0, len(planted))
	for _, id := range planted {
		deletions = append(deletions, testutils.ResourceDeletion{ResourceType: "connection", ID: id})
	}
	result := suite.importWith(testutils.ImportRequest{Deletions: deletions})

	suite.Require().Len(result.Results, len(planted))
	for _, outcome := range result.Results {
		suite.Equal("success", outcome.Status, "%s: %s", outcome.ResourceID, outcome.Message)
		suite.Empty(outcome.Message, "%s was reported absent rather than removed", outcome.ResourceID)
	}
	suite.Equal(len(planted), result.Summary.Deleted)

	for vendor, id := range planted {
		suite.Equal(http.StatusNotFound, suite.getStatus("/connections/"+vendor+"/"+id),
			"the %s connection is still there", vendor)
	}
}

// plantConnection creates a connection of the given vendor and returns its id, removing it at the
// end of the test if the deletion under test did not.
func (suite *ImportDeletionsSuite) plantConnection(vendor string, body map[string]interface{}) string {
	suite.T().Helper()

	ouID, ouErr := testutils.DefaultConnectionOUID()
	suite.Require().NoError(ouErr, "could not resolve the connection organization unit")
	body["ouId"] = ouID
	payload, err := json.Marshal(body)
	suite.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost,
		testutils.TestServerURL+"/connections/"+vendor, bytes.NewReader(payload))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusCreated, resp.StatusCode, "could not plant a %s connection: %s",
		vendor, string(respBody))

	var created struct {
		ID string `json:"id"`
	}
	suite.Require().NoError(json.Unmarshal(respBody, &created))
	suite.Require().NotEmpty(created.ID)

	suite.T().Cleanup(func() {
		cleanup, reqErr := http.NewRequest(http.MethodDelete,
			testutils.TestServerURL+"/connections/"+vendor+"/"+created.ID, nil)
		if reqErr != nil {
			return
		}
		if cleanupResp, doErr := testutils.GetHTTPClient().Do(cleanup); doErr == nil {
			_ = cleanupResp.Body.Close()
		}
	})
	return created.ID
}

// getStatus returns the status a GET of the path answers with.
func (suite *ImportDeletionsSuite) getStatus(path string) int {
	suite.T().Helper()

	resp, err := testutils.GetHTTPClient().Get(testutils.TestServerURL + path)
	suite.Require().NoError(err)
	_ = resp.Body.Close()
	return resp.StatusCode
}

// plantTheme imports a throwaway theme with the management API key and returns its id, registering it
// for cleanup.
func (suite *ImportDeletionsSuite) plantTheme(suffix string) string {
	suite.T().Helper()

	handle := fmt.Sprintf("import-deletions-%s-%d", suffix, time.Now().UnixNano())
	id, err := testutils.ImportResource("theme", map[string]interface{}{
		"handle":      handle,
		"displayName": handle,
		"theme":       map[string]interface{}{},
	}, false)
	suite.Require().NoError(err, "could not plant a theme")
	suite.Require().NotEmpty(id)

	suite.created = append(suite.created, id)
	return id
}

// overrides returns the custom translations a language resolves in the namespace.
func (suite *ImportDeletionsSuite) overrides(language, namespace string) map[string]string {
	suite.T().Helper()

	resp, err := testutils.GetHTTPClient().Get(fmt.Sprintf("%s/i18n/languages/%s/translations/resolve?namespace=%s",
		testutils.TestServerURL, language, namespace))
	suite.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var resolved struct {
		Translations map[string]map[string]string `json:"translations"`
	}
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&resolved))
	return resolved.Translations[namespace]
}

// importWith posts an import request with the management API key and returns the response, failing
// the test if it is not a 200.
func (suite *ImportDeletionsSuite) importWith(reqBody testutils.ImportRequest) *testutils.ImportResponse {
	suite.T().Helper()
	return suite.importWithClient(testutils.GetManagementAPIKeyHTTPClient(), reqBody)
}

// importWithClient posts an import request with the given client and returns the response, failing the
// test if it is not a 200.
func (suite *ImportDeletionsSuite) importWithClient(client *http.Client,
	reqBody testutils.ImportRequest) *testutils.ImportResponse {
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

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, resp.StatusCode, "import failed: %s", string(body))

	var parsed testutils.ImportResponse
	suite.Require().NoError(json.Unmarshal(body, &parsed))
	return &parsed
}

// importExpectingStatus posts an import request and returns the status and body as they came back,
// for the requests the server is meant to refuse outright rather than report on.
func (suite *ImportDeletionsSuite) importExpectingStatus(reqBody testutils.ImportRequest) (int, string) {
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

	resp, err := testutils.GetManagementAPIKeyHTTPClient().Do(req)
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
