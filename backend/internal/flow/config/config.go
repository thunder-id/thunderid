// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package flowconfig holds flow-specific configuration injected at initialization.
package flowconfig

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Config holds configuration values required by flow services.
type Config struct {
	Flow engineconfig.FlowConfig
	// SecureCookies marks SSO cookies Secure; it is derived from the deployment's HTTP-only setting and
	// public URL scheme.
	SecureCookies bool
	// SessionActivityRefresh is the shortest time between two writes of an SSO session's activity, from
	// session.activity_refresh_interval_seconds.
	SessionActivityRefresh time.Duration
	// SessionMaxIdleTimeout is the longest idle timeout the "session" section may set, from
	// session.max_idle_timeout_seconds.
	SessionMaxIdleTimeout time.Duration
	// SessionMaxAbsoluteTimeout is the longest absolute timeout the "session" section may set, from
	// session.max_absolute_timeout_seconds.
	SessionMaxAbsoluteTimeout time.Duration
	// SessionTracking records a direct session for a login through a flow without a Session node, from
	// session.tracking.enabled.
	SessionTracking bool
	// TokenLeeway is how long past exp a token is still accepted, from the deployment's JWT leeway.
	TokenLeeway time.Duration
	// GateHostname is the host of the gate that serves the sign-in pages.
	GateHostname string
}

// FlowTypeConfig holds the server-level defaults for one flow type.
type FlowTypeConfig struct {
	DefaultHandle string `json:"defaultHandle,omitempty"`
	ExpirySeconds int64  `json:"expirySeconds,omitempty"`
}

// FlowSectionConfig is the value of the server-config "flow" section. It carries per-type default
// handles and context TTLs. A zero ExpirySeconds falls back to the built-in default for that type.
// The lifecycle entries name the suspend, unsuspend and unlock administration flows. Applications
// never lock, so have no unlock. AdminAttributeVerificationFlow names the administrator-initiated flow
// the Console starts to send a user a verification link, and AdminUserCredentialResetFlow the one that resets a
// user's credential. AdminAgentOwnerVerificationFlow names the one the Console starts to send an agent's external owner
// a verification link. DefaultReturnURL is where a sign-in page sends the user when it has no flow to continue
// and no application URL to return to.
type FlowSectionConfig struct {
	DefaultReturnURL string `json:"defaultReturnUrl,omitempty"`

	AuthFlow            FlowTypeConfig `json:"authFlow"`
	RegistrationFlow    FlowTypeConfig `json:"registrationFlow"`
	UserOnboardingFlow  FlowTypeConfig `json:"userOnboardingFlow"`
	RecoveryFlow        FlowTypeConfig `json:"recoveryFlow"`
	SignOutFlow         FlowTypeConfig `json:"signOutFlow"`
	SelfFlow            FlowTypeConfig `json:"selfFlow"`
	AdminInitiatedFlow  FlowTypeConfig `json:"adminInitiatedFlow"`
	AgentOnboardingFlow FlowTypeConfig `json:"agentOnboardingFlow"`

	AdminAttributeVerificationFlow  FlowTypeConfig `json:"adminAttributeVerificationFlow"`
	AdminUserCredentialResetFlow    FlowTypeConfig `json:"adminUserCredentialResetFlow"`
	AdminAgentOwnerVerificationFlow FlowTypeConfig `json:"adminAgentOwnerVerificationFlow"`

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
		SessionActivityRefresh: time.Duration(runtime.Config.Session.ActivityRefreshIntervalSeconds) *
			time.Second,
		SessionMaxIdleTimeout:     time.Duration(runtime.Config.Session.MaxIdleTimeoutSeconds) * time.Second,
		SessionMaxAbsoluteTimeout: time.Duration(runtime.Config.Session.MaxAbsoluteTimeoutSeconds) * time.Second,
		SessionTracking:           runtime.Config.Session.Tracking.IsEnabled(),
		TokenLeeway:               time.Duration(runtime.Config.JWT.Leeway) * time.Second,
		GateHostname:              runtime.Config.GateClient.Hostname,
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

// ServerConfigReader reads a merged server config section. The server config service implements it.
type ServerConfigReader interface {
	GetMergedConfig(ctx context.Context, name string) (any, *tidcommon.ServiceError)
}

// ReadFlowSectionConfig reads the merged server config "flow" section. It returns a zero FlowSectionConfig when
// reader is nil or the section can't be read, so callers fall back to built-in defaults.
func ReadFlowSectionConfig(ctx context.Context, reader ServerConfigReader) FlowSectionConfig {
	if reader == nil {
		return FlowSectionConfig{}
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowConfig"))
	merged, svcErr := reader.GetMergedConfig(ctx, "flow")
	if svcErr != nil {
		logger.Error(ctx, "Failed to read the flow server config section; using built-in defaults",
			log.String("error", svcErr.Error.DefaultValue))
		return FlowSectionConfig{}
	}

	cfg, ok := merged.(FlowSectionConfig)
	if !ok {
		logger.Error(ctx, "Unexpected type for the flow server config section; using built-in defaults",
			log.String("type", fmt.Sprintf("%T", merged)))
		return FlowSectionConfig{}
	}

	return cfg
}
