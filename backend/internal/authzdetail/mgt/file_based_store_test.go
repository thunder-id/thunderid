// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type FileBasedStoreTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedStoreTestSuite))
}

func (s *FileBasedStoreTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *FileBasedStoreTestSuite) TestStore() {
	second := flightType()
	second.ID, second.Type = "type-3", "a_first"
	fileStore := newTestFileStore(s.T(), flightType(), paymentType(), second)

	t, err := fileStore.GetTypeByID(s.ctx, "type-1")
	s.Require().NoError(err)
	s.True(t.IsReadOnly)
	_, err = fileStore.GetTypeByID(s.ctx, "missing")
	s.ErrorIs(err, errTypeNotFound)

	t, err = fileStore.GetTypeByValue(s.ctx, testPaymentType)
	s.Require().NoError(err)
	s.Equal(paymentType().ID, t.ID)
	_, err = fileStore.GetTypeByValue(s.ctx, "missing")
	s.ErrorIs(err, errTypeNotFound)

	all, err := fileStore.ListTypes(s.ctx)
	s.Require().NoError(err)
	s.Len(all, 3)
	s.Equal("a_first", all[0].Type, "types are ordered by type value")

	isDeclarative, err := fileStore.IsTypeDeclarative(s.ctx, "type-1")
	s.NoError(err)
	s.True(isDeclarative)
	isDeclarative, _ = fileStore.IsTypeDeclarative(s.ctx, "missing")
	s.False(isDeclarative)

	s.ErrorIs(fileStore.CreateType(s.ctx, flightType()), errTypeImmutable)
	s.ErrorIs(fileStore.UpdateType(s.ctx, flightType(), true), errTypeImmutable)
	s.ErrorIs(fileStore.DeleteType(s.ctx, "type-1"), errTypeImmutable)
	s.ErrorIs(fileStore.Create("bad", "not a type"), errTypeDataCorrupted)

	s.Require().NoError(fileStore.GenericFileBasedStore.Create("corrupt", "not a type"))
	_, err = fileStore.GetTypeByID(s.ctx, "corrupt")
	s.ErrorIs(err, errTypeDataCorrupted)
	all, err = fileStore.ListTypes(s.ctx)
	s.NoError(err)
	s.Len(all, 3, "a malformed entry is skipped")
	count, err := fileStore.CountTypes(s.ctx)
	s.NoError(err)
	s.Equal(3, count)
	page, err := fileStore.ListTypePage(s.ctx, 2, 2)
	s.NoError(err)
	s.Equal([]providers.AuthorizationDetailType{all[2]}, page)
	page, err = fileStore.ListTypePage(s.ctx, 2, 5)
	s.NoError(err)
	s.Empty(page, "an offset past the end is an empty page")
}

// newTestFileStore creates a file-based store with its own storage, holding the given types.
func newTestFileStore(t *testing.T, types ...providers.AuthorizationDetailType) *typeFileBasedStore {
	t.Helper()
	fileStore := &typeFileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStoreForTest(
			entity.KeyTypeAuthorizationDetailType),
	}
	for i := range types {
		typ := types[i]
		require.NoError(t, fileStore.Create(typ.ID, &typ))
	}
	return fileStore
}
