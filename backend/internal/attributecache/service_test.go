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

package attributecache

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// AttributeCacheServiceTestSuite is the test suite for the attribute cache service.
type AttributeCacheServiceTestSuite struct {
	suite.Suite
	service   AttributeCacheServiceInterface
	mockStore *attributeCacheStoreInterfaceMock
	ctx       context.Context
	testCache AttributeCache
}

func TestAttributeCacheServiceSuite(t *testing.T) {
	suite.Run(t, new(AttributeCacheServiceTestSuite))
}

func (suite *AttributeCacheServiceTestSuite) SetupTest() {
	suite.mockStore = newAttributeCacheStoreInterfaceMock(suite.T())
	suite.service = newAttributeCacheService(suite.mockStore)
	suite.ctx = context.Background()

	suite.testCache = AttributeCache{
		ID:         "test-cache-id",
		Attributes: map[string]interface{}{"key": "value"},
		TTLSeconds: 3600, // 1 hour
	}
}

// Tests for CreateAttributeCache

func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_Success() {
	cache := &AttributeCache{
		Attributes: map[string]interface{}{"user": "john", "role": "admin"},
		TTLSeconds: 3600,
	}

	suite.mockStore.On("CreateAttributeCache", suite.ctx, mock.MatchedBy(func(c AttributeCache) bool {
		return c.ID != "" && len(c.Attributes) > 0 && c.TTLSeconds == 3600
	})).Return(nil).Once()

	result, err := suite.service.CreateAttributeCache(suite.ctx, cache)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.NotEmpty(suite.T(), result.ID, "ID should be generated")
	assert.Equal(suite.T(), cache.Attributes, result.Attributes)
	assert.Equal(suite.T(), cache.TTLSeconds, result.TTLSeconds)
}

func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_NilCache() {
	result, err := suite.service.CreateAttributeCache(suite.ctx, nil)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidRequestFormat.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_MissingAttributes() {
	cache := &AttributeCache{
		Attributes: map[string]interface{}{}, // Empty attributes
		TTLSeconds: 3600,
	}

	result, err := suite.service.CreateAttributeCache(suite.ctx, cache)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingAttributes.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_ZeroTTL() {
	cache := &AttributeCache{
		Attributes: map[string]interface{}{"key": "value"},
		TTLSeconds: 0, // Zero TTL
	}

	result, err := suite.service.CreateAttributeCache(suite.ctx, cache)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidExpiryTime.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_NegativeTTL() {
	cache := &AttributeCache{
		Attributes: map[string]interface{}{"key": "value"},
		TTLSeconds: -100,
	}

	result, err := suite.service.CreateAttributeCache(suite.ctx, cache)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidExpiryTime.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_StoreError() {
	cache := &AttributeCache{
		Attributes: map[string]interface{}{"key": "value"},
		TTLSeconds: 3600,
	}

	suite.mockStore.On("CreateAttributeCache", suite.ctx, mock.Anything).
		Return(errors.New("database error")).Once()

	result, err := suite.service.CreateAttributeCache(suite.ctx, cache)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
}

// Tests for GetAttributeCache

func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_Success() {
	suite.mockStore.On("GetAttributeCache", suite.ctx, suite.testCache.ID).
		Return(suite.testCache, nil).Once()

	result, err := suite.service.GetAttributeCache(suite.ctx, suite.testCache.ID)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), suite.testCache.ID, result.ID)
	assert.Equal(suite.T(), suite.testCache.Attributes, result.Attributes)
	assert.Equal(suite.T(), suite.testCache.TTLSeconds, result.TTLSeconds)
}

func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_EmptyID() {
	result, err := suite.service.GetAttributeCache(suite.ctx, "")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingCacheID.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_WhitespaceID() {
	result, err := suite.service.GetAttributeCache(suite.ctx, "   ")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingCacheID.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_NotFound() {
	suite.mockStore.On("GetAttributeCache", suite.ctx, "non-existent-id").
		Return(AttributeCache{}, errAttributeCacheNotFound).Once()

	result, err := suite.service.GetAttributeCache(suite.ctx, "non-existent-id")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorAttributeCacheNotFound.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_StoreError() {
	suite.mockStore.On("GetAttributeCache", suite.ctx, suite.testCache.ID).
		Return(AttributeCache{}, errors.New("database error")).Once()

	result, err := suite.service.GetAttributeCache(suite.ctx, suite.testCache.ID)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
}

// Tests for ExtendAttributeCacheTTL

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_Success() {
	newTTL := 7200 // 2 hours

	suite.mockStore.On("ExtendAttributeCacheTTL", suite.ctx, suite.testCache.ID, newTTL).
		Return(nil).Once()

	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, suite.testCache.ID, newTTL)

	assert.Nil(suite.T(), err)
}

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_EmptyID() {
	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, "", 3600)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingCacheID.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_WhitespaceID() {
	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, "  ", 3600)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingCacheID.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_ZeroTTL() {
	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, suite.testCache.ID, 0)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidExpiryTime.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_NegativeTTL() {
	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, suite.testCache.ID, -100)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidExpiryTime.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_NotFound() {
	suite.mockStore.On("ExtendAttributeCacheTTL", suite.ctx, "non-existent-id", 3600).
		Return(errAttributeCacheNotFound).Once()

	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, "non-existent-id", 3600)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorAttributeCacheNotFound.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_StoreUpdateError() {
	suite.mockStore.On("ExtendAttributeCacheTTL", suite.ctx, suite.testCache.ID, 3600).
		Return(errors.New("database error")).Once()

	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, suite.testCache.ID, 3600)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
}

// Tests for DeleteAttributeCache

func (suite *AttributeCacheServiceTestSuite) TestDeleteAttributeCache_Success() {
	suite.mockStore.On("DeleteAttributeCache", suite.ctx, suite.testCache.ID).
		Return(nil).Once()

	err := suite.service.DeleteAttributeCache(suite.ctx, suite.testCache.ID)

	assert.Nil(suite.T(), err)
}

func (suite *AttributeCacheServiceTestSuite) TestDeleteAttributeCache_EmptyID() {
	err := suite.service.DeleteAttributeCache(suite.ctx, "")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingCacheID.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestDeleteAttributeCache_WhitespaceID() {
	err := suite.service.DeleteAttributeCache(suite.ctx, "   ")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingCacheID.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestDeleteAttributeCache_NotFound() {
	suite.mockStore.On("DeleteAttributeCache", suite.ctx, "non-existent-id").
		Return(errAttributeCacheNotFound).Once()

	err := suite.service.DeleteAttributeCache(suite.ctx, "non-existent-id")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorAttributeCacheNotFound.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestDeleteAttributeCache_StoreError() {
	suite.mockStore.On("DeleteAttributeCache", suite.ctx, suite.testCache.ID).
		Return(errors.New("database error")).Once()

	err := suite.service.DeleteAttributeCache(suite.ctx, suite.testCache.ID)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
}
