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

package group

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// testEncodingErrorBody is the expected response body when a response write fails mid-encode.
var testEncodingErrorBody = func() string {
	resp := apierror.ErrorResponse{
		Code:        serviceerror.ErrorEncodingError.Code,
		Message:     serviceerror.ErrorEncodingError.Error,
		Description: serviceerror.ErrorEncodingError.ErrorDescription,
	}
	b, _ := json.Marshal(resp)
	return string(b)
}()

type flakyResponseWriter struct {
	*httptest.ResponseRecorder
	failNext bool
}

type GroupHandlerTestSuite struct {
	suite.Suite
}

func TestGroupHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(GroupHandlerTestSuite))
}

type handlerTestCase struct {
	name           string
	url            string
	method         string
	pathParamKey   string
	pathParamValue string
	body           string
	useFlaky       bool
	setJSONHeader  bool
	setup          func(*GroupServiceInterfaceMock)
	assert         func(*httptest.ResponseRecorder)
	assertService  func(*GroupServiceInterfaceMock)
}

const testOUID = "ou-001"

func runHandlerTestCases(
	suite *GroupHandlerTestSuite,
	testCases []handlerTestCase,
	invoke func(*groupHandler, http.ResponseWriter, *http.Request),
) {
	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			serviceMock := NewGroupServiceInterfaceMock(suite.T())
			handler := newGroupHandler(serviceMock)

			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}

			method := tc.method
			if method == "" {
				method = http.MethodGet
			}

			req := httptest.NewRequest(method, tc.url, body)
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

func (suite *GroupHandlerTestSuite) SetupTest() {
	config.ResetServerRuntime()

	err := config.InitializeServerRuntime("", &config.Config{})
	suite.Require().NoError(err)
}

func (suite *GroupHandlerTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *GroupHandlerTestSuite) ensureRuntime() {
	config.ResetServerRuntime()

	err := config.InitializeServerRuntime("", &config.Config{})
	suite.Require().NoError(err)
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

func mapStringToValues(m map[string]string) url.Values {
	values := url.Values{}
	for k, v := range m {
		values.Set(k, v)
	}
	return values
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_RegisterRoutesOptionsGroups() {
	t := suite.T()
	suite.ensureRuntime()
	mux := http.NewServeMux()
	registerRoutes(mux, newGroupHandler(nil))

	req := httptest.NewRequest(http.MethodOptions, "/groups", nil)
	resp := httptest.NewRecorder()

	mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNoContent, resp.Code)
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_RegisterRoutesGroupIDDispatch() {
	t := suite.T()
	suite.ensureRuntime()
	mux := http.NewServeMux()
	serviceMock := NewGroupServiceInterfaceMock(t)
	handler := newGroupHandler(serviceMock)
	registerRoutes(mux, handler)

	serviceMock.
		On("GetGroup", mock.Anything, "grp-001", false).
		Return(&Group{ID: "grp-001"}, nil).
		Once()

	req := httptest.NewRequest(http.MethodGet, "/groups/grp-001", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_RegisterRoutesGroupMembersDispatch() {
	t := suite.T()
	suite.ensureRuntime()
	mux := http.NewServeMux()
	serviceMock := NewGroupServiceInterfaceMock(t)
	handler := newGroupHandler(serviceMock)
	registerRoutes(mux, handler)

	serviceMock.
		On("GetGroupMembers", mock.Anything, "grp-001", serverconst.DefaultPageSize, 0, false).
		Return(&MemberListResponse{}, nil).
		Once()

	req := httptest.NewRequest(http.MethodGet, "/groups/grp-001/members", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_RegisterRoutesGroupIDNotFoundPath() {
	t := suite.T()
	suite.ensureRuntime()
	mux := http.NewServeMux()
	registerRoutes(mux, newGroupHandler(nil))

	req := httptest.NewRequest(http.MethodGet, "/groups/grp-001/unknown", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_RegisterRoutesOptionsGroupID() {
	t := suite.T()
	suite.ensureRuntime()
	mux := http.NewServeMux()
	registerRoutes(mux, newGroupHandler(nil))

	req := httptest.NewRequest(http.MethodOptions, "/groups/grp-001", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNoContent, resp.Code)
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_RegisterRoutesOptionsTreePath() {
	t := suite.T()
	suite.ensureRuntime()
	mux := http.NewServeMux()
	registerRoutes(mux, newGroupHandler(nil))

	req := httptest.NewRequest(http.MethodOptions, "/groups/tree/root", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNoContent, resp.Code)
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_HandleGroupListRequest() {
	type responseCheck func(*httptest.ResponseRecorder)
	type serviceSetup func(*GroupServiceInterfaceMock)
	type serviceAssert func(*GroupServiceInterfaceMock)

	testCases := []struct {
		name        string
		requestPath string
		setup       serviceSetup
		useFlaky    bool
		assertBody  responseCheck
		assertSvc   serviceAssert
	}{
		{
			name:        "success",
			requestPath: "/groups?limit=3&offset=2",
			setup: func(svc *GroupServiceInterfaceMock) {
				svc.
					On("GetGroupList", mock.Anything, 3, 2, false).
					Return(&GroupListResponse{
						TotalResults: 5,
						StartIndex:   3,
						Count:        2,
						Groups: []GroupBasic{
							{ID: "g1", Name: "group-1"},
							{ID: "g2", Name: "group-2"},
						},
					}, nil).
					Once()
			},
			assertBody: func(recorder *httptest.ResponseRecorder) {
				suite.Require().Equal(http.StatusOK, recorder.Code)
				suite.Require().Equal(serverconst.ContentTypeJSON,
					recorder.Header().Get(serverconst.ContentTypeHeaderName))

				var body GroupListResponse
				suite.Require().NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Require().Equal(5, body.TotalResults)
				suite.Require().Equal(2, body.Count)
				suite.Require().Len(body.Groups, 2)
				suite.Require().Equal("group-1", body.Groups[0].Name)
			},
		},
		{
			name:        "success with include display",
			requestPath: "/groups?limit=3&offset=0&include=display",
			setup: func(svc *GroupServiceInterfaceMock) {
				svc.
					On("GetGroupList", mock.Anything, 3, 0, true).
					Return(&GroupListResponse{
						TotalResults: 1,
						Count:        1,
						Groups: []GroupBasic{
							{ID: "g1", Name: "group-1", OUHandle: "root"},
						},
					}, nil).
					Once()
			},
			assertBody: func(recorder *httptest.ResponseRecorder) {
				suite.Require().Equal(http.StatusOK, recorder.Code)
				var body GroupListResponse
				suite.Require().NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Require().Equal("root", body.Groups[0].OUHandle)
			},
		},
		{
			name:        "invalid limit",
			requestPath: "/groups?limit=invalid",
			assertBody: func(recorder *httptest.ResponseRecorder) {
				suite.Require().Equal(http.StatusBadRequest, recorder.Code)
				suite.Require().Equal(serverconst.ContentTypeJSON,
					recorder.Header().Get(serverconst.ContentTypeHeaderName))

				var body apierror.ErrorResponse
				suite.Require().NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Require().Equal(ErrorInvalidLimit.Code, body.Code)
				suite.Require().Equal(ErrorInvalidLimit.Error, body.Message)
			},
			assertSvc: func(svc *GroupServiceInterfaceMock) {
				svc.AssertNotCalled(suite.T(), "GetGroupList", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:        "response write error",
			requestPath: "/groups",
			useFlaky:    true,
			setup: func(svc *GroupServiceInterfaceMock) {
				svc.
					On("GetGroupList", mock.Anything, serverconst.DefaultPageSize, 0, false).
					Return(&GroupListResponse{}, nil).
					Once()
			},
			assertBody: func(recorder *httptest.ResponseRecorder) {
				suite.Require().Equal(http.StatusOK, recorder.Code)
				suite.Require().Equal("", recorder.Body.String()) // Write fails, body remains empty
			},
		},
		{
			name:        "client error response write failure",
			requestPath: "/groups?limit=invalid",
			useFlaky:    true,
			assertBody: func(recorder *httptest.ResponseRecorder) {
				suite.Require().Equal(http.StatusBadRequest, recorder.Code)
				suite.Require().Equal(testEncodingErrorBody, recorder.Body.String())
			},
			assertSvc: func(svc *GroupServiceInterfaceMock) {
				svc.AssertNotCalled(suite.T(), "GetGroupList", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:        "service error",
			requestPath: "/groups",
			setup: func(svc *GroupServiceInterfaceMock) {
				svc.
					On("GetGroupList", mock.Anything, serverconst.DefaultPageSize, 0, false).
					Return((*GroupListResponse)(nil), &serviceerror.InternalServerError).
					Once()
			},
			assertBody: func(recorder *httptest.ResponseRecorder) {
				suite.Require().Equal(http.StatusInternalServerError, recorder.Code)
				var body apierror.ErrorResponse
				suite.Require().NoError(json.Unmarshal(recorder.Body.Bytes(), &body))
				suite.Require().Equal(serviceerror.InternalServerError.Code, body.Code)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			serviceMock := NewGroupServiceInterfaceMock(suite.T())
			if tc.setup != nil {
				tc.setup(serviceMock)
			}

			handler := newGroupHandler(serviceMock)
			req := httptest.NewRequest(http.MethodGet, tc.requestPath, nil)

			var (
				writer   http.ResponseWriter
				recorder *httptest.ResponseRecorder
			)

			if tc.useFlaky {
				flaky := newFlakyResponseWriter()
				writer = flaky
				recorder = flaky.ResponseRecorder
			} else {
				recorder = httptest.NewRecorder()
				writer = recorder
			}

			handler.HandleGroupListRequest(writer, req)

			tc.assertBody(recorder)

			if tc.assertSvc != nil {
				tc.assertSvc(serviceMock)
			}

			serviceMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_HandleGroupListByPathRequest() {
	testCases := []handlerTestCase{
		{
			name:           "success",
			method:         http.MethodGet,
			url:            "/ous/root/groups",
			pathParamKey:   "path",
			pathParamValue: "root",
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("GetGroupsByPath", mock.Anything, "root", serverconst.DefaultPageSize, 0, false).
					Return(&GroupListResponse{
						TotalResults: 1,
						StartIndex:   1,
						Count:        1,
						Groups:       []GroupBasic{{ID: "g1", Name: "root-group"}},
					}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusOK, rr.Code)
				var body GroupListResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), 1, body.TotalResults)
				require.Equal(suite.T(), "root-group", body.Groups[0].Name)
			},
		},
		{
			name:           "success with include display",
			method:         http.MethodGet,
			url:            "/ous/root/groups?include=display",
			pathParamKey:   "path",
			pathParamValue: "root",
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("GetGroupsByPath", mock.Anything, "root",
						serverconst.DefaultPageSize, 0, true).
					Return(&GroupListResponse{
						TotalResults: 1,
						StartIndex:   1,
						Count:        1,
						Groups: []GroupBasic{
							{ID: "g1", Name: "root-group", OUHandle: "root"},
						},
					}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusOK, rr.Code)
				var body GroupListResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), "root", body.Groups[0].OUHandle)
			},
		},
		{
			name:   "missing path",
			method: http.MethodGet,
			url:    "/ous//groups",
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), ErrorInvalidRequestFormat.Code, body.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetGroupsByPath",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:           "service error",
			method:         http.MethodGet,
			url:            "/ous/root/groups",
			pathParamKey:   "path",
			pathParamValue: "root",
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("GetGroupsByPath", mock.Anything, "root", serverconst.DefaultPageSize, 0, false).
					Return((*GroupListResponse)(nil), &serviceerror.InternalServerError).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusInternalServerError, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), serviceerror.InternalServerError.Code, body.Code)
			},
		},
		{
			name:           "pagination error",
			method:         http.MethodGet,
			url:            "/ous/root/groups?limit=invalid",
			pathParamKey:   "path",
			pathParamValue: "root",
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), ErrorInvalidLimit.Code, body.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetGroupsByPath",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:           "response write error",
			method:         http.MethodGet,
			url:            "/ous/root/groups",
			pathParamKey:   "path",
			pathParamValue: "root",
			useFlaky:       true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("GetGroupsByPath", mock.Anything, "root", serverconst.DefaultPageSize, 0, false).
					Return(&GroupListResponse{}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusOK, rr.Code)
				require.Equal(suite.T(), "", rr.Body.String()) // Write fails, body remains empty
			},
		},
	}

	runHandlerTestCases(suite, testCases, func(handler *groupHandler, writer http.ResponseWriter, req *http.Request) {
		handler.HandleGroupListByPathRequest(writer, req)
	})
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_HandleGroupPostRequest() {
	testCases := []struct {
		name          string
		body          string
		useFlaky      bool
		setup         func(*GroupServiceInterfaceMock)
		assert        func(*httptest.ResponseRecorder)
		assertService func(*GroupServiceInterfaceMock)
	}{
		{
			name: "invalid json",
			body: "{invalid json",
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				require.Equal(suite.T(), serverconst.ContentTypeJSON,
					rr.Header().Get(serverconst.ContentTypeHeaderName))
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), ErrorInvalidRequestFormat.Code, body.Code)
				require.Contains(suite.T(), body.Description.DefaultValue, "Failed to parse request body")
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "CreateGroup", mock.Anything, mock.Anything)
			},
		},
		{
			name: "success sanitizes payload",
			body: `{
				"name": "  Team <script> ",
				"description": " desc ",
				"ouId": " ou-001 ",
				"members": [
					{"id": " member-1 ", "type": "user"}
				]
			}`,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("CreateGroup", mock.Anything, mock.MatchedBy(func(request CreateGroupRequest) bool {
						return request.Name == "Team &lt;script&gt;" &&
							request.Description == "desc" &&
							request.OUID == testOUID &&
							len(request.Members) == 1 &&
							request.Members[0].ID == "member-1" &&
							request.Members[0].Type == MemberTypeUser
					})).
					Return(&Group{ID: "grp-001", Name: "Team &lt;script&gt;",
						OUID: testOUID}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusCreated, rr.Code)
				require.Equal(suite.T(), serverconst.ContentTypeJSON,
					rr.Header().Get(serverconst.ContentTypeHeaderName))
				var body Group
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), "grp-001", body.ID)
				require.Equal(suite.T(), "Team &lt;script&gt;", body.Name)
			},
		},
		{
			name: "service error",
			body: `{"name":"group","ouId":"ou"}`,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("CreateGroup", mock.Anything, mock.AnythingOfType("group.CreateGroupRequest")).
					Return((*Group)(nil), &ErrorGroupNameConflict).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusConflict, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), ErrorGroupNameConflict.Code, body.Code)
			},
		},
		{
			name: "internal error",
			body: `{"name":"group","ouId":"ou"}`,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("CreateGroup", mock.Anything, mock.AnythingOfType("group.CreateGroupRequest")).
					Return((*Group)(nil), &serviceerror.InternalServerError).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusInternalServerError, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), serviceerror.InternalServerError.Code, body.Code)
			},
		},
		{
			name:     "response write error",
			body:     `{"name":"team","ouId":"ou"}`,
			useFlaky: true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("CreateGroup", mock.Anything, mock.MatchedBy(func(request CreateGroupRequest) bool {
						return request.Name == "team" && request.OUID == "ou"
					})).
					Return(&Group{ID: "grp-001", Name: "team"}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusCreated, rr.Code)
				require.Equal(suite.T(), "", rr.Body.String()) // Write fails, body remains empty
			},
		},
		{
			name:     "error response write failure",
			body:     "{",
			useFlaky: true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				require.Equal(suite.T(), testEncodingErrorBody, rr.Body.String())
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "CreateGroup", mock.Anything, mock.Anything)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			serviceMock := NewGroupServiceInterfaceMock(suite.T())
			handler := newGroupHandler(serviceMock)

			req := httptest.NewRequest(http.MethodPost, "/groups", strings.NewReader(tc.body))
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

			handler.HandleGroupPostRequest(writer, req)

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

func (suite *GroupHandlerTestSuite) TestGroupHandler_HandleGroupPostByPathRequest() {
	testCases := []handlerTestCase{
		{
			name:           "success",
			method:         http.MethodPost,
			url:            "/ous/root/groups",
			pathParamKey:   "path",
			pathParamValue: "root",
			body:           `{"name":"name"}`,
			setJSONHeader:  true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("CreateGroupByPath", mock.Anything, "root", CreateGroupByPathRequest{Name: "name"}).
					Return(&Group{ID: "grp-001", Name: "name"}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusCreated, rr.Code)
				var body Group
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), "grp-001", body.ID)
			},
		},
		{
			name:           "invalid json",
			method:         http.MethodPost,
			url:            "/ous/root/groups",
			pathParamKey:   "path",
			pathParamValue: "root",
			body:           "{",
			setJSONHeader:  true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), ErrorInvalidRequestFormat.Code, body.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "CreateGroupByPath", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:          "invalid path",
			method:        http.MethodPost,
			url:           "/ous//groups",
			body:          `{"name":"n"}`,
			setJSONHeader: true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "CreateGroupByPath", mock.Anything, mock.Anything)
			},
		},
		{
			name:           "service error",
			method:         http.MethodPost,
			url:            "/ous/root/groups",
			pathParamKey:   "path",
			pathParamValue: "root",
			body:           `{"name":"n"}`,
			setJSONHeader:  true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("CreateGroupByPath", mock.Anything, "root", CreateGroupByPathRequest{Name: "n"}).
					Return((*Group)(nil), &ErrorGroupNotFound).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusNotFound, rr.Code)
			},
		},
		{
			name:           "response write error",
			method:         http.MethodPost,
			url:            "/ous/root/groups",
			pathParamKey:   "path",
			pathParamValue: "root",
			body:           `{"name":"team"}`,
			useFlaky:       true,
			setJSONHeader:  true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("CreateGroupByPath", mock.Anything, "root", CreateGroupByPathRequest{Name: "team"}).
					Return(&Group{ID: "grp-001", Name: "team"}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusCreated, rr.Code)
				require.Equal(suite.T(), "", rr.Body.String()) // Write fails, body remains empty
			},
		},
		{
			name:           "error response write failure",
			method:         http.MethodPost,
			url:            "/ous/root/groups",
			pathParamKey:   "path",
			pathParamValue: "root",
			body:           "{",
			useFlaky:       true,
			setJSONHeader:  true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				require.Equal(suite.T(), testEncodingErrorBody, rr.Body.String())
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "CreateGroupByPath", mock.Anything, mock.Anything)
			},
		},
		{
			name:           "internal error",
			method:         http.MethodPost,
			url:            "/ous/root/groups",
			pathParamKey:   "path",
			pathParamValue: "root",
			body:           `{"name":"n"}`,
			setJSONHeader:  true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("CreateGroupByPath", mock.Anything, "root", CreateGroupByPathRequest{Name: "n"}).
					Return((*Group)(nil), &serviceerror.InternalServerError).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusInternalServerError, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), serviceerror.InternalServerError.Code, body.Code)
			},
		},
	}

	runHandlerTestCases(suite, testCases, func(handler *groupHandler, writer http.ResponseWriter, req *http.Request) {
		handler.HandleGroupPostByPathRequest(writer, req)
	})
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_HandleGroupGetRequest() {
	testCases := []handlerTestCase{
		{
			name:           "success with include display",
			method:         http.MethodGet,
			url:            "/groups/grp-001?include=display",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("GetGroup", mock.Anything, "grp-001", true).
					Return(&Group{ID: "grp-001", OUHandle: "root"}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusOK, rr.Code)
				var body Group
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), "root", body.OUHandle)
			},
		},
		{
			name:           "not found",
			method:         http.MethodGet,
			url:            "/groups/grp-404",
			pathParamKey:   "id",
			pathParamValue: "grp-404",
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("GetGroup", mock.Anything, "grp-404", false).
					Return(nil, &ErrorGroupNotFound).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusNotFound, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), ErrorGroupNotFound.Code, body.Code)
				require.Equal(suite.T(), ErrorGroupNotFound.Error, body.Message)
			},
		},
		{
			name:           "internal error",
			method:         http.MethodGet,
			url:            "/groups/grp-001",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("GetGroup", mock.Anything, "grp-001", false).
					Return((*Group)(nil), &serviceerror.InternalServerError).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusInternalServerError, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), serviceerror.InternalServerError.Code, body.Code)
			},
		},
		{
			name:           "response write error",
			method:         http.MethodGet,
			url:            "/groups/grp-001",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			useFlaky:       true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("GetGroup", mock.Anything, "grp-001", false).
					Return(&Group{ID: "grp-001"}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusOK, rr.Code)
				require.Equal(suite.T(), "", rr.Body.String()) // Write fails, body remains empty
			},
		},
		{
			name:     "error response write failure",
			method:   http.MethodGet,
			url:      "/groups/",
			useFlaky: true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				require.Equal(suite.T(), testEncodingErrorBody, rr.Body.String())
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetGroup", mock.Anything, mock.Anything)
			},
		},
		{
			name:   "missing id",
			method: http.MethodGet,
			url:    "/groups/",
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), ErrorMissingGroupID.Code, body.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetGroup", mock.Anything, mock.Anything)
			},
		},
	}

	runHandlerTestCases(suite, testCases, func(handler *groupHandler, writer http.ResponseWriter, req *http.Request) {
		handler.HandleGroupGetRequest(writer, req)
	})
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_HandleGroupPutRequest() {
	testCases := []handlerTestCase{
		{
			name:           "success sanitizes payload",
			method:         http.MethodPut,
			url:            "/groups/grp-001",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body: `{
				"name": " team <script> ",
				"description": " desc ",
				"ouId": " ou-001 "
			}`,
			setJSONHeader: true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("UpdateGroup", mock.Anything, "grp-001", mock.MatchedBy(func(request UpdateGroupRequest) bool {
						return request.Name == "team &lt;script&gt;" &&
							request.Description == "desc" &&
							request.OUID == testOUID
					})).
					Return(&Group{ID: "grp-001"}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusOK, rr.Code)
				var body Group
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), "grp-001", body.ID)
			},
		},
		{
			name:           "service error",
			method:         http.MethodPut,
			url:            "/groups/grp-001",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body:           `{"name":"group","ouId":"ou"}`,
			setJSONHeader:  true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("UpdateGroup", mock.Anything, "grp-001", mock.AnythingOfType("group.UpdateGroupRequest")).
					Return(nil, &ErrorGroupNotFound).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusNotFound, rr.Code)
			},
		},
		{
			name:           "internal error",
			method:         http.MethodPut,
			url:            "/groups/grp-001",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body:           `{"name":"group","ouId":"ou"}`,
			setJSONHeader:  true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("UpdateGroup", mock.Anything, "grp-001", mock.AnythingOfType("group.UpdateGroupRequest")).
					Return(nil, &serviceerror.InternalServerError).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusInternalServerError, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), serviceerror.InternalServerError.Code, body.Code)
			},
		},
		{
			name:           "response write error",
			method:         http.MethodPut,
			url:            "/groups/grp-001",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body:           `{"name":"group","ouId":"ou"}`,
			useFlaky:       true,
			setJSONHeader:  true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("UpdateGroup", mock.Anything, "grp-001", mock.AnythingOfType("group.UpdateGroupRequest")).
					Return(&Group{ID: "grp-001"}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusOK, rr.Code)
				require.Equal(suite.T(), "", rr.Body.String()) // Write fails, body remains empty
			},
		},
		{
			name:           "invalid json",
			method:         http.MethodPut,
			url:            "/groups/grp-001",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body:           "{",
			setJSONHeader:  true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)

				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), ErrorInvalidRequestFormat.Code, body.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "UpdateGroup", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:           "invalid json response write failure",
			method:         http.MethodPut,
			url:            "/groups/grp-001",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body:           "{",
			useFlaky:       true,
			setJSONHeader:  true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				require.Equal(suite.T(), testEncodingErrorBody, rr.Body.String())
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "UpdateGroup", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:          "missing id",
			method:        http.MethodPut,
			url:           "/groups/",
			body:          "{}",
			setJSONHeader: true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)

				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), ErrorMissingGroupID.Code, body.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "UpdateGroup", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:          "missing id response write failure",
			method:        http.MethodPut,
			url:           "/groups/",
			body:          "{}",
			useFlaky:      true,
			setJSONHeader: true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				require.Equal(suite.T(), testEncodingErrorBody, rr.Body.String())
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "UpdateGroup", mock.Anything, mock.Anything, mock.Anything)
			},
		},
	}

	runHandlerTestCases(suite, testCases, func(handler *groupHandler, writer http.ResponseWriter, req *http.Request) {
		handler.HandleGroupPutRequest(writer, req)
	})
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_HandleGroupDeleteRequest() {
	testCases := []handlerTestCase{
		{
			name:   "missing id",
			method: http.MethodDelete,
			url:    "/groups/",
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), ErrorMissingGroupID.Code, body.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "DeleteGroup", mock.Anything, mock.Anything)
			},
		},
		{
			name:     "error response write failure",
			method:   http.MethodDelete,
			url:      "/groups/",
			useFlaky: true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				require.Equal(suite.T(), testEncodingErrorBody, rr.Body.String())
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "DeleteGroup", mock.Anything, mock.Anything)
			},
		},
		{
			name:           "conflict",
			method:         http.MethodDelete,
			url:            "/groups/grp-001",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("DeleteGroup", mock.Anything, "grp-001").
					Return(&ErrorCannotDeleteGroup).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), ErrorCannotDeleteGroup.Code, body.Code)
			},
		},
		{
			name:           "internal error",
			method:         http.MethodDelete,
			url:            "/groups/grp-001",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("DeleteGroup", mock.Anything, "grp-001").
					Return(&serviceerror.InternalServerError).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusInternalServerError, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), serviceerror.InternalServerError.Code, body.Code)
			},
		},
		{
			name:           "success",
			method:         http.MethodDelete,
			url:            "/groups/grp-001",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("DeleteGroup", mock.Anything, "grp-001").
					Return(nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusNoContent, rr.Code)
				require.Empty(suite.T(), rr.Body.String())
			},
		},
	}

	runHandlerTestCases(suite, testCases, func(handler *groupHandler, writer http.ResponseWriter, req *http.Request) {
		handler.HandleGroupDeleteRequest(writer, req)
	})
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_HandleGroupMembersGetRequest() {
	testCases := []handlerTestCase{
		{
			name:           "success",
			method:         http.MethodGet,
			url:            "/groups/grp-001/members?limit=2&offset=1",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("GetGroupMembers", mock.Anything, "grp-001", 2, 1, false).
					Return(&MemberListResponse{
						TotalResults: 3,
						StartIndex:   2,
						Count:        2,
						Members: []Member{
							{ID: "usr-1", Type: MemberTypeUser},
							{ID: "grp-2", Type: MemberTypeGroup},
						},
					}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusOK, rr.Code)
				require.Equal(suite.T(), serverconst.ContentTypeJSON,
					rr.Header().Get(serverconst.ContentTypeHeaderName))
				var body MemberListResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), 3, body.TotalResults)
				require.Len(suite.T(), body.Members, 2)
				require.Equal(suite.T(), "usr-1", body.Members[0].ID)
			},
		},
		{
			name:           "success with include=display",
			method:         http.MethodGet,
			url:            "/groups/grp-001/members?limit=2&offset=0&include=display",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("GetGroupMembers", mock.Anything, "grp-001", 2, 0, true).
					Return(&MemberListResponse{
						TotalResults: 1,
						StartIndex:   1,
						Count:        1,
						Members: []Member{
							{ID: "usr-1", Type: MemberTypeUser, Display: "alice@example.com"},
						},
					}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusOK, rr.Code)
				var body MemberListResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Len(suite.T(), body.Members, 1)
				require.Equal(suite.T(), "alice@example.com", body.Members[0].Display)
			},
		},
		{
			name:           "invalid limit",
			method:         http.MethodGet,
			url:            "/groups/grp-001/members?limit=NaN",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetGroupMembers",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:           "service error",
			method:         http.MethodGet,
			url:            "/groups/grp-001/members",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("GetGroupMembers", mock.Anything, "grp-001", serverconst.DefaultPageSize, 0, false).
					Return((*MemberListResponse)(nil), &ErrorGroupNotFound).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusNotFound, rr.Code)
			},
		},
		{
			name:           "response write error",
			method:         http.MethodGet,
			url:            "/groups/grp-001/members",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			useFlaky:       true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("GetGroupMembers", mock.Anything, "grp-001", serverconst.DefaultPageSize, 0, false).
					Return(&MemberListResponse{}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusOK, rr.Code)
				require.Equal(suite.T(), "", rr.Body.String()) // Write fails, body remains empty
			},
		},
		{
			name:     "error response write failure",
			method:   http.MethodGet,
			url:      "/groups//members",
			useFlaky: true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				require.Equal(suite.T(), testEncodingErrorBody, rr.Body.String())
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetGroupMembers",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:           "internal error",
			method:         http.MethodGet,
			url:            "/groups/grp-001/members",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("GetGroupMembers", mock.Anything, "grp-001", serverconst.DefaultPageSize, 0, false).
					Return((*MemberListResponse)(nil), &serviceerror.InternalServerError).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusInternalServerError, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), serviceerror.InternalServerError.Code, body.Code)
			},
		},
		{
			name:   "missing id",
			method: http.MethodGet,
			url:    "/groups//members",
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "GetGroupMembers",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
	}

	runHandlerTestCases(suite, testCases, func(handler *groupHandler, writer http.ResponseWriter, req *http.Request) {
		handler.HandleGroupMembersGetRequest(writer, req)
	})
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_ParsePaginationParamsInvalidOffset() {
	t := suite.T()
	limit, offset, err := parsePaginationParams(mapStringToValues(map[string]string{
		"limit":  "10",
		"offset": "abc",
	}))

	require.Zero(t, limit)
	require.Zero(t, offset)
	require.NotNil(t, err)
	require.Equal(t, ErrorInvalidOffset, *err)
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_ExtractAndValidatePathEncodeFailure() {
	t := suite.T()
	writer := newFlakyResponseWriter()
	req := httptest.NewRequest(http.MethodGet, "/ous//groups", nil)

	path, failed := extractAndValidatePath(writer, req)

	require.True(t, failed)
	require.Equal(t, "", path)
	require.Equal(t, http.StatusBadRequest, writer.Code)
	require.Equal(t, testEncodingErrorBody, writer.Body.String())
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_HandleErrorInternalServer() {
	t := suite.T()
	handler := newGroupHandler(nil)
	rr := httptest.NewRecorder()

	handler.handleError(rr, &serviceerror.ServiceError{
		Type:             serviceerror.ServerErrorType,
		Code:             "GRP-9999",
		Error:            i18ncore.I18nMessage{DefaultValue: "boom"},
		ErrorDescription: i18ncore.I18nMessage{DefaultValue: "explosion"},
	})

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	var body apierror.ErrorResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "GRP-9999", body.Code)
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_HandleGroupMembersAddRequest() {
	testCases := []handlerTestCase{
		{
			name:           "success",
			method:         http.MethodPost,
			url:            "/groups/grp-001/members/add",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body:           `{"members":[{"id":"usr-001","type":"user"}]}`,
			setJSONHeader:  true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("AddGroupMembers", mock.Anything, "grp-001",
						[]Member{{ID: "usr-001", Type: MemberTypeUser}}).
					Return(&Group{ID: "grp-001", Name: "Test Group"}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusOK, rr.Code)
				var group Group
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &group))
				require.Equal(suite.T(), "grp-001", group.ID)
			},
		},
		{
			name:           "invalid body",
			method:         http.MethodPost,
			url:            "/groups/grp-001/members/add",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body:           `{invalid`,
			setJSONHeader:  true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "AddGroupMembers",
					mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:           "service error - group not found",
			method:         http.MethodPost,
			url:            "/groups/grp-001/members/add",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body:           `{"members":[{"id":"usr-001","type":"user"}]}`,
			setJSONHeader:  true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("AddGroupMembers", mock.Anything, "grp-001", mock.Anything).
					Return(nil, &ErrorGroupNotFound).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusNotFound, rr.Code)
			},
		},
		{
			name:           "service error - empty members",
			method:         http.MethodPost,
			url:            "/groups/grp-001/members/add",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body:           `{"members":[]}`,
			setJSONHeader:  true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("AddGroupMembers", mock.Anything, "grp-001", mock.Anything).
					Return(nil, &ErrorEmptyMembers).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
				var body apierror.ErrorResponse
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(suite.T(), ErrorEmptyMembers.Code, body.Code)
			},
		},
		{
			name:          "missing id",
			method:        http.MethodPost,
			url:           "/groups//members/add",
			body:          `{"members":[{"id":"usr-001","type":"user"}]}`,
			setJSONHeader: true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "AddGroupMembers",
					mock.Anything, mock.Anything, mock.Anything)
			},
		},
	}

	runHandlerTestCases(suite, testCases, func(handler *groupHandler, writer http.ResponseWriter, req *http.Request) {
		handler.HandleGroupMembersAddRequest(writer, req)
	})
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_HandleGroupMembersRemoveRequest() {
	testCases := []handlerTestCase{
		{
			name:           "success",
			method:         http.MethodPost,
			url:            "/groups/grp-001/members/remove",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body:           `{"members":[{"id":"usr-001","type":"user"}]}`,
			setJSONHeader:  true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("RemoveGroupMembers", mock.Anything, "grp-001",
						[]Member{{ID: "usr-001", Type: MemberTypeUser}}).
					Return(&Group{ID: "grp-001", Name: "Test Group"}, nil).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusOK, rr.Code)
				var group Group
				require.NoError(suite.T(), json.Unmarshal(rr.Body.Bytes(), &group))
				require.Equal(suite.T(), "grp-001", group.ID)
			},
		},
		{
			name:           "invalid body",
			method:         http.MethodPost,
			url:            "/groups/grp-001/members/remove",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body:           `{invalid`,
			setJSONHeader:  true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "RemoveGroupMembers",
					mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:           "service error - group not found",
			method:         http.MethodPost,
			url:            "/groups/grp-001/members/remove",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body:           `{"members":[{"id":"usr-001","type":"user"}]}`,
			setJSONHeader:  true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("RemoveGroupMembers", mock.Anything, "grp-001", mock.Anything).
					Return(nil, &ErrorGroupNotFound).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusNotFound, rr.Code)
			},
		},
		{
			name:           "internal server error",
			method:         http.MethodPost,
			url:            "/groups/grp-001/members/remove",
			pathParamKey:   "id",
			pathParamValue: "grp-001",
			body:           `{"members":[{"id":"usr-001","type":"user"}]}`,
			setJSONHeader:  true,
			setup: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.
					On("RemoveGroupMembers", mock.Anything, "grp-001", mock.Anything).
					Return(nil, &serviceerror.InternalServerError).
					Once()
			},
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusInternalServerError, rr.Code)
			},
		},
		{
			name:          "missing id",
			method:        http.MethodPost,
			url:           "/groups//members/remove",
			body:          `{"members":[{"id":"usr-001","type":"user"}]}`,
			setJSONHeader: true,
			assert: func(rr *httptest.ResponseRecorder) {
				require.Equal(suite.T(), http.StatusBadRequest, rr.Code)
			},
			assertService: func(serviceMock *GroupServiceInterfaceMock) {
				serviceMock.AssertNotCalled(suite.T(), "RemoveGroupMembers",
					mock.Anything, mock.Anything, mock.Anything)
			},
		},
	}

	runHandlerTestCases(suite, testCases, func(handler *groupHandler, writer http.ResponseWriter, req *http.Request) {
		handler.HandleGroupMembersRemoveRequest(writer, req)
	})
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_RegisterRoutesMembersAddDispatch() {
	t := suite.T()
	suite.ensureRuntime()
	mux := http.NewServeMux()
	serviceMock := NewGroupServiceInterfaceMock(t)
	handler := newGroupHandler(serviceMock)
	registerRoutes(mux, handler)

	serviceMock.
		On("AddGroupMembers", mock.Anything, "grp-001",
			[]Member{{ID: "usr-001", Type: MemberTypeUser}}).
		Return(&Group{ID: "grp-001", Name: "Test Group"}, nil).
		Once()

	body := strings.NewReader(`{"members":[{"id":"usr-001","type":"user"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/groups/grp-001/members/add", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_RegisterRoutesMembersRemoveDispatch() {
	t := suite.T()
	suite.ensureRuntime()
	mux := http.NewServeMux()
	serviceMock := NewGroupServiceInterfaceMock(t)
	handler := newGroupHandler(serviceMock)
	registerRoutes(mux, handler)

	serviceMock.
		On("RemoveGroupMembers", mock.Anything, "grp-001",
			[]Member{{ID: "usr-001", Type: MemberTypeUser}}).
		Return(&Group{ID: "grp-001", Name: "Test Group"}, nil).
		Once()

	body := strings.NewReader(`{"members":[{"id":"usr-001","type":"user"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/groups/grp-001/members/remove", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_RegisterRoutesMembersInvalidAction() {
	t := suite.T()
	suite.ensureRuntime()
	mux := http.NewServeMux()
	registerRoutes(mux, newGroupHandler(nil))

	body := strings.NewReader(`{"members":[{"id":"usr-001","type":"user"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/groups/grp-001/members/invalid", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func (suite *GroupHandlerTestSuite) TestGroupHandler_HandleErrorClientError() {
	t := suite.T()
	handler := newGroupHandler(nil)
	rr := httptest.NewRecorder()

	handler.handleError(rr, &ErrorGroupNameConflict)

	require.Equal(t, http.StatusConflict, rr.Code)

	var body apierror.ErrorResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, ErrorGroupNameConflict.Code, body.Code)
}
