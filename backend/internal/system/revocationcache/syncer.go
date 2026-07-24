/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package revocationcache

import (
	"context"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
)

const loggerComponentName = "RevocationCacheSyncer"

// Syncer owns the background refresh loop that keeps the cache in sync with the source. Its lifecycle
// is owned by the caller: Start begins periodic refresh after the initial load, and Stop halts it
// during graceful shutdown.
type Syncer interface {
	// Start begins the periodic refresh loop. It returns immediately; refresh runs in the background.
	Start(ctx context.Context)
	// Stop halts the refresh loop and releases its resources. It is safe to call once.
	Stop()
}

// syncer refreshes the cache from the source on a fixed interval. A failed refresh is logged and the
// previous snapshot is kept (last-known-good), so a transient source outage never empties the deny
// list and lets revoked tokens back in.
type syncer struct {
	source   syncSource
	cache    *revokedCache
	interval time.Duration
	logger   *log.Logger
	cancel   context.CancelFunc
	doneCh   chan struct{}
	stopOnce sync.Once
}

// newSyncer creates a syncer for the given source, cache, and refresh interval.
func newSyncer(source syncSource, cache *revokedCache, interval time.Duration) *syncer {
	return &syncer{
		source:   source,
		cache:    cache,
		interval: interval,
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
		doneCh:   make(chan struct{}),
	}
}

// refresh reads a fresh snapshot from the source and atomically replaces the cache. On error the
// cache is left untouched so the last-known-good snapshot continues to serve lookups.
func (s *syncer) refresh(ctx context.Context) error {
	snapshot, err := s.source.Snapshot(ctx)
	if err != nil {
		return err
	}
	s.cache.replace(snapshot)
	return nil
}

// Start launches the periodic refresh loop. The initial snapshot is loaded synchronously by
// Initialize before Start is called, so the loop only handles subsequent refreshes. It derives a
// cancelable context so Stop can abort an in-flight refresh rather than block until it returns.
func (s *syncer) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	go func() {
		defer close(s.doneCh)
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.refresh(ctx); err != nil {
					s.logger.Error(ctx, "Failed to refresh revoked token cache; keeping last-known-good snapshot",
						log.Error(err))
				}
			}
		}
	}()
}

// Stop cancels the refresh loop's context — aborting any in-flight refresh — and waits for the loop
// to exit, so graceful shutdown cannot stall on a slow source read. It is safe to call more than once.
func (s *syncer) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		<-s.doneCh
	})
}

// noopSyncer is returned when RS revocation enforcement is disabled; its lifecycle methods do nothing.
type noopSyncer struct{}

// Start does nothing.
func (noopSyncer) Start(context.Context) {}

// Stop does nothing.
func (noopSyncer) Stop() {}
