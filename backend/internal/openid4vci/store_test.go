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

package openid4vci

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testStoreDeploymentID = "test-deployment-id"

type OpenID4VCIStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *openID4VCIStore
}

func TestOpenID4VCIStoreTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VCIStoreTestSuite))
}

func (suite *OpenID4VCIStoreTestSuite) refreshMocks() {
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &openID4VCIStore{
		dbProvider:   suite.mockDBProvider,
		deploymentID: testStoreDeploymentID,
	}
}

func (suite *OpenID4VCIStoreTestSuite) SetupTest() {
	suite.refreshMocks()
}

func (suite *OpenID4VCIStoreTestSuite) TestSaveNonce() {
	expiry := time.Now().Add(time.Minute)
	testCases := []struct {
		name       string
		setupMocks func()
		shouldErr  bool
	}{
		{
			name: "Success",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertNonce,
					"n1", testStoreDeploymentID, expiry.UTC()).Return(int64(1), nil)
			},
		},
		{
			name: "ExecuteError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertNonce,
					mock.Anything, mock.Anything, mock.Anything).Return(int64(0), errors.New("insert failed"))
			},
			shouldErr: true,
		},
		{
			name: "DBClientError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db error"))
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.refreshMocks()
			tc.setupMocks()
			err := suite.store.SaveNonce(context.Background(), &nonceRecord{Nonce: "n1", ExpiresAt: expiry})
			if tc.shouldErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *OpenID4VCIStoreTestSuite) TestGetNonce() {
	expiry := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	testCases := []struct {
		name       string
		setupMocks func()
		wantOK     bool
	}{
		{
			name: "Success",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetNonce, "n1", testStoreDeploymentID).
					Return([]map[string]interface{}{
						{"nonce": "n1", "expiry_time": expiry},
					}, nil)
			},
			wantOK: true,
		},
		{
			name: "NotFound",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetNonce, "n1", testStoreDeploymentID).
					Return([]map[string]interface{}{}, nil)
			},
		},
		{
			name: "BadExpiry",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetNonce, "n1", testStoreDeploymentID).
					Return([]map[string]interface{}{
						{"nonce": "n1", "expiry_time": 12345},
					}, nil)
			},
		},
		{
			name: "QueryError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetNonce, "n1", testStoreDeploymentID).
					Return(nil, errors.New("query failed"))
			},
		},
		{
			name: "DBClientError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db error"))
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.refreshMocks()
			tc.setupMocks()
			rec, ok := suite.store.GetNonce(context.Background(), "n1")
			suite.Equal(tc.wantOK, ok)
			if tc.wantOK {
				suite.Require().NotNil(rec)
				suite.Equal("n1", rec.Nonce)
				suite.True(rec.ExpiresAt.Equal(expiry))
			} else {
				suite.Nil(rec)
			}
		})
	}
}

func (suite *OpenID4VCIStoreTestSuite) TestDeleteNonce() {
	testCases := []struct {
		name       string
		setupMocks func()
		shouldErr  bool
	}{
		{
			name: "Success",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteNonce, "n1", testStoreDeploymentID).
					Return(int64(1), nil)
			},
		},
		{
			name: "ExecuteError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteNonce, "n1", testStoreDeploymentID).
					Return(int64(0), errors.New("delete failed"))
			},
			shouldErr: true,
		},
		{
			name: "DBClientError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db error"))
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.refreshMocks()
			tc.setupMocks()
			err := suite.store.DeleteNonce(context.Background(), "n1")
			if tc.shouldErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *OpenID4VCIStoreTestSuite) TestSaveOfferMarshalError() {
	suite.refreshMocks()
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	rec := &offerRecord{
		ID:        "o1",
		Offer:     map[string]interface{}{"bad": make(chan int)},
		ExpiresAt: time.Now().Add(time.Minute),
	}
	err := suite.store.SaveOffer(context.Background(), rec)
	suite.Error(err)
}

func (suite *OpenID4VCIStoreTestSuite) TestSaveOffer() {
	expiry := time.Now().Add(time.Minute)
	rec := &offerRecord{ID: "o1", Offer: map[string]interface{}{"k": "v"}, ExpiresAt: expiry}
	testCases := []struct {
		name       string
		setupMocks func()
		shouldErr  bool
	}{
		{
			name: "Success",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertOffer,
					"o1", testStoreDeploymentID, `{"k":"v"}`, expiry.UTC()).Return(int64(1), nil)
			},
		},
		{
			name: "ExecuteError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertOffer,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(int64(0), errors.New("insert failed"))
			},
			shouldErr: true,
		},
		{
			name: "DBClientError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db error"))
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.refreshMocks()
			tc.setupMocks()
			err := suite.store.SaveOffer(context.Background(), rec)
			if tc.shouldErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *OpenID4VCIStoreTestSuite) TestGetOffer() {
	expiry := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	testCases := []struct {
		name       string
		setupMocks func()
		wantOK     bool
	}{
		{
			name: "Success",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetOffer, "o1", testStoreDeploymentID).
					Return([]map[string]interface{}{
						{"id": "o1", "offer": `{"k":"v"}`, "expiry_time": expiry},
					}, nil)
			},
			wantOK: true,
		},
		{
			name: "NotFound",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetOffer, "o1", testStoreDeploymentID).
					Return([]map[string]interface{}{}, nil)
			},
		},
		{
			name: "BadJSON",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetOffer, "o1", testStoreDeploymentID).
					Return([]map[string]interface{}{
						{"id": "o1", "offer": `not-json`, "expiry_time": expiry},
					}, nil)
			},
		},
		{
			name: "BadExpiry",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetOffer, "o1", testStoreDeploymentID).
					Return([]map[string]interface{}{
						{"id": "o1", "offer": `{"k":"v"}`, "expiry_time": 999},
					}, nil)
			},
		},
		{
			name: "QueryError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetOffer, "o1", testStoreDeploymentID).
					Return(nil, errors.New("query failed"))
			},
		},
		{
			name: "DBClientError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db error"))
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.refreshMocks()
			tc.setupMocks()
			rec, ok := suite.store.GetOffer(context.Background(), "o1")
			suite.Equal(tc.wantOK, ok)
			if tc.wantOK {
				suite.Require().NotNil(rec)
				suite.Equal("o1", rec.ID)
				suite.Equal("v", rec.Offer["k"])
			} else {
				suite.Nil(rec)
			}
		})
	}
}

func (suite *OpenID4VCIStoreTestSuite) TestVCIColumnString() {
	suite.Equal("hello", vciColumnString("hello"))
	suite.Equal("bytes", vciColumnString([]byte("bytes")))
	suite.Equal("", vciColumnString(123))
}

func (suite *OpenID4VCIStoreTestSuite) TestVCIColumnBytes() {
	suite.Equal([]byte("hello"), vciColumnBytes([]byte("hello")))
	suite.Equal([]byte("str"), vciColumnBytes("str"))
	suite.Nil(vciColumnBytes(123))
}

func (suite *OpenID4VCIStoreTestSuite) TestParseVCITime() {
	now := time.Date(2030, 6, 1, 12, 0, 0, 0, time.UTC)

	got, err := parseVCITime(now)
	suite.Require().NoError(err)
	suite.True(got.Equal(now))

	got, err = parseVCITime("2030-06-01 12:00:00.000000000")
	suite.Require().NoError(err)
	suite.Equal(2030, got.Year())

	got, err = parseVCITime([]byte("2030-06-01 12:00:00 +0000 UTC"))
	suite.Require().NoError(err)
	suite.Equal(2030, got.Year())

	got, err = parseVCITime("2030-06-01T12:00:00Z")
	suite.Require().NoError(err)
	suite.Equal(2030, got.Year())

	_, err = parseVCITime("not-a-time")
	suite.Error(err)

	_, err = parseVCITime(42)
	suite.Error(err)
}
