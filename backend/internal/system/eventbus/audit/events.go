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
	// EventClientAuthenticate names a refused attempt by an OAuth client to authenticate.
	EventClientAuthenticate = "client.authenticate"
	// EventGatewayApply names an attempt to apply a configuration version to a gateway.
	EventGatewayApply = "gateway.apply"
	// EventGatewayRevert names an attempt to return a gateway to the configuration it held before.
	EventGatewayRevert = "gateway.revert"
	// EventConnectionCreate names an attempt to create a connection.
	EventConnectionCreate = "connection.create"
	// EventConnectionUpdate names an attempt to change a connection.
	EventConnectionUpdate = "connection.update"
	// EventConnectionDelete names an attempt to delete a connection.
	EventConnectionDelete = "connection.delete"
	// EventResourceServerCreate names an attempt to create a resource server.
	EventResourceServerCreate = "resource_server.create"
	// EventResourceServerUpdate names an attempt to change a resource server.
	EventResourceServerUpdate = "resource_server.update"
	// EventResourceServerDelete names an attempt to delete a resource server.
	EventResourceServerDelete = "resource_server.delete"
	// EventResourceServerResourceCreate names an attempt to create a resource of a resource server.
	EventResourceServerResourceCreate = "resource_server.resource.create"
	// EventResourceServerResourceUpdate names an attempt to change a resource of a resource server.
	EventResourceServerResourceUpdate = "resource_server.resource.update"
	// EventResourceServerResourceDelete names an attempt to delete a resource of a resource server.
	EventResourceServerResourceDelete = "resource_server.resource.delete"
	// EventResourceServerActionCreate names an attempt to create an action of a resource server or of one of
	// its resources.
	EventResourceServerActionCreate = "resource_server.action.create"
	// EventResourceServerActionUpdate names an attempt to change an action of a resource server or of one of
	// its resources.
	EventResourceServerActionUpdate = "resource_server.action.update"
	// EventResourceServerActionDelete names an attempt to delete an action of a resource server or of one of
	// its resources.
	EventResourceServerActionDelete = "resource_server.action.delete"
	// EventConfigurationImport names an attempt to import configuration, as one event for the whole
	// request; each resource it writes is recorded by its own service.
	EventConfigurationImport = "configuration.import"
	// EventConfigurationDelete names an attempt to delete a declarative resource file.
	EventConfigurationDelete = "configuration.delete"
	// EventConfigurationExport names an attempt to export configuration, which can carry secrets.
	EventConfigurationExport = "configuration.export"
	// EventNotificationSenderCreate names an attempt to create a notification sender.
	EventNotificationSenderCreate = "notification_sender.create"
	// EventNotificationSenderUpdate names an attempt to change a notification sender.
	EventNotificationSenderUpdate = "notification_sender.update"
	// EventNotificationSenderDelete names an attempt to delete a notification sender.
	EventNotificationSenderDelete = "notification_sender.delete"
	// EventNotificationTemplateCreate names an attempt to create a notification template.
	EventNotificationTemplateCreate = "notification_template.create"
	// EventNotificationTemplateUpdate names an attempt to change a notification template.
	EventNotificationTemplateUpdate = "notification_template.update"
	// EventNotificationTemplateDelete names an attempt to delete a notification template.
	EventNotificationTemplateDelete = "notification_template.delete"
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
	// EventTokenIssue names a token request.
	EventTokenIssue = "token.issue"
	// EventTokenRevoke names a token revocation request.
	EventTokenRevoke = "token.revoke"
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
	// TargetTypeConnection is the target type of an event about a connection of any kind: an identity
	// provider, an AuthZEN PDP or an HTTP endpoint.
	TargetTypeConnection = "connection"
	// TargetTypeResourceServer is the target type of an event about a resource server.
	TargetTypeResourceServer = "resource_server"
	// TargetTypeResource is the target type of an event about a resource of a resource server.
	TargetTypeResource = "resource"
	// TargetTypeAction is the target type of an event about an action of a resource server or of one of its
	// resources.
	TargetTypeAction = "action"
	// TargetTypeConfiguration is the target type of an event about the configuration as a whole, which
	// has no ID.
	TargetTypeConfiguration = "configuration"
	// TargetTypeAgent is the target type of an event about an agent.
	TargetTypeAgent = "agent"
	// TargetTypeAgentType is the target type of an event about an agent type.
	TargetTypeAgentType = "agent_type"
	// TargetTypeGateway is the target type of an event about a gateway.
	TargetTypeGateway = "gateway"
	// TargetTypeNotificationSender is the target type of an event about a notification sender.
	TargetTypeNotificationSender = "notification_sender"
	// TargetTypeNotificationTemplate is the target type of an event about a notification template.
	TargetTypeNotificationTemplate = "notification_template"
	// TargetTypeRole is the target type of an event about a role.
	TargetTypeRole = "role"
	// TargetTypeGroup is the target type of an event about a group.
	TargetTypeGroup = "group"
	// TargetTypeServerConfig is the target type of an event about a section of the server configuration,
	// identified by the section name.
	TargetTypeServerConfig = "server_config"
	// TargetTypeToken is the target type of an event about a token. It never carries the token.
	TargetTypeToken = "token"
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

// DetailGrantType is the detail key of the grant type a token was requested with.
const DetailGrantType = "grant_type"

// DetailDryRun records whether an import only predicted its outcome, as "true" or "false".
const DetailDryRun = "dry_run"

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

	EventFlowCreate:         nil,
	EventFlowUpdate:         nil,
	EventFlowDelete:         nil,
	EventFlowRestore:        nil,
	EventClientAuthenticate: nil,

	EventGatewayApply:     nil,
	EventGatewayRevert:    nil,
	EventConnectionCreate: nil,
	EventConnectionUpdate: nil,
	EventConnectionDelete: nil,

	EventResourceServerCreate:         nil,
	EventResourceServerUpdate:         nil,
	EventResourceServerDelete:         nil,
	EventResourceServerResourceCreate: nil,
	EventResourceServerResourceUpdate: nil,
	EventResourceServerResourceDelete: nil,
	EventResourceServerActionCreate:   nil,
	EventResourceServerActionUpdate:   nil,
	EventResourceServerActionDelete:   nil,

	EventConfigurationImport: {DetailDryRun},
	EventConfigurationDelete: nil,
	EventConfigurationExport: nil,

	EventNotificationSenderCreate: nil,
	EventNotificationSenderUpdate: nil,
	EventNotificationSenderDelete: nil,

	EventNotificationTemplateCreate: nil,
	EventNotificationTemplateUpdate: nil,
	EventNotificationTemplateDelete: nil,

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
	EventTokenIssue:         {DetailGrantType},
	EventTokenRevoke:        nil,
	EventLayoutCreate:       nil,
	EventLayoutUpdate:       nil,
	EventLayoutDelete:       nil,

	EventThemeCreate: nil,
	EventThemeUpdate: nil,
	EventThemeDelete: nil,
}
