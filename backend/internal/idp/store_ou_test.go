// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package idp

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

// newDetectProvider returns a database provider whose config client answers the OU_ID column check with results
// and queryErr, or fails to give a client with clientErr.
func newDetectProvider(results []map[string]interface{}, queryErr, clientErr error,
) *providermock.DBProviderInterfaceMock {
	dbProvider := &providermock.DBProviderInterfaceMock{}
	if clientErr != nil {
		dbProvider.On("GetConfigDBClient").Return(nil, clientErr)
		return dbProvider
	}
	dbClient := &providermock.DBClientInterfaceMock{}
	dbProvider.On("GetConfigDBClient").Return(dbClient, nil)
	dbClient.On("QueryContext", context.Background(), queryCheckIdentityProviderOUColumn).Return(results, queryErr)
	return dbProvider
}

// TestCreateIdentityProvider_WithOUColumn records the owning organization unit when the table has the column.
func (s *IDPStoreTestSuite) TestCreateIdentityProvider_WithOUColumn() {
	s.store.hasOUColumn = true
	idp := providers.IDPDTO{ID: "idp-123", Name: "Test IDP", Description: "Desc", Type: providers.IDPTypeOIDC,
		OUID: "ou-1"}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryCreateIdentityProviderWithOU, idp.ID, idp.Name,
		idp.Description, idp.Type, "ou-1", "", nil, testDeploymentID).Return(int64(1), nil)

	s.NoError(s.store.CreateIdentityProvider(context.Background(), idp))
	s.mockDBClient.AssertExpectations(s.T())
}

// TestCreateIdentityProvider_WithoutOUColumnIgnoresOwner leaves the owner out when the table has no column.
func (s *IDPStoreTestSuite) TestCreateIdentityProvider_WithoutOUColumnIgnoresOwner() {
	idp := providers.IDPDTO{ID: "idp-123", Name: "Test IDP", Type: providers.IDPTypeOIDC, OUID: "ou-1"}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryCreateIdentityProvider, idp.ID, idp.Name,
		idp.Description, idp.Type, "", nil, testDeploymentID).Return(int64(1), nil)

	s.NoError(s.store.CreateIdentityProvider(context.Background(), idp))
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProviderList_WithOUColumn reads the owner, and treats a NULL owner as none.
func (s *IDPStoreTestSuite) TestGetIdentityProviderList_WithOUColumn() {
	s.store.hasOUColumn = true
	results := []map[string]interface{}{
		{"id": "idp-1", "name": "IDP 1", "description": "", "type": "OIDC", "ou_id": "ou-1"},
		{"id": "idp-2", "name": "IDP 2", "description": "", "type": "GOOGLE", "ou_id": nil},
	}
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderListWithOU,
		testDeploymentID).Return(results, nil)

	list, err := s.store.GetIdentityProviderList(context.Background())

	s.NoError(err)
	s.Require().Len(list, 2)
	s.Equal("ou-1", list[0].OUID)
	s.Empty(list[1].OUID)
}

// TestGetIdentityProvider_WithOUColumn reads the owner of an identity provider found by id.
func (s *IDPStoreTestSuite) TestGetIdentityProvider_WithOUColumn() {
	s.store.hasOUColumn = true
	results := []map[string]interface{}{
		{"id": "idp-123", "name": "Test IDP", "description": "", "type": "OIDC", "ou_id": "ou-1"},
	}
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByIDWithOU,
		"idp-123", testDeploymentID).Return(results, nil)

	idp, err := s.store.GetIdentityProvider(context.Background(), "idp-123")

	s.NoError(err)
	s.Equal("ou-1", idp.OUID)
}

// TestGetIdentityProviderByName_WithOUColumn reads the owner of an identity provider found by name.
func (s *IDPStoreTestSuite) TestGetIdentityProviderByName_WithOUColumn() {
	s.store.hasOUColumn = true
	results := []map[string]interface{}{
		{"id": "idp-123", "name": "Test IDP", "description": "", "type": "OIDC", "ou_id": "ou-1"},
	}
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByNameWithOU,
		"Test IDP", testDeploymentID).Return(results, nil)

	idp, err := s.store.GetIdentityProviderByName(context.Background(), "Test IDP")

	s.NoError(err)
	s.Equal("ou-1", idp.OUID)
}

// TestGetIdentityProvidersByProperty_WithOUColumn reads the owner of the identity providers found by a property.
func (s *IDPStoreTestSuite) TestGetIdentityProvidersByProperty_WithOUColumn() {
	s.store.hasOUColumn = true
	results := []map[string]interface{}{
		{"id": "idp-123", "name": "Test IDP", "description": "", "type": "OIDC", "ou_id": "ou-1"},
	}
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProvidersByPropertyWithOU,
		"issuer", "https://idp.example.com", testDeploymentID).Return(results, nil)

	idps, err := s.store.GetIdentityProvidersByProperty(context.Background(), "issuer", "https://idp.example.com")

	s.NoError(err)
	s.Require().Len(idps, 1)
	s.Equal("ou-1", idps[0].OUID)
}

// TestDetectOUColumn tests whether the store finds the OU_ID column on the IDP table.
func (s *IDPStoreTestSuite) TestDetectOUColumn() {
	s.Run("present", func() {
		dbProvider := newDetectProvider([]map[string]interface{}{{"1": int64(1)}}, nil, nil)
		has, err := detectOUColumn(dbProvider)
		s.NoError(err)
		s.True(has)
	})
	s.Run("absent", func() {
		dbProvider := newDetectProvider([]map[string]interface{}{}, nil, nil)
		has, err := detectOUColumn(dbProvider)
		s.NoError(err)
		s.False(has)
	})
	s.Run("query fails", func() {
		dbProvider := newDetectProvider(nil, errors.New("query failed"), nil)
		has, err := detectOUColumn(dbProvider)
		s.ErrorContains(err, "query failed")
		s.False(has)
	})
	s.Run("client fails", func() {
		dbProvider := newDetectProvider(nil, nil, errors.New("no client"))
		has, err := detectOUColumn(dbProvider)
		s.ErrorContains(err, "no client")
		s.False(has)
	})
}
