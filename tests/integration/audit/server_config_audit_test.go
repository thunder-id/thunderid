// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// auditConfigOrigin is the value these tests write to a section; no line may carry it.
const auditConfigOrigin = "https://audit-config-origin.example.com"

// A write to a section of the server configuration writes one line naming the section, with the admin
// as actor. A write the service refuses names the section the caller asked for and has no subject. No
// line carries the value written.
func (ts *AuditLogTestSuite) TestAServerConfigWriteRecordsTheSectionOnly() {
	adminID := ts.adminUserID()
	previous, err := testutils.GetWritableServerConfig("cors")
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.PutWritableServerConfig("cors", previous) })
	unknownSection := ts.prefix + "no-such-section"

	written := ts.send(http.MethodPut, "/server-config/cors", `{"allowedOrigins":["`+auditConfigOrigin+`"]}`)
	ts.Require().Equal(http.StatusOK, written.StatusCode)
	refused := ts.send(http.MethodPut, "/server-config/cors", `{"allowedOrigins":"`+auditConfigOrigin+`"}`)
	ts.Require().Equal(http.StatusBadRequest, refused.StatusCode)
	unknown := ts.send(http.MethodPut, "/server-config/"+unknownSection,
		`{"allowedOrigins":["`+auditConfigOrigin+`"]}`)
	ts.Require().Equal(http.StatusBadRequest, unknown.StatusCode)

	lines := ts.linesOf(written, refused, unknown)
	ts.Equal(lineKeys, ts.keys(lines[0].Raw))
	for i, tc := range []struct {
		target  testutils.AuditRef
		subject testutils.AuditRef
		outcome map[string]string
	}{
		{
			target:  testutils.AuditRef{Type: "server_config", ID: "cors"},
			subject: testutils.AuditRef{Type: "server_config", ID: "cors"},
			outcome: map[string]string{"result": "succeeded"},
		},
		{
			target:  testutils.AuditRef{Type: "server_config", ID: "cors"},
			outcome: map[string]string{"result": "failed", "reason": "scf_1003"},
		},
		{
			target:  testutils.AuditRef{Type: "server_config", ID: unknownSection},
			outcome: map[string]string{"result": "failed", "reason": "scf_1001"},
		},
	} {
		line := lines[i]
		ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
		ts.Equal("server_config.update", line.Event)
		ts.Equal("update", line.Action)
		ts.Equal(tc.target, line.Target)
		ts.Equal(tc.subject, line.Subject)
		ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor)
		ts.Equal(tc.outcome, line.Outcome)
		ts.NotContains(line.Raw, auditConfigOrigin, "the value of a section is never recorded")
		ts.NotContains(line.Raw, "allowedOrigins", "no part of the value of a section is recorded")
	}
}
