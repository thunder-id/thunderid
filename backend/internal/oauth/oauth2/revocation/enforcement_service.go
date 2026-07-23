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
	// EnsureNotRevoked returns nil when the token may proceed. It returns ErrTokenRevoked when the
	// token's jti is on the single-token deny list or its token family id (tfid) is on the criteria
	// deny list, and ErrEnforcementUnavailable when a deny list cannot be consulted (fail-closed).
	// Empty jti and tokenFamilyID are each a no-op for their respective check.
	EnsureNotRevoked(ctx context.Context, jti, tokenFamilyID string) error
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

// EnsureNotRevoked checks the single-token deny list (by jti) and the criteria deny list (by token
// family id), applying the circuit breaker and the fail-closed policy.
func (c *enforcementService) EnsureNotRevoked(ctx context.Context, jti, tokenFamilyID string) error {
	if jti == "" && tokenFamilyID == "" {
		return nil
	}

	if !c.breaker.allow() {
		c.logger.Debug(ctx, "Runtime-persistent DB circuit is open; failing closed for revocation check")
		return ErrEnforcementUnavailable
	}

	if jti != "" {
		revoked, err := c.store.IsTokenRevoked(ctx, jti)
		if err != nil {
			return c.failClosed(ctx, err)
		}
		if revoked {
			c.breaker.recordSuccess()
			return ErrTokenRevoked
		}
	}

	if tokenFamilyID != "" {
		revoked, err := c.store.isCriterionRevoked(ctx, criterionTypeTokenFamily, tokenFamilyID)
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
