// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mgt

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	errorInvalidFlowOU      = "FLM-1027"
	errorOUUpdateNotAllowed = "FLM-1028"
	errorFlowOURequired     = "FLM-1029"
	errorOUHasDependents    = "OU-1006"
	unknownOUID             = "01900000-0000-7000-8000-0000000000ff"
)

// FlowOwnerOUTestSuite covers the organization unit a flow is owned by: it is required on every create or update
// request, it has to exist, and it cannot be changed afterwards. Nothing restricts access by owner yet.
type FlowOwnerOUTestSuite struct {
	suite.Suite
	createdFlowIDs []string
	firstOUID      string
	secondOUID     string
}

func TestFlowOwnerOUTestSuite(t *testing.T) {
	suite.Run(t, new(FlowOwnerOUTestSuite))
}

func (suite *FlowOwnerOUTestSuite) SetupSuite() {
	var err error
	suite.firstOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: generateUniqueHandle("flow-owner-first"), Name: "Flow owner first",
	})
	suite.Require().NoError(err)
	suite.secondOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: generateUniqueHandle("flow-owner-second"), Name: "Flow owner second",
	})
	suite.Require().NoError(err)
}

func (suite *FlowOwnerOUTestSuite) TearDownSuite() {
	for _, flowID := range suite.createdFlowIDs {
		req, _ := http.NewRequest(http.MethodDelete, testServerURL+flowsEndpoint+"/"+flowID, nil)
		if resp, err := testutils.GetHTTPClient().Do(req); err == nil {
			resp.Body.Close()
		}
	}
	for _, ouID := range []string{suite.firstOUID, suite.secondOUID} {
		if ouID != "" {
			if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
				suite.T().Logf("teardown: failed to delete organization unit %s: %v", ouID, err)
			}
		}
	}
}

// flowBody returns a valid flow with a handle of its own, and the organization unit when ouID is not empty.
func (suite *FlowOwnerOUTestSuite) flowBody(ouID string) map[string]interface{} {
	flow := cloneFlowWithUniqueHandle(testAuthFlow)
	flow.OUID = ouID
	data, err := json.Marshal(flow)
	suite.Require().NoError(err)
	var body map[string]interface{}
	suite.Require().NoError(json.Unmarshal(data, &body))
	return body
}

// send sends a JSON request and returns the status and the body.
func (suite *FlowOwnerOUTestSuite) send(method, path string, payload interface{}) (int, []byte) {
	var reader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		suite.Require().NoError(err)
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, body
}

// create creates a flow, expecting it to succeed, and tracks it for clean-up.
func (suite *FlowOwnerOUTestSuite) create(path string, payload map[string]interface{}) *CompleteFlowDefinition {
	status, body := suite.send(http.MethodPost, path, payload)
	suite.Require().Equal(http.StatusCreated, status, string(body))
	var flow CompleteFlowDefinition
	suite.Require().NoError(json.Unmarshal(body, &flow))
	suite.createdFlowIDs = append(suite.createdFlowIDs, flow.ID)
	return &flow
}

func (suite *FlowOwnerOUTestSuite) expectError(method, path string, payload interface{}, status int, code string) {
	got, body := suite.send(method, path, payload)
	suite.Equal(status, got, string(body))
	var errResp ErrorResponse
	suite.Require().NoError(json.Unmarshal(body, &errResp))
	suite.Equal(code, errResp.Code)
}

func (suite *FlowOwnerOUTestSuite) TestCreateFlow_RequiresAnOU() {
	suite.expectError(http.MethodPost, flowsEndpoint, suite.flowBody(""),
		http.StatusBadRequest, errorFlowOURequired)
}

func (suite *FlowOwnerOUTestSuite) TestCreateFlow_OwnedByTheOUItNames() {
	flow := suite.create(flowsEndpoint, suite.flowBody(suite.firstOUID))

	suite.Equal(suite.firstOUID, flow.OUID)
	status, body := suite.send(http.MethodGet, flowsEndpoint+"/"+flow.ID, nil)
	suite.Equal(http.StatusOK, status)
	var fetched CompleteFlowDefinition
	suite.Require().NoError(json.Unmarshal(body, &fetched))
	suite.Equal(suite.firstOUID, fetched.OUID)
}

func (suite *FlowOwnerOUTestSuite) TestCreateFlow_RefusesAnOUThatDoesNotExist() {
	suite.expectError(http.MethodPost, flowsEndpoint, suite.flowBody(unknownOUID),
		http.StatusBadRequest, errorInvalidFlowOU)
}

func (suite *FlowOwnerOUTestSuite) TestListFlows_ShowsTheOwner() {
	flow := suite.create(flowsEndpoint, suite.flowBody(suite.firstOUID))

	for offset := 0; ; offset += 100 {
		status, body := suite.send(http.MethodGet,
			flowsEndpoint+"?limit=100&offset="+strconv.Itoa(offset), nil)
		suite.Require().Equal(http.StatusOK, status)
		var listed FlowListResponse
		suite.Require().NoError(json.Unmarshal(body, &listed))
		for _, item := range listed.Flows {
			if item.ID == flow.ID {
				suite.Equal(suite.firstOUID, item.OUID)
				return
			}
		}
		if len(listed.Flows) < 100 {
			break
		}
	}
	suite.Fail("the flow is not in the list")
}

func (suite *FlowOwnerOUTestSuite) TestUpdateFlow_CannotChangeTheOU() {
	flow := suite.create(flowsEndpoint, suite.flowBody(suite.firstOUID))
	update := func(ouID string) map[string]interface{} {
		body := suite.flowBody(ouID)
		body["handle"] = flow.Handle
		return body
	}

	suite.expectError(http.MethodPut, flowsEndpoint+"/"+flow.ID, update(suite.secondOUID),
		http.StatusBadRequest, errorOUUpdateNotAllowed)

	status, body := suite.send(http.MethodPut, flowsEndpoint+"/"+flow.ID, update(suite.firstOUID))
	suite.Require().Equal(http.StatusOK, status, string(body))
	var updated CompleteFlowDefinition
	suite.Require().NoError(json.Unmarshal(body, &updated))
	suite.Equal(suite.firstOUID, updated.OUID, "repeating the organization unit the flow already has keeps it")
}

func (suite *FlowOwnerOUTestSuite) TestUpdateFlow_KeepsTheOUWhenItIsLeftOut() {
	flow := suite.create(flowsEndpoint, suite.flowBody(suite.firstOUID))
	update := suite.flowBody("")
	update["handle"] = flow.Handle

	status, body := suite.send(http.MethodPut, flowsEndpoint+"/"+flow.ID, update)
	suite.Require().Equal(http.StatusOK, status, string(body))
	var updated CompleteFlowDefinition
	suite.Require().NoError(json.Unmarshal(body, &updated))
	suite.Equal(suite.firstOUID, updated.OUID, "an update that leaves the organization unit out keeps the current one")
}

func (suite *FlowOwnerOUTestSuite) TestDeleteOU_RefusedWhileItOwnsAFlow() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: generateUniqueHandle("flow-owner-delete"), Name: "Flow owner delete",
	})
	suite.Require().NoError(err)
	flow := suite.create(flowsEndpoint, suite.flowBody(ouID))

	suite.expectError(http.MethodDelete, "/organization-units/"+ouID, nil,
		http.StatusBadRequest, errorOUHasDependents)

	status, body := suite.send(http.MethodDelete, flowsEndpoint+"/"+flow.ID, nil)
	suite.Require().Equal(http.StatusNoContent, status, string(body))
	status, body = suite.send(http.MethodDelete, "/organization-units/"+ouID, nil)
	suite.Equal(http.StatusNoContent, status, string(body))
}
