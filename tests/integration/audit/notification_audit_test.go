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
	auditSenderAuthToken  = "AuditSenderAuthToken0123456789"
	auditSenderAccountSID = "AC" + "0123456789abcdef" + "0123456789abcdef"
	auditTemplateSubject  = "Audited welcome subject"
	auditTemplateBody     = "<p>Audited welcome body for {{ctx(name)}}</p>"
)

// Creating, updating and deleting a notification sender through the connections API writes one line
// each, with the admin as actor and the sender as target and subject. No property of the sender, its
// provider credential, account or name, appears in any line.
func (ts *AuditLogTestSuite) TestANotificationSenderLifecycleWritesOneLinePerChange() {
	ouID := ts.createOrganizationUnit("sender-lifecycle")
	adminID := ts.adminUserID()
	name := ts.prefix + "lifecycle-sender"

	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/connections/twilio",
		ts.twilioBody(name, ouID))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	senderID := ts.idOf(body)
	ts.T().Cleanup(func() { ts.send(http.MethodDelete, "/connections/twilio/"+senderID, "") })
	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/connections/twilio/"+senderID,
		ts.twilioBody(name+"-renamed", ouID))
	ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
	deleted := ts.send(http.MethodDelete, "/connections/twilio/"+senderID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	senderRef := testutils.AuditRef{Type: "notification_sender", ID: senderID}
	lines := ts.linesOf(created, updated, deleted)
	for i, tc := range []struct {
		event  string
		action string
	}{
		{"notification_sender.create", "create"},
		{"notification_sender.update", "update"},
		{"notification_sender.delete", "delete"},
	} {
		line := lines[i]
		ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
		ts.Equal(lineKeys, ts.keys(line.Raw), tc.event)
		ts.Equal(tc.event, line.Event)
		ts.Equal(tc.action, line.Action)
		ts.Equal(senderRef, line.Target, tc.event)
		ts.Equal(senderRef, line.Subject, tc.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
		ts.Contains([]string{"127.0.0.1", "::1"}, line.Client["address"])
		ts.requireNoSenderProperty(line, name)
	}
}

// Every attempt to create or update a notification sender that reaches the sender service and fails is
// recorded as failed, with no subject. A create names no sender; an update names the one asked for.
func (ts *AuditLogTestSuite) TestEveryFailedNotificationSenderChangeIsRecorded() {
	ouID := ts.createOrganizationUnit("sender-failed")
	name := ts.prefix + "failed-sender"
	other := ts.prefix + "failed-other"
	senderID := ts.createSender(name, ouID)
	ts.createSender(other, ouID)

	cases := []struct {
		name    string
		method  string
		path    string
		body    string
		status  int
		event   string
		target  testutils.AuditRef
		outcome map[string]string
	}{
		{
			name: "a create with a name another sender has", method: http.MethodPost, path: "/connections/twilio",
			body: ts.twilioBody(name, ouID), status: http.StatusConflict, event: "notification_sender.create",
			target:  testutils.AuditRef{Type: "notification_sender"},
			outcome: map[string]string{"result": "failed", "reason": "mns_1005"},
		},
		{
			name: "a create in an organization unit that does not exist", method: http.MethodPost,
			path: "/connections/twilio", body: ts.twilioBody(ts.prefix+"failed-orphan", unusedID()),
			status: http.StatusBadRequest, event: "notification_sender.create",
			target:  testutils.AuditRef{Type: "notification_sender"},
			outcome: map[string]string{"result": "failed", "reason": "mns_1018"},
		},
		{
			name: "an update to a name another sender has", method: http.MethodPut,
			path: "/connections/twilio/" + senderID, body: ts.twilioBody(other, ouID),
			status: http.StatusConflict, event: "notification_sender.update",
			target:  testutils.AuditRef{Type: "notification_sender", ID: senderID},
			outcome: map[string]string{"result": "failed", "reason": "mns_1005"},
		},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		response, body := ts.exchange(testutils.GetHTTPClient(), tc.method, tc.path, tc.body)
		ts.Require().Equal(tc.status, response.StatusCode, "%s: %s", tc.name, string(body))
		responses[i] = response
	}

	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		ts.Equal(tc.event, line.Event, tc.name)
		ts.Equal(tc.target, line.Target, tc.name)
		ts.Equal(testutils.AuditRef{}, line.Subject, "%s: a failed attempt has no subject", tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		ts.requireNoSenderProperty(line, name)
	}
}

// The connections API looks a sender up before it hands an update or a delete to the sender service, so
// an update or a delete of a sender that does not exist never reaches it and writes no line. The delete
// still succeeds.
func (ts *AuditLogTestSuite) TestAChangeOfAMissingNotificationSenderWritesNoLine() {
	ouID := ts.createOrganizationUnit("sender-missing")
	missing := unusedID()

	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/connections/twilio/"+missing,
		ts.twilioBody(ts.prefix+"missing-sender", ouID))
	ts.Require().Equal(http.StatusNotFound, updated.StatusCode, string(body))
	deleted := ts.send(http.MethodDelete, "/connections/twilio/"+missing, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	for _, written := range ts.linesPerCall(updated, deleted) {
		ts.Empty(written, "a change of a missing sender writes no line")
	}
}

// An import that creates a notification sender writes the sender service's event for it and nothing
// else: the lookup that decides between update and create writes no line.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfTheNotificationSenderItCreates() {
	ouID := ts.createOrganizationUnit("sender-import")
	senderID := unusedID()
	name := ts.prefix + "imported-sender"
	request, err := json.Marshal(testutils.ImportRequest{
		Content: fmt.Sprintf("resource_type: connection\nid: %s\ntype: twilio\nname: %s\nouId: %s\n"+
			"accountSid: %s\nauthToken: %s\nsenderId: \"+15005550006\"\n",
			senderID, name, ouID, auditSenderAccountSID, auditSenderAuthToken),
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	ts.Require().NoError(err)

	imported, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	ts.T().Cleanup(func() { ts.send(http.MethodDelete, "/connections/twilio/"+senderID, "") })
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Equal(1, result.Summary.Imported, string(body))

	line := ts.linesOf(imported)[0]
	senderRef := testutils.AuditRef{Type: "notification_sender", ID: senderID}
	ts.Equal("notification_sender.create", line.Event)
	ts.Equal(senderRef, line.Target)
	ts.Equal(senderRef, line.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
	ts.requireNoSenderProperty(line, name)
}

// Creating, updating and deleting a notification template writes one line each, with the admin as actor
// and the template as target and subject. No part of the template, its handle, name, subject or body,
// appears in any line.
func (ts *AuditLogTestSuite) TestANotificationTemplateLifecycleWritesOneLinePerChange() {
	ouID := ts.createOrganizationUnit("template-lifecycle")
	adminID := ts.adminUserID()
	handle := ts.prefix + "lifecycle-template"

	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost,
		"/notification-templates/email/templates", ts.templateBody(handle, ouID))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	templateID := ts.idOf(body)
	ts.T().Cleanup(func() { ts.send(http.MethodDelete, "/notification-templates/email/templates/"+templateID, "") })
	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut,
		"/notification-templates/email/templates/"+templateID, ts.templateBody("", ""))
	ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
	deleted := ts.send(http.MethodDelete, "/notification-templates/email/templates/"+templateID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	templateRef := testutils.AuditRef{Type: "notification_template", ID: templateID}
	lines := ts.linesOf(created, updated, deleted)
	for i, tc := range []struct {
		event  string
		action string
	}{
		{"notification_template.create", "create"},
		{"notification_template.update", "update"},
		{"notification_template.delete", "delete"},
	} {
		line := lines[i]
		ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
		ts.Equal(lineKeys, ts.keys(line.Raw), tc.event)
		ts.Equal(tc.event, line.Event)
		ts.Equal(tc.action, line.Action)
		ts.Equal(templateRef, line.Target, tc.event)
		ts.Equal(templateRef, line.Subject, tc.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
		ts.requireNoTemplateContent(line, handle)
	}
}

// Every attempt to change a notification template that the template service refuses is recorded as
// failed, with no subject; an update of a template that does not exist names the ID asked for.
func (ts *AuditLogTestSuite) TestEveryFailedNotificationTemplateChangeIsRecorded() {
	ouID := ts.createOrganizationUnit("template-failed")
	handle := ts.prefix + "failed-template"
	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost,
		"/notification-templates/email/templates", ts.templateBody(handle, ouID))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	templateID := ts.idOf(body)
	ts.T().Cleanup(func() { ts.send(http.MethodDelete, "/notification-templates/email/templates/"+templateID, "") })
	missing := unusedID()
	noTemplate := testutils.AuditRef{Type: "notification_template"}

	cases := []struct {
		name    string
		method  string
		path    string
		body    string
		status  int
		event   string
		target  testutils.AuditRef
		outcome map[string]string
	}{
		{
			name: "a create with a handle another template has", method: http.MethodPost,
			path: "/notification-templates/email/templates", body: ts.templateBody(handle, ouID),
			status: http.StatusConflict, event: "notification_template.create", target: noTemplate,
			outcome: map[string]string{"result": "failed", "reason": "ntm_1009"},
		},
		{
			name: "a create without a handle", method: http.MethodPost,
			path: "/notification-templates/email/templates", body: ts.templateBody("", ouID),
			status: http.StatusBadRequest, event: "notification_template.create", target: noTemplate,
			outcome: map[string]string{"result": "failed", "reason": "ntm_1008"},
		},
		{
			name: "a create on a channel that does not exist", method: http.MethodPost,
			path: "/notification-templates/fax/templates", body: ts.templateBody(handle+"-fax", ouID),
			status: http.StatusBadRequest, event: "notification_template.create", target: noTemplate,
			outcome: map[string]string{"result": "failed", "reason": "ntm_1007"},
		},
		{
			name: "an update of a template that does not exist", method: http.MethodPut,
			path: "/notification-templates/email/templates/" + missing, body: ts.templateBody("", ""),
			status: http.StatusNotFound, event: "notification_template.update",
			target:  testutils.AuditRef{Type: "notification_template", ID: missing},
			outcome: map[string]string{"result": "failed", "reason": "ntm_1004"},
		},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		response, body := ts.exchange(testutils.GetHTTPClient(), tc.method, tc.path, tc.body)
		ts.Require().Equal(tc.status, response.StatusCode, "%s: %s", tc.name, string(body))
		responses[i] = response
	}

	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		ts.Equal(tc.event, line.Event, tc.name)
		ts.Equal(tc.target, line.Target, tc.name)
		ts.Equal(testutils.AuditRef{}, line.Subject, "%s: a failed attempt has no subject", tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		ts.requireNoTemplateContent(line, handle)
	}
}

// A delete of a notification template that does not exist still succeeds, but writes no line, so a
// caller cannot record the deletion of an ID it made up.
func (ts *AuditLogTestSuite) TestADeleteOfAMissingNotificationTemplateWritesNoLine() {
	deleted := ts.send(http.MethodDelete, "/notification-templates/email/templates/"+unusedID(), "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	ts.Empty(ts.linesPerCall(deleted)[0], "a delete that removed nothing writes no line")
}

// An import that creates a notification template writes the template service's event for it and
// nothing else: the lookup that decides between update and create writes no line.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfTheNotificationTemplateItCreates() {
	ouID := ts.createOrganizationUnit("template-import")
	templateID := unusedID()
	handle := ts.prefix + "imported-template"
	request, err := json.Marshal(testutils.ImportRequest{
		Content: fmt.Sprintf("resource_type: notification_template\nid: %s\nchannel: email\nhandle: %s\n"+
			"ouId: %s\ndisplayName: Audited Import\ncontent:\n  subject: %s\n  body: %q\n",
			templateID, handle, ouID, auditTemplateSubject, auditTemplateBody),
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	ts.Require().NoError(err)

	imported, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	ts.T().Cleanup(func() { ts.send(http.MethodDelete, "/notification-templates/email/templates/"+templateID, "") })
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Equal(1, result.Summary.Imported, string(body))

	line := ts.linesOf(imported)[0]
	templateRef := testutils.AuditRef{Type: "notification_template", ID: templateID}
	ts.Equal("notification_template.create", line.Event)
	ts.Equal(templateRef, line.Target)
	ts.Equal(templateRef, line.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
	ts.requireNoTemplateContent(line, handle)
}

// twilioBody is the request body of a Twilio sender in ouID, carrying its provider credentials.
func (ts *AuditLogTestSuite) twilioBody(name, ouID string) string {
	ts.T().Helper()
	body, err := json.Marshal(map[string]string{
		"name":       name,
		"ouId":       ouID,
		"accountSid": auditSenderAccountSID,
		"authToken":  auditSenderAuthToken,
		"senderId":   "+15005550006",
	})
	ts.Require().NoError(err)
	return string(body)
}

// createSender creates a Twilio sender, removed when the test ends, and returns its ID.
func (ts *AuditLogTestSuite) createSender(name, ouID string) string {
	ts.T().Helper()
	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/connections/twilio",
		ts.twilioBody(name, ouID))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	senderID := ts.idOf(body)
	ts.T().Cleanup(func() { ts.send(http.MethodDelete, "/connections/twilio/"+senderID, "") })
	return senderID
}

// templateBody is the request body of an email template. An update leaves handle and ouID empty, as
// both are fixed once the template is created.
func (ts *AuditLogTestSuite) templateBody(handle, ouID string) string {
	ts.T().Helper()
	request := map[string]interface{}{
		"displayName": "Audited Welcome",
		"content":     map[string]string{"subject": auditTemplateSubject, "body": auditTemplateBody},
	}
	if handle != "" {
		request["handle"] = handle
	}
	if ouID != "" {
		request["ouId"] = ouID
	}
	body, err := json.Marshal(request)
	ts.Require().NoError(err)
	return string(body)
}

// requireNoSenderProperty checks that no property or name of a sender appears in line.
func (ts *AuditLogTestSuite) requireNoSenderProperty(line testutils.AuditLine, name string) {
	ts.T().Helper()
	ts.NotContains(line.Raw, auditSenderAuthToken, "a provider credential is never recorded")
	ts.NotContains(line.Raw, "AuditSenderAuthToken", "no part of a provider credential is recorded")
	ts.NotContains(line.Raw, auditSenderAccountSID, "no property of the sender is recorded")
	ts.NotContains(line.Raw, "+15005550006", "no property of the sender is recorded")
	ts.NotContains(line.Raw, name, "the name of the sender is not recorded")
}

// requireNoTemplateContent checks that no part of a template appears in line.
func (ts *AuditLogTestSuite) requireNoTemplateContent(line testutils.AuditLine, handle string) {
	ts.T().Helper()
	ts.NotContains(line.Raw, handle, "the handle of the template is not recorded")
	ts.NotContains(line.Raw, "Audited Welcome", "the name of the template is not recorded")
	ts.NotContains(line.Raw, auditTemplateSubject, "the subject of the template is not recorded")
	ts.NotContains(line.Raw, "Audited welcome body", "the body of the template is not recorded")
}
