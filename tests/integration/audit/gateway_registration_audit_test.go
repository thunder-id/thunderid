// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// auditRegistrationKey is the key the registration tests hand a gateway. No line may carry it.
const auditRegistrationKey = "AuditGatewayRegistrationKey0123456789"

// Registering, renaming and removing a gateway writes one line each, with the admin as actor and the
// gateway as target and subject. Removing it again fails, since a gateway that is not registered answers
// as not found, and records the ID the caller sent with no subject. Neither the gateway's name, address
// nor key appears in any line.
func (ts *AuditLogTestSuite) TestAGatewayRegistrationLifecycleWritesOneLinePerChange() {
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	name, baseURL := ts.gatewayNameAndAddress("reg")

	created, gatewayID := ts.registerAuditGateway(map[string]any{
		"name": name, "baseUrl": baseURL, "key": auditRegistrationKey,
	})
	updated := ts.gatewayCall(http.MethodPut, "/gateways/"+gatewayID,
		ts.jsonBody(map[string]any{"name": name + "-renamed"}), http.StatusOK)
	deleted := ts.gatewayCall(http.MethodDelete, "/gateways/"+gatewayID, "", http.StatusNoContent)
	deletedAgain := ts.gatewayCall(http.MethodDelete, "/gateways/"+gatewayID, "", http.StatusNotFound)

	gatewayRef := testutils.AuditRef{Type: "gateway", ID: gatewayID}
	lines := ts.linesOf(created, updated, deleted, deletedAgain)
	for i, want := range []struct {
		event   string
		action  string
		subject testutils.AuditRef
		outcome map[string]string
	}{
		{"gateway.create", "create", gatewayRef, map[string]string{"result": "succeeded"}},
		{"gateway.update", "update", gatewayRef, map[string]string{"result": "succeeded"}},
		{"gateway.delete", "delete", gatewayRef, map[string]string{"result": "succeeded"}},
		{"gateway.delete", "delete", testutils.AuditRef{}, map[string]string{"result": "failed", "reason": "gtw_1002"}},
	} {
		line := lines[i]
		ts.Equal(want.event, line.Event, "line %d", i)
		ts.Equal(want.action, line.Action, "line %d", i)
		ts.Equal(gatewayRef, line.Target, "line %d", i)
		ts.Equal(want.subject, line.Subject, "line %d: only an attempt on a registered gateway names it", i)
		ts.Equal(adminRef, line.Actor, "line %d", i)
		ts.Equal(want.outcome, line.Outcome, "line %d", i)
		ts.Nil(line.Client, "line %d: a loopback peer is not recorded", i)
		ts.requireNothingOfTheGateway(line, name, baseURL)
	}
}

// Every refused registration, change and removal of a gateway records the gateway ID the caller sent, or
// none on a registration, the reason from the service's error code and no subject.
func (ts *AuditLogTestSuite) TestEveryFailedGatewayChangeIsRecorded() {
	name, baseURL := ts.gatewayNameAndAddress("fail")
	_, gatewayID := ts.registerAuditGateway(map[string]any{
		"name": name, "baseUrl": baseURL, "key": auditRegistrationKey,
	})
	missing := unusedID()

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		status int
		event  string
		target string
		reason string
	}{
		{"a registration without a name", http.MethodPost, "/gateways",
			ts.jsonBody(map[string]any{"baseUrl": baseURL + "1"}), http.StatusBadRequest, "gateway.create", "",
			"gtw_1003"},
		{"a registration at an address that is not https", http.MethodPost, "/gateways",
			ts.jsonBody(map[string]any{"name": name + "-http", "baseUrl": "http://audit-gw.example.com"}),
			http.StatusBadRequest, "gateway.create", "", "gtw_1009"},
		{"a registration at an address already registered", http.MethodPost, "/gateways",
			ts.jsonBody(map[string]any{"name": name + "-twin", "baseUrl": baseURL}), http.StatusConflict,
			"gateway.create", "", "gtw_1008"},
		{"a change of a gateway that is not registered", http.MethodPut, "/gateways/" + missing,
			ts.jsonBody(map[string]any{"name": name}), http.StatusNotFound, "gateway.update", missing, "gtw_1002"},
		{"a change that names nothing to change", http.MethodPut, "/gateways/" + gatewayID, "{}",
			http.StatusBadRequest, "gateway.update", gatewayID, "gtw_1011"},
		{"a removal of a gateway that is not registered", http.MethodDelete, "/gateways/" + missing, "",
			http.StatusNotFound, "gateway.delete", missing, "gtw_1002"},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		responses[i] = ts.gatewayCall(tc.method, tc.path, tc.body, tc.status)
	}
	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		ts.Equal(tc.event, line.Event, tc.name)
		ts.Equal(testutils.AuditRef{Type: "gateway", ID: tc.target}, line.Target, tc.name)
		ts.Equal(testutils.AuditRef{}, line.Subject, "%s: a failed attempt may name no gateway", tc.name)
		ts.Equal(map[string]string{"result": "failed", "reason": tc.reason}, line.Outcome, tc.name)
		ts.requireNothingOfTheGateway(line, name, baseURL)
	}
}

// An import registers a gateway it finds no gateway of that name for, and changes the one it finds. Each
// writes the gateway's own line before the import's, and the registration is not preceded by a failed
// change: the import looks the gateway up by name rather than attempting a change first.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfTheGatewayItWrites() {
	name, baseURL := ts.gatewayNameAndAddress("imp")
	content := fmt.Sprintf("resource_type: gateway\nname: %s\nbaseUrl: %s\nkey: %s\n", name, baseURL,
		auditRegistrationKey)

	createdByImport, createdResponse := ts.importUpsert(content, 1)
	created := createdResponse.Results[0]
	ts.Require().Equal("success", created.Status, created.Message)
	ts.Require().Equal("create", created.Operation, created.Message)
	gatewayID := created.ResourceID
	ts.Require().NotEmpty(gatewayID)
	ts.T().Cleanup(func() { ts.send(http.MethodDelete, "/gateways/"+gatewayID, "") })
	updatedByImport, updatedResponse := ts.importUpsert(content, 1)
	updated := updatedResponse.Results[0]
	ts.Require().Equal("success", updated.Status, updated.Message)
	ts.Require().Equal("update", updated.Operation, updated.Message)

	succeeded := map[string]string{"result": "succeeded"}
	gatewayRef := testutils.AuditRef{Type: "gateway", ID: gatewayID}
	for i, line := range []testutils.AuditLine{
		ts.resourceLineOfImport(createdByImport, succeeded),
		ts.resourceLineOfImport(updatedByImport, succeeded),
	} {
		event := []string{"gateway.create", "gateway.update"}[i]
		ts.Equal(event, line.Event)
		ts.Equal(gatewayRef, line.Target, event)
		ts.Equal(gatewayRef, line.Subject, event)
		ts.Equal(succeeded, line.Outcome, event)
		ts.requireNothingOfTheGateway(line, name, baseURL)
	}
}

// gatewayNameAndAddress returns a gateway name and address no other registration of this run has.
func (ts *AuditLogTestSuite) gatewayNameAndAddress(kind string) (string, string) {
	name := ts.prefix + "gateway-" + kind
	return name, "https://" + name + ".example.com"
}

// requireNothingOfTheGateway checks that line carries neither the name, the address nor the key of a
// gateway.
func (ts *AuditLogTestSuite) requireNothingOfTheGateway(line testutils.AuditLine, name, baseURL string) {
	ts.T().Helper()
	ts.NotContains(line.Raw, name, "%s: the gateway's name is not recorded", line.Event)
	ts.NotContains(line.Raw, baseURL, "%s: the gateway's address is not recorded", line.Event)
	ts.NotContains(line.Raw, "example.com", "%s: no part of the gateway's address is recorded", line.Event)
	ts.NotContains(line.Raw, auditRegistrationKey, "%s: the gateway's key is never recorded", line.Event)
}
