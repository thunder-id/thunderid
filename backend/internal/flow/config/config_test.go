// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowconfig

import (
	"testing"
	"time"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type FlowConfigTestSuite struct {
	suite.Suite
}

func TestFlowConfigTestSuite(t *testing.T) {
	suite.Run(t, new(FlowConfigTestSuite))
}

func (s *FlowConfigTestSuite) SetupTest() {
	config.ResetServerRuntime()
}

func (s *FlowConfigTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *FlowConfigTestSuite) TestFromServerRuntime() {
	cfg := &config.Config{
		Flow: engineconfig.FlowConfig{},
		Server: engineconfig.ServerConfig{
			HTTPOnly: true,
		},
		JWT: engineconfig.JWTConfig{Leeway: 30},
	}
	err := config.InitializeServerRuntime("/tmp/test-flow-config", cfg)
	s.Require().NoError(err)

	result := FromServerRuntime()

	s.False(result.SecureCookies, "HTTPOnly deployment must not mark cookies Secure")
	s.Equal(30*time.Second, result.TokenLeeway)
	// Session config is sourced from the server-config section at the composition root, not here.
	s.Zero(result.Session.IdleTimeoutSeconds)
	s.Zero(result.Session.AbsoluteTimeoutSeconds)
}
