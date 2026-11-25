// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

import "context"

// EventBusInterface publishes events.
type EventBusInterface interface {
	// Publish records the event for every subscriber of its categories and returns. It never runs
	// subscriber code, and a slow or full subscriber never makes it return an error.
	Publish(ctx context.Context, evt *Event) error
}

// RegistryInterface registers subscribers. Only the composition root holds it.
type RegistryInterface interface {
	// Register adds a subscriber of a category under a stable id. Prefer Subscribe, which is typed.
	Register(category Category, id string, fn DeliveryFunc) error
}

// StopperInterface stops the event bus. Only the composition root holds it.
type StopperInterface interface {
	// Stop refuses new events and lets the queues drain for a fixed wait, then cancels the deliveries in
	// flight and drops the rest. A second call returns at once, even while the first drains.
	Stop()
}

// Payload is the data an event carries for one category. It must be a value, not a pointer, and must
// not be changed after Publish. It is passed by value, but a map, slice or pointer-typed field inside it
// is shared by every subscriber, so a subscriber must not modify a payload or retain state it shares.
type Payload interface {
	// Category returns the category the payload puts its event in.
	Category() Category
}

// Subscriber receives the events of the category of P.
type Subscriber[P Payload] interface {
	// Handle processes one event and returns an error when delivery failed. Its ctx is canceled when
	// Stop gives up waiting.
	Handle(ctx context.Context, env Envelope, payload P) error
}
