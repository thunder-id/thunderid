// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"sync"
	"sync/atomic"

	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ConfigReader reads the merged (effective) value of a server-config section.
type ConfigReader interface {
	GetMergedConfig(ctx context.Context, name string) (any, *common.ServiceError)
}

// AccountAccessSource reads the `accountAccess` section on every call, so a write on any node is
// seen. Without a reader, or when a read fails, the deployment configuration applies.
type AccountAccessSource struct {
	base   AccountAccessValue
	logger *log.Logger

	readerMu sync.RWMutex
	reader   ConfigReader
}

// activeSource is the source InstallConfigReader installs a reader on.
var activeSource atomic.Pointer[AccountAccessSource]

// NewAccountAccessSource builds the source over the deployment configuration and sets it as the
// active source.
func NewAccountAccessSource(base sysconfig.AccountAccessConfig, logger *log.Logger) *AccountAccessSource {
	source := &AccountAccessSource{base: NewAccountAccessHandler(base).Base(), logger: logger}
	activeSource.Store(source)
	return source
}

// Current reads the effective server-config section.
func (s *AccountAccessSource) Current(ctx context.Context) AccountAccessValue {
	s.readerMu.RLock()
	reader := s.reader
	s.readerMu.RUnlock()
	if reader == nil {
		return s.base
	}

	value, svcErr := reader.GetMergedConfig(ctx, ConfigNameAccountAccess)
	if svcErr != nil {
		s.logger.Warn(ctx, "Failed to read the accountAccess server config; using the deployment policy",
			log.String("code", svcErr.Code))
		return s.base
	}
	section, ok := value.(AccountAccessValue)
	if !ok {
		s.logger.Warn(ctx, "Unexpected accountAccess server config type; using the deployment policy")
		return s.base
	}
	return section
}

// install sets the reader.
func (s *AccountAccessSource) install(reader ConfigReader) {
	s.readerMu.Lock()
	s.reader = reader
	s.readerMu.Unlock()
}

// InstallConfigReader installs the server-config reader on the active source. Until it is called,
// the deployment configuration applies.
func InstallConfigReader(reader ConfigReader) {
	source := activeSource.Load()
	if source == nil || reader == nil {
		return
	}
	source.install(reader)
}
