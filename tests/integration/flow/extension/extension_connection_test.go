// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	extConnConnectionName = "ext-conn-service"
	extConnFlowName       = "Extension Connection Flow"
	extConnTenantHeader   = "X-Tenant"
)

var (
	extConnOU = testutils.OrganizationUnit{
		Handle:      "ext-conn-ou",
		Name:        "Extension Connection Test OU",
		Description: "OU for extension executor connection tests",
	}

	extConnUserType = testutils.UserType{
		Handle:      "ext_conn_person",
		DisplayName: "Extension Connection Test Person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}

	extConnUser = testutils.User{
		Type: "ext_conn_person",
		Attributes: json.RawMessage(`{
			"username": "extconnuser",
			"password": "ExtConnPass123!"
		}`),
	}

	extConnApp = testutils.Application{
		Name:                      "Extension Connection Test Application",
		Description:               "Application whose login flow calls an extension through a connection",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "ext_conn_client",
		ClientSecret:              "ext_conn_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"ext_conn_person"},
	}
)

// ExtensionConnectionTestSuite covers how an extension step resolves its connection (AC1.1 to AC1.3),
// that it has no inline endpoint, and the connection's usages and delete protection (R3).
type ExtensionConnectionTestSuite struct {
	suite.Suite
	config       *common.TestSuiteConfig
	service      *mockService
	fixtures     *identityFixtures
	connectionID string
	flowID       string
}

func TestExtensionConnectionTestSuite(t *testing.T) {
	suite.Run(t, new(ExtensionConnectionTestSuite))
}

func (ts *ExtensionConnectionTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}
	ts.service = newMockService(seedValues{})
	ts.fixtures = createIdentityFixtures(&ts.Suite, extConnOU, extConnUserType, extConnUser)

	connectionID, err := testutils.CreateHTTPConnection(ts.connection("tenant-a"))
	ts.Require().NoError(err, "Failed to create the HTTP endpoint connection")
	ts.connectionID = connectionID

	flowID, err := testutils.CreateFlow(singleExtensionFlow(extConnFlowName, "ext_conn_flow",
		map[string]interface{}{"connectionId": ts.connectionID}))
	ts.Require().NoError(err, "Failed to create the extension connection flow")
	ts.flowID = flowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	ts.fixtures.createApplication(&ts.Suite, extConnApp, flowID)
}

func (ts *ExtensionConnectionTestSuite) TearDownSuite() {
	ts.fixtures.deleteApplication(&ts.Suite)
	for i := len(ts.config.CreatedFlowIDs) - 1; i >= 0; i-- {
		if err := testutils.DeleteFlow(ts.config.CreatedFlowIDs[i]); err != nil {
			ts.T().Logf("Failed to delete test flow %s during teardown: %v", ts.config.CreatedFlowIDs[i], err)
		}
	}
	if ts.connectionID != "" {
		if err := testutils.DeleteHTTPConnection(ts.connectionID); err != nil {
			ts.T().Logf("Failed to delete HTTP connection during teardown: %v", err)
		}
	}
	ts.fixtures.deleteIdentities(&ts.Suite)
	if ts.service != nil {
		ts.service.close()
	}
}

func (ts *ExtensionConnectionTestSuite) SetupTest() {
	ts.service.reset()
}

func (ts *ExtensionConnectionTestSuite) connection(tenant string) testutils.HTTPConnection {
	return testutils.HTTPConnection{
		Name:      extConnConnectionName,
		URL:       ts.service.connectionURL(),
		TimeoutMS: 5000,
		Headers:   []testutils.HTTPConnectionHeader{{Name: extConnTenantHeader, Value: tenant}},
	}
}

// createFlow creates a flow that is deleted when the test ends.
func (ts *ExtensionConnectionTestSuite) createFlow(name, handle string, extProperties map[string]interface{}) string {
	ts.T().Helper()
	flowID, err := testutils.CreateFlow(singleExtensionFlow(name, handle, extProperties))
	ts.Require().NoError(err, "Failed to create flow %s", handle)
	ts.T().Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow %s: %v", flowID, err)
		}
	})
	return flowID
}

// bindFlow points the suite application at flowID until the test ends. The restore is registered
// first, so it runs even if the update fails, and before the flow's own cleanup deletes it.
func (ts *ExtensionConnectionTestSuite) bindFlow(flowID string) {
	ts.T().Helper()
	ts.T().Cleanup(func() {
		if err := common.UpdateAppConfig(ts.fixtures.appID, ts.flowID, ""); err != nil {
			ts.T().Logf("Failed to restore the application's flow: %v", err)
		}
	})
	ts.Require().NoError(common.UpdateAppConfig(ts.fixtures.appID, flowID, ""),
		"Failed to bind the application to flow %s", flowID)
}

// assertFlowRefused checks that creating flow fails as an invalid executor configuration. A flow
// that is wrongly accepted is deleted again.
func (ts *ExtensionConnectionTestSuite) assertFlowRefused(flow testutils.Flow, context string) {
	ts.T().Helper()
	status, body := apiRequest(&ts.Suite, http.MethodPost, "/flows", flow)
	if status == http.StatusCreated {
		var created struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(body, &created) == nil && created.ID != "" {
			if err := testutils.DeleteFlow(created.ID); err != nil {
				ts.T().Logf("Failed to delete wrongly accepted flow %s: %v", created.ID, err)
			}
		}
	}
	ts.Equal(http.StatusBadRequest, status, "%s should be refused at save: %s", context, string(body))
	ts.Equal(errCodeFlowInvalidExecConfig, errorCode(body), "%s should be an executor config error", context)
}

// assertConfigErrorBeforeAnyCall signs in and checks the extension step ends the flow with the
// configuration error without calling anything.
func (ts *ExtensionConnectionTestSuite) assertConfigErrorBeforeAnyCall() {
	ts.T().Helper()
	result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
	ts.Equal("ERROR", result.FlowStatus, "A step without a usable connection should end the flow: %s", raw)
	ts.Empty(result.Assertion, "A failed flow must not return an assertion")
	ts.Require().NotNil(result.Error, "The flow should report the step's error: %s", raw)
	ts.Equal(errCodeExtensionConfigInvalid, result.Error.Code, "The error should be the configuration error")
	ts.Equal(msgExtensionConfigInvalid, result.Error.Message.DefaultValue,
		"The client should see the configuration error message")
	ts.Empty(ts.service.allCalls(), "No request may be sent when there is no connection to call")
}

func (ts *ExtensionConnectionTestSuite) TestUpdatedConnectionIsUsedWithoutEditingTheFlow() {
	ts.T().Cleanup(func() {
		if err := testutils.UpdateHTTPConnection(ts.connectionID, ts.connection("tenant-a")); err != nil {
			ts.T().Logf("Failed to restore the HTTP connection: %v", err)
		}
	})
	for _, tenant := range []string{"tenant-a", "tenant-b"} {
		ts.service.reset()
		ts.Require().NoError(testutils.UpdateHTTPConnection(ts.connectionID, ts.connection(tenant)),
			"Failed to update the HTTP connection")

		result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
		ts.fixtures.assertSignedIn(&ts.Suite, result, raw)
		calls := ts.service.callsFrom("ext_step")
		ts.Require().Len(calls, 1, "The extension step should call the connection once")
		ts.Equal(http.MethodPost, calls[0].Method, "The call should be a POST")
		ts.Equal(serviceQuery, calls[0].Query, "The call should go to the connection's whole URL")
		ts.Equal(tenant, calls[0].Headers.Get(extConnTenantHeader),
			"The call should carry the connection's current header")
	}
}

func (ts *ExtensionConnectionTestSuite) TestExtensionWithoutConnectionIsRefusedAtSave() {
	for name, properties := range map[string]map[string]interface{}{
		"Absent": nil,
		"Empty":  {"connectionId": ""},
	} {
		ts.Run(name, func() {
			ts.assertFlowRefused(singleExtensionFlow("Extension No Connection "+name+" Flow",
				"ext_conn_none_"+strings.ToLower(name)+"_flow", properties), "A step without a connection")
		})
	}
}

func (ts *ExtensionConnectionTestSuite) TestExtensionWithDeletedConnectionFailsTheStep() {
	goneID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name: "ext-conn-deleted", URL: ts.service.connectionURL(),
	})
	ts.Require().NoError(err, "Failed to create the connection to delete")
	ts.Require().NoError(testutils.DeleteHTTPConnection(goneID), "Failed to delete the unused connection")

	flowID := ts.createFlow("Extension Deleted Connection Flow", "ext_conn_deleted_flow",
		map[string]interface{}{"connectionId": goneID})
	ts.bindFlow(flowID)
	ts.assertConfigErrorBeforeAnyCall()
}

func (ts *ExtensionConnectionTestSuite) TestInlineEndpointPropertiesAreRefused() {
	for property, value := range map[string]interface{}{
		"url":     ts.service.connectionURL(),
		"headers": map[string]interface{}{"X-Inline": "inline"},
		"timeout": 5,
	} {
		ts.Run(property, func() {
			ts.assertFlowRefused(singleExtensionFlow("Extension Inline "+property+" Flow",
				"ext_conn_inline_"+property+"_flow",
				map[string]interface{}{"connectionId": ts.connectionID, property: value}), "An inline "+property)
		})
	}
}

func (ts *ExtensionConnectionTestSuite) TestUsagesListTheFlowAndBlockDelete() {
	usages := getConnectionUsages(&ts.Suite, ts.connectionID)
	ts.Equal(1, *usages.TotalResults, "Only the suite's flow uses this connection")
	ts.Require().Len(usages.Usages, 1, "The suite's flow should be the one usage")
	ts.Equal("flow", usages.Usages[0].ResourceType, "The usage should be a flow")
	ts.Equal(ts.flowID, usages.Usages[0].ID, "The usage should be the suite's flow")
	ts.Equal(extConnFlowName, usages.Usages[0].DisplayName, "The usage should name the flow")
	ts.Equal("restrict", usages.Usages[0].BehaviorOnDelete, "A flow should block deletion of its connection")

	status, body := apiRequest(&ts.Suite, http.MethodDelete, "/connections/http/"+ts.connectionID, nil)
	ts.Equal(http.StatusConflict, status, "Deleting a connection a flow uses should be refused: %s", string(body))
	ts.Equal(errCodeConnectionInUse, errorCode(body), "The error should be the blocking-dependencies error")

	status, body = apiRequest(&ts.Suite, http.MethodGet, "/connections/http/"+ts.connectionID, nil)
	ts.Equal(http.StatusOK, status, "A refused delete should leave the connection in place: %s", string(body))
}

func (ts *ExtensionConnectionTestSuite) TestDeleteSucceedsOnceNoFlowUsesTheConnection() {
	connectionID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name: "ext-conn-usage-target", URL: ts.service.connectionURL(),
	})
	ts.Require().NoError(err, "Failed to create the HTTP connection")
	deleted := false
	ts.T().Cleanup(func() {
		if deleted {
			return
		}
		if err := testutils.DeleteHTTPConnection(connectionID); err != nil {
			ts.T().Logf("Failed to delete HTTP connection %s: %v", connectionID, err)
		}
	})

	flowID, err := testutils.CreateFlow(singleExtensionFlow("Extension Usage Flow", "ext_conn_usage_flow",
		map[string]interface{}{"connectionId": connectionID}))
	ts.Require().NoError(err, "Failed to create the flow that uses the connection")
	flowDeleted := false
	ts.T().Cleanup(func() {
		if flowDeleted {
			return
		}
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow %s: %v", flowID, err)
		}
	})

	usages := getConnectionUsages(&ts.Suite, connectionID)
	ts.Require().Equal(1, *usages.TotalResults, "The new flow should be the only usage")
	ts.Equal(flowID, usages.Usages[0].ID, "The usage should be the new flow")

	status, body := apiRequest(&ts.Suite, http.MethodDelete, "/connections/http/"+connectionID, nil)
	ts.Equal(http.StatusConflict, status, "Delete should be refused while the flow exists: %s", string(body))
	ts.Equal(errCodeConnectionInUse, errorCode(body), "The error should be the blocking-dependencies error")

	ts.Require().NoError(testutils.DeleteFlow(flowID), "Failed to delete the flow")
	flowDeleted = true

	usages = getConnectionUsages(&ts.Suite, connectionID)
	ts.Equal(0, *usages.TotalResults, "A deleted flow should no longer be a usage")
	ts.Empty(usages.Usages, "A deleted flow should no longer be listed")

	status, body = apiRequest(&ts.Suite, http.MethodDelete, "/connections/http/"+connectionID, nil)
	ts.Require().Equal(http.StatusNoContent, status, "Delete should succeed once no flow uses the connection: %s",
		string(body))
	deleted = true

	status, body = apiRequest(&ts.Suite, http.MethodGet, "/connections/http/"+connectionID, nil)
	ts.Equal(http.StatusNotFound, status, "The deleted connection should be gone")
	ts.Equal(errCodeConnectionNotFound, errorCode(body), "A read after delete should be the not-found error")
}
