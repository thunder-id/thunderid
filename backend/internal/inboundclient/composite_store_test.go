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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
)

type CompositeStoreTestSuite struct {
	suite.Suite
	fileStore *fileBasedStore
	dbMock    *inboundClientStoreInterfaceMock
	composite inboundClientStoreInterface
}

func TestCompositeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeStoreTestSuite))
}

func (suite *CompositeStoreTestSuite) SetupTest() {
	sysconfig.ResetServerRuntime()
	suite.Require().NoError(sysconfig.InitializeServerRuntime("/tmp/test", &sysconfig.Config{}))
	suite.fileStore = newFileBasedStoreForTest()
	suite.dbMock = newInboundClientStoreInterfaceMock(suite.T())
	suite.composite = newCompositeStore(suite.fileStore, suite.dbMock)
}

// GetTotalInboundClientCount sums DB and file counts.
func (suite *CompositeStoreTestSuite) TestGetTotalInboundClientCount_SumsBoth() {
	ctx := context.Background()
	suite.dbMock.EXPECT().GetTotalInboundClientCount(mock.Anything).Return(3, nil)

	// Add 2 entries to file store
	suite.Require().NoError(suite.fileStore.CreateInboundClient(ctx, inboundmodel.InboundClient{ID: "f1"}))
	suite.Require().NoError(suite.fileStore.CreateInboundClient(ctx, inboundmodel.InboundClient{ID: "f2"}))

	count, err := suite.composite.GetTotalInboundClientCount(ctx)
	suite.NoError(err)
	suite.Equal(5, count)
}

// GetInboundClientList merges DB and file clients.
func (suite *CompositeStoreTestSuite) TestGetInboundClientList_MergesBoth() {
	ctx := context.Background()
	dbClients := []inboundmodel.InboundClient{{ID: "db1"}}
	suite.dbMock.EXPECT().GetTotalInboundClientCount(mock.Anything).Return(1, nil).Maybe()
	suite.dbMock.EXPECT().GetInboundClientList(mock.Anything, mock.Anything).Return(dbClients, nil)

	suite.Require().NoError(suite.fileStore.CreateInboundClient(ctx, inboundmodel.InboundClient{ID: "f1"}))

	list, err := suite.composite.GetInboundClientList(ctx, 0)
	suite.NoError(err)
	suite.NotEmpty(list)
	ids := make(map[string]bool)
	for _, c := range list {
		ids[c.ID] = true
	}
	suite.True(ids["db1"])
	suite.True(ids["f1"])
}

// GetInboundClientByEntityID — DB has it.
func (suite *CompositeStoreTestSuite) TestGetInboundClientByEntityID_FromDB() {
	ctx := context.Background()
	want := &inboundmodel.InboundClient{ID: "db1"}
	suite.dbMock.EXPECT().GetInboundClientByEntityID(mock.Anything, "db1").Return(want, nil)

	got, err := suite.composite.GetInboundClientByEntityID(ctx, "db1")
	suite.NoError(err)
	suite.Equal("db1", got.ID)
}

// GetInboundClientByEntityID — not in DB, falls back to file store.
func (suite *CompositeStoreTestSuite) TestGetInboundClientByEntityID_FallsBackToFile() {
	ctx := context.Background()
	suite.dbMock.EXPECT().GetInboundClientByEntityID(mock.Anything, "f1").Return(nil, ErrInboundClientNotFound)
	suite.Require().NoError(suite.fileStore.CreateInboundClient(ctx, inboundmodel.InboundClient{ID: "f1"}))

	got, err := suite.composite.GetInboundClientByEntityID(ctx, "f1")
	suite.NoError(err)
	suite.Equal("f1", got.ID)
}

// GetInboundClientByEntityID — not found anywhere.
func (suite *CompositeStoreTestSuite) TestGetInboundClientByEntityID_NotFound() {
	ctx := context.Background()
	suite.dbMock.EXPECT().GetInboundClientByEntityID(mock.Anything, "missing").Return(nil, ErrInboundClientNotFound)

	got, err := suite.composite.GetInboundClientByEntityID(ctx, "missing")
	suite.Nil(got)
	suite.ErrorIs(err, ErrInboundClientNotFound)
}

// GetOAuthProfileByEntityID — DB has it.
func (suite *CompositeStoreTestSuite) TestGetOAuthProfileByEntityID_FromDB() {
	ctx := context.Background()
	want := &inboundmodel.OAuthProfile{GrantTypes: []string{"authorization_code"}}
	suite.dbMock.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "db1").Return(want, nil)

	got, err := suite.composite.GetOAuthProfileByEntityID(ctx, "db1")
	suite.NoError(err)
	suite.Equal(want, got)
}

// GetOAuthProfileByEntityID — falls back to file store.
func (suite *CompositeStoreTestSuite) TestGetOAuthProfileByEntityID_FallsBackToFile() {
	ctx := context.Background()
	suite.dbMock.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "f1").Return(nil, ErrInboundClientNotFound)

	profileData := inboundmodel.OAuthProfile{GrantTypes: []string{"authorization_code"}}
	suite.Require().NoError(suite.fileStore.CreateInboundClient(ctx, inboundmodel.InboundClient{
		ID:         "f1",
		Properties: map[string]interface{}{PropOAuthProfile: profileData},
	}))

	got, err := suite.composite.GetOAuthProfileByEntityID(ctx, "f1")
	suite.NoError(err)
	suite.Equal([]string{"authorization_code"}, got.GrantTypes)
}

// CreateInboundClient delegates to DB store.
func (suite *CompositeStoreTestSuite) TestCreateInboundClient_DelegatesToDB() {
	ctx := context.Background()
	client := inboundmodel.InboundClient{ID: "new1"}
	suite.dbMock.EXPECT().CreateInboundClient(mock.Anything, client).Return(nil)

	err := suite.composite.CreateInboundClient(ctx, client)
	suite.NoError(err)
}

// CreateOAuthProfile delegates to DB store.
func (suite *CompositeStoreTestSuite) TestCreateOAuthProfile_DelegatesToDB() {
	ctx := context.Background()
	p := &inboundmodel.OAuthProfile{}
	suite.dbMock.EXPECT().CreateOAuthProfile(mock.Anything, "e1", p).Return(nil)

	err := suite.composite.CreateOAuthProfile(ctx, "e1", p)
	suite.NoError(err)
}

// UpdateInboundClient delegates to DB store.
func (suite *CompositeStoreTestSuite) TestUpdateInboundClient_DelegatesToDB() {
	ctx := context.Background()
	client := inboundmodel.InboundClient{ID: "u1"}
	suite.dbMock.EXPECT().UpdateInboundClient(mock.Anything, client).Return(nil)

	err := suite.composite.UpdateInboundClient(ctx, client)
	suite.NoError(err)
}

// UpdateOAuthProfile delegates to DB store.
func (suite *CompositeStoreTestSuite) TestUpdateOAuthProfile_DelegatesToDB() {
	ctx := context.Background()
	p := &inboundmodel.OAuthProfile{}
	suite.dbMock.EXPECT().UpdateOAuthProfile(mock.Anything, "e1", p).Return(nil)

	err := suite.composite.UpdateOAuthProfile(ctx, "e1", p)
	suite.NoError(err)
}

// DeleteInboundClient delegates to DB store.
func (suite *CompositeStoreTestSuite) TestDeleteInboundClient_DelegatesToDB() {
	ctx := context.Background()
	suite.dbMock.EXPECT().DeleteInboundClient(mock.Anything, "e1").Return(nil)

	err := suite.composite.DeleteInboundClient(ctx, "e1")
	suite.NoError(err)
}

// DeleteOAuthProfile delegates to DB store.
func (suite *CompositeStoreTestSuite) TestDeleteOAuthProfile_DelegatesToDB() {
	ctx := context.Background()
	suite.dbMock.EXPECT().DeleteOAuthProfile(mock.Anything, "e1").Return(nil)

	err := suite.composite.DeleteOAuthProfile(ctx, "e1")
	suite.NoError(err)
}

// InboundClientExists — found in file store.
func (suite *CompositeStoreTestSuite) TestInboundClientExists_InFileStore() {
	ctx := context.Background()
	suite.Require().NoError(suite.fileStore.CreateInboundClient(ctx, inboundmodel.InboundClient{ID: "f1"}))

	exists, err := suite.composite.InboundClientExists(ctx, "f1")
	suite.NoError(err)
	suite.True(exists)
}

// InboundClientExists — found in DB store.
func (suite *CompositeStoreTestSuite) TestInboundClientExists_InDBStore() {
	ctx := context.Background()
	suite.dbMock.EXPECT().InboundClientExists(mock.Anything, "db1").Return(true, nil)

	exists, err := suite.composite.InboundClientExists(ctx, "db1")
	suite.NoError(err)
	suite.True(exists)
}

// InboundClientExists — not found in either.
func (suite *CompositeStoreTestSuite) TestInboundClientExists_NotFound() {
	ctx := context.Background()
	suite.dbMock.EXPECT().InboundClientExists(mock.Anything, "missing").Return(false, nil)

	exists, err := suite.composite.InboundClientExists(ctx, "missing")
	suite.NoError(err)
	suite.False(exists)
}

// IsDeclarative — true when entity is in file store.
func (suite *CompositeStoreTestSuite) TestIsDeclarative_True() {
	ctx := context.Background()
	suite.Require().NoError(suite.fileStore.CreateInboundClient(ctx, inboundmodel.InboundClient{ID: "f1"}))

	suite.True(suite.composite.IsDeclarative(ctx, "f1"))
}

// IsDeclarative — false when entity is not in file store.
func (suite *CompositeStoreTestSuite) TestIsDeclarative_False() {
	ctx := context.Background()

	suite.False(suite.composite.IsDeclarative(ctx, "db1"))
}

// mergeAndDeduplicateInboundClients — DB entries are mutable, file entries declarative.
func (suite *CompositeStoreTestSuite) TestMergeAndDeduplicate_SetsReadOnly() {
	dbClients := []inboundmodel.InboundClient{{ID: "shared"}, {ID: "dbonly"}}
	fileClients := []inboundmodel.InboundClient{{ID: "shared"}, {ID: "fileonly"}}

	result := mergeAndDeduplicateInboundClients(dbClients, fileClients)
	suite.Len(result, 3)

	byID := make(map[string]inboundmodel.InboundClient)
	for _, c := range result {
		byID[c.ID] = c
	}

	suite.False(byID["shared"].IsReadOnly)
	suite.False(byID["dbonly"].IsReadOnly)
	suite.True(byID["fileonly"].IsReadOnly)
}
