// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// importOutcome is what an import answers with.
type importOutcome struct {
	Summary struct {
		Imported int `json:"imported"`
		Failed   int `json:"failed"`
	} `json:"summary"`
	Results []struct {
		ResourceType string `json:"resourceType"`
		Status       string `json:"status"`
		Code         string `json:"code"`
		Message      string `json:"message"`
	} `json:"results"`
}

// toDataPlane sends a JSON request to the Data Plane as a deployment pipeline does, with the
// management API key, and returns the status and body.
func (ts *ControlPlaneTestSuite) toDataPlane(method, path string, in any) (int, []byte) {
	ts.T().Helper()
	body, err := json.Marshal(in)
	ts.Require().NoError(err)
	req, err := http.NewRequest(method, testutils.DataPlaneURL+path, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(testutils.ManagementAPIKeyHeader, testutils.ManagementAPIKey)
	resp, err := testutils.GetNoRedirectHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, raw
}

// importToDataPlane imports content into the Data Plane's runtime.
func (ts *ControlPlaneTestSuite) importToDataPlane(content string) importOutcome {
	ts.T().Helper()
	status, raw := ts.toDataPlane(http.MethodPost, "/import", map[string]any{
		"content": content,
		"options": map[string]any{"target": "runtime", "continueOnError": true},
	})
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var outcome importOutcome
	ts.Require().NoError(json.Unmarshal(raw, &outcome), string(raw))
	return outcome
}

// What the Control Plane exports is what a Data Plane imports: the Data Plane fills each reference
// from the values it holds, and refuses a resource it holds no value for rather than storing the
// reference text where the value belongs.
func (ts *ControlPlaneTestSuite) TestADataPlaneImportsTheExportWithItsOwnValues() {
	ouHandle := unique("dp-ou")
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ouHandle,
		Name:   "Data Plane suite",
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(ouID) }()

	name := unique("DP Import App")
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:         ouID,
		Name:         name,
		ClientID:     unique("cp-only-client"),
		ClientSecret: unique("cp-only-secret"),
		RedirectURIs: []string{"https://dp-import.example.com/callback"},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	var exported struct {
		Resources string `json:"resources"`
	}
	status, raw := ts.call(http.MethodPost, "/export", map[string]any{
		"organizationUnits": []string{ouID},
		"applications":      []string{appID},
	}, &exported)
	ts.Require().Equal(http.StatusOK, status, string(raw))

	variable := strings.ToUpper(strings.NewReplacer(" ", "_", "-", "_").Replace(name))
	clientIDName := "APPLICATION_" + variable + "_CLIENT_ID"
	clientSecretName := "APPLICATION_" + variable + "_CLIENT_SECRET"
	redirectURIsName := "APPLICATION_" + variable + "_REDIRECT_URIS"

	outcome := ts.importToDataPlane(exported.Resources)
	ts.Equal(1, outcome.Summary.Failed, "an application was imported with no values for its references")
	refused := false
	for _, result := range outcome.Results {
		if result.ResourceType == "application" && result.Status == "failed" {
			refused = true
			ts.Equal("IMP-1006", result.Code, result.Message)
		}
	}
	ts.True(refused, "the application was not the resource refused: %+v", outcome.Results)

	clientID := unique("dp-client")
	clientSecret := unique("dp-secret")
	status, raw = ts.toDataPlane(http.MethodPut, "/variables/"+clientIDName, map[string]any{"value": clientID})
	ts.Require().Less(status, http.StatusBadRequest, string(raw))
	status, raw = ts.toDataPlane(http.MethodPut, "/secrets/"+clientSecretName, map[string]any{"value": clientSecret})
	ts.Require().Less(status, http.StatusBadRequest, string(raw))
	status, raw = ts.toDataPlane(http.MethodPut, "/variables/"+redirectURIsName,
		map[string]any{"value": `["https://dp.example.com/callback"]`})
	ts.Require().Less(status, http.StatusBadRequest, string(raw))

	outcome = ts.importToDataPlane(exported.Resources)
	ts.Require().Equal(0, outcome.Summary.Failed, "%+v", outcome.Results)

	// The Data Plane answers for the client with the values it holds, not the Control Plane's.
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequest(http.MethodPost, testutils.DataPlaneURL+"/oauth2/token",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)
	resp, err := testutils.GetNoRedirectHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	tokenBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, resp.StatusCode, fmt.Sprintf("the client the data plane holds was refused: %s", tokenBody))
}
