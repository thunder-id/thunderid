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

// DeclarativeTemplatesTestSuite exercises the notification-template API while the store runs in
// composite mode (the product default): a file-declared template is merged with the database
// templates, is reported read-only, and every write against it is refused. The fixture is
// resources/declarative_resources/notification_templates/email-welcome-declarative.yaml.
const (
	declTemplateChannel     = "email"
	declTemplateID          = "decl-notification-template-1"
	declTemplateHandle      = "it-declarative-welcome"
	declTemplateDisplayName = "Declarative Welcome"
)

type DeclarativeTemplatesTestSuite struct {
	suite.Suite
	client     *http.Client
	ouID       string
	createdIDs []channelID
}

func TestDeclarativeTemplatesTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeTemplatesTestSuite))
}

func (ts *DeclarativeTemplatesTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()
	ouID, err := testutils.FindBootstrapOUID()
	ts.Require().NoError(err)
	ts.ouID = ouID
}

// TearDownSuite removes any database templates the suite created so re-runs do not hit conflicts.
func (ts *DeclarativeTemplatesTestSuite) TearDownSuite() {
	for _, c := range ts.createdIDs {
		status, _ := ts.do(http.MethodDelete,
			fmt.Sprintf("/notification-templates/%s/templates/%s", c.channel, c.id), nil)
		ts.Assert().Equal(http.StatusNoContent, status)
	}
}

// do issues an authenticated request and returns the status code and raw body.
func (ts *DeclarativeTemplatesTestSuite) do(method, path string, body interface{}) (int, []byte) {
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

// TestGetDeclarativeTemplateIsReadOnly verifies the file-declared template is served from the file
// store and the full representation reports it as read-only.
func (ts *DeclarativeTemplatesTestSuite) TestGetDeclarativeTemplateIsReadOnly() {
	status, body := ts.do(http.MethodGet,
		"/notification-templates/"+declTemplateChannel+"/templates/"+declTemplateID, nil)
	ts.Require().Equalf(http.StatusOK, status, "body: %s", body)

	var got struct {
		ID          string `json:"id"`
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName"`
		IsReadOnly  bool   `json:"isReadOnly"`
	}
	ts.Require().NoError(json.Unmarshal(body, &got))
	ts.Require().Equal(declTemplateID, got.ID)
	ts.Require().Equal(declTemplateHandle, got.Handle)
	ts.Require().Equal(declTemplateDisplayName, got.DisplayName)
	ts.Require().True(got.IsReadOnly, "a file-declared template must be reported read-only")
}

// TestListMergesDeclarativeAndDatabaseTemplates verifies the composite store returns both the
// file-declared template and a database template in a single channel listing.
func (ts *DeclarativeTemplatesTestSuite) TestListMergesDeclarativeAndDatabaseTemplates() {
	status, body := ts.do(http.MethodPost,
		"/notification-templates/"+declTemplateChannel+"/templates", map[string]interface{}{
			"handle":      "it-declarative-merge-probe",
			"ouId":        ts.ouID,
			"displayName": "Merge Probe",
			"content":     map[string]string{"subject": "Probe", "body": "<p>b</p>"},
		})
	ts.Require().Equalf(http.StatusCreated, status, "body: %s", body)
	dbID := idOf(body)
	ts.Require().NotEmpty(dbID)
	ts.createdIDs = append(ts.createdIDs, channelID{channel: declTemplateChannel, id: dbID})

	status, body = ts.do(http.MethodGet,
		"/notification-templates/"+declTemplateChannel+"/templates?limit=100", nil)
	ts.Require().Equalf(http.StatusOK, status, "body: %s", body)

	var list struct {
		Templates []struct {
			ID string `json:"id"`
		} `json:"templates"`
	}
	ts.Require().NoError(json.Unmarshal(body, &list))

	ids := make([]string, 0, len(list.Templates))
	for _, t := range list.Templates {
		ids = append(ids, t.ID)
	}
	ts.Require().Containsf(ids, declTemplateID, "listing must include the declarative template, got %v", ids)
	ts.Require().Containsf(ids, dbID, "listing must include the database template, got %v", ids)
}

// TestUpdateDeclarativeTemplateIsRejected verifies a file-declared template cannot be updated and
// remains readable after the rejected write.
func (ts *DeclarativeTemplatesTestSuite) TestUpdateDeclarativeTemplateIsRejected() {
	status, body := ts.do(http.MethodPut,
		"/notification-templates/"+declTemplateChannel+"/templates/"+declTemplateID,
		map[string]interface{}{
			"displayName": "Renamed Declarative",
			"content":     map[string]string{"subject": "Renamed", "body": "<p>b</p>"},
		})
	ts.Require().Equalf(http.StatusForbidden, status, "body: %s", body)
	ts.Require().Equal("NTM-1022", errorCode(body))

	status, _ = ts.do(http.MethodGet,
		"/notification-templates/"+declTemplateChannel+"/templates/"+declTemplateID, nil)
	ts.Require().Equal(http.StatusOK, status, "the template must survive a rejected update")
}

// TestDeleteDeclarativeTemplateIsRejected verifies a file-declared template cannot be deleted and
// remains readable after the rejected delete.
func (ts *DeclarativeTemplatesTestSuite) TestDeleteDeclarativeTemplateIsRejected() {
	status, body := ts.do(http.MethodDelete,
		"/notification-templates/"+declTemplateChannel+"/templates/"+declTemplateID, nil)
	ts.Require().Equalf(http.StatusForbidden, status, "body: %s", body)
	ts.Require().Equal("NTM-1022", errorCode(body))

	status, _ = ts.do(http.MethodGet,
		"/notification-templates/"+declTemplateChannel+"/templates/"+declTemplateID, nil)
	ts.Require().Equal(http.StatusOK, status, "the template must survive a rejected delete")
}

// TestCreateWithDeclarativeHandleIsRejected verifies the handle-uniqueness check spans the file
// store, so a database template cannot reuse a handle a file already declares.
func (ts *DeclarativeTemplatesTestSuite) TestCreateWithDeclarativeHandleIsRejected() {
	status, body := ts.do(http.MethodPost,
		"/notification-templates/"+declTemplateChannel+"/templates", map[string]interface{}{
			"handle":      declTemplateHandle,
			"ouId":        ts.ouID,
			"displayName": "Shadow Attempt",
			"content":     map[string]string{"subject": "Shadow", "body": "<p>b</p>"},
		})

	// If the conflict check ever stops spanning the file store the template is created instead;
	// record it so the leaked row does not shadow the declarative template on later runs.
	if status == http.StatusCreated {
		ts.createdIDs = append(ts.createdIDs, channelID{channel: declTemplateChannel, id: idOf(body)})
	}

	ts.Require().Equalf(http.StatusConflict, status, "body: %s", body)
	ts.Require().Equal("NTM-1009", errorCode(body))
}
