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

// A target that names no organization unit in storage carries the issuing unit as its anchor. The
// response drops it, so what an API returns is what a create would have sent.
func (s *ModelTestSuite) TestPolicyResponseDropsTheImpliedAnchor() {
	response := ToPolicyResponse(Policy{
		ID: "p1", InitiatingOUID: "root", OwningOUID: "root", Stage: stageShare, Version: 2,
		Targets: []Target{
			{ID: "t1", Scope: ScopeAllChildren, OUID: "root", ExcludedOUIDs: []string{"child-b"}},
			{ID: "t2", Scope: ScopeChild, OUID: "child-b"},
			{ID: "t3", Scope: ScopeAllOUs},
		},
	})

	s.Require().Len(response.Targets, 3)
	s.Empty(response.Targets[0].OUID, "allChildren anchors on the issuer, which is implied")
	s.Equal([]string{"child-b"}, response.Targets[0].ExcludedOUIDs)
	s.Equal("child-b", response.Targets[1].OUID, "a named target keeps the unit it names")
	s.Empty(response.Targets[2].OUID, "allOus names no unit at all")
	s.Equal("share", response.Stage)
	s.Equal(2, response.Version)
	s.Equal("p1", response.ID)
}

// A policy a resource file declares is answered as read-only, in the name the rest of the API uses
// for the same idea. What the caller can act on is that it cannot be changed, not where it came
// from.
func (s *ModelTestSuite) TestADeclaredPolicyIsAnsweredAsReadOnly() {
	declared, err := json.Marshal(ToPolicyResponse(Policy{ID: "p1", Declared: true}))
	s.Require().NoError(err)
	stored, err := json.Marshal(ToPolicyResponse(Policy{ID: "p2", Declared: false}))
	s.Require().NoError(err)

	var asDeclared, asStored map[string]interface{}
	s.Require().NoError(json.Unmarshal(declared, &asDeclared))
	s.Require().NoError(json.Unmarshal(stored, &asStored))

	s.Equal(true, asDeclared["isReadOnly"])
	s.Equal(false, asStored["isReadOnly"])
	s.NotContains(asDeclared, "declared", "the framework's own word stays inside the framework")
}

// The row identifiers a policy carries are the framework's own, so they stay out of the response.
func (s *ModelTestSuite) TestPolicyResponseCarriesNoRowIdentifiers() {
	encoded, err := json.Marshal(ToPolicyResponse(Policy{
		ID: "p1", Targets: []Target{{ID: "target-row-id", Scope: ScopeChild, OUID: "child-a"}},
	}))
	s.Require().NoError(err)
	s.NotContains(string(encoded), "target-row-id")
}

// The owning organization unit is the resource's own, identical on every one of its policies, and
// these endpoints are mounted under the resource that states it. Repeating it on each policy of a
// page says nothing the caller did not already supply.
func (s *ModelTestSuite) TestPolicyResponseDoesNotRepeatTheOwningUnit() {
	encoded, err := json.Marshal(ToPolicyResponse(Policy{
		ID: "p1", OwningOUID: "acme-root", InitiatingOUID: "acme-customer-a",
	}))
	s.Require().NoError(err)

	var decoded map[string]interface{}
	s.Require().NoError(json.Unmarshal(encoded, &decoded))
	s.NotContains(decoded, "owningOuId")
	s.Equal("acme-customer-a", decoded["initiatingOuId"],
		"whose decision it is does vary, and is carried")
}

// A page keeps its counts, and every policy in it is shaped the same way.
func (s *ModelTestSuite) TestPolicyListResponseShapesEveryPolicy() {
	response := ToPolicyListResponse(PolicyList{
		TotalResults: 7, StartIndex: 1, Count: 1,
		Policies: []Policy{{ID: "p1", Targets: []Target{
			{ID: "t1", Scope: ScopeAllChildren, OUID: "root"}}}},
	})

	s.Equal(7, response.TotalResults)
	s.Equal(1, response.StartIndex)
	s.Require().Len(response.Policies, 1)
	s.Empty(response.Policies[0].Targets[0].OUID)
}

// The empty collections come back as collections rather than as null, so a caller reads them
// without guarding against two different shapes.
func (s *ModelTestSuite) TestResolvedOverlayResponseIsNeverNull() {
	encoded, err := json.Marshal(ToResolvedOverlayResponse(ResolvedOverlay{
		OUID: "child-a", Visible: true,
	}))
	s.Require().NoError(err)

	var decoded map[string]interface{}
	s.Require().NoError(json.Unmarshal(encoded, &decoded))
	s.NotNil(decoded["rules"], "an absent rule set is an empty object")
	s.NotNil(decoded["policyIds"], "an absent policy list is an empty array")
	s.Equal(true, decoded["visible"])
}
