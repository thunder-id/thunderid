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
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/cache"
	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/cachemock"
)

type CacheBackedStoreTestSuite struct {
	suite.Suite
	mockStore    *inboundClientStoreInterfaceMock
	clientCache  *cachemock.CacheInterfaceMock[*inboundmodel.InboundClient]
	profileCache *cachemock.CacheInterfaceMock[*inboundmodel.OAuthProfile]
	cachedStore  *cachedBackStore
}

func TestCacheBackedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CacheBackedStoreTestSuite))
}

func (suite *CacheBackedStoreTestSuite) SetupTest() {
	sysconfig.ResetServerRuntime()
	suite.Require().NoError(sysconfig.InitializeServerRuntime("/tmp/test", &sysconfig.Config{}))

	suite.mockStore = newInboundClientStoreInterfaceMock(suite.T())
	suite.clientCache = cachemock.NewCacheInterfaceMock[*inboundmodel.InboundClient](suite.T())
	suite.profileCache = cachemock.NewCacheInterfaceMock[*inboundmodel.OAuthProfile](suite.T())
	suite.cachedStore = &cachedBackStore{
		inboundClientCache: suite.clientCache,
		oauthProfileCache:  suite.profileCache,
		inner:              suite.mockStore,
	}
}

// CreateInboundClient — inner succeeds, result is cached.
func (suite *CacheBackedStoreTestSuite) TestCreateInboundClient_CachesOnSuccess() {
	ctx := context.Background()
	client := inboundmodel.InboundClient{ID: "c1"}
	suite.mockStore.EXPECT().CreateInboundClient(mock.Anything, client).Return(nil)
	suite.clientCache.EXPECT().Set(mock.Anything, cache.CacheKey{Key: "c1"}, &client).Return(nil)

	err := suite.cachedStore.CreateInboundClient(ctx, client)
	suite.NoError(err)
}

// CreateInboundClient — inner fails, no caching.
func (suite *CacheBackedStoreTestSuite) TestCreateInboundClient_InnerError() {
	ctx := context.Background()
	client := inboundmodel.InboundClient{ID: "c1"}
	storeErr := errors.New("db error")
	suite.mockStore.EXPECT().CreateInboundClient(mock.Anything, client).Return(storeErr)

	err := suite.cachedStore.CreateInboundClient(ctx, client)
	suite.ErrorIs(err, storeErr)
}

// CreateOAuthProfile — delegates to inner.
func (suite *CacheBackedStoreTestSuite) TestCreateOAuthProfile_Delegates() {
	ctx := context.Background()
	p := &inboundmodel.OAuthProfile{}
	suite.mockStore.EXPECT().CreateOAuthProfile(mock.Anything, "e1", p).Return(nil)

	err := suite.cachedStore.CreateOAuthProfile(ctx, "e1", p)
	suite.NoError(err)
}

// GetInboundClientByEntityID — cache hit.
func (suite *CacheBackedStoreTestSuite) TestGetInboundClientByEntityID_CacheHit() {
	ctx := context.Background()
	cached := &inboundmodel.InboundClient{ID: "c1"}
	suite.clientCache.EXPECT().Get(mock.Anything, cache.CacheKey{Key: "c1"}).Return(cached, true)

	got, err := suite.cachedStore.GetInboundClientByEntityID(ctx, "c1")
	suite.NoError(err)
	suite.Equal("c1", got.ID)
}

// GetInboundClientByEntityID — cache miss, fetches from inner and caches.
func (suite *CacheBackedStoreTestSuite) TestGetInboundClientByEntityID_CacheMiss() {
	ctx := context.Background()
	var nilClient *inboundmodel.InboundClient
	suite.clientCache.EXPECT().Get(mock.Anything, cache.CacheKey{Key: "c1"}).Return(nilClient, false)
	inner := &inboundmodel.InboundClient{ID: "c1"}
	suite.mockStore.EXPECT().GetInboundClientByEntityID(mock.Anything, "c1").Return(inner, nil)
	suite.clientCache.EXPECT().Set(mock.Anything, cache.CacheKey{Key: "c1"}, inner).Return(nil)

	got, err := suite.cachedStore.GetInboundClientByEntityID(ctx, "c1")
	suite.NoError(err)
	suite.Equal("c1", got.ID)
}

// GetInboundClientByEntityID — cache miss + inner error.
func (suite *CacheBackedStoreTestSuite) TestGetInboundClientByEntityID_InnerError() {
	ctx := context.Background()
	var nilClient *inboundmodel.InboundClient
	suite.clientCache.EXPECT().Get(mock.Anything, cache.CacheKey{Key: "c1"}).Return(nilClient, false)
	storeErr := errors.New("db error")
	suite.mockStore.EXPECT().GetInboundClientByEntityID(mock.Anything, "c1").Return(nil, storeErr)

	got, err := suite.cachedStore.GetInboundClientByEntityID(ctx, "c1")
	suite.Nil(got)
	suite.ErrorIs(err, storeErr)
}

// GetOAuthProfileByEntityID — cache hit.
func (suite *CacheBackedStoreTestSuite) TestGetOAuthProfileByEntityID_CacheHit() {
	ctx := context.Background()
	cached := &inboundmodel.OAuthProfile{GrantTypes: []string{"authorization_code"}}
	suite.profileCache.EXPECT().Get(mock.Anything, cache.CacheKey{Key: "e1"}).Return(cached, true)

	got, err := suite.cachedStore.GetOAuthProfileByEntityID(ctx, "e1")
	suite.NoError(err)
	suite.Equal(cached, got)
}

// GetOAuthProfileByEntityID — cache miss, fetches inner and caches.
func (suite *CacheBackedStoreTestSuite) TestGetOAuthProfileByEntityID_CacheMiss() {
	ctx := context.Background()
	var nilProfile *inboundmodel.OAuthProfile
	suite.profileCache.EXPECT().Get(mock.Anything, cache.CacheKey{Key: "e1"}).Return(nilProfile, false)
	inner := &inboundmodel.OAuthProfile{GrantTypes: []string{"authorization_code"}}
	suite.mockStore.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "e1").Return(inner, nil)
	suite.profileCache.EXPECT().Set(mock.Anything, cache.CacheKey{Key: "e1"}, inner).Return(nil)

	got, err := suite.cachedStore.GetOAuthProfileByEntityID(ctx, "e1")
	suite.NoError(err)
	suite.Equal(inner, got)
}

// GetOAuthProfileByEntityID — cache miss + inner error.
func (suite *CacheBackedStoreTestSuite) TestGetOAuthProfileByEntityID_InnerError() {
	ctx := context.Background()
	var nilProfile *inboundmodel.OAuthProfile
	suite.profileCache.EXPECT().Get(mock.Anything, cache.CacheKey{Key: "e1"}).Return(nilProfile, false)
	storeErr := errors.New("db error")
	suite.mockStore.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "e1").Return(nil, storeErr)

	got, err := suite.cachedStore.GetOAuthProfileByEntityID(ctx, "e1")
	suite.Nil(got)
	suite.ErrorIs(err, storeErr)
}

// GetInboundClientList — delegates to inner.
func (suite *CacheBackedStoreTestSuite) TestGetInboundClientList_Delegates() {
	ctx := context.Background()
	list := []inboundmodel.InboundClient{{ID: "c1"}}
	suite.mockStore.EXPECT().GetInboundClientList(mock.Anything, 10).Return(list, nil)

	got, err := suite.cachedStore.GetInboundClientList(ctx, 10)
	suite.NoError(err)
	suite.Len(got, 1)
}

// GetTotalInboundClientCount — delegates to inner.
func (suite *CacheBackedStoreTestSuite) TestGetTotalInboundClientCount_Delegates() {
	ctx := context.Background()
	suite.mockStore.EXPECT().GetTotalInboundClientCount(mock.Anything).Return(5, nil)

	count, err := suite.cachedStore.GetTotalInboundClientCount(ctx)
	suite.NoError(err)
	suite.Equal(5, count)
}

// UpdateInboundClient — inner succeeds, invalidates and re-caches.
func (suite *CacheBackedStoreTestSuite) TestUpdateInboundClient_InvalidatesAndRecaches() {
	ctx := context.Background()
	client := inboundmodel.InboundClient{ID: "c1"}
	suite.mockStore.EXPECT().UpdateInboundClient(mock.Anything, client).Return(nil)
	suite.clientCache.EXPECT().Delete(mock.Anything, cache.CacheKey{Key: "c1"}).Return(nil)
	suite.clientCache.EXPECT().Set(mock.Anything, cache.CacheKey{Key: "c1"}, &client).Return(nil)

	err := suite.cachedStore.UpdateInboundClient(ctx, client)
	suite.NoError(err)
}

// UpdateInboundClient — inner fails.
func (suite *CacheBackedStoreTestSuite) TestUpdateInboundClient_InnerError() {
	ctx := context.Background()
	client := inboundmodel.InboundClient{ID: "c1"}
	storeErr := errors.New("update failed")
	suite.mockStore.EXPECT().UpdateInboundClient(mock.Anything, client).Return(storeErr)

	err := suite.cachedStore.UpdateInboundClient(ctx, client)
	suite.ErrorIs(err, storeErr)
}

// UpdateOAuthProfile — inner succeeds, invalidates OAuth cache.
func (suite *CacheBackedStoreTestSuite) TestUpdateOAuthProfile_InvalidatesCache() {
	ctx := context.Background()
	p := &inboundmodel.OAuthProfile{}
	suite.mockStore.EXPECT().UpdateOAuthProfile(mock.Anything, "e1", p).Return(nil)
	suite.profileCache.EXPECT().Delete(mock.Anything, cache.CacheKey{Key: "e1"}).Return(nil)

	err := suite.cachedStore.UpdateOAuthProfile(ctx, "e1", p)
	suite.NoError(err)
}

// UpdateOAuthProfile — inner fails.
func (suite *CacheBackedStoreTestSuite) TestUpdateOAuthProfile_InnerError() {
	ctx := context.Background()
	p := &inboundmodel.OAuthProfile{}
	storeErr := errors.New("update failed")
	suite.mockStore.EXPECT().UpdateOAuthProfile(mock.Anything, "e1", p).Return(storeErr)

	err := suite.cachedStore.UpdateOAuthProfile(ctx, "e1", p)
	suite.ErrorIs(err, storeErr)
}

// DeleteInboundClient — invalidates both caches.
func (suite *CacheBackedStoreTestSuite) TestDeleteInboundClient_InvalidatesBothCaches() {
	ctx := context.Background()
	suite.mockStore.EXPECT().DeleteInboundClient(mock.Anything, "c1").Return(nil)
	suite.clientCache.EXPECT().Delete(mock.Anything, cache.CacheKey{Key: "c1"}).Return(nil)
	suite.profileCache.EXPECT().Delete(mock.Anything, cache.CacheKey{Key: "c1"}).Return(nil)

	err := suite.cachedStore.DeleteInboundClient(ctx, "c1")
	suite.NoError(err)
}

// DeleteInboundClient — inner fails.
func (suite *CacheBackedStoreTestSuite) TestDeleteInboundClient_InnerError() {
	ctx := context.Background()
	storeErr := errors.New("delete failed")
	suite.mockStore.EXPECT().DeleteInboundClient(mock.Anything, "c1").Return(storeErr)

	err := suite.cachedStore.DeleteInboundClient(ctx, "c1")
	suite.ErrorIs(err, storeErr)
}

// DeleteOAuthProfile — invalidates OAuth cache only.
func (suite *CacheBackedStoreTestSuite) TestDeleteOAuthProfile_InvalidatesOAuthCache() {
	ctx := context.Background()
	suite.mockStore.EXPECT().DeleteOAuthProfile(mock.Anything, "e1").Return(nil)
	suite.profileCache.EXPECT().Delete(mock.Anything, cache.CacheKey{Key: "e1"}).Return(nil)

	err := suite.cachedStore.DeleteOAuthProfile(ctx, "e1")
	suite.NoError(err)
}

// DeleteOAuthProfile — inner fails.
func (suite *CacheBackedStoreTestSuite) TestDeleteOAuthProfile_InnerError() {
	ctx := context.Background()
	storeErr := errors.New("delete failed")
	suite.mockStore.EXPECT().DeleteOAuthProfile(mock.Anything, "e1").Return(storeErr)

	err := suite.cachedStore.DeleteOAuthProfile(ctx, "e1")
	suite.ErrorIs(err, storeErr)
}

// InboundClientExists — delegates to inner.
func (suite *CacheBackedStoreTestSuite) TestInboundClientExists_Delegates() {
	ctx := context.Background()
	suite.mockStore.EXPECT().InboundClientExists(mock.Anything, "c1").Return(true, nil)

	exists, err := suite.cachedStore.InboundClientExists(ctx, "c1")
	suite.NoError(err)
	suite.True(exists)
}

// IsDeclarative — delegates to inner.
func (suite *CacheBackedStoreTestSuite) TestIsDeclarative_Delegates() {
	ctx := context.Background()
	suite.mockStore.EXPECT().IsDeclarative(mock.Anything, "c1").Return(true)

	suite.True(suite.cachedStore.IsDeclarative(ctx, "c1"))
}

// ----- cache helper edge cases -----

func (suite *CacheBackedStoreTestSuite) TestCacheInboundClient_NilNoOp() {
	suite.cachedStore.cacheInboundClient(context.Background(), nil)
}

func (suite *CacheBackedStoreTestSuite) TestCacheInboundClient_EmptyIDNoOp() {
	suite.cachedStore.cacheInboundClient(context.Background(), &inboundmodel.InboundClient{})
}

func (suite *CacheBackedStoreTestSuite) TestCacheInboundClient_LogsOnSetError() {
	client := &inboundmodel.InboundClient{ID: "c1"}
	suite.clientCache.EXPECT().Set(mock.Anything, cache.CacheKey{Key: "c1"}, client).
		Return(errors.New("cache down"))
	suite.cachedStore.cacheInboundClient(context.Background(), client)
}

func (suite *CacheBackedStoreTestSuite) TestCacheOAuthProfile_NilNoOp() {
	suite.cachedStore.cacheOAuthProfile(context.Background(), "a1", nil)
}

func (suite *CacheBackedStoreTestSuite) TestCacheOAuthProfile_EmptyAppIDNoOp() {
	suite.cachedStore.cacheOAuthProfile(context.Background(), "", &inboundmodel.OAuthProfile{})
}

func (suite *CacheBackedStoreTestSuite) TestCacheOAuthProfile_LogsOnSetError() {
	profile := &inboundmodel.OAuthProfile{GrantTypes: []string{"authorization_code"}}
	suite.profileCache.EXPECT().Set(mock.Anything, cache.CacheKey{Key: "a1"}, profile).
		Return(errors.New("cache down"))
	suite.cachedStore.cacheOAuthProfile(context.Background(), "a1", profile)
}

func (suite *CacheBackedStoreTestSuite) TestInvalidateInboundClient_EmptyIDNoOp() {
	suite.cachedStore.invalidateInboundClient(context.Background(), "")
}

func (suite *CacheBackedStoreTestSuite) TestInvalidateInboundClient_LogsOnDeleteError() {
	suite.clientCache.EXPECT().Delete(mock.Anything, cache.CacheKey{Key: "x"}).
		Return(errors.New("cache down"))
	suite.cachedStore.invalidateInboundClient(context.Background(), "x")
}

func (suite *CacheBackedStoreTestSuite) TestInvalidateOAuthProfile_EmptyIDNoOp() {
	suite.cachedStore.invalidateOAuthProfile(context.Background(), "")
}

func (suite *CacheBackedStoreTestSuite) TestInvalidateOAuthProfile_LogsOnDeleteError() {
	suite.profileCache.EXPECT().Delete(mock.Anything, cache.CacheKey{Key: "y"}).
		Return(errors.New("cache down"))
	suite.cachedStore.invalidateOAuthProfile(context.Background(), "y")
}
