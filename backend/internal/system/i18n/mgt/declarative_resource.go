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
	"fmt"

	"gopkg.in/yaml.v3"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	paramTypeTranslation    = "Translation"
	resourceTypeTranslation = "translation"
)

// translationExporter implements declarativeresource.ResourceExporter for translations.
type translationExporter struct {
	store i18nStoreInterface
}

// newTranslationExporter creates a new Translation exporter.
func newTranslationExporter(store i18nStoreInterface) *translationExporter {
	return &translationExporter{store: store}
}

// GetResourceType returns the resource type for translations.
func (e *translationExporter) GetResourceType() string {
	return resourceTypeTranslation
}

// GetParameterizerType returns the parameterizer type for translations.
func (e *translationExporter) GetParameterizerType() string {
	return paramTypeTranslation
}

// GetAllResourceIDs retrieves all languages from the database store.
// One file per language.
func (e *translationExporter) GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError) {
	// Get all translations from store
	languages, err := e.store.GetDistinctLanguages()
	if err != nil {
		return nil, &serviceerror.ServiceError{
			Code: "I18N_EXPORT_ERROR",
			Type: serviceerror.ServerErrorType,
			Error: core.I18nMessage{
				Key:          "error.i18nservice.export_error",
				DefaultValue: "Failed to fetch translation languages for export",
			},
		}
	}
	return languages, nil
}

// GetResourceByID retrieves all translations for a specific language.
func (e *translationExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *serviceerror.ServiceError,
) {
	translations, err := e.store.GetTranslations()
	if err != nil {
		return nil, "", &serviceerror.ServiceError{
			Code: "I18N_FETCH_ERROR",
			Type: serviceerror.ServerErrorType,
			Error: core.I18nMessage{
				Key:          "error.i18nservice.fetch_error",
				DefaultValue: "Failed to fetch translations for export",
			},
		}
	}

	result := make(map[string]map[string]string)

	for _, translationsByLang := range translations {
		for _, translation := range translationsByLang {
			if translation.Language == id {
				if result[translation.Namespace] == nil {
					result[translation.Namespace] = make(map[string]string)
				}
				result[translation.Namespace][translation.Key] = translation.Value
			}
		}
	}

	if len(result) == 0 {
		return nil, "", &serviceerror.ServiceError{
			Code: "TRANSLATION_NOT_FOUND",
			Type: serviceerror.ClientErrorType,
			Error: core.I18nMessage{
				Key:          "error.i18nservice.translation_not_found",
				DefaultValue: fmt.Sprintf("Translation not found for %s", id),
			},
		}
	}

	return &LanguageTranslations{
		Language:     id,
		Translations: result,
	}, id, nil
}

// ValidateResource validates a translation resource.
func (e *translationExporter) ValidateResource(
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	trans, ok := resource.(*LanguageTranslations)
	if !ok {
		return "", declarativeresource.CreateTypeError(e.GetResourceType(), id)
	}

	if trans.Language == "" || trans.Translations == nil {
		return "", &declarativeresource.ExportError{
			ResourceType: e.GetResourceType(),
			ResourceID:   id,
			Code:         "INVALID_TRANSLATION",
			Error:        "Translation missing required fields",
		}
	}

	return id, nil
}

// GetResourceRules returns the parameterization rules for translations.
func (e *translationExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{
		Variables:      []string{},
		ArrayVariables: []string{},
	}
}

// loadDeclarativeResources loads immutable translation resources from files.
func loadDeclarativeResources(fileStore i18nStoreInterface) error {
	// Type assert to get the file-based store for resource loading
	store, ok := fileStore.(*fileBasedStore)
	if !ok {
		return fmt.Errorf("fileStore must be a file-based store implementation")
	}

	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "Translation",
		DirectoryName: "translations",
		Parser:        parseToTranslationWrapper,
		Validator: func(data interface{}) error {
			return validateTranslationWrapper(data, store)
		},
		IDExtractor: func(data interface{}) string {
			trans := data.(*LanguageTranslations)
			return trans.Language
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, store)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load translation resources: %w", err)
	}

	return nil
}

// parseToTranslationWrapper wraps parseToTranslation to match the expected signature.
func parseToTranslationWrapper(data []byte) (interface{}, error) {
	return parseToLangTranslation(data)
}

// parseToLangTranslation parses YAML data to Translation.
func parseToLangTranslation(data []byte) (*LanguageTranslations, error) {
	var transRequest struct {
		Language     string                       `yaml:"language"`
		Translations map[string]map[string]string `yaml:"translations"`
	}

	err := yaml.Unmarshal(data, &transRequest)
	if err != nil {
		return nil, err
	}

	trans := &LanguageTranslations{
		Language:     transRequest.Language,
		Translations: transRequest.Translations,
	}

	return trans, nil
}

// validateTranslationWrapper wraps validation logic.
func validateTranslationWrapper(data interface{}, fileStore *fileBasedStore) error {
	trans, ok := data.(*LanguageTranslations)
	if !ok {
		return fmt.Errorf("invalid type: expected *LanguageTranslations")
	}

	if trans.Language == "" {
		return fmt.Errorf("translation language is required")
	}
	if trans.Translations == nil {
		return fmt.Errorf("translation translations is required")
	}

	id := trans.Language

	// Check for duplicate ID in the file store
	if existingData, err := fileStore.GenericFileBasedStore.Get(id); err == nil && existingData != nil {
		return fmt.Errorf("duplicate translation ID '%s': "+
			"a translation with this ID already exists in declarative resources", id)
	}

	return nil
}
