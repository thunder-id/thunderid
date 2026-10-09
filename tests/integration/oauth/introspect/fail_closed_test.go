// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package introspect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	failClosedClientID     = "introspect_fail_closed_client"
	failClosedClientSecret = "introspect_fail_closed_secret"
	failClosedResourceID   = "https://introspect-fail-closed.example.com"

	// unreadableDenyListPath points database.runtime_persistent at a SQLite file that carries only the
	// account runtime table (ENTITY_RUNTIME_DATA), which client authentication reads when it loads
	// the client's entity. Every other runtime-persistent table is missing, so client
	// authentication works and every deny-list read fails at query time. Pointing at a
	// genuinely unopenable target instead is not usable here, because the SSO session service aborts
	// startup when it cannot get a runtime-persistent transactioner. The path has no directory
	// component so it resolves against the server home, which exists under every database profile.
	unreadableDenyListPath = "introspect_fail_closed_no_schema.db"
)

// IntrospectFailClosedTestSuite verifies that token introspection fails closed when the
// runtime-persistent database, which holds the RFC 7009 revocation deny list, cannot be read. If the
// deny list cannot be consulted the server must not claim the token is inactive (that would be a
// silent, unauthenticated denial) and must not claim it is active (that would honor a token whose
// revocation status is unknown). It has to surface a server error instead.
//
// The fail-closed test deliberately breaks the shared deployment: it repoints
// database.runtime_persistent at a database without the revocation tables and restarts the server. While that
// config is in place /oauth2/revoke, the refresh and token-exchange grants, and /oauth2/userinfo are
// broken too, so the test restores the original database section, restarts, and re-obtains the admin
// token from a deferred block that runs even if an assertion aborts the test.
type IntrospectFailClosedTestSuite struct {
	suite.Suite
	client           *http.Client
	ouID             string
	resourceServerID string
	appID            string
	accessToken      string

	// originalDatabaseConfig is the deployment.yaml database section captured before the break, used
	// verbatim to restore it.
	originalDatabaseConfig map[string]interface{}
	configBroken           bool
}

func TestIntrospectFailClosedTestSuite(t *testing.T) {
	suite.Run(t, new(IntrospectFailClosedTestSuite))
}

func (ts *IntrospectFailClosedTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "introspect-fail-closed-ou",
		Name:        "Introspection Fail Closed OU",
		Description: "Organization unit for the introspection fail-closed integration test",
		Parent:      nil,
	})
	ts.Require().NoError(err, "failed to create test organization unit")
	ts.ouID = ouID

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Introspection Fail Closed Resource Server",
		Description: "Resource server for the introspection fail-closed integration test",
		Identifier:  failClosedResourceID,
		OUID:        ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err, "failed to create resource server")
	ts.resourceServerID = resourceServerID

	ts.appID = ts.createApp()

	// Obtain the token while the deployment is still healthy. Tokens are stateless JWTs, so the token
	// stays verifiable across the restart the test performs.
	ts.accessToken = ts.clientCredentialsToken()
}

func (ts *IntrospectFailClosedTestSuite) TearDownSuite() {
	// Safety net. The test restores the config in a deferred block, so this only fires if the break
	// happened and the restore did not.
	if ts.configBroken {
		ts.restoreRuntimePersistentDB()
	}

	if ts.appID != "" {
		deleteURL := fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, ts.appID)
		req, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
		if err == nil {
			resp, doErr := ts.client.Do(req)
			if doErr != nil {
				ts.T().Logf("Failed to delete application: %v", doErr)
			} else {
				resp.Body.Close()
			}
		}
	}
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete resource server: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

// breakRuntimePersistentDB repoints database.runtime_persistent at a schema-less SQLite database and
// restarts the server. Every other database section is preserved verbatim, so the change behaves the
// same under the SQLite and the PostgreSQL integration profiles.
func (ts *IntrospectFailClosedTestSuite) breakRuntimePersistentDB() {
	original, err := testutils.ReadDeploymentConfigKey("database")
	ts.Require().NoError(err, "failed to read the database section of deployment.yaml")
	originalMap, ok := original.(map[string]interface{})
	ts.Require().True(ok, "deployment.yaml has no database section to break")
	ts.originalDatabaseConfig = originalMap

	broken := make(map[string]interface{}, len(originalMap))
	for key, value := range originalMap {
		broken[key] = value
	}
	ts.createDatabaseWithoutDenyList()
	broken["runtime_persistent"] = map[string]interface{}{
		"type": "sqlite",
		"sqlite": map[string]interface{}{
			"path": unreadableDenyListPath,
		},
	}

	ts.configBroken = true
	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{"database": broken}),
		"failed to point runtime_persistent at a schema-less database")
	ts.Require().NoError(testutils.RestartServer(),
		"the server must still boot when the runtime-persistent schema is missing")
}

// createDatabaseWithoutDenyList writes the broken database: the shipped runtime-persistent schema's
// ENTITY_RUNTIME_DATA table and nothing else, through the sqlite3 CLI the harness already requires.
func (ts *IntrospectFailClosedTestSuite) createDatabaseWithoutDenyList() {
	home := testutils.GetExtractedProductHome()
	schema, err := os.ReadFile(filepath.Join(home, "dbscripts", "runtime_persistent", "sqlite.sql"))
	ts.Require().NoError(err, "failed to read the runtime-persistent schema")
	start := strings.Index(string(schema), `CREATE TABLE "ENTITY_RUNTIME_DATA"`)
	ts.Require().GreaterOrEqual(start, 0, "the runtime-persistent schema has no ENTITY_RUNTIME_DATA table")
	end := strings.Index(string(schema[start:]), ");")
	ts.Require().Greater(end, 0, "unterminated ENTITY_RUNTIME_DATA definition")

	dbPath := filepath.Join(home, unreadableDenyListPath)
	_ = os.Remove(dbPath)
	cmd := exec.Command("sqlite3", dbPath)
	cmd.Stdin = strings.NewReader(string(schema[start:start+end+2]) + "\n")
	out, err := cmd.CombinedOutput()
	ts.Require().NoError(err, "failed to create the database without a deny list: %s", string(out))
}

// restoreRuntimePersistentDB puts the captured database section back, restarts, and re-obtains the
// admin token. Every step uses a non-fatal assertion so one failure still lets the others run and the
// shared server is left healthy for the following packages.
func (ts *IntrospectFailClosedTestSuite) restoreRuntimePersistentDB() {
	if !ts.configBroken {
		return
	}
	ts.Assert().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		"database": ts.originalDatabaseConfig,
	}), "failed to restore the database section of deployment.yaml")
	ts.Assert().NoError(testutils.RestartServer(), "failed to restart the server after the config restore")
	ts.Assert().NoError(testutils.ObtainAdminAccessToken(), "failed to re-obtain the admin token after restore")
	ts.configBroken = false
}

// createApp registers the confidential client whose token is introspected.
func (ts *IntrospectFailClosedTestSuite) createApp() string {
	app := map[string]interface{}{
		"name":                      "IntrospectFailClosedApp",
		"description":               "Application for the introspection fail-closed integration test",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                failClosedClientID,
					"clientSecret":            failClosedClientSecret,
					"redirectUris":            []string{"https://localhost:3000"},
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusCreated, resp.StatusCode, "create application failed: %s", string(body))

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &created))
	id, ok := created["id"].(string)
	ts.Require().True(ok, "application id missing from create response")
	return id
}

// clientCredentialsToken obtains an access token for the fail-closed client.
func (ts *IntrospectFailClosedTestSuite) clientCredentialsToken() string {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("resource", failClosedResourceID)

	req, err := http.NewRequest(http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(failClosedClientID, failClosedClientSecret)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusOK, resp.StatusCode, "client_credentials request failed: %s", string(body))

	var parsed map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &parsed))
	token, ok := parsed["access_token"].(string)
	ts.Require().True(ok, "access_token missing from token response")
	return token
}

// introspect posts a single introspection request authenticated with client_secret_basic.
func (ts *IntrospectFailClosedTestSuite) introspect(token string) introspectResult {
	form := url.Values{}
	form.Set("token", token)

	req, err := http.NewRequest(http.MethodPost, introspectEndpoint, strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(failClosedClientID, failClosedClientSecret)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	result := introspectResult{StatusCode: resp.StatusCode, Header: resp.Header, Raw: body}
	if len(body) > 0 {
		ts.Require().NoErrorf(json.Unmarshal(body, &result.Body), "introspection body is not JSON: %s", string(body))
	}
	return result
}

// TestIntrospect_RevocationDenyListUnreadable_FailsClosed asserts that when the revocation deny list
// cannot be read the endpoint answers 500 server_error instead of 200 {"active": false}. The
// distinction is the whole point of failing closed: an unreadable deny list means the revocation
// status is unknown, which is not the same statement as "this token is not valid". The same token is
// asserted to be active before the break and active again after the restore, so the 500 can only be
// attributed to the deny list.
func (ts *IntrospectFailClosedTestSuite) TestIntrospect_RevocationDenyListUnreadable_FailsClosed() {
	baseline := ts.introspect(ts.accessToken)
	ts.Require().Equalf(http.StatusOK, baseline.StatusCode, "baseline introspection body: %s", string(baseline.Raw))
	ts.Require().True(baseline.active(), "the token must introspect as active while the deny list is readable")

	defer func() {
		ts.restoreRuntimePersistentDB()

		restored := ts.introspect(ts.accessToken)
		ts.Assert().Equalf(http.StatusOK, restored.StatusCode,
			"introspection must work again after the restore, body: %s", string(restored.Raw))
		ts.Assert().True(restored.active(), "the token must introspect as active again after the restore")
	}()

	ts.breakRuntimePersistentDB()

	res := ts.introspect(ts.accessToken)
	ts.Require().Equalf(http.StatusInternalServerError, res.StatusCode,
		"an unreadable revocation deny list must surface a server error, body: %s", string(res.Raw))
	ts.Equal("server_error", res.errorCode())
	ts.NotContains(res.Body, "active", "failing closed must not be reported as an inactive token")
}
