// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	propertiesUsername     = "sso_properties_user"
	propertiesLongUsername = "sso_properties_long_ua_user"
	propertiesUserAgent    = "ThunderID-Integration/1.0 (session properties)"
)

// userAgentTransport sets a fixed User-Agent on every request it sends.
type userAgentTransport struct {
	userAgent string
	next      http.RoundTripper
}

// RoundTrip sets the User-Agent on a copy of the request and sends it.
func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", t.userAgent)
	return t.next.RoundTrip(req)
}

// sessionProperties reads the PROPERTIES column of the subject's only SSO session.
func (ts *SSOLogoutTestSuite) sessionProperties(subjectID string) map[string]string {
	ts.T().Helper()

	out, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`SELECT PROPERTIES FROM "SESSION" WHERE SUBJECT_ID = '%s';`, subjectID))
	ts.Require().NoError(err, "failed to read the session properties")
	ts.Require().NotEmpty(out, "the subject must have an SSO session")

	var properties map[string]string
	ts.Require().NoError(json.Unmarshal([]byte(out), &properties), "PROPERTIES must be one JSON document: %s", out)
	return properties
}

func (ts *SSOLogoutTestSuite) TestSSOSession_RecordsClientProperties() {
	subjectID := ts.createUser(propertiesUsername)
	client := ts.newSessionClient()
	client.Transport = &userAgentTransport{userAgent: propertiesUserAgent, next: client.Transport}

	ts.login(client, propertiesUsername, "properties_state_1")

	properties := ts.sessionProperties(subjectID)
	ts.Equal(propertiesUserAgent, properties["userAgent"])
	// The suite trusts no proxy, so the client IP is the address the test connects from.
	loopback := []string{"127.0.0.1", "::1"}
	ts.Contains(loopback, properties["creationIp"])
	ts.Equal(properties["creationIp"], properties["lastActiveIp"])
}

func (ts *SSOLogoutTestSuite) TestSSOSession_TruncatesLongUserAgent() {
	subjectID := ts.createUser(propertiesLongUsername)
	client := ts.newSessionClient()
	// The cap counts characters, so the two-byte "é" is kept whole as the 512th and the rest is cut.
	prefix := strings.Repeat("a", 511)
	client.Transport = &userAgentTransport{userAgent: prefix + "éx", next: client.Transport}

	ts.login(client, propertiesLongUsername, "properties_state_2")

	ts.Equal(prefix+"é", ts.sessionProperties(subjectID)["userAgent"])
}
