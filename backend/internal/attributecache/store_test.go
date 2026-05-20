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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

// AttributeCacheStoreTestSuite is the test suite for the attribute cache store.
type AttributeCacheStoreTestSuite struct {
	suite.Suite
	store            *attributeCacheStore
	mockDBProvider   *providermock.DBProviderInterfaceMock
	mockDBClient     *providermock.DBClientInterfaceMock
	ctx              context.Context
	testCache        AttributeCache
	futureTime       time.Time
	testDeploymentID string
}

func TestAttributeCacheStoreSuite(t *testing.T) {
	suite.Run(t, new(AttributeCacheStoreTestSuite))
}

func (suite *AttributeCacheStoreTestSuite) SetupTest() {
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.ctx = context.Background()
	suite.testDeploymentID = "test-deployment-id"
	suite.futureTime = time.Now().Add(1 * time.Hour)

	suite.testCache = AttributeCache{
		ID:         "test-cache-id",
		Attributes: map[string]interface{}{"key": "value"},
		TTLSeconds: 3600, // 1 hour
	}

	suite.store = &attributeCacheStore{
		dbProvider:   suite.mockDBProvider,
		deploymentID: suite.testDeploymentID,
	}
}

// Tests for CreateAttributeCache

func (suite *AttributeCacheStoreTestSuite) TestCreateAttributeCache_Success() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("ExecuteContext", suite.ctx, queryInsertAttributeCache,
		suite.testCache.ID, `{"key":"value"}`,
		mock.MatchedBy(func(t time.Time) bool {
			return !t.IsZero() && t.After(time.Now())
		}), mock.MatchedBy(func(t time.Time) bool {
			return !t.IsZero()
		}), suite.testDeploymentID).Return(int64(1), nil).Once()

	err := suite.store.CreateAttributeCache(suite.ctx, suite.testCache)

	assert.Nil(suite.T(), err)
}

func (suite *AttributeCacheStoreTestSuite) TestCreateAttributeCache_DBProviderError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db provider error")).Once()

	err := suite.store.CreateAttributeCache(suite.ctx, suite.testCache)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to get database client")
}

func (suite *AttributeCacheStoreTestSuite) TestCreateAttributeCache_ExecuteError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("ExecuteContext", suite.ctx, queryInsertAttributeCache,
		suite.testCache.ID, `{"key":"value"}`,
		mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"), suite.testDeploymentID).
		Return(int64(0), errors.New("database error")).Once()

	err := suite.store.CreateAttributeCache(suite.ctx, suite.testCache)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to insert attribute cache")
}

func (suite *AttributeCacheStoreTestSuite) TestCreateAttributeCache_NoRowsAffected() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("ExecuteContext", suite.ctx, queryInsertAttributeCache,
		suite.testCache.ID, `{"key":"value"}`,
		mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"), suite.testDeploymentID).
		Return(int64(0), nil).Once()

	err := suite.store.CreateAttributeCache(suite.ctx, suite.testCache)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no rows affected")
}

// Tests for GetAttributeCache

func (suite *AttributeCacheStoreTestSuite) TestGetAttributeCache_Success() {
	resultRow := map[string]interface{}{
		"id":          suite.testCache.ID,
		"attributes":  `{"key":"value"}`,
		"expiry_time": suite.futureTime,
	}

	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("QueryContext", suite.ctx, queryGetAttributeCache,
		suite.testCache.ID, suite.testDeploymentID).
		Return([]map[string]interface{}{resultRow}, nil).Once()

	result, err := suite.store.GetAttributeCache(suite.ctx, suite.testCache.ID)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), suite.testCache.ID, result.ID)
	assert.Equal(suite.T(), suite.testCache.Attributes, result.Attributes)
	// TTL should be approximately 3600 seconds (allowing for small time differences)
	assert.Greater(suite.T(), result.TTLSeconds, 3500)
	assert.Less(suite.T(), result.TTLSeconds, 3700)
}

func (suite *AttributeCacheStoreTestSuite) TestGetAttributeCache_SuccessWithBytesAttributes() {
	resultRow := map[string]interface{}{
		"id":          suite.testCache.ID,
		"attributes":  []byte(`{"key":"value"}`),
		"expiry_time": suite.futureTime,
	}

	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("QueryContext", suite.ctx, queryGetAttributeCache,
		suite.testCache.ID, suite.testDeploymentID).
		Return([]map[string]interface{}{resultRow}, nil).Once()

	result, err := suite.store.GetAttributeCache(suite.ctx, suite.testCache.ID)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), suite.testCache.ID, result.ID)
	assert.Equal(suite.T(), suite.testCache.Attributes, result.Attributes)
	assert.Greater(suite.T(), result.TTLSeconds, 3500)
	assert.Less(suite.T(), result.TTLSeconds, 3700)
}

func (suite *AttributeCacheStoreTestSuite) TestGetAttributeCache_DBProviderError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db provider error")).Once()

	result, err := suite.store.GetAttributeCache(suite.ctx, suite.testCache.ID)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to get database client")
	assert.Equal(suite.T(), AttributeCache{}, result)
}

func (suite *AttributeCacheStoreTestSuite) TestGetAttributeCache_QueryError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("QueryContext", suite.ctx, queryGetAttributeCache,
		suite.testCache.ID, suite.testDeploymentID).
		Return(nil, errors.New("query error")).Once()

	result, err := suite.store.GetAttributeCache(suite.ctx, suite.testCache.ID)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to execute query")
	assert.Equal(suite.T(), AttributeCache{}, result)
}

func (suite *AttributeCacheStoreTestSuite) TestGetAttributeCache_NotFound() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("QueryContext", suite.ctx, queryGetAttributeCache,
		suite.testCache.ID, suite.testDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()

	result, err := suite.store.GetAttributeCache(suite.ctx, suite.testCache.ID)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), errAttributeCacheNotFound, err)
	assert.Equal(suite.T(), AttributeCache{}, result)
}

func (suite *AttributeCacheStoreTestSuite) TestGetAttributeCache_MultipleResults() {
	resultRow1 := map[string]interface{}{
		"id":          suite.testCache.ID,
		"attributes":  `{"key":"value"}`,
		"expiry_time": suite.futureTime,
	}
	resultRow2 := map[string]interface{}{
		"id":          suite.testCache.ID,
		"attributes":  `{"key":"value"}`,
		"expiry_time": suite.futureTime,
	}

	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("QueryContext", suite.ctx, queryGetAttributeCache,
		suite.testCache.ID, suite.testDeploymentID).
		Return([]map[string]interface{}{resultRow1, resultRow2}, nil).Once()

	result, err := suite.store.GetAttributeCache(suite.ctx, suite.testCache.ID)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "multiple attribute cache entries found")
	assert.Equal(suite.T(), AttributeCache{}, result)
}

func (suite *AttributeCacheStoreTestSuite) TestGetAttributeCache_InvalidIDType() {
	resultRow := map[string]interface{}{
		"id":          12345, // Invalid type: int instead of string
		"attributes":  `{"key":"value"}`,
		"expiry_time": suite.futureTime,
	}

	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("QueryContext", suite.ctx, queryGetAttributeCache,
		suite.testCache.ID, suite.testDeploymentID).
		Return([]map[string]interface{}{resultRow}, nil).Once()

	result, err := suite.store.GetAttributeCache(suite.ctx, suite.testCache.ID)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to parse id as string")
	assert.Equal(suite.T(), AttributeCache{}, result)
}

func (suite *AttributeCacheStoreTestSuite) TestGetAttributeCache_InvalidAttributesType() {
	resultRow := map[string]interface{}{
		"id":          suite.testCache.ID,
		"attributes":  12345, // Invalid type: int instead of string
		"expiry_time": suite.futureTime,
	}

	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("QueryContext", suite.ctx, queryGetAttributeCache,
		suite.testCache.ID, suite.testDeploymentID).
		Return([]map[string]interface{}{resultRow}, nil).Once()

	result, err := suite.store.GetAttributeCache(suite.ctx, suite.testCache.ID)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to parse attributes: expected string or []byte")
	assert.Equal(suite.T(), AttributeCache{}, result)
}

func (suite *AttributeCacheStoreTestSuite) TestGetAttributeCache_InvalidExpiryTimeType() {
	resultRow := map[string]interface{}{
		"id":          suite.testCache.ID,
		"attributes":  `{"key":"value"}`,
		"expiry_time": "not-a-time", // Invalid type: string instead of time.Time
	}

	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("QueryContext", suite.ctx, queryGetAttributeCache,
		suite.testCache.ID, suite.testDeploymentID).
		Return([]map[string]interface{}{resultRow}, nil).Once()

	result, err := suite.store.GetAttributeCache(suite.ctx, suite.testCache.ID)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "error parsing expiry_time")
	assert.Equal(suite.T(), AttributeCache{}, result)
}

// Tests for ExtendAttributeCacheTTL

func (suite *AttributeCacheStoreTestSuite) TestExtendAttributeCacheTTL_Success() {
	newTTL := 7200 // 2 hours

	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("ExecuteContext", suite.ctx, queryUpdateAttributeCacheExpiry,
		suite.testCache.ID, mock.MatchedBy(func(t time.Time) bool {
			return !t.IsZero() && t.After(time.Now())
		}), suite.testDeploymentID).Return(int64(1), nil).Once()

	err := suite.store.ExtendAttributeCacheTTL(suite.ctx, suite.testCache.ID, newTTL)

	assert.Nil(suite.T(), err)
}

func (suite *AttributeCacheStoreTestSuite) TestExtendAttributeCacheTTL_DBProviderError() {
	newTTL := 7200

	suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db provider error")).Once()

	err := suite.store.ExtendAttributeCacheTTL(suite.ctx, suite.testCache.ID, newTTL)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to get database client")
}

func (suite *AttributeCacheStoreTestSuite) TestExtendAttributeCacheTTL_ExecuteError() {
	newTTL := 7200

	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("ExecuteContext", suite.ctx, queryUpdateAttributeCacheExpiry,
		suite.testCache.ID, mock.AnythingOfType("time.Time"), suite.testDeploymentID).
		Return(int64(0), errors.New("database error")).Once()

	err := suite.store.ExtendAttributeCacheTTL(suite.ctx, suite.testCache.ID, newTTL)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to update attribute cache expiry")
}

func (suite *AttributeCacheStoreTestSuite) TestExtendAttributeCacheTTL_NoRowsAffected() {
	newTTL := 7200

	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("ExecuteContext", suite.ctx, queryUpdateAttributeCacheExpiry,
		suite.testCache.ID, mock.AnythingOfType("time.Time"), suite.testDeploymentID).
		Return(int64(0), nil).Once()

	err := suite.store.ExtendAttributeCacheTTL(suite.ctx, suite.testCache.ID, newTTL)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), errAttributeCacheNotFound, err)
}

// Tests for DeleteAttributeCache

func (suite *AttributeCacheStoreTestSuite) TestDeleteAttributeCache_Success() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("ExecuteContext", suite.ctx, queryDeleteAttributeCache,
		suite.testCache.ID, suite.testDeploymentID).
		Return(int64(1), nil).Once()

	err := suite.store.DeleteAttributeCache(suite.ctx, suite.testCache.ID)

	assert.Nil(suite.T(), err)
}

func (suite *AttributeCacheStoreTestSuite) TestDeleteAttributeCache_DBProviderError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db provider error")).Once()

	err := suite.store.DeleteAttributeCache(suite.ctx, suite.testCache.ID)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to get database client")
}

func (suite *AttributeCacheStoreTestSuite) TestDeleteAttributeCache_ExecuteError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("ExecuteContext", suite.ctx, queryDeleteAttributeCache,
		suite.testCache.ID, suite.testDeploymentID).
		Return(int64(0), errors.New("database error")).Once()

	err := suite.store.DeleteAttributeCache(suite.ctx, suite.testCache.ID)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to delete attribute cache")
}

func (suite *AttributeCacheStoreTestSuite) TestDeleteAttributeCache_NoRowsAffected() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.On("ExecuteContext", suite.ctx, queryDeleteAttributeCache,
		suite.testCache.ID, suite.testDeploymentID).
		Return(int64(0), nil).Once()

	err := suite.store.DeleteAttributeCache(suite.ctx, suite.testCache.ID)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), errAttributeCacheNotFound, err)
}

// Tests for buildAttributeCacheFromResultRow

func (suite *AttributeCacheStoreTestSuite) TestBuildAttributeCacheFromResultRow_Success() {
	resultRow := map[string]interface{}{
		"id":          suite.testCache.ID,
		"attributes":  `{"key":"value"}`,
		"expiry_time": suite.futureTime,
	}

	result, err := suite.store.buildAttributeCacheFromResultRow(resultRow)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), suite.testCache.ID, result.ID)
	assert.Equal(suite.T(), suite.testCache.Attributes, result.Attributes)
	// TTL should be approximately 3600 seconds (allowing for small time differences)
	assert.Greater(suite.T(), result.TTLSeconds, 3500)
	assert.Less(suite.T(), result.TTLSeconds, 3700)
}

func (suite *AttributeCacheStoreTestSuite) TestBuildAttributeCacheFromResultRow_AttributesAsBytes() {
	resultRow := map[string]interface{}{
		"id":          suite.testCache.ID,
		"attributes":  []byte(`{"key":"value"}`),
		"expiry_time": suite.futureTime,
	}

	result, err := suite.store.buildAttributeCacheFromResultRow(resultRow)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), suite.testCache.ID, result.ID)
	assert.Equal(suite.T(), suite.testCache.Attributes, result.Attributes)
	assert.Greater(suite.T(), result.TTLSeconds, 3500)
	assert.Less(suite.T(), result.TTLSeconds, 3700)
}

func (suite *AttributeCacheStoreTestSuite) TestBuildAttributeCacheFromResultRow_MissingID() {
	resultRow := map[string]interface{}{
		"attributes":  `{"key":"value"}`,
		"expiry_time": suite.futureTime,
	}

	result, err := suite.store.buildAttributeCacheFromResultRow(resultRow)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to parse id as string")
	assert.Equal(suite.T(), AttributeCache{}, result)
}

func (suite *AttributeCacheStoreTestSuite) TestBuildAttributeCacheFromResultRow_MissingAttributes() {
	resultRow := map[string]interface{}{
		"id":          suite.testCache.ID,
		"expiry_time": suite.futureTime,
	}

	result, err := suite.store.buildAttributeCacheFromResultRow(resultRow)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to parse attributes")
	assert.Equal(suite.T(), AttributeCache{}, result)
}

func (suite *AttributeCacheStoreTestSuite) TestBuildAttributeCacheFromResultRow_MissingExpiryTime() {
	resultRow := map[string]interface{}{
		"id":         suite.testCache.ID,
		"attributes": `{"key":"value"}`,
	}

	result, err := suite.store.buildAttributeCacheFromResultRow(resultRow)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "unexpected type for expiry_time")
	assert.Equal(suite.T(), AttributeCache{}, result)
}

// Tests for parseTimeField and trimTimeString helpers

const testTimeString = "2023-12-01 10:30:45.123456789"

func (suite *AttributeCacheStoreTestSuite) TestParseTimeField_StringInput() {
	testTime := testTimeString + " extra content"
	expectedTime, _ := time.Parse("2006-01-02 15:04:05.999999999", testTimeString)

	result, err := parseTimeField(testTime, "test_field")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedTime, result)
}

func (suite *AttributeCacheStoreTestSuite) TestParseTimeField_TimeInput() {
	testTime := time.Now()

	result, err := parseTimeField(testTime, "test_field")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), testTime, result)
}

func (suite *AttributeCacheStoreTestSuite) TestTrimTimeString() {
	input := testTimeString + " extra content here"
	expected := testTimeString

	result := trimTimeString(input)
	assert.Equal(suite.T(), expected, result)
}

func (suite *AttributeCacheStoreTestSuite) TestTrimTimeString_ShortInput() {
	input := "2023-12-01"

	result := trimTimeString(input)
	assert.Equal(suite.T(), input, result)
}
