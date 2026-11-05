// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
)

// Sink receives audit events, each with the envelope the bus filled in.
type Sink = eventbus.Subscriber[Payload]

// Subscribe registers sink for audit events under a stable id. The sink receives only events that pass
// validation; the bus logs the reason it dropped any other.
func Subscribe(registry eventbus.RegistryInterface, id string, sink Sink) error {
	if sink == nil {
		return errNilSink
	}
	return eventbus.Subscribe[Payload](registry, id, validatingSink{sink: sink})
}

// validatingSink hands a sink only the events whose payload is valid for the event's name.
type validatingSink struct {
	sink Sink
}

// Handle passes a valid event on and refuses any other, so it is never written.
func (s validatingSink) Handle(ctx context.Context, env eventbus.Envelope, payload Payload) error {
	if err := payload.validate(env.Name); err != nil {
		return err
	}
	return s.sink.Handle(ctx, env, payload)
}
