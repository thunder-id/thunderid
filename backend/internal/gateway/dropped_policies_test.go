// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type DroppedPoliciesTestSuite struct {
	suite.Suite
}

func TestDroppedPoliciesTestSuite(t *testing.T) {
	suite.Run(t, new(DroppedPoliciesTestSuite))
}

// dropped is the policies droppedPolicies lists, for the tests that ask only for those.
func dropped(held, sent []bundleResource) []DroppedPolicy {
	got, _ := droppedPolicies(held, sent)
	return got
}

func flowDoc(id string, policies ...string) bundleResource {
	lines := []string{"resource_type: flow", "id: " + id, "name: Flow", "nodes:", "  - id: start"}
	if len(policies) > 0 {
		lines = append(lines, "sharingPolicies:")
		lines = append(lines, policies...)
	}
	content := strings.Join(lines, "\n")
	return bundleResource{Type: "flow", ID: id, Name: "Flow", Content: content}
}

func (s *DroppedPoliciesTestSuite) TestPolicyInitiators() {
	doc := flowDoc("f1",
		"  - childUnits:", "      mode: all",
		"  - initiatingOuId: ou-a", "    childUnits:", "      mode: all",
		"  - childUnits:", "      mode: selected", "    initiatingOuId: \"ou-b\"")

	got, readable := policyInitiators(doc.Content)
	s.True(readable)
	s.Equal([]string{"", "ou-a", "ou-b"}, got)

	got, readable = policyInitiators(flowDoc("f2").Content)
	s.True(readable)
	s.Empty(got)
}

func (s *DroppedPoliciesTestSuite) TestPolicyInitiatorsReadsHowTheListIsWritten() {
	document := func(lines ...string) string {
		return strings.Join(append([]string{"resource_type: flow", "id: f1"}, lines...), "\n")
	}
	cases := map[string]struct {
		content string
		want    []string
	}{
		"a list at the left edge": {document("sharingPolicies:", "- childUnits:", "    mode: all",
			"- initiatingOuId: ou-a", "  childUnits:", "    mode: all", "nodes: []"), []string{"", "ou-a"}},
		"a comment at the left edge inside the list": {document("sharingPolicies:", "# a note",
			"  - childUnits:", "      mode: all", "# another", "  - initiatingOuId: ou-a", "    childUnits:",
			"      mode: all", "nodes: []"), []string{"", "ou-a"}},
		"a list in flow style": {document("sharingPolicies: [{childUnits: {mode: all}}, "+
			"{initiatingOuId: ou-a, childUnits: {mode: all}}]", "nodes: []"), []string{"", "ou-a"}},
		"an empty list in flow style": {document("sharingPolicies: []", "nodes: []"), []string{}},
		"a placeholder as the issuing unit": {document("sharingPolicies:", "  - initiatingOuId: {{.OU_ID}}",
			"    childUnits:", "      mode: all", "nodes: []"), []string{"{{.OU_ID}}"}},
		"a key that follows the list is not read": {document("sharingPolicies:", "  - childUnits:",
			"      mode: all", "nodes:", "  - initiatingOuId: x"), []string{""}},
	}
	for name, c := range cases {
		s.Run(name, func() {
			got, readable := policyInitiators(c.content)
			s.True(readable)
			s.Equal(c.want, got)
		})
	}
}

// An application writes its lists as template ranges, which are not YAML, and its policies are still read.
func (s *DroppedPoliciesTestSuite) TestPolicyInitiatorsReadsAPolicyInADocumentThatIsNotYAML() {
	content := strings.Join([]string{"resource_type: application", "id: a1", "redirectUris:",
		"{{- range .APP_REDIRECT_URIS}}", "  - {{.}}", "{{- end}}",
		"sharingPolicies:", "  - initiatingOuId: ou-a", "    childUnits:", "      mode: all"}, "\n")

	got, readable := policyInitiators(content)

	s.True(readable)
	s.Equal([]string{"ou-a"}, got)
}

// A list a template range writes cannot be read, which is not the same as holding no policies.
func (s *DroppedPoliciesTestSuite) TestPolicyInitiatorsReportsAListItCannotRead() {
	content := strings.Join([]string{"resource_type: flow", "id: f1", "sharingPolicies:",
		"{{- range .POLICIES}}", "  - childUnits:", "      mode: all", "{{- end}}"}, "\n")

	_, readable := policyInitiators(content)

	s.False(readable)
}

func (s *DroppedPoliciesTestSuite) TestDroppedPolicies() {
	owner := []string{"  - childUnits:", "      mode: all"}
	reshare := []string{"  - initiatingOuId: ou-a", "    childUnits:", "      mode: all"}
	both := append(append([]string{}, owner...), reshare...)

	s.Run("a policy the new document no longer carries is dropped", func() {
		got, _ := droppedPolicies([]bundleResource{flowDoc("f1", both...)}, []bundleResource{flowDoc("f1", owner...)})
		s.Equal([]DroppedPolicy{{ResourceType: "flow", ResourceID: "f1", ResourceName: "Flow",
			InitiatingOUID: "ou-a"}}, got)
	})
	s.Run("a document with no policies drops them all", func() {
		got, _ := droppedPolicies([]bundleResource{flowDoc("f1", both...)}, []bundleResource{flowDoc("f1")})
		s.Len(got, 2)
	})
	s.Run("an edited policy is not dropped", func() {
		edited := []string{"  - childUnits:", "      mode: selected"}
		s.Empty(dropped([]bundleResource{flowDoc("f1", owner...)},
			[]bundleResource{flowDoc("f1", edited...)}))
	})
	s.Run("a resource with no id is not listed, since it cannot be named to the gateway", func() {
		noID := func(policies ...string) bundleResource {
			resource := flowDoc("", policies...)
			resource.Content = strings.Replace(resource.Content, "id: \n", "", 1)
			return resource
		}
		s.Empty(dropped([]bundleResource{noID(owner...)}, []bundleResource{noID()}))
	})
	s.Run("a resource whose policies cannot be read is not compared and is reported", func() {
		unreadableDoc := flowDoc("f1")
		unreadableDoc.Content += "\nsharingPolicies:\n{{- range .POLICIES}}\n  - childUnits:\n{{- end}}"

		got, unreadable := droppedPolicies([]bundleResource{flowDoc("f1", both...)},
			[]bundleResource{unreadableDoc})

		s.Empty(got, "the document was read as having no policies, so every policy was named as dropped")
		s.Len(unreadable, 1)
		s.Equal("f1", unreadable[0].ID)
	})
	s.Run("a resource of a type a gateway applies no sharing for is not listed", func() {
		agent := func(policies ...string) bundleResource {
			resource := flowDoc("a1", policies...)
			resource.Type = "agent"
			resource.Content = strings.Replace(resource.Content, "resource_type: flow", "resource_type: agent", 1)
			return resource
		}
		s.Empty(dropped([]bundleResource{agent(owner...)}, []bundleResource{agent()}))
	})
	s.Run("a resource the gateway does not hold has none dropped", func() {
		s.Empty(dropped(nil, []bundleResource{flowDoc("f1")}))
	})
	s.Run("a resource not sent again is not compared", func() {
		s.Empty(dropped([]bundleResource{flowDoc("f1", both...)}, nil))
	})
}

func (s *DroppedPoliciesTestSuite) TestRejectUnacceptedRemovals() {
	removals := []gatewaySharingRemoval{{ResourceType: "flow", ID: "f1"}, {ResourceType: "flow", ID: "f1"}}
	status := func(answer json.RawMessage, resourceID string) string {
		var parsed struct {
			Results []map[string]string `json:"results"`
		}
		s.Require().NoError(json.Unmarshal(answer, &parsed))
		for _, result := range parsed.Results {
			if result["resourceId"] == resourceID && result["operation"] != "delete" {
				return result["status"]
			}
		}
		return ""
	}

	s.Run("a success the gateway did not stand behind becomes a refusal", func() {
		answer := json.RawMessage(`{"summary":{"failed":0},"results":[` +
			`{"resourceType":"flow","resourceId":"f1","status":"success","operation":"update"},` +
			`{"resourceType":"flow","resourceId":"f2","status":"success","operation":"update"}]}`)

		got := rejectUnacceptedRemovals(answer, removals)

		s.Equal("failed", status(got, "f1"))
		s.Equal("success", status(got, "f2"), "a resource with no removals is left as answered")
		s.Contains(string(got), sharingRemovalsNotAcceptedCode)
		s.Contains(string(got), `"summary"`, "what else the answer carried is kept")
	})
	s.Run("a resource the answer does not name is refused too", func() {
		got := rejectUnacceptedRemovals(json.RawMessage(`{"summary":{"failed":0},"results":[]}`), removals)

		s.Equal("failed", status(got, "f1"))
	})
	s.Run("a removal of the resource itself is not taken for its write", func() {
		answer := json.RawMessage(`{"results":[` +
			`{"resourceType":"flow","resourceId":"f1","status":"success","operation":"delete"}]}`)

		got := rejectUnacceptedRemovals(answer, removals)

		s.Equal("failed", status(got, "f1"), "the write was not named, so it is added as refused")
		s.Contains(string(got), `"operation":"delete"`)
	})
	s.Run("an answer that says the gateway accepted them is left as it is", func() {
		answer := json.RawMessage(`{"results":[{"resourceType":"flow","resourceId":"f1","status":"success"}],` +
			`"sharingRemovalsAccepted":true}`)

		s.JSONEq(string(answer), string(rejectUnacceptedRemovals(answer, removals)))
	})
	s.Run("no removals leaves the answer as it is", func() {
		answer := json.RawMessage(`{"results":[{"resourceType":"flow","resourceId":"f1","status":"success"}]}`)

		s.Equal(answer, rejectUnacceptedRemovals(answer, nil))
	})
	s.Run("an answer that cannot be read is left as it is", func() {
		answer := json.RawMessage(`not json`)

		s.Equal(answer, rejectUnacceptedRemovals(answer, removals))
	})
}
