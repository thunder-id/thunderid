// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resolve

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const designTestTheme = `{
	"defaultColorScheme": "dark",
	"colorSchemes": {
		"light": {"palette": {"primary": {"main": "#fa7b3f"}, "contrastThreshold": 3, "dark": false}},
		"dark":  {"palette": {"primary": {"main": "#bb86fc"}, "dark": true}}
	}
}`

const designFullTheme = `{
	"defaultColorScheme": "dark",
	"shape": {"borderRadius": 12},
	"typography": {"fontFamily": "'Inter', sans-serif"},
	"colorSchemes": {
		"light": {"palette": {"primary": {"main": "#fa7b3f"}}},
		"dark":  {"palette": {"primary": {"main": "#bb86fc"}}}
	}
}`

// Test Suite
type ResolveUtilsTestSuite struct {
	suite.Suite
}

func TestResolveUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(ResolveUtilsTestSuite))
}

// resolveAndFlatten resolves the applicable theme and flattens it, mirroring resolveDesignTokens
// without the GetTheme hop, so the pure helpers are exercised together with json.Number decoding.
func (suite *ResolveUtilsTestSuite) resolveAndFlatten(themeJSON, scheme string) map[string]string {
	applicable, err := resolveApplicableTheme(json.RawMessage(themeJSON), scheme)
	suite.Require().Nil(err)
	return flattenTokens("", applicable)
}

// An explicit scheme is flattened, with numbers and booleans kept in their authored form.
func (suite *ResolveUtilsTestSuite) TestResolveApplicableTheme_ExplicitScheme() {
	tokens := suite.resolveAndFlatten(designTestTheme, "light")

	assert.Equal(suite.T(), "#fa7b3f", tokens["palette.primary.main"])
	assert.Equal(suite.T(), "3", tokens["palette.contrastThreshold"])
	assert.Equal(suite.T(), "false", tokens["palette.dark"])
	_, ok := tokens["palette.missing"]
	assert.False(suite.T(), ok)
}

// An empty scheme falls back to the theme's defaultColorScheme.
func (suite *ResolveUtilsTestSuite) TestResolveApplicableTheme_EmptySchemeUsesDefault() {
	tokens := suite.resolveAndFlatten(designTestTheme, "")

	assert.Equal(suite.T(), "#bb86fc", tokens["palette.primary.main"])
	assert.Equal(suite.T(), "true", tokens["palette.dark"])
}

// A requested scheme the theme does not define falls back to defaultColorScheme.
func (suite *ResolveUtilsTestSuite) TestResolveApplicableTheme_UnknownSchemeFallsBackToDefault() {
	tokens := suite.resolveAndFlatten(designTestTheme, "sepia")

	assert.Equal(suite.T(), "#bb86fc", tokens["palette.primary.main"])
}

// The applicable scheme is promoted to the root namespace and root-level values (typography, shape) are
// addressable, while the resolution inputs and the non-selected schemes are not exposed as tokens.
func (suite *ResolveUtilsTestSuite) TestResolveApplicableTheme_PromotesSchemeAndExposesRootTokens() {
	tokens := suite.resolveAndFlatten(designFullTheme, "light")

	assert.Equal(suite.T(), "#fa7b3f", tokens["palette.primary.main"])
	assert.Equal(suite.T(), "'Inter', sans-serif", tokens["typography.fontFamily"])
	assert.Equal(suite.T(), "12", tokens["shape.borderRadius"])

	_, hasDefault := tokens["defaultColorScheme"]
	assert.False(suite.T(), hasDefault)
	_, hasSchemePath := tokens["colorSchemes.dark.palette.primary.main"]
	assert.False(suite.T(), hasSchemePath)
}

// Array and null leaves are skipped; sibling scalars still resolve.
func (suite *ResolveUtilsTestSuite) TestResolveApplicableTheme_SkipsArrayAndNullLeaves() {
	theme := `{"defaultColorScheme": "light", "colorSchemes": {"light": {"palette": {` +
		`"main": "#fff", "shadows": ["a", "b"], "divider": null}}}}`

	tokens := suite.resolveAndFlatten(theme, "light")

	assert.Equal(suite.T(), "#fff", tokens["palette.main"])
	_, hasArray := tokens["palette.shadows"]
	assert.False(suite.T(), hasArray)
	_, hasNull := tokens["palette.divider"]
	assert.False(suite.T(), hasNull)
}

// A theme without a colorSchemes object returns an error, not an empty scheme.
func (suite *ResolveUtilsTestSuite) TestResolveApplicableTheme_NoColorSchemes() {
	_, err := resolveApplicableTheme(json.RawMessage(`{"foo": "bar"}`), "light")

	assert.NotNil(suite.T(), err)
	assert.ErrorIs(suite.T(), err, errNoApplicableColorScheme)
}

// A missing requested scheme with no usable default returns an error.
func (suite *ResolveUtilsTestSuite) TestResolveApplicableTheme_NoUsableDefault() {
	_, err := resolveApplicableTheme(
		json.RawMessage(`{"defaultColorScheme": "dark", "colorSchemes": {"light": {"palette": {}}}}`), "sepia")

	assert.NotNil(suite.T(), err)
	assert.ErrorIs(suite.T(), err, errNoApplicableColorScheme)
}

// An empty theme body returns the empty-theme error rather than an empty scheme.
func (suite *ResolveUtilsTestSuite) TestResolveApplicableTheme_EmptyTheme() {
	_, err := resolveApplicableTheme(nil, "light")

	assert.NotNil(suite.T(), err)
	assert.ErrorIs(suite.T(), err, errEmptyTheme)
}

// Malformed theme JSON returns the malformed-theme error.
func (suite *ResolveUtilsTestSuite) TestResolveApplicableTheme_InvalidJSON() {
	_, err := resolveApplicableTheme(json.RawMessage(`{`), "light")

	assert.NotNil(suite.T(), err)
	assert.ErrorIs(suite.T(), err, errMalformedTheme)
}

// Trailing content after the first JSON value returns the malformed-theme error.
func (suite *ResolveUtilsTestSuite) TestResolveApplicableTheme_TrailingContent() {
	theme := `{"defaultColorScheme": "light", "colorSchemes": {"light": {"palette": {}}}} garbage`

	_, err := resolveApplicableTheme(json.RawMessage(theme), "light")

	assert.NotNil(suite.T(), err)
	assert.ErrorIs(suite.T(), err, errMalformedTheme)
}

// substituteDesignTokens replaces known placeholders with HTML-escaped values, leaves unknown ones
// untouched, and returns content without placeholders unchanged.
func (suite *ResolveUtilsTestSuite) TestSubstituteDesignTokens() {
	tokens := map[string]string{"palette.primary.main": "#fa7b3f"}

	out := substituteDesignTokens("color: {{design(palette.primary.main)}};", tokens)
	assert.Equal(suite.T(), "color: #fa7b3f;", out)

	out = substituteDesignTokens("{{design(palette.missing)}}", tokens)
	assert.Equal(suite.T(), "{{design(palette.missing)}}", out)

	out = substituteDesignTokens("no tokens here", tokens)
	assert.Equal(suite.T(), "no tokens here", out)

	// Repeated placeholders are all substituted.
	out = substituteDesignTokens(
		"{{design(palette.primary.main)}} and {{design(palette.primary.main)}}", tokens)
	assert.Equal(suite.T(), "#fa7b3f and #fa7b3f", out)

	// Hyphenated token keys are matched and substituted.
	hyphen := map[string]string{"palette.primary.on-hover": "#e86a2f"}
	out = substituteDesignTokens("{{design(palette.primary.on-hover)}}", hyphen)
	assert.Equal(suite.T(), "#e86a2f", out)

	// A quote in a value is HTML-escaped so it cannot break out of the attribute it lands in.
	raw := map[string]string{"x": `red" onmouseover="x`}
	out = substituteDesignTokens(`<a title="{{design(x)}}">`, raw)
	assert.Equal(suite.T(), `<a title="red&#34; onmouseover=&#34;x">`, out)

	// Angle brackets in a value are escaped, so it renders as text rather than injecting markup.
	markup := map[string]string{"x": "<b>injected</b>"}
	out = substituteDesignTokens("{{design(x)}}", markup)
	assert.Equal(suite.T(), "&lt;b&gt;injected&lt;/b&gt;", out)

	// Substitution is a single pass: a value that itself looks like a placeholder is not re-expanded.
	nested := map[string]string{"x": "{{design(y)}}", "y": "#fff"}
	out = substituteDesignTokens("{{design(x)}}", nested)
	assert.Equal(suite.T(), "{{design(y)}}", out)
}
