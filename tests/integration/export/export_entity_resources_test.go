// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ExportEntityResourcesTestSuite covers the export hooks of the resource types that no other export
// suite reaches: users, user types, agent types, and translations.
//
// Each exporter contributes four hooks to the shared export pipeline (GetAllResourceIDs,
// GetResourceByID, ValidateResource, GetResourceRules), so every type is exercised both by ID and by
// wildcard. The wildcard path is the one that runs GetAllResourceIDs, and it is also where a type
// filter mistake shows up, since the emitted bundle would then carry documents of the wrong type.
//
// The user exporter is the interesting one: it declares Credentials as a dynamic property field, so
// the credential the fixture user is created with must be replaced by a template variable and must
// not appear in the exported document.
type ExportEntityResourcesTestSuite struct {
	suite.Suite

	ouID       string
	userTypeID string
	userID     string
}

const (
	entityExportOUHandle = "export-entity-ou"

	entityExportUserTypeName = "export-entity-person"
	entityExportUsername     = "export-entity-user"
	entityExportPassword     = "ExportEntity@123"

	// The agent type is the shared singleton `default`, so its name is fixed.
	entityExportAgentTypeName = "default"

	// A language of this suite's own, so clearing it never touches the system defaults other
	// suites resolve against.
	entityExportLanguage  = "pt-BR"
	entityExportNamespace = "integration-export"
	entityExportKey       = "greeting"
	entityExportValue     = "Ola"
)

func TestExportEntityResourcesTestSuite(t *testing.T) {
	suite.Run(t, new(ExportEntityResourcesTestSuite))
}

func (ts *ExportEntityResourcesTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      entityExportOUHandle,
		Name:        "Export Entity OU",
		Description: "Organization unit for the entity resource export tests",
	})
	ts.Require().NoError(err, "Failed to create the test organization unit")
	ts.ouID = ouID

	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: entityExportUserTypeName,
		OUID: ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "Failed to create the user type")
	ts.userTypeID = userTypeID

	// The server allows a single `default` agent type, shared across suites and never deleted.
	_, err = testutils.CreateAgentType(testutils.UserType{
		OUID: ts.ouID,
		Schema: map[string]interface{}{
			"description": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "Failed to create the agent type")

	userID, err := testutils.CreateUser(testutils.User{
		Type: entityExportUserTypeName,
		OUID: ts.ouID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q, "email": "export-entity@example.com"}`,
			entityExportUsername, entityExportPassword)),
	})
	ts.Require().NoError(err, "Failed to create the user")
	ts.userID = userID

	ts.setTranslationOverride()
}

func (ts *ExportEntityResourcesTestSuite) TearDownSuite() {
	ts.clearTranslationLanguage()

	if ts.userID != "" {
		if err := testutils.DeleteUser(ts.userID); err != nil {
			ts.T().Logf("Failed to delete the test user: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("Failed to delete the user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete the test organization unit: %v", err)
		}
	}
}

// setTranslationOverride writes one override so the language becomes exportable. A language with no
// overrides is not a resource the exporter can enumerate.
func (ts *ExportEntityResourcesTestSuite) setTranslationOverride() {
	ts.T().Helper()

	body, err := json.Marshal(map[string]string{"value": entityExportValue})
	ts.Require().NoError(err)

	target := fmt.Sprintf("%s/i18n/languages/%s/translations/ns/%s/keys/%s",
		testServerURL, url.PathEscape(entityExportLanguage),
		url.PathEscape(entityExportNamespace), url.PathEscape(entityExportKey))

	req, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, resp.StatusCode,
		"failed to seed the translation override, body: %s", respBody)
}

// clearTranslationLanguage removes the suite's overrides so the shared server is left as found.
func (ts *ExportEntityResourcesTestSuite) clearTranslationLanguage() {
	ts.T().Helper()

	target := fmt.Sprintf("%s/i18n/languages/%s/translations",
		testServerURL, url.PathEscape(entityExportLanguage))

	req, err := http.NewRequest(http.MethodDelete, target, nil)
	if err != nil {
		ts.T().Logf("Failed to build the translation cleanup request: %v", err)
		return
	}

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		ts.T().Logf("Failed to clear the translation language: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		ts.T().Logf("Unexpected status clearing the translation language: %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

// TestUserExportByID verifies a user is emitted as a user document carrying its attributes.
func (ts *ExportEntityResourcesTestSuite) TestUserExportByID() {
	yamlContent, err := ts.exportResourcesYAML(ExportRequest{Users: []string{ts.userID}})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	ts.Assert().Contains(yamlContent, "resource_type: user")
	ts.Assert().Contains(yamlContent, `username: "`+entityExportUsername+`"`)
	ts.Assert().Contains(yamlContent, "export-entity@example.com")
	ts.Assert().Contains(yamlContent, "type: "+entityExportUserTypeName)
}

// TestUserExportParameterizesCredentials verifies the exported document does not carry the user's
// password. The user exporter declares Credentials as a dynamic property field precisely so secrets
// leave as template variables, and a bundle is a shareable artifact.
func (ts *ExportEntityResourcesTestSuite) TestUserExportParameterizesCredentials() {
	yamlContent, err := ts.exportResourcesYAML(ExportRequest{Users: []string{ts.userID}})
	ts.Require().NoError(err)

	ts.Assert().NotContains(yamlContent, entityExportPassword,
		"an exported user must not carry its plaintext credential")

	// The credential leaves as the template variable its DynamicPropertyFields declaration implies,
	// so the assertion above holds because the field is parameterized rather than dropped.
	ts.Assert().Contains(yamlContent, "credentials:",
		"an exported user must carry its credentials block")
	ts.Assert().Contains(yamlContent, `password: "{{.USER_EXPORT_ENTITY_USER_PASSWORD}}"`,
		"the credential must leave as a template variable")
}

// TestUserExportRefusesTwoUsersWithOneVariableName verifies an export that cannot represent both
// users is refused rather than returned.
//
// A user's password placeholder is named after the username, so two usernames that normalize to the
// same name claim one variable. Returning that bundle would import both users with the same password,
// and dropping one silently would leave a bundle that looks complete.
func (ts *ExportEntityResourcesTestSuite) TestUserExportRefusesTwoUsersWithOneVariableName() {
	first, err := testutils.CreateUser(testutils.User{
		Type: entityExportUserTypeName,
		OUID: ts.ouID,
		Attributes: json.RawMessage(
			`{"username": "clash@example.com", "password": "ExportEntity@123", "email": "a@example.com"}`),
	})
	ts.Require().NoError(err, "Failed to create the first user")
	defer func() { _ = testutils.DeleteUser(first) }()

	second, err := testutils.CreateUser(testutils.User{
		Type: entityExportUserTypeName,
		OUID: ts.ouID,
		Attributes: json.RawMessage(
			`{"username": "clash.example.com", "password": "ExportEntity@123", "email": "b@example.com"}`),
	})
	ts.Require().NoError(err, "Failed to create the second user")
	defer func() { _ = testutils.DeleteUser(second) }()

	_, err = ts.exportResourcesYAML(ExportRequest{Users: []string{first, second}})

	ts.Require().Error(err, "expected the export to be refused")
	ts.Assert().Contains(err.Error(), "EXP-1003",
		"the refusal must name the duplicate template variable error")
}

// TestUserExportWithWildcard verifies the wildcard form enumerates users and includes the fixture.
func (ts *ExportEntityResourcesTestSuite) TestUserExportWithWildcard() {
	yamlContent, err := ts.exportResourcesYAML(ExportRequest{Users: []string{"*"}})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	ts.Assert().Contains(yamlContent, "resource_type: user")
	ts.Assert().Contains(yamlContent, `username: "`+entityExportUsername+`"`)
}

// TestExportWithInvalidUserID verifies an unknown user ID yields no resources rather than a partial
// bundle that would silently omit it.
func (ts *ExportEntityResourcesTestSuite) TestExportWithInvalidUserID() {
	_, err := ts.exportResourcesYAML(ExportRequest{Users: []string{"non-existent-user-id"}})
	ts.Require().Error(err)
}

// ---------------------------------------------------------------------------
// User types and agent types
// ---------------------------------------------------------------------------

// TestUserTypeExportByID verifies a user type is emitted as a user_type document with its schema.
func (ts *ExportEntityResourcesTestSuite) TestUserTypeExportByID() {
	yamlContent, err := ts.exportResourcesYAML(ExportRequest{UserTypes: []string{ts.userTypeID}})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	ts.Assert().Contains(yamlContent, "resource_type: user_type")
	ts.Assert().Contains(yamlContent, "name: "+entityExportUserTypeName)
	ts.Assert().Contains(yamlContent, "username")
	ts.Assert().Contains(yamlContent, "email")
}

// TestUserTypeExportWithWildcard verifies the wildcard form enumerates user types.
func (ts *ExportEntityResourcesTestSuite) TestUserTypeExportWithWildcard() {
	yamlContent, err := ts.exportResourcesYAML(ExportRequest{UserTypes: []string{"*"}})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	ts.Assert().Contains(yamlContent, "resource_type: user_type")
	ts.Assert().Contains(yamlContent, "name: "+entityExportUserTypeName)
}

// TestAgentTypeExportWithWildcard verifies agent types export under their own resource type. The two
// entity-type exporters share one implementation split only by category, so the agent-type bundle
// must not be labelled user_type.
func (ts *ExportEntityResourcesTestSuite) TestAgentTypeExportWithWildcard() {
	yamlContent, err := ts.exportResourcesYAML(ExportRequest{AgentTypes: []string{"*"}})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	ts.Assert().Contains(yamlContent, "resource_type: agent_type")
	ts.Assert().Contains(yamlContent, "name: "+entityExportAgentTypeName)
	ts.Assert().NotContains(yamlContent, "resource_type: user_type",
		"an agent type export must not emit user_type documents")
}

// ---------------------------------------------------------------------------
// Translations
// ---------------------------------------------------------------------------

// TestTranslationExportByLanguage verifies translations export per language, keyed by namespace.
func (ts *ExportEntityResourcesTestSuite) TestTranslationExportByLanguage() {
	yamlContent, err := ts.exportResourcesYAML(
		ExportRequest{Translations: []string{entityExportLanguage}})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	ts.Assert().Contains(yamlContent, "resource_type: translation")
	ts.Assert().Contains(yamlContent, "language: "+entityExportLanguage)
	ts.Assert().Contains(yamlContent, entityExportNamespace)
	ts.Assert().Contains(yamlContent, entityExportKey+": "+entityExportValue)
}

// TestTranslationExportWithWildcard verifies the wildcard form enumerates every language holding
// overrides, which is what GetAllResourceIDs resolves from the store.
func (ts *ExportEntityResourcesTestSuite) TestTranslationExportWithWildcard() {
	yamlContent, err := ts.exportResourcesYAML(ExportRequest{Translations: []string{"*"}})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	ts.Assert().Contains(yamlContent, "resource_type: translation")
	ts.Assert().Contains(yamlContent, "language: "+entityExportLanguage)
}

// TestExportWithUnknownTranslationLanguage verifies a language with no overrides is not exportable,
// rather than producing an empty translation document that would clear overrides on re-import.
func (ts *ExportEntityResourcesTestSuite) TestExportWithUnknownTranslationLanguage() {
	_, err := ts.exportResourcesYAML(ExportRequest{Translations: []string{"zz-ZZ"}})
	ts.Require().Error(err)
}

// ---------------------------------------------------------------------------
// Mixed bundle
// ---------------------------------------------------------------------------

// TestEntityResourcesExportedTogether verifies one request spanning several of these types emits a
// document per type, since the pipeline iterates the resource map rather than short-circuiting on
// the first type it finds.
func (ts *ExportEntityResourcesTestSuite) TestEntityResourcesExportedTogether() {
	yamlContent, err := ts.exportResourcesYAML(ExportRequest{
		Users:        []string{ts.userID},
		UserTypes:    []string{ts.userTypeID},
		Translations: []string{entityExportLanguage},
	})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	ts.Assert().Contains(yamlContent, "resource_type: user")
	ts.Assert().Contains(yamlContent, "resource_type: user_type")
	ts.Assert().Contains(yamlContent, "resource_type: translation")
}

// exportResourcesYAML posts an export request and returns the emitted resource bundle.
func (ts *ExportEntityResourcesTestSuite) exportResourcesYAML(
	exportRequest ExportRequest,
) (string, error) {
	reqJSON, err := json.Marshal(exportRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal export request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/export", bytes.NewReader(reqJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create export request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send export request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read export response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, body)
	}

	var jsonResponse JSONExportResponse
	if err := json.Unmarshal(body, &jsonResponse); err != nil {
		return "", fmt.Errorf("failed to parse JSON export response: %w", err)
	}
	return jsonResponse.Resources, nil
}
