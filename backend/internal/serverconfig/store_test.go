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
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment-id"

type StoreTestSuite struct {
	suite.Suite
	ctx            context.Context
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *serverConfigStore
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (suite *StoreTestSuite) SetupTest() {
	suite.ctx = context.Background()
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &serverConfigStore{dbProvider: suite.mockDBProvider, deploymentID: testDeploymentID}
}

func (suite *StoreTestSuite) expectDBClient() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
}

func (suite *StoreTestSuite) expectDBClientError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("client error"))
}

// --- GetServerConfig ---

func (suite *StoreTestSuite) TestGetServerConfig_Found() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetServerConfigByName,
		string(ConfigNameCORS), testDeploymentID).
		Return([]map[string]interface{}{{"name": "cors", "value": `["https://x.com"]`}}, nil)

	layers, err := suite.store.GetServerConfig(suite.ctx, ConfigNameCORS)

	suite.NoError(err)
	suite.Equal(json.RawMessage(`["https://x.com"]`), layers.Writable)
	suite.Nil(layers.ReadOnly)
}

func (suite *StoreTestSuite) TestGetServerConfig_Found_ByteValue() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetServerConfigByName,
		string(ConfigNameCORS), testDeploymentID).
		Return([]map[string]interface{}{{"value": []byte(`["https://x.com"]`)}}, nil)

	layers, err := suite.store.GetServerConfig(suite.ctx, ConfigNameCORS)

	suite.NoError(err)
	suite.Equal(json.RawMessage(`["https://x.com"]`), layers.Writable)
}

func (suite *StoreTestSuite) TestGetServerConfig_NotFound() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetServerConfigByName,
		string(ConfigNameCORS), testDeploymentID).
		Return([]map[string]interface{}{}, nil)

	layers, err := suite.store.GetServerConfig(suite.ctx, ConfigNameCORS)

	suite.NoError(err)
	suite.Equal(storeLayers{}, layers)
}

func (suite *StoreTestSuite) TestGetServerConfig_QueryError() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetServerConfigByName,
		string(ConfigNameCORS), testDeploymentID).
		Return(nil, errors.New("db error"))

	_, err := suite.store.GetServerConfig(suite.ctx, ConfigNameCORS)
	suite.Error(err)
}

func (suite *StoreTestSuite) TestGetServerConfig_BadValueType() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetServerConfigByName,
		string(ConfigNameCORS), testDeploymentID).
		Return([]map[string]interface{}{{"value": 123}}, nil) // value not string/[]byte

	_, err := suite.store.GetServerConfig(suite.ctx, ConfigNameCORS)
	suite.Error(err)
}

func (suite *StoreTestSuite) TestGetServerConfig_DBClientError() {
	suite.expectDBClientError()
	_, err := suite.store.GetServerConfig(suite.ctx, ConfigNameCORS)
	suite.Error(err)
}

// --- UpsertServerConfig ---

func (suite *StoreTestSuite) TestUpsertServerConfig_OK() {
	suite.expectDBClient()
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpsertServerConfig,
		string(ConfigNameCORS), string(corsValue), testDeploymentID).
		Return(int64(1), nil)

	err := suite.store.UpsertServerConfig(suite.ctx, ServerConfig{Name: ConfigNameCORS, Value: corsValue})
	suite.NoError(err)
}

func (suite *StoreTestSuite) TestUpsertServerConfig_Error() {
	suite.expectDBClient()
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpsertServerConfig,
		string(ConfigNameCORS), string(corsValue), testDeploymentID).
		Return(int64(0), errors.New("db error"))

	err := suite.store.UpsertServerConfig(suite.ctx, ServerConfig{Name: ConfigNameCORS, Value: corsValue})
	suite.Error(err)
}

func (suite *StoreTestSuite) TestUpsertServerConfig_DBClientError() {
	suite.expectDBClientError()
	suite.Error(suite.store.UpsertServerConfig(suite.ctx, ServerConfig{Name: ConfigNameCORS, Value: corsValue}))
}
