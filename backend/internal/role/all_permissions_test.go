// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testRSSystem  = "01900000-0000-7000-8000-000000000020"
	testRSPayroll = "01900000-0000-7000-8000-0000000000aa"
)

// ---------------------------------------------------------------------------
// buildAllPermissionsForAssigneesQuery
// ---------------------------------------------------------------------------

func TestBuildAllPermissionsForAssigneesQuery(t *testing.T) {
	const deploymentID = "test-deployment"

	t.Run("NoEntityAndNoGroupsYieldsMatchNothingQuery", func(t *testing.T) {
		query, args := buildAllPermissionsForAssigneesQuery("", nil, deploymentID)

		assert.Equal(t, "RLQ-ROLE_MGT-26", query.ID)
		assert.Empty(t, args)
		assert.Contains(t, query.PostgresQuery, "WHERE 1=0")
		assert.Contains(t, query.SQLiteQuery, "WHERE 1=0")
	})

	t.Run("ProjectsResourceServerAlongsidePermission", func(t *testing.T) {
		query, _ := buildAllPermissionsForAssigneesQuery("user1", nil, deploymentID)

		assert.Contains(t, query.PostgresQuery, "rp.RESOURCE_SERVER_ID")
		assert.Contains(t, query.PostgresQuery, "rp.PERMISSION")
	})

	// The point of this query is to enumerate, so neither filter may appear.
	t.Run("AppliesNoResourceServerOrPermissionFilter", func(t *testing.T) {
		query, _ := buildAllPermissionsForAssigneesQuery("user1", []string{"grp1"}, deploymentID)

		assert.NotContains(t, query.PostgresQuery, "rp.PERMISSION IN")
		assert.NotContains(t, query.PostgresQuery, "rp.RESOURCE_SERVER_ID =")
		assert.NotContains(t, query.SQLiteQuery, "rp.PERMISSION IN")
		assert.NotContains(t, query.SQLiteQuery, "rp.RESOURCE_SERVER_ID =")
	})

	t.Run("EntityOnly", func(t *testing.T) {
		query, args := buildAllPermissionsForAssigneesQuery("user1", nil, deploymentID)

		assert.Contains(t, query.PostgresQuery, "ra.ASSIGNEE_TYPE = 'entity' AND ra.ASSIGNEE_ID = $2")
		assert.NotContains(t, query.PostgresQuery, "ASSIGNEE_TYPE = 'group'")
		assert.Equal(t, []interface{}{deploymentID, "user1"}, args)
	})

	t.Run("GroupsOnly", func(t *testing.T) {
		query, args := buildAllPermissionsForAssigneesQuery("", []string{"grp1", "grp2"}, deploymentID)

		assert.Contains(t, query.PostgresQuery, "ra.ASSIGNEE_ID IN ($2,$3)")
		assert.NotContains(t, query.PostgresQuery, "ASSIGNEE_TYPE = 'entity'")
		assert.Equal(t, []interface{}{deploymentID, "grp1", "grp2"}, args)
	})

	t.Run("EntityAndGroupsAreOred", func(t *testing.T) {
		query, args := buildAllPermissionsForAssigneesQuery("user1", []string{"grp1", "grp2"}, deploymentID)

		assert.Contains(t, query.PostgresQuery, " OR ")
		assert.Contains(t, query.PostgresQuery, "ra.ASSIGNEE_ID = $2")
		assert.Contains(t, query.PostgresQuery, "ra.ASSIGNEE_ID IN ($3,$4)")
		assert.Equal(t, []interface{}{deploymentID, "user1", "grp1", "grp2"}, args)
	})

	// DEPLOYMENT_ID is $1 and occurs three times. SQLite gives the named form $1 the first free
	// index and reuses it, so the following ? placeholders must line up with args[1:].
	t.Run("SQLitePlaceholderCountMatchesArgsAfterDeploymentID", func(t *testing.T) {
		for _, n := range []int{0, 1, 3, 12} {
			groupIDs := make([]string, n)
			for i := range groupIDs {
				groupIDs[i] = fmt.Sprintf("grp%d", i)
			}
			query, args := buildAllPermissionsForAssigneesQuery("user1", groupIDs, deploymentID)

			require.Len(t, args, n+2, "args for %d groups: deployment + entity + groups", n)
			assert.Equal(t, n+1, strings.Count(query.SQLiteQuery, "?"),
				"sqlite ? count must equal args after the deployment ID, for %d groups", n)
			assert.Equal(t, 3, strings.Count(query.SQLiteQuery, "$1"),
				"deployment ID must stay the first parameter, for %d groups", n)
		}
	})

	t.Run("OrdersDeterministically", func(t *testing.T) {
		query, _ := buildAllPermissionsForAssigneesQuery("user1", nil, deploymentID)

		assert.Contains(t, query.PostgresQuery, "ORDER BY rp.RESOURCE_SERVER_ID, rp.PERMISSION")
		assert.Contains(t, query.SQLiteQuery, "ORDER BY rp.RESOURCE_SERVER_ID, rp.PERMISSION")
	})
}

// ---------------------------------------------------------------------------
// resourcePermissionsFromMap / mergeResourcePermissions
// ---------------------------------------------------------------------------

func TestResourcePermissionsFromMap(t *testing.T) {
	t.Run("EmptyMapYieldsEmptySlice", func(t *testing.T) {
		assert.Empty(t, resourcePermissionsFromMap(map[string][]string{}))
	})

	t.Run("DeduplicatesAndSorts", func(t *testing.T) {
		result := resourcePermissionsFromMap(map[string][]string{
			testRSPayroll: {"payroll:write", "payroll:read", "payroll:read"},
			testRSSystem:  {"system:user"},
		})

		require.Len(t, result, 2)
		// Resource servers are sorted by ID. testRSSystem ends in "...20" and testRSPayroll in
		// "...aa", and '2' < 'a', so the system resource server comes first.
		assert.Equal(t, testRSSystem, result[0].ResourceServerID)
		assert.Equal(t, testRSPayroll, result[1].ResourceServerID)
		assert.Equal(t, []string{"system:user"}, result[0].Permissions)
		assert.Equal(t, []string{"payroll:read", "payroll:write"}, result[1].Permissions)
	})
}

func TestMergeResourcePermissions(t *testing.T) {
	t.Run("UnionsAcrossSources", func(t *testing.T) {
		result := mergeResourcePermissions(
			[]ResourcePermissions{{ResourceServerID: testRSSystem, Permissions: []string{"system:user"}}},
			[]ResourcePermissions{{ResourceServerID: testRSSystem, Permissions: []string{"system:group"}}},
			[]ResourcePermissions{{ResourceServerID: testRSPayroll, Permissions: []string{"payroll:read"}}},
		)

		require.Len(t, result, 2)
		byRS := map[string][]string{}
		for _, rp := range result {
			byRS[rp.ResourceServerID] = rp.Permissions
		}
		assert.Equal(t, []string{"system:group", "system:user"}, byRS[testRSSystem])
		assert.Equal(t, []string{"payroll:read"}, byRS[testRSPayroll])
	})

	t.Run("DeduplicatesOverlapBetweenSources", func(t *testing.T) {
		result := mergeResourcePermissions(
			[]ResourcePermissions{{ResourceServerID: testRSSystem, Permissions: []string{"system"}}},
			[]ResourcePermissions{{ResourceServerID: testRSSystem, Permissions: []string{"system"}}},
		)

		require.Len(t, result, 1)
		assert.Equal(t, []string{"system"}, result[0].Permissions)
	})

	t.Run("NoSourcesYieldsEmpty", func(t *testing.T) {
		assert.Empty(t, mergeResourcePermissions())
	})
}

// ---------------------------------------------------------------------------
// compositeRoleStore.GetAllPermissionsForAssignees
// ---------------------------------------------------------------------------

func TestCompositeGetAllPermissionsForAssignees(t *testing.T) {
	ctx := context.Background()

	newComposite := func(t *testing.T) (*roleStoreInterfaceMock, *roleStoreInterfaceMock, roleStoreInterface) {
		dbStore := newRoleStoreInterfaceMock(t)
		fileStore := newRoleStoreInterfaceMock(t)
		return dbStore, fileStore, newCompositeRoleStore(fileStore, dbStore)
	}

	t.Run("NoEntityAndNoGroupsShortCircuits", func(t *testing.T) {
		_, _, store := newComposite(t)

		result, err := store.GetAllPermissionsForAssignees(ctx, "", nil)

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("UnionsDatabaseAndDeclarativeSources", func(t *testing.T) {
		dbStore, fileStore, store := newComposite(t)
		dbStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
			Return([]ResourcePermissions{
				{ResourceServerID: testRSSystem, Permissions: []string{"system:group"}},
			}, nil).Once()
		fileStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
			Return([]ResourcePermissions{
				{ResourceServerID: testRSSystem, Permissions: []string{"system:user"}},
			}, nil).Once()
		dbStore.EXPECT().GetEntityRoleIDs(ctx, "user1", []string{}).Return([]string{}, nil).Once()

		result, err := store.GetAllPermissionsForAssignees(ctx, "user1", []string{})

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, []string{"system:group", "system:user"}, result[0].Permissions)
	})

	// The third source: a declarative role definition whose assignment was added at runtime and
	// therefore lives in the database. Neither store sees this on its own.
	t.Run("IncludesCrossStoreDeclarativeRoleWithDatabaseAssignment", func(t *testing.T) {
		dbStore, fileStore, store := newComposite(t)
		dbStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
			Return([]ResourcePermissions{}, nil).Once()
		fileStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
			Return([]ResourcePermissions{}, nil).Once()
		dbStore.EXPECT().GetEntityRoleIDs(ctx, "user1", []string{}).
			Return([]string{"declarativeRole"}, nil).Once()
		fileStore.EXPECT().IsRoleExist(ctx, "declarativeRole").Return(true, nil).Once()
		fileStore.EXPECT().GetRole(ctx, "declarativeRole").Return(RoleWithPermissions{
			ID: "declarativeRole",
			Permissions: []ResourcePermissions{
				{ResourceServerID: testRSSystem, Permissions: []string{"system"}},
			},
		}, nil).Once()

		result, err := store.GetAllPermissionsForAssignees(ctx, "user1", []string{})

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, []string{"system"}, result[0].Permissions,
			"a declarative role assigned at runtime must still be reported")
	})

	t.Run("SkipsDatabaseOnlyRolesInCrossStorePass", func(t *testing.T) {
		dbStore, fileStore, store := newComposite(t)
		dbStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
			Return([]ResourcePermissions{}, nil).Once()
		fileStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
			Return([]ResourcePermissions{}, nil).Once()
		dbStore.EXPECT().GetEntityRoleIDs(ctx, "user1", []string{}).Return([]string{"dbRole"}, nil).Once()
		fileStore.EXPECT().IsRoleExist(ctx, "dbRole").Return(false, nil).Once()

		result, err := store.GetAllPermissionsForAssignees(ctx, "user1", []string{})

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("SkipsRemovedDeclarativeRole", func(t *testing.T) {
		dbStore, fileStore, store := newComposite(t)
		dbStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
			Return([]ResourcePermissions{}, nil).Once()
		fileStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
			Return([]ResourcePermissions{}, nil).Once()
		dbStore.EXPECT().GetEntityRoleIDs(ctx, "user1", []string{}).Return([]string{"gone"}, nil).Once()
		fileStore.EXPECT().IsRoleExist(ctx, "gone").Return(true, nil).Once()
		fileStore.EXPECT().GetRole(ctx, "gone").Return(RoleWithPermissions{}, ErrRoleNotFound).Once()

		result, err := store.GetAllPermissionsForAssignees(ctx, "user1", []string{})

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	// A storage failure must not read as "holds nothing": the caller compares this against the
	// permissions a grant would transfer, so an empty set would wrongly permit the grant.
	t.Run("UnexpectedFileStoreErrorPropagates", func(t *testing.T) {
		dbStore, fileStore, store := newComposite(t)
		dbStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
			Return([]ResourcePermissions{}, nil).Once()
		fileStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
			Return([]ResourcePermissions{}, nil).Once()
		dbStore.EXPECT().GetEntityRoleIDs(ctx, "user1", []string{}).Return([]string{"r1"}, nil).Once()
		fileStore.EXPECT().IsRoleExist(ctx, "r1").Return(true, nil).Once()
		fileStore.EXPECT().GetRole(ctx, "r1").
			Return(RoleWithPermissions{}, errors.New("disk failure")).Once()

		result, err := store.GetAllPermissionsForAssignees(ctx, "user1", []string{})

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("DatabaseStoreErrorPropagates", func(t *testing.T) {
		dbStore, _, store := newComposite(t)
		dbStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
			Return(nil, errors.New("db down")).Once()

		result, err := store.GetAllPermissionsForAssignees(ctx, "user1", []string{})

		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// ---------------------------------------------------------------------------
// roleService.GetAllPermissions
// ---------------------------------------------------------------------------

func TestRoleServiceGetAllPermissions(t *testing.T) {
	ctx := context.Background()

	t.Run("ConvertsToPermissionSetKeyedByResourceServer", func(t *testing.T) {
		store := newRoleStoreInterfaceMock(t)
		store.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{"grp1"}).
			Return([]ResourcePermissions{
				{ResourceServerID: testRSSystem, Permissions: []string{"system:group"}},
				{ResourceServerID: testRSPayroll, Permissions: []string{"payroll:read"}},
			}, nil).Once()
		svc := &roleService{roleStore: store}

		result, svcErr := svc.GetAllPermissions(ctx, "user1", []string{"grp1"})

		require.Nil(t, svcErr)
		assert.Equal(t, []string{"system:group"}, result[testRSSystem])
		assert.Equal(t, []string{"payroll:read"}, result[testRSPayroll])
	})

	// Unlike GetAuthorizedPermissionsByResourceServer, holding nothing is a valid answer rather
	// than a missing-argument error.
	t.Run("NoEntityAndNoGroupsIsAnEmptySetNotAnError", func(t *testing.T) {
		svc := &roleService{roleStore: newRoleStoreInterfaceMock(t)}

		result, svcErr := svc.GetAllPermissions(ctx, "", nil)

		require.Nil(t, svcErr)
		assert.Empty(t, result)
	})

	t.Run("NilGroupsIsNormalized", func(t *testing.T) {
		store := newRoleStoreInterfaceMock(t)
		store.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
			Return([]ResourcePermissions{}, nil).Once()
		svc := &roleService{roleStore: store}

		result, svcErr := svc.GetAllPermissions(ctx, "user1", nil)

		require.Nil(t, svcErr)
		assert.Empty(t, result)
	})

	t.Run("StoreErrorBecomesInternalServerErrorAndNilSet", func(t *testing.T) {
		store := newRoleStoreInterfaceMock(t)
		store.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
			Return(nil, errors.New("db down")).Once()
		svc := &roleService{roleStore: store}

		result, svcErr := svc.GetAllPermissions(ctx, "user1", []string{})

		require.NotNil(t, svcErr)
		assert.Nil(t, result, "a failed lookup must not be reported as an empty permission set")
	})
}

// ---------------------------------------------------------------------------
// Grant checks on the role paths
// ---------------------------------------------------------------------------

// newRefusingRoleAuthz returns an authz mock that refuses every grant check.
func newRefusingRoleAuthz(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
	mockAuthz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	mockAuthz.On("CanGrantPermissions", mock.Anything, mock.Anything).
		Return(&sysauthz.ErrorGrantNotPermitted).Maybe()
	mockAuthz.On("CanGrantMembership", mock.Anything, mock.Anything, mock.Anything).
		Return(&sysauthz.ErrorGrantNotPermitted).Maybe()
	return mockAuthz
}

func TestRoleAssignmentGuard(t *testing.T) {
	ctx := context.Background()
	assignments := []RoleAssignment{{ID: "bob", Type: AssigneeTypeUser}}

	// Assigning a role transfers its permissions, so a caller that does not hold them is refused
	// before anything reaches the store.
	t.Run("AddAssignmentsIsRefused", func(t *testing.T) {
		store := newRoleStoreInterfaceMock(t)
		txr := &fakeTransactioner{}
		svc := &roleAssignmentService{
			roleStore:     store,
			transactioner: txr,
			authzService:  newRefusingRoleAuthz(t),
		}

		svcErr := svc.AddAssignments(ctx, "librarianRole", assignments)

		require.NotNil(t, svcErr)
		assert.Equal(t, sysauthz.ErrorGrantNotPermitted.Code, svcErr.Code)
		assert.Zero(t, txr.transactCalls, "no transaction may be opened for a refused grant")
		store.AssertNotCalled(t, "AddAssignments", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("RemoveAssignmentsIsRefused", func(t *testing.T) {
		store := newRoleStoreInterfaceMock(t)
		txr := &fakeTransactioner{}
		svc := &roleAssignmentService{
			roleStore:     store,
			transactioner: txr,
			authzService:  newRefusingRoleAuthz(t),
		}

		svcErr := svc.RemoveAssignments(ctx, "librarianRole", assignments)

		require.NotNil(t, svcErr)
		assert.Equal(t, sysauthz.ErrorGrantNotPermitted.Code, svcErr.Code)
		assert.Zero(t, txr.transactCalls)
		store.AssertNotCalled(t, "RemoveAssignments", mock.Anything, mock.Anything, mock.Anything)
	})

	// The guard runs before assignee validation, so a refusal cannot be used to probe which
	// principals exist.
	t.Run("GuardRunsBeforeAssigneeValidation", func(t *testing.T) {
		entitySvc := entitymock.NewEntityServiceInterfaceMock(t)
		svc := &roleAssignmentService{
			roleStore:     newRoleStoreInterfaceMock(t),
			entityService: entitySvc,
			transactioner: &fakeTransactioner{},
			authzService:  newRefusingRoleAuthz(t),
		}

		svcErr := svc.AddAssignments(ctx, "librarianRole", assignments)

		require.NotNil(t, svcErr)
		entitySvc.AssertNotCalled(t, "GetEntitiesByIDs", mock.Anything, mock.Anything)
	})
}

func TestToPermissionSet(t *testing.T) {
	t.Run("GroupsByResourceServer", func(t *testing.T) {
		result := toPermissionSet([]ResourcePermissions{
			{ResourceServerID: testRSSystem, Permissions: []string{"system:user"}},
			{ResourceServerID: testRSPayroll, Permissions: []string{"payroll:read"}},
		})

		assert.Equal(t, []string{"system:user"}, result[testRSSystem])
		assert.Equal(t, []string{"payroll:read"}, result[testRSPayroll])
	})

	t.Run("MergesDuplicateResourceServerEntries", func(t *testing.T) {
		result := toPermissionSet([]ResourcePermissions{
			{ResourceServerID: testRSSystem, Permissions: []string{"system:user"}},
			{ResourceServerID: testRSSystem, Permissions: []string{"system:group"}},
		})

		assert.ElementsMatch(t, []string{"system:user", "system:group"}, result[testRSSystem])
	})

	t.Run("EmptyInputYieldsEmptySet", func(t *testing.T) {
		assert.Empty(t, toPermissionSet(nil))
	})
}

// A corrupt declarative role must fail the whole enumeration rather than being skipped. Skipping
// would understate what an assignment confers.
func TestCrossStoreAllPermissions_CorruptRoleFailsClosed(t *testing.T) {
	ctx := context.Background()
	dbStore := newRoleStoreInterfaceMock(t)
	fileStore := newRoleStoreInterfaceMock(t)
	store := newCompositeRoleStore(fileStore, dbStore)

	dbStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
		Return([]ResourcePermissions{}, nil).Once()
	fileStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
		Return([]ResourcePermissions{}, nil).Once()
	dbStore.EXPECT().GetEntityRoleIDs(ctx, "user1", []string{}).Return([]string{"corrupt"}, nil).Once()
	fileStore.EXPECT().IsRoleExist(ctx, "corrupt").Return(true, nil).Once()
	fileStore.EXPECT().GetRole(ctx, "corrupt").
		Return(RoleWithPermissions{}, ErrRoleDataCorrupted).Once()

	result, err := store.GetAllPermissionsForAssignees(ctx, "user1", []string{})

	require.Error(t, err, "corruption must not be silently skipped")
	assert.Nil(t, result)
}

// A role whose declarative definition was removed after the assignment was made genuinely confers
// nothing, so skipping it remains correct.
func TestCrossStoreAllPermissions_RemovedRoleIsStillSkipped(t *testing.T) {
	ctx := context.Background()
	dbStore := newRoleStoreInterfaceMock(t)
	fileStore := newRoleStoreInterfaceMock(t)
	store := newCompositeRoleStore(fileStore, dbStore)

	dbStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
		Return([]ResourcePermissions{}, nil).Once()
	fileStore.EXPECT().GetAllPermissionsForAssignees(ctx, "user1", []string{}).
		Return([]ResourcePermissions{}, nil).Once()
	dbStore.EXPECT().GetEntityRoleIDs(ctx, "user1", []string{}).Return([]string{"gone"}, nil).Once()
	fileStore.EXPECT().IsRoleExist(ctx, "gone").Return(true, nil).Once()
	fileStore.EXPECT().GetRole(ctx, "gone").Return(RoleWithPermissions{}, ErrRoleNotFound).Once()

	result, err := store.GetAllPermissionsForAssignees(ctx, "user1", []string{})

	require.NoError(t, err)
	assert.Empty(t, result)
}

// A role defined declaratively in the file store but assigned to the user via the DB must still
// appear in the user's role names, not just in GetRoleAssignments/GetAllPermissionsForAssignees.
func TestGetUserRoles_BridgesDBAssignmentOntoFileDefinedRole(t *testing.T) {
	ctx := context.Background()
	dbStore := newRoleStoreInterfaceMock(t)
	fileStore := newRoleStoreInterfaceMock(t)
	store := newCompositeRoleStore(fileStore, dbStore)

	dbStore.EXPECT().GetUserRoles(ctx, "user1", []string{}).Return([]string{"dbRole"}, nil).Once()
	fileStore.EXPECT().GetUserRoles(ctx, "user1", []string{}).Return([]string{}, nil).Once()
	dbStore.EXPECT().GetEntityRoleIDs(ctx, "user1", []string{}).Return([]string{"declarativeRole"}, nil).Once()
	fileStore.EXPECT().IsRoleExist(ctx, "declarativeRole").Return(true, nil).Once()
	fileStore.EXPECT().GetRole(ctx, "declarativeRole").
		Return(RoleWithPermissions{Name: "Declarative Admin"}, nil).Once()

	result, err := store.GetUserRoles(ctx, "user1", []string{})

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"dbRole", "Declarative Admin"}, result)
}

// A role ID from a DB assignment that is not declarative is already covered by dbStore.GetUserRoles,
// so it must not be resolved (or double-counted) via the file store.
func TestCrossStoreUserRoles_DBOnlyRoleIsSkipped(t *testing.T) {
	ctx := context.Background()
	dbStore := newRoleStoreInterfaceMock(t)
	fileStore := newRoleStoreInterfaceMock(t)
	store := newCompositeRoleStore(fileStore, dbStore)

	dbStore.EXPECT().GetUserRoles(ctx, "user1", []string{}).Return([]string{"dbRole"}, nil).Once()
	fileStore.EXPECT().GetUserRoles(ctx, "user1", []string{}).Return([]string{}, nil).Once()
	dbStore.EXPECT().GetEntityRoleIDs(ctx, "user1", []string{}).Return([]string{"dbOnlyRole"}, nil).Once()
	fileStore.EXPECT().IsRoleExist(ctx, "dbOnlyRole").Return(false, nil).Once()

	result, err := store.GetUserRoles(ctx, "user1", []string{})

	require.NoError(t, err)
	assert.Equal(t, []string{"dbRole"}, result)
}

// A role whose declarative definition was removed after the assignment was made genuinely confers
// no name, so skipping it remains correct.
func TestCrossStoreUserRoles_RemovedRoleIsSkipped(t *testing.T) {
	ctx := context.Background()
	dbStore := newRoleStoreInterfaceMock(t)
	fileStore := newRoleStoreInterfaceMock(t)
	store := newCompositeRoleStore(fileStore, dbStore)

	dbStore.EXPECT().GetUserRoles(ctx, "user1", []string{}).Return([]string{}, nil).Once()
	fileStore.EXPECT().GetUserRoles(ctx, "user1", []string{}).Return([]string{}, nil).Once()
	dbStore.EXPECT().GetEntityRoleIDs(ctx, "user1", []string{}).Return([]string{"gone"}, nil).Once()
	fileStore.EXPECT().IsRoleExist(ctx, "gone").Return(true, nil).Once()
	fileStore.EXPECT().GetRole(ctx, "gone").Return(RoleWithPermissions{}, ErrRoleNotFound).Once()

	result, err := store.GetUserRoles(ctx, "user1", []string{})

	require.NoError(t, err)
	assert.Empty(t, result)
}

// Corruption is skipped here (unlike crossStoreAllPermissions' fail-closed enumeration): role names
// feed an informational token claim, not an authorization decision, so this mirrors
// crossStoreAuthorizedPermissions in skipping both ErrRoleNotFound and ErrRoleDataCorrupted.
func TestCrossStoreUserRoles_CorruptRoleIsSkipped(t *testing.T) {
	ctx := context.Background()
	dbStore := newRoleStoreInterfaceMock(t)
	fileStore := newRoleStoreInterfaceMock(t)
	store := newCompositeRoleStore(fileStore, dbStore)

	dbStore.EXPECT().GetUserRoles(ctx, "user1", []string{}).Return([]string{}, nil).Once()
	fileStore.EXPECT().GetUserRoles(ctx, "user1", []string{}).Return([]string{}, nil).Once()
	dbStore.EXPECT().GetEntityRoleIDs(ctx, "user1", []string{}).Return([]string{"corrupt"}, nil).Once()
	fileStore.EXPECT().IsRoleExist(ctx, "corrupt").Return(true, nil).Once()
	fileStore.EXPECT().GetRole(ctx, "corrupt").
		Return(RoleWithPermissions{}, ErrRoleDataCorrupted).Once()

	result, err := store.GetUserRoles(ctx, "user1", []string{})

	require.NoError(t, err)
	assert.Empty(t, result)
}

// A load failure other than ErrRoleNotFound/ErrRoleDataCorrupted is an actionable storage/IO error
// and must propagate rather than silently producing an incomplete roles claim.
func TestCrossStoreUserRoles_OtherErrorPropagates(t *testing.T) {
	ctx := context.Background()
	dbStore := newRoleStoreInterfaceMock(t)
	fileStore := newRoleStoreInterfaceMock(t)
	store := newCompositeRoleStore(fileStore, dbStore)

	dbStore.EXPECT().GetUserRoles(ctx, "user1", []string{}).Return([]string{}, nil).Once()
	fileStore.EXPECT().GetUserRoles(ctx, "user1", []string{}).Return([]string{}, nil).Once()
	dbStore.EXPECT().GetEntityRoleIDs(ctx, "user1", []string{}).Return([]string{"broken"}, nil).Once()
	fileStore.EXPECT().IsRoleExist(ctx, "broken").Return(true, nil).Once()
	fileStore.EXPECT().GetRole(ctx, "broken").
		Return(RoleWithPermissions{}, errors.New("disk read failure")).Once()

	result, err := store.GetUserRoles(ctx, "user1", []string{})

	require.Error(t, err)
	assert.Nil(t, result)
}

// A role widened between the pre-transaction check and the write must not be assignable on the
// strength of the earlier result. The second CanGrantMembership call, made inside the transaction,
// is what closes that window.
func TestModifyAssignments_RechecksInsideTransaction(t *testing.T) {
	ctx := context.Background()
	store := newRoleStoreInterfaceMock(t)
	txr := &fakeTransactioner{}

	// First call succeeds (pre-transaction), second refuses (inside the transaction), simulating a
	// concurrent role update that widened the role in between.
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	call := 0
	authz.On("CanGrantMembership", mock.Anything, sysauthz.PrincipalTypeRole, "role1").
		Return(func(context.Context, sysauthz.PrincipalType, string) *tidcommon.ServiceError {
			call++
			if call == 1 {
				return nil
			}
			return &sysauthz.ErrorGrantNotPermitted
		})

	store.EXPECT().IsRoleExist(ctx, "role1").Return(true, nil).Once()
	store.EXPECT().AddAssignments(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	groupSvc := groupmock.NewGroupServiceInterfaceMock(t)
	groupSvc.EXPECT().ValidateGroupIDs(ctx, []string{"grp1"}).Return(nil).Once()

	svc := &roleAssignmentService{
		roleStore:     store,
		groupService:  groupSvc,
		transactioner: txr,
		authzService:  authz,
	}
	svcErr := svc.AddAssignments(ctx, "role1", []RoleAssignment{{ID: "grp1", Type: AssigneeTypeGroup}})

	require.NotNil(t, svcErr)
	assert.Equal(t, sysauthz.ErrorGrantNotPermitted.Code, svcErr.Code,
		"the in-transaction refusal must surface, not be masked as an internal error")
	assert.Equal(t, 2, call, "the guard must run again inside the transaction")
	store.AssertNotCalled(t, "AddAssignments", mock.Anything, mock.Anything, mock.Anything)
}
