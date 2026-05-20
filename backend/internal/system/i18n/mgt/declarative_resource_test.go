/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package mgt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

type DeclarativeResourceTestSuite struct {
	suite.Suite
	mockStore *i18nStoreInterfaceMock
	exporter  declarativeresource.ResourceExporter
}

func TestDeclarativeResourceTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeResourceTestSuite))
}

func (s *DeclarativeResourceTestSuite) SetupTest() {
	s.mockStore = newI18nStoreInterfaceMock(s.T())
	s.exporter = newTranslationExporter(s.mockStore)
}

func (s *DeclarativeResourceTestSuite) TestGetResourceType() {
	resourceType := s.exporter.GetResourceType()
	assert.Equal(s.T(), "translation", resourceType)
}

func (s *DeclarativeResourceTestSuite) TestGetParameterizerType() {
	paramType := s.exporter.GetParameterizerType()
	assert.Equal(s.T(), "Translation", paramType)
}

func (s *DeclarativeResourceTestSuite) TestGetResourceByID() {
	translations := map[string]map[string]Translation{
		"welcome": {
			"en-US": {
				Key:       "welcome",
				Language:  "en-US",
				Namespace: "common",
				Value:     "Welcome",
			},
		},
		"goodbye": {
			"en-US": {
				Key:       "goodbye",
				Language:  "en-US",
				Namespace: "common",
				Value:     "Goodbye",
			},
		},
	}

	s.mockStore.On("GetTranslations").Return(translations, nil)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "en-US")
	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "en-US", name)
	assert.NotNil(s.T(), resource)

	trans, ok := resource.(*LanguageTranslations)
	assert.True(s.T(), ok)
	assert.Equal(s.T(), "en-US", trans.Language)
	assert.Equal(s.T(), "Welcome", trans.Translations["common"]["welcome"])
	assert.Equal(s.T(), "Goodbye", trans.Translations["common"]["goodbye"])
}

func (s *DeclarativeResourceTestSuite) TestGetResourceByID_NotFound() {
	s.mockStore.On("GetTranslations").Return(map[string]map[string]Translation{}, nil)

	_, _, err := s.exporter.GetResourceByID(context.Background(), "fr-FR")
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "TRANSLATION_NOT_FOUND", err.Code)
}

func (s *DeclarativeResourceTestSuite) TestGetResourceByID_StoreError() {
	s.mockStore.On("GetTranslations").Return(nil, errors.New("db error"))

	_, _, err := s.exporter.GetResourceByID(context.Background(), "en-US")
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "I18N_FETCH_ERROR", err.Code)
}

func (s *DeclarativeResourceTestSuite) TestValidateResource() {
	trans := &LanguageTranslations{
		Language: "en-US",
		Translations: map[string]map[string]string{
			"common": {"ok": "OK"},
		},
	}

	name, err := s.exporter.ValidateResource(trans, "en-US", nil)
	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "en-US", name)
}

func (s *DeclarativeResourceTestSuite) TestValidateResourceInvalidType() {
	invalidResource := "not a translation"

	name, err := s.exporter.ValidateResource(invalidResource, "en-US", nil)
	assert.NotNil(s.T(), err)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), "INVALID_TYPE", err.Code)
}

func (s *DeclarativeResourceTestSuite) TestValidateResourceMissingLanguage() {
	trans := &LanguageTranslations{
		Translations: map[string]map[string]string{
			"common": {"ok": "OK"},
		},
	}

	name, err := s.exporter.ValidateResource(trans, "en-US", nil)
	assert.NotNil(s.T(), err)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), "INVALID_TRANSLATION", err.Code)
}

func (s *DeclarativeResourceTestSuite) TestValidateResourceMissingTranslations() {
	trans := &LanguageTranslations{
		Language: "en-US",
	}

	name, err := s.exporter.ValidateResource(trans, "en-US", nil)
	assert.NotNil(s.T(), err)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), "INVALID_TRANSLATION", err.Code)
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDs() {
	languages := []string{"en-US", "fr-FR"}
	s.mockStore.On("GetDistinctLanguages").Return(languages, nil)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())
	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.Contains(s.T(), ids, "en-US")
	assert.Contains(s.T(), ids, "fr-FR")
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDs_StoreError() {
	s.mockStore.On("GetDistinctLanguages").Return(nil, errors.New("db error"))

	ids, err := s.exporter.GetAllResourceIDs(context.Background())
	assert.NotNil(s.T(), err)
	assert.Nil(s.T(), ids)
	assert.Equal(s.T(), "I18N_EXPORT_ERROR", err.Code)
}

func (s *DeclarativeResourceTestSuite) TestParseToLangTranslation() {
	yamlData := []byte(`
language: en-US
translations:
  common:
    welcome: Welcome
    goodbye: Goodbye
`)

	trans, err := parseToLangTranslation(yamlData)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), trans)
	assert.Equal(s.T(), "en-US", trans.Language)
	assert.Equal(s.T(), "Welcome", trans.Translations["common"]["welcome"])
}

func (s *DeclarativeResourceTestSuite) TestValidateTranslationWrapper() {
	store := newFileBasedStore().(*fileBasedStore)
	trans := &LanguageTranslations{
		Language: "en-US",
		Translations: map[string]map[string]string{
			"common": {"ok": "OK"},
		},
	}

	err := validateTranslationWrapper(trans, store)
	assert.NoError(s.T(), err)
}

func (s *DeclarativeResourceTestSuite) TestValidateTranslationWrapperInvalidType() {
	store := newFileBasedStore().(*fileBasedStore)
	err := validateTranslationWrapper("invalid", store)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "invalid type")
}

func (s *DeclarativeResourceTestSuite) TestValidateTranslationWrapperMissingLang() {
	store := newFileBasedStore().(*fileBasedStore)
	trans := &LanguageTranslations{
		Translations: map[string]map[string]string{
			"common": {"ok": "OK"},
		},
	}
	err := validateTranslationWrapper(trans, store)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "language is required")
}

func (s *DeclarativeResourceTestSuite) TestValidateTranslationWrapperMissingTrans() {
	store := newFileBasedStore().(*fileBasedStore)
	trans := &LanguageTranslations{
		Language: "en-US",
	}
	err := validateTranslationWrapper(trans, store)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "translations is required")
}

func (s *DeclarativeResourceTestSuite) TestValidateTranslationWrapperDuplicateID() {
	store := newFileBasedStore().(*fileBasedStore)
	trans := &LanguageTranslations{
		Language: "en-US",
		Translations: map[string]map[string]string{
			"common": {"ok": "OK"},
		},
	}

	err := store.Create("en-US", trans)
	assert.NoError(s.T(), err)

	// validate duplicate
	err = validateTranslationWrapper(trans, store)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "duplicate translation ID")
}

func (s *DeclarativeResourceTestSuite) TestGetResourceRules() {
	rules := s.exporter.GetResourceRules()
	assert.NotNil(s.T(), rules)
	assert.Empty(s.T(), rules.Variables)
	assert.Empty(s.T(), rules.ArrayVariables)
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_InvalidStoreType() {
	// Pass a mock store that is NOT *fileBasedStore
	// s.mockStore is *i18nStoreInterfaceMock which satisfies i18nStoreInterface
	// but is not *fileBasedStore.
	err := loadDeclarativeResources(s.mockStore)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "fileStore must be a file-based store implementation")
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_Success() {
	// Setup temp dir
	tempDir, err := os.MkdirTemp("", "test_resources_success")
	assert.NoError(s.T(), err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	// Setup Runtime with temp dir
	config.ResetServerRuntime()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	_ = config.InitializeServerRuntime(tempDir, testConfig)
	defer config.ResetServerRuntime()

	// Create translations directory
	translationsDir := filepath.Join(tempDir, "repository", "resources", "translations")
	err = os.MkdirAll(translationsDir, 0750)
	assert.NoError(s.T(), err)

	// Create valid YAML file
	yamlContent := []byte(`
language: es-ES
translations:
  common:
    hello: Hola
`)
	validFile := filepath.Join(translationsDir, "es.yaml")
	err = os.WriteFile(validFile, yamlContent, 0600)
	assert.NoError(s.T(), err)

	// Create actual fileStore (using generic store underneath)
	// We use NewGenericFileBasedStoreForTest to avoid using the singleton entity store
	// and ensuring test isolation.
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeTranslation)
	store := &fileBasedStore{
		GenericFileBasedStore: genericStore,
	}

	err = loadDeclarativeResources(store)
	assert.NoError(s.T(), err)

	exists, err := store.IsTranslationExists("es-ES")
	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
}

func (s *DeclarativeResourceTestSuite) TestParseToTranslationWrapper() {
	yamlData := []byte(`
language: de-DE
translations:
  common:
    hello: Hallo
`)
	data, err := parseToTranslationWrapper(yamlData)
	assert.NoError(s.T(), err)
	trans, ok := data.(*LanguageTranslations)
	assert.True(s.T(), ok)
	assert.Equal(s.T(), "de-DE", trans.Language)
}

func (s *DeclarativeResourceTestSuite) TestParseToLangTranslation_Error() {
	yamlData := []byte(`invalid_yaml: [`)
	_, err := parseToLangTranslation(yamlData)
	assert.Error(s.T(), err)
}
