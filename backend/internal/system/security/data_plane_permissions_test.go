// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
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

// The onboarding tools require the permission of the action their flow performs, as the Console does.
func (s *DataPlanePermissionsTestSuite) TestOnboardingToolsRequireTheCreatePermission() {
	s.Equal("org:agent:create", ResolveMCPToolPermission("onboard_agents"))
	s.Equal("org:user:create", ResolveMCPToolPermission("onboard_users"))
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
		{http.MethodPost, "/flow/execute", ""},
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

// /flow/execute requires no permission, so every token reaches the flow engine as its caller and the
// flow decides what each flow type needs.
func (s *DataPlanePermissionsTestSuite) TestFlowExecuteKeepsTheCaller() {
	for permission, wantRuntime := range map[string]bool{"system": false, "org": false, "org:user": false} {
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

// On the data plane a caller without the root system permission acts in the organization unit its token names, so
// a token naming none is refused before any service runs. The root system permission needs no unit, and a route
// any authenticated caller may use asks for none.
func (s *DataPlanePermissionsTestSuite) TestATokenNamingNoUnitNeedsTheSystemPermission() {
	tests := []struct {
		name        string
		ouID        string
		permissions []string
		path        string
		wantErr     error
	}{
		{name: "org without a unit", permissions: []string{"org"}, path: "/users", wantErr: errForbidden},
		{name: "org child without a unit", permissions: []string{"org:user:view"}, path: "/users",
			wantErr: errForbidden},
		{name: "org root for another resource", permissions: []string{"org"}, path: "/applications",
			wantErr: errForbidden},
		{name: "org with a unit", ouID: "ou-1", permissions: []string{"org"}, path: "/users"},
		{name: "org child with a unit", ouID: "ou-1", permissions: []string{"org:user:view"}, path: "/users"},
		{name: "system without a unit", permissions: []string{"system"}, path: "/users"},
		{name: "self service without a unit", permissions: []string{"org"}, path: "/users/me"},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.Equal(tc.wantErr, s.process(tc.ouID, tc.permissions, tc.path))
		})
	}
}

// A public path also serves callers without a token, so a token naming no unit keeps its caller there, and the
// service behind the path decides what it may do.
func (s *DataPlanePermissionsTestSuite) TestAPublicPathKeepsACallerNamingNoUnit() {
	auth := &AuthenticatorInterfaceMock{}
	revocation := &RevocationEnforcerInterfaceMock{}
	revocation.On("EnsureNotRevoked", mock.Anything, mock.Anything).Return(nil)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", nil)
	auth.On("CanHandle", req).Return(true)
	auth.On("Authenticate", req).Return(newSecurityContext("user-1", "", "token", []string{"org"}, nil), nil)
	svc, err := newSecurityService([]AuthenticatorInterface{auth}, revocation, publicPaths, apiPermissionEntries)
	s.Require().NoError(err)

	ctx, err := svc.Process(req)
	s.Require().NoError(err)
	s.False(IsRuntimeContext(ctx), "the caller is kept rather than dropped to the runtime")
	s.Equal("user-1", GetSubject(ctx))
}

// The control plane bounds no permission to an organization unit, so a token needs none.
func (s *DataPlanePermissionsTestSuite) TestAControlPlaneTokenNeedsNoUnit() {
	InitControlPlanePermissions("")
	s.Nil(s.process("", []string{"system:user:view"}, "/users"))
	s.Nil(s.process("", []string{"system"}, "/users"))
	s.False(LacksOrganizationUnit(WithSecurityContextTest(context.Background(),
		NewSecurityContextForTest("user-1", "", "", []string{"system:user"}, nil))))
}

// Only an organization permission is confined to the token's unit on the data plane; the root system permission
// reaches the whole deployment, and the control plane confines nothing.
func (s *DataPlanePermissionsTestSuite) TestIsConfinedToOrganizationUnit() {
	callerWith := func(permissions ...string) context.Context {
		return WithSecurityContextTest(context.Background(),
			NewSecurityContextForTest("user-1", "ou-1", "", permissions, nil))
	}
	s.True(IsConfinedToOrganizationUnit(callerWith("org")))
	s.True(IsConfinedToOrganizationUnit(callerWith("org:user:create")))
	s.False(IsConfinedToOrganizationUnit(callerWith("system")))
	s.False(IsConfinedToOrganizationUnit(callerWith("system", "org")))

	InitControlPlanePermissions("")
	s.False(IsConfinedToOrganizationUnit(callerWith("system:user:create")))
}

// process authenticates a GET of path as a caller holding permissions in ouID, and returns the refusal, if any.
func (s *DataPlanePermissionsTestSuite) process(ouID string, permissions []string, path string) error {
	auth := &AuthenticatorInterfaceMock{}
	revocation := &RevocationEnforcerInterfaceMock{}
	revocation.On("EnsureNotRevoked", mock.Anything, mock.Anything).Return(nil)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	auth.On("CanHandle", req).Return(true)
	auth.On("Authenticate", req).Return(newSecurityContext("user-1", ouID, "token", permissions, nil), nil)
	svc, err := newSecurityService([]AuthenticatorInterface{auth}, revocation, publicPaths, apiPermissionEntries)
	s.Require().NoError(err)
	_, err = svc.Process(req)
	return err
}

// An organization permission reaches only the unit its token names, so only the system root permission lists the
// root organization units on the data plane. The control plane keeps the view permission of organization units.
func (s *DataPlanePermissionsTestSuite) TestListingTheRootOrganizationUnitsNeedsSystem() {
	svc, err := newSecurityService(nil, nil, []string{}, apiPermissionEntries)
	s.Require().NoError(err)
	s.Equal("system", GetSystemPermissions().OUListRoots)
	s.Equal("system", svc.getRequiredPermissionForAPI(http.MethodGet, "/organization-units"))
	s.Equal("system", ResolveActionPermission(ActionListOUs))
	s.Equal("system", ResolveMCPToolPermission("list_organization_units"))
	s.Equal("org:ou:view", svc.getRequiredPermissionForAPI(http.MethodGet, "/organization-units/ou-1/ous"))
	s.Equal("org:ou:view", ResolveActionPermission(ActionListChildOUs))

	InitControlPlanePermissions("")
	svc, err = newSecurityService(nil, nil, []string{}, apiPermissionEntries)
	s.Require().NoError(err)
	s.Equal("system:ou:view", svc.getRequiredPermissionForAPI(http.MethodGet, "/organization-units"))
	s.Equal("system:ou:view", ResolveActionPermission(ActionListOUs))
}
