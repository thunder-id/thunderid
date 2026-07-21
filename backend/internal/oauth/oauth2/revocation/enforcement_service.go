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

// EnforcementServiceInterface enforces the single-token deny list on the AS hot path (introspection, refresh
// grant, token exchange) under a fail-closed policy: when the deny list cannot be consulted, tokens
// are rejected rather than allowed.
type EnforcementServiceInterface interface {
	// EnsureNotRevoked returns nil when the token identified by jti may proceed. It returns
	// ErrTokenRevoked when the jti is on the deny list, and ErrEnforcementUnavailable when the
	// deny list cannot be consulted (fail-closed). An empty jti is a no-op (nothing to enforce).
	EnsureNotRevoked(ctx context.Context, jti string) error
}

// enforcement service is the default EnforcementServiceInterface. It consults the runtime persistent DB behind a
// circuit breaker and alerts (via an observability event) when the breaker trips.
type enforcementService struct {
	store            RevokedTokenStoreInterface
	breaker          *circuitBreaker
	observabilitySvc providers.ObservabilityProvider
	logger           *log.Logger
}

// newEnforcementService creates a deny-list enforcement service backed by the runtime persistent DB, guarded
// by a circuit breaker and the fail-closed policy. It is unexported and constructed once via
// Initialize so the shared enforcement instance — and its circuit breaker — cannot be duplicated by
// external callers.
func newEnforcementService(observabilitySvc providers.ObservabilityProvider) EnforcementServiceInterface {
	return &enforcementService{
		store:            newRevokedTokenStore(),
		breaker:          newCircuitBreaker(enforcementFailureThreshold, enforcementOpenDuration),
		observabilitySvc: observabilitySvc,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EnforcementService")),
	}
}

// EnsureNotRevoked checks the deny list for the given jti, applying the circuit breaker and the
// fail-closed policy.
func (c *enforcementService) EnsureNotRevoked(ctx context.Context, jti string) error {
	if jti == "" {
		return nil
	}

	if !c.breaker.allow() {
		c.logger.Debug(ctx, "Runtime-persistent DB circuit is open; failing closed for revocation check")
		return ErrEnforcementUnavailable
	}

	revoked, err := c.store.IsTokenRevoked(ctx, jti)
	if err != nil {
		c.logger.Error(ctx, "Failed to consult token revocation deny list; failing closed",
			log.Error(err))
		if c.breaker.recordFailure() {
			c.publishRuntimePersistentDBUnavailableEvent(ctx, err)
		}
		return ErrEnforcementUnavailable
	}

	c.breaker.recordSuccess()
	if revoked {
		return ErrTokenRevoked
	}
	return nil
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
