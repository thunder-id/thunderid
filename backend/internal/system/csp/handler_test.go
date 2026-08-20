// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package csp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyHandler_Decode(t *testing.T) {
	t.Run("empty input yields the zero policy", func(t *testing.T) {
		v, err := PolicyHandler{}.Decode(nil)
		require.NoError(t, err)
		assert.Equal(t, PolicyConfig{}, v)
	})

	t.Run("valid JSON decodes", func(t *testing.T) {
		v, err := PolicyHandler{}.Decode([]byte(`{"reportOnly":false,"reportUri":"/r","directives":{"img-src":["x"]}}`))
		require.NoError(t, err)
		cfg, ok := v.(PolicyConfig)
		require.True(t, ok)
		assert.False(t, cfg.EffectiveReportOnly())
		assert.Equal(t, "/r", cfg.ReportURI)
		assert.Equal(t, []string{"x"}, cfg.Directives["img-src"])
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		_, err := PolicyHandler{}.Decode([]byte(`{bad`))
		assert.Error(t, err)
	})
}

func TestPolicyHandler_Validate(t *testing.T) {
	assert.NoError(t, PolicyHandler{}.Validate(context.Background(), PolicyConfig{}, nil, nil))
	// Any directive may be configured, including default-src.
	assert.NoError(t, PolicyHandler{}.Validate(context.Background(),
		PolicyConfig{Directives: map[string][]string{"default-src": {"'self'"}}}, nil, nil))
	// An invalid directive name is rejected.
	assert.Error(t, PolicyHandler{}.Validate(context.Background(),
		PolicyConfig{Directives: map[string][]string{"bad name": {"'self'"}}}, nil, nil))
}

func TestPolicyHandler_Merge(t *testing.T) {
	t.Run("writable report-only overrides read-only", func(t *testing.T) {
		merged := PolicyHandler{}.Merge(
			PolicyConfig{ReportOnly: boolPtr(true)},
			PolicyConfig{ReportOnly: boolPtr(false)},
		).(PolicyConfig)
		assert.False(t, merged.EffectiveReportOnly())
	})

	t.Run("read-only report-only stands when writable is unset", func(t *testing.T) {
		merged := PolicyHandler{}.Merge(
			PolicyConfig{ReportOnly: boolPtr(false)},
			PolicyConfig{},
		).(PolicyConfig)
		assert.False(t, merged.EffectiveReportOnly())
	})

	t.Run("writable report_uri wins when set", func(t *testing.T) {
		merged := PolicyHandler{}.Merge(
			PolicyConfig{ReportURI: "/ro"},
			PolicyConfig{ReportURI: "/wr"},
		).(PolicyConfig)
		assert.Equal(t, "/wr", merged.ReportURI)
	})

	t.Run("writable directive replaces read-only, others are kept", func(t *testing.T) {
		merged := PolicyHandler{}.Merge(
			PolicyConfig{Directives: map[string][]string{"img-src": {"a", "b"}}},
			PolicyConfig{Directives: map[string][]string{"img-src": {"c"}, "font-src": {"f"}}},
		).(PolicyConfig)
		assert.Equal(t, []string{"c"}, merged.Directives["img-src"])
		assert.Equal(t, []string{"f"}, merged.Directives["font-src"])
	})

	t.Run("both layers empty yields nil directives", func(t *testing.T) {
		merged := PolicyHandler{}.Merge(PolicyConfig{}, PolicyConfig{}).(PolicyConfig)
		assert.Nil(t, merged.Directives)
	})

	t.Run("writable path replaces read-only path of the same prefix, others kept", func(t *testing.T) {
		merged := PolicyHandler{}.Merge(
			PolicyConfig{Paths: []PathPolicy{
				{Location: "/console/", Directives: map[string][]string{"img-src": {"'self'"}}},
				{Location: "/gate/", Directives: map[string][]string{"font-src": {"'self'"}}},
			}},
			PolicyConfig{Paths: []PathPolicy{
				{Location: "/console/", Directives: map[string][]string{"img-src": {"'self'", "https:"}}},
			}},
		).(PolicyConfig)

		require.Len(t, merged.Paths, 2)
		byPrefix := map[string]map[string][]string{}
		for _, p := range merged.Paths {
			byPrefix[p.Location] = p.Directives
		}
		assert.Equal(t, []string{"'self'", "https:"}, byPrefix["/console/"]["img-src"])
		assert.Equal(t, []string{"'self'"}, byPrefix["/gate/"]["font-src"])
	})
}

// ensure PolicyConfig round-trips through JSON as the section store persists it.
func TestPolicyConfig_JSONRoundTrip(t *testing.T) {
	original := PolicyConfig{
		ReportOnly: boolPtr(false),
		ReportURI:  "/csp",
		Directives: map[string][]string{"connect-src": {"https://api.example.com"}},
	}
	raw, err := json.Marshal(original)
	require.NoError(t, err)

	decoded, err := PolicyHandler{}.Decode(raw)
	require.NoError(t, err)
	assert.Equal(t, original, decoded.(PolicyConfig))
}
