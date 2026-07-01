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

	got, ok := suite.store.GetByName(ConfigNameCORS)
	suite.True(ok)
	suite.Equal(value, got)
}

func (suite *FileBasedStoreTestSuite) TestGetByNameMissing() {
	got, ok := suite.store.GetByName(ConfigNameCORS)
	suite.False(ok)
	suite.Nil(got)
}

func (suite *FileBasedStoreTestSuite) TestCreateRejectsUnexpectedType() {
	suite.Error(suite.store.Create("cors", "not a doc"))
}

func (suite *FileBasedStoreTestSuite) TestCreateOverwritesOnRepeatedName() {
	suite.NoError(suite.store.Create("cors", &serverConfigDoc{Name: ConfigNameCORS, Value: json.RawMessage(`["a"]`)}))
	suite.NoError(suite.store.Create("cors", &serverConfigDoc{Name: ConfigNameCORS, Value: json.RawMessage(`["b"]`)}))

	got, ok := suite.store.GetByName(ConfigNameCORS)
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
