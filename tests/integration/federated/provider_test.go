// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

/*
GitHub-specific behaviour.

GitHub is the one provider here whose endpoints are not configurable: a GitHub connection carries none,
so the harness rewrites the scheme and host of the hardcoded defaults to github_base_url in
deployment.yaml, which is pinned to the mock's port. Paths are preserved, which is why the mock mirrors
GitHub's real ones. It also has its own direct endpoints — the cross-type allowance covers OAUTH and OIDC
only, so the standard pair rejects it as AUTHN-1003.

What is GitHub-specific in the product is the email: the profile may carry no address at all, since
GitHub hides them by default, so the authenticator falls back to the separate email API and takes the
primary entry. Everything downstream depends on which address that produces.
*/

// githubIdentity registers a GitHub identity and returns the login the mock is keyed by.
func (s *FederatedMappingSuite) githubIdentity(
	publicEmail *string, emails []*testutils.GithubEmail) string {
	s.T().Helper()
	login := s.nextSubject()
	s.subCounter++
	s.mockGitHub.AddUser(&testutils.GithubUserInfo{
		Login: login,
		ID:    int64(1000 + s.subCounter),
		Name:  "GitHub Test User",
		Email: publicEmail,
	}, emails)
	return login
}

// authenticateGitHub applies a configuration to the GitHub connection and authenticates the identity
// through GitHub's own direct endpoints.
func (s *FederatedMappingSuite) authenticateGitHub(
	config *testutils.AttributeConfiguration, login string) (int, testutils.AuthenticationResponse) {
	s.T().Helper()
	s.applyConfigTo("github", s.githubIDPID, config)
	status, response, _ := s.authenticateVia(githubAuthStart, githubAuthFinish, s.githubIDPID, login)
	return status, response
}

// linkOnEmail is the configuration these scenarios share: whichever address the authenticator settles on
// is what resolves the local user, which is how the selection becomes observable.
func linkOnEmail() *testutils.AttributeConfiguration {
	config := mapping(fedPersonType.Name, pair("email", "email"))
	config.AccountLinking = &testutils.AccountLinking{Attributes: []string{"email"}}
	return config
}

// BO24: the profile carries no public address, so the primary entry from the email API is used. Note it
// is not the first entry returned, which is what makes this worth asserting.
func (s *FederatedMappingSuite) TestGitHubPrimaryEmailUsedWhenProfileHasNone() {
	primary := s.nextSubject() + "@example.com"
	other := s.nextSubject() + "@example.com"
	login := s.githubIdentity(nil, []*testutils.GithubEmail{
		{Email: other, Primary: false, Verified: true},
		{Email: primary, Primary: true, Verified: true},
	})

	existingID := s.createLocalUser(map[string]interface{}{"username": primary, "email": primary})

	status, response := s.authenticateGitHub(linkOnEmail(), login)

	s.Require().Equal(http.StatusOK, status)
	s.Equal(existingID, response.ID,
		"the primary entry should be selected, not simply the first one returned")
}

// BO25: no entry is marked primary, so no address is produced.
//
// A local user is created holding the non-primary address deliberately. Without it the scenario would
// pass whether the implementation correctly emitted nothing or incorrectly picked the non-primary
// address, since neither would resolve anyone. With it present, an implementation that fell back to a
// non-primary address would resolve that user and this test would fail — which is the only way the
// assertion means anything.
func (s *FederatedMappingSuite) TestGitHubWithoutPrimaryEmail() {
	nonPrimary := s.nextSubject() + "@example.com"
	login := s.githubIdentity(nil, []*testutils.GithubEmail{
		{Email: nonPrimary, Primary: false, Verified: true},
	})
	s.createLocalUser(map[string]interface{}{"username": nonPrimary, "email": nonPrimary})

	status, response := s.authenticateGitHub(linkOnEmail(), login)

	s.NotEqual(http.StatusOK, status,
		"with no primary address there is nothing to link on and nobody to resolve")
	s.Empty(response.ID, "a non-primary address must not be used as a fallback")
}

// BO26: several entries claim to be primary. The selection must be deterministic rather than depending
// on iteration order, or the same identity would resolve to different users between runs.
func (s *FederatedMappingSuite) TestGitHubWithMultiplePrimaryEmails() {
	first := s.nextSubject() + "@example.com"
	second := s.nextSubject() + "@example.com"
	login := s.githubIdentity(nil, []*testutils.GithubEmail{
		{Email: first, Primary: true, Verified: true},
		{Email: second, Primary: true, Verified: true},
	})

	firstID := s.createLocalUser(map[string]interface{}{"username": first, "email": first})
	s.createLocalUser(map[string]interface{}{"username": second, "email": second})

	status, response := s.authenticateGitHub(linkOnEmail(), login)

	s.Require().Equal(http.StatusOK, status)
	s.Equal(firstID, response.ID,
		"the first primary entry should win, so the selection is stable across runs")
}

// BO27: GitHub's human identifier is its login, not an address. Mapping it onto a local attribute and
// linking on that is how a GitHub identity resolves without relying on email at all.
//
// This runs through the direct endpoint rather than a flow; GithubOAuthExecutor as a flow node is a
// separate concern. What is proven here is the login claim's mapping and its use as a linking key.
func (s *FederatedMappingSuite) TestGitHubLoginMappedAndUsedForLinking() {
	primary := s.nextSubject() + "@example.com"
	login := s.githubIdentity(nil, []*testutils.GithubEmail{
		{Email: primary, Primary: true, Verified: true},
	})

	// The local user is identified by its username, which the login claim maps onto.
	existingID := s.createLocalUser(map[string]interface{}{"username": login, "email": primary})

	config := mapping(fedPersonType.Name, pair("login", "username"))
	config.AccountLinking = &testutils.AccountLinking{Attributes: []string{"login"}}
	status, response := s.authenticateGitHub(config, login)

	s.Require().Equal(http.StatusOK, status)
	s.Equal(existingID, response.ID,
		"the login claim should map onto username and resolve the user through it")
}
