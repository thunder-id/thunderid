// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package controlplane tests the Control Plane binary (cmd/cpserver): what it does differently from
// the all-in-one server. It runs only under make test_integration_cp, which swaps that binary into
// the distribution and trusts the suite's own identity provider for management tokens.
package controlplane

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type ControlPlaneTestSuite struct {
	suite.Suite
	ouID string
}

func TestControlPlaneTestSuite(t *testing.T) {
	suite.Run(t, new(ControlPlaneTestSuite))
}

func (ts *ControlPlaneTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: unique("cp-suite"),
		Name:   "Control Plane suite",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID
}

func (ts *ControlPlaneTestSuite) TearDownSuite() {
	_ = testutils.DeleteOrganizationUnit(ts.ouID)
}

// unique returns a name no other run of the suite uses.
func unique(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// send sends a request with the given client and returns the status and body.
func (ts *ControlPlaneTestSuite) send(client *http.Client, method, path, contentType string,
	body []byte) (int, []byte) {
	ts.T().Helper()
	req, err := http.NewRequest(method, testutils.TestServerURL+path, bytes.NewReader(body))
	ts.Require().NoError(err)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, raw
}

// call sends a JSON request as the administrator and decodes a successful answer into out.
func (ts *ControlPlaneTestSuite) call(method, path string, in, out any) (int, []byte) {
	ts.T().Helper()
	var body []byte
	if in != nil {
		encoded, err := json.Marshal(in)
		ts.Require().NoError(err)
		body = encoded
	}
	status, raw := ts.send(testutils.GetHTTPClient(), method, path, "application/json", body)
	if out != nil && status < http.StatusBadRequest {
		ts.Require().NoError(json.Unmarshal(raw, out), string(raw))
	}
	return status, raw
}

// A Control Plane serves no runtime traffic: the OAuth2 and flow execution endpoints a data plane
// answers are not there.
func (ts *ControlPlaneTestSuite) TestRuntimeEndpointsAreNotServed() {
	for _, path := range []string{"/oauth2/token", "/oauth2/authorize", "/flow/execute"} {
		status, _ := ts.send(testutils.GetHTTPClient(), http.MethodPost, path,
			"application/x-www-form-urlencoded", []byte("grant_type=client_credentials"))
		ts.Equal(http.StatusNotFound, status, path)
	}
}

// A token from the trusted issuer is accepted only when it carries the system scope.
func (ts *ControlPlaneTestSuite) TestATokenWithoutTheSystemScopeIsRefused() {
	unscoped := os.Getenv(testutils.UnscopedTokenEnv)
	ts.Require().NotEmpty(unscoped, "the harness exports a token without the system scope")

	status, _ := ts.send(testutils.GetHTTPClientWithToken(unscoped), http.MethodGet, "/applications", "", nil)
	ts.Equal(http.StatusForbidden, status)

	status, _ = ts.call(http.MethodGet, "/applications", nil, nil)
	ts.Equal(http.StatusOK, status, "the scoped token was refused too")
}

// A token naming the trusted issuer but signed with another key is refused: the issuer's JWKS is
// what decides.
func (ts *ControlPlaneTestSuite) TestATokenTheTrustedIssuerDidNotSignIsRefused() {
	forger, err := testutils.NewMockOIDCServer(0, "", "")
	ts.Require().NoError(err)
	now := time.Now()
	forged, err := forger.SignJWT(map[string]interface{}{"typ": "at+jwt"}, map[string]interface{}{
		"iss":   fmt.Sprintf("http://localhost:%d", testutils.TrustedIssuerPort),
		"aud":   testutils.TrustedIssuerAudience,
		"sub":   "control-plane-admin",
		"scope": "system",
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	ts.Require().NoError(err)

	status, _ := ts.send(testutils.GetHTTPClientWithToken(forged), http.MethodGet, "/applications", "", nil)
	ts.Equal(http.StatusUnauthorized, status)
}

// A Control Plane holds no values for the deployments it configures, so its export refers to each
// value by name instead of carrying it, and an import of that export back into it keeps the
// references as they are.
func (ts *ControlPlaneTestSuite) TestExportRefersToValuesAndImportKeepsTheReferences() {
	name := unique("CP Reference App")
	clientID := unique("cp-reference-client")
	clientSecret := unique("cp-reference-secret")
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:         ts.ouID,
		Name:         name,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURIs: []string{"https://cp-reference.example.com/callback"},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	var exported struct {
		Resources string `json:"resources"`
	}
	status, raw := ts.call(http.MethodPost, "/export", map[string]any{"applications": []string{appID}}, &exported)
	ts.Require().Equal(http.StatusOK, status, string(raw))

	variable := strings.ToUpper(strings.NewReplacer(" ", "_", "-", "_").Replace(name))
	clientIDRef := "var:APPLICATION_" + variable + "_CLIENT_ID"
	clientSecretRef := "sec:APPLICATION_" + variable + "_CLIENT_SECRET"
	ts.Contains(exported.Resources, clientIDRef)
	ts.Contains(exported.Resources, clientSecretRef)
	ts.NotContains(exported.Resources, clientID, "the export carried the client ID")
	ts.NotContains(exported.Resources, clientSecret, "the export carried the client secret")

	ts.Require().NoError(testutils.DeleteApplication(appID))
	var imported struct {
		Summary struct {
			Imported int `json:"imported"`
			Failed   int `json:"failed"`
		} `json:"summary"`
		Results []struct {
			ResourceID string `json:"resourceId"`
			Status     string `json:"status"`
		} `json:"results"`
	}
	status, raw = ts.call(http.MethodPost, "/import", map[string]any{
		"content": exported.Resources,
		"options": map[string]any{"target": "runtime", "continueOnError": true},
	}, &imported)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().Equal(0, imported.Summary.Failed, string(raw))
	ts.Require().Equal(1, imported.Summary.Imported, string(raw))

	var app struct {
		InboundAuthConfig []struct {
			Config struct {
				ClientID string `json:"clientId"`
			} `json:"config"`
		} `json:"inboundAuthConfig"`
	}
	status, raw = ts.call(http.MethodGet, "/applications/"+appID, nil, &app)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().NotEmpty(app.InboundAuthConfig)
	ts.Equal(clientIDRef, app.InboundAuthConfig[0].Config.ClientID,
		"the import resolved a reference a Control Plane has no value for")
}

// A Control Plane runs no executor, so a flow is validated against the static executor catalog:
// a known executor is accepted and an unknown one refused.
func (ts *ControlPlaneTestSuite) TestFlowsAreValidatedAgainstTheExecutorCatalog() {
	flowID, err := testutils.CreateIsolatedAuthFlow(unique("cp-catalog-flow"))
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteFlow(flowID) }()

	status, raw := ts.call(http.MethodPost, "/flows", map[string]any{
		"name":     "Unknown executor flow",
		"flowType": "AUTHENTICATION",
		"handle":   unique("cp-unknown-executor"),
		"nodes": []map[string]any{
			{"id": "start", "type": "START", "onSuccess": "task"},
			{"id": "task", "type": "TASK_EXECUTION", "executor": map[string]any{"name": "NoSuchExecutor"},
				"onSuccess": "auth_assert"},
			{"id": "auth_assert", "type": "TASK_EXECUTION",
				"executor": map[string]any{"name": "AuthAssertExecutor"}, "onSuccess": "end"},
			{"id": "end", "type": "END"},
		},
	}, nil)
	ts.Require().Equal(http.StatusBadRequest, status, string(raw))
	var refusal struct {
		Code        string `json:"code"`
		Description struct {
			DefaultValue string `json:"defaultValue"`
		} `json:"description"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &refusal))
	ts.Equal("FLM-1023", refusal.Code, string(raw))
	ts.Contains(refusal.Description.DefaultValue, "is not registered", "the flow was refused for another reason")
}
