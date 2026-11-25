// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

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
	svc := &resourceService{managed: store, logger: *log.GetLogger()}

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeResourceServer).Return(true, nil).Once()
	svcErr := svc.refuseManaged(ctx, "id-1")
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorManagedResourceServer.Code, svcErr.Code)

	importCtx := managedresource.WithManagedImport(ctx)
	assert.Nil(t, svc.refuseManaged(importCtx, "id-1"))

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeResourceServer).
		Return(false, errors.New("marker store unavailable")).Once()
	svcErr = svc.refuseManaged(ctx, "id-1")
	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeResourceServer).Return(false, nil).Once()
	assert.Nil(t, svc.refuseManaged(ctx, "id-1"))
}

// A deployed resource server reads as read only, alone and in a list; a failed read serves it without the
// flag.
func TestADeployedResourceServerReadsAsReadOnly(t *testing.T) {
	ctx := context.Background()
	store := managedresourcemock.NewStoreInterfaceMock(t)
	svc := &resourceService{managed: store, logger: *log.GetLogger()}

	store.EXPECT().IsManaged(ctx, "rs-1", managedresource.TypeResourceServer).Return(true, nil).Once()
	server, svcErr := svc.withManaged(ctx, &providers.ResourceServer{ID: "rs-1"})
	require.Nil(t, svcErr)
	assert.True(t, server.IsReadOnly)
	store.EXPECT().IsManaged(ctx, "rs-1", managedresource.TypeResourceServer).
		Return(false, errors.New("marker store unavailable")).Once()
	server, svcErr = svc.withManaged(ctx, &providers.ResourceServer{ID: "rs-1"})
	require.Nil(t, svcErr)
	assert.False(t, server.IsReadOnly)

	servers := []providers.ResourceServer{{ID: "rs-1"}, {ID: "rs-2"}}
	store.EXPECT().ManagedIDs(ctx, []string{"rs-1", "rs-2"}, managedresource.TypeResourceServer).
		Return(map[string]bool{"rs-2": true}, nil).Once()
	svc.markManaged(ctx, servers)
	assert.False(t, servers[0].IsReadOnly)
	assert.True(t, servers[1].IsReadOnly)
	store.EXPECT().ManagedIDs(ctx, mock.Anything, managedresource.TypeResourceServer).
		Return(nil, errors.New("marker store unavailable")).Once()
	svc.markManaged(ctx, []providers.ResourceServer{{ID: "rs-1"}})
}
