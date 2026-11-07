// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
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

func (suite *CacheBackedStoreTestSuite) TestGetServerConfig_CacheHit() {
	suite.mockCache.EXPECT().Get(mock.Anything, corsCacheKey).
		Return(&ServerConfig{Name: ConfigNameCORS, Value: corsValue}, true)

	layers, err := suite.cached.GetServerConfig(suite.ctx, ConfigNameCORS)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), corsValue, layers.Writable)
	suite.mockInner.AssertNotCalled(suite.T(), "GetServerConfig", mock.Anything, mock.Anything)
}

func (suite *CacheBackedStoreTestSuite) TestGetServerConfig_CacheMiss_PopulatesCache() {
	suite.mockCache.EXPECT().Get(mock.Anything, corsCacheKey).Return(nil, false)
	suite.mockInner.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{Writable: corsValue}, nil)
	suite.mockCache.EXPECT().Set(mock.Anything, corsCacheKey, mock.Anything).Return(nil)

	layers, err := suite.cached.GetServerConfig(suite.ctx, ConfigNameCORS)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), corsValue, layers.Writable)
}

func (suite *CacheBackedStoreTestSuite) TestGetServerConfig_MissUnset_CachesNegativeLookup() {
	suite.mockCache.EXPECT().Get(mock.Anything, corsCacheKey).Return(nil, false)
	suite.mockInner.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)
	suite.mockCache.EXPECT().Set(mock.Anything, corsCacheKey, mock.Anything).Return(nil)

	layers, err := suite.cached.GetServerConfig(suite.ctx, ConfigNameCORS)
	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), layers.Writable)
}

func (suite *CacheBackedStoreTestSuite) TestGetServerConfig_InnerError() {
	suite.mockCache.EXPECT().Get(mock.Anything, corsCacheKey).Return(nil, false)
	suite.mockInner.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{}, errors.New("db error"))

	_, err := suite.cached.GetServerConfig(suite.ctx, ConfigNameCORS)
	assert.Error(suite.T(), err)
	suite.mockCache.AssertNotCalled(suite.T(), "Set", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *CacheBackedStoreTestSuite) TestGetServerConfig_CacheSetError_StillReturns() {
	suite.mockCache.EXPECT().Get(mock.Anything, corsCacheKey).Return(nil, false)
	suite.mockInner.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{Writable: corsValue}, nil)
	suite.mockCache.EXPECT().Set(mock.Anything, corsCacheKey, mock.Anything).Return(errors.New("cache error"))

	layers, err := suite.cached.GetServerConfig(suite.ctx, ConfigNameCORS)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), corsValue, layers.Writable)
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

func (suite *CacheBackedStoreTestSuite) TestUpsertServerConfig_CacheDeleteError_StillSucceeds() {
	cfg := ServerConfig{Name: ConfigNameCORS, Value: corsValue}
	suite.mockInner.EXPECT().UpsertServerConfig(mock.Anything, cfg).Return(nil)
	suite.mockCache.EXPECT().Delete(mock.Anything, corsCacheKey).Return(errors.New("cache error"))

	assert.NoError(suite.T(), suite.cached.UpsertServerConfig(suite.ctx, cfg))
}

func (suite *CacheBackedStoreTestSuite) TestNewCachedBackStore() {
	assert.NotNil(suite.T(), newCachedBackStore(suite.mockInner, suite.mockCache))
}

// A conditional write drops the cached layer whether or not it wrote, so a lost write reads the database
// next.
func (suite *CacheBackedStoreTestSuite) TestReplaceServerConfigIf_InvalidatesKey() {
	cfg := ServerConfig{Name: ConfigNameCORS, Value: corsValue}
	suite.mockInner.EXPECT().ReplaceServerConfigIf(mock.Anything, cfg, json.RawMessage(nil)).Return(false, nil)
	suite.mockCache.EXPECT().Delete(mock.Anything, corsCacheKey).Return(nil)

	written, err := suite.cached.ReplaceServerConfigIf(suite.ctx, cfg, nil)

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), written)
}
