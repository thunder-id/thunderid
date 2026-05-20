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

// Package mgt provides internationalization functionality.
package mgt

import (
	"context"
	"slices"
	"strings"

	goi18n "golang.org/x/text/language"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	sysi18n "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const loggerComponentName = "I18nMgtService"

// I18nServiceInterface defines the interface for the i18n service.
type I18nServiceInterface interface {
	ListLanguages() ([]string, *serviceerror.ServiceError)
	ResolveTranslations(language string, namespace string) (
		*LanguageTranslationsResponse, *serviceerror.ServiceError)
	SetTranslationOverrides(language string, translations map[string]map[string]string) (
		*LanguageTranslationsResponse, *serviceerror.ServiceError)
	ClearTranslationOverrides(language string) *serviceerror.ServiceError
	ResolveTranslationsForKey(language string, namespace string, key string) (
		*TranslationResponse, *serviceerror.ServiceError)
	SetTranslationOverrideForKey(language string, namespace string, key string, value string) (
		*TranslationResponse, *serviceerror.ServiceError)
	SetTranslationOverridesForNamespace(ctx context.Context, namespace string,
		entries map[string]map[string]string) *serviceerror.ServiceError
	ClearTranslationOverrideForKey(language string, namespace string, key string) *serviceerror.ServiceError
	DeleteTranslationsByNamespace(ctx context.Context, namespace string) *serviceerror.ServiceError
	DeleteTranslationsByKey(ctx context.Context, namespace string, key string) *serviceerror.ServiceError
	// GetTranslationsByNamespace returns all raw translations for a namespace as
	// map[key]map[language]value without locale resolution or best-match logic.
	GetTranslationsByNamespace(namespace string) (map[string]map[string]string, *serviceerror.ServiceError)
}

// i18nService is the default implementation of I18nServiceInterface.
type i18nService struct {
	store  i18nStoreInterface
	logger *log.Logger
}

// newI18nService creates a new instance of i18nService with injected dependencies.
func newI18nService(store i18nStoreInterface) I18nServiceInterface {
	return &i18nService{
		store:  store,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// ListLanguages retrieves all locale codes that have translations in the system.
// The default locale is always included in the response, even if it has no translations in the DB.
func (s *i18nService) ListLanguages() ([]string, *serviceerror.ServiceError) {
	localeCodes, err := s.store.GetDistinctLanguages()
	if err != nil {
		s.logger.Error("Failed to get locales from store", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// Ensure default language is always in the list
	hasDefaultLanguage := false
	for _, code := range localeCodes {
		if code == SystemLanguage {
			hasDefaultLanguage = true
			break
		}
	}

	if !hasDefaultLanguage {
		localeCodes = append(localeCodes, SystemLanguage)
	}

	return localeCodes, nil
}

// ResolveTranslationsForKey resolves a single translation by language, namespace, and key.
// It merges custom overrides with default values.
func (s *i18nService) ResolveTranslationsForKey(
	language string, namespace string, key string) (*TranslationResponse, *serviceerror.ServiceError) {
	if err := validate(language, namespace, key); err != nil {
		return nil, err
	}

	trans, err := s.store.GetTranslationsByKey(key, namespace)
	if err != nil {
		s.logger.Error("Failed to get translation from store", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	if _, exists := trans[SystemLanguage]; !exists {
		if namespace == SystemNamespace {
			// If no custom override for system language, use default translation for system language
			defaultValue, exists := sysi18n.GetDefault(key)
			if exists {
				trans[SystemLanguage] = Translation{
					Key:       key,
					Language:  SystemLanguage,
					Namespace: SystemNamespace,
					Value:     defaultValue,
				}
			}
		}
	}

	requestedLang := goi18n.Make(language)

	bestTranslation := selectBestTranslation(trans, requestedLang)

	if bestTranslation.Value != "" {
		return &TranslationResponse{
			Language:  language,
			Namespace: bestTranslation.Namespace,
			Key:       bestTranslation.Key,
			Value:     bestTranslation.Value,
		}, nil
	}

	return nil, &ErrorTranslationNotFound
}

// SetTranslationOverrideForKey creates or updates a custom override for a single translation.
func (s *i18nService) SetTranslationOverrideForKey(
	language string, namespace string, key string, value string) (
	*TranslationResponse, *serviceerror.ServiceError) {
	if err := declarativeresource.CheckDeclarativeUpdate(); err != nil {
		return nil, err
	}
	if err := validate(language, namespace, key); err != nil {
		return nil, err
	}
	if value == "" {
		return nil, &ErrorMissingValue
	}

	trans := Translation{
		Key:       key,
		Language:  language,
		Namespace: namespace,
		Value:     value,
	}

	// Use upsert to create or update
	if err := s.store.UpsertTranslation(trans); err != nil {
		s.logger.Error("Failed to set translation override", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return &TranslationResponse{
		Language:  language,
		Namespace: namespace,
		Key:       key,
		Value:     value,
	}, nil
}

// SetTranslationOverridesForNamespace creates or updates all provided key/language/value entries
// for a single namespace. When ctx carries an outer configDB transaction the writes join it.
// entries is map[key]map[language]value.
func (s *i18nService) SetTranslationOverridesForNamespace(
	ctx context.Context, namespace string, entries map[string]map[string]string) *serviceerror.ServiceError {
	if err := declarativeresource.CheckDeclarativeUpdate(); err != nil {
		return err
	}
	if !ValidateNamespace(namespace) {
		return &ErrorInvalidNamespace
	}
	translations := make([]Translation, 0)
	for key, langMap := range entries {
		if !ValidateKey(key) {
			return &ErrorInvalidKey
		}
		for language, value := range langMap {
			if language == "" {
				return &ErrorMissingLanguage
			}
			if !ValidateLanguage(language) {
				return &ErrorInvalidLanguage
			}
			if value == "" {
				return &ErrorMissingValue
			}
			translations = append(translations, Translation{
				Key:       key,
				Language:  language,
				Namespace: namespace,
				Value:     value,
			})
		}
	}
	if len(translations) == 0 {
		return nil
	}
	if err := s.store.UpsertTranslations(ctx, translations); err != nil {
		s.logger.Error("Failed to set translation overrides for namespace", log.Error(err))
		return &serviceerror.InternalServerError
	}
	return nil
}

// ClearTranslationOverrideForKey removes the custom override for a single translation.
func (s *i18nService) ClearTranslationOverrideForKey(
	language string, namespace string, key string) *serviceerror.ServiceError {
	if err := declarativeresource.CheckDeclarativeDelete(); err != nil {
		return err
	}
	if err := validate(language, namespace, key); err != nil {
		return err
	}

	if err := s.store.DeleteTranslation(language, key, namespace); err != nil {
		s.logger.Error("Failed to clear translation override", log.Error(err))
		return &serviceerror.InternalServerError
	}

	return nil
}

// ResolveTranslations resolves all translations for a language, organized by namespace.
// Merges custom overrides with default values.
func (s *i18nService) ResolveTranslations(
	language string, namespace string) (*LanguageTranslationsResponse, *serviceerror.ServiceError) {
	if language == "" {
		language = SystemLanguage
	}
	if !ValidateLanguage(language) {
		return nil, &ErrorInvalidLanguage
	}

	// If namespace is provided, validate it and filter by it
	if namespace != "" && !ValidateNamespace(namespace) {
		return nil, &ErrorInvalidNamespace
	}

	requestedLang := goi18n.Make(language)

	var allTranslations map[string]map[string]Translation
	var err error

	if namespace == "" {
		// Get all namespaces
		allTranslations, err = s.store.GetTranslations()
		if err != nil {
			s.logger.Error("Failed to get translations from store", log.Error(err))
			return nil, &serviceerror.InternalServerError
		}
	} else {
		allTranslations, err = s.store.GetTranslationsByNamespace(namespace)
		if err != nil {
			s.logger.Error("Failed to get translations from store", log.Error(err))
			return nil, &serviceerror.InternalServerError
		}
	}

	if namespace == "" || namespace == SystemNamespace {
		// Get system default translations
		systemDefaults := sysi18n.GetAllDefaults()

		for key, value := range systemDefaults {
			compositeKey := SystemNamespace + "|" + key
			if allTranslations[compositeKey] == nil {
				allTranslations[compositeKey] = make(map[string]Translation)
			}
			if _, exists := allTranslations[compositeKey][SystemLanguage]; !exists {
				allTranslations[compositeKey][SystemLanguage] = Translation{
					Key:       key,
					Language:  SystemLanguage,
					Namespace: SystemNamespace,
					Value:     value,
				}
			}
		}
	}

	result := make(map[string]map[string]string)
	for _, translations := range allTranslations {
		translation := selectBestTranslation(translations, requestedLang)

		if translation.Value == "" {
			continue
		}
		if result[translation.Namespace] == nil {
			result[translation.Namespace] = make(map[string]string)
		}
		result[translation.Namespace][translation.Key] = translation.Value
	}

	return &LanguageTranslationsResponse{
		Language:     language,
		TotalResults: len(allTranslations),
		Translations: result,
	}, nil
}

// SetTranslationOverrides replaces all custom overrides for a language with provided values.
func (s *i18nService) SetTranslationOverrides(
	language string, translations map[string]map[string]string) (
	*LanguageTranslationsResponse, *serviceerror.ServiceError) {
	if err := declarativeresource.CheckDeclarativeUpdate(); err != nil {
		return nil, err
	}
	if language == "" {
		return nil, &ErrorMissingLanguage
	}
	if !ValidateLanguage(language) {
		return nil, &ErrorInvalidLanguage
	}
	if len(translations) == 0 {
		return nil, &ErrorEmptyTranslations
	}

	// Validate all entries first
	for ns, keys := range translations {
		if !ValidateNamespace(ns) {
			return nil, &ErrorInvalidNamespace
		}
		for key, value := range keys {
			if !ValidateKey(key) {
				return nil, &ErrorInvalidKey
			}
			if value == "" {
				return nil, &ErrorMissingValue
			}
		}
	}

	flattenedTranslations := []Translation{}
	for ns, keys := range translations {
		for key, value := range keys {
			flattenedTranslations = append(flattenedTranslations, Translation{
				Key:       key,
				Language:  language,
				Namespace: ns,
				Value:     value,
			})
		}
	}

	if err := s.store.UpsertTranslationsByLanguage(language, flattenedTranslations); err != nil {
		s.logger.Error("Failed to upsert translations", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// TODO: return actual stored translations from DB
	return &LanguageTranslationsResponse{
		Language:     language,
		TotalResults: len(flattenedTranslations),
		Translations: translations,
	}, nil
}

// ClearTranslationOverrides removes all custom overrides for a language.
func (s *i18nService) ClearTranslationOverrides(language string) *serviceerror.ServiceError {
	if err := declarativeresource.CheckDeclarativeDelete(); err != nil {
		return err
	}
	if language == "" {
		return &ErrorMissingLanguage
	}
	if !ValidateLanguage(language) {
		return &ErrorInvalidLanguage
	}

	if err := s.clearAllOverrides(language); err != nil {
		s.logger.Error("Failed to clear overrides", log.Error(err))
		return &serviceerror.InternalServerError
	}

	return nil
}

// DeleteTranslationsByKey removes all translations for a specific namespace+key pair.
func (s *i18nService) DeleteTranslationsByKey(
	ctx context.Context, namespace string, key string) *serviceerror.ServiceError {
	if !ValidateNamespace(namespace) {
		return &ErrorInvalidNamespace
	}
	if !ValidateKey(key) {
		return &ErrorInvalidKey
	}
	if err := s.store.DeleteTranslationsByKey(ctx, namespace, key); err != nil {
		s.logger.Error("Failed to delete translations by namespace and key", log.Error(err))
		return &serviceerror.InternalServerError
	}
	return nil
}

// DeleteTranslationsByNamespace removes all translations under the given namespace.
// When ctx carries an outer configDB transaction the delete joins it.
func (s *i18nService) DeleteTranslationsByNamespace(
	ctx context.Context, namespace string) *serviceerror.ServiceError {
	if !ValidateNamespace(namespace) {
		return &ErrorInvalidNamespace
	}
	if err := s.store.DeleteTranslationsByNamespace(ctx, namespace); err != nil {
		s.logger.Error("Failed to delete translations by namespace", log.Error(err))
		return &serviceerror.InternalServerError
	}
	return nil
}

// GetTranslationsByNamespace returns all raw translations for a namespace as
// map[key]map[language]value without locale resolution. Used to load all locale
// variants for a resource in a single query.
func (s *i18nService) GetTranslationsByNamespace(
	namespace string) (map[string]map[string]string, *serviceerror.ServiceError) {
	if !ValidateNamespace(namespace) {
		return nil, &ErrorInvalidNamespace
	}
	byNs, err := s.store.GetTranslationsByNamespace(namespace)
	if err != nil {
		s.logger.Error("Failed to get translations by namespace", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	result := make(map[string]map[string]string, len(byNs))
	for compositeKey, langs := range byNs {
		// compositeKey is "namespace|key" — drop the namespace prefix.
		idx := strings.Index(compositeKey, "|")
		if idx < 0 {
			continue
		}
		fieldKey := compositeKey[idx+1:]
		for lang, trans := range langs {
			if result[fieldKey] == nil {
				result[fieldKey] = make(map[string]string)
			}
			result[fieldKey][lang] = trans.Value
		}
	}
	return result, nil
}

func (s *i18nService) clearAllOverrides(language string) error {
	err := s.store.DeleteTranslationsByLanguage(language)
	if err != nil {
		return err
	}
	return nil
}

func validate(language string, namespace string, key string) *serviceerror.ServiceError {
	if language == "" {
		return &ErrorMissingLanguage
	}
	if !ValidateLanguage(language) {
		return &ErrorInvalidLanguage
	}
	if !ValidateNamespace(namespace) {
		return &ErrorInvalidNamespace
	}
	if !ValidateKey(key) {
		return &ErrorInvalidKey
	}
	return nil
}

func selectBestTranslation(availableTranslations map[string]Translation, requestedLang goi18n.Tag) Translation {
	if len(availableTranslations) == 0 {
		return Translation{}
	}

	availableLangTags := make([]string, 0, len(availableTranslations))
	for langTag := range availableTranslations {
		availableLangTags = append(availableLangTags, langTag)
	}
	slices.SortFunc(availableLangTags, compareLangs)

	availableLangs := make([]goi18n.Tag, 0, len(availableLangTags))

	for _, langTag := range availableLangTags {
		availableLangs = append(availableLangs, goi18n.BCP47.Make(langTag))
	}

	matcher := goi18n.NewMatcher(availableLangs)
	_, index, _ := matcher.Match(requestedLang)

	return availableTranslations[availableLangTags[index]]
}

func compareLangs(langA, langB string) int {
	langAPref, aExists := LanguagePreferenceOrder[langA]
	langBPref, bExists := LanguagePreferenceOrder[langB]

	if aExists && bExists {
		return langAPref - langBPref
	}
	if aExists {
		return -1
	}
	if bExists {
		return 1
	}
	return 0
}
