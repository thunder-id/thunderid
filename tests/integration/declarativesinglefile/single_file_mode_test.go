/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/*
Single-File Declarative Resource Integration Tests

This suite validates that Thunder correctly loads resources from a single
multi-document YAML file passed via the -resources flag on startup.

Setup:
  - The running server is stopped.
  - A temporary resources.yaml (with env var placeholders) is written to disk.
  - The required env vars are injected into the test process before the server
    is started, so the server inherits them via os.Environ().
  - The server is started with -resources=<path>.
  - The admin token is re-obtained against the fresh server.

Teardown:
  - The server is stopped and restarted without a resources file so subsequent
    test packages see the original server state.
  - The admin token is re-obtained.
*/
package declarativesinglefile

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// resourcesYAML is the fixture loaded by the server under test. It contains:
//   - Two identity_provider documents (one references an env var)
//   - One organization_unit document
//
// The {{ t(...) }} expression in the GitHub IDP description verifies that
// non-Go-template expressions inside the file are preserved as-is and do not
// cause parse errors during env-var substitution.
const resourcesYAML = `resource_type: organization_unit
id: sf-decl-ou-1
handle: sf-decl-ou-1
name: Single File Declarative OU
description: Organization unit loaded via single-file mode
---
resource_type: identity_provider
id: sf-decl-idp-1
name: Single File GitHub IDP
description: "{{ t(idp.github.description) }}"
type: GITHUB
properties:
  - name: client_id
    value: {{.SF_TEST_GITHUB_CLIENT_ID}}
    isSecret: false
  - name: client_secret
    value: sf-test-github-secret
    isSecret: true
  - name: redirect_uri
    value: https://localhost:8095/callback
    isSecret: false
---
resource_type: identity_provider
id: sf-decl-idp-2
name: Single File Google IDP
description: IDP loaded via single-file mode
type: GOOGLE
properties:
  - name: client_id
    value: sf-test-google-client
    isSecret: false
  - name: client_secret
    value: sf-test-google-secret
    isSecret: true
  - name: redirect_uri
    value: https://localhost:8095/callback
    isSecret: false
`

// envVars are set in the test process before the server starts so they are inherited
// via os.Environ() when the server binary is exec'd.
var envVars = map[string]string{
	"SF_TEST_GITHUB_CLIENT_ID": "sf-github-client-id-substituted",
}

type SingleFileModeSuite struct {
	suite.Suite
	resourcesFile string
	originalEnv   map[string]*string // nil pointer means the key was absent before the test
}

func TestSingleFileModeSuite(t *testing.T) {
	suite.Run(t, new(SingleFileModeSuite))
}

func (s *SingleFileModeSuite) SetupSuite() {
	// Snapshot original env values so TearDownSuite can restore them exactly.
	s.originalEnv = make(map[string]*string, len(envVars))
	for k := range envVars {
		if v, exists := os.LookupEnv(k); exists {
			val := v
			s.originalEnv[k] = &val
		} else {
			s.originalEnv[k] = nil
		}
	}

	// Write the fixture YAML to a temp file in the OS temp dir.
	tmpDir, err := os.MkdirTemp("", "thunder-sf-test-*")
	s.Require().NoError(err, "failed to create temp dir for resources fixture")

	s.resourcesFile = filepath.Join(tmpDir, "resources.yaml")
	s.Require().NoError(
		os.WriteFile(s.resourcesFile, []byte(resourcesYAML), 0600),
		"failed to write resources fixture",
	)

	// Inject env vars so the server process inherits them.
	for k, v := range envVars {
		s.Require().NoError(os.Setenv(k, v), "failed to set env var %s", k)
	}

	// Restart the server with the -resources flag.
	s.Require().NoError(
		testutils.RestartServerWithResourcesFile(s.resourcesFile),
		"failed to restart server with resources file",
	)

	// Re-obtain admin token since the server process was replaced.
	s.Require().NoError(
		testutils.ObtainAdminAccessToken(),
		"failed to obtain admin access token after server restart",
	)
}

func (s *SingleFileModeSuite) TearDownSuite() {
	// Restore env vars to their original values (unset those that were absent before the test).
	for k, orig := range s.originalEnv {
		if orig == nil {
			_ = os.Unsetenv(k)
		} else {
			_ = os.Setenv(k, *orig)
		}
	}

	// Restore the server to its original state (no resources file).
	s.Require().NoError(
		testutils.RestartServer(),
		"failed to restore server after single-file mode test",
	)

	// Re-obtain admin token for subsequent test packages.
	s.Require().NoError(
		testutils.ObtainAdminAccessToken(),
		"failed to re-obtain admin token after server restore",
	)

	// Best-effort: remove the temp resources file.
	if s.resourcesFile != "" {
		_ = os.RemoveAll(filepath.Dir(s.resourcesFile))
	}
}

// TestOrganizationUnitLoadedFromFile verifies that the organization_unit declared in the
// single resources.yaml is visible via the REST API.
func (s *SingleFileModeSuite) TestOrganizationUnitLoadedFromFile() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/organization-units/sf-decl-ou-1", testutils.TestServerURL))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode, "declarative OU from single-file should be visible")

	var ou map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&ou))
	s.Equal("sf-decl-ou-1", ou["id"], "OU id should match")
	s.Equal("Single File Declarative OU", ou["name"], "OU name should match")
}

// TestIdentityProviderWithEnvVarLoadedFromFile verifies that the IDP whose client_id
// uses {{.SF_TEST_GITHUB_CLIENT_ID}} had the env var substituted correctly.
func (s *SingleFileModeSuite) TestIdentityProviderWithEnvVarLoadedFromFile() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/identity-providers/sf-decl-idp-1", testutils.TestServerURL))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode, "GitHub IDP from single-file should be visible")

	var idp map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&idp))
	s.Equal("sf-decl-idp-1", idp["id"])
	s.Equal("Single File GitHub IDP", idp["name"])
	s.Equal("GITHUB", idp["type"])

	// Verify the env var was substituted in the client_id property.
	properties, ok := idp["properties"].([]interface{})
	s.Require().True(ok, "IDP should have properties")

	clientIDFound := false
	for _, raw := range properties {
		prop, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if prop["name"] == "client_id" {
			clientIDFound = true
			s.Equal(
				envVars["SF_TEST_GITHUB_CLIENT_ID"],
				prop["value"],
				"client_id should contain the substituted env var value",
			)
		}
	}
	s.True(clientIDFound, "client_id property should be present in IDP")
}

// TestSecondIdentityProviderLoadedFromFile verifies that a second IDP in the same file
// is also correctly loaded.
func (s *SingleFileModeSuite) TestSecondIdentityProviderLoadedFromFile() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/identity-providers/sf-decl-idp-2", testutils.TestServerURL))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode, "Google IDP from single-file should be visible")

	var idp map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&idp))
	s.Equal("sf-decl-idp-2", idp["id"])
	s.Equal("GOOGLE", idp["type"])
}

// TestAllIDPsFromFileAppearInListing verifies that both declarative IDPs appear in the
// collection endpoint.
func (s *SingleFileModeSuite) TestAllIDPsFromFileAppearInListing() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/identity-providers", testutils.TestServerURL))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var idps []map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&idps))

	idpIDs := make([]string, 0, len(idps))
	for _, idp := range idps {
		if id, ok := idp["id"].(string); ok {
			idpIDs = append(idpIDs, id)
		}
	}

	s.Contains(idpIDs, "sf-decl-idp-1", "listing should include sf-decl-idp-1")
	s.Contains(idpIDs, "sf-decl-idp-2", "listing should include sf-decl-idp-2")
}

// TestDeclarativeResourcesAreImmutable verifies that a resource loaded from the single
// file cannot be deleted (declarative resources are read-only).
func (s *SingleFileModeSuite) TestDeclarativeResourcesAreImmutable() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/identity-providers/sf-decl-idp-1", testutils.TestServerURL), nil)
	s.Require().NoError(err)

	resp, err := client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Equal(http.StatusBadRequest, resp.StatusCode,
		"deleting a declarative IDP should return 400 Bad Request")
}
