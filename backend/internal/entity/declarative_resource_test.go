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

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolab/hash"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/hashmock"
)

type DeclarativeResourceTestSuite struct {
	suite.Suite
}

func TestDeclarativeResourceTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeResourceTestSuite))
}

func (s *DeclarativeResourceTestSuite) SetupTest() {
	config.ResetServerRuntime()
}

func (s *DeclarativeResourceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_MutableStore_Skipped() {
	mockStore := newEntityStoreInterfaceMock(s.T())
	mockSvc := newEntityServiceMock(s.T())
	cfg := DeclarativeLoaderConfig{Directory: "users", Category: EntityCategoryUser}

	err := loadDeclarativeResources(mockStore, mockSvc, cfg)
	s.NoError(err)
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_FileStore_EmptyDirectory() {
	tmpDir := s.T().TempDir()
	resourceDir := filepath.Join(tmpDir, "repository", "resources", "users")
	s.Require().NoError(os.MkdirAll(resourceDir, 0750))

	s.Require().NoError(config.InitializeServerRuntime(tmpDir, &config.Config{}))

	fileStore := newEntityFileBasedStore()
	mockSvc := newEntityServiceMock(s.T())

	cfg := DeclarativeLoaderConfig{
		Directory: "users",
		Category:  EntityCategoryUser,
		Parser: func(data []byte) (*Entity, json.RawMessage, json.RawMessage, error) {
			return &Entity{ID: "test-id"}, nil, nil, nil
		},
	}

	err := loadDeclarativeResources(fileStore, mockSvc, cfg)
	s.NoError(err)
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_CompositeStore_ExtractsFileStore() {
	tmpDir := s.T().TempDir()
	resourceDir := filepath.Join(tmpDir, "repository", "resources", "users")
	s.Require().NoError(os.MkdirAll(resourceDir, 0750))
	s.Require().NoError(config.InitializeServerRuntime(tmpDir, &config.Config{}))

	fileStore := newEntityFileBasedStore()
	dbStoreMock := newEntityStoreInterfaceMock(s.T())
	compositeStore := newEntityCompositeStore(fileStore, dbStoreMock)
	mockSvc := newEntityServiceMock(s.T())

	cfg := DeclarativeLoaderConfig{
		Directory: "users",
		Category:  EntityCategoryUser,
		Parser: func(data []byte) (*Entity, json.RawMessage, json.RawMessage, error) {
			return &Entity{ID: "test-id"}, nil, nil, nil
		},
	}

	err := loadDeclarativeResources(compositeStore, mockSvc, cfg)
	s.NoError(err)
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_CompositeStore_NonFileStore_Skipped() {
	mockFileStore := newEntityStoreInterfaceMock(s.T())
	mockDBStore := newEntityStoreInterfaceMock(s.T())
	compositeStore := newEntityCompositeStore(mockFileStore, mockDBStore)
	mockSvc := newEntityServiceMock(s.T())

	cfg := DeclarativeLoaderConfig{Directory: "users", Category: EntityCategoryUser}
	err := loadDeclarativeResources(compositeStore, mockSvc, cfg)
	s.NoError(err)
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_WithValidator_Called() {
	tmpDir := s.T().TempDir()
	resourceDir := filepath.Join(tmpDir, "repository", "resources", "items")
	s.Require().NoError(os.MkdirAll(resourceDir, 0750))

	entityYAML := []byte(`id: "item-1"
ou_id: "ou-1"
type: "thing"
category: "user"
attributes: {}
`)
	s.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "item1.yaml"), entityYAML, 0600))
	s.Require().NoError(config.InitializeServerRuntime(tmpDir, &config.Config{}))

	fileStore := newEntityFileBasedStore()
	mockSvc := newEntityServiceMock(s.T())

	validatorCalled := false
	cfg := DeclarativeLoaderConfig{
		Directory: "items",
		Category:  EntityCategoryUser,
		Parser: func(data []byte) (*Entity, json.RawMessage, json.RawMessage, error) {
			attrs, _ := json.Marshal(map[string]interface{}{})
			return &Entity{ID: "item-1", Category: EntityCategoryUser, Type: "thing",
				OUID: "ou-1", Attributes: json.RawMessage(attrs)}, nil, nil, nil
		},
		Validator: func(e *Entity, svc EntityServiceInterface) error {
			validatorCalled = true
			return nil
		},
		IDExtractor: func(e *Entity) string {
			return e.ID
		},
	}

	err := loadDeclarativeResources(fileStore, mockSvc, cfg)
	s.NoError(err)
	s.True(validatorCalled)
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_ParserError() {
	tmpDir := s.T().TempDir()
	resourceDir := filepath.Join(tmpDir, "repository", "resources", "items")
	s.Require().NoError(os.MkdirAll(resourceDir, 0750))
	s.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "bad.yaml"), []byte("id: x"), 0600))
	s.Require().NoError(config.InitializeServerRuntime(tmpDir, &config.Config{}))

	fileStore := newEntityFileBasedStore()
	mockSvc := newEntityServiceMock(s.T())

	cfg := DeclarativeLoaderConfig{
		Directory: "items",
		Category:  EntityCategoryUser,
		Parser: func(data []byte) (*Entity, json.RawMessage, json.RawMessage, error) {
			return nil, nil, nil, errors.New("parse failed")
		},
	}

	err := loadDeclarativeResources(fileStore, mockSvc, cfg)
	s.Error(err)
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_ValidatorError() {
	tmpDir := s.T().TempDir()
	resourceDir := filepath.Join(tmpDir, "repository", "resources", "items")
	s.Require().NoError(os.MkdirAll(resourceDir, 0750))
	s.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "item.yaml"), []byte("id: x"), 0600))
	s.Require().NoError(config.InitializeServerRuntime(tmpDir, &config.Config{}))

	fileStore := newEntityFileBasedStore()
	mockSvc := newEntityServiceMock(s.T())

	cfg := DeclarativeLoaderConfig{
		Directory: "items",
		Category:  EntityCategoryUser,
		Parser: func(data []byte) (*Entity, json.RawMessage, json.RawMessage, error) {
			return &Entity{ID: "x"}, nil, nil, nil
		},
		Validator: func(e *Entity, svc EntityServiceInterface) error {
			return errors.New("validation failed")
		},
	}

	err := loadDeclarativeResources(fileStore, mockSvc, cfg)
	s.Error(err)
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_HashesSystemCredentialsBeforeStoreWrite() {
	tmpDir := s.T().TempDir()
	resourceDir := filepath.Join(tmpDir, "repository", "resources", "applications")
	s.Require().NoError(os.MkdirAll(resourceDir, 0750))
	s.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "app.yaml"), []byte("id: x"), 0600))
	s.Require().NoError(config.InitializeServerRuntime(tmpDir, &config.Config{}))

	fileStore := newEntityFileBasedStore()
	hashService := hashmock.NewHashServiceInterfaceMock(s.T())
	hashService.On("Generate", []byte("plain-secret")).Return(hash.Credential{
		Algorithm: "PBKDF2",
		Hash:      "hashed-secret",
		Parameters: hash.CredParameters{
			Salt: "salt", Iterations: 1, KeySize: 32,
		},
	}, nil).Once()
	svc := newEntityService(fileStore, hashService, nil, nil, transaction.NewNoOpTransactioner())

	cfg := DeclarativeLoaderConfig{
		Directory: "applications",
		Category:  EntityCategoryApp,
		Parser: func(data []byte) (*Entity, json.RawMessage, json.RawMessage, error) {
			return &Entity{ID: "app-1", Category: EntityCategoryApp, Type: "application", State: EntityStateActive},
				nil,
				json.RawMessage(`{"clientSecret":"plain-secret"}`),
				nil
		},
		IDExtractor: func(e *Entity) string {
			return e.ID
		},
	}

	err := loadDeclarativeResources(fileStore, svc, cfg)
	s.NoError(err)

	result, err := fileStore.GetEntityWithCredentials(context.Background(), "app-1")
	s.NoError(err)

	var systemCreds map[string][]StoredCredential
	err = json.Unmarshal(result.SystemCredentials, &systemCreds)
	s.NoError(err)
	s.Equal([]StoredCredential{{
		StorageAlgo: "PBKDF2",
		StorageAlgoParams: hash.CredParameters{
			Salt: "salt", Iterations: 1, KeySize: 32,
		},
		Value: "hashed-secret",
	}}, systemCreds["clientSecret"])
}

func newEntityServiceMock(t *testing.T) *EntityServiceInterfaceMock {
	m := &EntityServiceInterfaceMock{}
	m.Mock.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
