// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

/*
Account linking attributes, observed through a flow.

A match is visible as the linking prompt: the linking node forwards a matched account there and names
the account it offers, so the address on the prompt *is* the result. The graph has no provisioning
step, so an identity that matches nobody ends the flow without an assertion.

The direct /auth/oauth/{provider}/finish endpoints cannot verify an account before linking it, so they resolve a
federated identity only through a recorded link, which BR12 pins. They use
/auth/oauth/standard/*, which is the only direct federated route. An OIDC connection reaches it through
the cross-type allowance in validateIDPType.
*/

const (
	directAuthStart  = "/auth/oauth/standard/start"
	directAuthFinish = "/auth/oauth/standard/finish"
)

// authenticateDirect drives the direct endpoints and returns the finish status, the decoded response and
// the error code when the response carries one.
func (s *FederatedMappingSuite) authenticateDirect(
	config *testutils.AttributeConfiguration, user *testutils.OIDCUserInfo,
) (int, testutils.AuthenticationResponse, string) {
	s.T().Helper()
	s.applyConfig(config)
	s.mockOIDC.AddUser(user)
	return s.authenticateDirectVia(s.idpID, user.Sub)
}

// authenticateDirectVia drives the direct endpoints against any connection, for the OAuth scenarios
// whose identities live on a different mock.
func (s *FederatedMappingSuite) authenticateDirectVia(
	idpID, sub string,
) (int, testutils.AuthenticationResponse, string) {
	s.T().Helper()
	s.activeSub = sub

	status, body := s.postJSON(directAuthStart, map[string]interface{}{"idpId": idpID})
	s.Require().Equal(http.StatusOK, status, "failed to start federated authentication: %s", string(body))

	var start struct {
		SessionToken string `json:"sessionToken"`
		RedirectURL  string `json:"redirectUrl"`
	}
	s.Require().NoError(json.Unmarshal(body, &start))

	code, _, err := testutils.SimulateFederatedOAuthFlow(start.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")

	status, body = s.postJSON(directAuthFinish, map[string]interface{}{
		"sessionToken": start.SessionToken,
		"code":         code,
	})

	var response testutils.AuthenticationResponse
	var failure struct {
		Code string `json:"code"`
	}
	if status == http.StatusOK {
		s.Require().NoError(json.Unmarshal(body, &response), "failed to decode: %s", string(body))
	} else {
		_ = json.Unmarshal(body, &failure)
	}
	return status, response, failure.Code
}

func (s *FederatedMappingSuite) postJSON(path string, body interface{}) (int, []byte) {
	s.T().Helper()
	payload, err := json.Marshal(body)
	s.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+path, bytes.NewReader(payload))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return resp.StatusCode, responseBody
}

// createLocalUser creates a fed_person and registers it for cleanup after the test.
func (s *FederatedMappingSuite) createLocalUser(attributes map[string]interface{}) string {
	s.T().Helper()
	userID, err := testutils.CreateUser(testutils.User{
		Type:       fedPersonType.Handle,
		OUID:       s.ouID,
		Attributes: mustJSON(attributes),
	})
	s.Require().NoError(err, "failed to create local user %v", attributes)
	s.config.CreatedUserIDs = append(s.config.CreatedUserIDs, userID)
	return userID
}

// linkOn builds a configuration that maps the claims a scenario needs and links on the named attributes.
func linkOn(attributes []string, pairs ...testutils.AttributeMapping) *testutils.AttributeConfiguration {
	config := mapping(fedPersonType.Handle, pairs...)
	config.AccountLinking = &testutils.AccountLinking{Attributes: attributes}
	return config
}

// linkPassword is what a scenario proves a matched account with when it records a link.
const linkPassword = "Linked#Secret1"

// matchingApp creates a scenario application whose flow runs the named federated executor at the
// connection, then the linking node, with no provisioning behind it.
func (s *FederatedMappingSuite) matchingApp(executorName, idpID, clientID string) string {
	s.T().Helper()
	return s.createScenarioApp(common.VerifiedLinkingFlow("auth_flow_"+clientID, executorName, idpID,
		s.createVerifyFlow(common.VerifyPasswordFlow("verify_flow_"+clientID))), clientID)
}

// promptedDetails returns the rows the linking prompt shows, label to value, or nil when the step is
// not that prompt. The rows are the linking attribute values the identity matched an account on, so
// every scenario here creates a single account those values can reach.
func promptedDetails(step *common.FlowStep) map[string]string {
	if step == nil || step.FlowStatus != "INCOMPLETE" {
		return nil
	}
	var details []struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}
	if json.Unmarshal([]byte(step.Data.AdditionalData["linkingPromptDetails"]), &details) != nil ||
		len(details) == 0 {
		return nil
	}
	rows := make(map[string]string, len(details))
	for _, detail := range details {
		rows[detail.Label] = detail.Value
	}
	return rows
}

// matchedOn applies a configuration, signs the identity in through the OIDC connection, and returns
// the linking attribute values it matched an account on, or nil when they matched nobody.
func (s *FederatedMappingSuite) matchedOn(
	config *testutils.AttributeConfiguration, user *testutils.OIDCUserInfo) map[string]string {
	s.T().Helper()
	details, err := s.tryMatchedOn(config, user)
	s.Require().NoError(err, "the federated sign-in should be answered")
	return details
}

// tryMatchedOn is matchedOn for callers that treat a failed sign-in as a rejection.
func (s *FederatedMappingSuite) tryMatchedOn(
	config *testutils.AttributeConfiguration, user *testutils.OIDCUserInfo) (map[string]string, error) {
	s.T().Helper()
	s.applyConfig(config)
	s.mockOIDC.AddUser(user)
	step, err := s.authenticateFlow(
		s.matchingApp("OIDCAuthExecutor", s.idpID, "federated-match-"+user.Sub), user.Sub)
	if err != nil {
		return nil, err
	}
	return promptedDetails(step), nil
}

// recordLink records the (connection, subject) link for the local account the connection currently
// matches, the way an End-User would: through a verified linking flow, proving the account with its
// username and linkPassword. The mock must already hold the identity and the connection must already
// be configured.
func (s *FederatedMappingSuite) recordLink(executorName, idpID, sub, username string) {
	s.T().Helper()
	s.activeSub = sub
	s.Require().NoError(common.LinkAccount(common.LinkRequest{
		Handle:       "fed-link-" + sub,
		ExecutorName: executorName,
		IDPID:        idpID,
		OUID:         s.ouID,
		UserType:     fedPersonType.Handle,
		Username:     username,
		Password:     linkPassword,
	}), "failed to record the federated link")
}

// B10: a federated identity is matched to an existing user through the configured linking attribute.
func (s *FederatedMappingSuite) TestLinkingAttributeMatchesExistingUser() {
	email := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": email, "email": email})

	user := s.baseUser(s.nextSubject())
	user.Email = email

	s.Equal(map[string]string{"Email": email}, s.matchedOn(linkOn([]string{"email"}, pair("email", "email")), user),
		"the identity should match the user sharing its email")
}

// B11: the recorded link is tried first, so an identity that has linked before resolves to the user
// it linked to even when the connection's linking attributes now point at a different one.
func (s *FederatedMappingSuite) TestRecordedLinkTakesPrecedenceOverLinkingAttribute() {
	linkedEmail := s.nextSubject() + "-linked@example.com"
	linkedUserID := s.createLocalUser(map[string]interface{}{
		"username": linkedEmail, "email": linkedEmail, "password": linkPassword,
	})

	user := s.baseUser(s.nextSubject())
	user.Email = linkedEmail
	s.applyConfig(linkOn([]string{"email"}, pair("email", "email")))
	s.mockOIDC.AddUser(user)
	s.recordLink("OIDCAuthExecutor", s.idpID, user.Sub, linkedEmail)

	// A second user now owns the value the connection links on, and the identity carries it. costCenter
	// is used because it is the only non-unique attribute. Uniqueness is deployment-global, so the two
	// users could not share an email.
	otherEmail := s.nextSubject() + "-other@example.com"
	s.createLocalUser(map[string]interface{}{
		"username": otherEmail, "email": otherEmail, "costCenter": "CC-RELINK",
	})
	user.Custom["cost_centre"] = "CC-RELINK"
	s.applyConfig(linkOn([]string{"costCenter"}, pair("cost_centre", "costCenter")))
	s.mockOIDC.AddUser(user)

	step, err := s.authenticateFlow(s.matchingApp("OIDCAuthExecutor", s.idpID, "federated-relink"), user.Sub)

	s.Require().NoError(err, "the linked identity should authenticate")
	s.Require().Equal("COMPLETE", step.FlowStatus,
		"the recorded link should authenticate outright, with no prompt, got %+v", step)
	claims, err := testutils.DecodeJWT(step.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(linkedUserID, claims.Sub, "the recorded link should win over the linking attributes")
}

// B12: linking may name the *external* claim. It is resolved to its local counterpart through the
// configured mappings before the lookup runs.
func (s *FederatedMappingSuite) TestLinkingAttributeNamedByExternalClaim() {
	email := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": email, "email": email})

	user := s.baseUser(s.nextSubject())
	user.Custom["mail"] = email

	// The linking list names "mail"; the mapping says mail becomes email locally.
	s.Equal(map[string]string{"Email": email}, s.matchedOn(linkOn([]string{"mail"}, pair("mail", "email")), user),
		"the external claim name should resolve to its local counterpart")
}

// B12a: a linking attribute matches on every local attribute it reaches, so when those name two
// different users both are offered, and verification decides which one is the End-User's. One user
// holds the address as a username and the other as an email, so both rows show.
func (s *FederatedMappingSuite) TestLinkingAlternativesNamingDifferentUsersAreBothOffered() {
	email := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": email, "email": s.nextSubject() + "@example.com"})
	s.createLocalUser(map[string]interface{}{"username": s.nextSubject(), "email": email})

	user := s.baseUser(s.nextSubject())
	user.Email = email

	s.Equal(map[string]string{"Email": email, "Username": email},
		s.matchedOn(linkOn([]string{"email"}, pair("email", "username")), user),
		"a split match should offer the prompt rather than fail")
}

// B13: several linking attributes are combined, so the lookup identifies a user by all of them together.
func (s *FederatedMappingSuite) TestMultipleLinkingAttributesCombined() {
	email := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{
		"username": email, "email": email, "costCenter": "CC-100",
	})

	user := s.baseUser(s.nextSubject())
	user.Email = email
	user.Custom["cost_centre"] = "CC-100"

	s.Equal(map[string]string{"Cost center": "CC-100", "Email": email}, s.matchedOn(
		linkOn([]string{"email", "costCenter"}, pair("email", "email"), pair("cost_centre", "costCenter")),
		user), "both attributes together should match the user")
}

// B14a: two users share the linked value, so the lookup names both, and verification decides which
// one is the End-User's. costCenter is used because it is the only non-unique attribute.
// Uniqueness is deployment-global, so email could not be duplicated.
func (s *FederatedMappingSuite) TestAmbiguousLinkingAttributeOffersPrompt() {
	first := s.nextSubject() + "@example.com"
	second := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": first, "email": first, "costCenter": "CC-AMB"})
	s.createLocalUser(map[string]interface{}{"username": second, "email": second, "costCenter": "CC-AMB"})

	user := s.baseUser(s.nextSubject())
	user.Custom["cost_centre"] = "CC-AMB"

	s.Equal(map[string]string{"Cost center": "CC-AMB"},
		s.matchedOn(linkOn([]string{"costCenter"}, pair("cost_centre", "costCenter")), user),
		"an ambiguous match should offer the prompt rather than fail")
}

// B15: a linking attribute whose claim carries no value contributes nothing, and there is nothing else
// to consult.
func (s *FederatedMappingSuite) TestLinkingAttributeAbsentMatchesNobody() {
	sub := s.nextSubject()
	email := sub + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": email, "email": email})

	// The identity carries no cost_centre claim, so the configured linking attribute has no value.
	s.Empty(s.matchedOn(linkOn([]string{"costCenter"}, pair("cost_centre", "costCenter")), s.baseUser(sub)),
		"an identity with nothing to link on must not match a user")
}

// B16: with no linking configured there is nothing to match on, so a connection that maps claims but
// lists no linking attributes matches nobody. The local user is given the identity's email and it does
// not reach it: a mapped claim only joins the lookup when the connection names it as a linking attribute.
func (s *FederatedMappingSuite) TestWithoutLinkingNothingMatches() {
	sub := s.nextSubject()
	email := sub + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": email, "email": email})

	s.Empty(s.matchedOn(mapping(fedPersonType.Handle, pair("email", "email")), s.baseUser(sub)),
		"without account linking configured nothing should match")
}

// BR7: one configured linking attribute has a value and another does not. Only those with values join
// the filter, so the present one still matches the user.
func (s *FederatedMappingSuite) TestPartiallyPopulatedLinkingAttributes() {
	email := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": email, "email": email})

	user := s.baseUser(s.nextSubject())
	user.Email = email // cost_centre is absent

	s.Equal(map[string]string{"Email": email}, s.matchedOn(
		linkOn([]string{"email", "costCenter"}, pair("email", "email"), pair("cost_centre", "costCenter")),
		user), "an absent attribute should not prevent the present one matching")
}

// BR8: the attributes are combined with AND, so values that individually match different users together
// match none.
func (s *FederatedMappingSuite) TestLinkingAttributesMatchingDifferentUsersMatchNone() {
	emailOwner := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": emailOwner, "email": emailOwner})

	costOwner := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{
		"username": costOwner, "email": costOwner, "costCenter": "CC-SPLIT",
	})

	user := s.baseUser(s.nextSubject())
	user.Email = emailOwner
	user.Custom["cost_centre"] = "CC-SPLIT"

	s.Empty(s.matchedOn(
		linkOn([]string{"email", "costCenter"}, pair("email", "email"), pair("cost_centre", "costCenter")),
		user), "values matching two different users must not match either of them")
}

// BR9: the filter stringifies the claim before looking it up, so a numeric claim still matches a value
// stored as a string.
func (s *FederatedMappingSuite) TestNumericLinkingClaimMatchesStoredString() {
	email := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": email, "email": email, "costCenter": "4200"})

	user := s.baseUser(s.nextSubject())
	user.Custom["cost_centre"] = 4200

	s.Equal(map[string]string{"Cost center": "4200"},
		s.matchedOn(linkOn([]string{"costCenter"}, pair("cost_centre", "costCenter")), user),
		"a numeric claim should stringify and match the stored value")
}

// BR10: linking on an email whose casing differs from the stored value.
func (s *FederatedMappingSuite) TestEmailLinkingCaseHandling() {
	local := s.nextSubject()
	stored := local + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": stored, "email": stored})

	user := s.baseUser(s.nextSubject())
	user.Email = local + "@EXAMPLE.COM"

	// Email linking is case-sensitive: the lookup compares the claim verbatim, so an address differing
	// only in case does not match. Worth pinning because addresses are case-insensitive in practice, so
	// the same person signing in from a provider that normalises casing differently is treated as
	// unknown. Recorded as G19.
	s.Empty(s.matchedOn(linkOn([]string{"email"}, pair("email", "email")), user),
		"a differently cased address does not match")
}

// BR11: linking on a value padded with whitespace. Nothing trims the claim before the lookup.
func (s *FederatedMappingSuite) TestWhitespaceAroundLinkingValueDoesNotMatch() {
	email := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": email, "email": email})

	user := s.baseUser(s.nextSubject())
	user.Custom["mail"] = "  " + email + "  "

	s.Empty(s.matchedOn(linkOn([]string{"mail"}, pair("mail", "email")), user),
		"a padded value is not trimmed before the lookup, so it should not match the stored address")
}

// BR12: the direct endpoint has no flow to verify an account in, so a linking attribute match alone
// does not sign the identity in there. Once a flow has recorded the link, the same endpoint resolves
// it, and keeps resolving the same user on every sign-in after.
func (s *FederatedMappingSuite) TestDirectEndpointResolvesOnlyRecordedLinks() {
	email := s.nextSubject() + "@example.com"
	existingID := s.createLocalUser(map[string]interface{}{
		"username": email, "email": email, "password": linkPassword,
	})

	user := s.baseUser(s.nextSubject())
	user.Email = email
	config := linkOn([]string{"email"}, pair("email", "email"))

	status, response, code := s.authenticateDirect(config, user)
	s.Empty(response.ID, "an unlinked identity must not resolve through its linking attributes")
	s.Equal(http.StatusBadRequest, status, "an unlinked identity should surface as a client error")
	s.Equal("AUTHN-FED-1001", code, "an unlinked identity should be reported as a federated failure")

	s.recordLink("OIDCAuthExecutor", s.idpID, user.Sub, email)

	for range 2 {
		status, response, _ = s.authenticateDirect(config, user)
		s.Require().Equal(http.StatusOK, status)
		s.Equal(existingID, response.ID, "the recorded link should resolve the linked user")
	}
}

// inFrameApp creates a scenario application whose flow verifies a matched account with steps in the
// same flow rather than a called one.
func (s *FederatedMappingSuite) inFrameApp(clientID string) string {
	s.T().Helper()
	return s.createScenarioApp(
		common.InFrameLinkingFlow("auth_flow_"+clientID, "OIDCAuthExecutor", s.idpID), clientID)
}

// B17: verification runs as steps in the same flow. The connection maps its email claim onto the
// local username, which is the verification prompt's identifier, and the prompt still asks for it:
// the linking node clears the external claims for verification. Verifying the candidate links the
// identity, and the next sign-in resolves through the link with no prompt.
func (s *FederatedMappingSuite) TestInFrameVerificationLinksCandidate() {
	email := s.nextSubject() + "@example.com"
	userID := s.createLocalUser(map[string]interface{}{
		"username": email, "email": email, "password": linkPassword,
	})

	user := s.baseUser(s.nextSubject())
	user.Email = email
	s.applyConfig(linkOn([]string{"email"}, pair("email", "username")))
	s.mockOIDC.AddUser(user)
	appID := s.inFrameApp("federated-inframe-link")

	step, err := s.authenticateFlow(appID, user.Sub)
	s.Require().NoError(err)
	s.Require().NotNil(promptedDetails(step), "expected the linking prompt, got %+v", step)

	step, err = common.CompleteFlow(step.ExecutionID, nil, common.LinkAccountAction, step.ChallengeToken)
	s.Require().NoError(err)
	s.Require().Equal("INCOMPLETE", step.FlowStatus, "expected the verification prompt, got %+v", step)
	s.True(common.HasInput(step.Data.Inputs, "username"),
		"a claim the connection asserted must not answer the verification prompt's identifier")

	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": email, "password": linkPassword}, common.VerifyPasswordAction,
		step.ChallengeToken)
	s.Require().NoError(err)
	s.Require().Equal("COMPLETE", step.FlowStatus, "verifying the candidate should link it, got %+v", step)
	claims, err := testutils.DecodeJWT(step.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(userID, claims.Sub, "the flow should complete as the verified account")

	step, err = s.authenticateFlow(appID, user.Sub)
	s.Require().NoError(err)
	s.Require().Equal("COMPLETE", step.FlowStatus,
		"the recorded link should resolve the account with no prompt, got %+v", step)
	claims, err = testutils.DecodeJWT(step.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(userID, claims.Sub, "the recorded link should resolve the verified account")
}

// B18: in-frame verification by an account that is not a candidate records nothing and offers the
// linking prompt again with the error. Verifying the candidate on the retry then links it.
func (s *FederatedMappingSuite) TestInFrameVerificationByNonCandidateRetries() {
	email := s.nextSubject() + "@example.com"
	candidateID := s.createLocalUser(map[string]interface{}{
		"username": email, "email": email, "password": linkPassword,
	})
	otherEmail := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{
		"username": otherEmail, "email": otherEmail, "password": linkPassword,
	})

	user := s.baseUser(s.nextSubject())
	user.Email = email
	s.applyConfig(linkOn([]string{"email"}, pair("email", "email")))
	s.mockOIDC.AddUser(user)
	appID := s.inFrameApp("federated-inframe-other")

	step, err := s.authenticateFlow(appID, user.Sub)
	s.Require().NoError(err)
	s.Require().NotNil(promptedDetails(step), "expected the linking prompt, got %+v", step)

	step, err = common.CompleteFlow(step.ExecutionID, nil, common.LinkAccountAction, step.ChallengeToken)
	s.Require().NoError(err)
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": otherEmail, "password": linkPassword}, common.VerifyPasswordAction,
		step.ChallengeToken)
	s.Require().NoError(err)
	s.Require().NotNil(promptedDetails(step), "a non-candidate should get the linking prompt again, got %+v", step)
	s.Require().NotNil(step.Error)
	s.Equal(nonCandidateVerifiedCode, step.Error.Code)

	step, err = common.CompleteFlow(step.ExecutionID, nil, common.LinkAccountAction, step.ChallengeToken)
	s.Require().NoError(err)
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": email, "password": linkPassword}, common.VerifyPasswordAction,
		step.ChallengeToken)
	s.Require().NoError(err)
	s.Require().Equal("COMPLETE", step.FlowStatus, "verifying the candidate should link it, got %+v", step)
	claims, err := testutils.DecodeJWT(step.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(candidateID, claims.Sub, "the flow should complete as the candidate")
}
