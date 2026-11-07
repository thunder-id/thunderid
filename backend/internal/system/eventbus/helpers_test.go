// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

const (
	categoryOne Category = "test_one"
	categoryTwo Category = "test_two"

	testEventName = "user.create"
	// waitLimit outlasts the flush wait, so a test can wait for a Stop that gives up.
	waitLimit = flushWait + 3*time.Second
)

var errHandlerFailed = errors.New("handler failed")

type payloadOne struct{ value string }

func (payloadOne) Category() Category { return categoryOne }

type payloadTwo struct{ value string }

func (payloadTwo) Category() Category { return categoryTwo }

// otherPayloadOne is a second payload type in the category of payloadOne.
type otherPayloadOne struct{}

func (otherPayloadOne) Category() Category { return categoryOne }

// pointerPayload is a payload published through a pointer.
type pointerPayload struct{ category Category }

func (p *pointerPayload) Category() Category { return p.category }

// panickingPayload panics when asked for its category.
type panickingPayload struct{}

func (panickingPayload) Category() Category { panic("category") }

// recorder keeps what a subscriber mock received. The mock's own call list is not read while a
// worker may still be calling it.
type recorder[P Payload] struct {
	mu        sync.Mutex
	envelopes []Envelope
	payloads  []P
}

func (r *recorder[P]) add(env Envelope, payload P) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.envelopes = append(r.envelopes, env)
	r.payloads = append(r.payloads, payload)
}

func (r *recorder[P]) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.envelopes)
}

func (r *recorder[P]) received() ([]Envelope, []P) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Envelope(nil), r.envelopes...), append([]P(nil), r.payloads...)
}

// busSuite holds the helpers the bus suites share.
type busSuite struct {
	suite.Suite
	stoppers map[EventBusInterface]StopperInterface
}

// newBus returns a bus and its registry. The bus is stopped when the test ends.
func (s *busSuite) newBus() (EventBusInterface, RegistryInterface) {
	bus, registry, stopper := Initialize()
	if s.stoppers == nil {
		s.stoppers = make(map[EventBusInterface]StopperInterface)
	}
	s.stoppers[bus] = stopper
	s.T().Cleanup(stopper.Stop)
	return bus, registry
}

func (s *busSuite) shutdown(bus EventBusInterface) {
	s.stoppers[bus].Stop()
}

// subscriberCount reads the number of registered subscribers from the concrete bus.
func (s *busSuite) subscriberCount(bus EventBusInterface) int {
	service := bus.(*eventBusService)
	service.mu.RLock()
	defer service.mu.RUnlock()
	return len(service.subs)
}

// fillQueue blocks the subscriber's worker on a first event, then fills its queue, so the next
// publish to it is dropped. After each step it waits for each of others to take every event, so none of
// them drops one.
func (s *busSuite) fillQueue(bus EventBusInterface, started <-chan struct{}, others ...*recorder[payloadOne]) {
	s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
	await(s, started)
	s.received(1, others...)
	for range queueSize {
		s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
	}
	s.received(queueSize+1, others...)
}

// received waits until each recorder holds n events.
func (s *busSuite) received(n int, recs ...*recorder[payloadOne]) {
	for _, rec := range recs {
		s.eventually(func() bool { return rec.count() == n })
	}
}

func (s *busSuite) eventually(condition func() bool) {
	s.Require().Eventually(condition, waitLimit, 5*time.Millisecond)
}

// await receives from ch and fails the test unless a value arrives within waitLimit.
func await[T any](s *busSuite, ch <-chan T) T {
	select {
	case value := <-ch:
		return value
	case <-time.After(waitLimit):
		s.FailNow("nothing received in time")
	}
	var zero T
	return zero
}

// completes runs f and fails the test unless f returns within waitLimit.
func (s *busSuite) completes(f func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(waitLimit):
		s.FailNow("did not complete in time")
	}
}

func (s *busSuite) publish(bus EventBusInterface, name string, payloads ...Payload) error {
	return bus.Publish(context.Background(), s.event(context.Background(), name, payloads...))
}

func (s *busSuite) event(ctx context.Context, name string, payloads ...Payload) *Event {
	evt := NewEvent(ctx, name)
	for _, payload := range payloads {
		evt = evt.With(payload)
	}
	return evt
}

// subscribe registers a subscriber mock for the category of P that expects exactly calls deliveries.
// Each delivery runs hook, when set, and records the event when hook returns nil. hook may block, fail
// or panic.
func subscribe[P Payload](s *busSuite, registry RegistryInterface, id string, calls int,
	hook func(ctx context.Context) error) *recorder[P] {
	rec := &recorder[P]{}
	sub := NewSubscriberMock[P](s.T())
	// Times(0) would allow any number of calls, so a subscriber expecting none gets no expectation and
	// any delivery fails the test.
	if calls > 0 {
		sub.EXPECT().Handle(mock.Anything, mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, env Envelope, payload P) error {
				if hook != nil {
					if err := hook(ctx); err != nil {
						return err
					}
				}
				rec.add(env, payload)
				return nil
			}).Times(calls)
	}
	s.Require().NoError(Subscribe[P](registry, id, sub))
	return rec
}

// blockingHook blocks every delivery until release is closed and signals the first one on started.
func blockingHook(started chan<- struct{}, release <-chan struct{}) func(context.Context) error {
	var once sync.Once
	return func(context.Context) error {
		once.Do(func() { close(started) })
		<-release
		return nil
	}
}

// linesWith returns the lines of output that contain message.
func linesWith(output, message string) []string {
	var lines []string
	for line := range strings.Lines(output) {
		if strings.Contains(line, message) {
			lines = append(lines, line)
		}
	}
	return lines
}
