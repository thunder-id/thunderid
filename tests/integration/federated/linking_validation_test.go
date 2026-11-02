// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

/*
Configurations that account linking refuses to store.

A linking node and a connection's linking attributes are both checked when they are saved, so a graph
or a configuration that would link the wrong account, or never settle, is rejected up front instead of
failing a sign-in later. Each scenario breaks one rule on an otherwise valid configuration and asserts
the refusal names that rule.
*/

const (
	invalidExecutorConfigCode  = "FLM-1023"
	invalidAttributeConfigCode = "IDP-1012"
)

// linkingFlowWith returns fedVerifiedLinkingFlow under a handle of its own with edit applied to its
// nodes, keyed by node id. A node edit deletes from the map is dropped, and one it adds is appended.
func (s *FederatedMappingSuite) linkingFlowWith(handle string,
	edit func(nodes map[string]map[string]interface{})) testutils.Flow {
	s.T().Helper()
	flow := fedVerifiedLinkingFlow(s.idpID, "unused-verify-flow")
	flow.Handle = handle
	original := flow.Nodes.([]map[string]interface{})
	nodes := map[string]map[string]interface{}{}
	for _, node := range original {
		nodes[node["id"].(string)] = node
	}
	edit(nodes)

	edited := make([]map[string]interface{}, 0, len(nodes))
	for _, node := range original {
		if id := node["id"].(string); nodes[id] != nil {
			edited = append(edited, nodes[id])
			delete(nodes, id)
		}
	}
	for _, node := range nodes {
		edited = append(edited, node)
	}
	flow.Nodes = edited
	return flow
}

// requireFlowRejected asserts the flow is refused for the linking rule whose message contains reason.
func (s *FederatedMappingSuite) requireFlowRejected(flow testutils.Flow, reason string) {
	s.T().Helper()
	flowID, err := testutils.CreateFlow(flow)
	if err == nil {
		s.perTestFlowIDs = append(s.perTestFlowIDs, flowID)
	}
	s.Require().Error(err, "the flow should be refused")
	s.Contains(err.Error(), invalidExecutorConfigCode, "expected the executor configuration error, got %v", err)
	s.Contains(err.Error(), reason, "expected the refusal to name the broken rule, got %v", err)
}

// linkingPrompt returns the prompt actions of the linking prompt node.
func linkingPrompt(nodes map[string]map[string]interface{}) []map[string]interface{} {
	return nodes["linking_prompt"]["prompts"].([]map[string]interface{})
}

// BV1: a linking node with nowhere to send the End-User for verification is refused.
func (s *FederatedMappingSuite) TestLinkingFlowWithoutOnIncompleteIsRefused() {
	s.requireFlowRejected(s.linkingFlowWith("fed_linking_no_incomplete", func(nodes map[string]map[string]interface{}) {
		delete(nodes["linking"], "onIncomplete")
		delete(nodes, "linking_prompt")
		delete(nodes, "verify_account")
	}), "requires onIncomplete")
}

// BV2: a refusal that does not return to the linking node never reaches it, so it is refused.
func (s *FederatedMappingSuite) TestLinkingRejectActionNotReturningIsRefused() {
	s.requireFlowRejected(s.linkingFlowWith("fed_linking_reject_misrouted", func(nodes map[string]map[string]interface{}) {
		linkingPrompt(nodes)[1]["action"].(map[string]interface{})["nextNode"] = "provisioning"
	}), "must point back to this node")
}

// BV3: a verification flow called from the prompt must hand back to the linking node.
func (s *FederatedMappingSuite) TestLinkingVerificationCallNotReturningIsRefused() {
	s.requireFlowRejected(s.linkingFlowWith("fed_linking_call_misrouted", func(nodes map[string]map[string]interface{}) {
		nodes["verify_account"]["onSuccess"] = "provisioning"
	}), "must return to this node through onSuccess")
}

// BV4: verification steps in the flow itself must not sign in before the linking node checks who
// verified.
func (s *FederatedMappingSuite) TestLinkingVerificationSegmentSigningInIsRefused() {
	s.requireFlowRejected(s.linkingFlowWith("fed_linking_segment_asserts", func(nodes map[string]map[string]interface{}) {
		linkingPrompt(nodes)[0]["action"].(map[string]interface{})["nextNode"] = "auth_assert"
		delete(nodes, "verify_account")
	}), "must not end the flow, link, provision or sign in")
}

// BV5: a failed link that loops back to the linking node without signing in again would retry with
// the same identity, so it is refused.
func (s *FederatedMappingSuite) TestLinkingFailurePathReturningIsRefused() {
	s.requireFlowRejected(s.linkingFlowWith("fed_linking_failure_loops", func(nodes map[string]map[string]interface{}) {
		nodes["linking"]["onFailure"] = "linking_prompt"
	}), "must sign in at a connection again")
}

// BV6: a failure path that signs in at the connection again before returning is accepted.
func (s *FederatedMappingSuite) TestLinkingFailurePathThroughSignInIsAccepted() {
	flowID, err := testutils.CreateFlow(s.linkingFlowWith("fed_linking_failure_signs_in",
		func(nodes map[string]map[string]interface{}) {
			nodes["linking"]["onFailure"] = "link_failed"
			nodes["link_failed"] = map[string]interface{}{
				"id":   "link_failed",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{"action": map[string]interface{}{"ref": "sign_in_again", "nextNode": "oidc_auth"}},
				},
			}
			nodes["verify_account"]["flow"] = map[string]interface{}{
				"ref": s.createVerifyFlow(common.VerifyPasswordFlow("verify_flow_failure_signs_in"))}
		}))
	s.Require().NoError(err, "a failure path through the federated sign-in should be accepted")
	s.perTestFlowIDs = append(s.perTestFlowIDs, flowID)
}

// requireConfigRejected asserts the connection refuses config for the linking rule whose message
// contains reason, and keeps what it had.
func (s *FederatedMappingSuite) requireConfigRejected(config *testutils.AttributeConfiguration, reason string) {
	s.T().Helper()
	err := testutils.UpdateIDP(s.idpID, s.idpFixture(config))
	s.Require().Error(err, "the configuration should be refused")
	s.Contains(err.Error(), invalidAttributeConfigCode, "expected the attribute configuration error, got %v", err)
	s.Contains(err.Error(), reason, "expected the refusal to name the broken rule, got %v", err)
}

// BV7: a blank linking attribute names nothing to match on.
func (s *FederatedMappingSuite) TestBlankLinkingAttributeIsRefused() {
	s.requireConfigRejected(linkOn([]string{" "}, pair("email", "email")), "must not be empty")
}

// BV8: a server-owned attribute cannot be matched on.
func (s *FederatedMappingSuite) TestReservedLinkingAttributeIsRefused() {
	s.requireConfigRejected(linkOn([]string{"linkedIds"}, pair("email", "email")),
		"'linkedIds' is reserved")
}

// BV9: a claim path is not a local attribute name, so it links only where a mapping names one for
// every user type.
func (s *FederatedMappingSuite) TestUnmappedClaimPathLinkingAttributeIsRefused() {
	s.requireConfigRejected(linkOn([]string{"address.locality"}, pair("email", "email")),
		"must be mapped to a local attribute for every user type")

	partly := linkOn([]string{"address.locality"}, pair("address.locality", "firstName"))
	partly.UserTypeAttributeMappings = append(partly.UserTypeAttributeMappings,
		testutils.UserTypeAttributeMapping{
			UserType: fedContractorType.Handle, Attributes: []testutils.AttributeMapping{pair("email", "email")}})
	s.requireConfigRejected(partly, "must be mapped to a local attribute for every user type")
}

// BV10: a claim path mapped for every user type is accepted.
func (s *FederatedMappingSuite) TestMappedClaimPathLinkingAttributeIsAccepted() {
	config := linkOn([]string{"address.locality"}, pair("address.locality", "firstName"))
	config.UserTypeAttributeMappings = append(config.UserTypeAttributeMappings,
		testutils.UserTypeAttributeMapping{
			UserType:   fedContractorType.Handle,
			Attributes: []testutils.AttributeMapping{pair("address.locality", "firstName")},
		})
	s.Require().NoError(testutils.UpdateIDP(s.idpID, s.idpFixture(config)),
		"a claim path mapped for every user type should be accepted")
}

// BV11: linking attributes that fan out into more lookups than a sign-in may run are refused.
func (s *FederatedMappingSuite) TestLinkingAttributesWithTooManyLookupsAreRefused() {
	s.requireConfigRejected(linkOn([]string{"email", "given_name"},
		pair("email", "email"), pair("email", "username"), pair("email", "firstName"), pair("email", "lastName"),
		pair("given_name", "city"), pair("given_name", "costCenter"), pair("given_name", "mobile_number"),
	), "lookups for user type")
}
