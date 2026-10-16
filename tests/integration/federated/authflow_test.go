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
user. On the authentication path the same mapped claims resolve an *existing* user instead, and that
resolution-through-a-flow is exercised nowhere else: the linking scenarios use the direct
/auth/{provider}/finish endpoints, which bypass the flow executors entirely.

The mapping computation is shared between the two paths, so what these assert is the consumption. BA2
and BA3 additionally cover the only branch in the executor that depends on the flow type at all.
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

// BA1: an identity with no matching sub resolves to an existing local user through the configured
// linking attribute, and the assertion is issued for that user rather than a new one.
func (s *FederatedMappingSuite) TestAuthenticationFlowLinksToExistingUserByMappedAttribute() {
	linkEmail := s.nextSubject() + "@example.com"
	existingID, err := testutils.CreateUser(testutils.User{
		Type: fedPersonType.Name,
		OUID: s.ouID,
		Attributes: mustJSON(map[string]interface{}{
			"username": linkEmail,
			"email":    linkEmail,
		}),
	})
	s.Require().NoError(err, "failed to create the user the identity should link to")
	defer func() {
		if err := testutils.DeleteUser(existingID); err != nil {
			s.T().Logf("failed to delete the linked user: %v", err)
		}
	}()

	// The identity's own subject matches nobody; only the mapped email can resolve it.
	user := s.baseUser(s.nextSubject())
	user.Email = linkEmail

	config := mapping(fedPersonType.Name, pair("email", "email"))
	config.AccountLinking = &testutils.AccountLinking{Attributes: []string{"email"}}

	// The strict flow carries no provisioning step, so a completed run proves the identity resolved to
	// the existing user rather than being created.
	step, err := s.authenticate(s.strictAuthAppID, config, user)
	s.Require().NoError(err, "the linked identity should authenticate cleanly")

	s.Require().Equal("COMPLETE", step.FlowStatus,
		"the identity should have linked to the existing user, got %+v", step)
	s.Require().NotEmpty(step.Assertion, "a completed authentication should carry an assertion")

	claims, err := testutils.DecodeJWT(step.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(existingID, claims.Sub,
		"the assertion should be issued for the pre-existing user, not a newly created one")
}

// BA2: an identity matching no local user proceeds past the federated node when the node allows
// authentication without one. This is the only executor branch that depends on the flow type.
func (s *FederatedMappingSuite) TestAuthenticationFlowWithoutLocalUserProceedsWhenAllowed() {
	user := s.baseUser(s.nextSubject())

	// Provisioning needs every required attribute, so the mapping supplies the username too; without it
	// the flow would stop to collect it, which Phase 3 already covers.
	config := mapping(fedPersonType.Name, pair("email", "email"), pair("email", "username"))

	step, err := s.authenticate(s.authAppID, config, user)

	s.Require().NoError(err, "an unmatched identity should be provisioned rather than failing")
	s.Require().Equal("COMPLETE", step.FlowStatus,
		"the identity should have been provisioned just in time, got %+v", step)

	provisioned, err := testutils.FindUserByAttribute("sub", user.Sub)
	s.Require().NoError(err, "failed to look up the provisioned user")
	s.Require().NotNil(provisioned, "the allowance should have provisioned a user for %s", user.Sub)
	s.config.CreatedUserIDs = append(s.config.CreatedUserIDs, provisioned.ID)
}

// BA3: the same identity against a flow whose federated node does not set the property. Without a local
// user and without the allowance there is nothing to authenticate, so the flow cannot complete.
func (s *FederatedMappingSuite) TestAuthenticationFlowWithoutLocalUserFailsWhenNotAllowed() {
	user := s.baseUser(s.nextSubject())

	step, err := s.authenticate(s.strictAuthAppID, mapping(fedPersonType.Name, pair("email", "email")), user)

	// Asserted exactly rather than "any failure": an unrelated OIDC, state or executor fault would
	// otherwise satisfy this test. The identity currently fails as an opaque HTTP 500 carrying
	// SSE-5000, which is the G18 behaviour; when that is corrected to a client error this assertion
	// fails loudly and is updated, which is the intent.
	s.Require().Error(err, "an unmatched identity must not authenticate, got %+v", step)
	s.Contains(err.Error(), "500", "expected the documented G18 internal error, got %v", err)
	s.Contains(err.Error(), "SSE-5000", "expected the documented G18 error code, got %v", err)

	unexpected, lookupErr := testutils.FindUserByAttribute("sub", user.Sub)
	s.Require().NoError(lookupErr, "failed to check whether a user was created")
	s.Nil(unexpected, "no user should be created when the flow does not allow authentication without one")
}
