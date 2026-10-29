// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type fakeImportService struct {
	importFn func(context.Context, *ImportRequest) (*ImportResponse, *tidcommon.ServiceError)
	deleteFn func(context.Context, *DeleteResourceRequest) (*DeleteResourceResponse, *tidcommon.ServiceError)
}

func (f *fakeImportService) ImportResources(
	ctx context.Context, r *ImportRequest,
) (*ImportResponse, *tidcommon.ServiceError) {
	return f.importFn(ctx, r)
}

func (f *fakeImportService) DeleteResource(
	ctx context.Context, r *DeleteResourceRequest,
) (*DeleteResourceResponse, *tidcommon.ServiceError) {
	return f.deleteFn(ctx, r)
}

type ImportHandlerTestSuite struct {
	suite.Suite
	service *fakeImportService
	handler *importHandler
}

func (suite *ImportHandlerTestSuite) SetupTest() {
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test", &config.Config{}))

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
	) (*ImportResponse, *tidcommon.ServiceError) {
		return nil, &tidcommon.InternalServerError
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
	) (*ImportResponse, *tidcommon.ServiceError) {
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
	) (*DeleteResourceResponse, *tidcommon.ServiceError) {
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
	) (*DeleteResourceResponse, *tidcommon.ServiceError) {
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

// An import runs longer than the server's write timeout allows other requests, and answers within
// the import timeout instead. The import only has to outlast the server's 100 ms, which a slow machine
// makes it do all the more; the import timeout is an hour, so no pause can reach it.
func (suite *ImportHandlerTestSuite) TestHandleImportRequest_AnswersWithinTheImportTimeout() {
	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime(suite.T().TempDir(), &config.Config{
		Import: config.ImportConfig{TimeoutSeconds: 3600},
	}))
	suite.service.importFn = func(context.Context, *ImportRequest) (*ImportResponse, *tidcommon.ServiceError) {
		time.Sleep(300 * time.Millisecond)
		return &ImportResponse{}, nil
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(suite.handler.HandleImportRequest))
	server.Config.WriteTimeout = 100 * time.Millisecond
	server.Start()
	suite.T().Cleanup(server.Close)

	resp, err := server.Client().Post(server.URL+"/import", "application/json", strings.NewReader(`{"content":"x"}`))

	suite.Require().NoError(err, "the import lost its connection at the server's write timeout")
	suite.Equal(http.StatusOK, resp.StatusCode)
	suite.Require().NoError(resp.Body.Close())
}
