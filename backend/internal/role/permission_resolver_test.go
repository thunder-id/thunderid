// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/security"

	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
)

func TestResolveForEntity(t *testing.T) {
	ctx := context.Background()

	t.Run("EmptyEntityIDYieldsEmptySet", func(t *testing.T) {
		resolver := NewEffectivePermissionResolver(nil, nil, nil)

		result, err := resolver.ResolveForEntity(ctx, "")

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("CombinesDirectAndGroupDerivedPermissions", func(t *testing.T) {
		entitySvc := entitymock.NewEntityServiceInterfaceMock(t)
		roleSvc := NewRoleServiceInterfaceMock(t)
		entitySvc.EXPECT().GetEntityProfile(ctx, "david").
			Return(&providers.Entity{ID: "david", OUID: "ou-a"}, nil).Once()
		entitySvc.EXPECT().GetTransitiveEntityGroups(ctx, "david").
			Return([]providers.EntityGroup{{ID: "grpA", OUID: "ou-a"}, {ID: "grpB", OUID: "ou-a"}}, nil).Once()
		roleSvc.EXPECT().GetAllPermissions(ctx, "david", []string{"grpA", "grpB"}, "ou-a").
			Return(security.PermissionSet{testRSSystem: {"system:group"}}, nil).Once()
		resolver := NewEffectivePermissionResolver(roleSvc, nil, entitySvc)

		result, err := resolver.ResolveForEntity(ctx, "david")

		require.NoError(t, err)
		assert.Equal(t, []string{"system:group"}, result[testRSSystem])
	})

	// Tokens count only roles owned by the entity's own organization unit, and groups in it. The
	// grant check must count the same, or a caller could grant what its tokens no longer carry.
	t.Run("BoundsRolesAndGroupsToTheEntitysOwnOU", func(t *testing.T) {
		entitySvc := entitymock.NewEntityServiceInterfaceMock(t)
		roleSvc := NewRoleServiceInterfaceMock(t)
		entitySvc.EXPECT().GetEntityProfile(ctx, "david").
			Return(&providers.Entity{ID: "david", OUID: "ou-a"}, nil).Once()
		entitySvc.EXPECT().GetTransitiveEntityGroups(ctx, "david").
			Return([]providers.EntityGroup{{ID: "grpA", OUID: "ou-a"}, {ID: "grpRoot", OUID: "ou-root"}}, nil).Once()
		roleSvc.EXPECT().GetAllPermissions(ctx, "david", []string{"grpA"}, "ou-a").
			Return(security.PermissionSet{testRSSystem: {"system:role"}}, nil).Once()
		resolver := NewEffectivePermissionResolver(roleSvc, nil, entitySvc)

		result, err := resolver.ResolveForEntity(ctx, "david")

		require.NoError(t, err)
		assert.Equal(t, []string{"system:role"}, result[testRSSystem])
	})

	t.Run("EntityWithNoGroups", func(t *testing.T) {
		entitySvc := entitymock.NewEntityServiceInterfaceMock(t)
		roleSvc := NewRoleServiceInterfaceMock(t)
		entitySvc.EXPECT().GetEntityProfile(ctx, "loner").
			Return(&providers.Entity{ID: "loner", OUID: "ou-a"}, nil).Once()
		entitySvc.EXPECT().GetTransitiveEntityGroups(ctx, "loner").
			Return([]providers.EntityGroup{}, nil).Once()
		roleSvc.EXPECT().GetAllPermissions(ctx, "loner", []string{}, "ou-a").
			Return(security.PermissionSet{}, nil).Once()
		resolver := NewEffectivePermissionResolver(roleSvc, nil, entitySvc)

		result, err := resolver.ResolveForEntity(ctx, "loner")

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	// A subject with no local record has no organization unit to count roles in, as for its tokens.
	t.Run("UnknownEntityHoldsNothing", func(t *testing.T) {
		entitySvc := entitymock.NewEntityServiceInterfaceMock(t)
		entitySvc.EXPECT().GetEntityProfile(ctx, "ghost").Return(nil, entity.ErrEntityNotFound).Once()
		resolver := NewEffectivePermissionResolver(nil, nil, entitySvc)

		result, err := resolver.ResolveForEntity(ctx, "ghost")

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("EntityWithoutOUHoldsNothing", func(t *testing.T) {
		entitySvc := entitymock.NewEntityServiceInterfaceMock(t)
		entitySvc.EXPECT().GetEntityProfile(ctx, "david").
			Return(&providers.Entity{ID: "david"}, nil).Once()
		resolver := NewEffectivePermissionResolver(nil, nil, entitySvc)

		result, err := resolver.ResolveForEntity(ctx, "david")

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	// A failed lookup must surface as an error. Reporting an empty set would understate the
	// caller's own authority and cause a false denial, or worse, be read as "holds nothing".
	t.Run("EntityLookupErrorPropagates", func(t *testing.T) {
		entitySvc := entitymock.NewEntityServiceInterfaceMock(t)
		entitySvc.EXPECT().GetEntityProfile(ctx, "david").
			Return(nil, errors.New("entity store down")).Once()
		resolver := NewEffectivePermissionResolver(nil, nil, entitySvc)

		result, err := resolver.ResolveForEntity(ctx, "david")

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("GroupLookupErrorPropagates", func(t *testing.T) {
		entitySvc := entitymock.NewEntityServiceInterfaceMock(t)
		entitySvc.EXPECT().GetEntityProfile(ctx, "david").
			Return(&providers.Entity{ID: "david", OUID: "ou-a"}, nil).Once()
		entitySvc.EXPECT().GetTransitiveEntityGroups(ctx, "david").
			Return(nil, errors.New("entity store down")).Once()
		resolver := NewEffectivePermissionResolver(nil, nil, entitySvc)

		result, err := resolver.ResolveForEntity(ctx, "david")

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("PermissionLookupErrorPropagates", func(t *testing.T) {
		entitySvc := entitymock.NewEntityServiceInterfaceMock(t)
		roleSvc := NewRoleServiceInterfaceMock(t)
		entitySvc.EXPECT().GetEntityProfile(ctx, "david").
			Return(&providers.Entity{ID: "david", OUID: "ou-a"}, nil).Once()
		entitySvc.EXPECT().GetTransitiveEntityGroups(ctx, "david").
			Return([]providers.EntityGroup{}, nil).Once()
		roleSvc.EXPECT().GetAllPermissions(ctx, "david", []string{}, "ou-a").
			Return(nil, &tidcommon.InternalServerError).Once()
		resolver := NewEffectivePermissionResolver(roleSvc, nil, entitySvc)

		result, err := resolver.ResolveForEntity(ctx, "david")

		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestResolveForGroup(t *testing.T) {
	ctx := context.Background()

	t.Run("EmptyGroupIDYieldsEmptySet", func(t *testing.T) {
		resolver := NewEffectivePermissionResolver(nil, nil, nil)

		result, err := resolver.ResolveForGroup(ctx, "")

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	// The group itself plus every ancestor. A group carrying no roles of its own still confers
	// whatever its ancestors carry, because its members are transitively members of them.
	t.Run("IncludesAncestorsAlongsideTheGroupItself", func(t *testing.T) {
		groupSvc := groupmock.NewGroupServiceInterfaceMock(t)
		roleSvc := NewRoleServiceInterfaceMock(t)
		groupSvc.EXPECT().GetTransitiveAncestorGroups(ctx, "innerGroup").
			Return([]string{"middleGroup", "librarianGroup"}, nil).Once()
		roleSvc.EXPECT().GetAllPermissions(ctx, "",
			[]string{"innerGroup", "middleGroup", "librarianGroup"}, "").
			Return(security.PermissionSet{testRSSystem: {"system:user"}}, nil).Once()
		resolver := NewEffectivePermissionResolver(roleSvc, groupSvc, nil)

		result, err := resolver.ResolveForGroup(ctx, "innerGroup")

		require.NoError(t, err)
		assert.Equal(t, []string{"system:user"}, result[testRSSystem],
			"a harmless group nested inside a privileged one must confer the privileged permissions")
	})

	t.Run("GroupItselfIsQueriedFirstAndAlwaysIncluded", func(t *testing.T) {
		groupSvc := groupmock.NewGroupServiceInterfaceMock(t)
		roleSvc := NewRoleServiceInterfaceMock(t)
		groupSvc.EXPECT().GetTransitiveAncestorGroups(ctx, "topGroup").
			Return([]string{}, nil).Once()
		roleSvc.EXPECT().GetAllPermissions(ctx, "", []string{"topGroup"}, "").
			Return(security.PermissionSet{testRSSystem: {"system"}}, nil).Once()
		resolver := NewEffectivePermissionResolver(roleSvc, groupSvc, nil)

		result, err := resolver.ResolveForGroup(ctx, "topGroup")

		require.NoError(t, err)
		assert.Equal(t, []string{"system"}, result[testRSSystem])
	})

	// No entity ID is passed: the question is what the group confers, not what any member holds.
	t.Run("ResolvesWithoutAnEntityID", func(t *testing.T) {
		groupSvc := groupmock.NewGroupServiceInterfaceMock(t)
		roleSvc := NewRoleServiceInterfaceMock(t)
		groupSvc.EXPECT().GetTransitiveAncestorGroups(ctx, "grp1").Return([]string{}, nil).Once()
		roleSvc.EXPECT().GetAllPermissions(ctx, "", []string{"grp1"}, "").
			Return(security.PermissionSet{}, nil).Once()
		resolver := NewEffectivePermissionResolver(roleSvc, groupSvc, nil)

		_, err := resolver.ResolveForGroup(ctx, "grp1")

		require.NoError(t, err)
	})

	// If the ancestor walk fails, treating the group as conferring only its own roles would
	// understate what it confers through the nested-group path.
	t.Run("AncestorLookupErrorPropagates", func(t *testing.T) {
		groupSvc := groupmock.NewGroupServiceInterfaceMock(t)
		groupSvc.EXPECT().GetTransitiveAncestorGroups(ctx, "grp1").
			Return(nil, &tidcommon.InternalServerError).Once()
		resolver := NewEffectivePermissionResolver(nil, groupSvc, nil)

		result, err := resolver.ResolveForGroup(ctx, "grp1")

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("PermissionLookupErrorPropagates", func(t *testing.T) {
		groupSvc := groupmock.NewGroupServiceInterfaceMock(t)
		roleSvc := NewRoleServiceInterfaceMock(t)
		groupSvc.EXPECT().GetTransitiveAncestorGroups(ctx, "grp1").Return([]string{}, nil).Once()
		roleSvc.EXPECT().GetAllPermissions(ctx, "", []string{"grp1"}, "").
			Return(nil, &tidcommon.InternalServerError).Once()
		resolver := NewEffectivePermissionResolver(roleSvc, groupSvc, nil)

		result, err := resolver.ResolveForGroup(ctx, "grp1")

		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestResolveForRole(t *testing.T) {
	ctx := context.Background()

	t.Run("EmptyRoleIDYieldsEmptySet", func(t *testing.T) {
		resolver := NewEffectivePermissionResolver(nil, nil, nil)

		result, err := resolver.ResolveForRole(ctx, "")

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("GroupsPermissionsByResourceServer", func(t *testing.T) {
		roleSvc := NewRoleServiceInterfaceMock(t)
		roleSvc.EXPECT().GetRoleWithPermissions(ctx, "librarian").Return(&RoleWithPermissions{
			ID: "librarian",
			Permissions: []ResourcePermissions{
				{ResourceServerID: testRSSystem, Permissions: []string{"system:user", "system:group"}},
				{ResourceServerID: testRSPayroll, Permissions: []string{"payroll:read"}},
			},
		}, nil).Once()
		resolver := NewEffectivePermissionResolver(roleSvc, nil, nil)

		result, err := resolver.ResolveForRole(ctx, "librarian")

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"system:user", "system:group"}, result[testRSSystem])
		assert.Equal(t, []string{"payroll:read"}, result[testRSPayroll])
	})

	t.Run("RoleWithNoPermissions", func(t *testing.T) {
		roleSvc := NewRoleServiceInterfaceMock(t)
		roleSvc.EXPECT().GetRoleWithPermissions(ctx, "empty").Return(&RoleWithPermissions{
			ID:          "empty",
			Permissions: []ResourcePermissions{},
		}, nil).Once()
		resolver := NewEffectivePermissionResolver(roleSvc, nil, nil)

		result, err := resolver.ResolveForRole(ctx, "empty")

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	// A missing role must not read as "confers nothing", which would permit assigning it.
	t.Run("LookupErrorPropagates", func(t *testing.T) {
		roleSvc := NewRoleServiceInterfaceMock(t)
		roleSvc.EXPECT().GetRoleWithPermissions(ctx, "gone").
			Return(nil, &ErrorRoleNotFound).Once()
		resolver := NewEffectivePermissionResolver(roleSvc, nil, nil)

		result, err := resolver.ResolveForRole(ctx, "gone")

		require.Error(t, err)
		assert.Nil(t, result)
	})
}
