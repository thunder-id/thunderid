// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const (
	escRSSystem  = "01900000-0000-7000-8000-000000000020"
	escRSPayroll = "01900000-0000-7000-8000-0000000000aa"
)

// resolverSetup says what a PermissionResolverMock answers. entities and roles answer per ID where a test needs it,
// and delimiters left nil reports every resource server named as ":"-delimited, as the system one is.
type resolverSetup struct {
	entityPerms security.PermissionSet
	entityErr   error
	entities    map[string]security.PermissionSet

	groupPerms security.PermissionSet
	groupErr   error

	rolePerms security.PermissionSet
	roleErr   error
	roles     map[string]security.PermissionSet

	delimiters map[string]string
}

// newResolver returns a PermissionResolverMock answering as setup says, so tests can count the lookups it took.
func newResolver(t *testing.T, setup resolverSetup) *PermissionResolverMock {
	resolver := NewPermissionResolverMock(t)
	entity := func(_ context.Context, entityID string) (security.PermissionSet, error) {
		if setup.entityErr != nil {
			return nil, setup.entityErr
		}
		if setup.entities != nil {
			return setup.entities[entityID], nil
		}
		return setup.entityPerms, nil
	}
	resolver.EXPECT().ResolveForEntity(mock.Anything, mock.Anything).RunAndReturn(entity).Maybe()
	resolver.EXPECT().ResolveHeldByEntity(mock.Anything, mock.Anything).RunAndReturn(entity).Maybe()
	resolver.EXPECT().ResolveForGroup(mock.Anything, mock.Anything).Return(setup.groupPerms, setup.groupErr).Maybe()
	resolver.EXPECT().ResolveForRole(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, roleID string) (security.PermissionSet, error) {
			if setup.roles != nil {
				return setup.roles[roleID], setup.roleErr
			}
			return setup.rolePerms, setup.roleErr
		}).Maybe()
	resolver.EXPECT().ResolveDelimiters(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, resourceServerIDs []string) map[string]string {
			if setup.delimiters != nil {
				return setup.delimiters
			}
			defaults := make(map[string]string, len(resourceServerIDs))
			for _, id := range resourceServerIDs {
				defaults[id] = ":"
			}
			return defaults
		}).Maybe()
	return resolver
}

// entityCalls counts the entity lookups a resolver took, for what an entity may use or holds.
func entityCalls(resolver *PermissionResolverMock) int {
	return calls(resolver, "ResolveForEntity", "ResolveHeldByEntity")
}

// groupCalls counts the group lookups a resolver took.
func groupCalls(resolver *PermissionResolverMock) int {
	return calls(resolver, "ResolveForGroup")
}

// roleCalls counts the role lookups a resolver took.
func roleCalls(resolver *PermissionResolverMock) int {
	return calls(resolver, "ResolveForRole")
}

// calls counts the lookups a resolver took through the named methods.
func calls(resolver *PermissionResolverMock, methods ...string) int {
	count := 0
	for _, call := range resolver.Calls {
		for _, method := range methods {
			if call.Method == method {
				count++
			}
		}
	}
	return count
}

// newGuardService returns a service with the given resolver injected.
func newGuardService(resolver PermissionResolver) *systemAuthorizationService {
	svc := &systemAuthorizationService{
		logger: log.GetLogger().With(log.String("component", "SystemAuthorizationService")),
	}
	if resolver != nil {
		svc.SetPermissionResolver(resolver)
	}
	return svc
}

// callerWith builds an authenticated context carrying the given token permissions.
func callerWith(subject string, tokenPermissions ...string) context.Context {
	authCtx := security.NewSecurityContextForTest(subject, "ou1", "token", tokenPermissions, nil)
	return security.WithSecurityContextTest(context.Background(), authCtx)
}

// ---------------------------------------------------------------------------
// Short-circuits
// ---------------------------------------------------------------------------

func TestCanGrantMembership_ShortCircuits(t *testing.T) {
	// Bootstrap seeds the administrator group and role under a runtime context. Denying there
	// would stop the server from starting.
	t.Run("RuntimeContextIsAllowedWithoutAnyLookup", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(security.WithRuntimeContext(context.Background()),
			PrincipalTypeGroup, "grp1")

		assert.Nil(t, svcErr)
		assert.Zero(t, groupCalls(resolver), "runtime callers must not trigger a lookup")
		assert.Zero(t, entityCalls(resolver))
	})

	// A wiring regression must not silently disable the guard.
	t.Run("MissingResolverFailsClosed", func(t *testing.T) {
		svc := newGuardService(nil)

		svcErr := svc.CanGrantMembership(callerWith("user1", "system:group"), PrincipalTypeGroup, "grp1")

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	})

	t.Run("RootCallerIsAllowedWithoutAnyLookup", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("admin", "system"), PrincipalTypeGroup, "grp1")

		assert.Nil(t, svcErr)
		assert.Zero(t, groupCalls(resolver), "the root permission already covers everything, so no lookup is needed")
		assert.Zero(t, entityCalls(resolver))
	})

	t.Run("EmptyContainerIDFailsClosed", func(t *testing.T) {
		svc := newGuardService(newResolver(t, resolverSetup{}))

		svcErr := svc.CanGrantMembership(callerWith("user1", "system:group"), PrincipalTypeGroup, "")

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	})

	t.Run("UnknownPrincipalTypeFailsClosed", func(t *testing.T) {
		svc := newGuardService(newResolver(t, resolverSetup{}))

		svcErr := svc.CanGrantMembership(callerWith("user1", "system:group"),
			PrincipalType("application"), "app1")

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	})
}

// ---------------------------------------------------------------------------
// Coverage comparison
// ---------------------------------------------------------------------------

func TestCanGrantMembership_Coverage(t *testing.T) {
	// The assistant librarian holds group write and user read. The librarian group confers user
	// write, which the assistant does not hold, so adding anyone to it is refused.
	t.Run("AssistantCannotAddAnyoneToLibrarianGroup", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			groupPerms: security.PermissionSet{
				escRSSystem: {"system:user", "system:group"},
			},
			entityPerms: security.PermissionSet{
				escRSSystem: {"system:group", "system:user:view"},
			},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "librarianGroup")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
	})

	// Self-assignment is the same comparison; the guard never branches on who benefits.
	t.Run("AssistantCannotAddSelfEither", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			groupPerms:  security.PermissionSet{escRSSystem: {"system:user"}},
			entityPerms: security.PermissionSet{escRSSystem: {"system:group", "system:user:view"}},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "librarianGroup")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
	})

	t.Run("GrantOfExactlyWhatTheCallerHoldsIsAllowed", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			groupPerms:  security.PermissionSet{escRSSystem: {"system:group"}},
			entityPerms: security.PermissionSet{escRSSystem: {"system:group", "system:user:view"}},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "readingGroup")

		assert.Nil(t, svcErr)
	})

	t.Run("GrantOfAChildScopeOfWhatTheCallerHoldsIsAllowed", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			groupPerms:  security.PermissionSet{escRSSystem: {"system:group:view"}},
			entityPerms: security.PermissionSet{escRSSystem: {"system:group"}},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "viewerGroup")

		assert.Nil(t, svcErr)
	})

	// Most groups carry no roles. This is the hot path and must not resolve the caller at all.
	t.Run("GroupConferringNothingIsAllowedWithoutResolvingTheCaller", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{groupPerms: security.PermissionSet{}})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "plainGroup")

		assert.Nil(t, svcErr)
		assert.Equal(t, 1, groupCalls(resolver))
		assert.Zero(t, entityCalls(resolver), "caller lookup is unnecessary when nothing is conferred")
	})

	t.Run("GroupWithEmptyPermissionListsCountsAsConferringNothing", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			groupPerms: security.PermissionSet{escRSSystem: {}, escRSPayroll: {}},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "plainGroup")

		assert.Nil(t, svcErr)
		assert.Zero(t, entityCalls(resolver))
	})

	// Coverage is per resource server. Root on the system resource server says nothing about a
	// business resource server, so a non-root caller cannot use it to grant there.
	t.Run("PermissionsDoNotCoverAcrossResourceServers", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			groupPerms:  security.PermissionSet{escRSPayroll: {"payroll:read"}},
			entityPerms: security.PermissionSet{escRSSystem: {"system:group", "system:user"}},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "payrollGroup")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
	})

	t.Run("CoverageAcrossMultipleResourceServersIsAllowedWhenAllAreHeld", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			groupPerms: security.PermissionSet{
				escRSSystem:  {"system:group"},
				escRSPayroll: {"payroll:read"},
			},
			entityPerms: security.PermissionSet{
				escRSSystem:  {"system:group"},
				escRSPayroll: {"payroll"},
			},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "mixedGroup")

		assert.Nil(t, svcErr)
	})

	t.Run("RolesUseTheSameComparison", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			rolePerms:   security.PermissionSet{escRSSystem: {"system"}},
			entityPerms: security.PermissionSet{escRSSystem: {"system:group"}},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeRole, "administratorRole")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
		assert.Equal(t, 1, roleCalls(resolver))
		assert.Zero(t, groupCalls(resolver), "a role container must not be resolved as a group")
	})
}

// ---------------------------------------------------------------------------
// Failure handling
// ---------------------------------------------------------------------------

func TestCanGrantMembership_FailuresAreRefusals(t *testing.T) {
	// A failed container lookup must never read as "confers nothing", which would permit exactly
	// the grant this guard exists to refuse.
	t.Run("ContainerLookupErrorIsRefused", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{groupErr: errors.New("database unavailable")})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "grp1")

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
		assert.Zero(t, entityCalls(resolver))
	})

	t.Run("CallerLookupErrorIsRefused", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			groupPerms: security.PermissionSet{escRSSystem: {"system:user"}},
			entityErr:  errors.New("database unavailable"),
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "grp1")

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	})

	// A caller whose subject does not resolve to a local principal holds nothing we can verify.
	t.Run("CallerWithNoResolvablePermissionsIsRefused", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			groupPerms:  security.PermissionSet{escRSSystem: {"system:user"}},
			entityPerms: security.PermissionSet{},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("federated", "system:group"),
			PrincipalTypeGroup, "grp1")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
	})

	t.Run("UnauthenticatedCallerIsRefused", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			groupPerms: security.PermissionSet{escRSSystem: {"system:user"}},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(context.Background(), PrincipalTypeGroup, "grp1")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
		assert.Zero(t, entityCalls(resolver))
	})
}

// ---------------------------------------------------------------------------
// CanGrantPermissions
// ---------------------------------------------------------------------------

func TestCanGrantPermissions(t *testing.T) {
	t.Run("RuntimeContextIsAllowed", func(t *testing.T) {
		svc := newGuardService(newResolver(t, resolverSetup{}))

		svcErr := svc.CanGrantPermissions(security.WithRuntimeContext(context.Background()),
			security.PermissionSet{escRSSystem: {"system"}})

		assert.Nil(t, svcErr)
	})

	t.Run("MissingResolverFailsClosed", func(t *testing.T) {
		svc := newGuardService(nil)

		svcErr := svc.CanGrantPermissions(callerWith("user1", "system:group"),
			security.PermissionSet{escRSSystem: {"system:group"}})

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	})

	t.Run("RootCallerMayGrantAnything", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantPermissions(callerWith("admin", "system"),
			security.PermissionSet{escRSPayroll: {"payroll:write"}})

		assert.Nil(t, svcErr)
		assert.Zero(t, entityCalls(resolver))
	})

	// Defining a role that grants more than the caller holds is refused on the same basis as
	// adding someone to a group that confers it.
	t.Run("CallerCannotDefineARoleExceedingItsOwnPermissions", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			entityPerms: security.PermissionSet{escRSSystem: {"system:group"}},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantPermissions(callerWith("david", "system:group"),
			security.PermissionSet{escRSSystem: {"system:user"}})

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
	})

	t.Run("EmptyGrantIsAllowedWithoutResolvingTheCaller", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantPermissions(callerWith("david", "system:group"), security.PermissionSet{})

		assert.Nil(t, svcErr)
		assert.Zero(t, entityCalls(resolver))
	})

	// Covers refuses a malformed grant, and the guard must surface that as a refusal rather than
	// letting it through.
	t.Run("MalformedGrantIsRefused", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			entityPerms: security.PermissionSet{escRSSystem: {"system"}},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantPermissions(callerWith("david", "system:group"),
			security.PermissionSet{escRSSystem: {""}})

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
	})
}

// ---------------------------------------------------------------------------
// SetPermissionResolver
// ---------------------------------------------------------------------------

func TestSetPermissionResolver(t *testing.T) {
	t.Run("NilResolverDoesNotClearAnInjectedOne", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{groupPerms: security.PermissionSet{}})
		svc := newGuardService(resolver)

		svc.SetPermissionResolver(nil)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"), PrincipalTypeGroup, "grp1")
		assert.Nil(t, svcErr)
		assert.Equal(t, 1, groupCalls(resolver), "the previously injected resolver must still be used")
	})
}

// ---------------------------------------------------------------------------
// grantsNothing
// ---------------------------------------------------------------------------

func TestGrantsNothing(t *testing.T) {
	tests := []struct {
		name    string
		granted security.PermissionSet
		want    bool
	}{
		{name: "Nil", granted: nil, want: true},
		{name: "Empty", granted: security.PermissionSet{}, want: true},
		{name: "AllEmptyLists", granted: security.PermissionSet{escRSSystem: {}}, want: true},
		{name: "OnePermission", granted: security.PermissionSet{escRSSystem: {"system"}}, want: false},
		{
			name:    "OneNonEmptyAmongEmpties",
			granted: security.PermissionSet{escRSSystem: {}, escRSPayroll: {"payroll:read"}},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, grantsNothing(tt.granted))
		})
	}
}

func TestGrantableMemberships(t *testing.T) {
	// A caller who is not root may grant only the roles whose permissions it holds, and its own permissions are
	// resolved once for the whole list.
	t.Run("NonRootCallerGetsOnlyTheRolesItCovers", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			entityPerms: security.PermissionSet{escRSSystem: {"system:user"}},
			roles: map[string]security.PermissionSet{
				"user-admin":    {escRSSystem: {"system:user"}},
				"payroll-admin": {escRSPayroll: {"payroll:write"}},
				"empty":         {},
			},
		})
		svc := newGuardService(resolver)

		ids, svcErr := svc.GrantableMemberships(callerWith("david", "system:user"), PrincipalTypeRole,
			[]string{"payroll-admin", "user-admin", "empty"})

		require.Nil(t, svcErr)
		assert.Equal(t, []string{"user-admin", "empty"}, ids)
		assert.Equal(t, 1, entityCalls(resolver))
	})

	t.Run("RootAndRuntimeCallersGetEveryRoleWithoutALookup", func(t *testing.T) {
		for name, ctx := range map[string]context.Context{
			"root":    callerWith("root", "system"),
			"runtime": security.WithRuntimeContext(context.Background()),
		} {
			resolver := newResolver(t, resolverSetup{})
			svc := newGuardService(resolver)

			ids, svcErr := svc.GrantableMemberships(ctx, PrincipalTypeRole, []string{"a", "b"})

			require.Nil(t, svcErr, name)
			assert.Equal(t, []string{"a", "b"}, ids, name)
			assert.Zero(t, entityCalls(resolver)+roleCalls(resolver), name)
		}
	})

	t.Run("AnUnauthenticatedCallerGetsOnlyRolesConferringNothing", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{roles: map[string]security.PermissionSet{
			"user-admin": {escRSSystem: {"system:user"}},
			"empty":      {},
		}})
		svc := newGuardService(resolver)

		ids, svcErr := svc.GrantableMemberships(context.Background(), PrincipalTypeRole,
			[]string{"user-admin", "empty"})

		require.Nil(t, svcErr)
		assert.Equal(t, []string{"empty"}, ids)
		assert.Zero(t, entityCalls(resolver))
	})

	t.Run("AnEmptyListLooksNothingUp", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{})
		svc := newGuardService(resolver)

		ids, svcErr := svc.GrantableMemberships(callerWith("david", "system:user"), PrincipalTypeRole, nil)

		require.Nil(t, svcErr)
		assert.Empty(t, ids)
		assert.Zero(t, entityCalls(resolver)+roleCalls(resolver))
	})

	t.Run("AFailedLookupFailsTheCheck", func(t *testing.T) {
		for name, setup := range map[string]resolverSetup{
			"caller": {
				entityErr: errors.New("store down"),
				roles:     map[string]security.PermissionSet{"user-admin": {escRSSystem: {"system:user"}}},
			},
			"role": {roleErr: errors.New("store down")},
		} {
			svc := newGuardService(newResolver(t, setup))

			ids, svcErr := svc.GrantableMemberships(callerWith("david", "system:user"), PrincipalTypeRole,
				[]string{"user-admin"})

			assert.Nil(t, ids, name)
			require.NotNil(t, svcErr, name)
			assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code, name)
		}
	})

	t.Run("WithoutAResolverEveryRoleIsRefused", func(t *testing.T) {
		svc := newGuardService(nil)

		ids, svcErr := svc.GrantableMemberships(callerWith("david", "system:user"), PrincipalTypeRole,
			[]string{"user-admin"})

		assert.Nil(t, ids)
		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	})
}

// Missing metadata must not enable parent coverage.
func TestCanGrantPermissions_ExactWhenMatchingMetadataIsUnavailable(t *testing.T) {
	resolver := newResolver(t, resolverSetup{
		entityPerms: security.PermissionSet{escRSSystem: {"reservations"}},
		delimiters:  map[string]string{},
	})
	svc := newGuardService(resolver)

	svcErr := svc.CanGrantPermissions(callerWith("david", "system:group"),
		security.PermissionSet{escRSSystem: {"reservations:create"}})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
}

// A resolved delimiter can represent enabled hierarchy or legacy delegation.
func TestCanGrantPermissions_CoveredByResolvedDelimiter(t *testing.T) {
	resolver := newResolver(t, resolverSetup{
		entityPerms: security.PermissionSet{escRSSystem: {"reservations"}},
		delimiters:  map[string]string{escRSSystem: ":"},
	})
	svc := newGuardService(resolver)

	svcErr := svc.CanGrantPermissions(callerWith("david", "system:group"),
		security.PermissionSet{escRSSystem: {"reservations:create"}})

	assert.Nil(t, svcErr)
}

func TestGrantableMemberships_UsesResolvedDelimiters(t *testing.T) {
	tests := []struct {
		name, server, held, required string
		delimiters                   map[string]string
		allowed                      bool
	}{
		{"configured delimiter", "custom", "reports", "reports-view", map[string]string{"custom": "-"}, true},
		{"legacy colon", "custom", "reports", "reports:view", map[string]string{"custom": ":"}, true},
		{"missing metadata stays exact", "custom", "reports", "reports:view", map[string]string{}, false},
		{"System retains colon semantics", escRSSystem, "system:user", "system:user:view", map[string]string{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver := newResolver(t, resolverSetup{
				entityPerms: security.PermissionSet{tc.server: {tc.held}},
				rolePerms:   security.PermissionSet{tc.server: {tc.required}},
				delimiters:  tc.delimiters,
			})
			service := newGuardService(resolver)
			ids, svcErr := service.GrantableMemberships(callerWith("david", "system:user"), PrincipalTypeRole,
				[]string{"role-1"})
			require.Nil(t, svcErr)
			if tc.allowed {
				assert.Equal(t, []string{"role-1"}, ids)
			} else {
				assert.Empty(t, ids)
			}
			assert.Equal(t, 1, entityCalls(resolver))
		})
	}
}

// ---------------------------------------------------------------------------
// Entity control
// ---------------------------------------------------------------------------

func TestCanControlEntity(t *testing.T) {
	t.Run("EntityControlsSelfWithoutLookup", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{})
		svc := newGuardService(resolver)

		svcErr := svc.CanControlEntity(callerWith("user1", "system:user"), "user1")

		assert.Nil(t, svcErr)
		assert.Zero(t, entityCalls(resolver), "controlling self must not trigger lookups")
	})

	// The caller counts only what it may use, and the entity everything its roles hold, since AuthZEN evaluation
	// counts every assignment.
	t.Run("ComparesEverythingTheEntityHoldsWithWhatTheCallerMayUse", func(t *testing.T) {
		resolver := NewPermissionResolverMock(t)
		resolver.EXPECT().ResolveHeldByEntity(mock.Anything, "bob").Return(security.PermissionSet{
			escRSSystem: {"system:user:view"}, escRSPayroll: {"payroll:write"},
		}, nil).Once()
		resolver.EXPECT().ResolveForEntity(mock.Anything, "david").Return(security.PermissionSet{
			escRSSystem: {"system:user"},
		}, nil).Once()
		resolver.EXPECT().ResolveDelimiters(mock.Anything, mock.Anything).Return(map[string]string{}).Maybe()
		svc := newGuardService(resolver)

		svcErr := svc.CanControlEntity(callerWith("david", "system:user"), "bob")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
	})

	t.Run("AssistantCannotControlAdministratorEntity", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			entities: map[string]security.PermissionSet{
				"david": {escRSSystem: {"system:user:update"}},
				"admin": {escRSSystem: {"system:user", "system:group"}},
			},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanControlEntity(callerWith("david", "system:user:update"), "admin")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
		assert.Equal(t, 2, entityCalls(resolver))
	})

	t.Run("AdminCanControlEntityWhosePermissionsAreCovered", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			entities: map[string]security.PermissionSet{
				"admin": {escRSSystem: {"system:user"}},
				"bob":   {escRSSystem: {"system:user:view"}},
			},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanControlEntity(callerWith("admin", "system:user"), "bob")

		assert.Nil(t, svcErr)
		assert.Equal(t, 2, entityCalls(resolver))
	})

	t.Run("AdminCanControlUnprivilegedEntityConferringNothing", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{
			entities: map[string]security.PermissionSet{
				"admin":   {escRSSystem: {"system:user"}},
				"regular": {},
			},
		})
		svc := newGuardService(resolver)

		svcErr := svc.CanControlEntity(callerWith("admin", "system:user"), "regular")

		assert.Nil(t, svcErr)
		assert.Equal(t, 1, entityCalls(resolver), "unprivileged entity confers nothing, avoiding caller lookup")
	})

	t.Run("EntityLookupErrorIsRefused", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{entityErr: errors.New("database unavailable")})
		svc := newGuardService(resolver)

		svcErr := svc.CanControlEntity(callerWith("david", "system:user"), "otherUser")

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	})

	t.Run("MissingEntityIDFailsClosed", func(t *testing.T) {
		resolver := newResolver(t, resolverSetup{})
		svc := newGuardService(resolver)

		svcErr := svc.CanControlEntity(callerWith("david", "system:user"), "")

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
		assert.Zero(t, entityCalls(resolver))
	})
}
