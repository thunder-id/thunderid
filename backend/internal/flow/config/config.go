// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package flowconfig holds flow-specific configuration injected at initialization.
package flowconfig

import (
	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Config holds configuration values required by flow services.
type Config struct {
	Flow engineconfig.FlowConfig
	// SecureCookies marks SSO cookies Secure; it is derived from the deployment's HTTP-only setting.
	SecureCookies bool
	// Session holds the SSO session lifetime configuration used for both server-side timeouts and the
	// cookie lifetime. It is sourced from the server-config "session" section at the composition root,
	// not the static server runtime, so FromServerRuntime leaves it zero for the caller to populate.
	Session flowsession.Config
}

// FlowTypeConfig holds the server-level defaults for one flow type.
type FlowTypeConfig struct {
	DefaultHandle string `json:"defaultHandle,omitempty"`
	ExpirySeconds int64  `json:"expirySeconds,omitempty"`
}

// FlowSectionConfig is the value of the server-config "flow" section. It carries per-type default
// handles and context TTLs. A zero ExpirySeconds falls back to the built-in default for that type.
//
// UserDeletionFlow names the administration flow that carries out a user deletion. There is no
// separate mode switch: a console deletes through the flow when one is configured and present, and
// through DELETE /users/{id} otherwise, mirroring how user onboarding falls back to manual creation.
//
// ApplicationDeletionFlow and ClientSecretRegenerationFlow name the administration flows for the two
// application actions that must revoke before they act. They follow the same configured-and-present
// rule as UserDeletionFlow, so a deployment that unsets either handle keeps the native API behavior.
type FlowSectionConfig struct {
	AuthFlow                     FlowTypeConfig `json:"authFlow"`
	RegistrationFlow             FlowTypeConfig `json:"registrationFlow"`
	UserOnboardingFlow           FlowTypeConfig `json:"userOnboardingFlow"`
	RecoveryFlow                 FlowTypeConfig `json:"recoveryFlow"`
	SignOutFlow                  FlowTypeConfig `json:"signOutFlow"`
	UserDeletionFlow             FlowTypeConfig `json:"userDeletionFlow"`
	ApplicationDeletionFlow      FlowTypeConfig `json:"applicationDeletionFlow"`
	ClientSecretRegenerationFlow FlowTypeConfig `json:"clientSecretRegenerationFlow"`
}

// FromServerRuntime builds flow configuration from the global server runtime.
func FromServerRuntime() Config {
	runtime := config.GetServerRuntime()
	return Config{
		Flow:          runtime.Config.Flow,
		SecureCookies: !runtime.Config.Server.HTTPOnly,
	}
}
