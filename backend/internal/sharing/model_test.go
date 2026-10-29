// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
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
// expectations below do not contain. It is not a request the service would accept: every section is
// stated, with the fields of both modes, because the wire names are all this is here to pin.
func fullRequest() PolicyRequest {
	rules := map[string]OverlayRule{
		"permissions": {
			Value:          members("billing"),
			AllowedValues:  members("billing", "bookings"),
			ExcludedValues: members("billing:refunds"),
		},
	}
	return PolicyRequest{
		ID:             "policy-1",
		InitiatingOUID: "ou-1",
		AllUnits:       &AllUnitsRequest{ExcludedOUIDs: []string{"ou-3"}, OverlayRules: rules},
		ChildUnits: &ChildUnitsRequest{
			Mode:          ChildUnitsSelected,
			ExcludedOUIDs: []string{"ou-4"},
			OverlayRules:  rules,
			Units: []ChildUnitRequest{{
				OUID: "ou-2", Reach: ReachSubtree,
				OverlayRules: map[string]OverlayRule{"assignments": {Editable: true}},
			}},
		},
		OtherTreeRoots: &OtherTreeRootsRequest{
			Mode:          OtherTreeRootsSelected,
			ExcludedOUIDs: []string{"root-2"},
			OverlayRules:  rules,
			Units:         []RootUnitRequest{{OUID: "root-1", OverlayRules: rules}},
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

	s.ElementsMatch(
		[]string{"id", "initiatingOuId", "allUnits", "childUnits", "otherTreeRoots", "version"},
		keysOf(fromJSON))

	allUnits, ok := fromJSON["allUnits"].(map[string]interface{})
	s.Require().True(ok, "allUnits is absent, so the field is untagged")
	s.ElementsMatch([]string{"excludedOuIds", "overlayRules"}, keysOf(allUnits))

	childUnits, ok := fromJSON["childUnits"].(map[string]interface{})
	s.Require().True(ok, "childUnits is absent, so the field is untagged")
	s.ElementsMatch([]string{"mode", "excludedOuIds", "overlayRules", "units"}, keysOf(childUnits))
	s.Equal("selected", childUnits["mode"])

	unit, ok := childUnits["units"].([]interface{})[0].(map[string]interface{})
	s.Require().True(ok)
	s.ElementsMatch([]string{"ouId", "reach", "overlayRules"}, keysOf(unit))
	s.Equal("subtree", unit["reach"])

	// The roots of other trees are stated the way child units are: the same modes, and a list
	// called units.
	roots, ok := fromJSON["otherTreeRoots"].(map[string]interface{})
	s.Require().True(ok, "otherTreeRoots is absent, so the field is untagged")
	s.ElementsMatch([]string{"mode", "excludedOuIds", "overlayRules", "units"}, keysOf(roots))
	s.Equal("selected", roots["mode"])

	root, ok := roots["units"].([]interface{})[0].(map[string]interface{})
	s.Require().True(ok)
	s.ElementsMatch([]string{"ouId", "overlayRules"}, keysOf(root))

	rule, ok := allUnits["overlayRules"].(map[string]interface{})["permissions"].(map[string]interface{})
	s.Require().True(ok)
	s.ElementsMatch([]string{"editable", "value", "allowedValues", "excludedValues"}, keysOf(rule))
}

// A section the policy does not state is left off the wire altogether, which is what tells a reader
// "not decided" apart from "decided, with nothing to add".
func (s *ModelTestSuite) TestPolicyRequestLeavesAnUnstatedSectionOffTheWire() {
	req := PolicyRequest{ChildUnits: &ChildUnitsRequest{Mode: ChildUnitsAll}}

	asJSON, err := json.Marshal(req)
	s.Require().NoError(err)
	s.JSONEq(`{"childUnits":{"mode":"all"}}`, string(asJSON))

	asYAML, err := yaml.Marshal(req)
	s.Require().NoError(err)
	s.YAMLEq("childUnits:\n  mode: all\n", string(asYAML))
}

// A resource file states a section the way it reads, and each one lands in its own field.
func (s *ModelTestSuite) TestPolicyRequestReadsTheSectionsOfAResourceFile() {
	document := []byte(`
id: storefront-policy
childUnits:
  mode: selected
  units:
    - ouId: acme-retail
      reach: subtree
      overlayRules:
        tokenValidity:
          editable: false
          value: ["900"]
    - ouId: acme-wholesale
      reach: unit
otherTreeRoots:
  mode: all
  excludedOuIds: [contoso]
`)

	var req PolicyRequest
	s.Require().NoError(yaml.Unmarshal(document, &req))

	s.Equal("storefront-policy", req.ID)
	s.Nil(req.AllUnits)
	s.Require().NotNil(req.ChildUnits)
	s.Equal(ChildUnitsSelected, req.ChildUnits.Mode)
	s.Require().Len(req.ChildUnits.Units, 2)
	s.Equal(ChildUnitRequest{
		OUID: "acme-retail", Reach: ReachSubtree,
		OverlayRules: map[string]OverlayRule{"tokenValidity": {Value: members("900")}},
	}, req.ChildUnits.Units[0])
	s.Equal(ChildUnitRequest{OUID: "acme-wholesale", Reach: ReachUnit}, req.ChildUnits.Units[1])
	s.Require().NotNil(req.OtherTreeRoots)
	s.Equal(OtherTreeRootsAll, req.OtherTreeRoots.Mode)
	s.Equal([]string{"contoso"}, req.OtherTreeRoots.ExcludedOUIDs)
}

// A bare "allUnits: {}" states the section with nothing to exclude, which is the whole of the
// decision for a resource shared with everyone.
func (s *ModelTestSuite) TestPolicyRequestReadsAnEmptyAllUnitsSectionAsStated() {
	var fromYAML PolicyRequest
	s.Require().NoError(yaml.Unmarshal([]byte("allUnits: {}\n"), &fromYAML))
	s.NotNil(fromYAML.AllUnits)

	var fromJSON PolicyRequest
	s.Require().NoError(json.Unmarshal([]byte(`{"allUnits":{}}`), &fromJSON))
	s.NotNil(fromJSON.AllUnits)
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

// A rule that leaves editable out decodes as one that states nothing, in both encodings, so a field
// taking editable alone can refuse it rather than read the omission as false.
func (s *ModelTestSuite) TestOverlayRuleDecodingKeepsAnOmittedEditableApart() {
	tests := []struct {
		name         string
		json         string
		yaml         string
		editable     bool
		editableOnly bool
	}{
		{"editable left out", `{}`, `{}`, false, false},
		{"editable false", `{"editable": false}`, `editable: false`, false, true},
		{"editable true", `{"editable": true}`, `editable: true`, true, true},
		{"members named", `{"editable": true, "allowedValues": ["a"]}`, "editable: true\nallowedValues: [a]",
			true, false},
	}

	for _, tt := range tests {
		s.Run(tt.name+" (json)", func() {
			var got OverlayRule
			s.Require().NoError(json.Unmarshal([]byte(tt.json), &got))
			s.Equal(tt.editable, got.Editable)
			s.Equal(tt.editableOnly, got.statesEditableOnly())
		})
		s.Run(tt.name+" (yaml)", func() {
			var got OverlayRule
			s.Require().NoError(yaml.Unmarshal([]byte(tt.yaml), &got))
			s.Equal(tt.editable, got.Editable)
			s.Equal(tt.editableOnly, got.statesEditableOnly())
		})
	}
}

// A rule refuses a key it does not declare in both encodings, so a misspelled bound is not read as
// no bound at all.
func (s *ModelTestSuite) TestOverlayRuleDecodingRefusesAnUndeclaredKey() {
	var fromJSON OverlayRule
	s.Error(json.Unmarshal([]byte(`{"editable": true, "allowedValue": ["a"]}`), &fromJSON))
	var fromYAML OverlayRule
	s.Error(yaml.Unmarshal([]byte("editable: true\nallowedValue: [a]"), &fromYAML))
}

// keysOf returns a map's keys, so field naming can be asserted independently of values.
func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// A policy comes back with its sections in the shape a request states them in, so what a caller
// reads is what it sends back to edit the policy. A section the policy does not state is left out.
func (s *ModelTestSuite) TestPolicyResponseStatesTheSectionsAsARequestDoes() {
	response := ToPolicyResponse(Policy{
		ID: "p1", InitiatingOUID: "root", OwningOUID: "root", Stage: stageShare, Version: 2,
		ChildUnits: &ChildUnits{Mode: ChildUnitsAll, ExcludedOUIDs: []string{"child-b"}},
		OtherTreeRoots: &OtherTreeRoots{
			Mode: OtherTreeRootsSelected, Units: []string{"other-root"},
		},
	})

	s.Equal(&ChildUnitsRequest{Mode: ChildUnitsAll, ExcludedOUIDs: []string{"child-b"}}, response.ChildUnits)
	s.Equal(&OtherTreeRootsRequest{
		Mode: OtherTreeRootsSelected, Units: []RootUnitRequest{{OUID: "other-root"}},
	}, response.OtherTreeRoots)
	s.Nil(response.AllUnits)
	s.Equal("share", response.Stage)
	s.Equal(2, response.Version)
	s.Equal("p1", response.ID)

	encoded, err := json.Marshal(response)
	s.Require().NoError(err)
	var decoded map[string]interface{}
	s.Require().NoError(json.Unmarshal(encoded, &decoded))
	s.NotContains(decoded, "allUnits", "a section the policy does not state is left off the wire")
	s.Contains(decoded, "childUnits")
	s.Contains(decoded, "otherTreeRoots")
}

// A named child keeps its reach, and a policy reaching every organization unit names none.
func (s *ModelTestSuite) TestPolicyResponseKeepsNamedUnitsAndTheirReach() {
	named := ToPolicyResponse(Policy{
		ID: "p1",
		ChildUnits: &ChildUnits{Mode: ChildUnitsSelected, Units: []ChildUnit{
			{OUID: "child-a", Reach: ReachUnit}, {OUID: "child-b", Reach: ReachSubtree},
		}},
	})
	s.Equal([]ChildUnitRequest{
		{OUID: "child-a", Reach: ReachUnit}, {OUID: "child-b", Reach: ReachSubtree},
	}, named.ChildUnits.Units)

	everyone := ToPolicyResponse(Policy{ID: "p2", AllUnits: &AllUnits{ExcludedOUIDs: []string{"child-a"}}})
	s.Equal(&AllUnitsRequest{ExcludedOUIDs: []string{"child-a"}}, everyone.AllUnits)
	s.Nil(everyone.ChildUnits)
	s.Nil(everyone.OtherTreeRoots)
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
		Policies: []Policy{{ID: "p1", ChildUnits: &ChildUnits{Mode: ChildUnitsAll}}},
	}, "/things/t1/sharing-policies", 1, 0)

	s.Equal(7, response.TotalResults)
	s.Equal(1, response.StartIndex)
	s.Require().Len(response.Policies, 1)
	s.Equal(&ChildUnitsRequest{Mode: ChildUnitsAll}, response.Policies[0].ChildUnits)
}

// The page links point at the resource type's own listing, the way every other paginated list
// in the API does.
func (s *ModelTestSuite) TestPolicyListResponseLinksThePages() {
	response := ToPolicyListResponse(PolicyList{TotalResults: 5, StartIndex: 3, Count: 2},
		"/things/t1/sharing-policies", 2, 2)

	s.Equal([]utils.Link{
		{Href: "/things/t1/sharing-policies?offset=0&limit=2", Rel: "first"},
		{Href: "/things/t1/sharing-policies?offset=0&limit=2", Rel: "prev"},
		{Href: "/things/t1/sharing-policies?offset=4&limit=2", Rel: "next"},
		{Href: "/things/t1/sharing-policies?offset=4&limit=2", Rel: "last"},
	}, response.Links)
}

// A single page has nowhere to link to, and says so with an empty list rather than null.
func (s *ModelTestSuite) TestPolicyListResponseOnOnePageHasNoLinks() {
	encoded, err := json.Marshal(ToPolicyListResponse(PolicyList{TotalResults: 1, StartIndex: 1, Count: 1},
		"/things/t1/sharing-policies", 30, 0))
	s.Require().NoError(err)

	var decoded map[string]interface{}
	s.Require().NoError(json.Unmarshal(encoded, &decoded))
	s.Equal([]interface{}{}, decoded["links"])
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

// A create body becomes the framework request section by section, with whose decision it is.
func (s *ModelTestSuite) TestPolicyCreateBodyBecomesTheFrameworkRequest() {
	var body PolicyCreateBody
	s.Require().NoError(json.Unmarshal([]byte(`{
		"initiatingOuId": "child-a",
		"childUnits": {"mode": "selected", "units": [{"ouId": "grand-a", "reach": "subtree"}]},
		"otherTreeRoots": {"mode": "selected", "units": [{"ouId": "partner-root"}]}
	}`), &body))

	req := body.ToPolicyRequest()

	s.Equal("child-a", req.InitiatingOUID)
	s.Require().NotNil(req.ChildUnits)
	s.Equal(ChildUnitsSelected, req.ChildUnits.Mode)
	s.Equal([]ChildUnitRequest{{OUID: "grand-a", Reach: ReachSubtree}}, req.ChildUnits.Units)
	s.Require().NotNil(req.OtherTreeRoots)
	s.Equal([]RootUnitRequest{{OUID: "partner-root"}}, req.OtherTreeRoots.Units)
	s.Nil(req.AllUnits)
}

// Every section of a body carries its terms across, so an edit that restates them keeps them.
func (s *ModelTestSuite) TestPolicyBodyCarriesTheOverlayRules() {
	var body PolicyUpdateBody
	s.Require().NoError(json.Unmarshal([]byte(`{
		"allUnits": {"overlayRules": {"f": {"editable": false}}},
		"childUnits": {"mode": "all", "overlayRules": {"f": {"editable": false}},
			"units": [{"ouId": "child-a", "reach": "unit", "overlayRules": {"f": {"editable": false}}}]},
		"otherTreeRoots": {"mode": "all", "overlayRules": {"f": {"editable": false}},
			"units": [{"ouId": "partner-root", "overlayRules": {"f": {"editable": false}}}]}
	}`), &body))

	req := body.ToPolicyRequest()

	pinned := map[string]OverlayRule{"f": {Editable: false}}
	s.Equal(pinned, req.AllUnits.OverlayRules)
	s.Equal(pinned, req.ChildUnits.OverlayRules)
	s.Equal(pinned, req.ChildUnits.Units[0].OverlayRules)
	s.Equal(pinned, req.OtherTreeRoots.OverlayRules)
	s.Equal(pinned, req.OtherTreeRoots.Units[0].OverlayRules)
}

// An update body carries the version and the sections, and names no issuer.
func (s *ModelTestSuite) TestPolicyUpdateBodyCarriesTheVersion() {
	var body PolicyUpdateBody
	s.Require().NoError(json.Unmarshal([]byte(
		`{"version": 3, "allUnits": {"excludedOuIds": ["child-a"]}}`), &body))

	req := body.ToPolicyRequest()

	s.Equal(3, req.Version)
	s.Empty(req.InitiatingOUID)
	s.Require().NotNil(req.AllUnits)
	s.Equal([]string{"child-a"}, req.AllUnits.ExcludedOUIDs)
}

// Each framework client error is answered in the resource type's own code, keeping the framework's
// description; a server error stays a server error.
func (s *ModelTestSuite) TestErrorTranslationAnswersInTheTypesOwnCodes() {
	own := func(code string) tidcommon.ServiceError {
		return tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: code}
	}
	translation := ErrorTranslation{
		PolicyNotFound: own("T-1"), PolicyExists: own("T-2"), VersionMismatch: own("T-3"),
		PolicyDeclared: own("T-4"), InvalidLimit: own("T-5"), InvalidOffset: own("T-6"),
		InvalidPolicy: own("T-7"),
	}
	cases := map[string]tidcommon.ServiceError{
		"T-1": ErrorPolicyNotFound, "T-2": ErrorPolicyExists, "T-3": ErrorVersionMismatch,
		"T-4": ErrorPolicyDeclared, "T-5": ErrorInvalidLimit, "T-6": ErrorInvalidOffset,
		"T-7": ErrorReshareNotPermitted,
	}
	for want, from := range cases {
		translated := translation.Translate(&from)
		s.Require().NotNil(translated)
		s.Equal(want, translated.Code)
		s.Equal(from.ErrorDescription, translated.ErrorDescription)
	}

	s.Equal(&tidcommon.InternalServerError, translation.Translate(&tidcommon.InternalServerError))
	s.Nil(translation.Translate(nil))
}

func (suite *ModelTestSuite) TestPolicyRequestIsDeploymentWide() {
	everyOtherRoot := &OtherTreeRootsRequest{Mode: OtherTreeRootsAll}
	namedRoots := &OtherTreeRootsRequest{Mode: OtherTreeRootsSelected}
	for name, tc := range map[string]struct {
		req  PolicyRequest
		want bool
	}{
		"every organization unit":       {PolicyRequest{AllUnits: &AllUnitsRequest{}}, true},
		"every other tree's root":       {PolicyRequest{OtherTreeRoots: everyOtherRoot}, true},
		"named roots of other trees":    {PolicyRequest{OtherTreeRoots: namedRoots}, false},
		"everything beneath the issuer": {PolicyRequest{ChildUnits: &ChildUnitsRequest{Mode: ChildUnitsAll}}, false},
		"no section":                    {PolicyRequest{}, false},
	} {
		suite.Run(name, func() {
			suite.Equal(tc.want, tc.req.IsDeploymentWide())
		})
	}
}
