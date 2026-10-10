// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

// ModelTestSuite covers the wire encodings of the request and export types.
type ModelTestSuite struct {
	suite.Suite
}

func TestModelTestSuite(t *testing.T) {
	suite.Run(t, new(ModelTestSuite))
}

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
		Targets: []TargetRequest{
			{Scope: ScopeAllOUs, ExcludedOUIDs: []string{"ou-3"}, OverlayRules: map[string]OverlayRule{
				"permissions": {
					Value:          members("billing"),
					AllowedValues:  members("billing", "bookings"),
					ExcludedValues: members("billing:refunds"),
				},
			}},
			{Scope: ScopeRoot, OUID: "root-1"},
			{Scope: ScopeChildSubtree, OUID: "ou-2", OverlayRules: map[string]OverlayRule{
				"assignments": {Editable: true},
			}},
		},
		Version: 3,
	}
}

// The declarative and REST surfaces have to agree, so the two encodings must use the same names.
func (s *ModelTestSuite) TestPolicyRequestUsesTheSameCamelCaseNamesInBothEncodings() {
	req := fullRequest()

	asJSON, err := json.Marshal(req)
	s.Require().NoError(err)
	asYAML, err := yaml.Marshal(req)
	s.Require().NoError(err)

	var fromJSON map[string]interface{}
	s.Require().NoError(json.Unmarshal(asJSON, &fromJSON))

	// Compared whole rather than key by key, so a missing tag on a nested struct cannot slip past:
	// yaml lowercases field names of its own when a tag is absent.
	s.Equal(fromJSON, s.normalize(asYAML),
		"the two encodings disagree, so some field is missing a matching tag")

	targets, ok := fromJSON["targets"].([]interface{})
	s.Require().True(ok, "targets is absent, so the field is untagged")

	broad, ok := targets[0].(map[string]interface{})
	s.Require().True(ok)
	s.ElementsMatch([]string{"scope", "excludedOuIds", "overlayRules"}, keysOf(broad))

	named, ok := targets[2].(map[string]interface{})
	s.Require().True(ok)
	s.ElementsMatch([]string{"scope", "ouId", "overlayRules"}, keysOf(named))

	rule, ok := broad["overlayRules"].(map[string]interface{})["permissions"].(map[string]interface{})
	s.Require().True(ok)
	s.ElementsMatch([]string{"editable", "value", "allowedValues", "excludedValues"}, keysOf(rule))
}

// An export is written as YAML and served as JSON, so a field named differently by the two
// encodings would not survive the round trip.
func (s *ModelTestSuite) TestReplayablePolicyUsesTheSameCamelCaseNamesInBothEncodings() {
	asJSON, err := json.Marshal(ReplayablePolicy{InitiatingOUID: "ou-1", Request: fullRequest()})
	s.Require().NoError(err)
	asYAML, err := yaml.Marshal(ReplayablePolicy{InitiatingOUID: "ou-1", Request: fullRequest()})
	s.Require().NoError(err)

	var fromJSON map[string]interface{}
	s.Require().NoError(json.Unmarshal(asJSON, &fromJSON))

	s.ElementsMatch([]string{"initiatingOuId", "request"}, keysOf(fromJSON))
	s.Equal(fromJSON, s.normalize(asYAML))
}

// normalize re-encodes a yaml document through json, so the two can be compared on field names
// without their differing number types getting in the way.
func (s *ModelTestSuite) normalize(document []byte) map[string]interface{} {
	s.T().Helper()
	var intermediate map[string]interface{}
	s.Require().NoError(yaml.Unmarshal(document, &intermediate))
	encoded, err := json.Marshal(intermediate)
	s.Require().NoError(err)
	var out map[string]interface{}
	s.Require().NoError(json.Unmarshal(encoded, &out))
	return out
}

// The distinction the pointer types exist for has to survive the wire, or a rule that permits
// nothing comes back as one that permits everything.
func (s *ModelTestSuite) TestOverlayRuleRoundTripsAbsentSeparatelyFromEmpty() {
	tests := []struct {
		name  string
		rule  OverlayRule
		check func(got OverlayRule)
	}{
		{
			name: "omitted stays omitted",
			rule: OverlayRule{Editable: true},
			check: func(got OverlayRule) {
				s.Nil(got.AllowedValues, "an omitted bound must not come back as a constraint")
			},
		},
		{
			name: "explicitly empty stays empty",
			rule: OverlayRule{Editable: true, AllowedValues: members()},
			check: func(got OverlayRule) {
				s.Require().NotNil(got.AllowedValues, "an empty bound permits nothing and must survive")
				s.Empty(*got.AllowedValues)
			},
		},
		{
			name: "a populated bound survives",
			rule: OverlayRule{Editable: true, AllowedValues: members("a", "b")},
			check: func(got OverlayRule) {
				s.Require().NotNil(got.AllowedValues)
				s.Equal([]string{"a", "b"}, *got.AllowedValues)
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name+" (json)", func() {
			data, err := json.Marshal(tt.rule)
			s.Require().NoError(err)
			var got OverlayRule
			s.Require().NoError(json.Unmarshal(data, &got))
			tt.check(got)
		})
		s.Run(tt.name+" (yaml)", func() {
			data, err := yaml.Marshal(tt.rule)
			s.Require().NoError(err)
			var got OverlayRule
			s.Require().NoError(yaml.Unmarshal(data, &got))
			tt.check(got)
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
