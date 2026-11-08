// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type DataPlanePermissionsTestSuite struct {
	suite.Suite
}

func TestDataPlanePermissionsTestSuite(t *testing.T) {
	suite.Run(t, new(DataPlanePermissionsTestSuite))
}

func (s *DataPlanePermissionsTestSuite) SetupTest() {
	InitDataPlanePermissions("")
}

func (s *DataPlanePermissionsTestSuite) TearDownSuite() {
	InitControlPlanePermissions("")
}

func (s *DataPlanePermissionsTestSuite) TestRootsAndAdministrationFlow() {
	p := GetSystemPermissions()
	s.Equal("system", p.Root)
	s.Equal("org", p.Org)
	s.Equal("org", p.AdministrationFlow)
}

// execute_flow starts an administration flow by ID, so it requires what the flow service requires.
func (s *DataPlanePermissionsTestSuite) TestExecuteFlowToolRequiresTheAdministrationFlowPermission() {
	s.Equal("org", ResolveMCPToolPermission("execute_flow"))
}

func (s *DataPlanePermissionsTestSuite) TestOrgChildrenForOUConfinedResources() {
	p := GetSystemPermissions()
	s.Equal("org:ou", p.OU)
	s.Equal("org:ou:view", p.OUView)
	s.Equal("org:ou:create", p.OUCreate)
	s.Equal("org:user", p.User)
	s.Equal("org:user:create", p.UserCreate)
	s.Equal("org:group:update", p.GroupUpdate)
	s.Equal("org:agent:view", p.AgentView)
	s.Equal("org:usertype:delete", p.UserTypeDelete)
	s.Equal("org:agenttype:delete", p.AgentTypeDelete)
}

func (s *DataPlanePermissionsTestSuite) TestOtherOUOwnedResourcesRequireOrgRoot() {
	p := GetSystemPermissions()
	for _, perm := range []string{
		p.AppView, p.AppCreate, p.AppUpdate, p.AppDelete,
		p.RoleView, p.RoleCreate, p.RoleUpdate, p.RoleDelete,
		p.ResourceServerView, p.ResourceServerCreate, p.ResourceServerUpdate, p.ResourceServerDelete,
		p.ConnectionView, p.ConnectionCreate, p.ConnectionUpdate, p.ConnectionDelete,
		p.FlowView, p.FlowCreate, p.FlowUpdate, p.FlowDelete,
		p.NotificationTemplate, p.NotificationTemplateView, p.NotificationTemplateCreate,
		p.NotificationTemplateUpdate, p.NotificationTemplateDelete,
		p.VCView, p.VCCreate, p.VCUpdate, p.VCDelete,
	} {
		s.Equal("org", perm)
	}
}

// Only the Control Plane serves projects, so the Data Plane declares no project permission, and every
// project action, route and tool falls back to the system root.
func (s *DataPlanePermissionsTestSuite) TestDeclaresNoProjectPermission() {
	p := GetSystemPermissions()
	for _, perm := range []string{p.ProjectView, p.ProjectCreate, p.ProjectUpdate, p.ProjectDelete} {
		s.Empty(perm)
	}

	for _, action := range []Action{ActionCreateProject, ActionReadProject, ActionUpdateProject,
		ActionDeleteProject, ActionListProjects} {
		s.Equal("system", ResolveActionPermission(action), action)
	}
	for _, tool := range []string{"list_projects", "get_project_by_id", "create_project", "update_project"} {
		s.Equal("system", ResolveMCPToolPermission(tool), tool)
	}

	svc, err := newSecurityService(nil, nil, []string{}, apiPermissionEntries)
	s.Require().NoError(err)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		s.Equal("system", svc.getRequiredPermissionForAPI(method, "/projects"), method)
	}
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		s.Equal("system", svc.getRequiredPermissionForAPI(method, "/projects/p-1"), method)
	}
}

func (s *DataPlanePermissionsTestSuite) TestDeploymentWideReadsRequireOrgAndWritesRequireSystem() {
	p := GetSystemPermissions()
	for _, perm := range []string{p.DesignView, p.I18nView, p.AuthzDetailView} {
		s.Equal("org", perm)
	}
	for _, perm := range []string{
		p.DesignCreate, p.DesignUpdate, p.DesignDelete,
		p.I18nCreate, p.I18nUpdate, p.I18nDelete,
		p.AuthzDetailCreate, p.AuthzDetailUpdate, p.AuthzDetailDelete,
	} {
		s.Equal("system", perm)
	}
}

func (s *DataPlanePermissionsTestSuite) TestFullyDeploymentWideResourcesRequireSystem() {
	p := GetSystemPermissions()
	for _, perm := range []string{
		p.ServerConfigView, p.ServerConfigUpdate,
		p.GatewayView, p.GatewayCreate, p.GatewayUpdate, p.GatewayDelete,
		p.ConfigVersionView, p.ConfigVersionCreate,
	} {
		s.Equal("system", perm)
	}
}

func (s *DataPlanePermissionsTestSuite) TestPrefixIsHonored() {
	InitDataPlanePermissions("mgmt")
	p := GetSystemPermissions()
	s.Equal("mgmt:system", p.Root)
	s.Equal("mgmt:org", p.Org)
	s.Equal("mgmt:org:user:view", p.UserView)
	s.Equal("mgmt:org", p.AppView)
	s.Equal("mgmt:system", p.DesignCreate)
	s.Equal("mgmt:org:ou:create", ResolveActionPermission(ActionCreateOU))
}

func (s *DataPlanePermissionsTestSuite) TestCatalogIsTheOrgTree() {
	p := GetSystemPermissions()
	s.Equal("org", p.Catalog[0])
	s.Equal([]string{
		"org",
		"org:ou", "org:ou:view", "org:ou:create", "org:ou:update", "org:ou:delete",
		"org:user", "org:user:view", "org:user:create", "org:user:update", "org:user:delete",
		"org:group", "org:group:view", "org:group:create", "org:group:update", "org:group:delete",
		"org:agent", "org:agent:view", "org:agent:create", "org:agent:update", "org:agent:delete",
		"org:usertype", "org:usertype:view", "org:usertype:create", "org:usertype:update",
		"org:usertype:delete",
		"org:agenttype", "org:agenttype:view", "org:agenttype:create", "org:agenttype:update",
		"org:agenttype:delete",
	}, p.Catalog)
}

func (s *DataPlanePermissionsTestSuite) TestRouteTable() {
	svc, err := newSecurityService(nil, nil, []string{}, apiPermissionEntries)
	s.Require().NoError(err)

	tests := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/users", "org:user:view"},
		{http.MethodPost, "/organization-units", "org:ou:create"},
		{http.MethodGet, "/users/user-1/ous", "system"},
		{http.MethodPost, "/applications", "org"},
		{http.MethodPut, "/roles/role-1", "org"},
		{http.MethodGet, "/design/themes", "org"},
		{http.MethodPut, "/design/themes/theme-1", "system"},
		{http.MethodPost, "/i18n/languages/en-US/translations", "system"},
		{http.MethodGet, "/server-config/cors", "system"},
		{http.MethodGet, "/gateways", "system"},
		{http.MethodPost, "/import", "system"},
		{http.MethodGet, "/variables", "system"},
		{http.MethodPost, "/flow/execute", "org"},
		{http.MethodGet, "/not-a-route", "system"},
	}
	for _, tc := range tests {
		s.Equal(tc.want, svc.getRequiredPermissionForAPI(tc.method, tc.path), "%s %s", tc.method, tc.path)
	}
}

// TestNoControlPlanePermissionIsEnforced verifies that every route, action and MCP tool on the data
// plane requires the system root, the org root or an org catalog permission, never a system child.
func (s *DataPlanePermissionsTestSuite) TestNoControlPlanePermissionIsEnforced() {
	p := GetSystemPermissions()
	allowed := func(perm string) bool {
		return perm == "" || perm == p.Root || slices.Contains(p.Catalog, perm)
	}
	for _, entry := range apiPermissionEntries {
		s.True(allowed(entry.permission), "route %q requires %q", entry.pattern, entry.permission)
	}
	for action, perm := range actionPermissionMap {
		s.True(allowed(perm), "action %q requires %q", action, perm)
	}
	for tool, perm := range mcpToolPermissions {
		s.True(allowed(perm), "MCP tool %q requires %q", tool, perm)
	}
}

func (s *DataPlanePermissionsTestSuite) TestHasSufficientSystemPermission() {
	s.True(HasSufficientSystemPermission([]string{"system"}, "org:user:view"))
	s.True(HasSufficientSystemPermission([]string{"system"}, "org"))
	s.True(HasSufficientSystemPermission([]string{"org"}, "org:ou:update"))
	s.False(HasSufficientSystemPermission([]string{"org"}, "system"))
	// Stale strings from before the split reach nothing on the data plane.
	s.False(HasSufficientSystemPermission([]string{"system:user"}, "org:user:view"))
	s.False(HasSufficientSystemPermission([]string{"system:user:view"}, "system"))
	s.False(HasSufficientSystemPermission(nil, "org"))
	s.True(HasSufficientSystemPermission(nil, ""))
}

func (s *DataPlanePermissionsTestSuite) TestHasAnySystemPermissionIncludesOrg() {
	s.True(HasAnySystemPermission([]string{"org"}))
	s.True(HasAnySystemPermission([]string{"org:user:view"}))
	s.True(HasAnySystemPermission([]string{"system"}))
	s.False(HasAnySystemPermission([]string{"organization"}))
	s.False(HasAnySystemPermission([]string{"other"}))
}

func (s *DataPlanePermissionsTestSuite) TestAuthorizeLetsSystemPassOrgRoutes() {
	service, err := newSecurityService(nil, nil, nil, apiPermissionEntries)
	s.Require().NoError(err)

	cases := []struct {
		name        string
		permissions []string
		wantErr     error
	}{
		{"system", []string{"system"}, nil},
		{"org", []string{"org"}, nil},
		{"org user view", []string{"org:user:view"}, nil},
		{"stale system user view", []string{"system:user:view"}, errInsufficientPermissions},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			ctx := withSecurityContext(req.Context(),
				newSecurityContext("user123", "ou456", "token", tc.permissions, nil))
			err := service.authorize(req.WithContext(ctx))
			if tc.wantErr == nil {
				s.NoError(err)
			} else {
				s.ErrorIs(err, tc.wantErr)
			}
		})
	}
}

func (s *DataPlanePermissionsTestSuite) TestForEachPermissionPlaneForTest() {
	var roots []string
	ForEachPermissionPlaneForTest(func(plane string) {
		roots = append(roots, plane+"="+GetSystemPermissions().Org)
	})
	s.Equal([]string{"ControlPlane=", "DataPlane=org"}, roots)
	s.Empty(GetSystemPermissions().Org, "the control plane's set is restored")
}

// /flow/execute is public, so a token that fails the route's permission is handed on as a runtime caller.
// The route requires the administration flow permission, so an org holder reaches the flow as itself and
// the flow's own checks decide.
func (s *DataPlanePermissionsTestSuite) TestFlowExecuteKeepsTheAdministrationCaller() {
	for permission, wantRuntime := range map[string]bool{"system": false, "org": false, "org:user": true} {
		s.Run(permission, func() {
			req := httptest.NewRequest(http.MethodPost, "/flow/execute", nil)
			auth := NewAuthenticatorInterfaceMock(s.T())
			auth.On("CanHandle", req).Return(true)
			auth.On("Authenticate", req).Return(
				newSecurityContext("admin", "ou-1", "token", []string{permission}, nil), nil)
			revocation := NewRevocationEnforcerInterfaceMock(s.T())
			revocation.On("EnsureNotRevoked", mock.Anything, mock.Anything).Return(nil)
			svc, err := newSecurityService([]AuthenticatorInterface{auth}, revocation,
				[]string{"/flow/execute/**"}, apiPermissionEntries)
			s.Require().NoError(err)

			ctx, err := svc.Process(req)

			s.Require().NoError(err)
			s.Equal(wantRuntime, IsRuntimeContext(ctx))
			if !wantRuntime {
				s.Equal("admin", GetSubject(ctx))
			}
		})
	}
}
