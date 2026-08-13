// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

/*
Executor misconfiguration.

A federated node depends on a connection it names by id, and on that connection still existing and being
of the type the executor expects. These cover what a deployment sees when one of those assumptions is
wrong — the failures an administrator meets while wiring a flow up, rather than anything a user does.
*/

// federatedNodeFlow builds a single-node authentication graph with the given node properties, so a
// scenario can express exactly the misconfiguration it is about.
func federatedNodeFlow(handle, executor string, properties map[string]interface{}) testutils.Flow {
	return testutils.Flow{
		Name:     "Misconfigured " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "federated"},
			{
				"id":         "federated",
				"type":       "TASK_EXECUTION",
				"properties": properties,
				"executor":   map[string]interface{}{"name": executor},
				"onSuccess":  "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}

// BE1: the node names no connection, or names one that is not a string.
//
// A missing or empty idpId is caught when the flow is authored, not when it runs: flow creation rejects
// it as FLM-1023. That is the better outcome and the one worth pinning — an administrator cannot even
// save the broken graph. A non-string value passes that check and fails later, so the two halves assert
// different things.
func (s *FederatedMappingSuite) TestFederatedNodeWithoutUsableIdpID() {
	for name, properties := range map[string]map[string]interface{}{
		"missing": {},
		"empty":   {"idpId": ""},
	} {
		s.Run(name+"_rejected_at_authoring", func() {
			_, err := testutils.CreateFlow(
				federatedNodeFlow("auth_flow_bad_idp_"+name, "OIDCAuthExecutor", properties))

			s.Require().Error(err, "a federated node with a %s idpId should not be saveable", name)
			s.Contains(err.Error(), "FLM-1023",
				"expected the executor-configuration rejection, got %v", err)
			s.Contains(err.Error(), "idpId", "the rejection should name the missing property, got %v", err)
		})
	}

	s.Run("non_string_fails_at_runtime", func() {
		appID := s.createScenarioApp(
			federatedNodeFlow("auth_flow_bad_idp_non_string", "OIDCAuthExecutor",
				map[string]interface{}{"idpId": 42}),
			"bad-idp-non-string")

		step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")

		// Asserted rather than tolerated: accepting any error would let an unrelated HTTP or server
		// fault satisfy this. The executor cannot resolve a connection, so the failure is a server-side
		// one and the flow never reaches a provider.
		s.Require().Error(err, "a non-string idpId must not start a federated exchange")
		s.Contains(err.Error(), "500",
			"expected the executor's own failure to resolve the connection, got %v", err)
		s.Nil(step, "no flow step should be returned when the node cannot be resolved")
	})
}

// BE2: the node names a connection of a different type from the executor driving it.
//
// Nothing rejects the pairing. A GitHub connection carries the same property names an OIDC connection
// does — its endpoints are defaults rather than configured values — so the OIDC executor reads them and
// issues a perfectly ordinary redirect. The mismatch only surfaces later, when the exchange produces no
// ID token, which means an administrator discovers it from users failing to sign in rather than from
// anything at authoring time. Recorded as G20.
func (s *FederatedMappingSuite) TestFederatedNodeWithMismatchedConnectionType() {
	appID := s.createScenarioApp(
		federatedNodeFlow("auth_flow_type_mismatch", "OIDCAuthExecutor",
			map[string]interface{}{"idpId": s.githubIDPID}),
		"type-mismatch")

	s.activeSub = s.githubIdentity(nil, []*testutils.GithubEmail{
		{Email: s.nextSubject() + "@example.com", Primary: true, Verified: true},
	})

	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	s.Require().NoError(err, "the mismatch is not caught here: the redirect is built from properties "+
		"a GitHub connection also has")
	s.Require().Equal("REDIRECTION", step.Type, "expected a redirect, got %+v", step)

	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization")

	completed, err := common.CompleteFlow(
		step.ExecutionID, map[string]string{"code": code, "state": state}, "", step.ChallengeToken)

	s.assertNotAuthenticated(completed, err,
		"an OIDC exchange against a GitHub connection must not authenticate")
}

// BE3: a connection cannot be deleted while a flow still references it, which is what prevents the
// mid-session case this scenario was originally written for.
//
// The plan expected to delete a connection between the redirect and the callback and assert the callback
// failed. That situation cannot arise: deletion is refused with IDP-1013 while any flow depends on the
// connection, so an administrator cannot pull it out from under a user mid-session in the first place.
// Asserting the guard is more useful than asserting the consequence of a state the product prevents —
// and the redirect is issued first, so the guard is shown holding while an exchange is genuinely in
// flight rather than merely at rest.
func (s *FederatedMappingSuite) TestConnectionInUseCannotBeDeletedMidExchange() {
	fixture := s.idpFixture(nil)
	fixture.Name = "Federated Throwaway Connection " + s.nextSubject()
	throwaway, err := testutils.CreateIDP(fixture)
	s.Require().NoError(err, "failed to create the throwaway connection")

	appID := s.createScenarioApp(
		federatedNodeFlow("auth_flow_deleted_conn", "OIDCAuthExecutor",
			map[string]interface{}{"idpId": throwaway}),
		"deleted-conn")

	user := s.knownOIDCIdentity()
	s.activeSub = user.Sub

	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	s.Require().NoError(err, "the flow should start")
	s.Require().Equal("REDIRECTION", step.Type, "expected a redirect, got %+v", step)

	// The user is now at the provider. Removing the connection underneath them is refused.
	err = testutils.DeleteIDP(throwaway)
	s.Require().Error(err, "a connection a flow depends on should not be deletable")
	s.Contains(err.Error(), "IDP-1013", "expected the blocking-dependency refusal, got %v", err)

	// The exchange in flight is unaffected, which is the point of the guard.
	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization")
	completed, err := common.CompleteFlow(
		step.ExecutionID, map[string]string{"code": code, "state": state}, "", step.ChallengeToken)
	s.Require().NoError(err, "the in-flight exchange should still complete")
	s.Equal("COMPLETE", completed.FlowStatus, "got %+v", completed)

	// The connection outlives the test's application, so it is removed once the flow referencing it is.
	s.T().Cleanup(func() {
		if err := testutils.DeleteIDP(throwaway); err != nil {
			s.T().Logf("failed to delete the throwaway connection: %v", err)
		}
	})
}
