// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mgt

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ResourceDependency is one resource reported as referencing the flow.
type ResourceDependency struct {
	ResourceType     string `json:"resourceType"`
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	BehaviorOnDelete string `json:"behaviorOnDelete"`
}

// FlowUsagesResponse is the body of GET /flows/{flowId}/usages.
//
// TotalResults and Summary are pointers/maps that stay nil when dependency data is unavailable,
// which the API distinguishes from a confirmed-empty result, so the tests assert on them directly
// rather than through a zero value.
type FlowUsagesResponse struct {
	TotalResults *int                 `json:"totalResults"`
	Count        int                  `json:"count"`
	Summary      map[string]int       `json:"summary"`
	Usages       []ResourceDependency `json:"usages"`
}

var usagesTestOU = testutils.OrganizationUnit{
	Handle:      "flow_usages_test_ou",
	Name:        "Test OU for Flow Usages",
	Description: "Organization unit created for flow usages testing",
	Parent:      nil,
}

func (suite *FlowMgtAPITestSuite) getFlowUsages(flowID string) *FlowUsagesResponse {
	req, _ := http.NewRequest(http.MethodGet, testServerURL+flowsEndpoint+"/"+flowID+"/usages", nil)

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	suite.Require().Equal(http.StatusOK, resp.StatusCode, "unexpected usages response: %s", string(bodyBytes))

	var response FlowUsagesResponse
	suite.Require().NoError(json.Unmarshal(bodyBytes, &response))

	return &response
}

func (suite *FlowMgtAPITestSuite) getFlowUsagesExpectError(flowID string, expectedStatus int, expectedCode string) {
	req, _ := http.NewRequest(http.MethodGet, testServerURL+flowsEndpoint+"/"+flowID+"/usages", nil)

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(expectedStatus, resp.StatusCode)

	bodyBytes, _ := io.ReadAll(resp.Body)
	var errorResp ErrorResponse
	suite.Require().NoError(json.Unmarshal(bodyBytes, &errorResp))
	suite.Equal(expectedCode, errorResp.Code)
}

// A flow nothing references reports a confirmed-empty result rather than unknown, so TotalResults
// must be present and zero rather than nil.
func (suite *FlowMgtAPITestSuite) TestGetFlowUsages_UnreferencedFlow() {
	createdFlow := suite.createFlow(cloneFlowWithUniqueHandle(testAuthFlow))
	suite.trackFlow(createdFlow.ID)

	response := suite.getFlowUsages(createdFlow.ID)

	suite.Equal(0, response.Count)
	suite.Empty(response.Usages)
	if suite.NotNil(response.TotalResults, "an unreferenced flow should report a known total") {
		suite.Equal(0, *response.TotalResults)
	}
}

// An application bound to the flow must appear as a usage, which is what the Console relies on to
// warn before a flow is deleted.
func (suite *FlowMgtAPITestSuite) TestGetFlowUsages_ReportsBoundApplication() {
	createdFlow := suite.createFlow(cloneFlowWithUniqueHandle(testAuthFlow))
	suite.trackFlow(createdFlow.ID)

	ouID, err := testutils.CreateOrganizationUnit(usagesTestOU)
	suite.Require().NoError(err, "Failed to create test organization unit")
	defer func() {
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			suite.T().Logf("Failed to delete organization unit during cleanup: %v", err)
		}
	}()

	app := testutils.Application{
		Name:         "Flow Usages Test Application",
		Description:  "Application bound to the flow under test",
		ClientID:     "flow_usages_test_client",
		ClientSecret: "flow_usages_test_secret",
		RedirectURIs: []string{"http://localhost:3000/callback"},
		OUID:         ouID,
		AuthFlowID:   createdFlow.ID,
	}
	appID, err := testutils.CreateApplication(app)
	suite.Require().NoError(err, "Failed to create test application")
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			suite.T().Logf("Failed to delete application during cleanup: %v", err)
		}
	}()

	response := suite.getFlowUsages(createdFlow.ID)

	suite.GreaterOrEqual(response.Count, 1, "the bound application should be reported as a usage")

	found := false
	for _, usage := range response.Usages {
		if usage.ID == appID {
			found = true
			suite.Equal("application", usage.ResourceType)
			suite.NotEmpty(usage.DisplayName, "a usage should carry a display name for the Console")
			suite.NotEmpty(usage.BehaviorOnDelete, "a usage should state what happens on delete")
		}
	}
	suite.True(found, "expected application %s in the reported usages", appID)
}

func (suite *FlowMgtAPITestSuite) TestGetFlowUsages_NotFound() {
	suite.getFlowUsagesExpectError("non-existent-id", http.StatusNotFound, "FLM-1003")
}
