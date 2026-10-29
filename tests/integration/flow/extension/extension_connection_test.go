// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"encoding/json"
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

// ExtensionConnectionTestSuite covers how an extension step resolves its connection at run time (AC1.1
// to AC1.3), and that a stored connection that is missing or damaged fails the step closed without
// calling the service. Saving a flow with an extension step is covered in flow/mgt, and the
// connection's usages and delete protection in connection.
type ExtensionConnectionTestSuite struct {
	suite.Suite
	config       *common.TestSuiteConfig
	service      *mockService
	fixtures     *identityFixtures
	connectionID string
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

	flowID, err := testutils.CreateFlow(testutils.ExtensionAuthFlow(extConnFlowName, "ext_conn_flow",
		map[string]interface{}{"connectionId": ts.connectionID}))
	ts.Require().NoError(err, "Failed to create the extension connection flow")
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
		OUID:      ts.fixtures.ouID,
		URL:       ts.service.connectionURL(),
		TimeoutMS: 5000,
		Headers:   []testutils.HTTPConnectionHeader{{Name: extConnTenantHeader, Value: tenant}},
	}
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

func (ts *ExtensionConnectionTestSuite) TestPaddedConnectionIDIsUsed() {
	connectionID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name: "ext-conn-padded-target", OUID: ts.fixtures.ouID, URL: ts.service.connectionURL(), TimeoutMS: 5000,
	})
	ts.Require().NoError(err, "Failed to create the HTTP connection")
	// Registered first, so it runs after the flow's cleanup has removed the usage.
	ts.T().Cleanup(func() {
		if err := testutils.DeleteHTTPConnection(connectionID); err != nil {
			ts.T().Logf("Failed to delete HTTP connection %s: %v", connectionID, err)
		}
	})

	flowID := createTestFlow(&ts.Suite, testutils.ExtensionAuthFlow("Extension Padded Connection Flow",
		"ext_conn_padded_flow", map[string]interface{}{"connectionId": " \t" + connectionID + "  "}))
	ts.fixtures.bindFlow(&ts.Suite, flowID)
	result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
	ts.fixtures.assertSignedIn(&ts.Suite, result, raw)
	ts.Len(ts.service.callsFrom("ext_step"), 1, "The step should call the connection its padded ID names")
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
		OUID:      ts.fixtures.ouID,
		URL:       "https://" + strings.TrimPrefix(serviceURL, "http://"),
		TimeoutMS: 5000,
		Authentication: &testutils.OutboundAuthentication{Type: "basic", Properties: map[string]string{
			"username": extConnBasicUsername, "password": extConnBasicPassword,
		}},
	})
	ts.Require().NoError(err, "Failed to create the basic-authenticated HTTP connection")
	// Registered first, so it runs after the flow's cleanup has removed the usage.
	ts.T().Cleanup(func() {
		if err := testutils.DeleteHTTPConnection(connectionID); err != nil {
			ts.T().Logf("Failed to delete HTTP connection %s: %v", connectionID, err)
		}
	})
	flowID := createTestFlow(&ts.Suite, testutils.ExtensionAuthFlow("Extension "+name+" Flow",
		strings.ReplaceAll(name, "-", "_")+"_flow", map[string]interface{}{"connectionId": connectionID}))
	ts.fixtures.bindFlow(&ts.Suite, flowID)
	return connectionID
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

// assertFailedClosed signs in and checks the step's connection could not be used: a failure inside
// ThunderID, so the request ends with the generic internal error, and service received nothing, so
// neither the request nor the credential left the server.
func (ts *ExtensionConnectionTestSuite) assertFailedClosed(service *mockService) {
	ts.T().Helper()
	body := submitExpectingInternalError(&ts.Suite, ts.fixtures.startLogin(&ts.Suite),
		map[string]string{"username": ts.fixtures.username, "password": ts.fixtures.password}, credentialsAction)
	ts.NotContains(body, extConnBasicPassword, "The stored password must not reach the client")
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
	// test proves is called, so only the damage stops the call. The deleted row is a connection that no
	// longer exists at run time, which the API's delete refusal otherwise prevents.
	cases := []struct {
		name   string
		damage func(connectionID string)
	}{
		{
			name: "RowDeleted",
			damage: func(connectionID string) {
				ts.editStoredRow(connectionID, `DELETE FROM "HTTP_CONNECTION" WHERE ID = $ID`)
			},
		},
		{
			name: "URLUnparseable",
			damage: func(connectionID string) {
				ts.setStoredURL(connectionID, ts.service.server.URL+"/%zz")
			},
		},
		{
			name: "SecretUnreadable",
			damage: func(connectionID string) {
				ts.setStoredURL(connectionID, ts.service.connectionURL())
				ts.setStoredAuthProperty(connectionID, "authentication_password", "not-an-encrypted-value")
			},
		},
		{
			name: "AuthenticationTypeUnknown",
			damage: func(connectionID string) {
				ts.setStoredURL(connectionID, ts.service.connectionURL())
				ts.setStoredAuthProperty(connectionID, "authentication_type", "kerberos")
			},
		},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			ts.service.reset()
			connectionID := ts.basicAuthConnectionFlow("ext-conn-stored-"+strings.ToLower(tc.name),
				ts.service.connectionURL())
			tc.damage(connectionID)
			ts.assertFailedClosed(ts.service)
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
	ts.assertFailedClosed(remote)

	// The control: without the credential the same URL is called, so the refusal above withheld a
	// request that would have reached the service.
	remote.reset()
	ts.Require().NoError(testutils.UpdateHTTPConnection(connectionID, testutils.HTTPConnection{
		Name: "ext-conn-stored-remote", URL: remote.connectionURL(), TimeoutMS: 5000,
		Authentication: &testutils.OutboundAuthentication{Type: "none"},
	}), "Failed to remove the connection's authentication")
	result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
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
