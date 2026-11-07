// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The Control Plane writes an audit line for each notification sender and template it creates, updates
// and deletes, with the administrator the trusted issuer's token names as the actor and no provider
// credential or template content. It writes its output to the server log only when a test started it,
// so the test restarts it first.
func (ts *ControlPlaneTestSuite) TestNotificationSenderAndTemplateChangesAreAudited() {
	ts.Require().NoError(testutils.RestartServer(), "the Control Plane must restart so its output is captured")
	reader := testutils.NewAuditLogReader()
	ts.Require().NoError(reader.Reset())

	senderName := unique("cp-audit-sender")
	authToken := unique("CpAuditAuthToken")
	sender := map[string]any{
		"name": senderName, "ouId": ts.ouID, "accountSid": "AC00000000000000000000000000000000",
		"authToken": authToken, "senderId": "+15005550006",
	}
	var createdSender struct {
		ID string `json:"id"`
	}
	status, raw := ts.call(http.MethodPost, "/connections/twilio", sender, &createdSender)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.T().Cleanup(func() { ts.call(http.MethodDelete, "/connections/twilio/"+createdSender.ID, nil, nil) })
	sender["name"] = senderName + "-renamed"
	status, raw = ts.call(http.MethodPut, "/connections/twilio/"+createdSender.ID, sender, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	status, raw = ts.call(http.MethodDelete, "/connections/twilio/"+createdSender.ID, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	handle := unique("cp-audit-template")
	subject := unique("Control Plane audited subject")
	var createdTemplate struct {
		ID string `json:"id"`
	}
	status, raw = ts.call(http.MethodPost, "/notification-templates/email/templates", map[string]any{
		"handle": handle, "ouId": ts.ouID, "displayName": "Control Plane Audit",
		"content": map[string]string{"subject": subject, "body": "<p>Hi</p>"},
	}, &createdTemplate)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	templatePath := "/notification-templates/email/templates/" + createdTemplate.ID
	ts.T().Cleanup(func() { ts.call(http.MethodDelete, templatePath, nil, nil) })
	status, raw = ts.call(http.MethodPut, templatePath, map[string]any{
		"displayName": "Control Plane Audit", "content": map[string]string{"subject": subject, "body": "<p>Hello</p>"},
	}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	status, raw = ts.call(http.MethodDelete, templatePath, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	senderRef := testutils.AuditRef{Type: "notification_sender", ID: createdSender.ID}
	templateRef := testutils.AuditRef{Type: "notification_template", ID: createdTemplate.ID}
	for _, tc := range []struct {
		event  string
		target testutils.AuditRef
	}{
		{"notification_sender.create", senderRef},
		{"notification_sender.update", senderRef},
		{"notification_sender.delete", senderRef},
		{"notification_template.create", templateRef},
		{"notification_template.update", templateRef},
		{"notification_template.delete", templateRef},
	} {
		line, err := reader.Await(func(line testutils.AuditLine) bool {
			return line.Event == tc.event && line.Target == tc.target
		})
		ts.Require().NoError(err, tc.event)
		ts.Equal(tc.target, line.Subject, tc.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}, line.Actor, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
		ts.NotContains(line.Raw, authToken, "a provider credential is never recorded")
		ts.NotContains(line.Raw, senderName, "the name of the sender is not recorded")
		ts.NotContains(line.Raw, handle, "the handle of the template is not recorded")
		ts.NotContains(line.Raw, subject, "the content of the template is not recorded")
	}
}
