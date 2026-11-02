// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"bytes"
	"crypto/mldsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	pkjServerURL          = "https://localhost:8095"
	pkjIssuer             = "https://localhost:8095" // server issuer = required aud value
	pkjResourceIdentifier = "https://pkj-test.example.com"
	pkjRSAClientID        = "pkj_rsa_client"
	pkjECClientID         = "pkj_ec_client"
	pkjPSClientID         = "pkj_ps_client"
	pkjRSAKid             = "pkj-rsa-kid"
	pkjECKid              = "pkj-ec-kid"
	pkjPSKid              = "pkj-ps-kid"
)

type PrivateKeyJWTTestSuite struct {
	suite.Suite
	adminClient      *http.Client
	rawClient        *http.Client
	ouID             string
	resourceServerID string
	rsaAppID         string
	ecAppID          string
	psAppID          string
	mldsaAppIDs      []string
	rsaKey           *testutils.DPoPKey
	ecKey            *testutils.DPoPKey
	psKey            *testutils.DPoPKey
	// mldsaKeys holds one key per ML-DSA parameter set, keyed by its JWS alg.
	mldsaKeys map[string]*mldsa.PrivateKey
}

// mldsaParameterSets maps each ML-DSA JWS alg (RFC 9964) to its parameter set.
var mldsaParameterSets = map[string]mldsa.Parameters{
	"ML-DSA-44": mldsa.MLDSA44(),
	"ML-DSA-65": mldsa.MLDSA65(),
	"ML-DSA-87": mldsa.MLDSA87(),
}

// mldsaClientID returns the client ID registered with an AKP key for the given ML-DSA alg. Each
// alg gets its own client because an inline JWKS is limited to 4096 characters, and ML-DSA public
// keys are large.
func mldsaClientID(alg string) string {
	return "pkj_" + alg + "_client"
}

// mldsaKid returns the kid of the AKP key registered for the given ML-DSA alg.
func mldsaKid(alg string) string {
	return "pkj-" + alg + "-kid"
}

func TestPrivateKeyJWTTestSuite(t *testing.T) {
	suite.Run(t, new(PrivateKeyJWTTestSuite))
}

func (ts *PrivateKeyJWTTestSuite) SetupSuite() {
	ts.adminClient = testutils.GetHTTPClient()
	ts.rawClient = testutils.GetRawHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "pkj-test-ou",
		Name:        "Private Key JWT Test OU",
		Description: "Organization unit for private_key_jwt integration tests",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Private Key JWT Resource Server",
		Description: "Resource server for private_key_jwt integration tests",
		Identifier:  pkjResourceIdentifier,
		OUID:        ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err)
	ts.resourceServerID = resourceServerID

	rsaKey, err := testutils.GenerateDPoPKey("RS256")
	ts.Require().NoError(err)
	ts.rsaKey = rsaKey

	ecKey, err := testutils.GenerateDPoPKey("ES256")
	ts.Require().NoError(err)
	ts.ecKey = ecKey

	psKey, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	ts.psKey = psKey

	ts.rsaAppID = ts.createPrivateKeyJWTApp(pkjRSAClientID, "PrivateKeyJWTRSAApp",
		ts.rsaKey.JWKS(pkjRSAKid))
	ts.ecAppID = ts.createPrivateKeyJWTApp(pkjECClientID, "PrivateKeyJWTECApp",
		ts.ecKey.JWKS(pkjECKid))
	ts.psAppID = ts.createPrivateKeyJWTApp(pkjPSClientID, "PrivateKeyJWTPSApp",
		ts.psKey.JWKS(pkjPSKid))

	ts.mldsaKeys = make(map[string]*mldsa.PrivateKey, len(mldsaParameterSets))
	for alg, params := range mldsaParameterSets {
		key, err := mldsa.GenerateKey(params)
		ts.Require().NoError(err)
		ts.mldsaKeys[alg] = key
		ts.mldsaAppIDs = append(ts.mldsaAppIDs, ts.createPrivateKeyJWTApp(mldsaClientID(alg),
			"PrivateKeyJWT"+alg+"App", akpJWKS(key, mldsaKid(alg), alg)))
	}
	ts.mldsaAppIDs = append(ts.mldsaAppIDs, ts.createPrivateKeyJWTApp(pkjMLDSABadAlgClientID,
		"PrivateKeyJWTMLDSABadAlgApp", akpJWKS(ts.mldsaKeys["ML-DSA-44"], mldsaKid("ML-DSA-44"), "ML-DSA-99")))
}

// pkjMLDSABadAlgClientID names a client whose AKP key carries an alg that is not ML-DSA.
const pkjMLDSABadAlgClientID = "pkj_mldsa_bad_alg_client"

// akpJWKS builds an inline JWKS JSON document holding one AKP public key (RFC 9964) for an ML-DSA
// key, tagged with the given kid and alg.
func akpJWKS(key *mldsa.PrivateKey, kid, alg string) string {
	jwk := map[string]any{
		"kty": "AKP",
		"use": "sig",
		"kid": kid,
		"alg": alg,
		"pub": base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
	}
	jwksJSON, _ := json.Marshal(map[string]any{"keys": []map[string]any{jwk}})
	return string(jwksJSON)
}

func (ts *PrivateKeyJWTTestSuite) TearDownSuite() {
	if ts.rsaAppID != "" {
		ts.deleteApplication(ts.rsaAppID)
	}
	if ts.ecAppID != "" {
		ts.deleteApplication(ts.ecAppID)
	}
	if ts.psAppID != "" {
		ts.deleteApplication(ts.psAppID)
	}
	for _, appID := range ts.mldsaAppIDs {
		ts.deleteApplication(appID)
	}
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete resource server: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

// createPrivateKeyJWTApp creates an OAuth2 application configured for private_key_jwt token
// endpoint authentication with an inline JWKS certificate.
func (ts *PrivateKeyJWTTestSuite) createPrivateKeyJWTApp(clientID, appName, jwksJSON string) string {
	app := map[string]interface{}{
		"name":                      appName,
		"description":               "Application for private_key_jwt integration tests",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"redirectUris":            []string{"https://localhost:3000"},
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "private_key_jwt",
					"certificate": map[string]interface{}{
						"type":  "JWKS",
						"value": jwksJSON,
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", pkjServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.adminClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Failed to create application. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&respData))

	appID, ok := respData["id"].(string)
	ts.Require().True(ok, "response did not contain an id")
	ts.T().Logf("Created private_key_jwt test application with ID: %s", appID)
	return appID
}

func (ts *PrivateKeyJWTTestSuite) deleteApplication(appID string) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/applications/%s", pkjServerURL, appID), nil)
	if err != nil {
		ts.T().Errorf("Failed to create delete request: %v", err)
		return
	}

	resp, err := ts.adminClient.Do(req)
	if err != nil {
		ts.T().Errorf("Failed to delete application: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Errorf("Failed to delete application. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
	}
}

// makeTokenRequest builds a POST request to /oauth2/token with the given form values.
func makeTokenRequest(formValues map[string]string) (*http.Request, error) {
	form := url.Values{}
	for k, v := range formValues {
		form.Set(k, v)
	}

	req, err := http.NewRequest("POST", pkjServerURL+"/oauth2/token", bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func (ts *PrivateKeyJWTTestSuite) assertTokenSuccess(resp *http.Response) {
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var respBody map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&respBody))

	accessToken, ok := respBody["access_token"].(string)
	ts.Require().True(ok, "response does not contain access_token")
	ts.Assert().NotEmpty(respBody["token_type"], "response does not contain token_type")
	ts.Assert().NotEmpty(respBody["expires_in"], "response does not contain expires_in")

	claims, err := testutils.DecodeJWT(accessToken)
	ts.Require().NoError(err)
	ts.Assert().Equal(pkjResourceIdentifier, claims.Aud)
}

func (ts *PrivateKeyJWTTestSuite) assertTokenError(resp *http.Response, expectedStatus int, expectedError string) {
	ts.Require().Equal(expectedStatus, resp.StatusCode)

	var respBody map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&respBody))

	ts.Assert().Equal(expectedError, respBody["error"])
}

func (ts *PrivateKeyJWTTestSuite) TestSuccess_RS256() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: pkjIssuer,
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: ts.rsaKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenSuccess(resp)
}

func (ts *PrivateKeyJWTTestSuite) TestSuccess_ES256() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjECClientID,
		Aud: pkjIssuer,
		Kid: pkjECKid,
		Alg: "ES256",
		Key: ts.ecKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenSuccess(resp)
}

func (ts *PrivateKeyJWTTestSuite) TestSuccess_PS256() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjPSClientID,
		Aud: pkjIssuer,
		Kid: pkjPSKid,
		Alg: "PS256",
		Key: ts.psKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenSuccess(resp)
}

// TestSuccess_MLDSA authenticates with a client assertion signed with each ML-DSA parameter set,
// verified against the client's AKP key in its inline JWKS.
func (ts *PrivateKeyJWTTestSuite) TestSuccess_MLDSA() {
	for alg := range mldsaParameterSets {
		ts.Run(alg, func() {
			assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
				Sub: mldsaClientID(alg),
				Aud: pkjIssuer,
				Kid: mldsaKid(alg),
				Alg: alg,
				Key: ts.mldsaKeys[alg],
			})

			req, err := makeTokenRequest(map[string]string{
				"grant_type":            "client_credentials",
				"resource":              pkjResourceIdentifier,
				"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
				"client_assertion":      assertion,
			})
			ts.Require().NoError(err)

			resp, err := ts.rawClient.Do(req)
			ts.Require().NoError(err)
			defer resp.Body.Close()

			ts.assertTokenSuccess(resp)
		})
	}
}

// TestMLDSAUnsupportedAKPAlg rejects an assertion whose AKP key carries an alg that is not an
// ML-DSA parameter set, even though the signature itself is valid.
func (ts *PrivateKeyJWTTestSuite) TestMLDSAUnsupportedAKPAlg() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjMLDSABadAlgClientID,
		Aud: pkjIssuer,
		Kid: mldsaKid("ML-DSA-44"),
		Alg: "ML-DSA-44",
		Key: ts.mldsaKeys["ML-DSA-44"],
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusUnauthorized, "invalid_client")
}

func (ts *PrivateKeyJWTTestSuite) TestSuccess_WithClientIDInBody() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: pkjIssuer,
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: ts.rsaKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
		"client_id":             pkjRSAClientID,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenSuccess(resp)
}

func (ts *PrivateKeyJWTTestSuite) TestSuccess_WithScopes() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: pkjIssuer,
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: ts.rsaKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"scope":                 "some_scope",
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenSuccess(resp)
}

func (ts *PrivateKeyJWTTestSuite) TestMissingClientAssertion() {
	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusUnauthorized, "invalid_client")
}

func (ts *PrivateKeyJWTTestSuite) TestMissingClientAssertionType() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: pkjIssuer,
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: ts.rsaKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":       "client_credentials",
		"resource":         pkjResourceIdentifier,
		"client_assertion": assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusUnauthorized, "invalid_client")
}

func (ts *PrivateKeyJWTTestSuite) TestUnsupportedAssertionType() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: pkjIssuer,
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: ts.rsaKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": "urn:ietf:params:oauth:client-assertion-type:saml2-bearer",
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusUnauthorized, "invalid_client")
}

func (ts *PrivateKeyJWTTestSuite) TestMalformedJWT() {
	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      "not-a-valid-jwt",
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusUnauthorized, "invalid_client")
}

func (ts *PrivateKeyJWTTestSuite) TestEmptySub() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: "",
		Aud: pkjIssuer,
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: ts.rsaKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusUnauthorized, "invalid_client")
}

func (ts *PrivateKeyJWTTestSuite) TestWrongSigningKey() {
	otherKey, err := testutils.GenerateDPoPKey("RS256")
	ts.Require().NoError(err)

	// Sign with the unregistered key but present the registered kid, so the server resolves
	// the correct public key from the app's JWKS and signature verification must fail.
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: pkjIssuer,
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: otherKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusUnauthorized, "invalid_client")
}

func (ts *PrivateKeyJWTTestSuite) TestExpiredAssertion() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: pkjIssuer,
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: ts.rsaKey.Private,
		Exp: time.Now().Unix() - 3600,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusUnauthorized, "invalid_client")
}

// Assertions violating the configured iat/exp policy (max_lifetime of 300s, max_iat_age of 60s) are rejected.
func (ts *PrivateKeyJWTTestSuite) TestAssertionTimestampPolicyViolations() {
	now := time.Now().Unix()
	cases := []struct {
		name    string
		iat     int64
		exp     int64
		omitExp bool
	}{
		{name: "IatInFuture", iat: now + 3600, exp: now + 3700},
		{name: "IatTooOld", iat: now - 3600, exp: now + 60},
		{name: "LifetimeExceedsMax", exp: now + 3600},
		{name: "MissingExp", omitExp: true},
	}

	for _, tc := range cases {
		ts.Run(tc.name, func() {
			assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
				Sub:     pkjRSAClientID,
				Aud:     pkjIssuer,
				Kid:     pkjRSAKid,
				Alg:     "RS256",
				Key:     ts.rsaKey.Private,
				IAT:     tc.iat,
				Exp:     tc.exp,
				OmitExp: tc.omitExp,
			})

			req, err := makeTokenRequest(map[string]string{
				"grant_type":            "client_credentials",
				"resource":              pkjResourceIdentifier,
				"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
				"client_assertion":      assertion,
			})
			ts.Require().NoError(err)

			resp, err := ts.rawClient.Do(req)
			ts.Require().NoError(err)
			defer resp.Body.Close()

			ts.assertTokenError(resp, http.StatusUnauthorized, "invalid_client")
		})
	}
}

func (ts *PrivateKeyJWTTestSuite) TestWrongAudience() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: "https://wrong-issuer.example.com",
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: ts.rsaKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusUnauthorized, "invalid_client")
}

func (ts *PrivateKeyJWTTestSuite) TestArrayAudience() {
	// FAPI 2.0 Security Profile Section 5.3.2.1 requires the 'aud' claim to be a single string;
	// an array value, even with the correct issuer as its only element, must be rejected.
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: []string{pkjIssuer},
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: ts.rsaKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusUnauthorized, "invalid_client")
}

func (ts *PrivateKeyJWTTestSuite) TestMissingJTI() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub:     pkjRSAClientID,
		Aud:     pkjIssuer,
		Kid:     pkjRSAKid,
		Alg:     "RS256",
		Key:     ts.rsaKey.Private,
		OmitJTI: true,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusUnauthorized, "invalid_client")
}

func (ts *PrivateKeyJWTTestSuite) TestReplayedAssertion() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: pkjIssuer,
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: ts.rsaKey.Private,
		JTI: testutils.RandomJTI(),
	})

	formValues := map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	}

	firstReq, err := makeTokenRequest(formValues)
	ts.Require().NoError(err)
	firstResp, err := ts.rawClient.Do(firstReq)
	ts.Require().NoError(err)
	defer firstResp.Body.Close()
	ts.assertTokenSuccess(firstResp)

	secondReq, err := makeTokenRequest(formValues)
	ts.Require().NoError(err)
	secondResp, err := ts.rawClient.Do(secondReq)
	ts.Require().NoError(err)
	defer secondResp.Body.Close()
	ts.assertTokenError(secondResp, http.StatusUnauthorized, "invalid_client")
}

func (ts *PrivateKeyJWTTestSuite) TestKidNotInJWKS() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: pkjIssuer,
		Kid: "unknown-kid-12345",
		Alg: "RS256",
		Key: ts.rsaKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusUnauthorized, "invalid_client")
}

func (ts *PrivateKeyJWTTestSuite) TestClientIDMismatch() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: pkjIssuer,
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: ts.rsaKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
		"client_id":             "different_client_id",
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusBadRequest, "invalid_request")
}

func (ts *PrivateKeyJWTTestSuite) TestWithBasicAuth() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: pkjIssuer,
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: ts.rsaKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
	})
	ts.Require().NoError(err)
	req.SetBasicAuth(pkjRSAClientID, "some-secret")

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusBadRequest, "invalid_request")
}

func (ts *PrivateKeyJWTTestSuite) TestWithClientSecretInBody() {
	assertion := testutils.CreateClientAssertion(testutils.ClientAssertionOptions{
		Sub: pkjRSAClientID,
		Aud: pkjIssuer,
		Kid: pkjRSAKid,
		Alg: "RS256",
		Key: ts.rsaKey.Private,
	})

	req, err := makeTokenRequest(map[string]string{
		"grant_type":            "client_credentials",
		"resource":              pkjResourceIdentifier,
		"client_assertion_type": testutils.ClientAssertionTypeJWTBearer,
		"client_assertion":      assertion,
		"client_secret":         "some-secret",
	})
	ts.Require().NoError(err)

	resp, err := ts.rawClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.assertTokenError(resp, http.StatusBadRequest, "invalid_request")
}
