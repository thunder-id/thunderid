/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package authzen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/suite"

	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
)

type HandlerTestSuite struct {
	suite.Suite
}

type testService struct {
	evaluateAccess func(context.Context, AccessEvaluationRequest) (
		*AccessEvaluationResponse, *tidcommon.ServiceError)
	evaluateAccessBatch func(context.Context, AccessEvaluationsRequest) (
		*AccessEvaluationsResponse, *tidcommon.ServiceError)
	searchActions func(context.Context, AccessActionSearchRequest) (
		*AccessSearchResponse, *tidcommon.ServiceError)
}

func (s *testService) EvaluateAccess(ctx context.Context, request AccessEvaluationRequest) (
	*AccessEvaluationResponse, *tidcommon.ServiceError) {
	return s.evaluateAccess(ctx, request)
}

func (s *testService) EvaluateAccessBatch(ctx context.Context, request AccessEvaluationsRequest) (
	*AccessEvaluationsResponse, *tidcommon.ServiceError) {
	return s.evaluateAccessBatch(ctx, request)
}

func (s *testService) SearchActions(ctx context.Context, request AccessActionSearchRequest) (
	*AccessSearchResponse, *tidcommon.ServiceError) {
	return s.searchActions(ctx, request)
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) TestHandleMetadataRequestSuccess() {
	sysconfig.ResetServerRuntime()
	s.Require().NoError(sysconfig.InitializeServerRuntime("/tmp/test", &sysconfig.Config{
		Server: engineconfig.ServerConfig{
			PublicURL: "https://pdp.example.com",
		},
	}))
	defer sysconfig.ResetServerRuntime()

	h := newHandler(&testService{})
	req := httptest.NewRequest(http.MethodGet, "/.well-known/authzen-configuration", nil)
	w := httptest.NewRecorder()

	h.HandleMetadataRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
	var resp MetadataResponse
	s.NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	s.Equal("https://pdp.example.com", resp.PolicyDecisionPoint)
	s.Equal("https://pdp.example.com/access/v1/evaluation", resp.AccessEvaluationEndpoint)
	s.Equal("https://pdp.example.com/access/v1/evaluations", resp.AccessEvaluationsEndpoint)
	s.Equal("https://pdp.example.com/access/v1/search/action", resp.SearchActionEndpoint)
}

func (s *HandlerTestSuite) TestHandleAccessEvaluationRequestSuccess() {
	h := newHandler(&testService{
		evaluateAccess: func(_ context.Context, request AccessEvaluationRequest) (
			*AccessEvaluationResponse, *tidcommon.ServiceError) {
			s.Equal("user1", request.Subject.ID)
			return &AccessEvaluationResponse{Decision: true}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/access/v1/evaluation",
		strings.NewReader(
			`{"subject":{"type":"user","id":"user1"},"resource":{"type":"booking","id":"booking1"},`+
				`"action":{"name":"read"}}`))
	w := httptest.NewRecorder()

	h.HandleAccessEvaluationRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
	var resp AccessEvaluationResponse
	s.NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	s.True(resp.Decision)
}

func (s *HandlerTestSuite) TestHandleAccessEvaluationRequestInvalidJSON() {
	h := newHandler(&testService{})
	req := httptest.NewRequest(http.MethodPost, "/access/v1/evaluation", strings.NewReader(`{`))
	w := httptest.NewRecorder()

	h.HandleAccessEvaluationRequest(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	s.NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	s.Equal(ErrorInvalidRequestFormat.Error.DefaultValue, resp["error"])
}

func (s *HandlerTestSuite) TestHandleAccessEvaluationRequestServiceError() {
	h := newHandler(&testService{
		evaluateAccess: func(_ context.Context, _ AccessEvaluationRequest) (
			*AccessEvaluationResponse, *tidcommon.ServiceError) {
			return nil, &ErrorMissingAction
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/access/v1/evaluation",
		strings.NewReader(`{"subject":{"id":"user1"},"resource":{"type":"booking","id":"booking1"},"action":{}}`))
	w := httptest.NewRecorder()

	h.HandleAccessEvaluationRequest(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	s.NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	s.Equal(ErrorMissingAction.Error.DefaultValue, resp["error"])
}

func (s *HandlerTestSuite) TestHandleAccessEvaluationsRequestSuccess() {
	h := newHandler(&testService{
		evaluateAccessBatch: func(_ context.Context, request AccessEvaluationsRequest) (
			*AccessEvaluationsResponse, *tidcommon.ServiceError) {
			s.Len(request.Evaluations, 2)
			return &AccessEvaluationsResponse{
				Evaluations: []AccessEvaluationResponse{
					{Decision: true},
					{Decision: false},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/access/v1/evaluations",
		strings.NewReader(
			`{"evaluations":[{"subject":{"id":"user1"},"resource":{"type":"booking","id":"booking1"},`+
				`"action":{"name":"read"}},{"subject":{"id":"user1"},"resource":{"type":"booking","id":"booking1"},`+
				`"action":{"name":"delete"}}]}`))
	w := httptest.NewRecorder()

	h.HandleAccessEvaluationsRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
	var resp AccessEvaluationsResponse
	s.NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	s.Len(resp.Evaluations, 2)
	s.True(resp.Evaluations[0].Decision)
	s.False(resp.Evaluations[1].Decision)
}

func (s *HandlerTestSuite) TestHandleActionSearchRequestSuccess() {
	h := newHandler(&testService{
		searchActions: func(_ context.Context, request AccessActionSearchRequest) (
			*AccessSearchResponse, *tidcommon.ServiceError) {
			s.Equal("user1", request.Subject.ID)
			return &AccessSearchResponse{
				Results: []Action{{Name: "read"}},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/access/v1/search/action",
		strings.NewReader(
			`{"subject":{"id":"user1"},"resource":{"type":"booking:booking","id":"booking1"}}`))
	w := httptest.NewRecorder()

	h.HandleActionSearchRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
	var resp AccessSearchResponse
	s.NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	s.Len(resp.Results, 1)
}

func (s *HandlerTestSuite) TestHandleActionSearchRequestInvalidJSON() {
	h := newHandler(&testService{})
	req := httptest.NewRequest(http.MethodPost, "/access/v1/search/action", strings.NewReader(`{`))
	w := httptest.NewRecorder()

	h.HandleActionSearchRequest(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	s.NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	s.Equal(ErrorInvalidRequestFormat.Error.DefaultValue, resp["error"])
}

func (s *HandlerTestSuite) TestHandleActionSearchRequestServiceError() {
	h := newHandler(&testService{
		searchActions: func(_ context.Context, _ AccessActionSearchRequest) (
			*AccessSearchResponse, *tidcommon.ServiceError) {
			return nil, &ErrorMissingResource
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/access/v1/search/action",
		strings.NewReader(`{"subject":{"id":"user1"},"resource":{"type":"booking:booking","id":"booking1"}}`))
	w := httptest.NewRecorder()

	h.HandleActionSearchRequest(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	s.NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	s.Equal(ErrorMissingResource.Error.DefaultValue, resp["error"])
}
