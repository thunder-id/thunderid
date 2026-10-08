// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package backchannel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/observabilityprovidermock"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) SetupSuite() {
	// The HTTP client reads the TLS settings from the server runtime.
	s.Require().NoError(config.InitializeServerRuntime("", &config.Config{}))
}

func (s *InitTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
}

// initialize calls Initialize with fresh mocks; none of them is called while building.
func (s *InitTestSuite) initialize(cfg oauthconfig.Config) DispatcherInterface {
	return Initialize(tokenservicemock.NewTokenBuilderInterfaceMock(s.T()),
		actorprovidermock.NewActorProviderMock(s.T()),
		observabilityprovidermock.NewObservabilityProviderMock(s.T()), cfg)
}

func oauthConfig(enabled bool) oauthconfig.Config {
	var cfg oauthconfig.Config
	cfg.OAuth.Logout.Backchannel = engineconfig.BackchannelLogoutConfig{
		Enabled: &enabled, RequestTimeout: 5, MaxAttempts: 3, RetryDelay: 2, MaxInFlight: 16, QueueSize: 1024,
	}
	return cfg
}

func (s *InitTestSuite) TestInitialize_ReturnsDispatcherWhenEnabled() {
	d := s.initialize(oauthConfig(true))

	s.Require().NotNil(d)
	assert.IsType(s.T(), &dispatcher{}, d)
}

func (s *InitTestSuite) TestInitialize_ReturnsNilWhenDisabled() {
	assert.Nil(s.T(), s.initialize(oauthConfig(false)))
}
