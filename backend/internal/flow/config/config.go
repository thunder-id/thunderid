// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package flowconfig holds flow-specific configuration injected at initialization.
package flowconfig

import (
	"net/url"
	"time"

	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Config holds configuration values required by flow services.
type Config struct {
	Flow engineconfig.FlowConfig
	// SecureCookies marks SSO cookies Secure; it is derived from the deployment's HTTP-only setting and
	// public URL scheme.
	SecureCookies bool
	// Session holds the SSO session lifetime configuration used for both server-side timeouts and the
	// cookie lifetime. It is sourced from the server-config "session" section at the composition root,
	// not the static server runtime, so FromServerRuntime leaves it zero for the caller to populate.
	Session flowsession.Config
	// TokenLeeway is how long past exp a token is still accepted, from the deployment's JWT leeway.
	TokenLeeway time.Duration
}

// FlowTypeConfig holds the server-level defaults for a flow operation.
type FlowTypeConfig struct {
	DefaultHandle string `json:"defaultHandle,omitempty"`
	ExpirySeconds int64  `json:"expirySeconds,omitempty"`
}

// FlowSectionConfig is the value of the server-config "flow" section. It carries per-type default
// handles and context TTLs. A zero ExpirySeconds falls back to the built-in default for that type.
// The lifecycle entries name the suspend, unsuspend and unlock administration flows. Applications
// never lock, so have no unlock.
type FlowSectionConfig struct {
	AuthFlow                FlowTypeConfig `json:"authFlow"`
	RegistrationFlow        FlowTypeConfig `json:"registrationFlow"`
	UserOnboardingFlow      FlowTypeConfig `json:"userOnboardingFlow"`
	RecoveryFlow            FlowTypeConfig `json:"recoveryFlow"`
	SignOutFlow             FlowTypeConfig `json:"signOutFlow"`
	SelfServiceFlow         FlowTypeConfig `json:"selfServiceFlow"`
	UserDeletionFlow        FlowTypeConfig `json:"userDeletionFlow"`
	ApplicationDeletionFlow FlowTypeConfig `json:"applicationDeletionFlow"`
	SecretRegenerationFlow  FlowTypeConfig `json:"secretRegenerationFlow"`
	AgentOnboardingFlow     FlowTypeConfig `json:"agentOnboardingFlow"`

	RoleAssignmentRemovalFlow  FlowTypeConfig `json:"roleAssignmentRemovalFlow"`
	RoleDeletionFlow           FlowTypeConfig `json:"roleDeletionFlow"`
	RolePermissionRemovalFlow  FlowTypeConfig `json:"rolePermissionRemovalFlow"`
	GroupDeletionFlow          FlowTypeConfig `json:"groupDeletionFlow"`
	GroupMembershipRemovalFlow FlowTypeConfig `json:"groupMembershipRemovalFlow"`
	ActionDeletionFlow         FlowTypeConfig `json:"actionDeletionFlow"`

	UserUnlockFlow           FlowTypeConfig `json:"userUnlockFlow"`
	UserSuspendFlow          FlowTypeConfig `json:"userSuspendFlow"`
	UserUnsuspendFlow        FlowTypeConfig `json:"userUnsuspendFlow"`
	AgentUnlockFlow          FlowTypeConfig `json:"agentUnlockFlow"`
	AgentSuspendFlow         FlowTypeConfig `json:"agentSuspendFlow"`
	AgentUnsuspendFlow       FlowTypeConfig `json:"agentUnsuspendFlow"`
	ApplicationSuspendFlow   FlowTypeConfig `json:"applicationSuspendFlow"`
	ApplicationUnsuspendFlow FlowTypeConfig `json:"applicationUnsuspendFlow"`
}

// FromServerRuntime builds flow configuration from the global server runtime.
func FromServerRuntime() Config {
	runtime := config.GetServerRuntime()
	return Config{
		Flow:          runtime.Config.Flow,
		SecureCookies: secureCookies(runtime.Config.Server),
		TokenLeeway:   time.Duration(runtime.Config.JWT.Leeway) * time.Second,
	}
}

// secureCookies reports whether SSO cookies must be marked Secure. They are Secure when the server
// serves TLS itself, or when its public URL is HTTPS, as behind a proxy that terminates TLS.
func secureCookies(server engineconfig.ServerConfig) bool {
	if !server.HTTPOnly {
		return true
	}
	publicURL, err := url.Parse(server.PublicURL)
	return err == nil && publicURL.Scheme == "https"
}
