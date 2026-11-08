// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/managedresourcemock"
)

var errMarkers = errors.New("marker store unavailable")

func initMutableRoleStore(t *testing.T) {
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime(t.TempDir(), &config.Config{}))
	t.Cleanup(config.ResetServerRuntime)
}

func managedRoleService(t *testing.T) (*roleService, *managedresourcemock.StoreInterfaceMock) {
	initMutableRoleStore(t)
	store := managedresourcemock.NewStoreInterfaceMock(t)
	svc := &roleService{roleStore: newRoleStoreInterfaceMock(t), logger: log.GetLogger(), managed: store}
	return svc, store
}

// A role a control plane deployed is changed only by a deploy from it, whatever the store mode.
func TestAManagedRoleIsNotChanged(t *testing.T) {
	svc, store := managedRoleService(t)
	ctx := context.Background()

	store.EXPECT().IsManaged(ctx, "role-1", managedresource.TypeRole).Return(true, nil).Once()
	svcErr := svc.validateRoleMutable(ctx, "role-1")
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorManagedRole.Code, svcErr.Code)

	assert.Nil(t, svc.validateRoleMutable(managedresource.WithManagedImport(ctx), "role-1"))

	store.EXPECT().IsManaged(ctx, "role-1", managedresource.TypeRole).Return(false, errMarkers).Once()
	svcErr = svc.validateRoleMutable(ctx, "role-1")
	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

func TestSetManagedStoreGivesTheServiceTheMarks(t *testing.T) {
	svc := &roleService{}
	store := managedresourcemock.NewStoreInterfaceMock(t)
	setManagedStore(svc, store)
	assert.Same(t, store, svc.managed)
}

// A deployed role reads as read only, alone and in a list; a failed read of the marks serves it without
// the flag.
func TestADeployedRoleReadsAsReadOnly(t *testing.T) {
	svc, store := managedRoleService(t)
	ctx := context.Background()

	store.EXPECT().IsManaged(ctx, "role-1", managedresource.TypeRole).Return(true, nil).Once()
	assert.True(t, svc.isReadOnlyRole(ctx, "role-1", log.GetLogger()))
	store.EXPECT().IsManaged(ctx, "role-1", managedresource.TypeRole).Return(false, errMarkers).Once()
	assert.False(t, svc.isReadOnlyRole(ctx, "role-1", log.GetLogger()))

	roles := []Role{{ID: "role-1"}, {ID: "role-2"}}
	store.EXPECT().ManagedIDs(ctx, []string{"role-1", "role-2"}, managedresource.TypeRole).
		Return(map[string]bool{"role-1": true}, nil).Once()
	svc.markManagedRoles(ctx, roles)
	assert.True(t, roles[0].IsReadOnly)
	assert.False(t, roles[1].IsReadOnly)

	store.EXPECT().ManagedIDs(ctx, mock.Anything, managedresource.TypeRole).Return(nil, errMarkers).Once()
	unflagged := []Role{{ID: "role-2"}}
	svc.markManagedRoles(ctx, unflagged)
	assert.False(t, unflagged[0].IsReadOnly)
}

func TestAnAssignmentKeyReadsBack(t *testing.T) {
	assignment := RoleAssignment{OUID: "ou-1", Type: AssigneeTypeGroup, ID: "grp-1"}
	key := AssignmentKey(assignment)
	assert.Equal(t, "ou-1/group/grp-1", key)

	read, ok := ParseAssignmentKey(key)
	require.True(t, ok)
	assert.Equal(t, assignment, read)

	_, ok = ParseAssignmentKey("not-a-key")
	assert.False(t, ok)
}

func managedAssignmentService(t *testing.T) (*roleAssignmentService, *managedresourcemock.StoreInterfaceMock) {
	store := managedresourcemock.NewStoreInterfaceMock(t)
	return &roleAssignmentService{logger: log.GetLogger(), managed: store}, store
}

// An assignment a control plane deployed is removed only by a deploy; one made here stays removable. A
// removal by assignee id alone matches the deployed assignment of any type in the unit.
func TestAManagedAssignmentIsNotRemoved(t *testing.T) {
	svc, store := managedAssignmentService(t)
	ctx := context.Background()
	store.EXPECT().PartKeys(mock.Anything, managedresource.TypeRole, "role-1", managedresource.PartAssignment).
		Return([]string{"ou-1/user/usr-1"}, nil)

	svcErr := svc.refuseManagedAssignments(ctx, "role-1", []RoleAssignment{{OUID: "ou-1", ID: "usr-1"}},
		log.GetLogger())
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorManagedRoleAssignment.Code, svcErr.Code)

	assert.Nil(t, svc.refuseManagedAssignments(ctx, "role-1", []RoleAssignment{{OUID: "ou-2", ID: "usr-1"}},
		log.GetLogger()), "the same assignee in another unit is not the deployed assignment")
	assert.Nil(t, svc.refuseManagedAssignments(managedresource.WithManagedImport(ctx), "role-1",
		[]RoleAssignment{{OUID: "ou-1", ID: "usr-1"}}, log.GetLogger()))
}

func TestAFailedReadOfTheDeployedAssignmentsRefusesTheRemoval(t *testing.T) {
	svc, store := managedAssignmentService(t)
	store.EXPECT().PartKeys(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errMarkers)

	svcErr := svc.refuseManagedAssignments(context.Background(), "role-1",
		[]RoleAssignment{{OUID: "ou-1", ID: "usr-1"}}, log.GetLogger())
	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

// The listed assignments a control plane deployed read as read only.
func TestDeployedAssignmentsReadAsReadOnly(t *testing.T) {
	svc, store := managedAssignmentService(t)
	ctx := context.Background()
	store.EXPECT().PartKeys(ctx, managedresource.TypeRole, "role-1", managedresource.PartAssignment).
		Return([]string{"ou-1/user/usr-1"}, nil).Once()

	listed := []RoleAssignmentWithDisplay{{ID: "usr-1", OUID: "ou-1"}, {ID: "usr-2", OUID: "ou-1"}}
	svc.markManagedAssignments(ctx, "role-1", listed)
	assert.True(t, listed[0].IsManaged)
	assert.False(t, listed[1].IsManaged)

	store.EXPECT().PartKeys(ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil, errMarkers).Once()
	unflagged := []RoleAssignmentWithDisplay{{ID: "usr-1", OUID: "ou-1"}}
	svc.markManagedAssignments(ctx, "role-1", unflagged)
	assert.False(t, unflagged[0].IsManaged)
}

// Each assignment is placed in the unit it names, or in its assignee's own when it names none.
func TestResolveAssignmentOUsPlacesEachAssignment(t *testing.T) {
	entities := entitymock.NewEntityServiceInterfaceMock(t)
	groups := groupmock.NewGroupServiceInterfaceMock(t)
	svc := &roleAssignmentService{entityService: entities, groupService: groups, logger: log.GetLogger()}
	entities.On("GetEntitiesByIDs", mock.Anything, []string{"usr-1"}).
		Return([]providers.Entity{{ID: "usr-1", OUID: "ou-user"}}, nil)
	groups.On("GetGroupsByIDs", mock.Anything, []string{"grp-1"}).
		Return(map[string]*group.Group{"grp-1": {ID: "grp-1", OUID: "ou-group"}},
			(*tidcommon.ServiceError)(nil))

	resolved, svcErr := svc.ResolveAssignmentOUs(context.Background(), []RoleAssignment{
		{ID: "usr-1", Type: AssigneeTypeUser},
		{ID: "grp-1", Type: AssigneeTypeGroup},
		{ID: "usr-2", Type: AssigneeTypeUser, OUID: "ou-named"},
	})

	require.Nil(t, svcErr)
	assert.ElementsMatch(t, []RoleAssignment{
		{ID: "usr-1", Type: AssigneeTypeUser, OUID: "ou-user"},
		{ID: "grp-1", Type: AssigneeTypeGroup, OUID: "ou-group"},
		{ID: "usr-2", Type: AssigneeTypeUser, OUID: "ou-named"},
	}, resolved)
}

func TestResolveAssignmentOUsRefusesAnUnknownAssignee(t *testing.T) {
	entities := entitymock.NewEntityServiceInterfaceMock(t)
	svc := &roleAssignmentService{entityService: entities, logger: log.GetLogger()}
	entities.On("GetEntitiesByIDs", mock.Anything, []string{"usr-gone"}).Return([]providers.Entity{}, nil)

	_, svcErr := svc.ResolveAssignmentOUs(context.Background(),
		[]RoleAssignment{{ID: "usr-gone", Type: AssigneeTypeUser}})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorInvalidAssignmentID.Code, svcErr.Code)
}
