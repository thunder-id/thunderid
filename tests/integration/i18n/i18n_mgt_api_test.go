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

// I18nMgtAPITestSuite covers the translation *management* endpoints: setting and clearing overrides
// both per-key and in bulk per-language, the round-trip through the resolve endpoints, the validation
// branches for language/namespace/key/value, and the auth gate. The resolve endpoints are public; the
// write endpoints are not (backend/internal/system/security/permissions.go).
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

	suite.setKeyOverride(testLanguage, testNamespace, "greeting", "Bonjour")

	suite.Contains(suite.listLanguages(), testLanguage)
}

func (suite *I18nMgtAPITestSuite) TestListLanguagesIsPublic() {
	status, _ := suite.do(suite.plainClient, http.MethodGet, languagesURL, nil)

	suite.Equal(http.StatusOK, status)
}

// --- Single-key override: POST/GET/DELETE round-trip ---

func (suite *I18nMgtAPITestSuite) TestSetKeyOverrideRoundTrip() {
	setResp := suite.setKeyOverride(testLanguage, testNamespace, "greeting", "Bonjour")
	suite.Equal(testLanguage, setResp.Language)
	suite.Equal(testNamespace, setResp.Namespace)
	suite.Equal("greeting", setResp.Key)
	suite.Equal("Bonjour", setResp.Value)

	resolved := suite.resolveKey(testLanguage, testNamespace, "greeting")
	suite.Equal("Bonjour", resolved.Value)
	suite.Equal(testNamespace, resolved.Namespace)
	suite.Equal("greeting", resolved.Key)
}

func (suite *I18nMgtAPITestSuite) TestSetKeyOverrideIsIdempotentUpsert() {
	suite.setKeyOverride(testLanguage, testNamespace, "greeting", "Bonjour")
	suite.setKeyOverride(testLanguage, testNamespace, "greeting", "Salut")

	// The second write updates in place rather than creating a second row.
	suite.Equal("Salut", suite.resolveKey(testLanguage, testNamespace, "greeting").Value)
}

func (suite *I18nMgtAPITestSuite) TestClearKeyOverrideRemovesOnlyThatKey() {
	suite.setKeyOverride(testLanguage, testNamespace, "greeting", "Bonjour")
	suite.setKeyOverride(testLanguage, testNamespace, "farewell", "Au revoir")

	status, _ := suite.do(suite.adminClient, http.MethodDelete,
		keyURL(testLanguage, testNamespace, "greeting"), nil)
	suite.Equal(http.StatusNoContent, status)

	// The cleared key is gone; the sibling key in the same namespace survives.
	suite.Equal(http.StatusNotFound, suite.resolveKeyStatus(testLanguage, testNamespace, "greeting"))
	suite.Equal("Au revoir", suite.resolveKey(testLanguage, testNamespace, "farewell").Value)
}

// TestClearKeyOverrideIsIdempotent asserts deleting an override that does not exist is not an error;
// the store delete is unconditional.
func (suite *I18nMgtAPITestSuite) TestClearKeyOverrideIsIdempotent() {
	status, _ := suite.do(suite.adminClient, http.MethodDelete,
		keyURL(testLanguage, testNamespace, "never-written"), nil)

	suite.Equal(http.StatusNoContent, status)
}

// --- Bulk override: POST/GET/DELETE round-trip ---

func (suite *I18nMgtAPITestSuite) TestSetBulkOverridesRoundTrip() {
	setResp := suite.setBulkOverrides(testLanguage, map[string]map[string]string{
		testNamespace: {"greeting": "Bonjour", "farewell": "Au revoir"},
	})
	suite.Equal(testLanguage, setResp.Language)
	suite.Equal(2, setResp.TotalResults)

	resolved := suite.resolveLanguage(testLanguage, testNamespace)
	suite.Equal(testLanguage, resolved.Language)
	suite.Equal(map[string]string{"greeting": "Bonjour", "farewell": "Au revoir"},
		resolved.Translations[testNamespace])
}

// TestSetBulkOverridesSpanMultipleNamespaces asserts one bulk write can populate more than one
// namespace, and that a namespace-filtered resolve returns only the requested namespace.
func (suite *I18nMgtAPITestSuite) TestSetBulkOverridesSpanMultipleNamespaces() {
	otherNamespace := testNamespace + "-other"
	suite.setBulkOverrides(testLanguage, map[string]map[string]string{
		testNamespace:  {"greeting": "Bonjour"},
		otherNamespace: {"greeting": "Coucou"},
	})
	defer suite.clearNamespaceKey(testLanguage, otherNamespace, "greeting")

	filtered := suite.resolveLanguage(testLanguage, testNamespace)
	suite.Equal(map[string]string{"greeting": "Bonjour"}, filtered.Translations[testNamespace])
	suite.NotContains(filtered.Translations, otherNamespace)

	all := suite.resolveLanguage(testLanguage, "")
	suite.Equal("Bonjour", all.Translations[testNamespace]["greeting"])
	suite.Equal("Coucou", all.Translations[otherNamespace]["greeting"])
}

// TestSetBulkOverridesReplacesLanguage asserts the bulk write replaces every existing override for the
// language rather than merging into it.
func (suite *I18nMgtAPITestSuite) TestSetBulkOverridesReplacesLanguage() {
	suite.setBulkOverrides(testLanguage, map[string]map[string]string{
		testNamespace: {"greeting": "Bonjour", "farewell": "Au revoir"},
	})

	suite.setBulkOverrides(testLanguage, map[string]map[string]string{
		testNamespace: {"greeting": "Salut"},
	})

	resolved := suite.resolveLanguage(testLanguage, testNamespace)
	suite.Equal(map[string]string{"greeting": "Salut"}, resolved.Translations[testNamespace])
	suite.Equal(http.StatusNotFound, suite.resolveKeyStatus(testLanguage, testNamespace, "farewell"))
}

func (suite *I18nMgtAPITestSuite) TestClearLanguageOverridesRemovesAll() {
	suite.setBulkOverrides(testLanguage, map[string]map[string]string{
		testNamespace: {"greeting": "Bonjour", "farewell": "Au revoir"},
	})

	status, _ := suite.do(suite.adminClient, http.MethodDelete, translationsURL(testLanguage), nil)
	suite.Equal(http.StatusNoContent, status)

	resolved := suite.resolveLanguage(testLanguage, testNamespace)
	suite.Empty(resolved.Translations[testNamespace])
	suite.Equal(http.StatusNotFound, suite.resolveKeyStatus(testLanguage, testNamespace, "greeting"))
}

// TestClearLanguageOverridesLeavesOtherLanguagesIntact pins that the delete is scoped by language:
// clearing one language leaves another language's overrides for the same key in place.
//
// Note the cleared language does not then resolve to 404 for that key. Resolution is BCP 47
// best-match over whatever translations remain, so fr-CA falls back to the surviving de-DE entry
// rather than reporting the key as missing. The bulk resolve below is the language-scoped view that
// actually shows the override is gone.
func (suite *I18nMgtAPITestSuite) TestClearLanguageOverridesLeavesOtherLanguagesIntact() {
	otherLanguage := "de-DE"
	suite.setKeyOverride(testLanguage, testNamespace, "greeting", "Bonjour")
	suite.setKeyOverride(otherLanguage, testNamespace, "greeting", "Hallo")
	defer suite.clearLanguage(otherLanguage)

	suite.clearLanguage(testLanguage)

	suite.Equal("Hallo", suite.resolveKey(otherLanguage, testNamespace, "greeting").Value)

	// The cleared language no longer has its own value; what remains is the de-DE fallback.
	suite.Equal("Hallo", suite.resolveKey(testLanguage, testNamespace, "greeting").Value)
	suite.Equal(map[string]string{"greeting": "Hallo"},
		suite.resolveLanguage(testLanguage, testNamespace).Translations[testNamespace])
}

// --- Overrides interact with system defaults ---

// TestOverrideShadowsSystemDefault asserts a system-namespace override wins over the compiled-in
// default for the same key, and that clearing the override restores the default.
func (suite *I18nMgtAPITestSuite) TestOverrideShadowsSystemDefault() {
	key := suite.anySystemDefaultKey()
	defaultValue := suite.resolveKey(systemLanguage, systemNamespace, key).Value
	suite.Require().NotEmpty(defaultValue)

	override := defaultValue + " (overridden)"
	suite.setKeyOverride(systemLanguage, systemNamespace, key, override)

	suite.Equal(override, suite.resolveKey(systemLanguage, systemNamespace, key).Value)

	suite.clearNamespaceKey(systemLanguage, systemNamespace, key)
	suite.Equal(defaultValue, suite.resolveKey(systemLanguage, systemNamespace, key).Value)
}

// --- Validation branches ---

func (suite *I18nMgtAPITestSuite) TestSetKeyOverrideInvalidLanguageReturns400() {
	status, body := suite.do(suite.adminClient, http.MethodPost,
		keyURL("not_a_language", testNamespace, "greeting"), setKeyBody("Bonjour"))

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1001", suite.errorCode(body))
}

func (suite *I18nMgtAPITestSuite) TestSetKeyOverrideNonCanonicalLanguageReturns400() {
	// ValidateLanguage requires the canonical BCP 47 form; "fr-ca" is parseable but not canonical.
	status, body := suite.do(suite.adminClient, http.MethodPost,
		keyURL("fr-ca", testNamespace, "greeting"), setKeyBody("Bonjour"))

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1001", suite.errorCode(body))
}

func (suite *I18nMgtAPITestSuite) TestSetKeyOverrideInvalidNamespaceReturns400() {
	// Namespaces allow only alphanumerics, underscores and hyphens; a dot is rejected.
	status, body := suite.do(suite.adminClient, http.MethodPost,
		keyURL(testLanguage, "bad.namespace", "greeting"), setKeyBody("Bonjour"))

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1002", suite.errorCode(body))
}

func (suite *I18nMgtAPITestSuite) TestSetKeyOverrideInvalidKeyReturns400() {
	// Keys allow only alphanumerics, dots, underscores and hyphens; a colon is rejected.
	status, body := suite.do(suite.adminClient, http.MethodPost,
		keyURL(testLanguage, testNamespace, "bad:key"), setKeyBody("Bonjour"))

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1003", suite.errorCode(body))
}

func (suite *I18nMgtAPITestSuite) TestSetKeyOverrideEmptyValueReturns400() {
	status, body := suite.do(suite.adminClient, http.MethodPost,
		keyURL(testLanguage, testNamespace, "greeting"), setKeyBody(""))

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1005", suite.errorCode(body))
}

func (suite *I18nMgtAPITestSuite) TestSetKeyOverrideMalformedBodyReturns400() {
	status, body := suite.do(suite.adminClient, http.MethodPost,
		keyURL(testLanguage, testNamespace, "greeting"), []byte(`{`))

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1007", suite.errorCode(body))
}

func (suite *I18nMgtAPITestSuite) TestClearKeyOverrideInvalidLanguageReturns400() {
	status, body := suite.do(suite.adminClient, http.MethodDelete,
		keyURL("not_a_language", testNamespace, "greeting"), nil)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1001", suite.errorCode(body))
}

func (suite *I18nMgtAPITestSuite) TestSetBulkOverridesEmptyTranslationsReturns400() {
	status, body := suite.do(suite.adminClient, http.MethodPost,
		translationsURL(testLanguage), setBulkBody(map[string]map[string]string{}))

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1008", suite.errorCode(body))
}

func (suite *I18nMgtAPITestSuite) TestSetBulkOverridesInvalidLanguageReturns400() {
	status, body := suite.do(suite.adminClient, http.MethodPost,
		translationsURL("not_a_language"),
		setBulkBody(map[string]map[string]string{testNamespace: {"greeting": "Bonjour"}}))

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1001", suite.errorCode(body))
}

func (suite *I18nMgtAPITestSuite) TestSetBulkOverridesInvalidNamespaceReturns400AndDoesNotPersist() {
	status, body := suite.do(suite.adminClient, http.MethodPost, translationsURL(testLanguage),
		setBulkBody(map[string]map[string]string{"bad.namespace": {"greeting": "Bonjour"}}))

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1002", suite.errorCode(body))

	// Validation runs over the whole payload before any write.
	suite.Empty(suite.resolveLanguage(testLanguage, "").Translations["bad.namespace"])
}

func (suite *I18nMgtAPITestSuite) TestSetBulkOverridesInvalidKeyReturns400() {
	status, body := suite.do(suite.adminClient, http.MethodPost, translationsURL(testLanguage),
		setBulkBody(map[string]map[string]string{testNamespace: {"bad:key": "Bonjour"}}))

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1003", suite.errorCode(body))
}

func (suite *I18nMgtAPITestSuite) TestSetBulkOverridesEmptyValueReturns400() {
	status, body := suite.do(suite.adminClient, http.MethodPost, translationsURL(testLanguage),
		setBulkBody(map[string]map[string]string{testNamespace: {"greeting": ""}}))

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1005", suite.errorCode(body))
}

// TestSetBulkOverridesRejectsPayloadWithOneBadEntry asserts a single invalid entry rejects the whole
// payload; no partial write survives.
func (suite *I18nMgtAPITestSuite) TestSetBulkOverridesRejectsPayloadWithOneBadEntry() {
	status, body := suite.do(suite.adminClient, http.MethodPost, translationsURL(testLanguage),
		setBulkBody(map[string]map[string]string{
			testNamespace: {"greeting": "Bonjour", "bad:key": "Salut"},
		}))

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1003", suite.errorCode(body))
	suite.Equal(http.StatusNotFound, suite.resolveKeyStatus(testLanguage, testNamespace, "greeting"))
}

func (suite *I18nMgtAPITestSuite) TestSetBulkOverridesMalformedBodyReturns400() {
	status, body := suite.do(suite.adminClient, http.MethodPost,
		translationsURL(testLanguage), []byte(`{`))

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1007", suite.errorCode(body))
}

func (suite *I18nMgtAPITestSuite) TestClearLanguageOverridesInvalidLanguageReturns400() {
	status, body := suite.do(suite.adminClient, http.MethodDelete,
		translationsURL("not_a_language"), nil)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("I18N-1001", suite.errorCode(body))
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

// --- Auth gate: resolve is public, writes are not ---

func (suite *I18nMgtAPITestSuite) TestResolveEndpointsArePublic() {
	suite.setKeyOverride(testLanguage, testNamespace, "greeting", "Bonjour")

	keyStatus, _ := suite.do(suite.plainClient, http.MethodGet,
		resolveKeyURL(testLanguage, testNamespace, "greeting"), nil)
	suite.Equal(http.StatusOK, keyStatus)

	bulkStatus, _ := suite.do(suite.plainClient, http.MethodGet, resolveURL(testLanguage), nil)
	suite.Equal(http.StatusOK, bulkStatus)
}

func (suite *I18nMgtAPITestSuite) TestWriteEndpointsRejectUnauthenticatedRequests() {
	setKeyStatus, setKeyBodyResp := suite.do(suite.plainClient, http.MethodPost,
		keyURL(testLanguage, testNamespace, "greeting"), setKeyBody("Bonjour"))
	suite.Equal(http.StatusUnauthorized, setKeyStatus)
	suite.Equal("AUTH-4010", suite.errorCode(setKeyBodyResp))

	clearKeyStatus, _ := suite.do(suite.plainClient, http.MethodDelete,
		keyURL(testLanguage, testNamespace, "greeting"), nil)
	suite.Equal(http.StatusUnauthorized, clearKeyStatus)

	setBulkStatus, _ := suite.do(suite.plainClient, http.MethodPost, translationsURL(testLanguage),
		setBulkBody(map[string]map[string]string{testNamespace: {"greeting": "Bonjour"}}))
	suite.Equal(http.StatusUnauthorized, setBulkStatus)

	clearBulkStatus, _ := suite.do(suite.plainClient, http.MethodDelete,
		translationsURL(testLanguage), nil)
	suite.Equal(http.StatusUnauthorized, clearBulkStatus)

	// The rejected writes left no trace.
	suite.Equal(http.StatusNotFound, suite.resolveKeyStatus(testLanguage, testNamespace, "greeting"))
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

// setKeyOverride writes a single override and requires a 200 response.
func (suite *I18nMgtAPITestSuite) setKeyOverride(
	language, namespace, key, value string,
) translationResponse {
	status, body := suite.do(suite.adminClient, http.MethodPost,
		keyURL(language, namespace, key), setKeyBody(value))
	suite.Require().Equal(http.StatusOK, status, "set override failed: %s", body)

	var resp translationResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	return resp
}

// setBulkOverrides replaces a language's overrides and requires a 200 response.
func (suite *I18nMgtAPITestSuite) setBulkOverrides(
	language string, translations map[string]map[string]string,
) languageTranslationsResponse {
	status, body := suite.do(suite.adminClient, http.MethodPost,
		translationsURL(language), setBulkBody(translations))
	suite.Require().Equal(http.StatusOK, status, "bulk set failed: %s", body)

	var resp languageTranslationsResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	return resp
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

// clearLanguage removes every override for a language and requires a 204 response.
func (suite *I18nMgtAPITestSuite) clearLanguage(language string) {
	status, body := suite.do(suite.adminClient, http.MethodDelete, translationsURL(language), nil)
	suite.Require().Equal(http.StatusNoContent, status, "clear language failed: %s", body)
}

// clearNamespaceKey removes a single override and requires a 204 response.
func (suite *I18nMgtAPITestSuite) clearNamespaceKey(language, namespace, key string) {
	status, body := suite.do(suite.adminClient, http.MethodDelete,
		keyURL(language, namespace, key), nil)
	suite.Require().Equal(http.StatusNoContent, status, "clear key failed: %s", body)
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
