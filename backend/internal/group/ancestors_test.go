// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

// ---------------------------------------------------------------------------
// buildGetDirectGroupParentsQuery
// ---------------------------------------------------------------------------

func TestBuildGetDirectGroupParentsQuery(t *testing.T) {
	t.Run("NoGroupIDsYieldsMatchNothingQuery", func(t *testing.T) {
		query, args := buildGetDirectGroupParentsQuery(nil, testDeploymentID)

		assert.Equal(t, "GRQ-GROUP_MGT-24", query.ID)
		assert.Empty(t, args)
		assert.Contains(t, query.PostgresQuery, "WHERE 1=0")
		assert.Contains(t, query.SQLiteQuery, "WHERE 1=0")
	})

	t.Run("SingleGroupID", func(t *testing.T) {
		query, args := buildGetDirectGroupParentsQuery([]string{"grp1"}, testDeploymentID)

		assert.Contains(t, query.PostgresQuery, "MEMBER_ID IN ($1)")
		assert.Contains(t, query.PostgresQuery, "DEPLOYMENT_ID = $2")
		assert.Contains(t, query.SQLiteQuery, "MEMBER_ID IN (?)")
		assert.Contains(t, query.SQLiteQuery, "DEPLOYMENT_ID = ?")
		assert.Equal(t, []interface{}{"grp1", testDeploymentID}, args)
	})

	t.Run("MultipleGroupIDsKeepDeploymentIDLast", func(t *testing.T) {
		query, args := buildGetDirectGroupParentsQuery([]string{"grp1", "grp2", "grp3"}, testDeploymentID)

		assert.Contains(t, query.PostgresQuery, "MEMBER_ID IN ($1,$2,$3)")
		assert.Contains(t, query.PostgresQuery, "DEPLOYMENT_ID = $4")
		assert.Contains(t, query.SQLiteQuery, "MEMBER_ID IN (?,?,?)")
		assert.Equal(t, []interface{}{"grp1", "grp2", "grp3", testDeploymentID}, args)
	})

	t.Run("PlaceholderCountMatchesArgCount", func(t *testing.T) {
		for _, n := range []int{1, 2, 5, 17} {
			groupIDs := make([]string, n)
			for i := range groupIDs {
				groupIDs[i] = fmt.Sprintf("grp%d", i)
			}
			query, args := buildGetDirectGroupParentsQuery(groupIDs, testDeploymentID)

			require.Len(t, args, n+1, "arg count for %d group IDs", n)
			assert.Equal(t, n+1, strings.Count(query.SQLiteQuery, "?"),
				"sqlite placeholder count for %d group IDs", n)
			assert.Contains(t, query.PostgresQuery, fmt.Sprintf("DEPLOYMENT_ID = $%d", n+1))
		}
	})

	t.Run("OnlyGroupTypeMembershipEdgesAreFollowed", func(t *testing.T) {
		query, _ := buildGetDirectGroupParentsQuery([]string{"grp1"}, testDeploymentID)

		assert.Contains(t, query.PostgresQuery, "MEMBER_TYPE = 'group'")
		assert.Contains(t, query.SQLiteQuery, "MEMBER_TYPE = 'group'")
	})

	// A declarative parent has no row in "GROUP", so joining it would drop that parent from the
	// ancestor set and understate what the group confers.
	t.Run("DoesNotJoinGroupTable", func(t *testing.T) {
		query, _ := buildGetDirectGroupParentsQuery([]string{"grp1"}, testDeploymentID)

		assert.NotContains(t, query.PostgresQuery, "JOIN")
		assert.NotContains(t, query.SQLiteQuery, "JOIN")
	})
}

// ---------------------------------------------------------------------------
// resolveTransitiveGroupAncestors
// ---------------------------------------------------------------------------

func TestResolveTransitiveGroupAncestors(t *testing.T) {
	ctx := context.Background()

	t.Run("NoParentsYieldsEmptyAncestors", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.EXPECT().GetDirectGroupParents(ctx, []string{"leaf"}).Return([]string{}, nil).Once()

		ancestors, err := resolveTransitiveGroupAncestors(ctx, store, "leaf")

		require.NoError(t, err)
		assert.Empty(t, ancestors)
	})

	t.Run("SingleLevel", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.EXPECT().GetDirectGroupParents(ctx, []string{"child"}).Return([]string{"parent"}, nil).Once()
		store.EXPECT().GetDirectGroupParents(ctx, []string{"parent"}).Return([]string{}, nil).Once()

		ancestors, err := resolveTransitiveGroupAncestors(ctx, store, "child")

		require.NoError(t, err)
		assert.Equal(t, []string{"parent"}, ancestors)
	})

	// A group nested two levels below one that carries roles. Stopping at the first level would
	// miss the higher ancestor entirely.
	t.Run("TwoLevelNestReachesTheTopAncestor", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.EXPECT().GetDirectGroupParents(ctx, []string{"inner"}).Return([]string{"middle"}, nil).Once()
		store.EXPECT().GetDirectGroupParents(ctx, []string{"middle"}).Return([]string{"librarian"}, nil).Once()
		store.EXPECT().GetDirectGroupParents(ctx, []string{"librarian"}).Return([]string{}, nil).Once()

		ancestors, err := resolveTransitiveGroupAncestors(ctx, store, "inner")

		require.NoError(t, err)
		assert.Equal(t, []string{"middle", "librarian"}, ancestors)
	})

	t.Run("DiamondIsNotVisitedTwice", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.EXPECT().GetDirectGroupParents(ctx, []string{"base"}).
			Return([]string{"left", "right"}, nil).Once()
		store.EXPECT().GetDirectGroupParents(ctx, []string{"left", "right"}).
			Return([]string{"top", "top"}, nil).Once()
		store.EXPECT().GetDirectGroupParents(ctx, []string{"top"}).Return([]string{}, nil).Once()

		ancestors, err := resolveTransitiveGroupAncestors(ctx, store, "base")

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"left", "right", "top"}, ancestors)
		assert.Len(t, ancestors, 3, "top must appear once despite two paths to it")
	})

	t.Run("DirectCycleTerminates", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.EXPECT().GetDirectGroupParents(ctx, []string{"a"}).Return([]string{"b"}, nil).Once()
		store.EXPECT().GetDirectGroupParents(ctx, []string{"b"}).Return([]string{"a"}, nil).Once()

		ancestors, err := resolveTransitiveGroupAncestors(ctx, store, "a")

		require.NoError(t, err)
		assert.Equal(t, []string{"b"}, ancestors, "the seed group is never its own ancestor")
	})

	t.Run("SelfReferenceTerminates", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.EXPECT().GetDirectGroupParents(ctx, []string{"a"}).Return([]string{"a"}, nil).Once()

		ancestors, err := resolveTransitiveGroupAncestors(ctx, store, "a")

		require.NoError(t, err)
		assert.Empty(t, ancestors)
	})

	t.Run("SeedGroupIsExcludedFromAncestors", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.EXPECT().GetDirectGroupParents(ctx, []string{"self"}).Return([]string{"parent"}, nil).Once()
		store.EXPECT().GetDirectGroupParents(ctx, []string{"parent"}).Return([]string{}, nil).Once()

		ancestors, err := resolveTransitiveGroupAncestors(ctx, store, "self")

		require.NoError(t, err)
		assert.NotContains(t, ancestors, "self")
	})

	// A store error must surface, never be reported as "no ancestors" — the caller treats an empty
	// ancestor set as "this group confers nothing", which would let a grant through.
	t.Run("StoreErrorPropagates", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.EXPECT().GetDirectGroupParents(ctx, []string{"child"}).
			Return(nil, errors.New("database unavailable")).Once()

		ancestors, err := resolveTransitiveGroupAncestors(ctx, store, "child")

		require.Error(t, err)
		assert.Nil(t, ancestors)
		assert.Contains(t, err.Error(), "database unavailable")
	})

	t.Run("ErrorAtDeeperLevelPropagates", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.EXPECT().GetDirectGroupParents(ctx, []string{"child"}).Return([]string{"parent"}, nil).Once()
		store.EXPECT().GetDirectGroupParents(ctx, []string{"parent"}).
			Return(nil, errors.New("timeout")).Once()

		ancestors, err := resolveTransitiveGroupAncestors(ctx, store, "child")

		require.Error(t, err)
		assert.Nil(t, ancestors)
	})

	// An unbounded chain must fail closed rather than issue queries forever.
	t.Run("ExcessiveDepthIsAnError", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.EXPECT().GetDirectGroupParents(ctx, mock.Anything).
			RunAndReturn(func(_ context.Context, frontier []string) ([]string, error) {
				// Every group has exactly one distinct parent, forever.
				return []string{frontier[0] + "x"}, nil
			})

		ancestors, err := resolveTransitiveGroupAncestors(ctx, store, "g")

		require.Error(t, err)
		assert.Nil(t, ancestors)
		assert.Contains(t, err.Error(), "maximum supported depth")
	})

	t.Run("WalkIsBatchedPerLevelNotPerGroup", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		// Three siblings at one level must be resolved in a single call.
		store.EXPECT().GetDirectGroupParents(ctx, []string{"base"}).
			Return([]string{"p1", "p2", "p3"}, nil).Once()
		store.EXPECT().GetDirectGroupParents(ctx, []string{"p1", "p2", "p3"}).
			Return([]string{}, nil).Once()

		ancestors, err := resolveTransitiveGroupAncestors(ctx, store, "base")

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"p1", "p2", "p3"}, ancestors)
	})
}

// ---------------------------------------------------------------------------
// compositeGroupStore.GetDirectGroupParents
// ---------------------------------------------------------------------------

func TestCompositeGetDirectGroupParents(t *testing.T) {
	ctx := context.Background()

	newComposite := func(t *testing.T) (*groupStoreInterfaceMock, *groupStoreInterfaceMock, groupStoreInterface) {
		dbStore := newGroupStoreInterfaceMock(t)
		fileStore := newGroupStoreInterfaceMock(t)
		return dbStore, fileStore, newCompositeGroupStore(fileStore, dbStore)
	}

	t.Run("UnionsBothStores", func(t *testing.T) {
		dbStore, fileStore, store := newComposite(t)
		dbStore.EXPECT().GetDirectGroupParents(ctx, []string{"g"}).Return([]string{"dbParent"}, nil).Once()
		fileStore.EXPECT().GetDirectGroupParents(ctx, []string{"g"}).
			Return([]string{"fileParent"}, nil).Once()

		parents, err := store.GetDirectGroupParents(ctx, []string{"g"})

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"dbParent", "fileParent"}, parents)
	})

	t.Run("DeduplicatesAcrossStores", func(t *testing.T) {
		dbStore, fileStore, store := newComposite(t)
		dbStore.EXPECT().GetDirectGroupParents(ctx, []string{"g"}).Return([]string{"shared"}, nil).Once()
		fileStore.EXPECT().GetDirectGroupParents(ctx, []string{"g"}).Return([]string{"shared"}, nil).Once()

		parents, err := store.GetDirectGroupParents(ctx, []string{"g"})

		require.NoError(t, err)
		assert.Equal(t, []string{"shared"}, parents)
	})

	// Unioning at each hop is what makes a nesting chain that crosses stores resolvable. Asking
	// either store to resolve the whole chain alone stops at the edge it cannot see.
	t.Run("ResolvesNestingChainThatCrossesStores", func(t *testing.T) {
		dbStore, fileStore, store := newComposite(t)
		// dbChild is nested in a declarative group, which is itself nested in a database group.
		dbStore.EXPECT().GetDirectGroupParents(ctx, []string{"dbChild"}).Return([]string{}, nil).Once()
		fileStore.EXPECT().GetDirectGroupParents(ctx, []string{"dbChild"}).
			Return([]string{"declarativeMiddle"}, nil).Once()
		dbStore.EXPECT().GetDirectGroupParents(ctx, []string{"declarativeMiddle"}).
			Return([]string{"dbTop"}, nil).Once()
		fileStore.EXPECT().GetDirectGroupParents(ctx, []string{"declarativeMiddle"}).
			Return([]string{}, nil).Once()
		dbStore.EXPECT().GetDirectGroupParents(ctx, []string{"dbTop"}).Return([]string{}, nil).Once()
		fileStore.EXPECT().GetDirectGroupParents(ctx, []string{"dbTop"}).Return([]string{}, nil).Once()

		ancestors, err := resolveTransitiveGroupAncestors(ctx, store, "dbChild")

		require.NoError(t, err)
		assert.Equal(t, []string{"declarativeMiddle", "dbTop"}, ancestors)
	})

	t.Run("DatabaseStoreErrorPropagates", func(t *testing.T) {
		dbStore, _, store := newComposite(t)
		dbStore.EXPECT().GetDirectGroupParents(ctx, []string{"g"}).
			Return(nil, errors.New("db down")).Once()

		parents, err := store.GetDirectGroupParents(ctx, []string{"g"})

		require.Error(t, err)
		assert.Nil(t, parents)
	})

	t.Run("FileStoreErrorPropagates", func(t *testing.T) {
		dbStore, fileStore, store := newComposite(t)
		dbStore.EXPECT().GetDirectGroupParents(ctx, []string{"g"}).Return([]string{}, nil).Once()
		fileStore.EXPECT().GetDirectGroupParents(ctx, []string{"g"}).
			Return(nil, errors.New("file store unreadable")).Once()

		parents, err := store.GetDirectGroupParents(ctx, []string{"g"})

		require.Error(t, err)
		assert.Nil(t, parents)
	})
}

// ---------------------------------------------------------------------------
// modifyGroupMembers grant checks
// ---------------------------------------------------------------------------

// newGrantRefusingAuthz returns an authz mock that permits the OU access check but refuses the
// membership grant, mirroring a caller whose permissions do not cover what the group confers.
func newGrantRefusingAuthz(t *testing.T) *sysauthzmock.SystemAuthorizationServiceInterfaceMock {
	mockAuthz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	mockAuthz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(true, (*tidcommon.ServiceError)(nil)).Maybe()
	mockAuthz.On("GetAccessibleResources", mock.Anything, mock.Anything, security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{AllAllowed: true}, (*tidcommon.ServiceError)(nil)).Maybe()
	mockAuthz.On("CanGrantMembership", mock.Anything, mock.Anything, mock.Anything).
		Return(&sysauthz.ErrorGrantNotPermitted).Maybe()
	return mockAuthz
}

func TestModifyGroupMembers_RefusesDisallowedGrant(t *testing.T) {
	ctx := context.Background()
	member := []Member{{ID: "bob", Type: MemberTypeUser}}

	// An actor authorized to manage groups adds a member to a group conferring permissions the
	// actor does not hold. The write must not reach the store.
	t.Run("AddIsRefusedAndNothingIsWritten", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.On("GetGroup", mock.Anything, "librarianGroup").
			Return(GroupDAO{ID: "librarianGroup", OUID: "ou-1"}, nil)
		svc := &groupService{
			bus:           discardingBus{},
			groupStore:    store,
			authzService:  newGrantRefusingAuthz(t),
			entityService: entitymock.NewEntityServiceInterfaceMock(t),
			transactioner: &stubTransactioner{},
		}

		grp, svcErr := svc.AddGroupMembers(ctx, "librarianGroup", member)

		require.NotNil(t, svcErr)
		assert.Equal(t, sysauthz.ErrorGrantNotPermitted.Code, svcErr.Code)
		assert.Nil(t, grp)
		store.AssertNotCalled(t, "AddGroupMembers", mock.Anything, mock.Anything, mock.Anything)
	})

	// Removal carries the same standing requirement, so a limited administrator cannot strip
	// members from a group more powerful than itself.
	t.Run("RemoveIsRefusedAndNothingIsWritten", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.On("GetGroup", mock.Anything, "librarianGroup").
			Return(GroupDAO{ID: "librarianGroup", OUID: "ou-1"}, nil)
		svc := &groupService{
			bus:           discardingBus{},
			groupStore:    store,
			authzService:  newGrantRefusingAuthz(t),
			entityService: entitymock.NewEntityServiceInterfaceMock(t),
			transactioner: &stubTransactioner{},
		}

		grp, svcErr := svc.RemoveGroupMembers(ctx, "librarianGroup", member)

		require.NotNil(t, svcErr)
		assert.Equal(t, sysauthz.ErrorGrantNotPermitted.Code, svcErr.Code)
		assert.Nil(t, grp)
		store.AssertNotCalled(t, "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
	})

	// Nesting a group the actor controls into a privileged group makes that group's members
	// transitive members of the privileged one, so group-typed members are guarded too.
	t.Run("NestingAGroupIntoAPrivilegedGroupIsRefused", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.On("GetGroup", mock.Anything, "librarianGroup").
			Return(GroupDAO{ID: "librarianGroup", OUID: "ou-1"}, nil)
		svc := &groupService{
			bus:           discardingBus{},
			groupStore:    store,
			authzService:  newGrantRefusingAuthz(t),
			entityService: entitymock.NewEntityServiceInterfaceMock(t),
			transactioner: &stubTransactioner{},
		}

		grp, svcErr := svc.AddGroupMembers(ctx, "librarianGroup",
			[]Member{{ID: "actorControlledGroup", Type: MemberTypeGroup}})

		require.NotNil(t, svcErr)
		assert.Equal(t, sysauthz.ErrorGrantNotPermitted.Code, svcErr.Code)
		assert.Nil(t, grp)
		store.AssertNotCalled(t, "AddGroupMembers", mock.Anything, mock.Anything, mock.Anything)
	})

	// The guard runs before member validation, so a refusal does not depend on the members
	// resolving successfully and cannot be probed for member existence.
	t.Run("GuardRunsBeforeMemberValidation", func(t *testing.T) {
		store := newGroupStoreInterfaceMock(t)
		store.On("GetGroup", mock.Anything, "librarianGroup").
			Return(GroupDAO{ID: "librarianGroup", OUID: "ou-1"}, nil)
		entitySvc := entitymock.NewEntityServiceInterfaceMock(t)
		svc := &groupService{
			bus:           discardingBus{},
			groupStore:    store,
			authzService:  newGrantRefusingAuthz(t),
			entityService: entitySvc,
			transactioner: &stubTransactioner{},
		}

		_, svcErr := svc.AddGroupMembers(ctx, "librarianGroup", member)

		require.NotNil(t, svcErr)
		entitySvc.AssertNotCalled(t, "GetEntitiesByIDs", mock.Anything, mock.Anything)
	})
}
