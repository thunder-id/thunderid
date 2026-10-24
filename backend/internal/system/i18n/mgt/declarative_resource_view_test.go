// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mgt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

func viewTranslationDocument(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("resource_type: translation\n"+content), &root))
	return root.Content[0]
}

func exportedLanguage(t *testing.T) *yaml.Node {
	t.Helper()
	exported, err := yaml.Marshal(&LanguageTranslations{
		Language: "fr",
		Translations: map[string]map[string]string{
			"system":         {"signin.title": "Se connecter", "signin.button": "var:TRANSLATION_FR_SIGNIN_BUTTON"},
			"flowCustomI18n": {"welcome": "Bienvenue"},
		},
	})
	require.NoError(t, err)
	return viewTranslationDocument(t, string(exported))
}

// An exported language is shown as the resolve read returns it, with its references as they are.
func TestViewResourceShowsAnExportedLanguageAsItsRead(t *testing.T) {
	view, err := newTranslationExporter(nil).ViewResource(context.Background(), exportedLanguage(t))

	require.NoError(t, err)
	resp, ok := view.(*providers.LanguageTranslationsResponse)
	require.True(t, ok)
	assert.Equal(t, "fr", resp.Language)
	assert.Equal(t, 3, resp.TotalResults)
	assert.Equal(t, "var:TRANSLATION_FR_SIGNIN_BUTTON", resp.Translations["system"]["signin.button"])
	assert.Equal(t, "Bienvenue", resp.Translations["flowCustomI18n"]["welcome"])
}

// A language has a part ns/{namespace} for each namespace its translations hold, showing only that
// namespace, as the read's namespace filter does.
func TestViewResourcePartsShowsEachNamespace(t *testing.T) {
	parts, err := newTranslationExporter(nil).ViewResourceParts(context.Background(), exportedLanguage(t))

	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"ns/flowCustomI18n": &providers.LanguageTranslationsResponse{
			Language: "fr", TotalResults: 1,
			Translations: map[string]map[string]string{"flowCustomI18n": {"welcome": "Bienvenue"}},
		},
		"ns/system": &providers.LanguageTranslationsResponse{
			Language: "fr", TotalResults: 2,
			Translations: map[string]map[string]string{"system": {
				"signin.title": "Se connecter", "signin.button": "var:TRANSLATION_FR_SIGNIN_BUTTON",
			}},
		},
	}, parts)
}

// A document that is not a language is refused.
func TestViewResourceRefusesAnUnreadableLanguage(t *testing.T) {
	exporter := newTranslationExporter(nil)

	_, err := exporter.ViewResource(context.Background(), viewTranslationDocument(t, "translations: [a, b]"))
	assert.Error(t, err)

	_, err = exporter.ViewResourceParts(context.Background(),
		viewTranslationDocument(t, "translations: [not, a, map]"))
	assert.Error(t, err)
}
