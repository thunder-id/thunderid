// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package revocationcache maintains the Resource Server's in-memory snapshot of revoked tokens and
// answers revocation checks for the RS enforcement point. It is read-only: it syncs the deny list
// from a pluggable source on a configurable interval and never writes to it. The single-token
// revocation write path lives in internal/oauth/oauth2/revocation and must not be imported here.
package revocationcache

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// The cfg.Source values: the operation-database sync source, and revocation events pushed by the
// trusted issuer.
const (
	sourceDB     = "db"
	sourceEvents = "events"
)

// defaultSyncInterval is used when cfg.SyncInterval is not a positive duration.
const defaultSyncInterval = time.Minute

// Initialize builds the RS revocation enforcer and its background syncer from cfg. When disabled it
// returns no-op implementations. For the "events" source it mounts the push endpoint on mux, verifying
// events with jwtService, and returns a no-op syncer: the deny list starts empty and only pushed events
// fill it. Otherwise it selects the sync source from cfg.Source, performs a synchronous initial load of
// the deny-list snapshot, and returns the enforcer (consulted on the request hot path) together with
// the syncer (whose lifecycle the caller owns). An unsupported source returns a non-nil error; a
// failed initial load does not, so a transient source outage never stops startup: enforcement begins
// with an empty deny list that the syncer repopulates on its next tick.
func Initialize(cfg Config, mux *http.ServeMux, jwtService jwt.JWTServiceInterface) (EnforcerInterface, Syncer,
	error) {
	if !cfg.Enabled {
		return noopEnforcer{}, noopSyncer{}, nil
	}
	if cfg.Source == sourceEvents {
		return initializeEvents(cfg, mux, jwtService)
	}

	source, err := selectSource(cfg.Source)
	if err != nil {
		return nil, nil, err
	}

	enforcer, syncer := initializeWithSource(cfg, source)
	return enforcer, syncer, nil
}

// initializeEvents builds the enforcer over an empty cache that the push endpoint fills. It needs the
// trusted issuer, since that is the only party whose events are accepted.
func initializeEvents(cfg Config, mux *http.ServeMux, jwtService jwt.JWTServiceInterface) (EnforcerInterface,
	Syncer, error) {
	if !cfg.TrustedIssuer.isConfigured() {
		return nil, nil, errors.New("the events revocation source requires server.security.trusted_issuer " +
			"with issuer, jwks_url and audience")
	}
	cache := newRevokedCache()
	receiver := newEventReceiver(cfg.TrustedIssuer, jwtService, cache)
	mux.HandleFunc("POST "+EventsPath, receiver.handlePush)
	return newEnforcer(cache), noopSyncer{}, nil
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
