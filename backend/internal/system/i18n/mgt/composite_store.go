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

package mgt

import (
	"context"
)

// compositeI18nStore combines a file-backed (declarative) store and a database-backed
// (mutable) store for translations.
//   - Read operations merge both stores; database values (overrides) take precedence over
//     file-based values (base translations) for the same key and language.
//   - Write operations target the database store only.
type compositeI18nStore struct {
	fileStore i18nStoreInterface
	dbStore   i18nStoreInterface
}

func newCompositeI18nStore(fileStore, dbStore i18nStoreInterface) i18nStoreInterface {
	return &compositeI18nStore{fileStore: fileStore, dbStore: dbStore}
}

// GetDistinctLanguages returns the union of languages from both stores.
func (c *compositeI18nStore) GetDistinctLanguages() ([]string, error) {
	dbLangs, err := c.dbStore.GetDistinctLanguages()
	if err != nil {
		return nil, err
	}
	fileLangs, err := c.fileStore.GetDistinctLanguages()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(dbLangs)+len(fileLangs))
	result := make([]string, 0, len(dbLangs)+len(fileLangs))
	for _, l := range dbLangs {
		if !seen[l] {
			seen[l] = true
			result = append(result, l)
		}
	}
	for _, l := range fileLangs {
		if !seen[l] {
			seen[l] = true
			result = append(result, l)
		}
	}
	return result, nil
}

// GetTranslations merges translations from both stores.
// DB values (overrides) take precedence over file-based values for the same key+language.
func (c *compositeI18nStore) GetTranslations() (map[string]map[string]Translation, error) {
	fileTrans, err := c.fileStore.GetTranslations()
	if err != nil {
		return nil, err
	}
	dbTrans, err := c.dbStore.GetTranslations()
	if err != nil {
		return nil, err
	}
	return mergeTranslationMaps(fileTrans, dbTrans), nil
}

// GetTranslationsByNamespace merges translations for a namespace from both stores.
// DB values take precedence over file-based values for the same key+language.
func (c *compositeI18nStore) GetTranslationsByNamespace(namespace string) (
	map[string]map[string]Translation, error,
) {
	fileTrans, err := c.fileStore.GetTranslationsByNamespace(namespace)
	if err != nil {
		return nil, err
	}
	dbTrans, err := c.dbStore.GetTranslationsByNamespace(namespace)
	if err != nil {
		return nil, err
	}
	return mergeTranslationMaps(fileTrans, dbTrans), nil
}

// GetTranslationsByKey retrieves a translation for a specific key and namespace.
// Returns the DB override if present, otherwise falls back to the file-based value.
func (c *compositeI18nStore) GetTranslationsByKey(key string, namespace string) (
	map[string]Translation, error,
) {
	fileTrans, err := c.fileStore.GetTranslationsByKey(key, namespace)
	if err != nil {
		return nil, err
	}
	dbTrans, err := c.dbStore.GetTranslationsByKey(key, namespace)
	if err != nil {
		return nil, err
	}

	// Start with file-based values, then let DB override per language.
	result := make(map[string]Translation, len(fileTrans)+len(dbTrans))
	for lang, t := range fileTrans {
		result[lang] = t
	}
	for lang, t := range dbTrans {
		result[lang] = t
	}
	return result, nil
}

func (c *compositeI18nStore) UpsertTranslationsByLanguage(language string, translations []Translation) error {
	return c.dbStore.UpsertTranslationsByLanguage(language, translations)
}

func (c *compositeI18nStore) UpsertTranslation(trans Translation) error {
	return c.dbStore.UpsertTranslation(trans)
}

func (c *compositeI18nStore) UpsertTranslations(ctx context.Context, translations []Translation) error {
	return c.dbStore.UpsertTranslations(ctx, translations)
}

func (c *compositeI18nStore) DeleteTranslationsByLanguage(language string) error {
	return c.dbStore.DeleteTranslationsByLanguage(language)
}

func (c *compositeI18nStore) DeleteTranslation(language string, key string, namespace string) error {
	return c.dbStore.DeleteTranslation(language, key, namespace)
}

func (c *compositeI18nStore) DeleteTranslationsByNamespace(ctx context.Context, namespace string) error {
	return c.dbStore.DeleteTranslationsByNamespace(ctx, namespace)
}

func (c *compositeI18nStore) DeleteTranslationsByKey(ctx context.Context, namespace string, key string) error {
	return c.dbStore.DeleteTranslationsByKey(ctx, namespace, key)
}

// mergeTranslationMaps merges base (file) and override (DB) translation maps.
// For each compositeKey (namespace|key), DB values take precedence over file values
// per language, so a DB override for a specific language replaces the file-based value.
func mergeTranslationMaps(
	base, overrides map[string]map[string]Translation,
) map[string]map[string]Translation {
	result := make(map[string]map[string]Translation, len(base)+len(overrides))

	for compositeKey, langMap := range base {
		result[compositeKey] = make(map[string]Translation, len(langMap))
		for lang, t := range langMap {
			result[compositeKey][lang] = t
		}
	}

	for compositeKey, langMap := range overrides {
		if result[compositeKey] == nil {
			result[compositeKey] = make(map[string]Translation, len(langMap))
		}
		for lang, t := range langMap {
			result[compositeKey][lang] = t
		}
	}

	return result
}
