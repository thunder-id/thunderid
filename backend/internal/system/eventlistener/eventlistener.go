// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package eventlistener lets a service announce a state change to listeners chosen by the
// composition root, without depending on them.
//
// The contract, shared by every topic:
//
//   - The producer calls Notify only after the change it announces is durable, so a listener is
//     never told about something that did not happen.
//   - Listeners are called synchronously, in registration order. They must return promptly and hand
//     slow work to a queue of their own; the producer waits on the calls, not on the work.
//   - A panic in a listener is recovered and logged. It neither reaches the producer nor stops the
//     next listener.
//   - The context given to a listener carries the producer's values, such as the trace id, but is
//     never cancelled, so a listener may pass it to work that outlives the request.
//   - Listeners are registered during startup through the Hook the producer hands to the composition
//     root. Registration is additive: nil is rejected and nothing can be removed. A consumer of the
//     producing service never holds the Hook, so it cannot change who observes the topic.
package eventlistener

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// Listener receives every event of one topic.
type Listener[T any] interface {
	OnEvent(ctx context.Context, event T)
}

// Hook registers listeners on one topic. The producer hands it to the composition root, which needs
// it because listeners usually depend on services built after the producer.
type Hook[T any] interface {
	// Add registers a listener. It rejects a nil listener. Listeners cannot be removed.
	Add(listener Listener[T]) error
}

// Topic is the producer's end. It owns the listener list and the fan-out.
type Topic[T any] struct {
	name      string
	logger    *log.Logger
	listeners []Listener[T]
}

// NewTopic creates a topic. The name only labels log lines.
func NewTopic[T any](name string) *Topic[T] {
	return &Topic[T]{
		name:   name,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EventListener")),
	}
}

// Hook returns the registration end of the topic.
func (t *Topic[T]) Hook() Hook[T] {
	return hook[T]{topic: t}
}

// HasListeners reports whether anything is registered, so a producer can skip work that only
// feeds listeners. A nil topic has none.
func (t *Topic[T]) HasListeners() bool {
	return t != nil && len(t.listeners) > 0
}

// Notify hands the event to every listener in registration order, each isolated from the others'
// panics, with a context that keeps the caller's values but not its cancellation. A nil topic
// notifies nobody.
func (t *Topic[T]) Notify(ctx context.Context, event T) {
	if t == nil {
		return
	}
	detached := context.WithoutCancel(ctx)
	for _, listener := range t.listeners {
		t.notifyOne(ctx, detached, listener, event)
	}
}

// notifyOne calls one listener, recovering and logging a panic so the remaining listeners still run.
func (t *Topic[T]) notifyOne(ctx, detached context.Context, listener Listener[T], event T) {
	defer func() {
		if r := recover(); r != nil {
			t.logger.Error(ctx, "Event listener panicked", log.String("topic", t.name), log.Any("panic", r))
		}
	}()
	listener.OnEvent(detached, event)
}

// hook implements Hook by appending to its topic's listener list.
type hook[T any] struct {
	topic *Topic[T]
}

// Add implements Hook.
func (h hook[T]) Add(listener Listener[T]) error {
	if listener == nil {
		return errors.New("event listener must not be nil")
	}
	h.topic.listeners = append(h.topic.listeners, listener)
	return nil
}
