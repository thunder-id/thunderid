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

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

type ServiceTestSuite struct {
	suite.Suite
	ctx       context.Context
	mockStore *serverConfigStoreInterfaceMock
	service   ServerConfigService
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (suite *ServiceTestSuite) SetupTest() {
	suite.ctx = context.Background()
	suite.mockStore = newServerConfigStoreInterfaceMock(suite.T())
	suite.service = newServerConfigService(suite.mockStore)
}

// registerValidator registers a mock validator for cors that returns validErr.
func (suite *ServiceTestSuite) registerValidator(validErr error) {
	v := NewServerConfigValidatorInterfaceMock(suite.T())
	v.EXPECT().Validate(mock.Anything).Return(validErr)
	suite.service.RegisterValidator(ConfigNameCORS, v)
}

var corsValue = json.RawMessage(`["https://app.example.com"]`)

// --- GetConfig ---

func (suite *ServiceTestSuite) TestGetConfig_UnsupportedName() {
	raw, svcErr := suite.service.GetConfig(suite.ctx, ConfigName("bogus"))
	assert.Nil(suite.T(), raw)
	assert.Same(suite.T(), &ErrorUnsupportedConfigName, svcErr)
}

func (suite *ServiceTestSuite) TestGetConfig_NotFound() {
	suite.mockStore.EXPECT().GetServerConfigByName(mock.Anything, ConfigNameCORS).Return(nil, nil)
	raw, svcErr := suite.service.GetConfig(suite.ctx, ConfigNameCORS)
	assert.Nil(suite.T(), raw)
	assert.Same(suite.T(), &ErrorConfigNotFound, svcErr)
}

func (suite *ServiceTestSuite) TestGetConfig_StoreError() {
	suite.mockStore.EXPECT().GetServerConfigByName(mock.Anything, ConfigNameCORS).
		Return(nil, errors.New("db error"))
	raw, svcErr := suite.service.GetConfig(suite.ctx, ConfigNameCORS)
	assert.Nil(suite.T(), raw)
	assert.Same(suite.T(), &serviceerror.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetConfig_OK() {
	suite.mockStore.EXPECT().GetServerConfigByName(mock.Anything, ConfigNameCORS).
		Return(&ServerConfig{Name: ConfigNameCORS, Value: corsValue}, nil)
	raw, svcErr := suite.service.GetConfig(suite.ctx, ConfigNameCORS)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), corsValue, raw)
}

// --- SetConfig ---

func (suite *ServiceTestSuite) TestSetConfig_NoValidatorRegistered_FailClosed() {
	// No validator registered for cors → server misconfiguration, not persisted.
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, corsValue)
	assert.Same(suite.T(), &serviceerror.InternalServerError, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfig_UnsupportedName() {
	svcErr := suite.service.SetConfig(suite.ctx, ConfigName("bogus"), corsValue)
	assert.Same(suite.T(), &ErrorUnsupportedConfigName, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfig_ValidatorRejects() {
	suite.registerValidator(errors.New("bad value"))
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, corsValue)
	assert.Same(suite.T(), &ErrorInvalidConfigValue, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfig_OK() {
	suite.registerValidator(nil)
	suite.mockStore.EXPECT().
		UpsertServerConfig(mock.Anything, ServerConfig{Name: ConfigNameCORS, Value: corsValue}).
		Return(nil)
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, corsValue)
	assert.Nil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestSetConfig_StoreError() {
	suite.registerValidator(nil)
	suite.mockStore.EXPECT().UpsertServerConfig(mock.Anything, mock.Anything).
		Return(errors.New("db error"))
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, corsValue)
	assert.Same(suite.T(), &serviceerror.InternalServerError, svcErr)
}

// --- SetConfigs ---

func (suite *ServiceTestSuite) TestSetConfigs_OneBadRejectsWhole_NoWrite() {
	suite.registerValidator(errors.New("bad value"))
	svcErr := suite.service.SetConfigs(suite.ctx, map[ConfigName]json.RawMessage{ConfigNameCORS: corsValue})
	assert.Same(suite.T(), &ErrorInvalidConfigValue, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfigs", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfigs_OK() {
	suite.registerValidator(nil)
	suite.mockStore.EXPECT().
		UpsertServerConfigs(mock.Anything, mock.MatchedBy(func(cfgs []ServerConfig) bool {
			return len(cfgs) == 1 && cfgs[0].Name == ConfigNameCORS
		})).
		Return(nil)
	svcErr := suite.service.SetConfigs(suite.ctx, map[ConfigName]json.RawMessage{ConfigNameCORS: corsValue})
	assert.Nil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestSetConfigs_Empty_NoOp() {
	svcErr := suite.service.SetConfigs(suite.ctx, map[ConfigName]json.RawMessage{})
	assert.Nil(suite.T(), svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfigs", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfigs_StoreError() {
	suite.registerValidator(nil)
	suite.mockStore.EXPECT().UpsertServerConfigs(mock.Anything, mock.Anything).
		Return(errors.New("db error"))
	svcErr := suite.service.SetConfigs(suite.ctx, map[ConfigName]json.RawMessage{ConfigNameCORS: corsValue})
	assert.Same(suite.T(), &serviceerror.InternalServerError, svcErr)
}

// --- ListConfigs ---

func (suite *ServiceTestSuite) TestListConfigs_OK() {
	suite.mockStore.EXPECT().GetServerConfigList(mock.Anything).
		Return([]ServerConfig{{Name: ConfigNameCORS, Value: corsValue}}, nil)
	configs, svcErr := suite.service.ListConfigs(suite.ctx)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), corsValue, configs[ConfigNameCORS])
}

func (suite *ServiceTestSuite) TestListConfigs_StoreError() {
	suite.mockStore.EXPECT().GetServerConfigList(mock.Anything).Return(nil, errors.New("db error"))
	configs, svcErr := suite.service.ListConfigs(suite.ctx)
	assert.Nil(suite.T(), configs)
	assert.Same(suite.T(), &serviceerror.InternalServerError, svcErr)
}
