// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mgt

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type CompositeStoreTestSuite struct {
	suite.Suite
	fileStore *i18nStoreInterfaceMock
	dbStore   *i18nStoreInterfaceMock
	store     i18nStoreInterface
}

func TestCompositeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeStoreTestSuite))
}

func (s *CompositeStoreTestSuite) SetupTest() {
	s.fileStore = newI18nStoreInterfaceMock(s.T())
	s.dbStore = newI18nStoreInterfaceMock(s.T())
	s.store = newCompositeI18nStore(s.fileStore, s.dbStore)
}

// GetDistinctLanguages

func (s *CompositeStoreTestSuite) TestGetDistinctLanguages_MergesAndDeduplicates() {
	s.dbStore.EXPECT().GetDistinctLanguages(mock.Anything).Return([]string{"en-US", "fr-FR"}, nil)
	s.fileStore.EXPECT().GetDistinctLanguages(mock.Anything).Return([]string{"en-US", "es-ES"}, nil)

	langs, err := s.store.GetDistinctLanguages(context.Background())
	assert.NoError(s.T(), err)
	assert.Len(s.T(), langs, 3)
	assert.Contains(s.T(), langs, "en-US")
	assert.Contains(s.T(), langs, "fr-FR")
	assert.Contains(s.T(), langs, "es-ES")
}

func (s *CompositeStoreTestSuite) TestGetDistinctLanguages_DBStoreError() {
	s.dbStore.EXPECT().GetDistinctLanguages(mock.Anything).Return(nil, errors.New("db error"))

	langs, err := s.store.GetDistinctLanguages(context.Background())
	assert.Error(s.T(), err)
	assert.Nil(s.T(), langs)
}

func (s *CompositeStoreTestSuite) TestGetDistinctLanguages_FileStoreError() {
	s.dbStore.EXPECT().GetDistinctLanguages(mock.Anything).Return([]string{"en-US"}, nil)
	s.fileStore.EXPECT().GetDistinctLanguages(mock.Anything).Return(nil, errors.New("file error"))

	langs, err := s.store.GetDistinctLanguages(context.Background())
	assert.Error(s.T(), err)
	assert.Nil(s.T(), langs)
}

// GetTranslations

func (s *CompositeStoreTestSuite) TestGetTranslations_DBOverridesFile() {
	fileTrans := map[string]map[string]Translation{
		"ns1|key1": {
			"en-US": {Key: "key1", Namespace: "ns1", Language: "en-US", Value: "file-value"},
		},
	}
	dbTrans := map[string]map[string]Translation{
		"ns1|key1": {
			"en-US": {Key: "key1", Namespace: "ns1", Language: "en-US", Value: "db-override"},
		},
	}
	s.fileStore.EXPECT().GetTranslations(mock.Anything).Return(fileTrans, nil)
	s.dbStore.EXPECT().GetTranslations(mock.Anything).Return(dbTrans, nil)

	result, err := s.store.GetTranslations(context.Background())
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "db-override", result["ns1|key1"]["en-US"].Value)
}

func (s *CompositeStoreTestSuite) TestGetTranslations_FileOnlyKeyPreserved() {
	fileTrans := map[string]map[string]Translation{
		"ns1|key1": {
			"en-US": {Key: "key1", Namespace: "ns1", Language: "en-US", Value: "file-only"},
		},
	}
	s.fileStore.EXPECT().GetTranslations(mock.Anything).Return(fileTrans, nil)
	s.dbStore.EXPECT().GetTranslations(mock.Anything).Return(map[string]map[string]Translation{}, nil)

	result, err := s.store.GetTranslations(context.Background())
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "file-only", result["ns1|key1"]["en-US"].Value)
}

func (s *CompositeStoreTestSuite) TestGetTranslations_DBOnlyKeyPreserved() {
	dbTrans := map[string]map[string]Translation{
		"ns1|key2": {
			"en-US": {Key: "key2", Namespace: "ns1", Language: "en-US", Value: "db-only"},
		},
	}
	s.fileStore.EXPECT().GetTranslations(mock.Anything).Return(map[string]map[string]Translation{}, nil)
	s.dbStore.EXPECT().GetTranslations(mock.Anything).Return(dbTrans, nil)

	result, err := s.store.GetTranslations(context.Background())
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "db-only", result["ns1|key2"]["en-US"].Value)
}

func (s *CompositeStoreTestSuite) TestGetTranslations_FileStoreError() {
	s.fileStore.EXPECT().GetTranslations(mock.Anything).Return(nil, errors.New("file error"))

	result, err := s.store.GetTranslations(context.Background())
	assert.Error(s.T(), err)
	assert.Nil(s.T(), result)
}

func (s *CompositeStoreTestSuite) TestGetTranslations_DBStoreError() {
	s.fileStore.EXPECT().GetTranslations(mock.Anything).Return(map[string]map[string]Translation{}, nil)
	s.dbStore.EXPECT().GetTranslations(mock.Anything).Return(nil, errors.New("db error"))

	result, err := s.store.GetTranslations(context.Background())
	assert.Error(s.T(), err)
	assert.Nil(s.T(), result)
}

// GetTranslationsByNamespace

func (s *CompositeStoreTestSuite) TestGetTranslationsByNamespace_DBOverridesFile() {
	fileTrans := map[string]map[string]Translation{
		"signin|forms.title": {
			"en-US": {Key: "forms.title", Namespace: "signin", Language: "en-US", Value: "Sign In"},
		},
	}
	dbTrans := map[string]map[string]Translation{
		"signin|forms.title": {
			"en-US": {Key: "forms.title", Namespace: "signin", Language: "en-US", Value: "Log In"},
		},
	}
	s.fileStore.EXPECT().GetTranslationsByNamespace(mock.Anything, "signin").Return(fileTrans, nil)
	s.dbStore.EXPECT().GetTranslationsByNamespace(mock.Anything, "signin").Return(dbTrans, nil)

	result, err := s.store.GetTranslationsByNamespace(context.Background(), "signin")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "Log In", result["signin|forms.title"]["en-US"].Value)
}

func (s *CompositeStoreTestSuite) TestGetTranslationsByNamespace_FileStoreError() {
	s.fileStore.EXPECT().GetTranslationsByNamespace(mock.Anything, "signin").Return(nil, errors.New("file error"))

	result, err := s.store.GetTranslationsByNamespace(context.Background(), "signin")
	assert.Error(s.T(), err)
	assert.Nil(s.T(), result)
}

func (s *CompositeStoreTestSuite) TestGetTranslationsByNamespace_DBStoreError() {
	s.fileStore.EXPECT().GetTranslationsByNamespace(mock.Anything, "signin").Return(map[string]map[string]Translation{},
		nil)
	s.dbStore.EXPECT().GetTranslationsByNamespace(mock.Anything, "signin").Return(nil, errors.New("db error"))

	result, err := s.store.GetTranslationsByNamespace(context.Background(), "signin")
	assert.Error(s.T(), err)
	assert.Nil(s.T(), result)
}

// GetTranslationsByKey

func (s *CompositeStoreTestSuite) TestGetTranslationsByKey_DBOverridesFilePerLanguage() {
	fileTrans := map[string]Translation{
		"en-US": {Key: "forms.title", Namespace: "signin", Language: "en-US", Value: "Sign In"},
		"fr-FR": {Key: "forms.title", Namespace: "signin", Language: "fr-FR", Value: "Se connecter"},
	}
	dbTrans := map[string]Translation{
		"en-US": {Key: "forms.title", Namespace: "signin", Language: "en-US", Value: "Log In"},
	}
	s.fileStore.EXPECT().GetTranslationsByKey(mock.Anything, "forms.title", "signin").Return(fileTrans, nil)
	s.dbStore.EXPECT().GetTranslationsByKey(mock.Anything, "forms.title", "signin").Return(dbTrans, nil)

	result, err := s.store.GetTranslationsByKey(context.Background(), "forms.title", "signin")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "Log In", result["en-US"].Value)
	assert.Equal(s.T(), "Se connecter", result["fr-FR"].Value)
}

func (s *CompositeStoreTestSuite) TestGetTranslationsByKey_FileOnlyLanguagePreserved() {
	fileTrans := map[string]Translation{
		"en-US": {Key: "key1", Namespace: "ns1", Language: "en-US", Value: "file-value"},
	}
	s.fileStore.EXPECT().GetTranslationsByKey(mock.Anything, "key1", "ns1").Return(fileTrans, nil)
	s.dbStore.EXPECT().GetTranslationsByKey(mock.Anything, "key1", "ns1").Return(map[string]Translation{}, nil)

	result, err := s.store.GetTranslationsByKey(context.Background(), "key1", "ns1")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "file-value", result["en-US"].Value)
}

func (s *CompositeStoreTestSuite) TestGetTranslationsByKey_FileStoreError() {
	s.fileStore.EXPECT().GetTranslationsByKey(mock.Anything, "key1", "ns1").Return(nil, errors.New("file error"))

	result, err := s.store.GetTranslationsByKey(context.Background(), "key1", "ns1")
	assert.Error(s.T(), err)
	assert.Nil(s.T(), result)
}

func (s *CompositeStoreTestSuite) TestGetTranslationsByKey_DBStoreError() {
	s.fileStore.EXPECT().GetTranslationsByKey(mock.Anything, "key1", "ns1").Return(map[string]Translation{}, nil)
	s.dbStore.EXPECT().GetTranslationsByKey(mock.Anything, "key1", "ns1").Return(nil, errors.New("db error"))

	result, err := s.store.GetTranslationsByKey(context.Background(), "key1", "ns1")
	assert.Error(s.T(), err)
	assert.Nil(s.T(), result)
}

// Write operations — all delegate to dbStore only

func (s *CompositeStoreTestSuite) TestUpsertTranslationsByLanguage_DelegatesToDB() {
	trans := []Translation{{Key: "key1", Language: "en-US", Namespace: "ns1", Value: "v1"}}
	s.dbStore.EXPECT().UpsertTranslationsByLanguage("en-US", trans).Return(nil)

	err := s.store.UpsertTranslationsByLanguage("en-US", trans)
	assert.NoError(s.T(), err)
}

func (s *CompositeStoreTestSuite) TestUpsertTranslation_DelegatesToDB() {
	trans := Translation{Key: "key1", Language: "en-US", Namespace: "ns1", Value: "v1"}
	s.dbStore.EXPECT().UpsertTranslation(trans).Return(nil)

	err := s.store.UpsertTranslation(trans)
	assert.NoError(s.T(), err)
}

func (s *CompositeStoreTestSuite) TestUpsertTranslations_DelegatesToDB() {
	ctx := context.Background()
	trans := []Translation{{Key: "key1", Language: "en-US", Namespace: "ns1", Value: "v1"}}
	s.dbStore.EXPECT().UpsertTranslations(ctx, trans).Return(nil)

	err := s.store.UpsertTranslations(ctx, trans)
	assert.NoError(s.T(), err)
}

func (s *CompositeStoreTestSuite) TestDeleteTranslationsByLanguage_DelegatesToDB() {
	s.dbStore.EXPECT().DeleteTranslationsByLanguage("en-US").Return(nil)

	err := s.store.DeleteTranslationsByLanguage("en-US")
	assert.NoError(s.T(), err)
}

func (s *CompositeStoreTestSuite) TestDeleteTranslation_DelegatesToDB() {
	s.dbStore.EXPECT().DeleteTranslation("en-US", "key1", "ns1").Return(nil)

	err := s.store.DeleteTranslation("en-US", "key1", "ns1")
	assert.NoError(s.T(), err)
}

func (s *CompositeStoreTestSuite) TestDeleteTranslationsByNamespace_DelegatesToDB() {
	ctx := context.Background()
	s.dbStore.EXPECT().DeleteTranslationsByNamespace(ctx, "ns1").Return(nil)

	err := s.store.DeleteTranslationsByNamespace(ctx, "ns1")
	assert.NoError(s.T(), err)
}

func (s *CompositeStoreTestSuite) TestDeleteTranslationsByKey_DelegatesToDB() {
	ctx := context.Background()
	s.dbStore.EXPECT().DeleteTranslationsByKey(ctx, "ns1", "key1").Return(nil)

	err := s.store.DeleteTranslationsByKey(ctx, "ns1", "key1")
	assert.NoError(s.T(), err)
}

// mergeTranslationMaps

func (s *CompositeStoreTestSuite) TestMergeTranslationMaps_EmptyBase() {
	overrides := map[string]map[string]Translation{
		"ns1|key1": {"en-US": {Value: "override"}},
	}
	result := mergeTranslationMaps(map[string]map[string]Translation{}, overrides)
	assert.Equal(s.T(), "override", result["ns1|key1"]["en-US"].Value)
}

func (s *CompositeStoreTestSuite) TestMergeTranslationMaps_EmptyOverrides() {
	base := map[string]map[string]Translation{
		"ns1|key1": {"en-US": {Value: "base"}},
	}
	result := mergeTranslationMaps(base, map[string]map[string]Translation{})
	assert.Equal(s.T(), "base", result["ns1|key1"]["en-US"].Value)
}

func (s *CompositeStoreTestSuite) TestMergeTranslationMaps_OverrideWins() {
	base := map[string]map[string]Translation{
		"ns1|key1": {"en-US": {Value: "base"}},
	}
	overrides := map[string]map[string]Translation{
		"ns1|key1": {"en-US": {Value: "override"}},
	}
	result := mergeTranslationMaps(base, overrides)
	assert.Equal(s.T(), "override", result["ns1|key1"]["en-US"].Value)
}

func (s *CompositeStoreTestSuite) TestMergeTranslationMaps_PerLanguageOverride() {
	base := map[string]map[string]Translation{
		"ns1|key1": {
			"en-US": {Value: "base-en"},
			"fr-FR": {Value: "base-fr"},
		},
	}
	overrides := map[string]map[string]Translation{
		"ns1|key1": {
			"en-US": {Value: "override-en"},
		},
	}
	result := mergeTranslationMaps(base, overrides)
	assert.Equal(s.T(), "override-en", result["ns1|key1"]["en-US"].Value)
	assert.Equal(s.T(), "base-fr", result["ns1|key1"]["fr-FR"].Value)
}

func (s *CompositeStoreTestSuite) TestMergeTranslationMaps_DisjointKeys() {
	base := map[string]map[string]Translation{
		"ns1|key1": {"en-US": {Value: "v1"}},
	}
	overrides := map[string]map[string]Translation{
		"ns1|key2": {"en-US": {Value: "v2"}},
	}
	result := mergeTranslationMaps(base, overrides)
	assert.Len(s.T(), result, 2)
	assert.Equal(s.T(), "v1", result["ns1|key1"]["en-US"].Value)
	assert.Equal(s.T(), "v2", result["ns1|key2"]["en-US"].Value)
}
