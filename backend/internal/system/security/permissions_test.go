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
	InitSystemPermissions("")

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

// ---------------------------------------------------------------------------
// HasSufficientPermission
// ---------------------------------------------------------------------------

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
	InitSystemPermissions("")
	p := GetSystemPermissions()

	tests := []struct {
		name     string
		action   Action
		wantPerm string
	}{
		// OU actions.
		{name: "CreateOU", action: ActionCreateOU, wantPerm: p.OU},
		{name: "ReadOU", action: ActionReadOU, wantPerm: p.OUView},
		{name: "UpdateOU", action: ActionUpdateOU, wantPerm: p.OU},
		{name: "DeleteOU", action: ActionDeleteOU, wantPerm: p.OU},
		{name: "ListOUs", action: ActionListOUs, wantPerm: p.OUView},

		// User actions.
		{name: "CreateUser", action: ActionCreateUser, wantPerm: p.User},
		{name: "ReadUser", action: ActionReadUser, wantPerm: p.UserView},
		{name: "UpdateUser", action: ActionUpdateUser, wantPerm: p.User},
		{name: "DeleteUser", action: ActionDeleteUser, wantPerm: p.User},
		{name: "ListUsers", action: ActionListUsers, wantPerm: p.UserView},

		// Group actions.
		{name: "CreateGroup", action: ActionCreateGroup, wantPerm: p.Group},
		{name: "ReadGroup", action: ActionReadGroup, wantPerm: p.GroupView},
		{name: "UpdateGroup", action: ActionUpdateGroup, wantPerm: p.Group},
		{name: "DeleteGroup", action: ActionDeleteGroup, wantPerm: p.Group},
		{name: "ListGroups", action: ActionListGroups, wantPerm: p.GroupView},

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
	InitSystemPermissions("")
	for action, expectedPerm := range actionPermissionMap {
		s.Run(string(action), func() {
			s.Equal(expectedPerm, ResolveActionPermission(action))
		})
	}
}

// ---------------------------------------------------------------------------
// InitSystemPermissions
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// GetSystemRootPermission
// ---------------------------------------------------------------------------

func (s *SecurityContextTestSuite) TestGetSystemRootPermission_AfterInitEmptyHandle() {
	InitSystemPermissions("")
	s.Equal("system", GetSystemRootPermission())
}

func (s *SecurityContextTestSuite) TestGetSystemRootPermission_AfterInitNonEmptyHandle() {
	InitSystemPermissions("mgmt")
	defer InitSystemPermissions("")
	s.Equal("mgmt:system", GetSystemRootPermission())
}

// ---------------------------------------------------------------------------

func TestInitSystemPermissions_EmptyHandle(t *testing.T) {
	InitSystemPermissions("")
	p := GetSystemPermissions()
	require.NotNil(t, p)

	assert.Equal(t, "system", p.Root)
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

func TestInitSystemPermissions_NonEmptyHandle(t *testing.T) {
	InitSystemPermissions("mgmt")
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
	InitSystemPermissions("")
}

func TestInitSystemPermissions_RebuildsActionMap(t *testing.T) {
	InitSystemPermissions("x")
	assert.Equal(t, "x:system:ou", ResolveActionPermission(ActionCreateOU))

	InitSystemPermissions("")
	assert.Equal(t, "system:ou", ResolveActionPermission(ActionCreateOU))
}

func TestHasSystemPermission_WithCustomHandle(t *testing.T) {
	InitSystemPermissions("mgmt")
	defer InitSystemPermissions("")

	assert.True(t, HasSystemPermission([]string{"mgmt:system"}))
	assert.False(t, HasSystemPermission([]string{"system"}))
}

// ---------------------------------------------------------------------------
// GetRequiredPermissionForAPI
// ---------------------------------------------------------------------------

func TestGetRequiredPermissionForAPI(t *testing.T) {
	InitSystemPermissions("")
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
			method: http.MethodPost, path: "/organization-units", wantPerm: p.OU,
		},
		{name: "GET /users exact", method: http.MethodGet, path: "/users", wantPerm: p.UserView},
		{name: "POST /users exact", method: http.MethodPost, path: "/users", wantPerm: p.User},
		{name: "GET /groups exact", method: http.MethodGet, path: "/groups", wantPerm: p.GroupView},
		{name: "POST /groups exact", method: http.MethodPost, path: "/groups", wantPerm: p.Group},

		// ---- Notification template APIs ----
		{
			name:   "GET notification-templates list",
			method: http.MethodGet, path: "/notification-templates/email/templates",
			wantPerm: p.NotificationTemplateView,
		},
		{
			name:   "POST notification-templates create",
			method: http.MethodPost, path: "/notification-templates/sms/templates",
			wantPerm: p.NotificationTemplate,
		},
		{
			name:   "GET notification-template by id",
			method: http.MethodGet, path: "/notification-templates/email/templates/tmpl-1",
			wantPerm: p.NotificationTemplateView,
		},
		{
			name:   "PUT notification-template by id",
			method: http.MethodPut, path: "/notification-templates/email/templates/tmpl-1",
			wantPerm: p.NotificationTemplate,
		},
		{
			name:   "DELETE notification-template by id",
			method: http.MethodDelete, path: "/notification-templates/sms/templates/tmpl-2",
			wantPerm: p.NotificationTemplate,
		},

		// ---- Self-service paths (empty permission = any authenticated user) ----
		{name: "GET /users/me self-service", method: http.MethodGet, path: "/users/me", wantPerm: ""},
		{name: "PUT /users/me self-service", method: http.MethodPut, path: "/users/me", wantPerm: ""},
		{
			name:     "POST /users/me/update-credentials self-service",
			method:   http.MethodPost,
			path:     "/users/me/update-credentials",
			wantPerm: "",
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
			method: http.MethodPut, path: "/organization-units/ou-123", wantPerm: p.OU,
		},
		{
			name:   "DELETE /organization-units/{id} prefix",
			method: http.MethodDelete, path: "/organization-units/ou-123", wantPerm: p.OU,
		},
		{
			name:   "GET /users/{id} prefix",
			method: http.MethodGet, path: "/users/user-456", wantPerm: p.UserView,
		},
		{
			name:   "PUT /users/{id} prefix",
			method: http.MethodPut, path: "/users/user-456", wantPerm: p.User,
		},
		{
			name:   "DELETE /users/{id} prefix",
			method: http.MethodDelete, path: "/users/user-789", wantPerm: p.User,
		},
		{
			name:   "GET /groups/{id} prefix",
			method: http.MethodGet, path: "/groups/grp-111", wantPerm: p.GroupView,
		},
		{
			name:   "DELETE /groups/{id} prefix",
			method: http.MethodDelete, path: "/groups/grp-222", wantPerm: p.Group,
		},

		// ---- Self-service wins over parent prefix ----
		{name: "GET /users/me wins over /users/ prefix", method: http.MethodGet, path: "/users/me", wantPerm: ""},
		{
			name:   "GET /users/me/profile wins over /users/ prefix",
			method: http.MethodGet, path: "/users/me/profile", wantPerm: "",
		},

		// ---- Session management paths ----
		{name: "GET /sessions falls back to system", method: http.MethodGet, path: "/sessions", wantPerm: p.Root},
		{name: "GET /users/me/sessions self-service", method: http.MethodGet, path: "/users/me/sessions", wantPerm: ""},

		// ---- OU tree paths ----
		{
			name:   "GET /organization-units/tree",
			method: http.MethodGet, path: "/organization-units/tree", wantPerm: p.OUView,
		},
		{
			name:   "PUT /organization-units/tree",
			method: http.MethodPut, path: "/organization-units/tree", wantPerm: p.OU,
		},
		{
			name:   "DELETE /organization-units/tree",
			method: http.MethodDelete, path: "/organization-units/tree", wantPerm: p.OU,
		},

		// ---- Unmapped paths fall back to Root ----
		{
			name:   "Unmapped path falls back to system",
			method: http.MethodGet, path: "/applications", wantPerm: p.Root,
		},
		{name: "Root path falls back to system", method: http.MethodGet, path: "/", wantPerm: p.Root},
		{
			name:   "Resource servers require system",
			method: http.MethodPost, path: "/resource-servers", wantPerm: p.Root,
		},
		{
			name:   "Authorization detail types require system, as resource servers do",
			method: http.MethodGet, path: "/authorization-detail-types", wantPerm: p.Root,
		},
		{
			name:   "Authorization detail type updates require system",
			method: http.MethodPut, path: "/authorization-detail-types/adt-1", wantPerm: p.Root,
		},
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
	InitSystemPermissions("")

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
			assert.Equal(t, tt.want, Covers(tt.actor, tt.required))
		})
	}
}

// TestCoversIsNotSymmetric documents that coverage is a partial order, not an equivalence:
// incomparable sets cover each other in neither direction.
func TestCoversIsNotSymmetric(t *testing.T) {
	InitSystemPermissions("")

	groupAdmin := PermissionSet{rsSystem: {"system:group"}}
	userAdmin := PermissionSet{rsSystem: {"system:user"}}

	assert.False(t, Covers(groupAdmin, userAdmin), "group admin must not cover user admin")
	assert.False(t, Covers(userAdmin, groupAdmin), "user admin must not cover group admin")
}

// TestCoversHonoursConfiguredPermissionPrefix verifies that coverage is computed from the
// permission strings themselves, so a non-empty SystemPermissionPrefix needs no special handling.
func TestCoversHonoursConfiguredPermissionPrefix(t *testing.T) {
	InitSystemPermissions("mgmt")
	t.Cleanup(func() { InitSystemPermissions("") })

	p := GetSystemPermissions()
	require.Equal(t, "mgmt:system", p.Root)

	assert.True(t, Covers(
		PermissionSet{rsSystem: {p.Root}},
		PermissionSet{rsSystem: {p.User, p.GroupView}},
	))
	assert.False(t, Covers(
		PermissionSet{rsSystem: {p.UserView}},
		PermissionSet{rsSystem: {p.User}},
	))
	// An unprefixed permission must not satisfy a prefixed requirement.
	assert.False(t, Covers(
		PermissionSet{rsSystem: {"system"}},
		PermissionSet{rsSystem: {p.User}},
	))
}

// ---------------------------------------------------------------------------
// Permission catalog
// ---------------------------------------------------------------------------

const (
	bootstrapBundlePath    = "../../../cmd/server/bootstrap/01-default-resources.yaml"
	systemResourceServerID = "01900000-0000-7000-8000-000000000020"
)

// bootstrapSystemPermissions returns the delimiter of the System resource server in the bootstrap
// bundle and every permission declared under its root resource.
func bootstrapSystemPermissions(t *testing.T) (string, []string) {
	t.Helper()
	content, err := os.ReadFile(bootstrapBundlePath)
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
		if !strings.Contains(doc, "id: "+systemResourceServerID) {
			continue
		}
		require.NoError(t, yaml.Unmarshal([]byte(doc), &server))
	}
	require.Equal(t, systemResourceServerID, server.ID, "System resource server not found in the bootstrap bundle")
	require.NotEmpty(t, server.Resources)
	require.Equal(t, resource{Name: "System", Handle: "system"}, server.Resources[0])

	var permissions []string
	for _, res := range server.Resources[1:] {
		require.Equal(t, "system", res.Parent, "resource %q must sit directly under the root", res.Handle)
		require.NotEmpty(t, res.Name, "resource %q needs a display name", res.Handle)
		path := res.Parent + server.Delimiter + res.Handle
		permissions = append(permissions, path)
		for _, a := range res.Actions {
			permissions = append(permissions, path+server.Delimiter+a.Handle)
		}
	}
	return server.Delimiter, permissions
}

// TestSystemCatalogMatchesBootstrapBundle verifies that the Go catalog and the System resource
// server declared in the bootstrap bundle hold the same permissions.
func TestSystemCatalogMatchesBootstrapBundle(t *testing.T) {
	InitSystemPermissions("")

	delimiter, declared := bootstrapSystemPermissions(t)
	assert.Equal(t, ":", delimiter)
	assert.ElementsMatch(t, GetSystemPermissions().Catalog, declared)
}

// TestSystemCatalogKeepsExistingPermissions verifies that the catalog holds every permission
// enforced before it existed, unchanged, and that it honors the configured prefix.
func TestSystemCatalogKeepsExistingPermissions(t *testing.T) {
	for _, prefix := range []string{"", "mgmt"} {
		InitSystemPermissions(prefix)
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
	InitSystemPermissions("")
	assert.Contains(t, GetSystemPermissions().Catalog, "system:user:view")
}

// TestRouteTableUsesCatalogPermissions verifies that every route rule requires a catalog
// permission, or is one of the listed root or self-service rules. A route mapped to root or opened
// to any authenticated caller has to be added to these lists on purpose.
func TestRouteTableUsesCatalogPermissions(t *testing.T) {
	InitSystemPermissions("")
	p := GetSystemPermissions()

	selfService := map[string]bool{
		"GET /users/me": true, "PUT /users/me": true, "GET /users/me/**": true, "PUT /users/me/**": true,
		"POST /users/me/update-credentials": true,
		"GET /register/passkey/**":          true, "POST /register/passkey/**": true,
		"GET /scim/v2/Me": true, "PUT /scim/v2/Me": true,
		"GET /scim/v2/Schemas": true, "GET /scim/v2/Schemas/**": true,
		"GET /scim/v2/ResourceTypes": true, "GET /scim/v2/ResourceTypes/**": true,
	}
	root := map[string]bool{
		// Projects keep root until the project permissions are enabled with their confinement.
		"GET /projects": true, "POST /projects": true, "GET /projects/**": true,
		"PUT /projects/**": true, "DELETE /projects/**": true,
		"GET /variables": true, "POST /variables": true, "GET /variables/**": true,
		"PUT /variables/**": true, "DELETE /variables/**": true,
		"GET /secrets": true, "POST /secrets": true, "GET /secrets/**": true,
		"PUT /secrets/**": true, "DELETE /secrets/**": true,
		"POST /import": true, "POST /import/delete": true,
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
