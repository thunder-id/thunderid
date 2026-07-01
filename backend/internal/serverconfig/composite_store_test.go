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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CompositeStoreTestSuite struct {
	suite.Suite
	ctx       context.Context
	mockFile  *serverConfigStoreInterfaceMock
	mockDB    *serverConfigStoreInterfaceMock
	composite serverConfigStoreInterface
}

func TestCompositeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeStoreTestSuite))
}

func (suite *CompositeStoreTestSuite) SetupTest() {
	suite.ctx = context.Background()
	suite.mockFile = newServerConfigStoreInterfaceMock(suite.T())
	suite.mockDB = newServerConfigStoreInterfaceMock(suite.T())
	suite.composite = newCompositeServerConfigStore(suite.mockFile, suite.mockDB)
}

func (suite *CompositeStoreTestSuite) TestGetServerConfig_CombinesLayers() {
	suite.mockFile.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative}, nil)
	suite.mockDB.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{Writable: corsValue}, nil)

	layers, err := suite.composite.GetServerConfig(suite.ctx, ConfigNameCORS)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), declarative, layers.ReadOnly)
	assert.Equal(suite.T(), corsValue, layers.Writable)
}

func (suite *CompositeStoreTestSuite) TestGetServerConfig_FileError() {
	suite.mockFile.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{}, errors.New("file error"))

	_, err := suite.composite.GetServerConfig(suite.ctx, ConfigNameCORS)
	assert.Error(suite.T(), err)
	suite.mockDB.AssertNotCalled(suite.T(), "GetServerConfig", mock.Anything, mock.Anything)
}

func (suite *CompositeStoreTestSuite) TestGetServerConfig_DBError() {
	suite.mockFile.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative}, nil)
	suite.mockDB.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{}, errors.New("db error"))

	_, err := suite.composite.GetServerConfig(suite.ctx, ConfigNameCORS)
	assert.Error(suite.T(), err)
}

func (suite *CompositeStoreTestSuite) TestUpsertServerConfig_GoesToDB() {
	cfg := ServerConfig{Name: ConfigNameCORS, Value: corsValue}
	suite.mockDB.EXPECT().UpsertServerConfig(mock.Anything, cfg).Return(nil)

	assert.NoError(suite.T(), suite.composite.UpsertServerConfig(suite.ctx, cfg))
	suite.mockFile.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}
