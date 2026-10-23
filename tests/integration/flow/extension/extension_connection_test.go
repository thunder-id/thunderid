// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"encoding/json"
	"fmt"
	"net"
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

	// The basic credentials of the connections whose stored rows the tests damage.
	extConnBasicUsername = "ext-conn-svc"
	extConnBasicPassword = "ExtConnSvcPass-7d21"
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
// that it has no inline endpoint, the connection's usages and delete protection (R3), and that a
// stored connection that is missing or damaged fails the step closed without calling the service.
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

// assertFlowAccepted checks that creating flow succeeds, and deletes it when the test ends.
func (ts *ExtensionConnectionTestSuite) assertFlowAccepted(flow testutils.Flow, context string) string {
	ts.T().Helper()
	status, body := apiRequest(&ts.Suite, http.MethodPost, "/flows", flow)
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &created)
	if created.ID != "" {
		ts.T().Cleanup(func() {
			if err := testutils.DeleteFlow(created.ID); err != nil {
				ts.T().Logf("Failed to delete flow %s: %v", created.ID, err)
			}
		})
	}
	ts.Require().Equal(http.StatusCreated, status, "%s should be accepted at save: %s", context, string(body))
	ts.Require().NotEmpty(created.ID, "%s: the created flow should have an ID: %s", context, string(body))
	return created.ID
}

// extensionStepProperties reads the stored properties of ext_step in flowID.
func (ts *ExtensionConnectionTestSuite) extensionStepProperties(flowID string) map[string]interface{} {
	ts.T().Helper()
	status, body := apiRequest(&ts.Suite, http.MethodGet, "/flows/"+flowID, nil)
	ts.Require().Equal(http.StatusOK, status, "The flow should be readable: %s", string(body))
	var flow struct {
		Nodes []struct {
			ID         string                 `json:"id"`
			Properties map[string]interface{} `json:"properties"`
		} `json:"nodes"`
	}
	ts.Require().NoError(json.Unmarshal(body, &flow), "The flow should decode: %s", string(body))
	for _, node := range flow.Nodes {
		if node.ID == "ext_step" {
			return node.Properties
		}
	}
	ts.Require().Fail("The flow should hold ext_step", string(body))
	return nil
}

// deletedConnectionID returns the ID of a connection that existed and is gone.
func (ts *ExtensionConnectionTestSuite) deletedConnectionID() string {
	ts.T().Helper()
	goneID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name: "ext-conn-deleted", URL: ts.service.connectionURL(),
	})
	ts.Require().NoError(err, "Failed to create the connection to delete")
	ts.Require().NoError(testutils.DeleteHTTPConnection(goneID), "Failed to delete the unused connection")
	return goneID
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

func (ts *ExtensionConnectionTestSuite) TestExtensionWithMissingConnectionIsRefusedAtSave() {
	goneID := ts.deletedConnectionID()
	for name, connectionID := range map[string]string{
		"Deleted":       goneID,
		"PaddedDeleted": "  " + goneID + "  ",
		"NeverExisted":  "00000000-0000-0000-0000-00000000c0de",
	} {
		ts.Run(name, func() {
			ts.assertFlowRefused(singleExtensionFlow("Extension Missing Connection "+name+" Flow",
				"ext_conn_missing_"+strings.ToLower(name)+"_flow",
				map[string]interface{}{"connectionId": connectionID}), "A step naming a connection that does not exist")
		})
	}
}

func (ts *ExtensionConnectionTestSuite) TestUpdateToAMissingConnectionIsRefused() {
	flowID := createTestFlow(&ts.Suite, singleExtensionFlow("Extension Update Missing Connection Flow",
		"ext_conn_update_missing_flow", map[string]interface{}{"connectionId": ts.connectionID}))

	status, body := apiRequest(&ts.Suite, http.MethodPut, "/flows/"+flowID,
		singleExtensionFlow("Extension Update Missing Connection Flow", "ext_conn_update_missing_flow",
			map[string]interface{}{"connectionId": ts.deletedConnectionID()}))
	ts.Equal(http.StatusBadRequest, status, "An update naming a missing connection should be refused: %s",
		string(body))
	ts.Equal(errCodeFlowInvalidExecConfig, errorCode(body), "The refusal should be an executor config error")
	ts.Equal(ts.connectionID, ts.extensionStepProperties(flowID)["connectionId"],
		"A refused update must leave the stored flow unchanged")
}

func (ts *ExtensionConnectionTestSuite) TestMalformedPropertiesAreRefusedAtSave() {
	cases := []struct {
		name     string
		property string
		value    interface{}
	}{
		{name: "InputKeysEmpty", property: "inputKeys", value: []string{}},
		{name: "InputKeysBlank", property: "inputKeys", value: []string{" "}},
		{name: "InputKeysNonString", property: "inputKeys", value: []interface{}{1}},
		{name: "InputKeysString", property: "inputKeys", value: "username"},
		{name: "RuntimeDataKeysEmpty", property: "runtimeDataKeys", value: []string{}},
		{name: "RuntimeDataKeysBlank", property: "runtimeDataKeys", value: []string{"clientId", ""}},
		{name: "RuntimeDataKeysNonString", property: "runtimeDataKeys", value: []interface{}{true}},
		{name: "RuntimeDataKeysString", property: "runtimeDataKeys", value: "clientId"},
		{name: "ResponseKeysEmpty", property: "responseKeys", value: []string{}},
		{name: "ResponseKeysBlank", property: "responseKeys", value: []string{"riskScore", " "}},
		{name: "ResponseKeysNonString", property: "responseKeys", value: []interface{}{1}},
		{name: "ResponseKeysBlankString", property: "responseKeys", value: " "},
		{name: "MaxAttemptsZero", property: "maxAttempts", value: 0},
		{name: "MaxAttemptsTwentyOne", property: "maxAttempts", value: 21},
		{name: "MaxAttemptsNegative", property: "maxAttempts", value: -1},
		{name: "MaxAttemptsFraction", property: "maxAttempts", value: 2.5},
		{name: "MaxAttemptsString", property: "maxAttempts", value: "5"},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			ts.assertFlowRefused(singleExtensionFlow("Extension Malformed "+tc.name+" Flow",
				"ext_conn_malformed_"+strings.ToLower(tc.name)+"_flow",
				map[string]interface{}{"connectionId": ts.connectionID, tc.property: tc.value}),
				tc.property+" "+tc.name)
		})
	}

	// A refused update leaves the stored flow as it was.
	flowID := createTestFlow(&ts.Suite, singleExtensionFlow("Extension Malformed Update Flow",
		"ext_conn_malformed_update_flow", map[string]interface{}{"connectionId": ts.connectionID}))
	status, body := apiRequest(&ts.Suite, http.MethodPut, "/flows/"+flowID,
		singleExtensionFlow("Extension Malformed Update Flow", "ext_conn_malformed_update_flow",
			map[string]interface{}{"connectionId": ts.connectionID, "inputKeys": []string{}}))
	ts.Equal(http.StatusBadRequest, status, "An update with an empty inputKeys should be refused: %s", string(body))
	ts.Equal(errCodeFlowInvalidExecConfig, errorCode(body), "The refusal should be an executor config error")
	ts.NotContains(ts.extensionStepProperties(flowID), "inputKeys", "A refused update must leave the flow unchanged")
}

func (ts *ExtensionConnectionTestSuite) TestWellFormedPropertiesAreAcceptedAtSave() {
	// The controls for the refusals: the bounds of maxAttempts and each accepted form of the key lists.
	cases := []struct {
		name     string
		property string
		value    interface{}
	}{
		{name: "MaxAttemptsOne", property: "maxAttempts", value: 1},
		{name: "MaxAttemptsTwenty", property: "maxAttempts", value: 20},
		{name: "InputKeysList", property: "inputKeys", value: []string{"username"}},
		{name: "RuntimeDataKeysList", property: "runtimeDataKeys", value: []string{"clientId",
			extensionKey("ext_risk", "riskScore")}},
		{name: "ResponseKeysList", property: "responseKeys", value: []string{"riskScore"}},
		{name: "ResponseKeysString", property: "responseKeys", value: "riskScore,challengeId"},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			ts.assertFlowAccepted(singleExtensionFlow("Extension Well Formed "+tc.name+" Flow",
				"ext_conn_wellformed_"+strings.ToLower(tc.name)+"_flow",
				map[string]interface{}{"connectionId": ts.connectionID, tc.property: tc.value}),
				tc.property+" "+tc.name)
		})
	}
}

func (ts *ExtensionConnectionTestSuite) TestPaddedConnectionIDIsUsedAndCountedAsAUsage() {
	connectionID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name: "ext-conn-padded-target", URL: ts.service.connectionURL(), TimeoutMS: 5000,
	})
	ts.Require().NoError(err, "Failed to create the HTTP connection")
	// Registered first, so it runs after the flow's cleanup has removed the usage.
	ts.T().Cleanup(func() {
		if err := testutils.DeleteHTTPConnection(connectionID); err != nil {
			ts.T().Logf("Failed to delete HTTP connection %s: %v", connectionID, err)
		}
	})

	flowID := ts.assertFlowAccepted(singleExtensionFlow("Extension Padded Connection Flow",
		"ext_conn_padded_flow", map[string]interface{}{"connectionId": " \t" + connectionID + "  "}),
		"A padded ID of an existing connection")

	usages := getConnectionUsages(&ts.Suite, connectionID)
	ts.Equal(1, *usages.TotalResults, "The flow naming the padded ID should be the connection's one usage")
	ts.Require().Len(usages.Usages, 1, "The flow naming the padded ID should be listed")
	ts.Equal(flowID, usages.Usages[0].ID, "The usage should be the flow naming the padded ID")

	status, body := apiRequest(&ts.Suite, http.MethodDelete, "/connections/http/"+connectionID, nil)
	ts.Equal(http.StatusConflict, status, "A connection named with padding should not be deletable: %s",
		string(body))
	ts.Equal(errCodeConnectionInUse, errorCode(body), "The error should be the blocking-dependencies error")

	// At run time the padded ID resolves to the same connection.
	ts.fixtures.bindFlow(&ts.Suite, flowID)
	result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
	ts.fixtures.assertSignedIn(&ts.Suite, result, raw)
	ts.Len(ts.service.callsFrom("ext_step"), 1, "The step should call the connection its padded ID names")
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

// requireSQLite skips a test that edits the stored connection row, which only SQLite lets the
// harness do.
func (ts *ExtensionConnectionTestSuite) requireSQLite() {
	ts.T().Helper()
	if dbType := testutils.GetDBType(); dbType != "sqlite" {
		ts.T().Skipf("Editing the stored connection needs SQLite; the configuration database is %s", dbType)
	}
}

// basicAuthConnectionFlow creates a connection with basic authentication at the https form of
// serviceURL, which the API requires for a credential, and a flow whose extension step uses it. The
// application is bound to the flow until the test ends.
func (ts *ExtensionConnectionTestSuite) basicAuthConnectionFlow(name, serviceURL string) string {
	ts.T().Helper()
	connectionID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name:      name,
		URL:       "https://" + strings.TrimPrefix(serviceURL, "http://"),
		TimeoutMS: 5000,
		Authentication: &testutils.OutboundAuthentication{Type: "basic", Properties: map[string]string{
			"username": extConnBasicUsername, "password": extConnBasicPassword,
		}},
	})
	ts.Require().NoError(err, "Failed to create the basic-authenticated HTTP connection")
	// Registered first, so it runs after the flow's cleanup has removed the usage.
	ts.T().Cleanup(func() {
		if ts.storedRowCount(connectionID) == "0" {
			return
		}
		if err := testutils.DeleteHTTPConnection(connectionID); err != nil {
			ts.T().Logf("Failed to delete HTTP connection %s: %v", connectionID, err)
		}
	})
	flowID := createTestFlow(&ts.Suite, singleExtensionFlow("Extension "+name+" Flow",
		strings.ReplaceAll(name, "-", "_")+"_flow", map[string]interface{}{"connectionId": connectionID}))
	ts.fixtures.bindFlow(&ts.Suite, flowID)
	return connectionID
}

// storedRowCount returns how many stored rows connectionID has, or "" when the count cannot be read.
func (ts *ExtensionConnectionTestSuite) storedRowCount(connectionID string) string {
	// connectionID is a server-generated UUID, so it is safe to inline.
	count, err := testutils.QueryConfigDB(
		fmt.Sprintf(`SELECT COUNT(*) FROM "HTTP_CONNECTION" WHERE ID = '%s';`, connectionID))
	if err != nil {
		return ""
	}
	return count
}

// editStoredRow runs statement, an UPDATE or DELETE of the HTTP_CONNECTION row with ID $ID, directly
// on the configuration database, past the API's validation, and checks it changed exactly that row.
func (ts *ExtensionConnectionTestSuite) editStoredRow(connectionID, statement string) {
	ts.T().Helper()
	changed, err := testutils.QueryConfigDB(strings.ReplaceAll(statement, "$ID", "'"+connectionID+"'") +
		"; SELECT changes();")
	ts.Require().NoError(err, "Editing the stored connection failed")
	ts.Require().Equal("1", changed, "The edit should change exactly the connection's row")
}

// setStoredURL replaces the stored URL of the connection.
func (ts *ExtensionConnectionTestSuite) setStoredURL(connectionID, url string) {
	ts.T().Helper()
	ts.editStoredRow(connectionID,
		`UPDATE "HTTP_CONNECTION" SET PROPERTIES = json_set(PROPERTIES, '$.url', '`+url+`') WHERE ID = $ID`)
}

// setStoredAuthProperty replaces the stored value of one authentication property, leaving it marked
// secret or not as it was.
func (ts *ExtensionConnectionTestSuite) setStoredAuthProperty(connectionID, name, value string) {
	ts.T().Helper()
	ts.editStoredRow(connectionID, `UPDATE "HTTP_CONNECTION" SET PROPERTIES = json_set(PROPERTIES, (
		SELECT '$.authentication[' || key || '].value' FROM json_each(PROPERTIES, '$.authentication')
		WHERE json_extract(value, '$.name') = '`+name+`'), '`+value+`') WHERE ID = $ID`)
}

// assertFailedClosed checks the step ended the flow with the given error, and that service received
// nothing, so neither the request nor the credential left the server.
func (ts *ExtensionConnectionTestSuite) assertFailedClosed(service *mockService, result *common.FlowStep,
	raw, code, message string) {
	ts.T().Helper()
	ts.Equal("ERROR", result.FlowStatus, "The flow should end in error: %s", raw)
	ts.Empty(result.Assertion, "A step that cannot use its connection must not lead to an assertion")
	ts.Require().NotNil(result.Error, "The flow should report the step's error: %s", raw)
	ts.Equal(code, result.Error.Code, "The flow should end with the step's error code")
	ts.Equal(message, result.Error.Message.DefaultValue, "The flow should end with the step's error message")
	ts.NotContains(raw, extConnBasicPassword, "The stored password must not reach the client")
	ts.Empty(service.allCalls(), "The service must receive no request")
}

func (ts *ExtensionConnectionTestSuite) TestStoredConnectionEditIsUsedOnTheNextCall() {
	ts.requireSQLite()
	// The control for the damaged-row cases: the same edit path, leaving the row valid, is read on the
	// next call, and plain HTTP to loopback still carries the credential.
	connectionID := ts.basicAuthConnectionFlow("ext-conn-stored-control", ts.service.connectionURL())
	ts.setStoredURL(connectionID, ts.service.connectionURL())

	result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
	ts.fixtures.assertSignedIn(&ts.Suite, result, raw)
	calls := ts.service.callsFrom("ext_step")
	ts.Require().Len(calls, 1, "The step should call the URL now stored")
	username, password, ok := (&http.Request{Header: calls[0].Headers}).BasicAuth()
	ts.True(ok, "The call should carry basic authentication")
	ts.Equal(extConnBasicUsername, username, "The call should carry the stored username")
	ts.Equal(extConnBasicPassword, password, "The call should carry the stored password, decrypted")
}

func (ts *ExtensionConnectionTestSuite) TestDamagedStoredConnectionFailsClosed() {
	ts.requireSQLite()
	// Every case but the deleted row first points the row at the plain-HTTP loopback URL the control
	// test proves is called, so only the damage stops the call.
	cases := []struct {
		name    string
		damage  func(connectionID string)
		code    string
		message string
	}{
		{
			name: "RowDeleted",
			damage: func(connectionID string) {
				ts.editStoredRow(connectionID, `DELETE FROM "HTTP_CONNECTION" WHERE ID = $ID`)
			},
			code: errCodeExtensionConfigInvalid, message: msgExtensionConfigInvalid,
		},
		{
			name: "URLUnparseable",
			damage: func(connectionID string) {
				ts.setStoredURL(connectionID, ts.service.server.URL+"/%zz")
			},
			code: errCodeExtensionCallFailed, message: msgExtensionCallFailed,
		},
		{
			name: "SecretUnreadable",
			damage: func(connectionID string) {
				ts.setStoredURL(connectionID, ts.service.connectionURL())
				ts.setStoredAuthProperty(connectionID, "authentication_password", "not-an-encrypted-value")
			},
			code: errCodeExtensionCallFailed, message: msgExtensionCallFailed,
		},
		{
			name: "AuthenticationTypeUnknown",
			damage: func(connectionID string) {
				ts.setStoredURL(connectionID, ts.service.connectionURL())
				ts.setStoredAuthProperty(connectionID, "authentication_type", "kerberos")
			},
			code: errCodeExtensionCallFailed, message: msgExtensionCallFailed,
		},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			ts.service.reset()
			connectionID := ts.basicAuthConnectionFlow("ext-conn-stored-"+strings.ToLower(tc.name),
				ts.service.connectionURL())
			tc.damage(connectionID)

			result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
			ts.assertFailedClosed(ts.service, result, raw, tc.code, tc.message)
		})
	}
}

func (ts *ExtensionConnectionTestSuite) TestCredentialIsNotSentInClearToARemoteHost() {
	ts.requireSQLite()
	// The credential guard lets plain HTTP through to loopback, so the service must answer on an
	// address of this host that is not loopback.
	host := nonLoopbackIPv4()
	if host == "" {
		ts.T().Skip("The host has no non-loopback IPv4 address to serve the remote service on")
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	ts.Require().NoError(err, "Failed to listen on %s", host)
	remote := newMockServiceOn(seedValues{}, listener)
	ts.T().Cleanup(remote.close)

	connectionID := ts.basicAuthConnectionFlow("ext-conn-stored-remote", remote.connectionURL())
	ts.setStoredURL(connectionID, remote.connectionURL())
	result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
	ts.assertFailedClosed(remote, result, raw, errCodeExtensionCallFailed, msgExtensionCallFailed)

	// The control: without the credential the same URL is called, so the refusal above withheld a
	// request that would have reached the service.
	remote.reset()
	ts.Require().NoError(testutils.UpdateHTTPConnection(connectionID, testutils.HTTPConnection{
		Name: "ext-conn-stored-remote", URL: remote.connectionURL(), TimeoutMS: 5000,
		Authentication: &testutils.OutboundAuthentication{Type: "none"},
	}), "Failed to remove the connection's authentication")
	result, raw = ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
	ts.fixtures.assertSignedIn(&ts.Suite, result, raw)
	calls := remote.callsFrom("ext_step")
	ts.Require().Len(calls, 1, "The step should call the remote URL once it carries no credential")
	ts.Empty(calls[0].Headers.Get("Authorization"), "A connection without authentication sends no credential")
}

// nonLoopbackIPv4 returns an IPv4 address of an interface of this host that is up and not loopback
// or link-local, or "" when there is none.
func nonLoopbackIPv4() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ip := ipNet.IP.To4(); ip != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
				return ip.String()
			}
		}
	}
	return ""
}
