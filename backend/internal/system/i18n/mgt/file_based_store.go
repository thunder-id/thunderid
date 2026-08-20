// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mgt

import (
	"context"
	"errors"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

type fileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// newFileBasedStore creates a new instance of a file-based store.
func newFileBasedStore() i18nStoreInterface {
	genericStore := declarativeresource.NewGenericFileBasedStore(entity.KeyTypeTranslation)
	return &fileBasedStore{
		GenericFileBasedStore: genericStore,
	}
}

// Create implements declarativeresource.Storer interface for resource loader
func (f *fileBasedStore) Create(id string, data interface{}) error {
	trans := data.(*LanguageTranslations)
	return f.GenericFileBasedStore.Create(id, trans)
}

// GetDistinctLanguages retrieves all distinct language codes that have translations.
func (f *fileBasedStore) GetDistinctLanguages(ctx context.Context) ([]string, error) {
	list, err := f.GenericFileBasedStore.List(ctx)
	if err != nil {
		return nil, err
	}

	languages := make([]string, 0, len(list))
	for _, item := range list {
		if langTrans, ok := item.Data.(*LanguageTranslations); ok {
			languages = append(languages, langTrans.Language)
		}
	}
	return languages, nil
}

// GetTranslations retrieves all translations.
func (f *fileBasedStore) GetTranslations(ctx context.Context) (map[string]map[string]Translation, error) {
	list, err := f.GenericFileBasedStore.List(ctx)
	if err != nil {
		return nil, err
	}

	translations := make(map[string]map[string]Translation)
	for _, item := range list {
		if langTrans, ok := item.Data.(*LanguageTranslations); ok {
			for ns, nsTrans := range langTrans.Translations {
				for key, value := range nsTrans {
					compositeKey := ns + "|" + key
					if translations[compositeKey] == nil {
						translations[compositeKey] = make(map[string]Translation)
					}
					translations[compositeKey][langTrans.Language] = Translation{
						Key:       key,
						Language:  langTrans.Language,
						Namespace: ns,
						Value:     value,
					}
				}
			}
		}
	}
	return translations, nil
}

// GetTranslationsByNamespace retrieves all translations for a namespace.
func (f *fileBasedStore) GetTranslationsByNamespace(ctx context.Context,
	namespace string) (map[string]map[string]Translation, error) {
	list, err := f.GenericFileBasedStore.List(ctx)
	if err != nil {
		return nil, err
	}

	translations := make(map[string]map[string]Translation)
	for _, item := range list {
		if langTrans, ok := item.Data.(*LanguageTranslations); ok {
			for ns, nsTrans := range langTrans.Translations {
				if ns != namespace {
					continue
				}
				for k, v := range nsTrans {
					compositeKey := ns + "|" + k
					if translations[compositeKey] == nil {
						translations[compositeKey] = make(map[string]Translation)
					}
					translations[compositeKey][langTrans.Language] = Translation{
						Key:       k,
						Language:  langTrans.Language,
						Namespace: ns,
						Value:     v,
					}
				}
			}
		}
	}
	return translations, nil
}

// GetTranslationsByKey retrieves a single translation by key and namespace.
func (f *fileBasedStore) GetTranslationsByKey(ctx context.Context, key string,
	namespace string) (map[string]Translation, error) {
	list, err := f.GenericFileBasedStore.List(ctx)
	if err != nil {
		return nil, err
	}

	translations := make(map[string]Translation)
	for _, item := range list {
		if langTrans, ok := item.Data.(*LanguageTranslations); ok {
			for ns, nsTrans := range langTrans.Translations {
				if ns != namespace {
					continue
				}
				for k, v := range nsTrans {
					if k != key {
						continue
					}
					translations[langTrans.Language] = Translation{
						Key:       k,
						Language:  langTrans.Language,
						Namespace: ns,
						Value:     v,
					}
				}
			}
		}
	}

	return translations, nil
}

// UpsertTranslationsByLanguage is not supported in file-based store.
func (f *fileBasedStore) UpsertTranslationsByLanguage(language string, translations []Translation) error {
	return errors.New("UpsertTranslationsByLanguage is not supported in file-based store")
}

// UpsertTranslation is not supported in file-based store.
func (f *fileBasedStore) UpsertTranslation(trans Translation) error {
	return errors.New("UpsertTranslation is not supported in file-based store")
}

// UpsertTranslations is not supported in file-based store.
func (f *fileBasedStore) UpsertTranslations(ctx context.Context, _ []Translation) error {
	return errors.New("UpsertTranslations is not supported in file-based store")
}

// DeleteTranslationsByLanguage is not supported in file-based store.
func (f *fileBasedStore) DeleteTranslationsByLanguage(language string) error {
	return errors.New("DeleteTranslationsByLanguage is not supported in file-based store")
}

// DeleteTranslation is not supported in file-based store.
func (f *fileBasedStore) DeleteTranslation(language string, key string, namespace string) error {
	return errors.New("DeleteTranslation is not supported in file-based store")
}

// DeleteTranslationsByNamespace is not supported in file-based store.
func (f *fileBasedStore) DeleteTranslationsByNamespace(ctx context.Context, _ string) error {
	return errors.New("DeleteTranslationsByNamespace is not supported in file-based store")
}

// DeleteTranslationsByKey is not supported in file-based store.
func (f *fileBasedStore) DeleteTranslationsByKey(ctx context.Context, namespace string, key string) error {
	return errors.New("DeleteTranslationsByKey is not supported in file-based store")
}

// IsTranslationDeclarative checks if a translation is immutable (exists in file store).
// Helper method for composite store.
func (f *fileBasedStore) IsTranslationDeclarative(ctx context.Context, id string) bool {
	item, err := f.GenericFileBasedStore.Get(ctx, id)
	return err == nil && item != nil
}

// IsTranslationExists checks if a translation exists.
func (f *fileBasedStore) IsTranslationExists(ctx context.Context, id string) (bool, error) {
	item, err := f.GenericFileBasedStore.Get(ctx, id)
	if err != nil {
		return false, nil // Treat get error as not found
	}
	return item != nil, nil
}
