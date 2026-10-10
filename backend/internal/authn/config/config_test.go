// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type ConfigTestSuite struct {
	suite.Suite
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (s *ConfigTestSuite) SetupTest() {
	config.ResetServerRuntime()
}

func (s *ConfigTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *ConfigTestSuite) initRuntime(cfg *config.Config) {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), cfg))
}

func (s *ConfigTestSuite) TestFromServerRuntime() {
	enabled := true
	s.initRuntime(&config.Config{
		Server: engineconfig.ServerConfig{
			SecurityConfig: engineconfig.SecurityConfig{DirectAuthSecret: "test-secret"},
		},
		DirectAPI: config.DirectAPIConfig{Enabled: &enabled},
	})

	result := FromServerRuntime()

	s.Equal("test-secret", result.DirectAuthSecret)
	s.True(result.DirectAPIEnabled)
}

func (s *ConfigTestSuite) TestFromServerRuntime_DirectAPIDisabled() {
	disabled := false
	s.initRuntime(&config.Config{DirectAPI: config.DirectAPIConfig{Enabled: &disabled}})

	s.False(FromServerRuntime().DirectAPIEnabled)
}

func (s *ConfigTestSuite) TestFromServerRuntime_Unset() {
	s.initRuntime(&config.Config{})

	result := FromServerRuntime()

	s.Empty(result.DirectAuthSecret)
	s.False(result.DirectAPIEnabled, "unset falls back to false; default.json supplies true")
}
