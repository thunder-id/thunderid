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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package importer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	yaml "gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

type fakeImportService struct {
	importFn func(context.Context, *ImportRequest) (*ImportResponse, *serviceerror.ServiceError)
	deleteFn func(context.Context, *DeleteResourceRequest) (*DeleteResourceResponse, *serviceerror.ServiceError)
}

func (f *fakeImportService) ImportResources(
	ctx context.Context, r *ImportRequest,
) (*ImportResponse, *serviceerror.ServiceError) {
	return f.importFn(ctx, r)
}

func (f *fakeImportService) DeleteResource(
	ctx context.Context, r *DeleteResourceRequest,
) (*DeleteResourceResponse, *serviceerror.ServiceError) {
	return f.deleteFn(ctx, r)
}

type ImportHandlerTestSuite struct {
	suite.Suite
	service *fakeImportService
	handler *importHandler
}

func (suite *ImportHandlerTestSuite) SetupTest() {
	config.ResetServerRuntime()
	var allowedOrigins cors.OriginEntries
	suite.Require().NoError(yaml.Unmarshal([]byte(`
- https://localhost:3000
`), &allowedOrigins))
	testConfig := &config.Config{
		CORS: config.CORSConfig{AllowedOrigins: allowedOrigins},
	}
	suite.Require().NoError(cors.InitializeMatcher(testConfig.CORS.AllowedOrigins))
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", testConfig))

	suite.service = &fakeImportService{}
	suite.handler = newImportHandler(suite.service)
}

func (suite *ImportHandlerTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func TestImportHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(ImportHandlerTestSuite))
}

func (suite *ImportHandlerTestSuite) TestHandleImportRequest_InvalidJSON() {
	req := httptest.NewRequest("POST", "/import", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleImportRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *ImportHandlerTestSuite) TestHandleImportRequest_ServerError() {
	suite.service.importFn = func(
		_ context.Context, _ *ImportRequest,
	) (*ImportResponse, *serviceerror.ServiceError) {
		return nil, &serviceerror.InternalServerError
	}

	req := httptest.NewRequest("POST", "/import", strings.NewReader(`{"content":"foo"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleImportRequest(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
}

func (suite *ImportHandlerTestSuite) TestHandleImportRequest_Success() {
	suite.service.importFn = func(
		_ context.Context, _ *ImportRequest,
	) (*ImportResponse, *serviceerror.ServiceError) {
		return &ImportResponse{Summary: &ImportSummary{}, Results: []ImportItemOutcome{}}, nil
	}

	req := httptest.NewRequest("POST", "/import", strings.NewReader(`{"content":"foo"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleImportRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
}

func (suite *ImportHandlerTestSuite) TestHandleDeleteImportRequest_InvalidJSON() {
	req := httptest.NewRequest("DELETE", "/import", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleDeleteImportRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *ImportHandlerTestSuite) TestHandleDeleteImportRequest_ClientError() {
	suite.service.deleteFn = func(
		_ context.Context, _ *DeleteResourceRequest,
	) (*DeleteResourceResponse, *serviceerror.ServiceError) {
		return nil, &ErrorInvalidImportRequest
	}

	req := httptest.NewRequest("DELETE", "/import", strings.NewReader(`{"resourceType":"application"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleDeleteImportRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *ImportHandlerTestSuite) TestHandleDeleteImportRequest_Success() {
	suite.service.deleteFn = func(
		_ context.Context, _ *DeleteResourceRequest,
	) (*DeleteResourceResponse, *serviceerror.ServiceError) {
		return &DeleteResourceResponse{
			ResourceType: "application",
			ResourceKey:  "app1",
			DeletedFile:  "app1.yaml",
		}, nil
	}

	body := `{"resourceType":"application","resourceKey":"app1"}`
	req := httptest.NewRequest("DELETE", "/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleDeleteImportRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
}
