// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"regexp"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
)

// CategoryAudit is the category of audit payloads: who did what to what, and with what outcome.
const CategoryAudit eventbus.Category = "audit"

// Action is what an actor attempted. The set is open, so a new action needs no change here.
type Action string

const (
	// ActionCreate creates a resource.
	ActionCreate Action = "create"
	// ActionExport reads configuration out as declarative resources.
	ActionExport Action = "export"
	// ActionImport writes declarative resources into the configuration.
	ActionImport Action = "import"
	// ActionUpdate changes a resource.
	ActionUpdate Action = "update"
	// ActionDelete removes a resource.
	ActionDelete Action = "delete"
	// ActionAdd adds a principal to a resource, as a role assignee or a group member.
	ActionAdd Action = "add"
	// ActionRemove takes a principal off a resource, as a role assignee or a group member.
	ActionRemove Action = "remove"
	// ActionRestore brings back an earlier version of a resource.
	ActionRestore Action = "restore"
)

// ActorType is the kind of principal that acted. The set is open.
type ActorType string

const (
	// ActorTypeUser is a person.
	ActorTypeUser ActorType = "user"
	// ActorTypeApplication is an OAuth client acting for itself.
	ActorTypeApplication ActorType = "application"
	// ActorTypeAgent is an AI agent.
	ActorTypeAgent ActorType = "agent"
	// ActorTypeSystem is the deployment itself, such as its management API key.
	ActorTypeSystem ActorType = "system"
	// ActorTypeAnonymous is a caller that presented no verified identity.
	ActorTypeAnonymous ActorType = "anonymous"
)

// Result is how an attempt ended.
type Result string

const (
	// ResultSucceeded is an attempt that completed.
	ResultSucceeded Result = "succeeded"
	// ResultDenied is an attempt a security decision refused.
	ResultDenied Result = "denied"
	// ResultFailed is an attempt that was allowed but did not complete.
	ResultFailed Result = "failed"
)

const (
	// maxIDLength is the most characters of an actor, subject or target ID an event records.
	maxIDLength = 128

	// reasonForbidden is the reason of an attempt denied because the caller lacks the permission for it.
	reasonForbidden = "forbidden"
	// reasonServerError is the reason of a service error code that cannot make a valid reason.
	reasonServerError = "server_error"
	// reasonItemFailed is the reason of a request that completed with at least one of its items failed.
	reasonItemFailed = "item_failed"
)

// reasonPattern is the grammar of an outcome reason: a stable lowercase code, never free text.
var reasonPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
