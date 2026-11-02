// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

type HTTPConnectionConfigTestSuite struct {
	suite.Suite
}

func TestHTTPConnectionConfigTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionConfigTestSuite))
}

func (s *HTTPConnectionConfigTestSuite) SetupTest() {
	config.ResetServerRuntime()
}

func (s *HTTPConnectionConfigTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *HTTPConnectionConfigTestSuite) TestStoreModeResolution() {
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
			initStoreRuntime(s.T(), tc.store, tc.declarative)
			mode, err := resolvedStoreMode(config.HTTPConnectionConfig{Store: tc.store})
			s.Require().NoError(err)
			s.Equal(tc.want, mode)
			s.Equal(tc.loads, ShouldLoadDeclarativeHTTPConnectionResources())
		})
	}
}

func (s *HTTPConnectionConfigTestSuite) TestAnUnknownStoreModeIsRefused() {
	initStoreRuntime(s.T(), "file", false)

	_, err := resolvedStoreMode(config.HTTPConnectionConfig{Store: "file"})

	s.Require().ErrorContains(err, `invalid http_connection store mode "file"`)
	s.False(ShouldLoadDeclarativeHTTPConnectionResources())
}

// initStoreRuntime initializes the server runtime with the given HTTP connection store and global
// declarative setting.
func initStoreRuntime(t *testing.T, store string, declarative bool) {
	t.Helper()
	config.ResetServerRuntime()
	if err := config.InitializeServerRuntime("/tmp/test", &config.Config{
		HTTPConnection:       config.HTTPConnectionConfig{Store: store},
		DeclarativeResources: config.DeclarativeResources{Enabled: declarative},
	}); err != nil {
		t.Fatal(err)
	}
}
