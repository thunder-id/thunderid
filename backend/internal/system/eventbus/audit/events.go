// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// Audited event names. A publisher uses these constants, and auditedEvents lists what each may carry.
const (
	// EventAgentCreate names an attempt to create an agent.
	EventAgentCreate = "agent.create"
	// EventAgentUpdate names an attempt to change an agent.
	EventAgentUpdate = "agent.update"
	// EventAgentDelete names an attempt to delete an agent.
	EventAgentDelete = "agent.delete"
	// EventAgentTypeCreate names an attempt to create an agent type.
	EventAgentTypeCreate = "agent_type.create"
	// EventAgentTypeUpdate names an attempt to change an agent type.
	EventAgentTypeUpdate = "agent_type.update"
	// EventAgentTypeDelete names an attempt to delete an agent type.
	EventAgentTypeDelete = "agent_type.delete"
	// EventUserCreate names an attempt to create a user.
	EventUserCreate = "user.create"
	// EventUserUpdate names an attempt to change a user.
	EventUserUpdate = "user.update"
	// EventUserDelete names an attempt to delete a user.
	EventUserDelete = "user.delete"
	// EventUserTypeCreate names an attempt to create a user type.
	EventUserTypeCreate = "user_type.create"
	// EventUserTypeUpdate names an attempt to change a user type.
	EventUserTypeUpdate = "user_type.update"
	// EventUserTypeDelete names an attempt to delete a user type.
	EventUserTypeDelete = "user_type.delete"
	// EventApplicationCreate names an attempt to create an application.
	EventApplicationCreate = "application.create"
	// EventApplicationUpdate names an attempt to change an application.
	EventApplicationUpdate = "application.update"
	// EventApplicationDelete names an attempt to delete an application.
	EventApplicationDelete = "application.delete"
	// EventFlowCreate names an attempt to create a flow.
	EventFlowCreate = "flow.create"
	// EventFlowUpdate names an attempt to change a flow.
	EventFlowUpdate = "flow.update"
	// EventFlowDelete names an attempt to delete a flow.
	EventFlowDelete = "flow.delete"
	// EventFlowRestore names an attempt to make an earlier version of a flow its current one.
	EventFlowRestore = "flow.restore"
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
	// EventServerConfigUpdate names an attempt to change a section of the server configuration.
	EventServerConfigUpdate = "server_config.update"
	// EventLayoutCreate names an attempt to create a layout.
	EventLayoutCreate = "layout.create"
	// EventLayoutUpdate names an attempt to change a layout.
	EventLayoutUpdate = "layout.update"
	// EventLayoutDelete names an attempt to delete a layout.
	EventLayoutDelete = "layout.delete"
	// EventThemeCreate names an attempt to create a theme.
	EventThemeCreate = "theme.create"
	// EventThemeUpdate names an attempt to change a theme.
	EventThemeUpdate = "theme.update"
	// EventThemeDelete names an attempt to delete a theme.
	EventThemeDelete = "theme.delete"
)

const (
	// TargetTypeUser is the target type of an event about a user.
	TargetTypeUser = "user"
	// TargetTypeUserType is the target type of an event about a user type.
	TargetTypeUserType = "user_type"
	// TargetTypeApplication is the target type of an event about an application.
	TargetTypeApplication = "application"
	// TargetTypeFlow is the target type of an event about a flow.
	TargetTypeFlow = "flow"
	// TargetTypeAgent is the target type of an event about an agent.
	TargetTypeAgent = "agent"
	// TargetTypeAgentType is the target type of an event about an agent type.
	TargetTypeAgentType = "agent_type"
	// TargetTypeRole is the target type of an event about a role.
	TargetTypeRole = "role"
	// TargetTypeGroup is the target type of an event about a group.
	TargetTypeGroup = "group"
	// TargetTypeServerConfig is the target type of an event about a section of the server configuration,
	// identified by the section name.
	TargetTypeServerConfig = "server_config"
	// TargetTypeLayout is the target type of an event about a layout.
	TargetTypeLayout = "layout"
	// TargetTypeTheme is the target type of an event about a theme.
	TargetTypeTheme = "theme"
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
	EventAgentCreate: nil,
	EventAgentUpdate: nil,
	EventAgentDelete: nil,

	EventAgentTypeCreate: nil,
	EventAgentTypeUpdate: nil,
	EventAgentTypeDelete: nil,

	EventUserCreate: nil,
	EventUserUpdate: nil,
	EventUserDelete: nil,

	EventUserTypeCreate: nil,
	EventUserTypeUpdate: nil,
	EventUserTypeDelete: nil,

	EventApplicationCreate: nil,
	EventApplicationUpdate: nil,
	EventApplicationDelete: nil,

	EventFlowCreate:  nil,
	EventFlowUpdate:  nil,
	EventFlowDelete:  nil,
	EventFlowRestore: nil,

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

	EventServerConfigUpdate: nil,
	EventLayoutCreate:       nil,
	EventLayoutUpdate:       nil,
	EventLayoutDelete:       nil,

	EventThemeCreate: nil,
	EventThemeUpdate: nil,
	EventThemeDelete: nil,
}
