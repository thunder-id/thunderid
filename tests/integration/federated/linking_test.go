// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

/*
Account linking through the direct federated endpoints.

Linking is observable here in a way it is not in a flow: the response carries no claims, only which
local user the identity resolved to, so the returned id *is* the result. These use
/auth/oauth/standard/*, which is the only direct federated route — an OIDC connection reaches it through
the cross-type allowance in validateIDPType, which is why the existing OIDC suite uses the same path.
*/

const (
	directAuthStart  = "/auth/oauth/standard/start"
	directAuthFinish = "/auth/oauth/standard/finish"

	// GitHub is not reachable through the standard endpoints: the cross-type allowance in
	// validateIDPType covers OAUTH and OIDC only, so a GitHub connection there is rejected as
	// AUTHN-1003. It has its own dedicated pair.
	githubAuthStart  = "/auth/oauth/github/start"
	githubAuthFinish = "/auth/oauth/github/finish"
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

// authenticateDirectVia drives the direct endpoints against any connection, for the OAuth and GitHub
// scenarios whose identities live on a different mock.
func (s *FederatedMappingSuite) authenticateDirectVia(
	idpID, sub string,
) (int, testutils.AuthenticationResponse, string) {
	s.T().Helper()
	return s.authenticateVia(directAuthStart, directAuthFinish, idpID, sub)
}

// authenticateVia drives a given pair of direct endpoints, since GitHub has its own.
func (s *FederatedMappingSuite) authenticateVia(
	startPath, finishPath, idpID, sub string,
) (int, testutils.AuthenticationResponse, string) {
	s.T().Helper()
	s.activeSub = sub

	status, body := s.postJSON(startPath, map[string]interface{}{"idpId": idpID})
	s.Require().Equal(http.StatusOK, status, "failed to start federated authentication: %s", string(body))

	var start struct {
		SessionToken string `json:"sessionToken"`
		RedirectURL  string `json:"redirectUrl"`
	}
	s.Require().NoError(json.Unmarshal(body, &start))

	code, _, err := testutils.SimulateFederatedOAuthFlow(start.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")

	status, body = s.postJSON(finishPath, map[string]interface{}{
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
		Type:       fedPersonType.Name,
		OUID:       s.ouID,
		Attributes: mustJSON(attributes),
	})
	s.Require().NoError(err, "failed to create local user %v", attributes)
	s.config.CreatedUserIDs = append(s.config.CreatedUserIDs, userID)
	return userID
}

// linkOn builds a configuration that maps the claims a scenario needs and links on the named attributes.
func linkOn(attributes []string, pairs ...testutils.AttributeMapping) *testutils.AttributeConfiguration {
	config := mapping(fedPersonType.Name, pairs...)
	config.AccountLinking = &testutils.AccountLinking{Attributes: attributes}
	return config
}

// B10: a federated identity whose subject matches nobody resolves to an existing user through the
// configured linking attribute.
func (s *FederatedMappingSuite) TestLinksToExistingUserByMappedAttribute() {
	email := s.nextSubject() + "@example.com"
	existingID := s.createLocalUser(map[string]interface{}{"username": email, "email": email})

	user := s.baseUser(s.nextSubject())
	user.Email = email

	status, response, _ := s.authenticateDirect(linkOn([]string{"email"}, pair("email", "email")), user)

	s.Require().Equal(http.StatusOK, status)
	s.Equal(existingID, response.ID, "the identity should resolve to the user sharing its email")
}

// B11: the subject is tried first, so an identity whose sub already matches a user resolves to that user
// even when its linking attribute points at a different one.
func (s *FederatedMappingSuite) TestSubTakesPrecedenceOverLinkingAttribute() {
	sub := s.nextSubject()
	subEmail := sub + "-bysub@example.com"
	subUserID := s.createLocalUser(map[string]interface{}{
		"username": subEmail, "email": subEmail, "sub": sub,
	})

	linkEmail := s.nextSubject() + "-bylink@example.com"
	linkUserID := s.createLocalUser(map[string]interface{}{"username": linkEmail, "email": linkEmail})

	user := s.baseUser(sub)
	user.Email = linkEmail

	status, response, _ := s.authenticateDirect(linkOn([]string{"email"}, pair("email", "email")), user)

	s.Require().Equal(http.StatusOK, status)
	s.Equal(subUserID, response.ID, "the subject match should win")
	s.NotEqual(linkUserID, response.ID, "the linking attribute should not override a subject match")
}

// B12: linking may name the *external* claim. It is resolved to its local counterpart through the
// configured mappings before the lookup runs.
func (s *FederatedMappingSuite) TestLinkingAttributeNamedByExternalClaim() {
	email := s.nextSubject() + "@example.com"
	existingID := s.createLocalUser(map[string]interface{}{"username": email, "email": email})

	user := s.baseUser(s.nextSubject())
	user.Custom["mail"] = email

	// The linking list names "mail"; the mapping says mail becomes email locally.
	status, response, _ := s.authenticateDirect(linkOn([]string{"mail"}, pair("mail", "email")), user)

	s.Require().Equal(http.StatusOK, status)
	s.Equal(existingID, response.ID, "the external claim name should resolve to its local counterpart")
}

// B13: several linking attributes are combined, so the lookup identifies a user by all of them together.
func (s *FederatedMappingSuite) TestMultipleLinkingAttributesCombined() {
	email := s.nextSubject() + "@example.com"
	existingID := s.createLocalUser(map[string]interface{}{
		"username": email, "email": email, "costCenter": "CC-100",
	})

	user := s.baseUser(s.nextSubject())
	user.Email = email
	user.Custom["cost_centre"] = "CC-100"

	status, response, _ := s.authenticateDirect(
		linkOn([]string{"email", "costCenter"}, pair("email", "email"), pair("cost_centre", "costCenter")),
		user)

	s.Require().Equal(http.StatusOK, status)
	s.Equal(existingID, response.ID, "both attributes together should resolve the user")
}

// B14a: two users share the linked value, so the lookup cannot identify one. The direct endpoint reports
// that as a client error rather than picking arbitrarily. costCenter is used because it is the only
// non-unique attribute — uniqueness is deployment-global, so email could not be duplicated.
func (s *FederatedMappingSuite) TestAmbiguousLinkingAttributeIsReported() {
	first := s.nextSubject() + "@example.com"
	second := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": first, "email": first, "costCenter": "CC-AMB"})
	s.createLocalUser(map[string]interface{}{"username": second, "email": second, "costCenter": "CC-AMB"})

	user := s.baseUser(s.nextSubject())
	user.Custom["cost_centre"] = "CC-AMB"

	status, response, code := s.authenticateDirect(
		linkOn([]string{"costCenter"}, pair("cost_centre", "costCenter")), user)

	// It must not pick one of them.
	s.Empty(response.ID, "an ambiguous link must not resolve to an arbitrary user")

	// The manager classifies ambiguity as a *client* error (AUTHN-MGR-1009, ClientErrorType), but
	// mapCredentialsGetAttributesError maps only two codes and sends everything else to its default
	// branch, so the classification is discarded and the caller sees an opaque 500. Asserted exactly,
	// because an earlier draft asserted merely "not 200" and hid this. Recorded as G18.
	s.Equal(http.StatusInternalServerError, status,
		"ambiguity currently surfaces as an internal error rather than a client one")
	s.Equal("SSE-5000", code, "the ambiguity code AUTHN-MGR-1009 does not reach the caller")
}

// B15: a linking attribute whose claim carries no value contributes nothing, so the lookup falls back to
// the subject filter — which is the only path that does fall back.
func (s *FederatedMappingSuite) TestLinkingAttributeAbsentFallsBackToSub() {
	sub := s.nextSubject()
	email := sub + "@example.com"
	existingID := s.createLocalUser(map[string]interface{}{
		"username": email, "email": email, "sub": sub,
	})

	// The identity carries no cost_centre claim, so the configured linking attribute has no value.
	user := s.baseUser(sub)

	status, response, _ := s.authenticateDirect(
		linkOn([]string{"costCenter"}, pair("cost_centre", "costCenter")), user)

	s.Require().Equal(http.StatusOK, status)
	s.Equal(existingID, response.ID, "with no linking value the subject filter should still resolve")
}

// B16: with no linking configured the subject is the only thing consulted.
func (s *FederatedMappingSuite) TestWithoutLinkingOnlySubResolves() {
	sub := s.nextSubject()
	email := sub + "@example.com"
	existingID := s.createLocalUser(map[string]interface{}{
		"username": email, "email": email, "sub": sub,
	})

	user := s.baseUser(sub)
	status, response, _ := s.authenticateDirect(mapping(fedPersonType.Name, pair("email", "email")), user)

	s.Require().Equal(http.StatusOK, status)
	s.Equal(existingID, response.ID, "the subject alone should resolve the user")
}

// BR7: one configured linking attribute has a value and another does not. Only those with values join
// the filter, so the present one still resolves the user.
func (s *FederatedMappingSuite) TestPartiallyPopulatedLinkingAttributes() {
	email := s.nextSubject() + "@example.com"
	existingID := s.createLocalUser(map[string]interface{}{"username": email, "email": email})

	user := s.baseUser(s.nextSubject())
	user.Email = email // cost_centre is absent

	status, response, _ := s.authenticateDirect(
		linkOn([]string{"email", "costCenter"}, pair("email", "email"), pair("cost_centre", "costCenter")),
		user)

	s.Require().Equal(http.StatusOK, status)
	s.Equal(existingID, response.ID, "an absent attribute should not prevent the present one resolving")
}

// BR8: the attributes are combined with AND, so values that individually match different users together
// match none.
func (s *FederatedMappingSuite) TestLinkingAttributesMatchingDifferentUsersResolveNone() {
	emailOwner := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": emailOwner, "email": emailOwner})

	costOwner := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{
		"username": costOwner, "email": costOwner, "costCenter": "CC-SPLIT",
	})

	user := s.baseUser(s.nextSubject())
	user.Email = emailOwner
	user.Custom["cost_centre"] = "CC-SPLIT"

	status, _, _ := s.authenticateDirect(
		linkOn([]string{"email", "costCenter"}, pair("email", "email"), pair("cost_centre", "costCenter")),
		user)

	s.NotEqual(http.StatusOK, status,
		"values matching two different users must not resolve either of them")
}

// BR9: the filter stringifies the claim before looking it up, so a numeric claim still matches a value
// stored as a string.
func (s *FederatedMappingSuite) TestNumericLinkingClaimMatchesStoredString() {
	email := s.nextSubject() + "@example.com"
	existingID := s.createLocalUser(map[string]interface{}{
		"username": email, "email": email, "costCenter": "4200",
	})

	user := s.baseUser(s.nextSubject())
	user.Custom["cost_centre"] = 4200

	status, response, _ := s.authenticateDirect(
		linkOn([]string{"costCenter"}, pair("cost_centre", "costCenter")), user)

	s.Require().Equal(http.StatusOK, status)
	s.Equal(existingID, response.ID, "a numeric claim should stringify and match the stored value")
}

// BR10: linking on an email whose casing differs from the stored value.
func (s *FederatedMappingSuite) TestEmailLinkingCaseHandling() {
	local := s.nextSubject()
	stored := local + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": stored, "email": stored})

	user := s.baseUser(s.nextSubject())
	user.Email = local + "@EXAMPLE.COM"

	status, _, _ := s.authenticateDirect(linkOn([]string{"email"}, pair("email", "email")), user)

	// Email linking is case-sensitive: the lookup compares the claim verbatim, so an address differing
	// only in case does not link and the subject fallback finds nobody. Worth pinning because addresses
	// are case-insensitive in practice, so the same person signing in from a provider that normalises
	// casing differently is treated as unknown. Recorded as G19.
	s.NotEqual(http.StatusOK, status,
		"a differently cased address does not link, so the identity resolves to nobody")
}

// BR11: linking on a value padded with whitespace. Nothing trims the claim before the lookup.
func (s *FederatedMappingSuite) TestWhitespaceAroundLinkingValueDoesNotMatch() {
	email := s.nextSubject() + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": email, "email": email})

	user := s.baseUser(s.nextSubject())
	user.Custom["mail"] = "  " + email + "  "

	status, _, _ := s.authenticateDirect(linkOn([]string{"mail"}, pair("mail", "email")), user)

	s.NotEqual(http.StatusOK, status,
		"a padded value is not trimmed before the lookup, so it should not match the stored address")
}

// BR12: signing in again after a first successful link resolves the same user rather than creating or
// matching another.
func (s *FederatedMappingSuite) TestRepeatedLoginResolvesTheSameUser() {
	email := s.nextSubject() + "@example.com"
	existingID := s.createLocalUser(map[string]interface{}{"username": email, "email": email})

	user := s.baseUser(s.nextSubject())
	user.Email = email
	config := linkOn([]string{"email"}, pair("email", "email"))

	firstStatus, first, _ := s.authenticateDirect(config, user)
	s.Require().Equal(http.StatusOK, firstStatus)
	s.Require().Equal(existingID, first.ID)

	secondStatus, second, _ := s.authenticateDirect(config, user)
	s.Require().Equal(http.StatusOK, secondStatus)
	s.Equal(existingID, second.ID, "a repeated sign-in should resolve the same user")
}
