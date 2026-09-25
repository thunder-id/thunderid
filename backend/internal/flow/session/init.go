// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/eventlistener"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// Initialize builds the SSO session Service and the hook that installs its termination listener.
// Store construction stays inside this package: callers receive only the Service and never hold a
// store. Timeouts fall back per field to the built-in
// defaults so an unset (zero) value never makes sessions expire immediately.
func Initialize(dbProvider provider.DBProviderInterface, deploymentID string,
	timeouts Timeouts, criteriaRevoker CriteriaRevoker) (Service, TerminationHook, error) {
	transactioner, err := dbProvider.GetRuntimePersistentDBTransactioner()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"failed to get runtime persistent DB transactioner for the SSO session service: %w", err)
	}

	def := DefaultTimeouts()
	if timeouts.Idle <= 0 {
		timeouts.Idle = def.Idle
	}
	if timeouts.Absolute <= 0 {
		timeouts.Absolute = def.Absolute
	}
	if timeouts.ActivityRefresh <= 0 {
		timeouts.ActivityRefresh = def.ActivityRefresh
	}

	store := newStore(dbProvider, deploymentID)
	svc := &service{
		store:           store,
		resolver:        newResolver(store),
		transactioner:   transactioner,
		criteriaRevoker: criteriaRevoker,
		terminated:      eventlistener.NewTopic[TerminatedSession]("session.terminated"),
		timeouts:        timeouts,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SSOSessionService")),
	}
	return svc, svc.terminated.Hook(), nil
}
