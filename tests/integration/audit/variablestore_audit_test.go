// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// The markers are written as values and descriptions so a line can be searched for any part of them.
	storeValueMarker       = "AuditStoreValueMarkerQ7"
	storeDescriptionMarker = "AuditStoreDescriptionMarkerZ4"
)

// storeKind is one of the two name-keyed collections of the variable store.
type storeKind struct {
	path       string
	targetType string
}

var storeKinds = []storeKind{
	{path: "/variables", targetType: "variable"},
	{path: "/secrets", targetType: "secret"},
}

// Creating a variable or secret, replacing it, storing one under an unused name with PUT and deleting it
// write one line each, with the admin as actor and the name as target and subject. A PUT that created
// the name is recorded as the update the caller attempted. No value and no description appears in any
// line.
func (ts *AuditLogTestSuite) TestAVariableAndSecretLifecycleWritesOneLinePerChange() {
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	for _, kind := range storeKinds {
		name := storeName("LIFE")
		putName := storeName("PUT")

		created := ts.storeCall(http.MethodPost, kind.path, ts.storeBody(name, "created"), http.StatusCreated)
		ts.T().Cleanup(func() { ts.send(http.MethodDelete, kind.path+"/"+name, "") })
		replaced := ts.storeCall(http.MethodPut, kind.path+"/"+name, ts.storeBody("", "replaced"), http.StatusOK)
		putCreated := ts.storeCall(http.MethodPut, kind.path+"/"+putName, ts.storeBody("", "put"),
			http.StatusCreated)
		ts.T().Cleanup(func() { ts.send(http.MethodDelete, kind.path+"/"+putName, "") })
		deleted := ts.storeCall(http.MethodDelete, kind.path+"/"+name, "", http.StatusNoContent)

		lines := ts.linesOf(created, replaced, putCreated, deleted)
		for i, tc := range []struct {
			event  string
			action string
			name   string
		}{
			{kind.targetType + ".create", "create", name},
			{kind.targetType + ".update", "update", name},
			{kind.targetType + ".update", "update", putName},
			{kind.targetType + ".delete", "delete", name},
		} {
			line := lines[i]
			ref := testutils.AuditRef{Type: kind.targetType, ID: tc.name}
			ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
			ts.Equal(lineKeys, ts.keys(line.Raw), tc.event)
			ts.Equal(tc.event, line.Event)
			ts.Equal(tc.action, line.Action, tc.event)
			ts.Equal(ref, line.Target, "%s: a %s is identified by its name", tc.event, kind.targetType)
			ts.Equal(ref, line.Subject, "%s: a succeeded change has its target as subject", tc.event)
			ts.Equal(adminRef, line.Actor, "%s: the admin made the change", tc.event)
			ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
			ts.Nil(line.Client, "%s: a loopback peer is not recorded", tc.event)
			ts.requireNoStoreContent(line)
		}
	}
}

// Every refused create, update and delete of a variable or secret is recorded as failed with the
// reason the store gave and the name the caller sent as target, since the name is the identifier.
// Nothing the caller sent as a value or description is recorded.
func (ts *AuditLogTestSuite) TestEveryRefusedVariableAndSecretChangeIsRecordedAsFailed() {
	for _, kind := range storeKinds {
		existing := storeName("REFUSED")
		refCreate := storeName("REFCREATE")
		ts.storeCall(http.MethodPost, kind.path, ts.storeBody(existing, "existing"), http.StatusCreated)
		ts.T().Cleanup(func() { ts.send(http.MethodDelete, kind.path+"/"+existing, "") })
		named := func(name string) testutils.AuditRef {
			return testutils.AuditRef{Type: kind.targetType, ID: name}
		}

		cases := []struct {
			name   string
			method string
			path   string
			body   string
			status int
			action string
			reason string
			target testutils.AuditRef
		}{
			{
				name: "a create under a name the store does not accept", method: http.MethodPost, path: kind.path,
				body: ts.storeBody("not-a-name", "invalid"), status: http.StatusBadRequest, action: "create",
				reason: "var_1002", target: named("not-a-name"),
			},
			{
				name: "a create under a name already used", method: http.MethodPost, path: kind.path,
				body: ts.storeBody(existing, "duplicate"), status: http.StatusConflict, action: "create",
				reason: "var_1009", target: named(existing),
			},
			{
				name: "a create whose value is a reference", method: http.MethodPost, path: kind.path,
				body:   ts.storeReferenceBody(refCreate),
				status: http.StatusBadRequest, action: "create", reason: "var_1011", target: named(refCreate),
			},
			{
				name: "an update whose value is a reference", method: http.MethodPut,
				path: kind.path + "/" + existing, body: ts.storeReferenceBody(""),
				status: http.StatusBadRequest, action: "update", reason: "var_1011", target: named(existing),
			},
			{
				name: "an update under a name the store does not accept", method: http.MethodPut,
				path: kind.path + "/not-a-name", body: ts.storeBody("", "invalid"),
				status: http.StatusBadRequest, action: "update", reason: "var_1002", target: named("not-a-name"),
			},
			{
				name: "a delete under a name the store does not accept", method: http.MethodDelete,
				path: kind.path + "/not-a-name", status: http.StatusBadRequest, action: "delete",
				reason: "var_1002", target: named("not-a-name"),
			},
		}
		responses := make([]*http.Response, 0, len(cases))
		for _, tc := range cases {
			responses = append(responses, ts.storeCall(tc.method, tc.path, tc.body, tc.status))
		}

		for i, line := range ts.linesOf(responses...) {
			tc := cases[i]
			ts.Equal(kind.targetType+"."+tc.action, line.Event, tc.name)
			ts.Equal(tc.action, line.Action, tc.name)
			ts.Equal(tc.target, line.Target, tc.name)
			ts.Equal(testutils.AuditRef{}, line.Subject, "%s: a failed attempt has no subject", tc.name)
			ts.Equal(map[string]string{"result": "failed", "reason": tc.reason}, line.Outcome, tc.name)
			ts.Nil(line.Client, "%s: a loopback peer is not recorded", tc.name)
			ts.requireNoStoreContent(line)
		}
	}
	// The empty value is refused for a secret only: a variable set to nothing is a valid variable.
	emptyName := storeName("EMPTY")
	refused := ts.storeCall(http.MethodPost, "/secrets", fmt.Sprintf(`{"name":%q,"value":""}`, emptyName),
		http.StatusBadRequest)
	line := ts.linesOf(refused)[0]
	ts.Equal("secret.create", line.Event)
	ts.Equal(testutils.AuditRef{Type: "secret", ID: emptyName}, line.Target,
		"a create that stored nothing still names the secret the caller asked for")
	ts.Equal(map[string]string{"result": "failed", "reason": "var_1003"}, line.Outcome)
}

// A refused create keeps the invalid name the caller sent, cut to the target ID cap and escaped, so a
// name written to look like JSON or a second line stays inside the target ID of one valid line.
func (ts *AuditLogTestSuite) TestARefusedCreateRecordsAnInvalidNameEscaped() {
	forged := `x","outcome":{"result":"succeeded"},"y":"` + "\n" + auditLinePrefix + `\ <b>`
	name := forged + strings.Repeat("é", 150)
	ts.Require().Greater(utf8.RuneCountInString(name), idCap, "the name is longer than the cap")
	responses := make([]*http.Response, 0, len(storeKinds))
	for _, kind := range storeKinds {
		body, err := json.Marshal(map[string]string{"name": name, "value": storeValueMarker})
		ts.Require().NoError(err)
		responses = append(responses, ts.storeCall(http.MethodPost, kind.path, string(body), http.StatusBadRequest))
	}

	// A failed line has every key of lineKeys but the subject.
	failedLineKeys := []string{
		"log_type", "schema_version", "id", "time", "event", "trace_id", "actor", "target", "action", "outcome",
	}
	for i, line := range ts.linesOf(responses...) {
		kind := storeKinds[i]
		ts.True(json.Valid([]byte(line.Raw)), "%s: the line is one JSON object: %s", kind.targetType, line.Raw)
		ts.Equal(failedLineKeys, ts.keys(line.Raw), "%s: the name adds no key to the line", kind.targetType)
		ts.Equal(testutils.AuditRef{}, line.Subject, "%s: a failed attempt has no subject", kind.targetType)
		ts.Equal(kind.targetType+".create", line.Event)
		ts.Equal(testutils.AuditRef{Type: kind.targetType, ID: string([]rune(name)[:idCap])},
			line.Target, "%s: the target ID is the name cut to the cap", kind.targetType)
		ts.Equal(map[string]string{"result": "failed", "reason": "var_1002"}, line.Outcome, kind.targetType)
		ts.requireNoStoreContent(line)
	}
}

// A delete of a variable or secret that is not stored still succeeds, but writes no line, so a caller
// cannot record the deletion of a name it made up.
func (ts *AuditLogTestSuite) TestADeleteOfAMissingVariableOrSecretWritesNoLine() {
	responses := make([]*http.Response, 0, len(storeKinds))
	for _, kind := range storeKinds {
		responses = append(responses,
			ts.storeCall(http.MethodDelete, kind.path+"/"+storeName("MISSING"), "", http.StatusNoContent))
	}

	for i, written := range ts.linesPerCall(responses...) {
		ts.Empty(written, "%s: a delete that removed nothing writes no line", storeKinds[i].targetType)
	}
}

// storeName returns a name no earlier run used, short enough to read in a line. A name is at most 255
// characters and may hold only letters, digits and underscores.
func storeName(name string) string {
	return fmt.Sprintf("AUDIT_%d_%s", time.Now().UnixNano(), name)
}

// storeBody is a create body when name is set and an update body otherwise, with the markers in the
// value and the description.
func (ts *AuditLogTestSuite) storeBody(name, value string) string {
	ts.T().Helper()
	request := map[string]any{
		"value":       storeValueMarker + "-" + value,
		"description": storeDescriptionMarker + " " + value,
	}
	if name != "" {
		request["name"] = name
	}
	return ts.jsonBody(request)
}

// storeReferenceBody is a create or update body whose value is a reference, which the store refuses.
func (ts *AuditLogTestSuite) storeReferenceBody(name string) string {
	ts.T().Helper()
	request := map[string]any{"value": "var:AUDIT_REFERENCE", "description": storeDescriptionMarker}
	if name != "" {
		request["name"] = name
	}
	return ts.jsonBody(request)
}

// storeCall makes a variable store call as the admin and requires the status it should get.
func (ts *AuditLogTestSuite) storeCall(method, path, body string, status int) *http.Response {
	ts.T().Helper()
	resp, read := ts.exchange(testutils.GetHTTPClient(), method, path, body)
	ts.Require().Equal(status, resp.StatusCode, "%s %s: %s", method, path, string(read))
	return resp
}

// requireNoStoreContent checks that no part of a value or description is in the line.
func (ts *AuditLogTestSuite) requireNoStoreContent(line testutils.AuditLine) {
	ts.NotContains(line.Raw, storeValueMarker, "%s: a value is never recorded", line.Event)
	ts.NotContains(line.Raw, "AUDIT_REFERENCE", "%s: a refused value is never recorded", line.Event)
	ts.NotContains(line.Raw, storeDescriptionMarker, "%s: a description is never recorded", line.Event)
}
