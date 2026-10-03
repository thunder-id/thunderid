// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thememgt

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

const designTokenTestTheme = `{
	"defaultColorScheme": "dark",
	"colorSchemes": {
		"light": {"palette": {"primary": {"main": "#fa7b3f"}, "contrastThreshold": 3, "dark": false}},
		"dark":  {"palette": {"primary": {"main": "#bb86fc"}, "dark": true}}
	}
}`

func TestFlattenDesignTokens(t *testing.T) {
	// Explicit scheme selected.
	light := flattenDesignTokens(json.RawMessage(designTokenTestTheme), "light")
	require.Equal(t, "#fa7b3f", light["palette.primary.main"])
	// Numbers keep their authored form.
	require.Equal(t, "3", light["palette.contrastThreshold"])
	// Booleans are encoded as "true"/"false".
	require.Equal(t, "false", light["palette.dark"])

	// Empty scheme falls back to defaultColorScheme (dark).
	def := flattenDesignTokens(json.RawMessage(designTokenTestTheme), "")
	require.Equal(t, "#bb86fc", def["palette.primary.main"])
	require.Equal(t, "true", def["palette.dark"])

	// Unknown scheme and unparseable JSON yield an empty map.
	require.Empty(t, flattenDesignTokens(json.RawMessage(designTokenTestTheme), "sepia"))
	require.Empty(t, flattenDesignTokens(json.RawMessage(`{`), "light"))
	require.Empty(t, flattenDesignTokens(nil, "light"))
}

func (suite *ThemeServiceTestSuite) TestResolveDesignTokens() {
	suite.mockStore.On("GetTheme", testCtx(), "t1").Return(
		Theme{ID: "t1", Theme: json.RawMessage(designTokenTestTheme)}, nil)

	// Returns the flattened token map for the selected scheme.
	tokens, err := suite.service.ResolveDesignTokens(testCtx(), "t1", "light")
	suite.Require().Nil(err)
	suite.Require().Equal("#fa7b3f", tokens["palette.primary.main"])
	suite.Require().Equal("3", tokens["palette.contrastThreshold"])
	_, ok := tokens["palette.missing"]
	suite.Require().False(ok)
}

func (suite *ThemeServiceTestSuite) TestResolveDesignTokens_ThemeNotFound() {
	suite.mockStore.On("GetTheme", testCtx(), "missing").Return(Theme{}, errThemeNotFound)

	tokens, err := suite.service.ResolveDesignTokens(testCtx(), "missing", "light")
	suite.Require().Nil(tokens)
	suite.Require().Equal(ErrorThemeNotFound.Code, err.Code)
}
