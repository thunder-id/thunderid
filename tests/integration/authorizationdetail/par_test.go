// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authorizationdetail

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const parPath = "/oauth2/par"

// Details pushed through PAR, and absent from the authorization request that redeems the request_uri, are
// prompted to the user and issued in the token once approved.
func (s *AuthorizationCodeTestSuite) TestDetailsPushedThroughPARReachTheToken() {
	details := `[{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":400}]`
	status, body := s.push(details)
	s.Require().Equal(http.StatusCreated, status, string(body))
	var pushed testutils.PARResponse
	s.Require().NoError(json.Unmarshal(body, &pushed))
	s.Require().NotEmpty(pushed.RequestURI)

	resp, err := testutils.InitiateAuthorizationFlowWithRequestURI(acClientID, pushed.RequestURI)
	s.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	authID, executionID, step, purpose := s.signInExpectingDetailsPrompt(resp.Header.Get("Location"))
	s.Require().Len(purpose.Optional, 1, "the pushed detail is prompted")
	s.Equal(s.flightType, purpose.Optional[0].DetailType)
	s.Equal("Book CMB-SIN", purpose.Optional[0].Summary)

	final := s.decide(executionID, step.ChallengeToken, purpose, true)
	token := s.exchange(authID, final.Assertion, "")
	s.Require().Equal(http.StatusOK, token.status, string(token.body))
	s.Require().Len(token.details, 1)
	s.JSONEq(`{"type":"`+s.flightType+`","route":"CMB-SIN","maxFare":400}`, string(token.details[0]))
}

// Invalid details are refused when they are pushed, before a request_uri is issued for them.
func (s *AuthorizationCodeTestSuite) TestInvalidDetailsAreRefusedAtThePAREndpoint() {
	for name, details := range map[string]string{
		"an unknown type":     `[{"type":"` + uniqueTypeValue("unknown") + `"}]`,
		"an undeclared field": `[{"type":"` + s.flightType + `","route":"CMB-SIN","note":"x"}]`,
	} {
		status, body := s.push(details)
		s.Equal(http.StatusBadRequest, status, name)
		s.Equal(errorInvalidAuthorizationDetails, oauthError(s.T(), body).Error, name)
	}
}

// push sends a pushed authorization request for the given details.
func (s *AuthorizationCodeTestSuite) push(details string) (int, []byte) {
	form := url.Values{}
	form.Set("response_type", "code")
	form.Set("redirect_uri", acRedirectURI)
	form.Set("scope", "openid")
	form.Set("state", "authz_details_par_state")
	form.Set("resource", acResource)
	form.Set("authorization_details", details)
	return postForm(s.T(), parPath, form, acClientID, acClientSecret)
}
