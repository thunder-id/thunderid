// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package idp

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

func (s *IDPServiceTestSuite) newIDPForOU(ouID string) *providers.IDPDTO {
	return &providers.IDPDTO{Name: "Test IDP", Type: providers.IDPTypeOIDC, Properties: createOIDCProperties(),
		OUID: ouID}
}

// callerOUContext is a context for a caller that belongs to an organization unit, as every request that creates an
// identity provider does.
func callerOUContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("user-1", "caller-ou", "", nil, nil))
}

func (s *IDPServiceTestSuite) expectCreate(ouID string) {
	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Test IDP").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	s.mockStore.On("CreateIdentityProvider", mock.Anything, mock.MatchedBy(func(dto providers.IDPDTO) bool {
		return dto.OUID == ouID
	})).Return(nil)
}

// TestCreateIdentityProvider_DefaultsToCallerOU gives an identity provider created without an organization unit the
// caller's.
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_DefaultsToCallerOU() {
	s.expectCreate("caller-ou")
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("user-1", "caller-ou", "", nil, nil))

	result, err := s.idpService.CreateIdentityProvider(ctx, s.newIDPForOU(""))

	s.Nil(err)
	s.Equal("caller-ou", result.OUID)
}

// TestCreateIdentityProvider_NoCallerOU refuses an identity provider that names no organization unit when the caller
// has none to default to, without creating anything.
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_NoCallerOU() {
	result, err := s.idpService.CreateIdentityProvider(context.Background(), s.newIDPForOU(""))

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(ErrorIDPOURequired.Code, err.Code)
	s.mockStore.AssertNotCalled(s.T(), "CreateIdentityProvider", mock.Anything, mock.Anything)
}

// TestCreateIdentityProvider_GivenOU records the organization unit the request names, whoever the caller is.
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_GivenOU() {
	ouSvc := newOuProviderMock(s.T())
	ouSvc.On("GetOrganizationUnit", mock.Anything, "ou-1").Return(ou.OrganizationUnit{ID: "ou-1"}, nil)
	s.idpService.ouService = ouSvc
	s.expectCreate("ou-1")
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("user-1", "caller-ou", "", nil, nil))

	result, err := s.idpService.CreateIdentityProvider(ctx, s.newIDPForOU("ou-1"))

	s.Nil(err)
	s.Equal("ou-1", result.OUID)
}

// TestCreateIdentityProvider_UnknownOU refuses an organization unit that does not exist.
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_UnknownOU() {
	ouSvc := newOuProviderMock(s.T())
	ouSvc.On("GetOrganizationUnit", mock.Anything, "missing").
		Return(ou.OrganizationUnit{}, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})
	s.idpService.ouService = ouSvc

	result, err := s.idpService.CreateIdentityProvider(context.Background(), s.newIDPForOU("missing"))

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidIDPOU.Code, err.Code)
}

// TestCreateIdentityProvider_OULookupFails reports a failure to look the organization unit up as a server error.
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_OULookupFails() {
	ouSvc := newOuProviderMock(s.T())
	ouSvc.On("GetOrganizationUnit", mock.Anything, "ou-1").
		Return(ou.OrganizationUnit{}, &tidcommon.ServiceError{Type: tidcommon.ServerErrorType})
	s.idpService.ouService = ouSvc

	result, err := s.idpService.CreateIdentityProvider(context.Background(), s.newIDPForOU("ou-1"))

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *IDPServiceTestSuite) expectUpdate(existingOU string, updatedOU string) {
	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(&providers.IDPDTO{
		ID: "idp-123", Name: "Test IDP", Type: providers.IDPTypeOIDC, Properties: createOIDCProperties(),
		OUID: existingOU}, nil)
	if updatedOU != "" {
		s.mockStore.On("UpdateIdentityProvider", mock.Anything, mock.MatchedBy(func(dto *providers.IDPDTO) bool {
			return dto.OUID == updatedOU
		})).Return(nil)
	}
}

// TestUpdateIdentityProvider_KeepsOU keeps the organization unit the identity provider has when the update omits it.
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_KeepsOU() {
	s.expectUpdate("ou-1", "ou-1")

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", s.newIDPForOU(""))

	s.Nil(err)
	s.Equal("ou-1", result.OUID)
}

// TestUpdateIdentityProvider_SameOU accepts an update that names the organization unit the identity provider has.
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_SameOU() {
	s.expectUpdate("ou-1", "ou-1")

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", s.newIDPForOU("ou-1"))

	s.Nil(err)
	s.Equal("ou-1", result.OUID)
}

// TestUpdateIdentityProvider_ChangeOUNotAllowed refuses an update that names another organization unit.
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_ChangeOUNotAllowed() {
	s.expectUpdate("ou-1", "")

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", s.newIDPForOU("ou-2"))

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(ErrorIDPOUUpdateNotAllowed.Code, err.Code)
}

// noOUStore is a store whose table has no OU_ID column.
type noOUStore struct{ *idpStoreInterfaceMock }

func (noOUStore) recordsOU() bool { return false }

// TestCreateIdentityProvider_StoreWithoutOUColumn ignores the organization unit when the store cannot keep it: none is
// defaulted, looked up or stored.
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_StoreWithoutOUColumn() {
	s.idpService.idpStore = noOUStore{s.mockStore}
	s.idpService.ouService = newOuProviderMock(s.T())
	s.expectCreate("")
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("user-1", "caller-ou", "", nil, nil))

	result, err := s.idpService.CreateIdentityProvider(ctx, s.newIDPForOU("ou-1"))

	s.Nil(err)
	s.Empty(result.OUID)
}

// TestUpdateIdentityProvider_StoreWithoutOUColumn lets an update that names an organization unit through when the
// store cannot keep one.
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_StoreWithoutOUColumn() {
	s.idpService.idpStore = noOUStore{s.mockStore}
	s.expectUpdate("", "")
	s.mockStore.On("UpdateIdentityProvider", mock.Anything, mock.MatchedBy(func(dto *providers.IDPDTO) bool {
		return dto.OUID == ""
	})).Return(nil)

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", s.newIDPForOU("ou-1"))

	s.Nil(err)
	s.Empty(result.OUID)
}

// TestStoreRecordsOU follows the store a wrapper stands in front of.
func (s *IDPServiceTestSuite) TestStoreRecordsOU() {
	s.True(storeRecordsOU(s.mockStore), "a store that does not say records the owner")
	s.True(storeRecordsOU(&idpStore{hasOUColumn: true}))
	s.False(storeRecordsOU(&idpStore{}))
	s.False(storeRecordsOU(newCompositeIDPStore(s.mockStore, &idpStore{})))
	s.True(storeRecordsOU(newCompositeIDPStore(s.mockStore, &idpStore{hasOUColumn: true})))
	s.False(storeRecordsOU(newCacheBackedIDPStore(nil, nil, &idpStore{})))
	s.True(storeRecordsOU(newCacheBackedIDPStore(nil, nil, &idpStore{hasOUColumn: true})))
}
