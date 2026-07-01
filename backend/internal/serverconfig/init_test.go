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

package serverconfig

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
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
