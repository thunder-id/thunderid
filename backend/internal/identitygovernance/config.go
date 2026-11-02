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
// When a read fails, the deployment configuration applies.
type accountAccessSource struct {
	base   accountAccessValue
	reader serverconfig.ServerConfigService
	logger *log.Logger
}

// newAccountAccessSource builds the source over the deployment configuration and the server-config
// service the section is read from.
func newAccountAccessSource(base accountAccessValue,
	reader serverconfig.ServerConfigService) accountAccessSourceInterface {
	return &accountAccessSource{
		base:   base,
		reader: reader,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AccountAccessSource")),
	}
}

// Current reads the effective server-config section.
func (s *accountAccessSource) Current(ctx context.Context) accountAccessValue {
	value, svcErr := s.reader.GetMergedConfig(ctx, string(serverconfig.ConfigNameAccountAccess))
	if svcErr != nil {
		s.logger.Warn(ctx, "Failed to read the accountAccess server config; using the deployment policy",
			log.String("errorCode", svcErr.Code))
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
	return validateCoherence(c.AccountAccess)
}
