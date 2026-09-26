// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type HandlerTestSuite struct {
	suite.Suite
	service *ServiceInterfaceMock
	mux     *http.ServeMux
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) SetupTest() {
	s.service = NewServiceInterfaceMock(s.T())
	s.mux = http.NewServeMux()
	registerRoutes(s.mux, newHandler(s.service))
}

func (s *HandlerTestSuite) send(method, target, body string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, target, reader)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	s.mux.ServeHTTP(recorder, request)
	return recorder
}

func (s *HandlerTestSuite) TestCreateVariableReturnsCreated() {
	created := &Variable{Name: "API_URL", Value: "https://example.test"}
	s.service.On("CreateVariable", mock.Anything, VariableRequest{
		Name: "API_URL", Value: "https://example.test"}).Return(created, nil).Once()

	response := s.send(http.MethodPost, pathVariables, `{"name":"API_URL","value":"https://example.test"}`)

	s.Equal(http.StatusCreated, response.Code)
	s.Contains(response.Body.String(), "https://example.test")
}

func (s *HandlerTestSuite) TestAMalformedBodyIsRefused() {
	response := s.send(http.MethodPost, pathVariables, `{"name":`)

	s.Equal(http.StatusBadRequest, response.Code)
	s.Contains(response.Body.String(), ErrorInvalidRequestFormat.Code)
}

func (s *HandlerTestSuite) TestNotFoundBecomes404() {
	s.service.On("GetVariable", mock.Anything, "ABSENT").Return(nil, &ErrorNotFound).Once()

	response := s.send(http.MethodGet, pathVariables+"/ABSENT", "")

	s.Equal(http.StatusNotFound, response.Code)
}

func (s *HandlerTestSuite) TestAlreadyExistsBecomes409() {
	s.service.On("CreateVariable", mock.Anything, mock.Anything).Return(nil, &ErrorAlreadyExists).Once()

	response := s.send(http.MethodPost, pathVariables, `{"name":"API_URL","value":"x"}`)

	s.Equal(http.StatusConflict, response.Code)
}

func (s *HandlerTestSuite) TestAServerErrorBecomes500() {
	s.service.On("GetVariable", mock.Anything, "API_URL").Return(nil, &ErrorInternalServerError).Once()

	response := s.send(http.MethodGet, pathVariables+"/API_URL", "")

	s.Equal(http.StatusInternalServerError, response.Code)
}

func (s *HandlerTestSuite) TestDeleteReturnsNoContent() {
	s.service.On("DeleteVariable", mock.Anything, "API_URL").Return(nil).Once()

	response := s.send(http.MethodDelete, pathVariables+"/API_URL", "")

	s.Equal(http.StatusNoContent, response.Code)
	s.Empty(response.Body.String())
}

// The response to a secret request must not carry a value, whatever the service returns.
func (s *HandlerTestSuite) TestASecretResponseCarriesNoValue() {
	s.service.On("GetSecret", mock.Anything, "DB_PASSWORD").Return(
		&Secret{Name: "DB_PASSWORD", Exists: true}, nil).Once()

	response := s.send(http.MethodGet, pathSecrets+"/DB_PASSWORD", "")

	s.Equal(http.StatusOK, response.Code)
	var decoded map[string]interface{}
	s.Require().NoError(json.Unmarshal(response.Body.Bytes(), &decoded))
	_, hasValue := decoded["value"]
	s.False(hasValue, "a secret response must not carry a value")
	s.Equal(true, decoded["exists"])
}

func (s *HandlerTestSuite) TestCreateSecretReturnsCreatedWithoutEchoingTheValue() {
	s.service.On("CreateSecret", mock.Anything, SecretRequest{
		Name: "DB_PASSWORD", Value: "hunter2"}).Return(
		&Secret{Name: "DB_PASSWORD", Exists: true}, nil).Once()

	response := s.send(http.MethodPost, pathSecrets, `{"name":"DB_PASSWORD","value":"hunter2"}`)

	s.Equal(http.StatusCreated, response.Code)
	s.NotContains(response.Body.String(), "hunter2")
}

func (s *HandlerTestSuite) TestListPassesPagingThrough() {
	s.service.On("ListVariables", mock.Anything, listQuery{limit: 5, offset: 10}).Return(
		&VariableListResponse{TotalResults: 0}, nil).Once()

	response := s.send(http.MethodGet, pathVariables+"?limit=5&offset=10", "")

	s.Equal(http.StatusOK, response.Code)
}

func (s *HandlerTestSuite) TestBadPagingIsRefusedBeforeTheService() {
	for _, query := range []string{"?limit=0", "?limit=1000", "?limit=abc", "?offset=-1"} {
		response := s.send(http.MethodGet, pathVariables+query, "")
		s.Equal(http.StatusBadRequest, response.Code, "expected %q to be refused", query)
	}
}

func TestParseListQuery(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    listQuery
		wantErr *tidcommon.ServiceError
	}{
		{name: "defaults", raw: "", want: listQuery{limit: defaultLimit}},
		{name: "paging", raw: "limit=5&offset=10", want: listQuery{limit: 5, offset: 10}},
		{
			name: "names",
			raw:  "names=A,B",
			want: listQuery{limit: defaultLimit, names: []string{"A", "B"}},
		},
		{
			name: "filter eq",
			raw:  `filter=name eq "API_URL"`,
			want: listQuery{limit: defaultLimit, nameEquals: "API_URL"},
		},
		{
			name: "filter sw",
			raw:  `filter=name sw "DB_"`,
			want: listQuery{limit: defaultLimit, namePrefix: "DB_"},
		},
		{name: "unknown operator", raw: `filter=name like "x"`, wantErr: &ErrorInvalidFilter},
		{name: "unquoted operand", raw: "filter=name eq API_URL", wantErr: &ErrorInvalidFilter},
		{name: "mismatched quotes", raw: `filter=name eq 'API_URL"`, wantErr: &ErrorInvalidFilter},
		{name: "empty operand", raw: `filter=name eq ""`, wantErr: &ErrorInvalidFilter},
		{
			name: "single quotes",
			raw:  "filter=name eq 'API_URL'",
			want: listQuery{limit: defaultLimit, nameEquals: "API_URL"},
		},
		{name: "unknown attribute", raw: `filter=value eq "x"`, wantErr: &ErrorInvalidFilter},
		{name: "malformed filter", raw: "filter=name", wantErr: &ErrorInvalidFilter},
		{name: "bad name in names", raw: "names=A,not a name", wantErr: &ErrorInvalidName},
		{
			name:    "too many names",
			raw:     "names=" + strings.TrimSuffix(strings.Repeat("A,", maxNamesPerQuery+1), ","),
			wantErr: &ErrorTooManyNames,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values, err := url.ParseQuery(test.raw)
			if err != nil {
				t.Fatalf("bad test query: %v", err)
			}

			got, svcErr := parseListQuery(values)
			if test.wantErr != nil {
				if svcErr == nil || svcErr.Code != test.wantErr.Code {
					t.Fatalf("expected %v, got %v", test.wantErr.Code, svcErr)
				}
				return
			}
			if svcErr != nil {
				t.Fatalf("unexpected error: %v", svcErr)
			}
			if got.limit != test.want.limit || got.offset != test.want.offset ||
				got.nameEquals != test.want.nameEquals || got.namePrefix != test.want.namePrefix ||
				strings.Join(got.names, ",") != strings.Join(test.want.names, ",") {
				t.Fatalf("got %+v, want %+v", got, test.want)
			}
		})
	}
}

// A page link must carry the narrowing forward, or following "next" widens the result.
func TestPaginationLinksKeepTheFilter(t *testing.T) {
	links := buildLinks(pathVariables, listQuery{limit: 2, offset: 0, namePrefix: "DB_"}, 10)
	if len(links) != 1 || links[0].Rel != "next" {
		t.Fatalf("expected one next link, got %+v", links)
	}
	if !strings.Contains(links[0].Href, "filter=") || !strings.Contains(links[0].Href, "DB_") {
		t.Fatalf("next link dropped the filter: %s", links[0].Href)
	}
}

func TestPageBoundsClampsPastTheEnd(t *testing.T) {
	start, end := pageBounds(3, listQuery{limit: 10, offset: 50})
	if start != 3 || end != 3 {
		t.Fatalf("expected an empty page at the end, got [%d:%d]", start, end)
	}
}

func (s *HandlerTestSuite) TestReplacingAVariableReturnsOK() {
	s.service.On("UpdateVariable", mock.Anything, "API_URL",
		VariableUpdateRequest{Value: "https://new.test"}).
		Return(&Variable{Name: "API_URL", Value: "https://new.test"}, false, nil).Once()

	response := s.send(http.MethodPut, pathVariables+"/API_URL", `{"value":"https://new.test"}`)

	s.Equal(http.StatusOK, response.Code)
	s.Contains(response.Body.String(), "https://new.test")
}

// PUT is create-or-replace, and the two cases are told apart by the status.
func (s *HandlerTestSuite) TestCreatingAVariableByPutReturnsCreated() {
	s.service.On("UpdateVariable", mock.Anything, "NEW", VariableUpdateRequest{Value: "v"}).
		Return(&Variable{Name: "NEW", Value: "v"}, true, nil).Once()

	response := s.send(http.MethodPut, pathVariables+"/NEW", `{"value":"v"}`)

	s.Equal(http.StatusCreated, response.Code)
}

func (s *HandlerTestSuite) TestListSecretsReturnsNamesOnly() {
	s.service.On("ListSecrets", mock.Anything, listQuery{limit: defaultLimit}).Return(
		&SecretListResponse{
			TotalResults: 1,
			Count:        1,
			StartIndex:   1,
			Secrets:      []Secret{{Name: "DB_PASSWORD", Exists: true}},
		}, nil).Once()

	response := s.send(http.MethodGet, pathSecrets, "")

	s.Equal(http.StatusOK, response.Code)
	var decoded SecretListResponse
	s.Require().NoError(json.Unmarshal(response.Body.Bytes(), &decoded))
	s.Require().Len(decoded.Secrets, 1)
	s.Equal("DB_PASSWORD", decoded.Secrets[0].Name)
	s.NotContains(strings.ToLower(response.Body.String()), `"value"`)
}

func (s *HandlerTestSuite) TestRotateSecretReturnsOKWithoutEchoingTheValue() {
	s.service.On("UpdateSecret", mock.Anything, "DB_PASSWORD",
		SecretUpdateRequest{Value: "rotated"}).
		Return(&Secret{Name: "DB_PASSWORD", Exists: true}, false, nil).Once()

	response := s.send(http.MethodPut, pathSecrets+"/DB_PASSWORD", `{"value":"rotated"}`)

	s.Equal(http.StatusOK, response.Code)
	s.NotContains(response.Body.String(), "rotated")
}

func (s *HandlerTestSuite) TestStoringASecretByPutReturnsCreated() {
	s.service.On("UpdateSecret", mock.Anything, "NEW", SecretUpdateRequest{Value: "v"}).
		Return(&Secret{Name: "NEW", Exists: true}, true, nil).Once()

	response := s.send(http.MethodPut, pathSecrets+"/NEW", `{"value":"v"}`)

	s.Equal(http.StatusCreated, response.Code)
	s.NotContains(response.Body.String(), `"v"`)
}

func (s *HandlerTestSuite) TestDeleteSecretReturnsNoContent() {
	s.service.On("DeleteSecret", mock.Anything, "DB_PASSWORD").Return(nil).Once()

	response := s.send(http.MethodDelete, pathSecrets+"/DB_PASSWORD", "")

	s.Equal(http.StatusNoContent, response.Code)
}

func (s *HandlerTestSuite) TestPreflightIsAnsweredForEveryCollection() {
	for _, target := range []string{pathVariables, pathSecrets, pathVariables + "/A", pathSecrets + "/A"} {
		response := s.send(http.MethodOptions, target, "")
		s.Equal(http.StatusNoContent, response.Code, "preflight for %s", target)
	}
}

// A listing narrows only by what it understood. A parameter it did not understand is refused, so a
// caller never reads a successful response to a query that was silently widened.
func TestParseListQueryRefusesWhatItDoesNotUnderstand(t *testing.T) {
	for name, raw := range map[string]string{
		"misspelled parameter": "filtter=name+eq+%22A%22",
		"unknown parameter":    "sort=name",
		"empty filter":         "filter=",
		"empty names":          "names=",
		"empty limit":          "limit=",
		"repeated filter":      `filter=name+eq+%22A%22&filter=name+eq+%22B%22`,
		"repeated limit":       "limit=5&limit=10",
	} {
		values, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatalf("%s: bad test query: %v", name, err)
		}

		_, svcErr := parseListQuery(values)

		if svcErr == nil {
			t.Fatalf("%s: expected %q to be refused", name, raw)
		}
	}
}

// Every handler answers a store failure as an internal error rather than as a client mistake.
func (s *HandlerTestSuite) TestEveryHandlerReportsAStoreFailure() {
	cases := map[string]struct {
		expect func()
		method string
		target string
		body   string
	}{
		"list variables": {func() {
			s.service.On("ListVariables", mock.Anything, mock.Anything).
				Return(nil, &ErrorInternalServerError).Once()
		}, http.MethodGet, pathVariables, ""},
		"get variable": {func() {
			s.service.On("GetVariable", mock.Anything, "A").Return(nil, &ErrorInternalServerError).Once()
		}, http.MethodGet, pathVariables + "/A", ""},
		"put variable": {func() {
			s.service.On("UpdateVariable", mock.Anything, "A", mock.Anything).
				Return(nil, false, &ErrorInternalServerError).Once()
		}, http.MethodPut, pathVariables + "/A", `{"value":"v"}`},
		"delete variable": {func() {
			s.service.On("DeleteVariable", mock.Anything, "A").Return(&ErrorInternalServerError).Once()
		}, http.MethodDelete, pathVariables + "/A", ""},
		"list secrets": {func() {
			s.service.On("ListSecrets", mock.Anything, mock.Anything).
				Return(nil, &ErrorInternalServerError).Once()
		}, http.MethodGet, pathSecrets, ""},
		"create secret": {func() {
			s.service.On("CreateSecret", mock.Anything, mock.Anything).
				Return(nil, &ErrorInternalServerError).Once()
		}, http.MethodPost, pathSecrets, `{"name":"A","value":"v"}`},
		"get secret": {func() {
			s.service.On("GetSecret", mock.Anything, "A").Return(nil, &ErrorInternalServerError).Once()
		}, http.MethodGet, pathSecrets + "/A", ""},
		"put secret": {func() {
			s.service.On("UpdateSecret", mock.Anything, "A", mock.Anything).
				Return(nil, false, &ErrorInternalServerError).Once()
		}, http.MethodPut, pathSecrets + "/A", `{"value":"v"}`},
		"delete secret": {func() {
			s.service.On("DeleteSecret", mock.Anything, "A").Return(&ErrorInternalServerError).Once()
		}, http.MethodDelete, pathSecrets + "/A", ""},
	}

	for name, tc := range cases {
		s.Run(name, func() {
			s.SetupTest()
			tc.expect()

			response := s.send(tc.method, tc.target, tc.body)

			s.Equal(http.StatusInternalServerError, response.Code,
				"%s did not report the store failure as an internal error", name)
		})
	}
}

// A malformed body is refused by every route that takes one, before the service sees it.
func (s *HandlerTestSuite) TestEveryWriteRefusesAMalformedBody() {
	for name, tc := range map[string]struct{ method, target string }{
		"create variable": {http.MethodPost, pathVariables},
		"put variable":    {http.MethodPut, pathVariables + "/A"},
		"create secret":   {http.MethodPost, pathSecrets},
		"put secret":      {http.MethodPut, pathSecrets + "/A"},
	} {
		s.Run(name, func() {
			s.SetupTest()

			response := s.send(tc.method, tc.target, `{"value":`)

			s.Equal(http.StatusBadRequest, response.Code)
			s.Contains(response.Body.String(), ErrorInvalidRequestFormat.Code)
		})
	}
}

// A secret stored by PUT is a creation, and says so.
func (s *HandlerTestSuite) TestPuttingANewSecretAnswersCreated() {
	s.service.On("UpdateSecret", mock.Anything, "NEW", SecretUpdateRequest{Value: "v"}).
		Return(&Secret{Name: "NEW", Exists: true}, true, nil).Once()

	response := s.send(http.MethodPut, pathSecrets+"/NEW", `{"value":"v"}`)

	s.Equal(http.StatusCreated, response.Code)
}
