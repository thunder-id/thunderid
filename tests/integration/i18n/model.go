// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package i18n

import "github.com/thunder-id/thunderid/tests/integration/testutils"

const (
	testServerURL = testutils.TestServerURL
	languagesURL  = testServerURL + "/i18n/languages"

	// systemLanguage / systemNamespace mirror the server defaults
	// (backend/internal/system/i18n/mgt/constants.go).
	systemLanguage  = "en-US"
	systemNamespace = "system"

	// testLanguage is the language the management tests write to. It is deliberately not the system
	// language so clearing it never removes system defaults other suites resolve against.
	testLanguage = "fr-CA"

	// testNamespace is the namespace owned by this suite; TearDown clears it.
	testNamespace = "integration-i18n"
)

// i18nMessage mirrors the i18n message structure returned in API error responses.
type i18nMessage struct {
	Key          string `json:"key"`
	DefaultValue string `json:"defaultValue"`
}

// apiErrorResponse mirrors apierror.ErrorResponse for decoding error responses.
type apiErrorResponse struct {
	Code        string      `json:"code"`
	Message     i18nMessage `json:"message"`
	Description i18nMessage `json:"description"`
}

// languageListResponse mirrors mgt.LanguageListResponse.
type languageListResponse struct {
	Languages []string `json:"languages"`
}

// translationResponse mirrors mgt.TranslationResponse, the single-translation payload returned by the
// resolve and set-override endpoints.
type translationResponse struct {
	Language  string `json:"language"`
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
	Value     string `json:"value"`
}

// setTranslationRequest mirrors mgt.SetTranslationRequest.
type setTranslationRequest struct {
	Value string `json:"value"`
}

// setTranslationsRequest mirrors mgt.SetTranslationsRequest; translations is namespace -> key -> value.
type setTranslationsRequest struct {
	Translations map[string]map[string]string `json:"translations"`
}

// languageTranslationsResponse mirrors providers.LanguageTranslationsResponse, the bulk payload
// returned by the bulk resolve and bulk set endpoints.
type languageTranslationsResponse struct {
	Language     string                       `json:"language"`
	TotalResults int                          `json:"totalResults"`
	Translations map[string]map[string]string `json:"translations"`
}
