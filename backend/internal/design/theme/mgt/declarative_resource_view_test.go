// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thememgt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func viewThemeDocument(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("resource_type: theme\n"+content), &root))
	return root.Content[0]
}

// An exported theme, its configuration written as a JSON string, is shown as a read returns it,
// with its references as they are.
func TestViewResourceShowsAnExportedThemeAsItsRead(t *testing.T) {
	exported, err := yaml.Marshal(map[string]interface{}{
		"id": "theme-1", "handle": "classic", "displayName": "Classic",
		"description": "var:THEME_CLASSIC_DESCRIPTION", "theme": `{"a":1}`,
		"createdAt": "2026-01-01", "updatedAt": "2026-01-02", "isReadOnly": true,
	})
	require.NoError(t, err)

	view, err := newThemeExporter(nil).ViewResource(context.Background(), viewThemeDocument(t, string(exported)))

	require.NoError(t, err)
	theme, ok := view.(Theme)
	require.True(t, ok)
	assert.Equal(t, "theme-1", theme.ID)
	assert.Equal(t, "classic", theme.Handle)
	assert.Equal(t, "Classic", theme.DisplayName)
	assert.Equal(t, "var:THEME_CLASSIC_DESCRIPTION", theme.Description)
	assert.JSONEq(t, `{"a":1}`, string(theme.Theme))
	assert.Equal(t, "2026-01-01", theme.CreatedAt)
	assert.Equal(t, "2026-01-02", theme.UpdatedAt)
	assert.False(t, theme.IsReadOnly)
}

// A theme written as a map structure is shown with that structure as JSON.
func TestViewResourceShowsAThemeMapAsJSON(t *testing.T) {
	view, err := newThemeExporter(nil).ViewResource(context.Background(),
		viewThemeDocument(t, "id: theme-1\ndisplayName: Classic\ntheme:\n  a: 1\n"))

	require.NoError(t, err)
	theme, ok := view.(Theme)
	require.True(t, ok)
	assert.JSONEq(t, `{"a":1}`, string(theme.Theme))
}

// A document that is not a theme is refused.
func TestViewResourceRefusesAnUnreadableTheme(t *testing.T) {
	exporter := newThemeExporter(nil)

	_, err := exporter.ViewResource(context.Background(), viewThemeDocument(t, "displayName: [not, a, name]"))
	assert.Error(t, err)
}
