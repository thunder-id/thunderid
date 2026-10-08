// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// TemplatesTestSuite exercises the notification-template REST surface end to end, focusing on the
// per-channel content rules: an email requires a subject, while SMS forbids one. Templates are
// deployment-scoped, so no organization unit setup is needed.
type TemplatesTestSuite struct {
	suite.Suite
	client     *http.Client
	createdIDs []channelID
}

type channelID struct {
	channel string
	id      string
}

func TestTemplatesTestSuite(t *testing.T) {
	suite.Run(t, new(TemplatesTestSuite))
}

func (ts *TemplatesTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()
}

// TearDownSuite deletes every template the suite created so re-runs do not hit handle conflicts.
func (ts *TemplatesTestSuite) TearDownSuite() {
	for _, c := range ts.createdIDs {
		status, _ := ts.do(http.MethodDelete,
			fmt.Sprintf("/notification-templates/%s/templates/%s", c.channel, c.id), nil)
		// Non-fatal: a failed delete must not abort the loop, or the remaining fixed-handle fixtures
		// leak and cause 409 conflicts on the next run. Delete is idempotent, so 204 is expected.
		ts.Assert().Equal(http.StatusNoContent, status)
	}
}

// do issues an authenticated request and returns the status code and raw body.
func (ts *TemplatesTestSuite) do(method, path string, body interface{}) (int, []byte) {
	ts.T().Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		ts.Require().NoError(err)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, respBody
}

func errorCode(body []byte) string {
	var resp struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &resp)
	return resp.Code
}

func idOf(body []byte) string {
	var resp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &resp)
	return resp.ID
}

// create posts a template and, on success, records its id for cleanup.
func (ts *TemplatesTestSuite) create(channel string, body map[string]interface{}) (int, []byte) {
	status, respBody := ts.do(http.MethodPost,
		fmt.Sprintf("/notification-templates/%s/templates", channel), body)
	if status == http.StatusCreated {
		ts.createdIDs = append(ts.createdIDs, channelID{channel: channel, id: idOf(respBody)})
	}
	return status, respBody
}

// TestEmailLifecycle is the happy path: an email with a subject can be created, read, updated, and
// deleted.
func (ts *TemplatesTestSuite) TestEmailLifecycle() {
	status, body := ts.create("email", map[string]interface{}{
		"handle":      "it-welcome",
		"displayName": "Welcome",
		"content":     map[string]string{"subject": "Welcome", "body": "<p>Hi {{ctx(name)}}</p>"},
	})
	ts.Require().Equalf(http.StatusCreated, status, "body: %s", body)
	id := idOf(body)
	ts.Require().NotEmpty(id)

	status, _ = ts.do(http.MethodGet, "/notification-templates/email/templates/"+id, nil)
	ts.Require().Equal(http.StatusOK, status)

	status, body = ts.do(http.MethodPut, "/notification-templates/email/templates/"+id,
		map[string]interface{}{
			"displayName": "Welcome v2",
			"content":     map[string]string{"subject": "Welcome again", "body": "<p>Hi</p>"},
		})
	ts.Require().Equalf(http.StatusOK, status, "body: %s", body)

	status, _ = ts.do(http.MethodDelete, "/notification-templates/email/templates/"+id, nil)
	ts.Require().Equal(http.StatusNoContent, status)
}

// TestListAndDesign creates an email template with a design, reads it back (covering the DESIGN
// column round-trip), and lists the channel asserting the summary envelope. self appears only on
// list items, not on the single-template response.
func (ts *TemplatesTestSuite) TestListAndDesign() {
	status, body := ts.create("email", map[string]interface{}{
		"handle":      "it-list-design",
		"displayName": "List Design",
		"content":     map[string]string{"subject": "Hi", "body": "<p>b</p>"},
		"design":      map[string]string{"colorScheme": "light"},
	})
	ts.Require().Equalf(http.StatusCreated, status, "body: %s", body)
	id := idOf(body)

	// Single GET covers the DESIGN column read-back and confirms self is absent here.
	status, body = ts.do(http.MethodGet, "/notification-templates/email/templates/"+id, nil)
	ts.Require().Equalf(http.StatusOK, status, "body: %s", body)
	var got struct {
		ID     string `json:"id"`
		Design struct {
			ColorScheme string `json:"colorScheme"`
		} `json:"design"`
		Self string `json:"self"`
	}
	ts.Require().NoError(json.Unmarshal(body, &got))
	ts.Require().Equal("light", got.Design.ColorScheme)
	ts.Require().Empty(got.Self)

	// List returns the summary envelope; each item carries a self link.
	status, body = ts.do(http.MethodGet, "/notification-templates/email/templates", nil)
	ts.Require().Equalf(http.StatusOK, status, "body: %s", body)
	var list struct {
		TotalResults int `json:"totalResults"`
		Count        int `json:"count"`
		Templates    []struct {
			ID          string `json:"id"`
			Handle      string `json:"handle"`
			DisplayName string `json:"displayName"`
			Self        string `json:"self"`
		} `json:"templates"`
	}
	ts.Require().NoError(json.Unmarshal(body, &list))
	ts.Require().GreaterOrEqual(list.TotalResults, 1)

	found := false
	for _, t := range list.Templates {
		if t.ID == id {
			found = true
			ts.Require().Equal("it-list-design", t.Handle)
			ts.Require().Equal("List Design", t.DisplayName)
			ts.Require().NotEmpty(t.Self)
		}
	}
	ts.Require().True(found, "created template not found in list")
}

// TestEmailRequiresSubject locks the new rule: an email template must carry a non-blank subject on
// both create and update.
func (ts *TemplatesTestSuite) TestEmailRequiresSubject() {
	// Create without a subject.
	status, body := ts.create("email", map[string]interface{}{
		"handle":      "it-no-subject",
		"displayName": "No Subject",
		"content":     map[string]string{"body": "<p>b</p>"},
	})
	ts.Require().Equal(http.StatusBadRequest, status)
	ts.Require().Equal("NTM-1003", errorCode(body))

	// A whitespace-only subject is treated as missing.
	status, body = ts.create("email", map[string]interface{}{
		"handle":      "it-blank-subject",
		"displayName": "Blank Subject",
		"content":     map[string]string{"subject": "   ", "body": "<p>b</p>"},
	})
	ts.Require().Equal(http.StatusBadRequest, status)
	ts.Require().Equal("NTM-1003", errorCode(body))

	// Create with a subject, then try to drop it on update.
	status, body = ts.create("email", map[string]interface{}{
		"handle":      "it-drop-subject",
		"displayName": "Drop Subject",
		"content":     map[string]string{"subject": "Present", "body": "<p>b</p>"},
	})
	ts.Require().Equalf(http.StatusCreated, status, "body: %s", body)
	id := idOf(body)

	status, body = ts.do(http.MethodPut, "/notification-templates/email/templates/"+id,
		map[string]interface{}{
			"displayName": "Drop Subject",
			"content":     map[string]string{"body": "<p>b</p>"},
		})
	ts.Require().Equal(http.StatusBadRequest, status)
	ts.Require().Equal("NTM-1003", errorCode(body))
}

// TestSMSSubjectRules confirms the mirror rule: SMS needs no subject and rejects one if supplied.
func (ts *TemplatesTestSuite) TestSMSSubjectRules() {
	// SMS without a subject succeeds.
	status, body := ts.create("sms", map[string]interface{}{
		"handle":      "it-sms-ok",
		"displayName": "SMS OK",
		"content":     map[string]string{"body": "Your code is {{ctx(otp)}}"},
	})
	ts.Require().Equalf(http.StatusCreated, status, "body: %s", body)

	// SMS with a subject is rejected.
	status, body = ts.create("sms", map[string]interface{}{
		"handle":      "it-sms-subject",
		"displayName": "SMS Subject",
		"content":     map[string]string{"subject": "nope", "body": "b"},
	})
	ts.Require().Equal(http.StatusBadRequest, status)
	ts.Require().Equal("NTM-1012", errorCode(body))
}

// TestInvalidChannel confirms an unknown channel path segment is rejected as an invalid channel
// (not misreported as a bad request body).
func (ts *TemplatesTestSuite) TestInvalidChannel() {
	status, body := ts.do(http.MethodPost, "/notification-templates/emailf/templates",
		map[string]interface{}{
			"handle":      "it-bad-channel",
			"displayName": "Bad Channel",
			"content":     map[string]string{"subject": "s", "body": "b"},
		})
	ts.Require().Equal(http.StatusBadRequest, status)
	ts.Require().Equal("NTM-1007", errorCode(body))
}

// TestHandleConflict confirms a duplicate handle in the same channel is rejected with 409 against the
// real DB UNIQUE constraint.
func (ts *TemplatesTestSuite) TestHandleConflict() {
	body := map[string]interface{}{
		"handle":      "it-dup-handle",
		"displayName": "Dup",
		"content":     map[string]string{"subject": "s", "body": "<p>b</p>"},
	}
	status, resp := ts.create("email", body)
	ts.Require().Equalf(http.StatusCreated, status, "body: %s", resp)

	status, resp = ts.create("email", body)
	ts.Require().Equal(http.StatusConflict, status)
	ts.Require().Equal("NTM-1009", errorCode(resp))
}

// TestGetAndPutNotFound confirms GET and PUT on a missing template return 404.
func (ts *TemplatesTestSuite) TestGetAndPutNotFound() {
	missing := "/notification-templates/email/templates/00000000-0000-0000-0000-000000000000"

	status, body := ts.do(http.MethodGet, missing, nil)
	ts.Require().Equal(http.StatusNotFound, status)
	ts.Require().Equal("NTM-1004", errorCode(body))

	status, body = ts.do(http.MethodPut, missing, map[string]interface{}{
		"displayName": "x",
		"content":     map[string]string{"subject": "s", "body": "<p>b</p>"},
	})
	ts.Require().Equal(http.StatusNotFound, status)
	ts.Require().Equal("NTM-1004", errorCode(body))
}

// TestInvalidColorScheme confirms an email design with an unknown color scheme is rejected.
func (ts *TemplatesTestSuite) TestInvalidColorScheme() {
	status, body := ts.create("email", map[string]interface{}{
		"handle":      "it-bad-scheme",
		"displayName": "Bad Scheme",
		"content":     map[string]string{"subject": "s", "body": "<p>b</p>"},
		"design":      map[string]string{"colorScheme": "teal"},
	})
	ts.Require().Equal(http.StatusBadRequest, status)
	ts.Require().Equal("NTM-1017", errorCode(body))
}
