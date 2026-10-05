// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package serverconfig provides the server-wide configuration store and service layer.
package serverconfig

// ConfigName identifies a server-wide configuration section.
type ConfigName string

const (
	// ConfigNameCORS is the configuration key for runtime-mutable CORS allowed origins.
	ConfigNameCORS ConfigName = "cors"
	// ConfigNameDefaultResourceServer is the configuration key for the default resource server.
	ConfigNameDefaultResourceServer ConfigName = "defaultResourceServer"
	// ConfigNameSession is the configuration key for the SSO session lifetime timeouts.
	ConfigNameSession ConfigName = "session"
	// ConfigNameFlow is the configuration key for the flow defaults.
	ConfigNameFlow ConfigName = "flow"
	// ConfigNameCSP is the configuration key for the deny-first Content-Security-Policy baseline.
	ConfigNameCSP ConfigName = "csp"
	// ConfigNameOAuth is the configuration key for the runtime-mutable OAuth policy, one block per
	// sub-domain. It currently carries the graceful refresh token rotation policy.
	ConfigNameOAuth ConfigName = "oauth"
)

// supportedConfigNames lists all the supported server configuration names.
var supportedConfigNames = []ConfigName{
	ConfigNameCORS,
	ConfigNameDefaultResourceServer,
	ConfigNameSession,
	ConfigNameFlow,
	ConfigNameCSP,
	ConfigNameOAuth,
}

// IsValid reports whether the config name is one of the supported values.
func (n ConfigName) IsValid() bool {
	for _, supported := range supportedConfigNames {
		if n == supported {
			return true
		}
	}
	return false
}
