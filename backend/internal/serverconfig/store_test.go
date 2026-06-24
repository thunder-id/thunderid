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
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/modelmock"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment-id"

type mockResult struct{}

func (mockResult) LastInsertId() (int64, error) { return 0, nil }
func (mockResult) RowsAffected() (int64, error) { return 1, nil }

var _ sql.Result = mockResult{}

type StoreTestSuite struct {
	suite.Suite
	ctx            context.Context
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	mockTx         *modelmock.TxInterfaceMock
	store          *serverConfigStore
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (suite *StoreTestSuite) SetupTest() {
	suite.ctx = context.Background()
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.mockTx = modelmock.NewTxInterfaceMock(suite.T())
	suite.store = &serverConfigStore{dbProvider: suite.mockDBProvider, deploymentID: testDeploymentID}
}

func (suite *StoreTestSuite) expectDBClient() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
}

// --- GetServerConfigByName ---

func (suite *StoreTestSuite) TestGetServerConfigByName_Found() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetServerConfigByName,
		string(ConfigNameCORS), testDeploymentID).
		Return([]map[string]interface{}{{"name": "cors", "value": `["https://x.com"]`}}, nil)

	cfg, err := suite.store.GetServerConfigByName(suite.ctx, ConfigNameCORS)

	suite.NoError(err)
	suite.Require().NotNil(cfg)
	suite.Equal(ConfigNameCORS, cfg.Name)
	suite.Equal(json.RawMessage(`["https://x.com"]`), cfg.Value)
}

func (suite *StoreTestSuite) TestGetServerConfigByName_Found_ByteValue() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetServerConfigByName,
		string(ConfigNameCORS), testDeploymentID).
		Return([]map[string]interface{}{{"name": "cors", "value": []byte(`["https://x.com"]`)}}, nil)

	cfg, err := suite.store.GetServerConfigByName(suite.ctx, ConfigNameCORS)

	suite.NoError(err)
	suite.Equal(json.RawMessage(`["https://x.com"]`), cfg.Value)
}

func (suite *StoreTestSuite) TestGetServerConfigByName_NotFound() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetServerConfigByName,
		string(ConfigNameCORS), testDeploymentID).
		Return([]map[string]interface{}{}, nil)

	cfg, err := suite.store.GetServerConfigByName(suite.ctx, ConfigNameCORS)

	suite.NoError(err)
	suite.Nil(cfg)
}

func (suite *StoreTestSuite) TestGetServerConfigByName_QueryError() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetServerConfigByName,
		string(ConfigNameCORS), testDeploymentID).
		Return(nil, errors.New("db error"))

	cfg, err := suite.store.GetServerConfigByName(suite.ctx, ConfigNameCORS)

	suite.Error(err)
	suite.Nil(cfg)
}

func (suite *StoreTestSuite) TestGetServerConfigByName_BadRow() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetServerConfigByName,
		string(ConfigNameCORS), testDeploymentID).
		Return([]map[string]interface{}{{"value": `["x"]`}}, nil) // missing "name"

	cfg, err := suite.store.GetServerConfigByName(suite.ctx, ConfigNameCORS)

	suite.Error(err)
	suite.Nil(cfg)
}

// --- GetServerConfigList ---

func (suite *StoreTestSuite) TestGetServerConfigList_OK() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryListServerConfigs, testDeploymentID).
		Return([]map[string]interface{}{
			{"name": "cors", "value": `["https://x.com"]`},
		}, nil)

	configs, err := suite.store.GetServerConfigList(suite.ctx)

	suite.NoError(err)
	suite.Len(configs, 1)
	suite.Equal(ConfigNameCORS, configs[0].Name)
}

func (suite *StoreTestSuite) TestGetServerConfigList_Error() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryListServerConfigs, testDeploymentID).
		Return(nil, errors.New("db error"))

	configs, err := suite.store.GetServerConfigList(suite.ctx)

	suite.Error(err)
	suite.Nil(configs)
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

// --- UpsertServerConfigs (transactional batch) ---

func (suite *StoreTestSuite) TestUpsertServerConfigs_Commit() {
	suite.expectDBClient()
	suite.mockDBClient.On("BeginTx").Return(suite.mockTx, nil)
	suite.mockTx.On("Exec", queryUpsertServerConfig, "cors", `["a"]`, testDeploymentID).
		Return(mockResult{}, nil)
	suite.mockTx.On("Commit").Return(nil)

	err := suite.store.UpsertServerConfigs(suite.ctx,
		[]ServerConfig{{Name: ConfigNameCORS, Value: json.RawMessage(`["a"]`)}})
	suite.NoError(err)
}

func (suite *StoreTestSuite) TestUpsertServerConfigs_MidBatchError_RollsBack() {
	suite.expectDBClient()
	suite.mockDBClient.On("BeginTx").Return(suite.mockTx, nil)
	suite.mockTx.On("Exec", queryUpsertServerConfig, "cors", `["a"]`, testDeploymentID).
		Return(mockResult{}, nil)
	suite.mockTx.On("Exec", queryUpsertServerConfig, "cors", `["b"]`, testDeploymentID).
		Return(nil, errors.New("exec error"))
	suite.mockTx.On("Rollback").Return(nil)

	err := suite.store.UpsertServerConfigs(suite.ctx, []ServerConfig{
		{Name: ConfigNameCORS, Value: json.RawMessage(`["a"]`)},
		{Name: ConfigNameCORS, Value: json.RawMessage(`["b"]`)},
	})

	suite.Error(err)
	suite.mockTx.AssertNotCalled(suite.T(), "Commit")
}

func (suite *StoreTestSuite) TestUpsertServerConfigs_RollbackError() {
	suite.expectDBClient()
	suite.mockDBClient.On("BeginTx").Return(suite.mockTx, nil)
	suite.mockTx.On("Exec", queryUpsertServerConfig, "cors", string(corsValue), testDeploymentID).
		Return(nil, errors.New("exec error"))
	suite.mockTx.On("Rollback").Return(errors.New("rollback error"))

	err := suite.store.UpsertServerConfigs(suite.ctx,
		[]ServerConfig{{Name: ConfigNameCORS, Value: corsValue}})

	suite.Error(err)
	suite.Contains(err.Error(), "rollback")
}

func (suite *StoreTestSuite) TestUpsertServerConfigs_BeginTxError() {
	suite.expectDBClient()
	suite.mockDBClient.On("BeginTx").Return(nil, errors.New("begin error"))

	err := suite.store.UpsertServerConfigs(suite.ctx,
		[]ServerConfig{{Name: ConfigNameCORS, Value: corsValue}})
	suite.Error(err)
}

func (suite *StoreTestSuite) TestUpsertServerConfigs_CommitError() {
	suite.expectDBClient()
	suite.mockDBClient.On("BeginTx").Return(suite.mockTx, nil)
	suite.mockTx.On("Exec", queryUpsertServerConfig, "cors", string(corsValue), testDeploymentID).
		Return(mockResult{}, nil)
	suite.mockTx.On("Commit").Return(errors.New("commit error"))

	err := suite.store.UpsertServerConfigs(suite.ctx,
		[]ServerConfig{{Name: ConfigNameCORS, Value: corsValue}})
	suite.Error(err)
}

// --- DeleteServerConfig ---

func (suite *StoreTestSuite) TestDeleteServerConfig_OK() {
	suite.expectDBClient()
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteServerConfig,
		string(ConfigNameCORS), testDeploymentID).
		Return(int64(1), nil)

	suite.NoError(suite.store.DeleteServerConfig(suite.ctx, ConfigNameCORS))
}

func (suite *StoreTestSuite) TestDeleteServerConfig_Error() {
	suite.expectDBClient()
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteServerConfig,
		string(ConfigNameCORS), testDeploymentID).
		Return(int64(0), errors.New("db error"))

	suite.Error(suite.store.DeleteServerConfig(suite.ctx, ConfigNameCORS))
}

// --- DB client acquisition failure (shared getDBClient error path) ---

func (suite *StoreTestSuite) expectDBClientError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("client error"))
}

func (suite *StoreTestSuite) TestGetServerConfigByName_DBClientError() {
	suite.expectDBClientError()
	cfg, err := suite.store.GetServerConfigByName(suite.ctx, ConfigNameCORS)
	suite.Error(err)
	suite.Nil(cfg)
}

func (suite *StoreTestSuite) TestGetServerConfigList_DBClientError() {
	suite.expectDBClientError()
	configs, err := suite.store.GetServerConfigList(suite.ctx)
	suite.Error(err)
	suite.Nil(configs)
}

func (suite *StoreTestSuite) TestUpsertServerConfig_DBClientError() {
	suite.expectDBClientError()
	suite.Error(suite.store.UpsertServerConfig(suite.ctx, ServerConfig{Name: ConfigNameCORS, Value: corsValue}))
}

func (suite *StoreTestSuite) TestUpsertServerConfigs_DBClientError() {
	suite.expectDBClientError()
	suite.Error(suite.store.UpsertServerConfigs(suite.ctx,
		[]ServerConfig{{Name: ConfigNameCORS, Value: corsValue}}))
}

func (suite *StoreTestSuite) TestDeleteServerConfig_DBClientError() {
	suite.expectDBClientError()
	suite.Error(suite.store.DeleteServerConfig(suite.ctx, ConfigNameCORS))
}

// --- Row-mapping failures ---

func (suite *StoreTestSuite) TestGetServerConfigByName_BadValueType() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetServerConfigByName,
		string(ConfigNameCORS), testDeploymentID).
		Return([]map[string]interface{}{{"name": "cors", "value": 123}}, nil) // value not string/[]byte

	cfg, err := suite.store.GetServerConfigByName(suite.ctx, ConfigNameCORS)

	suite.Error(err)
	suite.Nil(cfg)
}

func (suite *StoreTestSuite) TestGetServerConfigList_BadRow() {
	suite.expectDBClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryListServerConfigs, testDeploymentID).
		Return([]map[string]interface{}{{"value": `["x"]`}}, nil) // missing "name"

	configs, err := suite.store.GetServerConfigList(suite.ctx)

	suite.Error(err)
	suite.Nil(configs)
}
