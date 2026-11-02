// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package config holds authentication-specific configuration injected at initialization.
package config

import (
	"github.com/thunder-id/thunderid/internal/system/config"
)

// Config holds the authentication service configuration.
type Config struct {
	// DirectAPIEnabled controls whether the Direct API endpoints are registered.
	DirectAPIEnabled bool
	// DirectAuthSecret gates the Direct API endpoints. An empty value blocks them.
	DirectAuthSecret string
}

// FromServerRuntime builds a Config from the server runtime.
func FromServerRuntime() Config {
	runtime := config.GetServerRuntime()
	return Config{
		DirectAuthSecret: runtime.Config.Server.SecurityConfig.DirectAuthSecret,
		DirectAPIEnabled: runtime.Config.DirectAPI.IsEnabled(),
	}
}
