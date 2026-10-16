// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// endedSessionTTL bounds how long an ended session is remembered. It only has to outlive the access
// tokens issued in the session, because ending the session also revokes its refresh tokens.
const endedSessionTTL = 24 * 60 * 60

// endedSessions records the SSO sessions that ended, so the BFF can sign out a browser whose cookie
// still names one. The back-channel logout endpoint feeds it.
type endedSessions struct {
	store  providers.RuntimeStoreProvider
	logger *log.Logger

	// forget removes the sessions held for an ended SSO session straight away. Only a session store
	// that keeps sessions on the server sets it.
	forget func(ctx context.Context, sessionID string) error
}

func newEndedSessions(store providers.RuntimeStoreProvider) *endedSessions {
	return &endedSessions{
		store:  store,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "BFF")),
	}
}

// end records the SSO session as ended and removes the sessions held for it. The record covers
// sessions held in cookies and requests already in flight.
func (e *endedSessions) end(ctx context.Context, sessionID string) error {
	if err := e.store.Put(ctx, providers.NamespaceConsoleEnded, sessionID, []byte("1"),
		endedSessionTTL); err != nil {
		return err
	}
	if e.forget != nil {
		return e.forget(ctx, sessionID)
	}
	return nil
}

// isEnded reports whether the session has ended. A store failure is logged and treated as not
// ended: the revoked tokens are still refused by the authorization server and the API gate.
func (e *endedSessions) isEnded(ctx context.Context, sessionID string) bool {
	if sessionID == "" {
		return false
	}
	value, err := e.store.Get(ctx, providers.NamespaceConsoleEnded, sessionID)
	if err != nil {
		if !errors.Is(err, providers.ErrRuntimeStoreKeyNotFound) {
			e.logger.Warn(ctx, "Failed to look up an ended session", log.Error(err))
		}
		return false
	}
	return value != nil
}
