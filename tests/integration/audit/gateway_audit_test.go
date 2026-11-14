// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// auditGatewayKey is the management API key the integration server trusts. Registering the server as a
// gateway of itself with it lets a version be applied without a second server.
const auditGatewayKey = "integration-management-api-key"

// Applying versions to a gateway and reverting it writes one line each, with the admin as actor and the
// gateway as target and subject. Neither the gateway's key, address or certificate nor the version
// applied appears in any line. A selection that names nothing keeps what the server holds as it is.
func (ts *AuditLogTestSuite) TestApplyAndRevertWriteOneLineEach() {
	gatewayID, certificate := ts.registerSelfAsGateway()
	adminID := ts.adminUserID()
	first := ts.captureVersion()
	ts.createOrganizationUnit("gateway-versioned")
	second := ts.captureVersion()
	ts.Require().NotEqual(first, second)

	appliedFirst := ts.gatewayCall(http.MethodPost, "/gateways/"+gatewayID+"/apply",
		fmt.Sprintf(`{"version":%q,"selection":[]}`, first), http.StatusOK)
	appliedSecond := ts.gatewayCall(http.MethodPost, "/gateways/"+gatewayID+"/apply",
		fmt.Sprintf(`{"version":%q,"selection":[]}`, second), http.StatusOK)
	reverted := ts.gatewayCall(http.MethodPost, "/gateways/"+gatewayID+"/revert", "", http.StatusOK)

	gatewayRef := testutils.AuditRef{Type: "gateway", ID: gatewayID}
	lines := ts.linesOf(appliedFirst, appliedSecond, reverted)
	for i, tc := range []struct {
		event  string
		action string
	}{
		{"gateway.apply", "apply"},
		{"gateway.apply", "apply"},
		{"gateway.revert", "revert"},
	} {
		line := lines[i]
		ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
		ts.Equal(lineKeys, ts.keys(line.Raw), tc.event)
		ts.Equal(tc.event, line.Event)
		ts.Equal(tc.action, line.Action)
		ts.Equal(gatewayRef, line.Target, tc.event)
		ts.Equal(gatewayRef, line.Subject, tc.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
		ts.Nil(line.Client, "%s: a loopback peer is not recorded", tc.event)
		ts.NotContains(line.Raw, auditGatewayKey, "the gateway's key is never recorded")
		ts.NotContains(line.Raw, testutils.TestServerURL, "the gateway's address is not recorded")
		ts.NotContains(line.Raw, "BEGIN CERTIFICATE", "the gateway's certificate is not recorded")
		ts.NotContains(line.Raw, certificate[30:60], "no part of the certificate is recorded")
		ts.NotContains(line.Raw, first, "the version applied is not recorded")
		ts.NotContains(line.Raw, second, "the version applied is not recorded")
	}
}

// Every apply and revert that reaches the gateway service records its outcome: a failed attempt keeps
// the gateway ID the caller sent and has no subject. A dry run changes nothing and writes no line.
func (ts *AuditLogTestSuite) TestEveryApplyAndRevertPathRecordsItsOutcome() {
	gatewayID, _ := ts.registerSelfAsGateway()
	ts.captureVersion()
	missing := unusedID()

	cases := []struct {
		name   string
		path   string
		body   string
		status int
		event  string
		target string
		// outcome is the line's outcome; nil when the call writes no line.
		outcome map[string]string
	}{
		{
			name: "an apply to a gateway that does not exist", path: "/gateways/" + missing + "/apply",
			status: http.StatusNotFound, event: "gateway.apply", target: missing,
			outcome: map[string]string{"result": "failed", "reason": "gtw_1002"},
		},
		{
			name: "an apply whose origin is too long", path: "/gateways/" + gatewayID + "/apply",
			body:   fmt.Sprintf(`{"origin":%q}`, strings.Repeat("o", 256)),
			status: http.StatusBadRequest, event: "gateway.apply", target: gatewayID,
			outcome: map[string]string{"result": "failed", "reason": "gtw_1035"},
		},
		{
			name: "an apply scoped to a project the server does not have", path: "/gateways/" + gatewayID + "/apply",
			body: `{"project":"no-such-project","selection":[]}`, status: http.StatusNotFound,
			event: "gateway.apply", target: gatewayID,
			outcome: map[string]string{"result": "failed", "reason": "gtw_1038"},
		},
		{
			name: "a revert of a gateway that does not exist", path: "/gateways/" + missing + "/revert",
			status: http.StatusNotFound, event: "gateway.revert", target: missing,
			outcome: map[string]string{"result": "failed", "reason": "gtw_1002"},
		},
		{
			name: "a revert with nothing to return to", path: "/gateways/" + gatewayID + "/revert",
			status: http.StatusBadRequest, event: "gateway.revert", target: gatewayID,
			outcome: map[string]string{"result": "failed", "reason": "gtw_1016"},
		},
		{
			name: "a revert of a project the server does not have", path: "/gateways/" + gatewayID + "/revert",
			body: `{"project":"no-such-project"}`, status: http.StatusNotFound, event: "gateway.revert",
			target: gatewayID, outcome: map[string]string{"result": "failed", "reason": "gtw_1038"},
		},
		{
			name: "a dry run of an apply", path: "/gateways/" + gatewayID + "/apply",
			body: `{"dryRun":true,"selection":[]}`, status: http.StatusOK,
		},
		{
			name: "a dry run of a revert", path: "/gateways/" + gatewayID + "/revert",
			body: `{"dryRun":true}`, status: http.StatusBadRequest,
		},
	}

	responses := make([]*http.Response, 0, len(cases))
	for _, tc := range cases {
		responses = append(responses, ts.gatewayCall(http.MethodPost, tc.path, tc.body, tc.status))
	}
	written := ts.linesPerCall(responses...)
	for i, tc := range cases {
		ts.Run(tc.name, func() {
			if tc.outcome == nil {
				ts.Empty(written[i], "a dry run wrote a line")
				return
			}
			ts.Require().Len(written[i], 1)
			line := written[i][0]
			ts.Equal(tc.event, line.Event)
			ts.Equal(testutils.AuditRef{Type: "gateway", ID: tc.target}, line.Target)
			ts.Equal(testutils.AuditRef{}, line.Subject, "a failed attempt may name no gateway, so it has no subject")
			ts.Equal(tc.outcome, line.Outcome)
			ts.NotContains(line.Raw, "no-such-project", "the request's body is not recorded")
			ts.NotContains(line.Raw, auditGatewayKey, "the gateway's key is never recorded")
		})
	}
}

// A capture that stores a new version writes one line naming the version by its hash; a capture of a
// configuration a version already holds stores nothing and writes no line. Neither the name nor the note
// the caller gave the version appears in the line.
func (ts *AuditLogTestSuite) TestACaptureWritesALineOnlyWhenItStoresAVersion() {
	// A unit of this run's own makes the configuration one no earlier version holds.
	ts.createOrganizationUnit("capture")
	versionName := ts.prefix + "audit-version-name"
	versionNote := "Audit capture note " + ts.prefix
	captureBody := ts.jsonBody(map[string]any{"name": versionName, "note": versionNote})

	captured, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/configuration-versions",
		captureBody)
	ts.Require().Equal(http.StatusCreated, captured.StatusCode, string(body))
	var version struct {
		Version string `json:"version"`
	}
	ts.Require().NoError(json.Unmarshal(body, &version))
	ts.Require().NotEmpty(version.Version)
	unchanged := ts.gatewayCall(http.MethodPost, "/configuration-versions", captureBody, http.StatusOK)

	written := ts.linesPerCall(captured, unchanged)
	ts.Require().Len(written[0], 1, "a capture that stored a version wrote one line")
	ts.Empty(written[1], "a capture of a configuration already held writes no line")
	line := written[0][0]
	versionRef := testutils.AuditRef{Type: "configuration_version", ID: version.Version}
	ts.Equal(lineKeys, ts.keys(line.Raw))
	ts.Equal("configuration_version.create", line.Event)
	ts.Equal("create", line.Action)
	ts.Equal(versionRef, line.Target)
	ts.Equal(versionRef, line.Subject)
	ts.Equal(testutils.AuditRef{Type: "user", ID: ts.adminUserID()}, line.Actor)
	ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
	ts.Nil(line.Client, "a loopback peer is not recorded")
	ts.NotContains(line.Raw, versionName, "the name of a version is not recorded")
	ts.NotContains(line.Raw, versionNote, "the note of a version is not recorded")
	ts.NotContains(line.Raw, ts.prefix, "nothing of the configuration is recorded")
}

// registerSelfAsGateway registers this server as a gateway of itself, removed when the test ends, and
// returns its ID and the certificate it is trusted by.
func (ts *AuditLogTestSuite) registerSelfAsGateway() (string, string) {
	ts.T().Helper()
	certificate, err := os.ReadFile(filepath.Join(testutils.GetExtractedProductHome(), "config", "certs",
		"server.cert"))
	ts.Require().NoError(err, "the server certificate is what this gateway is trusted by")
	_, gatewayID := ts.registerAuditGateway(map[string]any{
		"name":          fmt.Sprintf("audit-self-%d", time.Now().UnixNano()),
		"baseUrl":       testutils.TestServerURL,
		"key":           auditGatewayKey,
		"caCertificate": string(certificate),
	})
	return gatewayID, string(certificate)
}

// registerAuditGateway registers a gateway as the admin, removed when the test ends, and returns the
// response and the gateway's ID.
func (ts *AuditLogTestSuite) registerAuditGateway(registration map[string]any) (*http.Response, string) {
	ts.T().Helper()
	resp, raw := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/gateways", ts.jsonBody(registration))
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(raw))
	gatewayID := ts.idOf(raw)
	ts.T().Cleanup(func() { ts.send(http.MethodDelete, "/gateways/"+gatewayID, "") })
	return resp, gatewayID
}

// captureVersion captures the server's configuration as a version and returns its hash.
func (ts *AuditLogTestSuite) captureVersion() string {
	ts.T().Helper()
	resp, raw := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/configuration-versions", "{}")
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK}, resp.StatusCode, string(raw))
	var captured struct {
		Version string `json:"version"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &captured))
	ts.Require().NotEmpty(captured.Version)
	return captured.Version
}

// gatewayCall makes a management call as the admin and checks it answered with status.
func (ts *AuditLogTestSuite) gatewayCall(method, path, body string, status int) *http.Response {
	ts.T().Helper()
	resp, raw := ts.exchange(testutils.GetHTTPClient(), method, path, body)
	ts.Require().Equal(status, resp.StatusCode, string(raw))
	return resp
}
