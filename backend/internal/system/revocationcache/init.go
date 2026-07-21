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

// Package revocationcache maintains the Resource Server's in-memory snapshot of revoked tokens and
// answers revocation checks for the RS enforcement point. It is read-only: it syncs the deny list
// from a pluggable source on a configurable interval and never writes to it. The single-token
// revocation write path lives in internal/oauth/oauth2/revocation and must not be imported here.
package revocationcache

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// sourceDB is the cfg.Source value selecting the operation-database sync source (the only one today).
const sourceDB = "db"

// defaultSyncInterval is used when cfg.SyncInterval is not a positive duration.
const defaultSyncInterval = time.Minute

// Initialize builds the RS revocation enforcer and its background syncer from cfg. When disabled it
// returns no-op implementations. Otherwise it selects the sync source from cfg.Source, performs a
// synchronous initial load of the deny-list snapshot, and returns the enforcer (consulted on the
// request hot path) together with the syncer (whose lifecycle the caller owns). An unsupported source
// returns a non-nil error; a failed initial load does not, so a transient source outage never stops
// startup: enforcement begins with an empty deny list that the syncer repopulates on its next tick.
func Initialize(cfg Config) (EnforcerInterface, Syncer, error) {
	if !cfg.Enabled {
		return noopEnforcer{}, noopSyncer{}, nil
	}

	source, err := selectSource(cfg.Source)
	if err != nil {
		return nil, nil, err
	}

	enforcer, syncer := initializeWithSource(cfg, source)
	return enforcer, syncer, nil
}

// selectSource resolves cfg.Source to a syncSource. An empty value defaults to the database source.
func selectSource(source string) (syncSource, error) {
	switch source {
	case "", sourceDB:
		return newDBSource(), nil
	default:
		return nil, fmt.Errorf("%w: %q", errUnsupportedSource, source)
	}
}

// initializeWithSource wires the cache, enforcer, and syncer against the given source and performs a
// best-effort synchronous initial load. It is the seam the tests drive with a fake source. A failed
// initial load is logged and does not stop startup: the deny list starts empty and the syncer
// repopulates it on its next tick.
func initializeWithSource(cfg Config, source syncSource) (EnforcerInterface, Syncer) {
	interval := cfg.SyncInterval
	if interval <= 0 {
		interval = defaultSyncInterval
	}

	cache := newRevokedCache()
	s := newSyncer(source, cache, interval)
	if err := s.refresh(context.Background()); err != nil {
		s.logger.Error(context.Background(),
			"Failed to load initial revoked token snapshot; starting with an empty deny list and "+
				"retrying on the next sync", log.Error(err))
	}

	return newEnforcer(cache), s
}
