// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cors"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

type InitTestSuite struct {
	suite.Suite
	mux *http.ServeMux
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupTest() {
	config.ResetServerRuntime()
	cfg := &config.Config{Server: engineconfig.ServerConfig{Identifier: "test-deployment"}}
	suite.Require().NoError(config.InitializeServerRuntime("", cfg))
	suite.mux = http.NewServeMux()
}

func (suite *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// TestInitialize wires the module via the public Initialize entrypoint (store + cache + service +
// handler + routes) and verifies the routes are registered by hitting the OPTIONS handler, which
// returns without touching the service or database.
func (suite *InitTestSuite) TestInitialize() {
	cacheManager := cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment")

	svc, exporter, err := Initialize(suite.mux, cacheManager, map[ConfigName]ServerConfigHandlerInterface{})
	suite.Require().NoError(err)
	suite.Require().NotNil(svc)
	suite.Require().NotNil(exporter)

	req := httptest.NewRequest(http.MethodOptions, "/server-config", nil)
	w := httptest.NewRecorder()
	suite.mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNoContent, w.Code)

	req = httptest.NewRequest(http.MethodOptions, "/server-config/cors", nil)
	w = httptest.NewRecorder()
	suite.mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNoContent, w.Code)
}

// TestInitialize_SeedsCORSFromDeploymentConfig verifies that a deployment.yaml cors.allowedOrigins
// section reaches the merged CORS config end to end through the public Initialize entrypoint, when the
// server config store mode has a read-only declarative layer.
func (suite *InitTestSuite) TestInitialize_SeedsCORSFromDeploymentConfig() {
	config.ResetServerRuntime()

	var corsCfg cors.OriginConfig
	suite.Require().NoError(yaml.Unmarshal(
		[]byte("allowedOrigins:\n  - https://app.example.com\n"), &corsCfg))
	cfg := &config.Config{
		Server:       engineconfig.ServerConfig{Identifier: "test-deployment"},
		ServerConfig: config.ServerConfigConfig{Store: "declarative"},
		CORS:         corsCfg,
	}
	suite.Require().NoError(config.InitializeServerRuntime("", cfg))

	cacheManager := cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment")
	svc, _, err := Initialize(suite.mux, cacheManager,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}})
	suite.Require().NoError(err)

	merged, svcErr := svc.GetMergedConfig(context.Background(), string(ConfigNameCORS))
	suite.Require().Nil(svcErr)
	originCfg, ok := merged.(cors.OriginConfig)
	suite.Require().True(ok)
	suite.Require().Equal(corsCfg.AllowedOrigins, originCfg.AllowedOrigins)
}

// TestInitializeStore_DeclarativeMode_SeedCORSErrorPropagates verifies that initializeStore surfaces a
// seedCORSFromDeploymentConfig failure (a deployment.yaml cors section with no registered handler) as
// its own error under the declarative store mode, instead of silently returning a usable store.
func (suite *InitTestSuite) TestInitializeStore_DeclarativeMode_SeedCORSErrorPropagates() {
	suite.assertInitializeStoreErrorsOnMissingCORSHandler("declarative")
}

// TestInitializeStore_CompositeMode_SeedCORSErrorPropagates is the same check under the composite store
// mode, which seeds the same read-only layer as declarative mode before the resource-file scan.
func (suite *InitTestSuite) TestInitializeStore_CompositeMode_SeedCORSErrorPropagates() {
	suite.assertInitializeStoreErrorsOnMissingCORSHandler("composite")
}

// TestInitializeStore_MutableMode_SeedMutableCORSErrorPropagates is the same check under the mutable
// store mode, which seeds the writable (db) layer via seedMutableCORSFromDeploymentConfig instead. The
// missing-handler error is returned before any store/db calls, so this is safe to exercise without a
// live database.
func (suite *InitTestSuite) TestInitializeStore_MutableMode_SeedMutableCORSErrorPropagates() {
	suite.assertInitializeStoreErrorsOnMissingCORSHandler("mutable")
}

// assertInitializeStoreErrorsOnMissingCORSHandler configures a deployment.yaml cors section under the
// given server_config.store mode but passes no cors handler, so the seed step's "no handler registered"
// error must propagate out of initializeStore as a non-nil error and a nil store.
func (suite *InitTestSuite) assertInitializeStoreErrorsOnMissingCORSHandler(storeMode string) {
	config.ResetServerRuntime()

	var corsCfg cors.OriginConfig
	suite.Require().NoError(yaml.Unmarshal(
		[]byte("allowedOrigins:\n  - https://app.example.com\n"), &corsCfg))
	cfg := &config.Config{
		Server:       engineconfig.ServerConfig{Identifier: "test-deployment"},
		ServerConfig: config.ServerConfigConfig{Store: storeMode},
		CORS:         corsCfg,
	}
	suite.Require().NoError(config.InitializeServerRuntime("", cfg))

	cacheManager := cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment")
	resultStore, err := initializeStore(cacheManager, map[ConfigName]ServerConfigHandlerInterface{})
	suite.Require().Error(err)
	suite.Require().Nil(resultStore)
}
