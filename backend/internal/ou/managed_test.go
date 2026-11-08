// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/managedresource"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/managedresourcemock"
)

// What a control plane deployed is changed only by a deploy from it: anything else is refused, the managed
// import is let through, and a failed read of the marks refuses the change.
func TestAManagedResourceIsChangedOnlyByTheManagedImport(t *testing.T) {
	ctx := context.Background()
	store := managedresourcemock.NewStoreInterfaceMock(t)
	svc := &organizationUnitService{managed: store}
	svc.ouStore = newOrganizationUnitStoreInterfaceMock(t)
	svc.ouStore.(*organizationUnitStoreInterfaceMock).
		On("IsOrganizationUnitDeclarative", mock.Anything, "id-1").Return(false)

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeOrganizationUnit).Return(true, nil).Once()
	svcErr := svc.refuseChange(ctx, "id-1")
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorManagedOrganizationUnit.Code, svcErr.Code)

	importCtx := managedresource.WithManagedImport(ctx)
	assert.Nil(t, svc.refuseChange(importCtx, "id-1"))

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeOrganizationUnit).
		Return(false, errors.New("marker store unavailable")).Once()
	svcErr = svc.refuseChange(ctx, "id-1")
	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeOrganizationUnit).Return(false, nil).Once()
	assert.Nil(t, svc.refuseChange(ctx, "id-1"))
}

// A deployed unit reads as read only, alone and in a list; a failed read serves it without the flag.
func TestADeployedUnitReadsAsReadOnly(t *testing.T) {
	ctx := context.Background()
	store := managedresourcemock.NewStoreInterfaceMock(t)
	svc := &organizationUnitService{managed: store}

	store.EXPECT().IsManaged(ctx, "ou-1", managedresource.TypeOrganizationUnit).Return(true, nil).Once()
	unit, svcErr := svc.withManaged(ctx, OrganizationUnit{ID: "ou-1"})
	require.Nil(t, svcErr)
	assert.True(t, unit.IsReadOnly)
	store.EXPECT().IsManaged(ctx, "ou-1", managedresource.TypeOrganizationUnit).
		Return(false, errors.New("marker store unavailable")).Once()
	unit, svcErr = svc.withManaged(ctx, OrganizationUnit{ID: "ou-1"})
	require.Nil(t, svcErr)
	assert.False(t, unit.IsReadOnly)

	units := []OrganizationUnitBasic{{ID: "ou-1"}, {ID: "ou-2"}}
	store.EXPECT().ManagedIDs(ctx, []string{"ou-1", "ou-2"}, managedresource.TypeOrganizationUnit).
		Return(map[string]bool{"ou-1": true}, nil).Once()
	svc.markManaged(ctx, units)
	assert.True(t, units[0].IsReadOnly)
	assert.False(t, units[1].IsReadOnly)
	store.EXPECT().ManagedIDs(ctx, mock.Anything, managedresource.TypeOrganizationUnit).
		Return(nil, errors.New("marker store unavailable")).Once()
	svc.markManaged(ctx, []OrganizationUnitBasic{{ID: "ou-2"}})
}
