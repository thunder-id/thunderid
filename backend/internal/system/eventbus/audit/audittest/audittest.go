// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package audittest holds the test helpers a package that publishes audit events shares.
package audittest

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

// Event is one audit event a service published.
type Event struct {
	Name    string
	Payload audit.Payload
}

// recordingSink keeps every audit event it is handed, in order.
type recordingSink struct {
	mu     sync.Mutex
	events []Event
}

func (r *recordingSink) Handle(_ context.Context, env eventbus.Envelope, payload audit.Payload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, Event{Name: env.Name, Payload: payload})
	return nil
}

// NewRecordingBus returns an event bus with a sink subscribed under id the way the binaries subscribe
// theirs, so an event audit refuses never reaches it, and a function that stops the bus and returns the
// audit events written in order.
func NewRecordingBus(t *testing.T, id string) (eventbus.EventBusInterface, func() []Event) {
	t.Helper()
	bus, registry, stopper := eventbus.Initialize()
	t.Cleanup(stopper.Stop)

	sink := &recordingSink{}
	require.NoError(t, audit.Subscribe(registry, id, sink))
	return bus, func() []Event {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// NewDiscardingBus returns an event bus that accepts and drops every event, for a test the audit event
// does not matter to. A test of what a service publishes uses NewRecordingBus.
func NewDiscardingBus(t *testing.T) eventbus.EventBusInterface {
	t.Helper()
	bus := eventbusmock.NewEventBusInterfaceMock(t)
	bus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()
	return bus
}
