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
	"strings"
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
	codeInvalidSchema = "RAR-1006"
	// codeTypeConflict is returned for a type value another type already has.
	codeTypeConflict = "RAR-1003"
	// codeProcessorAuthenticationRequired is returned when a processor endpoint changes without its credential.
	codeProcessorAuthenticationRequired = "RAR-1009"
	// codeInvalidRequestFormat is returned for a request body that cannot be decoded.
	codeInvalidRequestFormat = "RAR-1001"
	// codeTypeNotFound is returned when no type has the given id.
	codeTypeNotFound = "RAR-1002"
	// codeInvalidTypeValue is returned for a missing or malformed type value.
	codeInvalidTypeValue = "RAR-1004"
	// codeInvalidName is returned for a missing or overlong name.
	codeInvalidName = "RAR-1005"
	// codeInvalidProcessor is returned for a processor endpoint the registry refuses.
	codeInvalidProcessor = "RAR-1007"
	// codeInvalidProcessorAuthentication is returned for a processor authentication the registry refuses.
	codeInvalidProcessorAuthentication = "RAR-1008"
	// codeTypeValueImmutable is returned when an update changes the type value.
	codeTypeValueImmutable = "RAR-1010"
	// codeInvalidConsentTemplate is returned for a consent template the registry refuses.
	codeInvalidConsentTemplate = "RAR-1011"
	// codeInvalidPagination is returned for a limit or offset out of range.
	codeInvalidPagination = "RAR-1012"
	// codeUnauthorized is returned to a caller that may not perform the operation.
	codeUnauthorized = "SSE-4030"
	// codeImportTemplateFailed is returned by an import whose content is not a valid template.
	codeImportTemplateFailed = "IMP-1003"

	// typeResourceType is the import resource type of an authorization details type.
	typeResourceType = "authorization_detail_type"
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

	// A name change keeps the version; a schema or reuse change moves it, since earlier approvals no longer apply.
	renamed := typeRequest{Type: value, Name: "Flight", Schema: flightSchema, ConsentTemplate: "Book {{route}}",
		Reuse: true}
	updated := updateType(s.T(), created.ID, renamed)
	s.Equal("Flight", updated.Name)
	s.Equal(1, updated.Version)

	renamed.Schema = json.RawMessage(`{"type":"object","properties":{"route":{"type":"string"}}}`)
	updated = updateType(s.T(), created.ID, renamed)
	s.Equal(2, updated.Version)

	renamed.Reuse = false
	updated = updateType(s.T(), created.ID, renamed)
	s.Equal(3, updated.Version)

	s.Require().NoError(testutils.DeleteImportedResource(typeResourceType, created.ID))
	status, _ = doJSON(s.T(), http.MethodGet, typesPath+"/"+created.ID, nil)
	s.Equal(http.StatusNotFound, status)
	s.NoError(testutils.DeleteImportedResource(typeResourceType, created.ID),
		"deleting a type that does not exist succeeds")
}

// A data plane takes no type from a token, whatever its scopes: the type is written only through an import
// with the management API key.
func (s *TypeAPITestSuite) TestTokenWritesAreForbidden() {
	created := createType(s.T(), typeRequest{Type: uniqueTypeValue("token"), Name: "Token", Schema: flightSchema})
	defer deleteType(s.T(), created.ID)

	status, body := doJSON(s.T(), http.MethodPost, typesPath,
		typeRequest{Type: uniqueTypeValue("token"), Name: "Token", Schema: flightSchema})
	deleteIfCreated(s.T(), status, body)
	s.Equal(http.StatusForbidden, status, string(body))
	s.Equal(codeUnauthorized, errorCode(s.T(), body))

	// A type is sent back as it was fetched.
	fetched, err := json.Marshal(created)
	s.Require().NoError(err)
	status, body = doRaw(s.T(), http.MethodPut, typesPath+"/"+created.ID, fetched)
	s.Equal(http.StatusForbidden, status, string(body))
	s.Equal(codeUnauthorized, errorCode(s.T(), body))

	status, body = doJSON(s.T(), http.MethodDelete, typesPath+"/"+created.ID, nil)
	s.Equal(http.StatusForbidden, status, string(body))
	s.Equal(codeUnauthorized, errorCode(s.T(), body))

	s.Equal(created, getType(s.T(), created.ID), "the refused writes changed nothing")
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
		"a reference": json.RawMessage(
			`{"type":"object","properties":{"route":{"$ref":"#/$defs/route"}}}`),
		"a schema that is not JSON":    json.RawMessage(`"not a schema"`),
		"a root that is not an object": json.RawMessage(`{"type":"string"}`),
		"another JSON Schema draft": json.RawMessage(
			`{"$schema":"http://json-schema.org/draft-07/schema#","type":"object"}`),
		"a type naming two types":       objectSchema(`"route":{"type":["string","null"]}`),
		"an unknown type":               objectSchema(`"route":{"type":"text"}`),
		"an integer type":               objectSchema(`"count":{"type":"integer"}`),
		"a null type":                   objectSchema(`"gone":{"type":"null"}`),
		"a property without a type":     objectSchema(`"route":{"title":"Route"}`),
		"an empty title":                objectSchema(`"route":{"type":"string","title":""}`),
		"an empty enum":                 objectSchema(`"route":{"type":"string","enum":[]}`),
		"an enum listing a value twice": objectSchema(`"route":{"type":"string","enum":["a","a"]}`),
		"an enum value padded":          objectSchema(`"route":{"type":"string","enum":[" a"]}`),
		"an unsupported format":         objectSchema(`"route":{"type":"string","format":"email"}`),
		"a property name with a space":  objectSchema(`" route":{"type":"string"}`),
		"a property name with a dot":    objectSchema(`"route.code":{"type":"string"}`),
		"a property that is no schema":  objectSchema(`"route":"string"`),
		"properties that are no object": json.RawMessage(`{"type":"object","properties":["route"]}`),
		"items that are no schema":      objectSchema(`"lines":{"type":"array","items":"string"}`),
		"required that is no list":      json.RawMessage(`{"type":"object","required":"route"}`),
		"required naming no property":   json.RawMessage(`{"type":"object","required":["route"]}`),
		"a number no double holds":      objectSchema(`"amount":{"type":"number","maximum":1e400}`),
		"multipleOf zero":               objectSchema(`"amount":{"type":"number","multipleOf":0}`),
		"multipleOf below zero":         objectSchema(`"amount":{"type":"number","multipleOf":-2}`),
		"minimum above maximum":         objectSchema(`"amount":{"type":"number","minimum":10,"maximum":1}`),
		"exclusive bounds reversed": objectSchema(
			`"amount":{"type":"number","exclusiveMinimum":5,"exclusiveMaximum":4}`),
		"minLength above maxLength": objectSchema(`"code":{"type":"string","minLength":5,"maxLength":2}`),
		"a negative minLength":      objectSchema(`"code":{"type":"string","minLength":-1}`),
		"minItems above maxItems": objectSchema(
			`"tags":{"type":"array","items":{"type":"string"},"minItems":3,"maxItems":1}`),
		"a negative maxItems":            objectSchema(`"tags":{"type":"array","items":{"type":"string"},"maxItems":-1}`),
		"a type member that is a number": objectSchema(`"type":{"type":"number"}`),
		"a type member with a format":    objectSchema(`"type":{"type":"string","format":"date"}`),
		"a type member enum of numbers":  objectSchema(`"type":{"enum":[1]}`),
		"a type member const of another type": objectSchema(
			`"type":{"type":"string","const":"https://types.example.com/another"}`),
		"a type member enum without the type": objectSchema(
			`"type":{"type":"string","enum":["https://types.example.com/another"]}`),
		"a policy setting spelled otherwise": objectSchema(
			`"amount":{"type":"number","policy":{"SUPPLIEDBY":"client"}}`),
		"a policy that is no object": objectSchema(`"amount":{"type":"number","policy":"decrease"}`),
		"an unknown suppliedBy":      objectSchema(`"amount":{"type":"number","policy":{"suppliedBy":"user"}}`),
		"an unknown modification": objectSchema(
			`"amount":{"type":"number","policy":{"modification":"halve","modifiableBy":["user"]}}`),
		"an unknown party": objectSchema(
			`"amount":{"type":"number","policy":{"modification":"decrease","modifiableBy":["admin"]}}`),
		"a party named twice": objectSchema(
			`"amount":{"type":"number","policy":{"modification":"decrease","modifiableBy":["user","user"]}}`),
		"an empty modifiableBy": objectSchema(
			`"amount":{"type":"number","policy":{"modification":"decrease","modifiableBy":[]}}`),
		"modifiableBy without a modification": objectSchema(
			`"amount":{"type":"number","policy":{"modifiableBy":["user"]}}`),
		"a modification without modifiableBy": objectSchema(
			`"amount":{"type":"number","policy":{"modification":"decrease"}}`),
		"decrease on a plain string": objectSchema(
			`"code":{"type":"string","policy":{"modification":"decrease","modifiableBy":["user"]}}`),
		"removeItems on a number": objectSchema(
			`"amount":{"type":"number","policy":{"modification":"removeItems","modifiableBy":["user"]}}`),
		"the user changing a date and time": objectSchema(
			`"until":{"type":"string","format":"date-time",` +
				`"policy":{"modification":"decrease","modifiableBy":["user"]}}`),
		"policy limits on a number": objectSchema(
			`"amount":{"type":"number","policy":{"minimum":1}}`),
		"policy limits on a plain string": objectSchema(
			`"code":{"type":"string","policy":{"maximum":"z"}}`),
		"the type supplied by the processor": objectSchema(
			`"type":{"type":"string","policy":{"suppliedBy":"processor"}}`),
		"a processor-supplied field that changes": objectSchema(
			`"amount":{"type":"number","policy":{"suppliedBy":"processor","modification":"decrease",` +
				`"modifiableBy":["user"]}}`),
	}
	for name, schema := range cases {
		item := importType(s.T(), "", typeRequest{Type: uniqueTypeValue("invalid"), Name: "Invalid", Schema: schema})
		deleteIfImported(s.T(), item)
		want := codeInvalidSchema
		if name == "a schema that is not JSON" {
			// An import refuses a schema that is not JSON while reading the document.
			want = codeImportInvalidYAML
		}
		s.Equal("failed", item.Status, name)
		s.Equal(want, item.Code, "%s: %s", name, item.Message)
	}
}

// A nested field cannot change inside an object the processor supplies, and a field the processor supplies
// or changes needs the type to have a processor.
func (s *TypeAPITestSuite) TestProcessorPoliciesAreRefusedWhereTheyCannotApply() {
	processor := &processorConfig{Endpoint: "https://processor.example.com/rar"}
	nested := objectSchema(`"account":{"type":"object","policy":{"suppliedBy":"processor"},"properties":{` +
		`"limit":{"type":"number","policy":{"modification":"decrease","modifiableBy":["user"]}}}}`)
	item := importType(s.T(), "", typeRequest{Type: uniqueTypeValue("nested"), Name: "Nested", Schema: nested,
		Processor: processor})
	deleteIfImported(s.T(), item)
	s.Equal("failed", item.Status, item.Message)
	s.Equal(codeInvalidSchema, item.Code)

	processorSupplied := objectSchema(`"balance":{"type":"number","policy":{"suppliedBy":"processor"}}`)
	item = importType(s.T(), "", typeRequest{Type: uniqueTypeValue("no-processor"), Name: "No processor",
		Schema: processorSupplied})
	deleteIfImported(s.T(), item)
	s.Equal("failed", item.Status, item.Message)
	s.Equal(codeInvalidSchema, item.Code)

	processorChanged := objectSchema(`"amount":{"type":"number",` +
		`"policy":{"modification":"decrease","modifiableBy":["processor"]}}`)
	item = importType(s.T(), "", typeRequest{Type: uniqueTypeValue("no-processor"), Name: "No processor",
		Schema: processorChanged})
	deleteIfImported(s.T(), item)
	s.Equal("failed", item.Status, item.Message)
	s.Equal(codeInvalidSchema, item.Code)

	// With a processor, the same field is accepted.
	created := createType(s.T(), typeRequest{Type: uniqueTypeValue("with-processor"), Name: "With processor",
		Schema: processorSupplied, Processor: processor})
	deleteType(s.T(), created.ID)
}

// The type value, name, consent template and processor endpoint are checked at registration.
func (s *TypeAPITestSuite) TestDefinitionFieldsAreChecked() {
	longText := strings.Repeat("a", 1001)
	cases := []struct {
		name string
		req  typeRequest
		code string
	}{
		{"an empty type value", typeRequest{Type: "", Name: "Empty", Schema: flightSchema}, codeInvalidTypeValue},
		{"a type value with a space", typeRequest{Type: "https://types.example.com/with space", Name: "Space",
			Schema: flightSchema}, codeInvalidTypeValue},
		{"a type value over 512 characters", typeRequest{Type: "https://types.example.com/" +
			strings.Repeat("v", 512), Name: "Long", Schema: flightSchema}, codeInvalidTypeValue},
		{"a type value outside ASCII", typeRequest{Type: "https://types.example.com/café", Name: "Accent",
			Schema: flightSchema}, codeInvalidTypeValue},
		{"an empty name", typeRequest{Type: uniqueTypeValue("name"), Name: "  ", Schema: flightSchema},
			codeInvalidName},
		{"a name over 100 characters", typeRequest{Type: uniqueTypeValue("name"), Name: strings.Repeat("n", 101),
			Schema: flightSchema}, codeInvalidName},
		{"no schema", typeRequest{Type: uniqueTypeValue("schema"), Name: "No schema"}, codeInvalidSchema},
		{"a schema over 64 KiB", typeRequest{Type: uniqueTypeValue("schema"), Name: "Large schema",
			Schema: json.RawMessage(`{"type":"object","description":"` + strings.Repeat("d", 64<<10) + `"}`)},
			codeInvalidSchema},
		{"a template naming an unknown field", typeRequest{Type: uniqueTypeValue("template"), Name: "Template",
			Schema: flightSchema, ConsentTemplate: "Book {{destination}}"}, codeInvalidConsentTemplate},
		// An import reads its content as a template, so it refuses this one before the registry sees it.
		{"a template with a space inside a name", typeRequest{Type: uniqueTypeValue("template"),
			Name: "Template", Schema: flightSchema, ConsentTemplate: "Book {{max fare}}"},
			codeImportTemplateFailed},
		{"a template over 1000 bytes", typeRequest{Type: uniqueTypeValue("template"), Name: "Template",
			Schema: flightSchema, ConsentTemplate: longText}, codeInvalidConsentTemplate},
		{"an HTTP processor off loopback", typeRequest{Type: uniqueTypeValue("processor"), Name: "Processor",
			Schema: flightSchema, Processor: &processorConfig{Endpoint: "http://processor.example.com/rar"}},
			codeInvalidProcessor},
		{"a processor with user information", typeRequest{Type: uniqueTypeValue("processor"),
			Name: "Processor", Schema: flightSchema,
			Processor: &processorConfig{Endpoint: "https://user:pass@processor.example.com/rar"}},
			codeInvalidProcessor},
		{"a processor on port 0", typeRequest{Type: uniqueTypeValue("processor"), Name: "Processor",
			Schema: flightSchema, Processor: &processorConfig{Endpoint: "https://processor.example.com:0/rar"}},
			codeInvalidProcessor},
		{"a processor that is not HTTP", typeRequest{Type: uniqueTypeValue("processor"), Name: "Processor",
			Schema: flightSchema, Processor: &processorConfig{Endpoint: "ftp://processor.example.com/rar"}},
			codeInvalidProcessor},
		{"a relative processor", typeRequest{Type: uniqueTypeValue("processor"), Name: "Processor",
			Schema: flightSchema, Processor: &processorConfig{Endpoint: "/rar"}}, codeInvalidProcessor},
		// An import refuses an authentication it does not know while reading the document.
		{"an unknown processor authentication", typeRequest{Type: uniqueTypeValue("processor"),
			Name: "Processor", Schema: flightSchema, Processor: &processorConfig{
				Endpoint:       "https://processor.example.com/rar",
				Authentication: &processorAuthentication{Type: "kerberos"}}},
			codeImportInvalidYAML},
		{"a bearer authentication without a token", typeRequest{Type: uniqueTypeValue("processor"),
			Name: "Processor", Schema: flightSchema, Processor: &processorConfig{
				Endpoint:       "https://processor.example.com/rar",
				Authentication: &processorAuthentication{Type: "bearer"}}},
			codeInvalidProcessorAuthentication},
	}
	for _, tc := range cases {
		item := importType(s.T(), "", tc.req)
		deleteIfImported(s.T(), item)
		s.Equal("failed", item.Status, tc.name)
		s.Equal(tc.code, item.Code, "%s: %s", tc.name, item.Message)
	}
}

// A request body that cannot be read, or that names a field the API does not have, is refused before it
// reaches the registry, so a token is answered as on any server.
func (s *TypeAPITestSuite) TestMalformedRequestsAreRefused() {
	created := createType(s.T(), typeRequest{Type: uniqueTypeValue("malformed"), Name: "Malformed",
		Schema: flightSchema})
	defer deleteType(s.T(), created.ID)

	oversized := []byte(`{"type":"` + uniqueTypeValue("oversized") + `","name":"Oversized","schema":` +
		`{"type":"object","description":"` + strings.Repeat("d", 1<<20) + `"}}`)
	unknownField := []byte(fmt.Sprintf(`{"type":%q,"name":"Unknown","schema":%s,"owner":"someone"}`,
		uniqueTypeValue("unknown"), flightSchema))
	for name, body := range map[string][]byte{
		"a body that is not JSON":   []byte(`{"type":`),
		"a body that is a list":     []byte(`[]`),
		"an unknown top-level key":  unknownField,
		"a body over 1 MiB":         oversized,
		"a name that is not a text": []byte(`{"type":"https://types.example.com/n","name":5,"schema":{}}`),
	} {
		status, resp := doRaw(s.T(), http.MethodPost, typesPath, body)
		deleteIfCreated(s.T(), status, resp)
		s.Equal(http.StatusBadRequest, status, "%s: %s", name, resp)
		s.Equal(codeInvalidRequestFormat, errorCode(s.T(), resp), "POST %s", name)

		status, resp = doRaw(s.T(), http.MethodPut, typesPath+"/"+created.ID, body)
		s.Equal(http.StatusBadRequest, status, "%s: %s", name, resp)
		s.Equal(codeInvalidRequestFormat, errorCode(s.T(), resp), "PUT %s", name)
	}
}

// The limit and offset of a list must be whole numbers in range.
func (s *TypeAPITestSuite) TestBadPaginationIsRefused() {
	for _, query := range []string{"limit=abc", "offset=abc", "offset=-1", "limit=0", "limit=101", "limit=-5"} {
		status, body := doJSON(s.T(), http.MethodGet, typesPath+"?"+query, nil)
		s.Equal(http.StatusBadRequest, status, "%s: %s", query, body)
		s.Equal(codeInvalidPagination, errorCode(s.T(), body), query)
	}

	// Without either, the first page is listed.
	status, body := doJSON(s.T(), http.MethodGet, typesPath, nil)
	s.Require().Equal(http.StatusOK, status, string(body))
	var page typeListResponse
	s.Require().NoError(json.Unmarshal(body, &page))
	s.Equal(1, page.StartIndex)
}

// The type value of a registered type cannot change, and a type that does not exist is not found. An import
// creates a type it does not find, so only a read can miss one.
func (s *TypeAPITestSuite) TestTypeValueIsImmutableAndUnknownTypesAreNotFound() {
	value := uniqueTypeValue("immutable")
	created := createType(s.T(), typeRequest{Type: value, Name: "Immutable", Schema: flightSchema})
	defer deleteType(s.T(), created.ID)

	item := importType(s.T(), created.ID, typeRequest{Type: value + "-changed", Name: "Immutable",
		Schema: flightSchema})
	s.Equal("failed", item.Status, item.Message)
	s.Equal(codeTypeValueImmutable, item.Code)

	// A valid change to the other fields is still checked like a create.
	item = importType(s.T(), created.ID, typeRequest{Type: value, Name: "", Schema: flightSchema})
	s.Equal("failed", item.Status, item.Message)
	s.Equal(codeInvalidName, item.Code)

	const missing = "00000000-0000-7000-8000-00000000dead"
	status, body := doJSON(s.T(), http.MethodGet, typesPath+"/"+missing, nil)
	s.Equal(http.StatusNotFound, status, string(body))
	s.Equal(codeTypeNotFound, errorCode(s.T(), body))
}

// A processor reached over plain HTTP is accepted on a loopback host, and adding, sending back unchanged and
// removing it move the version only when the grant changes.
func (s *TypeAPITestSuite) TestLoopbackHTTPProcessorIsAccepted() {
	value := uniqueTypeValue("loopback")
	req := typeRequest{Type: value, Name: "Loopback", Schema: flightSchema,
		Processor: &processorConfig{Endpoint: "http://localhost:8080/rar"}}
	created := createType(s.T(), req)
	defer deleteType(s.T(), created.ID)
	s.Require().NotNil(created.Processor)
	s.Equal("http://localhost:8080/rar", created.Processor.Endpoint)
	s.Require().NotNil(created.Processor.Authentication)
	s.Equal("none", created.Processor.Authentication.Type)

	// Sending the type back as it was returned changes nothing.
	unchanged := updateType(s.T(), created.ID, req)
	s.Equal(created.Version, unchanged.Version)

	// Moving to another loopback address keeps the grant, while removing the processor changes it.
	req.Processor = &processorConfig{Endpoint: "http://127.0.0.1:9090/rar",
		Authentication: &processorAuthentication{Type: "basic",
			Properties: map[string]string{"username": "svc", "password": "loopback-secret"}}}
	moved := updateType(s.T(), created.ID, req)
	s.Equal(created.Version, moved.Version)
	s.Equal(maskedSecret, moved.Processor.Authentication.Properties["password"])
	s.Equal("svc", moved.Processor.Authentication.Properties["username"])

	req.Processor = nil
	removed := updateType(s.T(), created.ID, req)
	s.Nil(removed.Processor)
	s.Equal(created.Version+1, removed.Version, "removing the processor changes the grant")
}

// A date or date-time field may carry its own limits in its policy, which must be written as its values are
// and must not be reversed.
func (s *TypeAPITestSuite) TestDatePolicyLimits() {
	valid := objectSchema(`"until":{"type":"string","format":"date-time","policy":{"modification":"decrease",` +
		`"modifiableBy":["client"],"minimum":"2026-01-01T00:00:00Z","maximum":"2027-01-01T00:00:00+05:30"}},` +
		`"day":{"type":"string","format":"date","policy":{"modification":"increase","modifiableBy":["user"],` +
		`"minimum":"2026-01-01","maximum":"2026-12-31"}}`)
	created := createType(s.T(), typeRequest{Type: uniqueTypeValue("date-limits"), Name: "Date limits",
		Schema: valid, ConsentTemplate: "Until {{until}} from {{day}}"})
	deleteType(s.T(), created.ID)

	refused := map[string]json.RawMessage{
		"reversed date-time limits": objectSchema(`"until":{"type":"string","format":"date-time",` +
			`"policy":{"minimum":"2027-01-01T00:00:00Z","maximum":"2026-01-01T00:00:00Z"}}`),
		"a date-time limit without an offset": objectSchema(`"until":{"type":"string","format":"date-time",` +
			`"policy":{"minimum":"2026-01-01T00:00:00"}}`),
		"a date-time limit that is a date": objectSchema(`"until":{"type":"string","format":"date-time",` +
			`"policy":{"maximum":"2026-01-01"}}`),
		"a date-time limit that is a number": objectSchema(`"until":{"type":"string","format":"date-time",` +
			`"policy":{"maximum":20260101}}`),
		"reversed date limits": objectSchema(`"day":{"type":"string","format":"date",` +
			`"policy":{"minimum":"2026-12-31","maximum":"2026-01-01"}}`),
		"a malformed date limit": objectSchema(`"day":{"type":"string","format":"date",` +
			`"policy":{"minimum":"31/12/2026"}}`),
	}
	for name, schema := range refused {
		item := importType(s.T(), "",
			typeRequest{Type: uniqueTypeValue("date-limits"), Name: "Date limits", Schema: schema})
		deleteIfImported(s.T(), item)
		s.Equal("failed", item.Status, "%s: %+v", name, item)
		s.Equal(codeInvalidSchema, item.Code, "%s: %+v", name, item)
	}
}

// A schema that pins its type member to the registered type value is accepted, as are the limits and
// keywords a value can satisfy.
func (s *TypeAPITestSuite) TestSatisfiableSchemasAreAccepted() {
	value := uniqueTypeValue("pinned")
	schema := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object",` +
		`"required":["type","amount"],"properties":{` +
		`"type":{"type":"string","enum":["` + value + `","https://types.example.com/other"]},` +
		`"amount":{"type":"number","minimum":1,"maximum":1,"multipleOf":0.5},` +
		`"count":{"type":"number","multipleOf":1,"exclusiveMinimum":0,"exclusiveMaximum":10},` +
		`"code":{"type":"string","minLength":2,"maxLength":2,"pattern":"^[A-Z]+$"},` +
		`"tags":{"type":"array","items":{"type":"string"},"minItems":0,"maxItems":3,"uniqueItems":true,` +
		`"policy":{"modification":"removeItems","modifiableBy":["user"]}},` +
		`"excluded":{"type":"array","items":{"type":"string"},` +
		`"policy":{"modification":"addItems","modifiableBy":["client"]}},` +
		`"kind":{"const":"retail"},` +
		`"account":{"type":"object","properties":{"iban":{"type":"string","title":"IBAN"}}}}}`)
	created := createType(s.T(), typeRequest{Type: value, Name: "Pinned", Schema: schema,
		ConsentTemplate: "Pay {{amount}} from {{account.iban}} as {{ type }}"})
	defer deleteType(s.T(), created.ID)

	// The same definition with its members reordered keeps the version.
	reordered := json.RawMessage(`{"type":"object","$schema":"https://json-schema.org/draft/2020-12/schema",` +
		`"required":["amount","type"],"properties":{` +
		`"account":{"type":"object","properties":{"iban":{"title":"IBAN","type":"string"}}},` +
		`"kind":{"const":"retail"},` +
		`"excluded":{"type":"array","items":{"type":"string"},` +
		`"policy":{"modifiableBy":["client"],"modification":"addItems"}},` +
		`"tags":{"type":"array","items":{"type":"string"},"minItems":0,"maxItems":3,"uniqueItems":true,` +
		`"policy":{"modification":"removeItems","modifiableBy":["user"],"suppliedBy":"client"}},` +
		`"code":{"type":"string","minLength":2,"maxLength":2,"pattern":"^[A-Z]+$"},` +
		`"count":{"type":"number","multipleOf":1,"exclusiveMinimum":0,"exclusiveMaximum":10},` +
		`"amount":{"type":"number","minimum":1.0,"maximum":1,"multipleOf":0.50},` +
		`"type":{"type":"string","enum":["` + value + `","https://types.example.com/other"]}}}`)
	updated := updateType(s.T(), created.ID, typeRequest{Type: value, Name: "Pinned", Schema: reordered,
		ConsentTemplate: "Pay {{amount}} from {{account.iban}} as {{ type }}"})
	s.Equal(created.Version, updated.Version, "an equivalent schema keeps the version")
	s.JSONEq(string(reordered), string(updated.Schema), "the reordered schema is stored as sent")
}

// objectSchema returns an object schema with the given properties, written as JSON members.
func objectSchema(properties string) json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` + properties + `}}`)
}

func (s *TypeAPITestSuite) TestDuplicateTypeValueIsRefused() {
	value := uniqueTypeValue("duplicate")
	created := createType(s.T(), typeRequest{Type: value, Name: "Original", Schema: flightSchema})
	defer deleteType(s.T(), created.ID)

	item := importType(s.T(), "", typeRequest{Type: value, Name: "Copy", Schema: flightSchema})
	deleteIfImported(s.T(), item)
	s.Equal("failed", item.Status, item.Message)
	s.Equal(codeTypeConflict, item.Code)
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
	item := importType(s.T(), created.ID, req)
	s.Equal("failed", item.Status, item.Message)
	s.Equal(codeProcessorAuthenticationRequired, item.Code)

	// Authentication none sent at the same endpoint clears the stored credential.
	req.Processor = &processorConfig{Endpoint: endpoint, Authentication: &processorAuthentication{Type: "none"}}
	cleared := updateType(s.T(), created.ID, req)
	s.Require().NotNil(cleared.Processor)
	s.Require().NotNil(cleared.Processor.Authentication)
	s.Equal("none", cleared.Processor.Authentication.Type)
	s.Empty(cleared.Processor.Authentication.Properties)
}

// uniqueTypeValue returns a type value no other test run has registered.
func uniqueTypeValue(name string) string {
	return fmt.Sprintf("https://types.example.com/%s/%d", name, time.Now().UnixNano())
}

// typeImportDocument is an authorization details type as an import document.
type typeImportDocument struct {
	ResourceType string `json:"resource_type"`
	ID           string `json:"id,omitempty"`
	typeRequest
}

// importType writes a type through an import with the management API key, the only way a data plane takes
// one, and returns the outcome. A type with an id is updated, or created with that id when none has it. A
// request the import refuses as a whole is reported as a failed outcome carrying the refusal's code.
func importType(t *testing.T, id string, req typeRequest) testutils.ImportItem {
	t.Helper()
	document, err := json.Marshal(typeImportDocument{ResourceType: typeResourceType, ID: id, typeRequest: req})
	if err != nil {
		t.Fatalf("encoding type document: %v", err)
	}
	payload, err := json.Marshal(testutils.ImportRequest{Content: string(document),
		Options: testutils.ImportOptions{Upsert: true, Target: "runtime"}})
	if err != nil {
		t.Fatalf("encoding import request: %v", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/import", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("creating import request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetManagementAPIKeyHTTPClient().Do(httpReq)
	if err != nil {
		t.Fatalf("sending import request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading import response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return testutils.ImportItem{Status: "failed", Code: errorCode(t, body), Message: string(body)}
	}
	var importResp testutils.ImportResponse
	if err := json.Unmarshal(body, &importResp); err != nil {
		t.Fatalf("decoding import response %s: %v", body, err)
	}
	if len(importResp.Results) != 1 {
		t.Fatalf("expected one import result, got %s", body)
	}
	return importResp.Results[0]
}

// createType registers a type through an import and returns it as the management API returns it.
func createType(t *testing.T, req typeRequest) typeResponse {
	t.Helper()
	item := importType(t, "", req)
	if item.Status != "success" {
		t.Fatalf("creating type %s: %s %s", req.Type, item.Code, item.Message)
	}
	return getType(t, item.ResourceID)
}

// updateType replaces a type through an import and returns it as the management API returns it.
func updateType(t *testing.T, id string, req typeRequest) typeResponse {
	t.Helper()
	item := importType(t, id, req)
	if item.Status != "success" {
		t.Fatalf("updating type %s: %s %s", id, item.Code, item.Message)
	}
	return getType(t, id)
}

// getType returns a type as the management API returns it.
func getType(t *testing.T, id string) typeResponse {
	t.Helper()
	status, body := doJSON(t, http.MethodGet, typesPath+"/"+id, nil)
	if status != http.StatusOK {
		t.Fatalf("getting type %s: status %d, body %s", id, status, body)
	}
	var got typeResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decoding type: %v", err)
	}
	return got
}

// deleteType removes a type through an import, logging rather than failing when it cannot.
func deleteType(t *testing.T, id string) {
	t.Helper()
	if err := testutils.DeleteImportedResource(typeResourceType, id); err != nil {
		t.Logf("Failed to delete authorization details type %s: %v", id, err)
	}
}

// deleteIfImported removes the type an import unexpectedly created, so a failing test leaves nothing behind.
func deleteIfImported(t *testing.T, item testutils.ImportItem) {
	t.Helper()
	if item.Status == "success" && item.ResourceID != "" {
		deleteType(t, item.ResourceID)
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
	return doRawWith(t, testutils.GetHTTPClient(), method, path, body)
}

// doRawWith sends a request with the given client and body and returns the status and body.
func doRawWith(t *testing.T, client *http.Client, method, path string, body []byte) (int, []byte) {
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
	resp, err := client.Do(req)
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
