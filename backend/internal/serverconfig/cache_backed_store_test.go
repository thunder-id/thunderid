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

package serverconfig

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/tests/mocks/cachemock"
)

type CacheBackedStoreTestSuite struct {
	suite.Suite
	ctx       context.Context
	mockInner *serverConfigStoreInterfaceMock
	mockCache *cachemock.CacheInterfaceMock[*ServerConfig]
	cached    serverConfigStoreInterface
}

func TestCacheBackedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CacheBackedStoreTestSuite))
}

func (suite *CacheBackedStoreTestSuite) SetupTest() {
	suite.ctx = context.Background()
	suite.mockInner = newServerConfigStoreInterfaceMock(suite.T())
	suite.mockCache = cachemock.NewCacheInterfaceMock[*ServerConfig](suite.T())
	suite.cached = &cachedBackStore{configCache: suite.mockCache, inner: suite.mockInner}
}

var corsCacheKey = cache.CacheKey{Key: string(ConfigNameCORS)}

func (suite *CacheBackedStoreTestSuite) TestGetServerConfigByName_CacheHit() {
	cached := &ServerConfig{Name: ConfigNameCORS, Value: corsValue}
	suite.mockCache.EXPECT().Get(mock.Anything, corsCacheKey).Return(cached, true)

	got, err := suite.cached.GetServerConfigByName(suite.ctx, ConfigNameCORS)
	assert.NoError(suite.T(), err)
	assert.Same(suite.T(), cached, got)
	suite.mockInner.AssertNotCalled(suite.T(), "GetServerConfigByName", mock.Anything, mock.Anything)
}

func (suite *CacheBackedStoreTestSuite) TestGetServerConfigByName_CacheMiss_PopulatesCache() {
	inner := &ServerConfig{Name: ConfigNameCORS, Value: corsValue}
	suite.mockCache.EXPECT().Get(mock.Anything, corsCacheKey).Return(nil, false)
	suite.mockInner.EXPECT().GetServerConfigByName(mock.Anything, ConfigNameCORS).Return(inner, nil)
	suite.mockCache.EXPECT().Set(mock.Anything, corsCacheKey, inner).Return(nil)

	got, err := suite.cached.GetServerConfigByName(suite.ctx, ConfigNameCORS)
	assert.NoError(suite.T(), err)
	assert.Same(suite.T(), inner, got)
}

func (suite *CacheBackedStoreTestSuite) TestGetServerConfigByName_MissNotFound_SkipsCache() {
	suite.mockCache.EXPECT().Get(mock.Anything, corsCacheKey).Return(nil, false)
	suite.mockInner.EXPECT().GetServerConfigByName(mock.Anything, ConfigNameCORS).Return(nil, nil)

	got, err := suite.cached.GetServerConfigByName(suite.ctx, ConfigNameCORS)
	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), got)
	suite.mockCache.AssertNotCalled(suite.T(), "Set", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *CacheBackedStoreTestSuite) TestGetServerConfigByName_InnerError() {
	suite.mockCache.EXPECT().Get(mock.Anything, corsCacheKey).Return(nil, false)
	suite.mockInner.EXPECT().GetServerConfigByName(mock.Anything, ConfigNameCORS).
		Return(nil, errors.New("db error"))

	got, err := suite.cached.GetServerConfigByName(suite.ctx, ConfigNameCORS)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), got)
	suite.mockCache.AssertNotCalled(suite.T(), "Set", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *CacheBackedStoreTestSuite) TestGetServerConfigList_PassThrough() {
	list := []ServerConfig{{Name: ConfigNameCORS, Value: corsValue}}
	suite.mockInner.EXPECT().GetServerConfigList(mock.Anything).Return(list, nil)

	got, err := suite.cached.GetServerConfigList(suite.ctx)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), list, got)
}

func (suite *CacheBackedStoreTestSuite) TestUpsertServerConfig_InvalidatesKey() {
	cfg := ServerConfig{Name: ConfigNameCORS, Value: corsValue}
	suite.mockInner.EXPECT().UpsertServerConfig(mock.Anything, cfg).Return(nil)
	suite.mockCache.EXPECT().Delete(mock.Anything, corsCacheKey).Return(nil)

	assert.NoError(suite.T(), suite.cached.UpsertServerConfig(suite.ctx, cfg))
}

func (suite *CacheBackedStoreTestSuite) TestUpsertServerConfig_InnerError_NoInvalidate() {
	cfg := ServerConfig{Name: ConfigNameCORS, Value: corsValue}
	suite.mockInner.EXPECT().UpsertServerConfig(mock.Anything, cfg).Return(errors.New("db error"))

	assert.Error(suite.T(), suite.cached.UpsertServerConfig(suite.ctx, cfg))
	suite.mockCache.AssertNotCalled(suite.T(), "Delete", mock.Anything, mock.Anything)
}

func (suite *CacheBackedStoreTestSuite) TestUpsertServerConfigs_InvalidatesEachKey() {
	cfgs := []ServerConfig{{Name: ConfigNameCORS, Value: corsValue}}
	suite.mockInner.EXPECT().UpsertServerConfigs(mock.Anything, cfgs).Return(nil)
	suite.mockCache.EXPECT().Delete(mock.Anything, corsCacheKey).Return(nil)

	assert.NoError(suite.T(), suite.cached.UpsertServerConfigs(suite.ctx, cfgs))
}

func (suite *CacheBackedStoreTestSuite) TestUpsertServerConfigs_InnerError_NoInvalidate() {
	cfgs := []ServerConfig{{Name: ConfigNameCORS, Value: corsValue}}
	suite.mockInner.EXPECT().UpsertServerConfigs(mock.Anything, cfgs).Return(errors.New("db error"))

	assert.Error(suite.T(), suite.cached.UpsertServerConfigs(suite.ctx, cfgs))
	suite.mockCache.AssertNotCalled(suite.T(), "Delete", mock.Anything, mock.Anything)
}

func (suite *CacheBackedStoreTestSuite) TestDeleteServerConfig_InnerError_NoInvalidate() {
	suite.mockInner.EXPECT().DeleteServerConfig(mock.Anything, ConfigNameCORS).Return(errors.New("db error"))

	assert.Error(suite.T(), suite.cached.DeleteServerConfig(suite.ctx, ConfigNameCORS))
	suite.mockCache.AssertNotCalled(suite.T(), "Delete", mock.Anything, mock.Anything)
}

func (suite *CacheBackedStoreTestSuite) TestDeleteServerConfig_InvalidatesKey() {
	suite.mockInner.EXPECT().DeleteServerConfig(mock.Anything, ConfigNameCORS).Return(nil)
	suite.mockCache.EXPECT().Delete(mock.Anything, corsCacheKey).Return(nil)

	assert.NoError(suite.T(), suite.cached.DeleteServerConfig(suite.ctx, ConfigNameCORS))
}

func (suite *CacheBackedStoreTestSuite) TestNewCachedBackStore() {
	assert.NotNil(suite.T(), newCachedBackStore(suite.mockInner, suite.mockCache))
}

func (suite *CacheBackedStoreTestSuite) TestGetServerConfigByName_CacheSetError_StillReturns() {
	inner := &ServerConfig{Name: ConfigNameCORS, Value: corsValue}
	suite.mockCache.EXPECT().Get(mock.Anything, corsCacheKey).Return(nil, false)
	suite.mockInner.EXPECT().GetServerConfigByName(mock.Anything, ConfigNameCORS).Return(inner, nil)
	suite.mockCache.EXPECT().Set(mock.Anything, corsCacheKey, inner).Return(errors.New("cache error"))

	got, err := suite.cached.GetServerConfigByName(suite.ctx, ConfigNameCORS)
	assert.NoError(suite.T(), err)
	assert.Same(suite.T(), inner, got)
}

func (suite *CacheBackedStoreTestSuite) TestUpsertServerConfig_CacheDeleteError_StillSucceeds() {
	cfg := ServerConfig{Name: ConfigNameCORS, Value: corsValue}
	suite.mockInner.EXPECT().UpsertServerConfig(mock.Anything, cfg).Return(nil)
	suite.mockCache.EXPECT().Delete(mock.Anything, corsCacheKey).Return(errors.New("cache error"))

	assert.NoError(suite.T(), suite.cached.UpsertServerConfig(suite.ctx, cfg))
}
