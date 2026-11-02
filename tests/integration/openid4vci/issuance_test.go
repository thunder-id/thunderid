// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package openid4vci exercises the OpenID4VCI credential issuer end to end against the live test server.
package openid4vci

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	credentialConfigHandle = "integration_test_credential"
	credentialVCT          = "https://credentials.thunderid.local/IntegrationTestCredential"

	vciClientID     = "openid4vci_test_client"
	vciClientSecret = "openid4vci_test_secret"
	vciAppName      = "OpenID4VCITestApp"
	vciRedirectURI  = "https://localhost:3000"

	vciTestUsername = "vci_test_user"
	vciTestPassword = "VciTest123!"
)

var (
	vciTestOU = testutils.OrganizationUnit{
		Handle:      "openid4vci-test-ou",
		Name:        "OpenID4VCI Test OU",
		Description: "Organization unit for OpenID4VCI integration testing",
		Parent:      nil,
	}

	vciUserSchema = testutils.UserType{
		Handle:      "openid4vci-test-person",
		DisplayName: "Openid4vci Test Person",
		Schema: map[string]any{
			"username":    map[string]any{"type": "string"},
			"password":    map[string]any{"type": "string", "credential": true},
			"email":       map[string]any{"type": "string"},
			"given_name":  map[string]any{"type": "string"},
			"family_name": map[string]any{"type": "string"},
		},
	}

	vciAuthFlow = testutils.Flow{
		Name:     "OpenID4VCI Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_openid4vci_test",
		Nodes: []map[string]any{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]any{
					{
						"inputs": []map[string]any{
							{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
							{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
						},
						"action": map[string]any{"ref": "action_001", "nextNode": "credentials_auth"},
					},
				},
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]any{
					"name": "CredentialsAuthExecutor",
					"inputs": []map[string]any{
						{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]any{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
)

// OpenID4VCITestSuite runs the OpenID4VCI issuance tests against the live server.
type OpenID4VCITestSuite struct {
	suite.Suite
	ouID         string
	userSchemaID string
	authFlowID   string
	userID       string
	appID        string
	configID     string
	client       *http.Client
}

// TestOpenID4VCITestSuite is the single entrypoint that runs every Test* method.
func TestOpenID4VCITestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VCITestSuite))
}

func (ts *OpenID4VCITestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(vciTestOU)
	ts.Require().NoError(err, "create test OU")
	ts.ouID = ouID

	vciUserSchema.OUID = ouID
	schemaID, err := testutils.CreateUserType(vciUserSchema)
	ts.Require().NoError(err, "create user schema")
	ts.userSchemaID = schemaID

	flowID, err := testutils.CreateFlow(vciAuthFlow)
	ts.Require().NoError(err, "create auth flow")
	ts.authFlowID = flowID

	user := testutils.User{
		OUID: ts.ouID,
		Type: "openid4vci-test-person",
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": %q,
			"password": %q,
			"email": "vci_test_user@example.com",
			"given_name": "Ada",
			"family_name": "Lovelace"
		}`, vciTestUsername, vciTestPassword)),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "create test user")
	ts.userID = userID

	ts.appID = ts.createApp()

	validity := 3600
	configID, err := testutils.CreateCredentialConfiguration(testutils.CredentialConfiguration{
		Handle:      credentialConfigHandle,
		OUID:        ts.ouID,
		Name:        "Integration Test Credential",
		Description: "Credential configuration for OpenID4VCI integration testing",
		Format:      "dc+sd-jwt",
		VCT:         credentialVCT,
		Claims: []testutils.ClaimMapping{
			{Name: "given_name", DisplayName: "Given Name"},
			{Name: "family_name", DisplayName: "Family Name"},
		},
		ValiditySeconds: &validity,
	})
	ts.Require().NoError(err, "create credential configuration")
	ts.configID = configID
}

func (ts *OpenID4VCITestSuite) TearDownSuite() {
	if ts.configID != "" {
		_ = testutils.DeleteCredentialConfiguration(ts.configID)
	}
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
	}
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.authFlowID != "" {
		_ = testutils.DeleteFlow(ts.authFlowID)
	}
	if ts.userSchemaID != "" {
		_ = testutils.DeleteUserType(ts.userSchemaID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// createApp registers the OAuth application used to obtain user access tokens.
func (ts *OpenID4VCITestSuite) createApp() string {
	app := map[string]any{
		"name":        vciAppName,
		"description": "OpenID4VCI integration test app",
		"ouId":        ts.ouID,
		"type":        "mobile",
		// Credential issuance is restricted to wallet applications.
		"template":                  "wallet",
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"openid4vci-test-person"},
		"inboundAuthConfig": []map[string]any{
			{
				"type": "oauth2",
				"config": map[string]any{
					"clientId":                vciClientID,
					"clientSecret":            vciClientSecret,
					"redirectUris":            []string{vciRedirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_post",
					"scopes":                  []string{"openid", "profile", "email"},
				},
			},
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

	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusCreated, resp.StatusCode, "create app failed: %s", string(body))

	var created map[string]any
	ts.Require().NoError(json.Unmarshal(body, &created))
	id, _ := created["id"].(string)
	ts.Require().NotEmpty(id, "app id missing")
	return id
}

// obtainUserAccessToken runs the password-based auth-code flow and returns an
// access token whose subject is the test user.
func (ts *OpenID4VCITestSuite) obtainUserAccessToken(scope string) string {
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		vciClientID, vciRedirectURI, scope, vciTestUsername, vciTestPassword, true, vciClientSecret)
	ts.Require().NoError(err, "obtain user access token")
	ts.Require().NotNil(tokenResp)
	ts.Require().NotEmpty(tokenResp.AccessToken, "empty access token")
	return tokenResp.AccessToken
}

// credentialIssuer reads the credential_issuer identifier from issuer metadata,
// used as the holder proof audience.
func (ts *OpenID4VCITestSuite) credentialIssuer() string {
	res, meta, err := testutils.GetVCIIssuerMetadata()
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "metadata: %s", string(res.Body))
	issuer, _ := meta["credential_issuer"].(string)
	ts.Require().NotEmpty(issuer, "credential_issuer missing from metadata")
	return issuer
}

// jwtProof wraps a holder proof JWT in the single-proof credential-request shape.
func jwtProof(proof string) map[string]any {
	return map[string]any{"proof_type": "jwt", "jwt": proof}
}

// TestMetadata_ReflectsConfiguration verifies the issuer metadata advertises the
// seeded credential configuration keyed by its handle.
func (ts *OpenID4VCITestSuite) TestMetadata_ReflectsConfiguration() {
	res, meta, err := testutils.GetVCIIssuerMetadata()
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "metadata: %s", string(res.Body))

	ts.NotEmpty(meta["credential_issuer"])
	ts.NotEmpty(meta["credential_endpoint"])

	supported, ok := meta["credential_configurations_supported"].(map[string]any)
	ts.Require().Truef(ok, "credential_configurations_supported missing: %s", string(res.Body))
	cfg, ok := supported[credentialConfigHandle].(map[string]any)
	ts.Require().Truef(ok, "configuration %q not advertised: %s", credentialConfigHandle, string(res.Body))
	ts.Equal(credentialConfigHandle, cfg["scope"])
	ts.Equal(credentialVCT, cfg["vct"])
}

// TestCredentialOffer verifies an issuer-initiated offer, its deep link, and
// resolving the linked stored offer by id.
func (ts *OpenID4VCITestSuite) TestCredentialOffer() {
	res, offer, err := testutils.GetVCICredentialOffer(credentialConfigHandle)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "offer: %s", string(res.Body))

	credentialOffer, ok := offer["credential_offer"].(map[string]any)
	ts.Require().Truef(ok, "credential_offer missing: %s", string(res.Body))
	configs, ok := credentialOffer["credential_configuration_ids"].([]any)
	ts.Require().Truef(ok, "credential_configuration_ids missing: %s", string(res.Body))
	ts.Contains(configs, credentialConfigHandle)

	deepLink, _ := offer["credential_offer_uri"].(string)
	ts.Require().NotEmpty(deepLink, "credential_offer_uri missing")

	// Resolve the deep link's credential_offer_uri to the stored offer by id.
	parsed, err := url.Parse(deepLink)
	ts.Require().NoError(err, "parse deep link")
	offerURI := parsed.Query().Get("credential_offer_uri")
	ts.Require().NotEmpty(offerURI, "credential_offer_uri query param missing from deep link")
	offerID := offerURI[strings.LastIndex(offerURI, "/")+1:]
	ts.Require().NotEmpty(offerID, "offer id missing")

	storedRes, stored, err := testutils.GetVCIStoredOffer(offerID)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, storedRes.StatusCode, "stored offer: %s", string(storedRes.Body))
	storedConfigs, ok := stored["credential_configuration_ids"].([]any)
	ts.Require().Truef(ok, "stored offer credential_configuration_ids missing: %s", string(storedRes.Body))
	ts.Contains(storedConfigs, credentialConfigHandle)
}

// TestNonce verifies fresh c_nonce issuance.
func (ts *OpenID4VCITestSuite) TestNonce() {
	res, nonce, err := testutils.RequestVCINonce()
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "nonce: %s", string(res.Body))
	ts.NotEmpty(nonce)

	res2, nonce2, err := testutils.RequestVCINonce()
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, res2.StatusCode)
	ts.NotEqual(nonce, nonce2, "each c_nonce must be unique")
}

// TestIssueCredential_HappyPath drives the full issuance flow: token -> nonce ->
// holder proof -> credential, and asserts the issued SD-JWT VC binds the holder
// key and discloses the configured claims.
func (ts *OpenID4VCITestSuite) TestIssueCredential_HappyPath() {
	token := ts.obtainUserAccessToken("openid")
	issuer := ts.credentialIssuer()

	_, nonce, err := testutils.RequestVCINonce()
	ts.Require().NoError(err)

	holderKey, err := testutils.GenerateDPoPKey("ES256")
	ts.Require().NoError(err)
	proof, err := testutils.CreateVCIHolderProof(holderKey, issuer, nonce, testutils.VCIHolderProofOptions{})
	ts.Require().NoError(err)

	res, err := testutils.RequestVCICredential("Bearer", token, "", map[string]any{
		"credential_configuration_id": credentialConfigHandle,
		"proof":                       jwtProof(proof),
	})
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "credential: %s", string(res.Body))

	credential := ts.firstCredential(res.Body)
	issuerClaims, disclosed := ts.decodeSDJWT(credential)

	ts.Equal(credentialVCT, issuerClaims["vct"])
	ts.NotEmpty(issuerClaims["sub"], "issued credential must carry sub")
	cnf, ok := issuerClaims["cnf"].(map[string]any)
	ts.Require().True(ok, "cnf missing from issued credential")
	cnfJWK, ok := cnf["jwk"].(map[string]any)
	ts.Require().True(ok, "cnf.jwk missing")
	ts.Equal(holderKey.JWK, cnfJWK, "credential must bind the full holder public key")

	ts.Equal("Ada", disclosed["given_name"])
	ts.Equal("Lovelace", disclosed["family_name"])
}

// TestIssueCredential_MissingToken rejects a request with no access token.
func (ts *OpenID4VCITestSuite) TestIssueCredential_MissingToken() {
	res, err := testutils.RequestVCICredential("", "", "", map[string]any{
		"credential_configuration_id": credentialConfigHandle,
		"proof":                       jwtProof("dummy"),
	})
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, res.StatusCode)
	ts.Equal("invalid_token", ts.errorCode(res.Body))
}

// TestIssueCredential_AccessTokenInQuery rejects tokens presented in the query
// string (RFC 6750 §2).
func (ts *OpenID4VCITestSuite) TestIssueCredential_AccessTokenInQuery() {
	token := ts.obtainUserAccessToken("openid")
	body, _ := json.Marshal(map[string]any{
		"credential_configuration_id": credentialConfigHandle,
		"proof":                       jwtProof("dummy"),
	})
	req, err := http.NewRequest("POST",
		testutils.TestServerURL+"/openid4vci/credential?access_token="+token, strings.NewReader(string(body)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	ts.Equal(http.StatusUnauthorized, resp.StatusCode)
	ts.Equal("invalid_token", ts.errorCode(raw))
}

// TestIssueCredential_InvalidProofAudience rejects a proof whose aud is not the
// credential issuer and returns a fresh c_nonce for retry.
func (ts *OpenID4VCITestSuite) TestIssueCredential_InvalidProofAudience() {
	token := ts.obtainUserAccessToken("openid")
	_, nonce, err := testutils.RequestVCINonce()
	ts.Require().NoError(err)

	holderKey, err := testutils.GenerateDPoPKey("ES256")
	ts.Require().NoError(err)
	proof, err := testutils.CreateVCIHolderProof(holderKey, "https://wrong-issuer.example.com", nonce,
		testutils.VCIHolderProofOptions{})
	ts.Require().NoError(err)

	res, err := testutils.RequestVCICredential("Bearer", token, "", map[string]any{
		"credential_configuration_id": credentialConfigHandle,
		"proof":                       jwtProof(proof),
	})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, res.StatusCode)
	ts.Equal("invalid_proof", ts.errorCode(res.Body))
	freshNonce := ts.errorField(res.Body, "c_nonce")
	ts.NotEmpty(freshNonce, "error response must supply a c_nonce")
	ts.NotEqual(nonce, freshNonce, "retry c_nonce must be rotated, not the original")
}

// TestIssueCredential_InvalidNonce rejects a proof carrying an unknown nonce.
func (ts *OpenID4VCITestSuite) TestIssueCredential_InvalidNonce() {
	token := ts.obtainUserAccessToken("openid")
	issuer := ts.credentialIssuer()

	holderKey, err := testutils.GenerateDPoPKey("ES256")
	ts.Require().NoError(err)
	proof, err := testutils.CreateVCIHolderProof(holderKey, issuer, "not-a-real-nonce",
		testutils.VCIHolderProofOptions{})
	ts.Require().NoError(err)

	res, err := testutils.RequestVCICredential("Bearer", token, "", map[string]any{
		"credential_configuration_id": credentialConfigHandle,
		"proof":                       jwtProof(proof),
	})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, res.StatusCode)
	ts.Equal("invalid_nonce", ts.errorCode(res.Body))
}

// TestIssueCredential_NonceSingleUse rejects a second issuance that reuses an
// already consumed c_nonce.
func (ts *OpenID4VCITestSuite) TestIssueCredential_NonceSingleUse() {
	token := ts.obtainUserAccessToken("openid")
	issuer := ts.credentialIssuer()

	_, nonce, err := testutils.RequestVCINonce()
	ts.Require().NoError(err)
	holderKey, err := testutils.GenerateDPoPKey("ES256")
	ts.Require().NoError(err)

	proof1, err := testutils.CreateVCIHolderProof(holderKey, issuer, nonce, testutils.VCIHolderProofOptions{})
	ts.Require().NoError(err)
	res1, err := testutils.RequestVCICredential("Bearer", token, "", map[string]any{
		"credential_configuration_id": credentialConfigHandle,
		"proof":                       jwtProof(proof1),
	})
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res1.StatusCode, "first issuance: %s", string(res1.Body))

	proof2, err := testutils.CreateVCIHolderProof(holderKey, issuer, nonce, testutils.VCIHolderProofOptions{})
	ts.Require().NoError(err)
	res2, err := testutils.RequestVCICredential("Bearer", token, "", map[string]any{
		"credential_configuration_id": credentialConfigHandle,
		"proof":                       jwtProof(proof2),
	})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, res2.StatusCode)
	ts.Equal("invalid_nonce", ts.errorCode(res2.Body))
}

// TestIssueCredential_UnknownConfiguration rejects an unknown credential
// configuration id.
func (ts *OpenID4VCITestSuite) TestIssueCredential_UnknownConfiguration() {
	token := ts.obtainUserAccessToken("openid")
	issuer := ts.credentialIssuer()
	_, nonce, err := testutils.RequestVCINonce()
	ts.Require().NoError(err)
	holderKey, err := testutils.GenerateDPoPKey("ES256")
	ts.Require().NoError(err)
	proof, err := testutils.CreateVCIHolderProof(holderKey, issuer, nonce, testutils.VCIHolderProofOptions{})
	ts.Require().NoError(err)

	res, err := testutils.RequestVCICredential("Bearer", token, "", map[string]any{
		"credential_configuration_id": "does_not_exist",
		"proof":                       jwtProof(proof),
	})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, res.StatusCode)
}

// firstCredential extracts credentials[0].credential from a success response.
func (ts *OpenID4VCITestSuite) firstCredential(body []byte) string {
	var parsed struct {
		Credentials []struct {
			Credential string `json:"credential"`
		} `json:"credentials"`
	}
	ts.Require().NoErrorf(json.Unmarshal(body, &parsed), "parse credential response: %s", string(body))
	ts.Require().NotEmpty(parsed.Credentials, "no credentials returned")
	ts.Require().NotEmpty(parsed.Credentials[0].Credential, "empty credential")
	return parsed.Credentials[0].Credential
}

// decodeSDJWT splits a combined SD-JWT VC into the issuer JWT payload claims and
// the map of disclosed claim name -> value.
func (ts *OpenID4VCITestSuite) decodeSDJWT(sdjwt string) (map[string]any, map[string]any) {
	parts := strings.Split(sdjwt, "~")
	ts.Require().GreaterOrEqual(len(parts), 1)

	issuerClaims, err := testutils.DecodeJWTPayloadMap(parts[0])
	ts.Require().NoErrorf(err, "decode issuer JWT: %s", parts[0])

	disclosed := map[string]any{}
	for _, seg := range parts[1:] {
		if seg == "" {
			continue
		}
		decoded, err := base64.RawURLEncoding.DecodeString(seg)
		if err != nil {
			continue
		}
		var arr []any
		if err := json.Unmarshal(decoded, &arr); err != nil || len(arr) != 3 {
			continue
		}
		name, _ := arr[1].(string)
		disclosed[name] = arr[2]
	}
	return issuerClaims, disclosed
}

// errorCode extracts the "error" field from an OpenID4VCI error body.
func (ts *OpenID4VCITestSuite) errorCode(body []byte) string {
	return ts.errorField(body, "error")
}

// errorField extracts a string field from a JSON error body.
func (ts *OpenID4VCITestSuite) errorField(body []byte, field string) string {
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	v, _ := parsed[field].(string)
	return v
}
