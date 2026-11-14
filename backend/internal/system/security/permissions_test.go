// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// HasSystemPermission
// ---------------------------------------------------------------------------

func (s *SecurityContextTestSuite) TestHasSystemPermission() {
	InitControlPlanePermissions("")

	tests := []struct {
		name        string
		permissions []string
		want        bool
	}{
		{
			name:        "SystemPermissionAlone",
			permissions: []string{"system"},
			want:        true,
		},
		{
			name:        "SystemPermissionAmongOthers",
			permissions: []string{"system:ou", "system", "system:user:view"},
			want:        true,
		},
		{
			name:        "OnlyChildScopes",
			permissions: []string{"system:ou", "system:user:view"},
			want:        false,
		},
		{
			name:        "EmptySlice",
			permissions: []string{},
			want:        false,
		},
		{
			name:        "NilSlice",
			permissions: nil,
			want:        false,
		},
		{
			name:        "PrefixOfSystemDoesNotCount",
			permissions: []string{"sys", "systems"},
			want:        false,
		},
		{
			name:        "SystemAsSubstringDoesNotCount",
			permissions: []string{"supersystem", "system:admin"},
			want:        false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, HasSystemPermission(tt.permissions))
		})
	}
}

func (s *SecurityContextTestSuite) TestHasAnySystemPermission() {
	InitControlPlanePermissions("")

	s.True(HasAnySystemPermission([]string{"system"}))
	s.True(HasAnySystemPermission([]string{"openid", "system:user:create"}))
	s.False(HasAnySystemPermission([]string{"openid", "systemx", "app:system"}))
	s.False(HasAnySystemPermission(nil))
	// The control plane has no organization tree.
	s.False(HasAnySystemPermission([]string{"org", "org:user:view"}))

	InitControlPlanePermissions("mgmt")
	s.True(HasAnySystemPermission([]string{"mgmt:system:group:view"}))
	s.False(HasAnySystemPermission([]string{"system:group:view"}))
	InitControlPlanePermissions("")
}

func (s *SecurityContextTestSuite) TestHasSufficientSystemPermission_ControlPlane() {
	InitControlPlanePermissions("")

	s.True(HasSufficientSystemPermission([]string{"system"}, "system:user:view"))
	s.True(HasSufficientSystemPermission([]string{"system:user"}, "system:user:view"))
	s.False(HasSufficientSystemPermission([]string{"system:user:view"}, "system:user"))
	s.False(HasSufficientSystemPermission([]string{"org"}, "system:user:view"))
	s.True(HasSufficientSystemPermission(nil, ""))
}

// ---------------------------------------------------------------------------
// HasSufficientPermission
// ---------------------------------------------------------------------------

func (s *SecurityContextTestSuite) TestHeldPermissions() {
	current := PermissionSet{"rs-1": {"system:user"}, "rs-2": {"orders:read"}}

	held := HeldPermissions([]string{"openid", "system:user:view", "system:group:view", "orders:read"}, current)

	s.Equal([]string{"system:user:view", "orders:read"}, held)
	s.Empty(HeldPermissions([]string{"system"}, nil))
}

func (s *SecurityContextTestSuite) TestHasSufficientPermission() {
	tests := []struct {
		name            string
		userPermissions []string
		required        string
		want            bool
	}{
		// Empty required → always satisfied.
		{
			name:            "EmptyRequired_AlwaysSatisfied",
			userPermissions: []string{},
			required:        "",
			want:            true,
		},
		{
			name:            "EmptyRequired_NilPermissions_AlwaysSatisfied",
			userPermissions: nil,
			required:        "",
			want:            true,
		},
		// Exact match.
		{
			name:            "ExactMatch",
			userPermissions: []string{"system:ou:view"},
			required:        "system:ou:view",
			want:            true,
		},
		{
			name:            "ExactMatch_RootPermission",
			userPermissions: []string{"system"},
			required:        "system",
			want:            true,
		},
		// Parent scope covers child.
		{
			name:            "ParentSatisfiesImmediateChild",
			userPermissions: []string{"system:ou"},
			required:        "system:ou:view",
			want:            true,
		},
		{
			name:            "ParentSatisfiesDeepChild",
			userPermissions: []string{"system"},
			required:        "system:ou:view",
			want:            true,
		},
		{
			name:            "RootSatisfiesAnyDescendant",
			userPermissions: []string{"system"},
			required:        "system:user:view",
			want:            true,
		},
		// Child does NOT satisfy parent.
		{
			name:            "ChildDoesNotSatisfyParent",
			userPermissions: []string{"system:ou:view"},
			required:        "system:ou",
			want:            false,
		},
		{
			name:            "ChildDoesNotSatisfyRoot",
			userPermissions: []string{"system:ou"},
			required:        "system",
			want:            false,
		},
		// Unrelated scopes.
		{
			name:            "UnrelatedSiblingScope",
			userPermissions: []string{"system:user"},
			required:        "system:ou:view",
			want:            false,
		},
		// Multiple user permissions — at least one must satisfy.
		{
			name:            "OneOfMultiplePermissionsSatisfies",
			userPermissions: []string{"system:user", "system:ou"},
			required:        "system:ou:view",
			want:            true,
		},
		{
			name:            "NoneOfMultiplePermissionsSatisfy",
			userPermissions: []string{"system:user", "system:group"},
			required:        "system:ou:view",
			want:            false,
		},
		// Edge: partial prefix must not match.
		{
			name:            "PartialPrefixDoesNotMatch",
			userPermissions: []string{"sys"},
			required:        "system:ou",
			want:            false,
		},
		// Empty user permissions.
		{
			name:            "EmptyUserPermissions",
			userPermissions: []string{},
			required:        "system:ou",
			want:            false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, HasSufficientPermission(tt.userPermissions, tt.required))
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveActionPermission
// ---------------------------------------------------------------------------

func (s *SecurityContextTestSuite) TestResolveActionPermission() {
	InitControlPlanePermissions("")
	p := GetSystemPermissions()

	tests := []struct {
		name     string
		action   Action
		wantPerm string
	}{
		// OU actions.
		{name: "CreateOU", action: ActionCreateOU, wantPerm: p.OUCreate},
		{name: "ReadOU", action: ActionReadOU, wantPerm: p.OUView},
		{name: "UpdateOU", action: ActionUpdateOU, wantPerm: p.OUUpdate},
		{name: "DeleteOU", action: ActionDeleteOU, wantPerm: p.OUDelete},
		{name: "ListOUs", action: ActionListOUs, wantPerm: p.OUView},

		// User actions.
		{name: "CreateUser", action: ActionCreateUser, wantPerm: p.UserCreate},
		{name: "ReadUser", action: ActionReadUser, wantPerm: p.UserView},
		{name: "UpdateUser", action: ActionUpdateUser, wantPerm: p.UserUpdate},
		{name: "DeleteUser", action: ActionDeleteUser, wantPerm: p.UserDelete},
		{name: "ListUsers", action: ActionListUsers, wantPerm: p.UserView},

		// Group actions.
		{name: "CreateGroup", action: ActionCreateGroup, wantPerm: p.GroupCreate},
		{name: "ReadGroup", action: ActionReadGroup, wantPerm: p.GroupView},
		{name: "UpdateGroup", action: ActionUpdateGroup, wantPerm: p.GroupUpdate},
		{name: "DeleteGroup", action: ActionDeleteGroup, wantPerm: p.GroupDelete},
		{name: "ListGroups", action: ActionListGroups, wantPerm: p.GroupView},

		// Flow actions.
		{name: "CreateFlow", action: ActionCreateFlow, wantPerm: p.FlowCreate},
		{name: "ReadFlow", action: ActionReadFlow, wantPerm: p.FlowView},
		{name: "UpdateFlow", action: ActionUpdateFlow, wantPerm: p.FlowUpdate},
		{name: "DeleteFlow", action: ActionDeleteFlow, wantPerm: p.FlowDelete},
		{name: "ListFlows", action: ActionListFlows, wantPerm: p.FlowView},

		// Role actions.
		{name: "CreateRole", action: ActionCreateRole, wantPerm: p.RoleCreate},
		{name: "ReadRole", action: ActionReadRole, wantPerm: p.RoleView},
		{name: "UpdateRole", action: ActionUpdateRole, wantPerm: p.RoleUpdate},
		{name: "DeleteRole", action: ActionDeleteRole, wantPerm: p.RoleDelete},
		{name: "ListRoles", action: ActionListRoles, wantPerm: p.RoleView},

		// Resource server actions.
		{name: "CreateResourceServer", action: ActionCreateResourceServer, wantPerm: p.ResourceServerCreate},
		{name: "ReadResourceServer", action: ActionReadResourceServer, wantPerm: p.ResourceServerView},
		{name: "UpdateResourceServer", action: ActionUpdateResourceServer, wantPerm: p.ResourceServerUpdate},
		{name: "DeleteResourceServer", action: ActionDeleteResourceServer, wantPerm: p.ResourceServerDelete},
		{name: "ListResourceServers", action: ActionListResourceServers, wantPerm: p.ResourceServerView},

		// Connection actions.
		{name: "CreateConnection", action: ActionCreateConnection, wantPerm: p.ConnectionCreate},
		{name: "ReadConnection", action: ActionReadConnection, wantPerm: p.ConnectionView},
		{name: "UpdateConnection", action: ActionUpdateConnection, wantPerm: p.ConnectionUpdate},
		{name: "DeleteConnection", action: ActionDeleteConnection, wantPerm: p.ConnectionDelete},
		{name: "ListConnections", action: ActionListConnections, wantPerm: p.ConnectionView},

		// Verifiable credential actions.
		{name: "CreateVC", action: ActionCreateVC, wantPerm: p.VCCreate},
		{name: "ReadVC", action: ActionReadVC, wantPerm: p.VCView},
		{name: "UpdateVC", action: ActionUpdateVC, wantPerm: p.VCUpdate},
		{name: "DeleteVC", action: ActionDeleteVC, wantPerm: p.VCDelete},
		{name: "ListVCs", action: ActionListVCs, wantPerm: p.VCView},

		// Design actions.
		{name: "CreateDesign", action: ActionCreateDesign, wantPerm: p.DesignCreate},
		{name: "ReadDesign", action: ActionReadDesign, wantPerm: p.DesignView},
		{name: "UpdateDesign", action: ActionUpdateDesign, wantPerm: p.DesignUpdate},
		{name: "DeleteDesign", action: ActionDeleteDesign, wantPerm: p.DesignDelete},
		{name: "ListDesigns", action: ActionListDesigns, wantPerm: p.DesignView},

		// Translation actions.
		{name: "CreateI18n", action: ActionCreateI18n, wantPerm: p.I18nCreate},
		{name: "ReadI18n", action: ActionReadI18n, wantPerm: p.I18nView},
		{name: "UpdateI18n", action: ActionUpdateI18n, wantPerm: p.I18nUpdate},
		{name: "DeleteI18n", action: ActionDeleteI18n, wantPerm: p.I18nDelete},
		{name: "ListI18n", action: ActionListI18n, wantPerm: p.I18nView},

		// Gateway actions.
		{name: "CreateGateway", action: ActionCreateGateway, wantPerm: p.GatewayCreate},
		{name: "ReadGateway", action: ActionReadGateway, wantPerm: p.GatewayView},
		{name: "UpdateGateway", action: ActionUpdateGateway, wantPerm: p.GatewayUpdate},
		{name: "DeleteGateway", action: ActionDeleteGateway, wantPerm: p.GatewayDelete},
		{name: "ListGateways", action: ActionListGateways, wantPerm: p.GatewayView},

		// Server configuration actions.
		{name: "ReadServerConfig", action: ActionReadServerConfig, wantPerm: p.ServerConfigView},
		{name: "UpdateServerConfig", action: ActionUpdateServerConfig, wantPerm: p.ServerConfigUpdate},
		{name: "ListServerConfigs", action: ActionListServerConfigs, wantPerm: p.ServerConfigView},

		// Configuration version actions.
		{name: "CreateConfigVersion", action: ActionCreateConfigVersion, wantPerm: p.ConfigVersionCreate},
		{name: "ReadConfigVersion", action: ActionReadConfigVersion, wantPerm: p.ConfigVersionView},
		{name: "ListConfigVersions", action: ActionListConfigVersions, wantPerm: p.ConfigVersionView},

		// Authorization detail type actions.
		{name: "CreateAuthzDetail", action: ActionCreateAuthzDetail, wantPerm: p.AuthzDetailCreate},
		{name: "ReadAuthzDetail", action: ActionReadAuthzDetail, wantPerm: p.AuthzDetailView},
		{name: "UpdateAuthzDetail", action: ActionUpdateAuthzDetail, wantPerm: p.AuthzDetailUpdate},
		{name: "DeleteAuthzDetail", action: ActionDeleteAuthzDetail, wantPerm: p.AuthzDetailDelete},
		{name: "ListAuthzDetails", action: ActionListAuthzDetails, wantPerm: p.AuthzDetailView},

		// Application actions.
		{name: "CreateApplication", action: ActionCreateApplication, wantPerm: p.AppCreate},
		{name: "ReadApplication", action: ActionReadApplication, wantPerm: p.AppView},
		{name: "UpdateApplication", action: ActionUpdateApplication, wantPerm: p.AppUpdate},
		{name: "DeleteApplication", action: ActionDeleteApplication, wantPerm: p.AppDelete},
		{name: "ListApplications", action: ActionListApplications, wantPerm: p.AppView},

		// Unmapped action falls back to Root (system).
		{name: "UnmappedAction_FallsBackToSystem", action: Action("custom:unknown"), wantPerm: p.Root},
		{name: "EmptyAction_FallsBackToSystem", action: Action(""), wantPerm: p.Root},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.wantPerm, ResolveActionPermission(tt.action))
		})
	}
}

// TestResolveActionPermission_CoversAllMappedActions ensures every entry in
// actionPermissionMap is reachable and returns the expected permission.
func (s *SecurityContextTestSuite) TestResolveActionPermission_CoversAllMappedActions() {
	InitControlPlanePermissions("")
	for action, expectedPerm := range actionPermissionMap {
		s.Run(string(action), func() {
			s.Equal(expectedPerm, ResolveActionPermission(action))
		})
	}
}

// ---------------------------------------------------------------------------
// InitControlPlanePermissions
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// GetSystemRootPermission
// ---------------------------------------------------------------------------

func (s *SecurityContextTestSuite) TestGetSystemRootPermission_AfterInitEmptyHandle() {
	InitControlPlanePermissions("")
	s.Equal("system", GetSystemRootPermission())
}

func (s *SecurityContextTestSuite) TestGetSystemRootPermission_AfterInitNonEmptyHandle() {
	InitControlPlanePermissions("mgmt")
	defer InitControlPlanePermissions("")
	s.Equal("mgmt:system", GetSystemRootPermission())
}

// ---------------------------------------------------------------------------

func TestInitControlPlanePermissions_EmptyHandle(t *testing.T) {
	InitControlPlanePermissions("")
	p := GetSystemPermissions()
	require.NotNil(t, p)

	assert.Equal(t, "system", p.Root)
	assert.Empty(t, p.Org)
	assert.Equal(t, "system", p.AdministrationFlow)
	assert.Equal(t, "system:app:create", p.AppCreate)
	assert.Equal(t, "system:design:view", p.DesignView)
	assert.Equal(t, "system:ou", p.OU)
	assert.Equal(t, "system:ou:view", p.OUView)
	assert.Equal(t, "system:user", p.User)
	assert.Equal(t, "system:user:view", p.UserView)
	assert.Equal(t, "system:group", p.Group)
	assert.Equal(t, "system:group:view", p.GroupView)
	assert.Equal(t, "system:usertype", p.UserType)
	assert.Equal(t, "system:usertype:view", p.UserTypeView)
	assert.Equal(t, "system:agenttype", p.AgentType)
	assert.Equal(t, "system:agenttype:view", p.AgentTypeView)
}

func TestInitControlPlanePermissions_NonEmptyHandle(t *testing.T) {
	InitControlPlanePermissions("mgmt")
	p := GetSystemPermissions()
	require.NotNil(t, p)

	assert.Equal(t, "mgmt:system", p.Root)
	assert.Equal(t, "mgmt:system:ou", p.OU)
	assert.Equal(t, "mgmt:system:ou:view", p.OUView)
	assert.Equal(t, "mgmt:system:user", p.User)
	assert.Equal(t, "mgmt:system:user:view", p.UserView)
	assert.Equal(t, "mgmt:system:group", p.Group)
	assert.Equal(t, "mgmt:system:group:view", p.GroupView)
	assert.Equal(t, "mgmt:system:usertype", p.UserType)
	assert.Equal(t, "mgmt:system:usertype:view", p.UserTypeView)
	assert.Equal(t, "mgmt:system:agenttype", p.AgentType)
	assert.Equal(t, "mgmt:system:agenttype:view", p.AgentTypeView)

	// Restore default for other tests.
	InitControlPlanePermissions("")
}

func TestInitControlPlanePermissions_RebuildsActionMap(t *testing.T) {
	InitControlPlanePermissions("x")
	assert.Equal(t, "x:system:ou:create", ResolveActionPermission(ActionCreateOU))

	InitControlPlanePermissions("")
	assert.Equal(t, "system:ou:create", ResolveActionPermission(ActionCreateOU))
}

func TestHasSystemPermission_WithCustomHandle(t *testing.T) {
	InitControlPlanePermissions("mgmt")
	defer InitControlPlanePermissions("")

	assert.True(t, HasSystemPermission([]string{"mgmt:system"}))
	assert.False(t, HasSystemPermission([]string{"system"}))
}

// ---------------------------------------------------------------------------
// GetRequiredPermissionForAPI
// ---------------------------------------------------------------------------

func TestGetRequiredPermissionForAPI(t *testing.T) {
	InitControlPlanePermissions("")
	p := GetSystemPermissions()

	svc, err := newSecurityService(nil, nil, []string{}, apiPermissionEntries)
	require.NoError(t, err)

	tests := []struct {
		name     string
		method   string
		path     string
		wantPerm string
	}{
		// ---- Exact matches ----
		{
			name:   "GET /organization-units exact",
			method: http.MethodGet, path: "/organization-units", wantPerm: p.OUView,
		},
		{
			name:   "POST /organization-units exact",
			method: http.MethodPost, path: "/organization-units", wantPerm: p.OUCreate,
		},
		{name: "GET /users exact", method: http.MethodGet, path: "/users", wantPerm: p.UserView},
		{name: "POST /users exact", method: http.MethodPost, path: "/users", wantPerm: p.UserCreate},
		{name: "GET /groups exact", method: http.MethodGet, path: "/groups", wantPerm: p.GroupView},
		{name: "POST /groups exact", method: http.MethodPost, path: "/groups", wantPerm: p.GroupCreate},

		// ---- Notification template APIs ----
		{
			name:   "GET notification-templates list",
			method: http.MethodGet, path: "/notification-templates/email/templates",
			wantPerm: p.NotificationTemplateView,
		},
		{
			name:   "POST notification-templates create",
			method: http.MethodPost, path: "/notification-templates/sms/templates",
			wantPerm: p.NotificationTemplateCreate,
		},
		{
			name:   "GET notification-template by id",
			method: http.MethodGet, path: "/notification-templates/email/templates/tmpl-1",
			wantPerm: p.NotificationTemplateView,
		},
		{
			name:   "PUT notification-template by id",
			method: http.MethodPut, path: "/notification-templates/email/templates/tmpl-1",
			wantPerm: p.NotificationTemplateUpdate,
		},
		{
			name:   "DELETE notification-template by id",
			method: http.MethodDelete, path: "/notification-templates/sms/templates/tmpl-2",
			wantPerm: p.NotificationTemplateDelete,
		},

		// ---- User and group sub-resources ----
		{
			name:   "POST user by OU path creates",
			method: http.MethodPost, path: "/users/tree/root", wantPerm: p.UserCreate,
		},
		{
			name:   "POST user credentials updates the user",
			method: http.MethodPost, path: "/users/u-1/update-credentials", wantPerm: p.UserUpdate,
		},
		{
			name:   "POST user sharing policy updates the user",
			method: http.MethodPost, path: "/users/u-1/sharing-policies", wantPerm: p.UserUpdate,
		},
		{
			name:   "DELETE user sharing policy updates the user",
			method: http.MethodDelete, path: "/users/u-1/sharing-policies/p-1", wantPerm: p.UserUpdate,
		},
		{
			name:   "POST group by OU path creates",
			method: http.MethodPost, path: "/groups/tree/root", wantPerm: p.GroupCreate,
		},
		{
			name:   "POST group members updates the group",
			method: http.MethodPost, path: "/groups/g-1/members/add", wantPerm: p.GroupUpdate,
		},
		{name: "PATCH SCIM user updates", method: http.MethodPatch, path: "/scim/v2/Users/u-1", wantPerm: p.UserUpdate},
		{name: "DELETE agent", method: http.MethodDelete, path: "/agents/a-1", wantPerm: p.AgentDelete},
		{name: "PUT user type", method: http.MethodPut, path: "/user-types/t-1", wantPerm: p.UserTypeUpdate},
		{name: "POST agent type", method: http.MethodPost, path: "/agent-types", wantPerm: p.AgentTypeCreate},

		// ---- Role APIs ----
		{name: "GET /roles", method: http.MethodGet, path: "/roles", wantPerm: p.RoleView},
		{name: "POST /roles", method: http.MethodPost, path: "/roles", wantPerm: p.RoleCreate},
		{name: "GET role", method: http.MethodGet, path: "/roles/r-1", wantPerm: p.RoleView},
		{name: "PUT role", method: http.MethodPut, path: "/roles/r-1", wantPerm: p.RoleUpdate},
		{name: "DELETE role", method: http.MethodDelete, path: "/roles/r-1", wantPerm: p.RoleDelete},
		{
			name:   "POST role assignments add updates the role",
			method: http.MethodPost, path: "/roles/r-1/assignments/add", wantPerm: p.RoleUpdate,
		},
		{
			name:   "POST role assignments remove updates the role",
			method: http.MethodPost, path: "/roles/r-1/assignments/remove", wantPerm: p.RoleUpdate,
		},
		{
			name:   "DELETE role sharing policy updates the role",
			method: http.MethodDelete, path: "/roles/r-1/sharing-policies/p-1", wantPerm: p.RoleUpdate,
		},

		// ---- Resource server APIs ----
		{
			name:   "GET /resource-servers",
			method: http.MethodGet, path: "/resource-servers", wantPerm: p.ResourceServerView,
		},
		{
			name:   "POST /resource-servers",
			method: http.MethodPost, path: "/resource-servers", wantPerm: p.ResourceServerCreate,
		},
		{
			name:   "GET resource server",
			method: http.MethodGet, path: "/resource-servers/rs-1", wantPerm: p.ResourceServerView,
		},
		{
			name:   "PUT resource server",
			method: http.MethodPut, path: "/resource-servers/rs-1", wantPerm: p.ResourceServerUpdate,
		},
		{
			name:   "DELETE resource server",
			method: http.MethodDelete, path: "/resource-servers/rs-1", wantPerm: p.ResourceServerDelete,
		},
		{
			name:   "POST resource updates the resource server",
			method: http.MethodPost, path: "/resource-servers/rs-1/resources", wantPerm: p.ResourceServerUpdate,
		},
		{
			name:   "POST resource action updates the resource server",
			method: http.MethodPost, path: "/resource-servers/rs-1/resources/r-1/actions",
			wantPerm: p.ResourceServerUpdate,
		},
		{
			name:   "DELETE resource updates the resource server",
			method: http.MethodDelete, path: "/resource-servers/rs-1/resources/r-1",
			wantPerm: p.ResourceServerUpdate,
		},
		{
			name:   "DELETE action updates the resource server",
			method: http.MethodDelete, path: "/resource-servers/rs-1/actions/a-1", wantPerm: p.ResourceServerUpdate,
		},

		// ---- Connection APIs ----
		{name: "GET /connections", method: http.MethodGet, path: "/connections", wantPerm: p.ConnectionView},
		{name: "GET connection meta", method: http.MethodGet, path: "/connections/meta", wantPerm: p.ConnectionView},
		{
			name:   "GET vendor connections",
			method: http.MethodGet, path: "/connections/google", wantPerm: p.ConnectionView,
		},
		{name: "POST connection", method: http.MethodPost, path: "/connections/google", wantPerm: p.ConnectionCreate},
		{name: "GET connection", method: http.MethodGet, path: "/connections/google/c-1", wantPerm: p.ConnectionView},
		{name: "PUT connection", method: http.MethodPut, path: "/connections/google/c-1", wantPerm: p.ConnectionUpdate},
		{
			name:   "DELETE connection",
			method: http.MethodDelete, path: "/connections/google/c-1", wantPerm: p.ConnectionDelete,
		},
		{
			name:   "POST connection sharing policy updates the connection",
			method: http.MethodPost, path: "/connections/http/c-1/sharing-policies", wantPerm: p.ConnectionUpdate,
		},
		{
			name:   "PUT connection sharing policy updates the connection",
			method: http.MethodPut, path: "/connections/http/c-1/sharing-policies/p-1", wantPerm: p.ConnectionUpdate,
		},
		{
			name:   "DELETE connection sharing policy updates the connection",
			method: http.MethodDelete, path: "/connections/http/c-1/sharing-policies/p-1",
			wantPerm: p.ConnectionUpdate,
		},

		// ---- Verifiable credential APIs ----
		{
			name:   "GET credential configurations",
			method: http.MethodGet, path: "/openid4vci/credential-configurations", wantPerm: p.VCView,
		},
		{
			name:   "POST credential configuration",
			method: http.MethodPost, path: "/openid4vci/credential-configurations", wantPerm: p.VCCreate,
		},
		{
			name:   "PUT credential configuration",
			method: http.MethodPut, path: "/openid4vci/credential-configurations/c-1", wantPerm: p.VCUpdate,
		},
		{
			name:   "DELETE credential configuration",
			method: http.MethodDelete, path: "/openid4vci/credential-configurations/c-1", wantPerm: p.VCDelete,
		},
		{
			name:   "GET presentation definition",
			method: http.MethodGet, path: "/openid4vp/presentation-definitions/d-1", wantPerm: p.VCView,
		},
		{
			name:   "POST presentation definition",
			method: http.MethodPost, path: "/openid4vp/presentation-definitions", wantPerm: p.VCCreate,
		},
		{
			name:   "DELETE presentation definition",
			method: http.MethodDelete, path: "/openid4vp/presentation-definitions/d-1", wantPerm: p.VCDelete,
		},
		{name: "GET trust anchors", method: http.MethodGet, path: "/openid4vp/trust-anchors", wantPerm: p.VCView},

		// ---- Design APIs ----
		{name: "GET /design/themes", method: http.MethodGet, path: "/design/themes", wantPerm: p.DesignView},
		{name: "POST /design/themes", method: http.MethodPost, path: "/design/themes", wantPerm: p.DesignCreate},
		{name: "GET theme usages", method: http.MethodGet, path: "/design/themes/t-1/usages", wantPerm: p.DesignView},
		{name: "PUT layout", method: http.MethodPut, path: "/design/layouts/l-1", wantPerm: p.DesignUpdate},
		{name: "DELETE layout", method: http.MethodDelete, path: "/design/layouts/l-1", wantPerm: p.DesignDelete},

		// ---- Translation APIs ----
		{
			name:   "POST language translations updates",
			method: http.MethodPost, path: "/i18n/languages/en/translations", wantPerm: p.I18nUpdate,
		},
		{
			name:   "DELETE language translations",
			method: http.MethodDelete, path: "/i18n/languages/en/translations", wantPerm: p.I18nDelete,
		},
		{
			name:   "POST translation key updates",
			method: http.MethodPost, path: "/i18n/languages/en/translations/ns/app/keys/title", wantPerm: p.I18nUpdate,
		},
		{
			name:   "DELETE translation key",
			method: http.MethodDelete, path: "/i18n/languages/en/translations/ns/app/keys/title",
			wantPerm: p.I18nDelete,
		},

		// ---- Gateway APIs ----
		{name: "GET /gateways", method: http.MethodGet, path: "/gateways", wantPerm: p.GatewayView},
		{name: "POST /gateways", method: http.MethodPost, path: "/gateways", wantPerm: p.GatewayCreate},
		{name: "GET gateway diff", method: http.MethodGet, path: "/gateways/g-1/diff", wantPerm: p.GatewayView},
		{name: "PUT gateway", method: http.MethodPut, path: "/gateways/g-1", wantPerm: p.GatewayUpdate},
		{name: "DELETE gateway", method: http.MethodDelete, path: "/gateways/g-1", wantPerm: p.GatewayDelete},
		{
			name:   "POST gateway apply updates",
			method: http.MethodPost, path: "/gateways/g-1/apply", wantPerm: p.GatewayUpdate,
		},
		{
			name:   "POST gateway revert updates",
			method: http.MethodPost, path: "/gateways/g-1/revert", wantPerm: p.GatewayUpdate,
		},
		{name: "GET /projects", method: http.MethodGet, path: "/projects", wantPerm: p.ProjectView},
		{name: "POST /projects", method: http.MethodPost, path: "/projects", wantPerm: p.ProjectCreate},
		{name: "GET project", method: http.MethodGet, path: "/projects/p-1", wantPerm: p.ProjectView},
		{name: "PUT project", method: http.MethodPut, path: "/projects/p-1", wantPerm: p.ProjectUpdate},
		{name: "DELETE project", method: http.MethodDelete, path: "/projects/p-1", wantPerm: p.ProjectDelete},
		{
			name:   "GET gateway variables needs root",
			method: http.MethodGet, path: "/gateways/g-1/variables", wantPerm: p.Root,
		},
		{
			name:   "PUT gateway secret needs root",
			method: http.MethodPut, path: "/gateways/g-1/secrets/db-password", wantPerm: p.Root,
		},
		{
			name:   "POST gateway secret needs root",
			method: http.MethodPost, path: "/gateways/g-1/secrets", wantPerm: p.Root,
		},

		// ---- Server configuration APIs ----
		{name: "GET /server-config", method: http.MethodGet, path: "/server-config", wantPerm: p.ServerConfigView},
		{name: "GET server config", method: http.MethodGet, path: "/server-config/cors", wantPerm: p.ServerConfigView},
		{
			name:   "PUT server config",
			method: http.MethodPut, path: "/server-config/cors", wantPerm: p.ServerConfigUpdate,
		},
		{
			name:   "DELETE server config has no rule",
			method: http.MethodDelete, path: "/server-config/cors", wantPerm: p.Root,
		},

		// ---- Configuration version APIs ----
		{
			name:   "GET /configuration-versions",
			method: http.MethodGet, path: "/configuration-versions", wantPerm: p.ConfigVersionView,
		},
		{
			name:   "POST /configuration-versions",
			method: http.MethodPost, path: "/configuration-versions", wantPerm: p.ConfigVersionCreate,
		},
		{
			name:   "GET configuration version",
			method: http.MethodGet, path: "/configuration-versions/3", wantPerm: p.ConfigVersionView,
		},

		// ---- Authorization detail type APIs ----
		{
			name:   "GET /authorization-detail-types",
			method: http.MethodGet, path: "/authorization-detail-types", wantPerm: p.AuthzDetailView,
		},
		{
			name:   "POST /authorization-detail-types",
			method: http.MethodPost, path: "/authorization-detail-types", wantPerm: p.AuthzDetailCreate,
		},
		{
			name:   "PUT authorization detail type",
			method: http.MethodPut, path: "/authorization-detail-types/adt-1", wantPerm: p.AuthzDetailUpdate,
		},
		{
			name:   "DELETE authorization detail type",
			method: http.MethodDelete, path: "/authorization-detail-types/adt-1", wantPerm: p.AuthzDetailDelete,
		},

		// ---- Flow APIs ----
		{name: "GET /flows", method: http.MethodGet, path: "/flows", wantPerm: p.FlowView},
		{name: "POST /flows", method: http.MethodPost, path: "/flows", wantPerm: p.FlowCreate},
		{name: "GET flow", method: http.MethodGet, path: "/flows/f-1", wantPerm: p.FlowView},
		{name: "GET flow version", method: http.MethodGet, path: "/flows/f-1/versions/2", wantPerm: p.FlowView},
		{name: "PUT flow", method: http.MethodPut, path: "/flows/f-1", wantPerm: p.FlowUpdate},
		{name: "PUT flow variables", method: http.MethodPut, path: "/flows/f-1/variables", wantPerm: p.FlowUpdate},
		{name: "DELETE flow", method: http.MethodDelete, path: "/flows/f-1", wantPerm: p.FlowDelete},
		{
			name:   "POST flow restore updates the flow",
			method: http.MethodPost, path: "/flows/f-1/restore", wantPerm: p.FlowUpdate,
		},
		{
			name:   "POST flow sharing policy updates the flow",
			method: http.MethodPost, path: "/flows/f-1/sharing-policies", wantPerm: p.FlowUpdate,
		},
		{
			name:   "DELETE flow sharing policy updates the flow",
			method: http.MethodDelete, path: "/flows/f-1/sharing-policies/p-1", wantPerm: p.FlowUpdate,
		},

		// ---- Application APIs ----
		{name: "GET /applications", method: http.MethodGet, path: "/applications", wantPerm: p.AppView},
		{name: "POST /applications", method: http.MethodPost, path: "/applications", wantPerm: p.AppCreate},
		{name: "GET application", method: http.MethodGet, path: "/applications/app-1", wantPerm: p.AppView},
		{name: "PUT application", method: http.MethodPut, path: "/applications/app-1", wantPerm: p.AppUpdate},
		{
			name:   "DELETE application",
			method: http.MethodDelete, path: "/applications/app-1", wantPerm: p.AppDelete,
		},
		{
			name:   "GET application sharing policies",
			method: http.MethodGet, path: "/applications/app-1/sharing-policies", wantPerm: p.AppView,
		},
		{
			name:   "POST application sharing policy updates the application",
			method: http.MethodPost, path: "/applications/app-1/sharing-policies", wantPerm: p.AppUpdate,
		},
		{
			name:   "PUT application sharing policy updates the application",
			method: http.MethodPut, path: "/applications/app-1/sharing-policies/p-1", wantPerm: p.AppUpdate,
		},
		{
			name:   "DELETE application sharing policy updates the application",
			method: http.MethodDelete, path: "/applications/app-1/sharing-policies/p-1", wantPerm: p.AppUpdate,
		},
		{
			name:   "GET application overlay rules",
			method: http.MethodGet, path: "/applications/app-1/overlay-rules", wantPerm: p.AppView,
		},
		{
			name:   "POST application client secret regeneration updates the application",
			method: http.MethodPost, path: "/applications/app-1/client-secret/regenerate", wantPerm: p.AppUpdate,
		},
		{
			name:   "POST agent client secret regeneration updates the agent",
			method: http.MethodPost, path: "/agents/agent-1/client-secret/regenerate", wantPerm: p.AgentUpdate,
		},
		{
			name:   "POST CIMD preview needs app create",
			method: http.MethodPost, path: "/cimd/preview", wantPerm: p.AppCreate,
		},
		{
			name:   "PATCH application has no rule and falls back to system",
			method: http.MethodPatch, path: "/applications/app-1", wantPerm: p.Root,
		},

		// ---- Self-service paths (empty permission = any authenticated user) ----
		{name: "GET /users/me self-service", method: http.MethodGet, path: "/users/me", wantPerm: ""},
		{name: "PUT /users/me self-service", method: http.MethodPut, path: "/users/me", wantPerm: ""},
		{name: "GET /capabilities self-service", method: http.MethodGet, path: "/capabilities", wantPerm: ""},
		{
			name:     "POST /users/me/update-credentials self-service",
			method:   http.MethodPost,
			path:     "/users/me/update-credentials",
			wantPerm: "",
		},
		{
			name:   "GET /users/me/agents self-service",
			method: http.MethodGet, path: "/users/me/agents", wantPerm: "",
		},
		{
			name:   "PUT /users/me/agents/{id} self-service",
			method: http.MethodPut, path: "/users/me/agents/agent-1", wantPerm: "",
		},
		{
			name:   "DELETE /users/me/agents/{id} self-service",
			method: http.MethodDelete, path: "/users/me/agents/agent-1", wantPerm: "",
		},
		{
			name:   "POST /users/me/agents/suspend self-service",
			method: http.MethodPost, path: "/users/me/agents/suspend", wantPerm: "",
		},
		{
			name:   "POST /users/me/agents/{id}/suspend self-service",
			method: http.MethodPost, path: "/users/me/agents/agent-1/suspend", wantPerm: "",
		},
		{
			name:   "POST /users/me/agents/{id}/unsuspend self-service",
			method: http.MethodPost, path: "/users/me/agents/agent-1/unsuspend", wantPerm: "",
		},
		{
			name:   "DELETE deeper under /users/me/agents keeps the user delete rule",
			method: http.MethodDelete, path: "/users/me/agents/agent-1/extra", wantPerm: p.UserDelete,
		},
		{
			name:   "DELETE /users/{id} keeps the user delete rule",
			method: http.MethodDelete, path: "/users/user-1", wantPerm: p.UserDelete,
		},
		{
			name:   "GET /register/passkey/start self-service",
			method: http.MethodGet, path: "/register/passkey/start", wantPerm: "",
		},
		{
			name:   "POST /register/passkey/finish self-service",
			method: http.MethodPost, path: "/register/passkey/finish", wantPerm: "",
		},

		// ---- Prefix match — dynamic path segments ----
		{
			name:   "GET /organization-units/{id} prefix",
			method: http.MethodGet, path: "/organization-units/ou-123", wantPerm: p.OUView,
		},
		{
			name:   "PUT /organization-units/{id} prefix",
			method: http.MethodPut, path: "/organization-units/ou-123", wantPerm: p.OUUpdate,
		},
		{
			name:   "DELETE /organization-units/{id} prefix",
			method: http.MethodDelete, path: "/organization-units/ou-123", wantPerm: p.OUDelete,
		},
		{
			name:   "GET /users/{id} prefix",
			method: http.MethodGet, path: "/users/user-456", wantPerm: p.UserView,
		},
		{
			name:   "GET /users/{id}/ous needs root, not user view",
			method: http.MethodGet, path: "/users/user-456/ous", wantPerm: p.Root,
		},
		{
			name:   "PUT /users/{id} prefix",
			method: http.MethodPut, path: "/users/user-456", wantPerm: p.UserUpdate,
		},
		{
			name:   "DELETE /users/{id} prefix",
			method: http.MethodDelete, path: "/users/user-789", wantPerm: p.UserDelete,
		},
		{
			name:   "GET /groups/{id} prefix",
			method: http.MethodGet, path: "/groups/grp-111", wantPerm: p.GroupView,
		},
		{
			name:   "DELETE /groups/{id} prefix",
			method: http.MethodDelete, path: "/groups/grp-222", wantPerm: p.GroupDelete,
		},

		// ---- Self-service wins over parent prefix ----
		{name: "GET /users/me wins over /users/ prefix", method: http.MethodGet, path: "/users/me", wantPerm: ""},
		{
			name:   "GET /users/me/profile wins over /users/ prefix",
			method: http.MethodGet, path: "/users/me/profile", wantPerm: "",
		},

		// ---- OU tree paths ----
		{
			name:   "GET /organization-units/tree",
			method: http.MethodGet, path: "/organization-units/tree/root/child", wantPerm: p.OUView,
		},
		{
			name:   "PUT /organization-units/tree",
			method: http.MethodPut, path: "/organization-units/tree/root/child", wantPerm: p.OUUpdate,
		},
		{
			name:   "DELETE /organization-units/tree",
			method: http.MethodDelete, path: "/organization-units/tree/root/child", wantPerm: p.OUDelete,
		},

		// ---- Unmapped paths fall back to Root ----
		{
			name:   "Unmapped path falls back to system",
			method: http.MethodGet, path: "/unknown-resources", wantPerm: p.Root,
		},
		{name: "Root path falls back to system", method: http.MethodGet, path: "/", wantPerm: p.Root},
		{
			name:   "Unknown POST falls back to system",
			method: http.MethodPost, path: "/configs", wantPerm: p.Root,
		},
		{
			name:   "GET /users/menu matches users wildcard",
			method: http.MethodGet, path: "/users/menu", wantPerm: p.UserView,
		},

		// ---- Wrong method does not match mapped path ----
		{
			name:   "PATCH /users unmapped method falls back to system",
			method: http.MethodPatch, path: "/users", wantPerm: p.Root,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantPerm, svc.getRequiredPermissionForAPI(tt.method, tt.path))
		})
	}
}

// ---------------------------------------------------------------------------
// Covers
// ---------------------------------------------------------------------------

const (
	rsSystem  = "01900000-0000-7000-8000-000000000020"
	rsPayroll = "01900000-0000-7000-8000-0000000000aa"
)

func TestCovers(t *testing.T) {
	InitControlPlanePermissions("")

	tests := []struct {
		name     string
		actor    PermissionSet
		required PermissionSet
		want     bool
	}{
		// ---- Vacuous coverage ----
		{
			name:     "NilRequiredIsCovered",
			actor:    nil,
			required: nil,
			want:     true,
		},
		{
			name:     "EmptyRequiredIsCovered",
			actor:    PermissionSet{},
			required: PermissionSet{},
			want:     true,
		},
		{
			name:     "RequiredEntryWithNoPermissionsContributesNothing",
			actor:    PermissionSet{},
			required: PermissionSet{rsSystem: {}},
			want:     true,
		},
		{
			name:     "NilActorCannotCoverNonEmptyRequirement",
			actor:    nil,
			required: PermissionSet{rsSystem: {"system:user"}},
			want:     false,
		},

		// ---- Exact and hierarchical matching within a resource server ----
		{
			name:     "ExactMatch",
			actor:    PermissionSet{rsSystem: {"system:group"}},
			required: PermissionSet{rsSystem: {"system:group"}},
			want:     true,
		},
		{
			name:     "ParentCoversChild",
			actor:    PermissionSet{rsSystem: {"system:user"}},
			required: PermissionSet{rsSystem: {"system:user:view"}},
			want:     true,
		},
		{
			name:     "ChildDoesNotCoverParent",
			actor:    PermissionSet{rsSystem: {"system:user:view"}},
			required: PermissionSet{rsSystem: {"system:user"}},
			want:     false,
		},
		{
			name:     "RootCoversEverythingBeneathIt",
			actor:    PermissionSet{rsSystem: {"system"}},
			required: PermissionSet{rsSystem: {"system:user", "system:group", "system:ou:view"}},
			want:     true,
		},
		{
			name:     "SiblingDoesNotCover",
			actor:    PermissionSet{rsSystem: {"system:group"}},
			required: PermissionSet{rsSystem: {"system:user"}},
			want:     false,
		},
		{
			name:     "PartialCoverageIsNotCoverage",
			actor:    PermissionSet{rsSystem: {"system:group"}},
			required: PermissionSet{rsSystem: {"system:group", "system:user"}},
			want:     false,
		},
		{
			name:     "StringPrefixIsNotScopePrefix",
			actor:    PermissionSet{rsSystem: {"system:use"}},
			required: PermissionSet{rsSystem: {"system:user"}},
			want:     false,
		},

		// ---- Incomparable sets ----
		{
			name:     "AssistantCannotGrantLibrarian",
			actor:    PermissionSet{rsSystem: {"system:group", "system:user:view"}},
			required: PermissionSet{rsSystem: {"system:user", "system:group"}},
			want:     false,
		},
		{
			name:     "AssistantCanGrantWhatItAlreadyHolds",
			actor:    PermissionSet{rsSystem: {"system:group", "system:user:view"}},
			required: PermissionSet{rsSystem: {"system:group", "system:user:view"}},
			want:     true,
		},

		// ---- Coverage never crosses resource servers ----
		{
			name:     "RootOnOneResourceServerDoesNotCoverAnother",
			actor:    PermissionSet{rsSystem: {"system"}},
			required: PermissionSet{rsPayroll: {"payroll:read"}},
			want:     false,
		},
		{
			name:     "IdenticalStringOnDifferentResourceServerDoesNotCover",
			actor:    PermissionSet{rsSystem: {"system:user"}},
			required: PermissionSet{rsPayroll: {"system:user"}},
			want:     false,
		},
		{
			name: "CoverageIsPerResourceServer",
			actor: PermissionSet{
				rsSystem:  {"system:group"},
				rsPayroll: {"payroll"},
			},
			required: PermissionSet{
				rsSystem:  {"system:group"},
				rsPayroll: {"payroll:read"},
			},
			want: true,
		},
		{
			name: "OneUncoveredResourceServerFailsTheWholeGrant",
			actor: PermissionSet{
				rsSystem:  {"system"},
				rsPayroll: {"payroll:read"},
			},
			required: PermissionSet{
				rsSystem:  {"system:user"},
				rsPayroll: {"payroll:write"},
			},
			want: false,
		},

		// ---- Malformed requirements are never covered ----
		{
			name:     "EmptyRequiredPermissionStringIsNeverCovered",
			actor:    PermissionSet{rsSystem: {"system"}},
			required: PermissionSet{rsSystem: {""}},
			want:     false,
		},
		{
			name:     "EmptyPermissionAmongValidOnesFailsTheGrant",
			actor:    PermissionSet{rsSystem: {"system"}},
			required: PermissionSet{rsSystem: {"system:user", ""}},
			want:     false,
		},
		{
			name:     "EmptyResourceServerIDIsNeverCovered",
			actor:    PermissionSet{"": {"system"}},
			required: PermissionSet{"": {"system"}},
			want:     false,
		},

		// ---- Extra actor permissions are harmless ----
		{
			name:     "ActorMayHoldMoreThanRequired",
			actor:    PermissionSet{rsSystem: {"system:group", "system:user", "system:ou"}},
			required: PermissionSet{rsSystem: {"system:group"}},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Covers(tt.actor, tt.required, map[string]string{rsSystem: ":", rsPayroll: ":"}))
		})
	}
}

// TestCoversIsNotSymmetric documents that coverage is a partial order, not an equivalence:
// incomparable sets cover each other in neither direction.
func TestCoversIsNotSymmetric(t *testing.T) {
	InitControlPlanePermissions("")

	groupAdmin := PermissionSet{rsSystem: {"system:group"}}
	userAdmin := PermissionSet{rsSystem: {"system:user"}}

	systemDelimiters := map[string]string{rsSystem: ":"}

	assert.False(t, Covers(groupAdmin, userAdmin, systemDelimiters), "group admin must not cover user admin")
	assert.False(t, Covers(userAdmin, groupAdmin, systemDelimiters), "user admin must not cover group admin")
}

// TestCoversHonoursConfiguredPermissionPrefix verifies that coverage is computed from the
// permission strings themselves, so a non-empty SystemPermissionPrefix needs no special handling.
func TestCoversHonoursConfiguredPermissionPrefix(t *testing.T) {
	InitControlPlanePermissions("mgmt")
	t.Cleanup(func() { InitControlPlanePermissions("") })

	p := GetSystemPermissions()
	require.Equal(t, "mgmt:system", p.Root)

	systemDelimiters := map[string]string{rsSystem: ":"}

	assert.True(t, Covers(
		PermissionSet{rsSystem: {p.Root}},
		PermissionSet{rsSystem: {p.User, p.GroupView}},
		systemDelimiters,
	))
	assert.False(t, Covers(
		PermissionSet{rsSystem: {p.UserView}},
		PermissionSet{rsSystem: {p.User}},
		systemDelimiters,
	))
	// An unprefixed permission must not satisfy a prefixed requirement.
	assert.False(t, Covers(
		PermissionSet{rsSystem: {"system"}},
		PermissionSet{rsSystem: {p.User}},
		systemDelimiters,
	))
}

// ---------------------------------------------------------------------------
// Permission catalog
// ---------------------------------------------------------------------------

const (
	bootstrapBundlePath = "../../../cmd/server/bootstrap/01-default-resources.yaml"
	// controlPlaneResourceServerPath is the Control Plane resource server, imported into the
	// identity provider that signs operators into the Control Plane.
	controlPlaneResourceServerPath = "../../../cmd/cpserver/idp-bootstrap/control-plane-resource-server.yaml"
	rsControlPlane                 = "01900000-0000-7000-8000-000000000021"
)

// declaredPermissions returns the delimiter of the resource server with the given ID in the bundle at
// path, and every permission it declares other than the system root. The first resource must be the
// system root. Every other resource either is one of roots, with no parent, or sits directly under the
// system root or one of roots.
func declaredPermissions(t *testing.T, path, id string, roots ...string) (string, []string) {
	t.Helper()
	content, err := os.ReadFile(path) // #nosec G304 -- test reads a fixed bundle in the repository.
	require.NoError(t, err)

	type action struct {
		Handle string `yaml:"handle"`
	}
	type resource struct {
		Name    string   `yaml:"name"`
		Handle  string   `yaml:"handle"`
		Parent  string   `yaml:"parent"`
		Actions []action `yaml:"actions"`
	}
	var server struct {
		ID        string     `yaml:"id"`
		Delimiter string     `yaml:"delimiter"`
		Resources []resource `yaml:"resources"`
	}
	for _, doc := range strings.Split(string(content), "\n---\n") {
		if !strings.Contains(doc, "id: "+id) {
			continue
		}
		require.NoError(t, yaml.Unmarshal([]byte(doc), &server))
	}
	require.Equal(t, id, server.ID, "resource server %s not found in %s", id, path)
	require.NotEmpty(t, server.Resources)
	require.Equal(t, resource{Name: "System", Handle: "system"}, server.Resources[0])

	parents := append([]string{"system"}, roots...)
	var permissions []string
	for _, res := range server.Resources[1:] {
		require.NotEmpty(t, res.Name, "resource %q needs a display name", res.Handle)
		if res.Parent == "" {
			require.Contains(t, roots, res.Handle, "resource %q must not be a root", res.Handle)
			permissions = append(permissions, res.Handle)
			continue
		}
		require.Contains(t, parents, res.Parent,
			"resource %q must sit directly under one of %v", res.Handle, parents)
		resourcePath := res.Parent + server.Delimiter + res.Handle
		permissions = append(permissions, resourcePath)
		for _, a := range res.Actions {
			permissions = append(permissions, resourcePath+server.Delimiter+a.Handle)
		}
	}
	return server.Delimiter, permissions
}

// TestDataPlaneCatalogMatchesBootstrapBundle verifies that the System resource server in the bootstrap
// bundle is the Data Plane's representation: the system root with no children, the org root and its
// children. Both planes import this bundle, and the Control Plane publishes roles built on it to the
// Data Planes.
func TestDataPlaneCatalogMatchesBootstrapBundle(t *testing.T) {
	InitDataPlanePermissions("")
	defer InitControlPlanePermissions("")

	delimiter, declared := declaredPermissions(t, bootstrapBundlePath, rsSystem, "org")
	assert.Equal(t, ":", delimiter)
	assert.ElementsMatch(t, GetSystemPermissions().Catalog, declared)
}

// TestControlPlaneCatalogMatchesResourceServer verifies that the Control Plane resource server, which
// the identity provider of the Control Plane issues tokens for, declares the Control Plane's catalog
// and nothing else.
func TestControlPlaneCatalogMatchesResourceServer(t *testing.T) {
	InitControlPlanePermissions("")

	delimiter, declared := declaredPermissions(t, controlPlaneResourceServerPath, rsControlPlane)
	assert.Equal(t, ":", delimiter)
	assert.ElementsMatch(t, GetSystemPermissions().Catalog, declared)
}

// TestSystemCatalogKeepsExistingPermissions verifies that the catalog holds every permission
// enforced before it existed, unchanged, and that it honors the configured prefix.
func TestSystemCatalogKeepsExistingPermissions(t *testing.T) {
	for _, prefix := range []string{"", "mgmt"} {
		InitControlPlanePermissions(prefix)
		p := GetSystemPermissions()
		for _, existing := range []string{
			p.OU, p.OUView, p.User, p.UserView, p.Group, p.GroupView, p.Agent, p.AgentView,
			p.UserType, p.UserTypeView, p.AgentType, p.AgentTypeView,
			p.NotificationTemplate, p.NotificationTemplateView,
		} {
			assert.Contains(t, p.Catalog, existing)
		}
		for _, perm := range p.Catalog {
			assert.True(t, strings.HasPrefix(perm, p.Root+":"), "%q is not under %q", perm, p.Root)
		}
	}
	InitControlPlanePermissions("")
	assert.Contains(t, GetSystemPermissions().Catalog, "system:user:view")
}

// TestRouteTableUsesCatalogPermissions verifies that every route rule requires a catalog
// permission, or is one of the listed root or self-service rules. A route mapped to root or opened
// to any authenticated caller has to be added to these lists on purpose.
func TestRouteTableUsesCatalogPermissions(t *testing.T) {
	InitControlPlanePermissions("")
	p := GetSystemPermissions()

	selfService := map[string]bool{
		"GET /users/me": true, "PUT /users/me": true, "GET /users/me/**": true, "PUT /users/me/**": true,
		"POST /users/me/update-credentials": true, "DELETE /users/me/agents/*": true,
		"POST /users/me/agents/suspend": true, "POST /users/me/agents/*/suspend": true,
		"POST /users/me/agents/*/unsuspend": true,
		"GET /register/passkey/**":          true, "POST /register/passkey/**": true,
		"* /mcp/**":       true,
		"GET /scim/v2/Me": true, "PUT /scim/v2/Me": true,
		"GET /scim/v2/Schemas": true, "GET /scim/v2/Schemas/**": true,
		"GET /scim/v2/ResourceTypes": true, "GET /scim/v2/ResourceTypes/**": true,
		"GET /capabilities":  true,
		"POST /flow/execute": true,
	}
	root := map[string]bool{
		"GET /variables": true, "POST /variables": true, "GET /variables/**": true,
		"PUT /variables/**": true, "DELETE /variables/**": true,
		"GET /secrets": true, "POST /secrets": true, "GET /secrets/**": true,
		"PUT /secrets/**": true, "DELETE /secrets/**": true,
		"POST /import": true, "POST /import/delete": true, "POST /import/managed": true,
		"GET /users/*/ous": true,
		// The administration flow permission is the root permission on the control plane.
		"POST /flow/execute": true,
		// A gateway's variable and secret stores hold credentials, as the server's own does.
		"GET /gateways/*/variables/**": true, "POST /gateways/*/variables/**": true,
		"PUT /gateways/*/variables/**": true, "DELETE /gateways/*/variables/**": true,
		"GET /gateways/*/secrets/**": true, "POST /gateways/*/secrets/**": true,
		"PUT /gateways/*/secrets/**": true, "DELETE /gateways/*/secrets/**": true,
	}

	for _, entry := range apiPermissionEntries {
		switch entry.permission {
		case "":
			assert.True(t, selfService[entry.pattern], "%q is open to any authenticated caller", entry.pattern)
		case p.Root:
			assert.True(t, root[entry.pattern], "%q requires root but is not listed as a root rule", entry.pattern)
		default:
			assert.Contains(t, p.Catalog, entry.permission, "%q requires a permission outside the catalog",
				entry.pattern)
		}
	}
}

// TestResourceRoutesUseResourcePermissions verifies that every route rule of a resource requires one
// of that resource's catalog permissions, so a fine-grained administrator is never sent to another
// resource's permission. Root rules are checked by TestRouteTableUsesCatalogPermissions.
func TestResourceRoutesUseResourcePermissions(t *testing.T) {
	InitControlPlanePermissions("")
	p := GetSystemPermissions()

	resources := []struct {
		resource ResourceType
		paths    []string
	}{
		{ResourceTypeOU, []string{" /organization-units"}},
		{ResourceTypeUser, []string{" /users", " /scim/v2/Users"}},
		{ResourceTypeGroup, []string{" /groups", " /scim/v2/Groups"}},
		{ResourceTypeAgent, []string{" /agents"}},
		{ResourceTypeUserType, []string{" /user-types"}},
		{ResourceTypeAgentType, []string{" /agent-types"}},
		{ResourceTypeNotificationTemplate, []string{" /notification-templates"}},
		{ResourceTypeApplication, []string{" /applications", " /cimd/"}},
		{ResourceTypeRole, []string{" /roles"}},
		{ResourceTypeResourceServer, []string{" /resource-servers"}},
		{ResourceTypeConnection, []string{" /connections"}},
		{ResourceTypeVerifiableCredential, []string{
			" /openid4vci/credential-configurations", " /openid4vp/presentation-definitions",
			" /openid4vp/trust-anchors",
		}},
		{ResourceTypeDesign, []string{" /design/"}},
		{ResourceTypeI18n, []string{" /i18n/"}},
		{ResourceTypeGateway, []string{" /gateways"}},
		{ResourceTypeServerConfig, []string{" /server-config"}},
		{ResourceTypeConfigurationVersion, []string{" /configuration-versions"}},
		{ResourceTypeAuthorizationDetailType, []string{" /authorization-detail-types"}},
		{ResourceTypeFlow, []string{" /flows"}},
	}

	for _, r := range resources {
		t.Run(string(r.resource), func(t *testing.T) {
			var allowed []string
			for _, perm := range p.Catalog {
				if strings.HasPrefix(perm, "system:"+string(r.resource)+":") {
					allowed = append(allowed, perm)
				}
			}
			require.NotEmpty(t, allowed)

			found := 0
			for _, entry := range apiPermissionEntries {
				matches := false
				for _, path := range r.paths {
					matches = matches || strings.Contains(entry.pattern, path)
				}
				if !matches || entry.permission == p.Root || entry.permission == "" {
					continue
				}
				found++
				assert.Contains(t, allowed, entry.permission, "%q must require a %s permission",
					entry.pattern, r.resource)
			}
			assert.NotZero(t, found)
		})
	}
}

// TestMCPToolPermissionsUseCatalogPermissions verifies that every mapped MCP tool requires a catalog
// permission, and that an unmapped tool requires root.
func TestMCPToolPermissionsUseCatalogPermissions(t *testing.T) {
	for _, prefix := range []string{"", "mgmt"} {
		InitControlPlanePermissions(prefix)
		p := GetSystemPermissions()
		for name, perm := range mcpToolPermissions {
			assert.Contains(t, p.Catalog, perm, "MCP tool %q requires a permission outside the catalog", name)
			assert.Equal(t, perm, ResolveMCPToolPermission(name))
		}
		assert.Equal(t, p.Root, ResolveMCPToolPermission("unmapped_tool"))
	}
	InitControlPlanePermissions("")
}

func (suite *SecurityContextTestSuite) TestPermissionCovers() {
	t := suite.T()
	tests := []struct {
		name      string
		held      string
		required  string
		delimiter string
		want      bool
	}{
		{"Exact", "reservations:create", "reservations:create", ":", true},
		{"ParentCoversChild", "reservations", "reservations:create", ":", true},
		{"AncestorCoversDescendant", "reservations", "reservations:online:create", ":", true},
		{"ChildDoesNotCoverParent", "reservations:create", "reservations", ":", false},
		{"SiblingDoesNotCover", "reservations:view", "reservations:create", ":", false},
		{"PrefixWithoutDelimiterBoundary", "reserve", "reservations:create", ":", false},
		{"NonColonDelimiter", "reservations", "reservations/create", "/", true},
		{"WrongDelimiterDoesNotCover", "reservations", "reservations/create", ":", false},
		{"EmptyDelimiterFallsBackToEquality", "reservations", "reservations:create", "", false},
		{"EmptyDelimiterStillMatchesExactly", "reservations", "reservations", "", true},
		{"EmptyHeld", "", "reservations", ":", false},
		{"EmptyRequired", "reservations", "", ":", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, PermissionCovers(tt.held, tt.required, tt.delimiter))
		})
	}
}

func (suite *SecurityContextTestSuite) TestAnyPermissionCovers() {
	t := suite.T()
	held := []string{"invoices:read", "reservations"}

	assert.True(t, AnyPermissionCovers(held, "reservations:create", ":"))
	assert.True(t, AnyPermissionCovers(held, "invoices:read", ":"))
	assert.False(t, AnyPermissionCovers(held, "invoices:write", ":"))
	assert.False(t, AnyPermissionCovers(nil, "reservations", ":"))
}

func (suite *SecurityContextTestSuite) TestAncestorPermissions() {
	t := suite.T()
	tests := []struct {
		name       string
		permission string
		delimiter  string
		want       []string
	}{
		{
			name:       "NearestAncestorFirst",
			permission: "reservations:online:create",
			delimiter:  ":",
			want:       []string{"reservations:online", "reservations"},
		},
		{
			name:       "SingleLevelHasOneAncestor",
			permission: "reservations:create",
			delimiter:  ":",
			want:       []string{"reservations"},
		},
		{"TopLevelHasNone", "reservations", ":", nil},
		{"NonColonDelimiter", "a/b", "/", []string{"a"}},
		{"EmptyDelimiter", "a:b", "", nil},
		{"EmptyPermission", "", ":", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AncestorPermissions(tt.permission, tt.delimiter))
		})
	}
}

// Whether one permission confers another is a property of the resource server, so the coverage
// check has to be told. A resource server it was not told about is matched exactly, which refuses a
// grant it cannot justify rather than approving one.
func (suite *SecurityContextTestSuite) TestCoversMatchesExactlyWithoutADelimiter() {
	t := suite.T()
	const rs = "booking-api"
	actor := PermissionSet{rs: {"reservations"}}

	assert.False(t, Covers(actor, PermissionSet{rs: {"reservations:create"}}, nil),
		"no delimiter means no nesting, so the resource confers only itself")
	assert.False(t, Covers(actor, PermissionSet{rs: {"reservations:create"}}, map[string]string{}),
		"a resource server absent from the map is matched exactly")
	assert.True(t, Covers(actor, PermissionSet{rs: {"reservations"}}, nil),
		"the permission still covers itself")
	assert.True(t, Covers(actor, PermissionSet{rs: {"reservations:create"}}, map[string]string{rs: ":"}),
		"told the delimiter, the resource confers what sits beneath it")
}

// Matching on a character the strings were not built with can read two unrelated permissions as
// parent and child, which would approve a grant the resource server's tree does not support.
func (suite *SecurityContextTestSuite) TestCoversUsesEachResourceServersOwnDelimiter() {
	t := suite.T()
	const dashRS = "dash-api"

	// A real child on a "-" delimited server: only the right delimiter sees it.
	assert.True(t, Covers(
		PermissionSet{dashRS: {"reservations"}},
		PermissionSet{dashRS: {"reservations-create"}},
		map[string]string{dashRS: "-"},
	))
	assert.False(t, Covers(
		PermissionSet{dashRS: {"reservations"}},
		PermissionSet{dashRS: {"reservations-create"}},
		map[string]string{dashRS: ":"},
	))

	// Two flat handles on a "-" delimited server are unrelated, whatever a colon would suggest.
	assert.False(t, Covers(
		PermissionSet{dashRS: {"https"}},
		PermissionSet{dashRS: {"https://www.googleapis.com/auth/drive"}},
		map[string]string{dashRS: "-"},
	))

	// Each resource server is resolved independently within one grant.
	assert.False(t, Covers(
		PermissionSet{dashRS: {"reservations"}, "other": {"a"}},
		PermissionSet{dashRS: {"reservations-create"}, "other": {"a:b"}},
		map[string]string{dashRS: "-"},
	), "the second resource server has no delimiter, so a:b is not conferred")
}
