// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authn

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/suite"

	authnconfig "github.com/thunder-id/thunderid/internal/authn/config"
	"github.com/thunder-id/thunderid/internal/system/config"
)

type AuthenticationInitTestSuite struct {
	suite.Suite
}

var (
	initRuntimeMutex sync.Mutex
)

func TestAuthenticationInitTestSuite(t *testing.T) {
	suite.Run(t, new(AuthenticationInitTestSuite))
}

func initializeTestRuntime(root string) error {
	testConfig := &config.Config{
		Server: engineconfig.ServerConfig{
			Hostname: "localhost",
			Port:     8090,
		},
		JWT: engineconfig.JWTConfig{
			Issuer: "test-issuer",
		},
	}
	return config.InitializeServerRuntime(root, testConfig)
}

func (suite *AuthenticationInitTestSuite) SetupSuite() {
	initRuntimeMutex.Lock()
	config.ResetServerRuntime()
	suite.Require().NoError(initializeTestRuntime(suite.T().TempDir()))
}

func (suite *AuthenticationInitTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
	initRuntimeMutex.Unlock()
}

var directAPIPaths = []string{
	"/auth/credentials/authenticate",
	"/auth/otp/sms/send",
	"/auth/otp/sms/verify",
	"/auth/oauth/google/start",
	"/auth/oauth/google/finish",
	"/auth/oauth/github/start",
	"/auth/oauth/github/finish",
	"/auth/oauth/standard/start",
	"/auth/oauth/standard/finish",
	"/register/passkey/start",
	"/register/passkey/finish",
	"/auth/passkey/start",
	"/auth/passkey/finish",
}

func (suite *AuthenticationInitTestSuite) initialize(directAPIEnabled bool) (*http.ServeMux,
	DirectAuthGuardInterface) {
	mux := http.NewServeMux()
	_, guard := Initialize(mux, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		authnconfig.Config{DirectAuthSecret: "test-secret", DirectAPIEnabled: directAPIEnabled})
	return mux, guard
}

func serve(mux *http.ServeMux, method, path string) int {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec.Code
}

func (suite *AuthenticationInitTestSuite) TestInitialize_DirectAPIEnabled_RegistersRoutes() {
	mux, guard := suite.initialize(true)

	suite.NotNil(guard)
	for _, path := range directAPIPaths {
		suite.Equal(http.StatusUnauthorized, serve(mux, http.MethodPost, path), "POST %s", path)
		suite.Equal(http.StatusNoContent, serve(mux, http.MethodOptions, path), "OPTIONS %s", path)
	}
}

func (suite *AuthenticationInitTestSuite) TestInitialize_DirectAPIDisabled_SkipsRoutes() {
	mux, guard := suite.initialize(false)

	suite.NotNil(guard)
	for _, path := range directAPIPaths {
		suite.Equal(http.StatusNotFound, serve(mux, http.MethodPost, path), "POST %s", path)
		suite.Equal(http.StatusNotFound, serve(mux, http.MethodOptions, path), "OPTIONS %s", path)
	}
}
