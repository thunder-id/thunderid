// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/managedresourcemock"
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
