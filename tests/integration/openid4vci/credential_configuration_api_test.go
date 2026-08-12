// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	configAPIBasePath = "/openid4vci/credential-configurations"

	// Handles are prefixed to keep this suite's fixtures distinct from the
	// issuance suite's, which shares the same package and database.
	crudConfigHandle    = "cfg_crud_primary"
	crudConfigAltHandle = "cfg_crud_secondary"
	crudConfigVCT       = "https://credentials.thunderid.local/CrudTestCredential"

	// Error codes returned by the credential configuration management API.
	codeConfigInvalidRequest    = "VCI-2001"
	codeConfigNotFound          = "VCI-2002"
	codeConfigAlreadyExists     = "VCI-2003"
	codeConfigUnsupportedFormat = "VCI-2004"
	codeConfigImmutable         = "VCI-2005"
	codeConfigInvalidOU         = "VCI-2007"

	// Declarative (file-backed) configurations seeded from
	// resources/declarative_resources/credential_configurations. They are
	// immutable: the management API must reject updates and deletes.
	declConfigID     = "decl-credential-config-1"
	declConfigHandle = "decl_credential_config_1"
	declConfigVCT    = "https://credentials.thunderid.local/DeclarativeTestCredential"
)

var crudConfigOU = testutils.OrganizationUnit{
	Handle:      "vci-config-crud-ou",
	Name:        "OpenID4VCI Config CRUD OU",
	Description: "Organization unit for credential configuration CRUD testing",
	Parent:      nil,
}

// CredentialConfigurationAPITestSuite exercises the credential configuration
// management API (CRUD, validation, and error mapping) against the live server.
type CredentialConfigurationAPITestSuite struct {
	suite.Suite
	ouID      string
	createdID []string
}

// TestCredentialConfigurationAPITestSuite is the single entrypoint that runs every Test* method.
func TestCredentialConfigurationAPITestSuite(t *testing.T) {
	suite.Run(t, new(CredentialConfigurationAPITestSuite))
}

func (ts *CredentialConfigurationAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(crudConfigOU)
	ts.Require().NoError(err, "create test OU")
	ts.ouID = ouID
}

func (ts *CredentialConfigurationAPITestSuite) TearDownSuite() {
	for _, id := range ts.createdID {
		if err := testutils.DeleteCredentialConfiguration(id); err != nil {
			ts.T().Logf("Failed to delete credential configuration %s: %v", id, err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

// configRequest issues a request against the configuration API and returns the
// raw status and body so tests can assert on error codes.
func (ts *CredentialConfigurationAPITestSuite) configRequest(
	method, path string, body any,
) *testutils.VCHTTPResult {
	var reader io.Reader
	if body != nil {
		switch b := body.(type) {
		case string:
			reader = bytes.NewBufferString(b)
		default:
			raw, err := json.Marshal(b)
			ts.Require().NoError(err)
			reader = bytes.NewBuffer(raw)
		}
	}

	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return &testutils.VCHTTPResult{StatusCode: resp.StatusCode, Body: raw}
}

// createConfig posts a configuration and registers the created id for cleanup.
func (ts *CredentialConfigurationAPITestSuite) createConfig(
	cfg testutils.CredentialConfiguration,
) (*testutils.VCHTTPResult, map[string]any) {
	res := ts.configRequest(http.MethodPost, configAPIBasePath, cfg)
	if res.StatusCode != http.StatusCreated {
		return res, nil
	}
	parsed := ts.decode(res.Body)
	if id, ok := parsed["id"].(string); ok && id != "" {
		ts.createdID = append(ts.createdID, id)
	}
	return res, parsed
}

// decode parses a JSON object response body.
func (ts *CredentialConfigurationAPITestSuite) decode(body []byte) map[string]any {
	var parsed map[string]any
	ts.Require().NoErrorf(json.Unmarshal(body, &parsed), "decode body: %s", string(body))
	return parsed
}

// errorCodeOf extracts the "code" field from an API error body.
func (ts *CredentialConfigurationAPITestSuite) errorCodeOf(body []byte) string {
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	code, _ := parsed["code"].(string)
	return code
}

// baseConfig returns a valid configuration body owned by the suite's OU.
func (ts *CredentialConfigurationAPITestSuite) baseConfig(handle string) testutils.CredentialConfiguration {
	validity := 7200
	return testutils.CredentialConfiguration{
		Handle:      handle,
		OUID:        ts.ouID,
		Name:        "CRUD Test Credential",
		Description: "Credential configuration for CRUD testing",
		Format:      "dc+sd-jwt",
		VCT:         crudConfigVCT,
		Claims: []testutils.ClaimMapping{
			{Name: "given_name", DisplayName: "Given Name"},
			{Name: "family_name", DisplayName: "Family Name"},
		},
		Display:         &testutils.CredentialDisplay{Locale: "en-US", LogoURI: "https://example.com/logo.png"},
		ValiditySeconds: &validity,
	}
}

// TestCreateAndGet verifies a created configuration round-trips through the
// single-resource read with every field intact.
func (ts *CredentialConfigurationAPITestSuite) TestCreateAndGet() {
	cfg := ts.baseConfig(crudConfigHandle)
	res, created := ts.createConfig(cfg)
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))

	id, _ := created["id"].(string)
	ts.Require().NotEmpty(id, "created id missing")

	getRes := ts.configRequest(http.MethodGet, configAPIBasePath+"/"+id, nil)
	ts.Require().Equalf(http.StatusOK, getRes.StatusCode, "get: %s", string(getRes.Body))

	fetched := ts.decode(getRes.Body)
	ts.Equal(id, fetched["id"])
	ts.Equal(crudConfigHandle, fetched["handle"])
	ts.Equal(crudConfigVCT, fetched["vct"])
	ts.Equal("dc+sd-jwt", fetched["format"])
	ts.Equal(ts.ouID, fetched["ouId"])
	ts.Equal("CRUD Test Credential", fetched["name"])
	ts.Equal("Credential configuration for CRUD testing", fetched["description"])
	ts.Equal(float64(7200), fetched["validitySeconds"])
	// The owning OU handle is resolved for display on read.
	ts.Equal(crudConfigOU.Handle, fetched["ouHandle"])

	claims, ok := fetched["claims"].([]any)
	ts.Require().Truef(ok, "claims missing: %s", string(getRes.Body))
	ts.Equal([]any{
		map[string]any{"name": "given_name", "displayName": "Given Name"},
		map[string]any{"name": "family_name", "displayName": "Family Name"},
	}, claims)

	display, ok := fetched["display"].(map[string]any)
	ts.Require().Truef(ok, "display missing: %s", string(getRes.Body))
	ts.Equal("en-US", display["locale"])
	ts.Equal("https://example.com/logo.png", display["logoUri"])
}

// TestList verifies the list endpoint returns the summary projection including
// the created configuration.
func (ts *CredentialConfigurationAPITestSuite) TestList() {
	cfg := ts.baseConfig("cfg_crud_listed")
	res, created := ts.createConfig(cfg)
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	id, _ := created["id"].(string)

	listRes := ts.configRequest(http.MethodGet, configAPIBasePath, nil)
	ts.Require().Equalf(http.StatusOK, listRes.StatusCode, "list: %s", string(listRes.Body))

	var summaries []map[string]any
	ts.Require().NoErrorf(json.Unmarshal(listRes.Body, &summaries), "decode list: %s", string(listRes.Body))
	ts.Require().NotEmpty(summaries, "list returned no configurations")

	var found map[string]any
	for _, s := range summaries {
		if s["id"] == id {
			found = s
			break
		}
	}
	ts.Require().NotNilf(found, "created configuration missing from list: %s", string(listRes.Body))
	ts.Equal("cfg_crud_listed", found["handle"])
	ts.Equal(crudConfigVCT, found["vct"])
	ts.Equal("dc+sd-jwt", found["format"])
	// The list projection is a summary: per-claim detail is not included.
	ts.NotContains(found, "claims")
	ts.NotContains(found, "validitySeconds")
}

// TestUpdate verifies an update persists and is reflected on a subsequent read.
func (ts *CredentialConfigurationAPITestSuite) TestUpdate() {
	res, created := ts.createConfig(ts.baseConfig("cfg_crud_updatable"))
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	id, _ := created["id"].(string)

	updated := ts.baseConfig("cfg_crud_updated_handle")
	updated.Name = "Renamed Credential"
	updated.Claims = []testutils.ClaimMapping{{Name: "email", DisplayName: "Email"}}

	updateRes := ts.configRequest(http.MethodPut, configAPIBasePath+"/"+id, updated)
	ts.Require().Equalf(http.StatusOK, updateRes.StatusCode, "update: %s", string(updateRes.Body))

	body := ts.decode(updateRes.Body)
	ts.Equal("cfg_crud_updated_handle", body["handle"])
	ts.Equal("Renamed Credential", body["name"])

	getRes := ts.configRequest(http.MethodGet, configAPIBasePath+"/"+id, nil)
	ts.Require().Equal(http.StatusOK, getRes.StatusCode)
	fetched := ts.decode(getRes.Body)
	ts.Equal("cfg_crud_updated_handle", fetched["handle"])
	ts.Equal("Renamed Credential", fetched["name"])
	claims, ok := fetched["claims"].([]any)
	ts.Require().True(ok, "claims missing after update")
	ts.Equal([]any{
		map[string]any{"name": "email", "displayName": "Email"},
	}, claims, "update must replace the claim set, not merge into it")
}

// TestUpdate_SameHandleAllowed verifies updating a configuration while keeping
// its own handle is not treated as a handle conflict.
func (ts *CredentialConfigurationAPITestSuite) TestUpdate_SameHandleAllowed() {
	res, created := ts.createConfig(ts.baseConfig("cfg_crud_same_handle"))
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	id, _ := created["id"].(string)

	same := ts.baseConfig("cfg_crud_same_handle")
	same.Description = "Description changed, handle retained"

	updateRes := ts.configRequest(http.MethodPut, configAPIBasePath+"/"+id, same)
	ts.Require().Equalf(http.StatusOK, updateRes.StatusCode, "update: %s", string(updateRes.Body))
	ts.Equal("Description changed, handle retained", ts.decode(updateRes.Body)["description"])
}

// TestCreate_DefaultsFormat verifies an omitted format defaults to dc+sd-jwt.
func (ts *CredentialConfigurationAPITestSuite) TestCreate_DefaultsFormat() {
	cfg := ts.baseConfig("cfg_crud_default_format")
	cfg.Format = ""

	res, created := ts.createConfig(cfg)
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	ts.Equal("dc+sd-jwt", created["format"])
}

// TestCreate_ResolvesOUByHandle verifies the owning OU can be supplied as a
// handle path instead of an id.
func (ts *CredentialConfigurationAPITestSuite) TestCreate_ResolvesOUByHandle() {
	cfg := ts.baseConfig("cfg_crud_ou_by_handle")
	cfg.OUID = ""
	cfg.OUHandle = crudConfigOU.Handle

	res, created := ts.createConfig(cfg)
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	ts.Equal(ts.ouID, created["ouId"], "OU handle must resolve to the OU id")
}

// TestDelete verifies a deleted configuration is no longer readable, and that
// deleting an absent configuration succeeds idempotently.
func (ts *CredentialConfigurationAPITestSuite) TestDelete() {
	res, created := ts.createConfig(ts.baseConfig("cfg_crud_deletable"))
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	id, _ := created["id"].(string)

	delRes := ts.configRequest(http.MethodDelete, configAPIBasePath+"/"+id, nil)
	ts.Require().Equalf(http.StatusNoContent, delRes.StatusCode, "delete: %s", string(delRes.Body))

	getRes := ts.configRequest(http.MethodGet, configAPIBasePath+"/"+id, nil)
	ts.Equal(http.StatusNotFound, getRes.StatusCode)

	// Deleting an already-absent configuration is idempotent.
	repeatRes := ts.configRequest(http.MethodDelete, configAPIBasePath+"/"+id, nil)
	ts.Equal(http.StatusNoContent, repeatRes.StatusCode)
}

// TestCreate_DuplicateHandle rejects a second configuration reusing a handle.
func (ts *CredentialConfigurationAPITestSuite) TestCreate_DuplicateHandle() {
	first, _ := ts.createConfig(ts.baseConfig(crudConfigAltHandle))
	ts.Require().Equalf(http.StatusCreated, first.StatusCode, "create: %s", string(first.Body))

	dup := ts.baseConfig(crudConfigAltHandle)
	dup.Name = "Duplicate handle attempt"
	res, _ := ts.createConfig(dup)
	ts.Equal(http.StatusConflict, res.StatusCode)
	ts.Equal(codeConfigAlreadyExists, ts.errorCodeOf(res.Body))
}

// TestUpdate_HandleConflict rejects renaming a configuration onto a handle
// another configuration already holds.
func (ts *CredentialConfigurationAPITestSuite) TestUpdate_HandleConflict() {
	occupied := "cfg_crud_occupied"
	first, _ := ts.createConfig(ts.baseConfig(occupied))
	ts.Require().Equalf(http.StatusCreated, first.StatusCode, "create: %s", string(first.Body))

	second, created := ts.createConfig(ts.baseConfig("cfg_crud_renamer"))
	ts.Require().Equalf(http.StatusCreated, second.StatusCode, "create: %s", string(second.Body))
	id, _ := created["id"].(string)

	clash := ts.baseConfig(occupied)
	res := ts.configRequest(http.MethodPut, configAPIBasePath+"/"+id, clash)
	ts.Equal(http.StatusConflict, res.StatusCode)
	ts.Equal(codeConfigAlreadyExists, ts.errorCodeOf(res.Body))
}

// TestCreate_ValidationErrors rejects requests missing required fields or
// carrying unsupported values.
func (ts *CredentialConfigurationAPITestSuite) TestCreate_ValidationErrors() {
	zero := 0
	negative := -10

	cases := []struct {
		name     string
		mutate   func(*testutils.CredentialConfiguration)
		wantCode string
	}{
		{
			name:     "missing handle",
			mutate:   func(c *testutils.CredentialConfiguration) { c.Handle = "" },
			wantCode: codeConfigInvalidRequest,
		},
		{
			name:     "missing vct",
			mutate:   func(c *testutils.CredentialConfiguration) { c.VCT = "" },
			wantCode: codeConfigInvalidRequest,
		},
		{
			name:     "unsupported format",
			mutate:   func(c *testutils.CredentialConfiguration) { c.Format = "jwt_vc_json" },
			wantCode: codeConfigUnsupportedFormat,
		},
		{
			name:     "zero validity",
			mutate:   func(c *testutils.CredentialConfiguration) { c.ValiditySeconds = &zero },
			wantCode: codeConfigInvalidRequest,
		},
		{
			name:     "negative validity",
			mutate:   func(c *testutils.CredentialConfiguration) { c.ValiditySeconds = &negative },
			wantCode: codeConfigInvalidRequest,
		},
		{
			name: "unknown organization unit",
			mutate: func(c *testutils.CredentialConfiguration) {
				c.OUID = "00000000-0000-0000-0000-000000000000"
			},
			wantCode: codeConfigInvalidOU,
		},
		{
			name: "missing organization unit",
			mutate: func(c *testutils.CredentialConfiguration) {
				c.OUID = ""
				c.OUHandle = ""
			},
			wantCode: codeConfigInvalidOU,
		},
		{
			name: "unresolvable organization unit handle",
			mutate: func(c *testutils.CredentialConfiguration) {
				c.OUID = ""
				c.OUHandle = "no-such-ou-handle"
			},
			wantCode: codeConfigInvalidOU,
		},
	}

	for i, tc := range cases {
		ts.Run(tc.name, func() {
			cfg := ts.baseConfig(fmt.Sprintf("cfg_crud_invalid_%d", i))
			tc.mutate(&cfg)

			res, _ := ts.createConfig(cfg)
			ts.Equalf(http.StatusBadRequest, res.StatusCode, "body: %s", string(res.Body))
			ts.Equal(tc.wantCode, ts.errorCodeOf(res.Body))
		})
	}
}

// TestCreate_MalformedBody rejects a request body that is not valid JSON.
func (ts *CredentialConfigurationAPITestSuite) TestCreate_MalformedBody() {
	res := ts.configRequest(http.MethodPost, configAPIBasePath, "{not-json")
	ts.Equal(http.StatusBadRequest, res.StatusCode)
	ts.Equal(codeConfigInvalidRequest, ts.errorCodeOf(res.Body))
}

// TestUnknownID returns not-found for reads, updates, and malformed bodies
// against an id that does not exist.
func (ts *CredentialConfigurationAPITestSuite) TestUnknownID() {
	unknown := configAPIBasePath + "/11111111-2222-3333-4444-555555555555"

	getRes := ts.configRequest(http.MethodGet, unknown, nil)
	ts.Equal(http.StatusNotFound, getRes.StatusCode)
	ts.Equal(codeConfigNotFound, ts.errorCodeOf(getRes.Body))

	putRes := ts.configRequest(http.MethodPut, unknown, ts.baseConfig("cfg_crud_ghost"))
	ts.Equal(http.StatusNotFound, putRes.StatusCode)
	ts.Equal(codeConfigNotFound, ts.errorCodeOf(putRes.Body))

	malformedRes := ts.configRequest(http.MethodPut, unknown, "{not-json")
	ts.Equal(http.StatusBadRequest, malformedRes.StatusCode)
	ts.Equal(codeConfigInvalidRequest, ts.errorCodeOf(malformedRes.Body))
}

// TestDeclarativeVisibility verifies a file-backed configuration is readable
// through the management API in composite mode.
func (ts *CredentialConfigurationAPITestSuite) TestDeclarativeVisibility() {
	res := ts.configRequest(http.MethodGet, configAPIBasePath+"/"+declConfigID, nil)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "get declarative: %s", string(res.Body))

	fetched := ts.decode(res.Body)
	ts.Equal(declConfigID, fetched["id"])
	ts.Equal(declConfigHandle, fetched["handle"])
	ts.Equal(declConfigVCT, fetched["vct"])
	ts.Equal("dc+sd-jwt", fetched["format"])

	ts.Equal("Declarative Test Credential", fetched["name"])
	ts.Equal("A declarative credential configuration", fetched["description"])

	claims, ok := fetched["claims"].([]any)
	ts.Require().Truef(ok, "claims missing: %s", string(res.Body))
	ts.Equal([]any{
		map[string]any{"name": "given_name", "displayName": "Given Name"},
		map[string]any{"name": "family_name", "displayName": "Family Name"},
		map[string]any{"name": "email", "displayName": "Email"},
	}, claims)

	display, ok := fetched["display"].(map[string]any)
	ts.Require().Truef(ok, "display missing: %s", string(res.Body))
	ts.Equal("en-US", display["locale"])
	ts.Equal("https://example.com/declarative-credential.png", display["logoUri"])
	ts.Equal(float64(3600), fetched["validitySeconds"])
}

// TestDeclarativeAppearsInList verifies declarative and runtime configurations
// are merged into a single listing.
func (ts *CredentialConfigurationAPITestSuite) TestDeclarativeAppearsInList() {
	res, created := ts.createConfig(ts.baseConfig("cfg_crud_merged"))
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	runtimeID, _ := created["id"].(string)

	listRes := ts.configRequest(http.MethodGet, configAPIBasePath, nil)
	ts.Require().Equalf(http.StatusOK, listRes.StatusCode, "list: %s", string(listRes.Body))

	var summaries []map[string]any
	ts.Require().NoErrorf(json.Unmarshal(listRes.Body, &summaries), "decode list: %s", string(listRes.Body))

	ids := make(map[string]bool, len(summaries))
	for _, s := range summaries {
		if id, ok := s["id"].(string); ok {
			ts.False(ids[id], "duplicate id %q in merged listing", id)
			ids[id] = true
		}
	}
	ts.True(ids[declConfigID], "declarative configuration missing from list")
	ts.True(ids[runtimeID], "runtime configuration missing from list")
}

// TestDeclarativeUpdateReject verifies a file-backed configuration cannot be
// modified through the management API.
func (ts *CredentialConfigurationAPITestSuite) TestDeclarativeUpdateReject() {
	update := ts.baseConfig(declConfigHandle)
	update.Name = "Attempted rename of declarative configuration"

	res := ts.configRequest(http.MethodPut, configAPIBasePath+"/"+declConfigID, update)
	ts.Equalf(http.StatusConflict, res.StatusCode, "update declarative: %s", string(res.Body))
	ts.Equal(codeConfigImmutable, ts.errorCodeOf(res.Body))

	// The configuration must be unchanged.
	getRes := ts.configRequest(http.MethodGet, configAPIBasePath+"/"+declConfigID, nil)
	ts.Require().Equal(http.StatusOK, getRes.StatusCode)
	ts.Equal("Declarative Test Credential", ts.decode(getRes.Body)["name"])
}

// TestDeclarativeDeleteReject verifies a file-backed configuration cannot be
// deleted through the management API.
func (ts *CredentialConfigurationAPITestSuite) TestDeclarativeDeleteReject() {
	res := ts.configRequest(http.MethodDelete, configAPIBasePath+"/"+declConfigID, nil)
	ts.Equalf(http.StatusConflict, res.StatusCode, "delete declarative: %s", string(res.Body))
	ts.Equal(codeConfigImmutable, ts.errorCodeOf(res.Body))

	// The configuration must still be readable.
	getRes := ts.configRequest(http.MethodGet, configAPIBasePath+"/"+declConfigID, nil)
	ts.Equal(http.StatusOK, getRes.StatusCode)
}

// TestDeclarativeHandleConflict rejects a runtime configuration reusing a
// declarative configuration's handle.
func (ts *CredentialConfigurationAPITestSuite) TestDeclarativeHandleConflict() {
	clash := ts.baseConfig(declConfigHandle)
	res, _ := ts.createConfig(clash)
	ts.Equalf(http.StatusConflict, res.StatusCode, "create clashing handle: %s", string(res.Body))
	ts.Equal(codeConfigAlreadyExists, ts.errorCodeOf(res.Body))
}

// TestDeclarativeAdvertisedInIssuerMetadata verifies a file-backed configuration
// is served by the issuer, not just the management API.
func (ts *CredentialConfigurationAPITestSuite) TestDeclarativeAdvertisedInIssuerMetadata() {
	res, meta, err := testutils.GetVCIIssuerMetadata()
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "metadata: %s", string(res.Body))

	supported, ok := meta["credential_configurations_supported"].(map[string]any)
	ts.Require().Truef(ok, "credential_configurations_supported missing: %s", string(res.Body))

	cfg, ok := supported[declConfigHandle].(map[string]any)
	ts.Require().Truef(ok, "declarative configuration not advertised: %s", string(res.Body))
	ts.Equal(declConfigVCT, cfg["vct"])
	ts.Equal(declConfigHandle, cfg["scope"])
}

// TestUnauthenticated rejects management requests carrying no access token.
func (ts *CredentialConfigurationAPITestSuite) TestUnauthenticated() {
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+configAPIBasePath, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusUnauthorized, resp.StatusCode)
}
