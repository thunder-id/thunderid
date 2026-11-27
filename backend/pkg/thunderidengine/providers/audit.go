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
	Actor AuditActor
	// OnBehalfOf is the principal the actor acted for, empty when it acted for itself.
	OnBehalfOf AuditRef
	// Subject is the principal whose record or access the attempt concerned, empty when none is known.
	Subject AuditRef
	// Target is the resource the attempt was made on. On a failed attempt its ID is caller-chosen text.
	Target AuditRef
	// Action is what the actor attempted, for example update.
	Action AuditAction
	// Outcome is how the attempt ended.
	Outcome AuditOutcome
	// Client is where the request came from. The engine does not populate it, because it owns no HTTP
	// layer, so it is empty for an embedder, whose own HTTP layer sees the peer.
	Client AuditClient
	// Details holds the event's allowlisted detail keys. It never holds a credential, token or secret.
	Details map[string]string
}

// AuditRef names a principal or a resource by type and, where one exists, ID.
type AuditRef struct {
	Type string
	ID   string
}

// AuditActor is the principal that made an attempt. An anonymous actor has no ID.
type AuditActor struct {
	Type AuditActorType
	ID   string
}

// AuditOutcome is how an attempt ended. Reason is a stable code, empty only when the attempt succeeded.
type AuditOutcome struct {
	Result AuditResult
	Reason string
}

// AuditClient is where a request came from.
type AuditClient struct {
	// Address is the TCP peer address.
	Address string
	// UserAgent is the request's user agent, capped at 256 characters.
	UserAgent string
}

// AuditAction is what an actor attempted. The set is open, so a sink must accept a value it does not know.
type AuditAction string

const (
	// AuditActionAuthenticate proves an identity.
	AuditActionAuthenticate AuditAction = "authenticate"
	// AuditActionApply applies a configuration version to a resource.
	AuditActionApply AuditAction = "apply"
	// AuditActionCreate creates a resource.
	AuditActionCreate AuditAction = "create"
	// AuditActionExport reads configuration out as declarative resources.
	AuditActionExport AuditAction = "export"
	// AuditActionImport writes declarative resources into the configuration.
	AuditActionImport AuditAction = "import"
	// AuditActionUpdate changes a resource.
	AuditActionUpdate AuditAction = "update"
	// AuditActionDelete removes a resource.
	AuditActionDelete AuditAction = "delete"
	// AuditActionAdd adds a principal to a resource, as a role assignee or a group member.
	AuditActionAdd AuditAction = "add"
	// AuditActionRemove takes a principal off a resource, as a role assignee or a group member.
	AuditActionRemove AuditAction = "remove"
	// AuditActionRestore brings back an earlier version of a resource.
	AuditActionRestore AuditAction = "restore"
	// AuditActionIssue issues a credential, such as a token.
	AuditActionIssue AuditAction = "issue"
	// AuditActionRevoke withdraws a credential, such as a token.
	AuditActionRevoke AuditAction = "revoke"
	// AuditActionRevert returns a resource to the configuration it held before.
	AuditActionRevert AuditAction = "revert"
	// AuditActionEnroll registers a new credential, such as a passkey, for an identity.
	AuditActionEnroll AuditAction = "enroll"
	// AuditActionLock holds an identity from authenticating.
	AuditActionLock AuditAction = "lock"
	// AuditActionUnlock releases the locks on an identity.
	AuditActionUnlock AuditAction = "unlock"
	// AuditActionSuspend places an administrative hold on an identity.
	AuditActionSuspend AuditAction = "suspend"
	// AuditActionUnsuspend releases an administrative hold on an identity.
	AuditActionUnsuspend AuditAction = "unsuspend"
)

// AuditActorType is the kind of principal that acted. The set is open, so a sink must accept a value it
// does not know.
type AuditActorType string

const (
	// AuditActorTypeUser is a person.
	AuditActorTypeUser AuditActorType = "user"
	// AuditActorTypeApplication is an OAuth client acting for itself.
	AuditActorTypeApplication AuditActorType = "application"
	// AuditActorTypeAgent is an AI agent.
	AuditActorTypeAgent AuditActorType = "agent"
	// AuditActorTypeSystem is the deployment itself, such as its management API key.
	AuditActorTypeSystem AuditActorType = "system"
	// AuditActorTypeAnonymous is a caller that presented no verified identity.
	AuditActorTypeAnonymous AuditActorType = "anonymous"
)

// AuditResult is how an attempt ended.
type AuditResult string

const (
	// AuditResultSucceeded is an attempt that completed.
	AuditResultSucceeded AuditResult = "succeeded"
	// AuditResultDenied is an attempt a security decision refused.
	AuditResultDenied AuditResult = "denied"
	// AuditResultFailed is an attempt that was allowed but did not complete.
	AuditResultFailed AuditResult = "failed"
)
