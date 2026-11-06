// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package openid4vp exercises the OpenID4VP verifier end to end against the live test server.
package openid4vp

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	presentationDefinitionHandle = "integration_test_presentation"
	presentationVCT              = "https://credentials.thunderid.local/IntegrationTestCredential"

	// expectedClientID is derived by the verifier from the ecdsa signing cert
	// under the default client_id_scheme (x509_san_dns) whose DNS SAN is localhost.
	expectedClientID = "x509_san_dns:localhost"
)

var (
	vpTestOU = testutils.OrganizationUnit{
		Handle:      "openid4vp-test-ou",
		Name:        "OpenID4VP Test OU",
		Description: "Organization unit for OpenID4VP integration testing",
		Parent:      nil,
	}
)

// OpenID4VPTestSuite runs the OpenID4VP verification tests against the live server.
type OpenID4VPTestSuite struct {
	suite.Suite
	ouID         string
	definitionID string
}

// TestOpenID4VPTestSuite is the single entrypoint that runs every Test* method.
func TestOpenID4VPTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPTestSuite))
}

func (ts *OpenID4VPTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(vpTestOU)
	ts.Require().NoError(err, "create test OU")
	ts.ouID = ouID

	enforceTrustedIssuer := false
	definitionID, err := testutils.CreatePresentationDefinition(testutils.PresentationDefinition{
		Handle:               presentationDefinitionHandle,
		OUID:                 ts.ouID,
		Name:                 "Integration Test Presentation",
		Description:          "Presentation definition for OpenID4VP integration testing",
		VCT:                  presentationVCT,
		Format:               "dc+sd-jwt",
		RequestedClaims:      []string{"given_name", "family_name"},
		MandatoryClaims:      []string{"given_name"},
		EnforceTrustedIssuer: &enforceTrustedIssuer,
	})
	ts.Require().NoError(err, "create presentation definition")
	ts.definitionID = definitionID
}

func (ts *OpenID4VPTestSuite) TearDownSuite() {
	if ts.definitionID != "" {
		_ = testutils.DeletePresentationDefinition(ts.definitionID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// initiateSession starts a verification session and returns the parsed response.
func (ts *OpenID4VPTestSuite) initiateSession() *testutils.VPInitiateResponse {
	res, init, err := testutils.InitiateVP(presentationDefinitionHandle)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "initiate: %s", string(res.Body))
	ts.Require().NotEmpty(init.TxnID, "txn_id missing")
	return init
}

// TestInitiate verifies a verification session is created with a wallet deep
// link and status URL.
func (ts *OpenID4VPTestSuite) TestInitiate() {
	init := ts.initiateSession()
	ts.NotEmpty(init.WalletURL, "wallet_url missing")
	ts.True(strings.HasPrefix(init.WalletURL, "openid4vp://"), "wallet_url should be an openid4vp deep link")
	ts.Equal("/openid4vp/status/"+init.TxnID, init.StatusURL)
	ts.NotEmpty(init.ExpiresAt, "expires_at missing")
}

// TestRequestObject verifies the signed request object served to the wallet
// carries the client id, nonce, DCQL query and the verifier encryption key.
func (ts *OpenID4VPTestSuite) TestRequestObject() {
	init := ts.initiateSession()

	res, jar, ctype, err := testutils.FetchVPRequestObject(init.TxnID)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "request object: %s", string(res.Body))
	ts.Contains(ctype, "application/oauth-authz-req+jwt")

	req, err := testutils.ParseVPRequestObject(jar)
	ts.Require().NoError(err)
	ts.Equal(expectedClientID, req.ClientID)
	ts.NotEmpty(req.Nonce, "nonce missing from request object")
	ts.Equal(presentationVCT, req.VCT)
	ts.Equal(presentationDefinitionHandle, req.CredentialID)
	ts.NotNil(req.EncKey, "verifier encryption key missing")
}

// TestStatus_PendingBeforeResponse verifies a fresh session reports PENDING.
func (ts *OpenID4VPTestSuite) TestStatus_PendingBeforeResponse() {
	init := ts.initiateSession()

	res, status, err := testutils.GetVPStatus(init.TxnID)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "status: %s", string(res.Body))
	ts.Equal("PENDING", status.Status)
	ts.Empty(status.ResultToken, "result token must not be present before verification")
}

// TestFullVerification_HappyPath drives the full flow: initiate, fetch request
// object, present a self-signed SD-JWT VC from the wallet, and read the result
// token from the status endpoint.
func (ts *OpenID4VPTestSuite) TestFullVerification_HappyPath() {
	init := ts.initiateSession()

	_, jar, _, err := testutils.FetchVPRequestObject(init.TxnID)
	ts.Require().NoError(err)
	req, err := testutils.ParseVPRequestObject(jar)
	ts.Require().NoError(err)

	wallet, err := testutils.NewVPWallet()
	ts.Require().NoError(err)
	presentation, err := wallet.BuildPresentation(req, map[string]any{
		"given_name":  "Ada",
		"family_name": "Lovelace",
	})
	ts.Require().NoError(err)
	response, err := wallet.EncryptResponse(req, presentation)
	ts.Require().NoError(err)

	res, err := testutils.SubmitVPResponse(init.TxnID, response)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "submit response: %s", string(res.Body))

	status, err := testutils.PollVPStatusUntilTerminal(init.TxnID, 10, 200*time.Millisecond)
	ts.Require().NoError(err)
	ts.Require().Equalf("COMPLETED", status.Status, "final status; error=%q", status.Error)
	ts.Require().NotEmpty(status.ResultToken, "result_token missing")

	claims, err := testutils.DecodeJWTPayloadMap(status.ResultToken)
	ts.Require().NoError(err)
	ts.Equal(presentationDefinitionHandle, claims["definition_id"])

	claimsJSON, _ := json.Marshal(claims)
	ts.Contains(string(claimsJSON), "Ada", "result token should carry the verified given_name")
	ts.Contains(string(claimsJSON), "Lovelace", "result token should carry the verified family_name")

	// The result is delivered once: a second poll finds no session.
	res2, _, err := testutils.GetVPStatus(init.TxnID)
	ts.Require().NoError(err)
	ts.Equal(http.StatusNotFound, res2.StatusCode, "session must be consumed after result delivery")
}

// TestWalletErrorResponse verifies a wallet-posted error marks the session FAILED.
func (ts *OpenID4VPTestSuite) TestWalletErrorResponse() {
	init := ts.initiateSession()

	res, err := testutils.SubmitVPError(init.TxnID, "access_denied", "user cancelled the request")
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "submit error: %s", string(res.Body))

	status, err := testutils.PollVPStatusUntilTerminal(init.TxnID, 10, 200*time.Millisecond)
	ts.Require().NoError(err)
	ts.Equal("FAILED", status.Status)
}

// TestResponse_MalformedJWE rejects a response body that is not a valid JWE.
func (ts *OpenID4VPTestSuite) TestResponse_MalformedJWE() {
	init := ts.initiateSession()

	res, err := testutils.SubmitVPResponse(init.TxnID, "this-is-not-a-valid-jwe")
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, res.StatusCode)
}

// TestStatus_UnknownState returns 404 for an unknown transaction id.
func (ts *OpenID4VPTestSuite) TestStatus_UnknownState() {
	res, _, err := testutils.GetVPStatus("00000000-0000-0000-0000-000000000000")
	ts.Require().NoError(err)
	ts.Equal(http.StatusNotFound, res.StatusCode)
}

// TestInitiate_UnknownDefinition rejects an unknown presentation definition.
func (ts *OpenID4VPTestSuite) TestInitiate_UnknownDefinition() {
	res, _, err := testutils.InitiateVP("no_such_definition")
	ts.Require().NoError(err)
	ts.NotEqual(http.StatusOK, res.StatusCode)
	ts.True(res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusBadRequest,
		"expected 400 or 404, got %d: %s", res.StatusCode, string(res.Body))
}

// TestInitiate_MissingDefinitionID rejects a request with no definition id.
func (ts *OpenID4VPTestSuite) TestInitiate_MissingDefinitionID() {
	res, _, err := testutils.InitiateVP("")
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, res.StatusCode)
}
