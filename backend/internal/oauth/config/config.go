// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package oauthconfig holds OAuth-specific configuration injected at initialization.
package oauthconfig

import (
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Config holds configuration values required by OAuth services.
type Config struct {
	DeploymentID           string
	RuntimeTransientDBType string
	BaseURL                string
	JWT                    engineconfig.JWTConfig
	OAuth                  engineconfig.OAuthConfig
	GateClient             engineconfig.GateClientConfig
}

// FromServerRuntime builds OAuth configuration from the global server runtime, seeding the
// additional OIDC fields with the server defaults.
func FromServerRuntime() Config {
	runtime := config.GetServerRuntime()
	oauth := runtime.Config.OAuth.ToEngineConfig()
	applyOIDCDefaults(&oauth)

	return Config{
		DeploymentID:           runtime.Config.Server.Identifier,
		RuntimeTransientDBType: runtime.Config.Database.RuntimeTransient.Type,
		BaseURL:                config.GetServerURL(&runtime.Config.Server),
		JWT:                    runtime.Config.JWT,
		OAuth:                  oauth,
		GateClient:             runtime.Config.GateClient,
	}
}

// applyOIDCDefaults seeds the default values for the additional OIDC fields on the given OAuthConfig.
func applyOIDCDefaults(oauth *engineconfig.OAuthConfig) {
	mapping := make(map[string][]string, len(constants.StandardOIDCScopes))
	scopes := make([]string, 0, len(constants.StandardOIDCScopes))

	claimSet := make(map[string]struct{})
	for _, c := range constants.GetStandardClaims() {
		claimSet[c] = struct{}{}
	}
	for scope, def := range constants.StandardOIDCScopes {
		claims := make([]string, len(def.Claims))
		copy(claims, def.Claims)
		mapping[scope] = claims
		scopes = append(scopes, scope)
		for _, c := range def.Claims {
			claimSet[c] = struct{}{}
		}
	}

	claims := make([]string, 0, len(claimSet))
	for c := range claimSet {
		claims = append(claims, c)
	}

	oauth.DefaultScopeClaimsMapping = mapping
	oauth.AllowedScopes = scopes
	oauth.AllowedClaims = claims

	oauth.AllowedSubjectTypes = defaultAllowedSubjectTypes()
	if len(oauth.AllowedGrantTypes) == 0 {
		oauth.AllowedGrantTypes = defaultAllowedGrantTypes()
	}
	if len(oauth.AllowedResponseTypes) == 0 {
		oauth.AllowedResponseTypes = defaultAllowedResponseTypes()
	}
	if len(oauth.AllowedAuthMethods) == 0 {
		oauth.AllowedAuthMethods = defaultAllowedAuthMethods()
	}
}

// defaultAllowedSubjectTypes returns the default allowed OIDC subject types for the server.
func defaultAllowedSubjectTypes() []string {
	return []string{constants.SubjectTypePublic}
}

// defaultAllowedGrantTypes returns the default allowed grant types for the server.
func defaultAllowedGrantTypes() []string {
	result := make([]string, len(providers.SupportedGrantTypes))
	for i, v := range providers.SupportedGrantTypes {
		result[i] = string(v)
	}
	return result
}

// defaultAllowedResponseTypes returns the default allowed response types for the server.
func defaultAllowedResponseTypes() []string {
	result := make([]string, len(providers.SupportedResponseTypes))
	for i, v := range providers.SupportedResponseTypes {
		result[i] = string(v)
	}
	return result
}

// defaultAllowedAuthMethods returns the default allowed token endpoint authentication methods for the server.
func defaultAllowedAuthMethods() []string {
	result := make([]string, len(providers.SupportedTokenEndpointAuthMethods))
	for i, v := range providers.SupportedTokenEndpointAuthMethods {
		result[i] = string(v)
	}
	return result
}
