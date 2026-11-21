// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"regexp"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// CategoryAudit is the category of audit payloads: who did what to what, and with what outcome.
const CategoryAudit eventbus.Category = "audit"

// Action is what an actor attempted. The set is open, so a new action needs no change here.
type Action = providers.AuditAction

const (
	// ActionAuthenticate proves an identity.
	ActionAuthenticate = providers.AuditActionAuthenticate
	// ActionApply applies a configuration version to a resource.
	ActionApply = providers.AuditActionApply
	// ActionCreate creates a resource.
	ActionCreate = providers.AuditActionCreate
	// ActionExport reads configuration out as declarative resources.
	ActionExport = providers.AuditActionExport
	// ActionImport writes declarative resources into the configuration.
	ActionImport = providers.AuditActionImport
	// ActionUpdate changes a resource.
	ActionUpdate = providers.AuditActionUpdate
	// ActionDelete removes a resource.
	ActionDelete = providers.AuditActionDelete
	// ActionAdd adds a principal to a resource, as a role assignee or a group member.
	ActionAdd = providers.AuditActionAdd
	// ActionRemove takes a principal off a resource, as a role assignee or a group member.
	ActionRemove = providers.AuditActionRemove
	// ActionRestore brings back an earlier version of a resource.
	ActionRestore = providers.AuditActionRestore
	// ActionIssue issues a credential, such as a token.
	ActionIssue = providers.AuditActionIssue
	// ActionRevoke withdraws a credential, such as a token.
	ActionRevoke = providers.AuditActionRevoke
	// ActionRevert returns a resource to the configuration it held before.
	ActionRevert = providers.AuditActionRevert
	// ActionEnroll registers a new credential, such as a passkey, for an identity.
	ActionEnroll = providers.AuditActionEnroll
	// ActionLock holds an identity from authenticating.
	ActionLock = providers.AuditActionLock
	// ActionUnlock releases the locks on an identity.
	ActionUnlock = providers.AuditActionUnlock
	// ActionSuspend places an administrative hold on an identity.
	ActionSuspend = providers.AuditActionSuspend
	// ActionUnsuspend releases an administrative hold on an identity.
	ActionUnsuspend = providers.AuditActionUnsuspend
)

// ActorType is the kind of principal that acted. The set is open.
type ActorType = providers.AuditActorType

const (
	// ActorTypeUser is a person.
	ActorTypeUser = providers.AuditActorTypeUser
	// ActorTypeApplication is an OAuth client acting for itself.
	ActorTypeApplication = providers.AuditActorTypeApplication
	// ActorTypeAgent is an AI agent.
	ActorTypeAgent = providers.AuditActorTypeAgent
	// ActorTypeSystem is the deployment itself, such as its management API key.
	ActorTypeSystem = providers.AuditActorTypeSystem
	// ActorTypeAnonymous is a caller that presented no verified identity.
	ActorTypeAnonymous = providers.AuditActorTypeAnonymous
)

// Result is how an attempt ended.
type Result = providers.AuditResult

const (
	// ResultSucceeded is an attempt that completed.
	ResultSucceeded = providers.AuditResultSucceeded
	// ResultDenied is an attempt a security decision refused.
	ResultDenied = providers.AuditResultDenied
	// ResultFailed is an attempt that was allowed but did not complete.
	ResultFailed = providers.AuditResultFailed
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
