// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/managedresourcemock"
)

// What a control plane deployed is changed only by a deploy from it: anything else is refused, the managed
// import is let through, and a failed read of the marks refuses the change.
func TestAManagedResourceIsChangedOnlyByTheManagedImport(t *testing.T) {
	ctx := context.Background()
	store := managedresourcemock.NewStoreInterfaceMock(t)
	svc := &userService{managed: store, logger: log.GetLogger()}

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeUser).Return(true, nil).Once()
	svcErr := svc.refuseManaged(ctx, "id-1", log.GetLogger())
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorManagedUser.Code, svcErr.Code)

	importCtx := managedresource.WithManagedImport(ctx)
	assert.Nil(t, svc.refuseManaged(importCtx, "id-1", log.GetLogger()))

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeUser).
		Return(false, errors.New("marker store unavailable")).Once()
	svcErr = svc.refuseManaged(ctx, "id-1", log.GetLogger())
	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeUser).Return(false, nil).Once()
	assert.Nil(t, svc.refuseManaged(ctx, "id-1", log.GetLogger()))
}

// The listed users a control plane deployed read as read only; a failed read serves them without the flag.
func TestDeployedUsersReadAsReadOnly(t *testing.T) {
	ctx := context.Background()
	store := managedresourcemock.NewStoreInterfaceMock(t)
	svc := &userService{managed: store}
	users := []providers.User{{ID: "u-1"}, {ID: "u-2"}}
	store.EXPECT().ManagedIDs(ctx, []string{"u-1", "u-2"}, managedresource.TypeUser).
		Return(map[string]bool{"u-1": true}, nil).Once()
	svc.markManaged(ctx, users, log.GetLogger())
	assert.True(t, users[0].IsReadOnly)
	assert.False(t, users[1].IsReadOnly)

	store.EXPECT().ManagedIDs(ctx, mock.Anything, managedresource.TypeUser).
		Return(nil, errors.New("marker store unavailable")).Once()
	unflagged := []providers.User{{ID: "u-2"}}
	svc.markManaged(ctx, unflagged, log.GetLogger())
	assert.False(t, unflagged[0].IsReadOnly)
}
