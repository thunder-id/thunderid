// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package i18n

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// I18nMgtAPITestSuite covers translation management on a data plane: overrides written per language
// through an import with the management API key, the only way a data plane takes them, read back
// through the resolve endpoints; the validation branches for language/namespace/key/value; and the
// gates on the write endpoints, which refuse every token. The resolve endpoints are public; the write
// endpoints are not (backend/internal/system/security/permissions.go).
type I18nMgtAPITestSuite struct {
	suite.Suite
	adminClient *http.Client
	plainClient *http.Client
}

func TestI18nMgtAPITestSuite(t *testing.T) {
	suite.Run(t, new(I18nMgtAPITestSuite))
}

func (suite *I18nMgtAPITestSuite) SetupSuite() {
	suite.adminClient = testutils.GetHTTPClient()
	suite.plainClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// SetupTest clears the suite's language so each test starts from a known state.
func (suite *I18nMgtAPITestSuite) SetupTest() {
	suite.clearLanguage(testLanguage)
}

// TearDownSuite leaves no overrides behind for the shared server.
func (suite *I18nMgtAPITestSuite) TearDownSuite() {
	suite.clearLanguage(testLanguage)
}

// --- GET /i18n/languages ---

func (suite *I18nMgtAPITestSuite) TestListLanguagesAlwaysIncludesSystemLanguage() {
	status, body := suite.do(suite.adminClient, http.MethodGet, languagesURL, nil)

	suite.Equal(http.StatusOK, status)
	var resp languageListResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Contains(resp.Languages, systemLanguage)
}

func (suite *I18nMgtAPITestSuite) TestListLanguagesReflectsNewlyWrittenLanguage() {
	suite.Require().NotContains(suite.listLanguages(), testLanguage)

	suite.setOverrides(testLanguage, map[string]map[string]string{testNamespace: {"greeting": "Bonjour"}})

	suite.Contains(suite.listLanguages(), testLanguage)
}

func (suite *I18nMgtAPITestSuite) TestListLanguagesIsPublic() {
	status, _ := suite.do(suite.plainClient, http.MethodGet, languagesURL, nil)

	suite.Equal(http.StatusOK, status)
}

// --- Overrides written through import ---

func (suite *I18nMgtAPITestSuite) TestSetOverridesRoundTrip() {
	suite.setOverrides(testLanguage, map[string]map[string]string{
		testNamespace: {"greeting": "Bonjour", "farewell": "Au revoir"},
	})

	resolved := suite.resolveLanguage(testLanguage, testNamespace)
	suite.Equal(testLanguage, resolved.Language)
	suite.Equal(map[string]string{"greeting": "Bonjour", "farewell": "Au revoir"},
		resolved.Translations[testNamespace])

	key := suite.resolveKey(testLanguage, testNamespace, "greeting")
	suite.Equal("Bonjour", key.Value)
	suite.Equal(testNamespace, key.Namespace)
	suite.Equal("greeting", key.Key)
}

// TestSetOverridesSpanMultipleNamespaces asserts one write can populate more than one namespace, and
// that a namespace-filtered resolve returns only the requested namespace.
func (suite *I18nMgtAPITestSuite) TestSetOverridesSpanMultipleNamespaces() {
	otherNamespace := testNamespace + "-other"
	suite.setOverrides(testLanguage, map[string]map[string]string{
		testNamespace:  {"greeting": "Bonjour"},
		otherNamespace: {"greeting": "Coucou"},
	})

	filtered := suite.resolveLanguage(testLanguage, testNamespace)
	suite.Equal(map[string]string{"greeting": "Bonjour"}, filtered.Translations[testNamespace])
	suite.NotContains(filtered.Translations, otherNamespace)

	all := suite.resolveLanguage(testLanguage, "")
	suite.Equal("Bonjour", all.Translations[testNamespace]["greeting"])
	suite.Equal("Coucou", all.Translations[otherNamespace]["greeting"])
}

// TestSetOverridesReplacesLanguage asserts a write replaces every existing override for the language
// rather than merging into it.
func (suite *I18nMgtAPITestSuite) TestSetOverridesReplacesLanguage() {
	suite.setOverrides(testLanguage, map[string]map[string]string{
		testNamespace: {"greeting": "Bonjour", "farewell": "Au revoir"},
	})

	suite.setOverrides(testLanguage, map[string]map[string]string{
		testNamespace: {"greeting": "Salut"},
	})

	resolved := suite.resolveLanguage(testLanguage, testNamespace)
	suite.Equal(map[string]string{"greeting": "Salut"}, resolved.Translations[testNamespace])
	suite.Equal(http.StatusNotFound, suite.resolveKeyStatus(testLanguage, testNamespace, "farewell"))
}

func (suite *I18nMgtAPITestSuite) TestDeleteLanguageRemovesAllOverrides() {
	suite.setOverrides(testLanguage, map[string]map[string]string{
		testNamespace: {"greeting": "Bonjour", "farewell": "Au revoir"},
	})

	suite.clearLanguage(testLanguage)

	resolved := suite.resolveLanguage(testLanguage, testNamespace)
	suite.Empty(resolved.Translations[testNamespace])
	suite.Equal(http.StatusNotFound, suite.resolveKeyStatus(testLanguage, testNamespace, "greeting"))
}

// TestDeleteLanguageIsIdempotent asserts deleting a language with no overrides is not an error.
func (suite *I18nMgtAPITestSuite) TestDeleteLanguageIsIdempotent() {
	suite.clearLanguage(testLanguage)
	suite.clearLanguage(testLanguage)
}

// TestDeleteLanguageLeavesOtherLanguagesIntact pins that the delete is scoped by language: clearing
// one language leaves another language's overrides for the same key in place.
//
// Note the cleared language does not then resolve to 404 for that key. Resolution is BCP 47
// best-match over whatever translations remain, so fr-CA falls back to the surviving de-DE entry
// rather than reporting the key as missing. The bulk resolve below is the language-scoped view that
// actually shows the override is gone.
func (suite *I18nMgtAPITestSuite) TestDeleteLanguageLeavesOtherLanguagesIntact() {
	otherLanguage := "de-DE"
	suite.setOverrides(testLanguage, map[string]map[string]string{testNamespace: {"greeting": "Bonjour"}})
	suite.setOverrides(otherLanguage, map[string]map[string]string{testNamespace: {"greeting": "Hallo"}})
	defer suite.clearLanguage(otherLanguage)

	suite.clearLanguage(testLanguage)

	suite.Equal("Hallo", suite.resolveKey(otherLanguage, testNamespace, "greeting").Value)

	// The cleared language no longer has its own value; what remains is the de-DE fallback.
	suite.Equal("Hallo", suite.resolveKey(testLanguage, testNamespace, "greeting").Value)
	suite.Equal(map[string]string{"greeting": "Hallo"},
		suite.resolveLanguage(testLanguage, testNamespace).Translations[testNamespace])
}

// --- Overrides interact with system defaults ---

// TestOverrideShadowsSystemDefault asserts an override of a system-namespace key wins over the
// compiled-in default the language otherwise falls back to, and that clearing the override restores
// the default. It overrides the suite's language rather than the system language, because a write
// replaces every override of its language.
func (suite *I18nMgtAPITestSuite) TestOverrideShadowsSystemDefault() {
	key := suite.anySystemDefaultKey()
	defaultValue := suite.resolveKey(testLanguage, systemNamespace, key).Value
	suite.Require().NotEmpty(defaultValue)

	override := defaultValue + " (overridden)"
	suite.setOverrides(testLanguage, map[string]map[string]string{systemNamespace: {key: override}})

	suite.Equal(override, suite.resolveKey(testLanguage, systemNamespace, key).Value)

	suite.clearLanguage(testLanguage)
	suite.Equal(defaultValue, suite.resolveKey(testLanguage, systemNamespace, key).Value)
}

// --- Validation branches ---

func (suite *I18nMgtAPITestSuite) TestSetOverridesValidation() {
	tests := []struct {
		name         string
		language     string
		translations map[string]map[string]string
		wantCode     string
	}{
		{"EmptyTranslations", testLanguage, map[string]map[string]string{}, "I18N-1008"},
		{"InvalidLanguage", "not_a_language",
			map[string]map[string]string{testNamespace: {"greeting": "Bonjour"}}, "I18N-1001"},
		// ValidateLanguage requires the canonical BCP 47 form; "fr-ca" is parseable but not canonical.
		{"NonCanonicalLanguage", "fr-ca",
			map[string]map[string]string{testNamespace: {"greeting": "Bonjour"}}, "I18N-1001"},
		// Namespaces allow only alphanumerics, underscores and hyphens; a dot is rejected.
		{"InvalidNamespace", testLanguage,
			map[string]map[string]string{"bad.namespace": {"greeting": "Bonjour"}}, "I18N-1002"},
		// Keys allow only alphanumerics, dots, underscores and hyphens; a colon is rejected.
		{"InvalidKey", testLanguage, map[string]map[string]string{testNamespace: {"bad:key": "Bonjour"}}, "I18N-1003"},
		{"EmptyValue", testLanguage, map[string]map[string]string{testNamespace: {"greeting": ""}}, "I18N-1005"},
	}
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			item := suite.importOverrides(tc.language, tc.translations)

			suite.Equal("failed", item.Status)
			suite.Equal(tc.wantCode, item.Code)
		})
	}

	// Validation runs over the whole payload before any write.
	suite.Empty(suite.resolveLanguage(testLanguage, "").Translations["bad.namespace"])
}

// TestSetOverridesRejectsPayloadWithOneBadEntry asserts a single invalid entry rejects the whole
// payload; no partial write survives.
func (suite *I18nMgtAPITestSuite) TestSetOverridesRejectsPayloadWithOneBadEntry() {
	item := suite.importOverrides(testLanguage, map[string]map[string]string{
		testNamespace: {"greeting": "Bonjour", "bad:key": "Salut"},
	})

	suite.Equal("failed", item.Status)
	suite.Equal("I18N-1003", item.Code)
	suite.Equal(http.StatusNotFound, suite.resolveKeyStatus(testLanguage, testNamespace, "greeting"))
}

func (suite *I18nMgtAPITestSuite) TestDeleteLanguageInvalidLanguageFails() {
	item, err := testutils.DeleteImportedResourceOutcome("translation", "not_a_language")
	suite.Require().NoError(err)

	suite.Equal("failed", item.Status)
	suite.Equal("I18N-1001", item.Code)
}

func (suite *I18nMgtAPITestSuite) TestMalformedBodyReturns400() {
	for _, target := range []string{translationsURL(testLanguage), keyURL(testLanguage, testNamespace, "greeting")} {
		status, body := suite.do(suite.adminClient, http.MethodPost, target, []byte(`{`))

		suite.Equal(http.StatusBadRequest, status)
		suite.Equal("I18N-1007", suite.errorCode(body))
	}
}

// --- Resolve branches ---

func (suite *I18nMgtAPITestSuite) TestResolveKeyNotFoundReturns404() {
	status, body := suite.do(suite.adminClient, http.MethodGet,
		resolveKeyURL(testLanguage, testNamespace, "never-written"), nil)

	suite.Equal(http.StatusNotFound, status)
	suite.Equal("I18N-1006", suite.errorCode(body))
}

func (suite *I18nMgtAPITestSuite) TestResolveKeyInvalidNamespaceReturns400() {
	status, body := suite.do(suite.adminClient, http.MethodGet,
		resolveKeyURL(testLanguage, "bad.namespace", "greeting"), nil)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1002", suite.errorCode(body))
}

func (suite *I18nMgtAPITestSuite) TestResolveLanguageInvalidNamespaceReturns400() {
	status, body := suite.do(suite.adminClient, http.MethodGet,
		resolveURL(testLanguage)+"?namespace=bad.namespace", nil)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1002", suite.errorCode(body))
}

// TestResolveLanguageFallsBackToSystemLanguage asserts a language with no overrides still resolves the
// system defaults rather than returning an empty document.
func (suite *I18nMgtAPITestSuite) TestResolveLanguageFallsBackToSystemLanguage() {
	resolved := suite.resolveLanguage(testLanguage, systemNamespace)

	suite.Equal(testLanguage, resolved.Language)
	suite.NotEmpty(resolved.Translations[systemNamespace])
}

// --- Gates: resolve is public; writes take no token on a data plane ---

func (suite *I18nMgtAPITestSuite) TestResolveEndpointsArePublic() {
	suite.setOverrides(testLanguage, map[string]map[string]string{testNamespace: {"greeting": "Bonjour"}})

	keyStatus, _ := suite.do(suite.plainClient, http.MethodGet,
		resolveKeyURL(testLanguage, testNamespace, "greeting"), nil)
	suite.Equal(http.StatusOK, keyStatus)

	bulkStatus, _ := suite.do(suite.plainClient, http.MethodGet, resolveURL(testLanguage), nil)
	suite.Equal(http.StatusOK, bulkStatus)
}

// TestWriteEndpointsRefuseTokens asserts a data plane takes no translation write from a token, whatever
// its scopes, and that an unauthenticated write is rejected before that.
func (suite *I18nMgtAPITestSuite) TestWriteEndpointsRefuseTokens() {
	suite.setOverrides(testLanguage, map[string]map[string]string{testNamespace: {"greeting": "Bonjour"}})

	writes := []struct {
		method string
		target string
		body   []byte
	}{
		{http.MethodPost, keyURL(testLanguage, testNamespace, "greeting"), setKeyBody("Salut")},
		{http.MethodDelete, keyURL(testLanguage, testNamespace, "greeting"), nil},
		{http.MethodPost, translationsURL(testLanguage),
			setBulkBody(map[string]map[string]string{testNamespace: {"greeting": "Salut"}})},
		{http.MethodDelete, translationsURL(testLanguage), nil},
	}
	for _, write := range writes {
		status, body := suite.do(suite.adminClient, write.method, write.target, write.body)
		suite.Equal(http.StatusForbidden, status, "%s %s", write.method, write.target)
		suite.Equal("SSE-4030", suite.errorCode(body))

		status, body = suite.do(suite.plainClient, write.method, write.target, write.body)
		suite.Equal(http.StatusUnauthorized, status, "%s %s", write.method, write.target)
		suite.Equal("AUTH-4010", suite.errorCode(body))
	}

	// The refused writes left the override as it was.
	suite.Equal("Bonjour", suite.resolveKey(testLanguage, testNamespace, "greeting").Value)
}

// --- helpers ---

func translationsURL(language string) string {
	return languagesURL + "/" + url.PathEscape(language) + "/translations"
}

func resolveURL(language string) string {
	return translationsURL(language) + "/resolve"
}

func keyURL(language, namespace, key string) string {
	return translationsURL(language) + "/ns/" + url.PathEscape(namespace) + "/keys/" + url.PathEscape(key)
}

func resolveKeyURL(language, namespace, key string) string {
	return keyURL(language, namespace, key) + "/resolve"
}

func setKeyBody(value string) []byte {
	body, err := json.Marshal(setTranslationRequest{Value: value})
	if err != nil {
		panic(err)
	}
	return body
}

func setBulkBody(translations map[string]map[string]string) []byte {
	body, err := json.Marshal(setTranslationsRequest{Translations: translations})
	if err != nil {
		panic(err)
	}
	return body
}

// importOverrides replaces a language's overrides through an import with the management API key and
// returns the outcome, whether it succeeded or not.
func (suite *I18nMgtAPITestSuite) importOverrides(
	language string, translations map[string]map[string]string,
) testutils.ImportItem {
	item, err := testutils.ImportResourceOutcome("translation",
		map[string]any{"language": language, "translations": translations}, true)
	suite.Require().NoError(err)
	return item
}

// setOverrides replaces a language's overrides through an import and requires it to succeed.
func (suite *I18nMgtAPITestSuite) setOverrides(language string, translations map[string]map[string]string) {
	item := suite.importOverrides(language, translations)
	suite.Require().Equal("success", item.Status, "set overrides failed with %s: %s", item.Code, item.Message)
}

// resolveKey resolves a single translation and requires a 200 response.
func (suite *I18nMgtAPITestSuite) resolveKey(language, namespace, key string) translationResponse {
	status, body := suite.do(suite.adminClient, http.MethodGet,
		resolveKeyURL(language, namespace, key), nil)
	suite.Require().Equal(http.StatusOK, status, "resolve failed: %s", body)

	var resp translationResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	return resp
}

// resolveKeyStatus returns only the status code of a single-key resolve, for absence assertions.
func (suite *I18nMgtAPITestSuite) resolveKeyStatus(language, namespace, key string) int {
	status, _ := suite.do(suite.adminClient, http.MethodGet,
		resolveKeyURL(language, namespace, key), nil)
	return status
}

// resolveLanguage resolves all translations for a language, optionally filtered by namespace.
func (suite *I18nMgtAPITestSuite) resolveLanguage(
	language, namespace string,
) languageTranslationsResponse {
	target := resolveURL(language)
	if namespace != "" {
		target += "?namespace=" + url.QueryEscape(namespace)
	}

	status, body := suite.do(suite.adminClient, http.MethodGet, target, nil)
	suite.Require().Equal(http.StatusOK, status, "resolve language failed: %s", body)

	var resp languageTranslationsResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	return resp
}

func (suite *I18nMgtAPITestSuite) listLanguages() []string {
	status, body := suite.do(suite.adminClient, http.MethodGet, languagesURL, nil)
	suite.Require().Equal(http.StatusOK, status)

	var resp languageListResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	return resp.Languages
}

// clearLanguage removes every override for a language through an import deletion.
func (suite *I18nMgtAPITestSuite) clearLanguage(language string) {
	suite.Require().NoError(testutils.DeleteImportedResource("translation", language))
}

// anySystemDefaultKey returns a key that the system namespace resolves a compiled-in default for, so
// the override-shadows-default test does not hard-code a key that may be renamed.
func (suite *I18nMgtAPITestSuite) anySystemDefaultKey() string {
	resolved := suite.resolveLanguage(systemLanguage, systemNamespace)
	defaults := resolved.Translations[systemNamespace]
	suite.Require().NotEmpty(defaults, "expected system namespace to carry default translations")

	// Map iteration order is random; sort for a deterministic pick.
	keys := make([]string, 0, len(defaults))
	for key := range defaults {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys[0]
}

// do issues a request and returns the status code and response body.
func (suite *I18nMgtAPITestSuite) do(
	client *http.Client, method, target string, body []byte,
) (int, []byte) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, target, reader)
	suite.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	respBody, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, respBody
}

// errorCode decodes the error code from an API error response body.
func (suite *I18nMgtAPITestSuite) errorCode(body []byte) string {
	var errResp apiErrorResponse
	suite.Require().NoError(json.Unmarshal(body, &errResp), "body: %s", body)
	return errResp.Code
}

func closeBodyQuietly(t *testing.T, body io.ReadCloser) {
	if body != nil {
		if err := body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}
}
