// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

import (
	"context"
	"sync/atomic"

	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// subscription is one subscriber with its queue.
type subscription struct {
	deliver DeliveryFunc
	queue   chan delivery
	logger  *log.Logger
	// dropWarned keeps a full queue to one warning until the worker empties it, so a slow subscriber
	// cannot flood the log.
	dropWarned atomic.Bool
}

// run delivers queued events one at a time until the queue is closed, dropping them once canceled.
func (s *subscription) run(bus *eventBusService) {
	for d := range s.queue {
		if bus.ctx.Err() != nil {
			continue
		}
		s.handle(bus.ctx, d)
		s.noteDrained()
	}
}

// handle calls the delivery function and logs a failure. A panic or error never stops the worker.
func (s *subscription) handle(busCtx context.Context, d delivery) {
	ctx := sysContext.WithTraceID(busCtx, d.env.TraceID)
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error(ctx, "Subscriber panicked while handling an event",
				eventFields(d.env, log.Any("panic", r))...)
		}
	}()
	if err := s.deliver(ctx, d.env, d.payload); err != nil {
		s.logger.Error(ctx, "Subscriber failed to handle an event", eventFields(d.env, log.Error(err))...)
	}
}

// noteDrop warns the first time the subscriber's queue is full since the worker last emptied it.
func (s *subscription) noteDrop(ctx context.Context) {
	if s.dropWarned.CompareAndSwap(false, true) {
		s.logger.Warn(ctx, "Subscriber queue is full, events are being dropped")
	}
}

// noteDrained ends a drop episode once the queue is empty, so the next full queue warns again. Ending it
// on the next accepted event instead would warn once per event the worker takes under sustained overload.
func (s *subscription) noteDrained() {
	if len(s.queue) == 0 && s.dropWarned.Load() {
		s.dropWarned.Store(false)
	}
}

// eventFields returns the log fields that identify an event, followed by extra.
func eventFields(env Envelope, extra ...log.Field) []log.Field {
	return append([]log.Field{log.String("eventID", env.ID), log.String("event", env.Name)}, extra...)
}
