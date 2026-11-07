// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// auditConfigOrigin is the value these tests write to a section; no line may carry it.
const auditConfigOrigin = "https://audit-config-origin.example.com"

// A write to a section of the server configuration writes one line naming the section. A data plane takes
// a section only from an import with the management API key, so the key is the actor of each write, and a
// token's write is denied with the admin as actor. A write the service refuses names the section the caller
// asked for and has no subject. No line carries the value written.
func (ts *AuditLogTestSuite) TestAServerConfigWriteRecordsTheSectionOnly() {
	adminID := ts.adminUserID()
	previous, err := testutils.GetWritableServerConfig("cors")
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.PutWritableServerConfig("cors", previous) })
	unknownSection := ts.prefix + "no-such-section"

	written := ts.importSection("cors", `{"allowedOrigins":["`+auditConfigOrigin+`"]}`)
	refused := ts.importSection("cors", `{"allowedOrigins":"`+auditConfigOrigin+`"}`)
	unknown := ts.importSection(unknownSection, `{"allowedOrigins":["`+auditConfigOrigin+`"]}`)
	denied := ts.send(http.MethodPut, "/server-config/cors", `{"allowedOrigins":["`+auditConfigOrigin+`"]}`)
	ts.Require().Equal(http.StatusForbidden, denied.StatusCode)

	managementAPIKey := testutils.AuditRef{Type: "system", ID: "management-api-key"}
	lines := ts.linesOf(written, refused, unknown, denied)
	ts.Equal(lineKeys, ts.keys(lines[0].Raw))
	for i, tc := range []struct {
		target  testutils.AuditRef
		subject testutils.AuditRef
		actor   testutils.AuditRef
		outcome map[string]string
	}{
		{
			target:  testutils.AuditRef{Type: "server_config", ID: "cors"},
			subject: testutils.AuditRef{Type: "server_config", ID: "cors"},
			actor:   managementAPIKey,
			outcome: map[string]string{"result": "succeeded"},
		},
		{
			target:  testutils.AuditRef{Type: "server_config", ID: "cors"},
			actor:   managementAPIKey,
			outcome: map[string]string{"result": "failed", "reason": "scf_1003"},
		},
		{
			target:  testutils.AuditRef{Type: "server_config", ID: unknownSection},
			actor:   managementAPIKey,
			outcome: map[string]string{"result": "failed", "reason": "scf_1001"},
		},
		{
			// A denied change names the section it was refused on as its subject.
			target:  testutils.AuditRef{Type: "server_config", ID: "cors"},
			subject: testutils.AuditRef{Type: "server_config", ID: "cors"},
			actor:   testutils.AuditRef{Type: "user", ID: adminID},
			outcome: map[string]string{"result": "denied", "reason": "forbidden"},
		},
	} {
		line := lines[i]
		ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
		ts.Equal("server_config.update", line.Event)
		ts.Equal("update", line.Action)
		ts.Equal(tc.target, line.Target)
		ts.Equal(tc.subject, line.Subject)
		ts.Equal(tc.actor, line.Actor)
		ts.Equal(tc.outcome, line.Outcome)
		ts.NotContains(line.Raw, auditConfigOrigin, "the value of a section is never recorded")
		ts.NotContains(line.Raw, "allowedOrigins", "no part of the value of a section is recorded")
	}
}

// importSection writes a section of the server configuration through an import with the management API key
// and returns the response, which answers 200 whatever the outcome of the section.
func (ts *AuditLogTestSuite) importSection(name, value string) *http.Response {
	ts.T().Helper()
	content, err := json.Marshal(map[string]any{
		"resource_type": "server_config", "name": name, "value": json.RawMessage(value),
	})
	ts.Require().NoError(err)
	request, err := json.Marshal(testutils.ImportRequest{Content: string(content),
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"}})
	ts.Require().NoError(err)
	resp := ts.sendWith(testutils.GetManagementAPIKeyHTTPClient(), http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, resp.StatusCode)
	return resp
}
