// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outype

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
)

type OUTypeHandlerTestSuite struct {
	suite.Suite
	service *OUTypeServiceInterfaceMock
	mux     *http.ServeMux
}

func TestOUTypeHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(OUTypeHandlerTestSuite))
}

func (s *OUTypeHandlerTestSuite) SetupTest() {
	s.service = NewOUTypeServiceInterfaceMock(s.T())
	s.mux = http.NewServeMux()
	registerRoutes(s.mux, newOUTypeHandler(s.service))
}

func (s *OUTypeHandlerTestSuite) serve(method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w
}

func (s *OUTypeHandlerTestSuite) requireError(w *httptest.ResponseRecorder, status int, code string) {
	s.T().Helper()
	s.Require().Equal(status, w.Code, w.Body.String())
	var errResp apierror.ErrorResponse
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &errResp))
	s.Equal(code, errResp.Code)
}

func (s *OUTypeHandlerTestSuite) TestList() {
	s.service.On("GetOUTypeList", mock.Anything, 10, 5, true).
		Return(&OUTypeListResponse{TotalResults: 1, Count: 1, Types: []OUTypeListItem{{ID: testOUTypeID}}}, nil).
		Once()

	w := s.serve(http.MethodGet, "/organization-unit-types?limit=10&offset=5&include=display", "")

	s.Require().Equal(http.StatusOK, w.Code)
	var list OUTypeListResponse
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &list))
	s.Equal(testOUTypeID, list.Types[0].ID)
}

func (s *OUTypeHandlerTestSuite) TestList_DefaultPagination() {
	s.service.On("GetOUTypeList", mock.Anything, serverconst.DefaultPageSize, 0, false).
		Return(&OUTypeListResponse{}, nil).Once()

	w := s.serve(http.MethodGet, "/organization-unit-types", "")

	s.Equal(http.StatusOK, w.Code)
}

func (s *OUTypeHandlerTestSuite) TestList_InvalidPagination() {
	cases := []struct {
		query string
		code  string
	}{
		{"limit=0", ErrorInvalidLimit.Code},
		{"limit=abc", ErrorInvalidLimit.Code},
		{"offset=-1", ErrorInvalidOffset.Code},
		{"offset=abc", ErrorInvalidOffset.Code},
	}
	for _, tc := range cases {
		s.Run(tc.query, func() {
			s.requireError(s.serve(http.MethodGet, "/organization-unit-types?"+tc.query, ""),
				http.StatusBadRequest, tc.code)
		})
	}
}

func (s *OUTypeHandlerTestSuite) TestCreate() {
	s.service.On("CreateOUType", mock.Anything, CreateOUTypeRequest{
		Handle:           "partner",
		DisplayName:      "Partner",
		OUID:             testOUID,
		SystemAttributes: &SystemAttributes{Display: "registrationNo"},
		Schema:           testSchema,
	}).Return(&OUType{ID: testOUTypeID, Handle: "partner"}, nil).Once()

	w := s.serve(http.MethodPost, "/organization-unit-types", `{"handle":"partner","displayName":"Partner",`+
		`"ouId":"`+testOUID+`","systemAttributes":{"display":"registrationNo"},"schema":`+string(testSchema)+`}`)

	s.Require().Equal(http.StatusCreated, w.Code, w.Body.String())
	var created OUType
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &created))
	s.Equal(testOUTypeID, created.ID)
}

func (s *OUTypeHandlerTestSuite) TestCreate_RejectsUnsupportedFields() {
	cases := []struct {
		name string
		body string
	}{
		{"self registration", `{"handle":"partner","displayName":"Partner","ouId":"` + testOUID +
			`","allowSelfRegistration":true,"schema":{}}`},
		{"primary email", `{"handle":"partner","displayName":"Partner","ouId":"` + testOUID +
			`","systemAttributes":{"primaryEmail":"email"},"schema":{}}`},
		{"malformed body", `{"handle":`},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.requireError(s.serve(http.MethodPost, "/organization-unit-types", tc.body),
				http.StatusBadRequest, ErrorInvalidRequestFormat.Code)
		})
	}
}

func (s *OUTypeHandlerTestSuite) TestCreate_MissingRequiredField() {
	w := s.serve(http.MethodPost, "/organization-unit-types", `{"displayName":"Partner","ouId":"`+testOUID+
		`","schema":{}}`)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *OUTypeHandlerTestSuite) TestGet() {
	s.service.On("GetOUType", mock.Anything, testOUTypeID, false).
		Return(&OUType{ID: testOUTypeID}, nil).Once()

	w := s.serve(http.MethodGet, "/organization-unit-types/"+testOUTypeID, "")

	s.Equal(http.StatusOK, w.Code)
}

func (s *OUTypeHandlerTestSuite) TestUpdate() {
	s.service.On("UpdateOUType", mock.Anything, testOUTypeID, UpdateOUTypeRequest{
		DisplayName: "Partner", OUID: testOUID, Schema: testSchema,
	}).Return(&OUType{ID: testOUTypeID}, nil).Once()

	w := s.serve(http.MethodPut, "/organization-unit-types/"+testOUTypeID,
		`{"displayName":"Partner","ouId":"`+testOUID+`","schema":`+string(testSchema)+`}`)

	s.Equal(http.StatusOK, w.Code, w.Body.String())
}

func (s *OUTypeHandlerTestSuite) TestUpdate_RejectsSelfRegistration() {
	w := s.serve(http.MethodPut, "/organization-unit-types/"+testOUTypeID,
		`{"displayName":"Partner","ouId":"`+testOUID+`","allowSelfRegistration":true,"schema":{}}`)

	s.requireError(w, http.StatusBadRequest, ErrorInvalidRequestFormat.Code)
}

func (s *OUTypeHandlerTestSuite) TestDelete() {
	s.service.On("DeleteOUType", mock.Anything, testOUTypeID).Return(nil).Once()

	w := s.serve(http.MethodDelete, "/organization-unit-types/"+testOUTypeID, "")

	s.Equal(http.StatusNoContent, w.Code)
}

func (s *OUTypeHandlerTestSuite) TestErrorStatuses() {
	cases := []struct {
		err    *tidcommon.ServiceError
		status int
	}{
		{&ErrorOUTypeNotFound, http.StatusNotFound},
		{&ErrorOUTypeHandleConflict, http.StatusConflict},
		{&ErrorCannotModifyDeclarativeOUType, http.StatusForbidden},
		{&ErrorCannotModifyManagedOUType, http.StatusForbidden},
		{&tidcommon.ErrorUnauthorized, http.StatusForbidden},
		{&ErrorCredentialAttributeNotAllowed, http.StatusBadRequest},
		{&tidcommon.InternalServerError, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		s.Run(tc.err.Code, func() {
			s.service.On("DeleteOUType", mock.Anything, testOUTypeID).Return(tc.err).Once()

			s.requireError(s.serve(http.MethodDelete, "/organization-unit-types/"+testOUTypeID, ""),
				tc.status, tc.err.Code)
		})
	}
}

func (s *OUTypeHandlerTestSuite) TestPreflight() {
	for _, target := range []string{"/organization-unit-types", "/organization-unit-types/" + testOUTypeID} {
		s.Equal(http.StatusNoContent, s.serve(http.MethodOptions, target, "").Code, target)
	}
}
