// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	teAttrFilterIssuerClientID     = "te_attr_filter_issuer_client"
	teAttrFilterIssuerClientSecret = "te_attr_filter_issuer_secret"
	teAttrFilterExchangeClientID   = "te_attr_filter_exchange_client"
	teAttrFilterExchangeSecret     = "te_attr_filter_exchange_secret"
	teAttrFilterRedirectURI        = "https://localhost:3000"
	teAttrFilterUsername           = "te_attr_filter_test_user"
	teAttrFilterPassword           = "TeAttrFilterPass1!"
)

var teAttrFilterUserType = testutils.UserType{
	Name: "te-attr-filter-person",
	Schema: map[string]interface{}{
		"username":    map[string]interface{}{"type": "string"},
		"password":    map[string]interface{}{"type": "string", "credential": true},
		"given_name":  map[string]interface{}{"type": "string"},
		"family_name": map[string]interface{}{"type": "string"},
	},
}

var teAttrFilterAuthFlow = testutils.Flow{
	Name:     "Token Exchange Attribute Filter Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_te_attr_filter_test",
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

// TokenExchangeAttributeFilterTestSuite verifies that a token-exchange-issued access token only
// carries the subject attributes selected by the exchange app's
// token.accessToken.userConfig.attributes allow-list, mirroring the allow-list enforcement already
// proven for the authorization_code grant. The subject_token presented for exchange is a real
// access token (subject_token_type=access_token) minted for a separate "issuer" app so the two
// apps' allow-lists can differ and the exchange step's own filtering can be observed independently.
type TokenExchangeAttributeFilterTestSuite struct {
	suite.Suite
	client           *http.Client
	ouID             string
	entityTypeID     string
	authFlowID       string
	userID           string
	issuerAppID      string
	exchangeAppID    string
	resourceServerID string
}

// TestTokenExchangeAttributeFilterTestSuite runs the TokenExchangeAttributeFilterTestSuite.
func TestTokenExchangeAttributeFilterTestSuite(t *testing.T) {
	suite.Run(t, new(TokenExchangeAttributeFilterTestSuite))
}

// SetupSuite creates the shared organization unit, user type, auth flow, resource server,
// issuer/exchange applications, and test user for the suite.
func (ts *TokenExchangeAttributeFilterTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "te-attr-filter-ou",
		Name:        "Token Exchange Attribute Filter OU",
		Description: "Organization unit for token exchange attribute allow-list integration tests",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	teAttrFilterUserType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(teAttrFilterUserType)
	ts.Require().NoError(err)
	ts.entityTypeID = entityTypeID

	flowID, err := testutils.CreateFlow(teAttrFilterAuthFlow)
	ts.Require().NoError(err)
	ts.authFlowID = flowID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Token Exchange Attribute Filter API",
		Description: "Resource server for token exchange attribute allow-list testing",
		Identifier:  "https://te-attr-filter.example.com",
		OUID:        ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err)
	ts.resourceServerID = rsID

	ts.issuerAppID = ts.createIssuerApplication()
	ts.exchangeAppID = ts.createExchangeApplication()

	attributesJSON, err := json.Marshal(map[string]interface{}{
		"username":    teAttrFilterUsername,
		"password":    teAttrFilterPassword,
		"given_name":  "Ada",
		"family_name": "Lovelace",
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{
		OUID:       ouID,
		Type:       "te-attr-filter-person",
		Attributes: json.RawMessage(attributesJSON),
	})
	ts.Require().NoError(err)
	ts.userID = userID
}

// TearDownSuite deletes the resources created in SetupSuite.
func (ts *TokenExchangeAttributeFilterTestSuite) TearDownSuite() {
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.exchangeAppID != "" {
		_ = testutils.DeleteApplication(ts.exchangeAppID)
	}
	if ts.issuerAppID != "" {
		_ = testutils.DeleteApplication(ts.issuerAppID)
	}
	if ts.resourceServerID != "" {
		_ = testutils.DeleteResourceServer(ts.resourceServerID)
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

// createIssuerApplication creates the authorization_code app whose access tokens carry both
// given_name and family_name; its issued access token is used as the token-exchange subject_token.
func (ts *TokenExchangeAttributeFilterTestSuite) createIssuerApplication() string {
	return ts.createApplication(map[string]interface{}{
		"clientId":                teAttrFilterIssuerClientID,
		"clientSecret":            teAttrFilterIssuerClientSecret,
		"redirectUris":            []string{teAttrFilterRedirectURI},
		"grantTypes":              []string{"authorization_code"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "client_secret_basic",
		"token": map[string]interface{}{
			"accessToken": map[string]interface{}{
				"userConfig": map[string]interface{}{
					"attributes": []string{"given_name", "family_name"},
				},
			},
		},
	}, true)
}

// createExchangeApplication creates the token-exchange app whose allow-list only permits given_name.
func (ts *TokenExchangeAttributeFilterTestSuite) createExchangeApplication() string {
	return ts.createApplication(map[string]interface{}{
		"clientId":                teAttrFilterExchangeClientID,
		"clientSecret":            teAttrFilterExchangeSecret,
		"grantTypes":              []string{"urn:ietf:params:oauth:grant-type:token-exchange"},
		"tokenEndpointAuthMethod": "client_secret_basic",
		"token": map[string]interface{}{
			"accessToken": map[string]interface{}{
				"userConfig": map[string]interface{}{
					"attributes": []string{"given_name"},
				},
			},
		},
	}, false)
}

// createApplication creates an application with the given OAuth2 inbound config, optionally
// bound to the shared auth flow.
func (ts *TokenExchangeAttributeFilterTestSuite) createApplication(
	oauthConfig map[string]interface{}, withAuthFlow bool,
) string {
	app := map[string]interface{}{
		"name":                      "TokenExchangeAttrFilterApp-" + oauthConfig["clientId"].(string),
		"description":               "Application for token exchange attribute allow-list testing",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"te-attr-filter-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type":   "oauth2",
				"config": oauthConfig,
			},
		},
	}
	if withAuthFlow {
		app["authFlowId"] = ts.authFlowID
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

// obtainIssuerAccessToken performs the full authorization code flow against the issuer app and
// returns the resulting access token (which carries given_name and family_name).
func (ts *TokenExchangeAttributeFilterTestSuite) obtainIssuerAccessToken() string {
	resp, err := testutils.InitiateAuthorizationFlow(
		teAttrFilterIssuerClientID, teAttrFilterRedirectURI, "code", "openid", "test-state")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location)

	authID, executionID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err)

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": teAttrFilterUsername,
		"password": teAttrFilterPassword,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)
	ts.Require().NotEmpty(flowStep.Assertion)

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err)

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)

	tokenResult, err := testutils.RequestToken(
		teAttrFilterIssuerClientID, teAttrFilterIssuerClientSecret, code, teAttrFilterRedirectURI, "authorization_code")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, string(tokenResult.Body))
	ts.Require().NotNil(tokenResult.Token)
	ts.Require().NotEmpty(tokenResult.Token.AccessToken)

	// Sanity check: the issuer app's own allow-list surfaces both attributes.
	claims, err := testutils.DecodeJWT(tokenResult.Token.AccessToken)
	ts.Require().NoError(err)
	ts.Require().Equal("Ada", claims.Additional["given_name"])
	ts.Require().Equal("Lovelace", claims.Additional["family_name"])

	return tokenResult.Token.AccessToken
}

// doTokenExchange submits a token-exchange grant request authenticated as the exchange app.
func (ts *TokenExchangeAttributeFilterTestSuite) doTokenExchange(formData url.Values) (int, map[string]interface{}) {
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token", strings.NewReader(formData.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(teAttrFilterExchangeClientID, teAttrFilterExchangeSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var respBody map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().NoError(json.Unmarshal(bodyBytes, &respBody), "body: %s", string(bodyBytes))
	return resp.StatusCode, respBody
}

// TestTokenExchange_AttributeAllowList verifies that only the allow-listed subject attribute
// (given_name) is present in the exchanged access token, and a non-allow-listed attribute
// (family_name) that was present on the subject_token is excluded.
func (ts *TokenExchangeAttributeFilterTestSuite) TestTokenExchange_AttributeAllowList() {
	issuerAccessToken := ts.obtainIssuerAccessToken()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", issuerAccessToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:access_token")
	formData.Set("resource", "https://te-attr-filter.example.com")

	status, body := ts.doTokenExchange(formData)
	ts.Require().Equal(http.StatusOK, status, "%v", body)

	token, ok := body["access_token"].(string)
	ts.Require().True(ok, "Response should contain access_token")

	claims, err := testutils.DecodeJWT(token)
	ts.Require().NoError(err)
	ts.Require().Equal(ts.userID, claims.Sub)

	ts.Assert().Equal("Ada", claims.Additional["given_name"], "allow-listed attribute should be present")
	ts.Assert().NotContains(claims.Additional, "family_name", "non-allow-listed attribute must be excluded")
}
