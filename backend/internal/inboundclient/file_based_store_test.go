/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package inboundclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

type FileBasedStoreTestSuite struct {
	suite.Suite
}

func TestFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedStoreTestSuite))
}

func (suite *FileBasedStoreTestSuite) SetupTest() {
	sysconfig.ResetServerRuntime()
	suite.Require().NoError(sysconfig.InitializeServerRuntime("/tmp/test", &sysconfig.Config{}))
}

func newFileBasedStoreForTest() *fileBasedStore {
	return &fileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeApplication),
	}
}

func (suite *FileBasedStoreTestSuite) TestCreate_ValidInboundClient() {
	store := newFileBasedStoreForTest()
	client := &inboundmodel.InboundClient{ID: "app-1", AuthFlowID: "flow-1"}

	err := store.Create("app-1", client)
	suite.NoError(err)
}

func (suite *FileBasedStoreTestSuite) TestCreate_InvalidType() {
	store := newFileBasedStoreForTest()

	err := store.Create("app-1", "not-an-inbound-client")
	suite.Error(err)
}

func (suite *FileBasedStoreTestSuite) TestCreateInboundClient_StoresAndRetrieves() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()
	client := inboundmodel.InboundClient{ID: "app-1", AuthFlowID: "flow-1"}

	err := store.CreateInboundClient(ctx, client)
	suite.NoError(err)

	got, err := store.GetInboundClientByEntityID(ctx, "app-1")
	suite.NoError(err)
	suite.Equal("app-1", got.ID)
	suite.Equal("flow-1", got.AuthFlowID)
}

func (suite *FileBasedStoreTestSuite) TestCreateOAuthProfile_NotSupported() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	err := store.CreateOAuthProfile(ctx, "app-1", &inboundmodel.OAuthProfile{})
	suite.Error(err)
}

func (suite *FileBasedStoreTestSuite) TestGetInboundClientByEntityID_NotFound() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	got, err := store.GetInboundClientByEntityID(ctx, "nonexistent")
	suite.Nil(got)
	suite.ErrorIs(err, ErrInboundClientNotFound)
}

func (suite *FileBasedStoreTestSuite) TestGetOAuthProfileByEntityID_EmbeddedValueType() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	profileData := inboundmodel.OAuthProfile{
		GrantTypes: []string{"authorization_code"},
	}
	client := inboundmodel.InboundClient{
		ID: "app-2",
		Properties: map[string]interface{}{
			PropOAuthProfile: profileData,
		},
	}
	suite.NoError(store.CreateInboundClient(ctx, client))

	profile, err := store.GetOAuthProfileByEntityID(ctx, "app-2")
	suite.NoError(err)
	suite.NotNil(profile)
	suite.Equal([]string{"authorization_code"}, profile.GrantTypes)
}

func (suite *FileBasedStoreTestSuite) TestGetOAuthProfileByEntityID_EmbeddedPointerType() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	profileData := &inboundmodel.OAuthProfile{
		GrantTypes: []string{"client_credentials"},
	}
	client := inboundmodel.InboundClient{
		ID: "app-3",
		Properties: map[string]interface{}{
			PropOAuthProfile: profileData,
		},
	}
	suite.NoError(store.CreateInboundClient(ctx, client))

	profile, err := store.GetOAuthProfileByEntityID(ctx, "app-3")
	suite.NoError(err)
	suite.NotNil(profile)
	suite.Equal([]string{"client_credentials"}, profile.GrantTypes)
}

func (suite *FileBasedStoreTestSuite) TestGetOAuthProfileByEntityID_NilPointer() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	var nilProfile *inboundmodel.OAuthProfile
	client := inboundmodel.InboundClient{
		ID: "app-nil",
		Properties: map[string]interface{}{
			PropOAuthProfile: nilProfile,
		},
	}
	suite.NoError(store.CreateInboundClient(ctx, client))

	profile, err := store.GetOAuthProfileByEntityID(ctx, "app-nil")
	suite.NoError(err)
	suite.Nil(profile)
}

func (suite *FileBasedStoreTestSuite) TestGetOAuthProfileByEntityID_NoProperties() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	client := inboundmodel.InboundClient{ID: "app-4"}
	suite.NoError(store.CreateInboundClient(ctx, client))

	profile, err := store.GetOAuthProfileByEntityID(ctx, "app-4")
	suite.NoError(err)
	suite.Nil(profile)
}

func (suite *FileBasedStoreTestSuite) TestGetOAuthProfileByEntityID_KeyAbsent() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	client := inboundmodel.InboundClient{
		ID:         "app-5",
		Properties: map[string]interface{}{"other_key": "value"},
	}
	suite.NoError(store.CreateInboundClient(ctx, client))

	profile, err := store.GetOAuthProfileByEntityID(ctx, "app-5")
	suite.NoError(err)
	suite.Nil(profile)
}

func (suite *FileBasedStoreTestSuite) TestGetOAuthProfileByEntityID_InvalidType() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	client := inboundmodel.InboundClient{
		ID: "app-bad",
		Properties: map[string]interface{}{
			PropOAuthProfile: 12345,
		},
	}
	suite.NoError(store.CreateInboundClient(ctx, client))

	profile, err := store.GetOAuthProfileByEntityID(ctx, "app-bad")
	suite.Nil(profile)
	suite.ErrorIs(err, ErrInboundClientDataCorrupted)
}

func (suite *FileBasedStoreTestSuite) TestGetOAuthProfileByEntityID_NotFound() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	_, err := store.GetOAuthProfileByEntityID(ctx, "missing")
	suite.ErrorIs(err, ErrInboundClientNotFound)
}

func (suite *FileBasedStoreTestSuite) TestGetInboundClientList_Empty() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	list, err := store.GetInboundClientList(ctx, 0)
	suite.NoError(err)
	suite.Empty(list)
}

func (suite *FileBasedStoreTestSuite) TestGetInboundClientList_MultipleClients() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	for _, id := range []string{"c1", "c2", "c3"} {
		suite.NoError(store.CreateInboundClient(ctx, inboundmodel.InboundClient{ID: id}))
	}

	list, err := store.GetInboundClientList(ctx, 0)
	suite.NoError(err)
	suite.Len(list, 3)
	for _, c := range list {
		suite.True(c.IsReadOnly)
	}
}

func (suite *FileBasedStoreTestSuite) TestGetInboundClientList_RespectsLimit() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	for _, id := range []string{"c1", "c2", "c3"} {
		suite.NoError(store.CreateInboundClient(ctx, inboundmodel.InboundClient{ID: id}))
	}

	list, err := store.GetInboundClientList(ctx, 2)
	suite.NoError(err)
	suite.Len(list, 2)
}

func (suite *FileBasedStoreTestSuite) TestGetTotalInboundClientCount() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	count, err := store.GetTotalInboundClientCount(ctx)
	suite.NoError(err)
	suite.Equal(0, count)

	suite.NoError(store.CreateInboundClient(ctx, inboundmodel.InboundClient{ID: "c1"}))
	suite.NoError(store.CreateInboundClient(ctx, inboundmodel.InboundClient{ID: "c2"}))

	count, err = store.GetTotalInboundClientCount(ctx)
	suite.NoError(err)
	suite.Equal(2, count)
}

func (suite *FileBasedStoreTestSuite) TestUpdateInboundClient_NotSupported() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	err := store.UpdateInboundClient(ctx, inboundmodel.InboundClient{ID: "c1"})
	suite.Error(err)
}

func (suite *FileBasedStoreTestSuite) TestUpdateOAuthProfile_NotSupported() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	err := store.UpdateOAuthProfile(ctx, "c1", &inboundmodel.OAuthProfile{})
	suite.Error(err)
}

func (suite *FileBasedStoreTestSuite) TestDeleteInboundClient_NotSupported() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	err := store.DeleteInboundClient(ctx, "c1")
	suite.Error(err)
}

func (suite *FileBasedStoreTestSuite) TestDeleteOAuthProfile_NotSupported() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	err := store.DeleteOAuthProfile(ctx, "c1")
	suite.Error(err)
}

func (suite *FileBasedStoreTestSuite) TestInboundClientExists() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	exists, err := store.InboundClientExists(ctx, "c1")
	suite.NoError(err)
	suite.False(exists)

	suite.NoError(store.CreateInboundClient(ctx, inboundmodel.InboundClient{ID: "c1"}))

	exists, err = store.InboundClientExists(ctx, "c1")
	suite.NoError(err)
	suite.True(exists)
}

func (suite *FileBasedStoreTestSuite) TestIsDeclarative() {
	store := newFileBasedStoreForTest()
	ctx := context.Background()

	suite.False(store.IsDeclarative(ctx, "c1"))

	suite.NoError(store.CreateInboundClient(ctx, inboundmodel.InboundClient{ID: "c1"}))

	suite.True(store.IsDeclarative(ctx, "c1"))
}
