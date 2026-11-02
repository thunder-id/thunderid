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
	// EnsureNotRevoked rejects an artifact when its JTI (ErrTokenIDRevoked) or any supplied criterion
	// (ErrTokenRevoked) is revoked.
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

// EnsureNotRevoked checks the single-token deny list (by jti) and the criteria deny list (every
// criterion in one query), applying the circuit breaker and the fail-closed policy.
func (c *enforcementService) EnsureNotRevoked(ctx context.Context, identity RevocationIdentity) error {
	criteria := populatedCriteria(identity.Criteria)
	if identity.JTI == "" && len(criteria) == 0 {
		return nil
	}

	if !c.breaker.allow() {
		c.logger.Debug(ctx, "Runtime-persistent DB circuit is open; failing closed for revocation check")
		return ErrEnforcementUnavailable
	}

	if identity.JTI != "" {
		revoked, err := c.store.IsTokenRevoked(ctx, identity.JTI)
		if err != nil {
			return c.failClosed(ctx, err)
		}
		if revoked {
			c.breaker.recordSuccess()
			return ErrTokenIDRevoked
		}
	}

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

	c.breaker.recordSuccess()
	return nil
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
