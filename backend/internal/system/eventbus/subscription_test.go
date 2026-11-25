// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/suite"

	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

const (
	dropWarning    = "Subscriber queue is full, events are being dropped"
	failureWarning = "Subscriber failed to handle an event"
	panicWarning   = "Subscriber panicked while handling an event"
)

type SubscriptionTestSuite struct {
	busSuite
}

func TestSubscriptionTestSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionTestSuite))
}

func (s *SubscriptionTestSuite) TestPanicIsLoggedAndTheWorkerContinues() {
	bus, registry := s.newBus()
	var calls atomic.Int32
	rec := subscribe[payloadOne](&s.busSuite, registry, "panicky", 2, func(context.Context) error {
		if calls.Add(1) == 1 {
			panic("boom")
		}
		return nil
	})

	output := testhelpers.CaptureLogs(s.T(), "INFO", func() {
		s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
		s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
		s.shutdown(bus)
	})

	s.Equal(1, rec.count())
	s.Equal(1, strings.Count(output, panicWarning))
}

func (s *SubscriptionTestSuite) TestDropWarningIsLoggedOncePerSubscriber() {
	bus, registry := s.newBus()
	firstStarted, secondStarted := make(chan struct{}), make(chan struct{})
	release := make(chan struct{})
	first := subscribe[payloadOne](&s.busSuite, registry, "first", queueSize+1, blockingHook(firstStarted, release))
	second := subscribe[payloadOne](&s.busSuite, registry, "second", queueSize+1, blockingHook(secondStarted, release))

	output := testhelpers.CaptureLogs(s.T(), "INFO", func() {
		s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
		await(&s.busSuite, firstStarted)
		await(&s.busSuite, secondStarted)
		for range queueSize {
			s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
		}
	})
	s.NotContains(output, dropWarning)

	output = testhelpers.CaptureLogs(s.T(), "INFO", func() {
		for range 5 {
			s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
		}
		close(release)
		s.shutdown(bus)
	})

	warnings := linesWith(output, dropWarning)
	s.Require().Len(warnings, 2, "one warning per subscriber")
	s.Len(linesWith(strings.Join(warnings, ""), "subscriberID=first"), 1)
	s.Len(linesWith(strings.Join(warnings, ""), "subscriberID=second"), 1)
	for _, warning := range warnings {
		s.Contains(warning, "category="+string(categoryOne))
	}
	s.Equal(queueSize+1, first.count())
	s.Equal(queueSize+1, second.count())
}

func (s *SubscriptionTestSuite) TestDropWarningIsLoggedOncePerOverflow() {
	bus, registry := s.newBus()
	entered := make(chan struct{}, 1)
	var gate sync.RWMutex
	rec := subscribe[payloadOne](&s.busSuite, registry, "slow", 2*(queueSize+1), func(context.Context) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		gate.RLock()
		defer gate.RUnlock()
		return nil
	})
	overflow := func() string {
		return testhelpers.CaptureLogs(s.T(), "INFO", func() {
			gate.Lock()
			defer gate.Unlock()
			s.fillQueue(bus, entered)
			for range 5 {
				s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
			}
		})
	}

	first := linesWith(overflow(), dropWarning)
	slow := bus.(*eventBusService).subs["slow"]
	s.eventually(func() bool { return rec.count() == queueSize+1 && !slow.dropWarned.Load() })
	<-entered
	second := linesWith(overflow(), dropWarning)
	s.shutdown(bus)

	s.Require().Len(first, 1, "one warning per overflow")
	s.Require().Len(second, 1, "one warning per overflow")
	for _, warning := range append(first, second...) {
		s.Contains(warning, "subscriberID=slow")
		s.Contains(warning, "category="+string(categoryOne))
	}
	s.Equal(2*(queueSize+1), rec.count())
}

// A drop warning is written once Publish has released the bus lock, so a slow log sink cannot hold up
// register or stop, nor the publishes queued behind a waiting writer.
func (s *SubscriptionTestSuite) TestDropWarningIsLoggedWithoutHoldingTheBusLock() {
	bus, registry := s.newBus()
	started := make(chan struct{})
	release := make(chan struct{})
	subscribe[payloadOne](&s.busSuite, registry, "slow", queueSize+1, blockingHook(started, release))
	ctx := &lockProbeContext{Context: context.Background(), bus: bus.(*eventBusService), lockFree: make(chan bool, 1)}

	output := testhelpers.CaptureLogs(s.T(), "INFO", func() {
		s.fillQueue(bus, started)
		s.Require().NoError(bus.Publish(ctx, s.event(context.Background(), testEventName, payloadOne{})))
	})
	close(release)
	s.shutdown(bus)

	s.True(await(&s.busSuite, ctx.lockFree), "the warning was written while the bus lock was held")
	s.Len(linesWith(output, dropWarning), 1)
}

// lockProbeContext records, when the logger reads the trace ID to write a warning, whether the bus
// write lock that register and stop take is free.
type lockProbeContext struct {
	context.Context
	bus      *eventBusService
	lockFree chan bool
}

func (c *lockProbeContext) Value(key any) any {
	if key == sysContext.TraceIDKey {
		free := c.bus.mu.TryLock()
		if free {
			c.bus.mu.Unlock()
		}
		select {
		case c.lockFree <- free:
		default:
		}
	}
	return c.Context.Value(key)
}

func (s *SubscriptionTestSuite) TestEveryFailureAndPanicIsLoggedWithTheEvent() {
	bus, registry := s.newBus()
	subscribe[payloadOne](&s.busSuite, registry, "failing", 3, func(context.Context) error { return errHandlerFailed })
	subscribe[payloadOne](&s.busSuite, registry, "panicking", 3, func(context.Context) error { panic("boom") })

	output := testhelpers.CaptureLogs(s.T(), "INFO", func() {
		for range 3 {
			s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
		}
		s.shutdown(bus)
	})

	s.Equal(3, strings.Count(output, failureWarning))
	s.Equal(3, strings.Count(output, panicWarning))
	s.Contains(output, errHandlerFailed.Error())
	s.Contains(output, "eventID=")
	s.Contains(output, "event="+testEventName)
	s.Contains(output, "category="+string(categoryOne))
}
