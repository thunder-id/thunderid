// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	httpConnInUseCode = "CON-1026"

	httpConnUsagesFlowName = "HTTP Connection Usages Flow"
	httpConnUsagesURL      = "https://ext.example.com/http-conn-usages"
)

// httpConnUsagesOU owns every connection the usages suite creates.
var httpConnUsagesOU = testutils.OrganizationUnit{Handle: "http-conn-usages-ou", Name: "HTTP Connection Usages OU"}

// httpConnUsages mirrors the payload of GET /connections/http/{id}/usages.
type httpConnUsages struct {
	TotalResults *int           `json:"totalResults"`
	Count        int            `json:"count"`
	Summary      map[string]int `json:"summary"`
	Usages       []struct {
		ResourceType     string `json:"resourceType"`
		ID               string `json:"id"`
		DisplayName      string `json:"displayName"`
		BehaviorOnDelete string `json:"behaviorOnDelete"`
	} `json:"usages"`
}

// HTTPConnectionUsagesTestSuite covers the usages of an HTTP connection and its delete protection
// while a flow's extension step calls it (R3).
type HTTPConnectionUsagesTestSuite struct {
	suite.Suite
	ouID         string
	connectionID string
	flowID       string
}

func TestHTTPConnectionUsagesTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionUsagesTestSuite))
}

func (s *HTTPConnectionUsagesTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(httpConnUsagesOU)
	s.Require().NoError(err, "Failed to create the organization unit")
	s.ouID = ouID

	connectionID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name: "http-conn-usages", OUID: s.ouID, URL: httpConnUsagesURL,
	})
	s.Require().NoError(err, "Failed to create the HTTP connection")
	s.connectionID = connectionID

	flowID, err := testutils.CreateFlow(testutils.ExtensionAuthFlow(httpConnUsagesFlowName,
		"http_conn_usages_flow", map[string]interface{}{"connectionId": connectionID}))
	s.Require().NoError(err, "Failed to create the flow that uses the connection")
	s.flowID = flowID
}

func (s *HTTPConnectionUsagesTestSuite) TearDownSuite() {
	// The flow goes first: a flow that uses a connection blocks its deletion.
	if s.flowID != "" {
		if err := testutils.DeleteFlow(s.flowID); err != nil {
			s.T().Logf("Failed to delete flow %s during teardown: %v", s.flowID, err)
		}
	}
	if s.connectionID != "" {
		if err := testutils.DeleteHTTPConnection(s.connectionID); err != nil {
			s.T().Logf("Failed to delete HTTP connection during teardown: %v", err)
		}
	}
	// The OU goes last: an OU that owns a connection cannot be deleted.
	if s.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(s.ouID); err != nil {
			s.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}
}

// createConnection creates a connection and deletes it when the test ends. The cleanup is registered
// first, so it runs after the cleanup of any flow the test creates later has removed the usage.
func (s *HTTPConnectionUsagesTestSuite) createConnection(name string) string {
	s.T().Helper()
	connectionID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name: name, OUID: s.ouID, URL: httpConnUsagesURL,
	})
	s.Require().NoError(err, "Failed to create the HTTP connection")
	s.T().Cleanup(func() {
		if err := testutils.DeleteHTTPConnection(connectionID); err != nil {
			s.T().Logf("Failed to delete HTTP connection %s: %v", connectionID, err)
		}
	})
	return connectionID
}

// createFlow creates flow and deletes it when the test ends, unless the test deleted it already.
func (s *HTTPConnectionUsagesTestSuite) createFlow(flow testutils.Flow) string {
	s.T().Helper()
	flowID, err := testutils.CreateFlow(flow)
	s.Require().NoError(err, "Failed to create flow %s", flow.Handle)
	s.T().Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			s.T().Logf("Failed to delete flow %s: %v", flowID, err)
		}
	})
	return flowID
}

func (s *HTTPConnectionUsagesTestSuite) usages(connectionID string) httpConnUsages {
	s.T().Helper()
	res, err := doRequest(http.MethodGet, httpConnPath+"/"+connectionID+"/usages", nil)
	s.Require().NoError(err, "Usages request failed")
	s.Require().Equal(http.StatusOK, res.status, "Usages should be listed: %s", string(res.body))
	var usages httpConnUsages
	s.Require().NoError(res.decode(&usages), "Usages response should decode: %s", string(res.body))
	s.Require().NotNil(usages.TotalResults, "Usages should report a known total: %s", string(res.body))
	return usages
}

func (s *HTTPConnectionUsagesTestSuite) assertDeleteRefused(connectionID, context string) {
	s.T().Helper()
	res, err := doRequest(http.MethodDelete, httpConnPath+"/"+connectionID, nil)
	s.Require().NoError(err, "Delete request failed")
	s.Equal(http.StatusConflict, res.status, "%s: delete should be refused: %s", context, string(res.body))
	s.Equal(httpConnInUseCode, res.errorCode(), "%s: the error should be the blocking-dependencies error", context)

	res, err = doRequest(http.MethodGet, httpConnPath+"/"+connectionID, nil)
	s.Require().NoError(err, "Get request failed")
	s.Equal(http.StatusOK, res.status, "%s: a refused delete should leave the connection in place", context)
}

func (s *HTTPConnectionUsagesTestSuite) TestUsagesListTheFlowAndBlockDelete() {
	usages := s.usages(s.connectionID)
	s.Equal(1, *usages.TotalResults, "Only the suite's flow uses this connection")
	s.Require().Len(usages.Usages, 1, "The suite's flow should be the one usage")
	s.Equal("flow", usages.Usages[0].ResourceType, "The usage should be a flow")
	s.Equal(s.flowID, usages.Usages[0].ID, "The usage should be the suite's flow")
	s.Equal(httpConnUsagesFlowName, usages.Usages[0].DisplayName, "The usage should name the flow")
	s.Equal("restrict", usages.Usages[0].BehaviorOnDelete, "A flow should block deletion of its connection")

	s.assertDeleteRefused(s.connectionID, "A connection a flow uses")
}

func (s *HTTPConnectionUsagesTestSuite) TestEachUsingFlowIsCounted() {
	connectionID := s.createConnection("http-conn-usages-count")
	first := s.createFlow(testutils.ExtensionAuthFlow("HTTP Connection Usages Count One",
		"http_conn_usages_count_one", map[string]interface{}{"connectionId": connectionID}))
	second := s.createFlow(testutils.ExtensionAuthFlow("HTTP Connection Usages Count Two",
		"http_conn_usages_count_two", map[string]interface{}{"connectionId": connectionID}))

	usages := s.usages(connectionID)
	s.Equal(2, *usages.TotalResults, "Both flows should be counted")
	var ids []string
	for _, usage := range usages.Usages {
		ids = append(ids, usage.ID)
	}
	s.ElementsMatch([]string{first, second}, ids, "Both flows should be listed")
}

func (s *HTTPConnectionUsagesTestSuite) TestPaddedConnectionIDIsCountedAsAUsage() {
	connectionID := s.createConnection("http-conn-usages-padded")
	flowID := s.createFlow(testutils.ExtensionAuthFlow("HTTP Connection Usages Padded",
		"http_conn_usages_padded", map[string]interface{}{"connectionId": " \t" + connectionID + "  "}))

	usages := s.usages(connectionID)
	s.Equal(1, *usages.TotalResults, "The flow naming the padded ID should be the connection's one usage")
	s.Require().Len(usages.Usages, 1, "The flow naming the padded ID should be listed")
	s.Equal(flowID, usages.Usages[0].ID, "The usage should be the flow naming the padded ID")

	s.assertDeleteRefused(connectionID, "A connection named with padding")
}

func (s *HTTPConnectionUsagesTestSuite) TestDeleteSucceedsOnceNoFlowUsesTheConnection() {
	connectionID := s.createConnection("http-conn-usages-target")
	flowID := s.createFlow(testutils.ExtensionAuthFlow("HTTP Connection Usages Target",
		"http_conn_usages_target", map[string]interface{}{"connectionId": connectionID}))

	usages := s.usages(connectionID)
	s.Require().Equal(1, *usages.TotalResults, "The new flow should be the only usage")
	s.Equal(flowID, usages.Usages[0].ID, "The usage should be the new flow")
	s.assertDeleteRefused(connectionID, "While the flow exists")

	s.Require().NoError(testutils.DeleteFlow(flowID), "Failed to delete the flow")
	usages = s.usages(connectionID)
	s.Equal(0, *usages.TotalResults, "A deleted flow should no longer be a usage")
	s.Empty(usages.Usages, "A deleted flow should no longer be listed")

	res, err := doRequest(http.MethodDelete, httpConnPath+"/"+connectionID, nil)
	s.Require().NoError(err, "Delete request failed")
	s.Require().Equal(http.StatusNoContent, res.status,
		"Delete should succeed once no flow uses the connection: %s", string(res.body))

	res, err = doRequest(http.MethodGet, httpConnPath+"/"+connectionID, nil)
	s.Require().NoError(err, "Get after delete request failed")
	s.Equal(http.StatusNotFound, res.status, "The deleted connection should be gone")
	s.Equal(httpConnNotFoundCode, res.errorCode(), "A read after delete should be the not-found error")
}
