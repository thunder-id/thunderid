// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

/*
Federated authentication through a flow.

Phase 3 covers mapping and resolution on the registration path, where a mapped claim populates a new
user. On the authentication path the same mapped claims match an *existing* user instead, which the
linking scenarios cover.

BA2 and BA3 cover the only branch in the executor that depends on the flow type at all.
*/

// authenticate drives an authentication flow for a mock identity and returns the terminal step.
func (s *FederatedMappingSuite) authenticate(
	appID string, config *testutils.AttributeConfiguration,
	user *testutils.OIDCUserInfo) (*common.FlowStep, error) {
	s.T().Helper()
	s.applyConfig(config)
	s.mockOIDC.AddUser(user)
	s.activeSub = user.Sub

	flowStep, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	s.Require().NoError(err, "failed to initiate the authentication flow")
	s.Require().Equal("REDIRECTION", flowStep.Type,
		"expected a redirection to the identity provider, got %+v", flowStep)

	code, state, err := testutils.SimulateFederatedOAuthFlow(flowStep.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")

	// The error is returned rather than asserted away: a flow that cannot authenticate is a valid
	// outcome here, and how it fails is what two of these scenarios are about.
	step, err := common.CompleteFlow(
		flowStep.ExecutionID, map[string]string{"code": code, "state": state}, "", flowStep.ChallengeToken)
	return step, err
}

// BA2: an identity matching no local user proceeds past the federated node when the node allows
// authentication without one. This is the only executor branch that depends on the flow type.
func (s *FederatedMappingSuite) TestAuthenticationFlowWithoutLocalUserProceedsWhenAllowed() {
	user := s.baseUser(s.nextSubject())

	// Provisioning needs every required attribute, so the mapping supplies the username too; without it
	// the flow would stop to collect it, which Phase 3 already covers.
	config := mapping(fedPersonType.Handle, pair("email", "email"), pair("email", "username"))

	step, err := s.authenticate(s.authAppID, config, user)

	s.Require().NoError(err, "an unmatched identity should be provisioned rather than failing")
	s.Require().Equal("COMPLETE", step.FlowStatus,
		"the identity should have been provisioned just in time, got %+v", step)

	provisioned, err := testutils.GetUserFromAssertion(step.Assertion)
	s.Require().NoError(err, "the allowance should have provisioned a user for %s", user.Sub)
	s.config.CreatedUserIDs = append(s.config.CreatedUserIDs, provisioned.ID)
}

// BA3: the same identity against a flow whose federated node does not set the property. Without a local
// user and without the allowance there is nothing to authenticate, so the flow cannot complete.
func (s *FederatedMappingSuite) TestAuthenticationFlowWithoutLocalUserFailsWhenNotAllowed() {
	user := s.baseUser(s.nextSubject())

	step, err := s.authenticate(s.strictAuthAppID, mapping(fedPersonType.Handle, pair("email", "email")), user)

	// Asserted exactly rather than "any failure": an unrelated OIDC, state or executor fault would
	// otherwise satisfy this test. An identity that names no local user is the caller's input, so it
	// terminates the flow in ERROR carrying the client error code, not an opaque HTTP 500 with
	// SSE-5000, which is what the lost error classification used to produce.
	s.Require().NoError(err, "a failed flow is still a 200 carrying its error, got %v", err)
	s.Require().NotNil(step, "expected a terminal step for the unmatched identity")
	s.Require().Equal("ERROR", step.FlowStatus, "an unmatched identity must not authenticate, got %+v", step)
	s.Require().NotNil(step.Error, "expected the terminal step to carry its error, got %+v", step)
	s.Equal("FET-1002", step.Error.Code, "expected the client error for an unidentifiable user, got %+v", step.Error)

	unexpected, lookupErr := testutils.FindUserByAttribute("email", user.Email)
	s.Require().NoError(lookupErr, "failed to check whether a user was created")
	s.Nil(unexpected, "no user should be created when the flow does not allow authentication without one")
}
