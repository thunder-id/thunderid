// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

// dataPlaneOrgCatalog lists, in declaration order, each resource declared under the org root resource
// on the System resource server and the actions declared on it. These are the resources whose services
// confine a caller to the organization unit its token names. The System resource server in the
// bootstrap bundle (cmd/server/bootstrap/01-default-resources.yaml) declares it.
var dataPlaneOrgCatalog = []catalogResource{
	{ResourceTypeOU, crudActions},
	{ResourceTypeUser, crudActions},
	{ResourceTypeGroup, crudActions},
	{ResourceTypeAgent, crudActions},
	{ResourceTypeUserType, crudActions},
	{ResourceTypeAgentType, crudActions},
}

// sessionActions are the actions declared on SSO sessions, which can only be read and revoked.
var sessionActions = []string{catalogActionView, catalogActionDelete}

// dataPlaneSystemCatalog lists, in declaration order, each resource declared under the system root
// resource on the System resource server and the actions declared on it. These are the resources that
// reach the whole deployment.
var dataPlaneSystemCatalog = []catalogResource{
	{ResourceTypeSession, sessionActions},
}

// InitDataPlanePermissions activates the data plane's permissions. When handle is non-empty (e.g.
// "mgmt"), every permission is prefixed with it.
//   - The system root permission reaches the whole deployment, and so does a child permission for each
//     resource and action in dataPlaneSystemCatalog.
//   - The org root permission, and a child permission for each resource and action in
//     dataPlaneOrgCatalog, are bounded to the organization unit the token names.
//   - Every other organization-owned resource requires the org root permission.
//   - Themes, layouts, translations and authorization detail types serve every organization unit:
//     reading one requires the org root permission, and changing one requires the system root
//     permission.
//   - Server configurations, gateways and configuration versions require the system root permission.
//
// It must be called once at startup before any service or middleware uses permissions.
func InitDataPlanePermissions(handle string) {
	system := buildPermission(handle, "system")
	org := buildPermission(handle, "org")
	orgChild := func(resource ResourceType, action string) string {
		return buildPermission(handle, "org", string(resource), action)
	}

	p := &SystemPermissions{
		Root:               system,
		Org:                org,
		AdministrationFlow: org,
		OUListRoots:        system,

		OU:              orgChild(ResourceTypeOU, ""),
		OUView:          orgChild(ResourceTypeOU, catalogActionView),
		OUCreate:        orgChild(ResourceTypeOU, catalogActionCreate),
		OUUpdate:        orgChild(ResourceTypeOU, catalogActionUpdate),
		OUDelete:        orgChild(ResourceTypeOU, catalogActionDelete),
		User:            orgChild(ResourceTypeUser, ""),
		UserView:        orgChild(ResourceTypeUser, catalogActionView),
		UserCreate:      orgChild(ResourceTypeUser, catalogActionCreate),
		UserUpdate:      orgChild(ResourceTypeUser, catalogActionUpdate),
		UserDelete:      orgChild(ResourceTypeUser, catalogActionDelete),
		Group:           orgChild(ResourceTypeGroup, ""),
		GroupView:       orgChild(ResourceTypeGroup, catalogActionView),
		GroupCreate:     orgChild(ResourceTypeGroup, catalogActionCreate),
		GroupUpdate:     orgChild(ResourceTypeGroup, catalogActionUpdate),
		GroupDelete:     orgChild(ResourceTypeGroup, catalogActionDelete),
		Agent:           orgChild(ResourceTypeAgent, ""),
		AgentView:       orgChild(ResourceTypeAgent, catalogActionView),
		AgentCreate:     orgChild(ResourceTypeAgent, catalogActionCreate),
		AgentUpdate:     orgChild(ResourceTypeAgent, catalogActionUpdate),
		AgentDelete:     orgChild(ResourceTypeAgent, catalogActionDelete),
		UserType:        orgChild(ResourceTypeUserType, ""),
		UserTypeView:    orgChild(ResourceTypeUserType, catalogActionView),
		UserTypeCreate:  orgChild(ResourceTypeUserType, catalogActionCreate),
		UserTypeUpdate:  orgChild(ResourceTypeUserType, catalogActionUpdate),
		UserTypeDelete:  orgChild(ResourceTypeUserType, catalogActionDelete),
		AgentType:       orgChild(ResourceTypeAgentType, ""),
		AgentTypeView:   orgChild(ResourceTypeAgentType, catalogActionView),
		AgentTypeCreate: orgChild(ResourceTypeAgentType, catalogActionCreate),
		AgentTypeUpdate: orgChild(ResourceTypeAgentType, catalogActionUpdate),
		AgentTypeDelete: orgChild(ResourceTypeAgentType, catalogActionDelete),

		NotificationTemplate:       org,
		NotificationTemplateView:   org,
		NotificationTemplateCreate: org,
		NotificationTemplateUpdate: org,
		NotificationTemplateDelete: org,
		AppView:                    org,
		AppCreate:                  org,
		AppUpdate:                  org,
		AppDelete:                  org,
		RoleView:                   org,
		RoleCreate:                 org,
		RoleUpdate:                 org,
		RoleDelete:                 org,
		ResourceServerView:         org,
		ResourceServerCreate:       org,
		ResourceServerUpdate:       org,
		ResourceServerDelete:       org,
		ConnectionView:             org,
		ConnectionCreate:           org,
		ConnectionUpdate:           org,
		ConnectionDelete:           org,
		FlowView:                   org,
		FlowCreate:                 org,
		FlowUpdate:                 org,
		FlowDelete:                 org,
		VCView:                     org,
		VCCreate:                   org,
		VCUpdate:                   org,
		VCDelete:                   org,

		DesignView:        org,
		DesignCreate:      system,
		DesignUpdate:      system,
		DesignDelete:      system,
		I18nView:          org,
		I18nCreate:        system,
		I18nUpdate:        system,
		I18nDelete:        system,
		AuthzDetailView:   org,
		AuthzDetailCreate: system,
		AuthzDetailUpdate: system,
		AuthzDetailDelete: system,

		GatewayView:         system,
		GatewayCreate:       system,
		GatewayUpdate:       system,
		GatewayDelete:       system,
		ServerConfigView:    system,
		ServerConfigUpdate:  system,
		ConfigVersionView:   system,
		ConfigVersionCreate: system,

		SessionView:   buildPermission(handle, "system", string(ResourceTypeSession), catalogActionView),
		SessionDelete: buildPermission(handle, "system", string(ResourceTypeSession), catalogActionDelete),
	}
	p.Catalog = append([]string{org}, buildCatalog(handle, "org", dataPlaneOrgCatalog)...)
	p.Catalog = append(p.Catalog, buildCatalog(handle, "system", dataPlaneSystemCatalog)...)
	activatePermissions(p)
}
