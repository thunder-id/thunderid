// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outype

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
	ouTypeBasePath   = "/organization-unit-types"
	userTypeBasePath = "/user-types"

	invalidRequestFormatCode = "OUT-1001"
	ouTypeNotFoundCode       = "OUT-1002"
	handleConflictCode       = "OUT-1003"
	invalidOUTypeCode        = "OUT-1004"
	credentialNotAllowedCode = "OUT-1009"
	userTypeNotFoundCode     = "USRS-1002"
	ouCannotBeDeletedCode    = "OU-1006"
)

// partnerSchema is an organization unit type schema without credentials.
var partnerSchema = json.RawMessage(`{
	"registrationNo": {"type": "string", "required": true, "unique": true},
	"tier": {"type": "string", "enum": ["silver", "gold"]}
}`)

// OUTypeAPITestSuite covers the /organization-unit-types API. Every organization unit type a test
// creates is its own, in the suite's organization unit, and is deleted by the test.
type OUTypeAPITestSuite struct {
	suite.Suite
	ouID string
}

func TestOUTypeAPITestSuite(t *testing.T) {
	suite.Run(t, new(OUTypeAPITestSuite))
}

func (s *OUTypeAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "ou-type-api-ou",
		Name:        "OU Type API OU",
		Description: "Organization unit that owns the organization unit types of this suite",
	})
	s.Require().NoError(err)
	s.ouID = ouID
}

func (s *OUTypeAPITestSuite) TearDownSuite() {
	if s.ouID != "" {
		s.NoError(testutils.DeleteOrganizationUnit(s.ouID), "teardown: delete OU")
	}
}

// do sends a request with the admin client and returns the response.
func (s *OUTypeAPITestSuite) do(method, path string, body any) *http.Response {
	s.T().Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		s.Require().NoError(err)
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	s.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	s.Require().NoError(err)
	return resp
}

// requireError asserts the status and the product error code of a response.
func (s *OUTypeAPITestSuite) requireError(resp *http.Response, status int, code string) {
	s.T().Helper()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.Require().Equal(status, resp.StatusCode, "body: %s", string(body))
	var errResp ErrorResponse
	s.Require().NoError(json.Unmarshal(body, &errResp), "body: %s", string(body))
	s.Equal(code, errResp.Code, "body: %s", string(body))
}

// create creates an organization unit type and registers its deletion.
func (s *OUTypeAPITestSuite) create(request OUTypeRequest) OUType {
	s.T().Helper()

	resp := s.do(http.MethodPost, ouTypeBasePath, request)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "body: %s", string(body))

	var created OUType
	s.Require().NoError(json.Unmarshal(body, &created))
	s.Require().NotEmpty(created.ID)
	s.T().Cleanup(func() {
		resp := s.do(http.MethodDelete, ouTypeBasePath+"/"+created.ID, nil)
		resp.Body.Close()
	})
	return created
}

// partnerRequest is a request for a partner type in the suite's organization unit.
func (s *OUTypeAPITestSuite) partnerRequest(handle string) OUTypeRequest {
	return OUTypeRequest{Handle: handle, DisplayName: "Partner", OUID: s.ouID, Schema: partnerSchema}
}

func (s *OUTypeAPITestSuite) TestCreateGetUpdateDelete() {
	created := s.create(s.partnerRequest("ou-type-crud"))
	s.Equal("ou-type-crud", created.Handle)
	s.Equal(s.ouID, created.OUID)

	resp := s.do(http.MethodGet, ouTypeBasePath+"/"+created.ID, nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	fetchedBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	var fetched OUType
	s.Require().NoError(json.Unmarshal(fetchedBody, &fetched))
	s.Equal(created.ID, fetched.ID)
	s.JSONEq(string(partnerSchema), string(fetched.Schema))
	var fields map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(fetchedBody, &fields))
	s.NotContains(fields, "allowSelfRegistration", "organization unit types do not support self registration")

	updatedSchema := json.RawMessage(`{"registrationNo":{"type":"string","required":true,"unique":true},` +
		`"region":{"type":"string"}}`)
	resp = s.do(http.MethodPut, ouTypeBasePath+"/"+created.ID, OUTypeRequest{
		DisplayName: "Partner organizations", OUID: s.ouID, Schema: updatedSchema,
	})
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var updated OUType
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&updated))
	resp.Body.Close()
	s.Equal("Partner organizations", updated.DisplayName)
	s.JSONEq(string(updatedSchema), string(updated.Schema))

	resp = s.do(http.MethodDelete, ouTypeBasePath+"/"+created.ID, nil)
	resp.Body.Close()
	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	s.requireError(s.do(http.MethodGet, ouTypeBasePath+"/"+created.ID, nil),
		http.StatusNotFound, ouTypeNotFoundCode)
}

func (s *OUTypeAPITestSuite) TestListIncludesOnlyOUTypes() {
	created := s.create(s.partnerRequest("ou-type-list"))

	resp := s.do(http.MethodGet, ouTypeBasePath+"?limit=100", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var ouTypes OUTypeListResponse
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&ouTypes))
	resp.Body.Close()
	s.Contains(listIDs(ouTypes), created.ID)

	resp = s.do(http.MethodGet, userTypeBasePath+"?limit=100", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var userTypes OUTypeListResponse
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&userTypes))
	resp.Body.Close()
	s.NotContains(listIDs(userTypes), created.ID, "an organization unit type is not a user type")

	s.requireError(s.do(http.MethodGet, userTypeBasePath+"/"+created.ID, nil),
		http.StatusNotFound, userTypeNotFoundCode)
}

func (s *OUTypeAPITestSuite) TestCreateRejectsDuplicateHandle() {
	s.create(s.partnerRequest("ou-type-duplicate"))

	s.requireError(s.do(http.MethodPost, ouTypeBasePath, s.partnerRequest("ou-type-duplicate")),
		http.StatusConflict, handleConflictCode)
}

func (s *OUTypeAPITestSuite) TestCreateRejectsCredentialAttribute() {
	schemas := map[string]string{
		"top level":    `{"apiKey":{"type":"string","credential":true}}`,
		"in an object": `{"contact":{"type":"object","properties":{"apiKey":{"type":"string","credential":true}}}}`,
		"in array items": `{"keys":{"type":"array","items":{"type":"object","properties":` +
			`{"secret":{"type":"string","credential":true}}}}}`,
	}
	for name, schema := range schemas {
		s.Run(name, func() {
			request := s.partnerRequest("ou-type-credential")
			request.Schema = json.RawMessage(schema)

			s.requireError(s.do(http.MethodPost, ouTypeBasePath, request),
				http.StatusBadRequest, credentialNotAllowedCode)
		})
	}
}

func (s *OUTypeAPITestSuite) TestCreateRejectsEmptySchema() {
	request := s.partnerRequest("ou-type-empty")
	request.Schema = json.RawMessage(`{}`)

	s.requireError(s.do(http.MethodPost, ouTypeBasePath, request),
		http.StatusBadRequest, invalidOUTypeCode)
}

func (s *OUTypeAPITestSuite) TestSelfRegistrationIsRejected() {
	created := s.create(s.partnerRequest("ou-type-self-registration"))

	s.requireError(s.do(http.MethodPost, ouTypeBasePath, map[string]any{
		"handle":                "ou-type-self-registration-new",
		"displayName":           "Partner",
		"ouId":                  s.ouID,
		"allowSelfRegistration": true,
		"schema":                partnerSchema,
	}), http.StatusBadRequest, invalidRequestFormatCode)

	s.requireError(s.do(http.MethodPut, ouTypeBasePath+"/"+created.ID, map[string]any{
		"displayName":           "Partner",
		"ouId":                  s.ouID,
		"allowSelfRegistration": true,
		"schema":                partnerSchema,
	}), http.StatusBadRequest, invalidRequestFormatCode)
}

func (s *OUTypeAPITestSuite) TestOwnerOUCannotBeDeletedWhileItHasOUTypes() {
	ownerID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "ou-type-owner", Name: "OU Type Owner",
	})
	s.Require().NoError(err)
	ownerDeleted := false
	// Registered before s.create, so the type's cleanup runs first and the owner can then be deleted.
	s.T().Cleanup(func() {
		if !ownerDeleted {
			_ = testutils.DeleteOrganizationUnit(ownerID)
		}
	})

	request := s.partnerRequest("ou-type-owned")
	request.OUID = ownerID
	created := s.create(request)

	s.requireError(s.do(http.MethodDelete, "/organization-units/"+ownerID, nil),
		http.StatusBadRequest, ouCannotBeDeletedCode)

	resp := s.do(http.MethodDelete, ouTypeBasePath+"/"+created.ID, nil)
	resp.Body.Close()
	s.Require().Equal(http.StatusNoContent, resp.StatusCode)
	s.Require().NoError(testutils.DeleteOrganizationUnit(ownerID))
	ownerDeleted = true
}

// listIDs returns the IDs in a list response.
func listIDs(list OUTypeListResponse) []string {
	ids := make([]string, 0, len(list.Types))
	for _, item := range list.Types {
		ids = append(ids, item.ID)
	}
	return ids
}
