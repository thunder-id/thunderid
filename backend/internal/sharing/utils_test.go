// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// testInvalidFormat stands in for the error a mounting resource type answers a malformed body with.
var testInvalidFormat = tidcommon.ServiceError{
	Type:  tidcommon.ClientErrorType,
	Code:  "TST-1001",
	Error: tidcommon.I18nMessage{Key: "error.test.invalid_request_format", DefaultValue: "Invalid request format"},
}

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (suite *UtilsTestSuite) TestDecodePolicyBody_ReadsAWellFormedBody() {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
		`{"initiatingOuId":"ou-1","childUnits":{"mode":"all","excludedOuIds":["ou-2"]}}`))

	body, ok := DecodePolicyBody[PolicyCreateBody](context.Background(), w, r, testInvalidFormat)

	suite.Require().True(ok)
	suite.Equal("ou-1", body.InitiatingOUID)
	suite.Require().NotNil(body.ChildUnits)
	suite.Equal([]string{"ou-2"}, body.ChildUnits.ExcludedOUIDs)
	suite.Equal(http.StatusOK, w.Code, "nothing is written for a body that decodes")
}

// A field the body does not declare is refused rather than dropped, and the refusal carries the
// mounting resource type's own error.
func (suite *UtilsTestSuite) TestDecodePolicyBody_RefusesAnUnknownField() {
	for name, raw := range map[string]string{
		"a misspelled exclusion": `{"childUnits":{"mode":"all","excludeOuIds":["ou-2"]}}`,
		"an issuer on an edit":   `{"version":1,"initiatingOuId":"ou-1","childUnits":{"mode":"all"}}`,
		"malformed JSON":         `{`,
	} {
		suite.Run(name, func() {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(raw))

			body, ok := DecodePolicyBody[PolicyUpdateBody](context.Background(), w, r, testInvalidFormat)

			suite.False(ok)
			suite.Nil(body)
			suite.Equal(http.StatusBadRequest, w.Code)
			var resp map[string]interface{}
			suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &resp))
			suite.Equal(testInvalidFormat.Code, resp["code"])
		})
	}
}

// policyDocument is a resource document's sharingPolicies, as a resource file or an import states it.
type policyDocument struct {
	SharingPolicies []PolicyRequest `yaml:"sharingPolicies"`
}

func (suite *UtilsTestSuite) TestPolicyRequestYAML_DecodesTheDeclaredFields() {
	var doc policyDocument
	err := yaml.Unmarshal([]byte(`
sharingPolicies:
  - id: p1
    childUnits:
      mode: selected
      units:
        - ouId: ou-a
          reach: unit
          overlayRules:
            resources: { editable: false, excludedValues: [billing] }
    otherTreeRoots:
      mode: all
      excludedOuIds: [partner-root]
`), &doc)

	suite.Require().NoError(err)
	suite.Require().Len(doc.SharingPolicies, 1)
	policy := doc.SharingPolicies[0]
	suite.Equal("p1", policy.ID)
	suite.Equal("ou-a", policy.ChildUnits.Units[0].OUID)
	suite.Equal([]string{"billing"}, *policy.ChildUnits.Units[0].OverlayRules["resources"].ExcludedValues)
	suite.Equal([]string{"partner-root"}, policy.OtherTreeRoots.ExcludedOUIDs)
}

func (suite *UtilsTestSuite) TestPolicyRequestYAML_DecodesMergeKeys() {
	var doc struct {
		Defaults       yaml.Node `yaml:"defaults"`
		Exclusion      yaml.Node `yaml:"exclusion"`
		policyDocument `yaml:",inline"`
	}
	err := yaml.Unmarshal([]byte(`
defaults: &defaults
  mode: all
exclusion: &exclusion
  excludedOuIds: [partner-root]
sharingPolicies:
  - id: p1
    otherTreeRoots:
      <<: *defaults
  - id: p2
    otherTreeRoots:
      <<: [*defaults, *exclusion]
`), &doc)

	suite.Require().NoError(err)
	suite.Require().Len(doc.SharingPolicies, 2)
	suite.Equal(OtherTreeRootsAll, doc.SharingPolicies[0].OtherTreeRoots.Mode)
	suite.Equal(OtherTreeRootsAll, doc.SharingPolicies[1].OtherTreeRoots.Mode)
	suite.Equal([]string{"partner-root"}, doc.SharingPolicies[1].OtherTreeRoots.ExcludedOUIDs)
}

// A merge key cannot bring in a key the policy does not declare, from a single mapping or from a
// sequence of them.
func (suite *UtilsTestSuite) TestPolicyRequestYAML_RefusesAnUnknownFieldThroughAMergeKey() {
	for name, merge := range map[string]string{
		"an alias":              "*misspelled",
		"a sequence of aliases": "[*valid, *misspelled]",
		"an inline mapping":     "{ exclude: [x] }",
	} {
		suite.Run(name, func() {
			var doc struct {
				Valid          yaml.Node `yaml:"valid"`
				Misspelled     yaml.Node `yaml:"misspelled"`
				policyDocument `yaml:",inline"`
			}
			err := yaml.Unmarshal([]byte(`
valid: &valid
  mode: all
misspelled: &misspelled
  exclude: [x]
sharingPolicies:
  - otherTreeRoots:
      <<: `+merge+`
`), &doc)
			suite.Require().Error(err)
			suite.Contains(err.Error(), `unknown sharing policy field "exclude"`)
		})
	}
}

func (suite *UtilsTestSuite) TestPolicyRequestYAML_AnAnchorMergingItselfInDoesNotLoop() {
	var node yaml.Node
	suite.Require().NoError(yaml.Unmarshal([]byte("&self { mode: all, <<: *self }\n"), &node))

	err := decodeYAMLStrict(node.Content[0], &OtherTreeRootsRequest{})

	suite.Require().Error(err)
}

// A key a policy does not declare is refused at every level, whether the whole document is decoded,
// as a resource file is, or one node of it, as an import does.
func (suite *UtilsTestSuite) TestPolicyRequestYAML_RefusesAnUnknownField() {
	const prefix = "sharingPolicies:\n  - "
	for name, tc := range map[string]struct{ doc, field string }{
		"on the policy":       {prefix + "targets: []\n", "targets"},
		"on all units":        {prefix + "allUnits: { excludeOuIds: [x] }\n", "excludeOuIds"},
		"on child units":      {prefix + "childUnits: { mode: all, excludeOuIds: [x] }\n", "excludeOuIds"},
		"on other tree roots": {prefix + "otherTreeRoots: { mode: all, exclude: [x] }\n", "exclude"},
		"on a named child": {
			prefix + "childUnits: { mode: selected, units: [ { ouId: a, reach: unit, scope: child } ] }\n",
			"scope",
		},
		"on a named root": {
			prefix + "otherTreeRoots: { mode: selected, units: [ { ouId: r, reach: unit } ] }\n",
			"reach",
		},
		"on an overlay rule": {
			prefix + "childUnits: { mode: all, overlayRules: { resources: { editabel: true } } }\n",
			"editabel",
		},
	} {
		suite.Run(name, func() {
			var doc policyDocument
			err := yaml.Unmarshal([]byte(tc.doc), &doc)
			suite.Require().Error(err)
			suite.Contains(err.Error(), `unknown sharing policy field "`+tc.field+`"`)

			var node yaml.Node
			suite.Require().NoError(yaml.Unmarshal([]byte(tc.doc), &node))
			suite.Require().Error(node.Content[0].Decode(&doc))
		})
	}
}
