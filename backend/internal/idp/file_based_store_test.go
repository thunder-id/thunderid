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
	"testing"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"

	"github.com/stretchr/testify/suite"
)

type FileBasedStoreTestSuite struct {
	suite.Suite
	store idpStoreInterface
}

func (suite *FileBasedStoreTestSuite) SetupTest() {
	// Create a new store instance for each test to ensure isolation
	suite.store = &idpFileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeIDP),
	}
}

func TestFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedStoreTestSuite))
}

func (suite *FileBasedStoreTestSuite) TestCreateIdentityProvider() {
	// Create a test IDP
	prop1, err := cmodels.NewProperty("client_id", "test_client_id", false)
	suite.NoError(err)
	prop2, err := cmodels.NewProperty("client_secret", "test_secret", false)
	suite.NoError(err)

	idp := IDPDTO{
		ID:          "test-idp-1",
		Name:        "Test IDP",
		Description: "Test Identity Provider",
		Type:        IDPTypeGoogle,
		Properties:  []cmodels.Property{*prop1, *prop2},
	}

	// Test creation
	err = suite.store.CreateIdentityProvider(context.Background(), idp)
	suite.NoError(err)

	// Verify it was stored
	retrievedIDP, err := suite.store.GetIdentityProvider(context.Background(), "test-idp-1")
	suite.NoError(err)
	suite.NotNil(retrievedIDP)
	suite.Equal("test-idp-1", retrievedIDP.ID)
	suite.Equal("Test IDP", retrievedIDP.Name)
	suite.Equal(IDPTypeGoogle, retrievedIDP.Type)
}

func (suite *FileBasedStoreTestSuite) TestGetIdentityProviderByID() {
	// Create and store an IDP
	prop, err := cmodels.NewProperty("client_id", "test_client_id", false)
	suite.NoError(err)

	idp := IDPDTO{
		ID:          "test-idp-2",
		Name:        "Test IDP 2",
		Description: "Test Identity Provider 2",
		Type:        IDPTypeGitHub,
		Properties:  []cmodels.Property{*prop},
	}

	err = suite.store.CreateIdentityProvider(context.Background(), idp)
	suite.NoError(err)

	// Test retrieval
	retrievedIDP, err := suite.store.GetIdentityProvider(context.Background(), "test-idp-2")
	suite.NoError(err)
	suite.NotNil(retrievedIDP)
	suite.Equal("test-idp-2", retrievedIDP.ID)
	suite.Equal("Test IDP 2", retrievedIDP.Name)
	suite.Equal(IDPTypeGitHub, retrievedIDP.Type)
}

func (suite *FileBasedStoreTestSuite) TestGetIdentityProviderByID_NotFound() {
	// Test retrieval of non-existent IDP
	retrievedIDP, err := suite.store.GetIdentityProvider(context.Background(), "non-existent-id")
	suite.Error(err)
	suite.Nil(retrievedIDP)
	suite.Equal(ErrIDPNotFound, err)
}

func (suite *FileBasedStoreTestSuite) TestGetIdentityProviderByName() {
	// Create and store an IDP
	prop, err := cmodels.NewProperty("client_id", "test_client_id", false)
	suite.NoError(err)

	idp := IDPDTO{
		ID:          "test-idp-3",
		Name:        "Test IDP By Name",
		Description: "Test Identity Provider 3",
		Type:        IDPTypeOIDC,
		Properties:  []cmodels.Property{*prop},
	}

	err = suite.store.CreateIdentityProvider(context.Background(), idp)
	suite.NoError(err)

	// Test retrieval by name
	retrievedIDP, err := suite.store.GetIdentityProviderByName(context.Background(), "Test IDP By Name")
	suite.NoError(err)
	suite.NotNil(retrievedIDP)
	suite.Equal("test-idp-3", retrievedIDP.ID)
	suite.Equal("Test IDP By Name", retrievedIDP.Name)
}

func (suite *FileBasedStoreTestSuite) TestGetIdentityProviderByName_NotFound() {
	// Test retrieval of non-existent IDP by name
	retrievedIDP, err := suite.store.GetIdentityProviderByName(context.Background(), "Non-Existent IDP")
	suite.Error(err)
	suite.Nil(retrievedIDP)
	suite.Equal(ErrIDPNotFound, err)
}

func (suite *FileBasedStoreTestSuite) TestGetIdentityProviderList() {
	// Create and store multiple IDPs
	prop1, _ := cmodels.NewProperty("client_id", "client1", false)
	prop2, _ := cmodels.NewProperty("client_id", "client2", false)

	idp1 := IDPDTO{
		ID:          "test-idp-4",
		Name:        "Test IDP 4",
		Description: "Test Identity Provider 4",
		Type:        IDPTypeGoogle,
		Properties:  []cmodels.Property{*prop1},
	}

	idp2 := IDPDTO{
		ID:          "test-idp-5",
		Name:        "Test IDP 5",
		Description: "Test Identity Provider 5",
		Type:        IDPTypeGitHub,
		Properties:  []cmodels.Property{*prop2},
	}

	suite.NoError(suite.store.CreateIdentityProvider(context.Background(), idp1))
	suite.NoError(suite.store.CreateIdentityProvider(context.Background(), idp2))

	// Test list retrieval
	idpList, err := suite.store.GetIdentityProviderList(context.Background())
	suite.NoError(err)
	suite.Len(idpList, 2)

	// Verify the list contains both IDPs
	idNames := make(map[string]bool)
	for _, basicIDP := range idpList {
		idNames[basicIDP.Name] = true
	}
	suite.True(idNames["Test IDP 4"])
	suite.True(idNames["Test IDP 5"])
}

func (suite *FileBasedStoreTestSuite) TestUpdateIdentityProvider_NotSupported() {
	// Test that update is not supported in file-based store
	idp := &IDPDTO{
		ID:   "test-idp-6",
		Name: "Test IDP 6",
		Type: IDPTypeGoogle,
	}

	err := suite.store.UpdateIdentityProvider(context.Background(), idp)
	suite.Error(err)
	suite.Contains(err.Error(), "not supported in file-based store")
}

func (suite *FileBasedStoreTestSuite) TestDeleteIdentityProvider_NotSupported() {
	// Test that delete is not supported in file-based store
	err := suite.store.DeleteIdentityProvider(context.Background(), "test-idp-7")
	suite.Error(err)
	suite.Contains(err.Error(), "not supported in file-based store")
}
