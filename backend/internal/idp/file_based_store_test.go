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
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

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

	idp := providers.IDPDTO{
		ID:          "test-idp-1",
		Name:        "Test IDP",
		Description: "Test Identity Provider",
		Type:        providers.IDPTypeGoogle,
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
	suite.Equal(providers.IDPTypeGoogle, retrievedIDP.Type)
}

func (suite *FileBasedStoreTestSuite) TestGetIdentityProviderByID() {
	// Create and store an IDP
	prop, err := cmodels.NewProperty("client_id", "test_client_id", false)
	suite.NoError(err)

	idp := providers.IDPDTO{
		ID:          "test-idp-2",
		Name:        "Test IDP 2",
		Description: "Test Identity Provider 2",
		Type:        providers.IDPTypeGitHub,
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
	suite.Equal(providers.IDPTypeGitHub, retrievedIDP.Type)
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

	idp := providers.IDPDTO{
		ID:          "test-idp-3",
		Name:        "Test IDP By Name",
		Description: "Test Identity Provider 3",
		Type:        providers.IDPTypeOIDC,
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

	idp1 := providers.IDPDTO{
		ID:          "test-idp-4",
		Name:        "Test IDP 4",
		Description: "Test Identity Provider 4",
		Type:        providers.IDPTypeGoogle,
		Properties:  []cmodels.Property{*prop1},
	}

	idp2 := providers.IDPDTO{
		ID:          "test-idp-5",
		Name:        "Test IDP 5",
		Description: "Test Identity Provider 5",
		Type:        providers.IDPTypeGitHub,
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

func (suite *FileBasedStoreTestSuite) TestGetIdentityProviderList_IDJagEnabled() {
	trustedProp, _ := cmodels.NewProperty(PropIDJagEnabled, "true", false)
	disabledProp, _ := cmodels.NewProperty(PropIDJagEnabled, "false", false)
	plainProp, _ := cmodels.NewProperty("client_id", "client1", false)

	trusted := providers.IDPDTO{
		ID:         "test-idp-trusted",
		Name:       "Trusted Issuer",
		Type:       providers.IDPTypeOIDC,
		Properties: []cmodels.Property{*trustedProp},
	}
	disabled := providers.IDPDTO{
		ID:         "test-idp-disabled",
		Name:       "Disabled Issuer",
		Type:       providers.IDPTypeOIDC,
		Properties: []cmodels.Property{*disabledProp},
	}
	plain := providers.IDPDTO{
		ID:         "test-idp-plain",
		Name:       "Plain Federation",
		Type:       providers.IDPTypeOIDC,
		Properties: []cmodels.Property{*plainProp},
	}

	suite.NoError(suite.store.CreateIdentityProvider(context.Background(), trusted))
	suite.NoError(suite.store.CreateIdentityProvider(context.Background(), disabled))
	suite.NoError(suite.store.CreateIdentityProvider(context.Background(), plain))

	idpList, err := suite.store.GetIdentityProviderList(context.Background())
	suite.NoError(err)
	suite.Len(idpList, 3)

	byID := make(map[string]BasicIDPDTO, len(idpList))
	for _, basicIDP := range idpList {
		byID[basicIDP.ID] = basicIDP
	}

	suite.Require().NotNil(byID["test-idp-trusted"].IDJagEnabled)
	suite.True(*byID["test-idp-trusted"].IDJagEnabled)
	suite.Require().NotNil(byID["test-idp-disabled"].IDJagEnabled)
	suite.False(*byID["test-idp-disabled"].IDJagEnabled)
	suite.Nil(byID["test-idp-plain"].IDJagEnabled)
}

func (suite *FileBasedStoreTestSuite) TestUpdateIdentityProvider_NotSupported() {
	// Test that update is not supported in file-based store
	idp := &providers.IDPDTO{
		ID:   "test-idp-6",
		Name: "Test IDP 6",
		Type: providers.IDPTypeGoogle,
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

func (suite *FileBasedStoreTestSuite) TestGetIdentityProvidersByProperty_ReturnsMatchingIDPs() {
	prop1, _ := cmodels.NewProperty("issuer", "https://example.com", false)
	prop2, _ := cmodels.NewProperty("client_id", "client1", false)

	idp1 := providers.IDPDTO{
		ID:         "test-idp-prop-1",
		Name:       "IDP Prop 1",
		Type:       providers.IDPTypeOIDC,
		Properties: []cmodels.Property{*prop1, *prop2},
	}

	prop3, _ := cmodels.NewProperty("issuer", "https://other.com", false)
	idp2 := providers.IDPDTO{
		ID:         "test-idp-prop-2",
		Name:       "IDP Prop 2",
		Type:       providers.IDPTypeOIDC,
		Properties: []cmodels.Property{*prop3},
	}

	suite.NoError(suite.store.CreateIdentityProvider(context.Background(), idp1))
	suite.NoError(suite.store.CreateIdentityProvider(context.Background(), idp2))

	result, err := suite.store.GetIdentityProvidersByProperty(
		context.Background(), "issuer", "https://example.com")
	suite.NoError(err)
	suite.Len(result, 1)
	suite.Equal("test-idp-prop-1", result[0].ID)
}

func (suite *FileBasedStoreTestSuite) TestGetIdentityProvidersByProperty_NonMatchingValue() {
	prop, _ := cmodels.NewProperty("issuer", "https://example.com", false)
	idp := providers.IDPDTO{
		ID:         "test-idp-prop-3",
		Name:       "IDP Prop 3",
		Type:       providers.IDPTypeOIDC,
		Properties: []cmodels.Property{*prop},
	}

	suite.NoError(suite.store.CreateIdentityProvider(context.Background(), idp))

	result, err := suite.store.GetIdentityProvidersByProperty(
		context.Background(), "issuer", "https://nomatch.com")
	suite.ErrorIs(err, ErrIDPNotFound)
	suite.Nil(result)
}

func (suite *FileBasedStoreTestSuite) TestGetIdentityProvidersByProperty_MultipleMatches() {
	prop1, _ := cmodels.NewProperty("issuer", "https://shared.com", false)
	idp1 := providers.IDPDTO{
		ID:         "test-idp-prop-4",
		Name:       "IDP Prop 4",
		Type:       providers.IDPTypeOIDC,
		Properties: []cmodels.Property{*prop1},
	}

	prop2, _ := cmodels.NewProperty("issuer", "https://shared.com", false)
	idp2 := providers.IDPDTO{
		ID:         "test-idp-prop-5",
		Name:       "IDP Prop 5",
		Type:       providers.IDPTypeOIDC,
		Properties: []cmodels.Property{*prop2},
	}

	suite.NoError(suite.store.CreateIdentityProvider(context.Background(), idp1))
	suite.NoError(suite.store.CreateIdentityProvider(context.Background(), idp2))

	result, err := suite.store.GetIdentityProvidersByProperty(
		context.Background(), "issuer", "https://shared.com")
	suite.NoError(err)
	suite.Len(result, 2)
}

func (suite *FileBasedStoreTestSuite) TestGetIdentityProviderListCount() {
	prop1, _ := cmodels.NewProperty("client_id", "client1", false)
	prop2, _ := cmodels.NewProperty("client_id", "client2", false)

	idp1 := providers.IDPDTO{
		ID:         "test-idp-count-1",
		Name:       "Count IDP 1",
		Type:       providers.IDPTypeOIDC,
		Properties: []cmodels.Property{*prop1},
	}
	idp2 := providers.IDPDTO{
		ID:         "test-idp-count-2",
		Name:       "Count IDP 2",
		Type:       providers.IDPTypeOIDC,
		Properties: []cmodels.Property{*prop2},
	}

	suite.NoError(suite.store.CreateIdentityProvider(context.Background(), idp1))
	suite.NoError(suite.store.CreateIdentityProvider(context.Background(), idp2))

	count, err := suite.store.GetIdentityProviderListCount(context.Background())
	suite.NoError(err)
	suite.Equal(2, count)
}
