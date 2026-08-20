// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type FileBasedStoreTestSuite struct {
	suite.Suite
	store *fileBasedStore
}

func TestFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedStoreTestSuite))
}

// SetupTest clears the shared singleton store so each test starts empty.
func (suite *FileBasedStoreTestSuite) SetupTest() {
	suite.store = newFileBasedStore()
	suite.Require().NoError(suite.store.ClearByType())
}

func (suite *FileBasedStoreTestSuite) TestCreateAndGetByName() {
	value := json.RawMessage(`["https://app.example.com"]`)
	suite.NoError(suite.store.Create("cors", &serverConfigDoc{Name: ConfigNameCORS, Value: value}))

	got, ok := suite.store.GetByName(context.Background(), ConfigNameCORS)
	suite.True(ok)
	suite.Equal(value, got)
}

func (suite *FileBasedStoreTestSuite) TestGetByNameMissing() {
	got, ok := suite.store.GetByName(context.Background(), ConfigNameCORS)
	suite.False(ok)
	suite.Nil(got)
}

func (suite *FileBasedStoreTestSuite) TestCreateRejectsUnexpectedType() {
	suite.Error(suite.store.Create("cors", "not a doc"))
}

func (suite *FileBasedStoreTestSuite) TestCreateOverwritesOnRepeatedName() {
	suite.NoError(suite.store.Create("cors", &serverConfigDoc{Name: ConfigNameCORS, Value: json.RawMessage(`["a"]`)}))
	suite.NoError(suite.store.Create("cors", &serverConfigDoc{Name: ConfigNameCORS, Value: json.RawMessage(`["b"]`)}))

	got, ok := suite.store.GetByName(context.Background(), ConfigNameCORS)
	suite.True(ok)
	suite.Equal(json.RawMessage(`["b"]`), got)
}

func (suite *FileBasedStoreTestSuite) TestGetServerConfig_ServesReadOnlyLayer() {
	value := json.RawMessage(`["https://static.example.com"]`)
	suite.NoError(suite.store.Create("cors", &serverConfigDoc{Name: ConfigNameCORS, Value: value}))

	layers, err := suite.store.GetServerConfig(context.Background(), ConfigNameCORS)
	suite.NoError(err)
	suite.Equal(value, layers.ReadOnly)
	suite.Nil(layers.Writable)
}

func (suite *FileBasedStoreTestSuite) TestGetServerConfig_Unset() {
	layers, err := suite.store.GetServerConfig(context.Background(), ConfigNameCORS)
	suite.NoError(err)
	suite.Equal(storeLayers{}, layers)
}

func (suite *FileBasedStoreTestSuite) TestUpsertServerConfig_Rejected() {
	err := suite.store.UpsertServerConfig(context.Background(), ServerConfig{Name: ConfigNameCORS, Value: corsValue})
	suite.Error(err)
}
