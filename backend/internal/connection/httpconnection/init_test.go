// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

type HTTPConnectionInitTestSuite struct {
	suite.Suite
}

func TestHTTPConnectionInitTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionInitTestSuite))
}

func (s *HTTPConnectionInitTestSuite) initRuntime(store string, declarative bool) {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
		HTTPConnection:       config.HTTPConnectionConfig{Store: store},
		DeclarativeResources: config.DeclarativeResources{Enabled: declarative},
	}))
	s.T().Cleanup(config.ResetServerRuntime)
}

func (s *HTTPConnectionInitTestSuite) TestStoreModeResolution() {
	cases := []struct {
		store       string
		declarative bool
		want        serverconst.StoreMode
		loads       bool
	}{
		{"", false, serverconst.StoreModeMutable, false},
		{"", true, serverconst.StoreModeDeclarative, true},
		{"mutable", true, serverconst.StoreModeMutable, false},
		{" Composite ", false, serverconst.StoreModeComposite, true},
		{"declarative", false, serverconst.StoreModeDeclarative, true},
	}
	for _, tc := range cases {
		s.Run(tc.store, func() {
			s.initRuntime(tc.store, tc.declarative)
			mode, err := resolvedStoreMode(config.HTTPConnectionConfig{Store: tc.store})
			s.Require().NoError(err)
			s.Equal(tc.want, mode)
			s.Equal(tc.loads, ShouldLoadDeclarativeHTTPConnectionResources())
		})
	}
}

func (s *HTTPConnectionInitTestSuite) TestStoreFollowsTheMode() {
	s.initRuntime("", false)
	s.IsType(&fileBasedStore{}, initializeStore(serverconst.StoreModeDeclarative))
	s.IsType(&compositeStore{}, initializeStore(serverconst.StoreModeComposite))
	s.IsType(&httpConnectionStore{}, initializeStore(serverconst.StoreModeMutable))
}

func (s *HTTPConnectionInitTestSuite) TestAnUnknownStoreModeFailsStartup() {
	s.initRuntime("", false)

	service, err := Initialize(config.HTTPConnectionConfig{Store: "file"})

	s.Require().ErrorContains(err, `invalid http_connection store mode "file"`)
	s.Nil(service)

	s.initRuntime("file", false)
	s.False(ShouldLoadDeclarativeHTTPConnectionResources())
}

func (s *HTTPConnectionInitTestSuite) TestInitializeGivesTheServiceItsResolvedMode() {
	s.initRuntime("", false)

	service, err := Initialize(config.HTTPConnectionConfig{Store: "Declarative"})

	s.Require().NoError(err)
	s.Equal(serverconst.StoreModeDeclarative, service.(*httpConnectionService).storeMode)
	s.IsType(&fileBasedStore{}, service.(*httpConnectionService).store)
}
