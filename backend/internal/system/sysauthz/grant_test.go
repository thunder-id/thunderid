// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const (
	escRSSystem  = "01900000-0000-7000-8000-000000000020"
	escRSPayroll = "01900000-0000-7000-8000-0000000000aa"
)

// stubPermissionResolver is a configurable PermissionResolver for testing. It records how often
// each method was called so tests can assert that short-circuits avoid unnecessary lookups.
type stubPermissionResolver struct {
	entityPerms security.PermissionSet
	entityErr   error
	entityCalls int

	groupPerms security.PermissionSet
	groupErr   error
	groupCalls int

	rolePerms security.PermissionSet
	roleErr   error
	roleCalls int

	// delimiters is what the resource servers being granted on report. Left nil, every resource
	// server named is reported as ":"-delimited, which is what the system one is and what the cases
	// below are about. Set it to override administrative delimiters or model missing metadata.
	delimiters map[string]string
}

func (s *stubPermissionResolver) ResolveDelimiters(
	_ context.Context, resourceServerIDs []string,
) map[string]string {
	if s.delimiters != nil {
		return s.delimiters
	}
	defaults := make(map[string]string, len(resourceServerIDs))
	for _, id := range resourceServerIDs {
		defaults[id] = ":"
	}
	return defaults
}

func (r *stubPermissionResolver) ResolveForEntity(
	_ context.Context, _ string,
) (security.PermissionSet, error) {
	r.entityCalls++
	return r.entityPerms, r.entityErr
}

func (r *stubPermissionResolver) ResolveForGroup(
	_ context.Context, _ string,
) (security.PermissionSet, error) {
	r.groupCalls++
	return r.groupPerms, r.groupErr
}

func (r *stubPermissionResolver) ResolveForRole(
	_ context.Context, _ string,
) (security.PermissionSet, error) {
	r.roleCalls++
	return r.rolePerms, r.roleErr
}

// perEntityResolver resolves each entity to its own permissions.
type perEntityResolver struct {
	stubPermissionResolver
	entities map[string]security.PermissionSet
}

func (r *perEntityResolver) ResolveForEntity(_ context.Context, entityID string) (security.PermissionSet, error) {
	r.entityCalls++
	if r.entityErr != nil {
		return nil, r.entityErr
	}
	return r.entities[entityID], nil
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
		resolver := &stubPermissionResolver{}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(security.WithRuntimeContext(context.Background()),
			PrincipalTypeGroup, "grp1")

		assert.Nil(t, svcErr)
		assert.Zero(t, resolver.groupCalls, "runtime callers must not trigger a lookup")
		assert.Zero(t, resolver.entityCalls)
	})

	// A wiring regression must not silently disable the guard.
	t.Run("MissingResolverFailsClosed", func(t *testing.T) {
		svc := newGuardService(nil)

		svcErr := svc.CanGrantMembership(callerWith("user1", "system:group"), PrincipalTypeGroup, "grp1")

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	})

	t.Run("RootCallerIsAllowedWithoutAnyLookup", func(t *testing.T) {
		resolver := &stubPermissionResolver{}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("admin", "system"), PrincipalTypeGroup, "grp1")

		assert.Nil(t, svcErr)
		assert.Zero(t, resolver.groupCalls, "the root permission already covers everything, so no lookup is needed")
		assert.Zero(t, resolver.entityCalls)
	})

	t.Run("EmptyContainerIDFailsClosed", func(t *testing.T) {
		svc := newGuardService(&stubPermissionResolver{})

		svcErr := svc.CanGrantMembership(callerWith("user1", "system:group"), PrincipalTypeGroup, "")

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	})

	t.Run("UnknownPrincipalTypeFailsClosed", func(t *testing.T) {
		svc := newGuardService(&stubPermissionResolver{})

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
		resolver := &stubPermissionResolver{
			groupPerms: security.PermissionSet{
				escRSSystem: {"system:user", "system:group"},
			},
			entityPerms: security.PermissionSet{
				escRSSystem: {"system:group", "system:user:view"},
			},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "librarianGroup")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
	})

	// Self-assignment is the same comparison; the guard never branches on who benefits.
	t.Run("AssistantCannotAddSelfEither", func(t *testing.T) {
		resolver := &stubPermissionResolver{
			groupPerms:  security.PermissionSet{escRSSystem: {"system:user"}},
			entityPerms: security.PermissionSet{escRSSystem: {"system:group", "system:user:view"}},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "librarianGroup")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
	})

	t.Run("GrantOfExactlyWhatTheCallerHoldsIsAllowed", func(t *testing.T) {
		resolver := &stubPermissionResolver{
			groupPerms:  security.PermissionSet{escRSSystem: {"system:group"}},
			entityPerms: security.PermissionSet{escRSSystem: {"system:group", "system:user:view"}},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "readingGroup")

		assert.Nil(t, svcErr)
	})

	t.Run("GrantOfAChildScopeOfWhatTheCallerHoldsIsAllowed", func(t *testing.T) {
		resolver := &stubPermissionResolver{
			groupPerms:  security.PermissionSet{escRSSystem: {"system:group:view"}},
			entityPerms: security.PermissionSet{escRSSystem: {"system:group"}},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "viewerGroup")

		assert.Nil(t, svcErr)
	})

	// Most groups carry no roles. This is the hot path and must not resolve the caller at all.
	t.Run("GroupConferringNothingIsAllowedWithoutResolvingTheCaller", func(t *testing.T) {
		resolver := &stubPermissionResolver{groupPerms: security.PermissionSet{}}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "plainGroup")

		assert.Nil(t, svcErr)
		assert.Equal(t, 1, resolver.groupCalls)
		assert.Zero(t, resolver.entityCalls, "caller lookup is unnecessary when nothing is conferred")
	})

	t.Run("GroupWithEmptyPermissionListsCountsAsConferringNothing", func(t *testing.T) {
		resolver := &stubPermissionResolver{
			groupPerms: security.PermissionSet{escRSSystem: {}, escRSPayroll: {}},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "plainGroup")

		assert.Nil(t, svcErr)
		assert.Zero(t, resolver.entityCalls)
	})

	// Coverage is per resource server. Root on the system resource server says nothing about a
	// business resource server, so a non-root caller cannot use it to grant there.
	t.Run("PermissionsDoNotCoverAcrossResourceServers", func(t *testing.T) {
		resolver := &stubPermissionResolver{
			groupPerms:  security.PermissionSet{escRSPayroll: {"payroll:read"}},
			entityPerms: security.PermissionSet{escRSSystem: {"system:group", "system:user"}},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "payrollGroup")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
	})

	t.Run("CoverageAcrossMultipleResourceServersIsAllowedWhenAllAreHeld", func(t *testing.T) {
		resolver := &stubPermissionResolver{
			groupPerms: security.PermissionSet{
				escRSSystem:  {"system:group"},
				escRSPayroll: {"payroll:read"},
			},
			entityPerms: security.PermissionSet{
				escRSSystem:  {"system:group"},
				escRSPayroll: {"payroll"},
			},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "mixedGroup")

		assert.Nil(t, svcErr)
	})

	t.Run("RolesUseTheSameComparison", func(t *testing.T) {
		resolver := &stubPermissionResolver{
			rolePerms:   security.PermissionSet{escRSSystem: {"system"}},
			entityPerms: security.PermissionSet{escRSSystem: {"system:group"}},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeRole, "administratorRole")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
		assert.Equal(t, 1, resolver.roleCalls)
		assert.Zero(t, resolver.groupCalls, "a role container must not be resolved as a group")
	})
}

// ---------------------------------------------------------------------------
// Failure handling
// ---------------------------------------------------------------------------

func TestCanGrantMembership_FailuresAreRefusals(t *testing.T) {
	// A failed container lookup must never read as "confers nothing", which would permit exactly
	// the grant this guard exists to refuse.
	t.Run("ContainerLookupErrorIsRefused", func(t *testing.T) {
		resolver := &stubPermissionResolver{groupErr: errors.New("database unavailable")}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "grp1")

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
		assert.Zero(t, resolver.entityCalls)
	})

	t.Run("CallerLookupErrorIsRefused", func(t *testing.T) {
		resolver := &stubPermissionResolver{
			groupPerms: security.PermissionSet{escRSSystem: {"system:user"}},
			entityErr:  errors.New("database unavailable"),
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"),
			PrincipalTypeGroup, "grp1")

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	})

	// A caller whose subject does not resolve to a local principal holds nothing we can verify.
	t.Run("CallerWithNoResolvablePermissionsIsRefused", func(t *testing.T) {
		resolver := &stubPermissionResolver{
			groupPerms:  security.PermissionSet{escRSSystem: {"system:user"}},
			entityPerms: security.PermissionSet{},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(callerWith("federated", "system:group"),
			PrincipalTypeGroup, "grp1")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
	})

	t.Run("UnauthenticatedCallerIsRefused", func(t *testing.T) {
		resolver := &stubPermissionResolver{
			groupPerms: security.PermissionSet{escRSSystem: {"system:user"}},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantMembership(context.Background(), PrincipalTypeGroup, "grp1")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
		assert.Zero(t, resolver.entityCalls)
	})
}

// ---------------------------------------------------------------------------
// CanGrantPermissions
// ---------------------------------------------------------------------------

func TestCanGrantPermissions(t *testing.T) {
	t.Run("RuntimeContextIsAllowed", func(t *testing.T) {
		svc := newGuardService(&stubPermissionResolver{})

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
		resolver := &stubPermissionResolver{}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantPermissions(callerWith("admin", "system"),
			security.PermissionSet{escRSPayroll: {"payroll:write"}})

		assert.Nil(t, svcErr)
		assert.Zero(t, resolver.entityCalls)
	})

	// Defining a role that grants more than the caller holds is refused on the same basis as
	// adding someone to a group that confers it.
	t.Run("CallerCannotDefineARoleExceedingItsOwnPermissions", func(t *testing.T) {
		resolver := &stubPermissionResolver{
			entityPerms: security.PermissionSet{escRSSystem: {"system:group"}},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantPermissions(callerWith("david", "system:group"),
			security.PermissionSet{escRSSystem: {"system:user"}})

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
	})

	t.Run("EmptyGrantIsAllowedWithoutResolvingTheCaller", func(t *testing.T) {
		resolver := &stubPermissionResolver{}
		svc := newGuardService(resolver)

		svcErr := svc.CanGrantPermissions(callerWith("david", "system:group"), security.PermissionSet{})

		assert.Nil(t, svcErr)
		assert.Zero(t, resolver.entityCalls)
	})

	// Covers refuses a malformed grant, and the guard must surface that as a refusal rather than
	// letting it through.
	t.Run("MalformedGrantIsRefused", func(t *testing.T) {
		resolver := &stubPermissionResolver{
			entityPerms: security.PermissionSet{escRSSystem: {"system"}},
		}
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
		resolver := &stubPermissionResolver{groupPerms: security.PermissionSet{}}
		svc := newGuardService(resolver)

		svc.SetPermissionResolver(nil)

		svcErr := svc.CanGrantMembership(callerWith("david", "system:group"), PrincipalTypeGroup, "grp1")
		assert.Nil(t, svcErr)
		assert.Equal(t, 1, resolver.groupCalls, "the previously injected resolver must still be used")
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

// perRoleResolver resolves each role to its own permissions and counts the caller lookups.
type perRoleResolver struct {
	stubPermissionResolver
	roles map[string]security.PermissionSet
}

func (r *perRoleResolver) ResolveForRole(_ context.Context, roleID string) (security.PermissionSet, error) {
	r.roleCalls++
	return r.roles[roleID], r.roleErr
}

func TestGrantableMemberships(t *testing.T) {
	// A caller who is not root may grant only the roles whose permissions it holds, and its own permissions are
	// resolved once for the whole list.
	t.Run("NonRootCallerGetsOnlyTheRolesItCovers", func(t *testing.T) {
		resolver := &perRoleResolver{
			stubPermissionResolver: stubPermissionResolver{
				entityPerms: security.PermissionSet{escRSSystem: {"system:user"}},
			},
			roles: map[string]security.PermissionSet{
				"user-admin":    {escRSSystem: {"system:user"}},
				"payroll-admin": {escRSPayroll: {"payroll:write"}},
				"empty":         {},
			},
		}
		svc := newGuardService(resolver)

		ids, svcErr := svc.GrantableMemberships(callerWith("david", "system:user"), PrincipalTypeRole,
			[]string{"payroll-admin", "user-admin", "empty"})

		require.Nil(t, svcErr)
		assert.Equal(t, []string{"user-admin", "empty"}, ids)
		assert.Equal(t, 1, resolver.entityCalls)
	})

	t.Run("RootAndRuntimeCallersGetEveryRoleWithoutALookup", func(t *testing.T) {
		for name, ctx := range map[string]context.Context{
			"root":    callerWith("root", "system"),
			"runtime": security.WithRuntimeContext(context.Background()),
		} {
			resolver := &stubPermissionResolver{}
			svc := newGuardService(resolver)

			ids, svcErr := svc.GrantableMemberships(ctx, PrincipalTypeRole, []string{"a", "b"})

			require.Nil(t, svcErr, name)
			assert.Equal(t, []string{"a", "b"}, ids, name)
			assert.Zero(t, resolver.entityCalls+resolver.roleCalls, name)
		}
	})

	t.Run("AnUnauthenticatedCallerGetsOnlyRolesConferringNothing", func(t *testing.T) {
		resolver := &perRoleResolver{roles: map[string]security.PermissionSet{
			"user-admin": {escRSSystem: {"system:user"}},
			"empty":      {},
		}}
		svc := newGuardService(resolver)

		ids, svcErr := svc.GrantableMemberships(context.Background(), PrincipalTypeRole,
			[]string{"user-admin", "empty"})

		require.Nil(t, svcErr)
		assert.Equal(t, []string{"empty"}, ids)
		assert.Zero(t, resolver.entityCalls)
	})

	t.Run("AnEmptyListLooksNothingUp", func(t *testing.T) {
		resolver := &stubPermissionResolver{}
		svc := newGuardService(resolver)

		ids, svcErr := svc.GrantableMemberships(callerWith("david", "system:user"), PrincipalTypeRole, nil)

		require.Nil(t, svcErr)
		assert.Empty(t, ids)
		assert.Zero(t, resolver.entityCalls+resolver.roleCalls)
	})

	t.Run("AFailedLookupFailsTheCheck", func(t *testing.T) {
		for name, resolver := range map[string]*perRoleResolver{
			"caller": {
				stubPermissionResolver: stubPermissionResolver{entityErr: errors.New("store down")},
				roles:                  map[string]security.PermissionSet{"user-admin": {escRSSystem: {"system:user"}}},
			},
			"role": {stubPermissionResolver: stubPermissionResolver{roleErr: errors.New("store down")}},
		} {
			svc := newGuardService(resolver)

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
	resolver := &stubPermissionResolver{
		entityPerms: security.PermissionSet{escRSSystem: {"reservations"}},
		delimiters:  map[string]string{},
	}
	svc := newGuardService(resolver)

	svcErr := svc.CanGrantPermissions(callerWith("david", "system:group"),
		security.PermissionSet{escRSSystem: {"reservations:create"}})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
}

// A resolved delimiter can represent enabled hierarchy or legacy delegation.
func TestCanGrantPermissions_CoveredByResolvedDelimiter(t *testing.T) {
	resolver := &stubPermissionResolver{
		entityPerms: security.PermissionSet{escRSSystem: {"reservations"}},
		delimiters:  map[string]string{escRSSystem: ":"},
	}
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
			resolver := &stubPermissionResolver{
				entityPerms: security.PermissionSet{tc.server: {tc.held}},
				rolePerms:   security.PermissionSet{tc.server: {tc.required}},
				delimiters:  tc.delimiters,
			}
			service := newGuardService(resolver)
			ids, svcErr := service.GrantableMemberships(callerWith("david", "system:user"), PrincipalTypeRole,
				[]string{"role-1"})
			require.Nil(t, svcErr)
			if tc.allowed {
				assert.Equal(t, []string{"role-1"}, ids)
			} else {
				assert.Empty(t, ids)
			}
			assert.Equal(t, 1, resolver.entityCalls)
		})
	}
}

// ---------------------------------------------------------------------------
// Entity control
// ---------------------------------------------------------------------------

func TestCanControlEntity(t *testing.T) {
	t.Run("EntityControlsSelfWithoutLookup", func(t *testing.T) {
		resolver := &stubPermissionResolver{}
		svc := newGuardService(resolver)

		svcErr := svc.CanControlEntity(callerWith("user1", "system:user"), "user1")

		assert.Nil(t, svcErr)
		assert.Zero(t, resolver.entityCalls, "controlling self must not trigger lookups")
	})

	t.Run("AssistantCannotControlAdministratorEntity", func(t *testing.T) {
		resolver := &perEntityResolver{
			entities: map[string]security.PermissionSet{
				"david": {escRSSystem: {"system:user:update"}},
				"admin": {escRSSystem: {"system:user", "system:group"}},
			},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanControlEntity(callerWith("david", "system:user:update"), "admin")

		require.NotNil(t, svcErr)
		assert.Equal(t, ErrorGrantNotPermitted.Code, svcErr.Code)
		assert.Equal(t, 2, resolver.entityCalls)
	})

	t.Run("AdminCanControlEntityWhosePermissionsAreCovered", func(t *testing.T) {
		resolver := &perEntityResolver{
			entities: map[string]security.PermissionSet{
				"admin": {escRSSystem: {"system:user"}},
				"bob":   {escRSSystem: {"system:user:view"}},
			},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanControlEntity(callerWith("admin", "system:user"), "bob")

		assert.Nil(t, svcErr)
		assert.Equal(t, 2, resolver.entityCalls)
	})

	t.Run("AdminCanControlUnprivilegedEntityConferringNothing", func(t *testing.T) {
		resolver := &perEntityResolver{
			entities: map[string]security.PermissionSet{
				"admin":   {escRSSystem: {"system:user"}},
				"regular": {},
			},
		}
		svc := newGuardService(resolver)

		svcErr := svc.CanControlEntity(callerWith("admin", "system:user"), "regular")

		assert.Nil(t, svcErr)
		assert.Equal(t, 1, resolver.entityCalls, "unprivileged entity confers nothing, avoiding caller lookup")
	})

	t.Run("EntityLookupErrorIsRefused", func(t *testing.T) {
		resolver := &stubPermissionResolver{entityErr: errors.New("database unavailable")}
		svc := newGuardService(resolver)

		svcErr := svc.CanControlEntity(callerWith("david", "system:user"), "otherUser")

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
	})

	t.Run("MissingEntityIDFailsClosed", func(t *testing.T) {
		resolver := &stubPermissionResolver{}
		svc := newGuardService(resolver)

		svcErr := svc.CanControlEntity(callerWith("david", "system:user"), "")

		require.NotNil(t, svcErr)
		assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
		assert.Zero(t, resolver.entityCalls)
	})
}
