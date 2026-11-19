// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// timeoutsSource supplies the session timeouts in effect.
type timeoutsSource interface {
	current(ctx context.Context) resolvedTimeouts
}

// sectionTimeouts reads the "session" section on every use, so a change made on any node applies
// without a restart. When the section cannot be read, the timeouts last read apply, so a failed read
// never lengthens a session; before any read succeeds, the defaults apply.
type sectionTimeouts struct {
	reader          serverconfig.ServerConfigService
	activityRefresh time.Duration
	logger          *log.Logger

	// mu guards the fields below. reads numbers each read in the order it started, and last holds the
	// timeouts of the latest started read that succeeded, numbered lastSeq, so a read that started
	// earlier and finished later can't replace them.
	mu      sync.Mutex
	reads   uint64
	lastSeq uint64
	last    *resolvedTimeouts
}

// newSectionTimeouts builds the source over the server-config service the section is read from and the
// deployment's activity refresh interval.
func newSectionTimeouts(reader serverconfig.ServerConfigService, activityRefresh time.Duration) timeoutsSource {
	return &sectionTimeouts{
		reader:          reader,
		activityRefresh: activityRefresh,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SessionTimeouts")),
	}
}

// current reads the effective "session" section and resolves its timeouts.
func (s *sectionTimeouts) current(ctx context.Context) resolvedTimeouts {
	s.mu.Lock()
	s.reads++
	seq := s.reads
	s.mu.Unlock()

	value, svcErr := s.reader.GetMergedConfig(ctx, string(serverconfig.ConfigNameSession))
	if svcErr != nil {
		s.logger.Warn(ctx, "Failed to read the session server config; using the timeouts last read",
			log.String("errorCode", svcErr.Code))
		return s.lastReadOrDefault()
	}
	cfg, ok := value.(Config)
	if !ok {
		s.logger.Warn(ctx, "Unexpected session server config type; using the timeouts last read")
		return s.lastReadOrDefault()
	}
	timeouts := newTimeouts(cfg, s.activityRefresh)

	s.mu.Lock()
	if seq > s.lastSeq {
		s.lastSeq = seq
		s.last = &timeouts
	}
	s.mu.Unlock()
	return timeouts
}

// lastReadOrDefault returns the timeouts last read from the section, or the defaults before any read
// succeeded.
func (s *sectionTimeouts) lastReadOrDefault() resolvedTimeouts {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last != nil {
		return *s.last
	}
	return newTimeouts(Config{}, s.activityRefresh)
}
