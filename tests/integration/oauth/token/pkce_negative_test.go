// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	pkceNegClientID    = "pkce_negative_test_client"
	pkceNegRedirectURI = "https://localhost:3000"
	pkceNegUsername    = "pkce_negative_test_user"
	pkceNegPassword    = "PkceNegativePass1!"
)

var pkceNegUserType = testutils.UserType{
	Handle:      "pkce-negative-person",
	DisplayName: "Pkce Negative Person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

var pkceNegAuthFlow = testutils.Flow{
	Name:     "PKCE Negative Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_pkce_negative_test",
	Nodes: []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "prompt_credentials",
		},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
				},
			},
		},
		{
			"id":   "credentials_auth",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "CredentialsAuthExecutor",
				"inputs": []map[string]interface{}{
					{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
			},
			"onSuccess": "auth_assert",
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

// PKCENegativeTestSuite covers PKCE negative paths not exercised by the happy-path PKCE usage already
// proven across the PAR, declarative-authz, and ID-JAG suites: a wrong or missing code_verifier at the
// token endpoint, an authorize request missing code_challenge on a pkceRequired application, and
// rejection of the plain code_challenge_method (only S256 is supported).
type PKCENegativeTestSuite struct {
	suite.Suite
	client       *http.Client
	ouID         string
	entityTypeID string
	authFlowID   string
	appID        string
	pkceAppID    string
	userID       string
}

// TestPKCENegativeTestSuite runs the PKCENegativeTestSuite.
func TestPKCENegativeTestSuite(t *testing.T) {
	suite.Run(t, new(PKCENegativeTestSuite))
}

// SetupSuite creates the shared organization unit, user type, auth flow, a regular app (PKCE not
// required), a pkceRequired app, and the test user.
func (ts *PKCENegativeTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "pkce-negative-test-ou",
		Name:        "PKCE Negative Test OU",
		Description: "Organization unit for PKCE negative-path integration tests",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	pkceNegUserType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(pkceNegUserType)
	ts.Require().NoError(err)
	ts.entityTypeID = entityTypeID

	flowID, err := testutils.CreateFlow(pkceNegAuthFlow)
	ts.Require().NoError(err)
	ts.authFlowID = flowID

	ts.appID = ts.createApplication(map[string]interface{}{
		"clientId":                pkceNegClientID,
		"clientSecret":            "pkce_negative_test_secret",
		"redirectUris":            []string{pkceNegRedirectURI},
		"grantTypes":              []string{"authorization_code"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "client_secret_post",
	})

	ts.pkceAppID = ts.createApplication(map[string]interface{}{
		"clientId":                "pkce_negative_required_client",
		"redirectUris":            []string{pkceNegRedirectURI},
		"grantTypes":              []string{"authorization_code"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "none",
		"publicClient":            true,
		"pkceRequired":            true,
	})

	attributesJSON, err := json.Marshal(map[string]interface{}{
		"username": pkceNegUsername,
		"password": pkceNegPassword,
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{
		OUID:       ouID,
		Type:       "pkce-negative-person",
		Attributes: json.RawMessage(attributesJSON),
	})
	ts.Require().NoError(err)
	ts.userID = userID
}

// TearDownSuite deletes the resources created in SetupSuite.
func (ts *PKCENegativeTestSuite) TearDownSuite() {
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.pkceAppID != "" {
		_ = testutils.DeleteApplication(ts.pkceAppID)
	}
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
	}
	if ts.authFlowID != "" {
		_ = testutils.DeleteFlow(ts.authFlowID)
	}
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// createApplication creates an application bound to the shared auth flow with the given OAuth2
// inbound config.
func (ts *PKCENegativeTestSuite) createApplication(oauthConfig map[string]interface{}) string {
	clientID, _ := oauthConfig["clientId"].(string)
	app := map[string]interface{}{
		"name":                      "PKCENegativeApp-" + clientID,
		"description":               "Application for PKCE negative-path integration testing",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"pkce-negative-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{"type": "oauth2", "config": oauthConfig},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(bodyBytes))

	var respData map[string]interface{}
	ts.Require().NoError(json.Unmarshal(bodyBytes, &respData))
	return respData["id"].(string)
}

// obtainAuthorizationCode runs the confidential app's authorization_code flow with the given PKCE
// parameters and returns the resulting authorization code.
func (ts *PKCENegativeTestSuite) obtainAuthorizationCode(codeChallenge, codeChallengeMethod string) string {
	resp, err := testutils.InitiateAuthorizationFlowWithPKCE(
		pkceNegClientID, pkceNegRedirectURI, "code", "openid", "test-state", "",
		codeChallenge, codeChallengeMethod)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err)

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)
	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": pkceNegUsername,
		"password": pkceNegPassword,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err)

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)
	return code
}

// TestPKCE_WrongCodeVerifier_InvalidGrant verifies that presenting a code_verifier that does not hash
// to the code_challenge established at authorize time is rejected as invalid_grant.
func (ts *PKCENegativeTestSuite) TestPKCE_WrongCodeVerifier_InvalidGrant() {
	// Fixed, distinct RFC 7636-valid verifiers rather than two random ones, so the test can't
	// spuriously pass or fail based on whether two independent random draws happen to collide.
	correctVerifier := strings.Repeat("a", 43)
	wrongVerifier := strings.Repeat("b", 43)
	codeChallenge := testutils.GenerateCodeChallenge(correctVerifier)

	code := ts.obtainAuthorizationCode(codeChallenge, "S256")

	result, err := testutils.RequestTokenWithPKCE(
		pkceNegClientID, "pkce_negative_test_secret", code, pkceNegRedirectURI, "authorization_code", wrongVerifier)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, result.StatusCode, string(result.Body))

	var errResp map[string]interface{}
	ts.Require().NoError(json.Unmarshal(result.Body, &errResp))
	ts.Equal("invalid_grant", errResp["error"])
	ts.Contains(errResp["error_description"], "Invalid code verifier")
}

// TestPKCE_MissingCodeVerifier_InvalidGrant verifies that omitting code_verifier entirely at the token
// endpoint, when PKCE was used at authorize time, is rejected as invalid_grant.
func (ts *PKCENegativeTestSuite) TestPKCE_MissingCodeVerifier_InvalidGrant() {
	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)
	codeChallenge := testutils.GenerateCodeChallenge(verifier)

	code := ts.obtainAuthorizationCode(codeChallenge, "S256")

	result, err := testutils.RequestTokenWithPKCE(
		pkceNegClientID, "pkce_negative_test_secret", code, pkceNegRedirectURI, "authorization_code", "")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, result.StatusCode, string(result.Body))

	var errResp map[string]interface{}
	ts.Require().NoError(json.Unmarshal(result.Body, &errResp))
	ts.Equal("invalid_grant", errResp["error"])
	ts.Contains(errResp["error_description"], "code_verifier is required")
}

// TestPKCE_RequiredButMissing_InvalidRequest verifies that an authorize request to a pkceRequired
// application without a code_challenge is rejected as invalid_request via an error redirect.
func (ts *PKCENegativeTestSuite) TestPKCE_RequiredButMissing_InvalidRequest() {
	resp, err := testutils.InitiateAuthorizationFlow(
		"pkce_negative_required_client", pkceNegRedirectURI, "code", "openid", "test-state")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	err = testutils.ValidateOAuth2ErrorRedirect(
		resp.Header.Get("Location"), "invalid_request", "code_challenge is required for this application")
	ts.NoError(err)
}

// TestPKCE_PlainMethod_Rejected verifies that code_challenge_method=plain is rejected as
// invalid_request; only S256 is a supported PKCE transform.
func (ts *PKCENegativeTestSuite) TestPKCE_PlainMethod_Rejected() {
	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)

	resp, err := testutils.InitiateAuthorizationFlowWithPKCE(
		pkceNegClientID, pkceNegRedirectURI, "code", "openid", "test-state", "", verifier, "plain")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	err = testutils.ValidateOAuth2ErrorRedirect(resp.Header.Get("Location"), "invalid_request", "")
	ts.NoError(err)
}
