// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package eventbus publishes events to subscribers by category. An event carries one typed payload
// per category, and each subscriber receives only its own category's payload from a bounded queue.
package eventbus

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// eventBusService is the in-process event bus.
type eventBusService struct {
	logger *log.Logger
	// ctx is the parent of every delivery context. Stop cancels it once the flush wait ends.
	ctx    context.Context
	cancel context.CancelFunc
	// flushWait is how long stop lets the queues drain.
	flushWait time.Duration

	mu         sync.RWMutex
	closed     bool
	subs       map[string]*subscription
	byCategory map[Category][]*subscription
	wg         sync.WaitGroup
}

// newEventBusService returns a bus with its registry and stopper.
func newEventBusService() (EventBusInterface, RegistryInterface, StopperInterface) {
	ctx, cancel := context.WithCancel(context.Background())
	bus := &eventBusService{
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
		ctx:        ctx,
		cancel:     cancel,
		flushWait:  flushWait,
		subs:       make(map[string]*subscription),
		byCategory: make(map[Category][]*subscription),
	}
	return bus, &registrar{bus: bus}, &stopper{bus: bus}
}

// Publish records the event for the subscribers of each category it carries. The envelope is
// stamped only when a category has a subscriber, so an event nobody receives costs validation alone.
func (s *eventBusService) Publish(ctx context.Context, evt *Event) error {
	if err := evt.validate(); err != nil {
		return err
	}

	full, err := s.enqueue(evt)
	// Logged after the lock is released, so a slow log sink cannot hold up stop, register or publish.
	for _, sub := range full {
		sub.noteDrop(ctx)
	}
	return err
}

// enqueue offers evt to the queue of each of its subscribers and returns those whose queue was full.
func (s *eventBusService) enqueue(evt *Event) ([]*subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errBusClosed
	}

	var env Envelope
	var full []*subscription
	stamped := false
	for category, payload := range evt.payloads {
		subscribers := s.byCategory[category]
		if len(subscribers) == 0 {
			continue
		}
		if !stamped {
			var err error
			if env, err = stamp(evt); err != nil {
				return nil, err
			}
			stamped = true
		}
		env.Category = category
		for _, sub := range subscribers {
			select {
			case sub.queue <- delivery{env: env, payload: payload}:
			default:
				full = append(full, sub)
			}
		}
	}
	return full, nil
}

// register adds a subscriber of a category and starts its worker.
func (s *eventBusService) register(category Category, id string, fn DeliveryFunc) error {
	if id == "" {
		return errEmptySubscriberID
	}
	if fn == nil {
		return errNilDeliveryFunc
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errBusClosed
	}
	if _, exists := s.subs[id]; exists {
		return errDuplicateSubscriberID
	}

	sub := &subscription{
		deliver: fn,
		queue:   make(chan delivery, queueSize),
		logger:  s.logger.With(log.String("subscriberID", id), log.String("category", string(category))),
	}
	s.subs[id] = sub
	s.byCategory[category] = append(s.byCategory[category], sub)
	s.wg.Go(func() { sub.run(s) })
	return nil
}

// stop refuses new events, lets the queues drain within the flush wait, then cancels the deliveries
// in flight and leaves the workers to drop what is left.
func (s *eventBusService) stop() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	for sub := range maps.Values(s.subs) {
		close(sub.queue)
	}
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(s.flushWait):
		s.logger.Warn(context.Background(), "Event bus stopped before delivering every queued event")
	}
	s.cancel()
}

// stamp returns the envelope of one publish of evt with the fields the bus owns filled in. An ID
// cannot be generated only when the clock is before the Unix epoch, and the event is then rejected
// rather than published under an ID that does not sort by time.
func stamp(evt *Event) (Envelope, error) {
	id, err := utils.GenerateUUIDv7()
	if err != nil {
		return Envelope{}, fmt.Errorf("%w: %w", errEventID, err)
	}
	env := evt.envelope
	env.ID = id
	env.Time = time.Now().UTC().Truncate(time.Microsecond)
	return env, nil
}

// registrar is the RegistryInterface of a bus, kept apart so that a holder of the bus cannot register.
type registrar struct {
	bus *eventBusService
}

// Register implements RegistryInterface.
func (r *registrar) Register(category Category, id string, fn DeliveryFunc) error {
	return r.bus.register(category, id, fn)
}

// stopper is the StopperInterface of a bus, kept apart so that a holder of the bus cannot stop it.
type stopper struct {
	bus *eventBusService
}

// Stop implements StopperInterface.
func (s *stopper) Stop() {
	s.bus.stop()
}
