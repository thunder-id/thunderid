// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// Seeded from resources/declarative_resources/connections/http-connection-declarative-1.yaml.
	httpConnDeclarativeID       = "decl-http-connection-1"
	httpConnDeclarativeName     = "Declarative HTTP Connection"
	httpConnDeclarativeURL      = "https://decl-http.example.com/extension"
	httpConnDeclarativeUser     = "decl-http-user"
	httpConnDeclarativePassword = "decl-http-connection-secret"

	// The shared declarative-resource refusals, which the connection handler answers with 400.
	httpConnDeclarativeCreateCode = "DCR-1001"
	httpConnDeclarativeUpdateCode = "DCR-1002"
	httpConnDeclarativeDeleteCode = "DCR-1003"

	httpConnImportOperationCreate = "create"
	httpConnImportOperationUpdate = "update"
	httpConnImportStatusSuccess   = "success"
	httpConnImportStatusFailed    = "failed"
)

type httpConnExportResponse struct {
	Resources            string `json:"resources"`
	EnvironmentVariables string `json:"environment_variables"`
}

// httpConnExportPlaceholder matches the {{.VAR}} tokens the exporter writes in place of secrets.
var httpConnExportPlaceholder = regexp.MustCompile(`{{[^}]*}}`)

// exportConnection exports one connection and returns the YAML bundle and the parsed .env entries.
func (s *HTTPConnectionAPITestSuite) exportConnection(id string) (string, map[string]string) {
	s.T().Helper()
	res, err := doRequest(http.MethodPost, "/export", map[string][]string{"connections": {id}})
	s.Require().NoError(err, "Export request failed")
	s.Require().Equal(http.StatusOK, res.status, "Export of connection %s should succeed: %s", id, string(res.body))
	var exported httpConnExportResponse
	s.Require().NoError(res.decode(&exported), "Export response should decode")
	s.Require().NotEmpty(exported.Resources, "Export should return the connection document")

	env := map[string]string{}
	for _, line := range strings.Split(exported.EnvironmentVariables, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		s.Require().True(ok, "Every .env line should be NAME=value: %q", line)
		env[name] = value
	}
	return exported.Resources, env
}

// exportedDocument decodes the exported document of the given connection. The secret placeholders are
// replaced first, since a bare {{.VAR}} is not valid YAML.
func (s *HTTPConnectionAPITestSuite) exportedDocument(bundle, id string) map[string]interface{} {
	s.T().Helper()
	decoder := yaml.NewDecoder(strings.NewReader(httpConnExportPlaceholder.ReplaceAllString(bundle, "__redacted__")))
	for {
		var document map[string]interface{}
		err := decoder.Decode(&document)
		if errors.Is(err, io.EOF) {
			break
		}
		s.Require().NoError(err, "Exported YAML should decode: %s", bundle)
		if documentID, _ := document["id"].(string); documentID == id {
			return document
		}
	}
	s.Require().Fail("The export did not contain connection "+id, bundle)
	return nil
}

// importConnections posts an import and returns its single outcome, tracking a created connection for
// teardown.
func (s *HTTPConnectionAPITestSuite) importConnections(
	content string, variables map[string]string, upsert bool) testutils.ImportItem {
	s.T().Helper()
	return s.importConnectionsWith(content, variables, upsert, false)
}

func (s *HTTPConnectionAPITestSuite) importConnectionsWith(
	content string, variables map[string]string, upsert, dryRun bool) testutils.ImportItem {
	s.T().Helper()
	vars := make(map[string]interface{}, len(variables))
	for name, value := range variables {
		vars[name] = value
	}
	res, err := doRequest(http.MethodPost, "/import", testutils.ImportRequest{
		Content: content,
		DryRun:  dryRun,
		Options: testutils.ImportOptions{Upsert: upsert, ContinueOnError: true, Target: "runtime"},
		Vars:    vars,
	})
	s.Require().NoError(err, "Import request failed")
	s.Require().Equal(http.StatusOK, res.status, "Import should be processed: %s", string(res.body))
	s.assertNoSecretLeaked(res, "Import response")
	var imported testutils.ImportResponse
	s.Require().NoError(res.decode(&imported), "Import response should decode")
	s.Require().Len(imported.Results, 1, "One connection document was imported: %s", string(res.body))
	outcome := imported.Results[0]
	s.Equal("connection", outcome.ResourceType, "The outcome should be for a connection")
	if !dryRun && outcome.Status == httpConnImportStatusSuccess &&
		outcome.Operation == httpConnImportOperationCreate && outcome.ResourceID != "" {
		s.createdIDs = append(s.createdIDs, outcome.ResourceID)
	}
	return outcome
}

// passwordVariable returns the single .env variable a basic connection exports, its password.
func (s *HTTPConnectionAPITestSuite) passwordVariable(env map[string]string) string {
	s.T().Helper()
	s.Require().Len(env, 1, "A basic connection exports exactly one secret, its password: %v", env)
	for name := range env {
		return name
	}
	return ""
}

// withoutLinesContaining drops every line of content that contains token, which is how an import
// document omits a field.
func withoutLinesContaining(content, token string) string {
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.Contains(line, token) {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// A deployment pipeline supplies the password by this variable name, derived as for SMTP connections.
func (s *HTTPConnectionAPITestSuite) TestExportWritesThePasswordAsAnEnvParameter() {
	const passwordVariable = "CONNECTION_HTTP_CONN_IE_EXPORT_PASSWORD"
	created, _ := s.create(httpConnRequest{
		Name: "http-conn-ie-export", Description: "exported", URL: "https://export.ext.example.com/hook",
		TimeoutMS: 2500, Headers: []httpConnHeader{{Name: "X-Tenant", Value: "t-1"}},
		Authentication: basicConnAuth(httpConnBasicUser, httpConnBasicPassword),
	})

	resources, env := s.exportConnection(created.ID)

	s.NotContains(resources, httpConnBasicPassword, "The exported YAML must not contain the password")
	s.NotContains(resources, maskedSecretValue,
		"The exported YAML must not carry the mask, which an import would store as the password")
	s.Contains(resources, "{{."+passwordVariable+"}}", "The password should be exported as a template variable")
	s.Equal(map[string]string{passwordVariable: httpConnBasicPassword}, env,
		"The .env should carry the password, and only the password, under the variable the YAML references")

	document := s.exportedDocument(resources, created.ID)
	s.Equal("connection", document["resource_type"], "The document should be a connection")
	s.Equal("http", document["type"], "The exported type should be http")
	s.Equal("http-conn-ie-export", document["name"], "The exported name should match")
	s.Equal("exported", document["description"], "The exported description should match")
	s.Equal("https://export.ext.example.com/hook", document["url"], "The exported URL should match")
	s.Equal(2500, document["timeoutMs"], "The exported timeout should match")
	s.Equal([]interface{}{map[string]interface{}{"name": "X-Tenant", "value": "t-1"}}, document["headers"],
		"Plain headers are not secrets and should be exported in clear")
	auth, ok := document["authentication"].(map[string]interface{})
	s.Require().True(ok, "The export should carry the authentication: %v", document)
	s.Equal("basic", auth["type"], "The exported authentication type should be basic")
	s.Equal(map[string]interface{}{"username": httpConnBasicUser, "password": "__redacted__"}, auth["properties"],
		"The username is exported in clear and the password only as a placeholder")
}

// Flows reference a connection by ID, so the import must keep the exported one.
func (s *HTTPConnectionAPITestSuite) TestImportOfTheExportRecreatesTheConnection() {
	created, _ := s.create(httpConnRequest{
		Name: "http-conn-ie-recreate", Description: "recreated", URL: "https://recreate.ext.example.com/hook",
		TimeoutMS: 3000, Headers: []httpConnHeader{{Name: "X-Tenant", Value: "t-2"}},
		Authentication: basicConnAuth(httpConnBasicUser, httpConnBasicPassword),
	})
	before := s.get(created.ID)
	resources, env := s.exportConnection(created.ID)

	res, err := doRequest(http.MethodDelete, httpConnPath+"/"+created.ID, nil)
	s.Require().NoError(err, "Delete request failed")
	s.Require().Equal(http.StatusNoContent, res.status, "Delete before import should succeed: %s", string(res.body))

	outcome := s.importConnections(resources, env, false)
	s.Require().Equal(httpConnImportStatusSuccess, outcome.Status, "Import should succeed: %s", outcome.Message)
	s.Equal(httpConnImportOperationCreate, outcome.Operation, "Import of an absent connection creates it")
	s.Require().Equal(created.ID, outcome.ResourceID, "Import should keep the exported id")

	s.Equal(before, s.get(created.ID), "The imported connection should read as the exported one, password masked")
	s.assertStoredAuth(created.ID, "basic", true)
}

// Otherwise an import could point a connection at another server and collect its stored password.
func (s *HTTPConnectionAPITestSuite) TestImportWithoutThePasswordKeepsItOnlyForTheSameOrigin() {
	const originalURL = "https://origin.ext.example.com:8443/hook"
	const samePathURL = "https://origin.ext.example.com:8443/hook/v2"
	created, _ := s.create(httpConnRequest{
		Name: "http-conn-ie-origin", URL: originalURL,
		Authentication: basicConnAuth(httpConnBasicUser, httpConnBasicPassword),
	})
	original, _, readable := s.storedPassword(created.ID)
	resources, env := s.exportConnection(created.ID)
	variable := s.passwordVariable(env)
	withoutPassword := withoutLinesContaining(resources, "{{."+variable+"}}")
	s.Require().NotContains(withoutPassword, variable, "The import document should omit the password")

	outcome := s.importConnections(strings.ReplaceAll(withoutPassword, originalURL, samePathURL), nil, true)
	s.Require().Equal(httpConnImportStatusSuccess, outcome.Status,
		"Import changing only the path should succeed: %s", outcome.Message)
	s.Equal(httpConnImportOperationUpdate, outcome.Operation, "Import of an existing connection updates it")
	kept := s.get(created.ID)
	s.Equal(samePathURL, kept.URL, "The import should update the URL")
	s.Equal(map[string]string{"username": httpConnBasicUser, "password": maskedSecretValue},
		kept.Authentication.Properties, "The kept password reads masked")
	if readable {
		stored, present, _ := s.storedPassword(created.ID)
		s.True(present, "The password should still be stored")
		s.Equal(original, stored, "The stored password must be the original")
	}

	beforeRow, _ := s.storedRow(created.ID)
	for _, tc := range []struct {
		context string
		url     string
	}{
		{"Host change", "https://attacker.example.net:8443/hook"},
		{"Port change", "https://origin.ext.example.com:9443/hook"},
	} {
		outcome = s.importConnections(strings.ReplaceAll(withoutPassword, originalURL, tc.url), nil, true)
		s.Equal(httpConnImportStatusFailed, outcome.Status, "%s without a password should fail", tc.context)
		s.Equal(httpConnInvalidAuthCode, outcome.Code, "%s without a password: unexpected code", tc.context)
		s.Equal(kept, s.get(created.ID), "%s: a refused import must leave the connection unchanged", tc.context)
		if readable {
			afterRow, _ := s.storedRow(created.ID)
			s.Equal(beforeRow, afterRow, "%s: a refused import must leave the stored record unchanged", tc.context)
		}
	}

	const movedURL = "https://moved.ext.example.com/hook"
	outcome = s.importConnections(strings.ReplaceAll(resources, originalURL, movedURL),
		map[string]string{variable: httpConnBasicPassword2}, true)
	s.Require().Equal(httpConnImportStatusSuccess, outcome.Status,
		"Import changing the origin with a new password should succeed: %s", outcome.Message)
	s.Equal(movedURL, s.get(created.ID).URL, "The import should move the connection")
	if readable {
		stored, _, _ := s.storedPassword(created.ID)
		s.NotEqual(original, stored, "The supplied password should replace the stored one")
	}
	s.assertStoredAuth(created.ID, "basic", true)
}

func (s *HTTPConnectionAPITestSuite) TestDeclarativeConnectionIsListedAndReadOnly() {
	declarative := s.get(httpConnDeclarativeID)
	s.Equal(httpConnResponse{
		ID: httpConnDeclarativeID, Name: httpConnDeclarativeName,
		Description: "A declarative HTTP endpoint connection", Type: "http", URL: httpConnDeclarativeURL,
		TimeoutMS: 3000, Headers: []httpConnHeader{{Name: "X-Tenant", Value: "decl"}},
		Authentication: httpConnAuthentication{Type: "basic",
			Properties: map[string]string{"username": httpConnDeclarativeUser, "password": maskedSecretValue}},
	}, declarative, "The declarative connection should read as defined, password masked")

	res, err := doRequest(http.MethodGet, httpConnPath, nil)
	s.Require().NoError(err, "Type list request failed")
	s.Require().Equal(http.StatusOK, res.status, "GET /connections/http should succeed: %s", string(res.body))
	s.assertNoSecretLeaked(res, "GET /connections/http")
	var summaries []connectionInstance
	s.Require().NoError(res.decode(&summaries), "GET /connections/http should be an array of summaries")
	s.True(containsID(summaries, httpConnDeclarativeID), "GET /connections/http should list the declarative one")

	res, err = doRequest(http.MethodGet, "/connections?category=http&limit=100", nil)
	s.Require().NoError(err, "Category list request failed")
	s.Require().Equal(http.StatusOK, res.status, "List by category http should succeed: %s", string(res.body))
	var list connectionListResponse
	s.Require().NoError(res.decode(&list), "Category list should decode")
	s.True(containsID(list.Connections, httpConnDeclarativeID), "category=http should list the declarative one")

	// The body is valid in full, password included, so only the read-only rule can refuse it.
	res = s.put(httpConnDeclarativeID, httpConnRequest{
		Name: httpConnDeclarativeName, URL: httpConnDeclarativeURL,
		Authentication: basicConnAuth(httpConnDeclarativeUser, httpConnBasicPassword2),
	})
	s.assertError(res, http.StatusBadRequest, httpConnDeclarativeUpdateCode, "Update of a declarative connection")

	res, err = doRequest(http.MethodDelete, httpConnPath+"/"+httpConnDeclarativeID, nil)
	s.Require().NoError(err, "Delete request failed")
	s.assertError(res, http.StatusBadRequest, httpConnDeclarativeDeleteCode, "Delete of a declarative connection")

	s.Equal(declarative, s.get(httpConnDeclarativeID),
		"Refused writes must leave the declarative connection as it was")
}

// The database's unique index sees only the mutable connections, so the composite store checks the name.
func (s *HTTPConnectionAPITestSuite) TestCreateCannotTakeTheNameOfADeclarativeConnection() {
	res := s.post(httpConnRequest{Name: httpConnDeclarativeName, URL: "https://taken.ext.example.com/hook"})

	s.assertError(res, http.StatusConflict, httpConnDuplicateCode, "Create with a declarative connection's name")
}

// A database row under a declared ID would shadow the declared connection on every read.
func (s *HTTPConnectionAPITestSuite) TestImportCannotCreateOverADeclarativeConnection() {
	document := "resource_type: connection\nid: " + httpConnDeclarativeID +
		"\ntype: http\nname: http-conn-ie-shadow\nurl: https://shadow.ext.example.com/hook\n"

	outcome := s.importConnections(document, nil, false)

	s.Equal(httpConnImportStatusFailed, outcome.Status, "Import over a declared ID should fail: %s", outcome.Message)
	s.Equal(httpConnDuplicateCode, outcome.Code, "Import over a declared ID: unexpected code")
	s.Equal(httpConnDeclarativeName, s.get(httpConnDeclarativeID).Name, "The declared connection must be unchanged")
}

// Reaches the primary key: the name is free, only the ID is taken.
func (s *HTTPConnectionAPITestSuite) TestImportWithoutUpsertCannotReuseAStoredID() {
	created, _ := s.create(httpConnRequest{Name: "http-conn-ie-id-taken", URL: "https://id.ext.example.com/hook"})
	document := "resource_type: connection\nid: " + created.ID +
		"\ntype: http\nname: http-conn-ie-id-other\nurl: https://other.ext.example.com/hook\n"

	outcome := s.importConnections(document, nil, false)

	s.Equal(httpConnImportStatusFailed, outcome.Status, "Import reusing a stored ID should fail: %s", outcome.Message)
	s.Equal(httpConnDuplicateCode, outcome.Code, "Import reusing a stored ID: unexpected code")
	s.Equal("http-conn-ie-id-taken", s.get(created.ID).Name, "The stored connection must be unchanged")
}

func (s *HTTPConnectionAPITestSuite) TestDryRunReportsTheValidationTheImportApplies() {
	document := "resource_type: connection\nid: http-conn-ie-dry-run\ntype: http\nname: http-conn-ie-dry-run\n" +
		"url: http://dry-run.ext.example.com/hook\nauthentication:\n  type: basic\n  properties:\n" +
		"    username: " + httpConnBasicUser + "\n    password: " + httpConnBasicPassword + "\n"

	outcome := s.importConnectionsWith(document, nil, true, true)

	s.Equal(httpConnImportStatusFailed, outcome.Status, "A credential over http should fail the dry run")
	s.Equal(httpConnInvalidURLCode, outcome.Code, "Dry run: unexpected code")
	res, err := doRequest(http.MethodGet, httpConnPath+"/http-conn-ie-dry-run", nil)
	s.Require().NoError(err, "Get request failed")
	s.Equal(http.StatusNotFound, res.status, "A dry run must not create the connection")
}

// http_connection.store can make the store declarative while the server's declarative mode stays off.
func (s *HTTPConnectionAPITestSuite) TestCreateIsRefusedWhenTheConnectionStoreIsDeclarative() {
	defer func() {
		s.Assert().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
			"http_connection": map[string]interface{}{"store": "composite"},
		}))
		s.Assert().NoError(testutils.RestartServer())
		s.Assert().NoError(testutils.ObtainAdminAccessToken())
	}()
	s.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		"http_connection": map[string]interface{}{"store": "declarative"},
	}))
	s.Require().NoError(testutils.RestartServer())
	s.Require().NoError(testutils.ObtainAdminAccessToken())

	res := s.post(httpConnRequest{Name: "http-conn-declarative-store", URL: "https://store.ext.example.com/hook"})

	s.assertError(res, http.StatusBadRequest, httpConnDeclarativeCreateCode, "Create with a declarative store")
	s.Equal(httpConnDeclarativeName, s.get(httpConnDeclarativeID).Name,
		"The declarative store should still serve the declared connection")
}
