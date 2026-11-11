// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"context"
	"time"
)

// AuditSink receives the audit events the engine publishes. An embedder adds one with the engine's
// WithAuditSink option. Without a sink the engine writes no audit output.
type AuditSink interface {
	// Handle receives one audit event. It runs on the sink's own queue, so a slow sink never delays a
	// request or another sink, and a full queue drops events. A returned error is logged and the event
	// is not retried. The event is the sink's own copy and may be changed.
	Handle(ctx context.Context, event AuditEvent) error
}

// AuditEvent is the envelope and audit payload of one event, as plain values.
type AuditEvent struct {
	// ID is unique per event, a UUID v7 so that IDs sort by time.
	ID string
	// Name identifies the attempted action, for example user.update.
	Name string
	// Time is when the event was published, in UTC.
	Time time.Time
	// TraceID is the correlation ID of the request. A caller can supply it, so it is not trusted.
	TraceID string
	// Actor is the principal that made the attempt. An anonymous actor has no ID.
	Actor AuditRef
	// OnBehalfOf is the principal the actor acted for, empty when it acted for itself.
	OnBehalfOf AuditRef
	// Subject is the principal whose record or access the attempt concerned, empty when none is known.
	Subject AuditRef
	// Target is the resource the attempt was made on. On a failed attempt its ID is caller-chosen text.
	Target AuditRef
	// Action is what the actor attempted, for example update.
	Action string
	// Outcome is how the attempt ended.
	Outcome AuditOutcome
	// Client is where the request came from, empty unless the peer was a public address.
	Client AuditClient
	// Details holds the event's allowlisted detail keys. It never holds a credential, token or secret.
	Details map[string]string
}

// AuditRef names a principal or a resource by type and, where one exists, ID.
type AuditRef struct {
	Type string
	ID   string
}

// AuditOutcome is how an attempt ended. Result is succeeded, denied or failed. Reason is a stable code,
// empty only when the attempt succeeded.
type AuditOutcome struct {
	Result string
	Reason string
}

// AuditClient is where a request came from.
type AuditClient struct {
	// Address is the TCP peer address.
	Address string
	// UserAgent is the request's user agent, capped at 256 characters.
	UserAgent string
}
