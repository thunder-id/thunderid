// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	auditAppRedirectURI = "https://audit-app.example.com/callback"
	auditAppSecret      = "AuditApplicationSecret0123456789"
)

// Creating, updating and deleting an application through the management API writes one line each, with
// the admin as actor and the application as target and subject. No part of the application, its client
// secret, client ID, redirect URI or name, appears in any line.
func (ts *AuditLogTestSuite) TestAnApplicationLifecycleWritesOneLinePerChange() {
	ouID := ts.createOrganizationUnit("app-lifecycle")
	authFlowID := ts.authFlowID()
	adminID := ts.adminUserID()
	name := ts.prefix + "lifecycle-app"
	clientID := ts.prefix + "lifecycle-client"

	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/applications",
		ts.applicationBody(name, ouID, authFlowID, clientID))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	var app struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &app))
	ts.Require().NotEmpty(app.ID)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(app.ID) })
	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/applications/"+app.ID,
		ts.applicationBody(name+"-renamed", ouID, authFlowID, clientID))
	ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
	deleted := ts.send(http.MethodDelete, "/applications/"+app.ID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	appRef := testutils.AuditRef{Type: "application", ID: app.ID}
	lines := ts.linesOf(created, updated, deleted)
	for i, tc := range []struct {
		event  string
		action string
	}{
		{"application.create", "create"},
		{"application.update", "update"},
		{"application.delete", "delete"},
	} {
		line := lines[i]
		ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
		ts.Equal(lineKeys, ts.keys(line.Raw), tc.event)
		ts.Equal(tc.event, line.Event)
		ts.Equal(tc.action, line.Action)
		ts.Equal(appRef, line.Target, tc.event)
		ts.Equal(appRef, line.Subject, tc.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
		ts.Contains([]string{"127.0.0.1", "::1"}, line.Client["address"])
		ts.NotContains(line.Raw, auditAppSecret, "a client secret is never recorded")
		ts.NotContains(line.Raw, "AuditApplicationSecret", "no part of a client secret is recorded")
		ts.NotContains(line.Raw, clientID, "the client ID is not recorded")
		ts.NotContains(line.Raw, auditAppRedirectURI, "no OAuth configuration is recorded")
		ts.NotContains(line.Raw, name, "no attribute of the application is recorded")
	}
}

// An update of an application that does not exist reaches the application service, which records the
// failed attempt.
func (ts *AuditLogTestSuite) TestAnUpdateOfAMissingApplicationIsRecordedAsFailed() {
	missing := unusedID()
	ouID := ts.createOrganizationUnit("app-missing")

	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/applications/"+missing,
		ts.applicationBody(ts.prefix+"missing-app", ouID, ts.authFlowID(), ts.prefix+"missing-client"))
	ts.Require().Equal(http.StatusNotFound, updated.StatusCode, string(body))

	line := ts.linesOf(updated)[0]
	ts.Equal("application.update", line.Event)
	ts.Equal(testutils.AuditRef{Type: "application", ID: missing}, line.Target)
	ts.Equal(testutils.AuditRef{}, line.Subject, "a failed attempt may name no application, so it has no subject")
	ts.Equal(map[string]string{"result": "failed", "reason": "app_1001"}, line.Outcome)
	ts.NotContains(line.Raw, auditAppSecret, "a client secret is never recorded")
}

// A delete of an application that does not exist still succeeds, but writes no line, so a caller cannot
// record the deletion of an ID it made up.
func (ts *AuditLogTestSuite) TestADeleteOfAMissingApplicationWritesNoLine() {
	missing := unusedID()

	deleted := ts.send(http.MethodDelete, "/applications/"+missing, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	ts.Empty(ts.linesPerCall(deleted)[0], "a delete that removed nothing writes no line")
}

// An import that creates an application writes the application service's event for it and nothing
// else: the lookup that decides between update and create writes no line.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfTheApplicationItCreates() {
	ouID := ts.createOrganizationUnit("app-import")
	appID := unusedID()
	request, err := json.Marshal(testutils.ImportRequest{
		Content: fmt.Sprintf("resource_type: application\nid: %s\nname: %s\ntype: fullstack\nouId: %s\n"+
			"authFlowId: %s\n", appID, ts.prefix+"imported-app", ouID, ts.authFlowID()),
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	ts.Require().NoError(err)

	imported, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Equal(1, result.Summary.Imported, string(body))

	line := ts.linesOf(imported)[0]
	appRef := testutils.AuditRef{Type: "application", ID: appID}
	ts.Equal("application.create", line.Event)
	ts.Equal(appRef, line.Target)
	ts.Equal(appRef, line.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
}

// An upsert import of an existing application that the application service refuses writes the service's
// failed update and nothing else, and leaves the application as it was. The line names no subject and
// carries none of the imported document.
func (ts *AuditLogTestSuite) TestAnImportOfARefusedUpdateWritesTheFailedUpdate() {
	ouID := ts.createOrganizationUnit("app-import-refused")
	name := ts.prefix + "import-refused-app"
	clientID := ts.prefix + "import-refused-client"
	authFlowID := ts.authFlowID()
	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/applications",
		ts.applicationBody(name, ouID, authFlowID, clientID))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	var app struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &app))
	ts.Require().NotEmpty(app.ID)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(app.ID) })

	request, err := json.Marshal(testutils.ImportRequest{
		Content: fmt.Sprintf("resource_type: application\nid: %s\nname: %s\ntype: fullstack\nouId: %s\n"+
			"authFlowId: %s\ninboundAuthConfig:\n  - type: oauth2\n    config:\n      clientId: %s\n"+
			"      clientSecret: %s\n      redirectUris:\n        - %s\n      grantTypes:\n"+
			"        - authorization_code\n      responseTypes:\n        - code\n"+
			"      tokenEndpointAuthMethod: client_secret_basic\n",
			app.ID, name+"-renamed", ouID, unusedID(), clientID, auditAppSecret, auditAppRedirectURI),
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	ts.Require().NoError(err)

	imported, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Len(result.Results, 1, string(body))
	ts.Equal("failed", result.Results[0].Status, "an unknown authentication flow must refuse the update")
	ts.Equal("update", result.Results[0].Operation, "the application exists, so the import is an update")
	ts.Equal("APP-1007", result.Results[0].Code, string(body))
	stored, body := ts.exchange(testutils.GetHTTPClient(), http.MethodGet, "/applications/"+app.ID, "")
	ts.Require().Equal(http.StatusOK, stored.StatusCode, string(body))
	var unchanged testutils.Application
	ts.Require().NoError(json.Unmarshal(body, &unchanged))
	ts.Equal(name, unchanged.Name, "a refused update must not rename the application")
	ts.Equal(authFlowID, unchanged.AuthFlowID, "a refused update must not change the authentication flow")

	line := ts.linesOf(imported)[0]
	ts.Equal("application.update", line.Event)
	ts.Equal("update", line.Action)
	ts.Equal(testutils.AuditRef{Type: "application", ID: app.ID}, line.Target)
	ts.Equal(testutils.AuditRef{}, line.Subject, "a failed update has no subject")
	ts.Equal(map[string]string{"result": "failed", "reason": "app_1007"}, line.Outcome)
	ts.NotContains(line.Raw, "AuditApplicationSecret", "no part of a client secret is recorded")
	ts.NotContains(line.Raw, clientID, "the client ID is not recorded")
	ts.NotContains(line.Raw, auditAppRedirectURI, "no OAuth configuration is recorded")
	ts.NotContains(line.Raw, name, "no attribute of the application is recorded")
}

// applicationBody is the request body of an application with a confidential OAuth client.
func (ts *AuditLogTestSuite) applicationBody(name, ouID, authFlowID, clientID string) string {
	ts.T().Helper()
	body, err := json.Marshal(map[string]interface{}{
		"name":       name,
		"ouId":       ouID,
		"type":       "fullstack",
		"authFlowId": authFlowID,
		"inboundAuthConfig": []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                clientID,
				"clientSecret":            auditAppSecret,
				"redirectUris":            []string{auditAppRedirectURI},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err)
	return string(body)
}

// authFlowID returns the ID of the default authentication flow.
func (ts *AuditLogTestSuite) authFlowID() string {
	ts.T().Helper()
	flowID, err := testutils.GetFlowIDByHandle("default-flow", "AUTHENTICATION")
	ts.Require().NoError(err)
	return flowID
}
