// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thunderidengine

import (
	"context"
	"fmt"

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

// Handle passes the event on as plain values.
func (a auditSinkAdapter) Handle(ctx context.Context, env eventbus.Envelope, payload audit.Payload) error {
	return a.sink.Handle(ctx, newAuditEvent(env, payload))
}

// newAuditEvent copies an envelope and its payload into the public view. The payload hands out a copy of
// its details, so a sink's edits reach no other sink.
func newAuditEvent(env eventbus.Envelope, payload audit.Payload) providers.AuditEvent {
	outcome := payload.Outcome()
	client := payload.Client()
	actor := payload.Actor()
	return providers.AuditEvent{
		ID:         env.ID,
		Name:       env.Name,
		Time:       env.Time.UTC(),
		TraceID:    env.TraceID,
		Actor:      providers.AuditRef{Type: string(actor.Type), ID: actor.ID},
		OnBehalfOf: auditRef(payload.OnBehalfOf()),
		Subject:    auditRef(payload.Subject()),
		Target:     auditRef(payload.Target()),
		Action:     string(payload.Action()),
		Outcome:    providers.AuditOutcome{Result: string(outcome.Result), Reason: outcome.Reason},
		Client:     providers.AuditClient{Address: client.Address, UserAgent: client.UserAgent},
		Details:    payload.Details(),
	}
}

// auditRef converts a reference to the public view.
func auditRef(ref audit.Ref) providers.AuditRef {
	return providers.AuditRef{Type: ref.Type, ID: ref.ID}
}
