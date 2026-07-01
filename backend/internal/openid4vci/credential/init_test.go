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

package credential

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) SetupSuite() {
	// InitializeServerRuntime is guarded by sync.Once; initialize it once for the
	// whole package so config-dependent functions have a runtime to read from.
	_ = config.InitializeServerRuntime("/test/thunderid/home", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: testDeploymentID},
	})
}

func (s *InitTestSuite) TestRegisterRoutes() {
	mux := http.NewServeMux()
	h := newConfigurationHandler(NewCredentialConfigurationServiceInterfaceMock(s.T()))
	registerRoutes(mux, h)

	// Each registered route should resolve to a handler.
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, configurationsPath},
		{http.MethodGet, configurationsPath},
		{http.MethodGet, configurationsPath + "/{id}"},
		{http.MethodPut, configurationsPath + "/{id}"},
		{http.MethodDelete, configurationsPath + "/{id}"},
		{http.MethodOptions, configurationsPath},
		{http.MethodOptions, configurationsPath + "/{id}"},
	} {
		req, err := http.NewRequest(tc.method, "http://example.com"+tc.path, nil)
		s.Require().NoError(err)
		handler, pattern := mux.Handler(req)
		s.NotNil(handler)
		s.NotEmpty(pattern, "expected a registered pattern for %s %s", tc.method, tc.path)
	}
}

func (s *InitTestSuite) TestGetCredentialStoreModeValid() {
	cfg := config.GetServerRuntime().Config

	original := cfg.OpenID4VCI.Store
	defer func() { config.GetServerRuntime().Config.OpenID4VCI.Store = original }()

	for _, mode := range []serverconst.StoreMode{
		serverconst.StoreModeMutable,
		serverconst.StoreModeDeclarative,
		serverconst.StoreModeComposite,
	} {
		config.GetServerRuntime().Config.OpenID4VCI.Store = string(mode)
		got, err := getCredentialStoreMode()
		s.Require().NoError(err)
		s.Equal(mode, got)
	}
}

func (s *InitTestSuite) TestGetCredentialStoreModeInvalid() {
	original := config.GetServerRuntime().Config.OpenID4VCI.Store
	defer func() { config.GetServerRuntime().Config.OpenID4VCI.Store = original }()

	config.GetServerRuntime().Config.OpenID4VCI.Store = "bogus"
	_, err := getCredentialStoreMode()
	s.Error(err)
}

func (s *InitTestSuite) TestGetCredentialStoreModeDefaults() {
	original := config.GetServerRuntime().Config.OpenID4VCI.Store
	defer func() { config.GetServerRuntime().Config.OpenID4VCI.Store = original }()

	config.GetServerRuntime().Config.OpenID4VCI.Store = ""
	got, err := getCredentialStoreMode()
	s.Require().NoError(err)
	s.Equal(serverconst.StoreModeMutable, got)
}
