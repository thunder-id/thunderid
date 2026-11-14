// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

// A field set on top wins, objects set in both are laid over each other, and a list or a scalar replaces
// the base's whole. A field set to null stays unset.
func (suite *UtilsTestSuite) TestOverlayFields() {
	base := map[string]any{
		"kept":     "base",
		"replaced": "base",
		"list":     []any{"a", "b"},
		"nested":   map[string]any{"x": 1, "y": 2},
		"nulled":   "base",
	}
	top := map[string]any{
		"replaced": "top",
		"list":     []any{"c"},
		"nested":   map[string]any{"y": 3},
		"nulled":   nil,
		"added":    true,
	}

	merged := overlayFields(base, top)

	suite.Equal(map[string]any{
		"kept":     "base",
		"replaced": "top",
		"list":     []any{"c"},
		"nested":   map[string]any{"x": 1, "y": 3},
		"nulled":   "base",
		"added":    true,
	}, merged)
	suite.Equal(map[string]any{"x": 1, "y": 2}, base["nested"], "the base is not changed")
	suite.Equal(map[string]any{"a": 1}, overlayFields(nil, map[string]any{"a": 1}))
}

// A section without a default takes its configured value as it is, and one with a default lays the
// configured value over it.
func (suite *UtilsTestSuite) TestEffectiveValue() {
	configured, err := effectiveValue(objectSection(suite.T(), nil), map[string]any{}, map[string]any{"a": "b"})
	suite.Require().NoError(err)
	suite.Equal(map[string]any{"a": "b"}, configured)

	handler := objectSection(suite.T(), map[string]any{"a": "default", "b": "default"})
	merged, err := effectiveValue(handler, map[string]any{}, map[string]any{"b": "set"})
	suite.Require().NoError(err)
	suite.Equal(map[string]any{"a": "default", "b": "set"}, merged)
}

// A number keeps every digit on its way through the overlay, so a section's integer field reads back as
// it was set.
func (suite *UtilsTestSuite) TestJSONObjectKeepsNumbersExact() {
	fields, err := jsonObject(map[string]any{"large": int64(9007199254740993)})
	suite.Require().NoError(err)
	suite.Equal(json.Number("9007199254740993"), fields["large"])
}

// A value that is not a JSON object can't be laid over a default.
func (suite *UtilsTestSuite) TestEffectiveValueRefusesAValueThatIsNotAnObject() {
	_, err := effectiveValue(objectSection(suite.T(), map[string]any{"a": 1}), map[string]any{}, []any{"a"})
	suite.Error(err)
}
