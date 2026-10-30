// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/variablestore"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// captureTimeout bounds working out a resource's values, and each write of one to the gateway.
const captureTimeout = 10 * time.Second

// pushRetryDelay is how long a value whose write failed waits before it is written again, doubled
// for each failure in a row up to pushRetryDelayMax; pushAttempts is how many times it is tried.
const (
	pushRetryDelay    = 2 * time.Second
	pushRetryDelayMax = time.Minute
	pushAttempts      = 10
)

// placeholderValuer computes the values an export of one resource refers to by name, and the secrets
// the exports of every stored resource of a type refer to.
type placeholderValuer interface {
	PlaceholderValues(ctx context.Context, resourceType string, resource interface{}) (
		map[string]string, map[string]string, error)
	ReferencedSecretNames(ctx context.Context, resourceType string) (map[string]bool, error)
}

// ValueCapture writes the values a resource's export refers to into the default gateway's store,
// under the names the export writes.
//
// A control plane exports references rather than values, and the gateway a version is applied to
// fills each reference from its own store at import. Something has to put the value there. Whoever
// writes a resource is the only one holding all of it, a generated client secret above all, which
// is hashed on write and never read back. So the value is taken from the write, as it happens.
//
// Only the default gateway receives values. There is no telling which other gateway a value is meant
// for: another may hold its own value under the same name on purpose.
//
// It is built before the services that call it and bound once the export and the gateway store
// exist, because the export is built from those services' exporters. A capture before the binding
// does nothing. The binding happens during startup, before any request is served, so it is not
// guarded.
type ValueCapture struct {
	values   placeholderValuer
	gateways storeInterface
	store    StoreServiceInterface
	logger   *log.Logger

	// retryDelay and attempts are how a write that failed is tried again, pushRetryDelay and
	// pushAttempts unless a test shortens them.
	retryDelay time.Duration
	attempts   int

	mu      sync.Mutex
	pending map[string]*pendingValue
}

// pendingValue is one value on its way to a gateway's store: the latest value given for its name, or
// its removal, and the write in flight or waiting for it.
type pendingValue struct {
	gatewayID, collection, name string
	value                       string
	remove                      bool
	failures                    int
	timer                       *time.Timer
	running                     bool
	// again records a newer value given while a write ran, to be written once it returns.
	again bool
}

// NewValueCapture returns a capture that does nothing until Initialize binds it.
func NewValueCapture() *ValueCapture {
	return &ValueCapture{
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayValueCapture")),
		retryDelay: pushRetryDelay,
		attempts:   pushAttempts,
		pending:    map[string]*pendingValue{},
	}
}

func (c *ValueCapture) bind(values placeholderValuer, gateways storeInterface, store StoreServiceInterface) {
	c.values = values
	c.gateways = gateways
	c.store = store
}

// CaptureValues writes the values the resource's export would refer to into the default gateway's
// store, variables and secrets each to its own collection.
//
// It never fails the caller. The values are worked out, and the default gateway found, as the
// resource is written, since the write is what holds them; the writes to the gateway then follow in
// the background, so a gateway that is slow or down does not slow the caller. A write that fails is
// tried again, with a delay that grows with each failure, until it succeeds or the attempts run out;
// a newer value for the same name replaces one still on its way. A value that never arrives is
// logged by name, never by value, and the apply that later needs it reports it missing, which is
// where it can be set by hand.
func (c *ValueCapture) CaptureValues(ctx context.Context, resourceType string, resource interface{}) {
	if c == nil || c.values == nil {
		return
	}
	// Detached and bounded, so working out the values and finding the gateway are as safe from a
	// client that hangs up as the writes are.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), captureTimeout)
	defer cancel()

	variables, secrets, err := c.values.PlaceholderValues(ctx, resourceType, resource)
	if err != nil {
		c.logger.Warn(ctx, "Failed to work out the values a resource refers to, so none were captured",
			log.String("resourceType", resourceType), log.Error(err))
		return
	}
	if len(variables) == 0 && len(secrets) == 0 {
		return
	}

	gw, err := c.defaultGateway(ctx)
	if err != nil {
		c.logger.Warn(ctx, "Failed to find the default gateway, so no values were captured",
			log.String("resourceType", resourceType), log.Error(err))
		return
	}
	if gw == nil {
		c.logger.Debug(ctx, "No gateway is the default, so no values were captured",
			log.String("resourceType", resourceType))
		return
	}

	c.enqueue(ctx, gw.ID, collectionVariables, variables)
	c.enqueue(ctx, gw.ID, collectionSecrets, secrets)
}

// ReleaseSecrets removes from the default gateway's store each secret the resource's export referred
// to that the export of no stored resource of the type still refers to.
//
// A secret's name is derived from the resource's name, so another resource of the type can refer to the
// very copy this one did. A name is therefore kept whenever anything stored still
// refers to it, and when that cannot be worked out nothing is removed: a copy left behind is replaced
// by the next capture under its name, while one removed in error breaks whatever refers to it.
//
// Like a capture, it never fails the caller, and each removal is tried again as a write is.
func (c *ValueCapture) ReleaseSecrets(ctx context.Context, resourceType string, resource interface{}) {
	if c == nil || c.values == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), captureTimeout)
	defer cancel()

	_, secrets, err := c.values.PlaceholderValues(ctx, resourceType, resource)
	if err != nil {
		c.logger.Warn(ctx, "Failed to work out the secrets a resource referred to, so none were released",
			log.String("resourceType", resourceType), log.Error(err))
		return
	}
	if len(secrets) == 0 {
		return
	}

	gw, err := c.defaultGateway(ctx)
	if err != nil {
		c.logger.Warn(ctx, "Failed to find the default gateway, so no secrets were released",
			log.String("resourceType", resourceType), log.Error(err))
		return
	}
	if gw == nil {
		return
	}

	// Held from before the stored resources are read until the removals are queued, so a resource
	// written meanwhile, which the read may miss, queues its capture after them and wins.
	c.mu.Lock()
	defer c.mu.Unlock()
	referenced, err := c.values.ReferencedSecretNames(ctx, resourceType)
	if err != nil {
		c.logger.Warn(ctx, "Failed to work out the secrets stored resources refer to, so none were released",
			log.String("resourceType", resourceType), log.Error(err))
		return
	}
	for _, name := range slices.Sorted(maps.Keys(secrets)) {
		if !referenced[name] {
			c.queue(ctx, gw.ID, collectionSecrets, name, "", true)
		}
	}
}

// enqueue puts each value in a collection on its way to the gateway's store, in the order of their
// names. A value already on its way under the same name is replaced by the newer one.
func (c *ValueCapture) enqueue(ctx context.Context, gatewayID, collection string, values map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, name := range slices.Sorted(maps.Keys(values)) {
		c.queue(ctx, gatewayID, collection, name, values[name], false)
	}
}

// queue makes a value, or its removal, the latest write for its name, and starts it unless a write for
// the name is in flight, which this one follows once it returns. The caller holds c.mu.
func (c *ValueCapture) queue(ctx context.Context, gatewayID, collection, name, value string, remove bool) {
	key := gatewayID + "/" + collection + "/" + name
	p := c.pending[key]
	if p == nil {
		p = &pendingValue{gatewayID: gatewayID, collection: collection, name: name}
		c.pending[key] = p
	}
	p.value, p.remove = value, remove
	p.failures = 0
	if p.running {
		p.again = true
		return
	}
	if p.timer != nil {
		p.timer.Stop()
	}
	p.timer = time.AfterFunc(0, func() { c.push(ctx, key) })
}

// push writes one pending value to the gateway's store, and keeps it pending to be tried again when
// the write fails or a newer value arrived meanwhile.
func (c *ValueCapture) push(ctx context.Context, key string) {
	c.mu.Lock()
	p := c.pending[key]
	// A write already in flight carries the latest value, or is followed by one that does.
	if p == nil || p.running {
		c.mu.Unlock()
		return
	}
	p.timer = nil
	p.running = true
	value, remove := p.value, p.remove
	c.mu.Unlock()

	// The write is the capture's own work, on a context of its own, bounded like the rest.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), captureTimeout)
	var refusal *StoreRefusal
	var svcErr *tidcommon.ServiceError
	if remove {
		refusal, svcErr = c.deleteSecret(ctx, p.gatewayID, p.name)
	} else {
		refusal, svcErr = c.set(ctx, p.gatewayID, p.collection, p.name, value)
	}
	cancel()

	c.mu.Lock()
	defer c.mu.Unlock()
	p.running = false
	switch {
	case p.again:
		// A newer value arrived while this one was written; it goes now, whatever became of this one.
		p.again, p.failures = false, 0
		p.timer = time.AfterFunc(0, func() { c.push(ctx, key) })
	case svcErr != nil && p.failures+1 < c.attempts:
		p.failures++
		c.logger.Warn(ctx, "Failed to capture a value in the default gateway's store; it will be tried again",
			log.String("gatewayId", p.gatewayID), log.String("collection", p.collection),
			log.String("name", p.name), log.Bool("removal", remove), log.Int("failures", p.failures),
			log.String("error", svcErr.Error.DefaultValue))
		p.timer = time.AfterFunc(c.pushDelay(p.failures), func() { c.push(ctx, key) })
	case svcErr != nil:
		c.logger.Error(ctx, "Gave up capturing a value in the default gateway's store",
			log.String("gatewayId", p.gatewayID), log.String("collection", p.collection),
			log.String("name", p.name), log.Bool("removal", remove), log.Int("failures", p.failures+1),
			log.String("error", svcErr.Error.DefaultValue))
		delete(c.pending, key)
	case refusal != nil:
		c.logger.Warn(ctx, "The default gateway's store refused a captured value",
			log.String("gatewayId", p.gatewayID), log.String("collection", p.collection),
			log.String("name", p.name), log.Bool("removal", remove), log.Int("status", refusal.Status))
		delete(c.pending, key)
	default:
		delete(c.pending, key)
	}
}

// pushDelay is how long the next write waits after the given failures in a row: the usual delay,
// doubled for each failure after the first, up to pushRetryDelayMax.
func (c *ValueCapture) pushDelay(failures int) time.Duration {
	delay := c.retryDelay
	for i := 1; i < failures && delay < pushRetryDelayMax; i++ {
		delay *= 2
	}
	return min(delay, pushRetryDelayMax)
}

// set writes one value to a collection of the gateway's store. A value is set rather than created:
// it is set whether or not the name is held, so a resource written again, or a secret regenerated,
// replaces what the gateway held under that name. The call goes through the same store service an
// administrator's write to the store does, so the key is presented the same way.
func (c *ValueCapture) set(ctx context.Context, gatewayID, collection, name, value string) (*StoreRefusal,
	*tidcommon.ServiceError) {
	if collection == collectionSecrets {
		answer, svcErr := c.store.SetSecret(ctx, gatewayID, name, variablestore.SecretUpdateRequest{Value: value})
		if svcErr != nil {
			return nil, svcErr
		}
		return answer.Refusal, nil
	}
	answer, svcErr := c.store.SetVariable(ctx, gatewayID, name, variablestore.VariableUpdateRequest{Value: value})
	if svcErr != nil {
		return nil, svcErr
	}
	return answer.Refusal, nil
}

// deleteSecret removes one secret from the gateway's store, through the same store service set writes
// it with. Removing a name the store does not hold succeeds.
func (c *ValueCapture) deleteSecret(ctx context.Context, gatewayID, name string) (*StoreRefusal,
	*tidcommon.ServiceError) {
	answer, svcErr := c.store.DeleteSecret(ctx, gatewayID, name)
	if svcErr != nil {
		return nil, svcErr
	}
	return answer.Refusal, nil
}

// idle reports whether no value is on its way to a gateway.
func (c *ValueCapture) idle() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending) == 0
}

// defaultGateway returns the default gateway, or nil when none is.
func (c *ValueCapture) defaultGateway(ctx context.Context) (*Gateway, error) {
	gateways, err := c.gateways.List(ctx)
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
