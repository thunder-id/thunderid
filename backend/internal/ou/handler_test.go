/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package ou

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

type OrganizationUnitHandlerTestSuite struct {
	suite.Suite
}

const (
	defaultOURequestID = "ou-1"
	defaultOUPath      = "root"
	defaultOUHandle    = "finance"
	testOUNameFinance  = "Finance"
)

func TestOUHandler_OrganizationUnitHandlerTestSuite_Run(t *testing.T) {
	suite.Run(t, new(OrganizationUnitHandlerTestSuite))
}

type flakyResponseWriter struct {
	*httptest.ResponseRecorder
	failNext bool
}

func newFlakyResponseWriter() *flakyResponseWriter {
	return &flakyResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		failNext:         true,
	}
}

func (w *flakyResponseWriter) Write(b []byte) (int, error) {
	if w.failNext {
		w.failNext = false
		return 0, errors.New("write failure")
	}
	return w.ResponseRecorder.Write(b)
}

func (suite *OrganizationUnitHandlerTestSuite) SetupTest() {
	suite.ensureRuntime()
}

func (suite *OrganizationUnitHandlerTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *OrganizationUnitHandlerTestSuite) ensureRuntime() {
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("", &config.Config{})
	suite.Require().NoError(err)
}

type ouHandlerTestCase struct {
	name           string
	method         string
	url            string
	body           string
	pathParamKey   string
	pathParamValue string
	useFlaky       bool
	setJSONHeader  bool
	setup          func(*OrganizationUnitServiceInterfaceMock)
	assert         func(*httptest.ResponseRecorder)
	assertService  func(*OrganizationUnitServiceInterfaceMock)
}

func (suite *OrganizationUnitHandlerTestSuite) runHandlerTestCases(
	testCases []ouHandlerTestCase,
	invoke func(*organizationUnitHandler, http.ResponseWriter, *http.Request),
) {
	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			serviceMock := NewOrganizationUnitServiceInterfaceMock(suite.T())
			handler := newOrganizationUnitHandler(serviceMock)

			method := tc.method
			if method == "" {
				method = http.MethodGet
			}

			var bodyReader io.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			}

			req := httptest.NewRequest(method, tc.url, bodyReader)
			if tc.pathParamKey != "" && tc.pathParamValue != "" {
				req.SetPathValue(tc.pathParamKey, tc.pathParamValue)
			}
			if tc.setJSONHeader {
				req.Header.Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeJSON)
			}

			var writer http.ResponseWriter
			var recorder *httptest.ResponseRecorder
			if tc.useFlaky {
				flaky := newFlakyResponseWriter()
				writer = flaky
				recorder = flaky.ResponseRecorder
			} else {
				recorder = httptest.NewRecorder()
				writer = recorder
			}

			if tc.setup != nil {
				tc.setup(serviceMock)
			}

			invoke(handler, writer, req)

			if tc.assert != nil {
				tc.assert(recorder)
			}

			if tc.assertService != nil {
				tc.assertService(serviceMock)
			} else {
				serviceMock.AssertExpectations(suite.T())
			}
		})
	}
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_RegisterRoutes() {
	tests := []struct {
		name       string
		method     string
		path       string
		setup      func(*OrganizationUnitServiceInterfaceMock)
		wantStatus int
	}{
		{
			name:       "options",
			method:     http.MethodOptions,
			path:       "/organization-units",
			wantStatus: http.StatusNoContent,
		},
		{
			name:   "get dispatch",
			method: http.MethodGet,
			path:   "/organization-units/ou-123",
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnit", mock.Anything, "ou-123").
					Return(OrganizationUnit{ID: "ou-123"}, nil).
					Once()
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "children dispatch",
			method: http.MethodGet,
			path:   "/organization-units/ou-123/ous",
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On(
						"GetOrganizationUnitChildren", mock.Anything, "ou-123",
						serverconst.DefaultPageSize, 0, mock.Anything,
					).
					Return(&OrganizationUnitListResponse{}, nil).
					Once()
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "tree dispatch",
			method: http.MethodGet,
			path:   "/organization-units/tree/root",
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitByPath", mock.Anything, "root").
					Return(OrganizationUnit{ID: "ou-root"}, nil).
					Once()
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "unknown subresource",
			method:     http.MethodGet,
			path:       "/organization-units/ou-123/unknown",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "too many segments",
			method:     http.MethodGet,
			path:       "/organization-units/ou-123/foo/bar",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "id options route",
			method:     http.MethodOptions,
			path:       "/organization-units/ou-123",
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "tree options route",
			method:     http.MethodOptions,
			path:       "/organization-units/tree/root",
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			serviceMock := NewOrganizationUnitServiceInterfaceMock(suite.T())
			handler := newOrganizationUnitHandler(serviceMock)
			mux := http.NewServeMux()
			registerRoutes(mux, handler)

			if tc.setup != nil {
				tc.setup(serviceMock)
			}

			req := httptest.NewRequest(tc.method, tc.path, nil)
			resp := httptest.NewRecorder()

			mux.ServeHTTP(resp, req)

			suite.Equal(tc.wantStatus, resp.Code)
			serviceMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUListRequest() {
	testCases := []struct {
		name          string
		url           string
		useFlaky      bool
		setup         func(*OrganizationUnitServiceInterfaceMock)
		assert        func(*httptest.ResponseRecorder)
		assertService func(*OrganizationUnitServiceInterfaceMock)
	}{
		{
			name: "success",
			url:  "/organization-units?limit=3&offset=2",
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitList", mock.Anything, 3, 2, mock.Anything).
					Return(&OrganizationUnitListResponse{
						TotalResults: 4,
						Count:        2,
						OrganizationUnits: []OrganizationUnitBasic{
							{ID: "ou-1", Handle: "root"},
							{ID: "ou-2", Handle: "child"},
						},
					}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				suite.Equal(serverconst.ContentTypeJSON, recorder.Header().Get(serverconst.ContentTypeHeaderName))
				var resp OrganizationUnitListResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(4, resp.TotalResults)
				suite.Len(resp.OrganizationUnits, 2)
			},
		},
		{
			name: "default limit applied",
			url:  "/organization-units?offset=1",
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitList", mock.Anything, serverconst.DefaultPageSize, 1, mock.Anything).
					Return(&OrganizationUnitListResponse{}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
			},
		},
		{
			name: "invalid limit",
			url:  "/organization-units?limit=abc",
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var body apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Equal(ErrorInvalidLimit.Code, body.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetOrganizationUnitList", mock.Anything)
			},
		},
		{
			name: "invalid offset",
			url:  "/organization-units?offset=abc",
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var body apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Equal(ErrorInvalidOffset.Code, body.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetOrganizationUnitList", mock.Anything)
			},
		},
		{
			name: "invalid filter",
			url:  "/organization-units?filter=invalid",
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var body apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Equal(ErrorInvalidFilter.Code, body.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetOrganizationUnitList", mock.Anything)
			},
		},
		{
			name: "service error",
			url:  "/organization-units",
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitList", mock.Anything, serverconst.DefaultPageSize, 0, mock.Anything).
					Return((*OrganizationUnitListResponse)(nil), &serviceerror.InternalServerError).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusInternalServerError, recorder.Code)
				var body apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Equal(serviceerror.InternalServerError.Code, body.Code)
			},
		},
		{
			name:     "response write error",
			url:      "/organization-units",
			useFlaky: true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitList", mock.Anything, serverconst.DefaultPageSize, 0, mock.Anything).
					Return(&OrganizationUnitListResponse{}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				suite.Equal("", recorder.Body.String()) // Write fails, body remains empty
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			serviceMock := NewOrganizationUnitServiceInterfaceMock(suite.T())
			handler := newOrganizationUnitHandler(serviceMock)

			req := httptest.NewRequest(http.MethodGet, tc.url, nil)

			var writer http.ResponseWriter
			var recorder *httptest.ResponseRecorder
			if tc.useFlaky {
				flaky := newFlakyResponseWriter()
				writer = flaky
				recorder = flaky.ResponseRecorder
			} else {
				recorder = httptest.NewRecorder()
				writer = recorder
			}

			if tc.setup != nil {
				tc.setup(serviceMock)
			}

			handler.HandleOUListRequest(writer, req)

			if tc.assert != nil {
				tc.assert(recorder)
			}

			if tc.assertService != nil {
				tc.assertService(serviceMock)
			} else {
				serviceMock.AssertExpectations(suite.T())
			}
		})
	}
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUPostRequest() {
	testCases := []struct {
		name          string
		body          string
		useFlaky      bool
		setup         func(*OrganizationUnitServiceInterfaceMock)
		assert        func(*httptest.ResponseRecorder)
		assertService func(*OrganizationUnitServiceInterfaceMock)
	}{
		{
			name: "invalid json",
			body: "{invalid",
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidRequestFormat.Code, resp.Code)
				suite.Equal(ErrorInvalidRequestFormat.ErrorDescription.DefaultValue, resp.Description.DefaultValue)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "CreateOrganizationUnit", mock.Anything)
			},
		},
		{
			name:     "invalid json response write error",
			body:     "{invalid",
			useFlaky: true,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				suite.Contains(recorder.Body.String(), serviceerror.ErrorEncodingError.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "CreateOrganizationUnit", mock.Anything)
			},
		},
		{
			name: "sanitizes payload",
			body: `{
				"handle": "  finance ",
				"name": " Finance <script> ",
				"description": " desc "
			}`,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("CreateOrganizationUnit", mock.Anything,
						mock.MatchedBy(func(req OrganizationUnitRequestWithID) bool {
							return req.Handle == defaultOUHandle &&
								req.Name == "Finance &lt;script&gt;" &&
								req.Description == "desc"
						})).
					Return(OrganizationUnit{ID: "ou-1", Name: "Finance &lt;script&gt;"}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusCreated, recorder.Code)
				var resp OrganizationUnit
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal("ou-1", resp.ID)
			},
		},
		//nolint:dupl // Similar structure but tests different fields (design vs URI fields)
		{
			name: "passes design fields",
			body: `{
				"handle": "finance",
				"name": "` + testOUNameFinance + `",
				"themeId": "theme-123",
				"layoutId": "layout-456",
				"logoUrl": "https://example.com/logo.png"
			}`,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("CreateOrganizationUnit", mock.Anything,
						mock.MatchedBy(func(req OrganizationUnitRequestWithID) bool {
							return req.Handle == defaultOUHandle &&
								req.Name == testOUNameFinance &&
								req.ThemeID == "theme-123" &&
								req.LayoutID == "layout-456" &&
								req.LogoURL == "https://example.com/logo.png"
						})).
					Return(OrganizationUnit{
						ID:       "ou-1",
						Handle:   "finance",
						Name:     testOUNameFinance,
						ThemeID:  "theme-123",
						LayoutID: "layout-456",
						LogoURL:  "https://example.com/logo.png",
					}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusCreated, recorder.Code)
				var resp OrganizationUnit
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal("ou-1", resp.ID)
				suite.Equal("theme-123", resp.ThemeID)
				suite.Equal("layout-456", resp.LayoutID)
				suite.Equal("https://example.com/logo.png", resp.LogoURL)
			},
		},
		//nolint:dupl // Similar structure but tests different fields (design vs URI fields)
		{
			name: "passes URI fields",
			body: `{
				"handle": "finance",
				"name": "` + testOUNameFinance + `",
				"tosUri": "https://example.com/tos",
				"policyUri": "https://example.com/privacy",
				"cookiePolicyUri": "https://example.com/cookie-policy"
			}`,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("CreateOrganizationUnit", mock.Anything,
						mock.MatchedBy(func(req OrganizationUnitRequestWithID) bool {
							return req.Handle == defaultOUHandle &&
								req.Name == testOUNameFinance &&
								req.TosURI == "https://example.com/tos" &&
								req.PolicyURI == "https://example.com/privacy" &&
								req.CookiePolicyURI == "https://example.com/cookie-policy"
						})).
					Return(OrganizationUnit{
						ID:              "ou-1",
						Handle:          "finance",
						Name:            testOUNameFinance,
						TosURI:          "https://example.com/tos",
						PolicyURI:       "https://example.com/privacy",
						CookiePolicyURI: "https://example.com/cookie-policy",
					}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusCreated, recorder.Code)
				var resp OrganizationUnit
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal("ou-1", resp.ID)
				suite.Equal("https://example.com/tos", resp.TosURI)
				suite.Equal("https://example.com/privacy", resp.PolicyURI)
				suite.Equal("https://example.com/cookie-policy", resp.CookiePolicyURI)
			},
		},
		{
			name: "service conflict",
			body: `{"handle":"finance","name":"` + testOUNameFinance + `"}`,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("CreateOrganizationUnit", mock.Anything,
						mock.AnythingOfType("ou.OrganizationUnitRequestWithID")).
					Return(OrganizationUnit{}, &ErrorOrganizationUnitNameConflict).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusConflict, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorOrganizationUnitNameConflict.Code, resp.Code)
			},
		},
		{
			name: "service error",
			body: `{"handle":"finance","name":"` + testOUNameFinance + `"}`,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("CreateOrganizationUnit", mock.Anything,
						mock.AnythingOfType("ou.OrganizationUnitRequestWithID")).
					Return(OrganizationUnit{}, &serviceerror.InternalServerError).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusInternalServerError, recorder.Code)
				var body apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Equal(serviceerror.InternalServerError.Code, body.Code)
			},
		},
		{
			name:     "service error response write failure",
			body:     `{"handle":"finance","name":"` + testOUNameFinance + `"}`,
			useFlaky: true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("CreateOrganizationUnit", mock.Anything,
						mock.AnythingOfType("ou.OrganizationUnitRequestWithID")).
					Return(OrganizationUnit{}, &serviceerror.InternalServerError).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusInternalServerError, recorder.Code)
				suite.Contains(recorder.Body.String(), serviceerror.ErrorEncodingError.Code)
			},
		},
		{
			name:     "response write error",
			body:     `{"handle":"finance","name":"` + testOUNameFinance + `"}`,
			useFlaky: true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("CreateOrganizationUnit", mock.Anything,
						mock.AnythingOfType("ou.OrganizationUnitRequestWithID")).
					Return(OrganizationUnit{ID: "ou-1"}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusCreated, recorder.Code)
				suite.Equal("", recorder.Body.String()) // Write fails, body remains empty
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			serviceMock := NewOrganizationUnitServiceInterfaceMock(suite.T())
			handler := newOrganizationUnitHandler(serviceMock)

			req := httptest.NewRequest(http.MethodPost, "/organization-units", strings.NewReader(tc.body))
			req.Header.Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeJSON)

			var writer http.ResponseWriter
			var recorder *httptest.ResponseRecorder
			if tc.useFlaky {
				flaky := newFlakyResponseWriter()
				writer = flaky
				recorder = flaky.ResponseRecorder
			} else {
				recorder = httptest.NewRecorder()
				writer = recorder
			}

			if tc.setup != nil {
				tc.setup(serviceMock)
			}

			handler.HandleOUPostRequest(writer, req)

			if tc.assert != nil {
				tc.assert(recorder)
			}

			if tc.assertService != nil {
				tc.assertService(serviceMock)
			} else {
				serviceMock.AssertExpectations(suite.T())
			}
		})
	}
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUGetRequest() {
	testCases := []ouHandlerTestCase{
		{
			name: "missing id",
			url:  "/organization-units/" + defaultOURequestID,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorMissingOUID.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetOrganizationUnit", mock.Anything)
			},
		},
		{
			name:     "missing id response write error",
			url:      "/organization-units/" + defaultOURequestID,
			useFlaky: true,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				suite.Contains(recorder.Body.String(), serviceerror.ErrorEncodingError.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetOrganizationUnit", mock.Anything)
			},
		},
		{
			name:           "not found",
			url:            "/organization-units/" + defaultOURequestID,
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnit", mock.Anything, defaultOURequestID).
					Return(OrganizationUnit{}, &ErrorOrganizationUnitNotFound).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusNotFound, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorOrganizationUnitNotFound.Code, resp.Code)
			},
		},
		{
			name:           "response write error",
			url:            "/organization-units/" + defaultOURequestID,
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			useFlaky:       true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnit", mock.Anything, defaultOURequestID).
					Return(OrganizationUnit{ID: defaultOURequestID}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				suite.Equal("", recorder.Body.String()) // Write fails, body remains empty
			},
		},
		{
			name:           "success",
			url:            "/organization-units/" + defaultOURequestID,
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnit", mock.Anything, defaultOURequestID).
					Return(OrganizationUnit{ID: defaultOURequestID, Name: testOUNameFinance}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp OrganizationUnit
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(testOUNameFinance, resp.Name)
			},
		},
		{
			name:           "returns design fields",
			url:            "/organization-units/" + defaultOURequestID,
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnit", mock.Anything, defaultOURequestID).
					Return(OrganizationUnit{
						ID:       defaultOURequestID,
						Name:     "Finance",
						ThemeID:  "theme-123",
						LayoutID: "layout-456",
						LogoURL:  "https://example.com/logo.png",
					}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp OrganizationUnit
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal("Finance", resp.Name)
				suite.Equal("theme-123", resp.ThemeID)
				suite.Equal("layout-456", resp.LayoutID)
				suite.Equal("https://example.com/logo.png", resp.LogoURL)
			},
		},
	}

	suite.runHandlerTestCases(testCases,
		func(handler *organizationUnitHandler, writer http.ResponseWriter, req *http.Request) {
			handler.HandleOUGetRequest(writer, req)
		})
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUPutRequest() {
	bodySanitize := `{"handle":"  finance ","name":" Finance <script> ","description":" desc "}`
	bodyValid := `{"handle":"finance","name":"Finance"}`
	testCases := []ouHandlerTestCase{
		{
			name:          "missing id",
			method:        http.MethodPut,
			url:           "/organization-units/" + defaultOURequestID,
			body:          bodyValid,
			setJSONHeader: true,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorMissingOUID.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "UpdateOrganizationUnit", mock.Anything)
			},
		},
		{
			name:           "invalid json",
			method:         http.MethodPut,
			url:            "/organization-units/" + defaultOURequestID,
			body:           "{invalid",
			setJSONHeader:  true,
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidRequestFormat.Code, resp.Code)
			},
		},
		{
			name:           "invalid json response write error",
			method:         http.MethodPut,
			url:            "/organization-units/" + defaultOURequestID,
			body:           "{invalid",
			setJSONHeader:  true,
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			useFlaky:       true,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				suite.Contains(recorder.Body.String(), serviceerror.ErrorEncodingError.Code)
			},
		},
		{
			name:           "sanitizes payload",
			method:         http.MethodPut,
			url:            "/organization-units/" + defaultOURequestID,
			body:           bodySanitize,
			setJSONHeader:  true,
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("UpdateOrganizationUnit", mock.Anything,
						defaultOURequestID,
						mock.MatchedBy(func(req OrganizationUnitRequestWithID) bool {
							return req.Handle == defaultOUHandle &&
								req.Name == "Finance &lt;script&gt;" &&
								req.Description == "desc"
						}),
					).
					Return(OrganizationUnit{ID: defaultOURequestID, Name: "Finance &lt;script&gt;"}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp OrganizationUnit
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal("Finance &lt;script&gt;", resp.Name)
			},
		},
		{
			name:   "passes design fields on update",
			method: http.MethodPut,
			url:    "/organization-units/" + defaultOURequestID,
			body: `{
				"handle": "finance",
				"name": "` + testOUNameFinance + `",
				"themeId": "theme-new",
				"layoutId": "layout-new",
				"logoUrl": "https://example.com/new-logo.png"
			}`,
			setJSONHeader:  true,
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("UpdateOrganizationUnit", mock.Anything,
						defaultOURequestID,
						mock.MatchedBy(func(req OrganizationUnitRequestWithID) bool {
							return req.Handle == defaultOUHandle &&
								req.Name == testOUNameFinance &&
								req.ThemeID == "theme-new" &&
								req.LayoutID == "layout-new" &&
								req.LogoURL == "https://example.com/new-logo.png"
						}),
					).
					Return(OrganizationUnit{
						ID:       defaultOURequestID,
						Handle:   "finance",
						Name:     testOUNameFinance,
						ThemeID:  "theme-new",
						LayoutID: "layout-new",
						LogoURL:  "https://example.com/new-logo.png",
					}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp OrganizationUnit
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal("theme-new", resp.ThemeID)
				suite.Equal("layout-new", resp.LayoutID)
				suite.Equal("https://example.com/new-logo.png", resp.LogoURL)
			},
		},
		{
			name:           "service conflict",
			method:         http.MethodPut,
			url:            "/organization-units/" + defaultOURequestID,
			body:           bodyValid,
			setJSONHeader:  true,
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("UpdateOrganizationUnit", mock.Anything, defaultOURequestID,
						mock.AnythingOfType("ou.OrganizationUnitRequestWithID")).
					Return(OrganizationUnit{}, &ErrorOrganizationUnitHandleConflict).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusConflict, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorOrganizationUnitHandleConflict.Code, resp.Code)
			},
		},
		{
			name:           "response write error",
			method:         http.MethodPut,
			url:            "/organization-units/" + defaultOURequestID,
			body:           bodyValid,
			setJSONHeader:  true,
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			useFlaky:       true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("UpdateOrganizationUnit", mock.Anything, defaultOURequestID,
						mock.AnythingOfType("ou.OrganizationUnitRequestWithID")).
					Return(OrganizationUnit{ID: defaultOURequestID}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				suite.Equal("", recorder.Body.String()) // Write fails, body remains empty
			},
		},
	}

	suite.runHandlerTestCases(testCases,
		func(handler *organizationUnitHandler, writer http.ResponseWriter, req *http.Request) {
			handler.HandleOUPutRequest(writer, req)
		})
}
func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUDeleteRequest() {
	testCases := []struct {
		name          string
		setID         bool
		setup         func(*OrganizationUnitServiceInterfaceMock)
		assert        func(*httptest.ResponseRecorder)
		assertService func(*OrganizationUnitServiceInterfaceMock)
	}{
		{
			name: "missing id",
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorMissingOUID.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "DeleteOrganizationUnit", mock.Anything)
			},
		},
		{
			name:  "not found",
			setID: true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("DeleteOrganizationUnit", mock.Anything, "ou-1").
					Return(&ErrorOrganizationUnitNotFound).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusNotFound, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorOrganizationUnitNotFound.Code, resp.Code)
			},
		},
		{
			name:  "service error",
			setID: true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("DeleteOrganizationUnit", mock.Anything, "ou-1").
					Return(&serviceerror.InternalServerError).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusInternalServerError, recorder.Code)
				var body apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Equal(serviceerror.InternalServerError.Code, body.Code)
			},
		},
		{
			name:  "success",
			setID: true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("DeleteOrganizationUnit", mock.Anything, "ou-1").
					Return((*serviceerror.ServiceError)(nil)).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusNoContent, recorder.Code)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			serviceMock := NewOrganizationUnitServiceInterfaceMock(suite.T())
			handler := newOrganizationUnitHandler(serviceMock)

			req := httptest.NewRequest(http.MethodDelete, "/organization-units/ou-1", nil)
			if tc.setID {
				req.SetPathValue("id", "ou-1")
			}

			recorder := httptest.NewRecorder()

			if tc.setup != nil {
				tc.setup(serviceMock)
			}

			handler.HandleOUDeleteRequest(recorder, req)

			if tc.assert != nil {
				tc.assert(recorder)
			}

			if tc.assertService != nil {
				tc.assertService(serviceMock)
			} else {
				serviceMock.AssertExpectations(suite.T())
			}
		})
	}
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUChildrenListRequest() {
	testCases := []ouHandlerTestCase{
		{
			name: "missing id",
			url:  "/organization-units/" + defaultOURequestID + "/ous",
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorMissingOUID.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(
					suite.T(),
					"GetOrganizationUnitChildren",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				)
			},
		},
		{
			name:           "invalid limit",
			url:            "/organization-units/" + defaultOURequestID + "/ous?limit=abc",
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidLimit.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(
					suite.T(),
					"GetOrganizationUnitChildren",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				)
			},
		},
		{
			name:           "invalid filter",
			url:            "/organization-units/" + defaultOURequestID + "/ous?filter=invalid",
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidFilter.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(
					suite.T(),
					"GetOrganizationUnitChildren",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				)
			},
		},
		{
			name:           "service error",
			url:            "/organization-units/" + defaultOURequestID + "/ous",
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitChildren", mock.Anything,
						defaultOURequestID, serverconst.DefaultPageSize, 0, mock.Anything).
					Return((*OrganizationUnitListResponse)(nil), &serviceerror.InternalServerError).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusInternalServerError, recorder.Code)
				var body apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Equal(serviceerror.InternalServerError.Code, body.Code)
			},
		},
		{
			name:           "response write error",
			url:            "/organization-units/" + defaultOURequestID + "/ous",
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			useFlaky:       true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitChildren", mock.Anything,
						defaultOURequestID, serverconst.DefaultPageSize, 0, mock.Anything).
					Return(&OrganizationUnitListResponse{}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				suite.Equal("", recorder.Body.String()) // Write fails, body remains empty
			},
		},
		{
			name:           "success",
			url:            "/organization-units/" + defaultOURequestID + "/ous?limit=2&offset=1",
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitChildren", mock.Anything, defaultOURequestID, 2, 1, mock.Anything).
					Return(&OrganizationUnitListResponse{TotalResults: 1}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp OrganizationUnitListResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(1, resp.TotalResults)
			},
		},
	}

	suite.runHandlerTestCases(testCases,
		func(handler *organizationUnitHandler, writer http.ResponseWriter, req *http.Request) {
			handler.HandleOUChildrenListRequest(writer, req)
		})

	testCasesByPath := []ouHandlerTestCase{
		{
			name:           "path invalid limit",
			url:            "/organization-units/tree/" + defaultOUPath + "/ous?limit=abc",
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidLimit.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(
					suite.T(),
					"GetOrganizationUnitChildrenByPath",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				)
			},
		},
		{
			name:           "path invalid filter",
			url:            "/organization-units/tree/" + defaultOUPath + "/ous?filter=invalid",
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidFilter.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(
					suite.T(),
					"GetOrganizationUnitChildrenByPath",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				)
			},
		},
		{
			name:           "path success",
			url:            "/organization-units/tree/" + defaultOUPath + "/ous",
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitChildrenByPath", mock.Anything,
						defaultOUPath, serverconst.DefaultPageSize, 0, mock.Anything).
					Return(&OrganizationUnitListResponse{TotalResults: 2, Count: 2}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp OrganizationUnitListResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(2, resp.TotalResults)
			},
		},
	}

	suite.runHandlerTestCases(testCasesByPath,
		func(handler *organizationUnitHandler, writer http.ResponseWriter, req *http.Request) {
			handler.HandleOUChildrenListByPathRequest(writer, req)
		})
}
func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUGetByPathRequest() {
	testCases := []ouHandlerTestCase{
		{
			name: "missing path",
			url:  "/organization-units/tree/" + defaultOUPath,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidHandlePath.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetOrganizationUnitByPath", mock.Anything)
			},
		},
		{
			name:     "missing path response write error",
			url:      "/organization-units/tree/" + defaultOUPath,
			useFlaky: true,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				suite.Contains(recorder.Body.String(), serviceerror.ErrorEncodingError.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetOrganizationUnitByPath", mock.Anything)
			},
		},
		{
			name:           "not found",
			url:            "/organization-units/tree/" + defaultOUPath,
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitByPath", mock.Anything, defaultOUPath).
					Return(OrganizationUnit{}, &ErrorOrganizationUnitNotFound).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusNotFound, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorOrganizationUnitNotFound.Code, resp.Code)
			},
		},
		{
			name:           "response write error",
			url:            "/organization-units/tree/" + defaultOUPath,
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			useFlaky:       true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitByPath", mock.Anything, defaultOUPath).
					Return(OrganizationUnit{ID: defaultOURequestID}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				suite.Equal("", recorder.Body.String()) // Write fails, body remains empty
			},
		},
		{
			name:           "success",
			url:            "/organization-units/tree/" + defaultOUPath,
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitByPath", mock.Anything, defaultOUPath).
					Return(OrganizationUnit{ID: defaultOURequestID}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp OrganizationUnit
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(defaultOURequestID, resp.ID)
			},
		},
	}

	suite.runHandlerTestCases(testCases,
		func(handler *organizationUnitHandler, writer http.ResponseWriter, req *http.Request) {
			handler.HandleOUGetByPathRequest(writer, req)
		})
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUPutByPathRequest() {
	testCases := []ouHandlerTestCase{
		{
			name:          "missing path",
			method:        http.MethodPut,
			url:           "/organization-units/tree/" + defaultOUPath,
			body:          `{"handle":"finance","name":"Finance"}`,
			setJSONHeader: true,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidHandlePath.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "UpdateOrganizationUnitByPath", mock.Anything)
			},
		},
		{
			name:           "invalid json",
			method:         http.MethodPut,
			url:            "/organization-units/tree/" + defaultOUPath,
			body:           "{invalid",
			setJSONHeader:  true,
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidRequestFormat.Code, resp.Code)
			},
		},
		{
			name:           "service error",
			method:         http.MethodPut,
			url:            "/organization-units/tree/" + defaultOUPath,
			body:           `{"handle":"finance","name":"Finance"}`,
			setJSONHeader:  true,
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("UpdateOrganizationUnitByPath", mock.Anything, defaultOUPath,
						mock.AnythingOfType("ou.OrganizationUnitRequestWithID")).
					Return(OrganizationUnit{}, &serviceerror.InternalServerError).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusInternalServerError, recorder.Code)
				var body apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Equal(serviceerror.InternalServerError.Code, body.Code)
			},
		},
		{
			name:           "response write error",
			method:         http.MethodPut,
			url:            "/organization-units/tree/" + defaultOUPath,
			body:           `{"handle":"finance","name":"Finance"}`,
			setJSONHeader:  true,
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			useFlaky:       true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("UpdateOrganizationUnitByPath", mock.Anything, defaultOUPath,
						mock.AnythingOfType("ou.OrganizationUnitRequestWithID")).
					Return(OrganizationUnit{ID: defaultOURequestID}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				suite.Equal("", recorder.Body.String()) // Write fails, body remains empty
			},
		},
		{
			name:           "success",
			method:         http.MethodPut,
			url:            "/organization-units/tree/" + defaultOUPath,
			body:           `{"handle":"finance","name":"Finance"}`,
			setJSONHeader:  true,
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("UpdateOrganizationUnitByPath", mock.Anything, defaultOUPath,
						mock.AnythingOfType("ou.OrganizationUnitRequestWithID")).
					Return(OrganizationUnit{ID: defaultOURequestID}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp OrganizationUnit
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(defaultOURequestID, resp.ID)
			},
		},
	}

	suite.runHandlerTestCases(testCases,
		func(handler *organizationUnitHandler, writer http.ResponseWriter, req *http.Request) {
			handler.HandleOUPutByPathRequest(writer, req)
		})
}
func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUDeleteByPathRequest() {
	testCases := []struct {
		name          string
		setPath       bool
		setup         func(*OrganizationUnitServiceInterfaceMock)
		assert        func(*httptest.ResponseRecorder)
		assertService func(*OrganizationUnitServiceInterfaceMock)
	}{
		{
			name: "missing path",
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidHandlePath.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "DeleteOrganizationUnitByPath", mock.Anything)
			},
		},
		{
			name:    "service error",
			setPath: true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("DeleteOrganizationUnitByPath", mock.Anything, "root").
					Return(&serviceerror.InternalServerError).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusInternalServerError, recorder.Code)
				var body apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Equal(serviceerror.InternalServerError.Code, body.Code)
			},
		},
		{
			name:    "success",
			setPath: true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("DeleteOrganizationUnitByPath", mock.Anything, "root").
					Return((*serviceerror.ServiceError)(nil)).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusNoContent, recorder.Code)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			serviceMock := NewOrganizationUnitServiceInterfaceMock(suite.T())
			handler := newOrganizationUnitHandler(serviceMock)

			req := httptest.NewRequest(http.MethodDelete, "/organization-units/tree/root", nil)
			if tc.setPath {
				req.SetPathValue("path", "root")
			}

			recorder := httptest.NewRecorder()

			if tc.setup != nil {
				tc.setup(serviceMock)
			}

			handler.HandleOUDeleteByPathRequest(recorder, req)

			if tc.assert != nil {
				tc.assert(recorder)
			}

			if tc.assertService != nil {
				tc.assertService(serviceMock)
			} else {
				serviceMock.AssertExpectations(suite.T())
			}
		})
	}
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUUsersListByPathRequest() {
	testCases := []ouHandlerTestCase{
		{
			name: "missing path",
			url:  "/organization-units/tree/" + defaultOUPath + "/users",
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidHandlePath.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(
					suite.T(),
					"GetOrganizationUnitUsersByPath",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				)
			},
		},
		{
			name:           "invalid limit",
			url:            "/organization-units/tree/" + defaultOUPath + "/users?limit=abc",
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidLimit.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(
					suite.T(),
					"GetOrganizationUnitUsersByPath",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				)
			},
		},
		{
			name:           "service error",
			url:            "/organization-units/tree/" + defaultOUPath + "/users",
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitUsersByPath",
						mock.Anything, defaultOUPath,
						serverconst.DefaultPageSize, 0, false).
					Return((*UserListResponse)(nil), &serviceerror.InternalServerError).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:           "response write error",
			url:            "/organization-units/tree/" + defaultOUPath + "/users",
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			useFlaky:       true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitUsersByPath",
						mock.Anything, defaultOUPath,
						serverconst.DefaultPageSize, 0, false).
					Return(&UserListResponse{}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				suite.Equal("", recorder.Body.String()) // Write fails, body remains empty
			},
		},
		{
			name:           "success",
			url:            "/organization-units/tree/" + defaultOUPath + "/users?limit=2&offset=1",
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitUsersByPath", mock.Anything, defaultOUPath, 2, 1, false).
					Return(&UserListResponse{TotalResults: 3}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp UserListResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(3, resp.TotalResults)
			},
		},
	}

	suite.runHandlerTestCases(testCases,
		func(handler *organizationUnitHandler, writer http.ResponseWriter, req *http.Request) {
			handler.HandleOUUsersListByPathRequest(writer, req)
		})
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUGroupsListByPathRequest() {
	testCases := []ouHandlerTestCase{
		{
			name: "groups missing path",
			url:  "/organization-units/tree/" + defaultOUPath + "/groups",
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidHandlePath.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				suite.NotNil(serviceMock) // Avoid AST duplication
				serviceMock.AssertNotCalled(
					suite.T(),
					"GetOrganizationUnitGroupsByPath",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				)
			},
		},
		{
			name:           "groups service error",
			url:            "/organization-units/tree/" + defaultOUPath + "/groups",
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitGroupsByPath",
						mock.Anything, defaultOUPath,
						serverconst.DefaultPageSize, 0).
					Return((*GroupListResponse)(nil), &serviceerror.InternalServerError).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:           "groups success",
			url:            "/organization-units/tree/" + defaultOUPath + "/groups?limit=2&offset=1",
			pathParamKey:   "path",
			pathParamValue: defaultOUPath,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitGroupsByPath", mock.Anything, defaultOUPath, 2, 1).
					Return(&GroupListResponse{TotalResults: 1}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp GroupListResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(1, resp.TotalResults)
			},
		},
	}

	suite.runHandlerTestCases(testCases,
		func(handler *organizationUnitHandler, writer http.ResponseWriter, req *http.Request) {
			handler.HandleOUGroupsListByPathRequest(writer, req)
		})
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUUsersListRequest() {
	testCases := []ouHandlerTestCase{
		{
			name: "missing id",
			url:  "/organization-units/" + defaultOURequestID + "/users",
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorMissingOUID.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(
					suite.T(),
					"GetOrganizationUnitUsers",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				)
			},
		},
		{
			name:           "invalid limit",
			url:            "/organization-units/" + defaultOURequestID + "/users?limit=abc",
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorInvalidLimit.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(
					suite.T(),
					"GetOrganizationUnitUsers",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				)
			},
		},
		{
			name:           "service error",
			url:            "/organization-units/" + defaultOURequestID + "/users",
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitUsers",
						mock.Anything, defaultOURequestID,
						serverconst.DefaultPageSize, 0, false).
					Return((*UserListResponse)(nil), &serviceerror.InternalServerError).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusInternalServerError, recorder.Code)
				var body apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Equal(serviceerror.InternalServerError.Code, body.Code)
			},
		},
		{
			name:           "response write error",
			url:            "/organization-units/" + defaultOURequestID + "/users",
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			useFlaky:       true,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitUsers",
						mock.Anything, defaultOURequestID,
						serverconst.DefaultPageSize, 0, false).
					Return(&UserListResponse{}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				suite.Equal("", recorder.Body.String()) // Write fails, body remains empty
			},
		},
		{
			name:           "success",
			url:            "/organization-units/" + defaultOURequestID + "/users?limit=2&offset=1",
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitUsers", mock.Anything, defaultOURequestID, 2, 1, false).
					Return(&UserListResponse{TotalResults: 1}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp UserListResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(1, resp.TotalResults)
			},
		},
	}

	suite.runHandlerTestCases(testCases,
		func(handler *organizationUnitHandler, writer http.ResponseWriter, req *http.Request) {
			handler.HandleOUUsersListRequest(writer, req)
		})
}

func (suite *OrganizationUnitHandlerTestSuite) buildIncludeDisplayTestCases(
	baseURL, paramKey, paramValue, serviceMethod string,
) []ouHandlerTestCase {
	return []ouHandlerTestCase{
		{
			name:           "include=display passes true to service",
			url:            baseURL + "?limit=10&include=display",
			pathParamKey:   paramKey,
			pathParamValue: paramValue,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On(serviceMethod, mock.Anything, paramValue, 10, 0, true).
					Return(&UserListResponse{
						TotalResults: 1,
						Users:        []User{{ID: "user-1", Display: "Alice"}},
					}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp UserListResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal("Alice", resp.Users[0].Display)
			},
		},
		{
			name:           "invalid include value passes false to service",
			url:            baseURL + "?limit=10&include=invalid",
			pathParamKey:   paramKey,
			pathParamValue: paramValue,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On(serviceMethod, mock.Anything, paramValue, 10, 0, false).
					Return(&UserListResponse{
						TotalResults: 1,
						Users:        []User{{ID: "user-1"}},
					}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp UserListResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal("", resp.Users[0].Display)
			},
		},
	}
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUUsersListRequest_WithIncludeDisplay() {
	testCases := suite.buildIncludeDisplayTestCases(
		"/organization-units/"+defaultOURequestID+"/users",
		"id", defaultOURequestID, "GetOrganizationUnitUsers",
	)
	suite.runHandlerTestCases(testCases,
		func(handler *organizationUnitHandler, writer http.ResponseWriter, req *http.Request) {
			handler.HandleOUUsersListRequest(writer, req)
		})
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUUsersListByPathRequest_WithIncludeDisplay() {
	testCases := suite.buildIncludeDisplayTestCases(
		"/organization-units/tree/"+defaultOUPath+"/users",
		"path", defaultOUPath, "GetOrganizationUnitUsersByPath",
	)
	suite.runHandlerTestCases(testCases,
		func(handler *organizationUnitHandler, writer http.ResponseWriter, req *http.Request) {
			handler.HandleOUUsersListByPathRequest(writer, req)
		})
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_HandleOUGroupsListRequest() {
	testCases := []ouHandlerTestCase{
		{
			name: "missing id",
			url:  "/organization-units/" + defaultOURequestID + "/groups",
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusBadRequest, recorder.Code)
				var resp apierror.ErrorResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(ErrorMissingOUID.Code, resp.Code)
			},
			assertService: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.AssertNotCalled(
					suite.T(),
					"GetOrganizationUnitGroups",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				)
			},
		},
		{
			name:           "success",
			url:            "/organization-units/" + defaultOURequestID + "/groups?limit=2&offset=1",
			pathParamKey:   "id",
			pathParamValue: defaultOURequestID,
			setup: func(serviceMock *OrganizationUnitServiceInterfaceMock) {
				serviceMock.
					On("GetOrganizationUnitGroups", mock.Anything, defaultOURequestID, 2, 1).
					Return(&GroupListResponse{TotalResults: 5}, nil).
					Once()
			},
			assert: func(recorder *httptest.ResponseRecorder) {
				suite.Equal(http.StatusOK, recorder.Code)
				var resp GroupListResponse
				suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &resp))
				suite.Equal(5, resp.TotalResults)
			},
		},
	}

	suite.runHandlerTestCases(testCases,
		func(handler *organizationUnitHandler, writer http.ResponseWriter, req *http.Request) {
			handler.HandleOUGroupsListRequest(writer, req)
		})
}

func (suite *OrganizationUnitHandlerTestSuite) TestOUHandler_handleErrorStatusMapping() {
	handler := newOrganizationUnitHandler(NewOrganizationUnitServiceInterfaceMock(suite.T()))

	tests := []struct {
		name       string
		err        *serviceerror.ServiceError
		wantStatus int
	}{
		{
			name:       "not found maps 404",
			err:        &ErrorOrganizationUnitNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "name conflict maps 409",
			err:        &ErrorOrganizationUnitNameConflict,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "unauthorized maps 403",
			err:        &serviceerror.ErrorUnauthorized,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "invalid filter maps 400",
			err:        &ErrorInvalidFilter,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "server error maps 500",
			err:        &serviceerror.InternalServerError,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			recorder := httptest.NewRecorder()

			handler.handleError(recorder, tc.err)

			suite.Equal(tc.wantStatus, recorder.Code)
			var body apierror.ErrorResponse
			suite.NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
			suite.Equal(tc.err.Code, body.Code)
		})
	}
}
