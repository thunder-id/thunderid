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
	"testing"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type FileBasedStoreTestSuite struct {
	suite.Suite
	store *fileBasedStore
}

func TestFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedStoreTestSuite))
}

func (s *FileBasedStoreTestSuite) SetupTest() {
	// Create a file-based store with test instance
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeTranslation)
	s.store = &fileBasedStore{
		GenericFileBasedStore: genericStore,
	}
}

func (s *FileBasedStoreTestSuite) TestCreateAndGetDistinctLanguages() {
	// Create translations for English
	enTrans := &LanguageTranslations{
		Language: "en-US",
		Translations: map[string]map[string]string{
			"common": {
				"welcome": "Welcome",
			},
		},
	}
	err := s.store.Create("en-US", enTrans)
	assert.NoError(s.T(), err)

	// Create translations for Spanish
	esTrans := &LanguageTranslations{
		Language: "es-ES",
		Translations: map[string]map[string]string{
			"common": {
				"welcome": "Bienvenido",
			},
		},
	}
	err = s.store.Create("es-ES", esTrans)
	assert.NoError(s.T(), err)

	// Verify distinct languages
	languages, err := s.store.GetDistinctLanguages()
	assert.NoError(s.T(), err)
	assert.Len(s.T(), languages, 2)
	assert.Contains(s.T(), languages, "en-US")
	assert.Contains(s.T(), languages, "es-ES")
}

func (s *FileBasedStoreTestSuite) TestGetTranslations() {
	// Setup data
	enTrans := &LanguageTranslations{
		Language: "en-US",
		Translations: map[string]map[string]string{
			"ns1": {
				"key1": "value1-en",
			},
		},
	}
	err := s.store.Create("en-US", enTrans)
	assert.NoError(s.T(), err)

	frTrans := &LanguageTranslations{
		Language: "fr-FR",
		Translations: map[string]map[string]string{
			"ns1": {
				"key1": "value1-fr",
			},
			"ns2": {
				"key2": "value2-fr",
			},
		},
	}
	err = s.store.Create("fr-FR", frTrans)
	assert.NoError(s.T(), err)

	// Get all translations
	translations, err := s.store.GetTranslations()
	assert.NoError(s.T(), err)

	// Verify structure
	assert.Contains(s.T(), translations, "ns1|key1")
	assert.Contains(s.T(), translations, "ns2|key2")

	assert.Contains(s.T(), translations["ns1|key1"], "en-US")
	assert.Equal(s.T(), "value1-en", translations["ns1|key1"]["en-US"].Value)
	assert.Equal(s.T(), "ns1", translations["ns1|key1"]["en-US"].Namespace)

	assert.Contains(s.T(), translations["ns1|key1"], "fr-FR")
	assert.Equal(s.T(), "value1-fr", translations["ns1|key1"]["fr-FR"].Value)

	assert.Contains(s.T(), translations["ns2|key2"], "fr-FR")
	assert.Equal(s.T(), "value2-fr", translations["ns2|key2"]["fr-FR"].Value)
}

func (s *FileBasedStoreTestSuite) TestGetTranslationsByNamespace() {
	// Setup data
	enTrans := &LanguageTranslations{
		Language: "en-US",
		Translations: map[string]map[string]string{
			"ns1": {
				"key1": "value1-en",
			},
			"ns2": {
				"key2": "value2-en",
			},
		},
	}
	err := s.store.Create("en-US", enTrans)
	assert.NoError(s.T(), err)

	// Get translations for ns1
	translations, err := s.store.GetTranslationsByNamespace("ns1")
	assert.NoError(s.T(), err)

	assert.Contains(s.T(), translations, "ns1|key1")
	assert.NotContains(s.T(), translations, "ns2|key2")
	assert.Equal(s.T(), "value1-en", translations["ns1|key1"]["en-US"].Value)
}

func (s *FileBasedStoreTestSuite) TestGetTranslationsByKey() {
	// Setup data
	enTrans := &LanguageTranslations{
		Language: "en-US",
		Translations: map[string]map[string]string{
			"ns1": {
				"key1": "value1-en",
			},
		},
	}
	err := s.store.Create("en-US", enTrans)
	assert.NoError(s.T(), err)

	frTrans := &LanguageTranslations{
		Language: "fr-FR",
		Translations: map[string]map[string]string{
			"ns1": {
				"key1": "value1-fr",
			},
		},
	}
	err = s.store.Create("fr-FR", frTrans)
	assert.NoError(s.T(), err)

	// Get translation for key1 in ns1
	translations, err := s.store.GetTranslationsByKey("key1", "ns1")
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), translations)

	assert.Contains(s.T(), translations, "en-US")
	assert.Equal(s.T(), "value1-en", translations["en-US"].Value)

	assert.Contains(s.T(), translations, "fr-FR")
	assert.Equal(s.T(), "value1-fr", translations["fr-FR"].Value)
}

func (s *FileBasedStoreTestSuite) TestGetTranslationsByKey_NotFound() {
	// Setup data
	enTrans := &LanguageTranslations{
		Language: "en-US",
		Translations: map[string]map[string]string{
			"ns1": {
				"key1": "value1-en",
			},
		},
	}
	err := s.store.Create("en-US", enTrans)
	assert.NoError(s.T(), err)

	// Wrong key
	translations, err := s.store.GetTranslationsByKey("key2", "ns1")
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), translations)
	assert.Empty(s.T(), translations)

	// Wrong namespace
	translations, err = s.store.GetTranslationsByKey("key1", "ns2")
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), translations)
	assert.Empty(s.T(), translations)
}

func (s *FileBasedStoreTestSuite) TestIsTranslationExists() {
	enTrans := &LanguageTranslations{
		Language: "en-US",
		Translations: map[string]map[string]string{
			"common": {"ok": "OK"},
		},
	}
	err := s.store.Create("en-US", enTrans)
	assert.NoError(s.T(), err)

	exists, err := s.store.IsTranslationExists("en-US")
	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)

	exists, err = s.store.IsTranslationExists("fr-FR")
	assert.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *FileBasedStoreTestSuite) TestUpsertTranslationsByLanguage_NotSupported() {
	err := s.store.UpsertTranslationsByLanguage("en-US", []Translation{})
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not supported")
}

func (s *FileBasedStoreTestSuite) TestUpsertTranslation_NotSupported() {
	err := s.store.UpsertTranslation(Translation{})
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not supported")
}

func (s *FileBasedStoreTestSuite) TestDeleteTranslationsByLanguage_NotSupported() {
	err := s.store.DeleteTranslationsByLanguage("en-US")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not supported")
}

func (s *FileBasedStoreTestSuite) TestDeleteTranslation_NotSupported() {
	err := s.store.DeleteTranslation("en-US", "key", "ns")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not supported")
}

func (s *FileBasedStoreTestSuite) TestDeleteTranslationsByNamespace_NotSupported() {
	err := s.store.DeleteTranslationsByNamespace(context.Background(), "app.test-id")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not supported")
}

func (s *FileBasedStoreTestSuite) TestDeleteTranslationsByKey_NotSupported() {
	err := s.store.DeleteTranslationsByKey(context.Background(), "custom", "app.test-id.name")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not supported")
}

func (s *FileBasedStoreTestSuite) TestIsTranslationDeclarative() {
	enTrans := &LanguageTranslations{
		Language: "en-US",
		Translations: map[string]map[string]string{
			"common": {"ok": "OK"},
		},
	}
	err := s.store.Create("en-US", enTrans)
	assert.NoError(s.T(), err)

	// Checks if "en-US" is in the store
	isDeclarative := s.store.IsTranslationDeclarative("en-US")
	assert.True(s.T(), isDeclarative)

	isDeclarative = s.store.IsTranslationDeclarative("fr-FR")
	assert.False(s.T(), isDeclarative)
}

func (s *FileBasedStoreTestSuite) TestNewFileBasedStore() {
	store := newFileBasedStore()
	assert.NotNil(s.T(), store)

	fbStore, ok := store.(*fileBasedStore)
	assert.True(s.T(), ok)
	assert.NotNil(s.T(), fbStore.GenericFileBasedStore)
}
