// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import "strings"

// publicPaths defines the list of public paths using glob patterns.
// - "*": Matches a single path segment (e.g., /a/*/b).
// - "**": Matches zero or more path segments (subpaths) at the end of the path (e.g., /a/**).
// Not allowed in the middle of the path (e.g., /a/**/b is invalid).
//
// The Direct API paths (/auth/**, /register/passkey/**, /access/**) are public here; their Direct
// Auth Secret gating is owned by the authn service (internal/authn).
var publicPaths = []string{
	"/ou/*/oauth2/token",
	"/ou/*/oauth2/authorize",
	"/health/**",
	"/flow/execute/**",
	"/flow/meta",
	"/oauth2/**",
	// OpenID4VP wallet- and RP-facing endpoints are public; management endpoints
	// (e.g. /openid4vp/presentation-definitions) are deliberately excluded.
	"/openid4vp/request",
	"/openid4vp/response",
	"/openid4vp/initiate",
	"/openid4vp/status/**",
	// OpenID4VCI wallet-facing endpoints are public; management endpoints
	// (e.g. /openid4vci/credential-configurations) are deliberately excluded.
	"/openid4vci/offer",
	"/openid4vci/credential-offer/**",
	"/openid4vci/nonce",
	"/openid4vci/credential",
	"/.well-known/authzen-configuration",
	"/.well-known/openid-configuration/**",
	"/.well-known/openid-credential-issuer",
	"/.well-known/oauth-authorization-server/**",
	"/.well-known/oauth-protected-resource",
	"/gate/**",
	"/console/**",
	"/error/**",
	"/design/resolve/**",
	"/i18n/languages",
	"/i18n/languages/*/translations/resolve",
	"/i18n/languages/*/translations/ns/*/keys/*/resolve",
	"/mcp/**", // MCP authorization is handled at MCP server handler.
	"/auth/**",
	"/register/passkey/**",
	"/access/**",
	// SCIM ServiceProviderConfig exposed without authentication.
	"/scim/v2/ServiceProviderConfig",
}

// ---- Resource types ----

// ResourceType defines the category of system resource being acted upon.
type ResourceType string

// ResourceType defines the category of system resource being acted upon.
const (
	// ResourceTypeOU identifies an organization unit resource.
	ResourceTypeOU ResourceType = "ou"
	// ResourceTypeUser identifies a user resource.
	ResourceTypeUser ResourceType = "user"
	// ResourceTypeGroup identifies a group resource.
	ResourceTypeGroup ResourceType = "group"
	// ResourceTypeAgent identifies an agent resource.
	ResourceTypeAgent ResourceType = "agent"
	// ResourceTypeUserType identifies a user-category entity type resource.
	ResourceTypeUserType ResourceType = "usertype"
	// ResourceTypeAgentType identifies an agent-category entity type resource.
	ResourceTypeAgentType ResourceType = "agenttype"
	// ResourceTypeNotificationTemplate identifies a notification template resource.
	ResourceTypeNotificationTemplate ResourceType = "notificationtemplate"
	// ResourceTypeApplication identifies an application resource.
	ResourceTypeApplication ResourceType = "app"
	// ResourceTypeRole identifies a role resource.
	ResourceTypeRole ResourceType = "role"
	// ResourceTypeResourceServer identifies a resource server resource.
	ResourceTypeResourceServer ResourceType = "rs"
	// ResourceTypeConnection identifies a connection resource.
	ResourceTypeConnection ResourceType = "connection"
	// ResourceTypeFlow identifies a flow resource.
	ResourceTypeFlow ResourceType = "flow"
	// ResourceTypeDesign identifies a design (theme or layout) resource.
	ResourceTypeDesign ResourceType = "design"
	// ResourceTypeI18n identifies a translation resource.
	ResourceTypeI18n ResourceType = "i18n"
	// ResourceTypeGateway identifies a gateway resource.
	ResourceTypeGateway ResourceType = "gateway"
	// ResourceTypeAuthorizationDetailType identifies an authorization detail type resource.
	ResourceTypeAuthorizationDetailType ResourceType = "authzdetail"
	// ResourceTypeVerifiableCredential identifies a verifiable credential configuration resource.
	ResourceTypeVerifiableCredential ResourceType = "vc"
	// ResourceTypeProject identifies a project resource.
	ResourceTypeProject ResourceType = "project"
	// ResourceTypeServerConfig identifies a server configuration resource.
	ResourceTypeServerConfig ResourceType = "serverconfig"
	// ResourceTypeConfigurationVersion identifies a configuration version resource.
	ResourceTypeConfigurationVersion ResourceType = "configversion"
)

// ---- Permission catalog ----

// Action handles declared under each catalog resource.
const (
	catalogActionView   = "view"
	catalogActionCreate = "create"
	catalogActionUpdate = "update"
	catalogActionDelete = "delete"
)

// crudActions are the actions declared on a catalog resource unless it lists its own.
var crudActions = []string{catalogActionView, catalogActionCreate, catalogActionUpdate, catalogActionDelete}

// serverConfigActions are the actions declared on server configurations, which can only be read and
// replaced.
var serverConfigActions = []string{catalogActionView, catalogActionUpdate}

// configVersionActions are the actions declared on configuration versions, which can only be read and
// captured.
var configVersionActions = []string{catalogActionView, catalogActionCreate}

// systemCatalog lists, in declaration order, each resource declared under the root system resource
// on the System resource server and the actions declared on it. It must match the System resource
// server in the bootstrap bundle (cmd/server/bootstrap/01-default-resources.yaml).
var systemCatalog = []struct {
	resource ResourceType
	actions  []string
}{
	{ResourceTypeOU, crudActions},
	{ResourceTypeUser, crudActions},
	{ResourceTypeGroup, crudActions},
	{ResourceTypeAgent, crudActions},
	{ResourceTypeUserType, crudActions},
	{ResourceTypeAgentType, crudActions},
	{ResourceTypeNotificationTemplate, crudActions},
	{ResourceTypeApplication, crudActions},
	{ResourceTypeRole, crudActions},
	{ResourceTypeResourceServer, crudActions},
	{ResourceTypeConnection, crudActions},
	{ResourceTypeFlow, crudActions},
	{ResourceTypeVerifiableCredential, crudActions},
	{ResourceTypeProject, crudActions},
	{ResourceTypeDesign, crudActions},
	{ResourceTypeI18n, crudActions},
	{ResourceTypeGateway, crudActions},
	{ResourceTypeAuthorizationDetailType, crudActions},
	{ResourceTypeServerConfig, serverConfigActions},
	{ResourceTypeConfigurationVersion, configVersionActions},
}

// ---- Actions ----

// Action defines a system operation that can be authorized.
type Action string

const (
	// ActionCreateOU creates a new organization unit.
	ActionCreateOU Action = "ou:create"
	// ActionReadOU reads an organization unit.
	ActionReadOU Action = "ou:read"
	// ActionUpdateOU updates an organization unit.
	ActionUpdateOU Action = "ou:update"
	// ActionDeleteOU deletes an organization unit.
	ActionDeleteOU Action = "ou:delete"
	// ActionListOUs lists organization units.
	ActionListOUs Action = "ou:list"
	// ActionListChildOUs lists child organization units of a parent OU.
	ActionListChildOUs Action = "ou:list-children"

	// ActionCreateUser creates a new user.
	ActionCreateUser Action = "user:create"
	// ActionReadUser reads a user.
	ActionReadUser Action = "user:read"
	// ActionUpdateUser updates a user.
	ActionUpdateUser Action = "user:update"
	// ActionDeleteUser deletes a user.
	ActionDeleteUser Action = "user:delete"
	// ActionListUsers lists users.
	ActionListUsers Action = "user:list"

	// ActionCreateGroup creates a new group.
	ActionCreateGroup Action = "group:create"
	// ActionReadGroup reads a group.
	ActionReadGroup Action = "group:read"
	// ActionUpdateGroup updates a group.
	ActionUpdateGroup Action = "group:update"
	// ActionDeleteGroup deletes a group.
	ActionDeleteGroup Action = "group:delete"
	// ActionListGroups lists groups.
	ActionListGroups Action = "group:list"

	// ActionCreateAgent creates a new agent.
	ActionCreateAgent Action = "agent:create"
	// ActionReadAgent reads an agent.
	ActionReadAgent Action = "agent:read"
	// ActionUpdateAgent updates an agent.
	ActionUpdateAgent Action = "agent:update"
	// ActionDeleteAgent deletes an agent.
	ActionDeleteAgent Action = "agent:delete"
	// ActionListAgents lists agents.
	ActionListAgents Action = "agent:list"

	// ActionCreateUserType creates a new user type.
	ActionCreateUserType Action = "usertype:create"
	// ActionReadUserType reads a user type.
	ActionReadUserType Action = "usertype:read"
	// ActionUpdateUserType updates a user type.
	ActionUpdateUserType Action = "usertype:update"
	// ActionDeleteUserType deletes a user type.
	ActionDeleteUserType Action = "usertype:delete"
	// ActionListUserTypes lists user types.
	ActionListUserTypes Action = "usertype:list"

	// ActionCreateAgentType creates a new agent type.
	ActionCreateAgentType Action = "agenttype:create"
	// ActionReadAgentType reads an agent type.
	ActionReadAgentType Action = "agenttype:read"
	// ActionUpdateAgentType updates an agent type.
	ActionUpdateAgentType Action = "agenttype:update"
	// ActionDeleteAgentType deletes an agent type.
	ActionDeleteAgentType Action = "agenttype:delete"
	// ActionListAgentTypes lists agent types.
	ActionListAgentTypes Action = "agenttype:list"

	// ActionCreateNotificationTemplate creates a new notification template.
	ActionCreateNotificationTemplate Action = "notificationtemplate:create"
	// ActionReadNotificationTemplate reads a notification template.
	ActionReadNotificationTemplate Action = "notificationtemplate:read"
	// ActionUpdateNotificationTemplate updates a notification template.
	ActionUpdateNotificationTemplate Action = "notificationtemplate:update"
	// ActionDeleteNotificationTemplate deletes a notification template.
	ActionDeleteNotificationTemplate Action = "notificationtemplate:delete"
	// ActionListNotificationTemplates lists notification templates.
	ActionListNotificationTemplates Action = "notificationtemplate:list"

	// ActionCreateFlow creates a new flow.
	ActionCreateFlow Action = "flow:create"
	// ActionReadFlow reads a flow.
	ActionReadFlow Action = "flow:read"
	// ActionUpdateFlow updates a flow.
	ActionUpdateFlow Action = "flow:update"
	// ActionDeleteFlow deletes a flow.
	ActionDeleteFlow Action = "flow:delete"
	// ActionListFlows lists flows.
	ActionListFlows Action = "flow:list"

	// ActionCreateRole creates a new role.
	ActionCreateRole Action = "role:create"
	// ActionReadRole reads a role.
	ActionReadRole Action = "role:read"
	// ActionUpdateRole updates a role.
	ActionUpdateRole Action = "role:update"
	// ActionDeleteRole deletes a role.
	ActionDeleteRole Action = "role:delete"
	// ActionListRoles lists roles.
	ActionListRoles Action = "role:list"

	// ActionCreateResourceServer creates a new resource server.
	ActionCreateResourceServer Action = "rs:create"
	// ActionReadResourceServer reads a resource server.
	ActionReadResourceServer Action = "rs:read"
	// ActionUpdateResourceServer updates a resource server.
	ActionUpdateResourceServer Action = "rs:update"
	// ActionDeleteResourceServer deletes a resource server.
	ActionDeleteResourceServer Action = "rs:delete"
	// ActionListResourceServers lists resource servers.
	ActionListResourceServers Action = "rs:list"

	// ActionCreateConnection creates a new connection.
	ActionCreateConnection Action = "connection:create"
	// ActionReadConnection reads a connection.
	ActionReadConnection Action = "connection:read"
	// ActionUpdateConnection updates a connection.
	ActionUpdateConnection Action = "connection:update"
	// ActionDeleteConnection deletes a connection.
	ActionDeleteConnection Action = "connection:delete"
	// ActionListConnections lists connections.
	ActionListConnections Action = "connection:list"

	// ActionCreateVC creates a new verifiable credential configuration.
	ActionCreateVC Action = "vc:create"
	// ActionReadVC reads a verifiable credential configuration.
	ActionReadVC Action = "vc:read"
	// ActionUpdateVC updates a verifiable credential configuration.
	ActionUpdateVC Action = "vc:update"
	// ActionDeleteVC deletes a verifiable credential configuration.
	ActionDeleteVC Action = "vc:delete"
	// ActionListVCs lists verifiable credential configurations.
	ActionListVCs Action = "vc:list"

	// ActionCreateDesign creates a new theme or layout.
	ActionCreateDesign Action = "design:create"
	// ActionReadDesign reads a theme or layout.
	ActionReadDesign Action = "design:read"
	// ActionUpdateDesign updates a theme or layout.
	ActionUpdateDesign Action = "design:update"
	// ActionDeleteDesign deletes a theme or layout.
	ActionDeleteDesign Action = "design:delete"
	// ActionListDesigns lists themes or layouts.
	ActionListDesigns Action = "design:list"

	// ActionCreateI18n creates a new translation.
	ActionCreateI18n Action = "i18n:create"
	// ActionReadI18n reads a translation.
	ActionReadI18n Action = "i18n:read"
	// ActionUpdateI18n updates a translation.
	ActionUpdateI18n Action = "i18n:update"
	// ActionDeleteI18n deletes a translation.
	ActionDeleteI18n Action = "i18n:delete"
	// ActionListI18n lists translations.
	ActionListI18n Action = "i18n:list"

	// ActionCreateGateway creates a new gateway.
	ActionCreateGateway Action = "gateway:create"
	// ActionReadGateway reads a gateway.
	ActionReadGateway Action = "gateway:read"
	// ActionUpdateGateway updates a gateway.
	ActionUpdateGateway Action = "gateway:update"
	// ActionDeleteGateway deletes a gateway.
	ActionDeleteGateway Action = "gateway:delete"
	// ActionListGateways lists gateways.
	ActionListGateways Action = "gateway:list"

	// ActionReadServerConfig reads a server configuration.
	ActionReadServerConfig Action = "serverconfig:read"
	// ActionUpdateServerConfig updates a server configuration.
	ActionUpdateServerConfig Action = "serverconfig:update"
	// ActionListServerConfigs lists server configurations.
	ActionListServerConfigs Action = "serverconfig:list"

	// ActionCreateConfigVersion captures a new configuration version.
	ActionCreateConfigVersion Action = "configversion:create"
	// ActionReadConfigVersion reads a configuration version.
	ActionReadConfigVersion Action = "configversion:read"
	// ActionListConfigVersions lists configuration versions.
	ActionListConfigVersions Action = "configversion:list"

	// ActionCreateAuthzDetail creates a new authorization detail type.
	ActionCreateAuthzDetail Action = "authzdetail:create"
	// ActionReadAuthzDetail reads an authorization detail type.
	ActionReadAuthzDetail Action = "authzdetail:read"
	// ActionUpdateAuthzDetail updates an authorization detail type.
	ActionUpdateAuthzDetail Action = "authzdetail:update"
	// ActionDeleteAuthzDetail deletes an authorization detail type.
	ActionDeleteAuthzDetail Action = "authzdetail:delete"
	// ActionListAuthzDetails lists authorization detail types.
	ActionListAuthzDetails Action = "authzdetail:list"
)

// ---- Permissions ----

// systemResourceServerID is the ID the bootstrap bundle gives the System resource server.
const systemResourceServerID = "01900000-0000-7000-8000-000000000020"

// SystemPermissions holds the runtime-resolved permission strings for the system resource server.
// All values are set by InitSystemPermissions and must not be used before it is called.
type SystemPermissions struct {
	Root                       string
	OU                         string
	OUView                     string
	User                       string
	UserView                   string
	Group                      string
	GroupView                  string
	Agent                      string
	AgentView                  string
	UserType                   string
	UserTypeView               string
	AgentType                  string
	AgentTypeView              string
	NotificationTemplate       string
	NotificationTemplateView   string
	FlowView                   string
	FlowCreate                 string
	FlowUpdate                 string
	FlowDelete                 string
	OUCreate                   string
	OUUpdate                   string
	OUDelete                   string
	UserCreate                 string
	UserUpdate                 string
	UserDelete                 string
	GroupCreate                string
	GroupUpdate                string
	GroupDelete                string
	AgentCreate                string
	AgentUpdate                string
	AgentDelete                string
	UserTypeCreate             string
	UserTypeUpdate             string
	UserTypeDelete             string
	AgentTypeCreate            string
	AgentTypeUpdate            string
	AgentTypeDelete            string
	NotificationTemplateCreate string
	NotificationTemplateUpdate string
	NotificationTemplateDelete string
	RoleView                   string
	RoleCreate                 string
	RoleUpdate                 string
	RoleDelete                 string
	ResourceServerView         string
	ResourceServerCreate       string
	ResourceServerUpdate       string
	ResourceServerDelete       string
	ConnectionView             string
	ConnectionCreate           string
	ConnectionUpdate           string
	ConnectionDelete           string
	VCView                     string
	VCCreate                   string
	VCUpdate                   string
	VCDelete                   string
	ProjectView                string
	ProjectCreate              string
	ProjectUpdate              string
	ProjectDelete              string
	DesignView                 string
	DesignCreate               string
	DesignUpdate               string
	DesignDelete               string
	I18nView                   string
	I18nCreate                 string
	I18nUpdate                 string
	I18nDelete                 string
	GatewayView                string
	GatewayCreate              string
	GatewayUpdate              string
	GatewayDelete              string
	ServerConfigView           string
	ServerConfigUpdate         string
	ConfigVersionView          string
	ConfigVersionCreate        string
	AuthzDetailView            string
	AuthzDetailCreate          string
	AuthzDetailUpdate          string
	AuthzDetailDelete          string
	// Catalog holds every permission declared under Root, in declaration order: each resource
	// followed by its actions.
	Catalog []string
}

// sysPerms holds the active system permissions, initialized by InitSystemPermissions.
var sysPerms *SystemPermissions

// buildPermission constructs a permission string by joining non-empty parts with ":".
func buildPermission(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, ":")
}

// InitSystemPermissions initializes the system permission strings using the given handle prefix.
// When handle is empty, permissions match the legacy values ("system", "system:ou", etc.).
// When handle is non-empty (e.g. "mgmt"), permissions are prefixed ("mgmt:system", "mgmt:system:ou", etc.).
// This function must be called once at startup before any service or middleware uses permissions.
func InitSystemPermissions(handle string) {
	p := &SystemPermissions{
		Root:                       buildPermission(handle, "system"),
		OU:                         buildPermission(handle, "system", "ou"),
		OUView:                     buildPermission(handle, "system", "ou", "view"),
		User:                       buildPermission(handle, "system", "user"),
		UserView:                   buildPermission(handle, "system", "user", "view"),
		Group:                      buildPermission(handle, "system", "group"),
		GroupView:                  buildPermission(handle, "system", "group", "view"),
		Agent:                      buildPermission(handle, "system", "agent"),
		AgentView:                  buildPermission(handle, "system", "agent", "view"),
		UserType:                   buildPermission(handle, "system", "usertype"),
		UserTypeView:               buildPermission(handle, "system", "usertype", "view"),
		AgentType:                  buildPermission(handle, "system", "agenttype"),
		AgentTypeView:              buildPermission(handle, "system", "agenttype", "view"),
		NotificationTemplate:       buildPermission(handle, "system", "notificationtemplate"),
		NotificationTemplateView:   buildPermission(handle, "system", "notificationtemplate", "view"),
		FlowView:                   buildPermission(handle, "system", "flow", catalogActionView),
		FlowCreate:                 buildPermission(handle, "system", "flow", catalogActionCreate),
		FlowUpdate:                 buildPermission(handle, "system", "flow", catalogActionUpdate),
		FlowDelete:                 buildPermission(handle, "system", "flow", catalogActionDelete),
		OUCreate:                   buildPermission(handle, "system", "ou", catalogActionCreate),
		OUUpdate:                   buildPermission(handle, "system", "ou", catalogActionUpdate),
		OUDelete:                   buildPermission(handle, "system", "ou", catalogActionDelete),
		UserCreate:                 buildPermission(handle, "system", "user", catalogActionCreate),
		UserUpdate:                 buildPermission(handle, "system", "user", catalogActionUpdate),
		UserDelete:                 buildPermission(handle, "system", "user", catalogActionDelete),
		GroupCreate:                buildPermission(handle, "system", "group", catalogActionCreate),
		GroupUpdate:                buildPermission(handle, "system", "group", catalogActionUpdate),
		GroupDelete:                buildPermission(handle, "system", "group", catalogActionDelete),
		AgentCreate:                buildPermission(handle, "system", "agent", catalogActionCreate),
		AgentUpdate:                buildPermission(handle, "system", "agent", catalogActionUpdate),
		AgentDelete:                buildPermission(handle, "system", "agent", catalogActionDelete),
		UserTypeCreate:             buildPermission(handle, "system", "usertype", catalogActionCreate),
		UserTypeUpdate:             buildPermission(handle, "system", "usertype", catalogActionUpdate),
		UserTypeDelete:             buildPermission(handle, "system", "usertype", catalogActionDelete),
		AgentTypeCreate:            buildPermission(handle, "system", "agenttype", catalogActionCreate),
		AgentTypeUpdate:            buildPermission(handle, "system", "agenttype", catalogActionUpdate),
		AgentTypeDelete:            buildPermission(handle, "system", "agenttype", catalogActionDelete),
		NotificationTemplateCreate: buildPermission(handle, "system", "notificationtemplate", catalogActionCreate),
		NotificationTemplateUpdate: buildPermission(handle, "system", "notificationtemplate", catalogActionUpdate),
		NotificationTemplateDelete: buildPermission(handle, "system", "notificationtemplate", catalogActionDelete),
		RoleView:                   buildPermission(handle, "system", "role", catalogActionView),
		RoleCreate:                 buildPermission(handle, "system", "role", catalogActionCreate),
		RoleUpdate:                 buildPermission(handle, "system", "role", catalogActionUpdate),
		RoleDelete:                 buildPermission(handle, "system", "role", catalogActionDelete),
		ResourceServerView:         buildPermission(handle, "system", "rs", catalogActionView),
		ResourceServerCreate:       buildPermission(handle, "system", "rs", catalogActionCreate),
		ResourceServerUpdate:       buildPermission(handle, "system", "rs", catalogActionUpdate),
		ResourceServerDelete:       buildPermission(handle, "system", "rs", catalogActionDelete),
		ConnectionView:             buildPermission(handle, "system", "connection", catalogActionView),
		ConnectionCreate:           buildPermission(handle, "system", "connection", catalogActionCreate),
		ConnectionUpdate:           buildPermission(handle, "system", "connection", catalogActionUpdate),
		ConnectionDelete:           buildPermission(handle, "system", "connection", catalogActionDelete),
		VCView:                     buildPermission(handle, "system", "vc", catalogActionView),
		VCCreate:                   buildPermission(handle, "system", "vc", catalogActionCreate),
		VCUpdate:                   buildPermission(handle, "system", "vc", catalogActionUpdate),
		VCDelete:                   buildPermission(handle, "system", "vc", catalogActionDelete),
		ProjectView:                buildPermission(handle, "system", "project", catalogActionView),
		ProjectCreate:              buildPermission(handle, "system", "project", catalogActionCreate),
		ProjectUpdate:              buildPermission(handle, "system", "project", catalogActionUpdate),
		ProjectDelete:              buildPermission(handle, "system", "project", catalogActionDelete),
		DesignView:                 buildPermission(handle, "system", "design", catalogActionView),
		DesignCreate:               buildPermission(handle, "system", "design", catalogActionCreate),
		DesignUpdate:               buildPermission(handle, "system", "design", catalogActionUpdate),
		DesignDelete:               buildPermission(handle, "system", "design", catalogActionDelete),
		I18nView:                   buildPermission(handle, "system", "i18n", catalogActionView),
		I18nCreate:                 buildPermission(handle, "system", "i18n", catalogActionCreate),
		I18nUpdate:                 buildPermission(handle, "system", "i18n", catalogActionUpdate),
		I18nDelete:                 buildPermission(handle, "system", "i18n", catalogActionDelete),
		GatewayView:                buildPermission(handle, "system", "gateway", catalogActionView),
		GatewayCreate:              buildPermission(handle, "system", "gateway", catalogActionCreate),
		GatewayUpdate:              buildPermission(handle, "system", "gateway", catalogActionUpdate),
		GatewayDelete:              buildPermission(handle, "system", "gateway", catalogActionDelete),
		ServerConfigView:           buildPermission(handle, "system", "serverconfig", catalogActionView),
		ServerConfigUpdate:         buildPermission(handle, "system", "serverconfig", catalogActionUpdate),
		ConfigVersionView:          buildPermission(handle, "system", "configversion", catalogActionView),
		ConfigVersionCreate:        buildPermission(handle, "system", "configversion", catalogActionCreate),
		AuthzDetailView:            buildPermission(handle, "system", "authzdetail", catalogActionView),
		AuthzDetailCreate:          buildPermission(handle, "system", "authzdetail", catalogActionCreate),
		AuthzDetailUpdate:          buildPermission(handle, "system", "authzdetail", catalogActionUpdate),
		AuthzDetailDelete:          buildPermission(handle, "system", "authzdetail", catalogActionDelete),
	}
	for _, entry := range systemCatalog {
		p.Catalog = append(p.Catalog, buildPermission(handle, "system", string(entry.resource)))
		for _, action := range entry.actions {
			p.Catalog = append(p.Catalog, buildPermission(handle, "system", string(entry.resource), action))
		}
	}
	sysPerms = p

	actionPermissionMap = map[Action]string{
		// Organization unit actions.
		ActionCreateOU:     p.OUCreate,
		ActionReadOU:       p.OUView,
		ActionUpdateOU:     p.OUUpdate,
		ActionDeleteOU:     p.OUDelete,
		ActionListOUs:      p.OUView,
		ActionListChildOUs: p.OUView,

		// User actions.
		ActionCreateUser: p.UserCreate,
		ActionReadUser:   p.UserView,
		ActionUpdateUser: p.UserUpdate,
		ActionDeleteUser: p.UserDelete,
		ActionListUsers:  p.UserView,

		// Group actions.
		ActionCreateGroup: p.GroupCreate,
		ActionReadGroup:   p.GroupView,
		ActionUpdateGroup: p.GroupUpdate,
		ActionDeleteGroup: p.GroupDelete,
		ActionListGroups:  p.GroupView,

		// Agent actions.
		ActionCreateAgent: p.AgentCreate,
		ActionReadAgent:   p.AgentView,
		ActionUpdateAgent: p.AgentUpdate,
		ActionDeleteAgent: p.AgentDelete,
		ActionListAgents:  p.AgentView,

		// User type actions.
		ActionCreateUserType: p.UserTypeCreate,
		ActionReadUserType:   p.UserTypeView,
		ActionUpdateUserType: p.UserTypeUpdate,
		ActionDeleteUserType: p.UserTypeDelete,
		ActionListUserTypes:  p.UserTypeView,

		// Agent schema actions.
		ActionCreateAgentType: p.AgentTypeCreate,
		ActionReadAgentType:   p.AgentTypeView,
		ActionUpdateAgentType: p.AgentTypeUpdate,
		ActionDeleteAgentType: p.AgentTypeDelete,
		ActionListAgentTypes:  p.AgentTypeView,

		// Notification template actions.
		ActionCreateNotificationTemplate: p.NotificationTemplateCreate,
		ActionReadNotificationTemplate:   p.NotificationTemplateView,
		ActionUpdateNotificationTemplate: p.NotificationTemplateUpdate,
		ActionDeleteNotificationTemplate: p.NotificationTemplateDelete,
		ActionListNotificationTemplates:  p.NotificationTemplateView,

		// Flow actions.
		ActionCreateFlow: p.FlowCreate,
		ActionReadFlow:   p.FlowView,
		ActionUpdateFlow: p.FlowUpdate,
		ActionDeleteFlow: p.FlowDelete,
		ActionListFlows:  p.FlowView,

		// Role actions.
		ActionCreateRole: p.RoleCreate,
		ActionReadRole:   p.RoleView,
		ActionUpdateRole: p.RoleUpdate,
		ActionDeleteRole: p.RoleDelete,
		ActionListRoles:  p.RoleView,

		// Resource server actions.
		ActionCreateResourceServer: p.ResourceServerCreate,
		ActionReadResourceServer:   p.ResourceServerView,
		ActionUpdateResourceServer: p.ResourceServerUpdate,
		ActionDeleteResourceServer: p.ResourceServerDelete,
		ActionListResourceServers:  p.ResourceServerView,

		// Connection actions.
		ActionCreateConnection: p.ConnectionCreate,
		ActionReadConnection:   p.ConnectionView,
		ActionUpdateConnection: p.ConnectionUpdate,
		ActionDeleteConnection: p.ConnectionDelete,
		ActionListConnections:  p.ConnectionView,

		// Verifiable credential actions.
		ActionCreateVC: p.VCCreate,
		ActionReadVC:   p.VCView,
		ActionUpdateVC: p.VCUpdate,
		ActionDeleteVC: p.VCDelete,
		ActionListVCs:  p.VCView,

		// Design actions.
		ActionCreateDesign: p.DesignCreate,
		ActionReadDesign:   p.DesignView,
		ActionUpdateDesign: p.DesignUpdate,
		ActionDeleteDesign: p.DesignDelete,
		ActionListDesigns:  p.DesignView,

		// Translation actions.
		ActionCreateI18n: p.I18nCreate,
		ActionReadI18n:   p.I18nView,
		ActionUpdateI18n: p.I18nUpdate,
		ActionDeleteI18n: p.I18nDelete,
		ActionListI18n:   p.I18nView,

		// Gateway actions.
		ActionCreateGateway: p.GatewayCreate,
		ActionReadGateway:   p.GatewayView,
		ActionUpdateGateway: p.GatewayUpdate,
		ActionDeleteGateway: p.GatewayDelete,
		ActionListGateways:  p.GatewayView,

		// Server configuration actions.
		ActionReadServerConfig:   p.ServerConfigView,
		ActionUpdateServerConfig: p.ServerConfigUpdate,
		ActionListServerConfigs:  p.ServerConfigView,

		// Configuration version actions.
		ActionCreateConfigVersion: p.ConfigVersionCreate,
		ActionReadConfigVersion:   p.ConfigVersionView,
		ActionListConfigVersions:  p.ConfigVersionView,

		// Authorization detail type actions.
		ActionCreateAuthzDetail: p.AuthzDetailCreate,
		ActionReadAuthzDetail:   p.AuthzDetailView,
		ActionUpdateAuthzDetail: p.AuthzDetailUpdate,
		ActionDeleteAuthzDetail: p.AuthzDetailDelete,
		ActionListAuthzDetails:  p.AuthzDetailView,
	}

	apiPermissionEntries = []apiPermissionEntry{
		// Self-service paths — accessible to any authenticated user (empty permission).
		// Listed before their parent wildcards so they always win on first-match.
		{"GET /users/me", ""},
		{"PUT /users/me", ""},
		{"GET /users/me/**", ""},
		{"PUT /users/me/**", ""},
		{"POST /users/me/update-credentials", ""},
		{"GET /register/passkey/**", ""},
		{"POST /register/passkey/**", ""},

		// Organization unit APIs. "/tree/{path...}" addresses an organization unit by its handle path.
		{"GET /organization-units", p.OUView},
		{"POST /organization-units", p.OUCreate},
		{"GET /organization-units/**", p.OUView},
		{"PUT /organization-units/**", p.OUUpdate},
		{"DELETE /organization-units/**", p.OUDelete},

		// User APIs. Credentials and sharing policies are part of the user they belong to, so changing
		// one updates the user. "/tree/{path...}" creates a user in the organization unit at that path.
		{"GET /users", p.UserView},
		{"POST /users", p.UserCreate},
		{"POST /users/tree/**", p.UserCreate},
		{"POST /users/*/update-credentials", p.UserUpdate},
		{"POST /users/*/sharing-policies", p.UserUpdate},
		{"DELETE /users/*/sharing-policies/**", p.UserUpdate},
		{"GET /users/**", p.UserView},
		{"PUT /users/**", p.UserUpdate},
		{"DELETE /users/**", p.UserDelete},

		// Agent APIs.
		{"GET /agents", p.AgentView},
		{"POST /agents", p.AgentCreate},
		{"GET /agents/**", p.AgentView},
		{"PUT /agents/**", p.AgentUpdate},
		{"DELETE /agents/**", p.AgentDelete},

		// SCIM API.
		{"GET /scim/v2/Me", ""},
		{"PUT /scim/v2/Me", ""},
		{"GET /scim/v2/Schemas", ""},
		{"GET /scim/v2/Schemas/**", ""},
		{"GET /scim/v2/ResourceTypes", ""},
		{"GET /scim/v2/ResourceTypes/**", ""},
		{"GET /scim/v2/Users", p.UserView},
		{"POST /scim/v2/Users", p.UserCreate},
		{"POST /scim/v2/Users/.search", p.UserView},
		{"GET /scim/v2/Users/**", p.UserView},
		{"PUT /scim/v2/Users/**", p.UserUpdate},
		{"PATCH /scim/v2/Users/**", p.UserUpdate},
		{"DELETE /scim/v2/Users/**", p.UserDelete},
		{"GET /scim/v2/Groups", p.GroupView},
		{"POST /scim/v2/Groups", p.GroupCreate},
		{"GET /scim/v2/Groups/**", p.GroupView},
		{"PUT /scim/v2/Groups/**", p.GroupUpdate},
		{"PATCH /scim/v2/Groups/**", p.GroupUpdate},
		{"DELETE /scim/v2/Groups/**", p.GroupDelete},

		// Group APIs. "/tree/{path...}" creates a group in the organization unit at that path. Adding or
		// removing members updates the group.
		{"GET /groups", p.GroupView},
		{"POST /groups", p.GroupCreate},
		{"POST /groups/tree/**", p.GroupCreate},
		{"GET /groups/**", p.GroupView},
		{"POST /groups/**", p.GroupUpdate},
		{"PUT /groups/**", p.GroupUpdate},
		{"DELETE /groups/**", p.GroupDelete},

		// User type APIs.
		{"GET /user-types", p.UserTypeView},
		{"POST /user-types", p.UserTypeCreate},
		{"GET /user-types/**", p.UserTypeView},
		{"PUT /user-types/**", p.UserTypeUpdate},
		{"DELETE /user-types/**", p.UserTypeDelete},

		// Agent schema APIs.
		{"GET /agent-types", p.AgentTypeView},
		{"POST /agent-types", p.AgentTypeCreate},
		{"GET /agent-types/**", p.AgentTypeView},
		{"PUT /agent-types/**", p.AgentTypeUpdate},
		{"DELETE /agent-types/**", p.AgentTypeDelete},

		// Flow APIs. Restoring a version, sharing policies, and flow variables are part of the flow they
		// belong to, so changing one updates the flow. These rules are listed before the "**" rules.
		{"GET /flows", p.FlowView},
		{"POST /flows", p.FlowCreate},
		{"POST /flows/*/restore", p.FlowUpdate},
		{"POST /flows/*/sharing-policies", p.FlowUpdate},
		{"DELETE /flows/*/sharing-policies/**", p.FlowUpdate},
		{"GET /flows/**", p.FlowView},
		{"PUT /flows/**", p.FlowUpdate},
		{"DELETE /flows/**", p.FlowDelete},

		// Role APIs. Assignments and sharing policies are part of the role they belong to, so changing
		// one updates the role. These rules are listed before the "**" rules.
		{"GET /roles", p.RoleView},
		{"POST /roles", p.RoleCreate},
		{"POST /roles/*/assignments/**", p.RoleUpdate},
		{"POST /roles/*/sharing-policies", p.RoleUpdate},
		{"DELETE /roles/*/sharing-policies/**", p.RoleUpdate},
		{"GET /roles/**", p.RoleView},
		{"PUT /roles/**", p.RoleUpdate},
		{"DELETE /roles/**", p.RoleDelete},

		// Resource server APIs. The System resource server declares the permissions these rules check,
		// so changing it requires root; a resource server permission only reads it. Resources and actions
		// are part of the resource server they belong to, so changing one updates the resource server.
		// These rules are listed before the "**" rules.
		// ponytail: matches the System resource server by its bootstrap ID; a deployment that created it
		// under another ID needs a service check instead.
		{"POST /resource-servers/" + systemResourceServerID + "/**", p.Root},
		{"PUT /resource-servers/" + systemResourceServerID + "/**", p.Root},
		{"DELETE /resource-servers/" + systemResourceServerID + "/**", p.Root},
		{"GET /resource-servers", p.ResourceServerView},
		{"POST /resource-servers", p.ResourceServerCreate},
		{"POST /resource-servers/*/actions", p.ResourceServerUpdate},
		{"POST /resource-servers/*/resources", p.ResourceServerUpdate},
		{"POST /resource-servers/*/resources/*/actions", p.ResourceServerUpdate},
		{"DELETE /resource-servers/*/actions/**", p.ResourceServerUpdate},
		{"DELETE /resource-servers/*/resources/**", p.ResourceServerUpdate},
		{"GET /resource-servers/**", p.ResourceServerView},
		{"PUT /resource-servers/**", p.ResourceServerUpdate},
		{"DELETE /resource-servers/**", p.ResourceServerDelete},

		// Connection APIs. Each vendor has its own collection, "/connections/{vendor}". A sharing policy is
		// part of the connection it governs, so changing one updates the connection. Sharing policy rules
		// are listed before the "**" rules.
		{"GET /connections", p.ConnectionView},
		{"POST /connections/*/*/sharing-policies", p.ConnectionUpdate},
		{"DELETE /connections/*/*/sharing-policies/**", p.ConnectionUpdate},
		{"POST /connections/*", p.ConnectionCreate},
		{"GET /connections/**", p.ConnectionView},
		{"PUT /connections/**", p.ConnectionUpdate},
		{"DELETE /connections/**", p.ConnectionDelete},

		// Verifiable credential APIs: credential configurations issued over OpenID4VCI, presentation
		// definitions requested over OpenID4VP, and the trust anchors presentations are verified against.
		{"GET /openid4vci/credential-configurations", p.VCView},
		{"POST /openid4vci/credential-configurations", p.VCCreate},
		{"GET /openid4vci/credential-configurations/**", p.VCView},
		{"PUT /openid4vci/credential-configurations/**", p.VCUpdate},
		{"DELETE /openid4vci/credential-configurations/**", p.VCDelete},
		{"GET /openid4vp/presentation-definitions", p.VCView},
		{"POST /openid4vp/presentation-definitions", p.VCCreate},
		{"GET /openid4vp/presentation-definitions/**", p.VCView},
		{"PUT /openid4vp/presentation-definitions/**", p.VCUpdate},
		{"DELETE /openid4vp/presentation-definitions/**", p.VCDelete},
		{"GET /openid4vp/trust-anchors", p.VCView},

		// Design APIs: themes and layouts.
		{"GET /design/themes", p.DesignView},
		{"POST /design/themes", p.DesignCreate},
		{"GET /design/themes/**", p.DesignView},
		{"PUT /design/themes/**", p.DesignUpdate},
		{"DELETE /design/themes/**", p.DesignDelete},
		{"GET /design/layouts", p.DesignView},
		{"POST /design/layouts", p.DesignCreate},
		{"GET /design/layouts/**", p.DesignView},
		{"PUT /design/layouts/**", p.DesignUpdate},
		{"DELETE /design/layouts/**", p.DesignDelete},

		// Translation APIs. Reading translations is public. A POST sets or replaces the override of a
		// translation the server already has, so it updates rather than creates; a DELETE clears overrides.
		{"POST /i18n/languages/*/translations", p.I18nUpdate},
		{"DELETE /i18n/languages/*/translations", p.I18nDelete},
		{"POST /i18n/languages/*/translations/ns/*/keys/*", p.I18nUpdate},
		{"DELETE /i18n/languages/*/translations/ns/*/keys/*", p.I18nDelete},

		// Gateway APIs. A gateway's variable and secret stores hold credentials, so they require root, as
		// the server's own variable store does. Applying or reverting a configuration updates the
		// gateway. These rules are listed before the "**" rules.
		{"GET /gateways/*/variables/**", p.Root},
		{"POST /gateways/*/variables/**", p.Root},
		{"PUT /gateways/*/variables/**", p.Root},
		{"DELETE /gateways/*/variables/**", p.Root},
		{"GET /gateways/*/secrets/**", p.Root},
		{"POST /gateways/*/secrets/**", p.Root},
		{"PUT /gateways/*/secrets/**", p.Root},
		{"DELETE /gateways/*/secrets/**", p.Root},
		{"GET /gateways", p.GatewayView},
		{"POST /gateways", p.GatewayCreate},
		{"POST /gateways/*/apply", p.GatewayUpdate},
		{"POST /gateways/*/revert", p.GatewayUpdate},
		{"GET /gateways/**", p.GatewayView},
		{"PUT /gateways/**", p.GatewayUpdate},
		{"DELETE /gateways/**", p.GatewayDelete},

		// Server configuration APIs.
		{"GET /server-config", p.ServerConfigView},
		{"GET /server-config/**", p.ServerConfigView},
		{"PUT /server-config/**", p.ServerConfigUpdate},

		// Configuration version APIs.
		{"GET /configuration-versions", p.ConfigVersionView},
		{"POST /configuration-versions", p.ConfigVersionCreate},
		{"GET /configuration-versions/**", p.ConfigVersionView},

		// Authorization detail type APIs.
		{"GET /authorization-detail-types", p.AuthzDetailView},
		{"POST /authorization-detail-types", p.AuthzDetailCreate},
		{"GET /authorization-detail-types/**", p.AuthzDetailView},
		{"PUT /authorization-detail-types/**", p.AuthzDetailUpdate},
		{"DELETE /authorization-detail-types/**", p.AuthzDetailDelete},

		// Project APIs.
		{"GET /projects", p.ProjectView},
		{"POST /projects", p.ProjectCreate},
		{"GET /projects/**", p.ProjectView},
		{"PUT /projects/**", p.ProjectUpdate},
		{"DELETE /projects/**", p.ProjectDelete},

		// Variable store APIs. The whole surface requires root rather than a narrower permission:
		// the store holds credentials, and even the list of names is sensitive. This matches the
		// default for an unlisted path, and says so rather than relying on it.
		{"GET /variables", p.Root},
		{"POST /variables", p.Root},
		{"GET /variables/**", p.Root},
		{"PUT /variables/**", p.Root},
		{"DELETE /variables/**", p.Root},
		{"GET /secrets", p.Root},
		{"POST /secrets", p.Root},
		{"GET /secrets/**", p.Root},
		{"PUT /secrets/**", p.Root},
		{"DELETE /secrets/**", p.Root},

		// Notification template APIs. The {channel} segment is matched by "*"; item sub-paths by "**".
		// Collection rules are listed before the "**" rules so first-match-wins picks them for the list.
		{"GET /notification-templates/*/templates", p.NotificationTemplateView},
		{"POST /notification-templates/*/templates", p.NotificationTemplateCreate},
		{"GET /notification-templates/**", p.NotificationTemplateView},
		{"PUT /notification-templates/**", p.NotificationTemplateUpdate},
		{"DELETE /notification-templates/**", p.NotificationTemplateDelete},

		// Import APIs.
		{"POST /import", p.Root},
		{"POST /import/delete", p.Root},
	}

	mcpToolPermissions = map[string]string{
		// Organization unit tools.
		"list_organization_units": p.OUView,

		// User type tools.
		"list_user_types": p.UserTypeView,

		// Project tools.
		"list_projects":     p.ProjectView,
		"get_project_by_id": p.ProjectView,
		"create_project":    p.ProjectCreate,
		"update_project":    p.ProjectUpdate,

		// Flow tools.
		"list_flows":         p.FlowView,
		"get_flow_by_handle": p.FlowView,
		"get_flow_by_id":     p.FlowView,
		"create_flow":        p.FlowCreate,
		"update_flow":        p.FlowUpdate,
		// Connection tools.
		"list_http_connections":     p.ConnectionView,
		"get_http_connection_by_id": p.ConnectionView,
		"create_http_connection":    p.ConnectionCreate,
		"update_http_connection":    p.ConnectionUpdate,
		// Design tools.
		"list_themes":     p.DesignView,
		"get_theme_by_id": p.DesignView,
		// Gateway and configuration version tools. A gateway's variable and secret tools stay root, as
		// their APIs do.
		"list_gateways":                 p.GatewayView,
		"get_gateway_applied_version":   p.GatewayView,
		"get_gateway_configuration":     p.GatewayView,
		"diff_version":                  p.GatewayView,
		"deploy_version":                p.GatewayUpdate,
		"promote_gateway":               p.GatewayUpdate,
		"rollback_gateway":              p.GatewayUpdate,
		"list_configuration_versions":   p.ConfigVersionView,
		"get_configuration_version":     p.ConfigVersionView,
		"capture_configuration_version": p.ConfigVersionCreate,

		// Authorization detail type tools.
		"list_authorization_detail_types":     p.AuthzDetailView,
		"get_authorization_detail_type_by_id": p.AuthzDetailView,
		"create_authorization_detail_type":    p.AuthzDetailCreate,
		"update_authorization_detail_type":    p.AuthzDetailUpdate,
	}
}

// GetSystemPermissions returns the active system permissions.
// Returns nil if InitSystemPermissions has not been called.
func GetSystemPermissions() *SystemPermissions {
	return sysPerms
}

// GetSystemRootPermission returns the root system permission string.
// It panics if InitSystemPermissions has not been called, which fails closed
// rather than allowing unauthenticated access.
func GetSystemRootPermission() string {
	return sysPerms.Root
}

// ---- Action → Permission map ----

// actionPermissionMap maps each system action to the minimum permission required to perform it.
// Actions not present in this map default to requiring the root system permission.
// Rebuilt by InitSystemPermissions at startup.
var actionPermissionMap map[Action]string

// ---- API → Permission map ----

// apiPermissionEntry pairs a "METHOD glob-path" pattern with the minimum permission
// required for matching requests.
type apiPermissionEntry struct {
	pattern    string
	permission string
}

// apiPermissionEntries defines the ordered set of API permission rules.
// Evaluation is first-match-wins, so more specific patterns must appear before
// broader wildcard patterns. Pattern syntax (applied to the full "METHOD /path" string)
// follows the same glob rules used by publicPaths:
//   - "*"  matches exactly one path segment (e.g., a resource ID).
//   - "**" matches zero or more path segments; only valid as the final component
//     after "/" (e.g., "GET /users/me/**" covers all sub-paths of /users/me).
//
// Rebuilt by InitSystemPermissions at startup.
var apiPermissionEntries []apiPermissionEntry

// ---- MCP tool → Permission map ----

// mcpToolPermissions maps each MCP tool name to the minimum permission required to call it.
// Tools not present in this map default to requiring the root system permission.
// Rebuilt by InitSystemPermissions at startup.
var mcpToolPermissions map[string]string

// ResolveMCPToolPermission returns the minimum permission required to call the named MCP tool.
// Falls back to the root system permission for tools not listed in the MCP tool permission map.
func ResolveMCPToolPermission(toolName string) string {
	if perm, ok := mcpToolPermissions[toolName]; ok {
		return perm
	}
	return GetSystemRootPermission()
}

// ---- Helper functions ----

// HasSystemPermission returns true if the caller holds the root system permission.
// This is a fast-path check used to grant unconditional access (skipping policy evaluation).
func HasSystemPermission(permissions []string) bool {
	if sysPerms == nil {
		return false
	}
	for _, p := range permissions {
		if p == sysPerms.Root {
			return true
		}
	}
	return false
}

// HasSufficientPermission returns true if any permission in userPermissions satisfies
// the required permission using hierarchical scope matching.
//
// Matching rules:
//   - Empty required: always satisfied (self-service paths with no specific permission requirement)
//   - Exact match: "system:ou:view" satisfies "system:ou:view"
//   - Parent scope: "system:ou" satisfies "system:ou:view" (parent covers all children)
//   - Root scope: "system" satisfies any "system:*" permission
func HasSufficientPermission(userPermissions []string, required string) bool {
	if required == "" {
		return true
	}
	for _, p := range userPermissions {
		if p == required || strings.HasPrefix(required, p+":") {
			return true
		}
	}
	return false
}

// ---- Permission set coverage ----

// PermissionSet maps a resource server ID to the permissions granted on that resource server.
type PermissionSet map[string][]string

// Covers reports whether actor holds every permission in required, and answers "may this caller
// confer these permissions on someone else?".
//
// Coverage is evaluated per resource server: a permission held on one never satisfies a
// requirement on another. Within a resource server, matching is hierarchical, so "system:user"
// covers "system:user:view" but not the reverse. Coverage is a partial order, not an ordering:
// two sets can each hold what the other lacks.
//
// An empty required set is covered. A required permission that is empty, or keyed under an empty
// resource server ID, is never covered, since approving a malformed grant would be unsafe.
func Covers(actor, required PermissionSet) bool {
	for resourceServerID, requiredPerms := range required {
		if len(requiredPerms) == 0 {
			continue
		}
		if resourceServerID == "" {
			return false
		}
		heldPerms := actor[resourceServerID]
		for _, requiredPerm := range requiredPerms {
			if requiredPerm == "" {
				return false
			}
			if !HasSufficientPermission(heldPerms, requiredPerm) {
				return false
			}
		}
	}
	return true
}

// ResolveActionPermission returns the minimum permission required to perform the given
// action. Falls back to the root system permission for actions not listed in the action permission map.
func ResolveActionPermission(action Action) string {
	if perm, ok := actionPermissionMap[action]; ok {
		return perm
	}
	return GetSystemRootPermission()
}
