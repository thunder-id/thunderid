// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package managedresource_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/tests/mocks/managedresourcemock"
)

type item struct {
	id       string
	readOnly bool
}

func TestTheManagedImportIsRecognisedByItsContext(t *testing.T) {
	ctx := context.Background()
	assert.False(t, managedresource.IsManagedImport(ctx))
	assert.True(t, managedresource.IsManagedImport(managedresource.WithManagedImport(ctx)))
}

// A nil store, which every service has where nothing is deployed from a control plane, refuses nothing
// and marks nothing.
func TestANilStoreAnswersThatNothingIsManaged(t *testing.T) {
	ctx := context.Background()

	readOnly, err := managedresource.IsReadOnly(ctx, nil, "g-1", managedresource.TypeGroup)
	assert.NoError(t, err)
	assert.False(t, readOnly)

	deployed, err := managedresource.IsDeployed(ctx, nil, "g-1", managedresource.TypeGroup)
	assert.NoError(t, err)
	assert.False(t, deployed)

	items := []item{{id: "g-1"}}
	assert.NoError(t, managedresource.MarkDeployed(ctx, nil, items, idOf, markReadOnly, managedresource.TypeGroup))
	assert.False(t, items[0].readOnly)
}

func TestAManagedResourceIsReadOnlyExceptToTheManagedImport(t *testing.T) {
	ctx := context.Background()
	store := managedresourcemock.NewStoreInterfaceMock(t)
	store.EXPECT().IsManaged(ctx, "g-1", managedresource.TypeGroup).Return(true, nil).Once()

	readOnly, err := managedresource.IsReadOnly(ctx, store, "g-1", managedresource.TypeGroup)
	assert.NoError(t, err)
	assert.True(t, readOnly)

	readOnly, err = managedresource.IsReadOnly(managedresource.WithManagedImport(ctx), store, "g-1",
		managedresource.TypeGroup)
	assert.NoError(t, err)
	assert.False(t, readOnly, "the managed import is never refused, so the store is not asked")
}

func TestADeployedResourceIsShownReadOnlyToEveryReader(t *testing.T) {
	ctx := managedresource.WithManagedImport(context.Background())
	store := managedresourcemock.NewStoreInterfaceMock(t)
	store.EXPECT().IsManaged(ctx, "g-1", managedresource.TypeGroup).Return(true, nil).Once()

	deployed, err := managedresource.IsDeployed(ctx, store, "g-1", managedresource.TypeGroup)
	assert.NoError(t, err)
	assert.True(t, deployed)
}

func TestMarkDeployedMarksOnlyTheDeployedItemsInOneRead(t *testing.T) {
	ctx := context.Background()
	store := managedresourcemock.NewStoreInterfaceMock(t)
	store.EXPECT().ManagedIDs(ctx, []string{"a", "b"}, managedresource.TypeRole).
		Return(map[string]bool{"b": true}, nil).Once()

	items := []item{{id: "a"}, {id: "b"}}
	assert.NoError(t, managedresource.MarkDeployed(ctx, store, items, idOf, markReadOnly, managedresource.TypeRole))
	assert.False(t, items[0].readOnly)
	assert.True(t, items[1].readOnly)

	assert.NoError(t, managedresource.MarkDeployed(ctx, store, []item{}, idOf, markReadOnly,
		managedresource.TypeRole), "an empty list reads nothing")
}

func TestMarkDeployedReportsAFailedRead(t *testing.T) {
	ctx := context.Background()
	failure := errors.New("database unavailable")
	store := managedresourcemock.NewStoreInterfaceMock(t)
	store.EXPECT().ManagedIDs(ctx, mock.Anything, managedresource.TypeRole).Return(nil, failure).Once()

	items := []item{{id: "a"}}
	assert.ErrorIs(t, managedresource.MarkDeployed(ctx, store, items, idOf, markReadOnly,
		managedresource.TypeRole), failure)
	assert.False(t, items[0].readOnly)
}

func idOf(i *item) string { return i.id }

func markReadOnly(i *item) { i.readOnly = true }
