// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const trustedIssuerVCT = "https://credentials.thunderid.local/TrustedIssuerTestCredential"

var trustedIssuerTestOU = testutils.OrganizationUnit{
	Handle:      "openid4vp-trusted-issuer-ou",
	Name:        "OpenID4VP Trusted Issuer OU",
	Description: "Organization unit for OpenID4VP trusted issuer testing",
	Parent:      nil,
}

// OpenID4VPTrustedIssuerTestSuite verifies issuer trust end to end: a presentation definition
// accepts a credential only when the issuer's x5c chain verifies against a certificate
// registered on that definition.
type OpenID4VPTrustedIssuerTestSuite struct {
	suite.Suite
	ouID          string
	definitionIDs []string
}

func TestOpenID4VPTrustedIssuerTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPTrustedIssuerTestSuite))
}

func (ts *OpenID4VPTrustedIssuerTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(trustedIssuerTestOU)
	ts.Require().NoError(err, "create test OU")
	ts.ouID = ouID
}

func (ts *OpenID4VPTrustedIssuerTestSuite) TearDownSuite() {
	for _, id := range ts.definitionIDs {
		if err := testutils.DeletePresentationDefinition(id); err != nil {
			ts.T().Logf("Failed to delete presentation definition %s: %v", id, err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

// definition returns a definition body that requests the test credential.
func (ts *OpenID4VPTrustedIssuerTestSuite) definition(
	handle string, enforce bool, authorities ...testutils.TrustedAuthority,
) testutils.PresentationDefinition {
	return testutils.PresentationDefinition{
		Handle:               handle,
		OUID:                 ts.ouID,
		Name:                 "Trusted Issuer Test Presentation",
		VCT:                  trustedIssuerVCT,
		Format:               "dc+sd-jwt",
		RequestedClaims:      []string{"given_name", "family_name"},
		MandatoryClaims:      []string{"given_name"},
		EnforceTrustedIssuer: &enforce,
		TrustedAuthorities:   authorities,
	}
}

// createDefinition creates a definition and registers it for cleanup.
func (ts *OpenID4VPTrustedIssuerTestSuite) createDefinition(def testutils.PresentationDefinition) string {
	id, err := testutils.CreatePresentationDefinition(def)
	ts.Require().NoError(err, "create presentation definition")
	ts.definitionIDs = append(ts.definitionIDs, id)
	return id
}

// otherCertificate mints a certificate unrelated to any wallet issuer.
func (ts *OpenID4VPTrustedIssuerTestSuite) otherCertificate() string {
	certificate, err := testutils.NewSelfSignedCertificatePEM("Unrelated Issuer CA", time.Now().Add(24*time.Hour))
	ts.Require().NoError(err)
	return certificate
}

// present runs a verification for handle with a presentation from wallet.
func (ts *OpenID4VPTrustedIssuerTestSuite) present(
	handle string, wallet *testutils.VPWallet,
) (*testutils.VPRequestObject, int, *testutils.VPStatusResponse) {
	result, err := testutils.RunVPVerification(handle, wallet,
		map[string]any{"given_name": "Ada", "family_name": "Lovelace"})
	ts.Require().NoError(err)
	return result.Request, result.SubmitStatusCode, result.Status
}

// trustedAuthoritiesOf returns the DCQL trusted_authorities of the request object's credential query.
func trustedAuthoritiesOf(req *testutils.VPRequestObject) ([]any, bool) {
	dcql, _ := req.Raw["dcql_query"].(map[string]any)
	credentials, _ := dcql["credentials"].([]any)
	if len(credentials) == 0 {
		return nil, false
	}
	credential, _ := credentials[0].(map[string]any)
	authorities, ok := credential["trusted_authorities"].([]any)
	return authorities, ok
}

func (ts *OpenID4VPTrustedIssuerTestSuite) TestPinnedIssuerCertificateVerifies() {
	wallet, err := testutils.NewVPWallet()
	ts.Require().NoError(err)
	ts.createDefinition(ts.definition("trusted_issuer_pinned", true,
		testutils.TrustedAuthority{Name: "wallet-issuer", Certificate: wallet.IssuerCertificatePEM()}))

	req, submitStatus, status := ts.present("trusted_issuer_pinned", wallet)

	ts.Equal(http.StatusOK, submitStatus)
	ts.Require().Equalf("COMPLETED", status.Status, "final status; error=%q", status.Error)
	ts.NotEmpty(status.ResultToken, "result_token missing")

	// The wallet is told which issuer key the definition trusts.
	keyID, err := wallet.IssuerKeyID()
	ts.Require().NoError(err)
	authorities, ok := trustedAuthoritiesOf(req)
	ts.Require().True(ok, "trusted_authorities missing from the DCQL query")
	ts.Equal([]any{map[string]any{"type": "aki", "values": []any{keyID}}}, authorities)
}

func (ts *OpenID4VPTrustedIssuerTestSuite) TestCertificateIsScopedToDefinition() {
	wallet, err := testutils.NewVPWallet()
	ts.Require().NoError(err)
	ts.createDefinition(ts.definition("trusted_issuer_scope_a", true,
		testutils.TrustedAuthority{Name: "wallet-issuer", Certificate: wallet.IssuerCertificatePEM()}))
	ts.createDefinition(ts.definition("trusted_issuer_scope_b", true,
		testutils.TrustedAuthority{Name: "other-issuer", Certificate: ts.otherCertificate()}))

	_, submitStatus, status := ts.present("trusted_issuer_scope_a", wallet)
	ts.Equal(http.StatusOK, submitStatus)
	ts.Equalf("COMPLETED", status.Status, "definition A trusts the wallet issuer; error=%q", status.Error)

	_, submitStatus, status = ts.present("trusted_issuer_scope_b", wallet)
	ts.Equal(http.StatusBadRequest, submitStatus)
	ts.Equal("FAILED", status.Status, "definition B must not trust a certificate registered only on definition A")
}

func (ts *OpenID4VPTrustedIssuerTestSuite) TestUpdateDrivesIssuerTrust() {
	wallet, err := testutils.NewVPWallet()
	ts.Require().NoError(err)
	def := ts.definition("trusted_issuer_updated", true,
		testutils.TrustedAuthority{Name: "wallet-issuer", Certificate: wallet.IssuerCertificatePEM()})
	id := ts.createDefinition(def)

	// Naming the authority without its certificate keeps the stored certificate.
	def.Name = "Renamed Trusted Issuer Presentation"
	def.TrustedAuthorities = []testutils.TrustedAuthority{{Name: "wallet-issuer"}}
	ts.Require().NoError(testutils.UpdatePresentationDefinition(id, def))

	_, submitStatus, status := ts.present("trusted_issuer_updated", wallet)
	ts.Equal(http.StatusOK, submitStatus)
	ts.Equalf("COMPLETED", status.Status, "kept certificate must still verify; error=%q", status.Error)

	// Replacing the authority stops trusting the wallet issuer.
	def.TrustedAuthorities = []testutils.TrustedAuthority{{Name: "other-issuer", Certificate: ts.otherCertificate()}}
	ts.Require().NoError(testutils.UpdatePresentationDefinition(id, def))

	_, submitStatus, status = ts.present("trusted_issuer_updated", wallet)
	ts.Equal(http.StatusBadRequest, submitStatus)
	ts.Equal("FAILED", status.Status)
}

func (ts *OpenID4VPTrustedIssuerTestSuite) TestCertificatesUnusedWhenNotEnforced() {
	wallet, err := testutils.NewVPWallet()
	ts.Require().NoError(err)
	ts.createDefinition(ts.definition("trusted_issuer_not_enforced", false,
		testutils.TrustedAuthority{Name: "other-issuer", Certificate: ts.otherCertificate()}))

	req, submitStatus, status := ts.present("trusted_issuer_not_enforced", wallet)

	ts.Equal(http.StatusOK, submitStatus)
	ts.Equalf("COMPLETED", status.Status, "error=%q", status.Error)
	_, ok := trustedAuthoritiesOf(req)
	ts.False(ok, "trusted_authorities must not be advertised when issuer trust is not enforced")
}
