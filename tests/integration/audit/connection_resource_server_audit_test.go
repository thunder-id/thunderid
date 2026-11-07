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
	auditIDPClientID     = "audit-idp-client-id"
	auditIDPClientSecret = "AuditIdentityProviderSecret0123456789"
	auditPDPToken        = "AuditPolicyDecisionPointToken0123456789"
	auditHTTPPassword    = "AuditHTTPConnectionPassword0123456789"
	auditHTTPHeaderValue = "audit-http-tenant-header"
	auditRSIdentifier    = "https://audit-orders.example.com/api"
)

// Creating, updating and deleting a connection of each kind the connection API serves writes one line
// each, with the admin as actor and the connection as target and subject. No client secret, credential,
// header, endpoint or name of the connection appears in any line.
func (ts *AuditLogTestSuite) TestAConnectionLifecycleWritesOneLinePerChange() {
	ouID := ts.createOrganizationUnit("conn-lifecycle")
	adminID := ts.adminUserID()
	cases := []struct {
		kind    string
		path    string
		body    func(name string) map[string]interface{}
		secrets []string
	}{
		{
			kind: "identity provider",
			path: "/connections/google",
			body: func(name string) map[string]interface{} {
				return map[string]interface{}{"name": name, "ouId": ouID, "clientId": auditIDPClientID,
					"clientSecret": auditIDPClientSecret, "redirectUri": "https://localhost:3000/google/callback"}
			},
			secrets: []string{auditIDPClientSecret, auditIDPClientID},
		},
		{
			kind: "AuthZEN PDP",
			path: "/connections/authzen-pdp",
			body: func(name string) map[string]interface{} {
				return map[string]interface{}{"name": name, "ouId": ouID,
					"endpoint": "https://audit-pdp.example.com/access/v1/evaluation",
					"authentication": map[string]interface{}{"type": "bearer",
						"properties": map[string]string{"token": auditPDPToken}}}
			},
			secrets: []string{auditPDPToken, "audit-pdp.example.com"},
		},
		{
			kind: "HTTP endpoint",
			path: "/connections/http",
			body: func(name string) map[string]interface{} {
				return map[string]interface{}{"name": name, "ouId": ouID,
					"url":     "https://audit-http.example.com/evaluate",
					"headers": []map[string]string{{"name": "X-Tenant", "value": auditHTTPHeaderValue}},
					"authentication": map[string]interface{}{"type": "basic",
						"properties": map[string]string{"username": "audit-user", "password": auditHTTPPassword}}}
			},
			secrets: []string{auditHTTPPassword, auditHTTPHeaderValue, "audit-http.example.com"},
		},
	}
	for _, tc := range cases {
		ts.Run(tc.kind, func() {
			ts.Require().NoError(ts.reader.Reset())
			name := ts.prefix + "conn-" + strings.ReplaceAll(strings.ToLower(tc.kind), " ", "-")
			created, id := ts.create(tc.path, tc.body(name))
			updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, tc.path+"/"+id,
				ts.jsonBody(tc.body(name+"-renamed")))
			ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
			deleted := ts.send(http.MethodDelete, tc.path+"/"+id, "")
			ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

			connectionRef := testutils.AuditRef{Type: "connection", ID: id}
			lines := ts.linesOf(created, updated, deleted)
			for i, event := range []string{"connection.create", "connection.update", "connection.delete"} {
				line := lines[i]
				ts.Equal(event, line.Event)
				ts.Equal(strings.TrimPrefix(event, "connection."), line.Action)
				ts.Equal(connectionRef, line.Target, event)
				ts.Equal(connectionRef, line.Subject, event)
				ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, event)
				ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, event)
				for _, secret := range tc.secrets {
					ts.NotContains(line.Raw, secret, "no secret or property of the connection is recorded")
				}
				ts.NotContains(line.Raw, name, "the name of the connection is not recorded")
			}
		})
	}
}

// Creating, updating and deleting a resource server, a resource and an action of each, through the
// management API, writes one line each with the thing changed as target and subject. No name,
// identifier, handle or permission appears in any line.
func (ts *AuditLogTestSuite) TestAResourceServerLifecycleWritesOneLinePerChange() {
	ouID := ts.createOrganizationUnit("rs-lifecycle")
	adminID := ts.adminUserID()
	rsName := ts.prefix + "orders"

	rsCreated, rsID := ts.create("/resource-servers",
		map[string]interface{}{"name": rsName, "identifier": auditRSIdentifier + "/" + ts.prefix, "ouId": ouID})
	rsPath := "/resource-servers/" + rsID
	resCreated, resID := ts.create(rsPath+"/resources",
		map[string]interface{}{"name": "Audit Invoices", "handle": "auditinvoices"})
	resPath := rsPath + "/resources/" + resID
	resActionCreated, resActionID := ts.create(resPath+"/actions",
		map[string]interface{}{"name": "Audit Approve", "handle": "auditapprove"})
	rsActionCreated, rsActionID := ts.create(rsPath+"/actions",
		map[string]interface{}{"name": "Audit Export", "handle": "auditexport"})

	rsUpdated := ts.put(rsPath, map[string]interface{}{"name": rsName + "-renamed", "ouId": ouID})
	resUpdated := ts.put(resPath, map[string]interface{}{"name": "Audit Invoices Renamed"})
	resActionUpdated := ts.put(resPath+"/actions/"+resActionID, map[string]interface{}{"name": "Audit Approve 2"})
	rsActionUpdated := ts.put(rsPath+"/actions/"+rsActionID, map[string]interface{}{"name": "Audit Export 2"})

	resActionDeleted := ts.deleted(resPath + "/actions/" + resActionID)
	resDeleted := ts.deleted(resPath)
	rsActionDeleted := ts.deleted(rsPath + "/actions/" + rsActionID)
	rsDeleted := ts.deleted(rsPath)

	lines := ts.linesOf(rsCreated, resCreated, resActionCreated, rsActionCreated,
		rsUpdated, resUpdated, resActionUpdated, rsActionUpdated,
		resActionDeleted, resDeleted, rsActionDeleted, rsDeleted)
	rs := testutils.AuditRef{Type: "resource_server", ID: rsID}
	res := testutils.AuditRef{Type: "resource", ID: resID}
	resAction := testutils.AuditRef{Type: "action", ID: resActionID}
	rsAction := testutils.AuditRef{Type: "action", ID: rsActionID}
	expected := []struct {
		event  string
		target testutils.AuditRef
	}{
		{"resource_server.create", rs},
		{"resource_server.resource.create", res},
		{"resource_server.action.create", resAction},
		{"resource_server.action.create", rsAction},
		{"resource_server.update", rs},
		{"resource_server.resource.update", res},
		{"resource_server.action.update", resAction},
		{"resource_server.action.update", rsAction},
		{"resource_server.action.delete", resAction},
		{"resource_server.resource.delete", res},
		{"resource_server.action.delete", rsAction},
		{"resource_server.delete", rs},
	}
	for i, want := range expected {
		line := lines[i]
		ts.Equal(want.event, line.Event, "line %d", i)
		ts.Equal(want.event[strings.LastIndex(want.event, ".")+1:], line.Action, want.event)
		ts.Equal(want.target, line.Target, want.event)
		ts.Equal(want.target, line.Subject, want.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, want.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, want.event)
		for _, value := range []string{rsName, auditRSIdentifier, "Audit Invoices", "auditinvoices",
			"auditapprove", "auditexport", "Audit Approve", "Audit Export"} {
			ts.NotContains(line.Raw, value, "no name, identifier, handle or permission is recorded")
		}
	}
}

// Every attempt the connection or resource service refuses writes one failed line with the reason
// from the service's error code and the target only: a create names no ID, and an update or delete
// names the ID the caller sent.
func (ts *AuditLogTestSuite) TestEveryRefusedConnectionAndResourceServerChangeIsRecordedAsFailed() {
	ouID := ts.createOrganizationUnit("conn-rs-refused")
	missing := unusedID()
	idpName := ts.prefix + "taken-idp"
	ts.create("/connections/google", map[string]interface{}{"name": idpName, "ouId": ouID,
		"clientId": auditIDPClientID, "clientSecret": auditIDPClientSecret,
		"redirectUri": "https://localhost:3000/google/callback"})
	_, rsID := ts.create("/resource-servers",
		map[string]interface{}{"name": ts.prefix + "refused", "identifier": auditRSIdentifier + "/refused/" +
			ts.prefix, "ouId": ouID})
	rsPath := "/resource-servers/" + rsID
	ts.create(rsPath+"/resources", map[string]interface{}{"name": "Audit Taken", "handle": "audittaken"})

	cases := []struct {
		name   string
		method string
		path   string
		body   map[string]interface{}
		status int
		target testutils.AuditRef
		reason string
	}{
		{"an identity provider whose name is taken", http.MethodPost, "/connections/google",
			map[string]interface{}{"name": idpName, "ouId": ouID, "clientId": auditIDPClientID,
				"clientSecret": auditIDPClientSecret, "redirectUri": "https://localhost:3000/google/callback"},
			http.StatusConflict, testutils.AuditRef{Type: "connection"}, "idp_1005"},
		{"an AuthZEN PDP that does not exist", http.MethodPut, "/connections/authzen-pdp/" + missing,
			map[string]interface{}{"name": ts.prefix + "missing-pdp", "ouId": ouID,
				"endpoint": "https://audit-pdp.example.com/access/v1/evaluation"},
			http.StatusNotFound, testutils.AuditRef{Type: "connection", ID: missing}, "con_1004"},
		{"an HTTP connection with no URL", http.MethodPost, "/connections/http",
			map[string]interface{}{"name": ts.prefix + "no-url", "ouId": ouID, "url": "ftp://audit.example.com"},
			http.StatusBadRequest, testutils.AuditRef{Type: "connection"}, "con_1018"},
		{"a resource server that does not exist", http.MethodPut, "/resource-servers/" + missing,
			map[string]interface{}{"name": ts.prefix + "missing", "ouId": ouID},
			http.StatusNotFound, testutils.AuditRef{Type: "resource_server", ID: missing}, "res_1003"},
		{"a resource server that still has resources", http.MethodDelete, rsPath, nil,
			http.StatusBadRequest, testutils.AuditRef{Type: "resource_server", ID: rsID}, "res_1006"},
		{"a resource whose handle is taken", http.MethodPost, rsPath + "/resources",
			map[string]interface{}{"name": "Audit Taken", "handle": "audittaken"},
			http.StatusConflict, testutils.AuditRef{Type: "resource"}, "res_1014"},
		{"an action that does not exist", http.MethodPut, rsPath + "/actions/" + missing,
			map[string]interface{}{"name": "Audit Missing"},
			http.StatusNotFound, testutils.AuditRef{Type: "action", ID: missing}, "res_1009"},
	}
	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		body := ""
		if tc.body != nil {
			body = ts.jsonBody(tc.body)
		}
		resp, raw := ts.exchange(testutils.GetHTTPClient(), tc.method, tc.path, body)
		ts.Require().Equal(tc.status, resp.StatusCode, "%s: %s", tc.name, string(raw))
		responses[i] = resp
	}

	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		ts.Equal(tc.target, line.Target, tc.name)
		ts.Equal(testutils.AuditRef{}, line.Subject, "%s: a failed attempt has no subject", tc.name)
		ts.Equal(adminRef, line.Actor, tc.name)
		ts.Equal(map[string]string{"result": "failed", "reason": tc.reason}, line.Outcome, tc.name)
		ts.NotContains(line.Raw, auditIDPClientSecret, "a client secret is never recorded")
		ts.NotContains(line.Raw, auditIDPClientID, "no property of a connection is recorded")
		ts.NotContains(line.Raw, ts.prefix, "no name is recorded")
	}
}

// A delete of something that does not exist writes no line, so a caller cannot record the deletion of
// an ID it made up. The resource service treats such a delete as done; the connection API answers it
// itself before any connection service is reached.
func (ts *AuditLogTestSuite) TestADeleteOfAMissingConnectionOrResourceServerWritesNoLine() {
	ouID := ts.createOrganizationUnit("conn-rs-missing")
	_, rsID := ts.create("/resource-servers",
		map[string]interface{}{"name": ts.prefix + "missing-children", "identifier": auditRSIdentifier +
			"/missing/" + ts.prefix, "ouId": ouID})
	missing := unusedID()
	cases := []struct {
		path   string
		status int
	}{
		{"/resource-servers/" + missing, http.StatusNoContent},
		{"/resource-servers/" + rsID + "/resources/" + missing, http.StatusNoContent},
		{"/resource-servers/" + rsID + "/actions/" + missing, http.StatusNoContent},
		{"/resource-servers/" + rsID + "/resources/" + missing + "/actions/" + missing, http.StatusNoContent},
		{"/connections/google/" + missing, http.StatusNotFound},
		{"/connections/authzen-pdp/" + missing, http.StatusNotFound},
	}
	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		responses[i] = ts.send(http.MethodDelete, tc.path, "")
		ts.Require().Equal(tc.status, responses[i].StatusCode, tc.path)
	}

	for i, written := range ts.linesPerCall(responses...) {
		ts.Empty(written, "%s: a delete that removed nothing writes no line", cases[i].path)
	}
}

// An import writes the event of the connection or resource server it writes and nothing else: the lookup
// that decides between update and create writes no line, so a create is not preceded by a failed update,
// and an update the service refuses writes its failed update alone.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfTheConnectionOrResourceServerItWrites() {
	ouID := ts.createOrganizationUnit("conn-rs-import")
	idpID := unusedID()
	ts.T().Cleanup(func() { ts.send(http.MethodDelete, "/connections/google/"+idpID, "") })
	idpDocument := func(name string) string {
		return fmt.Sprintf("resource_type: connection\nid: %s\ntype: google\nname: %s\nouId: %s\nclientId: %s\n"+
			"clientSecret: %s\nredirectUri: https://localhost:3000/callback\n",
			idpID, name, ouID, auditIDPClientID, auditIDPClientSecret)
	}
	takenIDPName := ts.prefix + "import-taken-idp"
	ts.create("/connections/google", map[string]interface{}{"name": takenIDPName, "ouId": ouID,
		"clientId": auditIDPClientID, "clientSecret": auditIDPClientSecret,
		"redirectUri": "https://localhost:3000/google/callback"})
	idpCreated := ts.importDocument(idpDocument(ts.prefix+"imported-idp"), "create", "")
	idpUpdated := ts.importDocument(idpDocument(ts.prefix+"imported-idp-renamed"), "update", "")
	idpRefused := ts.importDocument(idpDocument(takenIDPName), "update", "IDP-1005")

	identifier := auditRSIdentifier + "/import/" + ts.prefix
	_, rsID := ts.create("/resource-servers",
		map[string]interface{}{"name": ts.prefix + "imported", "identifier": identifier, "ouId": ouID})
	takenRSName := ts.prefix + "import-taken"
	ts.create("/resource-servers", map[string]interface{}{"name": takenRSName,
		"identifier": identifier + "/taken", "ouId": ouID})
	rsDocument := func(name string) string {
		return fmt.Sprintf("resource_type: resource_server\nid: %s\nname: %s\nidentifier: %s\nouId: %s\n",
			rsID, name, identifier, ouID)
	}
	rsUpdated := ts.importDocument(rsDocument(ts.prefix+"imported-renamed"), "update", "")
	rsRefused := ts.importDocument(rsDocument(takenRSName), "update", "RES-1004")

	idpRef := testutils.AuditRef{Type: "connection", ID: idpID}
	rsRef := testutils.AuditRef{Type: "resource_server", ID: rsID}
	succeeded := map[string]string{"result": "succeeded"}
	lines := ts.linesOf(idpCreated, idpUpdated, idpRefused, rsUpdated, rsRefused)
	for i, want := range []struct {
		event   string
		target  testutils.AuditRef
		subject testutils.AuditRef
		outcome map[string]string
	}{
		{"connection.create", idpRef, idpRef, succeeded},
		{"connection.update", idpRef, idpRef, succeeded},
		{"connection.update", idpRef, testutils.AuditRef{}, map[string]string{"result": "failed", "reason": "idp_1005"}},
		{"resource_server.update", rsRef, rsRef, succeeded},
		{"resource_server.update", rsRef, testutils.AuditRef{},
			map[string]string{"result": "failed", "reason": "res_1004"}},
	} {
		line := lines[i]
		ts.Equal(want.event, line.Event, "line %d", i)
		ts.Equal(want.target, line.Target, "line %d", i)
		ts.Equal(want.subject, line.Subject, "line %d", i)
		ts.Equal(want.outcome, line.Outcome, "line %d", i)
		ts.NotContains(line.Raw, auditIDPClientSecret, "a client secret is never recorded")
		ts.NotContains(line.Raw, ts.prefix, "no name or identifier is recorded")
	}
}

// create makes a management call that creates a resource at path, removed when the test ends unless the
// test deletes it, and returns the response and the new ID.
func (ts *AuditLogTestSuite) create(path string, body map[string]interface{}) (*http.Response, string) {
	ts.T().Helper()
	resp, raw := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, path, ts.jsonBody(body))
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(raw))
	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &created))
	ts.Require().NotEmpty(created.ID)
	if path == "/resource-servers" {
		ts.T().Cleanup(func() { _ = testutils.DeleteResourceServerWithChildren(created.ID) })
	} else {
		ts.T().Cleanup(func() { ts.send(http.MethodDelete, path+"/"+created.ID, "") })
	}
	return resp, created.ID
}

// put makes a management call that updates the resource at path and checks it succeeded.
func (ts *AuditLogTestSuite) put(path string, body map[string]interface{}) *http.Response {
	ts.T().Helper()
	resp, raw := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, path, ts.jsonBody(body))
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(raw))
	return resp
}

// deleted makes a management call that deletes the resource at path and checks it succeeded.
func (ts *AuditLogTestSuite) deleted(path string) *http.Response {
	ts.T().Helper()
	resp := ts.send(http.MethodDelete, path, "")
	ts.Require().Equal(http.StatusNoContent, resp.StatusCode, path)
	return resp
}

// importDocument imports content to the runtime with upsert on and checks the one document in it was
// written with operation, or refused with code when code is not empty.
func (ts *AuditLogTestSuite) importDocument(content, operation, code string) *http.Response {
	ts.T().Helper()
	request, err := json.Marshal(testutils.ImportRequest{
		Content: content,
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	ts.Require().NoError(err)
	resp, raw := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(raw))
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(raw, &result))
	ts.Require().Len(result.Results, 1, string(raw))
	ts.Require().Equal(operation, result.Results[0].Operation, string(raw))
	ts.Require().Equal(code, result.Results[0].Code, string(raw))
	return resp
}
