// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type CompositeStoreTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestCompositeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeStoreTestSuite))
}

func (s *CompositeStoreTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CompositeStoreTestSuite) TestStore() {
	fileType := paymentType()
	fileStore := newTestFileStore(s.T(), fileType)
	dbStore := newTypeStoreInterfaceMock(s.T())
	composite := newCompositeTypeStore(fileStore, dbStore)
	dbType := flightType()

	dbStore.EXPECT().GetTypeByID(mock.Anything, fileType.ID).
		Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()
	t, err := composite.GetTypeByID(s.ctx, fileType.ID)
	s.Require().NoError(err)
	s.True(t.IsReadOnly, "a declarative type is read from the file store")

	dbStore.EXPECT().GetTypeByID(mock.Anything, dbType.ID).Return(dbType, nil).Once()
	t, err = composite.GetTypeByID(s.ctx, dbType.ID)
	s.Require().NoError(err)
	s.False(t.IsReadOnly)

	dbStore.EXPECT().GetTypeByValue(mock.Anything, "missing").Return(providers.AuthorizationDetailType{},
		errTypeNotFound).Once()
	_, err = composite.GetTypeByValue(s.ctx, "missing")
	s.ErrorIs(err, errTypeNotFound)

	dbStore.EXPECT().ListTypes(mock.Anything).Return([]providers.AuthorizationDetailType{dbType}, nil).Once()
	types, err := composite.ListTypes(s.ctx)
	s.Require().NoError(err)
	s.Equal([]string{testFlightType, testPaymentType}, []string{types[0].Type, types[1].Type})

	dbStore.EXPECT().ListTypes(mock.Anything).Return([]providers.AuthorizationDetailType{dbType, fileType},
		nil).Once()
	types, err = composite.ListTypes(s.ctx)
	s.Require().NoError(err)
	s.Len(types, 2, "a type held by both stores is listed once")
	s.False(types[1].IsReadOnly, "the database copy takes precedence")

	dbStore.EXPECT().ListTypes(mock.Anything).Return([]providers.AuthorizationDetailType{dbType, fileType},
		nil).Twice()
	count, err := composite.CountTypes(s.ctx)
	s.NoError(err)
	s.Equal(2, count, "a type held by both stores is counted once")
	page, err := composite.ListTypePage(s.ctx, 1, 1)
	s.Require().NoError(err)
	s.Equal([]string{testPaymentType}, []string{page[0].Type})

	dbStore.EXPECT().CreateType(mock.Anything, dbType).Return(nil).Once()
	s.NoError(composite.CreateType(s.ctx, dbType))
	dbStore.EXPECT().UpdateType(mock.Anything, dbType, true).Return(nil).Once()
	s.NoError(composite.UpdateType(s.ctx, dbType, true))
	s.ErrorIs(composite.UpdateType(s.ctx, fileType, true), errTypeImmutable)
	dbStore.EXPECT().DeleteType(mock.Anything, dbType.ID).Return(nil).Once()
	s.NoError(composite.DeleteType(s.ctx, dbType.ID))
	s.ErrorIs(composite.DeleteType(s.ctx, fileType.ID), errTypeImmutable)

	isDeclarative, err := composite.IsTypeDeclarative(s.ctx, fileType.ID)
	s.NoError(err)
	s.True(isDeclarative)
}

func (s *CompositeStoreTestSuite) TestDatabaseErrors() {
	dbStore := newTypeStoreInterfaceMock(s.T())
	composite := newCompositeTypeStore(newTestFileStore(s.T()), dbStore)
	dbErr := errors.New("db down")

	dbStore.EXPECT().ListTypes(mock.Anything).Return(nil, dbErr).Once()
	_, err := composite.ListTypes(s.ctx)
	s.ErrorIs(err, dbErr)
	dbStore.EXPECT().ListTypes(mock.Anything).Return(nil, dbErr).Once()
	_, err = composite.ListTypePage(s.ctx, 10, 0)
	s.ErrorIs(err, dbErr)
}

// A merged list over the composite mode limit is refused rather than paged.
func (s *CompositeStoreTestSuite) TestRefusesAListOverTheLimit() {
	dbStore := newTypeStoreInterfaceMock(s.T())
	composite := newCompositeTypeStore(newTestFileStore(s.T(), paymentType()), dbStore)
	dbTypes := make([]providers.AuthorizationDetailType, serverconst.MaxCompositeStoreRecords)
	for i := range dbTypes {
		dbTypes[i] = providers.AuthorizationDetailType{ID: fmt.Sprintf("db-type-%d", i), Type: fmt.Sprintf("t%d", i)}
	}
	dbStore.EXPECT().ListTypes(mock.Anything).Return(dbTypes, nil).Twice()

	_, err := composite.CountTypes(s.ctx)
	s.ErrorIs(err, errResultLimitExceededInCompositeMode)
	_, err = composite.ListTypePage(s.ctx, 10, 0)
	s.ErrorIs(err, errResultLimitExceededInCompositeMode)
}
