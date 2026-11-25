// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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
	svc := &entityTypeService{managed: store}

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeAgentType).Return(true, nil).Once()
	svcErr := svc.refuseManaged(ctx, TypeCategoryAgent, "id-1")
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorManagedEntityType.Code, svcErr.Code)

	importCtx := managedresource.WithManagedImport(ctx)
	assert.Nil(t, svc.refuseManaged(importCtx, TypeCategoryAgent, "id-1"))

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeAgentType).
		Return(false, errors.New("marker store unavailable")).Once()
	svcErr = svc.refuseManaged(ctx, TypeCategoryAgent, "id-1")
	require.NotNil(t, svcErr)
	assert.Equal(t, tidcommon.InternalServerError.Code, svcErr.Code)

	store.EXPECT().IsManaged(ctx, "id-1", managedresource.TypeAgentType).Return(false, nil).Once()
	assert.Nil(t, svc.refuseManaged(ctx, TypeCategoryAgent, "id-1"))
}

// A deployed type reads as read only, and a user type and an agent type are marked under their own types.
func TestADeployedTypeReadsAsReadOnly(t *testing.T) {
	ctx := context.Background()
	store := managedresourcemock.NewStoreInterfaceMock(t)
	svc := &entityTypeService{managed: store}
	assert.Equal(t, managedresource.TypeUserType, managedType(TypeCategoryUser))

	store.EXPECT().IsManaged(ctx, "ut-1", managedresource.TypeUserType).Return(true, nil).Once()
	deployed := &EntityType{ID: "ut-1"}
	svc.withManaged(ctx, TypeCategoryUser, deployed)
	assert.True(t, deployed.IsReadOnly)

	store.EXPECT().IsManaged(ctx, "ut-1", managedresource.TypeUserType).
		Return(false, errors.New("marker store unavailable")).Once()
	unflagged := &EntityType{ID: "ut-1"}
	svc.withManaged(ctx, TypeCategoryUser, unflagged)
	assert.False(t, unflagged.IsReadOnly)

	other := &entityTypeService{}
	setManagedStore(other, store)
	assert.Same(t, store, other.managed)
}
