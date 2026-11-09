// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

// serverConfigActions are the actions declared on server configurations, which can only be read and
// replaced.
var serverConfigActions = []string{catalogActionView, catalogActionUpdate}

// configVersionActions are the actions declared on configuration versions, which can only be read and
// captured.
var configVersionActions = []string{catalogActionView, catalogActionCreate}

// controlPlaneCatalog lists, in declaration order, each resource declared under the system root
// resource on the Control Plane resource server and the actions declared on it. That resource server
// (cmd/cpserver/idp-bootstrap/control-plane-resource-server.yaml) is imported into the identity
// provider that signs operators into the Control Plane.
var controlPlaneCatalog = []catalogResource{
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

// InitControlPlanePermissions activates the control plane's permissions: the system root permission and
// a child permission for each resource and action in controlPlaneCatalog. When handle is empty, they
// are "system", "system:ou" and so on; when it is non-empty (e.g. "mgmt"), they are prefixed
// ("mgmt:system", "mgmt:system:ou" and so on). The control plane enforces no organization permission.
// It must be called once at startup before any service or middleware uses permissions.
func InitControlPlanePermissions(handle string) {
	p := &SystemPermissions{
		Root:                       buildPermission(handle, "system"),
		OU:                         buildPermission(handle, "system", "ou"),
		OUView:                     buildPermission(handle, "system", "ou", "view"),
		OUListRoots:                buildPermission(handle, "system", "ou", "view"),
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
		AppView:                    buildPermission(handle, "system", "app", catalogActionView),
		AppCreate:                  buildPermission(handle, "system", "app", catalogActionCreate),
		AppUpdate:                  buildPermission(handle, "system", "app", catalogActionUpdate),
		AppDelete:                  buildPermission(handle, "system", "app", catalogActionDelete),
	}
	p.AdministrationFlow = p.Root
	p.Catalog = buildCatalog(handle, "system", controlPlaneCatalog)
	activatePermissions(p)
}
