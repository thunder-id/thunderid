// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"maps"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/constants"
)

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

// catalogResource is a resource declared under a root resource on the System resource server, with the
// actions declared on it.
type catalogResource struct {
	resource ResourceType
	actions  []string
}

// buildCatalog returns, in declaration order, the permission of each resource under root followed by
// the permissions of its actions.
func buildCatalog(handle, root string, resources []catalogResource) []string {
	var catalog []string
	for _, entry := range resources {
		catalog = append(catalog, buildPermission(handle, root, string(entry.resource)))
		for _, action := range entry.actions {
			catalog = append(catalog, buildPermission(handle, root, string(entry.resource), action))
		}
	}
	return catalog
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

	// ActionCreateApplication creates a new application.
	ActionCreateApplication Action = "app:create"
	// ActionReadApplication reads an application.
	ActionReadApplication Action = "app:read"
	// ActionUpdateApplication updates an application.
	ActionUpdateApplication Action = "app:update"
	// ActionDeleteApplication deletes an application.
	ActionDeleteApplication Action = "app:delete"
	// ActionListApplications lists applications.
	ActionListApplications Action = "app:list"

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

	// ActionCreateProject creates a new project.
	ActionCreateProject Action = "project:create"
	// ActionReadProject reads a project.
	ActionReadProject Action = "project:read"
	// ActionUpdateProject updates a project.
	ActionUpdateProject Action = "project:update"
	// ActionDeleteProject deletes a project.
	ActionDeleteProject Action = "project:delete"
	// ActionListProjects lists projects.
	ActionListProjects Action = "project:list"
)

// ---- Permissions ----

// SystemPermissions holds the runtime-resolved permission strings for the system resource server. Each
// field is the permission the plane requires for that resource or action. All values are set by
// InitControlPlanePermissions or InitDataPlanePermissions and must not be used before one is called.
type SystemPermissions struct {
	// Root is the system root permission. It reaches every organization unit in the deployment.
	Root string
	// Org is the organization root permission, bounded to the organization unit the token names. It is
	// empty on the control plane, which enforces no organization permission.
	Org string
	// AdministrationFlow is the permission a caller needs to run an administration flow.
	AdministrationFlow string
	// OUListRoots is the permission a caller needs to list the root organization units. On the data plane it
	// is the system root permission, since an organization permission reaches only the unit its token names.
	OUListRoots                string
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
	AppView                    string
	AppCreate                  string
	AppUpdate                  string
	AppDelete                  string
	// Catalog holds every permission the plane declares besides Root, in declaration order: each
	// resource followed by its actions.
	Catalog []string
}

// sysPerms holds the active system permissions, initialized by InitControlPlanePermissions or
// InitDataPlanePermissions.
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

// activatePermissions makes p the active permission set and rebuilds the action, route and MCP tool
// maps from it. Each plane's initializer calls it once at startup, before any service or middleware
// uses permissions.
func activatePermissions(p *SystemPermissions) {
	sysPerms = p

	actionPermissionMap = map[Action]string{
		// Organization unit actions.
		ActionCreateOU:     p.OUCreate,
		ActionReadOU:       p.OUView,
		ActionUpdateOU:     p.OUUpdate,
		ActionDeleteOU:     p.OUDelete,
		ActionListOUs:      p.OUListRoots,
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

		// Application actions.
		ActionCreateApplication: p.AppCreate,
		ActionReadApplication:   p.AppView,
		ActionUpdateApplication: p.AppUpdate,
		ActionDeleteApplication: p.AppDelete,
		ActionListApplications:  p.AppView,

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
		{"GET /capabilities", ""},
		{"DELETE /users/me/agents/*", ""},
		{"POST /users/me/agents/suspend", ""},
		{"POST /users/me/agents/*/suspend", ""},
		{"POST /users/me/agents/*/unsuspend", ""},
		{"GET /register/passkey/**", ""},
		{"POST /register/passkey/**", ""},
		// MCP checks each tool call against the permission the tool is mapped to, so a token reaches it as its caller.
		{"* /mcp/**", ""},
		// Flow execution: a token reaches the flow engine as its caller, which decides what each flow type needs.
		{"POST /flow/execute", ""},

		// Organization unit APIs. "/tree/{path...}" addresses an organization unit by its handle path.
		{"GET /organization-units", p.OUListRoots},
		{"POST /organization-units", p.OUCreate},
		{"GET /organization-units/**", p.OUView},
		{"PUT /organization-units/**", p.OUUpdate},
		{"DELETE /organization-units/**", p.OUDelete},

		// User APIs. Credentials and sharing policies are part of the user they belong to, so changing
		// one updates the user. "/tree/{path...}" creates a user in the organization unit at that path.
		// The units a user can access span the deployment, so listing them needs the root permission.
		{"GET /users", p.UserView},
		{"GET /users/*/ous", p.Root},
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
		{"POST /agents/*/client-secret/regenerate", p.AgentUpdate},
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

		// Resource server APIs. Resources and actions are part of the resource server they belong to, so
		// changing one updates the resource server. These rules are listed before the "**" rules.
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

		// Application APIs. A sharing policy is part of the application it governs, so changing one
		// updates the application. Sharing policy rules are listed before the "**" rules.
		{"GET /applications", p.AppView},
		{"POST /applications", p.AppCreate},
		{"POST /applications/*/sharing-policies", p.AppUpdate},
		{"DELETE /applications/*/sharing-policies/**", p.AppUpdate},
		{"POST /applications/*/client-secret/regenerate", p.AppUpdate},
		{"GET /applications/**", p.AppView},
		{"PUT /applications/**", p.AppUpdate},
		{"DELETE /applications/**", p.AppDelete},
		// CIMD preview is a step of creating an application. When another resource uses CIMD, give
		// each its own route on the shared CIMD service, gated by that resource's create permission.
		{"POST /cimd/preview", p.AppCreate},

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
		{"POST /import/managed", p.Root},
	}

	mcpToolPermissions = map[string]string{
		// Organization unit tools.
		"list_organization_units": p.OUListRoots,

		// User tools, at the permissions of the user API they act through.
		"list_users":  p.UserView,
		"get_user":    p.UserView,
		"update_user": p.UserUpdate,
		"create_user": p.UserCreate,

		// User type tools.
		"list_user_types": p.UserTypeView,
		// Onboarding tools, at the permission of the action their flow performs, as the Console needs. The
		// flow checks its own permissions and the services authorize each write as the caller, so a
		// custom flow that needs more still refuses a caller who lacks it.
		"onboard_agents": p.AgentCreate,
		"onboard_users":  p.UserCreate,

		// Agent tools, at the permissions of the agent API they act through.
		"list_agents":              p.AgentView,
		"get_agent":                p.AgentView,
		"onboard_agent":            p.AgentCreate,
		"update_agent":             p.AgentUpdate,
		"rotate_agent_credentials": p.AgentUpdate,

		// Agent type tools.
		"list_agent_types": p.AgentTypeView,

		// Role tools, at the permissions of the role and resource server APIs they act through.
		"list_roles":                       p.RoleView,
		"get_role":                         p.RoleView,
		"create_role":                      p.RoleCreate,
		"update_role_permissions":          p.RoleUpdate,
		"update_role_assignments":          p.RoleUpdate,
		"list_resource_servers":            p.ResourceServerView,
		"list_resource_server_permissions": p.ResourceServerView,

		// Group tools, at the permissions of the group API they act through.
		"list_groups":              p.GroupView,
		"get_group":                p.GroupView,
		"create_group":             p.GroupCreate,
		"update_group_memberships": p.GroupUpdate,

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

		// Application tools.
		"list_applications":             p.AppView,
		"get_application_by_id":         p.AppView,
		"get_application_by_client_id":  p.AppView,
		"get_application_templates":     p.AppView,
		"create_application":            p.AppCreate,
		"update_application":            p.AppUpdate,
		"regenerate_application_secret": p.AppUpdate,
	}

	// Only the Control Plane serves projects. A plane that declares no project permission leaves the
	// project actions, APIs and tools out, so they require the root permission.
	if p.ProjectView != "" {
		maps.Copy(actionPermissionMap, map[Action]string{
			ActionCreateProject: p.ProjectCreate,
			ActionReadProject:   p.ProjectView,
			ActionUpdateProject: p.ProjectUpdate,
			ActionDeleteProject: p.ProjectDelete,
			ActionListProjects:  p.ProjectView,
		})
		apiPermissionEntries = append(apiPermissionEntries,
			apiPermissionEntry{"GET /projects", p.ProjectView},
			apiPermissionEntry{"POST /projects", p.ProjectCreate},
			apiPermissionEntry{"GET /projects/**", p.ProjectView},
			apiPermissionEntry{"PUT /projects/**", p.ProjectUpdate},
			apiPermissionEntry{"DELETE /projects/**", p.ProjectDelete},
		)
		maps.Copy(mcpToolPermissions, map[string]string{
			"list_projects":     p.ProjectView,
			"get_project_by_id": p.ProjectView,
			"create_project":    p.ProjectCreate,
			"update_project":    p.ProjectUpdate,
		})
	}
}

// GetSystemPermissions returns the active system permissions.
// Returns nil if no plane's permissions have been initialized.
func GetSystemPermissions() *SystemPermissions {
	return sysPerms
}

// GetSystemRootPermission returns the root system permission string.
// It panics if no plane's permissions have been initialized, which fails closed
// rather than allowing unauthenticated access.
func GetSystemRootPermission() string {
	return sysPerms.Root
}

// ---- Action → Permission map ----

// actionPermissionMap maps each system action to the minimum permission required to perform it.
// Actions not present in this map default to requiring the root system permission.
// Rebuilt by activatePermissions at startup.
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
// Rebuilt by activatePermissions at startup.
var apiPermissionEntries []apiPermissionEntry

// ---- MCP tool → Permission map ----

// mcpToolPermissions maps each MCP tool name to the minimum permission required to call it.
// Tools not present in this map default to requiring the root system permission.
// Rebuilt by activatePermissions at startup.
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

// HasAnySystemPermission reports whether permissions include the root system permission or any permission
// under it. On the data plane the organization root and any permission under it count as well.
func HasAnySystemPermission(permissions []string) bool {
	if sysPerms == nil {
		return false
	}
	for _, p := range permissions {
		if p == sysPerms.Root || strings.HasPrefix(p, sysPerms.Root+":") {
			return true
		}
		if sysPerms.Org != "" && (p == sysPerms.Org || strings.HasPrefix(p, sysPerms.Org+":")) {
			return true
		}
	}
	return false
}

// LacksOrganizationUnit reports whether the caller must name an organization unit and its token names none. Where
// the organization root permission is bounded to the organization unit a token names, as on the data plane, a
// caller without the root system permission acts only in that unit, so a token naming no unit reaches nothing. The
// root system permission reaches the whole deployment and needs no unit.
func LacksOrganizationUnit(ctx context.Context) bool {
	if sysPerms == nil || sysPerms.Org == "" {
		return false
	}
	return GetOUID(ctx) == "" && !HasSystemPermission(GetPermissions(ctx))
}

// HasSufficientSystemPermission reports whether permissions satisfy a permission of the System resource
// server. It applies HasSufficientPermission and additionally lets the root system permission satisfy
// every requirement, so that on the data plane it covers the organization tree as well as its own.
func HasSufficientSystemPermission(permissions []string, required string) bool {
	if HasSufficientPermission(permissions, required) {
		return true
	}
	return required != "" && HasSystemPermission(permissions)
}

// HasSufficientPermission returns true if any permission in userPermissions satisfies
// the required permission using hierarchical scope matching.
//
// Matching rules:
//   - Empty required: always satisfied (self-service paths with no specific permission requirement)
//   - Exact match: "system:ou:view" satisfies "system:ou:view"
//   - Parent scope: "system:ou" satisfies "system:ou:view" (parent covers all children)
//   - Root scope: "system" satisfies any "system:*" permission
//
// System permissions always use DefaultPermissionDelimiter.
func HasSufficientPermission(userPermissions []string, required string) bool {
	if required == "" {
		return true
	}
	return AnyPermissionCovers(userPermissions, required, DefaultPermissionDelimiter)
}

// ---- Permission set coverage ----

// PermissionSet maps a resource server ID to the permissions granted on that resource server.
type PermissionSet map[string][]string

// Covers reports whether actor may grant every required permission on the same resource server.
// Custom permissions use their resolved administrative delimiter. System permissions on the
// built-in System resource server retain fixed colon coverage. Empty required sets are covered;
// empty permission names and resource-server IDs are refused.
func Covers(actor, required PermissionSet, delimiters map[string]string) bool {
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
			delimiter := delimiters[resourceServerID]
			if resourceServerID == constants.SystemResourceServerID &&
				PermissionCovers(GetSystemRootPermission(), requiredPerm, DefaultPermissionDelimiter) {
				delimiter = DefaultPermissionDelimiter
			}
			if !AnyPermissionCovers(heldPerms, requiredPerm, delimiter) {
				return false
			}
		}
	}
	return true
}

// HeldPermissions returns the recorded permissions that current still covers on some resource server. A
// permission revoked since it was recorded is left out, and so is a scope no resource server holds.
func HeldPermissions(recorded []string, current PermissionSet) []string {
	held := make([]string, 0, len(recorded))
	for _, permission := range recorded {
		for _, onServer := range current {
			if HasSufficientPermission(onServer, permission) {
				held = append(held, permission)
				break
			}
		}
	}
	return held
}

// ResolveActionPermission returns the minimum permission required to perform the given
// action. Falls back to the root system permission for actions not listed in the action permission map.
func ResolveActionPermission(action Action) string {
	if perm, ok := actionPermissionMap[action]; ok {
		return perm
	}
	return GetSystemRootPermission()
}

// PermissionCovers reports whether held equals required or is its delimiter-separated ancestor.
// Coverage never works upwards. An empty delimiter permits only exact matches.
func PermissionCovers(held, required, delimiter string) bool {
	if held == "" || required == "" {
		return false
	}
	if held == required {
		return true
	}
	if delimiter == "" {
		return false
	}
	return strings.HasPrefix(required, held+delimiter)
}

// AnyPermissionCovers reports whether any of held authorizes required.
func AnyPermissionCovers(held []string, required, delimiter string) bool {
	for _, p := range held {
		if PermissionCovers(p, required, delimiter) {
			return true
		}
	}
	return false
}

// AncestorPermissions returns the permissions of the resources a permission sits under, nearest
// ancestor first. A top-level permission has none.
func AncestorPermissions(permission, delimiter string) []string {
	if permission == "" || delimiter == "" {
		return nil
	}
	segments := strings.Split(permission, delimiter)
	if len(segments) < 2 {
		return nil
	}
	ancestors := make([]string, 0, len(segments)-1)
	for i := len(segments) - 1; i > 0; i-- {
		ancestors = append(ancestors, strings.Join(segments[:i], delimiter))
	}
	return ancestors
}

// DefaultPermissionDelimiter is the fixed System permission separator.
const DefaultPermissionDelimiter = ":"
