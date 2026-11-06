// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
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

func (suite *CompositeStoreTestSuite) TestReplaceServerConfigIf_GoesToDB() {
	cfg := ServerConfig{Name: ConfigNameCORS, Value: corsValue}
	suite.mockDB.EXPECT().ReplaceServerConfigIf(mock.Anything, cfg, json.RawMessage(nil)).Return(true, nil)

	written, err := suite.composite.ReplaceServerConfigIf(suite.ctx, cfg, nil)

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), written)
}
