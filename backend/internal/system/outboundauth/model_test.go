// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ModelTestSuite struct {
	suite.Suite
}

func TestModelTestSuite(t *testing.T) {
	suite.Run(t, new(ModelTestSuite))
}

func (s *ModelTestSuite) TestParseType() {
	cases := []struct {
		value    string
		expected Type
		ok       bool
	}{
		{"basic", TypeBasic, true},
		{"BASIC", TypeBasic, true},
		{"  Basic  ", TypeBasic, true},
		{"none", TypeNone, true},
		{"", TypeNone, true},
		{"   ", TypeNone, true},
		{"bearer", TypeBearer, true},
		{"oauth2", "", false},
	}

	for _, tc := range cases {
		parsed, ok := ParseType(tc.value)
		s.Equal(tc.ok, ok, "value %q", tc.value)
		s.Equal(tc.expected, parsed, "value %q", tc.value)
	}
}

// ParseType reads the registry rather than a list of its own, so a method added to the table is
// parsable without a second edit.
func (s *ModelTestSuite) TestParseTypeFollowsRegistry() {
	const apiKey Type = "custom_api_key"

	_, ok := ParseType(string(apiKey))
	s.False(ok)

	withTemporaryMethod(Method{Type: apiKey, DisplayName: "API key"}, func() {
		parsed, ok := ParseType("  CUSTOM_API_KEY  ")
		s.True(ok)
		s.Equal(apiKey, parsed)
	})
}

func (s *ModelTestSuite) TestConfigEnabled() {
	s.False(Config{}.Enabled())
	s.False(Config{Type: TypeNone}.Enabled())
	s.True(basicConfig().Enabled())
}

// recordingTarget is a stand-in transport target that records which method touched it.
