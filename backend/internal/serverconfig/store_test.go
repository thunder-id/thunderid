// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
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
	loadRuntimeForScope()
	suite.ctx = context.Background()
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &serverConfigStore{dbProvider: suite.mockDBProvider}
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

// loadRuntimeForScope loads a server runtime naming the deployment these tests assert on. The store
// resolves its deployment from the runtime rather than holding one, and other suites in this package
// reset the runtime, so it is loaded per test rather than once for the package.
func loadRuntimeForScope() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: testDeploymentID},
	})
}
