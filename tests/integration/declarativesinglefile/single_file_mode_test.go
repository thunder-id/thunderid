// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
//   - Two connection documents (one references an env var)
//   - One organization_unit document
//
// The {{ t(...) }} expression in the GitHub IDP description verifies that
// non-Go-template expressions inside the file are preserved as-is and do not
// cause parse errors during env-var substitution.
//
// The GitHub IDP also carries an attributeConfiguration. Its nested keys are snake_case
// (user_type_resolution, user_type_attribute_mappings, user_type, external_attribute,
// local_attribute) while the outer key and accountLinking are camelCase, because
// providers.AttributeConfiguration tags them inconsistently. That is recorded as G13; camelCase
// nested keys would silently fail to parse. Update this fixture when G13 is fixed.
const resourcesYAML = `resource_type: organization_unit
id: sf-decl-ou-1
handle: sf-decl-ou-1
name: Single File Declarative OU
description: Organization unit loaded via single-file mode
---
resource_type: connection
id: sf-decl-idp-1
name: Single File GitHub IDP
description: "{{ t(idp.github.description) }}"
type: github
clientId: {{.SF_TEST_GITHUB_CLIENT_ID}}
clientSecret: sf-test-github-secret
redirectUri: https://localhost:8095/callback
attributeConfiguration:
  user_type_resolution:
    default: Person
  user_type_attribute_mappings:
    - user_type: Person
      attributes:
        - external_attribute: login
          local_attribute: username
  accountLinking:
    attributes:
      - email
---
resource_type: connection
id: sf-decl-idp-2
name: Single File Google IDP
description: IDP loaded via single-file mode
type: google
clientId: sf-test-google-client
clientSecret: sf-test-google-secret
redirectUri: https://localhost:8095/callback
---
resource_type: connection
id: sf-decl-idp-3
name: Single File Empty Config IDP
description: IDP whose attributeConfiguration is present but empty
type: google
clientId: sf-test-empty-client
clientSecret: sf-test-empty-secret
redirectUri: https://localhost:8095/callback
attributeConfiguration:
  accountLinking: {}
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

// TestIdentityProviderWithEnvVarLoadedFromFile verifies that the IDP whose clientId
// uses {{.SF_TEST_GITHUB_CLIENT_ID}} had the env var substituted correctly.
func (s *SingleFileModeSuite) TestIdentityProviderWithEnvVarLoadedFromFile() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/connections/github/sf-decl-idp-1", testutils.TestServerURL))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode, "GitHub IDP from single-file should be visible")

	var idp map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&idp))
	s.Equal("sf-decl-idp-1", idp["id"])
	s.Equal("Single File GitHub IDP", idp["name"])
	s.Equal("github", idp["type"])

	// Verify the env var was substituted in the typed clientId field.
	s.Equal(
		envVars["SF_TEST_GITHUB_CLIENT_ID"],
		idp["clientId"],
		"clientId should contain the substituted env var value",
	)
}

// TestSecondIdentityProviderLoadedFromFile verifies that a second IDP in the same file
// is also correctly loaded.
func (s *SingleFileModeSuite) TestSecondIdentityProviderLoadedFromFile() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/connections/google/sf-decl-idp-2", testutils.TestServerURL))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode, "Google IDP from single-file should be visible")

	var idp map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&idp))
	s.Equal("sf-decl-idp-2", idp["id"])
	s.Equal("google", idp["type"])
}

// TestAllIDPsFromFileAppearInListing verifies that every declarative IDP in the file appears in the
// collection endpoint.
func (s *SingleFileModeSuite) TestAllIDPsFromFileAppearInListing() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/connections?category=identity-provider", testutils.TestServerURL))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var listResp struct {
		Connections []map[string]interface{} `json:"connections"`
	}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	idpIDs := make([]string, 0, len(listResp.Connections))
	for _, idp := range listResp.Connections {
		if id, ok := idp["id"].(string); ok {
			idpIDs = append(idpIDs, id)
		}
	}

	s.Contains(idpIDs, "sf-decl-idp-1", "listing should include sf-decl-idp-1")
	s.Contains(idpIDs, "sf-decl-idp-2", "listing should include sf-decl-idp-2")
	s.Contains(idpIDs, "sf-decl-idp-3", "listing should include sf-decl-idp-3")
}

// AD1 verifies a full attributeConfiguration declared in the single file is parsed and returned by the
// connection API, including the mapping entries and the account-linking list.
func (s *SingleFileModeSuite) TestAttributeConfigurationLoadedFromFile() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/connections/github/sf-decl-idp-1", testutils.TestServerURL))
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var idp map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&idp))

	config, ok := idp["attributeConfiguration"].(map[string]interface{})
	s.Require().True(ok, "expected attributeConfiguration on the response: %v", idp)

	// The response is camelCase even though the file is snake_case: the wire format follows the json
	// tags while the file is parsed with the yaml ones. See G13.
	resolution, ok := config["userTypeResolution"].(map[string]interface{})
	s.Require().True(ok, "expected userTypeResolution in %v", config)
	s.Equal("Person", resolution["default"])

	linking, ok := config["accountLinking"].(map[string]interface{})
	s.Require().True(ok, "expected accountLinking in %v", config)
	s.Equal([]interface{}{"email"}, linking["attributes"])

	mappings, ok := config["userTypeAttributeMappings"].([]interface{})
	s.Require().True(ok, "expected userTypeAttributeMappings in %v", config)
	s.Require().Len(mappings, 1)
	entry, ok := mappings[0].(map[string]interface{})
	s.Require().True(ok)
	s.Equal("Person", entry["userType"])
	attributes, ok := entry["attributes"].([]interface{})
	s.Require().True(ok)
	s.Require().Len(attributes, 1)
	attribute, ok := attributes[0].(map[string]interface{})
	s.Require().True(ok)
	s.Equal("login", attribute["externalAttribute"])
	s.Equal("username", attribute["localAttribute"])
}

// AD3 verifies that a declared-but-empty configuration section stays distinguishable from an absent one
// after loading. Declarative connections are seeded at startup exactly as REST-created ones are, so an
// omitted accountLinking is filled in while a present-but-empty one is left alone. sf-decl-idp-3 declares
// `accountLinking: {}` and sf-decl-idp-2 declares nothing.
func (s *SingleFileModeSuite) TestEmptyVersusOmittedAttributeConfigurationFromFile() {
	client := testutils.GetHTTPClient()

	resp, err := client.Get(fmt.Sprintf("%s/connections/google/sf-decl-idp-3", testutils.TestServerURL))
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var withEmpty map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&withEmpty))
	emptyConfig, ok := withEmpty["attributeConfiguration"].(map[string]interface{})
	s.Require().True(ok, "a declared-but-empty configuration should survive the load: %v", withEmpty)
	linking, ok := emptyConfig["accountLinking"].(map[string]interface{})
	s.Require().True(ok, "the empty section must remain present rather than being treated as absent")
	s.Empty(linking["attributes"], "a present-but-empty section is not seeded")

	resp2, err := client.Get(fmt.Sprintf("%s/connections/google/sf-decl-idp-2", testutils.TestServerURL))
	s.Require().NoError(err)
	defer resp2.Body.Close()
	s.Require().Equal(http.StatusOK, resp2.StatusCode)

	var withNone map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp2.Body).Decode(&withNone))
	noneConfig, ok := withNone["attributeConfiguration"].(map[string]interface{})
	s.Require().True(ok, "an omitted configuration should be seeded, not left absent: %v", withNone)
	seededLinking, ok := noneConfig["accountLinking"].(map[string]interface{})
	s.Require().True(ok, "an omitted accountLinking should be seeded: %v", noneConfig)

	// Seeding fills the default only for attributes every user type declares unique, so one type allowing
	// duplicate emails disables it deployment-wide. Naming that type here keeps the cause visible instead
	// of surfacing as an unexplained empty list on the connection.
	userTypes, err := testutils.ListUserTypes()
	s.Require().NoError(err, "failed to list user types")
	for _, userType := range userTypes {
		s.Require().True(userType.IsAttributeUnique("email"),
			"seeding precondition: user type %q allows duplicate emails", userType.Name)
	}

	s.Equal([]interface{}{"email"}, seededLinking["attributes"],
		"an omitted section is seeded, which is the distinction the empty one suppresses")
}

// AD6 verifies that masking the declarative clientSecret does not disturb the configuration returned
// alongside it.
func (s *SingleFileModeSuite) TestSecretMaskingLeavesAttributeConfigurationIntactFromFile() {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/connections/github/sf-decl-idp-1", testutils.TestServerURL))
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var idp map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&idp))

	s.Equal("******", idp["clientSecret"], "the declarative secret should be masked")
	config, ok := idp["attributeConfiguration"].(map[string]interface{})
	s.Require().True(ok, "masking must not drop the attributeConfiguration")

	// Presence alone would still pass if masking emptied or rewrote the sections, so the values are
	// compared against what the fixture declares.
	resolution, ok := config["userTypeResolution"].(map[string]interface{})
	s.Require().True(ok, "expected userTypeResolution in %v", config)
	s.Equal("Person", resolution["default"])

	linking, ok := config["accountLinking"].(map[string]interface{})
	s.Require().True(ok, "expected accountLinking in %v", config)
	s.Equal([]interface{}{"email"}, linking["attributes"])

	mappings, ok := config["userTypeAttributeMappings"].([]interface{})
	s.Require().True(ok, "expected userTypeAttributeMappings in %v", config)
	s.Require().Len(mappings, 1)
	entry, ok := mappings[0].(map[string]interface{})
	s.Require().True(ok)
	s.Equal("Person", entry["userType"])
	attributes, ok := entry["attributes"].([]interface{})
	s.Require().True(ok)
	s.Require().Len(attributes, 1)
	attribute, ok := attributes[0].(map[string]interface{})
	s.Require().True(ok)
	s.Equal("login", attribute["externalAttribute"])
	s.Equal("username", attribute["localAttribute"])
}

// TestDeclarativeResourcesAreImmutable verifies that a resource loaded from the single
// file cannot be deleted (declarative resources are read-only).
func (s *SingleFileModeSuite) TestDeclarativeResourcesAreImmutable() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/connections/github/sf-decl-idp-1", testutils.TestServerURL), nil)
	s.Require().NoError(err)

	resp, err := client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Equal(http.StatusBadRequest, resp.StatusCode,
		"deleting a declarative IDP should return 400 Bad Request")
}
