// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cimd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{}))
}

func (s *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *InitTestSuite) TestInitialize() {
	mux := http.NewServeMux()

	svc := Initialize(mux)

	s.Require().NotNil(svc)
	options := httptest.NewRecorder()
	mux.ServeHTTP(options, httptest.NewRequest(http.MethodOptions, "/cimd/preview", nil))
	s.Equal(http.StatusNoContent, options.Code)

	preview := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cimd/preview",
		strings.NewReader(`{"clientId":"http://client.example.com/client.json"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(preview, req)
	s.Equal(http.StatusBadRequest, preview.Code)
	s.Contains(preview.Body.String(), ErrorInvalidClientID.Code)
}
