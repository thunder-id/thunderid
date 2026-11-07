// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// Audited event names. A publisher uses these constants, and auditedEvents lists what each may carry.
const (
	// EventUserCreate names an attempt to create a user.
	EventUserCreate = "user.create"
	// EventUserUpdate names an attempt to change a user.
	EventUserUpdate = "user.update"
	// EventUserDelete names an attempt to delete a user.
	EventUserDelete = "user.delete"
	// EventApplicationCreate names an attempt to create an application.
	EventApplicationCreate = "application.create"
	// EventApplicationUpdate names an attempt to change an application.
	EventApplicationUpdate = "application.update"
	// EventApplicationDelete names an attempt to delete an application.
	EventApplicationDelete = "application.delete"
	// EventRoleCreate names an attempt to create a role.
	EventRoleCreate = "role.create"
	// EventRoleUpdate names an attempt to change a role.
	EventRoleUpdate = "role.update"
	// EventRoleDelete names an attempt to delete a role.
	EventRoleDelete = "role.delete"
	// EventRoleAssignmentAdd names an attempt to assign a role to principals.
	EventRoleAssignmentAdd = "role.assignment.add"
	// EventRoleAssignmentRemove names an attempt to take a role off principals.
	EventRoleAssignmentRemove = "role.assignment.remove"
	// EventGroupCreate names an attempt to create a group.
	EventGroupCreate = "group.create"
	// EventGroupUpdate names an attempt to change a group.
	EventGroupUpdate = "group.update"
	// EventGroupDelete names an attempt to delete a group.
	EventGroupDelete = "group.delete"
	// EventGroupMemberAdd names an attempt to add members to a group.
	EventGroupMemberAdd = "group.member.add"
	// EventGroupMemberRemove names an attempt to remove members from a group.
	EventGroupMemberRemove = "group.member.remove"
)

const (
	// TargetTypeUser is the target type of an event about a user.
	TargetTypeUser = "user"
	// TargetTypeApplication is the target type of an event about an application.
	TargetTypeApplication = "application"
	// TargetTypeAgent is the target type of an event about an agent.
	TargetTypeAgent = "agent"
	// TargetTypeRole is the target type of an event about a role.
	TargetTypeRole = "role"
	// TargetTypeGroup is the target type of an event about a group.
	TargetTypeGroup = "group"
)

// principalTargetTypes maps the type a role assignment or a group membership names its principal by to
// the target type of that principal.
var principalTargetTypes = map[string]string{
	string(providers.EntityCategoryUser):  TargetTypeUser,
	string(providers.EntityCategoryApp):   TargetTypeApplication,
	string(providers.EntityCategoryAgent): TargetTypeAgent,
	TargetTypeGroup:                       TargetTypeGroup,
}

// auditedEvents maps every audited event to the detail keys it may carry, so what is audited, and what
// each event may record, is reviewed in one place. An event not listed here is never written.
var auditedEvents = map[string][]string{
	EventUserCreate: nil,
	EventUserUpdate: nil,
	EventUserDelete: nil,

	EventApplicationCreate: nil,
	EventApplicationUpdate: nil,
	EventApplicationDelete: nil,

	EventRoleCreate:           nil,
	EventRoleUpdate:           nil,
	EventRoleDelete:           nil,
	EventRoleAssignmentAdd:    nil,
	EventRoleAssignmentRemove: nil,

	EventGroupCreate:       nil,
	EventGroupUpdate:       nil,
	EventGroupDelete:       nil,
	EventGroupMemberAdd:    nil,
	EventGroupMemberRemove: nil,
}
