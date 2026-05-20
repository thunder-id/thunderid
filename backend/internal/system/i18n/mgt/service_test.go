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
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

const (
	testErrKey = "error.i18nservice.invalid_language"
	testErrVal = "Invalid language tag format"
)

type I18nMgtServiceTestSuite struct {
	suite.Suite
	mockStore *i18nStoreInterfaceMock
	service   I18nServiceInterface
}

func TestI18nMgtServiceTestSuite(t *testing.T) {
	suite.Run(t, new(I18nMgtServiceTestSuite))
}

func (suite *I18nMgtServiceTestSuite) SetupTest() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)
	suite.mockStore = newI18nStoreInterfaceMock(suite.T())
	suite.service = newI18nService(suite.mockStore)
}

func (suite *I18nMgtServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// ListLanguages Tests
func (suite *I18nMgtServiceTestSuite) TestListLanguages_Success() {
	expectedLangs := []string{"en-US", "fr-FR"}
	suite.mockStore.On("GetDistinctLanguages").Return(expectedLangs, nil)

	result, err := suite.service.ListLanguages()

	suite.Nil(err)
	suite.NotNil(result)
	suite.Contains(result, "en-US")
	suite.Contains(result, "fr-FR")
}

func (suite *I18nMgtServiceTestSuite) TestListLanguages_StoreError() {
	suite.mockStore.On("GetDistinctLanguages").Return(nil, errors.New("db error"))

	result, err := suite.service.ListLanguages()

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestListLanguages_AddsSystemLanguage() {
	// If store returns empty or doesn't have system language, it should be added
	suite.mockStore.On("GetDistinctLanguages").Return([]string{"fr-FR"}, nil)

	result, err := suite.service.ListLanguages()

	suite.Nil(err)
	suite.Contains(result, SystemLanguage)
	suite.Contains(result, "fr-FR")
}

// ResolveTranslationsForKey Tests
func (suite *I18nMgtServiceTestSuite) TestResolveTranslationsForKey_Success_FromStore() {
	translation := Translation{
		Key:       "welcome",
		Namespace: "common",
		Language:  "en-US",
		Value:     "Welcome override",
	}
	translationsMap := map[string]Translation{"en-US": translation}

	suite.mockStore.On("GetTranslationsByKey", "welcome", "common").Return(translationsMap, nil)

	result, err := suite.service.ResolveTranslationsForKey("en-US", "common", "welcome")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("Welcome override", result.Value)
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslationsForKey_ValidationErrors() {
	testCases := []struct {
		name      string
		lang      string
		namespace string
		key       string
		errCode   string
	}{
		{"MissingLanguage", "", "ns", "key", ErrorMissingLanguage.Code},
		{"InvalidLanguage", "invalid", "ns", "key", ErrorInvalidLanguage.Code},
		{"InvalidNamespace", "en-US", "invalid!", "key", ErrorInvalidNamespace.Code},
		{"InvalidKey", "en-US", "ns", "invalid key!", ErrorInvalidKey.Code},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result, err := suite.service.ResolveTranslationsForKey(tc.lang, tc.namespace, tc.key)
			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.errCode, err.Code)
		})
	}
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslationsForKey_NotFound() {
	suite.mockStore.On("GetTranslationsByKey", "unknown", "common").Return((map[string]Translation)(nil), nil)

	result, err := suite.service.ResolveTranslationsForKey("en-US", "common", "unknown")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorTranslationNotFound.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslationsForKey_StoreError() {
	suite.mockStore.On("GetTranslationsByKey", "welcome", "common").Return(nil, errors.New("db error"))

	result, err := suite.service.ResolveTranslationsForKey("en-US", "common", "welcome")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslationsForKey_UsesSystemDefault_WhenKeyMissingInDB() {
	key := testErrKey
	expectedValue := testErrVal

	suite.mockStore.On("GetTranslationsByKey", key, SystemNamespace).Return(make(map[string]Translation), nil)

	// Request en-US, expecting fallback to system default (en)
	result, err := suite.service.ResolveTranslationsForKey("en-US", SystemNamespace, key)

	suite.Nil(err)
	suite.NotNil(result)
	// The resolved translation should be the system default
	suite.Equal(expectedValue, result.Value)
	// The returned language should match the requested language
	suite.Equal("en-US", result.Language)
	suite.Equal(SystemNamespace, result.Namespace)
	suite.Equal(key, result.Key)
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslationsForKey_SystemDefault_WhenOnlyDiffLangsInDB() {
	key := testErrKey
	expectedDefaultValue := testErrVal

	// Mock: DB has translation for "fr-FR", but not for "en" (SystemLanguage)
	frTranslation := Translation{
		Key:       key,
		Namespace: SystemNamespace,
		Language:  "fr-FR",
		Value:     "Erreur interne du serveur",
	}
	dbTranslations := map[string]Translation{"fr-FR": frTranslation}

	suite.mockStore.On("GetTranslationsByKey", key, SystemNamespace).Return(dbTranslations, nil)

	// Request en-US, expecting fallback to system default (en)
	result, err := suite.service.ResolveTranslationsForKey("en-US", SystemNamespace, key)

	suite.Nil(err)
	suite.NotNil(result)
	// The resolved translation should be the system default
	suite.Equal(expectedDefaultValue, result.Value)
	// The returned language should match the requested language
	suite.Equal("en-US", result.Language)
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslationsForKey_UsesDBValue_WithSystemLangOverride() {
	key := testErrKey

	// Mock: DB has translation for "en" (SystemLanguage) with an override value
	overrideValue := "Custom Internal Error"
	dbTranslation := Translation{
		Key:       key,
		Namespace: SystemNamespace,
		Language:  SystemLanguage,
		Value:     overrideValue,
	}
	dbTranslations := map[string]Translation{SystemLanguage: dbTranslation}

	suite.mockStore.On("GetTranslationsByKey", key, SystemNamespace).Return(dbTranslations, nil)

	// Request en-US, expecting fallback to system default (en)
	result, err := suite.service.ResolveTranslationsForKey("en-US", SystemNamespace, key)

	suite.Nil(err)
	suite.NotNil(result)
	// Should use the DB override, not the hardcoded default "Internal server error"
	suite.Equal(overrideValue, result.Value)
	// The returned language should match the requested language
	suite.Equal("en-US", result.Language)
}

// SetTranslationOverrideForKey Tests
func (suite *I18nMgtServiceTestSuite) TestSetTranslationOverrideForKey_Success() {
	suite.mockStore.On("UpsertTranslation", mock.AnythingOfType("mgt.Translation")).Return(nil)

	result, err := suite.service.SetTranslationOverrideForKey("en-US", "common", "welcome", "Hello")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("Hello", result.Value)
}

func (suite *I18nMgtServiceTestSuite) TestSetTranslationOverrideForKey_ValidationErrors() {
	// Simple check for one validation case as others share logic
	result, err := suite.service.SetTranslationOverrideForKey("", "ns", "key", "val")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingLanguage.Code, err.Code)

	// Invalid Lang
	result, err = suite.service.SetTranslationOverrideForKey("invalid", "ns", "key", "val")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidLanguage.Code, err.Code)

	// Invalid Namespace
	result, err = suite.service.SetTranslationOverrideForKey("en-US", "invalid!", "key", "val")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidNamespace.Code, err.Code)

	// Invalid Key
	result, err = suite.service.SetTranslationOverrideForKey("en-US", "common", "invalid key!", "val")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidKey.Code, err.Code)

	result, err = suite.service.SetTranslationOverrideForKey("en-US", "ns", "key", "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingValue.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestSetTranslationOverrideForKey_StoreError() {
	suite.mockStore.On("UpsertTranslation", mock.AnythingOfType("mgt.Translation")).Return(errors.New("db error"))

	result, err := suite.service.SetTranslationOverrideForKey("en-US", "common", "welcome", "Hello")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestSetTranslationOverrideForKey_Declarative() {
	// Enable declarative mode
	config.GetServerRuntime().Config.DeclarativeResources.Enabled = true
	defer func() {
		config.GetServerRuntime().Config.DeclarativeResources.Enabled = false
	}()

	result, err := suite.service.SetTranslationOverrideForKey("en-US", "common", "welcome", "Hello")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(declarativeresource.ErrorDeclarativeResourceUpdateOperation.Code, err.Code)
}

// ClearTranslationOverrideForKey Tests
func (suite *I18nMgtServiceTestSuite) TestClearTranslationOverrideForKey_Success() {
	suite.mockStore.On("DeleteTranslation", "en-US", "welcome", "common").Return(nil)

	err := suite.service.ClearTranslationOverrideForKey("en-US", "common", "welcome")

	suite.Nil(err)
}

func (suite *I18nMgtServiceTestSuite) TestClearTranslationOverrideForKey_ValidationErrors() {
	err := suite.service.ClearTranslationOverrideForKey("", "ns", "key")
	suite.NotNil(err)
	suite.Equal(ErrorMissingLanguage.Code, err.Code)

	err = suite.service.ClearTranslationOverrideForKey("invalid", "ns", "key")
	suite.NotNil(err)
	suite.Equal(ErrorInvalidLanguage.Code, err.Code)

	err = suite.service.ClearTranslationOverrideForKey("en-US", "invalid!", "key")
	suite.NotNil(err)
	suite.Equal(ErrorInvalidNamespace.Code, err.Code)

	err = suite.service.ClearTranslationOverrideForKey("en-US", "ns", "invalid key!")
	suite.NotNil(err)
	suite.Equal(ErrorInvalidKey.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestClearTranslationOverrideForKey_StoreError() {
	suite.mockStore.On("DeleteTranslation", "en-US", "welcome", "common").Return(errors.New("db error"))

	err := suite.service.ClearTranslationOverrideForKey("en-US", "common", "welcome")

	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestClearTranslationOverrideForKey_Declarative() {
	// Enable declarative mode
	config.GetServerRuntime().Config.DeclarativeResources.Enabled = true
	defer func() {
		config.GetServerRuntime().Config.DeclarativeResources.Enabled = false
	}()

	err := suite.service.ClearTranslationOverrideForKey("en-US", "common", "welcome")

	suite.NotNil(err)
	suite.Equal(declarativeresource.ErrorDeclarativeResourceDeleteOperation.Code, err.Code)
}

// ResolveTranslations Tests
func (suite *I18nMgtServiceTestSuite) TestResolveTranslations_CustomNamespace_Success() {
	translation := Translation{
		Key:       "btn_ok",
		Namespace: "console",
		Language:  "en-US",
		Value:     "OK",
	}

	// Let's correct the mock data structure
	mockDataCorrect := map[string]map[string]Translation{
		"btn_ok": {
			"en-US": translation,
		},
	}

	suite.mockStore.On("GetTranslationsByNamespace", "console").Return(mockDataCorrect, nil)

	result, err := suite.service.ResolveTranslations("en-US", "console")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(1, result.TotalResults)
	suite.Equal("OK", result.Translations["console"]["btn_ok"])
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslations_InvalidNamespace() {
	result, err := suite.service.ResolveTranslations("en-US", "invalid!")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidNamespace.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslations_StoreError() {
	suite.mockStore.On("GetTranslationsByNamespace", "console").Return(nil, errors.New("db error"))

	result, err := suite.service.ResolveTranslations("en-US", "console")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslations_UsesSystemDefaults_WhenKeyMissingInDB() {
	// Mock: no custom translations in DB
	suite.mockStore.On("GetTranslationsByNamespace", "system").
		Return(make(map[string]map[string]Translation), nil)

	key := testErrKey
	expectedDefaultValue := testErrVal

	result, err := suite.service.ResolveTranslations("en-US", "system")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Contains(result.Translations[SystemNamespace], key)
	suite.Equal(expectedDefaultValue, result.Translations[SystemNamespace][key])
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslations_UsesSystemDefaults_WhenDBHasOnlyDifferentLanguages() {
	// Key known to be in defaults
	key := testErrKey
	expectedDefaultValue := testErrVal

	// Mock: DB has translation for "fr-FR", but not for "en" (SystemLanguage)
	frTranslation := Translation{
		Key:       key,
		Namespace: SystemNamespace,
		Language:  "fr-FR",
		Value:     "Erreur interne du serveur",
	}
	dbTranslations := map[string]map[string]Translation{
		SystemNamespace + "|" + key: {"fr-FR": frTranslation},
	}

	suite.mockStore.On("GetTranslationsByNamespace", "system").Return(dbTranslations, nil)

	result, err := suite.service.ResolveTranslations("en-US", "system")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Contains(result.Translations[SystemNamespace], key)
	suite.Equal(expectedDefaultValue, result.Translations[SystemNamespace][key])
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslations_UsesDBValue_WhenDBOverrideExistsForSystemLanguage() {
	key := testErrKey
	overrideValue := "Custom Internal Error"

	// Mock: DB has an override for the system language
	translationDB := Translation{
		Key:       key,
		Namespace: SystemNamespace,
		Language:  SystemLanguage,
		Value:     overrideValue,
	}
	dbTranslations := map[string]map[string]Translation{
		SystemNamespace + "|" + key: {SystemLanguage: translationDB},
	}

	suite.mockStore.On("GetTranslationsByNamespace", "system").Return(dbTranslations, nil)

	result, err := suite.service.ResolveTranslations("en-US", "system")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Contains(result.Translations[SystemNamespace], key)
	suite.Equal(overrideValue, result.Translations[SystemNamespace][key])
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslations_DefaultsFallback() {
	// Mock: no custom translations in DB
	suite.mockStore.On("GetTranslationsByNamespace", "system").
		Return(make(map[string]map[string]Translation), nil)

	result, err := suite.service.ResolveTranslations("fr-FR", "system")

	suite.Nil(err)
	suite.NotNil(result)
	// Just verify we got a result structure back even if translations loop was empty or default only
	suite.Equal("fr-FR", result.Language)
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslations_MergeLogic() {
	// DB has an override for "welcome"
	translationDB := Translation{
		Key:       "welcome",
		Namespace: "system",
		Language:  "en-US",
		Value:     "Welcome Override",
	}
	dbTranslations := map[string]map[string]Translation{
		"welcome": {"en-US": translationDB},
	}

	suite.mockStore.On("GetTranslationsByNamespace", "system").Return(dbTranslations, nil)

	result, err := suite.service.ResolveTranslations("en-US", "system")

	suite.Nil(err)
	suite.NotNil(result)
	// Should contain override
	suite.Equal("Welcome Override", result.Translations["system"]["welcome"])
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslations_AllNamespaces() {
	// Test without namespace filter
	// Mock: returns mixed namespaces
	dbTranslations := map[string]map[string]Translation{
		"k1": {"en-US": {Key: "k1", Namespace: "ns1", Language: "en-US", Value: "v1"}},
		"k2": {"en-US": {Key: "k2", Namespace: "ns2", Language: "en-US", Value: "v2"}},
	}

	suite.mockStore.On("GetTranslations").Return(dbTranslations, nil)

	result, err := suite.service.ResolveTranslations("en-US", "")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("v1", result.Translations["ns1"]["k1"])
	suite.Equal("v2", result.Translations["ns2"]["k2"])
}

func (suite *I18nMgtServiceTestSuite) TestResolveTranslations_AllNamespaces_StoreError() {
	suite.mockStore.On("GetTranslations").Return(nil, errors.New("db error"))

	result, err := suite.service.ResolveTranslations("en-US", "")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

const testAppNamespace = "app-abc-123"

// ResolveTranslations — non-system namespace exact-match behavior.

// TestResolveTranslations_AppNamespace_ExactMatch verifies that when the requested language
// exactly matches a stored translation in a non-system namespace, that value is returned.
func (suite *I18nMgtServiceTestSuite) TestResolveTranslations_AppNamespace_ExactMatch() {
	ns := testAppNamespace
	dbTranslations := map[string]map[string]Translation{
		ns + "|name": {
			"fr": {Key: "name", Namespace: ns, Language: "fr", Value: "Mon Application"},
		},
	}
	suite.mockStore.On("GetTranslationsByNamespace", ns).Return(dbTranslations, nil)

	result, err := suite.service.ResolveTranslations("fr", ns)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("Mon Application", result.Translations[ns]["name"])
}

// TestResolveTranslations_AppNamespace_BestMatchFallback verifies that when the requested
// language has no exact match in a non-system namespace, BCP47 best-match returns the closest
// available translation (consistent with system-namespace behavior).
func (suite *I18nMgtServiceTestSuite) TestResolveTranslations_AppNamespace_BestMatchFallback() {
	ns := testAppNamespace
	// Only "fr" is stored; "en" is requested — best-match returns the only available value.
	dbTranslations := map[string]map[string]Translation{
		ns + "|name": {
			"fr": {Key: "name", Namespace: ns, Language: "fr", Value: "Mon Application"},
		},
	}
	suite.mockStore.On("GetTranslationsByNamespace", ns).Return(dbTranslations, nil)

	result, err := suite.service.ResolveTranslations("en", ns)

	suite.Nil(err)
	suite.NotNil(result)
	// Best-match returns "fr" (the only stored language).
	suite.Equal("Mon Application", result.Translations[ns]["name"])
}

// TestResolveTranslations_SystemNamespace_StillFallsBack verifies that the system namespace
// continues to use BCP47 best-match fallback after the fix.
func (suite *I18nMgtServiceTestSuite) TestResolveTranslations_SystemNamespace_StillFallsBack() {
	key := testErrKey
	frTranslation := Translation{Key: key, Namespace: SystemNamespace, Language: "fr", Value: "Erreur"}
	dbTranslations := map[string]map[string]Translation{
		SystemNamespace + "|" + key: {"fr": frTranslation},
	}
	suite.mockStore.On("GetTranslationsByNamespace", SystemNamespace).Return(dbTranslations, nil)

	// Request "en-US" — no en-US stored, but system defaults fill it in.
	result, err := suite.service.ResolveTranslations("en-US", SystemNamespace)

	suite.Nil(err)
	suite.NotNil(result)
	// System default should be present (filled from sysi18n defaults), not the French value.
	suite.Contains(result.Translations[SystemNamespace], key)
	suite.NotEqual("Erreur", result.Translations[SystemNamespace][key])
}

// ResolveTranslationsForKey — non-system namespace exact-match behavior.

// TestResolveTranslationsForKey_AppNamespace_ExactMatch verifies exact-match lookup for a key
// in a non-system namespace when the requested language is stored.
func (suite *I18nMgtServiceTestSuite) TestResolveTranslationsForKey_AppNamespace_ExactMatch() {
	ns := testAppNamespace
	key := "name"
	suite.mockStore.On("GetTranslationsByKey", key, ns).Return(map[string]Translation{
		"fr": {Key: key, Namespace: ns, Language: "fr", Value: "Mon Application"},
	}, nil)

	result, err := suite.service.ResolveTranslationsForKey("fr", ns, key)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("Mon Application", result.Value)
}

// TestResolveTranslationsForKey_AppNamespace_BestMatchFallback verifies that when the requested
// language has no exact match in a non-system namespace, BCP47 best-match returns the closest
// available translation (consistent with system-namespace behavior).
func (suite *I18nMgtServiceTestSuite) TestResolveTranslationsForKey_AppNamespace_BestMatchFallback() {
	ns := testAppNamespace
	key := "name"
	// Only "fr" stored; "en" requested — best-match returns the only available value.
	suite.mockStore.On("GetTranslationsByKey", key, ns).Return(map[string]Translation{
		"fr": {Key: key, Namespace: ns, Language: "fr", Value: "Mon Application"},
	}, nil)

	result, err := suite.service.ResolveTranslationsForKey("en", ns, key)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("Mon Application", result.Value)
}

// NormaliseBCP47Tag Tests

func TestNormaliseBCP47Tag(t *testing.T) {
	tests := []struct {
		name      string
		tag       string
		wantTag   string
		wantValid bool
	}{
		{"EmptyTag", "", "", false},
		{"TooLong", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "", false},
		{"InvalidTag", "not-a-valid-!!-tag", "", false},
		{"ValidSimple", "fr", "fr", true},
		{"ValidWithRegion", "en-US", "en-US", true},
		{"NormalisesCase", "en-us", "en-US", true},
		{"NormalisesUppercase", "FR", "fr", true},
		{"ValidComplex", "zh-Hans-CN", "zh-Hans-CN", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, valid := NormaliseBCP47Tag(tc.tag)
			if valid != tc.wantValid {
				t.Errorf("NormaliseBCP47Tag(%q) valid = %v, want %v", tc.tag, valid, tc.wantValid)
			}
			if tc.wantValid && got != tc.wantTag {
				t.Errorf("NormaliseBCP47Tag(%q) tag = %q, want %q", tc.tag, got, tc.wantTag)
			}
		})
	}
}

func (suite *I18nMgtServiceTestSuite) TestCompareLangs() {
	// Directly test the unexported compareLangs function

	// Case 1: Both exist (en-US=0, en=1) -> 0 - 1 = -1
	suite.Equal(-1, compareLangs("en-US", "en"))

	// Case 2: Both exist reverse (en=1, en-US=0) -> 1 - 0 = 1
	suite.Equal(1, compareLangs("en", "en-US"))

	// Case 3: A exists, B does not
	suite.Equal(-1, compareLangs("en-US", "fr"))

	// Case 4: A does not exist, B does
	suite.Equal(1, compareLangs("fr", "en-US"))

	// Case 5: Neither exists
	suite.Equal(0, compareLangs("fr", "de"))

	// Case 6: Same language
	suite.Equal(0, compareLangs("en-US", "en-US"))
}

// SetTranslationOverrides Tests
func (suite *I18nMgtServiceTestSuite) TestSetTranslationOverrides_Success() {
	translations := map[string]map[string]string{
		"console": {
			"k1": "v1",
		},
	}

	suite.mockStore.On("UpsertTranslationsByLanguage", "en-US", mock.AnythingOfType("[]mgt.Translation")).Return(nil)

	result, err := suite.service.SetTranslationOverrides("en-US", translations)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(1, result.TotalResults)
}

func (suite *I18nMgtServiceTestSuite) TestSetTranslationOverrides_Empty() {
	translations := map[string]map[string]string{}
	result, err := suite.service.SetTranslationOverrides("en-US", translations)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorEmptyTranslations.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestSetTranslationOverrides_ValidationErrors() {
	// Invalid Namespace
	translations1 := map[string]map[string]string{
		"invalid!": {"k": "v"},
	}
	result, err := suite.service.SetTranslationOverrides("en-US", translations1)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidNamespace.Code, err.Code)

	// Invalid Key
	translations2 := map[string]map[string]string{
		"console": {"invalid key!": "v"},
	}
	result, err = suite.service.SetTranslationOverrides("en-US", translations2)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidKey.Code, err.Code)

	// Empty Value
	translations3 := map[string]map[string]string{
		"console": {"key": ""},
	}
	result, err = suite.service.SetTranslationOverrides("en-US", translations3)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingValue.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestSetTranslationOverrides_StoreError() {
	translations := map[string]map[string]string{
		"console": {"k": "v"},
	}
	suite.mockStore.On("UpsertTranslationsByLanguage", "en-US", mock.AnythingOfType("[]mgt.Translation")).
		Return(errors.New("db error"))

	result, err := suite.service.SetTranslationOverrides("en-US", translations)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestSetTranslationOverrides_Declarative() {
	// Enable declarative mode
	config.GetServerRuntime().Config.DeclarativeResources.Enabled = true
	defer func() {
		config.GetServerRuntime().Config.DeclarativeResources.Enabled = false
	}()

	translations := map[string]map[string]string{
		"console": {"k": "v"},
	}

	result, err := suite.service.SetTranslationOverrides("en-US", translations)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(declarativeresource.ErrorDeclarativeResourceUpdateOperation.Code, err.Code)
}

// ClearTranslationOverrides Tests
func (suite *I18nMgtServiceTestSuite) TestClearTranslationOverrides_Success() {
	suite.mockStore.On("DeleteTranslationsByLanguage", "en-US").Return(nil)

	err := suite.service.ClearTranslationOverrides("en-US")

	suite.Nil(err)
}

func (suite *I18nMgtServiceTestSuite) TestClearTranslationOverrides_StoreError() {
	suite.mockStore.On("DeleteTranslationsByLanguage", "en-US").Return(errors.New("db error"))

	err := suite.service.ClearTranslationOverrides("en-US")

	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestClearTranslationOverrides_ValidationErrors() {
	err := suite.service.ClearTranslationOverrides("")
	suite.NotNil(err)
	suite.Equal(ErrorMissingLanguage.Code, err.Code)

	err = suite.service.ClearTranslationOverrides("invalid")
	suite.NotNil(err)
	suite.Equal(ErrorInvalidLanguage.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestClearTranslationOverrides_Declarative() {
	// Enable declarative mode
	config.GetServerRuntime().Config.DeclarativeResources.Enabled = true
	defer func() {
		config.GetServerRuntime().Config.DeclarativeResources.Enabled = false
	}()

	err := suite.service.ClearTranslationOverrides("en-US")

	suite.NotNil(err)
	suite.Equal(declarativeresource.ErrorDeclarativeResourceDeleteOperation.Code, err.Code)
}

// GetTranslationsByNamespace Tests

func (suite *I18nMgtServiceTestSuite) TestGetTranslationsByNamespace_InvalidNamespace() {
	result, err := suite.service.GetTranslationsByNamespace("invalid!")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidNamespace.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestGetTranslationsByNamespace_StoreError() {
	suite.mockStore.On("GetTranslationsByNamespace", "app-test").
		Return(nil, errors.New("db error"))

	result, err := suite.service.GetTranslationsByNamespace("app-test")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestGetTranslationsByNamespace_Success() {
	ns := "app-test"
	dbData := map[string]map[string]Translation{
		ns + "|name": {
			"fr": {Key: "name", Namespace: ns, Language: "fr", Value: "Mon App"},
			"de": {Key: "name", Namespace: ns, Language: "de", Value: "Meine App"},
		},
		ns + "|logo_uri": {
			"fr": {Key: "logo_uri", Namespace: ns, Language: "fr", Value: "https://example.com/fr/logo.png"},
		},
	}
	suite.mockStore.On("GetTranslationsByNamespace", ns).Return(dbData, nil)

	result, err := suite.service.GetTranslationsByNamespace(ns)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("Mon App", result["name"]["fr"])
	suite.Equal("Meine App", result["name"]["de"])
	suite.Equal("https://example.com/fr/logo.png", result["logo_uri"]["fr"])
}

func (suite *I18nMgtServiceTestSuite) TestGetTranslationsByNamespace_SkipsMalformedCompositeKey() {
	ns := "app-test"
	// A key without "|" separator should be skipped
	dbData := map[string]map[string]Translation{
		"malformed-key": {
			"fr": {Key: "name", Namespace: ns, Language: "fr", Value: "Mon App"},
		},
		ns + "|name": {
			"en": {Key: "name", Namespace: ns, Language: "en", Value: "My App"},
		},
	}
	suite.mockStore.On("GetTranslationsByNamespace", ns).Return(dbData, nil)

	result, err := suite.service.GetTranslationsByNamespace(ns)

	suite.Nil(err)
	suite.NotNil(result)
	// Malformed key is skipped; valid key is present
	suite.Equal("My App", result["name"]["en"])
	suite.NotContains(result, "malformed-key")
}

// DeleteTranslationsByNamespace Tests

func (suite *I18nMgtServiceTestSuite) TestDeleteTranslationsByNamespace_InvalidNamespace() {
	err := suite.service.DeleteTranslationsByNamespace(context.Background(), "invalid!")

	suite.NotNil(err)
	suite.Equal(ErrorInvalidNamespace.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestDeleteTranslationsByNamespace_StoreError() {
	suite.mockStore.On("DeleteTranslationsByNamespace", mock.Anything, "app-test").
		Return(errors.New("db error"))

	err := suite.service.DeleteTranslationsByNamespace(context.Background(), "app-test")

	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestDeleteTranslationsByNamespace_Success() {
	suite.mockStore.On("DeleteTranslationsByNamespace", mock.Anything, "app-test").Return(nil)

	err := suite.service.DeleteTranslationsByNamespace(context.Background(), "app-test")

	suite.Nil(err)
}

// DeleteTranslationsByKey Tests

func (suite *I18nMgtServiceTestSuite) TestDeleteTranslationsByKey_InvalidNamespace() {
	err := suite.service.DeleteTranslationsByKey(context.Background(), "invalid!", "name")

	suite.NotNil(err)
	suite.Equal(ErrorInvalidNamespace.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestDeleteTranslationsByKey_InvalidKey() {
	err := suite.service.DeleteTranslationsByKey(context.Background(), "custom", "invalid key!")

	suite.NotNil(err)
	suite.Equal(ErrorInvalidKey.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestDeleteTranslationsByKey_StoreError() {
	suite.mockStore.On("DeleteTranslationsByKey", mock.Anything, "custom", "app.test-id.name").
		Return(errors.New("db error"))

	err := suite.service.DeleteTranslationsByKey(context.Background(), "custom", "app.test-id.name")

	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *I18nMgtServiceTestSuite) TestDeleteTranslationsByKey_Success() {
	suite.mockStore.On("DeleteTranslationsByKey", mock.Anything, "custom", "app.test-id.name").Return(nil)

	err := suite.service.DeleteTranslationsByKey(context.Background(), "custom", "app.test-id.name")

	suite.Nil(err)
}
