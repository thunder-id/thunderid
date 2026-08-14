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

func (suite *ValidateTestSuite) TestCORSOrigin_UnmarshalJSON() {
	suite.T().Run("literal string decodes to Origin", func(t *testing.T) {
		var o CORSOrigin
		require.NoError(t, json.Unmarshal([]byte(`"https://app.example.com"`), &o))
		assert.Equal(t, CORSOrigin{Origin: "https://app.example.com"}, o)
	})

	suite.T().Run("regex object decodes to Regex", func(t *testing.T) {
		var o CORSOrigin
		require.NoError(t, json.Unmarshal([]byte(`{"regex":"^https://.*$"}`), &o))
		assert.Equal(t, CORSOrigin{Regex: "^https://.*$"}, o)
	})

	suite.T().Run("regex object missing regex field fails", func(t *testing.T) {
		var o CORSOrigin
		assert.Error(t, json.Unmarshal([]byte(`{}`), &o))
	})

	suite.T().Run("non-string non-object fails", func(t *testing.T) {
		var o CORSOrigin
		assert.Error(t, json.Unmarshal([]byte(`42`), &o))
	})
}

func (suite *ValidateTestSuite) TestCORSOrigin_UnmarshalYAML() {
	suite.T().Run("scalar decodes to Origin", func(t *testing.T) {
		var o CORSOrigin
		require.NoError(t, yaml.Unmarshal([]byte(`https://app.example.com`), &o))
		assert.Equal(t, CORSOrigin{Origin: "https://app.example.com"}, o)
	})

	suite.T().Run("mapping decodes to Regex", func(t *testing.T) {
		var o CORSOrigin
		require.NoError(t, yaml.Unmarshal([]byte(`regex: "^https://.*$"`), &o))
		assert.Equal(t, CORSOrigin{Regex: "^https://.*$"}, o)
	})

	suite.T().Run("mapping missing regex field fails", func(t *testing.T) {
		var o CORSOrigin
		assert.Error(t, yaml.Unmarshal([]byte(`{}`), &o))
	})
}

func (suite *ValidateTestSuite) TestCORSOrigin_Marshal() {
	suite.T().Run("literal round-trips through JSON", func(t *testing.T) {
		o := CORSOrigin{Origin: "https://app.example.com"}
		data, err := json.Marshal(o)
		require.NoError(t, err)
		assert.JSONEq(t, `"https://app.example.com"`, string(data))
	})

	suite.T().Run("regex round-trips through JSON", func(t *testing.T) {
		o := CORSOrigin{Regex: "^https://.*$"}
		data, err := json.Marshal(o)
		require.NoError(t, err)
		assert.JSONEq(t, `{"regex":"^https://.*$"}`, string(data))
	})

	suite.T().Run("both set fails to marshal", func(t *testing.T) {
		o := CORSOrigin{Origin: "https://app.example.com", Regex: "^https://.*$"}
		_, err := json.Marshal(o)
		assert.Error(t, err)
	})

	suite.T().Run("neither set fails to marshal", func(t *testing.T) {
		_, err := json.Marshal(CORSOrigin{})
		assert.Error(t, err)
	})
}

func (suite *ValidateTestSuite) TestCORSConfig_UnmarshalJSON() {
	var cfg CORSConfig
	raw := `{"allowedOrigins":["https://app.example.com",{"regex":"^https://.*\\.example\\.com$"}]}`
	suite.Require().NoError(json.Unmarshal([]byte(raw), &cfg))
	assert.Equal(suite.T(), []CORSOrigin{
		{Origin: "https://app.example.com"},
		{Regex: `^https://.*\.example\.com$`},
	}, cfg.AllowedOrigins)
}
