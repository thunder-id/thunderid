// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// GatewayState says whether a gateway can take what this control plane sends it.
type GatewayState string

const (
	// StatePending is the default gateway's state while there is none: no data plane is registered.
	StatePending GatewayState = "pending"
	// StateProvisioning is a gateway that has never answered its health check, as one still being
	// provisioned does not.
	StateProvisioning GatewayState = "provisioning"
	// StateReady is a gateway that answered its last health check.
	StateReady GatewayState = "ready"
	// StateUnreachable is a gateway that answered a health check before, but not its last one.
	StateUnreachable GatewayState = "unreachable"
)

// GatewayStatus is a gateway's state, with when it was last checked and when it first answered.
type GatewayStatus struct {
	State         GatewayState `json:"state"`
	ProvisionedAt *time.Time   `json:"provisionedAt,omitempty"`
	CheckedAt     *time.Time   `json:"checkedAt,omitempty"`
}

// A gateway is checked again once its last check is older than the interval for its state: often
// while it is not ready, so a data plane coming up is seen soon, and seldom once it is.
const (
	notReadyCheckInterval = 15 * time.Second
	readyCheckInterval    = 90 * time.Second
	// healthTimeout bounds one health check. A check made while a request waits on it is kept short.
	healthTimeout = 5 * time.Second
)

// healthProber calls a gateway's health check.
type healthProber interface {
	Health(ctx context.Context, gw *Gateway) error
}

// readiness tells whether a gateway is ready, from health checks made on demand.
//
// A gateway is checked when something needs its state and its last check is older than the interval
// for that state, never on a timer: a deployment nobody uses is never checked, and no node needs to
// list every deployment's gateways. A gateway is also checked as soon as it is registered or
// changed. What a check finds is stored, so every control plane node, and every request, sees it.
//
// A state that is out of date never refuses anything. Stored as not ready, it is checked again
// before the answer is given, so the first request after a data plane came up is not refused for
// it. Stored as ready, it is used as it is and checked again in the background: once a gateway was
// provisioned, being unreachable refuses nothing anyway.
type readiness struct {
	gateways storeInterface
	statuses statusStoreInterface
	prober   healthProber
	logger   *log.Logger
	now      func() time.Time

	mu sync.Mutex
	// checking holds a channel per gateway whose check is running, closed once it is recorded, so a
	// gateway is checked once at a time on this node however many requests need it.
	checking map[string]chan struct{}
}

func newReadiness(gateways storeInterface, statuses statusStoreInterface, prober healthProber) *readiness {
	return &readiness{
		gateways: gateways,
		statuses: statuses,
		prober:   prober,
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayReadiness")),
		now:      time.Now,
		checking: map[string]chan struct{}{},
	}
}

// Status returns a gateway's state. A gateway stored as not ready, or never checked, whose check is
// out of date is checked before the answer is given.
func (r *readiness) Status(ctx context.Context, gw *Gateway) (*GatewayStatus, error) {
	stored, err := r.statuses.Get(ctx, gw.ID)
	if err != nil {
		return nil, err
	}
	if r.stale(stored) {
		if stateOf(stored) != StateReady {
			r.check(ctx, gw)
			if stored, err = r.statuses.Get(ctx, gw.ID); err != nil {
				return nil, err
			}
		} else {
			r.Refresh(ctx, gw)
		}
	}
	return statusOf(stored), nil
}

// Peek returns a gateway's state as stored, checking it again in the background when that is out
// of date. It is for showing many gateways at once, where waiting on each check would be too slow.
func (r *readiness) Peek(ctx context.Context, gw *Gateway) (*GatewayStatus, error) {
	stored, err := r.statuses.Get(ctx, gw.ID)
	if err != nil {
		return nil, err
	}
	if r.stale(stored) {
		r.Refresh(ctx, gw)
	}
	return statusOf(stored), nil
}

// DefaultStatus returns the default gateway's state, pending when no gateway is the default.
func (r *readiness) DefaultStatus(ctx context.Context) (*GatewayStatus, *tidcommon.ServiceError) {
	gw, err := r.defaultGateway(ctx)
	if err != nil {
		r.logger.Error(ctx, "Failed to find the default gateway", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if gw == nil {
		return &GatewayStatus{State: StatePending}, nil
	}
	status, err := r.Status(ctx, gw)
	if err != nil {
		r.logger.Error(ctx, "Failed to read the default gateway's status", log.String("gatewayId", gw.ID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return status, nil
}

// CheckDataPlaneReady refuses, until the default gateway has been provisioned, a write whose values
// have to reach it: with no default gateway, or one that has never answered its health check.
func (r *readiness) CheckDataPlaneReady(ctx context.Context) *tidcommon.ServiceError {
	status, svcErr := r.DefaultStatus(ctx)
	if svcErr != nil {
		return svcErr
	}
	switch status.State {
	case StatePending:
		return &declarativeresource.ErrorNoDataPlane
	case StateProvisioning:
		return &declarativeresource.ErrorDataPlaneNotReady
	default:
		return nil
	}
}

// RequireProvisioned refuses a gateway that has never answered its health check: a deploy to it, or
// a call to its store, could only fail.
func (r *readiness) RequireProvisioned(ctx context.Context, gw *Gateway) *tidcommon.ServiceError {
	status, err := r.Status(ctx, gw)
	if err != nil {
		r.logger.Error(ctx, "Failed to read the gateway's status", log.String("gatewayId", gw.ID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if status.State == StateProvisioning {
		return &declarativeresource.ErrorDataPlaneNotReady
	}
	return nil
}

// Refresh checks a gateway in the background, as one just registered or changed is.
func (r *readiness) Refresh(ctx context.Context, gw *Gateway) {
	// Detached, so the check outlives the request that asked for it, and keeps its deployment.
	detached := context.WithoutCancel(ctx)
	go r.check(detached, gw)
}

// Forget drops a removed gateway's status.
func (r *readiness) Forget(ctx context.Context, gatewayID string) {
	if err := r.statuses.Delete(ctx, gatewayID); err != nil {
		r.logger.Warn(ctx, "Failed to forget a removed gateway's status", log.String("gatewayId", gatewayID),
			log.Error(err))
	}
}

// check calls a gateway's health check and records what it found. A check already running for the
// gateway on this node is waited for instead of started again.
func (r *readiness) check(ctx context.Context, gw *Gateway) {
	key := deployment.Resolve(ctx) + "/" + gw.ID
	r.mu.Lock()
	if running, ok := r.checking[key]; ok {
		r.mu.Unlock()
		select {
		case <-running:
		case <-ctx.Done():
		}
		return
	}
	done := make(chan struct{})
	r.checking[key] = done
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.checking, key)
		r.mu.Unlock()
		close(done)
	}()

	before, _ := r.statuses.Get(ctx, gw.ID)
	checkCtx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	healthErr := r.prober.Health(checkCtx, gw)
	if err := r.statuses.Record(ctx, gw.ID, healthErr == nil); err != nil {
		r.logger.Error(ctx, "Failed to record a gateway's health check", log.String("gatewayId", gw.ID),
			log.Error(err))
		return
	}
	after, err := r.statuses.Get(ctx, gw.ID)
	if err != nil || stateOf(before) == stateOf(after) {
		return
	}
	fields := []log.Field{log.String("gatewayId", gw.ID), log.String("state", string(stateOf(after)))}
	if healthErr != nil {
		fields = append(fields, log.Error(healthErr))
	}
	r.logger.Info(ctx, "A gateway's state changed", fields...)
}

// stale reports whether a gateway's last check is older than the interval for its state.
func (r *readiness) stale(stored *checkedStatus) bool {
	if stored == nil {
		return true
	}
	interval := notReadyCheckInterval
	if stateOf(stored) == StateReady {
		interval = readyCheckInterval
	}
	return r.now().Sub(stored.CheckedAt) >= interval
}

func (r *readiness) defaultGateway(ctx context.Context) (*Gateway, error) {
	gateways, err := r.gateways.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range gateways {
		if gateways[i].IsDefault {
			return &gateways[i], nil
		}
	}
	return nil, nil
}

// stateOf turns a gateway's last check into its state. One never checked, or never answering, is
// still being provisioned.
func stateOf(stored *checkedStatus) GatewayState {
	switch {
	case stored == nil || stored.ProvisionedAt == nil:
		return StateProvisioning
	case stored.Reachable:
		return StateReady
	default:
		return StateUnreachable
	}
}

func statusOf(stored *checkedStatus) *GatewayStatus {
	status := &GatewayStatus{State: stateOf(stored)}
	if stored != nil {
		checked := stored.CheckedAt
		status.CheckedAt = &checked
		status.ProvisionedAt = stored.ProvisionedAt
	}
	return status
}

// Health calls the gateway's readiness check, which is public: no key is presented. Anything but 200
// is not ready.
func (c *gatewayClient) Health(ctx context.Context, gw *Gateway) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, gw.BaseURL+"/health/readiness", nil)
	if err != nil {
		return fmt.Errorf("failed to build the health check: %w", err)
	}
	client, err := httpClientFor(gw, healthTimeout)
	if err != nil {
		return err
	}
	defer releaseConnections(client)
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to reach the gateway: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the gateway is not ready: status %d", resp.StatusCode)
	}
	return nil
}
