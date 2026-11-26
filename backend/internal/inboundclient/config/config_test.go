// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package inboundclientconfig

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type ConfigTestSuite struct {
	suite.Suite
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (s *ConfigTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *ConfigTestSuite) TestFromServerRuntimeReadsAllowPrivateAddresses() {
	for _, allow := range []bool{true, false} {
		config.ResetServerRuntime()
		cfg := &config.Config{}
		cfg.Server.AllowPrivateAddresses = allow
		s.Require().NoError(config.InitializeServerRuntime("", cfg))

		s.Equal(allow, FromServerRuntime().AllowPrivateAddresses)
	}
}
