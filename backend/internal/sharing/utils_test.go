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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
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

func (suite *UtilsTestSuite) TestSharedIntoCallerOU() {
	const (
		resourceID = "res-1"
		ownerOU    = "ou-owner"
		callerOU   = "ou-caller"
	)
	tests := []struct {
		name       string
		boundary   string
		refusal    *tidcommon.ServiceError
		visible    bool
		visibleErr *tidcommon.ServiceError
		asksShared bool
		wantOU     string
		wantErr    *tidcommon.ServiceError
	}{
		{name: "shared into the caller's unit", boundary: callerOU, refusal: &tidcommon.ErrorUnauthorized,
			visible: true, asksShared: true, wantOU: callerOU},
		{name: "not shared into the caller's unit", boundary: callerOU, refusal: &tidcommon.ErrorUnauthorized,
			asksShared: true},
		{name: "the caller's unit no longer exists", boundary: callerOU, refusal: &tidcommon.ErrorUnauthorized,
			visibleErr: &oupkg.ErrorOrganizationUnitNotFound, asksShared: true},
		{name: "the share cannot be checked", boundary: callerOU, refusal: &tidcommon.ErrorUnauthorized,
			visibleErr: &tidcommon.InternalServerError, asksShared: true, wantErr: &tidcommon.InternalServerError},
		{name: "the caller's unit owns the resource", boundary: ownerOU, refusal: &tidcommon.ErrorUnauthorized},
		{name: "a refusal a share cannot lift", boundary: callerOU, refusal: &tidcommon.InternalServerError},
	}
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
			authz.On("GetBoundary", mock.Anything).
				Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: tc.boundary}).Maybe()
			sharingSvc := NewSharingServiceInterfaceMock(suite.T())
			if tc.asksShared {
				sharingSvc.On("IsVisible", mock.MatchedBy(security.IsRuntimeContext), ResourceType("app"),
					resourceID, callerOU).Return(tc.visible, tc.visibleErr).Once()
			}

			ouID, svcErr := SharedIntoCallerOU(context.Background(), authz, sharingSvc, log.GetLogger(),
				ResourceType("app"), resourceID, ownerOU, tc.refusal)

			suite.Equal(tc.wantOU, ouID)
			suite.Equal(tc.wantErr, svcErr)
		})
	}
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

const ownPoliciesType ResourceType = "application"

// ListReadablePoliciesTestSuite covers listing the policies of a resource a caller may read.
type ListReadablePoliciesTestSuite struct {
	suite.Suite
	svc *SharingServiceInterfaceMock
}

func TestListReadablePoliciesTestSuite(t *testing.T) {
	suite.Run(t, new(ListReadablePoliciesTestSuite))
}

func (s *ListReadablePoliciesTestSuite) SetupTest() {
	s.svc = NewSharingServiceInterfaceMock(s.T())
}

// mayReadOU lets the caller read in the given organization units only.
func mayReadOU(ouIDs ...string) func(string) *tidcommon.ServiceError {
	return func(ouID string) *tidcommon.ServiceError {
		for _, allowed := range ouIDs {
			if allowed == ouID {
				return nil
			}
		}
		return &tidcommon.ErrorUnauthorized
	}
}

// untranslated returns the sharing error as it is, so a test sees which errors went through translate.
func untranslated(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError { return svcErr }

func (s *ListReadablePoliciesTestSuite) policies(ouIDs ...string) {
	policies := make([]Policy, 0, len(ouIDs))
	for i, ouID := range ouIDs {
		policies = append(policies, Policy{ID: string(rune('a' + i)), InitiatingOUID: ouID})
	}
	s.svc.On("ListPolicies", mock.Anything, ownPoliciesType, "res-1").Return(policies, nil)
}

func (s *ListReadablePoliciesTestSuite) TestKeepsTheCallersOwn() {
	s.policies("ou-1", "ou-2", "ou-1")

	list, svcErr := ListReadablePolicies(context.Background(), s.svc, ownPoliciesType, "res-1", "owner-ou", 10, 0,
		mayReadOU("ou-1"), untranslated)
	s.Require().Nil(svcErr)
	s.Equal(2, list.TotalResults)
	s.Equal(2, list.Count)
	s.Equal(1, list.StartIndex)
	s.Equal([]string{"a", "c"}, []string{list.Policies[0].ID, list.Policies[1].ID})
}

func (s *ListReadablePoliciesTestSuite) TestPages() {
	s.policies("ou-1", "ou-1", "ou-1")

	list, svcErr := ListReadablePolicies(context.Background(), s.svc, ownPoliciesType, "res-1", "owner-ou", 2, 2,
		mayReadOU("ou-1"), untranslated)
	s.Require().Nil(svcErr)
	s.Equal(3, list.TotalResults)
	s.Equal(1, list.Count)
	s.Equal("c", list.Policies[0].ID)

	list, svcErr = ListReadablePolicies(context.Background(), s.svc, ownPoliciesType, "res-1", "owner-ou", 2, 5,
		mayReadOU("ou-1"), untranslated)
	s.Require().Nil(svcErr)
	s.Equal(3, list.TotalResults)
	s.Empty(list.Policies)
}

func (s *ListReadablePoliciesTestSuite) TestRefusesACallerWhoMadeNone() {
	s.policies("ou-2")

	_, svcErr := ListReadablePolicies(context.Background(), s.svc, ownPoliciesType, "res-1", "owner-ou", 10, 0,
		mayReadOU("ou-1"), untranslated)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ListReadablePoliciesTestSuite) TestPassesOnAReadFailure() {
	s.policies("ou-1")

	_, svcErr := ListReadablePolicies(context.Background(), s.svc, ownPoliciesType, "res-1", "owner-ou", 10, 0,
		func(ouID string) *tidcommon.ServiceError {
			if ouID == "owner-ou" {
				return &tidcommon.ErrorUnauthorized
			}
			return &tidcommon.InternalServerError
		},
		func(*tidcommon.ServiceError) *tidcommon.ServiceError { return &ErrorPolicyNotFound })
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ListReadablePoliciesTestSuite) TestTranslatesSharingErrors() {
	s.svc.On("ListPolicies", mock.Anything, ownPoliciesType, "res-1").Return(nil, &ErrorPolicyNotFound)
	translated := &tidcommon.ServiceError{Code: "APP-0000", Type: tidcommon.ClientErrorType}
	translate := func(*tidcommon.ServiceError) *tidcommon.ServiceError { return translated }

	_, svcErr := ListReadablePolicies(context.Background(), s.svc, ownPoliciesType, "res-1", "owner-ou", 10, 0,
		mayReadOU("ou-1"), translate)
	s.Equal(translated, svcErr)

	_, svcErr = ListReadablePolicies(context.Background(), s.svc, ownPoliciesType, "res-1", "owner-ou", 0, 0,
		mayReadOU("ou-1"), translate)
	s.Equal(translated, svcErr)
}

func (s *ListReadablePoliciesTestSuite) TestReadsEveryPolicyForAnOwnerReader() {
	page := PolicyList{TotalResults: 3, Count: 1, Policies: []Policy{{ID: "a"}}}
	s.svc.On("GetPolicyList", mock.Anything, ownPoliciesType, "res-1", 1, 0).Return(page, nil)

	list, svcErr := ListReadablePolicies(context.Background(), s.svc, ownPoliciesType, "res-1", "owner-ou", 1, 0,
		mayReadOU("owner-ou"), untranslated)
	s.Require().Nil(svcErr)
	s.Equal(page, list)
}

func (s *ListReadablePoliciesTestSuite) TestTranslatesAnOwnerReadFailure() {
	s.svc.On("GetPolicyList", mock.Anything, ownPoliciesType, "res-1", 10, 0).
		Return(PolicyList{}, &ErrorPolicyNotFound)
	translated := &tidcommon.ServiceError{Code: "APP-0000", Type: tidcommon.ClientErrorType}

	_, svcErr := ListReadablePolicies(context.Background(), s.svc, ownPoliciesType, "res-1", "owner-ou", 10, 0,
		mayReadOU("owner-ou"), func(*tidcommon.ServiceError) *tidcommon.ServiceError { return translated })
	s.Equal(translated, svcErr)
}

func (s *ListReadablePoliciesTestSuite) TestPassesOnAnOwnerCheckFailure() {
	_, svcErr := ListReadablePolicies(context.Background(), s.svc, ownPoliciesType, "res-1", "owner-ou", 10, 0,
		func(string) *tidcommon.ServiceError { return &tidcommon.InternalServerError }, untranslated)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}
