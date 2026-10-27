// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// ServerMode is the role a running server plays.
type ServerMode string

const (
	// ServerModeDataPlane runs authentication and issues tokens. It is the mode a server starts in
	// unless its entry point says otherwise.
	ServerModeDataPlane ServerMode = "data_plane"
	// ServerModeControlPlane authors and deploys configuration, and serves no runtime traffic.
	ServerModeControlPlane ServerMode = "control_plane"
)

// ServerRuntime holds the runtime configuration for the server.
type ServerRuntime struct {
	ServerHome            string `yaml:"server_home"`
	GateClientLoginURL    *url.URL
	GateClientCallbackURL *url.URL
	Config                Config `yaml:"config"`
	// Mode is set by the server's entry point and is not part of Config, so no configuration file
	// can change it.
	Mode ServerMode
}

// RuntimeOption changes how the server runtime is initialized.
type RuntimeOption func(*ServerRuntime)

// WithMode sets the mode the server runs in. Only an entry point chooses it; it is never read from
// configuration.
func WithMode(mode ServerMode) RuntimeOption {
	return func(runtime *ServerRuntime) {
		runtime.Mode = mode
	}
}

var (
	runtimeConfig *ServerRuntime
	once          sync.Once
)

// InitializeServerRuntime initializes the server runtime configurations. The server runs as a data
// plane unless WithMode says otherwise.
func InitializeServerRuntime(serverHome string, config *Config, opts ...RuntimeOption) error {
	once.Do(func() {
		loginPath := config.GateClient.LoginPath
		if strings.TrimSpace(loginPath) == "" {
			loginPath = "/signin"
		}
		callbackPath := config.GateClient.CallbackPath
		if strings.TrimSpace(callbackPath) == "" {
			callbackPath = "/callback"
		}

		portStr := strconv.Itoa(config.GateClient.Port)
		hostWithPort := net.JoinHostPort(config.GateClient.Hostname, portStr)

		baseURL := &url.URL{
			Scheme: config.GateClient.Scheme,
			Host:   hostWithPort,
		}

		parsedPath, err := url.Parse(loginPath)
		if err != nil || parsedPath == nil {
			// Runtime initialization runs during application startup, outside any request.
			log.GetLogger().Warn(context.Background(),
				"Invalid gate client login path configured. Falling back to default '/signin'",
				log.String("configuredPath", loginPath),
				log.Error(err),
			)
			parsedPath = &url.URL{Path: "/signin"}
		}

		parsedCallbackPath, err := url.Parse(callbackPath)
		if err != nil || parsedCallbackPath == nil {
			// Runtime initialization runs during application startup, outside any request.
			log.GetLogger().Warn(context.Background(),
				"Invalid gate client callback path configured. Falling back to default '/callback'",
				log.String("configuredPath", callbackPath),
				log.Error(err),
			)
			parsedCallbackPath = &url.URL{Path: "/callback"}
		}

		parsedURL := baseURL.ResolveReference(parsedPath)
		parsedCallbackURL := baseURL.ResolveReference(parsedCallbackPath)

		runtimeConfig = &ServerRuntime{
			ServerHome:            serverHome,
			GateClientLoginURL:    parsedURL,
			GateClientCallbackURL: parsedCallbackURL,
			Config:                *config,
			Mode:                  ServerModeDataPlane,
		}
		for _, opt := range opts {
			opt(runtimeConfig)
		}
	})
	return nil
}

// GetServerRuntime returns the server runtime configurations.
func GetServerRuntime() *ServerRuntime {
	if runtimeConfig == nil {
		panic("Server runtime is not initialized")
	}
	return runtimeConfig
}

// GetServerMode returns the mode the server is running in.
func GetServerMode() ServerMode {
	return GetServerRuntime().Mode
}

// IsServerRuntimeInitialized reports whether the server runtime has been initialized, without
// panicking. Useful for callers that have a sensible fallback when config isn't loaded yet
// (e.g. in unit tests that don't call InitializeServerRuntime).
func IsServerRuntimeInitialized() bool {
	return runtimeConfig != nil
}

// ResetServerRuntime resets the server runtime.
// This should only be used in tests to reset the singleton state.
func ResetServerRuntime() {
	runtimeConfig = nil
	once = sync.Once{}
}
