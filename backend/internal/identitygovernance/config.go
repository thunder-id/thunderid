// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// accountAccessSourceInterface supplies the effective accountAccess section.
type accountAccessSourceInterface interface {
	// Current returns the effective section, or the deployment configuration when it cannot be read.
	Current(ctx context.Context) accountAccessValue
}

// accountAccessSource reads the accountAccess section on every call, so a write on any node is seen.
// Without a reader, or when a read fails, the deployment configuration applies.
type accountAccessSource struct {
	base   accountAccessValue
	reader serverconfig.ServerConfigService
	logger *log.Logger
}

// newAccountAccessSource builds the source over the deployment configuration and the server-config
// reader, which may be nil.
func newAccountAccessSource(base accountAccessValue,
	reader serverconfig.ServerConfigService) accountAccessSourceInterface {
	return &accountAccessSource{
		base:   effectiveAccountAccess(base, accountAccessValue{}),
		reader: reader,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AccountAccessSource")),
	}
}

// Current reads the effective server-config section.
func (s *accountAccessSource) Current(ctx context.Context) accountAccessValue {
	if s.reader == nil {
		return s.base
	}

	value, svcErr := s.reader.GetMergedConfig(ctx, string(serverconfig.ConfigNameAccountAccess))
	if svcErr != nil {
		s.logger.Warn(ctx, "Failed to read the accountAccess server config; using the deployment policy",
			log.String("code", svcErr.Code))
		return s.base
	}
	section, ok := value.(accountAccessValue)
	if !ok {
		s.logger.Warn(ctx, "Unexpected accountAccess server config type; using the deployment policy")
		return s.base
	}
	return section
}

// Config holds the configuration identity governance is initialized with.
type Config struct {
	// AccountAccess is the deployment's account access policy, the base layer of the accountAccess
	// server-config section.
	AccountAccess accountAccessValue
}

// FromServerRuntime builds the identity governance configuration from the server runtime.
func FromServerRuntime() Config {
	return Config{AccountAccess: accountAccessValueFrom(sysconfig.GetServerRuntime().Config.AccountAccess)}
}

// Validate checks the deployment account access policy on its own. An enabled policy must be able to
// form a lock without any later layer.
func (c Config) Validate() error {
	if err := validateStructure(c.AccountAccess); err != nil {
		return err
	}
	return validateCoherence(effectiveAccountAccess(c.AccountAccess, accountAccessValue{}))
}

// accountAccessValueFrom converts the deployment configuration into its section form.
func accountAccessValueFrom(cfg sysconfig.AccountAccessConfig) accountAccessValue {
	return accountAccessValue{
		User:  accountAccessCategoryFrom(cfg.User),
		Agent: accountAccessCategoryFrom(cfg.Agent),
	}
}

// accountAccessCategoryFrom converts a deployment category. A category that states nothing converts
// to nil, so absence survives the conversion.
func accountAccessCategoryFrom(c sysconfig.CategoryAccessConfig) *accountAccessCategory {
	out := &accountAccessCategory{
		RecordLastLogin:            c.RecordLastLogin,
		LastLoginResolutionSeconds: c.LastLoginResolutionSeconds,
	}
	if c.LockGranularity != "" {
		granularity := c.LockGranularity
		out.LockGranularity = &granularity
	}
	if policy := accountAccessPolicyFrom(c.Default); !policy.IsEmpty() {
		out.Default = &policy
	}
	out.Notifications = accountAccessNotificationsFrom(c.Notifications)
	if out.LockGranularity == nil && out.Default == nil && out.Notifications == nil &&
		out.RecordLastLogin == nil && out.LastLoginResolutionSeconds == nil {
		return nil
	}
	return out
}

// accountAccessPolicyFrom converts a deployment policy.
func accountAccessPolicyFrom(c sysconfig.ScopeAccessConfig) accountAccessPolicy {
	return accountAccessPolicy{
		Enabled:              c.Enabled,
		Threshold:            c.Threshold,
		FailureWindowSeconds: c.FailureWindowSeconds,
		LockDurationsSeconds: c.LockDurationsSeconds,
		LockDecaySeconds:     c.LockDecaySeconds,
	}
}

// accountAccessNotificationsFrom converts a deployment notification block, or returns nil when it
// states nothing.
func accountAccessNotificationsFrom(n sysconfig.AccessNotificationsConfig) *accountAccessNotifications {
	email := n.OnLock.Email
	if email.Enabled == nil && email.RecipientAttribute == nil {
		return nil
	}
	return &accountAccessNotifications{OnLock: &accountAccessLockNotification{Email: &accountAccessLockEmail{
		Enabled:            email.Enabled,
		RecipientAttribute: email.RecipientAttribute,
	}}}
}
