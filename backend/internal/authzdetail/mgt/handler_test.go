// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type HandlerTestSuite struct {
	suite.Suite
	service *AuthorizationDetailMgtServiceInterfaceMock
	handler *authorizationDetailHandler
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) SetupTest() {
	s.service = NewAuthorizationDetailMgtServiceInterfaceMock(s.T())
	s.handler = newAuthorizationDetailHandler(s.service)
}

func request(method, body, id string) *http.Request {
	r := httptest.NewRequest(method, "/authorization-detail-types", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if id != "" {
		r.SetPathValue("id", id)
	}
	return r
}

const typeRequestBody = `{"type":"` + testFlightType + `","name":"  Flight  ","schema":` + flightSchema +
	`,"consentTemplate":"Book {{route}}","reuse":false}`

func (s *HandlerTestSuite) TestPost() {
	s.service.EXPECT().CreateAuthorizationDetailType(mock.Anything,
		mock.MatchedBy(func(t providers.AuthorizationDetailType) bool {
			return t.Type == testFlightType && t.Name == "Flight" && t.ConsentTemplate == "Book {{route}}" &&
				!t.Reuse
		})).
		Return(&providers.AuthorizationDetailType{
			ID: "type-1", Type: testFlightType, Version: 1,
		}, nil).
		Once()

	w := httptest.NewRecorder()
	s.handler.HandleTypePostRequest(w, request(http.MethodPost, typeRequestBody, ""))
	s.Equal(http.StatusCreated, w.Code)

	var resp AuthorizationDetailTypeResponse
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	s.Equal("type-1", resp.ID)
}

func (s *HandlerTestSuite) TestPost_InvalidBody() {
	w := httptest.NewRecorder()
	s.handler.HandleTypePostRequest(w, request(http.MethodPost, `{`, ""))
	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *HandlerTestSuite) TestTypeFromRequest_ReuseDefaultsToFalse() {
	s.False(typeFromRequest(&AuthorizationDetailTypeRequest{}).Reuse, "an omitted reuse asks every time")
	reuse := true
	s.True(typeFromRequest(&AuthorizationDetailTypeRequest{Reuse: &reuse}).Reuse)
}

func (s *HandlerTestSuite) TestList() {
	s.service.EXPECT().ListAuthorizationDetailTypes(mock.Anything, 1, 1).
		Return(&AuthorizationDetailTypePage{TotalResults: 3, Types: []providers.AuthorizationDetailType{flightType()}},
			nil).
		Once()
	w := httptest.NewRecorder()
	req := request(http.MethodGet, "", "")
	req.URL.RawQuery = "limit=1&offset=1"
	s.handler.HandleTypeListRequest(w, req)
	s.Equal(http.StatusOK, w.Code)

	var resp AuthorizationDetailTypeListResponse
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	s.Equal(3, resp.TotalResults)
	s.Equal(2, resp.StartIndex)
	s.Equal(1, resp.Count)
	s.Equal(testFlightType, resp.Types[0].Type)
	rels := make([]string, 0, len(resp.Links))
	for _, link := range resp.Links {
		rels = append(rels, link.Rel)
	}
	s.Equal([]string{"first", "prev", "next", "last"}, rels)
	s.True(strings.HasPrefix(resp.Links[0].Href, "/authorization-detail-types?"), resp.Links[0].Href)

	w = httptest.NewRecorder()
	req = request(http.MethodGet, "", "")
	req.URL.RawQuery = "limit=ten"
	s.handler.HandleTypeListRequest(w, req)
	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *HandlerTestSuite) TestGetPutDelete() {
	schema := `{"type":"object","properties":{"zeta":{"type":"string"},"alpha":{"type":"string"}}}`
	s.service.EXPECT().GetAuthorizationDetailType(mock.Anything, "type-1").
		Return(&providers.AuthorizationDetailType{ID: "type-1", IsReadOnly: true,
			Schema: json.RawMessage(schema)}, nil).Once()
	w := httptest.NewRecorder()
	s.handler.HandleTypeGetRequest(w, request(http.MethodGet, "", "type-1"))
	s.Equal(http.StatusOK, w.Code)
	var got AuthorizationDetailTypeResponse
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &got))
	s.True(got.IsReadOnly, "a declarative type is reported read-only")
	s.Equal(schema, string(got.Schema), "the schema is returned in the order it declares its properties")

	s.service.EXPECT().UpdateAuthorizationDetailType(mock.Anything, "type-1", mock.Anything).
		Return(&providers.AuthorizationDetailType{ID: "type-1", Version: 2}, nil).Once()
	w = httptest.NewRecorder()
	s.handler.HandleTypePutRequest(w, request(http.MethodPut, typeRequestBody, "type-1"))
	s.Equal(http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	s.handler.HandleTypePutRequest(w, request(http.MethodPut, `nope`, "type-1"))
	s.Equal(http.StatusBadRequest, w.Code)

	// A fetched type can be sent back as it was returned.
	s.service.EXPECT().UpdateAuthorizationDetailType(mock.Anything, "type-1", mock.Anything).
		Return(&providers.AuthorizationDetailType{ID: "type-1", Version: 2}, nil).Once()
	fetched := `{"id":"type-1","version":1,"isReadOnly":false,` + typeRequestBody[1:]
	w = httptest.NewRecorder()
	s.handler.HandleTypePutRequest(w, request(http.MethodPut, fetched, "type-1"))
	s.Equal(http.StatusOK, w.Code)

	misspelled := `{"procesor":{"endpoint":"https://bank.example.com/rar"},` + typeRequestBody[1:]
	w = httptest.NewRecorder()
	s.handler.HandleTypePostRequest(w, request(http.MethodPost, misspelled, ""))
	s.Equal(http.StatusBadRequest, w.Code, "a misspelled key is refused rather than dropped")

	s.service.EXPECT().DeleteAuthorizationDetailType(mock.Anything, "type-1").Return(nil).Once()
	w = httptest.NewRecorder()
	s.handler.HandleTypeDeleteRequest(w, request(http.MethodDelete, "", "type-1"))
	s.Equal(http.StatusNoContent, w.Code)
}

func (s *HandlerTestSuite) TestErrorStatusMapping() {
	cases := map[*tidcommon.ServiceError]int{
		&ErrorTypeNotFound:             http.StatusNotFound,
		&ErrorTypeConflict:             http.StatusConflict,
		&ErrorTypeIDConflict:           http.StatusConflict,
		&ErrorTypeChanged:              http.StatusConflict,
		&ErrorImmutableType:            http.StatusBadRequest,
		&ErrorInvalidSchema:            http.StatusBadRequest,
		&tidcommon.InternalServerError: http.StatusInternalServerError,
	}
	for svcErr, status := range cases {
		s.service.EXPECT().DeleteAuthorizationDetailType(mock.Anything, "type-1").Return(svcErr).Once()
		w := httptest.NewRecorder()
		s.handler.HandleTypeDeleteRequest(w, request(http.MethodDelete, "", "type-1"))
		s.Equal(status, w.Code, svcErr.Code)
	}

	s.service.EXPECT().ListAuthorizationDetailTypes(mock.Anything, 30, 0).
		Return(nil, &tidcommon.InternalServerError).Once()
	w := httptest.NewRecorder()
	s.handler.HandleTypeListRequest(w, request(http.MethodGet, "", ""))
	s.Equal(http.StatusInternalServerError, w.Code)

	s.service.EXPECT().GetAuthorizationDetailType(mock.Anything, "x").Return(nil, &ErrorTypeNotFound).Once()
	w = httptest.NewRecorder()
	s.handler.HandleTypeGetRequest(w, request(http.MethodGet, "", "x"))
	s.Equal(http.StatusNotFound, w.Code)

	s.service.EXPECT().CreateAuthorizationDetailType(mock.Anything, mock.Anything).
		Return(nil, &ErrorTypeConflict).Once()
	w = httptest.NewRecorder()
	s.handler.HandleTypePostRequest(w, request(http.MethodPost, typeRequestBody, ""))
	s.Equal(http.StatusConflict, w.Code)

	s.service.EXPECT().UpdateAuthorizationDetailType(mock.Anything, "type-1", mock.Anything).
		Return(nil, &ErrorTypeValueImmutable).Once()
	w = httptest.NewRecorder()
	s.handler.HandleTypePutRequest(w, request(http.MethodPut, typeRequestBody, "type-1"))
	s.Equal(http.StatusBadRequest, w.Code)

	s.service.EXPECT().UpdateAuthorizationDetailType(mock.Anything, "missing", mock.Anything).
		Return(nil, &ErrorTypeNotFound).Once()
	w = httptest.NewRecorder()
	s.handler.HandleTypePutRequest(w, request(http.MethodPut, typeRequestBody, "missing"))
	s.Equal(http.StatusNotFound, w.Code)
}

func (s *HandlerTestSuite) TestRegisterRoutes() {
	mux := http.NewServeMux()
	registerRoutes(mux, s.handler)
	for _, pattern := range []string{
		"/authorization-detail-types",
		"/authorization-detail-types/id",
	} {
		r := httptest.NewRequest(http.MethodOptions, pattern, nil)
		_, matched := mux.Handler(r)
		s.NotEmpty(matched, pattern)
	}
}
