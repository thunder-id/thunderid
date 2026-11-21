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
			ProductName:    "Acme IAM",
			SecurityConfig: engineconfig.SecurityConfig{DirectAuthSecret: "test-secret"},
		},
		DirectAPI: config.DirectAPIConfig{Enabled: &enabled},
	})

	result := FromServerRuntime()

	s.Equal("test-secret", result.DirectAuthSecret)
	s.True(result.DirectAPIEnabled)
	s.Equal("Acme IAM", result.ProductName)
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

func (s *ConfigTestSuite) TestFromServerRuntime_PasskeyAllowedOrigins() {
	s.initRuntime(&config.Config{
		Passkey:    config.PasskeyConfig{AllowedOrigins: []string{"https://app.example.com"}},
		GateClient: engineconfig.GateClientConfig{Scheme: "https", Hostname: "signin.example.org", Port: 443},
	})

	s.Equal([]string{"https://app.example.com"}, FromServerRuntime().PasskeyAllowedOrigins)
}

func (s *ConfigTestSuite) TestFromServerRuntime_PasskeyAllowedOriginsDefaultToTheGate() {
	s.initRuntime(&config.Config{
		GateClient: engineconfig.GateClientConfig{Scheme: "https", Hostname: "signin.example.org", Port: 8090},
	})

	s.Equal([]string{"https://signin.example.org:8090"}, FromServerRuntime().PasskeyAllowedOrigins)
}

func (s *ConfigTestSuite) TestGateOrigin() {
	cases := []struct {
		name string
		gate engineconfig.GateClientConfig
		want string
	}{
		{"https default port", engineconfig.GateClientConfig{Scheme: "https", Hostname: "a.example.org", Port: 443},
			"https://a.example.org"},
		{"http default port", engineconfig.GateClientConfig{Scheme: "http", Hostname: "a.example.org", Port: 80},
			"http://a.example.org"},
		{"other port", engineconfig.GateClientConfig{Scheme: "https", Hostname: "a.example.org", Port: 8090},
			"https://a.example.org:8090"},
		{"IPv6 other port", engineconfig.GateClientConfig{Scheme: "https", Hostname: "::1", Port: 8090},
			"https://[::1]:8090"},
		{"IPv6 default port", engineconfig.GateClientConfig{Scheme: "https", Hostname: "::1", Port: 443},
			"https://[::1]"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Equal(tc.want, gateOrigin(tc.gate))
		})
	}
}
