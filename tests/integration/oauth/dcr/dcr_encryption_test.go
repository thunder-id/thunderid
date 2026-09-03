// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dcr

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	encOUHandle    = "dcr-enc-test-ou"
	encUserType    = "dcr-enc-person"
	encFlowHandle  = "dcr_enc_auth_flow"
	encUsername    = "dcr_enc_test_user"
	encPassword    = "SecurePass123!"
	encRedirectURI = "https://localhost:3000"
	encScope       = "openid profile email"
)

// DCREncryptionTestSuite covers the Dynamic Client Registration UserInfo and ID token signing and
// encryption metadata: the registration matrix (accepted and rejected combinations) plus the
// end-to-end UserInfo response shape that each accepted combination produces.
//
// The suite provisions its own organization unit with an explicit authentication flow so that
// DCR-registered clients inherit a login flow this suite fully controls.
type DCREncryptionTestSuite struct {
	suite.Suite
	client     *http.Client
	ouID       string
	flowID     string
	userTypeID string
	userID     string
	appIDs     []string
}

func TestDCREncryptionTestSuite(t *testing.T) {
	suite.Run(t, new(DCREncryptionTestSuite))
}

func (ts *DCREncryptionTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ts.flowID = ts.createAuthenticationFlow()
	ts.ouID = ts.createOrganizationUnitWithAuthFlow(ts.flowID)
	ts.userTypeID = ts.createUserType()
	ts.userID = ts.createUser()
}

func (ts *DCREncryptionTestSuite) TearDownSuite() {
	for _, appID := range ts.appIDs {
		if appID == "" {
			continue
		}
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application %s during teardown: %v", appID, err)
		}
	}
	if ts.userID != "" {
		testutils.DeleteUser(ts.userID)
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}
	if ts.flowID != "" {
		if err := testutils.DeleteFlow(ts.flowID); err != nil {
			ts.T().Logf("Failed to delete authentication flow during teardown: %v", err)
		}
	}
}

// createAuthenticationFlow creates a username and password authentication flow that DCR clients
// registered into this suite's organization unit inherit as their default authentication flow.
func (ts *DCREncryptionTestSuite) createAuthenticationFlow() string {
	flow := testutils.Flow{
		Name:     "DCR Encryption Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   encFlowHandle,
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
							{
								"ref":        "input_001",
								"identifier": "username",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
							{
								"ref":        "input_002",
								"identifier": "password",
								"type":       "PASSWORD_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_001",
							"nextNode": "credentials_auth",
						},
					},
				},
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_001",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_002",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
					},
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":   "auth_assert",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthAssertExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}

	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "Failed to create authentication flow")
	return flowID
}

// createOrganizationUnitWithAuthFlow creates the suite organization unit with an explicit
// authFlowId. testutils.CreateOrganizationUnit does not expose authFlowId, so the request is
// issued directly.
func (ts *DCREncryptionTestSuite) createOrganizationUnitWithAuthFlow(authFlowID string) string {
	payload := map[string]interface{}{
		"handle":      encOUHandle,
		"name":        "DCR Encryption Test OU",
		"description": "Organization unit for DCR response encryption integration tests",
		"authFlowId":  authFlowID,
	}
	body, err := json.Marshal(payload)
	ts.Require().NoError(err, "Failed to marshal organization unit payload")

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/organization-units", bytes.NewReader(body))
	ts.Require().NoError(err, "Failed to build organization unit request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to create organization unit")
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read organization unit response")
	ts.Require().Equal(http.StatusCreated, resp.StatusCode,
		"Unexpected organization unit response: %s", string(respBody))

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &created), "Failed to parse organization unit response")

	ouID, ok := created["id"].(string)
	ts.Require().True(ok, "Organization unit response does not contain an id: %s", string(respBody))
	return ouID
}

func (ts *DCREncryptionTestSuite) createUserType() string {
	userType := testutils.UserType{
		Name: encUserType,
		OUID: ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"given_name": map[string]interface{}{
				"type": "string",
			},
		},
	}

	userTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err, "Failed to create user type")
	return userTypeID
}

func (ts *DCREncryptionTestSuite) createUser() string {
	attributes := map[string]interface{}{
		"username":   encUsername,
		"password":   encPassword,
		"email":      "dcr_enc_test@example.com",
		"given_name": "Encryption",
	}
	attributesJSON, err := json.Marshal(attributes)
	ts.Require().NoError(err, "Failed to marshal user attributes")

	userID, err := testutils.CreateUser(testutils.User{
		Type:       encUserType,
		OUID:       ts.ouID,
		Attributes: attributesJSON,
	})
	ts.Require().NoError(err, "Failed to create test user")
	return userID
}

// register posts a DCR registration request and returns the decoded success body, the HTTP status
// code, and the decoded error body. Exactly one of the success or error body is non-nil.
func (ts *DCREncryptionTestSuite) register(
	request DCRRegistrationRequest,
) (*DCRRegistrationResponse, int, *DCRErrorResponse) {
	requestJSON, err := json.Marshal(request)
	ts.Require().NoError(err, "Failed to marshal DCR request")

	req, err := http.NewRequest(http.MethodPost, testServerURL+dcrEndpoint, bytes.NewReader(requestJSON))
	ts.Require().NoError(err, "Failed to build DCR request")
	req.Header.Set("Content-Type", "application/json")

	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err, "Failed to obtain admin access token")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to send DCR request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read DCR response")

	if resp.StatusCode == http.StatusCreated {
		var success DCRRegistrationResponse
		ts.Require().NoError(json.Unmarshal(body, &success), "Failed to decode DCR response: %s", string(body))
		ts.appIDs = append(ts.appIDs, success.AppID)
		return &success, resp.StatusCode, nil
	}

	var errResp DCRErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp), "Failed to decode DCR error response: %s", string(body))
	return nil, resp.StatusCode, &errResp
}

// registerSuccessfully posts a DCR registration request and fails the test unless it is accepted.
func (ts *DCREncryptionTestSuite) registerSuccessfully(request DCRRegistrationRequest) *DCRRegistrationResponse {
	response, statusCode, errResp := ts.register(request)
	if errResp != nil {
		ts.Require().FailNowf("DCR registration was rejected",
			"status=%d error=%s description=%s", statusCode, errResp.Error, errResp.ErrorDescription)
	}
	ts.Require().Equal(http.StatusCreated, statusCode)
	return response
}

// baseRequest builds a DCR registration request for this suite's organization unit that is valid
// on its own, so that each test only adds the metadata under test.
func (ts *DCREncryptionTestSuite) baseRequest(clientName string) DCRRegistrationRequest {
	return DCRRegistrationRequest{
		OUID:          ts.ouID,
		RedirectURIs:  []string{encRedirectURI},
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
		ClientName:    clientName,
		Scope:         encScope,
	}
}

// supportedSigningAlgs reads the algorithms this deployment advertises for ID token signing, so
// the tests assert against the running server's keys rather than a hardcoded list.
func (ts *DCREncryptionTestSuite) supportedSigningAlgs() []string {
	req, err := http.NewRequest(http.MethodGet, testServerURL+"/.well-known/openid-configuration", nil)
	ts.Require().NoError(err, "Failed to build discovery request")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to fetch discovery document")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read discovery document")

	var metadata struct {
		IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	}
	ts.Require().NoError(json.Unmarshal(body, &metadata), "Failed to decode discovery document")
	ts.Require().NotEmpty(metadata.IDTokenSigningAlgValuesSupported,
		"Server must advertise at least one ID token signing algorithm")
	return metadata.IDTokenSigningAlgValuesSupported
}

// buildRSAJWKS generates an RSA key pair and returns a JWK set holding the public key, together
// with the private key so that tests can decrypt JWE responses issued to it.
func buildRSAJWKS() (map[string]interface{}, *rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	eBytes := big.NewInt(int64(privateKey.PublicKey.E)).Bytes()
	jwks := map[string]interface{}{
		"keys": []interface{}{
			map[string]interface{}{
				"kty": "RSA",
				"use": "enc",
				"alg": "RSA-OAEP-256",
				"kid": "dcr-enc-test-key",
				"n":   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(eBytes),
			},
		},
	}
	return jwks, privateKey, nil
}

// TestDCRUserInfoSignedResponseAlgOnly verifies that userinfo_signed_response_alg alone is accepted
// and echoed back, with no encryption metadata added to the response.
func (ts *DCREncryptionTestSuite) TestDCRUserInfoSignedResponseAlgOnly() {
	request := ts.baseRequest("DCR UserInfo Signed Only")
	request.UserInfoSignedResponseAlg = "RS256"

	response := ts.registerSuccessfully(request)

	ts.Equal("RS256", response.UserInfoSignedResponseAlg)
	ts.Empty(response.UserInfoEncryptedResponseAlg)
	ts.Empty(response.UserInfoEncryptedResponseEnc)
	ts.Empty(response.IDTokenEncryptedResponseAlg)
	ts.Empty(response.IDTokenEncryptedResponseEnc)
}

// TestDCRUserInfoEncryptedResponseWithJWKS verifies that userinfo encryption metadata backed by an
// inline JWKS is accepted and echoed back.
func (ts *DCREncryptionTestSuite) TestDCRUserInfoEncryptedResponseWithJWKS() {
	jwks, _, err := buildRSAJWKS()
	ts.Require().NoError(err, "Failed to generate RSA JWKS")

	request := ts.baseRequest("DCR UserInfo Encrypted Only")
	request.UserInfoEncryptedResponseAlg = "RSA-OAEP-256"
	request.UserInfoEncryptedResponseEnc = "A256GCM"
	request.JWKS = jwks

	response := ts.registerSuccessfully(request)

	ts.Equal("RSA-OAEP-256", response.UserInfoEncryptedResponseAlg)
	ts.Equal("A256GCM", response.UserInfoEncryptedResponseEnc)
	ts.Empty(response.UserInfoSignedResponseAlg)
}

// TestDCRUserInfoSignedAndEncryptedResponse verifies that combining userinfo signing and encryption
// metadata is accepted and that all three values are echoed back.
func (ts *DCREncryptionTestSuite) TestDCRUserInfoSignedAndEncryptedResponse() {
	jwks, _, err := buildRSAJWKS()
	ts.Require().NoError(err, "Failed to generate RSA JWKS")

	request := ts.baseRequest("DCR UserInfo Signed And Encrypted")
	request.UserInfoSignedResponseAlg = "RS256"
	request.UserInfoEncryptedResponseAlg = "RSA-OAEP-256"
	request.UserInfoEncryptedResponseEnc = "A256GCM"
	request.JWKS = jwks

	response := ts.registerSuccessfully(request)

	ts.Equal("RS256", response.UserInfoSignedResponseAlg)
	ts.Equal("RSA-OAEP-256", response.UserInfoEncryptedResponseAlg)
	ts.Equal("A256GCM", response.UserInfoEncryptedResponseEnc)
}

// TestDCRIDTokenEncryptedResponseWithJWKS verifies that ID token encryption metadata backed by an
// inline JWKS is accepted and echoed back, without affecting the userinfo metadata.
func (ts *DCREncryptionTestSuite) TestDCRIDTokenEncryptedResponseWithJWKS() {
	jwks, _, err := buildRSAJWKS()
	ts.Require().NoError(err, "Failed to generate RSA JWKS")

	request := ts.baseRequest("DCR ID Token Encrypted")
	request.IDTokenEncryptedResponseAlg = "RSA-OAEP-256"
	request.IDTokenEncryptedResponseEnc = "A256GCM"
	request.JWKS = jwks

	response := ts.registerSuccessfully(request)

	ts.Equal("RSA-OAEP-256", response.IDTokenEncryptedResponseAlg)
	ts.Equal("A256GCM", response.IDTokenEncryptedResponseEnc)
	ts.Empty(response.UserInfoSignedResponseAlg)
	ts.Empty(response.UserInfoEncryptedResponseAlg)
	ts.Empty(response.UserInfoEncryptedResponseEnc)
}

// TestDCRIDTokenSignedResponseAlgOnly verifies that id_token_signed_response_alg alone is accepted
// and echoed back, with no encryption metadata added to the response.
func (ts *DCREncryptionTestSuite) TestDCRIDTokenSignedResponseAlgOnly() {
	request := ts.baseRequest("DCR ID Token Signed Only")
	request.IDTokenSignedResponseAlg = "RS256"

	response := ts.registerSuccessfully(request)

	ts.Equal("RS256", response.IDTokenSignedResponseAlg)
	ts.Empty(response.IDTokenEncryptedResponseAlg)
	ts.Empty(response.IDTokenEncryptedResponseEnc)
	ts.Empty(response.UserInfoSignedResponseAlg)
}

// TestDCRIDTokenSignedAndEncryptedResponse verifies that combining ID token signing and encryption
// metadata is accepted and that all three values are echoed back.
func (ts *DCREncryptionTestSuite) TestDCRIDTokenSignedAndEncryptedResponse() {
	jwks, _, err := buildRSAJWKS()
	ts.Require().NoError(err, "Failed to generate RSA JWKS")

	request := ts.baseRequest("DCR ID Token Signed And Encrypted")
	request.IDTokenSignedResponseAlg = "RS256"
	request.IDTokenEncryptedResponseAlg = "RSA-OAEP-256"
	request.IDTokenEncryptedResponseEnc = "A256GCM"
	request.JWKS = jwks

	response := ts.registerSuccessfully(request)

	ts.Equal("RS256", response.IDTokenSignedResponseAlg)
	ts.Equal("RSA-OAEP-256", response.IDTokenEncryptedResponseAlg)
	ts.Equal("A256GCM", response.IDTokenEncryptedResponseEnc)
}

// TestDCRSignedResponseAlgAdvertisedInDiscovery verifies that every algorithm the server advertises
// in id_token_signing_alg_values_supported is accepted for both ID token and userinfo signing.
func (ts *DCREncryptionTestSuite) TestDCRSignedResponseAlgAdvertisedInDiscovery() {
	for _, alg := range ts.supportedSigningAlgs() {
		ts.Run(alg, func() {
			request := ts.baseRequest("DCR Signed Alg " + alg)
			request.IDTokenSignedResponseAlg = alg
			request.UserInfoSignedResponseAlg = alg

			response := ts.registerSuccessfully(request)

			ts.Equal(alg, response.IDTokenSignedResponseAlg)
			ts.Equal(alg, response.UserInfoSignedResponseAlg)
		})
	}
}

// TestDCRUnsupportedSignedResponseAlgRejected verifies that a signing algorithm the deployment has
// no key for is rejected at registration, rather than being stored and failing at token issuance.
func (ts *DCREncryptionTestSuite) TestDCRUnsupportedSignedResponseAlgRejected() {
	testCases := []struct {
		name   string
		mutate func(*DCRRegistrationRequest)
	}{
		{
			name:   "IDTokenUnsupportedAlg",
			mutate: func(r *DCRRegistrationRequest) { r.IDTokenSignedResponseAlg = "PS512" },
		},
		{
			name:   "IDTokenNotAnAlgorithm",
			mutate: func(r *DCRRegistrationRequest) { r.IDTokenSignedResponseAlg = "bogus-alg" },
		},
		{
			name:   "UserInfoUnsupportedAlg",
			mutate: func(r *DCRRegistrationRequest) { r.UserInfoSignedResponseAlg = "PS512" },
		},
		{
			name:   "UserInfoNotAnAlgorithm",
			mutate: func(r *DCRRegistrationRequest) { r.UserInfoSignedResponseAlg = "bogus-alg" },
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			request := ts.baseRequest("DCR Unsupported Alg " + tc.name)
			tc.mutate(&request)

			_, statusCode, errResp := ts.register(request)

			ts.Require().NotNil(errResp)
			ts.Equal(http.StatusBadRequest, statusCode)
			ts.Equal("invalid_client_metadata", errResp.Error)
		})
	}
}

// TestDCRRegistrationWithoutResponseEncryptionFields verifies that a registration that omits all
// six signing and encryption fields is accepted and that none of them appear in the response.
func (ts *DCREncryptionTestSuite) TestDCRRegistrationWithoutResponseEncryptionFields() {
	response := ts.registerSuccessfully(ts.baseRequest("DCR No Response Encryption"))

	ts.Empty(response.IDTokenSignedResponseAlg)
	ts.Empty(response.UserInfoSignedResponseAlg)
	ts.Empty(response.UserInfoEncryptedResponseAlg)
	ts.Empty(response.UserInfoEncryptedResponseEnc)
	ts.Empty(response.IDTokenEncryptedResponseAlg)
	ts.Empty(response.IDTokenEncryptedResponseEnc)
}

// TestDCRUserInfoEncryptionEncWithoutAlgRejected verifies that a userinfo content-encryption
// algorithm without a key management algorithm is rejected as invalid client metadata.
func (ts *DCREncryptionTestSuite) TestDCRUserInfoEncryptionEncWithoutAlgRejected() {
	request := ts.baseRequest("DCR UserInfo Enc Without Alg")
	request.UserInfoEncryptedResponseEnc = "A256GCM"

	_, statusCode, errResp := ts.register(request)

	ts.Require().NotNil(errResp)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_client_metadata", errResp.Error)
	ts.Equal("userinfo encryptionAlg is required when encryptionEnc is set", errResp.ErrorDescription)
}

// TestDCRUserInfoEncryptionAlgWithoutEncRejected verifies that a userinfo key management algorithm
// without a content-encryption algorithm is rejected as invalid client metadata.
func (ts *DCREncryptionTestSuite) TestDCRUserInfoEncryptionAlgWithoutEncRejected() {
	jwks, _, err := buildRSAJWKS()
	ts.Require().NoError(err, "Failed to generate RSA JWKS")

	request := ts.baseRequest("DCR UserInfo Alg Without Enc")
	request.UserInfoEncryptedResponseAlg = "RSA-OAEP-256"
	request.JWKS = jwks

	_, statusCode, errResp := ts.register(request)

	ts.Require().NotNil(errResp)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_client_metadata", errResp.Error)
	ts.Equal("userinfo encryptionEnc is required when encryptionAlg is set", errResp.ErrorDescription)
}

// TestDCRUserInfoEncryptionWithoutCertificateRejected verifies that userinfo encryption without a
// jwks or jwks_uri is rejected, because the response has no recipient key to be encrypted to.
func (ts *DCREncryptionTestSuite) TestDCRUserInfoEncryptionWithoutCertificateRejected() {
	request := ts.baseRequest("DCR UserInfo Encryption Without Certificate")
	request.UserInfoEncryptedResponseAlg = "RSA-OAEP-256"
	request.UserInfoEncryptedResponseEnc = "A256GCM"

	_, statusCode, errResp := ts.register(request)

	ts.Require().NotNil(errResp)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_client_metadata", errResp.Error)
	ts.Equal("a certificate (JWKS or JWKS_URI) is required when userinfo encryption is configured",
		errResp.ErrorDescription)
}

// TestDCRUserInfoUnsupportedSigningAlgRejected verifies that a signing algorithm the deployment
// cannot sign with is rejected.
func (ts *DCREncryptionTestSuite) TestDCRUserInfoUnsupportedSigningAlgRejected() {
	request := ts.baseRequest("DCR UserInfo Unsupported Signing Alg")
	request.UserInfoSignedResponseAlg = "HS256"

	_, statusCode, errResp := ts.register(request)

	ts.Require().NotNil(errResp)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_client_metadata", errResp.Error)
	ts.Equal("userinfo signing algorithm is not supported", errResp.ErrorDescription)
}

// TestDCRUserInfoUnsupportedEncryptionAlgRejected verifies that an unknown userinfo key management
// algorithm is rejected.
func (ts *DCREncryptionTestSuite) TestDCRUserInfoUnsupportedEncryptionAlgRejected() {
	jwks, _, err := buildRSAJWKS()
	ts.Require().NoError(err, "Failed to generate RSA JWKS")

	request := ts.baseRequest("DCR UserInfo Unsupported Encryption Alg")
	request.UserInfoEncryptedResponseAlg = "BOGUS-ALG"
	request.UserInfoEncryptedResponseEnc = "A256GCM"
	request.JWKS = jwks

	_, statusCode, errResp := ts.register(request)

	ts.Require().NotNil(errResp)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_client_metadata", errResp.Error)
	ts.Equal("userinfo encryption algorithm is not supported", errResp.ErrorDescription)
}

// TestDCRUserInfoUnsupportedEncryptionEncRejected verifies that an unknown userinfo content
// encryption algorithm is rejected.
func (ts *DCREncryptionTestSuite) TestDCRUserInfoUnsupportedEncryptionEncRejected() {
	jwks, _, err := buildRSAJWKS()
	ts.Require().NoError(err, "Failed to generate RSA JWKS")

	request := ts.baseRequest("DCR UserInfo Unsupported Encryption Enc")
	request.UserInfoEncryptedResponseAlg = "RSA-OAEP-256"
	request.UserInfoEncryptedResponseEnc = "BOGUS-ENC"
	request.JWKS = jwks

	_, statusCode, errResp := ts.register(request)

	ts.Require().NotNil(errResp)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_client_metadata", errResp.Error)
	ts.Equal("userinfo content-encryption algorithm is not supported", errResp.ErrorDescription)
}

// TestDCRIDTokenEncryptionEncWithoutAlgRejected verifies that an ID token content-encryption
// algorithm without a key management algorithm is rejected. The DCR layer always derives a JWE
// response type from either ID token field, so the product reports the paired alg and enc
// requirement rather than a message naming the missing alg.
func (ts *DCREncryptionTestSuite) TestDCRIDTokenEncryptionEncWithoutAlgRejected() {
	jwks, _, err := buildRSAJWKS()
	ts.Require().NoError(err, "Failed to generate RSA JWKS")

	request := ts.baseRequest("DCR ID Token Enc Without Alg")
	request.IDTokenEncryptedResponseEnc = "A256GCM"
	request.JWKS = jwks

	_, statusCode, errResp := ts.register(request)

	ts.Require().NotNil(errResp)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_client_metadata", errResp.Error)
	ts.Equal("idToken encryptionEnc is required when encryptionAlg is set", errResp.ErrorDescription)
}

// authorizationCodeAccessToken runs a full authorization code login for a DCR-registered client and
// returns the issued access token. The UserInfo endpoint rejects client_credentials tokens, so the
// end-to-end response shape checks have to go through an interactive login.
func (ts *DCREncryptionTestSuite) authorizationCodeAccessToken(clientID, clientSecret string) string {
	authzResp, err := testutils.InitiateAuthorizationFlow(clientID, encRedirectURI, "code", encScope, "dcr_enc_state")
	ts.Require().NoError(err, "Failed to initiate authorization flow")
	defer authzResp.Body.Close()

	location := authzResp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Authorization response has no Location header")

	authID, executionID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err, "Failed to extract auth data from %s", location)

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err, "Failed to start authentication flow")

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": encUsername,
		"password": encPassword,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to execute authentication flow")
	ts.Require().NotEmpty(flowStep.Assertion, "Authentication flow did not return an assertion")

	authzResult, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err, "Failed to complete authorization")

	code, err := testutils.ExtractAuthorizationCode(authzResult.RedirectURI)
	ts.Require().NoError(err, "Failed to extract authorization code")

	tokenResult, err := testutils.RequestToken(clientID, clientSecret, code, encRedirectURI, "authorization_code")
	ts.Require().NoError(err, "Failed to exchange authorization code")
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode,
		"Unexpected token response: %s", string(tokenResult.Body))
	ts.Require().NotNil(tokenResult.Token)
	ts.Require().NotEmpty(tokenResult.Token.AccessToken, "Token response has no access token")

	return tokenResult.Token.AccessToken
}

// callUserInfo calls the UserInfo endpoint with a bearer token and returns the status code, the
// Content-Type header, and the raw body.
func (ts *DCREncryptionTestSuite) callUserInfo(accessToken string) (int, string, string) {
	req, err := http.NewRequest(http.MethodGet, testServerURL+"/oauth2/userinfo", nil)
	ts.Require().NoError(err, "Failed to build UserInfo request")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read UserInfo response")

	return resp.StatusCode, resp.Header.Get("Content-Type"), string(body)
}

// decodeJOSEHeader decodes the protected header of a compact JWS or JWE serialization.
func decodeJOSEHeader(compact string) (map[string]interface{}, error) {
	parts := strings.Split(compact, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("not a compact JOSE serialization")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to base64url-decode the protected header: %w", err)
	}
	var header map[string]interface{}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("failed to parse the protected header: %w", err)
	}
	return header, nil
}

// decryptJWE decrypts an RSA-OAEP-256 plus AES-GCM JWE compact serialization using only the Go
// standard library, and returns the protected header and the decrypted payload.
func decryptJWE(compact string, privateKey *rsa.PrivateKey) (map[string]interface{}, []byte, error) {
	parts := strings.Split(compact, ".")
	if len(parts) != 5 {
		return nil, nil, fmt.Errorf("expected 5 JWE parts, got %d", len(parts))
	}

	header, err := decodeJOSEHeader(compact)
	if err != nil {
		return nil, nil, err
	}
	if alg, _ := header["alg"].(string); alg != "RSA-OAEP-256" {
		return nil, nil, fmt.Errorf("unexpected key management algorithm %q", alg)
	}

	encryptedKey, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to base64url-decode the encrypted key: %w", err)
	}
	iv, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to base64url-decode the initialization vector: %w", err)
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to base64url-decode the ciphertext: %w", err)
	}
	tag, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to base64url-decode the authentication tag: %w", err)
	}

	cek, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, encryptedKey, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to unwrap the content encryption key: %w", err)
	}

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build the AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build the AES-GCM AEAD: %w", err)
	}

	payload, err := aead.Open(nil, iv, append(ciphertext, tag...), []byte(parts[0]))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt the JWE content: %w", err)
	}
	return header, payload, nil
}

// decodeJWTClaims decodes the claim set of a compact JWS serialization.
func decodeJWTClaims(compact string) (map[string]interface{}, error) {
	parts := strings.Split(compact, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected 3 JWS parts, got %d", len(parts))
	}
	claimBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to base64url-decode the claim set: %w", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(claimBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse the claim set: %w", err)
	}
	return claims, nil
}

// TestDCRUserInfoJWSResponseEndToEnd verifies that a DCR client registered with only
// userinfo_signed_response_alg receives a signed JWT UserInfo response whose aud claim is the
// client_id issued by the registration.
func (ts *DCREncryptionTestSuite) TestDCRUserInfoJWSResponseEndToEnd() {
	request := ts.baseRequest("DCR UserInfo JWS End To End")
	request.UserInfoSignedResponseAlg = "RS256"

	client := ts.registerSuccessfully(request)

	accessToken := ts.authorizationCodeAccessToken(client.ClientID, client.ClientSecret)
	statusCode, contentType, body := ts.callUserInfo(accessToken)

	ts.Require().Equal(http.StatusOK, statusCode, "Unexpected UserInfo response: %s", body)
	ts.Equal("application/jwt", contentType)
	ts.Require().Len(strings.Split(body, "."), 3, "Expected a compact JWS with 3 parts")

	header, err := decodeJOSEHeader(body)
	ts.Require().NoError(err)
	ts.Equal("RS256", header["alg"])

	claims, err := decodeJWTClaims(body)
	ts.Require().NoError(err)
	ts.Equal(client.ClientID, claims["aud"])
	ts.NotEmpty(claims["sub"])
}

// TestDCRUserInfoJWEResponseEndToEnd verifies that a DCR client registered with only userinfo
// encryption metadata receives an encrypted UserInfo response that decrypts, with the test's own
// RSA private key, to the JSON claim set.
func (ts *DCREncryptionTestSuite) TestDCRUserInfoJWEResponseEndToEnd() {
	jwks, privateKey, err := buildRSAJWKS()
	ts.Require().NoError(err, "Failed to generate RSA JWKS")

	request := ts.baseRequest("DCR UserInfo JWE End To End")
	request.UserInfoEncryptedResponseAlg = "RSA-OAEP-256"
	request.UserInfoEncryptedResponseEnc = "A256GCM"
	request.JWKS = jwks

	client := ts.registerSuccessfully(request)

	accessToken := ts.authorizationCodeAccessToken(client.ClientID, client.ClientSecret)
	statusCode, contentType, body := ts.callUserInfo(accessToken)

	ts.Require().Equal(http.StatusOK, statusCode, "Unexpected UserInfo response: %s", body)
	ts.Equal("application/jwt", contentType)
	ts.Require().Len(strings.Split(body, "."), 5, "Expected a compact JWE with 5 parts")

	header, payload, err := decryptJWE(body, privateKey)
	ts.Require().NoError(err, "Failed to decrypt the UserInfo JWE")
	ts.Equal("RSA-OAEP-256", header["alg"])
	ts.Equal("A256GCM", header["enc"])

	var claims map[string]interface{}
	ts.Require().NoError(json.Unmarshal(payload, &claims), "Decrypted payload is not JSON: %s", string(payload))
	ts.NotEmpty(claims["sub"])
}

// TestDCRUserInfoNestedJWTResponseEndToEnd verifies that a DCR client registered with both userinfo
// signing and encryption metadata receives a nested JWT: a JWE whose decrypted payload is itself a
// signed JWT, flagged by cty JWT in the JWE protected header.
func (ts *DCREncryptionTestSuite) TestDCRUserInfoNestedJWTResponseEndToEnd() {
	jwks, privateKey, err := buildRSAJWKS()
	ts.Require().NoError(err, "Failed to generate RSA JWKS")

	request := ts.baseRequest("DCR UserInfo Nested JWT End To End")
	request.UserInfoSignedResponseAlg = "RS256"
	request.UserInfoEncryptedResponseAlg = "RSA-OAEP-256"
	request.UserInfoEncryptedResponseEnc = "A256GCM"
	request.JWKS = jwks

	client := ts.registerSuccessfully(request)

	accessToken := ts.authorizationCodeAccessToken(client.ClientID, client.ClientSecret)
	statusCode, contentType, body := ts.callUserInfo(accessToken)

	ts.Require().Equal(http.StatusOK, statusCode, "Unexpected UserInfo response: %s", body)
	ts.Equal("application/jwt", contentType)
	ts.Require().Len(strings.Split(body, "."), 5, "Expected a compact JWE with 5 parts")

	header, payload, err := decryptJWE(body, privateKey)
	ts.Require().NoError(err, "Failed to decrypt the UserInfo nested JWT")
	ts.Equal("RSA-OAEP-256", header["alg"])
	ts.Equal("A256GCM", header["enc"])
	ts.Equal("JWT", header["cty"])

	nested := string(payload)
	ts.Require().Len(strings.Split(nested, "."), 3, "Expected the decrypted payload to be a compact JWS")

	nestedHeader, err := decodeJOSEHeader(nested)
	ts.Require().NoError(err)
	ts.Equal("RS256", nestedHeader["alg"])

	claims, err := decodeJWTClaims(nested)
	ts.Require().NoError(err)
	ts.Equal(client.ClientID, claims["aud"])
	ts.NotEmpty(claims["sub"])
}

// TestDCRUserInfoJSONResponseEndToEnd verifies that a DCR client registered without any signing or
// encryption metadata receives a plain JSON UserInfo response.
func (ts *DCREncryptionTestSuite) TestDCRUserInfoJSONResponseEndToEnd() {
	client := ts.registerSuccessfully(ts.baseRequest("DCR UserInfo JSON End To End"))

	accessToken := ts.authorizationCodeAccessToken(client.ClientID, client.ClientSecret)
	statusCode, contentType, body := ts.callUserInfo(accessToken)

	ts.Require().Equal(http.StatusOK, statusCode, "Unexpected UserInfo response: %s", body)
	ts.Contains(contentType, "application/json")

	var claims map[string]interface{}
	ts.Require().NoError(json.Unmarshal([]byte(body), &claims), "UserInfo response is not JSON: %s", body)
	ts.NotEmpty(claims["sub"])
}
