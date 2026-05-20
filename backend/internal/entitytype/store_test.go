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

package entitytype

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	dbMock "github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type StoreTestSuite struct {
	suite.Suite
	mockProvider *dbMock.DBProviderInterfaceMock
	mockClient   *dbMock.DBClientInterfaceMock
	store        *entityTypeStore
}

func (suite *StoreTestSuite) SetupTest() {
	suite.mockProvider = dbMock.NewDBProviderInterfaceMock(suite.T())
	suite.mockClient = dbMock.NewDBClientInterfaceMock(suite.T())

	suite.store = &entityTypeStore{
		dbProvider:   suite.mockProvider,
		deploymentID: "test-node",
	}
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (suite *StoreTestSuite) TestGetEntityTypeListByOUIDs() {
	ctx := context.Background()

	testCases := []struct {
		name          string
		ouIDs         []string
		limit         int
		offset        int
		mockSetup     func()
		expectedItems []EntityTypeListItem
		expectErr     bool
	}{
		{
			name:          "Empty OUIDs",
			ouIDs:         []string{},
			limit:         10,
			offset:        0,
			mockSetup:     func() {}, // No DB calls expected
			expectedItems: []EntityTypeListItem{},
			expectErr:     false,
		},
		{
			name:   "DB Provider Error",
			ouIDs:  []string{"ou-1"},
			limit:  10,
			offset: 0,
			mockSetup: func() {
				suite.mockProvider.On("GetConfigDBClient").Return(nil, errors.New("provider error")).Once()
			},
			expectedItems: nil,
			expectErr:     true,
		},
		{
			name:   "Query Execution Error",
			ouIDs:  []string{"ou-1"},
			limit:  10,
			offset: 0,
			mockSetup: func() {
				suite.mockProvider.On("GetConfigDBClient").Return(suite.mockClient, nil).Once()
				suite.mockClient.On(
					"QueryContext", ctx, mock.AnythingOfType("model.DBQuery"),
					"ou-1", string(TypeCategoryUser), "test-node", 10, 0).
					Return(nil, errors.New("query error")).Once()
			},
			expectedItems: nil,
			expectErr:     true,
		},
		{
			name:   "Successful Query with Parse Error (Skipped Row)",
			ouIDs:  []string{"ou-1"},
			limit:  10,
			offset: 0,
			mockSetup: func() {
				suite.mockProvider.On("GetConfigDBClient").Return(suite.mockClient, nil).Once()

				rows := []map[string]interface{}{
					{
						// Invalid row missing fields
						"id": "schema-1",
					},
					{
						"id":                      "schema-2",
						"category":                "user",
						"name":                    "Schema 2",
						"ou_id":                   "ou-1",
						"allow_self_registration": true,
					},
				}

				suite.mockClient.On(
					"QueryContext", ctx, mock.AnythingOfType("model.DBQuery"),
					"ou-1", string(TypeCategoryUser), "test-node", 10, 0).
					Return(rows, nil).Once()
			},
			expectedItems: []EntityTypeListItem{
				{
					ID:                    "schema-2",
					Category:              TypeCategoryUser,
					Name:                  "Schema 2",
					OUID:                  "ou-1",
					AllowSelfRegistration: true,
				},
			},
			expectErr: false,
		},
		{
			name:   "Successful Query Multiple OUs",
			ouIDs:  []string{"ou-1", "ou-2"},
			limit:  5,
			offset: 10,
			mockSetup: func() {
				suite.mockProvider.On("GetConfigDBClient").Return(suite.mockClient, nil).Once()

				rows := []map[string]interface{}{
					{
						"id":                      "schema-1",
						"category":                "user",
						"name":                    "Schema 1",
						"ou_id":                   "ou-1",
						"allow_self_registration": false,
					},
				}

				suite.mockClient.On(
					"QueryContext", ctx, mock.AnythingOfType("model.DBQuery"),
					"ou-1", "ou-2", string(TypeCategoryUser), "test-node", 5, 10).
					Return(rows, nil).Once()
			},
			expectedItems: []EntityTypeListItem{
				{
					ID:                    "schema-1",
					Category:              TypeCategoryUser,
					Name:                  "Schema 1",
					OUID:                  "ou-1",
					AllowSelfRegistration: false,
				},
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // Reset mocks
			tc.mockSetup()

			items, err := suite.store.GetEntityTypeListByOUIDs(ctx, TypeCategoryUser, tc.ouIDs, tc.limit, tc.offset)

			if tc.expectErr {
				assert.Error(suite.T(), err)
			} else {
				assert.NoError(suite.T(), err)
				assert.Equal(suite.T(), tc.expectedItems, items)
			}
			suite.mockProvider.AssertExpectations(suite.T())
			suite.mockClient.AssertExpectations(suite.T())
		})
	}
}

func (suite *StoreTestSuite) TestGetEntityTypeListCountByOUIDs() {
	ctx := context.Background()

	testCases := []struct {
		name          string
		ouIDs         []string
		mockSetup     func()
		expectedCount int
		expectErr     bool
	}{
		{
			name:          "Empty OUIDs",
			ouIDs:         []string{},
			mockSetup:     func() {}, // No DB calls expected
			expectedCount: 0,
			expectErr:     false,
		},
		{
			name:  "DB Provider Error",
			ouIDs: []string{"ou-1"},
			mockSetup: func() {
				suite.mockProvider.On("GetConfigDBClient").Return(nil, errors.New("provider error")).Once()
			},
			expectedCount: 0,
			expectErr:     true,
		},
		{
			name:  "Query Execution Error",
			ouIDs: []string{"ou-1"},
			mockSetup: func() {
				suite.mockProvider.On("GetConfigDBClient").Return(suite.mockClient, nil).Once()
				suite.mockClient.On(
					"QueryContext", ctx, mock.AnythingOfType("model.DBQuery"),
					"ou-1", string(TypeCategoryUser), "test-node").
					Return(nil, errors.New("query error")).Once()
			},
			expectedCount: 0,
			expectErr:     true,
		},
		{
			name:  "Invalid Count Type",
			ouIDs: []string{"ou-1"},
			mockSetup: func() {
				suite.mockProvider.On("GetConfigDBClient").Return(suite.mockClient, nil).Once()

				rows := []map[string]interface{}{
					{
						"total": "not-an-int",
					},
				}

				suite.mockClient.On(
					"QueryContext", ctx, mock.AnythingOfType("model.DBQuery"),
					"ou-1", string(TypeCategoryUser), "test-node").
					Return(rows, nil).Once()
			},
			expectedCount: 0,
			expectErr:     true,
		},
		{
			name:  "Successful Query Count",
			ouIDs: []string{"ou-1", "ou-2"},
			mockSetup: func() {
				suite.mockProvider.On("GetConfigDBClient").Return(suite.mockClient, nil).Once()

				rows := []map[string]interface{}{
					{
						"total": int64(42),
					},
				}

				suite.mockClient.On(
					"QueryContext", ctx, mock.AnythingOfType("model.DBQuery"),
					"ou-1", "ou-2", string(TypeCategoryUser), "test-node").
					Return(rows, nil).Once()
			},
			expectedCount: 42,
			expectErr:     false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // Reset mocks
			tc.mockSetup()

			count, err := suite.store.GetEntityTypeListCountByOUIDs(ctx, TypeCategoryUser, tc.ouIDs)

			if tc.expectErr {
				assert.Error(suite.T(), err)
			} else {
				assert.NoError(suite.T(), err)
				assert.Equal(suite.T(), tc.expectedCount, count)
			}
			suite.mockProvider.AssertExpectations(suite.T())
			suite.mockClient.AssertExpectations(suite.T())
		})
	}
}
