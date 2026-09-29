// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package clientip

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const trustedProxiesKey = "trusted_proxies"

// trustedLoopback trusts the address the tests connect from, so the server reads the X-Forwarded-For
// they send. The x-real-ip entry matches no test connection; it is there for the header validation.
var trustedLoopback = []map[string]interface{}{
	{"cidr": "127.0.0.1/32"},
	{"cidr": "::1/128"},
	{"cidr": "192.0.2.0/24", "header": "x-real-ip"},
}

// ClientIPTestSuite runs the server behind a trusted proxy and checks the client IP it resolves, as
// recorded in the access log, and that it refuses to start on an invalid trusted proxy.
type ClientIPTestSuite struct {
	suite.Suite
}

func TestClientIPTestSuite(t *testing.T) {
	suite.Run(t, new(ClientIPTestSuite))
}

func (ts *ClientIPTestSuite) SetupSuite() {
	ts.Require().NoError(testutils.PatchServerConfig(trustedProxiesKey, trustedLoopback))
	ts.Require().NoError(testutils.RestartServer(), "the server must start with valid trusted proxies")
}

func (ts *ClientIPTestSuite) TearDownSuite() {
	if err := testutils.PatchServerConfig(trustedProxiesKey, nil); err != nil {
		ts.T().Logf("teardown: failed to remove the trusted proxies: %v", err)
	}
	if err := testutils.RestartServer(); err != nil {
		ts.T().Logf("teardown: server did not restart cleanly after the config restore: %v", err)
	}
	if err := testutils.ObtainAdminAccessToken(); err != nil {
		ts.T().Logf("teardown: failed to re-obtain the admin token: %v", err)
	}
}

func (ts *ClientIPTestSuite) TestClientIPFromTrustedProxy() {
	tests := []struct {
		name string
		xff  []string
		want string
	}{
		{"reads the client from the header", []string{"203.0.113.10"}, "203.0.113.10"},
		{"skips trusted addresses from the right", []string{"203.0.113.11, 127.0.0.1"}, "203.0.113.11"},
		{"ignores addresses left of the client", []string{"garbage, 198.51.100.7, 203.0.113.12"}, "203.0.113.12"},
		{"joins header lines", []string{"203.0.113.13", "127.0.0.1"}, "203.0.113.13"},
	}
	for i, tc := range tests {
		ts.Run(tc.name, func() {
			ts.Equal(tc.want, ts.loggedClientIP(fmt.Sprintf("client-ip-%d-%d", time.Now().UnixNano(), i), tc.xff))
		})
	}
}

func (ts *ClientIPTestSuite) TestFallsBackToConnectingAddress() {
	tests := []struct {
		name string
		xff  []string
	}{
		{"no header", nil},
		{"invalid address before the client", []string{"203.0.113.10, garbage"}},
	}
	for i, tc := range tests {
		ts.Run(tc.name, func() {
			got := ts.loggedClientIP(fmt.Sprintf("client-ip-fallback-%d-%d", time.Now().UnixNano(), i), tc.xff)
			ts.Contains([]string{"127.0.0.1", "::1"}, got, "the connecting address is recorded")
		})
	}
}

// TestRefusesToStartOnInvalidTrustedProxy stops the server, so it restarts it with the valid trusted
// proxies before returning.
func (ts *ClientIPTestSuite) TestRefusesToStartOnInvalidTrustedProxy() {
	defer func() {
		ts.Require().NoError(testutils.PatchServerConfig(trustedProxiesKey, trustedLoopback))
		ts.Require().NoError(testutils.RestartServer())
	}()

	tests := []struct {
		name    string
		entry   map[string]interface{}
		wantErr string
	}{
		{"invalid cidr", map[string]interface{}{"cidr": "10.0.0.5"}, "is not a valid CIDR range"},
		{"invalid header name", map[string]interface{}{"cidr": "10.0.0.0/24", "header": "x real ip"},
			"is not a valid header name"},
		{"forwarded header", map[string]interface{}{"cidr": "10.0.0.0/24", "header": "Forwarded"},
			"is not supported"},
	}
	for _, tc := range tests {
		ts.Run(tc.name, func() {
			before, err := testutils.ReadServerLog()
			ts.Require().NoError(err)
			ts.Require().NoError(testutils.PatchServerConfig(trustedProxiesKey, []map[string]interface{}{tc.entry}))
			ts.Require().NoError(testutils.StartServerExpectingExit(20*time.Second),
				"the server must refuse to start")

			serverLog, err := testutils.ReadServerLog()
			ts.Require().NoError(err)
			// The text log escapes the quotes in the error, so only the unquoted parts are matched.
			startupLog := serverLog[len(before):]
			ts.Contains(startupLog, "invalid server.trusted_proxies[0]")
			ts.Contains(startupLog, tc.wantErr)
		})
	}
}

// loggedClientIP sends a request with the correlation ID and X-Forwarded-For lines, and returns the
// client IP that ends its access log line.
func (ts *ClientIPTestSuite) loggedClientIP(correlationID string, xff []string) string {
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/health/liveness", nil)
	ts.Require().NoError(err)
	req.Header.Set("X-Correlation-ID", correlationID)
	for _, v := range xff {
		req.Header.Add("X-Forwarded-For", v)
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	_ = resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	// The access log line is written after the response, so wait for it.
	var clientIP string
	ts.Require().Eventually(func() bool {
		serverLog, err := testutils.ReadServerLog()
		if err != nil {
			return false
		}
		for _, line := range strings.Split(serverLog, "\n") {
			_, after, found := strings.Cut(line, " "+correlationID+" ")
			if found {
				// The text log quotes the message, so the client IP may carry its closing quote.
				clientIP = strings.TrimSuffix(strings.Fields(after)[0], `"`)
				return true
			}
		}
		return false
	}, 5*time.Second, 100*time.Millisecond, "the request must have an access log line")
	return clientIP
}
