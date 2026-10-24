// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

/*
One federated identity, two sign-ins.

A subject may be linked to several users. A sign-in checks for a link when it starts, so two sign-ins
for the same subject that both pause before writing would each find it free. These scenarios pause one
at a prompt, let the other record the link, then resume the first. A registration still refuses to
create a second user where one already holds the link, while a verified account is linked beside the
user that holds it, and a later sign-in asks which of the two to continue with.
*/

const (
	entityAlreadyExistsCode = "FET-1007"
	linkingWriteFailedCode  = "FET-1096"
	identifyFailedCode      = "FET-1002"
)

// pauseAtProvisioning starts a registration for user under a configuration that maps nothing onto the
// required attributes, so provisioning stops to ask for them.
func (s *FederatedMappingSuite) pauseAtProvisioning(user *testutils.OIDCUserInfo) *common.FlowStep {
	s.T().Helper()
	step := s.registerExpectingPrompt(mapping(fedPersonType.Handle), user)
	s.assertPromptsFor(step, "username")
	return step
}

// completeProvisioning answers the provisioning prompt with a fresh username and email.
func (s *FederatedMappingSuite) completeProvisioning(step *common.FlowStep) (*common.FlowStep, error) {
	s.T().Helper()
	name := s.nextSubject()
	return common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": name, "email": name + "@example.com"}, "", step.ChallengeToken)
}

// BC1: two registrations for one subject both reach provisioning. The first creates the user and the
// link; the second is refused as an existing user rather than creating a second user beside it.
func (s *FederatedMappingSuite) TestProvisioningRefusesAnIdentityLinkedMeanwhile() {
	user := s.baseUser(s.nextSubject())
	first := s.pauseAtProvisioning(user)
	second := s.pauseAtProvisioning(user)

	created, err := s.completeProvisioning(second)
	s.Require().NoError(err, "the second registration should provision")
	s.Require().Equal("COMPLETE", created.FlowStatus, "the second registration should provision, got %+v", created)
	provisioned, err := testutils.GetUserFromAssertion(created.Assertion)
	s.Require().NoError(err, "failed to look up the provisioned user")
	s.config.CreatedUserIDs = append(s.config.CreatedUserIDs, provisioned.ID)

	refused, err := s.completeProvisioning(first)
	s.Require().NoError(err, "a failed flow is still a 200 carrying its error, got %v", err)
	s.Require().Equal("ERROR", refused.FlowStatus, "the identity is linked already, got %+v", refused)
	s.Require().NotNil(refused.Error, "the terminal step should carry its error, got %+v", refused)
	s.Equal(entityAlreadyExistsCode, refused.Error.Code, "expected the existing user refusal, got %+v", refused.Error)
	s.Empty(refused.Assertion, "a refused registration must not issue an assertion")

	repeat, err := s.authenticate(s.authAppID, mapping(fedPersonType.Handle), user)
	s.Require().NoError(err, "the linked identity should authenticate")
	s.Require().Equal("COMPLETE", repeat.FlowStatus, "the linked identity should authenticate, got %+v", repeat)
	claims, err := testutils.DecodeJWT(repeat.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(provisioned.ID, claims.Sub, "the identity should stay with the user that linked it first")
}

// fedLinkResolvingFlow is fedVerifiedLinkingFlow with a resolve node between the federated node and the
// linking node, which asks which account to continue with when several hold the identity.
func fedLinkResolvingFlow(idpID, verifyFlowID string) testutils.Flow {
	flow := fedVerifiedLinkingFlow(idpID, verifyFlowID)
	flow.Handle = "auth_flow_federated_link_resolving"
	nodes := flow.Nodes.([]map[string]interface{})
	for _, node := range nodes {
		if node["id"] == "oidc_auth" {
			node["onSuccess"] = "link_resolver"
		}
	}
	flow.Nodes = append(nodes,
		map[string]interface{}{
			"id":           "link_resolver",
			"type":         "TASK_EXECUTION",
			"executor":     map[string]interface{}{"name": "IdentifyingExecutor", "mode": "resolve"},
			"onSuccess":    "linking",
			"onIncomplete": "link_picker",
		},
		map[string]interface{}{
			"id":   "link_picker",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{"action": map[string]interface{}{"ref": "action_pick", "nextNode": "link_resolver"}},
			},
		},
	)
	return flow
}

// BC2: a sign-in pauses on the linking prompt while another provisions a user for the same subject.
// Verifying the candidate afterwards links it beside that user, and a later sign-in asks which of the
// two accounts to continue with.
func (s *FederatedMappingSuite) TestLinkingBesideAnIdentityLinkedMeanwhile() {
	email, candidateID := s.linkTarget()
	user, config := s.matchingIdentity(email)
	verifyFlowID := s.createVerifyFlow(common.VerifyPasswordFlow("verify_flow_federated-verified-race"))
	appID := s.createScenarioApp(fedVerifiedLinkingFlow(s.idpID, verifyFlowID), "federated-verified-race")

	step, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the matched identity should reach the linking prompt")
	s.requireLinkingPrompt(step, email)

	// The same subject, now carrying an address no account holds, provisions a user and its link.
	other := *user
	other.Email = s.nextSubject() + "@example.com"
	holder := s.registerEntity(mapping(fedPersonType.Handle, pair("email", "email"), pair("email", "username")),
		&other)
	s.Require().NotEqual(candidateID, holder.ID, "the registration should provision a user of its own")

	verification, err := common.CompleteFlow(step.ExecutionID, nil, common.LinkAccountAction, step.ChallengeToken)
	s.Require().NoError(err, "confirming the match should route to the verification step")
	final, err := common.CompleteFlow(verification.ExecutionID,
		map[string]string{"username": email, "password": candidatePassword}, common.VerifyPasswordAction,
		verification.ChallengeToken)
	s.Require().NoError(err, "verifying the account should complete the flow")
	s.Require().Equal("COMPLETE", final.FlowStatus, "the verified account should be linked too, got %+v", final)
	claims, err := testutils.DecodeJWT(final.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(candidateID, claims.Sub, "the assertion should be issued for the verified account")

	// Both accounts now hold the identity, so the next sign-in asks which one to continue with.
	resolvingAppID := s.createScenarioApp(fedLinkResolvingFlow(s.idpID, verifyFlowID), "federated-link-resolving")
	pick, err := s.authenticateFlow(resolvingAppID, user.Sub)
	s.Require().NoError(err, "the identity linked to two accounts should reach the picker")
	s.Require().Equal("INCOMPLETE", pick.FlowStatus, "two linked accounts should be picked between, got %+v", pick)
	s.Empty(pick.Assertion, "no assertion may be issued before the account is picked")
	s.Require().True(common.HasInput(pick.Data.Inputs, "username"),
		"the accounts differ on username, so it should be offered, got %+v", pick.Data.Inputs)

	picked, err := common.CompleteFlow(pick.ExecutionID, map[string]string{"username": email}, "action_pick",
		pick.ChallengeToken)
	s.Require().NoError(err, "picking an account should complete the flow")
	s.Require().Equal("COMPLETE", picked.FlowStatus, "the picked account should sign in, got %+v", picked)
	claims, err = testutils.DecodeJWT(picked.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(candidateID, claims.Sub, "the assertion should be issued for the picked account")
}

// BC3: a linking attribute that more users share than a sign-in may choose between matches nobody,
// rather than offering an unbounded list to verify against.
func (s *FederatedMappingSuite) TestLinkingAttributeSharedByTooManyUsersFails() {
	costCenter := "CC-" + s.nextSubject()
	for i := 0; i <= 10; i++ {
		username := s.nextSubject()
		s.createLocalUser(map[string]interface{}{
			"username":   username,
			"email":      username + "@example.com",
			"costCenter": costCenter,
		})
	}
	user := s.baseUser(s.nextSubject())
	user.Custom["cost_centre"] = costCenter
	appID := s.passwordLinkingApp("federated-verified-crowded")

	step, err := s.authenticate(appID, linkOn([]string{"costCenter"}, pair("cost_centre", "costCenter")), user)
	s.Require().NoError(err, "a failed flow is still a 200 carrying its error, got %v", err)
	s.Require().Equal("ERROR", step.FlowStatus,
		"eleven matches are too many to verify against, got %+v", step)
	s.Require().NotNil(step.Error, "the terminal step should carry its error, got %+v", step)
	s.Equal(identifyFailedCode, step.Error.Code, "expected the identification failure, got %+v", step.Error)
}

// BC4: a user holds at most a hundred linked identities, so verifying the account for one more is
// refused at the write rather than growing the user's links without bound.
func (s *FederatedMappingSuite) TestLinkingBeyondTheLinkLimitIsRefused() {
	email, _ := s.linkTarget()
	appID := s.passwordLinkingApp("federated-verified-limit")
	s.applyConfig(linkOn([]string{"email"}, pair("email", "email"), pair("email", "username")))

	linkNext := func() *common.FlowStep {
		user := s.baseUser(s.nextSubject())
		user.Email = email
		s.mockOIDC.AddUser(user)
		step, err := s.authenticateFlow(appID, user.Sub)
		s.Require().NoError(err, "the matched identity should reach the linking prompt")
		s.Require().Equal("INCOMPLETE", step.FlowStatus, "expected the linking prompt, got %+v", step)
		verification, err := common.CompleteFlow(step.ExecutionID, nil, common.LinkAccountAction, step.ChallengeToken)
		s.Require().NoError(err, "confirming the match should route to the verification step")
		final, err := common.CompleteFlow(verification.ExecutionID,
			map[string]string{"username": email, "password": candidatePassword}, common.VerifyPasswordAction,
			verification.ChallengeToken)
		s.Require().NoError(err, "a failed flow is still a 200 carrying its error, got %v", err)
		return final
	}

	for i := 0; i < 100; i++ {
		linked := linkNext()
		s.Require().Equal("COMPLETE", linked.FlowStatus, "link %d should be recorded, got %+v", i+1, linked)
	}

	refused := linkNext()
	s.Require().Equal("ERROR", refused.FlowStatus, "the hundred and first link should be refused, got %+v", refused)
	s.Require().NotNil(refused.Error, "the terminal step should carry its error, got %+v", refused)
	s.Equal(linkingWriteFailedCode, refused.Error.Code, "expected the refused link write, got %+v", refused.Error)
}
