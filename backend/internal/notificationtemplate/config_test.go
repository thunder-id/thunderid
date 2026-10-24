// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

type ConfigTestSuite struct {
	suite.Suite
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

// withRuntime installs a server runtime with the given explicit store mode and global declarative
// flag, resetting the singleton before and after so the tests do not leak state.
func (s *ConfigTestSuite) withRuntime(storeMode string, declarativeEnabled bool) {
	config.ResetServerRuntime()
	cfg := &config.Config{}
	cfg.Notification.Template.Store = storeMode
	cfg.DeclarativeResources.Enabled = declarativeEnabled
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), cfg))
}

func (s *ConfigTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *ConfigTestSuite) TestExplicitStoreModeWins() {
	cases := []struct {
		input string
		want  serverconst.StoreMode
	}{
		{"mutable", serverconst.StoreModeMutable},
		{"declarative", serverconst.StoreModeDeclarative},
		{"composite", serverconst.StoreModeComposite},
		// Case and surrounding whitespace are normalized.
		{"  COMPOSITE ", serverconst.StoreModeComposite},
	}
	for _, c := range cases {
		s.withRuntime(c.input, false)
		mode, err := getStoreMode()
		s.Require().NoError(err, "store=%q", c.input)
		s.Require().Equal(c.want, mode, "store=%q", c.input)
	}
}

func (s *ConfigTestSuite) TestInvalidExplicitModeErrors() {
	// An unrecognized explicit value is a hard error (mirrors group), not a silent fallback.
	s.withRuntime("garbage", true)
	_, err := getStoreMode()
	s.Require().Error(err)
}

func (s *ConfigTestSuite) TestUnsetFollowsDeclarativeFlag() {
	s.withRuntime("", true)
	mode, err := getStoreMode()
	s.Require().NoError(err)
	s.Require().Equal(serverconst.StoreModeDeclarative, mode)

	s.withRuntime("", false)
	mode, err = getStoreMode()
	s.Require().NoError(err)
	s.Require().Equal(serverconst.StoreModeMutable, mode)
}
