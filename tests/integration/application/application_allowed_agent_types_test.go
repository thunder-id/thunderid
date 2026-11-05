// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

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

// applicationInvalidAgentTypeCode is what /applications returns for an allowedAgentTypes list it refuses.
const applicationInvalidAgentTypeCode = "APP-1046"

// ApplicationAllowedAgentTypesTestSuite covers the `*` entry of an application's allowedAgentTypes:
// `["*"]` admits an agent of any agent type and is stored as given, and `*` combined with any other
// entry is refused on create and on update.
type ApplicationAllowedAgentTypesTestSuite struct {
	suite.Suite
	ouID   string
	appIDs []string
}

func TestApplicationAllowedAgentTypesTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationAllowedAgentTypesTestSuite))
}

func (ts *ApplicationAllowedAgentTypesTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "app-allowed-agent-types-ou",
		Name:        "App Allowed Agent Types OU",
		Description: "Organization unit for application allowedAgentTypes testing",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID
}

func (ts *ApplicationAllowedAgentTypesTestSuite) TearDownSuite() {
	for _, id := range ts.appIDs {
		if err := testutils.DeleteApplication(id); err != nil {
			ts.T().Logf("teardown: failed to delete application %s: %v", id, err)
		}
	}
	if ts.ouID != "" {
		ts.NoError(testutils.DeleteOrganizationUnit(ts.ouID))
	}
}

// appBody builds an application request with the given allowed agent types.
func (ts *ApplicationAllowedAgentTypesTestSuite) appBody(
	name string, allowedAgentTypes []string,
) map[string]interface{} {
	return map[string]interface{}{
		"name":              name,
		"description":       "Application for allowedAgentTypes testing",
		"ouId":              ts.ouID,
		"type":              "fullstack",
		"allowedAgentTypes": allowedAgentTypes,
	}
}

// send issues a JSON request and returns the status and body.
func (ts *ApplicationAllowedAgentTypesTestSuite) send(method, path string, body interface{}) (int, []byte) {
	ts.T().Helper()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		ts.Require().NoError(err)
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, respBody
}

// createApp creates an application and registers it for teardown.
func (ts *ApplicationAllowedAgentTypesTestSuite) createApp(name string, allowedAgentTypes []string) string {
	ts.T().Helper()

	status, body := ts.send(http.MethodPost, "/applications", ts.appBody(name, allowedAgentTypes))
	ts.Require().Equal(http.StatusCreated, status, "create failed: %s", string(body))

	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &created))
	ts.appIDs = append(ts.appIDs, created.ID)
	return created.ID
}

// storedAllowedAgentTypes reads an application's allowedAgentTypes back.
func (ts *ApplicationAllowedAgentTypesTestSuite) storedAllowedAgentTypes(appID string) []string {
	ts.T().Helper()

	status, body := ts.send(http.MethodGet, "/applications/"+appID, nil)
	ts.Require().Equal(http.StatusOK, status, "get failed: %s", string(body))

	var app struct {
		AllowedAgentTypes []string `json:"allowedAgentTypes"`
	}
	ts.Require().NoError(json.Unmarshal(body, &app))
	return app.AllowedAgentTypes
}

// requireInvalidAgentType asserts a refusal of the allowedAgentTypes list.
func (ts *ApplicationAllowedAgentTypesTestSuite) requireInvalidAgentType(status int, body []byte) {
	ts.T().Helper()

	ts.Require().Equal(http.StatusBadRequest, status, "body: %s", string(body))
	var errResp struct {
		Code string `json:"code"`
	}
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal(applicationInvalidAgentTypeCode, errResp.Code, "body: %s", string(body))
}

func (ts *ApplicationAllowedAgentTypesTestSuite) TestAnyAgentTypeAccepted() {
	appID := ts.createApp("Any Agent Type App", []string{"*"})

	ts.Equal([]string{"*"}, ts.storedAllowedAgentTypes(appID))
}

func (ts *ApplicationAllowedAgentTypesTestSuite) TestAnyAgentTypeWithOtherEntriesRejectedOnCreate() {
	for i, allowed := range [][]string{{"*", "default"}, {"default", "*"}, {"*", "*"}} {
		ts.Run(fmt.Sprintf("%v", allowed), func() {
			status, body := ts.send(http.MethodPost, "/applications",
				ts.appBody(fmt.Sprintf("Mixed Agent Types App %d", i), allowed))
			ts.requireInvalidAgentType(status, body)
		})
	}
}

func (ts *ApplicationAllowedAgentTypesTestSuite) TestAnyAgentTypeWithOtherEntriesRejectedOnUpdate() {
	appID := ts.createApp("Any Agent Type Update App", []string{"*"})

	status, body := ts.send(http.MethodPut, "/applications/"+appID,
		ts.appBody("Any Agent Type Update App", []string{"default", "*"}))

	ts.requireInvalidAgentType(status, body)
	ts.Equal([]string{"*"}, ts.storedAllowedAgentTypes(appID), "a refused update must keep the stored list")
}
