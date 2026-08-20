// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vci

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	configAPIBasePath = "/openid4vci/credential-configurations"

	// Seeded from resources/declarative_resources/credential_configurations, which the
	// server loads at startup because openid4vci.store is composite. Declarative
	// configurations are immutable: the management API must refuse writes to them.
	declConfigID     = "decl-credential-config-1"
	declConfigHandle = "decl_credential_config_1"
	declConfigOUID   = "decl-ou-1"
	declConfigVCT    = "https://credentials.thunderid.local/DeclarativeTestCredential"
)

// CredentialConfigurationDeclarativeSuite covers file-backed credential configurations
// in composite mode: they are readable, immutable, and coexist with runtime ones.
type CredentialConfigurationDeclarativeSuite struct {
	suite.Suite
	ouID      string
	runtimeID string
}

func TestCredentialConfigurationDeclarativeSuite(t *testing.T) {
	suite.Run(t, new(CredentialConfigurationDeclarativeSuite))
}

func (ts *CredentialConfigurationDeclarativeSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "vci-declarative-ou",
		Name:        "OpenID4VCI Declarative OU",
		Description: "Organization unit for declarative credential configuration tests",
	})
	ts.Require().NoError(err, "create test OU")
	ts.ouID = ouID
}

func (ts *CredentialConfigurationDeclarativeSuite) TearDownSuite() {
	if ts.runtimeID != "" {
		ts.NoError(testutils.DeleteCredentialConfiguration(ts.runtimeID))
	}
	if ts.ouID != "" {
		ts.NoError(testutils.DeleteOrganizationUnit(ts.ouID))
	}
}

// request issues an authenticated management-API call and returns the status and raw body.
func (ts *CredentialConfigurationDeclarativeSuite) request(method, path string, body any) (int, []byte) {
	ts.T().Helper()

	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		ts.Require().NoError(err)
		payload = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, testutils.TestServerURL+path, payload)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, raw
}

// object decodes a single-resource response body.
func (ts *CredentialConfigurationDeclarativeSuite) object(raw []byte) map[string]any {
	ts.T().Helper()
	decoded := map[string]any{}
	ts.Require().NoErrorf(json.Unmarshal(raw, &decoded), "body: %s", string(raw))
	return decoded
}

// TestDeclarativeVisibleWithOU verifies a file-backed configuration is readable and
// carries the organization unit declared in its YAML.
func (ts *CredentialConfigurationDeclarativeSuite) TestDeclarativeVisibleWithOU() {
	status, raw := ts.request(http.MethodGet, configAPIBasePath+"/"+declConfigID, nil)
	ts.Require().Equalf(http.StatusOK, status, "get declarative: %s", string(raw))

	body := ts.object(raw)
	ts.Equal(declConfigID, body["id"])
	ts.Equal(declConfigHandle, body["handle"])
	ts.Equal(declConfigVCT, body["vct"])
	ts.Equal("Declarative Test Credential", body["name"])
	ts.Equal(declConfigOUID, body["ouId"], "the ouId declared in YAML must survive the load")
}

// TestDeclarativeAppearsInList verifies declarative and runtime configurations are merged
// into a single listing in composite mode.
func (ts *CredentialConfigurationDeclarativeSuite) TestDeclarativeAppearsInList() {
	status, raw := ts.request(http.MethodGet, configAPIBasePath, nil)
	ts.Require().Equalf(http.StatusOK, status, "list: %s", string(raw))

	var items []map[string]any
	ts.Require().NoErrorf(json.Unmarshal(raw, &items), "list body: %s", string(raw))

	var found bool
	for _, item := range items {
		if item["id"] == declConfigID {
			found = true
		}
	}
	ts.Truef(found, "declarative configuration missing from list: %s", string(raw))
}

// TestDeclarativeUpdateRejected verifies a file-backed configuration cannot be updated.
func (ts *CredentialConfigurationDeclarativeSuite) TestDeclarativeUpdateRejected() {
	status, raw := ts.request(http.MethodPut, configAPIBasePath+"/"+declConfigID,
		testutils.CredentialConfiguration{
			Handle: declConfigHandle,
			OUID:   declConfigOUID,
			Name:   "Attempted rename of declarative configuration",
			VCT:    declConfigVCT,
		})
	ts.Equalf(http.StatusConflict, status, "updating a declarative configuration must be refused: %s", string(raw))

	status, raw = ts.request(http.MethodGet, configAPIBasePath+"/"+declConfigID, nil)
	ts.Require().Equal(http.StatusOK, status)
	ts.Equal("Declarative Test Credential", ts.object(raw)["name"], "the declarative name must be unchanged")
}

// TestDeclarativeDeleteRejected verifies a file-backed configuration cannot be deleted.
func (ts *CredentialConfigurationDeclarativeSuite) TestDeclarativeDeleteRejected() {
	status, raw := ts.request(http.MethodDelete, configAPIBasePath+"/"+declConfigID, nil)
	ts.Equalf(http.StatusConflict, status, "deleting a declarative configuration must be refused: %s", string(raw))

	status, _ = ts.request(http.MethodGet, configAPIBasePath+"/"+declConfigID, nil)
	ts.Equal(http.StatusOK, status, "the declarative configuration must still be present")
}

// TestDeclarativeHandleConflict verifies a runtime configuration cannot claim a handle
// already owned by a declarative one.
func (ts *CredentialConfigurationDeclarativeSuite) TestDeclarativeHandleConflict() {
	status, raw := ts.request(http.MethodPost, configAPIBasePath, testutils.CredentialConfiguration{
		Handle: declConfigHandle,
		OUID:   ts.ouID,
		VCT:    "https://credentials.thunderid.local/Clash",
	})
	ts.Equalf(http.StatusConflict, status, "reusing a declarative handle must be refused: %s", string(raw))
}

// TestRuntimeCreateStillWorks verifies composite mode still accepts runtime writes: the
// file store refuses creates, so the composite store must route them to the database.
func (ts *CredentialConfigurationDeclarativeSuite) TestRuntimeCreateStillWorks() {
	id, err := testutils.CreateCredentialConfiguration(testutils.CredentialConfiguration{
		Handle: "runtime_alongside_declarative",
		OUID:   ts.ouID,
		Name:   "Runtime Configuration",
		VCT:    "https://credentials.thunderid.local/RuntimeCredential",
	})
	ts.Require().NoError(err, "composite mode must still accept runtime creates")
	ts.runtimeID = id

	status, raw := ts.request(http.MethodGet, configAPIBasePath+"/"+id, nil)
	ts.Require().Equal(http.StatusOK, status)
	ts.Equal("runtime_alongside_declarative", ts.object(raw)["handle"])
}

// TestDeclarativeAdvertisedInIssuerMetadata verifies a file-backed configuration reaches
// the issuer engine and is advertised to wallets.
func (ts *CredentialConfigurationDeclarativeSuite) TestDeclarativeAdvertisedInIssuerMetadata() {
	res, meta, err := testutils.GetVCIIssuerMetadata()
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "metadata: %s", string(res.Body))

	supported, ok := meta["credential_configurations_supported"].(map[string]any)
	ts.Require().Truef(ok, "credential_configurations_supported missing: %s", string(res.Body))

	cfg, ok := supported[declConfigHandle].(map[string]any)
	ts.Require().Truef(ok, "declarative configuration not advertised: %s", string(res.Body))
	ts.Equal(declConfigVCT, cfg["vct"])
}
