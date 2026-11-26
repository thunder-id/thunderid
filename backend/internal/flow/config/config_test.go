// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowconfig

import (
	"context"
	"testing"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/mock"
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
		JWT:        engineconfig.JWTConfig{Leeway: 30},
		GateClient: engineconfig.GateClientConfig{Hostname: "signin.example.org"},
		Session: config.SessionConfig{
			ActivityRefreshIntervalSeconds: 90,
			MaxIdleTimeoutSeconds:          3600,
			MaxAbsoluteTimeoutSeconds:      86400,
		},
	}
	err := config.InitializeServerRuntime("/tmp/test-flow-config", cfg)
	s.Require().NoError(err)

	result := FromServerRuntime()

	s.False(result.SecureCookies, "HTTPOnly deployment must not mark cookies Secure")
	s.Equal(30*time.Second, result.TokenLeeway)
	s.Equal("signin.example.org", result.GateHostname)
	s.Equal(90*time.Second, result.SessionActivityRefresh)
	s.Equal(time.Hour, result.SessionMaxIdleTimeout)
	s.Equal(24*time.Hour, result.SessionMaxAbsoluteTimeout)
}

func (s *FlowConfigTestSuite) TestFromServerRuntimeSessionTracking() {
	cases := []struct {
		name    string
		enabled *bool
		want    bool
	}{
		{name: "enabled", enabled: new(true), want: true},
		{name: "disabled", enabled: new(false), want: false},
		{name: "unset", enabled: nil, want: false},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			config.ResetServerRuntime()
			cfg := &config.Config{
				Session: config.SessionConfig{Tracking: config.SessionTrackingConfig{Enabled: tc.enabled}},
			}
			s.Require().NoError(config.InitializeServerRuntime("/tmp/test-flow-config", cfg))

			s.Equal(tc.want, FromServerRuntime().SessionTracking)
		})
	}
}

func (s *FlowConfigTestSuite) TestFromServerRuntimeSecureCookies() {
	cases := []struct {
		name      string
		httpOnly  bool
		publicURL string
		want      bool
	}{
		{name: "TLS server", httpOnly: false, want: true},
		{name: "TLS server with HTTP public URL", httpOnly: false, publicURL: "http://id.example.com", want: true},
		{name: "HTTP-only server without public URL", httpOnly: true, want: false},
		{name: "HTTP-only server with HTTP public URL", httpOnly: true, publicURL: "http://id.example.com",
			want: false},
		{name: "HTTP-only server behind TLS proxy", httpOnly: true, publicURL: "https://id.example.com",
			want: true},
		{name: "HTTP-only server with uppercase HTTPS public URL", httpOnly: true,
			publicURL: "HTTPS://id.example.com", want: true},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			config.ResetServerRuntime()
			cfg := &config.Config{
				Server: engineconfig.ServerConfig{HTTPOnly: tc.httpOnly, PublicURL: tc.publicURL},
			}
			s.Require().NoError(config.InitializeServerRuntime("/tmp/test-flow-config", cfg))

			s.Equal(tc.want, FromServerRuntime().SecureCookies)
		})
	}
}

func (s *FlowConfigTestSuite) TestReadFlowSectionConfig() {
	section := FlowSectionConfig{DefaultReturnURL: "https://home.example.com"}
	testCases := []struct {
		name     string
		merged   any
		svcErr   *tidcommon.ServiceError
		expected FlowSectionConfig
	}{
		{"Section", section, nil, section},
		{"ReadError", nil, &tidcommon.InternalServerError, FlowSectionConfig{}},
		{"UnexpectedType", "not a section", nil, FlowSectionConfig{}},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			reader := NewServerConfigReaderMock(s.T())
			reader.EXPECT().GetMergedConfig(mock.Anything, "flow").Return(tc.merged, tc.svcErr).Once()

			s.Equal(tc.expected, ReadFlowSectionConfig(context.Background(), reader))
		})
	}
}

func (s *FlowConfigTestSuite) TestReadFlowSectionConfig_NilReader() {
	s.Equal(FlowSectionConfig{}, ReadFlowSectionConfig(context.Background(), nil))
}
