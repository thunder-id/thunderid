// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package config holds authentication-specific configuration injected at initialization.
package config

import (
	"net"
	"strconv"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// Config holds the authentication service configuration.
type Config struct {
	// DirectAPIEnabled controls whether the Direct API endpoints are registered.
	DirectAPIEnabled bool
	// DirectAuthSecret gates the Direct API endpoints. An empty value blocks them.
	DirectAuthSecret string
	// ProductName is the product name the authentication MCP tools name in their text.
	ProductName string
	// PasskeyAllowedOrigins are the origins a passkey operation is accepted from when the application
	// names none: the configured passkey allowed origins, or else the origin of the sign-in pages.
	PasskeyAllowedOrigins []string
}

// FromServerRuntime builds a Config from the server runtime.
func FromServerRuntime() Config {
	runtime := config.GetServerRuntime()
	passkeyOrigins := runtime.Config.Passkey.AllowedOrigins
	if len(passkeyOrigins) == 0 {
		passkeyOrigins = []string{gateOrigin(runtime.Config.GateClient)}
	}
	return Config{
		DirectAuthSecret:      runtime.Config.Server.SecurityConfig.DirectAuthSecret,
		DirectAPIEnabled:      runtime.Config.DirectAPI.IsEnabled(),
		ProductName:           runtime.Config.Server.ProductName,
		PasskeyAllowedOrigins: passkeyOrigins,
	}
}

// gateOrigin returns the origin a browser reports for the gate's pages, which leaves out the
// scheme's default port.
func gateOrigin(gate engineconfig.GateClientConfig) string {
	host := gate.Hostname
	isDefaultPort := (gate.Scheme == "https" && gate.Port == 443) || (gate.Scheme == "http" && gate.Port == 80)
	if gate.Port != 0 && !isDefaultPort {
		host = net.JoinHostPort(gate.Hostname, strconv.Itoa(gate.Port))
	} else if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		host = "[" + host + "]"
	}
	return gate.Scheme + "://" + host
}
