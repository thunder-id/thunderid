// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

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
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

// What a control plane deployed is changed only by a deploy from it: anything else is refused, the managed
// import is let through, and a failed read of the marks refuses the change.
func TestAManagedResourceIsChangedOnlyByTheManagedImport(t *testing.T) {
	ctx := context.Background()
	store := managedresourcemock.NewStoreInterfaceMock(t)
	svc := &flowMgtService{managed: store, logger: log.GetLogger()}

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeFlow).Return(true, nil).Once()
	svcErr := svc.refuseManaged(ctx, "id-1")
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorManagedFlow.Code, svcErr.Code)

	importCtx := managedresource.WithManagedImport(ctx)
	assert.Nil(t, svc.refuseManaged(importCtx, "id-1"))

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeFlow).
		Return(false, errors.New("marker store unavailable")).Once()
	svcErr = svc.refuseManaged(ctx, "id-1")
	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeFlow).Return(false, nil).Once()
	assert.Nil(t, svc.refuseManaged(ctx, "id-1"))
}

// The listed flows a control plane deployed read as read only; a failed read serves them without the flag.
func TestDeployedFlowsReadAsReadOnly(t *testing.T) {
	ctx := context.Background()
	store := managedresourcemock.NewStoreInterfaceMock(t)
	svc := &flowMgtService{managed: store, logger: log.GetLogger()}
	flows := []BasicFlowDefinition{{ID: "f-1"}, {ID: "f-2"}}
	store.EXPECT().ManagedIDs(ctx, []string{"f-1", "f-2"}, managedresource.TypeFlow).
		Return(map[string]bool{"f-2": true}, nil).Once()
	svc.markManaged(ctx, flows)
	assert.False(t, flows[0].IsReadOnly)
	assert.True(t, flows[1].IsReadOnly)

	store.EXPECT().ManagedIDs(ctx, []string{"f-1"}, managedresource.TypeFlow).
		Return(nil, errors.New("marker store unavailable")).Once()
	unflagged := []BasicFlowDefinition{{ID: "f-1"}}
	svc.markManaged(ctx, unflagged)
	assert.False(t, unflagged[0].IsReadOnly)
}

// Reading a flow a control plane deployed reports it read only without marking the stored flow, which the
// cache serves to the next read: a later update from the managed import reads that flow, and must not find
// it read only.
func TestReadingADeployedFlowLeavesTheStoredFlowAsItIs(t *testing.T) {
	ctx := context.Background()
	stored := &providers.CompleteFlowDefinition{ID: "f-1", Handle: "login"}
	flowStore := newFlowStoreInterfaceMock(t)
	flowStore.EXPECT().GetFlowByID(mock.Anything, "f-1").Return(stored, nil)
	managed := managedresourcemock.NewStoreInterfaceMock(t)
	managed.EXPECT().IsManaged(mock.Anything, "f-1", managedresource.TypeFlow).Return(true, nil)
	svc := &flowMgtService{store: flowStore, managed: managed, logger: log.GetLogger()}

	read, svcErr := svc.getFlow(ctx, "f-1")

	require.Nil(t, svcErr)
	assert.True(t, read.IsReadOnly)
	assert.False(t, stored.IsReadOnly)

	// The managed import's update then reads the stored flow, and may change it.
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authz.EXPECT().IsActionAllowed(mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	svc.authzService = authz
	again, err := flowStore.GetFlowByID(ctx, "f-1")
	require.NoError(t, err)
	assert.Nil(t, svc.checkFlowWritable(managedresource.WithManagedImport(ctx), again))
}
