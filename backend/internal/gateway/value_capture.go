// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"sync"
	"time"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
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

// placeholderValuer computes the values an export of one resource refers to by name.
type placeholderValuer interface {
	PlaceholderValues(ctx context.Context, resourceType string, resource interface{}) (
		map[string]string, map[string]string, error)
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
	// pipeline finds the gateway the values go to: the first gateway, which auto apply deploys to.
	pipeline pipeline
	logger   *log.Logger

	// retryDelay and attempts are how a write that failed is tried again, pushRetryDelay and
	// pushAttempts unless a test shortens them.
	retryDelay time.Duration
	attempts   int

	mu      sync.Mutex
	pending map[string]*pendingValue
	// fills counts the fills running, and capturedDuringFill the values a write captured meanwhile,
	// by key. A fill reads a value before it writes it, so one a write captured in between is newer
	// than the fill's, even once its own write has finished and it is no longer pending.
	fills              int
	capturedDuringFill map[string]bool
}

// pendingValue is one value on its way to a gateway's store: the latest value given for its name,
// and the write in flight or waiting for it.
type pendingValue struct {
	gatewayID, collection, name string
	value                       string
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

func (c *ValueCapture) bind(values placeholderValuer, gateways storeInterface, store StoreServiceInterface,
	p pipeline) {
	c.values = values
	c.gateways = gateways
	c.store = store
	c.pipeline = p
}

// CaptureValues writes the values the resource's export would refer to into the store of the gateway of
// the pipeline's first gateway, the one auto apply deploys to, which is the default gateway unless
// gateway.pipeline names another; variables and secrets each go to their own collection.
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

	gw, err := c.firstGateway(ctx)
	if err != nil {
		c.logger.Warn(ctx, "Failed to find the first gateway, so no values were captured",
			log.String("resourceType", resourceType), log.Error(err))
		return
	}
	if gw == nil {
		c.logger.Debug(ctx, "The pipeline has no first gateway, so no values were captured",
			log.String("resourceType", resourceType))
		return
	}

	c.enqueue(ctx, gw.ID, collectionVariables, variables, true)
	c.enqueue(ctx, gw.ID, collectionSecrets, secrets, true)
}

// enqueue puts each value in a collection on its way to the gateway's store, in the order of their
// names. A value a write captures, with replace set, replaces one already on its way under the same
// name. A value a fill puts, without it, is dropped when a value under that name is on its way or was
// captured since a fill started.
func (c *ValueCapture) enqueue(ctx context.Context, gatewayID, collection string, values map[string]string,
	replace bool) {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, name := range names {
		key := gatewayID + "/" + collection + "/" + name
		p := c.pending[key]
		if !replace && (p != nil || c.capturedDuringFill[key]) {
			continue
		}
		if replace && c.fills > 0 {
			c.capturedDuringFill[key] = true
		}
		if p == nil {
			p = &pendingValue{gatewayID: gatewayID, collection: collection, name: name}
			c.pending[key] = p
		}
		p.value = values[name]
		p.failures = 0
		if p.running {
			p.again = true
			continue
		}
		if p.timer != nil {
			p.timer.Stop()
		}
		p.timer = time.AfterFunc(0, func() { c.push(ctx, key) })
	}
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
	value := p.value
	c.mu.Unlock()

	// The write is the capture's own work, on a context of its own, bounded like the rest.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), captureTimeout)
	refusal, svcErr := c.set(ctx, p.gatewayID, p.collection, p.name, value)
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
			log.String("gatewayID", p.gatewayID), log.String("collection", p.collection),
			log.String("name", p.name), log.Int("failures", p.failures),
			log.String("error", svcErr.Error.DefaultValue))
		p.timer = time.AfterFunc(c.pushDelay(p.failures), func() { c.push(ctx, key) })
	case svcErr != nil:
		c.logger.Error(ctx, "Gave up capturing a value in the default gateway's store",
			log.String("gatewayID", p.gatewayID), log.String("collection", p.collection),
			log.String("name", p.name), log.Int("failures", p.failures+1),
			log.String("error", svcErr.Error.DefaultValue))
		delete(c.pending, key)
	case refusal != nil:
		c.logger.Warn(ctx, "The default gateway's store refused a captured value",
			log.String("gatewayID", p.gatewayID), log.String("collection", p.collection),
			log.String("name", p.name), log.Int("status", refusal.Status))
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

// idle reports whether no value is on its way to a gateway.
func (c *ValueCapture) idle() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending) == 0
}

// firstGateway returns the pipeline's first gateway, or nil when there is none.
func (c *ValueCapture) firstGateway(ctx context.Context) (*Gateway, error) {
	gateways, err := c.gateways.List(ctx)
	if err != nil {
		return nil, err
	}
	gw, ok := c.pipeline.first(ctx, gateways)
	if !ok {
		return nil, nil
	}
	return &gw, nil
}

// fillMissingVariables writes to the first gateway each variable the exporters' resources refer to
// that its store does not hold, with the value the resource holds here.
//
// A write captures only the values it makes, so a resource written before one of its fields was
// referred to by name has no value for that field on the gateway, and every apply that sends the
// resource is refused until one is set. This puts it there. A value the gateway holds, or one a write
// has already put on its way, is never replaced, so it is safe to run on every start, and a secret is
// never filled, since a read holds none. A gateway that does not answer is asked again the way a
// captured value is written; one that refuses is not.
//
// It is the system's own work, so it reads every resource whoever is signed in.
func (c *ValueCapture) fillMissingVariables(ctx context.Context, exporters []declarativeresource.ResourceExporter) {
	if c == nil || c.values == nil {
		return
	}
	ctx = security.WithRuntimeContext(ctx)
	c.mu.Lock()
	c.fills++
	if c.capturedDuringFill == nil {
		c.capturedDuringFill = map[string]bool{}
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.fills--; c.fills == 0 {
			c.capturedDuringFill = nil
		}
	}()
	for failures := 0; ; failures++ {
		err := c.fillMissingVariablesOnce(ctx, exporters)
		if err == nil {
			return
		}
		var refusal *StoreRefusal
		if errors.As(err, &refusal) || failures+1 >= c.attempts {
			c.logger.Warn(ctx, "Gave up filling the variables the first gateway lacks", log.Error(err))
			return
		}
		time.Sleep(c.pushDelay(failures + 1))
	}
}

// fillMissingVariablesOnce finds the first gateway, works out the variables the resources refer to
// now, and puts on its way each one the gateway does not hold. A resource that could not be read is
// reported once the rest are filled, so the next try reads it again.
func (c *ValueCapture) fillMissingVariablesOnce(ctx context.Context,
	exporters []declarativeresource.ResourceExporter) error {
	gw, err := c.firstGateway(ctx)
	if err != nil {
		return err
	}
	if gw == nil {
		c.logger.Debug(ctx, "The pipeline has no first gateway, so no variables were filled")
		return nil
	}
	wanted, readErr := c.referredVariables(ctx, exporters)
	if len(wanted) == 0 {
		return readErr
	}
	missing, err := c.unheldVariables(ctx, gw.ID, wanted)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		c.logger.Info(ctx, "Filling the variables the first gateway lacks", log.String("gatewayID", gw.ID),
			log.Int("variables", len(missing)))
		c.enqueue(ctx, gw.ID, collectionVariables, missing, false)
	}
	return readErr
}

// referredVariables returns, by name, the variables the exporters' resources refer to, with their
// values. An exporter whose rules name no variable is passed over. A resource that could not be read
// is left out, and the error returned beside the rest.
func (c *ValueCapture) referredVariables(ctx context.Context,
	exporters []declarativeresource.ResourceExporter) (map[string]string, error) {
	wanted := map[string]string{}
	var readErr error
	for _, exporter := range exporters {
		rules := exporter.GetResourceRules()
		if rules == nil || len(rules.Variables) == 0 && len(rules.ArrayVariables) == 0 {
			continue
		}
		resourceType := exporter.GetResourceType()
		ids, svcErr := exporter.GetAllResourceIDs(ctx)
		if svcErr != nil {
			readErr = fmt.Errorf("failed to list %s resources: %s", resourceType, svcErr.Code)
			continue
		}
		for _, id := range ids {
			resource, _, svcErr := exporter.GetResourceByID(ctx, id)
			if svcErr != nil {
				readErr = fmt.Errorf("failed to read a %s resource: %s", resourceType, svcErr.Code)
				continue
			}
			variables, _, err := c.values.PlaceholderValues(ctx, resourceType, resource)
			if err != nil {
				c.logger.Warn(ctx, "Failed to work out the values a resource refers to",
					log.String("resourceType", resourceType), log.Error(err))
				continue
			}
			maps.Copy(wanted, variables)
		}
	}
	return wanted, readErr
}

// unheldVariables returns those of the wanted variables the gateway's store does not hold.
func (c *ValueCapture) unheldVariables(ctx context.Context, gatewayID string, wanted map[string]string) (
	map[string]string, error) {
	names := slices.Sorted(maps.Keys(wanted))
	missing := maps.Clone(wanted)
	for start := 0; start < len(names); start += maxNamesPerLookup {
		answer, svcErr := c.store.ListVariables(ctx, gatewayID,
			lookupOf(names[start:min(start+maxNamesPerLookup, len(names))]))
		if svcErr != nil {
			return nil, fmt.Errorf("%s: %s", svcErr.Code, svcErr.Error.DefaultValue)
		}
		if answer.Refusal != nil {
			return nil, answer.Refusal
		}
		for _, variable := range answer.Value.Variables {
			delete(missing, variable.Name)
		}
	}
	return missing, nil
}
