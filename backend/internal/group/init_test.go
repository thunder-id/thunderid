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

package group

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type GroupInitTestSuite struct {
	suite.Suite
}

func TestGroupInitTestSuite(t *testing.T) {
	suite.Run(t, new(GroupInitTestSuite))
}

func (s *GroupInitTestSuite) SetupTest() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	})
}

func (s *GroupInitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// --- getGroupStoreMode ---

func (s *GroupInitTestSuite) TestGetGroupStoreMode_ExplicitMutable() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Group: config.GroupConfig{Store: "mutable"},
	})

	mode, err := getGroupStoreMode()

	s.NoError(err)
	s.Equal(serverconst.StoreModeMutable, mode)
}

func (s *GroupInitTestSuite) TestGetGroupStoreMode_ExplicitDeclarative() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Group: config.GroupConfig{Store: "declarative"},
	})

	mode, err := getGroupStoreMode()

	s.NoError(err)
	s.Equal(serverconst.StoreModeDeclarative, mode)
}

func (s *GroupInitTestSuite) TestGetGroupStoreMode_ExplicitComposite() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Group: config.GroupConfig{Store: "composite"},
	})

	mode, err := getGroupStoreMode()

	s.NoError(err)
	s.Equal(serverconst.StoreModeComposite, mode)
}

func (s *GroupInitTestSuite) TestGetGroupStoreMode_FallbackToDeclarative() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: true},
		Group:                config.GroupConfig{Store: ""},
	})

	mode, err := getGroupStoreMode()

	s.NoError(err)
	s.Equal(serverconst.StoreModeDeclarative, mode)
}

func (s *GroupInitTestSuite) TestGetGroupStoreMode_FallbackToMutable() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
		Group:                config.GroupConfig{Store: ""},
	})

	mode, err := getGroupStoreMode()

	s.NoError(err)
	s.Equal(serverconst.StoreModeMutable, mode)
}

func (s *GroupInitTestSuite) TestGetGroupStoreMode_InvalidValue() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Group: config.GroupConfig{Store: "invalid"},
	})

	mode, err := getGroupStoreMode()

	s.Error(err)
	s.Empty(mode)
	s.Contains(err.Error(), "invalid group store mode")
}

// --- isGroupDeclarativeModeEnabled ---

func (s *GroupInitTestSuite) TestIsGroupDeclarativeModeEnabled_True() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Group: config.GroupConfig{Store: "declarative"},
	})

	s.True(isGroupDeclarativeModeEnabled())
}

func (s *GroupInitTestSuite) TestIsGroupDeclarativeModeEnabled_False() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Group: config.GroupConfig{Store: "mutable"},
	})

	s.False(isGroupDeclarativeModeEnabled())
}

// --- initializeGroupStore ---

func (s *GroupInitTestSuite) TestInitializeGroupStore_MutableMode() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Group: config.GroupConfig{Store: "mutable"},
	})

	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetUserDBTransactioner").Return(transaction.NewNoOpTransactioner(), nil)

	store, txer, fileStore, dbStore, err := initializeGroupStore(mockProvider)

	s.NoError(err)
	s.NotNil(store)
	s.NotNil(txer)
	s.Nil(fileStore)
	s.Nil(dbStore)
	mockProvider.AssertExpectations(s.T())
}

func (s *GroupInitTestSuite) TestInitializeGroupStore_DeclarativeMode() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Group: config.GroupConfig{Store: "declarative"},
	})

	mockProvider := &providermock.DBProviderInterfaceMock{}

	store, txer, fileStore, dbStore, err := initializeGroupStore(mockProvider)

	s.NoError(err)
	s.NotNil(store)
	s.NotNil(txer)
	s.NotNil(fileStore)
	s.Nil(dbStore)
	mockProvider.AssertExpectations(s.T())
}

func (s *GroupInitTestSuite) TestInitializeGroupStore_CompositeMode() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Group: config.GroupConfig{Store: "composite"},
	})

	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetUserDBTransactioner").Return(transaction.NewNoOpTransactioner(), nil)

	store, txer, fileStore, dbStore, err := initializeGroupStore(mockProvider)

	s.NoError(err)
	s.NotNil(store)
	s.NotNil(txer)
	s.NotNil(fileStore)
	s.NotNil(dbStore)
	mockProvider.AssertExpectations(s.T())
}

func (s *GroupInitTestSuite) TestInitializeGroupStore_MutableMode_TransactionerError() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Group: config.GroupConfig{Store: "mutable"},
	})

	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetUserDBTransactioner").Return(nil, errors.New("db transactioner error"))

	store, txer, fileStore, dbStore, err := initializeGroupStore(mockProvider)

	s.Error(err)
	s.Contains(err.Error(), "db transactioner error")
	s.Nil(store)
	s.Nil(txer)
	s.Nil(fileStore)
	s.Nil(dbStore)
	mockProvider.AssertExpectations(s.T())
}

func (s *GroupInitTestSuite) TestInitializeGroupStore_CompositeMode_TransactionerError() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Group: config.GroupConfig{Store: "composite"},
	})

	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetUserDBTransactioner").Return(nil, errors.New("db transactioner error"))

	store, txer, fileStore, dbStore, err := initializeGroupStore(mockProvider)

	s.Error(err)
	s.Contains(err.Error(), "db transactioner error")
	s.Nil(store)
	s.Nil(txer)
	s.Nil(fileStore)
	s.Nil(dbStore)
	mockProvider.AssertExpectations(s.T())
}

func (s *GroupInitTestSuite) TestInitializeGroupStore_InvalidStoreMode() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Group: config.GroupConfig{Store: "invalid"},
	})

	mockProvider := &providermock.DBProviderInterfaceMock{}

	store, txer, fileStore, dbStore, err := initializeGroupStore(mockProvider)

	s.Error(err)
	s.Contains(err.Error(), "invalid group store mode")
	s.Nil(store)
	s.Nil(txer)
	s.Nil(fileStore)
	s.Nil(dbStore)
	mockProvider.AssertExpectations(s.T())
}
