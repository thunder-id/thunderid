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

package dbstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const (
	testDeploymentID = "test-deployment"
	testNamespace    = providers.RuntimeStoreNamespace("test-ns")
	testKey          = "key1"
)

var testValue = []byte(`{"v":1}`)

type DBStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *dbStore
	ctx            context.Context
}

func TestDBStoreTestSuite(t *testing.T) {
	suite.Run(t, new(DBStoreTestSuite))
}

func (s *DBStoreTestSuite) SetupTest() {
	s.mockDBProvider = &providermock.DBProviderInterfaceMock{}
	s.mockDBClient = &providermock.DBClientInterfaceMock{}
	s.store = &dbStore{
		dbProvider:   s.mockDBProvider,
		deploymentID: testDeploymentID,
		logger:       log.GetLogger(),
	}
	s.ctx = context.Background()
}

// Put

func (s *DBStoreTestSuite) TestPut_WithTTL_Success() {
	const ttlSeconds int64 = 60
	before := time.Now().UTC()
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryPutRuntimeStore,
		testDeploymentID, string(testNamespace), testKey, testValue,
		mock.MatchedBy(func(t time.Time) bool {
			expected := before.Add(time.Duration(ttlSeconds) * time.Second)
			diff := t.Sub(expected)
			return diff >= -time.Second && diff <= time.Second
		}),
	).Return(int64(1), nil)

	err := s.store.Put(s.ctx, testNamespace, testKey, testValue, ttlSeconds)

	s.NoError(err)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

func (s *DBStoreTestSuite) TestPut_NoTTL_StoresNilExpiry() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryPutRuntimeStore,
		testDeploymentID, string(testNamespace), testKey, testValue, nil,
	).Return(int64(1), nil)

	err := s.store.Put(s.ctx, testNamespace, testKey, testValue, 0)

	s.NoError(err)
	s.mockDBClient.AssertExpectations(s.T())
}

func (s *DBStoreTestSuite) TestPut_DBClientError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(nil, errors.New("db client error"))

	err := s.store.Put(s.ctx, testNamespace, testKey, testValue, 60)

	s.Error(err)
}

func (s *DBStoreTestSuite) TestPut_ExecuteError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryPutRuntimeStore,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(int64(0), errors.New("insert failed"))

	err := s.store.Put(s.ctx, testNamespace, testKey, testValue, 60)

	s.Error(err)
	s.Contains(err.Error(), "failed to store in database")
}

// Get

func (s *DBStoreTestSuite) TestGet_Hit_StringValue() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetRuntimeStore,
		testDeploymentID, string(testNamespace), testKey, mock.Anything,
	).Return([]map[string]interface{}{{columnNameValue: string(testValue)}}, nil)

	got, err := s.store.Get(s.ctx, testNamespace, testKey)

	s.NoError(err)
	s.Equal(testValue, got)
}

func (s *DBStoreTestSuite) TestGet_Hit_BytesValue() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetRuntimeStore,
		testDeploymentID, string(testNamespace), testKey, mock.Anything,
	).Return([]map[string]interface{}{{columnNameValue: testValue}}, nil)

	got, err := s.store.Get(s.ctx, testNamespace, testKey)

	s.NoError(err)
	s.Equal(testValue, got)
}

func (s *DBStoreTestSuite) TestGet_Miss_ReturnsNilNil() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetRuntimeStore,
		testDeploymentID, string(testNamespace), testKey, mock.Anything,
	).Return([]map[string]interface{}{}, nil)

	got, err := s.store.Get(s.ctx, testNamespace, testKey)

	s.NoError(err)
	s.Nil(got)
}

func (s *DBStoreTestSuite) TestGet_DBClientError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(nil, errors.New("db client error"))

	got, err := s.store.Get(s.ctx, testNamespace, testKey)

	s.Error(err)
	s.Nil(got)
}

func (s *DBStoreTestSuite) TestGet_QueryError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetRuntimeStore,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, errors.New("query failed"))

	got, err := s.store.Get(s.ctx, testNamespace, testKey)

	s.Error(err)
	s.Nil(got)
}

func (s *DBStoreTestSuite) TestGet_BadValueType() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetRuntimeStore,
		testDeploymentID, string(testNamespace), testKey, mock.Anything,
	).Return([]map[string]interface{}{{columnNameValue: 123}}, nil)

	got, err := s.store.Get(s.ctx, testNamespace, testKey)

	s.Error(err)
	s.Nil(got)
}

// Update

func (s *DBStoreTestSuite) TestUpdate_Success() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateRuntimeStore,
		testDeploymentID, string(testNamespace), testKey, testValue, mock.Anything,
	).Return(int64(1), nil)

	err := s.store.Update(s.ctx, testNamespace, testKey, testValue)

	s.NoError(err)
}

func (s *DBStoreTestSuite) TestUpdate_NotFound_ReturnsError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateRuntimeStore,
		testDeploymentID, string(testNamespace), testKey, testValue, mock.Anything,
	).Return(int64(0), nil)

	err := s.store.Update(s.ctx, testNamespace, testKey, testValue)

	s.ErrorIs(err, providers.ErrRuntimeStoreKeyNotFound)
}

func (s *DBStoreTestSuite) TestUpdate_DBClientError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(nil, errors.New("db client error"))

	err := s.store.Update(s.ctx, testNamespace, testKey, testValue)

	s.Error(err)
}

func (s *DBStoreTestSuite) TestUpdate_ExecuteError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateRuntimeStore,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(int64(0), errors.New("update failed"))

	err := s.store.Update(s.ctx, testNamespace, testKey, testValue)

	s.Error(err)
	s.Contains(err.Error(), "failed to update in database")
}

// Delete

func (s *DBStoreTestSuite) TestDelete_Success() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteRuntimeStore,
		testDeploymentID, string(testNamespace), testKey,
	).Return(int64(1), nil)

	err := s.store.Delete(s.ctx, testNamespace, testKey)

	s.NoError(err)
}

func (s *DBStoreTestSuite) TestDelete_MissingIsIdempotent() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteRuntimeStore,
		testDeploymentID, string(testNamespace), testKey,
	).Return(int64(0), nil)

	err := s.store.Delete(s.ctx, testNamespace, testKey)

	s.NoError(err)
}

func (s *DBStoreTestSuite) TestDelete_DBClientError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(nil, errors.New("db client error"))

	err := s.store.Delete(s.ctx, testNamespace, testKey)

	s.Error(err)
}

func (s *DBStoreTestSuite) TestDelete_ExecuteError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteRuntimeStore,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(int64(0), errors.New("delete failed"))

	err := s.store.Delete(s.ctx, testNamespace, testKey)

	s.Error(err)
	s.Contains(err.Error(), "failed to delete from database")
}

// Take

func (s *DBStoreTestSuite) TestTake_Hit() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryTakeRuntimeStore,
		testDeploymentID, string(testNamespace), testKey, mock.Anything,
	).Return([]map[string]interface{}{{columnNameValue: string(testValue)}}, nil)

	got, err := s.store.Take(s.ctx, testNamespace, testKey)

	s.NoError(err)
	s.Equal(testValue, got)
}

// TestTake_Miss_ReturnsNilNil also covers the concurrent-consume case: the atomic
// DELETE ... RETURNING deletes zero rows and returns an empty result set.
func (s *DBStoreTestSuite) TestTake_Miss_ReturnsNilNil() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryTakeRuntimeStore,
		testDeploymentID, string(testNamespace), testKey, mock.Anything,
	).Return([]map[string]interface{}{}, nil)

	got, err := s.store.Take(s.ctx, testNamespace, testKey)

	s.NoError(err)
	s.Nil(got)
}

func (s *DBStoreTestSuite) TestTake_DBClientError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(nil, errors.New("db client error"))

	got, err := s.store.Take(s.ctx, testNamespace, testKey)

	s.Error(err)
	s.Nil(got)
}

func (s *DBStoreTestSuite) TestTake_QueryError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryTakeRuntimeStore,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, errors.New("query failed"))

	got, err := s.store.Take(s.ctx, testNamespace, testKey)

	s.Error(err)
	s.Nil(got)
	s.Contains(err.Error(), "failed to take data from database")
}

// ExtendTTL

func (s *DBStoreTestSuite) TestExtendTTL_Success() {
	const ttlSeconds int64 = 60
	before := time.Now().UTC()
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryExtendTTLRuntimeStore,
		testDeploymentID, string(testNamespace), testKey,
		mock.MatchedBy(func(t time.Time) bool {
			expected := before.Add(time.Duration(ttlSeconds) * time.Second)
			diff := t.Sub(expected)
			return diff >= -time.Second && diff <= time.Second
		}), mock.Anything,
	).Return(int64(1), nil)

	err := s.store.ExtendTTL(s.ctx, testNamespace, testKey, ttlSeconds)

	s.NoError(err)
}

func (s *DBStoreTestSuite) TestExtendTTL_ZeroTTL_ReturnsError() {
	err := s.store.ExtendTTL(s.ctx, testNamespace, testKey, 0)

	s.Error(err)
	s.Contains(err.Error(), "ttl seconds cannot be negative or zero")
}

func (s *DBStoreTestSuite) TestExtendTTL_NegativeTTL_ReturnsError() {
	err := s.store.ExtendTTL(s.ctx, testNamespace, testKey, -1)

	s.Error(err)
	s.Contains(err.Error(), "ttl seconds cannot be negative or zero")
}

func (s *DBStoreTestSuite) TestExtendTTL_DBClientError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(nil, errors.New("db client error"))

	err := s.store.ExtendTTL(s.ctx, testNamespace, testKey, 60)

	s.Error(err)
}

func (s *DBStoreTestSuite) TestExtendTTL_ExecuteError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryExtendTTLRuntimeStore,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(int64(0), errors.New("update failed"))

	err := s.store.ExtendTTL(s.ctx, testNamespace, testKey, 60)

	s.Error(err)
	s.Contains(err.Error(), "failed to extend TTL in database")
}

func (s *DBStoreTestSuite) TestExtendTTL_NotFound_ReturnsError() {
	s.mockDBProvider.On("GetRuntimeTransientDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryExtendTTLRuntimeStore,
		testDeploymentID, string(testNamespace), testKey, mock.Anything, mock.Anything,
	).Return(int64(0), nil)

	err := s.store.ExtendTTL(s.ctx, testNamespace, testKey, 60)

	s.ErrorIs(err, providers.ErrRuntimeStoreKeyNotFound)
}
