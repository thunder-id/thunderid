// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

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
	definitionAPIBasePath = "/openid4vp/presentation-definitions"

	// Handles are prefixed to keep this suite's fixtures distinct from the
	// verification suite's, which shares the same package and database.
	crudDefinitionHandle    = "pd_crud_primary"
	crudDefinitionAltHandle = "pd_crud_secondary"
	crudDefinitionVCT       = "https://credentials.thunderid.local/CrudTestPresentation"

	// Error codes returned by the presentation definition management API.
	codeDefinitionInvalidRequest    = "VP-2001"
	codeDefinitionNotFound          = "VP-2002"
	codeDefinitionAlreadyExists     = "VP-2003"
	codeDefinitionUnsupportedFormat = "VP-2004"
	codeDefinitionImmutable         = "VP-2005"
	codeDefinitionInvalidOU         = "VP-2007"

	// Declarative (file-backed) definitions seeded from
	// resources/declarative_resources/presentation_definitions. They are
	// immutable: the management API must reject updates and deletes.
	declDefinitionID     = "decl-presentation-def-1"
	declDefinitionHandle = "decl_presentation_def_1"
	declDefinitionVCT    = "https://credentials.thunderid.local/DeclarativeTestCredential"
)

var crudDefinitionOU = testutils.OrganizationUnit{
	Handle:      "openid4vp-definition-crud-ou",
	Name:        "OpenID4VP Definition CRUD OU",
	Description: "Organization unit for presentation definition CRUD testing",
	Parent:      nil,
}

// PresentationDefinitionAPITestSuite exercises the presentation definition
// management API (CRUD, validation, and error mapping) against the live server.
type PresentationDefinitionAPITestSuite struct {
	suite.Suite
	ouID      string
	createdID []string
}

// TestPresentationDefinitionAPITestSuite is the single entrypoint that runs every Test* method.
func TestPresentationDefinitionAPITestSuite(t *testing.T) {
	suite.Run(t, new(PresentationDefinitionAPITestSuite))
}

func (ts *PresentationDefinitionAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(crudDefinitionOU)
	ts.Require().NoError(err, "create test OU")
	ts.ouID = ouID
}

func (ts *PresentationDefinitionAPITestSuite) TearDownSuite() {
	for _, id := range ts.createdID {
		if err := testutils.DeletePresentationDefinition(id); err != nil {
			ts.T().Logf("Failed to delete presentation definition %s: %v", id, err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

// definitionRequest issues a request against the definition API and returns the
// raw status and body so tests can assert on error codes.
func (ts *PresentationDefinitionAPITestSuite) definitionRequest(
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

// createDefinition posts a definition and registers the created id for cleanup.
func (ts *PresentationDefinitionAPITestSuite) createDefinition(
	def testutils.PresentationDefinition,
) (*testutils.VCHTTPResult, map[string]any) {
	res := ts.definitionRequest(http.MethodPost, definitionAPIBasePath, def)
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
func (ts *PresentationDefinitionAPITestSuite) decode(body []byte) map[string]any {
	var parsed map[string]any
	ts.Require().NoErrorf(json.Unmarshal(body, &parsed), "decode body: %s", string(body))
	return parsed
}

// errorCodeOf extracts the "code" field from an API error body.
func (ts *PresentationDefinitionAPITestSuite) errorCodeOf(body []byte) string {
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	code, _ := parsed["code"].(string)
	return code
}

// baseDefinition returns a valid definition body owned by the suite's OU.
func (ts *PresentationDefinitionAPITestSuite) baseDefinition(handle string) testutils.PresentationDefinition {
	enforceTrustedIssuer := true
	return testutils.PresentationDefinition{
		Handle:               handle,
		OUID:                 ts.ouID,
		Name:                 "CRUD Test Presentation",
		Description:          "Presentation definition for CRUD testing",
		VCT:                  crudDefinitionVCT,
		Format:               "dc+sd-jwt",
		RequestedClaims:      []string{"given_name", "family_name", "email"},
		MandatoryClaims:      []string{"given_name"},
		OptionalClaims:       []string{"email"},
		ClaimValues:          map[string][]string{"country": {"LK", "US"}},
		EnforceTrustedIssuer: &enforceTrustedIssuer,
		TrustedAuthorities:   []string{"integration-test-anchor"},
	}
}

// TestCreateAndGet verifies a created definition round-trips through the
// single-resource read with every field intact.
func (ts *PresentationDefinitionAPITestSuite) TestCreateAndGet() {
	res, created := ts.createDefinition(ts.baseDefinition(crudDefinitionHandle))
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))

	id, _ := created["id"].(string)
	ts.Require().NotEmpty(id, "created id missing")

	getRes := ts.definitionRequest(http.MethodGet, definitionAPIBasePath+"/"+id, nil)
	ts.Require().Equalf(http.StatusOK, getRes.StatusCode, "get: %s", string(getRes.Body))

	fetched := ts.decode(getRes.Body)
	ts.Equal(id, fetched["id"])
	ts.Equal(crudDefinitionHandle, fetched["handle"])
	ts.Equal(crudDefinitionVCT, fetched["vct"])
	ts.Equal("dc+sd-jwt", fetched["format"])
	ts.Equal(ts.ouID, fetched["ouId"])
	ts.Equal("CRUD Test Presentation", fetched["name"])
	// The owning OU handle is resolved for display on read.
	ts.Equal(crudDefinitionOU.Handle, fetched["ouHandle"])
	ts.Equal(true, fetched["enforceTrustedIssuer"])

	requested, ok := fetched["requestedClaims"].([]any)
	ts.Require().Truef(ok, "requestedClaims missing: %s", string(getRes.Body))
	ts.Len(requested, 3)

	mandatory, ok := fetched["mandatoryClaims"].([]any)
	ts.Require().Truef(ok, "mandatoryClaims missing: %s", string(getRes.Body))
	ts.Equal([]any{"given_name"}, mandatory)

	authorities, ok := fetched["trustedAuthorities"].([]any)
	ts.Require().Truef(ok, "trustedAuthorities missing: %s", string(getRes.Body))
	ts.Equal([]any{"integration-test-anchor"}, authorities)

	claimValues, ok := fetched["claimValues"].(map[string]any)
	ts.Require().Truef(ok, "claimValues missing: %s", string(getRes.Body))
	ts.Equal([]any{"LK", "US"}, claimValues["country"])
}

// TestList verifies the list endpoint returns the summary projection including
// the created definition.
func (ts *PresentationDefinitionAPITestSuite) TestList() {
	res, created := ts.createDefinition(ts.baseDefinition("pd_crud_listed"))
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	id, _ := created["id"].(string)

	listRes := ts.definitionRequest(http.MethodGet, definitionAPIBasePath, nil)
	ts.Require().Equalf(http.StatusOK, listRes.StatusCode, "list: %s", string(listRes.Body))

	var summaries []map[string]any
	ts.Require().NoErrorf(json.Unmarshal(listRes.Body, &summaries), "decode list: %s", string(listRes.Body))
	ts.Require().NotEmpty(summaries, "list returned no definitions")

	var found map[string]any
	for _, s := range summaries {
		if s["id"] == id {
			found = s
			break
		}
	}
	ts.Require().NotNilf(found, "created definition missing from list: %s", string(listRes.Body))
	ts.Equal("pd_crud_listed", found["handle"])
	ts.Equal(crudDefinitionVCT, found["vct"])
	ts.Equal("dc+sd-jwt", found["format"])
	// The list projection is a summary: per-claim detail is not included.
	ts.NotContains(found, "requestedClaims")
	ts.NotContains(found, "trustedAuthorities")
}

// TestUpdate verifies an update persists and is reflected on a subsequent read.
func (ts *PresentationDefinitionAPITestSuite) TestUpdate() {
	res, created := ts.createDefinition(ts.baseDefinition("pd_crud_updatable"))
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	id, _ := created["id"].(string)

	enforceTrustedIssuer := false
	updated := ts.baseDefinition("pd_crud_updated_handle")
	updated.Name = "Renamed Presentation"
	updated.RequestedClaims = []string{"email"}
	updated.MandatoryClaims = []string{"email"}
	updated.OptionalClaims = nil
	updated.EnforceTrustedIssuer = &enforceTrustedIssuer
	updated.TrustedAuthorities = []string{"anchor-one", "anchor-two"}

	updateRes := ts.definitionRequest(http.MethodPut, definitionAPIBasePath+"/"+id, updated)
	ts.Require().Equalf(http.StatusOK, updateRes.StatusCode, "update: %s", string(updateRes.Body))

	body := ts.decode(updateRes.Body)
	ts.Equal("pd_crud_updated_handle", body["handle"])
	ts.Equal("Renamed Presentation", body["name"])

	getRes := ts.definitionRequest(http.MethodGet, definitionAPIBasePath+"/"+id, nil)
	ts.Require().Equal(http.StatusOK, getRes.StatusCode)
	fetched := ts.decode(getRes.Body)
	ts.Equal("pd_crud_updated_handle", fetched["handle"])
	ts.Equal("Renamed Presentation", fetched["name"])
	ts.Equal(false, fetched["enforceTrustedIssuer"])

	requested, ok := fetched["requestedClaims"].([]any)
	ts.Require().True(ok, "requestedClaims missing after update")
	ts.Equal([]any{"email"}, requested)

	// Clearing optionalClaims must drop the field, not retain the previous value.
	ts.NotContains(fetched, "optionalClaims", "optionalClaims must be cleared by the update")

	authorities, ok := fetched["trustedAuthorities"].([]any)
	ts.Require().True(ok, "trustedAuthorities missing after update")
	ts.Equal([]any{"anchor-one", "anchor-two"}, authorities)
}

// TestUpdate_SameHandleAllowed verifies updating a definition while keeping its
// own handle is not treated as a handle conflict.
func (ts *PresentationDefinitionAPITestSuite) TestUpdate_SameHandleAllowed() {
	res, created := ts.createDefinition(ts.baseDefinition("pd_crud_same_handle"))
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	id, _ := created["id"].(string)

	same := ts.baseDefinition("pd_crud_same_handle")
	same.Description = "Description changed, handle retained"

	updateRes := ts.definitionRequest(http.MethodPut, definitionAPIBasePath+"/"+id, same)
	ts.Require().Equalf(http.StatusOK, updateRes.StatusCode, "update: %s", string(updateRes.Body))
	ts.Equal("Description changed, handle retained", ts.decode(updateRes.Body)["description"])
}

// TestCreate_DefaultsFormat verifies an omitted format defaults to dc+sd-jwt.
func (ts *PresentationDefinitionAPITestSuite) TestCreate_DefaultsFormat() {
	def := ts.baseDefinition("pd_crud_default_format")
	def.Format = ""

	res, created := ts.createDefinition(def)
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	ts.Equal("dc+sd-jwt", created["format"])
}

// TestCreate_ResolvesOUByHandle verifies the owning OU can be supplied as a
// handle path instead of an id.
func (ts *PresentationDefinitionAPITestSuite) TestCreate_ResolvesOUByHandle() {
	def := ts.baseDefinition("pd_crud_ou_by_handle")
	def.OUID = ""
	def.OUHandle = crudDefinitionOU.Handle

	res, created := ts.createDefinition(def)
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	ts.Equal(ts.ouID, created["ouId"], "OU handle must resolve to the OU id")
}

// TestDelete verifies a deleted definition is no longer readable, and that
// deleting an absent definition succeeds idempotently.
func (ts *PresentationDefinitionAPITestSuite) TestDelete() {
	res, created := ts.createDefinition(ts.baseDefinition("pd_crud_deletable"))
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	id, _ := created["id"].(string)

	delRes := ts.definitionRequest(http.MethodDelete, definitionAPIBasePath+"/"+id, nil)
	ts.Require().Equalf(http.StatusNoContent, delRes.StatusCode, "delete: %s", string(delRes.Body))

	getRes := ts.definitionRequest(http.MethodGet, definitionAPIBasePath+"/"+id, nil)
	ts.Equal(http.StatusNotFound, getRes.StatusCode)

	// Deleting an already-absent definition is idempotent.
	repeatRes := ts.definitionRequest(http.MethodDelete, definitionAPIBasePath+"/"+id, nil)
	ts.Equal(http.StatusNoContent, repeatRes.StatusCode)
}

// TestCreate_DuplicateHandle rejects a second definition reusing a handle.
func (ts *PresentationDefinitionAPITestSuite) TestCreate_DuplicateHandle() {
	first, _ := ts.createDefinition(ts.baseDefinition(crudDefinitionAltHandle))
	ts.Require().Equalf(http.StatusCreated, first.StatusCode, "create: %s", string(first.Body))

	dup := ts.baseDefinition(crudDefinitionAltHandle)
	dup.Name = "Duplicate handle attempt"
	res, _ := ts.createDefinition(dup)
	ts.Equal(http.StatusConflict, res.StatusCode)
	ts.Equal(codeDefinitionAlreadyExists, ts.errorCodeOf(res.Body))
}

// TestUpdate_HandleConflict rejects renaming a definition onto a handle another
// definition already holds.
func (ts *PresentationDefinitionAPITestSuite) TestUpdate_HandleConflict() {
	occupied := "pd_crud_occupied"
	first, _ := ts.createDefinition(ts.baseDefinition(occupied))
	ts.Require().Equalf(http.StatusCreated, first.StatusCode, "create: %s", string(first.Body))

	second, created := ts.createDefinition(ts.baseDefinition("pd_crud_renamer"))
	ts.Require().Equalf(http.StatusCreated, second.StatusCode, "create: %s", string(second.Body))
	id, _ := created["id"].(string)

	clash := ts.baseDefinition(occupied)
	res := ts.definitionRequest(http.MethodPut, definitionAPIBasePath+"/"+id, clash)
	ts.Equal(http.StatusConflict, res.StatusCode)
	ts.Equal(codeDefinitionAlreadyExists, ts.errorCodeOf(res.Body))
}

// TestCreate_ValidationErrors rejects requests missing required fields or
// carrying unsupported values.
func (ts *PresentationDefinitionAPITestSuite) TestCreate_ValidationErrors() {
	cases := []struct {
		name     string
		mutate   func(*testutils.PresentationDefinition)
		wantCode string
	}{
		{
			name:     "missing handle",
			mutate:   func(d *testutils.PresentationDefinition) { d.Handle = "" },
			wantCode: codeDefinitionInvalidRequest,
		},
		{
			name:     "missing vct",
			mutate:   func(d *testutils.PresentationDefinition) { d.VCT = "" },
			wantCode: codeDefinitionInvalidRequest,
		},
		{
			name:     "unsupported format",
			mutate:   func(d *testutils.PresentationDefinition) { d.Format = "jwt_vc_json" },
			wantCode: codeDefinitionUnsupportedFormat,
		},
		{
			name: "unknown organization unit",
			mutate: func(d *testutils.PresentationDefinition) {
				d.OUID = "00000000-0000-0000-0000-000000000000"
			},
			wantCode: codeDefinitionInvalidOU,
		},
		{
			name: "missing organization unit",
			mutate: func(d *testutils.PresentationDefinition) {
				d.OUID = ""
				d.OUHandle = ""
			},
			wantCode: codeDefinitionInvalidOU,
		},
		{
			name: "unresolvable organization unit handle",
			mutate: func(d *testutils.PresentationDefinition) {
				d.OUID = ""
				d.OUHandle = "no-such-ou-handle"
			},
			wantCode: codeDefinitionInvalidOU,
		},
	}

	for i, tc := range cases {
		ts.Run(tc.name, func() {
			def := ts.baseDefinition(fmt.Sprintf("pd_crud_invalid_%d", i))
			tc.mutate(&def)

			res, _ := ts.createDefinition(def)
			ts.Equalf(http.StatusBadRequest, res.StatusCode, "body: %s", string(res.Body))
			ts.Equal(tc.wantCode, ts.errorCodeOf(res.Body))
		})
	}
}

// TestCreate_MalformedBody rejects a request body that is not valid JSON.
func (ts *PresentationDefinitionAPITestSuite) TestCreate_MalformedBody() {
	res := ts.definitionRequest(http.MethodPost, definitionAPIBasePath, "{not-json")
	ts.Equal(http.StatusBadRequest, res.StatusCode)
	ts.Equal(codeDefinitionInvalidRequest, ts.errorCodeOf(res.Body))
}

// TestUnknownID returns not-found for reads, updates, and malformed bodies
// against an id that does not exist.
func (ts *PresentationDefinitionAPITestSuite) TestUnknownID() {
	unknown := definitionAPIBasePath + "/11111111-2222-3333-4444-555555555555"

	getRes := ts.definitionRequest(http.MethodGet, unknown, nil)
	ts.Equal(http.StatusNotFound, getRes.StatusCode)
	ts.Equal(codeDefinitionNotFound, ts.errorCodeOf(getRes.Body))

	putRes := ts.definitionRequest(http.MethodPut, unknown, ts.baseDefinition("pd_crud_ghost"))
	ts.Equal(http.StatusNotFound, putRes.StatusCode)
	ts.Equal(codeDefinitionNotFound, ts.errorCodeOf(putRes.Body))

	malformedRes := ts.definitionRequest(http.MethodPut, unknown, "{not-json")
	ts.Equal(http.StatusBadRequest, malformedRes.StatusCode)
	ts.Equal(codeDefinitionInvalidRequest, ts.errorCodeOf(malformedRes.Body))
}

// TestDeclarativeVisibility verifies a file-backed definition is readable
// through the management API in composite mode.
func (ts *PresentationDefinitionAPITestSuite) TestDeclarativeVisibility() {
	res := ts.definitionRequest(http.MethodGet, definitionAPIBasePath+"/"+declDefinitionID, nil)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "get declarative: %s", string(res.Body))

	fetched := ts.decode(res.Body)
	ts.Equal(declDefinitionID, fetched["id"])
	ts.Equal(declDefinitionHandle, fetched["handle"])
	ts.Equal(declDefinitionVCT, fetched["vct"])
	ts.Equal("dc+sd-jwt", fetched["format"])

	requested, ok := fetched["requestedClaims"].([]any)
	ts.Require().Truef(ok, "requestedClaims missing: %s", string(res.Body))
	ts.Len(requested, 2)

	claimValues, ok := fetched["claimValues"].(map[string]any)
	ts.Require().Truef(ok, "claimValues missing: %s", string(res.Body))
	ts.Equal([]any{"LK", "US"}, claimValues["country"])
}

// TestDeclarativeAppearsInList verifies declarative and runtime definitions are
// merged into a single listing.
func (ts *PresentationDefinitionAPITestSuite) TestDeclarativeAppearsInList() {
	res, created := ts.createDefinition(ts.baseDefinition("pd_crud_merged"))
	ts.Require().Equalf(http.StatusCreated, res.StatusCode, "create: %s", string(res.Body))
	runtimeID, _ := created["id"].(string)

	listRes := ts.definitionRequest(http.MethodGet, definitionAPIBasePath, nil)
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
	ts.True(ids[declDefinitionID], "declarative definition missing from list")
	ts.True(ids[runtimeID], "runtime definition missing from list")
}

// TestDeclarativeUpdateReject verifies a file-backed definition cannot be
// modified through the management API.
func (ts *PresentationDefinitionAPITestSuite) TestDeclarativeUpdateReject() {
	update := ts.baseDefinition(declDefinitionHandle)
	update.Name = "Attempted rename of declarative definition"

	res := ts.definitionRequest(http.MethodPut, definitionAPIBasePath+"/"+declDefinitionID, update)
	ts.Equalf(http.StatusConflict, res.StatusCode, "update declarative: %s", string(res.Body))
	ts.Equal(codeDefinitionImmutable, ts.errorCodeOf(res.Body))

	// The definition must be unchanged.
	getRes := ts.definitionRequest(http.MethodGet, definitionAPIBasePath+"/"+declDefinitionID, nil)
	ts.Require().Equal(http.StatusOK, getRes.StatusCode)
	ts.Equal("Declarative Test Presentation", ts.decode(getRes.Body)["name"])
}

// TestDeclarativeDeleteReject verifies a file-backed definition cannot be
// deleted through the management API.
func (ts *PresentationDefinitionAPITestSuite) TestDeclarativeDeleteReject() {
	res := ts.definitionRequest(http.MethodDelete, definitionAPIBasePath+"/"+declDefinitionID, nil)
	ts.Equalf(http.StatusConflict, res.StatusCode, "delete declarative: %s", string(res.Body))
	ts.Equal(codeDefinitionImmutable, ts.errorCodeOf(res.Body))

	// The definition must still be readable.
	getRes := ts.definitionRequest(http.MethodGet, definitionAPIBasePath+"/"+declDefinitionID, nil)
	ts.Equal(http.StatusOK, getRes.StatusCode)
}

// TestDeclarativeHandleConflict rejects a runtime definition reusing a
// declarative definition's handle.
func (ts *PresentationDefinitionAPITestSuite) TestDeclarativeHandleConflict() {
	clash := ts.baseDefinition(declDefinitionHandle)
	res, _ := ts.createDefinition(clash)
	ts.Equalf(http.StatusConflict, res.StatusCode, "create clashing handle: %s", string(res.Body))
	ts.Equal(codeDefinitionAlreadyExists, ts.errorCodeOf(res.Body))
}

// TestDeclarativeUsableByVerifier verifies a file-backed definition can drive a
// verification session, not just the management API.
func (ts *PresentationDefinitionAPITestSuite) TestDeclarativeUsableByVerifier() {
	res, init, err := testutils.InitiateVP(declDefinitionHandle)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "initiate with declarative definition: %s", string(res.Body))
	ts.NotEmpty(init.TxnID, "txn_id missing")
}

// TestUnauthenticated rejects management requests carrying no access token.
func (ts *PresentationDefinitionAPITestSuite) TestUnauthenticated() {
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+definitionAPIBasePath, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusUnauthorized, resp.StatusCode)
}
