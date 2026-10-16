// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
)

const (
	consoleAppPath  = "/console"
	consoleBasePath = "/console/auth"
	consoleCookie   = "tid_console"
)

// Config describes one BFF: the OAuth client it signs in as and where it is mounted.
type Config struct {
	ClientID string
	Scopes   []string
	Resource string
	// PublicURL is the origin the browser reaches the server on, without a trailing slash.
	PublicURL string
	// AppPath is the path the app is served from. Sign-in and sign-out land back on it.
	AppPath string
	// BasePath is the path the BFF endpoints are mounted under.
	BasePath string
	// CookieName is the stem every BFF cookie name is built from.
	CookieName string
	// SecureCookies marks cookies Secure and gives them the __Host- and __Secure- prefixes.
	SecureCookies bool
	// BlockedProxyPrefixes are upstream paths the API proxy refuses to forward to.
	BlockedProxyPrefixes []string
	// SessionMode is SessionModeStateful or SessionModeStateless.
	SessionMode string
	// IdleTimeout is how long, in seconds, a stateful session lasts without being used.
	IdleTimeout int64
}

// consoleConfigFromServerRuntime builds the Console BFF configuration from the server configuration.
func consoleConfigFromServerRuntime() Config {
	cfg := config.GetServerRuntime().Config
	publicURL := strings.TrimSuffix(config.GetServerURL(&cfg.Server), "/")
	return Config{
		ClientID:             cfg.ConsoleClient.ClientID,
		Scopes:               cfg.ConsoleClient.Scopes,
		Resource:             cfg.ConsoleClient.Resource,
		PublicURL:            publicURL,
		AppPath:              consoleAppPath,
		BasePath:             consoleBasePath,
		CookieName:           consoleCookie,
		SecureCookies:        !cfg.Server.HTTPOnly,
		BlockedProxyPrefixes: []string{consoleAppPath + "/", cfg.GateClient.Path + "/"},
		SessionMode:          cfg.ConsoleClient.Session.Mode,
		IdleTimeout:          cfg.ConsoleClient.Session.IdleTimeout,
	}
}
