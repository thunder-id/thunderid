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
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// i18nStoreInterface defines the interface for i18n store operations.
type i18nStoreInterface interface {
	GetDistinctLanguages() ([]string, error)
	GetTranslations() (map[string]map[string]Translation, error)
	GetTranslationsByNamespace(namespace string) (map[string]map[string]Translation, error)
	GetTranslationsByKey(key string, namespace string) (map[string]Translation, error)
	UpsertTranslationsByLanguage(language string, translations []Translation) error
	UpsertTranslation(trans Translation) error
	UpsertTranslations(ctx context.Context, translations []Translation) error
	DeleteTranslationsByLanguage(language string) error
	DeleteTranslation(language string, key string, namespace string) error
	DeleteTranslationsByNamespace(ctx context.Context, namespace string) error
	DeleteTranslationsByKey(ctx context.Context, namespace string, key string) error
}

// i18nStore is the default implementation of i18nStoreInterface.
type i18nStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newI18nStore creates a new instance of i18nStore.
func newI18nStore() i18nStoreInterface {
	return &i18nStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// getDBClient is a helper method to get the database client.
func (s *i18nStore) getDBClient() (provider.DBClientInterface, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	return dbClient, nil
}

// GetDistinctLanguages retrieves all distinct language codes that have translations.
func (s *i18nStore) GetDistinctLanguages() ([]string, error) {
	dbClient, err := s.getDBClient()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.Query(queryGetDistinctLanguages, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get distinct languages: %w", err)
	}

	languages := make([]string, 0, len(results))
	for _, row := range results {
		lang, ok := row["language_code"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse language_code")
		}
		languages = append(languages, lang)
	}
	return languages, nil
}

// GetTranslations retrieves all translations.
// This implements TranslationStoreInterface.
func (s *i18nStore) GetTranslations() (map[string]map[string]Translation, error) {
	dbClient, err := s.getDBClient()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.Query(queryGetTranslations, s.deploymentID)

	if err != nil {
		return nil, fmt.Errorf("failed to get translations: %w", err)
	}

	return transformResults(results)
}

// GetTranslations retrieves all translations of the given namespace.
// This implements TranslationStoreInterface.
func (s *i18nStore) GetTranslationsByNamespace(namespace string) (map[string]map[string]Translation, error) {
	dbClient, err := s.getDBClient()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.Query(queryGetTranslationsByNamespace, namespace, s.deploymentID)

	if err != nil {
		return nil, fmt.Errorf("failed to get translations: %w", err)
	}

	return transformResults(results)
}

// GetTranslationsByKey retrieves a single translation by key, and namespace.
func (s *i18nStore) GetTranslationsByKey(key string, namespace string) (map[string]Translation, error) {
	dbClient, err := s.getDBClient()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.Query(queryGetTranslation, key, namespace, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get translation: %w", err)
	}

	return buildTranslationsMap(results)
}

// InsertTranslation inserts a new translation.
func (s *i18nStore) UpsertTranslationsByLanguage(language string, translations []Translation) error {
	dbClient, err := s.getDBClient()
	if err != nil {
		return err
	}

	tx, err := dbClient.BeginTx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	_, err = tx.Exec(queryDeleteTranslationsByLanguage, language, s.deploymentID)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to rollback transaction: %w", rollbackErr))
		}
		return fmt.Errorf("failed to delete translations: %w", err)
	}

	for _, trans := range translations {
		_, err = tx.Exec(queryInsertTranslation, trans.Key, trans.Language, trans.Namespace,
			trans.Value, s.deploymentID)
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				err = errors.Join(err, fmt.Errorf("failed to rollback transaction: %w", rollbackErr))
			}
			return fmt.Errorf("failed to insert translation: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// UpsertTranslation creates or updates a translation.
// Used for bulk operations where we want to insert or update as needed.
func (s *i18nStore) UpsertTranslation(trans Translation) error {
	dbClient, err := s.getDBClient()
	if err != nil {
		return err
	}

	_, err = dbClient.Execute(queryUpsertTranslation, trans.Key, trans.Language, trans.Namespace,
		trans.Value, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to upsert translation: %w", err)
	}
	return nil
}

// UpsertTranslations creates or updates multiple translations.
// When ctx carries an outer configDB transaction the upserts join it atomically.
// Without an outer transaction each upsert runs independently.
func (s *i18nStore) UpsertTranslations(ctx context.Context, translations []Translation) error {
	dbClient, err := s.getDBClient()
	if err != nil {
		return err
	}

	for _, trans := range translations {
		if _, err = dbClient.ExecuteContext(ctx, queryUpsertTranslation, trans.Key, trans.Language,
			trans.Namespace, trans.Value, s.deploymentID); err != nil {
			return fmt.Errorf("failed to upsert translation: %w", err)
		}
	}
	return nil
}

// DeleteTranslation deletes a translation by language, key, and namespace.
func (s *i18nStore) DeleteTranslation(language string, key string, namespace string) error {
	dbClient, err := s.getDBClient()
	if err != nil {
		return err
	}

	_, err = dbClient.Execute(queryDeleteTranslation, language, key, namespace, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to delete translation: %w", err)
	}
	return nil
}

// DeleteTranslationsByLanguage deletes all translations for the given language.
func (s *i18nStore) DeleteTranslationsByLanguage(language string) error {
	dbClient, err := s.getDBClient()
	if err != nil {
		return err
	}

	_, err = dbClient.Execute(queryDeleteTranslationsByLanguage, language, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to delete translation: %w", err)
	}
	return nil
}

// DeleteTranslationsByKey deletes all translations for the given namespace and key.
// When ctx carries an outer configDB transaction the delete joins it atomically.
func (s *i18nStore) DeleteTranslationsByKey(ctx context.Context, namespace string, key string) error {
	dbClient, err := s.getDBClient()
	if err != nil {
		return err
	}

	_, err = dbClient.ExecuteContext(ctx, queryDeleteTranslationsByKey, namespace, key, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to delete translations by namespace and key: %w", err)
	}
	return nil
}

// DeleteTranslationsByNamespace deletes all translations for the given namespace.
// When ctx carries an outer configDB transaction the delete joins it atomically.
func (s *i18nStore) DeleteTranslationsByNamespace(ctx context.Context, namespace string) error {
	dbClient, err := s.getDBClient()
	if err != nil {
		return err
	}

	_, err = dbClient.ExecuteContext(ctx, queryDeleteTranslationsByNamespace, namespace, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to delete translations by namespace: %w", err)
	}
	return nil
}

// buildTranslationFromRow constructs a Translation from a database result row.
func buildTranslationFromRow(row map[string]interface{}) (*Translation, error) {
	key, ok := row["message_key"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse message_key")
	}

	lang, ok := row["language_code"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse language_code")
	}

	namespace, ok := row["namespace"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse namespace")
	}

	value, ok := row["value"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse value")
	}

	return &Translation{
		Key:       key,
		Language:  lang,
		Namespace: namespace,
		Value:     value,
	}, nil
}

func transformResults(results []map[string]interface{}) (map[string]map[string]Translation, error) {
	translations := make(map[string]map[string]Translation)
	for _, row := range results {
		trans, err := buildTranslationFromRow(row)
		if err != nil {
			return nil, err
		}

		compositeKey := trans.Namespace + "|" + trans.Key
		if translations[compositeKey] == nil {
			translations[compositeKey] = make(map[string]Translation)
		}
		translations[compositeKey][trans.Language] = *trans
	}

	return translations, nil
}

func buildTranslationsMap(results []map[string]interface{}) (map[string]Translation, error) {
	translations := make(map[string]Translation)
	for _, row := range results {
		trans, err := buildTranslationFromRow(row)
		if err != nil {
			return nil, err
		}
		translations[trans.Language] = *trans
	}
	return translations, nil
}
