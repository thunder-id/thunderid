/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package security

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	svc, err := newSecurityService(nil, nil, []string{}, apiPermissionEntries, "")
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
