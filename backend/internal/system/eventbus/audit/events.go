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
	// EventUserSharingPolicyCreate names an attempt to create a sharing policy of a user.
	EventUserSharingPolicyCreate = "user.sharing_policy.create"
	// EventUserSharingPolicyUpdate names an attempt to change a sharing policy of a user.
	EventUserSharingPolicyUpdate = "user.sharing_policy.update"
	// EventUserSharingPolicyDelete names an attempt to delete a sharing policy of a user.
	EventUserSharingPolicyDelete = "user.sharing_policy.delete"
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
	// EventApplicationSharingPolicyCreate names an attempt to create a sharing policy of an application.
	EventApplicationSharingPolicyCreate = "application.sharing_policy.create"
	// EventApplicationSharingPolicyUpdate names an attempt to change a sharing policy of an application.
	EventApplicationSharingPolicyUpdate = "application.sharing_policy.update"
	// EventApplicationSharingPolicyDelete names an attempt to delete a sharing policy of an application.
	EventApplicationSharingPolicyDelete = "application.sharing_policy.delete"
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
	// EventOUCreate names an attempt to create an organization unit.
	EventOUCreate = "ou.create"
	// EventOUUpdate names an attempt to change an organization unit.
	EventOUUpdate = "ou.update"
	// EventOUDelete names an attempt to delete an organization unit.
	EventOUDelete = "ou.delete"
	// EventProjectCreate names an attempt to create a project.
	EventProjectCreate = "project.create"
	// EventProjectUpdate names an attempt to change a project.
	EventProjectUpdate = "project.update"
	// EventProjectDelete names an attempt to delete a project.
	EventProjectDelete = "project.delete"
)

const (
	// TargetTypeUser is the target type of an event about a user.
	TargetTypeUser = "user"
	// TargetTypeUserSharingPolicy is the target type of an event about a sharing policy of a user.
	TargetTypeUserSharingPolicy = "user_sharing_policy"
	// TargetTypeUserType is the target type of an event about a user type.
	TargetTypeUserType = "user_type"
	// TargetTypeApplication is the target type of an event about an application.
	TargetTypeApplication = "application"
	// TargetTypeApplicationSharingPolicy is the target type of an event about a sharing policy of an
	// application.
	TargetTypeApplicationSharingPolicy = "application_sharing_policy"
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
	// TargetTypeOU is the target type of an event about an organization unit.
	TargetTypeOU = "ou"
	// TargetTypeProject is the target type of an event about a project.
	TargetTypeProject = "project"
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

	EventUserSharingPolicyCreate: nil,
	EventUserSharingPolicyUpdate: nil,
	EventUserSharingPolicyDelete: nil,

	EventUserTypeCreate: nil,
	EventUserTypeUpdate: nil,
	EventUserTypeDelete: nil,

	EventApplicationCreate: nil,
	EventApplicationUpdate: nil,
	EventApplicationDelete: nil,

	EventApplicationSharingPolicyCreate: nil,
	EventApplicationSharingPolicyUpdate: nil,
	EventApplicationSharingPolicyDelete: nil,

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

	EventOUCreate: nil,
	EventOUUpdate: nil,
	EventOUDelete: nil,

	EventProjectCreate: nil,
	EventProjectUpdate: nil,
	EventProjectDelete: nil,
}
