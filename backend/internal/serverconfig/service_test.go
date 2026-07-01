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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type ServiceTestSuite struct {
	suite.Suite
	ctx         context.Context
	mockStore   *serverConfigStoreInterfaceMock
	mockHandler *ServerConfigHandlerInterfaceMock
	service     ServerConfigService
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (suite *ServiceTestSuite) SetupTest() {
	suite.ctx = context.Background()
	suite.mockStore = newServerConfigStoreInterfaceMock(suite.T())
	suite.mockHandler = NewServerConfigHandlerInterfaceMock(suite.T())
	suite.service = newServerConfigService(suite.mockStore,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: suite.mockHandler})
}

// serviceWithoutHandlers builds a service with no registered handlers, sharing the suite store mock.
func (suite *ServiceTestSuite) serviceWithoutHandlers() ServerConfigService {
	return newServerConfigService(suite.mockStore, map[ConfigName]ServerConfigHandlerInterface{})
}

// Raw (byte) layers shared across the store tests, plus an incoming PUT value.
var (
	corsValue   = json.RawMessage(`["https://app.example.com"]`)
	declarative = json.RawMessage(`["https://static.example.com"]`)
	mergedValue = json.RawMessage(`["https://static.example.com","https://app.example.com"]`)
	incomingRaw = json.RawMessage(`["https://new.example.com"]`)
)

// Decoded sentinels the mocked handler yields for the raw layers above; they flow through the service
// as opaque values, so simple strings suffice.
const (
	readOnlyVal = "decoded-readonly"
	writableVal = "decoded-writable"
	incomingVal = "decoded-incoming"
	mergedVal   = "decoded-merged"
)

// --- ListConfigNames ---

func (suite *ServiceTestSuite) TestListConfigNames() {
	names, svcErr := suite.service.ListConfigNames(suite.ctx)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), supportedConfigNames, names)
}

// --- GetConfig ---

func (suite *ServiceTestSuite) TestGetConfig_UnsupportedName() {
	layers, svcErr := suite.service.GetConfig(suite.ctx, ConfigName("bogus"))
	assert.Equal(suite.T(), ServerConfigLayers{}, layers)
	assert.Same(suite.T(), &ErrorUnsupportedConfigName, svcErr)
}

func (suite *ServiceTestSuite) TestGetConfig_NoHandlerRegistered_FailClosed() {
	layers, svcErr := suite.serviceWithoutHandlers().GetConfig(suite.ctx, ConfigNameCORS)
	assert.Equal(suite.T(), ServerConfigLayers{}, layers)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetConfig_StoreError() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{}, errors.New("db error"))
	layers, svcErr := suite.service.GetConfig(suite.ctx, ConfigNameCORS)
	assert.Equal(suite.T(), ServerConfigLayers{}, layers)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetConfig_DecodeError() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative, Writable: corsValue}, nil)
	suite.mockHandler.EXPECT().Decode(declarative).Return(nil, errors.New("corrupt stored value"))
	layers, svcErr := suite.service.GetConfig(suite.ctx, ConfigNameCORS)
	assert.Equal(suite.T(), ServerConfigLayers{}, layers)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetConfig_OK() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative, Writable: corsValue}, nil)
	suite.mockHandler.EXPECT().Decode(declarative).Return(readOnlyVal, nil)
	suite.mockHandler.EXPECT().Decode(corsValue).Return(writableVal, nil)
	suite.mockHandler.EXPECT().Merge(readOnlyVal, writableVal).Return(mergedVal)

	layers, svcErr := suite.service.GetConfig(suite.ctx, ConfigNameCORS)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), readOnlyVal, layers.ReadOnly)
	assert.Equal(suite.T(), writableVal, layers.Writable)
	assert.Equal(suite.T(), mergedVal, layers.Merged)
}

func (suite *ServiceTestSuite) TestGetConfig_Unset() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	suite.mockHandler.EXPECT().Merge(nil, nil).Return(mergedVal)

	layers, svcErr := suite.service.GetConfig(suite.ctx, ConfigNameCORS)
	assert.Nil(suite.T(), svcErr)
	assert.Nil(suite.T(), layers.ReadOnly)
	assert.Nil(suite.T(), layers.Writable)
	assert.Equal(suite.T(), mergedVal, layers.Merged)
}

// --- GetMergedConfig ---

func (suite *ServiceTestSuite) TestGetMergedConfig_OK() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative, Writable: corsValue}, nil)
	suite.mockHandler.EXPECT().Decode(declarative).Return(readOnlyVal, nil)
	suite.mockHandler.EXPECT().Decode(corsValue).Return(writableVal, nil)
	suite.mockHandler.EXPECT().Merge(readOnlyVal, writableVal).Return(mergedVal)

	merged, svcErr := suite.service.GetMergedConfig(suite.ctx, string(ConfigNameCORS))
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), mergedVal, merged)
}

func (suite *ServiceTestSuite) TestGetMergedConfig_UnsupportedName() {
	merged, svcErr := suite.service.GetMergedConfig(suite.ctx, "bogus")
	assert.Nil(suite.T(), merged)
	assert.Same(suite.T(), &ErrorUnsupportedConfigName, svcErr)
}

func (suite *ServiceTestSuite) TestGetMergedConfig_StoreError() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{}, errors.New("db error"))
	merged, svcErr := suite.service.GetMergedConfig(suite.ctx, string(ConfigNameCORS))
	assert.Nil(suite.T(), merged)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}

// --- SetConfig ---

func (suite *ServiceTestSuite) TestSetConfig_UnsupportedName() {
	svcErr := suite.service.SetConfig(suite.ctx, ConfigName("bogus"), incomingRaw)
	assert.Same(suite.T(), &ErrorUnsupportedConfigName, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfig_NoHandlerRegistered_FailClosed() {
	svcErr := suite.serviceWithoutHandlers().SetConfig(suite.ctx, ConfigNameCORS, incomingRaw)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfig_DecodeIncomingError() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(nil, errors.New("bad shape"))
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw)
	assert.Same(suite.T(), &ErrorInvalidConfigValue, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "GetServerConfig", mock.Anything, mock.Anything)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfig_ReadError() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{}, errors.New("db error"))
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfig_HandlerRejects() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative}, nil)
	suite.mockHandler.EXPECT().Decode(declarative).Return(readOnlyVal, nil)
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	suite.mockHandler.EXPECT().Validate(incomingVal, readOnlyVal, nil).Return(errors.New("bad value"))
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw)
	assert.Same(suite.T(), &ErrorInvalidConfigValue, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfig_OK() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative}, nil)
	suite.mockHandler.EXPECT().Decode(declarative).Return(readOnlyVal, nil)
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	suite.mockHandler.EXPECT().Validate(incomingVal, readOnlyVal, nil).Return(nil)
	suite.mockStore.EXPECT().
		UpsertServerConfig(mock.Anything, ServerConfig{Name: ConfigNameCORS, Value: incomingRaw}).Return(nil)
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw)
	assert.Nil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestSetConfig_UpsertError() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	suite.mockHandler.EXPECT().Validate(incomingVal, nil, nil).Return(nil)
	suite.mockStore.EXPECT().UpsertServerConfig(mock.Anything, mock.Anything).Return(errors.New("db error"))
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}
