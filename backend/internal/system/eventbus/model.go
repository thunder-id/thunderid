// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

import (
	"context"
	"time"
)

// Category is the purpose of an event, for example audit. A payload of a category puts the event in it.
type Category string

// Envelope holds the fields the framework fills in on every event.
type Envelope struct {
	// ID is unique per event, a UUID v7 so that IDs sort by time.
	ID string
	// Name identifies the attempted action, for example user.create.
	Name string
	// Category is the category of the payload delivered with this envelope.
	Category Category
	// Time is when the event was published, in UTC to the microsecond.
	Time time.Time
	// TraceID is the correlation ID of the request. A caller can supply it, so it is not trusted.
	TraceID string
}

// DeliveryFunc receives an event for one subscriber. Its ctx is canceled when Stop gives up waiting.
type DeliveryFunc func(ctx context.Context, env Envelope, payload Payload) error

// delivery is one event on its way to one subscriber, with only that subscriber's payload.
type delivery struct {
	env     Envelope
	payload Payload
}
