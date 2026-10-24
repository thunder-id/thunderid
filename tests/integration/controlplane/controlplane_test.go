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
	"path/filepath"
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

// The account access policy and the notification defaults are configuration, so a Control Plane
// serves both sections for authoring and export. A policy written here is the one exported.
func (ts *ControlPlaneTestSuite) TestTheAccountAccessAndNotificationSectionsAreServed() {
	for _, name := range []string{"accountAccess", "notification"} {
		status, raw := ts.call(http.MethodGet, "/server-config/"+name, nil, nil)
		ts.Equal(http.StatusOK, status, "%s: %s", name, raw)
	}

	original := ts.writableAccountAccess()
	status, raw := ts.call(http.MethodPut, "/server-config/accountAccess",
		map[string]any{"user": map[string]any{"default": map[string]any{"threshold": 7}}}, nil)
	ts.Require().Equal(http.StatusOK, status, "the policy must be writable here: %s", raw)
	defer ts.restoreAccountAccess(original)

	ts.Equal(7, ts.mergedUserThreshold(), "the written threshold must be in force")

	var exported struct {
		Resources string `json:"resources"`
	}
	status, raw = ts.call(http.MethodPost, "/export", map[string]any{"serverConfigs": []string{"accountAccess"}},
		&exported)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Contains(exported.Resources, "accountAccess", "the export must carry the section")
	ts.Regexp(`threshold"?:\s*7\b`, exported.Resources, "the export must carry the written policy")
}

// A Control Plane validates the policy as a server does: an invalid value is refused and the stored
// section is left as it was.
func (ts *ControlPlaneTestSuite) TestAnInvalidAccountAccessValueIsRefused() {
	original := ts.writableAccountAccess()
	status, raw := ts.call(http.MethodPut, "/server-config/accountAccess",
		map[string]any{"user": map[string]any{"default": map[string]any{"threshold": 7}}}, nil)
	ts.Require().Equal(http.StatusOK, status, "the policy must be writable here: %s", raw)
	defer ts.restoreAccountAccess(original)
	before := ts.writableAccountAccess()

	status, raw = ts.call(http.MethodPut, "/server-config/accountAccess",
		map[string]any{"user": map[string]any{"default": map[string]any{"failureWindowSeconds": -1}}}, nil)
	ts.Equal(http.StatusBadRequest, status, "a negative window must be refused: %s", raw)
	ts.Contains(string(raw), "SCF-1003", "the refusal must be an invalid value")
	ts.JSONEq(string(before), string(ts.writableAccountAccess()), "a refused write must change nothing")
	ts.Equal(7, ts.mergedUserThreshold(), "the stored policy must still be in force")
}

// writableAccountAccess returns the accountAccess section's writable layer.
func (ts *ControlPlaneTestSuite) writableAccountAccess() json.RawMessage {
	ts.T().Helper()
	var layers struct {
		Writable json.RawMessage `json:"writable"`
	}
	status, raw := ts.call(http.MethodGet, "/server-config/accountAccess", nil, &layers)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	if len(layers.Writable) == 0 || string(layers.Writable) == "null" {
		return json.RawMessage(`{}`)
	}
	return layers.Writable
}

// restoreAccountAccess puts the accountAccess section's writable layer back as it was.
func (ts *ControlPlaneTestSuite) restoreAccountAccess(layer json.RawMessage) {
	status, raw := ts.call(http.MethodPut, "/server-config/accountAccess", layer, nil)
	ts.Equal(http.StatusOK, status, "failed to restore the accountAccess section: %s", raw)
}

// mergedUserThreshold returns the user threshold the accountAccess section resolves to.
func (ts *ControlPlaneTestSuite) mergedUserThreshold() int {
	ts.T().Helper()
	var layers struct {
		Merged struct {
			User struct {
				Default struct {
					Threshold int `json:"threshold"`
				} `json:"default"`
			} `json:"user"`
		} `json:"merged"`
	}
	status, raw := ts.call(http.MethodGet, "/server-config/accountAccess", nil, &layers)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	return layers.Merged.User.Default.Threshold
}

// Runs in declarative mode, since outside it the notification service reads no declarative sender.
func (ts *ControlPlaneTestSuite) TestADeclarativeNotificationSectionCanNameADeclarativeSender() {
	const senderID = "cp-declarative-email-sender"
	dir := ts.T().TempDir()
	deployment, err := os.ReadFile(filepath.Join(testutils.GetExtractedProductHome(), "deployment.yaml"))
	ts.Require().NoError(err)
	snapshot := filepath.Join(dir, "deployment.yaml")
	ts.Require().NoError(os.WriteFile(snapshot, deployment, 0o600))
	resourcesPath := filepath.Join(dir, "resources.yaml")
	ts.Require().NoError(os.WriteFile(resourcesPath, []byte(`resource_type: connection
id: `+senderID+`
type: email-smtp
name: Control Plane Declarative Email Sender
host: localhost
port: 2525
fromAddress: noreply@thunderid.test
---
resource_type: server_config
name: notification
value:
  defaultEmailSenderId: `+senderID+`
`), 0o600))
	defer func() {
		ts.Assert().NoError(testutils.UpdateDeploymentConfig(snapshot), "the deployment must be put back")
		ts.Assert().NoError(testutils.RestartServer(), "the Control Plane must start again on the deployment it had")
	}()

	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		"declarative_resources": map[string]interface{}{"enabled": true},
	}))
	ts.Require().NoError(testutils.RestartServerWithResourcesFile(resourcesPath),
		"a declarative section naming a declarative sender must not stop the Control Plane starting")

	var layers struct {
		ReadOnly struct {
			DefaultEmailSenderID string `json:"defaultEmailSenderId"`
		} `json:"readOnly"`
	}
	status, raw := ts.call(http.MethodGet, "/server-config/notification", nil, &layers)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Equal(senderID, layers.ReadOnly.DefaultEmailSenderID, "the declarative layer must name the sender")

	status, raw = ts.call(http.MethodPut, "/server-config/notification", map[string]any{}, nil)
	ts.Equal(http.StatusBadRequest, status, "a write over a declarative sender must be refused: %s", raw)
	ts.Contains(string(raw), "SCF-1003", "a write over a declarative sender must be refused as an invalid value")
}

func (ts *ControlPlaneTestSuite) TestAnUpdateReportsTheDerivedStatus() {
	name := unique("CP Status App")
	appID, err := testutils.CreateApplication(testutils.Application{OUID: ts.ouID, Name: name})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	var updated struct {
		Status json.RawMessage `json:"status"`
	}
	status, raw := ts.call(http.MethodPut, "/applications/"+appID, map[string]any{
		"ouId": ts.ouID, "name": name, "description": "Updated on the Control Plane",
	}, &updated)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.JSONEq(`{"value":"ACTIVE"}`, string(updated.Status), "the update must report the derived status")
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

// A list such as an application's redirect URIs is exported as a list holding one reference, so
// each deployment supplies its own addresses, as many as it has.
func (ts *ControlPlaneTestSuite) TestExportRefersToAListAsOneReference() {
	name := unique("CP List App")
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:         ts.ouID,
		Name:         name,
		ClientID:     unique("cp-list-client"),
		ClientSecret: unique("cp-list-secret"),
		RedirectURIs: []string{"https://cp-list-one.example.com/callback", "https://cp-list-two.example.com/callback"},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	var exported struct {
		Resources string `json:"resources"`
	}
	status, raw := ts.call(http.MethodPost, "/export", map[string]any{"applications": []string{appID}}, &exported)
	ts.Require().Equal(http.StatusOK, status, string(raw))

	variable := strings.ToUpper(strings.NewReplacer(" ", "_", "-", "_").Replace(name))
	ts.Regexp(`redirectUris:\s*\n\s*- var:APPLICATION_`+variable+`_REDIRECT_URIS\s*\n`, exported.Resources)
	ts.NotContains(exported.Resources, "cp-list-one.example.com", "the export carried an address")
	ts.NotContains(exported.Resources, "cp-list-two.example.com", "the export carried an address")
}

// A resource server imported into a Control Plane keeps its identifier as the reference it was
// exported with, and the Control Plane's own export of it refers to the same name, even once the
// resource server is renamed.
func (ts *ControlPlaneTestSuite) TestAnImportedResourceServerKeepsItsIdentifierReference() {
	name := unique("CP Reference Orders")
	rsID := ts.createResourceServer(name, "https://"+unique("cp-reference-orders")+".example.com")

	var exported struct {
		Resources string `json:"resources"`
	}
	status, raw := ts.call(http.MethodPost, "/export", map[string]any{"resourceServers": []string{rsID}}, &exported)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	reference := "var:" + variableName("RESOURCE_SERVER", name, "IDENTIFIER")
	ts.Require().Contains(exported.Resources, "identifier: "+reference)

	ts.Require().NoError(testutils.DeleteResourceServer(rsID))
	var imported importOutcome
	status, raw = ts.call(http.MethodPost, "/import", map[string]any{
		"content": exported.Resources,
		"options": map[string]any{"target": "runtime", "continueOnError": true},
	}, &imported)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().Equal(0, imported.Summary.Failed, string(raw))
	ts.Require().Equal(1, imported.Summary.Imported, string(raw))

	var server struct {
		Identifier string `json:"identifier"`
		Delimiter  string `json:"delimiter"`
	}
	status, raw = ts.call(http.MethodGet, "/resource-servers/"+rsID, nil, &server)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Equal(reference, server.Identifier, "the import resolved a reference a Control Plane has no value for")
	ts.Equal(".", server.Delimiter)

	status, raw = ts.call(http.MethodPut, "/resource-servers/"+rsID, map[string]any{
		"name":       unique("CP Renamed Orders"),
		"identifier": reference,
		"ouId":       ts.ouID,
	}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	status, raw = ts.call(http.MethodPost, "/export", map[string]any{"resourceServers": []string{rsID}}, &exported)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Contains(exported.Resources, "identifier: "+reference, "the export named the identifier again")
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
