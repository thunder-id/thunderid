// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	auditConfigKey = "audit"
	logConfigKey   = "log"

	correlationIDHeader = "X-Correlation-ID"
	testUserAgent       = "audit-integration-test/1.0"
	auditLinePrefix     = `{"log_type":"AUDIT","schema_version":1,`

	// quietPeriod is how long a suite waits for an audit line that must not be written.
	quietPeriod = 2 * time.Second
)

var (
	// lineTimePattern is the time format of an audit line: UTC with exactly six fractional digits.
	lineTimePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$`)

	// lineKeys are the top-level keys of an event line that changed a resource, in the order written. A
	// test calls the server from a loopback address, which is not a public peer, so a line has no client.
	lineKeys = []string{
		"log_type", "schema_version", "id", "time", "event", "trace_id", "actor", "subject", "target", "action",
		"outcome",
	}
)

// AuditLogTestSuite drives user, application, role and group changes over the management API against a running
// server and reads the audit lines it writes to standard output. The server writes its output to the
// server log only when this test process starts it, so the suite restarts it first, and every scenario
// that changes the configuration restores it before the next one.
type AuditLogTestSuite struct {
	suite.Suite
	reader         *testutils.AuditLogReader
	originalAudit  interface{}
	originalLog    interface{}
	configModified bool
	// prefix namespaces every handle this run creates, so a leftover from an earlier run is invisible.
	prefix string
}

func TestAuditLogTestSuite(t *testing.T) {
	suite.Run(t, new(AuditLogTestSuite))
}

func (ts *AuditLogTestSuite) SetupSuite() {
	ts.prefix = fmt.Sprintf("audit-%d-", time.Now().UnixNano())

	originalAudit, err := testutils.ReadDeploymentConfigKey(auditConfigKey)
	ts.Require().NoError(err, "failed to read the audit config")
	ts.originalAudit = originalAudit
	originalLog, err := testutils.ReadDeploymentConfigKey(logConfigKey)
	ts.Require().NoError(err, "failed to read the log config")
	ts.originalLog = originalLog

	ts.Require().NoError(testutils.RestartServer(), "failed to restart the server so its output is captured")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "failed to re-obtain the admin token after restart")
	ts.reader = testutils.NewAuditLogReader()
}

// SetupTest puts back the configuration a previous scenario changed.
func (ts *AuditLogTestSuite) SetupTest() {
	if ts.configModified {
		ts.restartWith(ts.originalAudit, ts.originalLog)
		ts.configModified = false
	}
	ts.Require().NoError(ts.reader.Reset())
}

// TearDownSuite restores the configuration read in SetupSuite. A missing key is written back as null,
// which the server treats the same as an absent key.
func (ts *AuditLogTestSuite) TearDownSuite() {
	if !ts.configModified {
		return
	}
	ts.Assert().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		auditConfigKey: ts.originalAudit,
		logConfigKey:   ts.originalLog,
	}), "teardown: failed to restore the audit and log config")
	ts.Assert().NoError(testutils.RestartServer(), "teardown: server did not restart after the config restore")
	ts.Assert().NoError(testutils.ObtainAdminAccessToken(), "teardown: failed to re-obtain the admin token")
}

// Creating, updating and deleting a user through the management API writes one line each, with the
// marker first, the admin as actor, the user as target and subject, no client for the loopback peer
// and the trace ID of the response. The password the user is created with appears in no line.
func (ts *AuditLogTestSuite) TestAUserLifecycleWritesOneLinePerChange() {
	ouID, userType := ts.createUserType("lifecycle")
	adminID := ts.adminUserID()
	username := ts.prefix + "lifecycle"
	password := "AuditLifecycle123!"

	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/users", fmt.Sprintf(
		`{"ouId":%q,"type":%q,"attributes":{"username":%q,"password":%q}}`, ouID, userType, username, password))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	var user testutils.User
	ts.Require().NoError(json.Unmarshal(body, &user))
	ts.Require().NotEmpty(user.ID)
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(user.ID) })
	updated := ts.send(http.MethodPut, "/users/"+user.ID, fmt.Sprintf(
		`{"ouId":%q,"type":%q,"attributes":{"username":%q}}`, ouID, userType, username+"-renamed"))
	ts.Require().Equal(http.StatusOK, updated.StatusCode)
	deleted := ts.send(http.MethodDelete, "/users/"+user.ID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	userRef := testutils.AuditRef{Type: "user", ID: user.ID}
	lines := ts.linesOf(created, updated, deleted)
	for i, tc := range []struct {
		event  string
		action string
	}{
		{"user.create", "create"},
		{"user.update", "update"},
		{"user.delete", "delete"},
	} {
		line := lines[i]
		ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
		ts.Equal(lineKeys, ts.keys(line.Raw), tc.event)
		ts.Regexp(lineTimePattern, line.Time)
		ts.NotEmpty(line.ID)
		ts.Equal(tc.event, line.Event)
		ts.Equal(tc.action, line.Action)
		ts.Equal(userRef, line.Target, tc.event)
		ts.Equal(userRef, line.Subject, tc.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
		ts.Nil(line.Client, "%s: a loopback peer is not recorded", tc.event)
		ts.NotContains(line.Raw, testUserAgent, "%s: the user agent is recorded only with a public peer", tc.event)
		ts.NotContains(line.Raw, password, "a credential is never recorded")
		ts.NotContains(line.Raw, "AuditLifecycle", "no part of a credential is recorded")
	}
	ts.Contains(ts.accessLogLine(created.Header.Get(correlationIDHeader)), "/users",
		"the access log line of the request carries the same trace ID")
}

// An update of a user that does not exist reaches the user service, which records the failed attempt.
func (ts *AuditLogTestSuite) TestAnUpdateOfAMissingUserIsRecordedAsFailed() {
	missing := unusedID()
	ouID, userType := ts.createUserType("missing")

	updated := ts.send(http.MethodPut, "/users/"+missing, fmt.Sprintf(
		`{"ouId":%q,"type":%q,"attributes":{"username":"nobody"}}`, ouID, userType))
	ts.Require().Equal(http.StatusNotFound, updated.StatusCode)

	line := ts.linesOf(updated)[0]
	ts.Equal("user.update", line.Event)
	ts.Equal(testutils.AuditRef{Type: "user", ID: missing}, line.Target)
	ts.Equal(testutils.AuditRef{}, line.Subject, "a failed attempt may name no user, so it has no subject")
	ts.Equal(map[string]string{"result": "failed", "reason": "usr_1003"}, line.Outcome)
}

// A user manager scoped to one organization unit passes the security layer with system:user, and the
// user service refuses its delete of a user in a sibling unit, which is recorded as denied. The user
// exists, so it is the subject.
func (ts *AuditLogTestSuite) TestADeleteTheUserServiceRefusesIsRecordedAsDenied() {
	siblingOU, siblingType := ts.createUserType("sibling")
	siblingUserID := ts.createUser(siblingOU, siblingType, ts.prefix+"sibling", "AuditSibling123!")
	managerID, managerClient := ts.createScopedUserManager("scoped")

	refused := ts.sendWith(managerClient, http.MethodDelete, "/users/"+siblingUserID, "")
	ts.Require().Equal(http.StatusForbidden, refused.StatusCode)

	line := ts.linesOf(refused)[0]
	siblingRef := testutils.AuditRef{Type: "user", ID: siblingUserID}
	ts.Equal("user.delete", line.Event)
	ts.Equal(testutils.AuditRef{Type: "user", ID: managerID}, line.Actor)
	ts.Equal(siblingRef, line.Target)
	ts.Equal(siblingRef, line.Subject, "a denied delete names a user that exists")
	ts.Equal(map[string]string{"result": "denied", "reason": "forbidden"}, line.Outcome)
}

// Every path that creates or updates a user records its attempt, whether the attempt succeeded, failed
// or was denied. A succeeded or denied attempt on a user that exists has that user as its subject; a
// failed attempt, and a create that made no user, has none. No line carries a credential.
func (ts *AuditLogTestSuite) TestEveryCreateAndUpdatePathRecordsItsOutcome() {
	ouID, userType := ts.createUserType("paths")
	ouHandle := ts.prefix + "paths-ou"
	siblingOU, siblingType := ts.createUserType("paths-sibling")
	managerID, managerClient := ts.createScopedUserManager("paths-scoped")
	managerRef := testutils.AuditRef{Type: "user", ID: managerID}
	noUser := testutils.AuditRef{Type: "user"}
	const password = "AuditPathSecret123!"

	cases := []struct {
		name    string
		client  *http.Client
		method  string
		path    string
		body    string
		status  int
		event   string
		outcome map[string]string
		// target is the expected target; created takes it from the response instead.
		target  testutils.AuditRef
		created bool
		subject bool
	}{
		{
			name: "an attribute update of the caller", client: managerClient, method: http.MethodPut,
			path:   "/users/me",
			body:   fmt.Sprintf(`{"attributes":{"username":%q}}`, ts.prefix+"paths-scoped-renamed"),
			status: http.StatusOK, event: "user.update", outcome: map[string]string{"result": "succeeded"},
			target: managerRef, subject: true,
		},
		{
			name: "an attribute update that sets a credential", client: managerClient, method: http.MethodPut,
			path:   "/users/me",
			body:   fmt.Sprintf(`{"attributes":{"password":%q}}`, password),
			status: http.StatusBadRequest, event: "user.update",
			outcome: map[string]string{"result": "failed", "reason": "usr_1001"}, target: managerRef,
		},
		{
			name: "a create under an organization unit path", client: testutils.GetHTTPClient(),
			method: http.MethodPost, path: "/users/tree/" + ouHandle,
			body: fmt.Sprintf(`{"type":%q,"attributes":{"username":%q,"password":%q}}`,
				userType, ts.prefix+"paths-by-path", password),
			status: http.StatusCreated, event: "user.create", outcome: map[string]string{"result": "succeeded"},
			created: true, subject: true,
		},
		{
			name: "a create under an organization unit path that does not exist", client: testutils.GetHTTPClient(),
			method: http.MethodPost, path: "/users/tree/" + ts.prefix + "no-such-ou",
			body: fmt.Sprintf(`{"type":%q,"attributes":{"username":%q,"password":%q}}`,
				userType, ts.prefix+"paths-orphan", password),
			status: http.StatusNotFound, event: "user.create",
			outcome: map[string]string{"result": "failed", "reason": "usr_1005"}, target: noUser,
		},
		{
			name: "a create the user type's schema rejects", client: testutils.GetHTTPClient(),
			method: http.MethodPost, path: "/users",
			body: fmt.Sprintf(`{"ouId":%q,"type":%q,"attributes":{"username":42,"password":%q}}`,
				ouID, userType, password),
			status: http.StatusBadRequest, event: "user.create",
			outcome: map[string]string{"result": "failed", "reason": "usr_1019"}, target: noUser,
		},
		{
			name: "a create in a unit the caller does not manage", client: managerClient, method: http.MethodPost,
			path: "/users",
			body: fmt.Sprintf(`{"ouId":%q,"type":%q,"attributes":{"username":%q,"password":%q}}`,
				siblingOU, siblingType, ts.prefix+"paths-intruder", password),
			status: http.StatusForbidden, event: "user.create",
			outcome: map[string]string{"result": "denied", "reason": "forbidden"}, target: noUser,
		},
	}

	responses := make([]*http.Response, len(cases))
	createdIDs := make([]string, len(cases))
	for i, tc := range cases {
		response, body := ts.exchange(tc.client, tc.method, tc.path, tc.body)
		ts.Require().Equal(tc.status, response.StatusCode, "%s: %s", tc.name, string(body))
		responses[i] = response
		if tc.created {
			var user testutils.User
			ts.Require().NoError(json.Unmarshal(body, &user))
			ts.Require().NotEmpty(user.ID)
			ts.T().Cleanup(func() { _ = testutils.DeleteUser(user.ID) })
			createdIDs[i] = user.ID
		}
	}

	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		target := tc.target
		if tc.created {
			target = testutils.AuditRef{Type: "user", ID: createdIDs[i]}
		}
		ts.Equal(tc.event, line.Event, tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		ts.Equal(target, line.Target, tc.name)
		if tc.subject {
			ts.Equal(target, line.Subject, tc.name)
		} else {
			ts.Equal(testutils.AuditRef{}, line.Subject, tc.name)
		}
		if tc.client == managerClient {
			ts.Equal(managerRef, line.Actor, tc.name)
		}
		ts.NotContains(line.Raw, password, "%s: a credential is never recorded", tc.name)
		ts.NotContains(line.Raw, "AuditPathSecret", "%s: no part of a credential is recorded", tc.name)
	}
}

// An import that creates a user writes the user service's event for the user and nothing else: the
// lookup that decides between update and create writes no line, and the import itself is not audited.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfTheUserItCreates() {
	ouID, userType := ts.createUserType("import")
	userID := unusedID()
	password := "AuditImport123!"
	request, err := json.Marshal(testutils.ImportRequest{
		Content: fmt.Sprintf("resource_type: user\nid: %s\ntype: %s\nouId: %s\nattributes:\n  username: %s\n"+
			"credentials:\n  password: %s\n", userID, userType, ouID, ts.prefix+"imported", password),
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	ts.Require().NoError(err)

	imported, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(userID) })
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Equal(1, result.Summary.Imported, string(body))

	line := ts.linesOf(imported)[0]
	userRef := testutils.AuditRef{Type: "user", ID: userID}
	ts.Equal("user.create", line.Event)
	ts.Equal(userRef, line.Target)
	ts.Equal(userRef, line.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
	ts.NotContains(line.Raw, password, "a credential is never recorded")
}

// An ID is cut to the characters a line holds for it, so a caller cannot grow a line without bound. A
// request from the loopback peer has no client, so a long user agent it sends is not recorded at all;
// the 256 character cap on a public peer's user agent is covered by the client middleware unit tests.
func (ts *AuditLogTestSuite) TestATargetIDIsCutToItsCapAndALoopbackUserAgentIsNotRecorded() {
	const idCap = 128
	longID := strings.Repeat("0123456789", 20)
	longUserAgent := strings.Repeat("ü", 512)
	cases := []struct {
		name, userAgent, id, wantID string
	}{
		{name: "a long multibyte user agent", userAgent: longUserAgent, id: unusedID()},
		{name: "an ID over the cap", userAgent: testUserAgent, id: longID, wantID: longID[:idCap]},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		response, body := ts.exchangeAs(testutils.GetHTTPClient(), tc.userAgent, http.MethodDelete,
			"/users/"+tc.id, "")
		ts.Require().Equal(http.StatusNotFound, response.StatusCode, "%s: %s", tc.name, string(body))
		responses[i] = response
	}

	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		wantID := tc.wantID
		if wantID == "" {
			wantID = tc.id
		}
		ts.Nil(line.Client, tc.name)
		ts.NotContains(line.Raw, `"client"`, "%s: a line from a loopback peer has no client key", tc.name)
		ts.NotContains(line.Raw, "ü", "%s: no part of the user agent is recorded", tc.name)
		ts.NotContains(line.Raw, testUserAgent, "%s: no part of the user agent is recorded", tc.name)
		ts.Equal(testutils.AuditRef{Type: "user", ID: wantID}, line.Target, tc.name)
		ts.Equal("user.delete", line.Event, tc.name)
		ts.Equal(adminRef, line.Actor, tc.name)
		ts.Equal(map[string]string{"result": "failed", "reason": "usr_1003"}, line.Outcome, tc.name)
	}
}

// A token an application got by exchanging the admin's token, recording itself as the actor in the act
// claim (RFC 8693), names the application as the actor and the admin as the principal it acted for.
func (ts *AuditLogTestSuite) TestADelegatedCallNamesTheActorAndWhoItActedFor() {
	clientID, clientSecret := ts.prefix+"delegate", "AuditDelegateSecret123!"
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID: ts.createOrganizationUnit("delegate"),
		Name: "Audit Delegate " + ts.prefix,
		InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": map[string]interface{}{
			"clientId":                clientID,
			"clientSecret":            clientSecret,
			"grantTypes":              []string{"urn:ietf:params:oauth:grant-type:token-exchange"},
			"tokenEndpointAuthMethod": "client_secret_basic",
			"includeActClaim":         true,
		}}},
	})
	ts.Require().NoError(err, "failed to create the exchanging application")
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)

	form := url.Values{
		"grant_type":         {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token":      {adminToken},
		"subject_token_type": {"urn:ietf:params:oauth:token-type:access_token"},
		"resource":           {testutils.SystemResourceIdentifier},
	}
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/oauth2/token",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().NoError(resp.Body.Close())
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))
	var token struct {
		AccessToken string `json:"access_token"`
	}
	ts.Require().NoError(json.Unmarshal(body, &token))
	ts.Require().NotEmpty(token.AccessToken)

	// A 404 rather than a 401 or 403 shows the delegated token passed the security layer.
	delegated := ts.sendWith(testutils.GetHTTPClientWithToken(token.AccessToken), http.MethodDelete,
		"/users/"+unusedID(), "")
	ts.Require().Equal(http.StatusNotFound, delegated.StatusCode)

	line := ts.linesOf(delegated)[0]
	ts.Equal(testutils.AuditRef{Type: "agent", ID: appID}, line.Actor, "the act claim names the actor")
	ts.Equal(testutils.AuditRef{Type: "user", ID: ts.adminUserID()}, line.OnBehalfOf,
		"the token subject is the principal the actor acted for")
}

// The level and format of diagnostic logging do not affect audit lines.
func (ts *AuditLogTestSuite) TestAuditLinesIgnoreTheLogLevelAndFormat() {
	ts.restartWith(ts.originalAudit, map[string]interface{}{
		"level": "error",
		"output": map[string]interface{}{
			"console": map[string]interface{}{"enabled": true},
			"file":    map[string]interface{}{"format": "json"},
		},
	})
	ouID, userType := ts.createUserType("quiet")

	updated := ts.send(http.MethodPut, "/users/"+unusedID(), fmt.Sprintf(
		`{"ouId":%q,"type":%q,"attributes":{"username":"nobody"}}`, ouID, userType))
	ts.Require().Equal(http.StatusNotFound, updated.StatusCode)

	line := ts.linesOf(updated)[0]
	ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
	ts.Equal("user.update", line.Event)
}

// With audit off a user change still succeeds and is still in the access log, and writes no audit line.
func (ts *AuditLogTestSuite) TestAuditOffWritesNoLine() {
	ts.restartWith(map[string]interface{}{"enabled": false}, ts.originalLog)
	ouID, userType := ts.createUserType("off")

	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/users", fmt.Sprintf(
		`{"ouId":%q,"type":%q,"attributes":{"username":%q}}`, ouID, userType, ts.prefix+"off"))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	var user testutils.User
	ts.Require().NoError(json.Unmarshal(body, &user))
	deleted := ts.send(http.MethodDelete, "/users/"+user.ID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	ts.Contains(ts.accessLogLine(deleted.Header.Get(correlationIDHeader)), "/users/"+user.ID,
		"the access log line is written with audit off")
	ts.Never(func() bool {
		lines, err := ts.reader.Read()
		return err != nil || len(lines) > 0
	}, quietPeriod, 100*time.Millisecond, "an audit line was written with audit off")
}

// restartWith restarts the server with the audit and log sections replaced, and reads only what it
// writes from then on.
func (ts *AuditLogTestSuite) restartWith(audit, log interface{}) {
	ts.configModified = true
	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		auditConfigKey: audit,
		logConfigKey:   log,
	}), "failed to patch the audit and log config")
	ts.Require().NoError(testutils.RestartServer(), "failed to restart the server")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "failed to re-obtain the admin token")
	ts.Require().NoError(ts.reader.Reset())
}

// send makes a management call as the admin.
func (ts *AuditLogTestSuite) send(method, path, body string) *http.Response {
	ts.T().Helper()
	return ts.sendWith(testutils.GetHTTPClient(), method, path, body)
}

// sendWith makes a management call with client.
func (ts *AuditLogTestSuite) sendWith(client *http.Client, method, path, body string) *http.Response {
	ts.T().Helper()
	resp, _ := ts.exchange(client, method, path, body)
	return resp
}

// exchange makes a management call with client and returns the response and its body.
func (ts *AuditLogTestSuite) exchange(client *http.Client, method, path, body string) (*http.Response, []byte) {
	ts.T().Helper()
	return ts.exchangeAs(client, testUserAgent, method, path, body)
}

// exchangeAs makes a management call with client under userAgent and returns the response and its body.
func (ts *AuditLogTestSuite) exchangeAs(client *http.Client, userAgent, method, path, body string) (
	*http.Response, []byte) {
	ts.T().Helper()
	var reader io.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	ts.Require().NoError(err)
	req.Header.Set("User-Agent", userAgent)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	read, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().NoError(resp.Body.Close())
	return resp, read
}

// keys returns the top-level keys of a JSON line in the order they were written.
func (ts *AuditLogTestSuite) keys(raw string) []string {
	decoder := json.NewDecoder(strings.NewReader(raw))
	_, err := decoder.Token()
	ts.Require().NoError(err)
	var keys []string
	for decoder.More() {
		token, err := decoder.Token()
		ts.Require().NoError(err)
		keys = append(keys, token.(string))
		var skip json.RawMessage
		ts.Require().NoError(decoder.Decode(&skip))
	}
	return keys
}

// accessLogLine waits for the access log line carrying traceID and returns it.
func (ts *AuditLogTestSuite) accessLogLine(traceID string) string {
	ts.T().Helper()
	ts.Require().NotEmpty(traceID)
	var found string
	ts.Require().Eventually(func() bool {
		data, err := os.ReadFile(testutils.ServerLogPath())
		if err != nil {
			return false
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, traceID) && !strings.HasPrefix(line, `{"log_type"`) {
				found = line
				return true
			}
		}
		return false
	}, 30*time.Second, 250*time.Millisecond, "no access log line carries trace ID %s", traceID)
	return found
}

// linesOf returns the audit line each call that got one of responses wrote, in order, after checking
// each wrote exactly one.
func (ts *AuditLogTestSuite) linesOf(responses ...*http.Response) []testutils.AuditLine {
	ts.T().Helper()
	lines := make([]testutils.AuditLine, 0, len(responses))
	for i, written := range ts.linesPerCall(responses...) {
		ts.Require().Len(written, 1, "the call with trace ID %s did not write exactly one line",
			responses[i].Header.Get(correlationIDHeader))
		lines = append(lines, written[0])
	}
	return lines
}

// linesPerCall returns the audit lines each call that got one of responses wrote, in the order written.
// It first deletes a user that does not exist, a call that always writes a line, and waits for that line
// alone: the server publishes a call's events before it responds and writes them in the order published,
// so by then every line of the earlier calls is written too, and one read counts them all without
// waiting out lines that must not appear.
func (ts *AuditLogTestSuite) linesPerCall(responses ...*http.Response) [][]testutils.AuditLine {
	ts.T().Helper()
	fence := ts.send(http.MethodDelete, "/users/"+unusedID(), "")
	ts.Require().Equal(http.StatusNotFound, fence.StatusCode)
	fenceTraceID := fence.Header.Get(correlationIDHeader)
	ts.Require().NotEmpty(fenceTraceID)
	_, err := ts.reader.Await(func(line testutils.AuditLine) bool { return line.TraceID == fenceTraceID })
	ts.Require().NoError(err, "the fence call wrote no audit line")

	written, err := ts.reader.Read()
	ts.Require().NoError(err)
	byTraceID := make(map[string][]testutils.AuditLine, len(written))
	for _, line := range written {
		byTraceID[line.TraceID] = append(byTraceID[line.TraceID], line)
	}
	perCall := make([][]testutils.AuditLine, 0, len(responses))
	for _, response := range responses {
		traceID := response.Header.Get(correlationIDHeader)
		ts.Require().NotEmpty(traceID)
		perCall = append(perCall, byTraceID[traceID])
	}
	return perCall
}

// unusedID returns an ID no resource of this or an earlier run has.
func unusedID() string {
	return fmt.Sprintf("01900000-0000-7000-8000-%012x", time.Now().UnixNano()&0xffffffffffff)
}

// adminUserID returns the ID of the user the admin token was issued to.
func (ts *AuditLogTestSuite) adminUserID() string {
	ts.T().Helper()
	resp, body := ts.exchange(testutils.GetHTTPClient(), http.MethodGet, "/users/me", "")
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))
	var admin testutils.User
	ts.Require().NoError(json.Unmarshal(body, &admin))
	ts.Require().NotEmpty(admin.ID)
	return admin.ID
}

// createOrganizationUnit creates an organization unit, removed when the test ends, and returns its ID.
func (ts *AuditLogTestSuite) createOrganizationUnit(name string) string {
	ts.T().Helper()
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.prefix + name + "-ou",
		Name:   "Audit Log Test OU " + name,
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(ouID) })
	return ouID
}

// createUserTypeIn creates a user type in ouID with a username and a password, removed when the test
// ends, and returns its handle.
func (ts *AuditLogTestSuite) createUserTypeIn(ouID, name string) string {
	ts.T().Helper()
	userType := testutils.UserType{
		Handle:      ts.prefix + name + "-person",
		DisplayName: "Audit Log Test Person " + name,
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}
	userTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(userTypeID) })
	return userType.Handle
}

// createUserType creates an organization unit and a user type in it, and returns the unit's ID and the
// type's handle. name keeps the handles of two calls in one run apart.
func (ts *AuditLogTestSuite) createUserType(name string) (string, string) {
	ts.T().Helper()
	ouID := ts.createOrganizationUnit(name)
	return ouID, ts.createUserTypeIn(ouID, name)
}

// createUser creates a user, removed when the test ends, and returns its ID.
func (ts *AuditLogTestSuite) createUser(ouID, userType, username, password string) string {
	ts.T().Helper()
	attributes, err := json.Marshal(map[string]interface{}{"username": username, "password": password})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{OUID: ouID, Type: userType, Attributes: attributes})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(userID) })
	return userID
}

// createScopedUserManager creates a user manager in an organization unit of its own, named by name, who
// holds system:user through a role, all removed when the test ends. It returns the manager's ID and a
// client that calls with the manager's token.
func (ts *AuditLogTestSuite) createScopedUserManager(name string) (string, *http.Client) {
	ts.T().Helper()
	manager := ts.createScopedManager(name, []string{"user", "usertype"},
		[]string{"system:user", "system:usertype:view"})
	return manager.id, manager.client
}

// scopedManager is a manager whose permissions reach only its own organization unit.
type scopedManager struct {
	id               string
	ouID             string
	resourceServerID string
	client           *http.Client
}

// createScopedManager creates a manager in an organization unit of its own, named by name, who holds
// permissions through a role on a resource server with a resource for each of handles, all removed when
// the test ends. Its client calls with the manager's token.
func (ts *AuditLogTestSuite) createScopedManager(name string, handles, permissions []string) scopedManager {
	ts.T().Helper()
	ownOU, ownType := ts.createUserType(name)

	username := ts.prefix + name
	password := "AuditManager123!"
	managerID := ts.createUser(ownOU, ownType, username, password)

	identifier := "https://" + ts.prefix + name + ".audit.example.com/user"
	rsID, err := testutils.CreateSystemScopedResourceServer(ownOU, "Audit Test RS "+ts.prefix+name, identifier,
		handles...)
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteResourceServerWithChildren(rsID) })
	// A role name is unique only within its unit, and this unit is the run's own, so the name leaves out
	// ts.prefix: a role name holds at most 50 characters, which PostgreSQL enforces and SQLite does not.
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Audit Manager " + name,
		OUID: ownOU,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: permissions},
		},
		Assignments: []testutils.Assignment{{ID: managerID, Type: "user"}},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteRole(roleID) })

	// A sign-in is bounded to its application's organization unit, so the manager signs in through an
	// application in its own unit rather than the Console.
	var signIn testutils.OUSignInApps
	token, err := signIn.ObtainAccessToken(ownOU, "system "+strings.Join(permissions, " "), username, password,
		identifier)
	ts.T().Cleanup(func() { _ = signIn.DeleteAll() })
	ts.Require().NoError(err)
	ts.Require().NotEmpty(token.AccessToken)
	return scopedManager{
		id: managerID, ouID: ownOU, resourceServerID: rsID, client: testutils.GetHTTPClientWithToken(token.AccessToken),
	}
}
