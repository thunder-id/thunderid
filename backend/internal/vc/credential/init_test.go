// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
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

// TestDeclarativeModeRejectsCreate verifies the management API cannot create a
// configuration when the store is declarative-only. The write would otherwise land in
// the in-memory declarative store and disappear on restart.
func (s *InitTestSuite) TestDeclarativeModeRejectsCreate() {
	original := config.GetServerRuntime().Config.OpenID4VCI.Store
	defer func() { config.GetServerRuntime().Config.OpenID4VCI.Store = original }()
	config.GetServerRuntime().Config.OpenID4VCI.Store = string(serverconst.StoreModeDeclarative)

	ouSvc := newOUServiceMock(s.T(), map[string]bool{"ou-1": true},
		map[string]string{"root": "ou-1"}, map[string]string{"ou-1": "root"})
	fileStore, err := initializeStore(ouSvc)
	s.Require().NoError(err)

	svc := newCredentialConfigurationService(fileStore, ouSvc)
	_, svcErr := svc.CreateCredentialConfiguration(context.Background(), &CredentialConfigurationDTO{
		Handle: "decl-mode-create",
		OUID:   "ou-1",
		VCT:    "urn:example:vct",
	})

	s.Require().NotNil(svcErr, "a create must be rejected in declarative-only mode")
	s.Equal(ErrorConfigurationDeclarativeModeCreateNotAllowed.Code, svcErr.Code)
}
