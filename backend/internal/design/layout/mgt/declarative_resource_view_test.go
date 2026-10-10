// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package layoutmgt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func viewLayoutDocument(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("resource_type: layout\n"+content), &root))
	return root.Content[0]
}

// An exported layout, its configuration written as a JSON string, is shown as a read returns it,
// with its references as they are.
func TestViewResourceShowsAnExportedLayoutAsItsRead(t *testing.T) {
	exported, err := yaml.Marshal(map[string]interface{}{
		"id": "layout-1", "handle": "classic", "displayName": "Classic",
		"description": "var:LAYOUT_CLASSIC_DESCRIPTION", "layout": `{"a":1}`,
		"createdAt": "2026-01-01", "updatedAt": "2026-01-02", "isReadOnly": true,
	})
	require.NoError(t, err)

	view, err := newLayoutExporter(nil).ViewResource(context.Background(), viewLayoutDocument(t, string(exported)))

	require.NoError(t, err)
	layout, ok := view.(Layout)
	require.True(t, ok)
	assert.Equal(t, "layout-1", layout.ID)
	assert.Equal(t, "classic", layout.Handle)
	assert.Equal(t, "Classic", layout.DisplayName)
	assert.Equal(t, "var:LAYOUT_CLASSIC_DESCRIPTION", layout.Description)
	assert.JSONEq(t, `{"a":1}`, string(layout.Layout))
	assert.Equal(t, "2026-01-01", layout.CreatedAt)
	assert.Equal(t, "2026-01-02", layout.UpdatedAt)
	assert.True(t, layout.IsReadOnly)
}

// A layout written as a map structure is shown with that structure as JSON.
func TestViewResourceShowsALayoutMapAsJSON(t *testing.T) {
	view, err := newLayoutExporter(nil).ViewResource(context.Background(),
		viewLayoutDocument(t, "id: layout-1\ndisplayName: Classic\nlayout:\n  a: 1\n"))

	require.NoError(t, err)
	layout, ok := view.(Layout)
	require.True(t, ok)
	assert.JSONEq(t, `{"a":1}`, string(layout.Layout))
}

// A document that is not a layout is refused.
func TestViewResourceRefusesAnUnreadableLayout(t *testing.T) {
	exporter := newLayoutExporter(nil)

	_, err := exporter.ViewResource(context.Background(), viewLayoutDocument(t, "displayName: [not, a, name]"))
	assert.Error(t, err)
}
