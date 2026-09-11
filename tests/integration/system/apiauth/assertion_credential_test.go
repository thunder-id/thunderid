// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package apiauth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	assertionCredClientID     = "assertion_credential_test_client"
	assertionCredClientSecret = "assertion_credential_test_secret" //nolint:gosec // test credential
	assertionCredPassword     = "AssertionCredTest123!"
)

// AssertionCredentialTestSuite covers which self-issued tokens the API gate accepts as a bearer
// credential. It is separate from APIAuthTestSuite because it needs an OAuth application with the
// token exchange grant, which none of that suite's cases require.
type AssertionCredentialTestSuite struct {
	suite.Suite
	ouID         string
	entityTypeID string
	userID       string
	appID        string
	username     string
}

func TestAssertionCredentialTestSuite(t *testing.T) {
	suite.Run(t, new(AssertionCredentialTestSuite))
}

func (suite *AssertionCredentialTestSuite) SetupSuite() {
	unique := time.Now().UnixNano()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("assertion-cred-ou-%d", unique),
		Name:   "Assertion Credential Test OU",
	})
	suite.Require().NoError(err)
	suite.ouID = ouID

	entityType := testutils.UserType{
		Name: fmt.Sprintf("assertion-cred-person-%d", unique),
		OUID: suite.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	}
	entityTypeID, err := testutils.CreateUserType(entityType)
	suite.Require().NoError(err)
	suite.entityTypeID = entityTypeID

	suite.username = fmt.Sprintf("assertioncreduser_%d", unique)
	attrs, err := json.Marshal(map[string]interface{}{
		"username": suite.username,
		"password": assertionCredPassword,
		"email":    fmt.Sprintf("%s@example.com", suite.username),
	})
	suite.Require().NoError(err)

	userID, err := testutils.CreateUser(testutils.User{
		OUID:       suite.ouID,
		Type:       entityType.Name,
		Attributes: attrs,
	})
	suite.Require().NoError(err)
	suite.userID = userID

	// The assertion a flow issues names the application it was minted for in its aud claim, and the
	// subject token validator requires the exchanging client to be that same application, so the
	// token exchange grant has to sit on the application the flow runs for.
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             fmt.Sprintf("Assertion Credential Test App %d", unique),
		Description:      "Application for API gate assertion credential integration tests",
		OUID:             suite.ouID,
		Type:             "fullstack",
		AllowedUserTypes: []string{entityType.Name},
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                assertionCredClientID,
					"clientSecret":            assertionCredClientSecret,
					"grantTypes":              []string{"urn:ietf:params:oauth:grant-type:token-exchange"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"scopes":                  []string{"openid"},
				},
			},
		},
	})
	suite.Require().NoError(err)
	suite.appID = appID
}

func (suite *AssertionCredentialTestSuite) TearDownSuite() {
	if suite.appID != "" {
		if err := testutils.DeleteApplication(suite.appID); err != nil {
			suite.T().Logf("Failed to delete application: %v", err)
		}
	}
	if suite.userID != "" {
		if err := testutils.DeleteUser(suite.userID); err != nil {
			suite.T().Logf("Failed to delete user: %v", err)
		}
	}
	if suite.entityTypeID != "" {
		if err := testutils.DeleteUserType(suite.entityTypeID); err != nil {
			suite.T().Logf("Failed to delete user type: %v", err)
		}
	}
	if suite.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
			suite.T().Logf("Failed to delete organization unit: %v", err)
		}
	}
}

// TestAssertionIsRejectedUntilExchanged asserts the contract the API gate enforces: the assertion a
// completed sign-in flow returns is an intermediate credential to be redeemed at the token endpoint,
// not an API credential. The same authentication, once exchanged for an access token, is accepted.
//
// The two halves matter together. The assertion is signed by this server, unexpired, and names the
// authenticated subject, so it fails only because it is not an access token; exchanging it changes
// nothing about who the caller is, only the kind of token they present.
func (suite *AssertionCredentialTestSuite) TestAssertionIsRejectedUntilExchanged() {
	assertion := suite.obtainAssertion()

	resp, err := suite.getUsersMe(assertion)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	assertSecurityErrorResponse(&suite.Suite, resp, http.StatusUnauthorized, "AUTH-4010",
		"Authentication is required to access this resource")
	// A presented-but-rejected token draws the RFC 6750 invalid_token challenge, and the description
	// stays generic so the response does not disclose that an assertion was recognised as such.
	suite.Equal(
		`Bearer error="invalid_token", error_description="The access token is invalid, expired, or malformed"`,
		resp.Header.Get("WWW-Authenticate"))

	accessToken := suite.exchangeForAccessToken(assertion)

	// Pin why the exchanged token is accepted, so that loosening the gate fails this test rather
	// than silently widening what counts as an API credential.
	suite.Equal("at+jwt", jwtTypHeader(suite.T(), accessToken))

	exchangedResp, err := suite.getUsersMe(accessToken)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), exchangedResp.Body)

	suite.Equal(http.StatusOK, exchangedResp.StatusCode)
}

// obtainAssertion runs the application's authentication flow to completion and returns its assertion.
func (suite *AssertionCredentialTestSuite) obtainAssertion() string {
	flowStep, err := common.InitiateAuthenticationFlow(suite.appID, false, nil, "")
	suite.Require().NoError(err, "Failed to initiate authentication flow")
	suite.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	suite.Require().NotEmpty(flowStep.ExecutionID)

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username": suite.username,
		"password": assertionCredPassword,
	}, "action_001", flowStep.ChallengeToken)
	suite.Require().NoError(err, "Failed to complete authentication flow")
	suite.Require().Equal("COMPLETE", flowStep.FlowStatus)
	suite.Require().NotEmpty(flowStep.Assertion, "Completed flow should return an assertion")

	return flowStep.Assertion
}

// exchangeForAccessToken redeems the assertion at the token endpoint (RFC 8693) and returns the
// access token.
func (suite *AssertionCredentialTestSuite) exchangeForAccessToken(assertion string) string {
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	form.Set("subject_token", assertion)
	form.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token",
		strings.NewReader(form.Encode()))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString(
		[]byte(assertionCredClientID+":"+assertionCredClientSecret)))

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	suite.Require().Equal(http.StatusOK, resp.StatusCode, "Token exchange should succeed")

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&tokenResp))
	suite.Require().NotEmpty(tokenResp.AccessToken)

	return tokenResp.AccessToken
}

// getUsersMe calls a route that any authenticated principal may reach, so the response turns on
// authentication alone and no permission is involved.
//
// The client must come from GetHTTPClientWithToken. GetHTTPClient's transport overwrites the
// Authorization header with the admin token on every non-public path, which would silently make this
// call succeed as the admin no matter what token the test meant to present.
func (suite *AssertionCredentialTestSuite) getUsersMe(token string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, testServerURL+"/users/me", nil)
	if err != nil {
		return nil, err
	}
	return testutils.GetHTTPClientWithToken(token).Do(req)
}

// jwtTypHeader returns the typ header of a JWT.
func jwtTypHeader(t *testing.T, token string) string {
	t.Helper()

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token is not a JWT: %d segments", len(parts))
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("failed to decode JWT header: %v", err)
	}

	var header struct {
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		t.Fatalf("failed to unmarshal JWT header: %v", err)
	}
	return header.Typ
}
