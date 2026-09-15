// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	idTokenAlgClientID       = "id_token_alg_test_client"
	idTokenAlgClientSecret   = "id_token_alg_test_secret"
	idTokenAlgRedirectURI    = "https://localhost:3000/id-token-alg-callback"
	idTokenAlgResourceServer = "https://id-token-alg.example.com"
	idTokenAlgUsername       = "idtokenalguser"
	idTokenAlgPassword       = "testpassword"
)

var idTokenAlgTestOU = testutils.OrganizationUnit{
	Handle:      "id-token-alg-test-ou",
	Name:        "ID Token Signing Alg Test Organization Unit",
	Description: "Organization unit for ID token signing algorithm testing",
	Parent:      nil,
}

var idTokenAlgUserSchema = testutils.UserType{
	Name: "id_token_alg_test_person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"email":    map[string]interface{}{"type": "string"},
	},
}

var idTokenAlgTestUser = testutils.User{
	Type: idTokenAlgUserSchema.Name,
	Attributes: json.RawMessage(`{
		"username": "idtokenalguser",
		"password": "testpassword",
		"email": "idtokenalguser@example.com"
	}`),
}

var idTokenAlgFlow = testutils.Flow{
	Name:     "ID Token Signing Alg Test Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_id_token_alg_test",
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
			"onSuccess": "authorization_check",
		},
		{
			"id":        "authorization_check",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
			"onSuccess": "auth_assert",
		},
		{
			"id":        "auth_assert",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": "end",
		},
		{
			"id":   "end",
			"type": "END",
		},
	},
}

// IDTokenSigningAlgTestSuite verifies that the signing algorithm a client configures is the one
// that actually signs its ID tokens, asserted on the issued token's JWS header.
type IDTokenSigningAlgTestSuite struct {
	suite.Suite
	applicationID    string
	authFlowID       string
	ouID             string
	userSchemaID     string
	resourceServerID string
	userIDs          []string
}

func TestIDTokenSigningAlgTestSuite(t *testing.T) {
	suite.Run(t, new(IDTokenSigningAlgTestSuite))
}

func (ts *IDTokenSigningAlgTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(idTokenAlgTestOU)
	ts.Require().NoError(err, "failed to create OU")
	ts.ouID = ouID

	schema := idTokenAlgUserSchema
	schema.OUID = ts.ouID
	schemaID, err := testutils.CreateUserType(schema)
	ts.Require().NoError(err, "failed to create user schema")
	ts.userSchemaID = schemaID

	user := idTokenAlgTestUser
	user.OUID = ts.ouID
	userIDs, err := testutils.CreateMultipleUsers(user)
	ts.Require().NoError(err, "failed to create test user")
	ts.userIDs = userIDs

	flowID, err := testutils.CreateFlow(idTokenAlgFlow)
	ts.Require().NoError(err, "failed to create test flow")
	ts.authFlowID = flowID

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "ID Token Alg Resource Server",
		Description: "Resource server for ID token signing algorithm tests",
		Identifier:  idTokenAlgResourceServer,
		OUID:        ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err, "failed to create resource server")
	ts.resourceServerID = resourceServerID
}

func (ts *IDTokenSigningAlgTestSuite) TearDownSuite() {
	ts.deleteApplication()
	if err := testutils.CleanupUsers(ts.userIDs); err != nil {
		ts.T().Logf("failed to cleanup users: %v", err)
	}
	if ts.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.authFlowID); err != nil {
			ts.T().Logf("failed to delete flow: %v", err)
		}
	}
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("failed to delete resource server: %v", err)
		}
	}
	if ts.userSchemaID != "" {
		if err := testutils.DeleteUserType(ts.userSchemaID); err != nil {
			ts.T().Logf("failed to delete user schema: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("failed to delete OU: %v", err)
		}
	}
}

// TestIDTokenSignedWithConfiguredAlg asserts that every algorithm the deployment advertises can be
// selected by a client and appears in the issued ID token's "alg" header, with a "kid" naming a
// key the JWKS endpoint publishes for that algorithm. Without the kid check a token could be
// signed by one key while advertising another, which no client could verify.
func (ts *IDTokenSigningAlgTestSuite) TestIDTokenSignedWithConfiguredAlg() {
	jwks := ts.fetchJWKS()

	for _, alg := range ts.supportedIDTokenSigningAlgs() {
		ts.Run(alg, func() {
			ts.createApplication(alg)
			defer ts.deleteApplication()

			idToken := ts.obtainIDToken()

			header, err := testutils.DecodeJWTHeaderMap(idToken)
			ts.Require().NoError(err, "ID token header must be decodable")

			ts.Equal(alg, header["alg"], "ID token must be signed with the configured algorithm")
			ts.Equal("JWT", header["typ"], "ID token must carry the JWT typ header")

			kid, ok := header["kid"].(string)
			ts.Require().True(ok, "ID token must carry a kid header")
			ts.Equal(alg, jwks[kid],
				"kid must identify a published key whose algorithm matches the token's alg")
		})
	}
}

// TestIDTokenDefaultsToServerSigningAlg asserts that a client which configures no algorithm still
// receives a usable ID token, signed by the deployment's preferred key.
func (ts *IDTokenSigningAlgTestSuite) TestIDTokenDefaultsToServerSigningAlg() {
	jwks := ts.fetchJWKS()

	ts.createApplication("")
	defer ts.deleteApplication()

	idToken := ts.obtainIDToken()

	header, err := testutils.DecodeJWTHeaderMap(idToken)
	ts.Require().NoError(err, "ID token header must be decodable")

	ts.Equal("JWT", header["typ"], "ID token must carry the JWT typ header")

	kid, ok := header["kid"].(string)
	ts.Require().True(ok, "ID token must carry a kid header")
	ts.Equal(jwks[kid], header["alg"],
		"an unconfigured client must still be signed by a published key matching the alg header")
}

// obtainIDToken runs the full authorization_code flow and returns the issued ID token.
func (ts *IDTokenSigningAlgTestSuite) obtainIDToken() string {
	resp, err := ts.initiateAuthorize()
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusFound, resp.StatusCode)
	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location)

	authID, flowID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err)

	flowStep, err := testutils.ExecuteAuthenticationFlow(flowID, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	flowStep, err = testutils.ExecuteAuthenticationFlow(flowID, map[string]string{
		"username": idTokenAlgUsername,
		"password": idTokenAlgPassword,
	}, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "flow should complete after valid credentials")

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err)

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err, "authorization code must be in the redirect")

	tokenResult, err := testutils.RequestTokenWithResource(
		idTokenAlgClientID, idTokenAlgClientSecret, code, idTokenAlgRedirectURI,
		"authorization_code", idTokenAlgResourceServer)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, "token exchange should succeed")
	ts.Require().NotNil(tokenResult.Token)
	ts.Require().NotEmpty(tokenResult.Token.IDToken, "ID token must be present in token response")

	return tokenResult.Token.IDToken
}

// createApplication registers the test application, configuring signingAlg when one is given.
func (ts *IDTokenSigningAlgTestSuite) createApplication(signingAlg string) {
	oauthConfig := map[string]interface{}{
		"clientId":                idTokenAlgClientID,
		"clientSecret":            idTokenAlgClientSecret,
		"redirectUris":            []string{idTokenAlgRedirectURI},
		"grantTypes":              []string{"authorization_code"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "client_secret_basic",
	}
	if signingAlg != "" {
		oauthConfig["token"] = map[string]interface{}{
			"idToken": map[string]interface{}{"signingAlg": signingAlg},
		}
	}

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:                      "ID Token Signing Alg Test App",
		Description:               "Application for ID token signing algorithm tests",
		IsRegistrationFlowEnabled: false,
		OUID:                      ts.ouID,
		AuthFlowID:                ts.authFlowID,
		ClientID:                  idTokenAlgClientID,
		ClientSecret:              idTokenAlgClientSecret,
		RedirectURIs:              []string{idTokenAlgRedirectURI},
		AllowedUserTypes:          []string{idTokenAlgUserSchema.Name},
		InboundAuthConfig: []map[string]interface{}{
			{"type": "oauth2", "config": oauthConfig},
		},
	})
	ts.Require().NoError(err, "failed to create test application")
	ts.applicationID = appID
}

// deleteApplication removes the test application if one is registered.
func (ts *IDTokenSigningAlgTestSuite) deleteApplication() {
	if ts.applicationID == "" {
		return
	}
	if err := testutils.DeleteApplication(ts.applicationID); err != nil {
		ts.T().Logf("failed to delete application: %v", err)
	}
	ts.applicationID = ""
}

// supportedIDTokenSigningAlgs reads the algorithms this deployment advertises, so the assertions
// follow the running server's configured keys rather than a hardcoded list.
func (ts *IDTokenSigningAlgTestSuite) supportedIDTokenSigningAlgs() []string {
	var metadata struct {
		IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	}
	ts.getJSON(testutils.TestServerURL+"/.well-known/openid-configuration", &metadata)
	ts.Require().NotEmpty(metadata.IDTokenSigningAlgValuesSupported,
		"server must advertise at least one ID token signing algorithm")
	return metadata.IDTokenSigningAlgValuesSupported
}

// fetchJWKS returns the published signing keys as a kid to algorithm map.
func (ts *IDTokenSigningAlgTestSuite) fetchJWKS() map[string]string {
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Alg string `json:"alg"`
		} `json:"keys"`
	}
	ts.getJSON(testutils.TestServerURL+"/oauth2/jwks", &jwks)
	ts.Require().NotEmpty(jwks.Keys, "server must publish at least one signing key")

	byKid := make(map[string]string, len(jwks.Keys))
	for _, key := range jwks.Keys {
		byKid[key.Kid] = key.Alg
	}
	return byKid
}

// getJSON fetches url and decodes the response body into target.
func (ts *IDTokenSigningAlgTestSuite) getJSON(url string, target interface{}) {
	resp, err := testutils.GetHTTPClient().Get(url)
	ts.Require().NoError(err, "failed to fetch %s", url)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "failed to read response from %s", url)
	ts.Require().NoError(json.Unmarshal(body, target), "failed to decode response from %s", url)
}

func (ts *IDTokenSigningAlgTestSuite) initiateAuthorize() (*http.Response, error) {
	params := url.Values{}
	params.Set("client_id", idTokenAlgClientID)
	params.Set("redirect_uri", idTokenAlgRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	params.Set("state", "id_token_alg_state")

	req, err := http.NewRequest(http.MethodGet,
		testutils.TestServerURL+"/oauth2/authorize?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	return testutils.GetNoRedirectHTTPClient().Do(req)
}
