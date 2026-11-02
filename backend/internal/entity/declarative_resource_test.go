// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/hashmock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
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
	cfg := DeclarativeLoaderConfig{Directory: "users", Category: providers.EntityCategoryUser}

	err := loadDeclarativeResources(mockStore, mockSvc, cfg)
	s.NoError(err)
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_FileStore_EmptyDirectory() {
	tmpDir := s.T().TempDir()
	resourceDir := filepath.Join(tmpDir, "config", "resources", "users")
	s.Require().NoError(os.MkdirAll(resourceDir, 0750))

	s.Require().NoError(config.InitializeServerRuntime(tmpDir, &config.Config{}))

	fileStore := newEntityFileBasedStore()
	mockSvc := newEntityServiceMock(s.T())

	cfg := DeclarativeLoaderConfig{
		Directory: "users",
		Category:  providers.EntityCategoryUser,
		Parser: func(data []byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
			return &providers.Entity{ID: "test-id"}, nil, nil, nil
		},
	}

	err := loadDeclarativeResources(fileStore, mockSvc, cfg)
	s.NoError(err)
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_CompositeStore_ExtractsFileStore() {
	tmpDir := s.T().TempDir()
	resourceDir := filepath.Join(tmpDir, "config", "resources", "users")
	s.Require().NoError(os.MkdirAll(resourceDir, 0750))
	s.Require().NoError(config.InitializeServerRuntime(tmpDir, &config.Config{}))

	fileStore := newEntityFileBasedStore()
	dbStoreMock := newEntityStoreInterfaceMock(s.T())
	compositeStore := newEntityCompositeStore(fileStore, dbStoreMock)
	mockSvc := newEntityServiceMock(s.T())

	cfg := DeclarativeLoaderConfig{
		Directory: "users",
		Category:  providers.EntityCategoryUser,
		Parser: func(data []byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
			return &providers.Entity{ID: "test-id"}, nil, nil, nil
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

	cfg := DeclarativeLoaderConfig{Directory: "users", Category: providers.EntityCategoryUser}
	err := loadDeclarativeResources(compositeStore, mockSvc, cfg)
	s.NoError(err)
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_WithValidator_Called() {
	tmpDir := s.T().TempDir()
	resourceDir := filepath.Join(tmpDir, "config", "resources", "items")
	s.Require().NoError(os.MkdirAll(resourceDir, 0750))

	entityYAML := []byte(`id: "item-1"
ouId: "ou-1"
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
		Category:  providers.EntityCategoryUser,
		Parser: func(data []byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
			attrs, _ := json.Marshal(map[string]interface{}{})
			return &providers.Entity{ID: "item-1", Category: providers.EntityCategoryUser, Type: "thing",
				OUID: "ou-1", Attributes: json.RawMessage(attrs)}, nil, nil, nil
		},
		Validator: func(e *providers.Entity, svc EntityServiceInterface) error {
			validatorCalled = true
			return nil
		},
		IDExtractor: func(e *providers.Entity) string {
			return e.ID
		},
	}

	err := loadDeclarativeResources(fileStore, mockSvc, cfg)
	s.NoError(err)
	s.True(validatorCalled)
}

// A near miss of SUSPENDED loads as written, so the entity is not suspended, and is logged with the
// resource ID; other states log nothing.
func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_WarnsOnANearMissOfSuspended() {
	const warning = "Declarative state is not SUSPENDED"
	for name, tc := range map[string]struct {
		yamlState string
		want      providers.EntityState
		warns     bool
	}{
		"lower case":     {"state: suspended\n", "suspended", true},
		"mixed case":     {"state: Suspended\n", "Suspended", true},
		"spaces":         {"state: \" SUSPENDED \"\n", " SUSPENDED ", true},
		"exact":          {"state: SUSPENDED\n", providers.EntityStateSuspended, false},
		"empty":          {"", providers.EntityStateActive, false},
		"another value":  {"state: DISABLED\n", "DISABLED", false},
		"another active": {"state: active\n", "active", false},
	} {
		config.ResetServerRuntime()
		tmpDir := s.T().TempDir()
		resourceDir := filepath.Join(tmpDir, "config", "resources", "items")
		s.Require().NoError(os.MkdirAll(resourceDir, 0750))
		s.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "item.yaml"),
			[]byte("id: near-miss-item\n"+tc.yamlState), 0600))
		s.Require().NoError(config.InitializeServerRuntime(tmpDir, &config.Config{}))

		fileStore := newEntityFileBasedStore()
		cfg := DeclarativeLoaderConfig{
			Directory: "items",
			Category:  providers.EntityCategoryUser,
			Parser: func(data []byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
				return &providers.Entity{ID: "near-miss-item", Category: providers.EntityCategoryUser,
					Type: "thing", OUID: "ou-1", Attributes: json.RawMessage(`{}`)}, nil, nil, nil
			},
		}

		out := testhelpers.CaptureLogs(s.T(), "INFO", func() {
			s.Require().NoError(loadDeclarativeResources(fileStore, newEntityServiceMock(s.T()), cfg), name)
		})

		got, err := fileStore.GetEntity(context.Background(), "near-miss-item")
		s.Require().NoError(err, name)
		s.Equal(tc.want, got.State, "%s: the state must load as written", name)
		if tc.warns {
			s.Contains(out, warning, name)
			s.Contains(out, "entityID=", "%s: the warning must name the resource", name)
			s.NotContains(out, "near-miss-item", "%s: the resource ID must be masked", name)
		} else {
			s.NotContains(out, warning, name)
		}
	}
}

// A state longer than the runtime column holds refuses the load, naming the entity; one that fits loads.
func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_RefusesAStateLongerThanTheColumn() {
	for length, refused := range map[int]bool{maxEntityStateLength: false, maxEntityStateLength + 1: true} {
		config.ResetServerRuntime()
		state := strings.Repeat("S", length)
		tmpDir := s.T().TempDir()
		resourceDir := filepath.Join(tmpDir, "config", "resources", "items")
		s.Require().NoError(os.MkdirAll(resourceDir, 0750))
		s.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "item.yaml"),
			[]byte("id: long-state-item\nstate: "+state+"\n"), 0600))
		s.Require().NoError(config.InitializeServerRuntime(tmpDir, &config.Config{}))
		fileStore := newEntityFileBasedStore()
		cfg := DeclarativeLoaderConfig{
			Directory: "items",
			Category:  providers.EntityCategoryUser,
			Parser: func(data []byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
				return &providers.Entity{ID: "long-state-item", Category: providers.EntityCategoryUser,
					Type: "thing", OUID: "ou-1", Attributes: json.RawMessage(`{}`)}, nil, nil, nil
			},
		}

		err := loadDeclarativeResources(fileStore, newEntityServiceMock(s.T()), cfg)

		if refused {
			s.Require().Error(err, "%d characters", length)
			s.Contains(err.Error(), "state of entity long-state-item is longer than 50 characters")
			continue
		}
		s.Require().NoError(err, "%d characters", length)
		got, err := fileStore.GetEntity(context.Background(), "long-state-item")
		s.Require().NoError(err)
		s.Equal(providers.EntityState(state), got.State)
	}
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_ParserError() {
	tmpDir := s.T().TempDir()
	resourceDir := filepath.Join(tmpDir, "config", "resources", "items")
	s.Require().NoError(os.MkdirAll(resourceDir, 0750))
	s.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "bad.yaml"), []byte("id: x"), 0600))
	s.Require().NoError(config.InitializeServerRuntime(tmpDir, &config.Config{}))

	fileStore := newEntityFileBasedStore()
	mockSvc := newEntityServiceMock(s.T())

	cfg := DeclarativeLoaderConfig{
		Directory: "items",
		Category:  providers.EntityCategoryUser,
		Parser: func(data []byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
			return nil, nil, nil, errors.New("parse failed")
		},
	}

	err := loadDeclarativeResources(fileStore, mockSvc, cfg)
	s.Error(err)
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_ValidatorError() {
	tmpDir := s.T().TempDir()
	resourceDir := filepath.Join(tmpDir, "config", "resources", "items")
	s.Require().NoError(os.MkdirAll(resourceDir, 0750))
	s.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "item.yaml"), []byte("id: x"), 0600))
	s.Require().NoError(config.InitializeServerRuntime(tmpDir, &config.Config{}))

	fileStore := newEntityFileBasedStore()
	mockSvc := newEntityServiceMock(s.T())

	cfg := DeclarativeLoaderConfig{
		Directory: "items",
		Category:  providers.EntityCategoryUser,
		Parser: func(data []byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
			return &providers.Entity{ID: "x"}, nil, nil, nil
		},
		Validator: func(e *providers.Entity, svc EntityServiceInterface) error {
			return errors.New("validation failed")
		},
	}

	err := loadDeclarativeResources(fileStore, mockSvc, cfg)
	s.Error(err)
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_HashesSystemCredentialsBeforeStoreWrite() {
	tmpDir := s.T().TempDir()
	resourceDir := filepath.Join(tmpDir, "config", "resources", "applications")
	s.Require().NoError(os.MkdirAll(resourceDir, 0750))
	s.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "app.yaml"), []byte("id: x"), 0600))
	s.Require().NoError(config.InitializeServerRuntime(tmpDir, &config.Config{}))

	fileStore := newEntityFileBasedStore()
	hashService := hashmock.NewHashServiceInterfaceMock(s.T())
	hashService.On("Generate", []byte("plain-secret")).Return(cryptolib.Credential{
		Algorithm: "PBKDF2",
		Hash:      "hashed-secret",
		Parameters: cryptolib.CredParameters{
			Salt: "salt", Iterations: 1, KeySize: 32,
		},
	}, nil).Once()
	svc := newEntityService(fileStore, nil, hashService, nil, nil, transaction.NewNoOpTransactioner(), nil)

	cfg := DeclarativeLoaderConfig{
		Directory: "applications",
		Category:  providers.EntityCategoryApp,
		Parser: func(data []byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
			return &providers.Entity{
					ID: "app-1", Category: providers.EntityCategoryApp,
					Type: "application", State: providers.EntityStateActive,
				},
				nil,
				json.RawMessage(`{"clientSecret":"plain-secret"}`),
				nil
		},
		IDExtractor: func(e *providers.Entity) string {
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
		StorageAlgoParams: cryptolib.CredParameters{
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
