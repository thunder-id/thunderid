/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package idp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment-id"

type IDPStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *idpStore
}

func TestIDPStoreTestSuite(t *testing.T) {
	suite.Run(t, new(IDPStoreTestSuite))
}

func (s *IDPStoreTestSuite) SetupTest() {
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	s.mockDBProvider = &providermock.DBProviderInterfaceMock{}
	s.mockDBClient = &providermock.DBClientInterfaceMock{}

	s.store = &idpStore{
		dbProvider:   s.mockDBProvider,
		deploymentID: testDeploymentID,
	}
}

// TestNewIDPStore tests store creation
func (s *IDPStoreTestSuite) TestNewIDPStore() {
	store, _, err := newIDPStore()
	s.NoError(err)
	s.NotNil(store)
	s.Implements((*idpStoreInterface)(nil), store)
}

// TestCreateIdentityProvider_Success tests successful IDP creation
func (s *IDPStoreTestSuite) TestCreateIdentityProvider_Success() {
	prop, _ := cmodels.NewProperty("client_id", "test-client", false)
	idp := IDPDTO{
		ID:          "idp-123",
		Name:        "Test IDP",
		Description: "Test Description",
		Type:        IDPTypeOIDC,
		Properties:  []cmodels.Property{*prop},
	}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryCreateIdentityProvider, idp.ID, idp.Name,
		idp.Description, idp.Type, `[{"name":"client_id","value":"test-client","isSecret":false}]`, testDeploymentID).
		Return(int64(1), nil)

	err := s.store.CreateIdentityProvider(context.Background(), idp)

	s.NoError(err)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestCreateIdentityProvider_NoProperties tests IDP creation without properties
func (s *IDPStoreTestSuite) TestCreateIdentityProvider_NoProperties() {
	idp := IDPDTO{
		ID:          "idp-123",
		Name:        "Test IDP",
		Description: "Test Description",
		Type:        IDPTypeOIDC,
		Properties:  []cmodels.Property{},
	}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryCreateIdentityProvider, idp.ID, idp.Name,
		idp.Description, idp.Type, "", testDeploymentID).Return(int64(1), nil)

	err := s.store.CreateIdentityProvider(context.Background(), idp)

	s.NoError(err)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestCreateIdentityProvider_DBClientError tests DB client error
func (s *IDPStoreTestSuite) TestCreateIdentityProvider_DBClientError() {
	idp := IDPDTO{ID: "idp-123", Name: "Test", Type: IDPTypeOIDC}

	s.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db error"))

	err := s.store.CreateIdentityProvider(context.Background(), idp)

	s.Error(err)
	s.Contains(err.Error(), "failed to get database client")
	s.mockDBProvider.AssertExpectations(s.T())
}

// TestCreateIdentityProvider_ExecuteError tests execute error
func (s *IDPStoreTestSuite) TestCreateIdentityProvider_ExecuteError() {
	idp := IDPDTO{ID: "idp-123", Name: "Test", Type: IDPTypeOIDC}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryCreateIdentityProvider, idp.ID, idp.Name,
		idp.Description, idp.Type, "", testDeploymentID).Return(int64(0), errors.New("execute error"))

	err := s.store.CreateIdentityProvider(context.Background(), idp)

	s.Error(err)
	s.Contains(err.Error(), "failed to execute query")
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProviderList_Success tests successful list retrieval
func (s *IDPStoreTestSuite) TestGetIdentityProviderList_Success() {
	results := []map[string]interface{}{
		{
			"id":          "idp-1",
			"name":        "IDP 1",
			"description": "Desc 1",
			"type":        "OIDC",
		},
		{
			"id":          "idp-2",
			"name":        "IDP 2",
			"description": "Desc 2",
			"type":        "GOOGLE",
		},
	}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderList,
		testDeploymentID).Return(results, nil)

	list, err := s.store.GetIdentityProviderList(context.Background())

	s.NoError(err)
	s.Len(list, 2)
	s.Equal("idp-1", list[0].ID)
	s.Equal("IDP 1", list[0].Name)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProviderList_EmptyList tests empty list
func (s *IDPStoreTestSuite) TestGetIdentityProviderList_EmptyList() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderList,
		testDeploymentID).Return([]map[string]interface{}{}, nil)

	list, err := s.store.GetIdentityProviderList(context.Background())

	s.NoError(err)
	s.Len(list, 0)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProviderList_DBClientError tests DB client error
func (s *IDPStoreTestSuite) TestGetIdentityProviderList_DBClientError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db error"))

	list, err := s.store.GetIdentityProviderList(context.Background())

	s.Error(err)
	s.Nil(list)
	s.Contains(err.Error(), "failed to get database client")
	s.mockDBProvider.AssertExpectations(s.T())
}

// TestGetIdentityProviderList_QueryError tests query error
func (s *IDPStoreTestSuite) TestGetIdentityProviderList_QueryError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderList,
		testDeploymentID).Return(nil, errors.New("query error"))

	list, err := s.store.GetIdentityProviderList(context.Background())

	s.Error(err)
	s.Nil(list)
	s.Contains(err.Error(), "failed to execute query")
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProviderList_BuildError tests error building IDP from row
func (s *IDPStoreTestSuite) TestGetIdentityProviderList_BuildError() {
	results := []map[string]interface{}{
		{
			"id":          123, // Invalid type - should be string
			"name":        "IDP 1",
			"description": "Desc 1",
			"type":        "OIDC",
		},
	}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderList,
		testDeploymentID).Return(results, nil)

	list, err := s.store.GetIdentityProviderList(context.Background())

	s.Error(err)
	s.Nil(list)
	s.Contains(err.Error(), "failed to build idp from result row")
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProvider_Success tests successful IDP retrieval
func (s *IDPStoreTestSuite) TestGetIdentityProvider_Success() {
	results := []map[string]interface{}{
		{
			"id":          "idp-123",
			"name":        "Test IDP",
			"description": "Test Description",
			"type":        "OIDC",
			"properties":  `[{"name":"client_id","value":"test","is_secret":false}]`,
		},
	}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByID,
		"idp-123", testDeploymentID).Return(results, nil)

	idp, err := s.store.GetIdentityProvider(context.Background(), "idp-123")

	s.NoError(err)
	s.NotNil(idp)
	s.Equal("idp-123", idp.ID)
	s.Equal("Test IDP", idp.Name)
	s.Len(idp.Properties, 1)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProvider_NotFound tests IDP not found
func (s *IDPStoreTestSuite) TestGetIdentityProvider_NotFound() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByID,
		"non-existent", testDeploymentID).
		Return([]map[string]interface{}{}, nil)

	idp, err := s.store.GetIdentityProvider(context.Background(), "non-existent")

	s.Error(err)
	s.Nil(idp)
	s.ErrorIs(err, ErrIDPNotFound)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProvider_MultipleResults tests multiple results error
func (s *IDPStoreTestSuite) TestGetIdentityProvider_MultipleResults() {
	results := []map[string]interface{}{
		{"id": "idp-1", "name": "IDP 1", "description": "", "type": "OIDC"},
		{"id": "idp-2", "name": "IDP 2", "description": "", "type": "OIDC"},
	}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByID,
		"duplicate", testDeploymentID).Return(results, nil)

	idp, err := s.store.GetIdentityProvider(context.Background(), "duplicate")

	s.Error(err)
	s.Nil(idp)
	s.Contains(err.Error(), "unexpected number of results")
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProviderByName_Success tests successful IDP retrieval by name
func (s *IDPStoreTestSuite) TestGetIdentityProviderByName_Success() {
	results := []map[string]interface{}{
		{
			"id":          "idp-123",
			"name":        "Test IDP",
			"description": "Test Description",
			"type":        "OIDC",
			"properties":  "",
		},
	}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByName,
		"Test IDP", testDeploymentID).Return(results, nil)

	idp, err := s.store.GetIdentityProviderByName(context.Background(), "Test IDP")

	s.NoError(err)
	s.NotNil(idp)
	s.Equal("Test IDP", idp.Name)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProviderByName_NotFound tests IDP not found by name
func (s *IDPStoreTestSuite) TestGetIdentityProviderByName_NotFound() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByName,
		"Non-existent", testDeploymentID).
		Return([]map[string]interface{}{}, nil)

	idp, err := s.store.GetIdentityProviderByName(context.Background(), "Non-existent")

	s.Error(err)
	s.Nil(idp)
	s.ErrorIs(err, ErrIDPNotFound)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProviderByName_DBClientError tests DB client error
func (s *IDPStoreTestSuite) TestGetIdentityProviderByName_DBClientError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db error"))

	idp, err := s.store.GetIdentityProviderByName(context.Background(), "Test")

	s.Error(err)
	s.Nil(idp)
	s.Contains(err.Error(), "failed to get database client")
	s.mockDBProvider.AssertExpectations(s.T())
}

// TestGetIdentityProviderByName_QueryError tests query error
func (s *IDPStoreTestSuite) TestGetIdentityProviderByName_QueryError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByName, "Test", testDeploymentID).
		Return(nil, errors.New("query error"))

	idp, err := s.store.GetIdentityProviderByName(context.Background(), "Test")

	s.Error(err)
	s.Nil(idp)
	s.Contains(err.Error(), "failed to execute query")
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_Success tests successful IDP update
func (s *IDPStoreTestSuite) TestUpdateIdentityProvider_Success() {
	idp := &IDPDTO{
		ID:          "idp-123",
		Name:        "Updated IDP",
		Description: "Updated Description",
		Type:        IDPTypeOIDC,
		Properties:  []cmodels.Property{},
	}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryUpdateIdentityProviderByID, idp.ID, idp.Name,
		idp.Description, idp.Type, "", testDeploymentID).Return(int64(1), nil)

	err := s.store.UpdateIdentityProvider(context.Background(), idp)

	s.NoError(err)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_WithProperties tests IDP update with properties
func (s *IDPStoreTestSuite) TestUpdateIdentityProvider_WithProperties() {
	prop, _ := cmodels.NewProperty("client_id", "test", false)
	idp := &IDPDTO{
		ID:          "idp-123",
		Name:        "Updated IDP",
		Description: "Updated Description",
		Type:        IDPTypeOIDC,
		Properties:  []cmodels.Property{*prop},
	}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryUpdateIdentityProviderByID, idp.ID, idp.Name,
		idp.Description, idp.Type, `[{"name":"client_id","value":"test","isSecret":false}]`, testDeploymentID).
		Return(int64(1), nil)

	err := s.store.UpdateIdentityProvider(context.Background(), idp)

	s.NoError(err)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_DBClientError tests DB client error
func (s *IDPStoreTestSuite) TestUpdateIdentityProvider_DBClientError() {
	idp := &IDPDTO{ID: "idp-123", Name: "Test", Type: IDPTypeOIDC}

	s.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db error"))

	err := s.store.UpdateIdentityProvider(context.Background(), idp)

	s.Error(err)
	s.Contains(err.Error(), "failed to get database client")
	s.mockDBProvider.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_ExecuteError tests execute error
func (s *IDPStoreTestSuite) TestUpdateIdentityProvider_ExecuteError() {
	idp := &IDPDTO{ID: "idp-123", Name: "Test", Type: IDPTypeOIDC}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryUpdateIdentityProviderByID, idp.ID, idp.Name,
		idp.Description, idp.Type, "", testDeploymentID).Return(int64(0), errors.New("execute error"))

	err := s.store.UpdateIdentityProvider(context.Background(), idp)

	s.Error(err)
	s.Contains(err.Error(), "failed to execute query")
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_Success tests successful IDP deletion
func (s *IDPStoreTestSuite) TestDeleteIdentityProvider_Success() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryDeleteIdentityProviderByID,
		"idp-123", testDeploymentID).
		Return(int64(1), nil)

	err := s.store.DeleteIdentityProvider(context.Background(), "idp-123")

	s.NoError(err)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_NotFound tests deleting non-existent IDP
func (s *IDPStoreTestSuite) TestDeleteIdentityProvider_NotFound() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryDeleteIdentityProviderByID,
		"non-existent", testDeploymentID).
		Return(int64(0), nil)

	err := s.store.DeleteIdentityProvider(context.Background(), "non-existent")

	s.NoError(err) // No error for non-existent IDP
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_DBClientError tests DB client error
func (s *IDPStoreTestSuite) TestDeleteIdentityProvider_DBClientError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db error"))

	err := s.store.DeleteIdentityProvider(context.Background(), "idp-123")

	s.Error(err)
	s.Contains(err.Error(), "failed to get database client")
	s.mockDBProvider.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_ExecuteError tests execute error
func (s *IDPStoreTestSuite) TestDeleteIdentityProvider_ExecuteError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryDeleteIdentityProviderByID,
		"idp-123", testDeploymentID).
		Return(int64(0), errors.New("execute error"))

	err := s.store.DeleteIdentityProvider(context.Background(), "idp-123")

	s.Error(err)
	s.Contains(err.Error(), "failed to execute query")
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestBuildIDPFromResultRow tests building IDP from result row
func (s *IDPStoreTestSuite) TestBuildIDPFromResultRow() {
	testCases := []struct {
		name        string
		row         map[string]interface{}
		expectError bool
		expectedID  string
	}{
		{
			name: "Valid row",
			row: map[string]interface{}{
				"id":          "idp-123",
				"name":        "Test IDP",
				"description": "Test Description",
				"type":        "OIDC",
			},
			expectError: false,
			expectedID:  "idp-123",
		},
		{
			name: "Missing idp_id",
			row: map[string]interface{}{
				"name":        "Test IDP",
				"description": "Test Description",
				"type":        "OIDC",
			},
			expectError: true,
		},
		{
			name: "Invalid idp_id type",
			row: map[string]interface{}{
				"id":          123,
				"name":        "Test IDP",
				"description": "Test Description",
				"type":        "OIDC",
			},
			expectError: true,
		},
		{
			name: "Missing name",
			row: map[string]interface{}{
				"id":          "idp-123",
				"description": "Test Description",
				"type":        "OIDC",
			},
			expectError: true,
		},
		{
			name: "Invalid name type",
			row: map[string]interface{}{
				"id":          "idp-123",
				"name":        123,
				"description": "Test Description",
				"type":        "OIDC",
			},
			expectError: true,
		},
		{
			name: "Invalid description type",
			row: map[string]interface{}{
				"id":          "idp-123",
				"name":        "Test IDP",
				"description": 123,
				"type":        "OIDC",
			},
			expectError: true,
		},
		{
			name: "Invalid type field type",
			row: map[string]interface{}{
				"id":          "idp-123",
				"name":        "Test IDP",
				"description": "Test Description",
				"type":        123,
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			idp, err := buildIDPFromResultRow(tc.row)
			if tc.expectError {
				s.Error(err)
				s.Nil(idp)
			} else {
				s.NoError(err)
				s.NotNil(idp)
				s.Equal(tc.expectedID, idp.ID)
			}
		})
	}
}

// TestGetIdentityProvider_WithByteProperties tests properties as byte array
func (s *IDPStoreTestSuite) TestGetIdentityProvider_WithByteProperties() {
	results := []map[string]interface{}{
		{
			"id":          "idp-123",
			"name":        "Test IDP",
			"description": "Test Description",
			"type":        "OIDC",
			"properties":  []byte(`[{"name":"client_id","value":"test","is_secret":false}]`),
		},
	}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByID,
		"idp-123", testDeploymentID).Return(results, nil)

	idp, err := s.store.GetIdentityProvider(context.Background(), "idp-123")

	s.NoError(err)
	s.NotNil(idp)
	s.Len(idp.Properties, 1)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProvider_DBClientError tests DB client error
func (s *IDPStoreTestSuite) TestGetIdentityProvider_DBClientError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db error"))

	idp, err := s.store.GetIdentityProvider(context.Background(), "idp-123")

	s.Error(err)
	s.Nil(idp)
	s.Contains(err.Error(), "failed to get database client")
	s.mockDBProvider.AssertExpectations(s.T())
}

// TestGetIdentityProvider_QueryError tests query error
func (s *IDPStoreTestSuite) TestGetIdentityProvider_QueryError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByID, "idp-123", testDeploymentID).
		Return(nil, errors.New("query error"))

	idp, err := s.store.GetIdentityProvider(context.Background(), "idp-123")

	s.Error(err)
	s.Nil(idp)
	s.Contains(err.Error(), "failed to execute query")
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProvider_InvalidPropertyJSON tests invalid property JSON
func (s *IDPStoreTestSuite) TestGetIdentityProvider_InvalidPropertyJSON() {
	results := []map[string]interface{}{
		{
			"id":          "idp-123",
			"name":        "Test IDP",
			"description": "Test Description",
			"type":        "OIDC",
			"properties":  "invalid json",
		},
	}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByID,
		"idp-123", testDeploymentID).Return(results, nil)

	idp, err := s.store.GetIdentityProvider(context.Background(), "idp-123")

	s.Error(err)
	s.Nil(idp)
	s.Contains(err.Error(), "failed to deserialize properties from JSON")
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProvider_BuildRowError tests error building IDP from row
func (s *IDPStoreTestSuite) TestGetIdentityProvider_BuildRowError() {
	results := []map[string]interface{}{
		{
			"id":          123, // Invalid type
			"name":        "Test IDP",
			"description": "Test Description",
			"type":        "OIDC",
			"properties":  "",
		},
	}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByID,
		"idp-123", testDeploymentID).Return(results, nil)

	idp, err := s.store.GetIdentityProvider(context.Background(), "idp-123")

	s.Error(err)
	s.Nil(idp)
	s.Contains(err.Error(), "failed to build idp from result row")
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProviderByIssuer_Success tests successful IDP retrieval by issuer.
func (s *IDPStoreTestSuite) TestGetIdentityProviderByIssuer_Success() {
	results := []map[string]interface{}{
		{
			"id":          "idp-1",
			"name":        "IDP 1",
			"description": "Desc 1",
			"type":        "OIDC",
			"properties":  `[{"name":"issuer","value":"https://idp.example.com","isSecret":false}]`,
		},
	}

	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByIssuer,
		"https://idp.example.com", testDeploymentID).Return(results, nil)

	idp, err := s.store.GetIdentityProviderByIssuer(context.Background(), "https://idp.example.com")

	s.NoError(err)
	s.NotNil(idp)
	s.Equal("idp-1", idp.ID)
	s.Equal("IDP 1", idp.Name)
	s.Len(idp.Properties, 1)
	s.Equal("issuer", idp.Properties[0].GetName())
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProviderByIssuer_NotFound tests IDP not found by issuer.
func (s *IDPStoreTestSuite) TestGetIdentityProviderByIssuer_NotFound() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByIssuer,
		"https://unknown.example.com", testDeploymentID).Return([]map[string]interface{}{}, nil)

	idp, err := s.store.GetIdentityProviderByIssuer(context.Background(), "https://unknown.example.com")

	s.Error(err)
	s.Nil(idp)
	s.ErrorIs(err, ErrIDPNotFound)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

// TestGetIdentityProviderByIssuer_DBClientError tests DB client error.
func (s *IDPStoreTestSuite) TestGetIdentityProviderByIssuer_DBClientError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db error"))

	idp, err := s.store.GetIdentityProviderByIssuer(context.Background(), "https://idp.example.com")

	s.Error(err)
	s.Nil(idp)
	s.Contains(err.Error(), "failed to get database client")
	s.mockDBProvider.AssertExpectations(s.T())
}

// TestGetIdentityProviderByIssuer_QueryError tests query execution error.
func (s *IDPStoreTestSuite) TestGetIdentityProviderByIssuer_QueryError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetIdentityProviderByIssuer,
		"https://idp.example.com", testDeploymentID).Return(nil, errors.New("query error"))

	idp, err := s.store.GetIdentityProviderByIssuer(context.Background(), "https://idp.example.com")

	s.Error(err)
	s.Nil(idp)
	s.Contains(err.Error(), "failed to execute query")
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}
