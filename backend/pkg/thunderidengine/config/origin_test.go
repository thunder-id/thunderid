// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

func (suite *ValidateTestSuite) TestOriginEntry_UnmarshalJSON() {
	suite.T().Run("literal string decodes to Origin", func(t *testing.T) {
		var o OriginEntry
		require.NoError(t, json.Unmarshal([]byte(`"https://app.example.com"`), &o))
		assert.Equal(t, OriginEntry{Origin: "https://app.example.com"}, o)
	})

	suite.T().Run("regex object decodes to Regex", func(t *testing.T) {
		var o OriginEntry
		require.NoError(t, json.Unmarshal([]byte(`{"regex":"^https://.*$"}`), &o))
		assert.Equal(t, OriginEntry{Regex: "^https://.*$"}, o)
	})

	suite.T().Run("regex object missing regex field fails", func(t *testing.T) {
		var o OriginEntry
		assert.Error(t, json.Unmarshal([]byte(`{}`), &o))
	})

	suite.T().Run("non-string non-object fails", func(t *testing.T) {
		var o OriginEntry
		assert.Error(t, json.Unmarshal([]byte(`42`), &o))
	})
}

func (suite *ValidateTestSuite) TestOriginEntry_UnmarshalYAML() {
	suite.T().Run("scalar decodes to Origin", func(t *testing.T) {
		var o OriginEntry
		require.NoError(t, yaml.Unmarshal([]byte(`https://app.example.com`), &o))
		assert.Equal(t, OriginEntry{Origin: "https://app.example.com"}, o)
	})

	suite.T().Run("mapping decodes to Regex", func(t *testing.T) {
		var o OriginEntry
		require.NoError(t, yaml.Unmarshal([]byte(`regex: "^https://.*$"`), &o))
		assert.Equal(t, OriginEntry{Regex: "^https://.*$"}, o)
	})

	suite.T().Run("mapping missing regex field fails", func(t *testing.T) {
		var o OriginEntry
		assert.Error(t, yaml.Unmarshal([]byte(`{}`), &o))
	})

	suite.T().Run("mapping with non-scalar regex field fails", func(t *testing.T) {
		var o OriginEntry
		assert.Error(t, yaml.Unmarshal([]byte("regex: [1, 2, 3]"), &o))
	})

	suite.T().Run("sequence fails", func(t *testing.T) {
		var o OriginEntry
		assert.Error(t, yaml.Unmarshal([]byte(`- foo`), &o))
	})
}

//nolint:dupl // JSON and YAML marshal tests mirror the same shape, kept distinct per format
func (suite *ValidateTestSuite) TestOriginEntry_MarshalJSON() {
	suite.T().Run("literal round-trips through JSON", func(t *testing.T) {
		o := OriginEntry{Origin: "https://app.example.com"}
		data, err := json.Marshal(o)
		require.NoError(t, err)
		assert.JSONEq(t, `"https://app.example.com"`, string(data))
	})

	suite.T().Run("regex round-trips through JSON", func(t *testing.T) {
		o := OriginEntry{Regex: "^https://.*$"}
		data, err := json.Marshal(o)
		require.NoError(t, err)
		assert.JSONEq(t, `{"regex":"^https://.*$"}`, string(data))
	})

	suite.T().Run("both set fails to marshal", func(t *testing.T) {
		o := OriginEntry{Origin: "https://app.example.com", Regex: "^https://.*$"}
		_, err := json.Marshal(o)
		assert.Error(t, err)
	})

	suite.T().Run("neither set fails to marshal", func(t *testing.T) {
		_, err := json.Marshal(OriginEntry{})
		assert.Error(t, err)
	})
}

//nolint:dupl // JSON and YAML marshal tests mirror the same shape, kept distinct per format
func (suite *ValidateTestSuite) TestOriginEntry_MarshalYAML() {
	suite.T().Run("literal round-trips through YAML", func(t *testing.T) {
		o := OriginEntry{Origin: "https://app.example.com"}
		data, err := yaml.Marshal(o)
		require.NoError(t, err)
		assert.Equal(t, "https://app.example.com\n", string(data))
	})

	suite.T().Run("regex round-trips through YAML", func(t *testing.T) {
		o := OriginEntry{Regex: "^https://.*$"}
		data, err := yaml.Marshal(o)
		require.NoError(t, err)
		assert.Equal(t, "regex: ^https://.*$\n", string(data))
	})

	suite.T().Run("both set fails to marshal", func(t *testing.T) {
		o := OriginEntry{Origin: "https://app.example.com", Regex: "^https://.*$"}
		_, err := yaml.Marshal(o)
		assert.Error(t, err)
	})

	suite.T().Run("neither set fails to marshal", func(t *testing.T) {
		_, err := yaml.Marshal(OriginEntry{})
		assert.Error(t, err)
	})
}

func (suite *ValidateTestSuite) TestOriginConfig_Unmarshal() {
	cases := []struct {
		name       string
		unmarshal  func([]byte, any) error
		validRaw   string
		nullRaw    string
		malformed  string
		notListRaw string
	}{
		{
			name:       "JSON",
			unmarshal:  json.Unmarshal,
			validRaw:   `{"allowedOrigins":["https://app.example.com",{"regex":"^https://.*\\.example\\.com$"}]}`,
			nullRaw:    `{"allowedOrigins":null}`,
			malformed:  `[]`,
			notListRaw: `{"allowedOrigins":"https://app.example.com"}`,
		},
		{
			name:      "YAML",
			unmarshal: yaml.Unmarshal,
			validRaw: "allowedOrigins:\n  - https://app.example.com\n  - " +
				"regex: \"^https://.*\\\\.example\\\\.com$\"\n",
			nullRaw:    "allowedOrigins:",
			malformed:  "- foo",
			notListRaw: "allowedOrigins: https://app.example.com",
		},
	}

	for _, tc := range cases {
		suite.T().Run(tc.name+" decodes literal and regex entries", func(t *testing.T) {
			var cfg OriginConfig
			require.NoError(t, tc.unmarshal([]byte(tc.validRaw), &cfg))
			assert.Equal(t, []OriginEntry{
				{Origin: "https://app.example.com"},
				{Regex: `^https://.*\.example\.com$`},
			}, cfg.AllowedOrigins)
		})

		suite.T().Run(tc.name+" omitted field leaves AllowedOrigins nil", func(t *testing.T) {
			var cfg OriginConfig
			require.NoError(t, tc.unmarshal([]byte(`{}`), &cfg))
			assert.Nil(t, cfg.AllowedOrigins)
		})

		suite.T().Run(tc.name+" explicit null is rejected", func(t *testing.T) {
			var cfg OriginConfig
			assert.Error(t, tc.unmarshal([]byte(tc.nullRaw), &cfg))
		})

		suite.T().Run(tc.name+" malformed top level is rejected", func(t *testing.T) {
			var cfg OriginConfig
			assert.Error(t, tc.unmarshal([]byte(tc.malformed), &cfg))
		})

		suite.T().Run(tc.name+" non-list allowedOrigins is rejected", func(t *testing.T) {
			var cfg OriginConfig
			assert.Error(t, tc.unmarshal([]byte(tc.notListRaw), &cfg))
		})
	}

	suite.T().Run("YAML skips unrelated keys before allowedOrigins", func(t *testing.T) {
		var cfg OriginConfig
		raw := "other: value\nallowedOrigins:\n  - https://app.example.com\n"
		require.NoError(t, yaml.Unmarshal([]byte(raw), &cfg))
		assert.Equal(t, []OriginEntry{{Origin: "https://app.example.com"}}, cfg.AllowedOrigins)
	})
}
