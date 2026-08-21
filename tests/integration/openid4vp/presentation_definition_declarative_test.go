// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

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
	definitionAPIBasePath = "/openid4vp/presentation-definitions"

	// Seeded from resources/declarative_resources/presentation_definitions, which the
	// server loads at startup because openid4vp.store is composite. Declarative
	// definitions are immutable: the management API must refuse writes to them.
	declDefinitionID     = "decl-presentation-def-1"
	declDefinitionHandle = "decl_presentation_def_1"
	declDefinitionOUID   = "decl-ou-1"
	declDefinitionVCT    = "https://credentials.thunderid.local/DeclarativeTestCredential"
)

// PresentationDefinitionDeclarativeSuite covers file-backed presentation definitions in
// composite mode: they are readable, immutable, usable by the verifier, and coexist with
// runtime definitions.
type PresentationDefinitionDeclarativeSuite struct {
	suite.Suite
	ouID      string
	runtimeID string
}

func TestPresentationDefinitionDeclarativeSuite(t *testing.T) {
	suite.Run(t, new(PresentationDefinitionDeclarativeSuite))
}

func (ts *PresentationDefinitionDeclarativeSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "vp-declarative-ou",
		Name:        "OpenID4VP Declarative OU",
		Description: "Organization unit for declarative presentation definition tests",
	})
	ts.Require().NoError(err, "create test OU")
	ts.ouID = ouID
}

func (ts *PresentationDefinitionDeclarativeSuite) TearDownSuite() {
	if ts.runtimeID != "" {
		ts.NoError(testutils.DeletePresentationDefinition(ts.runtimeID))
	}
	if ts.ouID != "" {
		ts.NoError(testutils.DeleteOrganizationUnit(ts.ouID))
	}
}

// request issues an authenticated management-API call and returns the status and raw body.
func (ts *PresentationDefinitionDeclarativeSuite) request(method, path string, body any) (int, []byte) {
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
func (ts *PresentationDefinitionDeclarativeSuite) object(raw []byte) map[string]any {
	ts.T().Helper()
	decoded := map[string]any{}
	ts.Require().NoErrorf(json.Unmarshal(raw, &decoded), "body: %s", string(raw))
	return decoded
}

// TestDeclarativeVisibleWithOU verifies a file-backed definition is readable and carries
// the organization unit declared in its YAML.
func (ts *PresentationDefinitionDeclarativeSuite) TestDeclarativeVisibleWithOU() {
	status, raw := ts.request(http.MethodGet, definitionAPIBasePath+"/"+declDefinitionID, nil)
	ts.Require().Equalf(http.StatusOK, status, "get declarative: %s", string(raw))

	body := ts.object(raw)
	ts.Equal(declDefinitionID, body["id"])
	ts.Equal(declDefinitionHandle, body["handle"])
	ts.Equal(declDefinitionVCT, body["vct"])
	ts.Equal("Declarative Test Presentation", body["name"])
	ts.Equal(declDefinitionOUID, body["ouId"], "the ouId declared in YAML must survive the load")
}

// TestDeclarativeAppearsInList verifies declarative and runtime definitions are merged
// into a single listing in composite mode.
func (ts *PresentationDefinitionDeclarativeSuite) TestDeclarativeAppearsInList() {
	status, raw := ts.request(http.MethodGet, definitionAPIBasePath, nil)
	ts.Require().Equalf(http.StatusOK, status, "list: %s", string(raw))

	var items []map[string]any
	ts.Require().NoErrorf(json.Unmarshal(raw, &items), "list body: %s", string(raw))

	var found bool
	for _, item := range items {
		if item["id"] == declDefinitionID {
			found = true
		}
	}
	ts.Truef(found, "declarative definition missing from list: %s", string(raw))
}

// TestDeclarativeUpdateRejected verifies a file-backed definition cannot be updated.
func (ts *PresentationDefinitionDeclarativeSuite) TestDeclarativeUpdateRejected() {
	status, raw := ts.request(http.MethodPut, definitionAPIBasePath+"/"+declDefinitionID,
		testutils.PresentationDefinition{
			Handle: declDefinitionHandle,
			OUID:   declDefinitionOUID,
			Name:   "Attempted rename of declarative definition",
			VCT:    declDefinitionVCT,
		})
	ts.Equalf(http.StatusConflict, status, "updating a declarative definition must be refused: %s", string(raw))

	status, raw = ts.request(http.MethodGet, definitionAPIBasePath+"/"+declDefinitionID, nil)
	ts.Require().Equal(http.StatusOK, status)
	ts.Equal("Declarative Test Presentation", ts.object(raw)["name"], "the declarative name must be unchanged")
}

// TestDeclarativeDeleteRejected verifies a file-backed definition cannot be deleted.
func (ts *PresentationDefinitionDeclarativeSuite) TestDeclarativeDeleteRejected() {
	status, raw := ts.request(http.MethodDelete, definitionAPIBasePath+"/"+declDefinitionID, nil)
	ts.Equalf(http.StatusConflict, status, "deleting a declarative definition must be refused: %s", string(raw))

	status, _ = ts.request(http.MethodGet, definitionAPIBasePath+"/"+declDefinitionID, nil)
	ts.Equal(http.StatusOK, status, "the declarative definition must still be present")
}

// TestDeclarativeHandleConflict verifies a runtime definition cannot claim a handle
// already owned by a declarative one.
func (ts *PresentationDefinitionDeclarativeSuite) TestDeclarativeHandleConflict() {
	status, raw := ts.request(http.MethodPost, definitionAPIBasePath, testutils.PresentationDefinition{
		Handle: declDefinitionHandle,
		OUID:   ts.ouID,
		VCT:    "https://credentials.thunderid.local/Clash",
	})
	ts.Equalf(http.StatusConflict, status, "reusing a declarative handle must be refused: %s", string(raw))
}

// TestRuntimeCreateStillWorks verifies composite mode still accepts runtime writes: the
// file store refuses creates, so the composite store must route them to the database.
func (ts *PresentationDefinitionDeclarativeSuite) TestRuntimeCreateStillWorks() {
	enforceTrustedIssuer := false
	id, err := testutils.CreatePresentationDefinition(testutils.PresentationDefinition{
		Handle:               "runtime_alongside_declarative",
		OUID:                 ts.ouID,
		Name:                 "Runtime Definition",
		VCT:                  "https://credentials.thunderid.local/RuntimeCredential",
		RequestedClaims:      []string{"given_name"},
		EnforceTrustedIssuer: &enforceTrustedIssuer,
	})
	ts.Require().NoError(err, "composite mode must still accept runtime creates")
	ts.runtimeID = id

	status, raw := ts.request(http.MethodGet, definitionAPIBasePath+"/"+id, nil)
	ts.Require().Equal(http.StatusOK, status)
	ts.Equal("runtime_alongside_declarative", ts.object(raw)["handle"])
}

// TestDeclarativeUsableByVerifier verifies a file-backed definition reaches the verifier
// engine and can drive an authorization request. The verifier resolves definitions by
// handle, so this also covers the file store's by-handle lookup.
func (ts *PresentationDefinitionDeclarativeSuite) TestDeclarativeUsableByVerifier() {
	res, initiated, err := testutils.InitiateVP(declDefinitionHandle)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode,
		"initiate with declarative definition: %s", string(res.Body))
	ts.NotEmpty(initiated.TxnID)
}
