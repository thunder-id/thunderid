// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authorizationdetail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	typesPath = "/authorization-detail-types"

	// maskedSecret is how the management API returns a stored credential.
	maskedSecret = "******"

	// codeInvalidSchema is returned for a schema the registry refuses.
	codeInvalidSchema = "RAR-1007"
	// codeTypeConflict is returned for a type value another type already has.
	codeTypeConflict = "RAR-1004"
	// codeProcessorAuthenticationRequired is returned when a processor endpoint changes without its credential.
	codeProcessorAuthenticationRequired = "RAR-1032"
)

// flightSchema is a type schema with a field the user and the client may lower.
var flightSchema = json.RawMessage(`{"type":"object","required":["route"],"properties":{` +
	`"route":{"type":"string","title":"Route"},` +
	`"maxFare":{"type":"number","minimum":0,"maximum":5000,` +
	`"policy":{"modification":"decrease","modifiableBy":["user","client"]}}}}`)

type TypeAPITestSuite struct {
	suite.Suite
}

func TestTypeAPITestSuite(t *testing.T) {
	suite.Run(t, new(TypeAPITestSuite))
}

func (s *TypeAPITestSuite) TestCreateGetUpdateDelete() {
	value := uniqueTypeValue("crud")
	created := createType(s.T(), typeRequest{Type: value, Name: "Flight booking", Schema: flightSchema,
		ConsentTemplate: "Book {{route}}", Reuse: true})
	defer deleteType(s.T(), created.ID)

	s.Equal(value, created.Type)
	s.Equal(1, created.Version)
	s.False(created.IsReadOnly)
	s.JSONEq(string(flightSchema), string(created.Schema))

	status, body := doJSON(s.T(), http.MethodGet, typesPath+"/"+created.ID, nil)
	s.Require().Equal(http.StatusOK, status, string(body))
	var got typeResponse
	s.Require().NoError(json.Unmarshal(body, &got))
	s.Equal(created.ID, got.ID)
	s.Equal("Book {{route}}", got.ConsentTemplate)

	// A name change keeps the version; a schema change moves it, since earlier approvals no longer apply.
	renamed := typeRequest{Type: value, Name: "Flight", Schema: flightSchema, ConsentTemplate: "Book {{route}}",
		Reuse: true}
	updated := updateType(s.T(), created.ID, renamed)
	s.Equal("Flight", updated.Name)
	s.Equal(1, updated.Version)

	renamed.Schema = json.RawMessage(`{"type":"object","properties":{"route":{"type":"string"}}}`)
	updated = updateType(s.T(), created.ID, renamed)
	s.Equal(2, updated.Version)

	// A type changed this way is sent back as it was fetched.
	fetched, err := json.Marshal(updated)
	s.Require().NoError(err)
	status, body = doRaw(s.T(), http.MethodPut, typesPath+"/"+created.ID, fetched)
	s.Equal(http.StatusOK, status, string(body))

	status, body = doJSON(s.T(), http.MethodDelete, typesPath+"/"+created.ID, nil)
	s.Equal(http.StatusNoContent, status, string(body))
	status, _ = doJSON(s.T(), http.MethodGet, typesPath+"/"+created.ID, nil)
	s.Equal(http.StatusNotFound, status)
	status, body = doJSON(s.T(), http.MethodDelete, typesPath+"/"+created.ID, nil)
	s.Equal(http.StatusNoContent, status, "deleting a type that does not exist succeeds: %s", body)
}

func (s *TypeAPITestSuite) TestListPagesTheTypes() {
	first := createType(s.T(), typeRequest{Type: uniqueTypeValue("list-a"), Name: "List A", Schema: flightSchema})
	defer deleteType(s.T(), first.ID)
	second := createType(s.T(), typeRequest{Type: uniqueTypeValue("list-b"), Name: "List B", Schema: flightSchema})
	defer deleteType(s.T(), second.ID)

	all := listAllTypes(s.T())
	s.Require().GreaterOrEqual(len(all), 2)
	s.Contains(typeIDs(all), first.ID)
	s.Contains(typeIDs(all), second.ID)

	page := listTypes(s.T(), 1, 1)
	s.Equal(len(all), page.TotalResults)
	s.Require().Len(page.Types, 1)
	s.Equal(all[1].ID, page.Types[0].ID, "a page is cut from the list in type value order")

	status, _ := doJSON(s.T(), http.MethodGet, typesPath+"?limit=0", nil)
	s.Equal(http.StatusBadRequest, status)
}

func (s *TypeAPITestSuite) TestInvalidDefinitionsAreRefused() {
	cases := map[string]json.RawMessage{
		"a list without items": json.RawMessage(`{"type":"object","properties":{"actions":{"type":"array"}}}`),
		"a policy under list items": json.RawMessage(`{"type":"object","properties":{"lines":{"type":"array",` +
			`"items":{"type":"object","properties":{"amount":{"type":"number",` +
			`"policy":{"modification":"decrease","modifiableBy":["user"]}}}}}}}`),
		"an open object": json.RawMessage(`{"type":"object","additionalProperties":true,` +
			`"properties":{"route":{"type":"string"}}}`),
		"a reference": json.RawMessage(`{"type":"object","properties":{"route":{"$ref":"#/$defs/route"}}}`),
	}
	for name, schema := range cases {
		status, body := doJSON(s.T(), http.MethodPost, typesPath,
			typeRequest{Type: uniqueTypeValue("invalid"), Name: "Invalid", Schema: schema})
		deleteIfCreated(s.T(), status, body)
		s.Equal(http.StatusBadRequest, status, name)
		s.Equal(codeInvalidSchema, errorCode(s.T(), body), name)
	}
}

func (s *TypeAPITestSuite) TestDuplicateTypeValueIsRefused() {
	value := uniqueTypeValue("duplicate")
	created := createType(s.T(), typeRequest{Type: value, Name: "Original", Schema: flightSchema})
	defer deleteType(s.T(), created.ID)

	status, body := doJSON(s.T(), http.MethodPost, typesPath,
		typeRequest{Type: value, Name: "Copy", Schema: flightSchema})
	deleteIfCreated(s.T(), status, body)
	s.Equal(http.StatusConflict, status, string(body))
	s.Equal(codeTypeConflict, errorCode(s.T(), body))
}

func (s *TypeAPITestSuite) TestMisspelledKeyIsRefused() {
	body := []byte(fmt.Sprintf(`{"type":%q,"name":"Misspelled","schema":%s,`+
		`"procesor":{"endpoint":"https://bank.example.com/rar"}}`, uniqueTypeValue("misspelled"), flightSchema))
	status, resp := doRaw(s.T(), http.MethodPost, typesPath, body)
	deleteIfCreated(s.T(), status, resp)
	s.Equal(http.StatusBadRequest, status, "a misspelled processor key is refused rather than dropped: %s", resp)
}

func (s *TypeAPITestSuite) TestProcessorCredentialIsMaskedAndKept() {
	value := uniqueTypeValue("credential")
	endpoint := "https://processor.example.com/rar"
	req := typeRequest{Type: value, Name: "Credential", Schema: flightSchema, Processor: &processorConfig{
		Endpoint:       endpoint,
		Authentication: &processorAuthentication{Type: "bearer", Properties: map[string]string{"token": "secret"}},
	}}
	created := createType(s.T(), req)
	defer deleteType(s.T(), created.ID)

	s.Require().NotNil(created.Processor)
	s.Equal(endpoint, created.Processor.Endpoint)
	s.Require().NotNil(created.Processor.Authentication)
	s.Equal(maskedSecret, created.Processor.Authentication.Properties["token"], "the credential is masked")

	// A masked credential sent back at the same endpoint keeps the stored one.
	req.Name = "Credential renamed"
	req.Processor.Authentication = created.Processor.Authentication
	updated := updateType(s.T(), created.ID, req)
	s.Equal("Credential renamed", updated.Name)
	s.Equal(maskedSecret, updated.Processor.Authentication.Properties["token"])

	// A new endpoint needs the credential again.
	req.Processor = &processorConfig{Endpoint: "https://other.example.com/rar"}
	status, body := doJSON(s.T(), http.MethodPut, typesPath+"/"+created.ID, req)
	s.Equal(http.StatusBadRequest, status, string(body))
	s.Equal(codeProcessorAuthenticationRequired, errorCode(s.T(), body))
}

// uniqueTypeValue returns a type value no other test run has registered.
func uniqueTypeValue(name string) string {
	return fmt.Sprintf("https://types.example.com/%s/%d", name, time.Now().UnixNano())
}

// createType registers a type and returns it as the management API returns it.
func createType(t *testing.T, req typeRequest) typeResponse {
	t.Helper()
	status, body := doJSON(t, http.MethodPost, typesPath, req)
	if status != http.StatusCreated {
		t.Fatalf("creating type %s: status %d, body %s", req.Type, status, body)
	}
	var created typeResponse
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decoding created type: %v", err)
	}
	return created
}

// updateType replaces a type and returns it as the management API returns it.
func updateType(t *testing.T, id string, req typeRequest) typeResponse {
	t.Helper()
	status, body := doJSON(t, http.MethodPut, typesPath+"/"+id, req)
	if status != http.StatusOK {
		t.Fatalf("updating type %s: status %d, body %s", id, status, body)
	}
	var updated typeResponse
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("decoding updated type: %v", err)
	}
	return updated
}

// deleteType removes a type, logging rather than failing when it cannot.
func deleteType(t *testing.T, id string) {
	t.Helper()
	if status, body := doJSON(t, http.MethodDelete, typesPath+"/"+id, nil); status != http.StatusNoContent {
		t.Logf("Failed to delete authorization details type %s: status %d, body %s", id, status, body)
	}
}

// deleteIfCreated removes the type a create request unexpectedly created, so a failing test leaves nothing
// behind.
func deleteIfCreated(t *testing.T, status int, body []byte) {
	t.Helper()
	if status != http.StatusCreated {
		return
	}
	var created typeResponse
	if err := json.Unmarshal(body, &created); err == nil && created.ID != "" {
		deleteType(t, created.ID)
	}
}

// listAllTypes returns every registered type, paging through the list.
func listAllTypes(t *testing.T) []typeResponse {
	t.Helper()
	const pageSize = 100
	var all []typeResponse
	for offset := 0; ; offset += pageSize {
		page := listTypes(t, pageSize, offset)
		all = append(all, page.Types...)
		if len(page.Types) == 0 || offset+pageSize >= page.TotalResults {
			return all
		}
	}
}

// listTypes returns one page of the registered types.
func listTypes(t *testing.T, limit, offset int) typeListResponse {
	t.Helper()
	query := url.Values{}
	query.Set("limit", fmt.Sprint(limit))
	query.Set("offset", fmt.Sprint(offset))
	status, body := doJSON(t, http.MethodGet, typesPath+"?"+query.Encode(), nil)
	if status != http.StatusOK {
		t.Fatalf("listing types: status %d, body %s", status, body)
	}
	var page typeListResponse
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("decoding type list: %v", err)
	}
	return page
}

// typeIDs returns the ids of the given types.
func typeIDs(types []typeResponse) []string {
	ids := make([]string, 0, len(types))
	for _, t := range types {
		ids = append(ids, t.ID)
	}
	return ids
}

// errorCode returns the code of a management API error body.
func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var resp errorResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decoding error response %s: %v", body, err)
	}
	return resp.Code
}

// doJSON sends an authenticated request with a JSON body, nil sending none, and returns the status and body.
func doJSON(t *testing.T, method, path string, payload interface{}) (int, []byte) {
	t.Helper()
	var body []byte
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("encoding request body: %v", err)
		}
		body = encoded
	}
	return doRaw(t, method, path, body)
}

// doRaw sends an authenticated request with the given body and returns the status and body.
func doRaw(t *testing.T, method, path string, body []byte) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		t.Fatalf("sending %s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	return resp.StatusCode, respBody
}
