// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// members returns a pointer to a list, for the fields where absent and empty differ.
func members(v ...string) *[]string {
	out := append([]string{}, v...)
	return &out
}

// fullRequest exercises every tagged field, so a missing or misspelled tag shows up as a name the
// expectations below do not contain.
func fullRequest() PolicyRequest {
	return PolicyRequest{
		InitiatingOUID: "ou-1",
		TargetOUScope: TargetOUScope{
			AllOUs:            true,
			AllRoots:          true,
			RootOUIDs:         []string{"root-1"},
			ExcludedRootOUIDs: []string{"root-2"},
			AllChildren:       true,
			ChildOUIDs: []TargetEntry{{
				OUID:         "ou-2",
				AllChildren:  true,
				OverlayRules: map[string]OverlayRule{"assignments": {Editable: true}},
			}},
			ExcludedOUIDs: []string{"ou-3"},
		},
		OverlayRules: map[string]OverlayRule{"permissions": {
			Value:          members("billing"),
			AllowedValues:  members("billing", "bookings"),
			ExcludedValues: members("billing:refunds"),
		}},
		Version: 3,
	}
}

// The declarative and REST surfaces have to agree, so the two encodings must use the same names.
func TestPolicyRequestUsesTheSameCamelCaseNamesInBothEncodings(t *testing.T) {
	req := fullRequest()

	asJSON, err := json.Marshal(req)
	require.NoError(t, err)
	asYAML, err := yaml.Marshal(req)
	require.NoError(t, err)

	var fromJSON map[string]interface{}
	require.NoError(t, json.Unmarshal(asJSON, &fromJSON))

	// Compared whole rather than key by key, so a missing tag on a nested struct cannot slip past:
	// yaml lowercases field names of its own when a tag is absent.
	assert.Equal(t, fromJSON, normalize(t, asYAML),
		"the two encodings disagree, so some field is missing a matching tag")

	scope, ok := fromJSON["targetOuScope"].(map[string]interface{})
	require.True(t, ok, "targetOuScope is absent, so the field is untagged")
	assert.ElementsMatch(t,
		[]string{"allOus", "allRoots", "rootOuIds", "excludedRootOuIds", "allChildren", "childOuIds", "excludedOuIds"},
		keysOf(scope))

	entry, ok := scope["childOuIds"].([]interface{})[0].(map[string]interface{})
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"ouId", "allChildren", "overlayRules"}, keysOf(entry))

	rule, ok := fromJSON["overlayRules"].(map[string]interface{})["permissions"].(map[string]interface{})
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"editable", "value", "allowedValues", "excludedValues"}, keysOf(rule))
}

// An export is written as YAML and served as JSON, so a field named differently by the two
// encodings would not survive the round trip.
func TestReplayablePolicyUsesTheSameCamelCaseNamesInBothEncodings(t *testing.T) {
	asJSON, err := json.Marshal(ReplayablePolicy{InitiatingOUID: "ou-1", Request: fullRequest()})
	require.NoError(t, err)
	asYAML, err := yaml.Marshal(ReplayablePolicy{InitiatingOUID: "ou-1", Request: fullRequest()})
	require.NoError(t, err)

	var fromJSON map[string]interface{}
	require.NoError(t, json.Unmarshal(asJSON, &fromJSON))

	assert.ElementsMatch(t, []string{"initiatingOuId", "request"}, keysOf(fromJSON))
	assert.Equal(t, fromJSON, normalize(t, asYAML))
}

// normalize re-encodes a yaml document through json, so the two can be compared on field names
// without their differing number types getting in the way.
func normalize(t *testing.T, document []byte) map[string]interface{} {
	t.Helper()
	var intermediate map[string]interface{}
	require.NoError(t, yaml.Unmarshal(document, &intermediate))
	encoded, err := json.Marshal(intermediate)
	require.NoError(t, err)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(encoded, &out))
	return out
}

// The distinction the pointer types exist for has to survive the wire, or a rule that permits
// nothing comes back as one that permits everything.
func TestOverlayRuleRoundTripsAbsentSeparatelyFromEmpty(t *testing.T) {
	tests := []struct {
		name  string
		rule  OverlayRule
		check func(t *testing.T, got OverlayRule)
	}{
		{
			name: "omitted stays omitted",
			rule: OverlayRule{Editable: true},
			check: func(t *testing.T, got OverlayRule) {
				assert.Nil(t, got.AllowedValues, "an omitted bound must not come back as a constraint")
			},
		},
		{
			name: "explicitly empty stays empty",
			rule: OverlayRule{Editable: true, AllowedValues: members()},
			check: func(t *testing.T, got OverlayRule) {
				require.NotNil(t, got.AllowedValues, "an empty bound permits nothing and must survive")
				assert.Empty(t, *got.AllowedValues)
			},
		},
		{
			name: "a populated bound survives",
			rule: OverlayRule{Editable: true, AllowedValues: members("a", "b")},
			check: func(t *testing.T, got OverlayRule) {
				require.NotNil(t, got.AllowedValues)
				assert.Equal(t, []string{"a", "b"}, *got.AllowedValues)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (json)", func(t *testing.T) {
			data, err := json.Marshal(tt.rule)
			require.NoError(t, err)
			var got OverlayRule
			require.NoError(t, json.Unmarshal(data, &got))
			tt.check(t, got)
		})
		t.Run(tt.name+" (yaml)", func(t *testing.T) {
			data, err := yaml.Marshal(tt.rule)
			require.NoError(t, err)
			var got OverlayRule
			require.NoError(t, yaml.Unmarshal(data, &got))
			tt.check(t, got)
		})
	}
}

// keysOf returns a map's keys, so field naming can be asserted independently of values.
func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
