// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/managedresourcemock"
)

var errMarkers = errors.New("marker store unavailable")

func newManagedGroupFixture(t *testing.T) (*groupChangeFixture, *managedresourcemock.StoreInterfaceMock) {
	f := newGroupChangeFixture(t)
	store := managedresourcemock.NewStoreInterfaceMock(t)
	f.service.managed = store
	return f, store
}

// A group a control plane deployed is changed or removed only by a deploy from it.
func TestAManagedGroupIsNotUpdatedOrDeleted(t *testing.T) {
	ctx := context.Background()
	f, store := newManagedGroupFixture(t)
	f.store.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil)
	store.EXPECT().IsManaged(ctx, "grp-001", managedresource.TypeGroup).Return(true, nil)

	_, svcErr := f.service.UpdateGroup(ctx, "grp-001", UpdateGroupRequest{Name: "Ops", OUID: testOUID1})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorManagedGroup.Code, svcErr.Code)

	svcErr = f.service.ValidateDeleteGroup(ctx, "grp-001")
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorManagedGroup.Code, svcErr.Code)
}

// The managed import is not refused, and a failed read of the marks refuses the change.
func TestTheManagedGroupCheckLetsTheManagedImportThroughAndFailsClosed(t *testing.T) {
	f, store := newManagedGroupFixture(t)
	logger := log.GetLogger()

	assert.Nil(t, f.service.refuseManagedGroup(
		managedresource.WithManagedImport(context.Background()), "grp-001", logger))

	store.EXPECT().IsManaged(mock.Anything, "grp-001", managedresource.TypeGroup).Return(false, errMarkers).Once()
	svcErr := f.service.refuseManagedGroup(context.Background(), "grp-001", logger)
	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

// A member a control plane deployed is removed only by a deploy; one added here stays removable.
func TestAManagedMemberIsNotRemoved(t *testing.T) {
	ctx := context.Background()
	f, store := newManagedGroupFixture(t)
	store.EXPECT().PartKeys(mock.Anything, managedresource.TypeGroup, "grp-001", managedresource.PartMember).
		Return([]string{"user/usr-001"}, nil)
	f.mayUpdateGroup()
	f.entities.On("GetEntitiesByIDs", mock.Anything, []string{"usr-001"}).
		Return([]providers.Entity{{ID: "usr-001", Category: providers.EntityCategoryUser}}, nil)

	_, svcErr := f.service.RemoveGroupMembers(ctx, "grp-001", []Member{{ID: "usr-001", Type: MemberTypeUser}})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorManagedGroupMember.Code, svcErr.Code)

	svcErr = f.service.ValidateRemoveGroupMembers(ctx, "grp-001", []Member{{ID: "usr-001", Type: MemberTypeUser}})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorManagedGroupMember.Code, svcErr.Code)

	assert.Nil(t, f.service.refuseManagedMembers(ctx, "grp-001",
		[]Member{{ID: "usr-002", Type: MemberTypeUser}}, log.GetLogger()))
	assert.Nil(t, f.service.refuseManagedMembers(managedresource.WithManagedImport(ctx), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}}, log.GetLogger()))
}

func TestAFailedReadOfTheDeployedMembersRefusesTheRemoval(t *testing.T) {
	f, store := newManagedGroupFixture(t)
	store.EXPECT().PartKeys(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errMarkers)

	svcErr := f.service.refuseManagedMembers(context.Background(), "grp-001",
		[]Member{{ID: "usr-001", Type: MemberTypeUser}}, log.GetLogger())
	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)
}

// A deployed group reads as read only, alone and in a list; a failed read of the marks serves it without
// the flag.
func TestADeployedGroupReadsAsReadOnly(t *testing.T) {
	ctx := context.Background()
	f, store := newManagedGroupFixture(t)
	f.store.On("GetGroup", mock.Anything, "grp-001").Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil)
	f.authz.On("IsActionAllowed", mock.Anything, security.ActionReadGroup, mock.Anything).
		Return(true, (*tidcommon.ServiceError)(nil))
	store.EXPECT().IsManaged(ctx, "grp-001", managedresource.TypeGroup).Return(true, nil).Once()

	group, svcErr := f.service.GetGroup(ctx, "grp-001", false)
	require.Nil(t, svcErr)
	assert.True(t, group.IsReadOnly)

	store.EXPECT().IsManaged(ctx, "grp-001", managedresource.TypeGroup).Return(false, errMarkers).Once()
	group, svcErr = f.service.GetGroup(ctx, "grp-001", false)
	require.Nil(t, svcErr)
	assert.False(t, group.IsReadOnly)

	groups := []GroupBasic{{ID: "grp-001"}, {ID: "grp-002"}}
	store.EXPECT().ManagedIDs(ctx, []string{"grp-001", "grp-002"}, managedresource.TypeGroup).
		Return(map[string]bool{"grp-002": true}, nil).Once()
	f.service.markManagedReadOnly(ctx, groups, log.GetLogger())
	assert.False(t, groups[0].IsReadOnly)
	assert.True(t, groups[1].IsReadOnly)

	store.EXPECT().ManagedIDs(ctx, mock.Anything, managedresource.TypeGroup).Return(nil, errMarkers).Once()
	unflagged := []GroupBasic{{ID: "grp-001"}}
	f.service.markManagedReadOnly(ctx, unflagged, log.GetLogger())
	assert.False(t, unflagged[0].IsReadOnly)
}

func TestMemberKeyNamesAMemberByTypeAndID(t *testing.T) {
	assert.Equal(t, "group/grp-002", MemberKey(Member{ID: "grp-002", Type: MemberTypeGroup}))
}
