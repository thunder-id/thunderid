// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"context"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// EnforcementServiceInterface enforces the revocation deny lists on the AS hot path (introspection,
// refresh grant, token exchange) under a fail-closed policy: when a deny list cannot be consulted,
// tokens are rejected rather than allowed.
type EnforcementServiceInterface interface {
	// EnsureNotRevoked rejects an artifact when its JTI or any supplied criterion is revoked.
	//
	// It returns a *RotatedTokenError for a refresh token revoked by rotation, carrying the instant
	// of the rotation. That error wraps ErrTokenRevoked, so callers testing errors.Is(err,
	// ErrTokenRevoked) reject it like any other revoked token. Only the refresh_token grant inspects
	// it, to decide whether the presentation falls inside its application's grace window.
	EnsureNotRevoked(ctx context.Context, identity RevocationIdentity) error
}

// enforcement service is the default EnforcementServiceInterface. It consults the runtime persistent DB behind a
// circuit breaker and alerts (via an observability event) when the breaker trips.
type enforcementService struct {
	store            revocationStoreInterface
	breaker          *circuitBreaker
	observabilitySvc providers.ObservabilityProvider
	logger           *log.Logger
}

// newEnforcementService creates a deny-list enforcement service backed by the runtime persistent DB, guarded
// by a circuit breaker and the fail-closed policy. It consults both the single-token deny list
// (by jti) and the criteria deny list (by token family id). It is unexported and constructed once via
// Initialize so the shared enforcement instance — and its circuit breaker — cannot be duplicated by
// external callers.
func newEnforcementService(observabilitySvc providers.ObservabilityProvider,
	store revocationStoreInterface) EnforcementServiceInterface {
	return &enforcementService{
		store:            store,
		breaker:          newCircuitBreaker(enforcementFailureThreshold, enforcementOpenDuration),
		observabilitySvc: observabilitySvc,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EnforcementService")),
	}
}

// EnsureNotRevoked checks the criteria deny list (every criterion in one query) and the single-token
// deny list (by jti), applying the circuit breaker and the fail-closed policy.
func (c *enforcementService) EnsureNotRevoked(ctx context.Context, identity RevocationIdentity) error {
	criteria := populatedCriteria(identity.Criteria)
	if identity.JTI == "" && len(criteria) == 0 {
		return nil
	}

	if !c.breaker.allow() {
		c.logger.Debug(ctx, "Runtime-persistent DB circuit is open; failing closed for revocation check")
		return ErrEnforcementUnavailable
	}

	// The criteria deny list is consulted first so that a criteria-based denial, notably a revoked
	// token family, can never be softened into a graced one by the refresh grant. Family revocation
	// is the response to a detected compromise, and a compromise response must outrank a
	// concurrency accommodation.
	if len(criteria) > 0 {
		revoked, err := c.store.areCriteriaRevoked(ctx, criteria, identity.EstablishedAt)
		if err != nil {
			return c.failClosed(ctx, err)
		}
		if revoked {
			c.breaker.recordSuccess()
			return ErrTokenRevoked
		}
	}

	if identity.JTI != "" {
		entry, err := c.store.IsTokenRevoked(ctx, identity.JTI)
		if err != nil {
			return c.failClosed(ctx, err)
		}
		if entry != nil {
			c.breaker.recordSuccess()
			return rotationDenial(entry)
		}
	}

	c.breaker.recordSuccess()
	return nil
}

// rotationDenial reports the denial for a deny-list entry, distinguishing a token rotated out of
// use from one revoked explicitly.
//
// A rotated token carries the instant of its rotation so the refresh grant can decide whether the
// presentation falls inside its application's grace window. Every other reason denies outright, so
// a client cannot obtain a window by explicitly revoking its own refresh token and presenting it
// again. An unreadable REVOKED_AT arrives here as the zero time, which no window contains, so a
// data fault denies.
func rotationDenial(entry *revokedTokenEntry) error {
	if entry.Reason != RevocationReasonRefreshRotation || entry.RevokedAt.IsZero() {
		return ErrTokenRevoked
	}
	return &RotatedTokenError{RotatedAt: entry.RevokedAt}
}

// populatedCriteria drops criteria the artifact did not carry, so an absent dimension never widens
// the deny-list query.
func populatedCriteria(criteria []Criterion) []Criterion {
	populated := make([]Criterion, 0, len(criteria))
	for _, criterion := range criteria {
		if criterion.Value != "" {
			populated = append(populated, criterion)
		}
	}
	return populated
}

// failClosed records a deny-list lookup failure against the circuit breaker (alerting when it trips)
// and returns ErrEnforcementUnavailable so the caller rejects the token.
func (c *enforcementService) failClosed(ctx context.Context, cause error) error {
	c.logger.Error(ctx, "Failed to consult token revocation deny list; failing closed",
		log.Error(cause))
	if c.breaker.recordFailure() {
		c.publishRuntimePersistentDBUnavailableEvent(ctx, cause)
	}
	return ErrEnforcementUnavailable
}

// publishRuntimePersistentDBUnavailableEvent emits an alert event when the runtime-persistent-DB circuit trips.
func (c *enforcementService) publishRuntimePersistentDBUnavailableEvent(ctx context.Context, cause error) {
	if c.observabilitySvc == nil || !c.observabilitySvc.IsEnabled() {
		return
	}

	evt := event.NewEvent(
		syscontext.GetTraceID(ctx),
		string(event.EventTypeRuntimePersistentDBUnavailable),
		event.ComponentAuthHandler,
	).
		WithStatus(providers.StatusFailure).
		WithData(event.DataKey.Error, cause.Error())

	c.observabilitySvc.PublishEvent(ctx, evt)
}
