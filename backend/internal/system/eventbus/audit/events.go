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
	// EventAgentCredentialUpdate names an attempt to set credentials of an agent. It never carries a
	// credential or its type.
	EventAgentCredentialUpdate = "agent.credential.update" //nolint:gosec // an event name, not a credential
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
	// EventUserCredentialEnroll names an attempt to enroll a credential for a user. It never carries
	// credential data.
	EventUserCredentialEnroll = "user.credential.enroll" //nolint:gosec // an event name, not a credential
	// EventUserCredentialUpdate names an attempt to set credentials of a user. It never carries a
	// credential or its type.
	EventUserCredentialUpdate = "user.credential.update" //nolint:gosec // an event name, not a credential
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
	// EventVariableCreate names an attempt to create a variable.
	EventVariableCreate = "variable.create"
	// EventVariableUpdate names an attempt to store a variable under a name, replacing the one held
	// there or creating it when the name is unused.
	EventVariableUpdate = "variable.update"
	// EventVariableDelete names an attempt to delete a variable.
	EventVariableDelete = "variable.delete"
	// EventSecretCreate names an attempt to create a secret.
	EventSecretCreate = "secret.create"
	// EventSecretUpdate names an attempt to store a secret under a name, rotating the one held there or
	// creating it when the name is unused.
	EventSecretUpdate = "secret.update"
	// EventSecretDelete names an attempt to delete a secret.
	EventSecretDelete = "secret.delete"
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
	// EventFlowSharingPolicyCreate names an attempt to create a sharing policy of a flow.
	EventFlowSharingPolicyCreate = "flow.sharing_policy.create"
	// EventFlowSharingPolicyUpdate names an attempt to change a sharing policy of a flow.
	EventFlowSharingPolicyUpdate = "flow.sharing_policy.update"
	// EventFlowSharingPolicyDelete names an attempt to delete a sharing policy of a flow.
	EventFlowSharingPolicyDelete = "flow.sharing_policy.delete"
	// EventFlowVariablesUpdate names an attempt to set the values an organization unit holds for the
	// variables of a flow.
	EventFlowVariablesUpdate = "flow.variables.update"
	// EventClientAuthenticate names a refused attempt by an OAuth client to authenticate.
	EventClientAuthenticate = "client.authenticate"
	// EventGatewayCreate names an attempt to register a gateway.
	EventGatewayCreate = "gateway.create"
	// EventGatewayUpdate names an attempt to change the registration of a gateway.
	EventGatewayUpdate = "gateway.update"
	// EventGatewayDelete names an attempt to remove the registration of a gateway.
	EventGatewayDelete = "gateway.delete"
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
	// EventConsentCreate names an attempt to record a user's consent decisions as a new consent.
	EventConsentCreate = "consent.create"
	// EventConsentUpdate names an attempt to replace the decisions a consent holds.
	EventConsentUpdate = "consent.update"
	// EventConnectionSharingPolicyCreate names an attempt to create a sharing policy of a connection.
	EventConnectionSharingPolicyCreate = "connection.sharing_policy.create"
	// EventConnectionSharingPolicyUpdate names an attempt to change a sharing policy of a connection.
	EventConnectionSharingPolicyUpdate = "connection.sharing_policy.update"
	// EventConnectionSharingPolicyDelete names an attempt to delete a sharing policy of a connection.
	EventConnectionSharingPolicyDelete = "connection.sharing_policy.delete"
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
	// EventConfigurationVersionCreate names an attempt to capture the configuration as a version.
	EventConfigurationVersionCreate = "configuration_version.create"
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
	// EventRoleSharingPolicyCreate names an attempt to create a sharing policy of a role.
	EventRoleSharingPolicyCreate = "role.sharing_policy.create"
	// EventRoleSharingPolicyUpdate names an attempt to change a sharing policy of a role.
	EventRoleSharingPolicyUpdate = "role.sharing_policy.update"
	// EventRoleSharingPolicyDelete names an attempt to delete a sharing policy of a role.
	EventRoleSharingPolicyDelete = "role.sharing_policy.delete"
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
	// EventUserLock names a lock that failed authentication attempts placed on a user.
	EventUserLock = "user.lock"
	// EventUserUnlock names an attempt to release every automatic lock on a user.
	EventUserUnlock = "user.unlock"
	// EventUserSuspend names an attempt to suspend a user.
	EventUserSuspend = "user.suspend"
	// EventUserUnsuspend names an attempt to release the suspension of a user.
	EventUserUnsuspend = "user.unsuspend"
	// EventAgentLock names a lock that failed authentication attempts placed on an agent.
	EventAgentLock = "agent.lock"
	// EventAgentUnlock names an attempt to release every automatic lock on an agent.
	EventAgentUnlock = "agent.unlock"
	// EventAgentSuspend names an attempt to suspend an agent.
	EventAgentSuspend = "agent.suspend"
	// EventAgentUnsuspend names an attempt to release the suspension of an agent.
	EventAgentUnsuspend = "agent.unsuspend"
	// EventApplicationLock names a lock that failed authentication attempts placed on an application.
	EventApplicationLock = "application.lock"
	// EventApplicationUnlock names an attempt to release every automatic lock on an application.
	EventApplicationUnlock = "application.unlock"
	// EventApplicationSuspend names an attempt to suspend an application.
	EventApplicationSuspend = "application.suspend"
	// EventApplicationUnsuspend names an attempt to release the suspension of an application.
	EventApplicationUnsuspend = "application.unsuspend"
)

const (
	// TargetTypeUser is the target type of an event about a user.
	TargetTypeUser = "user"
	// TargetTypeUserSharingPolicy is the target type of an event about a sharing policy of a user.
	TargetTypeUserSharingPolicy = "user_sharing_policy"
	// TargetTypeUserType is the target type of an event about a user type.
	TargetTypeUserType = "user_type"
	// TargetTypeVariable is the target type of an event about a variable, identified by its name. It
	// never carries the value.
	TargetTypeVariable = "variable"
	// TargetTypeSecret is the target type of an event about a secret, identified by its name. It never
	// carries the value.
	TargetTypeSecret = "secret"
	// TargetTypeApplication is the target type of an event about an application.
	TargetTypeApplication = "application"
	// TargetTypeApplicationSharingPolicy is the target type of an event about a sharing policy of an
	// application.
	TargetTypeApplicationSharingPolicy = "application_sharing_policy"
	// TargetTypeFlow is the target type of an event about a flow.
	TargetTypeFlow = "flow"
	// TargetTypeFlowSharingPolicy is the target type of an event about a sharing policy of a flow.
	TargetTypeFlowSharingPolicy = "flow_sharing_policy"
	// TargetTypeConnection is the target type of an event about a connection of any kind: an identity
	// provider, an AuthZEN PDP or an HTTP endpoint.
	TargetTypeConnection = "connection"
	// TargetTypeConsent is the target type of an event about a consent. It never carries the purposes or
	// elements the consent holds.
	TargetTypeConsent = "consent"
	// TargetTypeConnectionSharingPolicy is the target type of an event about a sharing policy of a
	// connection.
	TargetTypeConnectionSharingPolicy = "connection_sharing_policy"
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
	// TargetTypeConfigurationVersion is the target type of an event about a captured version of the
	// configuration, identified by its hash.
	TargetTypeConfigurationVersion = "configuration_version"
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
	// TargetTypeRoleSharingPolicy is the target type of an event about a sharing policy of a role.
	TargetTypeRoleSharingPolicy = "role_sharing_policy"
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

// DetailGrantType is the detail key of the grant type a token was requested with.
const DetailGrantType = "grant_type"

// DetailDryRun records whether an import only predicted its outcome, as "true" or "false".
const DetailDryRun = "dry_run"

// DetailAuthenticationMethod is the detail key of the authentication method a lock holds.
const DetailAuthenticationMethod = "authentication_method"

// DetailAppliedBy is the detail key of who placed or lifted a suspension: organization, owner or cascade.
const DetailAppliedBy = "applied_by"

// DetailSelfService records whether an unlock was a recovery the identity completed itself, as "true" or
// "false".
const DetailSelfService = "self_service"

// auditedEvents maps every audited event to the detail keys it may carry, so what is audited, and what
// each event may record, is reviewed in one place. An event not listed here is never written.
var auditedEvents = map[string][]string{
	EventAgentCreate: nil,
	EventAgentUpdate: nil,
	EventAgentDelete: nil,

	EventAgentCredentialUpdate: nil,

	EventAgentTypeCreate: nil,
	EventAgentTypeUpdate: nil,
	EventAgentTypeDelete: nil,

	EventUserCreate: nil,
	EventUserUpdate: nil,
	EventUserDelete: nil,

	EventUserCredentialEnroll: nil,
	EventUserCredentialUpdate: nil,

	EventUserSharingPolicyCreate: nil,
	EventUserSharingPolicyUpdate: nil,
	EventUserSharingPolicyDelete: nil,

	EventUserTypeCreate: nil,
	EventUserTypeUpdate: nil,
	EventUserTypeDelete: nil,

	EventVariableCreate: nil,
	EventVariableUpdate: nil,
	EventVariableDelete: nil,

	EventSecretCreate: nil,
	EventSecretUpdate: nil,
	EventSecretDelete: nil,

	EventApplicationCreate: nil,
	EventApplicationUpdate: nil,
	EventApplicationDelete: nil,

	EventFlowCreate:         nil,
	EventFlowUpdate:         nil,
	EventFlowDelete:         nil,
	EventFlowRestore:        nil,
	EventClientAuthenticate: nil,

	EventGatewayCreate:           nil,
	EventGatewayUpdate:           nil,
	EventGatewayDelete:           nil,
	EventFlowSharingPolicyCreate: nil,
	EventFlowSharingPolicyUpdate: nil,
	EventFlowSharingPolicyDelete: nil,
	EventFlowVariablesUpdate:     nil,

	EventGatewayApply:     nil,
	EventGatewayRevert:    nil,
	EventConnectionCreate: nil,
	EventConnectionUpdate: nil,
	EventConnectionDelete: nil,

	EventConsentCreate:                 nil,
	EventConsentUpdate:                 nil,
	EventConnectionSharingPolicyCreate: nil,
	EventConnectionSharingPolicyUpdate: nil,
	EventConnectionSharingPolicyDelete: nil,

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

	EventConfigurationVersionCreate: nil,

	EventNotificationSenderCreate: nil,
	EventNotificationSenderUpdate: nil,
	EventNotificationSenderDelete: nil,

	EventNotificationTemplateCreate: nil,
	EventNotificationTemplateUpdate: nil,
	EventNotificationTemplateDelete: nil,

	EventApplicationSharingPolicyCreate: nil,
	EventApplicationSharingPolicyUpdate: nil,
	EventApplicationSharingPolicyDelete: nil,

	EventRoleCreate:           nil,
	EventRoleUpdate:           nil,
	EventRoleDelete:           nil,
	EventRoleAssignmentAdd:    nil,
	EventRoleAssignmentRemove: nil,

	EventRoleSharingPolicyCreate: nil,
	EventRoleSharingPolicyUpdate: nil,
	EventRoleSharingPolicyDelete: nil,

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

	EventOUCreate: nil,
	EventOUUpdate: nil,
	EventOUDelete: nil,

	EventProjectCreate: nil,
	EventProjectUpdate: nil,
	EventProjectDelete: nil,

	EventUserLock:      {DetailAuthenticationMethod},
	EventUserUnlock:    {DetailSelfService},
	EventUserSuspend:   {DetailAppliedBy},
	EventUserUnsuspend: {DetailAppliedBy},

	EventAgentLock:      {DetailAuthenticationMethod},
	EventAgentUnlock:    {DetailSelfService},
	EventAgentSuspend:   {DetailAppliedBy},
	EventAgentUnsuspend: {DetailAppliedBy},

	EventApplicationLock:      {DetailAuthenticationMethod},
	EventApplicationUnlock:    {DetailSelfService},
	EventApplicationSuspend:   {DetailAppliedBy},
	EventApplicationUnsuspend: {DetailAppliedBy},
}
