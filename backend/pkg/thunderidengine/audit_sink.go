// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thunderidengine

import (
	"context"
	"fmt"
	"reflect"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// auditSinkIDPrefix starts the stable subscriber ID of an embedder's sink. The ID ends with the sink's
// position among the WithAuditSink options, so it stays the same while the embedder's options do.
const auditSinkIDPrefix = "audit-embedder-"

// subscribeAuditSinks subscribes each sink to audit events on its own queue and worker. With no sink it
// subscribes nothing, so an event is validated and dropped and no worker runs.
func subscribeAuditSinks(registry eventbus.RegistryInterface, sinks []providers.AuditSink) error {
	for i, sink := range sinks {
		id := fmt.Sprintf("%s%d", auditSinkIDPrefix, i+1)
		if err := audit.Subscribe(registry, id, auditSinkAdapter{sink: sink}); err != nil {
			return fmt.Errorf("failed to subscribe the audit sink %s: %w", id, err)
		}
	}
	return nil
}

// auditSinkAdapter gives an embedder's sink the plain view of an event, so the sink imports no internal
// package.
type auditSinkAdapter struct {
	sink providers.AuditSink
}

// Handle passes the event on as plain values. The payload hands out a copy of its details, so a sink's
// edits reach no other sink.
func (a auditSinkAdapter) Handle(ctx context.Context, env eventbus.Envelope, payload audit.Payload) error {
	return a.sink.Handle(ctx, providers.AuditEvent{
		ID:         env.ID,
		Name:       env.Name,
		Time:       env.Time.UTC(),
		TraceID:    env.TraceID,
		Actor:      payload.Actor(),
		OnBehalfOf: payload.OnBehalfOf(),
		Subject:    payload.Subject(),
		Target:     payload.Target(),
		Action:     payload.Action(),
		Outcome:    payload.Outcome(),
		Client:     payload.Client(),
		Details:    payload.Details(),
	})
}

// isNilAuditSink reports whether sink is nil or an interface holding a nil pointer, map, slice, func or
// channel, which would panic on its first event.
func isNilAuditSink(sink providers.AuditSink) bool {
	if sink == nil {
		return true
	}
	switch value := reflect.ValueOf(sink); value.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return value.IsNil()
	default:
		return false
	}
}
