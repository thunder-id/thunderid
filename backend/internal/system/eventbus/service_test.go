// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

const (
	stopWarning = "Event bus stopped before delivering every queued event"
	// shortFlushWait stands in for the flush wait in a test that waits it out.
	shortFlushWait = 50 * time.Millisecond
)

type EventBusServiceTestSuite struct {
	busSuite
}

func TestEventBusServiceTestSuite(t *testing.T) {
	suite.Run(t, new(EventBusServiceTestSuite))
}

func (s *EventBusServiceTestSuite) TestRoutesEachCategoryToItsOwnSubscribersOnly() {
	bus, registry := s.newBus()
	defer s.shutdown(bus)
	one := subscribe[payloadOne](&s.busSuite, registry, "one", 2, nil)
	two := subscribe[payloadTwo](&s.busSuite, registry, "two", 2, nil)

	s.Require().NoError(s.publish(bus, "user.create", payloadOne{value: "a"}))
	s.Require().NoError(s.publish(bus, "user.update", payloadTwo{value: "b"}))
	s.Require().NoError(s.publish(bus, "user.delete", payloadOne{value: "c"}, payloadTwo{value: "d"}))

	s.eventually(func() bool { return one.count() == 2 && two.count() == 2 })
	_, onePayloads := one.received()
	_, twoPayloads := two.received()
	s.Equal([]payloadOne{{value: "a"}, {value: "c"}}, onePayloads)
	s.Equal([]payloadTwo{{value: "b"}, {value: "d"}}, twoPayloads)
}

func (s *EventBusServiceTestSuite) TestEventInSeveralCategoriesReachesEachSubscriberOnce() {
	bus, registry := s.newBus()
	firstOne := subscribe[payloadOne](&s.busSuite, registry, "one-a", 1, nil)
	secondOne := subscribe[payloadOne](&s.busSuite, registry, "one-b", 1, nil)
	two := subscribe[payloadTwo](&s.busSuite, registry, "two", 1, nil)

	s.Require().NoError(s.publish(bus, testEventName, payloadOne{value: "a"}, payloadTwo{value: "b"}))
	s.shutdown(bus)

	s.Equal(1, firstOne.count())
	s.Equal(1, secondOne.count())
	s.Equal(1, two.count())
	oneEnvelopes, _ := firstOne.received()
	twoEnvelopes, _ := two.received()
	s.Equal(oneEnvelopes[0].ID, twoEnvelopes[0].ID)
}

func (s *EventBusServiceTestSuite) TestSubscriberNeverSeesAnotherCategoryPayload() {
	bus, registry := s.newBus()
	var received []Payload
	var mu sync.Mutex
	err := registry.Register(categoryOne, "raw", func(_ context.Context, _ Envelope, payload Payload) error {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, payload)
		return nil
	})
	s.Require().NoError(err)

	s.Require().NoError(s.publish(bus, testEventName, payloadOne{value: "a"}, payloadTwo{value: "b"}))
	s.shutdown(bus)

	mu.Lock()
	defer mu.Unlock()
	s.Equal([]Payload{payloadOne{value: "a"}}, received)
}

func (s *EventBusServiceTestSuite) TestEnvelopeCarriesEveryField() {
	bus, registry := s.newBus()
	defer s.shutdown(bus)
	rec := subscribe[payloadOne](&s.busSuite, registry, "one", 1, nil)
	ctx := sysContext.WithTraceID(context.Background(), "trace-1")

	before := time.Now().UTC().Truncate(time.Microsecond)
	s.Require().NoError(bus.Publish(ctx, s.event(ctx, testEventName, payloadOne{})))
	after := time.Now().UTC()

	s.eventually(func() bool { return rec.count() == 1 })
	envelopes, _ := rec.received()
	env := envelopes[0]
	s.True(utils.IsValidUUID(env.ID), env.ID)
	s.Equal(byte('7'), env.ID[14], "event ID is a UUID v7")
	s.Equal(testEventName, env.Name)
	s.Equal(categoryOne, env.Category)
	s.Equal(time.UTC, env.Time.Location())
	s.Zero(env.Time.Nanosecond()%int(time.Microsecond), "time is truncated to the microsecond")
	s.False(env.Time.Before(before))
	s.False(env.Time.After(after))
	s.Equal("trace-1", env.TraceID)
}

func (s *EventBusServiceTestSuite) TestEachPublishGetsItsOwnID() {
	bus, registry := s.newBus()
	defer s.shutdown(bus)
	rec := subscribe[payloadOne](&s.busSuite, registry, "one", 2, nil)
	evt := s.event(context.Background(), testEventName, payloadOne{})

	s.Require().NoError(bus.Publish(context.Background(), evt))
	s.Require().NoError(bus.Publish(context.Background(), evt))

	s.eventually(func() bool { return rec.count() == 2 })
	envelopes, _ := rec.received()
	s.NotEqual(envelopes[0].ID, envelopes[1].ID)
}

func (s *EventBusServiceTestSuite) TestPublishNeverBlocksOnAFullSubscriberAndOthersAreUnaffected() {
	bus, registry := s.newBus()
	started := make(chan struct{})
	release := make(chan struct{})
	slow := subscribe[payloadOne](&s.busSuite, registry, "slow", queueSize+1, blockingHook(started, release))
	fast := subscribe[payloadOne](&s.busSuite, registry, "fast", queueSize+11, nil)
	s.fillQueue(bus, started, fast)

	s.completes(func() {
		for range 10 {
			s.NoError(s.publish(bus, testEventName, payloadOne{}))
		}
	})
	s.received(queueSize+11, fast)
	close(release)
	s.shutdown(bus)

	s.Equal(queueSize+11, fast.count())
	s.Equal(queueSize+1, slow.count(), "the events published while the queue was full are dropped")
}

func (s *EventBusServiceTestSuite) TestSubscriberReceivesWhatIsPublishedAfterItsQueueHasRoomAgain() {
	bus, registry := s.newBus()
	started := make(chan struct{})
	release := make(chan struct{})
	rec := subscribe[payloadOne](&s.busSuite, registry, "slow", queueSize+2, blockingHook(started, release))
	s.fillQueue(bus, started)
	s.Require().NoError(s.publish(bus, testEventName, payloadOne{value: "dropped"}))

	close(release)
	s.eventually(func() bool { return rec.count() == queueSize+1 })
	s.Require().NoError(s.publish(bus, testEventName, payloadOne{value: "last"}))
	s.shutdown(bus)

	_, payloads := rec.received()
	s.Len(payloads, queueSize+2)
	s.NotContains(payloads, payloadOne{value: "dropped"})
	s.Equal(payloadOne{value: "last"}, payloads[len(payloads)-1])
}

func (s *EventBusServiceTestSuite) TestFailureDoesNotAffectOtherSubscribers() {
	bus, registry := s.newBus()
	subscribe[payloadOne](&s.busSuite, registry, "failing", 1, func(context.Context) error { return errHandlerFailed })
	healthy := subscribe[payloadOne](&s.busSuite, registry, "healthy", 1, nil)

	s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
	s.shutdown(bus)

	s.Equal(1, healthy.count())
}

func (s *EventBusServiceTestSuite) TestEachSubscriberHandlesOneEventAtATime() {
	bus, registry := s.newBus()
	var active, maxActive atomic.Int32
	rec := subscribe[payloadOne](&s.busSuite, registry, "serial", 40, func(context.Context) error {
		now := active.Add(1)
		for {
			current := maxActive.Load()
			if now <= current || maxActive.CompareAndSwap(current, now) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		active.Add(-1)
		return nil
	})

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 5 {
				s.NoError(s.publish(bus, testEventName, payloadOne{}))
			}
		})
	}
	wg.Wait()

	s.eventually(func() bool { return rec.count() == 40 })
	s.Equal(int32(1), maxActive.Load())
	s.shutdown(bus)
}

func (s *EventBusServiceTestSuite) TestStopDrainsQueuedEvents() {
	bus, registry := s.newBus()
	rec := subscribe[payloadOne](&s.busSuite, registry, "drain", 10, nil)
	for range 10 {
		s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
	}

	s.shutdown(bus)

	s.Equal(10, rec.count())
}

// One Stop covers a subscriber that honors cancellation and one that ignores it.
func (s *EventBusServiceTestSuite) TestStopGivesUpAfterTheFlushWaitAndCancelsDeliveries() {
	bus, registry := s.newBus()
	service := bus.(*eventBusService)
	service.flushWait = shortFlushWait
	honorStarted, ignoreStarted := make(chan struct{}), make(chan struct{})
	canceled := make(chan error, 1)
	var honorCalls atomic.Int32
	err := registry.Register(categoryOne, "honors", func(ctx context.Context, _ Envelope, _ Payload) error {
		honorCalls.Add(1)
		close(honorStarted)
		<-ctx.Done()
		canceled <- ctx.Err()
		return ctx.Err()
	})
	s.Require().NoError(err)
	release := make(chan struct{})
	ignores := subscribe[payloadOne](&s.busSuite, registry, "ignores", 1, blockingHook(ignoreStarted, release))
	for range 3 {
		s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
	}
	await(&s.busSuite, honorStarted)
	await(&s.busSuite, ignoreStarted)

	began := time.Now()
	output := testhelpers.CaptureLogs(s.T(), "INFO", func() {
		s.completes(func() { s.shutdown(bus) })
	})

	s.GreaterOrEqual(time.Since(began), shortFlushWait)
	s.ErrorIs(await(&s.busSuite, canceled), context.Canceled)
	s.Contains(output, stopWarning)
	close(release)
	s.completes(service.wg.Wait)
	s.Equal(int32(1), honorCalls.Load(), "the queued events are dropped once Stop gives up")
	s.Equal(1, ignores.count(), "the queued events are dropped once Stop gives up")
}

func (s *EventBusServiceTestSuite) TestSecondStopReturnsWithoutWaitingForTheFirst() {
	bus, registry := s.newBus()
	started := make(chan struct{})
	release := make(chan struct{})
	releaseHandler := sync.OnceFunc(func() { close(release) })
	defer releaseHandler()
	rec := subscribe[payloadOne](&s.busSuite, registry, "stuck", 2, blockingHook(started, release))
	s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
	s.Require().NoError(s.publish(bus, testEventName, payloadOne{}))
	await(&s.busSuite, started)

	output := testhelpers.CaptureLogs(s.T(), "INFO", func() {
		firstDone := make(chan struct{})
		go func() {
			defer close(firstDone)
			s.shutdown(bus)
		}()
		service := bus.(*eventBusService)
		s.eventually(func() bool {
			service.mu.RLock()
			defer service.mu.RUnlock()
			return service.closed
		})
		s.completes(func() { s.shutdown(bus) })

		select {
		case <-firstDone:
			s.Fail("the first Stop returned while its subscriber was still blocked")
		default:
		}
		releaseHandler()
		await(&s.busSuite, firstDone)
	})

	s.Equal(2, rec.count())
	s.NotContains(output, stopWarning)
}

func (s *EventBusServiceTestSuite) TestPublishAndRegisterAfterStopAreRejected() {
	bus, registry := s.newBus()
	s.shutdown(bus)

	s.ErrorIs(s.publish(bus, testEventName, payloadOne{}), errBusClosed)
	s.ErrorIs(registry.Register(categoryOne, "late", discard), errBusClosed)
}

func (s *EventBusServiceTestSuite) TestSubscriberContextCarriesTheEventTrace() {
	bus, registry := s.newBus()
	received := make(chan string, 1)
	err := registry.Register(categoryOne, "one", func(ctx context.Context, _ Envelope, _ Payload) error {
		received <- sysContext.GetTraceID(ctx)
		return nil
	})
	s.Require().NoError(err)
	ctx := sysContext.WithTraceID(context.Background(), "trace-1")

	s.Require().NoError(bus.Publish(ctx, s.event(ctx, testEventName, payloadOne{})))
	s.shutdown(bus)

	s.Equal("trace-1", <-received)
}

// Every outcome is checked, not timing: once a call sees the bus closed, every later call of the same
// kind must too, and exactly the subscribers that registered are left.
func (s *EventBusServiceTestSuite) TestPublishRegisterAndStopRunConcurrently() {
	bus, registry := s.newBus()
	start := make(chan struct{})
	var registered atomic.Int64
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			<-start
			closed := false
			for range 200 {
				err := s.publish(bus, testEventName, payloadOne{})
				if closed || err != nil {
					s.ErrorIs(err, errBusClosed)
					closed = true
				}
			}
		})
	}
	for i := range 4 {
		wg.Go(func() {
			<-start
			closed := false
			for j := range 50 {
				err := registry.Register(categoryOne, fmt.Sprintf("sub-%d-%d", i, j), discard)
				if closed || err != nil {
					s.ErrorIs(err, errBusClosed)
					closed = true
					continue
				}
				registered.Add(1)
			}
		})
	}
	wg.Go(func() {
		<-start
		s.shutdown(bus)
	})

	close(start)
	s.completes(wg.Wait)

	s.ErrorIs(s.publish(bus, testEventName, payloadOne{}), errBusClosed)
	s.Equal(int(registered.Load()), s.subscriberCount(bus))
}

func (s *EventBusServiceTestSuite) TestDuplicateAndEmptySubscriberIDsAreRejected() {
	bus, registry := s.newBus()
	defer s.shutdown(bus)
	s.Require().NoError(registry.Register(categoryOne, "same", discard))

	s.ErrorIs(registry.Register(categoryOne, "same", discard), errDuplicateSubscriberID)
	s.ErrorIs(registry.Register(categoryTwo, "same", discard), errDuplicateSubscriberID)
	s.ErrorIs(registry.Register(categoryOne, "", discard), errEmptySubscriberID)
	s.ErrorIs(registry.Register(categoryOne, "nil-delivery-func", nil), errNilDeliveryFunc)
	s.Equal(1, s.subscriberCount(bus))
}

func (s *EventBusServiceTestSuite) TestInvalidEventsAreRejectedAndNothingIsDelivered() {
	bus, registry := s.newBus()
	rec := subscribe[payloadOne](&s.busSuite, registry, "one", 0, nil)

	s.ErrorIs(bus.Publish(context.Background(), nil), errNilEvent)
	s.ErrorIs(s.publish(bus, "BAD NAME", payloadOne{}), errInvalidEventName)
	s.ErrorIs(s.publish(bus, testEventName), errNoPayload)
	s.ErrorIs(s.publish(bus, testEventName, payloadOne{}, payloadOne{}), errDuplicateCategory)
	s.shutdown(bus)

	s.Zero(rec.count())
}

// benchmarkBus returns a bus and its registry. The bus is shut down when the benchmark ends.
func benchmarkBus(b *testing.B) (EventBusInterface, RegistryInterface) {
	bus, registry, stopper := Initialize()
	b.Cleanup(stopper.Stop)
	return bus, registry
}

// discard is a delivery function that accepts every event.
func discard(context.Context, Envelope, Payload) error { return nil }

func benchmarkPublish(b *testing.B, bus EventBusInterface) {
	ctx := sysContext.WithTraceID(context.Background(), "trace-1")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = bus.Publish(ctx, NewEvent(ctx, testEventName).With(payloadOne{value: "x"}))
	}
}

func BenchmarkNewEventAndPublishOneSubscriber(b *testing.B) {
	bus, registry := benchmarkBus(b)
	if err := registry.Register(categoryOne, "bench", discard); err != nil {
		b.Fatal(err)
	}
	benchmarkPublish(b, bus)
}

func BenchmarkNewEventAndPublishNoSubscriber(b *testing.B) {
	bus, _ := benchmarkBus(b)
	benchmarkPublish(b, bus)
}

func BenchmarkNewEventAndPublishFullQueue(b *testing.B) {
	bus, registry := benchmarkBus(b)
	release := make(chan struct{})
	b.Cleanup(func() { close(release) })
	if err := registry.Register(categoryOne, "stuck", func(context.Context, Envelope, Payload) error {
		<-release
		return nil
	}); err != nil {
		b.Fatal(err)
	}
	benchmarkPublish(b, bus)
}
